package cloudhypervisor

import "vistara-node/pkg/hypervisor/shared"

func DefaultKernelCmdLine() shared.KernelCmdLine {
	return shared.KernelCmdLine{
		"console": "hvc0",
		"root":    "/dev/vda",
		"rw":      "",
		"reboot":  "k",
		"panic":   "1",
	}
}
