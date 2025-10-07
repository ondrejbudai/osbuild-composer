#!/bin/bash

set -euo pipefail

# --jobs 0 to run things in parallel
podman build -t worker-poc --jobs 0 .
mkdir -p output
podman run \
    --rm \
    -it \
    --privileged \
    --pull=newer \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    registry.redhat.io/rhel9/bootc-image-builder:9.6 \
    --type raw \
    localhost/worker-poc
