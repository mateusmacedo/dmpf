---
id: SPEC-VJMM2DE5
slug: apps-autocontidos
title: DMPF — Apps autocontidos com contrato e deploy dentro do contexto
stage: done
priority: P1
depends_on: []
ticket_url: null
subtask_urls: []
created: 2026-09-26
---

# SPEC-VJMM2DE5: Apps autocontidos com contrato e deploy dentro do contexto

## Resumo

O material de cada contexto está espalhado por três diretórios compartilhados:
`contracts/` (proto, OpenAPI e fixtures), `libs/backend/go/contracts` (código
gerado e testes golden) e `infra/` (Kubernetes, Compose e provisionamento). Esta
spec leva esse material para dentro do app dono. Cada contexto ganha um módulo Go
de contrato (`apps/backend/<ctx>/contract/`), com Buf, gerado, OpenAPI e fixtures
próprios, e um diretório `deploy/`, com Kubernetes, Compose e um manifesto que
declara o que o app precisa da plataforma. O kernel de contratos fica em
`libs/backend/go/contracts`, e o diretório raiz `contracts/` deixa de existir. Os
arquivos compartilhados de `infra/` passam a ser gerados a partir dos manifestos.
O gate Buf, o generator, o release e as normas acompanham a mudança. A entrega é
feita em seis fases.

## Contexto

### Problema

**1. Contrato fora do contexto.** O `.proto` de cada contexto fica em
`contracts/proto/company/<ctx>/`, a OpenAPI em `contracts/openapi/<ctx>/v1/`, as
fixtures em `contracts/fixtures/<ctx>/`, e o gerado em
`libs/backend/go/contracts/gen/go/company/<ctx>/` (`contracts/buf.gen.yaml:6-16`).
Os testes golden de cada contexto ficam em `libs/backend/go/contracts/golden/`.
Alterar o contrato de um contexto toca quatro diretórios e duas árvores de
ownership.

**2. Consumo entre contextos amarrado ao kernel.** O `bff` importa os `service/v1`
dos três contextos (`apps/backend/bff/app/api/routes.go:13-15`,
`app/rpc/clients.go:17-19`). O `reservations` importa os eventos do `orders` em
produção (`apps/backend/reservations/app/consumer.go:17`, `appkit/harness.go:18`,
`distkit/roles.go:23`). As libs do kernel usam o evento `OrderPlaced` como
payload de exemplo nos testes (`libs/backend/go/app/consumer_test.go:16`,
`postgres/outbox_test.go:15`, `testkit/golden/oracle_test.go:14`,
`contracts/envelope/envelope_test.go:14`). Hoje o kernel depende de um contexto.

**3. Infra por app em arquivos compartilhados.** Só `infra/k8s/base/<app>/` e os
patches por app dos overlays são arquivos inteiros de um app. O restante mistura
trechos por app em arquivos compartilhados: `infra/local/compose/reference.yml`
(serviços e anchors, `:10-64`, `:173-330`), `postgres-init` (`:105-115`),
`redpanda-init` (`:139-166`), `pki.yml:36-39`, `swagger-ui.yml:11,16`,
`.env.example:47-63`, `overlays/dev/job-databases.yaml:56-66`, os
`kustomization.yaml` dos overlays e `overlays/hmg/secrets.example.yaml.tmpl`.
Adicionar um contexto exige editar cerca de oito arquivos compartilhados à mão, e
o generator não emite nada disso (`tools/dmpf-plugin/src/generators/bounded-context`).

**4. Normas que fixam o layout atual.** O ADR-033 põe a fonte em `contracts/` e o
gerado na lib (`033:36-51`). O ADR-046 mantém o contrato de cada contexto em
`libs` e rejeita o gerado no app (`046:44`, `046:62`), porque `bff` e
`reservations` importariam um app e o gerado sairia do módulo que os gates Buf
verificam. O ADR-048 fixa os arquivos permitidos na raiz do contexto (`048:25-37`).
O ADR-045 fixa o nome do projeto no basename e o glob `**/contracts/**` do lint
(`045:23`, `045:48`). O SPEC-F7S5B6KV exige um único módulo Buf (`:91-93`).

### Impacto

- Ownership e revisão ficam por contexto (REP-03 de FND-05): um PR de contrato ou
  de deploy toca só o diretório do app.
- O `nx affected` passa a disparar só o contrato e o app que mudaram.
- Adicionar um contexto deixa de exigir edição manual de arquivos compartilhados de
  infra.
- O kernel deixa de depender de um contexto nos testes.

### Links relevantes

- ADR-030, ADR-033, ADR-044, ADR-045, ADR-046, ADR-047, ADR-048, ADR-053
- `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05: REP-01, REP-02, REP-03, BUF-08)
- SPEC-WYX5GW87, SPEC-F7S5B6KV, SPEC-H1A190Y8, SPEC-ACYKBF9V, SPEC-F308KDSF
- `.claude/rules/dmpf-bounded-context.md`, `docs/guides/dmpf-composicao.md`

<constraints>
- Consumidores de outro contexto importam só o módulo de contrato, nunca o módulo
  do app (ADR-044, `044:15,22`).
- O caminho de cada `.proto` dentro do módulo continua espelhando o pacote (REP-01).
- O gerado nunca é editado à mão, e drift reprova (REP-02).
- Nenhum pacote `.proto` publicado deixa de existir: a proteção de `buf breaking`
  não pode ter janela durante a migração (BUF-08).
- Cada módulo Go é um projeto Nx com `dmpf-units.json` (ADR-030).
- A classificação de unidade (`bounded_context`, `public_integration_surface`)
  de cada contrato é preservada, inclusive `resource-scheduling/contract` do
  `bookings` (ADR-046, `rule/decide.go:28-37`).
</constraints>

## Requisitos

### Funcionais

**Kernel de contratos**
- [ ] **[P0] Kernel autocontido**: Mover `contracts/proto/io/cloudevents` para `libs/backend/go/contracts/proto/`, com `buf.yaml` e `buf.gen.yaml` próprios na lib. O gerado do kernel fica só em `gen/go/io/`.
- [ ] **[P0] Proto de teste do kernel**: Criar um evento de exemplo sob pacote do kernel (ex.: `dmpf.testing.v1`) e trocar por ele todo uso de `company/orders/event/v1` em testes de `libs/backend/go/{app,postgres,testkit,contracts}`.
- [ ] **[P0] Remover a raiz `contracts/`**: Ao fim da migração, o diretório `contracts/` não existe mais.

**Módulo de contrato por contexto**
- [ ] **[P0] Módulo `contract/`**: Criar `apps/backend/{orders,reservations,bookings}/contract/` como módulo Go próprio (`github.com/mateusmacedo/dmpf/apps/backend/<ctx>/contract`), com `proto/company/<ctx>/`, `openapi/v1/openapi.yaml`, `fixtures/{event,projection}/v1/`, `gen/go/` e os testes golden do contexto.
- [ ] **[P0] Buf por contexto**: Cada `contract/` tem `buf.yaml` (módulo `proto`) e `buf.gen.yaml` com `out: gen/go`, `clean: true` e `go_package_prefix` do próprio módulo.
- [ ] **[P0] Projeto Nx do contrato**: Cada `contract/` é projeto Nx `<ctx>-contract`, com tags `type:lib`, `scope:backend`, `stack:go` e `layer:contract`, e targets `buf-lint`, `buf-pins`, `buf-generate-check`, `buf-breaking`, além da cadeia Go.
- [ ] **[P0] Manifesto de unidades**: Mover cada unidade `<bc>/contract` de `libs/backend/go/contracts/dmpf-units.json` para o `dmpf-units.json` do módulo de contrato, preservando `bounded_context` e `public_integration_surface`.
- [ ] **[P0] Reescrever imports**: Atualizar todos os importadores de `gen/go/company/*` (apps, e2e e testes) para o módulo de contrato do contexto dono; `go.work` e `go.mod` sincronizados pelo `dmpf-modsync`.
- [ ] **[P0] Fixtures e OpenAPI**: Atualizar os leitores de fixture e de OpenAPI (`tb.LoadProjection`, testes golden, `bff/app/api/openapi_test.go`, `ContractRef` em `routes.go:35-37`, `bff/project.json:54`, `bff/Dockerfile:30-33`, `infra/k8s/base/bff/configmap.yaml:14-16`) para os caminhos novos.

**Gate Buf**
- [ ] **[P0] Gate parametrizado**: `tools/buf-gate.sh` recebe o diretório do módulo de contrato por argumento, sem caminho fixo, e é chamado pelos targets de cada projeto de contrato e do kernel.
- [ ] **[P0] Pins iguais**: O target `buf-pins` reprova quando a versão da CLI Buf, do `protoc-gen-go` ou do runtime protobuf diverge entre os módulos de contrato e o kernel.
- [ ] **[P0] Identidade por pacote**: O gate identifica cada `.proto` publicado pelo pacote. Quando um módulo novo contém pacotes que existiam em outro módulo na base, `buf breaking` roda contra o recorte desses pacotes na base. Um pacote publicado ausente de todos os módulos em HEAD reprova.
- [ ] **[P0] Baseline por módulo**: A marca de baseline passa a ser `contracts-baseline/<projeto>` (ex.: `contracts-baseline/orders-contract`, `contracts-baseline/contracts`). As regras de tag anotada e de tagger diferente do autor continuam.
- [ ] **[P0] Selftest**: `tools/tests/buf-gate/buf-gate.test.sh` cobre o gate parametrizado, a mudança de lugar por pacote (libera), o pacote desaparecido (reprova) e a divergência de pins entre módulos (reprova).

**Deploy por app**
- [ ] **[P0] Diretório `deploy/`**: Cada app (`bff`, `orders`, `reservations`, `bookings`) ganha `deploy/k8s/base/`, `deploy/k8s/overlays/{dev,hmg}/` e `deploy/compose.yml`, com o conteúdo hoje em `infra/k8s/base/<app>/`, nos patches por app e em `reference.yml`.
- [ ] **[P0] Manifesto `infra.json`**: Cada app declara em `deploy/infra.json` o que precisa da plataforma: banco e role (ADR-053), user, tópicos, DLQ e ACLs do Kafka (ADR-052), certificado de workload, imagem e réplicas por ambiente, e a URL da OpenAPI quando houver.
- [ ] **[P0] Gerador de infra**: Criar `tools/dmpf-conformance/cmd/infrasync` com `--write` e `--check`, que lê os manifestos e gera os arquivos compartilhados: script do `postgres-init`, `job-databases.yaml`, `redpanda-init`, lista de certificados do `pki.yml`, `swagger-ui.yml`, `.env.example`, template de secrets do `hmg` e listas de recursos, imagens e réplicas dos overlays.
- [ ] **[P0] Compose por app**: `infra/local/docker-compose.yml` inclui os `deploy/compose.yml` dos apps; os anchors compartilhados (`x-app`, `x-dmpf-env`, `x-kafka-env`, `x-grpc-server-env`, `x-pki`) viram serviços base num fragmento comum usado por `extends`.
- [ ] **[P0] Overlays compostos**: `infra/k8s/overlays/{dev,hmg}` referenciam `apps/backend/<app>/deploy/k8s/overlays/<env>` como recursos; namespace e observabilidade continuam na plataforma.
- [ ] **[P0] Gate de contexto**: `tools/dmpf-context-check.sh` valida os manifestos `deploy/infra.json` e varre também `apps/backend/*/deploy/`, em vez da lista `for pair in` (`:210-259`).

**Release, generator e normas**
- [ ] **[P1] Release group por contrato**: Criar um grupo por módulo de contrato (`go-contract-<ctx>`) com padrão literal `apps/backend/<ctx>/contract/v{version}`, porque o Nx Release não tem placeholder de diretório (`releaseTag` só por grupo, `nx-schema.json:218-224`), o mesmo mecanismo do `go-tools` (ADR-047); `contracts` segue no `go-libs` só com o kernel; o BOM ganha entradas `subject: contract` e a regra DMPF-B012 cobre essas tags.
- [ ] **[P1] Generator**: O generator `bounded-context` emite o esqueleto de `contract/` e de `deploy/` (incluindo `infra.json`), acrescenta o release group do contrato no `nx.json` e deixa de recusar o bloco de contrato (`generator.ts:115-118`).
- [ ] **[P1] Skill e agente**: `/dmpf-new-context`, `dmpf-context-author` e `.agents/skills/dmpf-bounded-context` passam a ensinar o layout novo, incluindo `infrasync --write` no rito.
- [ ] **[P1] ADR-054**: Registrar a decisão, substituindo o ADR-046 (`:44`, `:62`) e o ADR-033 (`:36-51`) e emendando o ADR-045 (exceção de nome `<app>-contract` e glob de lint), o ADR-048 (`contract/` e `deploy/` na raiz do contexto), o ADR-053 (local da infra por app) e o ADR-047 (release group novo).
- [ ] **[P1] Documentação e rastros**: Atualizar os caminhos em `.claude/rules/dmpf-bounded-context.md`, `docs/guides/dmpf-composicao.md`, ADR-040, ADR-041, ADR-051, `infra/README.md`, READMEs dos apps e libs afetados, `CODEOWNERS`, `.golangci.yml`, `tools/dmpf-harness-check.sh:49-62`, `tools/dmpf-gate-check.sh`, `tools/dmpf-baseline/units-baseline.json`, `dmpf-conformance/bom/registry.go:32`, `.github/actions/setup-go/action.yml:22,37` e o catálogo de evidências do testkit.

### Não-funcionais

- [ ] **[P0] Cache Nx correto**: A named input `go` dos apps exclui `{projectRoot}/deploy/**`; editar manifesto de deploy não invalida `fmt-check`, `vet`, `build` e `test-race`.
- [ ] **[P0] Wire inalterado**: Nenhum pacote, mensagem ou campo `.proto` muda; só o caminho do módulo e o import Go.
- [ ] **[P0] Commit verde por fase**: Cada fase termina com `pnpm nx affected -t lint,typecheck,test,build`, a cadeia Go e os gates DMPF passando.

## Localização de código

| Caminho | Ação |
|---------|------|
| `apps/backend/{orders,reservations,bookings}/contract/` | Criar (módulo de contrato) |
| `apps/backend/{bff,orders,reservations,bookings}/deploy/` | Criar (k8s, compose, `infra.json`) |
| `libs/backend/go/contracts/` | Reduzir ao kernel; receber `proto/`, `buf.yaml`, `buf.gen.yaml` |
| `contracts/` | Remover ao fim |
| `infra/k8s/base/{bff,orders,reservations,bookings}/` | Mover para `deploy/` |
| `infra/k8s/overlays/{dev,hmg}/` | Compor overlays dos apps; listas geradas |
| `infra/local/compose/reference.yml`, `pki.yml`, `swagger-ui.yml`, `.env.example` | Parte gerada, parte movida |
| `tools/buf-gate.sh`, `tools/tests/buf-gate/` | Parametrizar e estender |
| `tools/dmpf-conformance/cmd/infrasync/` | Criar |
| `tools/dmpf-context-check.sh`, `tools/dmpf-harness-check.sh`, `tools/dmpf-gate-check.sh` | Adaptar caminhos e regras |
| `tools/dmpf-plugin/src/generators/bounded-context/` | Emitir `contract/` e `deploy/` |
| `nx.json`, release config, `bom/` | Named input e release group |
| `docs/adr/054-*.md` | Criar |

## Design

### Layout final de um contexto

```text
apps/backend/orders/
├── go.mod, project.json, dmpf-units.json, README.md, Dockerfile
├── domain/ application/ provider/ ports/ app/ appkit/ distkit/ cmd/
├── contract/                      # projeto Nx orders-contract, módulo Go próprio
│   ├── go.mod, project.json, package.json, dmpf-units.json
│   ├── buf.yaml, buf.gen.yaml
│   ├── proto/company/orders/{event,service}/v1/*.proto
│   ├── openapi/v1/openapi.yaml
│   ├── fixtures/{event,projection}/v1/*.golden
│   ├── gen/go/company/orders/...
│   └── golden/                    # testes golden do contexto
└── deploy/
    ├── infra.json                 # necessidades de plataforma
    ├── compose.yml
    └── k8s/{base,overlays/{dev,hmg}}/
```

O `bff` tem só `deploy/`. O módulo do app não contém `contract/`, porque o Go
exclui subdiretórios com `go.mod` próprio. `bff` e `reservations` importam
`apps/backend/<ctx>/contract`, nunca `apps/backend/<ctx>`.

### Fluxo do provisionamento

`deploy/infra.json` (por app) → `infrasync --write` → arquivos gerados em `infra/`
(versionados). O CI roda `infrasync --check`, e o `dmpf-context-check.sh` valida
os manifestos. O que roda em Compose e Kubernetes continua sendo YAML e shell
comuns, revisáveis no diff.

### Gate Buf por pacote

Para cada módulo, o gate lista os pacotes `.proto` em HEAD e na base, em todos os
módulos conhecidos nos dois commits. Pacote presente na base em outro módulo:
`buf breaking` do módulo contra um diretório temporário com os arquivos desse
pacote extraídos da base, mantendo o caminho relativo (REP-01 garante que ele é o
mesmo). Pacote ausente de todos os módulos em HEAD: reprova. Módulo novo sem
pacote preexistente: estado "sem baseline", como hoje.

### Ordem de execução

1. **F1 — Gate e ferramentas**: `buf-gate.sh` parametrizado, identidade por pacote, pins iguais, selftest.
2. **F2 — Kernel**: `proto/` e Buf na lib `contracts`, proto de teste, libs do kernel desacopladas do `orders`.
3. **F3 — Contratos por contexto**: os três `contract/`, imports, fixtures, OpenAPI, unidades, rastros de tooling; remoção de `contracts/`.
4. **F4 — Deploy por app**: `deploy/`, `infra.json`, `infrasync`, compose e overlays compostos, gate de contexto, named input.
5. **F5 — Release**: um grupo por contrato, BOM e DMPF-B012.
6. **F6 — Generator e normas**: generator, skill, agente, ADR-054, emendas e documentação.

## Decisões técnicas

| Decisão | Escolha | Alternativa descartada |
|---------|---------|------------------------|
| Onde fica o contrato | Módulo Go próprio em `apps/backend/<ctx>/contract/` | Tudo no módulo do app (viola ADR-044); só a fonte no app (app parcialmente autocontido) |
| Organização do Buf | Um `buf.yaml`/`buf.gen.yaml` por módulo de contrato | Workspace Buf único com vários módulos (config fora do app, gate dispara para todos) |
| Transição do BUF-08 | Identidade por pacote, `buf breaking` contra o recorte da base | Reset declarado (janela sem `buf breaking`) |
| Infra por app | Tudo, inclusive o provisionamento declarado em `infra.json` | Só k8s/compose do app; só `k8s/base` |
| Consumo dos manifestos | Arquivos gerados e versionados, com `--check` no CI | Leitura em tempo de execução com `yq` |
| Generator | No escopo desta spec | Spec seguinte (contextos novos nasceriam fora da norma) |
| Release dos contratos | Um grupo por contrato com tag literal `apps/backend/<ctx>/contract/v{version}` | Grupo único (Nx sem placeholder de diretório); projeto `<ctx>/contract` com barra (exige spike); sem release (perde consumo externo) |
| Nome do projeto Nx | `<app>-contract`, exceção declarada ao ADR-045 | Diretório `<app>-contract/` (repete o contexto no caminho) |
| Kernel nos testes | Proto de teste próprio (`dmpf.testing.v1`) | Manter `OrderPlaced` (kernel importaria um app) |

## Verificação e testes

### Critérios de aceite

- [ ] `contracts/` não existe, e nenhum arquivo do repositório cita `contracts/proto`, `contracts/openapi` ou `contracts/fixtures` fora de ADRs e specs históricas.
- [ ] Nenhum arquivo em `libs/backend/go/**` importa `apps/backend/**`.
- [ ] Nenhum pacote em `apps/backend/<x>` importa `apps/backend/<y>` com `x ≠ y`, exceto `apps/backend/<y>/contract/**`.
- [ ] `pnpm nx run-many -t buf-lint,buf-pins,buf-generate-check,buf-breaking -p orders-contract,reservations-contract,bookings-contract,contracts` passa contra `NX_BASE` anterior à migração.
- [ ] `go run ./tools/dmpf-conformance/cmd/infrasync --root . --check` passa, e alterar um `infra.json` sem `--write` reprova.
- [ ] `pnpm nx run bff:infra-up` sobe a topologia local, e `pnpm nx run bff:k8s-render` renderiza os overlays `dev` e `hmg` sem erro.
- [ ] `go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop` e `tools/dmpf-context-check.sh` passam.
- [ ] Um contexto gerado pelo `/dmpf-new-context` nasce com `contract/` e `deploy/infra.json` e passa em todos os gates.
- [ ] O release group de cada contrato produz a tag `apps/backend/<ctx>/contract/v<versão>` num dry-run do Nx Release.

### Cenários de teste

1. **Mudança de lugar sem quebra**: com a base antes da migração e HEAD depois, `buf-breaking` de cada contrato roda contra o recorte do pacote e passa.
2. **Mudança incompatível após migrar**: alterar um campo `int64 → int32` em `orders-contract` reprova com `buf breaking (FILE)`.
3. **Pacote desaparecido**: remover `company/reservations/event/v1` de todos os módulos reprova com a regra de pacote publicado.
4. **Pins divergentes**: `protoc-gen-go` diferente num único módulo reprova em `buf-pins`.
5. **Drift de infra**: acrescentar um tópico no `infra.json` do `reservations` sem `infrasync --write` reprova no `--check`.
6. **Cache de deploy**: editar `apps/backend/orders/deploy/k8s/base/deployment-api.yaml` não torna `orders:build` afetado.
7. **Consumo entre contextos**: o `reservations` consome `OrderPlaced` do `orders-contract` no e2e com Kafka, e o `bff` atende as rotas dos três contextos no e2e.
8. **Kernel isolado**: `pnpm nx run-many -t test -p app,postgres,testkit,contracts` passa sem nenhum módulo de `apps/` no grafo dessas libs.

<critical_constraints>
- Consumidores de outro contexto importam só `apps/backend/<ctx>/contract`, nunca o app.
- Nenhum pacote `.proto` publicado some, e `buf breaking` vale durante toda a migração.
- O wire não muda: pacotes, mensagens e campos `.proto` ficam idênticos.
- O kernel (`libs/backend/go/**`) não importa nada de `apps/backend/**`.
- Cada fase termina com commit verde (lint, testes, cadeia Go e gates DMPF).
</critical_constraints>

## Escopo fora

- Alterar qualquer mensagem, campo ou pacote `.proto`, ou o conteúdo das OpenAPIs.
- Mover a stack de plataforma (observabilidade, Postgres, Redpanda, Redis, PKI base) para fora de `infra/`.
- Ruleset de tag no GitHub para `contracts-baseline/*`.
- CD real (o `cd-dev-hmg.yml` segue template desligado).
- Criar contrato para o `bff`, que só consome.
- Contextos novos além dos quatro apps existentes.
