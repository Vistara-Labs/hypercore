package hypervisor

import (
	"fmt"
	"vistara-node/internal/config"
	"vistara-node/pkg/hypervisor/cloudhypervisor"
	"vistara-node/pkg/hypervisor/docker"
	"vistara-node/pkg/hypervisor/firecracker"
	"vistara-node/pkg/hypervisor/runc"
	"vistara-node/pkg/ports"

	"github.com/spf13/afero"
)

// NewFromConfig will create instances of the vm providers based on the config.
func NewFromConfig(cfg *config.Config, networkSvc ports.NetworkService, diskSvc ports.DiskService, fs afero.Fs, containerd ports.MicroVMRepository) (map[string]ports.MicroVMService, error) {
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

	dockerProvider, err := docker.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker provider: %w", err)
	}

	providers[docker.HypervisorName] = dockerProvider

	providers[runc.HypervisorName] = runc.New(&runc.Config{
		StateRoot: fmt.Sprintf("%s/vm", cfg.StateRootDir),
	}, containerd, fs)

	return providers, nil
}
