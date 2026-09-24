# http

Bloco `provider` do kernel DMPF para a borda externa em REST/JSON (FND-06 §9,
ADR-024), só com a biblioteca padrão: timeout do cliente derivado do deadline
de quem chama, retry apenas em método com semântica idempotente, e admissão
por rota e tenant antes de ler o corpo.

Criado por `KRN-10` (ARQ-529, `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md`).

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/provider-http` | `provider` | raiz do módulo |

Dependências externas declaradas: `go.opentelemetry.io/otel/trace` e
`go.opentelemetry.io/otel/metric` (`observability`) — os tipos que a
configuração e a admissão expõem. Nenhum framework HTTP: `net/http` basta.

## O que o módulo contém

- **`route.go`** — `Route{Name, Method, Path, ContractRef, Budget,
  RetryableStatus, IdempotencyKey}`. `Validate` recusa rota sem referência ao
  contrato publicado (`ErrContractRequired`, RST-04), método fora de
  GET/HEAD/PUT/DELETE/POST/PATCH e orçamento sem folga. `Idempotent()` é o que
  RST-02 admite retentar: GET, HEAD, PUT e DELETE; POST só quando a rota
  declara a chave de idempotência que o provider envia; PATCH nunca.
- **`client.go`** — `Config`, `NewClient` e `Client.Do(ctx, rota, req)`. O
  prazo de cada chamada é `deadline.Outgoing(ctx, clock, budget)` convertido
  em `context.WithDeadline` (RST-03): o HTTP não carrega o deadline no
  protocolo, e por isso o provider o converte em timeout. A composição de
  RES-22 é **uma `Call` por cliente**: o status transiente vira
  `*RetryableStatusError` dentro da tentativa e a idempotência viaja na
  `Operation`. A resposta transiente é bufferizada (1 MiB) e drenada para
  reuso da conexão; esgotado o retry, a **última** resposta volta ao chamador
  com o status original e sem erro. Corpo reenviado só por `req.GetBody`
  (`ErrBodyNotReplayable` em rota retentável sem ele); a chave de
  idempotência é estável por chamada — a do chamador, ou 128 bits aleatórios.
  Cada tentativa corre num contexto próprio, cancelado quando ela termina; o
  contexto da chamada só é liberado no `Close` do `Body` da resposta final, que
  por isso continua legível depois de `Do` retornar. A composição de RES-22 vem
  de `transport/compose`.
- **`classifier.go`** — `Classifier`: status transiente declarado e
  `net.Error` são retentáveis; cancelamento, deadline e o resto não. A
  categoria de falha (span, séries, log) é `status_<n>`, `network`,
  `deadline_exceeded`, `cancelled` ou a categoria do erro de plataforma —
  nunca a mensagem.
- **`admission.go`** — `Admission(ctrl, route, tenant, instruments)`:
  middleware `func(http.Handler) http.Handler` sobre
  `transport/admission`. Recusa **429** com `Retry-After` antes do
  handler e antes de ler o corpo (RES-17), conta
  `dmpf_service_admission_rejections_total{route, tenant}` com o tenant
  colapsado pela allowlist (MET-07, MET-12); rota sem limite declarado
  responde 404 (RES-16). `RouteFunc` mapeia a requisição para a chave
  declarada (default `METHOD path`); a recusa de rota **não declarada** é
  contada sob a label fixa `UndeclaredRouteLabel` (`undeclared`), de modo que
  um path inventado por cliente nunca abre série nova (MET-07).

## O que o módulo não contém

Desenho de recurso, paginação, forma do corpo de erro e versionamento de URL
(TRP-03: contrato e adapter de entrada, fora do kernel); o documento OpenAPI —
a rota só o referencia (RST-04); servidor HTTP próprio — a admissão é um
middleware para o `http.Handler` do composition root; a identidade de FND-07 —
o tenant chega por `TenantFunc` injetada.

## Como rodar os testes localmente

Todos unitários: `httptest` para o upstream, `RoundTripper` falso para a
falha de rede, `ManualReader` para as séries. Os testes de retry usam o
relógio do sistema com backoff de 1 ms; o de timeout observa ≈ 750 ms
(800 ms de prazo − 50 ms de folga) contra um upstream de 3 s.

```bash
pnpm nx run http:test-race
```

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test-race,govulncheck -p http
go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop
```

O `dmpf-gate-check.sh` não alcança este módulo; o gate autoritativo é o
verificador.

## Referências

- `docs/dmpf/politicas-transporte.md` (FND-06) — §9 (`RST-01` a `RST-04`).
- `docs/dmpf/resiliencia-observabilidade.md` (FND-08) — `RES-16`, `RES-17`,
  `RES-22`, `RES-31`, `MET-12`.
- `docs/adr/024-rest-externo-grpc-interno-governo-do-tempo.md` — REST na borda externa.
- `libs/backend/go/transport/README.md` — `deadline`, `admission`, `observe`.
