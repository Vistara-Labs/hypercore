# Vistara Hypercore

Vistara Hypercore is an advanced Hypervisor Abstraction Layer designed to manage and provision microVMs across various hypervisor technologies effectively. It enables seamless creation and lifecycle management of isolated and secure virtual environments, facilitating scalable and efficient infrastructure for modern application deployments.

## Features

- **Multi-Hypervisor Support**: Seamless integration with multiple hypervisors including Firecracker, Cloud Hypervisor, and potentially Unikernels for optimized resource utilization.
- **Hardware-as-Code**: Simplifies infrastructure provisioning using `hac.toml`, enabling specifications that are easy to manage and version.
- **Comprehensive API Coverage**: Supports both gRPC and HTTP APIs to cater to a broad range of application requirements and developer preferences.

## Getting Started

### Building

Clone the repository:

```bash
git clone https://github.com/vistara-labs/hypercore.git
```

Build the unified hypercore binary for spawning and managing the lifecycle of MicroVMs. Depending on the binary name it either runs the hypercore CLI, or the containerd shim. The shim must be present in `$PATH` so we can symlink it to `/usr/local/bin`:

```bash
$ go build -o containerd-shim-hypercore-example cmd/main.go
# Assuming /usr/local/bin is in $PATH, we can symlink the binary there
$ sudo ln -s $PWD/containerd-shim-hypercore-example /usr/local/bin/
# For hypercore CLI
$ ln -s containerd-shim-hypercore-example hypercore
```

### Containerd Setup

We run a separate instance of containerd containing the relevant snapshotter configuration for it to play nicely with hypercore, it will take care of the scratch file setup required by the `blockfile` snapshotter, and all the state will be stored in `/var/lib/hypercore`:

```bash
./scripts/containerd.sh
```

### Spawning VMs

1. Setup a `hac.toml` file detailing the VM requirements:

```bash
[spacecore]
name = "Test Node"
description = "Testing"

[hardware] # resource allocation
cores = 4 # Number of CPU cores
memory = 4096 # Memory in MB
kernel = "/home/dev/images/vmlinux-5.10.217"
drive = "/home/dev/firecracker-containerd/tools/image-builder/rootfs.img"
ref = "docker.io/library/alpine:latest" # Reference of the image to use
interface = "ens2" # Host interface to bridge with the VM, eg. eth0
```

2. Use the hypercore CLI to spawn the VM (using firecracker as the VM provider):

```bash
$ sudo ./hypercore spawn --provider firecracker
Creating VM '67a20540-5cd6-4445-adc6-ac609575546a' with config {Spacecore:{name: description:} Hardware:{Cores:4 Memory:4096 Kernel:/home/dev/images/vmlinux-5.10.217 Drive:/home/dev/firecracker-containerd/tools/image-builder/rootfs.img Interface:ens2 Ref:docker.io/library/alpine:latest}}
ID: 08cf7306-1af6-48f2-b2f4-6d638fc428c0
```

3. Attach to the VM using the hypercore CLI

```bash
$ sudo ./hypercore attach 08cf7306-1af6-48f2-b2f4-6d638fc428c0
whoami
root
echo $$
1
cat /etc/os-release
NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.20.1
PRETTY_NAME="Alpine Linux v3.20"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://gitlab.alpinelinux.org/alpine/aports/-/issues"
```

### Architecture Overview

```
                        TTRPC                       TTRPC                  TTRPC/VSOCK            TTRPC
Hypercore CLI (client) <-----> containerd (daemon) <-----> Hypercore Shim <-----------> VM Agent <-----> runc (in VM)
```

- [**Hypercore CLI**](internal/hypercore): The CLI helps perform actions like creating VMs, attaching to them, and cleaning them up, leveraging [`containerd`](https://github.com/containerd/containerd) for pulling images, invoking the `blockfile` snapshotter, and talking with the shim

- [**Hypercore Shim**](pkg/shim): The shim manages the whole VM lifecycle, from provisioning the VM, communicating with the agent, and cleaning up the VM and it's associated resources

- [**VM Agent**](https://github.com/Vistara-Labs/firecracker-containerd/tree/feat-hypercore): The agent is responsible for spawning tasks inside the VM, proxying IO over VSOCK, and handling various API requests defined by containerd. Internally, all actions are directly or indirectly handed off to `runc` for another layer of sandboxing

This diagram shows how various components interact to spawn a VM:

```
               TTRPC                       TTRPC                                 TTRPC/VSOCK                               TTRPC
Hypercore CLI <-----> containerd <-----------------------> Hypercore Shim <-------------------------> VM Agent <----------------------------> runc
                       ` Pull image                         ` Create TAP device for networking         ` Prepare the environment
                                                                                                         for runc by mounting the snapshot
                       ` Invoke snapshotter for raw image   ` Spawn the VM with relevant config/args   ` Forward API requests to runc
                                                              including network interface details      ` Proxy IO for attaching to / spawning
                                                              and attaching the image snapshot           tasks, forwarding logs, etc.
                                                              as a mountable block device
                       ` Spawn shim to create VM
```

### Contributing

Contributions are what make the open-source community such an amazing place to learn, inspire, and create. Any contributions you make are greatly appreciated.


1. Fork the Project
2. Create your Feature Branch (git checkout -b feature/hypespacecore)
3. Commit your Changes (git commit -m 'Add some hypespacecore')
4. Push to the Branch (git push origin feature/hypespacecore)
5. Open a Pull Request

### Acknowledgments

1. Inspired by and builds upon concepts from the Flintlock project.
2. Thanks to all contributors who participate in this project.
