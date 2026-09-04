# dmpf-provider-postgres-go

Realização em PostgreSQL das três portas transacionais que o `KRN-04` fixou em
tipos de domínio: `UnitOfWork[R]`, `Repository[ID, S]` e `Outbox`. É o bloco
`provider` do kernel DMPF, e a primeira realização que prova a sequência
canônica de FND-04 §3.2 contra um banco de verdade — a de `example/memory` roda
sobre mutexes e declara não provar isolamento.

Duas unidades no manifesto, ambas `provider`, ambas em `dmpf-kernel`:

| Unidade | Diretório | O que é |
| --- | --- | --- |
| `dmpf-kernel/provider-postgres` | raiz | UoW sobre `pgx.Tx`, outbox, migração e purga |
| `dmpf-kernel/example-orders-postgres` | `example/orders` | repositório e mapeador do agregado de exemplo |

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
  `payload_hash`, com `UNIQUE (message_id)` e três `CHECK`. `Migrate` é
  idempotente e não há tabela de versão: não existe ferramenta de migração aqui.
- **`purge.go`** — `PurgePublished`, que devolve o que purgou e até quando.

O que `example/memory` declara não provar — isolamento e conflito de serialização
entre transações concorrentes — é provado aqui, em
`example/orders/concurrency_test.go`: dois escritores leem a mesma versão, e
exatamente um passa; o outro recebe `ErrVersionConflict` em vez de sobrescrever
em silêncio.

## O que o módulo não contém

Claim, lease, `SKIP LOCKED`, transição de `status` e publicação são do `KRN-08`;
inbox e deduplicação, do `KRN-07`; o codec do envelope e a fórmula do
`payload_hash`, do `KRN-05` — este módulo os chama, não os define; a tradução de
destino lógico para alvo físico, do `KRN-10`; o conteúdo de `metadata`, de
FND-07; métricas e prazos, de FND-08; um test kit exportado, do `KRN-11`.

A garantia é **at-least-once**. A deduplicação é do consumidor, via inbox.

## Como rodar os testes localmente

Os testes de banco levam a build tag `integration` e só rodam no target
`test-race`. Sem `DMPF_PG_DSN` eles pulam com instrução fora do CI, e **falham**
dentro dele — um skip silencioso deixaria a outbox sem prova executável.

```bash
docker compose -f infra/local/docker-compose.yml --profile postgres up -d
export DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable'
pnpm nx run dmpf-provider-postgres-go:test-race
```

Sem o compose no ar, `pnpm nx run-many -t test -p dmpf-provider-postgres-go`
segue verde: os testes unitários (destino, mapeador) não têm a tag.

O `test-race` deste módulo roda com `cache: false` no Nx **e** `-count=1` no
`go test`. São dois caches distintos, e os dois devolveriam resultado antigo
para teste de banco: nenhum deles enxerga o estado do Postgres.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,build,lint,test-race -p dmpf-provider-postgres-go
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --base develop
bash tools/dmpf-cell-check.sh
```

O `dmpf-gate-check.sh` **não** exercita este módulo: o `depguard` do
`.golangci.yml` seleciona por nome de diretório (`-domain/`, `-ports/`,
`-application/`, `-contracts/`) e não alcança `-postgres`. O gate autoritativo
aqui é o verificador, e o `dmpf-cell-check.sh` prova as duas células que este
módulo torna alcançáveis: 26 (`provider → application`) e 12
(`application → contract`).

## Referências

- `docs/adr/035-realizacao-postgres-da-outbox.md` — as decisões deste módulo.
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira que ele realiza.
- `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §2.3, §3.1 a §3.3, §4.1, §4.2.
- `libs/backend/go/dmpf-ports/README.md` — as portas em tipos de domínio.
- `libs/backend/go/dmpf-application/README.md` — os nove passos e quem os anda.
