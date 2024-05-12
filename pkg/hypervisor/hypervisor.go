package hypervisor

import (
	"errors"
	"fmt"
	"vistara-node/internal/config"
	"vistara-node/pkg/hypervisor/firecracker"
	"vistara-node/pkg/ports"

	"github.com/spf13/afero"
)

var errUnknownHypervisor = "unknown hypervisor"

func New(name string, cfg *config.Config, networkSvc ports.NetworkService, fs afero.Fs) (ports.MicroVMService, error) {
	switch name {
	case firecracker.HypervisorName:
		return firecracker.New(firecrackerConfig(cfg), networkSvc, fs), nil
	default:
		return nil, nil
	}

}

func firecrackerConfig(cfg *config.Config) *firecracker.Config {
	return &firecracker.Config{
		FirecrackerBin: cfg.FirecrackerBin,
		RunDetached:    cfg.FirecrackerDetatch,
		StateRoot:      fmt.Sprintf("%s/vm", cfg.StateRootDir),
	}
}

// NewFromConfig will create instances of the vm providers based on the config.
func NewFromConfig(cfg *config.Config, networkSvc ports.NetworkService, diskSvc ports.DiskService, fs afero.Fs) (map[string]ports.MicroVMService, error) {
	providers := map[string]ports.MicroVMService{}

	if cfg.FirecrackerBin != "" {
		providers[firecracker.HypervisorName] = firecracker.New(firecrackerConfig(cfg), networkSvc, fs)
	}

	if len(providers) == 0 {
		return nil, errors.New("you must enable at least 1 microvm provider")
	}

	return providers, nil
}
