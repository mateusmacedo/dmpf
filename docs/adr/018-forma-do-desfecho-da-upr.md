# ADR-018: Adotar a união exaustiva como forma observável do desfecho da UPR

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

## Contexto

A unidade de processamento de requisição (UPR) é a menor unidade de comportamento
de domínio: recebe uma mensagem semanticamente tipada e o estado sobre o qual ela
incide, e devolve o resultado da decisão. Toda execução precisa terminar em um
desfecho explícito, mas "explícito" não basta — falta dizer qual é a **forma**
desse desfecho e como o chamador da UPR, o application service, lê o que
aconteceu.

Duas forças puxam essa forma. A primeira é a exaustividade: quem chama precisa
saber, no próprio ponto da chamada, se a decisão aceitou ou recusou, sem terceiro
caminho, sem valor ausente e sem estado indeterminado. A segunda é a natureza da
recusa. Uma rejeição de negócio — pedido vazio, saldo insuficiente, item
duplicado — é resultado **normal e esperado** de uma regra de domínio, não uma
falha. Se ela chega ao chamador por um canal que ele não consegue separar de um
timeout ou de uma indisponibilidade de infraestrutura, a exaustividade se perde
justamente onde ela valeria: no consumidor.

Some-se a isso a restrição de paridade entre stacks. Go e TypeScript precisam
realizar o mesmo desfecho, e a premissa do épico é que essa paridade é
**conceitual, não sintática**. Impor uma construção de linguagem específica
excluiria um kernel; não dizer nada permitiria uma realização que se declarasse
conforme "por devolver uma rejeição tipada" enquanto, na prática, a entrega
ocorre por um mecanismo indistinguível de falha técnica. A base conceitual
anterior agrava o problema: ela modelava o desfecho como produto de sucesso com a
rejeição em um braço separado, o que este documento precisa reconciliar em uma
forma única.

## Decisão

Fixar o desfecho de toda UPR como uma **união exaustiva de dois ramos**, na forma
`Decision = Accepted(resposta, eventos) | Rejected(rejeição)`:

- **Exaustividade e exclusividade.** Não há terceiro desfecho; ausência de
  retorno, valor nulo e desfecho indeterminado não são conformes. Os dois ramos
  se excluem — não existe desfecho parcialmente aceito.
- **Rejeição como dado de domínio tipado.** A recusa de negócio chega ao chamador
  pelo próprio desfecho, carregando um **código estável** no formato
  `contexto/motivo` (por exemplo, `orders/empty-order`) e uma mensagem endereçada
  ao domínio; detalhes estruturados, quando presentes, ficam em tipos de domínio.
  A rejeição **não** carrega código HTTP ou gRPC, política de retry nem causa
  técnica encadeada — os três primeiros são mapeamento de borda, e o quarto não
  existe, porque a UPR não faz I/O.
- **Pós-condição da recusa.** Após um desfecho `Rejected`, o estado observável do
  alvo é idêntico ao de antes da chamada: nenhuma mutação parcial sobrevive, e o
  ramo de recusa não carrega eventos de domínio. Se a regra recusou, nada
  aconteceu no domínio.
- **A forma é normativa; o mecanismo é de cada kernel.** Nenhuma construção
  sintática é imposta. Uma realização é conforme se, na fronteira observável, o
  ramo é distinguível sem inspecionar texto de mensagem, a rejeição tipada com
  código estável é acessível e a exaustividade é verificável no chamador. Não são
  conformes: rejeição entregue por lançamento, ainda que a exceção seja tipada e
  classificada, nem entrega pelo mesmo canal por onde trafegam falhas de
  infraestrutura. Um par `(decisão, erro)` em Go pode ser conforme porque, como a
  UPR não executa I/O, nenhuma falha técnica se **origina** nela: o canal de erro
  transporta apenas rejeições de domínio, e o chamador as distingue por tipo ou
  valor.

Esta decisão reconcilia a base conceitual anterior, que modelava o desfecho como
produto de sucesso com a rejeição fora dele, substituindo-a pela união exaustiva
acima.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Impor a união discriminada como **mecanismo** obrigatório, e não apenas como forma | Daria fidelidade máxima ao enunciado da spec, mas contraria a premissa do épico de que a paridade entre stacks é conceitual, não sintática, e antecipa aqui uma decisão que pertence aos épicos de kernel (§3.4). |
| Entregar a rejeição de negócio por **lançamento** de exceção, ainda que tipada e classificada | Um lançamento interrompe o fluxo do chamador em vez de lhe devolver um desfecho, então a exaustividade deixa de ser verificável no ponto da chamada; e mistura a rejeição de negócio com o erro de programação, que a base conceitual separa como falha inesperada, forçando o chamador a distinguir os dois por texto (§3.4). |
| Tratar o desfecho como **produto de sucesso** com a rejeição em um braço separado, seguindo a realização atual dos drafts das duas stacks | Contradiz as decisões técnicas da spec aprovada, que fixam a rejeição como valor devolvido dentro de uma união exaustiva, não como braço à parte de um produto (§3.4). |

## Consequências

**Positivas:**

- A exaustividade fica verificável no ponto da chamada: o chamador não consegue
  ignorar o ramo de recusa nem confundi-lo com sucesso.
- A rejeição de negócio fica separada da falha técnica na fronteira observável —
  o application service nunca toma uma pela outra, porque a recusa é dado tipado e
  a falha técnica trafega por outro canal.
- As duas stacks ficam vinculadas por um contrato conceitual, não sintático: cada
  kernel escolhe o mecanismo idiomático (união discriminada, par com
  discriminante, tipo de resultado) desde que as observações na fronteira sejam
  equivalentes.
- A pós-condição de estado converte uma garantia da camada de persistência em
  garantia da própria decisão: nenhum agregado sujo sobrevive a uma recusa dentro
  do escopo do caso de uso.

**Negativas:**

- **Custo aceito:** ao deixar o mecanismo livre em vez de impor a união
  discriminada, abre-se mão da fidelidade máxima ao enunciado — duas realizações
  conformes podem diferir na sintaxe, então a conformidade deixa de ser conferível
  por um único formato canônico e passa a exigir a verificação da tabela de
  equivalência observável de §3.4 em cada stack, revisão que é mais custosa do que
  reconhecer uma forma única.
- As realizações atuais que tratavam o desfecho como produto de sucesso com a
  rejeição fora dele precisam ser reescritas para a união exaustiva; a
  reconciliação assume esse retrabalho nos drafts das duas stacks.
- O canal de exceção fica vedado para a rejeição de negócio mesmo quando tipado:
  um kernel cujo idioma favoreça exceções classificadas perde esse caminho e passa
  a devolver a rejeição como valor.

## Referências

- ADR-014 — proíbe a aresta `domain → port` e reserva `port` à fronteira. Importa
  aqui porque, sem porta no domínio, a UPR não alcança I/O; é o que garante, junto
  com o ADR-016, que nenhuma falha técnica se origina na unidade — premissa de que
  o canal de erro de uma UPR em Go transporta apenas rejeições de domínio.
- ADR-016 — domínio executável em memória por critério de duas metades. Importa
  aqui porque a distinguibilidade do desfecho repousa em "a UPR não executa I/O":
  timeout, indisponibilidade e falha de serialização acontecem fora dela, então o
  desfecho devolvido carrega apenas rejeição de negócio, nunca causa técnica
  encadeada.
- `docs/dmpf/upr-decision-mensagens.md` (FND-03) — §3.1 (o desfecho é uma união
  exaustiva), §3.3 (o ramo `Rejected`, o conteúdo da rejeição e a pós-condição de
  estado) e §3.4 (realização por kernel e equivalência observável): origem desta
  decisão.
- SPEC-DBTRMM3X — spec que esta série de ADRs implementa.
- ARQ-448 — ARQ-448 (destino da série de
  ADRs do DMPF).
