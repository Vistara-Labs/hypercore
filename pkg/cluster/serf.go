package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"sync"
	"time"
	vcontainerd "vistara-node/pkg/containerd"
	pb "vistara-node/pkg/proto/cluster"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/google/uuid"
	"github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	QueryName           = "hypercore_query"
	SpawnRequestLabel   = "hypercore-request-payload"
	StateBroadcastEvent = "hypercore_state_broadcast"

	WORKLOAD_BROADCAST_PERIOD = time.Second * 5
)

type SavedStatusUpdate struct {
	update     *pb.NodeStateResponse
	receivedAt time.Time
}

type Agent struct {
	eventCh         chan serf.Event
	serviceProxy    *ServiceProxy
	ctrRepo         *vcontainerd.Repo
	cfg             *serf.Config
	serf            *serf.Serf
	baseURL         string
	logger          *log.Logger
	lastStateMu     sync.Mutex
	lastStateUpdate map[string]SavedStatusUpdate
}

func NewAgent(logger *log.Logger, baseURL, bindAddr string, repo *vcontainerd.Repo, tlsConfig *TLSConfig) (*Agent, error) {
	eventCh := make(chan serf.Event, 64)

	serviceProxy, err := NewServiceProxy(logger, tlsConfig)
	if err != nil {
		return nil, err
	}

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
		eventCh:         eventCh,
		cfg:             cfg,
		baseURL:         baseURL,
		serviceProxy:    serviceProxy,
		serf:            serf,
		logger:          logger,
		ctrRepo:         repo,
		lastStateUpdate: make(map[string]SavedStatusUpdate),
	}
	go agent.broadcastWorkloads()
	go agent.monitorWorkloads()
	go agent.monitorStateUpdates()

	return agent, nil
}

func (a *Agent) handleNodeStateRequest() (ret []byte, retErr error) {
	defer func() {
		if retErr != nil {
			a.logger.WithError(retErr).Error("handleNodeStateRequest failed")
			ret, retErr = wrapClusterErrorMessage(retErr.Error())
		}
	}()

	ctx := context.Background()

	tasks, err := a.ctrRepo.GetTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing tasks to check capacity: %w", err)
	}

	var resp pb.NodeStateResponse

	for _, task := range tasks {
		id := task.GetID()
		_, err := a.ctrRepo.GetContainer(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get container %s: %w", id, err)
		}
		resp.Workloads = append(resp.Workloads, &pb.WorkloadState{
			Id: id,
		})
	}

	response, err := wrapClusterMessage(pb.ClusterEvent_NODE_STATE, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap cluster message: %w", err)
	}

	return response, nil
}

func (a *Agent) handleSpawnRequest(payload *pb.VmSpawnRequest) (ret []byte, retErr error) {
	ctx := context.Background()

	for _, port := range payload.GetPorts() {
		if port > 0xffff {
			return nil, fmt.Errorf("got invalid port %d greater than %d", port, 0xffff)
		}
	}

	if payload.GetDryRun() {
		response, err := wrapClusterMessage(pb.ClusterEvent_SPAWN, &pb.VmSpawnResponse{})
		if err != nil {
			return nil, fmt.Errorf("failed to wrap cluster message: %w", err)
		}

		return response, nil
	}

	defer func() {
		if retErr != nil {
			a.logger.WithError(retErr).Error("handleSpawnRequest failed")
			ret, retErr = wrapClusterErrorMessage(retErr.Error())
		}
	}()

	tasks, err := a.ctrRepo.GetTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing tasks to check capacity: %w", err)
	}

	vcpuUsed := 0
	memUsed := 0
	for _, task := range tasks {
		container, err := a.ctrRepo.GetContainer(ctx, task.GetID())
		if err != nil {
			return nil, fmt.Errorf("failed to get container %s: %w", task.GetID(), err)
		}

		labels, err := container.Labels(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get labels for container %s: %w", task.GetID(), err)
		}

		var labelPayload pb.VmSpawnRequest
		if err := json.Unmarshal([]byte(labels[SpawnRequestLabel]), &labelPayload); err != nil {
			return nil, err
		}

		vcpuUsed += int(labelPayload.GetCores())
		memUsed += int(labelPayload.GetMemory())
	}

	if (vcpuUsed + int(payload.GetCores())) > runtime.NumCPU() {
		return nil, fmt.Errorf("cannot spawn container: have capacity for %d vCPUs, already in use: %d, requested: %d", runtime.NumCPU(), vcpuUsed, payload.GetCores())
	}

	availableMem, err := getAvailableMem()
	if err != nil {
		return nil, err
	}
	availableMem /= 1024

	if (memUsed + int(payload.GetMemory())) > int(availableMem) {
		return nil, fmt.Errorf("cannot spawn container: have capacity for %d MB, already in use: %d MB, requested: %d MB", availableMem, memUsed, payload.GetMemory())
	}

	// We store the request payload as part of the container labels
	// so we can later check whether we have spare vCPUs left
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	id, err := a.ctrRepo.CreateContainer(ctx, vcontainerd.CreateContainerOpts{
		ImageRef:    payload.GetImageRef(),
		Snapshotter: "",
		Runtime: struct {
			Name    string
			Options interface{}
		}{
			Name: "io.containerd.runc.v2",
		},
		Limits: &struct {
			CPUFraction float64
			MemoryBytes uint64
		}{
			CPUFraction: float64(payload.GetCores()) / float64(runtime.NumCPU()),
			MemoryBytes: uint64(payload.GetMemory()) * 1024 * 1024,
		},
		CioCreator: cio.NewCreator(cio.WithStdio),
		Labels: map[string]string{
			SpawnRequestLabel: string(encodedPayload),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to spawn container: %w", err)
	}

	ip, err := a.ctrRepo.GetContainerPrimaryIP(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP for container %s", id)
	}

	for hostPort, containerPort := range payload.GetPorts() {
		addr := fmt.Sprintf("%s:%d", ip, containerPort)
		if err := a.serviceProxy.Register(hostPort, id, addr); err != nil {
			return nil, fmt.Errorf("failed to register container %s addr %s with proxy: %w", id, addr, err)
		}
	}

	response, err := wrapClusterMessage(pb.ClusterEvent_SPAWN, &pb.VmSpawnResponse{Id: id, Url: id + "." + a.baseURL})
	if err != nil {
		return nil, fmt.Errorf("failed to wrap cluster message: %w", err)
	}

	return response, nil
}

//nolint:gocognit
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
			case pb.ClusterEvent_NODE_STATE:
				var _ pb.NodeStateRequest
				response, err = a.handleNodeStateRequest()
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
		case serf.EventUser:
			userEvent := event.(serf.UserEvent)

			var workloads pb.NodeStateResponse

			if err := proto.Unmarshal(userEvent.Payload, &workloads); err != nil {
				a.logger.WithError(err).Error("failed to unmarshal")

				continue
			}

			if workloads.GetNode().GetId() == a.serf.LocalMember().Name {
				continue
			}

			member := a.findMember(workloads.GetNode().GetId())
			if member == nil {
				a.logger.Warnf("member for node %s not found", workloads.GetNode().GetId())

				continue
			}

			a.logger.Infof("Got workloads of node %s IP %v", workloads.GetNode().GetId(), member.Addr)
			a.lastStateMu.Lock()
			a.lastStateUpdate[member.Name] = SavedStatusUpdate{
				update:     &workloads,
				receivedAt: time.Now(),
			}
			a.lastStateMu.Unlock()

			for _, service := range workloads.GetWorkloads() {
				for _, port := range service.GetPorts() {
					addr := fmt.Sprintf("%s:%d", member.Addr.String(), port)
					if err := a.serviceProxy.Register(port, service.GetId(), addr); err != nil {
						a.logger.WithError(err).Errorf("failed to register node %s service %s addr %s with proxy", member.Name, service, addr)

						continue
					}
				}
			}
		case serf.EventMemberLeave:
			fallthrough
		case serf.EventMemberFailed:
			fallthrough
		case serf.EventMemberUpdate:
			fallthrough
		case serf.EventMemberReap:
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
		// Give 90 seconds to the node to pull the image from the network
		// and spawn the VM
		params.Timeout = time.Second * 90
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

			if resp.GetEvent() == pb.ClusterEvent_ERROR {
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

// Request state of all nodes including the VMs running on them
func (a *Agent) NodeStateRequest() (*pb.NodesStateResponse, error) {
	payload, err := wrapClusterMessage(pb.ClusterEvent_NODE_STATE, &pb.NodeStateRequest{})
	if err != nil {
		return nil, err
	}

	query, err := a.serf.Query(QueryName, payload, a.serf.DefaultQueryParams())
	if err != nil {
		return nil, err
	}

	stateResp := pb.NodesStateResponse{}

	for response := range query.ResponseCh() {
		var resp pb.ClusterMessage
		if err := proto.Unmarshal(response.Payload, &resp); err != nil {
			return nil, err
		}

		if resp.GetEvent() == pb.ClusterEvent_ERROR {
			var errorResp pb.ErrorResponse
			if err := resp.GetWrappedMessage().UnmarshalTo(&errorResp); err != nil {
				return nil, err
			}

			a.logger.Warnf("node returned failure response: %s", errorResp.GetError())

			continue
		}

		var wrappedResp pb.NodeStateResponse
		if err := resp.GetWrappedMessage().UnmarshalTo(&wrappedResp); err != nil {
			return nil, err
		}

		member := a.findMember(response.From)
		if member == nil {
			a.logger.Warnf("Got response from %s but did not find it in member list", response.From)

			continue
		}

		wrappedResp.Node = &pb.Node{
			Id: member.Name,
			Ip: member.Addr.String(),
		}

		stateResp.Responses = append(stateResp.Responses, &wrappedResp)
	}

	return &stateResp, nil
}

// broadcast the workloads running on the node every 30 seconds
// so existing nodes can update their state and new nodes can
// sync up with the current state of the cluster
func (a *Agent) broadcastWorkloads() {
	ticker := time.NewTicker(WORKLOAD_BROADCAST_PERIOD)
	for range ticker.C {
		resp := pb.NodeStateResponse{
			Node: &pb.Node{
				Id: a.serf.LocalMember().Name,
			},
		}
		for id, ports := range a.serviceProxy.Services() {
			resp.Workloads = append(resp.Workloads, &pb.WorkloadState{
				Id:    id,
				Ports: ports,
			})
		}

		marshaled, err := proto.Marshal(&resp)
		if err != nil {
			a.logger.WithError(err).Error("failed to marshal")

			continue
		}

		if err := a.serf.UserEvent(StateBroadcastEvent, marshaled, true); err != nil {
			a.logger.WithError(err).Error("failed to broadcast workload state")
		}
	}
}

func (a *Agent) monitorWorkloads() {
	ticker := time.NewTicker(time.Second * 10)

	for range ticker.C {
		tasks, err := a.ctrRepo.GetTasks(context.Background())
		if err != nil {
			a.logger.WithError(err).Error("failed to get tasks")
			continue
		}

		for _, task := range tasks {
			a.logger.Info("Got task %s, state: %s", task.GetID(), task.GetStatus())

			container, err := a.ctrRepo.GetContainer(context.Background(), task.GetID())
			if err != nil {
				a.logger.WithError(err).Error("failed to get container for task %s", task.GetID())
				continue
			}

			task, err := container.Task(context.Background(), nil)
			if err != nil {
				a.logger.WithError(err).Error("failed to get existing task for container %s: %w", task.ID, err)
				continue
			}

			status, err := task.Status(context.Background())
			if err != nil {
				a.logger.WithError(err).Error("failed to get status for task %s: %w", task.ID, err)
				continue
			}

			if status.Status == containerd.Stopped {
				if err := task.Start(context.Background()); err != nil {
					a.logger.WithError(err).Error("failed to restart task %s: %w", task.ID, err)
				} else {
					a.logger.Info("successfully restarted task %s", task.ID)
				}
			}
		}
	}
}

func (a *Agent) monitorStateUpdates() {
	ticker := time.NewTicker(WORKLOAD_BROADCAST_PERIOD)
	for range ticker.C {
		a.lastStateMu.Lock()
		for node, update := range a.lastStateUpdate {
			if time.Since(update.receivedAt) > (WORKLOAD_BROADCAST_PERIOD * 3) {
				a.logger.Warnf("Update from node %s last received at %v, re-scheduling workloads", node, update.receivedAt)
				for _, service := range update.update.GetWorkloads() {
				}
			}
		}
		a.lastStateMu.Unlock()
	}
}

func (a *Agent) findMember(name string) *serf.Member {
	for _, member := range a.serf.Members() {
		if member.Name == name {
			return &member
		}
	}

	return nil
}

func (a *Agent) Join(addr string) error {
	_, err := a.serf.Join([]string{addr}, true)

	return err
}
