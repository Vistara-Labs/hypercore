# Vistara Hypercore

Vistara Hypercore is an advanced Hypervisor Abstraction Layer designed to manage and provision microVMs across various hypervisor technologies effectively. It enables seamless creation and lifecycle management of isolated and secure virtual environments, facilitating scalable and efficient infrastructure for modern application deployments.

## Features

- **Multi-Hypervisor Support**: Seamless integration with multiple hypervisors including Firecracker, Cloud Hypervisor, and potentially Unikernels for optimized resource utilization.
- **Hardware-as-Code**: Simplifies infrastructure provisioning using `hac.toml`, enabling specifications that are easy to manage and version.
- **Comprehensive API Coverage**: Supports both gRPC and HTTP APIs to cater to a broad range of application requirements and developer preferences.

## Getting Started

### Prerequisites

- Go 1.20 or later
- Protobuf compiler (protoc)
- gRPC tools for Go

### Installation

Clone the repository and build the project:

```bash
git clone https://github.com/vistara-labs/hypercore.git
cd hypercore
make build
```

### Containerd Setup

`/etc/containerd/config.toml` must contain the following as we rely on the blockfile snapshotter:

```toml
version = 2

[plugins]
  [plugins.'io.containerd.snapshotter.v1.blockfile']
    scratch_file = "/opt/containerd/blockfile"
    root_path = "/opt/blocks"
    fs_type = 'ext4'
    mount_options = []
    recreate_scratch = true
```

Initialize the base blockfile with an empty ext4 filesystem:

```sh
$ sudo dd if=/dev/zero of=/opt/containerd/blockfile bs=1M count=500
$ sudo mkfs.ext4 /opt/containerd/blockfile
```

Build the shim and ensure it is present in `PATH` by symlinking to `/usr/local/bin`, the binary name must be the same as it is mandated by containerd:

```sh
$ go build -o containerd-shim-hypercore-example ./cmd/shim/main.go
$ sudo ln -s $PWD/containerd-shim-hypercore-example /usr/local/bin/
```

Pull the desired image:

```sh
$ ctr image pull docker.io/library/alpine:latest
```

Create a container, specifying our shim as the runtime. This will create a snapshot block device in `/opt/blocks`:

```sh
$ sudo ctr container create --snapshotter blockfile --runtime hypercore.example docker.io/library/alpine:latest shim-test
```

Spawn the task, which will internally execute into our shim and spawn the container under a hypervisor

```sh
$ sudo ctr task start shim-test
<execute commands>
<ls>
...
```

### Running the Service

To start the Vistara Hypercore service:

```bash
./vistarad run
```

This command initializes the service and starts listening for API calls to manage microVMs.

### Usage

Example: Creating a MicroVM
Provision a new microVM using a simple command:

WIP:

```bash
# vis run --config hac.toml
```

### API Example

Create a microVM using the gRPC API:

```bash

# kernel image, root volume are hardcoded for now
grpcurl -plaintext -H "tmptmp: 1" -d '{
    "microvm": {
      "id": "14",
      "labels": {},
      "vcpu": 2,
      "memoryInMb": 1024,
      "additionalVolumes": [],
      "interfaces": [
        {
          "type": "TAP",
          "guestMac": "02:FC:00:00:00:00",
          "overrides": {
            "bridgeName": "eth0"
          },
          "address": {
            "address": "192.254.0.22/32"
          },
          "deviceId": "eth0"
        }
      ],
      "metadata": {},
      "kernel": {
        "image": "/tmp/test",
        "addNetworkConfig": true
      },
      "uid": "testinguid",
      "provider": "firecracker",
      "rootVolume": {
        "id": "1"
      },
      "namespace": "test-b8"
    },
    "metadata": {}
}' localhost:9090 vm.services.api.VMService.Create
```


### Project Structure

This repo is inspired by the Flintlock project. https://github.com/weaveworks-liquidmetal/flintlock
Vistara Hypercore reuses a lot of code from the Flintlock project and create hypervisor abstraction interface and refactored the code to make it more modular and extensible.

Below is the project structure detailing major components and their organization within the repository:

```plaintext
├── cmd/                       #  Command line interface applications.
│   └── vistarad/main.go       #  Entry point for the Vistara daemon.
│
├── internal/
│   └── command/              # Handles CLI commands and options.
│   └── config/               # Configuration management utilities.
│   └── inject/               # Dependency injection setup.
│
├── pkg/
│   └── api/                  # Handles all API requests for the service.
│   └── services/             # Defines the microVM service and associated gRPC protocol files.
│   └── app/                  # Core application logic including initialization and command handling.
│   └── cloudinit/            # Manages cloud-init configurations for instances.
│   └── containerd/           # Integration with containerd for container management.
│   └── hypervisor/           # Abstractions for different hypervisor technologies like Firecracker.
│   └── models/               # Data models used throughout the application.
│   └── network/              # Network management utilities.
│   └── processors/           # Processors for handling specific backend tasks like VM lifecycle.
│   └── queue/                # Management of job queues for task processing.


### Important Components

- **Hypervisor Integration**: In `/pkg/hypervisor`, this module supports different hypervisors and is crucial for the creation and management of microVMs.
- **API and gRPC Services**: Defined within `/pkg/api`, facilitating communication and command execution through gRPC and REST (WIP).
- **Containerd Integration**: In `/pkg/containerd`, integrating with containerd to manage containers and their lifecycles efficiently.
- **Network Services**: Found in `/pkg/network`, crucial for setting up and managing network interfaces and configurations.
- **Cloud-init Configuration**: Located under `/pkg/cloudinit`, responsible for generating cloud-init configurations for instances.
- **Processors and Queue Services**: In `/pkg/processors` and `/pkg/queue`, respectively, these components manage asynchronous tasks and job queues for efficient task processing.
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