# servicemap

**English** · [한국어](#한국어)

Map running services, listening ports, and their proxy/connection relationships on Linux.

```bash
sudo servicemap
```

Output (tree format, default):

```
mariadb.service
  └─ listens :3306

myapp.service
  └─ listens :3000
  └─ connects to mariadb.service (:3306)

nginx.service
  └─ listens :80
  └─ proxies to myapp.service (:3000)

abcdef123456 [docker]
  └─ listens :8080
```

## Install

### From source

```bash
go install github.com/LeeSeokBln/servicemap/cmd/servicemap@latest
```

### From release tarball

Download the latest release from [GitHub Releases](https://github.com/LeeSeokBln/servicemap/releases), extract, and add to your PATH:

```bash
tar xzf servicemap_*_linux_amd64.tar.gz
sudo mv servicemap /usr/local/bin/
```

## Usage

```
servicemap [flags]

Flags:
  -f, --format FORMAT      output format: tree, mermaid, md, json (default "tree")
  -o, --output PATH        write output to file (infers format from extension)
  --all                    show all nodes (by default, plain processes that
                           neither listen nor are systemd/docker services are hidden)
  --version                print version
  --proc-root PATH         root of /proc for testing (default "/")
```

Exit codes:
- 0: success
- 1: error (bad flags, permission denied, etc.)

## Output formats

### Tree (default)

Tree view of services and their relationships:

```
mariadb.service
  └─ listens :3306

myapp.service
  └─ listens :3000
  └─ connects to mariadb.service (:3306)

nginx.service
  └─ listens :80, :443
  └─ proxies to myapp.service (:3000)

webapp [docker]
  └─ listens 127.0.0.1:8080
```

### Mermaid

Flowchart for visualization in documentation or Slack:

```mermaid
flowchart LR
    n0["mariadb.service<br/>:3306"]
    n1["myapp.service<br/>:3000"]
    n2["nginx.service<br/>:80, :443"]
    n3["webapp [docker]<br/>127.0.0.1:8080"]
    n1 -.->|connects :3306| n0
    n2 -->|proxies :3000| n1
```

### Markdown

Structured output for reports:

```markdown
# Service Map

| Service | Kind | Listens | Connections |
|---|---|---|---|
| mariadb.service | systemd | :3306 |  |
| myapp.service | systemd | :3000 | connects to mariadb.service (:3306) |
| nginx.service | systemd | :80, :443 | proxies to myapp.service (:3000) |
| webapp | docker | 127.0.0.1:8080 |  |

\`\`\`mermaid
flowchart LR
    n1 -.->|connects :3306| n0
    n2 -->|proxies :3000| n1
\`\`\`
```

### JSON

Complete graph in JSON for programmatic access:

```json
{
  "nodes": [
    {"id": "unit:mariadb.service", "name": "mariadb.service", "kind": "systemd", "listens": [":3306"], "pids": [300]},
    {"id": "unit:myapp.service", "name": "myapp.service", "kind": "systemd", "listens": [":3000"], "pids": [200]},
    {"id": "unit:nginx.service", "name": "nginx.service", "kind": "systemd", "listens": [":80", ":443"], "pids": [100, 101]}
  ],
  "edges": [
    {"from": "unit:myapp.service", "to": "unit:mariadb.service", "kind": "connects", "ports": [3306]},
    {"from": "unit:nginx.service", "to": "unit:myapp.service", "kind": "proxies", "ports": [3000]}
  ]
}
```

## How it works

servicemap has no daemon and requires no setup. It reads `/proc` socket tables (`/proc/net/tcp`, `/proc/net/udp`) to find listening ports, then matches them to running processes via `inodes`. For containers with separate network namespaces, it reads their `/proc/[pid]/net` socket tables too. Process-to-service mapping comes from systemd cgroups (`/proc/[pid]/cgroup`; unit names for systemd services, container IDs for Docker), and container names are resolved via the Docker socket. nginx proxy routes are extracted by parsing nginx config files (found via the nginx process's `-c` flag or defaulting to `/etc/nginx/nginx.conf`), matching `proxy_pass` directives to upstream `server` blocks and remote targets. UDP listens are shown but no UDP connection tracking is performed. By default, plain processes that neither listen nor are systemd/docker services are hidden (one-off clients, background tools); systemd/docker services are always shown. Use `--all` to display everything.

## Permissions

Running without `sudo` can read all listening ports from `/proc/net`, but cannot read other users' `/proc/[pid]/fd` directories — so services owned by other users lose their socket attribution and edges. The tool continues and prints a stderr warning with the count of uninspectable processes. For a complete map with all users' services:

```bash
sudo servicemap
```

## Limitations

- **nginx config parsing only** — other proxies (HAProxy, Envoy, Apache) are not yet parsed; their routes appear as external TCP connects only.
- **UDP connections not tracked** — UDP listening ports are shown, but no edge information about which process sends to which UDP service.
- **Docker published ports via DNAT** — ports exposed through Docker's port publishing (e.g. `-p 80:8080`) may show the `docker-proxy` process's bind instead of the container's internal port.
- **Hostname resolution** — nginx proxy targets given as hostnames that resolve to local IPs are shown as external nodes (not yet resolved to local services).

## License

MIT — see [LICENSE](LICENSE).

---

## 한국어

Linux에서 실행 중인 서비스, 리스닝 포트, 그리고 프록시/연결 관계를 지도화합니다.

```bash
sudo servicemap
```

출력 (기본값: tree 포맷):

```
mariadb.service
  └─ listens :3306

myapp.service
  └─ listens :3000
  └─ connects to mariadb.service (:3306)

nginx.service
  └─ listens :80
  └─ proxies to myapp.service (:3000)

abcdef123456 [docker]
  └─ listens :8080
```

### 설치

#### 소스에서 빌드

```bash
go install github.com/LeeSeokBln/servicemap/cmd/servicemap@latest
```

#### 릴리즈 타르볼

[GitHub Releases](https://github.com/LeeSeokBln/servicemap/releases)에서 최신 버전을 받아서 PATH에 추가합니다:

```bash
tar xzf servicemap_*_linux_amd64.tar.gz
sudo mv servicemap /usr/local/bin/
```

### 사용법

```
servicemap [플래그]

플래그:
  -f, --format FORMAT      출력 포맷: tree, mermaid, md, json (기본값 "tree")
  -o, --output PATH        출력을 파일에 저장 (확장자에서 포맷 자동 감지)
  --all                    모든 노드 표시 (기본값: systemd/docker 서비스를 제외한
                           순수 프로세스 노드는 숨김)
  --version                버전 출력
  --proc-root PATH         테스트용 /proc 루트 (기본값 "/")
```

종료 코드:
- 0: 성공
- 1: 에러 (잘못된 플래그, 권한 거부 등)

### 출력 포맷

#### Tree (기본값)

서비스와 관계를 트리로 표시:

```
mariadb.service
  └─ listens :3306

myapp.service
  └─ listens :3000
  └─ connects to mariadb.service (:3306)

nginx.service
  └─ listens :80, :443
  └─ proxies to myapp.service (:3000)

webapp [docker]
  └─ listens 127.0.0.1:8080
```

#### Mermaid

문서나 Slack에서 시각화할 수 있는 플로우차트:

```mermaid
flowchart LR
    n0["mariadb.service<br/>:3306"]
    n1["myapp.service<br/>:3000"]
    n2["nginx.service<br/>:80, :443"]
    n3["webapp [docker]<br/>127.0.0.1:8080"]
    n1 -.->|connects :3306| n0
    n2 -->|proxies :3000| n1
```

#### Markdown

리포트용 구조화된 출력:

```markdown
# Service Map

| Service | Kind | Listens | Connections |
|---|---|---|---|
| mariadb.service | systemd | :3306 |  |
| myapp.service | systemd | :3000 | connects to mariadb.service (:3306) |
| nginx.service | systemd | :80, :443 | proxies to myapp.service (:3000) |
| webapp | docker | 127.0.0.1:8080 |  |

\`\`\`mermaid
flowchart LR
    n1 -.->|connects :3306| n0
    n2 -->|proxies :3000| n1
\`\`\`
```

#### JSON

프로그래매틱 접근용 완전한 그래프:

```json
{
  "nodes": [
    {"id": "unit:mariadb.service", "name": "mariadb.service", "kind": "systemd", "listens": [":3306"], "pids": [300]},
    {"id": "unit:myapp.service", "name": "myapp.service", "kind": "systemd", "listens": [":3000"], "pids": [200]},
    {"id": "unit:nginx.service", "name": "nginx.service", "kind": "systemd", "listens": [":80", ":443"], "pids": [100, 101]}
  ],
  "edges": [
    {"from": "unit:myapp.service", "to": "unit:mariadb.service", "kind": "connects", "ports": [3306]},
    {"from": "unit:nginx.service", "to": "unit:myapp.service", "kind": "proxies", "ports": [3000]}
  ]
}
```

### 작동 원리

servicemap는 데몬이 없고 사전 설정이 필요 없습니다. `/proc` 소켓 테이블(`/proc/net/tcp`, `/proc/net/udp`)을 읽어서 리스닝 포트를 찾고, inode를 통해 실행 중인 프로세스와 매칭합니다. 컨테이너가 별도의 네트워크 네임스페이스를 사용하면 `/proc/[pid]/net` 소켓 테이블도 읽습니다. 프로세스-서비스 매핑은 systemd cgroup(`/proc/[pid]/cgroup`에서 systemd 서비스 이름이나 Docker 컨테이너 ID)으로 결정되며, 컨테이너 이름은 Docker 소켓을 통해 조회됩니다. nginx 프록시 경로는 nginx 설정 파일을 파싱해서 추출합니다 (nginx 프로세스의 `-c` 플래그로 찾거나 기본값 `/etc/nginx/nginx.conf`). `proxy_pass` 지시문을 upstream `server` 블록 및 원격 대상과 매칭합니다. UDP 리스닝은 표시되지만 UDP 연결 추적은 하지 않습니다. 기본적으로 리스닝도 하지 않고 systemd/docker 서비스도 아닌 순수 프로세스 노드(한 번 실행되는 클라이언트, 백그라운드 도구)는 숨겨집니다. systemd/docker 서비스는 나가는 연결만 있어도 항상 표시됩니다. `--all`을 사용하면 모든 노드를 표시합니다.

### 권한

`sudo` 없이 실행하면 `/proc/net`에서 모든 리스닝 포트는 읽을 수 있지만, 다른 사용자의 `/proc/[pid]/fd` 디렉토리는 읽을 수 없습니다. 따라서 다른 사용자가 소유한 서비스는 소켓 속성을 잃고 엣지 정보가 사라집니다. 도구는 계속 실행되며 읽을 수 없는 프로세스 개수를 stderr에 경고로 출력합니다. 모든 사용자의 서비스를 포함한 완전한 지도를 보려면:

```bash
sudo servicemap
```

### 제한사항

- **nginx 설정만 파싱** — HAProxy, Envoy, Apache 등 다른 프록시는 아직 지원하지 않습니다. 이들의 경로는 외부 TCP 연결로만 표시됩니다.
- **UDP 연결 추적 안 함** — UDP 리스닝 포트는 표시되지만, 어느 프로세스가 어느 UDP 서비스로 보내는지에 대한 엣지 정보는 없습니다.
- **Docker 포트 포워딩(DNAT)** — Docker 포트 포워딩(예: `-p 80:8080`)으로 노출된 포트는 컨테이너의 내부 포트가 아닌 `docker-proxy` 프로세스의 바인드를 표시할 수 있습니다.
- **호스트명 해석** — nginx 프록시 타겟이 로컬 IP로 해석되는 호스트명인 경우, 로컬 서비스로 해석되지 않고 외부 노드로 표시됩니다.

### 라이선스

MIT — [LICENSE](LICENSE) 참조.
