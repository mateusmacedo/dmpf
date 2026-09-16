---
id: SPEC-XMNBMY50
slug: dmpf-shared-kernel
title: DMPF — Shared kernel, o consumo do kernel por outros bounded contexts
stage: done
priority: P1
depends_on: [SPEC-WTAXFV8B]
ticket_url: null
subtask_urls: []
created: 2026-09-09
---

# SPEC-XMNBMY50: DMPF — Shared kernel, o consumo do kernel por outros bounded contexts

## Resumo

Pela norma vigente, nenhum bounded context fora de `kernel` consegue
consumir o kernel de runtime: a condição C2 da regra de dependência só libera
uma aresta entre contextos quando o destino é `contract` ou declara
`public_integration_surface: true`, e essa declaração é inválida em bloco
`domain` (`DMPF-M002`). Como `application.Outcome[R]` e
`ports.OutboxEntry` expõem tipos de `domain`, importar qualquer bloco
do kernel obriga a importar o seu `domain` — e o verificador emite `DMPF-D002`.
Esta spec introduz a noção de **shared kernel**: um conjunto nominal de
unidades do kernel — inclusive as de `domain` — designado por ato de
classificação como importável por qualquer outro contexto. A designação é por
**unidade**, nunca pelo contexto inteiro: `kernel` também classifica os
agregados de exemplo (`example-orders`, `example-reservations`) e a composition
root de referência (`reference-app`), que continuam privados. A lista vive no
baseline governado, não no manifesto do produtor, para que a proibição de
RFC §7.2 continue fechada aos domínios de negócio.

Como squad que cria um bounded context novo, quero usar `domain`,
`ports`, `application` e os providers do kernel e ser aprovada pelo
verificador sem declarar meu contexto como `kernel`.

## Contexto

- **Problema**: a prova do generator `bounded-context` (SPEC-H1A190Y8) gerou o
  contexto `genproofctx` e o verificador reprovou com dois `DMPF-D002`
  (`genproof-domain/{request,fulfillment} → kernel/domain`). Todos os
  manifestos do workspace declaram `bounded_context: kernel` (exceto o do
  próprio verificador), então a condição C2 nunca havia sido exercitada por um
  consumidor externo. O kernel foi desenhado para ser consumido, mas a norma
  que o classifica o trata como um contexto de negócio qualquer.
- **Impacto**: sem esta spec, o único caminho para um contexto novo é
  declarar-se `kernel` — o que apaga a identidade de limite que o
  ADR-017 existe para tornar verificável — ou reescrever o desfecho da UPR e
  os tipos de porta em cada contexto.
- **Inspiração**: o padrão *Shared Kernel* de DDD (Evans, cap. 14): um
  subconjunto do modelo compartilhado explicitamente entre contextos, com
  mudança governada. Aqui a governança já existe: o rito de classificação
  (`AUT-01`, ADR-028, ADR-031) e o baseline em commit próprio.
- **Links relevantes**:
  - ADR-010 — a função `decide(...)`, C1 e C2, `DMPF-D001`/`D002`
  - ADR-017 — bounded context declarado; superfície pública; `DMPF-M002`
  - ADR-012 — classificação por metadado declarado, nunca inferido
  - ADR-028, ADR-031 — baseline como ato de classificação em commit próprio
  - `docs/guides/dmpf-manifesto.md` — manifesto, baseline e gate
  - `SPEC-WTAXFV8B` — KRN-02, o verificador e o baseline
  - `SPEC-H1A190Y8` — o generator que expôs o bloqueio; primeira consumidora
- **Evidência no código** (verificada em 2026-09-09):
  - `libs/backend/go/conformance/internal/rule/decide.go:35` — C2 =
    `SameBoundedContext || PublicIntegrationSurface(target)`
  - `libs/backend/go/conformance/internal/manifest/validate.go:95` —
    `public_integration_surface: true` em `domain` emite `DMPF-M002`
  - `libs/backend/go/application/outcome.go:14,25,37` —
    `Outcome[R]` carrega `*domain.Rejection`
  - `libs/backend/go/ports/outbox.go:33` — `Event domain.DomainEvent`
  - `libs/backend/go/conformance/internal/baseline/{baseline.go:32-36,digest.go:15-29,authorization.go:12-18}`
    — `Document`/`Digest`/`Entries` e os cinco `Ato`s do T002

<constraints>
- [P0] NUNCA permitir que uma unidade de bloco `domain` se declare pública no próprio manifesto: `DMPF-M002` permanece; a designação de shared kernel é externa ao produtor e passa pelo rito de classificação.
- [P0] NUNCA designar shared kernel no mesmo commit que altera código de unidade: a designação é ato de classificação (`AUT-01`), sujeita a `DMPF-T001`/`T002` como o baseline.
- [P0] A matriz de blocos (C1) não muda: shared kernel relaxa apenas a condição de contexto (C2).
- [P1] A designação nomeia unidades, nunca o bounded context inteiro: agregados de exemplo e composition roots do kernel ficam privados (ADR-017, interior privado por default).
- [P1] Todo diagnóstico novo entra no catálogo do verificador com código, título, referência normativa e aplicabilidade, como os existentes.
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Designação de shared kernel no baseline governado**: o arquivo
  `tools/dmpf-baseline/units-baseline.json` ganha a chave `shared_kernel_units`
  (array de chaves canônicas de unidade, default `[]`), com o valor inicial
  cobrindo a API de runtime do kernel — `kernel/domain`,
  `kernel/port`, `kernel/application`, `kernel/provider-postgres`,
  `kernel/app-consumer`, `kernel/app-relay`, `kernel/observability`,
  `kernel/transport`, as unidades dos providers de transporte e os
  contratos (`contracts/*`, já públicos por construção) — e **excluindo**
  `example-*`, `example-memory` e `reference-app`.
  A chave vive no `Document` do baseline (`internal/baseline/baseline.go`), e o
  `Digest` passa a cobri-la junto com `Entries` (`internal/baseline/digest.go`
  hoje só fecha `Entries`) — alterar a lista sem recalcular reprova em
  `DMPF-T001`. Compatibilidade: a distinção "chave ausente" × "presente e
  vazia" é preservada — documento **sem** a chave é verificado pelo digest
  legado (só `Entries`) e continua válido; documento **com** a chave usa o
  digest novo; o próximo `--write-baseline` materializa a chave e o digest
  novo. Nenhum baseline já commitado deixa de verificar. Só o `--write-baseline` a preserva; nenhuma outra escrita a
  produz. Alterar a lista é ato de classificação: `authorization.go` ganha o
  `Ato` "designar shared kernel" (hoje há cinco `Ato`s fixos, nenhum para um
  campo de nível de documento), e o verificador exige que o commit que a muda
  não toque código de unidade (`DMPF-T002`, mesma regra do baseline).
  - Edge case: chave canônica que não existe em nenhum manifesto →
    `DMPF-M00x` novo ("unidade de shared kernel não existe"), fail-closed.
  - Edge case: unidade listada cujo `block` é `domain` continua válida — é
    exatamente o caso que a designação existe para cobrir; o que `M002` proíbe
    é a autodeclaração no manifesto, não a designação governada.
  - Edge case: chave ausente em baseline antigo → `[]` e digest legado.
- [ ] **[P0] Regra C2 estendida**: `Decide` passa a devolver `C2 = true` também
  quando `target.CanonicalKey ∈ shared_kernel_units`. `Endpoint` ganha o dado
  necessário (ou `Decide` recebe o conjunto) sem alterar C1 nem a política de
  dependências externas.
  - Edge case: source dentro do kernel importando contexto de negócio → C2
    continua exigindo superfície pública (a relação é unidirecional).
  - Edge case: `X/domain → kernel/example-orders` → `D002` (unidade do
    kernel fora da lista); `X/application → kernel/reference-app` →
    `D001` e `D002`.
- [ ] **[P0] `DMPF-M002` inalterado**: `public_integration_surface: true` em
  bloco `domain` continua inválido, inclusive dentro do shared kernel.
- [ ] **[P0] Testes do verificador**: vetores em `internal/rule` e
  `internal/conformance` cobrindo: contexto X → `kernel/domain` aprovado;
  X → `kernel/example-orders` reprovado com `D002`; X → Y/`domain` (Y de
  negócio) reprovado com `D002`; lista com chave inexistente reprovada;
  designação misturada com código → `T002`; baseline sem a chave → digest
  legado aceito; baseline com a chave e digest antigo → `T001`.
- [ ] **[P0] Gate mecânico**: `tools/dmpf-gate-check.sh` (ou script irmão)
  ganha vetores que provam a aprovação de X → `kernel/domain` e a
  reprovação de X → Y/`domain` **e** de X → `kernel/example-orders`, num
  repositório descartável, no mesmo molde dos vetores existentes.
- [ ] **[P1] ADR-042 "Shared kernel"**: registra a decisão, a alternativa
  descartada (flag por unidade) e a relação com ADR-017; ADR-017 ganha a nota
  "estendido por ADR-042" na seção Status.
- [ ] **[P1] Guia do manifesto**: `docs/guides/dmpf-manifesto.md` ganha a seção
  "Shared kernel": o que é, como se designa, o que continua proibido.
- [ ] **[P1] Baseline atual regravado** com `shared_kernel_units` (lista
  nominal acima) em commit próprio de classificação.

### Não-funcionais

- [ ] Compatibilidade: baselines sem a chave continuam válidos; nenhum
  diagnóstico novo aparece nos 15 módulos existentes (todos são `kernel`
  ou `conformance`).
- [ ] Determinismo: o verificador continua idempotente; `--write-baseline`
  produz a mesma saída para a mesma árvore.
- [ ] Sem dependência externa nova no `conformance`.

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | O que muda |
| --- | --- | --- |
| `domain`, `port`, `application`, `provider`, `app` do kernel | [ ] | Nenhum código muda; só a classificação do contexto `kernel` |
| Verificador (`conformance`) | [x] | Regra C2, leitura/escrita do baseline, validação, diagnóstico novo, testes |
| Baseline governado | [x] | `shared_kernel_units` em `tools/dmpf-baseline/units-baseline.json` |
| Documentação normativa | [x] | ADR-042, nota no ADR-017, guia do manifesto |

## Localização de código

```text
libs/backend/go/conformance/
  internal/rule/decide.go              — MODIFICAR: C2 com shared kernels
  internal/rule/diagnostic.go          — MODIFICAR: código novo (shared kernel inexistente)
  internal/manifest/validate.go        — MANTER M002; validar a lista contra os contextos declarados
  internal/baseline/baseline.go        — MODIFICAR: Document ganha SharedKernelUnits; ausente → [] (legado)
  internal/baseline/digest.go          — MODIFICAR: Digest cobre Entries + SharedKernelUnits quando a chave está presente; legado só Entries
  internal/baseline/authorization.go   — MODIFICAR: Ato "designar shared kernel" para o T002
  internal/fsstore/baseline.go         — MODIFICAR: I/O da chave shared_kernel_units (presença preservada)
  internal/conformance/check.go        — MODIFICAR: propagar a lista a Decide; T002 na mudança da lista
  internal/**/*_test.go                — MODIFICAR: vetores acima
tools/dmpf-baseline/units-baseline.json — MODIFICAR: shared_kernel_units (lista nominal; commit próprio)
tools/dmpf-gate-check.sh               — MODIFICAR: vetor de shared kernel
docs/adr/042-shared-kernel.md          — NOVO
docs/adr/017-bounded-context-declarado-superficie-publica.md — MODIFICAR: Status "estendido por ADR-042"
docs/guides/dmpf-manifesto.md          — MODIFICAR: seção "Shared kernel"
```

## Design

### Arquitetura

```text
 units-baseline.json ── shared_kernel_units: [kernel/domain, …] ──► conformance.check ──► rule.Decide(source, target, sharedUnits)
                                                                                   C1: AllowedByMatrix(blocks)        (inalterada)
                                                                                   C2: same || public(target) || target.key ∈ sharedUnits
 manifest.validate ── M002: domain + public_integration_surface → inválido    (inalterada)
 mudança em shared_kernels no mesmo commit que código de unidade ──► DMPF-T002 (rito de classificação)
```

### Fluxo principal

1. A squad gera ou escreve um contexto `X` que importa `domain`,
   `ports` e `application`.
2. O verificador lê `shared_kernel_units` do baseline; as unidades de runtime
   do kernel estão na lista.
3. Para cada aresta `X/* → kernel/<unidade listada>`, C1 decide pelo par
   de blocos e C2 passa; `X/* → kernel/example-orders` reprova em `D002`.
4. Para uma aresta `X/domain → Y/domain` com `Y` de negócio, C2 reprova com
   `DMPF-D002`, como hoje.
5. Alterar a lista exige commit próprio; misturada com código, o verificador
   emite `DMPF-T002`.

## Decisões técnicas

- **Designação no baseline, não no manifesto do produtor** porque o ADR-017
  fecha a burla trivial de RFC §7.2 justamente impedindo que um domínio se
  declare público; uma flag por unidade reabriria essa porta. No baseline, a
  designação é ato de classificação auditável, com a mesma proteção
  `T001`/`T002` do restante. Alternativa descartada: `shared_kernel: true` no
  `dmpf-units.json` do kernel, porque qualquer contexto poderia marcar o seu.
- **Lista de unidades, não de contextos** porque `kernel` classifica
  também `example-orders`, `example-reservations`, `example-memory` e
  `reference-app`: uma designação por contexto tornaria importável tudo o que
  C1 permitir, inclusive agregados de exemplo e a composition root — o oposto
  do "interior privado por default" do ADR-017. A lista nominal cobre a API de
  runtime (`domain`, `port`, `application`, providers, `app-consumer`/`relay`,
  `observability`, `transport`) e deixa o resto privado. Alternativa
  descartada: lista de contextos, pela autoridade excessiva; alternativa
  descartada: reclassificar exemplos em contexto próprio, porque muda a
  identidade de unidades existentes (mudança normativa, ADR-017 "estável")
  só para contornar a granularidade.
- **C1 intocada** porque a matriz de blocos responde a outra pergunta; um
  `domain` de negócio continua não podendo depender de `provider` do kernel.
- **`M002` mantida dentro do kernel** porque a chave que libera o consumo é
  outra; manter a proibição evita duas formas de dizer a mesma coisa.

## Regras relacionadas

- ADR-010, ADR-012, ADR-017, ADR-028, ADR-031 — regra de dependência, classificação declarada, baseline e rito.
- `SPEC-H1A190Y8` e suas sub-specs — dependem desta para que o contexto gerado seja aprovado.

## Verificação e testes

### Critérios de aceite

- [ ] `pnpm nx run conformance:test-race` verde com os vetores novos,
  incluindo: `Digest` muda quando `shared_kernel_units` muda; lista alterada sem
  recalcular → `T001`; `Ato` "designar shared kernel" detectado.
- [ ] Um contexto de teste `X` (fixture) importando `kernel/domain`,
  `port` e `application` é aprovado com `--base`; o mesmo `X` importando
  `Y/domain` ou `kernel/example-orders` reprova com `D002`.
- [ ] `tools/dmpf-baseline/units-baseline.json` contém `shared_kernel_units`
  com a lista nominal em commit próprio; `conformance
  --root . --base <antes>` aprova o workspace.
- [ ] `bash tools/dmpf-gate-check.sh` passa com o vetor novo.
- [ ] ADR-042 criado; ADR-017 anotado; guia atualizado; `pt-reviewer` ✓.
- [ ] Cadeia do workspace verde; nenhum diagnóstico novo nos módulos existentes.

### Cenários de teste

```text
DADO um baseline com shared_kernel_units contendo kernel/domain e um contexto X com bloco domain importando kernel/domain
QUANDO conformance --root . --base HEAD0 roda
ENTÃO nenhum D002 é emitido e a saída é "conforme"

DADO o mesmo baseline e um contexto X importando kernel/example-orders (fora da lista)
QUANDO o verificador roda
ENTÃO D002 nomeia X → kernel/example-orders e a saída é REPROVADO

DADO o mesmo baseline e um contexto X importando Y/domain, com Y fora da lista
QUANDO o verificador roda
ENTÃO D002 nomeia X → Y e a saída é REPROVADO

DADO um commit que altera shared_kernel_units e um arquivo .go de unidade ao mesmo tempo
QUANDO o verificador roda com --base
ENTÃO DMPF-T002 é emitido e a saída é REPROVADO

DADO um baseline commitado antes desta spec, sem a chave shared_kernel_units
QUANDO o verificador roda
ENTÃO a lista é tratada como vazia, o digest legado (só entries) é aceito e o comportamento atual se mantém
```

<critical_constraints>
- [P0] NUNCA permitir que uma unidade de bloco `domain` se declare pública no próprio manifesto: `DMPF-M002` permanece; a designação de shared kernel é externa ao produtor e passa pelo rito de classificação.
- [P0] NUNCA designar shared kernel no mesmo commit que altera código de unidade: a designação é ato de classificação (`AUT-01`), sujeita a `DMPF-T001`/`T002` como o baseline.
- [P0] A matriz de blocos (C1) não muda: shared kernel relaxa apenas a condição de contexto (C2).
- [P1] A designação nomeia unidades, nunca o bounded context inteiro: exemplos e composition roots do kernel ficam privados.
- [P1] Todo diagnóstico novo entra no catálogo do verificador com código, título, referência normativa e aplicabilidade.
</critical_constraints>

## Escopo fora

- **Superfície pública por unidade de negócio**: continua sendo
  `public_integration_surface: true` fora de `domain` (ADR-017); esta spec não
  a altera.
- **Refatorar o kernel para não expor `domain` em `ports`/`application`**:
  seria a alternativa sem mudança normativa, mas quebraria a API de 14 módulos
  e a forma canônica do desfecho da UPR (ADR-032); fica registrada como
  alternativa não escolhida no ADR-042.
- **Versionamento semântico do shared kernel**: como consumidores externos
  passam a depender do kernel, a evolução dele vira tema de release; pertence
  ao BOM (SPEC-8HWBWJCB, sub-spec 4).
