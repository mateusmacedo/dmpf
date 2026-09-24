---
id: SPEC-SJ66880S
slug: dmpf-testes-interop-go
title: DMPF KRN-11 — Testes, test kits e interoperabilidade Go ↔ TypeScript
stage: done
priority: P1
depends_on: [SPEC-WTAXFV8B, SPEC-XF9TF9A0, SPEC-ZHE7DN1H, SPEC-WYX5GW87, SPEC-3R80KNMS, SPEC-ANZX2WPG, SPEC-CGPX20NP, SPEC-NYD18TGD, SPEC-EAGAXQN1]
ticket_url: null
subtask_urls: []
created: 2026-09-06
---

# SPEC-SJ66880S: DMPF KRN-11 — Testes, test kits e interoperabilidade Go ↔ TypeScript

## Resumo

`KRN-02` a `KRN-10` entregaram o kernel Go com testes próprios em cada módulo,
mas ninguém entregou o **instrumento** que FND-09 especificou: os cinco test
kits com critério de aprovação decidível, o carregador de golden fixture com os
três oráculos reportados em separado, e a regra de dependência como *fitness
function* na suíte. Hoje cada módulo tem o seu `fixedClock{}`, o seu
`capturingPublisher` e o seu harness; o oráculo 3 do round-trip é um `t.Log`
que não reprova; e a matriz das 36 células é provada no oráculo interno do
`decide`, não por par de vetores executável no verificador. Esta spec entrega
o módulo `testkit` — um package por camada da pirâmide, cada um
classificado no bloco que a matriz permite —, o pipeline bidirecional das
fixtures com `DMPF-R001`..`R004`, a fitness function que reusa
`DMPF-D001`/`D002`/`E001`..`E004`, e o CI em estágios com a camada
distribuída em pipeline separado.

Como mantenedor de um bounded context sobre o kernel, quero certificar o meu
domínio, o meu service e o meu provider contra um contrato de aprovação que
alguém consegue reprovar, para que a conformidade com a RFC não dependa de
revisão manual nem de um teste que passa por acidente.

A entrega abre o incremento 5 — Entrega — e produz a evidência que `KRN-12`
exige para certificar uma entrada do BOM (SPEC-YRJRADY9, "Sequenciamento
sugerido").

## Contexto

- **Problema**: FND-09 §14 registra como pendência #1 "a implementação e a
  execução dos kits e das fixtures nas duas stacks", e §8.4 declara aberta a
  condição de fechamento até que os épicos de kernel a satisfaçam. Sem os
  kits, uma regra normativa só é "provada" pelo teste que cada módulo escreveu
  para si — e um teste sem contrato de aprovação não distingue conformidade de
  coincidência (`KIT-01`: critério não decidível é defeito). Sem a fitness
  function, a regra de dependência só é verificada pelo gate de CI, que roda
  depois do `go test` e fora dele (`FIT-04`). Sem o pipeline em estágios, a
  suíte de integração penaliza quem mexe só no domínio (`KIT-11`, rationale).
- **Impacto**: um módulo novo importa `testkit` no seu `_test.go`, pluga
  o seu domínio/service/provider no kit da camada, e recebe um `Verdict` com
  diagnóstico estável. O CI reprova cedo no domínio e tarde no broker, e a
  camada distribuída tem gate próprio. A evidência do round-trip sai em
  formato que o BOM consome.
- **Inspiração**: o próprio `conformance` já pratica o que o kit
  generaliza — oráculo transcrito à mão da norma e *red control* que corrompe
  a matriz para provar que o oráculo não é tautológico
  (`internal/rule/matrix_oracle_test.go`, `matrix_redcontrol_test.go`). O
  `contracts/golden` já carrega a fixture com todo escalar como string e
  rejeita versão de formato desconhecida (`fixture_test.go:22-73`,
  `golden_test.go:67`). O `app/example/reservations/relay_e2e_test.go`
  já força a janela at-least-once com um `crashingStore` deliberado. O kit
  reúne esses três gestos sob contrato observável e os torna reutilizáveis.
- **Links relevantes**:
  - `docs/specs/SPEC-YRJRADY9-dmpf-kernel-sdk-go.md` — spec guarda-chuva;
    linha `KRN-11` da decomposição (`:396`), requisito P1 (`:199-203`) e
    não-funcional de verificabilidade (`:207-210`).
  - `docs/specs/SPEC-6RQBN98G-dmpf-testes-interop.md` — spec que produziu
    FND-09, `done`.
  - `docs/dmpf/testes-interop.md` — FND-09, fonte normativa primária: §2
    (pirâmide), §5 (fixtures), §6 (oráculos de wire), §7 (projeção
    observável), §8 (kits, determinismo, pipeline), §9 (fitness function),
    §10 (par de vetores, `V27`), §13 (índice de regras), §14 (pendências).
  - `docs/specs/SPEC-WTAXFV8B-dmpf-verificador-conformidade-go.md` —
    `KRN-02`; diagnósticos e `decide` que a fitness function reusa.
  - `docs/specs/SPEC-WYX5GW87-dmpf-contratos-wire.md` — `KRN-05`; golden
    fixtures que esta spec promove a kit.
  - `docs/specs/SPEC-3R80KNMS-dmpf-outbox-postgres.md` — `KRN-06`; o outbox
    store que `KIT-04` modela de ponta a ponta.
  - `docs/specs/SPEC-ANZX2WPG-dmpf-inbox-consumo.md`,
    `docs/specs/SPEC-CGPX20NP-dmpf-relay-outbox.md` — `KRN-07`/`KRN-08`;
    inbox, relay e o `V32` in-process que `KIT-06` leva ao broker.
  - `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md` — `KRN-10`;
    Kafka provider que o harness distribuído usa, e os `hops_test.go`
    entregues como insumo a esta spec (`Escopo fora`, último item).
  - `docs/adr/031-verificador-de-conformidade-dmpf-em-go.md`,
    `docs/adr/033-adaptacoes-monorepo-dos-contratos-wire.md`,
    `docs/adr/030-granularidade-modulo-go-e-bom.md`.
  - RFC DMPF Foundation v0.1 (`docs/dmpf/rfc-dmpf-foundation-v0.1.md`):
    §4.5 regras 3 e 4, §7.4 (matriz), §9 (domínio em memória), §10.3
    (diagnósticos), §11 (`V13`..`V32`).

### Divergências entre o ticket e o repositório

O ticket `ARQ-530` foi escrito em 30/08/2026, antes de `KRN-03` a `KRN-10`
entrarem. Onze pontos precisam de reconciliação, e a spec prevalece sobre o
ticket em todos.

| Ticket `ARQ-530` diz | Repositório em 06/09/2026 | Esta spec adota |
| --- | --- | --- |
| "Criar `libs/shared/testkit` na convenção de `KRN-01`" | A convenção de `KRN-01` é `libs/<scope>/<stack>/<módulo>` com sufixo de stack no nome do projeto (`AGENTS.md`, "Caminho por scope e stack"); `scope:shared` na taxonomia 3D é para libs consumidas por backend **e** frontend, e o kit certifica só módulos Go de backend | `libs/backend/go/testkit`, projeto `testkit`, tags `type:lib`, `scope:backend`, `stack:go` |
| "Os cinco kits, um por camada (`KIT-01` a `KIT-06`)" | `KIT-01` é a regra-guarda (cada camada tem exatamente um kit, decidível); os kits são `KIT-02`..`KIT-06` (FND-09 §8.1) | Cinco packages, `KIT-02`..`KIT-06`; `KIT-01` é satisfeita pela forma de cada um |
| "`FIX-06`, `FIX-07` … `FIX-12`"; "`ORA-01` a `ORA-07`" | `FIX` vai até `FIX-13`; `ORA` tem duas faixas, `ORA-01`..`ORA-13` (wire, §6) e `ORA-30`..`ORA-40` (projeção observável, §7); `ORA-14`..`29` são reservados (§13.1) | As faixas reais; `FIX-13` (diagnóstico estável) e `ORA-30`..`ORA-40` entram no escopo |
| "Golden fixtures Go ↔ TypeScript … fonte única das duas stacks … pipeline bidirecional" | Não existe nenhum projeto TypeScript no workspace (`pnpm nx show projects` lista 13 projetos Go e o placeholder raiz); o próprio ticket põe a stack TS fora do escopo | O lado Go das **duas direções** — produtor (Go serializa a partir dos campos declarados) e consumidor (Go decodifica os bytes declarados) — com os três oráculos em separado e a evidência em JSON; a execução TS fica `encaminhada` ao épico de ordem 2, que consome a mesma fixture e o mesmo formato de evidência |
| "Reusando `DMPF-D001`, `DMPF-D002` e `DMPF-E001` a `DMPF-E004` em vez de cunhar código novo" | Os quinze códigos, `BuildUniverse`, `decide` e `Check` vivem sob `internal/` (`conformance/internal/rule/diagnostic.go:12-29`, `internal/conformance/check.go:60`); nenhum package exportado | O `conformance` ganha um package exportado de bloco `app`, `fitness`, que expõe o universo e a decisão de aresta sem o trust model; o kit o importa |
| Silente sobre a classificação do kit na matriz | Todo package não-teste é unidade do universo (`FIT-01`); um kit que importe domínio, ports, providers e contratos ao mesmo tempo só cabe no bloco `app` — e um `domainkit` de bloco `app` não provaria nada sobre o domínio | Um `dmpf-units.json` com **uma unidade por package**, cada package no bloco que a matriz permite: `domainkit` é `domain`, `golden` é `contract`, `serviceskit`/`providerkit`/`clock`/`ids`/`stable` são `provider`, `appkit`/`distkit`/`fitness`/`tb` são `app`. O precedente é `kernel/example-memory`, unidade `provider` dentro do módulo `application` |
| "As 36 células da matriz com par de vetores" | O oráculo das 36 células já é transcrito à mão e protegido por *red control* no `decide` (`matrix_oracle_test.go:16-53`, `matrix_redcontrol_test.go:38`); `vectors_test.go` exercita universos sintéticos para as células 1, 4, 5, 6 e 19; `tools/dmpf-cell-check.sh` prova 26 e 12 em worktree | O que falta é o **par executável no verificador por célula** — universo sintético com a aresta e sem ela — e a fitness function sobre o **universo de produção**. O oráculo do `decide` permanece onde está |
| "Harness distribuído com reentrega deliberada (`V32`, executada, nunca inspecionada)" | `V32` já é executado **in-process** sobre Postgres, com `capturingPublisher` e `crashingStore` (`app/example/reservations/relay_e2e_test.go:60,172`, `e2e_test.go:428`) | `KIT-06` exige dois ou mais **processos** sobre **broker real** (`PIR-14`); o harness re-executa o binário de teste como produtor e consumidor sobre o Redpanda que o CI já sobe, e o vetor negativo é um consumidor deliberadamente duplicador, reprovado com `DMPF-R004` |
| "Carregador de fixture com todo escalar lido como string" | O carregador existe, mas é privado ao `_test.go` do contracts (`fixture_test.go:22-73`); o oráculo 3 é `t.Log` informativo (`golden_test.go:124`); não há `DMPF-R00x`, nem diagnóstico por campo (`FIX-12`), nem reprovação por segunda fixture do mesmo contrato-major (`FIX-11`) | O carregador e os oráculos migram para `testkit/golden` (bloco `contract`); o `contracts/golden` passa a delegar e mantém o gerador e o `GOLDEN_UPDATE` (ele é o dono da fixture, `FIX-02`) |
| "Pipeline em estágios com gate por estágio" | `ci.yml` é um job único, `main`, que roda `nx affected` de tudo em sequência; `dmpf-verify.yml` é o precedente de gate em workflow separado | Os estágios de §8.3.1 dentro do `ci.yml`, ordenados e com gate declarado; a camada distribuída em `dmpf-distributed.yml`, workflow próprio com gate próprio |
| Não cita a fixture de projeção observável (§7) | `ORA-30`..`ORA-40` são regras próprias de FND-09 e definem o critério de aprovação de `KIT-02`; não há nada disso no repositório | Incluída: formato JSON de projeção em `contracts/fixtures/<ctx>/projection/v1/`, fonte única neutra de stack, consumida por `domainkit` |

### Fontes normativas

| Fonte | O que fixa para esta spec |
| --- | --- |
| FND-09 §2 (`PIR-01` a `PIR-18`) | Cinco camadas; camada decidida pelo escopo do teste, nunca pelo bloco do SUT; domínio sem duplo; aparato de determinismo em `services`/`provider`; fluxos distribuídos em pipeline separado; o que a pirâmide não decide |
| FND-09 §5 (`FIX-01` a `FIX-13`) | JSON com escalar como string; fonte única; bidirecional; seções obrigatórias; versão de formato com falha em desconhecida; uma fixture canônica por contrato-major; diagnóstico com fixture, direção, oráculo, campo, esperado, obtido e código estável |
| FND-09 §6 (`ORA-01` a `ORA-13`) | Passe = oráculo 1 ∧ oráculo 2 ∧ (oráculo 3 sob `ENV-24`); os três reportados em separado; casos discriminatórios obrigatórios; codificação de timestamps e decimais reservada a FND-05 |
| FND-09 §7 (`ORA-30` a `ORA-40`) | Projeção observável: ramo, resposta ou rejeição tipada, sequência ordenada de eventos; codificação abstrata; determinismo; acessor único; pós-condição de estado sob `Rejected`; par de vetores por ramo e negativo por proibição |
| FND-09 §8.1 (`KIT-01` a `KIT-06`) | Contrato observável de cada kit: o que exige, o que exercita, o que caracteriza aprovação; `KIT-04` modelado no outbox store e generalizado a inbox e UoW (§8.1.1) |
| FND-09 §8.2 (`KIT-07`, `KIT-08`) | Relógio fake, seed explícita, ordenação estável; o duplo vive em `services`/`provider`, nunca no domínio |
| FND-09 §8.3 (`KIT-09` a `KIT-11`) | Seis estágios ordenados com gate por estágio; contrato reusa os gates Buf; fluxos distribuídos em pipeline separado |
| FND-09 §8.4 | O kernel escolhe framework de teste e ferramenta de container, e produz a evidência |
| FND-09 §9 (`FIT-01` a `FIT-04`) | Fitness function sobre o universo de produção; reusa `DMPF-D001`/`D002`/`E001`..`E004` e os vetores `V13`..`V27`; não exerce o trust model; não fecha `ANC-10` |
| FND-09 §10 (`RAS-01` a `RAS-16`) | Par positivo/negativo por regra; negativo que passa é defeito; diagnóstico estável; família `DMPF-R001`..`R004`; `V31` é varredura, `V32` é reentrega executada; `V27` assimétrico, registrado com motivo |
| FND-09 §13.1 | Faixas de identificadores; `ORA-14`..`29` e `RAS-17`..`29` reservados |
| RFC §4.5 regras 3 e 4, `V29`/`V30` | O teste não reclassifica o SUT; código de produção rotulado como teste para escapar da regra reprova |
| RFC §7.4, `conformance/internal/rule/matrix.go` | `domain → domain` (1), `contract → contract` (36), `provider → {domain, port, provider, contract}` (25, 28, 29, 30) e `app → *` (13-18) permitidas; `domain → {application, app, port, provider, contract}` (2-6), `provider → {application, app}` (26, 27) e `contract → {domain, application, app, port, provider}` (31-35) proibidas |
| RFC §10.3, `internal/rule/diagnostic.go:12-29` | Conjunto fechado de quinze códigos; `DMPF-E004` não aplicável ao binding Go |
| `AGENTS.md` ("Libs", "Convenções obrigatórias") | Caminho por scope e stack; tags 3D; `package.json` com `private: true`; `dmpf-units.json`; registro em `go.work` e no baseline |

<constraints>
- [P0] NUNCA colocar duplo de teste, relógio, seed ou gerador de identidade em package de bloco `domain` do kit: `domainkit` recebe tudo por valor (`KIT-02`, `KIT-08`, `ORA-36`).
- [P0] NUNCA reduzir o round-trip a um único código de falha: cada oráculo reprova com o seu `DMPF-R00x`, e um passe no oráculo 2 não dispensa o 1 nem o 3 (`ORA-01`, `ORA-06`, `FIX-12`).
- [P0] NUNCA cunhar diagnóstico novo para defeito que a RFC §10.3 já cataloga: a fitness function emite `DMPF-D001`/`D002`/`E001`..`E004` do `conformance`, nunca uma cópia (`FIT-02`, `RAS-08`).
- [P0] NUNCA simular a reentrega de `V32`: o harness republica de fato sobre o broker; sem broker, o resultado é `t.Skip` nomeando a variável, nunca aprovado (`RAS-13`, `RAS-14`).
- [P0] NUNCA deixar o critério de aprovação de um kit sem decisão por valor: todo kit devolve `Verdict` com diagnóstico estável, e o `testing.TB` é adaptador, não contrato (`KIT-01`).
- [P1] Cada package do kit declara a sua unidade no bloco que a matriz permite; o verificador de conformidade aprova o módulo como qualquer outro (`RFC §4.5 r3`, `V29`).
- [P1] Nenhum cenário depende de relógio de parede, ordem de map ou porta aleatória (`KIT-07`).
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Módulo `testkit`**: criar `libs/backend/go/testkit` na
  convenção de `KRN-01`, com `go.mod`, `package.json` (`private: true`),
  `project.json` (tags 3D, targets `fmt-check`, `vet`, `test-race`,
  `govulncheck`, `lint`, `build`, `typecheck`), `doc.go`, `README.md`,
  `dmpf-units.json` com **uma unidade por package** e o `bounded_context`
  `kernel`; registrar em `go.work` e no baseline.
  - As unidades e os blocos: `kernel/testkit-domain` (`domainkit`,
    `domain`); `kernel/testkit-golden` (`golden`, `contract`);
    `kernel/testkit-services`, `-provider`, `-clock`, `-ids`, `-stable`
    (`provider`); `kernel/testkit-app`, `-dist`, `-fitness`, `-tb`
    (`app`).
  - Edge case: `conformance --root . --base develop` aprova o módulo;
    um package do kit que importe fora da sua linha da matriz reprova com
    `DMPF-D001` — e esse é o vetor `V29`/`V30` do próprio kit.
- [ ] **[P0] `domainkit` — kit de domínio em memória (`KIT-02`, §7)**:
  executar a UPR do candidato **por valor** e devolver a projeção observável.
  - `Projection`: ramo (`Accepted`/`Rejected`), resposta ou rejeição tipada
    com código `contexto/motivo`, sequência ordenada de eventos, e o estado
    observável do alvo antes e depois (`ORA-31`, `ORA-38`).
  - Codec da fixture de projeção: JSON, escalares como string (`FIX-07`),
    `format_version`, sem bytes de wire nem `payload_hash` (`ORA-30`,
    `ORA-33`); o codec opera sobre `[]byte` — nenhum I/O neste package.
  - Aprovação: fixadas as entradas, a projeção obtida é igual à esperada,
    campo a campo e evento a evento na ordem (`ORA-34`); uma segunda leitura
    do desfecho devolve a mesma projeção (`ORA-38`); sob `Rejected`, a
    sequência é vazia e o estado é idêntico ao anterior.
  - Fixtures de projeção para os dois agregados de exemplo (`orders`,
    `reservations`), com vetor positivo por ramo e negativo por proibição
    de FND-03 §3.4 (`ORA-39`), em
    `contracts/fixtures/<ctx>/projection/v1/<agregado>.golden`.
  - Edge case: fixture com `format_version` desconhecida faz o codec falhar
    (`FIX-09`); número JSON não-string reprova o carregamento (`FIX-07`).
- [ ] **[P0] `serviceskit` — kit de services com ports fakes (`KIT-03`)**:
  fakes que honram as portas de `ports` (`UnitOfWork`, `Repository`,
  `Outbox`, `Inbox`) com um **ledger** que registra a ordem dos gestos
  (begin, write, enqueue, commit, rollback, visibilidade).
  - Aprovação decidível a partir do ledger: escrita de estado e registro de
    outbox commitam juntos (`UOW-07`); nenhum gesto publica no broker
    (`UOW-08`); sob `Rejected`, nenhuma escrita e nenhum enfileiramento
    persistem (`UOW-06`); o commit é o único ponto de visibilidade.
  - Compõe sobre `application/example/memory` onde a realização em
    memória já existe; o ledger é a diferença.
  - Vetor negativo: um service de fixture que enfileira fora da transação
    reprova nomeando o gesto e a posição no ledger.
- [ ] **[P0] `providerkit` — kit de conformidade de provider (`KIT-04`,
  §8.1.1)**: suíte parametrizada pela porta, invocada com a implementação
  candidata e uma função de reset.
  - `OutboxStore`: claim sob lease, `MarkPublished` só pelo claimant
    corrente, claim substituído não muta o registro (`OBX-11` é sobre substituição, não expiração: um claim vencido sem sucessor ainda escreve)
    (`OBX-10`/`OBX-11`), purga e sinais.
  - `Inbox`: `insert-if-absent` serializando na chave (`INB-06`), com a
    corrida de duas inserções concorrentes resolvendo em exatamente um
    vencedor.
  - `UnitOfWork`: exatamente uma transação local por `Within`
    (`UOW-01`/`UOW-02`), rollback sem resíduo.
  - Roda contra `postgres` (build tag `integration`,
    `DMPF_PG_DSN`) e contra `application/example/memory` (sem infra);
    as duas aprovam. Vetor negativo: uma realização de fixture que permite
    `MarkPublished` com claim substituído reprova, nomeando a regra.
- [ ] **[P1] `appkit` — kit de app borda a borda (`KIT-05`)**: harness que
  compõe `app.Consumer` com as realizações concretas e aceita bytes na
  borda de protocolo; aprova quando a borda de efeito exibe o desfecho
  esperado (contagens de inbox, outbox e estado, ou nada sob `Rejected`).
  - Reusa o caso de `app/example/reservations/e2e_test.go` como
    primeira instância; sem segundo serviço (`PIR-12`).
- [ ] **[P1] `distkit` — harness distribuído com reentrega (`KIT-06`)**:
  dois processos OS — produtor e consumidor — sobre Redpanda
  (`DMPF_KAFKA_BROKERS`), com reentrega deliberada injetada pelo harness:
  republicação do mesmo `message_id` e publicação com `message_id` novo
  para o mesmo efeito.
  - Aprovação `V32`: o efeito final observável no consumidor é o mesmo com e
    sem reentrega (`RAS-13`).
  - Vetor negativo: um consumidor de fixture que aplica o efeito a cada
    entrega reprova com `DMPF-R004`, nomeando o `message_id` duplicado.
  - Sem `DMPF_KAFKA_BROKERS`: `t.Skip` nomeando a variável; nunca aprovado
    (`RAS-14`).
- [ ] **[P0] `clock`, `ids`, `stable` — determinismo (`KIT-07`, `KIT-08`)**:
  relógio fake realizando `ports.Clock` com avanço explícito; gerador
  de identificador sequencial e por seed realizando `ports.IDGenerator`;
  ordenação estável para toda coleção comparada. Bloco `provider`.
  - Os fakes ad hoc dos módulos existentes (`fixedClock{}`,
    `sequenceIDs{}`) **podem** migrar; a migração não é requisito desta spec.
- [ ] **[P0] `golden` — carregador e pipeline bidirecional (`FIX`, `ORA-01`
  a `ORA-13`)**: package de bloco `contract` que opera sobre `[]byte`.
  - Carregador: `format_version` desconhecida falha (`FIX-09`); qualquer
    escalar não-string falha (`FIX-07`); identidade é contrato qualificado +
    major (`FIX-11`); duas fixtures para o mesmo contrato-major reprovam por
    ambiguidade, nomeando os dois caminhos.
  - Direção **consumidor**: decodifica `payload_bytes_hex`, confere
    `payload_hash` (oráculo 2), decodifica o proto e compara campo a campo
    com os declarados (oráculo 1), envelopa e desenvelopa conferindo os
    atributos (oráculo 1).
  - Direção **produtor**: constrói a mensagem a partir dos campos declarados,
    serializa, e compara os bytes com `payload_bytes_hex` (oráculo 3) e o
    hash recomputado com `payload_hash` (oráculo 2).
  - Os três oráculos avaliados e reportados **em separado**, com
    `DMPF-R001` (semântica), `DMPF-R002` (hash), `DMPF-R003` (bytes); o
    diagnóstico traz fixture, direção, oráculo, campo, esperado, obtido e o
    código (`FIX-12`, `FIX-13`). O oráculo 3 **reprova** na direção
    produtor — é o caso de `ENV-24` — e não é mais informativo.
  - `Report` em JSON estável por fixture × direção × oráculo, que é a
    evidência que `KRN-12` consome.
  - `int64` além da precisão de double atravessa as duas direções sem perda.
- [ ] **[P0] Migração de `contracts/golden`**: os `_test.go` passam a
  usar `testkit/golden` para carregar e verificar; o gerador
  (`buildFixture`, `TestUpdateGolden`, `GOLDEN_UPDATE`) permanece no
  contracts, que segue dono das fixtures (`FIX-02`). Nenhuma fixture
  `.golden` muda de conteúdo.
- [ ] **[P0] Package exportado `fitness` no `conformance`**: bloco
  `app`, unidade `conformance/fitness`, que expõe o inventário do
  workspace, a construção do universo e a decisão de aresta e capability
  **sem** baseline e sem `--base` (`FIT-03`), devolvendo os mesmos
  `Diagnostic` e `Code` de `internal/rule`. Nada sai de `internal/`.
- [ ] **[P0] `fitness` no kit — a regra de dependência como fitness
  function (`FIT-01` a `FIT-04`)**:
  - `TestNenhumaArestaProibida`: constrói o universo de produção do
    workspace inteiro e assevera zero diagnósticos `DMPF-D001`, `DMPF-D002`,
    `DMPF-E001`..`E003` (`E004` não se aplica ao binding Go).
  - **36 células com par de vetores**: universos sintéticos em `testdata/`,
    um par por célula — para célula proibida, positivo sem a aresta e
    negativo com a aresta (`DMPF-D001`); para célula permitida, positivo
    intra-context e negativo inter-context sem superfície pública
    (`DMPF-D002`) — e a suíte reprova se algum negativo passar (`RAS-06`).
  - `V29`/`V30`: um package de teste de unidade `domain` cujo fechamento
    alcança `port`, `provider` ou capability não-`pure` reprova nomeando o
    import como violação — é o vetor "teste de domínio que precisa de
    duplo" do critério de aceite.
  - `V27` **registrado** como assimétrico: entrada na tabela de vetores do
    kit marcada `single-stack: typescript` com o motivo (Go não tem import
    apagado na emissão, RFC §11.4), e um teste que assevera que o registro
    existe e não é contado como lacuna (`RAS-15`, `RAS-16`).
- [ ] **[P2] `V31` como varredura (`RAS-12`)**: teste que varre
  `contracts/`, `libs/backend/go/**/README.md` e `libs/backend/go/**/dmpf-units.json`
  por promessa de exactly-once fim a fim, com allowlist das frases de
  vedação; reprova nomeando arquivo e linha. Não é execução.
- [ ] **[P0] `tb` — adaptador `testing.TB`**: converte `Verdict` em
  `t.Fatalf` com o diagnóstico estável na mensagem, lê fixtures do disco e
  as entrega como `[]byte` aos packages sem I/O, e resolve as variáveis de
  ambiente das dependências reais com `t.Skip` nomeado. Bloco `app`.
- [ ] **[P1] Pipeline em estágios (`KIT-09`, `KIT-10`, `KIT-11`)**:
  - `ci.yml` executa os estágios 1-5 de §8.3.1 **em ordem** — domínio,
    services, contrato, providers, apps — cada um com gate declarado que
    reprova por conta própria; o estágio de contrato é o conjunto de gates
    Buf já existente, reposicionado.
  - Cada projeto Go declara a sua camada por uma quarta tag Nx,
    `layer:{domain,services,contract,providers,apps,distributed}`, e cada
    estágio seleciona `--projects tag:layer:<camada>` sobre o `affected`.
  - `dmpf-distributed.yml`: workflow separado para `KIT-06`, com gate
    próprio; sua ausência não libera o merge das camadas de baixo e sua
    falha não é mascarada por elas.
  - Estágios 1 e 2 rodam **sem Docker**.
- [ ] **[P1] Registro e documentação**: `README.md` do kit com o contrato
  de cada kit (exige / exercita / aprova), a tabela regra → vetor
  (`RAS-01`) e as escolhas de §8.4 (framework `testing` da stdlib + `go
  test`; container por `docker run` no CI e `docker compose` local);
  `contracts/README.md` com a árvore `projection/`; `AGENTS.md` com o
  décimo quarto módulo e o comando do workflow distribuído; ADR desta
  realização.

### Não-funcionais

- [ ] **Determinismo**: nenhum teste do kit nem dos vetores depende de
  relógio de parede, ordem de map ou porta aleatória; `go test -race
  -count=3` passa (`KIT-07`).
- [ ] **Conformidade**: o módulo aprova em `conformance`; nenhum package
  de bloco `domain` ou `contract` do kit importa `testing`, `os`, `time` ou
  qualquer capability fora da allowlist do bloco.
- [ ] **Cadeia Go verde**: `fmt-check`, `vet`, `lint`, `test-race`,
  `govulncheck` nos quatorze módulos; `tools/dmpf-gate-check.sh` e
  `tools/dmpf-cell-check.sh` seguem passando.
- [ ] **Feedback rápido**: os estágios 1 e 2 do CI não sobem infraestrutura;
  a camada distribuída não roda no `ci.yml`.
- [ ] **Estabilidade do diagnóstico**: a mesma reprovação produz o mesmo
  código e a mesma mensagem entre execuções (`FIX-13`, `RAS-07`).
- [ ] **Sem dependência externa nova** além das já declaradas nos módulos
  que o kit consome; o kit não importa framework de asserção.

## Camadas afetadas

| Camada | Afetada | O que muda |
| --- | --- | --- |
| `domain` | [x] | `testkit/domainkit` (unidade `domain`); `domain` **não** muda — os seus exemplos ganham vetores via fixture de projeção |
| `application` | [ ] | Nada — `example/memory` é consumido por `serviceskit` e `providerkit` |
| `port` | [ ] | Nada — `Clock`, `IDGenerator`, `Outbox`, `Inbox`, `UnitOfWork`, `Repository` são realizados pelos fakes, não alterados |
| `contract` | [x] | `testkit/golden` (unidade `contract`); `contracts/golden/*_test.go` delega ao kit; `contracts/fixtures/<ctx>/projection/v1/` novas |
| `provider` | [x] | `testkit/{serviceskit,providerkit,clock,ids,stable}` (unidades `provider`); `postgres` recebe a suíte de conformidade em `_test.go` |
| `app` | [x] | `testkit/{appkit,distkit,fitness,tb}` (unidades `app`); `conformance/fitness` novo package exportado (unidade `app`) |
| Workspace | [x] | `go.work` (+1 `use`), `tools/dmpf-baseline/units-baseline.json` (+12 unidades), `.github/workflows/ci.yml` (estágios), `.github/workflows/dmpf-distributed.yml` (novo), tag `layer:*` nos 14 `project.json`, `AGENTS.md`, `contracts/README.md`, `docs/adr/040-*.md` |

## Localização de código

```text
libs/backend/go/testkit/                        — NOVO módulo; 11 unidades em 4 blocos
  domainkit/                 bloco domain — sem I/O, sem testing
    projection.go            — Projection, Branch, Event, Rejection; Equal (ordem de eventos comparada)
    fixture.go               — ProjectionFixture, Decode([]byte) (format_version, escalares como string)
    run.go                   — Run: executa a UPR por valor e devolve Projection; ReadTwice (ORA-38)
    verdict.go               — Verdict, Diagnostic (código, campo, esperado, obtido)
    *_test.go                — vetores por ramo e por proibição sobre os agregados de exemplo
  golden/                    bloco contract — sem I/O
    fixture.go               — Fixture, Case, Identity, Covers; Decode([]byte) (FIX-07, FIX-09)
    catalog.go               — Catalog: uma fixture canônica por contrato-major (FIX-11)
    roundtrip.go             — Consumer, Producer: as duas direções sobre proto.Message
    oracle.go                — Oracle1 (semântica), Oracle2 (hash), Oracle3 (bytes); códigos DMPF-R001..R003
    report.go                — Report em JSON estável: fixture × direção × oráculo
    *_test.go                — vetores positivo e negativo por oráculo, int64 além de double
  serviceskit/               bloco provider
    ledger.go                — Ledger: sequência de gestos com posição
    fakes.go                 — UnitOfWork, Repository, Outbox, Inbox sobre example/memory + ledger
    verdict.go               — Verdict UOW-06/07/08 a partir do ledger
    *_test.go                — service de fixture conforme e não conforme
  providerkit/               bloco provider
    outbox.go                — Suite para ports.Outbox / relay.Store: OBX-10, OBX-11, purga, sinais
    inbox.go                 — Suite para a porta de inbox: INB-06, corrida de duas inserções
    uow.go                   — Suite para ports.UnitOfWork: UOW-01, UOW-02, rollback
    suite.go                 — Candidate, Reset, Run → Verdict
    *_test.go                — contra example/memory; realização de fixture não conforme reprova
  clock/clock.go             bloco provider — Fake realizando ports.Clock; Advance
  ids/ids.go                 bloco provider — Sequence, Seeded realizando ports.IDGenerator
  stable/stable.go           bloco provider — Sort helpers para coleções comparadas
  appkit/                    bloco app
    harness.go               — Harness: compõe app.Consumer com realizações; Deliver([]byte) → Effects
    *_test.go                — //go:build integration; instância reservations
  distkit/                   bloco app
    harness.go               — Harness: re-exec do binário de teste como producer/consumer (os.Executable)
    producer.go, consumer.go — papéis sobre kafka; injeção de reentrega
    verdict.go               — DMPF-R004
    *_test.go                — //go:build integration; V32 positivo e consumidor duplicador
  fitness/                   bloco app
    universe.go              — Universe do workspace via conformance/fitness
    edges_test.go            — TestNenhumaArestaProibida (universo real)
    cells_test.go            — 36 células × par de vetores sobre testdata/cells
    domaintest_test.go       — V29/V30: package de teste de domínio com duplo
    v27_test.go              — registro da assimetria
    v31_test.go              — varredura de exactly-once (P2)
    vectors.go               — tabela regra → vetor → single-stack
    testdata/cells/<n>-<source>-<target>/{positive,negative}/ — universos sintéticos
  tb/                        bloco app
    tb.go                    — Require(t, Verdict); ReadFixture(t, path) []byte; Env(t, name) string
  doc.go, README.md, go.mod, go.sum, project.json, package.json, dmpf-units.json

libs/backend/go/conformance/
  fitness/                   — NOVO package exportado, bloco app, unidade conformance/fitness
    fitness.go               — Inventory(root), Universe(inv), Edges(u) []Diagnostic; reexporta Code, Diagnostic
    fitness_test.go
  dmpf-units.json            — MODIFICAR: +unidade fitness

libs/backend/go/contracts/golden/
  fixture_test.go            — MODIFICAR: remove o carregador; mantém specs, build, TestUpdateGolden
  golden_test.go             — MODIFICAR: delega a golden.Decode/Consumer/Producer; oráculo 3 reprova

libs/backend/go/postgres/
  conformance_test.go        — NOVO: //go:build integration; providerkit sobre OutboxStore, Inbox, UoW

contracts/fixtures/orders/projection/v1/order.golden           — NOVO
contracts/fixtures/reservations/projection/v1/reservation.golden — NOVO
contracts/README.md          — MODIFICAR: árvore projection/

.github/workflows/ci.yml     — MODIFICAR: estágios 1-5 de §8.3.1, seleção por tag layer:*
.github/workflows/dmpf-distributed.yml — NOVO: KIT-06 sobre Redpanda, gate próprio
libs/backend/go/*/project.json — MODIFICAR: +tag layer:* (14 projetos)
go.work                      — MODIFICAR: +1 use
tools/dmpf-baseline/units-baseline.json — REGRAVAR via --write-baseline
AGENTS.md                    — MODIFICAR: inventário (+1 módulo), comandos, tag layer
docs/adr/040-*.md            — NOVO: ADR desta realização
```

**Arquivos a modificar, e o que muda**:

- `libs/backend/go/conformance/fitness/` — a única superfície pública do
  verificador. Expõe o que `FIT-01` precisa (inventário, universo, decisão de
  aresta e capability) e **omite** o que `FIT-03` proíbe (baseline, `--base`,
  autorização de mudança de classificação). É bloco `app` porque importa
  `rule` (`domain`), `conformance` (`application`) e `fsstore`/`golist`
  (`provider`) — só a linha `app` alcança os três.
- `libs/backend/go/contracts/golden/*_test.go` — o carregador e os três
  oráculos saem daqui; o gerador fica. O contracts é dono da fixture, o kit
  é dono do oráculo — a divisão de FND-05 §8.4.
- `.github/workflows/ci.yml` — o job `main` passa a executar os estágios em
  steps ordenados com gate por estágio; a infraestrutura sobe **depois** dos
  estágios 1 e 2. O bloco `services:` continua inutilizável no `act_runner`
  (`ci.yml:52-55`), então a topologia permanece um job com steps.
- `libs/backend/go/*/project.json` — a tag `layer:*` é a quarta dimensão que
  o CI usa para selecionar o estágio; as três tags 3D não mudam.

## Design

### Arquitetura

```text
                        ┌──────────────────────────────────────────────┐
                        │  testkit (11 unidades, 4 blocos)         │
                        │                                              │
  domain  ──────────►   │  domainkit ──► domain            (1)    │
                        │                                              │
  contract ─────────►   │  golden ─────► contracts/{envelope,     │
                        │                payloadhash, gen}      (36)   │
                        │                                              │
  provider ─────────►   │  serviceskit ┐                               │
                        │  providerkit ├─► ports (28)             │
                        │  clock/ids   │   domain (25)            │
                        │  stable      ┘   application/example/   │
                        │                  memory (29)                 │
                        │                                              │
  app ──────────────►   │  appkit ──► app, providers, ports  (13-18)
                        │  distkit ─► kafka, app    │
                        │  fitness ─► conformance/fitness (15)    │
                        │  tb ──────► todos os kits + os/testing       │
                        └──────────────────────────────────────────────┘
                                          ▲
                                          │ importado só por *_test.go
                          ┌───────────────┴────────────────┐
                          │ domain, application, │
                          │ postgres, dmpf-  │
                          │ contracts, app, …         │
                          └────────────────────────────────┘
```

O número entre parênteses é a célula da matriz de RFC §7.4 que a aresta
ocupa; todas são permitidas. O kit é código de produção do ponto de vista do
verificador (`FIT-01`: só `_test.go` fica fora do universo), e por isso cada
package declara o bloco que as suas arestas permitem — é assim que `V29`
(teste não reclassifica o SUT) e `V30` (produção rotulada como teste) são
provados pelo próprio módulo.

### Fluxo 1 — round-trip bidirecional de uma golden fixture

1. `tb.ReadFixture` lê `contracts/fixtures/orders/event/v1/order-placed.golden`
   e entrega `[]byte` a `golden.Decode`.
2. `Decode` valida `format_version == "1"`, recusa qualquer escalar
   não-string, e devolve `Fixture{Identity, Covers, Cases, Discriminators}`.
3. `Catalog.Add` registra a identidade `orders/event/v1 + major`; uma segunda
   fixture com a mesma identidade reprova por ambiguidade.
4. Para cada caso, `Consumer.Run` decodifica `payload_bytes_hex`, recomputa o
   hash (oráculo 2), decodifica o proto e compara com os campos declarados
   (oráculo 1), envelopa e desenvelopa conferindo os quinze atributos
   (oráculo 1).
5. Para cada caso, `Producer.Run` constrói o proto a partir dos campos
   declarados, serializa e compara os bytes com `payload_bytes_hex`
   (oráculo 3) e o hash com `payload_hash` (oráculo 2).
6. `Report` acumula um `Outcome` por fixture × direção × oráculo, com o
   diagnóstico `FIX-12` em cada reprovação; `tb.Require` falha o teste
   listando todos os `DMPF-R00x`, nunca um só.

### Fluxo 2 — conformidade do outbox store Postgres (`KIT-04`)

1. `conformance_test.go` (tag `integration`) abre o pool por `DMPF_PG_DSN`
   via `tb.Env` — ausente, `t.Skip("DMPF_PG_DSN")`.
2. `providerkit.Run(Candidate{Outbox: store, Reset: truncate})` executa a
   suíte: enfileira, reivindica sob lease, avança o relógio fake além do
   lease, tenta `MarkPublished` com o claim substituído (deve ser rejeitado),
   reivindica de novo, marca com o claim corrente (deve transitar).
3. A mesma suíte roda contra `example/memory` sem tag; as duas produzem
   `Verdict{OK: true}`.
4. Uma realização de fixture no próprio kit, que aceita `MarkPublished` com
   claim substituído, produz `Verdict` reprovado nomeando `OBX-11`.

### Fluxo 3 — reentrega deliberada sobre o broker (`KIT-06`, `V32`)

1. `distkit_test.go` (tag `integration`) resolve `DMPF_KAFKA_BROKERS`;
   ausente, `t.Skip`.
2. `Harness.Start` re-executa o próprio binário de teste duas vezes com
   `DMPF_TESTKIT_ROLE=consumer` e `=producer`, tópico e grupo únicos por
   execução, sobre Postgres compartilhado com `TRUNCATE` no início.
3. O produtor publica `evt-1`; o harness aguarda o efeito; republica `evt-1`
   (mesmo `message_id`) e publica `evt-9` (`message_id` novo, mesmo pedido).
4. O harness lê a borda de efeito: uma reserva, duas linhas de inbox,
   `Verdict{OK: true}`.
5. Com `DMPF_TESTKIT_ROLE=consumer-naive` (consumidor de fixture que aplica o
   efeito a cada entrega), a leitura mostra duas reservas e o `Verdict` traz
   `DMPF-R004` com o `message_id` duplicado.

### Fluxo 4 — a fitness function e as 36 células

1. `TestNenhumaArestaProibida` chama `fitness.Inventory(root)` →
   `fitness.Universe` → `fitness.Edges` sobre o workspace real, e assevera
   `len(diagnostics) == 0`.
2. `cells_test.go` itera `testdata/cells/<n>-<source>-<target>/`: para cada
   célula, constrói o universo de `positive/` e assevera zero diagnósticos;
   constrói o de `negative/` e assevera exatamente um `DMPF-D001` (célula
   proibida) ou `DMPF-D002` (célula permitida, inter-context) apontando a
   aresta.
3. Um metateste confere que existem exatamente 36 diretórios, cada um com os
   dois lados, e que a soma bate com o oráculo de `matrix_oracle_test.go`
   (17 permitidas, 19 proibidas) — sem derivá-lo de `matrix.go`.

### Onde cada regra é provada

| Regra | Instrumento | Onde |
| --- | --- | --- |
| `KIT-02`, `ORA-30`..`ORA-39` | `domainkit.Run` + fixture de projeção | `domainkit/*_test.go`, `contracts/fixtures/*/projection/` |
| `KIT-03`, `UOW-06`..`UOW-08` | ledger de `serviceskit` | `serviceskit/*_test.go` |
| `KIT-04`, `OBX-10`, `OBX-11`, `INB-06`, `UOW-01`, `UOW-02` | `providerkit` contra Postgres e memória | `providerkit/*_test.go`, `postgres/conformance_test.go` |
| `KIT-05`, `PIR-12`, `PIR-13` | `appkit.Harness` | `appkit/*_test.go` |
| `KIT-06`, `PIR-14`, `PIR-15`, `V32`, `DMPF-R004` | `distkit.Harness` sobre Redpanda | `distkit/*_test.go`, `dmpf-distributed.yml` |
| `KIT-07`, `KIT-08` | `clock`, `ids`, `stable`; `-count=3` | todos os `_test.go` do kit |
| `KIT-09`..`KIT-11` | estágios do `ci.yml` e `dmpf-distributed.yml` | `.github/workflows/` |
| `FIX-07`, `FIX-09`, `FIX-11` | `golden.Decode`, `golden.Catalog` | `golden/fixture_test.go`, `catalog_test.go` |
| `FIX-12`, `FIX-13`, `ORA-01`..`ORA-06`, `DMPF-R001`..`R003` | `golden.Oracle*`, `golden.Report` | `golden/oracle_test.go`, `contracts/golden/golden_test.go` |
| `ORA-08`, `ORA-12`, `INT-03` | casos discriminatórios existentes | fixtures `.golden` de `KRN-05`, inalteradas |
| `FIT-01`..`FIT-04` | `fitness` sobre o universo real | `fitness/edges_test.go` |
| 36 células, `V13`..`V26`, `V28` | universos sintéticos | `fitness/cells_test.go`, `testdata/cells/` |
| `V29`, `V30`, `PIR-17` | package de teste de domínio com duplo | `fitness/domaintest_test.go` |
| `V27`, `RAS-15`, `RAS-16` | registro na tabela de vetores | `fitness/v27_test.go`, `README.md` |
| `V31`, `RAS-12` | varredura | `fitness/v31_test.go` (P2) |
| `RAS-01` | tabela regra → vetor | `fitness/vectors.go`, `README.md` |

## Decisões técnicas

- **Um módulo, onze unidades, quatro blocos**: `dmpf-units.json` declara uma
  unidade por package, no bloco que as arestas daquele package permitem,
  porque o kit é código de produção para o verificador (`FIT-01`) e um kit
  único de bloco `app` não provaria nada sobre a camada que certifica — um
  `domainkit` só é kit de domínio se ele próprio for `domain` (célula 1).
  Alternativa descartada: módulo por camada (`testkit-domain`, …),
  porque o custo de cinco `go.mod` não compra nada que a unidade por package
  não compre, e o precedente `kernel/example-memory` (unidade
  `provider` dentro de `application`) já mostra a forma.
- **Kits decidíveis por valor; `testing.TB` é adaptador**: cada kit devolve
  `Verdict{OK, Diagnostics}` com código estável, e `tb.Require` converte em
  `t.Fatalf`. O motivo é dobrado: `KIT-01` exige critério decidível, e os
  blocos `domain` e `contract` não podem importar `testing` (capability fora
  da allowlist). Alternativa descartada: kits recebendo `*testing.T`, porque
  amarraria o contrato ao framework e violaria a capability do bloco.
- **Package exportado `fitness` no `conformance`, sem mover `internal/`**:
  `FIT-02` exige reusar os diagnósticos, e hoje eles são inalcançáveis. O
  package expõe inventário, universo e decisão de aresta/capability, e omite
  baseline e `--base` — exatamente a fronteira de `FIT-03` (o teste de
  arquitetura não exerce o trust model). Alternativa descartada: invocar o
  binário por subprocesso e parsear a saída, porque não reusa `decide` nem
  os tipos, e porque o binário exige `--base` e emite `NaoVerificado` sem
  baseline — o gate e a fitness function têm contratos diferentes.
- **Bidirecionalidade sem stack TypeScript**: as duas direções de `FIX-03`
  são realizadas no lado Go — produtor (campos → bytes, oráculos 2 e 3) e
  consumidor (bytes → campos, oráculos 1 e 2) — e cada uma reprova por
  conta própria. A execução TypeScript é `encaminhada` ao épico de ordem 2,
  que consome a mesma fixture (`FIX-02`) e emite o mesmo `Report`. Um
  round-trip Go → Go não é o cross-stack de FND-09 §8.4, e a spec não o
  chama assim: a evidência registrada é a das duas direções Go. Alternativa
  descartada: fundar o primeiro projeto TypeScript do workspace aqui, porque
  o ticket a exclui e porque um consumidor TS de fixture sem kernel TS
  seria código sem dono.
- **Oráculo 3 reprova na direção produtor**: `ORA-04` exige o oráculo 3 onde
  `ENV-24` se aplica — publicação sem reserialização —, e a direção produtor
  é exatamente esse caso: os bytes que o Go produz a partir dos campos
  declarados devem ser os bytes da fixture. O `t.Log` de hoje
  (`golden_test.go:124`) vira `DMPF-R003`. Na direção consumidor, o oráculo
  3 é a identidade de `Any.value` após envelopar e desenvelopar, que já
  reprova.
- **Processos OS reais no harness distribuído**: `PIR-14` exige "dois ou
  mais processos"; o harness re-executa `os.Executable()` com uma variável
  de papel — o idioma de *helper process* da stdlib (`exec_test.go` do Go).
  Alternativa descartada: duas goroutines com clientes Kafka distintos,
  porque compartilham memória e não provam o que `V32` prova.
- **Redpanda como broker do `KIT-06`**: o CI já o sobe para `KRN-10`, e
  Kafka é o transporte-alvo do evento de domínio (ADR-025). Alternativa
  descartada: floci/SQS, porque a reentrega em SQS depende do timeout de
  visibilidade e tornaria o harness lento e dependente de relógio.
- **Estágios como steps ordenados de um job, não jobs paralelos**: o
  `act_runner` do Gitea não resolve o bloco `services:` (`ci.yml:52-55`), e
  jobs separados subiriam a infraestrutura por job. Steps ordenados com
  gate por step realizam `KIT-09` (ordem, gate declarado por estágio) sem
  multiplicar a infra; os estágios 1 e 2 rodam antes de qualquer `docker
  run`. `KIT-11` é satisfeita por workflow separado, não por job separado.
- **Tag Nx `layer:*` como quarta dimensão**: o estágio precisa selecionar
  projetos por camada, e as tags 3D não a expressam. A tag é aditiva, não
  altera `release.projects` (`tag:type:lib`) nem os `targetDefaults`.
  Alternativa descartada: um target por camada (`test-domain`,
  `test-providers`), porque duplicaria `test-race` em todos os módulos.
- **Fixture de projeção em `contracts/fixtures/<ctx>/projection/v1/`**:
  `contracts/` é a raiz neutra de stack do workspace (`contracts/README.md`)
  e a projeção é a fonte única que o kernel TS consumirá (`ORA-35`); a
  extensão `.golden` e o `format_version` seguem `FIX-09`/`FIX-10` por
  analogia. Alternativa descartada: `testdata/` dentro de `domain`,
  porque obrigaria a cópia na outra stack — o mesmo defeito que `FIX-02`
  proíbe para as fixtures de wire.
- **O contracts segue dono das fixtures de wire; o kit é dono do oráculo**:
  o gerador (`buildFixture`, `GOLDEN_UPDATE`) fica em `contracts/golden`
  porque quem gera a fixture é quem tem o proto (`FIX-02`); o carregador e
  os oráculos vão para o kit porque são instrumento, não contrato
  (FND-05 §8.4). Não há ciclo quando o `_test.go` externo do contracts
  importa o kit, e o kit importa `contracts/envelope` — o package
  `golden` do contracts não é importado por ninguém.
- **`V31` como P2**: é varredura (`RAS-12`), não execução; o valor é baixo
  enquanto o acervo é escrito pela mesma equipe. Entra se couber no
  incremento; sai sem afetar o critério de aceite do ticket.
- **Framework `testing` da stdlib e `go test`; container por `docker run` no
  CI e `docker compose` local**: as escolhas que §8.4 deixa ao kernel,
  registradas aqui e no README do kit. São as já praticadas pelos treze
  módulos; introduzir testify ou testcontainers-go criaria dependência
  externa nova sem ganho de decidibilidade.
- **O `depguard` não alcança `testkit/domainkit`**: a regra `domain` do
  `.golangci.yml` seleciona por path `**/*-domain/**`, que o package do kit
  não casa. O gate autoritativo entre módulos é o verificador, que
  classifica por `dmpf-units.json` e alcança o package. Registrado como
  limitação conhecida; estender o glob do `depguard` é opcional e fica fora.

## Regras relacionadas

- `AGENTS.md` — convenção de módulo Go (caminho, tags, `package.json`,
  `dmpf-units.json`, `go.work`, baseline), cadeia de validação, CI no
  `act_runner`.
- `.claude/rules/process-enforcement.md` — cadeia Go: `gofmt`, `go vet`,
  `golangci-lint`, `go build`, `go test`, `go test -race`, `govulncheck`.
- `.claude/rules/git-safety.md` — branches protegidas; nenhum `--no-verify`.
- `.claude/skills/skill-go-testing/` — table-driven tests, `t.Helper`,
  `t.Cleanup`, build tags, race detector.
- `docs/guides/dmpf-manifesto.md` — forma do `dmpf-units.json` e do
  baseline.
- `SPEC-YRJRADY9` — guarda-chuva; esta spec é a décima sub-spec.
- `SPEC-6RQBN98G` — produziu FND-09; esta spec o realiza na stack Go.
- `SPEC-WTAXFV8B` — verificador cujos diagnósticos a fitness function reusa.
- `SPEC-WYX5GW87` — fixtures de wire que esta spec promove a kit.
- `SPEC-3R80KNMS`, `SPEC-ANZX2WPG`, `SPEC-CGPX20NP` — outbox, inbox e relay
  que `KIT-04` e `KIT-06` certificam.
- `SPEC-EAGAXQN1` — Kafka provider do harness distribuído.

## Verificação e testes

### Critérios de aceite

Os sete primeiros são os do ticket `ARQ-530`, verbatim; os seguintes derivam
das fontes normativas e das divergências reconciliadas acima.

- [x] Cada uma das **36 células** da matriz tem vetor positivo **e** negativo
  executável, e a suíte reprova se algum negativo passar.
- [x] Um teste de domínio que precise de duplo de infraestrutura reprova,
  nomeando a dependência como violação.
- [x] O round-trip roda nas **duas direções**, e o passe numa não dispensa a
  outra.
- [x] Os três oráculos são reportados em separado, com o campo divergente, a
  direção e o valor esperado; código único de "round-trip falhou" reprova o
  próprio kit.
- [x] Fixture com versão de formato desconhecida faz o carregador **falhar**;
  uma segunda fixture para o mesmo contrato-major reprova por ambiguidade, e
  um inteiro de 64 bits atravessa o round-trip sem perda de precisão.
- [x] O harness injeta reentrega deliberada e aprova quando o efeito final é
  o mesmo; consumidor que duplique o efeito reprova.
- [x] Nenhum cenário depende de relógio de parede, ordem de map ou porta
  aleatória, e `V27` está registrado como single-stack.
- [x] `libs/backend/go/testkit` existe com onze unidades em
  `dmpf-units.json`, registradas no `go.work` e no baseline;
  `conformance --root . --base develop` aprova.
- [x] Nenhum package de bloco `domain` ou `contract` do kit importa
  `testing`, `os` ou `time`; um teste de compilação no próprio kit assevera
  isso pelo `go list -deps`.
- [x] `domainkit.Run` sobre `orders` e `reservations` aprova com as fixtures
  de projeção; um `Rejected` devolve sequência vazia e estado idêntico; a
  segunda leitura devolve a mesma projeção.
- [x] `serviceskit` aprova o caso de uso de referência de `orders`; um
  service de fixture que enfileira fora da transação reprova nomeando
  `UOW-08` e a posição no ledger.
- [x] `providerkit` aprova `example/memory` sem infra e
  `postgres` com `DMPF_PG_DSN`; a realização de fixture com
  claim substituído aceito reprova nomeando `OBX-11`; sem a variável, `t.Skip`
  nomeando-a.
- [x] `distkit` roda dois processos sobre `DMPF_KAFKA_BROKERS`; `V32`
  positivo aprova; `consumer-naive` reprova com `DMPF-R004`; sem a variável,
  `t.Skip` nomeando-a.
- [x] `golden.Decode` rejeita `format_version` `"2"`, rejeita um escalar
  numérico, e `Catalog.Add` rejeita a segunda fixture de
  `orders/event/v1`; os três diagnósticos nomeiam o caminho.
- [x] Na direção produtor, uma fixture cujos bytes divergem dos campos
  declarados reprova **só** no oráculo 3 (`DMPF-R003`) quando o hash foi
  gravado sobre os bytes divergentes, e nos oráculos 2 e 3 quando não foi;
  o `Report` lista os dois em separado.
- [x] `contracts/golden` passa com o kit e as três fixtures `.golden`
  são byte a byte as de antes; `GOLDEN_UPDATE=1` continua regravando.
- [ ] `fitness.Edges` sobre o workspace real devolve zero diagnósticos;
  `testdata/cells/` tem 36 diretórios com `positive/` e `negative/`, e o
  metateste bate 17 permitidas e 19 proibidas com o oráculo de
  `matrix_oracle_test.go` sem derivá-lo de `matrix.go`.
- [x] `fitness/domaintest_test.go` reprova um package de teste de unidade
  `domain` cujo fechamento alcança `ports`, nomeando o import.
- [x] `fitness/v27_test.go` assevera a entrada `V27` com
  `single-stack: typescript` e motivo; a tabela de vetores não a conta
  como lacuna.
- [x] `ci.yml` executa domínio → services → contrato → providers → apps,
  cada estágio com gate; os dois primeiros rodam antes de qualquer `docker
  run`; `dmpf-distributed.yml` existe, roda `distkit` e é gate próprio.
- [ ] Os quatorze `project.json` têm a tag `layer:*`; `pnpm nx show projects
  --projects tag:layer:domain` lista ao menos `domain` e
  `testkit`.
- [x] Cadeia Go verde nos quatorze módulos; `go test -race -count=3
  ./...` do kit passa; `tools/dmpf-gate-check.sh` e `tools/dmpf-cell-check.sh`
  passam.
- [x] `README.md` do kit traz o contrato dos cinco kits, a tabela regra →
  vetor, as escolhas de §8.4 e a assimetria de `V27`; `contracts/README.md`
  descreve `projection/`; `AGENTS.md` inventaria os quatorze módulos;
  ADR-040 registrado e indexado.

### Cenários de teste

**Golden — carregador e catálogo (`golden/fixture_test.go`, `catalog_test.go`)**

- Dado `order-placed.golden` com `format_version: "1"`, quando `Decode`
  roda, então devolve `Fixture` com `Identity{orders/event/v1,
  OrderPlaced}`, `Covers{profile_major: "1", contract_major: "v1"}` e a
  contagem de casos da fixture.
- Dado o mesmo arquivo com `format_version: "2"`, quando `Decode` roda,
  então falha nomeando o arquivo e a versão, sem tentar interpretar os
  casos.
- Dado um caso com `"quantity": 2` (número JSON), quando `Decode` roda,
  então falha nomeando o campo e o caso — `FIX-07`.
- Dado um `Catalog` com `order-placed.golden`, quando `Add` recebe um segundo
  arquivo com a mesma identidade e major, então reprova por ambiguidade
  nomeando os dois caminhos.

**Golden — os três oráculos em separado (`golden/oracle_test.go`)**

- Dado um caso conforme, quando `Consumer.Run` e `Producer.Run` rodam, então
  o `Report` traz seis `Outcome{OK: true}` (três oráculos × duas direções).
- Dado um caso cujo `payload_bytes_hex` foi alterado num campo e cujo
  `payload_hash` foi recalculado sobre os bytes alterados, quando
  `Consumer.Run` roda, então o oráculo 2 passa e o oráculo 1 reprova com
  `DMPF-R001`, campo, esperado e obtido; quando `Producer.Run` roda, então
  o oráculo 3 reprova com `DMPF-R003` e o oráculo 2 reprova com
  `DMPF-R002` — dois `Outcome` distintos.
- Dado um caso com `int64` `9007199254740993` (além de 2⁵³), quando as duas
  direções rodam, então o valor é preservado exatamente e o `Report` não
  traz reprovação.
- Dado um `Report` com uma reprovação em cada oráculo, quando serializado,
  então o JSON é idêntico entre duas execuções (`FIX-13`).

**Domainkit — projeção observável (`domainkit/run_test.go`)**

- Dado a fixture `order.golden` com o caso `place-accepted`, quando `Run`
  executa a UPR de `orders` com os valores da fixture, então a projeção é
  `Accepted`, a resposta bate campo a campo e a sequência tem exatamente os
  eventos declarados, na ordem.
- Dado o caso `place-rejected-empty`, quando `Run` executa, então o ramo é
  `Rejected` com código `orders/empty-order`, a sequência é vazia e o estado
  do alvo é idêntico ao anterior (`ORA-38`).
- Dado um domínio de fixture que expõe um segundo acessor de eventos, quando
  `Run` executa, então o `Verdict` reprova nomeando `ORA-37`.
- Dado qualquer projeção, quando `ReadTwice` roda, então as duas leituras
  são iguais.

**Serviceskit — a UoW pelo ledger (`serviceskit/verdict_test.go`)**

- Dado o caso de uso de `orders` sobre os fakes, quando `Accepted`, então o
  ledger mostra `begin, write, enqueue, commit` com `write` e `enqueue`
  entre o mesmo `begin` e `commit`, e nenhum gesto `publish` (`UOW-07`,
  `UOW-08`).
- Dado o mesmo caso com entrada que produz `Rejected`, quando executado,
  então o ledger mostra `begin, rollback` e os fakes não têm estado nem
  registro de outbox (`UOW-06`).
- Dado um service de fixture que chama `Outbox.Enqueue` antes de `begin`,
  quando executado, então o `Verdict` reprova nomeando `UOW-08` e a posição
  `0` do ledger.

**Providerkit — outbox store (`providerkit/outbox_test.go`,
`postgres/conformance_test.go`)**

- Dado um registro enfileirado e reivindicado com lease de 5s no relógio
  fake, quando o relógio avança 6s e o claimant original chama
  `MarkPublished`, então o store recusa e o registro permanece reivindicável
  (`OBX-11`).
- Dado o mesmo registro reivindicado por um segundo claimant após o
  vencimento, quando o segundo chama `MarkPublished`, então transita
  (`OBX-10`).
- Dado `Candidate{Outbox: memory}` e `Candidate{Outbox: postgres}`, quando a
  suíte roda, então ambas devolvem `Verdict{OK: true}` com o mesmo número de
  verificações.
- Dado `Candidate{Outbox: fixtureLenient}` (aceita claim substituído), quando a
  suíte roda, então o `Verdict` reprova nomeando `OBX-11` e a verificação.
- Dado `DMPF_PG_DSN` ausente, quando `conformance_test.go` roda, então
  `t.Skip` com a mensagem contendo `DMPF_PG_DSN`.

**Distkit — reentrega sobre o broker (`distkit/harness_test.go`)**

- Dado produtor e consumidor em processos separados sobre Redpanda, quando o
  produtor publica `evt-1`, republica `evt-1` e publica `evt-9` para o mesmo
  pedido, então a borda de efeito mostra uma reserva e duas linhas de inbox
  (`evt-1` R2 na segunda, `evt-9` R1×D2), e `Verdict{OK: true}`.
- Dado o consumidor `consumer-naive`, quando a mesma sequência roda, então a
  borda mostra duas reservas e o `Verdict` traz `DMPF-R004` com `evt-1`.
- Dado `DMPF_KAFKA_BROKERS` ausente, quando o teste roda, então `t.Skip`
  nomeando a variável, e o `Report` não registra o vetor como aprovado.

**Fitness — universo real e 36 células (`fitness/*_test.go`)**

- Dado o workspace atual, quando `TestNenhumaArestaProibida` roda, então
  zero diagnósticos.
- Dado `testdata/cells/05-domain-provider/negative/`, quando o universo é
  construído, então exatamente um `DMPF-D001` com origem na unidade
  `domain` e destino na `provider`; em `positive/`, zero.
- Dado `testdata/cells/01-domain-domain/negative/` (dois contexts distintos,
  sem superfície pública), quando construído, então exatamente um
  `DMPF-D002`; em `positive/` (mesmo context), zero.
- Dado um diretório de célula com `negative/` que o verificador aprove,
  quando o metateste roda, então reprova nomeando a célula (`RAS-06`).
- Dado um package de teste de unidade `domain` que importa `ports`,
  quando `domaintest_test.go` roda, então reprova nomeando
  `github.com/.../ports` como a dependência (`V29`/`V30`).
- Dado a tabela de vetores, quando `v27_test.go` roda, então a entrada
  `V27` tem `SingleStack == "typescript"` e motivo não vazio, e a contagem
  de lacunas Go é zero.

**CI — estágios (`ci.yml`, `dmpf-distributed.yml`)**

- Dado um PR que toca só `domain`, quando o `ci.yml` roda, então o
  estágio 1 executa antes de qualquer `docker run`, e os estágios 4 e 5 não
  selecionam projeto algum.
- Dado uma reprovação no estágio 2, quando o `ci.yml` roda, então os
  estágios 3 a 5 não executam e o job reprova nomeando o estágio.
- Dado um PR qualquer, quando `dmpf-distributed.yml` roda, então ele sobe
  Postgres e Redpanda, executa `testkit:test-race` com a tag
  `integration` restrito a `distkit`, e reprova por conta própria.

<critical_constraints>
- [P0] NUNCA colocar duplo de teste, relógio, seed ou gerador de identidade em package de bloco `domain` do kit: `domainkit` recebe tudo por valor (`KIT-02`, `KIT-08`, `ORA-36`).
- [P0] NUNCA reduzir o round-trip a um único código de falha: cada oráculo reprova com o seu `DMPF-R00x`, e um passe no oráculo 2 não dispensa o 1 nem o 3 (`ORA-01`, `ORA-06`, `FIX-12`).
- [P0] NUNCA cunhar diagnóstico novo para defeito que a RFC §10.3 já cataloga: a fitness function emite `DMPF-D001`/`D002`/`E001`..`E004` do `conformance`, nunca uma cópia (`FIT-02`, `RAS-08`).
- [P0] NUNCA simular a reentrega de `V32`: o harness republica de fato sobre o broker; sem broker, o resultado é `t.Skip` nomeando a variável, nunca aprovado (`RAS-13`, `RAS-14`).
- [P0] NUNCA deixar o critério de aprovação de um kit sem decisão por valor: todo kit devolve `Verdict` com diagnóstico estável, e o `testing.TB` é adaptador, não contrato (`KIT-01`).
- [P1] Cada package do kit declara a sua unidade no bloco que a matriz permite; o verificador de conformidade aprova o módulo como qualquer outro (`RFC §4.5 r3`, `V29`).
- [P1] Nenhum cenário depende de relógio de parede, ordem de map ou porta aleatória (`KIT-07`).
</critical_constraints>

## Escopo fora

- **A stack TypeScript**: kernel, kits e consumidor TS das fixtures são do
  épico de ordem 2. Esta spec entrega o lado Go das duas direções e o
  formato de `Report` que a outra stack emitirá; a execução cross-stack de
  FND-09 §8.4 permanece **aberta** até lá, e a spec não a declara fechada.
- **Conteúdo obrigatório da fixture e definição dos oráculos**: FND-05
  §8.2/§8.3; as fixtures `.golden` de `KRN-05` não mudam.
- **Codificação de wire de timestamps e decimais**: reservada a FND-05 sob
  `ANC-03` (`ORA-11`); a fixture mantém o slot reservado.
- **Backpressure, timing de retry, DLQ, quarantine e replay operacional**:
  FND-08, realizados por `KRN-09`; `KIT-06` nomeia a fronteira, não a
  decide (`PIR-16`).
- **O fechamento de `ANC-10`**: a fitness function não é o linter de
  produção de RFC §10 nem exerce o trust model (`FIT-03`, `FIT-04`); a
  condição de fechamento continua sendo o verificador conforme nos 32
  vetores de RFC §11, que `KRN-02` já cumpre no gate.
- **Localização do validador executável do perfil de envelope** (`RAS-41`,
  §14 item 2): co-decisão FND-06 × FND-09, fora desta spec.
- **Migração dos fakes ad hoc dos treze módulos** (`fixedClock{}`,
  `sequenceIDs{}`, `capturingPublisher`) para `clock`/`ids`/`serviceskit`:
  opcional; entra por Regra do Escoteiro se couber, não é critério de
  aceite.
- **Extensão do glob do `depguard`** para alcançar `testkit/domainkit`:
  registrado como limitação; o verificador cobre.
- **Evidência para o BOM e a entrada certificada**: `KRN-12` consome o
  `Report`; o `evidence_uri`/`evidence_digest` são dele.
- **Jobs paralelos no CI**: a topologia permanece um job com steps
  ordenados enquanto o `act_runner` não resolver `services:`.
- **Kits para as portas gRPC/HTTP/SQS**: `providerkit` cobre outbox, inbox
  e UoW (§8.1.1); os `hops_test.go` de `KRN-10` seguem como testes do
  próprio provider, e a generalização do kit às portas de transporte é
  trabalho posterior, sem regra de FND-09 que o exija agora.
