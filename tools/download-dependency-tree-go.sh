#!/usr/bin/env bash
#
# Resolves a module and its full transitive dependency tree through Chainguard
# Libraries for Go to confirm access. It builds a throwaway module in a
# temporary directory, downloads into a temporary module cache, and prints the
# resolved module graph. It changes no files in this repository and leaves your
# shared Go module cache and your global go env untouched.
#
# Credentials come from a Chainguard Libraries pull token. Export them first
# (see README.md):
#
#   eval "$(chainctl auth pull-token --output env --repository=go)"
#
# This sets CHAINGUARD_GO_IDENTITY_ID and CHAINGUARD_GO_TOKEN, which the script
# writes to a temporary netrc file for the download. Go reads proxy credentials
# from netrc, not from the GOPROXY URL — the Chainguard identity contains a
# slash, which breaks URL userinfo parsing. Your ~/.netrc is left untouched.
#
# Usage: download-dependency-tree-go.sh [MODULE]
#   MODULE  Module path and version, module@version. Defaults to
#           rsc.io/quote@v1.5.2.

set -euo pipefail

MODULE="${1:-rsc.io/quote@v1.5.2}"

if [[ -z "${CHAINGUARD_GO_IDENTITY_ID:-}" || -z "${CHAINGUARD_GO_TOKEN:-}" ]]; then
  echo "Error: CHAINGUARD_GO_IDENTITY_ID and CHAINGUARD_GO_TOKEN must be set." >&2
  echo "Export them with a Chainguard Libraries pull token:" >&2
  echo "  eval \"\$(chainctl auth pull-token --output env --repository=go)\"" >&2
  exit 1
fi

echo "Resolving module: $MODULE"
echo "Pass a different module@version as the first argument to change this, for example"
echo "  $0 github.com/google/uuid@v1.6.0"

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/chainguard-go-tree.XXXXXX")"
module_cache="$(mktemp -d "${TMPDIR:-/tmp}/chainguard-go-cache.XXXXXX")"
# Module cache files are written read-only; make them writable before removal.
trap 'chmod -R u+w "$module_cache" 2>/dev/null || true; rm -rf "$work_dir" "$module_cache"' EXIT

# Write the credentials to a temporary netrc and point Go at it with NETRC, so
# the token never lands in the GOPROXY URL, the process arguments, or ~/.netrc.
netrc_file="$work_dir/netrc"
( umask 077; printf 'machine libraries.cgr.dev\n  login %s\n  password %s\n' \
  "$CHAINGUARD_GO_IDENTITY_ID" "$CHAINGUARD_GO_TOKEN" > "$netrc_file" )

export NETRC="$netrc_file"
export GOMODCACHE="$module_cache"
export GOPROXY="https://libraries.cgr.dev/go"
export GONOSUMDB='*'
# Never bypass the proxy for modules matched by a global GOPRIVATE or GONOPROXY.
export GONOPROXY=none

cd "$work_dir"
go mod init chainguard.example/access-check >/dev/null

echo "Downloading $MODULE and its dependencies through $GOPROXY ..."
go get "$MODULE"

echo
echo "Resolved module graph:"
go list -m all

echo
echo "Dependency tree resolved successfully through Chainguard Libraries for Go."
