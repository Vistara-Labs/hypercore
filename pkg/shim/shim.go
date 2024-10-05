package shim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/protobuf"
	"github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/log"
	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl/v2"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/firecracker-microvm/firecracker-go-sdk/vsock"
	"github.com/google/uuid"
	"github.com/spf13/afero"
	"github.com/vistara-labs/firecracker-containerd/proto"
	ioproxy "github.com/vistara-labs/firecracker-containerd/proto/service/ioproxy/ttrpc"
	"github.com/vistara-labs/firecracker-containerd/utils"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/cloudhypervisor"
	"vistara-node/pkg/hypervisor/firecracker"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"
)

const ShimID = "hypercore.example"
const VSockPort = 10789

type HypervisorState struct {
	fsSvc         afero.Fs
	vmSvc         ports.MicroVMService
	vm            *models.MicroVM
	agentClient   taskAPI.TaskService
	ioProxyClient ioproxy.IOProxyService
	vmStopped     chan struct{}
}

type HyperShim struct {
	id        string
	stateRoot string
	//nolint:containedctx
	shimCtx         context.Context
	remotePublisher shim.Publisher
	eventExchange   *exchange.Exchange
	taskManager     utils.TaskManager
	vmState         *HypervisorState
	fifos           map[string]map[string]cio.Config
	fifosMutex      sync.Mutex
	portCountMutex  sync.Mutex
	portCount       uint32
	shimCancel      func()
}

func parseOpts(options *types.Any) (models.MicroVMSpec, error) {
	var metadata models.MicroVMSpec
	err := json.Unmarshal(options.GetValue(), &metadata)

	return metadata, err
}

func generateExtraData(baseVSockPort uint32, jsonBytes []byte, options *types.Any) *proto.ExtraData {
	var opts *types.Any
	if options != nil {
		valCopy := make([]byte, len(options.GetValue()))
		copy(valCopy, options.GetValue())
		opts = &types.Any{
			TypeUrl: options.GetTypeUrl(),
			Value:   valCopy,
		}
	}

	return &proto.ExtraData{
		JsonSpec:    jsonBytes,
		RuncOptions: opts,
		StdinPort:   baseVSockPort + 1,
		StdoutPort:  baseVSockPort + 2,
		StderrPort:  baseVSockPort + 3,
	}
}

func hypervisorStateForSpec(spec models.MicroVMSpec, stateRoot string) (*HypervisorState, error) {
	fsSvc := afero.NewOsFs()

	switch spec.Provider {
	case "firecracker":
		vmSvc := firecracker.New(&firecracker.Config{
			FirecrackerBin: "/usr/bin/firecracker",
			StateRoot:      stateRoot,
		}, fsSvc)

		return &HypervisorState{
			fsSvc:     fsSvc,
			vmSvc:     vmSvc,
			vmStopped: make(chan struct{}),
		}, nil
	case "cloudhypervisor":
		vmSvc := cloudhypervisor.New(&cloudhypervisor.Config{
			CloudHypervisorBin: "/usr/bin/cloud-hypervisor",
			StateRoot:          stateRoot,
		}, fsSvc)

		return &HypervisorState{
			fsSvc:     fsSvc,
			vmSvc:     vmSvc,
			vmStopped: make(chan struct{}),
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

	host := s.fifos[req.GetID()][req.GetExecID()]

	if resp.GetStatus() != task.Status_RUNNING {
		return resp, nil
	}

	resp.Stdin = host.Stdin
	resp.Stdout = host.Stdout
	resp.Stderr = host.Stderr

	state, err := s.vmState.ioProxyClient.State(ctx, &ioproxy.StateRequest{
		ID:     req.GetID(),
		ExecID: req.GetExecID(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check proxy status: %w", err)
	}

	if state.GetIsOpen() {
		return resp, nil
	}

	extraData := generateExtraData(s.getAndIncrementPortCount(), nil, nil)
	attach := ioproxy.AttachRequest{
		ID:         req.GetID(),
		ExecID:     req.GetExecID(),
		StdinPort:  extraData.GetStdinPort(),
		StdoutPort: extraData.GetStdoutPort(),
		StderrPort: extraData.GetStderrPort(),
	}

	_, err = s.vmState.ioProxyClient.Attach(ctx, &attach)
	if err != nil {
		return nil, fmt.Errorf("failed to attach IO Proxy: %w", err)
	}

	ioConnectorSet, err := utils.NewIOProxy(log.G(ctx), host.Stdin, host.Stdout, host.Stderr, s.vmState.vmSvc.VSockPath(s.vmState.vm), extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to create IO Proxy: %w", err)
	}

	if err := s.taskManager.AttachIO(ctx, req.GetID(), req.GetExecID(), ioConnectorSet); err != nil {
		return nil, fmt.Errorf("failed to attach IO Proxy: %w", err)
	}

	return resp, nil
}

func (s *HyperShim) vmCompletion(waitErr error) {
	if waitErr != nil {
		log.G(s.shimCtx).WithError(waitErr).Error("failed to wait for process")
	}

	close(s.vmState.vmStopped)
	s.shimCancel()
}

func (s *HyperShim) Create(ctx context.Context, req *taskAPI.CreateTaskRequest) (_ *taskAPI.CreateTaskResponse, retErr error) {
	ociSpec, err := oci.ReadSpec(req.GetBundle() + "/config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read spec at %s", req.GetBundle())
	}

	networkNs := ""
	filteredNs := make([]specs.LinuxNamespace, 0)
	for _, ns := range ociSpec.Linux.Namespaces {
		if ns.Type == "network" {
			networkNs = ns.Path
			// This OCI config is also passed to the agent inside the VM, so remove
			// our custom network namespace from the spec
			continue
		}
		filteredNs = append(filteredNs, ns)
	}
	ociSpec.Linux.Namespaces = filteredNs
	if networkNs == "" {
		return nil, errors.New("no network namespace specified")
	}

	if s.vmState != nil {
		return nil, errors.New("create called multiple times")
	}

	if len(req.GetRootfs()) != 1 {
		return nil, errors.New("got multiple entries in rootfs")
	}

	rootfs := req.GetRootfs()[0]
	if rootfs.GetType() != "ext4" {
		return nil, fmt.Errorf("got non-ext4 rootfs: %s", rootfs.GetType())
	}

	spec, err := parseOpts(req.GetOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to parse options: %w", err)
	}

	spec.ImagePath = rootfs.GetSource()
	spec.GuestMAC = "06:00:AC:10:00:02"

	hypervisorState, err := hypervisorStateForSpec(spec, s.stateRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create hypervisor state: %w", err)
	}

	hypervisorState.vm = &models.MicroVM{
		ID:   uuid.NewString(),
		Spec: spec,
	}

	if err := ns.WithNetNSPath(networkNs, func(_ ns.NetNS) error {
		return hypervisorState.vmSvc.Start(ctx, hypervisorState.vm, s.vmCompletion)
	}); err != nil {
		return nil, fmt.Errorf("failed to exec under ns %s: %w", networkNs, err)
	}

	s.vmState = hypervisorState

	defer func() {
		if retErr != nil {
			log.G(ctx).WithError(retErr).Error("Create failed, cleaning up VM and cancelling shim")

			if err := s.vmState.vmSvc.Stop(ctx, s.vmState.vm); err != nil {
				log.G(ctx).WithError(err).Error("failed to stop VM")
			}

			s.shimCancel()
		}
	}()

	// Set the dial timeout to 1 second to give enough time to firecracker or
	// cloud-hypervisor to create the VSOCK file
	conn, err := vsock.DialContext(ctx, hypervisorState.vmSvc.VSockPath(s.vmState.vm), VSockPort, vsock.WithDialTimeout(time.Second), vsock.WithLogger(log.G(ctx)))
	if err != nil {
		return nil, fmt.Errorf("failed to dial vsock connection: %w", err)
	}

	rpcClient := ttrpc.NewClient(conn, ttrpc.WithOnClose(func() { _ = conn.Close() }))

	s.vmState.agentClient = taskAPI.NewTaskClient(rpcClient)
	s.vmState.ioProxyClient = ioproxy.NewIOProxyClient(rpcClient)

	// The image will be exposed as an unmounted block device
	// in the guest, /dev/vdb (/dev/vda is the rootfs)
	req.Rootfs[0].Source = "/dev/vdb"

	ociConfig, err := json.Marshal(ociSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OCI spec: %w", err)
	}

	extraData := generateExtraData(s.getAndIncrementPortCount(), ociConfig, nil)

	req.Options, err = protobuf.MarshalAnyToProto(extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal options: %w", err)
	}

	ioConnectorSet, err := utils.NewIOProxy(log.G(ctx), req.GetStdin(), req.GetStdout(), req.GetStderr(), s.vmState.vmSvc.VSockPath(s.vmState.vm), extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to create IO Proxy: %w", err)
	}

	res, err := s.taskManager.CreateTask(ctx, req, s.vmState.agentClient, ioConnectorSet)

	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	if err := s.addFIFOs(req.GetID(), "", cio.Config{
		Terminal: req.GetTerminal(),
		Stdin:    req.GetStdin(),
		Stdout:   req.GetStdout(),
		Stderr:   req.GetStderr(),
	}); err != nil {
		return nil, fmt.Errorf("failed to add FIFOs: %w", err)
	}

	return res, nil
}

func (s *HyperShim) Start(ctx context.Context, req *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	return s.vmState.agentClient.Start(ctx, req)
}

func (s *HyperShim) Delete(ctx context.Context, req *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	log.G(ctx).Error(s.stateRoot)
	if s.vmState != nil && s.vmState.agentClient != nil {
		return s.taskManager.DeleteProcess(ctx, req, s.vmState.agentClient)
	}

	return nil, errors.New("VM not spawned")
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
	extraData := generateExtraData(s.getAndIncrementPortCount(), nil, req.GetSpec())

	var err error
	req.Spec, err = protobuf.MarshalAnyToProto(extraData)

	if err != nil {
		return nil, fmt.Errorf("failed to create Any: %w", err)
	}

	ioConnectorSet, err := utils.NewIOProxy(log.G(ctx), req.GetStdin(), req.GetStdout(), req.GetStderr(),
		s.vmState.vmSvc.VSockPath(s.vmState.vm), extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to create IO Proxy: %w", err)
	}

	if err := s.addFIFOs(req.GetID(), req.GetExecID(), cio.Config{
		Terminal: req.GetTerminal(),
		Stdin:    req.GetStdin(),
		Stdout:   req.GetStdout(),
		Stderr:   req.GetStderr(),
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
	// vmState being non-nil means that the VM was started
	//nolint:nestif
	if s.taskManager.ShutdownIfEmpty() && s.vmState != nil {
		if s.vmState.agentClient != nil {
			_, err := s.vmState.agentClient.Shutdown(ctx, req)

			if err != nil {
				log.G(ctx).WithError(err).Error("failed to shutdown via agent, force killing VM")
			} else {
				<-s.vmState.vmStopped
			}
		}

		if err := s.vmState.vmSvc.Stop(ctx, s.vmState.vm); err != nil {
			log.G(ctx).WithError(err).Error("failed to stop VM")
		}

		// Wait again since we might have killed the vm in the error case
		<-s.vmState.vmStopped
	}

	return &types.Empty{}, nil
}

func (s *HyperShim) Cleanup(_ context.Context) (*taskAPI.DeleteResponse, error) {
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

func Run() {
	typeurl.Register(&models.MicroVMSpec{})

	shim.Run(
		ShimID,
		func(ctx context.Context, id string, remotePublisher shim.Publisher, shimCancel func()) (shim.Shim, error) {
			hyperShim := &HyperShim{
				id:              id,
				stateRoot:       defaults.StateRootDir + "/shim",
				shimCtx:         ctx,
				remotePublisher: remotePublisher,
				eventExchange:   exchange.NewExchange(),
				taskManager:     utils.NewTaskManager(ctx, log.G(ctx)),
				fifos:           make(map[string]map[string]cio.Config),
				shimCancel:      shimCancel,
			}

			return hyperShim, nil
		},
	)
}
