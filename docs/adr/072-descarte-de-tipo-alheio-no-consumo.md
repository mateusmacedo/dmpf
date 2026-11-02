# ADR-072: Descartar no adapter de consumo o tipo alheio de um tópico multi-tipo, com o conjunto declarado no canal

## Status

Aceito — 2026-11-01. Implementa SPEC-93FRCDJV. Supersede em parte o [ADR-041](./041-sdk-de-referencia-generator-e-bom-certificado.md): a assinatura por tipo deixa a ponte do app (o `Sink` local do `reservations`) e passa ao adapter de consumo do kernel. Complementa o [ADR-036](./036-classificacao-de-recepcao-e-fronteira-pending.md), no que fica fora das sete disposições, e o [ADR-039](./039-providers-de-transporte-sink-e-gesto-de-release.md), na ponte por `Sink`.

## Contexto

O `KFK-03b` (`docs/dmpf/politicas-transporte.md`) admite vários tipos de evento no mesmo tópico, desde que o conjunto seja fechado e declarado na catalogação do canal. O kernel não tinha onde fazer essa declaração: o `channel.Channel` levava um único `EventType` e um único `ContractRef`. Mesmo assim, o `orders` publicava `order-placed.v1` e `item-added.v1` no mesmo tópico.

O consumidor que assina só parte dos tipos não tinha regra no kernel. O ADR-041 pôs a assinatura por tipo na ponte, e o `reservations` a realizou com um `Sink` próprio no bloco `app`, que dava `Ack` a todo tipo diferente do assinado e registrava apenas um log em debug. Esse `Sink` não media o descarte e não distinguia um tipo alheio de um tipo que o produtor não deveria publicar. Num consumidor sem esse `Sink`, o tipo alheio chegava ao `Handler`, falhava no `envelope.Unpack` e ia para a quarentena como R1×D4 (`terminal-failure`).

O caso que motivou a mudança é o Gateway do antecipa-market. Ele assina parte dos tipos do `partner.events` e ia copiar o `Sink` do `reservations`.

## Decisão

1. **O canal declara o conjunto que carrega.** `channel.Channel.Events []Event`, com `Event{Type, ContractRef}`, substitui `EventType` e `ContractRef`. No caso de `KFK-03`, o conjunto tem um elemento. `ContractMajor` e `Ordering` continuam no canal e valem para o conjunto inteiro. `channel.WireTypes` devolve os tipos de fio (`<tipo>.v<major>`) na ordem declarada.

2. **O consumidor declara o subconjunto que assina.** `kernelapp.Consumer.Accepts` lista os tipos de fio que o `Handler` trata, e `kernelapp.Channel.Carries` lista os tipos de fio do canal. Um `Accepts` vazio, repetido ou fora de `Carries` recusa a partida (`ErrSubscriptionOutsideChannel`).

3. **O passo 1 de §6.3 decide o tipo, depois da fronteira.** A ordem fica: envelope inválido, origem fora da fronteira, tipo fora de `Carries`, tipo fora de `Accepts` e, por fim, o `Handler`. A fronteira vem antes do tipo porque uma origem não confiável é um fato de segurança, qualquer que seja o tipo.

4. **O tipo alheio é descartado.** Um tipo que está em `Carries` mas não em `Accepts` recebe `Ack`. Não passa pelo `Handler`, pela inbox, pela quarentena nem pela DLQ, não abre span `process` e não registra `messaging.process.duration`. Cada descarte confirmado soma 1 em `dmpf.consumer.discarded`, que tem `cloudevents.event_type` como label. A cardinalidade do label fica limitada ao conjunto declarado. O descarte não é disposição (`INB-10`) e vale igual em Kafka e SQS, porque o gesto é o `Ack` de sempre.

5. **O tipo não declarado é contido.** Um tipo fora de `Carries` é o "schema desconhecido" de §7.4. Ele vai para a quarentena com o motivo `undeclared-type`, com os bytes recebidos, e só é confirmado depois.

6. **A ponte é do kernel.** `kernelapp.Sink` realiza o `Sink` de Kafka e de SQS sobre o `Consume`, sem filtro próprio. Nenhum contexto filtra tipo no bloco `app`.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Manter um `Sink` local por app, como no `reservations` | Cada consumidor reescreve o filtro, e o descarte não é medido. O tipo desconhecido também recebe `Ack` em silêncio, contra §7.4 |
| Descartar todo tipo fora do subconjunto assinado, sem conjunto declarado no canal | Não distingue tipo alheio de tipo desconhecido, e um erro do produtor sumiria sem rastro |
| Oitava disposição (`Discarded`), devolvida pelo `Handler` | §6.4 é fechada em sete. O `Handler` roda depois do passo 1, quando já montou `Receipt` e contexto, e o `INB-10` põe fora das disposições o que não chega à inbox |
| Quarentena com motivo próprio para o tipo alheio | O tipo alheio não é defeito. Tráfego legítimo encheria a quarentena |
| Manter `EventType` como tipo principal e acrescentar uma lista de extras | Daria ao canal multi-tipo um tipo privilegiado sem razão normativa e criaria dois caminhos de declaração |
| Filtrar no worker do Kafka | O SQS ficaria de fora, e o envelope seria decodificado duas vezes |

## Consequências

**Positivas:**

- O descarte de tipo alheio sai de cada app e fica num lugar só, igual para Kafka e SQS, e passa a ser medido (`dmpf.consumer.discarded`, `MET-21`).
- O tipo que o produtor publica sem declarar deixa de ser confirmado em silêncio: vai para a quarentena como `undeclared-type`, e o `postgres.InboxSignals` o conta em campo próprio (GAR-12), separado de `Other`.
- O canal passa a declarar o conjunto que carrega (`KFK-03b`). Assim, um consumidor novo sabe o que pode assinar sem ler o mapper do produtor.

**Negativas:**

- A API pública de `transport/channel` e de `app` muda: `NewKafka` passa a receber os eventos por parâmetro variádico, e `EventTypeOf` sai. **Custo aceito:** a mudança sai numa pré-release da linha `1.0.0-rc`, e o consumidor externo se ajusta na adoção ([ADR-060](./060-o-dmpf-tem-consumidor-externo.md)).
- O canal de um produtor que publica mais de um tipo precisa declarar todos eles: um tipo publicado e não declarado vai para a quarentena de cada consumidor. **Custo aceito:** os canais do workspace declaram o que os mappers publicam, e um teste por app prova isso.
- O descarte não deixa span. **Custo aceito:** o sinal operacional é a série, e o rastro fica no log em debug. Um span por descarte custaria um span por mensagem alheia, que é a maior parte do tráfego de um tópico multi-tipo.
- O `undeclared-type` vai para a quarentena, não para a DLQ. **Custo aceito:** segue o `ContainmentMap`, que manda todo motivo para a quarentena enquanto o DLQ não tem realização no adapter.
- O `ContractMajor` é um só por canal. Para levar um evento do conjunto a `.v2`, o canal inteiro muda de major, e a coexistência de `v1` e `v2` do mesmo tipo não pode ser declarada no mesmo canal. **Custo aceito:** o limite vem do `KFK-03b` ("variação modelada no contrato"). Um caso real que exija majors por evento precisa de um ADR próprio.

## Referências

- SPEC-93FRCDJV: descarte de tipo alheio em tópico multi-tipo.
- `docs/dmpf/politicas-transporte.md`: `KFK-03`, `KFK-03b` e `KFK-03c`.
- `docs/dmpf/uow-inbox-outbox.md`: §6.3, §6.4, §7.4 e `INB-10`.
- `docs/dmpf/resiliencia-observabilidade.md`: `MET-21`.
