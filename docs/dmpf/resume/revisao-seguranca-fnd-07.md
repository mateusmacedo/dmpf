# Revisão de Segurança — FND-07 | Guia resumido

> **Fonte:** [`docs/dmpf/revisao-seguranca-fnd-07.md`](../revisao-seguranca-fnd-07.md) | **Status:** Parecer de revisão (não normativo) | **Linhas:** ~500 → ~95
>
> **Propósito:** Evidência de que o gate `THR-03` (threat model de FND-07) foi executado; desfecho: **aprovado com ressalvas**.

---

## O que foi revisado

Objeto: [`contexto-erros-seguranca.md`](../contexto-erros-seguranca.md) §7 (threat model STRIDE) e §8 (governança do dado).

Gate: `THR-03` — «não se promove enquanto revisão de Segurança não validar o threat model».

Revisor: Mateus Macedo Dos Anjos (atribuído por [ADR-029](../../adr/029-titular-da-revisao-de-seguranca-fnd-07.md)).

**Limitação importante:** O revisor é o próprio author do artefato. Independência ausente. Essa fraqueza está declarada, não contornada. A re-revisão por titular independente é condição para encerramento formal (ver `REC-011` no [ledger](./reconciliacao.md)).

---

## Método: duas camadas

### 1. Camada mecânica (objetiva, reproduzível)

Critério verificável por varredura automática:

| O que | Procedimento |
|--|--|
| Cobertura STRIDE | 6 categorias × 7 vetores — nenhuma ausente sem justificativa |
| Owner por linha | Todas as 40+ linhas têm atribuição explícita |
| Regras citadas existem | Cada `ID-NN` referenciado existe em `docs/dmpf/*.md` |
| Coerência de âncora | Se um owner cita `FND-XX sob ANC-YY`, conferir RFC §12.3 |
| Integridade normativa | Contagem + continuidade de IDs por prefixo |

**Resultado:** ✓ Todos os critérios satisfeitos. Nenhuma linha sem owner. Nenhum identificador fictício.

### 2. Camada de julgamento (subjetiva, requer expertise)

O que **não pode** ser conferido por varredura: o motivo de uma exclusão sustenta a ausência? O owner atribuído é o certo? A regra faz o que se propõe?

**Resultado:** Aprovado. Seis categorias STRIDE nos sete vetores conferidas. Exclusões justificadas. Linhas específicas (`CTX-25`, `IDN-14`, `DAT-10`, `THR-01`) analisadas conforme ARQ-488.

---

## Desfecho

**Status:** `Aprovado com ressalvas`

**Ressalva 1 (insumo externo):** Classificação de dado e teto de retenção precisam de **policy de plataforma** (fora do escopo de FND-07). Mitigação: ambas estão documentadas como **pós-condições verificáveis** e encaminhas a governance + operação.

**Ressalva 2 (insumo externo):** Re-revisão por titular independente de Segurança, condicionada à constituição da área. Prazo: aberto.

---

## Tabela resumida de cobertura

| Vetor | Amenáças | STRIDE | Owner |
|--|--|--|--|
| **Adapters** (REST, gRPC) | 6 | M M M M M M | Nomeado |
| **Desserialização** (Protobuf, JSON) | 5 | M M M M M X | Nomeado |
| **Contratos** (schemas, wire) | 4 | M M M M X M | Nomeado |
| **Mensageria** (Kafka, SQS, SNS) | 6 | M M M M M M | Nomeado |
| **Secrets** (credenciais, chaves) | 4 | M M M M X X | Nomeado |
| **PII** (dados pessoais) | 4 | M M M X M M | Nomeado |
| **Permissões** (autorização, tenant) | 4 | M M M M X X | Nomeado |

`M` = Ameaça material própria | `X` = Excluída com justificativa | Nenhuma linha sem owner.

---

## Pontos de atenção mencionados em ARQ-488

✓ `CTX-25`: Proveniência vs. autorização no consumo assíncrono — aprovado.
✓ `IDN-14`: Isolamento por tenant (fail-closed) — aprovado.
✓ `DAT-10`: Classificação de dado é resultado fail-closed — aprovado.
✓ `THR-01`: Threat model é seleção, não cobertura universal — aprovado.

---

## Próximas etapas

1. ✓ Gate `THR-03` considerado executado (FND-07 pronto para merge em `develop`)
2. ◐ Re-revisão por titular independente — encaminhada a governance (ver `REC-011`)
3. ◐ Policy de classificação e retenção — fora de FND-07, delegada a plataforma

**Consulte a revisão integral** em [`docs/dmpf/revisao-seguranca-fnd-07.md`](../revisao-seguranca-fnd-07.md) para argumentação completa, critérios mecânicos e análise de cada vetor.
