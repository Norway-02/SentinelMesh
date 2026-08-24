package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	runID := os.Getenv("SENTINEL_RUN_ID")
	agentID := os.Getenv("SENTINEL_AGENT_ID")
	tenantID := os.Getenv("SENTINEL_TENANT_ID")

	fmt.Printf("[Agent] TaskProcessor initialized (PID: %d, RunID: %s, Agent: %s, Tenant: %s)\n",
		os.Getpid(), runID, agentID, tenantID)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Check if configured to simulate failure
	if os.Getenv("SIMULATE_FAILURE") == "true" {
		fmt.Fprintf(os.Stderr, "[Agent ERROR] Simulated task processor failure triggered!\n")
		os.Exit(2)
	}

	steps := 3
	if s := os.Getenv("WORKLOAD_STEPS"); s != "" {
		if val, err := strconv.Atoi(s); err == nil && val > 0 {
			steps = val
		}
	}

	fmt.Println("[Agent] Beginning distributed data processing task...")
	for i := 1; i <= steps; i++ {
		select {
		case sig := <-sigCh:
			fmt.Printf("[Agent] Received signal %v, performing graceful shutdown...\n", sig)
			time.Sleep(100 * time.Millisecond)
			fmt.Println("[Agent] Clean shutdown complete.")
			os.Exit(0)
		default:
			fmt.Printf("[Agent] Step %d/%d: Processing batch slice #%d at %s\n",
				i, steps, i*1024, time.Now().Format(time.RFC3339))
			time.Sleep(200 * time.Millisecond)
		}
	}

	fmt.Println("[Agent SUCCESS] Task processing completed successfully. Emitting results.")
	os.Exit(0)
}
