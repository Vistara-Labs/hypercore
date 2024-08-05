#!/bin/sh

set -eu

HYPERCORE_BASE="/var/lib/hypercore"

CONTAINERD_ROOT="$HYPERCORE_BASE/containerd"
CONTAINERD_SOCK="$HYPERCORE_BASE/containerd.sock"

# State is ephemeral
CONTAINERD_STATE="/run/hypercore/containerd"

SNAPSHOTTER_ROOT="$HYPERCORE_BASE/snapshotter"
SNAPSHOTTER_SCRATCH="$HYPERCORE_BASE/blockfile"

mkdir -p "$HYPERCORE_BASE"

if [ ! -f "$SNAPSHOTTER_SCRATCH" ]; then
	dd if=/dev/zero of="$SNAPSHOTTER_SCRATCH" bs=1M count=500
	mkfs.ext4 "$SNAPSHOTTER_SCRATCH"
fi

exec containerd --root "$CONTAINERD_ROOT" --state "$CONTAINERD_STATE" --config /dev/stdin <<EOF
version = 2

[grpc]
address = "$CONTAINERD_SOCK"

[plugins]
  [plugins.'io.containerd.snapshotter.v1.blockfile']
    scratch_file = "$SNAPSHOTTER_SCRATCH"
    root_path = "$SNAPSHOTTER_ROOT"
    fs_type = 'ext4'
    mount_options = []
    recreate_scratch = true
EOF
