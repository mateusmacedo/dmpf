# ADR-016: Adotar domínio executável em memória por critério de duas metades

## Status

Aceito — 2026-08-28. Implementa SPEC-DBTRMM3X.

## Contexto

A `domain library` concentra as regras e os invariantes de negócio e, pela regra
de dependência do acervo (ADR-010), é o núcleo para onde toda dependência aponta. Para que
esse núcleo permaneça independente de transporte, persistência e infraestrutura
e permaneça determinístico (princípio 8, RFC §2.1), é preciso um critério que
diga, de forma verificável, quando uma `domain library` está de fato livre de
I/O. Sem esse critério, "domínio puro" vira afirmação de intenção, não fato
auditável.

A dificuldade é que a ausência de I/O tem duas manifestações que não se
implicam. O grafo de imports pode estar limpo enquanto o código lê uma variável
de ambiente ou consulta o relógio por um caminho que o import não revela. No
sentido inverso, uma suíte de testes pode não tocar disco por acaso, sem jamais
exercitar o caminho que tocaria. Uma frente sozinha deixa a outra aberta.

Some-se a isso a força cultural mais insidiosa: a mockabilidade, usada como
argumento para legitimar justamente a dependência que a regra quer proibir — "o
domínio depende do repositório, mas tudo bem, o repositório é mockável". A
evidência do parque (RFC §9.4) confirma que o risco já se materializou: há
repositórios cuja `domain library` importa `@nestjs/swagger`, `class-validator`,
`@prisma/client` e, no caso mais grave, `mssql` na assinatura de uma porta. O
critério precisa fechar as duas frentes ao mesmo tempo e recusar o atalho do
duplo de teste, sob pena de ser inverificável ou contornável.

## Decisão

Adotar, para toda `domain library`, o critério de **domínio executável em
memória**, composto por duas metades necessárias e conjuntas (RFC §9.1):

- **Estática** — o fechamento transitivo de imports da unidade contém apenas
  a capability `pure` decidida pelo ADR-015 (RFC §6), verificável em modo
  `import-verifiable`.
- **Dinâmica** — executar seus testes não inicia processo, não abre socket, não
  toca disco e não depende de horário, verificável em modo `runtime-testable`.

Nenhuma das metades satisfaz o critério isoladamente: a conformidade exige as
duas ao mesmo tempo.

Decidir também (RFC §9.2) que substituir uma dependência de infraestrutura por
mock, fake ou stub **não** torna a unidade conforme. A necessidade de um duplo
de teste dentro do domínio é sintoma de violação de RFC §6.2; o duplo legítimo é o
de uma **porta** e pertence à camada de aplicação, que é quem possui a porta.

Como decorrência do princípio 8 (RFC §9.3), o domínio recebe hora atual,
identificadores, aleatoriedade, dados de outros agregados e configuração como
valores já resolvidos pelo `app`, em vez de buscá-los. Cada linha dessa tabela
entrega ao domínio um valor já resolvido; com isso, não resta recurso que ele
precise obter por uma porta. Fechar a aresta `domain → port` é decisão do
ADR-014 (RFC §5.2), não deste ADR — a tabela de RFC §9.3 apenas retira o caso residual
que a tornaria necessária. As duas decisões se sustentam mutuamente: a proibição
do ADR-014 só fica sem exceção graças a esta tabela, e esta tabela só tem efeito
prático porque o ADR-014 proíbe a aresta.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Exigir apenas a metade estática — considerar conforme toda unidade cujo fechamento transitivo de imports contenha somente pacotes puros | Insuficiente: uma unidade pode importar apenas pacotes puros e ainda assim ler uma variável de ambiente ou chamar o relógio por um caminho que o grafo de imports não distingue (RFC §9.1). |
| Exigir apenas a metade dinâmica — considerar conforme toda unidade cujos testes não iniciem processo nem toquem I/O em execução | Insuficiente: um teste pode passar por acaso, sem exercitar o caminho que toca I/O, deixando a dependência de infraestrutura latente e não detectada (RFC §9.1). |
| Aceitar a mockabilidade como satisfação — legitimar a dependência de infraestrutura no domínio desde que ela seja substituível por mock, fake ou stub | Inverte a regra: a facilidade de substituir o I/O não elimina o acoplamento a ele, apenas o torna confortável. A SPEC-8YVF0RR5 antecipa este caso ao exigir a demonstração inclusive quando a dependência proibida seria mockável (RFC §9.2). |

## Consequências

**Positivas:**

- Domínio testável em memória, sem orquestração externa (container, broker,
  banco): testes rápidos, determinísticos e reproduzíveis.
- Critério verificável mecanicamente em duas frentes complementares
  (`import-verifiable` e `runtime-testable`), fechando o buraco que cada metade
  deixaria sozinha.
- Remove o argumento da mockabilidade como legitimador de acoplamento a I/O,
  eliminando a rota mais comum de erosão da regra (RFC §9.2).
- Entrega ao domínio, pela tabela de RFC §9.3, todo recurso volátil (hora,
  identificador, aleatoriedade, dados de outro agregado, configuração) como
  valor já resolvido pelo `app`, retirando o caso residual que faria a aresta
  `domain → port` — proibida pelo ADR-014 (RFC §5.2) — parecer necessária.

**Negativas:**

- **Custo aceito:** a distância entre a norma e o parque é material e já está
  medida — `telesena-ativavel-services`, `telesena-live-services` e
  `telesena-titulos-services` violam a metade estática hoje, o último na
  assinatura de uma porta (`mssql`). A decisão assume esse débito de migração; a
  adoção fica para os épicos de kernel (RFC §1.4) e não é resolvida por este ADR
  (RFC §9.4).
- As assinaturas do domínio ganham aridade: cada recurso antes buscado (relógio,
  gerador de identificador, semente de aleatoriedade, configuração) passa a ser
  argumento de entrada, transferindo a responsabilidade de resolução para o
  `app` (RFC §9.3).
- Perde-se o atalho de testar via mock dentro da `domain library`: qualquer
  duplo de infraestrutura precisa migrar para a camada de aplicação, dona da
  porta, reescrevendo suítes que hoje mockam dependências no próprio domínio
  (RFC §9.2).

## Referências

- ADR-010 — regra de dependência e seis blocos. Estabelece a `domain library`
  como núcleo para onde toda dependência aponta; é essa regra que exige o núcleo
  puro e, portanto, torna necessário o critério deste ADR.
- ADR-014 — proibição da aresta `domain → port` (RFC §5.2). Possui a proibição
  que a tabela de RFC §9.3 deste ADR torna livre de caso residual; a proibição em si é
  decisão do ADR-014, não deste, e as duas se sustentam mutuamente.
- ADR-015 — capabilities externas por bloco (RFC §6). Decide a capability `pure`
  sobre a qual repousa a metade estática do critério: exigir que o fechamento
  transitivo de imports contenha apenas `pure` é aplicar ao domínio a decisão do
  ADR-015.
- RFC §9.1 — as duas metades necessárias e conjuntas, estática e dinâmica, do
  domínio executável em memória.
- RFC §9.2 — a recusa da mockabilidade como satisfação do critério.
- RFC §9.3 — a tabela dos recursos entregues ao domínio como valores já
  resolvidos pelo `app`.
- RFC §9.4 — a evidência do parque que fundamenta o custo aceito.
- RFC §2.1 — o princípio 8 (determinismo), do qual a tabela §9.3 decorre.
