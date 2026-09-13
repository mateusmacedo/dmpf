# ADR-031: Decidir a regra de dependência do DMPF por execução, sobre o grafo resolvido

## Status

Aceito — 2026-09-01. Implementa SPEC-WTAXFV8B.

## Contexto

A regra de dependência do DMPF era, até aqui, quase inteiramente documental. O
ADR-010 a define como a função
`decide(source_block, target_block, source_bc, target_bc, target_surface)`,
conjunção de C1 (o par de blocos na matriz de RFC §7.3) e C2 (mesmo
`bounded_context` ou superfície pública no destino). Nada disso era executado.

O `KRN-01` entregou um gate parcial e **declarou a própria limitação em código**:
o `depguard` do `.golangci.yml` seleciona por nome de diretório, analisa um
módulo por vez e não vê a aresta entre módulos. Um `domain` que importe outro
módulo do workspace que por sua vez use I/O passa verde.

O inventário mostra onde isso custa: `goservice/domain` do `legado-golibs` declara
`EventPublisher` — a unidade mal dimensionada que o ADR-014 cita —, e
`goweb/domain/http_request.go:6` importa `net/http`, violando P0-1. Sem
execução, cada módulo que `KRN-03`..`KRN-12` criar nasceria sob revisão manual.

## Decisão

Entregar `dmpf-conformance`: uma CLI Go que decide a regra sobre o **grafo real
de imports**, lendo a classificação declarada nos manifestos `dmpf/units@1`, e
reprova o PR no CI.

A resolução do destino ocorre **antes** da classificação e sobre o package
resolvido pelo toolchain (`go list -e -deps -json`), nunca sobre o texto do
import (RFC §3.5). Alias e caminhos distintos para o mesmo package produzem a
mesma aresta. O `go/parser` entra apenas para nomear o arquivo de origem na
mensagem — jamais para decidir.

O verificador é implementado **na própria arquitetura que verifica**: `rule`,
`manifest` e `baseline` como `domain`; `port` como `port`; `conformance` como
`application`; `fsstore` e `golist` como `provider`; `cmd` como `app`. Isso não é
elegância — é o que torna o DoD "passa no próprio gate" um teste real. Um pacote
único passaria trivialmente, por não ter aresta a decidir.

| Decisão | Onde vive |
| --- | --- |
| Matriz 6×6 de RFC §7.3 como dado versionado | `internal/rule/matrix.go` |
| Política de capabilities de RFC §6.2 como dado | `internal/rule/capability.go` |
| Capability dos builtins | `internal/rule/stdlib.go` |
| Baseline fora de todo `ownership_module` (T1) | `tools/dmpf-baseline/units-baseline.json` |
| Perfis de produção como dado versionado | `build-profiles.json` |

### `observability` é permitida em `application`, por leitura do complemento

A tabela de RFC §6.2 descreve o bloco `application service` de forma assimétrica
em relação aos outros: onde `domain` e `port` dizem "apenas `pure`", `application`
diz "`pure`; nenhuma capability `io.*`, `runtime.framework` nem `wire.codec`".

`observability` não aparece nem na coluna de permitidas nem na lista de negadas.
As duas leituras são gramaticalmente possíveis:

| Leitura | Resultado |
| --- | --- |
| A coluna é exaustiva | só `pure`; `observability` negada |
| A lista de negadas é exaustiva | o complemento do conjunto fechado, isto é, `pure` e `observability` |

**O verificador adota a segunda.** Três razões:

1. A assimetria da redação é deliberada. `domain` e `port` recebem "apenas
   `pure`", que é fechamento explícito; `application` recebe uma enumeração de
   negadas, que é a forma de dizer "tudo menos isto".
2. A própria §6.2 traz uma cláusula normativa separada vedando telemetria em
   `domain` e em `port` — e **só neles**. Se `observability` já estivesse negada
   em `application` pela coluna, a cláusula teria mencionado os três.
3. Um caso de uso que não pode nem registrar log estruturado empurra a
   observabilidade para o adapter, o que contraria o princípio de que ela vive
   nos pipelines e providers, não que ela seja proibida acima do domínio.

A consequência é declarada: se a norma for depois esclarecida na direção da
primeira leitura, unidades `application` que declarem dependência de telemetria
passam a violar, e o dado em `capabilityPolicy` muda em uma linha. É por isso
que a política é dado versionado e não condicional espalhada pelo código.

Esta é a **única** ambiguidade normativa que o verificador resolve por
interpretação própria. Todas as outras decisões desta entrega seguem texto
explícito.

### O `include` casa por import path exato

Casar por prefixo faria `x/domain/infra` herdar a classificação de `x/domain`
pelo lugar do diretório — inferência por convenção, que ADR-012 e RFC §4.4
proíbem, e um caminho para classificar package novo sem edição revisável do
manifesto. A RFC dá glob ao binding TypeScript e **import path** ao Go (§10.1).
O custo é manifesto verboso: cada package é enumerado.

Além do casamento exato, a unidade só possui packages do **próprio**
`ownership_module`. Sem essa condição, unidade de módulo pai capturaria package
de módulo aninhado cujo import path compartilhe o prefixo, classificando-o pelo
módulo errado.

### A allowlist e as exceções são por módulo

O `id` de unidade só é único **dentro** de um manifesto (RFC §10.1), então dois
módulos podem declarar `id: "domain"` legitimamente. Unificar a política faria a
exceção que um módulo declarou para si autorizar outro — a exceção deixaria de
ser nominal e viraria a política paralela que RFC §6.4 proíbe.

### A pureza transitiva vale para a allowlist, não para builtins

RFC §6.3 atribui capability ao builtin diretamente (`net/http` é `io.network`,
`crypto` é `pure`) e aplica a transitividade às **entradas da allowlist**, a
partir dos entrypoints declarados. Descer o fechamento da stdlib chegaria sempre
em `internal/abi` e `internal/runtime/*`, tornando todo package puro impuro por
construção — verificado empiricamente: a primeira execução do verificador
devolveu 21 falsos positivos exatamente por isso.

### `GOWORK` é resolvido pela origem do módulo

Membro do `go.work` resolve **no** workspace, porque é assim que o build o
compila: um `require` entre membros sem `replace` local cairia na versão
publicada, e o verificador analisaria um grafo que não é o que se constrói.
Módulo fora do `go.work` resolve isolado. Nos dois casos o valor é explícito, de
modo que o veredicto não depende de onde o binário foi invocado.

### O digest do baseline é codificado por prefixo de comprimento

O digest existe para **fechar o conjunto** (T2): obrigar quem altera uma entrada
a recalculá-lo, tornando a mudança explícita em revisão. Uma codificação por
delimitador só é injetiva enquanto nenhum valor contém o delimitador — e o
baseline é um arquivo que quem abre o PR edita.

A primeira implementação separava campos por `0x1f` e juntava o `membership`
com o mesmo byte. Verificado: `membership: ["a\x1fb"]` e `["a", "b"]` produziam
os mesmos bytes e o mesmo digest. Sem prefixo por campo, `Module`/`Unit` também
colidem por deslocamento — `"m"+"u"` contra `"mu"+""`.

A codificação passou a emitir o comprimento antes de cada campo e de cada lista,
de modo que a injetividade não dependa do conteúdo. A colisão não era explorável
sozinha, porque a comparação entrada a entrada ainda a pegava; mas uma defesa
que só funciona por outra camada tê-la pegado não é a garantia que T2 descreve.

### O gate nasceu em três PRs, com ativação atômica

O corte é por implementação, não por arquivo: PR1 entregou o núcleo puro sem
gate; PR2, a extração e as capabilities, com um passo **shadow** report-only no
CI; PR3 ligou o gate fail-closed e **removeu o shadow no mesmo commit**, para que
não exista estado em que os dois convivam e o resultado dependa de qual deles
alguém leu. Ligar antes reprovaria por classes de diagnóstico que ainda não eram
decidíveis.

## Limitações declaradas

### `DMPF-E004` não é aplicável ao binding Go

A gramática exige `ImportPath = string_lit`, e import dinâmico não existe na
linguagem. O código permanece **reservado e estável** no conjunto dos dezesseis —
classe não implementada é ausência declarada de verificação, nunca "conforme" —
com teste de não-emissão. Carregamento dinâmico por API (`plugin.Open` e afins)
já cai na política de capabilities. `E004` volta a ter vetor executável no
binding TypeScript (`KRN-11`).

### `fmt` e `time` são classificados como `pure`, e isso deixa uma fresta

Os dois misturam símbolos puros e de I/O: `fmt.Sprintf` e `fmt.Errorf` são
computação, `fmt.Println` escreve em `os.Stdout`; `time.Duration` é tipo,
`time.Now()` é `io.clock`. RFC §3.3 fixa o **package** como unidade de
verificação em Go, e nessa granularidade não há como separar os dois.

A escolha é `pure`, alinhada ao `.golangci.yml` do `KRN-01`, que já os permite no
bloco `domain`. O custo é conhecido e aceito: **`fmt.Println` em unidade `domain`
não é detectado**. Distinguir por símbolo exigiria type-check de cada call site —
escopo que esta entrega não abre. O comentário do `.golangci.yml` atribui essa
distinção ao `KRN-02`; ela fica pendente e precisa de história própria.

### O mecanismo mínimo de RFC §10.2 cobre metade da autorização

O rito do ADR-028 — Autoridade de Classificação, evidência persistida,
enumeração das arestas recém-permitidas — **não está vigente**: depende
cumulativamente do aceite de um titular (G2), da revisão da indicação por
Segurança e do fechamento da ANC-08 (G5). Até lá vale o mecanismo mínimo, e o
próprio ADR-028 determina que as duas normas nunca se aplicam ao mesmo tempo.

O mínimo tem duas partes, e só uma é avaliável dentro do CI:

| Parte | Quem avalia |
| --- | --- |
| Mudança normativa em **commit próprio**, separado de código | O verificador, lendo `git log $NX_BASE..HEAD` |
| Aprovação do commit por **revisor distinto do autor** | A forge, na revisão do PR |

A segunda é estado que só existe **depois** de o CI rodar; exigi-la no gate
travaria o fluxo por construção, porque o PR nunca ficaria verde. O verificador
implementa a primeira e declara a fronteira. Quando o rito viger, a evidência
persistida passa a ser verificável no repositório e entra no gate.

Isso significa que o alcance do R1 aqui é o mesmo que a RFC §10.2 declara:
**parcialmente mitigado**. O ataque deixa de ser silencioso e passa a exigir
mudança visível e isolada; um aprovador desatento ainda basta.

### O `depguard` não alcança o próprio verificador

A regra `domain` do `.golangci.yml` seleciona por `**/*-domain/**`, e os pacotes
`domain` do `dmpf-conformance` não casam esse glob — verificado: `net/http` em
`internal/rule` devolve 0 issues no golangci-lint. O `tools/dmpf-gate-check.sh`
passou a declarar esses módulos como fora do seu alcance, em vez de reprová-los
por um gate que não os cobre. A proteção deles vem da autoverificação do próprio
`dmpf-conformance`.

### Vinte das 36 células não têm evidência no parque

O ADR-010 registra que 20 células não foram observadas no universo inventariado.
Os vetores negativos delas derivam de RFC §7.4, célula a célula, e não de código
observado. A independência do oráculo em relação ao dado de produção é garantida
por controle **estrutural** sobre o AST — um red control em runtime não
distinguiria oráculo transcrito de oráculo derivado, porque o derivado
snapshotaria no init do package.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Nome `dmpf-verify` | Ocupado por `tools/dmpf-verify.mjs` e pelo workflow `dmpf-verify.yml`, que verificam a congruência do acervo `docs/dmpf/` — outro gate, outro objeto |
| Parsear os imports com `go/parser` | Daria o texto do import, não a resolução — exatamente o que RFC §3.5 proíbe como base da decisão |
| `go list` sem `-e` | Package errôneo vai para stderr fora do JSON, e `DMPF-E003` deixa de ser emissível de forma estável. O `-e` é transporte estruturado de erro; o fail-closed é do consumidor |
| Descobrir módulos só pelo `go.work` | Módulo omitido dele — com `go.mod`, código de produção e projeto Nx — ficaria fora do universo e nunca geraria o `DMPF-U004` que existe para detectá-lo |
| `GOWORK=off` sempre | Determinístico, mas troca o grafo real por outro em membro do `go.work` que requer outro membro sem `replace` local |
| Perfil de produção implícito do runner | Mantém "produção" indefinida: trocar o runner mudaria o veredicto em silêncio |
| Baseline só de `canonical_key` → (`block`, `bounded_context`) | Não detecta remapeamento por `include` — o delta efetivo que o ADR-028 regula. O rito seria evitável por construção |
| Baseline dentro do módulo que descreve | Viola T1: o mesmo commit que muda a classificação mexeria nos dois lados, e a comparação nunca acusaria |
| Verificador em pacote único | Passaria no próprio gate trivialmente, por não ter aresta a decidir |
| Gate ligado desde o primeiro PR | Reprovaria por classes de diagnóstico ainda não decidíveis, ensinando o time a ignorar o gate |
| Remover a regra `depguard` do `.golangci.yml` | Deixaria o desenvolvedor sem sinal local rápido. Ela vira defesa em profundidade |

## Consequências

- O gate é fail-closed a partir deste PR: lacuna de configuração, arquivo não
  coberto, import não resolvido e manifesto ausente **reprovam**. Condição que o
  verificador não consegue avaliar é reportada como "não verificado", que também
  reprova.
- `KRN-03`..`KRN-12` nascem sob gate, e cada módulo novo precisa declarar o
  próprio `dmpf-units.json` enumerando os packages. Manifesto ausente é
  `DMPF-U004`; package não declarado é `DMPF-U001`.
- O baseline passa a ser artefato de revisão obrigatória: alterá-lo é ato
  regulado, e o digest força a mudança a ser explícita.
- Mudança normativa precisa vir em commit próprio. Um PR que misture
  reclassificação com código reprova em `DMPF-T002`.
- O custo no CI é linear em módulos × perfis. Medido: 0,29-0,34s com 2 módulos e
  1 perfil, sendo o `go list` o termo dominante (0,18s isolado). Com os 12
  módulos previstos, a projeção fica na casa de 2s.
- A tabela de capability da stdlib é mantida à mão e casa **exato**: package fora
  dela reprova por não-sabido em bloco `default deny`. Herdar por prefixo era
  inseguro — `io/ioutil` herdava `pure` de `io`.
- Enquanto o `KRN-11` não entregar o binding TypeScript, o gate cobre apenas Go.
