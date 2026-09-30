# http

Bloco `provider` do kernel DMPF para a borda externa em REST/JSON (FND-06 §9,
ADR-024), só com a biblioteca padrão: timeout do cliente derivado do deadline
de quem chama, retry apenas em método com semântica idempotente, resolução da
identidade do sujeito contra a permissão que a rota declara (FND-07), e
admissão por rota e tenant antes de ler o corpo.

Criado por `KRN-10` (ARQ-529, `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md`);
a resolução de identidade e permissão é de `docs/specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md`
(ADR-052).

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/provider-http` | `provider` | raiz do módulo |

Sem dependência externa declarada (`external: []`): nenhum framework HTTP —
`net/http` basta — e os tipos que a configuração e a admissão expõem
(`ports.Permission`, `ports.SubjectID`, `ports.TenantID`) vêm do módulo `ports`
do próprio kernel, que não é dependência externa.

## O que o módulo contém

- **`route.go`** — `Route{Name, Method, Path, ContractRef, Budget,
  RetryableStatus, IdempotencyKey, Requires, PlatformReach, Permission}`.
  `Validate` recusa rota sem referência ao contrato publicado
  (`ErrContractRequired`, RST-04), método fora de
  GET/HEAD/PUT/DELETE/POST/PATCH, orçamento sem folga e a combinação inválida
  de `Requires`/`PlatformReach` (`ErrPlatformReachRequired`, IDN-19: uma rota
  de plataforma declara o alcance de dado que tem, uma rota escopada não
  declara nenhum). `ValidateEdge` é `Validate` mais a exigência de permissão
  em toda rota que exige sujeito (`ErrPermissionRequired`, IDN-16/17).
  `Idempotent()` é o que RST-02 admite retentar: GET, HEAD, PUT e DELETE; POST
  só quando a rota declara a chave de idempotência que o provider envia;
  PATCH nunca.
- **`identity.go`** — `Resolved{Subject, Tenant, Permissions}` e
  `ResolveIdentity(ctx, authenticator, route, credential)`: decide a
  requisição contra o que a credencial resolveu, numa ordem que nunca confunde
  "não autenticado" com "sem a permissão da operação" (IDN-06) — sem
  credencial numa rota que não exige nada, segue; sem credencial numa rota que
  exige, 401; credencial rejeitada, 401; sujeito exigido e ausente, 401;
  tenant exigido e ausente, 403 `tenant-unresolved`; sujeito exigido sem
  `Route.Permission` declarada, 403 `permission-undeclared`; permissão
  declarada e não concedida, 403 `permission-denied`.
  `RefuseAssertedIdentity(r, resolved)` recusa com 403 `identity-mismatch`
  quando o próprio request afirma, em header (`X-Subject-ID`, `X-Tenant-ID`)
  ou em query (`tenant_id`), um sujeito ou tenant diferente do que a
  verificação resolveu — presença já é a afirmação, mesmo com valor vazio
  (CTX-06). `WithExecutionContext`/`ExecutionContextFrom` delegam a `ports`
  em vez de chavear valor próprio: um segundo carrier faria o provedor ler de
  onde a borda nunca escreveu (CTX-05, ADR-049).
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

## A exigência de identidade de uma rota (`Requirement`, IDN-16/17)

`Requirement` é o que a operação declara precisar resolvido no contexto de
execução; o valor zero (`RequireSubjectAndTenant`) exige **os dois**, então
omitir a declaração fecha a rota em vez de abri-la (IDN-17):

| `Requirement` | Exige sujeito | Exige tenant |
| --- | --- | --- |
| `RequireSubjectAndTenant` (zero value) | sim | sim |
| `RequireSubject` | sim | não |
| `RequireTenant` | não | sim |
| `RequireNeither` | não | não |

Uma rota `RequireNeither` é uma **rota de plataforma**: `PlatformReach` declara
o alcance de dado que ela tem — hoje só `ReachAllTenants` — e é a única
combinação que dispensa `Requires`/`PlatformReach` de se corresponderem 1:1;
toda rota escopada (as outras três) deixa `PlatformReach` em `ReachUndeclared`.
O alcance de todo tenant nunca é o efeito colateral de um contexto sem tenant —
é uma decisão registrada por rota (IDN-19).

## O que o módulo não contém

Desenho de recurso, paginação, forma do corpo de erro e versionamento de URL
(TRP-03: contrato e adapter de entrada, fora do kernel); o documento OpenAPI —
a rota só o referencia (RST-04); servidor HTTP próprio — a admissão é um
middleware para o `http.Handler` do composition root; a verificação da
credencial em si — `ports.Authenticator` é injetado, e `libs/backend/go/authn`
é quem o realiza contra um authorization server OIDC.

## Como rodar os testes localmente

Todos unitários: `httptest` para o upstream, `RoundTripper` falso para a
falha de rede, `ManualReader` para as séries. Os testes de retry usam o
relógio do sistema com backoff de 1 ms; o de timeout observa ≈ 750 ms
(800 ms de prazo − 50 ms de folga) contra um upstream de 3 s. Os testes de
`identity.go` (`identity_test.go`) usam um `ports.Authenticator` fake e cobrem
as sete combinações de `ResolveIdentity` mais os três campos que
`RefuseAssertedIdentity` protege.

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
- `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — `IDN-06`, `IDN-16`,
  `IDN-17`, `IDN-19`, `CTX-01` a `CTX-06`.
- `docs/adr/024-rest-externo-grpc-interno-governo-do-tempo.md` — REST na borda externa.
- `docs/adr/049-contexto-de-execucao-viaja-no-context-context.md` — o carrier que `WithExecutionContext`/`ExecutionContextFrom` delegam a `ports`.
- `docs/adr/052-identidade-de-workload-no-grpc-e-no-kafka.md` — por que a permissão é conferida aqui, na borda, e não nos contextos internos.
- `libs/backend/go/transport/README.md` — `deadline`, `admission`, `observe`.
- `libs/backend/go/authn/README.md` — quem realiza `ports.Authenticator`.
