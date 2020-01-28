#!/bin/sh

make srpm
rhpkg --release rhel-8.2.0 scratch-build --srpm golang-github-osbuild-composer-5-1.fc31.src.rpm
