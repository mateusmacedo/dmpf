# agents/

Agentes especializados que o Claude Code pode invocar como subagentes.

## O que são

Agents são arquivos `.md` com frontmatter YAML que definem um subagente com ferramentas,
modelo e instruções específicas. O Claude os invoca automaticamente via `Agent` tool
quando a descrição do agente corresponde à tarefa.

## Catálogo completo

Esta pasta tem 50 agentes. O catálogo canônico, agrupado por foco (revisão e
diagnóstico, backend/dados/APIs, infraestrutura/CI/CD/segurança, testes e
automação, documentação e texto, orquestração e evolução), vive em
[`../README.md`](../README.md#agents) — evite duplicá-lo aqui.

## Anatomia de um agent

```yaml
---
name: nome-do-agent
model: haiku  # haiku, sonnet ou opus
description: "Descrição usada pelo Claude para decidir quando invocar. Quanto mais específica, melhor o trigger."
tools:
  - Read
  - Grep
  - Glob
---

Instruções do agente em markdown...
```

## Quando criar um novo agent

- Tarefa repetitiva que precisa de modelo/ferramentas específicas
- Revisão ou validação especializada
- Processamento que se beneficia de contexto isolado
