package cluster

import (
	"context"
	pb "vistara-node/pkg/proto/cluster"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedClusterServiceServer
	logger *log.Logger
	agent  *Agent
}

func (s *server) Spawn(_ context.Context, req *pb.VmSpawnRequest) (*pb.VmSpawnResponse, error) {
	s.logger.Infof("Received spawn request: %v", req)

	return s.agent.SpawnRequest(req)
}

func (s *server) Stop(_ context.Context, req *pb.VmStopRequest) (*pb.Node, error) {
	s.logger.Infof("Received stop request: %v", req)

	return s.agent.StopRequest(req)
}

func (s *server) List(_ context.Context, _ *pb.VmQueryRequest) (*pb.NodesStateResponse, error) {
	s.logger.Info("Received list request")

	return s.agent.nodeStates(), nil
}

func NewServer(logger *log.Logger, agent *Agent) *grpc.Server {
	grpcServer := grpc.NewServer()
	pb.RegisterClusterServiceServer(grpcServer, &server{
		logger: logger,
		agent:  agent,
	})

	return grpcServer
}
