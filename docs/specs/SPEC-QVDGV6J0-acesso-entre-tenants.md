---
id: SPEC-QVDGV6J0
slug: acesso-entre-tenants
title: DMPF — Acesso entre tenants declarado: leitura de plataforma sobre postgres.Table e projeção entre tenants
stage: backlog
priority: P1
depends_on: [SPEC-9B6SHEH8, SPEC-QPRR24C4]
ticket_url: https://linear.app/mmda/issue/DEVS-100/spec-qvdgv6j0-dmpf-acesso-entre-tenants-declarado-leitura-de
subtask_urls: []
created: 2026-10-12
---

# SPEC-QVDGV6J0: Acesso entre tenants declarado

## Resumo

Dar ao kernel os dois caminhos que o isolamento por tenant hoje não tem e que a
norma admite quando declarados: a leitura de plataforma, que alcança todos os
tenants de uma tabela, e a projeção, que grava dado no tenant de destino de uma
mensagem recebida em outro tenant. O IDN-19 admite alcance irrestrito declarado,
mas `postgres.Table` filtra sempre pelo tenant do portador e transforma a leitura
cruzada em `NotFound`; a declaração de alcance existe só em `http.Route`. Sem
esses caminhos, quem precisa deles é levado a abrir o driver no contexto, o que o
ADR-051 proíbe. A spec entrega os dois como API do kernel, com declaração
explícita, autorização, trilha de auditoria que falha fechada e gate estático, sem afrouxar a
leitura comum.

Origem: consumidor externo antecipa-market, SPEC-GXGASP24 (DEVS-76), que precisa
de backoffice com leitura entre tenants e de projeção de dados no tenant do
financiador. O Compliance Ledger do mesmo consumidor é o segundo usuário da leitura
de plataforma: o selo (SPEC-MSA607GT) e o verificador leem as cabeças da cadeia em
todos os tenants, e é por isso que a prioridade é P1.

## Contexto

### Problema

- O IDN-19 exige que a operação de plataforma declare o escopo de dado que
  alcança e admite o alcance "todos os tenants", declarado e autorizado
  (`docs/dmpf/contexto-erros-seguranca.md:929`); o IDN-11 e o IDN-12 mantêm o
  escopo e o registro de acesso cruzado para todo o resto (`:878-879`).
- O ADR-051 fecha o escopo num choke point e diz que o que o `Table` não cobre
  "exige estender o kernel, nunca reabrir o driver no contexto" (`:47`).
- Projetar dado no tenant do financiador exige gravar com um tenant diferente do
  que a mensagem carrega, e hoje o tenant da mensagem é sempre o do portador de
  quem a publicou. Isso colide com o ADR-050:23 (a escrita do consumidor é
  escopada como qualquer outra), com o CTX-13 e o CTX-24 a CTX-26.
- A dependência da SPEC-QPRR24C4 é de verificação: os testes de integração de
  plataforma e de projeção rodam sob a role de runtime sem posse
  (`OpenRuntimePool`), a role com que o caminho executa em produção; sem ela, o
  teste passaria com poder de dono.

### Estado atual

Verificado no código e nas normas de `lidercap-platform` em 2026-10-12. Os
caminhos `.go` da coluna "Evidência" são relativos a `libs/backend/go/`.

| Ponto | Evidência |
| ----- | --------- |
| `PlatformReach` (`ReachUndeclared`, `ReachAllTenants`) só existe em `http.Route`, e `Validate` o aceita apenas com `RequireNeither` | `libs/backend/go/http/route.go:54-62,81,102-107`; `libs/backend/go/http/README.md:119-124` |
| Nenhuma outra peça consome a declaração: persistência não a lê | `rg PlatformReach` só alcança `http/route.go`, testes e README |
| A autorização de aplicação nega a operação sem tenant | `libs/backend/go/application/authorize.go:39-41` |
| `Table` resolve o tenant do portador e recusa na ausência | `libs/backend/go/postgres/table.go:15-18,235-245` |
| Leitura cruzada vira `CrossTenantAccess`, que responde `ErrNotFound` e vira evento IDN-12 | `postgres/table.go:139-152,276-289`; `ports/repository.go:14-25`; `observability/usecase/instrumentation.go:30-31,117-140` |
| `ExecutionContext` tem nove campos (CTX-01), com sujeito e tenant opcionais; `Identity` também | `ports/execcontext.go:57-74`; `ports/authn.go:22-29` |
| A borda gRPC monta o contexto com tenant do metadado e sem sujeito | `grpc/interceptor_context.go:35-37,272-280,339-345`; IDN-02 em `contexto-erros-seguranca.md:810` |
| O tenant da mensagem é o do portador; `OutboxEntry` não tem campo de tenant | `postgres/outbox.go:69,121-134`; `ports/outbox.go:26-35` |
| O relay copia o tenant ao envelope e o consumidor reconstrói o contexto a partir dele | `app/relay/record.go:49-54`; `app/consumer.go:268-291` |
| O conjunto de extensões do envelope é fechado, e `tenantid` é o tenant resolvido por autenticação | ENV-11 (`docs/dmpf/cloudevents-protobuf-buf.md:545`), ENV-12 (`:587`), ENV-10 (`:544`) |
| A trilha de auditoria é um `Sink` próprio, com `Event{Subject, Object, Action, Outcome, At, Tenant, DataTenant}` | `libs/backend/go/observability/audit/sink.go:11-30` |
| Recusa do `Sink` hoje: o registro segue ao canal de log e o fluxo prossegue, o que basta ao acesso cruzado por engano, já negado (IDN-12) | `observability/usecase/instrumentation.go:131-138` |
| O gate do choke point é a regra `depguard` `context-provider`, provada por script | `.golangci.yml:431-452`; `tools/dmpf-plugin/scripts/dmpf-gate-check.sh:76,425` |

### Não verificado

- Se o IDN-19 admite alcance total numa operação que exige sujeito. O IDN-16
  reserva à operação de plataforma a declaração de não exigir sujeito nem tenant
  (`:926`), e `Route.Validate` só admite o alcance com `RequireNeither`; o texto
  não proíbe nem prevê o caso do operador autenticado sem tenant.
- Como a identidade do operador chega à trilha quando a leitura roda atrás de gRPC:
  o canal não carrega sujeito (IDN-02), e o contexto montado em
  `libs/backend/go/grpc/interceptor_context.go:272-280` não o tem.
- Como o tenant de destino chega ao consumidor sem violar o CTX-24 a CTX-26 (ver
  a investigação da Onda 2).
- Se o IDN-11 (escrita escopada ao tenant do contexto, `:878`) admite a escrita em
  tenant diferente do contexto, declarada e com política, e se o IDN-14 (`:881`) é
  satisfeito por uma política que a app declara na composition root: o IDN-19 trata
  de alcance de dado e não prevê a escrita entre tenants.
- Se `Sink.Emit` sem erro é confirmação durável: a `NewJSONSink` sobre um `io.Writer`
  comum não o garante, e a durabilidade (retenção, acesso e integridade, DAT-25) é do
  `Sink` que o consumidor configura.

### Fontes normativas

- `docs/dmpf/contexto-erros-seguranca.md`: IDN-11 a IDN-20 (`:878-881,925-930`), CTX-13 (`:676`), CTX-24 a CTX-26 (`:760-762`) e DAT-11 (`:1492`).
- `docs/dmpf/cloudevents-protobuf-buf.md`: ENV-10 a ENV-12.
- `docs/adr/049-contexto-de-execucao-viaja-no-context-context.md`, `050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md`, `051-escopo-de-tenant-por-choke-point-em-go.md` e `052-identidade-de-workload-no-grpc-e-no-kafka.md`.

<constraints>
- [P0] A leitura comum não muda: `Reader`, `Relation`, `Repository` e a sonda `CrossTenantAccess` (IDN-12, IDN-13) mantêm o filtro por tenant e o comportamento de hoje, e as cláusulas atuais de `providerkit.Repository` seguem verdes nas duas realizações.
- [P0] Alcance de todos os tenants nunca é efeito colateral da ausência de tenant (IDN-19, IDN-20): só existe com declaração explícita, autorização e trilha; sem declaração, o acesso recusa.
- [P0] A leitura de plataforma é somente leitura; nenhuma escrita ocorre sem tenant de destino explícito, política de destino e trilha.
- [P0] Todo acesso entre tenants emite o registro de auditoria antes de devolver ou gravar o dado, e o acesso autorizado falha fechado quando o `Sink` não confirma o registro durável (DAT-25): sem confirmação, nada é devolvido nem gravado. O canal de log é adicional, nunca substituto; o caminho de log de `recordCrossTenant` segue valendo só para o acesso cruzado por engano (IDN-12), que já é negado.
- [P0] O provider de contexto continua sem driver (`context-provider`): nenhum tipo do `pgx` entra na superfície pública nova.
- [P0] Nenhum tenant sintético (`system`, `default`, `unknown`) e nenhuma substituição da ausência por valor (IDN-20, CTX-26); `tenantid` do envelope continua o tenant resolvido por autenticação (ENV-12), salvo emenda ao artefato normativo (ENV-11).
- [P1] O `ExecutionContext` mantém os nove campos de CTX-01: a declaração de alcance viaja em carrier próprio.
- [P1] Nenhuma dependência de terceiros nova no `go.mod` do kernel.
</constraints>

## Requisitos

### Funcionais

#### Onda 1 — Leitura de plataforma

- [ ] **[P0] Alcance no `ports`**: `ports.PlatformReach` (`ReachUndeclared`, `ReachAllTenants`) e o carrier `ports.WithPlatformReach` / `ports.PlatformReachFrom`, montado só pela borda para operação declarada. `http.PlatformReach` e as constantes viram alias dos tipos de `ports`, sem mudar a API pública de `http`.
- [ ] **[P0] Porta de leitura de plataforma**: `ports.PlatformReader[ID, S]` com `LoadIn(ctx, tenant, id)` e `List(ctx, after, limit)`, cujas linhas trazem o tenant de origem. Fica separada de `ports.Reader` (`ports/repository.go:39-41`), que não ganha método.
- [ ] **[P0] `postgres.Table.PlatformReader(pool ReadPool)`**: compila `SELECT` sem predicado de tenant, no kernel e declarado com `declare(...)` como os demais (`libs/backend/go/postgres/table.go:174-181`), e recusa com `ErrPlatformReachUndeclared` quando o carrier não traz `ReachAllTenants`. `List` pagina por `(tenant_id, id)` com limite obrigatório e teto `PlatformPageMax`; limite zero ou acima do teto é erro.
- [ ] **[P0] `memory.Table.PlatformReader`**: mesma porta e mesmas recusas no dublê (`memory/store.go:49-108`), para a suíte de conformidade rodar nas duas realizações.
- [ ] **[P0] Autorização de plataforma**: `application.PlatformPermitted(required)`, ao lado de `Permitted` (`application/authorize.go:33-54`). Com sujeito, exige a permissão declarada; sem sujeito, aplica o IDN-18 (workload verificado pelo canal) e só passa com a declaração de alcance. Recusa operação sem declaração.
- [ ] **[P0] Auditoria**: `usecase.ActionPlatformRead` (`platform.cross_tenant_read`), ao lado de `ActionCrossTenantAccess` (`libs/backend/go/observability/usecase/instrumentation.go:30-31`). Cada chamada emite um `audit.Event` por tenant alcançado, com `Subject` do portador (ou ausente), `Object` da tabela, `Tenant` ausente, `DataTenant` do tenant alcançado e `Outcome` `accepted`; a recusa por falta de declaração emite o mesmo evento com `Outcome` `denied`. O `PlatformReader` e o `Projector` recebem o `audit.Sink` na construção (sem ele a construção recusa) e emitem o evento antes de devolver ou gravar o dado: o acesso autorizado só prossegue se `Emit` confirmar o registro, e sem confirmação falha fechado com `ErrAuditUnavailable`, sem devolver linha nem gravar. O log recebe o mesmo evento como canal adicional, nunca substituto, porque o DAT-25 exige trilha com retenção, acesso e integridade próprios, independente do pipeline de log. O caminho de log de `recordCrossTenant` (`observability/usecase/instrumentation.go:131-138`) continua só para o acesso negado, em que a falha do `Sink` não muda o desfecho. `audit.Event` não muda de forma.
- [ ] **[P0] Gate estático**: um provider de contexto fora de `provider/platform_*.go` não consegue usar a leitura de plataforma nem a projeção, e `dmpf-gate-check.sh` prova a reprovação, como faz com `context-provider` (`.golangci.yml:431`). O `depguard` decide por package, não por símbolo (`.golangci.yml:26-28`), e `PlatformReader` e `Projector` seriam métodos de `Table`, no package que o provider importa para declarar tabela; o ADR novo fixa o mecanismo (package próprio sob `depguard`, ou `forbidigo` por símbolo com exclusão por caminho), e a regra entra no `.golangci.yml` e no `.golangci.yml__tmpl__` do `init`. A mesma regra restringe a construção de `ports.WithPlatformReach` à borda (`libs/backend/go/http`, `libs/backend/go/grpc`) e ao composition root (`apps/**/app`), porque o IDN-14 proíbe isolamento por convenção: "montado só pela borda" passa a ser imposto, não revisado, e a prova do `dmpf-gate-check.sh` cobre também um chamador fora desses packages.
- [ ] **[P1] Declaração na borda**: `Route.Validate` (`http/route.go:102-107`) admite `ReachAllTenants` com `RequireSubject` quando a investigação concluir que o IDN-19 cobre o operador autenticado; a borda gRPC ganha o equivalente para método declarado de plataforma.
- [ ] **[P1] Conformidade**: `providerkit` ganha cláusulas para a leitura de plataforma (vê todos os tenants, recusa sem declaração, recusa limite inválido, emite o evento) e reafirma que `Reader` e `Relation` seguem filtrados no mesmo teste.

#### Onda 2 — Projeção entre tenants

- [ ] **[P0] Investigar a origem do tenant de destino sem violar CTX-24 a CTX-26, IDN-11 e IDN-14**: confrontar as alternativas da tabela "Origem do tenant de destino", em Design (destino no payload com escrita declarada; atributo novo do envelope; `tenantid` igual ao destino, que o ENV-12 e o CTX-13 já excluem), a regra de proveniência do tenant de origem (que não autoriza, como no CTX-25) e, para o atributo novo, o que o ENV-10 e o ENV-11 exigem. Confrontar também o IDN-11 (escrita escopada ao tenant do contexto, `docs/dmpf/contexto-erros-seguranca.md:878`) e o IDN-14 (`:881`): a projeção grava fora do tenant do contexto, e a investigação decide se a escrita declarada, com destino explícito, política e trilha, cabe nessas duas regras ou pede emenda normativa condicional (spec própria, como na investigação transversal). O resultado fixa o ADR novo.
- [ ] **[P0] ADR novo**: registra a origem do tenant de destino, a política de autorização do destino, a proveniência do tenant de origem, o alcance de plataforma da Onda 1, o resultado da investigação sobre IDN-11 e IDN-14 (com a dependência da emenda normativa, se houver) e as emendas ao ADR-050 (item sobre a escrita do consumidor) e ao ADR-051 (itens 1 e 4). `node tools/adr-verify.mjs --from 63` passa.
- [ ] **[P0] `postgres.Table.Projector(pool)`**: escrita com tenant de destino explícito (`Load(ctx, destination, id)` e `Save(ctx, destination, id, state, expected)`), com a mesma semântica de `version` do `Repository` (`libs/backend/go/postgres/table.go:185-192`). Exige `ports.ProjectionPolicy` e destino não vazio; grava `tenant_id` com o destino e nunca com o do portador.
- [ ] **[P0] Política de destino**: `ports.ProjectionPolicy func(ctx, origin *TenantID, destination TenantID) error`, declarada na composition root; ausente ou nula, o `Projector` recusa com `ErrProjectionDenied` (default fechado, IDN-17). A decisão de quem é o financiador de um tenant é da app, nunca do kernel.
- [ ] **[P0] Auditoria da projeção**: `usecase.ActionProjectionWrite` (`projection.cross_tenant_write`), com `Tenant` do portador (ou ausente), `DataTenant` do destino e `Outcome` `accepted` ou `denied`, emitido antes da gravação. A gravação só ocorre com o `Sink` confirmando o registro, e sem confirmação a projeção falha fechada com `ErrAuditUnavailable`, sob a mesma regra da leitura de plataforma.
- [ ] **[P0] Gate estático da projeção**: a mesma regra cobre o `Projector`, com a mesma prova em `dmpf-gate-check.sh`.
- [ ] **[P1] Envelope, se o ADR novo escolher o atributo novo**: emenda de `docs/dmpf/cloudevents-protobuf-buf.md` (ENV-08, ENV-11, com categoria, presença e finalidade), campo em `contracts/envelope/envelope.go` (`:34,57,97,141,202`), em `PublishIntent` e `OutboxEntry`, no relay (`libs/backend/go/app/relay/record.go:49-54`) e no consumidor (`libs/backend/go/app/consumer.go:268-291`), mais testes de conformidade. Se o ADR escolher o destino no payload, este requisito não se aplica e o envelope não muda.
- [ ] **[P1] Paridade e conformidade**: `memory.Table.Projector` e cláusulas de `providerkit` para o destino explícito, a política ausente, a política que nega e o destino vazio.

#### Investigação transversal

- [ ] **[P0] Investigar a atribuição do operador na leitura de plataforma**: (a) se o IDN-19 admite alcance total com sujeito autenticado, dado que `Route.Validate` só o aceita com `RequireNeither`; (b) como a identidade do operador chega à trilha quando a leitura roda atrás de gRPC, onde o canal não carrega sujeito (IDN-02; `libs/backend/go/grpc/interceptor_context.go:272-280`). Se o texto normativo exigir emenda, ela segue em spec própria e esta spec registra a dependência no ADR novo.

### Não-funcionais

- [ ] Custo: os statements de plataforma e de projeção só são compilados quando a app os usa; o caminho comum de `Table` não ganha statement nem consulta.
- [ ] Limite: a leitura de plataforma nunca devolve página sem limite; o teto é configurável pela composition root.
- [ ] Compatibilidade: Go `1.26.9`; os aliases mantêm todo identificador público de `http` compilando.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Kernel `ports` | `PlatformReach`, carrier, `PlatformReader`, `ProjectionPolicy` |
| Kernel `application` | `PlatformPermitted` |
| Kernel `postgres` e `memory` | `PlatformReader`, `Projector` |
| Kernel `http` e `grpc` | alias de `PlatformReach`; declaração na borda |
| Kernel `observability` | duas ações de auditoria |
| Kernel `testkit` (`providerkit`) | cláusulas de isolamento |
| Tooling | regra do gate de plataforma, `dmpf-gate-check.sh`, template do `init` |
| Normas e docs | ADR novo; `cloudevents-protobuf-buf.md` só se o ADR escolher o atributo novo |

## Localização de código

```text
libs/backend/go/ports/platform.go                        novo: PlatformReach, carrier, PlatformReader, ProjectionPolicy
libs/backend/go/http/route.go                            alias de PlatformReach; Validate (:102-107)
libs/backend/go/application/authorize.go                 PlatformPermitted
libs/backend/go/postgres/platform.go                     novo: PlatformReader, Projector
libs/backend/go/memory/store.go                          equivalentes
libs/backend/go/observability/usecase/instrumentation.go ActionPlatformRead, ActionProjectionWrite
libs/backend/go/testkit/providerkit/repository.go        cláusulas
.golangci.yml, tools/dmpf-plugin/templates/init/managed/.golangci.yml__tmpl__   regra do gate de plataforma
tools/dmpf-plugin/scripts/dmpf-gate-check.sh             prova da regra
docs/adr/0NN-acesso-entre-tenants-declarado.md           novo (número a resolver)
```

Só se o ADR novo escolher o atributo novo do envelope:

```text
docs/dmpf/cloudevents-protobuf-buf.md
libs/backend/go/contracts/envelope/envelope.go
libs/backend/go/ports/outbox.go, postgres/outbox.go, app/relay/record.go, app/consumer.go
```

## Design

### Dois caminhos, uma declaração

```text
Leitura de plataforma
  borda declara ReachAllTenants ──► carrier (ports.WithPlatformReach)
      │                                     │
      ▼                                     ▼
  PlatformPermitted ──► Table.PlatformReader.List/LoadIn ──► audit.Event por tenant alcançado
  (permissão ou IDN-18)   SELECT sem predicado de tenant         (DataTenant = tenant lido)

Projeção
  consumidor (contexto no tenant A, vindo do envelope)
      │ destino B (origem definida pelo ADR novo)
      ▼
  ProjectionPolicy(A, B) ──► Table.Projector.Save(B, ...) ──► audit.Event (Tenant = A, DataTenant = B)
  nula ou negada: ErrProjectionDenied, nada é gravado
```

### Origem do tenant de destino

| Alternativa | Envelope | Norma |
| ----------- | -------- | ----- |
| 1. Destino no payload de negócio, com escrita declarada | inalterado | ENV-11, ENV-12 e CTX-24 a CTX-26 intactos; IDN-11 e IDN-14 em aberto (nota abaixo); o ADR-050:23 é emendado |
| 2. Atributo novo do envelope | um atributo a mais (ENV-10: ASCII minúsculo e dígitos, até 20 caracteres) | exige emendar o artefato normativo (ENV-11) e dizer como o atributo convive com CTX-13 e CTX-24; IDN-11 e IDN-14 em aberto (nota abaixo) |
| 3. `tenantid` do envelope igual ao destino | valor muda | contraria ENV-12 (é o tenant resolvido por autenticação) e CTX-13; excluída |

O `Projector` serve às alternativas 1 e 2: muda só de onde o consumidor tira o
destino que passa à API.

Nas três alternativas a gravação cai fora do tenant do contexto, e o IDN-11 (escrita
escopada ao tenant do contexto, `:878`) e o IDN-14 (`:881`) não ficam intactos por si:
o `Projector` os atende com destino explícito, política que fecha por omissão, trilha e
gate estático, e a investigação da Onda 2 decide se a norma admite isso ou pede emenda
condicional, em spec própria.

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Onde vive a declaração de alcance | Carrier próprio em `ports`; `http` mantém alias | Décimo campo do `ExecutionContext` (CTX-01 fixa nove); inferir da ausência de tenant (IDN-19 proíbe); `postgres` importar `http` |
| Forma da leitura de plataforma | Tipo próprio `PlatformReader` sobre `ReadPool`, com statements compilados no kernel | Liberar o `pgx` ao contexto (ADR-051:47); parâmetro `unscoped` em `Table.Reader`, que afrouxaria o choke point com um booleano |
| Paginação | Keyset por `(tenant_id, id)`, limite obrigatório | `Relation.Query` sem limite (`libs/backend/go/postgres/table.go:101-134`), inaceitável entre tenants |
| Auditoria | Um evento por tenant alcançado, `audit.Event` inalterado | Campo novo no `Event`, que mudaria o contrato do LOG-14 |
| Falha do `Sink` | O acesso autorizado falha fechado sem a confirmação do registro durável; o log é canal adicional | Seguir ao log como `recordCrossTenant` faz: serve ao acesso já negado (IDN-12), mas não atende o DAT-25, porque o log não tem retenção, acesso e integridade próprios |
| Mecanismo do gate | Fixado no ADR novo entre package próprio sob `depguard` e `forbidigo` por símbolo; cobre `PlatformReader`, `Projector` e `ports.WithPlatformReach` | Regra só de revisão: o `context-provider` existe justamente para o isolamento não depender de convenção (IDN-14) |
| Pool da leitura de plataforma | A pool de runtime: o kernel só emite `SELECT` nesse caminho | Role de leitura própria: exige uma terceira role, fora do que a SPEC-QPRR24C4 provisiona |
| Destino da projeção | API com destino explícito e política que fecha por omissão | Destino implícito no `context.Context`, que apagaria o "explícito" |

## Riscos

| Risco | Mitigação |
| ----- | --------- |
| A leitura de plataforma vira via de extração em massa | Declaração, permissão, limite de página, evento por tenant e gate estático; a permissão é declarada por operação |
| A durabilidade da trilha é a do `Sink` configurado, e o DAT-25 pede retenção, acesso e integridade próprios | O kernel exige a confirmação do `Sink` antes do acesso e falha fechado sem ela; o `Sink` durável é do consumidor, e um `Sink` que confirma sem persistir derrota a garantia |
| RLS futura esconderia as linhas de outros tenants também da leitura de plataforma e da sonda de dono | A spec de RLS trata os três caminhos; os testes desta spec rodam sob a role sem posse da SPEC-QPRR24C4 |
| A política de destino é código da app e pode ser frouxa | Default fechado, evento por gravação e teste de conformidade da política ausente |

## Regras relacionadas

- IDN-11 a IDN-20, CTX-13, CTX-24 a CTX-26 e DAT-11; ENV-10 a ENV-12.
- ADR-049, ADR-050, ADR-051 e ADR-052.
- SPEC-9B6SHEH8 (contexto de execução e tenant), SPEC-QPRR24C4 (roles de banco), SPEC-XQWGGAXF (contexto de execução, erros, segurança e multi-tenancy) e SPEC-WYX5GW87 (contratos wire).

## Verificação e testes

### Critérios de aceite

- [ ] A suíte `providerkit` roda nas duas realizações (`memory` e `postgres`) e prova: a leitura de plataforma vê linhas de tenants distintos; sem declaração no carrier recusa; limite zero ou acima do teto é erro; `Reader` e `Relation` continuam sem enxergar outro tenant.
- [ ] A leitura de plataforma e a projeção rodam em teste de integração contra o Postgres sob a role de runtime sem posse (`OpenRuntimePool` da SPEC-QPRR24C4).
- [ ] Cada chamada de plataforma e cada gravação de projeção produz o número esperado de `audit.Event` (um por tenant alcançado) com `DataTenant` preenchido, inclusive nas recusas. Com um `Sink` que recusa o registro, a leitura e a projeção autorizadas falham fechadas com `ErrAuditUnavailable`, sem devolver linha nem gravar, e o log recebe o evento como canal adicional.
- [ ] O gate de plataforma reprova, por `dmpf-gate-check.sh`, um provider fora de `provider/platform_*.go` que use a leitura de plataforma ou a projeção, e um package fora da borda e do composition root que construa `ports.WithPlatformReach`.
- [ ] As duas investigações têm resultado registrado no ADR novo, e o requisito condicional do envelope está aplicado ou declarado não aplicável.
- [ ] `go run ./tools/dmpf-conformance/cmd/conformance --root .` e `modsync --check` passam, o job `external-consumer` (`.github/workflows/ci.yml:96`) passa, e `node tools/adr-verify.mjs --from 63` passa.
- [ ] `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source` passa.

### Cenários de teste

**Happy path — leitura de plataforma declarada**
- DADO linhas dos tenants A e B na mesma tabela e uma operação que declara `ReachAllTenants`, cujo sujeito tem a permissão
- QUANDO a app chama `PlatformReader.List` com limite 50
- ENTÃO devolve as linhas de A e de B com o tenant de cada uma, e a trilha recebe um evento com `DataTenant` A e outro com `DataTenant` B

**Erro — leitura sem declaração**
- DADO uma operação sem `ReachAllTenants` no carrier, mesmo com o contexto sem tenant
- QUANDO a app chama `PlatformReader.List`
- ENTÃO recusa com `ErrPlatformReachUndeclared`, não devolve linha e registra o evento com `Outcome` `denied`

**Edge case — leitura comum continua filtrada**
- DADO o mesmo estado e um contexto do tenant A cujo carrier também traz `ReachAllTenants`
- QUANDO a app lê o identificador de B por `Reader.Load`
- ENTÃO responde `ErrNotFound` com `CrossTenantAccess` e evento IDN-12, como antes

**Happy path — projeção no tenant do financiador**
- DADO um consumidor no tenant A, destino B vindo da origem escolhida no ADR novo e uma política que autoriza A para B
- QUANDO `Projector.Save` grava o estado
- ENTÃO a linha nasce com `tenant_id` B, a trilha registra `Tenant` A e `DataTenant` B, e `Reader` de A não a enxerga

**Erro — projeção sem política**
- DADO um `Projector` montado sem `ProjectionPolicy`
- QUANDO a app chama `Save` com destino B
- ENTÃO recusa com `ErrProjectionDenied`, nada é gravado e o evento sai com `Outcome` `denied`

**Erro — `Sink` não confirma o registro**
- DADO uma leitura de plataforma declarada e autorizada, uma projeção com política que autoriza e um `Sink` que recusa `Emit`
- QUANDO a app chama `PlatformReader.List` e, no mesmo estado, `Projector.Save`
- ENTÃO ambas falham com `ErrAuditUnavailable`, nenhuma linha é devolvida nem gravada, e o log recebe o evento como canal adicional

**Edge case — cadeia de plataforma sem tenant**
- DADO uma mensagem sem `tenantid` e uma política que decide sobre origem ausente
- QUANDO o consumidor projeta para B
- ENTÃO a origem chega como ausente (CTX-26), nenhum valor de preenchimento é inventado e a trilha registra `Tenant` vazio

<critical_constraints>
- [P0] A leitura comum não muda: `Reader`, `Relation`, `Repository` e a sonda `CrossTenantAccess` (IDN-12, IDN-13) mantêm o filtro por tenant e o comportamento de hoje, e as cláusulas atuais de `providerkit.Repository` seguem verdes nas duas realizações.
- [P0] Alcance de todos os tenants nunca é efeito colateral da ausência de tenant (IDN-19, IDN-20): só existe com declaração explícita, autorização e trilha; sem declaração, o acesso recusa.
- [P0] A leitura de plataforma é somente leitura; nenhuma escrita ocorre sem tenant de destino explícito, política de destino e trilha.
- [P0] Todo acesso entre tenants emite o registro de auditoria antes de devolver ou gravar o dado, e o acesso autorizado falha fechado quando o `Sink` não confirma o registro durável (DAT-25): sem confirmação, nada é devolvido nem gravado. O canal de log é adicional, nunca substituto; o caminho de log de `recordCrossTenant` segue valendo só para o acesso cruzado por engano (IDN-12), que já é negado.
- [P0] O provider de contexto continua sem driver (`context-provider`): nenhum tipo do `pgx` entra na superfície pública nova.
- [P0] Nenhum tenant sintético (`system`, `default`, `unknown`) e nenhuma substituição da ausência por valor (IDN-20, CTX-26); `tenantid` do envelope continua o tenant resolvido por autenticação (ENV-12), salvo emenda ao artefato normativo (ENV-11).
- [P1] O `ExecutionContext` mantém os nove campos de CTX-01: a declaração de alcance viaja em carrier próprio.
- [P1] Nenhuma dependência de terceiros nova no `go.mod` do kernel.
</critical_constraints>

## Escopo fora

- **Escrita de plataforma com alcance total**: só a leitura é irrestrita; toda gravação entre tenants passa pelo `Projector`, com destino explícito e política.
- **Alcance parcial** (lista de tenants): o IDN-19 pede alcance declarado, e esta spec declara só "todos os tenants", o único valor que `PlatformReach` tem hoje.
- **RLS e role de leitura própria**: pertencem à evolução da SPEC-QPRR24C4; a RLS precisa tratar a leitura de plataforma e a sonda de dono (`libs/backend/go/postgres/table.go:179-181`) em spec própria.
- **Retenção e imutabilidade da trilha**: o kernel exige a confirmação do `Sink` antes do acesso e falha fechado sem ela (DAT-25); retenção, acesso e integridade da trilha são do consumidor.
- **Regra de negócio de quem é o financiador de um tenant**: é a política da app, não do kernel.
- **Agregação, junção e exportação entre tenants**: o ADR-051:47 manda estender o kernel caso a caso; esta spec cobre carga de linha por chave e página, não relatório.
- **Emendar o texto normativo de IDN-11, IDN-14 e IDN-16 a IDN-19**: se uma das investigações concluir que é necessário, segue em spec própria.
