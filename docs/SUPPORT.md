# Platform support

[English](SUPPORT.md) · [한국어](SUPPORT.ko.md) · [README](../README.md)

## Release assets

| OS | Architectures | Format | Confidence |
| --- | --- | --- | --- |
| Linux | amd64, arm64 | tar.gz | Native CI tests on both architectures |
| Linux | armv7, 386 | tar.gz | Cross-built; runtime unverified |
| Windows | amd64, arm64 | ZIP, `netflow.exe` | Native CI tests on both architectures |
| macOS (`darwin`) | amd64, arm64 | tar.gz | Native CI tests on Intel and Apple Silicon |
| FreeBSD | amd64, arm64 | tar.gz | Experimental; cross-built, runtime unverified |

Native CI tests include local TCP/HTTP/TLS servers, source metadata, parsing, reporting, and CLI execution. This is not a certification of every OS version or network environment. See the release's Actions run for actual test results. The release is blocked if a native platform job fails.

The release toolchain follows `go.mod` (Go 1.26). Runtime support is limited by that Go toolchain's [minimum OS requirements](https://go.dev/wiki/MinimumRequirements) and [release notes](https://go.dev/doc/go1.26). Linux binaries use `CGO_ENABLED=0` and do not require a particular libc; TLS still needs a configured system CA trust store. ARMv7 builds use `GOARM=7` and are not ARMv6 binaries. No 32-bit Windows, mobile, browser/WASM, or native Windows container releases are supplied. macOS/Windows binaries are not vendor-signed or notarized.

## Windows PowerShell

Select `amd64` for x64 or `arm64` for Windows on ARM. Replace the version with a published release.

```powershell
$Version = 'v0.1.2'
$Arch = 'amd64'
$Asset = "netflow_${Version}_windows_${Arch}.zip"
$Base = "https://github.com/uknowpro/netflow/releases/download/$Version"
Invoke-WebRequest "$Base/$Asset" -OutFile $Asset
Invoke-WebRequest "$Base/checksums.txt" -OutFile checksums.txt
$Line = Get-Content checksums.txt | Where-Object { $_.EndsWith("  $Asset") }
if (@($Line).Count -ne 1) { throw 'Checksum entry missing or ambiguous' }
$Expected = ($Line -split '\s+')[0]
if ((Get-FileHash $Asset -Algorithm SHA256).Hash -ne $Expected) { throw 'Checksum mismatch' }
Expand-Archive $Asset -DestinationPath .
& ".\netflow_${Version}_windows_${Arch}\netflow.exe" version
& ".\netflow_${Version}_windows_${Arch}\netflow.exe" probe --host 127.0.0.1 --protocol dns
```

Windows native TLS uses the Windows trust store. Probes run with the network policy, routes and VPN context of that process. WSL2 is a Linux execution environment with potentially different routing/NAT; it is not a substitute for testing the Windows host's path.

## macOS

Choose `arm64` on Apple Silicon or `amd64` on Intel:

```bash
VERSION=v0.1.2
ARCH=arm64
ASSET="netflow_${VERSION}_darwin_${ARCH}.tar.gz"
BASE="https://github.com/uknowpro/netflow/releases/download/$VERSION"
curl --fail --location --remote-name "$BASE/$ASSET"
curl --fail --location --remote-name "$BASE/checksums.txt"
grep " $ASSET$" checksums.txt | shasum -a 256 --check
tar -xzf "$ASSET"
"./netflow_${VERSION}_darwin_${ARCH}/netflow" version
```

Use your normal organizational approval/trust process for downloaded unsigned executables. netflow does not disable OS security controls. Native macOS TLS uses the operating system's trust configuration.

## Containers, Kubernetes and VMs

`ghcr.io/uknowpro/netflow:<version>` contains Linux amd64 and arm64 images with system CA certificates. Docker selects the matching image. Docker Desktop runs this Linux image in its VM; use a native binary to test the host's own path. Podman and Kubernetes consume the same OCI image, but their network paths require environment-specific checks.

The CLI runs without root, raw sockets, Kubernetes API credentials, or a background service. The Kubernetes Job example includes Downward API metadata and a restricted security context. The image can use a platform-assigned non-root UID; writable storage is not needed for stdout reports. Standard VM/cloud providers are not detected or contacted automatically.

Use binaries on Linux distributions such as Ubuntu/Debian, RHEL-compatible systems, Amazon Linux or Alpine when the runtime requirements are met. These are compatibility targets, not individually tested/certified distributions. AKS/EKS/GKE, OpenShift and on-premises clusters remain environment-specific validation targets. Copying a binary into a workload does not make its network policy equivalent to another workload.

## What remains unverified

- Real inbound/outbound firewall paths, NAT translations, and application NetworkPolicies.
- DB login and queries; `authentication` is always `NOT_TESTED` in this version.
- Actual FreeBSD, Linux ARMv7 and Linux 386 execution.
- Every OS release, VPN, service mesh, proxy setup, and Kubernetes distribution.

Local addresses and environment-provided Pod identity are evidence about the test process only. Destination-observed source addresses and firewall decisions remain `UNKNOWN` without external evidence.
