#!/bin/sh
# Download modules from Chainguard into an empty temporary cache.
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"
proxy=$(go env GOPROXY)
if [ "$proxy" != 'https://libraries.cgr.dev/go' ]; then
    printf '%s\n' 'Expected GOPROXY=https://libraries.cgr.dev/go. Check go env GOPROXY and shell overrides.' >&2
    exit 1
fi
artifact_dir=artifacts
module_cache=$(mktemp -d "${TMPDIR:-/tmp}/chainguard-go-modules.XXXXXX")
trap 'chmod -R u+w "$module_cache"; rm -rf "$module_cache"' EXIT
export GOMODCACHE="$module_cache"
# Never bypass the proxy for modules matched by a global GOPRIVATE/GONOPROXY.
export GONOPROXY=none
mkdir -p "$artifact_dir"
printf 'Downloading modules through %s into an empty cache...\n' "$proxy"
go mod download -json > "$artifact_dir/module-downloads.json"
go test ./...
go mod verify
go build -trimpath -o "$artifact_dir/chainguard-bank" .
go version -m "$artifact_dir/chainguard-bank" > "$artifact_dir/build-modules.txt"
printf '%s\n' "$proxy" > "$artifact_dir/proxy.txt"
printf '%s\n' 'Proxy download, tests, checksum verification, and build succeeded.' "Evidence: $artifact_dir/module-downloads.json and $artifact_dir/build-modules.txt"
