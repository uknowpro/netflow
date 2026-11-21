# Detailed guide

[Quick start](../README.md) · [한국어 가이드](GUIDE.ko.md)

## Why netflow?

- **One CLI, multiple locations:** use a VM, a standalone container, or a Kubernetes Pod.
- **Explicit direction:** describe inbound and outbound flows relative to the system under test.
- **Layered evidence:** separate DNS resolution, TCP connection, TLS handshake, and HTTP expectations.
- **Automation friendly:** strict YAML input, bounded concurrency and retries, JSON output, and meaningful exit codes.
- **Small deployment footprint:** a Go executable; no controller, cluster API access, or privileged networking required.

## AKS and VM control-point checks

Environment-specific templates: [AKS](../examples/aks-flows.yaml), [Azure VM](../examples/azure-vm-flows.yaml), and [on-premises VM](../examples/onprem-vm-flows.yaml). These are illustrative paths, not discovered topology. Replace every endpoint, control-point name, and ordering with your actual design. Inbound/outbound is relative to the target AKS system.

```bash
# Run each command on its actual source; use a build containing these new fields.
netflow test --config examples/aks-flows.yaml --vantage aks --output json
netflow test --config examples/azure-vm-flows.yaml --vantage azure-vm
netflow test --config examples/onprem-vm-flows.yaml --vantage onprem-vm
```

`source_environment` optionally declares `aks` or `vm`; it does not detect or enforce the runtime. `vantage` remains the execution-label guard. `expected_path` is an ordered list of `{type, name}` declarations (maximum 32; names 1–200 bytes). Supported types: `external_firewall`, `external_waf`, `nsg`, `application_gateway`, `on_prem_firewall`, `network_policy`, `other`. These optional fields preserve existing version-1 configurations. These fields require v0.3.0 or later; older binaries reject the new keys.

Terminal output explicitly marks the expected path as operator-declared and unverified. JSON retains declarations under `results[].flow`; `assessment.expected_path_verification` is `NOT_TESTED` whenever a path is declared, including PASS and SKIPPED results. `flow.expect.status` records explicit HTTP expectations (an empty object means the default 200–299 range). The expected and observed HTTP statuses also appear in terminal output. Declared hops never become measured stages or per-device PASS results.

Use the real service FQDN for WAF/Application Gateway checks so HTTP Host and TLS SNI identify the intended service. The runtime DNS resolver determines the addresses, including private/split DNS. A TCP PASS proves one resolved address accepted a connection; it does not prove all backends work. An HTTP PASS proves only the configured response expectation: accepting 403 explicitly does not establish application access or authentication. Neither 403 nor a timeout identifies the blocking appliance.

For an AKS Job, mount the selected file as `network-flows.yaml` using the existing Job/ConfigMap example and change its `--vantage` and `--group` to `aks`. Match the application's namespace, labels, identity, DNS, node/network and service-mesh conditions. On VMs, prefer the native executable when testing the host's path. A temporary TCP receiver does not validate a WAF's HTTP path; use a real authorized HTTP(S) health endpoint.

For incident correlation, retain the report's UTC start time, flow ID, local/remote socket addresses and HTTP status, then compare them with the relevant firewall/WAF/NSG/gateway logs. NAT and proxies may change addresses; netflow does not query these logs, discover hops, inspect rules, or attribute failures to a particular appliance. Real AKS, VM and on-premises paths remain unverified until you run these checks there.


## How inbound and outbound work

```text
                         System under test
                         ┌───────────────────┐
External VM / Pod        │ New AKS            │       External service
[netflow] ── inbound ──► │ Service / Ingress   │
                         │ Pod [netflow] ─────┼─────► Database / API
                         └───────────────────┘       outbound
```

Both directions use the same connection engine. **Execution location determines the path.** An AKS Pod cannot reproduce an external inbound path by labeling a flow `inbound`. Run inbound tests at the real permitted source, targeting the actual AKS entry point.

`vantage` is an operator-supplied execution label, not a source IP binding or an attestation of location. NAT can change the address observed by a firewall. Reported local socket addresses are pre-NAT addresses.

## Supported environments

| Environment | Release binaries / image | Verification |
| --- | --- | --- |
| Linux | amd64, arm64, armv7, 386 | x64/ARM64 native CI; ARMv7/386 cross-build only |
| Windows | amd64, arm64 (`.exe` in ZIP) | x64/ARM64 native CI |
| macOS | Intel amd64, Apple Silicon arm64 (`darwin` archives) | Intel/Apple Silicon native CI |
| FreeBSD | amd64, arm64 | Experimental: cross-build only |
| Docker / Podman / Kubernetes | Linux amd64, arm64 container | Image build and Linux amd64 smoke test; cluster paths unverified |

VMs, physical hosts, WSL2, CI agents, and Kubernetes distributions can run the matching executable or Linux container. AKS, EKS, GKE, on-premises Kubernetes, and individual Linux distributions have **not** been certified through environment-specific network tests. Native Windows containers are not provided. See [platform support and installation](SUPPORT.md) for Windows PowerShell/macOS instructions, runtime requirements, and validation boundaries.

## Quick start

Requires Go 1.26 or later to build. Commands below assume a shell on Linux; the CLI does not require Kubernetes when running on a VM.

```bash
git clone https://github.com/uknowpro/netflow.git
cd netflow
go build -trimpath -o bin/netflow ./cmd/netflow

./bin/netflow --help
./bin/netflow probe --host 127.0.0.1 --protocol dns
./bin/netflow probe --host db.internal.example --port 5432 --protocol tcp
./bin/netflow probe --protocol https --url https://api.example.com/health
```

Example endpoints are placeholders. Replace them with your own authorized destinations. Tagged releases are built automatically; availability depends on a successful release workflow. See [Releases](https://github.com/uknowpro/netflow/releases) for published versions.

## Define network flows

Copy [examples/network-flows.yaml](../examples/network-flows.yaml) and replace the destinations:

```yaml
version: 1

defaults:
  timeout: 5s
  retries: 1

flows:
  - id: database-outbound
    group: FW-GROUP-001
    direction: outbound
    vantage: aks-app
    protocol: tcp
    host: db.internal.example
    port: 5432

  - id: api-inbound
    group: FW-GROUP-002
    direction: inbound
    vantage: external-vm
    protocol: https
    url: https://api.example.com/health
    expect:
      status: [200]
```

Run these commands at their respective source locations:

```bash
# Inside the application network in AKS
netflow test --config network-flows.yaml --vantage aks-app --group FW-GROUP-001

# On the external VM allowed to access AKS
netflow test --config network-flows.yaml --vantage external-vm --group FW-GROUP-002

# Save a machine-readable report
netflow test --config network-flows.yaml --vantage aks-app \
  --group FW-GROUP-001 --output json > result.json
```

Group selection happens before execution. Flows in other groups are omitted. Within the selected group, mismatched vantage labels produce `SKIPPED` without sending traffic. A flow without a vantage label runs at any location. If you omit `--group`, every flow is considered, including those that will be skipped.

### Configuration reference

| Field | Behavior |
| --- | --- |
| `version` | Required; currently `1` |
| `defaults.timeout` | Total deadline per attempt across all stages; default `5s`, maximum `5m` |
| `defaults.retries` | Additional attempts after failure; default `0`, range `0–5`; fixed 200 ms pause |
| `flows[].id` | Required, unique flow identifier |
| `group` | Optional firewall request or business grouping |
| `direction` | Optional `inbound` or `outbound` metadata; does not change routing |
| `vantage` | Optional execution label that must match `--vantage` |
| `protocol` | Required: `dns`, `tcp`, `tls`, `http`, or `https` |
| `host`, `port` | DNS uses `host` only; TCP/TLS require both |
| `url` | HTTP/HTTPS use a full URL instead of `host`/`port` |
| `expect.status` | HTTP/HTTPS accepted status codes; defaults to any `200–299` response |

Unknown keys, duplicate flow IDs, invalid settings, multiple YAML documents, and configurations over 1 MiB are rejected. Protocol names are lowercase. Concurrency defaults to 4 and can be set with `--concurrency 1..64` on `test`. Probe supports `--timeout` and `--retries`; test reads them from YAML.

## Repeated connection checks

```bash
netflow probe --host db.internal.example --port 5432 --protocol tcp \
  --count 20 --interval 1s

netflow probe --host db.internal.example --port 5432 --protocol tcp \
  --count 20 --interval 1s --output json > results/repeated.json
```

Create `results/` before redirecting into it. `--count` is 1–10000 (default 1); `--interval` is a pause **after** each completed sample, from 0 to 1h (default 1s). Repeated probes work with all existing probe protocols and require `--retries 0` so a retry cannot hide an intermittent failure. `--timeout` applies separately to every sample. `test` and YAML configuration do not accept count/interval yet.

Terminal output streams each sample, followed by the last sample's detail and aggregate statistics. JSON is a single report emitted at completion, containing every sample with its sequence number and UTC start time, plus a `repeat` summary: requested/completed/passed/failed/not-run counts, interruption flag, success percentage, and TCP connect min/average/max milliseconds. Stage `duration_us` supplies sub-millisecond precision alongside the existing `duration_ms` field.

Success rate is passed **configured checks** divided by completed samples; it is not packet loss. TCP timing covers the TCP stage (including attempts across resolved addresses), not ICMP RTT or application response time. Only successful TCP stages contribute to TCP timing, even when a later TLS/HTTP check fails. With no successful TCP connection, timing is omitted. On Ctrl+C, completed evidence is retained, the current canceled attempt can be recorded as failed, remaining samples are `not_run`, and the command exits 1. An empty run has a null success percentage. Any failed sample makes the overall exit code 1, even if the last sample passes.

## Temporary inbound receiver

Run a receiver on the intended destination VM/Pod, then probe it from the intended source:

```bash
# Destination: default bind is loopback; explicitly bind externally for remote tests.
netflow serve --listen 0.0.0.0:8080 --duration 10m --output json

# Source: use the receiver's reachable IP/DNS name.
netflow probe --host RECEIVER_IP --port 8080 --protocol tcp --count 20 --interval 1s
```

`serve` accepts TCP connections, records the local address and actual socket peer IP/port, then immediately closes the connection. It does not read/echo application data, provide HTTP/TLS, or perform authentication. Use a spare test port; it cannot share a port already bound by an application. This proves the test endpoint's path only, not a different service or policy.

The default bind is `127.0.0.1:8080`; IPv6 can use `[::]:8080`. Duration defaults to 10 minutes and must be greater than zero and at most 24 hours. `--max-connections N` stops after N accepted sockets (0, the default, means unlimited until the duration expires). Every accepted connection counts, including load balancer health checks or unrelated clients. Ctrl+C, the duration limit, and the connection limit stop normally with exit code 0; invalid arguments, bind/accept errors, or output errors return 2. Zero accepted connections is an idle receiver, not proof that a firewall blocked anything.

`--output json` emits **JSON Lines**, one event per line: `listening`, `connection`, then `stopped`. The first event includes source metadata. Connection events include `peer_address` and `observed_source_ip`; these describe the peer seen **at this listener**, which can be a proxy/load balancer/NAT address. No forwarded headers are trusted. The client report still reports its destination-observed source as unknown; receiver logs are not automatically returned or merged.

For containers:

```bash
docker run --rm -p 8080:8080 ghcr.io/uknowpro/netflow:v0.3.0 \
  serve --listen 0.0.0.0:8080 --duration 10m --output json
```

For Kubernetes, [examples/kubernetes-receiver.yaml](../examples/kubernetes-receiver.yaml) provides a temporary non-root Pod and a **ClusterIP** Service. It is not an external ingress by itself. Adapt it to the real namespace, labels, NetworkPolicy and entry path being tested; it does not configure firewall rules or create a public load balancer.

```bash
kubectl -n YOUR_NAMESPACE apply -f examples/kubernetes-receiver.yaml
kubectl -n YOUR_NAMESPACE logs -f pod/netflow-receiver
# Remove the temporary Pod and Service when finished.
kubectl -n YOUR_NAMESPACE delete -f examples/kubernetes-receiver.yaml
```

## Source identity and verification scope

Each JSON report includes a `source` object containing hostname, operating system, CPU architecture, the execution label, and addresses of active network interfaces. These interface addresses are an inventory, **not proof of the source address selected for a failed connection**. A successful connection records its actual `local_address` and `remote_address` separately per flow. IPv4 and IPv6 destinations are supported where the host network permits them.

The Kubernetes example injects `NETFLOW_POD_NAME`, `NETFLOW_POD_NAMESPACE`, `NETFLOW_NODE_NAME`, and `NETFLOW_POD_IP` through the Downward API. They are labeled as environment metadata; netflow does not authenticate them or query the Kubernetes/cloud metadata APIs. Missing metadata is omitted; hostname/interface collection failures produce warnings without preventing probes. No public IP lookup service is contacted. `observed_source_ip_status` stays `UNKNOWN` because only the destination or infrastructure logs can establish the address after NAT.

Each result includes `failed_stage` when applicable and an `assessment` separating TCP connectivity, authentication (`NOT_TESTED`), and firewall verdict (`UNKNOWN`). Remaining configured stages after a failure appear as `NOT_TESTED` / `NOT_REACHED`. These stage statuses do not change flow-level exit codes. TCP reachability does not validate DB login, queries, firewall rules, or all backend addresses. An HTTP 401/403 is reported as an HTTP response/expectation outcome, not a DB authentication diagnosis.

## Understand the results

```text
Source: worker-01 (linux/amd64)  Vantage: aks-app
Pod: netflow-outbound  Namespace: apps  Node: worker-01  Pod IP: 10.10.1.15 (environment metadata)
Source IP observed by target: UNKNOWN (NAT may change the local address)

[PASS] database-outbound  Target: db.internal.example:5432  12ms
  Connection: 10.10.1.15:48236 -> 10.20.1.30:5432 (local socket, before NAT)
  dns            PASS                                  3ms
  tcp            PASS                                  9ms
  authentication NOT_TESTED
  TCP reachable; configured checks passed. Authentication not tested.
```

| Status | Meaning |
| --- | --- |
| `PASS` | All configured checks passed |
| `FAIL` | An attempted check failed |
| `SKIPPED` | The execution label did not match; no verification performed |
| `UNKNOWN` | Reserved for future inconclusive probes; not emitted by current protocols |

A `403` response can mean DNS, TCP, and TLS passed while the HTTP expectation failed. A timeout does **not** prove a firewall blocked traffic. Check routing, service availability, policies, and relevant infrastructure logs.

Common codes include `DNS_ERROR`, `CONNECTION_REFUSED`, `CONNECT_TIMEOUT`, `CONNECT_ERROR`, `TLS_ERROR`, `HTTP_ERROR`, `HTTP_STATUS_MISMATCH`, `TIMEOUT`, `CANCELED`, and `VANTAGE_MISMATCH`.

JSON includes `schema_version`, `run_id`, UTC `started_at`, CLI version, execution label, summary, and ordered flow results. Each result includes stage outcomes and durations, resolved IPs, local/remote socket addresses when connected, attempt count, and HTTP status when available. On retries, stage evidence is from the **final attempt**; total duration includes prior attempts and retry pauses. Raw server error messages and response bodies are not included.

| Exit code | Meaning |
| --- | --- |
| `0` | All selected flows passed, or help/version succeeded |
| `1` | At least one selected flow failed, was skipped, or was inconclusive; also cancellation |
| `2` | Invalid command/configuration, empty group selection, or report encoding failure |

Skipped checks fail the automation gate so an untested route cannot silently appear successful. Select the appropriate group when a configuration covers multiple locations.

## Run in Kubernetes

Build and push an image to a registry your cluster can access:

```bash
docker build -t YOUR_REGISTRY/netflow:dev .
docker push YOUR_REGISTRY/netflow:dev
```

Edit the image and Pod labels in [examples/kubernetes-job.yaml](../examples/kubernetes-job.yaml). Use the intended application namespace and a configuration with real endpoints:

```bash
kubectl -n YOUR_NAMESPACE create configmap netflow-config \
  --from-file=network-flows.yaml=./network-flows.yaml
kubectl -n YOUR_NAMESPACE apply -f examples/kubernetes-job.yaml
kubectl -n YOUR_NAMESPACE logs -f job/netflow-outbound
kubectl -n YOUR_NAMESPACE get job netflow-outbound
```

The example runs one outbound group, uses a non-root user and read-only root filesystem, disables automatic service account token mounting, and does not retry failed Jobs. CLI retries still apply. Change the Job name or delete the completed Job before rerunning; updating a completed Job does not execute it again. `kubectl logs` shows output but does not propagate the container's exit code.

Match namespace, relevant labels, DNS configuration, service account identity where applicable, node placement, and service-mesh behavior to the application path you intend to test. A differently configured Pod can produce a different result. For inbound tests, run netflow outside the destination cluster on the relevant source network and use the real Service/Ingress endpoint.

## Run on a VM or in a container

Copy the built executable to the VM and run it there:

```bash
./netflow test --config network-flows.yaml \
  --vantage external-vm --group FW-GROUP-002
```

Or mount a configuration into the locally built image:

```bash
docker run --rm \
  -v "$PWD/network-flows.yaml:/config/flows.yaml:ro" \
  YOUR_REGISTRY/netflow:dev test --config /config/flows.yaml \
  --vantage external-vm --group FW-GROUP-002 --output json
```

Container networking may differ from the host's network path. Prefer the native executable when validating the VM's own path. TLS uses system trust roots and TLS 1.2 or newer. Install your organization's CA in the runtime trust store when needed; certificate validation cannot be disabled through the CLI.

## Scope and limitations

- HTTP uses GET, connects directly, ignores proxy environment variables, and does not follow redirects. It checks response headers/status; it does not read or validate response bodies.
- DNS uses the runtime's configured resolver. TCP attempts resolved addresses until one connects, sharing the attempt deadline. A pass establishes reachability to **one** address, not all DNS answers or backend instances.
- There is no UDP probe, source IP spoofing, firewall rule analysis, central agent/controller, or result aggregation service.
- Custom headers, authentication, environment-variable interpolation, mTLS, custom HTTP methods, and per-flow timeout overrides are not implemented yet.
- Use endpoints that do not require embedded secrets. Configured URLs are included in JSON reports; do not put tokens in URL query strings. URL user-info credentials are rejected.
- Reports can contain internal addresses and endpoint URLs; keep them in an appropriate location. `results/`, local configuration files, and `.env` files are ignored by Git.

## Releases and installation

Pushing a version tag such as `v0.3.0` triggers the release workflow. It runs tests, packages binaries for **10 OS/CPU combinations**, verifies the native executable and container, publishes a multi-platform image to `ghcr.io/uknowpro/netflow:<version>`, and then creates a GitHub Release with archives and `checksums.txt` (Windows ZIP; other platforms tar.gz). Prerelease tags such as `v0.3.0-rc.1` are marked as prereleases. Images use exact version tags; no moving `latest` tag is published.

After a version appears in [GitHub Releases](https://github.com/uknowpro/netflow/releases):

```bash
# Replace the version with an existing release; use arm64 for ARM machines.
VERSION=v0.3.0
ARCH=amd64
BASE="https://github.com/uknowpro/netflow/releases/download/$VERSION"
curl --fail --location --remote-name "$BASE/netflow_${VERSION}_linux_${ARCH}.tar.gz"
curl --fail --location --remote-name "$BASE/checksums.txt"
# Verify the downloaded architecture; the other archive is not downloaded here.
grep " netflow_${VERSION}_linux_${ARCH}.tar.gz$" checksums.txt | sha256sum --check --strict
tar -xzf "netflow_${VERSION}_linux_${ARCH}.tar.gz"
"./netflow_${VERSION}_linux_${ARCH}/netflow" version

docker run --rm "ghcr.io/uknowpro/netflow:$VERSION" version
```

For Kubernetes, replace the example image with the published version (or its digest). A newly created GHCR package may need its visibility set to public in GitHub package settings for anonymous pulls; otherwise configure registry credentials / `imagePullSecrets`. A public repository alone does not guarantee public package visibility.

Maintainers can prepare artifacts locally without publishing:

```bash
bash scripts/build-release.sh v0.3.0
# Output: dist/v0.3.0/ (ignored by Git)
```

To publish a reviewed commit:

```bash
git tag -a v0.3.0 -m "netflow v0.3.0"
git push origin v0.3.0
```

The workflow uses `GITHUB_TOKEN` with repository-content and package write permissions; no additional registry secret is required. Repository or organization policies must allow these permissions. Release tags are versioned outputs: do not move an existing tag. If a run fails before creating the GitHub Release, inspect the logs before retrying; an image may already have been pushed. The workflow does not overwrite existing GitHub Releases. Release publication is gated on native Linux, Windows, and macOS tests on x64 and ARM64 GitHub runners. Linux 386/ARMv7 and FreeBSD binaries are cross-built only. Real AKS/VM firewall paths remain separate environment checks.

Implementation references: [Docker multi-platform builds](https://docs.docker.com/build/ci/github-actions/multi-platform/) and [GitHub CLI release creation](https://cli.github.com/manual/gh_release_create).

## Development

```bash
gofmt -w cmd internal
go vet ./...
go test -race ./...
go build -o bin/netflow ./cmd/netflow
```

Tests use local listeners and HTTP/TLS servers to verify connection success/refusal, TLS certificate rejection, HTTP status checks, redirect behavior, timeouts, configuration validation, execution-location filtering, retry counts, JSON output, and exit codes. Actual AKS/VM firewall paths require environment-specific validation.

CI runs formatting, static checks, race-enabled tests, a CLI build, and a container build. Set a version string for your own build with `go build -ldflags="-X main.version=v0.3.0" -o bin/netflow ./cmd/netflow`.

## Roadmap

- Protocol-aware UDP checks with explicit inconclusive results.
- Authentication and private PKI configuration without exposing secrets in reports.
- Multi-location report aggregation and optional remote probe agents.
- Environment-specific AKS and VM validation (requires access to the target networks).

## License

[MIT](../LICENSE)
