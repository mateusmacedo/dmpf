---
name: nx-commit
description: "Cria commits seguindo conventional commits com scope correto para projetos NX, garantindo incremento de versão por lib. USE SEMPRE que o usuário pedir para commitar mudanças neste workspace. Palavras-chave: 'commita', 'faz commit', 'cria commits', 'commitar'."
---

# NX Commit — Conventional Commits com Scope por Projeto

Cria commits organizados em ordem lógica de precedência, com scope mapeado
ao nome do projeto NX afetado, garantindo que o NX Release incremente a versão
correta de cada lib via conventional commits.

## Regra central

> **O scope do commit = nome do projeto NX** (sem o prefixo de organização).
> Ex.: `@mateusmacedo/minha-lib` → scope `minha-lib`.
>
> Commits que tocam arquivos de **projetos distintos** devem ser **commits distintos**.
> Nunca misture arquivos de libs diferentes no mesmo commit.

---

## Passo 1 — Levantar o estado atual

```bash
# Ver todos os arquivos modificados (staged + unstaged)
git diff --name-only HEAD
git diff --name-only --cached

# Ou de forma unificada:
git status --short
```

---

## Passo 2 — Mapear arquivos → projetos NX

Use o CLI do NX para obter a raiz (`root`) de cada projeto e relacionar com
os arquivos modificados:

```bash
# Obter todos os projetos com suas raízes
pnpm nx show projects --json | node -e "
const names = JSON.parse(require('fs').readFileSync('/dev/stdin','utf8'));
const { execSync } = require('child_process');
names.forEach(name => {
  const proj = JSON.parse(execSync('pnpm nx show project ' + name + ' --json').toString());
  console.log(proj.root + '\t' + name);
});
"
```

Cada arquivo modificado é atribuído ao projeto cujo `root` é prefixo do path.
Arquivos sem match pertencem à categoria **infraestrutura**.

### Categorias

| Categoria | Exemplos de path | Scope sugerido |
|-----------|-----------------|----------------|
| **Lib/App NX** | `libs/shared/minha-lib/src/…`, `apps/api/…` | nome do projeto (`minha-lib`, `api`) |
| **Release / CI** | `nx.json` (seção release), `.github/workflows/` | `release`, `ci` |
| **Workspace** | `nx.json` (demais seções), `tsconfig.base.json`, `biome.json` | `workspace` |
| **Generator / Tooling** | `tools/generators/…`, `tools/executors/…` | `generator` ou nome específico |
| **Documentação** | `docs/…`, `*.md` na raiz | `nx`, `adr` ou tema específico |

---

## Passo 3 — Determinar tipo de commit por grupo

Para cada grupo de arquivos, escolha o tipo mais alto aplicável:

| Tipo | Quando usar |
|------|-------------|
| `feat` | Novo comportamento ou funcionalidade adicionada |
| `fix` | Correção de comportamento incorreto ou bug |
| `refactor` | Reorganização sem mudança de comportamento |
| `chore` | Configuração, build, dependências — sem efeito em runtime |
| `docs` | Apenas documentação |
| `ci` | Arquivos de pipeline de CI/CD |

---

## Passo 4 — Ordenar os commits

Ordem lógica de precedência (o que outros commits dependem vem primeiro):

1. **`fix`/`chore` de infraestrutura** — nx.json, tsconfig, biome, pnpm-workspace
2. **`ci`** — workflows GitHub Actions
3. **`fix` de libs/apps** — cada projeto em commit separado
4. **`feat` de libs/apps** — cada projeto em commit separado
5. **`feat`/`chore` de generator/tooling** — tools/
6. **`docs`** — docs/, ADRs, READMEs

> **Por que essa ordem?** Fixes de infraestrutura habilitam comportamentos corretos
> que os commits seguintes dependem. Docs vêm por último pois não afetam versioning.

---

## Passo 5 — Criar os commits

Para cada grupo, stage apenas os arquivos daquele grupo e commite:

```bash
# Exemplo: fix de infraestrutura
git add nx.json
git commit -m "fix(release): <descrição concisa em PT-BR>"

# Exemplo: fix de lib — SEPARADO por projeto
git add libs/shared/minha-lib/project.json
git commit -m "fix(minha-lib): <descrição concisa em PT-BR>"

git add libs/shared/outra-lib/project.json
git commit -m "fix(outra-lib): <descrição concisa em PT-BR>"

# Exemplo: feat de lib
git add libs/shared/minha-lib/src/lib/sort.types.ts libs/shared/minha-lib/src/index.ts
git commit -m "feat(minha-lib): <descrição concisa em PT-BR>"
```

### Formato da mensagem

```
<tipo>(<scope>): <descrição imperativa e concisa em PT-BR>

[corpo opcional: contexto do por quê, não do o quê — máx. 5 linhas]
```

- Linha do título: ≤ 72 caracteres
- Scope: nome do projeto NX sem prefixo de organização, ou categoria de infraestrutura
- Descrição: imperativo presente ("adiciona", "corrige", "remove"), em PT-BR
- Corpo: apenas quando o motivo não é óbvio pela descrição

### Quando usar `git add -p` (patch)

Se um arquivo único (ex.: `nx.json`) contém mudanças de categorias diferentes
(ex.: fix de release + feat de workspace), use staging interativo para separar:

```bash
# Aceitar/rejeitar hunks individualmente
printf "y\ny\nn\n" | git add -p arquivo.json
```

---

## Verificação final

```bash
git log --oneline -10
```

Confirme que:
- Cada commit de lib usa o scope correto (nome do projeto NX)
- Commits de infraestrutura NÃO usam scope de projeto existente
- A ordem segue: infra → ci → fix libs → feat libs → tooling → docs
- Nenhum commit mistura arquivos de projetos NX diferentes
