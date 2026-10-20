# Go demo steps

Run commands from the `go-bank` directory. From the repository root, run
`cd go-bank` first. Use terminal 1 for setup and the server. Use terminal 2 for
scans and exploit checks. Stop the server with Ctrl+C before each rebuild if it
is still running.

Go settings apply to all projects for your user. Shell environment variables
can override them. The cache cleanup commands clear shared Go caches.

As an alternative to changing your user-wide Go settings, run every command in
this demo with the proxy configuration set inline, through a temporary netrc
file and the `NETRC`, `GOPROXY`, and `GONOSUMDB` variables. This leaves
`~/.netrc` and your saved `go env` untouched. The scoped setup is documented in
the [tools README](../../tools/README.md#authentication).

To return to the committed patched state at any point, restore the manifest
from Git and rebuild, rather than running `go clean -modcache -cache`, which
clears the shared Go caches for all your projects:

```sh
git checkout -- go.mod go.sum
./build-and-run.sh
```

## 1. Set up the initial state

In terminal 1, reset the saved proxy settings and select the vulnerable version:

```sh
go env -u GOPROXY
go env -u GONOSUMDB
go clean -modcache -cache

go mod edit -require=github.com/buger/jsonparser@v1.1.1
go mod tidy

# Check that download URLs use the upstream Go proxy.
GOMODCACHE="$(mktemp -d)" go mod download -x

./build-and-run.sh
```

Open http://localhost:5003. In terminal 2, scan the project and run the exploit:

```sh
grype . --by-cve
./exploit.sh
```

The scan reports the vulnerable dependency:

```text
NAME                         INSTALLED  FIXED IN  TYPE       VULNERABILITY   SEVERITY
github.com/buger/jsonparser  v1.1.1     1.1.2     go-module  CVE-2026-32285  High
```

The exploit sends a malformed request to the running server. With `v1.1.1` the
server crashes, and the exploit reports a `VULNERABLE` verdict.

## 2. Set up the Chainguard state

In terminal 1, create a Go pull token and edit the credentials file:

```sh
chainctl auth pull-token create --repository=go
vi ~/.netrc
```

Add the returned username and password. Keep entries for other hosts:

```text
machine libraries.cgr.dev
  login YOUR_USERNAME
  password YOUR_PASSWORD
```

Configure Go to download through Chainguard, then rebuild:

```sh
chmod 600 ~/.netrc
go env -w GOPROXY=https://libraries.cgr.dev/go
go env -w GONOSUMDB='*'

# Check that download URLs use libraries.cgr.dev/go.
GOMODCACHE="$(mktemp -d)" go mod download -x

go clean -modcache
./build-and-run.sh # Download, verify, build, and run.
```

`GONOSUMDB='*'` skips the public checksum database. Go still checks existing
`go.sum` entries. The dependency remains at `v1.1.1` until the next stage.

In terminal 2, try to download a package that the policy denies:

```sh
go mod download -x gocommunity.io/orderedbtree@v1.3.1
```

The proxy rejects the request with an HTTP 403 response:

```text
go: gocommunity.io/orderedbtree@v1.3.1: reading https://libraries.cgr.dev/go/gocommunity.io/orderedbtree/@v/v1.3.1.info: 403 Forbidden
```

## 3. Remediate CVE-2026-32285

Stop the server in terminal 1. Select the patched Chainguard version and
rebuild:

```sh
go get github.com/buger/jsonparser@v1.1.2-cgr.1
go mod tidy

./build-and-run.sh
```

In terminal 2, scan the project and run the exploit again:

```sh
grype . --by-cve
./exploit.sh
```

The scan no longer reports CVE-2026-32285 for `jsonparser`:

```text
No vulnerabilities found
```

The exploit reports a `PATCHED` verdict, and the server stays running after the
exploit request with `v1.1.2-cgr.1`.
