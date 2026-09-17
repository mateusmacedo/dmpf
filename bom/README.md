# BOM da release do produto DMPF

Cada release do produto tem um BOM, em `bom/dmpf/<semver>.json`, revisável por
PR (`BOM-02`). A release é a tag anotada `dmpf@<semver>` (ADR-041). O validador
`dmpf-bom` roda no CI logo depois do verificador de conformidade e reprova o PR
em qualquer diagnóstico.

As normas vivem em [`docs/dmpf/governanca-bom-pilotos.md`](../docs/dmpf/governanca-bom-pilotos.md):
§4 para o BOM (`BOM-01` a `BOM-10`) e §5.1 para o escape hatch (`GOV-30` a
`GOV-36`). Este arquivo descreve o schema que as realiza.

## Validar

```bash
go run ./tools/dmpf-conformance/cmd/bom --root . --release latest --base develop
go run ./tools/dmpf-conformance/cmd/bom --root . --release 0.1.0 --now 2026-09-12T00:00:00Z
```

| Flag | Efeito |
| --- | --- |
| `--release` | Qual `bom/dmpf/<release>.json` validar: uma semver, ou `latest` para a maior semver do diretório. Sem ela, o único arquivo do diretório; mais de um reprova em `DMPF-B011` |
| `--base` | Ref git do BOM anterior; avalia as transições de `BOM-07` (`DMPF-B003`). Compara com o mesmo arquivo no ref; na release nova, com a maior semver de lá; sem BOM no ref, toda entrada é nova |
| `--now` | Instante RFC3339 contra o qual as validades vencem. Sem ela, o relógio |

Sai com `0` sem diagnóstico, `1` com diagnóstico e `2` em erro de leitura. O
workspace é lido por `os.Root`, que recusa symlink para fora da raiz.

Os BOMs das releases anteriores ficam em `bom/dmpf/`. O CI valida a maior release
(`--release latest`) contra o base; um BOM antigo não volta a ser validado, e
mudá-lo não passa pelo gate.

## Schema `dmpf/bom@1`

| Campo | Regra |
| --- | --- |
| `schema` | `dmpf/bom@1` |
| `product` | `dmpf` |
| `release` | Semver igual ao nome do arquivo (`DMPF-B011`) |
| `tag` | `dmpf@<release>` (`DMPF-B011`) |
| `runtimes`, `generators`, `drivers_clients_sdks`, `compatible_combinations`, `deprecations`, `cves` | Os seis itens de FND-10 §4.1, cada um `{entries: [...], reason}`. Seção ausente, ou vazia sem `reason`, reprova (`DMPF-B001`) |
| `semantic_conventions_messaging` | O slot de `BOM-10`: uma entrada, com `owner` |
| `exceptions` | Exceções E2 e E3 (ver [Pedir exceção](#pedir-exceção)); `[]` quando não houver |
| `metrics` | `vigentes`, `renovacoes` e `vencidas_sem_convergencia` (ver [Métricas](#métricas)) |

Chave JSON repetida no mesmo objeto, inclusive quando só a caixa muda, é erro de
leitura: o decodificador ficaria com a última, e quem revisa o PR lê a primeira.

### Entrada

Uma entrada é identificada por `subject`, `identity` e `version`, em qualquer
seção; a mesma chave duas vezes reprova (`DMPF-B002`).

| Campo | Obrigatório | Regra |
| --- | --- | --- |
| `subject` | sim | `product`, `kernel`, `contract`, `runtime`, `generator`, `driver`, `client`, `sdk` ou `semantic_conventions_messaging` |
| `identity` | sim | Coordenada exata: módulo, pacote, imagem ou binário |
| `version` | sim | Versão exata: semver (com ou sem `v`, pseudo-versão Go incluída) ou SHA de commit. Faixa ou `latest` reprova (`DMPF-B002`) |
| `state` | sim | `proposta`, `candidata`, `certificada`, `depreciada`, `nao_suportada` ou `rejeitada` |
| `criticality` | não | `critica` ou `padrao`; ausente é lido como `critica` |
| `compatible_with` | sim, ainda que `[]` | `{identity, version, evidence}` exercitados em conjunto (`DMPF-B006`) |
| `registry_ref` | para `runtime`, `generator` e o slot de `BOM-10` | `{file, selector}` do registro autoritativo (`DMPF-B007`) |
| `evidence_uri`, `evidence_digest`, `approved_by`, `certified_at`, `valid_until` | quando `certificada` | Os cinco juntos (`DMPF-B004`); digest `sha256:<hex>` (`DMPF-B005`); vencida reprova (`DMPF-B008`) |
| `promoted` | quando `certificada` | `{by, reviewed_by, pr}`: o ato de `BOM-05` |
| `deprecated_at`, `successor` | quando `depreciada` | `DMPF-B003` |
| `cve` | sim, ainda que `[]` | `{id, state, owner}`, com `state` entre `corrigida`, `mitigada` e `aberta`; `aberta` exige `owner` (`DMPF-B009`) |

Datas em RFC3339 ou `AAAA-MM-DD`. `valid_until` sem hora vale o dia inteiro.

## Registro autoritativo

O BOM referencia o registro de cada domínio alheio e não o duplica (`BOM-06`),
mas `BOM-03` exige `version` em toda entrada. A entrada declara as duas coisas, e
o validador resolve a referência e reprova divergência: a cópia deixa de poder
divergir em silêncio.

| `file` | `selector` | Valor lido | Vale para a `identity` |
| --- | --- | --- | --- |
| `go.work` | `go` | A linha `go` | `go` |
| `tools/buf.sh` | `buf` | A versão depois de `buf@` | terminada em `buf` |
| `contracts/buf.gen.yaml` | `plugin:<nome>` | A versão depois de `<nome>@` | terminada em `<nome>` |
| `nx.json` | `golangci-lint` | A versão depois de `golangci-lint@` | terminada em `golangci-lint` |
| `<módulo>/go.mod` | `require:<pacote>` | A versão do `require`, ignorando `replace`, `exclude` e `retract` | `<pacote>`; o módulo precisa estar no `go.work` |
| `<módulo>/package.json` | `version` | O campo `version` | igual ao `name` do `package.json` |
| `package.json` | `packageManager` | O campo | o gerenciador antes do `@` |
| `package.json` | `engines.node` | O campo | `node` |
| `pnpm-workspace.yaml` | `catalog:<pacote>` | A entrada do mapa `catalog` | `<pacote>` |
| `libs/backend/go/observability/otelboot/start.go` | `semconv` | A versão do import `semconv` | `go.opentelemetry.io/otel/semconv` |

Linha de comentário não é registro: um pin antigo comentado acima do atual é
ignorado. A comparação ignora o prefixo `v`, o `<gerenciador>@` e o sufixo
`+<hash>` do corepack (`pnpm@11.14.0+sha512…` casa com `11.14.0`). Faixa no
registro só é aceita como `^`; qualquer outra forma reprova como não suportada.

## Máquina de estados

| De | Para |
| --- | --- |
| `proposta` | `candidata`, `rejeitada` |
| `candidata` | `certificada`, `rejeitada` |
| `certificada` | `depreciada`, `candidata` |
| `depreciada` | `nao_suportada` |
| qualquer estado | `proposta` (recertificação) |

Com `--base`, a entrada é casada por `subject`, `identity` e `version`, sem olhar
a seção: a entrada que muda de seção ao ser depreciada continua sendo a mesma.
Entrada nova começa em `proposta` ou `candidata`, e só `rejeitada` sai do BOM:
remover qualquer outra reprova (`DMPF-B003`). Nenhuma transição acontece por
decurso de prazo: `certificada` com `valid_until` no passado reprova em
`DMPF-B008` até o commit que a declara `candidata`.

## Evidência

A evidência da certificação é commitada em `bom/evidence/<release>/<subject>.json`
e gerada pelo `dmpf-evidence` do `testkit`, sobre um commit com árvore limpa
e com o Postgres e o Redpanda das imagens pinadas no `dmpf-evidence.yml` — o
header grava a versão real de cada um, e outra imagem não reproduz no workflow:

```bash
CI=true GOTOOLCHAIN=go1.26.6 \
  DMPF_PG_DSN='postgres://dmpf:dmpf@localhost:5432/dmpf?sslmode=disable' \
  DMPF_KAFKA_BROKERS=localhost:9092 DMPF_REDPANDA_ADMIN=http://localhost:9644 \
  go run ./libs/backend/go/testkit/cmd/evidence --root . --release 0.1.0 --out bom/evidence/0.1.0
```

O comando publica um arquivo por subject (`golden`, `provider`, `domain`,
`services`, `app`, `dist` e `reference`) e o `index.json`, com o SHA-256 de cada
arquivo — o `evidence_digest` das entradas que o subject certifica. Duas
execuções sobre o mesmo commit publicam os mesmos bytes, e o workflow
`dmpf-evidence.yml` regenera a evidência no `header.commit` e a compara com
`diff -r`. Subjects, variáveis e códigos de saída estão na seção `evidence` do
[README do `testkit`](../libs/backend/go/testkit/README.md).

O header de cada subject nomeia `schema`, `release`, `subject`, `commit`,
`goversion`, `tags`, `packages`, `modules`, `externals`, `infra` (quando o subject
usa Postgres ou Redpanda) e, no `golden`, `tools`. O validador lê só `goversion`,
`modules` e `externals`:

```json
{
  "header": {
    "goversion": "1.26.6",
    "modules": [{ "path": "github.com/mateusmacedo/dmpf/libs/backend/go/domain", "version": "0.0.0" }],
    "externals": [{ "package": "github.com/jackc/pgx/v5", "version": "v5.10.0" }]
  }
}
```

- `evidence_uri` pode apontar para o forge, mas precisa conter
  `bom/evidence/<release>/`; o `evidence_digest` é conferido contra a cópia
  commitada (`DMPF-B005`).
- `compatible_with[].evidence` nomeia o subject do arquivo, e só conta quando
  alguma entrada do BOM ancora esse subject por `evidence_digest`; sem âncora, a
  evidência poderia ser trocada depois sem tocar no BOM.
- Os dois lados da combinação precisam constar do mesmo `header`: a própria
  entrada e o par de `compatible_with` (em `goversion` quando `identity` é `go`).
  Combinação que não consta é presumida, não exercitada (`BOM-04`, `DMPF-B006`).

## Rito da release

1. Gerar e commitar a evidência (ver [Evidência](#evidência)).
2. Promover as entradas que os headers alcançam, preencher os campos da
   `certificada` e validar com `--base develop`. O passo a passo está em
   [`docs/guides/dmpf-composicao.md`](../docs/guides/dmpf-composicao.md) §10.
3. Mergear em `develop` pela promoção de `BOM-05` e seguir por `release/<semver>`
   até `master`.
4. Cunhar a tag anotada `dmpf@<semver>` no merge commit de `master`, com a
   mensagem `DMPF release <semver> — BOM bom/dmpf/<semver>.json`. O
   `DMPF-B011` compara o campo `tag` do BOM com a release, e não a tag git:
   existência e alvo da tag são conferidos no rito, com `git cat-file` e
   `git rev-list` (ver [`CONTRIBUTING.md`](../CONTRIBUTING.md)).

## Pedir exceção

A exceção vive onde o objeto vive (`GOV-35`): dependência externa de uma unidade
(E1) no `dmpf-units.json` do módulo; combinação fora do BOM (E2) e instrumento
de governança (E3) em `exceptions` deste arquivo. Classe no registro errado
reprova em `DMPF-X007`.

| Campo | Regra |
| --- | --- |
| `id` | `X-` seguido de minúsculas, dígitos e hífens, único no registro (`GOV-33`) |
| `object` | `{kind, unit, identity}`, todos obrigatórios, com `kind` entre `external-dependency`, `bom-combination` e `governance-instrument` |
| `adr` | `ADR-NNN` |
| `owner` | A equipe, `team:<nome>` |
| `justification` | O que o golden path não resolve neste caso |
| `convergence` | `{kind: plan, deadline, condition}` ou `{kind: review, review_by, approved_by: [arquitetura, plataforma], replanning_condition}` |
| `valid_from`, `valid_until` | Vigência; `valid_until` é obrigatório, porque sem data de fim a exceção é inválida (`GOV-34`) |
| `review_by` | Revisão, obrigatória; não pode passar de `valid_until` |
| `history` | `[{event, at, by, reason}]`, com `event` entre `granted`, `renewed`, `revoked` e `converged`; o primeiro é `granted`, e `renewed` exige `reason` |

```json
{
  "id": "X-go-1-27-migracao",
  "object": { "kind": "bom-combination", "unit": "reference", "identity": "go@1.27.0" },
  "adr": "ADR-041",
  "owner": "team:plataforma",
  "justification": "serviço em runtime uma minor à frente do certificado durante a migração",
  "convergence": { "kind": "plan", "deadline": "2026-12-01", "condition": "go 1.27 certificado no BOM" },
  "valid_from": "2026-09-12",
  "valid_until": "2026-12-01",
  "review_by": "2026-11-01",
  "history": [{ "event": "granted", "at": "2026-09-12", "by": "team:plataforma" }]
}
```

O pedido é recusado sem exame de mérito quando o objeto cai no catálogo fechado
de `GOV-32` — constraint P0, célula da regra de dependência, âncora, `T1`–`T6`,
classificação, pin do Buf ou exigência de evidência (`DMPF-X003`, `DMPF-X004`).
Campo ausente reprova em `DMPF-X001`, e data que não parseia, em `DMPF-X005`.

Depois de `valid_until`, a exceção deixa de autorizar (`DMPF-X006`). Renovar é
conceder de novo: `valid_until` novo e um `renewed` com `reason`; `renewed` sem
vigência nova reprova em `DMPF-X005`. Um `revoked` ou `converged` encerra a
exceção, que para de autorizar e não vence mais. Um pedido admitido e um
recusado, passo a passo, estão em
[`docs/guides/dmpf-composicao.md`](../docs/guides/dmpf-composicao.md) §9.

## Métricas

`GOV-36` publica a saúde do instrumento junto do BOM. Os valores não são opinião:
derivam de `exceptions[].history`, e valor declarado divergente reprova
(`DMPF-B010`).

| Campo | Derivação |
| --- | --- |
| `vigentes` | Exceções não vencidas e sem `revoked` ou `converged` |
| `renovacoes` | Mapa `id → número de renewed`, com toda exceção listada, inclusive as de zero |
| `vencidas_sem_convergencia` | Exceções vencidas sem `converged`; a revogada dentro da vigência não conta, porque encerrou antes de vencer |

## Diagnósticos

| Código | Regra | Condição |
| --- | --- | --- |
| `DMPF-B001` | `BOM-01` | Seção ausente, ou vazia sem `reason` |
| `DMPF-B002` | `BOM-03` | Campo obrigatório ausente; `version` não exata; valor fora do conjunto; entrada repetida; `promoted` ausente em `certificada`; `schema`, `product`, slot de `BOM-10` ou `exceptions` ausentes ou divergentes; data fora de RFC3339 ou `AAAA-MM-DD`; CVE sem `id` ou com `state` fora do conjunto |
| `DMPF-B003` | `BOM-07` | Transição inválida contra `--base`; entrada nova fora de `proposta`/`candidata`; entrada removida que não estava `rejeitada`; `depreciada` sem `deprecated_at` ou `successor` |
| `DMPF-B004` | `BOM-07` | `certificada` sem um dos cinco campos de evidência |
| `DMPF-B005` | `BOM-03` | `evidence_digest` diferente do SHA-256 do arquivo em `bom/evidence/<release>/` |
| `DMPF-B006` | `BOM-04` | `compatible_with` com evidência não ancorada, ou com um dos lados sem execução registrada |
| `DMPF-B007` | `BOM-06` | `version` diferente do valor resolvido por `registry_ref`; `registry_ref` ausente onde é obrigatório, de outra `identity` ou fora do conjunto aceito |
| `DMPF-B008` | `BOM-08` | `certificada` com `valid_until` no passado — erro, nunca aviso |
| `DMPF-B009` | `BOM-03`, `BOM-09` | `cve` ausente; CVE `aberta` sem `owner` |
| `DMPF-B010` | `GOV-36` | `metrics` diferentes das derivadas de `exceptions[].history` |
| `DMPF-B011` | `BOM-02` | Mais de um BOM sem `--release`; `release` diferente do nome do arquivo ou fora de semver; `tag` diferente de `dmpf@<release>` |

As exceções do BOM trazem ainda os `DMPF-X001` a `DMPF-X007` da admissão comum.
A tabela completa, com seção normativa, vive em
`tools/dmpf-conformance/internal/rule/diagnostic.go`.
