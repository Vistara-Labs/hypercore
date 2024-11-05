#!/bin/bash

set -euo pipefail

CLOUDHYPERVISOR_URL="https://github.com/cloud-hypervisor/cloud-hypervisor/releases/download/v40.0/cloud-hypervisor-static"
CLOUDHYPERVISOR_SHA256="0010c1dfb81cccae81c3595a4267d226f824d878c86ef06be5dbe63106be4cce"

FIRECRACKER_URL="https://github.com/firecracker-microvm/firecracker/releases/download/v1.8.0/firecracker-v1.8.0-x86_64.tgz"
FIRECRACKER_SHA256_FILE="https://github.com/firecracker-microvm/firecracker/releases/download/v1.8.0/firecracker-v1.8.0-x86_64.tgz.sha256.txt"

CONTAINERD_URL="https://github.com/containerd/containerd/releases/download/v1.7.20/containerd-static-1.7.20-linux-amd64.tar.gz"
CONTAINERD_SHA256_FILE="https://github.com/containerd/containerd/releases/download/v1.7.20/containerd-static-1.7.20-linux-amd64.tar.gz.sha256sum"

CNI_PLUGINS_URL="https://github.com/containernetworking/plugins/releases/download/v1.5.1/cni-plugins-linux-amd64-v1.5.1.tgz"
CNI_SHA256_FILE="https://github.com/containernetworking/plugins/releases/download/v1.5.1/cni-plugins-linux-amd64-v1.5.1.tgz.sha256"

RUNC_URL="https://github.com/opencontainers/runc/releases/download/v1.1.14/runc.amd64"
RUNC_SHA256="a83c0804ebc16826829e7925626c4793da89a9b225bbcc468f2b338ea9f8e8a8"

cd /app

curl -LO "$CLOUDHYPERVISOR_URL"
sha256sum --check <<<"$CLOUDHYPERVISOR_SHA256 cloud-hypervisor-static"

curl -LO "$RUNC_URL"
sha256sum --check <<<"$RUNC_SHA256 runc.amd64"

curl -LO "$FIRECRACKER_URL"
curl -L "$FIRECRACKER_SHA256_FILE" | sha256sum --check

curl -LO "$CONTAINERD_URL"
curl -L "$CONTAINERD_SHA256_FILE" | sha256sum --check

curl -LO "$CNI_PLUGINS_URL"
curl -L "$CNI_SHA256_FILE" | sha256sum --check

chmod +x cloud-hypervisor-static
mv cloud-hypervisor-static bin/cloud-hypervisor

chmod +x runc.amd64
mv runc.amd64 bin/runc

tar xf firecracker-v1.8.0-x86_64.tgz
mv release-v1.8.0-x86_64/firecracker-v1.8.0-x86_64 bin/firecracker

(mkdir -p containerd && cd containerd && tar xf ../containerd-static-1.7.20-linux-amd64.tar.gz --strip-components=1)

mv containerd/containerd containerd/containerd-shim bin/

(mkdir -p cni && cd cni && tar xf ../cni-plugins-linux-amd64-v1.5.1.tgz)

mv cni/firewall cni/ptp bin/
mv /go/bin/tc-redirect-tap bin/

strip --strip-all bin/*

cp scripts/containerd.sh bin/hypercore-containerd

mkdir hypercore
mv bin/ hypercore/

tar c hypercore | gzip >/hypercore/hypercore.tar.gz
