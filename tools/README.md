# Tools and settings for Chainguard Libraries for Go

Configuration and example commands for consuming [Chainguard Libraries for
Go](https://edu.chainguard.dev/chainguard/libraries/go/). Use them to confirm
access from a workstation or CI job, then point a real build at the same proxy.
The [`go-bank`](../go-bank/README.md) demo in this repository shows the same
setup in a working application.

## In this folder

| Resource | Purpose |
|---|---|
| [`access-examples.md`](access-examples.md) | `chainctl` command examples — entitlements, pull tokens, and policies. Start here to set up credentials. See [Authentication](#authentication). |
| `download-single-file-curl.sh` | Download a single module archive with `curl` to confirm direct access. See [Test access](#test-access). |
| `download-dependency-tree-go.sh` | Resolve a module and its full dependency tree with the Go toolchain. See [Test access](#test-access). |

## How Go access works

Chainguard Libraries for Go is an authenticated Go module proxy that implements
the standard `GOPROXY` protocol. You configure Go to resolve modules through it,
and it serves Chainguard-remediated module versions alongside eligible upstream
versions under your organization's policy, malware, and cooldown controls.

Two Go settings and one credential control everything:

* `GOPROXY` — the module proxy endpoint, `https://libraries.cgr.dev/go`.
* `GONOSUMDB='*'` — skip the public checksum database. Go still verifies
  downloads against existing `go.sum` entries. `GOSUMDB=off` has the same
  effect.
* A Chainguard Libraries pull token, supplied through a netrc file, as
  described in [Authentication](#authentication).

Three settings to avoid, each of which silently breaks or bypasses the proxy:

* Do not append `,direct` or a public proxy such as `https://proxy.golang.org`
  to `GOPROXY`. Use the Chainguard endpoint as the only entry so every request
  stays within your organization's controls. Upstream fallback already happens
  on the Chainguard side when it is active for your organization.
* Do not set `GOPRIVATE` for this. It also sets `GONOPROXY`, which makes Go
  bypass the proxy for the matched modules.
* Do not set `GONOSUMCHECK`. It is not a current Go setting and has no effect.

## Authentication

Create a Chainguard Libraries pull token first — see
[`access-examples.md`](access-examples.md) for the full `chainctl` commands,
including entitlements and organization-scoped tokens:

```shell
eval "$(chainctl auth pull-token --output env --repository=go)"
```

This exports `CHAINGUARD_GO_IDENTITY_ID` and `CHAINGUARD_GO_TOKEN` into the
current shell. Go reads proxy credentials from a **netrc file** — it has no
dedicated credential variable like Maven's `settings.xml`, and credentials
cannot go in the `GOPROXY` URL because the Chainguard identity contains a
slash, which breaks URL userinfo parsing. Choose where the netrc file lives
based on whether you want a throwaway check or a persistent setup.

### Scoped — a temporary netrc through the `NETRC` variable

For a throwaway check or one isolated CI job, write the token to a temporary
netrc file and point Go at it with the `NETRC` variable. Nothing touches
`~/.netrc` or your persistent `go env`:

```shell
eval "$(chainctl auth pull-token --output env --repository=go)"
netrc="$(mktemp)"
printf 'machine libraries.cgr.dev\n  login %s\n  password %s\n' \
  "$CHAINGUARD_GO_IDENTITY_ID" "$CHAINGUARD_GO_TOKEN" > "$netrc"
chmod 600 "$netrc"
NETRC="$netrc" GOPROXY=https://libraries.cgr.dev/go GONOSUMDB='*' go mod download
rm -f "$netrc"
```

Keeping the token in a file rather than on the command line also keeps it out
of your shell history and the process table.

### Persistent — `~/.netrc` and `go env -w`

For ongoing work, store the credentials in `~/.netrc` and set the proxy once.
This is the setup the [`go-bank`](../go-bank/README.md) demo documents. Add an
entry, keeping any entries for other hosts:

```text
machine libraries.cgr.dev
  login YOUR_IDENTITY_ID
  password YOUR_TOKEN
```

Restrict the file and point Go at the proxy:

```shell
chmod 600 ~/.netrc
go env -w GOPROXY=https://libraries.cgr.dev/go
go env -w GONOSUMDB='*'
```

Use `~/.netrc` for anything persistent, and the temporary `NETRC` file for
one-off checks. Both keep the token out of your shell history and the process
table.

## Test access

Run these checks once a pull token is exported. They confirm the proxy serves
module bytes to your credentials before you point a real build at it. Two
scripts in this folder package the checks, each with a default module and an
optional argument to override it:

```shell
eval "$(chainctl auth pull-token --output env --repository=go)"

# Download a single module archive with curl.
./download-single-file-curl.sh
./download-single-file-curl.sh golang.org/x/text/@v/v0.3.0.zip

# Resolve a module and its full dependency tree with the Go toolchain,
# into a throwaway module cache.
./download-dependency-tree-go.sh
./download-dependency-tree-go.sh github.com/google/uuid@v1.6.0
```

The following sections explain the same two checks with raw commands, if you
prefer to run them directly rather than through the scripts.

Fetch a single module archive with `curl` and checksum it to confirm you
received real content:

```shell
eval "$(chainctl auth pull-token --output env --repository=go)"
curl -fsS -L -u "$CHAINGUARD_GO_IDENTITY_ID:$CHAINGUARD_GO_TOKEN" \
  "https://libraries.cgr.dev/go/rsc.io/quote/@v/v1.5.2.zip" | shasum -a 256
```

On Linux, use `sha256sum` in place of `shasum -a 256`.

Resolve a module with the Go toolchain into a throwaway cache, so the check
cannot be served from an existing local cache:

```shell
eval "$(chainctl auth pull-token --output env --repository=go)"
netrc="$(mktemp)"
printf 'machine libraries.cgr.dev\n  login %s\n  password %s\n' \
  "$CHAINGUARD_GO_IDENTITY_ID" "$CHAINGUARD_GO_TOKEN" > "$netrc"
chmod 600 "$netrc"
NETRC="$netrc" GOMODCACHE="$(mktemp -d)" GONOPROXY=none \
  GOPROXY=https://libraries.cgr.dev/go GONOSUMDB='*' \
  go mod download github.com/buger/jsonparser@v1.1.2-cgr.1
rm -f "$netrc"
```

A `401 Unauthorized` means the token is missing or expired — create a fresh
one. A `403 Forbidden` means the token is valid but your organization is not
cleared to pull that module. The `go-bank` demo's
[`scripts/verify-proxy.sh`](../go-bank/scripts/verify-proxy.sh) runs the same
kind of clean-cache check for a full project build.

## Remediated versions

Chainguard-remediated module versions carry a `-cgr.N` suffix, for example
`github.com/buger/jsonparser v1.1.2-cgr.1`. They are served only by the
Chainguard proxy, not by the public Go proxy. The `go-bank` demo pins that
version to remediate CVE-2026-32285 — see its
[demo steps](../go-bank/demo/readme.md) for the before-and-after comparison.

## Direct access or a repository manager

The preceding configuration is direct access — every workstation and CI job
authenticates to `libraries.cgr.dev/go` with a pull token. That suits
evaluation, small teams, and isolated CI jobs, and it needs per-workstation
setup.

For organization-wide use, proxy `https://libraries.cgr.dev/go/` through a
repository manager such as JFrog Artifactory or Sonatype Nexus instead.
Configure one Go remote repository that points at the Chainguard endpoint with
the pull-token credentials. The manager then handles local caching and access
control, and your Go toolchains point at the manager rather than at Chainguard
directly. Chainguard does not offer an uptime SLA for `libraries.cgr.dev`, so a
repository manager is the recommended path for production. Refer to the
[Chainguard Libraries for Go
documentation](https://edu.chainguard.dev/chainguard/libraries/go/) for the
repository manager setup.
