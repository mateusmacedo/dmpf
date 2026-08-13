---
# ephemeral-refs-ok-file: esta regra documenta o mecanismo de referências efêmeras
paths:
  - "apps/**"
  - "libs/**"
  - "docs/**"
  - ".claude/**"
  - "AGENTS.md"
  - "CONTRIBUTING.md"
  - "README.md"
---

# Referências a artefatos efêmeros

Código-fonte e documentação permanente devem citar apenas artefatos duráveis.
Artefatos efêmeros têm ciclo de vida curto: são reescritos ou removidos com
frequência, e uma referência a eles apodrece — vira link morto que engana quem
lê. Neste repositório, `plans` está listado no `.gitignore`, então planos são
arquivos locais e não versionados: apontar para eles a partir de conteúdo
versionado cria uma dependência que o próximo clone do repositório não resolve.
Esta regra define o que é efêmero, onde citá-los é permitido e como registrar
exceções conscientes.

## O que é artefato efêmero

- Planos de implementação: `plans/**`, `PLAN.md`, `CHECKPOINT.md`, `DRAFT.md`.
- Estados de pipeline: qualquer conteúdo sob `.state/` (por exemplo, estado de
  pipeline ou de acompanhamento de PR).

Esses caminhos descrevem trabalho em andamento. O conhecimento durável que
nasce neles deve migrar para um artefato versionado (spec ou ADR) antes de ser
citado em código ou em documentação permanente.

## Onde citar é permitido

- **`docs/specs/**`** — uma spec pode citar outras specs e planos livremente.
- **`docs/adr/**` (ADRs)** — pode citar **specs**. Um ADR é uma decisão durável,
  então cite a spec que originou a decisão em vez de um plano efêmero, que
  some e deixa o ADR sem lastro.
- **`docs/**/*.md`, `README*`, `CONTRIBUTING*`, `CHANGELOG*`** — isentos.
- Fixtures de teste (por exemplo, `**/tests/fixtures/**` e fixtures
  colocalizadas com o `*.spec.ts`) — isentas, porque citam caminhos como dado
  de entrada, não como referência viva.

Código-fonte (`.ts`, `.go`, `.py`, `.rs`, `.java`, …) e os meta-arquivos de
assistentes (`.claude/agents/`, `.claude/skills/`, `.claude/rules/`) ficam fora
da allowlist. Quando um desses precisa citar um efêmero, registre uma exceção
explícita.

## Marcadores de exceção

- **Inline** (isenta a linha): inclua o token `ephemeral-ref-ok: <razão>` na
  linha, no estilo de comentário da linguagem — `// ephemeral-ref-ok: <razão>`
  (TS, JS, Go, Rust, Java), `# ephemeral-ref-ok: <razão>` (Python, shell, YAML),
  `<!-- ephemeral-ref-ok: <razão> -->` (Markdown, HTML).
- **De arquivo** (isenta o arquivo inteiro): coloque
  `ephemeral-refs-ok-file: <razão>` em uma das 5 primeiras linhas. Use quando o
  arquivo, por natureza, descreve o próprio fluxo de planos ou de pipeline (por
  exemplo, um guia que explica o diretório `plans/`).

A razão é obrigatória: ela documenta por que a citação é aceitável e evita que a
exceção vire hábito silencioso.

## Ao encontrar uma citação a efêmero

Esta regra orienta a revisão manual; não é aplicada por hook. Os marcadores de
exceção acima servem de sinal para quem revisa — um `grep` por eles mostra onde a
citação foi deliberada.

Se você inseriu a citação nesta sessão, corrija-a escolhendo a ação adequada:

1. **Promova o conteúdo durável** — mova o conhecimento que justifica a citação
   para `docs/specs/` (spec) ou `docs/adr/` (ADR, citando specs) e aponte para o
   artefato durável.
2. **Remova a citação** quando ela não for essencial ao texto.
3. **Marque a exceção** (`ephemeral-ref-ok: <razão>`) quando o próprio arquivo
   documenta o mecanismo de planos ou de pipeline.

Se a citação já existia antes da sua alteração, preserve o conteúdo e registre a
ocorrência no resumo da entrega (path e linha), deixando a limpeza para um pedido
explícito do usuário.

---

## 🔹 Go: referências duráveis em código e godoc

O princípio vale igualmente para `.go`: arquivos-fonte e comentários de
documentação citam apenas artefatos versionados.

- **Comentários godoc** (`// Package ...` em `doc.go`, doc de tipos e de funções)
  são documentação durável — cite specs (`docs/specs/`) ou ADRs (`docs/adr/`) em
  vez de planos sob `plans/`.
- **Marcador inline** em Go usa o estilo `//`:
  `// ephemeral-ref-ok: descreve o fluxo de planos`.
- **Diretivas** como `//go:generate` e caminhos em build tags devem apontar para
  arquivos versionados; um gerador que lê de `plans/` quebra em um clone limpo.
- Quando um `doc.go` ou README de pacote precisar descrever o diretório de
  planos, use o marcador de arquivo (`ephemeral-refs-ok-file: <razão>`) na
  primeira linha do comentário de topo.
