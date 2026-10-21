# ADR-071: Escopo affected em todos os jobs de CI/CD

## Status

Aceito — 2026-10-21. Implementa SPEC-T87DFFH1. Emenda o [ADR-069](./069-esteira-de-ci-por-etapa-do-git-flow.md), nos itens 2, 3, 5, 6 e 7; o [ADR-035](./035-realizacao-postgres-da-outbox.md), no `cache: false` do `test-race`; o [ADR-040](./040-test-kits-golden-e-fitness-function-em-go.md), na fila serial do `test-race`; e o [ADR-030](./030-granularidade-modulo-go-e-bom.md), o [ADR-037](./037-observabilidade-otel-e-retry-por-conjuncao-em-go.md) e o [ADR-060](./060-o-dmpf-tem-consumidor-externo.md), nas emendas que o ADR-069 fez a eles sobre o `govulncheck` semanal e o input `full`.

## Contexto

O ADR-069 levou a esteira a escalonar pelas etapas do git-flow, mas deixou dois custos fixos. O primeiro está no PR para `develop`: a seleção de projetos por `affected` funcionava (um diff só em `apps/backend/bookings` afeta só `bookings`), e mesmo assim os gates sem noção de projeto rodavam em todo PR. No run 39479518368 (PR #59) foram o gate check do domain (150 s), os sete self-tests do platform (114 s), o generator check, o `init --dry-run`, os pins, Grafana e OTel (~55 s) e context check, modsync, infrasync e conformance (~20 s). O segundo está no PR para `master`: o input `full` trocava o `affected` por `run-many` em todos os projetos e ligava o `govulncheck`, o consumidor externo, o harness distribuído e o SonarQube Cloud sobre o repositório inteiro.

O `test-race` era o maior passo do job (307 s no PR #59) e rodava a cada push mesmo sem mudança de input, porque os alvos com infra tinham `cache: false` (ADR-035). Cada teste de banco cria o próprio banco `<projeto>_test_<id>` e o apaga ao fim (ADR-040), então o resultado depende do código, das migrations e da versão da infra, e não de estado herdado.

Dois gates reprovam sem diff: o vencimento de exceção do conformance (DMPF-X006) e o aviso novo do banco do `govulncheck`. Com o PR por `affected`, nenhum diff os dispararia.

O `detect-apps.yaml`, chamado pelo template de CD `cd-dev-hmg.yml`, calculava o diff à mão contra `github.event.before` e procurava `<app_dirs>/<pasta>/Dockerfile`. No layout `apps/backend/<app>` ele sempre devolvia `[]`.

## Decisão

1. **Todo passo de PR roda só o que o diff afeta.** Alvo Nx roda por `nx affected`, ou por `run-many -p` com a lista derivada do `affected`. Check sem noção de projeto roda se, e só se, o diff toca um arquivo que ele lê ou um projeto que ele valida. A action `.github/actions/dmpf-scope` (`scope.sh`) é a dona única desse critério e emite um output por check. O reutilizável `dmpf-go-ci.yml` e o `ci.yml` só leem esses outputs. Este ADR não repete a tabela de critérios: ela vive no `scope.sh` e na prova `tools/dmpf-scope-check.sh`.

2. **O PR para `master` também segue o `affected`, com base na `master`.** O diff de `release/X.Y.Z` contra `master` é tudo o que a release leva. O input `full` sai do reutilizável: um input que não faz nada enganaria o chamador. O `govulncheck` roda nos projetos do escopo (input `govulncheck`, ligado só no PR para `master`). O consumidor externo, o harness distribuído e o SonarQube Cloud seguem em jobs próprios e rodam só quando o escopo os alcança. A divisão por etapa do ADR-069 não muda; muda só o escopo dentro de cada job.

3. **Os vetores negativos rodam por gatilho, também no PR para `master`.** Cada vetor roda quando o diff toca o próprio script ou a ferramenta que ele prova, e o job `Vetores negativos` só sobe se algum vetor, o gate check do domain ou o selftest do Buf estiver no escopo.

4. **Gates que reprovam sem diff rodam num agendamento diário.** O `govulncheck.yml` roda às 09:00 UTC na `develop`, com dois jobs: `govulncheck` em todos os projetos Go e o `conformance` global. O consumidor externo não herda o agendamento, porque a `dmpf-scope` não tem modo agendado: sem diff, chamar o reutilizável num `schedule` pularia os dois gates. O guia `docs/guides/dmpf-consumo-externo.md` traz um workflow agendado próprio, com os mesmos dois jobs.

5. **Os testes com infra usam o cache do Nx.** Os alvos `test-race`, `test-integration`, `e2e` e `test-distributed` que dependem de `test-infra-up` ou `test-keycloak-up` têm `cache: true` e declaram como input o `infra/test/compose.yml`, onde as imagens são fixadas por digest, e o `.env.example`. O `authn` declara também o realm e o `seed.sh` do Keycloak. O `-count=1` continua no `go test`: ele impede o reaproveitamento de execução antiga dentro de um mesmo run, coisa que o cache do Nx não cobre. Os workflows leem as imagens do compose, que fica sendo a fonte única delas.

6. **A fila serial do `test-race` vem do `dependsOn`.** Roda em série, e primeiro, o provider (`layer:providers`) cujo `test-race` depende de `test-infra-up` ou `test-keycloak-up`. Antes o critério era o `cache: false`, que deixou de existir.

7. **O `detect-apps` usa o grafo do Nx.** O output `apps` passa a ser o root dos projetos de `nx show projects --affected --type app` que têm o `marker_file`. A base vem do `nrwl/nx-set-shas`: último run verde do workflow chamador na branch, para o mesmo evento. Assim, uma app cujo deploy falhou volta à fila. Os inputs `app_dirs` e `shared_dirs` saem, porque a propagação de uma lib às apps que dependem dela é o próprio grafo.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Acrescentar `--project` a cada ferramenta global (modsync, infrasync, conformance) | Custo alto de reescrita, com ganho marginal sobre pular o passo quando o diff não toca o que ela lê |
| Manter o `run-many` e o input `full` no PR para `master` | É o custo fixo que a SPEC-T87DFFH1 elimina; o diff da release contra a `master` já é o que vai a produção |
| Manter o input `full` como no-op | Um input que não faz nada engana o chamador; o template do consumidor não o passa |
| Manter conformance e `govulncheck` em todo PR | Contraria o escopo affected, e o PR seguiria sem ver o vencimento que acontece entre dois PRs |
| Agendamento no consumidor chamando o reutilizável | A `dmpf-scope` decide pelo diff; num `schedule` sem diff, conformance e `govulncheck` seriam pulados |
| Manter `cache: false` nos testes com infra (ADR-035) | O `test-race` é o maior passo do job e rodava a cada push sem mudança de input; com a infra fixada por digest nos inputs, o resultado é determinístico |
| Pular a subida da infra quando todos os testes vêm do cache | Exigiria saber o resultado do cache antes de rodar a task; a infra sobe junto do lint e do build, fora do caminho crítico |
| `detect-apps` com `--base=github.event.before` | Não cobre o `workflow_dispatch` e mantém a dependência do evento; um deploy que falhou não volta à fila |

## Consequências

**Positivas:**

- O PR que toca um projeto folha deixa de pagar pelos gates do repositório inteiro, e o PR só de documentação continua sem rodar o pesado.
- O PR de release paga pelo que a release muda em relação à `master`, não pelo workspace todo.
- O re-run no mesmo SHA serve os testes com infra do cache remoto: no PR #63, o job `DMPF` caiu de 10 min 55 s para 3 min 34 s.
- O vencimento de exceção e o aviso novo do `govulncheck` reprovam no dia em que acontecem, na `develop`, sem depender de um PR.
- O `detect-apps` volta a detectar as apps do layout `apps/backend/<app>`, e uma mudança numa lib alcança só as apps que dependem dela.

**Negativas:**

- A infra Docker do job sobe sempre que um projeto com infra está no escopo, mesmo quando todos os testes vêm do cache. **Custo aceito:** o boot corre em paralelo ao lint e ao build, e saber o resultado do cache antes da task exigiria um passo próprio por alvo.
- As flags de boot da infra e o `env` do job vivem no workflow e não entram no hash dos alvos com infra: mudar a forma de subir a infra sem mudar o compose, o `.env.example` ou o código serve o resultado antigo do cache. **Custo aceito:** a forma de subir a infra muda raramente e quase sempre junto do compose, que está no hash; pôr o workflow nos inputs faria qualquer edição de CI invalidar todos os testes com infra.
- Um defeito que só aparece em projeto fora do escopo da release não é visto no PR de release. **Custo aceito:** o grafo do Nx define o que a release alcança, e o projeto fora dele chega à `master` igual ao que já estava lá.
- Um gate global com critério de gatilho incompleto deixa de rodar quando deveria. **Custo aceito:** o critério tem um dono só, a prova `tools/dmpf-scope-check.sh` cobre cada gatilho, e a mudança na própria `dmpf-scope` ou no reutilizável liga todos os checks.
- O consumidor externo precisa montar o próprio agendamento para ter conformance e `govulncheck` sem diff. **Custo aceito:** o impacto no consumidor não limita o desenho do projeto (ADR-069, item 10), e o guia traz o workflow pronto.
- O primeiro `detect-apps` de uma branch, sem run verde anterior, usa o commit anterior (`HEAD~1`) como base. **Custo aceito:** o input `app` do `workflow_dispatch` continua disparando a app explicitamente.

## Referências

- SPEC-T87DFFH1 (`docs/specs/SPEC-T87DFFH1-ci-affected.md`) — requisitos, critério de cada check e decisões técnicas.
- [ADR-069](./069-esteira-de-ci-por-etapa-do-git-flow.md), [ADR-035](./035-realizacao-postgres-da-outbox.md), [ADR-040](./040-test-kits-golden-e-fitness-function-em-go.md), [ADR-030](./030-granularidade-modulo-go-e-bom.md), [ADR-037](./037-observabilidade-otel-e-retry-por-conjuncao-em-go.md) e [ADR-060](./060-o-dmpf-tem-consumidor-externo.md), emendados por este ADR.
- `docs/ci-cd/README.md` — «Esteira de CI por etapa».
- `docs/guides/dmpf-consumo-externo.md` — o job `DMPF` no consumidor e o workflow agendado.
- `.github/actions/dmpf-scope/scope.sh`, `tools/dmpf-scope-check.sh`, `.github/workflows/ci.yml`, `.github/workflows/dmpf-go-ci.yml`, `.github/workflows/govulncheck.yml`, `.github/workflows/detect-apps.yaml` e `.github/workflows/cd-dev-hmg.yml`.
