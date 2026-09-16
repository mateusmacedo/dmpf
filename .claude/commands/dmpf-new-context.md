---
description: Cria um bounded context sobre o kernel DMPF a partir de uma spec de bounded context (docs/specs/SPEC-<id>-<ctx>.md). Valida o template, invoca o agente dmpf-context-author com a skill dmpf-bounded-context e imprime o rito humano restante. Uso — /dmpf-new-context SPEC-<id>.
allowed-tools: Bash, Read, Glob, Grep, Agent
---
<!-- ephemeral-refs-ok-file: o command consome specs de bounded context por natureza -->

# Novo bounded context: $ARGUMENTS

Crie um bounded context a partir da spec `$ARGUMENTS`. Siga os passos na
ordem; cada um tem uma recusa explícita. Nada é escrito antes do passo 4.

## 1. Resolver a spec

- Localize o arquivo pelo catálogo:
  `bash ~/.claude/plugins/cache/plugins-claude/mmda-flow/0.4.8/scripts/spec-query.sh --format json`
  e selecione a entrada cujo `id` é `$ARGUMENTS`; o `path` vem do resultado.
  Se o script não existir, use `ls docs/specs/$ARGUMENTS-*.md`.
- Recusa: `$ARGUMENTS` não está no catálogo → "spec `$ARGUMENTS` não
  encontrada em `docs/specs/`"; encerre.

## 2. Validar o template

Leia a spec e verifique, nesta ordem:

1. `title` começa com `Bounded context —`.
2. `stage` é `planning` ou `building`.
3. As dez seções existem como títulos `## `, com este nome exato:
   `Identidade`, `Agregados`, `Comandos (UPRs)`, `Eventos de domínio`,
   `Consultas`, `Relações entre agregados`, `Integração`,
   `Políticas transversais`, `Critérios de aceite`, `Escopo fora`.
4. Nenhuma seção contém `[TODO]`, `[TBD]` ou instrução entre colchetes do
   template.

Recusa, sem escrever nada, nomeando o problema:

- título fora do padrão → "`$ARGUMENTS` não é spec de bounded context: o
  título precisa começar com `Bounded context —`";
- `stage` fora de `planning`/`building` → "`$ARGUMENTS` está em `stage:
  <valor>`; leve-a a `planning` antes";
- seção ausente → "`$ARGUMENTS` não tem a seção obrigatória `<nome>`" — a
  **primeira** que faltar, pelo nome exato acima;
- placeholder → "`$ARGUMENTS` tem placeholder em `<seção>`".

Comando de referência para a checagem das seções:

```bash
for s in 'Identidade' 'Agregados' 'Comandos (UPRs)' 'Eventos de domínio' 'Consultas' 'Relações entre agregados' 'Integração' 'Políticas transversais' 'Critérios de aceite' 'Escopo fora'; do
  grep -qxF "## $s" "<path>" || { echo "seção ausente: $s"; break; }
done
```

## 3. Conferir o terreno

- `git status --porcelain` vazio; caso contrário, avise e pergunte se
  continua (o agente escreve muitos arquivos).
- `git branch --show-current` fora de `master`, `develop` e `release/*`;
  caso contrário, recuse: "crie uma branch de trabalho antes".
- `libs/backend/go/<name>-domain` **não** existe (o `name` vem da seção
  Identidade); caso contrário, recuse: "o contexto `<name>` já existe; este
  command só cria".

## 4. Invocar o agente

Delegue ao agente `dmpf-context-author` (Agent tool, `subagent_type:
dmpf-context-author`, em foreground) com este prompt, substituindo o caminho:

> Escreva o bounded context descrito em `<path>`. Siga
> `.claude/rules/dmpf-bounded-context.md` e a skill
> `.agents/skills/dmpf-bounded-context/` (os doze passos de
> `references/golden-path.md`; as armadilhas de `references/armadilhas.md`).
> Esqueleto pelo generator; `include` por merge; nunca toque baseline nem
> `gen/go`; nunca faça `git commit`. Rode os gates até passar; pare e reporte
> em qualquer gate normativo. Ao terminar, imprima arquivos criados, resultado
> de cada gate e o rito humano restante.

Aguarde o retorno e reproduza o relatório do agente.

## 5. Imprimir o rito humano

Depois do relatório, imprima sempre:

```text
Rito restante (passos humanos):
  1. pnpm install                                    (se o agente não rodou)
  2. (cd contracts && bash ../tools/buf.sh generate)  → gen/go
     pnpm nx run contracts:buf-lint
     pnpm nx run contracts:buf-pins
     pnpm nx run contracts:buf-generate-check
     NX_BASE=origin/develop pnpm nx run contracts:buf-breaking
  3. go run ./libs/backend/go/conformance/cmd/conformance --root . --write-baseline
     git add tools/dmpf-baseline/units-baseline.json && git commit   (só o baseline — DMPF-T002)
  4. go run ./libs/backend/go/conformance/cmd/conformance --root . --base origin/develop
  5. Um commit por projeto Nx (AGENTS.md §Convenções obrigatórias); PR para develop.
```

Se o agente parou num gate normativo, imprima o gate em vez do rito e não
sugira contorno.
