---
id: SPEC-SBW7HT6Y
slug: keycloak-de-referencia
title: DMPF — Keycloak de referência na infra do DMPF: realm com Organizations, claim de tenant e eventos de sessão
stage: backlog
priority: P1
depends_on: [SPEC-VJMM2DE5]
ticket_url: https://linear.app/mmda/issue/DEVS-97/spec-sbw7ht6y-dmpf-keycloak-de-referencia-na-infra-do-dmpf-realm-com
subtask_urls: []
created: 2026-10-12
---

# SPEC-SBW7HT6Y: Keycloak de referência na infra do DMPF

## Resumo

Pôr um Keycloak 26.x de referência na infra do DMPF — Compose local, infra de testes e base Kustomize —, com um realm único importado (Organizations, código de autorização com PKCE e segundo fator por OTP ou WebAuthn), um mapper que entrega ao `authn` o tenant ativo como `tenant_id` string e um canal definido de eventos de sessão. O `authn` do kernel assume Keycloak nos defaults, mas nenhum teste dele exercita um Keycloak real, e a infra não oferece um; sem a referência, cada consumidor monta o realm por conta própria e descobre os desencontros em produção. A origem é o consumidor externo antecipa-market, SPEC-GXGASP24 (DEVS-76), que adota Keycloak 26.x autogerenciado, realm único, como IdP. O canal de eventos de sessão definido aqui é o insumo da SPEC-STEJG347 (revogação de sessão no `authn`).

## Contexto

### Problema

- O verificador `libs/backend/go/authn` exige issuer único, audiência e claim de tenant (`config.go:113-126`), e seus `Defaults()` leem um access token de Keycloak (`config.go:53-60`; `README.md:41-43`). A claim de tenant só vale se for string não vazia (`claims.go:37-47`; `oidc.go:93-96`), e a audiência só passa quando o client tem audience mapper (`oidc.go:34-36`, `:53`).
- No Keycloak 26.x, a claim `organization` é um objeto indexado pela organização, não um `tenant_id` string. Um usuário com vínculo em várias organizações precisa de um tenant ativo no token, e o kernel nunca o escolhe sozinho (CTX-06).
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

### Fatos externos (Keycloak, documentação consultada em 12/10/2026)

| Fato | Fonte |
| ---- | ----- |
| Organizations é suportado desde a 26.0 | https://www.keycloak.org/2024/10/keycloak-2600-released |
| A claim `organization` é um objeto por organização; id e atributos só entram com as opções "Add organization id" e "Add organization attributes" do mapper | https://www.keycloak.org/docs/latest/server_admin/index.html#_mapping_organization_claims_ |
| Scope `organization` (uma organização; com várias, o Keycloak pede a escolha no login), `organization:<alias>` (específica; recusa alias inexistente ou sem vínculo) e `organization:*` (todas); misturar formatos é erro | idem |
| Back-channel logout: o Keycloak faz POST de um logout token às URLs registradas, inclusive no logout administrativo ou da conta do usuário; "Backchannel logout session required" inclui o `sid` no logout token | https://www.keycloak.org/docs/latest/server_admin/index.html#backchannel-logout |
| Os listeners nativos são o de log e o de e-mail | https://www.keycloak.org/docs/latest/server_admin/index.html#event-listener |
| O Event Listener SPI é interno e pode mudar em qualquer release; os script providers são Preview e vêm desabilitados | https://www.keycloak.org/docs/latest/server_development/index.html |
| `--import-realm` lê `/opt/keycloak/data/import` e pula o realm que já existe | https://www.keycloak.org/server/importExport |
| `hostname` é obrigatório, aceita URL completa e não é resolvido dinamicamente | https://www.keycloak.org/server/hostname |
| Existem: método PKCE `S256` por client, "Condition - User Role", `OTP Form` e `WebAuthn Authenticator` no fluxo de browser | https://www.keycloak.org/docs/latest/server_admin/index.html |
| A tag `26.8.0` existe e era a mais nova `26.x` em `quay.io/keycloak/keycloak` | https://quay.io/repository/keycloak/keycloak?tab=tags |

### Não verificado

- O formato da claim `organization` antes da 26.8.0: a documentação consultada descreve só a 26.8.0.
- Se existe mapper nativo que achata a claim em string: a documentação lista só as opções de id e de atributos.
- A cobertura do back-channel logout fora do logout explícito (expiração de sessão, usuário desabilitado, revogação em massa) e a política de reentrega.
- Se o SPI de protocol mapper é documentado e estável: o guia de desenvolvimento consultado trata o Event Listener SPI e os script providers.
- A presença de `sid` no access token real do client de referência.
- Como o mesmo issuer fica alcançável do host e de um container Compose.
- Como o overlay `dev` provisiona o banco do Keycloak.

<constraints>
- [P0] O tenant e o sujeito do `authn` vêm só do token verificado (CTX-06, IDN-01): o mapper e a configuração de referência nunca leem tenant de header, query ou corpo.
- [P0] Tenant ausente é ausência (IDN-20): se o token não tem exatamente uma organização ativa, o Keycloak não emite `tenant_id`; nenhum valor sintético nem escolha arbitrária entre organizações.
- [P0] O Keycloak de referência é infra: nenhum módulo Go do kernel passa a importar ou a depender dele em runtime, e o `authn` segue OIDC genérico.
- [P0] O realm de base, importado em todo ambiente, não contém usuário, senha nem organização de teste; esses dados entram só no Compose local, na infra de testes e no overlay `dev`, por um passo de init. O overlay `hmg` recebe os Secrets de fora e nunca carrega credencial do repositório.
- [P0] A infra estática muda pelos templates de `tools/dmpf-plugin/templates/init/`, nunca pela cópia: `init --dry-run` sem diff no CI (`ci.yml:199-207`).
- [P0] O profile `keycloak` fica fora de `all` e `dmpf`: o `infra-budget` não pode regredir.
- [P1] Versão e digest fixos em todo manifesto; nenhuma tag móvel.
- [P1] Nenhuma dependência de feature Preview do Keycloak nem de SPI que a documentação declare instável sem registro do custo de atualização no ADR novo.
</constraints>

## Requisitos

### Funcionais

**A. Infra de referência**

- [ ] **[P0] Investigar a versão exata a fixar**: a documentação mais recente consultada em 12/10/2026 era a 26.8.0 e a tag `26.8.0` era a mais nova `26.x` do registry. Ler as notas de versão de 26.0 a 26.8, registrar o que muda em Organizations, no formato da claim `organization`, no back-channel logout e no import de realm, e fixar versão e digest (mesma disciplina de `infra/test/compose.yml:10,27`) em todos os manifestos.
- [ ] **[P0] Compose local**: criar `infra/local/compose/keycloak.yml` com profile `keycloak`, fora de `all` e `dmpf`; publicar em `127.0.0.1:${KEYCLOAK_PORT:-8180}` (8180 está livre na tabela de `infra/README.md:95-114`); declarar teto e mínimo medidos, healthcheck pelos endpoints de saúde do Keycloak e o realm importado por `--import-realm`.
- [ ] **[P0] Infra de testes**: acrescentar `keycloak` a `infra/test/compose.yml`, em `127.0.0.1:18180`, sem volume e com o mesmo realm, e `KEYCLOAK_URL=http://localhost:18180` ao `.env.example` da raiz.
- [ ] **[P0] Base Kustomize**: criar `infra/keycloak/` com os manifestos e o realm como fonte única — o Compose monta o arquivo por bind e o Kustomize o gera como ConfigMap, como em `infra/observability`. Os overlays `dev` e `hmg`, que o `infrasync` não gera, passam a compor `../../../keycloak`; o `hmg` recebe os Secrets de fora e estende com a chave do Keycloak o `secrets.example.yaml.tmpl` que já tem.
- [ ] **[P0] Investigar o banco do Keycloak na base Kustomize**: a base de Postgres só tem usuário administrativo e o Job de bancos do `dev` nasce dos `infra.json` das apps (`infra/README.md:3,70`). Decidir entre estender o provisionamento com um banco `keycloak` e usar o armazenamento embutido do Keycloak só no `dev`, e registrar a decisão no ADR novo.
- [ ] **[P0] Issuer único e estável**: fixar `hostname` com URL completa e provar que o `iss` emitido é exatamente o `OIDC_ISSUER` e que o `Verifier` o descobre a partir do host (BFF em `bff:serve`) e de um container da rede do Compose. Se a documentação de `hostname` exigir a URL de backchannel separada, ela entra na configuração.
- [ ] **[P0] Infra pelos templates do `init`**: todo arquivo novo entra em `templates/init/managed/` (`infra/local/compose/keycloak.yml`, `infra/test/compose.yml`, `infra/keycloak/**`, `.env.example`), o `include` entra no `seed` `docker-compose.yml__tmpl__` e o `dmpf.rendered.json` é regenerado.
- [ ] **[P1] Consumidores existentes**: o `sync-init` leva os arquivos `managed`; o `include` no `seed` não é reescrito, então o guia de consumo externo documenta o passo manual.

**B. Realm de referência**

- [ ] **[P0] Realm único `dmpf` importado**: Organizations ligadas, sem federação de IdP, sem cadastro aberto e sem usuário nem organização no arquivo.
- [ ] **[P0] Client de usuário `dmpf-portal`**: público, só código de autorização com PKCE `S256` obrigatório; direct access grant e implicit flow desligados em todo client do realm.
- [ ] **[P0] Audiência do `authn`**: audience mapper que inclui `dmpf-bff` no access token, para o `OIDC_AUDIENCE` passar (`oidc.go:34-36`).
- [ ] **[P0] Segundo fator**: fluxo de browser com o sub-fluxo condicional de 2FA habilitado (`OTP Form` e `WebAuthn Authenticator`), exigido pela role de realm `mfa-required` por "Condition - User Role"; o consumidor liga as roles dele a essa.
- [ ] **[P0] Tempo de vida do access token explícito**: `accessTokenLifespan` declarado no realm importado e documentado como o insumo do TTL da deny-list da SPEC-STEJG347.
- [ ] **[P0] Dados de teste fora do realm de base**: um passo de init (`keycloak-init` no Compose local e na infra de testes, no padrão de `redpanda-sasl-init` — `infra/test/compose.yml:74-76` — e um Job no overlay `dev`) cria as organizações `acme` e `globex`, cada uma com o atributo `tenant_id`, e os usuários `alice` (só `acme`), `bob` (as duas), `carol` (nenhuma) e `dave` (só `acme`, com a role `mfa-required`), com credenciais de desenvolvimento. O `hmg` não executa o passo.

**C. Tenant ativo**

- [ ] **[P0] Investigar o mecanismo do achatamento**: comparar (a) provider próprio com protocol mapper OIDC, (b) script mapper e (c) mapper nativo sem código. Critérios: nenhuma dependência de feature Preview, interface estável entre minors 26.x, teste automatizado do mapper e custo de atualização. A decisão e as evidências vão para o ADR novo; se nada atender, o ADR propõe a alternativa e esta spec é emendada antes da implementação.
- [ ] **[P0] Claim `tenant_id` string**: o access token carrega `tenant_id` com o valor do atributo `tenant_id` da organização ativa. Sem a organização, com mais de uma, ou sem o atributo (ou com mais de um valor), a claim não é emitida.
- [ ] **[P0] Tenant ativo por scope**: o client pede `organization:<alias>` para fixar o tenant, ou `organization` para o Keycloak pedir a escolha quando há várias organizações. Trocar de tenant é uma nova autorização com outro alias; o token anterior mantém o tenant antigo até expirar.
- [ ] **[P0] Configuração de referência do `authn`**: documentar, em `infra/README.md` e no README do `authn`, `OIDC_ISSUER`, `OIDC_AUDIENCE=dmpf-bff` e `OIDC_TENANT_CLAIM=tenant_id`, com `OIDC_PERMISSION_CLAIMS` no default (`scope,realm_access.roles`, `config.go:53-60`). O `.env.example` da raiz só ganha `KEYCLOAK_URL`: as variáveis de cada processo vivem em `apps/backend/<app>/deploy/.env.example` (`.env.example:9-10`). Nenhum código do `authn` muda nesta spec.

**D. Eventos de sessão**

- [ ] **[P0] Investigar a fonte de eventos de sessão**: avaliar (1) back-channel logout, (2) provider próprio no Event Listener SPI publicando em Kafka ou HTTP e (3) consulta periódica à Admin REST API. Para cada uma, medir: cobertura de logout do usuário, encerramento administrativo da sessão, expiração da sessão e usuário desabilitado; estabilidade da interface entre minors; garantia e reentrega de entrega; atraso p99 na infra de testes. Registrar a escolha e o que ela não cobre no ADR novo.
- [ ] **[P0] Contrato do evento de sessão**: o canal escolhido entrega `event_id` (único por evento, chave de idempotência), `kind` (`session_ended` com `session_id`, ou `subject_revoked` com `subject`), `issuer`, `subject`, `session_id` (opcional em `subject_revoked`) e `occurred_at`. Nenhum token nem e-mail viaja no evento; `subject` e `session_id` são identificadores pessoais pseudônimos (LGPD art. 13 §4º), classificados como tais por `DAT-01` (`docs/dmpf/contexto-erros-seguranca.md:1425`), com minimização, acesso restrito e retenção declarada, e continuam proibidos em log e métrica. Se o canal for Kafka, o envelope segue o do kernel.
- [ ] **[P0] Origem verificada do evento**: o evento de sessão tem origem verificada. Com back-channel logout, o logout token (JWS) só vale com assinatura, `iss`, `aud` e claim `events` verificados; se o canal for Kafka, só o emissor de eventos tem ACL de produtor no tópico. Quem escreve no canal não pode revogar sessão alheia.
- [ ] **[P0] Emissor no Keycloak de referência**: o realm e o Compose registram o emissor do canal escolhido (por exemplo, a URL de back-channel logout do client), alcançável pela rede do Compose e do cluster.
- [ ] **[P0] `sid` no token**: o access token do `dmpf-portal` carrega `sid`, e o `session_id` do evento é igual a ele; um teste de integração lê o `sid` do token real e o compara ao do evento de logout, porque a recusa em 60 s da SPEC-STEJG347 (P0) depende dele.

**E. Testes**

- [ ] **[P0] Teste de integração do `authn` contra o Keycloak**: obtém os tokens pelo fluxo real de código de autorização com PKCE, seguindo o login por HTTP, sem client de direct grant; lê `KEYCLOAK_URL` por `tb.Env` e verifica `Identity.Subject`, `Identity.Tenant` e a recusa dos casos de erro.
- [ ] **[P1] Target do `authn`**: o `test-race` do `authn` depende de `testkit:test-infra-up`, como os demais alvos de integração (ADR-054, item 7).

**F. Documentação**

- [ ] **[P1] ADR novo**: registra a versão fixada, o mecanismo do achatamento, a fonte de eventos de sessão, o banco da base Kustomize e o que cada alternativa descartada custaria.
- [ ] **[P1] Guias**: `infra/README.md` (profile, porta, variáveis, recriação do realm) e `docs/guides/dmpf-consumo-externo.md` (passo do `include` e variáveis `OIDC_*`).

### Não-funcionais

- **Reprodutibilidade**: o import de realm é determinístico; como o Keycloak pula o realm existente, a mudança no arquivo exige recriar o volume, e o guia diz como.
- **Orçamento**: `tools/infra-budget.sh` continua verde; o profile `keycloak` não entra em `all` nem em `dmpf`.
- **Exposição**: toda porta de host em `127.0.0.1`; porta administrativa fora da rede pública no `hmg`.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Infra | `infra/local/compose/keycloak.yml`, `infra/test/compose.yml`, `infra/keycloak/**` (realm, init dos dados de teste), overlays `dev` e `hmg` |
| Plugin Nx | `tools/dmpf-plugin/templates/init/{managed,seed}`, `dmpf.rendered.json` |
| `authn` | só testes de integração, target e README; sem mudança de código de produção |
| CI | `init --dry-run` sem diff e `infra-budget` verdes |
| Documentação | ADR novo, `infra/README.md`, guia de consumo externo |

## Localização de código

```text
infra/local/compose/keycloak.yml                                novo (profile keycloak)
infra/test/compose.yml                                          serviço keycloak
infra/keycloak/                                                 novo: kustomization, manifestos, realm-dmpf.json, dados de teste e init
infra/k8s/overlays/dev/kustomization.yaml                       compõe ../../../keycloak
infra/k8s/overlays/hmg/                                         Secret de fora + chave do Keycloak em secrets.example.yaml.tmpl
tools/dmpf-plugin/templates/init/managed/infra/...              os mesmos arquivos, como __tmpl__
tools/dmpf-plugin/templates/init/managed/.env.example__tmpl__   KEYCLOAK_URL
tools/dmpf-plugin/templates/init/seed/infra/local/docker-compose.yml__tmpl__   include
dmpf.rendered.json                                              regenerado pelo init
libs/backend/go/authn/                                          teste de integração, project.json, README
docs/adr/                                                       ADR novo
```

## Design

### Fluxo

```text
portal ──código + PKCE──> Keycloak (realm dmpf, Organizations)
  scope=openid organization:<alias>        │
                                           ├── eventos de sessão ──> canal da investigação ──> SPEC-STEJG347
  access token                             ▼
  { iss, aud=dmpf-bff, sub, sid,
    organization: { acme: { id, tenant_id: ["t-acme"] } },
    tenant_id: "t-acme" }
        │ Authorization: Bearer
        ▼
  bff → authn.Verifier (OIDC_ISSUER, OIDC_AUDIENCE, OIDC_TENANT_CLAIM=tenant_id) → ports.Identity
```

### Tenant ativo por pedido

| Scope pedido | Usuário | Claim `organization` | `tenant_id` |
| ------------ | ------- | -------------------- | ----------- |
| `organization:acme` | membro de `acme` | uma entrada | atributo de `acme` |
| `organization` | membro de uma organização | uma entrada | atributo dela |
| `organization` | membro de duas | o Keycloak pede a escolha; uma entrada | atributo da escolhida |
| `organization:*` | membro de duas | duas entradas | ausente |
| `organization:acme` | sem vínculo com `acme` | pedido recusado pelo Keycloak | sem token |
| sem scope `organization` | qualquer | sem claim | ausente |

### Forma do realm importado

| Elemento | Valor de referência |
| -------- | ------------------- |
| Realm | `dmpf`, Organizations ligadas, `accessTokenLifespan` explícito |
| Client `dmpf-portal` | público; código de autorização; PKCE `S256` obrigatório; sem direct grant nem implicit |
| Escopos | `openid`, `organization` (nativo, opcional) com o mapper de achatamento de `tenant_id` |
| Audiência | audience mapper `dmpf-bff` no access token |
| Fluxo de browser | 2FA condicional com `OTP Form` e `WebAuthn Authenticator`, por "Condition - User Role" (`mfa-required`) |
| Organizações e usuários | nenhum no arquivo; `acme`, `globex`, `alice`, `bob`, `carol` e `dave` entram pelo init, só em local, teste e `dev` |

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Fonte do valor de `tenant_id` | Atributo `tenant_id` da organização | `id` da organização (UUID gerado pelo Keycloak, prende o tenant ao objeto do IdP); alias (nome que um administrador escolhe e o scope expõe na URL) |
| Onde achatar | No Keycloak | No `authn`: um caminho de claim com curinga muda o contrato de `claims.go:14-35` e acopla o kernel ao formato de um IdP, que o README trata como configuração do operador (`README.md:94-95`) |
| Escolha do tenant | Pelo scope do pedido de autorização | `organization:*` e escolha no BFF: a escolha viria da requisição, e CTX-06 veda tenant com fonte na entrada (`contexto-erros-seguranca.md:616`) |
| Realm | Único, com Organizations | Um realm por organização: o `authn` aceita um issuer só (`config.go:44`) |
| Profile do Compose | `keycloak`, isolado | Dentro de `dmpf` ou `all`: o teto de recursos do Keycloak estoura o `infra-budget` (`infra/README.md:132`) |
| Provisionamento do realm | `--import-realm` de arquivo versionado | Admin API no boot: mais código, e o import já é idempotente por pular o realm existente |
| Mecanismo do achatamento e fonte de eventos | Decididos por investigação | Script mapper e Event Listener SPI como padrão: Preview e interface interna, segundo o guia de desenvolvimento |

## Riscos

| Risco | Mitigação |
| ----- | --------- |
| O Event Listener SPI é interno e pode quebrar numa minor 26.x (guia de desenvolvimento) | A investigação mede a interface estável (back-channel logout) primeiro; se o provider próprio vencer, o ADR novo registra o custo de cada atualização |
| O formato da claim `organization` mudou entre minors 26.x | A investigação de versão lê as notas de 26.0 a 26.8; o teste de integração trava o formato na versão fixada |
| O import pula o realm existente e a mudança do arquivo não chega a quem já tem o volume | O guia manda recriar o volume; o teste de integração roda na infra de testes, sempre em tmpfs |
| Keycloak consome memória e CPU de forma que pesa no ambiente local | O profile `keycloak` sobe só sob demanda e fica fora de `all` e `dmpf` |
| Usuário de teste com senha conhecida chega a um ambiente compartilhado | O arquivo do realm de base não tem usuários de teste, o init só roda em local, teste e `dev`, e um critério de aceite confere o `hmg` |

## Regras relacionadas

- `CTX-06`, `IDN-01`, `IDN-20`, `DAT-01` — `docs/dmpf/contexto-erros-seguranca.md:616,809,930,1425`.
- ADR-054 (infra de testes separada do runtime, item 7) e ADR-060 (consumo externo por versão).
- `libs/backend/go/authn/README.md` — o que o módulo contém e o que não contém (a escolha do IdP).
- SPEC-VJMM2DE5 (layout de infra por app), SPEC-9B6SHEH8 (`ports.Authenticator`) e SPEC-STEJG347 (consome o contrato de evento definido aqui).

## Verificação e testes

### Critérios de aceite

- [ ] `docker compose -f infra/local/docker-compose.yml --profile keycloak up -d` deixa o Keycloak saudável com o realm `dmpf` importado, e o `issuer` do discovery é igual ao `OIDC_ISSUER`.
- [ ] O token de `alice`, obtido por código com PKCE, passa no `authn.Verifier` com `OIDC_AUDIENCE=dmpf-bff` e `OIDC_TENANT_CLAIM=tenant_id`: `Identity.Subject` é o `sub` e `Identity.Tenant` é o atributo de `acme`; a mesma configuração, a partir do host e de um container da rede do Compose, descobre o issuer.
- [ ] `bob` obtém dois tokens, com `organization:acme` e com `organization:globex`, cada um com uma organização e `tenant_id` diferente.
- [ ] `bob` com `organization:*` e `carol` com qualquer scope recebem token válido sem `tenant_id`, e `Identity.Tenant` é nulo.
- [ ] Nenhum client do realm aceita direct access grant nem implicit flow, e o `dmpf-portal` recusa pedido sem `S256`.
- [ ] O login de `dave`, com a role `mfa-required`, não devolve código de autorização antes do segundo fator, e o de `alice` não pede segundo fator.
- [ ] O logout de `alice` gera, no canal escolhido, um evento com o `sub` e o `sid` do token dela, no contrato definido.
- [ ] O arquivo do realm de base não tem usuários nem organizações, e a saída de `kubectl kustomize infra/k8s/overlays/hmg` não contém `alice`, `bob`, `carol` nem `dave`.
- [ ] `pnpm nx g @mateusmacedo/dmpf-plugin:init --dry-run --no-interactive` não propõe CREATE, UPDATE nem DELETE; `bash tools/infra-budget.sh` passa; `kubectl kustomize infra/k8s/overlays/dev` e `.../hmg` renderizam.

### Cenários de teste

**Happy path — um tenant**
- DADO `alice`, membro só de `acme`, e o Keycloak de referência no ar
- QUANDO ela conclui o fluxo de código com PKCE pedindo `organization:acme` e o BFF chama `Authenticate`
- ENTÃO a identidade traz o `sub` dela e o tenant de `acme`, vindos do token verificado

**Edge case — usuário em duas organizações**
- DADO `bob`, membro de `acme` e `globex`
- QUANDO ele pede `organization:acme`, e depois `organization:globex` na mesma sessão SSO
- ENTÃO recebe dois tokens, cada um com uma só organização e o `tenant_id` dela; com `organization:*` o token não traz `tenant_id` e a rota que exige tenant responde 403 `tenant-unresolved`

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
- [P0] Tenant ausente é ausência (IDN-20): se o token não tem exatamente uma organização ativa, o Keycloak não emite `tenant_id`; nenhum valor sintético nem escolha arbitrária entre organizações.
- [P0] O Keycloak de referência é infra: nenhum módulo Go do kernel passa a importar ou a depender dele em runtime, e o `authn` segue OIDC genérico.
- [P0] O realm de base, importado em todo ambiente, não contém usuário, senha nem organização de teste; esses dados entram só no Compose local, na infra de testes e no overlay `dev`, por um passo de init. O overlay `hmg` recebe os Secrets de fora e nunca carrega credencial do repositório.
- [P0] A infra estática muda pelos templates de `tools/dmpf-plugin/templates/init/`, nunca pela cópia: `init --dry-run` sem diff no CI (`ci.yml:199-207`).
- [P0] O profile `keycloak` fica fora de `all` e `dmpf`: o `infra-budget` não pode regredir.
- [P1] Versão e digest fixos em todo manifesto; nenhuma tag móvel.
- [P1] Nenhuma dependência de feature Preview do Keycloak nem de SPI que a documentação declare instável sem registro do custo de atualização no ADR novo.
</critical_constraints>

## Escopo fora

- **Revogação de sessão e deny-list**: é a SPEC-STEJG347; esta spec só define o canal e o contrato do evento.
- **Alta disponibilidade e Keycloak de produção** (réplicas, cache distribuído, TLS de borda, patch de segurança): a base Kustomize é referência de desenvolvimento e homologação, com uma réplica.
- **Federação SSO por organização**: o realm de referência não configura IdP externo.
- **Step-up por ACR/LoA e sessões de 30 min e 8 h**: parâmetros de produto do consumidor, aplicados sobre o realm de referência.
- **Origem das permissões efetivas**: o `authn` lê as claims que o operador declara; mapear roles de domínio é do consumidor.
- **Keycloak na AWS e emulação de IdP no floci**: a base Kustomize é o artefato portável; o provisionamento em nuvem é do consumidor.
- **Tema, e-mail transacional e idioma**: o realm usa os defaults.
- **Mudança de código de produção no `authn`**: a referência prova que o contrato atual basta; qualquer extensão do `authn` é da SPEC-STEJG347.
