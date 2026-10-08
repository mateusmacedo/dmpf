# Kernel de contratos de wire do DMPF

Este módulo é o **kernel** dos contratos de integração: o envelope CloudEvents
vendorizado, o `envelope` e o `payloadhash` que o empacotam, o proto de teste
`dmpf.testing.v1` com as golden fixtures dele e os testes golden. O contrato de
cada contexto não mora aqui: ele é o módulo `apps/backend/<ctx>/contract`, com
`buf.yaml`, `buf.gen.yaml`, `proto/`, `openapi/`, `fixtures/` e `gen/go/`
próprios (ADR-054). A fonte `.proto` é neutra de stack; o pacote Go é aplicado
na geração, pelo managed mode (`BUF-10`).

A norma que este módulo implementa é `docs/dmpf/cloudevents-protobuf-buf.md`
(perfil CloudEvents `ENV-*`, política Protobuf `PTB-*`, governança Buf `BUF-*`,
layout `REP-*`, fixtures `INT-*`), com o codec e a fórmula do `payload_hash`
fixados em `docs/adr/022-codec-cloudevents-proto-data-e-payload-hash.md` e a
autoridade de validação em `docs/adr/023-autoridade-de-validacao-e-registry.md`.

## Árvore

```text
libs/backend/go/contracts/
├── buf.yaml            módulo publicado `proto` e módulo de teste `testdata/proto`, sem name: lint STANDARD, breaking FILE, deps vazias (BUF-01, BUF-02)
├── buf.gen.yaml        geração em managed mode para gen/go; plugin protoc-gen-go pinado (BUF-06, BUF-10)
├── proto/io/cloudevents/v1/cloudevents.proto  envelope oficial do CloudEvents, vendorizado (ENV-07)
├── testdata/proto/dmpf/testing/v1/*.proto     eventos de teste do kernel, sem vínculo com nenhum app e fora do buf-breaking
├── fixtures/testing/v1/*.golden              golden fixtures JSON dos eventos de teste (INT-01)
├── gen/go/                                   gerado, nunca editado à mão (REP-02)
├── envelope/, payloadhash/                   empacotamento CloudEvents e hash do payload
└── golden/                                   testes golden sobre o proto de teste
```

A semântica de `REP-02` vale aqui e em cada módulo de contrato: o gerado é
versionado, nunca editado à mão, e a divergência entre gerado e versionado
reprova o PR.

## Toolchain

Nenhum binário é instalado. A CLI entra por `tools/dmpf-plugin/scripts/buf.sh`, que executa
`go run github.com/bufbuild/buf/cmd/buf@<versão>`; o plugin `protoc-gen-go`
entra pelo `local` de `buf.gen.yaml`, também por `go run`. Os dois pins são
exatos; `tools/dmpf-plugin/scripts/buf-gate.sh pins` reprova `latest`, faixa ou divergência entre
a versão do plugin e a do runtime no `go.mod`, e exige o mesmo plugin em todos
os `buf.gen.yaml` do repositório.

Comandos do dia a dia, a partir da raiz do repositório (troque o diretório pelo
`apps/backend/<ctx>/contract` de um contexto):

```bash
bash tools/dmpf-plugin/scripts/buf.sh lint libs/backend/go/contracts
bash tools/dmpf-plugin/scripts/buf.sh format --diff --exit-code libs/backend/go/contracts
(cd libs/backend/go/contracts && bash ../../../../tools/dmpf-plugin/scripts/buf.sh generate)
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
sha256sum libs/backend/go/contracts/proto/io/cloudevents/v1/cloudevents.proto
```

O arquivo oficial carrega `option go_package` e opções de outras linguagens; o
managed mode as sobrescreve na geração, e por isso o arquivo não é editado.

## Gates e baseline

Os gates de `tools/dmpf-plugin/scripts/buf-gate.sh <cmd> <moddir> --project <nome>` rodam como
targets Nx de cada projeto de contrato — `contracts` e os `<ctx>-contract` — e
no CI; nenhum é advisory e nenhum tem bypass (`BUF-12`):

| Subcomando | O que prova |
|------------|-------------|
| `lint` | `buf format --diff --exit-code`, `buf lint` em `STANDARD` e varredura textual de P0-3 (nenhum artefato promete entrega única fim a fim) |
| `pins` | pin exato da CLI no `versions.json` do plugin, do plugin em `buf.gen.yaml`, igualdade plugin × runtime no `go.mod`, o mesmo plugin em todos os módulos e `buf.lock` presente quando há `deps` |
| `generate-check` | duas gerações idênticas byte a byte e ausência de drift entre gerado e versionado (`BUF-11`) |
| `breaking` | `buf breaking` em `FILE` contra `NX_BASE`, por pacote, sob a máquina de estados de `BUF-08` |

O subcomando `warmup` não é gate: compila a CLI e o plugin uma única vez, e os
targets `buf-lint`, `buf-generate-check` e `buf-gate-selftest` dependem dele
(`dependsOn`). Sem isso, os três `go run` a frio em paralelo estouravam o
timeout do job no runner.

O `breaking` identifica cada `.proto` publicado pelo pacote: um pacote que
mudou de módulo é comparado com o recorte dele em `NX_BASE`, e um pacote
publicado que some de todos os módulos reprova. O estado de cada módulo vem só
de `NX_BASE` (ADR-058):

- **`sem baseline`** — nenhum pacote do módulo está publicado em `NX_BASE`:
  `breaking` é dispensado só para ele, com aviso na saída. `lint`, `format`,
  `generate-check` e `pins` seguem obrigatórios.
- **`baseline estabelecido`** — algum pacote do módulo já está publicado em
  `NX_BASE`: `breaking` é obrigatório contra essa base, e `NX_BASE` vazio ou
  irresolvível reprova.

Não há tag de baseline nem ato pós-merge: o módulo passa a `baseline
estabelecido` no merge que o publica na branch-alvo.

## O que é e o que não é normatizado

- `proto/` e `fixtures/` seguem a norma integralmente: caminho espelha o pacote
  (`REP-01`), uma fixture por contrato `event` (`INT-02`), enum começa em
  `_UNSPECIFIED` (`PTB-09`), campo removido é `reserved` por número e nome
  (`PTB-06`).
- O `openapi/` de cada módulo de contrato existe porque a norma o registra
  (`REP-06`); nada aqui obriga sobre o seu conteúdo, versionamento ou gates.
- Semântica de entrega não é assunto deste módulo: ela é fixada pela RFC
  DMPF §2.3 e pelos artefatos de mensageria, e o gate `lint` só garante que
  nenhum arquivo daqui a contradiga.
