---
id: SPEC-XF9TF9A0
slug: dmpf-kernel-dominio-go
title: DMPF KRN-03 — Kernel de domínio Go: UPR, Decision e Rejection
stage: done
priority: P0
depends_on: [SPEC-MQA5HAXF, SPEC-WTAXFV8B]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-522
subtask_urls: []
created: 2026-09-02
---

# SPEC-XF9TF9A0: DMPF KRN-03 — Kernel de domínio Go: UPR, Decision e Rejection

## Resumo

Preencher o módulo `dmpf-domain-go` (`libs/backend/go/dmpf-domain`), hoje um
placeholder do `KRN-01`, com a realização Go do bloco `domain` da fundação DMPF:
o desfecho da UPR como par `(Accepted[R], *Rejection)`, o tipo `Rejection` com
código estável `contexto/motivo`, o contrato de evento de domínio como sequência
ordenada e fechada no retorno, e um agregado de exemplo com duas UPRs que serve
de sujeito executável em memória para o `KRN-04` e para os vetores. Como time de
plataforma, queremos que as doze invariantes da UPR e as treze regras do desfecho
(`docs/dmpf/upr-decision-mensagens.md` §2 e §3) deixem de ser prosa e passem a
ser código verificado pelo gate — para que P0-1 ("domínio sem I/O") tenha, pela
primeira vez, um sujeito real no repositório.

Esta é a primeira metade do Incremento 2 ("Núcleo") da spec guarda-chuva
[SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md): "Caso de uso completo em
memória, com UPR pura e UoW explícita". A segunda metade é o `KRN-04`, que
consome a `Decision` no passo 5 da sequência canônica.

## Contexto

A fundação definiu a UPR em detalhe e nenhuma linha de código realiza essa norma.
O FND-03 (`docs/dmpf/upr-decision-mensagens.md`) fixa doze invariantes
(`UPR-I01`..`UPR-I12`), cinco regras de ciclo de vida (`UPR-L01`..`UPR-L05`), o
determinismo (§2.4), o desfecho como união exaustiva
`Decision = Accepted(resposta, eventos) | Rejected(rejeição)` com as regras
`DEC-01`..`DEC-13`, e a fronteira com o application service (`FRT-01`..`FRT-04`).
O ADR-018 tornou a **forma** do desfecho normativa e deixou o **mecanismo** para
cada kernel, admitindo o par `(decisão, erro)` de Go desde que o canal de erro
transporte apenas rejeições de domínio. O ADR-014 fechou a aresta
`domain → port`; o ADR-016 fixou o domínio executável em memória em duas metades
conjuntas (estática e dinâmica) e recusou a mockabilidade como satisfação.

- **Problema**: o módulo `dmpf-domain-go` existe desde o `KRN-01` com um único
  símbolo, `DmpfDomain(name string) string`, criado apenas para exercitar a
  cadeia de sete passos. A SPEC-MQA5HAXF:128 registra que "`KRN-03` preenche o
  módulo com UPR, `Decision` e `Rejection`". Sem esse preenchimento, o `KRN-04`
  não tem o que orquestrar e os vetores de determinismo e recusa não têm sujeito.
- **Impacto**: o `KRN-04` recebe um agregado real para percorrer os dez passos da
  sequência canônica em memória; os módulos `KRN-05`..`KRN-12` recebem a forma de
  referência de UPR em Go; a fundação recebe a primeira prova de que a norma é
  realizável sem escape hatch.
- **Inspiração**: os exemplos §8.2 (determinismo em dupla execução) e §8.3
  (rejeição tipada, sem exceção e sem evento) do FND-03, transcritos para Go; o
  idioma Go de erros como valores, restrito aqui a um tipo fechado; a decomposição
  do verificador do `KRN-02` em unidades classificadas, que provou que "passar no
  próprio gate" precisa ser teste real.
- **Links relevantes**:
  - [SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md) — guarda-chuva; esta
    spec realiza o requisito funcional `KRN-03`
  - [SPEC-MQA5HAXF](./SPEC-MQA5HAXF-dmpf-fundacao-nx-go.md) — `KRN-01`, fixou o
    módulo, as tags, a cadeia e o `bounded_context: dmpf-kernel`
  - [SPEC-WTAXFV8B](./SPEC-WTAXFV8B-dmpf-verificador-conformidade-go.md) —
    `KRN-02`, o verificador que é o instrumento dos critérios mecânicos desta spec
  - `docs/dmpf/upr-decision-mensagens.md` — FND-03, §2 (UPR), §3 (desfecho), §4
    (fronteira), §5.3 (nomenclatura), §8.2 e §8.3 (exemplos)
  - `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §2.1 (princípio 8), §2.3 (P0-1,
    P0-2, P0-3), §3.3 (binding em Go: package é a unidade), §6.2 (`domain` é
    default deny, só `pure`), §7.4 (células 4, 5 e 6), §9.1–§9.3 (domínio
    executável em memória), §10.3 (diagnósticos), §11.2 (V07, V13, V14, V15,
    V27, V31)
  - `docs/adr/012-classificacao-por-metadado-declarado.md`,
    `014-proibir-aresta-domain-port.md`, `016-dominio-executavel-em-memoria.md`,
    `017-bounded-context-declarado-superficie-publica.md`,
    `018-forma-do-desfecho-da-upr.md`, `019-niveis-de-contrato-e-conversao-de-evento.md`,
    `028-processo-de-autorizacao-da-classificacao.md`,
    `030-granularidade-modulo-go-e-bom.md`,
    `031-verificador-de-conformidade-dmpf-em-go.md`

### Divergências entre o ticket e o repositório

O ticket ARQ-522 foi escrito em 30/08, antes de `KRN-01` e `KRN-02` serem
entregues. Quatro pontos do texto não batem com o estado do repositório e ficam
fixados aqui, sem reabrir decisão alguma:

| Ticket diz | Repositório | Esta spec fixa |
| --- | --- | --- |
| "Criar `libs/backend/dmpf-domain`" | O módulo existe em `libs/backend/go/dmpf-domain` (ADR-030), com projeto Nx `dmpf-domain-go` e unidade `dmpf-kernel/domain` no manifesto e no baseline | **Evoluir** o módulo existente; remover o placeholder `DmpfDomain` |
| "ADR a partir de `029`" | `029`, `030` e `031` já existem | O ADR desta história é o **`032`** |
| "Vetor negativo com `net/http` reprova com `DMPF-D001` (V13)" | No verificador, `domain → net/http` emite **`DMPF-E001`** (`io.network` não permitida para o bloco — `internal/rule/stdlib.go:17`, `integration_test.go` `TestArestaTransitivaEntreModulos`). `D001` é aresta unidade→unidade, e V13 propriamente dito é `domain → provider` (`rule/vectors_test.go:292`, `TestArestaInternaEntreModulosProduzD001`) | O critério exige **`E001`** para `net/http` e aponta a cobertura de `D001` já existente |
| Suíte "em memória, sem depender de horário", com instante no exemplo | O verificador classifica o package `time` inteiro como `io.clock` (`internal/rule/stdlib.go:28`, teste `capability_test.go:88`): importá-lo em `domain` reprova com `E001` | O instante entra como **valor de domínio próprio**, sem `time`. O comentário em `stdlib.go:67-73` e a linha do ADR-031 em `docs/adr/README.md:130` dizem "`time` puro" — divergência doc/código do `KRN-02`, registrada em "Escopo fora" |

### Fontes normativas

| Fonte | Força | O que restringe nesta spec |
| --- | --- | --- |
| FND-03 §2.1 | `normativo` | UPR síncrona, determinística, sem I/O; assinatura sem contexto, cancelamento, deadline ou falha técnica |
| FND-03 §2.2 | `normativo` | `UPR-I01`..`UPR-I12`; violação de `I07`..`I12` descaracteriza a unidade |
| FND-03 §2.3 | `normativo` | `UPR-L01`..`UPR-L05`: sem estado entre execuções, sem recurso, sem trabalho pendente |
| FND-03 §2.4 | `normativo` | Mesma entrada → mesma variante, resposta e **sequência ordenada** de eventos; tempo, id e aleatoriedade chegam resolvidos |
| FND-03 §3.1–§3.5 | `normativo` | `DEC-01`..`DEC-13`, incluindo a pós-condição de estado (`DEC-10`), ausência de eventos em `Rejected` (`DEC-11`) e imutabilidade (`DEC-12`, `DEC-13`) |
| FND-03 §3.4 | `normativo` | Tabela de equivalência observável: condição necessária de qualquer realização |
| FND-03 §4.1, §4.4 | `normativo` | `FRT-01`..`FRT-04`; validação de forma é da aplicação, invariante de estado é da UPR |
| FND-03 §5.3 | `normativo` | Evento no passado (`MSG-N01`), sem versão (`N02`), sem transporte (`N03`); código de rejeição `contexto/motivo` |
| RFC §6.2, §6.3 | `normativo` | `domain` é default deny: apenas capability `pure` no fechamento transitivo |
| RFC §9.1–§9.3 | `normativo` | Duas metades, estática e dinâmica; mock não satisfaz; o domínio recebe em vez de buscar |
| RFC §3.3, §10.1 | `normativo` | Em Go a unidade é o package; `include` por import path exato; cada package novo é unidade nova |
| RFC §10.2 | `normativo` | Mudança normativa (criar unidade) vem em commit próprio — mecanismo mínimo vigente enquanto o ADR-028 não é vigente |
| ADR-018 | aceito | Forma normativa; par `(decisão, erro)` conforme se o canal de erro só transporta rejeição de domínio |
| ADR-014, ADR-016 | aceito | Sem porta no domínio; sem `Clock`/`IdGenerator` ainda que sob interface |
| ADR-019 | aceito | Evento de domínio não atravessa inteiro; conversão para integration event ocorre fora do domínio e fora desta spec |

<constraints>
- [P0] Domínio sem I/O (P0-1; RFC §6.2; `UPR-I07`): o fechamento transitivo de
  imports das unidades do módulo contém APENAS capability `pure` segundo a tabela
  do verificador (`internal/rule/stdlib.go`). Em particular, NUNCA importar
  `time`, `context`, `os`, `net/*`, `math/rand`, `crypto/rand`, `encoding/json`,
  `log`, `log/slog`, `reflect` nem qualquer módulo de terceiro.
- [P0] Determinismo (FND-03 §2.4): fixados estado e requisição, duas execuções
  devolvem a MESMA variante, a MESMA resposta e a MESMA sequência de eventos, na
  MESMA ordem. Nenhuma UPR obtém tempo, identificador ou aleatoriedade por conta
  própria; nenhuma recebe `Clock`, `IdGenerator` ou porta, ainda que sob
  interface (ADR-016; RFC §9.2).
- [P0] Pós-condição da recusa (`DEC-10`, `DEC-11`): após `Rejected`, o estado
  observável do agregado é estruturalmente IGUAL ao anterior à chamada, e a
  sequência de eventos é VAZIA. Não existe coleção de eventos pendentes no
  agregado (`DEC-07`, `DEC-08`).
- [P0] Canal de erro fechado (ADR-018; `DEC-04`): o segundo valor de retorno de
  toda UPR é o tipo concreto `*Rejection`, NUNCA a interface `error`. Nenhum
  símbolo exportado do módulo devolve `error`. Nenhuma assinatura exportada
  recebe `context.Context` (`FRT-03`).
- [P0] Imutabilidade (`DEC-12`, `DEC-13`): `Accepted` e `Rejection` não expõem
  operação que altere o desfecho, e toda sequência devolvida é cópia — mutar o
  retorno NUNCA altera o desfecho.
- [P0] Classificação declarada (ADR-012; RFC §3.3, §10.1): cada package de
  produção do módulo é uma unidade com entrada própria no `dmpf-units.json`
  (`include` por import path exato) e no baseline, todas com `block: domain` e
  `bounded_context: dmpf-kernel`. A mudança de classificação vem em COMMIT
  PRÓPRIO, separado do código (RFC §10.2).
- [P0] Nenhum artefato desta entrega declara nem sugere exactly-once fim a fim
  (P0-3, V31); determinismo NÃO é idempotência (FND-03 §8.2).
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva: divergência
  entre a forma normativa e o idioma Go vira ADR (`032`), nunca alteração
  silenciosa da norma.
- [P1] A `Rejection` NÃO carrega código HTTP, código gRPC, política de retry nem
  causa técnica encadeada (FND-03 §3.3); o domínio não conhece 422 (§3.6).
- [P1] O módulo continua sem dependência de terceiro: `go.mod` sem `require`,
  `go.sum` ausente.
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Tipo `Rejection` no package raiz `dmpfdomain`** (`DEC-03`, `DEC-09`,
  FND-03 §3.3): valor imutável com três campos não exportados — `code` do tipo
  `Code`, `message` string e `details` como sequência ordenada de `Detail{Key,
  Value string}` —, acessíveis por `Code()`, `Message()` e `Details()` (este
  devolvendo cópia). Construtor `Reject(code Code, message string, details
  ...Detail) *Rejection`.
  - `Code` é `string` nomeado, no formato `contexto/motivo`; `Code.Valid()`
    aplica a expressão `^[a-z][a-z0-9]*(-[a-z0-9]+)*/[a-z][a-z0-9]*(-[a-z0-9]+)*$`.
    `Reject` não valida e não entra em pânico: a validade dos códigos declarados
    é assegurada por teste que enumera as constantes de cada agregado.
  - `*Rejection` implementa `error` (`Error()` devolve `"<code>: <message>"`)
    para que o application service possa embrulhá-la e usar `errors.As`. A UPR,
    porém, NUNCA a devolve como `error` — devolve `*Rejection`.
  - Sem campo, método ou construtor que aceite status HTTP, código gRPC,
    política de retry ou `error` encadeado. Edge case: `Details()` de rejeição
    sem detalhes devolve slice vazio, nunca `nil` — o chamador não distingue
    ausência de detalhe por nulidade.
- [x] **[P0] Tipo `Accepted[R any]` no package raiz** (`DEC-05`..`DEC-08`,
  `DEC-12`, `DEC-13`): valor imutável com `response R` e `events []DomainEvent`
  não exportados; construtor `Accept[R any](response R, events ...DomainEvent)
  Accepted[R]` que CLONA a sequência recebida; `Response() R`; `Events()
  []DomainEvent` devolvendo cópia nova a cada chamada, nunca `nil` (vazia quando
  não há evento).
  - Tipo `Empty struct{}` para resposta explicitamente vazia (`DEC-05`): "uma
    decisão sem resposta útil carrega resposta explicitamente vazia, nunca
    resposta ausente".
  - Nenhum método adiciona, remove, reordena ou filtra eventos após a construção.
- [x] **[P0] Contrato `DomainEvent` no package raiz** (`UPR-I06`, `DEC-06`,
  FND-03 §5.3): interface mínima `EventName() string`, em que o nome é um fato no
  passado (`MSG-N01`), sem versão (`MSG-N02`) e sem transporte, formato ou
  tecnologia (`MSG-N03`). Nenhum tipo de wire, contexto ou porta aparece na
  superfície do package (`UPR-I11`, `UPR-I12`).
- [x] **[P0] Forma da UPR em Go** (FND-03 §2.1, §3.4; ADR-018): método do
  agregado, ou função pura sobre ele, com a assinatura
  `func (a *Aggregate) Verb(cmd Command) (dmpfdomain.Accepted[Response],
  *dmpfdomain.Rejection)`. Exatamente um dos dois retornos é não zero: em
  `Rejected`, o `Accepted[R]` é o valor zero e `Events()` é vazio; em
  `Accepted`, a `*Rejection` é `nil`.
  - Sem `context.Context`, sem `time.Time`, sem interface de relógio, gerador de
    identificador ou aleatoriedade em parâmetro ou campo (`FRT-03`, RFC §9.3).
  - Realização **decide-sobre-cópia, efetiva-se-no-aceite**: a UPR clona o
    estado, aplica a decisão sobre a cópia e só substitui o receptor quando o
    desfecho é `Accepted`. Recusa nunca toca o receptor (`DEC-10`).
  - O agregado NÃO mantém coleção de eventos pendentes nem método que a
    exponha; eventos existem apenas no `Accepted` (`DEC-07`, `DEC-08`).
- [x] **[P0] Agregado de exemplo `Order`** no package
  `libs/backend/go/dmpf-domain/example/orders`, unidade `dmpf-kernel/example-orders`,
  transcrição em Go dos exemplos §8.2 e §8.3 do FND-03:
  - Estado: `id OrderID`, `status Status` (`Open`, `Placed`), `items []Item`
    (`SKU`, `Quantity`), `itemLimit int`. Construtor `NewOrder(id OrderID,
    itemLimit int) *Order`. `Snapshot() Snapshot` devolve valor com cópia dos
    itens, e `Snapshot.Equal(other Snapshot) bool` compara estruturalmente com
    `slices.Equal` — sem `reflect`, que é `runtime.framework` no verificador.
  - Valor `Instant` (inteiro de segundos Unix encapsulado, comparável), recebido
    nos commands e carregado nos eventos, sem importar `time` (RFC §9.3: o
    instante chega como valor de entrada).
  - UPR 1 — `AddItem(cmd AddItem{SKU, Quantity, At}) (Accepted[ItemAccepted],
    *Rejection)`: aceita quando o pedido está `Open` e `len(items)+1 <=
    itemLimit`, acrescenta o item e emite `ItemAdded{Order, SKU, Quantity, At}`;
    recusa com `orders/order-not-open` quando o pedido não está `Open` e com
    `orders/item-limit-exceeded` (detalhes `limit` e `attempted`) quando o
    limite seria excedido.
  - UPR 2 — `Place(cmd PlaceOrder{At}) (Accepted[PlacedResponse], *Rejection)`: aceita
    quando o pedido está `Open` e tem ao menos um item, muda o status para
    `Placed` e emite `OrderPlaced{Order, Items, At}`; recusa com
    `orders/empty-order` sem itens e com `orders/order-not-open` quando já
    colocado.
  - Edge case fixado por FND-03 §4.4: `Quantity <= 0` é validação de forma da
    entrada e pertence à fronteira de aplicação — a UPR NÃO a valida e o exemplo
    NÃO declara código para isso. O que a UPR valida depende do estado do
    agregado (`UPR-I03`).
  - Nomes das mensagens seguem §5.3: commands no imperativo (`AddItem`,
    `PlaceOrder`), eventos no passado (`ItemAdded`, `OrderPlaced`), códigos
    `orders/<motivo>`.
- [x] **[P0] Remoção do placeholder do `KRN-01`**: `dmpf-domain.go`
  (`DmpfDomain`) e `dmpf-domain_test.go` saem do módulo. A SPEC-MQA5HAXF:128 os
  define como "conteúdo mínimo" a ser preenchido por esta história; nenhum
  outro `.go` ou `.json` os referencia (verificar com
  `grep -rln DmpfDomain --include='*.go' --include='*.json'` antes de remover).
- [x] **[P0] Manifesto e baseline**: `dmpf-units.json` do módulo passa a
  declarar duas unidades, ambas `block: domain`, `bounded_context: dmpf-kernel`,
  `public_integration_surface: false`, `include` por import path exato —
  `dmpf-kernel/domain` (raiz, inalterada) e `dmpf-kernel/example-orders`
  (`.../dmpf-domain/example/orders`). `tools/dmpf-baseline/units-baseline.json`
  é regravado com `--write-baseline` e passa a ter a entrada nova.
  - As duas edições vêm em **um commit próprio**, sem código Go, com o ato
    declarado no assunto e no corpo ("criar unidade `dmpf-kernel/example-orders`",
    bloco, contexto, arestas liberadas: nenhuma), como exige RFC §10.2 e como o
    verificador cobra em `internal/baseline/authorization.go`
    (`VerificarAutorizacao`). Precedente: commit `92fcd4d` do ARQ-521.
  - Sem esse commit, o gate reprova com `DMPF-T001` (divergência) ou `DMPF-T002`
    (mudança sem commit próprio); com o código no mesmo commit, também `T002`.
- [x] **[P0] Suíte de domínio** em `*_test.go` dos dois packages, executável com
  `go test -race -count=2 -shuffle=on ./...`, cobrindo:
  - Determinismo (§2.4, §8.2): dois agregados construídos do mesmo snapshot
    recebem o mesmo command e produzem `Accepted` com respostas iguais e
    sequências de eventos iguais, elemento a elemento e na mesma ordem.
  - `DEC-10`: em cada recusa de cada UPR, `Snapshot()` antes e depois são
    `Equal`.
  - `DEC-11`: em cada recusa, `Accepted[R]` é o valor zero e `Events()` tem
    comprimento zero.
  - `DEC-12`, `DEC-13`: `append`, reatribuição de elemento e reordenação sobre o
    slice devolvido por `Events()` não alteram uma segunda chamada a `Events()`;
    o mesmo para `Details()`.
  - `DEC-05`: `Accept(Empty{})` é aceito com resposta explicitamente vazia.
  - `Code.Valid()`: positivos (`orders/empty-order`) e negativos (`EmptyOrder`,
    `orders/`, `/empty`, `orders/Empty-Order`, `orders//x`); todas as constantes
    de código do exemplo são válidas.
  - `*Rejection` como `error`: `errors.As(err, &rej)` recupera a rejeição
    quando o chamador a embrulha.
  - Ambos os ramos de ambas as UPRs (aceite com evento; recusa por cada código).
- [x] **[P1] `forbidigo` na regra `domain` do `.golangci.yml`**, restrito a
  `**/*-domain/**` como o `depguard`, proibindo os símbolos que o import não
  distingue e que o verificador declara não cobrir (`stdlib.go:67-73`):
  `time\.(Now|Since|Until|Tick|After|AfterFunc|NewTimer|NewTicker|Sleep)`,
  `fmt\.(Print|Println|Printf|Fprint|Fprintln|Fprintf)`, `errors\.New`,
  `fmt\.Errorf`, `panic\(`. A metade dinâmica de RFC §9.1 passa a ter defesa
  mecânica parcial além da estática.
  - `tools/dmpf-gate-check.sh` ganha um vetor de símbolo (`time.Now()` num
    arquivo `*-domain`) provando que a regra reprova; sem o vetor, remover a
    regra passaria despercebido — mesmo argumento dos vetores do `depguard`.
  - `errors.New` e `fmt.Errorf` ficam proibidos porque o domínio não fabrica
    erro não tipado: só `Rejection` trafega. `Rejection.Error()` usa
    `fmt.Sprintf`, que permanece permitido.
- [x] **[P1] ADR-032** em `docs/adr/032-realizacao-go-do-desfecho-da-upr.md`,
  indexado em `docs/adr/README.md`, registrando: o mecanismo escolhido
  (`(Accepted[R], *Rejection)` com tipo concreto, e não `error`); a tabela de
  equivalência observável de FND-03 §3.4 preenchida linha a linha com o que o
  código faz; decide-sobre-cópia como realização de `DEC-10`; cópia defensiva
  como realização de `DEC-12`/`DEC-13`; o instante como valor sem `time` e a
  razão (classificação `io.clock` do verificador); `forbidigo` como complemento
  de símbolo; e a pendência de adoção entre bounded contexts (ver "Escopo fora")
  como consequência declarada, não decidida.
- [x] **[P1] Documentação do workspace**: `AGENTS.md` deixa de dizer "Uma:" em
  Libs (linha 141) e passa a inventariar `dmpf-domain-go` (com o exemplo) e
  `dmpf-conformance-go`, e registra na cadeia Go que a regra `domain` do lint
  tem `depguard` e `forbidigo`.

### Não-funcionais

- [x] Sem I/O em teste: `go test ./...` do módulo termina em menos de 2 s a
  frio e não cria arquivo, socket ou processo — verificável por `strace -f -e
  trace=network,file` devolvendo apenas o acesso do próprio `go test` aos
  binários de teste, ou pela simples ausência de `os`, `net`, `os/exec` e
  `testing.T.TempDir` no fechamento (a metade estática já o garante).
- [x] Superfície mínima do package raiz: exatamente estes identificadores
  exportados — `DomainEvent`, `Accepted`, `Accept`, `Empty`, `Rejection`,
  `Reject`, `Code`, `Detail`. Qualquer acréscimo é decisão de spec, não de
  implementação.
- [x] `go.mod` do módulo sem `require`; `go.sum` ausente.
- [x] `go test -race -count=2 -shuffle=on ./...` verde: nenhuma corrida e nenhum
  teste dependente de ordem.
- [x] Cadeia Go verde por `pnpm nx`: `fmt-check`, `vet`, `lint`, `build`, `test`,
  `test-race`, `govulncheck` do projeto `dmpf-domain-go`.

## Camadas afetadas

| Camada | Impacto |
| --- | --- |
| **Módulo Go `dmpf-domain`** | Placeholder substituído pelo kernel de domínio (package raiz) e pelo agregado de exemplo (`example/orders`) |
| **Governança (manifesto + baseline)** | Unidade nova `dmpf-kernel/example-orders`; baseline regravado; commit próprio |
| **Lint** | `.golangci.yml` ganha `forbidigo` na regra `domain`; `tools/dmpf-gate-check.sh` ganha o vetor correspondente |
| **CI** | Nenhuma mudança: o gate do verificador já roda para projetos Go afetados (`ci.yml:94-97`) e a cadeia Go já cobre o módulo |
| **Nx** | Nenhuma mudança em `project.json` ou `nx.json`: targets inferidos e declarados pelo `KRN-01` bastam |
| **Documentação** | ADR-032, índice de ADRs, `AGENTS.md` |

Nenhuma camada de produto é afetada: não há app, endpoint, banco, fila, porta ou
provider nesta entrega.

## Localização de código

```text
lidercap-platform/
├── libs/backend/go/dmpf-domain/                 # projeto Nx dmpf-domain-go — EXISTE
│   ├── go.mod                                   # INTOCADO — module .../libs/backend/go/dmpf-domain, go 1.26.4
│   ├── package.json                             # INTOCADO — private: true
│   ├── project.json                             # INTOCADO — tags 3D e targets Go
│   ├── dmpf-units.json                          # MODIFICAR — 2ª unidade dmpf-kernel/example-orders (commit próprio)
│   ├── dmpf-domain.go                           # REMOVER — placeholder DmpfDomain do KRN-01
│   ├── dmpf-domain_test.go                      # REMOVER — teste do placeholder
│   ├── doc.go                                   # CRIAR — package dmpfdomain: o que é e o que não é (FND-03 §2.1)
│   ├── event.go                                 # CRIAR — DomainEvent
│   ├── decision.go                              # CRIAR — Accepted[R], Accept, Empty
│   ├── rejection.go                             # CRIAR — Rejection, Reject, Code, Detail
│   ├── decision_test.go                         # CRIAR — DEC-05, DEC-12, DEC-13
│   ├── rejection_test.go                        # CRIAR — Code.Valid, Details cópia, errors.As
│   └── example/orders/                          # CRIAR — unidade dmpf-kernel/example-orders
│       ├── order.go                             # Order, NewOrder, Snapshot, clone
│       ├── messages.go                          # AddItem, PlaceOrder, ItemAdded, OrderPlaced, ItemAccepted, PlacedResponse, Instant
│       ├── rejections.go                        # constantes Code orders/*
│       ├── add_item.go                          # UPR 1
│       ├── place.go                             # UPR 2
│       ├── add_item_test.go                     # ambos os ramos, DEC-10, DEC-11
│       ├── place_test.go                        # ambos os ramos, DEC-10, DEC-11
│       ├── determinism_test.go                  # §8.2 em Go
│       └── rejections_test.go                   # todos os códigos declarados são Valid
├── tools/dmpf-baseline/units-baseline.json      # MODIFICAR — regravado por --write-baseline (commit próprio)
├── tools/dmpf-gate-check.sh                     # MODIFICAR — vetor de símbolo (forbidigo)
├── .golangci.yml                                # MODIFICAR — forbidigo na regra domain
├── docs/adr/032-realizacao-go-do-desfecho-da-upr.md   # CRIAR
├── docs/adr/README.md                           # MODIFICAR — linha do ADR-032
├── AGENTS.md                                    # MODIFICAR — inventário de libs e cadeia Go
├── libs/backend/go/dmpf-conformance/            # INTOCADO — o verificador é instrumento, não objeto
└── docs/dmpf/                                   # INTOCADO — acervo normativo
```

Import paths canônicos (as `canonical_key` das duas unidades):

- `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain`
- `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders`

**Arquivos a modificar**:

- `libs/backend/go/dmpf-domain/dmpf-units.json` — acrescenta a unidade
  `dmpf-kernel/example-orders`, porque `include` casa por import path exato e um
  package não listado reprova com `DMPF-U001`.
- `tools/dmpf-baseline/units-baseline.json` — ganha a entrada correspondente,
  porque baseline divergente reprova com `DMPF-T001`.
- `.golangci.yml` — `forbidigo` como segunda camada da regra `domain`, com
  vetor em `tools/dmpf-gate-check.sh`.
- `AGENTS.md` e `docs/adr/README.md` — inventário e índice.

## Design

### Arquitetura

```text
libs/backend/go/dmpf-domain  (ownership_module; bounded_context: dmpf-kernel)

  ┌──────────────────────────────────────────────┐
  │ dmpfdomain  (unidade dmpf-kernel/domain)     │  block: domain
  │  DomainEvent · Accepted[R] · Accept · Empty  │  imports: errors, fmt, regexp, slices
  │  Rejection · Reject · Code · Detail          │
  └──────────────────────▲───────────────────────┘
                         │ domain → domain, mesmo bounded_context (célula 1 + C2: permitida)
  ┌──────────────────────┴───────────────────────┐
  │ orders  (unidade dmpf-kernel/example-orders) │  block: domain
  │  Order · AddItem · Place · eventos · códigos │  imports: dmpfdomain, slices
  └──────────────────────────────────────────────┘

  Fora do módulo, nada é importado. Nenhuma unidade port, provider, contract,
  application ou app existe neste módulo.
```

O verificador do `KRN-02` vê exatamente duas unidades, uma aresta interna
permitida e um fechamento transitivo de stdlib inteiramente `pure` pela tabela
de `internal/rule/stdlib.go`.

### Fluxo principal — uma execução de UPR

1. O application service (fora deste módulo; `KRN-04`) resolve instante e
   identificadores, carrega o `*Order` e monta o command com valores prontos
   (FND-03 §4.2, passos 2 e 4).
2. Chama `acc, rej := order.AddItem(cmd)`. A chamada é síncrona e não recebe
   contexto (`FRT-03`).
3. A UPR clona o estado (`next := o.clone()`), avalia cada invariante sobre a
   cópia e, na primeira violada, devolve `(Accepted[R]{}, Reject(code, msg,
   details...))` sem tocar `*o`.
4. Se nenhuma invariante é violada, aplica a mutação na cópia, substitui o
   receptor (`*o = next`) e devolve `(Accept(response, event), nil)`.
5. O chamador olha os dois valores: `rej != nil` é `Rejected`; caso contrário lê
   `acc.Response()` e `acc.Events()`. Nada é lançado, nada fica pendente no
   agregado, nada muda depois do retorno.

### Pseudocódigo — `AddItem`

```text
func (o *Order) AddItem(cmd AddItem) (Accepted[ItemAccepted], *Rejection):
    next := o.clone()                                  // decide sobre cópia
    if next.status != Open:
        return zero, Reject(CodeOrderNotOpen, "order is not open")
    if len(next.items) + 1 > next.itemLimit:
        return zero, Reject(CodeItemLimitExceeded, "item limit exceeded",
                            Detail{"limit", itoa(next.itemLimit)},
                            Detail{"attempted", itoa(len(next.items) + 1)})
    next.items = append(next.items, Item{cmd.SKU, cmd.Quantity})
    *o = next                                          // efetiva só no aceite
    return Accept(ItemAccepted{Order: o.id, Items: len(o.items)},
                  ItemAdded{Order: o.id, SKU: cmd.SKU, Quantity: cmd.Quantity, At: cmd.At}),
           nil
```

`clone()` copia o slice de itens (`slices.Clone`), de modo que `next` e `*o` não
compartilham backing array — sem isso, o `append` sobre a cópia poderia escrever
no array do original quando houver capacidade sobrando, e `DEC-10` quebraria sem
quebrar teste algum que só olhasse `len`.

### Equivalência observável — FND-03 §3.4 realizada em Go

| Observação na fronteira | Realização | Verificação |
| --- | --- | --- |
| Ramo distinguível sem inspecionar texto | `rej != nil` | teste dos dois ramos |
| Resposta de domínio acessível em `Accepted` | `acc.Response()` | teste |
| Sequência ordenada de eventos acessível; vazia em `Rejected` | `acc.Events()`; valor zero devolve slice vazio | teste `DEC-11` |
| Rejeição tipada com código estável acessível | `rej.Code()` do tipo `Code` | teste + `Code.Valid()` |
| Estado inalterado em `Rejected` | decide-sobre-cópia | teste `DEC-10` via `Snapshot().Equal` |
| Chegada por canal indistinguível de falha técnica: proibido | segundo retorno é `*Rejection`, não `error`; `forbidigo` barra `errors.New`/`fmt.Errorf` | compilação + lint |
| Chegada por lançamento: proibido | nenhum `panic(` no módulo | `forbidigo` |
| Mesmo evento por segundo caminho: proibido | agregado sem coleção pendente | revisão estrutural (superfície fixada) |
| Coleção pendente após o retorno: proibido | idem | idem |
| Meio de alterar o desfecho depois de produzido: proibido | campos não exportados; `Events()`/`Details()` devolvem cópia | teste `DEC-12`/`DEC-13` |
| Exaustividade verificável no chamador | dois valores, um deles zero | teste + ADR-032 |

## Decisões técnicas

- **Par `(Accepted[R], *Rejection)` com tipo concreto no segundo retorno**, e
  não `(Accepted[R], error)`, porque o ADR-018 admite o par de Go apenas se o
  canal de erro transportar **somente** rejeições de domínio — e o único jeito de
  o compilador garantir isso é o tipo. Com `error`, um `fmt.Errorf` vazaria
  falha técnica pelo mesmo canal (risco 1 do ticket). Alternativa descartada:
  `error` com `errors.As` no chamador — a garantia viraria convenção. Alternativa
  descartada pela guarda-chuva: struct com discriminante — menos idiomática e não
  melhora a exaustividade. `*Rejection` implementa `error` para que o
  application service continue idiomático **ao embrulhar**, não ao receber.
- **`Accepted[R]` genérico na resposta, `[]DomainEvent` na sequência**, porque a
  resposta tem tipo por UPR (§8.2: `ItemAdicionado{...}`) e o compilador deve
  pegá-lo, enquanto a sequência mistura tipos de evento e só precisa da ordem e
  do nome. Alternativa descartada: parâmetro de tipo também para o evento —
  forçaria um tipo por UPR ou uma união artificial.
- **Decide-sobre-cópia, efetiva-se-no-aceite** como realização de `DEC-10`, em
  vez de "validar tudo antes de mutar": a segunda forma é correta hoje e frágil
  ao próximo invariante intercalado; a primeira torna a recusa incapaz de tocar
  o receptor por construção. Custo aceito: uma cópia do agregado por chamada,
  irrelevante para um agregado em memória.
- **Cópia defensiva em `Accept`, `Events()` e `Details()`** como realização de
  `DEC-12`/`DEC-13`, porque Go não tem slice imutável e a norma exige
  comportamento, não mecanismo. Alternativa descartada: tipo `EventSequence`
  com `Len()`/`At()`/`All() iter.Seq` — mais cerimônia para a mesma garantia;
  fica disponível como evolução se o `KRN-04` precisar iterar sem alocar.
- **`Instant` como valor de domínio inteiro, sem `time`**, porque o verificador
  classifica `time` inteiro como `io.clock` e reprova o import com `E001`. O
  depguard local permite `time` "como tipo", mas o gate autoritativo é o
  verificador, e esta spec não muda a tabela dele. Alternativa descartada:
  pedir reclassificação de `time` para `pure` no `KRN-02` — reabriria decisão
  registrada no ADR-031 e é assunto de spec própria.
- **Agregado de exemplo em package exportado do mesmo módulo e do mesmo
  `bounded_context`** (`example/orders`), porque o `KRN-04` é outro módulo e não
  poderia importar `internal/`; um módulo à parte traria manifesto, baseline,
  `package.json` e `go.work` só para um exemplo; e um `bounded_context` de
  negócio (`orders`) faria `orders → dmpfdomain` reprovar com `D002`. O nome do
  contexto no código de rejeição (`orders/...`) é o contexto do **exemplo** e não
  altera o `bounded_context` declarado.
- **`forbidigo` como segunda camada da regra `domain`**, porque `depguard` e
  verificador decidem por package e ambos declaram não distinguir `time.Now()` de
  `time.Duration` nem `fmt.Println` de `fmt.Sprintf`. Alternativa descartada:
  teste estrutural com `go/ast` dentro do módulo — `go/ast` e `go/parser` não
  estão na allowlist do `depguard` e o lint reprovaria o próprio teste.
- **`Code` validado por método e por teste, não por construtor que falha**,
  porque o construtor não pode devolver `error` (canal fechado) nem `panic`
  (proibido). Custo aceito: um código malformado só é pego pelo teste que
  enumera as constantes — por isso o teste é requisito, não sugestão.
- **Manifesto e baseline em commit próprio**, porque RFC §10.2 e
  `authorization.go` exigem que a mudança normativa venha isolada do código;
  este é o mecanismo mínimo vigente enquanto o ADR-028 permanece definido e não
  vigente.
- **Remover o placeholder em vez de mantê-lo**, porque a SPEC-MQA5HAXF o definiu
  como conteúdo mínimo a ser substituído, e um símbolo exportado sem sentido de
  domínio contraria a superfície mínima exigida.

## Regras relacionadas

- `AGENTS.md` — taxonomia de tags 3D; vedação a redeclarar targets; Conventional
  Commits em PT-BR com scope igual ao projeto Nx (`dmpf-domain-go`); git-flow com
  `master` e `develop` protegidas
- `.claude/rules/process-enforcement.md` — cadeia de validação Go
- `.claude/rules/git-safety.md` — hooks, revisão de diff, testes nunca alterados
  para passar
- SPEC-YRJRADY9 — guarda-chuva; esta spec é a primeira metade do Incremento 2
- SPEC-MQA5HAXF — módulo, tags, cadeia e `bounded_context: dmpf-kernel`
- SPEC-WTAXFV8B — verificador; `include` exato, baseline, `T001`/`T002`

## Verificação e testes

### Critérios de aceite

- [x] `go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root .
  --base origin/develop` sai com código 0 e nenhum diagnóstico `DMPF-D001`,
  `DMPF-E001`, `DMPF-E002`, `DMPF-M002`, `DMPF-U001`, `DMPF-T001` ou `DMPF-T002`
  para as unidades do módulo.
- [x] `go list -deps ./...` executado em `libs/backend/go/dmpf-domain` não lista
  `time`, `context`, `os`, `net`, `net/http`, `math/rand`, `crypto/rand`,
  `encoding/json`, `log`, `log/slog`, `reflect` nem package fora da stdlib.
- [x] Vetor `net/http`: cópia temporária do módulo com `import _ "net/http"` em
  `example/orders` reprova no verificador com `DMPF-E001` (`Target: net/http`) e
  em `bash tools/dmpf-gate-check.sh` pelo `depguard`. Evidência anexada ao PR;
  a fixture permanente vive na suíte do `KRN-02`
  (`integration_test.go` `TestArestaTransitivaEntreModulos`).
- [x] Vetor V07: cópia temporária do manifesto com `public_integration_surface:
  true` na unidade `dmpf-kernel/example-orders` reprova no verificador com
  `DMPF-M002`. Evidência anexada ao PR; a fixture permanente é
  `rule/vectors_test.go:244`.
- [x] Vetor V13 propriamente dito (`domain → provider`, `DMPF-D001`) segue
  coberto por `rule/vectors_test.go:292` e
  `TestArestaInternaEntreModulosProduzD001`; nenhuma unidade `provider` existe no
  módulo para exercitá-lo aqui.
- [x] Teste de determinismo: para cada UPR, dois `*Order` construídos do mesmo
  snapshot e o mesmo command produzem `Accepted` com `Response()` iguais e
  `Events()` de mesmo comprimento, mesmos tipos, mesmos campos e mesma ordem.
- [x] Teste `DEC-10`: para cada código de rejeição de cada UPR, `Snapshot()`
  antes e depois da chamada são `Equal`.
- [x] Teste `DEC-11`: para cada recusa, `len(acc.Events()) == 0` e `acc` é o
  valor zero.
- [x] Teste `DEC-12`/`DEC-13`: mutar o slice devolvido por `Events()` (e por
  `Details()`) não altera uma segunda chamada.
- [x] Nenhuma assinatura exportada dos dois packages recebe `context.Context`,
  relógio, gerador de identificador ou aleatoriedade, e nenhuma devolve `error`:
  saída de `go doc -all` dos dois packages anexada ao PR e conferida na revisão;
  `context` ausente da allowlist faz o `lint` reprovar qualquer tentativa.
- [x] O agregado não expõe método que devolva eventos pendentes; eventos aparecem
  apenas em `Accepted` (conferido na mesma saída de `go doc -all`).
- [x] `grep -rniE 'exactly.once' libs/backend/go/dmpf-domain docs/adr/032-*.md`
  devolve zero ocorrências.
- [x] Todas as constantes `Code` do exemplo passam em `Code.Valid()`; os
  negativos listados nos requisitos falham.
- [x] `pnpm nx run dmpf-domain-go:lint` reprova, num arquivo temporário sob o
  módulo, `time.Now()`, `errors.New("x")`, `fmt.Errorf("x")` e `panic("x")`, e
  `bash tools/dmpf-gate-check.sh` prova o vetor `time.Now()` em cada execução.
- [x] `dmpf-units.json` declara exatamente 2 unidades; `--write-baseline`
  executado uma segunda vez não produz diff.
- [x] `git log origin/develop..HEAD -- libs/backend/go/dmpf-domain/dmpf-units.json
  tools/dmpf-baseline/units-baseline.json` mostra um único commit, e esse commit
  não toca arquivo `.go`.
- [x] `DmpfDomain` não existe mais em código nem em manifesto
  (`grep -rn DmpfDomain --include='*.go' --include='*.json'` vazio); esta spec e
  o ADR-032 continuam citando o nome como histórico.
- [x] Cadeia Go verde: `pnpm nx run-many -t fmt-check,vet,lint,build,test,test-race,govulncheck
  -p dmpf-domain-go`; `pnpm nx affected -t lint,typecheck,test,build
  --exclude=@nx-base-template/source` verde; `pnpm biome ci .` verde nos arquivos
  desta entrega.
- [x] `docs/adr/032-realizacao-go-do-desfecho-da-upr.md` existe, indexado em
  `docs/adr/README.md`, com a tabela de §3.4 preenchida.
- [x] `AGENTS.md` inventaria as duas libs Go e a segunda camada do lint.
- [x] Revisão PT-BR da spec, do ADR e do `AGENTS.md` sem erros.

### Cenários de teste

```text
DADO um Order aberto com limite 3 e 2 itens, construído com NewOrder e dois AddItem
QUANDO AddItem{SKU: "ABC", Quantity: 1, At: orders.Instant(1755432000)} é executado
ENTÃO rej é nil, acc.Response() é ItemAccepted{Order: "P-100", Items: 3},
      acc.Events() tem exatamente 1 elemento ItemAdded{Order: "P-100", SKU: "ABC",
      Quantity: 1, At: orders.Instant(1755432000)} e Snapshot().Items tem 3 itens

DADO um Order aberto com limite 3 e 3 itens
QUANDO AddItem{SKU: "XYZ", Quantity: 1, At: ...} é executado
ENTÃO rej.Code() == "orders/item-limit-exceeded", rej.Details() == [{limit 3} {attempted 4}],
      acc é o valor zero, len(acc.Events()) == 0 e Snapshot() antes e depois são Equal

DADO dois Order construídos do MESMO snapshot (id P-100, aberto, 2 itens, limite 3)
QUANDO o mesmo AddItem é executado em cada um
ENTÃO os dois Accepted têm Response() iguais e Events() iguais elemento a elemento,
      e os dois Snapshot() finais são Equal

DADO um Order aberto sem itens
QUANDO Place{At: ...} é executado
ENTÃO rej.Code() == "orders/empty-order", nenhum evento, status permanece Open

DADO um Order aberto com 1 item
QUANDO Place{At: ...} é executado
ENTÃO rej é nil, acc.Events() == [OrderPlaced{Order: "P-100", Items: 1, At: ...}],
      Snapshot().Status == Placed

DADO um Order já Placed
QUANDO AddItem ou Place é executado
ENTÃO rej.Code() == "orders/order-not-open" e Snapshot() é Equal ao anterior

DADO um Accepted produzido por Accept(resp, e1, e2)
QUANDO o chamador faz evs := acc.Events(); evs[0], evs[1] = evs[1], evs[0]; evs = append(evs, e3)
ENTÃO acc.Events() devolve [e1, e2], na ordem original, com comprimento 2

DADO um *Rejection devolvido por uma UPR e embrulhado pelo chamador com errors.Join(rej)
QUANDO errors.As(err, &target) é executado com target *dmpfdomain.Rejection
ENTÃO target.Code() é o código original

DADO uma cópia temporária do módulo com `import _ "net/http"` em example/orders
QUANDO o verificador roda sobre ela
ENTÃO o relatório contém DMPF-E001 com Target net/http e o gate sai com código 1

DADO um arquivo temporário sob libs/backend/go/dmpf-domain com `_ = time.Now()`
QUANDO pnpm nx run dmpf-domain-go:lint é executado
ENTÃO o forbidigo reprova apontando o símbolo, e tools/dmpf-gate-check.sh registra o vetor como reprovado
```

<critical_constraints>
- [P0] Domínio sem I/O (P0-1; RFC §6.2; `UPR-I07`): o fechamento transitivo de
  imports das unidades do módulo contém APENAS capability `pure` segundo a tabela
  do verificador. NUNCA importar `time`, `context`, `os`, `net/*`, `math/rand`,
  `crypto/rand`, `encoding/json`, `log`, `log/slog`, `reflect` nem módulo de
  terceiro.
- [P0] Determinismo (FND-03 §2.4): mesma entrada → MESMA variante, MESMA resposta,
  MESMA sequência de eventos na MESMA ordem. Nenhuma UPR obtém tempo,
  identificador ou aleatoriedade por conta própria nem recebe `Clock`,
  `IdGenerator` ou porta, ainda que sob interface (ADR-016; RFC §9.2).
- [P0] Pós-condição da recusa (`DEC-10`, `DEC-11`): após `Rejected`, o estado do
  agregado é estruturalmente IGUAL ao anterior e a sequência de eventos é VAZIA.
  Não existe coleção de eventos pendentes no agregado (`DEC-07`, `DEC-08`).
- [P0] Canal de erro fechado (ADR-018; `DEC-04`): o segundo retorno de toda UPR é
  o tipo concreto `*Rejection`, NUNCA `error`. Nenhum símbolo exportado devolve
  `error`. Nenhuma assinatura exportada recebe `context.Context` (`FRT-03`).
- [P0] Imutabilidade (`DEC-12`, `DEC-13`): `Accepted` e `Rejection` não expõem
  operação que altere o desfecho; toda sequência devolvida é cópia.
- [P0] Classificação declarada (ADR-012; RFC §3.3, §10.1): cada package de
  produção é uma unidade com `include` por import path exato no manifesto e
  entrada no baseline, `block: domain`, `bounded_context: dmpf-kernel`. A
  mudança de classificação vem em COMMIT PRÓPRIO (RFC §10.2).
- [P0] Nenhum artefato declara nem sugere exactly-once fim a fim (P0-3, V31);
  determinismo NÃO é idempotência.
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva: divergência
  vira ADR-032, nunca alteração silenciosa da norma.
- [P1] A `Rejection` NÃO carrega código HTTP, gRPC, política de retry nem causa
  técnica encadeada (FND-03 §3.3).
- [P1] `go.mod` sem `require`; `go.sum` ausente.
</critical_constraints>

## Escopo fora

- **Unit of Work, transação, persistência e outbox**: `KRN-04` e `KRN-06`. Esta
  spec entrega o passo 5 da sequência canônica e nada dos passos 1–4 e 6–10.
- **Conversão de evento de domínio para integration event**: o ADR-019 a põe
  fora do domínio; o wire é do `KRN-05`. `ItemAdded` e `OrderPlaced` são eventos
  de domínio e não atravessam inteiros (`CTR-05`).
- **Taxonomia de erros de borda e mapeamento de `DomainRejection` para
  protocolo**: FND-07. O domínio não conhece 422 (FND-03 §3.6).
- **Event Sourcing e CQRS físico**: extensões opt-in (FND-03 §7); o agregado
  de exemplo é de estado, não de eventos.
- **Lado TypeScript e fixtures pareadas**: `KRN-11`. A paridade é conceitual
  (princípio 10); nenhuma abstração é introduzida aqui para facilitá-la.
- **Vocabulário de tempo do kernel**: `Instant` é local ao exemplo. Um tipo de
  instante compartilhado, e o relógio como porta, são de `KRN-04`/`dmpf-ports`.
- **Adoção do kernel por bounded contexts de negócio**: pela C2 (RFC §5.5;
  ADR-017), um `domain` de contexto `orders` não pode importar
  `dmpf-kernel/domain` (`DMPF-D002`), e uma unidade `domain` nunca é superfície
  pública (`DMPF-M002`). Esta história fica inteira em `dmpf-kernel` e o problema
  não a alcança, mas ele alcança todo consumidor real do kernel. É decisão da
  fundação (ADR novo sobre kernel compartilhado, ou revisão da C2), registrada
  como consequência no ADR-032 e a ser levada à guarda-chuva SPEC-YRJRADY9 —
  nunca resolvida aqui por reclassificação ou exceção.
- **Divergência doc/código sobre `time` no `KRN-02`**: `stdlib.go:67-73` e
  `docs/adr/README.md:130` dizem "`time` puro"; o código e o teste dizem
  `io.clock`. Corrigir a prosa é tarefa do `KRN-02`, não desta spec, que apenas
  se conforma ao código.
- **Reclassificar `time` ou `context` no verificador**: qualquer mudança na
  tabela de capabilities da stdlib é decisão do ADR-031 e do `KRN-02`.
- **Alterar `project.json`, `nx.json` ou `ci.yml`**: a cadeia e o gate já
  cobrem o módulo; `-shuffle=on -count=2` fica na invocação da suíte, não no
  target, para não redeclarar o que o `KRN-01` fixou.
- **Suporte a `IdGenerator`, `Clock` ou qualquer porta no domínio**: proibido
  pela fundação (ADR-014, ADR-016), não adiado.
