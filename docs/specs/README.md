# Especificações — Ciclo de Vida e Template

Specs são o artefato central de desenvolvimento. Toda feature significativa
passa por especificação antes da implementação.

O catálogo de specs não é um arquivo central mantido à mão: ele é construído
**on demand** lendo o frontmatter YAML de cada `SPEC-*.md` deste diretório.
Cada spec carrega seus próprios metadados (`id`, `stage`, `priority`,
`depends_on`), então varrer os frontmatters basta para produzir a visão
consolidada (tabela markdown ou JSON). Para montar o catálogo ou inspecionar
IDs, leia os frontmatters diretamente — por exemplo, `rg` sobre os campos que
interessam.

## Ciclo de vida

```text
backlog  →  planning  →  building  →  done
   ↑           |  ↑         |  ↑        |
   |           |  |         |  +--------+ (reabrir)
   +-----------+  |         |
   +- - - - - - - +- - - - -+ (pausar → backlog)
                |           |
                |           +→ deferred    (bloqueada por dependência ou prioridade)
                |           +→ cancelled   (descartada com justificativa)
                +→ deferred / cancelled
```

O estado é controlado pelo campo `stage` no frontmatter YAML, que tem 9 campos:

```yaml
---
id: SPEC-XXXXXXXX               # 8 chars Crockford base32 (ver Nomenclatura)
slug: nome-da-feature           # kebab-case
title: Nome descritivo
stage: backlog                  # backlog | planning | building | done | deferred | cancelled
priority: P1                    # P0 | P1 | P2 | P3
depends_on: []                  # lista inline de IDs, ex: [SPEC-W4RWD02M]
ticket_url: null                # URL completa do ticket externo (Gitea/Linear/Jira) ou null
subtask_urls: []                # URLs completas das sub-issues; [] quando não há sub-issues
created: 2026-07-19             # data ISO YYYY-MM-DD
---
```

Os quatro campos de rastreamento (`id`, `slug`, `title`, `created`) são fixos
por spec; `stage`, `priority` e `depends_on` evoluem ao longo do ciclo. O schema
e a geração de ID seguem as convenções deste diretório (ver Nomenclatura).

### Transições

| De         | Para        | Quando                                               |
|------------|-------------|------------------------------------------------------|
| `backlog`  | `planning`  | Spec aprovada, dependências resolvidas, plano inicia |
| `planning` | `building`  | Plano aprovado, dev inicia                           |
| `planning` | `backlog`   | Pausar antes da implementação                        |
| `building` | `done`      | Todos os critérios de aceite passando                |
| `building` | `backlog`   | Pausar (voltar ao backlog)                           |
| `building` | `deferred`  | Bloqueada por dependência ou repriorizada            |
| `done`     | `building`  | Reabrir (desmarca checkboxes, alerta re-bloqueios)   |
| `backlog`  | `cancelled` | Descartada com justificativa documentada             |
| qualquer   | `deferred`  | Adiar indefinidamente                                |
| `deferred` | `backlog`   | Desbloqueada e pronta para retomar                   |

### Regras de transição

- Uma spec entra em `building` quando TODAS as dependências estão `done`.
- Specs `cancelled` mantêm a justificativa no corpo do documento.
- Specs `deferred` registram o motivo e a condição de desbloqueio.

## Convenções

### Nomenclatura

Formato: `SPEC-<id>-<slug>.md`, onde `<id>` é um identificador de 8 caracteres
em Crockford base32 e `<slug>` é o título em kebab-case. Gere o `<id>`
manualmente seguindo o alfabeto Crockford base32 (8 chars). Specs legadas no
formato `SPEC-NNN-...` continuam válidas — mantenha o nome como está.

Exemplos deste repositório:

- `SPEC-YWAWQHPX-melhorias-template-base-nx.md`
- `SPEC-W4RWD02M-evoluir-workflows-com-lidercap.md`
- `SPEC-4NR9KKS8-portar-melhorias-telesena-monorepo.md`

### Formato de requisitos (checklist com prioridade)

```markdown
- [ ] **[P0] Nome do requisito**: descrição imperativa do que fazer
- [ ] **[P1] Outro requisito**: descrição
- [x] **[P0] Requisito concluído**: descrição
```

| Tag  | Significado                                 | Quando usar          |
|------|---------------------------------------------|----------------------|
| [P0] | Crítico — sem isso a feature não funciona   | Core, constraints    |
| [P1] | Importante — feature funciona mas degradada | Edge cases, feedback |
| [P2] | Desejável — melhoria incremental            | Polish, nice-to-have |

### Escopo fora (obrigatório)

Toda spec tem uma seção "Escopo fora" listando o que NÃO será feito e por quê.
Essa seção delimita o trabalho e evita feature creep durante a implementação.

### Constraint sandwich

Constraints críticas usam XML tags e aparecem DUAS vezes na spec:

1. No início, após a seção Contexto (tag `<constraints>`).
2. No final, após Verificação e testes (tag `<critical_constraints>`).

```xml
<constraints>
- [P0] Constraint crítica (ex: "NUNCA quebrar `nx affected` / cache")
- [P1] Constraint importante
</constraints>
```

A repetição garante que o agente mantenha aderência às constraints mesmo em
specs longas.

## Template resumido

```markdown
---
id: SPEC-XXXXXXXX
slug: nome-descritivo
title: Nome descritivo
stage: backlog
priority: P1
depends_on: []
ticket_url: null
subtask_urls: []
created: 2026-07-19
---

# SPEC-XXXXXXXX: [Nome descritivo]

## Resumo
[1-3 frases: O QUE e POR QUÊ]

## Contexto
- Problema / Impacto / Links relevantes

<constraints>
- [P0] Constraints críticas
</constraints>

## Requisitos

### Funcionais
- [ ] **[P0] Requisito**: descrição imperativa

### Não-funcionais
- [ ] Performance / Acessibilidade / Compatibilidade

## Localização de código
[Paths exatos de onde implementar]

## Design
### Arquitetura / Fluxo principal / Pseudocódigo

## Decisões técnicas
[Escolhas e justificativas]

## Verificação e testes

### Critérios de aceite
- [ ] [Critério mensurável]

### Cenários de teste (mínimo 3)
DADO / QUANDO / ENTÃO

<critical_constraints>
[Repetir constraints do início]
</critical_constraints>

## Escopo fora
- **[Feature X]**: justificativa
```

## Checklist de qualidade

Antes de aprovar uma spec, verificar:

- [ ] Todas as seções obrigatórias presentes e preenchidas
- [ ] Sem `[TODO]`, `[TBD]`, `[PENDENTE]` ou seções vazias
- [ ] Constraints em XML tags (sandwich: início e final)
- [ ] Linguagem imperativa (não sugestiva)
- [ ] Escopo fora com justificativa
- [ ] Cenários de teste: mínimo 3 (happy path, edge case, erro)
- [ ] Critérios de aceite mensuráveis

### Tamanho ideal

- Spec total: menos de 200 linhas de requisitos.
- Se ultrapassar: quebrar em sub-specs, cada uma focada em 1 aspecto.
