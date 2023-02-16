#!/bin/bash

# USAGE: ./feng-file.sh <path>

# Scans input path and reports recognized file types.

IFS=$'\n'; set -f
for f in $(find $1 -type f -name '*'); do go run cmd/feng/main.go --brief $f; done
unset IFS; set +f
