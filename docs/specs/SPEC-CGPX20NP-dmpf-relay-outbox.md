---
id: SPEC-CGPX20NP
slug: dmpf-relay-outbox
title: DMPF KRN-08 — Relay da outbox: claim por lease, publicação e sinais
stage: done
priority: P0
depends_on: [SPEC-ZHE7DN1H, SPEC-WYX5GW87, SPEC-3R80KNMS]
ticket_url: null
subtask_urls: []
created: 2026-09-05
---

# SPEC-CGPX20NP: DMPF KRN-08 — Relay da outbox: claim por lease, publicação e sinais

## Resumo

O `KRN-06` grava a linha da outbox na mesma transação do estado de negócio.
Enquanto ninguém a drenar, o fato commitou e não saiu do serviço. Esta spec
entrega o drenador: o processo que reivindica registros elegíveis por lease,
publica no broker **fora de qualquer transação de banco** e transiciona o
registro **apenas se o claim ainda for o seu**.

Como operador do sistema, quero que todo fato commitado alcance o broker sem
que a latência da publicação prenda uma conexão do pool, para que um pico de
drenagem não derrube o caminho de request.

A entrega fecha o incremento 3 — Garantias — junto com o `KRN-07`: o que este
relay publica chega ao consumidor com o mesmo `message_id` e é absorvido pela
inbox.

## Contexto

- **Problema**: hoje `dmpf_outbox` acumula registros em `status = 'pending'` e
  nada os lê. `libs/backend/go/postgres/outbox.go:42-81`
  (`Enqueue`) é o único caminho de escrita da tabela, e ele deixa `status`,
  `attempt_count`, `locked_by`, `locked_until`, `published_at` e `last_error`
  ao default do schema — porque, no comentário do próprio arquivo
  (`outbox.go:18-20`), "their initial values are drain state, and the writer
  has no business authoring them". Ninguém escreve esse estado de drenagem
  porque o drenador não existe.
- **Impacto**: sem o relay, o padrão outbox está pela metade. A atomicidade
  entre estado e mensagem já vale, mas a mensagem nunca sai. Com ele, o
  `KRN-07` passa a receber tráfego real, e o incremento 3 tem o desfecho que
  a `SPEC-YRJRADY9` lhe atribui: "Mensagem publicada atomicamente e consumida
  com deduplicação".
- **Inspiração**: o próprio acervo. A RFC registra em `§7.5` que
  `legado-titulos-shared-services` é o único caso real do universo inventariado, com
  drenagem em apps dispatcher dedicados e claim por `FOR UPDATE SKIP LOCKED`.
  O desenho aqui é a normatização desse caso, não uma invenção.
- **Links relevantes**:
  - `docs/specs/SPEC-3R80KNMS-dmpf-outbox-postgres.md` — `KRN-06`, a escrita
    que produz os registros que este relay drena. Declara em `:650` que "o
    índice de elegibilidade ao claim (`OBX-09`) é do `KRN-08`, que conhece a
    query".
  - `docs/specs/SPEC-ANZX2WPG-dmpf-inbox-consumo.md` — `KRN-07`, o consumo que
    absorve a republicação deste relay, e a origem do bloco `app` no
    workspace.
  - `docs/specs/SPEC-WYX5GW87-dmpf-contratos-wire.md` — `KRN-05`, o codec do
    envelope que este relay usa para montar o CloudEvent na drenagem.
  - `docs/specs/SPEC-YRJRADY9-dmpf-kernel-sdk-go.md` — a spec guarda-chuva do
    épico; `KRN-08` é a quarta entrega do incremento 3.
  - `docs/adr/020-relay-polling-leasing-e-cdc-extensao.md` — decide o
    mecanismo.
  - `docs/adr/021-mapeamento-no-provider-serializacao-na-escrita.md` — decide
    que os bytes congelam na escrita.
  - `docs/adr/035-realizacao-postgres-da-outbox.md` — o schema real, e as duas
    obrigações que ele delega nominalmente ao `KRN-08`.
  - `docs/dmpf/uow-inbox-outbox.md` — FND-04, fonte normativa das cláusulas
    `OBX-*` e `BLK-02`.
  - `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — RFC, fonte de `P0-3` e `§7.5`.

### Divergências entre o ticket e o repositório

O ticket `ARQ-527` foi escrito em 30/08/2026, antes de `KRN-05`, `KRN-06` e
`KRN-07` entrarem. Sete pontos precisam de reconciliação, e a spec prevalece
sobre o ticket em todos.

| Ticket `ARQ-527` diz | Repositório em 05/09/2026 | Esta spec adota |
| --- | --- | --- |
| App `apps/backend/reference` no bloco `app` | `apps/backend/` contém apenas `.gitkeep`; nenhum projeto Nx registrado. O bloco `app` real nasceu em `libs/backend/go/app` (`KRN-07`), cujo `doc.go:15` já reserva o lugar: "the relay, KRN-08". A convenção do ADR-030 é `libs/<scope>/<stack>/<módulo>`, e `libs/backend/go/conformance/cmd` é o precedente de binário dentro de lib | Pacote `relay/` e binário `cmd/dmpf-relay/` **dentro de `libs/backend/go/app`**. O nome `reference` fica reservado ao composition root de exemplo do `KRN-12`, com quem colidiria |
| "O ADR-021 explica por que o relay é agnóstico ao que publica" | O congelamento dos bytes é decisão do ADR-021 (`:57`, "Serializar na escrita congela os bytes no commit"), mas a conclusão "e é isso que torna o relay agnóstico ao conteúdo do que publica" está no **ADR-020** (`:108-113`), na seção de Referências | Cita a origem correta de cada afirmação |
| Escopo silente sobre `payload_hash` | `docs/adr/035-realizacao-postgres-da-outbox.md:163-165`: "`payload_hash` é redundante enquanto ninguém o lê: quem passa a conferi-lo na drenagem é o `KRN-08`" | Conferência de `payload_hash` na drenagem é requisito **P0** desta entrega |
| Escopo silente sobre a montagem do CloudEvent | `docs/adr/035-...md:40-44`: "O relay (`KRN-08`) monta o CloudEvent na drenagem a partir das colunas — `message_id`→`id`, `message_type`→`type`, `schema_version`→`dataschema`, `occurred_at`→`time`, `partition_key`, `aggregate_version` — e de `metadata`". A coluna `payload` guarda **só** os bytes do `Any` do integration event (`envelope.Pack`, `outbox.go:58-64`), nunca o CloudEvent inteiro | Montagem do envelope a partir das colunas é entregável **P0**. Sem ela o relay não tem o que publicar |
| Escopo silente sobre a serialização do envelope | O package `envelope` expõe `Pack`, `Encode`, `Decode`, `Unmarshal` e `Unpack`. `Encode` devolve `*cloudeventsv1.CloudEvent` — o tipo gerado, **não bytes**. Não existe `Marshal`, o simétrico de `Unmarshal`, cujo godoc declara a intenção: "so adapters that receive raw transport bytes never import the CloudEvent generated type directly" | Acrescentar `envelope.Marshal(Envelope) ([]byte, error)` ao `contracts`, pelo mesmo motivo que justifica `Unmarshal`. Requisito **P0** |
| Trata claim e drenagem como um bloco só | RFC `§7.5` separa: **persistência da tabela e do claim** é do `provider` ("Tabela, `SKIP LOCKED` e backoff são tecnologia"); **drenagem e publicação** é do `app` ("É um processo próprio, com composition root e lifecycle") | Entrega em dois módulos: o claim e as transições em `postgres`, o laço e o lifecycle em `app` |
| Cita `OBX-03, 06, 07, 08, 09, 10, 12, 13, 14, 16, 18` | FND-04 tem `OBX-01`..`OBX-18`. Quatro cláusulas não citadas pelo ticket governam esta entrega: `OBX-04` (não existe transição `publishing → pending`), `OBX-05` (valores iniciais que tornam o registro elegível), `OBX-11` (nenhum claim substituído altera estado) e `OBX-17` (retenção: `published` purgável, `failed` não) | Cobre as quinze cláusulas aplicáveis. `OBX-15` (CDC) fica no escopo fora, como o ticket determina |

### Fontes normativas

Todas as citações abaixo são literais, com `arquivo:linha` verificado em
05/09/2026.

**Cláusulas de mecanismo — `docs/dmpf/uow-inbox-outbox.md`**

| ID | Linha | Texto normativo (literal, abreviado onde indicado) |
| --- | --- | --- |
| `BLK-02` | `:305-307` | "bloco `app`. A drenagem não roda dentro do processo que atende requisições. O relay tem composition root, configuração de concorrência e lifecycle próprios (§5)." |
| `OBX-03` | `:643-645` | "bloco `app`. `last_error` guarda diagnóstico **sanitizado**: uma mensagem de erro que reproduza payload de negócio, credencial ou stack trace com dado de usuário viola a vedação acima, porque a outbox é lida por operação." |
| `OBX-04` | `:682-685` | "Não existe transição de `publishing` para `pending`. Lease expirado e falha transitória devolvem o registro ao pool pela **comparação de prazos** de §5.1, não por uma escrita que o rebaixe de estado. O laço no diagrama é ausência de transição, não uma transição para si mesmo." |
| `OBX-05` | `:687-697` | "Os valores iniciais são declarados pelo schema, e um registro recém-escrito nasce elegível ao claim" — `status` → `pending`, `available_at` → `occurred_at`, `attempt_count` → zero. A escrita "**não** preenche `locked_by`, `locked_until` nem `published_at`." |
| `OBX-06` | `:719-720` | "`failed` é terminal para o ciclo automático. Retomar um registro `failed` é operação, com evidência (§7.4)." |
| `OBX-07` | `:777-781` | "**Nenhum lock de banco é mantido durante o I/O com o broker.** A transação de claim abre, marca o lote e fecha; a publicação ocorre fora de qualquer transação. `FOR UPDATE SKIP LOCKED` é admitido **somente durante o claim**." |
| `OBX-08` | `:789-793` | "**`locked_by` identifica a execução do claim, não o processo.** Um mesmo worker que readquira o mesmo registro depois recebe identidade nova. Sem isso, a comparação de §5.2 não distingue o claim corrente de um claim anterior do mesmo worker, e a proteção que ela oferece desaparece no caso mais provável de ocorrer." |
| `OBX-09` | `:795-797` | "Um registro é elegível ao claim quando `status` é `pending`, ou quando `status` é `publishing` e `locked_until` já passou — e, nos dois casos, quando `available_at` já passou." |
| `OBX-10` | `:807-809` | "bloco `app` (relay). Após o retorno do broker, o worker só transiciona o registro **se `locked_by` ainda for o seu claim**. A escrita de um claim que não é mais o corrente é rejeitada e não altera o registro." |
| `OBX-11` | `:834-836` | "A propriedade que esta subseção estabelece é enunciável sozinha, e é ela que os testes de §11.3 conferem: **nenhum claim substituído altera o estado de um registro.**" |
| `OBX-12` | `:876-878` | "**A exposição dos quatro sinais é propriedade do mecanismo e obriga aqui.** Um relay que não permita observar `pending`, `lag`, `attempts` e `failures` não satisfaz esta seção, ainda que funcione." |
| `OBX-13` | `:892-895` | "Graceful shutdown significa: parar de reivindicar novos lotes, concluir ou liberar os claims vivos, e só então encerrar. Um relay que encerre com claims vivos não perde mensagem — o lease expira e o registro volta ao pool —, mas atrasa a drenagem pelo prazo do lease sem necessidade." |
| `OBX-14` | `:930-932` | "O relay **não** interpreta o conteúdo do `payload` e não decide publicar ou não com base nele. Se o registro existe, o fato ocorreu e commitou; a decisão de publicar já foi tomada no caso de uso." |
| `OBX-16` | `:782-787` | "A transação de claim escreve, **no mesmo commit**: `status` para `publishing`, `locked_by` com a identidade do claim, `locked_until` com o prazo do lease, e incrementa `attempt_count`. A atomicidade é correção-crítica: um `status` em `publishing` sem `locked_until` gravado junto quebraria a elegibilidade que `OBX-09` define, e o registro ficaria reivindicado por ninguém, sem prazo para voltar ao pool." |
| `OBX-17` | `:730-757` | `published` é **purgável**; `failed` **não é purgável** pelo ciclo automático; `pending` e `publishing` não são purgáveis. "A purga de `published` é **operação com evidência**." |
| `OBX-18` | `:917-922` | "**No passo 3b, `locked_until` é liberado junto com o recálculo de `available_at`.** Sem isso o backoff não governa: `OBX-09` só devolve ao pool um registro `publishing` quando `locked_until` **e** `available_at` já passaram, de modo que um backoff menor que o lease seria inerte e a próxima tentativa esperaria o lease restante, não o backoff. A liberação ocorre no mesmo commit da transição, sob a condição de `OBX-10`." |

**Capacidades obrigatórias — `docs/dmpf/uow-inbox-outbox.md:853-874` (§5.3)**

As oito linhas da tabela: paginação; batch configurável; claim concorrente
entre workers sem lock mantido durante o I/O; lease com expiração; retry com
backoff exponencial e jitter; limite de concorrência; **exposição** dos sinais
`pending`, `lag`, `attempts`, `failures`; graceful shutdown.

O texto imediatamente após a tabela (`:870-874`) é decisivo para o desenho:

> `normativo` — A tabela lista **propriedades**, não realizações.
> `FOR UPDATE SKIP LOCKED` é a realização admitida do claim concorrente no
> baseline PostgreSQL (§5.1) e **não é exigida**: qualquer mecanismo que
> reivindique sem manter lock durante o I/O satisfaz a linha.

**Os quatro sinais.** FND-04 define **apenas a obrigação de exposição**. Nomes
de métrica, unidades, limiares, alarmes e runbook são `encaminhado` a FND-08
sob ANC-06 (`:880-882`). Esta spec, portanto, **não** fixa nome de métrica nem
limiar — fixa a superfície pela qual os quatro sinais são observáveis.

**Garantias de entrega — `docs/dmpf/uow-inbox-outbox.md:1405-1460` (§7.1)**

- `GAR-01` (`:1413-1417`): "**Esta fundação não promete exactly-once fim a
  fim, e nenhum documento, contrato, README ou configuração produzido sob ela
  pode prometê-lo.**"
- A janela entre publicar e marcar (`:1431-1438`) é uma das **duas** origens
  concretas de duplicata do desenho, e produz **republicação** pelo relay.
  `:924-928`: "A duplicata é aceitável e esperada: é a origem concreta do
  at-least-once (§7.1), e é o consumidor que a absorve (§6.4)."
- Vetores: `V31` (vedação a exactly-once, `structurally reviewable`) e `V32`
  (efeito idempotente sob redelivery, `runtime-testable`). `:1454-1457`:
  "**V32 não é satisfeito por revisão de código**: só a reentrega executada
  demonstra o efeito."

**Constraint `P0-3` — `docs/dmpf/rfc-dmpf-foundation-v0.1.md:230-234`**

> A semântica oficial de entrega é at-least-once com efeitos idempotentes.
> Nenhum componente, documento, configuração ou contrato pode prometer
> exactly-once fim a fim.

**Alocação por bloco — `docs/dmpf/rfc-dmpf-foundation-v0.1.md:950-974` (§7.5)**

| Responsabilidade | Bloco | Razão (literal) |
| --- | --- | --- |
| **Escrita** na mesma transação do estado | `application service` | "É parte do caso de uso" |
| **Persistência** concreta da tabela e do claim | `provider` | "Tabela, `SKIP LOCKED` e backoff são tecnologia" |
| **Drenagem** e publicação | `app` | "É um processo próprio, com composition root e lifecycle" |

E a blindagem que esta spec não pode furar: "O `application service` grava a
outbox **por uma porta**, nunca tocando a tabela: a célula 11
(`application → provider`) permanece proibida, e a gravação da outbox não é
exceção a ela."

**Modos de verificação — `docs/dmpf/uow-inbox-outbox.md:2351-2355`**

Todas as `OBX-*` são `runtime-testable`, **exceto** `OBX-02` e `OBX-03`, que
são `structurally reviewable` por inspeção. Nenhuma `OBX-*` é
`import-verifiable`.

<constraints>
- [P0] A publicação NUNCA ocorre dentro de transação de banco. `FOR UPDATE SKIP LOCKED` é admitido SOMENTE durante o claim (`OBX-07`).
- [P0] A transição final SEMPRE é condicional a `locked_by` ainda ser o claim corrente; escrita de claim substituído é rejeitada sem alterar o registro (`OBX-10`, `OBX-11`).
- [P0] NUNCA existe caminho de código que escreva `status = 'pending'` sobre um registro em `publishing`. A devolução ao pool é por comparação de prazos (`OBX-04`).
- [P0] A transação de claim SEMPRE grava `status`, `locked_by`, `locked_until` e o incremento de `attempt_count` no mesmo commit (`OBX-16`).
- [P0] `locked_by` SEMPRE identifica a execução do claim, nunca o processo: readquirir o mesmo registro produz identidade nova (`OBX-08`).
- [P0] O relay NUNCA lê o conteúdo de `payload` para decidir publicar (`OBX-14`).
- [P0] Nenhum artefato desta entrega declara ou sugere exactly-once fim a fim (`P0-3`, `GAR-01`).
- [P0] `last_error` NUNCA contém payload de negócio, credencial ou stack trace com dado de usuário (`OBX-03`).
- [P1] O `application service` NUNCA toca a tabela de outbox: a célula 11 (`application → provider`) permanece proibida (RFC §7.5).
- [P1] Esta spec NÃO fixa nome de métrica, unidade nem limiar — isso é de FND-08 sob ANC-06.
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] `envelope.Marshal` — o simétrico que falta**: acrescentar ao
  package `envelope` de `contracts` a função
  `Marshal(e Envelope) ([]byte, error)`, que valida o perfil, codifica via
  `Encode` e serializa o `*cloudeventsv1.CloudEvent` resultante.
  - Motivo declarado, o mesmo do godoc de `Unmarshal`: adaptadores que emitem
    bytes de transporte não devem importar o tipo gerado do CloudEvent.
  - `Marshal` e `Unmarshal` DEVEM ser inversas: `Unmarshal(Marshal(e))`
    reproduz `e` para todo `e` válido.
  - `Marshal` DEVE reprovar envelope inválido antes de serializar, com o
    mesmo erro que `Encode` retorna.
  - Alteração é de código Go em `contracts`, não de `.proto`: **não**
    dispara `buf-breaking`.

- [ ] **[P0] Índice de elegibilidade ao claim**: acrescentar a
  `libs/backend/go/postgres/schema.sql` o índice que serve à
  query de `OBX-09`.
  - O schema hoje tem apenas `dmpf_outbox_published_at_idx`, índice parcial
    sobre `published_at WHERE status = 'published'`, que serve à purga de
    `OBX-17` — não ao claim.
  - `Migrate()` (`migrate.go:17-20`) executa `schema.sql` embutido por
    `//go:embed`, com `CREATE ... IF NOT EXISTS`. Não há tabela de versão nem
    ferramenta de migration: o índice novo entra no mesmo arquivo, idempotente.
  - Nenhuma **coluna** precisa ser criada: `status`, `locked_by`,
    `locked_until`, `available_at`, `attempt_count`, `published_at` e
    `last_error` já existem em `schema.sql:1-30`.

- [ ] **[P0] Claim por lease no `provider`**: implementar em
  `libs/backend/go/postgres` a transação curta de claim.
  - Seleciona registros elegíveis por `OBX-09`: `available_at` já passado
    **e** (`status = 'pending'` **ou** `status = 'publishing'` com lease
    vencido).
  - `FOR UPDATE SKIP LOCKED` na seleção, dentro da transação de claim e só ali.
  - Grava no mesmo commit (`OBX-16`): `status = 'publishing'`, `locked_by`,
    `locked_until`, `attempt_count = attempt_count + 1`.
  - `locked_by` recebe identidade **nova a cada aquisição** (`OBX-08`), obtida
    de uma porta própria do relay. NÃO vem de `ports.IDGenerator`, que
    devolve `MessageID` — identidade de *mensagem*, não de *claim*.
  - A transação fecha **antes** de qualquer I/O com o broker (`OBX-07`).
  - Ordenação por `available_at, id` — FIFO estável, com `id` desempatando
    registros de mesmo instante.
  - Batch limitado por parâmetro; paginação por repetição do claim, não por
    cursor aberto (um cursor manteria transação viva durante o I/O).

- [ ] **[P0] As três transições finais, condicionais ao claim**: implementar
  em `postgres` as três escritas do passo 3, todas com
  `WHERE id = $1 AND locked_by = $2` (`OBX-10`).
  - **3a — sucesso**: `status = 'published'`, `published_at` gravado,
    `last_error` limpo.
  - **3b — falha transitória**: `status` **permanece** `publishing`
    (`OBX-04`), `available_at` recalculado por backoff e `locked_until`
    liberado **no mesmo commit** (`OBX-18`), `last_error` sanitizado.
  - **3c — tentativas esgotadas**: `status = 'failed'`, `last_error`
    sanitizado. Terminal para o ciclo automático (`OBX-06`).
  - Cada transição DEVE reportar ao chamador se afetou zero linhas — zero
    linhas significa claim substituído, e o relay registra o ocorrido e segue
    **sem republicar**.

- [ ] **[P0] Montagem do envelope na drenagem**: o relay monta o CloudEvent a
  partir das colunas, conforme `docs/adr/035-...md:40-44`.
  - `message_id` → `ID`; `message_type` → `Type`; `schema_version` →
    `DataSchema`; `occurred_at` → `Time`; `partition_key` → `PartitionKey`;
    `aggregate_version` → `AggregateVersion`; `metadata` fornece os atributos
    condicionais.
  - `payload` entra como `Envelope.Payload` **sem desserializar e sem
    reserializar** — são os bytes do `Any` congelados na escrita
    (`envelope.Pack`, `outbox.go:58-64`).
  - O envelope montado é validado (`Envelope.Validate`) antes de serializar;
    envelope inválido é falha do registro, não do lote.

- [ ] **[P0] Conferência de `payload_hash` na drenagem**: antes de publicar, o
  relay recalcula o hash sobre os bytes de `payload` e compara com a coluna
  `payload_hash`.
  - Obrigação de `docs/adr/035-...md:163-165`.
  - Divergência é corrupção em repouso, não falha transitória: o registro vai
    para `failed` sem novo backoff e **sem incremento além do que o claim já
    fez** (`OBX-16` incrementa `attempt_count` em toda aquisição, e nada é
    decrementado); `last_error` registra a divergência **sem** reproduzir os
    bytes.
  - Conferir o hash não é interpretar o payload: `OBX-14` proíbe decidir
    publicar **pelo conteúdo**, e a conferência é sobre integridade, não
    sobre conteúdo.

- [ ] **[P0] Porta de publicação declarada no relay**: o bloco `port`
  (`ports/outbox.go:36-38`) declara explicitamente que "this block
  declares no publishing port, because publishing happens after the commit and
  outside the unit of work". A porta de publicação é, portanto, declarada
  **no consumidor** — no pacote do relay, como manda o idioma Go.
  - Assinatura mínima: destino lógico e bytes do envelope.
  - O `KRN-08` entrega a interface e uma realização em memória para teste.
    Transportes concretos (Kafka, SNS/SQS) são do `KRN-10`.

- [ ] **[P0] Laço de drenagem no bloco `app`**: implementar em
  `libs/backend/go/app/relay/` o processo de drenagem.
  - Ciclo: claim de um lote → para cada registro, montar, conferir hash,
    publicar, transicionar → aguardar o intervalo de varredura → repetir.
  - Limite de concorrência configurável, com o relay nunca sendo fonte de
    saturação do broker nem do banco.
  - Backoff exponencial **com jitter** no recálculo de `available_at`.
  - Teto de tentativas configurável, que dispara a transição 3c.
  - Intervalo de varredura, tamanho de lote, prazo de lease, teto de
    tentativas e limite de concorrência são **declarados pelo chamador** —
    nenhum valor operacional é fixado no código (mesma regra que
    `app/doc.go:16-17` já aplica ao consumer).

- [ ] **[P0] Exposição dos quatro sinais** (`OBX-12`): o relay expõe
  `pending`, `lag`, `attempts` e `failures` por uma superfície observável.
  - `pending` e `lag` derivam de consulta ao banco sobre o conjunto elegível;
    `attempts` e `failures` são contadores da execução.
  - A superfície é uma interface do relay, com realização de teste no
    `KRN-08`. O binding a OpenTelemetry é do `KRN-09`.
  - Esta spec **não** define nome de métrica, unidade nem limiar — FND-08 sob
    ANC-06.

- [ ] **[P0] Graceful shutdown** (`OBX-13`): ao receber sinal de encerramento,
  o relay para de reivindicar novos lotes, conclui ou libera os claims vivos, e
  só então encerra.
  - "Liberar" um claim vivo é a transição 3b com `available_at` imediato: o
    registro volta ao pool sem esperar o lease.
  - Encerrar com claim órfão não perde mensagem, mas atrasa a drenagem pelo
    prazo do lease — e é isso que o requisito impede.

- [ ] **[P1] Composition root e binário**: `cmd/dmpf-relay/` monta pool,
  relógio, gerador de identidade, provider de claim, publisher e o laço, e
  traduz sinal do sistema operacional em encerramento gracioso.
  - Segue o precedente de `libs/backend/go/conformance/cmd`.
  - `libs/backend/go/app/example/reservations/consumer.go:9` já importa
    `pgxpool` em código de produção com o workspace conforme, então o
    composition root do relay não inaugura dependência nova no módulo.

- [ ] **[P1] Manifesto e baseline**: declarar as unidades novas em
  `libs/backend/go/app/dmpf-units.json` e regravar o baseline.
  - Unidades propostas: `kernel/app-relay` (o pacote `relay/`) e
    `kernel/app-relay-cmd` (o binário), ambas bloco `app`,
    `bounded_context: kernel`.
  - O baseline **nunca** é editado à mão: seu `digest` é SHA-256 sobre
    codificação length-prefixed dos campos, não hash do texto JSON. Regravar
    por `conformance --write-baseline --root .`, comando que
    `cmd/conformance/main.go:32` declara "NUNCA usar no gate".

### Não-funcionais

- [ ] **Correção sob concorrência**: dois relays concorrentes sobre o mesmo
  pool nunca publicam o mesmo registro no mesmo ciclo, e o claim substituído
  nunca altera o registro. Verificado por teste de corrida, não por revisão.
- [ ] **Nenhuma conexão presa durante I/O**: comprovado por teste que
  observa o pool durante a publicação — a métrica de conexões em uso do
  `pgxpool` não acusa conexão adquirida enquanto o publisher está bloqueado.
- [ ] **Cadeia Go verde**: `gofmt`, `go vet`, `golangci-lint`, `go build`,
  `go test`, `go test -race`, `govulncheck`.
- [ ] **Gates DMPF verdes**: `conformance` conforme;
  `tools/dmpf-gate-check.sh` e `tools/dmpf-cell-check.sh` passando.
- [ ] **Compatibilidade**: Go 1.26.6 (piso do `go.work`), `pgx/v5 >=5.10 <6`,
  PostgreSQL 16 (imagem do CI).
- [ ] **Segurança**: `last_error` sanitizado; nenhum dado de negócio,
  credencial ou stack trace com dado de usuário na coluna, nos logs ou nos
  sinais.

## Camadas afetadas

| Camada | Afetada? | Descrição |
| --- | --- | --- |
| `domain` (`domain`) | [ ] Não | O relay não conhece agregado nem evento de domínio; opera sobre colunas |
| `port` (`ports`) | [ ] Não | A porta de publicação é declarada no consumidor, porque publicar acontece fora da UoW — decisão já registrada em `ports/outbox.go:36-38` |
| `contract` (`contracts`) | [x] Sim | `envelope.Marshal`, o simétrico de `Unmarshal` que hoje não existe |
| `application` (`application`) | [ ] Não | A drenagem não é caso de uso; RFC §7.5 a aloca ao `app` |
| `provider` (`postgres`) | [x] Sim | Claim por lease, as três transições condicionais, a query de elegibilidade, o índice novo e as consultas dos sinais `pending` e `lag` |
| `app` (`app`) | [x] Sim | Laço de drenagem, concorrência, backoff, sinais, graceful shutdown, porta de publicação e composition root |
| Schema (`schema.sql`) | [x] Sim | Um índice novo. Nenhuma coluna nova |
| CI (`.github/workflows/ci.yml`) | [ ] Não | `:92` roda `nx affected -t fmt-check,vet,test-race,govulncheck` e `app` já está na cadeia; os gates DMPF já rodam em `:111`, `:117` e `:128-131` |

## Localização de código

```text
libs/backend/go/contracts/
  envelope/
    envelope.go            — MODIFICAR: +Marshal(Envelope) ([]byte, error)
    envelope_test.go       — MODIFICAR: +round-trip Marshal/Unmarshal

libs/backend/go/postgres/
  schema.sql               — MODIFICAR: +índice de elegibilidade ao claim
  claim.go                 — NOVO: Claim, as três transições, tipo Claimed
  claim_test.go            — NOVO: //go:build integration
  signals.go               — NOVO: consultas de pending e lag
  signals_test.go          — NOVO: //go:build integration
  errors.go                — MODIFICAR: +sentinelas do claim

libs/backend/go/app/
  relay/
    relay.go               — NOVO: laço, concorrência, backoff, shutdown
    publisher.go           — NOVO: porta de publicação (interface no consumidor)
    signals.go             — NOVO: superfície dos quatro sinais
    doc.go                 — NOVO: godoc do pacote
    relay_test.go          — NOVO: unitário, publisher e store falsos
  cmd/dmpf-relay/
    main.go                — NOVO: composition root + sinal do SO
  example/reservations/
    relay_e2e_test.go      — NOVO: //go:build integration, e2e sobre Postgres
  dmpf-units.json          — MODIFICAR: +unidades do relay
  project.json             — INTOCADO: `build` é `go build ./...`, que cobre cmd/

tools/dmpf-baseline/
  units-baseline.json      — REGRAVAR via --write-baseline (nunca à mão)
```

**Arquivos a modificar, e o que muda**:

- `libs/backend/go/contracts/envelope/envelope.go` — acrescenta
  `Marshal`, simétrico de `Unmarshal` (`:226-235`), pelo motivo que o godoc
  daquela função já declara.
- `libs/backend/go/postgres/schema.sql` — acrescenta o índice
  parcial de elegibilidade. O arquivo já tem `dmpf_outbox_published_at_idx`
  (`:28-29`) como precedente de índice parcial.
- `libs/backend/go/postgres/errors.go` — acrescenta as
  sentinelas do claim, seguindo o padrão de `ErrDuplicateMessage` (`:30`), que
  encapsula o erro do driver com `%w: %w`.
- `libs/backend/go/app/dmpf-units.json` — hoje declara duas unidades e
  `external: []`; passa a declarar as do relay.

## Design

### Arquitetura

```text
                    ┌──────────────────────────────────────┐
                    │  cmd/dmpf-relay  (composition root)   │
                    │  pool · clock · ids · sinal do SO     │
                    └──────────────┬───────────────────────┘
                                   │ monta
                    ┌──────────────▼───────────────────────┐
   bloco app        │  relay.Relay                          │
                    │  laço · concorrência · backoff        │
                    │  jitter · teto · shutdown             │
                    └───┬──────────────────────────┬────────┘
                        │ Store (porta)            │ Publisher (porta)
                        │                          │  declarada aqui, no
                        │                          │  consumidor
      ┌─────────────────▼──────────────┐    ┌──────▼─────────────────┐
      │  postgres        │    │  KRN-10: Kafka, SQS    │
      │  Claim  (txn curta, SKIP       │    │  KRN-08: em memória,   │
      │         LOCKED, OBX-16)        │    │          para teste    │
      │  MarkPublished / Reschedule /  │    └────────────────────────┘
      │  Fail  (WHERE locked_by, 10)   │
      │  Pending / Lag  (sinais)       │
      └─────────────────┬──────────────┘
                        │
                 ┌──────▼────────┐        ┌───────────────────────────┐
                 │  dmpf_outbox  │        │  contracts/envelope  │
                 │  (KRN-06)     │        │  Marshal (NOVO) · Encode  │
                 └───────────────┘        └───────────────────────────┘
```

A fronteira que o desenho respeita: o `provider` guarda **tudo que é
tecnologia** — a tabela, o `SKIP LOCKED`, a aritmética do backoff aplicada às
colunas —, e o `app` guarda **tudo que é processo** — o laço, a concorrência,
o lifecycle e a decisão de quando desistir. É a divisão literal de RFC `§7.5`.

O relay não importa `application` nem `domain`. A linha `app` da
matriz permite as duas arestas (células 13 e 14, ambas `P`), mas o relay não
tem o que fazer com elas: opera sobre colunas, e `OBX-14` o proíbe de
interpretar o conteúdo.

### Fluxo principal — os três passos de §5.4

1. **Claim** — transação curta. Seleciona até `batch` registros elegíveis por
   `OBX-09`, com `FOR UPDATE SKIP LOCKED`, e grava no mesmo commit
   `status = 'publishing'`, `locked_by` com identidade nova, `locked_until`
   com o prazo do lease e `attempt_count + 1` (`OBX-16`). **Commita.**
2. **Publicação** — fora de qualquer transação (`OBX-07`). Para cada registro
   do lote: montar o envelope a partir das colunas, conferir `payload_hash`,
   `envelope.Marshal`, entregar ao `Publisher`.
3. **Transição** — nova transação curta, condicional a `locked_by`
   (`OBX-10`), em um dos três desfechos:
   - **3a** publicado: `status = 'published'`, `published_at` gravado.
   - **3b** falha transitória: permanece `publishing`, `available_at`
     recalculado por backoff, `locked_until` liberado no mesmo commit
     (`OBX-18`).
   - **3c** tentativas esgotadas: `status = 'failed'` (`OBX-06`).

Entre 2 e 3 existe a janela que produz republicação. Ela é **aceitável e
esperada** (`:924-928`), é uma das duas origens do at-least-once, e o
consumidor a absorve pela inbox do `KRN-07` — a mensagem republicada chega com
o mesmo `message_id`.

### Índice de elegibilidade e a query de claim

O schema atual tem um único índice sobre `dmpf_outbox`, parcial, para a purga:

```sql
-- schema.sql:28-29, existente
CREATE INDEX IF NOT EXISTS dmpf_outbox_published_at_idx
  ON dmpf_outbox (published_at) WHERE status = 'published';
```

O claim precisa do seu, e ele é o complemento exato: cobre os dois estados que
`OBX-09` torna elegíveis e ordena pela chave da varredura.

```sql
-- schema.sql, NOVO
CREATE INDEX IF NOT EXISTS dmpf_outbox_claim_idx
  ON dmpf_outbox (available_at, id)
  WHERE status IN ('pending', 'publishing');
```

Os dois índices são disjuntos por construção: um cobre `status = 'published'`,
o outro cobre `pending` e `publishing`. `failed` fica fora de ambos, e é o
comportamento correto — `OBX-06` o torna terminal, e nenhuma varredura
automática deve alcançá-lo.

A query de claim, com o predicado literal de `OBX-09`:

```sql
SELECT id, message_id, message_type, schema_version,
       aggregate_type, aggregate_id, aggregate_version,
       partition_key, destination, payload, payload_hash, metadata,
       occurred_at, attempt_count
  FROM dmpf_outbox
 WHERE available_at <= $1
   AND ( status = 'pending'
      OR ( status = 'publishing'
           AND (locked_until IS NULL OR locked_until <= $1) ) )
 ORDER BY available_at, id
 LIMIT $2
 FOR UPDATE SKIP LOCKED
```

**A cláusula `locked_until IS NULL` é correção-crítica, e não é óbvia.**
`OBX-18` manda liberar `locked_until` no desfecho transitório. Se "liberar"
for gravar `NULL`, então a comparação `locked_until <= $1` avalia como `NULL`
— que em SQL não é verdadeiro —, e o registro **nunca voltaria ao pool**: o
backoff recalculado em `available_at` seria inerte e a mensagem ficaria presa
em `publishing` para sempre. O predicado precisa aceitar explicitamente o
lease ausente. A alternativa está avaliada em Decisões técnicas.

### Pseudocódigo — o ciclo e as três transições

```text
// bloco app — o laço
funcao Drenar(ctx):
    enquanto nao encerrando:
        lote = store.Claim(ctx, agora(), batch, prazoDoLease, ids.Novo())
        se lote vazio:
            aguardar(intervaloDeVarredura)   // ou encerrar, se sinalizado
            continuar

        para cada registro em lote, com limite de concorrencia:
            publicar(ctx, registro)

        expor(sinais)

funcao publicar(ctx, r):
    // OBX-14: nada aqui olha DENTRO de r.Payload
    se payloadhash.Soma(r.Payload) != r.PayloadHash:
        // corrupcao em repouso: nao e transitorio, nao gasta tentativa
        store.Fail(ctx, r.ID, r.LockedBy, "payload_hash divergente")
        sinais.Falha()
        retornar

    env = montarEnvelope(r)            // colunas -> Envelope (ADR-035)
    bytes, err = envelope.Marshal(env) // valida o perfil e serializa
    se err != nil:
        store.Fail(ctx, r.ID, r.LockedBy, sanitizar(err))
        sinais.Falha()
        retornar

    // OBX-07: daqui ate o retorno, NENHUMA conexao de banco esta em uso
    err = publisher.Publish(ctx, r.Destination, bytes)

    se err == nil:
        afetadas = store.MarkPublished(ctx, r.ID, r.LockedBy, agora())
    senao se r.AttemptCount >= tetoDeTentativas:
        afetadas = store.Fail(ctx, r.ID, r.LockedBy, sanitizar(err))
        sinais.Falha()
    senao:
        proxima = agora() + backoffComJitter(r.AttemptCount)
        afetadas = store.Reschedule(ctx, r.ID, r.LockedBy, proxima, sanitizar(err))

    se afetadas == 0:
        // OBX-10 e OBX-11: o claim nao e mais nosso. Registrar e seguir.
        // NUNCA republicar: outro claim ja assumiu o registro.
        sinais.ClaimSubstituido()
```

```text
// bloco provider — as tres transicoes, todas com a mesma guarda
MarkPublished:  UPDATE ... SET status='published', published_at=$3, last_error=NULL
                 WHERE id=$1 AND locked_by=$2

Reschedule:     UPDATE ... SET available_at=$3, locked_until=NULL, last_error=$4
                 WHERE id=$1 AND locked_by=$2
                 -- status PERMANECE 'publishing' (OBX-04)
                 -- locked_until e available_at no MESMO commit (OBX-18)

Fail:           UPDATE ... SET status='failed', last_error=$3
                 WHERE id=$1 AND locked_by=$2

// as tres devolvem RowsAffected ao chamador; zero significa claim substituido
```

O encerramento gracioso (`OBX-13`) usa a mesma `Reschedule`, com
`available_at` no instante corrente: o claim vivo é liberado e o registro
volta ao pool imediatamente, em vez de esperar o lease vencer.

### Onde cada regra é provada

| Regra | Modo | Onde é provada |
| --- | --- | --- |
| `OBX-03` | `structurally reviewable` | Inspeção: nenhuma chamada grava erro bruto em `last_error`; toda escrita passa por `sanitizar` |
| `OBX-04` | `runtime-testable` | Teste que varre o código do provider por `SET status='pending'` e teste que confirma o registro em `publishing` após 3b |
| `OBX-05` | `runtime-testable` | Já provado no `KRN-06`; reafirmado por teste que claima um registro recém-escrito sem espera |
| `OBX-06` | `runtime-testable` | Teste: esgotadas as tentativas, `status='failed'` e o próximo claim não o alcança |
| `OBX-07` | `runtime-testable` | Teste com publisher bloqueante que observa o `pgxpool.Stat().AcquiredConns()` durante o I/O |
| `OBX-08` | `runtime-testable` | Teste: mesmo worker readquire o mesmo registro e recebe `locked_by` diferente |
| `OBX-09` | `runtime-testable` | Teste de elegibilidade nos quatro casos: `pending`, `publishing` com lease vencido, `publishing` com lease vivo, `available_at` futuro. Inclui o caso `locked_until IS NULL` |
| `OBX-10` | `runtime-testable` | Teste de corrida: relay A claima, lease vence, relay B claima e publica, A tenta transicionar e é rejeitado |
| `OBX-11` | `runtime-testable` | O mesmo teste, verificando que o registro está **inalterado** após a rejeição de A |
| `OBX-12` | `runtime-testable` | Teste que exercita a superfície dos quatro sinais e confere os valores contra o estado do banco |
| `OBX-13` | `runtime-testable` | Teste: sinaliza encerramento com claims vivos e confirma que nenhum registro fica em `publishing` com lease vivo |
| `OBX-14` | `structurally reviewable` | Inspeção: o único acesso a `payload` no relay é o cálculo do hash e a cópia para `Envelope.Payload` |
| `OBX-16` | `runtime-testable` | Teste que lê o registro após o claim e confirma os quatro campos; e teste que nenhum registro em `publishing` tem `locked_until` nulo **logo após** o claim |
| `OBX-17` | — | Fora do escopo: a purga é operação com evidência, não ciclo automático |
| `OBX-18` | `runtime-testable` | Teste: após 3b com backoff menor que o lease, o próximo claim respeita o **backoff**, não o lease restante |
| `V31` / `GAR-01` | `structurally reviewable` | Varredura de documentação, código e configuração desta entrega por promessa de exactly-once |
| `V32` | `runtime-testable` | E2E que força republicação na janela 2→3 e confirma que o consumidor do `KRN-07` a absorve |
| `BLK-02` | `structurally reviewable` | O relay tem `cmd/` próprio, separado de qualquer processo de request |

## Decisões técnicas

- **Relay em `libs/backend/go/app`, não em `apps/backend/`**: o módulo já
  é o bloco `app` do kernel, seu `doc.go:15` reserva nominalmente o lugar do
  relay, e seu `test-race` já declara `dependsOn` sobre o do
  `postgres` — os dois compartilham o Postgres do job.
  Precedente de binário dentro de lib: `libs/backend/go/conformance/cmd`.
  Alternativa descartada: `apps/backend/dmpf-relay`, que inauguraria `apps/`
  sem generator Go documentado e cujo nome no ticket (`reference`)
  colidiria com o composition root de exemplo do `KRN-12`.

- **Porta de publicação declarada no relay, não em `ports`**: o godoc de
  `ports/outbox.go:36-38` já decidiu que aquele bloco não declara porta de
  publicação, "because publishing happens after the commit and outside the
  unit of work". Declarar a interface no consumidor é também o idioma Go —
  "accept interfaces, return structs". Alternativa descartada: acrescentar
  `Publisher` a `ports`, que contradiria uma decisão já registrada em
  código e ampliaria a superfície fechada daquele bloco sem necessidade.

- **`locked_until = NULL` no desfecho transitório, com o predicado aceitando
  `IS NULL`**: `OBX-18` manda liberar o lease; `NULL` é a representação
  honesta de "sem lease". A alternativa — gravar `locked_until` igual ao novo
  `available_at` — evitaria a cláusula extra no predicado, mas passaria a
  afirmar que existe um lease até aquele instante, o que é falso, e
  reintroduziria a confusão entre prazo de lease e prazo de backoff que
  `OBX-18` existe para desfazer. O custo da decisão é uma cláusula a mais na
  query, e o teste que a cobre é obrigatório.

- **`payload_hash` divergente vai direto para `failed`, sem consumir
  backoff**: retentar não conserta bytes corrompidos em repouso. Tratar
  corrupção como falha transitória gastaria o teto de tentativas e atrasaria a
  descoberta do problema. Alternativa descartada: tratar como transitória "por
  simetria" com os demais erros.

- **`envelope.Marshal` em `contracts`, não no relay**: manter a
  serialização do CloudEvent dentro do bloco `contract` preserva a propriedade
  que o godoc de `Unmarshal` já enuncia — adaptadores não importam o tipo
  gerado. Alternativa descartada: `Encode` + `proto.Marshal` no relay, que
  faria o bloco `app` importar `google.golang.org/protobuf` e o tipo gerado do
  CloudEvent, quebrando a simetria com o caminho de entrada do `KRN-07`.

- **Ordenação `available_at, id`**: FIFO estável. `available_at` sozinho
  empata para registros escritos no mesmo instante, e o empate faria a ordem
  variar entre execuções, dificultando o diagnóstico de backlog. `id` é
  `GENERATED ALWAYS AS IDENTITY`, monotônico, e desempata sem custo.

- **Paginação por repetição do claim, não por cursor**: um cursor aberto
  manteria transação viva durante o I/O, violando `OBX-07`. Cada ciclo abre e
  fecha a sua transação de claim.

- **Sinais expostos por interface do relay, sem OpenTelemetry**: `OBX-12`
  obriga a **exposição**; o binding é de FND-08 sob ANC-06, e a
  instrumentação OTel é do `KRN-09`. Entregar OTel aqui anteciparia decisão de
  outra spec e fixaria nomes de métrica que este artefato não pode fixar.

- **Nenhum target Nx novo e nenhum step de CI novo**: `libs/backend/go/conformance/project.json` é o precedente — tem `cmd/` e não declara target próprio para o binário, porque `build` já é `go build ./...`. E `.github/workflows/ci.yml:92` roda `nx affected` sobre os targets existentes, com `app` já na cadeia e os gates DMPF em `:111`, `:117` e `:128-131`. Acrescentar target ou step seria redeclarar o que já existe, contra a convenção do `AGENTS.md` de não redeclarar o que `targetDefaults` ou plugin já fornece.

- **Índice parcial em vez de índice total**: `failed` é terminal e nenhuma
  varredura automática o alcança; incluí-lo no índice custaria escrita e
  espaço sem servir a nenhuma consulta.

## Regras relacionadas

- `docs/dmpf/uow-inbox-outbox.md` — FND-04, §2.1, §4.1 a §4.3, §5.1 a §5.5 e
  §7.1. Fonte de `BLK-02` e de todas as `OBX-*`.
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §2.3 (`P0-3`), §7.1 a §7.5
  (função `decide`, matriz de blocos, bloco dono do outbox).
- `docs/adr/010-regra-de-dependencia-e-seis-blocos.md` — a matriz que autoriza
  as arestas do relay.
- `docs/adr/020-relay-polling-leasing-e-cdc-extensao.md` — o mecanismo.
- `docs/adr/021-mapeamento-no-provider-serializacao-na-escrita.md` — os bytes
  congelam na escrita.
- `docs/adr/025-kafka-transporte-alvo-sns-sqs-acervo.md` — a dedup do broker
  atua na publicação com janela, e **não** substitui as garantias de entrega
  (`:62-63`).
- `docs/adr/035-realizacao-postgres-da-outbox.md` — o schema, a montagem do
  CloudEvent na drenagem (`:40-44`) e a conferência de `payload_hash`
  (`:163-165`).
- `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md` — o relay
  drena também a outbox derivada do consumo (`:173-174`).
- `AGENTS.md` — convenções do workspace, cadeia de validação e tags 3D.
- **ADR novo: `038`** — o `037` foi tomado pelo `KRN-09`, que mergeou em
  paralelo a esta entrega. Registra as decisões
  desta spec que divergem da fundação ou a completam: a representação
  `NULL` do lease liberado com o predicado correspondente, o tratamento de
  `payload_hash` divergente como terminal, e `envelope.Marshal` como
  simétrico exigido pela drenagem.

## Verificação e testes

### Critérios de aceite

Os dez primeiros são os do ticket `ARQ-527`, verbatim; os seguintes derivam
das fontes normativas e das divergências reconciliadas acima.

- [ ] A publicação ocorre fora de qualquer transação de banco, com teste
  comprovando que nenhuma conexão fica presa durante o I/O.
- [ ] A transação de claim grava os quatro campos no mesmo commit; um registro
  em `publishing` sem `locked_until` nunca é produzido.
- [ ] Dado um registro cujo lease expirou, quando um segundo relay o reivindica
  e publica, o primeiro não consegue transicioná-lo: a escrita é rejeitada e
  nada no registro é alterado.
- [ ] Um mesmo worker que readquira o mesmo registro recebe identidade de claim
  diferente da anterior.
- [ ] Não existe caminho de código que escreva `pending` sobre um registro em
  `publishing`: a devolução ao pool é por comparação de prazos.
- [ ] No desfecho transitório, `locked_until` é liberado no mesmo commit do
  recálculo de `available_at`, e o próximo claim respeita o backoff, não o
  restante do lease.
- [ ] Esgotadas as tentativas, o registro vai para `failed` e não retorna ao
  pool sem intervenção.
- [ ] O relay não lê o `payload` para decidir publicar; os quatro sinais são
  observáveis; o graceful shutdown não deixa claim órfão.
- [ ] Uma falha entre publicar e marcar produz republicação no ciclo seguinte,
  declarada no teste como duplicata esperada sob at-least-once.
- [ ] Nenhum artefato desta entrega declara ou sugere exactly-once fim a fim, e
  `last_error` não contém payload de negócio, credencial nem stack trace com
  dado de usuário.
- [ ] `envelope.Marshal` existe, valida o perfil antes de serializar, e é
  inversa de `envelope.Unmarshal` para todo envelope válido.
- [ ] O relay monta o CloudEvent a partir das colunas conforme
  `docs/adr/035-...md:40-44`, sem desserializar nem reserializar `payload`.
- [ ] `payload_hash` é conferido antes de publicar; divergência leva o registro
  a `failed` sem novo backoff e sem incremento além do que o claim já fez, e
  `last_error` descreve a divergência sem reproduzir os bytes.
- [ ] O índice de elegibilidade existe em `schema.sql` e a query de claim o
  utiliza — comprovado por `EXPLAIN` no teste de integração.
- [ ] Um registro em `publishing` com `locked_until IS NULL` e `available_at`
  passado é elegível ao claim.
- [ ] Intervalo de varredura, tamanho de lote, prazo de lease, teto de
  tentativas e limite de concorrência são declarados pelo chamador; nenhum
  valor operacional está fixado no código do relay.
- [ ] `conformance --root . --base develop` reporta conforme com as
  unidades novas declaradas, e o baseline foi regravado por `--write-baseline`,
  nunca à mão.
- [ ] `tools/dmpf-gate-check.sh` e `tools/dmpf-cell-check.sh` passam.
- [ ] Cadeia Go verde: `gofmt`, `go vet`, `golangci-lint`, `go build`,
  `go test`, `go test -race`, `govulncheck`.
- [ ] `pnpm biome ci .` e `pnpm nx affected -t lint,typecheck,test,build`
  verdes.
- [ ] Critérios verificados no CI, não apenas localmente.
- [ ] Resultado do incremento 3 demonstrado com `KRN-05` a `KRN-07`: mensagem
  publicada atomicamente e consumida com deduplicação.
- [ ] Divergência com a fundação registrada como ADR `038`.

### Cenários de teste

```text
DADO um registro recém-escrito por Enqueue, com status 'pending',
     available_at igual a occurred_at e attempt_count zero
QUANDO o relay executa um ciclo de drenagem com o publisher devolvendo sucesso
ENTÃO o registro fica com status 'published', published_at preenchido,
     last_error nulo, attempt_count igual a 1, e o publisher recebeu
     exatamente uma vez os bytes de um CloudEvent cujo id é o message_id
     e cujo data é o payload gravado, byte a byte
```

```text
DADO dois relays A e B sobre o mesmo pool, e um registro reivindicado por A
     com lease de 100ms
QUANDO o lease de A expira, B reivindica o mesmo registro, B publica e marca
     como 'published', e só então A retorna do broker e tenta marcar
ENTÃO a escrita de A afeta zero linhas, o registro permanece 'published' com
     o locked_by de B, published_at é o de B, e A registra claim substituído
     sem republicar
```

```text
DADO um registro reivindicado, com lease de 60s e backoff configurado em 2s
QUANDO a publicação falha de forma transitória
ENTÃO no mesmo commit o status permanece 'publishing', available_at passa a
     agora+2s (mais jitter) e locked_until fica nulo;
     E um claim executado 3s depois reivindica o registro — provando que o
     backoff governa, e não o lease restante de 57s
```

```text
DADO um registro cuja coluna payload_hash foi adulterada para não
     corresponder aos bytes de payload
QUANDO o relay tenta publicá-lo
ENTÃO o publisher NÃO é chamado, o registro vai para 'failed', attempt_count
     não é incrementado além do claim, e last_error descreve a divergência
     sem conter nenhum byte do payload
```

```text
DADO um relay com um publisher que bloqueia por 500ms
QUANDO um lote de 10 registros é publicado
ENTÃO pgxpool.Stat().AcquiredConns() lido durante o bloqueio é zero,
     provando que nenhuma conexão fica presa durante o I/O
```

```text
DADO um relay com claims vivos sobre 5 registros e lease de 60s
QUANDO o contexto é cancelado e o encerramento gracioso executa
ENTÃO o relay não reivindica novos lotes, os 5 registros ficam com
     locked_until nulo e available_at no passado — imediatamente elegíveis —,
     e o processo encerra em menos que o prazo do lease
```

```text
DADO um registro publicado no broker mas cuja marcação falhou (falha na
     janela entre os passos 2 e 3)
QUANDO o ciclo seguinte reivindica e republica o registro
ENTÃO o consumidor do KRN-07 recebe a mesma mensagem com o mesmo message_id
     e a absorve pela inbox como redelivery, sem efeito duplicado
     — a duplicata é declarada no teste como esperada sob at-least-once (V32)
```

```text
DADO um registro em 'publishing' com locked_until nulo e available_at passado
QUANDO o claim é executado
ENTÃO o registro é reivindicado — cobrindo a cláusula IS NULL do predicado,
     sem a qual o registro ficaria preso em 'publishing' para sempre
```

```text
DADO um banco com 7 registros elegíveis, 2 em 'publishing' com lease vivo
     e 3 em 'failed'
QUANDO os sinais são observados
ENTÃO pending conta apenas os 7 elegíveis, lag reflete o occurred_at mais
     antigo entre eles, failures conta as falhas da execução corrente,
     e os registros em 'failed' não entram em pending
```

<critical_constraints>
- [P0] A publicação NUNCA ocorre dentro de transação de banco. `FOR UPDATE SKIP LOCKED` é admitido SOMENTE durante o claim (`OBX-07`).
- [P0] A transição final SEMPRE é condicional a `locked_by` ainda ser o claim corrente; escrita de claim substituído é rejeitada sem alterar o registro (`OBX-10`, `OBX-11`).
- [P0] NUNCA existe caminho de código que escreva `status = 'pending'` sobre um registro em `publishing`. A devolução ao pool é por comparação de prazos (`OBX-04`).
- [P0] A transação de claim SEMPRE grava `status`, `locked_by`, `locked_until` e o incremento de `attempt_count` no mesmo commit (`OBX-16`).
- [P0] `locked_by` SEMPRE identifica a execução do claim, nunca o processo (`OBX-08`).
- [P0] O relay NUNCA lê o conteúdo de `payload` para decidir publicar (`OBX-14`).
- [P0] Nenhum artefato desta entrega declara ou sugere exactly-once fim a fim (`P0-3`, `GAR-01`).
- [P0] `last_error` NUNCA contém payload de negócio, credencial ou stack trace com dado de usuário (`OBX-03`).
- [P0] O predicado de elegibilidade SEMPRE aceita `locked_until IS NULL`, sob pena de registro preso em `publishing` para sempre.
- [P1] O `application service` NUNCA toca a tabela de outbox: a célula 11 (`application → provider`) permanece proibida (RFC §7.5).
- [P1] Esta spec NÃO fixa nome de métrica, unidade nem limiar — isso é de FND-08 sob ANC-06.
- [P1] O baseline de unidades NUNCA é editado à mão: seu digest é SHA-256 sobre codificação length-prefixed, e regravá-lo é por `--write-baseline`.
</critical_constraints>

## Escopo fora

- **CDC e Debezium**: `OBX-15` os admite como extensão, nunca substituição, e o
  ticket os exclui explicitamente. Um contexto que adote CDC continua obrigado
  a tudo o que §4 e §7 estabelecem — muda apenas **como** o registro é
  descoberto.
- **Transportes concretos (Kafka, SNS/SQS, gRPC, REST)**: são do `KRN-10`. O
  `KRN-08` entrega a porta de publicação e uma realização em memória para
  teste.
- **ACK, nack, offset commit e dead-letter queue**: já declarados fora do bloco
  `app` em `app/doc.go:14-15`, e alocados ao `KRN-10`.
- **Instrumentação OpenTelemetry e retry por conjunção**: são do `KRN-09`. O
  `KRN-08` entrega a superfície dos sinais, não o binding de telemetria.
- **Nomes de métrica, unidades, limiares, alarmes e runbook de operação**: são
  de FND-08 sob ANC-06, por `encaminhado` explícito de FND-04 (`:880-882`).
- **Retomada de registros em `failed`**: `OBX-06` a define como operação com
  evidência, não ciclo automático.
- **Purga de registros `published`**: `OBX-17` a define como operação com
  evidência. O índice `dmpf_outbox_published_at_idx` que a serve já existe
  desde o `KRN-06`.
- **Infraestrutura de mensageria**: provisionamento de broker, tópicos,
  partições e políticas de retenção não são desta entrega.
- **Composition root de referência do SDK**: é do `KRN-12`, e é a ele que o
  nome `reference` do ticket pertence.
