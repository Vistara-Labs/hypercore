package shim

import (
	"context"
	"os"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/runtime/v2/shim"
)

const shimID = "hypercore"

type HyperShim struct {
}

func (s *HyperShim) Stop(ctx context.Context, id string) (opts shim.StopStatus) { panic("TODO") }

func (s *HyperShim) State(context.Context, *StateRequest) (*StateResponse, error) { panic("TODO") }

func (s *HyperShim) Create(context.Context, *CreateTaskRequest) (*CreateTaskResponse, error) {
	panic("TODO")
}

func (s *HyperShim) Start(context.Context, *StartRequest) (*StartResponse, error) {
	// TODO start agent under the VM
}

func (s *HyperShim) Delete(context.Context, *DeleteRequest) (*DeleteResponse, error) { panic("TODO") }

func (s *HyperShim) Pids(context.Context, *PidsRequest) (*PidsResponse, error) { panic("TODO") }

func (s *HyperShim) Pause(context.Context, *PauseRequest) (*emptypb.Empty, error) { panic("TODO") }

func (s *HyperShim) Resume(context.Context, *ResumeRequest) (*emptypb.Empty, error) { panic("TODO") }

func (s *HyperShim) Checkpoint(context.Context, *CheckpointTaskRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) Kill(context.Context, *KillRequest) (*emptypb.Empty, error) { panic("TODO") }

func (s *HyperShim) Exec(context.Context, *ExecProcessRequest) (*emptypb.Empty, error) { panic("TODO") }

func (s *HyperShim) ResizePty(context.Context, *ResizePtyRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) CloseIO(context.Context, *CloseIORequest) (*emptypb.Empty, error) { panic("TODO") }

func (s *HyperShim) Update(context.Context, *UpdateTaskRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func (s *HyperShim) Wait(context.Context, *WaitRequest) (*WaitResponse, error) { panic("TODO") }

func (s *HyperShim) Stats(context.Context, *StatsRequest) (*StatsResponse, error) { panic("TODO") }

func (s *HyperShim) Connect(context.Context, *ConnectRequest) (*ConnectResponse, error) {
	panic("TODO")
}

func (s *HyperShim) Shutdown(context.Context, *ShutdownRequest) (*emptypb.Empty, error) {
	panic("TODO")
}

func main() {
	shim.Run(
		shimID,
		func(ctx context.Context, id string, remotePublisher shim.Publisher, shimCancel func()) (shim.Shim, error) {
			// TODO Create directory for storing logs, FIFOs, etc.
			return shim.Shim{}, nil
		},
	)
}
