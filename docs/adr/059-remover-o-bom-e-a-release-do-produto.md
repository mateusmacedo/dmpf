# ADR-059: Remover o BOM e a release do produto

## Status

Aceito — 2026-10-06. Supersede parcialmente o [ADR-027](./027-bom-combinacao-certificada-compatibilidade-e-escape-hatch.md), o [ADR-041](./041-sdk-de-referencia-generator-e-bom-certificado.md) e o [ADR-047](./047-tags-de-modulo-go-e-consumo-fora-do-workspace.md), nos pontos listados na Decisão.

## Contexto

O BOM da release do produto (`bom/dmpf/<semver>.json`) registra, para cada release, a versão de runtimes, geradores, drivers e SDKs, o estado de certificação de cada entrada e a evidência de execução que a sustenta (`bom/evidence/<semver>/`). O validador `dmpf-bom` confere esse documento contra os registros reais (`go.work`, catálogo do pnpm, `go.mod`, `package.json`) e roda como gate em todo PR. Uma release do produto ainda pedia a tag `dmpf@<semver>`, cunhada pelo `dmpf-release.yml` depois de reproduzir a evidência com o `dmpf-evidence.yml`.

Esse custo não se paga neste projeto. O mantenedor é um só, não há consumidor externo e os apps são protótipos que exercitam o kernel. O BOM duplica à mão versões que já têm dono, e cada bump de dependência obriga a editar o documento no mesmo PR, sob pena de o gate reprovar.

A duplicação também quebra por construção. Na promoção da `1.0.0-rc.0`, o `DMPF BOM gate` reprovou o PR da release com `DMPF-B007`: o BOM registrava o `@mateusmacedo/dmpf-plugin` em `0.0.0`, e o `tools/dmpf-plugin/package.json` resolvia `0.1.0`. O `nx release` só grava a versão do `package.json` no commit de release da `master`, que não volta para a `develop`. Qualquer valor escrito no BOM fica errado em uma das duas branches.

## Decisão

**Sai:**

- o validador `dmpf-bom`, ou seja, `tools/dmpf-conformance/bom` e `cmd/bom`, e os códigos `DMPF-B001` a `DMPF-B012`;
- o diretório `bom/`, com o BOM, a evidência e o `README.md`;
- a evidência da release: o pacote `libs/backend/go/testkit/evidence`, o comando `testkit/cmd/evidence`, o target `testkit:evidence` e o parâmetro `record` do `tb.GoldenSuite`;
- os workflows `dmpf-release.yml` e `dmpf-evidence.yml`, o step `DMPF BOM gate` do `ci.yml` e as guardas de `dmpf@*` no `nx-release.yml` e no `nx-publish-libs.yml`;
- a tag do produto `dmpf@<semver>`;
- as exceções E2 (`bom-combination`) e E3 (`governance-instrument`), que só eram admitidas no registro do BOM. O `kind` delas passa a reprovar em `DMPF-X002`.

**Fica:**

- a exceção E1 (`external-dependency`), no `dmpf-units.json`, com o schema agora em `docs/guides/dmpf-manifesto.md`;
- as tags por módulo Go e o `nx release`;
- o `modsync`;
- os pins nos registros reais (`go.work`, catálogo do pnpm, `go.mod`, `tools/buf.sh`, `otelboot/config.go`), que passam a ser a única fonte da versão.

Pontos supersedidos:

- **ADR-027:** o BOM, a combinação certificada e as exceções E2 e E3. Fica a E1.
- **ADR-041:** os addenda do validador do BOM e da evidência de release, e a tag anotada `dmpf@<semver>`. Ficam o SDK de referência, o generator e o harness.
- **ADR-047:** o `dmpf-release.yml` e o `DMPF-B012`. Ficam as tags de módulo Go e os release groups.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Automatizar o campo `version` com um `dmpf-bom --write` no `nx-release.yml` | Resolve o `DMPF-B007`, mas exige recalcular a evidência no mesmo passo, porque o digest prende os bytes do BOM. É mais código para manter um documento que ninguém consome |
| Corrigir só a entrada do plugin | Trata o sintoma. O próximo bump de qualquer registro reabre a divergência |

## Consequências

**Positivas:**

- Cada versão passa a ter uma fonte só, o registro real, e o bump de dependência deixa de exigir uma edição paralela no BOM.
- A promoção para `master` deixa de depender de um gate que reprova por construção quando a `master` e a `develop` divergem no `package.json`.
- Saem cerca de 4,7 mil linhas de Go e dois workflows que só serviam à certificação.

**Negativas:**

- O `tb.GoldenSuite` perde o parâmetro `record` e o pacote `testkit/evidence` deixa de existir. É uma mudança incompatível na API do `testkit`, aceita porque o módulo não tem consumidor fora deste repositório.
- **Custo aceito:** perde-se o registro, por release, das combinações exercitadas pela suíte. Se um consumidor externo aparecer, a certificação volta por decisão nova.
- O corpo normativo de `docs/dmpf/` (`BOM-01` a `BOM-10`, `GOV-36`) não é reescrito. Cada documento afetado ganha uma nota de revogação no fim, e o texto fica como registro histórico, assim como as specs concluídas e os ADRs anteriores.
- O ADR de produção e o de testes da Onda 3 da SPEC-R8645FVR passam a usar os números 060 e 061. A tag `dmpf@0.1.0` continua no repositório como registro.

## Referências

- [ADR-027](./027-bom-combinacao-certificada-compatibilidade-e-escape-hatch.md), [ADR-041](./041-sdk-de-referencia-generator-e-bom-certificado.md), [ADR-047](./047-tags-de-modulo-go-e-consumo-fora-do-workspace.md) e [ADR-058](./058-projeto-solo-sem-controles-de-segunda-pessoa.md).
- `docs/dmpf/governanca-bom-pilotos.md` §4 e §5.1.
- DEVS-69 (remoção) e DEVS-68 (release `1.0.0-rc.0`).
