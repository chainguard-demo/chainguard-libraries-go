# Chainguard Go Bank

A single-page banking demo served by a Go web server. It follows the Chainguard
branding, colors, header navigation, cards, and tab layout of the
[Java](https://github.com/chainguard-demo/ynad/tree/main/java-libraries-demo)
and [JavaScript](https://github.com/chainguard-demo/ynad/tree/main/javascript-libraries-demo)
libraries demos, with a banking overview as its main screen.

The app uses your configured Go proxy to download modules. Transfer requests use
`github.com/buger/jsonparser` at version `v1.1.2-cgr.1`. The
[demo steps](demo/readme.md) explain how to compare this patched version with
the vulnerable `v1.1.1` version.

## Run

Complete [Go setup](#go-setup) first. Then run these commands from the
repository root:

```sh
cd go-bank
./build-and-run.sh
```

Open **http://localhost:5003**. Stop with Ctrl+C. To change the port:

```sh
PORT=8080 ./build-and-run.sh
```

The script downloads dependencies, verifies module checksums, builds
`bin/chainguard-bank`, and starts it. HTML, CSS, JavaScript, images, and
`go.mod` are embedded in the binary; there is no Node.js or frontend build
dependency. After building, `./bin/chainguard-bank` runs without Go, repository
files, or registry credentials.

The server binds to `127.0.0.1` by default. `HOST` overrides the bind address if
needed. This is an unauthenticated local demo with one shared fictitious
customer, not a production banking service. All clients see the same in-memory
data. Restarting resets balances and transactions. Transfers retain the latest
100 transactions; seeded activity is illustrative recent history, not a complete
ledger.

## Go setup

Run the commands in this section from the `go-bank` directory.

This project uses standard `go` commands and your user-wide Go settings.
`go.mod` requires Go **1.26.8**. Go 1.21 or later can download the required
toolchain when automatic toolchain selection is enabled.

1. Get a Chainguard Go pull token. Use its username and password in the next
   step.

   ```sh
   chainctl auth pull-token --repository=go
   ```

2. Add the credentials to `~/.netrc`:

   ```text
   machine libraries.cgr.dev
     login YOUR_USERNAME
     password YOUR_PASSWORD
   ```

   Keep any entries for other hosts. Restrict access to the file:

   ```sh
   chmod 600 ~/.netrc
   ```

3. Set the Go proxy and checksum database policy:

   ```sh
   go env -w GOPROXY=https://libraries.cgr.dev/go
   go env -w GONOSUMDB='*'
   ```

   These settings apply across projects for your user. `GONOSUMDB='*'` skips the
   public checksum database for modules. Go still checks module downloads
   against existing `go.sum` entries.

4. Check the effective settings:

   ```sh
   go env GOPROXY GONOSUMDB
   ```

5. Run the clean-cache check:

   ```sh
   ./scripts/verify-proxy.sh
   ```

Go uses `~/.netrc` for authentication by default. In CI, create this file from
the CI secret store. Shell environment variables can override the saved Go
settings. `GOENV` and `NETRC` can select different configuration and credential
files. The project does not set either variable.

The proxy points directly to Chainguard. It has no public proxy fallback. An old
project-local `.netrc`, if present, is unused and remains ignored by Git.

## What to demo

1. View checking, savings, and combined balances.
2. Transfer a small amount from checking to savings. Both balances and recent
   activity update from the Go API.
3. Search transactions or filter to an account. Incoming transfers appear as
   credits when viewing their destination account.
4. Open **Go dependencies** to inspect module versions and checksums from the
   running binary. Check that `jsonparser` shows `v1.1.2-cgr.1` for the default
   build.
5. Open **go.mod** to inspect or copy the build's embedded manifest.

The Java and JavaScript header links point to the separate reference demos on
ports 5001 and 5002; those applications must be running independently.

## Dependencies used by the app

| Module | Version | Actual use |
| --- | --- | --- |
| `github.com/buger/jsonparser` | `v1.1.2-cgr.1` | Reads transfer request fields and redacts credentials from the audit log |
| `github.com/go-chi/chi/v5` | `v5.3.2` | HTTP routes and middleware |
| `github.com/google/uuid` | `v1.6.0` | Transfer identifiers |
| `github.com/shopspring/decimal` | `v1.4.0` | Exact balance and transfer arithmetic |

Amounts cross the API as decimal strings. The server rejects nonpositive
amounts, more than two decimal places, insufficient funds, unknown accounts, and
transfers to the same account. A mutex makes balance changes and ledger updates
atomic across concurrent requests.

### JSON parser version

This project pins `github.com/buger/jsonparser` to the patched Chainguard
version `v1.1.2-cgr.1` in `go.mod`. It records module checksums in `go.sum`.

[CVE-2026-32285](https://pkg.go.dev/vuln/GO-2026-4514) affects versions before
`v1.1.2`, including `v1.1.1`. The [demo steps](demo/readme.md) select `v1.1.1`
to reproduce the vulnerability, then select `v1.1.2-cgr.1` to show the fix.

Stop any running instance before you run the build script. The script builds and
starts the updated application.

Every `POST /api/transfers` request uses `jsonparser.ObjectEach` to read
transfer fields and `jsonparser.ParseString` to decode string values. The
standard Go JSON decoder checks syntax and rejects extra JSON values. The
request size limit, unknown-field checks, string types, and transfer rules still
apply. JSON responses still use the standard Go encoder.

The handler also records a redacted audit copy of each request. Before the
standard decoder runs, it reads the raw body and, on a fire-and-forget
goroutine, calls `jsonparser.Delete(body, "secret", "pin")` to strip any
client-supplied credential before logging. Because this runs on the raw bytes
(not the decoded struct) and outside the chi `Recoverer` goroutine, a panic
inside `Delete` crashes the whole process. This is the code path
`CVE-2026-32285` exploits.

### Exploit demonstration

```sh
./exploit.sh
```

Start the bank in one terminal. Run `./exploit.sh` in a second terminal from the
`go-bank` directory. The script sends one malformed transfer request to the
running server and checks its health.

- With `v1.1.1`, a request with a truncated `secret` value causes a panic in
  `jsonparser.Delete`. The server process crashes.
- With `v1.1.2-cgr.1`, the server rejects the request and stays running.

The script targets `127.0.0.1:5003` by default. Use `PORT` to match a different
local server port. It does not build the application or change `go.mod` or
`go.sum`.

To check the module in the built application:

```sh
go version -m bin/chainguard-bank
```

The default build must include `github.com/buger/jsonparser v1.1.2-cgr.1`. The
module version and checksums identify the dependency.

Scan the project directory with Grype. Use `--by-cve` to display CVE
identifiers:

```sh
grype . --by-cve
```

## Prove a fresh proxy build

```sh
./scripts/verify-proxy.sh
```

This command checks the effective proxy and creates an empty temporary module
cache. It prevents `GONOPROXY` and `GOPRIVATE` from bypassing the proxy during
the check. It then downloads modules, runs tests, verifies checksums, and builds
the application. It removes the temporary cache after the check. It does not
clear your shared module cache.

The check writes these ignored local files:

- `artifacts/module-downloads.json`: module download information and checksums.
  The temporary cache paths no longer exist after the check.
- `artifacts/build-modules.txt`: module information from the built executable.
- `artifacts/proxy.txt`: the tested proxy endpoint.
- `artifacts/chainguard-bank`: the executable from the clean-cache build.

Successful downloads show that the configured proxy served the build inputs.
Module checksums and build information do not prove who built the modules. The
application displays module information from its running binary. It does not
report workstation settings as proof of the build's download source.

## Development and checks

```sh
go run .
go test -race ./...
go vet ./...
```

Rebuild/restart after editing frontend files because assets are embedded at
compile time. Tests exercise exact arithmetic, rejected transfers without
mutation, concurrent overdraft prevention, independent snapshots, API
validation, and static-file boundaries.

| Endpoint | Purpose |
| --- | --- |
| `GET /health` | Health check |
| `GET /api/dashboard` | Balances and recent transactions |
| `POST /api/transfers` | Transfer demo funds |
| `GET /api/dependencies` | Running binary's module versions/checksums |
| `GET /api/go-mod` | Embedded build manifest |

Example transfer:

```sh
curl http://localhost:5003/api/transfers \
  -H 'Content-Type: application/json' \
  -d '{"from":"checking","to":"savings","amount":"25.00","memo":"Weekend savings"}'
```

## Reference assets

The three SVG files in `web/images/` are copied from the JavaScript reference
demo at commit `98affe28a3f7b1bc0f09cbccb726edef61372176`. Branding belongs to
its respective owners. The banking UI and Go implementation are local to this
project.
