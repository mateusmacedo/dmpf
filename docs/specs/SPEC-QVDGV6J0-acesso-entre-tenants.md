---
id: SPEC-QVDGV6J0
slug: acesso-entre-tenants
title: DMPF — Acesso entre tenants declarado: leitura de plataforma sobre postgres.Table e projeção entre tenants
stage: done
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
  escopada como qualquer outra), com o CTX-13 e o CTX-24 a CTX-26. O [ADR-068](../adr/068-acesso-entre-tenants-declarado.md)
  resolve a colisão com o destino no payload e emenda o ADR-050 e, in-place, o
  IDN-11 e o IDN-12.
- A dependência da SPEC-QPRR24C4 é de verificação: os testes de integração de
  plataforma e de projeção rodam sob a role de runtime sem posse
  (`OpenRuntimePool`), a role com que o caminho executa em produção; sem ela, o
  teste passaria com poder de dono.

### Estado atual

Verificado no código e nas normas de `lidercap-platform` em 2026-10-12, com as
linhas reconferidas em 2026-10-18 na base `ae3d7c5a`, depois da DEVS-103. Os
caminhos `.go` da coluna "Evidência" são relativos a `libs/backend/go/`.

| Ponto | Evidência |
| ----- | --------- |
| `PlatformReach` (`ReachUndeclared`, `ReachAllTenants`) só existe em `http.Route`, e `Validate` o aceita apenas com `RequireNeither` | `libs/backend/go/http/route.go:54-62,81,102-107`; `libs/backend/go/http/README.md:119-124` |
| Nenhuma outra peça consome a declaração: persistência não a lê | `rg PlatformReach` só alcança `http/route.go`, testes e README |
| A autorização de aplicação nega a operação sem tenant | `libs/backend/go/application/authorize.go:39-41` |
| `Table` resolve o tenant do portador e recusa na ausência | `libs/backend/go/postgres/table.go:16-19,244-256` |
| Leitura cruzada vira `CrossTenantAccess`, que responde `ErrNotFound` e vira evento IDN-12 | `postgres/table.go:141-157,284-300`; `ports/repository.go:16-27`; `observability/usecase/instrumentation.go:30-31,117-140` |
| `ExecutionContext` tem nove campos (CTX-01), com sujeito e tenant opcionais; `Identity` também | `ports/execcontext.go:57-74`; `ports/authn.go:22-29` |
| A borda gRPC monta o contexto com tenant do metadado e sem sujeito | `grpc/interceptor_context.go:35-37,272-280,339-345`; IDN-02 em `contexto-erros-seguranca.md:810` |
| O tenant da mensagem é o do portador; `OutboxEntry` não tem campo de tenant | `postgres/outbox.go:69,125-138`; `ports/outbox.go:26-35` |
| O relay copia o tenant ao envelope e o consumidor reconstrói o contexto a partir dele | `app/relay/record.go:49-54`; `app/consumer.go:268-291` |
| O conjunto de extensões do envelope é fechado, e `tenantid` é o tenant resolvido por autenticação | ENV-11 (`docs/dmpf/cloudevents-protobuf-buf.md:545`), ENV-12 (`:587`), ENV-10 (`:544`) |
| A trilha de auditoria é um `Sink` próprio, com `Event{Subject, Object, Action, Outcome, At, Tenant, DataTenant}` | `libs/backend/go/observability/audit/sink.go:12-32` |
| Recusa do `Sink` hoje: o registro segue ao canal de log e o fluxo prossegue, o que basta ao acesso cruzado por engano, já negado (IDN-12) | `observability/usecase/instrumentation.go:131-138` |
| O gate do choke point é a regra `depguard` `context-provider`, provada por script | `.golangci.yml:431-454`; `tools/dmpf-plugin/scripts/dmpf-gate-check.sh:76,425` |

### Não verificado

As cinco perguntas abertas na escrita da spec foram respondidas pelo [ADR-068](../adr/068-acesso-entre-tenants-declarado.md):

- Se o IDN-19 admite alcance total numa operação que exige sujeito. O IDN-16
  reserva à operação de plataforma a declaração de não exigir sujeito nem tenant
  (`:926`), e `Route.Validate` só admite o alcance com `RequireNeither`. **Resposta
  (I1):** admite, sem emenda; `ReachAllTenants` vale com `RequireSubject` e com
  `RequireNeither`, nunca com declaração que exija tenant, e a restrição era só do
  código.
- Como a identidade do operador chega à trilha quando a leitura roda atrás de gRPC:
  o canal não carrega sujeito (IDN-02), e o contexto montado em
  `libs/backend/go/grpc/interceptor_context.go:272-280` não o tem. **Resposta (I2):**
  não chega; o evento do contexto sai sem `Subject`, e a trilha se liga à da borda,
  onde o operador foi autorizado, pelo `correlation_id`.
- Como o tenant de destino chega ao consumidor sem violar o CTX-24 a CTX-26.
  **Resposta (I3):** no payload ou em dado da própria app, passado explicitamente ao
  `Projector`; o envelope não muda.
- Se o IDN-11 (escrita escopada ao tenant do contexto, `:878`) admite a escrita em
  tenant diferente do contexto, declarada e com política, e se o IDN-14 (`:881`) é
  satisfeito por uma política que a app declara na composition root. **Resposta
  (I4):** o IDN-14 é atendido pelo choke point e pelo gate; o IDN-11 e o IDN-12 são
  emendados in-place pelo ADR-068, sem ID de regra novo.
- Se `Sink.Emit` sem erro é confirmação durável: a `NewJSONSink` sobre um `io.Writer`
  comum não o garante, e a durabilidade (retenção, acesso e integridade, DAT-25) é do
  `Sink` que o consumidor configura. **Resposta (I5):** o contrato do `Sink` nesses
  caminhos é devolver `nil` só depois de persistir o registro; a falha fechada é
  decisão do kernel, mais estrita que o DAT-25, que é condicional.

### Fontes normativas

- `docs/dmpf/contexto-erros-seguranca.md`: IDN-11 a IDN-20 (`:878-881,925-930`), CTX-13 (`:676`), CTX-24 a CTX-26 (`:760-762`), DAT-11 (`:1492`) e DAT-25 (`:1560`); o IDN-11 e o IDN-12 foram emendados in-place pelo ADR-068.
- `docs/dmpf/cloudevents-protobuf-buf.md`: ENV-10 a ENV-12.
- `docs/adr/049-contexto-de-execucao-viaja-no-context-context.md`, `050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md`, `051-escopo-de-tenant-por-choke-point-em-go.md`, `052-identidade-de-workload-no-grpc-e-no-kafka.md`, `066-unicidade-declarada-como-rejeicao.md` (precedente da emenda in-place) e `068-acesso-entre-tenants-declarado.md`.

<constraints>
- [P0] A leitura comum não muda: `Reader`, `Relation`, `Repository` e a sonda `CrossTenantAccess` (IDN-12, IDN-13) mantêm o filtro por tenant e o comportamento de hoje, e as cláusulas atuais de `providerkit.Repository` seguem verdes nas duas realizações.
- [P0] Alcance de todos os tenants nunca é efeito colateral da ausência de tenant (IDN-19, IDN-20): só existe com declaração explícita, autorização e trilha; sem declaração, o acesso recusa.
- [P0] A leitura de plataforma é somente leitura; nenhuma escrita ocorre sem tenant de destino explícito, política de destino e trilha.
- [P0] Todo acesso entre tenants emite o registro de auditoria antes de devolver ou gravar o dado, e o acesso autorizado falha fechado quando o `Sink` não confirma o registro durável: sem confirmação, nada é devolvido nem gravado. A falha fechada é decisão do kernel, com fonte no IDN-19, mais estrita que o DAT-25, que é condicional ao requisito regulatório e rege a separação da trilha. O canal de log é adicional, nunca substituto; o caminho de log de `recordCrossTenant` segue valendo só para o acesso cruzado por engano (IDN-12), que já é negado.
- [P0] O provider de contexto continua sem driver (`context-provider`): nenhum tipo do `pgx` entra na superfície pública nova.
- [P0] Nenhum tenant sintético (`system`, `default`, `unknown`) e nenhuma substituição da ausência por valor (IDN-20, CTX-26); `tenantid` do envelope continua o tenant resolvido por autenticação (ENV-12), salvo emenda ao artefato normativo (ENV-11).
- [P1] O `ExecutionContext` mantém os nove campos de CTX-01: a declaração de alcance viaja em carrier próprio.
- [P1] Nenhuma dependência de terceiros nova no `go.mod` do kernel.
</constraints>

## Requisitos

### Funcionais

#### Onda 1 — Leitura de plataforma

- [ ] **[P0] Alcance no `ports`**: `ports.PlatformReach` (`ReachUndeclared`, `ReachAllTenants`) e o carrier `ports.WithPlatformReach` / `ports.PlatformReachFrom`, montado só pela borda que verificou quem chama, para operação declarada (ADR-068, item 3). O `ports` concentra também `PlatformKey`, `PlatformRow`, `PlatformReader`, `Projector`, `ProjectionPolicy`, `PlatformPageMax` e os erros `ErrPlatformReachUndeclared`, `ErrPlatformLimitInvalid`, `ErrAuditUnavailable`, `ErrProjectionDenied` e `ErrProjectionDestinationAbsent`, todos categorizados por `application.CategoryOf`. `http.PlatformReach` e as constantes viram alias dos tipos de `ports`, sem mudar a API pública de `http`.
- [ ] **[P0] Porta de leitura de plataforma**: `ports.PlatformReader[ID, S]` com `LoadIn(ctx, tenant, id)` e `List(ctx, after *PlatformKey[ID], limit)`, cujas linhas (`PlatformRow`) trazem o tenant de origem; `after == nil` é a primeira página, e o cursor seguinte é a chave da última linha. Fica separada de `ports.Reader` (`ports/repository.go:58-60`), que não ganha método.
- [ ] **[P0] `postgres.Table.PlatformReader(pool ReadPool, ...)`**: recebe a trilha na construção (requisito "Auditoria"); compila `SELECT` sem predicado de tenant, no kernel e declarado com `declare(...)` como os demais (`compile`, `libs/backend/go/postgres/table.go:167-202`), e recusa com `ErrPlatformReachUndeclared` quando o carrier não traz `ReachAllTenants`. `List` pagina por `(tenant_id, id)` com limite obrigatório e teto `ports.PlatformPageMax` (500), trocável pela composition root por valor positivo; limite zero, negativo ou acima do teto devolve `ErrPlatformLimitInvalid`.
- [ ] **[P0] `memory.Table.PlatformReader`**: mesma porta e mesmas recusas no dublê (`memory/store.go:16-120`), para a suíte de conformidade rodar nas duas realizações.
- [ ] **[P0] Autorização de plataforma**: `application.PlatformPermitted(required)`, ao lado de `Permitted` (`application/authorize.go:33-54`). Com sujeito, exige a permissão declarada; sem sujeito, aplica o IDN-18 (workload verificado pelo canal) e só passa com a declaração de alcance, que só a borda verificada monta. Recusa operação sem declaração.
- [ ] **[P0] Auditoria**: `ports.AuditPlatformRead` (`platform.cross_tenant_read`) em `ports`, com o mesmo valor em `audit.ActionPlatformRead` e reexportada como `usecase.ActionPlatformRead` ao lado de `ActionCrossTenantAccess` (`libs/backend/go/observability/usecase/instrumentation.go:30-31`); a ação mora em `ports` porque o provider não importa o `usecase`, e o `memory` não importa `observability`. Cada chamada emite um `ports.AuditRecord` por tenant alcançado, com `Subject` do portador (ausente atrás do gRPC, onde o canal não carrega sujeito), `Object` da tabela, `Tenant` do contexto (ausente quando o contexto não tem tenant), `DataTenant` do tenant alcançado e `Outcome` `accepted`; página vazia não alcança tenant e não emite evento; a recusa por falta de declaração emite o mesmo evento com `Outcome` `denied`, e a falha do `Sink` nessa recusa não muda o desfecho. O `Tenant` do contexto é desvio explícito do texto original desta spec, que pedia `Tenant` ausente: o `audit.Event.Tenant` é o tenant do contexto, e gravá-lo vazio falsearia a trilha (ADR-068, item 11). O `PlatformReader` e o `Projector` recebem na construção a trilha como `ports.AuditTrail` (`Record(ctx, ports.AuditRecord) error`), realizada pelo `audit.Recorder` sobre o `audit.Sink`, o logger e o relógio, por `Recorder.Trail()`; sem a trilha, a construção entra em pânico. A porta fica em `ports` para manter o `memory` sem dependência de `observability`, cujos testes importam o `memory` e fechariam um ciclo no grafo do Nx (ADR-068, item 10). Emitem o evento antes de devolver ou gravar o dado: o acesso autorizado só prossegue se `Emit` confirmar o registro, e sem confirmação falha fechado com `ErrAuditUnavailable`, sem devolver linha nem gravar. O log recebe todo evento como canal adicional, nunca substituto, porque a trilha precisa de retenção, acesso e integridade próprios, independentes do pipeline de log (DAT-25, onde houver requisito regulatório). O provider emite, e não o `application service` do LOG-14, porque só ele conhece os tenants alcançados e o instante anterior à devolução ou à gravação. O caminho de log de `recordCrossTenant` (`observability/usecase/instrumentation.go:131-138`) continua só para o acesso negado, em que a falha do `Sink` não muda o desfecho. `audit.Event` não muda de forma.
- [ ] **[P0] Gate estático**: um provider de contexto fora de `provider/platform_*.go` não consegue usar a leitura de plataforma nem a projeção, e `dmpf-gate-check.sh` prova a reprovação, como faz com `context-provider` (`.golangci.yml:431`). O `depguard` decide por package, não por símbolo (`.golangci.yml:26-30`), e `PlatformReader` e `Projector` são métodos de `Table`, no package que o provider importa para declarar tabela; o ADR-068 fixa o `forbidigo` por símbolo, com `analyze-types` e exclusões por caminho e por `text` (o kernel, os kits de teste e os `_test.go` ficam isentos), e a regra entra no `.golangci.yml` e no `.golangci.yml__tmpl__` do `init`. O `forbidigo` não vê a chamada por interface local que o `Table` satisfaz, e a checagem estrutural do `dmpf-context-check.sh` fecha esse contorno, varrendo recursivamente todo `.go` de produção do contexto e reprovando a chamada ou a declaração dos símbolos fora de `provider/platform_*.go`, dos `_test.go` e dos kits, sem reprovar o uso dos tipos `ports.PlatformReader[...]` e `ports.Projector[...]`. A regra emenda o item 3 do ADR-051. A mesma regra restringe a construção de `ports.WithPlatformReach` à borda do kernel (`libs/backend/go/http`, `libs/backend/go/grpc`), sem exceção para `app/` nem `app/rpc` do contexto, porque o IDN-14 proíbe isolamento por convenção: "montado só pela borda" passa a ser imposto, não revisado, e a prova do `dmpf-gate-check.sh` cobre também um chamador em `app/` e em `app/rpc`. `http.WithVerifiedPlatformReach` decide só pelos campos públicos da `Route`, que qualquer código fabrica, e por isso também é gateado, com mensagem e exclusão próprias: o `forbidigo` só o admite na borda REST (`apps/**/app/api`), no kernel de borda, nos kits e nos `_test.go`, sem liberar em `app/api` o `ports.WithPlatformReach` direto; o `dmpf-context-check.sh` o reprova em qualquer arquivo de produção do contexto, que é só gRPC (ADR-044), e o `dmpf-gate-check.sh` prova a reprovação em `app/rpc` e no provider.
- [ ] **[P1] Declaração na borda**: `Route.Validate` (`http/route.go:102-107`) admite `ReachAllTenants` com `RequireSubject`, porque o IDN-19 cobre o operador autenticado (ADR-068, item 2), e segue exigindo o alcance com `RequireNeither`; a borda gRPC ganha o equivalente para método declarado de plataforma. Só a borda do kernel que verificou quem chama monta o carrier: no HTTP, `http.WithVerifiedPlatformReach`, que o middleware do `bff` chama, monta-o só na rota com `ReachAllTenants`, `RequireSubject` e permissão; a rota `RequireNeither` com `ReachAllTenants` segue válida na `Route`, por compatibilidade, mas não monta o carrier, e o leitor recusa. No gRPC, o método declarado de plataforma recebe o carrier só quando o `TrustedPeers` admitiu o workload pela allowlist e deixou no `ctx` o marcador privado do pacote; uma cadeia TLS verificada sem esse marcador responde `PermissionDenied`. Servidor `Insecure` ou sem `TrustedClients` recusa declarar método de plataforma.
- [ ] **[P1] Conformidade**: `providerkit` ganha cláusulas para a leitura de plataforma (vê todos os tenants, recusa sem declaração, recusa limite inválido, emite o evento, falha fechada com `Sink` que recusa, construção sem trilha) e reafirma que `Reader` segue filtrado nas duas realizações. `Relation` segue filtrada, provada só no Postgres (`postgres/table_test.go`), porque o `memory` não tem `Relation`.

#### Onda 2 — Projeção entre tenants

- [ ] **[P0] Investigar a origem do tenant de destino sem violar CTX-24 a CTX-26, IDN-11 e IDN-14** (respondida pelo ADR-068: destino no payload; IDN-14 atendido; IDN-11 e IDN-12 emendados in-place): confrontar as alternativas da tabela "Origem do tenant de destino", em Design (destino no payload com escrita declarada; atributo novo do envelope; `tenantid` igual ao destino, que o ENV-12 e o CTX-13 já excluem), a regra de proveniência do tenant de origem (que não autoriza, como no CTX-25) e, para o atributo novo, o que o ENV-10 e o ENV-11 exigem. Confrontar também o IDN-11 (escrita escopada ao tenant do contexto, `docs/dmpf/contexto-erros-seguranca.md:878`) e o IDN-14 (`:881`): a projeção grava fora do tenant do contexto, e a investigação decide se a escrita declarada, com destino explícito, política e trilha, cabe nessas duas regras ou pede emenda normativa. O resultado fixa o ADR novo; a emenda, quando necessária, é in-place no próprio ADR, como no precedente do ADR-066.
- [ ] **[P0] ADR novo**: o [ADR-068](../adr/068-acesso-entre-tenants-declarado.md) registra a origem do tenant de destino, a política de autorização do destino, a proveniência do tenant de origem, o alcance de plataforma da Onda 1, quem monta o carrier, o `Tenant` do evento, o resultado da investigação sobre IDN-11 e IDN-14, a emenda in-place do IDN-11 e do IDN-12 em `docs/dmpf/contexto-erros-seguranca.md`, e as emendas ao ADR-050 (item sobre a escrita do consumidor) e ao ADR-051 (itens 1, 3 e 4). `node tools/adr-verify.mjs --from 63` passa com o ADR-067 da DEVS-132 presente, dependência de integração registrada no ADR-068.
- [ ] **[P0] `postgres.Table.Projector(tx *Tx, ...)`**: realiza `ports.Projector[ID, S]` e nasce do `*Tx`, não da pool, para gravar na mesma transação da inbox e da outbox sob o `UoW.Within`; recebe a política e a trilha na construção. Lê e grava com tenant de destino explícito (`Load(ctx, destination, id)` e `Save(ctx, destination, id, state, expected)`), com a mesma semântica de `version` do `Repository` (`Save`, `libs/backend/go/postgres/table.go:329-358`). O `Load` aplica a política e a auditoria e não roda a sonda de dono: a falta da linha no destino responde `ErrNotFound` sem `CrossTenantAccess`. Exige `ports.ProjectionPolicy` e destino não vazio (`ErrProjectionDestinationAbsent`); grava `tenant_id` com o destino e nunca com o do portador.
- [ ] **[P0] Política de destino**: `ports.ProjectionPolicy func(ctx, origin *TenantID, destination TenantID) error`, declarada na composition root; ausente ou nula, o `Projector` recusa com `ErrProjectionDenied` (default fechado, IDN-17). A decisão de quem é o financiador de um tenant é da app, nunca do kernel.
- [ ] **[P0] Auditoria da projeção**: `ports.AuditProjectionWrite` (`projection.cross_tenant_write`) em `ports`, com o mesmo valor em `audit.ActionProjectionWrite` e reexportada como `usecase.ActionProjectionWrite`, com `Tenant` do contexto (ausente quando o contexto não tem tenant), `DataTenant` do destino e `Outcome` `accepted` ou `denied`: o evento é emitido antes de devolver o estado lido pelo `Load` e antes da gravação. A gravação só ocorre com o `Sink` confirmando o registro, e sem confirmação a projeção falha fechada com `ErrAuditUnavailable`, sob a mesma regra da leitura de plataforma.
- [ ] **[P0] Gate estático da projeção**: a mesma regra cobre o `Projector`, com a mesma prova em `dmpf-gate-check.sh`.
- [ ] **[P1] Envelope, se o ADR novo escolher o atributo novo** (não aplicável: o ADR-068 escolheu o destino no payload, e o envelope não muda): emenda de `docs/dmpf/cloudevents-protobuf-buf.md` (ENV-08, ENV-11, com categoria, presença e finalidade), campo em `contracts/envelope/envelope.go` (`:34,57,97,141,202`), em `PublishIntent` e `OutboxEntry`, no relay (`libs/backend/go/app/relay/record.go:49-54`) e no consumidor (`libs/backend/go/app/consumer.go:268-291`), mais testes de conformidade. Se o ADR escolher o destino no payload, este requisito não se aplica e o envelope não muda.
- [ ] **[P1] Paridade e conformidade**: `memory.Table.Projector` e cláusulas de `providerkit` para o destino explícito, a política ausente, a política que nega, o destino vazio, o `Load` (política, auditoria, falha fechada e falta sem `CrossTenantAccess`), a origem ausente sem tenant sintético e a construção sem trilha.

#### Investigação transversal

- [ ] **[P0] Investigar a atribuição do operador na leitura de plataforma**: (a) se o IDN-19 admite alcance total com sujeito autenticado, dado que `Route.Validate` só o aceita com `RequireNeither`; (b) como a identidade do operador chega à trilha quando a leitura roda atrás de gRPC, onde o canal não carrega sujeito (IDN-02; `libs/backend/go/grpc/interceptor_context.go:272-280`). Respondida pelo ADR-068, sem emenda normativa: (a) admite; (b) o evento do contexto sai sem `Subject`, e a trilha se liga à da borda pelo `correlation_id`. O carrier da identidade verificada do peer e a allowlist por método de plataforma ficam para emenda futura ao ADR-052.

### Não-funcionais

- [ ] Custo: os statements de plataforma e de projeção só são compilados quando a app os usa; o caminho comum de `Table` não ganha statement nem consulta.
- [ ] Limite: a leitura de plataforma nunca devolve página sem limite; o teto é configurável pela composition root.
- [ ] Compatibilidade: Go `1.26.9`; os aliases mantêm todo identificador público de `http` compilando.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Kernel `ports` | `PlatformReach`, carrier, `PlatformKey`, `PlatformRow`, `PlatformReader`, `Projector`, `ProjectionPolicy`, `PlatformPageMax` e os cinco erros |
| Kernel `application` | `PlatformPermitted` e a categoria dos erros novos |
| Kernel `postgres` e `memory` | `PlatformReader`, `Projector` |
| Kernel `http`, `grpc` e `app` | alias de `PlatformReach`; declaração na borda e carrier só em borda verificada |
| Kernel `observability` | duas ações de auditoria em `audit`, reexportadas em `usecase`, e o registrador com falha fechada |
| Kernel `testkit` (`providerkit`) | cláusulas de isolamento |
| App `bff` | carrier montado só em rota com sujeito, permissão e alcance |
| Tooling | regra do gate de plataforma, `dmpf-gate-check.sh`, checagem estrutural do `dmpf-context-check.sh`, template do `init` |
| Normas e docs | ADR-068; emendas aos ADR-050 e ADR-051; IDN-11 e IDN-12 emendados in-place; `cloudevents-protobuf-buf.md` não muda |

## Localização de código

```text
libs/backend/go/ports/platform.go                        novo: PlatformReach, carrier, PlatformKey, PlatformRow, PlatformReader, Projector, ProjectionPolicy, PlatformPageMax, erros
libs/backend/go/http/route.go                            alias de PlatformReach; Validate (:102-107)
libs/backend/go/application/authorize.go                 PlatformPermitted
libs/backend/go/postgres/platform.go                     novo: PlatformReader, Projector
libs/backend/go/postgres/table.go                        extração de loadAs e saveAs, com o tenant por parâmetro
libs/backend/go/memory/store.go, memory/tx.go            equivalentes
libs/backend/go/observability/audit/                     ActionPlatformRead, ActionProjectionWrite, registrador
libs/backend/go/observability/usecase/instrumentation.go reexport das duas ações
libs/backend/go/testkit/providerkit/platform.go          novo: cláusulas da leitura de plataforma
libs/backend/go/testkit/providerkit/projection.go        novo: cláusulas da projeção
libs/backend/go/grpc/, libs/backend/go/app/serve/        método declarado de plataforma
libs/backend/go/http/route.go                            WithVerifiedPlatformReach: carrier na borda verificada
apps/backend/bff/                                        chamada a WithVerifiedPlatformReach
.golangci.yml, tools/dmpf-plugin/templates/init/managed/.golangci.yml__tmpl__   regra do gate de plataforma
tools/dmpf-plugin/scripts/dmpf-gate-check.sh             prova da regra
tools/dmpf-plugin/scripts/dmpf-context-check.sh          checagem estrutural recursiva contra interface local
docs/adr/068-acesso-entre-tenants-declarado.md           novo
docs/adr/050-*.md, docs/adr/051-*.md                     emendas
docs/dmpf/contexto-erros-seguranca.md                    IDN-11 e IDN-12, in-place
```

O ADR-068 escolheu o destino no payload: `docs/dmpf/cloudevents-protobuf-buf.md`,
`contracts/envelope/envelope.go`, a outbox, o relay e o consumidor não mudam.

## Design

### Dois caminhos, uma declaração

```text
Leitura de plataforma
  borda declara ReachAllTenants ──► carrier (ports.WithPlatformReach)
      │                                     │
      ▼                                     ▼
  PlatformPermitted ──► Table.PlatformReader.List/LoadIn ──► audit.Event por tenant alcançado
  (permissão ou IDN-18)   SELECT sem predicado de tenant         (Tenant = do contexto, DataTenant = tenant lido)

Projeção
  consumidor (contexto no tenant A, vindo do envelope)
      │ destino B (do payload ou de dado da app, ADR-068)
      ▼
  ProjectionPolicy(A, B) ──► Table.Projector(tx).Load/Save(B, ...) ──► audit.Event (Tenant = A, DataTenant = B)
  nula ou negada: ErrProjectionDenied; destino vazio: ErrProjectionDestinationAbsent
```

### Origem do tenant de destino

| Alternativa | Envelope | Norma |
| ----------- | -------- | ----- |
| 1. Destino no payload de negócio, com escrita declarada (**escolhida pelo ADR-068**) | inalterado | ENV-11, ENV-12 e CTX-24 a CTX-26 intactos; IDN-11 e IDN-12 emendados in-place e IDN-14 atendido (nota abaixo); o ADR-050:23 é emendado |
| 2. Atributo novo do envelope | um atributo a mais (ENV-10: ASCII minúsculo e dígitos, até 20 caracteres) | exige emendar o artefato normativo (ENV-11) e dizer como o atributo convive com CTX-13 e CTX-24; IDN-11 e IDN-14 em aberto (nota abaixo) |
| 3. `tenantid` do envelope igual ao destino | valor muda | contraria ENV-12 (é o tenant resolvido por autenticação) e CTX-13; excluída |

O `Projector` serve às alternativas 1 e 2: muda só de onde o consumidor tira o
destino que passa à API.

Nas três alternativas a gravação cai fora do tenant do contexto, e o IDN-11 (escrita
escopada ao tenant do contexto, `:878`) e o IDN-14 (`:881`) não ficam intactos por si:
o `Projector` os atende com destino explícito, política que fecha por omissão, trilha e
gate estático. O ADR-068 concluiu que o IDN-14 é atendido pelo choke point e pelo gate
e que o IDN-11 pedia emenda, feita in-place, junto com a do IDN-12, porque o
`Projector.Load` devolve ao contexto o estado do destino.

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Onde vive a declaração de alcance | Carrier próprio em `ports`; `http` mantém alias | Décimo campo do `ExecutionContext` (CTX-01 fixa nove); inferir da ausência de tenant (IDN-19 proíbe); `postgres` importar `http` |
| Forma da leitura de plataforma | Tipo próprio `PlatformReader` sobre `ReadPool`, com statements compilados no kernel | Liberar o `pgx` ao contexto (ADR-051:47); parâmetro `unscoped` em `Table.Reader`, que afrouxaria o choke point com um booleano |
| Paginação | Keyset por `(tenant_id, id)`, limite obrigatório | `Relation.Query` sem limite (`libs/backend/go/postgres/table.go:103-139`), inaceitável entre tenants |
| Auditoria | Um evento por tenant alcançado, `audit.Event` inalterado | Campo novo no `Event`, que mudaria o contrato do LOG-14 |
| Falha do `Sink` | O acesso autorizado falha fechado sem a confirmação do registro durável, por decisão do kernel (IDN-19); o log recebe todo evento como canal adicional | Seguir ao log como `recordCrossTenant` faz: serve ao acesso já negado (IDN-12), mas deixaria o acesso autorizado prosseguir sem registro, e o log não tem retenção, acesso e integridade próprios (DAT-25) |
| Onde moram as ações de auditoria | `observability/audit`, reexportadas em `usecase` | Só em `usecase`: o provider não o importa |
| `Tenant` do evento | O tenant do contexto, ausente quando o contexto não tem tenant | `Tenant` sempre ausente na leitura de plataforma, como dizia o texto original: falsearia a trilha quando o contexto tem tenant |
| Quem monta o carrier | Só a borda do kernel que verificou quem chama: `http.WithVerifiedPlatformReach` na rota HTTP com sujeito e permissão; método gRPC cujo chamador o `TrustedPeers` admitiu pela allowlist | Montar em rota `RequireNeither` ou em servidor `Insecure`, que abriria leitura de todos os tenants a quem ninguém verificou |
| Onde nasce o `Projector` | Do `*Tx`, na transação da inbox e da outbox | Da pool, que perde a atomicidade sob o `UoW.Within` |
| `Load` do `Projector` | Com política e auditoria, sem a sonda de dono | Com a sonda, que geraria IDN-12 falso e revelaria existência em outro tenant; sem `Load`, que tiraria do consumidor a leitura do estado do destino |
| Mecanismo do gate | `forbidigo` por símbolo com exclusão por caminho e por `text`, mais a checagem estrutural do `dmpf-context-check.sh`; cobre `PlatformReader`, `Projector`, `ports.WithPlatformReach` e `http.WithVerifiedPlatformReach` | Package próprio sob `depguard`, inviável porque a leitura de plataforma depende de internals de `postgres` e de `memory`; regra só de revisão, porque o `context-provider` existe justamente para o isolamento não depender de convenção (IDN-14) |
| Pool da leitura de plataforma | A pool de runtime: o kernel só emite `SELECT` nesse caminho | Role de leitura própria: exige uma terceira role, fora do que a SPEC-QPRR24C4 provisiona |
| Destino da projeção | API com destino explícito e política que fecha por omissão | Destino implícito no `context.Context`, que apagaria o "explícito" |

## Riscos

| Risco | Mitigação |
| ----- | --------- |
| A leitura de plataforma vira via de extração em massa | Declaração, permissão, limite de página, evento por tenant e gate estático; a permissão é declarada por operação |
| A durabilidade da trilha é a do `Sink` configurado, e o DAT-25 pede retenção, acesso e integridade próprios | O kernel exige a confirmação do `Sink` antes do acesso e falha fechado sem ela; o `Sink` durável é do consumidor, e um `Sink` que confirma sem persistir derrota a garantia; o `NewLogSink` sempre confirma, e nenhuma app do repositório tem `Sink` durável |
| Todo workload admitido no servidor gRPC tem alcance total nos métodos de plataforma | Custo aceito no ADR-068; a allowlist por método e o carrier da identidade do peer ficam para emenda futura ao ADR-052 |
| Contorno do gate por interface local que o `Table` satisfaz | Checagem estrutural do `dmpf-context-check.sh`, com fixture do contorno |
| RLS futura esconderia as linhas de outros tenants também da leitura de plataforma e da sonda de dono | A spec de RLS trata os três caminhos; os testes desta spec rodam sob a role sem posse da SPEC-QPRR24C4 |
| A política de destino é código da app e pode ser frouxa | Default fechado, evento por gravação e teste de conformidade da política ausente |

## Regras relacionadas

- IDN-11 a IDN-20, CTX-13, CTX-24 a CTX-26, DAT-11 e DAT-25; ENV-10 a ENV-12; LOG-13 e LOG-14.
- ADR-049, ADR-050, ADR-051, ADR-052, ADR-066 e ADR-068.
- SPEC-9B6SHEH8 (contexto de execução e tenant), SPEC-QPRR24C4 (roles de banco), SPEC-XQWGGAXF (contexto de execução, erros, segurança e multi-tenancy) e SPEC-WYX5GW87 (contratos wire).

## Verificação e testes

### Critérios de aceite

- [ ] A suíte `providerkit` roda nas duas realizações (`memory` e `postgres`) e prova: a leitura de plataforma vê linhas de tenants distintos; sem declaração no carrier recusa; limite zero ou acima do teto é erro; `Reader` continua sem enxergar outro tenant nas duas realizações, e `Relation` no Postgres (o `memory` não tem `Relation`).
- [ ] A leitura de plataforma e a projeção rodam em teste de integração contra o Postgres sob a role de runtime sem posse (`OpenRuntimePool` da SPEC-QPRR24C4).
- [ ] Cada chamada de plataforma e cada gravação de projeção produz o número esperado de `audit.Event` (um por tenant alcançado) com `DataTenant` preenchido, inclusive nas recusas. Com um `Sink` que recusa o registro, a leitura e a projeção autorizadas falham fechadas com `ErrAuditUnavailable`, sem devolver linha nem gravar, e o log recebe o evento como canal adicional.
- [ ] O gate de plataforma reprova, por `dmpf-gate-check.sh`, um provider fora de `provider/platform_*.go` que use a leitura de plataforma ou a projeção, e um package fora do kernel de borda (`http`, `grpc`) que construa `ports.WithPlatformReach`, e um package fora da borda REST (`apps/**/app/api/`), como o `app/rpc` ou o provider do contexto, que chame `http.WithVerifiedPlatformReach`.
- [ ] As duas investigações têm resultado registrado no ADR novo, e o requisito condicional do envelope está aplicado ou declarado não aplicável.
- [ ] `go run ./tools/dmpf-conformance/cmd/conformance --root .` e `modsync --check` passam, a prova do consumidor externo (`tools/external-consumer.sh --verdaccio`, job `Consumidor externo por versão` do `ci.yml`) passa no PR de release para `master`, onde ela roda, e `node tools/adr-verify.mjs --from 63` passa com o ADR-067 da DEVS-132 presente.
- [ ] `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source` passa.

### Cenários de teste

**Happy path — leitura de plataforma declarada**
- DADO linhas dos tenants A e B na mesma tabela e uma operação que declara `ReachAllTenants`, cujo sujeito tem a permissão
- QUANDO a app chama `PlatformReader.List` com limite 50
- ENTÃO devolve as linhas de A e de B com o tenant de cada uma, e a trilha recebe um evento com `DataTenant` A e outro com `DataTenant` B, ambos com `Tenant` do contexto (ausente quando o contexto não tem tenant)

**Erro — leitura sem declaração**
- DADO uma operação sem `ReachAllTenants` no carrier, mesmo com o contexto sem tenant
- QUANDO a app chama `PlatformReader.List`
- ENTÃO recusa com `ErrPlatformReachUndeclared`, não devolve linha e registra o evento com `Outcome` `denied`

**Edge case — leitura comum continua filtrada**
- DADO o mesmo estado e um contexto do tenant A cujo carrier também traz `ReachAllTenants`
- QUANDO a app lê o identificador de B por `Reader.Load`
- ENTÃO responde `ErrNotFound` com `CrossTenantAccess` e evento IDN-12, como antes

**Happy path — projeção no tenant do financiador**
- DADO um consumidor no tenant A, destino B vindo do payload e uma política que autoriza A para B
- QUANDO `Projector.Save` grava o estado
- ENTÃO a linha nasce com `tenant_id` B, a trilha registra `Tenant` A e `DataTenant` B, e `Reader` de A não a enxerga

**Edge case — `Load` da projeção sem a linha no destino**
- DADO uma política que autoriza A para B e um identificador que só existe no tenant C
- QUANDO `Projector.Load` lê o identificador em B
- ENTÃO responde `ErrNotFound` sem `CrossTenantAccess`, e a trilha registra `DataTenant` B, sem evento IDN-12

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
- [P0] Todo acesso entre tenants emite o registro de auditoria antes de devolver ou gravar o dado, e o acesso autorizado falha fechado quando o `Sink` não confirma o registro durável: sem confirmação, nada é devolvido nem gravado. A falha fechada é decisão do kernel, com fonte no IDN-19, mais estrita que o DAT-25, que é condicional ao requisito regulatório e rege a separação da trilha. O canal de log é adicional, nunca substituto; o caminho de log de `recordCrossTenant` segue valendo só para o acesso cruzado por engano (IDN-12), que já é negado.
- [P0] O provider de contexto continua sem driver (`context-provider`): nenhum tipo do `pgx` entra na superfície pública nova.
- [P0] Nenhum tenant sintético (`system`, `default`, `unknown`) e nenhuma substituição da ausência por valor (IDN-20, CTX-26); `tenantid` do envelope continua o tenant resolvido por autenticação (ENV-12), salvo emenda ao artefato normativo (ENV-11).
- [P1] O `ExecutionContext` mantém os nove campos de CTX-01: a declaração de alcance viaja em carrier próprio.
- [P1] Nenhuma dependência de terceiros nova no `go.mod` do kernel.
</critical_constraints>

## Escopo fora

- **Escrita de plataforma com alcance total**: só a leitura é irrestrita; toda gravação entre tenants passa pelo `Projector`, com destino explícito e política.
- **Alcance parcial** (lista de tenants): o IDN-19 pede alcance declarado, e esta spec declara só "todos os tenants", o único valor que `PlatformReach` tem hoje.
- **RLS e role de leitura própria**: pertencem à evolução da SPEC-QPRR24C4; a RLS precisa tratar a leitura de plataforma, o `Load` da projeção e a sonda de dono (`libs/backend/go/postgres/table.go:183-187` e `:90-92`) em spec própria.
- **Retenção e imutabilidade da trilha**: o kernel exige a confirmação do `Sink` antes do acesso e falha fechado sem ela; retenção, acesso e integridade da trilha (DAT-25) são do consumidor.
- **Regra de negócio de quem é o financiador de um tenant**: é a política da app, não do kernel.
- **Agregação, junção e exportação entre tenants**: o ADR-051:47 manda estender o kernel caso a caso; esta spec cobre carga de linha por chave e página, não relatório.
- **Emendar o texto normativo de IDN-14 e IDN-16 a IDN-19**: o ADR-068 concluiu que não é necessário; o IDN-11 e o IDN-12 são emendados in-place por ele.
- **Carrier da identidade verificada do peer gRPC e allowlist por método de plataforma**: custo aceito no ADR-068, para emenda futura ao ADR-052.
- **`Relation` no `memory`** e **rota de plataforma real no `bff`**: a spec entrega só a montagem do carrier na borda.
