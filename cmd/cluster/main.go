package main

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/serf/serf"
	"os"
	"strconv"
)

type Agent struct {
	closeCh chan struct{}
	eventCh chan serf.Event
	cfg     *serf.Config
	serf    *serf.Serf
}

func NewAgent(port int) (*Agent, error) {
	eventCh := make(chan serf.Event, 64)

	cfg := serf.DefaultConfig()
	cfg.EventCh = eventCh
	cfg.NodeName = uuid.NewString()
	cfg.MemberlistConfig.BindPort = port
	cfg.MemberlistConfig.AdvertisePort = port
	cfg.Init()

	serf, err := serf.Create(cfg)
	if err != nil {
		return nil, err
	}

	agent := &Agent{
		closeCh: make(chan struct{}),
		eventCh: eventCh,
		cfg:     cfg,
		serf:    serf,
	}

	return agent, nil
}

func (a *Agent) EventsCh() <-chan serf.Event {
	return a.eventCh
}

func (a *Agent) Join(port int) error {
	_, err := a.serf.Join([]string{fmt.Sprintf("%s:%d", "0.0.0.0", port)}, true)
	return err
}

func main() {
	// 7946
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}

	agent, err := NewAgent(port)
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

		for event := range agent.EventsCh() {
			fmt.Println("Received event", event)
		}
	} else {
		for event := range agent.EventsCh() {
			fmt.Println("Received event", event)
			query, err := agent.serf.Query("test", []byte{}, agent.serf.DefaultQueryParams())
			if err != nil {
				panic(err)
			}

			for resp := range query.ResponseCh() {
				fmt.Println("Response", resp)
			}
		}
	}
}
