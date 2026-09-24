---
id: SPEC-538MS2D4
slug: dmpf-bom-validador-escape-hatch
title: DMPF KRN-12.3 — Modelo do BOM, validador dmpf-bom e escape hatch instrumentado
stage: done
priority: P2
depends_on: [SPEC-WTAXFV8B, SPEC-VVR1X71Q]
ticket_url: null
subtask_urls: []
created: 2026-09-08
---

# SPEC-538MS2D4: DMPF KRN-12.3 — Modelo do BOM, validador dmpf-bom e escape hatch instrumentado

## Resumo

Terceira sub-spec de `SPEC-8HWBWJCB` (`KRN-12`). Entrega o **modelo** do BOM
(`dmpf/bom@1`), o validador `dmpf-bom` no `conformance` — diagnósticos
`B001` a `B011`, fail-closed —, e o **escape hatch instrumentado**: um schema de
exceção com os quatro itens de `GOV-30`, o ciclo de vida de `GOV-34` com
`history[]`, a admissão de `GOV-32` por catálogo fechado N1–N7, aplicada ponta a
ponta no verificador (E1, manifesto) e no `dmpf-bom` (E2/E3, BOM), partilhando
`internal/exception`. Não certifica nada: o primeiro BOM com entradas
`certificada` é a sub-spec 4.

Como usuário de uma squad, quero que o repositório me diga, por regra e não por
opinião, se o BOM está completo, se uma certificação tem evidência e se o meu
pedido de exceção pode sequer ser examinado.

## Contexto

- **Problema**: nenhuma instância de BOM existe; o `exceptions[]` do
  `dmpf/units@1` tem cinco campos (`unit`, `dependency`, `reason`, `owner`,
  `review_by`), o decoder `fsstore` só conhece esses cinco
  (`fsstore/manifest.go:31-37`) e `politicaDoDocumento` copia só eles para
  `rule.ExceptionEntry` (`conformance/check.go:245-250`); `GOV-30` a `GOV-36`
  não têm instrumento.
- **Impacto**: o BOM passa a ser verificável no CI; a exceção passa a ter
  admissão mecânica; a sub-spec 4 tem onde gravar a certificação.
- **Inspiração**: `conformance/fitness` (package exportado, unidade
  `app`); `dmpf-units.json` e `units-baseline.json` (JSON como formato de
  dado verificável); `tools/adr-verify.mjs` (verificador de acervo com
  vetores).
- **Links relevantes**:
  - `SPEC-8HWBWJCB` — identidade da release (`dmpf@<semver>`); catálogo N1–N7;
    `history[]`; vencida **reprova**
  - `SPEC-VVR1X71Q` — FND-10, o template que esta spec instancia como schema
  - `SPEC-WTAXFV8B` — `KRN-02`, o verificador e o `internal/manifest`
  - ADR-012, ADR-015, ADR-027, ADR-031

### Divergências entre o ticket e o repositório

| O ticket diz | O repositório tem | O que esta spec adota |
| --- | --- | --- |
| "escape hatch instrumentado" com os quatro itens | `Exception{Unit, Dependency, Reason, Owner, ReviewBy}`; decoder e política só com esses campos | Extensão **compatível** do `dmpf/units@1` (todos os manifestos têm `exceptions: []`), levada por `fsstore` → `manifest` → `internal/exception` → `rule.ExternalPolicy`; teste ponta a ponta prova que só a E1 admitida autoriza o import nominal |
| "pedido sem prazo é recusado" | `GOV-34` admite prazo de **revisão** no lugar de convergência, com aprovação de Arquitetura **e** Plataforma e a condição que tornaria a convergência planejável (`governanca-bom-pilotos.md:931-937`) | `convergence` é união fechada: `{kind: plan, deadline, condition}` ou `{kind: review, review_by, approved_by: [arquitetura, plataforma], replanning_condition}`; sem nenhuma das duas datas → recusa (`GOV-30`) |
| "o BOM referencia os registros autoritativos sem duplicá-los (`BOM-06`)" | `BOM-03` exige `version` exata por entrada | Entrada com registro autoritativo leva `registry_ref` (`{file, selector}`) **e** `version`; `B007` resolve o valor pela referência e reprova divergência. A coexistência é registrada no ADR-041 |
| "entrada vencida é lida como `candidata` (`BOM-08`)" | `BOM-07`: "nenhuma transição ocorre por decurso de prazo sem ato declarado" | `B008` é **erro**: entrada `certificada` com `valid_until` vencido reprova o BOM até o commit que declara `state: candidata`; o validador não aceita a certificação vencida (`BOM-08`) e a transição continua sendo ato (`BOM-07`) |
| "métricas junto do BOM (`GOV-36`)" | nada persiste renovações | `history[]` por exceção; métricas derivadas; `B010` reprova valor declarado divergente |
| — | `include` do manifesto é import path exato; `internal/manifest` é unidade `domain` (`conformance/dmpf-units.json`) | `internal/exception` é unidade **nova** `conformance/exception`, bloco `domain`, recebe `now` por valor; `manifest` (domain) pode importá-la; `bom` (app) também |
| — | `libs/backend/go/conformance/internal/rule/codes.go` não existe; os códigos vivem em `internal/rule/diagnostic.go` | Caminho corrigido |

### Fontes normativas

| Fonte | O que fixa |
| --- | --- |
| FND-10 §4.1 (`:682-685`) | Seis itens canônicos |
| `BOM-01` a `BOM-10` (`:687-803`) | Item ausente reprova; um BOM por release; schema por entrada; `compatible_with` exercitado; autoridade; referência sem duplicação; cinco estados; validade; CVE; slot semconv |
| `BOM-07` tabela (`:754-771`) | Transições e evidência por transição |
| `GOV-26`, `GOV-30` a `GOV-36` (`:820-948`) | Escape hatch |
| `GOV-31` E1–E3 (`:865-869`); `GOV-32` N1–N7 (`:875-883`) | Universo positivo e negação |
| `GOV-34` (`:920-937`) | Cinco campos do ciclo de vida; ramo de revisão |
| ADR-015 | Política de capabilities por bloco — decide N1 para E1 em `domain`/`port` |
| ADR-027 | BOM por combinação certificada |
| RFC §7 (`decide`), §10.2 T1–T6, §12.3 âncoras; `BUF-06` | Os objetos de N2, N4, N3 e N6 |

<constraints>
- [P0] NUNCA aceitar entrada `certificada` sem `evidence_uri`, `evidence_digest`, `approved_by`, `certified_at` e `valid_until` (`B004`), nem com `evidence_digest` diferente do SHA-256 do arquivo (`B005`), nem com `compatible_with` sem entrada na evidência (`B006`).
- [P0] NUNCA admitir exceção com `kind` fora de E1–E3 (`X002`) nem cujo objeto caia no catálogo N1–N7 (`X003`..`X004`); a classificação é por estrutura, nunca por texto.
- [P0] NUNCA autorizar import por exceção não admitida: `rule.ExternalPolicy.exceptionFor` só vê entradas que passaram por `exception.Admit`.
- [P0] NUNCA aceitar BOM com um dos seis itens ausente (`B001`); item vazio existe com `reason`.
- [P0] NUNCA deixar `B008` como aviso: certificação vencida reprova até o ato declarado.
- [P1] `registry_ref` obrigatório para sujeitos da tabela de `BOM-06`; `B007` resolve e compara.
- [P1] `metrics` do BOM são derivadas de `history[]`; valor declarado divergente reprova (`B010`).
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Schema `dmpf/bom@1`** (`bom/README.md` documenta; `bom/schema.go`
  tipa): documento `{schema, product: "dmpf", release: <semver>, tag:
  "dmpf@<semver>", runtimes[], generators[], drivers_clients_sdks[],
  compatible_combinations[], deprecations[], cves[],
  semantic_conventions_messaging, exceptions[], metrics}`. Os seis itens de
  §4.1 são as seis seções; seção sem instância é `{entries: [], reason:
  "<não vazio>"}`. Cada entrada de seção segue `BOM-03`: `subject`
  (`product|kernel|contract|runtime|generator|driver|client|sdk`),
  `identity`, `version` (exata; faixa reprova), `state`
  (`proposta|candidata|certificada|depreciada|nao_suportada|rejeitada`),
  `criticality` (`critica|padrao`; ausente = `critica`), `compatible_with[]`
  (`{identity, version, evidence}`), `registry_ref` (`{file, selector}`,
  obrigatório quando o sujeito consta da tabela de `BOM-06`), `evidence_uri`,
  `evidence_digest` (`sha256:<hex>`), `approved_by`, `certified_at`,
  `valid_until`, `promoted` (`{by, reviewed_by, pr}`, obrigatório quando
  `certificada` — registra o ato de `BOM-05`), `deprecated_at`, `successor`,
  `cve[]` (sempre presente; `{id, state: corrigida|mitigada|aberta, owner}`).
  `semantic_conventions_messaging` é uma entrada como as outras, com `owner`
  (`BOM-10`).
- [ ] **[P0] Package `bom` e `cmd/bom`** no `conformance` — duas
  unidades novas, ambas bloco `app`, com `include` exato: `conformance/bom`
  e `conformance/cmd-bom` (a unidade `conformance/cmd` existente
  cobre só `cmd/conformance`). Invocação: `go run
  ./libs/backend/go/conformance/cmd/bom --root . [--release <semver>]
  [--now <RFC3339>]`. `--now` existe para os testes; ausente usa o relógio do
  sistema **no `cmd`**, passado por valor ao package. Diagnósticos:

  | Código | Regra | Condição |
  | --- | --- | --- |
  | `DMPF-B001` | `BOM-01` | seção ausente, ou vazia sem `reason` |
  | `DMPF-B002` | `BOM-03` | campo obrigatório ausente; `version` com faixa; `state`/`criticality`/`subject` fora do conjunto |
  | `DMPF-B003` | `BOM-07` | transição inválida entre o BOM anterior (`--base`, opcional) e o atual, ou `depreciada` sem `deprecated_at`/`successor` |
  | `DMPF-B004` | `BOM-07` | `certificada` sem um dos cinco campos de evidência |
  | `DMPF-B005` | `BOM-03` | `evidence_digest` ≠ SHA-256 do arquivo em `bom/evidence/<release>/` |
  | `DMPF-B006` | `BOM-04` | `compatible_with` com `{identity, version}` sem execução na evidência referenciada |
  | `DMPF-B007` | `BOM-06` | valor resolvido por `registry_ref` ≠ `version` declarada; `registry_ref` ausente em sujeito da tabela |
  | `DMPF-B008` | `BOM-08` | `certificada` com `valid_until < now` — **erro** |
  | `DMPF-B009` | `BOM-03`, `BOM-09` | `cve` ausente; CVE `aberta` sem `owner` |
  | `DMPF-B010` | `GOV-36` | `metrics` declaradas ≠ derivadas de `exceptions[].history` |
  | `DMPF-B011` | `BOM-02` | mais de um arquivo em `bom/dmpf/` sem `--release`; `release` do documento ≠ nome do arquivo; `tag` ≠ `dmpf@<release>` |

  `registry_ref.file` aceito: `go.work` (`selector: go`), `tools/buf.sh`
  (`selector: buf`), `contracts/buf.gen.yaml` (`selector: plugin:<nome>`),
  `nx.json` (`selector: golangci-lint`), `<módulo>/go.mod` (`selector:
  require:<pacote>`), `<módulo>/package.json` (`selector: version`),
  `package.json` (`selector: packageManager|engines.node`),
  `pnpm-workspace.yaml` (`selector: catalog:<pacote>`),
  `libs/backend/go/observability/otelboot/start.go` (`selector:
  semconv`).
  - Sai com 0 sem diagnóstico; 1 com diagnóstico; 2 com erro de leitura.
  - Roda no `ci.yml` logo após o `conformance`, no bloco "Gates DMPF".
- [ ] **[P0] Schema de exceção** (comum; `internal/exception/schema.go`):
  `id` (nominal, `^X-[a-z0-9-]+$`), `object` (`{kind:
  external-dependency|bom-combination|governance-instrument, unit, identity}`),
  `adr` (`^ADR-[0-9]{3}$`), `owner` (equipe; `^team:`), `justification`,
  `convergence` (união: `{kind: plan, deadline, condition}` | `{kind: review,
  review_by, approved_by: [arquitetura, plataforma], replanning_condition}`),
  `valid_from`, `valid_until`, `review_by` (≤ `valid_until`), `history[]`
  (`{event: granted|renewed|revoked|converged, at, by, reason}`; o primeiro é
  sempre `granted`). Campos legados `unit`, `dependency`, `reason`, `owner`,
  `review_by` continuam aceitos e mapeados para `object.unit`,
  `object.identity`, `justification`, `owner`, `review_by`; exceção só com
  legados reprova em `X001` por falta de `adr` e `convergence`.
- [ ] **[P0] Admissão `exception.Admit(x, now) []Diagnostic`** — catálogo
  fechado:

  | Código | Item | Reconhecimento |
  | --- | --- | --- |
  | `DMPF-X001` | `GOV-30` | falta `adr`, `owner`, `justification` ou `convergence` válida; `id` não nominal (`GOV-33`) |
  | `DMPF-X002` | `GOV-31` | `kind` fora de E1–E3 |
  | `DMPF-X003` | N2 | `object.identity` nomeia aresta entre unidades (`<unit>-><unit>`) ou célula (`cell:<n>`); ou E1 cujo `unit` é `domain`/`port` e a capability do pacote (allowlist ou ADR-015) não é `pure` — a política de bloco é P0-1, N1 |
  | `DMPF-X004` | N1, N3, N4, N5, N6, N7 | `object.identity` com prefixo reservado: `P0-`, `ANC-`, `T[1-6]$`, `block`/`bounded_context`, `tools/buf.sh`/`buf.gen.yaml`/`github.com/bufbuild/buf`/`protoc-gen-`, `evidence_` |
  | `DMPF-X005` | `GOV-34` | `review_by > valid_until`; `valid_from > valid_until`; `history[0] != granted`; `renewed` sem `reason` |
  | `DMPF-X006` | `GOV-34` vencimento | `valid_until < now` sem `renewed` posterior — a unidade fica não conforme |
  | `DMPF-X007` | `GOV-35` | E1 fora do manifesto da unidade, ou E2/E3 fora do BOM |

  Um vetor negativo por item N1–N7 em `admit_test.go`.
- [ ] **[P0] Ponta a ponta no verificador** (E1): `fsstore/manifest.go`
  decodifica os campos novos e legados; `manifest.Exception` ganha os campos;
  `manifest.Validate` delega `exceptions[]` a `exception.Admit(x, now)` (o
  `now` chega por `Input` do `conformance.Check`, lido no `cmd`);
  `politicaDoDocumento` só copia para `rule.ExceptionEntry` as exceções
  **admitidas**; `rule.ExternalPolicy.exceptionFor` inalterado. Teste
  `check_e2e_test.go`: manifesto com E1 completa autoriza exatamente o import
  nominal; com E1 sem `adr`, o import reprova como se a exceção não existisse
  e o relatório traz `X001`.
- [ ] **[P0] Unidade `conformance/exception`**, bloco `domain`, `include`
  `…/conformance/internal/exception`; sem `time` (o `now` é valor);
  baseline em commit próprio. Unidades `conformance/bom` e
  `conformance/cmd-bom`, bloco `app`.
- [ ] **[P1] BOM inicial vazio** `bom/dmpf/0.1.0.json` com as seis seções, todas
  as entradas em `candidata` ou `proposta`, sem evidência; `dmpf-bom` passa
  (nada `certificada`); a sub-spec 4 promove. `bom/README.md` explica o
  schema, a máquina de estados, como pedir exceção e os códigos.
- [ ] **[P1] Documentação**: `conformance/README.md` (dmpf-bom, códigos);
  `docs/guides/dmpf-composicao.md` seção "Divergir do golden path" com um pedido
  admitido e um recusado; `AGENTS.md` Comandos; addendum no ADR-041 com a
  reconciliação `BOM-03`/`BOM-06` e a leitura de `BOM-07`/`BOM-08`.

### Não-funcionais

- [ ] Determinismo: `dmpf-bom` com `--now` fixo produz o mesmo relatório.
- [ ] Conformidade: as três unidades novas aprovadas; `exception` sem I/O nem
  relógio.
- [ ] Cadeia verde; `adr-verify`; sem dependência Go nova (`crypto/sha256` e
  `encoding/json` da stdlib).
- [ ] Prazo: `dmpf-bom` sobre o BOM inicial em menos de 2 s.

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | O que muda |
| --- | --- | --- |
| `domain` | [x] | `internal/exception` (novo); `internal/manifest` (`Exception` estendida; `Validate` delega); `internal/rule/diagnostic.go` (`B*`, `X*`) |
| `application` | [x] | `internal/conformance/check.go` (`now` no `Input`; só admitidas viram `ExceptionEntry`) |
| `port` | [ ] | — |
| `contract` | [ ] | — |
| `provider` | [x] | `internal/fsstore/manifest.go` (decodifica campos novos e legados) |
| `app` | [x] | `bom/` package, `cmd/bom`; `cmd/conformance` (`--now`) |
| Workspace | [x] | `bom/` (raiz), `ci.yml`, `units-baseline.json` (commit próprio), docs |

## Localização de código

```text
bom/                                                  — NOVO
  README.md                                           — schema, estados, exceção, códigos
  dmpf/0.1.0.json                                     — BOM inicial (sem certificada); a sub-spec 4 promove
libs/backend/go/conformance/
  bom/                                                — NOVO package; unidade conformance/bom (app)
    schema.go, decode.go, validate.go, digest.go, registry.go, transitions.go, metrics.go, *_test.go, testdata/
  cmd/bom/main.go                                — NOVO: --root, --release, --now; unidade conformance/cmd-bom (app)
  cmd/conformance/main.go                        — MODIFICAR: --now (default relógio) passado ao Check
  internal/exception/                                 — NOVO; unidade conformance/exception (domain)
    schema.go, admit.go, denied.go, lifecycle.go, admit_test.go, testdata/
  internal/manifest/schema.go                          — MODIFICAR: Exception com campos novos (+ Present*)
  internal/manifest/validate.go                        — MODIFICAR: delega exceptions[] a exception.Admit
  internal/fsstore/manifest.go                         — MODIFICAR: wireException com campos novos e legados
  internal/conformance/check.go                        — MODIFICAR: Input.Now; politicaDoDocumento só com admitidas
  internal/conformance/check_e2e_test.go               — NOVO
  internal/rule/diagnostic.go                          — MODIFICAR: CodeB001..B011, CodeX001..X007
  dmpf-units.json                                      — MODIFICAR: 3 unidades novas
  README.md                                            — MODIFICAR
tools/dmpf-baseline/units-baseline.json                — MODIFICAR em commit próprio
.github/workflows/ci.yml                               — MODIFICAR: dmpf-bom após conformance
docs/guides/dmpf-composicao.md, AGENTS.md, docs/adr/041-*.md — MODIFICAR
```

## Design

### Arquitetura

```text
 <módulo>/dmpf-units.json exceptions[] (E1) ──► fsstore.DecodeManifest ──► manifest.Validate ──► exception.Admit(x, now)
                                                                                                      │ admitidas
                                                                                                      ▼
                                                                          conformance.politicaDoDocumento ──► rule.ExternalPolicy.exceptionFor
 bom/dmpf/<release>.json exceptions[] (E2/E3) ──► bom.Decode ──► bom.Validate(now) ──► exception.Admit(x, now) + B001..B011
                                                       │
                                                       ├── digest.go: sha256(bom/evidence/<release>/<subject>.json) == evidence_digest
                                                       ├── registry.go: resolve registry_ref → compara version (B007)
                                                       ├── transitions.go: BOM-07 entre --base e atual (B003); vencida (B008)
                                                       └── metrics.go: deriva de history[] (B010)
 internal/exception (domain, puro, now por valor) ◄── importado por manifest (domain) e bom (app)
```

### Fluxo 1 — validação do BOM

1. `cmd/bom` lê `--root`, resolve `--release` (único arquivo em
   `bom/dmpf/` ou erro `B011`), lê `now`.
2. `bom.Decode` → `bom.Validate(doc, now, fs)`: seções (`B001`), campos
   (`B002`), estados e transições (`B003`, `B008`), evidência (`B004`,
   `B005`, `B006`), registros (`B007`), CVE (`B009`), exceções
   (`exception.Admit`), métricas (`B010`), identidade (`B011`).
3. Relatório ordenado por código e caminho; exit 0/1/2.

### Fluxo 2 — admissão de uma exceção E1

1. A squad adiciona a exceção ao `exceptions[]` do manifesto da unidade.
2. `conformance` decodifica, `manifest.Validate` chama
   `exception.Admit`: `X001`..`X007`.
3. Só a exceção admitida entra na `ExternalPolicy`; o import nominal passa;
   qualquer outro import externo da unidade continua reprovado.
4. Vencida sem `renewed`, a exceção deixa de autorizar e `X006` reprova a
   unidade.

### Onde cada regra é provada

| Regra | Instrumento | Onde |
| --- | --- | --- |
| `BOM-01`..`BOM-10` | `bom/validate_test.go`: um vetor negativo por `B001`..`B011` | `test-race` |
| `BOM-06` | `registry_test.go`: cada `selector` resolvido contra fixture | `test-race` |
| `BOM-07`/`BOM-08` | `transitions_test.go`: transição inválida; vencida reprova; commit que declara `candidata` passa | `test-race` |
| `GOV-30`, `GOV-33` | `admit_test.go`: `X001` por item faltante; `id` não nominal | `test-race` |
| `GOV-31`, `GOV-32` N1–N7 | `admit_test.go`: um vetor por N (`X002`, `X003`, `X004`) | `test-race` |
| `GOV-34` (ramo de revisão; vencimento) | `admit_test.go`: `review` sem aprovação dual → `X001`; vencida → `X006` | `test-race` |
| `GOV-35` | `admit_test.go`: E2 no manifesto → `X007` | `test-race` |
| `GOV-36` | `metrics_test.go` | `test-race` |
| Ponta a ponta E1 | `check_e2e_test.go` | `test-race` |
| Classificação das unidades novas | verificador com `--base` | CI |

## Decisões técnicas

- **Validador em Go no `conformance`** porque `BOM-06` cruza `go.work` e
  manifestos que o verificador já lê, e `GOV-35` exige admissão comum a E1 e
  E2/E3. Alternativa descartada: `tools/bom-verify.mjs`, porque duplicaria o
  parser de manifesto em Node.
- **`internal/exception` como unidade `domain` com `now` por valor** porque
  `manifest` (domain) precisa importá-la e `domain` nega `time`; o `cmd` lê o
  relógio. Alternativa descartada: colocar a admissão em `bom` (app), porque
  `manifest` não pode importar `app`.
- **`registry_ref` + `version`** porque `BOM-03` exige `version` e `BOM-06`
  exige referência; o validador é a cola que impede divergência. Registrado no
  ADR-041 como reconciliação das duas regras.
- **`B008` como erro** porque `BOM-07` proíbe transição por decurso de prazo e
  `BOM-08` diz que vencida não é certificação: a única leitura que satisfaz as
  duas é o validador **não aceitar** a certificação vencida até o ato que a
  rebaixa. Alternativa descartada: rebaixar automaticamente, porque é
  transição por omissão.
- **Catálogo N1–N7 por estrutura** porque `GOV-32` nega na admissão sem exame
  de mérito; prefixos reservados de identificador normativo (`P0-`, `ANC-`,
  `T1`..`T6`), bloco da unidade + capability (N1 via ADR-015), forma de aresta
  (N2), pin Buf (N6) e `evidence_` (N7) são decidíveis sem ler
  `justification`. Alternativa descartada: lista de strings proibidas, porque
  texto livre é contornável.
- **`history[]` na própria exceção** porque `GOV-36` exige renovações por
  exceção e o ledger junto do registro é descobrível por quem lê o repositório
  (`GOV-35`). Alternativa descartada: arquivo de ledger separado, porque
  separa a evidência do registro.
- **Extensão compatível do `dmpf/units@1`** porque todos os manifestos têm
  `exceptions: []`. Alternativa descartada: `dmpf/units@2`, porque mudaria 15
  manifestos e o baseline por uma adição opcional.

## Regras relacionadas

- `SPEC-8HWBWJCB` — identidade da release; catálogo N1–N7; `history[]`.
- `docs/guides/dmpf-manifesto.md` — onde a exceção E1 vive.
- ADR-015 — política de capabilities que decide N1 para E1.

## Verificação e testes

### Critérios de aceite

- [x] O BOM está versionado e é revisável por PR; nenhum dos seis itens está
  ausente, e item sem instância aparece declarado vazio (critério 3 do ticket,
  parte do modelo).
- [x] Nenhuma entrada `certificada` existe sem `evidence_uri`, `evidence_digest`,
  `approved_by`, `certified_at` e `valid_until`; entrada vencida reprova até o
  ato que a rebaixa (critério 4, parte do validador).
- [x] Pedido de escape hatch sem plano de convergência **com prazo** (ou sem
  revisão com aprovação dual), ou sobre constraint P0, é recusado na admissão
  (critério 5).
- [x] Um vetor negativo por `B001`..`B011` e por `X001`..`X007`, incluindo um
  por N1–N7.
- [x] `check_e2e_test.go`: E1 admitida autoriza só o import nominal; E1 sem
  `adr` não autoriza e emite `X001`.
- [x] `dmpf-bom` roda no CI e passa sobre `bom/dmpf/0.1.0.json` inicial.
- [x] Unidades `exception`, `bom`, `cmd-bom` no baseline em commit próprio;
  verificador sem diagnóstico; cadeia verde.

### Cenários de teste

**BOM (`bom/validate_test.go`)**

- Dado o BOM inicial, quando `dmpf-bom --now 2026-09-08T00:00:00Z`, então
  exit 0.
- Dado sem `deprecations`, então `B001`; dado `deprecations: {entries: []}`
  sem `reason`, então `B001`.
- Dado `version: ">=5.10 <6"`, então `B002`.
- Dado `certificada` sem `evidence_digest`, então `B004`; sem `certified_at`,
  então `B004`.
- Dado digest divergente, então `B005`.
- Dado `compatible_with` sem entrada na evidência, então `B006`.
- Dado Go `1.26.5` com `registry_ref` para `go.work` (`1.26.6`), então
  `B007`; dado sujeito `runtime` sem `registry_ref`, então `B007`.
- Dado `certificada` com `valid_until` ontem, então `B008` e exit 1; dado o
  mesmo com `state: candidata` e `history` da transição, então exit 0.
- Dado sem `cve`, então `B009`; CVE `aberta` sem `owner`, então `B009`.
- Dado `metrics.vigentes: 0` com uma exceção vigente, então `B010`.
- Dado dois arquivos em `bom/dmpf/` sem `--release`, então `B011`; dado
  `tag: dmpf@0.2.0` em `0.1.0.json`, então `B011`.

**Admissão (`internal/exception/admit_test.go`)**

- Dado E1 completa com `convergence.kind: plan`, então admitida.
- Dado E1 com `convergence.kind: review` e `approved_by: [arquitetura]` só,
  então `X001`.
- Dado sem `convergence.deadline` e sem `review_by`, então `X001`.
- Dado `kind: p0-constraint`, então `X002`.
- Dado E1 em unidade `domain` para `github.com/jackc/pgx/v5` (`io.storage`),
  então `X003` (N1 pela política de bloco); dado `identity: "a->b"`, então
  `X003` (N2).
- Dado `identity: "ANC-09"`, então `X004` (N3); `"T4"`, `X004` (N4);
  `"block"`, `X004` (N5); `"tools/buf.sh"`, `X004` (N6); `"evidence_uri"`,
  `X004` (N7); `"P0-1"`, `X004` (N1).
- Dado `review_by` depois de `valid_until`, então `X005`; `history[0]:
  renewed`, então `X005`.
- Dado `valid_until` ontem sem `renewed`, então `X006`.
- Dado E2 no manifesto, então `X007`.

**Ponta a ponta (`internal/conformance/check_e2e_test.go`)**

- Dado unidade `provider` com E1 admitida para `example.com/sdk`, quando
  `Check`, então o import passa e outro import externo reprova.
- Dado a mesma E1 sem `adr`, então o import reprova e o relatório tem `X001`.

<critical_constraints>
- [P0] NUNCA aceitar entrada `certificada` sem `evidence_uri`, `evidence_digest`, `approved_by`, `certified_at` e `valid_until` (`B004`), nem com `evidence_digest` diferente do SHA-256 do arquivo (`B005`), nem com `compatible_with` sem entrada na evidência (`B006`).
- [P0] NUNCA admitir exceção com `kind` fora de E1–E3 (`X002`) nem cujo objeto caia no catálogo N1–N7 (`X003`..`X004`); a classificação é por estrutura, nunca por texto.
- [P0] NUNCA autorizar import por exceção não admitida: `rule.ExternalPolicy.exceptionFor` só vê entradas que passaram por `exception.Admit`.
- [P0] NUNCA aceitar BOM com um dos seis itens ausente (`B001`); item vazio existe com `reason`.
- [P0] NUNCA deixar `B008` como aviso: certificação vencida reprova até o ato declarado.
- [P1] `registry_ref` obrigatório para sujeitos da tabela de `BOM-06`; `B007` resolve e compara.
- [P1] `metrics` do BOM são derivadas de `history[]`; valor declarado divergente reprova (`B010`).
</critical_constraints>

## Escopo fora

- **Certificar entradas**: sub-spec 4 (exige evidência).
- **Alterar FND-10**: a tensão `BOM-07`/`BOM-08` é resolvida no validador.
- **Renovação automática**: `GOV-34` diz que não existe.
- **Rito de autorização de classificação (`AUT-*`)**: já coberto pelo
  verificador (`T001`/`T002`); esta spec só trata exceções.
- **Exceção por categoria/prefixo**: proibida por `GOV-33`; o schema não a
  representa.
