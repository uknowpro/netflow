# 플랫폼 지원 안내

[English](SUPPORT.md) · [한국어](SUPPORT.ko.md) · [README](README.ko.md)

## 배포 대상

| OS | CPU | 형식 | 검증 수준 |
| --- | --- | --- | --- |
| Linux | amd64, arm64 | tar.gz | 두 CPU의 네이티브 CI 테스트 |
| Linux | armv7, 386 | tar.gz | 교차 빌드, 실제 실행 미검증 |
| Windows | amd64, arm64 | ZIP, `netflow.exe` | 두 CPU의 네이티브 CI 테스트 |
| macOS (`darwin`) | amd64, arm64 | tar.gz | Intel·Apple Silicon 네이티브 CI 테스트 |
| FreeBSD | amd64, arm64 | tar.gz | 실험적 지원: 교차 빌드, 실제 실행 미검증 |

네이티브 CI는 로컬 TCP·HTTP·TLS 서버, 출발지 정보, 설정·보고서 및 CLI 실행을 검사합니다. 모든 OS 버전이나 실제 네트워크 환경의 인증을 의미하지 않습니다. 실제 성공 여부는 해당 릴리스의 Actions 결과에서 확인합니다. 네이티브 플랫폼 테스트가 실패하면 릴리스가 차단됩니다.

릴리스 빌드 도구는 `go.mod`의 Go 1.26을 따릅니다. 실행 가능한 OS 버전은 Go의 [최소 요구사항](https://go.dev/wiki/MinimumRequirements)과 [릴리스 노트](https://go.dev/doc/go1.26)의 범위입니다. Linux는 `CGO_ENABLED=0`으로 빌드하여 특정 libc 설치를 요구하지 않지만 TLS에는 시스템 CA 신뢰 저장소가 필요합니다. ARMv7은 `GOARM=7`이며 ARMv6용이 아닙니다. 32비트 Windows, 모바일, 브라우저/WASM, 네이티브 Windows 컨테이너는 배포하지 않습니다. macOS·Windows 파일에 별도 개발사 서명이나 공증은 없습니다.

## Windows PowerShell

x64는 `amd64`, Windows on ARM은 `arm64`를 선택합니다. 버전은 실제 배포 버전으로 바꾸세요.

```powershell
$Version = 'v0.2.0'
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

Windows에서는 Windows 인증서 신뢰 저장소를 사용하며 해당 프로세스의 네트워크 정책·라우팅·VPN 환경에서 실행합니다. WSL2는 별도 Linux 실행 환경이며 라우팅·NAT가 다를 수 있으므로 Windows 호스트 자체의 경로 검증을 대신하지 않습니다.

## macOS

Apple Silicon은 `arm64`, Intel은 `amd64`를 선택합니다.

```bash
VERSION=v0.2.0
ARCH=arm64
ASSET="netflow_${VERSION}_darwin_${ARCH}.tar.gz"
BASE="https://github.com/uknowpro/netflow/releases/download/$VERSION"
curl --fail --location --remote-name "$BASE/$ASSET"
curl --fail --location --remote-name "$BASE/checksums.txt"
grep " $ASSET$" checksums.txt | shasum -a 256 --check
tar -xzf "$ASSET"
"./netflow_${VERSION}_darwin_${ARCH}/netflow" version
```

서명되지 않은 다운로드 실행 파일은 조직의 일반적인 신뢰·승인 절차에 따라 실행하세요. netflow는 OS 보호 기능을 해제하지 않습니다. TLS는 macOS의 인증서 신뢰 설정을 사용합니다.

## 컨테이너·Kubernetes·VM

`ghcr.io/uknowpro/netflow:<버전>`은 CA 인증서를 포함한 Linux amd64·arm64 이미지입니다. Docker가 해당 CPU의 이미지를 선택합니다. Docker Desktop은 내부 VM의 Linux 환경이므로 호스트 자체의 경로를 검사하려면 네이티브 실행 파일을 사용하세요. Podman과 Kubernetes에서도 같은 OCI 이미지를 사용하지만 각 환경의 네트워크 경로는 별도로 확인해야 합니다.

root 권한, raw socket, Kubernetes API 인증, 상주 서비스가 필요하지 않습니다. Job 예제는 Downward API와 제한된 보안 설정을 포함합니다. 플랫폼이 지정한 비루트 UID로도 이미지를 실행할 수 있으며 표준 출력 보고서에 쓰기 가능한 저장소가 필요하지 않습니다. 클라우드 사업자를 자동 감지하거나 메타데이터 서비스에 접속하지 않습니다.

OS 요구사항을 만족하는 Ubuntu·Debian, RHEL 호환 배포판, Amazon Linux, Alpine 등에서 실행하는 것을 목표로 합니다. 개별 배포판을 모두 검증하거나 인증한 것은 아닙니다. AKS·EKS·GKE·OpenShift·온프레미스 Kubernetes는 실제 환경 검증이 필요합니다. 같은 실행 파일을 사용해도 다른 워크로드의 네트워크 정책이 자동으로 적용되지는 않습니다.

## 미검증 항목

- 실제 Inbound·Outbound 방화벽 경로, NAT 변환, 애플리케이션 NetworkPolicy
- DB 로그인·쿼리: 이번 버전의 `authentication`은 항상 `NOT_TESTED`
- 실제 FreeBSD, Linux ARMv7·386 환경의 실행
- 모든 OS 버전, VPN, 서비스 메시, 프록시, Kubernetes 배포판 조합

로컬 주소와 환경변수로 받은 Pod 정보는 테스트 프로세스의 정보입니다. 목적지가 본 출발지 IP와 방화벽 판정은 외부 증거가 없으면 `UNKNOWN`으로 유지합니다.
