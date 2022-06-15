#!/usr/bin/bash

awk '{print "Provides: bundled(golang("$1")) = "$2}' go.mod | sort --ignore-case | uniq | sed -e 's/-/_/g' -e '/bundled(golang())/d' -e '/bundled(golang(go\|module\|replace\|require))/d' >bundled-provides.spec
