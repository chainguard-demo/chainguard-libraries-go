#!/bin/sh
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$project_dir"
go mod download
go mod verify
mkdir -p bin
go build -trimpath -o bin/chainguard-bank .
exec ./bin/chainguard-bank
