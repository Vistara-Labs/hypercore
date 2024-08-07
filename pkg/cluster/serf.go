package cluster

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/serf/serf"
)

type Agent struct {
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
		eventCh: eventCh,
		cfg:     cfg,
		serf:    serf,
	}

	return agent, nil
}

func (a *Agent) Handler() {
	for event := range a.eventCh {
		fmt.Println("Received event", event)

		switch event.EventType() {
		case serf.EventQuery:
			query := event.(*serf.Query)
			fmt.Println(query)
		}
	}
}

func (a *Agent) Join(port int) error {
	_, err := a.serf.Join([]string{fmt.Sprintf("%s:%d", "0.0.0.0", port)}, true)
	return err
}
