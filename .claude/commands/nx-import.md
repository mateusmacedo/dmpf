---
description: Import, merge, or combine repositories into an Nx workspace using nx import. USE WHEN the user asks to adopt Nx across repos, move projects into a monorepo, or bring code/history from another repository.
allowed-tools: Bash, Read, Edit, Write, Glob, Grep
---

## Quick Start

- `nx import` brings code from a source repository or folder into the current workspace, preserving commit history.
- After nx `22.6.0`, `nx import` responds with .ndjson outputs and follow-up questions. For earlier versions, always run with `--no-interactive` and specify all flags directly.
- Run `nx import --help` for available options.
- Make sure the destination directory is empty before importing.
  EXAMPLE: target has `libs/utils` and `libs/models`; source has `libs/ui` and `libs/data-access` — you cannot import `libs/` into `libs/` directly. Import each source library individually.

Primary docs:

- https://nx.dev/docs/guides/adopting-nx/import-project
- https://nx.dev/docs/guides/adopting-nx/preserving-git-histories

## Import Strategy

**Subdirectory-at-a-time** (`nx import <source> apps --source=apps`):

- **Recommended for monorepo sources** — files land at top level, no redundant config
- Caveats: multiple import commands (separate merge commits each); dest must not have conflicting directories; root configs (deps, plugins, targetDefaults) not imported
- **Directory conflicts**: Import into alternate-named dir (e.g. `imported-apps/`), then rename

**Whole repo** (`nx import <source> imported --source=.`):

- **Only for non-monorepo sources** (single-project repos)
- For monorepos, creates messy nested config (`imported/nx.json`, `imported/tsconfig.base.json`, etc.)

### Directory Conventions

- **Always prefer the destination's existing conventions.** Source uses `libs/` but dest uses `packages/`? Import into `packages/` (`nx import <source> packages/foo --source=libs/foo`).
- If dest has no convention (empty workspace), ask the user.

### Application vs Library Detection

Before importing, identify whether the source is an **application** or a **library**:

- **Applications**: Deployable end products — Dockerfile, runnable entrypoint, no public API surface
- **Libraries**: Reusable packages — `"main"`/`"exports"` in `package.json`, named exports intended for import by other packages

**Destination directory rules**:

- Applications → `apps/<name>`
- Libraries → follow the dest's existing convention (`packages/`, `libs/`, etc.)

## Common Issues

### pnpm Workspace Globs (Critical)

`nx import` adds the imported directory itself (e.g. `apps`) to `pnpm-workspace.yaml`, **NOT** glob patterns for packages within it. Cross-package imports will fail with `Cannot find module`.

**Fix**: Replace with proper globs from the source config (e.g. `apps/*`, `libs/shared/*`), then `pnpm install`.

### Root Dependencies and Config Not Imported (Critical)

`nx import` does **NOT** merge from the source's root `dependencies`/`devDependencies`, `targetDefaults`, `namedInputs`, or plugin configurations.

**Fix**: Diff source and dest `package.json` + `nx.json`. Add missing deps, merge relevant `targetDefaults` and `namedInputs`.

### TypeScript Project References

After import, run `nx sync --yes`. If it reports nothing but typecheck still fails, `nx reset` first, then `nx sync --yes` again.

### Plugin Detection

- **Whole-repo import**: `nx import` detects and offers to install plugins. Accept them.
- **Subdirectory import**: Plugins NOT auto-detected. Manually add with `npx nx add @nx/PLUGIN`.
- Run `npx nx reset` after any plugin config changes.

## Technology-specific Guidance

Available references in `.agents/skills/nx-import/references/`:

- `ESLINT.md` — ESLint projects
- `GRADLE.md` — Gradle projects
- `JEST.md` — Jest testing setup
- `NEXT.md` — Next.js projects
- `TURBOREPO.md` — Turborepo migrations
- `VITE.md` — Vite projects
