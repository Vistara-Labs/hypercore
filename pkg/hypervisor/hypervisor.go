package hypervisor

import (
	"errors"
	"fmt"
	"vistara-node/internal/config"
	"vistara-node/pkg/hypervisor/cloudhypervisor"
	"vistara-node/pkg/hypervisor/firecracker"
	"vistara-node/pkg/ports"

	"github.com/spf13/afero"
)

// NewFromConfig will create instances of the vm providers based on the config.
func NewFromConfig(cfg *config.Config, networkSvc ports.NetworkService, diskSvc ports.DiskService, fs afero.Fs) (map[string]ports.MicroVMService, error) {
	providers := map[string]ports.MicroVMService{}

	if cfg.FirecrackerBin != "" {
		providers[firecracker.HypervisorName] = firecracker.New(&firecracker.Config{
			FirecrackerBin: cfg.FirecrackerBin,
			RunDetached:    cfg.FirecrackerDetatch,
			StateRoot:      fmt.Sprintf("%s/vm", cfg.StateRootDir),
		}, networkSvc, fs)
	}

	if cfg.CloudHypervisorBin != "" {
		providers[cloudhypervisor.HypervisorName] = cloudhypervisor.New(&cloudhypervisor.Config{
			CloudHypervisorBin: cfg.CloudHypervisorBin,
			RunDetached:        cfg.CloudHypervisorDetatch,
			StateRoot:          fmt.Sprintf("%s/vm", cfg.StateRootDir),
		}, networkSvc, diskSvc, fs)
	}

	if len(providers) == 0 {
		return nil, errors.New("you must enable at least 1 microvm provider")
	}

	return providers, nil
}
