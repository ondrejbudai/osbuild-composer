#!/bin/bash

source /etc/os-release
ARCH=$(uname -m)
DISTRO_CODE="${DISTRO_CODE:-${ID}_${VERSION_ID//./}}"
