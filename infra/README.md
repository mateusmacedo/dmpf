# infra/

Manifestos da plataforma do workspace, modulares: um recurso por arquivo, compostos por ambiente. O que é de cada app — Kubernetes, Compose, `.env.example` e o `infra.json` com banco, tópicos, ACLs e certificado — vive em `apps/backend/<app>/deploy/` (ADR-054); a parte agregada (provisionamento, PKI, Swagger UI, `local/.env.example`, listas dos overlays e o Job de bancos do `dev`) é gerada por `go run ./tools/dmpf-conformance/cmd/infrasync --root . --write` e conferida no CI com `--check`. As configurações dos componentes de observabilidade são **fonte única** — o Compose as monta por bind e o Kustomize as gera como ConfigMap a partir do mesmo arquivo (por isso elas vivem ao lado dos manifestos: o Kustomize recusa arquivo fora do diretório do kustomization).

```text
infra/
├── local/                          # desenvolvimento local (Docker Compose)
│   ├── docker-compose.yml          # projeto `lidercap-local`: só `name` + `include:` dos recursos e dos deploy/compose.yml das apps
│   ├── .env.example                # gerado pelo infrasync: variáveis da plataforma e das apps
│   └── compose/                    # um arquivo por recurso
│       ├── postgres.yml            # profile postgres
│       ├── redis.yml               # profile redis
│       ├── redpanda.yml            # profile redpanda (Kafka + admin/métricas em 9644); listener interno exige SASL
│       ├── pki.yml                 # profile dmpf: CA de desenvolvimento e certificados do mTLS gRPC interno (ADR-052)
│       ├── floci.yml               # profile floci (SQS e SNS)
│       ├── otel-collector.yml      # profile otel
│       ├── prometheus.yml          # profile prometheus
│       ├── loki.yml                # profile loki
│       ├── tempo.yml               # profile tempo
│       ├── alloy.yml               # profile alloy (logs dos containers de infraestrutura)
│       ├── grafana.yml             # profile grafana
│       ├── exporters.yml           # profile exporters (postgres, redis, blackbox, cAdvisor)
│       ├── redpanda-console.yml    # profile console (+ ../redpanda-console.yaml): UI do Kafka, autenticado por SASL
│       ├── app-base.yml            # serviços-base que o deploy/compose.yml de cada app estende
│       ├── provisioning.generated.yml  # gerado: postgres-init e redpanda-init a partir dos infra.json
│       ├── k6.yml                  # profile load: gerador de carga k6 (apps/backend/load)
│       └── swagger-ui.yml          # gerado: Swagger UI sobre as specs publicadas pelo BFF
├── observability/                  # config + manifestos K8s, um diretório por componente
│   ├── kustomization.yaml          # agrega os seis
│   ├── otel-collector/             # config.yaml, Deployment, Service, NetworkPolicy
│   ├── prometheus/                 # prometheus.yaml, blackbox.yaml, StatefulSet, Service
│   ├── loki/                       # loki.yaml, StatefulSet, Service
│   ├── tempo/                      # tempo.yaml, StatefulSet, Service
│   ├── alloy/                      # config do Docker e do K8s, DaemonSet, RBAC
│   └── grafana/                    # datasources, providers por pasta, dashboards/<pasta>/, Deployment, Service
├── k8s/                            # deploy (Kustomize)
│   ├── base/{postgres,redpanda}/
│   └── overlays/{dev,hmg}/         # compõem apps/backend/<app>/deploy/k8s/overlays/<env>; listas geradas
│                                   # hmg: TLS do receiver OTLP do Collector (otel-collector-tls.yaml, otel-collector-patch.yaml)
├── test/compose.yml                # infra dos testes de integração (projeto `lidercap-testinfra`, só tmpfs)
└── docker/Dockerfile.node.example  # referência para apps Node
```

## Desenvolvimento local (Compose)

Os profiles são cumulativos. `observability` sobe a plataforma inteira; `dmpf` sobe os oito processos da topologia de referência — o BFF, `api` e `relay` de orders, `api`, `relay` e `consumer` de reservations, `api` e `relay` de bookings — **com** tudo o que eles precisam e observam, inclusive o `postgres-init` (cria banco e role de mesmo nome para `orders`, `reservations` e `bookings`, com o banco pertencendo ao role, sem falhar quando já existem), o `redpanda-init` (usuários SASL, tópicos e ACLs) e o `pki-init` (CA e certificados do mTLS interno); `all` sobe a infraestrutura toda menos as apps.

```bash
# tudo o que o runtime local precisa, mais os processos das apps no host; parar o serve derruba a infra
pnpm nx run bff:serve

# a plataforma de observabilidade e os exporters
pnpm nx run bff:observability-up

# os oito processos com dependências, exporters e painéis (constrói as quatro imagens)
docker compose -f infra/local/docker-compose.yml --profile dmpf up -d --build

# derrubar (os volumes ficam; `-v` apaga os dados)
pnpm nx run bff:infra-down
```

Pelo Nx: `infra-up` (Postgres, Redpanda e floci), `observability-up` (plataforma + exporters), `infra-down` e `infra-budget`.

O profile `load` tem só o gerador de carga k6 (`compose/k6.yml`), fora de `all` e `dmpf` e, por isso, fora do `infra-budget`. Ele roda pelos targets do projeto `load` (`pnpm nx run load:<perfil>`) com a topologia `dmpf` já no ar; ver `apps/backend/load/README.md`.

O `deploy/compose.yml` de cada app não roda sozinho: ele estende os serviços-base de `compose/app-base.yml` e depende de serviços definidos em outros arquivos (`pki-init`, `postgres-init`, `otel-collector` e o `api` dos contextos que chama). Suba-o sempre pelo `include` de `infra/local/docker-compose.yml`.

### Bancos por app

O servidor Postgres tem um usuário só administrativo (`POSTGRES_USER`, padrão `postgres`). Cada app tem banco e role com o próprio nome, e só as estruturas do próprio schema: `orders` (`outbox` e `orders`), `reservations` (`outbox`, `inbox`, `quarantine` e `reservations`) e `bookings` (`outbox`, `bookings` e `resources`). Cada app conecta com o próprio role:

| App | `PG_DSN` |
|---|---|
| `orders` | `postgres://orders:${ORDERS_PG_PASSWORD:-orders-local}@postgres:5432/orders` |
| `reservations` | `postgres://reservations:${RESERVATIONS_PG_PASSWORD:-reservations-local}@postgres:5432/reservations` |
| `bookings` | `postgres://bookings:${BOOKINGS_PG_PASSWORD:-bookings-local}@postgres:5432/bookings` |

Os testes de integração não usam esta infra. Eles rodam na infra de testes (`infra/test/compose.yml`, `pnpm nx run testkit:test-infra-up`), com Postgres, Redpanda e floci em tmpfs nas portas 15432, 19092, 19093 e 14566; o `.env.example` da raiz aponta para ela, e cada teste ganha um banco `<projeto>_test_<id>`, apagado ao fim. O `tb/pg` recusa qualquer servidor cujo `cluster_name` não seja `test`, então um teste nunca escreve na infra de runtime local.

Não há migração a partir do layout antigo (banco `app` compartilhado e bancos e tabelas com o prefixo `dmpf`). Um volume criado antes precisa ser recriado, e o do PKI também, porque o `pki-init` só emite certificados na primeira subida:

```bash
docker compose -f infra/local/docker-compose.yml --profile dmpf down -v
docker compose -f infra/local/docker-compose.yml --profile dmpf up -d --build
```

### mTLS interno e SASL no Kafka (profile `dmpf`)

O `pki-init` gera, num volume nomeado e só na primeira subida, uma CA de desenvolvimento, o certificado de servidor de cada `api` (`dmpf-orders-api`, `dmpf-reservations-api`, `dmpf-bookings-api`) e o certificado de cliente do BFF, com a URI `spiffe://dmpf/bff` no SAN. Os `api` exigem e verificam esse certificado (`GRPC_CLIENT_CA_FILE`, `GRPC_TRUSTED_CLIENTS`); o BFF apresenta o seu (`GRPC_CA_FILE`, `GRPC_CLIENT_CERT_FILE`/`_KEY_FILE`) e autentica localmente por `AUTH_DEV_MOCK=true` — sem verificação real de identidade, só para o desenvolvimento (ver `apps/backend/bff/README.md`). O listener Kafka interno (`redpanda:29092`) exige SASL SCRAM-SHA-256; o `redpanda-init` cria um principal por processo (`orders`, `reservations`, `bookings`, `console`) e as ACLs por principal do ADR-052 — cada um só escreve no próprio tópico e na DLQ que alimenta, e só `reservations` lê `orders.events`. O listener externo (`localhost:${REDPANDA_PORT:-9092}`), que os testes de integração dos módulos Go usam, continua sem autenticação — é o mesmo cluster, alcançável por qualquer container da rede, e o principal desse listener é superusuário; serve só ao desenvolvimento local.

### Portas no host

A infraestrutura publica em `127.0.0.1`: os endereços valem no WSL (`localhost`) e no Windows (`localhost`, pelo encaminhamento do Docker Desktop), não pelo IP do WSL nem de outra máquina da rede. Swagger UI e dmpf-bff ainda publicam em `0.0.0.0`.

| Recurso | Porta | Para quê |
| --- | --- | --- |
| **Grafana** | **3000** | painéis, Explore, correlação log ↔ trace; sem acesso anônimo: o login `admin`/`admin` (senha trocável por `GRAFANA_ADMIN_PASSWORD`) abre o Explore, a edição de painéis e a administração do servidor |
| Swagger UI | 8082 | os contratos de `orders` e `reservations` com "Try it out" contra o BFF (profile `dmpf`) |
| Redpanda Console | 8083 | tópicos, grupos, mensagens e Admin API do Kafka (profile `console`), autenticado com o principal `console` |
| dmpf-bff `/openapi/{orders,reservations}/v1/openapi.yaml` | 8080 | os contratos servidos pelo BFF (`OPENAPI_ORDERS_PATH`, `OPENAPI_RESERVATIONS_PATH`) |
| Prometheus | 9090 | consulta PromQL, alvos de scrape |
| Loki | 3100 | API de logs |
| Tempo | 3200 | API de traces |
| Collector | 4317 / 8888 | OTLP/gRPC · métricas do próprio Collector |
| Alloy | 12345 | interface de componentes da coleta |
| dmpf-bff | 8080 | a única borda HTTP pública; os `api` de orders e reservations servem gRPC na 9090 só na rede do projeto, sob mTLS |
| Postgres | 5432 | banco |
| Redis | 6379 | cache |
| Redpanda | 9092 / 9644 | Kafka (listener externo, sem autenticação, só para o host) · admin e métricas Prometheus |
| floci | 4566 | SQS e SNS |
| cAdvisor | 8081 | métricas de container (8080 é do BFF) |
| postgres-exporter | 9187 | métricas do Postgres |
| redis-exporter | 9121 | métricas do Redis |
| blackbox-exporter | 9115 | probes de quem não expõe métricas |

Os valores mudam por `infra/local/.env` (ver `.env.example`).

Publicar em `127.0.0.1` fecha o que `0.0.0.0` abria pelo IP do WSL: quem estivesse na mesma rede alcançaria o Grafana, o Redis sem senha, a Admin API do Redpanda e o `/-/quit` do Prometheus. A porta interna não muda (`'127.0.0.1:${GRAFANA_PORT:-3000}:3000'`), e o Docker Desktop continua encaminhando `localhost` do Windows.

### Acesso pelo BFF

`GET /openapi/orders/v1/openapi.yaml` e `GET /openapi/reservations/v1/openapi.yaml` devolvem os contratos publicados (`apps/backend/<ctx>/contract/openapi/v1/`, copiados para a imagem do BFF); o Swagger UI em `:8082` os lista no seletor da barra superior pela variável `URLS` da imagem. Como o "Try it out" chama o BFF de outro origin, o compose passa `CORS_ORIGINS=http://localhost:8082,...` ao BFF — fora do compose a variável fica vazia e a borda é same-origin. O BFF autentica com `AUTH_DEV_MOCK=true` (lê a identidade do próprio Bearer, sem verificação) e fala com os contextos por gRPC sob mTLS (`dns:///dmpf-orders-api:9090` e `dns:///dmpf-reservations-api:9090`), com o certificado de cliente e a CA que o `pki-init` gera; os `api` não publicam porta no host.

### Orçamento de recursos

Todo serviço declara `deploy.resources` com teto (`limits`) e mínimo (`reservations`). O teto é do **conjunto**: `tools/infra-budget.sh` soma o que o `docker compose config` resolve e reprova se passar de 97% do host (`INFRA_BUDGET_FRACTION` muda a fração).

```bash
pnpm nx run bff:infra-budget
```

A soma dos tetos dá **7,70 vCPU** e **11,89 GiB** com os 27 serviços: os oito processos da topologia somam 1,53 vCPU (0,2808 no BFF e no `api` de orders, 0,195 nos `api` de reservations e bookings, 0,1872 no consumer e 0,13 em cada relay), e `postgres-init`, `redpanda-init` e `pki-init` ficam em 0,05–0,10 vCPU cada, todos de execução única. O teto é relativo ao host: num host de 8 vCPU os 97% (7,76 vCPU) comportam a soma, que fica em 96,3%. Os maiores tetos são o do Redpanda (0,84 vCPU / 1920 MiB, com `--memory=1280M` no próprio broker), o do Tempo (0,78624 vCPU / 1236 MiB), o do Collector (0,78624 vCPU / 592 MiB) e o do Prometheus (0,728 vCPU / 1649 MiB); o Grafana fica em 0,5616 vCPU / 1272 MiB e o Loki em 0,546 vCPU / 1236 MiB, e os exporters ficam em 0,065 vCPU / 74 MiB cada (0,0936 vCPU no `postgres-exporter`). O uso real em regime é da ordem de 0,15 vCPU e 1,2 GiB — os tetos protegem o host, não dimensionam o normal.

### O que observa o quê

```text
bff, contextos ──OTLP/gRPC──> Collector ──traces (tail_sampling)──> Tempo ──remote write──> Prometheus <── scrape ── exporters
                                ├──métricas──> Prometheus                                                            (postgres, redis,
                                └──logs──────> Loki <── Alloy <── containers de infraestrutura                       blackbox, cAdvisor)
                                                                                  Redpanda ──/public_metrics──┘
```

- **Traces**: a app exporta OTLP/gRPC para o Collector (`OTEL_EXPORTER_OTLP_*` com o protocolo `grpc` e o endpoint `http://otel-collector:4317`, em texto claro no Compose e em `dev`; em `hmg`, `https://otel-collector:4317` com a CA em `OTEL_EXPORTER_OTLP_CERTIFICATE`), com a cabeça do sampler em `OTEL_TRACES_SAMPLER_ARG=1.0` e o prazo de exportação de spans e logs em `OTEL_BSP_EXPORT_TIMEOUT=3000` e `OTEL_BLRP_EXPORT_TIMEOUT=3000` (ms), abaixo da fatia de cada sinal na janela de 10 s do encerramento (o default do SDK é 30000). O Collector é o `otel/opentelemetry-collector-contrib:0.160.0`, com uma réplica, e aplica `tail_sampling` antes de repassar ao Tempo: status `ERROR` e as classes `error`, `maintenance` e `write` de `dmpf.traffic_class` sempre; `read` e `unclassified` por `probabilistic`; e trace sem classe ou com classe fora da taxonomia à taxa de `unclassified`, para nunca ser descartado em silêncio (RF-E7). No Compose, `TAIL_SAMPLING_READ_PERCENTAGE` e `TAIL_SAMPLING_UNCLASSIFIED_PERCENTAGE` valem `100` (`compose/otel-collector.yml`); no Kubernetes, sem elas, `read` e `unclassified` ficam em 1%, a taxa de TRC-13. `write` a 100% mantém juntos os traces F1, F2 e F3 de uma escrita, e a réplica é única porque o `tail_sampling` decide sobre o trace inteiro, que precisa chegar a uma instância só. O `tail_sampling` segura cada trace na memória até o `decision_wait` (30 s), então o `memory_limiter` vem antes dele e é o primeiro processor dos três pipelines: `limit_percentage: 80` e `spike_limit_percentage: 25` sobre o teto do container (592 MiB e 0,78624 vCPU no Compose, 256 MiB no Kubernetes), com `GOMEMLIMIT` em 80% do mesmo teto (`473MiB` no Compose, `204MiB` no Kubernetes). Com o heap acima de 55% do teto, o Collector recusa dado novo com `Unavailable` (`data refused due to high memory usage`), que a app trata como falha de exportação, em vez de ser morto por OOM e perder de uma vez os traces em espera. O `batch` dos três pipelines fecha lotes de no máximo 2048 itens (`send_batch_size` e `send_batch_max_size`): sem esse teto, a liberação em bloco do `tail_sampling` e as retentativas do SDK sob recusa formavam lotes de 8195 spans, acima dos 4 MiB que o gRPC do Tempo aceita por mensagem, e o Tempo os recusava com `ResourceExhausted`, um erro permanente que descarta o lote. A propagação é W3C Trace Context (`traceparent` e `tracestate`): `OTEL_PROPAGATORS` só aceita `tracecontext` ou a ausência, outro valor recusa a partida (`ErrPropagatorNotW3C`), e o baggage nunca vai ao fio; o Collector declara `propagators: [tracecontext]` na própria telemetria.
- **Métricas**: as do kernel e das bibliotecas (`otelhttp`, `otelgrpc`, runtime Go, pool do Postgres) chegam ao Prometheus pelo receptor OTLP nativo (`--web.enable-otlp-receiver`), com `service.name`, `service.namespace`, `service.version` e `dmpf.process.role` promovidos a label e os nomes traduzidos com unidade e `_total` (`otlp.translation_strategy: UnderscoreEscapingWithSuffixes`): `http.server.request.duration` vira `http_server_request_duration_seconds`. O Tempo deriva `traces_spanmetrics_*` dos spans e as escreve por remote write.
- **Logs**: o log das apps sai só por OTLP, no OTel Logs Data Model (ADR-057): `slog` → `otelslog` → `sdk/log` → Collector → `otlp_http/loki` → `/otlp` do Loki. O `docker logs` de uma app mostra só o stderr, com a configuração recusada e o erro com que o processo termina. O `loki.yaml` não declara `otlp_config`, então vale a promoção default do Loki 3.7.7: do Resource das apps, só `service.name`, `service.instance.id` e `deployment.environment.name` viram label (`service_name`, `service_instance_id` e `deployment_environment_name`); o resto do Resource, o scope (`scope_name`), o `trace_id`, o `span_id` e os atributos do registro, como `dmpf_correlation_id`, ficam em structured metadata, com `_` no lugar de `.`, e o nível vem de `detected_level`. Nenhum identificador de correlação vira label, e não há label `level`, `service` nem `channel`. O Alloy coleta só os containers de infraestrutura, sem `stage.json`: descarta os serviços `dmpf-*` no Compose e os pods com `app.kubernetes.io/name` ∈ {`bff`, `orders`, `reservations`, `bookings`} no Kubernetes, e grava só os labels de origem (`container`, `compose_service`, `compose_project` e `platform` no Compose; `namespace`, `pod`, `container` e `app` no Kubernetes). A trilha de auditoria segue pelo mesmo caminho, com `EventName` `dmpf.audit` e o scope do pacote `observability/audit`, fora da amostragem de `LOG-12`; os erros de exportação do SDK OpenTelemetry passam pelo mesmo logger (`telemetry export failed`), em vez do `log.Printf` cru. Os registros de partida e de encerramento não têm `trace_id` nem as chaves de correlação, porque não há execução em curso.
- **Recursos sistêmicos**: Redpanda expõe métricas Prometheus nativas em `:9644/public_metrics`; Postgres e Redis têm exporter próprio; cAdvisor cobre CPU, memória e rede de todos os containers; o floci não expõe métricas, então o blackbox mede disponibilidade e latência do endpoint de saúde.

Os dashboards vêm provisionados de `observability/grafana/dashboards/<pasta>/`, uma pasta do Grafana por diretório, com `folderUid` fixo em `dashboards.yaml`:

| Pasta (`folderUid`) | Dashboard (`uid`) | Foco |
|---|---|---|
| DMPF · Plataforma (`dmpf-plataforma`) | Visão geral (`dmpf-overview`) | o que está de pé, borda, contextos, fluxo assíncrono, saturação e mapa de serviços |
| | Serviço (`reference`) | um serviço por vez, filtrável por `$service`, `$role` e `$instance`: borda, caso de uso (MET-08 a MET-10), saídas, consumo, pool, resiliência, runtime, traces e logs; o painel de logs exclui a trilha de auditoria pelo `scope_name` |
| DMPF · Kernel (`dmpf-kernel`) | Fluxo assíncrono (`dmpf-async`) | outbox → Kafka → inbox, com as disposições da inbox e o desfecho da idempotência por consulta de métrica sobre traces no Tempo |
| | Resiliência, admissão e contenção (`dmpf-resilience`) | breaker, retentativa, prazo, bulkhead, admissão e pool de banco |
| | Runtime Go e SDK OpenTelemetry (`dmpf-runtime`) | runtime do processo e pipeline do SDK (`otel.sdk.*`) |
| DMPF · Infra (`dmpf-infra`) | Redpanda (`dmpf-infra-redpanda`) e Pipeline de telemetria (`dmpf-infra-telemetry`) | broker; Collector, Loki, Tempo, Prometheus e Grafana |
| | Postgres (`dmpf-infra-postgres`) e Containers e serviços de apoio (`dmpf-infra-containers`) | só no Compose: o Kubernetes não tem postgres-exporter nem cAdvisor no scrape |
| DMPF · Carga (`dmpf-carga`) | Carga pela borda do BFF (`load-bff`) | só no Compose (ADR-055) |

O Compose monta `dashboards/` inteiro; o Kubernetes gera um ConfigMap por pasta (`grafana-dashboards-<pasta>`), montado no mesmo caminho, e `carga/` fica como diretório vazio. `tools/grafana-provisioning-check.sh` confere essa paridade, o PromQL pelo `promtool`, o datasource por UID, o vocabulário da spec e duas convenções de janela: as séries OTLP (exportadas a cada 60 s) e as do cAdvisor (amostras a cada 45 s, com `housekeeping_interval=30s`) usam `[$__rate_interval]` com `interval` mínimo de 2 min no painel, porque o Grafana resolve o `$__rate_interval` como o maior entre `interval` + `timeInterval` do datasource (15 s) e 4 × `timeInterval`, e a janela precisa cobrir duas amostras da fonte (o gate calcula essa janela efetiva por painel e reprova a que fica abaixo de duas vezes o intervalo da fonte); contador de evento raro soma as séries que nasceram na janela, porque o ponto que cria a série não entra no `increase`. Com `--live <url-do-prometheus> --from <iso> --to <iso>`, o gate executa as consultas contra um Prometheus no ar e lista os painéis vazios.

### Navegação entre sinais

Os datasources do Grafana (`observability/grafana/datasources.yaml`) ligam log, trace e correlação; o mesmo arquivo vale no Compose e no Kubernetes. O Grafana carrega só os plugins embutidos na imagem 13.2.1, entre eles os datasources Prometheus, Loki e Tempo: `GF_PLUGINS_PREINSTALL_DISABLED=true`, no Compose e no Deployment, desliga o preinstall, que a cada partida baixava do grafana.com versões mais novas desses datasources e os apps Drilldown; assim a partida não depende de rede e a versão de cada datasource é a da imagem. Os passos abaixo usam o Explore, que pede o login de administrador: `admin`/`admin` no Compose e no overlay `dev`.

1. **Log → trace.** No Explore do Loki, ao abrir um registro, o campo derivado `TraceID` lê o `trace_id` do structured metadata e abre o trace no Tempo. O `tail_sampling` só age no pipeline de traces: no Kubernetes, o log de uma leitura chega ao Loki, mas o trace dela pode faltar no Tempo, que guarda só 1% de `read` e `unclassified`.
2. **Log → traces da correlação.** O campo derivado `correlation_id` lê o `dmpf_correlation_id` e abre no Tempo `{ span.dmpf.correlation_id = "<v>" || span.messaging.message.conversation_id = "<v>" }`, que lista F1 (a requisição), F2 (a drenagem) e F3 (o consumo): o `send` e o `process` levam a correlação em `messaging.message.conversation_id`, e os spans abaixo do `process`, em `dmpf.correlation_id`.
3. **Trace → logs.** Na visão do trace, o link de logs de um span consulta o Loki com `{service_name="<serviço do span>"} | trace_id="<trace>" | scope_name != "github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"`, de 5 min antes do início a 5 min depois do fim do span (`tracesToLogsV2`). A trilha de auditoria fica de fora; para vê-la, filtre o Loki por esse `scope_name`.
4. **F1 → F2 e F3.** F1, F2 e F3 são traces distintos, ligados por link, nunca por parentesco (`TRC-08`). No Explore do Tempo, `{ link:traceID = "<trace id de F1>" }` devolve o trace `outbox drain <destino>` do relay, com um `send <tópico>` CLIENT por mensagem, e o trace `process <tópico>` CONSUMER do consumidor, porque o drain, o `send` e o `process` levam link ao contexto de criação gravado em F1. A correlação do passo 2 chega aos mesmos traces. Para seguir adiante, repita a busca com o trace id de F3: a outbox escrita durante o consumo grava o contexto do span ativo de F3, não o de F1.

## Deploy (Kustomize)

```bash
kubectl kustomize infra/k8s/overlays/dev     # renderizar sem cluster
pnpm nx run bff:k8s-render     # renderiza os dois overlays
kubectl apply -k infra/k8s/overlays/dev
```

| Overlay | Namespace | O que sobe | Credenciais |
| --- | --- | --- | --- |
| `dev` | `dmpf-dev` | Postgres, Redpanda, a plataforma de observabilidade e as quatro apps; Job `databases` cria banco e role de `orders`, `reservations` e `bookings`; Kafka, OTLP e gRPC interno sem TLS, `MIGRATE=true` nos `api`, Grafana só com login (`admin`/`admin`) | `secretGenerator` com valores de desenvolvimento (`orders`, `reservations`, `bookings`, `databases`, `grafana-admin`) |
| `hmg` | `dmpf-hmg` | Observabilidade e as quatro apps (2 réplicas do BFF e de cada `api`); bancos e Kafka externos, mTLS no gRPC interno e SASL no Kafka, Grafana só com login | `orders`, `reservations`, `bookings` (com `KAFKA_SASL_USERNAME`/`_PASSWORD`), `orders-grpc-tls`, `reservations-grpc-tls`, `bookings-grpc-tls`, `bff-grpc-ca`, `bff-grpc-client`, `grpc-client-ca`, `bff-oidc` e `grafana-admin` vêm de ExternalSecret/SealedSecret com esses nomes; `secrets.example.yaml.tmpl` mostra a forma e **não** é resource |

As bases são fail-closed e o overlay `dev` relaxa o que precisa: `KAFKA_INSECURE` nasce desligado, o acesso anônimo do Grafana fica desligado também em `dev`, e o transporte gRPC interno não tem default — `dev` declara `GRPC_INSECURE=true` por patch. A exportação OTLP das apps se configura só por `OTEL_*`: o ConfigMap da base aponta o endpoint de `OTEL_EXPORTER_OTLP_*` para `http://otel-collector:4317`, cujo esquema `http://` desliga o TLS em `dev`; em `hmg`, o patch do ConfigMap troca o endpoint por `https://otel-collector:4317` e declara `OTEL_EXPORTER_OTLP_CERTIFICATE`, e o receiver OTLP do Collector serve TLS com o certificado do Secret `otel-collector-tls`, montado em `/etc/otelcol-tls` e ligado por um segundo `--config` (`tls.yaml`, que o Collector funde ao `config.yaml` da base). O certificado vem da mesma CA dos certificados de servidor dos `api`, com `otel-collector` no SAN: os processos dos contextos o verificam pela CA do Secret `otel-collector-ca`, montada em `/etc/dmpf/otel`, e o BFF pela `bff-grpc-ca` que já monta (`/etc/dmpf/grpc/ca.crt`); `tools/otel-env-check.sh` reprova overlay `hmg` sem esse par e `https://` sem certificado no mesmo arquivo; o patch do overlay traz `service.version` e `deployment.environment.name` em `OTEL_RESOURCE_ATTRIBUTES`, e o Deployment acrescenta `service.instance.id` (o nome do pod) e `dmpf.process.role`. Em `hmg`, o mTLS é bidirecional: cada `api` monta o próprio certificado de servidor (`orders-grpc-tls`, `reservations-grpc-tls`) e a CA que verifica o cliente (`grpc-client-ca`, comum aos dois contextos porque o único cliente confiável é o BFF), e o BFF monta a CA que verifica os `api` (`bff-grpc-ca`) e o próprio certificado de cliente (`bff-grpc-client`); o Kafka gerenciado exige SASL SCRAM-SHA-512 por contexto, com usuário e senha no Secret do próprio contexto. A ACL do broker por principal do ADR-052 é **pré-requisito externo**, registrada como comentário no `secrets.example.yaml.tmpl`: sem ela, `hmg` não tem como impor que só `orders` publique em `orders.events`. As bases dos contextos não conhecem credencial: `PG_DSN`, `KAFKA_BROKERS` e as credenciais SASL vêm sempre do Secret do overlay; o resto vem do ConfigMap. Os seis Deployments têm `securityContext` restritivo (não root, sistema de arquivos só leitura, sem capabilities) e `terminationGracePeriodSeconds` acima do prazo interno de encerramento de cada papel: 30 s nos `api` e `relay` dos contextos e 35 s no `consumer`; no Compose, os serviços `dmpf-*` dos contextos declaram `stop_grace_period: 30s`.

A borda pública (o BFF) autentica o sujeito por OIDC ou pelo mock de desenvolvimento (`apps/backend/bff/README.md`): `dev` declara `AUTH_DEV_MOCK=true` no ConfigMap, e `hmg` lê `OIDC_ISSUER`, `OIDC_AUDIENCE` e `OIDC_TENANT_CLAIM` do Secret `bff-oidc`. O acesso à rede é decidido por `NetworkPolicy`: a do BFF só admite ingress de pods rotulados `dmpf/api-client: "true"`; a de cada contexto só admite ingress na 9090 do `api` vindo de pods do BFF e nega todo ingress a `relay` e `consumer`; a do Collector só admite a 4317 vinda dos pods das apps (`app.kubernetes.io/part-of: dmpf` com `app.kubernetes.io/component`, o que deixa Postgres e Redpanda de fora) e a 8888 vinda do Prometheus. Os seletores de peer usam `matchExpressions` porque o Kustomize injeta o label `app.kubernetes.io/name` da base em todo `matchLabels` de NetworkPolicy. O Alloy roda como DaemonSet e lê os logs dos pods de infraestrutura pela API do kubelet — os das quatro apps ele descarta pelo `app.kubernetes.io/name` —, com RBAC de `pods`, `pods/log` e `namespaces`, sem `hostPath` e sem `nodes/proxy`.

Limitações declaradas: o BFF não expõe rota de saúde (as sondas são de socket TCP); os `api` usam a sonda gRPC nativa do kubelet em `dev`, que não fala TLS, então em `hmg` — com mTLS ativo — voltam à sonda de socket; Postgres, Redpanda, Loki, Tempo e Prometheus das bases são de desenvolvimento (um pod, sem operador, sem TLS, armazenamento local) e em produção viram serviços gerenciados; não há Ingress nem a malha completa de NetworkPolicy (Prometheus → alvos, Grafana → datasources, Alloy → Loki), que dependem do cluster de destino e estão registradas como dívida no ADR-041; a configuração de autenticação do BFF (acima) é a mesma classe de dívida.
