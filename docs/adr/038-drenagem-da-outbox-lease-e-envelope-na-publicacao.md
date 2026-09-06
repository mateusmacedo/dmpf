# ADR-038: Drenar a outbox com lease liberado por `NULL`, montar o envelope na publicação e tratar `payload_hash` divergente como terminal

## Status

Aceito — 2026-09-06. Implementa SPEC-CGPX20NP.

## Contexto

O ADR-035 realizou a escrita da outbox sobre PostgreSQL: o registro é gravado na
mesma transação do estado de negócio, com os bytes do fato congelados na escrita
(`ENV-18`). Ninguém o drenava. O ADR-020 já decidira o mecanismo — polling com
leasing, com CDC como extensão —, e o FND-04 §5.4 fixa os três passos: claim em
transação curta, publicação fora de qualquer transação (`OBX-07`) e transição
final condicional ao claim ainda ser o corrente (`OBX-10`).

Ao implementar esses três passos, quatro decisões apareceram sem resposta na
fundação, e nenhuma delas é de tuning.

**A primeira é o que "liberar o lease" significa.** `OBX-18` manda liberar
`locked_until` no mesmo commit que recalcula `available_at` no desfecho
transitório, mas não diz com qual valor.

**A segunda é de onde vêm os atributos do envelope.** `ENV-14` fixa a
correspondência coluna → atributo, mas três atributos obrigatórios de `ENV-08` —
`correlationid`, `causationid` e `traceparent` — não têm coluna própria. O
FND-05 encaminha a forma de persistir ao FND-04, que devolve o conteúdo ao
FND-07: a referência é circular e a forma nunca foi decidida. Na prática o
provider grava `metadata = '{}'`
(`libs/backend/go/dmpf-provider-postgres/outbox.go:66-73`), porque inventar o
conteúdo seria o provider autorando o que não lhe pertence (`OBX-02`).

**A terceira é o que fazer com corrupção em repouso.** O ADR-035 (`:163-165`)
obriga conferir `payload_hash` na drenagem, mas não classifica a divergência
entre os três desfechos do passo 3.

**A quarta é o `aggregateversion`.** A coluna `aggregate_version` é `bigint`; o
atributo do envelope é `ce_integer`, que é `int32`. A conversão entre os dois
não é total.

Some-se que o bloco `contract` tinha `envelope.Unmarshal` mas não o simétrico:
não havia como um publicador entregar bytes a um transporte sem importar o tipo
gerado do CloudEvent, exatamente o que o godoc de `Unmarshal` existe para
evitar.

## Decisão

**O lease liberado é `NULL`, e o predicado de elegibilidade o aceita
explicitamente.** `Reschedule` grava `locked_until = NULL` junto do
`available_at` recalculado. Como consequência, o predicado de `OBX-09` precisa
da cláusula `locked_until IS NULL`: em SQL, `locked_until <= $1` avalia como
desconhecido para um lease ausente, e sem a cláusula o registro **nunca**
voltaria ao pool — o backoff seria inerte e a mensagem ficaria presa em
`publishing` para sempre. A cláusula está em
`libs/backend/go/dmpf-provider-postgres/claim.go` e tem teste próprio.

**O relay lê os três atributos de contexto de `metadata`, e um registro sem os
três não é reivindicado.** O predicado do claim exige as três chaves. Um
registro incompleto permanece `pending`, com `attempt_count` intocado: não vira
`failed`, não consome tentativa, não fica terminal. O sinal `pending` cresce e
comunica o sintoma correto, e no dia em que alguém gravar `metadata` o backlog
drena sozinho, sem intervenção.

**`payload_hash` divergente é terminal, sem consumir backoff.** Retentar não
conserta bytes corrompidos em repouso; tratar a divergência como falha
transitória gastaria o teto de tentativas e atrasaria a descoberta do problema.
O registro vai direto a `failed`, o publisher não é chamado, e `last_error`
descreve a divergência sem reproduzir os bytes.

**`aggregate_version` fora da faixa de `int32` é terminal e sanitizado.**
Converter silenciosamente publicaria uma versão negativa como se fosse a
verdadeira, e o consumidor não teria como saber.

**`envelope.Marshal` vive no bloco `contract`.** É o simétrico de `Unmarshal`:
valida o perfil por `Encode` e serializa. A alternativa — `Encode` mais
`proto.Marshal` no relay — faria o bloco `app` importar o tipo gerado do
CloudEvent, quebrando a simetria com o caminho de entrada do `KRN-07`.

**As transições do passo 3 rodam destacadas do cancelamento do chamador.** Uma
publicação que o broker já aceitou precisa ser registrada mesmo durante o
encerramento; perder essa escrita republicaria a mensagem à toa. O contexto de
escrita é `context.WithoutCancel` com deadline próprio — o mesmo motivo pelo
qual `uow.go:49` faz o rollback assim. A exceção é o publisher **cancelado pelo
encerramento**: ele não falhou por mérito próprio, então não é cobrado backoff;
o claim é devolvido ao pool imediatamente pelo encerramento gracioso
(`OBX-13`).

## Consequências

O backlog cresce enquanto ninguém gravar `metadata`, e **ninguém grava hoje**.
O `KRN-09` (ADR-037) era o candidato natural e não o fez: aquele ADR declara que
o `KRN-06` e o `KRN-07` "mergearam em paralelo a esta entrega e ainda não são
instrumentados por ela". Portanto a lacuna de `ENV-08` segue aberta depois do
`KRN-09`, e nenhuma entrega tem a produção dos três atributos no escopo.

Isso é deliberado e é o comportamento correto: a alternativa — publicar um
envelope inválido, ou queimar os registros em `failed` — perderia mensagens ou
exigiria intervenção manual para recuperá-las. O custo é que o sinal `pending`
fica alto e precisa ser lido com esse contexto até que a produção dos três
atributos seja atribuída a alguém.

A cláusula `locked_until IS NULL` é uma condição a mais no predicado mais quente
do relay. O índice parcial `dmpf_outbox_claim_idx` cobre a varredura
(`available_at, id` sob `status IN ('pending','publishing')`), e a aplicabilidade
é provada em teste com `enable_seqscan = off` — não pela escolha do planner em
tabela pequena, que nada diria.

Um registro em `failed` não retorna ao ciclo automático. Retomá-lo é operação
com evidência, fora desta entrega e do `KRN-08`.

O relay declara `Store` e `Publisher` no consumidor e é satisfeito
estruturalmente pelo `OutboxStore` do provider. Isso acopla o pacote `relay` ao
tipo `dmpfpostgres.Claimed`. A aresta `app → provider` é permitida pela matriz
de blocos (ADR-010), mas um segundo provider — outro banco — exigiria ou o mesmo
tipo ou um adaptador na composition root. A troca foi aceita para não inaugurar
uma camada de conversão antes de existir o segundo provider que a justifique.

`cmd/dmpf-relay` **não** foi entregue. Um binário sem `Publisher` concreto não
teria o que compor, e o transporte é do `KRN-10`. `BLK-02` fica satisfeito no
desenho: o relay tem composition root e lifecycle próprios, separados do
processo que atende requisições, e o exemplo está em
`libs/backend/go/dmpf-app/example/reservations/relay.go`.

## Alternativas descartadas

**Gravar `locked_until` igual ao novo `available_at` no desfecho transitório.**
Evitaria a cláusula extra no predicado, mas passaria a afirmar que existe um
lease até aquele instante, o que é falso, e reintroduziria a confusão entre
prazo de lease e prazo de backoff que `OBX-18` existe justamente para desfazer.

**Reivindicar o registro sem os três atributos e deixá-lo falhar na montagem.**
O registro viraria `failed` por uma lacuna da norma, não por defeito próprio, e
sairia do ciclo automático — precisaria de intervenção manual depois que a
lacuna fosse fechada.

**Tratar `payload_hash` divergente como falha transitória, por simetria com os
demais erros.** A simetria seria só aparente: os outros erros têm chance de
sucesso numa nova tentativa, e este não tem.

**Acrescentar `Publisher` a `dmpf-ports`.** Contradiria uma decisão já
registrada em código (`dmpf-ports/outbox.go:36-38`: aquele bloco não declara
porta de publicação porque publicar acontece depois do commit e fora da unidade
de trabalho) e ampliaria a superfície fechada daquele bloco sem necessidade.

**Paginar o claim por cursor.** Um cursor aberto manteria transação viva durante
o I/O, violando `OBX-07`. Cada ciclo abre e fecha a sua própria transação de
claim.

## Referências

- `docs/specs/SPEC-CGPX20NP-dmpf-relay-outbox.md` — a spec desta entrega.
- `docs/dmpf/uow-inbox-outbox.md` — FND-04 §4.2, §5.1 a §5.5 (`OBX-03` a `OBX-18`).
- `docs/dmpf/cloudevents-protobuf-buf.md` — FND-05 §4.1 (`ENV-08`, `ENV-11`, `ENV-14`, `ENV-17` a `ENV-19`).
- `docs/adr/010-regra-de-dependencia-e-seis-blocos.md` — a matriz que autoriza `app → provider`.
- `docs/adr/020-relay-polling-leasing-e-cdc-extensao.md` — o mecanismo de drenagem.
- `docs/adr/021-mapeamento-no-provider-serializacao-na-escrita.md` — os bytes congelam na escrita.
- `docs/adr/035-realizacao-postgres-da-outbox.md` — o schema e a conferência de `payload_hash`.
- `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md` — o consumo que absorve a republicação.
- `docs/adr/037-observabilidade-otel-e-retry-por-conjuncao-em-go.md` — o `KRN-09`, que mergeou em paralelo a esta entrega e deixou a lacuna de `ENV-08` aberta.
