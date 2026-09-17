---
id: SPEC-YRJRADY9
slug: kernel-sdk-go
title: DMPF — Kernel e SDK de Referência Go
stage: done
priority: P0
depends_on: [SPEC-QG2N8STY]
ticket_url: null
subtask_urls: [ARQ-520, ARQ-521, ARQ-522, ARQ-523, ARQ-524, ARQ-525, ARQ-526, ARQ-527, ARQ-528, ARQ-529, ARQ-530, ARQ-531]
created: 2026-08-30
---

# SPEC-YRJRADY9: DMPF — Kernel e SDK de Referência Go

## Resumo

Implementar em Go, dentro deste monorepo Nx, o kernel e o SDK de referência do
**Domain Message Processing Framework (DMPF)**, realizando em código executável a
fundação normativa que o épico ARQ-436
entregou como RFC e ADRs. Como time de plataforma, queremos um kernel Go
idiomático e verificável para que as squads construam serviços orientados a
domínio e mensagens sobre limites estáveis, em vez de reinventar transação,
mensageria, contrato e observabilidade a cada serviço.

Esta é uma **spec guarda-chuva**: ela fixa o escopo do épico, as invariantes que
nenhuma sub-spec pode relaxar e a decomposição em 12 sub-specs `KRN-01`..`KRN-12`.
A especificação detalhada de cada eixo pertence à sua sub-spec.

## Contexto

- **Problema**: a fundação DMPF é hoje inteiramente documental. Dezenove ADRs, uma
  RFC e nove artefatos normativos descrevem um golden path que **nenhuma linha de
  código realiza**. Sem kernel, cada squad continua resolvendo UPR, transação,
  outbox, contrato e telemetria por conta própria, e a distância entre a norma e o
  parque permanece do tamanho medido no inventário.
- **Impacto**: uma squad passa a compor um serviço a partir de blocos prontos e
  verificados, com a regra de dependência checada mecanicamente no CI em vez de
  por revisão manual. A conformidade deixa de ser promessa de documento e vira
  gate de pipeline.
- **A fundação deixou este espaço vazio de propósito.** A constraint **P0-4** da
  RFC declara: *"Kernels Go e TypeScript, adapters e providers de produção não são
  entregues nem especificados por este épico. Ele entrega fundação normativa"*
  (`docs/dmpf/rfc-dmpf-foundation-v0.1.md` §2.3). Esta spec preenche exatamente
  essa lacuna, e o faz sem reabrir nenhuma decisão da fundação.
- **Épico**: subsequente a ARQ-436, ordem 1 da tabela "Épicos subsequentes" —
  *"DMPF — Kernel e SDK de Referência Go: implementar a Parte 2 de forma
  idiomática e aderente à fundação em Monorepo NX utilizando Golang nas
  implementações"*. A issue Jira ainda não existe e será criada na Etapa 2 do
  pipeline.
- **Terreno**: este repositório não tem código Go. O `go.work` declara apenas
  `go 1.25`, `apps/` e `libs/` são diretórios de destino sem projeto Nx
  registrado. O `@nx-go/nx-go` 4.0.0 já está instalado e registrado em
  `nx.json`, com os generators `application`, `library`, `init`, `preset`,
  `convert-to-one-mod` e `convert-to-inferred`, e os executors `build`, `lint`,
  `test`, `tidy`, `serve`, `serve-air` e `generate`. O toolchain local é
  `go1.26.4`.

### Fontes normativas

Esta spec **consome** a fundação; não a reabre. Toda regra abaixo tem dono
declarado no acervo:

| Eixo | Fonte |
|------|-------|
| Unidade arquitetural e binding Go | RFC §3.1–§3.6; `docs/adr/011-verification-unit-binding-por-stack.md` |
| Seis blocos, classificador, regra de dependência | RFC §4, §7; `docs/adr/010-regra-de-dependencia-e-seis-blocos.md` |
| Manifesto e classificação declarada | RFC §10.1; `docs/adr/012-classificacao-por-metadado-declarado.md` |
| Trust model e autorização de reclassificação | RFC §10.2; `docs/adr/013-*.md`, `docs/adr/028-*.md` |
| Aresta `domain → port` proibida | RFC §5.2; `docs/adr/014-proibir-aresta-domain-port.md` |
| Capabilities externas e allowlist | RFC §6; `docs/adr/015-capabilities-externas-por-bloco.md` |
| Domínio executável em memória | RFC §9; `docs/adr/016-dominio-executavel-em-memoria.md` |
| Bounded context e superfície pública | RFC §5.4–§5.5; `docs/adr/017-*.md` |
| Forma do desfecho da UPR | `docs/adr/018-forma-do-desfecho-da-upr.md`; `docs/dmpf/upr-decision-mensagens.md` |
| Três níveis de contrato e conversão de evento | `docs/adr/019-*.md` |
| Relay, outbox e inbox | `docs/adr/020-*.md`, `docs/adr/021-*.md`; `docs/dmpf/uow-inbox-outbox.md` |
| CloudEvents, `payload_hash` e Buf | `docs/adr/022-*.md`, `docs/adr/023-*.md`; `docs/dmpf/cloudevents-protobuf-buf.md` |
| Transportes e governo do tempo | `docs/adr/024-*.md`, `docs/adr/025-*.md`; `docs/dmpf/politicas-transporte.md` |
| Resiliência e observabilidade | `docs/adr/026-*.md`; `docs/dmpf/resiliencia-observabilidade.md` |
| BOM, compatibilidade e escape hatch | `docs/adr/027-*.md`; `docs/dmpf/governanca-bom-pilotos.md` |
| Testes e interoperabilidade | `docs/dmpf/testes-interop.md` |
| Contexto, erros e segurança | `docs/dmpf/contexto-erros-seguranca.md` |

### Referências internas

- **`legado-golibs`** (`github.com/mateusmacedo/golibs`, commit `9106941`) —
  monorepo Nx + `go.work` com 15 pacotes Go, `@nx-go/nx-go` 4.0.0, Go 1.25,
  release por Nx independent versioning com tag `packages/<nome>/v*`. É o
  **precedente de forma** para o monorepo Go desta organização e a fonte de
  referências idiomáticas (`goresilience`, `gotelemetry`, `godata`). Inventário
  em `docs/specs/a análise AS-IS de sistemas legados/inventario-as-is-golibs.md`.
- **Gap medido do `legado-golibs` contra o DMPF**: não existe outbox, inbox nem relay;
  o wire é JSON ad hoc sem `.proto`, Buf ou CloudEvents; a UoW de `godata` não
  amarra a publicação ao commit; não há `Decision`, UPR, manifesto nem
  verificador. Duas violações estão nomeadas pela própria fundação:
  `goservice/domain` declara `EventPublisher` — o caso que o ADR-014 cita
  textualmente como unidade mal dimensionada — e `goweb/domain/http_request.go:6`
  importa `net/http`, violando P0-1.
- **Decisão de destino**: o kernel nasce **neste repositório**, em terreno limpo.
  O `legado-golibs` é referência de forma e fonte de padrões idiomáticos, não base de
  código a herdar — herdar traria junto a dívida que a fundação nomeia.

<constraints>
- [P0] Domínio sem I/O: nenhuma `verification_unit` de bloco `domain` importa Protobuf, ORM, broker, SDK de cloud, HTTP, logger, tracing, cache ou framework de execução
- [P0] Protobuf apenas no wire: tipo gerado de Protobuf NUNCA é estado de domínio, entrada de UPR ou evento de domínio
- [P0] At-least-once com efeitos idempotentes: nenhum componente, documento, configuração ou contrato deste épico promete exactly-once fim a fim
- [P0] A aresta `domain → port` é PROIBIDA, sem condicional e sem exceção
- [P0] Fail-closed: lacuna de configuração, classificação ausente, manifesto ausente e caso degenerado REPROVAM; nunca são lidos como conformidade
- [P0] Esta spec NÃO reabre decisão da fundação: divergência encontrada vira ADR novo ou escape hatch declarado, nunca alteração silenciosa da norma
- [P1] Paridade conceitual com o kernel TypeScript é por contrato e garantia, NUNCA por sintaxe ou mecanismo de concorrência: o kernel Go é idiomático em Go
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Fundação Nx-Go** (`KRN-01`): workspace Go operacional no monorepo —
  módulos declarados em `go.work`, tags 3D com `stack:go`, cadeia de validação e
  release integrados ao Nx.
  - A cadeia de validação Go é mais ampla que o trio TypeScript: `gofmt`,
    `go vet`, `golangci-lint`, `go build`, `go test`, `go test -race` e
    `govulncheck`.
  - Edge case: `go.work` declara `go 1.25` e o toolchain local é `go1.26.4` — a
    divergência é resolvida e registrada, não ignorada.
- [x] **[P0] Verificador de conformidade** (`KRN-02`): CLI Go que decide a regra
  de dependência sobre o grafo real de imports.
  - Lê o manifesto `dmpf/units@1`; `verification_unit` é o **package**;
    `canonical_key` é o **import path completo**.
  - Implementa `decide(source_block, target_block, source_bc, target_bc, target_surface)`
    como conjunção de C1 (matriz 6×6) e C2 (contexto).
  - Emite diagnósticos estáveis: `DMPF-D001` (C1 falha), `DMPF-D002` (C2 falha),
    `DMPF-M002` (`public_integration_surface` em unidade `domain`), `DMPF-T001`
    (divergência de baseline), `DMPF-T002` (reclassificação sem autorização).
  - Resolve a aresta sobre o **arquivo real resolvido**, nunca sobre o texto do
    import.
  - Edge case: import que não resolve para o universo é dependência externa e
    segue a política de capabilities, não a matriz.
- [x] **[P0] Kernel de domínio** (`KRN-03`): UPR síncrona, determinística e sem
  I/O, com desfecho na forma `Decision = Accepted(resposta, eventos) | Rejected(rejeição)`.
  - A `Rejection` carrega código estável no formato `contexto/motivo` (por
    exemplo `orders/empty-order`) e mensagem endereçada ao domínio.
  - A `Rejection` **não** carrega código HTTP ou gRPC, política de retry nem
    causa técnica encadeada.
  - Após `Rejected`, o estado observável do alvo é idêntico ao anterior à chamada
    e o ramo de recusa não carrega eventos de domínio.
  - O par `(decisão, erro)` idiomático de Go é admitido como realização conforme,
    desde que o canal de erro transporte **apenas** rejeições de domínio.
- [x] **[P0] Kernel de aplicação** (`KRN-04`): Unit of Work explícita no
  application service, com as portas vinculadas à transação.
  - Realiza a sequência canônica de escrita de nove passos, incluindo optimistic
    locking na persistência do agregado.
  - A UoW **não** repete automaticamente o callback transacional; retry de
    serialização ou deadlock é política explícita.
- [x] **[P0] Contratos wire** (`KRN-05`): `io.cloudevents.v1.CloudEvent` do
  formato Protobuf oficial, com perfil organizacional que apenas acrescenta
  obrigatoriedade.
  - Modalidade única `proto_data` com payload em `google.protobuf.Any`;
    `datacontenttype` é `application/protobuf` e `dataschema` é o *type URL* do
    `Any`.
  - `payload_hash` é SHA-256 em hexadecimal minúsculo sobre os bytes de
    `Any.value` **exatamente como transportados**, sem desserializar e sem
    reserializar; nenhum atributo do envelope entra no cálculo.
  - Gates Buf no CI: lint `STANDARD`, breaking `FILE` e geração determinística,
    todos fail-closed.
- [x] **[P0] Outbox** (`KRN-06`): porta que recebe `(domain event, intenção de
  publicação)` e provider que mapeia e serializa **dentro da transação**.
  - A porta expõe tipo de domínio, nunca tipo de wire.
  - O estado de negócio e o registro da outbox são gravados na mesma transação.
- [x] **[P0] Inbox e consumo** (`KRN-07`): deduplicação, efeitos locais e outbox
  derivada na mesma transação de consumo; ACK sempre depois do commit local.
  - `payload_hash` distingue redelivery legítima de reutilização indevida do
    identificador.
  - Disposições de consumo, poison message, DLQ e quarantine implementadas.
- [x] **[P0] Relay** (`KRN-08`): claim por lease com prazo, publicação fora de
  qualquer transação de banco, transição final condicional ao claim corrente.
  - `locked_by` identifica a **execução do claim**, não o processo.
  - Capacidades obrigatórias: paginação, batch configurável, lease com expiração,
    retry com backoff e jitter, limite de concorrência, exposição dos sinais
    `pending`, `lag`, `attempts` e `failures`, e graceful shutdown.
  - `FOR UPDATE SKIP LOCKED` é admitido **somente durante o claim**.
- [x] **[P1] Resiliência e observabilidade** (`KRN-09`): OpenTelemetry com a
  versão das *semantic conventions* fixada no BOM e propagador W3C Trace Context
  configurado explicitamente.
  - Retry autorizado se, e somente se, quatro fatores forem simultaneamente
    verdadeiros, verificados antes de cada tentativa: erro retentável, operação
    idempotente ou efeito conhecidamente ausente, orçamento suficiente e prazo
    remanescente suficiente.
  - Orçamento de repetição é por execução, com default de metade do prazo
    remanescente na primeira falha; a espera de backoff consome orçamento.
  - Sujeito restrito a `app`, `provider` e `application service` — nenhuma regra
    obriga `domain` ou `port`.
- [x] **[P1] Providers de transporte** (`KRN-10`): gRPC no síncrono interno e
  REST na borda externa.
  - Toda chamada gRPC de saída carrega deadline; o deadline é propagado e
    **nunca reiniciado**; no fio trafega a duração restante e o receptor
    reconstrói o instante.
  - O provider HTTP não retenta método sem semântica idempotente.
  - Kafka é o transporte-alvo do assíncrono de domínio; SNS/SQS permanece
    normatizado.
- [x] **[P1] Testes, test kits e interoperabilidade** (`KRN-11`), *na parte Go*:
  pirâmide de testes, test kit de conformidade de provider e golden fixtures.
  Entregue em `libs/backend/go/testkit` (`SPEC-SJ66880S`, `done`).
  - [x] Cada classe normativa tem ao menos um vetor **positivo** e um
    **negativo** — as 36 células por `Cell.Verify()` e os vetores `V13`–`V32`
    registrados em `fitness/vectors.go`, com `TestVectorsCoverV13ToV32Once`
    garantindo a cobertura sem repetição.
  - [ ] *(movido para o épico TypeScript)* Vetores pareados com o kernel
    TypeScript. O lado Go já registra a assimetria de `V27`
    (`TestV27IsRegisteredAsTypeScriptOnly`) e publica as fixtures; o pareamento
    só existe quando o outro lado nascer.
- [x] **[P2] SDK de referência, generator e BOM** (`KRN-12`): composition root de
  exemplo, generator Nx que scaffolda um bounded context conforme e BOM com
  combinação certificada.
  - Cada entrada certificada do BOM carrega `evidence_uri`, `evidence_digest`,
    aprovador e validade declarada.

### Não-funcionais

- [x] **Verificabilidade** *(atendida com ressalva registrada)*: as cláusulas
  `import-verifiable` da RFC §11 têm vetor executável no CI; as
  `runtime-testable` têm teste de integração; as `structurally reviewable` têm
  item de checklist. O mapa formal é FND-09 §13
  (`docs/dmpf/testes-interop.md:2699-2708`): **650 das 654 regras** têm mecanismo
  de prova. As quatro sem mecanismo — `THR-01`, `THR-02`, `TRP-31` (donas:
  FND-08 §12.1) e `THR-03` (dona: FND-07 §14, gate de revisão de Segurança sem
  owner nomeado) — são **fronteira declarada com dona nomeada**, não vazio
  silencioso. O critério original pedia 100%; a diferença é assumida aqui porque
  as quatro pertencem a outras fundações e não ao kernel Go.
- [x] **Domínio puro**: o fechamento transitivo de imports de toda unidade
  `domain` contém apenas capability `pure`, e executar seus testes não inicia
  processo, não abre socket, não toca disco e não depende de horário. Provado por
  `TestDomainTestsNeedNoInfrastructureDouble` e `TestDomainTestImportingAPortIsNamed`
  (`libs/backend/go/testkit/fitness`).
- [x] **Compatibilidade**: `@nx-go/nx-go` como plugin do workspace e módulos sob
  `github.com/mateusmacedo`. O piso Go declarado no `go.work` é **1.26.6** — o
  critério original dizia 1.25, redigido antes das subidas de toolchain, e foi
  atualizado aqui para o valor real. O `GOPRIVATE` que o critério citava deixa de
  ser requisito: o repositório é público, e sua remoção pertence ao `KRN-14`
  (`SPEC-95AHV4D4`).
- [ ] **Interoperabilidade** *(movido para o épico TypeScript)*: um contrato
  serializado em Go é desserializado em TypeScript sem perda semântica, com
  `payload_hash` idêntico nas duas stacks. **Inatingível nesta spec** — depende
  do kernel TypeScript, que o próprio "Escopo fora" declara ser o épico
  subsequente de ordem 2. A stack Go entrega os fixtures que o outro lado
  consome; a assimetria de `V27` está registrada em
  `TestV27IsRegisteredAsTypeScriptOnly`.
- [ ] **Segurança** *(parcial; a lacuna virou spec própria)*: redaction e TLS
  estão cumpridos — o `Redactor` é fail-closed por allowlist e aplicado na
  origem, por processo (`libs/backend/go/observability/redact/redact.go:37-63`),
  e o `validateTLS` recusa transporte sem TLS ou abaixo de 1.2 salvo opt-out
  explícito do operador (`libs/backend/go/grpc/config.go:117-125`). **Identidade
  e tenant após autenticação não foram realizados**: o BFF usa `Tenant = "public"`
  constante e um `tenantOf` que ignora a requisição
  (`apps/backend/bff/api/routes.go:21,119`), e o gancho de autorização do kernel
  é `AllowAll` (`libs/backend/go/application/authorize.go:12-16`), declarado como
  escolha explícita de um kernel que ainda não tem autorização. A realização é a
  `SPEC-9B6SHEH8`.
- [x] **Determinismo de build**: a geração de código a partir dos `.proto` é
  reprodutível — mesma entrada e mesmo pin de ferramenta produzem bytes iguais.

## Camadas afetadas

Esta spec é **de implementação**: as camadas abaixo são módulos de código a criar
neste repositório, não recortes normativos.

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Domínio (UPR, `Decision`, eventos) | [x] | Bloco `domain`: tipos e contratos puros, sem I/O |
| Application service (UoW, orquestração) | [x] | Bloco `application`: sequência canônica de escrita e consumo |
| Port (abstrações de fronteira) | [x] | Bloco `port`: capacidades de I/O requeridas, expondo tipo de domínio |
| Provider (drivers, SDKs, brokers) | [x] | Bloco `provider`: Postgres, Kafka, SQS, gRPC, HTTP, OTel |
| Contract package (wire) | [x] | Bloco `contract`: `.proto`, CloudEvents, tipos gerados |
| App (composition root, workers) | [x] | Bloco `app`: relay, consumers, wiring de exemplo |
| Tooling do workspace | [x] | Verificador de conformidade, generator Nx, gates de CI |
| Frontend | [ ] | Fora do escopo — o épico é backend Go |

## Localização de código

Estrutura pretendida. Os caminhos exatos de cada módulo são fixados pela sua
sub-spec; `KRN-01` fixa a convenção antes de qualquer outro módulo nascer.

```text
dmpf/
├── go.work                              — passa a declarar os módulos DMPF
├── libs/backend/go/                     — nível de stack; o kernel TS entra em libs/backend/ts/
│   ├── domain/                     — bloco domain: UPR, Decision, Rejection
│   ├── application/                — bloco application: UoW, sequência canônica
│   ├── ports/                      — bloco port: outbox, inbox, repositório, relógio
│   ├── postgres/          — bloco provider: outbox, inbox, optimistic locking
│   ├── kafka/             — bloco provider: publicação e consumo
│   ├── sqs/               — bloco provider: acervo normatizado
│   ├── grpc/              — bloco provider: governo do tempo
│   ├── http/              — bloco provider: borda externa
│   ├── observability/              — bloco provider: OTel, resiliência
│   └── contracts/                  — bloco contract: tipos gerados de Protobuf (KRN-05 fixou scope:backend)
├── libs/shared/go/                      — nível de stack, mesma convenção
│   └── testkit/                    — test kits de conformidade e fixtures
├── apps/backend/
│   └── reference/                  — bloco app: composition root e relay de exemplo
├── contracts/                           — .proto, buf.yaml, buf.gen.yaml
└── tools/
    └── dmpf-verify/                     — verificador de conformidade (CLI Go)
```

> **Convenção de path fixada por `KRN-01`**: módulos ficam sob
> `libs/<scope>/<stack>/<módulo>`, e o nome do projeto Nx leva o sufixo da stack
> (`domain`). Motivo: o épico de ordem 2 do ARQ-436 entrega o kernel
> TypeScript com os mesmos nomes conceituais, e no Nx o nome de projeto é chave
> única. Como a `canonical_key` é o import path e a RFC §5.4 a exige estável,
> a convenção é fixada antes do segundo módulo nascer. Detalhes e alternativas
> descartadas em [SPEC-MQA5HAXF](./SPEC-MQA5HAXF-dmpf-fundacao-nx-go.md).

**Arquivos a modificar**:

- `go.work` — passa a declarar cada módulo Go criado; hoje contém apenas `go 1.25`
- `nx.json` — `targetDefaults` para os targets Go, sem redeclarar o que o plugin
  `@nx-go/nx-go` já infere
- `pnpm-workspace.yaml` — `catalog:` com as versões pinadas do toolchain Go, se
  houver dependência Node envolvida na geração
- `.github/workflows/ci.yml` — acrescenta os gates Go e o gate do verificador de
  conformidade
- `AGENTS.md` — inventário de apps e libs deixa de ser "nenhuma"; a seção de
  comandos ganha a cadeia Go
- `docs/adr/` — ADRs novos desta parte, a partir de `029`

## Design

### Arquitetura

A regra de dependência é a mesma da fundação; o que esta spec acrescenta é a
realização em packages Go, com o import path como `canonical_key`.

```text
app  ──▶  application service  ──▶  domain
 │               │                    ▲
 │               ▼                    │
 └──────▶  port (abstração)  ─────────┘
                 ▲
                 │ implementa
           provider (driver, SDK, broker)

contract (wire) ──▶ usado por provider e app; NUNCA pelo domain
```

Matriz de blocos (condição C1) — linha é origem, coluna é destino:

| De ↓ / Para → | domain | application | app | port | provider | contract |
|---------------|:------:|:-----------:|:---:|:----:|:--------:|:--------:|
| **domain**      | P | ✗ | ✗ | ✗ | ✗ | ✗ |
| **application** | P | P | ✗ | P | ✗ | ✗ |
| **app**         | P | P | P | P | P | P |
| **port**        | P | ✗ | ✗ | P | ✗ | ✗ |
| **provider**    | P | ✗ | ✗ | P | P | P |
| **contract**    | ✗ | ✗ | ✗ | ✗ | ✗ | P |

Toda aresta `P` continua sujeita a C2 (mesmo bounded context **ou** superfície
pública de integração no destino) e à política de capabilities externas.

### Fluxo principal — sequência canônica de escrita

Executada pelo bloco `application service`, dentro de uma única transação:

1. valida autorização de aplicação
2. resolve tempo e identificadores (`message_id`, `occurred_at`)
3. abre a UoW, recebendo as portas vinculadas à transação
4. carrega ou cria o agregado
5. executa a UPR e recebe a `Decision`
6. persiste o agregado com optimistic locking
7. entrega `(domain event, intenção)` à porta da outbox — o provider mapeia para
   integration event e serializa
8. commit
9. devolve a response de aplicação

Os passos 6 e 7 estão na **mesma transação** do passo 8. É isso que torna a
publicação atômica com o estado de negócio.

### Fluxo de drenagem

Executado pelo bloco `app`, assíncrono e fora de qualquer transação de banco
durante o I/O:

1. relay faz claim por lease, em transação curta
2. publica no broker, fora de qualquer transação de banco
3. marca o registro, condicionalmente ao claim ainda ser seu

Se o relay falhar após publicar e antes de marcar, a mensagem é republicada. É
daí que vem o at-least-once — e é por isso que o efeito precisa ser idempotente.

### Pseudocódigo — a decisão do verificador

```text
para cada aresta (arquivo_origem -> import) do universo:
    unidade_origem  = unidade_que_contem(arquivo_origem)      // senão: reprova
    arquivo_destino = resolve_import(import)                  // arquivo real, não texto
    se arquivo_destino nao pertence ao universo:
        avalia_capability_externa(unidade_origem, import)     // politica do bloco
        continua
    unidade_destino = unidade_que_contem(arquivo_destino)     // senão: reprova

    C1 = matriz[unidade_origem.block][unidade_destino.block] == PERMITIDA
    C2 = unidade_origem.bounded_context == unidade_destino.bounded_context
         ou unidade_destino.public_integration_surface

    se nao C1: emite DMPF-D001
    se nao C2: emite DMPF-D002
```

O default de toda ramificação ausente é **reprovar**. Um verificador que não
consiga avaliar uma condição reporta "não verificado", nunca "conforme".

### Decomposição em sub-specs

Cada linha vira uma sub-spec própria e uma issue no Jira. O tamanho é indicativo
de complexidade relativa, não estimativa — a estimativa é do time no refinamento.

| ID lógico | Título | Depende de | Tamanho |
|-----------|--------|------------|---------|
| `KRN-01` | Fundação Nx-Go: workspace, módulos, tags e cadeia de validação | — | M |
| `KRN-02` | Verificador de conformidade DMPF em Go | `KRN-01` | L |
| `KRN-03` | Kernel de domínio: UPR, `Decision` e `Rejection` | `KRN-01` | M |
| `KRN-04` | Kernel de aplicação: Unit of Work e sequência canônica | `KRN-03` | L |
| `KRN-05` | Contratos wire: CloudEvents, Buf e `payload_hash` | `KRN-01` | L |
| `KRN-06` | Outbox: porta, provider e mapeamento na escrita | `KRN-04`, `KRN-05` | L |
| `KRN-07` | Inbox e consumo: deduplicação, efeitos e disposições | `KRN-04`, `KRN-05` | L |
| `KRN-08` | Relay: claim por lease, publicação e sinais | `KRN-06` | M |
| `KRN-09` | Resiliência e observabilidade: OTel e retry por conjunção | `KRN-04` | M |
| `KRN-10` | Providers de transporte: gRPC, REST, Kafka e SQS | `KRN-05`, `KRN-09` | L |
| `KRN-11` | Testes, test kits e interoperabilidade Go ↔ TypeScript | `KRN-02`..`KRN-10` | L |
| `KRN-12` | SDK de referência, generator Nx e BOM certificado | todos | M |

### Sequenciamento sugerido

| Incremento | Foco | Itens | Resultado verificável |
|------------|------|-------|-----------------------|
| **1 — Terreno** | Workspace e conformidade | `KRN-01`, `KRN-02` | Módulo Go compila, valida e é verificado pela regra de dependência no CI |
| **2 — Núcleo** | Domínio e aplicação | `KRN-03`, `KRN-04` | Caso de uso completo em memória, com UPR pura e UoW explícita |
| **3 — Garantias** | Wire e mensageria | `KRN-05`, `KRN-06`, `KRN-07`, `KRN-08` | Mensagem publicada atomicamente e consumida com deduplicação |
| **4 — Fronteiras** | Operação e transporte | `KRN-09`, `KRN-10` | Serviço observável, resiliente e falando gRPC/Kafka |
| **5 — Entrega** | Validação e SDK | `KRN-11`, `KRN-12` | Fixtures cruzando Go ↔ TS e generator scaffoldando contexto conforme |

`KRN-01` e `KRN-02` precedem tudo: sem terreno e sem verificador, cada módulo
seguinte nasceria sem gate e a conformidade voltaria a depender de revisão
manual.

## Decisões técnicas

- **O kernel nasce neste repositório, não no `legado-golibs`**, porque o `legado-golibs`
  carrega duas violações que a própria fundação nomeia (`EventPublisher` em
  `goservice/domain`, `net/http` em `goweb/domain`) e um wire JSON ad hoc
  incompatível com o perfil CloudEvents. Alternativa descartada: estender o
  `legado-golibs` — traria a dívida junto e obrigaria a conviver com escape hatch desde
  o primeiro commit. O `legado-golibs` permanece como referência de forma do monorepo Go
  e fonte de padrões idiomáticos.
- **Um módulo Go (`go.mod`) por lib Nx**, seguindo o precedente do `legado-golibs` (15
  módulos sob `go.work`), porque o `ownership_module` da RFC é o diretório com
  `go.mod` e é ele que carrega o `metadata_container` e o versionamento
  independente do Nx Release. Alternativa a avaliar em `KRN-01`: módulo único com
  múltiplos packages, via `@nx-go/nx-go:convert-to-one-mod` — reduz cerimônia de
  manifesto, mas colapsa o versionamento independente.
- **O verificador é escrito em Go e mora em `tools/`**, porque precisa do grafo
  real de imports que `go list -deps -json` produz, e porque `tools/` é o destino
  declarado de ferramentas do workspace. Alternativa descartada: `golangci-lint`
  com linter customizado — o `depguard` decide por prefixo de path, e a RFC
  proíbe classificação por nome de diretório.
- **`Decision` como par `(decisão, erro)` idiomático de Go**, autorizado
  explicitamente pelo ADR-018: como a UPR não executa I/O, nenhuma falha técnica
  se origina nela, então o canal de erro transporta apenas rejeições de domínio e
  o chamador as distingue por tipo. Alternativa descartada: struct com campo
  discriminante — menos idiomática em Go e não melhora a exaustividade.
- **A divergência de versão do Go é resolvida em `KRN-01`**: `go.work` declara
  `go 1.25`, o `legado-golibs` usa `1.25.0` e o toolchain local é `go1.26.4`. A escolha
  do piso é decisão de BOM, não preferência local.
- **Toda decisão estrutural nova vira ADR a partir de `029`**, continuando a
  numeração do acervo. Divergência com a fundação vira ADR ou escape hatch
  declarado, com owner e plano de convergência com prazo — nunca alteração
  silenciosa.

## Regras relacionadas

- `AGENTS.md` — tags 3D obrigatórias (`type:`, `scope:`, `stack:`), convenção de
  não redeclarar targets que o plugin já fornece, Conventional Commits em PT-BR
- `.claude/rules/process-enforcement.md` — cadeia de validação Go: `gofmt`,
  `go vet`, `golangci-lint`, `go build`, `go test`, `go test -race`,
  `govulncheck`
- `.claude/rules/git-safety.md` — branches protegidas e revisão antes do commit
- `docs/nx-reference/tasks.md` — configuração de tasks, tags e generators
- `SPEC-QG2N8STY` — fundação DMPF; esta spec é a sua sucessora de implementação
- a análise AS-IS de sistemas legados — inventário AS-IS, incluindo o levantamento do `legado-golibs`
- `SPEC-6RQBN98G` — estratégia de testes e interoperabilidade que `KRN-11` realiza

## Verificação e testes

### Critérios de aceite

- [x] Existe ao menos um módulo Go no `go.work` com as três tags 3D
      (`type:lib`, `scope:backend`, `stack:go`) e a cadeia de validação passando
- [x] O verificador de conformidade roda no CI, é fail-closed e reprova o PR
      quando uma aresta viola C1 ou C2
- [x] Cada uma das 36 células da matriz de blocos tem vetor positivo **e**
      negativo executável — `Cell.Verify()` roda os dois por célula
      (`libs/backend/go/testkit/fitness/cells.go:98-113`), o oráculo de RFC §7.4
      é conferido à mão em `TestCellsMatchTheNormativeOracle` (36 pares, 17
      permitidas e 19 proibidas), a adulteração é detectada por
      `TestATamperedCellIsNamedByVerify`, e `tools/dmpf-cell-check.sh` prova as
      células 26 e 12 contra a árvore real
- [x] Uma unidade `domain` cujo fechamento de imports alcance `net/http` reprova
      com diagnóstico `DMPF-D001` ou violação de capability
- [x] Um package de produção sem entrada no manifesto **reprova** — não é tratado
      como não classificado
- [x] A UPR devolve `Decision` exaustiva; após `Rejected` nenhum evento de domínio
      é emitido e nenhuma mutação sobrevive
- [x] Estado de negócio e registro de outbox são gravados na mesma transação,
      comprovado por teste que falha o commit e verifica que nenhum dos dois
      persistiu
- [x] O relay publica fora de transação e só transiciona o registro quando o claim
      ainda é seu
- [ ] *(movido para o épico TypeScript)* `payload_hash` calculado em Go é
      idêntico ao calculado em TypeScript para o mesmo `Any.value` — não há
      kernel TypeScript neste repositório, e o "Escopo fora" desta spec o declara
      como o épico subsequente de ordem 2. A stack Go entrega a fórmula
      (`libs/backend/go/contracts/payloadhash`) e as golden fixtures que o outro
      lado consumirá
- [x] Nenhum artefato deste épico — código, README, contrato ou configuração —
      declara ou sugere exactly-once fim a fim
- [x] As 12 sub-specs `KRN-01`..`KRN-12` existem, com dependências e critérios de
      entrada explícitos — uma por trilha de `KRN-01` a `KRN-11`, todas `done`; o
      `KRN-12` desdobrou-se na guarda-chuva `SPEC-8HWBWJCB` e mais cinco specs
      `done`, com três `deferred` (o motor DSL do generator e seus dois
      complementos), diferidas por decisão e não por lacuna
- [x] Validação do projeto passando: `pnpm biome ci .` (114 arquivos, sem fixes) e
      a cadeia de lint sobre os 19 módulos Go, 19 de 19 conformes. O `staticcheck`
      reprovava `apps/backend/{orders,reservations}/dbtrace_test.go:47` com
      `SA1019` (`attribute.Value.Emit` deprecado no OTel `v1.46.0`); a chamada
      passou a `Value.String`, que para `case STRING` devolve o mesmo
      `v.stringly`

### Cenários de teste

```text
DADO um package classificado como "domain" no manifesto dmpf/units@1
QUANDO ele importa um package classificado como "port"
ENTÃO o verificador reprova com DMPF-D001 e o CI falha

DADO dois packages "domain" em bounded contexts diferentes
QUANDO um importa o outro
ENTÃO o verificador reprova com DMPF-D002, porque C2 exige mesmo contexto
      ou superfície pública de integração no destino

DADO uma UPR que recusa a operação por regra de negócio
QUANDO o application service executa o caso de uso
ENTÃO o desfecho é Rejected com código estável no formato contexto/motivo,
      a lista de eventos de domínio está vazia e o estado do agregado é
      idêntico ao de antes da chamada

DADO um caso de uso que persiste o agregado e entrega um evento à porta da outbox
QUANDO o commit falha
ENTÃO nem o estado de negócio nem o registro da outbox persistem

DADO um registro de outbox reivindicado por um relay cujo lease expirou
QUANDO um segundo relay o reivindica e publica
ENTÃO o primeiro relay não consegue transicionar o registro, porque o claim
      corrente não é mais o dele

DADO uma mensagem já processada, identificada por message_id e payload_hash
QUANDO ela é reentregue pelo broker
ENTÃO a inbox reconhece a duplicata, nenhum efeito é reaplicado e o ACK é emitido

DADO uma mensagem com o mesmo message_id porém payload_hash diferente
QUANDO ela chega ao consumidor
ENTÃO o consumo reprova por reutilização indevida do identificador,
      e não é tratado como redelivery legítima

DADO um package de produção não coberto por nenhum include do manifesto
QUANDO o verificador roda
ENTÃO ele reprova — a lacuna de configuração nunca é lida como conformidade

DADO uma chamada gRPC de saída dentro de um caso de uso com deadline em vigor
QUANDO o provider a executa
ENTÃO a duração restante trafega no fio, o receptor reconstrói o instante
      e o deadline não é reiniciado

DADO um erro retentável em operação sem semântica idempotente comprovada
QUANDO a política de retry é avaliada
ENTÃO nenhuma tentativa adicional é autorizada, porque a conjunção
      dos quatro fatores é falsa
```

<critical_constraints>
- [P0] Domínio sem I/O: nenhuma `verification_unit` de bloco `domain` importa Protobuf, ORM, broker, SDK de cloud, HTTP, logger, tracing, cache ou framework de execução
- [P0] Protobuf apenas no wire: tipo gerado de Protobuf NUNCA é estado de domínio, entrada de UPR ou evento de domínio
- [P0] At-least-once com efeitos idempotentes: nenhum componente, documento, configuração ou contrato deste épico promete exactly-once fim a fim
- [P0] A aresta `domain → port` é PROIBIDA, sem condicional e sem exceção
- [P0] Fail-closed: lacuna de configuração, classificação ausente, manifesto ausente e caso degenerado REPROVAM; nunca são lidos como conformidade
- [P0] Esta spec NÃO reabre decisão da fundação: divergência encontrada vira ADR novo ou escape hatch declarado, nunca alteração silenciosa da norma
- [P1] Paridade conceitual com o kernel TypeScript é por contrato e garantia, NUNCA por sintaxe ou mecanismo de concorrência: o kernel Go é idiomático em Go
</critical_constraints>

## Escopo fora

- **Kernel e SDK TypeScript**: é o épico subsequente de ordem 2 do ARQ-436. Esta
  spec entrega apenas a stack Go; o pareamento de vetores em `KRN-11` consome os
  fixtures, não implementa o lado TypeScript.
- **Migração do `legado-golibs` e dos serviços existentes**: pertence ao épico de ordem
  5 (piloto, estabilização e adoção). Aqui o acervo é referência, não alvo de
  refatoração.
- **Aplicação vertical de referência completa**: é o épico de ordem 4 (golden
  path). `KRN-12` entrega composition root e generator, não uma vertical de
  negócio ponta a ponta.
- **Repositório de contratos como produto separado**: `KRN-05` entrega a
  estrutura Buf e os gates neste repositório; a decisão de extrair um repositório
  de contratos próprio, com ownership formal, é do épico de ordem 3.
- **Escolha de registry de schemas em runtime**: continua sendo a pendência
  aberta `ADR-DMPF-N`, cujo owner é a série de ADRs estruturais sob FND-11. O
  kernel não pode depender de resolução remota de schema para desserializar.
- **Event Sourcing e CQRS físico**: permanecem extensões opt-in, conforme a
  fundação. O kernel não os impõe nem os implementa.
- **CDC / Debezium**: o relay padrão é polling com leasing. CDC é extensão
  admitida, fora do escopo desta entrega.
- **Dashboards, alarmes e runbook em ambiente produtivo**: `KRN-09` entrega a
  instrumentação e a exposição dos sinais; o catálogo operacional é de FND-08 e a
  operação é do épico de piloto.
- **Provisionamento de infraestrutura Kafka**: a política Kafka vale como norma de
  desenho, não como autorização de operação, até a revisão de infraestrutura de
  mensageria.
