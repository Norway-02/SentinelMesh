package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sentinelmesh/sentinelmesh/internal/config"
	"github.com/sentinelmesh/sentinelmesh/internal/messaging"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/postgres"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Scheduler failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.DatabaseURL == "" {
		return fmt.Errorf("SENTINEL_DB_URL is required for scheduler")
	}

	if cfg.NATSURL == "" {
		return fmt.Errorf("SENTINEL_NATS_URL is required for scheduler")
	}

	// Initialize Logging & Tracing
	observability.InitLogging("sentinelmesh-scheduler", os.Stdout, slog.LevelInfo)
	obsCfg := observability.DefaultConfig("sentinelmesh-scheduler")
	tp, err := observability.InitTracing(obsCfg, nil)
	if err != nil {
		slog.Warn("Failed to initialize distributed tracing", slog.Any("error", err))
	} else {
		defer observability.ShutdownTracing(context.Background())
		_ = tp
	}

	slog.Info("Starting SentinelMesh Scheduler (Stage 7)", "env", cfg.Environment)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Prometheus Metrics Server on port 9091
	observability.StartMetricsServer(ctx, 9091)

	// Connect to Database
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	ctxPing, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := db.PingContext(ctxPing); err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	slog.Info("PostgreSQL connection established successfully")

	// Connect to NATS
	nc, err := messaging.ConnectNATS(cfg.NATSURL)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()
	slog.Info("Connected to NATS JetStream successfully")

	// Setup Repositories
	txManager := postgres.NewTxManager(db)
	agentRepo := postgres.NewAgentRepository(db)
	runRepo := postgres.NewRunRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)
	assignmentRepo := postgres.NewAssignmentRepository(db)

	// Setup Resource Provider
	var resourceProvider scheduler.ResourceProvider
	kubeconfigPath := os.Getenv("SENTINEL_KUBECONFIG")
	
	if kubeconfigPath != "" {
		slog.Info("SENTINEL_KUBECONFIG is set, initializing KubernetesResourceProvider")
		k8sCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			slog.Error("Failed to build kubeconfig", "error", err)
			os.Exit(1)
		}
		k8sClient, err := client.New(k8sCfg, client.Options{})
		if err != nil {
			slog.Error("Failed to create k8s client", "error", err)
			os.Exit(1)
		}
		resourceProvider = scheduler.NewKubernetesResourceProvider(k8sClient)
	} else {
		slog.Info("SENTINEL_KUBECONFIG is NOT set, falling back to StaticResourceProvider")
		resourceProvider = scheduler.NewStaticResourceProvider()
	}

	// Initialize algorithms and service
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, resourceProvider)

	// Setup outbox publisher just in case (scheduler emits events too!)
	publisher := outbox.NewPublisher(outboxRepo, nc, 2*time.Second)
	publisher.Start()
	defer publisher.Stop()

	// Consume RunCreated events
	js := nc.JetStream()

	cons, err := js.CreateOrUpdateConsumer(ctx, "SENTINEL_RUN", jetstream.ConsumerConfig{
		Durable:       "sentinel-scheduler",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "sentinel.run.v1.created",
		MaxDeliver:    5, // Retry up to 5 times
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	iter, err := cons.Messages()
	if err != nil {
		return fmt.Errorf("failed to get message iterator: %w", err)
	}

	slog.Info("Scheduler consumer started, waiting for RunCreated events...")

	go func() {
		for {
			msg, err := iter.Next()
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			// Extract W3C trace context from NATS headers
			msgCtx := observability.ExtractNATSHeaders(ctx, msg.Headers())

			// Parse Event
			var payload struct {
				RunID string `json:"run_id"`
			}

			// payload should be the generic event which wraps the payload.
			var event struct {
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(msg.Data(), &event); err != nil {
				slog.ErrorContext(msgCtx, "Failed to decode event wrapper", "error", err)
				msg.Ack() // Invalid message format, discard
				continue
			}

			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				slog.ErrorContext(msgCtx, "Failed to decode RunCreatedPayload", "error", err)
				msg.Ack()
				continue
			}

			msgCtx = observability.WithRunID(msgCtx, payload.RunID)
			msgCtx, span := observability.StartConsumerSpan(msgCtx, "scheduler.consume_run_created")

			// Process Scheduling
			slog.InfoContext(msgCtx, "Received RunCreated event", "run_id", payload.RunID, "msg_id", msg.Headers().Get("Nats-Msg-Id"))
			err = schedSvc.ScheduleRun(msgCtx, payload.RunID)
			
			if err != nil {
				observability.RecordSpanError(span, err)
				span.End()
				slog.ErrorContext(msgCtx, "Failed to schedule run", "run_id", payload.RunID, "error", err)
				msg.Nak()
			} else {
				span.End()
				msg.Ack()
				slog.InfoContext(msgCtx, "Successfully processed RunCreated event", "run_id", payload.RunID)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Scheduler shutting down...")
	return nil
}
