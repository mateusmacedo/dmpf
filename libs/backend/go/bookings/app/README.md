# bookings-app-go

Bloco `app` do bounded context `resource-scheduling`, gerado pelo generator
`bounded-context` (`tools/dmpf-plugin`).
Composição entre a entrega do transporte e o serviço de aplicação, com a borda HTTP do contexto.

Projeto Nx `bookings-app-go`, tags `type:lib`, `scope:backend`, `stack:go` e
`layer:apps`. Import path do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/bookings/app`.

## Unidades do manifesto

| Unidade | Bloco | `bounded_context` | Package |
| --- | --- | --- | --- |
| `resource-scheduling/app` | `app` | `resource-scheduling` | `bookingsapp` (raiz) |

`block` e `bounded_context` vêm das opções do generator e vão literalmente ao
`dmpf-units.json` — nunca são inferidos do nome do diretório (ADR-012).

## Resolução de dependências

O módulo é workspace-only: o `go.mod` declara apenas `module` e `go`, sem
`require`. Quem resolve as dependências é o `go.work` da raiz, onde o generator
já registrou a entrada `use`. Nenhum `go.sum` nasce aqui.

## Antes do primeiro commit

Unidade nova é ato de classificação (`AUT-01`): o baseline do verificador é
regravado em commit próprio, separado do commit do código.

```bash
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --write-baseline
```
