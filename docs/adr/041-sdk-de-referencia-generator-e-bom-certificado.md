# ADR-041: Entregar o SDK de referência como composition root sob `apps/`, com a release do produto por tag anotada e BOM certificado

## Status

Aceito — 2026-09-08. Implementa SPEC-6QT9SBAS, primeira sub-spec de SPEC-8HWBWJCB (KRN-12). As sub-specs seguintes — generator (SPEC-H1A190Y8), BOM e validador (SPEC-538MS2D4), evidência e tag (SPEC-JPP31095) — acrescentam addenda a este ADR em vez de abrir outro; a consolidação final é da última.

## Contexto

O KRN-02 a KRN-11 entregaram o kernel DMPF em Go como quatorze libs, cada uma provada por suíte própria, mas nenhum serviço mostrava os blocos cabeados num processo real: `apps/backend` era um `.gitkeep`, e o único lugar onde instanciar provider concreto é permissivo — a linha `app` da matriz de blocos (ADR-010, ADR-015) — só existia como composition root de exemplo dentro de `dmpf-app`. O ticket ARQ-531 pede o SDK de referência, o generator de bounded context, o BOM de combinação certificada (FND-10) e a evidência da release `0.1.0`; a spec guarda-chuva dividiu isso em quatro sub-specs porque a revisão externa mostrou blockers independentes entre os subsistemas.

Duas lacunas do kernel apareceram ao cabear: (1) ninguém gravava o contexto de mensagem de ENV-08 — o provider Postgres escrevia `metadata = '{}'` (ADR-035) e o relay só reivindica linhas com `correlationid`, `causationid` e `traceparent` (ADR-038), então nenhuma escrita real drenaria; (2) o `orders` autora dois eventos (`ItemAdded`, `OrderPlaced`) para um único destino lógico, e o consumer de `reservations` só entende o segundo.

Restrições herdadas: provider concreto só no bloco `app` (ADR-015); `Ack` estritamente depois do retorno da transação (INB-08); toda rota HTTP referencia um contrato publicado (RST-04); um transporte assíncrono por processo (decisão do usuário na guarda-chuva); nenhuma dependência Go nova; a criação de unidade é ato regulado com baseline em commit próprio (AUT-01, `DMPF-T002`).

## Decisão

**Um binário, três papéis, um módulo `type:app`.** `apps/backend/dmpf-reference` é o projeto `dmpf-reference-go` (`type:app`, `scope:backend`, `stack:go`, `layer:apps`) e a unidade `dmpf-kernel/reference-app` do bloco `app`. O `--role api|relay|consumer` escolhe o processo — a relay nunca divide processo com o caminho da requisição (BLK-02) — e a configuração entra só por variável de ambiente, validada por papel na partida, com exit 2 nomeando a variável ausente. Os três papéis reutilizam os agregados de exemplo do kernel (`orders`, `reservations`) e o padrão de `dmpf-app/example/reservations`.

**O contexto de mensagem é autorado na borda e atravessa o kernel.** `dmpf-ports` ganhou `MessageContext{CorrelationID, CausationID, Traceparent}` com `WithMessageContext`/`MessageContextFrom` e o campo `OutboxEntry.Context`; o provider Postgres serializa o que foi autorado em `metadata` (chave ausente é ausente, nunca `""`, para não produzir linha que o relay rejeitaria); `dmpfapplication.MessageContextFor(ctx, id)` copia o contexto e preenche só a causação de quem inicia a cadeia — o próprio `id`, como FND-05 exige para ENV-08; `dmpf-app.Consumer` põe no contexto do handler o `correlationid`, o `id` recebido como causação e o `traceparent` do envelope. Na `api`, o middleware abre o span de servidor, injeta o `traceparent` pelo propagador W3C e usa `X-Correlation-ID` do cliente ou cunha um. Com isso a linha escrita pelo `api` é drenada de fato — o critério 2 do ticket.

**`api` sobre `net/http.ServeMux`, com a cadeia por rota dentro do mux.** Cada rota é um `dmpfhttp.Route` com `ContractRef` apontando para `contracts/openapi/orders/v1/openapi.yaml` (JSON pointer), validado na construção; a cadeia `Admission → requireIdempotencyKey → withMessageContext → handler` é montada por rota, e não em volta do mux, para que a admissão seja chaveada por `r.Pattern` — três chaves, cardinalidade limitada (MET-07) — e decida antes de ler o corpo (RES-17). `GET /orders/{id}` lê por `orderspg.NewReader(pool)` sem abrir transação (UOW-11), provado por `QueryTracer`. `DMPF_MIGRATE=true` aplica o schema antes de servir.

**Canal nomeado pelo destino do caso de uso; assinatura por tipo na ponte.** O canal Kafka chama-se `ordersapp.Destination` (`orders.events`), porque o publisher resolve pelo destino que o application service autorou; só `Address`, `Group` e `Containment` vêm do ambiente. Como o destino carrega os dois eventos do agregado, o `Sink` da app — a ponte transporte→adapter de FND-06 §11 — confirma sem inbox as entregas de tipo diferente do assinado (`EventType` + major do canal) e entrega ao adapter o tipo assinado e o que não decodifica (INB-10 intacto). Um canal por tipo de evento é matéria do generator (SPEC-H1A190Y8), não desta entrega.

**Telemetria com um runtime por processo e fallback em memória.** `otelboot.Start` roda uma vez por processo; `Run` faz o boot e `RunWith` recebe o runtime pronto, que é como o e2e hospeda os três papéis num binário só. Com `DMPF_OTLP_ENDPOINT` os exportadores são OTLP/gRPC; sem ele, `tracetest` e `ManualReader` em memória, com aviso de modo de desenvolvimento — a app sobe sem Collector.

**Nx e release.** `build`, `fmt-check`, `vet`, `test-race` (com `dependsOn` sobre `dmpf-provider-postgres-go` e `dmpf-app-go`, porque os três compartilham o Postgres do job) e `govulncheck` seguem o padrão das libs; `serve-api`, `serve-relay` e `serve-consumer` são `nx:run-commands`. O `@nx-go/nx-go` deriva o nome do projeto do último segmento do diretório, então infere `build` e `serve` assim que `cmd/dmpf-reference/main.go` existe: o `build` explícito sobrescreve o inferido e o `serve` inferido fica sem uso, documentado. O `nx-release.yml` passa a selecionar candidatos Docker por `tag:type:app,!tag:stack:go` — a imagem é do golden path, não desta app.

**Identidade da release do produto (transversal, da guarda-chuva).** A release do produto DMPF é uma tag git anotada `dmpf@<semver>` em `master`, cunhada pelo rito do BOM, distinta das tags por projeto do `nx release`; um BOM por tag em `bom/dmpf/<semver>.json`, com `version` **efetiva** por entrada (o `package.json` das libs, o SHA curto do commit para a app, o `go.mod` para dependências externas). "Sem edição manual" significa que nenhum byte gerado muda entre o generator e a aprovação pelo verificador — o rito de classificação em commit próprio é ato humano exigido por AUT-01, não edição. O escape hatch admite pedido só por catálogo fechado N1–N7 reconhecido por regra, nunca por texto. Um transporte assíncrono por processo. As sub-specs 2 a 4 realizam esses itens e registram aqui o que decidirem além deles.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Um `cmd/` por papel | BLK-02 pede processo, não binário; três binários triplicariam config, telemetria e cablagem sem separar nada que o `--role` não separe |
| Framework HTTP (chi, gin) ou helper server-side em `dmpf-provider-http` | Dependência Go nova, ou mudança no KRN-10 fora de rito; o `ServeMux` de Go ≥ 1.22 já casa método e path |
| Mux envolto pela admissão, como o plano previa | Fora do mux `r.Pattern` está vazio e a chave de admissão teria de ser o path bruto — cardinalidade aberta (MET-07) |
| Deixar o contexto de mensagem como task bloqueante ou gravá-lo por SQL no e2e | O critério 2 do ticket exige a drenagem real; sem a plumbing o e2e provaria uma linha que nunca sai da outbox |
| Quarentenar o `ItemAdded` no consumer de `reservations` | Toda escrita geraria uma contenção por desenho; o filtro por tipo na ponte é o gesto do transporte, e a inbox continua a ver só o que é do consumidor |
| OTLP obrigatório | A app não subiria sem Collector em desenvolvimento; o fallback em memória é explícito e logado |
| Nome do canal por `DMPF_CHANNEL` | O publisher resolve pelo destino autorado pelo caso de uso; um nome divergente terminaria em `ErrUnknownChannel` |
| Release group do `nx release` para o produto | Exigiria `package.json` com versão agregada e conflitaria com o versionamento independente (ADR-030) |

## Consequências

**Positivas:**

- O kernel passa a ter um serviço executável de referência com escrita, drenagem e consumo provados fim a fim sobre Postgres e Redpanda, incluindo reentrega com `DuplicateIgnored`.
- A lacuna de ENV-08 documentada em ADR-038 fecha: a outbox recebe o contexto de mensagem autorado pela borda, e o sinal `pending` deixa de crescer por desenho.
- `orderspg.NewReader` dá ao lado de leitura a forma que UOW-11 pede, reutilizando o `SELECT` do repositório.
- O contrato OpenAPI de `orders` inaugura `contracts/openapi/` com referência a partir de cada rota (RST-04).

**Negativas:**

- **Custo aceito:** `orders.events` continua a carregar dois tipos de evento; o filtro fica na app, e o `ItemAdded` é confirmado sem inbox — um canal por tipo é dívida do generator.
- **Custo aceito:** o `serve` inferido pelo `nx-go` existe e não faz nada útil; renomear o diretório do comando para evitá-lo daria um binário com nome errado.
- **Custo aceito:** o `test-race` da app exige Postgres e Redpanda e serializa-se atrás de dois módulos; a cadeia local fica mais longa.
- **Custo aceito:** o `api` do e2e é servido por `httptest` sobre o mesmo handler; `ListenAndServe` e o shutdown gracioso do `serveAPI` só são exercitados manualmente.
- **Custo aceito:** o `dmpf-units.json` da app declara o `external` de todos os providers cabeados; qualquer provider novo no golden path exige atualizá-lo.

Nada aqui declara ou sugere entrega exactly-once fim a fim. A garantia é at-least-once com consumidor idempotente pela inbox (KRN-07).

## Addendum — 2026-09-08 (checklist de qualidade da sub-spec 1)

A revisão de segurança, robustez e performance da entrega, feita com tráfego real sobre o Compose de `infra/local`, decidiu o que segue e registrou o que fica.

**Decidido e aplicado:**

- **A borda valida o que o contrato publica.** O domínio de `orders` não valida `quantity` nem `sku`, então a api recusava nada: `quantity: -1` era aceito e publicado. A borda passou a exigir `id` e `X-Correlation-ID` em `[A-Za-z0-9._:-]{1,128}`, `sku` com 1 a 128 caracteres, `quantity ≥ 1`, um único objeto JSON sem campos desconhecidos — `400` com `invalid-request`/`malformed-body`, e o OpenAPI passou a dizer o mesmo (`maxLength`, `pattern`, `additionalProperties: false`). Alternativa descartada: rejeição de domínio para `quantity ≤ 0`, que é invariante do agregado e cabe ao kernel, não a esta task.
- **Correlação do cliente é limitada, não confiada.** Um `X-Correlation-ID` de 8 KB era gravado em `metadata` e publicado em todo envelope da cadeia. Fora do formato, a borda cunha um novo e o ecoa no mesmo header.
- **O servidor HTTP tem todos os prazos** (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes` de 64 KiB), não só o do cabeçalho: `MaxBytesReader` limitava tamanho, não tempo.
- **Todo destino que os casos de uso autoram tem canal.** `reservationsapp.Destination` (`reservations.events`) não estava no catálogo e cada `ReservationConfirmed` terminava `failed` após dez tentativas. O catálogo passou a ter dois canais (`DMPF_KAFKA_RESERVATIONS_TOPIC/DLQ`), e o e2e exige `failed = 0`.
- **W3C explícito nos dois lados.** O kernel já recusava propagador sem `traceparent`/`tracestate` (`ErrPropagatorNotW3C`); a app declara `TraceContext + Baggage` e o Collector `propagators: [tracecontext, baggage]`.
- **Um envelope de log.** A trilha de auditoria saía como `audit.Event` cru (sem `time`, `level`, `service`) e o SDK OpenTelemetry reportava falhas de exportação por `log.Printf`. A auditoria continua canal separado e não amostrado, mas no mesmo envelope do handler (`kind: "audit"`, com `trace_id` e `correlation_id` da requisição), e o `otel.ErrorHandler` passa pelo logger da plataforma. O `Fields` de LOG-01 passou a ser preenchido (`correlation_id`, `tenant_id`).
- **Buckets do histograma em segundos** (`dmpf-observability/metrics/instruments.go`). A série `dmpf_service_request_duration_seconds` gravava segundos com os buckets default do SDK (`5, 10 … 10000`, feitos para milissegundos): toda requisição caía em `(0, 5]` e `histogram_quantile` respondia 2,5 s para uma chamada de 0,5 ms. É a única mudança fora da app; um teste fixa a escala.
- **Grafana fail-closed no Kubernetes.** A base exige login (`grafana-admin`) e o acesso anônimo é patch do overlay `dev`; antes ele estava na base e o `hmg` herdava — um Admin anônimo cria datasource, e datasource é proxy HTTP para qualquer endereço do cluster. `DMPF_OTLP_INSECURE` seguiu o mesmo caminho (base `false`, `dev` liga). No Compose, o anônimo continua (é desenvolvimento) e o formulário de login abre a administração do servidor.
- **NetworkPolicy como controle compensatório da borda sem identidade.** Ingress na api só de pods rotulados como cliente; nenhum ingress em `relay` e `consumer`. O Secret de exemplo do `hmg` deixou de ser resource (um `apply -k` sobrescrevia o Secret real com placeholders) e o ClusterRole do Alloy perdeu `nodes`/`nodes/proxy`, que a coleta pela API não usa.
- **Orçamento de recursos** em todo serviço do Compose, com `tools/infra-budget.sh` reprovando acima de 60% do host: 8,00 vCPU (57,1%) e 7,9 GiB (50,5%) para 20 serviços.

**Dívida registrada** (mudanças maiores ou que exigem validação empírica):

- Identidade e autorização reais na borda (token → subject → `AuthorizeFunc`), que também daria ator à trilha de auditoria; é spec própria.
- Malha completa de `NetworkPolicy` (`default-deny` por namespace + política por componente: Prometheus → alvos, Grafana → datasources, Alloy → Loki, Collector → Tempo/Prometheus).
- Pin por digest das imagens em `hmg` e `imagePullPolicy` declarado; `securityContext` em Postgres e Redpanda das bases; rotação de credencial por rollout (`disableNameSuffixHash`).
- cAdvisor sem `privileged` e Alloy sem `root` no Compose — podem falhar no WSL, precisam de teste antes.
- Publicação em `0.0.0.0` no Compose é decisão do usuário (acesso do Windows); a consequência está no `infra/README.md`.
- Rota de saúde na api (sondas por socket hoje), pool pgx com defaults, `INFRA_BUDGET_FRACTION` sobrescrevível pelo ambiente.

## Referências

- `docs/specs/SPEC-8HWBWJCB-dmpf-sdk-referencia-bom.md` — guarda-chuva do KRN-12.
- `docs/specs/SPEC-6QT9SBAS-dmpf-reference-composition-root.md` — spec desta entrega.
- `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05) — ENV-08, `causationid` = `id` quando a mensagem inicia a cadeia.
- `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §3.2, §5.4, §6.3, UOW-11, INB-08.
- `docs/dmpf/politicas-transporte.md` (FND-06) — §9 (RST-02, RST-04), §11 (ponte por `Sink`).
- `docs/adr/015-*.md` — provider concreto só no bloco `app`.
- `docs/adr/035-realizacao-postgres-da-outbox.md` e `docs/adr/038-*.md` — os ADRs cuja lacuna de `metadata` esta entrega fecha (ver os addenda).
- `docs/adr/039-*.md` — ponte por `Sink`, catálogo de canal, admissão.
- `apps/backend/dmpf-reference/README.md` — como rodar os três papéis localmente.
