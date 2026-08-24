package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/runtime"
)

func main() {
	var runID string
	var agentID string
	var command string
	var argsStr string
	var timeoutSec int
	var followLogs bool

	flag.StringVar(&runID, "run-id", "", "SentinelMesh run identifier")
	flag.StringVar(&agentID, "agent-id", "agent-local", "SentinelMesh agent identifier")
	flag.StringVar(&command, "cmd", "", "Command to execute")
	flag.StringVar(&argsStr, "args", "", "Comma-separated arguments for command")
	flag.IntVar(&timeoutSec, "timeout", 60, "Execution timeout in seconds")
	flag.BoolVar(&followLogs, "follow", true, "Follow and stream stdout/stderr logs")
	flag.Parse()

	if runID == "" || command == "" {
		fmt.Println("SentinelMesh Runtime Daemon / CLI (Stage 9)")
		fmt.Println("Usage: runtime -run-id=<id> -cmd=<command> [-args=<arg1,arg2>] [-timeout=<seconds>]")
		return
	}

	// Initialize Observability
	observability.InitLogging("sentinelmesh-runtime", os.Stdout, slog.LevelInfo)
	obsCfg := observability.DefaultConfig("sentinelmesh-runtime")
	_, _ = observability.InitTracing(obsCfg, nil)
	defer observability.ShutdownTracing(context.Background())

	var args []string
	if argsStr != "" {
		args = strings.Split(argsStr, ",")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = observability.WithRunID(ctx, runID)
	ctx = observability.WithAgentID(ctx, agentID)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	rt := runtime.NewProcessRuntime()
	supervisor := runtime.NewSupervisor(rt, 250*time.Millisecond)

	supervisor.OnStateChange(func(st runtime.ExecutionStatus) {
		slog.InfoContext(ctx, "State transition detected",
			slog.String("run_id", st.RunID),
			slog.String("state", string(st.State)),
			slog.Int("exit_code", st.ExitCode),
		)
	})

	supervisor.Start(ctx)
	defer supervisor.Stop()

	req := runtime.ExecutionRequest{
		RunID:    runID,
		AgentID:  agentID,
		TenantID: "local-tenant",
		Command:  command,
		Args:     args,
		Timeout:  time.Duration(timeoutSec) * time.Second,
	}

	slog.InfoContext(ctx, "Starting agent workload via Runtime", "run_id", runID, "command", command, "args", args)
	handle, err := rt.Start(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to start agent workload", "error", err)
		os.Exit(1)
	}

	supervisor.Register(req)
	slog.InfoContext(ctx, "Workload running", "run_id", handle.RunID, "pid", handle.ProcessID)

	if followLogs {
		logStream, err := rt.Logs(ctx, runID, runtime.LogOptions{Follow: true})
		if err == nil {
			go func() {
				_, _ = io.Copy(os.Stdout, logStream)
			}()
		}
	}

	// Wait for process completion or signal
	done := make(chan struct{})
	go func() {
		for {
			st, err := rt.Status(ctx, runID)
			if err == nil && st.IsFinished() {
				slog.InfoContext(ctx, "Workload completed",
					slog.String("state", string(st.State)),
					slog.Int("exit_code", st.ExitCode),
					slog.String("error", st.ErrorReason),
				)
				close(done)
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	select {
	case <-done:
	case sig := <-sigCh:
		slog.WarnContext(ctx, "Received termination signal, stopping workload", "signal", sig)
		_ = rt.Stop(ctx, runID, 2*time.Second)
		<-done
	}
}
