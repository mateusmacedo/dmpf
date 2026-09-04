# dmpf-application-go

Bloco `application` do kernel DMPF em Go: o lado do chamador da UPR. Realiza o
desfecho que separa o canal de negócio do canal técnico, a resolução de
identidade anterior à transação, o gancho de autorização do passo 1 e um caso de
uso de escrita que percorre os nove passos da sequência canônica do FND-04
(`docs/dmpf/uow-inbox-outbox.md` §3.2) sobre o agregado de exemplo do
`dmpf-domain-go`.

Traz também, como unidade `provider`, uma realização em memória da Unit of Work
e das portas — o que fecha o caso de uso ponta a ponta sem banco. O `KRN-06` a
substitui pelo Postgres.

Projeto Nx `dmpf-application-go`, tags `type:lib`, `scope:backend`, `stack:go`.
Import path do módulo:
`gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application`.

## O que o módulo contém

| Package | Unidade DMPF | Bloco | Conteúdo |
| --- | --- | --- | --- |
| `dmpfapplication` (raiz) | `dmpf-kernel/application` | `application` | `Outcome[R]`, `Accepted`, `Rejected`; `Identity`, `ResolveIdentity`; `AuthorizeFunc`, `AllowAll` |
| `example/orders` | `dmpf-kernel/example-orders-application` | `application` | `Service`, `Resources`, `Command`, `AddItem`, `PlaceOrder`, `FindOrder`, `AggregateType`, `Destination` |
| `example/memory` | `dmpf-kernel/example-memory` | `provider` | `Store`, `New`, `Tx`, `NewUnitOfWork`, `Reader`, `Entries`, `FailNextCommit`, `WithinCalls`, `FixedClock`, `SequenceIDs` |

Três unidades, todas com `bounded_context: dmpf-kernel`. Cada package é uma
unidade porque, em Go, a unidade de verificação é o package (RFC §3.3), e a do
`example/memory` é `provider` — não `application` — porque ela **realiza** as
portas em vez de orquestrá-las.

## O desfecho: `Outcome` e `error` são canais distintos

```go
func (s Service) AddItem(ctx context.Context, cmd AddItem) (dmpfapplication.Outcome[orders.ItemAccepted], error)
```

`error` transporta **apenas** falha técnica: conflito de versão, erro de commit,
cancelamento, falha de porta. A recusa de negócio viaja no `Outcome`, com
`error == nil`. Essa separação existe porque, no serviço de aplicação, o canal
de erro do Go já está ocupado pela falha técnica — e o ADR-018 só admite o par
`(valor, error)` quando o canal de erro carrega exclusivamente rejeições de
domínio, o que é verdade na UPR e deixa de ser aqui.

```go
out, err := service.AddItem(ctx, ordersapp.AddItem{Order: "P-100", SKU: "ABC", Quantity: 1})
if err != nil {
    return err // falha técnica: conflito, commit, cancelamento
}
if rej, refused := out.Rejection(); refused {
    return present(rej) // recusa de negócio, com o commit já ocorrido
}
resp := out.Response() // orders.ItemAccepted
```

`Rejected(nil)` provoca panic: ausência de rejeição não é um terceiro desfecho
(`DEC-01`), e um `Outcome` nesse estado se leria como aceito.

## Os nove passos, e onde cada um está no código

A sequência do FND-04 §3.2 não é comentada por número no código: ela é legível
nas próprias chamadas. O mapa está aqui.

| Passo | Onde | O que acontece |
| --- | --- | --- |
| 1. autorizar | `add_item.go`, `s.Authorize(ctx, cmd)` | Erro interrompe antes de qualquer resolução ou transação |
| 2. resolver identidade | `add_item.go`, `dmpfapplication.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)` | Uma leitura de relógio e um identificador por evento possível, **antes** de `Within` |
| 3. abrir a UoW | `add_item.go`, `s.UoW.Within(ctx, func(...) error {` | Uma transação sobre um recurso |
| 4. carregar | `add_item.go`, `s.loadOrCreate(...)` → `res.Orders.Load` | `ErrNotFound` vira `orders.NewOrder` com `expected == 0` |
| 5. decidir | `add_item.go`, `order.AddItem(...)` | O único passo que ocorre no bloco `domain` |
| 6. persistir | `add_item.go`, `res.Orders.Save(ctx, cmd.Order, order.Snapshot(), stored)` | Optimistic locking; grava como `stored + 1` |
| 7. enfileirar | `service.go`, `enqueueAll(...)` → `res.Outbox.Enqueue` | Mesma transação do passo 6 |
| 8. commitar | `add_item.go`, o retorno `nil` do callback | O commit é do `Within`, não do caso de uso |
| 9. responder | `add_item.go`, `return outcome, nil` | `Outcome` no caminho de negócio, `error` no técnico |

`PlaceOrder` percorre os mesmos passos, com uma diferença no 4: só carrega. Um
agregado ausente volta como `error` técnico embrulhado, não como rejeição —
nenhuma UPR a produziu, e a categoria de borda é do FND-07.

Três detalhes que a norma fixa e o código realiza:

- **Sob recusa, o callback devolve `nil` de propósito.** A transação commita sem
  efeito, e o chamador recebe `(Rejected(rej), nil)`. Abortar tornaria a recusa
  indistinguível de falha técnica, o que `DEC-04` proíbe (`UOW-05`, `UOW-06`).
- **`Save` antes de `Enqueue`.** `AggregateVersion` é a versão efetivamente
  gravada, e o conflito de versão precisa interromper antes de existir qualquer
  registro de outbox na transação. Sob conflito, `Enqueue` nunca é chamado.
- **Nenhum retry.** `ErrVersionConflict` sobe como `error`, sem repetição
  (`UOW-09`, `UOW-10`). A política de retry, quando existir, é do caso de uso e
  do `KRN-09`.

`FindOrder` usa `s.Reader`, nunca `s.UoW`, e não toca a outbox: uma consulta não
abre transação (`UOW-11`).

## A autoria dos campos da outbox

O serviço de aplicação escreve os sete campos de `dmpfports.OutboxEntry`
(FND-04 §2.3, `BLK-04`, `BLK-05`): identidade e tempo (`MessageID`,
`OccurredAt`), roteamento (`Intent.Destination`, `Intent.PartitionKey`) e origem
de negócio (`AggregateType`, `AggregateID`, `AggregateVersion`), mais o evento.
Neste exemplo `Destination` é `orders.events` — um nome **lógico** de fluxo, não
um tópico — e `PartitionKey` é o identificador do pedido, para que fatos do
mesmo agregado preservem ordem.

Campos de wire e de estado de drenagem não existem no tipo entregue à porta: os
primeiros são do provider e do contrato, os segundos do schema e do relay.

## Garantias de entrega

A outbox entrega **at-least-once com efeitos idempotentes** (`GAR-02`). Entrega
exatamente-uma-vez fim a fim (*exactly-once*) é **vedada** como promessa em
qualquer artefato do DMPF (P0-3, `GAR-01`): não é realizável sobre transporte e
consumidor independentes, e quem depende dela está descrevendo idempotência no
consumidor com outro nome.

## O que a realização em memória prova, e o que não prova

`example/memory` prova: uma transação por `Within` sobre um recurso, o callback
invocado exatamente uma vez, o commit aplicando estado de negócio e outbox
juntos, o rollback por erro e por panic, uma falha de commit injetável
(`FailNextCommit`), o contexto cancelado que nunca abre transação, e snapshots
que não compartilham array com o `Store`.

Não prova **isolamento** nem conflito de serialização entre transações
concorrentes: `Within` retém `txMu` durante todo o callback, então as transações
são serializadas, e `Load` e `Save` da mesma `Tx` sempre veem o mesmo estado. Um
conflito de versão tem de ser injetado pelo chamador — é o que
`doubles_test.go` faz, embrulhando `tx.Orders()` em um repositório instrumentado.
Isolamento real é do `KRN-06`.

O `Store` guarda **dois** mutexes para que essa serialização não vire armadilha:
`txMu` é o que faz uma transação excluir a outra, e `dataMu` protege o estado,
adquirido por operação. Assim `Store.Reader()` chamado de dentro do callback lê
o estado já commitado — o que um leitor fora da transação veria — em vez de
travar. Com um mutex único fazendo os dois papéis, essa leitura seria deadlock;
`store_test.go` tem um teste que fixa o comportamento.

O `bind` que liga a transação ao tipo de recursos do caso de uso é escrito pelo
composition root — nos testes, o próprio arquivo de teste — porque
`provider → application` é célula proibida e a realização não pode conhecer
`ordersapp.Resources`.

## O que o módulo não contém

Provider Postgres, schema de outbox e isolamento real (`KRN-06`, entregue em
`dmpf-provider-postgres`); claim, lease, `SKIP LOCKED`, relay e publicação
(`KRN-08`); mapeamento para evento de integração, serialização e wire
(`KRN-05`); inbox e deduplicação (`KRN-07`); retry por conjunção,
orçamento e telemetria (`KRN-09`); transportes (`KRN-10`). A autorização real, o
contexto de execução de nove campos e a taxonomia de erros de borda são do
FND-07 — `AuthorizeFunc` é só o gancho que eles preencherão. Validar a forma da
entrada (`Quantity <= 0`, `SKU` vazio) é do bloco `app` (RFC §4.1).

Os imports de produção são `context`, `errors`, `fmt`, `slices`, `sync`, o
`dmpf-domain` e o `dmpf-ports` — todos capability `pure`. Sem `time`, sem
`rand`, sem `os`, sem `net`, sem `database/sql`, sem `encoding/*` e sem
terceiro. O bloco `application` poderia usar `log`/`log/slog`, que a política
lhe permite (`capability.go:47`), mas esta entrega não usa.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test,test-race,govulncheck -p dmpf-application-go
go -C libs/backend/go/dmpf-application test -race -count=2 -shuffle=on ./...
bash tools/dmpf-gate-check.sh
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --base origin/develop
```

O `lint` aplica ao bloco a regra `application` do `.golangci.yml`, com
`list-mode: strict`. O `files` dessa regra seleciona por caminho
(`**/*-application/**`) e por isso alcançaria `example/memory`, que é `provider`
e cuja capability o verificador deixa irrestrita (`capability.go:51`) — quem
realiza a porta precisa do I/O que os blocos de cima não podem ter. O
subpackage é portanto excluído do `depguard` em `linters.exclusions.rules`,
alinhando o gate local ao autoritativo em vez de manter uma restrição que o
provider teria de burlar ao trocar memória por banco. O `gate-check.sh` declara
essa unidade como fora do gate local, em vez de exercitá-la.

O `go.mod` não tem `require` nem `replace`, e não há `go.sum`: a resolução dos
módulos irmãos é do `go.work`. Por isso o target `tidy`, que o plugin do Nx
infere, **não** faz parte da cadeia.

## Governança

Criar um package novo aqui é criar uma unidade DMPF: ele precisa de entrada
própria no `dmpf-units.json` (`include` por import path exato) e o baseline em
`tools/dmpf-baseline/units-baseline.json` precisa ser regravado com
`--write-baseline`. Essa mudança vai em commit separado do código (RFC §10.2);
misturar os dois reprova no CI com `DMPF-T002`.

## Referências

- `docs/specs/SPEC-ZHE7DN1H-dmpf-kernel-aplicacao-go.md` — spec do `KRN-04`
- `docs/adr/034-fronteira-de-uow-em-go.md` — decisões desta realização
- `docs/adr/018-forma-do-desfecho-da-upr.md` — por que `Outcome` e não `(R, error)`
- `docs/adr/021-mapeamento-no-provider-serializacao-na-escrita.md` — a outbox recebe evento de domínio
- `docs/dmpf/uow-inbox-outbox.md` — FND-04: UoW, sequência canônica, outbox
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §3.3, §4.1, §6.2, §7.3, §10.2
