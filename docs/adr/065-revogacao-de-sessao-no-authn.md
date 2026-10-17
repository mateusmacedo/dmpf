# ADR-065: Revogar a sessão no authn por deny-list no Redis, alimentada por um papel próprio do BFF

## Status

Aceito — 2026-10-17. Implementa [SPEC-STEJG347](../specs/SPEC-STEJG347-revogacao-de-sessao-authn.md). Supersede em parte o item 8 do [ADR-064](./064-keycloak-de-referencia-sem-extensao.md): a frase sobre a entrega best-effort e a deduplicação do polling, e acrescenta uma linha ao contrato do evento de sessão.

## Contexto

O `Verifier` do `authn` confiava na assinatura e na expiração do access token, e a revogação estava delegada à «plataforma» (FND-07). Encerrar a sessão no Keycloak não invalida o token já emitido, então o atraso da revogação era o que restava da vida dele (300 s no realm de referência). O consumidor externo exige revogação efetiva em menos de 60 s.

O ADR-064 deu ao realm de referência duas fontes de eventos de sessão: o back-channel logout no `dmpf-portal` e os admin events lidos pelo `dmpf-session-events`. O back-channel é entregue uma única vez, de forma síncrona, sem reentrega: se o receptor estiver fora, o logout se perde. A SPEC-STEJG347 exige entrega ao menos uma vez e retomada após restart.

Três restrições moldaram a decisão:

- o BFF é «sem banco, outbox nem consumer» desde o ADR-044, e o DEVS-94 (ADR-063) acabara de fixar duas roles de banco por app, só para os contextos;
- `ports.Identity` não carrega o id da sessão, então a consulta precisa acontecer onde as claims verificadas existem;
- o cliente de armazenamento só pode viver em módulo provider próprio (ADR-030, mantido pelo ADR-045), e o kernel chega ao consumidor externo por versão (ADR-060).

## Decisão

1. **Armazenamento: Redis**, no provider novo `libs/backend/go/redis` (unidade `kernel/provider-redis`, em `shared_kernel_units`, para o `bff` a importar sem `DMPF-D002`), sobre `github.com/redis/go-redis/v9` `>=9.23.0 <10`.
   - Chaves `dmpf:revocation:sid:<sha256(sid)>` e `dmpf:revocation:sub:<sha256(sub)>`, em hexadecimal: nem o `sub` nem o `sid` aparecem em chave, valor ou erro.
   - Valor `<revoked_at_ms>:<expires_at_ms>`. A expiração vai no valor porque o `PEXPIRETIME` fica fora da ACL, e o script precisa dela para nunca encurtar uma entrada.
   - `Revoke` roda um script Lua (por `EVALSHA`, com `EVAL` quando o servidor não o conhece) que só regrava a entrada se o `revoked_at` novo for maior, com a maior expiração entre a guardada e a nova, aplicada por `SET ... PXAT`, e devolve se a entrada mudou. `Revoked` faz um único `MGET` das duas chaves, ou só da do sujeito quando o token não tem `sid`.
   - `REDIS_URL` (`redis://` ou `rediss://`) com o banco 0 obrigatório, porque o `SELECT` fica fora da ACL; `REDIS_DIAL_TIMEOUT_MS` (padrão 1000) e `REDIS_COMMAND_TIMEOUT_MS` (padrão 500), recusados acima da maior `time.Duration`; `REDIS_REQUIRE_TLS=true` recusa o `redis://` com `ErrTLSRequired`, e o overlay hmg do BFF o declara, porque o Redis de lá é externo e a URL tem de ser `rediss://`. O erro de configuração não repete a URL, que pode carregar a senha.
   - Servidor `redis:7.4.11-alpine` fixado por digest, com AOF `appendfsync everysec` e `maxmemory-policy noeviction`, e um `maxmemory` explícito abaixo do teto do container (192 MB no Kubernetes, 96 MB no Compose local): sem ele, o `noeviction` não vale antes de o container morrer por memória. Cheio, o Redis recusa a escrita em vez de despejar uma revogação.
   - Presente na infra de testes (porta 16379, tmpfs), no estágio de integração do `dmpf-go-ci.yml`, no Compose local (profile `redis`, também subido pela sessão do `bff:serve`), na base Kustomize `infra/k8s/base/redis` composta só pelo overlay `dev` (StatefulSet com PVC) e nos templates do `init`. No `hmg`, o Redis é externo e chega pelo Secret `redis` (chave `url`).

2. **Consulta no `Verifier`**, depois de assinatura, issuer, audiência, expiração e `sub`, e antes de montar a identidade. A referência leva `sub`, `sid` (vazio se ausente) e `iat`, todos já verificados. `RevocationChecker.Revoked` devolve o `RevocationKind`: o valor zero é «não revogado», e `session` precede `subject` quando as duas entradas negam o token. A entrada por `sid` nega todo token com aquele `sid`; a entrada por `sub` nega os tokens com `iat <= revoked_at`, e um login posterior passa. O token revogado sai com `ErrCredentialRejected` e `dmpf.auth.refusal_reason` = `session_revoked`.

3. **Falha fechada.** Erro ou prazo vencido da consulta (`OIDC_REVOCATION_TIMEOUT_MS`, padrão 250) recusam o token com `ErrCredentialRejected`, em `warn` `auth: revocation store unavailable`, com `error.type` = `TransientDependency` e sem motivo de recusa, como o JWKS indisponível. O erro do armazenamento não é embrulhado no erro devolvido, para não levar dado pessoal ao chamador. Sem checker, o `authn` se comporta como antes: o consumidor externo liga a revogação quando quiser. No BFF, a API exige `REDIS_URL` e a partida falha sem ele, porque a revogação desligada em silêncio anularia o requisito.

4. **CTX-16 e IDN-10.** A consulta acontece no provider que realiza `ports.Authenticator`, antes de a identidade chegar ao application service, e nenhum caso de uso consulta a deny-list (IDN-10). As entradas são fatos do IdP com vida própria, e não decisão derivada de uma execução; o que CTX-16 alcança é a resposta «não revogado», que nunca é guardada: cada `Authenticate` consulta o armazenamento, e o módulo não mantém espelho nem cache de ausência. O teste que aceita um token, aplica a revogação e recusa o mesmo token na requisição seguinte, com o checker consultado nas duas, fecha a análise da spec.

5. **Alimentação e TTL.** `Feed.Apply` recebe um `SessionEvent` já normalizado e grava `session_ended` por `sid` e `subject_revoked` por `sub`, com `revoked_at` igual ao `occurred_at` do evento. A entrada expira em `occurred_at + OIDC_REVOCATION_TTL_SECONDS + OIDC_REVOCATION_SKEW_SECONDS`: o TTL é o maior tempo de vida de token que o IdP emite, obrigatório (sem ele a alimentação não sobe), e a margem tem padrão 60. A reentrega não estende a entrada, o evento fora de ordem não reduz o `revoked_at`, e o evento já expirado é no-op. Cada evento termina em `applied`, `duplicate`, `expired`, `invalid` ou `foreign_issuer`.

6. **Papel `bff session-events`.** A alimentação roda num subcomando da mesma imagem do BFF, com um Deployment de uma réplica, e não dentro da API, que só consulta.
   - Tem configuração própria (`SessionEventsFromEnv`), despachada antes da configuração da API, sem chamar `FromEnv`. As duas leem as mesmas variáveis de infraestrutura (`ADMIN_ADDR`, `REDIS_*`, `DRAIN_DELAY` e os sinais OTel), e no Kubernetes o papel recebe o ConfigMap `bff` por `envFrom`: o `ADMIN_ADDR` do papel é fixado por `env` explícito no Deployment, e qualquer outra variável alterada nesse ConfigMap vale também para o papel.
   - `POST /backchannel-logout` na porta `SESSION_EVENTS_ADDR` (padrão 8091) e `/livez` e `/readyz` na porta `ADMIN_ADDR` (padrão 8092). O `/readyz` não depende do Redis nem do Keycloak.
   - Entre os papéis do BFF, só esse Deployment referencia o Secret `keycloak-session-events`, o client secret do polling (no molde do ADR-063, segredo só no processo que o usa); o Deployment do Keycloak também o referencia, por `secretKeyRef`, para importar o realm (ADR-064, item 9).
   - A NetworkPolicy do papel só admite entrada do Keycloak na porta do receptor. No `dev`, o realm entrega o back-channel em `http://bff-session-events:8091/backchannel-logout`, pelo `DMPF_BACKCHANNEL_LOGOUT_URL` do ConfigMap `keycloak`; no Compose local, pelo `KEYCLOAK_BACKCHANNEL_LOGOUT_URL`.

7. **Adaptadores no `authn`.** O receptor e o polling vivem no `authn`, que já tem o verifier OIDC e o issuer, e chegam ao consumidor externo por versão.
   - **`BackchannelLogout`** aceita só `POST` com o form `logout_token` (até 64 KiB) e o verifica por `VerifyLogout` do go-oidc v3.21.0, com o `ClientID` do client do back-channel (`OIDC_BACKCHANNEL_CLIENT_ID`). Como a biblioteca não confere o `iat`, o receptor exige `iat` presente e no máximo `OIDC_REVOCATION_SKEW_SECONDS` no futuro. O token com `sid` vira `session_ended`; o token só com `sub` vira `subject_revoked`, porque o Back-Channel Logout 1.0 §2.7 manda encerrar todas as sessões do sujeito nesse caso. Responde `200`, `400` (`{"error":"invalid_request"}`) ou `503`, e `405` com `Allow: POST` para qualquer outro método, sempre com `Cache-Control: no-store`.
   - **Desvio consciente do §2.8**, que pede `400` para toda falha: o receptor responde `503` com o armazenamento ou o JWKS fora, para separar a falha transitória do token inválido para quem observa o IdP. O Keycloak não reenvia em nenhum dos dois status, então o desvio não muda a entrega.
   - **`SessionPolling`** lê a Admin REST com client credentials (`golang.org/x/oauth2/clientcredentials`) do client `OIDC_SESSION_EVENTS_CLIENT_ID`, com `view-events`, em duas fontes, os admin events e os eventos de usuário `LOGOUT`. Os eventos `LOGOUT` viram `session_ended`; dos admin events, os de `USER` (`UPDATE` com `updated_enabled=false`, `DELETE` em `users/{id}` e `ACTION` em `users/{id}/logout`) viram `subject_revoked`, e o `DELETE` de `USER_SESSION` em `sessions/{id}` vira `session_ended` pelo id da sessão, que é o `sid` do token. O intervalo é `OIDC_SESSION_EVENTS_INTERVAL_SECONDS` (padrão 5). Uma fonte fora não para a outra, e a falha sai em `warn` com o status HTTP, quando houve resposta; um `401` descarta o token em cache, e a leitura seguinte pede outro.

8. **Entrega: caminho rápido e reconciliação.** O back-channel é o caminho rápido; o polling é a reconciliação do logout do usuário (eventos `LOGOUT`) e do encerramento administrativo que deixa admin event (usuário desabilitado, excluído ou deslogado por `users/{id}/logout`, sessão encerrada por `sessions/{id}`), inclusive do que o back-channel perdeu com o papel fora do ar. O `logout-all` do realm não tem reconciliação (item 16). O polling não guarda cursor: cada ciclo relê a janela inteira `[now − (TTL + margem), now]`, com `dateTo` fixo no ciclo e páginas de `first`/`max` (100) até uma página incompleta, e reaplica todos os eventos. Os ids já vistos na janela só servem para contar e registrar cada evento uma vez, nunca para pular o `Apply`, e saem do mapa quando deixam a janela. Uma janela que passe de 1000 páginas sai em `warn`, porque os eventos mais antigos dela ficariam sem aplicar.

9. **Recuperação de entrada perdida.** Como cada ciclo reaplica a janela, uma entrada que o Redis perca (o último segundo do AOF `everysec` num crash, um failover, o Redis externo do `hmg`) volta no ciclo seguinte. O `Apply` monotônico absorve a duplicata.

10. **Contrato do evento.** O mapeamento por fonte do ADR-064 ganha duas linhas, e a do admin event de `subject_revoked` passa a incluir o `DELETE` do usuário em `users/{id}`:

    | Fonte | `kind` | `event_id` | `issuer` | `subject` | `session_id` | `occurred_at` |
    | ----- | ------ | ---------- | -------- | --------- | ------------ | ------------- |
    | Evento de usuário `LOGOUT` | `session_ended` | id do evento | issuer do realm | `userId` | `sessionId` | `time` |
    | Admin event `DELETE` de `USER_SESSION` em `sessions/{id}` | `session_ended` | id do evento | issuer do realm | — | id da sessão | `time` |

    O logout token só com `sub` é mapeado para `subject_revoked` (§2.7). O adaptador não decodifica `ipAddress` nem `details` do evento `LOGOUT`.

11. **Realm.** `eventsEnabled: true`, `enabledEventTypes: ["LOGOUT"]` e `eventsExpiration: 3600`. A hora de retenção cobre a janela que o polling relê (`TTL + margem`, 360 s no realm de referência), e o teste de ponta a ponta confere essa invariante contra o realm importado. Ligar só o tipo `LOGOUT` e expirar em 1 h limita a retenção do IP que o Keycloak guarda em cada evento (LGPD, minimização). Os admin events seguem com detalhes por 7 dias (ADR-064): cada `UPDATE` de `USER` retém a representação do usuário e o IP de quem agiu, e o adaptador lê só o id, o instante, a operação, o tipo, o caminho do recurso e os `details`. A retenção está declarada no `infra/README.md`.

12. **Atualização do realm em `dev` e `hmg`.** O realm novo e a URL do back-channel só valem depois do procedimento do item 10 do ADR-064: no `dev`, escalar o Deployment do Keycloak a 0, apagar o PVC `keycloak-data` e o Job `keycloak-seed` e reaplicar; no `hmg`, `kc.sh import --override true` com o servidor parado. Sem o reimport, o polling dos admin events segue valendo, mas o `LOGOUT` não é guardado e o logout do usuário depende só do back-channel.

13. **Segurança do Redis.**
    - O usuário `default` fica desligado. O usuário `revocation` não tem nenhum comando por padrão, só alcança `~dmpf:revocation:*` e só usa `HELLO`, `AUTH`, `CLIENT SETINFO`, `PING`, `MGET`, `EVALSHA`, `EVAL`, `GET` e `SET`, a lista que o handshake do go-redis e o script exigem.
    - O `redis-exporter` do Compose local entra com o usuário `exporter`, sem chave nenhuma e só com os comandos de leitura que uma raspagem consulta (`INFO`, `PING`, `CLIENT SETNAME`, `CONFIG GET`, `LATENCY LATEST`, `LATENCY HISTOGRAM`, `SLOWLOG GET`, `SLOWLOG LEN`, `COMMAND INFO`).
    - No Kubernetes, a ACL chega por `--aclfile`, montado da chave `users.acl` do Secret `redis`, que o overlay `dev` gera de `redis-users.acl`; a chave `url` do mesmo Secret leva a URL do usuário da aplicação aos dois papéis do BFF e à sonda de prontidão.
    - A NetworkPolicy do Redis só admite entrada dos pods do BFF. Não há NetworkPolicy de egress, como no resto do repositório.

14. **Logout token repetido.** Não há deduplicação por `jti`: o `Apply` idempotente absorve a repetição, o receptor só aceita tráfego do Keycloak pela NetworkPolicy e o logout token vale 2 minutos.

15. **Métricas sem dado pessoal.** `dmpf.auth.revocation.refusals{kind}`, `dmpf.auth.revocation.store_errors`, `dmpf.auth.revocation.events{outcome}` e o histograma `dmpf.auth.revocation.apply_delay` (s, `now − occurred_at`), pelo `MeterProvider` injetado ou pelo global. Nenhum `sub`, `sid`, token, IP ou senha do Redis vai a atributo, log ou erro.

16. **Perdas aceitas.** Ficam fora da deny-list, cobertos só pela vida do token:
    - o logout que não deixa evento guardado no Keycloak (eventos desligados ou já expirados, reimport pendente) com o back-channel perdido;
    - a expiração da sessão, que não gera evento a tempo;
    - a revogação que chega por SCIM ou por federação sem passar por um evento que o polling lê;
    - o `logout-all` do realm (`POST /admin/realms/{realm}/logout-all`), que o back-channel não entrega (SPEC-SBW7HT6Y) e cujo admin event (`ACTION` de `REALM` em `logout-all`, no Keycloak 26.8.1) não traz sessão nem usuário: reconciliá-lo exigiria um corte global por `iat` no armazenamento, que hoje só tem entradas por `sid` e por `sub`.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Postgres como armazenamento | Contraria o BFF sem banco do ADR-044 e colidiria com as duas roles por app do ADR-063, que valem só para os contextos |
| Tópico Kafka compactado com espelho em memória por réplica | Vedado pela spec, que proíbe espelho com refresh, e sem suporte no provider `kafka` |
| Alimentação dentro da API do BFF | Cada réplica faria o próprio polling, e a API passaria a ser consumer |
| App nova para a alimentação | Custo de projeto, imagem e release sem ganho sobre um papel da mesma imagem |
| Adaptadores no provider `redis` | Mistura canal e armazenamento, e o adaptador não reaproveita o verifier OIDC |
| Adaptadores no BFF | O consumidor externo reescreveria receptor e polling |
| Chave com o `sub` e o `sid` crus | Expõe identificadores pessoais em qualquer leitura do Redis, sem ganho sobre o hash |
| Emendar a spec para back-channel best-effort | O logout se perde inteiro quando o papel está fora do ar |
| Cursor do polling com deduplicação antes do `Apply` | Não regrava a entrada que o Redis perdeu |
| `appendfsync always` | Cobre só o último segundo antes de um crash e não cobre failover nem o Redis externo |
| Deduplicação do logout token por `jti` | Estado novo sem ganho: o `Apply` idempotente já absorve a repetição |
| Responder `400` com o armazenamento fora, como o §2.8 | Confunde a dependência fora com o token inválido para quem observa o IdP, e o Keycloak não reenvia em nenhum dos dois casos |
| NetworkPolicy de egress no papel e no Redis | Quebraria DNS, OIDC e gRPC sem regras novas que o repositório não tem |
| Falha aberta, configurável ou com janela | Anula o requisito de 60 s justamente quando o armazenamento degrada |
| `REDIS_URL` opcional na API do BFF | A revogação desligaria em silêncio numa configuração incompleta |

## Consequências

**Positivas:**

- Um token de sessão encerrada é recusado em menos de 60 s, medido contra o Keycloak de referência: back-channel em cerca de 0,2 s; logout sem receptor, logout administrativo e usuário desabilitado, pelo polling, em cerca de 2 s.
- O logout de usuário chega ao menos uma vez, e uma entrada perdida no Redis volta sozinha no ciclo seguinte.
- O consumidor externo recebe deny-list, receptor e polling por versão, sem nada específico do BFF, e o `authn` sem checker se comporta como antes.
- O Redis da deny-list só alcança as próprias chaves, por comandos enumerados, e nem chave nem métrica carregam dado pessoal.

**Negativas:**

- Com o Redis fora ou lento além de 250 ms, a borda recusa todo token autenticado. **Custo aceito:** a falha fechada é a exigência da spec, e `store_errors` expõe a queda.
- O BFF passa a depender de um Redis em todo ambiente, e a API não sobe sem `REDIS_URL`. **Custo aceito:** é o preço de não desligar a revogação em silêncio.
- O polling relê a janela inteira a cada 5 s na Admin REST e reaplica cada evento no Redis. **Custo aceito:** a janela tem 360 s de eventos `LOGOUT` e administrativos de `USER`, em páginas de 100.
- O IP do usuário fica retido 1 h em cada evento `LOGOUT`, e os admin events retêm dados pessoais por 7 dias. **Custo aceito:** é a menor retenção que cobre a janela, declarada no `infra/README.md`.
- O receptor responde `503` onde o §2.8 pede `400`. **Custo aceito:** o desvio separa falha transitória de token inválido sem mudar a entrega.
- O realm novo exige o reimport destrutivo do ADR-064 em `dev` e manual em `hmg`. **Custo aceito:** sem ele, o back-channel e os admin events seguem valendo.
- Logout sem evento guardado, expiração de sessão e revogação por SCIM ou federação ficam cobertos só pela vida do token (300 s). **Custo aceito:** nenhuma fonte nativa do Keycloak, sem extensão, os entrega.

## Referências

- [SPEC-STEJG347](../specs/SPEC-STEJG347-revogacao-de-sessao-authn.md) (DEVS-101) e [SPEC-SBW7HT6Y](../specs/SPEC-SBW7HT6Y-keycloak-de-referencia.md) (DEVS-97).
- [ADR-030](./030-granularidade-modulo-go-e-bom.md), [ADR-044](./044-bff-rest-e-contextos-grpc-de-referencia.md), [ADR-045](./045-nomes-bare-e-contexto-em-modulo-unico.md), [ADR-060](./060-o-dmpf-tem-consumidor-externo.md), [ADR-063](./063-duas-roles-de-banco-por-app.md) e [ADR-064](./064-keycloak-de-referencia-sem-extensao.md).
- `docs/dmpf/contexto-erros-seguranca.md` — CTX-14 a CTX-16, IDN-01, IDN-06, IDN-10 e DAT-01.
- `libs/backend/go/authn/README.md` — deny-list, `Feed`, receptor, polling e métricas.
- `libs/backend/go/redis/README.md` — chaves, script e ACL exigida.
- `apps/backend/bff/README.md` — o papel `session-events` e as variáveis.
- `infra/README.md` — Redis por ambiente, eventos guardados pelo realm e retenção.
- OpenID Connect Back-Channel Logout 1.0, §2.7 e §2.8.
