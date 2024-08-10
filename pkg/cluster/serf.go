package cluster

import (
	"context"
	"errors"
	"fmt"
	"github.com/containerd/containerd/cio"
	"github.com/google/uuid"
	"github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"vistara-node/pkg/containerd"
	pb "vistara-node/pkg/proto/cluster"
)

var (
	SpawnQuery   = "spawn_query"
	SpawnAttempt = "spawn_attempt"
)

type Agent struct {
	eventCh chan serf.Event
	ctrRepo *containerd.Repo
	cfg     *serf.Config
	serf    *serf.Serf
	logger  *log.Logger
}

func NewAgent(port int, repo *containerd.Repo, logger *log.Logger) (*Agent, error) {
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
		ctrRepo: repo,
	}

	return agent, nil
}

func (a *Agent) Handler() {
	for event := range a.eventCh {
		switch event.EventType() {
		case serf.EventMemberJoin:
			join := event.(serf.MemberEvent)
			a.logger.Infof("Join event: %v", join)
		case serf.EventQuery:
			query := event.(*serf.Query)
			a.logger.Infof("Query event: %v", query)

			switch query.Name {
			case SpawnQuery:
				payload, err := proto.Marshal(&pb.VmSpawnResponse{})
				if err != nil {
					a.logger.WithError(err).Error("failed to marshal payload")
					continue
				}

				if err := query.Respond(payload); err != nil {
					a.logger.WithError(err).Error("failed to respond to query")
				}
			case SpawnAttempt:
				// Attempt to spawn the VM
				var payload pb.VmSpawnRequest
				if err := proto.Unmarshal(query.Payload, &payload); err != nil {
					a.logger.WithError(err).Error("failed to unmarshal payload")
					continue
				}

				id, err := a.ctrRepo.CreateContainer(context.Background(), containerd.CreateContainerOpts{
					ImageRef:    payload.GetImageRef(),
					Snapshotter: "",
					Runtime: struct {
						Name    string
						Options interface{}
					}{
						Name: "io.containerd.runc.v2",
					},
					CioCreator: cio.NewCreator(cio.WithStdio),
				})
				if err != nil {
					a.logger.WithError(err).Error("failed to create container")
					continue
				}

				resp, err := proto.Marshal(&pb.VmSpawnResponse{Id: id})
				if err != nil {
					a.logger.WithError(err).Error("failed to marshal payload")
					continue
				}
				if err := query.Respond(resp); err != nil {
					a.logger.WithError(err).Error("failed to respond to query")
				}
			}
		default:
			a.logger.Infof("Received event: %v", event)
		}
	}
}

// Request another node to spawn a VM
func (a *Agent) SpawnRequest(req *pb.VmSpawnRequest) (*pb.VmSpawnResponse, error) {
	var anyPayload anypb.Any
	if err := anypb.MarshalFrom(&anyPayload, req, proto.MarshalOptions{}); err != nil {
		return nil, err
	}

	payload, err := proto.Marshal(&pb.ClusterMessage{
		Event:          pb.ClusterEvent_SPAWN,
		WrappedMessage: &anyPayload,
	})
	if err != nil {
		return nil, err
	}

	query, err := a.serf.Query(SpawnQuery, payload, a.serf.DefaultQueryParams())
	if err != nil {
		return nil, err
	}

	for response := range query.ResponseCh() {
		a.logger.Infof("Successful response from node: %s", response.From)

		params := a.serf.DefaultQueryParams()
		params.FilterNodes = []string{response.From}

		query, err = a.serf.Query(SpawnAttempt, payload, params)
		if err != nil {
			return nil, err
		}

		for response := range query.ResponseCh() {
			a.logger.Infof("Successfully spawned VM on node: %s", response.From)

			var resp pb.VmSpawnResponse
			if err := proto.Unmarshal(response.Payload, &resp); err != nil {
				return nil, err
			}
			return &resp, nil
		}
	}

	return nil, errors.New("no response received from nodes")
}

func (a *Agent) Join(port int) error {
	_, err := a.serf.Join([]string{fmt.Sprintf("%s:%d", "0.0.0.0", port)}, true)
	return err
}
