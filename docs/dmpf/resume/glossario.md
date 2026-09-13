# Glossário — Legendas canônicas do DMPF

> **Uso:** Consulte este documento para a forma curta oficial de cada sigla, âncora, referência e prefixo do acervo DMPF.
>
> **Escopo:** Este é um arquivo de referência derivado do conteúdo de `docs/dmpf/`. Não normativa.

---

## Siglas e referências globais

| Forma longa | Forma curta | Legenda |
|--|--|--|
| Domain-and-Message Platform Foundation | `DMPF` | fundação normativa de arquitetura |
| Unidade de Processamento de Regra | `UPR` | operação atômica de decisão de negócio |
| Unit of Work | `UoW` | transação local e visível no service |
| Specification / Spec | `SPEC` | documento de requisitos e critérios de aceitação |
| Architecture Decision Record | `ADR` | registro de decisão de arquitetura |
| Issue / épico / tarefa | `ARQ` | number reference em Jira (`ARQ-436`, etc.) |

---

## Âncoras de extensão (ANC-01 a ANC-09)

As âncoras registram pontos de extensão da RFC onde futuras sub-specs adicionam conteúdo normatizado. Todos os nove artefatos "FND-" abaixo morrem por áncora.

| ID | Nome | Assunto | Artefato |
|--|--|--|--|
| `ANC-01` | Âncora de UPR e decisões | Modelo de mensagens, desfecho da UPR, taxonomia | FND-03 (`upr-decision-mensagens.md`) |
| `ANC-02` | Âncora de UoW e transação | Unit of Work, inbox, outbox, garantias | FND-04 (`uow-inbox-outbox.md`) |
| `ANC-03` | Âncora de contrato e wire | CloudEvents, Protobuf, Buf, repositório | FND-05 (`cloudevents-protobuf-buf.md`) |
| `ANC-04` | Âncora de transportes | REST, gRPC, Kafka, SNS/SQS, AsyncAPI | FND-06 (`politicas-transporte.md`) |
| `ANC-05` | Âncora de contexto e segurança | Execução, erros, classificação, permissões | FND-07 (`contexto-erros-seguranca.md`) |
| `ANC-06` | Âncora de resiliência | Timeout, retry, circuito, observabilidade | FND-08 (`resiliencia-observabilidade.md`) |
| `ANC-07` | Âncora de testes | Pirâmide, cenários, kits, fitness | FND-09 (`testes-interop.md`) |
| `ANC-08` | Âncora de autorização | Rito de controle de classificação | FND-10 (`governanca-bom-pilotos.md`) |
| `ANC-09` | Âncora de governança | Versões, pilotos, BOM, prontidão | FND-10 (`governanca-bom-pilotos.md`) |

---

## Prefixos de regras (por artefato)

Cada regra no acervo tem forma `PREFIXO-NN` (ex.: `UOW-06`, `ENV-13`). O prefixo identifica o artefato dono.

### RFC e inventário

| Prefixo | Artefato | Volume | Escopo |
|--|--|--|--|
| — | `rfc-dmpf-foundation-v0.1.md` | — | Limites, blocos, regra de dependência; normativa |
| — | `inventario-as-is.md` | — | Estado do parque; baseline candidato |

### FND-03 a FND-10 (adições por âncora)

| Prefixo | Artefato | Vol. | Exemplo | Escopo |
|--|--|--|--|--|
| `DEC` | FND-03 | 13 | `DEC-01` | Desfecho da UPR: exaustividade, exclusividade, rejeição |
| `CTR` | FND-03 | 7 | `CTR-01` | Níveis de contrato e fronteiras |
| `UPR-I` | FND-03 | 12 | `UPR-I1` | Invariantes da UPR (entrada/saída) |
| `UPR-L` | FND-03 | 5 | `UPR-L1` | Limites da UPR (o que não retém) |
| `MSG-N` | FND-03 | 4 | `MSG-N1` | Nomenclatura de evento/comando |
| `ESC` | FND-03 | 9 | `ESC-01` | Event Sourcing e CQRS (não pressupõe) |
| — | — | 54 (FND-03 total) | — | — |
| `UOW` | FND-04 | 11 | `UOW-06` | Transação local, visibilidade |
| `OBX` | FND-04 | 18 | `OBX-10` | Outbox: unicidade, escrita, drenagem |
| `INB` | FND-04 | 18 | `INB-08` | Inbox: deduplicação, consumo |
| `GAR` | FND-04 | 12 | `GAR-01` | Garantias de entrega (e o que não promete) |
| `BLK` | FND-04 | 5 | `BLK-01` | Bloco responsável por cada função |
| — | — | 64 (FND-04 total) | — | — |
| `ENV` | FND-05 | 25 | `ENV-08` | Envelope CloudEvents |
| `PTB` | FND-05 | 16 | `PTB-01` | Forma do pacote Protobuf |
| `BUF` | FND-05 | 12 | `BUF-01` | Workspace Buf e governança |
| `REP` | FND-05 | 6 | `REP-01` | Layout do repositório |
| `INT` | FND-05 | 5 | `INT-01` | Golden fixtures (interoperabilidade) |
| — | — | 64 (FND-05 total) | — | — |
| `TRP` | FND-06 | 54 | `TRP-01` | Transporte em geral |
| `RST` | FND-06 | 4 | `RST-01` | REST sobre HTTP |
| `GRP` | FND-06 | 19 | `GRP-01` | gRPC sobre HTTP/2 |
| `KFK` | FND-06 | 23 | `KFK-01` | Kafka (tópico, partição, ordem) |
| `SQS` | FND-06 | 19 | `SQS-01` | SNS e SQS (corpo, deduplicação) |
| `ASY` | FND-06 | 4 | `ASY-01` | AsyncAPI (catálogo) |
| `COE` | FND-06 | 8 | `COE-01` | Coexistência (um canal, um transporte) |
| — | — | 131 (FND-06 total) | — | — |
| `CTX` | FND-07 | 28 | `CTX-01` | Contexto de execução (9 campos) |
| `IDN` | FND-07 | 20 | `IDN-01` | Identidade (autenticação, autorização) |
| `ERR` | FND-07 | 28 | `ERR-01` | Taxonomia de erros |
| `MAP` | FND-07 | 7 | `MAP-01` | Mapeamento de categoria para protocolo |
| `THR` | FND-07 | 3 | `THR-01` | Limiar (o que **não** normatiza) |
| `DAT` | FND-07 | 26 | `DAT-01` | Classificação de dado |
| — | — | 112 (FND-07 total) | — | — |
| `RES` | FND-08 | 40 | `RES-01` | Resiliência (timeout, retry, circuito) |
| `MET` | FND-08 | 31 | `MET-01` | Métricas (catálogo por failure mode) |
| `TRC` | FND-08 | 16 | `TRC-01` | Tracing (ponta a ponta) |
| `LOG` | FND-08 | 14 | `LOG-01` | Log estruturado JSON |
| `RUN` | FND-08 | 20 | `RUN-01` | Runbook executável |
| — | — | 121 (FND-08 total) | — | — |
| `PIR` | FND-09 | 18 | `PIR-01` | Pirâmide de teste (5 camadas) |
| `CEN` | FND-09 | 44 | `CEN-01` | Cenários distribuídos |
| `FIX` | FND-09 | 13 | `FIX-01` | Golden fixture (formato) |
| `ORA` | FND-09 | 24 | `ORA-01` | Oráculos (round-trip) |
| `KIT` | FND-09 | 11 | `KIT-01` | Kits de teste (por camada) |
| `FIT` | FND-09 | 4 | `FIT-01` | Fitness functions (arquitetura) |
| `RAS` | FND-09 | 28 | `RAS-01` | Rastreabilidade (regra → vetor) |
| — | — | 142 (FND-09 total) | — | — |
| `GOV` | FND-10 | 33 | `GOV-01` | Governança e precedência |
| `BOM` | FND-10 | 10 | `BOM-01` | Bill of materials (6 itens) |
| `AUT` | FND-10 | 10 | `AUT-01` | Rito de autoridade |
| `ADO` | FND-10 | 9 | `ADO-01` | Adoção (por estágio) |
| `PIL` | FND-10 | 9 | `PIL-01` | Piloto (fluxo, não repositório) |
| `RDY` | FND-10 | 7 | `RDY-01` | Prontidão (7 itens de DoR) |
| — | — | 78 (FND-10 total) | — | — |

**Total:** 766 regras no acervo normativo.

---

## Blocos arquiteturais (RFC §4)

| Bloco | O que faz | Exemplos |
|--|--|--|
| `domain library` | Estado, invariantes, regras, UPRs, eventos | Modelo de negócio, decisões, agregados |
| `application service` | Orquestra caso de uso: autorização, transação, persistência | Sequência de passos, delegação a portas |
| `port` | Declara capacidade requerida | Interface de repositório, publisher |
| `provider` | Implementa porta com tecnologia concreta | Driver de BD, SDK, cliente HTTP |
| `contract package` | Define contratos de wire (schemas, tipos) | Protobuf, OpenAPI, AsyncAPI |
| `app` | Adapta protocolo, autentica, compõe dependências | Composition root, middleware, handler |

---

## Modos de verificação (RFC §2.2)

| Modo | Significado | Instrumento |
|--|--|--|
| `import-verifiable` | Análise estática do grafo de imports | Linter de dependências |
| `structurally reviewable` | Inspeção da estrutura (sem executar) | Revisão de código |
| `runtime-testable` | Só demonstrável em execução | Teste de integração |

---

## Capabilities externas (RFC §6)

| Capability | Significado | Exemplos |
|--|--|--|
| `io.storage` | Acesso a dados persistentes | BD, ORM, object storage |
| `io.messaging` | Acesso a broker/fila/tópico | SQS, Kafka, SNS |
| `io.network` | Comunicação externa síncrona | HTTP, gRPC |
| `io.filesystem` | Acesso a disco | File I/O |
| `io.clock` | Relógio de parede | `time.Now()`, `Date.now()` |
| `io.random` | Entropia não determinística | `rand.Int()` |
| `runtime.framework` | Framework de execução | NestJS, Gin, decorators |
| `wire.codec` | Serialização e schema | Protobuf, protoc |
| `observability` | Logger, tracer, métricas | OpenTelemetry, logs |
| `pure` | Computação determinística | Sem efeitos externos |

---

## Referências cruzadas

- **RFC base:** `rfc-dmpf-foundation-v0.1.md` — limites, blocos, regra de dependência
- **Mapa navegação:** `docs/dmpf/navegacao.md` — entrada por prefixo, pergunta e tema
- **Ledger reconciliação:** `docs/dmpf/reconciliacao.md` — estado das pendências
- **Diagramas:** `docs/dmpf/diagramas-metodologia.md` — fluxos e estados (visual)

---

## Convenção de legenda em documento

Toda ocorrência de sigla, âncora ou prefixo no corpo de um resumo deve vir legendada:

```
`UoW` (Unit of Work)
`ANC-02` (âncora de UoW e transação)
`INB-08` (regra de inbox — FND-04)
```

Forma canônica: `` `ID` (legenda curta) ``

A forma longa vive neste glossário.
