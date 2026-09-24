---
id: SPEC-JPP31095
slug: dmpf-evidencia-certificacao-release
title: DMPF KRN-12.4 — Evidência de execução e certificação da release dmpf@0.1.0
stage: done
priority: P2
depends_on: [SPEC-6QT9SBAS, SPEC-H1A190Y8, SPEC-538MS2D4, SPEC-SJ66880S]
ticket_url: null
subtask_urls: []
created: 2026-09-08
---

# SPEC-JPP31095: DMPF KRN-12.4 — Evidência de execução e certificação da release dmpf@0.1.0

## Resumo

Quarta e última sub-spec de `SPEC-8HWBWJCB` (`KRN-12`). Entrega o instrumento
que transforma uma execução da suíte de `KRN-11` em **evidência endereçável e
hasheável** — `cmd/evidence`, que roda a suíte por subject, junta os
veredictos que os testes gravam e publica um envelope canônico em
`bom/evidence/<release>/` —, promove as entradas do BOM com
execução real a `certificada`, cunha a tag `dmpf@0.1.0`, consolida o ADR-041 e o
`AGENTS.md`, e fecha `KRN-12`.

Como usuário de uma squad, quero abrir o BOM, seguir `evidence_uri` e ver o
relatório exato — com digest que bate — da execução que certificou a combinação
que estou prestes a usar.

## Contexto

- **Problema**: o `golden.Report` nasce dentro do processo de teste e é
  consumido por `tb.RequireReport` (`contracts/golden/golden_test.go:135`);
  nada o grava em disco; `providerkit`/`serviceskit`/`domainkit` devolvem
  `Verdict` por valor; `appkit` e `distkit` exigem `testing.TB`. `go test -json`
  emite eventos com `Time` e `Elapsed`, não o `Report`. Sem instrumento, o
  `evidence_digest` de `BOM-03` não tem o que hashear.
- **Impacto**: cada entrada `certificada` aponta para um arquivo versionado
  cujo SHA-256 é o `evidence_digest`; `B005` e `B006` da sub-spec 3 passam a
  ter conteúdo.
- **Inspiração**: `golden.Report.MarshalJSON` (ordenação estável, `FIX-13`);
  `units-baseline.json` (`digest` no próprio documento).
- **Links relevantes**:
  - `SPEC-8HWBWJCB` — identidade da release; `version` efetiva
  - `SPEC-538MS2D4` — schema `dmpf/bom@1`; `B004`..`B006`
  - `SPEC-SJ66880S` / ADR-040 — "o `Report` é a evidência que `KRN-12`
    consome"; `test-race` cobre tudo menos `distkit`, que roda em
    `test-distributed`
  - `SPEC-6QT9SBAS` — `reference` como entrada do BOM
  - `SPEC-H1A190Y8` — plugin como entrada do BOM

### Divergências entre o ticket e o repositório

| O ticket diz | O repositório tem | O que esta spec adota |
| --- | --- | --- |
| "promovendo só as entradas com evidência da suíte de `KRN-11`" | nenhum kit grava relatório; `go test -json` não carrega o `Report` | os testes dos kits gravam o `Report` e o `Verdict` sob `DMPF_EVIDENCE_DIR` (`evidence.RecordReport` e `RecordVerdict`); `cmd/evidence` roda `go test -json` por subject, junta os registros e grava envelope canônico, **filtrando** `Time`/`Elapsed` do stream e guardando `Action`/`Test`/`Package` |
| — | `test-race` do testkit não cobre `distkit`; `test-distributed` é target próprio com `integration,distributed` | A evidência de Kafka/`franz-go` só existe se `test-distributed` rodou; a combinação que o inclui só entra em `compatible_with` com essa evidência |
| "BOM da release" | `package.json` em `0.0.0`; app não versiona | `version` efetiva (guarda-chuva): módulos `0.0.0`, app = SHA curto, externas do `go.mod` |
| "aprovador" | `BOM-05`: Plataforma com revisão de Arquitetura | `approved_by: "team:plataforma"` + `promoted: {by, reviewed_by, pr}` na entrada (`DMPF-B002`) |
| "PR que preenche o BOM" | repositório sem remote nesta release, por decisão do dono | promoção da `0.1.0` por merge local `--no-ff` em `develop`, com `promoted.pr` = `local-merge:feat/ARQ-548-evidencia-certificacao-release`; tag no merge local em `master` (ADR-041) |

### Fontes normativas

| Fonte | O que fixa |
| --- | --- |
| `BOM-03` (`:703-718`) | `evidence_uri` endereço estável; `evidence_digest` torna a evidência não substituível em silêncio |
| `BOM-04` (`:720-725`) | `compatible_with` exercitado |
| `BOM-05` (`:729-732`) | Autoridade de promoção |
| `BOM-07` linha `candidata → certificada` | Suíte e fixtures nomeadas; execução concluída; resultado aprovado; `compatible_with` exercitado; aprovador; validade |
| `BOM-08` | Validade declarada |
| `BOM-10` | Slot semconv com owner |
| FND-09 `FIX-13` (ADR-040) | `MarshalJSON` do `Report` é determinístico |
| `GOV-36` | Métricas junto do BOM da release |

<constraints>
- [P0] NUNCA promover a `certificada` entrada cuja evidência não foi gerada por `dmpf-evidence` sobre o commit da release: `evidence_digest` é o SHA-256 do arquivo em `bom/evidence/<release>/` e `evidence_uri` aponta para esse arquivo nesse commit.
- [P0] NUNCA listar em `compatible_with` combinação com Kafka sem `test-distributed` na evidência.
- [P0] NUNCA incluir campo temporal ou não determinístico no envelope: duas execuções sobre o mesmo commit e as mesmas versões produzem o mesmo digest.
- [P0] NUNCA cunhar `dmpf@0.1.0` antes do merge do PR do BOM: a tag aponta para o merge commit.
- [P1] O envelope nomeia suíte, fixtures, versões resolvidas, commit e infraestrutura usada — o que `BOM-07` exige da transição.
- [P1] `valid_until` = `certified_at` + 90 dias na primeira release (default revisável, ADR-041).
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Package `evidence` e `cmd/evidence`** no `testkit`
  (unidades `kernel/testkit-evidence` e `kernel/testkit-cmd-evidence`,
  bloco `app`). Invocação: `go run
  ./libs/backend/go/testkit/cmd/evidence --root . --release 0.1.0
  --out bom/evidence/0.1.0 [--subjects golden,provider,domain,services,app,dist,reference]`.
  - `header`: `{schema: "dmpf/evidence@1", release, subject, commit (SHA
    completo), goversion, tags, packages, modules: [{path, version do
    package.json}], externals: [{package, version do go.mod resolvido}], infra:
    {postgres: <SHOW server_version>, redpanda: <Admin API em
    DMPF_REDPANDA_ADMIN>}, tools}` — `infra` só no subject que usa a
    infraestrutura e `tools` (buf e protoc-gen-go) só no `golden`; sem `Time`,
    sem `Elapsed`, sem hostname.
  - Os testes gravam e o comando orquestra: o `golden` grava o `Report` por
    `evidence.RecordReport`; `provider` (`providerkit` sobre Postgres e
    memória), `domain` e `services` gravam o `Verdict` por
    `evidence.RecordVerdict`, sempre sob `DMPF_EVIDENCE_DIR`. O comando roda
    `go test -json -count=1 -p 1` por subject, com a variável apontando para um
    diretório temporário, e junta os registros ao stream.
  - `app`, `dist` e `reference`: só o stream do `go test -json`
    (`-tags=integration`; o `dist` com `integration,distributed`).
  - Stream filtrado para `{Package, Test, Action}` com `Action ∈ {pass, fail,
    skip}`, ordenado; `skip` sob `CI` reprova (`tb.Env`). Teste que pula por
    construção fica fora do subject por `-skip` com o nome exato:
    `TestUpdateGolden` no `golden` e `TestDistkitRole` no `dist`.
  - Saída: um arquivo por subject em `<out>/<subject>.json`, cada um com o
    `header` e o corpo; `index.json` com `{subject, sha256}` de cada; exit 1 se
    qualquer `Verdict`/`Report`/`Action` reprovar.
  - Target Nx `evidence` no `testkit` (`cache: false`, `dependsOn`
    `postgres:test-race`, `app:test-race`,
    `reference-go:test-race`), que roda o comando com `--release` lido
    de `bom/dmpf/`.
  - Edge case: `--subjects dist` sem `DMPF_KAFKA_BROKERS` sob `CI` → exit 1
    nomeando a variável; fora de `CI`, o subject é gravado como
    `{skipped: "DMPF_KAFKA_BROKERS"}` e **não** conta como evidência.
- [x] **[P0] Determinismo**: `evidence_test.go` roda `Write` duas vezes sobre o
  mesmo `Report`/`Verdict` → mesmo digest; `cmd_test.go` (integration) roda o
  comando inteiro duas vezes sobre o mesmo commit → `index.json` idêntico.
- [x] **[P0] Certificação da release `0.1.0`**: PR que preenche
  `bom/dmpf/0.1.0.json`:
  - `runtimes`: Go (`registry_ref` `go.work`), Node.js, TypeScript, pnpm, Nx —
    `certificada` os que a suíte exercita (Go); `candidata` os demais, com
    `reason`.
  - `generators`: Buf CLI e `protoc-gen-go` `certificada` pela evidência
    `golden` — o round-trip exercita os bytes que o plugin pinado gerou — com
    `registry_ref` para `tools/buf.sh` e `contracts/buf.gen.yaml`. OpenAPI
    generator: vazio com `reason`.
  - `drivers_clients_sdks`: `pgx` (`provider`), `franz-go` (`dist`), `grpc`
    e `aws-sdk-go-v2` (`candidata`: exercitados só por `test-race` dos
    providers, não pelo `dmpf-evidence` desta release — `reason`
    registrada), `otel` (`app`), `protobuf` (`golden`), `golangci-lint` e
    `govulncheck` (`candidata`, ferramentas), os 14 módulos do kernel
    (`version` `0.0.0`, `certificada` pela suíte que os cobre),
    `reference` (`version` SHA curto, `certificada` por `app` +
    e2e), `@mateusmacedo/dmpf-plugin` (`candidata`, exercitado por
    `dmpf-generator-check.sh` — `reason`).
  - `compatible_combinations`: a combinação Go × pgx × protobuf × otel ×
    módulos (`certificada`, evidência `provider`+`golden`+`app`) e a mesma
    com `franz-go` (`certificada` só se `dist` rodou).
  - `deprecations`: vazio com `reason`; `cves`: saída do `govulncheck` da
    release, ainda que vazia, com `evidence_uri` do job;
    `semantic_conventions_messaging`: `semconv/v1.43.0`, `registry_ref`
    `otelboot/start.go`, owner `team:plataforma`, `certificada` por `app`.
  - Cada `certificada`: `evidence_uri`
    `https://github.com/mateusmacedo/dmpf/blob/<sha>/bom/evidence/0.1.0/<subject>.json`,
    no SHA do commit que publica a evidência, `evidence_digest` do `index.json`,
    `approved_by: "team:plataforma"`, `certified_at` (data do commit de
    certificação), `valid_until` (+90 d), `promoted: {by,
    reviewed_by, pr}` — o ato de `BOM-05`; `history[]` é campo da exceção,
    não da entrada.
  - `exceptions: []`; `metrics` zeradas.
  - `dmpf-bom` passa no CI do PR.
- [x] **[P0] Tag `dmpf@0.1.0`**: anotada, no merge commit do PR do BOM em
  `master`, mensagem "DMPF release 0.1.0 — BOM bom/dmpf/0.1.0.json". Documentada
  em `bom/README.md` e no `CONTRIBUTING.md` como parte do rito de release.
- [x] **[P1] Consolidação**: ADR-041 final (decisões das quatro sub-specs,
  `valid_until` default, reconciliações); addenda em ADR-039 (`TRP-09`/
  `TRP-46` → task sucessora com ID) e ADR-034 (consumo externo → task
  sucessora); `AGENTS.md` consolidado (Apps, Libs, `tools/`, `bom/`,
  Comandos com `evidence`, `dmpf-bom`, generator, `serve-*`); `README.md` da
  raiz; `docs/guides/dmpf-composicao.md` seção "Como certificar";
  `docs/dmpf/README.md` aponta `bom/`.
- [x] **[P1] `SPEC-8HWBWJCB` → `done`** e `SPEC-YRJRADY9` (guarda-chuva do
  épico) com `KRN-12` marcado.

### Não-funcionais

- [x] Determinismo provado por execução dupla do comando completo.
- [x] Conformidade: `evidence` e `cmd-evidence` aprovados; baseline em commit
  próprio.
- [ ] Cadeia verde; `adr-verify`; `dmpf-verify`; `biome ci`.
  Pendente fora do escopo desta sub-spec: `biome ci`, `conformance` e
  `dmpf-bom` passam; `adr-verify` reprova com 50 violações (49 já na `develop`
  e 1 da regra que só admite um addendum por ADR, no ADR-041); `dmpf-verify`
  reprova no `C4` e o harness dele num teste histórico, ambos já na `develop`;
  `fitness/TestDomainTestsNeedNoInfrastructureDouble` já reprovava na `develop`.
- [x] Sem dependência nova (stdlib `crypto/sha256`, `encoding/json`,
  `os/exec` para `go test -json`).
- [ ] Prazo: `evidence` completo em menos de 10 min no runner com infra.
  Medido localmente com as imagens pinadas: 73 s e 64 s. O `dmpf-evidence.yml`
  ainda não rodou no runner, porque o repositório segue sem remote.

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | O que muda |
| --- | --- | --- |
| `domain` … `provider` | [ ] | — |
| `app` | [x] | `testkit/evidence`, `cmd/evidence` |
| Workspace | [x] | `bom/dmpf/0.1.0.json` (promoção), `bom/evidence/0.1.0/`, tag, `dmpf-evidence.yml` (reprodução por `workflow_dispatch`), `ci.yml` (gate do BOM nos PRs de promoção), docs, ADRs |

## Localização de código

```text
libs/backend/go/testkit/
  evidence/                                            — NOVO; unidade kernel/testkit-evidence (app)
    doc.go, header.go, record.go, gotest.go, write.go, evidence_test.go
  cmd/evidence/main.go, cmd_test.go               — NOVO; unidade kernel/testkit-cmd-evidence (app)
  project.json                                         — MODIFICAR: target evidence
  dmpf-units.json                                      — MODIFICAR: 2 unidades
  README.md                                            — MODIFICAR
bom/dmpf/0.1.0.json                                    — MODIFICAR: promoção
bom/evidence/0.1.0/{golden,provider,domain,services,app,dist,reference}.json, index.json — NOVO (commitados)
bom/README.md, CONTRIBUTING.md                         — MODIFICAR: rito de release e tag
tools/dmpf-baseline/units-baseline.json                — MODIFICAR em commit próprio
.github/workflows/dmpf-evidence.yml                    — NOVO: regenera a evidência no header.commit sob workflow_dispatch (input release) e compara com diff -r
.github/workflows/ci.yml                               — MODIFICAR: dmpf-bom com --base develop nos PRs de promoção
docs/adr/041-*.md, 039-*.md, 034-*.md                  — MODIFICAR
AGENTS.md, README.md, docs/guides/dmpf-composicao.md, docs/dmpf/README.md — MODIFICAR
docs/specs/SPEC-8HWBWJCB-*.md, SPEC-YRJRADY9-*.md      — MODIFICAR: stage/checklist
```

## Design

### Arquitetura

```text
 dmpf-evidence --release 0.1.0 --out bom/evidence/0.1.0
   header ── commit, goversion, tags, packages, modules(package.json), externals(go.mod), infra, tools
   go test -json por subject, DMPF_EVIDENCE_DIR num diretório temporário
   golden ── RecordReport (Report.MarshalJSON) + stream ──────► golden.json
   provider/domain/services ── RecordVerdict + stream ─────────► provider.json, domain.json, services.json
   app/dist/reference ── stream (Package, Test, Action) ───────► app.json, dist.json, reference.json
   index.json ── {subject, sha256}
        │
        ▼
 bom/dmpf/0.1.0.json ── evidence_uri (GitHub blob/<sha>/…), evidence_digest (index) ── dmpf-bom ✓ ── PR ── merge ── tag dmpf@0.1.0
```

### Fluxo — da suíte à tag

1. No commit candidato, `pnpm nx run testkit:evidence` gera
   `bom/evidence/0.1.0/*.json` e `index.json`; exit 0.
2. Roda de novo; `index.json` idêntico (prova local do determinismo).
3. O PR de certificação commita a evidência e preenche o BOM com os digests e
   as URIs no SHA do commit da evidência; `dmpf-bom` passa no CI.
4. Revisão de Arquitetura; merge por Plataforma (`BOM-05`).
5. Tag anotada `dmpf@0.1.0` no merge commit; `dmpf-bom` sem `--release`
   resolve o BOM e casa com a tag (`B011`).

### Onde cada regra é provada

| Regra | Instrumento | Onde |
| --- | --- | --- |
| `BOM-03` (`evidence_digest`) | `B005` sobre os arquivos commitados | CI |
| `BOM-04` | `B006`: cada `compatible_with` tem subject na evidência | CI |
| `BOM-07` (`candidata → certificada`) | `header` nomeia suíte, fixtures, versões; `B004` | CI |
| `FIX-13` / determinismo | `evidence_test.go`; `cmd_test.go` dupla execução | `test-race` (integration) |
| `KIT-06` (`V32`) na combinação com Kafka | `dist.json` exigido por `B006` | CI |
| `BOM-10` | entrada semconv `certificada` por `app` | CI |
| `BOM-02` / tag | `B011` casa release e tag | CI |

## Decisões técnicas

- **Kits por API, `go test -json` só onde há `testing.TB`** porque
  `golden`, `providerkit`, `domainkit` e `serviceskit` devolvem valor e podem
  ser invocados fora de `go test`; `appkit` e `distkit` exigem `TB`, e o
  stream `-json` filtrado de `Time`/`Elapsed` é determinístico. Alternativa
  descartada: reescrever `appkit` sem `TB`, porque muda `KRN-11`.
- **Evidência commitada** porque `evidence_uri` precisa de endereço estável
  e o `act_runner` não retém artefato entre releases; o digest torna a
  substituição detectável. Alternativa descartada: só URL do job de CI.
- **`version` efetiva** (guarda-chuva).
- **Tag anotada no merge commit** porque é o ato declarado que `BOM-07` e a
  guarda-chuva exigem; a tag do `nx release` é por projeto e não representa o
  produto.
- **Ferramentas e SDKs não exercitados pelo `dmpf-evidence` ficam
  `candidata`** (gRPC, SQS, golangci, govulncheck, plugin) porque `BOM-04`
  proíbe presumir; o `reason` nomeia o job de CI que os exercita e a release
  seguinte pode promovê-los com subject próprio. Alternativa descartada:
  certificar pelo `test-race` do CI, porque não há digest.
- **90 dias de validade** — default revisável no ADR-041.

## Regras relacionadas

- `SPEC-8HWBWJCB` — identidade da release; `version` efetiva.
- `SPEC-538MS2D4` — schema e `B004`..`B006`, `B011`.
- `SPEC-SJ66880S` — `test-distributed`; `tb.Env`.
- `.claude/rules/git-safety.md` — tag em `master` só após merge; sem force.

## Verificação e testes

### Critérios de aceite

- [x] O BOM está versionado e é revisável por PR; nenhum dos seis itens está
  ausente, e item sem instância aparece declarado vazio (critério 3 do ticket,
  instância real).
- [x] Nenhuma entrada `certificada` existe sem `evidence_uri`, `evidence_digest`,
  `approved_by`, `certified_at` e `valid_until`; toda versão em
  `compatible_with` tem execução na suíte de `KRN-11` (critério 4, instância
  real).
- [ ] O `AGENTS.md` reflete o inventário real, nenhum artefato declara
  exactly-once, e a validação do workspace passa (critério 6).
  Inventário e varredura de exactly-once atendidos; a validação do workspace
  carrega as falhas preexistentes listadas nos requisitos não-funcionais.
- [x] Duas execuções de `dmpf-evidence` sobre o mesmo commit produzem
  `index.json` idêntico.
- [x] A combinação com `franz-go` só está `certificada` se `dist.json` existe
  e passou.
- [x] Tag `dmpf@0.1.0` existe, anotada, no merge commit do PR do BOM;
  `dmpf-bom` sem `--release` passa.
- [x] ADR-041 final; addenda ADR-039 e ADR-034 citam as tasks sucessoras
  ARQ-549 (`KRN-13`) e ARQ-550 (`KRN-14`); `SPEC-8HWBWJCB` `done`;
  `SPEC-YRJRADY9` com `KRN-12` marcado.

### Cenários de teste

**Evidência (`evidence_test.go`)**

- Dado um `Report` fixo e um `header` fixo, quando `Write` duas vezes, então
  o mesmo SHA-256.
- Dado o mesmo `Report` com um `Outcome` a menos, então digest diferente.
- Dado um stream `go test -json` com `Time` e `Elapsed`, quando filtrado,
  então só `Package`, `Test`, `Action` restam, ordenados.
- Dado `Action: skip` e `CI` definida, então o subject reprova.

**Comando (`cmd_test.go`, integration)**

- Dado Postgres e Redpanda, quando o comando roda duas vezes, então
  `index.json` idêntico e exit 0.
- Dado `DMPF_KAFKA_BROKERS` ausente e `CI`, quando `--subjects dist`, então
  exit 1 nomeando a variável.

**BOM certificado (CI do PR)**

- Dado `bom/dmpf/0.1.0.json` promovido, quando `dmpf-bom --root . --release
  0.1.0`, então exit 0.
- Dado um digest alterado à mão, então `B005`.
- Dado `compatible_with` com `franz-go` e sem `dist.json`, então `B006`.

<critical_constraints>
- [P0] NUNCA promover a `certificada` entrada cuja evidência não foi gerada por `dmpf-evidence` sobre o commit da release: `evidence_digest` é o SHA-256 do arquivo em `bom/evidence/<release>/` e `evidence_uri` aponta para esse arquivo nesse commit.
- [P0] NUNCA listar em `compatible_with` combinação com Kafka sem `test-distributed` na evidência.
- [P0] NUNCA incluir campo temporal ou não determinístico no envelope: duas execuções sobre o mesmo commit e as mesmas versões produzem o mesmo digest.
- [P0] NUNCA cunhar `dmpf@0.1.0` antes do merge do PR do BOM: a tag aponta para o merge commit.
- [P1] O envelope nomeia suíte, fixtures, versões resolvidas, commit e infraestrutura usada — o que `BOM-07` exige da transição.
- [P1] `valid_until` = `certified_at` + 90 dias na primeira release (default revisável, ADR-041).
</critical_constraints>

## Escopo fora

- **Certificar gRPC, SQS, golangci-lint, govulncheck e o plugin**: ficam
  `candidata` com `reason`; subject próprio em release seguinte.
- **Reescrever `appkit`/`distkit` sem `testing.TB`**: muda `KRN-11`.
- **Publicar módulos Go com tag de módulo**: task sucessora (ADR-034).
- **Portal ou dashboard do BOM**: o BOM é JSON no repositório.
- **Segunda release (`N−1` simultânea, `GOV-11`)**: só faz sentido a partir
  de `0.2.0`.
