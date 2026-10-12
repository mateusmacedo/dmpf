# ADR-062: Promover ao testkit os kits de processo, telemetria e contexto de execução

## Status

Aceito — 2026-10-10. Implementa SPEC-R8645FVR. Supersede em parte o [ADR-046](./046-libs-somente-kernel-de-reuso.md), nos pontos listados na Decisão.

## Contexto

O ADR-046 deixou `appkit` e `distkit` como harness do `reservations` porque não havia um segundo consumidor de um harness independente de plano: "abstração sem cliente". Também deixou a fixture de caso de uso das libs em `_test.go`, para não criar package de produção sem unidade (`DMPF-U001`). Hoje o quadro é outro:

- O ADR-048 tornou `appkit` e `distkit` obrigatórios em todo contexto. O `distkit` de `orders` e o de `bookings` são idênticos, salvo o nome; o do `reservations` repete o supervisor de processo e a criação de tópicos, e o `bff` tem um supervisor próprio no harness de e2e.
- Os testes de telemetria de cada contexto (manifesto, baggage de tenant, campos do registro de processo, allowlist por papel, trilha de auditoria do wiring) são cópias, trocado o nome do contexto, e o `bff` repete parte deles.
- Os testes de degradação de `bff`, `grpc`, `kafka` e `sqs` repetem os coletores OTLP de falha, e os leitores de telemetria de teste (rótulos de métrica, exportador de log em memória, espera de span, runtime com leitor de métrica, conexões do pool) estão espalhados por contextos e libs.
- O literal de `ExecutionContextSpec` é montado em 25 testes dos apps e 4 do kernel, com um contador de chaves de idempotência repetido.
- O caso de uso de referência `counterService` está copiado em `observability/usecase` e `testkit/serviceskit`.

Essas cópias formam a maior parte dos grupos de clone de teste da medição `dupl` da SPEC-R8645FVR.

## Decisão

Os kits vão para o `testkit`, em packages irmãos de `tb`:

- **`testkit/tb/dist` (T1).** Um supervisor genérico de processo filho (`Supervise`, com espera, encerramento por SIGTERM e prazo, e um `Output()` que devolve um snapshot da saída sob mutex, sem bloquear) e um harness por papel que reexecuta o binário de teste, com o vetor do relay (`DMPF-P001` a `DMPF-P003`) e a criação de tópicos. O `distkit` de cada contexto fica fino: o `Spec`, os papéis e, no `reservations`, o plano e o `DMPF-R004`. O `reservations` passa a provar também o vetor do próprio relay. O supervisor do harness de e2e do `bff` passa a ser o `Supervise` (T11b). Unidade nova: `kernel/testkit-dist` (bloco `app`, `public_integration_surface: true`).
- **`testkit/obstest` (T3, N8, N9).** O manifesto de telemetria parametrizado pela `Config` e pelo papel do contexto, o coletor OTLP de teste, a verificação do registro de processo e da allowlist por papel; os coletores de falha, de rede (porta fechada e conexão que não responde, para `bff` e `grpc`) e em processo (exportadores que sempre falham, para `kafka` e `sqs`); e os leitores de telemetria de teste. Unidade nova: `kernel/testkit-obs` (bloco `app`, `public_integration_surface: true`).
- **`testkit/tb/execctx` (T6).** A fixture de `ExecutionContext`, com opções para slot, chave automática por processo e as variantes que os testes usam (só subject, só tenant, tenant, requisição e deadline). Entra no `include` de `kernel/testkit-tb`.
- **`testkit/tb/reference` (N6).** O `counterService` de referência, superconjunto das duas cópias, num arquivo de produção. Entra no `include` de `kernel/testkit-tb`, sem unidade nova.
- **Irmãos de `tb`.** `dist`, `obstest`, `execctx` e `reference` não são importados por `tb` nem por `domainkit`: o guard dos testes de domínio (V29/V30) reprova unidade `domain` cujo fechamento de teste alcance `port`.
- **Dependências.** O `go.mod` do `testkit` passa a requerer diretamente o franz-go (`kgo` e `kadm`), `google.golang.org/grpc`, `go.opentelemetry.io/proto/otlp` e o SDK OTel (`sdk`, `sdk/metric`, `sdk/log`), com as entradas `external` no manifesto. O `dist` usa o franz-go direto e nunca o módulo `kafka` do kernel, que já requer o `testkit`. Os módulos `observability`, `grpc` e `transport` passam a requerer o `testkit` só em package `_test` externo; `kafka`, `sqs` e o `bff` já o requeriam.
- **Baseline.** As duas unidades novas e o `include` ampliado mudam o baseline de unidades; a regravação fica com o mantenedor, como manda o ADR-058. A medição de digest de evidência por subject não se aplica: o ADR-059 removeu a evidência.

Pontos supersedidos:

- **ADR-046, §Contexto e §Alternativas descartadas:** a extração de um harness genérico de `distkit` deixa de ser "abstração sem cliente"; há três contextos e o `bff` consumindo o supervisor. O `appkit` continua no contexto.
- **ADR-046, §Contexto:** o caso de uso de referência das libs sai do `_test.go` para `testkit/tb/reference`; o package de produção entra no `include` de uma unidade existente, o que satisfaz a `DMPF-U001`. O agregado `counter` do `domainkit` continua como está.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Manter o `distkit` por contexto, como fixou o ADR-046 | Três cópias do supervisor, duas delas idênticas, e o `reservations` sem a prova do próprio relay |
| Fixture de `ExecutionContext` dentro de `tb` | Os testes de `domain` importam `tb`, e a fixture importa `ports`: o guard V29/V30 reprova |
| `counterService` no `serviceskit` | O `serviceskit` é bloco `provider`; o caso de uso importa `application`, o que viola a `DMPF-D001` |
| Unidade nova para `execctx` e `reference` | Mesma regravação de baseline e nenhum consumidor de produção que justifique a separação |
| Coletor de rede também para `kafka` e `sqs` | As asserções desses testes dependem de exportadores em processo que falham de forma conhecida |
| `dist` importando o módulo `kafka` do kernel | Ciclo de módulo: o `kafka` requer o `testkit` |

## Consequências

**Positivas:**

- Os testes de telemetria, degradação, contexto de execução e harness distribuído viram chamadas a um kit, com uma semântica só para `Output()`, para a fixture e para os coletores.
- O `reservations` prova `DMPF-P001` a `DMPF-P003`, e o contexto gerado herda os kits sem cópia.
- A meta de redução de linhas de teste da SPEC-R8645FVR fica ao alcance só com o `obstest`.

**Negativas:**

- O `testkit` fica mais pesado: quem o importa passa a resolver franz-go, gRPC, OTLP e o SDK OTel.
- Os testes que viram subtestes de um kit mudam de nome no relatório do `go test`.
- `observability`, `grpc` e `transport` ganham o `testkit` como requisito de teste, e o ciclo de módulo `observability → memory → testkit → observability`, que já existia por transitividade, passa a ser direto.
- **Custo aceito:** os kits viram superfície pública do kernel para o consumidor externo (ADR-060), e mudança na API deles exige release em lockstep com o plugin.

## Referências

- SPEC-R8645FVR (DEVS-67, Onda 3).
- [ADR-040](./040-test-kits-golden-e-fitness-function-em-go.md), [ADR-046](./046-libs-somente-kernel-de-reuso.md), [ADR-048](./048-layout-canonico-de-bounded-context.md), [ADR-058](./058-projeto-solo-sem-controles-de-segunda-pessoa.md), [ADR-059](./059-remover-o-bom-e-a-release-do-produto.md) e [ADR-060](./060-o-dmpf-tem-consumidor-externo.md).
- `libs/backend/go/testkit/README.md` — roteamento dos packages do kit.
