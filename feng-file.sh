#!/bin/bash

# USAGE: ./feng-file.sh <path>

# Scans input path and reports recognized file types.

IFS=$'\n'; set -f

go install ./cmd/feng

for f in $(find $1 -type f -name '*'); do feng --brief $f; done
unset IFS; set +f
