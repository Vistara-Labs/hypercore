package cluster

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

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

func (s *server) Logs(_ context.Context, req *pb.VmLogsRequest) (*pb.VmLogsResponse, error) {
	s.logger.Infof("Received logs request: %v", req)

	return s.agent.LogsRequest(req.GetId())
}

func writeResponse(w http.ResponseWriter, response interface{}, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); err != nil {
			panic(err)
		}

		return
	}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{"response": response}); err != nil {
		panic(err)
	}
}

func NewServer(logger *log.Logger, agent *Agent) (*http.ServeMux, *grpc.Server) {
	grpcServer := grpc.NewServer()
	server := &server{
		logger: logger,
		agent:  agent,
	}

	pb.RegisterClusterServiceServer(grpcServer, server)

	mux := http.NewServeMux()
	mux.HandleFunc("/spawn", func(w http.ResponseWriter, r *http.Request) {
		var request pb.VmSpawnRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeResponse(w, nil, err)

			return
		}
		response, err := server.Spawn(context.Background(), &request)
		writeResponse(w, response, err)
	})
	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		response, err := server.Stop(context.Background(), &pb.VmStopRequest{Id: id})
		writeResponse(w, response, err)
	})
	mux.HandleFunc("/list", func(w http.ResponseWriter, _ *http.Request) {
		response, err := server.List(context.Background(), nil)
		writeResponse(w, response, err)
	})
	mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		// TODO sanitize id
		logs, err := os.ReadFile("/tmp/hypercore/" + id)
		writeResponse(w, string(logs), err)
	})

	return mux, grpcServer
}
