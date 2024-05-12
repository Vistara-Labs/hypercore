# Hypercore Node - Create and manage the lifecycle of microVMs.

Hypercore is a service that runs on every vimana node operator. It listens for requests to create and manage microVMs.

## API gRPC. TODO: HTTP 

```
./vistarad run # should start the node daemon, exposes gRPC API for creating and managing microVMs.

- Create and delete microVMs
- Manage the lifecycle of microVMs (i.e. start, stop, pause)

```

Hypercore - Vistara node:

Service is continuously running waiting for requests.
Service is running on every vimana node operator, running using, vimana run vistara full-node 
	- starts the vistara node (hypercore) daemon
	- listens on an HTTP or a gRPC API endpoint for requests with hac.toml file
	- performs tasks i.e. launches jobs

How to create provision request:
	1. Receive HTTP or gRPC request
	2. they can do it through a command. Let's do this approach of a command line first.
	   e.g. fl microvm create --host (flintlock_host to create the microvm on)
  3. Run a microVM
     vis run --config hac.toml
     Options:
      --host (port where the vistara node is running) --image (image to run) --name (name of the microvm) --memory (memory to allocate) --cpu (cpu to allocate) --disk (disk to allocate) --network (network to allocate)
  4. Create vis client e.g. a *app, a.createVisClient() -> calls vis.NewClient() -> returns a new client
  5. Create a microVM
     vis.CreateMicroVM() -> calls vis.NewMicroVM() -> returns a new microVM

Let's start with the command line interface first.
  e.g. vis run --config hac.toml (this will run the microvm with the configuration file hac.toml)

gRPC request vm.services.api.VMService Create()
      - vm.services.api.CreateMicroVMRequest
-> grpc.go -> s.commandSvc.Create -> Should create a microVM spec
  -> save it to containerd repo
    -> put the vmid in a queue
      -> processQueue() -> retrieve vm from vmid

Vistara Hypercore:
1. in vistarad, run() starts the gRPC server
2. RunProcessors starts event listeners
3. VMProcessors Runs VM Processor that listens to events
4. MicroVMService implements lifecycle management of microVMs
5. VMProcessors -> processQueue() -> retrieve vm from vmid

This repo is inspired by the Flintlock project. https://github.com/weaveworks-liquidmetal/flintlock
Vistara Hypercore reuses a lot of code from the Flintlock project and create hypervisor abstraction interface to support creation of multiple hypervisors.

We can also create a REST API for the same. We can use the gRPC API for the same.
  e.g.
    1. vis run (starts the gRPC server and listens for requests)
    2. vis createMicroVM() (creates a microVM)
    3. vis deleteMicroVM() (deletes a microVM)
    4. vis getMicroVM() (gets a microVM)
    5. vis listMicroVM() (lists all microVMs)


# Example provisioning a microVM

TODO: Add hac.toml file parser to read the configuration file and create a microVM.

```
grpcurl -plaintext -H "tmptmp: 1" -d '{
  "microvm": {
    "id": "12",
    "namespace": "testasazc",
    "labels": {},
    "vcpu": 2,
    "memoryInMb": 8,
    "additionalVolumes": [],
    "interfaces": [],
    "metadata": {},
    "kernel": {
      "cmdline": {
        "bash": "100"
      },
      "image": "/tmp/test",
      "addNetworkConfig": true
    },
    "uid": "testinguid",
    "provider": "firecracker"
  },
  "metadata": {}
}' localhost:9090 vm.services.api.VMService.Create
```

Let's create a gRPC proto file first.
  e.g. vistara.proto
  ```
  syntax = "proto3";
  package vistara;

  service Vistara {
    rpc CreateMicroVM(CreateMicroVMRequest) returns (CreateMicroVMResponse) {}
    rpc DeleteMicroVM(DeleteMicroVMRequest) returns (DeleteMicroVMResponse) {}
    rpc GetMicroVM(GetMicroVMRequest) returns (GetMicroVMResponse) {}
    rpc ListMicroVM(ListMicroVMRequest) returns (ListMicroVMResponse) {}
  }

  message CreateMicroVMRequest {
    string name = 1;
    string image = 2;
    int32 memory = 3;
    int32 cpu = 4;
    int32 disk = 5;
    int32 network = 6;
  }

  message CreateMicroVMResponse {
    string id = 1;
  }

  message DeleteMicroVMRequest {
    string id = 1;
  }

  message DeleteMicroVMResponse {
    string id = 1;
  }

  message GetMicroVMRequest {
    string id = 1;
  }

  message GetMicroVMResponse {
    string id = 1;
    string name = 2;
    string image = 3;
    int32 memory = 4;
    int32 cpu = 5;
    int32 disk = 6;
    int32 network = 7;
  }

  message ListMicroVMRequest {
    string id = 1;
  }

  message ListMicroVMResponse {
    repeated string id = 1;
    repeated string name = 2;
    repeated string image = 3;
    repeated int32 memory = 4;
    repeated int32 cpu = 5;
    repeated int32 disk = 6;
    repeated int32 network = 7;
  }
  ```
  We can use the protoc compiler to generate the gRPC server and client code.




/vistara-node
  /cmd
    /vistara-node
      - main.go  # CLI entry point
  /pkg
    /hypervisor
      - hypervisor.go  # Defines the Hypervisor interface
      /firecracker
        - firecracker.go  # Firecracker implementation
      /cloudhypervisor
        - cloudhypervisor.go  # Cloud Hypervisor implementation
      /nanovms
        - nanovms.go  # NanoVMs (Unikernel) implementation
    /api
      - api.go  # Defines the REST API handlers
  /internal
    /config
      - config.go  # Configuration loading and parsing
  /scripts
    - setup.sh  # Setup scripts for dependencies
  go.mod
  go.sum
  README.md
