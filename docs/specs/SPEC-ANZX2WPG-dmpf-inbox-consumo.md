---
id: SPEC-ANZX2WPG
slug: dmpf-inbox-consumo
title: DMPF KRN-07 — Inbox e consumo: deduplicação, disposições e contenção
stage: done
priority: P0
depends_on: [SPEC-ZHE7DN1H, SPEC-WYX5GW87, SPEC-3R80KNMS]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-526
subtask_urls: []
created: 2026-09-04
---

# SPEC-ANZX2WPG: DMPF KRN-07 — Inbox e consumo: deduplicação, disposições e contenção

## Resumo

Fechar o lado do consumo do Incremento 3 ("Garantias") entregando três peças que
hoje não existem no workspace: a **porta de inbox** em `dmpf-ports` (bloco `port`),
cuja operação de registro tem semântica de `insert-if-absent` **com retorno** e
devolve a classificação da recepção como tipo fechado; a **realização Postgres**
dessa porta em `dmpf-provider-postgres`, que serializa na chave sob concorrência
e nunca sinaliza erro de constraint; e o **application service de consumo** com o
adapter que o antecede, que ramifica pelas sete disposições de FND-04 §6.4 e põe
deduplicação, efeitos locais e outbox derivada dentro de **uma** fronteira
transacional.

A entrega cria o **primeiro módulo do bloco `app`** do workspace
(`libs/backend/go/dmpf-app`). Isso não é preferência de organização: é
consequência da matriz de blocos. O adapter precisa validar o envelope e computar
o `payload_hash`, o que exige importar `contract`; e precisa invocar o application
service, o que exige importar `application`. `application → contract` é a célula
12 e `provider → application` é a célula 26 — as duas são proibidas e as duas já
têm vetor negativo rodando no CI
(`tools/dmpf-cell-check.sh:38-41`). Só a linha `app` da matriz permite as duas
arestas ao mesmo tempo (`libs/backend/go/dmpf-conformance/internal/rule/matrix.go:56-64`),
e é por isso que o adapter nasce em módulo próprio.

Como time de plataforma, queremos que a semântica oficial — **at-least-once com
efeitos efetivamente idempotentes** (P0-3, `GAR-01`) — deixe de ser prosa
normativa e passe a ser propriedade executada: que a redelivery seja absorvida
pela inbox, que a colisão de identificador seja contida em vez de aplicada, que
duas transações concorrentes sobre a mesma chave produzam exatamente um efeito, e
que o vetor V32 — verificável **somente** em runtime, nunca por revisão de código
(FND-04 §7.1) — tenha um teste que o exerça de fato.

## Contexto

- **Problema**: `KRN-06` fechou a escrita. O agregado de exemplo grava estado de
  negócio e intenção de publicar na mesma `pgx.Tx`
  (`libs/backend/go/dmpf-provider-postgres/outbox.go:33-36`), e a unicidade de
  `message_id` é imposta pelo banco
  (`libs/backend/go/dmpf-provider-postgres/schema.sql:22`). Do outro lado não há
  nada: nenhuma porta de inbox em `dmpf-ports` (o pacote declara `UnitOfWork`,
  `Repository`, `Outbox`, `Clock` e `IDGenerator`, e mais nada), nenhuma tabela de
  deduplicação no `schema.sql`, nenhum application service que ramifique por
  disposição, e nenhum módulo do bloco `app`. As duas janelas de duplicata que
  FND-04 §7.1 nomeia — entre publicar e marcar, na drenagem; entre commitar e
  confirmar, no consumo — não têm quem as absorva.
- **Impacto**: sem `KRN-07`, o resultado verificável do Incremento 3 não existe.
  A spec guarda-chuva o enuncia como "mensagem publicada atomicamente **e
  consumida com deduplicação**"
  ([SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md):405), e metade dele
  ficaria por conta. O `KRN-10` (transporte) consome esta entrega: é ele que
  liga o gesto concreto de ACK, `nack` e commit de offset aos efeitos de broker
  que a tabela de §6.4 fixa. O `KRN-08` (relay) é independente e pode correr em
  paralelo.
- **O que muda no verificador**: a linha `app` da matriz de blocos nunca foi
  exercitada por módulo real. Ela é a única linha totalmente permissiva
  (`app → domain | application | app | port | provider | contract`), e um módulo
  `app` no `develop` dá vetor positivo de módulo às seis células — hoje provadas
  só por fixture em `testdata`.

### Divergências entre o ticket e o repositório

O ticket ARQ-526 foi escrito em 30/08/2026, antes de `KRN-05` e `KRN-06` entrarem
no `develop`. Quatro pontos divergem do estado atual, e a spec adota o
repositório:

| # | O ticket diz | O repositório mostra | Decisão |
|---|--------------|----------------------|---------|
| 1 | "A taxonomia de erros que classifica retentável × não retentável [...] é consumida aqui, não definida" | FND-07 **já a fixou**: catálogo de onze categorias em `docs/dmpf/contexto-erros-seguranca.md:952-966`, e `ERR-11` fecha nominalmente o placeholder de `INB-09` (`:1815`) | Consumir a taxonomia **real** de FND-07 §5.3, e a coluna de mensageria de §6.2, em vez de declarar uma taxonomia local. O que a entrega declara é o **subconjunto realizado em Go**, não uma taxonomia nova |
| 2 | "`registrar(consumer_name, message_id, payload_hash) -> classificação`" e "`concluir(processed \| rejected)`" como duas operações independentes | Go não tem união exaustiva; `INB-05` exige que `concluir` seja obrigatória **sob R1 e somente sob R1** | `Complete` é alcançável **apenas** pelo ramo R1, por construção do tipo (ver [Decisões técnicas](#decisões-técnicas), D2). A norma é preservada; a forma é a que o compilador consegue impor |
| 3 | Não nomeia onde vive o consumer adapter | A matriz de blocos exclui `application` e `provider` como hospedeiros | Módulo novo `libs/backend/go/dmpf-app`, bloco `app`, primeiro do workspace |
| 4 | "DLQ e quarantine distintos, com mapeamento declarado (`GAR-11`)" | Não há broker no kernel — o transporte é `KRN-10` | Quarantine é **realizada** aqui, sobre Postgres, porque não depende de broker. DLQ é **declarada** no mapeamento e encaminhada ao `KRN-10`, conforme `GAR-11` admite ("um contexto pode implementar apenas DLQ"; aqui é o inverso, e a distinção é declarada) |

### Fontes normativas

| Fonte | O que fixa | Onde |
|-------|-----------|------|
| FND-04 §6.1 | Schema mínimo, `INB-01` (chave e `consumer_name` lógico), `INB-02` (dois valores terminais), `INB-03` (a inbox só contém o que commitou) | `docs/dmpf/uow-inbox-outbox.md:960-1021` |
| FND-04 §6.2 | A porta: `INB-04` (retorno, nunca erro de constraint), `INB-05` (`concluir` só sob R1), `INB-06` (serializa na chave), `INB-17` (teto de espera → R1×D3), `INB-18` (classificação sempre de transação commitada) | `:1022-1151` |
| FND-04 §6.3 | Sequência de sete passos, `INB-07` (fronteira transacional única), `INB-08` (efeito de broker depois do commit) | `:1152-1212` |
| FND-04 §6.4 | As sete disposições, `INB-09` (D3×D4 pela classificação), `INB-10` (envelope inválido não é D4), `INB-11` (precedência), `INB-12` (R3 não reemite) | `:1213-1315` |
| FND-04 §6.5 | `INB-13`: H1 (estável sob serializações equivalentes) e H2 (restrito ao conteúdo de negócio) | `:1316-1357` |
| FND-04 §6.6 | `INB-14` (`retenção_inbox ≥ janela_redelivery`), `INB-15` (zona declarada), `INB-16` (purga com evidência) | `:1358-1404` |
| FND-04 §7.1 | `GAR-01` (vedação a exactly-once), `GAR-02`, V31 e V32 e os seus modos de verificação | `:1407-1461` |
| FND-04 §7.2 | `GAR-03`, `GAR-04`, `GAR-10`: idempotência de efeito é outra camada, permanente, e é ela que sustenta V32 | `:1462-1506` |
| FND-04 §7.4 | `GAR-07`, `GAR-08` (poison message), `GAR-09`, `GAR-11` (DLQ ≠ quarantine), `GAR-12` (sinais do lado do consumo) | `:1584-1658` |
| FND-07 §5.3 | Catálogo de categorias e a retryability de cada uma | `docs/dmpf/contexto-erros-seguranca.md:952-966` |
| FND-07 §6.2 | Coluna "Mensageria — disposição sob `R1`": o mapeamento categoria → disposição, literal | `:1160-1180` |
| FND-07 `MAP-04`, `MAP-05`, `MAP-07` | Contenção de envelope está fora do eixo; R2/R3/R4 não produzem erro da taxonomia; `Conflict` e `DeadlineExceeded` resolvem por predicado | `:1173-1218` |
| ADR-022 | Fórmula do `payload_hash`, realizada em `dmpf-contracts/payloadhash` | `docs/adr/022-*`, `libs/backend/go/dmpf-contracts/payloadhash` |
| ADR-032 | O desfecho da UPR como par `(Accepted[R], *Rejection)` com tipo concreto — o precedente de forma que D2 segue | `docs/adr/032-realizacao-go-do-desfecho-da-upr.md` |
| ADR-034 | `UnitOfWork[R]` com vínculo no composition root — a fronteira em que a inbox entra | `docs/adr/034-fronteira-de-uow-em-go.md` |
| ADR-035 | Realização Postgres da outbox; serialização na escrita; o gate `dmpf-cell-check.sh` | `docs/adr/035-realizacao-postgres-da-outbox.md` |

## Requisitos

### Funcionais

#### RF-01 — Porta de inbox (`dmpf-ports`, bloco `port`)

1. O pacote `dmpfports` declara `Inbox`, com **uma** operação de entrada:
   `Register(ctx, Receipt) (Reception, error)`, onde `Receipt{Consumer,
   MessageID, MessageType, PayloadHash, ReceivedAt}`. O ticket enunciava três
   escalares (`consumer, id, hash`); a forma mudou porque o schema de RF-02.1
   exige `message_type` e `received_at` na mesma linha, e a assinatura de três
   escalares não os carrega — não é retrofit, é o schema mandando na porta.
2. `Reception` é tipo concreto fechado do pacote, com quatro construtores
   **exportados** — as realizações vivem em outros módulos — e um ramificador
   **exaustivo por assinatura** (`Match`): quem o consome é obrigado, pelo
   compilador, a fornecer tratamento para R1, R2, R3 e R4 (`INB-04`, e o motivo
   de FND-04 §6.2 fazer o eixo 1 coincidir com o retorno).
3. Sob **R1**, e somente sob R1, o ramificador entrega um `Pending`, cujo método
   `Complete(ctx, Completion) error` fixa o estado terminal —
   `Completion{Status, At, LastError}`. `Status` admite exatamente dois valores:
   `StatusProcessed` e `StatusRejected` (`INB-02`). Não há caminho de tipo que
   alcance `Complete` sob R2, R3 ou R4 (`INB-05`). A compilação prova a
   inalcançabilidade fora de R1, não a chamada dentro dele: por isso `Pending`
   expõe `Completed()` e `Match` devolve `ErrPendingNotCompleted` quando o ramo
   R1 retorna sem `Complete` — registrar sem concluir é defeito ruidoso, nunca
   commit silencioso da linha provisória.
4. A porta não importa `contract` nem `provider`: os campos do `Receipt` são
   tipos de domínio ou primitivos, como `Outbox` já faz com `dmpfports.MessageID`
   (`libs/backend/go/dmpf-ports/outbox.go:26-37`). Os instantes (`ReceivedAt`,
   `Completion.At`) são autorados pelo application service a partir do `Clock`,
   nunca pelo provider (ADR-034).
5. `Register` devolve `error` **apenas** para falha técnica. Chave já presente é
   desfecho previsto e viaja em `Reception`, nunca em `error` (`INB-04`).
6. Existe um sentinela `ErrRegisterTimeout` no pacote, para o estouro do teto de
   espera de `INB-17`, e ele é distinguível por `errors.Is`.
7. Existe uma **suíte de contrato** da porta em
   `dmpf-ports/inbox_contract_test.go` (`package dmpfports_test`), no molde de
   `RunUnitOfWorkContract` (`libs/backend/go/dmpf-ports/contract_test.go:35`).
   Cada realização **duplica** as cláusulas, com a nota que explica o porquê:
   um arquivo `_test.go` nunca é importável, e o test kit exportado como pacote
   é escopo do `KRN-11` — precedente já estabelecido em
   `libs/backend/go/dmpf-application/example/memory/contract_test.go:11-13`.

#### RF-02 — Realização Postgres da porta (`dmpf-provider-postgres`, bloco `provider`)

1. `schema.sql` ganha `dmpf_inbox` com os oito campos de FND-04 §6.1:
   `consumer_name`, `message_id`, `message_type`, `payload_hash`, `received_at`,
   `processed_at`, `status` e `last_error`; com `UNIQUE (consumer_name,
   message_id)` (`INB-01`) e `CHECK (status IN ('processed','rejected'))`
   (`INB-02`). `Migrate` continua idempotente (`CREATE ... IF NOT EXISTS`).
2. `Tx.Inbox(consumer string, wait time.Duration) dmpfports.Inbox` vincula a
   porta à transação aberta, no mesmo molde de `Tx.Outbox(mapper)`
   (`libs/backend/go/dmpf-provider-postgres/outbox.go:33`); `wait` é o teto de
   `INB-17` declarado pelo chamador (`<= 0` desliga o teto). Entre `Register` e
   `Complete` a linha existe com `status` provisório dentro da transação —
   `status NOT NULL` com `CHECK` obriga um valor antes do desfecho — e `Complete`
   o sobrescreve; ninguém observa o provisório por `INB-18` (ADR-036).
3. `Register` realiza `insert-if-absent` com retorno por `INSERT ... ON CONFLICT
   DO NOTHING` seguido de leitura, e **nunca** deixa a transação abortada:
   nenhum caminho depende de erro de constraint (`INB-04`).
4. Sob concorrência na mesma chave, a segunda chamada **aguarda** o desfecho da
   primeira e recebe a classificação do desfecho **commitado** — R2 se a primeira
   commitou `processed`, R3 se commitou `rejected`, R1 se abortou (`INB-06`,
   `INB-18`).
5. A espera tem **teto declarado pelo chamador**. Ao estourá-lo, `Register`
   devolve `ErrRegisterTimeout` — e não erro terminal —, para que o application
   service o classifique como falha transitória e derive R1×D3 (`INB-17`).
6. O provider documenta, em godoc, que a propriedade de `INB-18` é entregue sob
   `READ COMMITTED`, e que isolamento superior exige outra realização — como
   FND-04 §6.2 exige nominalmente (`:1120-1128`).
7. `PurgeInbox(ctx, pool, consumer, before)` devolve **evidência**: quantas
   chaves foram removidas, de qual consumidor e até qual instante (`INB-16`), no
   molde de `PurgePublished` (`libs/backend/go/dmpf-provider-postgres/purge.go`).
8. `Quarantine` é realizada sobre Postgres: tabela `dmpf_quarantine` que preserva
   o envelope recebido **byte-idêntico** ao publicado, com o erro sanitizado e o
   motivo da contenção (`GAR-07`, `GAR-11`, `ERR-20`, `ERR-21`).
9. `InboxSignals(ctx, pool, consumer)` expõe, no mínimo: profundidade da
   quarantine, contagem de R1×D4 e contagem de R4 (`GAR-12`) — derivados da
   própria tabela por `GROUP BY reason`, sem contador em memória.

#### RF-03 — Application service de consumo (`dmpf-application`, bloco `application`)

1. O pacote `dmpfapplication` ganha `Disposition`, tipo fechado com as **sete**
   disposições de FND-04 §6.4, e `Classify(err error) Disposition`, que deriva
   D1..D4 a partir da retryability já resolvida do erro — nunca por inspeção
   ad hoc da mensagem (`INB-09`, `MAP-07`).
2. O pacote declara a **realização Go do subconjunto consumido** da taxonomia de
   FND-07 §5.3: as categorias com célula de mensageria e a retryability de cada
   uma, com `Conflict`, `DeadlineExceeded` e `Unexpected` resolvendo por
   predicado declarado e **fechando em não retentável** quando o predicado não é
   decidível (`ERR-10`, `ERR-11`, `ERR-24`).
3. O caso de uso de consumo do exemplo percorre a sequência de sete passos de
   FND-04 §6.3, com o passo 5 como **único** ponto de ramificação, e as sete
   disposições exaustivamente cobertas (`INB-11`).
4. Em R1×D1: aplica efeitos locais, grava a outbox derivada e chama
   `Complete(StatusProcessed)` — os três na **mesma** transação (`INB-07`).
5. Em R1×D2: chama `Complete(StatusRejected)` e, quando o contrato o previr,
   grava o rejection event na outbox derivada, na mesma transação.
6. Em R1×D3 e R1×D4: abandona a UoW. **Nada** é gravado na inbox, porque a
   transação não commita.
7. Em R2, R3 e R4: curto-circuita; nenhuma escrita na inbox. Em R3, **nenhum**
   rejection event é reemitido (`INB-12`).
8. O efeito local do caso de uso de consumo é idempotente **por chave natural
   permanente**, independentemente da inbox (`GAR-03`, `GAR-04`, `GAR-10`).
9. `Resources` do exemplo ganha `Inbox`; a realização em memória
   (`dmpf-application/example/memory`) a satisfaz, para que o `application`
   continue testável sem banco.

#### RF-04 — Consumer adapter (`dmpf-app`, bloco `app`, módulo novo)

1. Módulo `libs/backend/go/dmpf-app`, projeto Nx `dmpf-app-go`, com as três tags
   3D (`type:lib`, `scope:backend`, `stack:go`), `package.json` com
   `private: true` e `dmpf-units.json` declarando as unidades do bloco `app`.
2. O adapter recebe os bytes brutos da entrega (`Delivery{Raw, Attempt}`) e
   valida o envelope **antes** da UoW por `envelope.Unmarshal(Raw)` — função
   acrescentada a `dmpf-contracts` nesta entrega, simétrica a `Pack`, para que a
   decodificação de wire fique no bloco `contract` e o `app` não importe
   protobuf. Envelope inválido **não recebe classificação de recepção**, não
   entra em nenhuma das sete disposições e vai direto para a contenção
   (`INB-10`, `MAP-04`). Toda contenção grava `Raw`, nunca um re-marshal
   (`GAR-07`). O exemplo decodifica o `Any` em `OrderPlaced` por
   `envelope.Unpack(env, msg)`, também de `contract`.
3. O adapter computa o `payload_hash` sobre os bytes do `Any.value` **exatamente
   como transportados**, por `payloadhash.Sum`
   (`libs/backend/go/dmpf-contracts/payloadhash`), sem desserializar e sem
   resserializar (H1, H2, ADR-022, `ENV-18`).
4. O adapter aplica o efeito de broker **sempre depois** do retorno da transação
   (`INB-08`). Nesta entrega o "broker" é uma porta `Acknowledger` de duas
   operações, e o gesto concreto por transporte fica para o `KRN-10`.
5. O adapter respeita o **limite de tentativas** e retira do fluxo a mensagem que
   o esgota, para que uma poison message não bloqueie partição nem grupo FIFO
   (`GAR-08`).
6. O adapter declara o **mapeamento entre situação de contenção e mecanismo** —
   quarantine ou DLQ — em tabela, no README do módulo e no corpo do PR
   (`GAR-11`).

#### RF-05 — Documentação e rastreabilidade

1. ADR novo (a partir de `036`) registrando a forma Go da classificação de
   recepção e a fronteira `Pending` — a decisão que D2 fixa.
2. `README.md` do módulo `dmpf-app`, `README.md` do `dmpf-provider-postgres`
   atualizado com a inbox, e `AGENTS.md` com o inventário do novo módulo.
3. `tools/dmpf-baseline/units-baseline.json` atualizado com as unidades novas,
   via `--write-baseline`, em **commit normativo separado**.
4. **Nenhum** artefato desta entrega declara ou sugere exactly-once fim a fim
   (`GAR-01`, V31).

### Não-funcionais

| ID | Requisito | Verificação |
|----|-----------|-------------|
| RNF-01 | A cadeia Go passa: `fmt-check`, `vet`, `lint`, `build`, `test`, `test-race`, `govulncheck` | CI |
| RNF-02 | `pnpm biome ci .` e `pnpm nx affected -t lint,typecheck,test,build` verdes | CI |
| RNF-03 | O verificador `dmpf-conformance` aprova o grafo com o módulo `app` novo, e o baseline reflete as unidades criadas | CI, job `main` |
| RNF-04 | `tools/dmpf-gate-check.sh` continua provando os vetores negativos por bloco; o bloco `app` **não** ganha regra `depguard` própria, porque a sua linha na matriz é totalmente permissiva | CI |
| RNF-05 | Os testes que exigem banco levam a build tag `integration`, rodam só no `test-race`, com `cache: false` no Nx e `-count=1` no `go test` — o padrão que `KRN-06` fixou | `project.json` |
| RNF-06 | V32 é exercido em runtime, com reentrega deliberada. Revisão de código **não** o satisfaz (FND-04 §7.1) | Teste de integração |
| RNF-07 | Nenhum dado de negócio, credencial ou segredo em `last_error` ou na quarantine (`ERR-20`, `ERR-21`, `OBX-02`, `OBX-03`) | Teste + review |

## Camadas afetadas

| Bloco | Módulo | Natureza da mudança |
|-------|--------|---------------------|
| `port` | `libs/backend/go/dmpf-ports` | Aditiva: `Inbox`, `Reception`, `Pending`, `Status`, `ErrRegisterTimeout`, suíte de contrato |
| `application` | `libs/backend/go/dmpf-application` | Aditiva: `Disposition`, `Classify`, taxonomia consumida, caso de uso de consumo no exemplo, `Inbox` na realização em memória |
| `provider` | `libs/backend/go/dmpf-provider-postgres` | Aditiva: `dmpf_inbox`, `dmpf_quarantine`, `Tx.Inbox`, `PurgeInbox`, `Quarantine`, `InboxSignals` |
| `app` | `libs/backend/go/dmpf-app` | **Módulo novo** — primeira unidade do bloco `app` no workspace |
| `contract` | `libs/backend/go/dmpf-contracts` | Sem mudança de código; consumido pelo adapter |
| Workspace | `go.work`, `tools/dmpf-baseline`, `AGENTS.md`, `docs/adr` | Registro do módulo e das unidades novas |

## Localização de código

```text
contracts/proto/company/reservations/event/v1/reservation_confirmed.proto  # NOVO
contracts/fixtures/reservations/event/v1/reservation-confirmed.golden      # NOVO
libs/backend/go/
├── dmpf-contracts/
│   ├── gen/go/company/reservations/event/v1/                              # GERADO
│   ├── golden/reservation_confirmed_fixture_test.go                       # NOVO
│   └── envelope/envelope.go            # ALTERADO — Unmarshal(raw), Unpack(env, msg)
├── dmpf-domain/example/reservations/   # NOVO — Reservation, Reserve, ReservationConfirmed
├── dmpf-ports/
│   ├── inbox.go                        # NOVO — Receipt, Completion, Status, Pending,
│   │                                   #   Reception + Match, Inbox, ErrRegisterTimeout,
│   │                                   #   ErrPendingNotCompleted
│   ├── containment.go                  # NOVO — Reason, Contained, Containment
│   ├── acknowledger.go                 # NOVO — Acknowledger{Ack, Release}
│   └── inbox_contract_test.go          # NOVO — 7 cláusulas (duplicadas pelas realizações)
├── dmpf-application/
│   ├── disposition.go                  # NOVO — Disposition, Classify
│   ├── failure.go                      # NOVO — Category, Failure (subconjunto de FND-07 §5.3)
│   ├── example/memory/inbox.go         # NOVO — realização em memória da porta
│   └── example/reservations/           # NOVO — reservationsapp: Resources, Service, Consume
├── dmpf-provider-postgres/
│   ├── schema.sql                      # ALTERADO — dmpf_inbox, dmpf_quarantine, dmpf_example_reservations
│   ├── inbox.go                        # NOVO — Tx.Inbox(consumer, wait), Register, Complete
│   ├── inbox_concurrency_test.go       # NOVO — cenários 8–11 (spike de lock_timeout)
│   ├── quarantine.go                   # NOVO — NewQuarantine(pool)
│   ├── signals.go                      # NOVO — InboxSignals (GAR-12)
│   ├── purge.go                        # ALTERADO — PurgeInbox com evidência
│   └── example/reservations/           # NOVO — reservationspg: repositório (chave natural) + Mapper
└── dmpf-app/                           # MÓDULO NOVO — bloco app
    ├── go.mod, project.json, package.json, dmpf-units.json, README.md, doc.go
    ├── consumer.go                     # adapter: Unmarshal, hash, Handler, efeitos
    ├── containment.go                  # ContainmentMap, MechanismFor (GAR-11)
    └── example/reservations/           # composition root do consumidor + e2e (V32)
```

A porta `Acknowledger` ficou em `dmpf-ports`, não no `app` como o rascunho
previa: o KRN-10 a realizará a partir de `provider` ou `app`, e `provider → app`
é célula proibida. O consumidor de exemplo vive em `example/reservations`, não
em `example/orders` — consumidor e produtor são papéis distintos (D6).

## Design

### Arquitetura

A entrega tem quatro peças e uma única fronteira transacional. O que decide o
desenho é a matriz de blocos: o `payload_hash` e a validação de envelope são
`contract`, o efeito e a decisão são `application`, o banco é `provider`, e só o
`app` pode ver os três.

```text
          ┌──────────────────── bloco app ────────────────────┐
mensagem  │ 1. envelope.Decode + Validate     ← contract      │
─────────►│    inválido ──────────────────────────────────────┼──► quarantine
          │ 2. payloadhash.Sum(Any.value)     ← contract      │    (sem UoW)
          │ 3. invoca o application service   ← application   │
          └───────────────────────┬───────────────────────────┘
                                  │
          ┌───────────────────────▼──── bloco application ────┐
          │ 4. UoW.Within(...)                                │
          │ 5. Register(consumer, id, hash) → Reception       │
          │ 6. ramifica: R1{D1,D2,D3,D4} | R2 | R3 | R4       │
          │    R1×D1 → efeitos + outbox + Complete(processed) │  MESMA
          │    R1×D2 → Complete(rejected) [+ rejection event] │  transação
          │    R1×D3 → abandona   R1×D4 → abandona            │
          │    R2/R3/R4 → curto-circuita, nada escreve        │
          └───────────────────────┬───────────────────────────┘
                                  │ commit ou rollback
          ┌───────────────────────▼──── bloco app ────────────┐
          │ 7. efeito de broker, SEMPRE depois do passo 6     │
          └───────────────────────────────────────────────────┘
```

O passo 7 nunca antecede o 6. Confirmar antes do commit troca duplicidade — que o
sistema absorve pela inbox — por perda, que ele não absorve (`INB-08`).

### A classificação como tipo

Go não tem união exaustiva. O que ele tem, e que ADR-032 já explorou para o
desfecho da UPR, é tipo concreto com construtores fechados. A forma escolhida
soma a isso um ramificador que **exige as quatro funções**, de modo que
acrescentar uma quinta classificação — se a norma um dia a acrescentar — quebra a
compilação de todo consumidor, que é exatamente a propriedade que FND-04 §6.2
pede ("um consumidor que não trate uma das classificações é detectável pela
própria assinatura").

```go
// Reception is the eixo 1 of FND-04 §6.4 as a closed type: the four
// constructors are unexported, so the four cases are the only ones there are.
type Reception struct {
    kind    receptionKind
    pending Pending // non-nil only under R1
}

// Match forces every caller to exhaust the four classifications, and hands the
// Pending only on the R1 branch: Complete is unreachable under R2, R3 and R4
// (INB-05), which the type makes true rather than the reviewer.
func (r Reception) Match(
    first     func(Pending) error,
    processed func() error,
    rejected  func() error,
    collision func() error,
) error
```

### Schema mínimo — `dmpf_inbox`

```sql
CREATE TABLE IF NOT EXISTS dmpf_inbox (
  consumer_name text   NOT NULL,
  message_id    text   NOT NULL,
  message_type  text   NOT NULL,
  payload_hash  text   NOT NULL,
  received_at   bigint NOT NULL,
  processed_at  bigint NOT NULL,
  status        text   NOT NULL,
  last_error    text,
  CONSTRAINT dmpf_inbox_key UNIQUE (consumer_name, message_id),
  CONSTRAINT dmpf_inbox_status_check CHECK (status IN ('processed', 'rejected'))
);

CREATE INDEX IF NOT EXISTS dmpf_inbox_retention_idx
  ON dmpf_inbox (consumer_name, processed_at);
```

Não há `processing`, e não há coluna nula de estado. `INB-02` remove o terceiro
valor por razão estrutural, não por preferência: sob transação única, ou tudo
commita e o registro nasce terminal, ou nada commita e não há registro. O
`CHECK` de dois valores é o que torna `INB-03` — "a inbox só contém mensagens
cujo processamento commitou" — uma propriedade do banco, e não uma promessa do
código.

O índice é `(consumer_name, processed_at)` porque a única varredura prevista é a
purga por retenção, sempre escopada a um consumidor (`INB-14`, `INB-16`).

### Pseudocódigo — `Register`

```go
func (i txInbox) Register(ctx context.Context, id MessageID, hash string) (Reception, error) {
    ctx, cancel := context.WithTimeout(ctx, i.wait) // teto de INB-17
    defer cancel()

    // ON CONFLICT DO NOTHING não sinaliza erro e mantém a transação viva
    // (INB-04). Sob concorrência ele AGUARDA o desfecho da inserção especulativa
    // concorrente, que é a serialização na chave que INB-06 exige.
    tag, err := i.tx.conn.Exec(ctx, insertInbox, i.consumer, string(id), hash, ...)
    if isTimeout(err) {
        return Reception{}, ErrRegisterTimeout // → o service deriva R1×D3
    }
    if err != nil {
        return Reception{}, err
    }
    if tag.RowsAffected() == 1 {
        return firstReception(pendingRow{...}), nil // R1
    }

    // A linha existe e commitou (INB-18): a espera acima garante que não estamos
    // lendo escrita não commitada.
    var storedHash, status string
    if err := i.tx.conn.QueryRow(ctx, selectInbox, i.consumer, string(id)).
        Scan(&storedHash, &status); err != nil {
        return Reception{}, err
    }
    switch {
    case storedHash != hash:      return collisionReception(), nil  // R4
    case status == statusProcessed: return processedReception(), nil // R2
    default:                       return rejectedReception(), nil  // R3
    }
}
```

O ramo `RowsAffected() == 0` com linha ausente **não existe** sob a serialização
de `INB-06`: se a concorrente abortou, o `INSERT` insere e o retorno é 1. Ainda
assim, `sql.ErrNoRows` na leitura é tratado como falha técnica, e não silenciado —
porque a alternativa seria devolver uma classificação inventada.

### Onde cada regra é provada

| Regra | Prova |
|-------|-------|
| `INB-01` | Constraint `dmpf_inbox_key`; teste de chave duplicada entre consumidores distintos, que **não** deduplica |
| `INB-02`, `INB-03` | `CHECK` do schema; teste que tenta gravar terceiro valor e falha |
| `INB-04` | Teste que registra chave já presente e, **na mesma transação**, executa outro comando com sucesso |
| `INB-05` | Compilação: `Complete` só é alcançável pelo argumento `first` de `Match` |
| `INB-06`, `INB-18` | Teste de concorrência com duas goroutines e barreira, nos três desfechos (commit processed, commit rejected, rollback) |
| `INB-07` | Teste com commit forçado a falhar: nem inbox, nem estado, nem outbox persistem |
| `INB-08` | Teste com `Acknowledger` que registra o instante e falha se o ACK anteceder o retorno de `Within` |
| `INB-09` | Tabela de `Classify` derivada de FND-07 §6.2, com vetor por categoria |
| `INB-10` | Teste com envelope inválido: nenhuma linha em `dmpf_inbox`, uma linha em `dmpf_quarantine` |
| `INB-11` | Exaustividade de `Match` + teste que R2/R3/R4 nunca avaliam o eixo 2 |
| `INB-12` | Teste de R3: nenhuma linha nova em `dmpf_outbox` |
| `INB-13` | Reuso de `payloadhash.Sum` sobre `Any.value`; teste que atributos de envelope não alteram o hash (já existe em `envelope_test.go:380`) |
| `INB-17` | Teste que segura a chave além do teto e verifica `ErrRegisterTimeout` → R1×D3 |
| `GAR-04`, `GAR-10` | Teste V32: reentrega com `message_id` **novo** e mesma chave natural — R1, efeito **não** duplica |
| `GAR-08` | Teste de poison message: esgota tentativas, sai do fluxo, as mensagens seguintes da mesma partição avançam |
| `GAR-07` | Teste que compara os bytes na quarantine com os publicados |
| `GAR-12` | Teste de `InboxSignals` com quarantine povoada |
| V31 | Varredura textual por "exactly-once" nos artefatos da entrega |

## Decisões técnicas

**D1 — O consumer adapter vive em módulo próprio, do bloco `app`.**
Não é escolha de organização. `application → contract` (célula 12) e
`provider → application` (célula 26) são proibidas e têm vetor negativo rodando
no CI. O adapter precisa das duas arestas — `contract` para validar envelope e
hashear, `application` para invocar o caso de uso —, e a linha `app` da matriz é
a única que as permite simultaneamente. Alternativa descartada: mover
`payloadhash` para `dmpf-ports`. Ela violaria a autoria do `payload_hash`, que é
de ANC-03, e desfaria a razão de `KRN-05` existir.

**D2 — A classificação é tipo concreto com ramificador exaustivo, e `Pending`
só existe sob R1.** A norma exige que `concluir` seja obrigatória sob R1 e
proibida fora dele (`INB-05`). Duas operações independentes na interface tornam
isso uma regra de revisão; o `Pending` entregue apenas no ramo `first` de `Match`
torna isso uma regra do compilador. É o mesmo movimento de ADR-032: preferir o
tipo que impede o erro ao comentário que o descreve. Vira ADR-036.

**D3 — O teto de espera é do chamador, e o estouro é sentinela, não erro
terminal.** `INB-17` fixa que o teto exista e que o estouro seja D3; o valor é de
FND-08. A realização é `SET LOCAL lock_timeout` na transação, com o `context` do
chamador como rede — a ordem inversa da que esta spec previa no rascunho. O
motivo veio da pesquisa: o handler padrão de contexto do pgx v5 **fecha a
conexão** ao expirar o `context`, o que, no cenário de `INB-17` (duplicatas
empilhando sob visibility timeout curto), descarta conexões do pool em vez de
conter o esgotamento; `lock_timeout` aborta só o statement no servidor e mantém
a conexão viva. Confirmado empiricamente: a espera da inserção especulativa do
`ON CONFLICT DO NOTHING` por transação concorrente é interrompida com SQLSTATE
`55P03` em ~319 ms para um teto de 300 ms, e a transação bloqueada
comprovadamente esperou (`inbox_concurrency_test.go`). O provider traduz `55P03`
para `dmpfports.ErrRegisterTimeout`.

**D8 — Ajustes descobertos na execução.** (a) O manifesto de `dmpf-contracts`
lista os pacotes gerados um a um; o pacote `company/reservations/event/v1`
entrou no `include` da unidade `dmpf-contracts/gen`, e o commit normativo cobre
cinco manifestos, não quatro. (b) Dois módulos com teste de banco
(`dmpf-provider-postgres-go` e `dmpf-app-go`) não podem rodar `test-race` em
paralelo contra o mesmo Postgres — cada harness faz `TRUNCATE`; o `test-race`
do `dmpf-app` declara `dependsOn` sobre o do provider e o Nx os sequencia, mesmo
com o `--parallel=3` do CI. (c) `tools/dmpf-gate-check.sh` descobria a cláusula
`package` pelo primeiro `.go` alfabético do subpacote e, em
`example/reservations`, encontrava um `_test.go` externo — corrigido para
ignorar `_test.go`. (d) `envelope.Unmarshal` e `envelope.Unpack` foram
acrescentados a `dmpf-contracts` (ver RF-04.2).

**D4 — A taxonomia consumida é a de FND-07, e a entrega realiza o subconjunto,
não uma taxonomia local.** O ticket previa declarar uma; o artefato já a fixou e
`ERR-11` fecha nominalmente o placeholder de `INB-09`. `Classify` deriva a
disposição da coluna de mensageria de FND-07 §6.2, com fail-closed em não
retentável quando o predicado não é decidível.

**D5 — Quarantine é realizada; DLQ é declarada e encaminhada.** Quarantine não
depende de broker — é retenção para inspeção, fora do fluxo — e cabe em Postgres
agora. DLQ é destino terminal **do transporte**, e o transporte é `KRN-10`.
`GAR-11` exige que os dois sejam distintos e que o mapeamento seja declarado;
esta entrega os distingue e declara o mapeamento, realizando o que não depende de
infraestrutura ausente.

**D6 — O efeito local do exemplo é convergente por chave natural.** O caso de uso
de consumo cria uma reserva cuja chave natural é o identificador do pedido —
permanente, independente do `message_id`. É isso que permite escrever o teste de
V32 que **falha** se a idempotência de efeito não existir: reentrega com
`message_id` novo é classificada R1 pela inbox, e só a chave natural impede a
duplicação (`GAR-03`, `GAR-04`).

**D7 — Sem `processing`, sem escrita em duas fases.** Consequência de `INB-02` e
da fronteira transacional única. Registrada aqui porque é a divergência declarada
que FND-04 §6.1 mantém com a Parte-1 §10.6, e quem ler o schema vai procurar o
terceiro estado.

## Regras relacionadas

`INB-01` a `INB-18`; `GAR-01`, `GAR-02`, `GAR-03`, `GAR-04`, `GAR-07`, `GAR-08`,
`GAR-09`, `GAR-10`, `GAR-11`, `GAR-12`; `ERR-08` a `ERR-11`, `ERR-20`, `ERR-21`,
`ERR-24`; `MAP-04`, `MAP-05`, `MAP-07`; `UOW-01` a `UOW-11`; `BLK-01` a `BLK-05`;
`ENV-18`; P0-2 (regra de dependência) e **P0-3** (at-least-once com efeitos
idempotentes); vetores V31 e V32 de RFC §11.

## Verificação e testes

### Critérios de aceite

Os dez primeiros são os do ticket ARQ-526, verbatim; os seguintes derivam dos
requisitos desta spec.

- [x] Mensagem já processada, reentregue com `message_id` e hash iguais, é
      classificada R2: nenhum efeito é reaplicado e o ACK é emitido.
- [x] Mensagem já rejeitada, reentregue com hash igual, é classificada R3 e
      nenhum rejection event é reemitido.
- [x] Mensagem com o mesmo `message_id` e hash divergente é classificada R4 e
      retirada do fluxo, e não tratada como redelivery legítima.
- [x] Em R1×D3 e R1×D4 nada é gravado na inbox; o estouro do teto de espera
      produz R1×D3, e não erro terminal.
- [x] Duas transações concorrentes sobre a mesma chave produzem exatamente um
      efeito aplicado, e a segunda recebe a classificação do desfecho commitado
      da primeira.
- [x] Envelope inválido não recebe classificação de recepção e é contido antes
      da UoW.
- [x] O efeito de broker ocorre depois do retorno da transação; há teste que
      falha se a confirmação anteceder o commit.
- [x] Deduplicação, efeitos locais e outbox derivada commitam juntos; um commit
      forçado a falhar comprova que nenhum dos três persistiu.
- [x] Uma poison message não bloqueia a partição nem o grupo FIFO, e a contenção
      preserva o payload byte-idêntico ao publicado.
- [x] O teste de reentrega deliberada comprova que o efeito final é o mesmo: a
      idempotência é do efeito, e a inbox não a substitui.
- [x] Nenhum artefato desta entrega declara ou sugere exactly-once fim a fim.
- [x] `Complete` é inalcançável sob R2, R3 e R4 — provado por compilação, não por
      revisão.
- [x] O mesmo `message_id` sob `consumer_name` distinto **não** deduplica: são
      duas chaves.
- [x] `Register` sobre chave já presente mantém a transação viva: um comando
      subsequente na mesma transação tem sucesso.
- [x] `PurgeInbox` devolve evidência do que removeu, de qual consumidor e até
      qual instante.
- [x] `InboxSignals` expõe profundidade da quarantine e as contagens de R1×D4 e
      R4.
- [x] O verificador `dmpf-conformance` aprova o grafo com o módulo `app`, e o
      baseline registra as unidades novas.
- [x] Cadeia Go completa verde, e `pnpm nx affected -t lint,typecheck,test,build`
      verde.

### Cenários de teste

| # | Cenário | Nível | Regra |
|---|---------|-------|-------|
| 1 | Primeira recepção, efeito aplicado, outbox derivada gravada, ACK | integração | R1×D1, `INB-07` |
| 2 | Rejeição de negócio: `rejected` na inbox, rejection event na outbox, ACK | integração | R1×D2 |
| 3 | Falha transitória: rollback, nada na inbox, sem ACK | integração | R1×D3 |
| 4 | Falha terminal: rollback, nada na inbox, contenção | integração | R1×D4 |
| 5 | Redelivery de aplicada: R2, nenhum efeito novo, ACK | integração | R2 |
| 6 | Redelivery de rejeitada: R3, **nenhuma** linha nova na outbox | integração | R3, `INB-12` |
| 7 | Mesmo `message_id`, hash divergente: R4, contenção | integração | R4 |
| 8 | Duas goroutines na mesma chave, primeira commita `processed`: a segunda recebe R2 | integração, `-race` | `INB-06`, `INB-18` |
| 9 | Idem, primeira commita `rejected`: a segunda recebe R3 | integração, `-race` | `INB-06` |
| 10 | Idem, primeira aborta: a segunda recebe R1 e aplica o efeito | integração, `-race` | `INB-06` |
| 11 | Chave segurada além do teto: `ErrRegisterTimeout` → R1×D3 | integração | `INB-17` |
| 12 | Commit forçado a falhar: nem inbox, nem estado, nem outbox | integração | `INB-07` |
| 13 | `Acknowledger` que grava o instante: falha se ACK anteceder o commit | integração | `INB-08` |
| 14 | Envelope inválido: quarantine, zero linhas na inbox | integração | `INB-10` |
| 15 | **V32** — reentrega com `message_id` novo e mesma chave natural: R1, efeito **não** duplica | integração | `GAR-04`, `GAR-10` |
| 16 | Poison message esgota tentativas; a mensagem seguinte da mesma partição avança | integração | `GAR-08` |
| 17 | Bytes na quarantine idênticos aos publicados | integração | `GAR-07` |
| 18 | `Classify` por categoria de FND-07 §6.2, incluindo os predicados de `Conflict`, `DeadlineExceeded` e `Unexpected` | unitário | `INB-09`, `MAP-07`, `ERR-11` |
| 19 | `Match` exaustivo: remover uma classificação da união quebra a compilação | compilação | `INB-11` |
| 20 | Mesmo `message_id`, consumidores distintos: dois registros, dois efeitos | integração | `INB-01` |
| 21 | Cláusulas do contrato da porta exercidas pela realização em memória e pela Postgres (duplicadas, como o precedente do workspace) | ambos | RF-01.7 |
| 22 | `PurgeInbox` com evidência; purga escopada por consumidor | integração | `INB-16` |
| 23 | V31: varredura por "exactly-once" nos artefatos | estrutural | `GAR-01` |

## Escopo fora

- **Gesto concreto de ACK, `nack`, extensão de visibilidade ou commit de offset
  por transporte** — é `KRN-10`, sob ANC-04. Aqui existe a porta `Acknowledger`
  e a **ordem** (`INB-08`), não o comando.
- **Realização de DLQ** — depende do broker; declarada no mapeamento e
  encaminhada ao `KRN-10` (`GAR-11`).
- **Relay e drenagem da outbox** — é `KRN-08`. Esta entrega grava a outbox
  derivada; não a drena.
- **Valores operacionais**: teto de espera, limite de tentativas, prazos de
  backoff e jitter, retenção da inbox, limiares e alarmes — são de FND-08, sob
  ANC-06. Esta entrega exige que existam e sejam declarados pelo chamador.
- **Nomes de métricas e instrumentação OpenTelemetry** — é `KRN-09`. `GAR-12`
  exige a **capacidade** de expor os sinais; o catálogo é de lá.
- **Canonicalização cross-stack do `payload_hash`** — é FND-05, e a verificação
  entre Go e TypeScript é FND-09. A consequência conhecida (réplicas em stacks
  diferentes podendo divergir e produzir R4 sobre redelivery legítima) está
  nomeada em FND-04 §6.5 e não é resolvida aqui.
- **Acrescentar categoria à taxonomia de erros** — é alteração de FND-07
  (`ERR-08`). Esta entrega realiza o subconjunto consumido, não estende o
  catálogo.
- **Sagas e process managers** — FND-04 §7.5, fora do incremento.
