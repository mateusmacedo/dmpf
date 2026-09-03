---
id: SPEC-ZHE7DN1H
slug: dmpf-kernel-aplicacao-go
title: DMPF KRN-04 — Kernel de aplicação Go: Unit of Work e sequência canônica
stage: done
priority: P0
depends_on: [SPEC-MQA5HAXF, SPEC-WTAXFV8B, SPEC-XF9TF9A0]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-523
subtask_urls: []
created: 2026-09-03
---

# SPEC-ZHE7DN1H: DMPF KRN-04 — Kernel de aplicação Go: Unit of Work e sequência canônica

## Resumo

Criar dois módulos Go que realizam o lado do chamador da UPR: `dmpf-ports-go`
(`libs/backend/go/dmpf-ports`, bloco `port`), com a fronteira de Unit of Work,
o repositório com optimistic locking, a porta da outbox em tipos de domínio, o
relógio e o gerador de identificadores; e `dmpf-application-go`
(`libs/backend/go/dmpf-application`, bloco `application`), com o desfecho de
aplicação, a resolução de identidade do fato e um caso de uso de escrita que
percorre os nove passos da sequência canônica de FND-04 §3.2 sobre o agregado
`example/orders` do `KRN-03`. Uma realização em memória da UoW e das portas,
classificada como `provider`, fecha o caso de uso ponta a ponta com falha de
commit injetável. Como time de plataforma, queremos que as onze regras da UoW
(`UOW-01`..`UOW-11`) e a autoria dos campos da outbox (`BLK-01`, `BLK-04`,
`BLK-05`) deixem de ser prosa e passem a ser código que o verificador do
`KRN-02` e a suíte de aplicação provam — para que a atomicidade entre estado de
negócio e intenção de publicar tenha, pela primeira vez, um sujeito real.

Esta é a segunda metade do Incremento 2 ("Núcleo") da spec guarda-chuva
[SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md): "Caso de uso completo
em memória, com UPR pura e UoW explícita". A primeira metade é a
[SPEC-XF9TF9A0](./SPEC-XF9TF9A0-dmpf-kernel-dominio-go.md), que entregou o
passo 5. Esta spec entrega os passos 1–4 e 6–9, e o `KRN-06` substitui a
realização em memória pelo provider Postgres.

## Contexto

- **Problema**: o `dmpf-domain-go` devolve a `Decision`, mas ninguém a
  consome. A norma que descreve o consumidor — a UoW como fronteira de
  aplicação que envolve **uma** transação local e entrega ao callback as portas
  a ela vinculadas (`docs/dmpf/uow-inbox-outbox.md` §3.1, `UOW-01`..`UOW-04`),
  a sequência de nove passos com 6, 7 e 8 na mesma transação (§3.2, `UOW-05`..
  `UOW-08`), a ausência de retry automático (§3.3, `UOW-09`, `UOW-10`) e a
  autoria dos campos da outbox pelo application service (§2.3, `BLK-04`,
  `BLK-05`) — não tem realização Go. Sem ela, o `KRN-06` (outbox Postgres), o
  `KRN-07` (inbox) e o `KRN-09` (retry por conjunção) não têm o que estender, e
  a primeira contraprova de FND-04 §9.4 ("a outbox escrita pela tabela") segue
  sem vetor executável.
- **Impacto**: o `KRN-06` recebe a porta da outbox e a fronteira de UoW já
  fixadas em tipos de domínio, e implementa só o provider; o `KRN-07` reusa a
  UoW para inbox e efeitos locais na mesma transação; o `KRN-09` recebe a
  política de retry como ponto explícito, não como decorator; o verificador
  passa a ter, pela primeira vez, unidades `port` e `application` reais para
  exercitar as células 10, 11, 19, 22 e 23 sobre código do kernel.
- **Inspiração**: a forma canônica `UnitOfWork.within(contexto, recursos ->
  { recursos.pedidos.save(...); recursos.outbox.enqueue(...) })` de FND-04
  §3.1; o exemplo conforme de FND-04 §9.2 e a contraprova de §9.4; a decisão
  real de `shared-titulos-services` (RFC §7.5, evidência), única no universo
  inventariado que grava `backoffice_outbox` na mesma transação do caso de uso
  e drena em app dedicado; o `Outcome` como valor explícito em vez de `error`
  compartilhado, pela mesma razão que o ADR-032 escolheu `*Rejection` concreto.
- **Links relevantes**:
  - [SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md) — guarda-chuva;
    requisito `KRN-04` (linhas 147–152), fluxo principal (327–343), árvore de
    código (250–279), incremento 2 (404)
  - [SPEC-XF9TF9A0](./SPEC-XF9TF9A0-dmpf-kernel-dominio-go.md) — `KRN-03`; o
    agregado `example/orders` é o sujeito desta spec; "Escopo fora" delega a
    esta spec o relógio como porta e o tipo de instante compartilhado
  - [SPEC-WTAXFV8B](./SPEC-WTAXFV8B-dmpf-verificador-conformidade-go.md) —
    `KRN-02`; o verificador é o instrumento dos critérios de dependência
  - [SPEC-MQA5HAXF](./SPEC-MQA5HAXF-dmpf-fundacao-nx-go.md) — `KRN-01`;
    convenção de módulo, tags, cadeia de sete passos, `bounded_context:
    dmpf-kernel`
  - `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §2.1–§2.3, §3.1–§3.4, §7.1–§7.2
  - `docs/dmpf/upr-decision-mensagens.md` (FND-03) — §4.1–§4.4
  - `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §4.1, §6.2, §7.3, §7.4 (células
    10, 11, 12, 19, 22, 23, 24, 25, 30), §7.5, §9.3, P0-1, P0-2, P0-3
  - `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — `CTX-03`, `CTX-05`,
    `CTX-20`..`CTX-22`, `ERR-22`
  - ADRs: `docs/adr/010-*` (seis blocos e conjunção C1∧C2), `014-*` (aresta
    `domain → port` proibida), `016-*` (domínio executável em memória),
    `019-*` (três níveis de contrato), `021-*` (mapeamento no provider,
    serialização na escrita), `031-*` (verificador), `032-*` (desfecho da UPR
    em Go). O ADR desta história é o **`033`**.

### Divergências entre o ticket e o repositório

O ticket ARQ-523 foi escrito em 30/08, antes de `KRN-01`, `KRN-02` e `KRN-03`
serem entregues. Oito pontos do texto não batem com o estado do repositório e
ficam fixados aqui, sem reabrir decisão alguma:

| Ticket diz | Repositório | Esta spec fixa |
| --- | --- | --- |
| "Criar `libs/backend/dmpf-ports` e `libs/backend/dmpf-application`" | A convenção do ADR-030 e do `KRN-01` é `libs/<scope>/<stack>/<módulo>`, com nome de projeto Nx sufixado pela stack; o `KRN-05` (branch `feat/ARQ-524-contratos-wire`) já segue `libs/backend/go/dmpf-contracts` | `libs/backend/go/dmpf-ports` (projeto `dmpf-ports-go`) e `libs/backend/go/dmpf-application` (projeto `dmpf-application-go`) |
| "Os vetores negativos reprovam com `DMPF-D001` (V17, células 11 e 12)" | Na RFC §11.2, **V17** é a célula **23** (`port → provider`); a célula 11 (`application → provider`) é **V16**; a célula 12 (`application → contract`) não tem vetor numerado, e no `develop` não existe unidade `contract` para exercitá-la em módulo real | Vetores de módulo para V16 (c.11) e V17 (c.23), ambos `DMPF-D001`; a célula 12 é provada pela suíte de 36 células do `KRN-02` (`internal/rule/vectors_test.go`) e ganha vetor de módulo no `KRN-06`, primeiro módulo que coexiste com `dmpf-contracts-go` no `develop` |
| "ADR publicado e indexado" (sem número) | `032` é do `KRN-03` (`docs/adr/032-realizacao-go-do-desfecho-da-upr.md`); a spec do `KRN-05` na sua branch também reivindica `032` | O ADR desta história é o **`033`**. A colisão do `KRN-05` é registrada em "Escopo fora": ele renumera ao mergear |
| "Sequência canônica de nove passos (§3.2)" e, na `SPEC-XF9TF9A0:52`, "os dez passos" e "passos 1–4 e 6–10" | FND-04 §3.2 tem **nove** passos e registra a divergência com a Parte-1 §9.1 (dez): mapear e serializar colapsam em "entregar à porta" (`uow-inbox-outbox.md:546`). FND-03 §4.2 reproduz os dez da Parte-1 | **Nove**, numerados como em FND-04 §3.2. A prosa da `SPEC-XF9TF9A0` segue a Parte-1 e não é editada: está `done` |
| "Realização em memória da UoW e das portas; entradas no manifesto (`application`, `port`)" | Uma realização de porta é, por RFC §4.1, bloco `provider` — e `provider → application` é célula proibida, logo a realização não pode conhecer o tipo de recursos do caso de uso | A realização em memória é a unidade **`provider`** `dmpf-kernel/example-memory`, dentro de `dmpf-application`; o manifesto declara **três** blocos. O vínculo entre transação e recursos é feito por função de composição (`bind`) escrita pelo composition root |
| "Resolve tempo e identificadores" no passo 2 | O verificador classifica `time` como `io.clock` e os `rand` como `io.random` (`internal/rule/stdlib.go:28-33`), e a política de `application` e `port` admite só `pure` (`capability.go:46-47`) | O instante e o identificador entram por porta (`Clock`, `IDGenerator`), com valores próprios (`Instant`, `MessageID`); `dmpf-ports` e `dmpf-application` **não importam** `time`, `math/rand` nem `crypto/rand` |
| "Caso de uso de escrita executável sobre o agregado de `KRN-03`" | `example/orders` expõe `NewOrder` e `Snapshot()`, mas não reconstitui um `Order` a partir de `Snapshot` — sem isso, o passo 4 ("carrega") não tem forma pura | `example/orders` ganha `FromSnapshot(s Snapshot) *Order`, construtor puro no bloco `domain`, sem alteração de manifesto ou baseline. É a única edição ao `dmpf-domain-go` |
| "Vocabulário de tempo": a `SPEC-XF9TF9A0:685-686` delega a esta spec "um tipo de instante compartilhado e o relógio como porta" | O domínio não pode importar `port` (ADR-014), e a superfície do package raiz `dmpfdomain` está fechada em oito identificadores (`SPEC-XF9TF9A0:303-306`) | O tipo compartilhado é `dmpfports.Instant`, do bloco `port`; cada domínio mantém o seu valor (`orders.Instant`) e o application service converte. Nenhum símbolo é acrescentado ao raiz do `dmpfdomain` |

### Fontes normativas

Regras que esta spec realiza, com a força que cada fonte declara:

| Fonte | Regras | O que obriga aqui |
| --- | --- | --- |
| FND-04 §3.1 | `UOW-01`, `UOW-02`, `UOW-03`, `UOW-04` | Exatamente uma transação local sobre um recurso; UoW visível no service; recursos por parâmetro do callback; porta não recebida está fora da fronteira |
| FND-04 §3.2 | `UOW-05`, `UOW-06`, `UOW-07`, `UOW-08` | O ramo da `Decision` decide 6 e 7; sob `Rejected` nada persiste e o commit ocorre; 6 e 7 commitam juntos; nenhum passo publica |
| FND-04 §3.3 | `UOW-09`, `UOW-10` | O callback é invocado exatamente uma vez; retry de conflito é política explícita do caso de uso, só em operação idempotente |
| FND-04 §3.4 | `UOW-11` | Query não abre UoW de escrita nem grava outbox |
| FND-04 §2.1–§2.3 | `BLK-01`, `BLK-03`, `BLK-04`, `BLK-05` | Outbox por porta, nunca pela tabela; a porta recebe `(domain event, intenção)`; `destination` lógico; nenhum campo de estado de drenagem na escrita |
| FND-04 §7.1–§7.2 | `GAR-01`, `GAR-02`, `GAR-03`, `GAR-04` | At-least-once com efeitos idempotentes; transação local mais outbox; nenhum artefato promete exactly-once (P0-3, V31) |
| FND-03 §3, §4.1 | `DEC-01`..`DEC-04`, `DEC-10`, `DEC-11`, `FRT-01`..`FRT-04` | O chamador lê o desfecho exaustivo; rejeição de negócio nunca vira falha técnica; a UPR recebe o estado carregado e valores resolvidos, nunca contexto |
| FND-07 §3 | `CTX-03`, `CTX-05`, `CTX-20`, `CTX-21`, `CTX-22`, `ERR-22` | O contexto é argumento explícito; valores de `context.Context` não são fonte de correção; cancelamento e deadline alcançam o provider e não desfazem efeito commitado; panic é falha inesperada |
| RFC §6.2, §7.3, §7.4 | Política por bloco; células 4, 7, 10, 11, 12, 19, 22, 23, 24, 25 | `port` e `application` só `pure`; arestas permitidas e proibidas |
| RFC §9.3 | princípio 8 | Tempo, identificadores e dados externos entram como valores já resolvidos |
| ADR-021 | `BLK-03` | O mapeamento e a serialização são do provider; a porta expõe tipo de domínio |
| ADR-032 | forma do desfecho | `*Rejection` implementa `error` **para** que o application service o recupere com `errors.As`; a UPR nunca o devolve como `error` |

<constraints>
- [P0] Política de capabilities (RFC §6.2; `capability.go:46-47`): o fechamento
  transitivo de imports das unidades `port` e `application` contém APENAS
  capability `pure` segundo `internal/rule/stdlib.go`. NUNCA importar `time`,
  `math/rand`, `crypto/rand`, `os`, `net/*`, `database/sql`, `encoding/json`,
  `reflect`, `log` (em `port`), nem módulo de terceiro. `context`, `sync`,
  `errors`, `fmt`, `slices`, `maps` são puros e permitidos.
- [P0] Matriz de blocos (RFC §7.3; ADR-010): `port → domain` e `port → port`
  permitidas; `port → application`, `port → provider`, `port → contract`
  PROIBIDAS. `application → domain`, `application → application`,
  `application → port` permitidas; `application → provider` e
  `application → contract` PROIBIDAS. `domain → port` PROIBIDA (ADR-014): o
  `dmpf-domain-go` não importa `dmpf-ports`.
- [P0] Uma transação, um recurso, um callback (`UOW-01`, `UOW-02`, `UOW-09`):
  `Within` abre exatamente uma transação local sobre um recurso e invoca o
  callback exatamente uma vez. NUNCA repetir o callback, sob nenhum erro.
- [P0] Recursos por parâmetro (`UOW-03`, `UOW-04`, `CTX-05`): as portas
  vinculadas à transação chegam ao callback como argumento tipado. NUNCA por
  valor de `context.Context`, variável de package, service locator ou closure
  capturada fora do callback.
- [P0] O ramo decide 6 e 7 (`UOW-05`, `UOW-06`, `UOW-07`): sob `Accepted`,
  persistir com optimistic locking e entregar à outbox na MESMA transação; sob
  `Rejected`, nem repositório nem outbox recebem escrita, o commit OCORRE e o
  chamador recebe a rejeição tipada com `error == nil`.
- [P0] Outbox por porta em tipos de domínio (`BLK-01`, `BLK-03`, ADR-021,
  célula 24): a porta recebe `(DomainEvent, intenção de publicação)`. Nenhuma
  assinatura de `dmpf-ports` menciona driver, tabela, tópico, fila, ARN ou tipo
  de wire.
- [P0] Autoria dos campos (`BLK-04`, `BLK-05`): `message_id`, `occurred_at`,
  `destination`, `partition_key`, `aggregate_type`, `aggregate_id` e
  `aggregate_version` são escritos pelo application service; `destination` é
  lógico; NENHUM campo de estado de drenagem existe no tipo entregue à porta.
- [P0] Identidade antes da transação (§3.2 passo 2, `UOW-09` rationale):
  `occurred_at` e os `message_id` são resolvidos ANTES de `Within` abrir a
  transação, uma vez por execução do caso de uso.
- [P0] Classificação declarada (ADR-012; RFC §3.3, §10.1): cada package de
  produção é unidade com `include` por import path exato, `bounded_context:
  dmpf-kernel`, `public_integration_surface: false`, entrada no baseline;
  manifesto e baseline vêm em COMMIT PRÓPRIO, sem código Go (RFC §10.2).
- [P0] Nenhum artefato declara nem sugere exactly-once fim a fim (P0-3,
  `GAR-01`, V31); a documentação dos módulos declara at-least-once com efeitos
  idempotentes.
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva:
  divergência vira ADR-033, nunca alteração silenciosa da norma.
- [P1] `context.Context` transporta APENAS cancelamento e deadline (`CTX-20`,
  `CTX-21`); o contexto de execução de nove campos (`CTX-01`) e a autorização
  do passo 1 são de FND-07 e ficam fora desta spec.
- [P1] `go.mod` dos dois módulos sem `require` e sem `replace`; `go.sum`
  ausente. A resolução entre módulos irmãos é do `go.work`.
</constraints>

## Requisitos

### Funcionais

#### Módulo `dmpf-ports-go` — bloco `port`

- [x] **[P0] Módulo criado por `@nx-go/nx-go:library` em
  `libs/backend/go/dmpf-ports`**, package raiz `dmpfports`, projeto Nx
  `dmpf-ports-go`, tags exatamente `type:lib`, `scope:backend`, `stack:go`,
  `package.json` `{"name": "@lidercap-apps/dmpf-ports-go", "version":
  "0.0.0", "private": true}`, module path
  `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports`,
  `go 1.26.4`, entrada `./libs/backend/go/dmpf-ports` no bloco `use` do
  `go.work` em ordem alfabética. Cinco targets declarados com os mesmos
  comandos do `dmpf-domain` (`fmt-check`, `vet`, `build`, `test-race`,
  `govulncheck`); `lint` e `test` inferidos. Nenhum `implicitDependencies`:
  o `@nx-go/nx-go` deriva a aresta entre projetos lendo os `import` dos
  arquivos `.go` e casando-os com `go list -m -json`
  (`node_modules/@nx-go/nx-go/src/graph/create-dependencies.js`), sem
  depender de `require` no `go.mod`; o `affected` do CI enxerga
  `dmpf-ports-go → dmpf-domain-go` pelo próprio código.
- [x] **[P0] Valores de fronteira, sem `time` nem `rand`** (RFC §9.3; ADR-016):
  - `type Instant int64` — nanossegundos desde a época Unix, comparável e
    ordenável; método `Unix() int64` devolvendo segundos. É o tipo compartilhado
    que a `SPEC-XF9TF9A0` delegou; o domínio de cada contexto mantém o seu
    valor (`orders.Instant`) e o application service converte.
  - `type MessageID string` — identificador global único da mensagem
    (`message_id`, FND-04 §4.1). Vazio é inválido.
  - `type Version uint64` — versão do agregado para optimistic locking; `0`
    significa "ainda não persistido".
- [x] **[P0] Portas de tempo e identidade**, ambas I/O por RFC §6.2 (`io.clock`,
  `io.random`) e por isso `port`, nunca `domain` (ADR-014):
  ```go
  type Clock interface { Now() Instant }
  type IDGenerator interface { NewMessageID() MessageID }
  ```
- [x] **[P0] Repositório com optimistic locking** (FND-04 §3.3), genérico sobre
  identidade e estado persistido, em tipos do consumidor:
  ```go
  type Reader[ID comparable, S any] interface {
      Load(ctx context.Context, id ID) (S, Version, error)
  }
  type Repository[ID comparable, S any] interface {
      Reader[ID, S]
      Save(ctx context.Context, id ID, state S, expected Version) error
  }
  var ErrNotFound        = errors.New("dmpfports: aggregate not found")
  var ErrVersionConflict = errors.New("dmpfports: version conflict")
  ```
  - `Load` devolve `ErrNotFound` quando o agregado não existe.
  - `Save` grava `state` como versão `expected + 1` se, e somente se, a versão
    corrente for igual a `expected`; `expected == 0` cria; qualquer divergência
    devolve `ErrVersionConflict` **sem** gravar. Os sentinelas são comparáveis
    com `errors.Is` através de qualquer embrulho.
  - `Reader` existe separado para que a query de `UOW-11` receba só leitura.
- [x] **[P0] Porta da outbox em tipos de domínio** (`BLK-01`, `BLK-03`,
  `BLK-04`, `BLK-05`; ADR-021):
  ```go
  type PublishIntent struct {
      Destination  string // destino LÓGICO: nome do fluxo de integração (BLK-04)
      PartitionKey string // chave de ordenação; vazio quando não há ordem a preservar
  }
  type OutboxEntry struct {
      MessageID        MessageID
      OccurredAt       Instant
      Intent           PublishIntent
      AggregateType    string
      AggregateID      string
      AggregateVersion Version
      Event            dmpfdomain.DomainEvent
  }
  type Outbox interface { Enqueue(ctx context.Context, entry OutboxEntry) error }
  ```
  - Os sete campos são exatamente os três grupos de autoria do application
    service em FND-04 §2.3 (identidade e tempo, roteamento, origem de negócio)
    mais o evento. Os campos de wire (`message_type`, `schema_version`,
    `payload`) são do provider e NÃO existem aqui; os de estado de drenagem
    (`status`, `available_at`, `attempt_count`, `locked_by`, `locked_until`,
    `published_at`, `last_error`) são do schema e do relay e NÃO existem aqui.
  - `Enqueue` recebe `context.Context` só para cancelamento (`CTX-21`).
- [x] **[P0] Fronteira de Unit of Work** (FND-04 §3.1), genérica sobre o tipo
  de recursos que o caso de uso declara:
  ```go
  type UnitOfWork[R any] interface {
      Within(ctx context.Context, fn func(ctx context.Context, resources R) error) error
  }
  ```
  Contrato de `Within`, que toda realização cumpre:
  - abre exatamente **uma** transação local sobre **um** recurso (`UOW-01`,
    `UOW-02`); se `ctx.Err() != nil` antes de abrir, devolve `ctx.Err()` sem
    invocar `fn` — extensão por analogia de `CTX-21`, que enuncia a regra para
    o provider de I/O remota, à abertura da transação local;
  - invoca `fn` **exatamente uma vez** e nunca a repete (`UOW-09`);
  - `fn` devolve `nil` → commit; erro de commit é devolvido ao chamador tal
    como o provider o produziu; `fn` devolve erro → rollback e o mesmo erro é
    devolvido, sem embrulho que quebre `errors.Is`;
  - panic em `fn` → rollback e o panic é propagado (`ERR-22`: falha inesperada,
    nunca engolida nem convertida em rejeição);
  - `R` é montado pela realização a partir da transação aberta, e é o ÚNICO
    caminho pelo qual portas transacionais alcançam `fn` (`UOW-03`, `UOW-04`).
- [x] **[P0] Superfície mínima do package raiz `dmpfports`**: exatamente estes
  identificadores exportados — `Instant`, `MessageID`, `Version`, `Clock`,
  `IDGenerator`, `Reader`, `Repository`, `ErrNotFound`, `ErrVersionConflict`,
  `PublishIntent`, `OutboxEntry`, `Outbox`, `UnitOfWork`. Inbox, relay,
  cache, publicação e contexto de execução NÃO entram aqui (são de `KRN-07`,
  `KRN-08`, FND-07). Qualquer acréscimo é decisão de spec.
- [x] **[P0] Manifesto**: `dmpf-units.json` com uma unidade
  `dmpf-kernel/port`, `block: port`, `bounded_context: dmpf-kernel`,
  `public_integration_surface: false`, `include` =
  `[".../libs/backend/go/dmpf-ports"]`; `external: []`, `exceptions: []`.

#### Módulo `dmpf-application-go` — bloco `application`

- [x] **[P0] Módulo criado por `@nx-go/nx-go:library` em
  `libs/backend/go/dmpf-application`**, package raiz `dmpfapplication`,
  projeto Nx `dmpf-application-go`, mesmas tags, `package.json`
  `@lidercap-apps/dmpf-application-go` privado, module path
  `.../libs/backend/go/dmpf-application`, `go 1.26.4`, entrada no `go.work`,
  cinco targets idênticos, sem `implicitDependencies` (mesma razão do
  `dmpf-ports-go`).
- [x] **[P0] Desfecho de aplicação `Outcome[R]`** (`DEC-01`..`DEC-04`; FND-03
  §6): valor imutável que separa o canal de negócio do canal técnico:
  ```go
  type Outcome[R any] struct { response R; rejection *dmpfdomain.Rejection }
  func Accepted[R any](response R) Outcome[R]
  func Rejected[R any](rejection *dmpfdomain.Rejection) Outcome[R]
  func (o Outcome[R]) Response() R
  func (o Outcome[R]) Rejection() (*dmpfdomain.Rejection, bool)
  ```
  Todo caso de uso de escrita devolve `(Outcome[R], error)`: `error` transporta
  APENAS falha técnica (conflito de versão, commit, cancelamento, porta); a
  rejeição de negócio viaja no `Outcome` com `error == nil`. `Rejected(nil)` é
  defeito de programação e produz panic no construtor — ausência de rejeição
  não é um terceiro desfecho (`DEC-01`).
- [x] **[P0] Resolução de identidade — passo 2** (`UOW-09` rationale; FND-04
  §2.3 "por que a identidade nasce no application service"):
  ```go
  type Identity struct { OccurredAt dmpfports.Instant; MessageIDs []dmpfports.MessageID }
  func ResolveIdentity(clock dmpfports.Clock, ids dmpfports.IDGenerator, events int) Identity
  ```
  Lê o relógio **uma** vez e obtém `events` identificadores, onde `events` é o
  número máximo de eventos que o comando pode produzir, declarado pelo caso de
  uso. Chamada ANTES de `Within`. Identificador vazio ou repetido dentro da
  mesma resolução produz panic — é defeito do provider, não caso de uso.
- [x] **[P0] Gancho de autorização — passo 1** (`encaminhado` a FND-07):
  ```go
  type AuthorizeFunc[C any] func(ctx context.Context, cmd C) error
  func AllowAll[C any]() AuthorizeFunc[C]
  ```
  O caso de uso recebe a função e a invoca como primeiro passo; erro devolvido
  interrompe a sequência ANTES do passo 2 e é devolvido como `error` técnico.
  A taxonomia do erro de autorização é de FND-07 e não é modelada aqui.
- [x] **[P0] Caso de uso de escrita de referência em `example/orders`**
  (package `ordersapp`, unidade `dmpf-kernel/example-orders-application`),
  sobre `dmpf-domain/example/orders`:
  ```go
  type Resources struct {
      Orders dmpfports.Repository[orders.OrderID, orders.Snapshot]
      Outbox dmpfports.Outbox
  }
  type Service struct {
      UoW       dmpfports.UnitOfWork[Resources]
      Reader    dmpfports.Reader[orders.OrderID, orders.Snapshot] // fora da UoW (UOW-11)
      Clock     dmpfports.Clock
      IDs       dmpfports.IDGenerator
      Authorize dmpfapplication.AuthorizeFunc[Command]
      ItemLimit int
  }
  type AddItem    struct { Order orders.OrderID; SKU orders.SKU; Quantity int }
  type PlaceOrder struct { Order orders.OrderID }
  func (s Service) AddItem(ctx context.Context, cmd AddItem) (dmpfapplication.Outcome[orders.ItemAccepted], error)
  func (s Service) PlaceOrder(ctx context.Context, cmd PlaceOrder) (dmpfapplication.Outcome[orders.PlacedResponse], error)
  func (s Service) FindOrder(ctx context.Context, id orders.OrderID) (orders.Snapshot, error)
  const AggregateType = "orders.Order"
  const Destination   = "orders.events" // lógico (BLK-04)
  ```
  - `AddItem` e `PlaceOrder` percorrem os nove passos na ordem de FND-04 §3.2,
    com cada passo identificável no código (ver Design). `AddItem` "carrega
    ou cria": `ErrNotFound` no passo 4 vira `orders.NewOrder(cmd.Order,
    s.ItemLimit)` com `expected == 0`. `PlaceOrder` só carrega: `ErrNotFound`
    é devolvido como `error` técnico embrulhado — não é rejeição de domínio,
    porque nenhuma UPR a produziu, e a categoria de borda é de FND-07.
  - `PartitionKey` é `string(cmd.Order)`: eventos do mesmo agregado preservam
    ordem. `AggregateVersion` é a versão gravada no passo 6 (`expected + 1`).
  - `FindOrder` usa `s.Reader`, nunca `s.UoW`, e não toca a outbox (`UOW-11`).
  - Cada comando produz no máximo **um** evento, logo `ResolveIdentity` é
    chamada com `events = 1` e o `message_id` excedente não existe.
  - `Command` é a união `AddItem | PlaceOrder` via interface marcadora não
    exportada, para que um único `AuthorizeFunc` cubra o service.
- [x] **[P0] `FromSnapshot` no domínio** (`dmpf-domain/example/orders/order.go`):
  `func FromSnapshot(s Snapshot) *Order` reconstitui o agregado a partir do
  estado persistido, copiando `Items` com `slices.Clone` — a mesma disciplina
  de `Snapshot()` —, para que agregado e snapshot nunca compartilhem o slice
  (`DEC-12`). É computação pura sobre tipos do próprio package; não altera a
  superfície do raiz `dmpfdomain`, o manifesto nem o baseline. Commit com
  scope `dmpf-domain-go`.
- [x] **[P0] Realização em memória em `example/memory`** (package `memory`,
  unidade **`provider`** `dmpf-kernel/example-memory`):
  ```go
  type Store struct { /* mutex, orders map[orders.OrderID]record, outbox []dmpfports.OutboxEntry, ... */ }
  func New() *Store
  type Tx struct { /* cópia de trabalho + escritas pendentes */ }
  func (t *Tx) Orders() dmpfports.Repository[orders.OrderID, orders.Snapshot]
  func (t *Tx) Outbox() dmpfports.Outbox
  func NewUnitOfWork[R any](s *Store, bind func(tx *Tx) R) dmpfports.UnitOfWork[R]
  func (s *Store) Reader() dmpfports.Reader[orders.OrderID, orders.Snapshot] // visão fora da UoW
  func (s *Store) Entries() []dmpfports.OutboxEntry                            // cópia, para inspeção
  func (s *Store) FailNextCommit(err error)                                    // falha injetável
  func (s *Store) WithinCalls() int
  type FixedClock struct { At dmpfports.Instant }        // Now() devolve At
  type SequenceIDs struct { Prefix string }               // "m-000001", "m-000002", ...
  ```
  - `Within` copia o estado corrente para a `Tx`, invoca `fn` uma vez com
    `bind(tx)`, e no retorno `nil` aplica as escritas pendentes ao `Store` sob
    o mutex — a menos que `FailNextCommit` tenha armado um erro, caso em que
    descarta a `Tx` e devolve o erro. Erro de `fn` ou panic descartam a `Tx`.
  - O repositório da `Tx` grava e lê **valores** (`Snapshot`), nunca ponteiros
    compartilhados com o `Store`: uma UPR que mute o `*Order` carregado não
    alcança o `Store` antes do commit.
  - O `bind` é escrito pelo composition root (nos testes, o próprio arquivo de
    teste), porque `provider → application` é célula proibida e a realização
    não pode conhecer `ordersapp.Resources`.
  - O `Store` não simula conflito de versão: sob o mutex, `Load` e `Save` da
    mesma `Tx` veem sempre o mesmo estado. O conflito de `UOW-09` é injetado
    pelos testes de `ordersapp` (`doubles_test.go`): o `bind` embrulha
    `tx.Orders()` em um `Repository` instrumentado cujo `Save` devolve
    `dmpfports.ErrVersionConflict` e conta as chamadas. O mesmo arquivo
    declara os embrulhos que gravam a ordem das chamadas para o teste da
    sequência.
  - Não exercita isolamento nem conflito de serialização entre transações
    concorrentes: o mutex serializa `Within`. Isso é declarado no `doc.go` e
    pertence ao `KRN-06`.
- [x] **[P0] Manifesto**: `dmpf-units.json` com três unidades, todas
  `bounded_context: dmpf-kernel`, `public_integration_surface: false`:
  `dmpf-kernel/application` (`block: application`, raiz),
  `dmpf-kernel/example-orders-application` (`block: application`,
  `example/orders`), `dmpf-kernel/example-memory` (`block: provider`,
  `example/memory`); `external: []`, `exceptions: []`.
- [x] **[P0] Baseline**: `tools/dmpf-baseline/units-baseline.json` regravado
  com `--write-baseline` e passa a ter **quatro** entradas novas (uma de
  `dmpf-ports`, três de `dmpf-application`), digest recalculado. Os dois
  manifestos e o baseline vêm em **um commit próprio**, sem código Go, com o
  ato declarado no assunto e no corpo (unidades, blocos, contexto, arestas
  liberadas: `port → domain`, `application → domain`, `application → port`,
  `provider → port`, `provider → domain`), como exige RFC §10.2 e cobra
  `internal/baseline/authorization.go`. Precedentes: `92fcd4d` (ARQ-521),
  `6cc2ce7` (ARQ-522).

#### Gate local e vetores

- [x] **[P0] Regras `port` e `application` no `.golangci.yml`**, espelhando a
  regra `domain` (`list-mode: strict`), com `files` `**/*-ports/**` e
  `**/*-application/**`. A `allow` de ambas é a de `domain` **sem `time`** (o
  verificador o classifica `io.clock` e é o gate autoritativo) e com o
  universo do repositório; a de `application` acrescenta `log/slog` e `log`
  (observability é permitida ao bloco pelo verificador, `capability.go:47`);
  a `deny` de ambas acrescenta `time` com `desc` "io.clock; o instante entra
  por porta". `forbidigo` permanece restrito a `-domain/`: fora do domínio,
  `errors.New`, `fmt.Errorf` e `panic` são legítimos.
- [x] **[P0] `tools/dmpf-gate-check.sh` cobre os três blocos**: o filtro de
  `block === "domain"` passa a aceitar `domain`, `port` e `application`;
  `depguard_cobre()` reconhece `-ports/` e `-application/`; o array de
  vetores de package ganha `time|io.clock|dentro`, aplicado aos módulos
  `port` e `application`; o array de símbolos (`forbidigo`) segue aplicado
  só a `domain`. O vetor positivo (árvore limpa passa) roda nos quatro
  módulos. A mensagem final conta módulos por bloco.
- [ ] **[P0] Vetores do verificador sobre os módulos reais**, executados sobre
  cópia temporária (mesmo gesto da `SPEC-XF9TF9A0`), cada um exigindo o
  código esperado e saída `1` do gate:
  - V16 (célula 11): arquivo em `dmpf-application` raiz importando
    `.../dmpf-application/example/memory` → `DMPF-D001`.
  - V17 (célula 23): arquivo em `dmpf-ports` importando
    `.../dmpf-application/example/memory` → `DMPF-D001`.
  - Célula 20: arquivo em `dmpf-ports` importando `.../dmpf-application` →
    `DMPF-D001`.
  - Célula 4: arquivo em `dmpf-domain` importando `.../dmpf-ports` →
    `DMPF-D001` (ADR-014).
  - V21 (§6.2): `import _ "time"` em `dmpf-ports` e em `dmpf-application` →
    `DMPF-E001` com `Target time`.
  - Positivo: a árvore entregue passa no verificador com zero diagnósticos e
    zero `NAO VERIFICADO` novos.
  - **Parcial.** V16 (c.11) e V21 (nos dois módulos) emitiram `DMPF-D001` e
    `DMPF-E001` como especificado, e o positivo passou. V17 (c.23), a célula 20 e
    a célula 4 são inexequíveis como `DMPF-D001` sobre os módulos reais: a aresta
    proibida aponta na direção contrária a uma aresta de produção que já existe
    (`memory → ports`, `application → ports`, `ports → domain`), então o Go recusa
    por ciclo de imports antes de o verificador avaliar o bloco, e o diagnóstico é
    `DMPF-E003`. As três células seguem provadas, e com módulos sintéticos sem
    aresta contrária, pelo oráculo de 36 células do `KRN-02`
    (`internal/rule/matrix_oracle_test.go:20,36,39`). Registrado no ADR-033.

#### Documentação

- [x] **[P1] `README.md` em cada módulo**, no molde do `dmpf-domain-go`: o que
  o bloco é e não é, tabela package → unidade, o contrato de `Within`, os nove
  passos apontando para o código, "o que o módulo não contém" (Postgres, wire,
  inbox, relay, retry, telemetria, contexto de execução), validação e
  governança. A seção de garantias declara **at-least-once com efeitos
  idempotentes** e nomeia a vedação a exactly-once (P0-3).
- [x] **[P1] `AGENTS.md`**: a seção "### Libs" passa a listar quatro módulos
  ("Duas, ambas Go" → "Quatro, todos Go"), com um parágrafo para cada módulo
  novo no molde dos existentes; o bloco "Cadeia Go" cita
  `tools/dmpf-gate-check.sh` como prova dos blocos `domain`, `port` e
  `application`.
- [x] **[P1] ADR-033** em `docs/adr/033-fronteira-de-uow-em-go.md`, indexado
  em `docs/adr/README.md` no formato das linhas 129–131, registrando: UoW
  genérica sobre o tipo de recursos com vínculo por `bind` no composition
  root; `Outcome[R]` como separação dos canais; identidade resolvida antes da
  transação com `events` declarado; `Instant` e `MessageID` como valores de
  porta; realização em memória como `provider` com falha de commit injetável;
  resolução entre módulos irmãos pelo `go.work` sem `require`. Criado na
  Etapa 6 do pipeline.

### Não-funcionais

- [x] Sem I/O em teste: `go test ./...` de cada módulo termina em menos de 2 s
  a frio e não cria arquivo, socket ou processo; o fechamento de imports de
  produção não contém `os`, `net`, `os/exec`, `time` nem `testing`.
- [x] `go test -race -count=2 -shuffle=on ./...` verde nos dois módulos:
  nenhuma corrida no `Store` sob `Within` concorrente e nenhum teste
  dependente de ordem.
- [x] `go.mod` dos dois módulos sem `require` e sem `replace`; `go.sum`
  ausente. Verificado empiricamente (2026-09-03) que, em modo workspace, `go
  vet`/`go build`/`go test` resolvem o módulo irmão pelo `go.work` sem
  `require`, e que `go mod tidy` tenta a rede para resolvê-lo — por isso o
  target `tidy` inferido pelo plugin NÃO faz parte da cadeia e não é invocado
  pelo CI.
- [x] Cadeia Go verde por `pnpm nx` nos dois módulos: `fmt-check`, `vet`,
  `lint`, `build`, `test`, `test-race`, `govulncheck`; `bash
  tools/dmpf-gate-check.sh` verde; `go run
  ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --base
  origin/develop` com saída `0`.
- [ ] `pnpm biome ci .` e `pnpm nx affected -t lint,typecheck,test,build
  --exclude=@nx-base-template/source` verdes.
  - **Parcial.** O `nx affected` passou (4 projetos, `0 issues`). O `pnpm biome ci .`
    devolve `Checked 0 files` e exit 1, com "These paths were provided but ignored:
    - ." — achado **pré-existente**, não desta entrega: o `biome.json` não foi
    tocado na branch e o comportamento se reproduz num clone limpo fora do
    worktree. `pnpm biome ci libs tools` processa 35 arquivos e passa, e o
    `pre-commit` do lefthook rodou `biome check --write` em cada commit. O comando
    é o mesmo do `ci.yml:47`, então merece correção própria, fora do escopo desta
    história.
- [x] Nenhum comentário em código que reafirme assinatura ou corpo adjacente;
  os `doc.go` explicam o que cada bloco não contém e por quê, com referência
  à regra (mesma disciplina do `dmpf-domain-go`).

## Camadas afetadas

| Camada | Impacto |
| --- | --- |
| **Módulo Go `dmpf-ports`** (novo) | Bloco `port`: valores de fronteira, relógio, gerador de identificador, repositório com versão, outbox, UoW |
| **Módulo Go `dmpf-application`** (novo) | Bloco `application` (raiz e `example/orders`) e bloco `provider` (`example/memory`) |
| **Módulo Go `dmpf-domain`** | `FromSnapshot` em `example/orders`; nenhuma mudança de classificação |
| **Workspace Go (`go.work`)** | Duas entradas novas no bloco `use` |
| **Governança (manifesto + baseline)** | Dois manifestos novos, quatro unidades, baseline regravado; commit próprio |
| **Lint** | `.golangci.yml` ganha as regras `port` e `application`; `tools/dmpf-gate-check.sh` cobre os três blocos |
| **CI** | Nenhuma mudança em `ci.yml`: `Go gates (affected)`, `DMPF dependency gate` e `DMPF conformance gate` já cobrem projetos `stack:go` afetados |
| **Nx** | Dois `project.json` novos com tags e cinco targets; nenhuma mudança em `nx.json`; arestas entre projetos derivadas pelo plugin a partir dos `import` |
| **Documentação** | Dois `README.md`, ADR-033, índice de ADRs, `AGENTS.md` |

Nenhuma camada de produto é afetada: não há app, endpoint, banco, fila nem
broker nesta entrega. A realização em memória não é infraestrutura.

## Localização de código

```text
lidercap-platform/
├── go.work                                          # MODIFICAR — use ./libs/backend/go/dmpf-application e ./libs/backend/go/dmpf-ports
├── libs/backend/go/dmpf-ports/                      # CRIAR — projeto Nx dmpf-ports-go, bloco port
│   ├── go.mod                                       # module .../libs/backend/go/dmpf-ports, go 1.26.4, sem require
│   ├── package.json                                 # @lidercap-apps/dmpf-ports-go, private: true
│   ├── project.json                                 # tags 3D, 5 targets
│   ├── dmpf-units.json                              # unidade dmpf-kernel/port (commit próprio)
│   ├── README.md                                    # o que o bloco port é e não é
│   ├── doc.go                                       # package dmpfports
│   ├── values.go                                    # Instant, MessageID, Version
│   ├── clock.go                                     # Clock, IDGenerator
│   ├── repository.go                                # Reader, Repository, ErrNotFound, ErrVersionConflict
│   ├── outbox.go                                    # PublishIntent, OutboxEntry, Outbox
│   ├── uow.go                                       # UnitOfWork
│   ├── values_test.go                               # Instant.Unix, ordenação
│   └── contract_test.go                             # o contrato de Within enunciado como suíte reutilizável (func RunUnitOfWorkContract)
├── libs/backend/go/dmpf-application/                # CRIAR — projeto Nx dmpf-application-go
│   ├── go.mod                                       # module .../libs/backend/go/dmpf-application, go 1.26.4, sem require
│   ├── package.json                                 # @lidercap-apps/dmpf-application-go, private: true
│   ├── project.json                                 # tags 3D, 5 targets
│   ├── dmpf-units.json                              # 3 unidades: application, example-orders-application, example-memory (commit próprio)
│   ├── README.md                                    # os nove passos apontando para o código; at-least-once declarado
│   ├── doc.go                                       # package dmpfapplication
│   ├── outcome.go                                   # Outcome, Accepted, Rejected
│   ├── identity.go                                  # Identity, ResolveIdentity
│   ├── authorize.go                                 # AuthorizeFunc, AllowAll
│   ├── outcome_test.go                              # exaustividade, imutabilidade, Rejected(nil) panic
│   ├── identity_test.go                             # uma leitura de relógio, n identificadores, repetição panic
│   ├── example/orders/                              # unidade dmpf-kernel/example-orders-application (bloco application)
│   │   ├── doc.go                                   # package ordersapp
│   │   ├── service.go                               # Service, Resources, Command, constantes
│   │   ├── add_item.go                              # os nove passos, comentados por número
│   │   ├── place_order.go                           # os nove passos
│   │   ├── find_order.go                            # query fora da UoW (UOW-11)
│   │   ├── doubles_test.go                          # portas instrumentadas: gravam a ordem das chamadas, injetam ErrVersionConflict no Save (via bind)
│   │   ├── add_item_test.go                         # aceite, rejeição, criação, atomicidade, conflito, autoria
│   │   ├── place_order_test.go                      # aceite, rejeição, not found
│   │   ├── sequence_test.go                         # ordem dos nove passos observada por portas instrumentadas
│   │   └── find_order_test.go                       # zero Within, zero outbox
│   └── example/memory/                              # unidade dmpf-kernel/example-memory (bloco provider)
│       ├── doc.go                                   # package memory: o que não prova (isolamento, serialização)
│       ├── store.go                                 # Store, New, Reader, Entries, FailNextCommit, WithinCalls
│       ├── tx.go                                    # Tx, Orders, Outbox, NewUnitOfWork
│       ├── clock.go                                 # FixedClock, SequenceIDs
│       ├── store_test.go                            # commit, rollback, falha injetada, panic, ctx cancelado
│       └── contract_test.go                         # dmpfports.RunUnitOfWorkContract sobre NewUnitOfWork
├── libs/backend/go/dmpf-domain/example/orders/
│   └── order.go                                     # MODIFICAR — FromSnapshot (commit scope dmpf-domain-go)
├── tools/dmpf-baseline/units-baseline.json          # MODIFICAR — +4 entradas, digest (commit próprio)
├── tools/dmpf-gate-check.sh                         # MODIFICAR — blocos port e application, vetor time
├── .golangci.yml                                    # MODIFICAR — regras port e application
├── docs/adr/033-fronteira-de-uow-em-go.md           # CRIAR — Etapa 6
├── docs/adr/README.md                               # MODIFICAR — linha do ADR-033
├── AGENTS.md                                        # MODIFICAR — inventário de libs e cadeia Go
├── libs/backend/go/dmpf-conformance/                # INTOCADO — instrumento, não objeto
└── docs/dmpf/                                       # INTOCADO — acervo normativo
```

Import paths canônicos (as `canonical_key` das quatro unidades novas):

- `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports`
- `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application`
- `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/orders`
- `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory`

`RunUnitOfWorkContract` vive em arquivo `_test.go` de `dmpfports` e, como
arquivo de teste nunca é importável por outro módulo, a
`memory/contract_test.go` **duplica o corpo** do contrato (as cinco cláusulas
de `Within`) sobre `NewUnitOfWork`, sem importar nada de teste do
`dmpf-ports`. Um test kit exportado, que elimine a duplicação, é do `KRN-11`
(`dmpf-testkit`).

## Design

### Arquitetura

```text
libs/backend/go/dmpf-domain          libs/backend/go/dmpf-ports           libs/backend/go/dmpf-application
(bounded_context: dmpf-kernel)       (bounded_context: dmpf-kernel)       (bounded_context: dmpf-kernel)

┌──────────────────────────┐         ┌──────────────────────────────┐    ┌────────────────────────────────┐
│ dmpfdomain      [domain] │◄────────│ dmpfports             [port] │◄───│ dmpfapplication  [application] │
│ DomainEvent · Accepted   │  c.19   │ Instant · MessageID · Version│c.10│ Outcome · ResolveIdentity      │
│ Rejection · Code         │         │ Clock · IDGenerator          │    │ AuthorizeFunc                  │
└─────────▲────────────────┘         │ Reader · Repository          │    └──────────▲─────────────────────┘
          │ c.1                      │ PublishIntent · OutboxEntry  │               │ c.8
┌─────────┴────────────────┐         │ Outbox · UnitOfWork[R]       │    ┌──────────┴─────────────────────┐
│ orders          [domain] │◄──┐     └───────▲──────────────────────┘    │ ordersapp        [application] │
│ Order · FromSnapshot     │   │ c.7         │ c.10                      │ Service · Resources            │
│ AddItem · Place · eventos│   └─────────────┼───────────────────────────│ AddItem · PlaceOrder · Find    │
└─────────▲────────────────┘               ┌─┴────────────────────────┐  └────────────────────────────────┘
          │ c.25                           │ memory        [provider] │            ▲ só em _test.go
          └────────────────────────────────│ Store · Tx · NewUnitOfWork│───────────┘ (bind escrito no teste)
                                   c.28    │ FixedClock · SequenceIDs │
                                           └──────────────────────────┘

  Arestas de produção: c.1 domain→domain, c.7 application→domain, c.8 application→application,
  c.10 application→port, c.19 port→domain, c.25 provider→domain, c.28 provider→port — todas P, todas
  no mesmo bounded_context (C2). Nenhuma unidade importa `time`, `rand`, `encoding/*` ou terceiro.
  O provider NUNCA importa application (célula proibida): o vínculo Tx → Resources é o `bind` do
  composition root.
```

O verificador do `KRN-02` vê sete unidades do `dmpf-kernel` (três do `KRN-03`
mais quatro desta spec), sete arestas internas permitidas e um fechamento
transitivo de stdlib inteiramente `pure` pela tabela de `internal/rule/stdlib.go`.

### Fluxo principal — os nove passos em `AddItem`

```text
1. authorize    s.Authorize(ctx, cmd) → erro interrompe; nada foi resolvido nem aberto
2. identity     id := dmpfapplication.ResolveIdentity(s.Clock, s.IDs, 1)     ← antes da transação
3. uow          err := s.UoW.Within(ctx, func(ctx, res Resources) error {
4. load           snap, ver, err := res.Orders.Load(ctx, cmd.Order)
                  ErrNotFound → order = orders.NewOrder(cmd.Order, s.ItemLimit); ver = 0
                  senão       → order = orders.FromSnapshot(snap)
5. decide         acc, rej := order.AddItem(orders.AddItem{SKU, Quantity, At: orders.Instant(id.OccurredAt.Unix())})
                  if rej != nil { outcome = Rejected(rej); return nil }        ← 6 e 7 não ocorrem; commit ocorre
6. save           if err := res.Orders.Save(ctx, cmd.Order, order.Snapshot(), ver); err != nil { return err }
7. enqueue        for i, ev := range acc.Events() {
                    res.Outbox.Enqueue(ctx, dmpfports.OutboxEntry{
                      MessageID: id.MessageIDs[i], OccurredAt: id.OccurredAt,
                      Intent: {Destination, PartitionKey: string(cmd.Order)},
                      AggregateType, AggregateID: string(cmd.Order), AggregateVersion: ver + 1,
                      Event: ev }) }
                  outcome = Accepted(acc.Response()); return nil
8. commit       })                                                            ← dentro de Within
9. respond      if err != nil { return Outcome{}, err }; return outcome, nil
```

Sob `Rejected`, o callback devolve `nil` **de propósito**: a UoW commita uma
transação vazia de efeitos e o chamador recebe `(Rejected(rej), nil)`. Abortar
tornaria a rejeição indistinguível de falha técnica, o que `DEC-04` proíbe e
FND-04 §3.2 registra como decisão deliberada.

`ErrVersionConflict` no passo 6 sobe como `error`, sem repetição. A política de
retry, quando existir, é do caso de uso e do `KRN-09` (`RES-35`), nunca de
`Within`.

### Pseudocódigo — `memory.NewUnitOfWork`

```text
Within(ctx, fn):
  if ctx.Err() != nil: return ctx.Err()                         # CTX-21, antes de abrir
  store.mu.Lock(); defer store.mu.Unlock()                      # uma transação por vez
  store.withinCalls++
  tx := Tx{orders: clone(store.orders), pending: {}, outbox: []}
  resources := bind(&tx)                                        # o ÚNICO caminho até as portas transacionais
  defer recover-and-rethrow: descarta tx, propaga o panic       # ERR-22
  if err := fn(ctx, resources); err != nil: return err          # rollback: tx é descartada
  if store.failNextCommit != nil:                               # falha injetável
      err := store.failNextCommit; store.failNextCommit = nil; return err
  store.orders = tx.orders; store.outbox = append(store.outbox, tx.outbox...)   # commit atômico sob o mutex
  return nil
```

`Tx.Orders().Save` valida `expected` contra `tx.orders[id].version` e grava
em `tx.orders`; `Tx.Outbox().Enqueue` acrescenta a `tx.outbox`. Nada toca
`store` antes do commit. `store.Reader()` lê `store.orders` sob o mutex, fora
de qualquer `Tx`.

### Onde cada regra é provada

| Regra | Prova |
| --- | --- |
| `UOW-01`, `UOW-02` | `Store` tem um recurso; `Within` abre uma `Tx`; `WithinCalls()` conta 1 por caso de uso |
| `UOW-03` | `Resources` é parâmetro de `fn`; `Service` não tem campo de porta transacional |
| `UOW-04` | teste grava em um segundo `Store` (recurso não vinculado) de dentro do callback do primeiro e devolve erro: o primeiro sofre rollback e a escrita no segundo permanece — o efeito não participou do commit |
| `UOW-05`, `UOW-06` | `add_item_test`: sob `Rejected`, `Entries()` vazio, `Reader().Load` igual ao anterior, `WithinCalls() == 1`, `err == nil` |
| `UOW-07` | `FailNextCommit`: nem estado nem outbox persistem |
| `UOW-08` | `dmpfports` não tem porta de publicação; `Outbox.Enqueue` é o único sumidouro |
| `UOW-09` | `ErrVersionConflict` injetado pelo `Repository` instrumentado de `doubles_test.go` (via `bind`): `fn` invocada uma vez, `Save` chamado uma vez, `Enqueue` nunca |
| `UOW-10` | nenhum parâmetro de retry em `UnitOfWork`, `Service` ou `Store` |
| `UOW-11` | `find_order_test`: `WithinCalls() == 0`, `Entries()` vazio |
| `BLK-01` | `ordersapp` não importa `memory`; vetor V16 reprova a tentativa |
| `BLK-04`, `BLK-05` | `OutboxEntry` não tem campo de drenagem; `Destination == "orders.events"` |
| `DEC-04` | `Outcome` separado de `error`; `Rejected(nil)` panic |
| `CTX-21` | `Within` com contexto cancelado devolve `ctx.Err()` e `WithinCalls()` não incrementa |

## Decisões técnicas

- **`UnitOfWork[R any]` genérica sobre o tipo de recursos do caso de uso, com
  vínculo por `bind` no composition root**, porque `UOW-03` exige que as
  portas cheguem por parâmetro tipado e `UOW-04` exige que só as portas
  vinculadas cheguem — e a única forma de o compilador garantir as duas é o
  tipo `R` ser do caso de uso. O provider não pode conhecer `R` (célula
  `provider → application` proibida), logo alguém que conheça os dois lados
  monta `R` a partir da transação: é o composition root (bloco `app`; nos
  testes, o arquivo de teste). Alternativa descartada: `Within(ctx, func(ctx,
  tx Transaction) error)` com `tx.Repository("orders")` por nome — service
  locator dentro da fronteira, que `UOW-03` veda nominalmente. Alternativa
  descartada: recursos em `context.Context` — `CTX-05`.
- **`Outcome[R]` como valor e `error` só para falha técnica**, e não `(R,
  error)` com `*Rejection` viajando como `error`, porque no application service
  o canal de erro também transporta conflito, commit e cancelamento — e o
  ADR-018 só admite o par de Go quando o canal de erro carrega **apenas**
  rejeições de domínio. Com `Outcome`, a rejeição nunca compete com a falha
  técnica pelo mesmo `if err != nil`, e a exaustividade de `DEC-01` chega ao
  adapter. `*Rejection` continua implementando `error` para o uso que o ADR-032
  previu: o service pode embrulhá-lo em log com `errors.Join` sem perdê-lo.
- **Identidade resolvida no passo 2 com `events` declarado pelo caso de uso**,
  porque a sequência de FND-04 fixa o passo 2 antes da transação e o
  `rationale` de `UOW-09` explica por quê: uma reexecução geraria identidade
  nova para o mesmo fato. O caso de uso sabe quantos eventos o comando pode
  produzir (o exemplo produz no máximo um); o kernel não adivinha. Alternativa
  descartada: gerar `message_id` no passo 7, por evento — mantém a identidade
  dentro da transação e reabre a porta que `UOW-09` fechou.
- **`Instant` como inteiro de nanossegundos em `dmpfports`, sem `time`**,
  porque a política do bloco `port` é só `pure` e o verificador classifica
  `time` inteiro como `io.clock` (`stdlib.go:28`). Nanossegundos porque
  `available_at` ancora em `occurred_at` (FND-04 §4.2) e a drenagem ordena por
  ele; segundos colidiriam sob carga. `Unix()` existe para o exemplo, cujo
  `orders.Instant` é de segundos. Alternativa descartada: `time.Time` no
  domínio via depguard "como tipo" — o gate autoritativo é o verificador, e a
  `SPEC-XF9TF9A0` já registrou a divergência doc/código do `KRN-02`.
- **`Clock` e `IDGenerator` como portas em `dmpf-ports`, nunca no domínio**,
  porque ler o relógio e obter entropia são I/O (`io.clock`, `io.random`) e o
  domínio "não pede a hora; ele a recebe" (RFC §9.3; ADR-014; ADR-016).
  Alternativa descartada: `Clock` em `dmpfdomain` como interface "pura" — é o
  caso residual que o ADR-014 fecha explicitamente.
- **Realização em memória como unidade `provider` dentro de
  `dmpf-application`**, e não como módulo à parte nem como código só de
  teste, porque um módulo à parte traria manifesto, baseline, `package.json`,
  `project.json` e `go.work` só para um exemplo (mesma razão da
  `SPEC-XF9TF9A0` para `example/orders`), e código só de teste não seria
  reutilizável pelo `KRN-07` e pelo `KRN-09` nem exercitaria a célula 28
  (`provider → port`) no verificador. O manifesto de três blocos tem
  precedente: o `dmpf-conformance` declara oito unidades em cinco blocos.
- **`Repository[ID, S]` sobre o estado persistido (`Snapshot`), não sobre
  `*Order`**, porque a UPR muta o receptor no aceite (`*o = next`) e um
  repositório que compartilhasse o ponteiro com o `Store` faria a mutação
  vazar antes do commit, quebrando a prova de atomicidade. `FromSnapshot` no
  domínio é o custo: um construtor puro, sem UPR, sem evento, sem mudança de
  classificação. Alternativa descartada: `Order.Clone()` exportado — expõe
  mecanismo interno do agregado e não resolve a reconstituição a partir do
  que o provider guarda.
- **Ordem `Save` antes de `Enqueue`**, porque `AggregateVersion` é a versão
  gravada e o conflito de versão precisa interromper antes de qualquer
  registro de outbox existir na transação — mesmo que o rollback o apagasse,
  a ordem torna o teste de `UOW-09` legível: sob conflito, `Enqueue` nunca é
  chamado.
- **`go.mod` sem `require` de irmão, resolução pelo `go.work`**, porque a
  verificação empírica mostrou que o workspace resolve o import sem `require`
  e que `go mod tidy` tenta a rede para resolvê-lo — o que, com módulo ainda
  sem tag, falharia ou gravaria pseudo-versão remota divergente da árvore
  local. Consumo fora do workspace (tag `libs/backend/go/dmpf-ports/vX.Y.Z`,
  `require` com versão publicada) é matéria do `KRN-12` (SDK e BOM). O grafo
  do Nx não depende disso: o plugin deriva a aresta entre projetos Go pelos
  `import` dos arquivos, e por isso nenhum `implicitDependencies` é declarado.
- **Regras `port` e `application` no depguard, mas `forbidigo` só em
  `domain`**, porque fora do domínio `errors.New`, `fmt.Errorf` e `panic` são
  legítimos (falha técnica é `error`; panic é `Unexpected`, `ERR-22`), e o
  único símbolo que o depguard não distinguiria — `time.Now` — já cai com o
  package inteiro na `deny`.
- **Escopo L assumido como risco de revisão**: dois módulos, gate local,
  vetores, documentação e ADR em uma história só, como o ticket pede e como a
  `SPEC-XF9TF9A0` fez. Mitigação: commits por projeto Nx (`dmpf-ports-go`,
  `dmpf-domain-go`, `dmpf-application-go`, `workspace`), na ordem portas →
  domínio → aplicação → gate → classificação → documentação, para que cada
  commit compile e a revisão siga a dependência. Alternativa descartada:
  dividir em duas histórias — a segunda metade (`dmpf-application`) não tem
  como ser verificada sem a primeira, e a guarda-chuva fixa o `KRN-04` como um
  item de tamanho L.
- **Validação de forma da entrada (`Quantity <= 0`) fora desta spec**, porque
  FND-03 §4.4 a aloca na "fronteira de aplicação" e RFC §4.1 dá "validar
  entrada" ao bloco `app`: o service recebe mensagem estruturalmente válida
  (`UPR-I01`) e não a revalida. Fica registrado em "Escopo fora" para o
  primeiro adapter.

## Regras relacionadas

- `AGENTS.md` — taxonomia de tags 3D; vedação a redeclarar targets;
  Conventional Commits em PT-BR com scope igual ao projeto Nx
  (`dmpf-ports-go`, `dmpf-application-go`, `dmpf-domain-go`); projetos
  distintos em commits separados; git-flow com `master` e `develop` protegidas
- `.claude/rules/process-enforcement.md` — cadeia de validação Go
- `.claude/rules/git-safety.md` — hooks, revisão de diff, testes nunca
  alterados para passar
- SPEC-YRJRADY9 — guarda-chuva; esta spec fecha o Incremento 2
- SPEC-XF9TF9A0 — sujeito (`example/orders`), forma do desfecho, `Instant`
  local ao exemplo, superfície fechada do raiz
- SPEC-WTAXFV8B — verificador; `include` exato, baseline, `T001`/`T002`,
  política de capabilities por bloco
- SPEC-MQA5HAXF — módulo, tags, cadeia, `bounded_context: dmpf-kernel`,
  `--skipFormat`, ajustes pós-generator

## Verificação e testes

### Critérios de aceite

- [x] `AddItem` e `PlaceOrder` executam os nove passos de FND-04 §3.2 na ordem,
  observável por portas instrumentadas que registram `[authorize, clock, ids,
  within, load, save, enqueue, commit]` nessa sequência; somente o passo 5
  ocorre no bloco `domain` (`sequence_test`).
- [x] Um teste que arma `FailNextCommit(err)` sobre um `Store` vazio prova que,
  após `AddItem` aceito (ramo "cria"), `Reader().Load` devolve `ErrNotFound` e
  `Entries()` está vazio; o caso de uso devolve `(Outcome{}, err)` com
  `errors.Is(err, injetado)`.
- [x] Um teste que arma `FailNextCommit(err)` sobre um `Store` com `P-100` na
  versão 1 prova que, após `PlaceOrder` aceito (ramo "carrega"),
  `Reader().Load` devolve o snapshot anterior com `Status Open` e `Version 1`,
  e `Entries()` está vazio.
- [x] Sob `Rejected` (`orders/item-limit-exceeded`), o repositório e a outbox
  não recebem escrita, `WithinCalls() == 1` (o commit ocorreu), e o chamador
  recebe `Outcome` com `Rejection()` igual à rejeição da UPR e `err == nil`.
- [x] Um segundo `Store`, gravado por `B.Within` de dentro do callback de `A`,
  mantém a escrita quando o callback de `A` devolve erro — a prova de que porta
  não recebida está fora da fronteira de `A` (`UOW-04`).
- [x] Versão divergente (`Save` com `expected` desatualizado) devolve
  `ErrVersionConflict`, `Enqueue` não é chamado, o callback executou
  exatamente uma vez e nenhuma repetição ocorre (`UOW-09`).
- [ ] Nenhuma assinatura exportada de `dmpfports` menciona `database/sql`,
  driver, `time`, `encoding/*`, Protobuf ou tipo do `dmpf-conformance`;
  `dmpfapplication` e `ordersapp` não importam `example/memory`: os vetores
  V16 (c.11), V17 (c.23), c.20 e c.4 reprovam com `DMPF-D001` e o vetor V21
  com `DMPF-E001`.
  - **Parcial.** A primeira metade está plena: `go doc -all` do `dmpfports` não tem
    nenhuma assinatura citando `database/sql`, driver, `time`, `encoding/*`,
    Protobuf ou tipo do `dmpf-conformance` — as duas únicas menções no arquivo
    estão no texto do godoc de package que declara **não** usá-los. A segunda
    metade, dos vetores, é a mesma ressalva do requisito funcional acima: V17,
    c.20 e c.4 caem por `DMPF-E003` (ciclo de imports) em vez de `DMPF-D001`.
- [x] Após `AddItem` aceito, `Entries()[0]` tem `MessageID == "m-000001"`,
  `OccurredAt` igual ao `FixedClock`, `Intent.Destination ==
  "orders.events"`, `Intent.PartitionKey == "P-100"`, `AggregateType ==
  "orders.Order"`, `AggregateID == "P-100"`, `AggregateVersion == 1`,
  `Event.EventName() == "orders.item-added"`; o tipo `OutboxEntry` não tem
  campo de estado de drenagem (`BLK-04`, `BLK-05`).
- [x] `FindOrder` devolve o snapshot sem abrir UoW (`WithinCalls() == 0`) e
  sem gravar outbox (`Entries()` vazio) (`UOW-11`); `Store` tem um único
  recurso e `Within` abre uma única `Tx` (`UOW-01`, `UOW-02`).
- [x] Nenhum artefato declara exactly-once fim a fim: `grep -ri
  "exactly-once" libs/backend/go/dmpf-ports libs/backend/go/dmpf-application`
  encontra apenas a vedação nos `README.md`; os `README.md` declaram
  at-least-once com efeitos idempotentes (P0-3, `GAR-02`).
- [x] `Within` com contexto cancelado devolve `context.Canceled` sem invocar
  `fn` e sem incrementar `WithinCalls()`; panic em `fn` é propagado e a `Tx`
  é descartada (`CTX-21`, `ERR-22`).
- [x] `FromSnapshot(o.Snapshot()).Snapshot()` é `Equal` ao original e mutar o
  resultado não altera o snapshot de origem.
- [x] Manifestos e baseline em commit próprio: o gate com `--base
  origin/develop` sai `0`; movê-los para o commit do código faz o gate emitir
  `DMPF-T002` (provado uma vez, sobre cópia, e registrado no PR).
- [x] Cadeia Go verde nos dois módulos: `fmt-check`, `vet`, `lint`, `build`,
  `test`, `test-race`, `govulncheck`; `bash tools/dmpf-gate-check.sh` verde
  contando três blocos; `pnpm biome ci .` e `pnpm nx affected -t
  lint,typecheck,test,build --exclude=@nx-base-template/source` verdes.

### Cenários de teste

```text
DADO um Store vazio, FixedClock{At: 1755432000000000000}, SequenceIDs{Prefix: "m-"} e ItemLimit 3
QUANDO s.AddItem(ctx, AddItem{Order: "P-100", SKU: "ABC", Quantity: 1}) é executado
ENTÃO err é nil, out.Rejection() é (nil, false), out.Response() é ItemAccepted{Order: "P-100", Items: 1},
      Reader().Load("P-100") devolve snapshot com 1 item e Version 1, e Entries() tem exatamente 1 entrada
      com MessageID "m-000001", OccurredAt 1755432000000000000, Destination "orders.events",
      PartitionKey "P-100", AggregateVersion 1 e Event ItemAdded{At: orders.Instant(1755432000)}

DADO um Store com P-100 aberto, 3 itens, limite 3, versão 3
QUANDO s.AddItem(ctx, AddItem{Order: "P-100", SKU: "XYZ", Quantity: 1}) é executado
ENTÃO err é nil, out.Rejection() devolve rejeição com Code "orders/item-limit-exceeded",
      Reader().Load("P-100") é Equal ao anterior com Version 3, Entries() está vazio
      e WithinCalls() é 1

DADO um Store com P-100 aberto e 1 item, e FailNextCommit(errBoom) armado
QUANDO s.PlaceOrder(ctx, PlaceOrder{Order: "P-100"}) é executado
ENTÃO errors.Is(err, errBoom), out é o valor zero, Reader().Load("P-100").Status permanece Open
      e Entries() está vazio

DADO um Store com P-100 na versão 2 e um bind que embrulha tx.Orders() no Repository instrumentado de
      doubles_test.go, configurado para devolver ErrVersionConflict no Save
QUANDO s.AddItem é executado
ENTÃO errors.Is(err, dmpfports.ErrVersionConflict), o callback executou exatamente 1 vez, Save foi
      chamado exatamente 1 vez, Enqueue nunca foi chamado, Entries() está vazio e P-100 segue na versão 2

DADO dois Stores A e B, e um callback de A que executa B.Within gravando P-300 em B e depois devolve errBoom
QUANDO A.Within(ctx, callback) é executado
ENTÃO errors.Is(err, errBoom), A permanece vazio e B.Reader().Load("P-300") devolve o snapshot gravado:
      porta não recebida pelo callback está fora da fronteira transacional de A

DADO um contexto já cancelado
QUANDO Within(ctx, fn) é executado
ENTÃO o retorno é context.Canceled, fn não foi invocada e WithinCalls() não mudou

DADO um callback que faz panic("x")
QUANDO Within(ctx, fn) é executado dentro de recover no teste
ENTÃO o panic chega ao teste com valor "x" e o Store permanece igual ao anterior

DADO um Store com P-100 e 2 itens
QUANDO s.FindOrder(ctx, "P-100") é executado
ENTÃO o snapshot devolvido tem 2 itens, WithinCalls() é 0 e Entries() está vazio

DADO um Store sem P-200
QUANDO s.PlaceOrder(ctx, PlaceOrder{Order: "P-200"}) é executado
ENTÃO errors.Is(err, dmpfports.ErrNotFound), out é o valor zero e Entries() está vazio

DADO portas instrumentadas que registram cada chamada em uma lista compartilhada
QUANDO s.AddItem é executado com aceite
ENTÃO a lista é exatamente [authorize, clock.Now, ids.NewMessageID, within, orders.Load, orders.Save,
      outbox.Enqueue, commit], nessa ordem

DADO um AuthorizeFunc que devolve errDenied
QUANDO s.AddItem é executado
ENTÃO errors.Is(err, errDenied), clock.Now e ids.NewMessageID não foram chamados e WithinCalls() é 0

DADO uma cópia temporária do workspace com `import _ ".../dmpf-application/example/memory"` em dmpf-application/outcome.go
QUANDO o verificador roda sobre ela
ENTÃO o relatório contém DMPF-D001 com CanonicalKey .../dmpf-application e Target .../example/memory, e o gate sai 1

DADO uma cópia temporária com `import _ "time"` em dmpf-ports/uow.go
QUANDO o verificador roda sobre ela
ENTÃO o relatório contém DMPF-E001 com Target time e Detail "capability io.clock não permitida para o bloco port"

DADO um arquivo temporário zz_gate_*.go sob libs/backend/go/dmpf-application com `import "time"`
QUANDO pnpm nx run dmpf-application-go:lint é executado
ENTÃO o depguard reprova citando io.clock, e tools/dmpf-gate-check.sh registra o vetor como reprovado
```

<critical_constraints>
- [P0] Política de capabilities (RFC §6.2; `capability.go:46-47`): o fechamento
  transitivo de imports das unidades `port` e `application` contém APENAS
  capability `pure` segundo `internal/rule/stdlib.go`. NUNCA importar `time`,
  `math/rand`, `crypto/rand`, `os`, `net/*`, `database/sql`, `encoding/json`,
  `reflect`, `log` (em `port`), nem módulo de terceiro. `context`, `sync`,
  `errors`, `fmt`, `slices`, `maps` são puros e permitidos.
- [P0] Matriz de blocos (RFC §7.3; ADR-010): `port → domain` e `port → port`
  permitidas; `port → application`, `port → provider`, `port → contract`
  PROIBIDAS. `application → domain`, `application → application`,
  `application → port` permitidas; `application → provider` e
  `application → contract` PROIBIDAS. `domain → port` PROIBIDA (ADR-014): o
  `dmpf-domain-go` não importa `dmpf-ports`.
- [P0] Uma transação, um recurso, um callback (`UOW-01`, `UOW-02`, `UOW-09`):
  `Within` abre exatamente uma transação local sobre um recurso e invoca o
  callback exatamente uma vez. NUNCA repetir o callback, sob nenhum erro.
- [P0] Recursos por parâmetro (`UOW-03`, `UOW-04`, `CTX-05`): as portas
  vinculadas à transação chegam ao callback como argumento tipado. NUNCA por
  valor de `context.Context`, variável de package, service locator ou closure
  capturada fora do callback.
- [P0] O ramo decide 6 e 7 (`UOW-05`, `UOW-06`, `UOW-07`): sob `Accepted`,
  persistir com optimistic locking e entregar à outbox na MESMA transação; sob
  `Rejected`, nem repositório nem outbox recebem escrita, o commit OCORRE e o
  chamador recebe a rejeição tipada com `error == nil`.
- [P0] Outbox por porta em tipos de domínio (`BLK-01`, `BLK-03`, ADR-021,
  célula 24): a porta recebe `(DomainEvent, intenção de publicação)`. Nenhuma
  assinatura de `dmpf-ports` menciona driver, tabela, tópico, fila, ARN ou tipo
  de wire.
- [P0] Autoria dos campos (`BLK-04`, `BLK-05`): `message_id`, `occurred_at`,
  `destination`, `partition_key`, `aggregate_type`, `aggregate_id` e
  `aggregate_version` são escritos pelo application service; `destination` é
  lógico; NENHUM campo de estado de drenagem existe no tipo entregue à porta.
- [P0] Identidade antes da transação (§3.2 passo 2, `UOW-09` rationale):
  `occurred_at` e os `message_id` são resolvidos ANTES de `Within` abrir a
  transação, uma vez por execução do caso de uso.
- [P0] Classificação declarada (ADR-012; RFC §3.3, §10.1): cada package de
  produção é unidade com `include` por import path exato, `bounded_context:
  dmpf-kernel`, `public_integration_surface: false`, entrada no baseline;
  manifesto e baseline vêm em COMMIT PRÓPRIO, sem código Go (RFC §10.2).
- [P0] Nenhum artefato declara nem sugere exactly-once fim a fim (P0-3,
  `GAR-01`, V31); a documentação dos módulos declara at-least-once com efeitos
  idempotentes.
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva:
  divergência vira ADR-033, nunca alteração silenciosa da norma.
- [P1] `context.Context` transporta APENAS cancelamento e deadline (`CTX-20`,
  `CTX-21`); o contexto de execução de nove campos (`CTX-01`) e a autorização
  do passo 1 são de FND-07 e ficam fora desta spec.
- [P1] `go.mod` dos dois módulos sem `require` e sem `replace`; `go.sum`
  ausente. A resolução entre módulos irmãos é do `go.work`.
</critical_constraints>

## Escopo fora

- **Provider Postgres, schema da outbox, `FOR UPDATE SKIP LOCKED`, isolamento
  e conflito de serialização real**: `KRN-06`. A realização em memória
  serializa `Within` por mutex e declara no `doc.go` que não prova isolamento.
- **Mapeamento `domain event → integration event`, serialização e wire**: o
  ADR-021 os aloca no provider da outbox; o formato é do `KRN-05`. `OutboxEntry`
  carrega o `DomainEvent` e nada de wire.
- **Vetor de módulo para a célula 12 (`application → contract`)**: exige uma
  unidade `contract` no `develop`, que só existe na branch do `KRN-05`. Fica
  para o `KRN-06`, que coexiste com `dmpf-contracts-go`; até lá a célula é
  coberta pela suíte de 36 células do `KRN-02`.
- **Colisão de numeração de ADR com o `KRN-05`**: a `SPEC-WYX5GW87` (branch
  `feat/ARQ-524-contratos-wire`) reivindica `docs/adr/032-*.md`, já ocupado
  pelo `KRN-03` no `develop`. Esta spec ocupa o `033`; o `KRN-05` renumera ao
  mergear. Não é resolvido aqui.
- **Relay, drenagem e publicação** (`KRN-08`); **inbox, deduplicação e
  disposições de consumo** (`KRN-07`); **retry por conjunção, orçamento e
  telemetria** (`KRN-09`); **transportes** (`KRN-10`). `dmpfports` não declara
  porta de publicação nem de inbox.
- **Autorização do passo 1, contexto de execução de nove campos, taxonomia de
  erros de borda e mapeamento de `Rejection` para protocolo**: FND-07. O
  `AuthorizeFunc` é gancho; `context.Context` transporta só cancelamento.
- **Validação de forma da entrada** (`Quantity <= 0`, `SKU` vazio): bloco `app`
  (RFC §4.1; FND-03 §4.4). O service assume mensagem estruturalmente válida.
- **Test kit exportado da UoW e das portas** (`RunUnitOfWorkContract` como
  package importável, fixtures pareadas Go ↔ TS): `KRN-11` (`dmpf-testkit`).
- **Consumo dos módulos fora do workspace** (`require` com versão publicada,
  tags por módulo, BOM): `KRN-12`. Aqui a resolução é do `go.work`.
- **Reclassificar `time` ou `context` no verificador**: decisão do ADR-031 e
  do `KRN-02`; esta spec se conforma à tabela vigente.
- **Adoção do kernel por bounded contexts de negócio** (C2; `DMPF-D002`):
  pendência registrada no ADR-032 e levada à SPEC-YRJRADY9; as quatro unidades
  novas ficam em `dmpf-kernel` e o problema não as alcança.
- **Alterar `nx.json`, `ci.yml` ou os targets inferidos**: a cadeia e os três
  gates já cobrem projetos `stack:go` afetados; `-count=2 -shuffle=on` fica na
  invocação da suíte, não no target.
- **Registrar a própria rejeição na transação** (FND-04 §3.2 `rationale`):
  admitido pela norma, não exercitado pelo exemplo.
