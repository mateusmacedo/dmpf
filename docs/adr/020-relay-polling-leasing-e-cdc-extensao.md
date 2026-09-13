# ADR-020: Adotar polling com leasing como relay padrão da outbox, com CDC como extensão

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

## Contexto

No padrão outbox, um caso de uso grava, na **mesma transação** do estado de
negócio, uma linha que representa a mensagem a publicar. O commit torna o estado
e a intenção de publicar visíveis de forma atômica, mas não publica nada: quem lê
essas linhas e as entrega ao broker é um processo próprio — o relay, no bloco
`app`, com composition root e ciclo de vida separados do processo que atende
requisições.

Isso deixa duas perguntas em aberto, e é delas que este ADR trata. A primeira:
**como o relay descobre os registros elegíveis?** Consultando a tabela
periodicamente (polling) ou reagindo à escrita por captura de mudanças no banco
(CDC)? A segunda: **como o relay reivindica um registro para publicar** sem que
dois workers publiquem o mesmo, e sem que um worker morto o prenda para sempre?

As forças em jogo são concretas. Polling é simples e não exige nada além da
própria tabela; CDC reage ao commit com menor latência, mas depende de recurso
exclusivo do banco. Na reivindicação, segurar um lock de banco
(`SELECT FOR UPDATE`) do claim até o fim da publicação é a via mais direta, mas
prende uma conexão do pool pela latência do broker — e a separação do relay em
processo próprio existe justamente para que essa latência não alcance o caminho
de request.

Sobre tudo isso pesa uma restrição inegociável: a semântica de entrega é
**at-least-once**, sem exactly-once fim a fim (constraint P0-3). Nenhuma escolha
de mecanismo de relay pode alterá-la — e CDC, em particular, não entrega
exactly-once, de modo que adotá-lo não é ganho semântico.

## Decisão

**Polling com leasing é o mecanismo de relay padrão desta fundação.**

O relay reivindica um lote de registros elegíveis por **lease com prazo**. A
transação de claim marca, no mesmo commit, `status` como `publishing`, grava
`locked_by` com a identidade do claim e `locked_until` com o prazo do lease, e
incrementa `attempt_count`. A publicação ocorre **fora de qualquer transação de
banco**: nenhum lock é mantido durante o I/O com o broker (FND-04 §5.1,
`OBX-07`). Um registro é elegível quando está `pending`, ou quando está
`publishing` e o `locked_until` já venceu — e, nos dois casos, quando
`available_at` já passou (`OBX-09`). Um lease expirado devolve o registro ao pool
pela **comparação de prazos**, nunca por uma reescrita que o rebaixe de estado.

**CDC é extensão, não substituição** (FND-04 §5.5, `OBX-15`), admitida para alto
volume ou baixa latência. Um contexto que adote CDC continua obrigado a tudo o
que a fundação estabelece para a outbox: a linha continua escrita na mesma
transação do estado, e as garantias de entrega não mudam. O que muda é apenas
**como** o registro é descoberto.

**Nenhum dos dois mecanismos altera a semântica de entrega.** At-least-once
permanece a garantia oficial e exactly-once fim a fim permanece vedado (P0-3); um
texto de projeto que atribua exactly-once a CDC viola essa constraint.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Adotar CDC como mecanismo padrão, com polling como legado | CDC exige recurso exclusivo do banco, sem ganho semântico — não entrega exactly-once fim a fim (P0-3). Para o volume observado no universo inventariado, a complexidade não se justifica como default; por isso CDC fica como extensão (FND-04 §5.5, `OBX-15`). |
| Manter lock pessimista (`SELECT FOR UPDATE`) do claim até o fim da publicação, em vez do lease cooperativo | Segurar o lock durante o I/O com o broker prende uma conexão do pool pela latência da publicação. Sob carga, o pool esgota antes de o broker saturar, e a falha aparece no caminho de request — exatamente o lugar onde a separação de processos existe para que ela não apareça (FND-04 §5.1 `rationale`). |

O acervo registra, para este acionamento, quatro alternativas descartadas,
distribuídas por §2.1, §5.1, §5.2 e §5.5 (FND-04 §10.2). As duas acima são as que
caem **dentro** das seções de origem deste ADR (§5.1 e §5.5). As outras duas —
relay embutido no processo de request (§2.1) e fencing token em campo próprio do
schema (§5.2) — pertencem a seções adjacentes: a primeira é consequência da
separação de blocos, a segunda é propriedade interna do relay que FND-04 §10.3
deliberadamente **não** eleva a ADR. Registre-se ainda que §5.5 não traz bloco
`rationale`: o motivo para descartar «CDC como padrão» sai de sua prosa
`normativo` (`OBX-15` e a declaração de padrão), que é onde o acervo registra
essa alternativa; o único bloco `rationale` do par está em §5.1.

## Consequências

**Positivas:**

- Simplicidade e portabilidade: polling com leasing não exige broker especial nem
  recurso exclusivo do banco, roda sobre qualquer Postgres e é suficiente para o
  volume do universo inventariado.
- O lease cooperativo não segura conexão do pool durante a publicação; sob carga,
  o pool não esgota por latência do broker, e a falha não vaza para o caminho de
  request.
- CDC como extensão preserva um caminho de evolução para alto volume ou baixa
  latência sem reescrever a semântica: a outbox continua escrita na mesma
  transação e as garantias de entrega continuam valendo.
- A escolha é neutra quanto à entrega — nem polling nem CDC mexem em P0-3 —, então
  migrar um contexto para CDC não obriga a revisar contratos de consumidor.

**Negativas:**

- **Custo aceito:** polling impõe um piso de latência igual ao intervalo de
  varredura e uma carga de leitura contínua sobre a própria tabela outbox, mesmo
  quando não há nada a drenar — o preço de não reagir ao commit como o CDC faria.
- Contextos de alto volume ou baixa latência precisam construir e operar o CDC por
  conta própria, como extensão, e essa complexidade extra não compra ganho
  semântico algum (CDC não entrega exactly-once).
- O lease com prazo limita inferiormente a recuperação de um worker morto: um
  registro reivindicado por um worker que caiu só volta ao pool quando
  `locked_until` vence, não instantaneamente — o custo de não manter o lock
  pessimista.

## Referências

- **ADR-021** (acionamento `ADR-DMPF-L`, companheiro em FND-04 §10.2) — decide que
  o mapeamento `domain event → integration event` reside no `provider` da outbox e
  que a serialização ocorre na escrita. Importa aqui porque o relay publica o
  `payload` já serializado sem interpretá-lo (FND-04 §5.4, `OBX-14`): esses bytes
  são congelados na escrita por decisão de ADR-021, e é isso que torna o relay
  agnóstico ao conteúdo do que publica.
- **Artefato de origem**: FND-04 — `docs/dmpf/uow-inbox-outbox.md`, §5.1 (claim por
  lease) e §5.5 (polling e CDC), com apoio de §5.4 (sequência de drenagem) e §7.1
  (semântica at-least-once, P0-3).
- **SPEC-DBTRMM3X** — spec que esta série de ADRs implementa.
- **ARQ-448** (DMPF-FND-11) — story que redige, promove e aceita os ADRs acionados
  pelo acervo DMPF.
