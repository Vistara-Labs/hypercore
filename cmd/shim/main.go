package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/containerd/containerd/runtime/v2/shim"
	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"vistara-node/pkg/ports"
	"vistara-node/pkg/network"
	"vistara-node/pkg/models"
	"vistara-node/pkg/hypervisor/firecracker"
)

const ShimID = "hypercore.example"
const HostIface = "ens2"

type HypervisorState struct {
 	networkSvc      ports.NetworkService
	fsSvc           afero.Fs
    vmSvc ports.MicroVMService
    vm *models.MicroVM
}

type HyperShim struct {
	id string
	stateRoot string
	HypervisorState *HypervisorState
}

func (s *HyperShim) Stop(ctx context.Context, id string) (opts shim.StopStatus) {
    return shim.StopStatus{
        // Pid: s.HypervisorState.vmSvc.Pid(),
        // ExitedAt will be 0
    }
}

func (s *HyperShim) State(ctx context.Context, req *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
    return nil, errors.New("State")
}

func (s *HyperShim) Create(ctx context.Context, req *taskAPI.CreateTaskRequest) (*taskAPI.CreateTaskResponse, error) {
    if s.HypervisorState != nil {
        return nil, errors.New("Create called multiple times")
    }

    networkSvc := network.New(&network.Config{})
    fsSvc := afero.NewOsFs()
    // TODO pass metadata via containerd and use the appropriate
    // VM provider
    vmSvc := firecracker.New(&firecracker.Config{
        FirecrackerBin: "/usr/bin/firecracker",
        StateRoot: s.stateRoot,
    }, networkSvc, fsSvc)

    hypervisorState := &HypervisorState{
        networkSvc: network.New(&network.Config{
            BridgeName: HostIface,
        }),
        fsSvc: fsSvc,
        vmSvc: vmSvc,
    }

    if len(req.Rootfs) != 1 {
        return nil, errors.New("got multiple entries in rootfs")
    }

    rootfs := req.Rootfs[0]
    if rootfs.Type != "ext4" {
        return nil, fmt.Errorf("got non-ext4 rootfs: %s", rootfs.Type)
    }

    vmid, err := models.NewVMID("hypercore", "", 0)
    if err != nil {
        return nil, fmt.Errorf("failed to create new VMID: %w", err)
    }

    vm := &models.MicroVM{
        ID: *vmid,
        Spec: models.MicroVMSpec{
            Kernel: "",
            // TODO pick up from metadata
            VCPU: 1,
            MemoryInMb: 1024,
            HostNetDev: HostIface,
            RootfsPath: rootfs.Source,
        },
    }

    if err := vmSvc.Start(ctx, vm); err != nil {
        return nil, fmt.Errorf("failed to start VM: %w", err)
    }

    pid, err := vmSvc.Pid(ctx, nil)
    if err != nil {
        return nil, err
    }

    hypervisorState.vm = vm
    s.HypervisorState = hypervisorState
    return &taskAPI.CreateTaskResponse{
        Pid: uint32(pid),
    }, nil
}

func (s *HyperShim) Start(ctx context.Context, req *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
    return nil, errors.New("Start")
}

func (s *HyperShim) Delete(ctx context.Context, req *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
    return nil, errors.New("Delete")
}

func (s *HyperShim) Pids(ctx context.Context, req *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
    return nil, errors.New("Pids")
}

func (s *HyperShim) Pause(ctx context.Context, req *taskAPI.PauseRequest) (*emptypb.Empty, error) {
    return nil, errors.New("Pause")
}

func (s *HyperShim) Resume(ctx context.Context, req *taskAPI.ResumeRequest) (*emptypb.Empty, error) {
    return nil, errors.New("Resume")
}

func (s *HyperShim) Checkpoint(ctx context.Context, req *taskAPI.CheckpointTaskRequest) (*emptypb.Empty, error) {
    return nil, errors.New("Checkpoint")
}

func (s *HyperShim) Kill(ctx context.Context, req *taskAPI.KillRequest) (*emptypb.Empty, error) {
    return nil, errors.New("Kill")
}

func (s *HyperShim) Exec(ctx context.Context, req *taskAPI.ExecProcessRequest) (*emptypb.Empty, error) {
    return nil, errors.New("Exec")
}

func (s *HyperShim) ResizePty(ctx context.Context, req *taskAPI.ResizePtyRequest) (*emptypb.Empty, error) {
    return nil, errors.New("ResizePty")
}

func (s *HyperShim) CloseIO(ctx context.Context, req *taskAPI.CloseIORequest) (*emptypb.Empty, error) {
    return nil, errors.New("CloseIO")
}

func (s *HyperShim) Update(ctx context.Context, req *taskAPI.UpdateTaskRequest) (*emptypb.Empty, error) {
    return nil, errors.New("Update")
}

func (s *HyperShim) Wait(ctx context.Context, req *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
    return nil, errors.New("Wait")
}

func (s *HyperShim) Stats(ctx context.Context, req *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
    return nil, errors.New("Stats")
}

func (s *HyperShim) Connect(ctx context.Context, req *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
    return nil, errors.New("Connect")
}

func (s *HyperShim) Shutdown(ctx context.Context, req *taskAPI.ShutdownRequest) (*emptypb.Empty, error) {
    return nil, errors.New("Shutdown")
}

func (s *HyperShim) Cleanup(ctx context.Context) (*taskAPI.DeleteResponse, error) {
    return nil, errors.New("Cleanup")
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

func main() {
	shim.Run(
		ShimID,
		func(ctx context.Context, id string, remotePublisher shim.Publisher, shimCancel func()) (shim.Shim, error) {
			return &HyperShim{
				id: id,
				stateRoot: "/tmp",
			}, nil
		},
	)
}
