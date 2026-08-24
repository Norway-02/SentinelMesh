package grpc

import (
	"context"

	pb "github.com/sentinelmesh/sentinelmesh/api/v1"
	"github.com/sentinelmesh/sentinelmesh/internal/application"
)

type RunServer struct {
	pb.UnimplementedRunServiceServer
	svc *application.RunService
}

func NewRunServer(svc *application.RunService) *RunServer {
	return &RunServer{
		svc: svc,
	}
}

func (s *RunServer) CreateRun(ctx context.Context, req *pb.CreateRunRequest) (*pb.CreateRunResponse, error) {
	run, err := s.svc.CreateRun(ctx, req.AgentId)
	if err != nil {
		return nil, MapError(err)
	}

	return &pb.CreateRunResponse{Run: RunToProto(run)}, nil
}

func (s *RunServer) GetRun(ctx context.Context, req *pb.GetRunRequest) (*pb.GetRunResponse, error) {
	run, err := s.svc.GetRun(ctx, req.Id)
	if err != nil {
		return nil, MapError(err)
	}

	return &pb.GetRunResponse{Run: RunToProto(run)}, nil
}

func (s *RunServer) CancelRun(ctx context.Context, req *pb.CancelRunRequest) (*pb.CancelRunResponse, error) {
	run, err := s.svc.CancelRun(ctx, req.Id)
	if err != nil {
		return nil, MapError(err)
	}

	return &pb.CancelRunResponse{Run: RunToProto(run)}, nil
}

func (s *RunServer) GetRunState(ctx context.Context, req *pb.GetRunStateRequest) (*pb.GetRunStateResponse, error) {
	state, err := s.svc.GetRunState(ctx, req.Id)
	if err != nil {
		return nil, MapError(err)
	}

	return &pb.GetRunStateResponse{State: string(state)}, nil
}
