---
id: SPEC-7PJ5WVCS
slug: dmpf-uow-inbox-outbox
title: DMPF — Unit of Work, inbox, outbox e garantias de entrega
stage: done
priority: P0
depends_on: [SPEC-8MNDEWDP]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-441
subtask_urls: []
created: 2026-08-13
---
# SPEC-7PJ5WVCS: DMPF — Unit of Work, inbox, outbox e garantias de entrega

## Resumo

Entregar o item lógico **FND-04** do épico ARQ-436
(ARQ-441): produzir a evidência «Sequências transacionais, failure modes, idempotência, relay e disposições de consumo aprovados».

O artefato é promovido sob a âncora **ANC-02** da RFC DMPF Foundation v0.1, que
nomeia FND-04 como autoridade sobre Unit of Work, inbox, outbox e relay.

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)
- **ACs do épico**: AC-07
- **Evidência §11**: Sequências transacionais, failure modes, idempotência, relay e disposições de consumo
- **Dependência satisfeita**: [SPEC-8MNDEWDP](./SPEC-8MNDEWDP-dmpf-upr-decision-mensagens.md) (FND-03) está `done`; o artefato `docs/dmpf/upr-decision-mensagens.md` é a fonte da UPR, da `Decision` e da taxonomia de mensagens que esta spec orquestra
- **Consumidor a jusante**: [SPEC-6RQBN98G](./SPEC-6RQBN98G-dmpf-testes-interop.md) (FND-09 / ARQ-446) declara dependência desta spec; o catálogo de failure modes é insumo declarado do catálogo de testes distribuídos

### Autorização: âncora ANC-02

O artefato **adiciona** à RFC pela âncora ANC-02 (RFC §12.3). Não edita a RFC e
não incrementa a versão dela. O registro da âncora vincula o escopo:

| Campo | Valor, conforme o registro de ANC-02 |
|-------|--------------------------------------|
| Assunto | Unit of Work, inbox, outbox e relay |
| Escopo permitido | Mecanismos de atomicidade, deduplicação e drenagem, respeitando a atribuição de blocos de RFC §7.5 |
| Invariantes intocáveis | RFC §7.5 (escrita é `application service`, persistência é `provider`, drenagem é `app`); célula 11 de RFC §7.4 (`application → provider` proibida); **P0-3** |
| Artefato sucessor | Spec e seção própria |
| Condição de fechamento | FND-04 concluída e revisada |
| ADR exigido | **Sim** — escolha de mecanismo de relay (polling × CDC) |

### Divergências com a descrição da ARQ-441

A descrição da issue precede a promoção da RFC a normativa e divergiu dela em
dois pontos. Onde houver conflito, prevalece a RFC:

| # | Descrição da ARQ-441 | Norma vigente | Resolução |
|---|----------------------|---------------|-----------|
| 1 | Entregável «ADR-006 a ADR-009 aceitos», com DoD exigindo status Aceito | RFC §13.1 torna o gesto *trigger-only*: a sub-spec **aciona**; a redação, a promoção para `docs/adr/` na faixa `010`–`024` e o aceite são do FND-11 (ARQ-448) | Esta spec aciona um ADR e não redige nenhum. A numeração `006`–`009` da issue é inaplicável: os arquivos `docs/adr/006`…`009` já existem e tratam de outros assuntos |
| 2 | «Fora do escopo: CDC/Debezium (não incluído no épico)» | ANC-02 registra «ADR exigido: Sim — escolha de mecanismo de relay (polling × CDC)» | A **decisão** entre polling e CDC é acionada como ADR; a **implementação** de CDC permanece fora, como a issue pede. Os dois textos convivem sem conflito real |

<constraints>
- [P0-1] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0-2] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0-3] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0-4] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
- [P0] Atribuição de blocos de RFC §7.5 é invariante: a escrita da outbox ocorre por uma porta, NUNCA tocando a tabela
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] UoW explícita** no application service, visível no código do caso de uso: recursos vinculados à transação são recebidos pelo callback, e a UoW NÃO repete automaticamente esse callback
- [ ] **[P0] Unit of Work distinta de transação local**: a transação local é o mecanismo do banco; a UoW é a fronteira de aplicação que a envolve e vincula as portas ao callback. Uma UoW tem exatamente uma transação local
- [ ] **[P0] Sequência canônica de escrita** derivada da fonte, com a divergência declarada onde a matriz de RFC §7.4 obriga: optimistic locking na persistência do agregado, e os passos 7 e 8 da Parte-1 §9.1 colapsados, do ponto de vista do application service, na entrega à porta da outbox
- [ ] **[P0] Bloco do mapeamento** `domain event → integration event` decidido e justificado — obrigação delegada por escrito pelo FND-03 §6.3 («ficam com FND-04 [...] o bloco em que o mapeamento reside»)
- [ ] **[P0] Sequência de drenagem numerada à parte** da sequência de escrita: outro bloco, outro processo, outro ciclo de vida
- [ ] **[P0] Outbox**: estado de negócio + outbox na mesma transação, com schema mínimo normatizado (identidade, roteamento, agendamento, tentativas, lease e status)
- [ ] **[P0] Inbox**: inbox + efeitos locais + outbox derivada na mesma transação de consumo, com schema mínimo normatizado, chave `(consumer_name, message_id)` e `payload_hash`
- [ ] **[P0] Status da inbox restrito a estados terminais** `{processed, rejected}`: sob transação única, `processing` é inalcançável como estado persistido, e mantê-lo pressuporia a escrita em duas fases que esta spec proíbe
- [ ] **[P0] Porta de inbox com semântica `insert-if-absent`**: a tentativa de registrar a chave **retorna** a classificação da recepção e deixa a transação viva, nunca depende de erro de constraint para decidir
- [ ] **[P0] `payload_hash` com propriedades declaradas**: estável sob serializações equivalentes entre stacks, e restrito ao conteúdo de negócio — metadados de transporte, tracing e contadores de tentativa ficam fora
- [ ] **[P0] Retenção da inbox**: invariante `retenção_inbox ≥ janela_redelivery` declarada como desigualdade conferível, purga tratada como operação, e as duas zonas de proteção do replay distinguidas — dentro da retenção a proteção é dupla (inbox e idempotência de efeito); fora dela é única (só idempotência de efeito)
- [ ] **[P0] Disposições de consumo** (`disposition`) enumeradas de forma exaustiva e **decidível**, em dois eixos com precedência declarada: classificação da recepção (primeira recepção, reentrega de aplicada, reentrega de rejeitada, colisão de identificador) e desfecho do processamento (aplicado, rejeitado por negócio, falha transitória, falha terminal), este avaliado somente sob primeira recepção
- [ ] **[P0] Relay**: claim por lease; sem lock de banco durante I/O no broker; capacidades operacionais obrigatórias do drenador declaradas
- [ ] **[P0] Transição final condicional ao claim corrente**: um worker só transiciona o registro se `locked_by` ainda for o seu claim, e `locked_by` identifica a **execução do claim**, não o processo — sem isso, um claimant expirado sobrescreve o estado de um registro já publicado
- [ ] **[P0] Idempotência de negócio como camada distinta** da deduplicação por inbox: escopo na chave natural da operação no domínio, vida permanente, e declaração explícita de que a inbox não a substitui — Parte-1 §10.1 lista as duas como obrigações separadas
- [ ] **[P0] Retry, DLQ, quarantine e replay** especificados como operação, com proteção contra duplicidade no replay e vedação a poison message que bloqueie partição ou grupo FIFO
- [ ] **[P0] Atribuição de blocos** de RFC §7.5 declarada: escrita no `application service` por porta, persistência no `provider`, drenagem em `app` dedicado
- [ ] **[P1] Sagas e process managers**: política referenciada, com commands por outbox, eventos por inbox e compensação como ação de negócio explícita — sem implementação
- [ ] **[P0] Diagramas de sequência** dos fluxos de produção, de consumo e dos failure modes críticos, em Mermaid, declarados como derivados do texto normativo
- [ ] **[P1] Queries fora da UoW de escrita**: CQRS lógico admitido; separação física de bancos é decisão de contexto, não exigência da fundação

### Não-funcionais

- [ ] **[P0] Atomicidade sem transação distribuída**: as garantias são obtidas com transação local + outbox, nunca com two-phase commit sobre banco e broker
- [ ] **[P0] Ausência de lock durante I/O**: nenhuma sequência mantém transação de banco aberta enquanto aguarda o broker
- [ ] **[P0] Desfecho declarado por failure mode**, em três categorias: **recuperação automática** (o sistema volta sozinho ao estado correto e o efeito pretendido ocorre), **contenção automática** (o sistema isola sozinho e sem perda, mas o efeito pretendido NÃO ocorreu) e **reparação assistida** (exige decisão humana). Todo failure mode declara um desfecho automático — recuperação ou contenção —, e a contenção nomeia o que ficou pendente e quem decide
- [ ] **[P0] Monotonicidade sob ANC-02**: o artefato detalha e restringe as invariantes da âncora, nunca as relaxa, revoga ou reinterpreta; onde divergir da RFC, prevalece a RFC
- [ ] **[P0] Rastreabilidade aos vetores**: os cenários referenciam V31 (vedação a exactly-once E2E) e V32 (efeito idempotente sob redelivery) de RFC §11
- [ ] **[P0] Neutralidade de vendor**: o modelo é implementável com PostgreSQL e Kafka ou SNS/SQS como baseline, sem depender de recurso exclusivo de um banco ou broker
- [ ] **[P1] Convergência**: duplicata reprocessada converge ao mesmo estado final observável
- [ ] **[P0] Observabilidade do drenador como capacidade**: o drenador **expõe** os sinais `pending`, `lag`, `attempts` e `failures` — requisito do mecanismo, ancorado em Parte-1 §10.4. Os nomes das métricas, limiares, alarmes, catálogo por componente e runbook de DLQ/replay são de FND-08 (ANC-06) e aparecem aqui apenas como fronteira `encaminhado`

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [ ] | Permanece sem I/O; consome o resultado da UPR definida em FND-03 |
| Application service (UoW, orquestração) | [x] | Núcleo desta spec: define a fronteira transacional explícita e a escrita da outbox por porta |
| Port / Provider (adapters, drivers) | [x] | Define o contrato de relay e atribui ao provider a tabela, o claim e o backoff |
| App (drenagem, composition root) | [x] | RFC §7.5 atribui a drenagem a um processo próprio; esta spec normatiza o seu comportamento |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [ ] | O envelope publicado é definido em FND-05 |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | Define ACK após commit, redelivery e disposições de consumo; convenções por transporte são de FND-06 |
| Observabilidade e operação | [x] | Define DLQ, quarantine e replay como mecanismo, e a capacidade de observação do drenador. O catálogo de métricas e o runbook são de FND-08 |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fonte conceitual | `plans/references/Parte-1-conceitual.md` §§9–10 |
| Autorização e invariantes | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §7.5 (blocos do outbox), §12.3 (ANC-02), §13.3 (ADR de relay encaminhado a FND-04), §14.4 (sucessão) |
| Continuidade | `docs/dmpf/upr-decision-mensagens.md` (FND-03) — UPR, `Decision` e taxonomia que a UoW orquestra |
| Obrigação delegada | `docs/dmpf/upr-decision-mensagens.md` §6.3 — «ficam com FND-04 [...] o bloco em que o mapeamento reside» |
| Fronteiras a respeitar | `docs/specs/SPEC-E15TBHCD-dmpf-resiliencia-observabilidade.md` (FND-08, catálogo de métricas e runbook); `docs/specs/SPEC-7H08RZDG-dmpf-cloudevents-protobuf-buf.md` (FND-05, serialização e envelope) |
| Artefatos auxiliares (C4) | [`SPEC-7PJ5WVCS/`](./SPEC-7PJ5WVCS/) — diagramas C4 Mermaid derivados (Context, Container, Component, Dynamic); se divergirem do texto, prevalece o texto |
| Draft de trabalho | `plans/references/` — local, fora do versionamento |
| Promoção (ao ser aprovado) | `docs/dmpf/uow-inbox-outbox.md` — versionado e revisável por PR |

## Design

### Atribuição de blocos (RFC §7.5)

O padrão outbox tem três responsabilidades e cada uma pertence a um bloco
distinto. A atribuição é invariante da ANC-02, não escolha desta spec:

| Responsabilidade | Bloco | Consequência normativa |
|------------------|-------|------------------------|
| Escrita na mesma transação do estado | `application service` | Grava **por uma porta**; a célula 11 (`application → provider`) permanece proibida e a outbox não é exceção |
| Persistência da tabela e do claim | `provider` | Tabela, `SKIP LOCKED` e backoff são tecnologia |
| Drenagem e publicação | `app` | Processo próprio, com composition root e lifecycle |

> RFC §7.5 abre dizendo «duas responsabilidades distintas» e lista **três** na
> tabela. A tabela é a parte substantiva e é inequívoca. O artefato adota as
> três e registra a discrepância, sem editar a RFC.

### Bloco do mapeamento e momento da serialização

Obrigação delegada por escrito: o FND-03 §6.3 recusou a atribuição da Parte-1
(«um mapper na camada de aplicação») por colidir com a célula 12, e encerrou
declarando que «ficam com FND-04 [...] o bloco em que o mapeamento reside».

A matriz de RFC §7.4 fecha o espaço de escolha:

| Aresta | Célula | Status |
|--------|--------|--------|
| `application → contract` | 12 | ✗ **P0-2** — «contrato de wire é do adapter» |
| `application → provider` | 11 | ✗ — «caso de uso não conhece driver, tabela nem fila» |
| `port → contract` | 24 | ✗ **P0-2** — «assinatura de porta não expõe tipo de wire» |
| `provider → domain` | 25 | ✓ — «implementar a porta exige os tipos que ela expõe» |
| `provider → contract` | 30 | ✓ — «**serializar é papel do provider**» |

Logo o `application service` não pode construir nem serializar o integration
event, e a porta da outbox não pode receber tipo de wire.

**Decisão**: o mapeamento reside no `provider` da outbox e a serialização ocorre
**na escrita**. A porta recebe `(domain event, intenção de publicação)`; o
provider mapeia e serializa dentro da transação vinculada.

A alternativa — serializar na drenagem, com a outbox guardando forma neutra e o
relay convertendo ao publicar — foi descartada porque uma mudança de schema de
wire entre a escrita e a drenagem faria um fato antigo ser serializado sob o
contrato novo. Serializar na escrita congela os bytes no commit, e a mensagem
publicada é contemporânea do fato.

### Autoria dos campos da outbox

| Grupo | Campos | Autor |
|-------|--------|-------|
| Identidade e tempo do fato | `message_id`, `occurred_at` | `application service` — Parte-1 §9.1 passo 2, «resolver tempo e identificadores» |
| Roteamento | `destination` (destino **lógico**, nunca tópico ou fila), `partition_key` | `application service`, pela intenção de publicação |
| Origem de negócio | `aggregate_type`, `aggregate_id`, `aggregate_version` | derivados do domain event |
| Wire | `message_type`, `schema_version`, `payload` | `provider` |
| Estado de drenagem | `available_at`, `attempt_count`, `status`, `locked_by`, `locked_until`, `published_at`, `last_error` | relay |

Roteamento é decisão de orquestração; formato é decisão de wire. Derivar o
destino do tipo do evento no provider foi descartado: dois casos de uso que
emitem o mesmo evento de domínio para destinos distintos ficariam sem como
divergir.

### Sequência de produção (outbox)

A sequência de escrita do caso de uso e a sequência de drenagem são numeradas
**à parte**: são blocos, processos e ciclos de vida distintos.

**Sequência de escrita** (bloco `application service`):

```text
1. valida autorização de aplicação
2. resolve tempo e identificadores (message_id, occurred_at)
3. abre a UoW, recebendo as portas vinculadas à transação
4. carrega ou cria o agregado
5. executa a UPR e recebe a Decision
6. persiste o agregado com optimistic locking            ──┐
7. entrega (domain event, intenção) à porta da outbox      │  MESMA transação
   — o provider mapeia para integration event e serializa ──┘
8. commit
9. devolve a response de aplicação
```

**Sequência de drenagem** (bloco `app`, assíncrona):

```text
1. relay faz claim por lease, em transação curta
2. publica no broker, fora de qualquer transação de banco
3. marca o registro, condicionalmente ao claim ainda ser seu
```

Divergência declarada com a fonte: a Parte-1 §9.1 separa «mapear domain events
para integration events» (passo 7) de «serializar e inserir outbox» (passo 8),
ambos no application service. A matriz proíbe as duas coisas ali — célula 12 —,
e o FND-03 §6.3 já recusou essa atribuição. Do ponto de vista do caso de uso os
dois passos colapsam em um: entregar à porta.

Se o relay falhar após publicar e antes de marcar, a mensagem é republicada —
daí at-least-once.

A UoW não repete automaticamente o callback transacional: uma repetição criaria
novos identificadores, decisões ou efeitos sem que o service tivesse optado por
isso. Retry de serialização ou deadlock é política explícita, aplicada somente a
operação comprovadamente idempotente.

### Sequência de consumo (inbox)

```text
1. consumer adapter valida envelope e payload
2. adapter invoca um application service específico de consumo
3. o service abre a UoW incluindo a porta de inbox e as demais portas transacionais
4. chama insert-if-absent na porta de inbox com                ──┐
   (consumer_name, message_id, payload_hash) e RECEBE a         │
   classificação da recepção — a operação nunca falha por       │  MESMA
   violação de constraint                                       │  transação
5. sob primeira recepção, aplica efeitos locais e grava a       │
   outbox derivada; nas demais classificações, curto-circuita  ──┘
6. commit
7. o adapter confirma offset ou ACK somente após o retorno do commit
```

A operação da porta **devolve resultado, nunca erro**. Depender de violação de
constraint para decidir não é implementável no baseline: em PostgreSQL um erro
aborta o bloco de transação e todo comando seguinte é ignorado até o rollback,
de modo que a transação perdedora não teria como consultar o registro nem
aplicar disposição alguma. Como o provider obtém a semântica é tecnologia.

O status da inbox é restrito a `{processed, rejected}`. Sob transação única não
existe meio-termo observável: ou tudo commita e o registro nasce terminal, ou
nada commita e não há registro. O `processing` de Parte-1 §10.6 pressupõe
escrita em duas fases, que esta spec proíbe — a divergência é declarada, não
silenciosa.

A deduplicação, o efeito local e a outbox derivada constituem uma única
fronteira transacional. A inbox não é middleware externo que abra transação
diferente da usada pelo service.

### Disposições de consumo

A enumeração é exaustiva **e decidível**, em dois eixos com precedência
declarada. Uma lista plana mistura duas dimensões independentes: uma primeira
recepção pode terminar em rejeição, falha transitória ou falha terminal, de modo
que os casos não seriam mutuamente exclusivos.

**Eixo 1 — classificação da recepção**, retornada pela porta de inbox:

| | Condição | Curto-circuita o eixo 2 | Origem |
|---|----------|------------------------|--------|
| R1 | chave ausente — primeira recepção | não | Parte-1 §10.5 passo 4 |
| R2 | chave presente, hash igual, `processed` — reentrega de aplicada | sim | Parte-1 §10.5 passo 5 |
| R3 | chave presente, hash igual, `rejected` — reentrega de rejeitada | sim | Parte-1 §10.9 |
| R4 | chave presente, hash divergente — colisão de identificador | sim | Parte-1 §10.6 |

**Eixo 2 — desfecho do processamento**, avaliado somente sob R1:

| | Desfecho | Registro na inbox | Origem |
|---|----------|-------------------|--------|
| D1 | aplicado | commit, `processed` | Parte-1 §10.5 passo 6 |
| D2 | rejeitado por negócio | commit, `rejected` | Parte-1 §10.9; conecta com `Decision` rejeitada de FND-03 |
| D3 | falha transitória | rollback, nenhum registro | Parte-1 §10.9 — retry com backoff e jitter |
| D4 | falha terminal ou envelope inválido | contenção, nenhum registro | Parte-1 §10.9 — quarantine ou DLQ sem loop infinito |

As sete disposições são `R1×D1`, `R1×D2`, `R1×D3`, `R1×D4`, `R2`, `R3` e `R4`.
Cada uma declara o efeito na inbox, o efeito no broker (ACK, nack ou extensão de
visibilidade) e se produz mensagem derivada.

Propriedade derivada, verificável: **a inbox só contém mensagens cujo
processamento commitou**.

R3 emite ACK e **não reemite** o rejection event. O evento da primeira recepção
já está na outbox derivada e sua entrega é garantida pelo relay. Reemitir
produziria um evento com `message_id` novo, que o consumidor downstream não
deduplicaria — duplicando o efeito lá e violando V32 no elo seguinte.

### Retenção da inbox

A inbox cresce indefinidamente sem política de purga, mas purgar cedo demais
reabre a janela de duplicidade: uma chave removida antes de o transporte esgotar
a própria janela de redelivery volta a ser tratada como primeira recepção.

**Invariante**: `retenção_inbox ≥ janela_redelivery`. Purgar antes disso é erro
de operação, e a desigualdade torna a regra conferível.

O replay da DLQ, porém, pode ultrapassar qualquer retenção finita. Em vez de
inflar a inbox até o maior horizonte de replay concebível, o artefato declara
duas zonas:

| Zona | Proteção ativa |
|------|----------------|
| Replay dentro da retenção da inbox | dupla — inbox e idempotência de efeito |
| Replay além da retenção | única — apenas idempotência de efeito |

A operação de replay declara em qual zona opera. A purga é operação com
evidência.

### Failure modes

Cada cenário declara um **desfecho automático**, em três categorias:
**recuperação** (o sistema volta ao estado correto e o efeito pretendido
ocorre), **contenção** (isola sem perda, mas o efeito pretendido NÃO ocorreu) e
**reparação assistida** (exige decisão humana). Sem essa distinção, «foi para a
DLQ» satisfaria «recuperação automática», e o requisito ficaria vacuamente
verdadeiro nos cenários que mais importam.

| Falha | Momento | Desfecho | Categoria |
|-------|---------|----------|-----------|
| Crash antes do commit | escrita, passo 8 | nada persistiu; retry do caso de uso | recuperação |
| Crash após commit, antes do relay | drenagem | relay retoma pelo claim de lease | recuperação |
| Publicou mas não marcou enviado | drenagem | republicação; consumidor deduplica pela inbox | recuperação |
| Lease expirado com worker vivo | drenagem | registro volta ao pool; a republicação é duplicata aceitável | recuperação |
| **Claimant expirado escreve tarde** | drenagem | a transição é condicional ao claim corrente: a escrita do claim morto é rejeitada e o registro publicado não retorna ao pool | recuperação |
| Crash após commit, antes do ACK | consumo, passo 7 | redelivery; a inbox classifica como R2 e o adapter emite ACK | recuperação |
| Duplicata concorrente (corrida de inbox) | consumo, passo 4 | as duas transações chamam insert-if-absent; uma recebe R1 e aplica o efeito, a outra recebe R2 e curto-circuita, ambas vivas | recuperação |
| Colisão de `message_id` com payload divergente | consumo, passo 4 | classificação R4; o efeito não é reaplicado sob ID reutilizado | contenção |
| Mensagem sempre falha | consumo | contador de tentativas → DLQ ou quarantine | contenção |
| Replay conduzido a partir da DLQ | operação | reexecução auditada; a zona de retenção determina se a proteção é dupla ou só idempotência de efeito | reparação assistida |

O cenário de **claimant expirado** não estava catalogado e é material: worker A
adquire o lease e começa a publicar; o lease expira sem que A morra; worker B
adquire, publica e marca `published`; A retorna do broker e escreve sobre o
registro já concluído. A duplicação da publicação é aceitável sob at-least-once,
mas a sobrescrita do estado por um claim morto devolveria ao pool um registro já
publicado, a cada ciclo.

A **corrida de inbox** foi reescrita: a redação anterior dizia que a constraint
única faz a transação perdedora falhar na inserção e que ela então resolve pela
disposição de duplicata. Isso não é implementável no baseline — em PostgreSQL um
erro de constraint aborta o bloco de transação e todo comando seguinte é
ignorado até o rollback, de modo que a transação perdedora não teria como
consultar o registro nem aplicar disposição alguma.

### Relay

Claim por **lease** com prazo: o worker reivindica um lote, publica e libera.
Lease expirado volta ao pool. Nenhum lock de banco é mantido durante o I/O, e
`FOR UPDATE SKIP LOCKED` é admitido **somente durante o claim**.

**Propriedade do estado**: a transição final é condicional ao claim corrente — o
worker só transiciona o registro se `locked_by` ainda for o seu claim. Para isso
`locked_by` identifica a **execução do claim**, não o processo: um mesmo worker
que readquira o registro depois tem identidade nova, e um claim antigo não se
passa pelo corrente. Nenhum campo é acrescentado ao schema mínimo.

Capacidades obrigatórias do drenador: paginação, batch configurável, lease com
expiração, retry com backoff e jitter, limite de concorrência, exposição dos
sinais de `pending`, `lag`, `attempts` e `failures`, e graceful shutdown.

A **exposição** dos sinais é capacidade do mecanismo e é normatizada aqui,
conforme Parte-1 §10.4. Os nomes das métricas, os limiares, os alarmes, o
catálogo por componente e o runbook de DLQ e replay são de FND-08
([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445)) e aparecem neste
artefato apenas como fronteira `encaminhado`.

Polling com leasing é o mecanismo padrão. CDC é extensão para alto volume ou
baixa latência. A escolha entre os dois é decisão estrutural acionada como ADR
(ver «Acionamento de ADR»).

### Idempotência de negócio

Parte-1 §10.1 lista **três** obrigações quando há efeitos persistentes: «inbox,
idempotência de negócio e outbox». A inbox é uma delas, não as três. Ela impede
reaplicar a mesma identidade de mensagem enquanto o registro existir, e não
protege contra o mesmo efeito lógico chegando com `message_id` novo, contra
purga seguida de replay, nem contra duas mensagens distintas que representam a
mesma operação de negócio.

O artefato normatiza as duas camadas como distintas:

| Camada | Escopo | Vida | Mecanismo |
|--------|--------|------|-----------|
| Deduplicação por identidade de mensagem | `(consumer_name, message_id)` | limitada pela janela de retenção | inbox |
| Idempotência do efeito de negócio | chave natural da operação no domínio | permanente | do domínio: constraint natural, verificação de estado ou operação convergente |

A primeira **não substitui** a segunda: efeito persistente exige as duas. É a
segunda que sustenta o vetor **V32** (efeito idempotente sob redelivery), que é
P0 e não se apoia só na inbox.

### `payload_hash`

O `payload_hash` é o que distingue redelivery legítima (R2, R3) de reutilização
indevida do identificador (R4). Sem contrato, R4 falha nos dois sentidos: se o
hash variar entre serializações equivalentes de stacks diferentes, uma
redelivery legítima vira colisão e o efeito nunca se aplica; se o hash cobrir
metadados de transporte, **toda** reentrega tem hash diferente da primeira, R4
dispara sempre e a deduplicação para de funcionar.

O artefato normatiza duas propriedades e nenhuma fórmula:

1. **Estável sob serializações equivalentes** — o mesmo conteúdo de negócio
   produz o mesmo hash em qualquer stack
2. **Restrito ao conteúdo de negócio** — metadados de transporte, tracing e
   contadores de tentativa ficam fora

O algoritmo, a canonicalização, o escopo dos bytes e o versionamento são
`encaminhado` a FND-05 ([ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442)).
A verificação cross-stack é `encaminhado` a FND-09
([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)), que já usa o payload
hash como detector de divergência de serialização.

### Retry, DLQ, quarantine e replay

- Falha transitória: retry limitado com backoff exponencial e jitter.
- Envelope inválido ou schema desconhecido: quarantine ou DLQ, sem loop infinito.
- Rejeição de negócio esperada: ACK e, quando útil, rejection event.
- Falha técnica esgotada: DLQ com envelope íntegro e erro sanitizado.
- Replay da DLQ exige ferramenta, auditoria e proteção contra duplicidade.
- Poison message não bloqueia indefinidamente uma partição ou um grupo FIFO.

### Sagas e process managers

Fluxo distribuído de longa duração não usa transação distribuída. A política é
referenciada, não implementada: estado da saga persistido; commands emitidos por
outbox; eventos recebidos por inbox; compensação como ação de negócio explícita,
não rollback técnico; cada etapa idempotente; correlação e causação preservadas.
Uma saga pertence à camada de aplicação ou a um módulo de processo dedicado, e
não transforma agregados de bounded contexts distintos em agregado distribuído.

### Diagramas

Os fluxos de produção e de consumo e os failure modes críticos são
diagramados em Mermaid (`sequenceDiagram`), seguindo o tratamento de RFC §8: o
diagrama é **derivado** do texto normativo e não estabelece regra. Quando um
diagrama divergir das sequências desta especificação, prevalece o texto, e a
divergência é defeito do diagrama.

### Sucessão da Parte-1 §§9–10

O artefato declara, sob autorização da ANC-02, que sucede à Parte-1 §§9–10 no
recorte da âncora, seguindo o tratamento já adotado pelo FND-03: a cláusula de
sucessão vive no artefato sucessor, sem editar a RFC.

A sucessão é **por subseção, com ressalva**, e não por capítulo: várias
subseções da fonte têm ownership misto por dentro, e marcá-las inteiras como
consolidadas repetiria a invasão de âncora em escala menor. A tabela usa os três
estados do precedente (`Consolidado`, `Recepcionado`, `Vigente`) mais uma coluna
que nomeia o que dentro da subseção pertence a outra âncora:

| Subseção | Estado | Ressalva — pertence a outra âncora |
|----------|--------|-------------------------------------|
| §9.1 Responsabilidade e fluxo | Consolidado | passos 1, 2 e 5 (autorização, tempo e identificadores, UPR) — FND-07 e FND-03; formato da conversão — FND-05 |
| §9.2 Transação explícita | Consolidado | — |
| §9.3 Queries | Recepcionado | — |
| §10.1 Semântica oficial | Consolidado | — |
| §10.2 Fluxo de produção | Consolidado | — |
| §10.3 Outbox mínima | Consolidado | `message_type`, `schema_version` e a natureza do `payload` — ANC-03 / FND-05 |
| §10.4 Relay | Consolidado | nomes de métrica, limiares e alarmes — ANC-06 / FND-08 |
| §10.5 Fluxo de consumo | Consolidado | validação de envelope e comportamento de ACK do adapter — ANC-04 / FND-06 |
| §10.6 Inbox mínima | Consolidado | `message_id` como «ID do CloudEvent» e `message_type` como «contrato recebido» — ANC-03 / FND-05 |
| §10.7 Kafka | **Vigente** | inteira — ANC-04 / FND-06 |
| §10.8 SNS e SQS | **Vigente** | inteira — ANC-04 / FND-06 |
| §10.9 Retry, DLQ e quarantine | Consolidado | retry por transporte — FND-06; operação de DLQ e runbook — FND-08 |
| §10.10 Sagas e process managers | Recepcionado | timeouts, observabilidade e replay/repair — FND-08 |

A atualização da tabela de RFC §14.4 fica registrada como **pendência
aberta**, no mesmo formato do
precedente, e é escalada aos revisores no PR; se exigir rito de versão, torna-se
alteração própria da RFC.

## Decisões técnicas

- **Outbox transacional em vez de publicação direta**: garante que estado e
  intenção de publicar commitam juntos. Alternativa descartada: publicar no
  broker dentro da transação, porque broker não participa da transação do banco
  e a falha entre commit e publish perderia o evento silenciosamente.
- **At-least-once com efeitos idempotentes**: é a semântica oficial, e a
  constraint P0-3 proíbe prometer exactly-once E2E. Alternativa descartada:
  exactly-once via transação distribuída, porque o custo operacional é alto e
  a garantia não sobrevive à fronteira de rede.
- **Escrita da outbox por porta, não por acesso à tabela**: mantém a célula 11
  proibida e preserva o application service livre de tecnologia. Alternativa
  descartada: o service gravar direto na tabela, porque abriria exceção a uma
  invariante da ANC-02 e acoplaria o caso de uso ao provider.
- **Drenagem em app dedicado**: o relay tem lifecycle e composition root
  próprios. Alternativa descartada: drenar dentro do processo que atende
  requisições, porque mistura ciclos de vida e faz a latência do broker
  competir com o caminho de request.
- **Claim por lease, não lock pessimista**: o relay não segura transação
  durante I/O. Alternativa descartada: `SELECT FOR UPDATE` mantido durante o
  publish, porque prende conexão do pool pela latência do broker e degrada sob
  carga.
- **Inbox como tabela de idempotência, não cache**: a deduplicação participa da
  mesma transação dos efeitos. Alternativa descartada: dedupe em cache externo,
  porque cache e banco podem divergir e reintroduzem a janela de duplicidade.
- **`payload_hash` na inbox, não apenas a chave**: distingue redelivery legítima
  de reutilização indevida do `message_id`. Alternativa descartada: confiar só
  em `(consumer_name, message_id)`, porque um produtor que reaproveite o ID com
  outro conteúdo teria o segundo efeito silenciosamente descartado.
- **UoW sem retry automático do callback**: repetição implícita geraria novos
  identificadores e decisões. Alternativa descartada: retry transparente na
  UoW, porque transforma efeito não idempotente em duplicidade invisível ao
  autor do caso de uso.
- **Mapeamento e serialização no provider da outbox, na escrita**: é a única
  atribuição que a matriz endossa — a célula 30 diz literalmente «serializar é
  papel do provider» —, e congela os bytes no commit, de modo que a mensagem
  publicada é contemporânea do fato. Alternativa descartada: serializar na
  drenagem, porque uma mudança de schema entre a escrita e a publicação faria um
  fato antigo sair sob o contrato novo.
- **Roteamento pela intenção de publicação, não derivado do tipo do evento**:
  preserva no caso de uso uma decisão que é dele. Alternativa descartada:
  derivar o destino do tipo do domain event por registro no composition root,
  porque dois casos de uso que emitem o mesmo evento para destinos distintos
  ficariam sem como divergir.
- **Porta de inbox com retorno de classificação, não com erro de constraint**:
  torna a corrida de inbox implementável e faz a assinatura da porta coincidir
  com o eixo 1 das disposições. Alternativa descartada: especificar savepoint na
  norma, porque acopla a fundação a uma família de tecnologia.
- **Status da inbox restrito a estados terminais**: sob transação única não há
  meio-termo observável. Alternativa descartada: manter `processing` com escrita
  em duas fases, porque reabre a janela de duplicidade que a fronteira
  transacional única existe para fechar.
- **Transição do relay condicional ao claim corrente**: impede que um claimant
  expirado devolva ao pool um registro já publicado. Alternativa descartada:
  fencing token em campo próprio, porque o ganho só aparece sob desalinhamento
  de relógio que o lease já assume tolerável, e acrescenta campo que a fonte não
  tem.
- **Idempotência de negócio como camada separada da inbox**: a inbox protege
  identidade de mensagem, não identidade de operação. Alternativa descartada:
  tratar a inbox como suficiente, porque deixa V32 sem base nos casos de
  `message_id` novo, purga seguida de replay e mensagens distintas para a mesma
  operação.
- **`payload_hash` com propriedades declaradas em vez de fórmula**: mantém a
  fronteira com FND-05 e impede as duas falhas simétricas de R4. Alternativa
  descartada: definir o algoritmo aqui, porque invade a ANC-03.

## Acionamento de ADR

ANC-02 registra «ADR exigido: Sim — escolha de mecanismo de relay (polling ×
CDC)», e RFC §13.3 confirma que o mecanismo de relay da outbox é acionado por
esta sub-spec. O artefato aciona no formato de RFC §13.2 — nomear, definir
assunto, registrar origem e encaminhar ao FND-11
([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)) —, sem redigir nem
aceitar.

São **dois** acionamentos. O segundo responde à obrigação que o FND-03 §6.3
delegou por escrito a esta entrega, e o critério que o separa das demais
decisões deste artefato é que ele altera **qual bloco conhece qual** — a matriz
de dependências em uso —, enquanto as outras alteram o conteúdo da norma.

| ID provisório | Nome | Assunto | Origem | Destino | Owner | Estado |
|---------------|------|---------|--------|---------|-------|--------|
| `ADR-DMPF-K` | Mecanismo de relay da outbox | Polling com leasing como padrão, CDC como extensão para alto volume ou baixa latência | ANC-02, RFC §13.3 | ARQ-448 | FND-11 | `acionado` |
| `ADR-DMPF-L` | Bloco do mapeamento e momento da serialização | Mapear `domain event → integration event` e serializar no `provider` da outbox, na escrita, com a porta expondo tipo de domínio | FND-03 §6.3; RFC §7.4 células 12, 24, 25 e 30 | ARQ-448 | FND-11 | `acionado` |

A alternativa descartada de cada decisão está registrada em «Decisões técnicas»
e é insumo obrigatório da redação, conforme RFC §13.2.

A numeração `ADR-006` a `ADR-009`, pedida pela descrição da ARQ-441, não é
utilizável: `docs/adr/006-fechamento-port-melhorias.md`,
`007-fronteira-plugin-template.md`, `008-identidade-automacao-release.md` e
`009-remocao-libs-exemplo.md` já existem e tratam de outros assuntos. É por isso
que RFC §13.1 reserva a faixa `010`–`024` aos ADRs estruturais do DMPF.

Os identificadores acima são **provisórios**. RFC §13.2 estabelece que a
numeração definitiva na faixa `010`–`024` é atribuída pelo FND-11 na promoção, e
que referenciar um ADR por número definitivo antes disso é erro de
rastreabilidade. Como FND-04 e FND-05 correm em paralelo no grafo do épico, dois
acionamentos podem circular com o mesmo sufixo provisório em branches distintas;
a reconciliação é do FND-11, que detém a atribuição — a spec não estabelece
convenção própria para isso.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] Sequências e failure modes AC-07 documentados no artefato promovido para `docs/dmpf/uow-inbox-outbox.md`**
- [ ] **[P0] At-least-once explícito; exactly-once E2E proibido na especificação**
- [ ] **[P0] Autorização pela ANC-02 declarada no artefato, com escopo permitido, invariantes, monotonicidade e cláusula de precedência da RFC**
- [ ] **[P0] Atribuição de blocos de RFC §7.5 honrada, com a escrita da outbox por porta e a célula 11 preservada**
- [ ] **[P0] Schemas mínimos de outbox e inbox normatizados, com a chave `(consumer_name, message_id)` e o `payload_hash`**
- [ ] **[P0] Disposições de consumo em dois eixos com precedência declarada — 7 disposições, cada uma com efeito na inbox, efeito no broker e se produz derivada**
- [ ] **[P0] Bloco do mapeamento decidido e justificado, com a cadeia de células da matriz explicitada — obrigação delegada pelo FND-03 §6.3**
- [ ] **[P0] Autoria dos campos da outbox declarada por grupo, separando roteamento de formato**
- [ ] **[P0] Porta de inbox com semântica de retorno de classificação, não de erro de constraint**
- [ ] **[P0] Status da inbox restrito a `{processed, rejected}`, com a divergência com Parte-1 §10.6 declarada**
- [ ] **[P0] Transição do relay condicional ao claim corrente, com `locked_by` identificando a execução do claim**
- [ ] **[P0] Idempotência de negócio normatizada como camada distinta da deduplicação por inbox**
- [ ] **[P0] `payload_hash` com as duas propriedades declaradas, sem definir algoritmo**
- [ ] **[P0] Invariante `retenção_inbox ≥ janela_redelivery` e as duas zonas de proteção do replay declaradas**
- [ ] **[P0] Failure modes com categoria de desfecho declarada (recuperação, contenção, reparação assistida)**
- [ ] **[P0] Fronteira com FND-08 respeitada: capacidade de observação aqui, catálogo de métricas e runbook encaminhados**
- [ ] **[P0] Dois ADRs acionados no formato de 7 colunas de RFC §13.2, com alternativa descartada registrada para cada um**
- [ ] **[P0] Sucessão da Parte-1 §§9–10 declarada sob ANC-02 por subseção e com coluna de ressalva, com a pendência de RFC §14.4 registrada**
- [ ] **[P0] Diagramas de sequência dos fluxos e dos failure modes críticos presentes, em Mermaid e declarados derivados do texto**
- [ ] **[P0] Janela de retenção da inbox declarada, com a relação com o prazo de redelivery do transporte**
- [ ] **[P0] Corrida de inbox coberta no catálogo com semântica implementável no baseline PostgreSQL**
- [ ] **[P0] Sobrescrita tardia por claimant expirado coberta no catálogo de failure modes**
- [ ] **[P1] Índice de termos publicado, ligando cada termo novo à seção que o define**
- [ ] **[P0] Baseline PostgreSQL + Kafka/SNS/SQS viável sem recurso exclusivo de vendor**
- [ ] **[P0] Catálogo de failure modes encaminhado como insumo de ARQ-446 (SPEC-6RQBN98G)**
- [ ] **[P1] Política de sagas/process managers referenciada (sem implementação)**

### Cenários de teste (mínimo 3)

```text
DADO um caso de uso que grava estado de negócio e enfileira um evento na outbox
QUANDO o processo cai depois do commit e antes de o relay publicar
ENTÃO o relay retoma pelo claim de lease e publica o evento, sem perda

DADO um consumidor que já processou a mensagem de chave K e commitou a inbox
QUANDO a mesma mensagem K é redelivered pelo broker
ENTÃO o efeito não é reaplicado, o ACK é emitido e o estado final é o mesmo
     (vetor V32 de RFC §11)

DADO uma mensagem que falha em todas as tentativas configuradas
QUANDO o limite de tentativas é atingido
ENTÃO a mensagem vai para quarantine com a causa registrada e fica disponível
     para replay, sem ser descartada

DADO um produtor que reaproveita o message_id K com payload diferente
QUANDO o consumidor compara o payload_hash na inbox
ENTÃO a disposição é a de colisão, distinta da de duplicata, e o efeito não é
     aplicado sob o ID reutilizado

DADO duas entregas concorrentes da mesma mensagem K em dois workers
QUANDO ambas chamam insert-if-absent na porta de inbox com (consumer_name, K)
ENTÃO uma recebe a classificação R1 e aplica o efeito, a outra recebe R2 e
     curto-circuita, as duas transações permanecem vivas e o efeito é
     aplicado uma só vez

DADO o worker A com lease expirado que retorna do broker após o worker B ter
     publicado e marcado o registro
QUANDO A tenta escrever a transição final
ENTÃO a escrita é rejeitada por não deter o claim corrente, e o registro
     publicado não retorna ao pool

DADO um application service que entrega (domain event, intenção) à porta da outbox
QUANDO confrontado com a matriz de RFC §7.4
ENTÃO nenhuma aresta atravessa as células 11, 12 ou 24, e o mapeamento e a
     serialização ocorrem sob as células 25 e 30

DADO uma mensagem reexecutada da DLQ após a purga da sua chave na inbox
QUANDO o consumidor a recebe como primeira recepção
ENTÃO o efeito não duplica, porque a proteção nessa zona é a idempotência de
     negócio, e a norma declara que a inbox não a substitui

DADO um consumidor que recebe pela segunda vez uma mensagem já rejeitada
QUANDO a inbox classifica a recepção como R3
ENTÃO o adapter emite ACK e nenhum rejection event é reemitido

DADO um exemplo de application service que grava a outbox acessando a tabela
QUANDO confrontado com a atribuição de blocos de RFC §7.5
ENTÃO é rejeitado por violar a célula 11 (application → provider)

DADO qualquer trecho do artefato, do README ou de configuração de exemplo
QUANDO varrido por promessa de entrega
ENTÃO nenhum promete exactly-once fim a fim (vetor V31 de RFC §11)
```

<critical_constraints>
- [P0-1] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0-2] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0-3] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0-4] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
- [P0] Atribuição de blocos de RFC §7.5 é invariante: a escrita da outbox ocorre por uma porta, NUNCA tocando a tabela
</critical_constraints>

## Escopo fora

- **Implementação do relay e das tabelas**: aqui só a semântica; o código é
  dos épicos de kernel.
- **Implementação de CDC**: a decisão entre polling e CDC é acionada como ADR
  nesta spec, mas o mecanismo concreto de captura permanece fora da fundação.
- **Escolha de broker e tuning**: convenções por transporte são de FND-06
  (SPEC-YWFGNPG5), que devolve a esta spec a semântica transacional do consumo.
- **Formato do envelope publicado**: é de FND-05 (SPEC-7H08RZDG).
- **Implementação de sagas e process managers**: a política é referenciada,
  não implementada.
- **Redação e aceite dos ADRs acionados**: pertencem ao FND-11 (ARQ-448).
- **Catálogo de métricas, limiares, alarmes e runbook de DLQ/replay**: são de
  FND-08 (SPEC-E15TBHCD). Aqui só a capacidade de o drenador expor os sinais.
- **Algoritmo, canonicalização e versionamento do `payload_hash`**: são de
  FND-05 (SPEC-7H08RZDG). Aqui só as propriedades que R4 exige.
- **Verificação cross-stack do `payload_hash`**: é de FND-09 (SPEC-6RQBN98G).
- **Separação física de bancos para queries**: CQRS lógico é admitido; a
  separação de infraestrutura é decisão de contexto.
