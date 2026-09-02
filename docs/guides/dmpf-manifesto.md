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
      "include": ["gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/meu-modulo/internal/domain"]
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

Exceção à política de um bloco só vale se for **nominal** (RFC §6.4): o par
(unidade, dependência), com razão, owner e data de revisão.

```json
"exceptions": [
  {
    "unit": "meu-modulo/application",
    "dependency": "github.com/legado/sdk",
    "reason": "migração para porta em curso, ARQ-999",
    "owner": "time-plataforma",
    "review_by": "2026-12-31"
  }
]
```

Faltando qualquer um dos cinco, a exceção não autoriza nada e emite `DMPF-M001`.
Exceção por categoria, prefixo de pacote ou diretório é proibida — ela deixaria
de ser exceção e viraria política paralela não revisada.

A exceção vale só para o **próprio módulo**. Ela não autoriza uma unidade
homônima de outro manifesto.

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
```

O comando é separado da verificação de propósito: um gate que conserta o próprio
insumo deixa de detectar a divergência que existe para detectar.

## Rodar o verificador localmente

```bash
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root .
```

Saída `conforme` e exit 0 significa aprovado. Exit 1 é reprovação; exit 2 é falha
de execução — e a distinção importa, porque falha de execução nunca pode ser lida
como conformidade.

Cada diagnóstico nomeia a aresta, o arquivo que a introduz, o código e a seção
normativa. A tabela completa dos quinze códigos está em RFC §10.3.

## Referências

- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §3.3, §4.1, §4.5, §6, §7, §10
- `docs/adr/031-verificador-de-conformidade-dmpf-em-go.md` — decisões e limitações declaradas
- `docs/adr/012-classificacao-por-metadado-declarado.md` — por que não há inferência
- `docs/adr/014-proibir-aresta-domain-port.md` — a aresta proibida sem exceção
- `docs/adr/028-processo-de-autorizacao-da-classificacao.md` — o rito, quando viger
