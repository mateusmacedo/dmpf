# grpc

Bloco `provider` do kernel DMPF para o transporte síncrono interno em gRPC
(FND-06 §10, ADR-024): deadline em toda chamada de saída, propagado como
duração restante e nunca reiniciado; retry derivado da idempotência declarada
do método; TLS obrigatório fora de desenvolvimento; health por serviço;
admissão por método e tenant na borda do servidor.

Criado por `KRN-10` (ARQ-529, `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md`).

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/provider-grpc` | `provider` | raiz do módulo |

Dependências externas declaradas: `google.golang.org/grpc` (`io.network`),
`go.opentelemetry.io/otel/trace` e `go.opentelemetry.io/otel/metric`
(`observability`). O SDK OTel e o `bufconn` só aparecem em `_test.go`, que o
verificador não classifica.

## O que o módulo contém

- **`config.go`** — `Config` do cliente (TLS ou `InsecureForDevelopmentOnly`,
  `Sheet` de RES-21, `Methods` por nome completo, `Service`, `Clock`,
  `Tracer`, `Instruments`, `Logger`) e `MethodPolicy{Budget, Idempotent,
  RetryableCodes}`. `Validate` recusa cliente sem TLS e sem o opt-out
  (`ErrTLSRequired`, GRP-15), TLS que não verifica o par ou admite versão
  abaixo de 1.2 (`ErrTLSTooWeak`; o mesmo gate vale para `ServerConfig`),
  sheet com campo em branco e orçamento sem folga;
  `Policy(method)` devolve `ErrMethodNotDeclared` — não há prazo nem retry por
  default (GRP-04, GRP-16).
- **`interceptor_deadline.go`** — interceptors unário e de stream do cliente;
  no stream, o contexto derivado é cancelado no primeiro erro terminal de
  `RecvMsg`, `SendMsg`, `Header` ou `CloseSend` (o half-close bem-sucedido não
  cancela) e quando o contexto do próprio stream termina (conexão fechada
  incluída), para não vazar o timer até o fim do prazo; um stream abandonado
  sem drenar nem cancelar o contexto pai vive até o prazo, como no grpc-go.
  Cada chamada de saída recebe `deadline.Outgoing(ctx, clock, budget)`: o menor
  entre o restante do chamador e o `Limit` do método, menos a folga (GRP-05,
  GRP-17). Contexto sem deadline, prazo exausto ou método não declarado são
  recusados **antes** do fio (CTX-21). O fio carrega a duração restante e o
  receptor reconstrói o instante (GRP-06); `two_hops_test.go` prova
  `deadline_C < deadline_B < deadline_A` em A → B → C, que `Limit` maior não
  estende a rota e que o cancelamento em A chega a C.
- **`interceptor_compose.go`** — a composição de RES-22 via
  `transport/compose` (`Shared` para as posições por dependência, `Retry`
  por método), **uma `Call` por método declarado**, construída em `Dial`: o
  classificador de retry depende dos `RetryableCodes` do método. `Breaker` e
  `Bulkhead` são um por dependência e compartilhados; o `Breaker` conta como
  falha só indisponibilidade (`UNAVAILABLE`, `DEADLINE_EXCEEDED`,
  `RESOURCE_EXHAUSTED`, `INTERNAL`, `UNKNOWN`, `DATA_LOSS`) e erro de
  transporte — `NOT_FOUND`, `ABORTED` e os demais desfechos são resposta da
  dependência e não o abrem (RES-10, RES-12); `Timeout` reserva
  `Backoff.Base`; `Retry` só repete método `Idempotent` com código
  declarado transiente **e** orçamento instalado no contexto por
  `retry.WithBudget` (RES-31) — sem orçamento, uma tentativa. `Retry`
  declarado `false` na sheet recebe decorator identidade; `RateLimit`
  declarado é recusado, porque não há decorator que o realize (a admissão
  por rota e tenant vive no servidor).
- **`observe.go`** — liga o módulo a `transport/observe`, que realiza os
  três decorators que RES-23 exige e o KRN-09 não entrega (span de cliente
  com os atributos fechados de TRC-04, séries MET-08 a MET-10, log de falha
  só com categoria); aqui só se declara o prefixo do span e a categoria de
  falha — o código gRPC em minúsculas, nunca a mensagem (TRC-12).
- **`classifier.go`** — `StatusClassifier(codes)`: só um status com código
  declarado é retentável (GRP-09).
- **`dial.go`** — `ServiceConfig` (`round_robin` + `healthCheckConfig`, com o
  import em branco de `grpc/health`) e `Dial`: credenciais, service config,
  `WithDisableRetry` — o retry nativo não vê idempotência (GRP-08) — e as
  cadeias de interceptors.
- **`server.go`** — `ServerConfig` e `NewServer`: `grpc.Creds`, cadeias de
  interceptors e o serviço de health com cada serviço declarado começando em
  `NOT_SERVING` (GRP-13).
- **`admission.go`** — `Admission(ctrl, tenant, instruments)`: interceptor do
  servidor sobre `transport/admission`. Recusa `RESOURCE_EXHAUSTED` antes
  do handler (RES-17) e conta `dmpf_service_admission_rejections_total{route,
  tenant}` com o tenant colapsado pela allowlist (MET-07, MET-12); rota sem
  limite declarado responde `UNIMPLEMENTED` (RES-16).
- **`status.go`** — `HTTPStatus(codes.Code)`: a tabela canônica de GRP-14
  (`Canceled → 499`), o resto conforme o grpc-gateway, fora da tabela → 500.

## O que o módulo não contém

Connect e transcodificação; a superfície REST (recurso, paginação,
versionamento), que ADR-024 deixa fora do kernel; qualquer serviço próprio —
o health é o único protocolo que ele embarca, e é também a RPC dos testes,
para não haver código gerado; o rate limit de saída por dependência (RES-15,
decorator do KRN-09 ainda sem realização); a identidade de FND-07 — o tenant
chega por `TenantFunc` injetada pelo composition root.

## Como rodar os testes localmente

Todos os testes são unitários e sem rede: `bufconn` para o transporte,
`clock.Fake` para o tempo, `tracetest` e `ManualReader` para os sinais. O
teste de dois saltos usa o relógio do sistema porque atravessa transportes
reais, e tolera 20 ms de trânsito na folga observada por hop.

```bash
pnpm nx run grpc:test-race
```

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test-race,govulncheck -p grpc
go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop
```

O `dmpf-gate-check.sh` não alcança este módulo (o `depguard` seleciona por
`-domain/`, `-ports/`, `-application/`, `-contracts/`); o gate autoritativo é o
verificador.

## Referências

- `docs/dmpf/politicas-transporte.md` (FND-06) — §10 (`GRP-04` a `GRP-18`).
- `docs/dmpf/resiliencia-observabilidade.md` (FND-08) — `RES-16`, `RES-17`,
  `RES-21` a `RES-23`, `RES-31`, `MET-08` a `MET-12`, `TRC-04`, `TRC-12`.
- `docs/adr/024-rest-externo-grpc-interno-governo-do-tempo.md` — o transporte síncrono interno.
- `libs/backend/go/transport/README.md` — `deadline`, `admission` e `observe`.
- `libs/backend/go/observability/README.md` — a composição do KRN-09.
