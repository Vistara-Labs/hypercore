package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pbnjay/memory"
	"net"
	"runtime"
	"strconv"
	"time"
	"vistara-node/pkg/containerd"
	pb "vistara-node/pkg/proto/cluster"

	"github.com/containerd/containerd/cio"
	"github.com/google/uuid"
	"github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	QueryName         = "hypercore_query"
	SpawnRequestLabel = "hypercore-request-payload"
)

type Agent struct {
	eventCh chan serf.Event
	ctrRepo *containerd.Repo
	cfg     *serf.Config
	serf    *serf.Serf
	logger  *log.Logger
}

func NewAgent(bindAddr string, repo *containerd.Repo, logger *log.Logger) (*Agent, error) {
	eventCh := make(chan serf.Event, 64)

	addr, port, err := net.SplitHostPort(bindAddr)
	if err != nil {
		return nil, err
	}

	bindPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	cfg := serf.DefaultConfig()
	cfg.EventCh = eventCh
	cfg.NodeName = uuid.NewString()
	cfg.MemberlistConfig.BindAddr = addr
	cfg.MemberlistConfig.BindPort = bindPort
	cfg.MemberlistConfig.AdvertisePort = bindPort
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

func (a *Agent) handleSpawnRequest(payload *pb.VmSpawnRequest) ([]byte, error) {
	ctx := context.Background()

	if payload.GetDryRun() {
		response, err := wrapClusterMessage(pb.ClusterEvent_SPAWN, &pb.VmSpawnResponse{})
		if err != nil {
			return nil, fmt.Errorf("failed to wrap cluster message: %w", err)
		}

		return response, nil
	}

	tasks, err := a.ctrRepo.GetTasks(ctx)
	if err != nil {
		return nil, err
	}

	vcpuUsed := 0
	memUsed := 0
	for _, task := range tasks {
		container, err := a.ctrRepo.GetContainer(ctx, task.GetID())
		if err != nil {
			return nil, fmt.Errorf("failed to get container %s: %w", task.GetID(), err)
		}

		var labelPayload pb.VmSpawnRequest
		if err := json.Unmarshal([]byte(container.Labels[SpawnRequestLabel]), &labelPayload); err != nil {
			return nil, err
		}

		vcpuUsed += int(labelPayload.GetCores())
		memUsed += int(labelPayload.GetMemory())
	}

	if (vcpuUsed + int(payload.GetCores())) > runtime.NumCPU() {
		err := fmt.Errorf("have capacity for %d vCPUs, already in use: %d, requested: %d", runtime.NumCPU(), vcpuUsed, payload.GetCores())

		a.logger.WithError(err).Error("cannot spawn container")

		response, err := wrapClusterErrorMessage(err.Error())
		if err != nil {
			return nil, fmt.Errorf("failed to wrap cluster error message: %w", err)
		}

		return response, nil
	}

	if (memUsed + int(payload.GetMemory())) > memory.FreeMemory() {
		err := fmt.Errorf("have capacity for %d memory, already in use: %d, requested: %d", memory.FreeMemory(), memUsed, payload.GetMemory())

		a.logger.WithError(err).Error("cannot spawn container")

		response, err := wrapClusterErrorMessage(err.Error())
		if err != nil {
			return nil, fmt.Errorf("failed to wrap cluster error message: %w", err)
		}

		return response, nil
	}

	// We store the request payload as part of the container labels
	// so we can later check whether we have spare vCPUs left
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	id, err := a.ctrRepo.CreateContainer(ctx, containerd.CreateContainerOpts{
		ImageRef:    payload.GetImageRef(),
		Snapshotter: "",
		Runtime: struct {
			Name    string
			Options interface{}
		}{
			Name: "io.containerd.runc.v2",
		},
		CioCreator: cio.NewCreator(cio.WithStdio),
		Labels: map[string]string{
			SpawnRequestLabel: string(encodedPayload),
		},
	})
	if err != nil {
		a.logger.WithError(err).Error("failed to spawn container")

		response, err := wrapClusterErrorMessage(err.Error())
		if err != nil {
			return nil, fmt.Errorf("failed to wrap cluster error message: %w", err)
		}

		return response, nil
	}

	response, err := wrapClusterMessage(pb.ClusterEvent_SPAWN, &pb.VmSpawnResponse{Id: id})
	if err != nil {
		return nil, fmt.Errorf("failed to wrap cluster message: %w", err)
	}

	return response, nil
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

			if query.SourceNode() == a.cfg.NodeName {
				a.logger.Warn("Received event from self node, ignoring")

				continue
			}

			var baseMessage pb.ClusterMessage
			if err := proto.Unmarshal(query.Payload, &baseMessage); err != nil {
				a.logger.WithError(err).Error("failed to unmarshal base payload")

				continue
			}

			var response []byte
			var err error

			switch baseMessage.GetEvent() {
			case pb.ClusterEvent_SPAWN:
				var payload pb.VmSpawnRequest
				if err := baseMessage.GetWrappedMessage().UnmarshalTo(&payload); err != nil {
					a.logger.WithError(err).Error("failed to unmarshal payload")

					continue
				}

				response, err = a.handleSpawnRequest(&payload)
			case pb.ClusterEvent_ERROR:
				fallthrough
			default:
				a.logger.Errorf("got invalid event: %d", baseMessage.GetEvent())

				continue
			}

			if err != nil {
				a.logger.WithError(err).Errorf("failed to handle event: %d", baseMessage.GetEvent())

				continue
			}

			if err := query.Respond(response); err != nil {
				a.logger.WithError(err).Error("failed to respond to query")
			}
		case serf.EventMemberLeave:
			fallthrough
		case serf.EventMemberFailed:
			fallthrough
		case serf.EventMemberUpdate:
			fallthrough
		case serf.EventMemberReap:
			fallthrough
		case serf.EventUser:
			fallthrough
		default:
			a.logger.Infof("Received event: %v", event)
		}
	}
}

func wrapClusterMessage(event pb.ClusterEvent, message proto.Message) ([]byte, error) {
	var anyPayload anypb.Any
	if err := anypb.MarshalFrom(&anyPayload, message, proto.MarshalOptions{}); err != nil {
		return nil, err
	}

	payload, err := proto.Marshal(&pb.ClusterMessage{
		Event:          event,
		WrappedMessage: &anyPayload,
	})
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func wrapClusterErrorMessage(errorMessage string) ([]byte, error) {
	return wrapClusterMessage(pb.ClusterEvent_ERROR, &pb.ErrorResponse{
		Error: errorMessage,
	})
}

// Request another node to spawn a VM
func (a *Agent) SpawnRequest(req *pb.VmSpawnRequest) (*pb.VmSpawnResponse, error) {
	req.DryRun = true
	payload, err := wrapClusterMessage(pb.ClusterEvent_SPAWN, req)
	if err != nil {
		return nil, err
	}

	query, err := a.serf.Query(QueryName, payload, a.serf.DefaultQueryParams())
	if err != nil {
		return nil, err
	}

	req.DryRun = false
	payload, err = wrapClusterMessage(pb.ClusterEvent_SPAWN, req)
	if err != nil {
		return nil, err
	}

	for response := range query.ResponseCh() {
		a.logger.Infof("Successful response from node: %s", response.From)

		params := a.serf.DefaultQueryParams()
		// Give 30 seconds to the node to pull the image from the network
		// and spawn the VM
		params.Timeout = time.Second * 30
		// Only send the query to the node that sent the response
		params.FilterNodes = []string{response.From}

		query, err = a.serf.Query(QueryName, payload, params)
		if err != nil {
			return nil, err
		}

		for response := range query.ResponseCh() {
			a.logger.Infof("Successfully spawned VM on node: %s", response.From)

			var resp pb.ClusterMessage
			if err := proto.Unmarshal(response.Payload, &resp); err != nil {
				return nil, err
			}

			if resp.Event == pb.ClusterEvent_ERROR {
				var errorResp pb.ErrorResponse
				if err := resp.GetWrappedMessage().UnmarshalTo(&errorResp); err != nil {
					return nil, err
				}

				return nil, fmt.Errorf("node returned failure response: %s", errorResp.GetError())
			}

			var wrappedResp pb.VmSpawnResponse
			if err := resp.GetWrappedMessage().UnmarshalTo(&wrappedResp); err != nil {
				return nil, err
			}

			return &wrappedResp, nil
		}
	}

	return nil, errors.New("no response received from nodes")
}

func (a *Agent) Join(addr string) error {
	_, err := a.serf.Join([]string{addr}, true)

	return err
}
