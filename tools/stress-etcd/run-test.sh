#!/bin/bash

set -eu

# Please note that "host.docker.internal" doesn't work on docker-for-linux
SERVER=host.docker.internal:2379

time docker run --rm \
    -e ETCD_SERVER=${SERVER}\
    -e KEY_COUNT=100 \
    -e KEY_SIZE=1000 \
    -v ${PWD}/ssl:/ssl \
    stress-etcd
