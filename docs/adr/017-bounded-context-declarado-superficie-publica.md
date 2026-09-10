# ADR-017: Declarar o bounded context obrigatório e limitar a interação entre contexts à superfície pública

## Status

Aceito — 2026-08-28. Implementa SPEC-DBTRMM3X.

Estendido pelo [ADR-042](./042-shared-kernel.md) — 2026-09-09: a condição de
contexto C2 passa a aceitar também o destino designado como shared kernel no
baseline governado. O interior privado por default e a invalidade de
`public_integration_surface: true` em bloco `domain` seguem intactos.

## Contexto

O DMPF classifica todo código de produção em seis blocos e governa as arestas
entre unidades com uma regra de dependência. Essa regra, porém, não cabe numa
matriz pura de blocos: a mesma aresta `domain → domain` é legítima dentro de um
limite de negócio e ilícita quando cruza esse limite (RFC §7.1). Uma matriz
`6×6` responde a uma única pergunta — "esse tipo de código pode depender daquele
tipo de código?" — e a colapsa com uma segunda, "essas duas partes do sistema
podem se falar?". Sem uma noção declarada de limite, o verificador não observa a
segunda: um domínio importando o domínio de outro contexto passaria verde.

Três forças moldam a resposta. A primeira é a verificabilidade mecânica: o linter
só enxerga o que está declarado e não infere contexto a partir do layout. As dez
bases inventariadas sequer nomeiam bounded contexts à moda DDD, e suas
aproximações são heterogêneas — separação por `entity_type` no `FilterPolicy` de
SNS→SQS, subpastas `domain/{...}`, feature folders sem contrato que as sustente —,
nenhuma servindo de fonte canônica (RFC §5.4, evidência). A segunda é o
acoplamento entre contextos: é preciso permitir integração, mas mantendo o
interior privado por default e expondo apenas uma superfície pública (RFC §5.5).
A terceira é a superfície de burla: o campo que habilita import entre contextos é
alvo natural de reclassificação oportunista — se uma domain library pudesse se
declarar pública, a proibição de §5.5 seria trivialmente contornável (RFC §7.2).

## Decisão

Adotar `bounded_context` como identidade declarada e obrigatória. Toda
`verification_unit` declara um `bounded_context` no seu `metadata_container` —
string estável e única no universo — sujeita a quatro propriedades normativas
(RFC §5.4): **declarada** (nunca inferida de diretório, nome de pacote ou
topologia de fila), **obrigatória** (exatamente uma por unidade de produção;
ausência reprova, RFC §3.6), **estável** (renomear é mudança normativa, sujeita a
RFC §10) e **ortogonal ao bloco** (um mesmo context reúne unidades de vários
blocos, e um mesmo bloco aparece em vários contexts).

Uma unidade não pode importar a domain library de outro `bounded_context`; a
interação entre contexts ocorre apenas por contrato de integração ou API pública
(RFC §5.5). Disso decorre que a regra de dependência não é função apenas do par
de blocos: a mesma aresta `domain → domain` é permitida dentro de um context e
proibida entre contexts. A forma dessa regra — a função `decide(...)`, a condição
de bloco C1 (matriz de RFC §7.3) e os diagnósticos `DMPF-D001` e `DMPF-D002` — é
fixada pelo ADR-010; este ADR decide apenas a condição de contexto **C2** (RFC
§7.2): a aresta só passa quando `same_bounded_context` **ou**
`public_integration_surface` do destino é verdadeiro. `same_bounded_context` é a
comparação exata de strings dos dois `bounded_context`; `public_integration_surface`
é verdadeiro quando a unidade de destino é um contract package ou declara
`public_integration_surface: true` no seu `metadata_container` (RFC §7.2). Uma
domain library nunca é superfície pública de integração: declarar
`public_integration_surface: true` em unidade cujo `block` é `domain` é inválido e
emite `DMPF-M002` (RFC §7.2).

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Expressar a dependência como matriz pura `6×6` de blocos, colapsando cada par `(source_block, target_block)` numa única célula. | O bloco `rationale` de RFC §7.2 registra que separar C1 de C2 é o que torna a regra entre contexts verificável sem multiplicar a matriz por contexto: a matriz responde "esse tipo de código pode depender daquele tipo de código?" e o predicado responde "essas duas partes do sistema podem se falar?" — perguntas distintas, e "tratá-las como uma só era o defeito da matriz pura". Uma célula não diz que `domain → domain` é permitida dentro do context e proibida entre contexts (RFC §7.1). |
| Inferir o `bounded_context` da topologia — diretório, nome de pacote ou topologia de fila — em vez de exigir declaração explícita. | A regra "Declarada" de RFC §5.4 exige valor declarado, "nunca inferido", e o bloco `evidência` da mesma seção mostra o porquê: as aproximações dos dez repositórios são heterogêneas (`entity_type` em SNS→SQS, subpastas `domain/{...}`, feature folders sem contrato) e nenhuma serve de fonte canônica. Inferir herdaria essa heterogeneidade e não produziria identidade estável e única. |
| Permitir que uma domain library seja marcada como superfície pública (`public_integration_surface: true` em unidade de bloco `domain`) para habilitar import entre contexts. | O bloco `normativo` de RFC §7.2 fecha essa porta: sem a cláusula, o campo seria "o caminho trivial para burlar §5.5" — bastaria marcar o domínio como público para que outro context o importasse. Declará-lo em bloco `domain` é inválido e emite `DMPF-M002`. |

## Consequências

**Positivas:**

- A regra de dependência distingue "esse tipo de código pode depender daquele
  tipo?" de "essas duas partes do sistema podem se falar?", capturando violações
  entre contexts que a matriz pura de blocos deixaria passar (RFC §7.1, §7.2).
- A identidade de limite fica verificável mecanicamente por comparação exata de
  strings (`same_bounded_context`), sem depender de heurística de layout que o
  linter não consegue observar (RFC §5.4, §7.2).
- O interior de cada context é privado por default: importar de fora uma unidade
  não declarada como pública é violação, e o acoplamento entre contexts só passa
  por superfície explícita, tornando o contrato de integração visível (RFC §5.5).
- Fecha a burla trivial de §5.5 — como uma domain library não pode se declarar
  pública (`DMPF-M002`), a proibição de import entre contexts não é contornável
  por um flag (RFC §7.2).
- Renomear um `bounded_context` é mudança normativa rastreável e sujeita a RFC
  §10, preservando a estabilidade da identidade ao longo do tempo (RFC §5.4,
  regra "Estável").

**Negativas:**

- **Custo aceito:** declarar `bounded_context` em toda unidade de produção é
  adoção nova, sem lastro em nenhum dos dez repositórios inventariados — nenhum
  nomeia bounded contexts à moda DDD —; é trabalho de migração empurrado para os
  épicos de kernel (RFC §5.4 evidência, §1.4), aceito em troca da verificabilidade
  do limite.
- Um novo campo obrigatório amplia a superfície do metadado e do trust model —
  ausência reprova (RFC §3.6) e renomear exige autorização (RFC §10) —,
  aumentando o atrito de qualquer mudança legítima de contexto.
- O grafo de decisão fica mais complexo: ao acrescentar a dimensão de contexto
  (C2), a regra deixa de ser uma matriz pura de blocos e passa a distinguir falha
  de bloco de falha de contexto — a função de cinco argumentos e os dois
  diagnósticos que a materializam, fixados pelo ADR-010, no lugar de uma única
  célula.

## Referências

- ADR-010 — regra de dependência e os seis blocos. Fixa a forma da regra de
  dependência: a função `decide(...)`, a condição de bloco C1 (matriz de RFC
  §7.3) e os diagnósticos `DMPF-D001`/`DMPF-D002`. Este ADR não reescreve essa
  função — delega-a ao ADR-010 e decide apenas a condição de contexto C2.
- ADR-012 — classificação por metadado declarado. Institui a classificação por
  metadado autodeclarado no `metadata_container` e a recusa de inferir do nome ou
  do layout; o `bounded_context` que este ADR torna obrigatório é um desses
  metadados declarados e herda a mesma recusa de inferência.
- ADR-013 — autoridade sobre a classificação. Exige autorização distinta da
  autoria para mudar `block` e `bounded_context`; a regra "Estável" — renomear um
  `bounded_context` é mudança normativa — pressupõe essa autoridade.
- SPEC-DBTRMM3X — especificação da série de ADRs do DMPF, que este ADR implementa.
- RFC DMPF Foundation v0.1 — `docs/dmpf/rfc-dmpf-foundation-v0.1.md`:
  - **Origem:** §5.4 (identidade canônica de bounded context — as quatro
    propriedades), §5.5 (dependências entre contexts — interior privado e
    superfície pública) e §7.2 (predicados C2, `same_bounded_context`,
    `public_integration_surface` e o diagnóstico `DMPF-M002`).
  - **Apoio:** §3.6 (casos degenerados — ausência de `bounded_context` reprova),
    §7.1 (a decisão é uma função, não uma célula — dona no ADR-010) e §10
    (mudança normativa exige autorização; renomear um `bounded_context` recai
    aqui).
- ARQ-448 — https://lider-cap.atlassian.net/browse/ARQ-448 (FND-11, redação,
  promoção e aceite da série de ADRs do DMPF).
