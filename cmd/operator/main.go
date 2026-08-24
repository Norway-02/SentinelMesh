package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/sentinelmesh/sentinelmesh/internal/messaging"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/operator"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	sentinelmeshv1alpha1 "github.com/sentinelmesh/sentinelmesh/internal/kubernetes/api/v1alpha1"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(sentinelmeshv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Initialize Observability (Logging & Tracing)
	observability.InitLogging("sentinelmesh-operator", os.Stdout, slog.LevelInfo)
	obsCfg := observability.DefaultConfig("sentinelmesh-operator")
	tp, err := observability.InitTracing(obsCfg, nil)
	if err != nil {
		slog.Warn("Failed to initialize distributed tracing", slog.Any("error", err))
	} else {
		defer observability.ShutdownTracing(context.Background())
		_ = tp
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start SentinelMesh Prometheus metrics server on port 9092
	observability.StartMetricsServer(ctx, 9092)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "sentinelmesh-operator.sentinelmesh.io",
	})
	if err != nil {
		slog.Error("unable to start manager", "error", err)
		os.Exit(1)
	}

	// Connect to NATS JetStream
	natsURL := os.Getenv("SENTINEL_NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	natsClient, err := messaging.ConnectNATS(natsURL)
	if err != nil {
		slog.Error("unable to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()
	
	slog.Info("Connected to NATS JetStream successfully")

	// Start NATS Event Consumer
	eventConsumer := operator.NewEventConsumer(natsClient, mgr.GetClient())
	if err := eventConsumer.Start(context.Background()); err != nil {
		slog.Error("unable to start event consumer", "error", err)
		os.Exit(1)
	}

	// Register Reconciler
	reconciler := &operator.AgentRunReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err = reconciler.SetupWithManager(mgr); err != nil {
		slog.Error("unable to create controller", "controller", "AgentRun", "error", err)
		os.Exit(1)
	}

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		slog.Error("unable to set up health check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		slog.Error("unable to set up ready check", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting SentinelMesh Kubernetes Operator (Stage 8)")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		slog.Error("problem running manager", "error", err)
		os.Exit(1)
	}
}
