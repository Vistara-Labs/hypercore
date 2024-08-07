package cluster

import (
	"context"
	"google.golang.org/grpc"
	pb "vistara-node/pkg/proto/cluster"
)

type server struct {
	pb.UnimplementedClusterServiceServer
}

func (s *server) Spawn(ctx context.Context, req *pb.VmSpawnRequest) (*pb.VmSpawnResponse, error) {
	return nil, nil
}

func NewServer() *grpc.Server {
	grpcServer := grpc.NewServer()
	pb.RegisterClusterServiceServer(grpcServer, &server{})

	return grpcServer
}
