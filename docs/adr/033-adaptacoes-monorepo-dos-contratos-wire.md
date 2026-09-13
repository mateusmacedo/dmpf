# ADR-033: Adaptar os contratos de wire à fase monorepo sem relaxar a norma

## Status

Aceito — 2026-09-03. Implementa SPEC-WYX5GW87 (`KRN-05`, ARQ-524).

## Contexto

A norma `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05) descreve o bloco
`contract` para um repositório de contratos autônomo: fonte em `contracts/`,
gerado em `contracts/gen/<stack>/` (`REP-02`, §7.1), owner efetivo por bounded
context (`REP-03`) e uma máquina de estados para `buf breaking` que distingue o
primeiro conteúdo de um módulo da sua evolução (`BUF-08`).

O `KRN-05` entrega o primeiro conteúdo desse bloco dentro do monorepo, onde três
decisões anteriores já fixaram fronteiras que a norma não previa:

- o ADR-030 exige que todo módulo Go seja um projeto Nx em
  `libs/<scope>/<stack>/<módulo>`;
- o ADR-031 entregou o verificador `dmpf-conformance`, que classifica packages
  Go a partir de um `dmpf-units.json` por módulo e nega `runtime.framework` ao
  bloco `contract`;
- a forge é o Gitea (ADR-005), que lê `CODEOWNERS` em `.gitea/` e protege tags
  por padrão de nome, não por arquivo no repositório.

O ticket pediu que as escolhas de acomodação fossem feitas no refinamento e
registradas. A spec as fixou uma a uma (`SPEC-WYX5GW87`, "Decisões técnicas") e
apontou para um único ADR, porque as quatro decorrem da mesma tensão: o gerado
precisa ser um módulo Go verificável, e a fonte precisa continuar neutra de stack.

## Decisão

Quatro adaptações, todas de forma — a semântica de cada regra da norma é
preservada e verificada pelos gates.

### A fonte fica em `contracts/`; o gerado, dentro da lib

`contracts/` na raiz guarda o que é neutro de stack: `.proto`, `buf.yaml`,
`buf.gen.yaml`, fixtures e a proveniência do envelope vendorizado. O gerado Go
fica em `libs/backend/go/dmpf-contracts/gen/go/`, onde o `buf.gen.yaml` o
emite com `go_package_prefix` gerenciado.

O que `REP-02` exige — `gen/<stack>/`, nunca editado à mão, drift reprova —
continua valendo; muda apenas o diretório-raiz do `gen/`. O motivo é o ADR-031:
o verificador classifica packages de módulos Nx, e o bloco `contract` precisa
existir como um módulo com `dmpf-units.json` para entrar no grafo. Um `gen/`
solto em `contracts/` seria um módulo Go fora do Nx, contra o ADR-030.

O projeto `dmpf-contracts-go` declara `contracts/**` nos `inputs` dos targets
`buf-*`; um PR que toque só um `.proto` torna a lib afetada e dispara os gates
pelo mesmo `nx affected` do resto do CI.

### A marca de `BUF-08` é uma tag anotada; o baseline de comparação é `NX_BASE`

A marca de baseline de um módulo Buf é a tag anotada
`contracts-baseline/<módulo>` — `contracts-baseline/proto` para o único módulo
atual. A tag anotada carrega `tagger`, isto é, a identidade de quem autorizou; o
Gitea a protege por padrão de nome. O baseline contra o qual `buf breaking`
compara é `NX_BASE`, a branch-alvo protegida do PR, já usada pelo gate de
conformidade.

`tools/buf-gate.sh breaking` reconhece os dois estados da norma pela marca:

| Estado | Condição | `breaking` |
| --- | --- | --- |
| `sem baseline` | marca ausente **e** módulo ausente em `NX_BASE` | dispensado só para esse módulo, com aviso |
| `baseline estabelecido` | marca presente | obrigatório; `NX_BASE` vazio, irresolvível ou sem `contracts/` reprova |

Os três vetores negativos de `BUF-08` são verificados mecanicamente e cobertos
pelo autoteste (`tools/tests/buf-gate/buf-gate.test.sh`): marca ausente em módulo
que já existe em `NX_BASE`; diretório de pacote publicado que muda de caminho; e
marca cujo `tagger` é o autor do primeiro commit do módulo. `lint`, `format`,
`generate-check` e `pins` são obrigatórios em ambos os estados.

Este PR entra em `sem baseline`. É a transição que a norma prevê para o primeiro
conteúdo, não uma exceção.

### `reflect` e `unsafe` do gerado entram por exceção nominal

O `protoc-gen-go` emite `reflect`, `sync` e `unsafe` em todo arquivo gerado. O
verificador classifica `reflect` e `unsafe` como `runtime.framework`
(`internal/rule/stdlib.go`), capability que a tabela de RFC §6.2 nega ao bloco
`contract`. Sem tratamento, a unidade `dmpf-contracts/gen` reprova por
`DMPF-E001`.

A saída é o instrumento que a RFC §6.4 criou para isso: uma exceção por par
(unidade, dependência), com razão, owner e `review_by`, declarada no
`dmpf-units.json` da lib — duas entradas, ambas com owner `tech-leads` e
`review_by: 2027-03-02`. O verificador fica intocado.

Pelo mesmo manifesto, `google.golang.org/protobuf` é declarado `wire.codec` com
os cinco entrypoints que o gerado e o codec importam (`proto`, `anypb`,
`timestamppb`, `protoreflect`, `protoimpl`). `wire.codec` não é submetida à
pureza transitiva, o que evita reprovar o runtime Protobuf por alcançar `reflect`.

### A marca exige uma segunda pessoa, e isso é pré-requisito, não defeito

O terceiro vetor negativo de `BUF-08` — autoria igual a autorização — só é
satisfeito se quem cria a marca não for o autor do primeiro commit do módulo.
Criar a tag é, portanto, ato operacional **pós-merge**, executado por outra
pessoa com direito de push em `contracts-baseline/*` (no Gitea: equipes
`tech-leads` e `Owners`). O procedimento está em `contracts/README.md`, "Gates e
baseline".

Um repositório com um único aprovador não consegue sair de `sem baseline`. Este
ADR registra isso como pré-requisito organizacional e não o relaxa: enquanto a
marca não existir, `breaking` segue dispensado só para `proto`, e os outros
gates seguem obrigatórios.

## Limitações declaradas

### Toolchain só por `go run`

Buf CLI (`github.com/bufbuild/buf/cmd/buf@v1.72.0`, em `tools/buf.sh`) e
`protoc-gen-go` (`@v1.36.12`, em `buf.gen.yaml`) entram pelo mesmo padrão do BOM
do ADR-030: nenhum binário instalado, pin exato em arquivo versionado. O gate
`pins` confere que a versão do plugin é igual à do runtime
`google.golang.org/protobuf` no `go.mod` (`BUF-06`). Um segundo mecanismo de
distribuição — download de release com checksum, plugin remoto do BSR — não foi
adotado.

### Um só módulo Buf

O workspace v2 lista explicitamente o módulo `proto`, que contém o envelope
vendorizado (`io.cloudevents.v1`) e o contrato de exemplo. `BUF-01` exige lista
explícita, não exige mais de um módulo; um módulo separado para o envelope
duplicaria marca, `CODEOWNERS` e estado de `BUF-08` para um arquivo cuja
evolução é externa.

### `CODEOWNERS` vive em `.gitea/`, não em `contracts/`

`REP-03` pede owner efetivo. O Gitea não lê um `CODEOWNERS` aninhado em
`contracts/`; o arquivo lido é `.gitea/CODEOWNERS`, e é lá que os padrões de
`contracts/proto/company/orders/` e `contracts/fixtures/orders/` apontam para
`@mateusmacedo/tech-leads`. Criar `contracts/CODEOWNERS` satisfaria a figura
da norma, não a regra.

### Código gerado no verificador é história futura

Se toda unidade `contract` vier a repetir as mesmas duas exceções para `reflect`
e `unsafe`, a proposta de tratar código gerado no próprio verificador é uma
história de `KRN-02`, não desta. Excluir arquivos por `// Code generated` seria
heurística textual, contra o ADR-012.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| `contracts/gen/go/` como módulo Go próprio | Quebra `libs/<scope>/<stack>/<módulo>` (ADR-030) e cria um módulo fora do Nx |
| `contracts/` inteiro dentro da lib Go | Acopla a fonte neutra de stack à stack Go e obriga o `KRN-11` a gerar TypeScript de dentro de uma lib Go |
| Branch de baseline dedicada como marca | Não carrega a identidade de quem autorizou; o vetor "autoria = autorização" fica sem evidência |
| Arquivo de marca versionado no repositório | Qualquer autor o cria no próprio PR, anulando a autorização distinta |
| Criar a marca no próprio PR | Reprova pelo terceiro vetor de `BUF-08` |
| Rodar `breaking` contra `NX_BASE` sem `contracts/` no bootstrap | Falha por irresolução — correto em `baseline estabelecido`, errado no primeiro conteúdo; é a ambiguidade que `BUF-08` elimina |
| Reclassificar `reflect`/`unsafe` como `wire.codec` ou `pure` | Reabre `KRN-02`/ADR-031 e afrouxa a tabela de capabilities para todo bloco |
| `entrypoints: ["."]` para o runtime Protobuf | Autoriza só a raiz do módulo, que ninguém importa; o gerado importa subpacotes |
| Depender do `pb` do `cloudevents/sdk-go` para o envelope | Traz um módulo grande cuja geração não passa por `BUF-11` e fixa a versão do envelope à do SDK |
| Um ADR por adaptação | Quatro documentos para uma decisão coesa |

## Consequências

- O bloco `contract` nasce sob os dois gates: o verificador `dmpf-conformance`
  (grafo de imports, três unidades `contract` declaradas) e os quatro gates Buf
  (`lint`, `pins`, `generate-check`, `breaking`), todos fail-closed e sem bypass
  (`BUF-12`).
- Enquanto a marca `contracts-baseline/proto` não for criada por uma segunda
  pessoa, `buf breaking` é dispensado para `proto` com aviso. A criação da marca
  é ato pós-merge fora deste PR; sem ela, mudanças incompatíveis em `.proto`
  passam sem reprovação de `breaking` — e por isso os demais gates não têm
  dispensa.
- Cada stack futura (`KRN-11`, TypeScript) gera a partir da mesma fonte em
  `contracts/` para o seu próprio `gen/` dentro da lib correspondente; a raiz do
  `gen/` deixa de ser `contracts/` para todas as stacks, não só para Go.
- As duas exceções nominais vencem em 2027-03-02 e precisam ser revisadas pelo
  owner; se se repetirem em cada nova unidade `contract`, isso é o sinal para
  abrir a história no `KRN-02`.
- O pin do plugin e a versão do runtime Protobuf mudam juntos, ou o gate `pins`
  reprova.

## Addendum — 2026-09-12 (as exceções do gerado pelo rito de governança)

A seção «`reflect` e `unsafe` do gerado entram por exceção nominal» registrou
duas entradas, ambas de `dmpf-contracts/gen`. O manifesto tem quatro:
`resource-scheduling/contract`, a unidade de contrato de `bookings` declarada no
mesmo `dmpf-units.json`, é gerada pelo mesmo `protoc-gen-go` e recebe as mesmas
duas exceções, pela mesma razão. Esta decisão é a proveniência das quatro.

Com a `SPEC-538MS2D4`, a exceção deixou de ser só a forma de RFC §6.4 e passou
pela admissão de `GOV-30` a `GOV-35`. As quatro migraram para o schema novo,
mantendo os campos legados ao lado:

- `id` nominal, `X-<unidade>-<dependência>` (`GOV-33`);
- `adr: ADR-033` e `owner: team:tech-leads` — a equipe, com o prefixo que
  `GOV-30` exige;
- `convergence` no ramo de revisão, com `review_by: 2027-03-02`,
  `approved_by: [arquitetura, plataforma]` e `replanning_condition` amarrada ao
  `protoc-gen-go`. O ramo de prazo não cabe: o plugin emite esses imports em todo
  arquivo, e não há data de convergência planejável;
- `history[0]: granted`.

**Ressalva.** A admissão confere que as duas autoridades constam de
`approved_by`, não que aprovaram. A aprovação dual é declaração no manifesto e
só vale se existir de fato na revisão do PR que a introduz; sem isso, o array é
autoatestação.
