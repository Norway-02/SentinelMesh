package grpc

import (
	"context"

	pb "github.com/sentinelmesh/sentinelmesh/api/v1"
	"github.com/sentinelmesh/sentinelmesh/internal/application"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type AgentServer struct {
	pb.UnimplementedAgentServiceServer
	svc *application.AgentService
}

func NewAgentServer(svc *application.AgentService) *AgentServer {
	return &AgentServer{
		svc: svc,
	}
}

func (s *AgentServer) CreateAgent(ctx context.Context, req *pb.CreateAgentRequest) (*pb.CreateAgentResponse, error) {
	if req.Name == "" {
		return nil, MapError(repository.ErrNotFound) // Simplified validation mapping, typically we'd use status.Error(codes.InvalidArgument...)
	}

	agent := AgentFromProto(req)
	created, err := s.svc.CreateAgent(ctx, agent)
	if err != nil {
		return nil, MapError(err)
	}

	return &pb.CreateAgentResponse{Agent: AgentToProto(created)}, nil
}

func (s *AgentServer) GetAgent(ctx context.Context, req *pb.GetAgentRequest) (*pb.GetAgentResponse, error) {
	agent, err := s.svc.GetAgent(ctx, req.Id)
	if err != nil {
		return nil, MapError(err)
	}

	return &pb.GetAgentResponse{Agent: AgentToProto(agent)}, nil
}

func (s *AgentServer) ListAgents(ctx context.Context, req *pb.ListAgentsRequest) (*pb.ListAgentsResponse, error) {
	agents, nextToken, err := s.svc.ListAgents(ctx, repository.AgentFilter{
		TenantID:  req.TenantId,
		PageSize:  int(req.PageSize),
		PageToken: req.PageToken,
	})
	if err != nil {
		return nil, MapError(err)
	}

	var pbAgents []*pb.Agent
	for _, a := range agents {
		pbAgents = append(pbAgents, AgentToProto(a))
	}

	return &pb.ListAgentsResponse{
		Agents:        pbAgents,
		NextPageToken: nextToken,
	}, nil
}

func (s *AgentServer) DeleteAgent(ctx context.Context, req *pb.DeleteAgentRequest) (*pb.DeleteAgentResponse, error) {
	if err := s.svc.DeleteAgent(ctx, req.Id); err != nil {
		return nil, MapError(err)
	}

	return &pb.DeleteAgentResponse{}, nil
}
