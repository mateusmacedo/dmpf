# ports

Bloco `port` do kernel DMPF em Go: a fronteira pela qual um serviço de aplicação
alcança tempo, identidade de mensagem, estado persistido e a outbox, expressa
somente em tipos que o chamador possui. Realiza a fronteira de Unit of Work do
FND-04 (`docs/dmpf/uow-inbox-outbox.md` §3.1) e as portas que a sequência
canônica de nove passos usa (§3.2).

Aqui há apenas declaração. Nenhuma porta deste módulo tem realização: quem
realiza é um provider — o `memory`, em memória e sem banco, e o `postgres`
(`KRN-06`).

Projeto Nx `ports`, tags `type:lib`, `scope:backend`, `stack:go`.
Import path do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/ports`.

## O que o módulo contém

| Package | Unidade DMPF | Conteúdo |
| --- | --- | --- |
| `ports` (raiz) | `kernel/port` | `Instant`, `MessageID`, `Version`; `Clock`, `IDGenerator`; `Reader`, `Repository`, `ErrNotFound`, `ErrVersionConflict`; `PublishIntent`, `OutboxEntry`, `Outbox`; `UnitOfWork[R]` |

Uma unidade só, com `block: port` e `bounded_context: kernel`
(`dmpf-units.json`). São treze identificadores exportados, e a superfície é
fechada: acrescentar um é decisão de spec, não de implementação.

## Os valores de fronteira

`Instant` é um inteiro de nanossegundos desde a época Unix, não um `time.Time`.
O motivo é normativo antes de ser técnico: o verificador classifica o package
`time` inteiro como capability `io.clock`, e a política do bloco `port` admite
apenas `pure` (RFC §6.2). Ler o relógio é I/O, então o instante entra por
`Clock` e o domínio o recebe como valor (RFC §9.3, ADR-014, ADR-016). A
resolução é de nanossegundos porque a drenagem da outbox ordena por
`occurred_at` e segundos colidiriam sob carga (FND-04 §4.2); `Unix()` existe
para os domínios cujo próprio valor de tempo é em segundos.

`MessageID` é o identificador global da mensagem, e o vazio é inválido.
`Version` é a versão do agregado no optimistic locking, e o zero significa
"ainda não persistido": `Save` com `expected == 0` cria.

## O contrato de `Within`

```go
type UnitOfWork[R any] interface {
    Within(ctx context.Context, fn func(ctx context.Context, resources R) error) error
}
```

`R` é o tipo de recursos que o caso de uso declara, e não um tipo do kernel,
porque `UOW-03` exige que as portas transacionais cheguem ao callback como
argumento tipado e `UOW-04` exige que apenas as portas vinculadas cheguem. Um
provider não pode conhecer `R` — `provider → application` é célula proibida —,
então quem conhece os dois lados monta `R` a partir da transação aberta: o
composition root, por uma função `bind`. Passar recursos em `context.Context` é
vedado por `CTX-05`, e um `tx.Repository("nome")` dentro da fronteira é o
service locator que `UOW-03` recusa nominalmente.

Toda realização cumpre as seis cláusulas abaixo, e a suíte reutilizável
`providerkit.UnitOfWork` (em `testkit`, `KRN-11`) as executa contra cada
realização — `memory` e `postgres`:

1. Abre exatamente **uma** transação local sobre **um** recurso (`UOW-01`,
   `UOW-02`).
2. Se `ctx.Err()` não é nulo antes de a transação abrir, devolve esse erro e
   nunca invoca `fn`. É `CTX-21` lido por analogia, do provider de I/O remota
   para a transação local.
3. Invoca `fn` **exatamente uma vez** e nunca a repete, sob nenhum erro
   (`UOW-09`, `UOW-10`). Não há parâmetro de retry em lugar algum deste módulo:
   a política, quando existir, é do caso de uso e do `KRN-09`.
4. `fn` devolvendo `nil` commita; `fn` devolvendo erro faz rollback e devolve o
   mesmo erro, sem embrulho que quebre `errors.Is`.
5. Um erro de commit é devolvido como o provider o produziu, e nada persiste. A
   suíte pede um seam para armar essa falha; a realização que não puder
   oferecê-lo declara a cláusula como não exercitada, em vez de omiti-la em
   silêncio.
6. Panic em `fn` faz rollback e propaga, nunca engolido nem convertido em
   rejeição (`ERR-22`).

A sétima cláusula — `R` como único caminho até as portas transacionais — é
estrutural: quem a garante é o compilador, não um teste.

## Garantias de entrega

A outbox deste kernel entrega **at-least-once com efeitos idempotentes**
(`GAR-02`): o mesmo fato pode ser publicado mais de uma vez, e é o consumidor
que precisa tolerar a repetição. Entrega exatamente-uma-vez fim a fim
(*exactly-once*) é **vedada** como promessa em qualquer artefato do DMPF (P0-3,
`GAR-01`), porque não é realizável sobre transporte e consumidor independentes;
quem depende dela está descrevendo idempotência no consumidor com outro nome.

`Outbox.Enqueue` é o único sumidouro de intenção de publicação dentro da
transação (`UOW-08`). Não existe porta de publicação aqui, deliberadamente:
publicar acontece depois do commit e fora da unidade de trabalho, e é assunto do
relay (`KRN-08`).

## O que o módulo não contém

Nenhuma realização de porta, nenhum SQL, nenhum schema de outbox, nenhum campo
de wire (`message_type`, `schema_version`, `payload`) e nenhum campo de estado de
drenagem (`status`, `available_at`, `attempt_count`, `locked_by`,
`locked_until`, `published_at`, `last_error`). Os primeiros são do provider
(`KRN-06`) e do contrato (`KRN-05`); os segundos, do schema e do relay
(`KRN-08`). Também não há porta de inbox nem de deduplicação (`KRN-07`), nem
retry, orçamento ou telemetria (`KRN-09`).

O fechamento de imports do código de produção é `context`, `errors` e o
`domain` — todos capability `pure`. Sem `time`, sem `rand`, sem `os`, sem
`net`, sem `database/sql`, sem `encoding/*`, sem `log` e sem terceiro.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test,test-race,govulncheck -p ports
bash tools/dmpf-gate-check.sh
go run ./tools/dmpf-conformance/cmd/conformance --root . --base origin/develop
```

O `lint` aplica ao bloco `port` a regra `depguard` do `.golangci.yml`, com
`list-mode: strict`: só imports de capability `pure`, e `time` explicitamente na
`deny` com a mensagem `io.clock`. O `forbidigo` **não** se aplica aqui — fora do
domínio, `errors.New`, `fmt.Errorf` e `panic` são legítimos. O `gate-check`
prova a regra com vetores por bloco; o verificador `conformance` é o gate
autoritativo entre módulos e roda no CI.

O `go.mod` só declara o `require` do módulo irmão, sem `replace`, e não há
`go.sum`: a resolução é do `go.work`, que o `modsync`
(`go run ./tools/dmpf-conformance/cmd/modsync`) mantém sincronizado. Por isso o
target `tidy`, que o plugin do Nx infere, **não** faz parte da cadeia — ele
tentaria a rede para resolver um módulo que ainda não tem tag. Consumo fora do
workspace é matéria do `KRN-12`.

## Governança

Criar um package novo aqui é criar uma unidade DMPF: ele precisa de entrada
própria no `dmpf-units.json` (`include` por import path exato) e o baseline em
`tools/dmpf-baseline/units-baseline.json` precisa ser regravado com
`--write-baseline`. Essa mudança vai em commit separado do código (RFC §10.2);
misturar os dois reprova no CI com `DMPF-T002`.

## Referências

- `docs/specs/SPEC-ZHE7DN1H-dmpf-kernel-aplicacao-go.md` — spec do `KRN-04`
- `docs/adr/034-fronteira-de-uow-em-go.md` — decisões desta realização
- `docs/adr/014-proibir-aresta-domain-port.md` — por que o relógio não vive no domínio
- `docs/adr/021-mapeamento-no-provider-serializacao-na-escrita.md` — a outbox recebe evento de domínio, e o mapeamento é do provider
- `docs/dmpf/uow-inbox-outbox.md` — FND-04: UoW, sequência canônica, outbox
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §3.3, §6.2, §7.3, §10.2
