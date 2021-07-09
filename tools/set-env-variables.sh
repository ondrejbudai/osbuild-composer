#!/bin/bash

source /etc/os-release
ARCH=$(uname -m)

# TARGET_DISTRO_CODE defines which distro the testsuite builds images
# for. This is useful mainly for cross-building.
TARGET_DISTRO_CODE="${TARGET_DISTRO_CODE:-${ID}-${VERSION_ID//./}}"
