# domain

Kernel de domínio do DMPF em Go: a realização da unidade de processamento de
requisição (UPR) e do seu desfecho, conforme o FND-03
(`docs/dmpf/upr-decision-mensagens.md` §2 e §3) e o ADR-018. É o bloco `domain`
da fundação — síncrono, determinístico e sem I/O — e o tipo do desfecho que
todo agregado do workspace devolve. O agregado de exemplo que o exercitava,
`Order`, é hoje o bloco `domain` do contexto `orders`
(`apps/backend/orders/domain`, ADR-046): em `libs/backend/go` fica só o kernel
de reuso.

Projeto Nx `domain`, tags `type:lib`, `scope:backend`, `stack:go`.
Import path do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/domain`.

## O que o módulo contém

| Package | Unidade DMPF | Conteúdo |
| --- | --- | --- |
| `domain` (raiz) | `kernel/domain` | `DomainEvent`, `Accepted[R]`, `Accept`, `Empty`, `Rejection`, `Reject`, `Code`, `Detail` |

Uma unidade só, com `block: domain` e `bounded_context: kernel`
(`dmpf-units.json`); em Go, a unidade de verificação é o package (RFC §3.3). A
transcrição dos exemplos §8.2 e §8.3 do FND-03 — o agregado `Order` com as UPRs
`AddItem` e `Place` — é a unidade `orders/domain`, em
`apps/backend/orders/domain`.

## A forma de uma UPR

No agregado de referência (`apps/backend/orders/domain`):

```go
func (o *Order) AddItem(cmd AddItem) (domain.Accepted[ItemAccepted], *domain.Rejection)
```

Exatamente um dos dois retornos é não zero. O segundo é o tipo concreto
`*Rejection`, nunca a interface `error`: assim o compilador garante a condição
do ADR-018 de que só rejeição de domínio trafega pelo canal de recusa. A UPR não
recebe `context.Context`, relógio, gerador de identificador nem aleatoriedade;
tempo e identificadores chegam como valores já resolvidos (RFC §9.3).

Lendo o desfecho do lado de quem chama:

```go
acc, rej := order.AddItem(domain.AddItem{SKU: "ABC", Quantity: 1, At: at})
if rej != nil {
    // Rejected: rej.Code() é estável ("orders/item-limit-exceeded"),
    // rej.Details() traz os detalhes em termos de domínio, e o pedido
    // está exatamente como antes da chamada (DEC-10).
    return errors.Join(rej) // o application service pode embrulhar e usar errors.As
}
resp := acc.Response()   // ItemAccepted{Order, Items}
events := acc.Events()   // []DomainEvent, na ordem em que foram produzidos
```

Garantias que o código realiza e a suíte prova:

- **Determinismo** (§2.4): mesma entrada, mesma variante, mesma resposta e a
  mesma sequência de eventos, na mesma ordem.
- **Pós-condição da recusa** (`DEC-10`, `DEC-11`): a UPR decide sobre uma cópia e
  só substitui o agregado no aceite; uma recusa não toca o estado e não carrega
  evento.
- **Imutabilidade da sequência** (`DEC-13`): `Accept`, `Events()`, `Reject` e
  `Details()` copiam o que recebem e devolvem.
- **Contrato de conteúdo** (`DEC-12`): a imutabilidade do conteúdo da resposta e
  de cada evento é contrato dos tipos do agregado — valores comparáveis cujos
  campos não carregam ponteiro, slice, map nem função. Sem `reflect`, o kernel
  não consegue copiar em profundidade um `R` arbitrário. O teste de compilação do
  agregado de referência (`apps/backend/orders/domain`) prova a parte
  mecanizável (sem slice, map ou função); `comparable` não
  exclui ponteiro, e essa ausência é item de revisão de cada agregado. Detalhes
  e alternativas no ADR-032.

## O que o módulo não contém

Porta, provider, Unit of Work, outbox, conversão para evento de integração,
mapeamento de `Rejection` para protocolo (HTTP, gRPC) e qualquer tipo de wire.
Isso pertence ao `KRN-04`, ao `KRN-05` e ao FND-07, respectivamente. O módulo não
importa `time`, `context`, `os`, `net`, `encoding/json`, `log` nem `reflect`, e
não tem dependência de terceiro.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test,test-race,govulncheck -p domain
bash tools/dmpf-gate-check.sh
go run ./tools/dmpf-conformance/cmd/conformance --root . --base origin/develop
```

O `lint` aplica duas camadas ao bloco `domain` (`.golangci.yml`): `depguard`
por package (só imports de capability `pure`) e `forbidigo` por símbolo
(`time.Now`, `fmt.Print*`, `fmt.*Scan*`, `print`/`println`, `errors.New`,
`fmt.Errorf`, `panic`). O `gate-check` prova cada camada com vetores; o
verificador `conformance` é o gate autoritativo entre módulos e roda no CI.

## Governança

Criar um package novo aqui é criar uma unidade DMPF: ele precisa de entrada
própria no `dmpf-units.json` (`include` por import path exato) e o baseline em
`tools/dmpf-baseline/units-baseline.json` precisa ser regravado com
`--write-baseline`. Essa mudança vai em commit separado do código (RFC §10.2);
misturar os dois reprova no CI com `DMPF-T002`.

## Referências

- `docs/specs/SPEC-XF9TF9A0-dmpf-kernel-dominio-go.md` — spec do `KRN-03`
- `docs/adr/032-realizacao-go-do-desfecho-da-upr.md` — decisões desta realização
- `docs/adr/018-forma-do-desfecho-da-upr.md` — a forma normativa do desfecho
- `docs/dmpf/upr-decision-mensagens.md` — FND-03: UPR, `Decision`, mensagens
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §3.3, §6.2, §9, §10.2
