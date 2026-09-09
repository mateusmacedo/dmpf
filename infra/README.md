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
│       ├── redpanda.yml            # profile redpanda (Kafka + admin/métricas em 9644)
│       ├── floci.yml               # profile floci (SQS e SNS)
│       ├── otel-collector.yml      # profile otel
│       ├── prometheus.yml          # profile prometheus
│       ├── loki.yml                # profile loki
│       ├── tempo.yml               # profile tempo
│       ├── alloy.yml               # profile alloy (coleta de logs)
│       ├── grafana.yml             # profile grafana
│       ├── exporters.yml           # profile exporters (postgres, redis, blackbox, cAdvisor)
│       ├── redpanda-console.yml    # profile console (+ ../redpanda-console.yaml): UI do Kafka
│       ├── swagger-ui.yml          # profile dmpf: Swagger UI sobre o /openapi.yaml da api
│       └── dmpf-reference.yml      # profile dmpf: api, relay, consumer
├── observability/                  # config + manifestos K8s, um diretório por componente
│   ├── kustomization.yaml          # agrega os seis
│   ├── otel-collector/             # config.yaml, Deployment, Service
│   ├── prometheus/                 # prometheus.yaml, blackbox.yaml, StatefulSet, Service
│   ├── loki/                       # loki.yaml, StatefulSet, Service
│   ├── tempo/                      # tempo.yaml, StatefulSet, Service
│   ├── alloy/                      # config do Docker e do K8s, DaemonSet, RBAC
│   └── grafana/                    # datasources, provider e dashboards, Deployment, Service
├── k8s/                            # deploy (Kustomize)
│   ├── base/{dmpf-reference,postgres,redpanda}/
│   └── overlays/{dev,hmg}/
└── docker/Dockerfile.node.example  # referência para apps Node
```

## Desenvolvimento local (Compose)

Os profiles são cumulativos. `observability` sobe a plataforma inteira; `dmpf` sobe os três papéis **com** tudo o que eles precisam e observam; `all` sobe a infraestrutura toda menos a app.

```bash
# só o banco (o que os testes de integração dos módulos Go precisam)
docker compose -f infra/local/docker-compose.yml --profile postgres up -d

# a plataforma de observabilidade e os exporters
pnpm nx run dmpf-reference-go:observability-up

# os três papéis com dependências, exporters e painéis (constrói a imagem)
docker compose -f infra/local/docker-compose.yml --profile dmpf up -d --build

# derrubar (os volumes ficam; `-v` apaga os dados)
pnpm nx run dmpf-reference-go:infra-down
```

Pelo Nx: `infra-up` (Postgres, Redpanda e floci), `observability-up` (plataforma + exporters), `infra-down` e `infra-budget`.

### Portas no host

Tudo publica em `0.0.0.0`, então os endereços valem no WSL (`localhost`) e no Windows (`localhost`, pelo encaminhamento do Docker Desktop; pelo IP do WSL quando o encaminhamento não estiver ativo — `hostname -I`).

| Recurso | Porta | Para quê |
| --- | --- | --- |
| **Grafana** | **3000** | painéis, Explore, correlação log ↔ trace; login `admin`/`GRAFANA_ADMIN_PASSWORD` para a administração do servidor |
| Swagger UI | 8082 | o contrato de `orders` com "Try it out" contra a api (profile `dmpf`) |
| Redpanda Console | 8083 | tópicos, grupos, mensagens e Admin API do Kafka (profile `console`) |
| dmpf-api `/openapi.yaml` | 8080 | o contrato servido pela própria api (`DMPF_OPENAPI_PATH`) |
| Prometheus | 9090 | consulta PromQL, alvos de scrape |
| Loki | 3100 | API de logs |
| Tempo | 3200 | API de traces |
| Collector | 4317 / 8888 | OTLP/gRPC · métricas do próprio Collector |
| Alloy | 12345 | interface de componentes da coleta |
| dmpf-api | 8080 | a borda HTTP de `orders` |
| Postgres | 5432 | banco |
| Redis | 6379 | cache |
| Redpanda | 9092 / 9644 | Kafka · admin e métricas Prometheus |
| floci | 4566 | SQS e SNS |
| cAdvisor | 8081 | métricas de container (8080 é do papel api) |
| postgres-exporter | 9187 | métricas do Postgres |
| redis-exporter | 9121 | métricas do Redis |
| blackbox-exporter | 9115 | probes de quem não expõe métricas |

Os valores mudam por `infra/local/.env` (ver `.env.example`).

Publicar em `0.0.0.0` tem uma consequência, além de servir o Windows: pelo IP do WSL, quem estiver na mesma rede alcança o Grafana, o Redis sem senha, a Admin API do Redpanda e o `/-/quit` do Prometheus. É uma escolha de desenvolvimento local; numa rede compartilhada, publique em `127.0.0.1` editando a porta (`'127.0.0.1:3000:3000'`) — o Docker Desktop continua encaminhando `localhost` do Windows.

### Acesso pela api

`GET /openapi.yaml` devolve o contrato publicado (`contracts/openapi/orders/v1/openapi.yaml`, copiado para a imagem); o Swagger UI em `:8082` o carrega dali. Como o "Try it out" chama a api de outro origin, o papel `api` do compose recebe `DMPF_CORS_ORIGINS=http://localhost:8082,...` — fora do compose a variável fica vazia e a borda é same-origin. A api valida o que o contrato publica: `id` e `X-Correlation-ID` em `[A-Za-z0-9._:-]{1,128}`, `sku` com 1 a 128 caracteres, `quantity ≥ 1`, um único objeto JSON sem campos desconhecidos; fora disso, `400` com `invalid-request` ou `malformed-body`.

### Orçamento de recursos

Todo serviço declara `deploy.resources` com teto (`limits`) e mínimo (`reservations`). O teto é do **conjunto**: `tools/infra-budget.sh` soma o que o `docker compose config` resolve e reprova se passar de 60% do host (`INFRA_BUDGET_FRACTION` muda a fração).

```bash
pnpm nx run dmpf-reference-go:infra-budget
```

Na máquina de referência (14 vCPU, 15,36 GiB) a soma dá **8,00 vCPU (57,1%)** e **7,9 GiB (50,5%)** com os 20 serviços. O maior teto é do Redpanda (1,25 vCPU / 1,5 GiB, com `--memory=1G` no próprio broker); os exporters ficam em 0,10 vCPU / 64 MiB cada. O uso real em regime é da ordem de 0,15 vCPU e 1,2 GiB — os tetos protegem o host, não dimensionam o normal.

### O que observa o quê

```text
dmpf-reference ──OTLP──> Collector ──traces──> Tempo ──remote write──> Prometheus <── scrape ── exporters
      │                      └────métricas────> Prometheus                                      (postgres, redis,
      └──stdout (JSON)──> Alloy ──> Loki                                                         blackbox, cAdvisor)
                                                                        Redpanda ──/public_metrics──┘
```

- **Traces**: a app exporta OTLP/gRPC para o Collector, que repassa ao Tempo. A propagação é W3C Trace Context (`traceparent` e `tracestate`) — o kernel recusa outro propagador (`ErrPropagatorNotW3C`), e o Collector declara `propagators: [tracecontext, baggage]`.
- **Métricas**: as do kernel chegam ao Prometheus pelo receptor OTLP nativo (`--web.enable-otlp-receiver`), com `service.name`, `service.version` e `service.instance.id` promovidos a label. O Tempo deriva `traces_spanmetrics_*` dos spans e as escreve por remote write.
- **Logs**: o kernel escreve JSON em stdout; o Alloy segue os containers pelo socket do Docker, extrai `level` e `service` como label e o `trace_id` como metadado estruturado — é o que liga log a trace no Grafana. A trilha de auditoria sai no mesmo envelope (`kind: "audit"`, com `trace_id` e `correlation_id` da requisição) por um sink próprio, não amostrado; os erros de exportação do SDK OpenTelemetry passam pelo mesmo handler de log, em vez do `log.Printf` cru. Um registro de partida (`api listening`) tem `trace_id` e `correlation_id` vazios por natureza — não há requisição.
- **Recursos sistêmicos**: Redpanda expõe métricas Prometheus nativas em `:9644/public_metrics`; Postgres e Redis têm exporter próprio; cAdvisor cobre CPU, memória e rede de todos os containers; o floci não expõe métricas, então o blackbox mede disponibilidade e latência do endpoint de saúde.

O dashboard `DMPF — composition root de referência` já vem provisionado com as três séries de serviço do kernel (MET-08, MET-09, MET-10), admissão, retentativas, spans, traces e logs.

## Deploy (Kustomize)

```bash
kubectl kustomize infra/k8s/overlays/dev     # renderizar sem cluster
pnpm nx run dmpf-reference-go:k8s-render     # renderiza os dois overlays
kubectl apply -k infra/k8s/overlays/dev
```

| Overlay | Namespace | O que sobe | Credenciais |
| --- | --- | --- | --- |
| `dev` | `dmpf-dev` | Postgres, Redpanda, a plataforma de observabilidade e a app; Kafka e OTLP sem TLS, `DMPF_MIGRATE=true` na api, Grafana anônimo | `secretGenerator` com valores de desenvolvimento (`dmpf-reference`, `grafana-admin`) |
| `hmg` | `dmpf-hmg` | Observabilidade e a app (2 réplicas de api); banco e Kafka externos, TLS em tudo, Grafana só com login | `dmpf-reference` e `grafana-admin` vêm de ExternalSecret/SealedSecret com esses nomes; `secret.dmpf-reference.example.yaml.tmpl` mostra a forma e **não** é resource |

As bases são fail-closed e o overlay `dev` relaxa o que precisa: `DMPF_KAFKA_INSECURE`, `DMPF_OTLP_INSECURE` e o acesso anônimo do Grafana nascem desligados e só `dev` os liga por patch. A base `dmpf-reference` não conhece credencial: `DMPF_PG_DSN` e `DMPF_KAFKA_BROKERS` vêm sempre do Secret do overlay; o resto vem do ConfigMap. Os três papéis têm `securityContext` restritivo (não root, sistema de arquivos só leitura, sem capabilities) e `terminationGracePeriodSeconds` acima do prazo interno de encerramento de cada papel.

A borda não autentica ninguém (a identidade de FND-07 não tem realização no kernel), então quem a alcança é decidido pela rede: a `NetworkPolicy` da base só admite ingress na api de pods rotulados `dmpf.lidercap.com.br/api-client: "true"`, e nega todo ingress a `relay` e `consumer`. O Alloy roda como DaemonSet e lê os logs dos pods pela API do kubelet, com RBAC de `pods`, `pods/log` e `namespaces` — sem `hostPath` e sem `nodes/proxy`.

Limitações declaradas: a api não expõe rota de saúde (as sondas são de socket TCP); Postgres, Redpanda, Loki, Tempo e Prometheus das bases são de desenvolvimento (um pod, sem operador, sem TLS, armazenamento local) e em produção viram serviços gerenciados; não há Ingress nem a malha completa de NetworkPolicy (Prometheus → alvos, Grafana → datasources, Alloy → Loki), que dependem do cluster de destino e estão registradas como dívida no ADR-041.
