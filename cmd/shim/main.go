package main

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"time"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/protobuf"
	"github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/ttrpc"
	"github.com/firecracker-microvm/firecracker-containerd/eventbridge"
	"github.com/firecracker-microvm/firecracker-containerd/proto"
	"github.com/firecracker-microvm/firecracker-containerd/utils"
	"github.com/firecracker-microvm/firecracker-go-sdk/vsock"
	"github.com/google/uuid"
	"github.com/spf13/afero"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"vistara-node/pkg/hypervisor/firecracker"
	"vistara-node/pkg/models"
	"vistara-node/pkg/network"
	"vistara-node/pkg/ports"
)

const ShimID = "hypercore.example"
const HostIface = "ens2"
const KernelImage = "/home/dev/hypercore/vmlinux-5.10.217"
const RootfsPath = "/home/dev/firecracker-containerd/tools/image-builder/rootfs.img"
const VSockPort = 10789

type HypervisorState struct {
	networkSvc        ports.NetworkService
	fsSvc             afero.Fs
	vmSvc             ports.MicroVMService
	vm                *models.MicroVM
	eventBridgeClient eventbridge.Getter
	agentClient       taskAPI.TaskService
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
}

func generateExtraData(jsonBytes []byte, options *types.Any) *proto.ExtraData {
	var opts *types.Any
	if options != nil {
		// Copy values of existing options over
		valCopy := make([]byte, len(options.Value))
		copy(valCopy, options.Value)
		opts = &types.Any{
			TypeUrl: options.TypeUrl,
			Value:   valCopy,
		}
	}

	return &proto.ExtraData{
		JsonSpec:    jsonBytes,
		RuncOptions: opts,
		StdinPort:   VSockPort + 1,
		StdoutPort:  VSockPort + 2,
		StderrPort:  VSockPort + 3,
	}
}

func (s *HyperShim) State(ctx context.Context, req *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	// TODO IO Proxy
	return s.vmState.agentClient.State(ctx, req)
}

func (s *HyperShim) Create(ctx context.Context, req *taskAPI.CreateTaskRequest) (*taskAPI.CreateTaskResponse, error) {
	if s.vmState != nil {
		return nil, errors.New("Create called multiple times")
	}

	networkSvc := network.New(&network.Config{
		BridgeName: HostIface,
	})
	fsSvc := afero.NewOsFs()
	// TODO pass metadata via containerd and use the appropriate
	// VM provider
	vmSvc := firecracker.New(&firecracker.Config{
		FirecrackerBin: "/usr/bin/firecracker",
		StateRoot:      s.stateRoot,
	}, networkSvc, fsSvc)

	hypervisorState := &HypervisorState{
		networkSvc: networkSvc,
		fsSvc:      fsSvc,
		vmSvc:      vmSvc,
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

	hypervisorState.vm = &models.MicroVM{
		ID:      *vmid,
		Version: 2,
		Spec: models.MicroVMSpec{
			Kernel:     KernelImage,
			RootfsPath: RootfsPath,
			ImagePath:  rootfs.Source,
			// TODO pick up from metadata
			VCPU:       1,
			MemoryInMb: 1024,
			HostNetDev: HostIface,
			GuestMAC:   "06:00:AC:10:00:02",
		},
	}

	if err := vmSvc.Start(ctx, hypervisorState.vm); err != nil {
		return nil, fmt.Errorf("failed to start VM: %w", err)
	}

	s.vmState = hypervisorState

	conn, err := vsock.DialContext(ctx, vmSvc.VSockPath(s.vmState.vm), VSockPort, vsock.WithLogger(log.G(ctx)))
	if err != nil {
		return nil, err
	}

	rpcClient := ttrpc.NewClient(conn, ttrpc.WithOnClose(func() { _ = conn.Close() }))

	s.vmState.agentClient = taskAPI.NewTaskClient(rpcClient)
	s.vmState.eventBridgeClient = eventbridge.NewGetterClient(rpcClient)

	// The image will be exposed as an unmounted block device
	// in the guest, /dev/vdb (/dev/vda is the rootfs)
	req.Rootfs[0].Source = "/dev/vdb"

	ociConfig, err := os.ReadFile(req.Bundle + "/config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read config.json from %s: %w", req.Bundle, err)
	}

	extraData := generateExtraData(ociConfig, req.Options)

	req.Options, err = protobuf.MarshalAnyToProto(extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal options: %w", err)
	}

	ioConnectorSet, err := utils.NewIOProxy(log.G(ctx), req.Stdin, req.Stdout, req.Stderr, s.vmState.vmSvc.VSockPath(s.vmState.vm), extraData)
	if err != nil {
		return nil, fmt.Errorf("failed to create IOProxy: %w", err)
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
	// TODO IO Proxy
	return s.vmState.agentClient.Exec(ctx, req)
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
			}

			hyperShim.startEventForwarders()

			return hyperShim, nil
		},
	)
}
