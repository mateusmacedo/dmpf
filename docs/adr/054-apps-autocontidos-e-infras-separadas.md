# ADR-054: Cada app carrega o próprio contrato e o próprio deploy, e testes e runtime local usam infras separadas

## Status

Aceito — 2026-09-28. Implementa [SPEC-VJMM2DE5](../specs/SPEC-VJMM2DE5-apps-autocontidos.md). Supersede parcialmente o [ADR-033](./033-adaptacoes-monorepo-dos-contratos-wire.md), no layout de `contracts/` como módulo Buf único, e o [ADR-046](./046-libs-somente-kernel-de-reuso.md), no parágrafo que mantinha o contrato de cada contexto em `libs/backend/go/contracts`. Emenda o [ADR-045](./045-nomes-bare-e-contexto-em-modulo-unico.md), o [ADR-047](./047-tags-de-modulo-go-e-consumo-fora-do-workspace.md), o [ADR-048](./048-layout-canonico-de-bounded-context.md) e o [ADR-053](./053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md).

## Contexto

Depois do ADR-053, cada contexto tinha banco, borda e forma próprios, mas o material que só ele usava ainda morava fora dele:

1. **Contrato.** O `.proto`, a OpenAPI e as fixtures de `orders`, `reservations` e `bookings` ficavam em `contracts/`, e o gerado em `libs/backend/go/contracts/gen/go`. O kernel testava com o `OrderPlaced` do `orders`, então uma lib do kernel dependia do contrato de um app.
2. **Deploy.** Kubernetes, Compose, variáveis de ambiente e o provisionamento de banco, tópicos e certificados de cada app estavam espalhados por `infra/`, e acrescentar um contexto exigia editar listas em vários arquivos.
3. **Runtime local.** Subir os apps no host exigia disparar cada `serve-*` à mão, na ordem certa, depois da infra; logs e traces locais seguiam a amostragem de produção e os logs dos processos no host não chegavam ao Loki.
4. **Testes.** Os testes de integração usavam a mesma infra do runtime local, deixavam bancos `<projeto>_test`, tópicos, grupos e filas para trás e carregavam o prefixo `dmpf` nos nomes.

## Decisão

**Um app é coeso e autocontido: o contrato e o deploy dele vivem no diretório dele, a infra agregada é gerada a partir do que cada app declara, e testes e runtime local nunca dividem infra.**

1. **Contrato por contexto.** Cada contexto tem `apps/backend/<ctx>/contract/`, módulo Go próprio (`github.com/mateusmacedo/dmpf/apps/backend/<ctx>/contract`) e projeto Nx `<ctx>-contract` (`layer:contract`), com `buf.yaml`, `buf.gen.yaml` (`out: gen/go`, `clean: true`), `proto/`, `openapi/v1/`, `fixtures/`, `gen/go/`, os testes golden e o `dmpf-units.json` da unidade `<bc>/contract`. Quem consome outro contexto importa só `apps/backend/<ctx>/contract`, nunca o app. O nome `<app>-contract` é exceção declarada ao ADR-045: o diretório não repete o contexto, e o projeto Nx precisa de um nome único.
2. **Kernel de contratos autocontido.** `libs/backend/go/contracts` tem `proto/` e Buf próprios, com o gerado só em `gen/go/io/`, e um pacote de teste `dmpf.testing.v1` que substitui o `OrderPlaced` nos testes de `app`, `postgres`, `testkit` e `contracts`. O diretório `contracts/` da raiz deixa de existir.
3. **Gate Buf por módulo e por pacote.** `tools/buf-gate.sh <cmd> <moddir> --project <nome>` lê o diretório gerado do `buf.gen.yaml`. O `buf-breaking` identifica cada `.proto` publicado pelo pacote: um pacote que mudou de módulo é comparado com o recorte dele na base, e um pacote publicado que some de todos os módulos reprova. O `buf-pins` exige o mesmo `protoc-gen-go` em todos os `buf.gen.yaml`. A marca de baseline passa a `contracts-baseline/<projeto>`; a legada `contracts-baseline/proto` só vale para o módulo cuja raiz existe no commit da tag.
4. **Deploy por app.** Cada app tem `deploy/` com `k8s/{base,overlays/{dev,hmg}}`, `compose.yml` (por `extends` dos serviços-base de `infra/local/compose/app-base.yml`, porque anchors não atravessam o `include`), `.env.example` e `infra.json`, que declara banco, tópicos, ACLs, certificado, imagem, gRPC, OpenAPI e ambiente. O `infrasync --write` gera a infra agregada (provisionamento, PKI, Swagger UI, `infra/local/.env.example`, listas dos overlays e o Job de bancos do `dev`), e o CI roda `infrasync --check`. O gate de contexto valida os manifestos. A named input `go` exclui `{projectRoot}/deploy/**`.
5. **Runtime local no host.** `pnpm nx run bff:serve` sobe a infra de runtime, espera o provisionamento e dispara o `serve-*` de todos os contextos no host; parar o serve derruba a infra (`tools/infra-session.sh`), com os volumes preservados. O Postgres usa o banco e as credenciais padrão; cada app cria o próprio banco e as próprias tabelas. A depuração linha a linha usa as configurações `dlv` do `.vscode/launch.json`. O projeto Compose é `local`, e o Job e o Secret de bancos, o volume de PKI e a CA local não levam `dmpf` no nome.
6. **Observabilidade local completa.** `TRACE_SAMPLE_RATE` amostra traces e logs de toda classe pela mesma taxa (erro fica sempre em 1), `LOG_LEVEL` fixa o nível mínimo e `OTLP_LOGS=true` exporta os logs por OTLP; o runtime local declara `1`, `debug` e `true`. O Collector ganhou o pipeline de logs para o Loki. O app em container continua em stdout, coletado pelo Alloy, para que nenhum registro seja gravado duas vezes. Cada operação gera um registro `DEBUG` de sucesso: requisição HTTP do `bff`, chamada gRPC servida, chamada de transporte e registro Kafka processado; a falha segue em `WARN`, sem mensagem.
7. **Infra de testes separada.** `infra/test/compose.yml` (projeto `testinfra`) sobe Postgres, Redpanda, Redpanda com SASL e floci só em tmpfs, nas portas 15432, 19092, 19093 e 14566, com `pnpm nx run testkit:test-infra-up` e `test-infra-down`. O `.env.example` da raiz aponta para ela, e os targets de integração rodam por `tools/test-env.sh`, que só preenche as variáveis que o ambiente não declarou. Cada teste ganha um banco `<projeto>_test_<id>`, criado na primeira chamada e apagado com `DROP DATABASE ... WITH (FORCE)` ao fim, e o `tb/pg` recusa qualquer servidor cujo `cluster_name` não seja `test` — o do CI inclusive. Os testes apagam os grupos Kafka e as inscrições SNS que criam, e os recursos de teste perdem o prefixo `dmpf` (`it-`, `e2e-`, `distkit-`, `TESTKIT_*`, `TB_*`, `EVIDENCE_DIR`, usuário SASL `ci`).
8. **Release por contrato.** Cada contrato tem o grupo `go-contract-<ctx>` com tag literal `apps/backend/<ctx>/contract/v{version}`, porque o Nx Release não tem placeholder de diretório; o `contracts` segue no `go-libs`, só com o kernel. A DMPF-B012 cobre as entradas `subject: contract` do BOM pela mesma derivação de tag do kernel.
9. **Generator.** O generator `bounded-context` emite, ao lado do módulo do app, o módulo `contract/` (com um `doc.go` na raiz, porque um módulo sem Go reprova no `vet` e no `lint`, e a unidade `<bc>/contract` declarando a raiz e o pacote do serviço) e o `deploy/`, registra os dois módulos no `go.work`, acrescenta o release group do contrato no `nx.json` e o `include` do compose do app no compose local, e roda `dmpf-modsync` e `infrasync` no callback. A porta gRPC do processo no host é a opção `grpcPort`. O bloco `contract` passado em `--blocks` é aceito sem efeito.
10. **Targets do workspace que cumprem o propósito.** O `tidy` inferido pelo `@nx-go/nx-go` ignora o `go.work` e buscava a tag `v0.1.0` de cada irmão, ainda não publicada; cada `project.json` Go declara o `tidy` por `tools/go-tidy.sh`, que põe os `replace` do `go.work` no `go.mod` só durante o `go mod tidy` e os retira ao fim, e o `go.mod` publicado segue sem `replace` (ADR-047). O `generate` roda em `./...`, o `docker:build` usa a raiz como contexto com o Dockerfile do app, e o `evidence` grava em `dist/evidence/<commit>`.

## Desvios da spec

| Spec | Implementado | Motivo |
| ---- | ------------ | ------ |
| `deploy/infra.yaml` | `deploy/infra.json` | O `infrasync` lê com a biblioteca padrão do Go, sem dependência de YAML |
| Template de secrets do `hmg` gerado | Um `secrets.example.yaml.tmpl` por overlay de app | O Secret é do app e fica no diretório dele; não há lista a sincronizar |
| Editar `deploy/` não torna o app afetado | Não invalida o cache Go, mas o app continua afetado | O `nx affected` marca o projeto por arquivo tocado, independentemente da named input |
| ADR-053, item 5: banco `<projeto>_test` compartilhado | Banco por teste, apagado ao fim | Nada que um teste cria sobrevive a ele |
| ADR-053, item 10: tooling de teste mantém `DMPF_` | `TESTKIT_*`, `TB_*`, `EVIDENCE_DIR` | Os recursos e as variáveis de teste não levam o nome do framework |

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Contrato dentro do módulo do app | `bff` e `reservations` importariam o app inteiro (ADR-044) |
| Workspace Buf único com vários módulos | A configuração continuaria fora do app, e o gate dispararia para todos a cada mudança |
| Ler os manifestos em tempo de execução com `yq` | O que roda em Compose e Kubernetes deixaria de ser revisável no diff |
| Testes contra a infra de runtime, com bancos de nome fixo | Os testes apagavam dados do runtime e deixavam resíduo a cada execução |
| Exportar por OTLP também o log do app em container | O Alloy já coleta o stdout; o Loki guardaria cada registro duas vezes |

## Consequências

**Positivas:**

- Um contexto novo nasce com contrato e deploy, e nenhuma lista compartilhada é editada à mão.
- O kernel não conhece nenhum app, nem nos testes.
- Uma execução de teste não deixa banco, tópico, grupo, fila ou inscrição para trás, e nunca toca o runtime local.
- No runtime local, toda operação aparece com log, trace e métrica.

**Negativas:**

- São três módulos Go e três projetos Nx a mais, cada um com a sua cadeia.
- Os testes de integração locais exigem `testkit:test-infra-up` antes.
- O diretório temporário de registros do `dmpf-evidence` não é removido pelo próprio comando.
