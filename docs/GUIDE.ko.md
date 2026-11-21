# 상세 사용 가이드

[빠른 시작](README.ko.md) · [English guide](GUIDE.md)

## 주요 기능

- **Pod·VM 공통 CLI:** 별도 중앙 컨트롤러 없이 각 출발지에서 실행합니다.
- **양방향 통신 검증:** 검증 대상 시스템 기준으로 Inbound와 Outbound를 구분합니다.
- **단계별 결과:** DNS, TCP, TLS, HTTP 검증 결과를 구분합니다.
- **자동화 연동:** 엄격한 YAML 설정 검증, 제한된 동시 실행·재시도, JSON 보고서와 종료 코드를 제공합니다.
- **간단한 배포:** Go 실행 파일과 컨테이너로 실행하며 클러스터 API 접근이나 네트워크 특권이 필요하지 않습니다.

## AKS·VM 제어 지점별 검사

환경별 예제: [AKS](../examples/aks-flows.yaml), [Azure VM](../examples/azure-vm-flows.yaml), [온프레미스 VM](../examples/onprem-vm-flows.yaml). 예제는 가상의 경로입니다. 목적지, 장비 이름, 순서를 실제 구성으로 바꾸세요. Inbound/Outbound는 대상 AKS 시스템 기준입니다.

```bash
# 각 명령은 해당 출발지에서 실행합니다. 새 필드를 지원하는 빌드가 필요합니다.
netflow test --config examples/aks-flows.yaml --vantage aks --output json
netflow test --config examples/azure-vm-flows.yaml --vantage azure-vm
netflow test --config examples/onprem-vm-flows.yaml --vantage onprem-vm
```

선택 필드 `source_environment`는 `aks` 또는 `vm`이라는 사용자 선언입니다. 실행 환경을 자동 탐지하거나 강제하지 않으며, 기존 `vantage`가 실행 위치 이름을 대조합니다. `expected_path`는 예상 경유 순서대로 작성하는 `{type, name}` 목록입니다. 최대 32개, 이름은 1–200바이트이며 지원 유형은 `external_firewall`, `external_waf`, `nsg`, `application_gateway`, `on_prem_firewall`, `network_policy`, `other`입니다. 기존 version 1 설정은 그대로 지원합니다. 새 필드는 v0.3.0 이상에서 지원하며 구버전 실행 파일은 이를 거부합니다.

터미널은 경로를 사용자 선언·미검증으로 표시합니다. JSON의 `results[].flow`에 선언을 보존하고, 경로가 있으면 PASS·SKIPPED 여부와 관계없이 `assessment.expected_path_verification`을 `NOT_TESTED`로 표시합니다. `flow.expect.status`에는 명시한 기대 HTTP 상태를 기록하며 빈 객체는 기본값 200–299를 뜻합니다. 터미널에서도 기대 상태와 실제 HTTP 상태를 구별합니다. 예상 장비를 실제 관측 단계나 장비별 PASS로 표시하지 않습니다.

WAF·Application Gateway는 실제 서비스 FQDN으로 검사하여 HTTP Host와 TLS SNI가 맞도록 하세요. DNS는 실행 환경의 리졸버를 사용하므로 사설·분할 DNS도 해당 환경에 따라 해석됩니다. TCP PASS는 해석된 주소 중 하나에 연결했다는 의미입니다. HTTP PASS는 설정한 응답 조건만 만족했다는 뜻입니다. 예를 들어 403을 기대값으로 지정하면 PASS여도 서비스 접근·인증 성공은 아닙니다. 403이나 타임아웃만으로 차단 장비를 특정할 수 없습니다.

AKS Job에서는 기존 ConfigMap 예제를 이용해 선택한 파일을 `network-flows.yaml`로 마운트하고 Job의 `--vantage`, `--group`을 `aks`로 바꾸세요. 애플리케이션의 네임스페이스·라벨·실행 신원·DNS·노드/네트워크·서비스 메시 조건을 맞추세요. VM 자체 경로는 네이티브 실행 파일로 검사하는 것이 적합합니다. 임시 TCP receiver로는 WAF의 HTTP 경로를 검증할 수 없으므로 실제 허용된 HTTP(S) 상태 확인 엔드포인트를 사용하세요.

장애 분석 시 보고서의 UTC 시작 시각, flow ID, 로컬/원격 소켓 주소, HTTP 상태를 해당 방화벽·WAF·NSG·게이트웨이 로그와 대조하세요. NAT·프록시가 주소를 바꿀 수 있습니다. netflow는 장비 로그 조회, 경로 자동 탐지, 규칙 분석, 차단 장비 확정을 수행하지 않습니다. 실제 AKS·VM·온프레미스 경로는 현장에서 실행하기 전까지 미검증입니다.


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

## 반복 연결 검사

```bash
netflow probe --host db.internal.example --port 5432 --protocol tcp \
  --count 20 --interval 1s

netflow probe --host db.internal.example --port 5432 --protocol tcp \
  --count 20 --interval 1s --output json > results/repeated.json
```

파일 저장 전 `results/` 디렉터리를 생성하세요. `--count`는 1–10000회(기본 1회), `--interval`은 각 시도가 **끝난 후** 다음 시도까지의 대기 시간으로 0–1시간(기본 1초)입니다. 기존 `probe` 프로토콜 모두에 적용하며, 간헐적 실패를 재시도로 가리지 않도록 반복 검사에서는 `--retries 0`만 허용합니다. `--timeout`은 매 시도마다 별도로 적용됩니다. `test`·YAML에는 아직 count/interval을 적용하지 않습니다.

터미널은 시도별 결과를 바로 표시하고 마지막 시도의 상세 결과와 전체 통계를 출력합니다. JSON은 종료 시 하나의 보고서로 출력하며, 각 시도의 순번·UTC 시작 시각·결과와 `repeat` 요약을 포함합니다. 요약에는 요청·완료·성공·실패·미실행 횟수, 중단 여부, 성공률, TCP 연결 시간의 최소·평균·최댓값이 담깁니다. 단계별 `duration_us`를 추가하여 기존 `duration_ms`와 함께 1ms 미만 시간도 기록합니다.

성공률은 완료한 시도 중 **설정한 조건을 통과한 비율**이며 패킷 손실률이 아닙니다. TCP 시간은 여러 해석된 IP에 대한 연결 시도를 포함한 TCP 단계 시간이며 ICMP RTT나 애플리케이션 응답 시간이 아닙니다. TCP 성공 단계만 시간 통계에 포함되며 이후 TLS·HTTP가 실패해도 해당 TCP 시간은 포함됩니다. TCP 성공이 없으면 시간 통계는 생략합니다. Ctrl+C 시 완료한 결과를 보존하고 현재 취소된 시도는 실패로 기록될 수 있으며 나머지는 `not_run`으로 남깁니다. 이 경우 종료 코드는 1입니다. 실행된 시도가 없으면 성공률은 null입니다. 마지막 시도가 성공했더라도 앞서 실패가 한 번이라도 있으면 전체 종료 코드는 1입니다.

## 임시 Inbound 수신 서버

검증 대상 VM·Pod에서 수신 서버를 실행하고 실제 출발지에서 접속합니다.

```bash
# 목적지: 외부 접속을 받으려면 명시적으로 외부 바인딩을 지정합니다.
netflow serve --listen 0.0.0.0:8080 --duration 10m --output json

# 출발지: 접근 가능한 목적지 IP 또는 도메인으로 변경합니다.
netflow probe --host RECEIVER_IP --port 8080 --protocol tcp --count 20 --interval 1s
```

`serve`는 TCP 연결을 받아 로컬 주소와 **실제 소켓에서 보이는 상대 IP·포트**를 기록한 뒤 즉시 닫습니다. 애플리케이션 데이터를 읽거나 반환하지 않으며 HTTP·TLS·인증 기능을 제공하지 않습니다. 실제 앱이 점유한 포트를 공유할 수 없으므로 별도 테스트 포트를 사용하세요. 검증 결과는 이 테스트 엔드포인트의 경로에만 해당하며 다른 서비스나 정책의 성공을 보장하지 않습니다.

기본 바인딩은 `127.0.0.1:8080`이며 IPv6는 `[::]:8080`을 지정할 수 있습니다. 실행 시간은 기본 10분, 0보다 크고 최대 24시간입니다. `--max-connections N`은 N개 연결을 수락한 뒤 종료하며 기본 0은 실행 시간 내 연결 수 제한 없음을 의미합니다. 로드밸런서 상태 검사나 다른 클라이언트 등 모든 수락된 연결이 횟수에 포함됩니다. Ctrl+C·시간 제한·횟수 제한에 따른 정상 종료는 코드 0, 잘못된 인자·바인딩·수신·출력 오류는 코드 2입니다. 연결이 0건이었다는 사실만으로 방화벽 차단을 판정하지 않습니다.

`--output json`은 한 줄에 이벤트 하나를 기록하는 **JSON Lines**입니다. `listening` → `connection` → `stopped` 이벤트를 출력하며 첫 이벤트에 출발 환경 정보가 포함됩니다. 연결 이벤트의 `peer_address`, `observed_source_ip`는 **해당 수신 소켓이 본 상대 주소**로, 프록시·로드밸런서·NAT의 주소일 수 있습니다. 전달 헤더를 신뢰하지 않습니다. 클라이언트 보고서의 목적지 관측 주소는 여전히 미확인이며 수신 로그를 자동 반환하거나 병합하지 않습니다.

컨테이너 실행:

```bash
docker run --rm -p 8080:8080 ghcr.io/uknowpro/netflow:v0.3.0 \
  serve --listen 0.0.0.0:8080 --duration 10m --output json
```

[Kubernetes 수신 예제](../examples/kubernetes-receiver.yaml)는 비루트 임시 Pod와 **ClusterIP** Service를 만듭니다. 그 자체로 외부 진입점은 아닙니다. 실제 검증할 네임스페이스·라벨·NetworkPolicy·진입 경로에 맞게 수정해야 하며 방화벽 규칙이나 공개 로드밸런서를 자동 생성하지 않습니다.

```bash
kubectl -n YOUR_NAMESPACE apply -f examples/kubernetes-receiver.yaml
kubectl -n YOUR_NAMESPACE logs -f pod/netflow-receiver
# 완료 후 임시 Pod와 Service 정리
kubectl -n YOUR_NAMESPACE delete -f examples/kubernetes-receiver.yaml
```

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
- UDP, 출발지 IP 위조, 방화벽 규칙 분석, 중앙 에이전트·컨트롤러, 결과 집계 서비스는 포함되지 않습니다.
- 커스텀 헤더, 인증, 환경변수 치환, mTLS, HTTP 메서드 선택, 항목별 시간 제한은 아직 지원하지 않습니다.
- URL은 JSON에 포함됩니다. 쿼리 문자열에 토큰을 넣지 마세요. URL 사용자 정보 형식의 인증 정보는 거부합니다.
- 보고서에는 내부 주소가 포함될 수 있습니다. `results/`, 로컬 설정 파일, `.env` 파일은 Git에서 제외합니다.

## 자동 배포와 설치

`v0.3.0` 같은 버전 태그를 푸시하면 테스트, **10개 OS·CPU 조합**의 실행 파일 패키징, 네이티브 실행 파일·컨테이너 동작 확인을 수행합니다. 이후 `ghcr.io/uknowpro/netflow:<버전>`에 멀티 플랫폼 이미지를 올리고, 압축 파일(Windows는 ZIP, 나머지는 tar.gz)과 `checksums.txt`가 첨부된 GitHub Release를 생성합니다. `v0.3.0-rc.1` 같은 태그는 사전 릴리스로 표시합니다. 이미지는 정확한 버전 태그만 제공하며 `latest`는 만들지 않습니다.

[GitHub Releases](https://github.com/uknowpro/netflow/releases)에 버전이 생성된 뒤 설치합니다.

```bash
# 실제 배포된 버전으로 변경하세요. ARM 환경에서는 arm64를 사용합니다.
VERSION=v0.3.0
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
bash scripts/build-release.sh v0.3.0
# 결과: dist/v0.3.0/ (Git 제외)
```

검토가 끝난 커밋을 배포할 때는 다음과 같이 실행합니다.

```bash
git tag -a v0.3.0 -m "netflow v0.3.0"
git push origin v0.3.0
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

CI는 포맷, 정적 검사, 경쟁 상태 검사 포함 테스트, CLI 빌드 및 컨테이너 빌드를 수행합니다. 버전 문자열은 `go build -ldflags="-X main.version=v0.3.0" -o bin/netflow ./cmd/netflow`로 지정할 수 있습니다.

## 향후 확장

- 프로토콜별 응답을 확인하는 UDP 검증과 판정 불가 결과
- 보고서에 비밀값을 노출하지 않는 인증·사설 PKI 설정
- 여러 출발지의 결과 집계와 선택적 원격 에이전트
- 실제 AKS·VM 경로 검증: 해당 네트워크에 접근할 수 있는 환경에서 별도 수행

## 라이선스

[MIT](../LICENSE)
