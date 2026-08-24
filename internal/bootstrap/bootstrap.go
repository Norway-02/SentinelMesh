package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	pb "github.com/sentinelmesh/sentinelmesh/api/v1"
	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/application"
	"github.com/sentinelmesh/sentinelmesh/internal/config"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/messaging"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/onlinepolicy"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/postgres"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	grpchandler "github.com/sentinelmesh/sentinelmesh/internal/transport/grpc"
	resthandler "github.com/sentinelmesh/sentinelmesh/internal/transport/rest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Dependencies struct {
	TxManager  repository.TxManager
	AgentRepo  repository.AgentRepository
	RunRepo    repository.RunRepository
	OutboxRepo outbox.Repository
	NATSClient *messaging.NATSClient
	Publisher  *outbox.Publisher
	DB         *sql.DB // Can be nil if using memory
}

// InitializeDependencies wires the persistence layer based on configuration
func InitializeDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	var natsClient *messaging.NATSClient
	if cfg.NATSURL != "" {
		nc, err := messaging.ConnectNATS(cfg.NATSURL)
		if err != nil {
			slog.Warn("Failed to connect to NATS, outbox events will accumulate", slog.Any("error", err))
		} else {
			natsClient = nc
			slog.Info("Connected to NATS JetStream successfully")
		}
	} else {
		slog.Info("NATS URL not provided. Event publication is disabled.")
	}

	if cfg.DatabaseURL == "" {
		slog.Info("Database URL not provided. Using in-memory repositories.")
		return &Dependencies{
			TxManager:  memory.NewTxManager(),
			AgentRepo:  memory.NewAgentRepository(),
			RunRepo:    memory.NewRunRepository(),
			OutboxRepo: outbox.NewMemoryRepository(),
			NATSClient: natsClient,
		}, nil
	}

	slog.Info(fmt.Sprintf("Connecting to PostgreSQL at %s", cfg.LoggableDatabaseURL()))
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Ping database to ensure connection works (fail-fast)
	ctxPing, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctxPing); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	slog.Info("PostgreSQL connection established successfully.")

	outboxRepo := postgres.NewOutboxRepository(db)

	var publisher *outbox.Publisher
	if natsClient != nil {
		publisher = outbox.NewPublisher(outboxRepo, natsClient, 2*time.Second)
	}

	return &Dependencies{
		TxManager:  postgres.NewTxManager(db),
		AgentRepo:  postgres.NewAgentRepository(db),
		RunRepo:    postgres.NewRunRepository(db),
		OutboxRepo: outboxRepo,
		NATSClient: natsClient,
		Publisher:  publisher,
		DB:         db,
	}, nil
}

// Run configures and starts the API servers, blocking until a termination signal is received
func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.PrintDiagnostics()

	// Initialize Observability (Logging & Tracing)
	observability.InitLogging("sentinelmesh-api", os.Stdout, slog.LevelInfo)
	obsCfg := observability.DefaultConfig("sentinelmesh-api")
	tp, err := observability.InitTracing(obsCfg, nil)
	if err != nil {
		slog.Warn("Failed to initialize distributed tracing", slog.Any("error", err))
	} else {
		defer observability.ShutdownTracing(context.Background())
		_ = tp
	}

	deps, err := InitializeDependencies(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}
	defer func() {
		if deps.DB != nil {
			deps.DB.Close()
		}
		if deps.NATSClient != nil {
			deps.NATSClient.Close()
		}
	}()

	if deps.Publisher != nil {
		deps.Publisher.Start()
		defer deps.Publisher.Stop()
	}

	agentSvc := application.NewAgentService(deps.TxManager, deps.AgentRepo, deps.OutboxRepo)
	runSvc := application.NewRunService(deps.TxManager, deps.AgentRepo, deps.RunRepo, deps.OutboxRepo)

	// Pre-seed default system AI agent workloads
	_, _ = agentSvc.CreateAgent(ctx, domain.Agent{
		ID:        "sentinel-router-agent-v1",
		Name:      "SentinelMesh Multi-Model Router Agent",
		Version:   "v2.0.0",
		TenantID:  "default",
		Image:     "sentinelmesh/router-worker:v2.0.0",
		Resources: types.AgentResources{CPU: "1000m", Memory: "2Gi", GPU: 0},
		Priority:  "high",
		State:     "ACTIVE",
	})
	_, _ = agentSvc.CreateAgent(ctx, domain.Agent{
		ID:        "sentinel-evaluator-agent-v1",
		Name:      "Stage 18/19 Bayesian Quality Evaluator",
		Version:   "v1.5.0",
		TenantID:  "default",
		Image:     "sentinelmesh/evaluator-worker:v1.5.0",
		Resources: types.AgentResources{CPU: "500m", Memory: "1Gi", GPU: 0},
		Priority:  "normal",
		State:     "ACTIVE",
	})
	_, _ = agentSvc.CreateAgent(ctx, domain.Agent{
		ID:        "sentinelctl-agent-v1",
		Name:      "SentinelMesh CLI System Daemon",
		Version:   "v1.0.0",
		TenantID:  "default",
		Image:     "sentinelmesh/daemon:v1.0.0",
		Resources: types.AgentResources{CPU: "250m", Memory: "512Mi", GPU: 0},
		Priority:  "critical",
		State:     "ACTIVE",
	})

	// -------------------------------------------------------------------------
	// Build AI pipeline services
	// -------------------------------------------------------------------------

	// The in-process outbox is used for all AI pipeline event routing and SSE.
	// Retrieve the typed MemoryRepository if we are in memory mode for SSE bridging.
	var memOutbox *outbox.MemoryRepository
	if m, ok := deps.OutboxRepo.(*outbox.MemoryRepository); ok {
		memOutbox = m
	}

	modelRegistry := router.NewDefaultModelRegistry()

	execMode := router.ProviderExecutionMode(cfg.ExecutionMode)
	if execMode == "" {
		execMode = router.ModeSynthetic
	}

	liveProvider := router.NewLiveModelProvider(modelRegistry, execMode, false)

	// Configure OpenAI endpoints if API key is present or mode is LIVE
	if cfg.OpenAIAPIKey != "" || execMode == router.ModeLive {
		smallEndpoint := router.ProviderEndpointConfig{
			Type:        router.ProviderTypeOpenAI,
			BaseURL:     cfg.OpenAIBaseURL,
			APIKey:      cfg.OpenAIAPIKey,
			ModelTarget: cfg.OpenAISmallModel,
			Timeout:     30 * time.Second,
		}
		mediumEndpoint := router.ProviderEndpointConfig{
			Type:        router.ProviderTypeOpenAI,
			BaseURL:     cfg.OpenAIBaseURL,
			APIKey:      cfg.OpenAIAPIKey,
			ModelTarget: cfg.OpenAIMediumModel,
			Timeout:     30 * time.Second,
		}
		largeEndpoint := router.ProviderEndpointConfig{
			Type:        router.ProviderTypeOpenAI,
			BaseURL:     cfg.OpenAIBaseURL,
			APIKey:      cfg.OpenAIAPIKey,
			ModelTarget: cfg.OpenAILargeModel,
			Timeout:     30 * time.Second,
		}

		liveProvider.SetEndpoint("small-fast-v1", smallEndpoint)
		liveProvider.SetEndpoint("medium-balanced-v1", mediumEndpoint)
		liveProvider.SetEndpoint("large-reasoning-v1", largeEndpoint)
		liveProvider.SetEndpoint("medium-openai", mediumEndpoint)
		liveProvider.SetEndpoint(cfg.OpenAIMediumModel, mediumEndpoint)

		// In LIVE mode, update registered models to reflect live OpenAI provider mappings
		if execMode == router.ModeLive {
			if m, err := modelRegistry.GetModel(ctx, "small-fast-v1"); err == nil {
				m.Provider = "openai"
				m.ProviderModelID = cfg.OpenAISmallModel
				m.Name = "OpenAI " + cfg.OpenAISmallModel + " (Small)"
				_ = modelRegistry.RegisterModel(ctx, m)
			}
			if m, err := modelRegistry.GetModel(ctx, "medium-balanced-v1"); err == nil {
				m.Provider = "openai"
				m.ProviderModelID = cfg.OpenAIMediumModel
				m.Name = "OpenAI " + cfg.OpenAIMediumModel + " (Medium)"
				_ = modelRegistry.RegisterModel(ctx, m)
			}
			if m, err := modelRegistry.GetModel(ctx, "large-reasoning-v1"); err == nil {
				m.Provider = "openai"
				m.ProviderModelID = cfg.OpenAILargeModel
				m.Name = "OpenAI " + cfg.OpenAILargeModel + " (Large)"
				_ = modelRegistry.RegisterModel(ctx, m)
			}
		}

		// Register explicit OpenAI model definition in registry
		_ = modelRegistry.RegisterModel(ctx, router.ModelDefinition{
			ID:                    "medium-openai",
			Name:                  "OpenAI " + cfg.OpenAIMediumModel,
			Tier:                  router.TierMedium,
			Provider:              "openai",
			ProviderModelID:      cfg.OpenAIMediumModel,
			CostPer1kInputTokens:  0.00015,
			CostPer1kOutputTokens: 0.00060,
			NominalP50LatencyMs:   200.0,
			NominalP95LatencyMs:   400.0,
			ContextWindow:         128000,
			SecurityClasses:       []string{"public", "standard", "restricted"},
			HealthStatus:          router.HealthHealthy,
			TaskQualityMatrix: map[router.TaskComplexity]float64{
				router.ComplexitySimple:         0.98,
				router.ComplexityModerate:       0.95,
				router.ComplexityComplex:        0.92,
				router.ComplexityReasoningHeavy: 0.88,
			},
		})
	}

	decisionRepo := router.NewMemoryDecisionRepository()

	routerSvc := router.NewService(
		modelRegistry,
		liveProvider,
		decisionRepo,
		deps.OutboxRepo,
		deps.TxManager,
	)

	learningStore := adaptive.NewMemoryLearningStore()
	driftDetector := adaptive.NewDualWindowDriftDetector()
	prior := adaptive.DefaultBetaPrior()

	adaptiveSvc := adaptive.NewAdaptiveService(
		modelRegistry,
		liveProvider,
		learningStore,
		driftDetector,
		prior,
		decisionRepo,
		deps.OutboxRepo,
		deps.TxManager,
	)

	policyMgr := onlinepolicy.NewPolicyManager(onlinepolicy.DefaultPolicyState())
	guardrails := onlinepolicy.NewGuardrailEnforcer(onlinepolicy.DefaultGuardrailConfig())

	policySvc := onlinepolicy.NewOnlinePolicyService(
		modelRegistry,
		liveProvider,
		learningStore,
		prior,
		policyMgr,
		guardrails,
		decisionRepo,
		deps.OutboxRepo,
		deps.TxManager,
	)

	// -------------------------------------------------------------------------
	// Build SSE Hub + Outbox Bridge
	// -------------------------------------------------------------------------
	sseHub := resthandler.NewSSEHub()

	// Start the SSE bridge if we have an in-process outbox.
	if memOutbox != nil {
		bridge := resthandler.NewOutboxSSEBridge(memOutbox, sseHub, 200*time.Millisecond)
		bridge.Start(ctx)
		defer bridge.Stop()
	}

	// -------------------------------------------------------------------------
	// REST Server — register all handlers
	// -------------------------------------------------------------------------
	mux := http.NewServeMux()

	// Health and Readiness
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if deps.DB != nil {
			ctxPing, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := deps.DB.PingContext(ctxPing); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("Database unavailable"))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Prometheus Metrics Endpoint
	mux.Handle("/metrics", observability.MetricsHandler())

	// Existing domain handlers
	agentHandler := resthandler.NewAgentHandler(agentSvc)
	agentHandler.RegisterRoutes(mux)

	runHandler := resthandler.NewRunHandler(runSvc)
	runHandler.RegisterRoutes(mux)

	// New GUI API handlers
	modelHandler := resthandler.NewModelHandler(modelRegistry, liveProvider)
	modelHandler.RegisterRoutes(mux)

	routerHandler := resthandler.NewRouterHandler(routerSvc, decisionRepo)
	routerHandler.RegisterRoutes(mux)

	adaptiveHandler := resthandler.NewAdaptiveHandler(adaptiveSvc)
	adaptiveHandler.RegisterRoutes(mux)

	policyHandler := resthandler.NewPolicyHandler(policySvc, adaptiveSvc, routerSvc, policyMgr)
	policyHandler.RegisterRoutes(mux)

	metricsHandler := resthandler.NewMetricsHandler(decisionRepo, modelRegistry)
	metricsHandler.RegisterRoutes(mux)

	eventsHandler := resthandler.NewEventsHandler(deps.OutboxRepo, sseHub)
	eventsHandler.RegisterRoutes(mux)

	settingsHandler := resthandler.NewSettingsHandler(cfg, modelRegistry, policyMgr, liveProvider)
	settingsHandler.RegisterRoutes(mux)

	providerHandler := resthandler.NewProviderHandler(cfg, liveProvider)
	providerHandler.RegisterRoutes(mux)

	handler := resthandler.Middleware(mux, resthandler.CORSMiddleware, resthandler.Recoverer, resthandler.TracingMiddleware, resthandler.Logger)

	restServer := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}

	// gRPC Server with OpenTelemetry Interceptors
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(observability.UnaryServerInterceptor()),
		grpc.StreamInterceptor(observability.StreamServerInterceptor()),
	)
	pb.RegisterAgentServiceServer(grpcServer, grpchandler.NewAgentServer(agentSvc))
	pb.RegisterRunServiceServer(grpcServer, grpchandler.NewRunServer(runSvc))
	reflection.Register(grpcServer)

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port: %w", err)
	}

	errCh := make(chan error, 2)

	go func() {
		log.Printf("REST server listening on %s", cfg.HTTPAddr)
		if err := restServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("rest server error: %w", err)
		}
	}()

	go func() {
		log.Printf("gRPC server listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			errCh <- fmt.Errorf("grpc server error: %w", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		log.Printf("received signal: %v, shutting down...", sig)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	log.Println("shutting down gRPC server...")
	grpcServer.GracefulStop()

	log.Println("shutting down REST server...")
	if err := restServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("REST server shutdown error: %v", err)
	}

	log.Println("SentinelMesh API Server stopped gracefully.")
	return nil
}
