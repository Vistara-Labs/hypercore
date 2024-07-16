package cloudhypervisor

import "vistara-node/pkg/hypervisor/shared"

func DefaultKernelCmdLine() shared.KernelCmdLine {
    return shared.KernelCmdLine{
        "console": "ttyS0",
        "init": "/sbin/overlay-init",
        "root": "/dev/vda",
        "systemd.journald.forward_to_console": "",
        "systemd.unit": "firecracker.target",
    }
}
