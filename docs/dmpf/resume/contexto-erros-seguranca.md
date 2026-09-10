# Contexto, Erros, Segurança — FND-07 | Resumo

> **Fonte:** [`docs/dmpf/contexto-erros-seguranca.md`](../contexto-erros-seguranca.md) | **Âncora:** `ANC-05` | **Status:** Promovido para revisão | **Linhas:** ~2.000 → ~240
>
> **Propósito:** Contexto de execução (9 campos), classificação de dado, segurança, threat model STRIDE.

---

## Contexto de execução (CTX): 9 campos obrigatórios

Metadados que toda requisição carrega:

1. `id` — Identificador único da requisição (rastreamento)
2. `correlationid` — Cadeia de correlação (ponta a ponta)
3. `causationid` — O evento que causou esta (sequência)
4. `traceparent` — Header OpenTelemetry (W3C trace)
5. `tenant` — Isolamento multi-tenant (`tenant:resources`)
6. `principal` — Identidade autenticada (quem faz)
7. `authorization` — Capacidades do principal (o quê pode fazer)
8. `timestamp` — Quando entrou (audit)
9. `classification` — Nível de sensibilidade do dado (`public`, `internal`, `confidential`, `restricted`)

Todos obrigatórios em toda chamada.

---

## Identidade (IDN): autenticação e autorização

| Aspecto | Regra |
|--|--|
| **Autenticação** | Verificação de quem é (credencial → principal) |
| **Autorização** | Decisão se principal pode fazer a ação (ACL, RBAC, atributo) |
| **Tenant** | Isolamento garantido (fail-closed: nenhum recurso sem tenant) |

**IDN-14:** Isolamento por tenant é **resultado fail-closed** — nenhum caminho deixa vazar recurso entre tenants.

---

## Erros (ERR): taxonomia

36 categorias de erro, cada uma com código HTTP único:

| Categoria | Causa | HTTP | Retentável? |
|--|--|--|--|
| `invalid_input` | Validação falhou | 400 | Não |
| `not_found` | Recurso não existe | 404 | Não |
| `conflict` | Estado conflita | 409 | Não (idempotência) |
| `internal_error` | Falha interna | 500 | Sim |
| `unavailable` | Serviço indisponível | 503 | Sim |
| `timeout` | Prazo excedido | 504 | Sim (se idempotente) |

**MAP:** Cada categoria mapeia para exatamente um código HTTP.

---

## Classificação de dado (DAT)

Taxonomia de sensibilidade:

| Nível | Significado | Regra |
|--|--|--|
| **public** | Publicável | Sem controle especial |
| **internal** | Dentro da org | Sem compartilhar externo |
| **confidential** | Restrito | PII, segredos comerciais |
| **restricted** | Ultra-sensível | Criptografia em repouso + acesso auditado |

**DAT-10:** Classificação é resultado verificável (não prescrição) — o que puder sair é público; o resto é restricted.

---

## Threat model STRIDE

Varredura de 6 categorias em 7 vetores:

| Categoria STRIDE | O quê |
|--|--|
| **S**poofing | Falsificar identidade |
| **T**ampering | Alterar dados em trânsito ou repouso |
| **R**epudiation | Negar autoria |
| **I**nformation Disclosure | Vazar dados |
| **D**enial of Service | Indisponibilidade |
| **E**levation of Privilege | Escalar permissão |

**Vetores analisados:** Adapters, desserialização, contratos, mensageria, secrets, PII, permissões.

**Resultado:** Parecer em `revisao-seguranca-fnd-07.md` — aprovado com ressalvas (insumos externos).

---

## O que este artefato **não** cobre

- Cifra em repouso → delegada a plataforma
- Teto de retenção → delegada a plataforma
- Gestão de secrets → delegada a plataforma
- Observabilidade de contexto → FND-08

---

## Pendências

- **REC-011:** Owner da revisão de Segurança — parcial (ADR-029 atribui ao author; re-revisão encaminhada)
- Policy de classificação e retenção — externa ao FND-07

---

**Próximos passos:** FND-08 (observabilidade), FND-10 (governança).
