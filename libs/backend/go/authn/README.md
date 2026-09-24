# authn

Bloco `provider` do kernel DMPF que realiza `ports.Authenticator` contra um
authorization server OIDC (FND-07 §4, `IDN-01`): descoberta do issuer na
partida, verificação de assinatura, issuer, audiência e expiração, e a leitura
de sujeito, tenant e permissões das claims que o operador declarar.

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

Dependência externa declarada: `github.com/coreos/go-oidc/v3` `>=3.21.0 <4`
(`io.network`) — descoberta do issuer, JWKS e rotação são da biblioteca; o que
fica aqui é a leitura das claims que o operador nomeou (IDN-01). `public_integration_surface: false`:
nenhum outro contexto importa este módulo além do composition root que o
monta.

## O que o módulo contém

- **`config.go`** — `Config{Issuer, Audience, TenantClaim, PermissionClaims,
  DiscoveryTimeout, DevMock}`. `Defaults()` assume um access token de Keycloak:
  `scope` como string separada por espaço e roles de realm aninhadas em
  `realm_access.roles`. `ReadEnv(lookup)` lê `DMPF_OIDC_ISSUER`,
  `DMPF_OIDC_AUDIENCE`, `DMPF_OIDC_TENANT_CLAIM`, `DMPF_OIDC_PERMISSION_CLAIMS`
  (lista separada por vírgula), `DMPF_OIDC_DISCOVERY_TIMEOUT_SECONDS` (default
  10 s) e `DMPF_AUTH_DEV_MOCK`; `FromEnv` lê e valida em um só passo.
  `Validate` recusa a partida sem nenhum meio de resolver identidade
  (`ErrVerifierNotDeclared`: nem issuer, nem o mock de desenvolvimento), os
  dois meios juntos (`ErrMockWithVerifier`) e um issuer sem audiência ou sem
  a claim de tenant (`ErrMissingVariable`).
- **`oidc.go`** — `Verifier`, que realiza `ports.Authenticator`.
  `NewVerifier(ctx, cfg)` descobre o issuer **na partida** — um processo que
  não alcança a autoridade falha ao subir, em vez de servir requisições que
  não conseguiria autenticar — e falha nomeando o issuer quando a descoberta
  não resolve (comum quando o Keycloak não tem um audience mapper no client:
  sem ele, todo token sai com `aud: ["account"]` e é recusado aqui, o que é
  configuração do realm, não deste módulo). `Authenticate` verifica assinatura,
  issuer e expiração pela biblioteca, recusa esquema fora de `Bearer` e token
  sem `sub` (`ErrSubjectUnresolved`), e resolve tenant e permissões pelas
  claims declaradas em `Config`.
- **`identity.go`** — `CredentialFrom(r *http.Request)` lê o cabeçalho
  `Authorization` sem interpretar o conteúdo — o material, nunca o sujeito
  nem o tenant, que só a verificação resolve (`CTX-06`). `DevAuthenticator`
  resolve identidade **do próprio credencial**, que a requisição declara como
  JSON (`{"sub", "tenant", "permissions"}`) — qualquer chamador forja qualquer
  sujeito e tenant, e por isso a partida a recusa sem `DMPF_AUTH_DEV_MOCK`
  declarado; nenhuma parte de `IDN-01` é satisfeita por ela. Existe para
  desenvolvimento e para a suíte e2e caixa-preta do BFF (`bff:test-race`).
- **`claims.go`** — `claimValue`/`stringClaim`/`permissionsFrom`: toda claim é
  um **caminho**, testado primeiro como chave literal e só depois percorrido
  por `.`, para que um issuer que namespaceia a claim
  (`https://app.example.com/tenant_id`, padrão do Auth0) não colida com um que
  aninha (`realm_access.roles`, padrão do Keycloak) — testar a travessia
  primeiro quebraria o primeiro caso. `permissionsFrom` une todas as claims
  declaradas, aceita as duas formas que um authorization server usa — string
  separada por espaço (RFC 6749 §3.3) ou array — e devolve o conjunto
  deduplicado e ordenado.

## O que o módulo não contém

A escolha do identity provider, que FND-07 deixa a critério de quem opera o
sistema; a forma de uma recusa de autenticação — 401 e o corpo publicado são
do contrato de cada borda (`libs/backend/go/http`); a verificação de
identidade de workload entre processos internos (mTLS no gRPC, SASL no Kafka
— ADR-052); a decisão de permissão por rota, que é `http.Route.Permission` e
`http.ResolveIdentity`, não deste módulo.

## Como rodar os testes localmente

Todos unitários, sem rede: `oidc_test.go` sobe um provider OIDC falso local
(chaves geradas na suíte, sem depender de um Keycloak real) para exercitar
`NewVerifier`/`Authenticate`; `identity_test.go` cobre `DevAuthenticator` e
`CredentialFrom`; `claims_test.go` cobre as duas formas de claim (literal e
aninhada) e as duas formas de permissão (string e array).

```bash
pnpm nx run authn:test-race
```

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test-race,govulncheck -p authn
go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop
```

O `dmpf-gate-check.sh` não alcança este módulo (o `depguard` seleciona por
`-domain/`, `-ports/`, `-application/`, `-contracts/`); o gate autoritativo é o
verificador.

## Referências

- `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — §4, `IDN-01`, `IDN-06`, `CTX-02`, `CTX-06`.
- `docs/specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md` — a spec que declara `ports.Authenticator` e esta realização.
- `docs/adr/049-contexto-de-execucao-viaja-no-context-context.md` — o carrier em que a identidade resolvida aqui é depositada, na borda.
- `docs/adr/052-identidade-de-workload-no-grpc-e-no-kafka.md` — a verificação de workload entre processos, contraste com a autenticação de sujeito deste módulo.
- `libs/backend/go/http/README.md` — `ResolveIdentity`, `CredentialFrom` e a exigência de permissão por rota, que consomem este módulo.
- `libs/backend/go/ports/README.md` — `Authenticator`, `Credential`, `Identity`, `ExecutionContext`.
