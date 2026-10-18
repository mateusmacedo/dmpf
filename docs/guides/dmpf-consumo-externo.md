# Consumir o DMPF num workspace Nx de fora

Um workspace Nx fora deste monorepo usa o DMPF por versão: o plugin
`@mateusmacedo/dmpf-plugin` vem do GitHub Packages, e o kernel e o
`dmpf-conformance` vêm do proxy Go, pela tag que o `versions.json` do plugin
fixa (ADR-060). Este guia vai do acesso ao registry à atualização de versão. O
que cada generator, executor e migration faz está em
[`tools/dmpf-plugin/README.md`](../../tools/dmpf-plugin/README.md).

## Pré-requisitos

- Workspace Nx 23.x com pnpm: o plugin declara `@nx/devkit` `^23.0.0` como
  peer.
- Go na versão do campo `go.directive` do `versions.json` do plugin; o `init`
  grava essa diretiva no `go.work`.
- `bash`, `git` e `jq`, que os scripts do plugin usam, e Docker para a infra
  local.

## 1. Acesso ao GitHub Packages

O registry npm do GitHub Packages exige token até para instalar pacote
público. Fora do GitHub Actions, use um personal access token (classic) com o
escopo `read:packages`.

O registry do escopo vai no `.npmrc` da raiz do workspace, versionado:

```ini
@mateusmacedo:registry=https://npm.pkg.github.com
```

A credencial vai no `.npmrc` do seu usuário (`~/.npmrc`), nunca no do
workspace:

```ini
//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}
```

```bash
export NODE_AUTH_TOKEN=<token>
```

Desde a 11.5.3, o pnpm ignora `${...}` em credencial do `.npmrc` do repositório
e avisa `Ignored project-level auth setting` (GHSA-3qhv-2rgh-x77r): o arquivo vem
com o checkout, e expandir a variável deixaria um repositório desviar o token
para outro registry. No `.npmrc` de usuário a variável continua valendo, e o
arquivo não guarda o token. No CI, o workflow do DMPF (passo 5) preenche
`NODE_AUTH_TOKEN` com o `GITHUB_TOKEN` do job, que baixa pacote público de
qualquer repositório, e grava a credencial num `.npmrc` de usuário próprio.

## 2. Instalar e inicializar

```bash
pnpm nx add @mateusmacedo/dmpf-plugin
pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=github.com/<org>/<repo>
```

O `nx add` instala o pacote e roda o `init` sem opções, que grava o prefixo
reservado `example.com/change-me`. A segunda chamada troca pelo prefixo real
sem `--force`: os arquivos que o `init` gerou e ninguém editou são
reescritos, decididos pelo hash em `dmpf.rendered.json`.

O `init` grava o `dmpf.json` com `tooling.mode: version`, o `go.work`, o Go
setup do `nx.json`, o `@nx-go/nx-go`, o `.golangci.yml`, o esqueleto estático
de `infra/`, o baseline vazio, os ativos de IA em `.claude/` e `.agents/` e o
chamador de CI em `.github/workflows/dmpf-ci.yml`. Versione todos, inclusive o
`dmpf.rendered.json`.

## 3. Primeiro contexto

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <name> --boundedContext <ctx>
pnpm nx run-many -t tidy
```

O generator cria o módulo do contexto e o de contrato em
`<appsDir>/<name>`, roda o `modsync --write` com os `require` do kernel na
versão do `versions.json` e o `infrasync --write`, e imprime o comando que
regrava o baseline. Rode-o: unidades novas mudam a classificação.

```bash
CONFORMANCE="$(jq -r .conformance node_modules/@mateusmacedo/dmpf-plugin/versions.json)"
go run "github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/conformance@${CONFORMANCE}" --root . --write-baseline
```

O código de negócio dos blocos e o `.proto` do contrato ficam com o autor do
contexto. Com os ativos de IA do `init`, `/dmpf-new-context SPEC-<id>` escreve
o contexto a partir da spec e para em qualquer gate normativo. Os `go.mod`
exigem o kernel por tag, sem `replace`.

## 4. Gates locais

São os mesmos que o CI roda no job de gates:

```bash
DMPF=github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd
go run "${DMPF}/conformance@${CONFORMANCE}" --root .
go run "${DMPF}/modsync@${CONFORMANCE}" --root . --check
go run "${DMPF}/infrasync@${CONFORMANCE}" --root . --check
DMPF_APPS_DIR="$(jq -r .appsDir dmpf.json)" \
  DMPF_KERNEL_DDL="$(go mod download -json github.com/mateusmacedo/dmpf/libs/backend/go/postgres | jq -r .Dir)" \
  bash node_modules/@mateusmacedo/dmpf-plugin/scripts/dmpf-context-check.sh --phase structural
pnpm nx run-many -t buf-lint
```

A cadeia Go de cada projeto roda pelos targets que o generator declarou; os
testes de integração pedem a infra de testes do contexto:

```bash
pnpm nx run <name>:test-infra-up
pnpm nx run-many -t fmt-check,vet,test-race
```

O `conformance` sai com exit 2 e `NAO VERIFICADO` quando não consegue ler um
módulo do kernel, por exemplo sem rede e sem o módulo no cache: o gate nunca
aprova o que não leu.

## 5. CI

O `.github/workflows/dmpf-ci.yml` que o `init` escreve chama o
`dmpf-go-ci.yml` deste repositório fixado no `workflowRef` do `versions.json`,
o SHA do commit de onde a versão do plugin saiu, e declara a versão do plugin
para a qual foi escrito. O workflow roda dois jobs:

- **pirâmide Go**: os estágios por `layer:*`, com Postgres, Redpanda e floci
  para a integração, e os gates Buf no estágio de contrato;
- **gates DMPF**: camada única por módulo, gate de dependência, context-check,
  `modsync --check`, `infrasync --check`, `conformance` e `govulncheck`.

O chamador concede `packages: read`, que o workflow pede para instalar o
plugin. Se o plugin instalado divergir da versão do chamador, o workflow
reprova e indica as migrations. A base do `affected` é a branch default do
repositório.

## 6. Atualizar de versão

```bash
pnpm nx migrate @mateusmacedo/dmpf-plugin@<nova>
pnpm install
pnpm nx migrate --run-migrations
```

As migrations do plugin reaplicam os templates do `init`, inclusive o
chamador de CI com o novo `workflowRef`, e sobem o `require` do kernel nos
`go.mod`, os pins de `govulncheck`, `golangci-lint` e `@nx-go/nx-go` e a
diretiva do `go.work`. Rode, na ordem, os comandos que elas devolvem ao fim
(`pnpm install` e `pnpm nx run-many -t tidy`, quando houver, e por último o
`infrasync --write` do `conformance` instalado) e os gates do passo 4:

```bash
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/infrasync@<conformance> --root . --write
```

No modo `local`, o comando é `go run ./tools/dmpf-conformance/cmd/infrasync
--root . --write`. Ele é necessário porque a migration reaplica o
`infra/local/env.platform.example` e o `conformance` novo pode mudar o
provisionamento gerado; sem regenerar, o `infrasync --check` do passo 4
reprova. A saída do `infrasync` (provisionamento do Compose, PKI, Swagger,
`.env.example` e listas de apps dos overlays) é regravada: edite os manifestos
`deploy/infra.json` e o `env.platform.example`, não a saída. Commite os
arquivos gerados que mudarem.

Quem já subiu para a `1.0.0-rc.5` não recebeu esse passo, porque a migration
dessa versão não o devolve. Rode-o à mão e commite o
`infra/local/compose/provisioning.generated.yml` e o `infra/local/.env.example`:

```bash
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/infrasync@v1.0.0-rc.5 --root . --write
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/infrasync@v1.0.0-rc.5 --root . --check
```

O `infrasync` só gera o `.env.example`: copie à mão as variáveis novas dele
(`REDIS_REVOCATION_PASSWORD`, `REDIS_EXPORTER_PASSWORD`, `KEYCLOAK_*`) para o
seu `.env` local.

Arquivo do `init` editado à mão não é sobrescrito: a migration o lista no log.
Compare com a versão nova e, depois de salvar o que quiser manter, sobrescreva
com `pnpm nx g @mateusmacedo/dmpf-plugin:init --force`.

### Adotar as duas roles de banco (ADR-063)

Um contexto gerado antes do ADR-063 segue válido com a role única, dona do
banco. Para separar a role que migra da que serve:

1. Atualize o plugin e o `dmpf-conformance` para a mesma versão: um
   `conformance` anterior recusa o bloco novo do `infra.json`.
2. Acrescente ao `infra.json` do contexto o bloco `database.migrator`
   (`passwordEnv`, `local`, `dev`) e rode o `infrasync --write`.
3. Exporte em `provider/schema.go` os `Grants` de cada tabela do `schema.sql`
   (`postgres.Mutable` para a tabela que um `Table` escreve, `postgres.AppendOnly`
   para a que só recebe linhas novas) e passe-os em `serve.API.Grants`.
4. Leia `PG_MIGRATE_DSN` só no papel `api` (`serve.APIRole.MigrateDSN`) e
   declare-o só no processo com `MIGRATE=true`: no Compose, no serviço do `api`;
   no Kubernetes, num Secret `<ctx>-migrator` referenciado só pelo Deployment
   do `api`.
5. Recrie o banco: a partida recusa uma role de runtime dona de tabela. Com
   dado real, transfira antes a posse para a migradora
   (`REASSIGN OWNED BY "<ctx>" TO "<ctx>_migrator"`).

### Adotar a revogação de sessão (ADR-065)

Um workspace atualizado segue sem revogação: o `authn` sem
`Config.Revocations` se comporta como antes. O Redis dos templates sobe mesmo
assim (no `test-infra up` e, no Compose local, nos profiles `redis`, do
contexto de borda, `keycloak` e `all`), mas fica ocioso enquanto a borda não o
usa. Para ligar a deny-list na borda:

1. Atualize o plugin para a versão do ADR-065: a migration traz o Redis da
   infra de testes (porta 16379) e o `REDIS_URL` do `.env.example`, o Redis com
   ACL do Compose local, a base `infra/k8s/base/redis` e o realm com os eventos
   `LOGOUT`. Rode o `pnpm install` e o `tidy` que ela devolver.
2. Declare `REDIS_URL` como obrigatório na borda e monte o checker antes do
   `NewVerifier`, como no passo 8. Sem a variável, a partida deve falhar: a
   revogação desligada em silêncio anula o requisito.
3. Crie o papel `session-events` da borda, com uma réplica e configuração
   própria, e aponte o back-channel logout do realm para ele
   (`KEYCLOAK_BACKCHANNEL_LOGOUT_URL` no Compose local,
   `DMPF_BACKCHANNEL_LOGOUT_URL` no ConfigMap `keycloak` do Kubernetes; com o
   `edge` do `init`, os templates já o fazem).
4. Componha `infra/k8s/base/redis` no overlay de desenvolvimento e forneça o
   Secret `redis` (chaves `users.acl` e `url`); fora dele, aponte `url` para um
   Redis externo com a mesma ACL, AOF e `noeviction`.
5. Reimporte o realm em cada ambiente com estado, pelo procedimento do item 10
   do ADR-064: no `dev`, escale o Keycloak a 0, apague o PVC `keycloak-data` e o
   Job `keycloak-seed` e reaplique; fora dele, `kc.sh import --override true`
   com o servidor parado. Sem o reimport, o back-channel e os admin events
   valem, mas os eventos `LOGOUT` não são guardados, e o logout do usuário
   depende só do back-channel.

### Adotar a unicidade declarada (ADR-066)

Uma violação de `UNIQUE` que não é a chave primária chegava ao chamador como
`INTERNAL`. Com a declaração, a duplicata de negócio por tenant volta como
`DomainRejection`: a resposta gRPC sai com status `OK` e o campo `rejection` com o
código declarado (o BFF a expõe como `422`), gravada na inbox e reproduzida no replay. O índice continua sendo a garantia; a declaração
só traduz. O `bookings` é o exemplo de referência. Para cada unicidade de
negócio:

1. Declare a restrição dentro do `CREATE TABLE`, no `provider/schema.sql`, como
   `CONSTRAINT <tabela>_<colunas>_key UNIQUE (tenant_id, ...)`: o nome é a
   tabela, as colunas na ordem, separadas por `_`, e o sufixo `_key`, em
   minúsculas e com até 63 bytes, e `tenant_id` vem primeiro. `CREATE UNIQUE
   INDEX`, `DEFERRABLE` e `NULLS NOT DISTINCT` não servem, nem tabela
   particionada, partição ou com herança: na partição, o PostgreSQL reporta a
   restrição da partição folha, que não casa com a declarada, e na herança a
   `UNIQUE` vale por tabela e não cobre a hierarquia. Atributo que vive só no
   snapshot vira coluna gerada:

   ```sql
   name        text   GENERATED ALWAYS AS (snapshot->>'name') STORED NOT NULL,
   CONSTRAINT resources_pkey PRIMARY KEY (tenant_id, resource_id),
   CONSTRAINT resources_tenant_id_name_key UNIQUE (tenant_id, name)
   ```

2. Declare a rejeição estática no `domain`. Quem a levanta é o service, a
   partir do `Save`, nunca a UPR:

   ```go
   CodeResourceDuplicateName kernel.Code = "resource-scheduling/resource/duplicate-name"

   var RejectResourceDuplicateName = kernel.Reject(CodeResourceDuplicateName, "a resource with this name already exists in the tenant")
   ```

3. Ligue a restrição à rejeição no `Table` do provider, com `postgres.Unique`
   de mesmo nome e mesmas colunas:

   ```go
   table.Unique = []postgres.Unique{{
       Name:      "resources_tenant_id_name_key",
       Columns:   []string{"tenant_id", "name"},
       Rejection: domain.RejectResourceDuplicateName,
   }}
   ```

4. Exporte em `provider/schema.go` a lista de todo `Table` com `Unique`:

   ```go
   var Uniques = []postgres.UniqueSet{resourceTable.Uniques()}
   ```

5. Passe a lista em `serve.API` no wiring (`app/wiring.go`), ao lado de
   `Grants: provider.Grants`: `Uniques: provider.Uniques`. Na partida,
   `postgres.VerifyUniques` confere cada restrição em `pg_constraint` e recusa
   subir com `ErrUniqueDivergent` se o schema divergir da declaração ou se a
   tabela for particionada ou usar herança.
6. Espelhe a declaração nos dublês com `memory.Unique`, cuja `Key` devolve as
   colunas depois de `tenant_id`:

   ```go
   memory.Table[domain.ResourceCode, domain.ResourceSnapshot]{Name: "resources", Unique: []memory.Unique[domain.ResourceSnapshot]{{
       Name:      "resources_tenant_id_name_key",
       Key:       func(s domain.ResourceSnapshot) ([]string, bool) { return []string{string(s.Name)}, true },
       Rejection: domain.RejectResourceDuplicateName,
   }}}
   ```

7. Prove o provider com `providerkit.Unique` no teste de repositório,
   declarando cada restrição em `Constraints`
   (`providerkit.UniqueConstraint{Name, Columns, Code}`, em que `Columns` lista
   as colunas como no `postgres.Unique`, começando por `tenant_id`) e
   `Concurrent: true`. Declare as restrições, aqui e no `Table`, na ordem em que
   o DDL as cria: sob colisão em mais de uma, vence a primeira declarada, e o
   `VerifyUniques` reprova a partida quando a ordem diverge do catálogo.

Depois de atualizar o plugin, o `dmpf-context-check.sh` reprova toda
`CONSTRAINT ..._key UNIQUE` do `schema.sql` sem `postgres.Unique` de mesmo nome
no provider, toda `UNIQUE` fora da forma nomeada `CONSTRAINT <nome> UNIQUE`
(inclusive `CREATE UNIQUE INDEX`), toda tabela com `UNIQUE` que use
`PARTITION BY`, `PARTITION OF`, `INHERITS`, `ATTACH PARTITION` ou
`ALTER TABLE [IF EXISTS] [ONLY] <tabela> [*] INHERIT <mãe>`, com o `INHERIT` em
qualquer posição da lista de ações e com esquema, aspas, espaços ou comentários
entre o nome, o `*` e a ação (não `NO INHERIT` nem coluna chamada `inherit`), ou que
seja mãe de partição ou de herança, e todo `Table` com `Unique` fora de
`provider.Uniques`. Tabela particionada ou com herança sem
`UNIQUE` continua aprovada. Contexto sem declaração e sem `UNIQUE` não muda
nada.

O `Save` de uma tabela com `Unique` envia o savepoint, a escrita e o `RELEASE`
num único round trip. No modo padrão do pgx, o primeiro `Save` de cada escrita
numa conexão paga a mais, uma vez, para preparar os três statements por nome;
a recusa paga o `ROLLBACK TO SAVEPOINT` e, na criação, a sonda de identidade,
preparada na primeira criação recusada de cada conexão (4 escritas no socket nessa,
3 nas seguintes), e o `Save` seguinte volta a um round trip. Um `ResetSession` que rode `DEALLOCATE
ALL` ou `DISCARD ALL` invalida esses statements sem o pgx saber. Tabela sem
`Unique` não muda.

## 7. Keycloak de referência (opcional)

O `init` e a migration trazem o Keycloak de referência do ADR-064 como
arquivos gerenciados: `infra/keycloak/` (realm `dmpf`, base Kustomize e o
Component `dev/` com o seed dos dados de teste), `infra/local/compose/keycloak.yml`
(profile `keycloak`) e o serviço `keycloak` da infra de testes, no profile
`keycloak`, que o `test-infra-up` não sobe (`test-infra.sh keycloak` o sobe e
semeia). Três passos ficam à mão, porque os arquivos são de quem consome:

- acrescente `- compose/keycloak.yml` ao `include` de
  `infra/local/docker-compose.yml`, que o `init` só cria;
- nos overlays do Kubernetes, componha `infra/keycloak` (e, no de
  desenvolvimento, o Component `infra/keycloak/dev`) e forneça os Secrets
  `keycloak-admin`, `keycloak-db` e `keycloak-session-events` e o ConfigMap
  `keycloak-hostname` (o endereço público, que vira o issuer) onde não houver
  dados de teste;
- aponte o `authn` para o realm: `OIDC_ISSUER` com o issuer do ambiente,
  `OIDC_AUDIENCE=dmpf-bff` e `OIDC_TENANT_CLAIM=organization.*.tenant_id`, que
  lê o tenant da claim nativa `organization` sem extensão no Keycloak.

O input `keycloak` do `dmpf-go-ci.yml` serve aos testes de integração do
`authn` deste repositório; o chamador do consumidor não precisa ligá-lo.

## 8. Revogação de sessão (opcional)

O `authn` recusa, em menos de 60 s, o token de uma sessão encerrada no Keycloak
(ADR-065). Uma deny-list por `sid` e por `sub` é consultada dentro do
`Verifier`, depois das checagens do token, e falha fechada: com o armazenamento
fora ou mais lento que `OIDC_REVOCATION_TIMEOUT_MS` (padrão 250), o token é
recusado. A deny-list fica no Redis, pelo módulo `libs/backend/go/redis`, e é
alimentada por um processo à parte com os adaptadores do `authn`.

Na borda, monte o checker:

```go
redisConfig, err := redis.FromEnv(os.Getenv)
if err != nil {
	return err
}
client, err := redis.NewClient(redisConfig)
if err != nil {
	return err
}
cfg.Revocations = redis.NewRevocationStore(client, clock.System())
verifier, err := authn.NewVerifier(ctx, cfg)
```

No processo de alimentação (uma réplica), monte o `Feed` sobre o mesmo store e
os dois adaptadores:

- `authn.ReadRevocationFeedEnv` lê `OIDC_REVOCATION_TTL_SECONDS`, obrigatório
  (o maior tempo de vida de token do realm, `300` no de referência), e
  `OIDC_REVOCATION_SKEW_SECONDS` (padrão 60); `authn.NewFeed` recusa a partida
  sem o TTL;
- `authn.NewBackchannelLogout` é o `http.Handler` do back-channel logout, com o
  `ClientID` do client a que o Keycloak endereça o logout token (`dmpf-portal`
  no realm de referência). Responde `503` com o Redis ou o JWKS fora, desvio
  consciente do §2.8 do Back-Channel Logout 1.0;
- `authn.NewSessionPolling` lê os admin events e os eventos `LOGOUT` com o
  service account `dmpf-session-events` (`view-events`); `Run` relê a janela a
  cada ciclo (padrão 5 s) e reaplica tudo, o que recupera o logout que o
  back-channel perdeu e a entrada que o Redis perdeu.

O `apps/backend/bff` deste repositório é a referência dessa montagem: a API
exige `REDIS_URL`, e o papel `bff session-events` serve o receptor e roda o
polling (`apps/backend/bff/README.md`).

O Redis precisa de banco 0, AOF, `maxmemory` com `noeviction` e um usuário ACL
restrito a `~dmpf:revocation:*` e aos comandos que o
`libs/backend/go/redis/README.md` lista; o usuário `default` fica desligado.
Três passos ficam à mão:

- a NetworkPolicy `redis` da base admite entrada só de pods
  `app.kubernetes.io/name: bff`; com outra borda, acrescente um patch no
  overlay;
- o Secret `redis` do overlay leva o arquivo de ACL (`users.acl`) e a URL do
  usuário da aplicação (`url`);
- o papel de alimentação só aceita entrada do Keycloak na porta do receptor, e
  só ele recebe o client secret do polling.

O realm guarda só os eventos de usuário `LOGOUT`, por 1 h, mais que a janela
que o polling relê (`TTL + margem`). Cada evento retém o IP do usuário nessa
hora; declare essa retenção no registro de tratamento de dados do consumidor.

## Solução de problemas

| Sintoma | Causa e saída |
| --- | --- |
| `bounded-context` recusa o prefixo `example.com/change-me` | o `init` rodou sem `--modulePrefix`; rode-o com o prefixo real |
| `init` lista arquivos editados e não escreve nada | os arquivos foram alterados depois do `init`; salve as mudanças e rode com `--force` |
| `pnpm install` responde 401 no `npm.pkg.github.com` | `NODE_AUTH_TOKEN` ausente ou sem `read:packages`, ou a credencial está no `.npmrc` do workspace (o pnpm 11.5.3+ a ignora com `Ignored project-level auth setting`); mova-a para o `~/.npmrc` |
| CI: `dmpf-plugin instalado (X) difere do chamador do CI (Y)` | o plugin subiu sem as migrations; rode o passo 6 |
| `conformance` com exit 2 e `NAO VERIFICADO` | um módulo do kernel não foi lido; confira a rede e o proxy Go |

## Referências

- [`docs/adr/060-o-dmpf-tem-consumidor-externo.md`](../adr/060-o-dmpf-tem-consumidor-externo.md) — a decisão.
- [`docs/adr/065-revogacao-de-sessao-no-authn.md`](../adr/065-revogacao-de-sessao-no-authn.md) — a revogação de sessão no `authn` e o papel que alimenta a deny-list.
- [`docs/adr/066-unicidade-declarada-como-rejeicao.md`](../adr/066-unicidade-declarada-como-rejeicao.md) — a unicidade de negócio declarada no `Table` e devolvida como rejeição de domínio.
- [`tools/dmpf-plugin/README.md`](../../tools/dmpf-plugin/README.md) — generators, executors, migrations e `versions.json`.
- [`docs/guides/dmpf-composicao.md`](dmpf-composicao.md) — como um contexto se compõe sobre o kernel.
- [`docs/guides/dmpf-manifesto.md`](dmpf-manifesto.md) — manifesto, baseline e gate de conformidade.
