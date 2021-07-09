#!/bin/bash

source /etc/os-release
ARCH=$(uname -m)

# TARGET_DISTRO_CODE defines which distro the testsuite builds images
# for. This is useful mainly for cross-building.
TARGET_DISTRO_CODE="${TARGET_DISTRO_CODE:-${ID}-${VERSION_ID//./}}"

TARGET_DISTRO_ID="${TARGET_DISTRO_CODE%-*}"
TARGET_DISTRO_VERSION_ID="${TARGET_DISTRO_CODE#*-}"

# 84 => 8.4
if [[ "$TARGET_DISTRO_ID" == rhel ]]; then
  TARGET_DISTRO_VERSION_ID="${TARGET_DISTRO_VERSION_ID:0:1}.${TARGET_DISTRO_VERSION_ID:1:2}"
fi
