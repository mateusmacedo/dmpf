# ADR-061: Promover ao kernel as cópias de produção dos contextos

## Status

Aceito — 2026-10-10. Implementa SPEC-R8645FVR. Supersede em parte o [ADR-048](./048-layout-canonico-de-bounded-context.md) e o [ADR-053](./053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md), nos pontos listados na Decisão.

## Contexto

O ADR-048 promoveu ao kernel o que era igual por natureza entre os contextos e deixou no contexto o que "diverge por papel": `requirements()`, `RunWith`, `serveAPI`, `classify`, `subject` e os construtores de serviço tipado. Depois das Ondas 0 a 2 da SPEC-R8645FVR, parte dessa lista não diverge mais:

- `classify` e `subject` são idênticos em `orders`, `reservations` e `bookings`. O ADR-048 tentou levá-los para `observability/usecase`, e o verificador reprovou com `DMPF-D001`: aquela unidade é bloco `provider`, e provider não depende de `application`.
- `serveAPI` e `runRelay` têm o mesmo corpo nos três contextos. O que muda entre eles é parâmetro: o construtor do serviço, o tópico, o nome do componente e, no `reservations`, o catálogo de canais. A leitura de "diverge por papel" tinha sido aplicada ao corpo da função, não ao que de fato varia.
- `Instant` é declarado em cada `domain` de contexto e em `ports`, sempre como `int64` em nanossegundos (ADR-053 §8).
- A classificação de erro do gRPC existe duas vezes: interna no kernel `grpc` e exportada no `bff`, com a mesma tabela e duas diferenças nos erros de deadline.
- O handler gRPC de comando que a Onda 2 deixou local em cada `app/rpc` é idêntico byte a byte nos três contextos, com o `executionOf` e o `rejectionOf` que ele usa.

Essas cópias formam grupos de clone na medição `dupl` que a spec usa como meta (no máximo 34 grupos ao fim da Onda 3, metade da linha de base de 67).

## Decisão

As cópias de produção vão para o kernel, cada uma no bloco que a matriz de dependências permite:

- **`CategoryOf` e `SubjectOf` (P3).** `application.CategoryOf(err) string` devolve a categoria de um erro classificado, ou vazio quando não classifica; `ports.SubjectOf(ctx) string` devolve o subject do contexto de execução. O `obsusecase` já trata categoria vazia como `_OTHER`. A reversão do ADR-048 não se repete, porque o destino é o bloco `application` e o `ports`, que o bloco `app` do contexto pode importar.
- **`app/serve` (P6).** Package novo `libs/backend/go/app/serve`, com `API`, `ServeAPI` e `RunRelay`. O contexto entrega o registro dos seus serviços como função, o que cobre também o contexto gerado que ainda não tem serviço. `RunWith` continua no contexto, porque a escolha de papéis é dele. A unidade nova é `kernel/app-serve` (bloco `app`, `bounded_context` `kernel`, `public_integration_surface: false`), designada em `shared_kernel_units` como `kernel/app-relay`. O `go.mod` do módulo `app` passa a requerer `transport` e `kafka`.
- **`Instant` no `domain` (A6).** `type Instant int64` vai para o kernel `domain`, e `ports.Instant` vira alias dele. Saem as declarações dos três contextos e as conversões `domain.Instant(identity.OccurredAt)` dos casos de uso.
- **`kernelgrpc.Category` (B7).** A classificação interna do kernel `grpc` passa a ser exportada como `Category(err) string` e ganha os dois erros de deadline do `bff`: `deadline.ErrNoDeadline` vira `Unexpected` e `deadline.ErrDeadlineExhausted` vira `DeadlineExceeded`. O `bff` usa a função do kernel. Exceção declarada: a telemetria do kernel muda para esses dois erros, que deixam de sair como `_OTHER`.
- **Handler de comando no kernel (N10).** O kernel `grpc` ganha `Outcome[R]` (interface com `Response() R` e `Rejection() (*domain.Rejection, bool)`), `Command` e `ExecutionOf`. O `grpc` é bloco `provider` e não importa `application`; o `application.Outcome[R]` satisfaz a interface pela forma, sem dependência nominal. A função de recusa recebe o `*domain.Rejection` do kernel, e cada contexto mapeia para o tipo de rejeição do próprio contrato gerado, que o kernel não pode nomear. O `ExecutionOf` substitui o `executionOf` local; a precedência que motivou a decisão P2 da Onda 2 se mantém, porque o comando valida o identificador antes de ler o contexto de execução.
- **Generator.** Os templates do `@mateusmacedo/dmpf-plugin` passam a usar as funções promovidas. Pelo lockstep do ADR-060, uma versão do plugin só funciona com a versão do kernel que contém essas funções.

Pontos supersedidos:

- **ADR-048, §Decisão:** da lista do que "diverge por papel" saem `serveAPI`, `classify` e `subject`; ficam no contexto `requirements()`, `RunWith` e os construtores de serviço tipado. O parágrafo da reversão deixa de valer para o destino: `classify` e `subject` vão para `application` e `ports`, não para `observability/usecase`.
- **ADR-053, §7 e §14:** o contexto não expõe mais um `errors.go` próprio, e o generator deixa de emiti-lo. A tradução de erro para status gRPC é do kernel (`kernelgrpc.StatusOf`) desde a Onda 1 da SPEC-R8645FVR.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Manter as cópias no contexto, como fixou o ADR-048 | O corpo é igual nos três contextos; a cópia só se justifica quando o comportamento diverge |
| `classify` e `subject` em `observability/usecase` | Já foi tentado e revertido: bloco `provider` dependendo de `application` reprova com `DMPF-D001` |
| `Command` no `grpc` importando `application.Outcome` | Mesma violação de `DMPF-D001`: o `grpc` é bloco `provider` |
| `Command` em package novo de bloco `app` | Exigiria unidade e regravação de baseline sem ganho; a interface estrutural resolve dentro do `grpc` |
| `serve.API` com o serviço como `any` ao lado do descritor | Descritor e serviço podem divergir sem que o compilador acuse |
| `Instant` em `ports`, com alias no `domain` | O `domain` é o bloco mais interno e não importa `ports` |
| Classificação gRPC exportada só pelo `bff` | Kernel e borda classificariam o mesmo erro de formas diferentes |

## Consequências

**Positivas:**

- Saem dos três contextos `classify`, `subject`, `serveAPI`, `runRelay`, `keyChannelName`, `Instant`, `command`, `executionOf` e `rejectionOf`; o contexto gerado fica menor pelo mesmo motivo.
- Uma só classificação de erro gRPC entre kernel e `bff`.
- Os grupos de clone de produção dessas funções deixam de existir.

**Negativas:**

- Spans, logs e métricas dos dois erros de deadline mudam de `error.type`; painel ou alerta que dependa de `_OTHER` para eles precisa acompanhar.
- O módulo `app` ganha `transport` e `kafka` como requisitos diretos.
- As funções de recusa dos handlers passam a receber o tipo do kernel e a mapear para o contrato do contexto.
- **Custo aceito:** a superfície pública do kernel cresce (`app/serve`, `Command`, `ExecutionOf`, `Category`, `CategoryOf`, `SubjectOf`), e o template do plugin passa a depender dela, então qualquer mudança nessas funções exige release do kernel em lockstep com o plugin.

## Referências

- SPEC-R8645FVR (DEVS-67, Onda 3).
- [ADR-042](./042-shared-kernel.md), [ADR-046](./046-libs-somente-kernel-de-reuso.md), [ADR-048](./048-layout-canonico-de-bounded-context.md), [ADR-053](./053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md) e [ADR-060](./060-o-dmpf-tem-consumidor-externo.md).
- `docs/guides/dmpf-manifesto.md` — schema do `dmpf-units.json` e regravação do baseline.
