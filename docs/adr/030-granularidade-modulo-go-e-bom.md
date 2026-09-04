# ADR-030: Fixar um módulo Go por lib, com BOM declarado e segregação de stack no caminho

## Status

Aceito — 2026-08-31. Implementa SPEC-MQA5HAXF.

## Contexto

O repositório não tinha terreno Go: o `go.work` declarava apenas `go 1.25`, sem
bloco `use`, e não havia nenhum `go.mod` nem arquivo `.go` fora de
`node_modules`. O `ci.yml` não instalava Go e o `lefthook.yml` não cobria `*.go`.

A `SPEC-YRJRADY9` prevê onze módulos Go para o kernel DMPF, e a
[RFC](../dmpf/rfc-dmpf-foundation-v0.1.md) impõe duas exigências que o terreno
precisa satisfazer desde o primeiro módulo: §3.6 obriga todo `ownership_module`
de produção a portar um `metadata_container`, e §5.4 exige que a `canonical_key`
— em Go, o import path completo — seja **estável**. Decisão de layout tomada
tarde é reescrita de `canonical_key` em cascata.

O épico de ordem 2 do ARQ-436 entrega o kernel **TypeScript**, com os mesmos
nomes conceituais do kernel Go (`dmpf-domain`, `dmpf-application`, `dmpf-ports`,
`dmpf-contracts`). No Nx o nome de projeto é chave única no workspace, então as
duas stacks não podem ocupar `libs/backend/dmpf-domain`.

## Decisão

**Um `go.mod` por lib.** Cada lib Go é um `ownership_module` próprio, o que faz
módulo e projeto Nx coincidirem. Essa coincidência entrega de graça o manifesto
por lib, o versionamento independente no Nx Release e um alvo natural para
`nx affected`.

**Segregação de stack no caminho.** Módulos ficam em
`libs/<scope>/<stack>/<módulo>`, e o nome do projeto Nx leva o sufixo da stack.
O primeiro é `libs/backend/go/dmpf-domain`, projeto `dmpf-domain-go`. A raiz por
`scope` que o `AGENTS.md` documenta (`libs/backend`, `libs/frontend`,
`libs/shared`) é preservada; a dimensão `stack:` da tag 3D continua sendo a fonte
canônica para filtros do Nx, e o caminho apenas deixa de colidir.

**BOM declarado.** Três versões fixadas, cada uma em um lugar só:

| Item | Versão | Onde é declarada |
|------|--------|------------------|
| Go (piso e toolchain) | `1.26.4` | `go.work`, `go.mod` e o `go-version-file` do CI |
| golangci-lint | `v2.13.2` | `options.args` do target `lint` |
| govulncheck | `v1.7.0` | `command` do target `govulncheck` |

As ferramentas entram por `go run <pacote>@<versão>` inline nos targets: a versão
fica idêntica local e no CI, sem poluir o `go.mod` do domínio com dependência de
ferramenta. O CI lê o piso do `go.work` por `go-version-file`, e não de uma
versão literal no workflow, para não abrir uma segunda fonte da verdade.

> **Addendum (2026-09-04)** — o piso e o toolchain Go passaram a `1.26.8`
> (ARQ-528, KRN-09). No piso `1.26.4` o `govulncheck v1.7.0` reprova assim que
> os exportadores OTLP entram no grafo: GO-2026-6090 e GO-2026-5856 em
> `crypto/tls`, GO-2026-5972 em `encoding/asn1`. O `1.26.8` remove a
> alcançabilidade. Os três lugares de declaração da tabela continuam os mesmos, e
> a decisão de fonte única não muda — só o valor da primeira linha.

**`package.json` privado no módulo Go.** O módulo carrega um manifesto npm
mínimo — `name`, `version: 0.0.0` e `private: true`. Ele existe por uma razão
mecânica verificada no código instalado: o `JsVersionActions` do `@nx/js` declara
`validManifestFilenames = ['package.json']`, e `readCurrentVersionFromSourceManifest`
lança erro quando o arquivo não existe. Como o `nx.json` usa
`fallbackCurrentVersionResolver: "disk"`, um projeto com `tag:type:lib` sem
`package.json` faz `nx release` **abortar no versionamento** — antes de qualquer
filtro de publicação agir. O `private: true` também exclui o módulo da publicação
por conta própria; o filtro `!tag:stack:go` do `nx-publish-libs.yml` permanece
como redundância deliberada.

**Gate de dependência por `.golangci.yml`.** O bloco `domain` é `default deny`
(RFC §6.2), e o que materializa isso é `list-mode: strict` com uma allowlist
fechada de pacotes puros. A distinção não é cosmética: sem `list-mode`, o
depguard opera em modo permissivo e libera tudo que não esteja explicitamente
na `deny` — `syscall`, `encoding/xml`, Zap, Redis, qualquer SDK entrariam no
domínio sem serem desafiados. A `deny` permanece, mas com outro papel: dar
mensagem explicativa por capability (RFC §7) quando o import barrado for um dos
casos previstos. O `.golangci.yml` entra nos `inputs` do target `lint`, o que faz
alterar a política invalidar o cache: sem isso, afrouxar a regra devolveria um
verde antigo.

**Lint e teste em `targetDefaults` por executor.** O wiring do golangci-lint e os
inputs de teste vivem em `nx.json`, sob as chaves `@nx-go/nx-go:lint` e
`@nx-go/nx-go:test`, não no `project.json` de cada módulo. O motivo é um modo de
falha silencioso: um projeto Go sem esse bloco roda o `lint` inferido do plugin,
que é `go fmt ./...` — **reescreve** os arquivos e sai 0. O módulo ficaria verde
para sempre sem lintar nada. Pelo mesmo mecanismo, `@nx-go/nx-go:test` declara
`inputs: ["go", "^go"]`: o `test` inferido traz só os inputs locais
(`generate-targets.js`), então um módulo B que dependa de A não rehasheia quando
apenas a implementação de A muda, e `B:test` poderia sair do cache sem testar a
versão nova. Ao editar esses defaults, `dependsOn` e `cache` precisam ser
repetidos — o default por executor substitui o default por nome, não se soma a
ele.

**Divergência de taxonomia registrada.** O `ADR-002` (linha 39) descreve a
dimensão de stack como `stack:(node|react|angular|universal)` — lista anterior à
entrada de Go, Express, Fastify, Nest e Next no workspace. O `AGENTS.md` traz a
tabela atual e é **a fonte da verdade** da taxonomia; o `ADR-002` não é editado
retroativamente, e esta ADR registra a divergência para que a lista antiga não
seja lida como restrição vigente.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Um `go.mod` único para todo o kernel | Colapsaria os onze módulos em um `ownership_module`, com um manifesto só, enfraquecendo a fronteira de ownership que a RFC §3.6 usa para portar a classificação |
| `versionActions` customizado para Go | Conceitualmente mais correto — versionaria a partir do `go.mod`, sem manifesto npm em módulo Go —, mas é código de release novo a escrever e manter dentro de uma tarefa de terreno. Reavaliar quando houver mais de uma lib Go |
| Tirar Go do `release.projects` | Sem risco, mas desfaria o versionamento independente que motivou um `go.mod` por lib, e contraria o critério de aceite da ARQ-520 |
| Sufixo de stack no módulo (`dmpf-domain-go/`) | Levaria a stack para dentro da `canonical_key` de cada package, e não apenas para a fronteira de ownership |
| `libs/<stack>/<scope>/` | Segue o precedente de `shared-ro-sync-services/libs/node` (RFC §10.1), mas inverteria a hierarquia por `scope` já documentada no `AGENTS.md` |
| Binário do golangci-lint instalado no CI | Duplicaria a versão entre o target e o instalador, abrindo divergência entre o que roda local e o que roda no CI |
| Formatar no `pre-commit` com `gofmt -w` | Simétrico ao `biome`, mas reescreveria o código sem o autor ver. O gancho reprova e mostra os arquivos fora do formato |

## Consequências

**Positivas:**

- Todo módulo criado a partir de `KRN-02` nasce em terreno verificado: os sete
  passos da cadeia (`gofmt`, `go vet`, `golangci-lint`, `go build`, `go test`,
  `go test -race`, `govulncheck`) rodam por `pnpm nx`, no commit, no push e no CI.
- A regra de dependência do bloco `domain` é exercida por máquina. O
  `tools/dmpf-gate-check.sh` prova em cada execução do CI que quatro capabilities
  vedadas são reprovadas **pelo depguard** — e que a árvore limpa passa.
- O caminho e o nome do projeto comportam a chegada do kernel TypeScript sem
  reescrever nenhuma `canonical_key`.
- O `nx release --dry-run` conclui com o primeiro `tag:type:lib` presente.

**Negativas:**

- O escopo da regra do depguard usa `files: '**/*-domain/**'`, ou seja,
  classifica por nome de diretório — critério que a RFC §4.4 rejeita. É um proxy
  interino consciente: o classificador autoritativo é o `dmpf-units.json`, lido
  pelo verificador que chega em `KRN-02`. Uma unidade declarada `block: domain`
  em diretório com outro nome fica fora da regra.
- O gate **não enxerga a aresta entre módulos**. O golangci-lint analisa um
  módulo por vez, então um `domain` que importe outro módulo do workspace que
  use I/O passa verde: o import interno não está na `deny`, e os arquivos do
  outro módulo não entram no `./...` analisado. O enforcement transitivo da
  RFC §6.3 é do `KRN-02`; esta entrega não o reivindica.
- A tag gerada pelo Nx Release é `dmpf-domain-go@0.1.0`, mas o proxy de módulos
  do Go exige `libs/backend/go/dmpf-domain/vX.Y.Z` para módulo em subdiretório.
  O module path foi reescrito para o host Gitea pensando em consumo remoto, e o
  versionamento roda — mas o esquema de tag que tornaria o módulo buscável por
  versão ainda não existe. Enquanto não existir, o `version` do `package.json`
  não é lido por nenhum consumidor Go.
- Não há cache de módulos Go no CI: o `setup-go` declara `cache: false`, porque
  o cache nativo depende de um `go.sum` na raiz do workspace, que não existe.
  Cada execução rebaixa o grafo do golangci-lint e do govulncheck.
- O gate de dependência custa uma execução do golangci-lint por vetor, por
  módulo: hoje são 21s para um módulo, e a projeção para os onze módulos da
  guarda-chuva é de cerca de 4 minutos, contra o `timeout-minutes: 30` do CI.
  Cabe, mas cresce de forma linear e soma à compilação fria das ferramentas,
  já que não há cache de módulos. Quando o verificador do `KRN-02` existir,
  a checagem por vetor deve dar lugar a uma análise única do grafo.
- O pacote `time` **não** entra na denylist. O depguard casa import path, não
  símbolo, e vedar o pacote inteiro barraria `time.Duration` e `time.Time` como
  tipos puros, legítimos no domínio. `time.Now()` é `io.clock` e entra por porta,
  mas essa distinção só é verificável semanticamente — também em `KRN-02`.
- O manifesto `dmpf-units.json` tem, até `KRN-02`, apenas validação estrutural.
- O módulo Go carrega um `package.json`, artefato estranho à stack, e passa a
  figurar como importer no `pnpm-lock.yaml`. É o preço de manter Go no Nx Release.
- A combinação que faz o target `lint` rodar golangci-lint — `linter: "go"` com
  `args` começando em `run` — usa o executor do `@nx-go/nx-go` de uma forma que
  ele permite mas não documenta. Um bump do plugin pode quebrá-la, e o sintoma
  seria o linter rodando `go fmt` de novo, em silêncio.
- O comportamento do `gitea-runner` só é exercido na abertura do PR: se ele não
  resolver `actions/setup-go@v5` ou não tiver egress a `proxy.golang.org`, o
  plano B é pré-instalar o toolchain na imagem do runner ou versionar um cache
  de módulos, mantendo o `go-version-file` como fonte do piso.
