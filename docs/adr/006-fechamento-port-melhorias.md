# ADR-006: Fechamento do port de melhorias do monorepo derivado

## Status

Aceito — 2026-07-30. Implementa e fecha SPEC-4NR9KKS8.

Este ADR existe por exigência da própria spec, que pede em `critical_constraints`
(P1) o registro da regra de desempate, a justificativa de cada exceção preservada
e a rastreabilidade de origem por arquivo.

## Contexto

Este template originou, em 2026-07-16, um monorepo derivado de produto. Os dois
repositórios compartilham o commit raiz `7a42ab6` (`chore(source): init project`)
e **divergiram em paralelo**, não em série: o derivado não é um superset do
template, e o template não é um subconjunto do derivado.

Durante essa divergência, o derivado resolveu problemas concretos de tooling,
segurança e CI que continuavam abertos aqui — e que todo projeto novo gerado deste
template herdaria. Ao mesmo tempo, o template avançou em pontos onde o derivado
regrediu ou simplificou por conveniência local.

Origem do port: branch `develop`, commit
`309ab4eeb04b4844283d9a6c12e75da36ef5dc41` (`309ab4e`, 2026-07-27).

## Decisão

### Regra de desempate

**O repositório de origem prevalece por padrão. O template é preservado nos itens
em que está comprovadamente mais novo ou mais completo.**

A regra foi escolhida porque a análise de histórico provou evolução paralela a
partir do commit raiz comum. A alternativa — prevalência literal e irrestrita da
origem — foi descartada por causar regressão objetiva: downgrade de `nx` 23.1.0
para 22.7.5, desmonte do `catalog:` do pnpm e perda do generator local.

Cada aplicação da regra foi decidida por medição, não por preferência. Quando a
origem e o template divergiam, a pergunta era "qual dos dois está factualmente
mais correto para um template?", e a resposta foi registrada.

### Organização da entrega

Os arquivos foram classificados por **contrato verificável** — o que cada um
promete executar no template determina como ele é verificado:

- **Cópia literal** — sem input do repositório nem efeito de runtime. Verificação
  por identidade de hash contra a origem, sobre a lista de `git ls-files`.
- **Curadoria obrigatória** — docs, workflows e scripts que carregam nome de
  projeto, plataforma, conta, path, runner, secret, registry ou escopo npm.
  Verificação por leitura e por denylist ampliada.

## Exceções preservadas

Os 12 itens abaixo mantiveram o estado do template. Cada linha registra a
justificativa e o resultado verificado ao fim da entrega.

| # | Item | Estado preservado | Por que o template prevalece | Verificado |
|---|------|-------------------|------------------------------|------------|
| 1 | `nx` e plugins `@nx/*` | `23.1.0` | A origem está em 22.7.5; adotá-la seria downgrade | `catalog.nx` = 23.1.0 |
| 2 | `catalog:` do pnpm | **37 entradas** | A origem reduziu a 7 e pinou o resto nos `package.json`, perdendo a centralização | `yq '.catalog \| length'` = 37 |
| 3 | `tools/generators.json` | generator `shared-lib` | A origem esvaziou (`generators: {}`); o generator é capacidade do template | arquivo não alterado no diff |
| 4 | `nx.json` → `nxCloudId` | presente | A origem removeu | chave presente |
| 5 | `nx.json` → `sharedGlobals` | 5 inputs | A origem encolheu para só `biome.json`, degradando a invalidação de cache | 5 entradas, incluindo `pnpm-workspace.yaml` e `pnpm-lock.yaml` |
| 6 | plugin `@nx/jest` | sem `exclude` | O `exclude` da origem aponta para e2e de apps que não existem aqui | sem chave `exclude` |
| 7 | `release.yml` | release Docker (registry, esquema de versão, hotfix) | A origem removeu porque usa CD próprio; o template mantém a capacidade | 6 referências presentes |
| 8 | `publish-libs.yml` → `pnpm_version` | `"11"` | A origem está em `"9"`, incoerente com o próprio `packageManager` | `pnpm_version: "11"` |
| 9 | `libs/shared/` | `types` e `utils` | A lib de domínio da origem não pertence a um template | as duas libs intactas |
| 10 | `README.md` | orientado a template | O da origem é orientado ao produto | inalterado |
| 11 | `infra/`, `renovate.json`, `migrations.json`, `LICENSE`, `SECURITY.md`, `CODE_OF_CONDUCT.md` | presentes | Não existem na origem | 6/6 presentes |
| 12 | Aspas em YAML | `"` | O uso de `'` na origem é cosmético | mantido; as aspas simples remanescentes em `cd-dev-hmg.yml` são sintaxe obrigatória de expressão (`== ''`, `\|\| 'dev'`) |

### Correção ao item 2

A spec e o plano descreviam o `catalog:` como tendo **30 entradas**. A contagem
real, medida com `yq '.catalog | length'`, é **37**. O número nos documentos de
planejamento estava errado; o arquivo estava correto e nada foi removido. O
critério de aceite passa a ser 37.

## Rastreabilidade de origem

Todo arquivo aplicado tem origem identificável no repositório derivado, no commit
`309ab4e`. Os paths coincidem entre origem e destino em todos os casos.

### Cópia literal — identidade por hash

94 arquivos verificados, **94 idênticos, 0 divergências**:

| Grupo | Arquivos | Path na origem |
|-------|----------|----------------|
| `.claude/rules/` | 9 | idêntico |
| `.claude/skills/` | 33 (25 `SKILL.md` + 8 em `references/`) | idêntico |
| `.claude/agents/` | 50 (49 agentes + `README.md` do diretório) | idêntico |
| `tools/normalize-pruned-manifest.mjs` | 1 | idêntico |
| `.vscode/settings.json` | 1 | idêntico |

A cópia foi guiada por `git ls-files` da origem, nunca por diretório. Isso manteve
fora os 2 arquivos locais que existem no disco da origem mas não são versionados
(`settings.local.json` e `scheduled_tasks.lock`), cuja ausência foi verificada
explicitamente.

Dois arquivos já existiam no template e foram medidos como byte-idênticos antes de
serem sobrescritos, tornando a sobrescrita um no-op:
`.claude/agents/ci-monitor-subagent.md` e
`.claude/skills/fill-pr-template-from-diff/SKILL.md`.

Os 8 arquivos de `.claude/commands/` da origem **não** foram portados: são
byte-idênticos aos que o template já tinha.

### Aplicação parcial — trecho identificado

| Arquivo | O que veio da origem |
|---------|----------------------|
| `biome.json` | `parser.unsafeParameterDecoratorsEnabled` e 6 exclusões de `files.includes` |
| `jest.preset.js` | `passWithNoTests: true` |
| `.gitignore` | `test-output`, `.tokensave/`, `.headroom/`, `newrelic_agent.log` |
| `.dockerignore` | reescrita com globs `**/` e o comentário que justifica excluir `.env` |
| `package.json` | `"prepare": "lefthook install \|\| true"` |
| `pnpm-workspace.yaml` | 5 entradas `false` em `allowBuilds` |
| `.github/actions/setup-node-pnpm/action.yml` | remoção do input `pnpm-version` e o comentário sobre `ERR_PNPM_BAD_PM_VERSION` |
| `.github/workflows/create-release.yml` | o passo de abertura de PR via API, com `jq` + `curl` |
| `.vscode/extensions.json` | base da origem, com `esbenp.prettier-vscode` substituído por `biomejs.biome` |
| `CONTRIBUTING.md` | seção "Modelo develop → release (anti-drift)" |
| `docs/adr/001-baseline-monorepo.md` | apenas o formato do addendum; o conteúdo é deste workspace |

### Reescrita — origem como estrutura, conteúdo próprio

Nestes arquivos a origem forneceu a estrutura e o conteúdo foi reescrito a partir
da realidade medida deste workspace:

`AGENTS.md`, `CLAUDE.md`, `docs/onboarding.md`, `docs/ci-cd/README.md`,
`docs/ci-cd/DEPLOY_APPLICATIONS.md`, `docs/ci-cd/EXPANSION_MODELS.md`,
`.github/workflows/cd-dev-hmg.yml`, `infra/docker/Dockerfile.node.example`,
`docs/nx-reference/tasks.md`.

### Sem origem — autoral

- `.claude/README.md` — gerado do diretório real. O README da origem documenta 7
  agentes e cita `agents/architecture.md`, arquivo que não existe nem lá nem aqui.
- `docs/adr/005-plataforma-gitea.md` e este ADR-006.

## Consequências

- Projetos gerados deste template herdam as correções de tooling, segurança e CI
  já validadas em produção no repositório derivado.
- A regra de desempate fica registrada para o próximo port entre estes dois
  repositórios: quem for aplicar deve reavaliar as 12 exceções, porque algumas
  deixam de valer quando a origem atualizar (o item 1, por exemplo, cai no dia em
  que a origem passar de 23.1.0).
- O gate de generalização por denylist tem dois falsos-positivos conhecidos, ambos
  verificados linha a linha:
  1. O **arquivo da spec** contém o nome do repositório de origem no próprio nome
     e ao longo do texto — inevitável, porque é o documento que descreve o port. O
     gate deve excluí-lo do escopo. Este ADR foi escrito de modo a **não** depender
     desse nome: a origem é referida por papel ("repositório derivado", "origem") e
     por commit, de forma que ele passa no gate sem exceção.
  2. `JWT_SECRET` aparece em 4 exemplos didáticos de tipagem de variável de
     ambiente sob `.claude/skills/**/references/` — nome genérico de configuração,
     não o segredo da origem.
- Nenhuma infraestrutura real foi importada: identificador de conta de nuvem, path
  de parâmetro, nome de secret de produção, namespace e URL interna foram medidos
  como ausentes (contagem zero) nos arquivos aplicados.

## Referências

- Spec: `SPEC-4NR9KKS8` (em `docs/specs/`)
- [ADR-001](./001-baseline-monorepo.md) — baseline, com addendum de estado real
- [ADR-004](./004-workflows-verdaccio-release.md) — parcialmente superseded
- [ADR-005](./005-plataforma-gitea.md) — a plataforma é Gitea
