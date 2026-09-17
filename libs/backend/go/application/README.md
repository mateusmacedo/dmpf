# application

Bloco `application` do kernel DMPF em Go: o lado do chamador da UPR. Realiza o
desfecho que separa o canal de negócio do canal técnico, a resolução de
identidade anterior à transação, o gancho de autorização do passo 1 e a
taxonomia mínima do consumo. O caso de uso de escrita que percorre os nove
passos da sequência canônica do FND-04 (`docs/dmpf/uow-inbox-outbox.md` §3.2) e
o caso de uso de consumo são hoje os blocos `application` dos contextos de
referência, `apps/backend/orders/application` e
`apps/backend/reservations/application` (ADR-046); este README continua
descrevendo-os porque são o molde de quem escreve um contexto.

A realização em memória da Unit of Work e das portas — o que fecha um caso de
uso ponta a ponta sem banco — é módulo próprio, `memory`
(`libs/backend/go/memory`, unidade `kernel/provider-memory`). O `postgres` a
substitui por um banco de verdade (`KRN-06`).

Projeto Nx `application`, tags `type:lib`, `scope:backend`, `stack:go`.
Import path do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/application`.

## O que o módulo contém

| Package | Unidade DMPF | Bloco | Conteúdo |
| --- | --- | --- | --- |
| `application` (raiz) | `kernel/application` | `application` | `Outcome[R]`, `Accepted`, `Rejected`; `Identity`, `ResolveIdentity`; `AuthorizeFunc`, `AllowAll`; `Disposition` (as sete de FND-04 §6.4), `Category`, `Failure`, `Classify` |

Uma unidade só, com `bounded_context: kernel`; em Go, a unidade de verificação
é o package (RFC §3.3). Os packages que este módulo carregava como
`example/` viraram unidades dos próprios contextos:

| Antes (ADR-044) | Hoje (ADR-046) | Unidade | Bloco |
| --- | --- | --- | --- |
| `example/orders` | `apps/backend/orders/application` — `Service`, `Resources`, `Command`, `AddItem`, `PlaceOrder`, `FindOrder`, `AggregateType`, `Destination`; o lado da **escrita** | `orders/application` | `application` |
| `example/reservations` | `apps/backend/reservations/application` — `Service`, `Resources` (com `Inbox`), `ConsumeOrderPlaced`, `Consume`; o lado do **consumo** (`KRN-07`) | `reservations/application` | `application` |
| `example/memory` | `libs/backend/go/memory` — `Store`, `New`, `Tx`, `NewUnitOfWork`, `Table[ID, S]`, `Entries`, `FailNextCommit`, `WithinCalls`, `FixedClock`, `SequenceIDs`; `Tx.Inbox`, `Tx.Outbox` | `kernel/provider-memory` | `provider` |

A realização em memória é `provider` — não `application` — porque ela
**realiza** as portas em vez de orquestrá-las; por isso ganhou módulo próprio,
fora do alcance da regra `application` do `depguard`.

## O consumo: as sete disposições

O `Consume` do contexto `reservations`
(`apps/backend/reservations/application/consume.go`) percorre o lado do service da sequência de FND-04
§6.3: abre a UoW, chama `Inbox.Register(Receipt)` e ramifica **uma única vez**,
no `Reception.Match` (`INB-11`). Sob R1 decide entre aplicar (`Save` + outbox
derivada + `Complete(processed)`), rejeitar por negócio (`Complete(rejected)`
com o código da rejeição em `last_error`) ou devolver o erro técnico, que
desfaz a transação; sob R2, R3 e R4 curto-circuita sem escrever nada — e R3
**não** reemite o rejection event (`INB-12`). Fora do `Within`, um erro é
classificado por `Classify` em R1×D3 ou R1×D4: `Failure` pela retryability já
resolvida (`MAP-07`), `ErrRegisterTimeout` como transitório (`INB-17`),
`context.DeadlineExceeded`/`Canceled` e qualquer erro sem categoria como
terminal (`CTX-23`, `ERR-11`, `ERR-24`). O único predicado declarado aqui é o do
`Conflict`: `ErrVersionConflict` no consumo é retentável porque a reexecução
relê o estado antes de decidir.

O efeito de broker **não** acontece neste bloco: quem confirma, libera ou contém
é o adapter em `app`, sempre depois do retorno da transação (`INB-08`). A
taxonomia de erros é a de FND-07 §5.3, consumida — este módulo não cria
categoria (`ERR-08`).

## O desfecho: `Outcome` e `error` são canais distintos

No caso de uso de referência (`apps/backend/orders/application`, package
`application` do contexto, que importa este módulo homônimo sem alias; quem
importa os dois recebe o kernel como `usecase`, ADR-045):

```go
func (s Service) AddItem(ctx context.Context, cmd AddItem) (application.Outcome[domain.ItemAccepted], error)
```

`error` transporta **apenas** falha técnica: conflito de versão, erro de commit,
cancelamento, falha de porta. A recusa de negócio viaja no `Outcome`, com
`error == nil`. Essa separação existe porque, no serviço de aplicação, o canal
de erro do Go já está ocupado pela falha técnica — e o ADR-018 só admite o par
`(valor, error)` quando o canal de erro carrega exclusivamente rejeições de
domínio, o que é verdade na UPR e deixa de ser aqui.

```go
out, err := service.AddItem(ctx, application.AddItem{Order: "P-100", SKU: "ABC", Quantity: 1})
if err != nil {
    return err // falha técnica: conflito, commit, cancelamento
}
if rej, refused := out.Rejection(); refused {
    return present(rej) // recusa de negócio, com o commit já ocorrido
}
resp := out.Response() // domain.ItemAccepted
```

`Rejected(nil)` provoca panic: ausência de rejeição não é um terceiro desfecho
(`DEC-01`), e um `Outcome` nesse estado se leria como aceito.

## Os nove passos, e onde cada um está no código

A sequência do FND-04 §3.2 não é comentada por número no código: ela é legível
nas próprias chamadas. O mapa está aqui; os arquivos são os de
`apps/backend/orders/application`.

| Passo | Onde | O que acontece |
| --- | --- | --- |
| 1. autorizar | `add_item.go`, `s.Authorize(ctx, cmd)` | Erro interrompe antes de qualquer resolução ou transação |
| 2. resolver identidade | `add_item.go`, `application.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)` | Uma leitura de relógio e um identificador por evento possível, **antes** de `Within` |
| 3. abrir a UoW | `add_item.go`, `s.UoW.Within(ctx, func(...) error {` | Uma transação sobre um recurso |
| 4. carregar | `add_item.go`, `s.loadOrCreate(...)` → `res.Orders.Load` | `ErrNotFound` vira `domain.NewOrder` com `expected == 0` |
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

O serviço de aplicação escreve os sete campos de `ports.OutboxEntry`
(FND-04 §2.3, `BLK-04`, `BLK-05`): identidade e tempo (`MessageID`,
`OccurredAt`), roteamento (`Intent.Destination`, `Intent.PartitionKey`) e origem
de negócio (`AggregateType`, `AggregateID`, `AggregateVersion`), mais o evento.
Neste exemplo `Destination` é `orders.events` — um nome **lógico** de fluxo, não
um tópico — e `PartitionKey` é o identificador do pedido, para que fatos do
mesmo agregado preservem ordem.

Campos de wire e de estado de drenagem não existem no tipo entregue à porta: os
primeiros são do provider e do contrato, os segundos do schema e do relay.

## Instrumentação do caso de uso

O serviço de aplicação tem o campo `Instrumentation ports.Instrumentation`.
A ordem é fixa: abre a operação **antes** do passo 1 (autorização), o commit
acontece, a operação é fechada com a categoria do desfecho, e só então sai a
auditoria — com `Object`, `Action`, `Outcome` e o `OccurredAt` que a identidade
da mensagem já resolveu. Um campo nulo vira `NoInstrumentation()`, e o serviço
roda sem telemetria em vez de falhar.

O span nasce aqui, no serviço de aplicação, e não no provider (`TRC-16`). O
motivo é que só este bloco conhece a fronteira da operação de negócio: um span
aberto pelo provider mediria a chamada de saída, não o caso de uso.

A porta vive em `ports` e não neste módulo, apesar de ser este quem a usa.
Go satisfaz interface por assinatura idêntica, não por estrutura: um provider
que declarasse o próprio `Result` não satisfaria a interface daqui, e a seta
provider → application é célula proibida da matriz. Declarada acima, realizada
abaixo, como toda porta.

Quem realiza é o `usecase` do `observability`, que traduz cada desfecho
em span, nas três séries de serviço (`MET-08` a `MET-10`) e na trilha de
auditoria. Nada disso aparece nas assinaturas deste módulo — o bloco
`application` continua importando só `context`, `errors` e `ports`.

Dois desfechos merecem atenção porque é fácil confundi-los:

- **Negado** (`errors.Is(err, ports.ErrDenied)`) fecha a operação como
  `Denied` e **não** emite auditoria: nada foi acessado. Qualquer outro erro do
  autorizador é falha técnica e fecha como `Failed` — uma negação nunca é
  inferida a partir de um erro que não a declarou.
- **Rejeitado** é o ramo recusante da UPR, e não é falha (`DEC-04`). Conta como
  requisição, nunca como erro, e a transação commita normalmente.

Uma consulta (`FindOrder`) abre e fecha operação com classe de leitura e não
deixa trilha: consultar não acessa nada auditável.

## Garantias de entrega

A outbox entrega **at-least-once com efeitos idempotentes** (`GAR-02`). Entrega
exatamente-uma-vez fim a fim (*exactly-once*) é **vedada** como promessa em
qualquer artefato do DMPF (P0-3, `GAR-01`): não é realizável sobre transporte e
consumidor independentes, e quem depende dela está descrevendo idempotência no
consumidor com outro nome.

## O que a realização em memória prova, e o que não prova

O `memory` (`libs/backend/go/memory/README.md`) prova: uma transação por
`Within` sobre um recurso, o callback invocado exatamente uma vez, o commit
aplicando estado de negócio e outbox juntos, o rollback por erro e por panic,
uma falha de commit injetável (`FailNextCommit`), o contexto cancelado que
nunca abre transação, e snapshots que não compartilham array com o `Store`.

Não prova **isolamento** nem conflito de serialização entre transações
concorrentes: `Within` retém `txMu` durante todo o callback, então as transações
são serializadas, e `Load` e `Save` da mesma `Tx` sempre veem o mesmo estado. Um
conflito de versão tem de ser injetado pelo chamador — é o que o
`doubles_test.go` de `apps/backend/orders/application` faz, embrulhando o
repositório da `Table` em um repositório instrumentado. Isolamento real é do
`postgres` (`KRN-06`).

O `Store` guarda **dois** mutexes para que essa serialização não vire armadilha:
`txMu` é o que faz uma transação excluir a outra, e `dataMu` protege o estado,
adquirido por operação. Assim `Table.Reader(store)` chamado de dentro do
callback lê o estado já commitado — o que um leitor fora da transação veria —
em vez de travar. Com um mutex único fazendo os dois papéis, essa leitura seria
deadlock; o `store_test.go` do `memory` tem um teste que fixa o comportamento.

O `bind` que liga a transação ao tipo de recursos do caso de uso é escrito pelo
composition root — nos testes, o próprio arquivo de teste — porque
`provider → application` é célula proibida e a realização não pode conhecer o
`Resources` do contexto.

## O que o módulo não contém

Provider Postgres, schema de outbox e isolamento real (`KRN-06`, entregue em
`postgres`); claim, lease, `SKIP LOCKED`, relay e publicação
(`KRN-08`); mapeamento para evento de integração, serialização e wire
(`KRN-05`); a realização da inbox e a quarantine (`KRN-07`, em
`postgres`) e o adapter que aplica o efeito de broker (`KRN-07`,
em `app`); retry por conjunção, orçamento e telemetria (`KRN-09`);
transportes (`KRN-10`). A autorização real e o contexto de execução de nove
campos são do FND-07 — `AuthorizeFunc` é só o gancho que eles preencherão; da
taxonomia de erros do FND-07 este módulo realiza apenas o subconjunto que o
consumo precisa (`Category`, `Failure`, `Classify`). Validar a forma da entrada
(`Quantity <= 0`, `SKU` vazio) é do bloco `app` (RFC §4.1).

Os imports de produção são `context`, `errors`, `fmt`, `slices`, `sync`, o
`domain` e o `ports` — todos capability `pure`. Sem `time`, sem
`rand`, sem `os`, sem `net`, sem `database/sql`, sem `encoding/*` e sem
terceiro. O bloco `application` poderia usar `log`/`log/slog`, que a política
lhe permite (`capability.go:47`), mas esta entrega não usa.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test,test-race,govulncheck -p application
go -C libs/backend/go/application test -race -count=2 -shuffle=on ./...
bash tools/dmpf-gate-check.sh
go run ./tools/dmpf-conformance/cmd/conformance --root . --base origin/develop
```

O `lint` aplica ao bloco a regra `application` do `.golangci.yml`, com
`list-mode: strict`. O `files` dessa regra seleciona por caminho
(`**/application/**`, e `**/*-application/**` do layout anterior ao ADR-045), e
por isso alcança também o bloco `application` de cada contexto em
`apps/backend/<ctx>/application`. Enquanto a realização em memória vivia aqui
como `example/memory`, o glob a alcançava indevidamente — ela é `provider`, cuja
capability o verificador deixa irrestrita (`capability.go:51`) — e precisava de
uma exclusão em `linters.exclusions.rules` e de uma entrada em
`FORA_DO_DEPGUARD` do `gate-check.sh`. Com o `memory` em módulo próprio
(ADR-046), a exclusão e a entrada deixaram de existir.

O `go.mod` só declara `require` dos módulos irmãos, sem `replace`, e não há
`go.sum`: a resolução é do `go.work`, que o `modsync`
(`go run ./tools/dmpf-conformance/cmd/modsync`) mantém sincronizado. Por isso o
target `tidy`, que o plugin do Nx infere, **não** faz parte da cadeia.

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
