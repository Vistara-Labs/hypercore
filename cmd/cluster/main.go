package cluster

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"vistara-node/pkg/cluster"
)

func main() {
	// 7946
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}

	agent, err := cluster.NewAgent(port)
	if err != nil {
		panic(err)
	}

	if len(os.Args) > 2 {
		clusterPort, err := strconv.Atoi(os.Args[2])
		if err != nil {
			panic(err)
		}

		if err := agent.Join(clusterPort); err != nil {
			panic(err)
		}
	}

	grpcServer := cluster.NewServer()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", 6666))
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
