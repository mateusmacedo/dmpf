# dmpf-testkit-go

Instrumento de teste do kernel DMPF fixado por FND-09
(`docs/dmpf/testes-interop.md`): um kit por camada da pirâmide, o carregador
de golden fixture com os três oráculos reportados em separado, a regra de
dependência como *fitness function* na suíte, as primitivas de determinismo e o
adaptador de `testing.TB` que transforma um veredicto em falha de teste.

Criado por `KRN-11` (ARQ-530, `docs/specs/SPEC-SJ66880S-dmpf-testes-interop-go.md`);
decisões em `docs/adr/040-test-kits-golden-e-fitness-function-em-go.md`.

## Por que um módulo próprio

`KRN-02` a `KRN-10` entregaram o kernel com testes por módulo, cada um com o
seu `fixedClock{}`, o seu `capturingPublisher` e o seu harness; o oráculo 3 do
round-trip era um `t.Log` que não reprovava, e as 36 células da matriz eram
provadas no oráculo interno do `decide`, não por par de vetores executável. O
kit recolhe isso num único lugar — e, porque o verificador trata todo package
não-teste como produção (`FIT-01`), cada package declara o bloco que as suas
arestas permitem: um `domainkit` só é kit de domínio se ele próprio for
`domain`.

## Unidades do manifesto

| Unidade | Bloco | Package | Kit |
| --- | --- | --- | --- |
| `dmpf-kernel/testkit-domain` | `domain` | `domainkit` | `KIT-02` — domínio em memória, projeção observável |
| `dmpf-kernel/testkit-golden` | `contract` | `golden` | carregador, catálogo, duas direções, `DMPF-R001..R003` |
| `dmpf-kernel/testkit-services` | `provider` | `serviceskit` | `KIT-03` — ports fakes com ledger |
| `dmpf-kernel/testkit-provider` | `provider` | `providerkit` | `KIT-04` — conformidade de UoW, inbox e outbox store |
| `dmpf-kernel/testkit-clock` | `provider` | `clock` | `KIT-07` — relógio fake com avanço explícito |
| `dmpf-kernel/testkit-ids` | `provider` | `ids` | `KIT-07` — identificadores em sequência ou por seed |
| `dmpf-kernel/testkit-stable` | `provider` | `stable` | `KIT-08` — ordenação estável de coleções comparadas |
| `dmpf-kernel/testkit-app` | `app` | `appkit` | `KIT-05` — harness borda a borda |
| `dmpf-kernel/testkit-dist` | `app` | `distkit` | `KIT-06` — dois processos sobre o broker, `DMPF-R004` |
| `dmpf-kernel/testkit-fitness` | `app` | `fitness` | `FIT-01..FIT-04` — regra de dependência na suíte |
| `dmpf-kernel/testkit-tb` | `app` | `tb`, `tb/pg` | adaptador de `testing.TB`, fixtures, codec de projeção, pool Postgres |

## Os cinco kits — o que exigem, o que exercitam, o que aprovam

Todo kit devolve um **veredicto por valor**: uma lista de diagnósticos, vazia no
passe, cada um nomeando a regra normativa violada. `tb.Require(t, verdict)`
converte em `t.Errorf`, um por diagnóstico. Os blocos `domain` e `contract`
não podem importar `testing` (capability fora da allowlist), e é por isso que o
adaptador é um package `app` à parte.

| Kit | Exige do candidato | Exercita | Aprova quando |
| --- | --- | --- | --- |
| `domainkit` | `Subject[S,R]`: a UPR, a projeção da resposta, do evento e do estado, e um clone do alvo — tudo **por valor**, sem duplo (`ORA-36`) | `Run` executa a UPR e lê o desfecho duas vezes; `ReadTwice` compara duas execuções | `Equal(got, want)` sem diagnóstico: ramo, resposta ou rejeição, sequência ordenada de eventos e estado antes/depois iguais aos da fixture (`ORA-31`, `ORA-34`); sob `Rejected`, sequência vazia e estado idêntico (`ORA-38`); nenhum segundo acessor de eventos (`ORA-37`) |
| `serviceskit` | Um service composto sobre `Fakes.UnitOfWork` — os fakes envolvem `dmpf-application/example/memory` e registram cada gesto no `Ledger` | O caso de uso real, aceito e recusado | `Decide` sem diagnóstico: escrita e enfileiramento entre o mesmo `begin` e `commit` (`UOW-07`); nenhum `publish` (`UOW-08`); sob recusa nada persiste (`UOW-06`); uma transação por caso de uso (`UOW-01`) |
| `providerkit` | `UnitOfWorkSubject` (UoW, uma escrita, contagem do que persistiu), `InboxSubject` (`Within`, leitura do status), `OutboxSubject` (store, enfileirar, relógio fake, status) — com uma função que devolve o candidato **limpo** | As cláusulas de `Within` (UOW-01/02/06/07/09, CTX-21, ERR-22), as recepções R1-R4 e a corrida de duas inserções (`INB-06`), o lease e as três transições condicionadas ao claimant (`OBX-09/10/11/18/06`) | Sem diagnóstico. Cláusula que o candidato não consegue exercitar (Postgres não injeta falha de commit; `memory` serializa e não corre) vai para `Skipped`, nunca fica ausente em silêncio |
| `appkit` | Postgres (`DMPF_PG_DSN`) | `dmpfapp.Consumer` real composto sobre as realizações, alimentado com bytes na borda de protocolo | `Effects` mostra o desfecho esperado nas quatro tabelas; `Ack` prova que o gesto veio depois do commit (`INB-08`) |
| `distkit` | Redpanda (`DMPF_KAFKA_BROKERS`) e Postgres | Dois processos OS — `producer` publica `evt-1`, `evt-1`, `evt-9`; `consumer` consome pelo adapter — sobre um tópico único por execução | `Decide` sem `DMPF-R004`: uma reserva, com os itens de uma única entrega, escrita uma vez (`V32`). O papel `consumer-naive`, que aplica o efeito a cada entrega, reprova nomeando o `message_id` reentregue |

Cada kit tem, no próprio módulo, o **par de vetores** que `ORA-39` exige: o
positivo contra o exemplo do kernel (`orders`, `reservations`, `memory`,
Postgres) e o negativo contra uma realização de fixture não conforme — domínio
com segundo acessor de eventos, service que enfileira fora da transação, store
que aceita claim vencido, consumidor que duplica o efeito.

## Determinismo (`KIT-07`, `KIT-08`)

- `clock.Fake` realiza `dmpfports.Clock` e expõe `Observability()` como
  `clock.Clock` do `dmpf-observability` sobre o **mesmo instante**: `Advance`
  move os dois e dispara os timers pendentes. `Set` para trás descarta timers.
- `ids.Sequence` emite `prefixo + contador de seis dígitos`; `ids.Seeded` emite
  32 hex a partir de um PCG fixado pela seed; `ids.NewClaimIDs()` satisfaz
  `relay.ClaimIDs` por forma, sem importar `dmpf-app`.
- `stable.SortStrings`, `stable.SortBy` e `stable.Sequence[T]` (coleção
  *append-only* com posição, base do `Ledger`) tiram ordem de mapa e
  entrelaçamento de goroutines de qualquer comparação.

Toda suíte do kit passa em `go test -race -count=3`.

## `golden` — carregador, duas direções, três oráculos

- `Decode([]byte)` recusa `format_version` desconhecida (`FIX-09`) e **qualquer
  escalar que não seja string**, nomeando o caminho (`FIX-07`);
  `StringScalars` expõe essa regra para outros codecs de fixture.
- `Catalog.Add` recusa a segunda fixture do mesmo contrato-major, nomeando os
  dois caminhos (`FIX-11`).
- `Consumer.Run` (bytes → campos) e `Producer.Run` (campos → bytes) devolvem
  três `Outcome` cada, um por oráculo, com fixture, caso, direção, campo,
  esperado e obtido (`FIX-12`): `DMPF-R001` semântica, `DMPF-R002` hash,
  `DMPF-R003` bytes. O oráculo 3 **reprova** na direção produtor (`ENV-24`).
  `Evaluate` roda o consumidor sobre casos e discriminadores e o produtor só
  sobre os casos canônicos — um discriminador é não-canônico por construção.
- `Report` serializa em ordem estável (`FIX-13`); `tb.RequireReport` falha o
  teste uma vez por `Outcome` reprovado.

O `dmpf-contracts/golden` continua dono das fixtures de wire e do gerador
(`GOLDEN_UPDATE=1`); só delega o carregador e os oráculos ao kit (FND-05 §8.4).

As fixtures de **projeção observável** (`ORA-30`) vivem em
`contracts/fixtures/<ctx>/projection/v1/*.golden` e são lidas por
`tb.LoadProjection`; o codec fica em `tb` porque `encoding/json` é capability
`wire.codec`, vedada ao bloco `domain`.

## `fitness` — a regra de dependência na suíte

- `TestNenhumaArestaProibida` constrói o universo de produção do workspace via
  `dmpf-conformance/fitness` e assevera zero diagnósticos (`FIT-01`, `FIT-02`);
  o baseline e o `--base` ficam fora — o teste não exerce o trust model
  (`FIT-03`).
- `Cells` transcreve à mão as 36 células de RFC §7.4 e, para cada uma, constrói
  o par de universos sintéticos: célula proibida reprova com `DMPF-D001`, célula
  permitida entre contextos sem superfície pública reprova com `DMPF-D002`. O
  metateste exige 36 células, 17 permitidas, 19 proibidas, e uma célula
  adulterada é nomeada (`RAS-06`).
- `TestDomainTestsNeedNoInfrastructureDouble` (`V29`/`V30`, `PIR-17`): o
  fechamento dos packages de teste de toda unidade `domain` não alcança unidade
  `port` ou `provider`, e os imports diretos não trazem capability fora de
  `pure` — o `testing` é o instrumento e fica isento.
- `TestKitPackagesRespectBlockCapabilities` prova que `domainkit` e `golden`
  não importam `testing`, `os`, `time` (nem `encoding/json`, no domínio).
- `TestV31NoArtifactPromisesExactlyOnce` (`P0-3`, `RAS-12`) varre contratos,
  READMEs e manifestos por promessa de *exactly-once* sem frase de vedação.

## Tabela regra → vetor (`RAS-01`)

`fitness.Vectors` transcreve `V13`..`V32` de RFC §11.3 com a célula, o código e
o lugar da prova. **`V27` é o único vetor assimétrico** (`RAS-15`, `RAS-16`):
`single-stack: typescript`, porque Go não tem *import* apagado em compilação —
toda aresta é de runtime e já cai em `V13`..`V17`. `Gaps("go")` é zero: a
assimetria está registrada, não dissimulada.

## Escolhas de FND-09 §8.4

Framework de teste: `testing` da stdlib e `go test`. Ferramenta de container:
`docker run` no CI e `docker compose` local — as mesmas dos treze módulos
anteriores; testify e testcontainers-go criariam dependência externa nova sem
ganho de decidibilidade.

## Como rodar

```bash
# unitário (sem infra): clock, ids, stable, domainkit, golden, serviceskit, providerkit (memória), fitness, tb
pnpm nx run dmpf-testkit-go:test-race

# com Postgres (appkit, providerkit sobre Postgres via dmpf-provider-postgres, tb/pg)
docker compose -f infra/local/docker-compose.yml --profile postgres up -d
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' pnpm nx run dmpf-testkit-go:test-race

# distribuído (distkit; build tag `distributed`, fora do test-race)
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 \
  pnpm nx run dmpf-testkit-go:test-distributed
```

Sem a variável, os testes de integração fazem `t.Skip` nomeando-a; com `CI`
definido, falham (`tb.Env`, fail-closed). O `test-distributed` depende do
`test-race` de `dmpf-provider-postgres-go` e `dmpf-app-go`, porque os três
compartilham o Postgres do job. No CI, o `ci.yml` roda a cadeia por estágio
(`layer:domain` → `services` → infra → `contract` → `providers` → `apps`) e o
`dmpf-distributed.yml` roda o `distkit` em pipeline próprio (`KIT-11`).

## Limitação conhecida

O `depguard` do `.golangci.yml` seleciona a regra `domain` por caminho
(`**/*-domain/**`), que `dmpf-testkit/domainkit` não casa. O gate autoritativo
é o verificador `dmpf-conformance`, que classifica por `dmpf-units.json` e
alcança o package; o teste de capability do próprio kit é a segunda linha.

O grafo de projetos do Nx conta imports de `_test.go`, então os módulos que
rodam as suítes do kit apontam para ele e o kit aponta para `dmpf-app`,
`dmpf-application` e `dmpf-provider-postgres`. O `build` deste módulo declara
`dependsOn: []` para quebrar o ciclo de tasks que `^build` formaria; as arestas
de projeto continuam e o `affected` as vê.
