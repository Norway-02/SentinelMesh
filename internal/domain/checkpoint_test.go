package domain

import (
	"testing"
	"time"
)

func TestCheckpoint_Validate(t *testing.T) {
	tests := []struct {
		name       string
		checkpoint Checkpoint
		wantErr    bool
	}{
		{
			name: "valid checkpoint",
			checkpoint: Checkpoint{
				ID:          "cp-1",
				RunID:       "run-1",
				Version:     1,
				ArtifactURI: "s3://bucket/cp-1",
				Checksum:    "sha256:abcd",
				CreatedAt:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			checkpoint: Checkpoint{
				RunID:       "run-1",
				Version:     1,
				ArtifactURI: "s3://bucket/cp-1",
				Checksum:    "sha256:abcd",
			},
			wantErr: true,
		},
		{
			name: "missing RunID",
			checkpoint: Checkpoint{
				ID:          "cp-1",
				Version:     1,
				ArtifactURI: "s3://bucket/cp-1",
				Checksum:    "sha256:abcd",
			},
			wantErr: true,
		},
		{
			name: "invalid version",
			checkpoint: Checkpoint{
				ID:          "cp-1",
				RunID:       "run-1",
				Version:     0,
				ArtifactURI: "s3://bucket/cp-1",
				Checksum:    "sha256:abcd",
			},
			wantErr: true,
		},
		{
			name: "empty artifact URI",
			checkpoint: Checkpoint{
				ID:       "cp-1",
				RunID:    "run-1",
				Version:  1,
				Checksum: "sha256:abcd",
			},
			wantErr: true,
		},
		{
			name: "empty checksum",
			checkpoint: Checkpoint{
				ID:          "cp-1",
				RunID:       "run-1",
				Version:     1,
				ArtifactURI: "s3://bucket/cp-1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.checkpoint.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Checkpoint.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
