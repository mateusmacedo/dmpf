# Testes e Interoperabilidade — FND-09 | Resumo

> **Fonte:** [`docs/dmpf/testes-interop.md`](../testes-interop.md) | **Âncora:** `ANC-07` | **Status:** Promovido para revisão | **Linhas:** ~3.750 → ~280
>
> **Propósito:** Pirâmide de testes, golden fixtures, oráculos, fitness functions e rastreabilidade regra→vetor.

---

## Pirâmide de testes (PIR): 5 camadas

```
                ▲
               ╱ ╲  E2E / Distributed (build tag: integration)
              ╱   ╲
             ╱     ╲ App / Adapter
            ╱       ╲
           ╱ Service ╲
          ╱           ╲
         ╱   Provider  ╲
        ╱               ╲
       ╱ Unit / Domain  ╲
      ╱_________________╲
```

- **Unit (base):** Domínio isolado em memória — rápido, sem I/O
- **Provider:** Adaptadores com fakes/banco em memória
- **Service:** Aplicação sobre providers reais (Postgres, Kafka)
- **App:** Adapter de protocolo (REST, gRPC)
- **E2E:** Dois processos reais, mensageria real

---

## Golden fixtures (FIX)

Artefato versionado de teste:

```
contracts/fixtures/
  <bounded_context>/projection/v1/
    <rule_id>.json
```

**Conteúdo:** Triada (input, regra, output esperado).

**Oráculos:** 3 verificações independentes:
1. Desfecho correto (par `Accepted` / `Rejected`)
2. Projeção observável idêntica
3. Payload hash determinístico

---

## Oráculos (ORA): round-trip verificável

**Oráculo 1 (desfecho):** `Decision` é exatamente como esperado  
**Oráculo 2 (projeção):** Efeitos colaterais são observáveis (state)  
**Oráculo 3 (determinismo):** Reexecução com mesma entrada → idêntico

**Crítico:** **Não é byte-identity** — é observabilidade. Go e TypeScript podem divergir em tipo largo, desde que o oráculo capture.

---

## Kits (KIT): por camada

Um kit por nível da pirâmide:

| Kit | Camada | O que testa |
|--|--|--|
| **domainkit** | Unit | Regra isolada (projeção de domínio) |
| **providerkit** | Provider | Repositório + inbox + outbox |
| **appkit** | Service | Caso de uso com providers reais |
| **distkit** | E2E | Dois processos reais sobre Kafka |

Cada kit exporta interface com veredito por valor.

---

## Cenários (CEN): 44 distribuídos

Exemplo: `CEN-15` — publicar 3 mensagens, consumir em paralelo, verificar ordem.

Estrutura:
- Setup (estado inicial)
- Ação (passo)
- Verificação (oráculo)

---

## Fitness function (FIT): regra de arquitetura

Teste que falha se a regra de dependência é violada:

```
if (domain_imports_provider) fail("P0-1 violado")
if (application_imports_driver) fail("RES-14 violado")
```

---

## Rastreabilidade (RAS): regra → diagnóstico → vetor

Matriz 1:1:

```
| Regra | Diagnóstico | Vetor (pass) | Vetor (fail) |
|-------|-------------|------|------|
| UPR-I1 | "UPR received incomplete input" | UPR devolve Rejected | UPR não devolve Decision |
| ...    | ... | ... | ... |
```

**Total:** 650 de 672 regras têm vetor.

---

## Modo de verificação por regra

| Modo | Significado |
|--|--|
| `execution` | Cenário distribuído (duas caixas reais) |
| `inspection` | Code review (estrutura, nomes, convenção) |
| `conformance` | Padrão mecanicamente decidível |

**Calibração importante:** Exigir execution onde inspection basta produz overhead; exigir inspection onde execution é necessário deixa brecha.

---

## Mapa de consolidação

REC-010 (quitada): Todas as 9 métricas do artefato reconciliadas:

- 650 de 664 regras têm mecanismo de prova
- 10 exceções nomeadas (ausência declarada)
- 4 ADRs (`ADR-DMPF-A`..`Q`) correlacionados

---

## O que este artefato **não** cobre

- Implementação dos kits (delegada a épicos kernel)
- Localização do validador de envelope (delegada a FND-06 + co-decisão)
- Performance benchmarks (não é objetivo)

---

**Próximo passo:** FND-10 (governança, BOM, pilotos).
