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
archives=()
for target in linux/amd64 linux/arm64 linux/arm linux/386 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 freebsd/amd64 freebsd/arm64; do
  os=${target%/*}
  arch=${target#*/}
  label=$arch
  if [[ "$arch" == arm ]]; then label=armv7; fi
  package="netflow_${version}_${os}_${label}"
  binary=netflow
  if [[ "$os" == windows ]]; then binary=netflow.exe; fi
  mkdir -p "$staging/$package"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" GOARM=7 go build -trimpath \
    -ldflags="-s -w -X main.version=$version" \
    -o "$staging/$package/$binary" ./cmd/netflow
  cp LICENSE README.md "$staging/$package/"
  cp -R docs examples "$staging/$package/"
  if [[ "$os" == windows ]]; then
    # Remove only this version's archive so zip cannot retain obsolete entries.
    rm -f "$output/$package.zip"
    (cd "$staging" && zip -qr "$output/$package.zip" "$package")
    archives+=("$package.zip")
  else
    tar -czf "$output/$package.tar.gz" -C "$staging" "$package"
    archives+=("$package.tar.gz")
  fi
done
(cd "$output" && sha256sum "${archives[@]}" > checksums.txt)
printf 'Release assets: %s\n' "$output"
