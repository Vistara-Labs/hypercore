package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"sync"
	"time"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/protobuf"
	"github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl/v2"
	"github.com/firecracker-microvm/firecracker-go-sdk/vsock"
	"github.com/google/uuid"
	"github.com/spf13/afero"
	"github.com/vistara-labs/firecracker-containerd/eventbridge"
	"github.com/vistara-labs/firecracker-containerd/proto"
	ioproxy "github.com/vistara-labs/firecracker-containerd/proto/service/ioproxy/ttrpc"
	"github.com/vistara-labs/firecracker-containerd/utils"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"vistara-node/pkg/hypervisor/firecracker"
	"vistara-node/pkg/models"
	"vistara-node/pkg/network"
	"vistara-node/pkg/ports"
)

const ShimID = "hypercore.example"
const VSockPort = 10789

type HypervisorState struct {
	networkSvc        ports.NetworkService
	fsSvc             afero.Fs
	vmSvc             ports.MicroVMService
	vm                *models.MicroVM
	eventBridgeClient eventbridge.Getter
	agentClient       taskAPI.TaskService
	ioProxyClient     ioproxy.IOProxyService
}

type HyperShim struct {
	id              string
	stateRoot       string
	shimCtx         context.Context
	remotePublisher shim.Publisher
	eventExchange   *exchange.Exchange
	taskManager     utils.TaskManager
	vmReady         chan struct{}
	vmState         *HypervisorState
	fifos           map[string]map[string]cio.Config
	fifosMutex      sync.Mutex
	portCountMutex  sync.Mutex
	portCount       uint32
}

func parseOpts(options *types.Any) (models.MicroVMSpec, error) {
	var metadata models.MicroVMSpec
	err := json.Unmarshal(options.Value, &metadata)

	return metadata, err
}

func generateExtraData(baseVSockPort uint32, jsonBytes []byte) *proto.ExtraData {
	log.G(context.Background()).Infof("Generating extra options with base VSock port %d", baseVSockPort)

	return &proto.ExtraData{
		JsonSpec:    jsonBytes,
		RuncOptions: nil,
		StdinPort:   baseVSockPort + 1,
		StdoutPort:  baseVSockPort + 2,
		StderrPort:  baseVSockPort + 3,
	}
}

func hypervisorStateForSpec(spec models.MicroVMSpec, stateRoot string) (*HypervisorState, error) {
	networkSvc := network.New(&network.Config{
		BridgeName: spec.HostNetDev,
	})
	fsSvc := afero.NewOsFs()

	switch spec.Provider {
	case "firecracker":
		vmSvc := firecracker.New(&firecracker.Config{
			FirecrackerBin: "/usr/bin/firecracker",
			StateRoot:      stateRoot,
		}, networkSvc, fsSvc)

		return &HypervisorState{
			networkSvc: networkSvc,
			fsSvc:      fsSvc,
			vmSvc:      vmSvc,
		}, nil
	case "cloudhypervisor":
		return &HypervisorState{
			networkSvc: networkSvc,
			fsSvc:      fsSvc,
			vmSvc:      nil,
		}, nil
	}

	return nil, fmt.Errorf("unrecognized provider: %s", spec.Provider)
}

func (s *HyperShim) getAndIncrementPortCount() uint32 {
	s.portCountMutex.Lock()
	defer s.portCountMutex.Unlock()

	portCount := s.portCount
	s.portCount += 3

	return VSockPort + portCount
}

func (s *HyperShim) addFIFOs(taskID string, execID string, config cio.Config) error {
	s.fifosMutex.Lock()
	defer s.fifosMutex.Unlock()

	_, exists := s.fifos[taskID]
	if !exists {
		s.fifos[taskID] = make(map[string]cio.Config)
	}

	value, exists := s.fifos[taskID][execID]
	if exists {
		return fmt.Errorf("failed to add FIFO files for task %q (exec=%q), got %+v", taskID, execID, value)
	}

	s.fifos[taskID][execID] = config
	return nil
}

func (s *HyperShim) State(ctx context.Context, req *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	resp, err := s.vmState.agentClient.State(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request to agent failed: %w", err)
	}

	s.fifosMutex.Lock()
	defer s.fifosMutex.Unlock()

	host := s.fifos[req.ID][req.ExecID]

	if resp.Status != task.Status_RUNNING {
		return resp, nil
	}

	resp.Stdin = host.Stdin
	resp.Stdout = host.Stdout
	resp.Stderr = host.Stderr

	state, err := s.vmState.ioProxyClient.State(ctx, &ioproxy.StateRequest{
		ID:     req.ID,
		ExecID: req.ExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check proxy status: %w", err)
	}

	if state.IsOpen {
		return resp, nil
	}

	extraData := generateExtraData(s.getAndIncrementPortCount(), nil)
	attach := ioproxy.AttachRequest{
		ID:         req.ID,
		ExecID:     req.ExecID,
		StdinPort:  extraData.StdinPort,
		StdoutPort: extraData.StdoutPort,
		StderrPort: extraData.StderrPort,
	}

	_, err = s.vmState.ioProxyClient.Attach(ctx, &attach)
	if err != nil {
		return nil, fmt.Errorf("failed to attach IO Proxy: %w", err)
	}

	ioConnectorSet, err := utils.NewIOProxy(log.G(ctx), host.Stdin, host.Stdout, host.Stderr, s.vmState.vmSvc.VSockPath(s.vmState.vm), extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to create IO Proxy: %w", err)
	}

	if err := s.taskManager.AttachIO(ctx, req.ID, req.ExecID, ioConnectorSet); err != nil {
		return nil, fmt.Errorf("failed to attach IO Proxy: %w", err)
	}

	return resp, nil
}

func (s *HyperShim) Create(ctx context.Context, req *taskAPI.CreateTaskRequest) (*taskAPI.CreateTaskResponse, error) {
	py, _ := json.Marshal(req)
	fd, _ := os.Create("/tmp/create.json")
	fd.Write(py)
	fd.Close()

	if s.vmState != nil {
		return nil, errors.New("Create called multiple times")
	}

	if len(req.Rootfs) != 1 {
		return nil, errors.New("got multiple entries in rootfs")
	}

	rootfs := req.Rootfs[0]
	if rootfs.Type != "ext4" {
		return nil, fmt.Errorf("got non-ext4 rootfs: %s", rootfs.Type)
	}

	vmid, err := models.NewVMID("hypercore", "", uuid.NewString())
	if err != nil {
		return nil, fmt.Errorf("failed to create new VMID: %w", err)
	}

	spec, err := parseOpts(req.Options)
	if err != nil {
		return nil, fmt.Errorf("failed to parse options: %w", err)
	}

	spec.ImagePath = rootfs.Source
	spec.GuestMAC = "06:00:AC:10:00:02"

	// TODO pass metadata via containerd and use the appropriate
	// VM provider
	hypervisorState, err := hypervisorStateForSpec(spec, s.stateRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create hypervisor state: %w", err)
	}

	hypervisorState.vm = &models.MicroVM{
		ID:      *vmid,
		Version: 2,
		Spec:    spec,
	}

	if err := hypervisorState.vmSvc.Start(ctx, hypervisorState.vm); err != nil {
		return nil, fmt.Errorf("failed to start VM: %w", err)
	}

	s.vmState = hypervisorState

	conn, err := vsock.DialContext(ctx, hypervisorState.vmSvc.VSockPath(s.vmState.vm), VSockPort, vsock.WithLogger(log.G(ctx)))
	if err != nil {
		return nil, err
	}

	rpcClient := ttrpc.NewClient(conn, ttrpc.WithOnClose(func() { _ = conn.Close() }))

	s.vmState.agentClient = taskAPI.NewTaskClient(rpcClient)
	s.vmState.ioProxyClient = ioproxy.NewIOProxyClient(rpcClient)
	s.vmState.eventBridgeClient = eventbridge.NewGetterClient(rpcClient)

	// The image will be exposed as an unmounted block device
	// in the guest, /dev/vdb (/dev/vda is the rootfs)
	req.Rootfs[0].Source = "/dev/vdb"

	ociConfig, err := os.ReadFile(req.Bundle + "/config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read config.json from %s: %w", req.Bundle, err)
	}

	extraData := generateExtraData(s.getAndIncrementPortCount(), ociConfig)

	req.Options, err = protobuf.MarshalAnyToProto(extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal options: %w", err)
	}

	ioConnectorSet, err := utils.NewIOProxy(log.G(ctx), req.Stdin, req.Stdout, req.Stderr, s.vmState.vmSvc.VSockPath(s.vmState.vm), extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to create IO Proxy: %w", err)
	}

	res, err := s.taskManager.CreateTask(ctx, req, s.vmState.agentClient, ioConnectorSet)

	if err != nil {
		return nil, err
	}

	close(s.vmReady)

	return res, nil
}

func (s *HyperShim) Start(ctx context.Context, req *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	return s.vmState.agentClient.Start(ctx, req)
}

func (s *HyperShim) Delete(ctx context.Context, req *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	return s.taskManager.DeleteProcess(ctx, req, s.vmState.agentClient)
}

func (s *HyperShim) Pids(ctx context.Context, req *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	return s.vmState.agentClient.Pids(ctx, req)
}

func (s *HyperShim) Pause(ctx context.Context, req *taskAPI.PauseRequest) (*emptypb.Empty, error) {
	return s.vmState.agentClient.Pause(ctx, req)
}

func (s *HyperShim) Resume(ctx context.Context, req *taskAPI.ResumeRequest) (*emptypb.Empty, error) {
	return s.vmState.agentClient.Resume(ctx, req)
}

func (s *HyperShim) Checkpoint(ctx context.Context, req *taskAPI.CheckpointTaskRequest) (*emptypb.Empty, error) {
	return s.vmState.agentClient.Checkpoint(ctx, req)
}

func (s *HyperShim) Kill(ctx context.Context, req *taskAPI.KillRequest) (*emptypb.Empty, error) {
	return s.vmState.agentClient.Kill(ctx, req)
}

func (s *HyperShim) Exec(ctx context.Context, req *taskAPI.ExecProcessRequest) (*emptypb.Empty, error) {
	extraData := generateExtraData(s.getAndIncrementPortCount(), nil)

	var err error
	req.Spec, err = protobuf.MarshalAnyToProto(extraData)

	if err != nil {
		return nil, fmt.Errorf("failed to create Any: %w", err)
	}

	ioConnectorSet, err := utils.NewIOProxy(log.G(ctx), req.Stdin, req.Stdout, req.Stderr,
		s.vmState.vmSvc.VSockPath(s.vmState.vm), extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to create IO Proxy: %w", err)
	}

	if err := s.addFIFOs(req.ID, req.ExecID, cio.Config{
		Terminal: req.Terminal,
		Stdin:    req.Stdin,
		Stdout:   req.Stdout,
		Stderr:   req.Stderr,
	}); err != nil {
		return nil, fmt.Errorf("failed to add FIFOs: %w", err)
	}

	return s.taskManager.ExecProcess(ctx, req, s.vmState.agentClient, ioConnectorSet)
}

func (s *HyperShim) ResizePty(ctx context.Context, req *taskAPI.ResizePtyRequest) (*emptypb.Empty, error) {
	return s.vmState.agentClient.ResizePty(ctx, req)
}

func (s *HyperShim) CloseIO(ctx context.Context, req *taskAPI.CloseIORequest) (*emptypb.Empty, error) {
	return s.vmState.agentClient.CloseIO(ctx, req)
}

func (s *HyperShim) Update(ctx context.Context, req *taskAPI.UpdateTaskRequest) (*emptypb.Empty, error) {
	return s.vmState.agentClient.Update(ctx, req)
}

func (s *HyperShim) Wait(ctx context.Context, req *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	return s.vmState.agentClient.Wait(ctx, req)
}

func (s *HyperShim) Stats(ctx context.Context, req *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	return s.vmState.agentClient.Stats(ctx, req)
}

func (s *HyperShim) Connect(ctx context.Context, req *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	return s.vmState.agentClient.Connect(ctx, req)
}

func (s *HyperShim) Shutdown(ctx context.Context, req *taskAPI.ShutdownRequest) (*emptypb.Empty, error) {
	// TODO ensure hypervisor process is cleaned up
	return s.vmState.agentClient.Shutdown(ctx, req)
}

func (s *HyperShim) Cleanup(ctx context.Context) (*taskAPI.DeleteResponse, error) {
	return &taskAPI.DeleteResponse{
		ExitedAt:   protobuf.ToTimestamp(time.Now()),
		ExitStatus: 128 + uint32(unix.SIGKILL),
	}, nil
}

func (s *HyperShim) StartShim(ctx context.Context, opts shim.StartOpts) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get self exe: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get cwd: %w", err)
	}

	cmd, err := shim.Command(ctx, &shim.CommandConfig{
		Runtime:      exe,
		Address:      opts.Address,
		TTRPCAddress: opts.TTRPCAddress,
		Path:         cwd,
		SchedCore:    false,
		Args:         []string{},
	})
	if err != nil {
		return "", fmt.Errorf("creating shim command: %w", err)
	}

	sockAddr, err := shim.SocketAddress(ctx, opts.Address, s.id)
	if err != nil {
		return "", fmt.Errorf("getting socket address: %w", err)
	}

	socket, err := shim.NewSocket(sockAddr)
	if err != nil {
		return "", fmt.Errorf("creating shim socket: %w", err)
	}

	if err := shim.WriteAddress("address", sockAddr); err != nil {
		return "", fmt.Errorf("writing socket address file: %w", err)
	}

	sockF, err := socket.File()
	if err != nil {
		sockF.Close()
		return "", fmt.Errorf("getting shim socket: %w", err)
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, sockF)

	if err = cmd.Start(); err != nil {
		return "", fmt.Errorf("starting shim command: %w", err)
	}

	if err = shim.AdjustOOMScore(cmd.Process.Pid); err != nil {
		return "", fmt.Errorf("adjusting shim process OOM score: %w", err)
	}

	return sockAddr, nil
}

func (s *HyperShim) startEventForwarders() {
	republishCh := eventbridge.Republish(s.shimCtx, s.eventExchange, s.remotePublisher)

	go func() {
		defer s.remotePublisher.Close()

		<-s.vmReady

		attachCh := eventbridge.Attach(s.shimCtx, s.vmState.eventBridgeClient, s.eventExchange)

		err := <-attachCh
		if err != nil {
			panic(err)
		}

		err = <-republishCh
		if err != nil {
			panic(err)
		}
	}()
}

func main() {
	typeurl.Register(&models.MicroVMSpec{})

	shim.Run(
		ShimID,
		func(ctx context.Context, id string, remotePublisher shim.Publisher, shimCancel func()) (shim.Shim, error) {
			hyperShim := &HyperShim{
				id:              id,
				stateRoot:       "/tmp",
				shimCtx:         ctx,
				remotePublisher: remotePublisher,
				eventExchange:   exchange.NewExchange(),
				taskManager:     utils.NewTaskManager(ctx, log.G(ctx)),
				vmReady:         make(chan struct{}),
				fifos:           make(map[string]map[string]cio.Config),
			}

			hyperShim.startEventForwarders()

			return hyperShim, nil
		},
	)
}
