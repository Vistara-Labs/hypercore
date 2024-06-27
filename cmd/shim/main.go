package shim

import (
	"context"
	"fmt"
	"github.com/containerd/fifo"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"os"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/runtime/v2/shim"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

const shimID = "hypercore"

type HyperShim struct {
}

func (s *HyperShim) Stop(ctx context.Context, id string) (opts shim.StopStatus) { panic("TODO") }

func (s *HyperShim) State(context.Context, *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	panic("TODO")
}

func (s *HyperShim) Create(context.Context, *taskAPI.CreateTaskRequest) (*taskAPI.CreateTaskResponse, error) {
	panic("TODO")
}

func (s *HyperShim) Start(ctx context.Context, req *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	agent, err := s.agent()
	if err != nil {
		return nil, err
	}

	resp, err := agent.Start(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *HyperShim) Delete(context.Context, *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	panic("TODO")
}

func (s *HyperShim) Pids(context.Context, *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	panic("TODO")
}

func (s *HyperShim) Pause(context.Context, *taskAPI.PauseRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) Resume(context.Context, *taskAPI.ResumeRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) Checkpoint(context.Context, *taskAPI.CheckpointTaskRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) Kill(context.Context, *taskAPI.KillRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) Exec(context.Context, *taskAPI.ExecProcessRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) ResizePty(context.Context, *taskAPI.ResizePtyRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) CloseIO(context.Context, *taskAPI.CloseIORequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) Update(context.Context, *taskAPI.UpdateTaskRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) Wait(context.Context, *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	panic("TODO")
}

func (s *HyperShim) Stats(context.Context, *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	panic("TODO")
}

func (s *HyperShim) Connect(context.Context, *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	panic("TODO")
}

func (s *HyperShim) Shutdown(context.Context, *taskAPI.ShutdownRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) Cleanup(context.Context) (*taskAPI.DeleteResponse, error) {
	panic("TODO")
}

func (s *HyperShim) StartShim(ctx context.Context, opts shim.StartOpts) (string, error) {
	logFifo, err := fifo.OpenFifo(ctx, "log", unix.O_WRONLY, 0200)
	if err != nil {
		return "", err
	}

	logrus.SetOutput(logFifo)

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get cwd: %w", err)
	}

	vmid := uuid.NewString()
}

func main() {
	shim.Run(
		shimID,
		func(ctx context.Context, id string, remotePublisher shim.Publisher, shimCancel func()) (shim.Shim, error) {
			// TODO Create directory for storing logs, FIFOs, etc.
			return &HyperShim{}, nil
		},
	)
}
