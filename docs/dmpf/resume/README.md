# DMPF — Guia derivado em leitura humana

> **Status:** Guia visual derivado — não normativo. Aponta para artefatos donos em `../` (regra de precedência: em conflito, vale a norma).
>
> **Propósito:** Resumir o acervo DMPF (~21 mil linhas em 15 artefatos) em leitura densa, formal e acessível (PT-BR), com legendas obrigatórias em toda ocorrência.
>
> **Data:** 2026-09 | **Fonte:** `docs/dmpf/` normativa | **Revisão:** pt-review pendente

---

## Como ler este guia

1. **Comece pelo [glossário](./glossario.md)** para entender as siglas e referências canônicas.
2. **Escolha um artefato** pela pergunta que faz (tabela abaixo).
3. **Leia o resumo** aqui (15–25% do original) para orientação rápida.
4. **Consulte a fonte** (link no topo de cada resumo) para norma integral.

> Cada arquivo de resumo abre com metadados: status, link ao dono, data/versão da fonte, aviso de que **não obriga**.

## Entrada prática

Para estruturar e implementar um novo bounded context, consulte o
[guia prático de implementação DMPF](../../guides/dmpf-implementation.md). Por
ser derivado e não normativo, o guia fica separado dos resumos dos artefatos abaixo.

---

## Índice de resumos por artefato

| Se sua pergunta é… | Comece por | Linhas fonte | Linhas resumo | Tempo |
|--|--|--|--|--|
| **Limites arquiteturais e regra de dependência** | [`rfc-dmpf-foundation-v0.1.md`](./rfc-dmpf-foundation-v0.1.md) | ~2.100 | ~400–500 | 10 min |
| **Qual é o estado atual do parque** | [`inventario-as-is.md`](./inventario-as-is.md) | ~1.000 | ~150–200 | 5 min |
| **O que é UPR e como ela decide** | [`upr-decision-mensagens.md`](./upr-decision-mensagens.md) | ~1.200 | ~180–250 | 8 min |
| **Como funciona transação, inbox, outbox** | [`uow-inbox-outbox.md`](./uow-inbox-outbox.md) | ~1.800 | ~270–360 | 12 min |
| **Que forma tem a mensagem, Protobuf e contratos** | [`cloudevents-protobuf-buf.md`](./cloudevents-protobuf-buf.md) | ~1.800 | ~270–360 | 12 min |
| **Políticas por transporte (REST, gRPC, Kafka, SQS)** | [`politicas-transporte.md`](./politicas-transporte.md) | ~2.600 | ~390–520 | 15 min |
| **Contexto, erros, segurança e identidade** | [`contexto-erros-seguranca.md`](./contexto-erros-seguranca.md) | ~2.000 | ~300–400 | 12 min |
| **Resiliência, timeout, retry, observabilidade** | [`resiliencia-observabilidade.md`](./resiliencia-observabilidade.md) | ~2.200 | ~330–440 | 13 min |
| **Como testar com pirâmide e cenários** | [`testes-interop.md`](./testes-interop.md) | ~3.750 | ~560–750 | 18 min |
| **Governança, BOM, pilotos e prontidão** | [`governanca-bom-pilotos.md`](./governanca-bom-pilotos.md) | ~2.100 | ~315–420 | 12 min |
| **Estado das pendências cruzadas** | [`reconciliacao.md`](./reconciliacao.md) | ~1.200 | ~180–240 | 7 min |
| **Mapa por prefixo, tema e pergunta** | [`navegacao.md`](./navegacao.md) | ~800 | ~120–160 | 5 min |
| **Fluxos, processos e diagrama visual** | [`diagramas-metodologia.md`](./diagramas-metodologia.md) | ~1.600 | ~240–320 | 10 min |
| **Parecer de revisão de segurança (FND-07)** | [`revisao-seguranca-fnd-07.md`](./revisao-seguranca-fnd-07.md) | ~500 | ~75–100 | 3 min |

**Soma da fonte:** ~21.330 linhas  
**Soma dos resumos:** ~3.500–4.700 linhas (16–22% do volume)

---

## Legenda — Entrada rápida por categoria

### Arquitetura e estrutura
- **Como a regra de dependência funciona?** → RFC §7
- **Que são os seis blocos?** → RFC §4
- **O que é uma unidade de verificação?** → RFC §3

### Transacional e mensagens
- **Como a transação local se relaciona com outbox?** → FND-04 (`UoW (Unit of Work)`)
- **Quando dupliquei a mensagem?** → FND-04 (`INB (Inbox)`)
- **Como a UPR decide?** → FND-03 (`UPR-I` / `UPR-L`)

### Wire e contratos
- **Que forma tem o envelope?** → FND-05 (`ENV (Envelope CloudEvents)`)
- **Onde vão os arquivos `.proto`?** → FND-05 (`REP (Repositório)`)
- **Como o golden fixture prova equivalência?** → FND-09 (`INT (Interoperabilidade)`)

### Transporte
- **Qual transporte devo usar?** → FND-06 — leia a política do seu: `TRP`, `RST`, `GRP`, `KFK`, `SQS`
- **Posso misturar Kafka e SQS no mesmo canal?** → FND-06 (`COE (Coexistência)`) — resposta: não
- **Que tenho de fazer pra byte-preservar?** → FND-06 (`TRP-01` e matriz de §5)

### Segurança e contexto
- **Que campos o contexto carrega?** → FND-07 (`CTX (Contexto de execução)`)
- **Que categoria de erro devo retornar?** → FND-07 (`ERR` + `MAP (Mapeamento)`)
- **Este campo é sensível — o que isso obriga?** → FND-07 (`DAT (Classificação de dado)`)

### Resiliência e operação
- **Como configuro timeout e retry?** → FND-08 (`RES (Resiliência)`)
- **Que métrica preciso emitir?** → FND-08 (`MET (Métricas)`)
- **O que vai no runbook?** → FND-08 (`RUN (Runbook)`)

### Testes
- **Que tipo de teste devo escrever?** → FND-09 (`PIR (Pirâmide)` + `RAS (Rastreabilidade)`)
- **O que faz o round-trip passar?** → FND-09 (`ORA (Oráculos)`)
- **Que kit preciso?** → FND-09 (`KIT (Kits de teste)`)

### Governança
- **Como um artefato vira norma?** → FND-10 (`GOV (Governança)` + `AUT (Autoridade)`)
- **Estou pronto para começar?** → FND-10 (`RDY (Prontidão)`)

---

## Regras duras do guia

1. **Não alterar texto normativo** dos 15 artefatos. Resumos apontam, nunca replicam.
2. **Legendas em toda ocorrência** — `ID` (forma curta) — não só primeira menção.
3. **Formato:** PT-BR formal, frases curtas, seções escaneáveis.
4. **Meta de volume:** 15–25% do original por resumo (ex.: RFC ~2.100 → ~400–500).
5. **Não inventar regra** — toda obrigação aponta ao ID estável na fonte.

---

## Navegação da fonte (entrada alternativa)

Se preferir entrar pela **estrutura dos artefatos donos**, use:
- [`docs/dmpf/README.md`](../README.md) — índice versionado com status de promoção
- [`docs/dmpf/navegacao.md`](../navegacao.md) — mapa por prefixo de regra
- [`docs/dmpf/reconciliacao.md`](../reconciliacao.md) — ledger de pendências

---

## Informações gerais

| Item | Valor |
|--|--|
| **Épico** | [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436) — Golden Path para Sistemas Orientados a Domínio e Mensagens |
| **Especificação** | [SPEC-8YVF0RR5](../specs/SPEC-8YVF0RR5-dmpf-rfc-limites-deps.md) (limites, dependência) |
| **Repositório** | [`lidercap-platform` / `docs/dmpf/`](https://gitea.lidercap.com.br/lidercap-apps/lidercap-platform) |
| **Próximo passo** | Leia [`docs/onboarding.md`](../../onboarding.md) para setup e primeiro PR |

