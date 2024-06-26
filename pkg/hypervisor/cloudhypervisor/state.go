package cloudhypervisor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/shared"
	"vistara-node/pkg/models"

	"github.com/spf13/afero"
)

type RuntimeState struct {
	HostIface string `json:"hostIface"`
}

type State interface {
	Root() string

	PID() (int, error)
	PIDPath() string
	SetPid(pid int) error

	RuntimeState() (RuntimeState, error)
	SetRuntimeState(runtimeState RuntimeState) error

	LogPath() string
	StdoutPath() string
	StderrPath() string
	SockPath() string

	CloudInitImage() string

	Delete() error
}

func NewState(vmid models.VMID, stateDir string, fs afero.Fs) State {
	return &fsState{
		stateRoot: fmt.Sprintf("%s/%s", stateDir, vmid.String()),
		fs:        fs,
	}
}

type fsState struct {
	stateRoot string
	fs        afero.Fs
}

func (s *fsState) Delete() error {
	return os.RemoveAll(s.stateRoot)
}

func (s *fsState) Root() string {
	return s.stateRoot
}

func (s *fsState) PIDPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloudhypervisor.pid")
}

func (s *fsState) PID() (int, error) {
	return shared.PIDReadFromFile(s.PIDPath(), s.fs)
}

func (s *fsState) LogPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloudhypervisor.log")
}

func (s *fsState) StdoutPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloudhypervisor.stdout")
}

func (s *fsState) StderrPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloudhypervisor.stderr")
}

func (s *fsState) SockPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloudhypervisor.sock")
}

func (s *fsState) CloudInitImage() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "cloud-init.img")
}

func (s *fsState) SetPid(pid int) error {
	return shared.PIDWriteToFile(pid, s.PIDPath(), s.fs)
}

func (s *fsState) runtimeStatePath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "runtime-state.json")
}

func (s *fsState) RuntimeState() (RuntimeState, error) {
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

func (s *fsState) SetRuntimeState(runtimeState RuntimeState) error {
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
