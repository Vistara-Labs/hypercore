package runc

import (
	"fmt"
	"os"
	"vistara-node/pkg/models"

	"github.com/spf13/afero"
)

type State interface {
	Root() string

	StdoutPath() string
	StderrPath() string

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

func (s *fsState) StdoutPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "runc.stdout")
}

func (s *fsState) StderrPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "runc.stderr")
}
