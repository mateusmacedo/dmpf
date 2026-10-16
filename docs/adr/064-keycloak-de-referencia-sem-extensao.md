# ADR-064: Pôr um Keycloak de referência na infra do DMPF, sem extensão no IdP

## Status

Aceito — 2026-10-15. Implementa SPEC-SBW7HT6Y.

## Contexto

O `authn` do kernel lê um access token de Keycloak nos `Defaults()`, mas nenhum teste exercitava um Keycloak real e a infra do DMPF não oferecia um. O consumidor externo antecipa-market adota Keycloak 26.x com Organizations, e a SPEC-STEJG347 (revogação de sessão no `authn`) precisa de uma fonte de eventos de sessão.

Três fatos do Keycloak, lidos no código da tag `26.8.1` e medidos em container, moldaram as decisões:

- A claim nativa `organization` é um objeto indexado pelo alias da organização (`{"acme": {"tenant_id": ["t-acme"]}}`), e o `Organization Membership` mapper não tem opção que a achate em string; no modo não multivalorado ele devolve o alias da primeira organização resolvida, uma escolha arbitrária quando o pedido traz `organization:*`.
- O back-channel logout é entregue uma única vez, de forma síncrona, só a clients com sessão; cobre logout do usuário e encerramento administrativo da sessão, mas não usuário desabilitado nem expiração.
- O go-oidc deriva a URL de discovery do próprio issuer, então o host e um container só aceitam o mesmo token se enxergarem o Keycloak pelo mesmo nome.

A primeira versão da spec exigia achatar `organization` em `tenant_id` dentro do Keycloak, o que só era possível com um provider Java próprio ou um script mapper (feature Preview). O requisito foi revisto: nenhuma imposição pode obrigar o IdP a ser modificado quando um modo compatível com o formato nativo resolve.

## Decisão

1. **Versão**: `quay.io/keycloak/keycloak:26.8.1@sha256:79c623a22abacf3fe6c00a5fb30f5b7843d717b234033b80995891c06116d244`, imagem oficial em modo `start` (`--http-enabled=true`, `--health-enabled=true`, `--cache=local`), sem provider, script mapper, SPI próprio nem feature Preview. O rebuild da imagem oficial na partida custa cerca de 40 s.
2. **Tenant pelo `authn`**: `OIDC_TENANT_CLAIM=organization.*.tenant_id`. O `claimValue` aceita o segmento `*`, que casa exatamente uma chave de objeto; zero ou mais de uma deixam o caminho ausente (IDN-20). O `stringClaim` aceita lista com um único item, porque o atributo de organização chega como lista. O curinga é genérico e vale também nas permissões, sem nada que cite Keycloak. O realm usa o mapper nativo multivalorado, com JSON e atributos ligados.
3. **Realm**: arquivo único `infra/keycloak/dmpf-realm.json`, fonte do Compose (bind) e do Kustomize (ConfigMap). Os 15 `clientScopes` vêm de um export reproduzível da imagem fixada, porque o import não sobrescreve só o mapper `organization` (409), com o mapper editado. Clients: `dmpf-portal` (público, PKCE `S256`, back-channel logout com sessão obrigatória), `dmpf-bff` (bearer-only, alvo do audience mapper), `admin-cli` desligado e `dmpf-session-events` (service account com `view-events`). 2FA condicional pela role `mfa-required`, com OTP e WebAuthn como alternativas. Proteção contra força bruta ligada, com bloqueio temporário. `accessTokenLifespan: 300`; admin events ligados com expiração de 7 dias. A única entrada em `users` é a service account.
4. **Export do realm**: num container descartável da imagem fixada, `kc.sh import --db=dev-file --file min.json` de um realm mínimo (`{"realm":"dmpf","enabled":true,"organizationsEnabled":true}`) e `kc.sh export --db=dev-file --realm dmpf --users same_file --file export.json`; do export entram só `clientScopes`, `defaultDefaultClientScopes` e `defaultOptionalClientScopes`, sem `id`, e o resultado passa pelo Biome antes de virar o template `managed`.
5. **Issuer**: `http://keycloak.localhost:8180` no Compose local, com o alias de rede `keycloak.localhost` no serviço; o nome resolve para loopback no host e para o serviço nos containers (provado com musl, glibc e Go estático). Na infra de testes, `http://127.0.0.1:18180`, cujo único consumidor é o processo de teste. No Kubernetes, `KC_HOSTNAME` vem do ConfigMap obrigatório `keycloak-hostname`: o Component `dev` o gera com `http://keycloak:8080`, e o `hmg` o recebe de fora com o endereço público, como os Secrets; sem ele o pod não sobe, em vez de emitir um issuer interno.
6. **Banco**: `dev-file` no Compose local (volume), na infra de testes (tmpfs) e no overlay `dev` (PVC); Postgres no `hmg`, pelo Secret `keycloak-db`. A base Kustomize é Postgres; o Component `infra/keycloak/dev/` gera o Secret `keycloak-db` com `KC_DB=dev-file`, que prevalece sobre o ConfigMap no `envFrom`.
7. **Dados de teste**: `infra/keycloak/dev/seed.sh`, com o `kcadm.sh` da própria imagem, reconcilia organizações (`acme`, `globex`), perfil e senha de `alice`, `bob` e `carol`, vínculos e role, e recria `dave` a cada execução, porque o segredo OTP não é legível pela Admin API; converge de qualquer estado parcial em cerca de 40 s. Roda no Compose local, na infra de testes (`authn:test-keycloak-up`, só com container novo, que é removido se o seed falhar) e num Job do Component `dev`, cujo ConfigMap não leva hash para o Job, imutável, não quebrar o apply; nunca no `hmg`. Os testes de integração que alteram o realm usam client e usuário descartáveis por execução.
8. **Eventos de sessão**: back-channel logout no `dmpf-portal` e polling de admin events pelo `dmpf-session-events`. O contrato da SPEC-SBW7HT6Y é um mapeamento por fonte:

   | Fonte | `kind` | `event_id` | `issuer` | `subject` | `session_id` | `occurred_at` |
   | ----- | ------ | ---------- | -------- | --------- | ------------ | ------------- |
   | Logout token | `session_ended` | `jti` | `iss` | `sub` | `sid` | `iat` |
   | Admin event (`UPDATE USER` com `enabled` falso, `users/{id}/logout`) | `subject_revoked` | id do evento | issuer do realm | id do usuário | — | `time` |

   A entrega é best-effort: o back-channel não tem reentrega, e o polling é por `dateFrom` com deduplicação por `id`. Normalização, deduplicação e cursor ficam no adaptador da SPEC-STEJG347. A expiração da sessão não gera evento a tempo (`USER_SESSION_DELETED` sai minutos depois) e fica coberta pela vida do token. O logout token vale com assinatura, `iss`, `aud=dmpf-portal` e `events` verificados; o admin event, lido da Admin API do issuer com a credencial do service account.
9. **Segredo do polling**: o realm declara `secret: "${DMPF_SESSION_EVENTS_SECRET}"` sem padrão. Placeholder sem variável fica literal, então a variável é obrigatória: o Compose define o valor de desenvolvimento, e no Kubernetes ela vem do Secret `keycloak-session-events` por `secretKeyRef` não opcional, o que impede o pod de subir sem ele.
10. **Atualização do realm**: o import pula o realm existente. Mudar o arquivo exige recriar o armazenamento no Compose local e na infra de testes; no `dev`, escalar o Deployment para 0, apagar o PVC `keycloak-data` e o Job `keycloak-seed` e reaplicar; no `hmg`, rodar `kc.sh import --override true` com o servidor parado.
11. **Infra de testes e CI**: o Keycloak da infra de testes fica no profile `keycloak`, fora do `test-infra-up`, para o consumidor externo não pagar ~1 GiB e ~60 s por teste que não usa; o `test-race` do `authn` passa ao executor `test-env`, com `-tags=integration` e dependência de `authn:test-keycloak-up`. O `dmpf-go-ci.yml` ganha o input `keycloak` (padrão `false`): com ele ligado e o `authn` entre os afetados, o job sobe o Keycloak por `docker run --network host` (portas 18180 e 19000), semeia, exporta `KEYCLOAK_CALLBACK_HOST=127.0.0.1` (o resto vem do `.env.example` pelo `test-env`) e remove o container no cleanup; só o `ci.yml` deste repositório liga o input.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Achatar `organization` em `tenant_id` por provider Java próprio | Impõe extensão ao IdP, build Java ao monorepo e uma imagem derivada para publicar |
| Script mapper | Feature Preview, desligada por padrão |
| Mapper nativo não multivalorado gravando `tenant_id` | Emite o alias da primeira organização resolvida: escolha arbitrária com `organization:*` |
| `id` da organização como tenant | Prende o tenant a um UUID gerado pelo Keycloak |
| PUT no mapper pelo seed | O seed não roda no `hmg`, que ficaria com a claim padrão e sem tenant |
| Entrada `127.0.0.1 keycloak` no `/etc/hosts` | Impõe configuração ao host de quem desenvolve |
| Frontend `127.0.0.1` e `InsecureIssuerURLContext` no `authn` | Muda o `authn` além do curinga |
| Banco `keycloak` no Postgres do `dev` | O Job de bancos só provisiona apps, e um Job próprio duplicaria o provisionamento |
| `dev-file` no `hmg` | Sem suporte fora de desenvolvimento |
| Imagem derivada `--optimized` | Exige registry e publicação para ganhar cerca de 20 s na partida |
| Só back-channel logout | Não há fonte para usuário desabilitado, e o `subject_revoked` da SPEC-STEJG347 ficaria sem origem |
| Event Listener SPI | Exige provider próprio, e a interface é interna e muda entre releases |
| SSF | Experimental |
| Job de reconciliação do realm no `hmg` | Exige credencial administrativa no `hmg` |
| `requiresUserMembership` no authenticator `organization` | Recusa quem não tem organização, como `carol`, mesmo sem scope de organização |
| Keycloak dentro do `test-infra-up` | Todo consumidor pagaria o Keycloak em qualquer teste de integração, use ou não o `authn` |
| `KC_HOSTNAME` fixo na base e merge no `hmg` | Sem o merge, o `hmg` sobe com um issuer interno em silêncio; o ConfigMap obrigatório falha visível |
| `fullScopeAllowed: false` no `dmpf-portal` | Tiraria as roles de realm do token, que o `realm_access.roles` padrão lê; é decisão do consumidor |

## Consequências

**Positivas:**

- O realm de referência sobe igual no Compose local, na infra de testes e no Kubernetes, sem extensão, e o consumidor o adota sem build.
- O `authn` lê o formato nativo de qualquer emissor que indexe um objeto por chave dinâmica, sem perder IDN-20.
- O teste de integração do `authn` exercita um Keycloak real com PKCE, 2FA, tenants, back-channel logout e admin events, local e no CI.

**Negativas:**

- O realm com os `clientScopes` completos acopla o arquivo aos defaults da `26.8.1`: um upgrade refaz o export e compara.
- A entrega dos eventos é best-effort, e a expiração da sessão fica coberta só pela vida do token (300 s).
- O seed leva cerca de 40 s, porque cada chamada do `kcadm.sh` sobe uma JVM; o `test-infra.sh` só o roda com container novo.
- A lista de um item no `stringClaim` muda o comportamento de um `OIDC_TENANT_CLAIM` que hoje aponte para uma lista com um único valor: o tenant passa de ausente a presente.
- Os overlays `dev` e `hmg` não são templates do `init`, e o consumidor externo acrescenta o Keycloak à mão, assim como o `include` do Compose local.
- **Custo aceito:** mudar o realm em ambiente com estado (`dev`, `hmg`) é um procedimento manual e destrutivo no `dev`.

## Referências

- SPEC-SBW7HT6Y (DEVS-97) e SPEC-STEJG347 (DEVS-101).
- [ADR-030](./030-granularidade-modulo-go-e-bom.md), [ADR-045](./045-nomes-bare-e-contexto-em-modulo-unico.md), [ADR-054](./054-apps-autocontidos-e-infras-separadas.md) e [ADR-060](./060-o-dmpf-tem-consumidor-externo.md).
- `infra/README.md` — profile `keycloak`, portas e procedimento de atualização do realm.
- `libs/backend/go/authn/README.md` — curinga e teste de integração.
