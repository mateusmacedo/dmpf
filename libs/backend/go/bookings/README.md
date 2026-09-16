# bookings

Bounded context `resource-scheduling`, o golden do harness de bounded contexts
(`docs/specs/SPEC-AHPRBZCT-bookings.md`, ADR-045): um módulo Go, um package por
bloco.

Projeto Nx `bookings`, tags `type:lib`, `scope:backend`, `stack:go` e
`layer:apps` — a camada é a do estágio mais alto entre os blocos, porque é o
primeiro em que toda dependência do módulo está de pé. Import path do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/bookings`.

## Blocos e unidades

| Package | Bloco | Unidade | Conteúdo |
| --- | --- | --- | --- |
| `domain` | `domain` | `resource-scheduling/domain` | Agregados `Booking` e `Resource`, três UPRs, desfecho em tipos do kernel |
| `ports` | `port` | `resource-scheduling/ports` | `BookingsByResourceReader`, a consulta por relação que os genéricos do kernel não expressam |
| `application` | `application` | `resource-scheduling/application` | `Service` com `Reserve`, `Cancel` e `Register` na sequência canônica |
| `provider` | `provider` | `resource-scheduling/provider-postgres` | Repositórios, mapeadores e `schema.sql` sobre `pgx` |
| `app` | `app` | `resource-scheduling/app` | Borda HTTP até a outbox, sobre o provider HTTP do kernel |

A sexta unidade do contexto, `resource-scheduling/contract`, vive no manifesto
de `libs/backend/go/contracts`, porque o código gerado do Protobuf mora lá.

`block` e `bounded_context` vêm do manifesto (`dmpf-units.json`), nunca do nome
do diretório (ADR-012).

## Aliases de import

Os packages do kernel `domain`, `application` e `ports` têm o mesmo nome dos
deste módulo. Onde um arquivo importa os dois, o import do kernel recebe alias
pelo papel: `kernel` para o domínio, `usecase` para a aplicação, `port` para as
portas. Os packages do contexto ficam bare.

## Testes

`test-race` roda com `-tags=integration` e `DMPF_PG_DSN`, e declara `dependsOn`
sobre o `test-race` do `postgres`: os dois harnesses truncam as mesmas tabelas
do mesmo banco.

```bash
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' pnpm nx run bookings:test-race
```
