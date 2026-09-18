---
id: SPEC-WYX5GW87
slug: dmpf-contratos-wire
title: DMPF KRN-05 — Contratos wire: CloudEvents, Buf e payload_hash
stage: done
priority: P0
depends_on: [SPEC-MQA5HAXF]
ticket_url: null
subtask_urls: []
created: 2026-09-02
---

# SPEC-WYX5GW87: DMPF KRN-05 — Contratos wire: CloudEvents, Buf e payload_hash

## Resumo

Entregar o bloco `contract` do kernel Go: a árvore de `.proto` sob governança
Buf com gates fail-closed no CI, a geração determinística para Go, o codec do
envelope `io.cloudevents.v1.CloudEvent` na modalidade `proto_data`, a função de
`payload_hash` na versão `1` da fórmula e a golden fixture do contrato de exemplo
`company.orders.event.v1.OrderPlaced` como oráculo independente de stack. Como
time de plataforma, queremos que o wire do DMPF exista como código verificado —
não só como norma em `docs/dmpf/` — para que `KRN-06` mapeie e serialize dentro
da transação de escrita e `KRN-07` separe redelivery legítima de reutilização
indevida do identificador, ambas sobre um tipo de wire que já passou pelo gate.

Esta é a primeira história do Incremento 3 (Garantias) da spec guarda-chuva
[SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md) e a primeira unidade real
do bloco `contract` no repositório: até aqui, `contract` existia apenas como
constante do verificador e como linha da matriz.

## Contexto

- **Problema**: o repositório não tem nenhum `.proto`, nenhuma configuração Buf
  e nenhum codec de envelope. `docs/dmpf/cloudevents-protobuf-buf.md` fixa o
  perfil CloudEvents (`ENV-*`), a política Protobuf (`PTB-*`), a governança Buf
  (`BUF-*`) e o layout do repositório de contratos (`REP-*`); o ADR-022 decide o
  codec e a fórmula do `payload_hash`; o ADR-023 fixa o Buf local e fail-closed
  como autoridade de validação. Nada disso é executável hoje. Sem o bloco
  `contract`, `KRN-06` não tem tipo de wire para mapear e `KRN-07` não tem hash
  para comparar.
- **Impacto**: a norma vira código sob gate. Quem publica um contrato passa por
  `buf lint` em `STANDARD`, `buf breaking` em `FILE` e pelo oráculo de dupla
  geração; quem consome o envelope tem um codec que recusa modalidade errada,
  atributo obrigatório ausente e divergência entre `dataschema`, *type URL* e
  `type`; quem calcula o `payload_hash` só consegue fazê-lo sobre bytes.
- **Inspiração**: o formato Protobuf oficial do CloudEvents (`io.cloudevents.v1`)
  e a governança Buf v2 (workspace explícito, lint por categoria, breaking por
  categoria, managed mode) — adotados como estão, com perfil organizacional que
  só acrescenta obrigatoriedade. Padrão de módulo Go, BOM e cadeia de validação
  vêm de `KRN-01`; o manifesto `dmpf/units@1` e o gate de conformidade, de
  `KRN-02`.
- **Links relevantes**:
  - [SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md) — guarda-chuva; o
    requisito `KRN-05` (linhas 153–163), a linha da tabela de histórias (390) e
    a árvore de localização (250–271), que reserva `libs/backend/go/contracts/`
    e `contracts/` na raiz.
  - [SPEC-MQA5HAXF](./SPEC-MQA5HAXF-dmpf-fundacao-nx-go.md) — `KRN-01`: convenção
    `libs/<scope>/<stack>/<módulo>`, tags 3D, `package.json` privado, BOM por
    `go run <pacote>@<versão>`, cadeia de sete passos.
  - [SPEC-WTAXFV8B](./SPEC-WTAXFV8B-dmpf-verificador-conformidade-go.md) —
    `KRN-02`: o verificador que decide a matriz 6×6, a política de capabilities
    e o baseline de unidades sob os quais esta lib nasce.
  - `docs/dmpf/cloudevents-protobuf-buf.md` — norma de origem (FND-05): §3
    (perfil), §4.2 (modalidade), §4.3 (`payload_hash`), §5 (`PTB-01`..`PTB-16`),
    §6 (`BUF-01`..`BUF-12`), §7 (`REP-01`..`REP-06`), §8 (`INT-01`, `INT-02`).
  - `docs/dmpf/testes-interop.md` — FND-09: formato de arquivo da golden fixture
    (`FIX-01`..`FIX-11`), casos discriminatórios (`ORA-08`).
  - `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §2.3 (P0-2, P0-3), §6.2 (bloco
    `contract package` limitado a `pure` e `wire.codec`), §7.4 (células 6, 12,
    24, 30 e 31).
  - ADRs: `docs/adr/019-niveis-de-contrato-e-conversao-de-evento.md` (`CTR-02`),
    `docs/adr/021-mapeamento-no-provider-serializacao-na-escrita.md`,
    `docs/adr/022-codec-cloudevents-proto-data-e-payload-hash.md`,
    `docs/adr/023-autoridade-de-validacao-e-registry.md`,
    `docs/adr/030-granularidade-modulo-go-e-bom.md`,
    `docs/adr/031-verificador-de-conformidade-dmpf-em-go.md`.
  - `docs/guides/dmpf-manifesto.md` — como escrever `dmpf-units.json`, `external[]`
    e `exceptions[]`.

### Fontes normativas

Esta spec **não reabre** nenhuma decisão da fundação nem da guarda-chuva. Onde a
fase monorepo exige adaptar a forma (não a substância) de uma regra escrita para
um repositório de contratos separado, a adaptação está declarada em "Decisões
técnicas" e será registrada em ADR (a partir de `032`) na finalização. A tabela
abaixo diz de onde vem cada exigência:

| Exigência | Fonte |
|-----------|-------|
| Envelope é o `io.cloudevents.v1.CloudEvent` oficial com perfil que só acrescenta obrigatoriedade | `ENV-07`; ADR-022 |
| Quinze atributos com presença declarada; omissão de obrigatório reprova | `ENV-08` (`cloudevents-protobuf-buf.md:481-497`) |
| `id`, `source`, `spec_version`, `type` em campos próprios; o resto no mapa de atributos | `ENV-09` (`:512-519`) |
| Condicional presente é não-vazio; ausente é ausente — sem valor de preenchimento | `ENV-12` |
| Modalidade única `proto_data` com `Any`; `binary_data` e `text_data` vedados | `ENV-15`; ADR-022 |
| `datacontenttype = application/protobuf`; *type URL* do `Any` **igual** a `dataschema` (literal); `dataschema` e `type` concordam na major | `ENV-16` (`:748`), `ENV-22` |
| `payload_hash` = SHA-256 hex minúsculo sobre `Any.value` como transportado, sem desserializar nem reserializar, fórmula versão `1`, não transportada | `ENV-17`..`ENV-20` (`:808-810`); ADR-022 |
| Pacote `<org>.<bounded_context>.<categoria>.<major>`; `type` `<reverse-dns>.<bounded_context>.<fato>.<major>`; nada de transporte no nome | `PTB-01`..`PTB-04` (`:944-947`) |
| Enum começa em `<NOME>_UNSPECIFIED = 0`; campo removido é `reserved` por número e nome; campos desconhecidos preservados | `PTB-05`, `PTB-06`, `PTB-09`, `PTB-10` |
| Workspace Buf v2 com módulos explícitos; `deps` declaradas e `buf.lock` versionado (vacuidade com `deps: []`) | `BUF-01`, `BUF-02` (`:1117-1118`) |
| Lint `STANDARD` com `enum_zero_value_suffix: _UNSPECIFIED`; exceção só nominal | `BUF-03` (`:1148`) |
| Breaking `FILE`; baseline com identidade declarada e resolvível; ausência reprova | `BUF-04`, `BUF-05` (`:1166-1167`) |
| CLI e plugins pinados por versão em arquivo versionado | `BUF-06` (`:1168`) |
| Máquina de estados `sem baseline` → `baseline estabelecido`, transição única, marca criada por quem não é o autor | `BUF-08` (`:1198-1249`) |
| Buf local, obrigatório e fail-closed é a autoridade de validação; registry em runtime é pendência aberta | `BUF-09`; ADR-023 |
| Managed mode; `.proto` neutro de linguagem | `BUF-10` (`:1281`) |
| Dupla geração idêntica byte a byte; drift entre gerado e versionado reprova | `BUF-11` (`:1282`) |
| Nenhum gate é advisory nem tem bypass | `BUF-12` (`:1283`) |
| Caminho do `.proto` espelha o pacote; gerado sob `gen/<stack>/` e nunca editado; owner de equipe em `CODEOWNERS` | `REP-01`..`REP-03` (`:1369-1371`) |
| `openapi/` e `asyncapi/` registrados, não normatizados | `REP-06` (`:1401`) |
| Golden fixture versionada, fonte única das duas stacks; uma por contrato `event` | `INT-01`, `INT-02` (`:1468-1469`) |
| Fixture cobre os três condicionais nos dois estados e um caso de campo desconhecido | §8.2 (`:1479-1509`) |
| Fixture em JSON, escalares como string, seções obrigatórias, versão do formato, caminho `fixtures/<bc>/<categoria>/<major>/<nome>.golden` | `FIX-05`..`FIX-11` (`testes-interop.md:1340-1346`) |
| Casos discriminatórios: timestamp, decimal/inteiro de 64 bits, enum conhecido/`UNSPECIFIED`/desconhecido, campo desconhecido | `ORA-08` (`testes-interop.md:1465`) |
| Bloco `contract` admite apenas `pure` e `wire.codec`; `contract → domain` proibida | RFC §6.2 (`:761-788`), §7.4 célula 31 (`:930`); `capability.go:44-53`, `matrix.go:9-63` |
| Tipo gerado nunca entra no domínio | `CTR-02` (ADR-019); P0-2 |
| Nenhum artefato declara ou sugere exactly-once fim a fim | P0-3 (RFC §2.3) |

<constraints>
- [P0] O envelope é o `io.cloudevents.v1.CloudEvent` do formato Protobuf oficial.
  O perfil só ACRESCENTA obrigatoriedade: nenhum atributo é removido, nenhuma
  semântica é alterada, nenhum envelope proprietário é aceito (`ENV-07`; ADR-022).
- [P0] A modalidade do `data` é EXCLUSIVAMENTE `proto_data` com
  `google.protobuf.Any`. `binary_data` e `text_data` são recusados pelo codec —
  não são fallback nem exceção (`ENV-15`).
- [P0] O `payload_hash` é SHA-256 em hexadecimal minúsculo sobre os bytes de
  `Any.value` EXATAMENTE como transportados. A função recebe `[]byte` e não
  existe caminho que aceite estrutura desserializada; nenhum atributo do envelope
  entra no cálculo (`ENV-17`..`ENV-19`; ADR-022).
- [P0] Todo gate Buf barra o merge: nenhum é advisory, nenhum é ignorável por
  aprovação de revisor, nenhum tem bypass documentado. Baseline ausente,
  irresolvível ou corrompido em módulo com marca de baseline REPROVA (`BUF-05`,
  `BUF-08`, `BUF-12`).
- [P0] O código gerado sob `gen/go/` NUNCA é editado à mão. Divergência entre
  duas gerações consecutivas, ou entre o gerado e o versionado, REPROVA
  (`BUF-11`, `REP-02`).
- [P0] O bloco `contract` só depende de `contract`. Nenhuma unidade desta lib
  importa `domain` nem qualquer outro bloco; nenhuma unidade `domain`,
  `application` ou `port` importa esta lib (RFC §7.4, células 6, 12, 24 e 31;
  `CTR-02`).
- [P0] O bloco `contract` admite apenas as capabilities `pure` e `wire.codec`.
  Dependência externa entra pela allowlist `external[]` com os quatro elementos;
  desvio de classificação entra por exceção NOMINAL em `exceptions[]` — nunca por
  categoria, prefixo ou diretório (RFC §6.2, §6.3, §6.4).
- [P0] Nenhum artefato desta entrega — `.proto`, comentário, README, fixture,
  código, mensagem de erro — declara nem sugere exactly-once fim a fim; a
  garantia é at-least-once com efeitos idempotentes (RFC §2.3, P0-3).
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva: toda
  adaptação à fase monorepo está declarada em "Decisões técnicas" e vira ADR na
  finalização, nunca alteração silenciosa da norma.
- [P1] O `.proto` permanece neutro de linguagem: `go_package` e qualquer opção de
  stack são aplicados pelo managed mode na geração, nunca escritos nos arquivos
  do contrato da organização (`BUF-10`).
- [P1] NUNCA redeclarar em `project.json` um target que o `@nx-go/nx-go` ou os
  `targetDefaults` já forneçam com o mesmo executor (ADR-002; ADR-030).
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Árvore de contratos sob Buf v2** (`BUF-01`, `BUF-02`, `REP-01`,
  `REP-06`): criar `contracts/` na raiz do repositório com `buf.yaml` v2
  declarando explicitamente o único módulo (`path: proto`), `lint.use: [STANDARD]`
  com `enum_zero_value_suffix: _UNSPECIFIED`, `breaking.use: [FILE]` e `deps: []`.
  - Sem dependência externa, `buf.lock` é satisfeito por vacuidade; o gate de
    pins (abaixo) passa a exigir o lock no commit que introduzir a primeira
    `dep`.
  - Os diretórios `contracts/openapi/` e `contracts/asyncapi/` existem com um
    `.gitkeep` cada e nada mais: registrados, não normatizados.
  - `contracts/README.md` descreve a árvore, a proveniência do envelope
    vendorizado (URL do arquivo no repositório `cloudevents/spec`, tag da release
    e SHA-256 do arquivo copiado) e o ciclo de vida do baseline. Nenhuma linha
    do README fala em garantia de entrega.
- [ ] **[P0] Envelope oficial vendorizado** (`ENV-07`): copiar o
  `cloudevents.proto` do formato Protobuf oficial do CloudEvents (tag `v1.0.2`
  de `cloudevents/spec`) para `contracts/proto/io/cloudevents/v1/cloudevents.proto`
  (pacote `io.cloudevents.v1`; caminho espelha o pacote), submetido apenas a
  `buf format`: a formatação muda espaços em branco e a ordem de linhas de
  `option`, não o descriptor. O README registra o SHA-256 do upstream e o do
  arquivo vendorizado, e o comando que reproduz a conferência.
  - O arquivo oficial carrega `option go_package`; o managed mode o sobrescreve
    na geração. Nenhuma opção é removida ou acrescentada ao arquivo.
  - `buf lint` em `STANDARD` passa no arquivo oficial (verificado). Se uma
    versão futura reprovar, a exceção é declarada na forma nominal exigida por
    `BUF-03` — par (arquivo, regra), justificativa, owner e data de revisão —
    dentro de `buf.yaml` (`lint.ignore_only`). Nenhuma exceção por diretório.
- [ ] **[P0] Contrato de exemplo** (`PTB-01`..`PTB-09`): criar
  `contracts/proto/company/orders/event/v1/order_placed.proto` com o conteúdo de
  `cloudevents-protobuf-buf.md:983-997`: `package company.orders.event.v1`,
  `message OrderPlaced { string order_id = 1; string customer_id = 2; int64
  total_cents = 3; OrderChannel channel = 4; reserved 5; reserved
  "legacy_promo_code"; }` e `enum OrderChannel { ORDER_CHANNEL_UNSPECIFIED = 0;
  ORDER_CHANNEL_WEB = 1; ORDER_CHANNEL_APP = 2; }`.
  - Comentário do arquivo declara o `type` do envelope correspondente:
    `com.company.orders.order-placed.v1` (`PTB-03`).
  - Nenhum nome no arquivo contém transporte, destino, ambiente ou tecnologia
    (`PTB-04`).
- [ ] **[P0] Geração determinística em managed mode** (`BUF-06`, `BUF-10`,
  `BUF-11`, `REP-02`): `contracts/buf.gen.yaml` v2 com `managed.enabled: true`,
  `override` de `go_package_prefix` para
  `github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go`,
  plugin `protoc-gen-go` invocado como `local: [go, run,
  google.golang.org/protobuf/cmd/protoc-gen-go@v<X.Y.Z>]` com `opt:
  paths=source_relative`, `out: ../libs/backend/go/contracts/gen/go` e
  `clean: true`.
  - A CLI Buf é invocada exclusivamente por `tools/buf.sh`, que executa
    `go run github.com/bufbuild/buf/cmd/buf@v<A.B.C> "$@"`. O pin da CLI vive
    só nesse arquivo; o pin do plugin vive só em `buf.gen.yaml` (um lugar por
    ferramenta, como no BOM de ADR-030).
  - As versões concretas são resolvidas no plano pela última release estável
    listada por `go list -m -versions` de cada módulo, e a versão do plugin é
    IGUAL à de `google.golang.org/protobuf` no `go.mod` da lib.
  - O gerado é versionado em `libs/backend/go/contracts/gen/go/io/cloudevents/v1/cloudevents.pb.go`
    e `.../gen/go/company/orders/event/v1/order_placed.pb.go`.
- [ ] **[P0] Lib `contracts`** (`KRN-01`; ADR-030): criar
  `libs/backend/go/contracts/` como módulo Go
  `github.com/mateusmacedo/dmpf/libs/backend/go/contracts`,
  `go 1.26.4`, registrado no `go.work`, projeto Nx `contracts` com tags
  `["type:lib", "scope:backend", "stack:go"]`, `package.json`
  `{"name": "@mateusmacedo/contracts", "version": "0.0.0", "private": true}`,
  targets `fmt-check`, `vet`, `build`, `test-race`, `govulncheck` declarados
  como nos dois módulos existentes, e `lint`/`test` herdados dos `targetDefaults`.
  - Os `inputs` de todos os targets incluem `{workspaceRoot}/contracts/**` e
    `{workspaceRoot}/tools/buf.sh`, para que mudança só em contrato torne a lib
    afetada.
  - `go.mod` declara `google.golang.org/protobuf` como única dependência direta;
    `go.sum` é versionado.
- [ ] **[P0] Manifesto `dmpf/units@1` e baseline** (`KRN-02`; RFC §6.3, §6.4,
  §10.2): `libs/backend/go/contracts/dmpf-units.json` declara três unidades,
  todas `block: "contract"`, `bounded_context: "kernel"`,
  `public_integration_surface: true`:
  - `contracts/gen` — `include` com os dois import paths gerados;
  - `contracts/envelope` — `include` `.../contracts/envelope`;
  - `contracts/payloadhash` — `include` `.../contracts/payloadhash`.
  - `external[]` com uma entrada para `google.golang.org/protobuf`,
    `capability: "wire.codec"`, `versions` igual à faixa da major pinada e
    `entrypoints` listando exatamente os subpacotes importados pelo gerado e pelo
    codec (`proto`, `types/known/anypb`, `types/known/timestamppb`,
    `reflect/protoreflect`, `runtime/protoimpl`) — a lista final é a dos imports
    efetivamente emitidos pelo plugin pinado, verificada no plano.
  - `exceptions[]` com uma entrada nominal por par (`contracts/gen`,
    `reflect`) e (`contracts/gen`, `unsafe`): o `protoc-gen-go` emite esses
    imports em todo arquivo gerado e o verificador os classifica
    `runtime.framework` (`stdlib.go:64-65`); `reason` cita o plugin pinado,
    `owner` é a equipe dona do bounded context, `review_by` é `2027-03-02`
    (revisão a cada troca de versão do plugin ou em seis meses, o que vier
    primeiro).
  - `tools/dmpf-baseline/units-baseline.json` ganha as três unidades e o
    `digest` recalculado, em **commit próprio** separado do código (RFC §10.2
    T2; `DMPF-T002`).
- [ ] **[P0] Codec do envelope** (`ENV-08`, `ENV-09`, `ENV-12`, `ENV-15`,
  `ENV-16`, `ENV-22`): pacote `envelope` em
  `libs/backend/go/contracts/envelope/` com:
  - `type Envelope struct` com os quinze atributos: `ID`, `Source`,
    `SpecVersion`, `Type`, `Subject`, `Time *timestamppb.Timestamp`,
    `DataSchema`, `DataContentType`, `CorrelationID`, `CausationID`,
    `PartitionKey`, `TraceParent` (obrigatórios, `string` salvo `Time`),
    `AggregateVersion *int32`, `TenantID *string`, `TraceState *string`
    (condicionais, ponteiro nulo = ausente) e `Payload []byte` (os bytes de
    `Any.value`).
  - `Encode(e Envelope) (*cloudeventsv1.CloudEvent, error)`: valida e produz o
    proto oficial com `id`, `source`, `spec_version` e `type` nos campos
    próprios; os demais no mapa `attributes` com o `CloudEventAttributeValue`
    do tipo declarado em `ENV-08` (`subject`, `datacontenttype`,
    `correlationid`, `causationid`, `partitionkey`, `traceparent`, `tenantid`,
    `tracestate` → `ce_string`; `time` → `ce_timestamp`; `dataschema` →
    `ce_uri`; `aggregateversion` → `ce_integer`); `data` é `proto_data` com
    `Any{type_url: e.DataSchema, value: e.Payload}`. Condicional ausente não
    entra no mapa.
  - `Decode(ce *cloudeventsv1.CloudEvent) (Envelope, error)`: recusa `data` em
    `binary_data`, `text_data` ou ausente (`ErrModality`); recusa atributo
    obrigatório ausente ou vazio (`ErrMissingAttribute` com o nome); recusa
    condicional presente e vazio (`ErrEmptyConditional`); recusa
    `datacontenttype != application/protobuf` (`ErrContentType`); recusa
    `spec_version != "1.0"` (`ErrSpecVersion`); recusa *type URL* do `Any`
    diferente de `dataschema` por comparação literal (`ErrSchemaMismatch`);
    recusa `dataschema` que não seja um *type URL* de `Any` (prefixo
    `type.googleapis.com/` seguido do nome qualificado) (`ErrDataSchemaForm`);
    recusa major do nome qualificado em `dataschema` (último segmento do
    pacote, `v<N>`) diferente da major do `type` (último segmento após o ponto)
    (`ErrMajorMismatch`). Atributo com tipo de valor diferente do declarado
    reprova (`ErrAttributeType`). Ordem das conferências de `ENV-16`: (a) *type
    URL* literal antes de (b) concordância de major.
  - `Pack(msg proto.Message) (payload []byte, typeURL string, err error)`:
    serializa com `proto.MarshalOptions{Deterministic: true}` e devolve o
    *type URL* na forma que `anypb` produz (`type.googleapis.com/<nome
    qualificado>`); é o único caminho de produção de `Payload` e `DataSchema`
    a partir de mensagem.
  - Constantes exportadas `SpecVersion = "1.0"` e `ContentType =
    "application/protobuf"`. Toda constante de erro é `errors.Is`-comparável.
  - O pacote importa apenas `errors`, `fmt`, `strings`, os subpacotes de
    `google.golang.org/protobuf` listados no `external[]` e os pacotes gerados
    da própria lib. Não importa `time` (`io.clock`) nem `encoding/json`.
- [ ] **[P0] `payload_hash` versão 1** (`ENV-17`..`ENV-20`; ADR-022): pacote
  `payloadhash` em `libs/backend/go/contracts/payloadhash/` com
  `const FormulaVersion = 1` e `func Sum(anyValue []byte) string` que devolve
  `hex.EncodeToString(sha256.Sum256(anyValue))`. Não há função que receba
  `proto.Message`, `*anypb.Any` nem `*cloudeventsv1.CloudEvent`; quem precisa do
  hash de um envelope passa `Envelope.Payload`. Imports: `crypto/sha256` e
  `encoding/hex`, ambos `pure` (`stdlib.go:44,95`).
- [ ] **[P0] Golden fixture** (`INT-01`, `INT-02`, §8.2, `FIX-05`..`FIX-11`,
  `ORA-08`): criar `contracts/fixtures/orders/event/v1/order-placed.golden`,
  JSON, único por contrato-major, com **todo escalar como string** (`FIX-07`) e
  as seções:
  - `format_version: "1"` (`FIX-09`);
  - `identity`: `contract.package = "company.orders.event.v1"`,
    `contract.message = "OrderPlaced"`, `envelope.type =
    "com.company.orders.order-placed.v1"`, `dataschema =
    "type.googleapis.com/company.orders.event.v1.OrderPlaced"` (`FIX-05` a);
  - `covers`: `profile_major: "1"`, `contract_major: "v1"` (`FIX-04`, e);
  - `cases[]`: cada caso com `name`, `doc` (`FIX-08`), `envelope` (os quinze
    atributos com valores fixos, condicional ausente omitido), `payload` (campo
    a campo), `payload_bytes_hex` (os bytes de `Any.value` como transportados,
    literal) e `payload_hash` (o SHA-256 declarado). Casos obrigatórios:
    (1) `all-conditionals-present` — `aggregateversion`, `tenantid` e
    `tracestate` presentes, `channel = ORDER_CHANNEL_WEB`; (2)
    `all-conditionals-absent` — os três ausentes, `channel =
    ORDER_CHANNEL_APP`; (3) `channel-unspecified` — `channel =
    ORDER_CHANNEL_UNSPECIFIED`, campo omitido no wire; (4) `total-cents-beyond-
    double` — `total_cents = "9007199254740993"` (2^53 + 1, prova `FIX-07`);
    (5) `time-with-nanos` — `time` com nanossegundos não nulos.
  - `discriminators[]` (`FIX-05` d; `ORA-08`): (6) `unknown-field` — bytes com
    um campo de número 7 desconhecido, que a desserialização preserva
    (`PTB-10`) e o hash cobre; (7) `enum-unknown-value` — `channel = "99"`;
    (8) `non-canonical-field-order` — bytes válidos com os campos em ordem
    decrescente de número, cujo `payload_hash` declarado é o dos bytes
    transportados e **difere** do hash da reserialização em Go. É o caso que dá
    conteúdo a `ENV-18`.
  - O arquivo carrega apenas valores literais: nenhum relógio, aleatório ou
    ordem de `map` (`FIX-08`).
- [ ] **[P0] Teste de fixture em Go** (§8.3 oráculos 1 e 2): pacote de teste
  `libs/backend/go/contracts/golden/golden_test.go` carrega o `.golden` por
  caminho relativo, falha em `format_version` desconhecida (`FIX-09`) e, para
  cada caso: decodifica `payload_bytes_hex` no tipo gerado e confere campo a
  campo com `payload`; confere `payloadhash.Sum(bytes) == payload_hash`; monta
  o envelope a partir de `envelope` e passa `Encode` → `Decode` sem perda;
  para o caso (8), afirma adicionalmente que
  `payloadhash.Sum(proto.Marshal(decodificado)) != payload_hash`. O oráculo 3
  (identidade de bytes) é reportado como informativo e não reprova (§8.3).
- [ ] **[P0] Gates Buf no CI, fail-closed** (`BUF-03`..`BUF-06`, `BUF-08`,
  `BUF-11`, `BUF-12`): `tools/buf-gate.sh` com quatro subcomandos, cada um
  exposto como target Nx do projeto `contracts` e todos executados por
  um passo novo do `ci.yml`, `Contracts gates (affected)`, via `pnpm nx affected
  -t buf-lint,buf-breaking,buf-generate-check,buf-pins`:
  - `lint`: `buf format --diff --exit-code contracts` seguido de `buf lint
    contracts`; depois, varredura estrutural de P0-3: `grep -riE "exactly[ -]once"`
    em `contracts/` e `libs/backend/go/contracts/` reprova em qualquer
    ocorrência.
  - `breaking`: implementa `BUF-08`. Para cada módulo declarado em `buf.yaml`,
    a marca de baseline é a tag anotada `contracts-baseline/<path do módulo>`.
    Com a marca presente: `buf breaking contracts --against
    ".git#ref=${NX_BASE},subdir=contracts"`; `NX_BASE` vazio, ref irresolvível
    ou `contracts/` ausente na ref REPROVAM. Sem a marca: se o módulo já existe
    em `${NX_BASE}` (`git ls-tree`), REPROVA (marca removida de módulo com
    histórico); senão, estado `sem baseline` — o breaking é dispensado só para
    esse módulo, com aviso explícito na saída, e `lint`, `format` e geração
    continuam obrigatórios. Vetor de falsificação: diretório de pacote publicado
    em `${NX_BASE}` que apareça sob outro módulo em `HEAD` REPROVA. Autoria:
    e-mail do `tagger` da marca igual ao e-mail do autor do primeiro commit que
    adicionou o diretório do módulo REPROVA.
  - `generate-check`: duas execuções de `buf generate contracts -o <tmp1>` e
    `-o <tmp2>` em diretórios limpos; `diff -r` entre as duas REPROVA em
    divergência; `diff -r` entre `<tmp1>/libs/backend/go/contracts/gen/go`
    e o versionado REPROVA em divergência (drift).
  - `pins`: `tools/buf.sh` contém exatamente um `@v<major>.<minor>.<patch>`;
    `buf.gen.yaml` tem todo plugin `local` com `@v<major>.<minor>.<patch>`
    exato; a versão do `protoc-gen-go` é igual à de `google.golang.org/protobuf`
    no `go.mod` da lib; `deps` não-vazio em `buf.yaml` exige `buf.lock`
    presente. Qualquer `latest`, faixa ou ausência REPROVA.
  - Nenhum subcomando tem `continue-on-error`, `|| true` ou flag de bypass.
- [ ] **[P0] Autoteste do gate** (`BUF-12`; padrão de `tools/dmpf-gate-check.sh`):
  `tools/tests/buf-gate/buf-gate.test.sh` cria repositórios git temporários e
  prova que `breaking` reprova nos três vetores negativos de `BUF-08`, reprova
  com `NX_BASE` vazio e com ref irresolvível, dispensa corretamente em `sem
  baseline`, e que `generate-check` reprova ao editar um byte do gerado;
  exposto como target `buf-gate-selftest` com `inputs` em `tools/buf-gate.sh` e
  `tools/tests/buf-gate/**`, incluído no mesmo passo do CI.
- [ ] **[P0] `CODEOWNERS` com owner de equipe** (`REP-03`): o diretório
  `contracts/proto/company/orders/` e `contracts/fixtures/orders/` têm como
  owner uma equipe da organização `mateusmacedo` no arquivo `CODEOWNERS` que
  a forge lê. O `.github/CODEOWNERS` atual é herança do template com placeholder
  `@owner`; o caminho que o Gitea efetivamente lê (raiz, `.gitea/` ou `docs/`) e
  o handle da equipe são confirmados no plano pela documentação e pela API do
  Gitea (`/orgs/mateusmacedo/teams`), e o arquivo lido pela forge é o que recebe
  a entrada. Owner pessoa física REPROVA na revisão.
- [ ] **[P1] Documentação de apoio**: `docs/guides/dmpf-manifesto.md` ganha uma
  seção curta "Bloco `contract` e código gerado" com o exemplo de `external[]`
  para `google.golang.org/protobuf` e das exceções nominais para `reflect` e
  `unsafe`; `AGENTS.md` ("Libs" e "Comandos") passa a listar `contracts`
  e os quatro subcomandos de `tools/buf-gate.sh`; `docs/nx-reference/tasks.md`
  registra que Go é `scope:backend` neste workspace e lista os targets `buf-*`.

### Não-funcionais

- [ ] Determinismo: duas gerações consecutivas em ambiente limpo produzem bytes
  idênticos; o autoteste prova a reprovação por drift.
- [ ] Fail-closed: toda ramificação do gate que não consiga avaliar (variável
  vazia, ref irresolvível, arquivo ausente, tag sem tagger) termina com código
  de saída diferente de zero e mensagem que nomeia a condição.
- [ ] Reprodutibilidade de toolchain: nenhum binário instalado fora do BOM —
  `buf`, `protoc-gen-go`, `golangci-lint` e `govulncheck` entram por `go run
  <pacote>@<versão>` exata; o `setup-go` do CI já provê o Go `1.26.4`.
- [ ] Cadeia Go completa verde na lib: `fmt-check`, `vet`, `lint`, `build`,
  `test`, `test-race`, `govulncheck`; o `fmt-check` cobre o gerado, que o
  `protoc-gen-go` emite já em forma `gofmt`.
- [ ] Verificador de conformidade aprovando: zero `DMPF-D001`/`DMPF-D002`
  envolvendo unidade `contract`, zero `DMPF-E001` após allowlist e exceções,
  `DMPF-T001`/`DMPF-T002` ausentes com o baseline atualizado em commit próprio.
- [ ] Tempo: o passo `Contracts gates (affected)` só roda quando
  `contracts` está no `affected`; um PR só de TypeScript não paga por
  ele.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Contratos (`contracts/`) | [x] | Nasce: workspace Buf v2, envelope vendorizado, contrato de exemplo, fixture, pins, README |
| Bloco `contract` (lib Go) | [x] | Nasce `contracts`: gerado, codec do envelope, `payload_hash`, teste de fixture |
| Workspace Go | [x] | `go.work` ganha o terceiro módulo; primeiro módulo com dependência externa (`go.sum`) |
| Nx | [x] | Novo projeto `scope:backend`; targets `buf-*` e `buf-gate-selftest`; `inputs` cobrindo `contracts/**` |
| Governança DMPF | [x] | Primeira unidade real do bloco `contract`; `external[]` e `exceptions[]` inaugurados; baseline com três unidades novas |
| CI | [x] | Passo `Contracts gates (affected)`; sem mudança nos passos existentes |
| Ferramentas (`tools/`) | [x] | `buf.sh`, `buf-gate.sh`, `tests/buf-gate/` |
| Ownership | [x] | `CODEOWNERS` lido pela forge com owner de equipe para `orders` |
| Documentação | [x] | Guia do manifesto, `AGENTS.md`, `tasks.md`; ADR-033 na finalização |
| Bloco `domain`, `application`, `port`, `provider`, `app` | [ ] | Intocados — nenhum deles importa esta lib nesta história |
| `.golangci.yml` | [ ] | Intocado — a regra `depguard` cobre só `**/*-domain/**`; o gate do bloco `contract` é o verificador |

## Localização de código

```text
dmpf/
├── go.work                                   — MODIFICAR: use ./libs/backend/go/contracts
├── go.work.sum                               — CRIAR (gerado pelo toolchain ao resolver a dependência)
├── contracts/                                — CRIAR (fonte e governança; neutro de stack)
│   ├── README.md                             — árvore, proveniência do envelope, ciclo do baseline
│   ├── buf.yaml                              — workspace v2: modules [proto]; lint STANDARD; breaking FILE; deps []
│   ├── buf.gen.yaml                          — managed mode; protoc-gen-go local pinado; out ../libs/backend/go/contracts/gen/go
│   ├── proto/
│   │   ├── io/cloudevents/v1/cloudevents.proto           — envelope oficial vendorizado, íntegro
│   │   └── company/orders/event/v1/order_placed.proto    — contrato de exemplo (PTB-01..PTB-09)
│   ├── fixtures/orders/event/v1/order-placed.golden      — golden fixture JSON (FIX-05..FIX-11, ORA-08)
│   ├── openapi/.gitkeep                      — registrado, não normatizado (REP-06)
│   └── asyncapi/.gitkeep                     — registrado, não normatizado (REP-06)
├── libs/backend/go/contracts/            — CRIAR (bloco contract; projeto Nx contracts)
│   ├── go.mod                                — module .../libs/backend/go/contracts; go 1.26.4; require google.golang.org/protobuf
│   ├── go.sum
│   ├── package.json                          — @mateusmacedo/contracts, private
│   ├── project.json                          — tags [type:lib, scope:backend, stack:go]; targets Go + buf-* + buf-gate-selftest
│   ├── dmpf-units.json                       — 3 unidades contract; external[protobuf]; exceptions[reflect, unsafe]
│   ├── gen/go/                               — GERADO, nunca editado (REP-02)
│   │   ├── io/cloudevents/v1/cloudevents.pb.go
│   │   └── company/orders/event/v1/order_placed.pb.go
│   ├── envelope/
│   │   ├── envelope.go                       — Envelope, Encode, Decode, Pack, constantes, erros
│   │   └── envelope_test.go
│   ├── payloadhash/
│   │   ├── payloadhash.go                    — FormulaVersion, Sum
│   │   └── payloadhash_test.go
│   └── golden/
│       └── golden_test.go                    — carrega ../../../../contracts/fixtures/orders/event/v1/order-placed.golden
├── tools/
│   ├── buf.sh                                — CRIAR: go run github.com/bufbuild/buf/cmd/buf@v<A.B.C> "$@" (único pin da CLI)
│   ├── buf-gate.sh                           — CRIAR: lint | breaking | generate-check | pins
│   ├── tests/buf-gate/buf-gate.test.sh       — CRIAR: vetores negativos de BUF-08, drift, pins
│   └── dmpf-baseline/units-baseline.json     — MODIFICAR (commit próprio): +3 unidades, digest recalculado
├── .github/workflows/ci.yml                  — MODIFICAR: passo "Contracts gates (affected)"
├── <CODEOWNERS lido pela forge>              — MODIFICAR ou CRIAR: owner de equipe para contracts/**/orders/
├── docs/guides/dmpf-manifesto.md             — MODIFICAR: seção "Bloco contract e código gerado"
├── docs/nx-reference/tasks.md                — MODIFICAR: Go é scope:backend; targets buf-*
├── AGENTS.md                                 — MODIFICAR: inventário de libs e comandos
└── docs/adr/032-*.md                         — CRIAR na finalização (Etapa 6): adaptações da fase monorepo
```

Import path canônico do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/contracts`.

**Arquivos a modificar**:
- `go.work` — adicionar `./libs/backend/go/contracts` ao bloco `use`.
- `tools/dmpf-baseline/units-baseline.json` — três entradas novas e `digest`
  recalculado, em commit separado do código (RFC §10.2 T2).
- `.github/workflows/ci.yml` — novo passo após `Go gates (affected)`:
  `pnpm nx affected -t buf-lint,buf-breaking,buf-generate-check,buf-pins,buf-gate-selftest --base=$NX_BASE --head=$NX_HEAD --parallel=1 --exclude=$NX_EXCLUDE`.
  `--parallel=1` porque `breaking` e `generate-check` compartilham o
  `GOMODCACHE` na primeira resolução do `buf`.
- `CODEOWNERS` lido pela forge — entradas para `/contracts/proto/company/orders/`
  e `/contracts/fixtures/orders/` com handle de equipe.
- `docs/guides/dmpf-manifesto.md`, `docs/nx-reference/tasks.md`, `AGENTS.md` —
  conforme requisito P1.

## Design

### Arquitetura

```text
             contracts/ (fonte, neutro de stack)                 libs/backend/go/contracts (bloco contract)
 ┌──────────────────────────────────────────────┐    buf generate    ┌──────────────────────────────────────┐
 │ buf.yaml  buf.gen.yaml  tools/buf.sh (pins)  │ ────────────────▶ │ gen/go/io/cloudevents/v1   (gerado)  │
 │ proto/io/cloudevents/v1/cloudevents.proto    │  managed mode      │ gen/go/company/orders/event/v1       │
 │ proto/company/orders/event/v1/order_placed   │                    ├──────────────────────────────────────┤
 │ fixtures/orders/event/v1/order-placed.golden │ ──── lido por ───▶ │ golden/golden_test.go  (oráculos 1,2)│
 └──────────────────────────────────────────────┘                    │ envelope/   Encode · Decode · Pack   │
            ▲            ▲             ▲                             │ payloadhash/ Sum(anyValue) → hex     │
            │ lint       │ breaking    │ generate-check / pins       └──────────────────────────────────────┘
      tools/buf-gate.sh (4 subcomandos, fail-closed) ◀── tools/tests/buf-gate/*.test.sh (autoteste)
            ▲
      ci.yml "Contracts gates (affected)"  ·  conformance (matriz, capabilities, baseline)

 Consumidores futuros (fora desta história): provider (KRN-06/07) → contract   [célula 30, permitida]
 Nunca:                                       domain/application/port → contract [células 6, 12, 24]
                                              contract → qualquer outro bloco    [célula 31]
```

O `.proto` é a fonte; `gen/go/` é derivado e versionado; `envelope` e
`payloadhash` são código de mão que só conhece o gerado e o runtime Protobuf.
A fixture é lida pelo teste Go hoje e será lida pelo teste TypeScript em
`KRN-11`, sem cópia (`FIX-02`).

### Fluxo principal — publicar e consumir um evento

1. O produtor (um `provider`, em `KRN-06`) constrói a mensagem gerada
   `OrderPlaced` e chama `envelope.Pack(msg)` → recebe `payload` (bytes
   determinísticos) e `typeURL`.
2. Preenche `envelope.Envelope` com os doze atributos obrigatórios, os
   condicionais que o fato exige, `DataSchema = typeURL`, `DataContentType =
   envelope.ContentType`, `Payload = payload`.
3. `envelope.Encode(e)` valida `ENV-08`/`ENV-12`/`ENV-16` e devolve o
   `*cloudeventsv1.CloudEvent` oficial, com `data` em `proto_data`.
4. O produtor serializa o `CloudEvent` com `proto.Marshal` e grava na outbox
   (fora desta história). O `payload_hash` a persistir é
   `payloadhash.Sum(e.Payload)`.
5. O consumidor (`KRN-07`) desserializa o `CloudEvent`, chama
   `envelope.Decode(ce)`: recusa modalidade, atributo ausente, conteúdo, versão
   e divergência de schema; devolve `Envelope` com `Payload` = `Any.value`
   intocado.
6. O consumidor calcula `payloadhash.Sum(env.Payload)` e compara com o hash
   persistido pela chave `(source, id)` — igual é redelivery legítima, diferente
   é reutilização indevida (FND-04 `INB-13`). Só depois desserializa `Payload`
   no tipo gerado.

### Fluxo do gate — de PR a veredicto

```text
buf-gate.sh lint
  buf format --diff --exit-code contracts  → diff ≠ ∅ → REPROVA
  buf lint contracts (STANDARD)            → achado   → REPROVA
  grep -riE "exactly[ -]once" contracts/ libs/backend/go/contracts/ → match → REPROVA

buf-gate.sh breaking
  [ -z "$NX_BASE" ] → REPROVA ("baseline não declarado")
  git rev-parse --verify "$NX_BASE" falha → REPROVA ("baseline irresolvível")
  para cada módulo M em buf.yaml:
    marca = refs/tags/contracts-baseline/M
    se marca existe:
      tagger(marca) == autor(primeiro commit que adicionou contracts/M) → REPROVA (autoria = autorização)
      buf breaking contracts --against ".git#ref=$NX_BASE,subdir=contracts" → falha → REPROVA
    senão:
      git ls-tree "$NX_BASE" contracts/M não-vazio → REPROVA (marca ausente em módulo com histórico)
      senão → aviso "M em estado sem baseline: breaking dispensado" (lint/format/gen seguem obrigatórios)
  para cada diretório de pacote D publicado em $NX_BASE sob contracts/proto:
    D ausente no mesmo módulo em HEAD → REPROVA (redeclaração de pacote publicado)

buf-gate.sh generate-check
  buf generate contracts -o T1 ; buf generate contracts -o T2
  diff -r T1 T2 → ≠ → REPROVA (geração não reprodutível)
  diff -r T1/libs/backend/go/contracts/gen/go libs/backend/go/contracts/gen/go → ≠ → REPROVA (drift)

buf-gate.sh pins
  tools/buf.sh sem exatamente 1 "@vX.Y.Z"           → REPROVA
  buf.gen.yaml com plugin local sem "@vX.Y.Z"       → REPROVA
  versão protoc-gen-go ≠ google.golang.org/protobuf em go.mod → REPROVA
  buf.yaml deps ≠ [] e buf.lock ausente             → REPROVA
```

### Pseudocódigo — `Decode` e a dupla conferência de `ENV-16`

```text
Decode(ce):
  se ce.data não é proto_data → ErrModality
  env.ID, env.Source, env.SpecVersion, env.Type ← campos próprios
  para cada obrigatório em [id, source, specversion, type, subject, time, dataschema,
                            datacontenttype, correlationid, causationid, partitionkey, traceparent]:
      ausente ou vazio → ErrMissingAttribute(nome)
  para cada condicional em [tenantid, tracestate]:
      presente e vazio → ErrEmptyConditional(nome)
  // aggregateversion é inteiro: zero é valor legítimo, não há forma "vazia" (ENV-12 é sobre strings)
  tipo do CloudEventAttributeValue ≠ tipo de ENV-08 → ErrAttributeType(nome)
  env.SpecVersion ≠ "1.0" → ErrSpecVersion
  env.DataContentType ≠ "application/protobuf" → ErrContentType
  any := ce.proto_data
  any.type_url ≠ env.DataSchema (comparação literal) → ErrSchemaMismatch        // ENV-16 (a)
  fqn := sufixo de any.type_url após a última "/"                              // company.orders.event.v1.OrderPlaced
  majorSchema := penúltimo segmento de fqn                                     // v1
  majorType := último segmento de env.Type após o último "."                   // v1
  majorSchema ≠ majorType → ErrMajorMismatch                                   // ENV-16 (b), via ENV-22
  env.Payload = any.value                                                      // bytes intocados
  retorna env
```

A conferência (b) é por concordância de major, nunca por igualdade de string:
`dataschema` carrega a forma de `PTB-01` e `type` a de `PTB-03`, estruturalmente
diferentes.

## Decisões técnicas

| Decisão | Alternativas descartadas |
|---------|--------------------------|
| **`contracts/` na raiz como fonte; gerado dentro da lib.** A fonte (`.proto`, Buf, fixtures, pins) é neutra de stack e fica onde a guarda-chuva a desenhou (`SPEC-YRJRADY9:268`); o gerado Go fica em `libs/backend/go/contracts/gen/go/`, porque o verificador classifica pacotes Go de módulos Nx e o bloco `contract` precisa ser um módulo com `dmpf-units.json`. É a adaptação de `REP-02`/§7.1 (`contracts/gen/go/`) à fase monorepo: a semântica — `gen/<stack>/`, nunca editado, drift reprova — é preservada; muda só o diretório-raiz do `gen/`. O ticket pediu que esta escolha fosse feita no refinamento e registrada; vira ADR-033. | `contracts/gen/go/` como módulo Go próprio: quebraria `libs/<scope>/<stack>/<módulo>` e criaria um módulo fora do Nx. `contracts/` inteiro dentro da lib: acopla a fonte neutra de stack à stack Go e obriga `KRN-11` a gerar TypeScript a partir de dentro de uma lib Go. |
| **Envelope oficial vendorizado e gerado pelo mesmo toolchain.** `io.cloudevents.v1` entra como arquivo íntegro em `proto/io/cloudevents/v1/`, com proveniência (URL, tag, SHA-256) no README, e é gerado pelo `protoc-gen-go` pinado, sob o mesmo oráculo de drift que o contrato da organização. | Depender do `pb` do `cloudevents/sdk-go`: traz um módulo grande cuja geração não passa por `BUF-11`, e fixa a versão do envelope à do SDK. Depender de módulo no BSR: exige rede na resolução e uma entrada em `deps`, para um arquivo que não muda. |
| **Um só módulo Buf (`proto`).** O workspace v2 lista explicitamente `proto`, que contém o envelope vendorizado e o contrato de exemplo. `BUF-01` exige lista explícita, não exige mais de um módulo. | Módulo separado para o envelope: duplica marca de baseline, `CODEOWNERS` e estado de `BUF-08` para um arquivo cuja evolução é externa. |
| **Buf CLI e `protoc-gen-go` por `go run <pacote>@<versão>`.** Mesmo padrão do BOM de `KRN-01` (`golangci-lint`, `govulncheck`): nenhum binário instalado no runner, pin exato em arquivo versionado, cache no `GOMODCACHE`. O pin da CLI vive só em `tools/buf.sh`; o do plugin, só em `buf.gen.yaml`. | Download do binário `buf` das releases com checksum: funciona, mas introduz um segundo mecanismo de BOM. Plugin remoto do BSR: geração dependente de rede e de disponibilidade externa, contra o espírito de `BUF-09` (validação local). |
| **Marca de `BUF-08` = tag anotada `contracts-baseline/<módulo>`; baseline de comparação = `${NX_BASE}`.** A tag anotada tem `tagger` (identidade de quem autorizou) e é protegida na forge; `NX_BASE` é a branch-alvo protegida do PR, já usada pelo gate de conformidade. Os três vetores negativos de `BUF-08` são verificados mecanicamente; a parte "aprovação por revisor distinto" fica com a forge, como em ADR-031. | Branch de baseline dedicada: não carrega identidade de quem autorizou. Arquivo de marca no repositório: qualquer autor cria no próprio PR, anulando a autorização distinta. |
| **Este PR entra em estado `sem baseline`.** É a transição prevista por `BUF-08` para o primeiro conteúdo: `breaking` é dispensado só para o módulo `proto` e só até a marca existir; `lint`, `format`, `generate-check` e `pins` são obrigatórios desde o primeiro commit. A criação da marca exige uma segunda pessoa com direito de push de tag na forge — pré-requisito organizacional registrado no README e no ADR-033, não relaxado aqui. | Criar a marca no próprio PR: reprova por autoria = autorização (vetor 3). Rodar `breaking` contra um `NX_BASE` sem `contracts/`: falha por irresolução, o que é correto para `baseline estabelecido` e errado para bootstrap — é exatamente a ambiguidade que `BUF-08` elimina. |
| **`reflect` e `unsafe` do gerado por exceção nominal.** O `protoc-gen-go` emite `reflect`, `sync` e `unsafe` em todo arquivo; o verificador classifica `reflect`/`unsafe` como `runtime.framework` (`stdlib.go:64-65`), não admitida em `contract`. A exceção por par (unidade, dependência) com razão, owner e `review_by` é o instrumento que a RFC §6.4 criou para isso, e mantém o verificador intocado. | Reclassificar `reflect`/`unsafe` como `wire.codec` ou `pure` no verificador: reabre `KRN-02`/ADR-031 e afrouxa a tabela para todo bloco. Excluir arquivos `// Code generated` da análise: heurística por texto, contra ADR-012. Se a experiência mostrar que toda unidade `contract` repete as duas exceções, a proposta de tratar código gerado no verificador é uma história futura, não esta. |
| **`google.golang.org/protobuf` declarado `wire.codec` com entrypoints explícitos.** A allowlist exige os quatro elementos (`capability.go:115-118`); `wire.codec` não é submetida à pureza transitiva (`external.go:110-117`), então a entrada vale pelos subpacotes que o gerado e o codec importam. | `entrypoints: ["."]`: autoriza só a raiz do módulo, que ninguém importa; o gerado importa subpacotes. Declarar `pure`: reprovaria por `DMPF-E002`, porque o fechamento do runtime Protobuf alcança `reflect`. |
| **`bounded_context: "kernel"` e `public_integration_surface: true`.** A lib é infraestrutura do kernel, como `domain`; o contrato é superfície pública por construção (`decide.go:27-29`), e a declaração explícita torna o manifesto honesto. O bounded context `orders` do exemplo é do contrato, não do módulo Go. | `bounded_context: "orders"`: confundiria o exemplo com o kernel e faria `provider` de outro contexto depender de C2 pela superfície pública — que já é verdadeira. |
| **Codec com `Envelope` tipado e `Payload []byte`.** O tipo de mão expõe os quinze atributos com presença modelada no tipo (ponteiro = condicional) e mantém os bytes do payload opacos; `Pack` é o único caminho de mensagem para bytes. Isso torna impossível, pela assinatura, calcular o hash de estrutura desserializada. | Expor `*anypb.Any` diretamente: empurra a validação de `ENV-16` para cada chamador. Aceitar `proto.Message` em `Sum`: viola `ENV-18` por construção. |
| **`Pack` serializa com `Deterministic: true`.** Elimina a única fonte de não-determinismo do `proto.Marshal` (ordem de `map`), o que torna o oráculo 3 (identidade de bytes) reproduzível dentro da stack Go, ainda que §8.3 não o exija entre produtores independentes. | `proto.Marshal` puro: correto, mas deixa o caso de identidade de bytes ao acaso. |
| **Fixture carrega `payload_bytes_hex` e `payload_hash` por caso.** `FIX-05` fixa o seccionamento e §8.2 o conteúdo; o hash declarado só é oráculo se os bytes sobre os quais foi calculado também forem literais no arquivo — senão o hash passaria a depender do serializador da stack que lê. Os bytes ficam na seção de casos, ao lado do payload campo a campo. | Hash sem bytes: o teste teria de reserializar para conferir, que é o que `ENV-18` proíbe. Bytes em arquivo binário separado: perde a legibilidade e a diffabilidade de `FIX-08`. |
| **Caso `non-canonical-field-order` como prova de `ENV-18`.** Bytes válidos em ordem de campo decrescente decodificam no mesmo valor, mas a reserialização em Go emite ordem crescente; o hash dos bytes transportados difere do hash da reserialização. É um teste que falha se alguém trocar `Sum(bytes)` por `Sum(marshal(unmarshal(bytes)))`. | Testar `ENV-18` só por assinatura: prova que a API não aceita mensagem, não prova que a implementação não reserializa por dentro. |
| **Gate em bash (`tools/buf-gate.sh`) com autoteste.** Segue `tools/dmpf-gate-check.sh`: prova negativa versionada, sem framework. | Gate em Go dentro da lib: introduziria uma unidade `app` numa lib `contract`. Gate no verificador `conformance`: outro objeto (grafo de imports), outra história. |
| **`buf-*` como targets Nx do projeto `contracts`, com `inputs` em `contracts/**`.** Mudança só em `.proto` torna a lib afetada e dispara os gates pelo mesmo `nx affected` do resto do CI; nenhum passo do `ci.yml` precisa de lista fixa. | Passo do CI chamando `tools/buf-gate.sh` direto, sem Nx: perde `affected` e obriga a decidir na mão quando rodar. |
| **`CODEOWNERS` no arquivo que a forge lê.** `REP-03` quer owner efetivo, e o Gitea não lê um `contracts/CODEOWNERS` aninhado; o caminho lido e o handle da equipe são confirmados no plano contra a documentação e a API do Gitea. | Criar `contracts/CODEOWNERS` como em §7.1: satisfaz a figura, não a regra — ninguém seria acionado na revisão. |
| **ADR-033 na finalização.** Registra, num só documento, as adaptações da fase monorepo: raiz do `gen/` dentro da lib, marca de `BUF-08` como tag anotada, exceções nominais para imports do gerado e pré-requisito de segunda pessoa para a marca. | Um ADR por adaptação: quatro documentos para uma decisão coesa. |

## Regras relacionadas

- `docs/dmpf/cloudevents-protobuf-buf.md` — norma de origem; esta spec a
  implementa, não a redefine.
- `docs/dmpf/testes-interop.md` §5 (`FIX-*`) e §6 (`ORA-08`) — formato da
  fixture e casos discriminatórios.
- `docs/guides/dmpf-manifesto.md` — `external[]` e `exceptions[]`.
- `.claude/rules/process-enforcement.md` — cadeia Go de sete passos.
- SPEC-MQA5HAXF — dependência: convenção de módulo, tags, BOM.
- SPEC-WTAXFV8B — o verificador sob o qual a lib nasce; `KRN-02` já entregue.
- SPEC-YRJRADY9 — guarda-chuva; `KRN-06`, `KRN-07` (consumidores do tipo de
  wire e do hash), `KRN-10` (binding por protocolo) e `KRN-11` (lado TypeScript
  da fixture) dependem desta história.

## Verificação e testes

### Critérios de aceite

- [x] `contracts/buf.yaml` é v2, lista `proto` explicitamente, usa `STANDARD` com
  `enum_zero_value_suffix: _UNSPECIFIED`, `FILE` e `deps: []`; `tools/buf.sh
  lint contracts` e `tools/buf.sh format --diff --exit-code contracts` saem com 0.
- [x] O envelope carrega os quinze atributos de `ENV-08` com a presença
  declarada: teste de `Encode`/`Decode` para cada obrigatório ausente devolve
  `ErrMissingAttribute` com o nome; para cada condicional presente e vazio,
  `ErrEmptyConditional`; para cada condicional ausente, sucesso sem entrada no
  mapa.
- [x] A modalidade do `data` é `proto_data`: `Decode` de `CloudEvent` com
  `binary_data` e com `text_data` devolve `ErrModality`; `Encode` nunca produz
  outra modalidade.
- [x] `datacontenttype` é `application/protobuf` e o *type URL* do `Any` é
  literalmente igual a `dataschema`: um único caractere de diferença devolve
  `ErrSchemaMismatch`; `type` em `v2` com `dataschema` em `v1` devolve
  `ErrMajorMismatch`; `dataschema` e `type` de majors iguais com strings
  distintas passam (b).
- [x] `payloadhash.Sum` devolve 64 caracteres em `[0-9a-f]`; para o mesmo
  `Payload`, variar cada um dos quinze atributos do envelope não altera o valor
  (teste parametrizado por atributo).
- [x] Existe teste que falha se o hash for recomputado a partir de estrutura
  desserializada: caso (8) da fixture — `Sum(bytes) == payload_hash` e
  `Sum(proto.Marshal(decodificado)) != payload_hash`.
- [x] O valor calculado em Go coincide com o hash declarado em cada caso da
  golden fixture, e cada caso decodifica campo a campo no valor declarado,
  inclusive `total_cents = 9007199254740993` e o campo desconhecido preservado
  (`PTB-10`).
- [x] A fixture tem ao menos um caso presente e um ausente para cada um de
  `aggregateversion`, `tenantid` e `tracestate` (§8.2), o enum em `UNSPECIFIED`, em
  valor conhecido e em valor desconhecido, e um caso de campo desconhecido
  (§8.2; `ORA-08`); todo escalar é string; `format_version` desconhecida faz o
  carregador falhar.
- [x] `buf lint` roda em `STANDARD` e `buf breaking` em `FILE` no CI; nenhuma
  regra é desabilitada por diretório; com marca de baseline presente, `NX_BASE`
  vazio ou irresolvível reprova (provado pelo autoteste); com marca ausente em
  módulo já presente em `NX_BASE`, reprova; em módulo novo, o gate registra
  `sem baseline` e segue.
- [x] Duas gerações consecutivas em ambiente limpo produzem artefatos idênticos
  byte a byte, e a edição de um byte em `gen/go/` faz `generate-check` reprovar
  (autoteste).
- [x] `buf-pins` reprova `latest`, faixa e divergência entre a versão do plugin e
  a do runtime no `go.mod` (autoteste); passa na configuração versionada.
- [x] Nenhum artefato desta entrega declara ou sugere exactly-once fim a fim: a
  varredura de `lint` sai com 0 sobre `contracts/` e a lib.
- [ ] O verificador de conformidade não reporta aresta de `contract` para
  `domain` nem para qualquer outro bloco: `go run
  ./libs/backend/go/conformance/cmd/conformance --root . --base
  origin/develop` sai com 0 com as três unidades no manifesto e no baseline.
- [x] Teste negativo de matriz: um pacote de teste temporário em
  `domain` importando `contracts/envelope` reprova com `DMPF-D001`
  (célula 6), executado como caso do autoteste do gate ou como teste do
  verificador, e removido em seguida.
- [x] Cadeia Go verde na lib: `pnpm nx run-many -t fmt-check,vet,lint,build,test,test-race,govulncheck -p contracts`.
- [x] `pnpm biome ci .` e `pnpm nx affected -t lint,typecheck,test,build
  --exclude=@mateusmacedo/dmpf-source` verdes.
- [x] `tools/dmpf-baseline/units-baseline.json` atualizado em commit próprio;
  `bash tools/dmpf-gate-check.sh` continua verde (o módulo novo não é `domain`
  e é reportado fora do alcance do depguard, sem reprovar).
- [ ] `CODEOWNERS` lido pela forge tem owner de equipe para `orders`; o handle é
  uma equipe existente em `mateusmacedo`.
- [x] `docs/guides/dmpf-manifesto.md`, `docs/nx-reference/tasks.md` e
  `AGENTS.md` atualizados; ADR-033 criado na finalização.

### Cenários de teste

1. **Round-trip feliz.** Caso (1) da fixture → `Encode` → `proto.Marshal` →
   `proto.Unmarshal` → `Decode` → `Envelope` igual à de entrada, `Payload`
   byte-idêntico, `Sum(Payload)` igual ao declarado.
2. **Condicionais ausentes.** Caso (2) → `Encode` não cria chave no mapa para
   `aggregateversion`, `tenantid`, `tracestate`; `Decode` devolve ponteiros
   nulos.
3. **Condicional vazio.** `tenantid = ""` presente → `Decode` devolve
   `ErrEmptyConditional("tenantid")` (`ENV-12`).
4. **Obrigatório ausente.** Para cada um dos doze obrigatórios, remover e
   decodificar → `ErrMissingAttribute(nome)`; doze subtestes.
5. **Modalidade errada.** `CloudEvent` com `binary_data` → `ErrModality`;
   com `text_data` → `ErrModality`; sem `data` → `ErrModality`.
6. **Type URL divergente.** `dataschema =
   "type.googleapis.com/company.orders.event.v1.OrderPlaced"`, `any.type_url`
   com sufixo `OrderPlacedX` → `ErrSchemaMismatch`.
7. **Major divergente.** `type = "com.company.orders.order-placed.v2"` com
   `dataschema` em `v1` → `ErrMajorMismatch`; ambos `v1` com strings distintas →
   sucesso.
8. **Tipo de atributo errado.** `time` carregado como `ce_string` →
   `ErrAttributeType("time")`; `aggregateversion` como `ce_string` →
   `ErrAttributeType("aggregateversion")`.
9. **Hash indiferente ao envelope.** Fixar `Payload`; para cada um dos quinze
   atributos, trocar o valor e recomputar `Sum(Payload)` → igual em todos.
10. **Hash sobre bytes, não estrutura.** Caso (8) → `Sum(bytes) ==
    payload_hash` e `Sum(proto.Marshal(proto.Unmarshal(bytes))) !=
    payload_hash`. *É o vetor de `ENV-18`.*
11. **Campo desconhecido preservado.** Caso (6) → `proto.Unmarshal` mantém o
    campo 7 em `unknownFields`; `Sum(bytes)` igual ao declarado.
12. **Precisão de 64 bits.** Caso (4) → `total_cents` decodificado igual a
    `9007199254740993`; o carregador da fixture lê a string sem passar por
    `float64`.
13. **Enum desconhecido.** Caso (7) → `channel` decodificado com valor `99`,
    sem erro; `String()` não é chamado como oráculo.
14. **Versão de formato desconhecida.** Fixture com `format_version: "2"` →
    carregador falha antes de qualquer caso (`FIX-09`).
15. **Lint reprova enum sem `_UNSPECIFIED`.** Autoteste: `.proto` temporário com
    `enum X { A = 0; }` → `buf lint` reprova.
16. **Breaking sem baseline declarado.** Autoteste: `NX_BASE` vazio com marca
    presente → reprova com mensagem "baseline não declarado".
17. **Breaking com baseline irresolvível.** Autoteste: `NX_BASE=refs/heads/nao-existe`
    com marca presente → reprova.
18. **Marca removida de módulo com histórico.** Autoteste: módulo presente em
    `NX_BASE`, tag ausente → reprova.
19. **Autoria igual a autorização.** Autoteste: tag anotada cujo `tagger` tem o
    e-mail do autor do primeiro commit do módulo → reprova.
20. **Redeclaração de pacote publicado.** Autoteste: `company/orders/event/v1`
    movido para outro módulo em `HEAD` → reprova.
21. **Bootstrap legítimo.** Autoteste: módulo ausente em `NX_BASE`, sem tag →
    aviso `sem baseline`, código de saída 0 no subcomando `breaking`, e `lint`
    ainda obrigatório.
22. **Breaking `FILE` reprova mudança de tipo.** Autoteste: marca presente,
    `int64 total_cents` → `int32` em `HEAD` → `buf breaking` reprova
    (`PTB-07`).
23. **Drift no gerado.** Autoteste: alterar um byte em `order_placed.pb.go` →
    `generate-check` reprova nomeando o arquivo.
24. **Pin inválido.** Autoteste: `@latest` em `tools/buf.sh` → `pins` reprova;
    plugin `v1.36.0` com runtime `v1.36.1` no `go.mod` → reprova.
25. **Aresta proibida.** Pacote temporário em `domain` importando
    `.../contracts/envelope` → verificador emite `DMPF-D001`.
26. **Exactly-once no artefato.** Autoteste: linha `// exactly-once delivery`
    em arquivo temporário sob `contracts/` → `lint` reprova (P0-3).

<critical_constraints>
- [P0] O envelope é o `io.cloudevents.v1.CloudEvent` do formato Protobuf oficial.
  O perfil só ACRESCENTA obrigatoriedade: nenhum atributo é removido, nenhuma
  semântica é alterada, nenhum envelope proprietário é aceito (`ENV-07`; ADR-022).
- [P0] A modalidade do `data` é EXCLUSIVAMENTE `proto_data` com
  `google.protobuf.Any`. `binary_data` e `text_data` são recusados pelo codec —
  não são fallback nem exceção (`ENV-15`).
- [P0] O `payload_hash` é SHA-256 em hexadecimal minúsculo sobre os bytes de
  `Any.value` EXATAMENTE como transportados. A função recebe `[]byte` e não
  existe caminho que aceite estrutura desserializada; nenhum atributo do envelope
  entra no cálculo (`ENV-17`..`ENV-19`; ADR-022).
- [P0] Todo gate Buf barra o merge: nenhum é advisory, nenhum é ignorável por
  aprovação de revisor, nenhum tem bypass documentado. Baseline ausente,
  irresolvível ou corrompido em módulo com marca de baseline REPROVA (`BUF-05`,
  `BUF-08`, `BUF-12`).
- [P0] O código gerado sob `gen/go/` NUNCA é editado à mão. Divergência entre
  duas gerações consecutivas, ou entre o gerado e o versionado, REPROVA
  (`BUF-11`, `REP-02`).
- [P0] O bloco `contract` só depende de `contract`. Nenhuma unidade desta lib
  importa `domain` nem qualquer outro bloco; nenhuma unidade `domain`,
  `application` ou `port` importa esta lib (RFC §7.4, células 6, 12, 24 e 31;
  `CTR-02`).
- [P0] O bloco `contract` admite apenas as capabilities `pure` e `wire.codec`.
  Dependência externa entra pela allowlist `external[]` com os quatro elementos;
  desvio de classificação entra por exceção NOMINAL em `exceptions[]` — nunca por
  categoria, prefixo ou diretório (RFC §6.2, §6.3, §6.4).
- [P0] Nenhum artefato desta entrega — `.proto`, comentário, README, fixture,
  código, mensagem de erro — declara nem sugere exactly-once fim a fim; a
  garantia é at-least-once com efeitos idempotentes (RFC §2.3, P0-3).
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva: toda
  adaptação à fase monorepo está declarada em "Decisões técnicas" e vira ADR na
  finalização, nunca alteração silenciosa da norma.
- [P1] O `.proto` permanece neutro de linguagem: `go_package` e qualquer opção de
  stack são aplicados pelo managed mode na geração, nunca escritos nos arquivos
  do contrato da organização (`BUF-10`).
- [P1] NUNCA redeclarar em `project.json` um target que o `@nx-go/nx-go` ou os
  `targetDefaults` já forneçam com o mesmo executor (ADR-002; ADR-030).
</critical_constraints>

## Escopo fora

- **Registry de schemas em runtime.** Pendência `ADR-DMPF-N`, aberta por
  ADR-023; nenhum artefato desta história resolve `dataschema` pela rede.
- **`openapi/` e `asyncapi/`.** Registrados como diretórios vazios (`REP-06`);
  conteúdo, versionamento e gates não são normatizados por FND-05 e não são
  inventados aqui.
- **Repositório de contratos como produto separado.** Épico de ordem 3
  (`SPEC-YRJRADY9:561-563`); esta história entrega a estrutura Buf e os gates
  neste monorepo. `CONSUMERS.md` (`REP-04`) e o fluxo de depreciação
  (`PTB-13`..`PTB-16`) nascem quando houver segundo consumidor ou segunda major.
- **Lado TypeScript da fixture e pareamento cross-stack.** `KRN-11`: a fixture
  já é a fonte única; o carregador e os oráculos em TypeScript são daquela
  história.
- **Binding por protocolo.** `KRN-10`: como o `CloudEvent` viaja em Kafka, SQS,
  gRPC ou HTTP não é desta história.
- **Outbox e inbox.** `KRN-06` e `KRN-07` consomem `envelope` e `payloadhash`;
  a persistência, o mapeamento no `provider` (ADR-021) e a decisão R1–R4 da
  inbox ficam lá.
- **Segundo módulo Buf, segunda major, contrato `command`/`response`/`service`.**
  Fora: um módulo, um contrato `event`, uma major bastam para provar os gates.
- **Tratamento de código gerado no verificador.** Se a exceção nominal para
  `reflect`/`unsafe` se repetir em toda unidade `contract`, a mudança no
  verificador é proposta como história de `KRN-02`, não feita aqui.
- **`.golangci.yml` para o bloco `contract`.** O `depguard` cobre só `domain`
  por decisão de ADR-030; o gate do bloco `contract` é o verificador.
- **Criação da marca de baseline.** Exige segunda pessoa com direito de push de
  tag na forge (`BUF-08`); é ato operacional pós-merge, registrado no README e
  no ADR-033, não parte do PR.
