---
id: SPEC-7SRQ3RPK
slug: unicidade-como-rejeicao
title: DMPF — Violação de unicidade declarada traduzida em Rejection de domínio
stage: done
priority: P2
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-103/spec-7srq3rpk-dmpf-violacao-de-unicidade-declarada-traduzida-em
subtask_urls: []
created: 2026-10-12
---

# SPEC-7SRQ3RPK: Unicidade declarada como rejeição de domínio

## Resumo

Dar ao `postgres.Table` a declaração de restrições únicas de negócio, cada uma ligada a
uma rejeição de domínio com código estável, e fazer o kernel traduzir a violação dessa
restrição, descoberta no `Save`, em `Rejected` pelo caminho normal do desfecho. Hoje a
duplicata que passa pela corrida volta como erro do driver e termina `Unexpected`. Com a
spec, ela chega ao chamador como `DomainRejection`, fica gravada na inbox como recusa
(R3) e é reproduzida no replay sem nova auditoria. O índice único continua sendo a
única garantia atômica; a tradução nunca depende de pré-checagem.

Origem: consumidor externo antecipa-market, SPEC-7KW0HHF6 (DEVS-77), que precisa de
unicidade por tenant em `organizations(tax_id_hash, jurisdiction)`,
`establishments(organization_id, tax_id_hash)` e `persons(document_hash)`, devolvida
como `identity-organization/organization/duplicate-tax-id` e afins. O ADR-022 do
consumidor manda a extensão nascer no kernel (regra 3) e registra a divergência na
linha 15 do addendum de 2026-10-12.

## Contexto

### Estado atual

Verificado no código de `lidercap-platform` em 2026-10-12. Os caminhos `.go` são
relativos a `libs/backend/go/`.

| Ponto | Evidência |
| ----- | --------- |
| A criação usa `ON CONFLICT (tenant_id, <id>) DO NOTHING`, e só o conflito de PK vira `ErrVersionConflict` | `postgres/table.go:182-187,343-344` |
| A violação de outro índice único volta como erro do driver embrulhado, com a transação abortada | `postgres/table.go:339-342` |
| `CategoryOf` não reconhece a violação, que fecha em `Unexpected`; o gRPC a traduz em `INTERNAL` | `application/failure.go:69-85`; `grpc/status.go:62-90` |
| `ErrAlreadyExists` cobre só a mesma identidade, no loader `Absent`, e cai em `Conflict` | `application/decide.go:45-56`; `ports/repository.go:32-35`; `application/failure.go:77-79` |
| `Decide` grava antes de enfileirar: o `Save` é a primeira escrita do passo | `application/decide.go:86-91` |
| A recusa é gravada na inbox na mesma transação, e o replay não audita | `application/idempotent.go:133-148`; `application/execute.go:51-64,72-74` |
| A UoW é uma transação `READ COMMITTED`, desfeita por inteiro quando o callback erra | `postgres/uow.go:39,49-56` |
| A pré-checagem não é atômica: `Relation.Query` só roda no `ReadPool`, fora da transação | `postgres/table.go:41-44,73,101-107` |
| Precedente do kernel: a outbox já classifica o SQLSTATE `23505` sem perder o `*pgconn.PgError` | `postgres/outbox.go:17,81-85,136-139` |
| O span do banco guarda só o SQLSTATE, porque `Detail` cita os valores da linha | `postgres/dbtrace.go:109-114` |
| O span reconhece só `BEGIN`, `COMMIT` e `ROLLBACK` como controle de transação | `postgres/dbtrace.go:44-51` |
| O provider de contexto não importa o driver (gate `context-provider`) | `.golangci.yml:431-452` |
| O golden `bookings` não tem invariante de unicidade: só PKs e nenhuma rejeição de duplicata | `apps/backend/bookings/provider/schema.sql:5-32`; `apps/backend/bookings/domain/rejections.go:5-9` |

### Fatos do PostgreSQL e do pgx

Sondados nesta redação no PostgreSQL 16.15, imagem `postgres:16-alpine`, a mesma de
`infra/test/compose.yml:10`:

- A violação `23505` aborta a transação: o comando seguinte falha com `25P02`. Depois de
  `SAVEPOINT` e `ROLLBACK TO SAVEPOINT`, a transação segue utilizável e commita.
- O erro traz `CONSTRAINT NAME` e `TABLE NAME`, tanto para `CONSTRAINT ... UNIQUE` quanto
  para `CREATE UNIQUE INDEX`; o `DETAIL` traz os valores, inclusive o hash.
- Um `INSERT` aceita um só `ON CONFLICT`: dois dão `42601`. `ON CONFLICT DO NOTHING` sem
  alvo devolve `INSERT 0 0` sem dizer qual restrição conflitou, e `UPDATE` não tem
  `ON CONFLICT`. O `ON CONFLICT` da PK, usado hoje, não absorve a violação de outra
  restrição única.
- O mesmo valor em outro tenant passa, porque a restrição começa por `tenant_id`.
- pgx v5.10.0: `Tx.Begin` sobre uma transação abre `savepoint sp_N`, e `Commit` e
  `Rollback` emitem `release savepoint` e `rollback to savepoint` (`tx.go:164-171,309,322`
  no cache de módulos); `pgconn.PgError` expõe `Code` e `ConstraintName`
  (`pgconn/errors.go:35,47`).
- Numa tabela particionada, o `23505` traz `TABLE NAME` e `CONSTRAINT NAME` da partição
  folha (`devs103_parted_acme`, `devs103_parted_acme_tenant_id_name_key`), não os da
  tabela declarada.
- pgx v5.10.0, verificado na implementação: o modo de execução vale para o batch
  inteiro (`DefaultQueryExecMode` da conexão), e os modos que preparam ou descrevem o
  fazem antes de enviar o batch (`conn.go:1170`). Um prepare recusado antes do
  `SAVEPOINT` abortaria a transação com `25P02`.

### Custo medido

Medido na implementação, contando as escritas no socket por `Save`:

| Caso | Cada statement sozinho | Batch |
| ---- | ---------------------- | ----- |
| Criação ou atualização com `Unique`, conexão quente | 3 | 1 |
| Primeiro `Save` de cada escrita numa conexão, modo `CacheStatement` | — | 5, uma vez |
| Primeira criação recusada numa conexão, modo `CacheStatement` | — | 4 |
| Criação recusada seguinte na mesma conexão | — | 3 |
| `Save` seguinte à recusa | — | 1 |

Com conexão quente, `Exec` e `SimpleProtocol` também pagam 1; `CacheDescribe` paga
2, e `DescribeExec`, 3, porque mandam o `SAVEPOINT` sempre separado. O primeiro
`Save` paga mais porque envia o `SAVEPOINT` sozinho e prepara por nome a escrita, o
`SAVEPOINT` e o `RELEASE`, fora do LRU do cache de statements; o nome fixo do
savepoint é o que deixa preparar cada statement uma vez por conexão. Os statements
preparados por nome não são invalidados quando um batch falha, e o `Save` seguinte à
recusa volta a pagar 1. A criação recusada paga o batch, o `ROLLBACK TO SAVEPOINT` e a
sonda de identidade; na primeira recusa da conexão, a sonda é preparada e executada em
duas escritas, e nas seguintes, em uma: três criações recusadas seguidas medem
`[4 3 3]`.

O `BenchmarkSave` (`postgres/unique_bench_test.go`) grava na mesma tabela,
`unique_probes`, declarada com e sem `Unique`. Como o `-count` não intercala os
sub-benchmarks, o binário de teste rodou 12 vezes com `-test.count=1`, alternando os
dois casos na mesma execução, e comparam-se as medianas: 171 µs sem e 195 µs com
`Unique`, razão 1,14 (mediana das razões por rodada, 1,15). Na versão anterior ao
batch, o revisor mediu 177 e 356 µs (2,0). A máquina rodava outras cargas, e só a
razão compara. O teste `TestASaveUnderTheSavepointCostsOneRoundTrip` prende a escrita
única no socket com a conexão quente, e `TestEveryQueryExecModeSavesUnderTheSavepoint`,
o número de cada modo.

### Tensão normativa

A norma classifica hoje a unicidade como `Conflict`, não como `DomainRejection`:
`DomainRejection` é «a UPR decidiu `Rejected`» (`docs/dmpf/contexto-erros-seguranca.md:998`),
o predicado de `Conflict` exclui o «conflito de unicidade — o recurso já existe» (`:1041`)
e a matriz manda `ALREADY_EXISTS` na unicidade (`:1198`, rationale em `:1233-1238`). A
spec separa duas unicidades: a de **identidade** (o identificador já existe), que segue
`Conflict`, e a de **atributo declarada como regra de negócio** do contexto, que passa a
`DomainRejection`. A separação exige emenda de FND-07, FND-03 e FND-04, listada como
requisito P0 antes do código, e fica registrada no ADR-066.

### Não verificado

- O comportamento sob concorrência com savepoint (a segunda transação espera o índice e
  recebe `23505` após o commit da primeira, ou grava se ela desfizer) está provado
  contra o kernel por `postgres/unique_concurrency_test.go`, em 50 rodadas, e pelo e2e
  do `bookings` (`apps/backend/bookings/app/unique_e2e_test.go`). Segue sem prova em
  integração só a corrida de identidade que aciona a sonda, que depende da ordem em que
  o PostgreSQL insere nos índices e está provada por teste com transação falsa.
- Se a emenda de FND-07 cabe na autoridade de classificação vigente sem rito adicional.

<constraints>
- [P0] O índice único é a única garantia de unicidade: nenhuma etapa da tradução depende de pré-checagem por leitura, e a ausência de declaração nunca afrouxa o índice.
- [P0] A recusa por unicidade satisfaz DEC-10, DEC-11 e UOW-06: nada do agregado nem da outbox persiste, e a transação commita só com o registro da recusa na inbox (IDM-06).
- [P0] A UPR continua sem I/O (UPR-I07): a rejeição é declarada estaticamente pelo contexto, e quem a levanta é o serviço de aplicação, a partir do erro tipado do `Save`.
- [P0] Toda restrição declarada começa por `tenant_id` (IDN-11, IDN-14): a recusa só revela existência dentro do tenant do chamador (IDN-13).
- [P0] Nenhum valor da linha chega à rejeição, ao log ou ao span: o `DETAIL` do PostgreSQL nunca é copiado (ERR-07).
- [P0] O provider de contexto continua sem driver (`context-provider`): a declaração usa só tipos do `postgres` e do `domain`.
- [P0] A unicidade de identidade não muda: o conflito de PK segue `ErrVersionConflict` e o `Absent` segue `ErrAlreadyExists`, ambos `Conflict`.
- [P1] Tabela sem declaração de unicidade não ganha statement nem round trip.
- [P1] Nenhuma dependência de terceiros nova e nenhuma unidade nova no manifesto.
</constraints>

## Requisitos

### Funcionais

#### Onda 0 — normas e ADR (antes do código)

- [x] **[P0] Emendar FND-07**: em `docs/dmpf/contexto-erros-seguranca.md`, separar a unicidade de identidade (`Conflict`, `409`, `ALREADY_EXISTS`) da unicidade de atributo declarada como regra de negócio (`DomainRejection`, `422`, `FAILED_PRECONDITION`, `R1×D2`), nas linhas `:998`, `:1041`, `:1198` e no rationale `:1233-1238`, com ajuste neutro em TX-2 (`:443`, `:508`) e no ERR-03 (`:969`). Nenhuma categoria nova (ERR-08). Emenda in-place, neutra em número de linhas (o `dmpf-verify` prende linhas físicas).
- [x] **[P0] Emendar FND-04 e FND-03**: o FND-04 é o dono da regra. UOW-05 e UOW-06 (`docs/dmpf/uow-inbox-outbox.md:516,526`) admitem, sob unicidade declarada, o passo 6 tentado e desfeito sem efeito observável, com a transação viva até o passo 8; a norma enuncia a propriedade, não o savepoint (molde do INB-04), e o D2 de §6.4, o §8.2 e o índice §11.2 acompanham. O FND-03 só reflete o efeito no `Decide` (§3.6, §4.4, §5.2). DEC-03, DEC-09 e UPR-I07 (`upr-decision-mensagens.md:278`) ficam intactos, porque a âncora do FND-03 normatiza só dentro do bloco `domain library` (M4); IDM-06 (`uow-inbox-outbox.md:1737`) também. Emendas in-place, sem ID novo e neutras em número de linhas.
- [x] **[P0] ADR-066**: registra a separação das duas unicidades, o savepoint por `Save`, a guarda de escrita anterior, a declaração no `Table`, a verificação na partida e as emendas aos ADR-018 (`:38-47`, origem da rejeição), ADR-032, ADR-034 (savepoint dentro de `Within`, ainda uma transação, UOW-01) e ADR-051 (`:47`, o `Table` cobre a unicidade declarada). Passa em `node tools/adr-verify.mjs --from 63` e em `node tools/adr-verify.mjs`, e entra em `docs/adr/README.md`.

#### Onda 1 — kernel

- [x] **[P0] Erro tipado em `ports`**: `ErrUniqueViolation` e `UniqueViolation{Table, Constraint string; Rejection *domain.Rejection}`, que desembrulha para a sentinela. `Rejection` nulo significa «não convertível». O godoc do contrato de `Save` (`ports/repository.go:49-52`) passa a dizer que a violação declarada não grava nada e deixa a transação utilizável.
- [x] **[P0] Declaração no `postgres.Table`**: campo `Unique []postgres.Unique{Name, Columns, Rejection}`. `validate` (`postgres/table.go:196-215`) recusa nome ou coluna fora de minúsculas, nome diferente de `<tabela minúscula>_<col1>_<colN>_key` do ADR-053 (`docs/adr/053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md:22`) ou acima de 63 bytes, menos de duas colunas, primeira coluna diferente de `tenant_id`, rejeição nula, código que não passe em `domain.Code.Valid` (`domain/rejection.go:14-18`) e nome repetido.
- [x] **[P0] Tradução no `Save`**: só nas tabelas com `Unique`, o `Save` envia `SAVEPOINT dmpf_unique`, o `INSERT` ou o `UPDATE` e `RELEASE SAVEPOINT dmpf_unique` num único `pgx.Batch`. O nome é fixo: savepoints de mesmo nome empilham, e `RELEASE` e `ROLLBACK TO` atingem o do próprio `Save`, acima do `sp_N` do `Begin` aninhado do pgx. No modo `CacheStatement`, o primeiro `Save` de cada escrita numa conexão envia o `SAVEPOINT` sozinho e prepara por nome os três statements; depois, o batch é um round trip. Quando a escrita falha, inclusive no prepare, a falha vem depois do `SAVEPOINT`, o servidor descarta o `RELEASE`, e o `ROLLBACK TO SAVEPOINT` deixa a transação utilizável. `23505` com `TableName` igual ao da tabela e `ConstraintName` declarado desfaz o savepoint e devolve `UniqueViolation` com a rejeição declarada; restrição não declarada devolve `UniqueViolation` com `Rejection` nulo, embrulhando o `*pgconn.PgError` no molde de `postgres/outbox.go:81-85`. No ramo de criação, depois do `ROLLBACK TO SAVEPOINT`, uma sonda da identidade declarada separa a corrida de identidade (linha presente: `ErrVersionConflict`) da rejeição. O desfazer do savepoint roda com contexto destacado do cancelamento e prazo de limpeza. O caminho sem `Unique` não muda.
- [x] **[P0] Guarda de escrita anterior**: `postgres.Tx` conta as escritas de agregado e de outbox da transação (as da inbox não contam). Se já houve uma antes da violação, o `Save` devolve `Rejection` nulo: a recusa não é convertida, o callback erra e a UoW desfaz tudo, o que preserva DEC-10 sem depender de convenção.
- [x] **[P0] Conversão em `Decide`**: o predicado exportado `application.DeclaredRejection(err) (*domain.Rejection, bool)` reconhece o `UniqueViolation` com `Rejection` não nulo. Depois de `repo.Save` (`application/decide.go:86-88`), ele vira `Rejected[R](rejection)` com `error == nil`, sem chamar `Enqueue`. `RunIdempotent` grava `StatusRejected` sem mudança (`application/idempotent.go:139-143`).
- [x] **[P0] Classificação do caso não convertido**: `CategoryOf` (`application/failure.go:69-85`) devolve `Conflict` para `ErrUniqueViolation`, e `grpc.StatusOf` responde `ALREADY_EXISTS` antes da linha que leva `Conflict` a `ABORTED` (`grpc/status.go:74`), conforme `contexto-erros-seguranca.md:1198`. Nunca `Unexpected`.
- [x] **[P0] Verificação na partida**: `Table.Uniques()` devolve a declaração sem tipo genérico, e `postgres.VerifyUniques(ctx, pool, sets...)` confere em `pg_constraint`, pela tabela resolvida em `public`, que cada restrição existe com `contype = 'u'`, na ordem de colunas declarada, começando por `tenant_id`, não é `DEFERRABLE` (a verificação adiada dispararia no `COMMIT`, depois do savepoint) nem `NULLS NOT DISTINCT`, e que as restrições de cada conjunto seguem a ordem dos seus índices no catálogo, que é a do DDL, porque sob colisão em mais de uma o PostgreSQL reporta o primeiro índice que confere. Tabela particionada, com filhas por herança, ou ela própria partição ou filha diverge (`partitioned or inherited table`), porque o `23505` da partição traz a tabela e a restrição da partição folha, e a tradução não casaria. `serve.API.Uniques`, opcional, leva os conjuntos à preparação (`app/serve/serve.go:199-221`), que a roda nos três ramos, na pool de runtime, repetindo até um prazo no ramo sem migração, e falha a partida na divergência.
- [x] **[P0] Paridade no `memory`**: `memory.Table` (`memory/store.go:15-22`) ganha `Unique` com `Key func(S) ([]string, bool)` (componentes comparados um a um; `false` quando algum é nulo, que não colide) e a mesma rejeição, a mesma guarda de escrita anterior e o mesmo erro tipado.
- [x] **[P0] Kit `providerkit.Unique`**: cláusulas em Design, rodadas contra `memory` e `postgres`; a cláusula concorrente fica `Skipped` nomeada no `memory`, que serializa `Within` (KRN-06). `UniqueConstraint.Columns` segue a convenção do `postgres.Unique`, com `tenant_id` à frente, e o subject mal formado reprova antes das cláusulas.
- [x] **[P1] Spans do savepoint**: `transactionControl` (`postgres/dbtrace.go:47-63`) reconhece `SAVEPOINT`, `RELEASE SAVEPOINT` e `ROLLBACK TO SAVEPOINT`, para o span não levar o texto como consulta nem chamar o desfazer parcial de `ROLLBACK`. O tracer implementa `pgx.BatchTracer`, também no pool com métricas: um span por statement do batch, com o nome e os atributos de sempre. Os statements estão em voo juntos: cada span começa no envio do batch e termina ao ler o seu resultado, e a duração não separa o tempo de servidor de cada statement. Um erro do servidor fica no primeiro statement sem resultado, venha no resultado de um statement ou no fim do batch, e os statements que o servidor pulou não abrem span; um erro que não vem do servidor (timeout, rede, conexão fechada) marca com `error.type=_OTHER` todos os spans ainda abertos, venha antes de qualquer resultado, no resultado de um statement ou no fim do batch, e os statements com resultado já lido ficam sem status.

#### Onda 2 — golden

- [x] **[P0] Caso no `bookings`**: por emenda da SPEC-AHPRBZCT, o agregado `Resource` ganha `name`, obrigatório (`resource-scheduling/resource/name-empty`) e único por tenant (`resource-scheduling/resource/duplicate-name`), com igualdade exata, guardado no snapshot e no `ResourceRegistered`. O `schema.sql` deriva a coluna do snapshot (`name text GENERATED ALWAYS AS (snapshot->>'name') STORED NOT NULL`) e declara `CONSTRAINT resources_tenant_id_name_key UNIQUE (tenant_id, name)` no próprio `CREATE TABLE`, com os ambientes recriados; o `resourceTable` segue `SnapshotTable` com `Unique` e entra em `provider.Uniques`. O atributo vai ao contrato Protobuf (`string name = 2` no request), ao OpenAPI e ao BFF. A borda gRPC (`INVALID_ARGUMENT`) e o BFF (`400 invalid-request`) aplicam a mesma regra antes do caso de uso: de 1 a 200 caracteres (runas) de UTF-8 válido, sem caractere `Cc`, `Cf`, `Zl`, `Zp`, `Variation_Selector` nem `Other_Default_Ignorable_Code_Point`, e com ao menos um caractere visível, que não seja espaço, marca combinante nem ignorável, sem normalizar nem aparar, para que nomes que diferem só por caractere invisível não furem a igualdade exata. Por decisão deliberada (ADR-066, item 13), todo seletor de variação é recusado, inclusive o U+FE0F depois de emoji: "❤️" é recusado, e o emoji sem seletor, aceito. Limites conhecidos: U+2800, U+FFFC e homóglifos.
- [x] **[P0] Teste concorrente**: integração (`//go:build integration`) no `postgres` e e2e no `bookings`, descritos em Verificação.
- [x] **[P0] Plugin**: o generator e os templates de IA do `tools/dmpf-plugin` emitem a declaração e `provider.Uniques`, e o `dmpf-context-check.sh` reprova `CONSTRAINT <tabela>_..._key UNIQUE` sem `postgres.Unique` de mesmo nome e `Table` com `Unique` fora de `provider.Uniques`, e toda tabela com `UNIQUE` particionada, partição, com herança ou mãe de partição ou de herança, com sabotagem no self-test.
- [x] **[P1] Documentação**: READMEs de `postgres`, `application`, `memory` e `testkit`, e a seção de persistência de `docs/guides/dmpf-consumo-externo.md`.

### Não-funcionais

- [x] Custo: tabela com `Unique` paga dois statements a mais por `Save` (`SAVEPOINT`, `RELEASE`), no mesmo round trip da escrita depois do primeiro `Save` de cada escrita na conexão; a criação rejeitada paga ainda o `ROLLBACK TO SAVEPOINT` e a sonda de identidade (medição em Custo medido); tabela sem `Unique` mantém os statements de hoje.
- [x] Compatibilidade: Go `1.26.9`, pgx `v5.10.0`, PostgreSQL 16; nenhum identificador público removido.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| `ports` | `ErrUniqueViolation`, `UniqueViolation`, godoc de `Save` |
| `postgres` e `memory` | declaração, savepoint, guarda, verificação na partida, spans |
| `application` | conversão em `Decide`, `CategoryOf` |
| `grpc` e `app/serve` | `ALREADY_EXISTS` no caso não convertido; verificação após o `Migrate` |
| `testkit` | kit `providerkit.Unique` |
| `tools/dmpf-plugin` | generator, templates de IA e gate de unicidade no `dmpf-context-check.sh` |
| Golden e normas | `bookings`; FND-03, FND-04, FND-07; ADR novo |

## Localização de código

```text
libs/backend/go/ports/repository.go                  ErrUniqueViolation, UniqueViolation, godoc de Save
libs/backend/go/postgres/table.go, unique.go          Unique, validate, Save sob savepoint, Uniques, VerifyUniques
libs/backend/go/postgres/uow.go, outbox.go            contagem de escritas em Tx
libs/backend/go/postgres/dbtrace.go                   controle de savepoint nos spans
libs/backend/go/memory/store.go                       paridade
libs/backend/go/application/decide.go, failure.go     DeclaredRejection, conversão e categoria
libs/backend/go/grpc/status.go                        ALREADY_EXISTS
libs/backend/go/app/serve/serve.go                    API.Uniques e VerifyUniques nos três ramos da preparação
libs/backend/go/testkit/providerkit/unique.go         kit
libs/backend/go/postgres/unique_concurrency_test.go   integração concorrente
apps/backend/bookings/{domain,provider,contract,app}  caso do golden
apps/backend/bff, apps/backend/load                    name no registro do recurso
tools/dmpf-plugin/{src,templates,scripts}            generator, templates de IA e dmpf-context-check.sh
docs/dmpf/{contexto-erros-seguranca,upr-decision-mensagens,uow-inbox-outbox}.md   emendas
docs/adr/066-unicidade-declarada-como-rejeicao.md     ADR-066
```

## Design

```text
Execute ─► Within (READ COMMITTED) ─► RunIdempotent: Register (inbox)
                                        └─► Run ─► Decide: load ─► UPR (sem I/O) ─► Accepted
                                                     └─► Save: batch SAVEPOINT dmpf_unique, escrita, RELEASE
                                                          ├─ ok (um round trip, conexão quente) ─► Enqueue ─► Accepted
                                                          └─ 23505 declarado: ROLLBACK TO dmpf_unique
                                                               ├─ sem escrita anterior: Rejected(rejeição declarada)
                                                               │    └─► Complete(StatusRejected) ─► COMMIT (só a inbox)
                                                               └─ com escrita anterior: erro Conflict ─► rollback da UoW
```

```go
type Unique struct {
	Name      string            // <tabela>_<colunas>_key (ADR-053), até 63 bytes
	Columns   []string          // começa por tenant_id
	Rejection *domain.Rejection // código e mensagem estáticos, declarados no domain do contexto
}

// application.Decide, logo após repo.Save:
if rejection, ok := DeclaredRejection(err); ok {
	return Rejected[R](rejection), nil
}
```

| Cláusula do kit | Prova |
| --------------- | ----- |
| `unique.rejects` | segunda criação com o mesmo valor no mesmo tenant devolve `UniqueViolation` com a rejeição declarada |
| `unique.update` | atualizar para valor já usado devolve o mesmo, e a versão do agregado não muda |
| `unique.per-tenant` | o mesmo valor em outro tenant grava |
| `unique.tx-usable` | depois da violação, um statement seguinte da mesma transação executa e commita |
| `unique.prior-write` | com escrita anterior na transação, a violação volta com `Rejection` nulo |
| `unique.concurrent` | duas criações concorrentes: uma grava, a outra recebe a rejeição (`Skipped` no `memory`) |
| `unique.first-declared` | a linha que colide em todas as restrições ao mesmo tempo recebe a rejeição da primeira declarada (`Skipped` com menos de duas) |

No consumo por mensagem, o consumidor reconhece a recusa pelo mesmo
`application.DeclaredRejection`, e ela segue a célula `R1×D2` de `DomainRejection`
(`contexto-erros-seguranca.md:1196`); a não convertida, `R1×D4`.

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Como sobreviver ao `23505` | savepoint por `Save`, só nas tabelas com `Unique` | `ON CONFLICT ON CONSTRAINT` (um só alvo, `42601` com dois, não cobre `UPDATE`); `ON CONFLICT DO NOTHING` sem alvo (não diz a restrição e exige sonda); mapear depois do rollback (a inbox não grava falha técnica, IDM-06, e o rationale de UOW-06 recusa o rollback no ramo rejeitado) |
| Pré-checagem | nenhuma | leitura no `ReadPool` fora da transação (`postgres/table.go:73,101`) ou dentro dela sob `READ COMMITTED` (`postgres/uow.go:39`): nenhuma vê a inserção concorrente |
| Categoria | `DomainRejection`, com emenda de FND-07 | `Conflict` com `ALREADY_EXISTS`: não leva código de domínio, e o chamador teria de ler texto para separar `duplicate-tax-id` de `duplicate-document` (ERR-13) |
| Onde converter | `Save` classifica, `Decide` converte | a UPR (faria I/O, UPR-I07); `RunIdempotent` (não sabe qual escrita violou) |
| DEC-10 com várias escritas | guarda de escrita anterior em `Tx`, fechada por omissão | savepoint em volta do `Run` (porta nova no `application`, que não conhece o `Tx`); convenção de um agregado por comando (IDN-14 recusa correção por convenção) |
| Onde declarar | mapeamento no `Table` e DDL no `schema.sql`, conferidos na partida | gerar o DDL do `Table`: o `schema.sql` é escrito à mão e embutido (`apps/backend/bookings/provider/schema.go:5-9`) e carrega índices de sonda que o `Table` não conhece |
| Sentinela | `ErrUniqueViolation` nova | reusar `ErrAlreadyExists`, que é de identidade (`ports/repository.go:32-35`) |
| Mensagem | estática, declarada | qualquer parte do erro do driver: o `DETAIL` cita os valores |

## Regras relacionadas

- Origem: antecipa-market, SPEC-7KW0HHF6 (DEVS-77; nota †† em `docs/specs/SPEC-7KW0HHF6-identity-organization.md:54`, invariantes em `:74,91,103`, tabelas em `:432`) e ADR-022 do consumidor (`docs/adr/022-precedencia-dmpf-sobre-design.md:7` addendum de 2026-10-12, `:29-30` regra 3, `:50` linha 15).
- `docs/dmpf/contexto-erros-seguranca.md`: IDN-11 a IDN-14 (`:878-881`), ERR-07 (`:985`), ERR-08, ERR-10, ERR-13, §5.3 (`:998-1000`), §6.2 (`:1196-1198`).
- `docs/dmpf/upr-decision-mensagens.md`: UPR-I07 (`:278`), DEC-03, DEC-04, DEC-09 a DEC-11 (`:408-409,446-448`).
- `docs/dmpf/uow-inbox-outbox.md`: UOW-05, UOW-06 (`:516-536`), INB-12 (`:1288`), IDM-06 (`:1737`).
- ADR-018, ADR-032, ADR-034, ADR-051, ADR-053, ADR-056 e ADR-066 (auditoria só sem replay, `application/execute.go:72-74`).
- SPEC-QVDGV6J0 (K4 do consumidor, SPEC-GXGASP24:145), de que depende a unicidade entre tenants; SPEC-AHPRBZCT (golden).

## Verificação e testes

### Critérios de aceite

- [x] `providerkit.Unique` passa contra `memory` (com `unique.concurrent` em `Skipped` nomeado) e contra `postgres`, e as cláusulas atuais de `providerkit.Repository` seguem verdes.
- [x] Integração concorrente no `postgres`, no molde de `postgres/inbox_concurrency_test.go:19,71`: 50 rodadas de duas transações com identificadores e chaves distintas e o mesmo valor; em cada rodada exatamente uma grava e a outra recebe a rejeição declarada, com a inbox em `rejected`, nenhuma linha de outbox e nenhum erro técnico.
- [x] E2E do `bookings`: a duplicata, concorrente ou não, responde `resource-scheduling/resource/duplicate-name` pelo desfecho, nunca `INTERNAL`; o replay da mesma chave devolve a mesma rejeição com `idempotent-replayed` e sem segundo evento de auditoria.
- [x] `VerifyUniques` reprova restrição ausente, com colunas fora de ordem, sem `tenant_id` à frente, `DEFERRABLE`, declarada fora da ordem dos índices do DDL ou em tabela particionada, com herança ou partição, e `validate` reprova declaração inválida.
- [x] Emendas e ADR aplicados; `node tools/dmpf-verify.mjs`, `node --test tools/tests/dmpf-verify/verify.test.mjs`, `node tools/adr-verify.mjs --from 63`, `node tools/adr-verify.mjs`, `go run ./tools/dmpf-conformance/cmd/conformance --root .`, `modsync --check` e `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source` passam.

### Cenários de teste

```text
DADO uma organização com tax_id_hash H e jurisdição BR no tenant A
QUANDO outra organização com H e BR é registrada em A, por outra chave de idempotência
ENTÃO o desfecho é Rejected com identity-organization/organization/duplicate-tax-id, nada grava no agregado nem na outbox, e a inbox registra a recusa

DADO duas criações concorrentes com o mesmo valor no tenant A
QUANDO as duas transações chegam ao Save
ENTÃO uma é Accepted e a outra Rejected com o código declarado, nunca Unexpected nem ABORTED

DADO o mesmo valor já usado no tenant B
QUANDO a criação ocorre no tenant A
ENTÃO grava, e nenhuma resposta nem registro ao chamador revela o dado de B (IDN-13)

DADO um Run que já salvou outro agregado na mesma transação
QUANDO o segundo Save viola a restrição declarada
ENTÃO Decide devolve erro Conflict, a UoW desfaz as duas escritas e o gRPC responde ALREADY_EXISTS

DADO uma restrição declarada numa tabela particionada ou com herança
QUANDO o contexto parte ou o gate lê o schema.sql
ENTÃO VerifyUniques devolve ErrUniqueDivergent (partitioned or inherited table) e o dmpf-context-check.sh reprova

DADO um RegisterResource com name só de espaços, só com marca combinante, com U+200B ou com U+FE0F
QUANDO chega à borda gRPC ou ao BFF
ENTÃO a resposta é INVALID_ARGUMENT ou 400 invalid-request, sem chamar o caso de uso
```

<critical_constraints>
- [P0] O índice único é a única garantia de unicidade: nenhuma etapa da tradução depende de pré-checagem por leitura, e a ausência de declaração nunca afrouxa o índice.
- [P0] A recusa por unicidade satisfaz DEC-10, DEC-11 e UOW-06: nada do agregado nem da outbox persiste, e a transação commita só com o registro da recusa na inbox (IDM-06).
- [P0] A UPR continua sem I/O (UPR-I07): a rejeição é declarada estaticamente pelo contexto, e quem a levanta é o serviço de aplicação, a partir do erro tipado do `Save`.
- [P0] Toda restrição declarada começa por `tenant_id` (IDN-11, IDN-14): a recusa só revela existência dentro do tenant do chamador (IDN-13).
- [P0] Nenhum valor da linha chega à rejeição, ao log ou ao span: o `DETAIL` do PostgreSQL nunca é copiado (ERR-07).
- [P0] O provider de contexto continua sem driver (`context-provider`): a declaração usa só tipos do `postgres` e do `domain`.
- [P0] A unicidade de identidade não muda: o conflito de PK segue `ErrVersionConflict` e o `Absent` segue `ErrAlreadyExists`, ambos `Conflict`.
- [P1] Tabela sem declaração de unicidade não ganha statement nem round trip.
- [P1] Nenhuma dependência de terceiros nova e nenhuma unidade nova no manifesto.
</critical_constraints>

## Escopo fora

- **Unicidade entre tenants**: revelaria existência em outro tenant (IDN-12, IDN-13); depende da SPEC-QVDGV6J0 e de um registro operado pela plataforma.
- **Índice único parcial, por expressão, `DEFERRABLE` e exclusion constraint (`23P01`)**: o padrão `_key` do ADR-053 e a verificação cobrem só `UNIQUE` simples.
- **Unicidade declarada em tabela particionada ou com herança**: o `23505` cita a partição folha, e a herança não estende a restrição às filhas.
- **Gerar o DDL a partir do `Table`**: spec própria.
- **Remover a pré-checagem do consumidor e migrar as rejeições dele**: spec do consumidor, depois desta.
- **Unicidade de identidade** (PK e `Absent`): continua `Conflict`, como hoje.
