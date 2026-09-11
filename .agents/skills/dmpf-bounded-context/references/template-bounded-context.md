<!-- ephemeral-refs-ok-file: template de spec; o placeholder SPEC-XXXXXXXX é o formato do catálogo -->
# Template — spec de bounded context

Uma spec de bounded context descreve **um modelo**, não uma mudança: os
agregados, os comandos que os transformam, os eventos que emitem e como o
contexto se integra ao resto. É a única fonte do domínio que o agente
`dmpf-context-author` lê — não existe DSL paralela. O command
`/dmpf-new-context` valida as dez seções abaixo e recusa a spec nomeando a
primeira que faltar.

O frontmatter é o mesmo do catálogo de specs (nove campos), para que
`spec-query.sh` a liste como qualquer outra. O catálogo a distingue pelo
título, que começa com `Bounded context —`, e pela seção **Identidade**.
A spec preenchida vive em `docs/specs/`, como as demais.

Preencha as instruções entre colchetes e apague os colchetes. Regras e limiares
citam a norma (`docs/dmpf/`, ADRs) — nunca são reescritos aqui. Exemplo
preenchido: a spec do golden `bookings`.

```markdown
---
id: SPEC-XXXXXXXX
slug: <ctx>
title: Bounded context — <ctx>
stage: backlog
priority: P2
depends_on: []
ticket_url: null
subtask_urls: []
created: AAAA-MM-DD
---

# SPEC-XXXXXXXX: Bounded context — <ctx>

## Identidade

- **bounded_context**: `<ctx>` [nome declarado no `dmpf-units.json`; kebab-case]
- **name**: `<name>` [nome dos módulos `libs/backend/go/<name>-*` e do package
  Protobuf `company.<name>.event.v1`; identificador Go válido, sem hífen]
- **Linguagem ubíqua**: [um termo por linha, definição de uma linha]
  - *Termo* — definição.

## Agregados

[Um bloco por agregado.]

### <Agregado>

- **Identidade**: `generated` | `natural` por `<campo>`
- **Campos**: [nome: tipo — `string`, `int`, `bool`, `decimal`, `instant`, `id`]
- **Estados**: [valores possíveis do campo de estado, se houver]
- **Transições permitidas**: [origem → destino, uma por linha]
- **Invariantes de estado**: [o que é sempre verdadeiro do agregado, com a
  rejeição que cada violação produz]

## Comandos (UPRs)

[Um bloco por comando. As pré-condições pertencem ao **comando**, nunca ao
agregado. Exatamente um comando cria o agregado (`cria`) ou o inicializa
quando não existe (`inicializa se ausente`); os demais o exigem existente.]

### <Comando>

- **Agregado**: `<Agregado>`; **cria** | **inicializa se ausente** | exige existente
- **Entrada**: [campo: tipo, um por linha]
- **Pré-condições**: [regra → código de rejeição `<ctx>/<agregado>/<rejeicao>`]
- **Efeitos sobre o estado**: [campo ← valor ou `at`, um por linha]
- **Eventos emitidos**: [`<Evento>`]
- **Resposta**: [campos devolvidos ao chamador]

## Eventos de domínio

[Um bloco por evento. `EventName()` é `<ctx>.<agregado>.<evento>`.]

### <Evento>

- **Campos**: [nome: tipo ← origem (campo do agregado, campo do comando, `at`)]

## Consultas

[Uma linha por consulta; consultas correm fora da UoW.]

- `Find<Agregado>By<Campos>` — filtros: [campos]; cardinalidade: `one` | `many`

## Relações entre agregados

[Sempre por identidade, nunca por objeto.]

- `<Agregado>.<campo>` → `<Agregado>`

## Integração

- **Publica** (integration events): [`<Evento>` — vira
  `contracts/proto/company/<name>/event/v1/<evento>.proto`]
- **Consome**: [nenhum | por contrato: `messageFQN`, comando local, mapa de
  campos, disposições de consumo (FND-04 §6.4)]

## Políticas transversais

[Por referência à norma, sem reescrever.]

- **Idempotência**: [referência]
- **Autorização**: [referência]
- **Auditoria**: [referência]

## Critérios de aceite

- [ ] Os gates do workspace verdes para os módulos do contexto
  (`fmt-check`, `vet`, `build`, `lint`, `test-race`, verificador com `--base`).
- [ ] [Um cenário por comando: o aceite e cada rejeição, no formato
  DADO / QUANDO / ENTÃO.]

## Escopo fora

- [O que este contexto explicitamente não faz nesta versão, e por quê.]
```
