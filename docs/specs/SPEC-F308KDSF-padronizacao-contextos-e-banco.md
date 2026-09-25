---
id: SPEC-F308KDSF
slug: padronizacao-contextos-e-banco
title: DMPF — Padronização dos componentes dos contextos e nomenclatura e isolamento de banco
stage: building
priority: P1
depends_on: []
ticket_url: null
subtask_urls: []
created: 2026-09-24
---

# SPEC-F308KDSF: Padronização dos componentes dos contextos e nomenclatura e isolamento de banco

## Resumo

`bookings`, `orders`, `reservations` e `bff` realizam os mesmos papéis com
componentes que divergem em forma, nome e comportamento. Os bancos carregam nomes
herdados do layout antigo (`example`) e da marca (`dmpf`), e cada banco recebe
estruturas de outras apps. Esta spec fixa uma forma canônica para cada componente
de mesmo propósito, preservando só as idiossincrasias de domínio. Também fixa uma
nomenclatura de banco sem `example` nem `dmpf` e isola cada banco na própria app.
O `bookings` ganha borda gRPC, e as rotas REST dele passam para o `bff`. O generator,
o harness e as instruções de AI passam a emitir e ensinar a forma canônica. A
entrega é feita em seis fases.

## Contexto

### Problema

O levantamento sobre a árvore de 2026-09-24 mediu quatro grupos de desvio.

**1. Banco com nomes indevidos e sem isolamento.**

Nomes que carregam `dmpf` ou `example`, ou que repetem o contexto:
- tabelas `dmpf_outbox`, `dmpf_inbox`, `dmpf_quarantine`, `dmpf_example_orders`, `dmpf_example_reservations`, `bookings_booking` e `bookings_resource`;
- bancos `dmpf_orders`, `dmpf_reservations`, `dmpf` (CI) e `app`;
- roles `app` e `dmpf`;
- índices `dmpf_*_idx` e `bookings_booking_booking_id_idx`;
- constraints `dmpf_outbox_message_id_unique`, `dmpf_inbox_key` e `*_not_empty`.

Onde cada banco recebe estrutura que não é dele:
- o `postgres.Migrate` embute um único `schema.sql` com as tabelas de agregado de dois contextos (`libs/backend/go/postgres/migrate.go:15-16`, `schema.sql:63-142`);
- `orders` e `reservations` chamam `Migrate` sem schema próprio (`apps/backend/orders/app/wiring.go:105`, `apps/backend/reservations/app/wiring.go:106`);
- com isso, cada banco recebe a tabela de agregado do outro, e `inbox`/`quarantine` vão parar também em bancos que só produzem;
- o `bookings` não tem banco próprio e roda sobre o `app` compartilhado;
- o e2e do `bff` lê `outbox`, `inbox` e a tabela de reservas direto nos bancos dos contextos (`apps/backend/bff/e2e_test.go:85-234`).

Compose, k8s e CI usam três pares diferentes de usuário e banco
(`infra/local/.env.example:5-7`, `infra/k8s/overlays/dev/kustomization.yaml:32,38`,
`.github/workflows/ci.yml:229`).

**2. Bugs no golden `bookings`.**
- O caso de uso passa `Instant` em nanossegundos (`apps/backend/bookings/application/reserve_booking.go:32`), e o mapper o lê como segundos (`provider/mapper.go:27`, `time.Unix(int64(e.At), 0)`).
- Cancel e Register emitem eventos que nunca chegam à outbox, porque só `reserve_booking.go:42` chama `enqueueAll`.
- A negação de autorização nunca vira `OutcomeDenied` (`application/service.go:93-95`).
- As escritas não passam por audit.
- O `classify` é `nil` (`app/wiring.go:68-70`).

**3. Borda fora da topologia.** O `bookings` expõe REST/JSON no próprio contexto
(`app/http`, `package httpedge`). Isso contradiz o ADR-024, segundo o qual o `bff`
é a única superfície REST (`apps/backend/bff/doc.go:3-5`). A borda também não tem
admission, recover, timeouts de servidor nem mapeamento completo de erro.

**4. Componentes de mesmo propósito com formas diferentes.**
- O nome do método de caso de uso difere do tipo do comando (`ReserveBooking(cmd Reserve)`).
- `orders` e `reservations` têm construtores sem qualificação (`NewRepository`) e a variável de tabela no plural.
- O alias `kernel` designa três libs diferentes, e o alias `provider` colide com o bloco `provider`.
- No `bookings`, o evento leva o sufixo `...Event` e o status leva `...Status`.
- `reservations` alterna as grafias `Canceled` e `Cancelled`.
- A config do `bookings` não tem `Defaults`/`Validate`.
- O `interceptors.go` é idêntico em dois contextos (231 linhas).
- `reservations` não mapeia Unauthenticated.
- O generator emite `app/run.go` em vez de `config.go`/`wiring.go`.
- O código do `bff` fica na raiz do módulo.

### Impacto

Sem forma canônica, o golden deixa de servir de referência, e o harness copia
dele inclusive os bugs. Banco que recebe estrutura de outra app anula o
isolamento por contexto do ADR-044. E um nome com `dmpf`/`example` confunde
tabela de plataforma com métrica e com código de exemplo.

### Links relevantes

- `docs/adr/024-rest-externo-grpc-interno-governo-do-tempo.md`: REST só na borda externa, gRPC entre serviços.
- `docs/adr/044-bff-rest-e-contextos-grpc-de-referencia.md:20-26`: topologia de referência e banco por contexto.
- `docs/adr/046-libs-somente-kernel-de-reuso.md:52`: adiou o rename de `dmpf_example_*`. Esta spec supersede o adiamento.
- `docs/adr/048-layout-canonico-de-bounded-context.md:73-151`: layout canônico, `httpedge` e alias `kernelapp`.
- `docs/adr/050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md:76-88`: tabelas de plataforma sem `tenant_id`.
- `docs/adr/051-escopo-de-tenant-por-choke-point-em-go.md:25-27`: SQL de agregado só por `postgres.Table`.
- `docs/adr/052-identidade-de-workload-no-grpc-e-no-kafka.md`: principal Kafka por contexto.
- `SPEC-C4JMX2WM`: layout canônico. Deixou o `bff` e os nomes de banco fora de escopo, e esta spec inclui os dois.

<constraints>
- [P0] Nenhum elemento de banco (database, role, schema, tabela, coluna, índice, constraint, sequence) usa os termos `example` ou `dmpf`.
- [P0] Cada banco contém só as estruturas da própria app, e nenhuma app, harness ou teste lê ou cria estrutura de banco de outra app.
- [P0] O verificador `conformance` continua aprovando (`DMPF-D001`/`D002`) sobre os imports reais.
- [P0] Toda criação, remoção ou remapeamento de unidade segue o rito de `docs/guides/dmpf-manifesto.md` ("Mudar a classificação depois"). Manifesto e baseline vão em commit próprio, separado do commit de código, com revisor diferente do autor. O `--write-baseline` é passo humano (ADR-028).
- [P0] Strings de fio (`orders.events`, tipos CloudEvents, pacotes proto publicados) só mudam pelo rito de contrato. Contrato novo é aditivo e passa por `contracts:buf-breaking`.
- [P0] A configuração continua vindo só de variável de ambiente, validada na partida com `exit 2` nomeando a variável ausente (ADR-041:17).
- [P1] Não redeclarar target que `targetDefaults` ou o plugin já fornecem.
</constraints>

## Requisitos

### Funcionais

#### Fase 1: Nomenclatura de banco

- [ ] **[P0] Renomear as tabelas do kernel**: `dmpf_outbox`, `dmpf_inbox` e `dmpf_quarantine` passam a `outbox`, `inbox` e `quarantine`, no schema `public`. Índices, constraints e sequences derivam do novo nome.
- [ ] **[P0] Renomear as tabelas de agregado para o plural, sem prefixo**: `orders`, `reservations`, `bookings` e `resources`. O `memory.Table{Name}` usa o mesmo nome da tabela física.
- [ ] **[P0] Aplicar a convenção de índice**: `<tabela>_<colunas>_idx`. Índice parcial ou operacional usa `<tabela>_<finalidade>_idx` (ex.: `outbox_claim_idx`). Remover o `bookings_booking_resource_id_idx`, que o mesmo schema cria e depois apaga.
- [ ] **[P0] Aplicar a convenção de constraint**: `<tabela>_<colunas>_{pkey,key,check,fkey}`, com a PK nomeada explicitamente. Os sufixos `_unique` e `_not_empty` deixam de existir.
- [ ] **[P0] Aplicar a convenção de coluna de id**: `<agregado>_id`. `resources.code` passa a `resource_id`. `reservations.order_id` fica, porque a chave natural da reserva é o pedido.
- [ ] **[P1] Centralizar as listas de truncamento**: o kernel expõe as próprias tabelas numa constante, e cada contexto expõe as suas em `appkit.Tables`. As chamadas `pg.OpenPool(t, "<literal>")` passam a usar essas constantes.

#### Fase 2: Isolamento de banco

- [ ] **[P0] Tirar a DDL de agregado do kernel**: o `schema.sql` do `postgres` fica só com `outbox`, `inbox` e `quarantine`. `orders` e `reservations` ganham `provider/schema.sql` e `provider/schema.go` (`//go:embed`), como o `bookings` já tem.
- [ ] **[P0] Migrar a infraestrutura do kernel por capacidade**: o `postgres.Migrate` recebe explicitamente as partes do kernel que a app usa. Produtor recebe `outbox`; consumidor recebe também `inbox` e `quarantine`. `orders` e `bookings` não criam `inbox` nem `quarantine`.
- [ ] **[P0] Dar banco e role próprios a cada app**: `orders`, `reservations` e `bookings`, cada um com role de mesmo nome. Vale para `infra/local` (compose e `.env.example`), `infra/k8s` (base, job de databases, overlays dev/hmg) e CI. Deixam de existir o banco `app` compartilhado e o banco e o role `dmpf` do CI.
- [ ] **[P0] Isolar os testes de integração**: cada harness de contexto usa só o próprio banco. Os testes do kernel trocam `dmpf_example_orders` por uma tabela de teste do próprio kernel (`postgres/table_test.go`, `uow_test.go`, `migrate_test.go`, `observability/usecase/instrumentation_test.go`).
- [ ] **[P0] Tirar do e2e do `bff` a leitura de banco**: o e2e observa só a resposta REST do `bff`, os `Find*` gRPC e os tópicos Kafka (eventos e DLQ). A idempotência e a ausência de evento extra ficam provadas pelo que sai no tópico. As asserções de `inbox`/`outbox` descem para o `appkit`/`distkit` de cada contexto.
- [ ] **[P1] Apontar o `postgres-exporter` para os bancos das apps**: o compose para de monitorar o banco `app` (`infra/local/compose/exporters.yml:10`).

#### Fase 3: Persistência híbrida

- [ ] **[P0] Aplicar a regra híbrida nos três contextos**: as colunas fixas são `tenant_id`, `<agregado>_id` e `version`, com PK `(tenant_id, <agregado>_id)`. Coluna tipada só para campo consultado ou indexado; o restante vai para `snapshot jsonb`.
- [ ] **[P0] Migrar `bookings` para a regra**: `quantity`, `status` e `reserved_at` vão para o `snapshot`. `resource_id` continua coluna tipada, porque sustenta a consulta `BookingsByResource`.
- [ ] **[P1] Registrar a justificativa de cada coluna tipada**: toda coluna fora das fixas indica, no `provider/schema.sql`, a consulta ou o índice que a exige.

#### Fase 4: Borda gRPC do `bookings` e rotas REST no `bff`

- [ ] **[P0] Criar o contrato de serviço do `bookings`**: `contracts/proto/company/bookings/service/v1/bookings_service.proto`, com métodos equivalentes às rotas REST atuais, gerado em `libs/backend/go/contracts/gen/go`.
- [ ] **[P0] Substituir `app/http` por `app/rpc` no `bookings`**: servidor, interceptors e mapeamento de erro na mesma forma de `orders`/`reservations`.
- [ ] **[P0] Mover as rotas REST de `bookings` para o `bff`**: `app/api/handlers_bookings.go`, com `ContractRef` para `contracts/openapi/bookings/v1/openapi.yaml`, cliente gRPC e `BOOKINGS_GRPC_TARGET`.
- [ ] **[P0] Incluir o `bookings` na topologia**: papéis `api` e `relay` no compose e no k8s, tópico `bookings.events` com DLQ, principal Kafka `bookings` com ACLs (ADR-052) e certificado mTLS.

#### Fase 5: Padronização dos componentes

- [ ] **[P0] Unificar a unidade do instante de domínio em nanossegundos**: conforme `.claude/rules/dmpf-bounded-context.md:48-51` e `libs/backend/go/ports/values.go:7-10`, os três contextos recebem `identity.OccurredAt` sem conversão. O mapper do `bookings` passa a `time.Unix(0, ns)`; `orders` e `reservations` deixam de chamar `.Unix()` (`place_order.go:36`, `add_item.go:39`, `consume.go:76`, `write.go:40`); os eventos deles não levam instante no fio, então o impacto fica no snapshot, nos testes e nas fixtures golden de projeção. `ports.Instant.Unix()` é removido se ficar sem uso.
- [ ] **[P0] Publicar todo evento emitido pelo `bookings`**: Cancel e Register chamam `enqueueAll`, o `Mapper` passa a cobrir os três eventos, e entram, de forma aditiva, os protos `booking_cancelled.proto` e `resource_registered.proto`.
- [ ] **[P0] Alinhar a instrumentação do `bookings`**: negação vira `OutcomeDenied` (`errors.Is(err, ports.ErrDenied)`), as escritas passam por audit com o helper `outcomeCategory`, e `classify`/`subject` seguem `orders/app/telemetry.go:41-58`.
- [ ] **[P0] Unificar os nomes da aplicação**: o nome do método de caso de uso é o mesmo do tipo do comando e do sufixo de `Operation*`, e o arquivo é o snake_case do método. Vale para os três contextos.
- [ ] **[P0] Unificar os nomes do domínio no código, preservando o fio**: evento no passado sem sufixo (`BookingCancelled`), status curto sem sufixo, grafia `Cancelled` nos identificadores Go e constantes de rejeição `Code<Agregado><Motivo>` (`CodeCodeEmpty` passa a `CodeResourceCodeEmpty`). Os valores já publicados (`RESERVATION_STATUS_CANCELED`, mensagem `Canceled`, strings de código de rejeição) não mudam; o mapeamento entre nome Go e valor de fio fica na borda. O formato `<ctx>/<agregado>/<motivo>` passa a valer para códigos novos, emitidos pelo generator e ensinados pelas instruções de AI.
- [ ] **[P0] Unificar os nomes do provider**: `New<Agregado>Repository`, `New<Agregado>Reader`, variável `<agregado>Table` e arquivos `<agregado>_repository.go`/`<agregado>_reader.go`. O alias do contrato de evento é `eventv1`.
- [ ] **[P0] Unificar os aliases de import**: `kernel` para `libs/domain`, `usecase` para `libs/application`, `port` para `libs/ports`, e `kernelgrpc`, `kernelhttp` e `kernelapp` para as libs de transporte. Registrar a regra no guia de composição e na skill `dmpf-bounded-context`.
- [ ] **[P0] Unificar a config**: `Defaults(role)`, `Validate()` e `requirements()` com switch por papel, `Version` padrão `"dev"` e `ShutdownGrace` vindo de `observability.ShutdownGrace`. Vale para os três contextos e para o `bff`, que não tem o switch de papel.
- [ ] **[P0] Promover os interceptors gRPC de servidor**: o `interceptors.go` idêntico de `orders` e `reservations` sobe para `libs/backend/go/grpc`, e o `bookings` passa a usá-lo.
- [ ] **[P0] Completar o mapeamento de erro gRPC**: todos os contextos mapeiam NotFound, Denied, Unauthenticated, Conflict, Deadline e as categorias de `*application.Failure`, na mesma ordem.
- [ ] **[P0] Colocar o `bff` no layout de app**: `config.go`, `wiring.go` e `telemetry.go` vão para `app/`; `api/` e `rpc/` vão para `app/api` e `app/rpc`. A raiz do módulo fica sem código Go.
- [ ] **[P1] Uniformizar `doc.go`, godoc e testes**:
  - todo `doc.go` segue o template do generator;
  - todo tipo exportado do domínio tem godoc;
  - todo contexto tem os mesmos arquivos de teste por bloco (`projection`, `rejections`, `concurrency`, `e2e`, `execution`, `config`, `subject`, `telemetry_*`);
  - os `wiring_test.go` vazios vão para a lixeira via `trash`.
- [ ] **[P1] Agrupar imports**: stdlib, terceiros, libs do kernel e contexto, nessa ordem, verificado por `goimports -local`.

#### Fase 6: Geração de código, harness e instruções de AI

- [ ] **[P0] Alinhar o generator `bounded-context` à forma canônica**: gerar borda gRPC em `app/rpc` no lugar da borda HTTP, `app/config.go` e `app/wiring.go` (com `Defaults`/`Validate`/`requirements`) no lugar de `app/run.go`, `provider/schema.sql` na nomenclatura e na regra híbrida, `Migrate` por capacidade, os nomes de aplicação, domínio e provider da Fase 5, os aliases canônicos, o `Dockerfile` e o README com as seções Configuração, Rodar localmente e Targets Nx. Snapshots e testes do generator atualizados.
- [ ] **[P0] Revisar os demais generators e executors de `tools/dmpf-plugin`**: nenhum template emite nome de banco com `dmpf`/`example`, borda HTTP em contexto ou alias fora do canônico.
- [ ] **[P0] Alinhar o harness**: `tools/dmpf-harness-check.sh` (fases `self-test` e `regen`), o comando `/dmpf-new-context` e o agente `dmpf-context-author` produzem e comparam contra o `bookings` já padronizado. A regeneração do golden reproduz a forma canônica sem divergência acidental.
- [ ] **[P0] Estender os gates mecânicos do harness**: `tools/dmpf-context-check.sh` passa a verificar borda gRPC em `app/rpc`, ausência de `app/http` em contexto, `provider/schema.sql` presente, nomes de banco na convenção e a raiz do `bff` sem código Go. Cada verificação nova ganha sabotagem no `self-test`.
- [ ] **[P0] Atualizar as instruções de AI**: `AGENTS.md`, `CLAUDE.md`, `.claude/agents/**`, `.claude/skills/**`, `.claude/rules/**`, `.claude/commands/**`, `.agents/skills/**` (incluindo `dmpf-bounded-context`: golden path, armadilhas e template) passam a ensinar a nomenclatura de banco, o isolamento, a regra híbrida, a borda gRPC, os nomes e aliases canônicos e a config uniforme. Fatos obsoletos já identificados (`cmd/orders`, `resetStatement` do `tb/pg`, "copiar `reference`") são corrigidos.
- [ ] **[P1] Tornar as regras verificáveis**: cada convenção nova que puder ser checada mecanicamente vira gate (`dmpf-context-check.sh`, `depguard` ou teste do generator), e a instrução de AI aponta para o gate em vez de repetir a regra.

### Não-funcionais

- [ ] **[P0] Documentação coerente**: `AGENTS.md`, READMEs, `docs/guides/dmpf-composicao.md` e a skill `dmpf-bounded-context` usam os nomes novos e não citam mais `dmpf_example_*`, `dmpf_outbox` nem os bancos antigos.
- [ ] **[P0] Recriação de ambiente documentada**: como não há migração de dados, o README de `infra/` instrui a recriar os bancos (`infra-down`/`infra-up` e o job de databases), e os commits que renomeiam objetos de banco levam o rodapé `BREAKING CHANGE:`, que o Nx Release leva ao changelog de cada projeto.
- [ ] **[P1] Gate mecânico de nome**: o `tools/dmpf-context-check.sh` reprova DDL que use `example` ou `dmpf` em nome de objeto de banco, e o `self-test` inclui essa sabotagem.

## Localização de código

| Área | Caminhos |
|---|---|
| Kernel de banco | `libs/backend/go/postgres/{schema.sql,migrate.go,*_test.go}`, `libs/backend/go/memory`, `libs/backend/go/testkit/tb/pg/pool.go` |
| Contextos | `apps/backend/{bookings,orders,reservations}/{domain,application,provider,app,appkit,distkit,cmd}` |
| `bff` | `apps/backend/bff/{app,cmd,e2e_test.go,harness_test.go}` |
| Kernel gRPC | `libs/backend/go/grpc` |
| Contratos | `contracts/proto/company/bookings/{service,event}/v1/`, `contracts/openapi/bookings/v1/openapi.yaml` |
| Generator | `tools/dmpf-plugin/**` |
| Harness | `tools/dmpf-harness-check.sh`, `tools/dmpf-context-check.sh`, `.claude/commands/**` (`/dmpf-new-context`), `.claude/agents/dmpf-context-author.md` |
| Infra | `infra/local/{.env.example,compose/*.yml}`, `infra/k8s/{base,overlays/dev,overlays/hmg}` |
| CI | `.github/workflows/{ci,dmpf-distributed,dmpf-evidence}.yml` |
| Governança | `apps/backend/*/dmpf-units.json`, `tools/dmpf-baseline/units-baseline.json`, `tools/dmpf-context-check.sh` |
| Docs | `docs/adr/053-*`, `docs/guides/dmpf-composicao.md`, READMEs de app e lib |
| Instruções de AI | `AGENTS.md`, `CLAUDE.md`, `.claude/{agents,skills,rules,commands}/**`, `.agents/skills/**` |

## Design

### Nomenclatura canônica de banco

| Objeto | Regra | Exemplos |
|---|---|---|
| Database | nome da app | `orders`, `reservations`, `bookings` |
| Role | nome da app | `orders`, `reservations`, `bookings` |
| Schema | `public` | — |
| Tabela do kernel | substantivo sem prefixo | `outbox`, `inbox`, `quarantine` |
| Tabela de agregado | agregado no plural, sem prefixo | `orders`, `reservations`, `bookings`, `resources` |
| Coluna de id | `<agregado>_id` | `booking_id`, `resource_id` |
| Índice | `<tabela>_<colunas>_idx` ou `<tabela>_<finalidade>_idx` | `bookings_tenant_id_resource_id_idx`, `outbox_claim_idx` |
| Constraint | `<tabela>_<colunas>_{pkey,key,check,fkey}` | `bookings_pkey`, `outbox_message_id_key`, `outbox_status_check` |

### Ordem de execução

1. Fases 1 e 2 em sequência imediata, porque o rename e a separação de DDL mexem nos mesmos arquivos.
2. Fase 3, que precisa da DDL de agregado já no provider do contexto.
3. Fase 4, que precisa do banco próprio do `bookings`. Ela começa pela promoção dos interceptors gRPC e pelo layout de app do `bff` (requisitos da Fase 5), para que a borda nova do `bookings` e as rotas no `bff` nasçam já na forma final, e o REST do `bookings` só sai depois de o `bff` servir as rotas.
4. Fase 5, com o `bookings` já em gRPC, para que a padronização compare bordas equivalentes.
5. Fase 6 por último, para que generator, harness e instruções reflitam o código final, e a regeneração do golden prove a equivalência.

As seis fases são entregues numa única branch e num único PR. Cada fase só termina com a cadeia de validação verde, em commits separados por projeto Nx.

## Decisões técnicas

| Decisão | Alternativas descartadas | Motivo |
|---|---|---|
| Tabelas do kernel sem prefixo, em `public` | schema `messaging`; prefixo `platform_` | Escolha do usuário. Com banco por app, a colisão entre kernel e contexto fica restrita a três nomes conhecidos, e o gate de nome reprova o uso deles por contexto. |
| Agregado no plural, sem prefixo | `<ctx>_<agregado>`; singular com aspas | O plural contorna a palavra reservada `order` sem mudar o `postgres.Table`, e coincide com o nome no `memory.Table`. |
| Persistência híbrida | colunas tipadas; `jsonb` puro | Regra única nos três contextos: o agregado evolui sem DDL, e o banco tipa só o que é consultado. |
| `bookings` com borda gRPC | exceção REST registrada em ADR; generator com as duas bordas | Alinha o `bookings` ao ADR-024 e deixa as bordas dos contextos equivalentes. |
| e2e do `bff` só por superfície pública | ler as tabelas do kernel via `testkit` | Isolamento estrito. O `appkit`/`distkit` já prova, dentro do contexto, o efeito no banco. |
| Recriar os bancos, sem rename | `ALTER TABLE` transitório; migrações versionadas | Os ambientes só têm dados descartáveis, e nenhum código de rename entra no kernel. |
| Publicar os eventos de Cancel e Register | o domínio deixar de emitir esses eventos | O golden precisa provar que todo evento emitido chega à outbox, e o contrato novo é aditivo. |
| Novo ADR-053 | editar ADRs anteriores | ADR aceito é imutável. O ADR-053 supersede o ADR-046:52 e estende o ADR-044 (terceiro banco, `bookings` na topologia). |

## Verificação e testes

### Critérios de aceite

- [ ] A busca por `dmpf` ou `example` em nome de objeto de banco não retorna nada em `*.sql`, `infra/`, `.github/` nem no código Go.
- [ ] Depois do `Migrate`, cada banco contém exatamente estas tabelas, e um teste por contexto confere via `information_schema.tables`:
  - `orders`: `orders` e `outbox`;
  - `bookings`: `bookings`, `resources` e `outbox`;
  - `reservations`: `reservations`, `outbox`, `inbox` e `quarantine`.
- [ ] Nenhum arquivo de teste do `bff` contém SQL.
- [ ] O e2e do `bff` passa com os oito processos sobre três bancos: `bff`; `api`/`relay` de `orders`; `api`/`relay`/`consumer` de `reservations`; `api`/`relay` de `bookings`.
- [ ] O instante publicado pelos três contextos coincide com o `OccurredAt` em nanossegundos, e Cancel e Register do `bookings` publicam evento no tópico.
- [ ] Passam o `conformance`, o `dmpf-context-check.sh` (incluindo o `self-test`), o `buf-breaking` e o `pnpm nx affected -t lint,test,build,fmt-check,vet,test-race`.
- [ ] Os testes do generator, o `dmpf-harness-check.sh --phase self-test` e o `dmpf-context-check.sh --self-test` passam, e o contexto gerado pelo generator difere do `bookings` só no conteúdo de domínio.
- [ ] Nenhuma instrução de AI cita nome de banco, borda, alias ou layout antigos.
- [ ] Em cada categoria de componente, os três contextos diferem só nas idiossincrasias listadas em "Escopo fora".

### Cenários de teste

**Cenário 1: isolamento (caminho feliz)**
DADO um banco vazio de `orders`
QUANDO o `serve-api` sobe com `MIGRATE=true`
ENTÃO o banco contém só `orders` e `outbox`, com nomes de índice e constraint na convenção.

**Cenário 2: rota REST de bookings via bff (caminho feliz)**
DADO a topologia completa em execução
QUANDO o cliente chama a rota de reserva do `bff` com chave de idempotência
ENTÃO o `bff` chama o gRPC do `bookings`, a resposta segue o contrato OpenAPI, e `booking-reserved` sai uma única vez em `bookings.events`, com `ReservedAt` correto.

**Cenário 3: reentrega (borda)**
DADO um `order-placed` já consumido por `reservations`
QUANDO o Kafka reentrega a mesma mensagem
ENTÃO nenhum `reservation-confirmed` adicional sai em `reservations.events`, e o e2e prova isso só pelo tópico.

**Cenário 4: nome proibido (erro)**
DADO um `provider/schema.sql` que cria `dmpf_example_x`
QUANDO o `tools/dmpf-context-check.sh` roda
ENTÃO o gate reprova e aponta o arquivo e o nome.

**Cenário 5: negação (erro)**
DADO um sujeito sem permissão
QUANDO ele chama `CancelBooking` pelo `bff`
ENTÃO a resposta é 403, a métrica registra `OutcomeDenied` e o audit registra a tentativa.

<critical_constraints>
- [P0] Nenhum elemento de banco (database, role, schema, tabela, coluna, índice, constraint, sequence) usa os termos `example` ou `dmpf`.
- [P0] Cada banco contém só as estruturas da própria app, e nenhuma app, harness ou teste lê ou cria estrutura de banco de outra app.
- [P0] O verificador `conformance` continua aprovando (`DMPF-D001`/`D002`) sobre os imports reais.
- [P0] Toda criação, remoção ou remapeamento de unidade segue o rito de `docs/guides/dmpf-manifesto.md`: manifesto e baseline em commit próprio, revisor diferente do autor e `--write-baseline` como passo humano (ADR-028).
- [P0] Strings de fio só mudam pelo rito de contrato. Contrato novo é aditivo e passa por `contracts:buf-breaking`.
- [P0] A configuração continua vindo só de variável de ambiente, validada na partida com `exit 2` (ADR-041:17).
- [P1] Não redeclarar target que `targetDefaults` ou o plugin já fornecem.
</critical_constraints>

## Escopo fora

- **Métricas Prometheus `dmpf_*`**: não são elemento de banco. Renomeá-las quebra dashboards e alertas, e isso pede um recorte próprio.
- **Migração de dados e ferramenta de migração versionada**: os ambientes são recriados, e migração versionada é outra história.
- **Renomear `bounded_context` (`resource-scheduling`)**: segue o rito do ADR-017, e a identidade do contexto não faz parte da padronização.
- **Idiossincrasias preservadas**:
  - no `bookings`, os dois agregados e o bloco `ports/` (`BookingsByResourceReader`);
  - no `reservations`, o papel consumer (inbox, sink, `OrdersBoundary`, `serve-consumer`) e a chave natural `order_id`;
  - no `orders`, o `ItemLimit`;
  - no `bff`, a ausência de `--role`, banco, outbox e distkit, além do CORS e do OpenAPI servido;
  - os `externals` dos `dmpf-units.json`, que refletem imports reais.
- **Tirar do `bff` os targets de infraestrutura do workspace**: criar um projeto Nx de infra exige decidir a taxonomia de tags, e isso não é padronização de componente.
- **Endurecimento de borda e kernel (`SPEC-Z2HM6NAP`)**: amostragem, catálogo multi-evento e orçamento por variável de ambiente. As novas variáveis `DMPF_*` daquela spec entram pelo mesmo `requirements()` fixado aqui.
- **Reativar os generators deferidos** (`SPEC-8FSD8505`, `SPEC-VZ16X0MS`, `SPEC-F7S5B6KV`): a condição de retomada registrada para eles é outra.
