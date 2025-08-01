package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	vcontainerd "vistara-node/pkg/containerd"
	pb "vistara-node/pkg/proto/cluster"

	ctask "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/cio"
	"github.com/google/uuid"
	"github.com/hashicorp/serf/serf"
	log "github.com/sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	QueryName           = "hypercore_query"
	SpawnRequestLabel   = "hypercore-request-payload"
	StateBroadcastEvent = "hypercore_state_broadcast"

	WorkloadBroadcastPeriod = time.Second * 30 // Increased from 5s to 30s
	MaxQueueDepth           = 3600 // Increased for production safety
)

type SavedStatusUpdate struct {
	update     *pb.NodeStateResponse
	receivedAt time.Time
}

type NodeWorkload struct {
	ID       string `json:"id"`
	ImageRef string `json:"imageRef"`
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
	lastStateSelf   *pb.NodeStateResponse
	lastStateUpdate map[string]SavedStatusUpdate
	tmpStateUpdates map[string]*pb.NodeStateResponse
	lastStateHash   string // Track state hash to detect changes
	stateMu         sync.Mutex
	
	// Prometheus metrics
	serfQueueDepth    prometheus.Gauge
	workloadCount     prometheus.Gauge
	broadcastSkipped  prometheus.Counter
	stateChanges      prometheus.Counter
}

// hashWorkloadState creates a consistent hash of the workload state
func (a *Agent) hashWorkloadState(state *pb.NodeStateResponse) string {
	// Create a deterministic string representation
	workloadIDs := make([]string, 0, len(state.GetWorkloads()))
	for _, workload := range state.GetWorkloads() {
		workloadIDs = append(workloadIDs, workload.GetId())
	}

	// Sort for consistency
	sort.Strings(workloadIDs)

	// Create hash
	hash := sha256.New()
	hash.Write([]byte(fmt.Sprintf("%d:%s", len(workloadIDs), strings.Join(workloadIDs, ","))))
	return hex.EncodeToString(hash.Sum(nil))
}

func NewAgent(logger *log.Logger, baseURL, bindAddr string, respawn bool, repo *vcontainerd.Repo, tlsConfig *TLSConfig) (*Agent, error) {
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
	cfg.UserEventSizeLimit = 2048 // Increased event size limit

	// Conservative Serf configuration to reduce queue buildup
	cfg.MemberlistConfig.GossipInterval = time.Second * 2 // Conservative gossip interval
	cfg.MemberlistConfig.ProbeInterval = time.Second * 5  // Conservative probe interval
	cfg.MemberlistConfig.SuspicionMult = 6                // Increased suspicion multiplier for stability
	cfg.MemberlistConfig.GossipNodes = 2                  // Reduce gossip nodes to decrease load

	cfg.Init()

	serf, err := serf.Create(cfg)
	if err != nil {
		return nil, err
	}

	// Initialize Prometheus metrics
	serfQueueDepth := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "hypercore_serf_queue_depth",
		Help: "Current Serf event queue depth",
	})
	workloadCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "hypercore_workload_count",
		Help: "Number of running workloads on this node",
	})
	broadcastSkipped := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hypercore_broadcast_skipped_total",
		Help: "Total number of broadcasts skipped due to queue depth",
	})
	stateChanges := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hypercore_state_changes_total",
		Help: "Total number of state changes detected",
	})

	// Register metrics
	prometheus.MustRegister(serfQueueDepth, workloadCount, broadcastSkipped, stateChanges)

	agent := &Agent{
		eventCh:         eventCh,
		cfg:             cfg,
		baseURL:         baseURL,
		serviceProxy:    serviceProxy,
		serf:            serf,
		logger:          logger,
		ctrRepo:         repo,
		lastStateUpdate: make(map[string]SavedStatusUpdate),
		tmpStateUpdates: make(map[string]*pb.NodeStateResponse),
		serfQueueDepth:  serfQueueDepth,
		workloadCount:   workloadCount,
		broadcastSkipped: broadcastSkipped,
		stateChanges:    stateChanges,
	}

	return agent, nil
}

func (a *Agent) handleSpawnRequest(payload *pb.VmSpawnRequest) (ret []byte, retErr error) {
	ctx := a.ctrRepo.GetContext(context.Background())

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

	if (vcpuUsed + int(payload.GetCores())) > max(runtime.NumCPU(), 225) {
		return nil, fmt.Errorf("cannot spawn container: have capacity for %d vCPUs, already in use: %d, requested: %d", runtime.NumCPU(), vcpuUsed, payload.GetCores())
	}

	availableMem, err := getAvailableMem()
	if err != nil {
		return nil, err
	}
	availableMem /= 1024

	if (memUsed + int(payload.GetMemory())) > int(availableMem) {
		a.logger.Warnf("have capacity for %d MB, already in use: %d MB, requested: %d MB", availableMem, memUsed, payload.GetMemory())
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
		CioCreator: func(id string) (cio.IO, error) {
			uri, err := cio.LogURIGenerator("file", "/tmp/hypercore/"+id, nil)
			if err != nil {
				return nil, err
			}

			return cio.LogURI(uri)("")
		},
		Labels: map[string]string{
			SpawnRequestLabel: string(encodedPayload),
		},
		Env: payload.GetEnv(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to spawn container: %w", err)
	}

	response, err := wrapClusterMessage(pb.ClusterEvent_SPAWN, &pb.VmSpawnResponse{Id: id, Url: id + "." + a.baseURL})
	if err != nil {
		return nil, fmt.Errorf("failed to wrap cluster message: %w", err)
	}

	return response, nil
}

func (a *Agent) handleStopRequest(payload *pb.VmStopRequest) (ret []byte, retErr error) {
	ctx := a.ctrRepo.GetContext(context.Background())

	defer func() {
		if retErr != nil {
			a.logger.WithError(retErr).Error("handleStopRequest failed")
			ret, retErr = wrapClusterErrorMessage(retErr.Error())
		}
	}()

	tasks, err := a.ctrRepo.GetTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing tasks to check capacity: %w", err)
	}

	for _, task := range tasks {
		if task.GetID() == payload.GetId() {
			_, err := a.ctrRepo.DeleteContainer(ctx, task.GetID())
			a.logger.WithError(err).Infof("Deleted task: %s", task.GetID())

			response, err := wrapClusterMessage(pb.ClusterEvent_STOP, &pb.Node{
				Id: a.serf.LocalMember().Name,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to wrap cluster message: %w", err)
			}

			return response, nil
		}
	}

	return nil, errors.New("workload not found")
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

			if false && query.SourceNode() == a.cfg.NodeName {
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
			case pb.ClusterEvent_STOP:
				var payload pb.VmStopRequest
				if err := baseMessage.GetWrappedMessage().UnmarshalTo(&payload); err != nil {
					a.logger.WithError(err).Error("failed to unmarshal payload")

					continue
				}

				response, err = a.handleStopRequest(&payload)
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

			a.logger.Infof("Got partial workloads of node %s IP %v %s", workloads.GetNode().GetId(), member.Addr, workloads.GetNode().GetIp())

			splitID := strings.Split(workloads.GetNode().GetIp(), "_")
			id, kind := splitID[0], splitID[1]

			switch kind {
			case "complete":
				a.tmpStateUpdates[id] = &workloads
			case "finish":
				partialWorkloads, ok := a.tmpStateUpdates[id]
				if !ok {
					a.logger.Infof("Unknown part for node %s", workloads.GetNode().GetId())

					continue
				}

				partialWorkloads.Workloads = append(partialWorkloads.Workloads, workloads.GetWorkloads()...)
			case "begin":
				a.tmpStateUpdates[id] = &workloads

				continue
			case "part":
				partialWorkloads, ok := a.tmpStateUpdates[id]
				if !ok {
					a.logger.Infof("Unknown part for node %s", workloads.GetNode().GetId())

					continue
				}

				partialWorkloads.Workloads = append(partialWorkloads.Workloads, workloads.GetWorkloads()...)

				continue
			}

			a.logger.Infof("Got workloads of node %s IP %v", workloads.GetNode().GetId(), member.Addr)
			a.lastStateMu.Lock()
			a.lastStateUpdate[member.Name] = SavedStatusUpdate{
				update:     a.tmpStateUpdates[id],
				receivedAt: time.Now(),
			}
			delete(a.tmpStateUpdates, id)
			a.lastStateMu.Unlock()

			for _, service := range workloads.GetWorkloads() {
				for port := range service.GetSourceRequest().GetPorts() {
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

func (a *Agent) StopRequest(req *pb.VmStopRequest) (*pb.Node, error) {
	payload, err := wrapClusterMessage(pb.ClusterEvent_STOP, req)
	if err != nil {
		return nil, err
	}

	params := a.serf.DefaultQueryParams()
	// Give 90 seconds to the node to stop the VM
	params.Timeout = time.Second * 90
	query, err := a.serf.Query(QueryName, payload, a.serf.DefaultQueryParams())
	if err != nil {
		return nil, err
	}

	for response := range query.ResponseCh() {
		a.logger.Infof("Successful response from node: %s", response.From)

		var resp pb.ClusterMessage
		if err := proto.Unmarshal(response.Payload, &resp); err != nil {
			return nil, err
		}

		// ignore failures
		if resp.GetEvent() == pb.ClusterEvent_ERROR {
			continue
		}

		var wrappedResp pb.Node
		if err := resp.GetWrappedMessage().UnmarshalTo(&wrappedResp); err != nil {
			return nil, err
		}

		return &wrappedResp, nil
	}

	return nil, errors.New("no success response received from nodes")
}

func (a *Agent) LogsRequest(id string) (*pb.VmLogsResponse, error) {
	var member string
	findWorkload := func(workloads []*pb.WorkloadState) bool {
		for _, workload := range workloads {
			if workload.GetId() == id {
				return true
			}
		}

		return false
	}

	a.lastStateMu.Lock()
	if findWorkload(a.lastStateSelf.GetWorkloads()) {
		member = a.lastStateSelf.GetNode().GetId()
	} else {
		for node, update := range a.lastStateUpdate {
			if findWorkload(update.update.GetWorkloads()) {
				member = node

				break
			}
		}
	}
	a.lastStateMu.Unlock()

	if member == "" {
		return nil, fmt.Errorf("workload %s does not exist", id)
	}

	serfMember := a.findMember(member)
	if serfMember == nil {
		return nil, fmt.Errorf("member %s does not exist", member)
	}

	a.logger.Infof("workload %s found on member %s", id, member)

	url := fmt.Sprintf("http://%s/logs?id=%s", net.JoinHostPort(serfMember.Addr.String(), "8001"), id)
	//nolint:noctx
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call node at address %s: %w", url, err)
	}
	defer resp.Body.Close()

	logs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes: %w", err)
	}

	return &pb.VmLogsResponse{Logs: string(logs)}, nil
}

// broadcast the workloads running on the node every 30 seconds
// so existing nodes can update their state and new nodes can
// sync up with the current state of the cluster
// also cleanup and re-spawn dead containers
//
//nolint:gocognit
func (a *Agent) monitorWorkloads() {
	ticker := time.NewTicker(WorkloadBroadcastPeriod)
	for range ticker.C {
		ctx := a.ctrRepo.GetContext(context.Background())

		tasks, err := a.ctrRepo.GetTasks(ctx)
		if err != nil {
			a.logger.WithError(err).Error("failed to get tasks")

			continue
		}

		resp := pb.NodeStateResponse{
			Node: &pb.Node{
				Id: a.serf.LocalMember().Name,
			},
		}

		for _, task := range tasks {
			a.logger.Infof("Got task %s, state: %s", task.GetID(), task.GetStatus())

			container, err := a.ctrRepo.GetContainer(ctx, task.GetID())
			if err != nil {
				a.logger.WithError(err).Errorf("failed to get container for task %s", task.GetID())

				continue
			}

			labels, err := container.Labels(ctx)
			if err != nil {
				a.logger.WithError(err).Errorf("failed to get labels for container %s: %s", task.GetID(), err)

				continue
			}

			var labelPayload pb.VmSpawnRequest
			if err := json.Unmarshal([]byte(labels[SpawnRequestLabel]), &labelPayload); err != nil {
				a.logger.Errorf("failed to unmarshal request from label: %s", err)

				continue
			}

			if task.GetStatus() == ctask.Status_STOPPED {
				a.logger.Infof("task %s is stopped, deleting container and respawning", task.GetID())

				if _, err := a.ctrRepo.DeleteContainer(ctx, task.GetID()); err != nil {
					a.logger.Errorf("failed to stop task %s: %s", task.GetID(), err)
				}

				go func() {
					if _, err := a.handleSpawnRequest(&labelPayload); err != nil {
						a.logger.Errorf("failed to respawn container %s: %s", task.GetID(), err)
					}
				}()

				continue
			}

			ip, err := a.ctrRepo.GetContainerPrimaryIP(ctx, container.ID())
			if err != nil {
				a.logger.Errorf("failed to get IP for container %s: %s", container.ID(), err)

				continue
			}

			for hostPort, containerPort := range labelPayload.GetPorts() {
				addr := fmt.Sprintf("%s:%d", ip, containerPort)
				if err := a.serviceProxy.Register(hostPort, container.ID(), addr); err != nil {
					a.logger.Errorf("failed to register container %s addr %s with proxy: %s", container.ID(), addr, err)
				}
			}

			resp.Workloads = append(resp.Workloads, &pb.WorkloadState{Id: container.ID(), SourceRequest: &labelPayload})
		}

		a.lastStateMu.Lock()
		a.lastStateSelf = &resp
		a.lastStateMu.Unlock()

		// Check if state has actually changed
		currentHash := a.hashWorkloadState(&resp)
		a.stateMu.Lock()
		stateChanged := currentHash != a.lastStateHash
		if stateChanged {
			a.lastStateHash = currentHash
		}
		a.stateMu.Unlock()

		// Only broadcast if state changed
		if !stateChanged {
			a.logger.Debug("State unchanged, skipping broadcast")
			continue
		}

		// Check queue depth before broadcasting
		stats := a.serf.Stats()
		if queueDepthStr, ok := stats["event_queue_depth"]; ok {
			if queueDepth, err := strconv.Atoi(queueDepthStr); err == nil {
				// Update Prometheus metric
				a.serfQueueDepth.Set(float64(queueDepth))

				// Log queue depth for monitoring
				if queueDepth > 1000 {
					a.logger.Infof("Queue depth: %d (monitoring)", queueDepth)
				}

				if queueDepth > MaxQueueDepth {
					a.logger.Warnf("Queue depth %d exceeds limit %d, skipping broadcast to prevent message drops", queueDepth, MaxQueueDepth)
					a.broadcastSkipped.Inc()
					continue
				}
			}
		}

		// Update workload count metric
		a.workloadCount.Set(float64(len(resp.GetWorkloads())))

		// Increment state changes counter
		if stateChanged {
			a.stateChanges.Inc()
		}

		// batch size of 10
		parts := int(math.Ceil(float64(len(resp.GetWorkloads())) / 10))

		id := uuid.NewString()

		for part := range parts {
			partResp := pb.NodeStateResponse{
				Node:      resp.GetNode(),
				Workloads: resp.GetWorkloads()[(part * 10):min((part+1)*10, len(resp.GetWorkloads()))],
			}

			if parts == 1 {
				partResp.Node.Ip = id + "_complete"
			} else if (part + 1) == parts {
				partResp.Node.Ip = id + "_finish"
			} else if part == 0 {
				partResp.Node.Ip = id + "_begin"
			} else {
				partResp.Node.Ip = id + "_part"
			}

			marshaled, err := proto.Marshal(&partResp)
			if err != nil {
				a.logger.WithError(err).Error("failed to marshal")

				continue
			}

			if err := a.serf.UserEvent(StateBroadcastEvent, marshaled, true); err != nil {
				a.logger.WithError(err).Error("failed to broadcast workload state")
			}
		}
	}
}

func (a *Agent) monitorStateUpdates(respawn bool) {
	ticker := time.NewTicker(WorkloadBroadcastPeriod)
	for range ticker.C {
		a.lastStateMu.Lock()
		toDelete := make([]string, 0)
		for node, update := range a.lastStateUpdate {
			if time.Since(update.receivedAt) > (WorkloadBroadcastPeriod * 3) {
				toDelete = append(toDelete, node)
				if !respawn {
					continue
				}

				a.logger.Warnf("Update from node %s last received at %v, re-scheduling workloads", node, update.receivedAt)
				for _, service := range update.update.GetWorkloads() {
					go func() {
						if resp, err := a.SpawnRequest(service.GetSourceRequest()); err != nil {
							a.logger.WithError(err).Errorf("failed to respawn service %s", service.GetId())
						} else {
							a.logger.Infof("successfully respawned service %s: %+v", service.GetId(), resp)
						}
					}()
				}
			}
		}
		for _, node := range toDelete {
			delete(a.lastStateUpdate, node)
		}
		a.lastStateMu.Unlock()
	}
}

func (a *Agent) nodeStates() *pb.NodesStateResponse {
	a.lastStateMu.Lock()
	defer a.lastStateMu.Unlock()

	workloadStates := []*pb.NodeStateResponse{a.lastStateSelf}

	for _, update := range a.lastStateUpdate {
		workloadStates = append(workloadStates, update.update)
	}

	return &pb.NodesStateResponse{States: workloadStates}
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
