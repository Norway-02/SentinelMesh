package grpc

import (
	pb "github.com/sentinelmesh/sentinelmesh/api/v1"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func AgentToProto(agent domain.Agent) *pb.Agent {
	return &pb.Agent{
		Id:      agent.ID,
		Name:    agent.Name,
		Tenant:  agent.TenantID,
		Version: agent.Version,
		Image:   agent.Image,
		Resources: &pb.Resources{
			Cpu:    agent.Resources.CPU,
			Memory: agent.Resources.Memory,
			Gpu:    int32(agent.Resources.GPU),
		},
		Priority: agent.Priority,
		SecurityPolicy: &pb.SecurityPolicy{
			Profile: agent.SecurityPolicy.Profile,
		},
		NetworkPolicy: &pb.NetworkPolicy{
			Mode: agent.NetworkPolicy.Mode,
		},
		CheckpointPolicy: &pb.CheckpointPolicy{
			Enabled:  agent.CheckpointPolicy.Enabled,
			Interval: agent.CheckpointPolicy.Interval,
		},
		VerificationPolicy: &pb.VerificationPolicy{
			Enabled: agent.VerificationPolicy.Enabled,
		},
		Status:    agent.State,
		CreatedAt: timestamppb.New(agent.CreatedAt),
		UpdatedAt: timestamppb.New(agent.UpdatedAt),
	}
}

func AgentFromProto(req *pb.CreateAgentRequest) domain.Agent {
	var res types.AgentResources
	if req.Resources != nil {
		res = types.AgentResources{
			CPU:    req.Resources.Cpu,
			Memory: req.Resources.Memory,
			GPU:    int(req.Resources.Gpu),
		}
	}
	return domain.Agent{
		Name:      req.Name,
		TenantID:  req.TenantId,
		Version:   req.Version,
		Image:     req.Image,
		Resources: res,
		Priority:  req.Priority,
	}
}

func RunToProto(run domain.AgentRun) *pb.AgentRun {
	pbRun := &pb.AgentRun{
		Id:                run.ID,
		AgentId:           run.AgentID,
		State:             string(run.State),
		Node:              run.Node,
		Cluster:           run.Cluster,
		LastCheckpoint:    run.LastCheckpointID,
		RetryCount:        int32(run.RetryCount),
		ResourceUsage:     "",  // Default empty as it's not tracked in domain yet
		CostEstimate:      0.0, // Default 0
		VerificationState: run.VerificationState,
		FailureReason:     run.FailureReason,
	}
	if run.StartedAt != nil {
		pbRun.StartedAt = timestamppb.New(*run.StartedAt)
	}
	if run.FinishedAt != nil {
		pbRun.FinishedAt = timestamppb.New(*run.FinishedAt)
	}
	return pbRun
}
