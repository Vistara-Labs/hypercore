package firecracker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/shared"
	"vistara-node/pkg/models"

	"github.com/spf13/afero"
)

type State struct {
	stateRoot string
	fs        afero.Fs
}

func NewState(vmid models.VMID, stateDir string, fs afero.Fs) *State {
	return &State{
		stateRoot: fmt.Sprintf("%s/%s", stateDir, vmid.String()),
		fs:        fs,
	}
}

func (s *State) Delete() error {
	return os.RemoveAll(s.stateRoot)
}

func (s *State) Root() string {
	return s.stateRoot
}

func (s *State) PIDPath() string {
	return fmt.Sprintf("%s/firecracker.pid", s.stateRoot)
}

func (s *State) PID() (int, error) {
	return shared.PIDReadFromFile(s.PIDPath(), s.fs)
}

func (s *State) VSockPath() string {
	return fmt.Sprintf("%s/firecracker.vsock", s.stateRoot)
}

func (s *State) LogPath() string {
	return fmt.Sprintf("%s/firecracker.log", s.stateRoot)
}

func (s *State) MetricsPath() string {
	return fmt.Sprintf("%s/firecracker.metrics", s.stateRoot)
}

func (s *State) StdoutPath() string {
	return fmt.Sprintf("%s/firecracker.stdout", s.stateRoot)
}

func (s *State) StderrPath() string {
	return fmt.Sprintf("%s/firecracker.stderr", s.stateRoot)
}

func (s *State) SetPid(pid int) error {
	return shared.PIDWriteToFile(pid, s.PIDPath(), s.fs)
}

func (s *State) ConfigPath() string {
	return fmt.Sprintf("%s/firecracker.cfg", s.stateRoot)
}

func (s *State) Config() (VmmConfig, error) {
	cfg := VmmConfig{}

	err := s.readJSONFile(&cfg, s.ConfigPath())
	if err != nil {
		return VmmConfig{}, fmt.Errorf("firecracker config: %w", err)
	}

	return cfg, nil
}

func (s *State) SetConfig(cfg *VmmConfig) error {
	err := s.writeToFileAsJSON(cfg, s.ConfigPath())
	if err != nil {
		return fmt.Errorf("firecracker config: %w", err)
	}

	return nil
}

func (s *State) SetMetadata(meta *Metadata) error {
	decoded := &Metadata{
		Latest: map[string]string{},
	}

	// Try to base64 decode values, if we can't decode, use them at they are,
	// in case we got them without base64 encoding.
	for key, value := range meta.Latest {
		decodedValue, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			decoded.Latest[key] = value
		} else {
			decoded.Latest[key] = string(decodedValue)
		}
	}

	err := s.writeToFileAsJSON(decoded, s.MetadataPath())
	if err != nil {
		return fmt.Errorf("firecracker metadata: %w", err)
	}

	return nil
}

func (s *State) Metadata() (Metadata, error) {
	meta := Metadata{}

	err := s.readJSONFile(&meta, s.ConfigPath())
	if err != nil {
		return Metadata{}, fmt.Errorf("firecracker metadata: %w", err)
	}

	return meta, nil
}

func (s *State) MetadataPath() string {
	return fmt.Sprintf("%s/metadata.json", s.stateRoot)
}

func (s *State) readJSONFile(cfg interface{}, inputFile string) error {
	file, err := s.fs.Open(inputFile)
	if err != nil {
		return fmt.Errorf("opening file %s: %w", inputFile, err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("reading file %s: %w", inputFile, err)
	}

	err = json.Unmarshal(data, cfg)
	if err != nil {
		return fmt.Errorf("unmarshalling: %w", err)
	}

	return nil
}

func (s *State) writeToFileAsJSON(cfg interface{}, outputFilePath string) error {
	data, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}

	file, err := s.fs.OpenFile(outputFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening output file %s: %w", outputFilePath, err)
	}

	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("writing output file %s: %w", outputFilePath, err)
	}

	return nil
}
