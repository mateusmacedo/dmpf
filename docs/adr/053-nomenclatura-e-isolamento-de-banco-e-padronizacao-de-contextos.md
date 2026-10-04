# ADR-053: Cada contexto tem banco próprio, com nomes canônicos, e os três contextos seguem uma forma única

## Status

Aceito — 2026-09-25. Implementa [SPEC-F308KDSF](../specs/SPEC-F308KDSF-padronizacao-contextos-e-banco.md). Supersede parcialmente o [ADR-046](./046-libs-somente-kernel-de-reuso.md), no parágrafo que mantinha as tabelas `dmpf_example_*`, e estende o [ADR-044](./044-bff-rest-e-contextos-grpc-de-referencia.md) com o terceiro banco e o `bookings` na topologia. **Emendado pelo [ADR-054](./054-apps-autocontidos-e-infras-separadas.md) (2026-09-28)**: a infra de cada app vive em `apps/backend/<app>/deploy/`, e a agregada é gerada pelo `infrasync`; o banco de teste deixa de ser `<projeto>_test` compartilhado e passa a um banco por teste, apagado ao fim, num servidor com `cluster_name=test`; o tooling de teste perde o prefixo `DMPF_` (`TESTKIT_*`, `TB_*`, `EVIDENCE_DIR`).

## Contexto

Os três contextos (`orders`, `reservations` e `bookings`) evoluíram em momentos diferentes e divergiam em pontos que o generator e o harness precisavam tratar como iguais:

1. **Banco.** `orders` e `reservations` dividiam um banco, com as tabelas do kernel e as de agregado prefixadas por `dmpf_` e o DDL de agregado dentro de `libs/backend/go/postgres/schema.sql`. O `bookings` tinha migração própria e nomes próprios.
2. **Persistência.** Cada contexto gravava o agregado de um jeito: colunas tipadas em um, `jsonb` inteiro em outro.
3. **Borda.** O `bookings` servia REST direto, enquanto o ADR-024 põe o REST só no `bff` e os contextos atrás de gRPC.
4. **Forma do código.** Config, wiring, interceptors gRPC, mapeamento de erro, nomes do domínio e instante variavam entre os três, e o instante do `bookings` estava em segundos.
5. **Configuração.** As variáveis de ambiente carregavam o prefixo `DMPF_`, que amarrava a configuração de cada app ao nome do framework.

## Decisão

**Cada contexto é dono do seu banco, os nomes seguem uma tabela única, e os três contextos têm a mesma forma de borda, persistência, configuração e domínio.**

1. **Banco por app.** Database e role têm o nome da app (`orders`, `reservations`, `bookings`), com o schema `public`. O compose local cria role e banco por app no `postgres-init`; o overlay `dev` faz o mesmo por Job, e o `hmg` declara os Secrets por template. Os bancos foram recriados, sem rename de tabela: os ambientes só tinham dados descartáveis, e nenhum código de migração transitória entra no kernel.
2. **Nomes canônicos.** Tabela do kernel é substantivo sem prefixo (`outbox`, `inbox`, `quarantine`); tabela de agregado é o agregado no plural (`orders`, `bookings`, `resources`), o que contorna a palavra reservada `order` sem aspas; coluna de id é `<agregado>_id`; índice é `<tabela>_<colunas>_idx`; constraint é `<tabela>_<colunas>_{pkey,key,check,fkey}`. A tabela completa está no Design da spec.
3. **Migrate por capacidade.** O kernel publica o DDL de `outbox` e de `inbox`/`quarantine` como capacidades (`postgres.Outbox`, `postgres.Inbox`), e `postgres.Migrate` aplica só as que o contexto pede, seguidas do schema do próprio contexto, que vive no `provider/schema.sql` dele. `orders` pede `Outbox`; `reservations`, que consome, pede `Outbox` e `Inbox`.
4. **Persistência híbrida.** Coluna tipada só para o que uma consulta filtra; o resto do estado vai num `snapshot` `jsonb`, serializado por um struct de estado privado do provider, com tags JSON estáveis.
5. **Banco de teste por projeto (D5).** `tb/pg` deriva `<projeto>_test` do servidor em `PG_DSN`, recria o schema `public` na primeira abertura do processo e migra só as capacidades pedidas. Os testes de projetos distintos rodam em paralelo; dentro de um projeto, `test-distributed` depende de `test-race`, porque os dois usam o mesmo banco de teste.
6. **Borda gRPC no `bookings`.** O `bookings` serve só gRPC (`company.bookings.service.v1`), e o `bff` passa a expor as rotas REST dele. O e2e do `bff` fala só pela superfície pública (REST e tópico Kafka) e nunca lê tabela de contexto. Os eventos de cancelamento e de registro de recurso passam a ser publicados, porque o golden precisa provar que todo evento emitido chega à outbox.
7. **Cadeia gRPC no kernel.** A cadeia de interceptors de contexto (`kernelgrpc.ServerInterceptors`), os limites por método e as chaves de metadata saem dos contextos para `libs/backend/go/grpc`; cada contexto expõe apenas `rpc.Methods()` e o seu `errors.go`.
8. **Instante em nanossegundos (D1).** O instante de domínio é um inteiro de nanossegundos nos três contextos; `ports.Instant.Unix()` foi removido.
9. **Fio preservado (D2).** Os nomes Go seguem a forma canônica (`Code<Agregado><Motivo>`, `Cancelled`), e os valores publicados — códigos de rejeição, enums de status e nomes de evento — não mudam; a borda faz o mapeamento. A forma `<ctx>/<agregado>/<motivo>` vale para códigos novos, e `kernel.Code.Valid` passa a aceitar tanto `context/reason` quanto `context/aggregate/reason`.
10. **Configuração sem prefixo.** A configuração de apps e libs perde o `DMPF_`: `PG_DSN`, `GRPC_ADDR`, `GRPC_CLIENT_CA_FILE`, `KAFKA_BROKERS`, `BOOKINGS_GRPC_TARGET`. O tooling mantém o prefixo (`DMPF_GENERATOR_CHECK_*`, `DMPF_HARNESS_CHECK_AGENT_CMD`, `DMPF_TESTKIT_*`, `DMPF_TB_*`, `DMPF_EVIDENCE_DIR`), porque ali o nome identifica a ferramenta e não a app. Cada contexto lê a config pela mesma forma: `Defaults(role)`, `FromEnv` e `Validate`.
11. **Observabilidade do banco.** Um único exporter do Postgres, ligado ao banco de administração, cobre os bancos das apps.
12. **Autenticação documentada no contrato.** Os três OpenAPI declaram o esquema `bearerAuth`, para que o Swagger UI ofereça onde informar o token.
13. **Imports agrupados.** O `golangci-lint` roda o formatter `gci` com três seções: stdlib, externos e o módulo `github.com/mateusmacedo/dmpf`.
14. **Generator e regeneração (D3, D4).** O generator emite a borda gRPC genérica (`errors.go`, a cadeia do kernel e um `service.go` com `ServiceDesc` vazio), e `/dmpf-new-context --regen` aceita spec `done`, para que a regeneração do golden prove a equivalência com o código escrito.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Schema `messaging` para as tabelas do kernel | Com banco por app, a colisão entre kernel e contexto fica restrita a três nomes conhecidos; um schema a mais só acrescentaria `search_path` a cada DSN |
| Prefixo `platform_` ou `<ctx>_` nas tabelas | Com banco próprio o prefixo não desambigua nada, e o `memory.Table` já usa o nome curto |
| Colunas tipadas para todo o estado | Toda evolução do agregado passaria a exigir DDL |
| `jsonb` puro, sem colunas | As consultas por recurso e por tenant perderiam índice |
| Manter o REST no `bookings`, com exceção em ADR | Deixaria duas formas de borda para o generator e o harness sustentarem |
| `ALTER TABLE` transitório ou migrações versionadas | Os ambientes só têm dados descartáveis; o código de rename ficaria no kernel sem uso depois da primeira execução |
| Mudar os valores publicados junto com os nomes Go | Quebraria consumidores do fio sem ganho para o domínio |
| Um banco de teste separado para o `distkit` | Duplicaria o schema e o reset sem necessidade; a ordem entre `test-race` e `test-distributed` resolve a disputa |

## Consequências

**Positivas:**

- Um contexto não enxerga as tabelas de outro, nem por engano de DSN.
- O generator, o harness e as instruções de AI descrevem uma forma só, que os três contextos já seguem.
- O nome de cada variável diz o que ela configura, sem depender do nome do framework.

**Negativas:**

- Cada ambiente precisa de um role e de um banco por app, criados antes da primeira partida.
- A recriação dos bancos apaga os dados existentes; qualquer ambiente com dado real exigirá migração própria.
- Quem tinha `DMPF_*` em `.env`, Secret ou ConfigMap precisa renomear as variáveis.
- `contracts:buf-breaking` depende da tag `contracts-baseline/proto`, ainda ausente no remoto; até ela existir, a verificação de quebra de contrato é feita manualmente contra `develop`.

## Addendum — 2026-10-01 (as variáveis `OTEL_*`)

O item 10 tirou o prefixo `DMPF_` porque ele amarrava a configuração de cada app ao nome do framework. A [SPEC-1TFW24WV](../specs/SPEC-1TFW24WV-observabilidade-ponta-a-ponta.md) levou a telemetria das apps às variáveis `OTEL_*`, que são exceção: o prefixo é o da especificação de variáveis de ambiente do SDK do OpenTelemetry, e não o do framework, e o SDK e o `autoexport` leem a maior parte delas sem código da plataforma. Renomeá-las exigiria código próprio para ler o que o SDK já lê, e a configuração deixaria de ser a que qualquer ferramenta OTel reconhece.

- **O que as apps declaram.** `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES` (`service.version`, `service.instance.id`, `deployment.environment.name` e `dmpf.process.role`), `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_TRACES_SAMPLER_ARG`, `OTEL_LOGS_EXPORTER`, `OTEL_PROPAGATORS` e `OTEL_GO_X_OBSERVABILITY`, no `deploy/` de cada app (`.env.example`, `compose.yml` e `k8s/`) e em `infra/local/compose/app-base.yml`. O `service.instance.id` e o `dmpf.process.role` de cada processo são acrescentados ao `OTEL_RESOURCE_ATTRIBUTES` no `compose.yml` e nos Deployments do Kubernetes.
- **Quem lê.** O SDK lê `OTEL_SERVICE_NAME` e `OTEL_RESOURCE_ATTRIBUTES` (`resource.WithFromEnv`, em `otelboot/config.go`), `OTEL_BSP_*` e `OTEL_GO_X_OBSERVABILITY`; o `autoexport` lê `OTEL_*_EXPORTER` e `OTEL_EXPORTER_OTLP_*`. A plataforma lê `OTEL_TRACES_SAMPLER_ARG` e `OTEL_TRACES_SAMPLER` em `boot.SignalsFromEnv`, que o `FromEnv` de cada app chama, `OTEL_PROPAGATORS`, que, declarada, só aceita `tracecontext` (`otelboot/config.go`), e `OTEL_SDK_DISABLED` (`boot/telemetry.go`). A forma `Defaults(role)`, `FromEnv` e `Validate` não muda.
- **O que saiu.** `OTLP_ENDPOINT`, `OTLP_INSECURE`, `OTLP_LOGS`, `SERVICE`, `SERVICE_VERSION`, `INSTANCE_ID` e `TRACE_SAMPLE_RATE` saem dos manifestos das apps e do Compose local, e o `telemetry_manifest_test.go` de cada app reprova as sete no `deploy/.env.example`. O `boot` também não lê mais `TRACE_SAMPLE_RATE`: `boot.SignalsFromEnv` só lê `OTEL_TRACES_SAMPLER_ARG`, `OTEL_TRACES_SAMPLER` e `LOG_LEVEL`, e uma `TRACE_SAMPLE_RATE` declarada é ignorada. `LOG_LEVEL` continua sem prefixo: é configuração da plataforma, não do SDK.
- **Migração.** Quem tinha as variáveis antigas em `.env`, Secret ou ConfigMap precisa trocá-las pelas `OTEL_*`, como na troca do `DMPF_`.
