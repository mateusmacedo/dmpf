# Decisões Arquiteturais (ADRs)

Registro das decisões técnicas relevantes do projeto. Cada decisão vive em um
arquivo próprio neste diretório (`docs/adr/`), legível sem nenhuma ferramenta
externa. Este README é o índice e o guia de escrita.

## Quando criar um ADR

Abra um ADR quando a decisão tiver alcance e trade-offs que valha a pena
preservar:

- Escolha de tecnologia, framework ou biblioteca significativa.
- Padrão arquitetural que afeta múltiplos módulos ou apps.
- Mudança de abordagem de algo já estabelecido no projeto.
- Decisão com alternativas descartadas que precisam ficar rastreáveis.

Reserve o registro para o que sobrevive à sprint. Não abra um ADR para:

- Decisão trivial ou óbvia (por exemplo, adotar UTF-8) — siga o default.
- Escolha de implementação local que não cruza fronteiras de módulo — decida
  no próprio código.
- Correção de bug — documente no commit e, se couber, no teste de regressão.
- Mudança sem alternativas em jogo — não há trade-off a registrar.

## Nomenclatura

Um arquivo por decisão, numeração sequencial e slug em kebab-case:

```text
00N-slug-em-kebab-case.md
```

O número é o próximo livre na sequência, um a mais que o último registrado
na tabela abaixo. O título dentro do arquivo repete o número: `# ADR-00N: <título>`.

## Template

Copie o esqueleto abaixo. O formato é local e **sem frontmatter** — o arquivo
começa direto no `# ADR-00N:`.

```markdown
# ADR-00N: Título imperativo em PT-BR

## Status

Aceito — AAAA-MM-DD. Implementa SPEC-XXXXXXXX.

## Contexto

Qual problema ou necessidade levou à decisão. Restrições, requisitos e
contexto relevante.

## Decisão

O que foi decidido e por quê.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Opção A     | Motivo                |
| Opção B     | Motivo                |

## Consequências

**Positivas:**

- Benefício 1

**Negativas:**

- Trade-off 1
```

### `## Status` é opcional

O bloco `## Status` pode ser omitido: ADR-001 e ADR-002 não o trazem, enquanto
ADR-003 a ADR-009 o incluem. Quando presente, o padrão adotado é
`Aceito — AAAA-MM-DD`, seguido, quando aplicável, da referência à spec que a
decisão implementa (por exemplo, `Implementa SPEC-4NR9KKS8`). Uma decisão pode
substituir outra: registre isso na própria `## Decisão`, como ADR-005 fez ao
substituir itens do ADR-004.

### Addendum para revisões datadas

Para atualizar um ADR já aceito sem reescrever a decisão histórica, acrescente
ao final um bloco `## Addendum — AAAA-MM-DD`. Ele preserva o status e a
narrativa originais e apenas registra o estado novo — ver o addendum do
ADR-001.

## Decisões registradas

| Nº | Título | O que decide |
| --- | --- | --- |
| ADR-001 | Baseline do Monorepo Multistack | Parte do preset `--preset=ts` neutro e adiciona os plugins de forma controlada, em vez de herdar um preset focado em framework. |
| ADR-002 | Configuração de Tasks, Cache e Pipelines no NX | Define três camadas de configuração (plugin inferido, `targetDefaults`, `project.json`) e proíbe redeclarar targets para não quebrar o cache. |
| ADR-003 | Hardening do template (tooling, docs, CI/CD, infra) | Centraliza versões no `catalog:` do pnpm, fixa Node 24 / pnpm 11, migra o Biome para o formato 2.x e cria as docs de comunidade. |
| ADR-004 | Split version/publish e alvo Verdaccio | Separa o versionamento (`release.yml`) da publicação de libs (`publish-libs.yml`) e adota o runner `gitea-runner`. |
| ADR-005 | A plataforma de hospedagem é Gitea | Toda automação que fala com a plataforma usa a API do Gitea (`/api/v1`); o binário `gh` deixa de ser usado nos workflows. |
| ADR-006 | Fechamento do port de melhorias do monorepo derivado | Fixa a regra de desempate — a origem prevalece por padrão, o template é preservado onde está mais completo — ao portar melhorias. |
| ADR-007 | Fronteira entre o template e o plugin mmda-flow | **Supersedido** (2026-08-13). Registro histórico: bake vs consume. O template é autônomo e não depende de plugin externo. |
| ADR-008 | Destravar o release — proteções do repositório e ajuste do Nx Release ao Gitea | Remove as proteções de branch e tag, remove o provider de release incompatível e alinha o `release.yml` ao workflow de referência da organização. |
| ADR-009 | O template não carrega libs nem generator de exemplo | Remove `shared-types`, `shared-utils` e o generator `shared-lib` obsoleto, deixando `libs/shared/` e `tools/generators/` como destinos vazios; a infraestrutura de release fica armada com um guard para conjunto vazio. |
