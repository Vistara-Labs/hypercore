#!/bin/bash

set -euo pipefail

export PATH="/opt/hypercore/bin:$PATH"

HYPERCORE_BASE="/var/lib/hypercore"

CONTAINERD_ROOT="$HYPERCORE_BASE/containerd"
CONTAINERD_SOCK="$HYPERCORE_BASE/containerd.sock"

# State is ephemeral
CONTAINERD_STATE="/run/hypercore/containerd"

POOL_NAME="hypercore-dev-thinpool"
DEVMAPPER_ROOT_PATH="$HYPERCORE_BASE/devmapper"

mkdir -p "$DEVMAPPER_ROOT_PATH"

if [ ! -f "$DEVMAPPER_ROOT_PATH/data" ]; then
	: >"$DEVMAPPER_ROOT_PATH/data"
	truncate -s 512MB "$DEVMAPPER_ROOT_PATH/data"
fi

if [ ! -f "$DEVMAPPER_ROOT_PATH/metadata" ]; then
	: >"$DEVMAPPER_ROOT_PATH/metadata"
	truncate -s 1G "$DEVMAPPER_ROOT_PATH/metadata"
fi

find_dev() {
	_dev="$(losetup --output NAME --noheadings --associated "$1")"
	if [ -z "$_dev" ]; then
		_dev="$(losetup --find --show "$1")"
	fi
	echo "$_dev"
}

DATADEV="$(find_dev "$DEVMAPPER_ROOT_PATH/data")"
METADEV="$(find_dev "$DEVMAPPER_ROOT_PATH/metadata")"

SECTORSIZE=512
DATASIZE="$(blockdev --getsize64 -q "$DATADEV")"
LENGTH_SECTORS="$(echo "$DATASIZE/$SECTORSIZE" | bc)"
DATA_BLOCK_SIZE=128
LOW_WATER_MARK=32768
THINP_TABLE="0 $LENGTH_SECTORS thin-pool $METADEV $DATADEV $DATA_BLOCK_SIZE $LOW_WATER_MARK 1 skip_block_zeroing"

dmsetup reload "$POOL_NAME" --table "$THINP_TABLE" ||
	dmsetup create "$POOL_NAME" --table "$THINP_TABLE"

exec containerd --root "$CONTAINERD_ROOT" --state "$CONTAINERD_STATE" --config /dev/stdin <<EOF
version = 2

[grpc]
address = "$CONTAINERD_SOCK"

[plugins]
  [plugins.'io.containerd.snapshotter.v1.devmapper']
    pool_name = "$POOL_NAME"
    base_image_size = "512MB"
    root_path = "$DEVMAPPER_ROOT_PATH"
EOF
