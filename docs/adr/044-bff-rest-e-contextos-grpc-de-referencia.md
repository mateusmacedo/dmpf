# ADR-044: Separar a referência em um BFF REST público e dois contextos com gRPC interno

## Status

Aceito — 2026-09-15. Implementa SPEC-ACYKBF9V. Evolui a topologia do composition root do ADR-041 sem alterar as decisões transversais dele: papéis por flag, configuração só por ambiente, um runtime OTel por processo, release do produto por tag anotada e BOM.

## Contexto

O ADR-041 entregou `apps/backend/reference` como um binário único: o papel `api` servia REST direto do contexto `orders`, e o `consumer` de `reservations` rodava no mesmo módulo. O FND-06 fixa REST/JSON só para o consumidor externo (`RST-01`, ADR-024) e gRPC para a chamada síncrona interna (`GRP-01`), e a referência não exercitava nenhum dos dois como a norma separa. O contexto `reservations` só existia como consumidor assíncrono, sem comando síncrono, leitura nem cancelamento.

Restrições herdadas:

- O bloco `contract` só admite as capabilities `pure` e `wire.codec`; o `google.golang.org/grpc` é declarado `io.network` (ADR-015).
- A drenagem reivindica a outbox inteira, sem filtro por destino (ADR-038).
- Contextos distintos só se tocam pela superfície pública ou pelo shared kernel (ADR-017, ADR-042).
- `otelboot` inicia uma vez por processo (ADR-037).

## Decisão

**Três composition roots, seis processos.** `bff` é a única borda REST/JSON pública. `orders` roda os papéis `api` (servidor gRPC) e `relay`. `reservations` roda `api`, `relay` e `consumer`. `orders` não tem papel `consumer`, porque não consome canal. O `reference` é removido.

**O BFF vive em contexto próprio.** A unidade `bff/app` declara `bounded_context` `bff`. Ela alcança só o contrato, que é superfície pública por construção, e o shared kernel: providers HTTP e gRPC, transporte, observabilidade e portas. O verificador prova, por `DMPF-D002`, que ela não importa domínio nem aplicação dos exemplos. Os contextos permanecem em `kernel`.

**Mensagens no contrato, binding no app.** Os `.proto` de `OrdersService` e `ReservationsService` geram só mensagens e descriptor pelo `protoc-gen-go` já pinado. Cada app monta o próprio `grpc.ServiceDesc` (servidor) ou chama `ClientConn.Invoke` (cliente), com os nomes de serviço e método tirados do descriptor gerado. Um teste por app prova que todo método do descriptor está coberto. A rejeição de domínio viaja no `oneof result` da resposta; a falha técnica é status gRPC, mapeado por tabela.

**Banco por contexto.** Cada contexto recebe DSN próprio, e o `relay` de cada um drena só a outbox do próprio banco. A porta de outbox do kernel não muda.

**Prazo e contexto de mensagem nascem na borda.** No BFF, a cadeia é admissão → span de servidor e contexto → prazo da requisição derivado do `Budget` da rota → `Idempotency-Key` → handler. A metadata leva `x-correlation-id`, `x-causation-id` (o `request_id` do BFF) e o `traceparent` do span de cliente. No contexto, a cadeia é span de servidor → admissão → deadline obrigatório → `MessageContext` com a causação no `request_id` do próprio contexto → caso de uso. Só `FindOrder` e `FindReservation` são retentáveis (`GRP-08`, `GRP-09`), e o prazo de cada método é menor que o da rota (`GRP-17`).

**A saúde fica fora da cadeia do contexto.** Admissão, deadline obrigatório e `MessageContext` valem só para os métodos do serviço do contexto. A checagem de saúde passa direto: ela não é caso de uso, e a probe gRPC do Kubernetes precisa dela sem limite de admissão declarado. O serviço começa `NOT_SERVING`, inclusive o status geral, e só vira `SERVING` depois do `Ping` e do `Migrate` opcional.

**O breaker conta só indisponibilidade.** O primeiro e2e mostrou que o breaker do kernel contava qualquer erro como falha da dependência: uma rajada de `NOT_FOUND` em `FindReservation` o abria e recusava todas as leituras de `reservations` por 30 s, contra `RES-10` e `RES-12`. A correção fica no kernel, não no BFF. `resilience.Breaker` ganha `CountsAsFailure`, que `transport/compose` repassa por `Config.BreakerFailure`, e o `grpc` passa a contar só `UNAVAILABLE`, `DEADLINE_EXCEEDED`, `RESOURCE_EXHAUSTED`, `INTERNAL`, `UNKNOWN`, `DATA_LOSS` e erro de transporte. Sem classificador, os demais providers mantêm o comportamento anterior.

**A primeira decisão vence em `reservations`.** `Canceled` é o segundo estado terminal, ao lado de `Confirmed`. `Reserve` e `Cancel` síncronos carregam ou criam a reserva e percorrem os nove passos de FND-04 §3.2. O consumo de `OrderPlaced` sobre reserva cancelada é rejeitado por domínio, e o `Ack` vem depois do commit.

**Prova caixa-preta.** O e2e fica no módulo do BFF: compila os três binários, sobe os seis processos sobre dois bancos e fala só HTTP com o BFF.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| REST entre o BFF e os contextos | `GRP-01` fixa gRPC para a chamada síncrona interna quando as duas pontas são nossas |
| Transcodificação gRPC-JSON nos contextos | `GRP-02` a reserva a outro perfil e `GRP-03` a exclui do caminho que alimenta inbox; o BFF é o tradutor |
| BFF importando domínio ou aplicação dos exemplos | Cruza o bounded context (`DMPF-D002`) e acopla a borda ao modelo interno |
| `protoc-gen-go-grpc` gerando o binding em `contracts` | O código gerado importa `grpc`, de capability `io.network`, que o bloco `contract` não admite |
| Um único banco com filtro por destino na drenagem | Muda a porta de outbox do kernel por causa de um exemplo |
| Lib compartilhada de interceptors entre os apps | Tira composição do composition root (ADR-015) |
| E2e em goroutines no mesmo processo | O runtime OTel é único por processo e a topologia de seis processos ficaria sem prova |

## Consequências

**Positivas:**

- A topologia-alvo de FND-06 — REST na borda, gRPC interno, Kafka assíncrono — passa a existir executável e provada.
- O workspace ganha o primeiro contexto exposto por gRPC, com binding sem código gerado fora do contrato.
- A cadeia `correlationid`, `causationid` e `traceparent` fica provada do REST do BFF até o `ReservationConfirmed`.

**Negativas:**

- **Custo aceito:** os interceptors de servidor são duplicados nos dois contextos; os testes espelhados são a defesa contra divergência.
- **Custo aceito:** o binding manual precisa acompanhar cada método novo do `.proto`; o teste de cobertura do descriptor reprova o esquecimento.
- **Custo aceito:** seis processos pesam mais no orçamento local de recursos, e o `test-race` do BFF compila três binários e exige Postgres e Redpanda.

## Referências

- SPEC-ACYKBF9V — requisitos, fluxos e critérios de aceite desta decisão.
- ADR-015, ADR-017, ADR-024, ADR-037, ADR-038, ADR-041 e ADR-042.
