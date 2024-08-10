package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/reflection"
	"net"
	"os"
	"strconv"
	"sync"
	"vistara-node/pkg/cluster"
	"vistara-node/pkg/containerd"
	"vistara-node/pkg/defaults"
)

func main() {
	logger := log.New()

	grpcPort, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}

	// 7946
	port, err := strconv.Atoi(os.Args[2])
	if err != nil {
		panic(err)
	}

	repo, err := containerd.NewMicroVMRepository(&containerd.Config{
		SocketPath:         defaults.ContainerdSocket,
		ContainerNamespace: defaults.ContainerdNamespace,
	})
	if err != nil {
		panic(err)
	}

	agent, err := cluster.NewAgent(port, repo, logger)
	if err != nil {
		panic(err)
	}

	if len(os.Args) > 3 {
		clusterPort, err := strconv.Atoi(os.Args[3])
		if err != nil {
			panic(err)
		}

		if err := agent.Join(clusterPort); err != nil {
			panic(err)
		}
	}

	grpcServer := cluster.NewServer(logger, agent)
	reflection.Register(grpcServer)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		panic(err)
	}

	quitWg := sync.WaitGroup{}
	quitWg.Add(2)

	go func() {
		defer quitWg.Done()
		if err := grpcServer.Serve(listener); err != nil {
			panic(err)
		}
	}()

	go func() {
		defer quitWg.Done()
		agent.Handler()
	}()

	quitWg.Wait()
}
