package cluster

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	pb "vistara-node/pkg/proto/cluster"
)

var (
	SpawnQuery   = "spawn_query"
	SpawnAttempt = "spawn_attempt"
)

type Agent struct {
	eventCh chan serf.Event
	cfg     *serf.Config
	serf    *serf.Serf
	logger  *log.Logger
}

func NewAgent(port int, logger *log.Logger) (*Agent, error) {
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
		logger:  logger,
	}

	return agent, nil
}

func (a *Agent) Handler() {
	for event := range a.eventCh {
		a.logger.Infof("Received event: %v", event)

		switch event.EventType() {
		case serf.EventMemberJoin:
			join := event.(*serf.MemberEvent)
			a.logger.Infof("Join event: %v", join)
		case serf.EventQuery:
			query := event.(*serf.Query)
			a.logger.Infof("Query event: %v", query)

			switch query.Name {
			case SpawnQuery:
				// TODO parse query
				payload, err := proto.Marshal(&pb.VmSpawnResponse{})
				if err != nil {
					a.logger.WithError(err).Error("failed to marshal payload")
				}

				if err := query.Respond(payload); err != nil {
					a.logger.WithError(err).Error("failed to respond to query")
				}
			case SpawnAttempt:
			}
		}
	}
}

// Request another node to spawn a VM
func (a *Agent) SpawnRequest(req *pb.VmSpawnRequest) error {
	var anyPayload anypb.Any
	if err := anypb.MarshalFrom(&anyPayload, req, proto.MarshalOptions{}); err != nil {
		return err
	}

	payload, err := proto.Marshal(&pb.ClusterMessage{
		Event:          pb.ClusterEvent_SPAWN,
		WrappedMessage: &anyPayload,
	})
	if err != nil {
		return err
	}

	query, err := a.serf.Query(SpawnQuery, payload, a.serf.DefaultQueryParams())
	if err != nil {
		return err
	}

	for response := range query.ResponseCh() {
		var payload pb.ClusterMessage
		if err := proto.Unmarshal(response.Payload, &payload); err != nil {
			a.logger.WithError(err).Error("failed to unmarshal payload")
			continue
		}

		a.logger.Infof("Successful response from node: %s", response.From)
	}

	return errors.New("no response received from nodes")
}

func (a *Agent) Join(port int) error {
	_, err := a.serf.Join([]string{fmt.Sprintf("%s:%d", "0.0.0.0", port)}, true)
	return err
}
