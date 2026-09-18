---
id: SPEC-Z2HM6NAP
slug: endurecimento-borda-kernel
title: Endurecimento da borda e do kernel — amostragem, catálogo multi-evento e orçamento operacional
stage: backlog
priority: P3
depends_on: [SPEC-ACYKBF9V]
ticket_url: null
subtask_urls: []
created: 2026-09-15
---

# Endurecimento da borda e do kernel

## Resumo

Cinco achados do code review da SPEC-ACYKBF9V que **não** cabem como correção
local: cada um exige mudança em contrato público do kernel ou decisão de
arquitetura que afeta toda a topologia. Os outros 22 achados do mesmo review
foram corrigidos na própria entrega.

O denominador comum é que a correção ingênua de cada item quebra um
comportamento hoje testado e desejado. Nenhum deles bloqueia a topologia de
referência: a cadeia funcional está provada pelo e2e de seis processos.

## Contexto

O code review da SPEC-ACYKBF9V correu em quatro frentes (varredura mecânica,
revisor do BFF, revisor dos contextos e Codex) e produziu 1 P0, 13 P1 e 15 P2.
Após deduplicação sobraram 27 itens, dos quais 22 foram corrigidos com TDD na
entrega e 5 chegaram aqui.

Um sexto achado — o `causationid` da outbox não adotar o `x-causation-id` do
BFF — foi **refutado** durante a verificação: `CTX-08`
(`docs/dmpf/contexto-erros-seguranca.md:618`) fixa a causação como o passo
imediatamente anterior, que para um registro de outbox é a chamada gRPC do
próprio contexto, não a requisição da borda. O comportamento atual está certo
e `rpc/interceptors_test.go:60` já o trava.

<constraints>
- Toda mudança em `transport/channel` ou `observability/otelboot`
  afeta os quatro providers de transporte e os três apps de referência.
- Nenhum item pode afrouxar o gate: o verificador, o gate de dependência e os
  autotestes de célula continuam valendo.
- `contracts/` publicado é imutável; evolução de contrato tem rito próprio.
</constraints>

## Requisitos

### Funcionais

1. **[P1] Amostragem não decidida pelo cliente.** A borda pública aceita o
   `traceparent` para correlação, mas um cliente anônimo não pode forçar
   amostragem de 100% da topologia. Hoje `classSampler.ShouldSample`
   (`otelboot/sampler.go:72-78`) segue `parent.IsSampled()` sem reavaliar, e o
   flag propaga da borda até os contextos pelo `contextInterceptor`.

2. **[P1] Canal declara todos os eventos que carrega.** `channel.Channel`
   (`transport/channel/channel.go:131`) tem `EventType string` — um único
   tipo. O canal `orders.events` transporta `order-placed` **e**
   `item-added.v1` (`postgres/example/orders/mapper.go:14,32`),
   então a declaração ASY-01 está incompleta para quem for assinar o canal.

3. **[P1] Consumer drena na janela de graça.** O primeiro sinal cancela o mesmo
   contexto entregue a `consumer.Run`, em vez de parar os polls e deixar a
   mensagem em processamento terminar. A inbox evita perda, mas cada deploy
   gera rollback e reentrega evitáveis.

4. **[P2] Classe de tráfego declarada.** O span de servidor do BFF não recebe
   `tracing.KeyTrafficClass`, então as rotas de escrita amostram na taxa
   restritiva (1%) em vez de 10%; e `telemetry.go:29` fixa `ClassWrite` para
   todo log do processo, inclusive os dois GETs.

5. **[P2] Orçamento de resiliência por ambiente.** `Wait` (o `lock_timeout` da
   inbox), `processingDeadline`, `rebalanceTimeout`, `queuePerPartition`, o
   `relay.Config` e o `admission.Limit` são constantes de código. São
   exatamente os valores que se ajustam sob carga, e hoje exigem recompilação.

### Não-funcionais

- Cada mudança de contrato do kernel precisa manter os consumidores atuais
  compilando, ou migrar todos na mesma entrega.
- O item 1 não pode desligar a gravação do span da borda (ver "Armadilha
  verificada" abaixo).

## Armadilha verificada

A correção ingênua do item 1 — desligar o flag `sampled` do parent na borda —
**foi implementada e revertida** durante a SPEC-ACYKBF9V. O teste
`TestAnIncomingTraceparentIsContinued` (`api/handlers_test.go:309`) reprovou com
"no edge span recorded": sem o flag, o span raiz cai na taxa restritiva e deixa
de ser gravado. A proteção contra o abuso custou a observabilidade da borda.

A decisão pertence ao sampler, que é kernel compartilhado — por isso esta spec.

## Escopo fora

- Os 22 achados corrigidos na SPEC-ACYKBF9V.
- O `causationid`, refutado por `CTX-08`.
- Aplicar os manifestos Kubernetes num cluster e exercitar o TLS gRPC do
  overlay `hmg`, que seguem sem verificação de execução (pendência registrada
  no gate Nível C da SPEC-ACYKBF9V).

## Verificação e testes

### Critérios de aceite

- [ ] Um cliente que envie `traceparent` sempre amostrado não altera a taxa
      efetiva da topologia, e o span da borda continua sendo gravado —
      `TestAnIncomingTraceparentIsContinued` segue verde sem alteração.
- [ ] O catálogo de `orders.events` declara os dois tipos que o canal carrega,
      e o gate de conformidade segue verde nos quatro providers de transporte.
- [ ] SIGTERM no consumer termina a mensagem em voo dentro da janela, com o
      offset commitado, e só cancela o contexto ao expirar.
- [ ] Rota de escrita amostra na taxa de escrita; log de leitura não é
      declarado como escrita.
- [ ] Os valores de resiliência entram por variável de ambiente, validados na
      partida como os demais.

<critical_constraints>
- [P0] NUNCA corrigir a amostragem desligando o flag do parent na borda: já foi
  tentado e o teste provou que apaga o span da borda.
- [P0] NUNCA adotar o `x-causation-id` do BFF como causação da outbox: viola
  `CTX-08`.
- [P1] Mudança em `channel.Channel` migra os quatro providers na mesma entrega.
</critical_constraints>
