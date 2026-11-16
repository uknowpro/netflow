<div align="center">

# netflow

**Kubernetes Pod와 VM에서 실행하는 네트워크 연결 검증 CLI**

[![CI](https://github.com/uknowpro/netflow/actions/workflows/ci.yml/badge.svg)](https://github.com/uknowpro/netflow/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](../LICENSE)

[English](../README.md) · 한국어

</div>

netflow는 환경 이전과 방화벽 변경 후 실제 연결 가능 여부를 확인하는 CLI입니다. VM과 Kubernetes Job에서 같은 실행 파일과 설정 형식을 사용하고, 실행 위치에 맞는 통신 항목을 선택하여 단계별 결과를 터미널 또는 JSON으로 확인합니다.

> 현재 초기 구현 단계입니다. 실제 연결을 검사하며 방화벽 규칙을 분석하거나 실패 원인을 방화벽으로 확정하지 않습니다. Cisco NetFlow/IPFIX 수집기와는 별개 프로젝트입니다. 한국어 지원은 이 문서로 제공하며 CLI 명령·도움말·결과 코드는 영어입니다.

## 주요 기능

- **Pod·VM 공통 CLI:** 별도 중앙 컨트롤러 없이 각 출발지에서 실행합니다.
- **양방향 통신 검증:** 검증 대상 시스템 기준으로 Inbound와 Outbound를 구분합니다.
- **단계별 결과:** DNS, TCP, TLS, HTTP 검증 결과를 구분합니다.
- **자동화 연동:** 엄격한 YAML 설정 검증, 제한된 동시 실행·재시도, JSON 보고서와 종료 코드를 제공합니다.
- **간단한 배포:** Go 실행 파일과 컨테이너로 실행하며 클러스터 API 접근이나 네트워크 특권이 필요하지 않습니다.

## Inbound와 Outbound

```text
Inbound:  외부 VM/Pod [netflow] → 신규 AKS Service / Ingress
Outbound: 신규 AKS Pod [netflow] → 외부 DB / API / VM
```

두 방향은 같은 검증 엔진을 사용합니다. **실제 실행 위치가 검증 경로를 결정합니다.** AKS 내부에서 설정에 `inbound`라고 적는 것만으로 외부 Inbound 경로를 재현할 수 없습니다. 허용된 실제 출발지에서 AKS의 실제 진입점으로 접속해야 합니다.

`vantage`는 운영자가 지정하는 실행 위치 이름입니다. 출발지 IP를 변경하거나 위치를 인증하는 기능이 아닙니다. NAT가 있으면 방화벽에 보이는 출발지 주소가 달라질 수 있으며, 보고서의 로컬 소켓 주소는 NAT 이전 주소입니다.

## 지원 환경

| 환경 | 배포 파일 / 이미지 | 검증 범위 |
| --- | --- | --- |
| Linux | amd64, arm64, armv7, 386 | x64·ARM64 네이티브 CI, ARMv7·386 교차 빌드 |
| Windows | amd64, arm64 (`.exe`, ZIP) | x64·ARM64 네이티브 CI |
| macOS | Intel amd64, Apple Silicon arm64 (`darwin` 파일) | Intel·Apple Silicon 네이티브 CI |
| FreeBSD | amd64, arm64 | 실험적 지원: 교차 빌드만 검증 |
| Docker / Podman / Kubernetes | Linux amd64, arm64 컨테이너 | 이미지 빌드·Linux amd64 기본 실행, 클러스터 경로 미검증 |

VM, 물리 서버, WSL2, CI 실행기에서는 해당 OS·CPU에 맞는 파일을 사용합니다. AKS·EKS·GKE·온프레미스 Kubernetes와 개별 Linux 배포판별 네트워크 검증은 아직 수행하지 않았습니다. Windows 네이티브 컨테이너 이미지는 제공하지 않습니다. Windows PowerShell·macOS 설치법과 제약은 [플랫폼 지원 안내](SUPPORT.ko.md)를 참고하세요.

## 빠른 시작

빌드에는 Go 1.26 이상이 필요합니다. 아래는 Linux 셸 기준이며 VM 실행에 Kubernetes는 필요하지 않습니다.

```bash
git clone https://github.com/uknowpro/netflow.git
cd netflow
go build -trimpath -o bin/netflow ./cmd/netflow

./bin/netflow --help
./bin/netflow probe --host 127.0.0.1 --protocol dns
./bin/netflow probe --host db.internal.example --port 5432 --protocol tcp
./bin/netflow probe --protocol https --url https://api.example.com/health
```

예제 주소는 실제 환경에 맞게 바꾸세요. 버전 태그를 올리면 자동 배포가 실행됩니다. 실제 배포된 버전은 [Releases](https://github.com/uknowpro/netflow/releases)에서 확인하세요.

## 설정과 실행

[예제 설정](../examples/network-flows.yaml)을 복사하여 목적지를 수정합니다.

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

각 출발지에서 해당 명령을 실행합니다.

```bash
# AKS 애플리케이션 네트워크의 Pod
netflow test --config network-flows.yaml --vantage aks-app --group FW-GROUP-001

# AKS 접근이 허용된 외부 VM
netflow test --config network-flows.yaml --vantage external-vm --group FW-GROUP-002

# JSON 보고서 저장
netflow test --config network-flows.yaml --vantage aks-app \
  --group FW-GROUP-001 --output json > result.json
```

그룹 필터를 먼저 적용합니다. 다른 그룹은 결과에서 제외됩니다. 선택한 항목 중 실행 위치가 다르면 트래픽을 보내지 않고 `SKIPPED`로 기록합니다. 설정에 `vantage`가 없는 항목은 어느 위치에서든 실행됩니다. `--group`을 생략하면 실행 위치가 달라 건너뛸 항목을 포함하여 모든 항목이 검토됩니다.

### 설정 항목

| 항목 | 설명 |
| --- | --- |
| `version` | 필수, 현재 `1` |
| `defaults.timeout` | 한 번의 시도에서 모든 단계에 적용되는 전체 제한 시간. 기본 `5s`, 최대 `5m` |
| `defaults.retries` | 실패 후 추가 시도 횟수. 기본 `0`, 범위 `0–5`, 시도 사이 200ms 대기 |
| `flows[].id` | 필수, 중복 불가 식별자 |
| `group` | 선택, 방화벽 요청 또는 업무 단위 그룹 |
| `direction` | 선택, `inbound` 또는 `outbound`. 경로를 변경하지 않는 메타데이터 |
| `vantage` | 선택, `--vantage`와 일치해야 실행되는 위치 이름 |
| `protocol` | 필수, `dns`, `tcp`, `tls`, `http`, `https` |
| `host`, `port` | DNS는 `host`만 사용. TCP/TLS는 둘 다 필수 |
| `url` | HTTP/HTTPS에서 `host`·`port` 대신 사용하는 전체 URL |
| `expect.status` | HTTP/HTTPS에서 허용할 상태 코드 목록. 생략하면 `200–299` |

알 수 없는 키, 중복 ID, 잘못된 값, 여러 YAML 문서, 1MiB를 초과하는 설정 파일은 거부합니다. 프로토콜 이름은 소문자입니다. `test`의 동시 실행 수는 기본 4이며 `--concurrency 1..64`로 조정합니다. `probe`는 `--timeout`, `--retries`를 지원하고 `test`는 YAML 설정을 사용합니다.

## 출발지 정보와 검증 범위

JSON의 `source`에 호스트명, 운영체제, CPU 종류, 실행 위치 이름, 활성 네트워크 인터페이스 주소 목록을 기록합니다. 인터페이스 주소는 보유 주소 목록이며 **연결 실패 시 실제 사용한 출발지 주소의 증거는 아닙니다**. 연결 성공 시 해당 연결의 `local_address`와 `remote_address`를 별도로 기록합니다. 실행 환경 네트워크가 지원하면 IPv4·IPv6를 사용할 수 있습니다.

Kubernetes 예제는 Downward API로 `NETFLOW_POD_NAME`, `NETFLOW_POD_NAMESPACE`, `NETFLOW_NODE_NAME`, `NETFLOW_POD_IP`를 전달합니다. 환경변수로 제공된 정보임을 표시하며 Kubernetes·클라우드 메타데이터 API를 조회하거나 정보의 진위를 인증하지 않습니다. 없는 Pod 정보는 생략하고 호스트명·인터페이스 수집 오류는 경고로 남기되 통신 검사는 계속합니다. 외부 공인 IP 조회 서비스에는 접속하지 않습니다. NAT 이후 실제 관측 주소는 목적지 또는 인프라 로그로 확인해야 하므로 `observed_source_ip_status`는 `UNKNOWN`입니다.

각 결과는 실패 단계인 `failed_stage`와 `assessment`를 포함합니다. TCP 연결 여부, 인증 미검증(`NOT_TESTED`), 방화벽 판정 불가(`UNKNOWN`)를 구분합니다. 앞선 단계가 실패하여 실행하지 못한 후속 단계는 `NOT_TESTED` / `NOT_REACHED`로 표시합니다. 단계별 미실행 표시는 기존 항목별 종료 코드 규칙을 변경하지 않습니다. TCP 성공은 DB 로그인·쿼리·방화벽 규칙·모든 백엔드 검증을 의미하지 않습니다. HTTP 401/403도 HTTP 응답과 기대 조건 결과로 표시하며 DB 인증 실패로 해석하지 않습니다.

## 결과 해석

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

| 상태 | 의미 |
| --- | --- |
| `PASS` | 설정한 모든 검증 조건 충족 |
| `FAIL` | 실행한 검증 단계에서 실패 |
| `SKIPPED` | 실행 위치 불일치로 미검증 |
| `UNKNOWN` | 향후 판정 불가능한 검증을 위한 예약 상태. 현재 프로토콜에서는 발생하지 않음 |

HTTP `403`은 DNS·TCP·TLS가 통과하고 HTTP 기대 조건만 실패한 결과일 수 있습니다. 시간 초과만으로 방화벽 차단을 확정할 수 없습니다. 라우팅, 서비스 상태, 정책 및 관련 인프라 로그를 함께 확인해야 합니다.

오류 코드는 `DNS_ERROR`, `CONNECTION_REFUSED`, `CONNECT_TIMEOUT`, `CONNECT_ERROR`, `TLS_ERROR`, `HTTP_ERROR`, `HTTP_STATUS_MISMATCH`, `TIMEOUT`, `CANCELED`, `VANTAGE_MISMATCH` 등입니다.

JSON에는 스키마 버전, 실행 ID, UTC 시작 시간, CLI 버전, 실행 위치, 요약과 입력 순서의 개별 결과가 포함됩니다. 각 결과는 단계별 상태·시간, 해석된 IP, 연결 시 로컬·원격 소켓 주소, 시도 횟수, 수신한 HTTP 상태를 기록합니다. 재시도 시 단계별 증거는 **마지막 시도**의 결과이며 전체 시간은 앞선 시도와 대기를 포함합니다. 서버 오류 원문이나 응답 본문은 기록하지 않습니다.

| 종료 코드 | 의미 |
| --- | --- |
| `0` | 선택한 항목 모두 통과 또는 도움말·버전 표시 성공 |
| `1` | 실패·건너뜀·판정 불가 항목 존재 또는 실행 취소 |
| `2` | 잘못된 명령·설정, 선택한 그룹에 항목 없음, JSON 출력 실패 |

미검증 경로가 성공으로 처리되지 않도록 `SKIPPED`도 자동화 실패로 처리합니다. 여러 위치를 포함한 설정은 각 실행 위치에 맞는 그룹을 선택하세요.

## Kubernetes Job 실행

클러스터에서 접근할 수 있는 레지스트리에 이미지를 빌드하고 올립니다.

```bash
docker build -t YOUR_REGISTRY/netflow:dev .
docker push YOUR_REGISTRY/netflow:dev
```

[Kubernetes Job 예제](../examples/kubernetes-job.yaml)의 이미지와 Pod 라벨을 수정하고 애플리케이션 네임스페이스와 실제 목적지가 들어간 설정을 사용합니다.

```bash
kubectl -n YOUR_NAMESPACE create configmap netflow-config \
  --from-file=network-flows.yaml=./network-flows.yaml
kubectl -n YOUR_NAMESPACE apply -f examples/kubernetes-job.yaml
kubectl -n YOUR_NAMESPACE logs -f job/netflow-outbound
kubectl -n YOUR_NAMESPACE get job netflow-outbound
```

예제는 Outbound 한 그룹을 실행하며 비루트 사용자, 읽기 전용 루트 파일시스템, ServiceAccount 토큰 자동 마운트 해제를 적용합니다. Job 자체의 실패 재시도는 없고 CLI 설정의 재시도는 적용됩니다. 다시 실행하려면 Job 이름을 바꾸거나 완료된 Job을 삭제한 뒤 생성하세요. 완료된 Job에 설정을 적용하는 것만으로 재실행되지는 않습니다. `kubectl logs`의 종료 코드는 컨테이너의 종료 코드를 전달하지 않습니다.

검증하려는 애플리케이션과 네임스페이스, 관련 라벨, DNS 설정, 필요한 ServiceAccount 신원, 노드 배치, 서비스 메시 동작을 맞추세요. 다른 설정의 Pod는 다른 결과를 낼 수 있습니다. Inbound는 대상 클러스터 외부의 실제 출발지에서 실제 Service/Ingress 진입점으로 테스트합니다.

## VM·컨테이너 실행

VM에 빌드된 실행 파일과 설정을 복사한 뒤 실행합니다.

```bash
./netflow test --config network-flows.yaml \
  --vantage external-vm --group FW-GROUP-002
```

컨테이너에서는 설정을 마운트합니다.

```bash
docker run --rm \
  -v "$PWD/network-flows.yaml:/config/flows.yaml:ro" \
  YOUR_REGISTRY/netflow:dev test --config /config/flows.yaml \
  --vantage external-vm --group FW-GROUP-002 --output json
```

컨테이너 네트워크는 VM 자체의 네트워크 경로와 다를 수 있습니다. VM의 경로를 검증하려면 실행 파일을 직접 실행하는 방식이 적합합니다. TLS는 시스템 신뢰 저장소와 TLS 1.2 이상을 사용합니다. 사설 인증서가 필요하면 실행 환경의 신뢰 저장소에 조직의 CA를 설치하세요. CLI에서 인증서 검증을 비활성화하는 옵션은 제공하지 않습니다.

## 현재 범위와 제한

- HTTP는 GET으로 직접 접속합니다. 프록시 환경변수를 사용하지 않고 리다이렉트를 따라가지 않습니다. 응답 헤더·상태를 검사하며 본문을 읽거나 검증하지 않습니다.
- DNS는 실행 환경의 리졸버를 사용합니다. TCP는 반환된 주소에 순서대로 연결하며 한 시도의 제한 시간을 공유합니다. 성공은 주소 하나에 대한 연결을 의미하며 모든 IP나 백엔드 인스턴스의 성공을 보장하지 않습니다.
- UDP, 수신 서버, 출발지 IP 위조, 방화벽 규칙 분석, 중앙 에이전트·컨트롤러, 결과 집계 서비스는 포함되지 않습니다.
- 커스텀 헤더, 인증, 환경변수 치환, mTLS, HTTP 메서드 선택, 항목별 시간 제한은 아직 지원하지 않습니다.
- URL은 JSON에 포함됩니다. 쿼리 문자열에 토큰을 넣지 마세요. URL 사용자 정보 형식의 인증 정보는 거부합니다.
- 보고서에는 내부 주소가 포함될 수 있습니다. `results/`, 로컬 설정 파일, `.env` 파일은 Git에서 제외합니다.

## 자동 배포와 설치

`v0.1.2` 같은 버전 태그를 푸시하면 테스트, **10개 OS·CPU 조합**의 실행 파일 패키징, 네이티브 실행 파일·컨테이너 동작 확인을 수행합니다. 이후 `ghcr.io/uknowpro/netflow:<버전>`에 멀티 플랫폼 이미지를 올리고, 압축 파일(Windows는 ZIP, 나머지는 tar.gz)과 `checksums.txt`가 첨부된 GitHub Release를 생성합니다. `v0.1.2-rc.1` 같은 태그는 사전 릴리스로 표시합니다. 이미지는 정확한 버전 태그만 제공하며 `latest`는 만들지 않습니다.

[GitHub Releases](https://github.com/uknowpro/netflow/releases)에 버전이 생성된 뒤 설치합니다.

```bash
# 실제 배포된 버전으로 변경하세요. ARM 환경에서는 arm64를 사용합니다.
VERSION=v0.1.2
ARCH=amd64
BASE="https://github.com/uknowpro/netflow/releases/download/$VERSION"
curl --fail --location --remote-name "$BASE/netflow_${VERSION}_linux_${ARCH}.tar.gz"
curl --fail --location --remote-name "$BASE/checksums.txt"
# 내려받은 아키텍처의 파일만 검증합니다.
grep " netflow_${VERSION}_linux_${ARCH}.tar.gz$" checksums.txt | sha256sum --check --strict
tar -xzf "netflow_${VERSION}_linux_${ARCH}.tar.gz"
"./netflow_${VERSION}_linux_${ARCH}/netflow" version

docker run --rm "ghcr.io/uknowpro/netflow:$VERSION" version
```

Kubernetes 예제의 이미지를 배포된 버전 또는 이미지 digest로 바꾸면 됩니다. GHCR에 새로 생성한 패키지는 익명으로 내려받으려면 GitHub 패키지 설정에서 공개 범위 변경이 필요할 수 있습니다. 비공개라면 레지스트리 인증 또는 `imagePullSecrets`를 설정합니다. 저장소가 공개라고 패키지도 자동으로 공개되는 것은 아닙니다.

배포 없이 로컬에서 패키지를 만들 수 있습니다.

```bash
bash scripts/build-release.sh v0.1.2
# 결과: dist/v0.1.2/ (Git 제외)
```

검토가 끝난 커밋을 배포할 때는 다음과 같이 실행합니다.

```bash
git tag -a v0.1.2 -m "netflow v0.1.2"
git push origin v0.1.2
```

자동 배포는 저장소 콘텐츠와 패키지 쓰기 권한을 가진 `GITHUB_TOKEN`을 사용합니다. 별도 레지스트리 비밀값은 필요하지 않지만 저장소·조직 정책에서 해당 권한을 허용해야 합니다. 기존 버전 태그를 다른 커밋으로 이동하지 마세요. GitHub Release 생성 전에 실패하면 이미지는 이미 올라갔을 수 있으므로 로그를 확인한 뒤 재시도합니다. 기존 GitHub Release를 덮어쓰지는 않습니다. 릴리스 공개 전 GitHub의 Linux·Windows·macOS x64·ARM64 실행 환경에서 네이티브 테스트를 통과해야 합니다. Linux 386·ARMv7과 FreeBSD는 교차 빌드만 검증하며 실제 AKS·VM 방화벽 경로는 해당 환경에서 별도로 검증합니다.

구현 참고: [Docker 멀티 플랫폼 빌드](https://docs.docker.com/build/ci/github-actions/multi-platform/), [GitHub CLI 릴리스 생성](https://cli.github.com/manual/gh_release_create).

## 개발과 검증

```bash
gofmt -w cmd internal
go vet ./...
go test -race ./...
go build -o bin/netflow ./cmd/netflow
```

테스트는 로컬 서버로 연결 성공·거부, 신뢰되지 않은 TLS 인증서 거부, HTTP 상태 검증, 리다이렉트 미추적, 시간 초과, 설정 검증, 위치 필터링, 재시도 횟수, JSON과 종료 코드를 확인합니다. 실제 AKS·VM의 방화벽 경로는 해당 환경에서 별도로 검증해야 합니다.

CI는 포맷, 정적 검사, 경쟁 상태 검사 포함 테스트, CLI 빌드 및 컨테이너 빌드를 수행합니다. 버전 문자열은 `go build -ldflags="-X main.version=v0.1.2" -o bin/netflow ./cmd/netflow`로 지정할 수 있습니다.

## 향후 확장

- 프로토콜별 응답을 확인하는 UDP 검증과 판정 불가 결과
- 보고서에 비밀값을 노출하지 않는 인증·사설 PKI 설정
- 여러 출발지의 결과 집계와 선택적 원격 에이전트
- 실제 AKS·VM 경로 검증: 해당 네트워크에 접근할 수 있는 환경에서 별도 수행

## 라이선스

[MIT](../LICENSE)
