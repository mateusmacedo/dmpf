---
name: dmpf-bounded-context
description: >-
  Escreve um bounded context completo sobre o kernel DMPF deste workspace a
  partir de uma spec de bounded context: esqueleto pelo generator, código dos
  cinco blocos por agregado, contrato Protobuf e OpenAPI, gates até passar.
  Use ao criar ou estender um contexto de negócio (`libs/backend/go/<name>/`).
  Não use para o kernel `dmpf-*` nem para código Go sem relação com o DMPF.
---

# Bounded context sobre o kernel DMPF

Esta skill é o golden path de quem escreve um contexto de negócio — pessoa ou
o agente `dmpf-context-author`. A entrada é uma spec no template
[`references/template-bounded-context.md`](./references/template-bounded-context.md);
a saída são cinco módulos Go em `libs/backend/go/<name>/{domain,ports,application,provider,app}`, um contrato e um OpenAPI que passam pelos mesmos
gates de qualquer módulo do workspace. Não existe DSL: a spec é a única fonte
do domínio, e o julgamento de quem escreve é guiado pelos moldes do kernel.

As normas que regem cada passo estão em
[`.claude/rules/dmpf-bounded-context.md`](../../../.claude/rules/dmpf-bounded-context.md);
o passo a passo detalhado, com o arquivo-molde de cada peça, em
[`references/golden-path.md`](./references/golden-path.md); as armadilhas
verificadas, em [`references/armadilhas.md`](./references/armadilhas.md).

## Divisão de trabalho

| Quem | O quê |
| --- | --- |
| **Generator** (`bounded-context`) | `project.json`, `go.mod`, `go.work`, `dmpf-units.json`, tags 3D + `layer:*`, `dependsOn` intra-contexto, `doc.go` e `README.md` de cada módulo |
| **Agente / autor** | Todo `.go` de produção e teste dos cinco blocos; o `.proto` de cada evento publicado; o OpenAPI; o `include` de packages novos no manifesto (merge) |
| **Pessoa** | `pnpm install`; rito Buf (`buf.sh generate` + quatro gates); `--write-baseline` em commit próprio; `git commit`; PR |

O que o generator escreve, ninguém edita à mão. O que é da pessoa, o agente
não executa — ele **para e imprime** o rito restante.

## Os doze passos

1. **Validar a spec** contra o template: dez seções, `stage` em `planning` ou
   `building`. Faltou seção, recusa nomeando-a; nada é escrito.
2. **Esqueleto**: `pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <name> --boundedContext <ctx>`,
   depois `pnpm install` (o módulo novo é importer do pnpm).
3. **`domain`**, na raiz do módulo (package que o `doc.go` gerado declara), por agregado: struct, `Snapshot`/`From*Snapshot`/`Equal`/
   `clone`, UPRs `(cmd, at) → (Accepted[R], *Rejection)`, mensagens,
   rejeições com código estável. Sem `time`, sem porta; instante inteiro.
4. **`port`**: `Repository` por agregado, `Outbox()`, `Reader` por consulta;
   `Inbox()` só se o contexto consome.
5. **`application`**: `service.go` e um arquivo por comando percorrendo os nove
   passos de FND-04 §3.2; consultas fora da UoW; `Consume` pelas sete
   disposições de §6.4 **se** o contexto consome.
6. **`provider-postgres`**: `schema.sql`, `Migrate`, repositórios com optimistic
   locking, `Reader`, mapper para o payload do contrato; harness de teste
   próprio.
7. **`app`**: uma `dmpfhttp.Route` por comando e por consulta, com
   `ContractRef`; OpenAPI em `contracts/openapi/<name>/v1/`; e2e sobre
   Postgres. Consumer com `envelope.Unpack` **só se o contexto consome**.
8. **Contrato**: `.proto` por evento publicado em
   `contracts/proto/company/<name>/event/v1/`; unidade `<ctx>/contract` no
   manifesto do `dmpf-contracts` por merge; depois o rito Buf — passo humano.
9. **`include`** dos packages novos nos manifestos dos cinco módulos, por merge.
10. **Classificação**: `--write-baseline` em commit próprio — passo humano.
11. **Gates**: `fmt-check`, `vet`, `build`, `lint`, `test-race` (com
    `--parallel=1` quando há Postgres), `dmpf-conformance --base`. Corrigir
    até passar; gate **normativo** reprovando (ex.: `DMPF-D002`) é parada, não
    contorno.
12. **Checklist final**: a tabela acima conferida, o rito humano impresso.

## Forma do código

Copie a forma, não o conteúdo: `dmpf-domain/example/orders` para o agregado
produtor, `example/reservations` para o consumidor. Testes antes do código de
cada UPR, caso de uso, repositório e rota — um cenário de aceite e um por
rejeição, como a spec declara. Prosa em PT-BR; código, godoc e identificadores
em inglês; comentário só quando explica um porquê que o código não diz.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,build,lint -p <name>-domain-go,<name>-ports-go,<name>-application-go,<name>-provider-postgres-go,<name>-app-go
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' pnpm nx run-many -t test-race -p <name>-provider-postgres-go,<name>-app-go --parallel=1
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --base origin/develop
```
