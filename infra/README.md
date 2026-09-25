# infra/

Manifestos de infraestrutura do workspace, modulares: um recurso por arquivo, compostos por ambiente. As configurações dos componentes de observabilidade são **fonte única** — o Compose as monta por bind e o Kustomize as gera como ConfigMap a partir do mesmo arquivo (por isso elas vivem ao lado dos manifestos: o Kustomize recusa arquivo fora do diretório do kustomization).

```text
infra/
├── local/                          # desenvolvimento local (Docker Compose)
│   ├── docker-compose.yml          # só `name` + `include:` dos recursos
│   ├── .env.example                # todas as variáveis e portas
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
│       ├── alloy.yml               # profile alloy (coleta de logs)
│       ├── grafana.yml             # profile grafana
│       ├── exporters.yml           # profile exporters (postgres, redis, blackbox, cAdvisor)
│       ├── redpanda-console.yml    # profile console (+ ../redpanda-console.yaml): UI do Kafka, autenticado por SASL
│       ├── swagger-ui.yml          # profile dmpf: Swagger UI sobre as duas specs do BFF
│       └── reference.yml          # profile dmpf: BFF, orders e reservations, postgres-init, redpanda-init
├── observability/                  # config + manifestos K8s, um diretório por componente
│   ├── kustomization.yaml          # agrega os seis
│   ├── otel-collector/             # config.yaml, Deployment, Service
│   ├── prometheus/                 # prometheus.yaml, blackbox.yaml, StatefulSet, Service
│   ├── loki/                       # loki.yaml, StatefulSet, Service
│   ├── tempo/                      # tempo.yaml, StatefulSet, Service
│   ├── alloy/                      # config do Docker e do K8s, DaemonSet, RBAC
│   └── grafana/                    # datasources, provider e dashboards, Deployment, Service
├── k8s/                            # deploy (Kustomize)
│   ├── base/{bff,orders,reservations,postgres,redpanda}/
│   └── overlays/{dev,hmg}/
└── docker/Dockerfile.node.example  # referência para apps Node
```

## Desenvolvimento local (Compose)

Os profiles são cumulativos. `observability` sobe a plataforma inteira; `dmpf` sobe os oito processos da topologia de referência — o BFF, `api` e `relay` de orders, `api`, `relay` e `consumer` de reservations, `api` e `relay` de bookings — **com** tudo o que eles precisam e observam, inclusive o `postgres-init` (cria banco e role de mesmo nome para `orders`, `reservations` e `bookings`, com o banco pertencendo ao role, sem falhar quando já existem), o `redpanda-init` (usuários SASL, tópicos e ACLs) e o `pki-init` (CA e certificados do mTLS interno); `all` sobe a infraestrutura toda menos as apps.

```bash
# só o banco (o que os testes de integração dos módulos Go precisam)
docker compose -f infra/local/docker-compose.yml --profile postgres up -d

# a plataforma de observabilidade e os exporters
pnpm nx run bff:observability-up

# os oito processos com dependências, exporters e painéis (constrói as quatro imagens)
docker compose -f infra/local/docker-compose.yml --profile dmpf up -d --build

# derrubar (os volumes ficam; `-v` apaga os dados)
pnpm nx run bff:infra-down
```

Pelo Nx: `infra-up` (Postgres, Redpanda e floci), `observability-up` (plataforma + exporters), `infra-down` e `infra-budget`.

### Bancos por app

O servidor Postgres tem um usuário só administrativo (`POSTGRES_USER`, padrão `postgres`). Cada app tem banco e role com o próprio nome, e só as estruturas do próprio schema: `orders` (`outbox` e `orders`), `reservations` (`outbox`, `inbox`, `quarantine` e `reservations`) e `bookings` (`outbox`, `bookings` e `resources`). Cada app conecta com o próprio role:

| App | `PG_DSN` |
|---|---|
| `orders` | `postgres://orders:${ORDERS_PG_PASSWORD:-orders-local}@postgres:5432/orders` |
| `reservations` | `postgres://reservations:${RESERVATIONS_PG_PASSWORD:-reservations-local}@postgres:5432/reservations` |
| `bookings` | `postgres://bookings:${BOOKINGS_PG_PASSWORD:-bookings-local}@postgres:5432/bookings` |

Os testes de integração dos módulos Go apontam `PG_DSN` para o servidor, com um usuário que pode criar bancos (`postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable`): cada projeto cria e usa o próprio `<projeto>_test`.

Não há migração a partir do layout antigo (banco `app` compartilhado e bancos e tabelas com o prefixo `dmpf`). Um volume criado antes precisa ser recriado, e o do PKI também, porque o `pki-init` só emite certificados na primeira subida:

```bash
docker compose -f infra/local/docker-compose.yml --profile dmpf down -v
docker compose -f infra/local/docker-compose.yml --profile dmpf up -d --build
```

### mTLS interno e SASL no Kafka (profile `dmpf`)

O `pki-init` gera, num volume nomeado e só na primeira subida, uma CA de desenvolvimento, o certificado de servidor de cada `api` (`dmpf-orders-api`, `dmpf-reservations-api`, `dmpf-bookings-api`) e o certificado de cliente do BFF, com a URI `spiffe://dmpf/bff` no SAN. Os `api` exigem e verificam esse certificado (`GRPC_CLIENT_CA_FILE`, `GRPC_TRUSTED_CLIENTS`); o BFF apresenta o seu (`GRPC_CA_FILE`, `GRPC_CLIENT_CERT_FILE`/`_KEY_FILE`) e autentica localmente por `AUTH_DEV_MOCK=true` — sem verificação real de identidade, só para o desenvolvimento (ver `apps/backend/bff/README.md`). O listener Kafka interno (`redpanda:29092`) exige SASL SCRAM-SHA-256; o `redpanda-init` cria um principal por processo (`orders`, `reservations`, `bookings`, `console`) e as ACLs por principal do ADR-052 — cada um só escreve no próprio tópico e na DLQ que alimenta, e só `reservations` lê `orders.events`. O listener externo (`localhost:${REDPANDA_PORT:-9092}`), que os testes de integração dos módulos Go usam, continua sem autenticação — é o mesmo cluster, alcançável por qualquer container da rede, e o principal desse listener é superusuário; serve só ao desenvolvimento local.

### Portas no host

Tudo publica em `0.0.0.0`, então os endereços valem no WSL (`localhost`) e no Windows (`localhost`, pelo encaminhamento do Docker Desktop; pelo IP do WSL quando o encaminhamento não estiver ativo — `hostname -I`).

| Recurso | Porta | Para quê |
| --- | --- | --- |
| **Grafana** | **3000** | painéis, Explore, correlação log ↔ trace; login `admin`/`GRAFANA_ADMIN_PASSWORD` para a administração do servidor |
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

Publicar em `0.0.0.0` tem uma consequência, além de servir o Windows: pelo IP do WSL, quem estiver na mesma rede alcança o Grafana, o Redis sem senha, a Admin API do Redpanda e o `/-/quit` do Prometheus. É uma escolha de desenvolvimento local; numa rede compartilhada, publique em `127.0.0.1` editando a porta (`'127.0.0.1:3000:3000'`) — o Docker Desktop continua encaminhando `localhost` do Windows.

### Acesso pelo BFF

`GET /openapi/orders/v1/openapi.yaml` e `GET /openapi/reservations/v1/openapi.yaml` devolvem os contratos publicados (`contracts/openapi/`, copiados para a imagem do BFF); o Swagger UI em `:8082` os lista no seletor da barra superior pela variável `URLS` da imagem. Como o "Try it out" chama o BFF de outro origin, o compose passa `CORS_ORIGINS=http://localhost:8082,...` ao BFF — fora do compose a variável fica vazia e a borda é same-origin. O BFF autentica com `AUTH_DEV_MOCK=true` (lê a identidade do próprio Bearer, sem verificação) e fala com os contextos por gRPC sob mTLS (`dns:///dmpf-orders-api:9090` e `dns:///dmpf-reservations-api:9090`), com o certificado de cliente e a CA que o `pki-init` gera; os `api` não publicam porta no host.

### Orçamento de recursos

Todo serviço declara `deploy.resources` com teto (`limits`) e mínimo (`reservations`). O teto é do **conjunto**: `tools/infra-budget.sh` soma o que o `docker compose config` resolve e reprova se passar de 60% do host (`INFRA_BUDGET_FRACTION` muda a fração).

```bash
pnpm nx run bff:infra-budget
```

A soma dos tetos dá **4,80 vCPU** e **8,53 GiB** com os 27 serviços: os oito processos da topologia somam 1,00 vCPU (0,15 no BFF e em cada `api`, 0,10 em cada relay e no consumer), e `postgres-init`, `redpanda-init` e `pki-init` ficam em 0,05–0,10 vCPU cada, todos de execução única. O teto é relativo ao host: num host de 8 vCPU os 60% (4,80 vCPU) comportam a soma sem folga, no limite. O maior teto é do Redpanda (0,60 vCPU / 1,5 GiB, com `--memory=1G` no próprio broker); os exporters ficam em 0,05 vCPU / 64 MiB cada. O uso real em regime é da ordem de 0,15 vCPU e 1,2 GiB — os tetos protegem o host, não dimensionam o normal.

### O que observa o quê

```text
bff, contextos ──OTLP──> Collector ──traces──> Tempo ──remote write──> Prometheus <── scrape ── exporters
      │                      └────métricas────> Prometheus                                      (postgres, redis,
      └──stdout (JSON)──> Alloy ──> Loki                                                         blackbox, cAdvisor)
                                                                        Redpanda ──/public_metrics──┘
```

- **Traces**: a app exporta OTLP/gRPC para o Collector, que repassa ao Tempo. A propagação é W3C Trace Context (`traceparent` e `tracestate`) — o kernel recusa outro propagador (`ErrPropagatorNotW3C`), e o Collector declara `propagators: [tracecontext, baggage]`.
- **Métricas**: as do kernel chegam ao Prometheus pelo receptor OTLP nativo (`--web.enable-otlp-receiver`), com `service.name`, `service.version` e `service.instance.id` promovidos a label. O Tempo deriva `traces_spanmetrics_*` dos spans e as escreve por remote write.
- **Logs**: o kernel escreve JSON em stdout; o Alloy segue os containers pelo socket do Docker, extrai `level` e `service` como label e o `trace_id` como metadado estruturado — é o que liga log a trace no Grafana. A trilha de auditoria sai no mesmo envelope (`kind: "audit"`, com `trace_id` e `correlation_id` da requisição) por um sink próprio, não amostrado; os erros de exportação do SDK OpenTelemetry passam pelo mesmo handler de log, em vez do `log.Printf` cru. Um registro de partida tem `trace_id` e `correlation_id` vazios por natureza — não há requisição.
- **Recursos sistêmicos**: Redpanda expõe métricas Prometheus nativas em `:9644/public_metrics`; Postgres e Redis têm exporter próprio; cAdvisor cobre CPU, memória e rede de todos os containers; o floci não expõe métricas, então o blackbox mede disponibilidade e latência do endpoint de saúde.

O dashboard `DMPF — topologia de referência (BFF, orders, reservations)` já vem provisionado com as três séries de serviço do kernel (MET-08, MET-09, MET-10), admissão, retentativas, spans, traces e logs, filtráveis pelos três serviços em `$service`.

## Deploy (Kustomize)

```bash
kubectl kustomize infra/k8s/overlays/dev     # renderizar sem cluster
pnpm nx run bff:k8s-render     # renderiza os dois overlays
kubectl apply -k infra/k8s/overlays/dev
```

| Overlay | Namespace | O que sobe | Credenciais |
| --- | --- | --- | --- |
| `dev` | `dmpf-dev` | Postgres, Redpanda, a plataforma de observabilidade e as três apps; Job `dmpf-databases` cria banco e role de `orders`, `reservations` e `bookings`; Kafka, OTLP e gRPC interno sem TLS, `MIGRATE=true` nos `api`, Grafana anônimo | `secretGenerator` com valores de desenvolvimento (`orders`, `reservations`, `dmpf-databases`, `grafana-admin`) |
| `hmg` | `dmpf-hmg` | Observabilidade e as três apps (2 réplicas do BFF e de cada `api`); bancos e Kafka externos, mTLS no gRPC interno e SASL no Kafka, Grafana só com login | `orders`, `reservations` (com `KAFKA_SASL_USERNAME`/`_PASSWORD`), `orders-grpc-tls`, `reservations-grpc-tls`, `bff-grpc-ca`, `bff-grpc-client`, `grpc-client-ca`, `bff-oidc` e `grafana-admin` vêm de ExternalSecret/SealedSecret com esses nomes; `secrets.example.yaml.tmpl` mostra a forma e **não** é resource |

As bases são fail-closed e o overlay `dev` relaxa o que precisa: `KAFKA_INSECURE`, `OTLP_INSECURE` e o acesso anônimo do Grafana nascem desligados, e o transporte gRPC interno não tem default — `dev` declara `GRPC_INSECURE=true` por patch. Em `hmg`, o mTLS é bidirecional: cada `api` monta o próprio certificado de servidor (`orders-grpc-tls`, `reservations-grpc-tls`) e a CA que verifica o cliente (`grpc-client-ca`, comum aos dois contextos porque o único cliente confiável é o BFF), e o BFF monta a CA que verifica os `api` (`bff-grpc-ca`) e o próprio certificado de cliente (`bff-grpc-client`); o Kafka gerenciado exige SASL SCRAM-SHA-512 por contexto, com usuário e senha no Secret do próprio contexto. A ACL do broker por principal do ADR-052 é **pré-requisito externo**, registrada como comentário no `secrets.example.yaml.tmpl`: sem ela, `hmg` não tem como impor que só `orders` publique em `orders.events`. As bases dos contextos não conhecem credencial: `PG_DSN`, `KAFKA_BROKERS` e as credenciais SASL vêm sempre do Secret do overlay; o resto vem do ConfigMap. Os seis Deployments têm `securityContext` restritivo (não root, sistema de arquivos só leitura, sem capabilities) e `terminationGracePeriodSeconds` acima do prazo interno de encerramento de cada papel.

A borda pública (o BFF) autentica o sujeito por OIDC ou pelo mock de desenvolvimento (`apps/backend/bff/README.md`): `dev` declara `AUTH_DEV_MOCK=true` no ConfigMap, e `hmg` lê `OIDC_ISSUER`, `OIDC_AUDIENCE` e `OIDC_TENANT_CLAIM` do Secret `bff-oidc`. O acesso à rede é decidido por `NetworkPolicy`: a do BFF só admite ingress de pods rotulados `dmpf/api-client: "true"`; a de cada contexto só admite ingress na 9090 do `api` vindo de pods do BFF e nega todo ingress a `relay` e `consumer`. Os seletores de peer usam `matchExpressions` porque o Kustomize injeta o label `app.kubernetes.io/name` da base em todo `matchLabels` de NetworkPolicy. O Alloy roda como DaemonSet e lê os logs dos pods pela API do kubelet, com RBAC de `pods`, `pods/log` e `namespaces` — sem `hostPath` e sem `nodes/proxy`.

Limitações declaradas: o BFF não expõe rota de saúde (as sondas são de socket TCP); os `api` usam a sonda gRPC nativa do kubelet em `dev`, que não fala TLS, então em `hmg` — com mTLS ativo — voltam à sonda de socket; Postgres, Redpanda, Loki, Tempo e Prometheus das bases são de desenvolvimento (um pod, sem operador, sem TLS, armazenamento local) e em produção viram serviços gerenciados; não há Ingress nem a malha completa de NetworkPolicy (Prometheus → alvos, Grafana → datasources, Alloy → Loki), que dependem do cluster de destino e estão registradas como dívida no ADR-041; a configuração de autenticação do BFF (acima) é a mesma classe de dívida.
