# UPR, Decision e mensagens — FND-03 | Resumo

> **Fonte:** [`docs/dmpf/upr-decision-mensagens.md`](../upr-decision-mensagens.md) | **Âncora:** `ANC-01` | **Status:** Promovido para revisão | **Linhas:** ~1.200 → ~220
>
> **Propósito:** Definir a Unidade de Processamento de Requisição (UPR), seu desfecho `Decision` e nomenclatura de mensagens.

---

## O que é UPR

Unidade de Processamento de Requisição (`UPR` — Unidade de Processamento de Regra): função pura que recebe uma entrada, aplica regra, devolve `Decision` (resultado exaustivo e exclusivo).

- **Invariante:** UPR não retém estado; recebe tudo pronto; devolve `Decision` sempre
- **Ciclo:** entrada → determinismo → desfecho imutável
- **Realização:** Go ou TypeScript com semantics idêntica

---

## Decision: o desfecho

`Decision` é uma **união exaustiva e exclusiva**: `Accepted[R]` OU `Rejected`.

### Ramo `Accepted[R]`

Tipo genérico que carrega o resultado `R`:

```
type Decision[R] =
  | Accepted[R] (payload, timestamp, requestID)
  | Rejected (reasons: NonEmptyList[Reason], timestamp, requestID)
```

- **Imutável:** após criação, nunca muda
- **Observável:** pode ser serializado, comparado, replicado entre stacks
- **Determinístico:** mesmo input → sempre mesmo output

### Ramo `Rejected`

Lista **não-vazia** de razões (`Reason`). Uma rejeição nunca é silenciosa — cada `Reason` explica por quê.

---

## Doze invariantes da UPR

| # | Invariante | Significado |
|--|--|--|
| I1 | UPR não retém estado | Dados externos vêm como argumentos já resolvidos |
| I2 | UPR é determinística | Mesmo input → idêntico output, sempre |
| I3 | Entrada é completa | UPR não pede dados a ninguém |
| I4 | Output é `Decision` ou erro | Nunca retorna `null`, `undefined` ou `Maybe` |
| I5 | Rejeição nunca é vazia | Cada `Rejected` carrega motivos |
| I6 | Razão tem código estável | Rastreável e auditável |
| I7 | Timestamp no desfecho | Quando foi decidido |
| I8 | Request ID no desfecho | Rastreamento |
| I9 | `Accepted` carrega resultado | Dados do desfecho |
| I10 | Desfecho é imutável | Após criar, não muda |
| I11 | Sem efeito colateral | UPR só computa |
| I12 | Sem dependência externa | Relógio, aleatoriedade, IO — tudo vem como valor |

---

## Mensagens: nomenclatura

**Comando** (verbo imperativo, presente): `ColocarItemNoCarrinho`, `AprovarPedido`  
**Evento** (verbo no passado): `ItemAdicionado`, `PedidoAprovado`

Razão: semântica clara. Comando é **intencional** (ação futura). Evento é **fato** (já ocorreu).

---

## Ciclo de vida da UPR

1. Recebe entrada (completa)
2. Valida contra invariantes
3. Executa regra (determinismo)
4. Cria `Decision[R]` com timestamp
5. Devolve ao application service
6. Application service persiste em UoW

---

## Relação com application service

O application service **orquestra** sobre a UPR:

```
1. application service carrega dados por portas
2. resolve identidade, contexto, autorização
3. invoca UPR com entrada completa
4. recebe Decision
5. conforme ramo:
   - Accepted → grava saída em UoW + outbox
   - Rejected → devolve erro categorizado
```

A UPR não toca em nada disso — ela só decide.

---

## Equivalência entre stacks

Go e TypeScript precisam:

1. **Mesma semântica:** `Decision[R]` = union de `Accepted` + `Rejected`
2. **Mesmo formato:** 2 oráculos verificam equivalência
3. **Mesmos vetores:** Cada regra tem par positivo/negativo

Ver FND-09 (testes) para suite de conformidade.

---

## O que este artefato **não** cobre

- Implementação concreta (delegada a épicos kernel)
- Integração com transportes (delegada a FND-06)
- Persistência da Decision (delegada a FND-04)
- Observabilidade (delegada a FND-08)

---

## Pendências

- `REC-001`: Vetores de equivalência do desfecho — **quitada por FND-09** (54 regras com par de cenários)

---

## Próximos passos

Consulte:
- **Implementação:** FND-11 redação de SDK
- **Integração:** FND-04 (UoW), FND-08 (observabilidade)
- **Testes:** FND-09 (oráculos para Decision)

