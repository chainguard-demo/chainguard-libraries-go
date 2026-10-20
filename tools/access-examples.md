# Example commands for Chainguard Libraries for Go access

`chainctl` commands for setting up and managing access to [Chainguard Libraries
for Go](https://edu.chainguard.dev/chainguard/libraries/go/). The pull-token
commands produce the credentials the `go-bank` demo and the configuration in
[`README.md`](README.md) depend on. The entitlement and policy commands are for
administrators setting up an organization.

## Prerequisites

Authenticate to the Chainguard platform first:

```shell
chainctl auth login
```

Creating entitlements and policies requires appropriate permissions on the
organization. Generating a pull token requires an active libraries entitlement.

Most users belong to a single organization, so `chainctl` selects it
automatically or prompts you to choose interactively. Add `--parent=<org>` to
any command that follows only if you belong to more than one organization and
need it to run non-interactively, for example in CI.

## Entitlements

List existing entitlements:

```shell
chainctl libraries entitlements list
```

Create an entitlement for Go:

```shell
chainctl libraries entitlements create --ecosystems=GO
```

`--ecosystems` is required and takes a comma-separated list, for example
`GO,JAVA`. For Go there is no separate upstream-fallback policy to choose at
creation time — the Go entitlement already serves eligible upstream module
versions under Chainguard security controls alongside the Chainguard-remediated
versions.

## Pull tokens

A pull token provides the credentials that authenticate Go to the Chainguard
Libraries for Go repository. The `--output env` form prints shell commands that
set `CHAINGUARD_GO_IDENTITY_ID` and `CHAINGUARD_GO_TOKEN`, the two variables the
`README.md` configuration reads.

Print the export commands:

```shell
chainctl auth pull-token --output env --repository=go
```

Evaluate the output to export both variables into the current shell:

```shell
eval "$(chainctl auth pull-token --output env --repository=go)"
```

`--repository` must be one of `oci`, `apk`, `java`, `python`, `javascript`,
`dotnet`, `ruby`, or `go`. Control validity with `--ttl`, for example
`--ttl=24h`; the maximum is `8760h`, one year.

Write the export commands to a file to source later:

```shell
chainctl auth pull-token --output env --repository=go > go-access.sh
source go-access.sh
```

If you are a member of multiple organizations the preceding example commands
must use the `--parent` parameter with the name of your organization:

```shell
eval "$(chainctl auth pull-token --output env --parent=chainguard.edu --repository=go)"
```

```shell
chainctl auth pull-token --output env --repository=go --parent=chainguard.edu > go-access.sh
```

## Policies

Policies control the cooldown window and package gating for an ecosystem. A
newly created policy is inactive until it is activated for an ecosystem.

Create a policy with no cooldown:

```shell
chainctl libraries policy create --name=no-cooldown --cooldown-days=0
```

Show the policy details:

```shell
chainctl libraries policy describe no-cooldown
```

Activate it for Go in enforcing mode:

```shell
chainctl libraries policy enable --policy=no-cooldown --ecosystem=GO --mode=ENFORCE
```

List policies and their bindings to ecosystems:

```shell
chainctl libraries policy list
chainctl libraries policy bindings list
```
