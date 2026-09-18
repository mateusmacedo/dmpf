# Guia prático: compor um bounded context sobre o kernel DMPF

Este guia descreve o caminho curto para criar um bounded context novo: escrever
a definição, deixar o harness produzir o esqueleto e o código, e conduzir à mão
os passos que o agente não pode executar.

O caminho longo — gerar o esqueleto e preencher cada bloco manualmente — está em
[`dmpf-implementation.md`](./dmpf-implementation.md). Os dois produzem a mesma
coisa; o que muda é quem escreve o código. As normas que ambos respeitam vivem
em [`docs/dmpf/`](../dmpf/) e nos ADRs; este guia é operacional e não normativo.

## Índice

1. [Quando usar o harness](#1-quando-usar-o-harness)
2. [Passo 1 — escrever a definição](#2-passo-1--escrever-a-definição)
3. [Passo 2 — invocar o command](#3-passo-2--invocar-o-command)
4. [Passo 3 — o rito Buf](#4-passo-3--o-rito-buf)
5. [Passo 4 — classificar](#5-passo-4--classificar)
6. [Passo 5 — validar](#6-passo-5--validar)
7. [O que o agente nunca toca](#7-o-que-o-agente-nunca-toca)
8. [A prova de regressão](#8-a-prova-de-regressão)
9. [Divergir do golden path](#9-divergir-do-golden-path)
10. [Como certificar](#10-como-certificar)
11. [Consumir os módulos fora do workspace](#11-consumir-os-módulos-fora-do-workspace)
12. [Fontes normativas](#12-fontes-normativas)

---

## 1. Quando usar o harness

O harness serve para um contexto **novo**, escrito do zero sobre o kernel. Ele
não migra contexto existente, não altera o kernel e não decide domínio: a
modelagem é sua, escrita na spec, e o agente a realiza nos cinco blocos.

Os cinco artefatos que o compõem:

| Artefato | Papel |
| --- | --- |
| `.claude/rules/dmpf-bounded-context.md` | As normas curtas, com referência ao ADR de cada uma |
| `.agents/skills/dmpf-bounded-context/` | O golden path em doze passos e o catálogo de armadilhas |
| `.claude/agents/dmpf-context-author.md` | O agente que escreve o contexto e para em gate normativo |
| `.claude/commands/dmpf-new-context.md` | Valida a spec e invoca o agente |
| `tools/dmpf-harness-check.sh` | A prova de regressão do próprio harness |

O golden de referência é `bookings` (`apps/backend/bookings/`), gerado a
partir de `docs/specs/SPEC-AHPRBZCT-bookings.md`. Quando algo neste guia parecer
ambíguo, o golden é a resposta.

## 2. Passo 1 — escrever a definição

A spec é a entrada do harness, e o template é obrigatório:
`.agents/skills/dmpf-bounded-context/references/template-bounded-context.md`.

São **dez seções**, todas exigidas: `Identidade`, `Agregados`,
`Comandos (UPRs)`, `Eventos de domínio`, `Consultas`,
`Relações entre agregados`, `Integração`, `Políticas transversais`,
`Critérios de aceite` e `Escopo fora`.

Reserve o identificador antes de escrever, para não colidir com outra spec em
curso, e declare `stage: planning` no frontmatter — o command recusa spec em
qualquer outro estado.

O que a definição precisa fixar, porque o agente não inventa:

- **Identidade do contexto** — o `bounded_context` é estável e renomeá-lo é
  rito próprio (ADR-017). Escolha com cuidado.
- **Cada UPR pelas pré-condições**, com o código de rejeição literal que ela
  devolve (`<ctx>/<agregado>/<motivo>`). É o que vira teste.
- **Cada consulta com filtros e cardinalidade.** Uma consulta que atravessa
  relação vira porta no bloco `port`; as demais saem dos genéricos do kernel.
- **Os cenários**, em `Critérios de aceite`. Cada cenário deve virar um teste
  nomeado — é assim que se confere depois se a entrega fechou.

## 3. Passo 2 — invocar o command

```bash
/dmpf-new-context SPEC-<id>
```

O command resolve a spec pelo catálogo, confere o `stage` e as dez seções, e só
então invoca o agente. Spec incompleta é recusada **nomeando a seção ausente**,
sem escrever nada:

```text
seção ausente: Comandos (UPRs)
```

O agente então percorre o golden path: roda o generator para o esqueleto do
módulo — um package por bloco —, escreve o domínio por agregado, as portas, os casos de uso, o
provider com o esquema e os repositórios, e a borda do bloco `app`. Ele para em
qualquer gate normativo em vez de contornar — reprovação do verificador,
do `depguard` ou do rito Buf interrompem a execução e são reportadas.

O contexto nasce em `apps/<scope>/<ctx>` — um contexto de negócio é uma app
(`type:app`), não uma lib: em `libs/backend/go` fica só o kernel de reuso. É um
módulo Go por contexto, um package por bloco (`<ctx>/domain`, `<ctx>/ports`,
`<ctx>/application`, `<ctx>/provider`, `<ctx>/app`). O nome do projeto Nx é
`<ctx>`, sem prefixo nem sufixo: a árvore já diz o scope (ADR-045). Onde um
arquivo importa o package do kernel e o do contexto com o mesmo nome, o import
do kernel recebe alias pelo papel — `kernel`, `usecase`, `port`.

## 4. Passo 3 — o rito Buf

O `.proto` do contexto é escrito pelo agente; o código gerado, não. `gen/go` só
é escrito pelo rito:

```bash
cd contracts && bash ../tools/buf.sh generate
```

Depois, os quatro gates:

```bash
pnpm nx run contracts:buf-lint
pnpm nx run contracts:buf-pins
pnpm nx run contracts:buf-generate-check
NX_BASE=<ref> pnpm nx run contracts:buf-breaking
```

O `buf-generate-check` gera duas vezes e compara com o versionado: é ele que
pega drift entre a definição e o gerado.

Editar um `.pb.go` à mão não é apenas desaconselhado — não funciona. O `rawDesc`
é o descritor do `.proto` serializado, com prefixos de comprimento; qualquer
alteração textual que mude o tamanho de um campo corrompe o descritor e o pacote
entra em `panic` no `init()`.

## 5. Passo 4 — classificar

As unidades do contexto novo precisam entrar no baseline governado:

```bash
go run ./tools/dmpf-conformance/cmd/conformance --write-baseline
```

Este é um **passo humano**, nunca do agente (ADR-028): classificar é ato de
autoridade sobre a arquitetura, não consequência de escrever código.

O commit da classificação vai **sozinho**. `DMPF-T002` reprova o commit que
mistura mudança normativa com código, e "normativo" inclui tanto o baseline
quanto o `dmpf-units.json` do módulo:

```bash
git add tools/dmpf-baseline/units-baseline.json '**/dmpf-units.json'
git commit -m "chore(workspace): classificar as unidades de <ctx>"
```

## 6. Passo 5 — validar

O módulo novo não é importer do pnpm — `apps/backend/<ctx>` fica fora dos
globs de `pnpm-workspace.yaml` (`apps/*`) — e só o Nx o lê, então não há
`pnpm install` no rito:

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test -p <ctx>
```

Os testes que tocam Postgres levam a build tag `integration` e exigem o DSN.
Rode-os com `--parallel=1`: os harnesses truncam as tabelas do kernel, que todo
contexto compartilha.

```bash
pnpm nx run bff:infra-up
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' \
  pnpm nx run <ctx>:test-race
```

Por fim, o gate autoritativo entre módulos, com a base do intervalo em revisão:

```bash
go run ./tools/dmpf-conformance/cmd/conformance -base <ref>
```

Sem `-base`, a condição de commit próprio fica **não verificada** — e condição
não verificada nunca vira "conforme".

## 7. O que o agente nunca toca

| Artefato | Quem escreve | Por quê |
| --- | --- | --- |
| `dmpf-units.json`, `project.json`, `go.mod`, `go.work` | generator | Esqueleto é gerado; `include` entra por merge de campo |
| `gen/go` | rito Buf | Código gerado só se conclui regerando |
| `tools/dmpf-baseline/units-baseline.json` | humano | Classificar é ato de autoridade (ADR-028) |
| Qualquer arquivo do kernel | ninguém, neste fluxo | O contexto consome o kernel; não o altera |

Nenhum gate é afrouxado para acomodar o contexto novo. Se um gate reprova, a
resposta é corrigir o contexto — não relaxar o gate.

## 8. A prova de regressão

O harness tem uma prova própria, em duas fases:

```bash
tools/dmpf-harness-check.sh --phase self-test
tools/dmpf-harness-check.sh --phase regen
```

O **`self-test`** não usa LLM. Sobre o golden commitado, num worktree
descartável, ele retira `kernel/domain` de `shared_kernel_units` e exige
que o verificador reprove com `DMPF-D002` apontando `<ctx>/domain`. É a prova de
que o gate normativo ainda morde — se ele aprovar sem o shared kernel
designado, a regra de dependência parou de valer.

O **`regen`** apaga o golden num worktree, aciona o agente sobre a mesma spec e
roda os gates sobre o resultado. A garantia é pelos gates, não por bytes
idênticos: dois runs do agente divergem em forma, e isso é aceito (ADR-041). Ele
exige LLM, credenciais e tempo, então **não roda no CI** — o CI prova o golden
como qualquer outro módulo. Rode-o depois de mudar a skill, a rule ou o agente.

Vale rodar o `self-test` a cada mudança de forma do golden: a prova carrega o
layout em `MODULO` e `BLOCOS` e nos globs de comparação, e envelhece junto com ele.

## 9. Divergir do golden path

Quando o contexto precisa de algo que a regra de dependência nega, o caminho é a
exceção nominal de `GOV-30` a `GOV-36` — nunca afrouxar o gate, nem reclassificar
a unidade para o bloco que a permitiria (`GOV-26`). A exceção vive onde o objeto
vive: dependência externa de uma unidade (E1) no `dmpf-units.json` do módulo;
combinação fora do BOM (E2) e instrumento de governança (E3) no BOM da release,
`bom/dmpf/<semver>.json`.

O pedido só é examinado se trouxer os quatro itens de `GOV-30` — ADR, equipe
dona, justificativa e plano de convergência com data — e cair fora do catálogo
fechado N1–N7 de `GOV-32`. A admissão é mecânica: o verificador emite
`DMPF-X001` a `DMPF-X007`, e só a exceção admitida autoriza o import. O schema
completo está em [`bom/README.md`](../../bom/README.md#pedir-exceção).

### Um pedido admitido

Uma unidade `contract` gerada pelo `protoc-gen-go` importa `reflect`, que o
verificador classifica como `runtime.framework` — capability que o bloco
`contract` não tem. O plugin emite esse import em todo arquivo gerado, então não
há prazo de convergência planejável, e o pedido usa o ramo de revisão, com
aprovação de Arquitetura e Plataforma (`GOV-34`):

```json
{
  "id": "X-orders-contract-reflect",
  "object": { "kind": "external-dependency", "unit": "orders/contract", "identity": "reflect" },
  "adr": "ADR-033",
  "owner": "team:tech-leads",
  "justification": "import emitido pelo protoc-gen-go em todo arquivo gerado; plumbing do runtime Protobuf",
  "convergence": {
    "kind": "review",
    "review_by": "2027-03-02",
    "approved_by": ["arquitetura", "plataforma"],
    "replanning_condition": "protoc-gen-go deixar de emitir reflect no código gerado"
  },
  "valid_from": "2026-09-12",
  "valid_until": "2027-03-02",
  "review_by": "2027-03-02",
  "history": [{ "event": "granted", "at": "2026-09-12", "by": "team:tech-leads" }]
}
```

`approved_by` é declaração: o verificador confere que as duas autoridades
constam do array, não que aprovaram. A aprovação precisa existir de fato na
revisão do PR que introduz a exceção. As exceções reais de `contracts` são
desse tipo (ADR-033).

### Um pedido recusado

A mesma forma, completa, para uma unidade `domain` que peça `net/http`:

```json
{
  "id": "X-orders-domain-net-http",
  "object": { "kind": "external-dependency", "unit": "orders/domain", "identity": "net/http" }
}
```

é recusada na admissão, sem exame de mérito. `net/http` é `io.network`, e o bloco
`domain` só admite `pure`: é constraint P0, item N1 de `GOV-32`, e nenhuma
exceção o alcança.

```text
DMPF-X003: orders/domain -> net/http: N1: o bloco domain admite apenas pure (RFC §6.2, constraint P0), e a dependência é de capability io.network [GOV-32 N1, N2]
```

A exceção recusada não autoriza nada, e o import continua reprovando em
`DMPF-E001` no mesmo relatório. `DMPF-X*` não encerra a verificação justamente
para que as duas causas apareçam juntas.

### Vencimento e renovação

Depois de `valid_until`, a exceção deixa de autorizar e a unidade fica não
conforme (`DMPF-X006`). Renovar é conceder de novo, pelo mesmo rito: um
`valid_until` novo e um evento `renewed` no `history`, com `reason` que explique
o atraso. `renewed` sem vigência nova reprova em `DMPF-X005`, porque renovação
automática não existe (`GOV-34`). Um evento `revoked` ou `converged` encerra a
exceção: ela para de autorizar, não vence mais e fica como histórico para as
métricas de `GOV-36`. `--now` fixa o instante do vencimento quando é preciso
reproduzir um relatório.

## 10. Como certificar

A certificação promove a `certificada`, no BOM da release, as entradas que uma
execução real da suíte alcançou. O instrumento é o `dmpf-evidence` do
`testkit`; as normas são `BOM-03` a `BOM-08` de
[`governanca-bom-pilotos.md`](../dmpf/governanca-bom-pilotos.md) §4, e o schema
está em [`bom/README.md`](../../bom/README.md).

1. **Gerar a evidência.** Num commit com árvore limpa, suba o Postgres e o
   Redpanda com as imagens pinadas do `dmpf-evidence.yml` e rode o comando duas
   vezes, em diretórios distintos: `diff -r` vazio prova o determinismo. Publique
   uma das execuções em `bom/evidence/<semver>/` e commite só esse diretório.

   ```bash
   CI=true GOTOOLCHAIN=go1.26.6 \
     DMPF_PG_DSN='postgres://dmpf:dmpf@localhost:5432/dmpf?sslmode=disable' \
     DMPF_KAFKA_BROKERS=localhost:9092 DMPF_REDPANDA_ADMIN=http://localhost:9644 \
     go run ./libs/backend/go/testkit/cmd/evidence --root . --release <semver> --out /tmp/evidence-a/<semver>
   ```

2. **Promover só o que o header alcança.** Uma entrada vai a `certificada`
   quando a sua `identity` e a sua `version` constam do header de um subject
   aprovado — em `goversion`, `modules`, `externals` ou `tools`. O validador
   confere o digest, não o alcance: essa é regra de quem promove. O que nenhum
   subject exercita fica `candidata`, com `reason` nomeando onde é exercitado.
   Cada `certificada` recebe:

   | Campo | Valor |
   | --- | --- |
   | `evidence_uri` | `https://github.com/mateusmacedo/dmpf/blob/<sha>/bom/evidence/<semver>/<subject>.json`, no SHA do commit da evidência |
   | `evidence_digest` | o `sha256` do subject no `index.json` |
   | `approved_by` | `team:plataforma` (`BOM-05`) |
   | `certified_at` | a data do commit de certificação |
   | `valid_until` | `certified_at` + 90 dias, o default do ADR-041 |
   | `promoted` | `{by, reviewed_by, pr}`: quem promove, quem revisou por Arquitetura e o PR da promoção |

   `compatible_with` só nomeia combinação cujos dois lados constam do mesmo
   header, com `evidence` igual ao subject.

3. **Validar.** `dmpf-bom --release <semver> --base develop` sai com `0`. Um
   digest alterado reprova em `DMPF-B005`; uma combinação sem o subject que a
   exercita, em `DMPF-B006`.

4. **Revisar e cunhar a tag.** A promoção é um PR para `develop` revisado por
   Arquitetura e mergeado por Plataforma (`BOM-05`). A mesma árvore segue por
   `release/<semver>` até `master`, e a tag anotada `dmpf@<semver>` é cunhada
   pelo workflow `dmpf-release.yml`, disparado à mão com o `<semver>` da release;
   o rito está no [`CONTRIBUTING.md`](../../CONTRIBUTING.md).

## 11. Consumir os módulos fora do workspace

Cada módulo Go do kernel é uma lib independente, publicada por tag própria
(`libs/backend/go/<módulo>/vX.Y.Z`, e `tools/dmpf-conformance/vX.Y.Z` para o
verificador). A tag do produto, `dmpf@X.Y.Z`, é outra linha e não serve ao
toolchain Go. Há dois jeitos de consumir, e a diferença está em quem resolve o
`require`.

**Por tag.** O módulo declara no `go.mod` tudo o que importa, então um
`go get` basta — os irmãos vêm junto, cada um na versão que o `require` fixa:

```bash
go get github.com/mateusmacedo/dmpf/libs/backend/go/domain@v0.1.0
```

**Por clone local**, quando você quer editar o kernel enquanto desenvolve
contra ele. O consumidor monta um `go.work` que cobre o próprio módulo e o
clone:

```bash
git clone https://github.com/mateusmacedo/dmpf.git ../dmpf
go work init . ../dmpf/libs/backend/go/domain ../dmpf/libs/backend/go/ports
```

Um `replace` versionado no `go.work` do consumidor tem o mesmo efeito e
dispensa listar cada irmão em `use`.

Duas consequências que valem a leitura antes de depender disto:

- **O `require` fica na versão mínima.** Ele não é reescrito a cada release,
  então um módulo pode usar API nova de um irmão sem subir o `require`, e você,
  consumindo por tag, compilaria contra a versão anterior. Ao mexer em um irmão,
  suba o `require` no mesmo PR.
- **Dentro do repositório, quem manda é o `go.work`.** Os `replace` versionados
  da raiz vencem os `require`, e é isso que faz o build local usar o código da
  árvore. Quem grava os dois lados é o `dmpf-modsync`, que o generator roda ao
  criar um contexto e que o CI confere com `--check` a cada PR:

```bash
go run ./tools/dmpf-conformance/cmd/modsync --root . --check
```

Decisão e alternativas descartadas em
[`docs/adr/047-tags-de-modulo-go-e-consumo-fora-do-workspace.md`](../adr/047-tags-de-modulo-go-e-consumo-fora-do-workspace.md).

## 12. Fontes normativas

- [`docs/dmpf/rfc-dmpf-foundation-v0.1.md`](../dmpf/rfc-dmpf-foundation-v0.1.md) — a RFC
- [`.claude/rules/dmpf-bounded-context.md`](../../.claude/rules/dmpf-bounded-context.md) — as normas do contexto
- [`.agents/skills/dmpf-bounded-context/references/armadilhas.md`](../../.agents/skills/dmpf-bounded-context/references/armadilhas.md) — o que já deu errado
- ADR-010, ADR-012, ADR-017 — regra de dependência, classificação por manifesto, identidade estável
- ADR-028 — classificação como ato de autoridade
- ADR-030 — granularidade de módulo do kernel e BOM
- ADR-045 — nomes sem prefixo nem sufixo e o contexto como um módulo só
- ADR-041 — a virada do generator orientado ao domínio para o híbrido generator + agente
- ADR-042 — shared kernel
- ADR-015 — política de capabilities por bloco, que decide N1 para E1
- ADR-033 — as exceções de `reflect` e `unsafe` do código gerado
- [`docs/dmpf/governanca-bom-pilotos.md`](../dmpf/governanca-bom-pilotos.md) — §5.1, o escape hatch (`GOV-30` a `GOV-36`)

<!-- A seção "Contrato próprio" pertence à SPEC-F7S5B6KV, que ainda não foi
     implementada. Quando for, entra aqui. -->
