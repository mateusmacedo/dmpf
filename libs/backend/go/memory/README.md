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
`tools/dmpf-gate-check.sh`. Em módulo próprio, a parte tipada virou `Table`, e
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
| `store.go` | `Store`, `New`; `Table[ID, S]` com `Reader(*Store)` e `Repository(*Tx)`; inspeção para testes: `Entries`, `FailNextCommit`, `WithinCalls`, `Commits`, `InboxRows`, `InboxStatus`, `InboxLastError` |
| `tx.go` | `Tx` (`Inbox(consumer)`, `Outbox()`), `NewUnitOfWork[R](store, bind)` e `Within` |
| `inbox.go` | A inbox transacional: `Register` com as classificações R1–R4 e `Pending.Complete` |
| `clock.go` | `FixedClock` (`ports.Clock`) e `SequenceIDs` (`ports.IDGenerator`) |
| `errors.go` | `ErrInboxConsumerRequired`, `ErrInboxConsumerMismatch`, `ErrAlreadyCompleted`, `ErrInvalidCompletion` |

O código de produção importa só a stdlib e o `ports`; `external` é `[]`. O
`testkit` entra apenas pelos `_test.go` — o `providerkit` roda aqui a suíte de
conformidade de UoW e de inbox.

## `Table`: uma tabela lógica por agregado

```go
type Table[ID comparable, S any] struct {
    Name  string
    Clone func(S) S
}

func (t Table[ID, S]) Reader(s *Store) ports.Reader[ID, S]
func (t Table[ID, S]) Repository(tx *Tx) ports.Repository[ID, S]
```

`Name` é a tabela lógica — o equivalente da tabela SQL do `postgres` — e liga
um único par `(ID, S)` pela vida do `Store`: reusar o mesmo `Name` com outro `S`
entra em `panic` na asserção de tipo do `Load`. `Clone` é chamado em toda
travessia da fronteira (`Load`, `Save`, abertura e commit da transação); `nil`
significa que a cópia por valor basta — um `S` sem slice, map ou ponteiro.

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

O `bind` que monta o `Resources` do caso de uso é de quem compõe — aqui, o
próprio arquivo de teste —, porque `provider → application` é célula proibida
e esta realização não pode conhecer o tipo de recursos de nenhum contexto
(`UOW-03`, ADR-034). Para semear estado antes do cenário, o mesmo `bind` serve:
`memory.NewUnitOfWork(store, func(tx *memory.Tx) *memory.Tx { return tx })` e
`ordersTable.Repository(tx).Save(...)` dentro do `Within`.

## O que prova, e o que não prova

Prova, em `store_test.go`: uma transação por `Within` sobre um recurso, o
callback invocado exatamente uma vez, o commit aplicando estado de negócio e
outbox juntos, o rollback por erro e por panic, uma falha de commit injetável e
consumida uma única vez (`FailNextCommit`), o contexto cancelado — antes de abrir
e durante a espera por `txMu` — que nunca abre transação, snapshots que não
compartilham array com o `Store` nem com o chamador, o conflito de versão
(`ErrVersionConflict`), `ErrNotFound`, tabelas de nomes distintos que não
compartilham linhas, e a serialização de `Within` concorrentes.

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
go run ./tools/dmpf-conformance/cmd/conformance --root . --base origin/develop
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
`--write-baseline`. Essa mudança vai em commit separado do código (RFC §10.2);
misturar os dois reprova no CI com `DMPF-T002`. Tirar a unidade de
`shared_kernel_units` é ato de classificação com o mesmo rito.

## Referências

- `docs/adr/046-libs-somente-kernel-de-reuso.md` — por que é módulo próprio e
  por que a API é `Table`
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira que realiza
- `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md` — a inbox
- `docs/dmpf/uow-inbox-outbox.md` — FND-04: UoW, sequência canônica, outbox,
  inbox
- `libs/backend/go/ports/README.md` — as portas em tipos de domínio e as seis
  cláusulas de `Within`
- `libs/backend/go/postgres/README.md` — a realização que prova o isolamento
