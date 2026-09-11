# ADR-014: Proibir a aresta `domain → port` e reservar `port` à fronteira

## Status

Aceito — 2026-08-28. Implementa SPEC-DBTRMM3X.

## Contexto

O DMPF classifica cada `verification_unit` em um dos seis blocos de §4.1. Dois
deles entram em tensão neste ponto: a `domain library`, que expressa regra de
negócio determinística e não executa nem descreve I/O, e a `port`, que exprime a
capacidade de fronteira que a camada consumidora requer para o que ela não faz
sozinha. A pergunta é se o domínio pode depender de uma porta.

A norma de ownership de §5.1 já traça a fronteira: policies e domain services
puramente computacionais vivem no domínio, enquanto repositórios, transações,
outbox, cache e clientes externos vivem na camada de aplicação ou em um módulo de
portas compartilhadas. O problema nasce quando alguém lê essa fronteira como uma
permissão condicional — de que o domínio poderia importar um bloco `port` "desde
que a porta fosse computacional". Essa leitura introduz uma distinção que o
verificador não consegue observar: diante de `domain → port`, um linter vê dois
blocos e nada mais, sem como decidir se a porta do outro lado é computacional ou
de infraestrutura.

Três forças pressionam a decisão. A primeira é a verificabilidade: a regra
precisa ser decidível por um linter, e permitir a aresta sob condicional deixaria
passar a porta de infraestrutura que a spec proíbe nominalmente, enquanto
proibi-la sob a mesma condicional mataria a exceção — não há terceira saída com os
campos disponíveis. A segunda é conter a superfície do contrato do verificador
(§10): resolver a ambiguidade com um campo novo só para relaxar essa aresta custa
gramática, cardinalidade, cobertura de baseline e vetores próprios. A terceira é o
risco de burla: qualquer campo que afrouxe uma aresta P0 vira alvo de
reclassificação oportunista.

Resta o caso residual — o domínio precisar de porta para relógio, gerador de
identificador ou fonte de aleatoriedade. Ele colide com o princípio 8 de §2.1
(determinismo no domínio), que manda tempo, identificadores e dados externos
entrarem como valores já resolvidos ou por políticas puras. A tensão não é
teórica: `golibs/packages/goservice` declara a porta `EventPublisher` em uma
unidade `domain`, e publicar evento é I/O — uma violação observada, não uma
hipótese.

## Decisão

Reservar o bloco `port` exclusivamente a capacidades de fronteira (I/O) e
classificar toda interface puramente computacional como `domain`, em linha com a
definição de `port` de §4.1, segundo a qual a porta expressa "a capacidade
requerida pela camada consumidora" para o que ela não faz sozinha — e o que o
domínio faz sozinho é computar.

Com isso, uma `verification_unit` classificada como `domain library` não pode
importar uma unidade classificada como `port`: a aresta `domain → port` é
PROIBIDA, sem condicional e sem exceção (RFC §5.2).

Consequência operacional normativa: uma interface hoje colocada em uma unidade
`port` que seja puramente computacional está mal classificada. A correção é
reclassificá-la como `domain`, não abrir exceção na matriz (RFC §5.2). Onde a
unidade mistura computação com I/O — como a que declara `EventPublisher` — ela
está mal dimensionada e deve ser dividida pela regra 3 de §4.2.

O caso residual fica fechado pelo princípio 8 (§2.1) e pela tabela da §9.3, que o
ADR-016 estabelece: o domínio recebe a hora, o identificador, a semente de
aleatoriedade e o dado de outro agregado já resolvidos pela camada de aplicação.
Cada linha dessa tabela é uma porta que não precisa existir, de modo que não sobra
necessidade legítima de o domínio importar `port`.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Permitir a aresta `domain → port` sob condicional, autorizando o domínio a importar uma porta desde que ela seja "computacional" e não de infraestrutura. | Introduz uma distinção que o verificador não observa: diante de `domain → port`, um linter vê dois blocos e nada mais, sem como decidir se a porta do outro lado é computacional ou de infraestrutura. Permitir a aresta deixaria passar a porta de infraestrutura que a spec proíbe nominalmente; proibi-la sob condicional mataria a exceção. Sem terceira saída com os campos disponíveis, a regra ficaria inverificável (RFC §5.2). |
| Criar uma subclasse normativa fechada de porta — por exemplo `port_kind: computational \| infrastructure` — que tornasse a aresta decidível por um campo adicional. | Descartada por três razões (RFC §5.3): superfície — o campo novo exigiria gramática, cardinalidade, herança, cobertura no baseline, comportamento em ausência e valor desconhecido, duplicando o aparato de §10 para uma distinção que §5.2 elimina; redundância — uma porta "computacional" já é `domain` pela definição de §4.1, então a subclasse nomearia como variante de `port` algo que já tem bloco; e superfície de burla — um campo que relaxa uma aresta P0 é o alvo natural de reclassificação oportunista, exatamente o risco que §10 precisa conter. |

## Consequências

**Positivas:**

- A regra `domain → port = PROIBIDA` fica decidível por um linter sem campo
  adicional: basta observar os dois blocos envolvidos, o que torna a norma
  verificável (RFC §5.2).
- O contrato do verificador (§10) não ganha superfície nova — nenhuma gramática,
  cardinalidade, herança, baseline ou vetores para um `port_kind` (RFC §5.3).
- Fecha a superfície de burla: não existe campo que relaxe a aresta P0 e sirva de
  alvo para reclassificação oportunista (RFC §5.3).
- Alinha-se ao princípio 8 (§2.1): relógio, gerador de identificador e
  aleatoriedade entram no domínio como valores já resolvidos — pela tabela da
  §9.3, que o ADR-016 estabelece —, de modo que não sobra caso legítimo em que o
  domínio precise importar `port`.

**Negativas:**

- **Custo aceito:** interfaces hoje colocadas em unidades `port` que sejam puramente
  computacionais ficam retroativamente mal classificadas e precisam ser
  reclassificadas como `domain`; unidades que misturam computação com I/O — como
  `golibs/packages/goservice`, que declara `EventPublisher` em uma unidade
  `domain` — precisam ser divididas pela regra 3 de §4.2. Aceita-se esse trabalho
  de correção e migração em vez de abrir exceção na matriz (RFC §5.2).
- A proibição é absoluta, sem condicional: perde-se a expressividade de nomear uma
  "porta computacional" como variante de `port`, mesmo quando um autor a
  considerasse conceitualmente uma porta — ela deixa de ter nome próprio no
  vocabulário de blocos e passa a ser `domain` (RFC §5.3).
- Exige disciplina de classificação: a distinção entre computação e fronteira
  passa a depender do julgamento correto de bloco na declaração do metadado, e não
  de um campo que a tornasse explícita e autodocumentada.

## Referências

- ADR-010 — regra de dependência e os seis blocos. Estabelece os seis blocos e a
  precedência de desempate (RFC §4.2); esta decisão apoia-se nessa regra ao exigir
  que a unidade que mistura computação e I/O — como a que declara `EventPublisher`
  — seja dividida pela regra 3 de §4.2.
- ADR-012 — classificação por metadado declarado. Ancora a classificação no
  metadado declarado e recusa inferi-la do nome ou do layout (RFC §4.4); a
  verificabilidade desta decisão pressupõe que o bloco de cada unidade é declarado
  e observável pelo linter, não derivado do diretório em que a unidade vive.
- ADR-016 — domínio executável em memória. Estabelece a tabela da RFC §9.3, pela
  qual o domínio recebe hora, identificador, aleatoriedade e dados de outro
  agregado já resolvidos; esta decisão usa essa tabela para fechar o caso residual,
  demonstrando que não sobra necessidade legítima de o domínio importar `port`.
- RFC de fundação do DMPF — `docs/dmpf/rfc-dmpf-foundation-v0.1.md`: §2.1
  (princípio 8, determinismo no domínio), §4.1 (definição da `domain library` e da
  `port`), §5.2 (origem: a proibição da aresta `domain → port`), §5.3 (por que não
  uma subclasse de porta) e §9.3 (o que o domínio recebe em vez de buscar).
- SPEC-DBTRMM3X — spec que esta série de ADRs implementa.
- ARQ-448 — ARQ-448 (destino da série de
  ADRs do DMPF).
