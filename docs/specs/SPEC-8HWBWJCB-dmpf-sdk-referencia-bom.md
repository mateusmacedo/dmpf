---
id: SPEC-8HWBWJCB
slug: dmpf-sdk-referencia-bom
title: DMPF KRN-12 — SDK de referência, generator Nx e BOM certificado (guarda-chuva)
stage: planning
priority: P2
depends_on: [SPEC-MQA5HAXF, SPEC-WTAXFV8B, SPEC-XF9TF9A0, SPEC-ZHE7DN1H, SPEC-WYX5GW87, SPEC-3R80KNMS, SPEC-ANZX2WPG, SPEC-CGPX20NP, SPEC-NYD18TGD, SPEC-EAGAXQN1, SPEC-SJ66880S]
ticket_url: null
subtask_urls: [ARQ-545, ARQ-546, ARQ-547, ARQ-548]
created: 2026-09-08
---

# SPEC-8HWBWJCB: DMPF KRN-12 — SDK de referência, generator Nx e BOM certificado (guarda-chuva)

## Resumo

Ao fim de `KRN-11` o kernel existe em quatorze módulos Go, com verificador,
contratos, providers e test kits — mas ninguém o consome como produto. Não há um
serviço que mostre como compor os blocos, não há ferramenta que scaffolde um
bounded context conforme, e não há registro de qual combinação de versões foi de
fato exercitada: o BOM de FND-10 está normatizado, mas nenhuma instância existe.

Esta spec é a **guarda-chuva** de `KRN-12`. Ela fixa as decisões transversais —
a identidade da release do produto DMPF, o que "sem edição manual" significa para
um artefato gerado, um transporte assíncrono por processo, o catálogo fechado do
que nenhuma exceção alcança — e decompõe a entrega em quatro sub-specs, cada uma
com task própria no épico ARQ-519, branch própria e plano próprio:

| Ordem | Sub-spec | Entrega | Depende de |
| --- | --- | --- | --- |
| 1 | `SPEC-6QT9SBAS` (ARQ-545) — composition root `dmpf-reference` | `apps/backend/dmpf-reference`, contrato OpenAPI de `orders`, `orderspg.NewReader`, targets `serve-*`, exclusão do release Docker | as onze sub-specs de `KRN-01` a `KRN-11` |
| 2 | `SPEC-H1A190Y8` (ARQ-546) — generator `bounded-context` | plugin local `tools/dmpf-plugin`, generator, prova mecânica em worktree com dois commits e `--base`, guia de composição | `SPEC-MQA5HAXF`, `SPEC-WTAXFV8B`, `SPEC-SJ66880S` |
| 3 | `SPEC-538MS2D4` (ARQ-547) — modelo do BOM, validador e escape hatch | schema `dmpf/bom@1`, `dmpf-bom` (`B001`..`B010`), exceção com `history[]` e admissão `X001`..`X007` ponta a ponta no verificador | `SPEC-WTAXFV8B`, `SPEC-VVR1X71Q` |
| 4 | `SPEC-JPP31095` (ARQ-548) — evidência e certificação da release `0.1.0` | `cmd/dmpf-evidence` por API, `bom/evidence/`, o primeiro BOM certificado, tag `dmpf@0.1.0`, ADR-041 consolidado, `AGENTS.md` | 1, 2 e 3 |

O porquê é o do ADR-027: a combinação que quebra é a que nunca foi exercitada.
Como usuário de uma squad, quero chegar de um comando a um serviço aprovado pelo
verificador, e saber, lendo o repositório, quais versões estão certificadas
juntas e por qual evidência.

## Contexto

- **Problema**: os módulos `KRN-01` a `KRN-11` estão entregues, mas (1)
  `apps/backend` é um `.gitkeep`, sem serviço que os cabeie em um processo real;
  (2) `tools/generators/` é um `.gitkeep`, e criar um bounded context é copiar à
  mão cinco `project.json`, cinco `go.mod`, cinco `dmpf-units.json` e a entrada
  no `go.work`; (3) o BOM existe só como tabela de três linhas no ADR-030, sem os
  seis itens de FND-10 §4.1, sem máquina de estados e sem evidência; (4) o escape
  hatch de `GOV-30` a `GOV-36` não tem instrumento — o `exceptions[]` do
  `dmpf/units@1` carrega cinco campos, e nenhum é o plano de convergência com
  prazo.
- **Impacto**: uma squad passa a ter um caminho de um comando até um serviço
  rodando; o BOM diz quais versões usar e por qual evidência; divergir do golden
  path continua permitido, mas é ato rastreado com prazo, recusado na admissão
  quando toca constraint P0.
- **Inspiração**: `libs/backend/go/dmpf-app/example/reservations/` já cabeia
  Postgres, UoW, consumer e relay em miniatura; o guia
  `docs/guides/dmpf-manifesto.md` é o gesto que o generator automatiza; os
  `chore(workspace): [ARQ-xxx] Registrar ... no baseline` são o precedente do
  rito em commit próprio que a certificação copia.
- **Links relevantes**:
  - `SPEC-YRJRADY9` — spec guarda-chuva do épico ARQ-519; `KRN-12` é a última
    das doze histórias (incremento 5 — Entrega)
  - `SPEC-VVR1X71Q` — FND-10, que deixou "o preenchimento do BOM com versões
    reais" e "generators" para os épicos de tooling
  - `SPEC-SJ66880S` — `KRN-11`, cujo `Report` é a evidência que `KRN-12` consome
  - `SPEC-CGPX20NP` — `KRN-08`, que reservou o nome `dmpf-reference`
  - `docs/dmpf/governanca-bom-pilotos.md` — FND-10, §3, §4 e §5.1
  - ADR-012, ADR-015, ADR-027, ADR-030, ADR-031, ADR-034, ADR-039, ADR-040
  - `docs/guides/dmpf-manifesto.md`

### Divergências entre o ticket e o repositório

| O ticket diz | O repositório tem | O que esta guarda-chuva adota |
| --- | --- | --- |
| "Tamanho sugerido: M" (3 SP) | a entrega são cinco subsistemas: app com e2e multiprocesso, plugin Nx, modelo e validador de BOM, evidência, ciclo de vida de exceção | Decomposição em quatro sub-specs com task própria; `KRN-12` fecha quando a quarta fecha (decisão do usuário em 2026-09-08, após revisão externa) |
| "cabeia os providers de `KRN-10`" | `contracts/openapi/` vazio; sem serviço gRPC em `contracts/proto/`; `Route.ContractRef` (RST-04) exige contrato publicado | HTTP (com contrato OpenAPI mínimo), Kafka e Postgres. gRPC fica fora (rito Buf); SQS fica fora pela decisão sobre `TRP-09` |
| não menciona `TRP-09`/`TRP-46` | ADR-039 fixa prazo "antes do `KRN-12`" sem addendum | **Um transporte assíncrono por processo** (decisão do usuário em 2026-09-08); addendum no ADR-039 e task sucessora **ARQ-549** (`KRN-13`) |
| "aprovado pelo verificador, sem edição manual" | criar unidade é ato regulado (`AUT-01` A3); sem `--base` o verificador devolve "não verificado" (`conformance/check.go:188`); a autorização é lida nos commits `base..HEAD` | "Sem edição manual" = nenhum byte gerado muda entre o generator e a aprovação. A prova faz dois commits num worktree (classificação; código) e verifica com `--base`; o generator nunca regrava o baseline |
| "BOM da release" | não há release do produto DMPF: o `nx release` versiona projetos `type:lib` de forma independente; nenhuma tag existe; `package.json` em `0.0.0` | **Identidade do produto** definida abaixo: tag anotada `dmpf@<semver>`, um BOM por tag |
| "guia em `docs/dmpf/`" | norma em `docs/dmpf/`; guias operacionais em `docs/guides/` | `docs/guides/dmpf-composicao.md`, indexado em `docs/dmpf/README.md` |
| "`AGENTS.md` reflete o inventário real" | `AGENTS.md` e `dmpf-app/README.md` descrevem 2 unidades no `dmpf-app`; o manifesto tem 3 | A divergência preexistente fecha na sub-spec 1 |
| "consumo fora do workspace com `require` versionado" (ADR-034) | `go.mod` sem `require` de irmão; tags do `nx release` (`<projeto>@<versão>`) não são tags de módulo Go (`<path>/vX.Y.Z`) | Fora de `KRN-12`, com task sucessora **ARQ-550** (`KRN-14`) e addendum no ADR-034 com owner, dependência e aceite |

### Fontes normativas

| Fonte | O que fixa |
| --- | --- |
| FND-10 §3.2 (`docs/dmpf/governanca-bom-pilotos.md:481-488`) | O produto DMPF é sujeito versionado com release própria, "à qual corresponde exatamente um BOM publicado" |
| FND-10 §4.1, `BOM-01` a `BOM-10` | Seis itens; schema por entrada; cinco estados; evidência; validade; referência sem duplicação; slot de semantic conventions |
| FND-10 `GOV-26`, `GOV-30` a `GOV-36` | Escape hatch: quatro itens, universo E1–E3, negação N1–N7 na admissão, exceção nominal, ciclo de vida com ramo de revisão, evidência persistida, métricas |
| FND-10 §5.2 `AUT-01` | Criar unidade é ato regulado em commit próprio |
| RFC §10.2 T1–T6; ADR-028; ADR-031 | `DMPF-T001`/`T002`; o verificador é o gate autoritativo e não conserta o próprio insumo |
| ADR-015 | Provider concreto só no composition root (bloco `app`) |
| ADR-012 | `block` e `bounded_context` declarados, nunca inferidos |
| ADR-027 | BOM por combinação certificada; escape hatch de universo fechado |
| ADR-030 | `private: true` exclui publicação, **não** versionamento; `nx release` aborta sem `package.json` |
| FND-04 §5.4 / `BLK-02` | Relay em processo próprio |

<constraints>
- [P0] NUNCA instanciar provider concreto fora do bloco `app` (ADR-015).
- [P0] NUNCA inferir `block` ou `bounded_context` do nome do diretório (ADR-012).
- [P0] NUNCA promover entrada do BOM a `certificada` sem `evidence_uri`, `evidence_digest`, `approved_by`, `certified_at` e `valid_until`, nem listar em `compatible_with` versão sem execução registrada (`BOM-03`, `BOM-04`, `BOM-07`).
- [P0] NUNCA admitir exceção sem os quatro itens de `GOV-30` nem exceção cujo objeto esteja no catálogo N1–N7 de `GOV-32`; a admissão é decidida por catálogo fechado, nunca por texto livre.
- [P0] NUNCA deixar o generator regravar `tools/dmpf-baseline/units-baseline.json` (`AUT-01`, ADR-031).
- [P0] NUNCA declarar exactly-once em nenhum artefato de `KRN-12`.
- [P1] Um transporte assíncrono por processo no `dmpf-reference` (Kafka); `TRP-09`/`TRP-46` são de task sucessora.
- [P1] Toda decisão transversal desta guarda-chuva é citada pela sub-spec que a aplica, nunca reescrita.
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Identidade da release do produto DMPF**: a release é uma **tag git
  anotada** `dmpf@<semver>` em `master`, cunhada pelo rito do BOM — o PR que
  promove o BOM da release é mergeado e a tag aponta para o merge commit. Um BOM
  por tag, em `bom/dmpf/<semver>.json`. A primeira release é `dmpf@0.1.0`. A
  tag é distinta das tags por projeto do `nx release` (`<projeto>@<versão>`),
  que continuam a existir; o BOM referencia ambas.
  - `version` de cada entrada é o valor **efetivo** no commit da tag: para
    módulos `type:lib`, o `version` do `package.json` (`0.0.0` até o primeiro
    `nx release`); para `dmpf-reference` (`type:app`, não versionado), o SHA
    curto do commit; para dependências externas, a versão do `go.mod`.
    Certificar uma combinação não exige que a versão seja "bonita" — exige que
    seja a exercitada.
  - Edge case: dois arquivos em `bom/dmpf/` sem tag correspondente reprovam no
    validador (sub-spec 3, `B011`).
- [ ] **[P0] "Sem edição manual" para artefato gerado**: significa que nenhum
  byte escrito pelo generator muda entre a geração e a aprovação pelo
  verificador. O rito de classificação (`--write-baseline` em commit próprio) é
  ato humano exigido por `AUT-01`, não edição. A prova mecânica (sub-spec 2)
  commita os arquivos gerados, roda a cadeia e falha em `git diff --exit-code`.
- [ ] **[P0] Catálogo fechado N1–N7**: a sub-spec 3 define, para cada item de
  `GOV-32`, como o objeto é reconhecido e o diagnóstico que o recusa; nenhum
  pedido escapa por identidade textual não prevista, porque o universo positivo
  (`kind` ∈ E1–E3) é fechado e o objeto é classificado por regra, não por nome.
- [ ] **[P0] Ciclo de vida da exceção com histórico**: toda exceção carrega
  `history[]` (`granted`, `renewed`, `revoked`, `converged`, com data, autor e
  motivo), e as métricas de `GOV-36` derivam do histórico — vigentes, renovações
  por exceção, vencidas sem convergência.
- [ ] **[P1] Decomposição em quatro sub-specs** com as fronteiras da tabela do
  Resumo; cada uma tem task própria no Jira sob ARQ-531 (criadas na Etapa 2),
  branch própria e plano próprio. `subtask_urls` desta guarda-chuva recebe as
  quatro URLs.
- [ ] **[P1] Tasks sucessoras nomeadas**: `TRP-09`/`TRP-46` (ADR-039) é
  **ARQ-549** (`KRN-13`) e consumo fora do workspace (ADR-034) é **ARQ-550**
  (`KRN-14`), ambas no épico ARQ-519, com owner e critério de aceite; os
  addenda dos ADRs citam os IDs.
- [ ] **[P1] Ordem de execução**: 1 → 2 → 3 → 4. A sub-spec 2 depende
  logicamente da 1 pela abordagem do ticket ("o composition root é a
  especificação executável do que o generator precisa produzir"), mas não há
  dependência de código; as duas podem correr em paralelo se houver capacidade.

### Não-funcionais

- [ ] Cada sub-spec fecha com a cadeia do workspace verde (`fmt-check`, `vet`,
  `lint`, `build`, `test-race`, `govulncheck`, `biome ci`, `adr-verify`,
  `dmpf-verify`) e o verificador `dmpf-conformance` sem diagnóstico.
- [ ] Nenhuma sub-spec adiciona dependência Go nova; a sub-spec 2 adiciona
  `@nx/plugin` e `@nx/devkit` (npm), declaradas no BOM pela sub-spec 4.

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | Por qual sub-spec |
| --- | --- | --- |
| `domain` | [x] | 3 (`internal/exception` como unidade `domain` do `dmpf-conformance`) |
| `application` | [ ] | — |
| `port` | [ ] | — |
| `contract` | [x] | 1 (`contracts/openapi/orders/v1/openapi.yaml`) |
| `provider` | [x] | 1 (`orderspg.NewReader`); 3 (`fsstore` decodifica a exceção completa) |
| `app` | [x] | 1 (`dmpf-reference`); 3 (`bom`, `cmd/dmpf-bom`); 4 (`evidence`, `cmd/dmpf-evidence`) |
| Workspace | [x] | 1 (`go.work`, `nx-release.yml`); 2 (`tools/dmpf-plugin`, `package.json`, lockfile, `ci.yml`); 3 (`bom/`, `ci.yml`); 4 (tag, `AGENTS.md`, ADR-041) |

## Localização de código

Cada sub-spec traz a sua árvore. A guarda-chuva só fixa os pontos de
encontro:

```text
bom/                                       — sub-spec 3 cria README.md e o schema; sub-spec 4 cria dmpf/0.1.0.json e evidence/0.1.0/
docs/adr/041-sdk-de-referencia-generator-e-bom-certificado.md — sub-spec 1 cria com as decisões transversais desta guarda-chuva; 2, 3 e 4 acrescentam addenda
docs/guides/dmpf-composicao.md             — sub-spec 2 cria; sub-spec 4 acrescenta "como certificar"
AGENTS.md                                  — cada sub-spec atualiza a sua entrada; sub-spec 4 consolida
tools/dmpf-baseline/units-baseline.json    — cada sub-spec que cria unidade regrava em commit próprio
```

## Design

### Arquitetura

```text
 sub-spec 1                       sub-spec 2                          sub-spec 3                          sub-spec 4
 dmpf-reference (app)             tools/dmpf-plugin                   dmpf-conformance/{bom,exception}    dmpf-testkit/evidence
 HTTP → orders → UoW(pg) → outbox bounded-context → 5 módulos         dmpf/bom@1, B001..B011              cmd/dmpf-evidence → bom/evidence/
 relay → Kafka → consumer → inbox go.work, manifesto declarado        exceção + history, X001..X007       digest → BOM certificado
 NewReader(pool), OpenAPI         dmpf-generator-check.sh (2 commits) fsstore→manifest→admissão→policy    tag dmpf@0.1.0, ADR-041 final
        │                                   │                                   │                                   ▲
        └───────────────── entradas do BOM (identity, version efetiva) ─────────┴───────────────────────────────────┘
```

### Fluxo — do ticket à release

1. Sub-spec 1 entrega o serviço de referência e fixa, no ADR-041, a identidade
   da release e as demais decisões transversais.
2. Sub-spec 2 entrega o generator e prova, no CI, que o artefato gerado é
   aprovado sem edição.
3. Sub-spec 3 entrega o modelo do BOM, o validador e o escape hatch; o CI
   passa a rodar `dmpf-bom` sobre um BOM ainda sem entradas `certificada`.
4. Sub-spec 4 gera a evidência, promove as entradas com execução real,
   consolida a documentação, e a tag `dmpf@0.1.0` fecha `KRN-12`.

## Decisões técnicas

- **Tag anotada `dmpf@<semver>` como release do produto** porque FND-10 §3.2
  exige um sujeito "Produto DMPF" com release própria e um BOM por release, e o
  `nx release` versiona projetos independentes — não há eixo agregado. A tag é
  o ato declarado que `BOM-07` pede. Alternativa descartada: release group do
  `nx release` para o produto, porque ele exige `package.json` com versão
  agregada e conflitaria com o versionamento independente do ADR-030.
- **`version` efetiva, não pretendida** porque `B007` da sub-spec 3 confere a
  versão contra o registro autoritativo no commit da tag; declarar `0.1.0` para
  módulos em `0.0.0` seria defeito de evidência (`BOM-04`). Alternativa
  descartada: rodar `nx release` antes do BOM, porque o release de libs é rito
  próprio (`nx-release.yml`) e o BOM não pode depender dele para existir.
- **Quatro sub-specs em vez de uma** porque a revisão externa demonstrou que
  os cinco subsistemas somam mais de trinta arquivos novos e quinze
  modificados, com blockers independentes entre si; um plano único não teria
  gate intermediário. Alternativa descartada: reduzir o escopo do `KRN-12`,
  porque os seis critérios do ticket cobrem os cinco subsistemas.
- **Um transporte assíncrono por processo** (decisão do usuário). Alternativa
  descartada: Kafka + SQS com binding persistido, porque toca `KRN-06`,
  `KRN-08` e `KRN-10` fora de rito.
- **Catálogo fechado N1–N7 com reconhecimento por regra** porque `GOV-32`
  nega "na admissão, sem exame de mérito", e isso só é decidível se o objeto for
  classificado por estrutura (`kind`, bloco da unidade, capability, prefixo de
  identificador normativo), nunca por leitura do texto de `justification`.

## Regras relacionadas

- `AGENTS.md` — tags 3D e `layer:*`, targets Go via `nx:run-commands`, commits
  por projeto.
- `.claude/rules/process-enforcement.md` — spec antes de código; plano por
  sub-spec.
- `docs/guides/dmpf-manifesto.md` — rito de classificação.
- `SPEC-6QT9SBAS`, `SPEC-H1A190Y8`, `SPEC-538MS2D4`, `SPEC-JPP31095` — as
  sub-specs desta guarda-chuva.

## Verificação e testes

### Critérios de aceite

Os seis primeiros são os do ticket ARQ-531, verbatim, com a sub-spec que os
prova.

- [ ] Um bounded context gerado, sem edição manual, compila, passa na cadeia Go
  e é **aprovado** pelo verificador, com uma tag de cada dimensão por projeto e
  a unidade declarada no manifesto. → sub-spec 2
- [ ] O composition root executa a escrita e a drenagem fim a fim, com estado de
  negócio e registro de outbox na mesma transação. → sub-spec 1
- [ ] O BOM está versionado e é revisável por PR; nenhum dos seis itens está
  ausente, e item sem instância aparece declarado vazio. → sub-specs 3 e 4
- [ ] Nenhuma entrada `certificada` existe sem `evidence_uri`, `evidence_digest`,
  `approved_by` e `valid_until`; entrada vencida é lida como `candidata`, e toda
  versão em `compatible_with` tem execução na suíte de `KRN-11`. → sub-specs 3 e 4
- [ ] Pedido de escape hatch sem plano de convergência **com prazo**, ou sobre
  constraint P0, é recusado na admissão. → sub-spec 3
- [ ] O `AGENTS.md` reflete o inventário real, nenhum artefato declara
  exactly-once, e a validação do workspace passa. → sub-spec 4 (consolida)
- [ ] `subtask_urls` desta guarda-chuva lista as quatro tasks; as quatro
  sub-specs estão `done`; a tag `dmpf@0.1.0` existe e aponta para o merge do BOM.
- [ ] Tasks sucessoras de `TRP-09`/`TRP-46` e do consumo fora do workspace
  existem no épico, citadas nos addenda dos ADR-039 e ADR-034.

### Cenários de teste

Os cenários executáveis vivem nas sub-specs. Os desta guarda-chuva são de
integração entre elas:

- Dado as sub-specs 1, 2 e 3 `done`, quando a sub-spec 4 roda
  `dmpf-evidence` e `dmpf-bom`, então o BOM `0.1.0` lista `dmpf-reference` com
  `version` = SHA curto e os módulos do kernel com a versão do `package.json`,
  e o validador sai com 0.
- Dado a tag `dmpf@0.1.0` cunhada, quando `dmpf-bom --root .` roda sem
  `--release`, então resolve o único BOM e o casa com a tag.
- Dado um bounded context gerado pela sub-spec 2 e o BOM da sub-spec 4, quando
  a squad segue o guia de composição, então chega ao serviço rodando sem passo
  fora do guia (revisão manual, registrada no PR da sub-spec 4).

<critical_constraints>
- [P0] NUNCA instanciar provider concreto fora do bloco `app` (ADR-015).
- [P0] NUNCA inferir `block` ou `bounded_context` do nome do diretório (ADR-012).
- [P0] NUNCA promover entrada do BOM a `certificada` sem `evidence_uri`, `evidence_digest`, `approved_by`, `certified_at` e `valid_until`, nem listar em `compatible_with` versão sem execução registrada (`BOM-03`, `BOM-04`, `BOM-07`).
- [P0] NUNCA admitir exceção sem os quatro itens de `GOV-30` nem exceção cujo objeto esteja no catálogo N1–N7 de `GOV-32`; a admissão é decidida por catálogo fechado, nunca por texto livre.
- [P0] NUNCA deixar o generator regravar `tools/dmpf-baseline/units-baseline.json` (`AUT-01`, ADR-031).
- [P0] NUNCA declarar exactly-once em nenhum artefato de `KRN-12`.
- [P1] Um transporte assíncrono por processo no `dmpf-reference` (Kafka); `TRP-09`/`TRP-46` são de task sucessora.
- [P1] Toda decisão transversal desta guarda-chuva é citada pela sub-spec que a aplica, nunca reescrita.
</critical_constraints>

## Escopo fora

- **Aplicação vertical de referência completa**: golden path, épico de ordem 4.
- **Binding persistido por mensagem (`TRP-09`/`TRP-46`)**: task sucessora no
  épico; addendum no ADR-039.
- **gRPC e SNS/SQS no composition root**: rito Buf e dois transportes por
  processo, respectivamente.
- **Consumo dos módulos fora do workspace com tag de módulo Go**: task
  sucessora; addendum no ADR-034.
- **Imagem Docker do `dmpf-reference`**: golden path.
- **Migração do `legado-golibs`, pilotos e métricas de adoção**: épico de ordem 5.
- **Kernel TypeScript e repositório de contratos separado**: ordens 2 e 3.
- **Alteração de FND-10**: a tensão entre `BOM-07` (transição só por ato) e
  `BOM-08` (vencida volta a `candidata`) é resolvida no validador da sub-spec 3
  sem editar a norma — vencida **reprova** até o commit que declara a
  transição.
