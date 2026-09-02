# ADR-032: Realizar o desfecho da UPR em Go como par `(Accepted[R], *Rejection)` com tipo concreto

## Status

Aceito — 2026-09-02. Implementa SPEC-XF9TF9A0.

## Contexto

O FND-03 (`docs/dmpf/upr-decision-mensagens.md` §2 e §3) fixa a UPR como unidade
síncrona, determinística e sem I/O, e o seu desfecho como a união exaustiva
`Decision = Accepted(resposta, eventos) | Rejected(rejeição)`, com as regras
`DEC-01`..`DEC-13`. O ADR-018 tornou a **forma** normativa e deixou o
**mecanismo** para cada kernel, admitindo o par `(decisão, erro)` idiomático de
Go sob uma condição: o canal de erro transporta **apenas** rejeições de domínio.
A guarda-chuva SPEC-YRJRADY9 escolheu esse par. O ADR-014 fechou a aresta
`domain → port`; o ADR-016 fixou o domínio executável em memória em duas metades
e recusou a mockabilidade como satisfação; o ADR-031 entregou o verificador que
decide a regra de dependência sobre o grafo real e classifica cada builtin por
capability.

Até esta história, nenhuma linha de código realizava a norma: o módulo
`dmpf-domain-go` era um placeholder do `KRN-01`. Três forças pressionavam a
realização. A primeira é a **distinguibilidade** de `DEC-04`: se o segundo
retorno fosse `error`, um `fmt.Errorf` vazaria falha técnica pelo mesmo canal e a
exaustividade de `DEC-01` deixaria de ser verificável no chamador. A segunda é a
**pós-condição da recusa** (`DEC-10`): "validar tudo antes de mutar" é correto
hoje e frágil ao próximo invariante intercalado. A terceira é a
**imutabilidade** (`DEC-12`, `DEC-13`): Go não tem slice imutável, e uma cópia
profunda de um `R` arbitrário exigiria `reflect`, que o verificador classifica
como `runtime.framework` e o bloco `domain` não pode importar.

Dois achados do próprio terreno restringiram o desenho. O verificador do KRN-02
classifica o package `time` inteiro como `io.clock`
(`libs/backend/go/dmpf-conformance/internal/rule/stdlib.go`), então importá-lo
no domínio reprova com `DMPF-E001`. E tanto o `depguard` quanto o verificador
decidem por package e declaram não distinguir `time.Now()` de `time.Duration`
nem `fmt.Println` de `fmt.Sprintf` — a metade dinâmica de RFC §9.1 ficava sem
defesa mecânica.

## Decisão

**Forma do desfecho.** Toda UPR do kernel Go tem a assinatura
`func (a *Aggregate) Verb(cmd Command) (dmpfdomain.Accepted[R], *dmpfdomain.Rejection)`.
O segundo retorno é o **tipo concreto** `*Rejection`, nunca a interface `error`:
o compilador impede que qualquer outro valor trafegue pelo canal de recusa, o que
é a condição do ADR-018 tornada estrutural. `*Rejection` implementa `error`
apenas para que o application service a embrulhe (`errors.Join`) e a recupere
(`errors.As`) ao consumir — nunca para que a UPR a devolva como `error`.

**Superfície do package raiz.** Exatamente oito identificadores: `DomainEvent`
(interface mínima `EventName() string`), `Accepted[R]`, `Accept`, `Empty`,
`Rejection`, `Reject`, `Code` e `Detail`. `Code` é `string` nomeado no formato
`contexto/motivo`, validado por `Code.Valid()` e provado por teste que enumera
as constantes de cada agregado — o construtor não valida porque não pode
devolver `error` nem lançar `panic`.

**Pós-condição da recusa por construção.** A UPR clona o estado, decide sobre a
cópia e substitui o receptor apenas no aceite (`*o = next`). Uma recusa é
incapaz de tocar o receptor, o que realiza `DEC-10` sem depender da ordem dos
invariantes.

**Imutabilidade, com limite declarado.** `Accept`, `Events()`, `Reject` e
`Details()` copiam as sequências que recebem e devolvem, o que realiza
`DEC-13` sobre a **sequência**. O **conteúdo** da resposta `R` e de cada
`DomainEvent` é contrato dos tipos do agregado: valores comparáveis, sem slice,
map ou ponteiro, provado no exemplo por teste de compilação
(`mustBeComparable[T comparable]`). O kernel não garante `DEC-12` sobre um `R`
mutável — sem `reflect`, a cópia profunda de `any` não é realizável — e o FND-03
§3.5 admite a imutabilidade como convenção sustentada por cópia.

**Tempo como valor.** O instante entra como tipo de domínio próprio (`Instant`,
inteiro de segundos) e o módulo não importa `time`, porque o verificador o
classifica como `io.clock`. A tabela de RFC §9.3 já dizia que o domínio recebe o
instante em vez de buscá-lo; aqui a classificação do verificador torna a regra
mecânica.

**Segunda camada do lint, por símbolo.** O `.golangci.yml` ganha `forbidigo`
restrito ao caminho `-domain/` (`linters.exclusions.rules[].path-except`), com
`analyze-types: true` e seis famílias: relógio (`time.Now` e afins), saída
(`fmt.Print*`), entrada (`fmt.*Scan*`), builtins `print`/`println` (o `forbidigo`
descarta os próprios defaults quando recebe lista própria), erro não tipado
(`errors.New`, `fmt.Errorf`) e lançamento (`panic`). O `tools/dmpf-gate-check.sh`
prova um vetor por família em cada execução, além dos vetores de package do
`depguard`.

**Cada package é uma unidade.** O agregado de exemplo vive em `example/orders`,
package exportado do mesmo módulo e do mesmo `bounded_context: dmpf-kernel`,
declarado como unidade própria `dmpf-kernel/example-orders` no manifesto e no
baseline, em commit separado do código (RFC §10.2).

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Segundo retorno `error`, com `errors.As` no chamador | A garantia do ADR-018 viraria convenção: um `fmt.Errorf` no domínio vazaria falha técnica pelo canal de recusa e o compilador não acusaria. |
| Struct com campo discriminante (`Decision{Kind, Accepted, Rejected}`) | Descartada já pela guarda-chuva: menos idiomática em Go e não melhora a exaustividade — o chamador continua precisando olhar dois valores. |
| Restringir `R comparable` no kernel para garantir `DEC-12` sobre o conteúdo | Proibiria respostas legítimas com coleções; a garantia é do agregado, não do kernel. |
| Cópia profunda de `R` via `reflect` | `reflect` é `runtime.framework` no verificador e o bloco `domain` reprovaria. |
| "Validar tudo antes de mutar" como realização de `DEC-10` | Correta hoje, frágil ao próximo invariante intercalado; decidir sobre cópia torna a recusa incapaz de tocar o receptor. |
| Importar `time` e usar `time.Time` como valor | O verificador classifica o package inteiro como `io.clock` (ADR-031); o import reprova com `DMPF-E001`, e a spec não reabre a tabela. |
| Teste estrutural com `go/ast` dentro do módulo para a metade dinâmica | `go/ast` e `go/parser` estão fora da allowlist do `depguard`; o próprio teste reprovaria no lint. |
| `.golangci.yml` por módulo para escopar o `forbidigo` | O golangci sobe a partir do diretório e um arquivo local substituiria a política inteira, duplicando o `depguard`. |

## Consequências

**Positivas:**

- A condição do ADR-018 deixa de depender de disciplina: vazamento de erro
  técnico pelo canal de recusa é erro de compilação.
- `DEC-10` vale por construção, e a suíte prova a igualdade estrutural do
  snapshot antes e depois de cada recusa de cada UPR.
- A metade dinâmica de RFC §9.1 ganha defesa mecânica parcial: `time.Now()`,
  `fmt.Println`, `fmt.Scanln`, `println`, `fmt.Errorf` e `panic` reprovam no
  lint, com vetor persistente por família.
- Os módulos `KRN-04`..`KRN-12` herdam a forma de referência de UPR em Go e o
  contrato de tipos comparáveis para respostas e eventos.

**Negativas:**

- **Custo aceito:** `DEC-12` sobre o conteúdo de `R` e dos eventos é contrato,
  não garantia do kernel. Um agregado que devolva slice na resposta pode ter o
  desfecho alterado pelo chamador; o teste de compilação do exemplo é o modelo a
  replicar em cada agregado.
- Uma cópia do agregado por chamada de UPR e uma cópia da sequência por
  `Events()`. Irrelevante para agregados em memória; a alternativa
  `EventSequence` iterável fica como evolução se o `KRN-04` precisar.
- O vocabulário de tempo (`Instant`) é local ao exemplo. Um tipo compartilhado e
  o relógio como porta são do `KRN-04`.
- **Consequência declarada, não decidida:** pela C2 (RFC §5.5; ADR-017), um
  `domain` de bounded context de negócio não pode importar `dmpf-kernel/domain`
  (`DMPF-D002`), e uma unidade `domain` nunca é superfície pública
  (`DMPF-M002`). Esta história fica inteira em `dmpf-kernel` e o problema não a
  alcança, mas ele alcança todo consumidor real do kernel. A adoção por
  contexts de negócio exige decisão da fundação — ADR sobre kernel compartilhado
  ou revisão da C2 — e é levada à SPEC-YRJRADY9, nunca resolvida por
  reclassificação ou exceção.
- Divergência doc/código herdada e não corrigida aqui: o comentário em
  `internal/rule/stdlib.go` e a linha do ADR-031 no índice dizem "`time` puro",
  mas o código e o teste classificam `time` como `io.clock`. Corrigir a prosa é
  do `KRN-02`.

## Referências

- ADR-014 — proíbe a aresta `domain → port`. Sem porta, a UPR não alcança I/O:
  é o que sustenta que o canal de recusa só transporta rejeição de domínio.
- ADR-016 — domínio executável em memória. Recusa `Clock` e `IdGenerator` sob
  interface; o instante entra como valor.
- ADR-017 — bounded context declarado e superfície pública. Origem da
  consequência declarada sobre adoção entre contexts.
- ADR-018 — forma do desfecho da UPR. Este ADR fixa o mecanismo Go que aquele
  deixou a cada kernel.
- ADR-031 — verificador de conformidade. A classificação de `time` como
  `io.clock` e a decisão por package são as fronteiras que este ADR respeita e
  complementa com o `forbidigo`.
- `docs/dmpf/upr-decision-mensagens.md` (FND-03) — §2.4 (determinismo), §3.1–§3.5
  (`DEC-01`..`DEC-13`), §3.4 (equivalência observável), §8.2 e §8.3 (exemplos
  transcritos em `example/orders`).
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §6.2 (`domain` default deny), §9.1–§9.3
  (duas metades; o domínio recebe em vez de buscar), §10.2 (commit próprio).
- SPEC-XF9TF9A0 — spec que este ADR implementa.
- ARQ-522 — https://lider-cap.atlassian.net/browse/ARQ-522.
