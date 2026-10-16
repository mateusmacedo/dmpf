# ADR-063: Separar em cada app a role que migra da role que serve

## Status

Aceito — 2026-10-15. Implementa [SPEC-QPRR24C4](../specs/SPEC-QPRR24C4-duas-roles-de-banco-por-app.md). Emenda o [ADR-051](./051-escopo-de-tenant-por-choke-point-em-go.md), na premissa de role dona e na condição de RLS do §Consequências, e o [ADR-053](./053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md), nos itens 1 e 3 do §Contexto. Não há servidor de `hmg`: o overlay `hmg` é template, e as condições para quem provisionar um servidor a partir dele estão em §Pré-condições de servidor externo.

## Contexto

Até aqui cada app conectava, servia e migrava com uma única role, dona do banco (ADR-053, item 1), por um único `PG_DSN`, e o `postgres.Migrate` rodava na mesma pool que atendia requisições. Quem é dono de uma tabela pode reconceder a si o que um `REVOKE` tirou e pode alterá-la, então nenhum privilégio por tabela protegia a tabela contra a própria aplicação. O ADR-051 descartou a RLS pelo mesmo motivo e deixou registrada a condição para retomá-la: uma role de aplicação que não seja superuser nem dona das tabelas.

O consumidor externo antecipa-market (SPEC-GXGASP24) precisa de uma tabela de auditoria imutável contra a role de runtime da app. Esse requisito não se cumpre com uma role dona.

As três investigações da spec foram feitas contra a imagem `postgres:16-alpine` com o digest da infra de testes:

1. **Superuser e versão (Compose, CI e overlay `dev`).** O administrativo `postgres` é superuser, como declaram o Compose, o CI e o Secret `postgres` da base k8s. As roles das apps não são superuser, mas cada uma é dona do próprio banco e das tabelas. O servidor do digest de testes responde `server_version_num` 160015; Compose e k8s usam a tag `16-alpine`, que fixa a major 16, o que cumpre a pré-condição de PostgreSQL 15 ou superior. A premissa do ADR-051 (role superuser e dona) vale para o CI, onde os testes conectam como `postgres`; em local e `dev` a role da app é dona, mas não superuser.
2. **Privilégios mínimos do kernel.** A suíte inteira de integração do `postgres` passa sob uma role que só recebe o conjunto declarado: claim com `FOR UPDATE SKIP LOCKED`, purgas por `ctid`, sinais, inbox, quarantine, `Table`, `Relation`, `SnapshotTable` e `to_regclass`. As colunas `IDENTITY` de `outbox` e `quarantine` não exigem `USAGE` na sequência. O conjunto medido é:

   | Tabela | `SELECT` | `INSERT` | `UPDATE` | `DELETE` |
   |---|---|---|---|---|
   | `outbox` | sim | sim | sim | sim |
   | `inbox` | sim | sim | sim | sim |
   | `quarantine` | sim | sim | não | não |

   O grant é por tabela, comum aos três processos. A espera por processo segue a capability que cada um usa: o `api` espera `outbox`, `inbox` e os grants da app; o `relay`, só `outbox`; o `consumer`, `outbox`, `inbox` e os grants do provider.
3. **`INSERT ... ON CONFLICT DO NOTHING` sem `UPDATE`.** O PostgreSQL aceita o statement com `SELECT` e `INSERT`. `UPDATE` só é exigido por `DO UPDATE`.

A medição revelou também que o PostgreSQL concede `TEMPORARY` a `PUBLIC` em todo banco. Uma role sem `CREATE` em `public` ainda cria tabela temporária.

## Decisão

**Cada app tem duas roles. A migradora `<app>_migrator` é dona do banco e só é usada para migrar; a role de runtime `<app>` serve sem posse e só com o privilégio declarado por tabela.**

1. **Provisionamento.** O bloco opcional `database.migrator` do `infra.json` (`passwordEnv`, `local`, `dev`) faz o `infrasync` gerar, no Compose e no Job do overlay `dev`, as duas roles. A migradora é dona do banco: o banco é criado já com ela como dona, ou tem o dono trocado se existir com outro.
   - As duas roles têm os atributos reconciliados (`LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`): uma migradora criada à mão com `SUPERUSER` deixa de ser superuser na execução seguinte. A runtime tem `CONNECT` no banco e `USAGE` em `public`.
   - `CREATE` em `public` é revogado de `PUBLIC` e da runtime, e `CONNECT` e `TEMPORARY` são revogados de `PUBLIC`: as roles das outras apps não conectam no banco. O kernel não usa tabela temporária.
   - As duas senhas são reconciliadas a cada execução. A senha do migrador fica na chave `<app>_migrator` do Secret `databases`.
   - As senhas chegam pelo ambiente do container, e o psql as lê com `\getenv`: nunca passam por `sh -c` nem por argv, onde um caractere de shell seria interpretado e o valor ficaria em `/proc/<pid>/cmdline`. A sessão desliga `log_statement` e eleva `log_min_error_statement` a `panic` antes do `ALTER ROLE ... PASSWORD`, para uma falha não levar a senha ao log do servidor, que segue para o Loki.
   - O pod do Job não monta o token da ServiceAccount.
   - Sem o bloco, a saída gerada é idêntica à de antes.
2. **DSN do migrador.** `PG_MIGRATE_DSN` chega só ao processo `api` com `MIGRATE=true`.
   - No Compose, fica no serviço `<app>-api`, fora da âncora e do `extends` que `relay` e `consumer` compartilham.
   - No Kubernetes, vem do Secret `<app>-migrator`, referenciado só pelo Deployment do `api`. O Secret `<app>` entra por `envFrom` também no `relay` e no `consumer`, e uma chave a mais nele chegaria aos três.
   - Fora de container, a variável fica em `deploy/.env.migrator.example`, que só os alvos do `api` (`serve-api` e `docker:run`) carregam.
   - A configuração das apps só lê a variável no papel `api` com `MIGRATE=true`, e a exige nesse caso.
   - No template do overlay `hmg`, o `PG_MIGRATE_DSN` usa `sslmode=verify-full&channel_binding=require`. Com `require`, o certificado não é verificado, e um intermediário na rede repassaria a autenticação SCRAM e ganharia uma sessão de dona do banco.
   - O `dmpf-context-check` reprova a role `<app>_migrator`, em DSN URL ou chave/valor ou em `PGUSER`, fora do serviço `<app>-api`, e o Secret `<app>-migrator` referenciado fora do Deployment `<app>-api`.
3. **Partida.** Com `MIGRATE=true` e `PG_MIGRATE_DSN` preenchido, o `api` abre uma pool efêmera da migradora, sem o meter provider, cujo callback o `NewPool` nunca desregistra, e a fecha antes de servir; a pool de requisições nunca executa DDL. A role que recebe os grants é a role de sessão do `PG_DSN`, lida por `current_user` na pool de requisições, e não o usuário escrito no DSN, que um pooler pode trocar; não há variável nova para o mesmo fato. A partida segue esta ordem, e cada passo recusa antes do seguinte:
   1. **Mesmo banco.** `postgres.SessionOf` lê, pelas duas pools, a role, `current_database()`, o oid do banco e `pg_postmaster_start_time()`, que toda role executa. A mesma role nas duas é `serve.ErrMigratorIsRuntime`; banco, oid ou hora de partida diferentes são `serve.ErrMigratorElsewhere`, antes de qualquer DDL ou grant. O critério não compara endereço nem porta, para valer através de pooler, e a hora de partida separa dois servidores com bancos de mesmo nome e oid, inclusive uma réplica do primário. O `system_identifier` de `pg_control_system()` foi descartado porque é o mesmo no primário e nas réplicas físicas.
   2. **Posse.** `postgres.AssertGrantable` roda, pela migradora, as recusas do `ApplyGrants` sobre a própria runtime (item 5). Num banco criado com a role única, em que a runtime ainda é dona das tabelas, a partida recusa com o `REASSIGN OWNED BY` em vez de o DDL falhar com o SQLSTATE 42501 cru de `CREATE INDEX` em tabela alheia.
   3. **`postgres.Migrate`** e **`postgres.ApplyGrants`**, pela migradora.
   4. **Espera curta.** O `api` espera, por até 15 s dentro do prazo da migração, que a pool de requisições enxergue o privilégio que acabou de ser concedido, pelo mesmo `postgres.WaitForGrants` dos demais processos, e recusa a partida com `serve.ErrRuntimeUngranted` se o prazo estourar.

   Com `PG_MIGRATE_DSN` vazio, a app migra pela pool do `PG_DSN`, como antes, e não aplica grants.
4. **Grants.** O pacote `postgres` declara `Privilege` (`SELECT`, `INSERT`, `UPDATE`, `DELETE`) e `Grant`, com os construtores `Mutable` (tabela de agregado escrita por `Table`) e `AppendOnly`. `TRUNCATE`, `REFERENCES` e `TRIGGER` nunca são concedidos.
   - `KernelGrants` fixa o resultado da investigação 2: `outbox` e `inbox` com os quatro privilégios, `quarantine` com `SELECT` e `INSERT`, sem grant de sequência.
   - Cada capability declara os grants das próprias tabelas, e um teste reprova capability sem eles.
   - O provider de cada contexto exporta `Grants`, ao lado de `Schema`.
5. **`ApplyGrants`.** Numa transação, sob o lock da migração, valida antes de mudar qualquer privilégio. Recusa:
   - nome de tabela que não é identificador;
   - tabela declarada duas vezes ou privilégio fora do conjunto;
   - runtime que é a própria role conectada, é superuser, tem `BYPASSRLS`, é membro da migradora ou é dona de algum objeto do banco: relação, função ou tipo fora dos schemas de sistema (`pg_*` e `information_schema`), exceto o tipo de linha implícito de uma tabela e o tipo de array implícito, que acompanham o dono do objeto de origem, e large object. No último caso, a mensagem traz o `REASSIGN OWNED BY "<app>" TO "<app>_migrator"` que um superuser roda para corrigir. Essas recusas sobre a runtime são também o `postgres.AssertGrantable`, que a partida roda antes do `Migrate` (item 3);
   - tabela de `public` sem grant;
   - sequência de `public` que não pertence a uma coluna `IDENTITY` (`serial`, `bigserial` ou `nextval` avulso). A runtime não recebe privilégio em sequência, e a coluna `IDENTITY` é a única que insere sem ele; sem a recusa, a falha só apareceria na primeira inserção, como SQLSTATE 42501. O schema da app declara a coluna como `GENERATED ALWAYS AS IDENTITY`.

   Depois converge: `REVOKE ALL` em tabelas e sequências de `public` e `GRANT` exato por tabela. A role e a tabela são citadas como identificador, a tabela qualificada por `public`, porque o nome da app aceita hífen e o da tabela pode ser palavra reservada.

   Por fim, na mesma transação e sob o mesmo lock, confere a pós-condição pelo privilégio efetivo, e não pelas ACL que acabou de escrever, porque o PostgreSQL soma ao privilégio direto o que vem de `PUBLIC`, de membership e de role predefinida como `pg_write_all_data`. A transação só comita se a runtime tiver todo o declarado e nada além dele:
   - todo par tabela e privilégio declarado, do kernel e da app, efetivo na runtime, por `has_table_privilege`. Um `GRANT` emitido por quem não é dono da tabela, mas tem algum privilégio nela, só gera aviso e não concede nada; sem essa conferência, a partida seguiria e a falha só apareceria na primeira consulta;
   - nenhum privilégio de tabela de `public` (`SELECT`, `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE`, `REFERENCES`, `TRIGGER` e, a partir do PostgreSQL 17, `MAINTAIN`) fora do par tabela e privilégio declarado. `SELECT`, `INSERT`, `UPDATE` e `REFERENCES` são medidos por `has_any_column_privilege`, porque o `has_table_privilege` ignora o grant feito só a uma coluna, e um `GRANT UPDATE (coluna)` a `PUBLIC` alteraria uma tabela `AppendOnly`; o grant declarado de tabela também satisfaz essa função, e por isso só conta o privilégio não declarado. Os demais são medidos por `has_table_privilege`. `MAINTAIN` só entra na lista em servidor 17 ou superior, porque até o 16 a função rejeita o nome;
   - nenhum `USAGE`, `SELECT` nem `UPDATE` em sequência de `public`, por `has_sequence_privilege`. A coluna `IDENTITY` insere sem esses privilégios, e o `UPDATE` que o `pg_write_all_data` concede basta para um `setval` reposicionar a sequência. As suítes de integração do kernel passam sem nenhum deles;
   - nenhum `CREATE` em schema que não seja de sistema, e não só em `public`, por `has_schema_privilege`;
   - nenhum `CREATE` nem `TEMPORARY` no banco, por `has_database_privilege`;
   - nenhuma membership no dono do banco (`datdba`) nem no dono de schema que não seja de sistema, o que cobre `pg_database_owner`, dono de `public`;
   - nenhum dos atributos `REPLICATION`, `CREATEROLE` e `CREATEDB`;
   - nenhuma membership, direta ou indireta, em qualquer outra role, com ou sem atributo, por `pg_has_role` com `MEMBER`. Esse modo ignora o `INHERIT`, porque uma membership sem herança ainda permite `SET ROLE` por padrão. Recusar só a role com atributo deixaria passar as roles predefinidas sem atributo, como `pg_execute_server_program`, `pg_read_server_files`, `pg_write_server_files`, `pg_signal_backend` e `pg_write_all_data` concedida `WITH INHERIT FALSE`, que a runtime assumiria por `SET ROLE`. A runtime é provisionada sem membership; a mensagem cita o atributo da role, quando há;
   - nenhum `EXECUTE` em função `SECURITY DEFINER` fora dos schemas de sistema, por `has_function_privilege`. O PostgreSQL concede `EXECUTE` a `PUBLIC` em toda função nova, e uma função desse tipo criada pela migradora executaria com o privilégio da dona.

   O erro nomeia, de uma vez, o que falta e o que sobrou, e o rollback desfaz o `REVOKE` e os `GRANT`.

   O `Migrate` e o `ApplyGrants` fixam `search_path = public` com `SET LOCAL` na própria transação. Sem isso, o `search_path` padrão (`"$user", public`) resolveria o nome sem schema num schema com o nome da role conectada, se existisse, e o DDL criaria ou alteraria a tabela errada (a classe da CVE-2018-1058). `pg_catalog` não é listado: omitido, ele continua sendo procurado antes de `public`, e, listado em primeiro lugar, passaria a ser o schema em que o DDL sem qualificação cria as tabelas, o que o PostgreSQL recusa.
6. **Espera.** Todo processo que espera o schema (`api` sem migrar, `relay` e `consumer`) espera também o privilégio, com `postgres.WaitForGrants` sobre `has_table_privilege`. O `WaitForTables` sozinho devolveria quando o `Migrate` comita, antes dos grants. Dono e superuser têm o privilégio assim que a tabela existe, então a app sem migrador espera como antes. Cada passada das duas esperas é uma consulta só, que devolve a primeira pendência na ordem declarada, e não uma consulta por tabela e privilégio.
7. **Erro do migrador.** Falha no caminho do migrador vira `serve.ErrMigrator`, com o detalhe redigido: o driver escreve o usuário da conexão na mensagem, e DSN, usuário e senha do migrador nunca vão para log, span ou erro. `PG_MIGRATE_DSN` cuja role de sessão é a mesma do `PG_DSN` é recusado com `serve.ErrMigratorIsRuntime`, e o que alcança outro banco ou outro servidor, com `serve.ErrMigratorElsewhere` (item 3).
8. **Testes.** Os testes usam uma role de runtime por cluster de teste, `dmpf_test_runtime`, com senha fixa e exclusiva desse cluster. Atributos e senha são reaplicados a cada uso, sob um advisory lock tomado no banco do servidor de `PG_DSN`: `ALTER ROLE` concorrente na mesma role falha com `tuple concurrently updated`, e o advisory lock é por banco, então tomá-lo no banco de cada teste não serializaria nada.
   - Todo banco de teste nasce com `TEMPORARY` revogado de `PUBLIC`, como o provisionamento real faz, para a pós-condição do `ApplyGrants` valer também nos testes.
   - `tb/pg` expõe `OpenRuntimePool`, `GrantRuntime`, `RuntimeDSN` e `AssertDenied`. O harness distribuído aplica os grants e passa o DSN da runtime aos filhos.
   - `MigratorDSN` passa o banco do teste para `dmpf_test_migrator`, sem superuser e dona do banco, para o teste migrar como a `<app>_migrator`.
   - O e2e do `bff` e os testes de partida dos três contextos dão aos `api` o DSN de `MigratorDSN` como `PG_MIGRATE_DSN`.
   - A infra de testes segue com o servidor administrativo e cria as roles no banco por teste.

Pontos emendados:

- **ADR-051, §Contexto e §Consequências:** a role de conexão deixou de ser superuser fora do CI desde o ADR-053; o que ainda impedia a RLS era a posse. Em app com migrador, a condição "role que não seja superuser nem dona das tabelas" está cumprida. A RLS continua fora: a sonda de dono cruzado do `Table` lê entre tenants e precisa de política própria antes.
- **ADR-053, item 1 do §Contexto:** em app com migrador, o banco tem o nome da app e pertence a `<app>_migrator`; a role com o nome da app é a de runtime, sem posse. App sem o bloco segue com a role única.
- **ADR-053, item 3 do §Contexto:** o `Migrate` por capacidade é seguido do `ApplyGrants` dos grants do kernel e da app, pela pool da migradora.

### Pré-condições de servidor externo (template `hmg`)

O único ambiente é o local, emulado com containers (Compose e o overlay `dev`). O overlay `hmg` é template para um servidor provisionado fora do repositório, e quem o provisionar verifica `server_version_num`, `rolsuper` e `datdba` da role de conexão. Em servidor 14 ou anterior, o `REVOKE CREATE ON SCHEMA public FROM PUBLIC` precisa ser aplicado por superuser em cada banco de app, e também tira o `CREATE` da migradora, que então precisa ser dona do schema `public` (`ALTER SCHEMA public OWNER TO "<app>_migrator"`). O certificado do servidor precisa ser assinado por uma CA que a imagem do `api` reconheça, ou o `sslrootcert` precisa ser montado, porque o `PG_MIGRATE_DSN` do template usa `verify-full`.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Renomear a role de runtime para `<app>_app` | Quebra todo `PG_DSN` e Secret existente |
| Migrar pela pool de runtime com `SET ROLE` da migradora | Exige que a runtime seja membro da migradora, o que lhe devolve o privilégio do dono |
| Processo ou Job de migração à parte | Exige um papel novo nos três binários e nos manifestos; fica como o endurecimento seguinte |
| Segunda chave no Secret `<app>` | O `envFrom` do `relay` e do `consumer` também importa a chave |
| `ALTER DEFAULT PRIVILEGES` | Concede por omissão e contraria o fail-closed |
| `REVOKE` e `GRANT` em transações separadas | Abre uma janela sem privilégio para os processos vivos |
| Esperar só pela tabela | Devolve quando o `Migrate` comita, antes dos grants, e o processo abre a primeira consulta recusada |
| `PG_RUNTIME_ROLE` | Segunda fonte do mesmo fato que o usuário do `PG_DSN` |
| Uma role de teste por teste | Cria e remove roles do cluster inteiro a cada teste |
| Ligar a RLS agora | A sonda de dono cruzado lê entre tenants e precisaria de política própria |

## Consequências

**Positivas:**

- A role que serve requisições não executa DDL, `TRUNCATE` nem tabela temporária, e só toca as tabelas que o kernel ou a app declaram. Uma tabela sem grant fica inacessível a ela e reprova a partida.
- Uma tabela `AppendOnly` fica imutável contra a própria aplicação, o que o consumidor antecipa-market exige da auditoria.
- A condição do ADR-051 para somar RLS ao choke point fica cumprida em app com migrador.
- As suítes do kernel e dos contextos provam o caminho normal sob a role de runtime.

**Negativas:**

- O `api` com `MIGRATE=true` guarda o DSN do migrador no ambiente: a proteção vale contra a role de runtime e o caminho normal da aplicação, não contra um `api` comprometido. Tirar o migrador do processo servidor é o passo seguinte.
- Banco criado com a role única tem tabelas da role de runtime. O `AssertGrantable` reprova a partida antes do `Migrate` e indica o `REASSIGN OWNED BY "<app>" TO "<app>_migrator"`, que um superuser roda antes do deploy: o comando exige os privilégios da role de origem, e a migradora não os tem.
- **Risco residual: large object criado pela runtime.** Criar large object (`lo_create`, `lo_import`) não depende de privilégio que a pós-condição possa revogar, e quem o cria é o dono. A runtime pode, assim, ocupar espaço com large objects; a recusa de posse só os vê na partida seguinte, que fica recusada até um superuser rodar o `REASSIGN OWNED` ou removê-los com `lo_unlink`. O kernel não usa large object.
- A identidade do servidor é a hora de partida do postmaster. Um reinício do servidor entre as duas leituras da partida recusa com `serve.ErrMigratorElsewhere`, e a tentativa seguinte passa. `PG_DSN` apontado para uma réplica também é recusado, o que é desejado, porque o `api` escreve.
- Uma réplica antiga que reinicia durante um rollout reaplica a matriz antiga. Tabela nova ela recusa, porque não a declara; privilégio mudado numa tabela existente volta ao antigo até a réplica nova reiniciar. Mudança de grant em tabela existente só é segura depois do rollout completo.
- **Rollback de versão.** A recusa de tabela sem grant, exigida pela spec, trava o rollback para uma versão que não declara o grant de uma tabela que a versão nova criou: depois de um `kubectl rollout undo`, o `api` da versão anterior recusa a partida. O trade-off é aceito, porque rebaixar a recusa a aviso deixaria a falha para a primeira consulta. O procedimento é um de dois, decidido antes do rollback: a versão anterior passa a declarar o grant da tabela nova, num patch dela, ou a tabela nova é removida pela migradora, depois de preservado o dado que ela guardar.
- Um `conformance` anterior a este ADR recusa o bloco `database.migrator`, porque rejeita campo desconhecido. O consumidor externo precisa de `conformance` e plugin desta release juntos. O workflow de release só sobe os pins de `kernel` e `conformance` do `versions.json` do plugin quando a versão é explícita (`sync-versions.mjs --target`): primeira release, versão informada no dispatch ou branch de pré-release. Com a versão derivada dos commits, o script mantém `kernel`, `conformance` e as migrations e só atualiza o `workflowRef`, e os pins precisam subir à parte, num PR, antes de o consumidor adotar o bloco. O job `external-consumer` usa o `conformance` da árvore do PR e não prova essa ordem.
- O lock do `ApplyGrants` só coordena quem migra pela app: DDL feito por fora, sem o mesmo lock, não é excluído. É a mesma premissa do `Migrate`.
- O nome do Secret `<app>-migrator` colide com o Secret de uma app que se chame `<app>-migrator`.
- **Risco residual: uma só runtime para os três processos.** `api`, `relay` e `consumer` conectam com a mesma role `<app>`, e o grant é por tabela, não por processo. O `api` recebe, assim, `UPDATE` e `DELETE` na `outbox`, que só o `relay` usa, e o `relay` recebe o que o `api` precisa nas tabelas da app. Separar uma role por processo fica como endurecimento seguinte.
- **Custo aceito:** cada app passa a ter duas credenciais de banco, e o contexto declara os grants das próprias tabelas.

## Referências

- SPEC-QPRR24C4 (DEVS-94).
- [ADR-050](./050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md), [ADR-051](./051-escopo-de-tenant-por-choke-point-em-go.md), [ADR-053](./053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md), [ADR-054](./054-apps-autocontidos-e-infras-separadas.md) e [ADR-060](./060-o-dmpf-tem-consumidor-externo.md).
- `docs/dmpf/contexto-erros-seguranca.md`, regras IDN-14, DAT-11 e DAT-12.
- PostgreSQL 16, documentação de `INSERT` (privilégios de `ON CONFLICT`) e de privilégios (`TEMPORARY` concedido a `PUBLIC`).
