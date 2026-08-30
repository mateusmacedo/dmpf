# ADR-010: Adotar a regra de dependência como função de decisão sobre seis blocos de pertencimento único

## Status

Aceito — 2026-08-28. Implementa SPEC-DBTRMM3X.

## Contexto

O DMPF precisa de um critério verificável que responda a duas perguntas que o
parque atual mantém embaralhadas: a que categoria cada pedaço de código pertence
e quais dependências entre categorias são legítimas. Hoje o layout não carrega
essa informação. O diretório `domain/` designa três coisas diferentes conforme o
repositório — entidades de negócio em um, o modelo da capability técnica da
própria biblioteca em outro, DTOs de transporte com decorators de framework em um
terceiro (RFC §4.4) — e há repositório sem `domain/` algum que ainda assim
concentra regra de negócio distribuída junto de GORM, SQS e HTTP. Inferir a
categoria pelo nome do diretório erraria na maioria desses casos.

As violações efetivamente observadas no inventário concentram-se em `domain`,
`application` e `port` importando infraestrutura ou wire (RFC §7.4) — o retrato da
distância entre o parque e a norma que se quer fixar. A dificuldade é que a regra
"a dependência aponta para dentro" carrega duas exigências que se tensionam. De um
lado, ela precisa ser objetiva e mecanicamente checável por imports, para não
depender de julgamento caso a caso. De outro, a mesma aresta `domain → domain` é
legítima dentro de um bounded context e ilegítima entre contexts distintos
(RFC §5.5); uma matriz que decida apenas sobre pares de blocos não consegue dizer
as duas coisas com uma única célula.

Somam-se dois riscos. Toda categoria de exclusão do universo verificável — teste,
código gerado, migration — é um incentivo a esconder ali a aresta proibida
(RFC §4.5). E uma unidade que agrega responsabilidades distintas resiste a
qualquer classificação única, empurrando o critério a uma escolha arbitrária
(RFC §4.2). Por fim, há um limite assumido: o verificador prova que os imports são
compatíveis com a classificação declarada, não que a classificação corresponde à
responsabilidade real do código (RFC §4.0). O ADR precisa fixar o conjunto de
blocos, o critério de pertencimento e a forma da regra de dependência de modo que
essas forças fiquem resolvidas ao mesmo tempo.

## Decisão

Adotar seis blocos — domain library, application service, app, port, provider e
contract package (RFC §4.1) — com **pertencimento único** por
`verification_unit`. O bloco é determinado por um classificador total e ordenado:
a primeira condição satisfeita fixa o bloco, e a linha final é fail-closed —
nenhuma unidade de produção fica sem bloco, e uma unidade não classificável
reprova em vez de escapar (RFC §4.3). Quando uma unidade parece caber em mais de
um bloco, o desempate segue, nesta ordem: exclusão vence pertencimento,
responsabilidade dominante e, no empate irredutível, divisão da unidade em vez de
escolha (RFC §4.2). A classificação é declarada por unidade no `metadata_container`
e revisada, nunca inferida do nome de diretório, arquivo ou pacote — premissa
recebida da decisão de ADR-012 (RFC §4.4), não tomada aqui.

A regra de dependência **não é uma célula de matriz**, e sim uma função
`decide(source_block, target_block, source_bc, target_bc, target_surface)`,
definida como a conjunção de duas condições independentes (RFC §7.1):

- **C1 — bloco:** o par `(source_block, target_block)` é permitido na matriz 6×6
  de blocos (RFC §7.3).
- **C2 — contexto:** o predicado de contexto entre origem e destino. Sua definição
  — `same_bounded_context`, `public_integration_surface` e o diagnóstico
  `DMPF-M002` — é fixada em ADR-017 (RFC §7.2), que institui o `bounded_context`
  sobre o qual C2 decide; este ADR consome o predicado, não o define.

Reprovar em qualquer uma reprova a aresta. As 36 células têm decisão e fonte
normativa (RFC §7.4), com diagnóstico estável — `DMPF-D001` quando C1 falha,
`DMPF-D002` quando C2 falha (RFC §7.1). É assim que "a dependência aponta para dentro" vira
verificável por imports sem multiplicar a matriz por contexto. A regra anti-bypass
fecha o uso das exclusões do universo para esconder arestas (RFC §4.5), e o padrão
outbox é decomposto por responsabilidade — escrita no application service,
persistência no provider, drenagem no app —, com a aresta `application → provider`
permanecendo proibida (RFC §7.5).

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Matriz 6×6 pura de blocos, em que cada célula é a decisão completa, sem separar a dimensão de bounded context. | A mesma aresta `domain → domain` é permitida dentro de um context e proibida entre contexts, e uma única célula não diz as duas coisas. A matriz responde "esse tipo de código pode depender daquele tipo de código?"; o predicado de contexto responde "essas duas partes do sistema podem se falar?". Separar C1 de C2 é o que torna a regra inter-context verificável sem multiplicar a matriz por contexto (RFC §7.2). |
| Resolver o empate de classificação por escolha arbitrária de um bloco, mantendo a unidade agregada como está. | Uma unidade que é genuinamente `domain` e `provider` ao mesmo tempo não tem classificação correta; tem tamanho errado. Forçar uma escolha preservaria o pertencimento único só na aparência. A regra da divisão é o que mantém a exigência de pertencimento único sem arbitrar — o empate irredutível é sintoma de unidade mal dimensionada (RFC §4.2). |
| Confiar apenas nas três primeiras regras anti-bypass (dependência sobre elemento excluído, código gerado, testes), sem a regra da rotulagem oportunista. | As três primeiras criam categorias de exclusão, e toda categoria de exclusão é um incentivo. Sem a quarta regra, fica aberto o caminho de burla mais barato: marcar código de produção como gerado, migration ou teste para movê-lo para fora do que o verificador olha (RFC §4.5). |

## Consequências

**Positivas:**

- A regra "a dependência aponta para dentro" passa a ser decidível por leitura de
  imports: cada aresta recebe veredito PERMITIDA ou PROIBIDA da função `decide`,
  sem julgamento caso a caso (RFC §7.1).
- Todo módulo de produção passa a declarar `block` e `bounded_context` por unidade
  no `metadata_container` para ser verificável — a classificação declarada por
  unidade (ADR-012) vira dado de entrada explícito do verificador (RFC §10.1).
- A revisão de código ganha base normativa para recusar produção rotulada como
  teste, migration ou código gerado: a regra da rotulagem oportunista dá o
  critério que antes ficava a cargo do julgamento do revisor (RFC §4.5).
- O verificador emite exatamente dois diagnósticos de dependência — `DMPF-D001`
  (C1) e `DMPF-D002` (C2) —, o que reduz os vetores de teste da regra a dois
  códigos estáveis e rastreáveis (RFC §10.3).
- As cinco células com violação observada no inventário — 4, 5, 6, 11 e 23, todas
  com `domain`, `application` ou `port` importando infraestrutura ou wire — passam
  a ser os primeiros alvos de correção (RFC §7.4).

**Negativas:**

- A condição C2 depende de um `bounded_context` declarado e obrigatório em todo
  módulo, campo que este ADR consome mas não institui: sua adoção — e o custo de
  migração, já que nenhum dos dez repositórios inventariados nomeia bounded
  contexts hoje — é decidida e contabilizada em ADR-017 (RFC §5.4), não aqui.
  Enquanto o campo não existir, C2 não pode ser avaliada e a unidade reprova por
  ausência.
- O verificador prova que os imports são compatíveis com a classificação
  declarada, não que a classificação corresponde à responsabilidade real do
  código; essa correspondência fica com a revisão humana (RFC §4.0).
- Das 36 células da matriz, 20 não têm evidência no universo inventariado
  ("não observado"): a decisão dessas células apoia-se apenas na fonte normativa,
  sem lastro empírico (RFC §7.4).
- A regra anti-bypass da rotulagem oportunista é obrigação declarada que o
  verificador nem sempre detecta — depende de disciplina de autoria e de revisão
  para valer (RFC §4.5).
- **Custo aceito:** manter o pertencimento único obriga a dividir unidades hoje
  agregadas. Quando o desempate cai na regra da divisão (RFC §4.2), a unidade mal
  dimensionada é refatorada em unidades de bloco único, não resolvida por escolha
  arbitrária — trabalho de refatoração adicional sobre um parque que concentra
  violações justamente nos blocos `domain`, `application` e `port` (RFC §7.4).

## Referências

- ADR-012 — classificação por metadado declarado. Institui `block` e
  `bounded_context` como metadados autodeclarados pela unidade, nunca inferidos do
  layout; esta decisão pressupõe a classificação já declarada por unidade e não
  decide como se classifica.
- ADR-017 — bounded context declarado e superfície pública. Institui o
  `bounded_context` obrigatório e define o predicado de contexto C2
  (`same_bounded_context`, `public_integration_surface`, `DMPF-M002`) que a função
  `decide` consome; o custo de adoção do campo é contabilizado lá, não aqui.
- ADR-011 — verification_unit como unidade arquitetural. Define a
  `verification_unit` sobre a qual incidem o pertencimento único dos seis blocos e
  a regra de dependência desta decisão.
- SPEC-DBTRMM3X — especificação da série de ADRs do DMPF que esta decisão implementa.
- RFC DMPF Foundation v0.1 — `docs/dmpf/rfc-dmpf-foundation-v0.1.md`:
  - **Origem:** §4 (os seis blocos §4.1, a precedência de desempate §4.2, o
    classificador total §4.3, a regra anti-bypass §4.5) e §7 (a função `decide`
    §7.1, a matriz de blocos da condição C1 §7.3, a decisão por célula §7.4, o
    bloco dono do outbox §7.5).
  - **Apoio:** §10.1 (schema do metadado com `block` e `bounded_context`) e §10.3
    (diagnósticos `DMPF-D001` e `DMPF-D002`).
- ARQ-448 — https://lider-cap.atlassian.net/browse/ARQ-448 (destino da série de
  ADRs do DMPF).
