# ADR-069: Escalonar a esteira de CI pelas etapas do git-flow

## Status

Aceito — 2026-10-19. Emenda o [ADR-030](./030-granularidade-modulo-go-e-bom.md), no item das Consequências que manda o `test-race` e o `govulncheck` rodarem no push e no CI; o [ADR-037](./037-observabilidade-otel-e-retry-por-conjuncao-em-go.md), na obrigatoriedade do `govulncheck` no CI; o [ADR-040](./040-test-kits-golden-e-fitness-function-em-go.md), no CI em estágios por `layer:*` e na camada distribuída em pipeline separado; e o [ADR-060](./060-o-dmpf-tem-consumidor-externo.md), no workflow reutilizável e na prova do consumidor externo em todo PR.

## Contexto

A esteira de CI rodava o mesmo pacote em todo PR e de novo no push em `develop`, e boa parte do custo medido era repetição. Nos últimos sete dias o repositório gastou 1.758 runner-minutos: CI 68%, SonarQube Cloud 23%, Distribuído 6% e `govulncheck` 3%. A medição e a leitura dos workflows mostraram seis fatos:

- **O PR típico custava ~56 runner-minutos por push, em oito jobs**, e o push em `develop` reexecutava a esteira por ~50 runner-minutos, sobre código que o PR acabara de aprovar.
- **Cada job pagava 136 s de Setup Go, porque o cache nunca restaurava.** O `cache-dependency-path` do `setup-go` recebia o `github.action_path`, caminho absoluto que passa por `./` (no consumidor externo, por `.dmpf-ci/`), e o `setup-go` rejeita o caminho com `.` ou `..`. Sem cache, cada job recompilava o `golangci-lint`, o `govulncheck`, o `buf` e o `protoc-gen-go` por `go run`.
- **Os testes Go rodavam três vezes por PR**: o `test` em `Qualidade (affected)`, o `test-race` em `Pirâmide Go`, que repete o `test` com o race detector, e o `go test -tags=integration -coverprofile` do SonarQube Cloud, sobre todos os módulos.
- **Os vetores negativos dos gates custavam ~7,5 min por PR e tiveram 0 falhas em 300 execuções.** Eles provam o verificador, não o código, e só têm o que provar quando o diff toca o verificador.
- **O pre-push levava de 6 a 12 min**, porque rodava `build` e `test-race`, que sobe Docker, e o push por SSH caía por ociosidade antes de o hook terminar.
- **O Nx Cloud estava desligado por `neverConnectToCloud: true`**, embora o `nx.json` tivesse `nxCloudId`: nenhuma execução reaproveitava o cache de outra.

O git-flow do repositório (`CONTRIBUTING.md`) leva o trabalho a `develop` por PR e leva a `master` só por `release/X.Y.Z` ou `hotfix/*`, também por PR. O PR de release é o único caminho de código para a `master` e, por isso, o ponto natural da prova cara. O ruleset da `develop` exigia cada job como check, e um check que não dispara deixa o PR bloqueado: era o que impedia `paths-ignore` e obrigava o PR só de documentação a rodar o pipeline inteiro. O `dmpf-go-ci.yml` é o mesmo reutilizável que o `dmpf-ci.yml` do consumidor externo chama, fixado no `workflowRef` do `versions.json` (ADR-060).

## Decisão

1. **A etapa do git-flow decide o que roda.** A tabela por etapa vive em `docs/ci-cd/README.md`, na seção «Esteira de CI por etapa». O push em `develop` não roda CI: o PR já provou o código. O push em `master` roda só o `sonar.yml`, que mantém a análise da branch principal, referência das análises de PR no SonarCloud.

2. **PR para `develop` ou `release/**`: um job `DMPF`, por `affected`.** O `ci.yml` chama o reutilizável `dmpf-go-ci.yml`, que faz o setup uma vez e roda, em passos ordenados que reprovam por conta própria: `biome ci`; os gates estruturais (camada única por módulo, context-check estrutural, `modsync --check`, `infrasync --check` e `conformance`); `nx affected` de `lint`, `typecheck`, `build`, `fmt-check`, `vet`, `test` e dos gates Buf, numa invocação só; depois `test-race`, `test-integration` e `e2e`; e os checks do platform (generator estrutural, `init --dry-run` sem diff, `sync-versions --check`, Grafana e OTel). A infra Docker só sobe se algum projeto afetado a usa. A tag `layer:*` deixa de selecionar estágios do CI e passa a decidir só a concorrência do `test-race` (o provider com `cache: false` roda um por vez e divide a infra do job) e o guard de camada única. O PR para `develop` não roda `govulncheck`.

3. **Os vetores negativos só rodam se o diff toca o verificador.** O input `self-tests` liga quando o diff alcança `tools/**`, `.github/**` ou `.golangci.yml`, e sempre no PR para `master`.

4. **PR só de documentação não roda o pesado.** O job `Escopo` classifica o diff: `docs/**` e `*.md` fora de `tools/`, `.claude/` e `.agents/` valem como documentação, e o `DMPF` é pulado. O Markdown desses três diretórios é saída do `init` e não conta como documentação, porque alterá-lo faz o `init --dry-run` propor mudança.

5. **PR para `master`, vindo de `release/**` ou `hotfix/**`: pacote completo.** O input `full` troca o `affected` por `run-many` em todos os projetos e liga o `govulncheck`; os vetores negativos rodam todos; e o `ci.yml` chama, em jobs próprios, o `Consumidor externo por versão` (`tools/external-consumer.sh --verdaccio`), o `Distribuído` (`dmpf-distributed.yml`, a camada do `KIT-06` em pipeline separado, como o `KIT-11` exige) e o SonarQube Cloud (`sonar.yml`).

6. **Um check obrigatório, o job `CI`.** O job agrega `Escopo`, `DMPF`, consumidor externo, distribuído e Sonar, com `if: always()`: pulado conta como aprovado, falha ou cancelamento reprova. O ruleset da `develop` exige só ele. **A `master` não terá ruleset de checks.** Por decisão do mantenedor, ela só recebe código já validado pelo PR de release, que leva o pacote completo: o merge em `master` não é um segundo ponto de prova.

7. **`govulncheck` agendado, semanal e só na `develop`.** O `govulncheck.yml` roda às segundas-feiras, 09:00 UTC. O aviso novo do banco de vulnerabilidades reprova código que já passou no PR, e o PR para `develop` não o roda; a `master` recebe a mesma prova no PR de release.

8. **Pre-push mais curto.** O hook roda `nx affected -t lint,typecheck,fmt-check,vet,test`, sem Docker, contra o que o remoto já tem. `build`, `test-race`, os testes de integração e o `govulncheck` ficam no CI.

9. **Cache que restaura.** O composite `setup-go` calcula o `cache-dependency-path` relativo ao workspace, com `realpath --relative-to`. O `neverConnectToCloud` sai do `nx.json`: o Nx Cloud guarda o cache entre execuções, com o token no secret `NX_CLOUD_ACCESS_TOKEN` e, sem ele, o acesso que o `nxCloudId` concede ao workspace.

10. **O impacto no consumidor externo não limita a mudança.** O reutilizável é o mesmo que o consumidor chama, mas fixado no `workflowRef`: o consumidor adota o desenho novo ao subir a versão do plugin, e o guia `docs/guides/dmpf-consumo-externo.md` descreve o job `DMPF` e os inputs. O projeto decide a forma da própria esteira sem exigir compatibilidade com quem a consome.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Manter a esteira completa em todo PR e no push em `develop` | Custava ~56 runner-minutos por push de PR e reexecutava ~50 no merge, sobre código que o PR acabara de aprovar |
| Manter os oito jobs e só podar passos | Cada job repetiria o checkout e o setup de Node, pnpm e Go; o job único faz o setup uma vez e sobe a infra de testes uma vez |
| Rodar o pacote completo em todo PR para `develop` | Sonar (23%) e Distribuído (6%) pagariam em todo PR o que só protege a release |
| Ruleset de checks também na `master` | Exigiria de novo, sobre código idêntico, a prova que o PR de release já carrega |
| Manter o push em `develop` como rede de segurança | O PR roda contra a `develop` do momento; o risco restante, a interação entre PRs, é coberto pelo pacote completo do PR de release |
| `govulncheck` diário na `develop` e na `master` | O que muda por dia é o banco de vulnerabilidades, não o código; o semanal na `develop` e o do PR de release cobrem os dois |
| Vetores negativos em todo PR | ~7,5 min por PR, 0 falhas em 300 execuções; só há o que provar quando o diff toca `tools/**`, `.github/**` ou `.golangci.yml` |
| Pre-push com `test-race` e Docker | De 6 a 12 min por push, o que derrubava o push por SSH por ociosidade |
| Manter um passo do job por estágio `layer:*` | Cada estágio era uma invocação do Nx a mais sobre o mesmo grafo, e o `test-race` do `postgres` rodava em dois estágios por causa do `dependsOn` do `app` (ADR-040); uma invocação por tipo de target tira essa repetição, e a camada fica como tag de concorrência |

## Consequências

**Positivas:**

- O PR para `develop` paga um setup só e roda os targets só nos projetos afetados; os testes Go rodam duas vezes nele (`test` e `test-race`), e não três, porque o Sonar sai do PR de trabalho; e o PR só de documentação não roda o pesado.
- O push em `develop` e o push em `master` (fora o Sonar) deixam de gastar runner, e o `govulncheck` cai de diário em duas branches para semanal em uma.
- O cache de módulos e de build do Go passa a restaurar, e o Nx Cloud serve a mesma computação entre execuções.
- O pacote completo se concentra no PR de release, o único caminho de código para a `master`.
- O pre-push cabe na janela de ociosidade do push por SSH.

**Negativas:**

- Defeito que só o pacote completo vê (projeto não afetado, consumidor externo, harness distribuído, `govulncheck` sobre dependência nova) entra na `develop` e aparece no PR de release. **Custo aceito:** o PR de release e o de hotfix barram antes da `master`, e o `govulncheck` semanal cobre a `develop` entre as releases.
- A `master` sem exigência de checks aceita um merge cujo PR de release tenha check vermelho, e a plataforma não o impede. **Custo aceito:** é decisão do mantenedor, num projeto solo (ADR-058): quem faz o merge lê o `CI` do PR de release, e o fluxo do `CONTRIBUTING.md` só leva código à `master` por esse PR.
- O pipeline deixa de barrar camada a camada: o domínio e o provider quebrados reprovam no mesmo passo, e o Nx ordena pelo grafo de dependência. **Custo aceito:** a regressão de domínio ainda reprova antes de qualquer container subir, que é o feedback que o `KIT-09` buscava, e o `§8.3.1` de `docs/dmpf/testes-interop.md` passa a dizer em que etapa cada estágio roda.
- A quebra de um gate causada por mudança fora de `tools/**`, `.github/**` e `.golangci.yml` não é vista pelos vetores negativos até o PR de release. **Custo aceito:** os vetores provam o verificador, e o verificador só muda nesses caminhos.
- O consumidor externo que sobe o `workflowRef` passa a ter um job `DMPF` no lugar de dois, e o `govulncheck` só roda no `full`. **Custo aceito:** o impacto no consumidor não limita o desenho do projeto (item 10), e quem quiser o `govulncheck` em PR passa `full` ou o agenda por conta própria.

## Referências

- [ADR-030](./030-granularidade-modulo-go-e-bom.md), [ADR-037](./037-observabilidade-otel-e-retry-por-conjuncao-em-go.md), [ADR-040](./040-test-kits-golden-e-fitness-function-em-go.md) e [ADR-060](./060-o-dmpf-tem-consumidor-externo.md), emendados por este ADR; [ADR-058](./058-projeto-solo-sem-controles-de-segunda-pessoa.md), sobre o projeto de um mantenedor só.
- `docs/ci-cd/README.md` — «Esteira de CI por etapa» e os workflows do monorepo.
- `docs/dmpf/testes-interop.md` — `KIT-09` a `KIT-11` e `§8.3.1`.
- `docs/guides/dmpf-consumo-externo.md` — o job `DMPF` e os inputs `full` e `self-tests` no consumidor.
- `.github/workflows/ci.yml`, `.github/workflows/dmpf-go-ci.yml`, `.github/workflows/dmpf-distributed.yml`, `.github/workflows/sonar.yml`, `.github/workflows/govulncheck.yml`, `.github/actions/setup-go/action.yml`, `lefthook.yml` e `nx.json`.
