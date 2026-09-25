---
name: dmpf-bounded-context
description: >-
  Escreve um bounded context completo sobre o kernel DMPF deste workspace a
  partir de uma spec de bounded context: esqueleto pelo generator, código dos
  cinco blocos por agregado, contrato Protobuf e OpenAPI, gates até passar.
  Use ao criar ou estender um contexto de negócio (`apps/backend/<name>/`).
  Não use para o kernel `dmpf-*` nem para código Go sem relação com o DMPF.
---

# Bounded context sobre o kernel DMPF

Esta skill é o golden path de quem escreve um contexto de negócio — pessoa ou
o agente `dmpf-context-author`. A entrada é uma spec no template
[`references/template-bounded-context.md`](./references/template-bounded-context.md);
a saída é um módulo Go em `apps/backend/<name>` — um contexto de negócio é uma
app, `type:app`; em `libs/backend/go` fica só o kernel de reuso —, com um
package por bloco (`domain`, `ports`, `application`, `provider`, `app`), um
contrato e um OpenAPI que passam pelos mesmos gates de qualquer módulo do
workspace (ADR-045). Não existe DSL: a spec é a única fonte
do domínio, e o julgamento de quem escreve é guiado pelos moldes do kernel.

As normas que regem cada passo estão em
[`.claude/rules/dmpf-bounded-context.md`](../../../.claude/rules/dmpf-bounded-context.md);
o passo a passo detalhado, com o arquivo-molde de cada peça, em
[`references/golden-path.md`](./references/golden-path.md); as armadilhas
verificadas, em [`references/armadilhas.md`](./references/armadilhas.md).

## Divisão de trabalho

| Quem | O quê |
| --- | --- |
| **Generator** (`bounded-context`) | `project.json`, `go.mod`, `package.json`, `dmpf-units.json`, `README.md` e `Dockerfile` do módulo; tags 3D + `layer:*` do bloco mais alto; a entrada no `go.work`; o `doc.go` de cada bloco; o esqueleto canônico do `app` (config, wiring, telemetria, catálogo, `app/rpc`), o `provider/schema.{sql,go}` e o `appkit/pool.go` |
| **Agente / autor** | O conteúdo de domínio dos cinco blocos, preenchendo o esqueleto; o `.proto` do serviço e de cada evento publicado; o OpenAPI; o `include` de packages novos no manifesto (merge) |
| **Pessoa** | Rito Buf (`buf.sh generate` + quatro gates); `--write-baseline` em commit próprio; `git commit`; PR |

Os arquivos de configuração do módulo que o generator escreve ninguém edita à
mão; o esqueleto Go ele deixa para ser preenchido. O que é da pessoa, o agente
não executa — ele **para e imprime** o rito restante.

## Os doze passos

1. **Validar a spec** contra o template: dez seções, `stage` em `planning` ou
   `building`. Faltou seção, recusa nomeando-a; nada é escrito.
2. **Esqueleto**: `pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <name> --boundedContext <ctx>`.
   Não há `pnpm install`: `apps/backend/<name>` fica fora dos globs do
   `pnpm-workspace.yaml`, e só o Nx o lê.
3. **`domain`**, no package `domain` do módulo (o que o `doc.go` gerado declara), por agregado: struct, `Snapshot`/`From*Snapshot`/`Equal`/
   `clone`, UPRs `(cmd, at) → (Accepted[R], *Rejection)`, mensagens,
   rejeições com código estável. Sem `time`, sem porta; instante inteiro.
4. **`port`**: `Repository` por agregado, `Outbox()`, `Reader` por consulta;
   `Inbox()` só se o contexto consome.
5. **`application`**: `service.go` e um arquivo por comando percorrendo os nove
   passos de FND-04 §3.2; consultas fora da UoW; `Consume` pelas sete
   disposições de §6.4 **se** o contexto consome.
6. **`provider-postgres`**: `schema.sql` nos nomes canônicos (agregado no
   plural, sem prefixo), persistência híbrida (coluna tipada só para o que uma
   consulta filtra, o resto em `snapshot` `jsonb`), repositórios com
   optimistic locking, `Reader`, mapper para o payload do contrato; `Tables`
   do `appkit` com as tabelas do contexto.
7. **`app`**: o `ServiceDesc` de `app/rpc/service.go` com um método por
   comando e por consulta, o `Server` sobre o serviço de aplicação, o wiring e
   o catálogo; e2e gRPC sobre Postgres. Consumer com `envelope.Unpack` **só se
   o contexto consome**. A borda REST é do `bff`, em tarefa própria.
8. **Contrato**: `.proto` do serviço em
   `contracts/proto/company/<name>/service/v1/` e de cada evento publicado em
   `contracts/proto/company/<name>/event/v1/`; OpenAPI em
   `contracts/openapi/<name>/v1/`; unidade `<ctx>/contract` no manifesto do
   `contracts` por merge; depois o rito Buf — passo humano.
9. **`include`** dos packages novos no `dmpf-units.json` do módulo, na unidade do bloco certo, por merge.
10. **Classificação**: `--write-baseline` em commit próprio — passo humano.
11. **Gates**: `fmt-check`, `vet`, `build`, `lint`, `test-race`,
    `test-distributed`, `dmpf-context-check.sh --context`, `conformance
    --base`. Corrigir até passar; gate **normativo** reprovando (ex.:
    `DMPF-D002`) é parada, não contorno.
12. **Checklist final**: a tabela acima conferida, o rito humano impresso.

## Forma do código

Copie a forma, não o conteúdo: `apps/backend/bookings`, o golden da forma
canônica (ADR-053), e `apps/backend/reservations` só no que o contexto
consome. Testes antes do código de
cada UPR, caso de uso, repositório e rota — um cenário de aceite e um por
rejeição, como a spec declara. Prosa em PT-BR; código, godoc e identificadores
em inglês; comentário só quando explica um porquê que o código não diz.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,build,lint -p <name>
PG_DSN='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  pnpm nx run-many -t test-race,test-distributed -p <name>
bash tools/dmpf-context-check.sh --context apps/backend/<name>
go run ./tools/dmpf-conformance/cmd/conformance --root . --base origin/develop
```
