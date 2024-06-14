package cloudhypervisor

import (
	"fmt"
	"os"
	"vistara-node/pkg/hypervisor/shared"
	"vistara-node/pkg/models"

	"github.com/spf13/afero"
)

type State interface {
	Root() string

	PID() (int, error)
	PIDPath() string
	SetPid(pid int) error

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
