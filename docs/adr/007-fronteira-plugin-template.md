# ADR-007: Fronteira entre o template e o plugin mmda-flow

## Status

**Supersedido** — 2026-08-13.

Originalmente aceito em 2026-07-31.
A decisão de "assar (bake) vs consumir" um plugin externo deixou de valer: o
template é **autônomo** e não documenta nem depende de consumo de plugin para
operar. O corpo abaixo permanece como registro histórico da fronteira que
vigia naquela data.

## Contexto

O marketplace `plugins-claude` (plugin `mmda-flow`, canônico em
`https://github.com/mateusmacedo/plugins-claude`) concentra convenções,
artefatos de estrutura e uma pipeline madura de desenvolvimento assistido por
IA. O `nx-base-template` já adotava parcialmente o formato de specs do plugin,
mas faltavam duas coisas:

- o scaffold de engenharia que o plugin provisiona (catálogo de specs, regras de
  domínio, guia de workflow, índice de ADRs);
- a documentação explícita de qual parte vive no template e qual parte vem do
  plugin quando ele está instalado.

Sem essa fronteira registrada, cada projeto derivado tende a reinventar a
estrutura ou a copiar o plugin de forma destrutiva — duplicando skills homônimas
e, pior, importando o fluxo git do plugin, que é incompatível com o git-flow da
organização. Este ADR fixava essa divisão para que o próximo mantenedor não a
refizesse.

## Decisão

### O template assa (bake); o plugin é consumido

O template versiona no repositório, de forma que o conteúdo seja legível sem o
plugin instalado:

- `docs/specs/README.md` — catálogo e template de specs.
- `docs/rules/**` — regras de domínio (`BIZ-` / `APP-` / `PRD-`).
- `docs/guides/development-workflow.md` — guia detalhado do fluxo de trabalho.
- `docs/adr/README.md` — índice e template de ADRs.
- rules documentais em `.claude/rules/` (`git-safety`, `comment-discipline`,
  `ephemeral-refs` e a `process-enforcement` endurecida).
- skills de domínio em `.claude/skills/` (Nest, Go, TypeORM, TypeScript e demais).
- `.github/PULL_REQUEST_TEMPLATE.md`.

O template consome do plugin, quando ele está instalado no ambiente do
desenvolvedor:

- agentes e skills de pipeline;
- hooks de higiene (code-guardian, tracker-pr-guard e afins);
- `skill-init`, `skill-sync` e `skill-config`;
- sincronização de tracker (Linear/Jira).

Nenhum runtime do plugin é forkado para dentro do template. A pipeline evolui num
único lugar — o plugin —, o que evita drift de manutenção e colisão de skills.

### Precedência de skills homônimas

Três skills existem dos dois lados; a tabela define quem vale em cada caso:

| Skill | Origem | Precedência |
| --- | --- | --- |
| `bug-fix` / `code-review` / `quality-checklist` | template e plugin | O template vale para código da organização; o plugin vale quando o fluxo mmda-flow é invocado explicitamente. |
| `planner` / `spec-management` / `feature-pipeline` | só plugin | Sempre do plugin. |
| Nest / Go / TypeORM / TypeScript | só template | Sempre do template. |

### `plans/` permanece fora do versionamento

`plans/` continua inteiro no `.gitignore` (linha 81). Os planos são artefatos
locais e não versionados. A decisão apoia-se em dois pontos: coerência com a rule
`ephemeral-refs` que o template passa a documentar — artefato efêmero não entra
no histórico durável — e custo de migração zero, já que esse é o estado atual do
`.gitignore`. O tradeoff aceito é que o revisor de pull request não lê o plano; o
racional durável migra para a spec (`docs/specs/`) e para este ADR.

### Hierarquia da documentação de fluxo

`docs/guides/development-workflow.md` é a fonte detalhada do fluxo. `AGENTS.md` e
`CONTRIBUTING.md` permanecem como fontes normativas resumidas, cada uma com
cross-link para o guia. Quando algum detalhe divergir, `AGENTS.md` e os arquivos
de configuração do projeto prevalecem.

### A skill `c4-architecture` foi incorporada e traduzida

A skill `c4-architecture` veio da configuração pessoal do mantenedor, não do
plugin, e foi traduzida para PT-BR ao ser versionada. A incorporação é coerente
com o propósito declarado em `.claude/README.md`: quem clona o template recebe os
recursos de assistente prontos, sem depender de configuração pessoal na máquina.

### `create-release.yml` falha explicitamente sem `origin/develop`

O cálculo automático do incremento de versão degradava em silêncio para `patch`
quando `origin/develop` não existia — um incremento possivelmente errado, sem
sinal para o operador. O workflow passou a falhar com mensagem clara nesse caso.
A consequência operacional é que as branches `develop` e `release/X.Y.Z`
precisam ser provisionadas no remoto para o ciclo de release funcionar.

## Alternativas descartadas

| Alternativa | Por que foi descartada |
| --- | --- |
| Copiar o mmda-flow inteiro para `.claude/` | Duplicaria a manutenção da pipeline e colidiria com as skills de domínio do template; a pipeline deve evoluir num único lugar. |
| Migrar `docs/adr/` para `docs/decisions/` | Custo sem ganho: já há seis ADRs numerados e referências em `AGENTS.md`. |
| Copiar o guia de workflow do plugin sem adaptar | O guia do plugin assume "PR sempre para a branch principal" e `gh`; violaria o git-flow com Gitea do template. |
| Versionar `plans/<slug>/` ignorando só `.state/` | Daria revisão do plano no pull request, mas levaria artefato efêmero ao histórico de todo projeto derivado do template. |
| Deixar `create-release.yml` degradar para `patch` sem `origin/develop` | Mascara a falta de provisionamento com um incremento silencioso e possivelmente incorreto. |

## Consequências

### Positivas

- Projetos derivados nascem com scaffold de engenharia (specs, regras de domínio,
  guia de workflow, índice de ADRs) alinhado ao mmda-flow e legível sem o plugin
  instalado.
- A pipeline evolui num único lugar; o template não carrega runtime de pipeline
  para manter, o que elimina o drift entre template e plugin.
- As skills de domínio do template permanecem intactas, e a precedência registrada
  evita colisão silenciosa com as homônimas do plugin.
- `docs/guides/development-workflow.md` centraliza o fluxo detalhado, enquanto
  `AGENTS.md` e `CONTRIBUTING.md` ficam enxutos com cross-link para o guia.
- O `create-release.yml` falha cedo, com mensagem clara, quando falta
  `origin/develop`, em vez de gerar um incremento de versão silencioso.
- A skill `c4-architecture` fica disponível a quem clona o template, sem depender
  de configuração pessoal na máquina do mantenedor.

### Negativas (assumidas)

- **Incompatibilidade de fluxo git ao instalar o plugin.** A instalação do
  mmda-flow carrega a rule `rules/git-workflow.md`, que declara substituir fluxos
  em que `develop` é branch de integração, impõe "PR sempre para a branch
  principal", nega precedência a `develop` como base e resolve a base via
  `gh repo view --json defaultBranchRef`. Isso colide com o template em dois
  pontos: o git-flow da organização promove trabalho → `develop` → `release/X.Y.Z`
  → `master`, exatamente o inverso do que a rule do plugin prescreve; e `gh` não
  opera contra Gitea (ver ADR-005). A incompatibilidade é assumida como custo do
  consumo do plugin. A mitigação é que o `docs/onboarding.md` nomeia os pipelines
  do plugin afetados — os que resolvem a base de pull request, como o de
  commit/push — e orienta a tratar `rules/git-workflow.md` como anti-referência
  quanto a fluxo, usando o guia do template como fonte de verdade.
- O revisor de pull request não lê o plano, porque `plans/` fica fora do
  versionamento; o conhecimento durável precisa migrar para spec ou ADR antes de
  ser citado.
- Parte das capacidades — pipeline, sincronização de tracker e hooks de higiene —
  só existe com o plugin instalado; o template sozinho não as fornece, e o
  ambiente do desenvolvedor passa a exigir a instalação e a manutenção do
  marketplace para obtê-las.

## Referências

- [ADR-001](./001-baseline-monorepo.md) — baseline do monorepo
- [ADR-003](./003-template-hardening.md) — hardening de tooling, docs e CI/CD
- [ADR-004](./004-workflows-verdaccio-release.md) — split version/publish
- [ADR-005](./005-plataforma-gitea.md) — a plataforma é Gitea, não GitHub/`gh`
- [ADR-006](./006-fechamento-port-melhorias.md) — fechamento do port de melhorias
- `docs/guides/development-workflow.md` — guia detalhado do fluxo
- `.claude/rules/ephemeral-refs.md` — política de referências a artefatos efêmeros
- `.github/workflows/create-release.yml` — cálculo de incremento e criação da release
- Plugin canônico: `https://github.com/mateusmacedo/plugins-claude`
