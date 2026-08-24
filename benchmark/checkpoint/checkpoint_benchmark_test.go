package checkpoint_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
)

func generateStatePayload(sizeBytes int) []byte {
	b := make([]byte, sizeBytes)
	for i := range b {
		b[i] = byte((i % 26) + 'a')
	}
	return b
}

func BenchmarkCheckpoint_SHA256Only(b *testing.B) {
	sizes := []int{1024, 10240, 102400, 1048576, 10485760} // 1KB, 10KB, 100KB, 1MB, 10MB

	for _, size := range sizes {
		payload := generateStatePayload(size)
		b.Run(fmt.Sprintf("Size_%dB", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h := sha256.Sum256(payload)
				_ = hex.EncodeToString(h[:])
			}
		})
	}
}

func BenchmarkCheckpoint_SaveAndVerify(b *testing.B) {
	ctx := context.Background()
	sizes := []int{1024, 10240, 102400, 1048576, 10485760} // 1KB, 10KB, 100KB, 1MB, 10MB

	for _, size := range sizes {
		payload := generateStatePayload(size)
		b.Run(fmt.Sprintf("Size_%dB", size), func(b *testing.B) {
			repo := checkpoint.NewMemoryRepository()
			outboxRepo := outbox.NewMemoryRepository()
			txManager := memory.NewTxManager()
			svc := checkpoint.NewService(repo, outboxRepo, txManager)

			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req := checkpoint.SaveCheckpointRequest{
					RunID:          fmt.Sprintf("run-cp-%d", i),
					AgentID:        "agent-1",
					TenantID:       "tenant-1",
					SequenceNumber: int64(i + 1),
					StateInline:    payload,
				}
				cp, err := svc.SaveCheckpoint(ctx, req)
				if err != nil {
					b.Fatalf("SaveCheckpoint failed: %v", err)
				}
				if !cp.VerifyIntegrity() {
					b.Fatal("VerifyIntegrity failed")
				}
			}
		})
	}
}

func BenchmarkCheckpoint_RestoreRead(b *testing.B) {
	ctx := context.Background()
	sizes := []int{1024, 10240, 102400, 1048576, 10485760}

	for _, size := range sizes {
		payload := generateStatePayload(size)
		repo := checkpoint.NewMemoryRepository()
		outboxRepo := outbox.NewMemoryRepository()
		txManager := memory.NewTxManager()
		svc := checkpoint.NewService(repo, outboxRepo, txManager)

		runID := fmt.Sprintf("run-restore-%d", size)
		_, _ = svc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
			RunID:          runID,
			AgentID:        "agent-1",
			TenantID:       "tenant-1",
			SequenceNumber: 1,
			StateInline:    payload,
		})

		b.Run(fmt.Sprintf("Size_%dB", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cp, err := svc.GetLatestCheckpoint(ctx, runID)
				if err != nil {
					b.Fatalf("GetLatestCheckpoint failed: %v", err)
				}
				if !cp.VerifyIntegrity() {
					b.Fatal("VerifyIntegrity failed")
				}
			}
		})
	}
}
