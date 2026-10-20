#!/usr/bin/env bash
#
# Downloads a single module archive from Chainguard Libraries for Go with curl
# to confirm direct access to the proxy at libraries.cgr.dev/go.
#
# Credentials come from a Chainguard Libraries pull token. Export them first
# (see README.md):
#
#   eval "$(chainctl auth pull-token --output env --repository=go)"
#
# This sets CHAINGUARD_GO_IDENTITY_ID and CHAINGUARD_GO_TOKEN.
#
# Usage: download-single-file-curl.sh [MODULE_PATH] [OUTPUT_FILE]
#   MODULE_PATH  Go module proxy path, relative to the go context, in the form
#                <module>/@v/<version>.zip. Defaults to
#                rsc.io/quote/@v/v1.5.2.zip
#   OUTPUT_FILE  Local file to write. Defaults to the archive file name.

set -euo pipefail

BASE_URL="https://libraries.cgr.dev/go"
MODULE_PATH="${1:-rsc.io/quote/@v/v1.5.2.zip}"
OUTPUT_FILE="${2:-$(basename "$MODULE_PATH")}"

if [[ -z "${CHAINGUARD_GO_IDENTITY_ID:-}" || -z "${CHAINGUARD_GO_TOKEN:-}" ]]; then
  echo "Error: CHAINGUARD_GO_IDENTITY_ID and CHAINGUARD_GO_TOKEN must be set." >&2
  echo "Export them with a Chainguard Libraries pull token:" >&2
  echo "  eval \"\$(chainctl auth pull-token --output env --repository=go)\"" >&2
  exit 1
fi

echo "Downloading: $BASE_URL/$MODULE_PATH"
echo "Writing to:  $OUTPUT_FILE"
echo "Pass a different module path (and optional output file) as arguments, for example"
echo "  $0 golang.org/x/text/@v/v0.3.0.zip"

# -L follows the redirect the proxy issues to its backing object storage;
# without it curl saves the redirect response instead of the archive.
curl -f -S -L -X GET \
  "$BASE_URL/$MODULE_PATH" \
  -u "$CHAINGUARD_GO_IDENTITY_ID:$CHAINGUARD_GO_TOKEN" \
  --output "$OUTPUT_FILE"

echo "Download successful. Archive saved to '$OUTPUT_FILE'."
