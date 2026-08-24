package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
)

func runCheckpointBenchmarks() []BenchmarkResult {
	fmt.Printf("\n[3/6] RUNNING CHECKPOINT MULTI-TIER STORAGE BENCHMARKS (1 KB to 100 MB)...\n")
	var results []BenchmarkResult
	ctx := context.Background()

	sizes := []struct {
		name  string
		bytes int
	}{
		{"1KB", 1024},
		{"10KB", 10240},
		{"100KB", 102400},
		{"1MB", 1048576},
		{"10MB", 10485760},
		{"50MB", 52428800},
		{"100MB", 104857600},
	}

	for _, s := range sizes {
		payload := make([]byte, s.bytes)
		for i := range payload {
			payload[i] = byte((i % 26) + 'a')
		}

		// 1. SHA-256 CPU Only
		iters := 20
		if s.bytes > 10485760 {
			iters = 5
		}
		hashDurations := make([]time.Duration, iters)
		for i := 0; i < iters; i++ {
			t0 := time.Now()
			h := sha256.Sum256(payload)
			_ = hex.EncodeToString(h[:])
			hashDurations[i] = time.Since(t0)
		}
		p50, p95, p99, mean := calculatePercentiles(hashDurations)
		mbPerSec := (float64(s.bytes) / (1024 * 1024)) / (float64(mean) / float64(time.Second))
		results = append(results, BenchmarkResult{
			Suite:        "Checkpoint",
			Scenario:     "SHA256-Hash-Only",
			Scale:        s.name,
			Iterations:   iters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   mbPerSec,
		})
		fmt.Printf("  • SHA-256 Only (%-5s): P50=%v | Bandwidth=%.2f MB/s\n", s.name, p50, mbPerSec)

		// 2. Inline/URI Checkpoint Save & Verify
		repo := checkpoint.NewMemoryRepository()
		outboxRepo := outbox.NewMemoryRepository()
		txManager := memory.NewTxManager()
		svc := checkpoint.NewService(repo, outboxRepo, txManager)

		saveDurations := make([]time.Duration, iters)
		for i := 0; i < iters; i++ {
			req := checkpoint.SaveCheckpointRequest{
				RunID:          fmt.Sprintf("run-cp-%s-%d", s.name, i),
				AgentID:        "agent-1",
				TenantID:       "tenant-1",
				SequenceNumber: int64(i + 1),
				StateInline:    payload,
			}
			t0 := time.Now()
			cp, err := svc.SaveCheckpoint(ctx, req)
			saveDurations[i] = time.Since(t0)
			if err != nil || !cp.VerifyIntegrity() {
				fmt.Printf("Save failed for %s: %v\n", s.name, err)
			}
		}
		p50, p95, p99, mean = calculatePercentiles(saveDurations)
		mbPerSec = (float64(s.bytes) / (1024 * 1024)) / (float64(mean) / float64(time.Second))
		results = append(results, BenchmarkResult{
			Suite:        "Checkpoint",
			Scenario:     "Save-and-Verify",
			Scale:        s.name,
			Iterations:   iters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   mbPerSec,
		})
		fmt.Printf("  • Save & Verify (%-5s): P50=%v | Bandwidth=%.2f MB/s\n", s.name, p50, mbPerSec)
	}

	return results
}
