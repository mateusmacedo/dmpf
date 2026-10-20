# memory

Realização em memória das portas transacionais do kernel DMPF — `UnitOfWork[R]`,
`Repository[ID, S]`, `Outbox` e `Inbox` — e das duas portas de identidade,
`Clock` e `IDGenerator`. É o bloco `provider` que fecha um caso de uso ponta a
ponta sem banco: as suítes dos blocos `application` do workspace rodam sobre
ele, e o `postgres` é a contraparte que prova as mesmas portas contra um banco
de verdade (`KRN-06`).

Nasceu como `application/example/memory` (ADR-034) e virou módulo próprio pelo
ADR-046: a realização era genérica, mas expunha `tx.Orders()` e
`tx.Reservations()` tipados pelos agregados de exemplo, e vivia dentro do módulo
`application` — sob o glob da regra `application` do `depguard`, precisava de
uma exclusão no `.golangci.yml` e de uma entrada em `FORA_DO_DEPGUARD` do
`tools/dmpf-plugin/scripts/dmpf-gate-check.sh`. Em módulo próprio, a parte tipada virou `Table`, e
as duas exceções deixaram de existir.

Projeto Nx `memory`, tags `type:lib`, `scope:backend`, `stack:go` e
`layer:providers`. Unidade `kernel/provider-memory`, `block: provider`,
`bounded_context: kernel`, designada shared kernel em `shared_kernel_units` do
baseline — é o que permite a um contexto de `apps/backend` importá-la sem
`DMPF-D002`. Import path do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/memory`.

## O que o módulo contém

| Arquivo | Conteúdo |
| --- | --- |
| `store.go` | `Store`, `New`; `Table[ID, S]` com `Reader(*Store)` e `Repository(*Tx)`, escopados por tenant; `Unique[S]`, a unicidade declarada (ADR-066); inspeção para testes: `Entries`, `FailNextCommit`, `WithinCalls`, `Commits`, `InboxRows`, `InboxStatus`, `InboxLastError`, `AttemptConflicts` |
| `tx.go` | `Tx` (`Inbox(consumer)`, `Outbox()`), `NewUnitOfWork[R](store, bind)` e `Within` |
| `attempt_ledger.go` | `Tx.AttemptLedger()`, o ledger de tentativas do comando externo na transação aberta, e `AttemptStore`/`NewAttemptStore(store)`, que roda cada chamada numa transação própria; mesma semântica do `postgres`, inclusive `Started`/`Pending`/`Done` (ADR-070) |
| `inbox.go` | A inbox transacional: `Register` com as classificações R1–R4 e `Pending.Complete`; `Tx.CommandInbox(consumer)`, a mesma inbox para comandos, com o tenant na chave, o desfecho gravado e o vencimento da entrada (FND-04 §7.6) |
| `platform.go` | `Table.PlatformReader(*Store, ports.AuditTrail, ...PlatformOption)`, `Table.Projector(*Tx, ports.ProjectionPolicy, ports.AuditTrail)` e `WithPageMax`: a paridade do acesso entre tenants declarado do `postgres` (ADR-068) |
| `clock.go` | `FixedClock` (`ports.Clock`) e `SequenceIDs` (`ports.IDGenerator`) |
| `errors.go` | `ErrTenantUnresolved`, `ErrInboxConsumerRequired`, `ErrInboxConsumerMismatch`, `ErrAlreadyCompleted`, `ErrInvalidCompletion`, `ErrCommandExpiryRequired`, `ErrIncompleteAttemptStore` |

O código de produção importa só a stdlib, o `ports` e o `domain` (o
`*domain.Rejection` de `Unique`); a trilha do acesso entre tenants chega como
`ports.AuditTrail`, e nenhum arquivo, nem de teste, importa `observability`,
cujos testes importam o `memory` (ADR-068). `external` é `[]`. O
`testkit` entra apenas pelos `_test.go` — o `providerkit` roda aqui a suíte de
conformidade de UoW e de inbox.

## `Table`: uma tabela lógica por agregado, escopada por tenant

```go
type Table[ID comparable, S any] struct {
    Name   string
    Clone  func(S) S
    Unique []Unique[S]
}

type Unique[S any] struct {
    Name      string
    Key       func(S) ([]string, bool)
    Rejection *domain.Rejection
}

func (t Table[ID, S]) Reader(s *Store) ports.Reader[ID, S]
func (t Table[ID, S]) Repository(tx *Tx) ports.Repository[ID, S]
```

`Name` é a tabela lógica — o equivalente da tabela SQL do `postgres` — e liga
um único par `(ID, S)` pela vida do `Store`: reusar o mesmo `Name` com outro `S`
entra em `panic` na asserção de tipo do `Load`. `Clone` é chamado em toda
travessia da fronteira (`Load`, `Save`, abertura e commit da transação); `nil`
significa que a cópia por valor basta — um `S` sem slice, map ou ponteiro.

`Unique` espelha o `postgres.Table.Unique` nos dublês (ADR-066). `Name` é o
nome da restrição, e `Key` devolve as colunas depois de `tenant_id`, comparadas
uma a uma, com `false` quando uma é nula, o que não colide com nada; a cadeia
vazia colide, como no PostgreSQL. A mesma identidade não é violação. Com
colisão em mais de uma restrição, vale a primeira declarada, por isso `Unique`
segue a ordem do DDL, como o `postgres.Unique`. O `Save`
que ocupa o valor de outra linha do tenant devolve `ports.UniqueViolation`, com
a rejeição declarada se for a primeira escrita da transação e `Rejection` nulo
se já houve `Save` ou `Enqueue` antes; a inbox não conta. `Repository` entra em
`panic` com declaração sem nome, sem `Key`, sem rejeição, com código inválido
ou com nome repetido, como o `postgres.Table`.

Todo `Load` e `Save` resolve o tenant do contexto de execução
(`ports.RequireExecutionContext(ctx).Tenant()`), pelo mesmo contrato que o
`postgres.Table` realiza (`IDN-14`, ADR-051): a chave interna, `rowKey`, pareia
o tenant com o identificador, então dois tenants com o mesmo identificador são
duas linhas e nenhum alcança a da outra. Ausência de tenant recusa —
`ErrTenantUnresolved`, nunca um alargamento da busca (`IDN-15`) — e nenhum
tenant é inventado para preenchê-la (`IDN-20`). Uma leitura que não encontra a
linha, mas encontra o identificador sob outro tenant, devolve
`ports.CrossTenantAccess` em vez de `ErrNotFound` puro, espelhando o que
`postgres.Table` relata; ver `libs/backend/go/ports/README.md`.

A API recebe o clone em vez de expor funções livres por dois motivos: a
garantia de snapshots sem backing array compartilhado exige um clone que o
módulo não consegue derivar de um `S` opaco; e, com `Reader` e `Repository`
avulsos, o consumidor repetiria nome e clone em cada chamada, e um nome
divergente entre os dois faria o reader não ver as linhas em silêncio. Um
descritor único por agregado fecha as duas portas sob o mesmo nome.

O padrão de uso é o dos contextos de referência — definido uma vez por package
de teste, como em `apps/backend/orders/application/doubles_test.go`:

```go
var ordersTable = memory.Table[domain.OrderID, domain.Snapshot]{
    Name:  "orders",
    Clone: func(s domain.Snapshot) domain.Snapshot { s.Items = slices.Clone(s.Items); return s },
}

store := memory.New()
bind := func(tx *memory.Tx) application.Resources {
    return application.Resources{Orders: ordersTable.Repository(tx), Outbox: tx.Outbox()}
}
service := application.Service{
    UoW:    memory.NewUnitOfWork(store, bind),
    Reader: ordersTable.Reader(store),
    Clock:  memory.FixedClock{At: occurred},
    IDs:    &memory.SequenceIDs{Prefix: "m-"},
}
```

O `ctx` passado a `Load`/`Save` precisa trazer um `ports.ExecutionContext` com
tenant resolvido — em teste, via `ports.WithExecutionContext` sobre um
contexto de execução construído com `ports.NewExecutionContext`. O `bind` que
monta o `Resources` do caso de uso é de quem compõe — aqui, o próprio arquivo
de teste —, porque `provider → application` é célula proibida e esta
realização não pode conhecer o tipo de recursos de nenhum contexto (`UOW-03`,
ADR-034). Para semear estado antes do cenário, o mesmo `bind` serve:
`memory.NewUnitOfWork(store, func(tx *memory.Tx) *memory.Tx { return tx })` e
`ordersTable.Repository(tx).Save(...)` dentro do `Within`.

## Acesso entre tenants: paridade com o `postgres`

`Table.PlatformReader` e `Table.Projector` têm a porta, as recusas e a trilha
de `postgres.Table.PlatformReader` e `postgres.Table.Projector`, na mesma ordem,
para que os dois respondam igual à chamada sem alcance declarado ou sem
contexto de execução; o contrato está em `libs/backend/go/postgres/README.md`, e
a decisão, no ADR-068. Construir qualquer dos dois sem `ports.AuditTrail` entra em
`panic` com o nome da tabela, e `WithPageMax` com valor menor que 1 também.

O que muda é só a realização:

- O `PlatformReader` lê o estado **commitado** do `Store`, como o `Reader`, e o
  `List` ordena por `(tenant, fmt.Sprintf("%#v", id))` em ordem de bytes: a
  forma Go do valor distingue identificadores distintos mesmo quando o `String()`
  deles coincide. Dois IDs distintos com a mesma forma `%#v` — um `GoString()`
  que colida ou ponteiros para valores iguais — empatam na chave e podem ser
  omitidos entre páginas: o dublê pressupõe IDs de forma distinta, como as strings
  do kit. Não é a ordem do keyset
  `(tenant_id, id)` do `postgres`, que segue a collation do banco: cada
  realização tem a sua ordem total estável, e o kit só afirma isso. O cursor
  `after` retoma estritamente depois da chave.
- O `Projector` lê e grava no estado da `Tx`, dentro do `Within`; o `Save` é o
  mesmo do `Repository` — versão e unicidade declarada — com a chave do destino,
  e o `Load` não sonda o dono: a linha ausente é `ports.ErrNotFound` puro.

## O que prova, e o que não prova

Prova, em `store_test.go`: uma transação por `Within` sobre um recurso, o
callback invocado exatamente uma vez, o commit aplicando estado de negócio e
outbox juntos, o rollback por erro e por panic, uma falha de commit injetável e
consumida uma única vez (`FailNextCommit`), o contexto cancelado — antes de abrir
e durante a espera por `txMu` — que nunca abre transação, snapshots que não
compartilham array com o `Store` nem com o chamador, o conflito de versão
(`ErrVersionConflict`), `ErrNotFound`, tabelas de nomes distintos que não
compartilham linhas, a serialização de `Within` concorrentes, e — em
`execution_test.go` — o escopo por tenant: duas linhas para o mesmo
identificador sob tenants distintos, `ErrTenantUnresolved` na ausência e
`ports.CrossTenantAccess` no cruzamento (mesmo contrato do `postgres`). A
unicidade declarada é provada em `store_test.go` e pelo `providerkit.Unique`
(`testkit/providerkit/unique_test.go`), com a cláusula concorrente em `Skipped`:
a serialização por `txMu` não deixa duas criações correrem juntas. Em
`platform_test.go`, o acesso entre tenants: a ordem e a retomada do `List`, inclusive
com identificadores de `String()` igual, um
evento `accepted` por tenant antes de devolver, página vazia sem evento, as
recusas com e sem evento, a falha fechada quando o `Sink` recusa e a leitura do
`Load` da projeção sem sonda de dono. As suítes `providerkit.Platform` e
`providerkit.Projection` rodam sobre este módulo em
`testkit/providerkit/platform_test.go` e `projection_test.go`.
Em `attempt_ledger_test.go`, o ledger de tentativas: as três classificações de
`Begin`, a serialização de `Begin` concorrente na mesma chave (uma só tentativa
`begun`), `Finish` e `Observe` com registro, no-op, duplicata e conflito, `Stale`
ordenado e limitado entre tenants, e o isolamento por tenant.

Não prova **isolamento** nem conflito de serialização entre transações
concorrentes: `Within` retém `txMu` durante todo o callback, então as transações
são serializadas em vez de isoladas, e `Load` e `Save` da mesma `Tx` sempre veem
o mesmo estado — um conflito de versão tem de ser injetado pelo chamador.
Isolamento real é do `postgres`.

O `Store` guarda **dois** mutexes para que essa serialização não vire armadilha:
`txMu` é o que faz uma transação excluir a outra, e `dataMu` protege o estado,
adquirido por operação. Assim `Table.Reader(store)` chamado de dentro do callback
lê o estado já commitado — o que um leitor fora da transação veria — em vez de
travar; uma porta que escapa do callback não alcança o `Store`.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test-race,govulncheck -p memory
go run ./tools/dmpf-conformance/cmd/conformance --root .
```

O `test-race` roda `go test -race ./...` sem `-tags=integration` e com cache:
nenhuma suíte deste módulo exige infraestrutura. O `depguard` não o seleciona —
os globs são `domain/`, `ports/`, `application/` e `contracts/` — porque um
provider tem a capability irrestrita (`capability.go:51`); o gate autoritativo
é o verificador.

O `go.mod` declara os `require` dos módulos irmãos, sem `replace`: a resolução é
do `go.work`, que o `modsync` (`go run ./tools/dmpf-conformance/cmd/modsync`)
mantém sincronizado. O `require` do `testkit` existe só pelos `_test.go`; o
ciclo `memory ↔ testkit` é o mesmo que `application ↔ testkit` já tinha, e o
`go.work` o resolve.

## Governança

Criar um package novo aqui é criar uma unidade DMPF: ele precisa de entrada
própria no `dmpf-units.json` (`include` por import path exato) e o baseline em
`tools/dmpf-baseline/units-baseline.json` precisa ser regravado com
`--write-baseline`; sem isso, o CI reprova com `DMPF-T001`. Tirar a unidade de
`shared_kernel_units` também passa pelo baseline.

## Referências

- `docs/adr/046-libs-somente-kernel-de-reuso.md` — por que é módulo próprio e
  por que a API é `Table`
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira que realiza
- `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md` — a inbox
- `docs/adr/051-escopo-de-tenant-por-choke-point-em-go.md` — o mesmo contrato
  de escopo que `postgres.Table` realiza
- `docs/adr/068-acesso-entre-tenants-declarado.md` — leitura de plataforma e
  projeção, que este módulo realiza em paridade com o `postgres`
- `docs/dmpf/uow-inbox-outbox.md` — FND-04: UoW, sequência canônica, outbox,
  inbox
- `libs/backend/go/ports/README.md` — as portas em tipos de domínio, o
  contexto de execução e as seis cláusulas de `Within`
- `libs/backend/go/postgres/README.md` — a realização que prova o isolamento
