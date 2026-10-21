---
id: SPEC-JN5CTRAX
slug: sonar-rating-a-seguranca-confiabilidade
title: Atingir rating A em Security e Reliability no SonarCloud
stage: done
priority: P1
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-137/atingir-rating-a-em-security-e-reliability-no-sonarcloud
subtask_urls: []
created: 2026-10-20
---

# SPEC-JN5CTRAX: Atingir rating A em Security e Reliability no SonarCloud

## Resumo

O projeto `mateusmacedo_dmpf` está com rating **C** em Security e **E** em
Reliability no SonarCloud. Esta spec corrige no código os alertas abertos
dessas duas qualidades até que ambos os ratings cheguem a **A**. Maintainability
já está em **A** e fica fora do escopo.

Como mantenedor, quero que o painel do SonarCloud reflita o estado real do
código, sem falsos negativos corrigidos por supressão.

## Contexto

- **Problema**: na análise de `develop` (commit `23db431a`), o SonarCloud
  registra 19 issues abertas de Security e 301 de Reliability. Os ratings
  atuais são C (Security) e E (Reliability). Medido pela API pública em
  2026-10-20.
- **Impacto**: o rating é o indicador de qualidade do repositório usado no
  gate de PR e na visão do time. Com E em Reliability, o indicador não
  comunica a saúde real do código.
- **Inspiração**: o próprio SonarCloud lista cada issue com regra, arquivo e
  linha. A correção segue a recomendação de cada regra, não supressão.
- **Links relevantes**:
  - Projeto SonarCloud: `mateusmacedo_dmpf` (organização `mateusmacedo`)
  - `sonar-project.properties` na raiz (exclusões e caminhos de cobertura)
  - `docs/specs/SPEC-Z2HM6NAP-endurecimento-borda-kernel.md` (segurança de borda)

<constraints>
- [P0] NUNCA marcar issues como `FALSE_POSITIVE`, `ACCEPTED` ou `WONT_FIX` no SonarCloud nesta spec. A correção é feita no código.
- [P0] NUNCA alterar teste para fazer a regra passar. Se o teste estiver errado, explicar antes de alterar.
- [P0] NUNCA alterar `sonar-project.properties` para excluir arquivos e reduzir o número de issues.
- [P1] Cada mudança de comportamento em script DEVE ser validada pelo gate do próprio script (ex: `tools/*-check.sh`) antes do commit.
- [P1] Correções de `shelldre:S7688` (`[` para `[[`) DEVEM preservar a semântica original em cada caso (glob, word splitting).
</constraints>

## Requisitos

### Funcionais

Os alertas abaixo são os medidos em 2026-10-20 (status `OPEN` ou `CONFIRMED`).

#### Security (19 issues)

- [ ] **[P0] Remoção do normalizador legado** (`tools/normalize-pruned-manifest.mjs`, S8707): o script não tem caller ativo e é removido. Com a remoção, o requisito de path traversal deixa de existir.
- [ ] **[P0] Permissões de workflow no nível errado** (S8233 em `nx-release.yml:27-28`; S8264 em `ci.yml:18`): mover `permissions` para o nível do job, mantendo o menor privilégio por job.
- [ ] **[P0] Actions sem pin por SHA** (S7637 em `nx-release.yml:239` e `publish-libs.yaml:93`): trocar a tag por SHA completo, com comentário da versão original.
- [ ] **[P1] `go.mod` sem `go.sum`** (S8566 em `libs/backend/go/application/go.mod` e `libs/backend/go/ports/go.mod`): avaliar se o módulo precisa de `go.sum`; gerá-lo ou justificar a ausência na issue.
- [ ] **[P1] Protocolo claro** (S5332): `tools/otel-env-check.sh:207,724,740` e `apps/backend/bff/app/probe.go:50`. Para cada caso, confirmar se o endpoint é local de teste. Se for, documentar o motivo no código com marcador `WHY:`. Se não for, trocar para HTTPS ou TLS.
- [ ] **[P2] `PATH` com diretórios graváveis** (S4036): `tools/dmpf-plugin/src/generators/bounded-context/generator.ts:510`, `tools/dmpf-conformance/internal/golist/golist.go:273`, `tools/dmpf-verify-emit.mjs:321,383`. Fixar o `PATH` de execução ou usar caminho absoluto de binário.

#### Reliability (301 issues)

- [ ] **[P0] BLOCKER em SQL sem `WHERE`** (`plsql:DeleteOrUpdateWithoutWhereCheck`, `libs/backend/go/postgres/inbox.sql:52`): analisar a query. Se a ausência de `WHERE` for um bug, corrigir. Se for intencional, justificar por escrito no PR (sem marcar no Sonar).
- [ ] **[P0] Shell `[` em vez de `[[`** (`shelldre:S7688`, 276 ocorrências em 16 arquivos, todas em `tools/` e `.github/skills/`): trocar por `[[` onde a semântica for equivalente. Revalidar cada script.
- [ ] **[P1] Cancel de contexto não diferido** (`godre:S8188`): `apps/backend/bff/app/sessionevents.go:74` (`stopPolling`) e `libs/backend/go/app/purge.go:79` (`cancel`). Adicionar `defer` logo após a criação do contexto.
- [ ] **[P1] Regex com backtracking** (`javascript:S8786`): `tools/adr-verify.mjs:19,127` e `tools/dmpf-verify.mjs:47,60`. Reescrever sem quantificadores aninhados, mantendo o comportamento dos testes.
- [ ] **[P1] Sort sem comparador** (`javascript:S2871`): `tools/adr-verify.mjs:586`, `tools/dmpf-verify-emit.mjs:199,244`, `tools/dmpf-verify.mjs:702`. Passar comparador explícito (`localeCompare` ou comparação numérica, conforme o dado).
- [ ] **[P2] `parseInt` sem `Number.`** (`javascript:S7773`, 12 ocorrências em `.github/skills/monitor-ci/scripts/`): trocar por `Number.parseInt`.
- [ ] **[P2] `async` sem `await`** (`typescript:S7503`): `tools/dmpf-plugin/src/generators/init/generator.ts:390` e `bounded-context/generator.ts:524`. Remover `async` quando não há `await`, ou manter se a assinatura exigir Promise.

### Não-funcionais

- [ ] Verificação: o rating **A** em Security e Reliability aparece no SonarCloud após a análise da branch.
- [ ] Ausência de regressão: `nx affected -t lint,typecheck,test,build` passa no mesmo commit.
- [ ] Ausência de supressão: nenhum issue marcado como `FALSE_POSITIVE`, `ACCEPTED` ou `WONT_FIX`.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Scripts de validação (`tools/`) | [x] | Shell, ESM, regex, sort e `PATH` |
| Workflows CI (`.github/`) | [x] | Permissões por job e pin de actions |
| Backend Go (`apps/`, `libs/backend/go/`) | [x] | Context cancel, probe HTTP, `go.sum` e SQL |
| Gerador TS (`tools/dmpf-plugin/`) | [x] | `PATH` e `async` |
| Contratos, infra, frontend | [ ] | Fora do escopo |

## Localização de código

Os caminhos exatos estão nos requisitos acima. Resumo por pasta:

```
tools/                       — scripts shell, ESM e checks (maior parte das issues)
.github/workflows/           — ci.yml, nx-release.yml, publish-libs.yaml
.github/skills/monitor-ci/   — parseInt em ci-poll-decide.mjs e ci-state-update.mjs
apps/backend/bff/app/        — probe.go, sessionevents.go
libs/backend/go/app/         — purge.go
libs/backend/go/postgres/    — inbox.sql
libs/backend/go/{application,ports}/ — go.mod
```

## Design

### Abordagem

Trabalho por categoria, não por arquivo, para que cada categoria tenha um
critério de pronto verificável. A ordem é: BLOCKER e P0 de segurança primeiro,
depois os demais por severidade.

### Fluxo principal

1. Corrigir os itens P0 de Security e o BLOCKER de Reliability.
2. Corrigir os itens P0 de Reliability (shell `[[`).
3. Corrigir os itens P1 e P2.
4. Rodar a validação do projeto e os gates DMPF.
5. Abrir o PR para `develop` e aguardar a análise do SonarCloud.
6. Confirmar os ratings A na análise da branch.

## Decisões técnicas

- **Correção no código, sem supressão**: os falsos positivos continuam abertos
  no SonarCloud. Motivo: decisão do mantenedor, para que o status do Sonar
  reflita a revisão humana e não um atalho. Alternativa descartada: marcar
  falsos positivos no Sonar, o que daria rating A sem mudar o código.
- **`[[` no lugar de `[`**: é a forma recomendada pela regra. Alternativa
  descartada: manter `[` e desativar a regra no `sonar-project.properties`,
  que também é supressão.
- **Threshold do rating A**: a regra exata que o SonarCloud usa para calcular
  o rating (quantidade de issues abertas por qualidade) ainda precisa ser
  confirmada na API antes de declarar o trabalho concluído. Esta spec não
  assume essa regra.

## Regras relacionadas

- `.claude/rules/process-enforcement.md` (plano antes de implementar)
- `.claude/rules/git-safety.md` (não alterar testes para passar)
- `docs/specs/SPEC-Z2HM6NAP-endurecimento-borda-kernel.md` (segurança de borda)

## Verificação e testes

### Critérios de aceite

- [ ] Security: 0 issues abertas (`OPEN`/`CONFIRMED`) com impacto em SECURITY no SonarCloud, e rating A.
- [ ] Reliability: 0 issues abertas (`OPEN`/`CONFIRMED`) com impacto em RELIABILITY no SonarCloud, e rating A.
- [ ] Maintainability: rating continua A após as mudanças.
- [ ] Nenhum status de issue alterado manualmente no SonarCloud.
- [ ] Validação do projeto passando (lint, typecheck, test).
- [ ] Gates DMPF (`dmpf-conformance`, `modsync --check`) passando.
- [ ] Scripts de `tools/` que tiveram linhas alteradas continuam passando seus checks.

### Cenários de teste

```
DADO o repositório após a remoção de tools/normalize-pruned-manifest.mjs
QUANDO uma busca por esse caminho é feita fora de plans/ e docs/
ENTÃO não há referências ativas ao script

DADO um script shell com comparação de padrão (ex: [ "$x" = *foo* ])
QUANDO a comparação for trocada para [[ ]]
ENTÃO o resultado do script continua o mesmo para as entradas de teste existentes

DADO o SQL de inbox.sql:52 sem WHERE
QUANDO a query é analisada
ENTÃO fica registrado se a ausência é bug (corrigido) ou intencional (justificado no PR)
```

<critical_constraints>
- [P0] NUNCA marcar issues como FALSE_POSITIVE, ACCEPTED ou WONT_FIX no SonarCloud.
- [P0] NUNCA alterar teste para fazer a regra passar.
- [P0] NUNCA alterar sonar-project.properties para excluir arquivos.
- [P1] Correções de `[` para `[[` DEVEM preservar a semântica.
</critical_constraints>

## Escopo fora

- **Maintainability (464 issues)**: rating já é A. Reduzir o volume de issues é trabalho separado.
- **Marcação de falsos positivos no SonarCloud**: decisão do mantenedor, fora desta spec.
- **Quality gate customizado**: mudança de configuração do SonarCloud, fora desta spec.
