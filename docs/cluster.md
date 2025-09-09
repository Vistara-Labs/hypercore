## Introduction

The hypercore cluster allows various nodes to join a decentralized serf-based cluster, and deploy arbitrary containerized workloads on the member nodes, which are further made publicly accessible via a reverse proxy setup

## Requirements

- Node with a static public IP address
- Domain (with DNS access)
- KVM support
- `dmsetup` (for the `containerd` snapshotter https://github.com/containerd/containerd/blob/main/docs/snapshotters/devmapper.md)

## Prerequisite Setup

For exposing services to the outside world, hypercore has a built-in reverse proxy that proxies all incoming traffic over HTTPS and routes it to the correct workload, for it to work securely we need a wildcard subdomain and it’s corresponding TLS certificate:

- Say we own the domain `vistara.dev`, we can create a wildcard subdomain entry, `*.deployments.vistara.dev` pointing to our node’s public IP
- Now, on the node, we can generate TLS certificates for the domain using [`certbot`](https://certbot.eff.org/instructions?ws=nginx&os=pip&tab=wildcard)
    - After following the linked installation steps, simply run `certbot certonly -d '*.my.domain'`
    - If your domain nameservers are not supported by `certbot`, this step would fail. Instead, the manual method can be used where a TXT entry is manually added for verification: `certbot certonly -d '*.my.domain' --manual`
    - The certificate and private key will be stored at `/etc/letsencrypt/live/my.domain`

For some context, each deployed workload will be exposed through the wildcard domain via an identifier. Using a wildcard domain prevents us from creating several new subdomains for each new deployed workload.

Let’s say we deploy a workload `X`, it will be publicly exposed via a unique identifier, such as `497b6ad8-11a3-4701-8b6a-11396e11ca7d.deployments.vistara.dev` and internally routed from the reverse proxy

```
             Browser                                 Hypercore Reverse Proxy                        Internal Workload IP
https://497b---.deployments.vistara.dev -> Host Header: 497b---.deployments.vistara.dev  -> Identifier (497b...) -> 192.168.127.15
                                           Port: 443                                        Port (443) -> 8080
```
 curl -X POST https://api.deployments.vistara.dev:8443/spawn -H "Content-Type: application/json" -d '{"cores": 1, "memory": 512, "image_ref": "registry.vistara.dev/75bbb1e1-1cc3-45fa-8a6c-fd6b99aac5d5:ea3babb98ecd05e1408e17e1da8f43c88a421ad7", "ports": {"443": 3000}, "env": ["OPENAI_API_KEY=sk-or-v1", "SUPABASE_URL=https://enlsvqrfgktlndrnbojk.supabase.co", "SUPABASE_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "GITHUB_ORG=vistara-apps"], "dry_run": false}' | jq 
## Installation & Usage

1. Download `hypercore-vX.Y.Z.tar.gz` from the hypercore repo and extract it to `/opt`. This contains the hypercore binary itself, along with all the dependencies like `containerd`, `cloud-hypervisor`, `firecracker` & `runc`:
    
    ```bash
    $ curl -LO https://github.com/Vistara-Labs/hypercore/releases/download/v0.0.2/hypercore.tar.gz
    $ sudo tar -xf hypercore.tar.gz -C /opt
    $ export PATH="$PATH:/opt/hypercore/bin"
    ```
    
2. Run the `hypercore-containerd` script in another shell to setup the device-mapper pool used for storing `containerd` image snapshots and spawn `containerd`: `sudo /opt/hypercore/bin/hypercore-containerd`
3. Now, spawn hypercore to join the cluster. This will expose your node at your public IP address so that other cluster members can also communicate with it:
    
    ```bash
    $ export PATH="$PATH:/opt/hypercore/bin"
    $ export PUBLIC_IP="$(curl ip.me)"
    $ export HYPERCORE_CLUSTER_IP="..."
    $ export BASE_URL="my.domain"
    $ export TLS_CERT="/etc/letsencrypt/archive/my.domain/fullchain1.pem"
    $ export TLS_KEY="/etc/letsencrypt/archive/deployments.vistara.dev/privkey1.pem"
    $ sudo hypercore cluster \
        --cluster-bind-addr "$PUBLIC_IP:7946" \
        "$HYPERCORE_CLUSTER_IP:7946" \
        --cluster-base-url "$BASE_URL" \
        --cluster-tls-cert "$TLS_CERT" \
        --cluster-tls-key "$TLS_KEY"
    ```
    
    - For testing out spawning on the same node, spawn two hypercore instances - the 1st node should be spawned without a cluster IP (omit the `$HYPERCORE_CLUSTER_IP:7946` argument) and the second should bind to `"$PUBLIC_IP:7947"`, pass `--grpc-bind-addr 0.0.0.0:8001` and `$HYPERCORE_CLUSTER_IP:7946` with `$HYPERCORE_CLUSTER_IP` being same as the public IP to join the cluster
4. To spawn a workload, simply run `hypercore cluster spawn`, this will broadcast a request to all other nodes in the cluster, requesting them to spawn this workload. All eligible nodes give a successful response, and then the host node attempts to spawn the workload on them one-by-one, stopping on the first successful deployment
    1. Example: `hypercore cluster spawn --grpc-bind-addr $NODE_IP:$GRPC_PORT --ports 443:3000 --image-ref registry.vistara.dev/next:latest`
        - Here `NODE_IP` is the public IP of the node and the `GRPC_PORT` is the port passed in the grpc bind address, eg. `8001` (default is `8000`)
5. The logged response will container an identifier for the workload, along with the public address where it’s exposed: `INFO[0001] Got response: id:"3d5e6a90-c184-4d80-896d-d61f1ce935c4"  url:"3d5e6a90-c184-4d80-896d-d61f1ce935c4.deployments.vistara.dev"`

## Limitations

Since we are at a very early stage, some security measures and general functionality is absent:

- Member nodes must have a public static IP along with the ability to expose arbitrary ports to the outside network
- Encryption within the cluster is not yet configured with serf
- Guest → Host network is not fully isolated, meaning a guest can access ports exposed on the host network, and say, brute-force a password-based SSH connection into the host
- Workload state is not maintained in the cluster, meaning that if a node hosting a workload becomes unavailable, there is no mechanism as of now to reschedule the workload on another node
- It is not possible to securely relay secrets onto the chosen node. Say a workload X needs certain environment variables set containing things like database credentials, there is not a straightforward way to make such secrets accessible to the application without a malicious host being able to snoop, either via a memory dump or simply spawning a shell in the VM with the relevant APIs
