# Guia: escrever o `dmpf-units.json` de um módulo

Este guia é para quem cria um módulo Go novo sob o gate do DMPF — os módulos de
`KRN-03` a `KRN-12` e os que vierem depois. Ele não reabre a RFC: descreve o que
o verificador `dmpf-conformance` exige e por quê, e cita a seção normativa de
cada exigência para quem quiser ir à fonte.

## O mínimo que faz o gate passar

Todo módulo com código de produção precisa de um `dmpf-units.json` na raiz. Sem
ele, o verificador emite `DMPF-U004` e a verificação **encerra** — sem
classificação não há decisão possível, e prosseguir seria adivinhar.

```json
{
  "schema": "dmpf/units@1",
  "units": [
    {
      "id": "meu-modulo/domain",
      "block": "domain",
      "bounded_context": "meu-contexto",
      "public_integration_surface": false,
      "include": ["github.com/mateusmacedo/dmpf/libs/backend/go/meu-modulo/internal/domain"]
    }
  ],
  "external": [],
  "exceptions": []
}
```

## Os quatro campos obrigatórios

| Campo | Regra | Ausente ou inválido |
| --- | --- | --- |
| `id` | único **dentro deste manifesto** | `DMPF-M001` / `DMPF-M003` |
| `block` | um dos seis: `domain`, `application`, `app`, `port`, `provider`, `contract` | `DMPF-M001` / `DMPF-M002` |
| `bounded_context` | string estável | `DMPF-M001` |
| `include` | um ou mais **import paths exatos** | `DMPF-M001` / `DMPF-M002` |

`public_integration_surface` é opcional e tem default `false`.

**Herança não existe** (RFC §10.1). Cada unidade declara os próprios campos por
completo — nada é derivado de diretório pai, de módulo ou de outra unidade.

## `include` enumera packages, não subárvores

Este é o ponto que mais surpreende quem vem de globs. Em Go o `include` casa por
**import path exato**:

```json
"include": [
  ".../internal/domain",
  ".../internal/domain/politica"
]
```

Declarar só `.../internal/domain` **não** cobre `.../internal/domain/politica`.
O subpackage não declarado emite `DMPF-U001`.

É verboso de propósito. Casar por prefixo faria um package novo herdar a
classificação pelo lugar do diretório — inferência por convenção, que o ADR-012 e
a RFC §4.4 proíbem —, e permitiria classificar código novo sem nenhuma edição
revisável do manifesto. A tolerância única é a barra final: `.../domain/` e
`.../domain` designam o mesmo package.

Uma unidade só possui packages do **próprio módulo**. Declarar no manifesto de um
módulo o import path de outro não o classifica.

## Escolher o bloco

| Bloco | O que vive nele | Capabilities permitidas |
| --- | --- | --- |
| `domain` | regra de negócio, invariante, policy computacional | só `pure` |
| `port` | contrato de capacidade de fronteira | só `pure` |
| `application` | caso de uso, orquestração | `pure` e `observability` [^1] |
| `contract` | schema de wire, tipos gerados, envelopes | `pure` e `wire.codec` |
| `provider` | adapter concreto, driver, SDK | qualquer |
| `app` | composition root, adapter de protocolo | qualquer |

Duas armadilhas frequentes:

- **`domain` não importa `port`.** É proibido sem condicional e sem exceção
  (ADR-014). Interface computacional é `domain`; porta é fronteira.
- **Telemetria não entra em `domain` nem em `port`**, ainda que a biblioteca de
  logging seja tecnicamente pura (RFC §6.2, princípio 11).

[^1]: A norma descreve `application` negando `io.*`, `runtime.framework` e
`wire.codec`, sem citar `observability` em nenhuma das duas listas. O
verificador lê o complemento — permitido é o que sobra — e por isso aceita
telemetria em `application`. A decisão e as alternativas estão em
[ADR-031](../adr/031-verificador-de-conformidade-dmpf-em-go.md); se você
declarar uma dependência de log num caso de uso, é essa leitura que a autoriza.

Casos-limite recorrentes já estão decididos em RFC §4.6 — validação de invariante
é `domain`, validação de formato é `app`, cache é `port` + `provider`, retry é
`provider` ou `app`.

## Dependência externa: `external[]`

Todo import que não resolve para o universo é dependência externa, e um bloco
`default deny` só pode usá-la se ela constar da allowlist com os **quatro**
elementos (RFC §6.3):

```json
"external": [
  {
    "package": "github.com/exemplo/decimal",
    "versions": ">=1.2 <2",
    "entrypoints": ["."],
    "capability": "pure"
  }
]
```

- `entrypoints: ["."]` declara a **raiz**. Lista vazia não autoriza nada — a
  regra dos entrypoints existe para que um pacote com subpath puro e subpath
  impuro possa ter só o primeiro declarado.
- `capability` é **declarada**, nunca inferida do nome do pacote.
- Entrada declarada `pure` cujo fechamento transitivo alcança outra capability
  emite `DMPF-E002`.

Builtins não precisam de entrada: eles têm capability atribuída na tabela do
verificador (`net/http` é `io.network`, `crypto` é `pure`). Builtin ausente da
tabela **reprova** em bloco `default deny` — não saber reprova.

## Exceção nominal: `exceptions[]`

Exceção à política de um bloco só vale se for **nominal** (RFC §6.4) e passar
pela admissão de `GOV-30` a `GOV-36`: o par (unidade, dependência), com ADR,
equipe dona, justificativa, plano de convergência, vigência, revisão e histórico.

```json
"exceptions": [
  {
    "id": "X-meu-modulo-application-legado-sdk",
    "object": {
      "kind": "external-dependency",
      "unit": "meu-modulo/application",
      "identity": "github.com/legado/sdk"
    },
    "adr": "ADR-099",
    "owner": "team:plataforma",
    "justification": "migração para porta em curso",
    "convergence": { "kind": "plan", "deadline": "2026-12-31", "condition": "porta publicada no kernel" },
    "valid_from": "2026-09-01",
    "valid_until": "2026-12-31",
    "review_by": "2026-11-30",
    "history": [{ "event": "granted", "at": "2026-09-01", "by": "team:plataforma" }]
  }
]
```

A admissão recusa o pedido incompleto ou fora do universo de `GOV-31` com
`DMPF-X001` a `DMPF-X007`, sem encerrar a verificação, e a exceção recusada não
autoriza nada: o import continua reprovando em `DMPF-E001`. Os cinco campos
legados (`unit`, `dependency`, `reason`, `owner`, `review_by`) ainda são aceitos
ao lado dos novos, desde que coincidam com eles. O schema completo e os códigos
estão em [`bom/README.md`](../../bom/README.md#pedir-exceção).
Exceção por categoria, prefixo de pacote ou diretório é proibida — ela deixaria
de ser exceção e viraria política paralela não revisada.

A exceção vale só para o **próprio módulo**. Ela não autoriza uma unidade
homônima de outro manifesto.

## Bloco `contract` e código gerado

O bloco `contract` admite só `pure` e `wire.codec` (RFC §6.2), e é o único
lugar do código gerado de Protobuf. O primeiro manifesto real desse bloco é o de
`libs/backend/go/dmpf-contracts`, e ele mostra os dois pontos que todo módulo
`contract` vai repetir.

O runtime Protobuf entra pela allowlist, declarado `wire.codec` e restrito aos
subpacotes que o gerado e o codec importam de fato — a raiz do módulo ninguém
importa, então `entrypoints: ["."]` não autorizaria nada:

```json
"external": [
  {
    "package": "google.golang.org/protobuf",
    "versions": ">=1.36.12 <2",
    "entrypoints": [
      "google.golang.org/protobuf/proto",
      "google.golang.org/protobuf/types/known/anypb",
      "google.golang.org/protobuf/types/known/timestamppb",
      "google.golang.org/protobuf/reflect/protoreflect",
      "google.golang.org/protobuf/runtime/protoimpl"
    ],
    "capability": "wire.codec"
  }
]
```

O `protoc-gen-go` emite `reflect` e `unsafe` em todo arquivo gerado, e a tabela
do verificador classifica os dois como `runtime.framework` — capability que o
bloco `contract` não admite. A saída não é reclassificar a stdlib nem excluir
arquivos gerados da análise: é a exceção nominal da RFC §6.4, um par (unidade,
dependência) por vez. Como o plugin emite esses imports em todo arquivo, não há
prazo de convergência planejável, e o pedido usa o ramo de revisão, com
aprovação de Arquitetura e Plataforma e a condição amarrada ao plugin:

```json
"exceptions": [
  {
    "id": "X-dmpf-contracts-gen-reflect",
    "object": { "kind": "external-dependency", "unit": "dmpf-contracts/gen", "identity": "reflect" },
    "adr": "ADR-033",
    "owner": "team:tech-leads",
    "justification": "import emitido pelo protoc-gen-go v1.36.12 em todo arquivo gerado; plumbing do runtime Protobuf, não uso de framework (RFC 6.4)",
    "convergence": {
      "kind": "review",
      "review_by": "2027-03-02",
      "approved_by": ["arquitetura", "plataforma"],
      "replanning_condition": "elimination of reflect and unsafe imports from protoc-gen-go output"
    },
    "valid_from": "2026-09-12T00:00:00Z",
    "valid_until": "2027-03-02",
    "review_by": "2027-03-02",
    "history": [{ "event": "granted", "at": "2026-09-12T00:00:00Z", "by": "team:tech-leads" }]
  }
]
```

A mesma entrada se repete para `unsafe`. Um módulo `contract` que troque a
versão do plugin revisa as duas exceções no mesmo commit normativo.

## Mudar a classificação depois

Alterar `block` ou `bounded_context`, criar unidade, remover unidade **e
remapear o `include` de modo a mudar quais packages a unidade captura** são atos
regulados (RFC §10.2 T4/T6; ADR-028). Todos exigem:

1. Atualizar o `dmpf-units.json` **e** o baseline em
   `tools/dmpf-baseline/units-baseline.json` — divergência entre os dois emite
   `DMPF-T001`.
2. Apresentar a mudança em **commit próprio**, separado de mudanças de código.
   Misturar os dois emite `DMPF-T002`.
3. Ter o commit aprovado por revisor distinto do autor. Essa parte é verificada
   pela revisão do PR, não pelo gate — a fronteira está declarada no ADR-031.

Para regravar o baseline depois de uma mudança legítima:

```bash
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --write-baseline
pnpm biome format --write tools/dmpf-baseline/units-baseline.json
```

O comando é separado da verificação de propósito: um gate que conserta o próprio
insumo deixa de detectar a divergência que existe para detectar.

O `--write-baseline` grava o JSON fora do formato do Biome, e o `biome ci` do CI
reprovaria o arquivo. Formate no mesmo commit do baseline; o `pre-commit` do
Lefthook faz o mesmo nos arquivos em stage.

## Shared kernel

Uma unidade designada como **shared kernel** pode ser importada por qualquer
bounded context. É a exceção que torna o kernel DMPF consumível como SDK: sem
ela, importar qualquer bloco do kernel de outro contexto emite `DMPF-D002`,
porque `dmpfapplication.Outcome[R]` e `dmpfports.OutboxEntry` expõem tipos de
`dmpfdomain` e arrastam o `domain` do kernel junto (ADR-042).

A designação **não fica no manifesto**. Ela vive na chave `shared_kernel_units`
do baseline, e lista chaves canônicas de unidade — o mesmo valor do campo `unit`
de cada entrada:

```json
{
  "schema": "dmpf/units-baseline@1",
  "digest": "sha256:…",
  "shared_kernel_units": [
    "dmpf-kernel/domain",
    "dmpf-kernel/port"
  ],
  "entries": [ … ]
}
```

O manifesto do módulo não muda: não existe campo de shared kernel em
`dmpf-units.json`. A razão é de autoridade — designar é decisão de arquitetura
sobre o repositório inteiro, e o manifesto é escrito por quem desenvolve o
módulo.

### O que a designação libera, e o que não

Libera **apenas o destino**. Outro contexto passa a poder importar a unidade
designada; a unidade designada **não** ganha licença para importar de fora do
seu próprio contexto. A exceção é unidirecional.

Continua valendo tudo o mais:

- A matriz de blocos (C1) é idêntica. Uma aresta que viola a matriz segue
  emitindo `DMPF-D001` mesmo com o destino designado — shared kernel relaxa a
  condição de contexto, nunca a de bloco.
- `public_integration_surface: true` em bloco `domain` segue inválido
  (`DMPF-M002`). Designar não é declarar superfície pública, e uma coisa não
  substitui a outra.
- O interior do contexto segue privado por default (ADR-017). Designa-se
  unidade nominal, nunca o bounded context inteiro: os agregados de exemplo e as
  composition roots do kernel permanecem privados.

### Como designar

Designar é **ato de classificação**, com o mesmo rito da seção anterior: commit
próprio, separado de código, aprovado por revisor distinto do autor. Alterar
`shared_kernel_units` junto com um `.go` emite `DMPF-T002`.

1. Acrescente a chave ao baseline, com as chaves canônicas das unidades.
2. Regrave o baseline, porque o digest incorpora a lista:

```bash
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --write-baseline
pnpm biome format --write tools/dmpf-baseline/units-baseline.json
```

Editar a chave sem regravar deixa o digest sem fechar, e toda verificação
seguinte reprova com `DMPF-T001`.

3. Commite **apenas** o baseline.

### Chave que não resolve reprova

`DMPF-M004` cobre a designação malformada, em três casos: chave sem nenhuma
entrada correspondente, chave com duas ou mais, e chave que resolve para uma
entrada fora do universo. O diagnóstico **interrompe a verificação** em vez de
seguir decidindo arestas — decidir sobre designação inválida produziria um
veredicto com aparência de conformidade.

Não confunda ausência com lista vazia. A chave ausente significa que o
repositório não adotou a designação, e mantém o digest legado byte a byte;
`[]` significa que adotou e não designou ninguém. `null` explícito é **recusado**
por ser ambíguo entre as duas intenções.

## Rodar o verificador localmente

```bash
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root .
```

Saída `conforme` e exit 0 significa aprovado. Exit 1 é reprovação; exit 2 é falha
de execução — e a distinção importa, porque falha de execução nunca pode ser lida
como conformidade.

Cada diagnóstico nomeia a aresta, o arquivo que a introduz, o código e a seção
normativa. A tabela completa dos dezesseis códigos está em RFC §10.3.

## Referências

- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §3.3, §4.1, §4.5, §6, §7, §10
- `docs/adr/031-verificador-de-conformidade-dmpf-em-go.md` — decisões e limitações declaradas
- `docs/adr/012-classificacao-por-metadado-declarado.md` — por que não há inferência
- `docs/adr/014-proibir-aresta-domain-port.md` — a aresta proibida sem exceção
- `docs/adr/028-processo-de-autorizacao-da-classificacao.md` — o rito, quando viger
- `docs/adr/017-bounded-context-declarado-superficie-publica.md` — a condição de contexto C2 e o interior privado por default
- `docs/adr/042-shared-kernel.md` — a designação de shared kernel e a extensão de C2
