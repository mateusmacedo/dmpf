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
9. [Fontes normativas](#9-fontes-normativas)

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

O golden de referência é `bookings` (`libs/backend/go/bookings/`), gerado a
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

O agente então percorre o golden path: roda o generator para o esqueleto dos
cinco módulos, escreve o domínio por agregado, as portas, os casos de uso, o
provider com o esquema e os repositórios, e a borda do bloco `app`. Ele para em
qualquer gate normativo em vez de contornar — reprovação do verificador,
do `depguard` ou do rito Buf interrompem a execução e são reportadas.

O contexto nasce em `libs/<scope>/<stack>/<ctx>/<bloco>` — uma pasta por
contexto, um subdiretório por bloco. O nome do projeto Nx continua composto e
com sufixo de stack (`<ctx>-domain-go`), porque nome de projeto é chave única
no workspace.

## 4. Passo 3 — o rito Buf

O `.proto` do contexto é escrito pelo agente; o código gerado, não. `gen/go` só
é escrito pelo rito:

```bash
cd contracts && bash ../tools/buf.sh generate
```

Depois, os quatro gates:

```bash
pnpm nx run dmpf-contracts-go:buf-lint
pnpm nx run dmpf-contracts-go:buf-pins
pnpm nx run dmpf-contracts-go:buf-generate-check
NX_BASE=<ref> pnpm nx run dmpf-contracts-go:buf-breaking
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
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --write-baseline
```

Este é um **passo humano**, nunca do agente (ADR-028): classificar é ato de
autoridade sobre a arquitetura, não consequência de escrever código.

O commit da classificação vai **sozinho**. `DMPF-T002` reprova o commit que
mistura mudança normativa com código, e "normativo" inclui tanto o baseline
quanto os `dmpf-units.json` de cada módulo:

```bash
git add tools/dmpf-baseline/units-baseline.json '**/dmpf-units.json'
git commit -m "chore(workspace): classificar as unidades de <ctx>"
```

## 6. Passo 5 — validar

O módulo novo é importer do pnpm, então o install vem antes:

```bash
pnpm install
pnpm nx run-many -t fmt-check,vet,lint,build,test -p '<ctx>-*'
```

Os testes que tocam Postgres levam a build tag `integration` e exigem o DSN.
Rode-os com `--parallel=1`: os harnesses truncam as tabelas do kernel, que todo
contexto compartilha.

```bash
pnpm nx run dmpf-reference-go:infra-up
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' \
  pnpm nx run-many -t test-race -p '<ctx>-*' --parallel=1
```

Por fim, o gate autoritativo entre módulos, com a base do intervalo em revisão:

```bash
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance -base <ref>
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
descartável, ele retira `dmpf-kernel/domain` de `shared_kernel_units` e exige
que o verificador reprove com `DMPF-D002` apontando `<ctx>/domain`. É a prova de
que o gate normativo ainda morde — se ele aprovar sem o shared kernel
designado, a regra de dependência parou de valer.

O **`regen`** apaga o golden num worktree, aciona o agente sobre a mesma spec e
roda os gates sobre o resultado. A garantia é pelos gates, não por bytes
idênticos: dois runs do agente divergem em forma, e isso é aceito (ADR-041). Ele
exige LLM, credenciais e tempo, então **não roda no CI** — o CI prova o golden
como qualquer outro módulo. Rode-o depois de mudar a skill, a rule ou o agente.

Vale rodar o `self-test` a cada mudança de forma do golden: a prova carrega o
layout em `MODULOS` e nos globs de comparação, e envelhece junto com ele.

## 9. Fontes normativas

- [`docs/dmpf/rfc-dmpf-foundation-v0.1.md`](../dmpf/rfc-dmpf-foundation-v0.1.md) — a RFC
- [`.claude/rules/dmpf-bounded-context.md`](../../.claude/rules/dmpf-bounded-context.md) — as normas do contexto
- [`.agents/skills/dmpf-bounded-context/references/armadilhas.md`](../../.agents/skills/dmpf-bounded-context/references/armadilhas.md) — o que já deu errado
- ADR-010, ADR-012, ADR-017 — regra de dependência, classificação por manifesto, identidade estável
- ADR-028 — classificação como ato de autoridade
- ADR-030 — granularidade de módulo e o layout de pasta por contexto
- ADR-041 — a virada do generator orientado ao domínio para o híbrido generator + agente
- ADR-042 — shared kernel

<!-- As seções "Divergir do golden path", "Como certificar" e "Contrato próprio"
     pertencem às specs SPEC-538MS2D4, SPEC-JPP31095 e SPEC-F7S5B6KV, que ainda
     não foram implementadas. Quando forem, entram aqui. -->
