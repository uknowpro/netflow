<div align="center">

# netflow

**Verify network reachability from Kubernetes Pods and virtual machines.**

[![CI](https://github.com/uknowpro/netflow/actions/workflows/ci.yml/badge.svg)](https://github.com/uknowpro/netflow/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

English · [한국어](docs/README.ko.md)

</div>

netflow is a small, active network verification CLI for migration checks and firewall change validation. Run the same binary on a VM or inside a Kubernetes Job, select the flows belonging to that location, and get stage-by-stage evidence in a terminal or JSON report.

> Initial implementation. netflow tests actual connections; it does not inspect firewall rules or prove that a firewall caused a failure. Despite its name, it is not a Cisco NetFlow/IPFIX collector.

## Why netflow?

- **One CLI, multiple locations:** use a VM, a standalone container, or a Kubernetes Pod.
- **Explicit direction:** describe inbound and outbound flows relative to the system under test.
- **Layered evidence:** separate DNS resolution, TCP connection, TLS handshake, and HTTP expectations.
- **Automation friendly:** strict YAML input, bounded concurrency and retries, JSON output, and meaningful exit codes.
- **Small deployment footprint:** a Go executable; no controller, cluster API access, or privileged networking required.

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

Example endpoints are placeholders. Replace them with your own authorized destinations. Prebuilt binaries and a published container image are not provided by this initial implementation.

## Define network flows

Copy [examples/network-flows.yaml](examples/network-flows.yaml) and replace the destinations:

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

## Understand the results

```text
FLOW                     STATUS    DETAIL                   TIME
database-outbound        PASS                               12ms
api-inbound              SKIPPED   VANTAGE_MISMATCH         0ms

PASS=1 FAIL=0 UNKNOWN=0 SKIPPED=1
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

Edit the image and Pod labels in [examples/kubernetes-job.yaml](examples/kubernetes-job.yaml). Use the intended application namespace and a configuration with real endpoints:

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
- There is no UDP probe, listening server, source IP spoofing, firewall rule analysis, central agent/controller, or result aggregation service.
- Custom headers, authentication, environment-variable interpolation, mTLS, custom HTTP methods, and per-flow timeout overrides are not implemented yet.
- Use endpoints that do not require embedded secrets. Configured URLs are included in JSON reports; do not put tokens in URL query strings. URL user-info credentials are rejected.
- Reports can contain internal addresses and endpoint URLs; keep them in an appropriate location. `results/`, local configuration files, and `.env` files are ignored by Git.

## Development

```bash
gofmt -w cmd internal
go vet ./...
go test -race ./...
go build -o bin/netflow ./cmd/netflow
```

Tests use local listeners and HTTP/TLS servers to verify connection success/refusal, TLS certificate rejection, HTTP status checks, redirect behavior, timeouts, configuration validation, execution-location filtering, retry counts, JSON output, and exit codes. Actual AKS/VM firewall paths require environment-specific validation.

CI runs formatting, static checks, race-enabled tests, a CLI build, and a container build. Set a version string for your own build with `go build -ldflags="-X main.version=v0.1.0" -o bin/netflow ./cmd/netflow`.

## Roadmap

- Protocol-aware UDP checks with explicit inconclusive results.
- Authentication and private PKI configuration without exposing secrets in reports.
- Multi-location report aggregation and optional remote probe agents.
- Automated binary and container releases.

## License

[MIT](LICENSE)
