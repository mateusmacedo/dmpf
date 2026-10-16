---
id: SPEC-SBW7HT6Y
slug: keycloak-de-referencia
title: DMPF — Keycloak de referência na infra do DMPF: realm com Organizations, claim de tenant e eventos de sessão
stage: done
priority: P1
depends_on: [SPEC-VJMM2DE5]
ticket_url: https://linear.app/mmda/issue/DEVS-97/spec-sbw7ht6y-dmpf-keycloak-de-referencia-na-infra-do-dmpf-realm-com
subtask_urls: []
created: 2026-10-12
---

# SPEC-SBW7HT6Y: Keycloak de referência na infra do DMPF

## Resumo

Pôr um Keycloak 26.x de referência na infra do DMPF — Compose local, infra de testes e base Kustomize —, com um realm único importado (Organizations, código de autorização com PKCE e segundo fator por OTP ou WebAuthn), o tenant ativo lido pelo `authn` direto da claim nativa `organization`, sem extensão no Keycloak, e um canal definido de eventos de sessão. O `authn` do kernel assume Keycloak nos defaults, mas nenhum teste dele exercita um Keycloak real, e a infra não oferece um; sem a referência, cada consumidor monta o realm por conta própria e descobre os desencontros em produção. A origem é o consumidor externo antecipa-market, SPEC-GXGASP24 (DEVS-76), que adota Keycloak 26.x autogerenciado, realm único, como IdP. O canal de eventos de sessão definido aqui é o insumo da SPEC-STEJG347 (revogação de sessão no `authn`).

## Contexto

### Problema

- O verificador `libs/backend/go/authn` exige issuer único, audiência e claim de tenant (`config.go:113-126`), e seus `Defaults()` leem um access token de Keycloak (`config.go:53-60`; `README.md:41-43`). A claim de tenant só vale se for string não vazia (`claims.go:37-47`; `oidc.go:93-96`), e a audiência só passa quando o client tem audience mapper (`oidc.go:34-36`, `:53`).
- No Keycloak 26.x, a claim `organization` é um objeto indexado pelo alias da organização, não um `tenant_id` string. Um usuário com vínculo em várias organizações precisa de um tenant ativo no token, e o kernel nunca o escolhe sozinho (CTX-06).
- O `authn` percorre caminhos de claim só por chaves fixas (`claims.go:14-35`): o alias, que é a chave dinâmica do objeto `organization`, não tem caminho que o alcance, e o atributo de organização chega como lista, que `stringClaim` recusa (`claims.go:37-47`).
- A infra do DMPF não tem IdP: nenhum arquivo de `infra/local/compose/` o declara, a infra de testes sobe Postgres, Redpanda, Redpanda com SASL e floci (`infra/test/compose.yml:9,26,49,84`) e as bases Kustomize são Postgres e Redpanda (`infra/k8s/overlays/dev/kustomization.yaml:12-15`). Os testes do `authn` usam um provider OIDC falso (`oidc_test.go:100-122`; `README.md:103-105`).
- Não há fonte de eventos de sessão (logout, revogação) definida, e a SPEC-STEJG347 precisa de uma para alimentar a deny-list.

### Estado verificado no repositório

| Ponto | Evidência |
| ----- | --------- |
| O go-oidc recusa issuer configurado diferente do issuer do discovery | `github.com/coreos/go-oidc/v3@v3.21.0/oidc/oidc.go:331` (versão em `libs/backend/go/authn/go.mod:6`) |
| Tenant ausente não derruba a autenticação: `Identity.Tenant` fica nulo e a rota que exige tenant responde 403 `tenant-unresolved` | `oidc.go:93-96`; `libs/backend/go/http/identity.go:44-46` |
| Profile fora de `all` e `dmpf` não entra no `infra-budget`; o `load` é o precedente | `tools/infra-budget.sh:11`; `infra/README.md:64` |
| Todo serviço do Compose declara teto e mínimo de recursos; a soma dos tetos já fica em 96,3% do limite de 97% | `infra/README.md:126,132` |
| O `infrasync` gera só `overlays/{dev,hmg}/apps` e `overlays/dev/databases`; o `kustomization.yaml` de cada overlay não é gerado nem hasheado no `dmpf.rendered.json` | `tools/dmpf-conformance/infrasync/templates.go:29-31` |
| A infra estática é saída do `init`; o CI reprova a cópia que diverge do template | `infra/README.md:3`; `.github/workflows/ci.yml:199-207` |
| Arquivo `managed` é do plugin enquanto o hash confere; `seed` nunca é reescrito | `tools/dmpf-plugin/src/generators/init/generator.ts:45-48,233-255` |
| O `sync-init` reaplica os templates sem sobrescrever arquivo editado | `tools/dmpf-plugin/migrations.json` |
| Integração: sem a variável, o teste pula localmente e falha no CI | `libs/backend/go/testkit/tb/tb.go:54-65` |

### Fatos externos (Keycloak: documentação consultada em 12/10/2026; código da tag `26.8.1` lido e executado em container em 15/10/2026)

| Fato | Fonte |
| ---- | ----- |
| Organizations é suportado desde a 26.0 | https://www.keycloak.org/2024/10/keycloak-2600-released |
| A claim `organization` é um objeto por organização; id e atributos só entram com as opções "Add organization id" e "Add organization attributes" do mapper | https://www.keycloak.org/docs/latest/server_admin/index.html#_mapping_organization_claims_ |
| O `Organization Membership` mapper nativo, multivalorado com JSON, emite `{<alias>: {<atributos como lista>, id}}` por organização; não multivalorado, emite só o alias da primeira organização resolvida e desliga id e atributos; não há opção que achate a claim em string; o arquivo é idêntico em `26.8.0`, `26.8.1` e `main` | `OrganizationMembershipMapper.java` (`resolveValue`, `getEffectiveModel`), https://github.com/keycloak/keycloak/blob/26.8.1/services/src/main/java/org/keycloak/organization/protocol/mappers/oidc/OrganizationMembershipMapper.java |
| Scope `organization` (uma organização; com várias, o Keycloak pede a escolha no login), `organization:<alias>` (específica) e `organization:*` (todas). Alias inexistente ou formatos misturados devolvem `invalid_scope`; usuário sem vínculo com o alias pedido autentica normalmente, sem a claim | idem; execução em container `26.8.1` |
| O import não sobrescreve só o mapper do scope `organization` (declarar o scope com os defaults ativos devolve 409); declarar os `clientScopes` completos com o mapper editado funciona | `RealmManager.java:739-743`; execução em container `26.8.1` |
| Back-channel logout: POST de um logout token (`typ: logout+jwt`; `iss`, `aud`=client, `sub`, `sid`, `events`, `jti`, `iat`) aos clients com sessão e front-channel desligado, inclusive público; cobre logout do usuário, encerramento administrativo da sessão, `POST /users/{id}/logout` e Account console; não cobre usuário desabilitado, expiração nem `logout-all`; entrega única e síncrona, sem reentrega | `AuthenticationManager.java` (`backchannelLogoutAll`), `DefaultTokenManager.java:352`; execução em container `26.8.1` (10-110 ms) |
| Admin events (`GET /admin/realms/{r}/admin-events`) registram desabilitar usuário (`UPDATE USER`, `details.previous_enabled`/`updated_enabled`) e `users/{id}/logout`; leitura com a role `view-events`; pull sem cursor (`dateFrom` + `id`) | `RealmAdminResource.java:899`; execução em container `26.8.1` |
| O access token carrega `sid` sempre, igual ao `session_state`, ao `sid` do logout token e ao `sessionId` dos eventos; `accessTokenLifespan` padrão é 300 s | `TokenManager.java:1130`, `Constants.java:59`; execução em container `26.8.1` |
| Health em `/health/ready` só na porta de management 9000; a imagem não tem curl nem wget, mas tem bash | `docs/guides/observability/health.adoc`; execução em container `26.8.1` |
| Placeholder `${VAR}` no realm importado vem só do ambiente; sem a variável e sem padrão, fica literal | `StringPropertyReplacer.java`; execução em container `26.8.1` |
| Num realm importado, só o `admin-cli` nasce com direct grant; declará-lo desligado no arquivo impede a criação padrão | `RealmManager.java:227-239,648-655` |
| Os listeners nativos são o de log e o de e-mail | https://www.keycloak.org/docs/latest/server_admin/index.html#event-listener |
| O Event Listener SPI é interno e pode mudar em qualquer release; os script providers são Preview e vêm desabilitados | https://www.keycloak.org/docs/latest/server_development/index.html |
| `--import-realm` lê `/opt/keycloak/data/import` e pula o realm que já existe | https://www.keycloak.org/server/importExport |
| `hostname` é obrigatório, aceita URL completa e não é resolvido dinamicamente | https://www.keycloak.org/server/hostname |
| Existem: método PKCE `S256` por client, "Condition - User Role", `OTP Form` e `WebAuthn Authenticator` no fluxo de browser | https://www.keycloak.org/docs/latest/server_admin/index.html |
| A tag `26.8.1` (13/10/2026) é a mais nova `26.x` em `quay.io/keycloak/keycloak`, com digest `sha256:79c623a22abacf3fe6c00a5fb30f5b7843d717b234033b80995891c06116d244`; as notas de 26.0 a 26.8 não mudam o formato da claim `organization` nem `organization:*` | https://quay.io/repository/keycloak/keycloak?tab=tags; `docs/documentation/{release_notes,upgrading}` da tag |
| `KC_HOSTNAME=http://keycloak.localhost:8180` com alias de rede `keycloak.localhost` no serviço dá o mesmo `issuer` no host (resolve para loopback) e em containers musl, glibc e Go estático da rede Compose | spike em container `26.8.1`, 15/10/2026 |

### Não verificado

- O atraso do back-channel logout e do polling de admin events na infra de testes do CI: medido só em máquina local.
- O `sid` de sessão offline.
- O teto de recursos do Keycloak no Compose fora da máquina de medição (540-563 MiB com limite de 1 GiB).

<constraints>
- [P0] O tenant e o sujeito do `authn` vêm só do token verificado (CTX-06, IDN-01): o mapper e a configuração de referência nunca leem tenant de header, query ou corpo.
- [P0] Tenant ausente é ausência (IDN-20): se a claim `organization` não traz exatamente uma organização com exatamente um valor de `tenant_id`, o `authn` não resolve tenant; nenhum valor sintético nem escolha arbitrária entre organizações.
- [P0] O Keycloak de referência é infra: nenhum módulo Go do kernel passa a importar ou a depender dele em runtime, e o `authn` segue OIDC genérico.
- [P0] O realm de base, importado em todo ambiente, não contém usuário, senha nem organização de teste; esses dados entram só no Compose local, na infra de testes e no overlay `dev`, por um passo de init. O overlay `hmg` recebe os Secrets de fora e nunca carrega credencial do repositório.
- [P0] A infra estática muda pelos templates de `tools/dmpf-plugin/templates/init/`, nunca pela cópia: `init --dry-run` sem diff no CI (`ci.yml:199-207`).
- [P0] O profile `keycloak` fica fora de `all` e `dmpf`: o `infra-budget` não pode regredir.
- [P0] O Keycloak de referência roda sem extensão: nenhum provider, script mapper, SPI próprio nem feature Preview. O `authn` se adapta ao formato nativo das claims por configuração genérica, sem nada específico de Keycloak.
- [P1] Versão e digest fixos em todo manifesto; nenhuma tag móvel.
</constraints>

## Requisitos

### Funcionais

**A. Infra de referência**

- [ ] **[P0] Versão fixada**: `quay.io/keycloak/keycloak:26.8.1@sha256:79c623a22abacf3fe6c00a5fb30f5b7843d717b234033b80995891c06116d244` em todos os manifestos (mesma disciplina de `infra/test/compose.yml:10,27`), em modo `start` com `--http-enabled=true`, `--health-enabled=true` e `--cache=local`, sobre a imagem oficial; o que as notas de 26.0 a 26.8 mudam em Organizations, back-channel logout e import vai para o ADR novo.
- [ ] **[P0] Compose local**: criar `infra/local/compose/keycloak.yml` com profile `keycloak`, fora de `all` e `dmpf`, e o passo `keycloak-init`; publicar em `127.0.0.1:${KEYCLOAK_PORT:-8180}` (8180 está livre na tabela de `infra/README.md:95-114`); declarar teto e mínimo medidos, healthcheck por bash na porta de management 9000 (a imagem não tem curl), `dev-file` em volume e o realm importado por `--import-realm`.
- [ ] **[P0] Infra de testes**: acrescentar `keycloak` (profile `keycloak`) e `keycloak-init` (profile `init`) a `infra/test/compose.yml`, em `127.0.0.1:18180`, com hostname `http://127.0.0.1:18180`, `dev-file` em tmpfs, o mesmo realm e `extra_hosts` com `host.docker.internal:host-gateway`; as variáveis `KEYCLOAK_*` da infra de testes (URL, host de callback, admin, secret do polling, senha e segredo OTP do seed) ao `.env.example` da raiz, que o `test-env.sh` já carrega. O `tools/dmpf-plugin/scripts/test-infra.sh keycloak` sobe e semeia o Keycloak fora do `up`, para o consumidor externo não o pagar em todo teste de integração, e `.github/workflows/sonar.yml` acompanha a lista de serviços.
- [ ] **[P0] Base Kustomize**: criar `infra/keycloak/` com os manifestos e o realm como fonte única — o Compose monta o arquivo por bind e o Kustomize o gera como ConfigMap, como em `infra/observability` —, e o Component `infra/keycloak/dev/` com o seed, o PVC e os Secrets de desenvolvimento. Os overlays `dev` e `hmg`, que nem o `infrasync` nem o `init` geram (`infra/README.md:3`), passam a compor `../../../keycloak` (o `dev` também o Component); o `hmg` recebe de fora os Secrets `keycloak-admin`, `keycloak-db` e `keycloak-session-events` e o ConfigMap `keycloak-hostname`, e os acrescenta ao `secrets.example.yaml.tmpl` que já tem. Os pods seguem o hardening do repositório (`runAsNonRoot`, `seccompProfile`, sem escalada de privilégio, `capabilities` vazias, sem token de service account).
- [ ] **[P0] Banco do Keycloak**: `dev-file` no Compose local (volume), na infra de testes (tmpfs) e no `dev` (PVC); Postgres no `hmg`, pelo Secret `keycloak-db`. O Job de bancos do `dev` só provisiona apps com `infra.json` (`infra/README.md:3,70`), e o `dev-file` dispensa banco e Job extras fora de produção.
- [ ] **[P0] Issuer único e estável**: `KC_HOSTNAME=http://keycloak.localhost:8180` e `KC_HTTP_PORT=8180` no Compose local, com o alias de rede `keycloak.localhost` no serviço, para o mesmo `iss` valer no host (BFF em `bff:serve`) e em container da rede do Compose sem editar o `/etc/hosts`; `http://127.0.0.1:18180` na infra de testes, cujo consumidor é só o host; no K8s, `KC_HOSTNAME` pelo ConfigMap obrigatório `keycloak-hostname` (`http://keycloak:8080` no `dev`, o endereço público fornecido pelo operador no `hmg`), para que a falta do valor impeça o pod de subir em vez de emitir um issuer interno.
- [ ] **[P0] CI**: `.github/workflows/dmpf-go-ci.yml` ganha o input `keycloak` (padrão `false`, para não exigir o realm do consumidor externo); com ele ligado e o `authn` entre os afetados, sobe o Keycloak por `docker run --network host` antes do estágio de providers, semeia, exporta `KEYCLOAK_URL` e `KEYCLOAK_CALLBACK_HOST=127.0.0.1` e remove o container no cleanup `if: always()`; o `ci.yml` liga o input.
- [ ] **[P1] Atualização do realm**: como o import pula o realm existente, mudar o arquivo exige, por ambiente, recriar o armazenamento (local e testes), apagar o PVC (`dev`) ou rodar `kc.sh import --override` com o servidor parado (`hmg`); `infra/README.md` e o ADR novo descrevem o procedimento.
- [ ] **[P0] Infra pelos templates do `init`**: todo arquivo novo entra em `templates/init/managed/` (`infra/local/compose/keycloak.yml`, `infra/test/compose.yml`, `infra/keycloak/**`, `.env.example`), o `include` entra no `seed` `docker-compose.yml__tmpl__` e o `dmpf.rendered.json` é regenerado.
- [ ] **[P1] Consumidores existentes**: o `sync-init` leva os arquivos `managed`; o `include` no `seed` não é reescrito, então o guia de consumo externo documenta o passo manual.

**B. Realm de referência**

- [ ] **[P0] Realm único `dmpf` importado**: Organizations ligadas, proteção contra força bruta com bloqueio temporário, sem federação de IdP, sem cadastro aberto e sem usuário de pessoa nem organização no arquivo; a única entrada em `users` é a service account do `dmpf-session-events`, que recebe a role por ali.
- [ ] **[P0] Client de usuário `dmpf-portal`**: público, só código de autorização com PKCE `S256` obrigatório; direct access grant e implicit flow desligados em todo client do realm, inclusive o `admin-cli` do realm `dmpf`, declarado desligado no arquivo (o init usa o `admin-cli` do realm `master`).
- [ ] **[P0] Audiência do `authn`**: audience mapper que inclui `dmpf-bff` no access token, para o `OIDC_AUDIENCE` passar (`oidc.go:34-36`); o client `dmpf-bff` existe no realm como bearer-only, porque sem ele o `aud` some em silêncio.
- [ ] **[P0] Mapper de organização nativo**: o scope `organization` usa o `Organization Membership` mapper nativo, multivalorado, com JSON e *Add organization attributes* ligados, incluído no access token; nenhum mapper customizado no realm. Como o import não sobrescreve só esse mapper, o realm declara os `clientScopes` completos com o mapper editado, partindo de um export reproduzível da imagem fixada.
- [ ] **[P0] Segundo fator**: fluxo de browser com o sub-fluxo condicional de 2FA habilitado (`OTP Form` e `WebAuthn Authenticator` como alternativas), exigido pela role de realm `mfa-required` por "Condition - User Role"; o consumidor liga as roles dele a essa. Usuário com a role e sem credencial não conclui o login, então quem atribui a role atribui também a required action de cadastro (`CONFIGURE_TOTP` ou `webauthn-register`).
- [ ] **[P0] Tempo de vida do access token explícito**: `accessTokenLifespan: 300` declarado no realm importado e documentado como o insumo do TTL da deny-list da SPEC-STEJG347.
- [ ] **[P0] Client de eventos `dmpf-session-events`**: confidencial, só service account, com a role `view-events` de `realm-management`; o secret vem de `${DMPF_SESSION_EVENTS_SECRET}`, variável obrigatória em todo ambiente (o Compose define o valor de desenvolvimento; no K8s ela vem do Secret `keycloak-session-events` por `secretKeyRef` não opcional, e o pod não sobe sem ele), para que o placeholder nunca vire um segredo literal conhecido. Admin events ligados no realm.
- [ ] **[P0] Dados de teste fora do realm de base**: um passo de init (`keycloak-init` no Compose local e na infra de testes, no padrão de `redpanda-sasl-init` — `infra/test/compose.yml:74-76` — e um Job no Component `dev`) reconcilia as organizações `acme` e `globex`, cada uma com o atributo `tenant_id`, e os usuários `alice` (só `acme`), `bob` (as duas), `carol` (nenhuma) e `dave` (só `acme`, com a role `mfa-required` e uma credencial OTP semeada), com credenciais de desenvolvimento. Cada relação é garantida, não só criada, e o passo converge também a partir de estado parcial. O `hmg` não executa o passo.

**C. Tenant ativo**

- [ ] **[P0] Curinga de casamento único no `authn`**: `claimValue` (`claims.go:14-35`) passa a aceitar o segmento `*`, que casa exatamente uma chave do objeto naquele nível; zero ou mais de uma chave deixam o caminho ausente. A busca literal pelo caminho inteiro continua primeiro, para as claims com ponto no nome. O curinga é genérico: vale para qualquer emissor que indexe um objeto por chave dinâmica, e não cita Keycloak. Nas permissões, vale a mesma unicidade, para que um caminho com curinga nunca una permissões de organizações diferentes.
- [ ] **[P0] Valor único**: a claim de tenant resolve para a string não vazia ou para a lista com exatamente uma string não vazia, porque o atributo de organização chega como lista; lista vazia, com mais de um item ou com item que não é string deixam o tenant ausente.
- [ ] **[P0] Tenant do atributo da organização ativa**: com `OIDC_TENANT_CLAIM=organization.*.tenant_id`, `Identity.Tenant` é o valor único do atributo `tenant_id` da organização ativa. Sem a organização, com mais de uma, sem o atributo ou com mais de um valor, o tenant fica ausente.
- [ ] **[P0] Tenant ativo por scope**: o client pede `organization:<alias>` para fixar o tenant, ou `organization` para o Keycloak pedir a escolha quando há várias organizações. Trocar de tenant é uma nova autorização com outro alias; o token anterior mantém o tenant antigo até expirar.
- [ ] **[P0] Configuração de referência do `authn`**: documentar, em `infra/README.md` e no README do `authn`, `OIDC_ISSUER`, `OIDC_AUDIENCE=dmpf-bff` e `OIDC_TENANT_CLAIM=organization.*.tenant_id`, com `OIDC_PERMISSION_CLAIMS` no default (`scope,realm_access.roles`, `config.go:53-60`). O `.env.example` da raiz ganha só as variáveis `KEYCLOAK_*` da infra de testes: as de cada processo vivem em `apps/backend/<app>/deploy/.env.example` (`.env.example:9-10`). As únicas mudanças de código de produção no `authn` são o curinga e a lista de um item no `stringClaim`.

**D. Eventos de sessão**

- [ ] **[P0] Fonte de eventos de sessão** (decidida com medição em container `26.8.1`): back-channel logout no `dmpf-portal`, que cobre logout do usuário, encerramento administrativo da sessão, `POST /users/{id}/logout` e Account console, em entrega única e síncrona, sem reentrega; mais polling de admin events pelo `dmpf-session-events`, que cobre desabilitar usuário e `users/{id}/logout`, por `dateFrom` com deduplicação por `id`. A expiração da sessão não gera evento a tempo e fica coberta pela vida do token. A entrega é best-effort; o Event Listener SPI (provider próprio) e o SSF (experimental) ficam fora pela restrição de Keycloak sem extensão, e o ADR novo registra o que fica descoberto.
- [ ] **[P0] Contrato do evento de sessão**: o contrato é um mapeamento por fonte para `event_id` (único por evento, chave de idempotência), `kind` (`session_ended` com `session_id`, ou `subject_revoked` com `subject`), `issuer`, `subject`, `session_id` (opcional em `subject_revoked`) e `occurred_at`. Logout token → `session_ended` (`event_id`=`jti`, `issuer`=`iss`, `subject`=`sub`, `session_id`=`sid`, `occurred_at`=`iat`); admin event → `subject_revoked` (`event_id`=id do evento, `issuer`=issuer do realm, `subject`=id do usuário, `occurred_at`=`time`). Normalização, deduplicação e cursor ficam no adaptador da SPEC-STEJG347. Nenhum token nem e-mail viaja no evento; `subject` e `session_id` são identificadores pessoais pseudônimos (LGPD art. 13 §4º), classificados como tais por `DAT-01` (`docs/dmpf/contexto-erros-seguranca.md:1425`), com minimização, acesso restrito e retenção declarada, e continuam proibidos em log e métrica. Se o canal for Kafka, o envelope segue o do kernel.
- [ ] **[P0] Origem verificada do evento**: o evento de sessão tem origem verificada. Com back-channel logout, o logout token (JWS) só vale com assinatura, `iss`, `aud` (`dmpf-portal`) e claim `events` verificados; o admin event só vale lido da Admin API do próprio issuer com a credencial do `dmpf-session-events`. Quem escreve no canal não pode revogar sessão alheia.
- [ ] **[P0] Emissor no Keycloak de referência**: o `dmpf-portal` registra o back-channel logout com sessão obrigatória e front-channel desligado, e a URL vem de `${DMPF_BACKCHANNEL_LOGOUT_URL:}` (vazia desliga o envio), alcançável pela rede do Compose e do cluster; o teste de integração registra a própria URL por Admin REST.
- [ ] **[P0] `sid` no token**: o access token do `dmpf-portal` carrega `sid`, e o `session_id` do evento é igual a ele; um teste de integração lê o `sid` do token real e o compara ao do evento de logout, porque a recusa em 60 s da SPEC-STEJG347 (P0) depende dele.

**E. Testes**

- [ ] **[P0] Teste de integração do `authn` contra o Keycloak**: obtém os tokens pelo fluxo real de código de autorização com PKCE, seguindo o login por HTTP, sem client de direct grant; lê `KEYCLOAK_URL` por `tb.Env` e verifica `Identity.Subject`, `Identity.Tenant` e a recusa dos casos de erro.
- [ ] **[P0] Testes unitários do curinga**: em `claims_test.go`, nenhum, um e dois casamentos do `*`; valor string, lista de um item, lista de dois itens e item não string; caminho literal com ponto no nome preservado; `realm_access.roles` e a suíte atual do `authn` sem alteração de caso existente.
- [ ] **[P1] Target do `authn`**: o `test-race` do `authn` usa o executor `test-env`, sem cache e com `-tags=integration`, e depende de `authn:test-keycloak-up`, que sobe só o Keycloak da infra de testes (ADR-054, item 7).

**F. Documentação**

- [ ] **[P1] ADR novo (ADR-064)**: registra a versão fixada e o comando de export do realm, a leitura do tenant pelo curinga no `authn` em vez de extensão no Keycloak, o issuer local, o banco por ambiente, o procedimento de atualização do realm, a fonte de eventos com as garantias de entrega e o que fica descoberto, o mapeamento por fonte para o contrato, o secret do polling, o input do CI e o que cada alternativa descartada custaria.
- [ ] **[P1] Guias**: `infra/README.md` (profile, portas, hostname, variáveis, atualização do realm, Secrets do `hmg`), `tools/dmpf-plugin/README.md` (diretório `infra/keycloak`) e `docs/guides/dmpf-consumo-externo.md` (passo do `include`, overlays e variáveis `OIDC_*`).

### Não-funcionais

- **Reprodutibilidade**: o import de realm é determinístico, e o realm parte de um export reproduzível da imagem fixada; como o Keycloak pula o realm existente, a mudança no arquivo segue o procedimento de atualização por ambiente.
- **Orçamento**: `tools/infra-budget.sh` continua verde; o profile `keycloak` não entra em `all` nem em `dmpf`.
- **Exposição**: toda porta de host em `127.0.0.1`; porta administrativa fora da rede pública no `hmg`.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Infra | `infra/local/compose/keycloak.yml`, `infra/test/compose.yml`, `infra/keycloak/**` (realm, base Kustomize e Component `dev` com o seed), overlays `dev` e `hmg` |
| Plugin Nx | `tools/dmpf-plugin/templates/init/{managed,seed}`, `dmpf.rendered.json`, `tools/dmpf-plugin/scripts/{test-infra,test-env}.sh`, README do plugin |
| `authn` | curinga de casamento único em `claims.go` e seus testes; teste de integração, target e README |
| CI | input `keycloak` no `dmpf-go-ci.yml`, `ci.yml` e `sonar.yml`; `init --dry-run` sem diff e `infra-budget` verdes |
| Documentação | ADR-064, `infra/README.md`, guia de consumo externo |

## Localização de código

```text
infra/local/compose/keycloak.yml                                novo (profile keycloak: keycloak + keycloak-init)
infra/test/compose.yml                                          serviços keycloak e keycloak-init
infra/keycloak/                                                 novo: kustomization, manifestos, dmpf-realm.json
infra/keycloak/dev/                                             novo: Component do dev (seed.sh, Job, PVC, Secrets de dev)
infra/k8s/overlays/{dev,hmg}/kustomization.yaml                 compõem ../../../keycloak (cópias mantidas à mão)
infra/k8s/overlays/hmg/secrets.example.yaml.tmpl                keycloak-admin, keycloak-db, keycloak-session-events
tools/dmpf-plugin/templates/init/managed/infra/...              os mesmos arquivos, como __tmpl__
tools/dmpf-plugin/templates/init/managed/{.env.example,env.platform.example}__tmpl__   KEYCLOAK_URL, KEYCLOAK_CALLBACK_HOST, KEYCLOAK_PORT
tools/dmpf-plugin/templates/init/seed/infra/local/docker-compose.yml__tmpl__   include
tools/dmpf-plugin/scripts/{test-infra,test-env}.sh              sobe e semeia o Keycloak; padrões das variáveis
.github/workflows/{dmpf-go-ci,ci,sonar}.yml                     input keycloak, docker run, seed, cleanup, lista de serviços
dmpf.rendered.json                                              regenerado pelo init
libs/backend/go/authn/claims.go                                 curinga de casamento único e valor de lista com um item
libs/backend/go/authn/claims_test.go                            testes unitários do curinga
libs/backend/go/authn/                                          teste de integração, project.json, go.mod, README
docs/adr/064-keycloak-de-referencia-sem-extensao.md             ADR novo
```

## Design

### Fluxo

```text
portal ──código + PKCE──> Keycloak (realm dmpf, Organizations)
  scope=openid organization:<alias>        │
                                           ├── back-channel logout ──┐
                                           ├── admin events (pull) ──┴──> SPEC-STEJG347
  access token                             ▼
  { iss, aud=dmpf-bff, sub, sid,
    organization: { acme: { tenant_id: ["t-acme"] } } }   ← mapper nativo
        │ Authorization: Bearer
        ▼
  bff → authn.Verifier (OIDC_ISSUER, OIDC_AUDIENCE,
                        OIDC_TENANT_CLAIM=organization.*.tenant_id) → ports.Identity{Tenant: "t-acme"}
```

### Tenant ativo por pedido

| Scope pedido | Usuário | Claim `organization` | `Identity.Tenant` |
| ------------ | ------- | -------------------- | ----------------- |
| `organization:acme` | membro de `acme` | uma entrada | atributo de `acme` |
| `organization` | membro de uma organização | uma entrada | atributo dela |
| `organization` | membro de duas | o Keycloak pede a escolha; uma entrada | atributo da escolhida |
| `organization:*` | membro de duas | duas entradas | ausente (o `*` casa duas chaves) |
| `organization:acme` | sem vínculo com `acme` | login concluído sem a claim | ausente |
| `organization:<alias inexistente>` | qualquer | pedido recusado com `invalid_scope` | sem token |
| sem scope `organization` | qualquer | sem claim | ausente |

### Forma do realm importado

| Elemento | Valor de referência |
| -------- | ------------------- |
| Realm | `dmpf`, Organizations ligadas, força bruta com bloqueio temporário, `accessTokenLifespan: 300`, admin events ligados |
| Client `dmpf-portal` | público; código de autorização; PKCE `S256` obrigatório; sem direct grant nem implicit; back-channel logout com sessão obrigatória, front-channel desligado |
| Client `dmpf-bff` | bearer-only, alvo do audience mapper |
| Client `admin-cli` | declarado desligado no realm `dmpf` |
| Client `dmpf-session-events` | confidencial, só service account, `view-events`, secret por `${DMPF_SESSION_EVENTS_SECRET}` obrigatório |
| Escopos | os `clientScopes` completos do export da imagem fixada, com `organization` (nativo, opcional) usando o `Organization Membership` mapper nativo: multivalorado, JSON, atributos ligados, no access token |
| Audiência | audience mapper `dmpf-bff` no access token |
| Fluxo de browser | 2FA condicional com `OTP Form` e `WebAuthn Authenticator`, por "Condition - User Role" (`mfa-required`) |
| Organizações e usuários | nenhum no arquivo; `acme`, `globex`, `alice`, `bob`, `carol` e `dave` entram pelo init, só em local, teste e `dev` |

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Fonte do valor de `tenant_id` | Atributo `tenant_id` da organização | `id` da organização (UUID gerado pelo Keycloak, prende o tenant ao objeto do IdP); alias (nome que um administrador escolhe e o scope expõe na URL) |
| Onde resolver o tenant | No `authn`, por curinga genérico de casamento único sobre a claim nativa, declarado pelo operador em `OIDC_TENANT_CLAIM` como qualquer outro caminho (`README.md:94-95`) | Achatar no Keycloak por provider Java ou script mapper: impõe extensão ao IdP, build Java ao monorepo e, no script mapper, feature Preview; mapper nativo não multivalorado gravando `tenant_id`: emite o alias da primeira organização resolvida, escolha arbitrária com `organization:*` (`OrganizationMembershipMapper.resolveValue`) |
| Escolha do tenant | Pelo scope do pedido de autorização | `organization:*` e escolha no BFF: a escolha viria da requisição, e CTX-06 veda tenant com fonte na entrada (`contexto-erros-seguranca.md:616`) |
| Realm | Único, com Organizations | Um realm por organização: o `authn` aceita um issuer só (`config.go:44`) |
| Profile do Compose | `keycloak`, isolado | Dentro de `dmpf` ou `all`: o teto de recursos do Keycloak estoura o `infra-budget` (`infra/README.md:132`) |
| Provisionamento do realm | `--import-realm` de arquivo versionado | Admin API no boot: mais código, e o import já é idempotente por pular o realm existente |
| Fonte de eventos de sessão | Back-channel logout (`session_ended`) e polling de admin events (`subject_revoked`) | Só back-channel: sem fonte para usuário desabilitado; Event Listener SPI: exige provider próprio, e a interface é interna e pode mudar em qualquer release; SSF: experimental |
| Issuer local | `http://keycloak.localhost:8180` com alias de rede | Entrada no `/etc/hosts`: impõe configuração ao host; frontend `127.0.0.1` com `InsecureIssuerURLContext` no `authn`: muda o `authn` além do curinga |
| Mapper `organization` no realm | `clientScopes` completos com o mapper editado | PUT no mapper pelo init: o init não roda no `hmg`, que ficaria sem tenant; passo de configuração em todo ambiente: exige credencial admin no `hmg` |
| Banco | `dev-file` em local, testes e `dev`; Postgres no `hmg` | Banco `keycloak` no Postgres do `dev`: o Job de bancos só provisiona apps, e um Job próprio duplica o provisionamento; `dev-file` no `hmg`: sem suporte fora de desenvolvimento |
| Imagem | Oficial por digest, com rebuild na partida (~40 s) | Imagem derivada `--optimized`: exige publicar imagem e registry |

## Riscos

| Risco | Mitigação |
| ----- | --------- |
| O back-channel logout não tem reentrega e nenhum mecanismo nativo cobre a expiração a tempo | Entrega best-effort declarada no ADR novo; o polling de admin events cobre o usuário desabilitado, e a SPEC-STEJG347 trata a perda como expiração natural do token |
| O formato da claim `organization` muda numa minor futura | O `resolveValue` é idêntico em 26.8.0, 26.8.1 e `main`; o teste de integração trava o formato na versão fixada, e o upgrade parte de um novo export |
| O realm com os `clientScopes` completos acopla aos defaults da versão fixada | O export é reproduzível e o ADR descreve o passo de upgrade (reexportar e comparar) |
| O `dmpf-go-ci.yml` reutilizável passa a exigir o realm do consumidor externo | Input `keycloak` com padrão `false`; só o `ci.yml` deste repositório o liga |
| O curinga muda a resolução de um caminho já configurado | Nenhuma configuração do repositório usa `*` num segmento; a busca literal continua primeiro, e os testes cobrem `realm_access.roles` e claims com ponto no nome |
| A lista de um item muda um `OIDC_TENANT_CLAIM` que já aponte para uma lista com um único valor: o tenant passa de ausente a presente | Nenhuma configuração do repositório aponta o tenant para uma lista; o README do `authn` e o ADR-064 registram a mudança |
| O import pula o realm existente e a mudança do arquivo não chega a quem já tem o realm | Procedimento de atualização por ambiente no guia e no ADR; o teste de integração roda na infra de testes, sempre em tmpfs |
| Keycloak consome memória e CPU de forma que pesa no ambiente local | O profile `keycloak` sobe só sob demanda e fica fora de `all` e `dmpf` |
| Usuário de teste com senha conhecida chega a um ambiente compartilhado | O arquivo do realm de base não tem usuários de teste, o init só roda em local, teste e `dev`, e um critério de aceite confere o `hmg` |

## Regras relacionadas

- `CTX-06`, `IDN-01`, `IDN-20`, `DAT-01` — `docs/dmpf/contexto-erros-seguranca.md:616,809,930,1425`.
- ADR-054 (infra de testes separada do runtime, item 7) e ADR-060 (consumo externo por versão).
- `libs/backend/go/authn/README.md` — o que o módulo contém e o que não contém (a escolha do IdP).
- SPEC-VJMM2DE5 (layout de infra por app), SPEC-9B6SHEH8 (`ports.Authenticator`) e SPEC-STEJG347 (consome o contrato de evento definido aqui).

## Verificação e testes

### Critérios de aceite

- [x] `docker compose -f infra/local/docker-compose.yml --profile keycloak up -d` deixa o Keycloak saudável com o realm `dmpf` importado, e o `issuer` do discovery é igual ao `OIDC_ISSUER`.
- [x] O token de `alice`, obtido por código com PKCE, passa no `authn.Verifier` com `OIDC_AUDIENCE=dmpf-bff` e `OIDC_TENANT_CLAIM=organization.*.tenant_id`: `Identity.Subject` é o `sub` e `Identity.Tenant` é o atributo de `acme`; a mesma configuração, a partir do host e de um container da rede do Compose, descobre o issuer.
- [x] `bob` obtém dois tokens, com `organization:acme` e com `organization:globex`, cada um com uma organização e `Identity.Tenant` diferente.
- [x] `bob` com `organization:*` e `carol` com qualquer scope recebem token válido, e `Identity.Tenant` é nulo.
- [x] O realm importado não declara provider, script mapper nem SPI próprio, e a mudança no `authn` não acrescenta nada específico de Keycloak ao código de produção.
- [x] Os testes unitários do curinga passam, e a suíte atual do `authn` passa sem alteração de caso existente.
- [x] Nenhum client do realm aceita direct access grant nem implicit flow, e o `dmpf-portal` recusa pedido sem `S256`.
- [x] O login de um usuário com a role `mfa-required` e OTP cadastrado, como o `dave` do seed, não devolve código de autorização antes do segundo fator, e o de `alice` não pede segundo fator.
- [x] O logout de `alice` entrega ao receptor registrado no `dmpf-portal` um logout token verificável (assinatura, `iss`, `aud=dmpf-portal`, `events`) com o `sub` e o `sid` do access token dela, mapeável para `session_ended`.
- [x] Desabilitar um usuário gera um admin event `UPDATE USER` legível pelo `dmpf-session-events`, mapeável para `subject_revoked`.
- [x] O arquivo do realm de base não tem usuário de pessoa (só a service account do `dmpf-session-events`), organização nem segredo literal, e a saída de `kubectl kustomize infra/k8s/overlays/hmg` não contém `alice`, `bob`, `carol` nem `dave`.
- [x] O seed roda de novo sem erro e converge depois de um vínculo de organização e da credencial OTP de `dave` serem removidos.
- [x] `pnpm nx g @mateusmacedo/dmpf-plugin:init --dry-run --no-interactive` não propõe CREATE, UPDATE nem DELETE; `bash tools/infra-budget.sh` passa; `kubectl kustomize infra/k8s/overlays/dev` e `.../hmg` renderizam.
- [ ] O CI deste repositório roda o teste de integração do `authn` contra o Keycloak com o input `keycloak` ligado, e o `dmpf-go-ci.yml` chamado sem o input não sobe o Keycloak.

### Cenários de teste

**Happy path — um tenant**
- DADO `alice`, membro só de `acme`, e o Keycloak de referência no ar
- QUANDO ela conclui o fluxo de código com PKCE pedindo `organization:acme` e o BFF chama `Authenticate`
- ENTÃO a identidade traz o `sub` dela e o tenant de `acme`, vindos do token verificado

**Edge case — usuário em duas organizações**
- DADO `bob`, membro de `acme` e `globex`
- QUANDO ele pede `organization:acme`, e depois `organization:globex` na mesma sessão SSO
- ENTÃO recebe dois tokens, cada um com uma só organização e o `tenant_id` dela; com `organization:*` o token traz as duas organizações, o curinga não casa uma única chave, o tenant fica ausente e a rota que exige tenant responde 403 `tenant-unresolved`

**Erro — usuário sem organização**
- DADO `carol`, sem vínculo com organização
- QUANDO ela se autentica e o BFF resolve a identidade
- ENTÃO o token é válido, `Identity.Tenant` é nulo e nenhum tenant sintético aparece (IDN-20)

**Edge case — realm já importado**
- DADO um volume com o realm `dmpf` de uma versão anterior do arquivo
- QUANDO o Keycloak sobe com o arquivo novo
- ENTÃO o import é pulado, e o guia manda recriar o volume para aplicar a mudança

<critical_constraints>
- [P0] O tenant e o sujeito do `authn` vêm só do token verificado (CTX-06, IDN-01): o mapper e a configuração de referência nunca leem tenant de header, query ou corpo.
- [P0] Tenant ausente é ausência (IDN-20): se a claim `organization` não traz exatamente uma organização com exatamente um valor de `tenant_id`, o `authn` não resolve tenant; nenhum valor sintético nem escolha arbitrária entre organizações.
- [P0] O Keycloak de referência é infra: nenhum módulo Go do kernel passa a importar ou a depender dele em runtime, e o `authn` segue OIDC genérico.
- [P0] O realm de base, importado em todo ambiente, não contém usuário, senha nem organização de teste; esses dados entram só no Compose local, na infra de testes e no overlay `dev`, por um passo de init. O overlay `hmg` recebe os Secrets de fora e nunca carrega credencial do repositório.
- [P0] A infra estática muda pelos templates de `tools/dmpf-plugin/templates/init/`, nunca pela cópia: `init --dry-run` sem diff no CI (`ci.yml:199-207`).
- [P0] O profile `keycloak` fica fora de `all` e `dmpf`: o `infra-budget` não pode regredir.
- [P0] O Keycloak de referência roda sem extensão: nenhum provider, script mapper, SPI próprio nem feature Preview. O `authn` se adapta ao formato nativo das claims por configuração genérica, sem nada específico de Keycloak.
- [P1] Versão e digest fixos em todo manifesto; nenhuma tag móvel.
</critical_constraints>

## Escopo fora

- **Revogação de sessão e deny-list**: é a SPEC-STEJG347; esta spec só define o canal e o contrato do evento.
- **Alta disponibilidade e Keycloak de produção** (réplicas, cache distribuído, TLS de borda, patch de segurança): a base Kustomize é referência de desenvolvimento e homologação, com uma réplica.
- **Federação SSO por organização**: o realm de referência não configura IdP externo.
- **Step-up por ACR/LoA e sessões de 30 min e 8 h**: parâmetros de produto do consumidor, aplicados sobre o realm de referência.
- **Origem das permissões efetivas**: o `authn` lê as claims que o operador declara; mapear roles de domínio é do consumidor.
- **Keycloak na AWS e emulação de IdP no floci**: a base Kustomize é o artefato portável; o provisionamento em nuvem é do consumidor.
- **Tema, e-mail transacional e idioma**: o realm usa os defaults.
- **Extensão do `authn` além do curinga**: a referência prova que o contrato atual, com o curinga, basta; o resto é da SPEC-STEJG347.
- **Provider, script mapper ou SPI próprio no Keycloak**: a referência usa só recursos nativos, e cada consumidor adota o mesmo realm sem build de extensão.
