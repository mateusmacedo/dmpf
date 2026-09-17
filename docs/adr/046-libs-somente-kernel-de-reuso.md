# ADR-046: Reservar `libs/backend/go` ao kernel de reuso, com os contextos de exemplo e o golden como apps e o verificador como tooling

## Status

Aceito — 2026-09-16. Evolui o ADR-044 e o ADR-045 e supersede parcialmente os dois: do ADR-044, a frase "os contextos permanecem em `kernel`" — `orders` e `reservations` passam a `bounded_context` próprios; do ADR-045, o caminho `libs/backend/go/<contexto>` do módulo único por contexto — o módulo vive em `apps/backend/<contexto>`, como app — e o alcance da exceção de tooling, que passa a cobrir o verificador. A topologia de três composition roots e seis processos (ADR-044), o nome bare, o alias pelo papel e o módulo único por contexto (ADR-045) permanecem.

## Contexto

`libs/backend/go` reunia quatro coisas de natureza distinta sob a mesma raiz:

- o **kernel de reuso** — as unidades de `shared_kernel_units` do baseline (ADR-042): `domain`, `ports`, `application`, `postgres`, `app`, `observability`, `transport`, `grpc`, `http`, `kafka`, `sqs` e `contracts` — e o `testkit`;
- os **exemplos** do kernel, espalhados como subpackages `example/` dentro dos módulos de produção: `domain/example/{orders,reservations}`, `application/example/{orders,reservations}`, `postgres/example/{orders,reservations}` e `app/example/reservations`, todos com `bounded_context` `kernel`;
- o **golden** do harness de bounded contexts, `bookings` (`resource-scheduling`), um contexto de negócio completo registrado como `type:lib`;
- o **tooling** de workspace, `conformance`, que carrega os binários `cmd/conformance`, `cmd/bom` e `cmd/modsync` e que nenhum contexto importa em produção.

Três efeitos dessa mistura já tinham nome no código:

1. **As libs dependiam dos exemplos para testar.** `testkit/appkit` e `testkit/distkit` só provavam `reservations` — `NewReservations`, SQL em `dmpf_example_reservations`, `RawOrderPlaced` de `orders`, `Effects.Reservations`, `Plan{Order, Items}` —, e `testkit/domainkit/examples_test.go`, `observability/usecase/orders_{integration,signals}_test.go`, `postgres/outbox_test.go` e o teste de `RES-25` de `resilience` exercitavam `orders`. Quem consumisse o kernel sem os exemplos não compilava a suíte das próprias libs.
2. **`application/example/memory` era híbrido.** A realização em memória da unit of work (ADR-034) é genérica — `Within`, inbox, outbox, `FixedClock`, `SequenceIDs` —, mas expunha `tx.Orders()` e `tx.Reservations()` tipados pelos agregados de exemplo, e vivia como unidade `provider` (`kernel/example-memory`) dentro do módulo `application`. Por estar sob `**/application/**`, precisava de uma exclusão no `.golangci.yml` (`path: application/example/memory/`) e de uma entrada em `FORA_DO_DEPGUARD` no `tools/dmpf-gate-check.sh` para que o `depguard` do bloco `application` não a reprovasse.
3. **Os contextos de referência estavam repartidos.** O ADR-044 pôs os composition roots em `apps/backend/{orders,reservations}` e deixou domínio, aplicação e provider em `libs/.../example/`, sob o `bounded_context` `kernel`. O verificador provava por `DMPF-D002` que o `bff` não importava domínio dos exemplos, mas não distinguia `orders` de `reservations` entre si: `reservations` importava `ordersapp.Destination` e o relay e2e usava a aplicação de `orders` sem que nenhuma condição de contexto fosse avaliada.

Somava-se o efeito no release: `bookings`, `orders` e `reservations` carregavam `type:lib` ou viviam dentro de módulos `type:lib`, e o `nx release` (`release.projects: tag:type:lib`) versionava exemplo como se fosse lib de reuso.

## Decisão

**Em `libs/backend/go` fica só o que um contexto novo importa.** Quatorze módulos: os doze do kernel designados como shared kernel, o `testkit` genérico e o `memory`, novo. O critério é o consumo: sai o que é um contexto — exemplo ou golden — e o que é ferramenta. Os subdiretórios `example/` de `domain`, `application`, `postgres` e `app` deixam de existir.

**`memory` é módulo próprio, unidade `kernel/provider-memory`, designado shared kernel.** A parte tipada pelos exemplos foi substituída por um descritor genérico:

```go
type Table[ID comparable, S any] struct { Name string; Clone func(S) S }
func (t Table[ID, S]) Reader(s *Store) ports.Reader[ID, S]
func (t Table[ID, S]) Repository(tx *Tx) ports.Repository[ID, S]
```

`Name` é a tabela lógica — a contraparte da tabela SQL do `postgres` — e pareia `Reader` e `Repository` sob o mesmo nome; `Clone` é chamado em toda travessia da fronteira (`Load`, `Save`, abertura e commit), porque a garantia do `doc.go` — "snapshots sem backing array compartilhado" — exige uma cópia profunda que o módulo não consegue derivar de um `S` opaco. `nil` significa que a cópia por valor basta. O resto da superfície (`New`, `NewUnitOfWork[R]`, `*Tx`, `Store`, `FixedClock`, `SequenceIDs`, os erros) não muda. Fora de `**/application/**`, a exclusão do `.golangci.yml` e a entrada em `FORA_DO_DEPGUARD` deixam de ter objeto e foram removidas. O módulo entra em `shared_kernel_units` porque é a contraparte sem banco do `postgres`, já designado, e todo contexto o usa em teste; o `test-race` roda sem `-tags=integration` e com cache. O ciclo `memory ↔ testkit` nos `go.mod` é o mesmo de `application ↔ testkit`, resolvido pelo `go.work`.

**`orders` e `reservations` são bounded contexts completos em `apps/backend`, com `bounded_context` próprio.** Cada um segue o layout do `bookings`: `domain/`, `application/`, `provider/` e o bloco `app` no package raiz, com `rpc/` e `cmd/`. As unidades são `orders/{domain,application,provider-postgres,app}` e `reservations/{domain,application,provider-postgres,app}`; não há `ports/` de contexto, porque as portas são todas do kernel. Onde o kernel e o contexto colidem no nome, o kernel recebe o alias pelo papel — `kernel` para `domain`, `usecase` para `application` —, como o ADR-045 fixou. Os prefixos internos de erro passam a `application:` e `provider:`; as strings visíveis no fio (`orders.events`, `reservations.Reservation`, `com.company.orders.*`) não mudam. Os dois cruzamentos entre os contextos foram desfeitos sem import: `reservations` declara a própria constante `ordersDestination = "orders.events"`, e o relay e2e produz o lado de `orders` pelo contrato público (`company/orders/event/v1`) com um mapper local de teste. A partir daqui o verificador decide `DMPF-D002` entre `orders` e `reservations`, o que antes era invisível por ambos serem `kernel`.

**`appkit` e `distkit` são harness do exemplo e vão com `reservations`.** Não sobrou nada genérico neles: `appkit` é o harness de `reservations` sobre Postgres e `distkit` é o de dois processos OS sobre Redpanda para o mesmo exemplo. Os candidatos a extração (`Process`, `WaitFor`, `createTopics`, `Role`, `Verdict`) são métodos ou glue do `Harness` desse plano, e desenhar um harness independente de plano sem um segundo consumidor é abstração sem cliente. Ficam como unidades próprias, `reservations/appkit` e `reservations/distkit`, no lugar de `kernel/testkit-app` e `kernel/testkit-dist`, para que o harness continue distinto do composition root no baseline e nos subjects da evidência; o target `test-distributed` acompanha. Para que os dois, agora em outro contexto, continuem a importar `testkit/tb`, `tb/pg`, `clock` e `ids`, as unidades `kernel/testkit-tb`, `kernel/testkit-clock` e `kernel/testkit-ids` passam a `public_integration_surface: true` — o mecanismo do manifesto, permitido fora de `domain` (`DMPF-M002`) e com o precedente de `conformance/fitness` —, e não a designação como shared kernel, que é ato de baseline maior do que o necessário.

**As libs testam com fixture própria.** `testkit/domainkit`, `testkit/serviceskit` e `observability/usecase` definem, cada um no próprio `_test.go`, um agregado `counter` — só domínio no primeiro, com caso de uso no segundo, instrumentado no terceiro. A fixture fica em `_test.go` e não em `internal/` de produção porque um package de produção exigiria unidade no manifesto (`DMPF-U001`). O `domainkit` continua fixture-driven (`ORA-30`, `ORA-39`), com o golden em `testdata/counter.golden`. `postgres/outbox_test.go` mantém o contrato `eventv1.OrderPlaced` do `contracts` — é kernel — e troca só o evento de domínio do exemplo por um tipo local; `resilience` usa uma tabela `ledgers` mínima. A evidência registra `domain/counter` no lugar de `orders`.

**`contracts` fica em `libs`, mas o contrato de cada contexto é unidade do contexto.** O bloco `contract` é superfície pública por construção (`rule.PublicIntegrationSurface`), então a colocação física em `libs` não abre nenhum contexto; o que importa é quem é o dono. `contracts/gen` (`kernel`) fica só com `gen/go/io/cloudevents/v1`; `orders/contract` e `reservations/contract` são unidades novas, com `bounded_context` próprio e `public_integration_surface: true`, espelhando `resource-scheduling/contract`, e herdam as exceções nominais de `reflect` e `unsafe` do ADR-033. Mover o gerado para dentro de `apps/backend/orders` faria o `bff` e o `reservations` importarem um app e tiraria o gerado do módulo que os gates do Buf governam (`buf-generate-check` sobre `gen/go`).

**`conformance` é tooling e vive em `tools/dmpf-conformance`.** Não é kernel de reuso — o único consumidor fora dele é o `testkit/fitness`, pelo package exportado `fitness` — e não é app. O diretório mantém o prefixo `dmpf-` pela exceção de tooling do ADR-045 (`tools/dmpf-plugin`, `tools/dmpf-baseline`, `tools/dmpf-*.sh`): `tools/` não é segregado por stack e o prefixo é o que distingue o tooling do DMPF de outro que o workspace venha a ter. O projeto Nx continua `conformance` e o pacote npm privado é `@mateusmacedo/dmpf-conformance`, como o plugin. As tags passam a `type:lib`, `scope:shared`, `stack:go`, sem `layer:*`, espelhando o plugin; por isso o `ci.yml` exclui `conformance` do check "toda lib Go declara exatamente uma camada" e ganha o step `Tooling Go — conformance`, logo após o estágio 2 e antes dos gates que o executam, com o mesmo `fmt-check`, `vet` e `test-race` dos estágios. Os binários rodam por `go run ./tools/dmpf-conformance/cmd/{conformance,bom,modsync}`; `tools/dmpf-cell-check.sh` e `tools/dmpf-shared-kernel-check.sh` apontam para lá, e os testes do módulo que montavam o caminho antigo por segmentos (`selfcheck_test.go`, `baseline_repo_test.go`, o `repoRoot` de `fitness_test.go`) foram ajustados. A fixture não designada do `dmpf-shared-kernel-check.sh` passa de `kernel/example-orders` a `kernel/testkit-domain`: mesma exigência do vetor — bloco `domain`, fora de `shared_kernel_units`, no mesmo `bounded_context` `kernel` da designada.

**`bookings` é app.** `apps/backend/bookings`, `projectType: application`, `type:app`, `layer:apps`. Um contexto de negócio é produto, não lib de reuso: não é versionado pelo `nx release` nem publicado. Mantém o `package.json` privado, como `bff`, `orders` e `reservations`; não ganha target `serve`, porque não tem `main`. O generator `bounded-context` gera em `apps/backend` (`DEFAULT_DIRECTORY` e default do schema), emite `type:app` incondicionalmente e o README do template acompanha.

**O baseline foi regravado.** É ato de classificação (ADR-028, `AUT-01`): 31 entradas removidas, 32 adicionadas — as de `bookings`, `orders`, `reservations`, `orders/contract`, `reservations/contract`, `kernel/provider-memory` e as quatorze de `conformance/*` nos caminhos novos —, `contracts/gen` com membership reduzida a `io/cloudevents/v1`, e `shared_kernel_units` com `kernel/provider-memory` a mais e nada a menos. O baseline fecha com 60 entradas e digest `sha256:4e69048f…`. O `modsync --write` sincronizou o `go.work` (19 módulos, 14 `replace`) e os `require` entre irmãos; `--check` responde `conforme`.

**As tabelas `dmpf_example_*` não mudam de nome.** O schema de `orders` e `reservations` segue em `libs/backend/go/postgres/schema.sql`, aplicado por `postgres.Migrate`; renomear tabela é migração de dados e cabe a uma história própria, como o `migrate.go` do `bookings` já indica o caminho.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Manter os exemplos em `libs` sob `example/` | As libs dependiam dos exemplos para testar, o `nx release` versionava exemplo como lib e `DMPF-D002` entre `orders` e `reservations` nunca era decidido |
| Manter só o composition root em `apps` (estado do ADR-044) | O contexto fica repartido entre dois diretórios e dois `bounded_context`, e um contexto novo não tem exemplo inteiro para copiar |
| `tools/conformance`, sem prefixo | Contraria a exceção de tooling do ADR-045; `tools/` não é segregado por stack e o prefixo é o que distingue o tooling do DMPF |
| `conformance` como app em `apps/backend` | Não é produto nem roda como serviço; entraria no estágio 5 e nos filtros de release |
| Gerado de cada contexto em `apps/backend/<contexto>` | `bff` e `reservations` importariam um app, e o gerado sairia do módulo que os gates do Buf governam |
| Contratos de `orders` e `reservations` dentro de `contracts/gen`, em `kernel` | O ADR-017 exige que a interação entre contexts passe por contrato de integração; deixar o contrato no contexto `kernel` esconde quem é o dono |
| Extrair um harness genérico de `appkit`/`distkit` | Sem segundo consumidor; abstração sem cliente |
| Fundir `appkit`/`distkit` no include de `reservations/app` | Perde a granularidade no baseline e nos subjects `app` e `dist` da evidência |
| Designar o `testkit` inteiro como shared kernel para liberar o harness | Ato de baseline maior que o necessário; a superfície pública em três unidades resolve |
| Fixture das libs em `internal/` de produção | Exigiria unidade no manifesto e classificação; em `_test.go` não é unidade |
| `memory` com funções livres `Repository[ID, S](tx, name)` e `Reader[ID, S](s, name)` | Repetiria nome e clone em cada call site, e um nome divergente entre reader e repository faria o reader não ver as linhas em silêncio |
| `bookings` como `type:lib` em `apps` | O `nx release` versionaria um app, contra a taxonomia em que `type:app` é o que roda |
| Renomear `dmpf_example_*` junto | Migração de dados fora do escopo de uma reorganização de árvore |

## Consequências

**Positivas:**

- `libs/backend/go` tem quatorze módulos, todos consumíveis por qualquer contexto, e nenhum teste de lib importa `apps/`.
- `DMPF-D002` passa a ser decidido entre `orders` e `reservations`; as arestas entre eles só existem pelo contrato.
- Um contexto novo tem três exemplos completos com o mesmo layout — `bookings`, `orders` e `reservations` — e o generator gera essa mesma forma.
- `go build ./...` passa nos 19 módulos do `go.work`; o verificador não emite nenhum `DMPF-D*`, `U*` ou `E*` sobre a árvore.

**Negativas:**

- **Custo aceito:** o baseline regravado e todos os `dmpf-units.json` alterados vão em commit próprio, separado do commit de código. Enquanto a mudança estiver só na árvore de trabalho, `conformance --base develop` devolve `DMPF-T002: mudança normativa sem histórico para avaliar o commit próprio` — o intervalo `develop..HEAD` está vazio — e só julga quando o commit normativo existir isolado.
- **Custo aceito:** o BOM `bom/dmpf/0.1.0.json` e a evidência `bom/evidence/0.1.0/` seguem como registro da combinação certificada. O validador aceita a árvore (`dmpf-bom --release latest --base develop`: `conforme`, sem tocar o arquivo), mas a evidência cita `dmpf-testkit/appkit`, `dmpf-testkit/distkit`, `dmpf-application/example/memory` e `apps/backend/dmpf-reference`, caminhos que este ADR e o ADR-045 removeram; a divergência dura até a próxima release regenerar a evidência por `testkit:evidence`. O catálogo do `cmd/evidence` já aponta para os caminhos novos, o subject `reference` cobre `bff`, `bookings`, `orders` e `reservations`, e `appkit` e `distkit`, sob `apps/backend/reservations/...`, ficam sobrepostos aos subjects `app` e `dist`.
- **Custo aceito:** os scripts que abrem worktree em `HEAD` — `dmpf-harness-check.sh`, `dmpf-shared-kernel-check.sh`, `dmpf-cell-check.sh` e `dmpf-generator-check.sh` — só voltam a passar depois do commit: em `HEAD` o verificador ainda está em `libs/backend/go/conformance` e o `build-profiles.json` não é encontrado em `tools/dmpf-conformance/`. Os vetores das células 26 e 12 foram provados contra a árvore de trabalho, com `DMPF-D001` emitido nos dois.
- O `ci.yml` ganha um step só para o `conformance` e o check de camadas o exclui; um segundo módulo de tooling Go precisaria do mesmo tratamento.
- Quem usa o `memory` em teste passa a declarar um `Table` por agregado no próprio package de teste, e os `rpc/testing_test.go` de `orders` e `reservations` importam o `domain` do contexto para tipá-lo.
- O `pnpm-lock.yaml` troca os importers `libs/backend/go/conformance` e `libs/backend/go/bookings` por `libs/backend/go/memory` e `tools/dmpf-conformance`.
- As tabelas `dmpf_example_orders` e `dmpf_example_reservations` seguem com o nome antigo, e os harnesses continuam a truncar as duas.

## Referências

- ADR-017, ADR-028, ADR-030, ADR-031, ADR-033, ADR-034, ADR-040, ADR-042, ADR-044 e ADR-045.
- SPEC-ACYKBF9V (topologia do ADR-044) e SPEC-AHPRBZCT (`bookings`, o golden do harness).
