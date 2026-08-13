---
id: SPEC-7VQFX9RP
slug: remover-libs-shared-exemplo
title: Remover as libs compartilhadas e o generator shared-lib do template
stage: done
priority: P2
depends_on: []
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-470
subtask_urls: []
created: 2026-08-12
---

# SPEC-7VQFX9RP: Remover as libs compartilhadas e o generator shared-lib

## Resumo

Remover `@lidercap-apps/shared-types` e `@lidercap-apps/shared-utils` do
workspace, deixando `libs/shared/` como diretório de destino vazio, no mesmo
padrão de `libs/backend/` e `libs/frontend/`. Como mantenedor do template, quero
que quem parte deste repositório não herde bibliotecas que não pediu, para que o
projeto derivado comece apenas com a infraestrutura e crie as próprias libs pelo
generator do Nx. Sai junto o generator interno `shared-lib`, obsoleto: ele não
resolve e duplica o que `@nx/js:lib` já faz.

## Contexto

- **Problema**: o template entrega duas bibliotecas concretas —
  tipos de API/paginação/ordenação e utilitários de string/objeto — que todo
  projeto derivado herda sem ter pedido. São 23 arquivos versionados sob
  `libs/shared/`, com CHANGELOG, README e histórico de versões próprio. Quem usa
  o template precisa decidir, logo no primeiro dia, entre adotar utilitários que
  não escolheu, apagá-los à mão ou conviver com código morto. Nenhum projeto do
  workspace consome as duas libs: o grafo do Nx não registra dependência para
  elas, porque o template não tem apps.
- **Impacto**: o workspace derivado nasce com zero projetos Nx de produção,
  espelhando o que `AGENTS.md` já declara sobre apps ("não contém apps — quem
  parte deste template cria as suas"). A promessa do template passa a ser
  uniforme entre `apps/` e `libs/`: estrutura e tooling prontos, conteúdo
  nenhum.
- **Inspiração**: o próprio repositório, no tratamento dado a `apps/backend`,
  `apps/frontend`, `apps/serverless`, `libs/backend` e `libs/frontend` — todos
  diretórios de destino marcados com `.gitkeep`, sem projeto Nx registrado. Esta
  spec estende esse tratamento a `libs/shared/`.
- **Links relevantes**:
  - `docs/adr/006-fechamento-port-melhorias.md` — os itens 9 (libs mantidas) e 3
    (generator preservado quando a origem o esvaziou) registram as decisões
    opostas; esta spec substitui ambos
  - `docs/adr/001-baseline-monorepo.md` — descreve as duas libs como parte do
    baseline
  - `docs/adr/004-workflows-verdaccio-release.md` — split entre versionamento e
    publicação, que esta spec preserva
  - SPEC-4NR9KKS8 — portou o monorepo de origem e decidiu manter `types` e
    `utils` enquanto descartava `libs/shared/money`
  - `docs/nx-reference/tasks.md` — já registra o generator `shared-lib` como
    débito não resolvível e indica `@nx/js:lib` como caminho recomendado

<constraints>
- [P0] A infraestrutura de release DEVE permanecer armada e funcional. A chave
  `release` do `nx.json` (incluindo `projects: ["tag:type:lib"]`) e o
  `nx-publish-libs.yml` NÃO são alterados. O `nx-release.yml` recebe uma única
  alteração: o step `Detect lib release candidates` e as condições `if` que dele
  dependem. Sem esse guard o job falha, porque o `nx release` aborta com
  `RELEASE_GROUP_MATCHES_NO_PROJECTS` quando o release group não casa com projeto
  algum — não conclui vazio.
- [P0] O generator `tools/generators/shared-lib/` DEVE ser removido junto com as
  libs. Ele está obsoleto: a coleção não resolve (`Cannot find module
  '@nx-base-template/tools/package.json'`, débito já registrado em
  `docs/nx-reference/tasks.md`) e duplica o que `@nx/js:lib` faz. O caminho
  documentado para criar libs passa a ser o generator do Nx, que a própria
  referência de tasks já indica como recomendado.
- [P0] ADRs e specs existentes NÃO DEVEM ser reescritos. São registro histórico.
  A reversão dos itens 3 e 9 do ADR-006 é feita por um ADR novo que os substitui.
- [P0] A remoção NÃO DEVE ser promovida para uma branch `release/**` cujo ciclo
  ainda não fechou. O `nx-release.yml` dispara no fechamento de PR para `master`
  e versiona projetos `tag:type:lib`; se a remoção entrar na árvore promovida
  antes disso, o versionamento encontra o conjunto vazio. Implementar na branch
  de trabalho e abrir PR para `develop` é seguro e não afeta o ciclo em curso.
- [P1] `libs/shared/` DEVE permanecer versionado como diretório de destino, com
  `.gitkeep`, no mesmo padrão de `libs/backend/` e `libs/frontend/`.
- [P1] A documentação viva NÃO DEVE citar projetos inexistentes após a remoção.
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Remover as duas libs**: os diretórios `libs/shared/types/` e
  `libs/shared/utils/` DEVEM ser removidos do repositório, com todo o seu
  conteúdo versionado (`src/`, `package.json`, `project.json`, `tsconfig*.json`,
  `jest.config.cts`, `README.md`, `CHANGELOG.md`).
  - A remoção usa `trash`, nunca `rm` — o histórico do git preserva o conteúdo,
    mas a política do workspace proíbe exclusão permanente.
  - Artefatos de build sob `libs/shared/*/dist/` saem junto, por serem
    subdiretórios dos alvos.
- [x] **[P0] Preservar `libs/shared/` como destino**: criar
  `libs/shared/.gitkeep` para que o diretório continue versionado após ficar
  vazio, replicando `libs/backend/.gitkeep` e `libs/frontend/.gitkeep`.
- [x] **[P0] Limpar as referências de build**: remover as duas entradas de
  `paths` em `tsconfig.base.json` e as duas entradas de `references` em
  `tsconfig.json`. O `pnpm-lock.yaml` é regenerado por `pnpm install`, não
  editado à mão.
- [x] **[P1] Sincronizar a documentação viva**: `AGENTS.md`, `README.md`,
  `CONTRIBUTING.md`, `docs/onboarding.md`, `docs/nx-reference/tasks.md`,
  `docs/guides/development-workflow.md` e `.agents/skills/nx-commit/SKILL.md`
  DEVEM deixar de citar `shared-types` e `shared-utils` como projetos
  existentes. Onde a citação é um exemplo ilustrativo de comando ou de escopo de
  commit, substituir por um nome genérico (ex.: `minha-lib`), preservando o
  propósito didático do trecho.
- [x] **[P0] Remover o generator obsoleto**: `tools/generators/shared-lib/` e
  `tools/generators.json` DEVEM ser removidos via `trash`, com `.gitkeep` no
  lugar em `tools/generators/`. A chave `nx.generators` sai do `package.json`, e
  o aviso de débito conhecido sai de `docs/nx-reference/tasks.md`.
- [x] **[P1] Registrar a decisão em ADR**: criar um ADR em `docs/adr/` que
  substitui explicitamente os itens 9 e 3 da matriz do ADR-006, declarando por
  que a decisão mudou e o que permanece intacto (a infraestrutura de release).
- [x] **[P2] Revisar o CODEOWNERS**: confirmar que a entrada `/libs/shared/` em
  `.github/CODEOWNERS` permanece válida — o diretório continua existindo como
  destino, então a entrada é mantida.

### Não-funcionais

- [x] Integridade do build: `pnpm nx show projects` retorna apenas
  `@nx-base-template/source` após a remoção.
- [x] Compatibilidade: nenhuma alteração em versões de Node (`^24`) ou pnpm
  (`11.14.0`); nenhuma dependência adicionada ou removida do `catalog:`.
- [x] Reversibilidade: a remoção DEVE ser revertível por um único `git revert`
  do commit, sem passos manuais fora do repositório.
- [x] Rastreabilidade: as tags git já publicadas
  (`@lidercap-apps/shared-types@0.0.3`, `@lidercap-apps/shared-utils@0.0.3` e
  anteriores) e os pacotes no Verdaccio permanecem intocados. Esta spec não
  despublica nada.

## Camadas afetadas

| Camada | Afetada? | Descrição |
| --- | --- | --- |
| Libs (`libs/shared/*`) | [x] | Remoção integral de `types` e `utils`; `.gitkeep` no lugar |
| Config de build (`tsconfig.base.json`, `tsconfig.json`) | [x] | Remoção de 2 `paths` e 2 `references` |
| Lockfile (`pnpm-lock.yaml`) | [x] | Regenerado por `pnpm install`; nunca editado à mão |
| Config Nx (`nx.json`) | [ ] | Nenhuma mudança — `release.projects: ["tag:type:lib"]` permanece armado |
| CI/CD (`.github/workflows/`) | [x] | `nx-release.yml` ganha o guard `Detect lib release candidates`; `nx-publish-libs.yml` sem mudança |
| Tooling (`tools/generators/`) | [x] | Remoção do generator `shared-lib` e do `generators.json`; `.gitkeep` no lugar |
| Docs vivos (`AGENTS.md`, `README.md`, `docs/`) | [x] | Remoção das citações às duas libs |
| Skills (`.agents/skills/nx-commit/`) | [x] | Exemplos de escopo de commit trocados por nome genérico |
| ADRs e specs (`docs/adr/`, `docs/specs/`) | [x] | Somente adição de um ADR novo; nenhum documento existente é reescrito |

## Localização de código

```text
nx-base-template/
  tools/generators/         — destino de generators; fica vazio com .gitkeep
    shared-lib/             — REMOVER (8 arquivos)
  libs/shared/              — destino das libs; fica vazio com .gitkeep
    types/                  — REMOVER (10 arquivos versionados + dist/)
    utils/                  — REMOVER (13 arquivos versionados + dist/)
  tsconfig.base.json        — paths das duas libs
  tsconfig.json             — references das duas libs
  docs/adr/                 — ADR novo que substitui o item 9 do ADR-006
```

**Arquivos a modificar**:

- `libs/shared/types/` — remover o diretório inteiro (via `trash`)
- `libs/shared/utils/` — remover o diretório inteiro (via `trash`)
- `libs/shared/.gitkeep` — criar, para manter o diretório versionado
- `tsconfig.base.json` — remover as chaves `@lidercap-apps/shared-utils` e
  `@lidercap-apps/shared-types` de `compilerOptions.paths`
- `tsconfig.json` — esvaziar o array `references` (as duas entradas apontam para
  as libs removidas)
- `pnpm-lock.yaml` — regenerado por `pnpm install`
- `AGENTS.md` — a árvore de diretórios, a tabela de libs, os exemplos de comando
  e o exemplo de escopo de commit
- `README.md` — os dois exemplos `pnpm nx test @lidercap-apps/shared-utils`
- `CONTRIBUTING.md` — o exemplo de escopo de commit e o exemplo de nome de
  branch que cita `shared-utils`
- `docs/onboarding.md` — a tabela de libs e os exemplos de comando
- `docs/nx-reference/tasks.md` — os exemplos `nx show project`
- `docs/guides/development-workflow.md` — os exemplos de escopo de commit e de
  nome de branch
- `.agents/skills/nx-commit/SKILL.md` — a tabela de mapeamento path → escopo e
  os exemplos de comando
- `docs/adr/<n>-remocao-libs-exemplo.md` — ADR novo (numeração seguinte à última
  existente)

## Design

### Arquitetura

Antes:

```text
libs/
  backend/    .gitkeep            (destino vazio)
  frontend/   .gitkeep            (destino vazio)
  shared/
    types/    projeto Nx type:lib (publicado no Verdaccio)
    utils/    projeto Nx type:lib (publicado no Verdaccio)
```

Depois:

```text
libs/
  backend/    .gitkeep            (destino vazio)
  frontend/   .gitkeep            (destino vazio)
  shared/     .gitkeep            (destino vazio)
```

A infraestrutura que operava sobre as libs permanece montada e ociosa:
`nx.json` continua declarando `release.projects: ["tag:type:lib"]`, e os
workflows continuam registrados. Sem nenhum projeto carregando a tag `type:lib`,
o `nx release` não encontra alvo e não versiona — o mesmo comportamento que o
template já tem para apps, cujo passo de release Docker é pulado por não haver
projeto com `tag:type:app`.

### Fluxo principal

1. Confirmar que a remoção não entrará em uma branch `release/**` com ciclo
   aberto. Trabalhar na branch de trabalho e mirar o PR em `develop`.
2. Remover `libs/shared/types/` e `libs/shared/utils/` via `trash`.
3. Criar `libs/shared/.gitkeep`.
4. Limpar `tsconfig.base.json` e `tsconfig.json`.
5. Rodar `pnpm install` para regenerar o `pnpm-lock.yaml`.
6. Verificar que `pnpm nx show projects` retorna apenas
   `@nx-base-template/source`.
7. Sincronizar a documentação viva.
8. Escrever o ADR que substitui o item 9 do ADR-006.
9. Rodar a cadeia de validação do projeto.

## Decisões técnicas

- **Manter a infraestrutura de release intacta**: escolhido porque a
  infraestrutura é parte do valor entregue pelo template — quem o adota quer o
  pipeline de versionamento e publicação no Verdaccio já resolvido, não quer
  remontá-lo. Com o conjunto de projetos `tag:type:lib` vazio, o pipeline fica
  armado e ocioso até a primeira lib do consumidor, exatamente como o passo de
  release Docker já se comporta na ausência de apps.
  Alternativa descartada: remover os workflows e a chave `release` do `nx.json`
  junto com as libs, porque transferiria ao consumidor o trabalho que o template
  existe para poupar, e desfaria as decisões dos ADRs 004 e 008 sem que nada
  nelas tenha se mostrado errado.
- **Manter uma lib como exemplo vivo foi descartado**: uma lib "de exemplo"
  continua sendo código que o consumidor não pediu e precisa avaliar, apenas em
  metade do volume. O papel de demonstrar a estrutura correta (tags 3D,
  `publishConfig`, tsconfig, jest) cabe ao generator do Nx, documentado em
  `docs/nx-reference/tasks.md` com as flags e tags corretas.
- **`.gitkeep` em vez de deixar o diretório sumir**: escolhido para preservar a
  simetria com `libs/backend/` e `libs/frontend/`, e para que a entrada
  `/libs/shared/` do CODEOWNERS continue apontando para um caminho existente.
  Alternativa descartada: remover `libs/shared/` inteiro, porque a estrutura de
  escopos do workspace (`shared`, `backend`, `frontend`) ficaria assimétrica e a
  entrada do CODEOWNERS ficaria órfã.
- **Remoção via `trash`, não `rm`**: exigência da política de exclusão do
  workspace. O conteúdo permanece recuperável tanto pela lixeira quanto pelo
  histórico do git.
- **ADR novo em vez de editar o ADR-006**: escolhido porque um ADR registra a
  decisão tomada em um momento, com o contexto daquele momento. Reescrevê-lo
  apagaria o registro de que a decisão já foi outra, e por quê.

## Regras relacionadas

- `AGENTS.md` § Convenções obrigatórias — tags 3D e a regra de que o Nx Release
  publica apenas projetos com `type:lib`
- `AGENTS.md` § Regras duras — escopo de alterações e refatoração de símbolos
- `.claude/rules/git-safety.md` § Renomeação e movimentação de arquivos — buscar
  todas as referências antes de remover
- `docs/adr/README.md` — índice de ADRs, a ser atualizado com o ADR novo
- SPEC-4NR9KKS8 — decidiu manter as duas libs ao portar do monorepo de origem;
  esta spec reverte aquela decisão para `types` e `utils`

## Verificação e testes

### Critérios de aceite

- [x] `pnpm nx show projects` retorna exatamente `["@nx-base-template/source"]`
- [x] `libs/shared/` existe, contém apenas `.gitkeep` e está versionado
- [x] `tsconfig.base.json` não contém nenhuma ocorrência de
  `@lidercap-apps/shared-`
- [x] `tsconfig.json` não referencia `libs/shared/utils` nem `libs/shared/types`
- [x] `.github/workflows/nx-publish-libs.yml` e `nx.json` estão byte a byte
  idênticos aos do commit anterior
- [x] `.github/workflows/nx-release.yml` difere do commit anterior apenas pelo
  step `Detect lib release candidates` e pelas duas condições `if` que dele
  dependem
- [x] `tools/generators/shared-lib/` e `tools/generators.json` não existem mais;
  `tools/generators/` contém apenas `.gitkeep`
- [x] `package.json` não contém a chave `nx.generators`
- [x] Nenhum documento vivo cita `@nx-base-template/tools:shared-lib`
- [x] Nenhum arquivo em `docs/adr/` ou `docs/specs/` existente foi modificado,
  com a única exceção de `docs/adr/README.md` (índice) e desta spec
- [x] Uma busca por `shared-types|shared-utils` fora de `docs/adr/`,
  `docs/specs/` e `libs/shared/*/CHANGELOG.md` não retorna nenhuma citação a
  projeto existente
- [x] `pnpm biome ci .` passa
- [x] Validação do projeto passando: `pnpm nx affected -t lint,typecheck,test,build --exclude=@nx-base-template/source`

### Cenários de teste (mínimo 3)

```text
DADO o workspace com as duas libs removidas e libs/shared/.gitkeep criado
QUANDO o desenvolvedor roda `pnpm install && pnpm nx show projects`
ENTÃO a saída é exatamente ["@nx-base-template/source"], sem erro de
      resolução de path no TypeScript

DADO o workspace sem nenhum projeto com a tag type:lib
QUANDO o job version do nx-release.yml executa
ENTÃO o step Detect lib release candidates emite has_libs=false, os steps de
      versionamento e push são pulados, e o job conclui verde sem criar tag

DADO o mesmo workspace sem projeto type:lib
QUANDO `pnpm nx release --skip-publish` é chamado diretamente, sem o guard
ENTÃO o comando aborta com exit 1 e a mensagem "Release group __default__
      matches no projects" — é por isso que o guard existe

DADO o workspace já sem as libs
QUANDO o consumidor cria uma lib em `libs/shared/minha-lib` com as três tags 3D
ENTÃO ela entra em `pnpm nx show projects`, o guard passa a emitir has_libs=true
      e ela vira alvo do nx release na próxima execução

DADO um PR de release/X.Y.Z aberto e pendente de merge para master
QUANDO a remoção é aplicada na branch de trabalho, com PR mirando develop
ENTÃO o ciclo em curso não é afetado: a árvore promovida em release/X.Y.Z
      mantém as duas libs e o nx-release.yml versiona normalmente ao fechar

DADO o mesmo PR de release/X.Y.Z aberto
QUANDO alguém promove a remoção para dentro de release/X.Y.Z antes do merge
ENTÃO a mudança DEVE ser bloqueada pela constraint P0, porque o versionamento
      do ciclo passaria a encontrar o conjunto tag:type:lib vazio
```

<critical_constraints>
- [P0] A infraestrutura de release DEVE permanecer armada e funcional. A chave
  `release` do `nx.json` (incluindo `projects: ["tag:type:lib"]`) e o
  `nx-publish-libs.yml` NÃO são alterados. O `nx-release.yml` recebe uma única
  alteração: o step `Detect lib release candidates` e as condições `if` que dele
  dependem. Sem esse guard o job falha, porque o `nx release` aborta com
  `RELEASE_GROUP_MATCHES_NO_PROJECTS` quando o release group não casa com projeto
  algum — não conclui vazio.
- [P0] O generator `tools/generators/shared-lib/` DEVE ser removido: está
  obsoleto, não resolve e duplica `@nx/js:lib`.
- [P0] ADRs e specs existentes NÃO DEVEM ser reescritos. A reversão do item 9 do
  ADR-006 é feita por um ADR novo que o substitui.
- [P0] A remoção NÃO DEVE ser promovida para uma branch `release/**` cujo ciclo
  ainda não fechou. Implementar na branch de trabalho e abrir PR para `develop`
  é seguro.
- [P1] `libs/shared/` DEVE permanecer versionado como diretório de destino, com
  `.gitkeep`.
- [P1] A documentação viva NÃO DEVE citar projetos inexistentes após a remoção.
</critical_constraints>

## Escopo fora

- **Despublicar os pacotes do Verdaccio**: as versões
  `@lidercap-apps/shared-types@0.0.3` e `@lidercap-apps/shared-utils@0.0.3` (e
  anteriores) permanecem no registry. Despublicar é operação de plataforma, fora
  do repositório, e quebraria qualquer consumidor que já as tenha instalado.
- **Remover as tags git de versão**: as tags `{projectName}@{version}` já
  criadas são registro histórico do release. Removê-las reescreveria o histórico
  publicado.
- **Reescrever ADRs e specs existentes**: são registro do que foi decidido, com
  o contexto de quando foi decidido. A mudança de rumo entra como ADR novo.
- **Corrigir o generator `shared-lib` em vez de removê-lo**: seria trabalho de
  manutenção num utilitário que duplica `@nx/js:lib` e que nunca chegou a
  funcionar. A remoção é a simplificação coerente com o propósito desta spec.
- **Alterar a estrutura de `apps/`**: já está no formato de destino vazio; nada
  a fazer.
- **Migrar o conteúdo das libs para outro repositório**: se os tipos e
  utilitários tiverem valor para a organização fora do template, publicá-los a
  partir de um repositório próprio é trabalho de outra spec.
