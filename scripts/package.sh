#!/bin/sh

set -eu

CLOUDHYPERVISOR_URL="https://github.com/cloud-hypervisor/cloud-hypervisor/releases/download/v40.0/cloud-hypervisor-static"
CLOUDHYPERVISOR_SHA256="0010c1dfb81cccae81c3595a4267d226f824d878c86ef06be5dbe63106be4cce"

FIRECRACKER_URL="https://github.com/firecracker-microvm/firecracker/releases/download/v1.8.0/firecracker-v1.8.0-x86_64.tgz"
FIRECRACKER_SHA256_FILE="https://github.com/firecracker-microvm/firecracker/releases/download/v1.8.0/firecracker-v1.8.0-x86_64.tgz.sha256.txt"

CONTAINERD_URL="https://github.com/containerd/containerd/releases/download/v1.7.20/containerd-static-1.7.20-linux-amd64.tar.gz"
CONTAINERD_SHA256_FILE="https://github.com/containerd/containerd/releases/download/v1.7.20/containerd-static-1.7.20-linux-amd64.tar.gz.sha256sum"

docker build -t hypercore:latest .
docker run -v .:/hypercore --rm -i hypercore:latest bash <<EOF
set -euo pipefail

cd /app

curl -LO "$CLOUDHYPERVISOR_URL"
sha256sum --check <<< "$CLOUDHYPERVISOR_SHA256 cloud-hypervisor-static"

curl -LO "$FIRECRACKER_URL"
curl -L "$FIRECRACKER_SHA256_FILE" | sha256sum --check

curl -LO "$CONTAINERD_URL"
curl -L "$CONTAINERD_SHA256_FILE" | sha256sum --check

mv cloud-hypervisor-static bin/

tar xf firecracker-v1.8.0-x86_64.tgz
mv release-v1.8.0-x86_64/firecracker-v1.8.0-x86_64 bin/firecracker

( mkdir -p containerd && cd containerd && tar xf ../containerd-static-1.7.20-linux-amd64.tar.gz --strip-components=1 )

mv containerd/containerd containerd/containerd-shim bin/

strip --strip-all bin/*

mkdir hypercore
mv bin/ hypercore/

tar c hypercore | gzip > /hypercore/hypercore.tar.gz
EOF
