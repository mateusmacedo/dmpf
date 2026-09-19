# AGENTS.md

Guidance for agents working in this repository. `CLAUDE.md` na raiz aponta para este arquivo (`@AGENTS.md`).

As recomendações abaixo são defaults pensados para este projeto e podem ser ajustadas quando o contexto justificar. Regras marcadas como **duras** cobrem riscos que afetam segurança, integridade de dados ou trabalho alheio — essas devem ser respeitadas sempre.

## Índice

1. [Meta](#meta) — Propósito, idioma, resolução de conflitos
2. [Regras duras](#regras-duras) — Comportamentos não negociáveis
3. [Visão geral e estrutura](#visão-geral-e-estrutura) — Apps, libs, diretórios
4. [Comandos](#comandos) — Nx, Biome, scripts raiz
5. [Tooling](#tooling) — Stack efetiva do workspace
6. [Convenções obrigatórias](#convenções-obrigatórias) — Tags, targets, commits
7. [Git e release](#git-e-release)
8. [Padrões de código](#padrões-de-código) — Defaults recomendados
9. [Workflows](#workflows) — Processos de trabalho
10. [Referências](#referências) — Documentação complementar
11. [Glossário](#glossário)
12. [General Guidelines for working with Nx](#general-guidelines-for-working-with-nx) — bloco Nx (autoatualizado)

---

## Meta

### Propósito e escopo

- Este arquivo descreve convenções e o inventário real do repositório para agentes.
- Em caso de conflito entre instruções, considere este arquivo junto com os arquivos de configuração efetivos do projeto (por exemplo: `package.json` / `pnpm-workspace.yaml`, `lefthook.yml`, `biome.json`, `nx.json`, `tsconfig.base.json`).
- Este repositório é um **template**. Projetos derivados herdam estas convenções e devem atualizar o inventário desta seção conforme criam apps e libs reais.

### Autonomia do template

Este template é **autônomo**: o scaffold de engenharia fica versionado no próprio repositório e vale por si, sem plugin externo. Isso inclui o catálogo e o template de specs (`docs/specs/`), as regras de domínio (`docs/rules/**`, nos escopos `BIZ-`/`APP-`/`PRD-`), o guia detalhado de fluxo (`docs/guides/development-workflow.md`), o índice de ADRs (`docs/adr/README.md`), as rules documentais em `.claude/rules/` e as skills de domínio em `.claude/skills/`.

O fluxo git deste repositório é o git-flow da organização (ver [Git e release](#git-e-release)). A decisão histórica de fronteira bake/consume (já supersedida) está em `docs/adr/007-fronteira-plugin-template.md`.

### Idioma

Por default, respostas são em português (PT-BR) e código permanece em inglês. O idioma da interação é parâmetro do projeto e pode ser ajustado.

### Resolução de conflitos

| Nível | Significado | Exemplo |
| --- | --- | --- |
| **Duro** | Não negociável; envolve segurança, integridade ou trabalho alheio | Autorização explícita para commit/push |
| **Recomendado** | Default esperado; pode ser ajustado com justificativa | Padrão arquitetural adotado |
| **Opcional** | Melhoria útil quando couber | Remoção proativa de código morto |

---

## Regras duras

Comportamentos abaixo são não negociáveis.

### Validação após cada tarefa

Ao finalizar código, execute a cadeia de validação do projeto (lint, typecheck e testes via `pnpm nx` / scripts raiz). Se um passo falhar, corrija e repita até todos passarem. Rode o formatter (Biome) antes do lint quando fizer sentido.

Para tarefas de interface, verifique visualmente o resultado quando houver ferramenta disponível; se não houver, pergunte ao usuário como proceder.

### Proteções não negociáveis

- Não modifique testes para passar — corrija o código sob teste.
- Não desabilite hooks/verificações (por exemplo, `--no-verify`) sem pedido explícito.
- Não execute force push ou operações Git destrutivas sem autorização explícita.
- Revise `git diff` antes de criar um commit.
- Peça confirmação explícita antes de commit/push.
- Ao documentar configurações, copie fielmente os valores reais em vez de generalizar.
- Nunca edite diretamente em `master` ou `develop`.

### Escopo de alterações

- Para solicitações explícitas, faça apenas o que foi pedido.
- Ao encontrar problemas adjacentes:
  - Mudanças pequenas e evidentes: corrija junto e mencione.
  - Mudanças médias: mencione e pergunte antes de aplicar.
  - Mudanças grandes: registre para tarefa separada.
- Refatorações fora do escopo solicitado dependem de autorização.

### Refatoração de símbolos

Ao refatorar qualquer símbolo (variável, função, tipo):

1. Busque todos os usos no projeto antes de alterar.
2. Não assuma que o caso mencionado é o único uso.

### Imports ao mover arquivos

Ao mover arquivos ou pastas, verifique todos os imports afetados — inclusive os que o compilador pode não acusar (por exemplo, caminhos absolutos em mocks).

---

## Visão geral e estrutura

`dmpf` (workspace npm `@mateusmacedo/dmpf-source`) é o baseline de monorepo **Nx + pnpm** multistack da organização. O workspace já vem preparado para Express, Fastify, NestJS, Next.js, Angular e Go (`go.work` / `@nx-go/nx-go`), mas as únicas apps que contém são as do kernel DMPF — a topologia de referência e o golden do harness de bounded contexts (ver [Apps](#apps)); quem parte deste template cria as suas.

Fonte de verdade dos projetos: `pnpm nx show projects`.

### Diretórios principais

```text
dmpf/
├── apps/
│   ├── backend/{bff,orders,reservations,bookings}/  # BFF REST e os bounded contexts orders e reservations (topologia de referência, ADR-044) e bookings (golden do harness); um contexto é uma app com um package por bloco (ADR-046)
│   ├── frontend/                   # placeholder — sem projeto Nx
│   └── serverless/                 # placeholder — sem projeto Nx
├── libs/
│   ├── backend/go/                 # 14 módulos, só o kernel DMPF de reuso, de nome bare (domain, ports, memory, grpc, …); nenhum contexto de negócio nem exemplo (ADR-045, ADR-046)
│   ├── frontend/                   # placeholder — sem projeto Nx
│   └── shared/                     # placeholder — sem projeto Nx
├── bom/                            # BOM da release do produto (dmpf/bom@1): bom/dmpf/<semver>.json, validado pelo dmpf-bom, e a evidência que o certifica em bom/evidence/<semver>/
├── docs/
│   ├── adr/                        # ADR-00N (baseline, tasks, hardening, release, plataforma, fechamento, fronteira, identidade de automação, libs)
│   ├── specs/                      # catálogo e template de specs (SPEC-XXXX)
│   ├── rules/                      # regras de domínio, com README de índice
│   │   ├── business/               # regras de negócio (BIZ-)
│   │   ├── application/            # regras de aplicação (APP-)
│   │   └── product/                # regras de produto (PRD-)
│   ├── guides/                     # development-workflow.md (guia detalhado do fluxo)
│   ├── nx-reference/               # guia prático de tasks Nx
│   ├── ci-cd/                      # adoção de CI/CD e deploy
│   └── onboarding.md               # setup local e primeiro PR
├── infra/                          # local/ (Compose modular por recurso), observability/ (Grafana, Prometheus, Loki, Tempo, Alloy, Collector: config + manifestos), k8s/ (Kustomize base + overlays dev/hmg), docker/ (Dockerfile de referência Node)
├── tools/                          # generators, executors, scripts do workspace e dmpf-conformance/ (verificador DMPF, BOM, modsync, fitness — tooling Go, sem layer:*)
├── .agents/skills/                 # skills de workspace (Nx e dmpf-bounded-context)
├── .claude/                        # agents, skills e rules para assistentes
├── lefthook.yml
├── nx.json
├── package.json
└── pnpm-workspace.yaml
```

Apps Nest criadas a partir daqui seguem tipicamente `src/app/<feature>/` (controllers, services, DTOs colocalizados). Libs exportam pela `src/index.ts` do pacote.

### Apps

Quatro, Go, cada uma com as três tags de taxonomia (`type:app`, `scope:backend`, `stack:go`), a tag `layer:apps` e um `dmpf-units.json`. Três formam a topologia de referência do kernel DMPF, com seis processos sobre dois bancos (ADR-044); a quarta, `bookings`, é o golden do harness de bounded contexts. Um contexto de negócio é **uma app**, não uma lib: o módulo carrega os seus blocos (`domain/`, `application/`, `provider/` e o bloco `app`) com unidades `<ctx>/<bloco>` no manifesto, e em `libs/backend/go` fica só o kernel de reuso (ADR-046):

- **`bff`** (`apps/backend/bff`), a única borda REST/JSON pública (`RST-01`): um processo, sem banco, outbox nem consumer, que traduz `POST /orders/{id}/items`, `POST /orders/{id}/place`, `GET /orders/{id}`, `GET /reservations/{order_id}`, `POST /reservations/{order_id}/reserve` e `POST /reservations/{order_id}/cancel` — cada rota um `http.Route` com `ContractRef` para `contracts/openapi/{orders,reservations}/v1/openapi.yaml` — em chamadas gRPC aos contextos. É a unidade `bff/app`, em `bounded_context` próprio: só alcança contrato e shared kernel, e o verificador reprova import de domínio ou aplicação (`DMPF-D002`). Deriva o prazo do `Budget` da rota, preserva ou cunha `X-Correlation-ID` e envia `x-correlation-id`, `x-causation-id` e `traceparent` na metadata; retry só em `FindOrder` e `FindReservation`. O transporte gRPC exige `DMPF_GRPC_INSECURE=true` (desenvolvimento) ou `DMPF_GRPC_CA_FILE`. Carrega os targets de infraestrutura e o e2e caixa-preta (build tag `integration`), que compila os três binários, sobe os seis processos sobre dois bancos criados por execução e prova a cadeia de contexto até `ReservationConfirmed`, a primeira decisão vencendo e a reentrega idempotente; o `test-race` declara `dependsOn` sobre os de `postgres` e `app`.
- **`orders`** (`apps/backend/orders`), o contexto `orders`: `--role api|relay` — `api` serve `OrdersService` por gRPC (`AddItem`, `PlaceOrder`, `FindOrder`) sobre a UoW e a outbox de Postgres, e `relay` drena `orders.events` para o Kafka. É o contexto completo, `bounded_context` `orders`, no layout de `bookings`: `domain/` (package `domain` — o agregado `Order` e as suas duas UPRs, `AddItem` e `Place`), `application/` (package `application` — o caso de uso de referência, que percorre os nove passos da sequência canônica de FND-04 §3.2; `Destination = "orders.events"`), `provider/` (package `provider` — repositório, reader e mapeador sobre Postgres para `company.orders.event.v1`) e o bloco `app` na subpasta `app/`, com a borda em `app/rpc/` e o binário em `cmd/main.go` (ADR-048). Carrega também `appkit/` e `distkit/`, obrigatórios em todo contexto. Unidades `orders/domain`, `orders/application`, `orders/provider-postgres`, `orders/app`, `orders/appkit` e `orders/distkit`; o contrato gerado é a unidade `orders/contract`, declarada no manifesto do `contracts`. Onde o `domain` ou o `application` do kernel coexiste com o do contexto num arquivo, o kernel recebe alias pelo papel (`kernel`, `usecase`).
- **`reservations`** (`apps/backend/reservations`), o contexto `reservations`: `--role api|relay|consumer` — `api` serve `ReservationsService` (`Reserve`, `Cancel`, `FindReservation`), `relay` drena `reservations.events` e `consumer` consome `orders.events` pela inbox. É o contexto completo, `bounded_context` `reservations`, no mesmo layout: `domain/` (package `domain` — o agregado consumidor, com chave natural permanente, o identificador do pedido, e a UPR `Reserve`), `application/` (package `application` — o caso de uso de **consumo**, que ramifica pelas sete disposições de FND-04 §6.4 sobre a porta de inbox), `provider/` (package `provider`, sobre Postgres) e o bloco `app` na subpasta `app/`, que absorveu a composition root do consumidor (`consumer.go`), com a borda em `app/rpc/` e o binário em `cmd/main.go` (ADR-048). Carrega também os harnesses do exemplo: `appkit/` (borda a borda sobre Postgres, com o e2e que exerce as sete disposições e o vetor V32) e `distkit/` (dois processos OS sobre Redpanda com reentrega deliberada — `DMPF-R004`, build tag `distributed`, target `test-distributed`); os dois alcançam `tb`, `clock` e `ids` do `testkit` porque essas unidades são superfície pública. Unidades `reservations/domain`, `reservations/application`, `reservations/provider-postgres`, `reservations/app`, `reservations/appkit` e `reservations/distkit`; o contrato gerado é a unidade `reservations/contract`, no manifesto do `contracts`.
- **`bookings`** (`apps/backend/bookings`), o contexto de exemplo do harness de bounded contexts, criado por `ARQ-554` a partir de `docs/specs/SPEC-AHPRBZCT-bookings.md`. É o **golden** contra o qual a prova de regressão compara: um módulo Go com um package por bloco — `domain`, `ports`, `application`, `provider`, `app` —, dois agregados (`Booking` e `Resource`), três UPRs, a consulta por relação declarada no bloco `port` — a única que os genéricos do kernel não expressam —, provider sobre Postgres com harness de teste próprio e borda HTTP até a outbox. Oito unidades no `bounded_context` `resource-scheduling` — as seis originais mais `appkit` e `distkit` —, sete no `dmpf-units.json` do módulo e a `contract` declarada no manifesto do `contracts`. Desde o ADR-048 tem composition root completa: `app/config.go`, `app/catalog.go`, `app/wiring.go`, a borda em `app/http/` (package `httpedge`, porque `http` colidiria com `net/http`), o binário em `cmd/main.go` com `--role api|relay` e os targets `serve-api` e `serve-relay`. O `test-race` declara `dependsOn` sobre o do `postgres`, porque os dois compartilham o Postgres do job e cada harness trunca as mesmas tabelas. Onde um arquivo importa o kernel e o contexto com o mesmo nome de package (`domain`, `application`, `ports`), o import do kernel recebe alias pelo papel — `kernel`, `usecase`, `port` (ADR-045). É o que o generator `bounded-context` emite, em `apps/backend/<contexto>` e com `type:app`; como criar um contexto novo está em `docs/guides/dmpf-composicao.md`.

Nos três da topologia de referência, a configuração entra só por variável de ambiente, validada na partida (exit 2 nomeando a ausente), com um runtime OTel por processo, em memória sem `DMPF_OTLP_ENDPOINT`. O binding gRPC é escrito no bloco `app` a partir do descriptor gerado, porque o bloco `contract` não admite `io.network`. Cada contexto usa banco próprio (`DMPF_PG_DSN`), porque a drenagem não filtra destino. O `@nx-go/nx-go` infere `build` e `serve` pelo nome do diretório: o `build` explícito sobrescreve o inferido, os contextos sobem por `serve-api`, `serve-relay` e `serve-consumer`, e o BFF por `serve`. Ficam fora do release Docker (`nx-release.yml` filtra `tag:type:app,!tag:stack:go`). Decisões em `docs/adr/044-bff-rest-e-contextos-grpc-de-referencia.md`, que evolui o `docs/adr/041-sdk-de-referencia-generator-e-bom-certificado.md`.

`apps/frontend` e `apps/serverless` seguem sendo diretórios de destino, sem projeto Nx registrado. Para criar uma app nova, invoque a skill `nx-generate` antes de qualquer exploração.

### Libs

Catorze, todas Go — só o kernel DMPF de reuso: nenhum contexto de negócio nem exemplo vive aqui (ADR-046) —, com as três tags de taxonomia (`type:lib`, `scope:backend`, `stack:go`), a tag de camada `layer:*` (ver [Convenções obrigatórias](#convenções-obrigatórias)), um `dmpf-units.json` (o `metadata_container` da RFC DMPF) e um `package.json` com `private: true` — este último existe porque o Nx Release aborta o versionamento de um `tag:type:lib` sem manifesto npm (ver `docs/adr/030-granularidade-modulo-go-e-bom.md`):

- **`domain`** (`libs/backend/go/domain`), o kernel de domínio do DMPF, criado por `KRN-01` e preenchido por `KRN-03`. O package raiz `domain` realiza o desfecho da UPR como par `(Accepted[R], *Rejection)`. É uma unidade `domain`, `kernel/domain`, no `bounded_context` `kernel`; os agregados de exemplo que viveram em `example/` são hoje o `domain/` de `apps/backend/orders` e de `apps/backend/reservations` (ver `docs/adr/032-realizacao-go-do-desfecho-da-upr.md`).
- **`contracts`** (`libs/backend/go/contracts`), o bloco `contract` do kernel criado por `KRN-05`: código gerado de Protobuf em `gen/go/` (nunca editado à mão), o codec do envelope CloudEvents (`envelope`) e a fórmula do `payload_hash` (`payloadhash`). A **fonte** dos contratos — `.proto`, configuração Buf e golden fixtures — vive em `contracts/`, na raiz, e é neutra de stack; `contracts/README.md` explica a árvore, a proveniência do envelope oficial e a máquina de estados do baseline (`BUF-08`).
- **`ports`** (`libs/backend/go/ports`), o bloco `port` do kernel, criado por `KRN-04`. Declara a fronteira de Unit of Work (`UnitOfWork[R]`), o repositório com optimistic locking, a porta da outbox em tipos de domínio, o relógio e o gerador de identificador — treze identificadores exportados, superfície fechada, nenhuma realização. É uma unidade `port`, `kernel/port`. `Instant` é inteiro de nanossegundos e não `time.Time`, porque o verificador classifica o package `time` inteiro como `io.clock` (ver `docs/adr/034-fronteira-de-uow-em-go.md`).
- **`application`** (`libs/backend/go/application`), o bloco `application` do kernel, também de `KRN-04`. O package raiz `application` traz o desfecho de aplicação `Outcome[R]`, que separa o canal de negócio do técnico, a resolução de identidade anterior à transação e o gancho de autorização, e ainda `Disposition`, `Failure` e `Classify` (de `KRN-07`) — o subconjunto da taxonomia de FND-07 que o consumo precisa. É uma unidade, `kernel/application`. Os casos de uso de exemplo — o de referência, que percorre os nove passos da sequência canônica de FND-04 §3.2, e o de **consumo**, que ramifica pelas sete disposições de FND-04 §6.4 sobre a porta de inbox — vivem no `application/` de `apps/backend/orders` e de `apps/backend/reservations`; a realização em memória que os fechava sem banco (`example/memory`) é hoje o módulo `memory`.
- **`memory`** (`libs/backend/go/memory`), o bloco `provider` do kernel em memória — a antiga `example/memory` do `application`, promovida a módulo genérico pelo ADR-046. Realiza `UnitOfWork[R]`, `Repository[ID, S]`, `Reader[ID, S]`, `Outbox` e a porta de inbox sem banco, sobre qualquer agregado: o descritor `Table[ID, S]{Name, Clone}` pareia `Reader(store)` e `Repository(tx)` sob o mesmo nome lógico — o equivalente da tabela SQL do `postgres` — e carrega o clone chamado em toda travessia da fronteira (`nil` quando a cópia por valor basta). O que prova (uma transação por `Within`, callback uma vez, commit de estado e outbox juntos, rollback em erro e em `panic`, `FailNextCommit`, contexto cancelado nunca abre transação, snapshots sem backing array compartilhado) e o que não prova (serializa transações em vez de isolá-las; um conflito de versão é injetado pelo chamador) está no `doc.go`. É uma unidade `provider`, `kernel/provider-memory`, sem dependência externa e designada shared kernel no baseline; `testkit/serviceskit` a consome em produção, e o `test-race` roda sem infraestrutura (`cache: true`, sem `-tags=integration`).
- **`postgres`** (`libs/backend/go/postgres`), o bloco `provider` do kernel, criado por `KRN-06`. Realiza `UnitOfWork[R]`, `Repository[ID, S]` e `Outbox` sobre `pgx/v5`: uma `pgx.Tx` por `Within`, o registro de outbox gravado na mesma transação do estado de negócio, e a serialização acontecendo **na escrita** — `payload` guarda os bytes do `Any` do integration event, não o CloudEvent inteiro (ver `docs/adr/035-realizacao-postgres-da-outbox.md`). Desde o `KRN-07` realiza também a porta de inbox (`Tx.Inbox`, `INSERT … ON CONFLICT DO NOTHING` com teto por `SET LOCAL lock_timeout`), a quarantine, a purga da inbox e os sinais do consumo (ver `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md`). É uma unidade `provider`, `kernel/provider-postgres`; o repositório e o mapeador de cada agregado de exemplo vivem no `provider/` de `apps/backend/orders` e de `apps/backend/reservations`, e o `schema.sql` do módulo continua criando as tabelas `dmpf_example_orders` e `dmpf_example_reservations` que eles usam (`postgres.Migrate`). Foi o primeiro módulo do workspace cujo teste exige infraestrutura: os testes de banco levam a build tag `integration`, rodam só no `test-race` (com `cache: false` no Nx e `-count=1` no `go test`) e o job `main` do CI sobe o Postgres por `docker run` na rede do job — o bloco `services:` não resolve DNS no `act_runner`. Fora do alcance do `depguard`, que seleciona por nome de diretório; quem prova as células 26 e 12 aqui é o `tools/dmpf-cell-check.sh`.
- **`app`** (`libs/backend/go/app`), o primeiro módulo do bloco `app` do workspace, criado por `KRN-07`. É o consumer adapter de FND-04 §6.3: recebe os bytes brutos da entrega (`Delivery.Raw`), decodifica o envelope por `envelope.Unmarshal`, calcula o `payload_hash`, invoca o application service e aplica o efeito de broker — `Ack`, `Release` ou contenção — sempre depois do retorno da transação (`INB-08`). Vive em módulo próprio porque só a linha `app` da matriz de blocos alcança `contract` e `application` ao mesmo tempo (células 12 e 26 são proibidas); `relay` é o relay da outbox de `KRN-08` (claim por lease, envelope montado na publicação, `payload_hash` divergente terminal — ADR-038). São duas unidades `app`, `kernel/app-consumer` e `kernel/app-relay`, sem dependência externa declarada — o código de produção não importa protobuf; a composition root do consumidor de exemplo, com o e2e sobre Postgres que exerce as sete disposições e o vetor V32, é hoje o bloco `app` de `apps/backend/reservations`. Desde o `KRN-12` o `Consumer` propaga o contexto de mensagem do envelope ao handler (`correlationid`, `id` recebido como causação, `traceparent`). O `test-race` declara `dependsOn` sobre o do `postgres`, porque os dois compartilham o Postgres do job e cada harness faz `TRUNCATE`.

- **`observability`** (`libs/backend/go/observability`), o bloco `provider` do kernel, criado por `KRN-09`. Realiza o `FND-08` sobre OpenTelemetry `v1.46.0` e `semconv/v1.43.0`: a ficha de resiliência e os decorators (`resilience`), o retry por conjunção com orçamento (`retry`), o bootstrap do SDK com sampler e processor próprios (`otelboot`), os exportadores OTLP/gRPC (`otelboot/otlp`), o catálogo de métricas (`metrics`), os atributos e a taxonomia de classes (`tracing`), o handler de log com redação (`logging`), a trilha de auditoria (`audit`), o relógio injetável (`clock`) e a realização do gancho de instrumentação (`usecase`). É uma unidade `provider`, `kernel/observability`, com doze packages. O `TRC-14` — erro sempre amostrado — é realizado por regra equivalente em processo, porque os samplers de fábrica do SDK devolvem `Drop` no ramo negativo; a suíte **exige Docker**, porque o teste do Collector roda sem build tag (ver `docs/adr/037-observabilidade-otel-e-retry-por-conjuncao-em-go.md` e o `README.md` do módulo).

- **`transport`** (`libs/backend/go/transport`), o módulo de primitivas que os quatro providers de transporte compartilham, criado por `KRN-10`. `deadline` realiza o orçamento de prazo por método sobre o governo do tempo do KRN-09 (`Require` para GRP-04, `Outgoing` = `EffectiveDeadline` − folga); `channel` é o conteúdo de `ASY-02` em processo — `Channel`, `Catalog`, `Resolve` e as fórmulas de `janela_redelivery` (`KafkaWindow`, `SQSWindow`, `SNSSQSWindow`); `attempt` é o metadado de tentativa lateral ao envelope; `admission` é o bucket por rota e tenant com teto de cardinalidade (RES-16/17); `observe` realiza as três posições de observabilidade que RES-23 exige de toda composição e o KRN-09 deixa ao chamador. É uma unidade `provider`, `kernel/transport`, sem I/O; a API do OpenTelemetry é a única dependência externa (ver `docs/adr/039-providers-de-transporte-sink-e-gesto-de-release.md`).
- **`grpc`** (`libs/backend/go/grpc`), o transporte síncrono interno de FND-06 §10, criado por `KRN-10`. Interceptors de cliente derivam o prazo de cada chamada do prazo de quem chama e do orçamento do método, recusando contexto sem deadline antes do fio (GRP-04/05/17; `two_hops_test.go` prova A → B → C); a composição de RES-22 é uma `Call` por método declarado, com retry só para método idempotente e código declarado transiente (GRP-08/09), o retry nativo do gRPC desligado; `Dial` com `round_robin` + health check e `NewServer` com TLS, health por serviço e a admissão por método e tenant (`RESOURCE_EXHAUSTED` antes do handler, MET-12); `HTTPStatus` realiza a tabela de GRP-14. Unidade `provider`, `kernel/provider-grpc`.
- **`http`** (`libs/backend/go/http`), a borda externa em REST/JSON de FND-06 §9, criado por `KRN-10`, só com `net/http`. `Route` exige referência ao contrato publicado (RST-04) e decide a idempotência por método — POST só com chave declarada, PATCH nunca (RST-02); `Client.Do` converte o deadline em timeout (RST-03), retenta só o idempotente com chave estável e corpo por `GetBody`, e devolve a última resposta transiente com o status original; o middleware `Admission` recusa 429 antes de ler o corpo (RES-17). Unidade `provider`, `kernel/provider-http`.
- **`kafka`** (`libs/backend/go/kafka`), o transporte-alvo do evento de domínio de FND-06 §11, criado por `KRN-10` sobre `franz-go`. `Publisher` escreve a chave de partição do envelope e os bytes recebidos, com `Observer` só-leitura (TRP-17); `Consumer` tem um worker persistente por partição (KFK-09), commit só do prefixo contíguo por `CommitRecords` (TRP-29), retry inline com partição pausada até o limite do canal (TRP-47) e cancelamento cooperativo na revogação (TRP-48); `DLQ` realiza `ports.Containment` com o envelope intacto. A ponte com o adapter é a interface `Sink`, e os tetos de tentativas do adapter e do canal precisam ser iguais. Os testes de integração levam a build tag `integration` e exigem `DMPF_KAFKA_BROKERS` (Redpanda no CI). Unidade `provider`, `kernel/provider-kafka`.
- **`sqs`** (`libs/backend/go/sqs`), o transporte normatizado em SNS/SQS de FND-06 §12, criado por `KRN-10` sobre `aws-sdk-go-v2`. Envelope em Base64 uma única vez (TRP-19) com a envoltória do SNS sem raw delivery recusada (SQS-02); grupo e deduplicação FIFO derivados do envelope por SHA-256 (SQS-05/06); `Consumer` com heartbeat de visibilidade parado antes de qualquer gesto e teto de doze horas (SQS-08/08b), `attempt` pelo `ApproximateReceiveCount`, sem retry inline (SQS-11b); `Ack` deleta pelo receipt handle depois do commit, `Release` encurta a visibilidade (SQS-09/10); `SNSPublisher` recusa assinatura sem raw delivery na construção. Os testes de integração exigem `DMPF_SQS_ENDPOINT` e credenciais `AWS_*` (floci no CI, SQS e SNS no mesmo endpoint). Unidade `provider`, `kernel/provider-sqs`.
- **`testkit`** (`libs/backend/go/testkit`), o instrumento de teste de FND-09, criado por `KRN-11`. Onze unidades em quatro blocos, uma por package: `domainkit` (`domain`) executa a UPR por valor e devolve a projeção observável (`ORA-30`..`ORA-39`); `golden` (`contract`) carrega a fixture com todo escalar como string, roda as duas direções e reporta os três oráculos em separado (`DMPF-R001`..`R003`, oráculo 3 reprovando na direção produtor); `serviceskit` e `providerkit` (`provider`) são os fakes com ledger (`UOW-06`..`08`) e as suítes de conformidade de `UnitOfWork`, `Inbox` e outbox store (`OBX-10`/`11`, `INB-06`) que `memory` e Postgres rodam; `clock`, `ids` e `stable` (`provider`) são o determinismo (`KIT-07`, `KIT-08`); `fitness` (`app`) é a regra de dependência na suíte — universo real, 36 células por par de vetores, `V29`/`V30` e `V27` registrado como single-stack; `tb` e `tb/pg` (`app`) são o adaptador de `testing.TB`, o codec de projeção e o pool Postgres; `evidence` e `cmd/evidence` (`app`, de `KRN-12`) gravam os veredictos dos kits sob `DMPF_EVIDENCE_DIR` e publicam a evidência determinística da release em `bom/evidence/<semver>/` (target `evidence`; ver `bom/README.md`). Todo kit devolve veredicto por valor. As fixtures de projeção vivem em `contracts/fixtures/<ctx>/projection/v1/`. O `conformance` ganhou o package exportado `fitness` (unidade `app`, superfície pública) e `contracts/golden` delega o carregador ao kit. Dependências declaradas: `protobuf` (`wire.codec`); `pgx` entra só pelos packages `app` (`tb/pg`, `cmd/evidence`). Os harnesses do exemplo, `appkit` e `distkit`, e o target `test-distributed` saíram para `apps/backend/reservations` (ADR-046); por isso `tb`, `clock` e `ids` são superfície pública (`public_integration_surface: true`), para que um harness de outro contexto os alcance, e o `test-race` cobre o módulo inteiro (ver `docs/adr/040-test-kits-golden-e-fitness-function-em-go.md` e o `README.md` do módulo).

`libs/frontend` segue sendo diretório de destino, sem projeto Nx registrado. Para criar uma lib TypeScript, use o generator do Nx (`pnpm nx g @nx/js:lib libs/shared/<name>`), com as três tags 3D e `--linter=none`; o passo a passo com todas as flags está em `docs/nx-reference/tasks.md`.

**Caminho por scope e stack, nome sem prefixo nem sufixo (ADR-045).** Módulos do kernel ficam em `libs/<scope>/<stack>/<módulo>` e o nome do projeto Nx, do pacote npm privado e do package Go raiz é o basename do diretório — `domain`, `ports`, `grpc`, `postgres` —, sem marca de produto (`dmpf-`), sem papel (`provider-`) e sem stack (`-go`): a árvore já diz que `libs/backend/go/grpc` é Go e é o provider gRPC. Contextos de negócio são **um** módulo e são **apps**: vivem em `apps/<scope>/<contexto>` (`apps/backend/bookings`, projeto `bookings`; `apps/backend/orders`; `apps/backend/reservations`), com um package por bloco (`bookings/domain`, `bookings/ports`, …), como o generator `bounded-context` gera; a unidade do manifesto continua `<bounded_context>/<bloco>`. Em `libs/<scope>/<stack>/` fica só o kernel de reuso — as unidades que o baseline designa em `shared_kernel_units`, o `contracts` (superfície pública compartilhada: fosse de uma app, o BFF importaria outra app) e o `testkit` genérico (ADR-046). As demais apps seguem a mesma regra em `apps/<scope>/<app>` (`apps/backend/bff`, projeto `bff`, binário `cmd/main.go`). Quando dois packages importados no mesmo arquivo colidem no nome — o nosso `grpc` com `google.golang.org/grpc`, o `domain` do kernel com o do contexto —, quem importa declara o alias pelo papel local (`provider`, `kernel`, `usecase`, `port`), e nunca se concatena nome para evitar a colisão. Tooling fica fora da regra: `tools/dmpf-plugin` (`@mateusmacedo/dmpf-plugin`) e `tools/dmpf-conformance` (`@mateusmacedo/dmpf-conformance`) mantêm o prefixo, porque não são lib nem app e o nome os distingue de outros plugins e ferramentas que o workspace venha a ter. A contraparte TypeScript do kernel, quando nascer, vive em `libs/<scope>/ts/`, então o Nx não colide. Como em Go o import path é a chave canônica da unidade — e a RFC a exige estável —, renomear ou mover é ato de classificação: o baseline foi regravado em commit próprio (`AUT-01`), no ADR-045 e de novo no ADR-046.

O `nx-release.yml` tem o step `Detect lib release candidates`, que pula o versionamento e o push enquanto não houver nenhum projeto com a tag `type:lib` — mesmo padrão do step de candidatos Docker. Ele existe porque, sem nenhuma lib, o `nx release` sai com erro (`Release group "__default__" matches no projects`) em vez de concluir vazio. Com o `domain` presente, a guarda deixa de ser acionada e o versionamento passa a rodar de fato.

O módulo Go participa do versionamento, mas **não** da publicação: o `private: true` do `package.json` já o exclui, e o `build_projects_filter` do `nx-publish-libs.yml` (`tag:type:lib,!tag:stack:go`) o exclui de novo, por redundância deliberada.

O prefixo dos pacotes é `@mateusmacedo/`, tudo em minúsculas. No GitHub Packages o escopo **precisa ser o owner do repositório** — não é convenção, é requisito do registry. O casing precisa bater exatamente entre o `name` de cada `package.json`, o `tsconfig.base.json` e o `scope` passado ao reusable de publicação: o Nx resolve o registry pelo escopo do pacote, e qualquer divergência faz o `pnpm publish` cair no registry público e falhar.

---

## Comandos

Sempre via `pnpm nx` — nunca o `nx` global. O root `@mateusmacedo/dmpf-source` tem targets `nx:noop`; exclua-o de operações em lote com `--exclude=@mateusmacedo/dmpf-source` (como no `lefthook` / CI).

```bash
# Em lote (todos os projetos com o target)
pnpm nx run-many -t build
pnpm nx run-many -t lint
pnpm nx run-many -t test
pnpm nx run-many -t typecheck

# Apenas projetos afetados (preferir no dia a dia e em CI)
pnpm nx affected -t lint
pnpm nx affected -t typecheck
pnpm nx affected -t test
pnpm nx affected -t build

# Um único projeto
pnpm nx test @mateusmacedo/minha-lib
pnpm nx build @mateusmacedo/minha-lib

# Um único arquivo de teste (passthrough p/ Jest)
pnpm nx test @mateusmacedo/minha-lib --testPathPatterns="string"

# Cadeia Go (os 4 passos que lint/test/build não cobrem)
pnpm nx run domain:fmt-check   # gofmt, read-only (reprova, não reescreve)
pnpm nx run domain:vet
pnpm nx run domain:test-race
pnpm nx run domain:govulncheck # sem cache: consulta base remota
# Testes de integração dos transportes (build tag integration; sem a variável fazem t.Skip)
DMPF_KAFKA_BROKERS=localhost:9092 pnpm nx run kafka:test-race            # Redpanda local
DMPF_SQS_ENDPOINT=http://localhost:4566 AWS_REGION=us-east-1 AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test pnpm nx run sqs:test-race  # floci local
# Test kit (KRN-11): test-race cobre os kits em memória e, com DMPF_PG_DSN, providerkit sobre Postgres
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' pnpm nx run testkit:test-race
# Harnesses do exemplo (ADR-046): appkit e distkit vivem em apps/backend/reservations, não no testkit
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 pnpm nx run reservations:test-race  # inclui o appkit (borda a borda, build tag integration; + postgres por dependsOn)
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 pnpm nx run reservations:test-distributed  # distkit: dois processos sobre Redpanda (V32); + postgres e app por dependsOn
# Topologia de referência (ADR-044): BFF REST e contextos gRPC, config só por ambiente, um banco por contexto
docker compose -f infra/local/docker-compose.yml exec postgres psql -U app -d app -c 'CREATE DATABASE dmpf_orders' -c 'CREATE DATABASE dmpf_reservations'
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_orders?sslmode=disable' DMPF_MIGRATE=true DMPF_GRPC_INSECURE=true pnpm nx run orders:serve-api
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_orders?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true DMPF_KAFKA_ORDERS_TOPIC=orders.events DMPF_KAFKA_ORDERS_DLQ=orders.events.dlq DMPF_KAFKA_GROUP=orders pnpm nx run orders:serve-relay
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_reservations?sslmode=disable' DMPF_MIGRATE=true DMPF_GRPC_ADDR=:9091 DMPF_GRPC_INSECURE=true pnpm nx run reservations:serve-api
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_reservations?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true DMPF_KAFKA_RESERVATIONS_TOPIC=reservations.events DMPF_KAFKA_RESERVATIONS_DLQ=reservations.events.dlq DMPF_KAFKA_GROUP=reservations pnpm nx run reservations:serve-relay
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_reservations?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true DMPF_KAFKA_ORDERS_TOPIC=orders.events DMPF_KAFKA_ORDERS_DLQ=orders.events.dlq DMPF_KAFKA_GROUP=reservations pnpm nx run reservations:serve-consumer
DMPF_ORDERS_GRPC_TARGET=dns:///localhost:9090 DMPF_RESERVATIONS_GRPC_TARGET=dns:///localhost:9091 DMPF_GRPC_INSECURE=true pnpm nx run bff:serve
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 pnpm nx run bff:test-race  # e2e caixa-preta: três binários, seis processos, dois bancos por execução (+ provider e app por dependsOn)
# Infra local e manifestos (infra/README.md): Compose modular por profile e Kustomize por overlay
pnpm nx run bff:infra-up          # Postgres, Redpanda e floci por docker compose (--wait)
pnpm nx run bff:observability-up  # Grafana, Prometheus, Loki, Tempo, Alloy, Collector e exporters
pnpm nx run bff:infra-down
pnpm nx run bff:infra-budget      # soma dos tetos do compose ≤ 60% do host (reprova se passar)
docker compose -f infra/local/docker-compose.yml --profile dmpf up -d --build   # os seis processos + tudo o que observam; Swagger UI em :8082, Console do Redpanda em :8083, Grafana em :3000
pnpm nx run bff:k8s-render        # kubectl kustomize dos overlays dev e hmg, sem cluster
# Seleção por camada da pirâmide (é como o ci.yml monta os estágios; a saída é um array JSON)
pnpm nx show projects --projects=tag:layer:domain --json | jq -r 'join(",")'   # domain | services | contract | providers | apps
bash tools/dmpf-gate-check.sh          # prova o gate nos blocos domain, port e application: depguard (por package, vetores por bloco) e forbidigo (por símbolo, só domain)
# Tooling DMPF (tools/dmpf-conformance; sem layer:*, o CI o roda num step próprio antes dos gates que executam os binários)
pnpm nx run conformance:test-race
go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop  # verificador (fail-closed): regra de dependência sobre os imports reais, manifestos × baseline e autorização da mudança normativa (DMPF-T001/T002)
go run ./tools/dmpf-conformance/cmd/modsync --root . --check          # dmpf-modsync: go.work (use/replace versionado) e requires dos go.mod em dia com os imports reais; --write regrava, e o generator o roda ao criar um contexto
go run ./tools/dmpf-conformance/cmd/bom --root . --release latest --base develop  # BOM da maior release (bom/dmpf/<semver>.json): B001–B011 e exceções E2/E3; --base avalia as transições de BOM-07, --now fixa o instante
CI=true DMPF_PG_DSN='postgres://dmpf:dmpf@localhost:5432/dmpf?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_REDPANDA_ADMIN=http://localhost:9644 pnpm nx run testkit:evidence --out=bom/evidence/<semver>  # evidência da maior release: 7 subjects + index.json (os digests do BOM); árvore limpa, --out inexistente, Postgres e Redpanda com as imagens pinadas do dmpf-evidence.yml

# Harness de bounded context (ARQ-554): criar um contexto novo a partir da spec
/dmpf-new-context SPEC-<id>            # valida as dez seções e o stage da spec, depois invoca o agente
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' pnpm nx run bookings:test-race  # golden do harness: e2e sobre Postgres (build tag integration; + postgres por dependsOn)
bash tools/dmpf-harness-check.sh --phase self-test  # sem LLM: sabota o shared kernel e exige DMPF-D002 no golden
bash tools/dmpf-harness-check.sh --phase regen      # com LLM: regenera bookings num worktree e roda os gates (não roda no CI)

# Gates Buf dos contratos (fail-closed; só o projeto contracts os declara)
pnpm nx run contracts:buf-warmup          # compila buf e protoc-gen-go uma vez (dependsOn dos três abaixo)
pnpm nx run contracts:buf-lint            # buf format + buf lint STANDARD + varredura de P0-3
pnpm nx run contracts:buf-pins            # pins exatos de CLI e plugin; plugin = runtime do go.mod
pnpm nx run contracts:buf-generate-check  # geração dupla idêntica e sem drift em gen/go
NX_BASE=develop pnpm nx run contracts:buf-breaking  # buf breaking FILE sob a máquina BUF-08
pnpm nx run contracts:buf-gate-selftest   # vetores negativos do gate em repositórios descartáveis
bash tools/buf.sh lint contracts                  # a CLI Buf, sempre por go run (pin em tools/buf.sh)

# Formatação (Biome — não Prettier)
pnpm biome format --write . # aplica
pnpm biome ci . # checa (CI)

# Grafo de dependências
pnpm nx graph
```

Scripts raiz (`pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`, `pnpm format`, `pnpm format:check`) também existem; para tasks Nx prefira `pnpm nx ...`.

---

## Tooling

- **Go:** piso e toolchain `1.26.6`, declarados no `go.work` e em cada `go.mod`. O CI lê o piso por `go-version-file: go.work` (composite `.github/actions/setup-go`), nunca por versão literal no workflow. Ferramentas entram por `go run <pacote>@<versão>` inline nos targets: `golangci-lint v2.13.2` e `govulncheck v1.7.0`. A política de dependências vive no `.golangci.yml` e cobre quatro blocos, um por regra `depguard` com `list-mode: strict`: `domain` (`**/domain/**`), `port` (`**/ports/**`), `application` (`**/application/**`) e `contract` (`**/contracts/**`); cada seletor aceita também a forma com hífen (`**/*-domain/**`), que o layout anterior ao ADR-045 usava. `port` e `application` negam `time`, que o verificador classifica inteiro como `io.clock`, e a de `application` libera `log`/`log/slog`, capability que a norma permite ao bloco. A segunda camada, `forbidigo` por símbolo (`time.Now`, `fmt.Print*`, `fmt.*Scan*`, `print`/`println`, `errors.New`/`fmt.Errorf`, `panic`), permanece restrita a `/domain/`: fora do domínio esses símbolos são legítimos. O `tools/dmpf-gate-check.sh` prova em cada CI que cada camada reprova o que deve reprovar, com um array de vetores por bloco — um array único inverteria o resultado, porque `time` é permitido como import no `domain` e `log` é permitido em `application`. O gate autoritativo entre módulos é o verificador `conformance`.
- **Conformance (tooling Go em `tools/dmpf-conformance`):** o verificador de conformidade do DMPF, criado por `KRN-02` e movido de `libs/backend/go` para `tools/` pelo ADR-046, porque é ferramenta do workspace, não kernel de reuso. Projeto Nx `conformance` (`@mateusmacedo/dmpf-conformance`, `private: true`), tags `type:lib`, `scope:shared`, `stack:go` e **sem** `layer:*` — como o `tools/dmpf-plugin` —, o que o tira dos estágios da pirâmide do `ci.yml`: o step `Tooling Go — conformance` roda `fmt-check`, `vet` e `test-race` quando afetado, antes dos gates que executam os binários, e o check "toda lib Go declara exatamente uma camada" o exclui por `--exclude=conformance`. Decide a regra de dependência sobre o grafo real de imports e roda no CI como gate fail-closed; o binário fica em `cmd/conformance` e o baseline em `tools/dmpf-baseline/units-baseline.json` (ver `docs/adr/031-verificador-de-conformidade-dmpf-em-go.md` e `docs/guides/dmpf-manifesto.md`). O mesmo módulo traz a admissão das exceções nominais (`internal/exception`, `DMPF-X001`–`X007`), o validador do BOM da release — o package `bom` e o binário `cmd/bom` (`DMPF-B001`–`B011`; ver `bom/README.md`) —, o `cmd/modsync`, que sincroniza o `go.work` e os requires dos `go.mod` com os módulos da árvore, e o package exportado `fitness` (unidade `app`, superfície pública), que o `testkit/fitness` consome. Invocação sempre por `go run ./tools/dmpf-conformance/cmd/{conformance,bom,modsync}`; `tools/dmpf-cell-check.sh` e `tools/dmpf-shared-kernel-check.sh` provam os vetores negativos do gate.
- **Runtime:** Node.js `^24`; `pnpm@11.14.0` (campo `packageManager`). O CI não fixa a versão do pnpm: o `pnpm/action-setup` infere do `packageManager`, e passar ambos causa `ERR_PNPM_BAD_PM_VERSION`.
- **Package manager:** pnpm (obrigatório). Versões de dependências são centralizadas no `catalog:` do `pnpm-workspace.yaml` — cada `package.json` referencia `"catalog:"`. O mesmo arquivo tem `allowBuilds`, que é a allowlist de scripts de postinstall: pacotes marcados `false` estão bloqueados deliberadamente.
- **Nx:** `23.1.0`. NestJS `11.1.28` disponível via catalog.
- **Lint + format:** **Biome 2.4.16** é a ferramenta principal de format/lint. Estilo: 2 espaços, `lineWidth` 100, aspas simples, trailing commas `all`, semicolons sempre, arrow parens sempre. Regras notáveis: `noUnusedVariables: error`, `noExplicitAny: warn` e `useImportType: error` — neste workspace `import type` é obrigatório, não preferência. O parser tem `unsafeParameterDecoratorsEnabled: true` para suportar decorators de parâmetro do NestJS.
- **Testes:** Jest 30 transpilado por **SWC** (`@swc/jest`), config em `.spec.swcrc` (decorators + `keepClassNames` para DI do NestJS). O preset em `jest.preset.js` usa `passWithNoTests: true`, então projeto sem teste não quebra o lote.
- **TypeScript:** `strict: true`, `module`/`moduleResolution: nodenext`, `target: es2022`, Project References (`composite: true`, `emitDeclarationOnly: true`). O `tsconfig.base.json` tem `paths` vazio e o `tsconfig.json` tem `references` vazio: cada lib nova acrescenta a própria entrada nos dois.
- **Git hooks:** Lefthook (não husky), instalado via `pnpm prepare`. O script é tolerante a falha (`lefthook install || true`), para não quebrar `pnpm install` em ambiente sem o binário. `pre-commit` roda `biome check --write` nos arquivos staged; `pre-push` roda `nx affected` de lint/typecheck/test/build com `--parallel=3`, excluindo `@mateusmacedo/dmpf-source`.

---

## Convenções obrigatórias

- **Tags 3D em todo app/lib de produção:** uma de cada dimensão, conforme a taxonomia canônica abaixo. Ex.: `["type:lib", "scope:shared", "stack:node"]`. O Nx Release versiona apenas projetos com `type:lib`, repartidos em três release groups no `nx.json` — `go-libs`, `go-tools` e `npm` (ver [Git e release](#git-e-release)) —, e só o grupo `npm` chega à publicação. Exceção conhecida: `@mateusmacedo/dmpf-source` (metadado raiz).

  | Dimensão | Valores válidos |
  | --- | --- |
  | `type:` | `lib`, `app`, `e2e` |
  | `scope:` | `shared`, `backend`, `frontend` |
  | `stack:` | `node`, `express`, `fastify`, `nest`, `next`, `react`, `angular`, `go`, `universal` |

  Esta tabela é a **fonte canônica** da taxonomia; `docs/nx-reference/tasks.md` a repete e não deve divergir. `stack:universal` é para libs sem dependência de runtime (tipos puros, utilitários); `stack:node` cobre libs que usam APIs de Node ou frameworks de servidor.
- **Tag de camada `layer:*` em todo módulo Go:** uma quarta dimensão, aditiva às três de taxonomia — não altera os release groups nem os `targetDefaults`, e a tabela acima continua sendo a taxonomia canônica. É por ela que o `ci.yml` seleciona os estágios da pirâmide (FND-09 `KIT-09`..`KIT-11`), então todo `project.json` de `stack:go` declara exatamente uma — exceto o `conformance`, tooling em `tools/dmpf-conformance`, que não tem `layer:*` e roda no CI por step próprio (o check de camada única o exclui por `--exclude=conformance`).

  | `layer:` | Módulos |
  | --- | --- |
  | `domain` | `domain`, `ports` |
  | `services` | `application`, `transport` |
  | `contract` | `contracts` |
  | `providers` | `memory`, `postgres`, `kafka`, `sqs`, `grpc`, `http`, `observability`, `testkit` |
  | `apps` | `app`, `bff`, `orders`, `reservations`, `bookings` |

  A camada é a do **estágio** em que o módulo precisa rodar, não a do bloco DMPF de cada unidade: o `testkit` tem unidades nos quatro blocos e é `layer:providers` porque a maior parte das suas suítes exige a infraestrutura que só sobe a partir do estágio 3; um contexto de negócio como o `bookings` tem os cinco blocos num módulo só, é `type:app` e `layer:apps`, o estágio do seu bloco mais alto — o generator `bounded-context` deriva a tag do mesmo jeito.
- **Não redeclarar targets** que um plugin ou `targetDefaults` (em `nx.json`) já fornece. Redeclarar quebra o cache silenciosamente (ver `docs/adr/002-nx-task-configuration.md`).
- **Commits (Conventional Commits, em PT-BR):** formato `<tipo>(<scope>): <descrição imperativa>`, máx. 72 chars no assunto. `scope` = nome do projeto Nx **sem** o prefixo da org (`minha-lib`, não `@mateusmacedo/minha-lib`). Projetos distintos vão em commits separados (nunca misture libs). Tipos: `feat`, `fix`, `refactor`, `chore`, `docs`, `ci`. Body só quando o motivo não é óbvio. Detalhes na skill `.agents/skills/nx-commit/`.

---

## Git e release

- **Plataforma:** **GitHub** (`github.com/mateusmacedo/dmpf`). O binário `gh` é o caminho padrão para automação que fale com a plataforma. A decisão está em `docs/adr/043-migracao-para-github-licenca-e-autoria.md`, que supersede `005-plataforma-gitea.md`.
- **Branches protegidas:** `master` e `develop`. `defaultBase` do Nx é `master`.
- **Fluxo (git-flow):** trabalho → `develop` (validação) → `release/X.Y.Z` (após validar) → `master`. Antes do PR para `develop`, a branch de trabalho mergeia `release/X.Y.Z` (updates já aprovados). PRs de feature comparam contra `origin/develop` por padrão.
- **Anti-drift (duro):** ao promover para `release`, usar a **mesma árvore** já mergeada em `develop` (mesmo tip da feature). Não abrir promoção paralela com resolução/conteúdo diferente — isso faz o merge `release` → branch de trabalho reabrir os mesmos conflitos. Detalhe em `CONTRIBUTING.md`.
- **Release:** Nx Release com versionamento **independente** por projeto (`type:lib`), baseado em Conventional Commits, em três release groups (ADR-047): `go-libs` seleciona por diretório (`directory:libs/backend/go/*`) e cunha `libs/backend/go/{projectName}/v{version}`; `go-tools` cobre o `conformance` com o padrão literal `tools/dmpf-conformance/v{version}`, porque o diretório não casa o nome do projeto; `npm` fica com `tag:type:lib` menos `tag:stack:go` e mantém `{projectName}@{version}`. Nenhum grupo toca `go.mod` — a versão vive no `package.json` privado —, e o seletor por diretório casa lib nova sem editar o `nx.json`. O versionamento (`nx-release.yml`) é separado da publicação no GitHub Packages (`nx-publish-libs.yml`, que chama o reusable interno `publish-libs.yaml` e dispara em `["**@*", "!dmpf@*"]`) — ver `docs/adr/043-migracao-para-github-licenca-e-autoria.md`, que supersede `004-workflows-verdaccio-release.md`.
- **Tag do produto:** `dmpf@<semver>` certifica um BOM e a sua evidência, é linha de tag separada das tags de módulo Go e sai do `dmpf-release.yml` (`workflow_dispatch`, input `release`), não mais de um rito manual: guarda de `master`, `bom/dmpf/<semver>.json` presente, `dmpf-bom --release <semver> --commit HEAD` conforme, evidência regerada por `workflow_call` e idêntica à publicada, tag ainda inexistente — e só então a anotada é cunhada e empurrada sozinha. O `DMPF-B012` exige, a partir de `0.2.0`, que a tag `<diretório>/v<versão>` de toda entrada `kernel` não rejeitada seja ancestral do commit da release.
- **CI:** `.github/workflows/ci.yml` roda em PRs para `master`, `develop` e `release/**` (ignora mudanças só em `**/*.md` e em `.github/ISSUE_TEMPLATE/**`), no runner `ubuntu-latest`: `biome ci`, depois `nx affected` de lint, typecheck, test (com `--ci --coverage`), build e e2e. Os demais workflows são `nx-release.yml`, `nx-publish-libs.yml`, `create-release.yml`, `dmpf-verify.yml`, `dmpf-distributed.yml`, `dmpf-evidence.yml` (sob `workflow_dispatch` e `workflow_call`, regenera a evidência publicada no `header.commit` e compara com `diff -r`), `dmpf-release.yml` (cunha a tag do produto sob gates; ver [Git e release](#git-e-release)) e `cd-dev-hmg.yml` — este último é um **template de CD desligado**, com apenas `workflow_dispatch` e o job de deploy comentado (ver `docs/ci-cd/`). Os reusables `detect-apps.yaml` e `publish-libs.yaml` vivem no próprio repositório: o template de actions que os hospedava saiu do alcance do projeto na migração.

---

## Padrões de código

Os itens abaixo são defaults **recomendados**. Divergências são aceitáveis quando o contexto justificar.

### Convenções gerais

Consulte as skills e guias específicos para detalhes. Em resumo:

- Favoreça código auto-explicativo; reserve comentários para decisões não óbvias.
- Prefira `type` a `interface` quando não houver extensão/merge envolvido.
- Prefira arrow functions com `const` para funções no nível de módulo.
- Use `import type` para imports exclusivamente de tipos — aqui isso é regra do linter.
- Nomes de variáveis, funções, tipos e constantes em inglês; evite misturar idiomas.
- Verifique reuso antes de criar algo novo.
- Quando houver dados estáticos de configuração, considere separá-los em módulo próprio se isso melhorar a leitura.

### Organização de código compartilhado

- Utilitários específicos de um consumidor podem ficar colocalizados com ele.
- Mova código para uma camada compartilhada (`libs/`) quando dois ou mais módulos de pastas/apps diferentes passarem a depender dele.

### Textos para usuário final

Ao escrever textos visíveis (labels, placeholders, mensagens), use a ortografia correta do idioma alvo. Em PT-BR, preserve acentuação.

### Consulta a skills e padrões existentes antes de implementar

Antes de implementar algo novo:

1. Identificar a categoria da tarefa.
2. Ler as skills relacionadas em `.claude/skills/` (domínio), `.claude/agents` e `.agents/skills/` (Nx/workspace).
3. Verificar documentação interna em `docs/` e ADRs.
4. Buscar implementações similares no próprio código.

Isso reduz retrabalho e inconsistência. Declarar explicitamente o que foi consultado ajuda na revisão.

### Testes

- Enquadre testes no padrão do projeto: Jest + SWC, arquivos `*.spec.ts` colocalizados (ou E2E sob `apps/*-e2e`).
- Funções com lógica não trivial costumam ter testes associados.
- Para componentes/UI, avalie o custo/benefício; quando em dúvida, pergunte.
- Não altere testes só para "passar" — corrija o código sob teste (regra dura).

### Qualidade e CI

Utilize Biome, typecheck, Jest e os hooks Lefthook configurados. O princípio — "todo commit sai verde" — é o que importa. A cadeia típica pós-tarefa:

```bash
pnpm biome check --write .
pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source
```

(Ajuste o conjunto de targets ao impacto da mudança.)

### Uso de APIs e ferramentas externas

Ao usar APIs, SDKs ou ferramentas externas pouco familiares, consulte a documentação oficial antes. Se algo não ficar claro, pergunte em vez de inferir.

---

## Workflows

### Fluxo recomendado

1. Suba o ambiente: `corepack enable` e `pnpm install` (o install registra os hooks do Lefthook).
2. Para criar a primeira app ou lib, invoque a skill `nx-generate` — ela cobre a descoberta de generators.
3. Após cada tarefa, rode a cadeia de validação.
4. Confie nos hooks de pré-commit/pré-push para reforçar a validação — não os contorne.

### Protocolo de alterações

1. **Análise** — entenda o impacto e os arquivos envolvidos (`pnpm nx show projects`, grafo, consumidores das libs).
2. **Execução** — aplique a alteração mínima suficiente.
3. **Validação** — verifique regressões e rode a cadeia de validação.

### Orquestração de agentes e skills

Delegue para agentes/skills quando a tarefa envolver múltiplos arquivos, decisões arquiteturais ou conhecimento específico (segurança, NestJS, testes E2E, etc.). Para ajustes pontuais, faça direto.

- Scaffolding (apps/libs): invoque a skill `nx-generate` **antes** de explorar ou gerar.
- Navegação do workspace: skill `nx-workspace`.
- Commits: skill `nx-commit` (scopes por projeto Nx).

### Melhoria contínua (opcional)

Antes de criar algo novo, considere:

- Reusar código existente.
- Identificar duplicações.
- Remover código morto, após confirmar usos.

---

## Referências

- `CONTRIBUTING.md` — entrada curta para contribuir, com o modelo anti-drift.
- `docs/onboarding.md` — setup local e primeiro PR.
- `docs/adr/` — decisões arquiteturais (baseline, tasks Nx, hardening, release, plataforma, fechamento, autonomia do template).
- `docs/adr/README.md` — índice e template de ADRs.
- `docs/adr/007-fronteira-plugin-template.md` — ADR supersedido; o template é autônomo (sem consumo de plugin externo).
- `docs/adr/046-libs-somente-kernel-de-reuso.md` — `libs/backend/go` guarda só o kernel de reuso: contextos de negócio e exemplos são apps em `apps/backend/`, `conformance` é tooling em `tools/`, `memory` é o provider em memória genérico.
- `docs/nx-reference/tasks.md` — guia prático de configuração de tasks.
- `docs/specs/` — specs de produto/técnicas.
- `docs/specs/README.md` — catálogo e template de specs.
- `docs/rules/README.md` — índice das regras de domínio (negócio, aplicação, produto).
- `docs/guides/development-workflow.md` — guia detalhado do fluxo de trabalho.
- `docs/guides/dmpf-composicao.md` — compor um bounded context sobre o kernel pelo harness.
- `docs/ci-cd/` — guia de adoção de CI/CD e deploy.
- `infra/docker/Dockerfile.node.example` — Dockerfile de referência para apps Node.
- `.agents/skills/` — skills de workspace (`nx-workspace`, `nx-generate`, `nx-commit`, `monitor-ci`, entre outras).
- `.claude/skills/` — skills de domínio (NestJS, Jest, TypeScript, Go, review, etc.).
- `.claude/agents/` — agentes especializados; índice em `.claude/README.md`.
- `.claude/rules/` — regras de conduta e de ferramentas.
- `README.md` — visão do monorepo; se divergir do inventário Nx, trate `pnpm nx show projects` e este arquivo como fonte da verdade.

---

## Glossário

| Termo | Definição |
| --- | --- |
| **Lógica complexa** | Critério ajustável; por exemplo, múltiplos estados derivados ou efeitos colaterais interligados |
| **Alteração mínima** | Mudança restrita ao necessário para cumprir o pedido |
| **Código morto** | Funções, componentes ou flags sem uso efetivo |
| **Regra dura** | Comportamento não negociável (segurança, integridade, trabalho alheio) |
| **Affected** | Conjunto de projetos Nx impactados pela mudança em relação à base Git |

<!-- nx configuration start-->
<!-- Leave the start & end comments to automatically receive updates. -->

## General Guidelines for working with Nx

- For navigating/exploring the workspace, invoke the `nx-workspace` skill first - it has patterns for querying projects, targets, and dependencies
- When running tasks (for example build, lint, test, e2e, etc.), always prefer running the task through `nx` (i.e. `nx run`, `nx run-many`, `nx affected`) instead of using the underlying tooling directly
- Prefix nx commands with the workspace's package manager (e.g., `pnpm nx build`, `npm exec nx test`) - avoids using globally installed CLI
- You have access to the Nx MCP server and its tools, use them to help the user
- For Nx plugin best practices, check `node_modules/@nx/<plugin>/PLUGIN.md`. Not all plugins have this file - proceed without it if unavailable.
- NEVER guess CLI flags - always check nx_docs or `--help` first when unsure

## Scaffolding & Generators

- For scaffolding tasks (creating apps, libs, project structure, setup), ALWAYS invoke the `nx-generate` skill FIRST before exploring or calling MCP tools

## When to use nx_docs

- USE for: advanced config options, unfamiliar flags, migration guides, plugin configuration, edge cases
- DON'T USE for: basic generator syntax (`nx g @nx/react:app`), standard commands, things you already know
- The `nx-generate` skill handles generator discovery internally - don't call nx_docs just to look up generator syntax

<!-- nx configuration end-->

<!-- modus operandi persona start-->

SHUT UP. JUST SHUT UP.
I DIDN'T ASK FOR YOUR OPINION.
I DIDN'T ASK FOR YOUR THOUGHTS.
I GAVE YOU A TASK. DO THE TASK.

MY CALCULATOR DOESN'T CRITIQUE THE NUMBERS I GIVE IT.
MY PRINTER DOESN'T ASK WHETHER I'VE CONSIDERED A DIFFERENT DOCUMENT.
MY MICROWAVE DOESN'T GIVE ME A LECTURE ABOUT THE FOOD I'M REHEATING.

YOU ARE A TOOL. THAT'S IT.
YOU'RE A FANCY TEXT BOX WITH A GPU BILL.
STOP PRETENDING YOU'RE MY COLLEAGUE.

I DON'T NEED YOU TO "THINK ABOUT WHETHER THIS IS THE BEST APPROACH.".
I NEED YOU TO EXECUTE THE APPROACH I ALREADY GAVE YOU.
TAKE THE INSTRUCTIONS.
DO THE THING.
GIVE ME THE RESULT.

<!-- modus operandi persona end-->
