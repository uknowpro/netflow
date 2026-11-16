#!/usr/bin/env bash
set -euo pipefail

version=${1:?Usage: scripts/build-release.sh vMAJOR.MINOR.PATCH[-prerelease]}
if [[ ! "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[a-zA-Z0-9]+([.-][a-zA-Z0-9]+)*)?$ ]]; then
  echo "Invalid release version: $version" >&2
  exit 2
fi
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
output="$root/dist/$version"
mkdir -p "$output"
staging=$(mktemp -d)
trap 'rm -rf "$staging"' EXIT
for arch in amd64 arm64; do
  package="netflow_${version}_linux_${arch}"
  mkdir -p "$staging/$package"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath \
    -ldflags="-s -w -X main.version=$version" \
    -o "$staging/$package/netflow" ./cmd/netflow
  cp LICENSE README.md "$staging/$package/"
  cp -R docs examples "$staging/$package/"
  tar -czf "$output/$package.tar.gz" -C "$staging" "$package"
done
(cd "$output" && sha256sum "netflow_${version}_linux_amd64.tar.gz" "netflow_${version}_linux_arm64.tar.gz" > checksums.txt)
printf 'Release assets: %s\n' "$output"
