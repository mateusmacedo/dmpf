# ADR-019: Separar os três níveis de contrato e converter evento de domínio em evento de integração fora do domínio

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

## Contexto

Um mesmo fato de negócio — "o pedido foi feito" — aparece em três lugares de um
sistema orientado a mensagens, e é tentador tratá-los como um só. Ele nasce como
evento interno do domínio (`OrderPlaced`), volta como resultado de um caso de uso
(a resposta de aplicação) e, quando outros contextos precisam saber, sai como
contrato público versionado no wire (`company.orders.event.v1.OrderPlaced`). A
proximidade dos nomes esconde que cada um vive em um bloco diferente, evolui em
ritmo diferente e tem dono diferente.

Duas economias aparentes puxam na direção errada. A primeira é reaproveitar um
único tipo: o tipo gerado do contrato de wire chega pronto, com serialização
embutida, e escrever um segundo tipo para o domínio soa como duplicação gratuita.
A segunda é publicar o evento de domínio direto, sem conversão: se o domínio já
emitiu `OrderPlaced`, por que não deixá-lo atravessar a fronteira como está? Cada
economia tem um custo que só aparece depois, e é caro de reverter quando aparece.

Há ainda uma fronteira a respeitar. Decidir que a conversão ocorre fora do
domínio não é o mesmo que decidir em qual bloco externo o mapeamento reside —
adapter, app ou outro. Essa escolha é governada pela regra de dependência
do acervo (a matriz que proíbe a aresta `application → contract` e autoriza
`app → contract`) e pertence às sub-specs de Unit of Work e de contratos de wire,
não a esta decisão. Este ADR fixa a separação e a direção da conversão; o endereço
do mapeamento fica de fora, por delegação explícita.

## Decisão

Adotar a separação dos três níveis de contrato — domínio, aplicação e wire —
como modelos distintos, com conversão explícita entre domínio e wire e recusa de
qualquer tipo único que atravesse os níveis. Fixa-se, no lado do domínio, que é o
alcance desta decisão:

- **Nenhum tipo atravessa os três níveis** (`CTR-01`). Os três são modelos
  separados, e o nome compartilhado do conceito não implica definição
  compartilhada.
- **O tipo gerado do contrato de wire não entra no domínio** (`CTR-02`): não é
  estado de domínio, entrada de UPR nem evento de domínio. É a mesma regra que,
  do lado da unidade de decisão, proíbe a UPR de receber ou produzir tipos
  gerados de Protobuf.
- **O tipo de domínio não é exposto como contrato público** (`CTR-03`):
  consumidores nunca importam os tipos do domínio do produtor.
- **A conversão de domain event para integration event ocorre fora do domínio**,
  na forma canônica `mapear(domain event, contexto de publicação) -> integration event`
  (`CTR-04`). Ela não roda dentro da UPR.
- **O evento de domínio não atravessa inteiro** (`CTR-05`): só a identidade do
  agregado, os fatos de que outros contextos precisam, o instante e a versão do
  contrato deixam o domínio; estado intermediário, estrutura interna e vocabulário
  não publicado permanecem privados.
- **A conversão é unidirecional** (`CTR-06`): não existe caminho de volta do
  integration event ao domain event do produtor.
- **A promoção a contrato público é decisão declarada** (`CTR-07`): nenhum evento
  de domínio é público por ter sido emitido; expô-lo é ato explícito, nunca
  consequência automática.

Em qual bloco externo o mapeamento reside não é decidido aqui: a regra de
dependência do acervo já o restringe, e a escolha cabe às sub-specs de transação
e de wire dentro do que a matriz permite.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Reaproveitar um único tipo gerado do contrato de wire nos três níveis, evitando escrever um modelo próprio de domínio | O ganho é imediato — serialização de graça — mas o custo aparece depois: o formato de wire passa a ditar a forma do domínio (campos opcionais onde a regra exige presença, inteiros onde há valor monetário, ausência de invariante no construtor), e cada mudança do contrato público vira mudança de domínio. O tipo separado não é duplicação; é a fronteira que permite a contrato e domínio evoluírem em ritmos distintos (§6.2, bloco `rationale`). |
| Publicar o evento de domínio direto, sem conversão, promovendo por padrão todo evento emitido | Quando ser publicado passa a ser consequência de ter sido emitido, o conjunto de contratos públicos fica definido por acidente de implementação: uma refatoração interna que renomeie ou divida um evento quebra consumidores externos que ninguém sabia existir (§6.3, bloco `rationale` de `CTR-07`). |

## Consequências

**Positivas:**

- Contrato público e domínio evoluem em ritmos distintos: mudança no formato de
  wire não força mudança de domínio, e refatoração interna do domínio não quebra
  consumidores externos.
- O conjunto de contratos públicos passa a ser definido por decisão explícita
  (`CTR-07`), não por acidente de emissão — a superfície de integração fica
  visível e intencional.
- A conversão parcial e unidirecional (`CTR-05`, `CTR-06`) mantém o vocabulário
  interno privado e impede que o contrato público acople o domínio de volta a si.
- O domínio permanece livre de tipos gerados de wire (`CTR-02`), preservando a
  sua execução em memória sem infraestrutura de serialização embutida.

**Negativas:**

- **Custo aceito:** manter um modelo por nível — três tipos onde a reutilização
  ofereceria um — e escrever um mapeamento explícito para cada evento promovido.
  É código a mais e serialização que não vem de graça do tipo gerado, aceito em
  troca da fronteira que sustenta os ritmos distintos de evolução.
- Promover um evento a contrato público vira passo deliberado: quem quer expor um
  fato precisa declará-lo e mapeá-lo, sem o atalho da publicação automática —
  mais atrito por novo contrato de integração.
- A decisão fixa a separação e a direção, mas deixa em aberto o endereço do
  mapeamento: cada kernel ainda precisa alocar o mapper no bloco que a matriz de
  dependência permite, uma escolha que este ADR delega em vez de encerrar.

## Referências

- ADR-010 — regra de dependência e os seis blocos. Fixa a função `decide(...)` e
  a matriz que governa em que bloco cada unidade pode residir e de quem pode
  depender. Este ADR não decide onde o mapeamento de conversão vive: delega esse
  ponto à regra do ADR-010 e às sub-specs de transação e de wire, dentro do que a
  matriz permite.
- ADR-016 — domínio executável em memória por critério de duas metades. Decide o
  critério de pureza do domínio; é essa pureza que a separação de níveis protege
  no nível da mensagem, ao impedir (`CTR-02`) que um tipo gerado de wire, com
  serialização e infraestrutura embutidas, entre no domínio como estado ou evento.
- ADR-017 — bounded context declarado e interação só por superfície pública.
  Decide que contexts só se falam por superfície pública; é por isso que um tipo
  de domínio não é contrato público (`CTR-03`) e que a promoção a evento de
  integração — a superfície pública do fato — é decisão declarada (`CTR-07`), não
  consequência automática de ele ter sido emitido.
- Artefato de origem — `docs/dmpf/upr-decision-mensagens.md` (FND-03): §6.1 (os
  três níveis de contrato), §6.2 (não existe DTO universal — `CTR-01` a `CTR-03`)
  e §6.3 (conversão de domain event para integration event — `CTR-04` a `CTR-07`).
- SPEC-DBTRMM3X — especificação da série de ADRs do DMPF, que este ADR implementa.
- ARQ-448 — ARQ-448 (FND-11, redação,
  promoção e aceite da série de ADRs do DMPF).
