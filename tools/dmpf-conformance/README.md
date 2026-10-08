# conformance

O gate normativo do DMPF que roda no CI: o verificador da regra de dependência
sobre o grafo real de imports (ADR-031). Ele reprova o PR com diagnóstico de
código estável e sai com `0` sem diagnóstico, `1` com diagnóstico e `2` em falha
de execução.
O mesmo módulo carrega o `modsync`, que sincroniza `go.work` e os `require` dos
`go.mod` com o grafo real de imports.

É tooling de workspace, não lib de reuso: nenhum contexto o importa em
produção, por isso vive em `tools/dmpf-conformance` — com o prefixo `dmpf-`,
pela exceção de tooling do ADR-045 — e não em `libs/backend/go` (ADR-046).
Projeto Nx `conformance`, tags `type:lib`, `scope:shared`, `stack:go`, sem
`layer:*`: o `ci.yml` o exclui do guard de camadas e roda a sua cadeia Go em
step próprio, antes dos gates que o executam. Import path do módulo:
`github.com/mateusmacedo/dmpf/tools/dmpf-conformance`.

## `conformance`

```bash
go run ./tools/dmpf-conformance/cmd/conformance --root .
go run ./tools/dmpf-conformance/cmd/conformance --root . --write-baseline
```

| Flag | Efeito |
| --- | --- |
| `--root` | Raiz do workspace |
| `--profiles` | `build-profiles.json`; sem ela, os perfis embutidos no verificador |
| `--now` | Instante RFC3339 contra o qual as exceções vencem (`DMPF-X006`). Sem ela, o relógio |
| `--write-baseline` | Regrava `tools/dmpf-baseline/units-baseline.json`. Executado por pessoa, nunca no gate |

A ordem da verificação é normativa: os manifestos são validados antes de
qualquer aresta, e um `DMPF-M*` encerra a fase. Em seguida vêm a designação de
shared kernel, a cobertura (`DMPF-U*`), as arestas (`DMPF-D*`, `DMPF-E*`) e a
autoridade sobre a classificação (`DMPF-T*`). O `include` de uma unidade só
classifica package do próprio módulo; apontar para outro módulo reprova em
`DMPF-M002`.

### Kernel recebido por versão

Num consumidor, o kernel chega por tag e fica fora do workspace. Todo módulo
fora do inventário que publica `dmpf-units.json` entra no universo como shared
kernel de leitura: as unidades dele servem de destino de aresta, satisfazem a
`DMPF-D002` sem `shared_kernel_units` e não são verificadas nem gravadas no
baseline. Aresta para package do kernel sem unidade reprova em `DMPF-U001`.
Manifesto do kernel inválido ou build list que o toolchain não carregou (módulo
fora do cache, sem rede) sai com `NAO VERIFICADO` e exit 2, citando
`<módulo>@<versão>`.

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

O schema da exceção está em [`docs/guides/dmpf-manifesto.md`](../../docs/guides/dmpf-manifesto.md#schema-da-exceção);
um pedido admitido e um recusado, em
[`docs/guides/dmpf-composicao.md`](../../docs/guides/dmpf-composicao.md) §9.

## `dmpf-modsync`

```bash
go run ./tools/dmpf-conformance/cmd/modsync --root . --write
go run ./tools/dmpf-conformance/cmd/modsync --root . --check
```

| Flag | Efeito |
| --- | --- |
| `--root` | Raiz do workspace, diretório do `go.work` (default: `.`) |
| `--write` | Grava o `require` e o `replace` versionado dos irmãos nos `go.mod` |
| `--check` | Confere os `go.mod` sem alterar e reprova divergência |
| `--require` | `<módulo>@<versão>` que dá dono a um import fora da build list, como o kernel no primeiro contexto de um consumidor. Repetível, só com `--write` |

Exige exatamente um entre `--write` e `--check`. A versão de um irmão sem
`require` vem da maior tag de release alcançável; tag de pré-release só conta
quando o módulo não tem release estável. Sincroniza o `go.work` (bloco
`use` e `replace` versionado) e os `require` de cada `go.mod` com o grafo real
de imports entre módulos irmãos (ADR-047) — sem isso, o `go.mod` gerado
carregaria imports não declarados. O generator `bounded-context`
(`tools/dmpf-plugin`) roda `--write` uma vez ao criar o módulo, num callback
pós-flush: o modsync lê `go.mod` e `go.work` do disco, não da Tree do Nx. O CI
roda `--check` como gate de conformidade dos dois lados.

## Códigos

| Família | Regra | Onde decide |
| --- | --- | --- |
| `DMPF-U*`, `DMPF-M*`, `DMPF-T*`, `DMPF-D*`, `DMPF-E*` | RFC §10.3 | `conformance` |
| `DMPF-X001`–`DMPF-X007` | `GOV-30` a `GOV-35` | `conformance` |

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
| `conformance/modsync` | `app` | `modsync` |
| `conformance/cmd-modsync` | `app` | `cmd/modsync` |

Todas com `bounded_context` `conformance`. O `fitness` é a única unidade com
`public_integration_surface: true`: é o package que o `testkit/fitness` e as
suítes de outros contextos consomem para construir o universo real.
