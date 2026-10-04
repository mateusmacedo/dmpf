# grpc

Bloco `provider` do kernel DMPF para o transporte síncrono interno em gRPC
(FND-06 §10, ADR-024): deadline em toda chamada de saída, propagado como
duração restante e nunca reiniciado; retry derivado da idempotência declarada
do método; TLS obrigatório fora de desenvolvimento; identidade do workload
verificada por mTLS em cada salto (ADR-052); health por serviço; admissão por
método e tenant na borda do servidor.

Criado por `KRN-10` (ARQ-529, `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md`);
a verificação de workload por mTLS é de `docs/specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md`
(ADR-052).

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/provider-grpc` | `provider` | raiz do módulo |

Dependências externas declaradas: `google.golang.org/grpc` `>=1.83.1` (`io.network`,
incluindo `credentials` e `health`), `go.opentelemetry.io/otel/trace` e
`go.opentelemetry.io/otel/metric` (`observability`). O SDK OTel e o `bufconn`
só aparecem em `_test.go`, que o verificador não classifica.

## O que o módulo contém

- **`config.go`** — `Config` do cliente (TLS ou `InsecureForDevelopmentOnly`,
  `Sheet` de RES-21, `Methods` por nome completo, `HealthServiceName`, `Clock`,
  `Tracer`, os providers e o propagador que o `otelgrpc` usa no `Dial` —
  `TracerProvider`, `MeterProvider`, `Propagator` —, `Instruments`,
  `LoggerProvider`, `Rand`) e `MethodPolicy{Budget, Idempotent,
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
- **`observe.go`** — liga o módulo a `transport/compose`, cujas posições de
  observabilidade (`transport/observe`) realizam o que RES-23 exige e o KRN-09
  não entrega: o span INTERNAL `dmpf.resilience {dmpf.dependency}` sobre as
  tentativas, com a degradação da sheet sob ele, e o log de cada tentativa só
  com a categoria; a posição de métricas só repassa a chamada, porque o RED do
  cliente é o `rpc.client.call.duration` do `otelgrpc`. Aqui só se declara a
  categoria de falha e o que o breaker conta como falha.
- **`classifier.go`** — `StatusClassifier(codes)`: só um status com código
  declarado é retentável (GRP-09). A categoria de falha (`error.type` e
  `dmpf.outcome_category`) é a do erro de plataforma; senão, a categoria FND-07
  do prazo (`DeadlineExceeded`), do cancelamento (`Cancelled`) ou do código
  gRPC — `InvalidArgument` → `Validation`, `FailedPrecondition` →
  `DomainRejection`, `NotFound` → `NotFound`, `Aborted` e `AlreadyExists` →
  `Conflict`, `PermissionDenied` → `Forbidden`, `Unauthenticated` →
  `Unauthenticated`, `Unavailable` → `TransientDependency`,
  `ResourceExhausted` → `RateLimited`, `Internal` → `Unexpected` —, e `_OTHER`
  fora da tabela; nunca a mensagem (TRC-12).
- **`dial.go`** — `ServiceConfig` (`round_robin` + `healthCheckConfig`, com o
  import em branco de `grpc/health`) e `Dial`: credenciais, service config,
  `WithDisableRetry` — o retry nativo não vê idempotência (GRP-08) —, o
  `otelgrpc.NewClientHandler`, que abre um CLIENT `{rpc.method}` por tentativa
  sob o span de resiliência e grava `rpc.client.call.duration`, com health e
  reflection fora pelo filtro (RF-B4), e as cadeias de interceptors.
- **`server.go`** — `ServerConfig{TLS, InsecureForDevelopmentOnly, Services,
  UnaryInterceptors, StreamInterceptors, LoggerProvider, TracerProvider,
  MeterProvider, Propagator}` e `NewServer`:
  `Validate` recusa servidor sem transporte seguro e sem o opt-out (GRP-15), e
  um servidor TLS cujo `ClientAuth` não é `RequireAndVerifyClientCert`
  (`ErrClientCARequired`, IDN-03) — um `ServerConfig` montado fora de
  `APIServerConfig` não escapa dessa exigência. O `otelgrpc.NewServerHandler`
  abre um SERVER `{rpc.method}` por chamada e grava `rpc.server.call.duration`,
  com health e reflection fora pelo filtro (RF-B4); provider ou propagador
  ausente é o global. Cada serviço declarado começa em `NOT_SERVING` (GRP-13).
  À frente das duas cadeias, unária e de stream, o `NewServer` põe a
  recuperação de panic, porque o `grpc-go` não recupera o handler e o runtime
  encerraria o processo com a pilha no stderr: a chamada responde `INTERNAL`
  com a projeção pública `internal failure`, o SERVER fica com erro e
  `error.type` e `dmpf.outcome_category` `Unexpected` (ERR-22), e o registro é
  o `grpc call` da chamada, emitido no `stats.End`, em `error`, com o contexto
  de execução que a cadeia unária já montou (o stream não monta nenhum) e sem
  o valor nem a pilha do panic; o processo segue servindo (ERR-23, RF-A1).
- **`server_config.go`** — a fachada de composição do servidor de API:
  `APIServer{CertFile, KeyFile, Insecure, ClientCAFile, TrustedClients,
  Services, Interceptors, LoggerProvider}` e `APIServerConfig`, que monta o
  `ServerConfig` já com mTLS ligado — sem o composition root ter que
  encadear `requireClientAuth` e os interceptors de confiança à mão.
  `ServerTLS(certFile, keyFile)` carrega o par declarado ou devolve `nil`
  para o opt-out explícito de `NewServer`. `HealthServices(name)` é
  `["", name]`: o nome vazio é o status geral que uma sonda sem serviço
  pergunta.
- **`admission.go`** — `Admission(ctrl, tenant, instruments)`: interceptor do
  servidor sobre `transport/admission`. Recusa `RESOURCE_EXHAUSTED` antes
  do handler (RES-17) e conta `dmpf_admission_rejections_total{rpc.method,
  dmpf.tenant_id}` — a série `dmpf.admission.rejections` — com o tenant
  colapsado pela allowlist (MET-07, MET-12); rota sem limite declarado
  responde `UNIMPLEMENTED` (RES-16). O `rpc.method` da série é o
  `FullMethod` sem a barra inicial
  (`company.orders.service.v1.OrdersService/PlaceOrder`), como o `otelgrpc`
  o grava.
- **`interceptor_context.go`** — `ServerInterceptors(service, ctrl,
  instruments, logs, opts...)`: a cadeia do servidor sob o SERVER do
  `otelgrpc`, só para os métodos do serviço — desfecho, registro de acesso,
  admissão, prazo e contexto. O desfecho grava `dmpf.outcome_category` no SERVER
  e devolve só a projeção pública do status (ERR-20), porque o `otelgrpc` copia
  a mensagem do status para o span. O registro de acesso é um `grpc call` por
  chamada (RF-A5), com `rpc.system.name`, `rpc.method`,
  `rpc.response.status_code`, `dmpf.outcome_category` e, na falha,
  `error.type`, no nível da tabela única de `logging.Severity`. O registro sai
  só no `stats.End`, pelo `stats.Handler` que o `NewServer` registra depois do
  `otelgrpc`, com o status com que o `grpc-go` fechou a chamada (a resposta
  acima de `grpc.MaxSendMsgSize` sai `RESOURCE_EXHAUSTED`). A cadeia só anota
  o contexto de execução, a chave e o erro do handler; a chamada recusada
  antes dela (corpo que não decodifica, mensagem acima de
  `grpc.MaxRecvMsgSize`, peer recusado pelo `TrustedPeers`) sai com o trace do
  SERVER e sem o contexto de execução. Um servidor sem esse `stats.Handler`
  recebe o registro da própria cadeia. O contexto de
  execução montado do que cruzou o salto vai ao SERVER (`dmpf.correlation_id`,
  `dmpf.request_id`, `dmpf.tenant_id`) e ao baggage (RF-B8), e o contexto de
  mensagem leva o `traceparent` e o `tracestate` do span ativo à outbox; um
  `tracestate` acima de 512 bytes, o piso que a W3C Trace Context (§3.3.1.5)
  pede para propagar, é descartado inteiro, nunca cortado no meio de um membro.
- **`interceptor_context.go`, comandos** — `ServerInterceptors(..., opts...)`
  aceita `WithCommands(métodos...)`, que declara os métodos de comando do
  serviço. Neles, a metadata `idempotency-key` é obrigatória e segue
  `ports.IdempotencyKeyPattern`: ausente ou fora do formato, a chamada é
  recusada com `InvalidArgument` antes do handler (`IDM-01`, `IDM-02`). A chave
  segue ao caso de uso pelo portador do `ports`, e o replay bem-sucedido
  responde com o header `idempotent-replayed: true` (`IDM-08`). O `grpc call`
  leva a chave em `dmpf.idempotency_key` ou, fora do formato,
  `dmpf.idempotency_key.invalid` sem o valor; o header de replay não entra em
  span nem em log.
- **`idempotency_status.go`** — `IdempotencyStatus(err)` traduz os desfechos
  do comando: divergência em `FailedPrecondition`, em andamento em `Aborted`,
  chave ausente em `InvalidArgument` e `ports.ErrAlreadyExists` em
  `AlreadyExists`. Cada status leva um `ErrorInfo` de domínio `dmpf`
  (`REUSED_IDEMPOTENCY_KEY`, `IN_FLIGHT_IDEMPOTENCY_KEY`,
  `MISSING_IDEMPOTENCY_KEY`, `INVALID_IDEMPOTENCY_KEY`), que o BFF lê por
  `ReasonOf` para escolher o código REST.
- **`status.go`** — `HTTPStatus(codes.Code)`: a tabela canônica de GRP-14
  (`Canceled → 499`), o resto conforme o grpc-gateway, fora da tabela → 500.
- **`serve.go`** — `Serve(ctx, listen, server, healthServer, services, ready,
  logs)`: roda `ready` **antes** de aceitar a primeira conexão — uma sonda
  que não fala o protocolo de saúde (o kubelet caindo para sonda TCP num
  listener TLS) só passa num processo que respondeu pelas próprias
  dependências —, e registra `grpc listening` com `server.address` e
  `server.port`. `Drain` para de aceitar, espera as chamadas em curso e força
  a saída ao fim de `observability.ShutdownGrace` (10 s), a mesma janela que
  o `boot` da telemetria usa, com um `warn` que leva `dmpf.shutdown.grace`.

## Identidade do workload por mTLS (`IDN-03`, ADR-052)

Com TLS ativo, `APIServerConfig` exige que o cliente apresente certificado e o
verifica contra uma CA declarada (`ClientCAFile`) — a partida recusa TLS sem
essa CA (`ErrClientCARequired`) e sem ao menos um workload confiável declarado
(`ErrTrustedClientsRequired`, `TrustedClients`). O interceptor `TrustedPeers`
(unário) e `TrustedStreamPeers` (stream) confere a identidade do certificado
verificado **antes** de qualquer outro interceptor — inclusive admissão e o
contexto de execução — contra a URI SAN ou o DNS SAN da folha, **nunca contra
o CN**, que não tem semântica de nome e que a própria CA pode emitir livre. Só
depois desse interceptor a metadata `x-tenant-id` chega a ser lida.

A identidade do BFF, o único cliente destes servidores hoje, é
`spiffe://dmpf/bff`. `orders` e `reservations` a leem em `GRPC_TRUSTED_CLIENTS`,
e a CA de clientes em `GRPC_CLIENT_CA_FILE` — as duas variáveis são lidas
pelo composition root de cada contexto (`apps/backend/{orders,reservations}/app/config.go`),
não por este módulo, que só declara os campos de `APIServer`. No compose local
a CA é de desenvolvimento, emitida pelo `pki-init` (`infra/local`); o opt-out
continua existindo como `GRPC_INSECURE`, registrado no log quando usado.

A sonda gRPC do kubelet não apresenta certificado de cliente: com mTLS ativo os
processos `api` trocam para sonda de socket TCP, e é por isso que `Serve`
resolve `ready` antes de aceitar qualquer conexão — a sonda de socket só
enxerga a porta aberta, nunca a saúde declarada.

## O que o módulo não contém

Connect e transcodificação; a superfície REST (recurso, paginação,
versionamento), que ADR-024 deixa fora do kernel; qualquer serviço próprio —
o health é o único protocolo que ele embarca, e é também a RPC dos testes,
para não haver código gerado; o rate limit de saída por dependência (RES-15,
decorator do KRN-09 ainda sem realização); a resolução da identidade do
sujeito — a autenticação e a autorização por permissão vivem na borda REST
(`libs/backend/go/http`, `libs/backend/go/authn`); o tenant chega por
`TenantFunc` injetada pelo composition root, e a leitura de
`GRPC_CLIENT_CA_FILE`/`GRPC_TRUSTED_CLIENTS` é do composition root de
cada contexto, não deste módulo.

## Como rodar os testes localmente

Todos os testes são unitários e sem rede: `bufconn` para o transporte,
`clock.Fake` para o tempo, `tracetest` e `ManualReader` para os sinais. O
teste de dois saltos usa o relógio do sistema porque atravessa transportes
reais, e tolera 20 ms de trânsito na folga observada por hop.

Os testes de mTLS (`mtls_test.go`, `server_config_test.go`) têm a própria
autoridade de certificados descartável, definida em `pki_test.go` — sem
depender do `testkit` (um `provider` não alcança o bloco `app`, que é onde
vive o equivalente `tb.PKI`).

```bash
pnpm nx run grpc:test-race
```

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test-race,govulncheck -p grpc
go run ./tools/dmpf-conformance/cmd/conformance --root .
```

O `dmpf-gate-check.sh` não alcança este módulo (o `depguard` seleciona por
`-domain/`, `-ports/`, `-application/`, `-contracts/`); o gate autoritativo é o
verificador.

## Referências

- `docs/dmpf/politicas-transporte.md` (FND-06) — §10 (`GRP-04` a `GRP-18`).
- `docs/dmpf/resiliencia-observabilidade.md` (FND-08) — `RES-16`, `RES-17`,
  `RES-21` a `RES-23`, `RES-31`, `MET-08` a `MET-12`, `TRC-04`, `TRC-12`.
- `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — `IDN-03`, `IDN-04`.
- `docs/adr/024-rest-externo-grpc-interno-governo-do-tempo.md` — o transporte síncrono interno.
- `docs/adr/052-identidade-de-workload-no-grpc-e-no-kafka.md` — a verificação por mTLS e a allowlist de workloads.
- `docs/specs/SPEC-1TFW24WV-observabilidade-ponta-a-ponta.md` — os `RF-*` citados aqui: `otelgrpc`, o `grpc call` e o baggage de execução.
- `libs/backend/go/transport/README.md` — `deadline`, `admission` e `observe`.
- `libs/backend/go/observability/README.md` — a composição do KRN-09.
- `libs/backend/go/authn/README.md` — a autenticação do sujeito na borda REST.
