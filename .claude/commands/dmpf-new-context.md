---
description: Cria um bounded context sobre o kernel DMPF a partir de uma spec de bounded context (docs/specs/SPEC-<id>-<ctx>.md). Valida o template, invoca o agente dmpf-context-author com a skill dmpf-bounded-context e imprime o rito humano restante. Uso — /dmpf-new-context SPEC-<id> [--regen].
allowed-tools: Bash, Read, Glob, Grep, Agent
---
<!-- ephemeral-refs-ok-file: o command consome specs de bounded context por natureza -->

# Novo bounded context: $ARGUMENTS

Crie um bounded context a partir da spec em `$ARGUMENTS`. Siga os passos na
ordem; cada um tem uma recusa explícita. Nada é escrito antes do passo 4.

`$ARGUMENTS` traz o id da spec (`SPEC-<id>`) e, opcionalmente, `--regen`. O
`--regen` é o modo da prova de regeneração (`tools/dmpf-harness-check.sh
--phase regen`): aceita spec em `stage: done`, roda sem perguntas e não edita
outra app. Abaixo, `<id>` é o id e `<regen>` diz se a flag veio.

## 1. Resolver a spec

- Localize o arquivo pelo catálogo, resolvendo o script pela versão instalada:

  ```bash
  SQ=$(find "$HOME/.claude/plugins/cache" -path '*/scripts/spec-query.sh' 2>/dev/null | sort -V | tail -1)
  [ -n "$SQ" ] && bash "$SQ" --format json
  ```

  e selecione a entrada cujo `id` é `<id>`; o `path` vem do resultado. Se o
  script não existir, use `ls docs/specs/<id>-*.md`.
- Recusa: `<id>` não está no catálogo → "spec `<id>` não encontrada em
  `docs/specs/`"; encerre.

## 2. Validar o template

Leia a spec e verifique, nesta ordem:

1. `title` começa com `Bounded context —`.
2. `stage` é `planning` ou `building`; com `<regen>`, também `done`.
3. As dez seções existem como títulos `## `, com este nome exato:
   `Identidade`, `Agregados`, `Comandos (UPRs)`, `Eventos de domínio`,
   `Consultas`, `Relações entre agregados`, `Integração`,
   `Políticas transversais`, `Critérios de aceite`, `Escopo fora`.
4. Nenhuma seção contém `[TODO]`, `[TBD]` ou instrução entre colchetes do
   template.

Recusa, sem escrever nada, nomeando o problema:

- título fora do padrão → "`<id>` não é spec de bounded context: o
  título precisa começar com `Bounded context —`";
- `stage` fora do aceito → "`<id>` está em `stage: <valor>`; leve-a a
  `planning` antes";
- seção ausente → "`<id>` não tem a seção obrigatória `<nome>`" — a
  **primeira** que faltar, pelo nome exato acima;
- placeholder → "`<id>` tem placeholder em `<seção>`".

Comando de referência para a checagem das seções:

```bash
for s in 'Identidade' 'Agregados' 'Comandos (UPRs)' 'Eventos de domínio' 'Consultas' 'Relações entre agregados' 'Integração' 'Políticas transversais' 'Critérios de aceite' 'Escopo fora'; do
  grep -qxF "## $s" "<path>" || { echo "seção ausente: $s"; break; }
done
```

## 3. Conferir o terreno

- `git status --porcelain` vazio; caso contrário, avise e pergunte se
  continua (o agente escreve muitos arquivos). Com `<regen>`, não pergunte:
  recuse se não estiver vazio, porque a prova parte de um commit.
- `git branch --show-current` fora de `master`, `develop` e `release/*`;
  caso contrário, recuse: "crie uma branch de trabalho antes".
- `apps/backend/<name>` **não** existe (o `name` vem da seção Identidade);
  caso contrário, recuse: "o contexto `<name>` já existe; este command só
  cria".

## 4. Invocar o agente

Delegue ao agente `dmpf-context-author` (Agent tool, `subagent_type:
dmpf-context-author`, em foreground) com este prompt, substituindo o caminho:

> Escreva o bounded context descrito em `<path>`. Siga
> `.claude/rules/dmpf-bounded-context.md` e a skill
> `.agents/skills/dmpf-bounded-context/` (os doze passos de
> `references/golden-path.md`; as armadilhas de `references/armadilhas.md`).
> A forma canônica é a de `apps/backend/bookings` (ADR-053): borda só gRPC em
> `app/rpc`, com o serviço em `contracts/proto/company/<name>/service/v1/`;
> config sem prefixo `DMPF_`; schema em `provider/schema.sql`; persistência
> híbrida; instante em nanossegundos. O REST público é do `bff`, fora desta
> tarefa. Esqueleto pelo generator; `include` por merge; nunca toque baseline
> nem `gen/go`; nunca faça `git commit`. Rode os gates até passar, incluindo
> `bash tools/dmpf-context-check.sh --context apps/backend/<name>`; pare e
> reporte em qualquer gate normativo. Ao terminar, imprima arquivos criados,
> resultado de cada gate e o rito humano restante.

Com `<regen>`, acrescente ao prompt:

> Modo regeneração: não edite nenhuma app além de `apps/backend/<name>`. O
> `bff` compila contra o contrato que você escrever, então preserve o fio
> descrito na spec.

Aguarde o retorno e reproduza o relatório do agente.

## 5. Imprimir o rito humano

Depois do relatório, imprima sempre:

```text
Rito restante (passos humanos):
  1. (cd contracts && bash ../tools/buf.sh generate)  → gen/go
     pnpm nx run contracts:buf-lint
     pnpm nx run contracts:buf-pins
     pnpm nx run contracts:buf-generate-check
     NX_BASE=origin/develop pnpm nx run contracts:buf-breaking
  2. go run ./tools/dmpf-conformance/cmd/conformance --root . --write-baseline
     git add tools/dmpf-baseline/units-baseline.json && git commit   (só o baseline — DMPF-T002)
  3. go run ./tools/dmpf-conformance/cmd/conformance --root . --base origin/develop
     bash tools/dmpf-context-check.sh
  4. Banco e role <name> na infra (postgres-init, job de databases, Secrets)
     e as rotas REST no bff, em tarefa própria.
  5. Um commit por projeto Nx (AGENTS.md §Convenções obrigatórias); PR para develop.
```

Com `<regen>`, imprima no lugar do rito: "o rito é executado pelo
`tools/dmpf-harness-check.sh --phase regen`".

Se o agente parou num gate normativo, imprima o gate em vez do rito e não
sugira contorno.
