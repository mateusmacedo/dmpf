# postgres

Realização em PostgreSQL das três portas transacionais que o `KRN-04` fixou em
tipos de domínio: `UnitOfWork[R]`, `Repository[ID, S]` e `Outbox`. É o bloco
`provider` do kernel DMPF, e a primeira realização que prova a sequência
canônica de FND-04 §3.2 contra um banco de verdade — a de `memory` roda
sobre mutexes e declara não provar isolamento.

Uma unidade no manifesto, `provider`, em `kernel`:

| Unidade | Diretório | O que é |
| --- | --- | --- |
| `kernel/provider-postgres` | raiz | UoW sobre `pgx.Tx`, outbox, inbox, quarantine, migração e purga |

Os repositórios e mapeadores dos agregados de referência, que este módulo
carregava em `example/orders` e `example/reservations`, são hoje o bloco
`provider` de cada contexto — `apps/backend/orders/provider` e
`apps/backend/reservations/provider` (ADR-046).

## O que o módulo contém

- **`uow.go`** — `NewUnitOfWork[R]`, `Tx` e `Within`. Uma transação por
  chamada, o callback invocado uma única vez, e as seis cláusulas do
  contrato provadas em `uow_test.go`.
- **`outbox.go`** — `Tx.Outbox(mapper)` e `Enqueue`. Grava na mesma `pgx.Tx` do
  estado de negócio: um commit torna os dois visíveis, ou nenhum.
- **`mapper.go`** — a interface `EventMapper` e a conferência de major. O
  mapeamento concreto é de cada bounded context.
- **`destination.go`** — a forma de `BLK-04`: nome de fluxo lógico em segmentos
  minúsculos separados por ponto. ARN, URL, caminho e maiúscula são recusados.
- **`schema.sql` / `migrate.go`** — os dezoito campos de FND-04 §4.1 mais
  `payload_hash`, com `UNIQUE (message_id)` e três `CHECK`; a `dmpf_inbox` de
  §6.1 com `UNIQUE (consumer_name, message_id)` e `CHECK` de dois valores
  terminais (`INB-01`, `INB-02`); a `dmpf_quarantine` com o envelope em `bytea`;
  e as tabelas `dmpf_example_orders` e `dmpf_example_reservations` dos
  contextos de referência, cujos repositórios vivem em
  `apps/backend/{orders,reservations}/provider`. `Migrate` é idempotente e não
  há tabela de versão: não existe ferramenta de migração aqui.
- **`inbox.go`** — `Tx.Inbox(consumer, wait)`, `Register` e `Pending.Complete`
  (`KRN-07`). `Register` é `INSERT … ON CONFLICT DO NOTHING` seguido de leitura:
  devolve a classificação (R1, R2, R3 ou R4), nunca erro de constraint, e a
  transação segue viva (`INB-04`). Sob concorrência a inserção aguarda a
  transação concorrente e a classificação é sempre de commit (`INB-06`,
  `INB-18`). `wait > 0` vira `SET LOCAL lock_timeout`; o estouro chega como
  `ports.ErrRegisterTimeout` (`55P03`), que o service classifica R1×D3
  (`INB-17`). Entre `Register` e `Complete` a linha existe com `status`
  provisório dentro da transação — ninguém a observa (ver
  `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md`).
- **`quarantine.go`** — `NewQuarantine(pool)`, a realização de
  `ports.Containment` fora de qualquer UoW: grava o envelope byte a byte
  como foi publicado (`GAR-07`) e o erro sanitizado (`ERR-20`, `ERR-21`).
- **`signals.go`** — `InboxSignals(pool, consumer)`: profundidade da quarantine
  e contagens por motivo (`terminal-failure`, `collision`,
  `attempts-exhausted`, `invalid-envelope`), lidas da própria tabela (`GAR-12`).
- **`purge.go`** — `PurgePublished` e `PurgeInbox`, que devolvem o que purgaram,
  de qual consumidor e até quando (`OBX-17`, `INB-16`). A invariante
  `retenção_inbox ≥ janela_redelivery` (`INB-14`) é do operador.

O que `memory` declara não provar — isolamento e conflito de serialização
entre transações concorrentes — é provado sobre este módulo, em
`apps/backend/orders/provider/concurrency_test.go`: dois escritores leem a mesma versão, e
exatamente um passa; o outro recebe `ErrVersionConflict` em vez de sobrescrever
em silêncio. `inbox_concurrency_test.go` faz o mesmo para a inbox: duas
transações registram a mesma chave, a segunda bloqueia até o desfecho da
primeira e recebe R2, R3 ou R1 conforme ela commitou `processed`, commitou
`rejected` ou desfez; e o teto de espera é interrompido pelo servidor.

`apps/backend/reservations/provider` é o lado do consumo do agregado de
referência: o repositório com chave natural (`order_id`, `ON CONFLICT DO
NOTHING` — a convergência de `GAR-10`) e o mapeador de `ReservationConfirmed`.
A composition root que liga tudo é o package raiz de `apps/backend/reservations`,
o bloco `app` do contexto.

## O que o módulo não contém

Claim, lease, `SKIP LOCKED`, transição de `status` e publicação são do `KRN-08`;
o codec do envelope e a fórmula do `payload_hash`, do `KRN-05` — este módulo os
chama, não os define; a tradução de destino lógico para alvo físico, o gesto de
ACK e a DLQ, do `KRN-10`; o conteúdo de `metadata`, de FND-07; métricas, o
valor do teto de espera e os prazos de retenção, de FND-08; um test kit
exportado, do `KRN-11`. O application service de consumo e as sete disposições
são do bloco `application` do contexto (`apps/backend/reservations/application`);
o adapter que aplica o efeito de broker é do bloco `app` (`app`).

A garantia é **at-least-once**. A deduplicação por identidade de mensagem é da
inbox; a idempotência do efeito é do domínio, por chave natural (`GAR-03`).

## Como rodar os testes localmente

Os testes de banco levam a build tag `integration` e só rodam no target
`test-race`. Sem `DMPF_PG_DSN` eles pulam com instrução fora do CI, e **falham**
dentro dele — um skip silencioso deixaria a outbox sem prova executável.

```bash
docker compose -f infra/local/docker-compose.yml --profile postgres up -d
export DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable'
pnpm nx run postgres:test-race
```

Sem o compose no ar, `pnpm nx run-many -t test -p postgres`
segue verde: os testes unitários (destino, mapeador) não têm a tag.

O `test-race` deste módulo roda com `cache: false` no Nx **e** `-count=1` no
`go test`. São dois caches distintos, e os dois devolveriam resultado antigo
para teste de banco: nenhum deles enxerga o estado do Postgres.

Desde o `KRN-07`, `app` também tem testes de banco, sobre as mesmas
tabelas. Os dois `test-race` não podem correr em paralelo — cada harness faz
`TRUNCATE` — e por isso o do `app` declara `dependsOn` sobre o deste
módulo: o Nx os sequencia mesmo com `--parallel=3`, que é como o CI roda.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,build,lint,test-race -p postgres
go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop
bash tools/dmpf-cell-check.sh
```

O `dmpf-gate-check.sh` **não** exercita este módulo: o `depguard` do
`.golangci.yml` seleciona por nome de diretório (`domain/`, `ports/`,
`application/`, `contracts/`) e não alcança `postgres`. O gate autoritativo
aqui é o verificador, e o `dmpf-cell-check.sh` prova as duas células que este
módulo torna alcançáveis: 26 (`provider → application`) e 12
(`application → contract`).

## Referências

- `docs/adr/035-realizacao-postgres-da-outbox.md` — as decisões deste módulo.
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira que ele realiza.
- `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §2.3, §3.1 a §3.3, §4.1, §4.2.
- `libs/backend/go/ports/README.md` — as portas em tipos de domínio.
- `libs/backend/go/application/README.md` — os nove passos e quem os anda.
