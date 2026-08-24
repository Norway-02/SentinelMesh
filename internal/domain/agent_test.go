package domain

import (
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestAgent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		agent   Agent
		wantErr bool
	}{
		{
			name: "valid agent",
			agent: Agent{
				ID:       "agent-1",
				Name:     "researcher",
				Version:  "v1",
				TenantID: "tenant-A",
				Image:    "researcher:latest",
				Resources: types.AgentResources{
					CPU:    "2",
					Memory: "4Gi",
					GPU:    0,
				},
				Priority: "normal",
				CheckpointPolicy: types.CheckpointPolicy{
					Enabled:  true,
					Interval: "30s",
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			agent: Agent{
				Name:     "researcher",
				Version:  "v1",
				Priority: "normal",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			agent: Agent{
				ID:       "agent-1",
				Version:  "v1",
				Priority: "normal",
			},
			wantErr: true,
		},
		{
			name: "invalid CPU (empty)",
			agent: Agent{
				ID:       "agent-1",
				Name:     "researcher",
				Version:  "v1",
				Priority: "normal",
				Resources: types.AgentResources{
					CPU: "",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid CPU (negative)",
			agent: Agent{
				ID:       "agent-1",
				Name:     "researcher",
				Version:  "v1",
				Priority: "normal",
				Resources: types.AgentResources{
					CPU: "-2",
				},
			},
			wantErr: true,
		},
		{
			name: "negative GPU",
			agent: Agent{
				ID:       "agent-1",
				Name:     "researcher",
				Version:  "v1",
				Priority: "normal",
				Resources: types.AgentResources{
					CPU:    "1",
					Memory: "1Gi",
					GPU:    -1,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid priority",
			agent: Agent{
				ID:       "agent-1",
				Name:     "researcher",
				Version:  "v1",
				Priority: "super-high",
				Resources: types.AgentResources{
					CPU:    "1",
					Memory: "1Gi",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid checkpoint interval when enabled",
			agent: Agent{
				ID:       "agent-1",
				Name:     "researcher",
				Version:  "v1",
				Priority: "normal",
				Resources: types.AgentResources{
					CPU:    "1",
					Memory: "1Gi",
				},
				CheckpointPolicy: types.CheckpointPolicy{
					Enabled:  true,
					Interval: "-10s",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.agent.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Agent.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
