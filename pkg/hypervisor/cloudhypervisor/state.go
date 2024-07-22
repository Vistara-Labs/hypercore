package cloudhypervisor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/shared"

	"github.com/spf13/afero"
)

type RuntimeState struct {
	HostIface string `json:"hostIface"`
}

func NewState(vmid, stateDir string, fs afero.Fs) *State {
	return &State{
		stateRoot: fmt.Sprintf("%s/%s", stateDir, vmid),
		fs:        fs,
	}
}

type State struct {
	stateRoot string
	fs        afero.Fs
}

func (s *State) Delete() error {
	return os.RemoveAll(s.stateRoot)
}

func (s *State) Root() string {
	return s.stateRoot
}

func (s *State) PIDPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloudhypervisor.pid")
}

func (s *State) PID() (int, error) {
	return shared.PIDReadFromFile(s.PIDPath(), s.fs)
}

func (s *State) VSockPath() string {
	return fmt.Sprintf("%s/cloudhypervisor.vsock", s.stateRoot)
}

func (s *State) LogPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloudhypervisor.log")
}

func (s *State) StdoutPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloudhypervisor.stdout")
}

func (s *State) StderrPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloudhypervisor.stderr")
}

func (s *State) SetPid(pid int) error {
	return shared.PIDWriteToFile(pid, s.PIDPath(), s.fs)
}

func (s *State) runtimeStatePath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "runtime-state.json")
}

func (s *State) RuntimeState() (RuntimeState, error) {
	runtimeState := RuntimeState{}

	file, err := s.fs.OpenFile(s.runtimeStatePath(), os.O_RDONLY, defaults.DataFilePerm)
	if err != nil {
		return runtimeState, fmt.Errorf("failed to open state file: %w", err)
	}

	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		return runtimeState, fmt.Errorf("failed to read state file: %w", err)
	}

	if err = json.Unmarshal(buf, &runtimeState); err != nil {
		return runtimeState, fmt.Errorf("failed to unmarshal state json: %w", err)
	}

	return runtimeState, nil
}

func (s *State) SetRuntimeState(runtimeState RuntimeState) error {
	stateBytes, err := json.Marshal(&runtimeState)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	file, err := s.fs.OpenFile(s.runtimeStatePath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("failed to open state file: %w", err)
	}

	defer file.Close()

	_, err = file.Write(stateBytes)
	if err != nil {
		return fmt.Errorf("failed to write to state file: %w", err)
	}

	return nil
}
