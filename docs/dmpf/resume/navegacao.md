# Navegação — Mapa de entrada | Guia resumido

> **Fonte:** [`docs/dmpf/navegacao.md`](../navegacao.md) | **Status:** Guia derivado (não normativo) | **Linhas:** ~800 → ~160
>
> **Propósito:** Encontrar a regra sobre X sem abrir todos os 15 artefatos — entrada por prefixo, tema e pergunta.

---

## Atalho: por pergunta

| Sua pergunta | Comece em |
|--|--|
| **Limites e regra de dependência** | RFC — `regra de dependência` em §7 |
| **O que cada bloco faz** | RFC — os 6 blocos em §4.1 |
| **Como classifico minha unidade** | RFC — classificador total em §4.3 |
| **Pode haver Event Sourcing** | FND-03 — `ESC` (Event Sourcing e CQRS não pressupõe) |
| **Como uso transação local** | FND-04 — `UOW` (Unit of Work) |
| **Recebi a mesma mensagem duas vezes** | FND-04 — `INB` (Inbox) |
| **Que forma tem o envelope** | FND-05 — `ENV` (CloudEvents) |
| **Onde ponho o `.proto`** | FND-05 — `REP` (Layout repositório) |
| **Qual transporte usar** | FND-06 — prefixos `TRP`, `RST`, `GRP`, `KFK`, `SQS`, `ASY` |
| **Posso publicar no Kafka E no SQS** | FND-06 — `COE` (Coexistência: não, um por vez) |
| **Que campos o contexto carrega** | FND-07 — `CTX` (Contexto de execução) |
| **Que erro retorno** | FND-07 — `ERR` + `MAP` (Taxonomia e mapeamento) |
| **Este dado é sensível** | FND-07 — `DAT` (Classificação de dado) |
| **Timeout e retry** | FND-08 — `RES` (Resiliência) |
| **Que métrica emito** | FND-08 — `MET` (Métricas) |
| **O que vai no runbook** | FND-08 — `RUN` (Runbook) |
| **Que tipo de teste escrevo** | FND-09 — `PIR` (Pirâmide) + `RAS` (Rastreabilidade) |
| **O que faz o round-trip passar** | FND-09 — `ORA` (Oráculos) |
| **Como um artefato vira norma** | FND-10 — `GOV` (Governança) + `AUT` (Autoridade) |
| **Estou pronto** | FND-10 — `RDY` (Prontidão) |

---

## Por prefixo (resumo dos 49 prefixos)

### RFC e baseline
- **RFC:** Limites, blocos (6), regra de dependência, modo de verificação
- **Inventário:** Estado do parque em 10 repos — baseline candidato

### FND-03: UPR e decisão
- `DEC` (13): Desfecho da UPR — exaustividade, exclusividade, rejeição
- `CTR` (7): Níveis de contrato e fronteiras
- `UPR-I` (12): Invariantes (entrada/saída)
- `UPR-L` (5): Limites (o que não retém)
- `MSG-N` (4): Nomenclatura evento/comando
- `ESC` (9): Event Sourcing (não pressupõe)

### FND-04: UoW, inbox, outbox
- `UOW` (11): Transação local, visibilidade
- `OBX` (18): Outbox — unicidade, escrita, drenagem
- `INB` (18): Inbox — deduplicação, consumo
- `GAR` (12): Garantias (o que não promete: não é exactly-once)
- `BLK` (5): Bloco responsável por cada função

### FND-05: Protobuf, CloudEvents, Buf
- `ENV` (25): Envelope CloudEvents
- `PTB` (16): Forma Protobuf
- `BUF` (12): Workspace Buf e governança
- `REP` (6): Layout repositório
- `INT` (5): Golden fixtures

### FND-06: Transportes
- `TRP` (54): Transporte em geral
- `RST` (4): REST/HTTP
- `GRP` (19): gRPC/HTTP2
- `KFK` (23): Kafka
- `SQS` (19): SNS e SQS
- `ASY` (4): AsyncAPI
- `COE` (8): Coexistência (um canal, um transporte)

### FND-07: Contexto, erros, segurança
- `CTX` (28): Execução (9 campos)
- `IDN` (20): Identidade
- `ERR` (28): Taxonomia de erros
- `MAP` (7): Mapeamento → protocolo
- `THR` (3): Limiar (o que não normatiza)
- `DAT` (26): Classificação de dado

### FND-08: Resiliência, observabilidade
- `RES` (40): Resiliência — timeout, retry, circuito
- `MET` (31): Métricas — catálogo por failure mode
- `TRC` (16): Tracing ponta a ponta
- `LOG` (14): Log estruturado JSON
- `RUN` (20): Runbook

### FND-09: Testes
- `PIR` (18): Pirâmide (5 camadas)
- `CEN` (44): Cenários distribuídos
- `FIX` (13): Golden fixture (formato)
- `ORA` (24): Oráculos (round-trip)
- `KIT` (11): Kits por camada
- `FIT` (4): Fitness (arquitetura)
- `RAS` (28): Rastreabilidade

### FND-10: Governança, BOM, pilotos
- `GOV` (33): Governança
- `BOM` (10): Bill of materials
- `AUT` (10): Autoridade e rito
- `ADO` (9): Adoção
- `PIL` (9): Piloto (fluxo)
- `RDY` (7): Prontidão

**Total:** 766 regras em 49 prefixos.

---

## Por tema (5 eixos)

### Estrutura e arquitetura
- RFC: unidade, blocos, regra de dependência
- FND-03: o que a UPR é
- Prefixos: `DEC`, `CTR`, `UPR-I`, `UPR-L`, `BLK`

### Transacional e confiabilidade
- FND-04: transação, outbox, inbox
- FND-06: transportes (byte-preservação)
- FND-08: resiliência
- Prefixos: `UOW`, `OBX`, `INB`, `GAR`, `TRP`, `RES`

### Wire e integração
- FND-05: Protobuf, CloudEvents, Buf
- FND-06: políticas por transporte
- Prefixos: `ENV`, `PTB`, `BUF`, `REP`, `INT`, `RST`, `GRP`, `KFK`, `SQS`, `ASY`, `COE`

### Contexto e segurança
- FND-07: contexto, identidade, erros, dados
- Prefixos: `CTX`, `IDN`, `ERR`, `MAP`, `DAT`

### Operação e testes
- FND-08: observabilidade, runbook
- FND-09: testes, cenários, kits
- Prefixos: `MET`, `TRC`, `LOG`, `RUN`, `PIR`, `CEN`, `ORA`, `KIT`, `RAS`

### Governança
- FND-10: quando um artefato vira norma, como adota
- Prefixos: `GOV`, `AUT`, `BOM`, `ADO`, `PIL`, `RDY`

---

## Contexto de cada artefato

| Artefato | Prefixos | Volume | O que trata |
|--|--|--|--|
| `rfc-dmpf-foundation-v0.1.md` | — | — | Fundação: 6 blocos, regra de dependência, critérios P0 |
| `inventario-as-is.md` | — | — | Estado do parque: SQS-cêntrico, sem Kafka, sem Protobuf |
| `upr-decision-mensagens.md` | `DEC`, `CTR`, `UPR-I`, `UPR-L`, `MSG-N`, `ESC` | 54 | UPR, desfecho, modelo de mensagens |
| `uow-inbox-outbox.md` | `UOW`, `OBX`, `INB`, `GAR`, `BLK` | 64 | Transação, outbox, inbox, garantias |
| `cloudevents-protobuf-buf.md` | `ENV`, `PTB`, `BUF`, `REP`, `INT` | 64 | Envelope, Protobuf, Buf, golden fixtures |
| `politicas-transporte.md` | `TRP`, `RST`, `GRP`, `KFK`, `SQS`, `ASY`, `COE` | 131 | Transportes (7 prefixos, 131 regras) |
| `contexto-erros-seguranca.md` | `CTX`, `IDN`, `ERR`, `MAP`, `THR`, `DAT` | 112 | Contexto, identidade, erros, dados |
| `resiliencia-observabilidade.md` | `RES`, `MET`, `TRC`, `LOG`, `RUN` | 121 | Resiliência, métricas, logging, runbook |
| `testes-interop.md` | `PIR`, `CEN`, `FIX`, `ORA`, `KIT`, `FIT`, `RAS` | 142 | Pirâmide de testes, cenários, oráculos, kits |
| `governanca-bom-pilotos.md` | `GOV`, `BOM`, `AUT`, `ADO`, `PIL`, `RDY` | 78 | Governança, BOM, autoridade, pilotos |

---

## Caso de emergência

Não encontrou? Consulte o **ledger de reconciliação** [`reconciliacao.md`](./reconciliacao.md) para pendências que mudaram de artefato.

Depois, a **normativa integral** em [`docs/dmpf/`](../), sempre a fonte de verdade.
