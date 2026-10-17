# redis

Bloco `provider` do kernel DMPF que guarda a deny-list de sessões do `authn` no
Redis, sobre `go-redis`. Realiza `authn.RevocationStore`: uma chave por sessão
ou por sujeito revogado, que expira no instante em que a revogação deixa de
importar.

Criado pela `docs/specs/SPEC-STEJG347-revogacao-de-sessao-authn.md` (DEVS-101).

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/provider-redis` | `provider` | raiz do módulo |

Dependência externa declarada (`io.storage`): `github.com/redis/go-redis/v9`
(`>=9.23.0 <10`, só o package raiz). A unidade está em `shared_kernel_units`
do `tools/dmpf-baseline/units-baseline.json`, para que um bounded context, como
o `bff`, a importe sem `DMPF-D002`.

## O que o módulo contém

- **`config.go`** — `FromEnv(lookup)` lê `REDIS_URL` por `redis.ParseURL`
  (`redis://` ou `rediss://`), `REDIS_DIAL_TIMEOUT_MS` (padrão 1000) e
  `REDIS_COMMAND_TIMEOUT_MS` (padrão 500, aplicado à leitura e à escrita),
  `REDIS_REQUIRE_TLS` (padrão `false`) e
  liga `ContextTimeoutEnabled`, para que o prazo do chamador limite cada
  comando. Os prazos vêm só dessas variáveis e substituem os que a query da URL
  declarar. Recusa:
  - `REDIS_URL` ausente (`ErrMissingURL`);
  - URL que o go-redis não aceita (`ErrInvalidURL`), sem repetir a URL nem o
    erro original, que podem carregar a senha;
  - banco diferente de 0 (`ErrDatabaseNotZero`), porque o `SELECT` está fora
    da ACL;
  - prazo não positivo ou acima da maior `time.Duration`, e `REDIS_REQUIRE_TLS`
    que não é booleano (`envconfig.ErrInvalidVariable`);
  - `redis://` com `REDIS_REQUIRE_TLS=true` (`ErrTLSRequired`). É a exigência
    de TLS fora do desenvolvimento: o overlay hmg do BFF declara `true`, e a
    URL do Redis externo tem de ser `rediss://`.

  `Config.LogValue` registra `addr`, `username`, `tls`, `dial_timeout` e
  `command_timeout`, nunca a senha. `NewClient(cfg)` valida e cria o cliente
  sem tocar o servidor: a primeira chamada é que conecta.
- **`revocation.go`** — `RevocationStore`, criado por
  `NewRevocationStore(client, clock)`. O `client` é qualquer
  `redis.Scripter` do go-redis com `MGet`; o `*redis.Client` atende.
  - **Chaves:** `dmpf:revocation:sid:<sha256(sid)>` e
    `dmpf:revocation:sub:<sha256(sub)>`, em hexadecimal. Nem o `sub` nem o
    `sid` aparecem em chave, valor ou erro.
  - **Valor:** `<revoked_at_ms>:<expires_at_ms>`. A expiração vai no valor
    porque o `PEXPIRETIME` está fora da ACL.
  - **`Revoke`:** recusa a entrada sem a chave que o `kind` exige
    (`authn.ErrInvalidRevocation`), devolve `false` sem tocar o servidor
    quando `ExpiresAt` já passou pelo relógio injetado, e roda um script Lua
    que lê a entrada e só a regrava se o `revoked_at` novo for maior. Na
    regravação, a expiração é a maior entre a guardada e a nova, aplicada por
    `SET ... PXAT`. Devolve se a entrada mudou: reentrega e evento fora de
    ordem devolvem `false` e nunca reduzem o `revoked_at`. Uma entrada
    ilegível não é regravada: o script recusa e o `Revoke` devolve
    `ErrUnreadableEntry`. O `false` é aproximado sob retry do go-redis: se a
    resposta do script se perde num read timeout e o cliente repete o
    comando, a repetição encontra a própria escrita e devolve `false`. A
    entrada fica correta; só a métrica do `authn` conta o evento como
    duplicado. O script roda por
    `EVALSHA`, com `EVAL` quando o servidor ainda não o conhece; não há
    `SCRIPT LOAD`.
  - **`Revoked`:** um `MGET` das duas chaves, ou só da do sujeito quando o
    token não tem `sid`. A entrada de sessão vence a de sujeito. A de sujeito
    nega o token com `iat <= revoked_at`, comparados em milissegundos. Uma
    entrada ilegível devolve `ErrUnreadableEntry`, nunca «não revogado».
  - **Erros:** do servidor ou da rede voltam embrulhados
    (`redis: revocation lookup` e `redis: revocation write`), e o `authn`
    trata como store fora, com falha fechada.

## ACL exigida

O usuário do provider não tem nenhum comando por padrão e só alcança chaves
sob o prefixo. É a ACL da infra de testes (`infra/test/compose.yml`), do
Secret do overlay `dev` (`infra/k8s/overlays/dev/redis-users.acl`, montado por
`infra/k8s/base/redis/`) e do Compose local (`infra/local/compose/redis.yml`):

```text
user default off
user revocation on >SENHA ~dmpf:revocation:* -@all +hello +auth +client|setinfo +ping +mget +evalsha +eval +get +set
```

- `HELLO`, `AUTH` e `CLIENT SETINFO` são o handshake do go-redis; `PING`, a
  prontidão.
- `MGET` é a consulta; `EVALSHA` e `EVAL` rodam o script, que chama `GET` e
  `SET`.
- O servidor precisa de `maxmemory-policy noeviction`: uma revogação despejada
  por memória reabriria a sessão. A durabilidade esperada é AOF com
  `appendfsync everysec`.

## O que o módulo não contém

O provisionamento do Redis e da ACL, que vive em `infra/`; a alimentação da
deny-list (o `Feed`/`Apply` e os adaptadores do canal, no `authn`); cache
local de ausência; leitura em réplica; Redis Cluster ou Sentinel.

## Como rodar os testes localmente

Os unitários cobrem a configuração, a recusa de entrada sem chave, a entrada já
expirada e o servidor inalcançável, sem Redis. O `revocation_test.go` também
afirma, em tempo de compilação, que `*RevocationStore` satisfaz
`authn.RevocationStore`.

Os testes de integração levam a build tag `integration` e exigem `REDIS_URL`
(sem ela fazem `t.Skip` nomeando-a). Usam o usuário ACL `revocation` contra o
Redis da infra de testes em `127.0.0.1:16379` e cobrem:

- sessão;
- sujeito com `iat` anterior, igual e posterior;
- as duas entradas, com a sessão vencendo;
- reentrega e desordem;
- expiração por `PXAT` e preservação da expiração mais longa;
- chaves por hash;
- entrada ilegível;
- `NOPERM` para chave fora do prefixo, `CONFIG` e `FLUSHALL`.

As chaves levam sufixo único por execução e expiram sozinhas, já que o usuário
não pode apagar nada.

O target sobe a infra de testes (`testkit:test-infra-up`) e o
`tools/dmpf-plugin/scripts/test-env.sh` preenche `REDIS_URL` a partir do
`.env.example` da raiz:

```bash
pnpm nx run redis:test-race
```

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test,test-race,govulncheck -p redis
go run ./tools/dmpf-conformance/cmd/conformance --root .
go run ./tools/dmpf-conformance/cmd/modsync --root . --check
```

## Referências

- `docs/specs/SPEC-STEJG347-revogacao-de-sessao-authn.md` — requisitos da
  deny-list, precedência e falha fechada.
- `libs/backend/go/authn/README.md` — `RevocationStore`, `MemoryStore` e o
  `Verifier`.
- `infra/README.md` — o Redis da infra de testes, do Compose local e do
  Kustomize.
