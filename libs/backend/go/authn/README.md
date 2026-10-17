# authn

Bloco `provider` do kernel DMPF que realiza `ports.Authenticator` contra um
authorization server OIDC (FND-07 §4, `IDN-01`): descoberta do issuer na
partida, verificação de assinatura, issuer, audiência e expiração, e a leitura
de sujeito, tenant e permissões das claims que o operador declarar. Opcionalmente,
consulta uma deny-list de sessões revogadas no IdP e a alimenta pelos eventos de
sessão do Keycloak (`docs/specs/SPEC-STEJG347-revogacao-de-sessao-authn.md`).

Alcançar a autoridade é `io.network`, e é por isso que a realização vive num
`provider` e não num bounded context: `CTX-02` já colocava a autorização com
porta e realização default no kernel, e a autenticação, sendo transversal do
mesmo jeito, tinha só a porta até este módulo promovê-la (`feat(authn): promover
autenticação para o kernel`). Um segundo BFF não precisa copiar código de
segurança.

Realiza a porta `ports.Authenticator` declarada em
`docs/specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md`; a
verificação de identidade do **workload** entre serviços — mTLS no gRPC, SASL
no Kafka — é uma decisão separada, em `docs/adr/052-identidade-de-workload-no-grpc-e-no-kafka.md`,
que este módulo não cobre: aqui é o sujeito humano ou de API na borda, lá é o
processo do outro lado do salto interno.

Projeto Nx `authn`, tags `type:lib`, `scope:backend`, `stack:go`,
`layer:providers`. Import path do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/authn`.

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/provider-authn` | `provider` | raiz do módulo |

Dependências externas declaradas: `github.com/coreos/go-oidc/v3` `>=3.21.0 <4`
(`io.network`) — descoberta do issuer, JWKS, rotação e a verificação do logout
token são da biblioteca; o que fica aqui é a leitura das claims que o operador
nomeou (IDN-01); `golang.org/x/oauth2` `>=0.36.0 <1` (`io.network`) — o client
credentials do polling de eventos; e `go.opentelemetry.io/otel/metric`
`>=1.46.0 <2` (`observability`) — as métricas da revogação. `public_integration_surface: false`:
nenhum outro contexto importa este módulo além do composition root que o
monta.

## O que o módulo contém

- **`config.go`** — `Config{Issuer, Audience, TenantClaim, PermissionClaims,
  DiscoveryTimeout, DevMock, LoggerProvider, Revocations, RevocationTimeout,
  MeterProvider}`. `Defaults()` assume um access
  token de Keycloak: `scope` como string separada por espaço e roles de realm
  aninhadas em `realm_access.roles`. `ReadEnv(lookup)` lê `OIDC_ISSUER`,
  `OIDC_AUDIENCE`, `OIDC_TENANT_CLAIM`, `OIDC_PERMISSION_CLAIMS`
  (lista separada por vírgula), `OIDC_DISCOVERY_TIMEOUT_SECONDS` (default
  10 s), `OIDC_REVOCATION_TIMEOUT_MS` (default 250) e `AUTH_DEV_MOCK`, e
  recusa com `envconfig.ErrInvalidVariable` o prazo acima da maior
  `time.Duration`; `FromEnv` lê e valida em um só passo. O `LogValue` informa `revocation=enabled` ou
  `disabled`, nunca o checker.
  `Validate` recusa a partida sem nenhum meio de resolver identidade
  (`ErrVerifierNotDeclared`: nem issuer, nem o mock de desenvolvimento), os
  dois meios juntos (`ErrMockWithVerifier`) e um issuer sem audiência ou sem
  a claim de tenant (`ErrMissingVariable`), e um checker com prazo não positivo.
- **`oidc.go`** — `Verifier`, que realiza `ports.Authenticator`.
  `NewVerifier(ctx, cfg)` descobre o issuer **na partida** — um processo que
  não alcança a autoridade falha ao subir, em vez de servir requisições que
  não conseguiria autenticar — e falha nomeando o issuer quando a descoberta
  não resolve (comum quando o Keycloak não tem um audience mapper no client:
  sem ele, todo token sai com `aud: ["account"]` e é recusado aqui, o que é
  configuração do realm, não deste módulo). `Authenticate` verifica assinatura,
  issuer e expiração pela biblioteca, recusa esquema fora de `Bearer` e token
  sem `sub` (`ErrSubjectUnresolved`), e resolve tenant e permissões pelas
  claims declaradas em `Config`. Um token recusado pela verificação, sem `sub`
  ou com claims ilegíveis sai em `warn` (`auth: token rejected`), com `error.type` =
  `Unauthenticated` e o motivo em `dmpf.auth.refusal_reason` — `token_expired`,
  `audience_mismatch`, `issuer_mismatch`, `signature_invalid` ou `malformed` —,
  nunca o token nem a mensagem da biblioteca (RF-A6). Quando o JWKS não pode
  ser obtido (o IdP fora do ar na primeira verificação ou na busca do `kid`
  novo de uma rotação), a falha não é recusa do token: sai em `warn`
  (`auth: signing keys unavailable`), com `error.type` = `TransientDependency`
  e sem `dmpf.auth.refusal_reason`, e a requisição continua recusada por
  `ErrCredentialRejected`. O logger nasce do
  `Config.LoggerProvider`, com o import path do módulo como scope. A
  descoberta e o JWKS passam pelo `otelhttp.NewTransport`, que abre um CLIENT
  `{http.request.method}` por requisição e grava `http.client.request.duration`
  (RF-B5, RF-D2).
- **`revocation.go`**, **`revocation_memory.go`** — a deny-list: `SessionRef`
  (`sub`, `sid` e `iat` verificados), `RevocationChecker`, `RevocationStore`,
  `Revocation` e `RevocationKind`. Uma entrada por `sid` nega todo token com
  aquele `sid`; uma por `sub` nega os tokens do sujeito com `iat <= revoked_at`,
  e um login posterior passa. Quando as duas negam, vence `session`. A entrada
  guarda o maior `revoked_at` visto e expira em `occurred_at + TTL + margem`.
  `MemoryStore` é a realização em memória, com relógio injetável, para teste e
  processo único; o Redis fica no provider `libs/backend/go/redis`.
- **Consulta no `Verifier`** — com `Config.Revocations`, o `Authenticate`
  consulta a deny-list depois de assinatura, issuer, audiência, expiração e
  `sub`, e antes de montar a identidade: no máximo uma consulta por requisição,
  nenhuma para token recusado antes, e nenhuma resposta guardada além dela
  (CTX-16). Um token revogado sai como os demais recusados, com
  `dmpf.auth.refusal_reason` = `session_revoked`. A consulta tem o prazo de
  `OIDC_REVOCATION_TIMEOUT_MS`, e erro ou prazo vencido **falham fechados**: o
  token é recusado com `ErrCredentialRejected` e sai em `warn`
  (`auth: revocation store unavailable`), com `error.type` =
  `TransientDependency` e sem motivo de recusa, como o JWKS indisponível. Sem
  checker, nada muda.
- **`revocation_apply.go`** — `Feed`, a alimentação. `ReadRevocationFeedEnv` lê
  `OIDC_REVOCATION_TTL_SECONDS` (obrigatório, o maior tempo de vida de token
  que o IdP emite; sem ele `NewFeed` recusa a partida) e
  `OIDC_REVOCATION_SKEW_SECONDS` (default 60). `Apply` grava um `SessionEvent`
  já normalizado: `session_ended` por `sid` e `subject_revoked` por `sub`, com
  o `occurred_at` do evento. É idempotente e monotônico, e devolve o desfecho
  (`applied`, `duplicate`, `expired`, `invalid` ou `foreign_issuer`); um erro do
  armazenamento volta ao chamador. `Record` conta e registra o desfecho
  separadamente, para que um adaptador que reaplica o mesmo evento o conte uma
  vez.
- **`backchannel.go`** — `BackchannelLogout`, `http.Handler` do OpenID Connect
  Back-Channel Logout 1.0. Aceita só `POST` com o form `logout_token` (até
  64 KiB), verifica-o por `VerifyLogout` do go-oidc com o `ClientID` do client
  do back-channel (o `aud` do logout token, distinto da audiência do access
  token) e exige `iat` presente e no máximo `OIDC_REVOCATION_SKEW_SECONDS` no
  futuro, que a biblioteca não confere. O token com `sid` vira `session_ended`
  (`jti`, `sub`, `sid`, `iat`); o token só com `sub` vira `subject_revoked`,
  que encerra todas as sessões do sujeito (Back-Channel Logout 1.0 §2.7).
  Responde `200`, `400` com `{"error":"invalid_request"}` ou `503` com o
  armazenamento ou o JWKS fora, sempre com `Cache-Control: no-store`. O `503`
  desvia, de propósito, do §2.8, que pede `400` para toda falha: separa a
  dependência fora do token recusado para quem observa o IdP. O Keycloak não
  reenvia o logout em nenhum dos dois status; quem cobre a perda é o polling.
  Não deduplica por `jti`: o `Apply` idempotente absorve a repetição.
- **`sessionpolling.go`** — `SessionPolling`, a reconciliação pela Admin REST
  do Keycloak, com client credentials de um client com `view-events`. Lê duas
  fontes: admin events de `USER` (`UPDATE` com `details.updated_enabled=false`,
  o usuário desabilitado, `DELETE` em `users/{id}` e `ACTION` em
  `users/{id}/logout`), que viram `subject_revoked`, o `DELETE` de
  `USER_SESSION` em `sessions/{id}`, que vira `session_ended` pelo `sid`, e
  eventos de usuário `LOGOUT`, que viram `session_ended`
  sem ler `ipAddress` nem `details`. Não guarda cursor: cada ciclo
  (`Interval`, default 5 s) relê a janela `[now − (TTL + margem), now]`, com
  `dateTo` fixo no ciclo e páginas de `first`/`max` até uma incompleta, e
  reaplica tudo. Assim, um logout que o back-channel perdeu e uma entrada que
  o armazenamento perdeu voltam no ciclo seguinte. Os ids vistos na janela só
  evitam contar o mesmo evento duas vezes e saem dela com o tempo. Uma fonte
  fora não para a outra; a falha sai em `warn`
  (`auth: session events unavailable`, `dmpf.auth.revocation.source` e, quando
  a Admin REST respondeu, `http.response.status_code`, que separa um `403` de
  configuração de uma queda) e o ciclo seguinte tenta de novo; depois de um
  `401`, com um token novo, porque o token em cache é descartado. Uma janela que
  passa de 1000 páginas sai em `warn`
  (`auth: session events window exceeded the page limit`), porque os eventos
  mais antigos dela ficam sem aplicar. O fim do contexto encerra o ciclo sem
  registrar falha. O realm precisa guardar os eventos `LOGOUT` por
  mais que a janela (`infra/README.md`).
- **`revocation_metrics.go`** — `dmpf.auth.revocation.refusals{kind}`,
  `dmpf.auth.revocation.store_errors`, `dmpf.auth.revocation.events{outcome}` e
  o histograma `dmpf.auth.revocation.apply_delay` (s, `now − occurred_at`), pelo
  `MeterProvider` injetado ou pelo global. Nenhum `sub`, `sid`, token ou IP vai
  a atributo, log ou erro.
- **`identity.go`** — `CredentialFrom(r *http.Request)` lê o cabeçalho
  `Authorization` sem interpretar o conteúdo — o material, nunca o sujeito
  nem o tenant, que só a verificação resolve (`CTX-06`). `DevAuthenticator`
  resolve identidade **do próprio credencial**, que a requisição declara como
  JSON (`{"sub", "tenant", "permissions"}`) — qualquer chamador forja qualquer
  sujeito e tenant, e por isso a partida a recusa sem `AUTH_DEV_MOCK`
  declarado; nenhuma parte de `IDN-01` é satisfeita por ela. Existe para
  desenvolvimento e para a suíte e2e caixa-preta do BFF (`bff:test-race`).
- **`claims.go`** — `claimValue`/`stringClaim`/`permissionsFrom`: toda claim é
  um **caminho**, testado primeiro como chave literal e só depois percorrido
  por `.`, para que um issuer que namespaceia a claim
  (`https://app.example.com/tenant_id`, padrão do Auth0) não colida com um que
  aninha (`realm_access.roles`, padrão do Keycloak) — testar a travessia
  primeiro quebraria o primeiro caso. O segmento `*` casa exatamente uma chave
  do objeto naquele nível (zero ou mais de uma deixam a claim ausente) e
  `stringClaim` aceita também uma lista de um único item. A configuração de
  referência do Keycloak, com a claim nativa `organization`, é
  `OIDC_TENANT_CLAIM=organization.*.tenant_id`. Em `OIDC_PERMISSION_CLAIMS`, o
  `*` lê as roles de qualquer chave que esteja sozinha naquele nível: com
  `resource_access.*.roles`, um token que traga só as roles do client `account`
  concede essas roles, e um que traga dois clients não concede nenhuma. Declare
  o client pelo nome (`resource_access.dmpf-bff.roles`). `permissionsFrom` une todas as
  claims declaradas, aceita as duas formas que um authorization server usa —
  string separada por espaço (RFC 6749 §3.3) ou array — e devolve o conjunto
  deduplicado e ordenado.

## O que o módulo não contém

A escolha do identity provider, que FND-07 deixa a critério de quem opera o
sistema; a forma de uma recusa de autenticação — 401 e o corpo publicado são
do contrato de cada borda (`libs/backend/go/http`); a verificação de
identidade de workload entre processos internos (mTLS no gRPC, SASL no Kafka
— ADR-052); a decisão de permissão por rota, que é `http.Route.Permission` e
`http.ResolveIdentity`, não deste módulo.

## Como rodar os testes localmente

Os unitários não usam rede: `oidc_test.go` sobe um provider OIDC falso local
(chaves geradas na suíte) para exercitar `NewVerifier`/`Authenticate`, inclusive
a claim `organization` nativa; `identity_test.go` cobre `DevAuthenticator` e
`CredentialFrom`; `claims_test.go` cobre as duas formas de claim (literal e
aninhada), o curinga de casamento único, a lista de um item e as duas formas de
permissão (string e array).

Os de integração (`keycloak_*_integration_test.go`, tag `integration`) rodam
contra o Keycloak de referência da infra de testes (ADR-064): login real por
código de autorização com PKCE seguido por HTTP, tenants por scope, recusa de
direct grant e implicit, segundo fator, back-channel logout e admin events,
com client e usuário descartáveis por execução onde o teste altera o realm.
`revocation_keycloak_integration_test.go` confere os eventos guardados pelo
realm e mede a recusa do mesmo token depois do logout pelo back-channel, do
logout sem receptor, do logout administrativo e do usuário desabilitado,
falhando se qualquer caso chegar a 60 s; `backchannel_test.go` e
`sessionpolling_test.go` cobrem os adaptadores com um IdP e uma Admin REST
falsos. O
`test-race` depende de `authn:test-keycloak-up`, que sobe só o Keycloak, e as
variáveis `KEYCLOAK_*` vêm do `.env.example` da raiz; sem `KEYCLOAK_URL`, a
suíte pula localmente e falha no CI.

```bash
pnpm nx run authn:test-race
```

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test-race,govulncheck -p authn
go run ./tools/dmpf-conformance/cmd/conformance --root .
```

O `dmpf-gate-check.sh` não alcança este módulo (o `depguard` seleciona por
`-domain/`, `-ports/`, `-application/`, `-contracts/`); o gate autoritativo é o
verificador.

## Referências

- `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — §4, `IDN-01`, `IDN-06`, `CTX-02`, `CTX-06`.
- `docs/specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md` — a spec que declara `ports.Authenticator` e esta realização.
- `docs/specs/SPEC-STEJG347-revogacao-de-sessao-authn.md` — a deny-list, a falha fechada e a alimentação pelos eventos de sessão.
- `docs/adr/064-keycloak-de-referencia-sem-extensao.md` — o Keycloak de referência, o back-channel logout e os admin events que os adaptadores leem.
- `docs/adr/049-contexto-de-execucao-viaja-no-context-context.md` — o carrier em que a identidade resolvida aqui é depositada, na borda.
- `docs/adr/052-identidade-de-workload-no-grpc-e-no-kafka.md` — a verificação de workload entre processos, contraste com a autenticação de sujeito deste módulo.
- `docs/specs/SPEC-1TFW24WV-observabilidade-ponta-a-ponta.md` — os `RF-*` citados aqui: a recusa de token e o `otelhttp` da descoberta.
- `libs/backend/go/http/README.md` — `ResolveIdentity`, `CredentialFrom` e a exigência de permissão por rota, que consomem este módulo.
- `libs/backend/go/ports/README.md` — `Authenticator`, `Credential`, `Identity`, `ExecutionContext`.
