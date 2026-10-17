---
id: SPEC-STEJG347
slug: revogacao-de-sessao-authn
title: DMPF — Revogação de sessão no authn: deny-list por sub/sid alimentada por eventos do IdP
stage: done
priority: P1
depends_on: [SPEC-9B6SHEH8, SPEC-SBW7HT6Y]
ticket_url: https://linear.app/mmda/issue/DEVS-101/spec-stejg347-dmpf-revogacao-de-sessao-no-authn-deny-list-por-subsid
subtask_urls: []
created: 2026-10-12
---

# SPEC-STEJG347: Revogação de sessão no authn

## Resumo

Fazer o `authn` recusar um token cuja sessão foi encerrada ou revogada no IdP em menos de 60 s, sem esperar o token expirar. Uma deny-list por `sub` e por `sid`, cujas entradas expiram em `occurred_at` mais o maior tempo de vida de token mais a margem (`OIDC_REVOCATION_SKEW_SECONDS`), é consultada dentro do `Verifier` (a realização de `ports.Authenticator`) e alimentada pelos eventos de sessão que a SPEC-SBW7HT6Y define. A origem é o consumidor externo antecipa-market, SPEC-GXGASP24 (DEVS-76), que exige revogação efetiva em menos de 60 s e adota Keycloak como IdP. Hoje o `Verifier` confia na assinatura e na expiração, e a revogação está delegada à plataforma.

## Contexto

### Problema

- O `Verifier` checa assinatura, issuer, audiência e expiração pela biblioteca (`oidc.go:53,69`) e exige `sub` (`:78-81`). A revogação de credencial figura como responsabilidade da «plataforma» (`docs/dmpf/contexto-erros-seguranca.md:1346`), e nenhum mecanismo do kernel a cumpre.
- Encerrar a sessão no Keycloak não invalida o access token já emitido: a documentação diz que os tokens em circulação precisam expirar naturalmente (https://www.keycloak.org/docs/latest/server_admin/index.html#signing-out-all-active-sessions). Sem deny-list, o atraso da revogação é o que restava da vida do token.
- `ports.Identity` não carrega o id da sessão (`ports/authn.go:25-29`), então a consulta precisa acontecer onde as claims verificadas existem (`oidc.go:83-87`), e não num decorator sobre `ports.Authenticator`, que teria de decodificar o JWT de novo.
- A borda responde 401 a qualquer erro do `Authenticate` (`libs/backend/go/http/identity.go:37-40`): revogação e indisponibilidade da deny-list são indistinguíveis para o chamador, e a diferença vive no log e na métrica. O precedente é o JWKS indisponível, que recusa com `ErrCredentialRejected` e sai em `warn` com `error.type` = `TransientDependency` (`oidc.go:73-76,119-122`; `README.md:65-69`).
- O Redis da infra existe só no Compose local (`infra/local/compose/redis.yml:2-4`, sem senha — `infra/README.md:118`); a infra de testes e as bases Kustomize não o têm (`infra/test/compose.yml:9,26,49,84`) e nenhum `go.mod` do repositório declara cliente de Redis.

### Estado verificado no repositório

| Ponto | Evidência |
| ----- | --------- |
| Motivos de recusa de hoje: `token_expired`, `audience_mismatch`, `issuer_mismatch`, `signature_invalid`, `malformed` | `oidc.go:100-106` |
| `Config` já injeta dependência por campo de interface (`LoggerProvider`) e imprime a configuração sem segredo | `config.go:43-51,99-108` |
| `ReadEnv` é o ponto único das variáveis `OIDC_*` | `config.go:77-97` |
| Medidor injetável com fallback para o global é o padrão do kernel | `libs/backend/go/app/consumer.go:98`; `app/metrics.go:20-24` |
| O módulo `authn` declara só o go-oidc como dependência externa; `otel/metric` é indireto | `authn/dmpf-units.json:12-18`; `authn/go.mod` |
| SDK de armazenamento fica em módulo provider próprio, com a capability declarada; o `sqs` é o precedente | `libs/backend/go/sqs/dmpf-units.json`; `docs/adr/030-granularidade-modulo-go-e-bom.md:27-30` (um `go.mod` por lib, mantido pelo ADR-045) |
| O BFF monta o autenticador num único ponto | `apps/backend/bff/app/wiring.go:95-100` |
| O repositório mapeia `CEN-42` e `RAS-35` (oráculo de CTX-16) só como linhas da norma, sem teste por ID | `tools/tests/dmpf-verify/profiles.json:678,772` |

### Compatibilidade com CTX-16 e IDN-10 (análise por leitura das regras)

- **CTX-16** (`contexto-erros-seguranca.md:699`) alcança «decisão derivada do contexto — resultado de autorização, permissões efetivas, identidade resolvida», e permite cache com chave de sujeito e tenant e validade que não passe do fim da execução. As entradas da deny-list não são decisão derivada de uma execução: são fatos do IdP («a sessão X terminou»), escritos por eventos fora de qualquer requisição, com vida própria (o TTL do token). A regra não as alcança.
- O que a regra alcança é a **resposta** da consulta: «este token não está revogado» faz parte da identidade resolvida. Guardá-la além da requisição seria cache de decisão derivada sem a validade exigida (CTX-14, CTX-15) e adiaria a revogação pelo tempo da guarda. Por isso a spec veda cache local de ausência.
- **IDN-10** (`:857`) veta consultar a autoridade de identidade dentro do caso de uso. A consulta ocorre no provider que realiza `ports.Authenticator`, antes de a identidade chegar ao `application service`, que segue decidindo sobre permissões já resolvidas.
- **IDN-01** (`:809`): a revogação endurece a segunda condição, «verificada contra a autoridade». A recusa usa `ports.ErrCredentialRejected` (`ports/authn.go:35-37`), distinta de falha de autorização (IDN-06, `:839`).
- **Conclusão**: compatível, desde que a consulta ocorra no provider e nenhuma ausência sobreviva à requisição. É leitura do texto das regras, e não está provada por teste; a investigação do requisito e o cenário «duas requisições seguidas» a fecham.

### Não verificado

- O armazenamento da deny-list: latência de leitura, perda de entradas num restart e consistência entre réplicas.
- O canal de eventos de sessão e o que ele não cobre: é a investigação da SPEC-SBW7HT6Y.
- A presença de `sid` no access token real do Keycloak de referência.
- O custo de um TTL de token curto como único mecanismo (refresh frequente contra o IdP).
- A conclusão de compatibilidade com CTX-16, até o teste do requisito.

<constraints>
- [P0] A deny-list é consultada só no provider que realiza `ports.Authenticator`, depois da verificação criptográfica do token; nenhum `application service` nem `domain` a consulta (IDN-10).
- [P0] Nenhuma resposta «não revogado» nem identidade resolvida sobrevive à requisição que a produziu: sem cache local de ausência (CTX-14, CTX-15, CTX-16).
- [P0] Indisponibilidade da deny-list nunca aceita o token: falha fechada, em categoria de log distinta da recusa por token inválido (IDN-06).
- [P0] A entrada guarda só `sub`, `sid` e instantes, identificadores pessoais pseudônimos (LGPD art. 13 §4º; `DAT-01`) sob minimização, acesso restrito e expiração; nunca token, e-mail ou outro dado pessoal, e o token nunca vai a log.
- [P0] Toda entrada tem expiração: a deny-list não cresce sem limite.
- [P0] O `authn` sem revogação configurada preserva o comportamento atual: `NewVerifier(ctx, cfg)` e as variáveis `OIDC_*` existentes não mudam.
- [P0] Cliente de armazenamento só em módulo provider próprio: `authn`, `ports`, `domain` e `application` não importam SDK de armazenamento (ADR-030: um `go.mod` por lib, mantido pelo ADR-045).
- [P1] A escrita é idempotente e monotônica: reentrega e reordenação do canal não reabrem sessão revogada.
- [P1] Nenhum `sub` nem `sid` em atributo de métrica ou em log.
</constraints>

## Requisitos

### Funcionais

**A. Verificação no `Verifier`**

- [ ] **[P0] Consulta de revogação no `authn`**: declarar `RevocationChecker` (`Revoked(ctx, ref) (RevocationKind, error)`, em que o valor zero é «não revogado» e `session` precede `subject` quando as duas entradas negam o token), recebido por `Config` como `LoggerProvider` (`config.go:50`), e consultá-lo depois de assinatura, issuer, audiência, expiração e `sub`, antes de montar a identidade (`oidc.go:78-97`). A referência leva `sub`, `sid` (claim string; vazio se ausente) e `iat` do token.
- [ ] **[P0] Recusa por revogação**: devolver `ports.ErrCredentialRejected` (`fmt.Errorf("%w: ...")`, como em `oidc.go:76`) e registrar `warn` `auth: token rejected` com `dmpf.auth.refusal_reason` = `session_revoked`, acrescentado aos motivos de `oidc.go:100-106`.
- [ ] **[P0] Entrada por sessão**: uma entrada com `sid` nega todo token que carregue esse `sid` até a entrada expirar.
- [ ] **[P0] Entrada por sujeito**: uma entrada com `sub` e `revoked_at` nega os tokens desse sujeito com `iat <= revoked_at`; um token emitido depois (novo login) passa. A comparação é `<=` para que uma emissão no mesmo segundo da revogação falhe fechada.
- [ ] **[P0] Token sem `sid`**: só a regra por `sub` se aplica; a ausência de `sid` não recusa o token.
- [ ] **[P0] Falha fechada declarada**: erro ou timeout da consulta recusa o token com `ErrCredentialRejected`, registra `warn` `auth: revocation store unavailable` com `error.type` = `TransientDependency` e sem `refusal_reason` (espelho de `oidc.go:119-122`) e conta em `store_errors`. A justificativa está em Decisões técnicas.
- [ ] **[P0] Prazo da consulta**: `OIDC_REVOCATION_TIMEOUT_MS`, default 250, lido por `ReadEnv`; `Validate` recusa checker configurado com prazo não positivo.
- [ ] **[P0] Sem cache de ausência**: cada `Authenticate` consulta o checker; nenhuma resposta é guardada além da chamada, e o módulo não mantém espelho da deny-list com refresh por polling.
- [ ] **[P1] Retrocompatibilidade e visibilidade**: sem checker, nada muda; `Config.LogValue` informa `revocation` como `enabled` ou `disabled` (`config.go:99-108`).

**B. Alimentação e armazenamento**

- [ ] **[P0] Evento de sessão em Go**: decodificar o contrato da SPEC-SBW7HT6Y (`event_id`, `kind`, `issuer`, `subject`, `session_id`, `occurred_at`). Evento malformado, sem a chave que o `kind` exige ou de outro `issuer` é recusado com log e contagem, sem retry infinito.
- [ ] **[P0] Origem verificada do evento**: o consumidor só aplica evento de origem verificada, como a SPEC-SBW7HT6Y exige do emissor: logout token (JWS) com assinatura, `iss`, `aud` e claim `events` verificados ou, se o canal for Kafka, ACL de produtor que só o emissor de eventos tem. Evento de origem não verificada é recusado com log e contagem (`invalid`), e quem escreve no canal não revoga sessão alheia.
- [ ] **[P0] `Apply` idempotente**: gravar a entrada por `sid` (`session_ended`) ou por `sub` (`subject_revoked`); `revoked_at` é o `occurred_at` do evento e a entrada guarda o maior valor visto; a expiração é `occurred_at + TTL + margem` e a reentrega não a estende; evento já expirado é no-op contado.
- [ ] **[P0] TTL**: `OIDC_REVOCATION_TTL_SECONDS` é o maior tempo de vida de token que o IdP emite, e `OIDC_REVOCATION_SKEW_SECONDS` (default 60) é a margem de relógio somada a ele. O operador declara o tempo de token; a SPEC-SBW7HT6Y fixa o do realm de referência. Sem o primeiro valor, a alimentação recusa a partida.
- [ ] **[P0] Adaptador do canal da SPEC-SBW7HT6Y**: ler os eventos do canal escolhido lá e chamar `Apply` só com evento de origem verificada, com entrega ao menos uma vez, retomada após restart e segurança de concorrência. Roda no composition root da app que expõe a borda (`apps/backend/bff/app/wiring.go:95-100`) ou em processo próprio, conforme o ADR novo.
- [ ] **[P0] Investigar o armazenamento da deny-list**: comparar (1) Redis, que existe no Compose local e falta na infra de testes, nas bases Kustomize e nos `go.mod`; (2) Postgres, que o kernel já tem em provider, mas o BFF não declara banco (`apps/backend/bff/deploy/infra.json`); e (3) tópico Kafka compactado com espelho em memória por réplica. Critérios: latência p99 de leitura por requisição, expiração atômica por entrada, perda de entradas num restart e como a alimentação se recupera (persistência ou reprocessamento do canal), consistência entre réplicas do BFF, pegada na infra de testes e no Kustomize, regras de dependência externa e compatibilidade com CTX-16 por candidato. A decisão vai para o ADR novo.
- [ ] **[P0] Provider do armazenamento**: módulo próprio no padrão do `sqs`, nome bare (ADR-045) decidido pela investigação, com a dependência externa e a capability em `dmpf-units.json`; `authn` define as interfaces, e o provider as realiza.
- [ ] **[P0] Fake em memória**: implementação do armazenamento no módulo `authn` para teste e processo único, com relógio injetável.
- [ ] **[P1] Infra do armazenamento escolhido**: o que faltar na infra de testes e nas bases Kustomize entra pelos templates do `init`, como na SPEC-SBW7HT6Y.

**C. Observabilidade**

- [ ] **[P0] Atraso evento → deny-list**: histograma `dmpf.auth.revocation.apply_delay`, em segundos, igual a `now − occurred_at` no `Apply`. Alvo: p99 ≤ 30 s, metade do orçamento de 60 s.
- [ ] **[P0] Contadores**: `dmpf.auth.revocation.refusals` (atributo `kind`: `session` ou `subject`), `dmpf.auth.revocation.store_errors` e `dmpf.auth.revocation.events` (atributo `outcome`: `applied`, `duplicate`, `expired`, `invalid`, `foreign_issuer`).
- [ ] **[P0] Medidor injetável**: `MeterProvider` por campo, com fallback para o global (`app/consumer.go:98`; `app/metrics.go:20-24`); `go.opentelemetry.io/otel/metric` entra em `dmpf-units.json` com a capability `observability`, como no `sqs`, e o `modsync --write` o promove a direto no `go.mod`.

**D. Verificação e documentação**

- [ ] **[P0] Teste de ponta a ponta com o Keycloak da SPEC-SBW7HT6Y**: login com PKCE, `Authenticate` aceita, a sessão é encerrada (logout do usuário num caso, Admin REST noutro) e o mesmo token é recusado em menos de 60 s; o teste mede e falha se chegar a 60 s.
- [ ] **[P0] Investigar a compatibilidade com CTX-16**: confirmar por teste que duas requisições seguidas com o mesmo token, com a revogação entre elas, dão aceite e depois recusa, e que nenhum caminho do módulo memoriza ausência; registrar a conclusão no ADR novo. Se o teste achar resposta guardada além da requisição, corrigir o código, não a regra.
- [ ] **[P1] ADR novo**: armazenamento, falha fechada, TTL, onde roda a alimentação e a análise de CTX-16 e IDN-10.
- [ ] **[P1] Documentação**: README do `authn` (deny-list, `OIDC_REVOCATION_*`) e `docs/guides/dmpf-consumo-externo.md`.

### Não-funcionais

- **Custo por requisição**: no máximo uma consulta ao armazenamento por requisição autenticada, e nenhuma para token recusado antes pela verificação.
- **Idempotência**: aplicar o mesmo evento N vezes, em qualquer ordem, produz o mesmo estado.
- **Compatibilidade**: Go `1.26.9` (`authn/go.mod`), go-oidc `v3.21.0`; sem revogação configurada o módulo se comporta como hoje.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| `authn` (provider) | `Verifier`, `Config`, `ReadEnv`, `Feed`/`Apply`, `MemoryStore`, métricas, receptor do back-channel logout e polling da Admin REST |
| Provider `redis` | módulo novo, no padrão do `sqs`: `Config`/`FromEnv`, `NewClient` e `RevocationStore`; unidade `kernel/provider-redis` em `shared_kernel_units` |
| Composition root do BFF | a API monta o checker em `apps/backend/bff/app/wiring.go`; o papel `session-events` monta o `Feed` e os adaptadores em `apps/backend/bff/app/sessionevents.go`, despachado por `cmd/main.go` |
| Infra | Redis na infra de testes, no CI, no Compose local e na base Kustomize `infra/k8s/base/redis` (só `dev`; externo no `hmg`), realm com os eventos `LOGOUT`, URL do back-channel apontando para o papel; tudo pelos templates do `init` |
| Documentação | ADR-065, addendum no ADR-064, READMEs do `authn`, do `redis`, do BFF e da infra, guia de consumo externo |

## Localização de código

```text
libs/backend/go/authn/oidc.go                       consulta depois do sub, motivo session_revoked, falha fechada
libs/backend/go/authn/config.go                     Revocations, RevocationTimeout, MeterProvider, OIDC_REVOCATION_TIMEOUT_MS
libs/backend/go/authn/revocation.go                 SessionRef, RevocationKind, RevocationChecker, Revocation, RevocationStore
libs/backend/go/authn/revocation_memory.go          MemoryStore com relógio injetável
libs/backend/go/authn/revocation_apply.go           SessionEvent, FeedConfig, ReadRevocationFeedEnv, Feed (Apply, Record)
libs/backend/go/authn/revocation_metrics.go         refusals, store_errors, events, apply_delay
libs/backend/go/authn/backchannel.go                BackchannelLogout (http.Handler)
libs/backend/go/authn/sessionpolling.go             SessionPolling (admin events e eventos LOGOUT)
libs/backend/go/authn/dmpf-units.json               x/oauth2 e otel/metric diretos
libs/backend/go/redis/                              módulo novo: config.go, revocation.go (Lua + MGET)
apps/backend/bff/app/wiring.go                      checker da API sobre o redis.RevocationStore
apps/backend/bff/app/sessionevents{,_config}.go     papel session-events
apps/backend/bff/cmd/main.go                        despacho do subcomando session-events
apps/backend/bff/deploy/                            Deployment, Service e NetworkPolicy do papel, Compose, .env.session-events.example
infra/k8s/base/redis/, infra/test/compose.yml       Redis com ACL, AOF e noeviction
infra/keycloak/dmpf-realm.json                      eventos de usuário LOGOUT por 1 h
docs/adr/065-revogacao-de-sessao-no-authn.md        ADR novo
```

## Design

### Fluxo

```text
Keycloak ──evento de sessão (canal da SPEC-SBW7HT6Y)──> adaptador ──> Apply ──> RevocationStore
                                                                                    ▲
requisição ──Bearer──> Verifier.Authenticate                                        │ Revoked(sub, sid, iat)
   1. assinatura, issuer, audiência, expiração   (go-oidc)                          │
   2. sub presente                                                                  │
   3. deny-list ────────────────────────────────────────────────────────────────────┘
   4. tenant e permissões das claims → ports.Identity
```

A consulta vem depois das checagens locais para que token forjado não gere leitura no armazenamento e para que só claims verificadas sejam usadas.

### Contratos (esboço; os nomes finais saem da implementação)

```go
type SessionRef struct {
	Subject  ports.SubjectID
	ID       string
	IssuedAt time.Time
}

type RevocationKind string

const (
	RevocationSession RevocationKind = "session"
	RevocationSubject RevocationKind = "subject"
)

type RevocationChecker interface {
	Revoked(ctx context.Context, ref SessionRef) (RevocationKind, error)
}

type Revocation struct {
	EventID    string
	Kind       RevocationKind
	Subject    ports.SubjectID
	SessionID  string
	OccurredAt time.Time
	ExpiresAt  time.Time
}

type RevocationStore interface {
	RevocationChecker
	Revoke(ctx context.Context, r Revocation) (bool, error)
}
```

### Semântica das entradas

| Entrada | Chave | Valor | Nega o token quando |
| ------- | ----- | ----- | ------------------- |
| Sessão | `sid` | `revoked_at` | o token tem o mesmo `sid` |
| Sujeito | `sub` | maior `revoked_at` visto | o token tem o mesmo `sub` e `iat <= revoked_at` |

Ambas expiram em `occurred_at + TTL + margem`. O token mais longevo possível, emitido logo antes do evento, expira antes disso.

### Resultado de cada situação

| Situação | Resultado | Log | Métrica |
| -------- | --------- | --- | ------- |
| Não revogado | aceita | nenhum | nenhuma |
| Revogado | `ErrCredentialRejected`, motivo `session_revoked` | `warn` `auth: token rejected` | `refusals{kind}` |
| Armazenamento indisponível ou lento | `ErrCredentialRejected` (falha fechada) | `warn` `auth: revocation store unavailable`, `TransientDependency` | `store_errors` |
| Revogação não configurada | comportamento de hoje | `revocation=disabled` na partida | nenhuma |

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Chave da entrada | `sid` e `sub` com `revoked_at` | `jti` por token: o IdP não lista os tokens da sessão no evento; lista só por `sub` sem `revoked_at`: nega o novo login até o TTL expirar |
| Onde consultar | Dentro do `Verifier`, com as claims verificadas | Decorator sobre `ports.Authenticator`: `Identity` não tem `sid` (`ports/authn.go:25-29`) e o JWT seria decodificado duas vezes; consulta no `application service`: viola IDN-10 |
| Indisponibilidade | Falha fechada | Falha aberta: anula o requisito de 60 s justamente quando o pipeline degrada; aberta com janela limitada: mantém a brecha pelo tamanho da janela. O custo é a borda recusar tudo enquanto o armazenamento cai, igual ao JWKS indisponível (`oidc.go:73-76`) |
| Resposta da consulta | Nunca guardada | Cache curto de «não revogado»: é cache de decisão derivada sem a validade de CTX-16 e soma seu TTL ao atraso da revogação |
| TTL | `occurred_at + maior vida de token + margem` | Sem TTL: cresce sem limite; TTL contado do `Apply`: cada reentrega estenderia a entrada |
| Revogação por IdP | Deny-list alimentada por evento | TTL de token ≤ 60 s como único mecanismo: troca o problema por refresh a cada minuto contra o IdP, custo não medido; introspecção por requisição: acopla a disponibilidade da borda à do IdP |
| Armazenamento | Redis, no provider `libs/backend/go/redis` (go-redis v9.23.0): chaves `dmpf:revocation:{sid,sub}:<sha256>`, valor `<revoked_at_ms>:<expires_at_ms>`, script Lua monotônico com `SET ... PXAT`, um `MGET` por consulta, banco 0, AOF `everysec`, `maxmemory` com `noeviction` e usuário ACL por lista de comandos (ADR-065) | Postgres: contraria o BFF sem banco (ADR-044) e as duas roles por app (ADR-063); Kafka compactado com espelho por réplica: vedado pelo «sem espelho» desta spec e sem suporte no provider |
| Onde roda a alimentação | Papel `bff session-events`, subcomando da mesma imagem, uma réplica, com configuração própria (`SessionEventsFromEnv`) e portas 8091 (receptor) e 8092 (administração) | Dentro da API: cada réplica faria o polling e a API viraria consumer; app nova: custo de projeto e release |
| Onde vivem os adaptadores | No `authn` (`backchannel.go`, `sessionpolling.go`), que já tem o verifier OIDC e o issuer e chega ao consumidor externo por versão | No provider `redis`: mistura canal e armazenamento; no BFF: o consumidor externo reescreveria |
| Entrega ao menos uma vez | Back-channel como caminho rápido e polling dos eventos de usuário `LOGOUT` e dos admin events de encerramento (usuário desabilitado, excluído ou deslogado; sessão encerrada por `sessions/{id}`) como reconciliação, sem cursor: cada ciclo relê a janela `[now − (TTL + margem), now]` e reaplica tudo, o que também regrava a entrada que o Redis perder | Back-channel best-effort: o logout se perde com o receptor fora; cursor com deduplicação antes do `Apply`: não regrava o que o Redis perdeu; `appendfsync always`: cobre só o último segundo antes de um crash |
| Logout token | `VerifyLogout` do go-oidc com o `ClientID` do back-channel, mais `iat` presente e no máximo a margem no futuro; só `sub` revoga o sujeito (§2.7); `503` com o store ou o JWKS fora, desvio consciente do §2.8 | Deduplicação por `jti`: o `Apply` idempotente já absorve a repetição; `400` para toda falha: confunde dependência fora com token inválido |
| `REDIS_URL` no BFF | Obrigatório na API e no papel; o `authn` mantém o checker opcional para o consumidor externo | Opcional no BFF: a revogação desligaria em silêncio |

## Riscos

| Risco | Mitigação |
| ----- | --------- |
| O armazenamento reinicia e perde as entradas, e tokens revogados voltam a valer até expirar | A investigação exige persistência ou reprocessamento do canal na retomada; a métrica de atraso e o teste de restart expõem a perda |
| O canal perde um evento | O limite volta a ser a expiração natural do token; a SPEC-SBW7HT6Y registra o que o canal não cobre e o ADR novo declara |
| Relógio do IdP diverge do verificador | `iat` e `occurred_at` vêm do mesmo IdP; a margem de 60 s no TTL absorve a diferença, e `<=` falha fechada |
| A falha fechada derruba a borda inteira com o armazenamento fora | Aceito e documentado; o armazenamento entra no painel de dependências e na leitura de `store_errors` |
| `sid` ausente em algum client | A regra por `sub` cobre; a SPEC-SBW7HT6Y verifica o `sid` do client de referência |

## Regras relacionadas

- `CTX-14`, `CTX-15`, `CTX-16`, `IDN-01`, `IDN-06`, `IDN-10`, `DAT-01` — `docs/dmpf/contexto-erros-seguranca.md:697-699,809,839,857,1425`; a revogação delegada à plataforma está em `:1346`.
- ADR-030 (dependência externa em módulo próprio), ADR-045 (nomes bare), ADR-054 (infra de testes separada).
- SPEC-9B6SHEH8 (`ports.Authenticator`), SPEC-SBW7HT6Y (Keycloak de referência e contrato do evento de sessão).

## Verificação e testes

### Critérios de aceite

- [x] Um token de sessão revogada é recusado em menos de 60 s após o evento, num teste com o Keycloak de referência, tanto no logout do usuário quanto no encerramento administrativo da sessão.
- [x] Com o armazenamento indisponível, `Authenticate` recusa o token válido com `ErrCredentialRejected`, o log traz `TransientDependency` sem `refusal_reason`, e a volta do armazenamento restabelece o aceite sem reiniciar o processo.
- [x] Um token com `iat` posterior ao `revoked_at` do `sub` é aceito; um com `iat` igual ou anterior é recusado.
- [x] Aplicar o mesmo evento duas vezes, ou em ordem inversa a outro mais novo do mesmo `sub`, deixa o mesmo estado e não reduz `revoked_at`.
- [x] Sem checker configurado, a suíte atual do `authn` passa sem alteração além do atributo `revocation` do `LogValue`, que o requisito P1 exige.
- [x] Duas requisições seguidas com o mesmo token, com a revogação entre elas, dão aceite e depois recusa.
- [x] `dmpf.auth.revocation.apply_delay` aparece com o atraso de cada evento; nenhum atributo ou log traz `sub`, `sid` ou token.
- [x] `modsync --check`, `lint`, `vet`, `test-race` do `authn` e o `conformance` passam.

### Cenários de teste

**Happy path — logout derruba a sessão**
- DADO um token válido de `alice`, aceito pelo `Verifier`
- QUANDO ela encerra a sessão no Keycloak e o evento `session_ended` chega ao `Apply`
- ENTÃO o mesmo token é recusado com `session_revoked`, e o atraso evento → deny-list fica registrado

**Edge case — revogação do sujeito e novo login**
- DADO um evento `subject_revoked` de `alice` com `revoked_at = T`
- QUANDO chegam um token com `iat <= T` e outro emitido depois de `T`, ambos de `alice`
- ENTÃO o primeiro é recusado e o segundo é aceito

**Edge case — reentrega e desordem**
- DADO o mesmo evento entregue duas vezes e um evento mais novo do mesmo `sub` entregue antes de um mais velho
- QUANDO o `Apply` processa todos
- ENTÃO o estado final é o do evento mais novo, e as reentregas contam `duplicate` sem mudar nada

**Edge case — token sem `sid`**
- DADO um token válido sem a claim `sid` e uma entrada por `sub` posterior ao `iat`
- QUANDO ele é verificado
- ENTÃO é recusado pela regra do sujeito; sem a entrada, é aceito

**Erro — armazenamento indisponível**
- DADO o armazenamento da deny-list fora do ar
- QUANDO chega uma requisição com token válido e não revogado
- ENTÃO é recusada (falha fechada), o log traz `auth: revocation store unavailable` e `store_errors` sobe; com o armazenamento de volta, o próximo pedido é aceito

**Erro — ausência não memorizada (CTX-16)**
- DADO duas requisições seguidas com o mesmo token
- QUANDO a revogação é aplicada entre a primeira e a segunda
- ENTÃO a primeira é aceita e a segunda é recusada, sem nenhuma resposta guardada entre elas

<critical_constraints>
- [P0] A deny-list é consultada só no provider que realiza `ports.Authenticator`, depois da verificação criptográfica do token; nenhum `application service` nem `domain` a consulta (IDN-10).
- [P0] Nenhuma resposta «não revogado» nem identidade resolvida sobrevive à requisição que a produziu: sem cache local de ausência (CTX-14, CTX-15, CTX-16).
- [P0] Indisponibilidade da deny-list nunca aceita o token: falha fechada, em categoria de log distinta da recusa por token inválido (IDN-06).
- [P0] A entrada guarda só `sub`, `sid` e instantes, identificadores pessoais pseudônimos (LGPD art. 13 §4º; `DAT-01`) sob minimização, acesso restrito e expiração; nunca token, e-mail ou outro dado pessoal, e o token nunca vai a log.
- [P0] Toda entrada tem expiração: a deny-list não cresce sem limite.
- [P0] O `authn` sem revogação configurada preserva o comportamento atual: `NewVerifier(ctx, cfg)` e as variáveis `OIDC_*` existentes não mudam.
- [P0] Cliente de armazenamento só em módulo provider próprio: `authn`, `ports`, `domain` e `application` não importam SDK de armazenamento (ADR-030: um `go.mod` por lib, mantido pelo ADR-045).
- [P1] A escrita é idempotente e monotônica: reentrega e reordenação do canal não reabrem sessão revogada.
- [P1] Nenhum `sub` nem `sid` em atributo de métrica ou em log.
</critical_constraints>

## Escopo fora

- **Escolher o canal de eventos de sessão e seu emissor no Keycloak**: é a SPEC-SBW7HT6Y; esta spec consome o contrato. Ressalva: para cumprir a entrega ao menos uma vez, esta spec acrescenta a leitura dos eventos de usuário `LOGOUT` pelo mesmo client de polling, com uma linha nova no contrato e o realm guardando esse tipo de evento (ADR-065).
- **`logout-all` do realm**: o back-channel não o entrega, e o admin event dele (`ACTION` de `REALM` em `logout-all`) não traz sessão nem usuário; a deny-list só tem entradas por `sid` e por `sub`, sem corte global por `iat`, e o caso fica coberto pela vida do token (ADR-065, perdas aceitas).
- **Falha aberta configurável**: a revogação em 60 s deixaria de valer justamente na degradação; um consumidor que precise dela emenda esta spec com um ADR.
- **Revogação de refresh token e de sessão offline**: é do IdP; a deny-list cobre o access token que chega à borda.
- **Revogação por mudança de pertencimento a organização ou de permissão**: exigiria evento próprio do IdP; a deny-list por `sub` cobre o caso de revogar o sujeito inteiro.
- **Deny-list por `jti`**: o IdP não lista os tokens da sessão no evento.
- **Introspecção de token por requisição**: acopla a borda ao IdP.
- **Painel e alerta de atraso**: as métricas ficam disponíveis; o dashboard é do consumidor ou de spec própria.
- **Autenticação de workload entre serviços**: mTLS no gRPC e SASL no Kafka seguem o ADR-052, sem relação com token de usuário.
