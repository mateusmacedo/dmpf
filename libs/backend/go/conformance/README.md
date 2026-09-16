# conformance

Os dois gates normativos do DMPF que rodam no CI: o verificador da regra de
dependência sobre o grafo real de imports (ADR-031) e o validador do BOM da
release (ADR-041). Os dois reprovam o PR com diagnóstico de código estável e
saem com `0` sem diagnóstico, `1` com diagnóstico e `2` em falha de execução.

## `conformance`

```bash
go run ./libs/backend/go/conformance/cmd/conformance --root . --base develop
go run ./libs/backend/go/conformance/cmd/conformance --root . --write-baseline
```

| Flag | Efeito |
| --- | --- |
| `--root` | Raiz do workspace |
| `--profiles` | `build-profiles.json`; default dentro deste módulo |
| `--base` | Ref do intervalo em revisão. Sem ela, o commit próprio de RFC §10.2 fica não verificado, e não verificado reprova |
| `--now` | Instante RFC3339 contra o qual as exceções vencem (`DMPF-X006`). Sem ela, o relógio |
| `--write-baseline` | Regrava `tools/dmpf-baseline/units-baseline.json`. Ato de classificação: commit próprio, executado por pessoa, nunca no gate |

A ordem da verificação é normativa: os manifestos são validados antes de
qualquer aresta, e um `DMPF-M*` encerra a fase. Em seguida vêm a designação de
shared kernel, a cobertura (`DMPF-U*`), as arestas (`DMPF-D*`, `DMPF-E*`) e a
autoridade sobre a classificação (`DMPF-T*`). O `include` de uma unidade só
classifica package do próprio módulo; apontar para outro módulo reprova em
`DMPF-M002`.

### Exceção E1

Dependência externa que a política de bloco nega entra por exceção nominal no
`exceptions` do `dmpf-units.json` do módulo. O `manifest.Validate` admite cada
pedido por `internal/exception`, e só a exceção admitida autoriza o import
nominal da própria unidade. A recusa emite `DMPF-X001` a `DMPF-X007` **sem**
encerrar a verificação: o import recusado continua reprovando em `DMPF-E001` no
mesmo relatório.

- O pedido exige `object.unit`, `object.identity`, `valid_until` e `review_by`,
  além dos itens de `GOV-30`. Data que não parseia reprova em `DMPF-X005`, e
  unidade que o manifesto não declara, em `DMPF-X007`. Um `id` repetido no
  mesmo manifesto reprova em `DMPF-X001`.
- Renovar exige `valid_until` novo; `renewed` sem vigência nova reprova em
  `DMPF-X005`. Exceção com `revoked` ou `converged` no histórico encerra: deixa
  de autorizar e não vence.
- Os cinco campos legados (`unit`, `dependency`, `reason`, `owner`, `review_by`)
  continuam aceitos ao lado dos novos, mas os pares precisam coincidir;
  divergência reprova em `DMPF-X001`.
- O `--write-baseline` passa pela mesma admissão e recusa manifesto com exceção
  não admitida.

O schema da exceção está em [`bom/README.md`](../../../../bom/README.md#pedir-exceção);
um pedido admitido e um recusado, em
[`docs/guides/dmpf-composicao.md`](../../../../docs/guides/dmpf-composicao.md) §9.

## `dmpf-bom`

```bash
go run ./libs/backend/go/conformance/cmd/bom --root . --release latest --base develop
```

Valida `bom/dmpf/<semver>.json` contra `BOM-01` a `BOM-10` (`DMPF-B001` a
`DMPF-B011`) e admite as exceções E2 e E3 pela mesma `internal/exception`. Lê o
workspace por `os.Root`, que recusa symlink para fora da raiz. Flags, schema e
códigos em [`bom/README.md`](../../../../bom/README.md).

## Códigos

| Família | Regra | Onde decide |
| --- | --- | --- |
| `DMPF-U*`, `DMPF-M*`, `DMPF-T*`, `DMPF-D*`, `DMPF-E*` | RFC §10.3 | `conformance` |
| `DMPF-X001`–`DMPF-X007` | `GOV-30` a `GOV-35` | os dois |
| `DMPF-B001`–`DMPF-B011` | `BOM-01` a `BOM-10`, `GOV-36` | `dmpf-bom` |

A tabela com resumo e seção normativa de cada código vive em
`internal/rule/diagnostic.go`. Os testes espelho conferem os literais transcritos
à mão, para que renomear uma constante não passe verde.

## Unidades

| Unidade | Bloco | Package |
| --- | --- | --- |
| `conformance/rule` | `domain` | `internal/rule` |
| `conformance/manifest` | `domain` | `internal/manifest` |
| `conformance/baseline` | `domain` | `internal/baseline` |
| `conformance/exception` | `domain` | `internal/exception` |
| `conformance/port` | `port` | `internal/port` |
| `conformance/conformance` | `application` | `internal/conformance` |
| `conformance/fsstore` | `provider` | `internal/fsstore` |
| `conformance/golist` | `provider` | `internal/golist` |
| `conformance/cmd` | `app` | `cmd/conformance` |
| `conformance/fitness` | `app` | `fitness` |
| `conformance/bom` | `app` | `bom` |
| `conformance/cmd-bom` | `app` | `cmd/bom` |
