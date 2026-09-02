# Contratos de wire do DMPF

Este diretório é a **fonte** dos contratos de integração da plataforma: os
arquivos `.proto`, a configuração do Buf e as golden fixtures. Ele é neutro de
stack — nenhuma opção de linguagem vive aqui; o pacote Go é aplicado na geração,
pelo managed mode (`BUF-10`).

A norma que este diretório implementa é `docs/dmpf/cloudevents-protobuf-buf.md`
(perfil CloudEvents `ENV-*`, política Protobuf `PTB-*`, governança Buf `BUF-*`,
layout `REP-*`, fixtures `INT-*`), com o codec e a fórmula do `payload_hash`
fixados em `docs/adr/022-codec-cloudevents-proto-data-e-payload-hash.md` e a
autoridade de validação em `docs/adr/023-autoridade-de-validacao-e-registry.md`.

## Árvore

```text
contracts/
├── buf.yaml            workspace Buf v2: módulo `proto`, lint STANDARD, breaking FILE, deps vazias (BUF-01, BUF-02)
├── buf.gen.yaml        geração em managed mode; plugin protoc-gen-go pinado (BUF-06, BUF-10)
├── proto/
│   ├── io/cloudevents/v1/cloudevents.proto           envelope oficial do CloudEvents, vendorizado (ENV-07)
│   └── company/orders/event/v1/order_placed.proto    contrato de exemplo (§5.1 da norma)
├── fixtures/
│   └── orders/event/v1/order-placed.golden           golden fixture JSON, fonte única das stacks (INT-01)
├── openapi/            registrado, não normatizado (REP-06)
└── asyncapi/           registrado, não normatizado (REP-06)
```

O código gerado **não** fica aqui: `buf.gen.yaml` escreve em
`libs/backend/go/dmpf-contracts/gen/go/`, dentro do módulo Go do bloco
`contract`, porque é esse módulo que o verificador de conformidade classifica.
A semântica de `REP-02` é preservada: o gerado é versionado, nunca editado à
mão, e a divergência entre gerado e versionado reprova o PR.

## Toolchain

Nenhum binário é instalado. A CLI entra por `tools/buf.sh`, que executa
`go run github.com/bufbuild/buf/cmd/buf@<versão>`; o plugin `protoc-gen-go`
entra pelo `local` de `buf.gen.yaml`, também por `go run`. Os dois pins são
exatos e ficam num único lugar cada; `tools/buf-gate.sh pins` reprova `latest`,
faixa ou divergência entre a versão do plugin e a do runtime no `go.mod`.

Comandos do dia a dia, sempre a partir da raiz do repositório:

```bash
bash tools/buf.sh lint contracts
bash tools/buf.sh format --diff --exit-code contracts
(cd contracts && bash ../tools/buf.sh generate)
```

## Proveniência do envelope vendorizado

`proto/io/cloudevents/v1/cloudevents.proto` é o formato Protobuf oficial do
CloudEvents, copiado do repositório `cloudevents/spec` na tag `v1.0.2`
(`cloudevents/formats/cloudevents.proto`) e submetido a `buf format`. A
formatação muda apenas espaços em branco e a ordem de linhas de `option`; o
descriptor resultante é o mesmo.

| Estado | SHA-256 |
|--------|---------|
| Upstream (`cloudevents/spec@v1.0.2`) | `2bc1e82754cc8b7abb08fa8329e50a7643f6da18f6d60479ecbe403ae6e5fecc` |
| Vendorizado (após `buf format`) | `38219b7ca3e791b336115b4ed4a79ebb136450e88487f30eb564a8fcdc7cc31f` |

Para reproduzir a conferência:

```bash
curl -sSL -o /tmp/cloudevents.proto \
  https://raw.githubusercontent.com/cloudevents/spec/v1.0.2/cloudevents/formats/cloudevents.proto
sha256sum /tmp/cloudevents.proto
sha256sum contracts/proto/io/cloudevents/v1/cloudevents.proto
```

O arquivo oficial carrega `option go_package` e opções de outras linguagens; o
managed mode as sobrescreve na geração, e por isso o arquivo não é editado.

## Gates e baseline

Os quatro gates de `tools/buf-gate.sh` rodam como targets Nx do projeto
`dmpf-contracts-go` e no passo `Contracts gates (affected)` do CI; nenhum é
advisory e nenhum tem bypass (`BUF-12`):

| Subcomando | O que prova |
|------------|-------------|
| `lint` | `buf format --diff --exit-code`, `buf lint` em `STANDARD` e varredura textual de P0-3 (nenhum artefato promete entrega única fim a fim) |
| `pins` | pin exato da CLI em `tools/buf.sh`, do plugin em `buf.gen.yaml`, igualdade plugin × runtime no `go.mod` e `buf.lock` presente quando há `deps` |
| `generate-check` | duas gerações idênticas byte a byte e ausência de drift entre gerado e versionado (`BUF-11`) |
| `breaking` | `buf breaking` em `FILE` contra `NX_BASE`, sob a máquina de estados de `BUF-08` |

`buf breaking` segue `BUF-08`: cada módulo do workspace está em um de dois
estados, reconhecidos pela **marca de baseline** — a tag anotada
`contracts-baseline/<módulo>` (para o módulo `proto`, `contracts-baseline/proto`).

- **`sem baseline`** — a marca não existe e o módulo também não existe em
  `NX_BASE`: é o primeiro conteúdo do módulo; `breaking` é dispensado só para
  ele, com aviso na saída. `lint`, `format`, `generate-check` e `pins` seguem
  obrigatórios. É o estado deste repositório enquanto a marca não for criada.
- **`baseline estabelecido`** — a marca existe: `breaking` é obrigatório;
  `NX_BASE` vazio, irresolvível ou sem `contracts/` reprova.

Três situações reprovam por construção: marca ausente em módulo que já existe em
`NX_BASE`; diretório de pacote publicado que mude de caminho; e marca cujo
`tagger` seja o autor do primeiro commit do módulo — a autorização precisa vir
de outra pessoa. Para criar a marca, alguém que **não** seja o autor do módulo
executa, após o merge:

```bash
git tag -a contracts-baseline/proto -m "baseline estabelecido" <commit-na-branch-principal>
git push origin contracts-baseline/proto
```

No Gitea, a tag protegida `contracts-baseline/*` (Settings → Tags) deve permitir
criação apenas às equipes `tech-leads` e `Owners`. Um repositório com um único
aprovador não consegue sair de `sem baseline`; a norma trata isso como
pré-requisito organizacional, não como defeito do gate.

## O que é e o que não é normatizado

- `proto/` e `fixtures/` seguem a norma integralmente: caminho espelha o pacote
  (`REP-01`), uma fixture por contrato `event` (`INT-02`), enum começa em
  `_UNSPECIFIED` (`PTB-09`), campo removido é `reserved` por número e nome
  (`PTB-06`).
- `openapi/` e `asyncapi/` existem porque a norma os registra (`REP-06`); nada
  aqui obriga sobre o seu conteúdo, versionamento ou gates.
- Semântica de entrega não é assunto deste diretório: ela é fixada pela RFC
  DMPF §2.3 e pelos artefatos de mensageria, e o gate `lint` só garante que
  nenhum arquivo daqui a contradiga.
