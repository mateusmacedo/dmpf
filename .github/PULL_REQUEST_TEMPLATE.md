## Contexto

<!-- Descreva o que muda e por quê. Vincule a spec (docs/specs/) ou a issue quando houver. -->

## Tipo de mudança

<!-- Alinhe ao tipo do Conventional Commit usado no assunto do PR. -->

- [ ] `feat` — nova funcionalidade
- [ ] `fix` — correção de bug
- [ ] `refactor` — refatoração sem mudança de comportamento
- [ ] `chore` — manutenção, tooling ou dependências
- [ ] `docs` — documentação
- [ ] `ci` — pipelines e automação

## Base

<!-- Selecione a branch de destino. `develop` é o padrão do fluxo. -->

- [ ] `develop` — padrão (validação antes da promoção)
- [ ] `release/**` — promoção de fim de sprint
- [ ] `master` — exceção declarada (ex.: hotfix)

## Escopo

<!-- Marque as áreas do monorepo tocadas por este PR. -->

- [ ] `apps/`
- [ ] `libs/`
- [ ] `docs/`
- [ ] `.claude/`
- [ ] `.github/`
- [ ] `infra/`
- [ ] `tools/`

## Validações executadas

<!-- Rode a cadeia localmente e marque cada passo. Reflete o que o CI executa. -->

- [ ] `pnpm biome ci .`
- [ ] `pnpm nx affected -t lint --exclude=@mateusmacedo/dmpf-source`
- [ ] `pnpm nx affected -t typecheck --exclude=@mateusmacedo/dmpf-source`
- [ ] `pnpm nx affected -t test --exclude=@mateusmacedo/dmpf-source`
- [ ] `pnpm nx affected -t build --exclude=@mateusmacedo/dmpf-source`
- [ ] `pnpm nx affected -t e2e --exclude=@mateusmacedo/dmpf-source`

## Checklist de revisão

- [ ] Escopo do PR está claro e descrito no Contexto
- [ ] Alterações restritas ao escopo declarado
- [ ] Commits seguem Conventional Commits em PT-BR, com escopo = nome do projeto Nx sem o prefixo da org
- [ ] Projetos novos têm as três tags 3D (`type:`, `scope:`, `stack:`)
- [ ] Documentação atualizada quando aplicável
- [ ] Nenhum segredo ou credencial commitado

## Evidências

<!-- Cole a saída dos comandos, prints ou links (CI, PRs relacionados). -->
