---
id: SPEC-NYD18TGD
slug: dmpf-resiliencia-observabilidade-go
title: DMPF KRN-09 — Resiliência e observabilidade Go: OpenTelemetry e retry por conjunção
stage: planning
priority: P1
depends_on: [SPEC-MQA5HAXF, SPEC-WTAXFV8B, SPEC-ZHE7DN1H]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-528
subtask_urls: []
created: 2026-09-04
---

# SPEC-NYD18TGD: DMPF KRN-09 — Resiliência e observabilidade Go: OpenTelemetry e retry por conjunção

## Resumo

Criar o módulo Go `dmpf-observability-go` (`libs/backend/go/dmpf-observability`,
bloco `provider`) que realiza a baseline de resiliência e observabilidade do
DMPF (FND-08, ADR-026) sem acrescentar um único import de telemetria a `domain`
nem a `port`: o bootstrap OpenTelemetry com propagador W3C Trace Context
configurado explicitamente e a versão das *semantic conventions* lida de pin
declarado; os decorators de saída — timeout, circuit breaker, bulkhead,
degradação e retry — compostos na ordem canônica de `RES-22` e observáveis por
construção; o avaliador de retry como função pura sobre os quatro fatores de
`RES-27`, com orçamento **por execução** e backoff que consome orçamento; o
catálogo de métricas de dependência e de serviço com nome, unidade e fórmula; e
o canal de auditoria separado do log. A instrumentação do application service
do `KRN-04` entra por um gancho **puro** declarado em `dmpf-application`
(`Instrumentation`), que o provider satisfaz estruturalmente e o composition root
liga — porque a matriz de blocos proíbe `application → provider` **e**
`provider → application`.

Como time de plataforma, queremos que `RES-01`, `RES-27` a `RES-33`, `TRC-03`,
`TRC-09`, `TRC-14`, `TRC-16`, `MET-03`, `MET-07`, `LOG-13` e `LOG-14` deixem de
ser prosa e passem a ser código que o verificador do `KRN-02` e a suíte do
módulo provam — para que a primeira dependência externa do kernel
(`go.opentelemetry.io/otel`, família `v1.46.0`) entre pela porta certa: com
pin, allowlist, capability declarada e ADR.

Esta é a primeira metade do Incremento 4 ("Fronteiras") da spec guarda-chuva
[SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md): "Serviço observável,
resiliente e falando gRPC/Kafka". A segunda metade é o `KRN-10`, que compõe com
estes decorators sem os reabrir. O `KRN-12` registra no BOM o pin das
convenções que esta spec fixa em código.

## Contexto

- **Problema**: a baseline de FND-08 é hoje inteiramente documental — quarenta
  regras `RES`, dezesseis `TRC`, trinta `MET` e quatorze `LOG` sem uma linha de
  código que as realize. O `KRN-04` entregou o application service onde o span
  da regra (`TRC-16`), a auditoria (`LOG-14`) e o orçamento da execução
  (`RES-01`) precisam pendurar-se, e o lugar mais cômodo para instrumentar
  continua sendo o proibido: o tipo de domínio e a porta. Sem esta entrega,
  cada provider do `KRN-06`, `KRN-07` e `KRN-10` decidiria timeout, retry e
  métrica por conta própria, e a divergência que a baseline existe para fechar
  reapareceria no primeiro provider concreto.
- **Impacto**: o `KRN-10` compõe transporte com decorators prontos, observáveis e
  já provados contra a norma; o `KRN-12` lê o pin das *semantic conventions* de
  uma constante em vez de o inventar; e uma squad que use o kernel herda o
  default fail-closed de retry (`RES-29`) sem poder invertê-lo por
  configuração. Um `POST` sem chave de idempotência que estoure o prazo deixa de
  ser repetido ainda que a taxonomia o classifique como retentável
  (ADR-026, exemplo 2).
- **Inspiração**: o próprio `KRN-04` — a realização em memória da UoW é bloco
  `provider` dentro de um módulo `application`, e o vínculo entre transação e
  recursos é função de composição escrita pelo composition root
  (`SPEC-ZHE7DN1H`, "Divergências", linha 5). Esta spec repete o gesto: o gancho
  de instrumentação é interface pura em `dmpfapplication`, a realização OTel é
  `provider`, e quem liga os dois é `app`. Na comunidade Go, o padrão de
  decorator por composição explícita — `sony/gobreaker`, `failsafe-go`,
  `cenkalti/backoff` — confirma a forma, mas nenhum deles decide retry por
  conjunção de quatro fatores nem conta orçamento por execução; por isso o
  avaliador é escrito aqui, como função pura, e não adotado de biblioteca.
- **Links relevantes**:
  - [SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md) — guarda-chuva;
    Incremento 4, linhas `KRN-09` e `KRN-10` da decomposição.
  - [SPEC-ZHE7DN1H](./SPEC-ZHE7DN1H-dmpf-kernel-aplicacao-go.md) — `KRN-04`:
    `dmpf-ports`, `dmpf-application`, `example/orders` e `example/memory`, o
    que esta spec instrumenta.
  - [SPEC-WTAXFV8B](./SPEC-WTAXFV8B-dmpf-verificador-conformidade-go.md) —
    `KRN-02`: o verificador que decide capability e matriz sobre o módulo novo.
  - [SPEC-E15TBHCD](./SPEC-E15TBHCD-dmpf-resiliencia-observabilidade.md) —
    a spec que produziu FND-08 (ARQ-445); seus critérios estão em FND-08 §13.2.
  - `docs/dmpf/resiliencia-observabilidade.md` — FND-08, §1.3, §3, §4, §5.3,
    §5.5, §5.6, §6.1, §6.2, §6.8 e §7.
  - `docs/dmpf/contexto-erros-seguranca.md` — FND-07, `ERR-11`, `ERR-12`,
    `CTX-18` a `CTX-23`, `CTX-28`, `DAT-22` a `DAT-25`.
  - `docs/adr/026-baseline-resiliencia-observabilidade-opentelemetry.md`,
    `docs/adr/015-capabilities-externas-por-bloco.md`,
    `docs/adr/010-*.md` e `docs/adr/014-*.md` (matriz e aresta `domain → port`),
    `docs/adr/031-verificador-de-conformidade-dmpf-em-go.md` (leitura do
    complemento: `observability` passa em `application`).
  - `libs/backend/go/dmpf-conformance/internal/rule/capability.go:46-53` —
    política por bloco; `matrix.go:57-62` — as células.
- **Referências externas**:
  - OpenTelemetry Go — `go.opentelemetry.io/otel` `v1.46.0` (publicado em
    2026-08-25, `go 1.25.0`), com `otel/trace`, `otel/metric`, `otel/sdk`,
    `otel/sdk/metric` e os exportadores OTLP na **mesma** versão; o módulo
    contém `semconv/v1.43.0`, a versão mais recente das *semantic conventions*
    empacotada nessa release (verificado por `go list -m -versions` e pelo
    conteúdo do módulo baixado em 2026-09-04).

### Errata de 2026-09-04 (revisão do plano)

Aprovada pelo dono da spec após a segunda opinião externa sobre o plano de
implementação. Nenhuma regra normativa foi relaxada; as mudanças fecham
contratos que não compilavam ou contradiziam o SDK:

1. Piso e toolchain Go passam de `1.26.4` para **`1.26.8`** (Fase 0 da
   entrega): no piso anterior, `govulncheck v1.7.0` reprova o fechamento dos
   exportadores OTLP/gRPC (GO-2026-6090, GO-2026-5856, GO-2026-5972).
2. `retry.Input` ganha `MaxAttempts` e `Rand`; `resilience.Operation` ganha
   `Kind`, `EffectAbsent` e `EstimatedDuration`; `RES-07` sai do `Compose` e
   vira `ValidateRoute(remaining, hops)`.
3. `EndOperation` recebe `Result{Outcome, Err}`; `dmpfapplication.ErrDenied` é
   o sentinela que distingue negação de falha técnica do autorizador; o
   provider classifica `error_category` com `Classifier` injetado.
4. `otelboot.Config` ganha `Transport` (TLS/insecure explícito) e `Sheets`
   (valores efetivos entram no `resource` na criação); o sampler é próprio
   (nunca `Drop`) e um único `classAwareProcessor` é dono do exporter.
5. `logging.Config.Fields` é o extractor injetado de `correlation_id`,
   `request_id` e `tenant_id`; o tempo do provider é `clock.Clock` (fake
   avançável), com `dmpf-ports` intocado.
6. Todos os packages nascem no walking skeleton e a membership é classificada
   em um único commit; o cenário de esgotamento do orçamento fixa
   `EstimatedDuration` e durações; o cenário de amostragem usa `TraceID`s
   construídos.

### Errata de 2026-09-04 (execução — o gancho é porta, não tipo de aplicação)

Aprovada pelo dono da spec durante a Fase 1, após medição em código. Corrige uma
premissa que a linguagem não sustenta.

7. **O gancho de instrumentação vive em `dmpf-ports`, não em `dmpf-application`.**
   Esta spec afirmava que o provider "satisfaz a interface estruturalmente, sem
   importar `dmpf-application`". Go satisfaz interface por assinaturas
   **idênticas**, nunca por estrutura: um provider que declarasse o seu próprio
   `Result` não satisfaria a interface, e `provider → application` é célula
   proibida (`matrix.go:58,61`). Medido em módulo descartável:

   ```text
   cannot use prov.Provider{} as app.Instrumentation value:
     have BeginOperation(context.Context, string) (context.Context, prov.EndOperation)
     want BeginOperation(context.Context, string) (context.Context, app.EndOperation)
   ```

   `Instrumentation`, `EndOperation`, `Result`, `OutcomeCategory`, `AuditEvent`
   e `ErrDenied` passam a viver em `dmpfports`. Ambos os blocos já importam
   ports (`application → port` e `provider → port` são células permitidas), então
   a satisfação é direta, com tipos nomeados, sem shim e sem alargar a matriz —
   o mesmo arranjo que o kernel já usa para `UnitOfWork`, `Repository`, `Outbox`
   e `Clock`: necessidade declarada acima, realizada abaixo.

   Consequências: onde esta spec escreve `dmpfapplication.<símbolo do gancho>`,
   leia-se `dmpfports.<símbolo>`; o item 5 desta errata deixa de valer quanto a
   "`dmpf-ports` intocado" — o módulo ganha `instrumentation.go` e
   `instrumentation_test.go`. O `dmpf-units.json` e o baseline de `dmpf-ports`
   **não** mudam: o `include` é por import path do package, e o package raiz
   `dmpfports` já está declarado. O `clock` do provider segue em
   `dmpf-observability`, porque `dmpf-ports` não pode importar `time`.

8. **`version_test.go` não usa `debug.ReadBuildInfo`.** Sob o `go.work` deste
   repositório, `ReadBuildInfo()` devolve `Deps` vazio no binário de teste
   (medido: `deps=0`) — em workspace mode o Go não grava o grafo de módulos no
   build info. A prova do pin lê o `go.mod` do módulo, que é a fonte declarativa
   do BOM (ADR-030), e a do semconv compara `semconv.SchemaURL` do package
   importado com `SemconvVersion`.

### Divergências entre o ticket e o repositório

O ticket ARQ-528 foi escrito em 30/08, antes de `KRN-04` e `KRN-05` serem
entregues e antes de a política de blocos ganhar as regras `port`, `application`
e `contract` no `.golangci.yml`. Dez pontos do texto não batem com o estado do
repositório e ficam fixados aqui, sem reabrir decisão alguma:

| Ticket diz | Repositório | Esta spec fixa |
| --- | --- | --- |
| "Criar `libs/backend/dmpf-observability`" | A convenção do ADR-030 e do `KRN-01` é `libs/<scope>/<stack>/<módulo>`, com o nome do projeto Nx sufixado pela stack | `libs/backend/go/dmpf-observability`, projeto Nx `dmpf-observability-go`, unidade `dmpf-kernel/observability` |
| "Instrumentar o application service de `KRN-04` sem tocar os tipos de `domain`" | A matriz (`matrix.go:58,61`) proíbe `application → provider` **e** `provider → application`; o application service não pode importar OTel diretamente sem o `.golangci.yml` abrir a `allow` estrita de `application`, e o provider não pode importar `dmpf-application` | Gancho **puro** `dmpfapplication.Instrumentation` (só `context` e tipos do próprio bloco); o provider satisfaz a interface estruturalmente, sem importar `dmpf-application`; o composition root (aqui, a suíte de integração) liga os dois. `dmpf-application` **não ganha `require`** |
| "Erro retentável" como primeiro fator da conjunção | Não existe taxonomia de erro no kernel Go: a realização de FND-07 não é história do épico, e o `KRN-04` a deixou fora de propósito | O avaliador recebe um predicado `Classifier func(error) Retryability` injetado pelo composition root; classificação ausente ou indeterminada resolve para **não retentável** (`ERR-11`, `RES-29`). A taxonomia é consumida, nunca definida aqui |
| "Orçamento **por execução**" | O `context.Context` transporta só cancelamento e deadline (`CTX-20`, `CTX-21`; `SPEC-ZHE7DN1H`, constraint P1), a porta não pode carregar tipo de política (`RES-24`) e o application service não pode importar o provider | O `Budget` vive no package `retry` do provider e é criado pela **borda** (`app`) com `retry.WithBudget(ctx)`; é o único valor de contexto que o kernel admite, e sua **ausência** só restringe (fator 3 falso, nenhum retry). Nunca é fonte de correção do resultado de negócio |
| "Decorators no `provider` (`RES-05` a `RES-24`): timeout, breaker, bulkhead e degradação" | A ordem canônica de `RES-22` tem nove posições, incluindo rate limiting e cache-aside; a admissão por rota e tenant é do `KRN-10` e a modelagem de cache está `encaminhada` (FND-08 §1.4) | `Compose` aceita as nove posições na ordem de `RES-22`; rate limiting e cache são posições **opcionais** que esta spec deixa vazias, e a `Sheet` as marca «não se aplica» com motivo, como `RES-21` exige |
| "Versão das *semantic conventions* lida de pin declarado (`TRC-03`)" | O BOM é do `KRN-12`; não existe ainda | O pin é o import path `go.opentelemetry.io/otel/semconv/v1.43.0`, re-exportado como constante `dmpfobservability.SemconvVersion = "1.43.0"` e repetido no campo `versions` da allowlist do manifesto; o `KRN-12` o copia para o BOM |
| "Registrar em ADR a escolha de exportador e destino de coleta" e "ADR a partir de `029`" | `034` é do `KRN-04`; o `KRN-06` (ARQ-525, em branch paralela) pode reivindicar `035` | O ADR desta história é o **`035`**. A colisão possível com o `KRN-06` fica em "Escopo fora": quem mergear por último renumera, como o `KRN-05` fez com o `033` |
| "Métricas com nome, unidade e fórmula (`MET-03`)" sem enumerar quais | FND-08 §6 tem trinta métricas, distribuídas entre `app`, `provider`, relay, inbox e pool | Esta spec realiza **seis regras, dez séries**: `MET-28` (três séries) e `MET-29` (duas) nos decorators; `MET-08`, `MET-09`, `MET-10` e `MET-13` (duas) pelo gancho do application service. `MET-11`, `MET-12` (admissão) e `MET-30` (pool) são do `KRN-10` e do `KRN-06` |
| "Erro sempre amostrado (`TRC-14`)" | A decisão de amostragem do SDK OTel ocorre no início do span, antes de o desfecho ser conhecido; os samplers de fábrica devolvem `Drop` no ramo negativo | Regra equivalente em processo, como `TRC-14` admite: `classSampler` próprio devolve `RecordAndSample` na taxa da classe e `RecordOnly` fora dela (nunca `Drop`); um único `classAwareProcessor`, dono do exporter, exporta **todo** span encerrado com `sampled` ou com status de erro. A garantia é do **span** em erro; o trace completo é do tail sampling no collector (ADR-035) |
| "Evento de auditoria com sujeito, objeto, ação, desfecho e instante (`LOG-14`)" | A identidade autenticada de FND-07 (`IDN-*`) não tem realização Go | `AuditEvent.Subject` é `string` preenchido pela realização do provider a partir de um `SubjectFunc` injetado pelo composition root; ausência é registrada como ausente (`""`), nunca substituída por default. A realização da identidade fica fora |

### Fontes normativas

Regras que esta spec realiza, com a força que cada fonte declara:

| Fonte | Regras | O que obriga aqui |
| --- | --- | --- |
| FND-08 §1.3 | `RES-01`, `RES-02` | O sujeito é `app`, `provider` ou `application service`; `domain` e `port` não recebem regra nem satisfazem regra |
| FND-08 §3.1 | `RES-05`, `RES-21`, `RES-22`, `RES-23`, `RES-24` | Nenhuma chamada externa sem timeout e política declarada; ficha de resiliência por dependência sem campo em branco; ordem canônica de composição; todo decorator observável; nenhum decorator em `domain`/`port` |
| FND-08 §3.2 | `RES-06`, `RES-07`, `RES-09` | Timeout efetivo = `min(prazo_do_método, prazo_remanescente, orçamento_restante − backoff_reservado)`; soma dos orçamentos a jusante verificada na composição; valor do prazo por método é de FND-06 |
| FND-08 §3.3 | `RES-10`, `RES-11`, `RES-12` | Breaker com janela 30 s, limiar 50 %, mínimo 20 amostras, cooldown 30 s, 1 sonda; vive no `provider`; aberto falha rápido com categoria própria |
| FND-08 §3.4 | `RES-13`, `RES-14` | Pool por dependência; fila do tamanho do pool; aquisição em 100 ms; saturação é rejeição rápida |
| FND-08 §4.1 | `RES-25`, `RES-26` | Retry é da chamada remota; nunca reabre a UoW nem reexecuta o caso de uso |
| FND-08 §4.2 | `RES-27`, `RES-28`, `RES-29` | Conjunção de quatro fatores antes de **cada** tentativa; retryability não autoriza; indeterminado é falso |
| FND-08 §4.3 | `RES-30`, `RES-31`, `RES-36` | Orçamento por execução, default metade do prazo remanescente na primeira falha; backoff consome orçamento; esgotamento observável em `MET-28` e no span |
| FND-08 §4.4 | `RES-32`, `RES-33`, `RES-34`, `RES-35` | Base 100 ms, fator 2, jitter total, teto 5 s por espera; 3 tentativas síncronas e 5 assíncronas; retry não atravessa commit; conflito de escrita só por política explícita |
| FND-08 §4.5 | `RES-37`, `RES-38`, `RES-39`, `RES-40` | Quatro modos de degradação declarados; degradar não silencia; contenção conjunta; override declarado, versionado e exposto |
| FND-08 §5.3, §5.5, §5.6 | `TRC-03`, `TRC-04`, `TRC-06`, `TRC-11`, `TRC-12`, `TRC-13`, `TRC-14`, `TRC-15`, `TRC-16` | Convenção OTel com versão fixada; atributos comuns; `correlation_id` como atributo; tentativa observável; erro por status e categoria sem payload; amostragem por classe; erro sempre amostrado; atributos por allowlist; span da regra aberto pelo application service |
| FND-08 §6.1, §6.2, §6.8 | `MET-02`, `MET-03`, `MET-04`, `MET-05`, `MET-07`, `MET-08`, `MET-09`, `MET-10`, `MET-13`, `MET-28`, `MET-29` | `dmpf_<componente>_<sinal>[_<unidade>]`; nome, unidade e fórmula; labels por allowlist; limiar condicional; sem identificador como label; oito métricas realizadas |
| FND-08 §7 | `LOG-01`, `LOG-02`, `LOG-04`, `LOG-06`, `LOG-07`, `LOG-10`, `LOG-11`, `LOG-12`, `LOG-13`, `LOG-14` | JSON estruturado com campos obrigatórios; correlação automática; redaction na origem preservando o campo; severidade por significado; log fora do domínio; amostragem alinhada ao trace; auditoria em canal separado, emitida pelo application service |
| FND-07 §5, §3 | `ERR-11`, `ERR-12`, `CTX-18` a `CTX-23`, `CTX-28`, `DAT-22` a `DAT-25` | Default fail-closed da retryability; `deadline` é instante monotônico; estouro e cancelamento têm categorias próprias; redaction alcança log, trace e métrica |
| ADR-026 | `TRC-09` e a decisão inteira | Propagador W3C configurado explicitamente, sem fallback proprietário; as quatro escolhas substantivas |
| ADR-015; RFC §6.2; `capability.go:46-53` | Política por bloco | `provider` permissivo; `application` só `pure` + `observability`; `observability` proibida em `domain` e `port` mesmo que a biblioteca seja pura |
| ADR-010, ADR-014; `matrix.go:57-62` | Células da matriz | `provider → domain`, `provider → port`, `provider → provider`, `provider → contract` permitidas; `provider → application` e `provider → app` **proibidas**; `application → provider` **proibida** |
| RFC §10.2; ADR-012 | Classificação declarada | Manifesto e baseline em commit próprio, sem código Go |

<constraints>
- [P0] Sujeito da baseline (`RES-01`, `RES-24`, `TRC-16`, `LOG-11`): nenhuma
  linha desta entrega acrescenta import de telemetria, decorator, parâmetro de
  retry, prazo ou política de falha a `dmpf-domain` nem a `dmpf-ports`. O
  fechamento transitivo de imports de toda unidade `domain` e `port` NÃO alcança
  `go.opentelemetry.io/*`, `log`, `log/slog` nem o módulo novo.
- [P0] Matriz de blocos (`matrix.go:58,61`; ADR-010): `dmpf-observability`
  NUNCA importa `dmpf-application` em código de produção
  (`provider → application` proibida) e `dmpf-application` NUNCA importa
  `dmpf-observability` (`application → provider` proibida). O gancho
  `Instrumentation` é satisfeito estruturalmente; o vínculo é do composition
  root.
- [P0] `dmpf-application` permanece com fechamento `pure` + `observability`
  segundo `internal/rule/stdlib.go`: o gancho usa APENAS `context` e tipos do
  próprio bloco e de `dmpfports`. O `go.mod` de `dmpf-application` NÃO ganha
  `require`.
- [P0] Conjunção de retry (`RES-27`, `RES-28`, `RES-29`): uma tentativa só é
  autorizada com os QUATRO fatores verdadeiros, verificados antes de CADA
  tentativa; classificação ausente ou indeterminada é falsa; o veredicto
  negativo NOMEIA o fator falso. Nenhuma opção de configuração inverte o
  default.
- [P0] Orçamento por execução (`RES-30`, `RES-31`, `RES-36`): um único
  `Budget` por execução, consumido por todas as dependências; a espera de
  backoff é debitada; ausência de `Budget` no contexto significa fator 3
  falso. O `Budget` é o ÚNICO valor de `context.Context` admitido pelo kernel e
  nunca é fonte de correção do resultado de negócio.
- [P0] Retry é da chamada remota (`RES-25`, `RES-34`): o decorator envolve a
  chamada do provider, NUNCA `UnitOfWork.Within`, NUNCA o caso de uso, NUNCA
  uma chamada posterior ao commit.
- [P0] Propagador explícito (`TRC-09`; ADR-026): o bootstrap exige um
  `propagation.TextMapPropagator` W3C Trace Context na configuração; sem ele
  falha com `ErrPropagatorRequired`. NUNCA cai em default silencioso nem em
  formato proprietário.
- [P0] Allowlists (`TRC-15`, `MET-04`, `MET-07`, `DAT-22`): atributo de span e
  label de métrica entram só por chave declarada; `message_id`,
  `correlation_id`, `request_id`, identificador de agregado e de usuário NUNCA
  são labels de métrica; erro entra por status e categoria, NUNCA por mensagem
  ou payload.
- [P0] Auditoria separada (`LOG-13`, `LOG-14`, `DAT-25`): o `audit.Sink` NÃO é
  derivado do handler de log, NÃO passa por amostragem e recebe sujeito,
  objeto, ação, desfecho e instante, emitidos pelo application service.
- [P0] Overrides (`RES-40`): todo desvio de default é registrado na `Sheet`
  com valor, motivo e data, e o valor efetivo é exposto como metadado de
  telemetria. Default alterado sem registro é defeito.
- [P0] Classificação declarada (ADR-012; RFC §10.2): a unidade
  `dmpf-kernel/observability` cobre cada package de produção por import path
  exato; as entradas `external[]` declaram pacote, faixa, entrypoints e
  capability `observability`; manifesto e baseline vêm em COMMIT PRÓPRIO, sem
  código Go.
- [P0] Nenhum artefato declara nem sugere exactly-once fim a fim (P0-3).
- [P0] Esta spec NÃO reabre decisão da fundação nem da guarda-chuva:
  divergência vira ADR-035, nunca alteração silenciosa da norma.
- [P1] Pin único (`TRC-03`; ADR-026): todos os módulos `go.opentelemetry.io/otel*`
  no `go.mod` estão na MESMA versão (`v1.46.0`); as *semantic conventions* são
  `semconv/v1.43.0`, e a constante `SemconvVersion` bate com o import path.
  `go.sum` do módulo e `go.work.sum` são commitados.
- [P1] Determinismo dos testes: tempo por `clock.Clock` do provider (`Now`,
  `After`, `NewTimer`, `WithTimeout`) com realização fake avançável — a porta
  `dmpfports.Clock` só expõe `Now()` e não desbloqueia `select` nem cancela
  contexto; jitter por semente explícita; espera por `Sleeper` injetado; NENHUM
  teste depende de relógio de parede nem de `time.Sleep` real. `dmpf-ports`
  permanece intocado.
</constraints>

### Errata de 2026-09-05 (execução — a numeração do ADR e o piso Go)

9. **O ADR desta entrega é o 037, não o 035.** Onde a spec cita
   `docs/adr/035-observabilidade-otel-e-retry-por-conjuncao-em-go.md`, leia-se
   `docs/adr/037-...`. Enquanto esta entrega corria, o `ARQ-525` (KRN-06) mergeou
   com o ADR-035 e o `ARQ-526` (KRN-07) com o ADR-036. A pergunta aberta do plano
   previa a colisão e apostava em `036`, que também foi tomado.

10. **O piso Go é `1.26.6`, não `1.26.8`.** Onde a spec fixa `1.26.8` — no
    Resumo, na Compatibilidade, na Localização de código e na Fase 0 — leia-se
    `1.26.6`. A escolha original mediu `1.26.4` reprovando no `govulncheck` e
    `1.26.8` passando, e **nunca mediu o `1.26.6` que estava entre os dois**; a
    própria spec já registrava que as três vulnerabilidades da stdlib estão
    corrigidas em `1.26.5`/`1.26.6`. Medido em 2026-09-05, `1.26.6` passa
    inclusive com `testcontainers-go`, desde que `github.com/moby/go-archive`
    esteja em `v0.3.0` ou superior — a vulnerabilidade que resta é dessa
    dependência, não do toolchain. Como o `develop` já havia ido a `1.26.6` por
    conta própria, o piso final coincide com o dele e nenhum módulo precisou
    subir por causa desta entrega.

## Requisitos

### Funcionais

#### Módulo e governança

- [ ] **[P0] Módulo `dmpf-observability-go`**: criar `libs/backend/go/dmpf-observability`
  com `go.mod` (`module gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability`,
  `go 1.26.8`), `package.json` (`@lidercap-apps/dmpf-observability-go`,
  `private: true`), `project.json` (tags `type:lib`, `scope:backend`,
  `stack:go`; os cinco targets `fmt-check`, `vet`, `build`, `test-race`,
  `govulncheck` idênticos aos de `dmpf-application`) e entrada
  `./libs/backend/go/dmpf-observability` no bloco `use` do `go.work`.
  - Edge case: `go.work.sum` muda ao resolver os módulos OTel; o arquivo é
    commitado no mesmo commit do `go.mod`/`go.sum`.
- [ ] **[P0] Dependências OTel pinadas**: `require` de `go.opentelemetry.io/otel`,
  `otel/trace`, `otel/metric`, `otel/sdk`, `otel/sdk/metric`,
  `otel/exporters/otlp/otlptrace/otlptracegrpc` e
  `otel/exporters/otlp/otlpmetric/otlpmetricgrpc`, todos em `v1.46.0`.
  - Sub-item: `go.opentelemetry.io/otel/semconv/v1.43.0` é o único package
    `semconv` importado; um teste falha se qualquer arquivo importar outra
    versão de `semconv`.
  - Sub-item: `dmpfobservability.SemconvVersion = "1.43.0"` e
    `dmpfobservability.OTelVersion = "1.46.0"`; um teste compara
    `OTelVersion` com a versão resolvida em `debug.ReadBuildInfo()`.
- [ ] **[P0] Manifesto `dmpf-units.json`**: uma unidade `dmpf-kernel/observability`,
  `block: provider`, `bounded_context: dmpf-kernel`,
  `public_integration_surface: false`, `include` com o import path de CADA
  package de produção do módulo (raiz, `clock`, `otelboot`, `otelboot/otlp`,
  `resilience`, `retry`, `metrics`, `tracing`, `logging`, `audit`, `redact`,
  `usecase`). Todos os packages nascem com `doc.go` no walking skeleton, e a
  membership completa é classificada em UM commit `chore` — o verificador
  emite `DMPF-U001` para package de produção fora de todo `include`
  (`internal/rule/universe.go:190`), então classificação parcial deixaria as
  fases intermediárias vermelhas.
  - Sub-item: `external[]` com uma entrada por módulo OTel requerido, cada uma
    com `package`, `versions: ">=1.46.0 <2"`, `entrypoints` exatos usados e
    `capability: "observability"`. `exceptions: []`.
  - Sub-item: `tools/dmpf-baseline/units-baseline.json` ganha a entrada e o
    `digest` é regravado. Manifesto e baseline em **commit próprio**, sem `.go`.
  - Edge case: o bloco `provider` é permissivo (`capability.go:53`) e o
    verificador aceitaria as dependências sem allowlist; declará-las é
    obrigatório aqui porque é a forma auditável do pin que o `KRN-12` lê.

#### Bootstrap OpenTelemetry (`otelboot`)

- [ ] **[P0] `otelboot.Start(ctx, Config) (*Runtime, error)`**: monta
  `TracerProvider` e `MeterProvider` do SDK a partir de `Config` e devolve um
  `Runtime` com `Tracer()`, `Meter()`, `Logger()` e `Shutdown(ctx) error`.
  - Sub-item: `Config.Propagator` é obrigatório e deve ser
    `propagation.TraceContext{}` (opcionalmente composto com `Baggage`); `nil`
    devolve `ErrPropagatorRequired`; propagador que não implemente W3C Trace
    Context devolve `ErrPropagatorNotW3C`. O propagador é registrado em
    `otel.SetTextMapPropagator` **somente** por `Start`.
  - Sub-item: `Config.Resource` exige `service.name`, `service.version` e
    `service.instance.id` (chaves de `semconv/v1.43.0`); ausência devolve
    `ErrResourceIncomplete`.
  - Sub-item: `Config.TraceExporter` e `Config.MetricReader` são injetados
    (interfaces do SDK); a suíte usa `tracetest.InMemoryExporter` e
    `sdkmetric.ManualReader`. `otelboot/otlp` fornece os construtores OTLP/gRPC
    de produção, com endpoint lido de `Config` — nunca literal no código.
  - Sub-item: `Config.Sampling` é o mapa classe → taxa de `TRC-13`, com os
    defaults `erro 1.0`, `escrita 0.10`, `leitura 0.01`, `manutenção 1.0`;
    classe ausente resolve para a taxa mais restritiva (`0.01`).
  - Sub-item: `Config.Transport{Insecure bool, TLS *tls.Config}` é escolha
    explícita e validada para traces e métricas — o exporter OTLP/gRPC exige
    TLS por default e só `WithInsecure` o desliga
    (`otlptracegrpc@v1.46.0/options.go:52`); `Insecure: true` sem
    `AllowInsecure` declarado na `Config` devolve `ErrInsecureNotAllowed`; a
    fixture do Collector é o único lugar que o liga.
  - Sub-item: `Config.Sheets []resilience.Sheet` — os valores efetivos
    (`Sheet.Effective()`) entram no `resource` **na criação** do provider como
    `dmpf.sheet.<dependency>.<field>` (`RES-40`); o `resource` não recebe
    atributos depois de criado, por isso as fichas são insumo do bootstrap.
  - Edge case: `Start` chamado duas vezes no mesmo processo devolve
    `ErrAlreadyStarted`; `Shutdown` é idempotente e respeita o `ctx`.
- [ ] **[P0] Sampler por classe (`TRC-13`)**: `classSampler` é um `Sampler`
  **próprio** (não decora `ParentBased`/`TraceIDRatioBased`, cujos ramos
  negativos devolvem `Drop` — `sampling.go:71-116,176-203`). Decide pelos
  bits do `TraceID` com a mesma fórmula de `TraceIDRatioBased` e pela classe
  em `dmpf.traffic_class` de `SamplingParameters.Attributes`; devolve
  `RecordAndSample` dentro da taxa e **`RecordOnly`** fora dela, nunca `Drop`.
  Matriz declarada: root → taxa da classe; pai local ou remoto **amostrado** →
  `RecordAndSample`; pai local ou remoto **não amostrado** → `RecordOnly`
  (para que um erro local ainda seja retido). Testes com `TraceID`s
  construídos (bits conhecidos), não com RNG.
  - Edge case: span sem `dmpf.traffic_class` é tratado como `leitura` (a taxa
    mais restritiva) e recebe o atributo `dmpf.traffic_class = "unclassified"`
    para que a omissão seja observável.
- [ ] **[P0] Processor único `classAwareProcessor` (`TRC-14`)**: um único
  `SpanProcessor` assíncrono, **dono exclusivo** do `SpanExporter`, enfileira
  em `OnEnd` (sem bloquear — `span_processor.go:25-27`) todo span com
  `FlagsSampled` **ou** `Status().Code == codes.Error`, e exporta em worker
  próprio com lote, `ForceFlush` e `Shutdown` ordenados. Substitui o
  `BatchSpanProcessor` do SDK (que descarta não-amostrados) — nunca convive com
  ele sobre o mesmo exporter (`span_exporter.go:16-19`: `ExportSpans` é
  síncrono e sem garantia de concorrência).
  - Garantia declarada (ADR-035): o **span** em erro é sempre exportado; o
    **trace** completo de erro (ancestrais e irmãos `RecordOnly` sem erro) é
    responsabilidade do tail sampling no Collector. Nenhum artefato chama o
    fragmento de "trace completo".
  - Edge case: fila cheia descarta o span mais antigo **não** amostrado e sem
    erro primeiro, e conta a perda em `dmpf_otel_spans_dropped_total`
    (métrica local, `MET-02`).

#### Decorators de saída (`resilience`)

- [ ] **[P0] Ficha de resiliência `Sheet` (`RES-21`)**: struct com os dez
  campos da tabela de `RES-21` — prazo por método, retry (predicado), orçamento,
  backoff, teto de tentativas, breaker, bulkhead, rate limit de saída, cache e
  modo de degradação. Cada campo é um valor **ou** `NotApplicable{Reason}`;
  `Sheet.Validate() error` reprova campo zero (nem valor nem motivo).
  - Sub-item: `Sheet.Overrides []Override{Field, Value, Reason, Since}`
    (`RES-40`); `Sheet.Effective()` devolve os valores em uso, e o bootstrap os
    expõe como atributos de recurso `dmpf.sheet.<dependency>.<field>` e os
    registra uma vez em `info` no arranque.
  - Sub-item: `Defaults()` devolve a ficha com os defaults da plataforma
    (§3.3, §3.4, §4.3, §4.4) e degradação `falha`.
- [ ] **[P0] Composição na ordem canônica (`RES-22`)**: `Compose(sheet, Slots) Call`
  com nove posições nomeadas — `Tracing`, `Metrics`, `Logging`, `Bulkhead`,
  `Breaker`, `RateLimit`, `Retry`, `Timeout` e a chamada — aplicadas de fora
  para dentro exatamente nessa ordem. `RateLimit` e `Cache` são opcionais e,
  vazias, exigem `NotApplicable` na `Sheet`.
  - Sub-item: `Call` é `func(ctx context.Context, op Operation, do func(context.Context) error) error`;
    `Operation{Dependency, Method string; Kind Kind; Idempotent bool;
    EffectAbsent func(error) bool; Deadline, EstimatedDuration time.Duration}`
    é o que a ficha declara por método. `Kind` ∈ {`Remote`, `UnitOfWork`};
    `EffectAbsent == nil` é `false` para todo erro; `EstimatedDuration == 0`
    reprova na validação (uma estimativa é obrigatória para os fatores 3 e 4).
  - Edge case: ordem diferente só por `ComposeWithOrder(sheet, order, reason)`;
    `reason` vazio devolve `ErrOrderReasonRequired`.
- [ ] **[P0] Timeout derivado (`RES-05`, `RES-06`, `RES-07`)**: o prazo efetivo
  é `min(op.Deadline, prazo_remanescente(ctx), orçamento_restante − backoff_reservado)`;
  `resilience.ValidateRoute(remaining time.Duration, hops []Operation) error`
  reprova (`ErrDeadlineComposition`) quando a soma dos `Deadline` dos hops a
  jusante, mais a folga por salto (`HopSlack`, default 50 ms), excede o prazo
  remanescente — é verificação de composição feita por quem conhece a rota
  (o composition root ou o application service), não pelo `Compose` de uma
  ficha isolada, que só vê uma dependência.
  - Edge case: `ctx` sem deadline usa `op.Deadline`; `op.Deadline == 0` reprova
    na validação da `Sheet` — chamada sem timeout não existe (`RES-05`).
- [ ] **[P0] Circuit breaker (`RES-10`, `RES-11`, `RES-12`)**: janela 30 s,
  limiar 50 %, mínimo 20 amostras, cooldown 30 s, 1 sonda em meia-abertura;
  estado exposto em `dmpf_dependency_breaker_state`; aberto devolve
  `ErrBreakerOpen` imediatamente, sem consumir timeout nem tentativa.
  - Edge case: com 3 chamadas na janela e 1 falha, o breaker permanece fechado
    (piso de amostras).
- [ ] **[P0] Bulkhead (`RES-13`, `RES-14`)**: semáforo por dependência com pool
  declarado, fila do tamanho do pool e aquisição em 100 ms; saturação devolve
  `ErrBulkheadSaturated` sem esperar.
- [ ] **[P0] Degradação (`RES-37`, `RES-38`)**: `Sheet.Degradation` ∈
  {`Fail`, `Degrade`, `Defer`, `Ignore`}; `Degrade` devolve `DegradedResult`
  distinguível e incrementa `dmpf_service_degraded_total`; `Ignore` incrementa
  `dmpf_service_omitted_total`; `Defer` é rejeitado nesta entrega
  (`ErrDeferIsOutbox`) porque o mecanismo é a outbox do `KRN-06`.
- [ ] **[P0] Todo decorator observável (`RES-23`)**: timeout emite
  `dmpf_dependency_deadline_exceeded_total`; retry emite
  `dmpf_dependency_retries_total`; breaker emite
  `dmpf_dependency_breaker_state`; bulkhead emite o evento de span
  `dmpf.bulkhead.saturated` e o contador
  `dmpf_dependency_bulkhead_rejections_total` (métrica adicional, declarada
  `local` em `Catalog()`, fora do catálogo obrigatório, nomeada por `MET-02`).

#### Avaliador de retry (`retry`)

- [ ] **[P0] `retry.Evaluate(in Input) Verdict` como função pura (`RES-27`)**:
  `Input{Err error; Classifier Classifier; Operation resilience.Operation;
  Attempt, MaxAttempts int; Budget *Budget; Remaining time.Duration;
  Backoff Backoff; Rand func() float64}`;
  `Verdict{Allowed bool, Denied Factor, Wait time.Duration}`; os quatro fatores
  são avaliados em ordem e o primeiro falso encerra, nomeado em `Denied`
  (`FactorAttempts`, `FactorRetryable`, `FactorIdempotent`, `FactorBudget`,
  `FactorDeadline`). Um teste de compilação instancia `Input` com todos os
  campos e chama `Evaluate` literalmente como no pseudocódigo.
  - Sub-item: fator 1 = `Classifier(err) == Retryable`; `Unknown` e `nil`
    classifier são `false` (`RES-29`).
  - Sub-item: fator 2 = `Operation.Idempotent || Operation.EffectAbsent(err)`;
    ausente é `false`.
  - Sub-item: fator 3 = `Budget.Remaining() >= estimativa_da_tentativa + Wait`;
    saldo acima de zero mas insuficiente é `false` (`RES-27`, fator 3).
  - Sub-item: fator 4 = `Remaining >= estimativa_da_tentativa + Wait`.
  - Edge case: `Attempt >= in.MaxAttempts` (copiado da `Sheet` pelo decorator)
    devolve `Denied = FactorAttempts` antes dos quatro fatores (`RES-33`).
  - Edge case (fronteira): `Budget.Remaining() == need` autoriza;
    `need − 1ns` nega com `FactorBudget`; o mesmo para `Remaining` e
    `FactorDeadline`.
- [ ] **[P0] `retry.Budget` por execução (`RES-30`, `RES-31`, `RES-36`)**:
  criado por `retry.WithBudget(ctx, opts...) context.Context` na borda; na
  primeira falha, se nenhum valor foi declarado, o orçamento é
  `prazo_remanescente / 2`; `Budget.Debit(d)` é chamado pela espera de backoff
  **e** pela duração da tentativa repetida; `Budget.Exhausted()` incrementa
  `dmpf_dependency_budget_exhausted_total` uma vez e marca o span com
  `dmpf.retry.budget_exhausted = true`.
  - Sub-item: `retry.BudgetFrom(ctx) (*Budget, bool)`; `false` ⇒ fator 3 falso.
  - Edge case: `Budget` é seguro para uso concorrente por várias dependências da
    mesma execução (`sync/atomic`).
- [ ] **[P0] Backoff exponencial com jitter (`RES-32`)**: base 100 ms, fator 2,
  jitter total (`rand.Float64() * intervalo`) sobre semente injetada, teto de
  5 s por espera; `Sleeper func(ctx, d) error` injetado; a espera é debitada
  do `Budget` **antes** de dormir.
- [ ] **[P0] Teto de tentativas (`RES-33`)**: `MaxAttempts` default 3 (síncrono,
  contando a original) e 5 (assíncrono, `Sheet.Path = Async`); o valor está na
  `Sheet` e qualquer alteração é `Override`.
- [ ] **[P0] Retry nunca atravessa commit nem envolve a UoW (`RES-25`, `RES-34`)**:
  `Call` recusa em construção (`ErrWrapsUnitOfWork`) quando `Operation.Kind ==
  UnitOfWork`; o teste de integração prova que `memory.Store.WithinCalls()`
  permanece `1` sob falha e retry de uma dependência interna.

#### Métricas (`metrics`)

- [ ] **[P0] Catálogo com nome, unidade e fórmula (`MET-02`, `MET-03`)**: cada
  métrica é uma constante tipada `Metric{Name, Unit, Kind, Formula}` e um
  construtor sobre `metric.Meter`; `metrics.Catalog()` devolve as dez séries
  obrigatórias (mais a local do bulkhead) para documentação e teste. Nomes
  exatos:
  `dmpf_dependency_retries_total` (labels `dependency`, `error_category`),
  `dmpf_dependency_budget_exhausted_total` (`dependency`),
  `dmpf_dependency_breaker_state` (gauge, `dependency`; 0 fechado, 1
  meio-aberto, 2 aberto),
  `dmpf_dependency_deadline_exceeded_total` (`dependency`, `operation`),
  `dmpf_dependency_cancellations_total` (`dependency`, `operation`),
  `dmpf_service_request_duration_seconds` (histograma; `service`, `operation`,
  `outcome_category`), `dmpf_service_requests_total` (`service`, `operation`,
  `outcome_category`), `dmpf_service_errors_total` (`service`, `operation`,
  `error_category`), `dmpf_service_degraded_total` e
  `dmpf_service_omitted_total` (`dependency`).
- [ ] **[P0] Labels por allowlist (`MET-04`, `MET-07`)**: `metrics.Labels` é um
  builder com métodos por chave permitida (`Dependency()`, `Operation()`,
  `Service()`, `ErrorCategory()`, `OutcomeCategory()`); não existe método
  genérico `Set(key, value)`; um teste de reflexão garante que nenhuma chave
  contém `id`, `message`, `correlation`, `request`, `user`, `tenant`.
- [ ] **[P1] Limiar condicional (`MET-05`)**: `Metric.Threshold` é `nil` para
  contadores sem invariante e preenchido só para
  `dmpf_dependency_breaker_state` (aberto por mais de um cooldown) — o único
  derivado de default desta baseline nesta entrega; os demais declaram
  `Owner: "serviço"`.

#### Tracing (`tracing`)

- [ ] **[P0] Atributos por allowlist (`TRC-04`, `TRC-06`, `TRC-15`)**:
  `tracing.Attributes` é builder fechado com `CorrelationID()`, `RequestID()`,
  `TenantID()`, `Service()`, `Version()`, `OutcomeCategory()`,
  `TrafficClass()`, `Dependency()`, `Operation()`, `Attempt()`; nenhum método
  aceita `error`, payload ou tipo de domínio.
  - Edge case: `TenantID("")` **não** emite o atributo (ausência é informação,
    `CTX-26`).
- [ ] **[P0] Tentativa observável (`TRC-11`)**: cada tentativa repetida é um
  evento `dmpf.retry.attempt` no span da operação com `dmpf.retry.attempt`
  (número) e `dmpf.retry.previous_category`.
- [ ] **[P0] Erro por status e categoria (`TRC-12`)**: `tracing.RecordError(span, category)`
  define `codes.Error` e o atributo `dmpf.error.category`; **não** chama
  `span.RecordError(err)` com a mensagem.

#### Logging e auditoria (`logging`, `audit`)

- [ ] **[P0] Handler JSON com campos obrigatórios (`LOG-01`, `LOG-02`, `LOG-04`)**:
  `logging.NewHandler(w, Config) slog.Handler` envolve `slog.NewJSONHandler` e
  injeta `trace_id` e `span_id` do `SpanContext` ativo no `ctx`, `service`,
  `version` e `instance` da `Config`, e `correlation_id`, `request_id` e
  `tenant_id` por `Config.Fields func(ctx context.Context) Fields` — um
  extractor **injetado pela borda** (`trace.Span` não expõe atributos já
  gravados, `trace@v1.46.0/span.go:43-73`, e o kernel não admite outro valor de
  `context.Context` além do `Budget`). O autor do registro não os passa.
  - Edge case: `Fields == nil` ou `ctx` sem dados → os três campos são
    registrados como ausentes (`""`), nunca inventados; um `Fields` que devolva
    chave fora de {`correlation_id`, `request_id`, `tenant_id`} é ignorado com
    aviso único em `warn`.
- [ ] **[P0] Redaction na origem (`LOG-06`, `LOG-07`, `DAT-22`, `DAT-23`)**:
  `redact.Attr(key, value)` só aceita chaves da allowlist do serviço
  (`Config.AllowedFields`); chave fora dela produz `key=<redacted>` com o campo
  presente e o valor substituído; `redact.Error(err) slog.Attr` emite só
  `error_category` e `error_code`, nunca `err.Error()`.
- [ ] **[P1] Severidade e amostragem (`LOG-10`, `LOG-12`)**: o handler aplica
  as mesmas classes de `TRC-13`; `error` nunca é amostrado.
- [ ] **[P0] Canal de auditoria separado (`LOG-13`, `LOG-14`, `DAT-25`)**:
  `audit.Sink` é interface própria (`Emit(ctx, Event) error`);
  `audit.Event{Subject, Object, Action, Outcome string; At dmpfports.Instant}`;
  `audit.NewJSONSink(w)` e `audit.Recording` (para testes); nenhum construtor
  aceita `slog.Handler`, e o `Sink` ignora `Config.Sampling`.

#### Gancho do application service (`dmpf-application` + `usecase`)

- [ ] **[P0] `dmpfapplication.Instrumentation` (novo, bloco `application`)**:
  ```go
  type Instrumentation interface {
      BeginOperation(ctx context.Context, operation string) (context.Context, EndOperation)
      Audit(ctx context.Context, event AuditEvent)
  }
  type EndOperation func(result Result)
  type Result struct { Outcome OutcomeCategory; Err error } // Err só em Failed
  type OutcomeCategory string // Accepted | Rejected | Denied | Failed
  type AuditEvent struct { Object, Action string; Outcome OutcomeCategory; At dmpfports.Instant }
  var ErrDenied = errors.New("dmpfapplication: authorization denied")
  ```
  com `NoInstrumentation()` como realização vazia. Só `context`, `errors` e
  `dmpfports` são importados; o package permanece `pure`. `Result.Err` carrega
  o erro técnico cru para que o **provider** o classifique (`error_category`
  de MET-10) com um `Classifier` injetado — a taxonomia é de FND-07 e não
  entra no bloco `application`. `ErrDenied` é o único sentinela: um
  `AuthorizeFunc` que negue **declara** a negação com `errors.Is(err,
  ErrDenied)`; qualquer outro erro do autorizador é falha técnica (`Failed`),
  nunca `Denied` por suposição.
- [ ] **[P0] `ordersapp.Service` instrumentado (`TRC-16`, `LOG-14`, `MET-08`
  a `MET-10`)**: campo `Instrumentation dmpfapplication.Instrumentation` (nil ⇒
  `NoInstrumentation`); `AddItem` e `PlaceOrder` chamam `BeginOperation` ANTES
  do passo 1 (autorização) e `EndOperation` DEPOIS do passo 9 com a categoria
  do desfecho; `Audit` é emitido após o commit, com `Object = OrderID`,
  `Action = "orders.AddItem" | "orders.PlaceOrder"`, `Outcome` e
  `At = identity.OccurredAt`.
  - Edge case: autorização negada (`errors.Is(err, ErrDenied)`) emite
    `EndOperation(Result{Denied, nil})` e **não** emite `Audit` (nada foi
    acessado). Qualquer outro erro do autorizador, e toda falha técnica,
    emite `EndOperation(Result{Failed, err})`. Vetores distintos para os dois.
  - Edge case: `FindOrder` chama `BeginOperation`/`EndOperation` com classe
    `leitura` e **não** emite `Audit`.
- [ ] **[P0] `usecase.Instrumentation` (provider)**: realiza a interface
  estruturalmente — abre o span `dmpf.usecase.<operation>` com
  `dmpf.traffic_class = "write"` (ou `"read"` para operações declaradas de
  leitura), registra `dmpf_service_request_duration_seconds`,
  `dmpf_service_requests_total` e, em `Failed`, `dmpf_service_errors_total`
  com `error_category` resolvida por `Classifier func(error) string` injetado
  (`nil` ou desconhecido → `"unclassified"`, nunca a mensagem do erro);
  `Audit` encaminha ao `audit.Sink` com `Subject` resolvido por `SubjectFunc`
  injetado. NÃO importa `dmpf-application`; o teste de integração em
  `usecase/orders_integration_test.go` importa `ordersapp` e `memory` e faz o
  papel do composition root. No walking skeleton, `usecase` nasce com span +
  auditoria sobre `sdktrace.TracerProvider` + `tracetest.SpanRecorder`
  diretos; as métricas de serviço e o `otelboot.Runtime` entram depois.

#### Documentação

- [ ] **[P0] `README.md` do módulo**: o que cada package realiza, a tabela
  regra → código, os defaults da plataforma e a declaração de at-least-once.
- [ ] **[P0] ADR-035**: exportador OTLP/gRPC e destino em OpenTelemetry
  Collector; pin `v1.46.0` e `semconv/v1.43.0`; `Budget` como único valor de
  contexto; regra equivalente de `TRC-14` em processo. Índice em
  `docs/adr/README.md`.
- [ ] **[P0] `AGENTS.md`**: inventário passa a "Seis, todos Go" com o item
  `dmpf-observability-go`; `README.md` de `dmpf-application` ganha a seção do
  gancho `Instrumentation`.

### Não-funcionais

- [ ] Cadeia Go verde em cada módulo tocado: `gofmt -l` vazio, `go vet`,
  `golangci-lint v2.13.2`, `go build`, `go test -count=2 -shuffle=on`,
  `go test -race`, `govulncheck v1.7.0` (sem CVE aberta nos módulos OTel).
- [ ] `bash tools/dmpf-gate-check.sh` e
  `go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --base develop`
  aprovam: nenhum `DMPF-D001`, `DMPF-D002`, `DMPF-E001` ou `DMPF-E002`.
- [ ] `pnpm biome ci .` e `pnpm nx affected -t lint,typecheck,test,build --exclude=@nx-base-template/source` verdes.
- [ ] Determinismo: nenhum teste usa `time.Now()`, `time.Sleep()` nem `math/rand`
  sem semente; a suíte completa do módulo roda em menos de 5 s de tempo de
  parede.
- [ ] Segurança: nenhum atributo, label ou registro de log carrega `err.Error()`
  cru, payload, `message_id`, `correlation_id`, identificador de usuário ou
  segredo — provado por teste que injeta um erro com texto sensível e inspeciona
  os três pipelines em memória.
- [ ] Compatibilidade: Go **`1.26.8`** — piso e toolchain elevados por esta
  spec (errata de 2026-09-04: no piso `1.26.4`, `govulncheck v1.7.0` reprova o
  fechamento dos exportadores OTLP/gRPC com GO-2026-6090, GO-2026-5856 e
  GO-2026-5972, corrigidas em `1.26.5`/`1.26.6`; `1.26.8` é o estável corrente).
  O bump é a Fase 0 da entrega, em commit `chore(workspace)` próprio: `go.work`,
  os cinco `go.mod` existentes, `AGENTS.md`, nota no ADR-030 e
  `docs/nx-reference/tasks.md`. OTel `v1.46.0` exige `go 1.25.0` — satisfeito.

## Camadas afetadas

| Camada | Impacto |
| --- | --- |
| **Módulo Go `dmpf-observability`** (novo) | Bloco `provider`: bootstrap OTel, decorators, avaliador de retry, catálogo de métricas, tracing, logging, auditoria, redaction e a realização do gancho |
| **Módulo Go `dmpf-application`** | Gancho puro `Instrumentation`, `OutcomeCategory`, `AuditEvent`, `NoInstrumentation` no raiz; `ordersapp.Service` ganha o campo e as chamadas nos passos 1 e 9; nenhuma mudança de manifesto ou baseline |
| **Módulos Go `dmpf-domain`, `dmpf-ports`, `dmpf-contracts`, `dmpf-conformance`** | Só a diretiva `go 1.26.8` no `go.mod` (Fase 0); nenhum símbolo, teste ou manifesto tocado |
| **Workspace Go (`go.work`, `go.work.sum`)** | Diretiva `go 1.26.8` (Fase 0); uma entrada nova em `use`; `go.work.sum` regravado |
| **Governança (manifesto + baseline)** | Um manifesto novo com uma unidade e sete entradas `external`; baseline regravado; commit próprio |
| **Lint** | `.golangci.yml` **sem mudança**: nenhum glob casa `*-observability/`; `tools/dmpf-gate-check.sh` **sem mudança**: módulo só-`provider` é pulado (linha 165) |
| **CI** | Nenhuma mudança em `ci.yml`: `Go gates (affected)` e `DMPF conformance gate` já cobrem projetos `stack:go` afetados |
| **Nx** | Um `project.json` novo com tags e cinco targets; nenhuma mudança em `nx.json` |
| **Documentação** | `README.md` do módulo, seção nova no `README.md` de `dmpf-application`, ADR-035, índice de ADRs, `AGENTS.md` |

Nenhuma camada de produto é afetada: não há app, endpoint, banco, fila nem
broker nesta entrega. A borda que cria o `Budget` e o composition root que liga
o gancho são desempenhados pela suíte de integração; o app real é do `KRN-08` e
do `KRN-12`.

## Localização de código

```text
lidercap-platform/
├── go.work                                              # MODIFICAR — use ./libs/backend/go/dmpf-observability
├── go.work.sum                                          # MODIFICAR — somas dos módulos OTel
├── go.work                                              # MODIFICAR (Fase 0) — go 1.26.8
├── libs/backend/go/{dmpf-domain,dmpf-conformance,dmpf-contracts,dmpf-ports,dmpf-application}/go.mod  # MODIFICAR (Fase 0) — go 1.26.8
├── docs/adr/030-granularidade-modulo-go-e-bom.md        # MODIFICAR (Fase 0) — nota: pin do toolchain passa a 1.26.8
├── docs/nx-reference/tasks.md                           # MODIFICAR (Fase 0) — go mod edit -go=1.26.8
├── libs/backend/go/dmpf-observability/                  # CRIAR — projeto Nx dmpf-observability-go, bloco provider
│   ├── go.mod                                           # module .../dmpf-observability, go 1.26.8, require otel* v1.46.0 (+ testcontainers-go v0.44.0 e moby/go-archive v0.3.3 para testes)
│   ├── go.sum                                           # CRIAR — commitado
│   ├── package.json                                     # @lidercap-apps/dmpf-observability-go, private: true
│   ├── project.json                                     # tags 3D, 5 targets
│   ├── dmpf-units.json                                  # unidade dmpf-kernel/observability + external[] (commit próprio)
│   ├── README.md                                        # regra → código; defaults; at-least-once
│   ├── doc.go                                           # package dmpfobservability
│   ├── version.go                                       # OTelVersion, SemconvVersion
│   ├── version_test.go                                  # bate com debug.ReadBuildInfo e com o import de semconv
│   ├── otelboot/                                        # bootstrap
│   │   ├── config.go                                    # Config, Sampling, ErrPropagatorRequired, ErrPropagatorNotW3C, ErrResourceIncomplete
│   │   ├── start.go                                     # Start, Runtime, Shutdown, ErrAlreadyStarted
│   │   ├── sampler.go                                   # classSampler próprio: RecordAndSample | RecordOnly, nunca Drop (TRC-13)
│   │   ├── processor.go                                 # classAwareProcessor: único dono do exporter; sampled || codes.Error (TRC-14)
│   │   ├── otlp/exporters.go                            # construtores OTLP/gRPC (ADR-035)
│   │   ├── start_test.go                                # propagador obrigatório, recurso incompleto, duplo Start
│   │   ├── sampler_test.go                              # taxas por classe, unclassified
│   │   └── processor_test.go                            # RecordOnly com erro chega ao exportador; fila cheia; ForceFlush; Shutdown ordenado
│   ├── clock/                                           # tempo do provider (dmpf-ports intocado)
│   │   ├── clock.go                                     # Clock{Now, After, NewTimer, WithTimeout}, System()
│   │   ├── fake.go                                      # Fake avançável e determinista para testes
│   │   └── fake_test.go                                 # Advance desbloqueia After/timers na ordem
│   ├── resilience/                                      # decorators
│   │   ├── sheet.go                                     # Sheet, NotApplicable, Override, Defaults, Validate, Effective
│   │   ├── operation.go                                 # Operation, Kind, Path
│   │   ├── compose.go                                   # Compose, ComposeWithOrder, Slots, Call, ordem canônica
│   │   ├── timeout.go                                   # prazo derivado (RES-06)
│   │   ├── route.go                                     # ValidateRoute(remaining, hops): ErrDeadlineComposition (RES-07)
│   │   ├── route_test.go                                # soma dos hops + folga excede → reprova; igual → passa
│   │   ├── breaker.go                                   # estados, janela, piso de amostras, ErrBreakerOpen
│   │   ├── bulkhead.go                                  # semáforo, fila, aquisição, ErrBulkheadSaturated
│   │   ├── degrade.go                                   # Fail/Degrade/Defer/Ignore, DegradedResult, ErrDeferIsOutbox
│   │   ├── retry_decorator.go                           # laço de tentativas sobre retry.Evaluate; ErrWrapsUnitOfWork
│   │   ├── errors.go                                    # categorias próprias (breaker aberto, saturação, prazo, cancelamento)
│   │   ├── sheet_test.go                                # campo em branco reprova; override registrado e exposto
│   │   ├── compose_test.go                              # ordem observada por sondas; ordem alternativa exige motivo
│   │   ├── timeout_test.go                              # mínimo dos três; composição que excede reprova
│   │   ├── breaker_test.go                              # 3 chamadas/1 falha fecha; 20+/50% abre; sonda; cooldown
│   │   ├── bulkhead_test.go                             # saturação rápida em 100 ms com relógio fake
│   │   └── degrade_test.go                              # Degrade distinguível; Ignore conta omissão; Defer reprova
│   ├── retry/                                           # avaliador
│   │   ├── classifier.go                                # Retryability{Retryable, NotRetryable, Unknown}, Classifier
│   │   ├── evaluate.go                                  # Input, Verdict, Factor, Evaluate (função pura)
│   │   ├── budget.go                                    # Budget, WithBudget, BudgetFrom, Debit, Remaining, Exhausted
│   │   ├── backoff.go                                   # Backoff{Base, Factor, Cap, Jitter}, Next(attempt, rand)
│   │   ├── evaluate_test.go                             # tabela: cada fator falso nomeado; Unknown é falso; saldo insuficiente
│   │   ├── budget_test.go                               # metade do remanescente; debit por backoff; exaustão única; concorrência
│   │   └── backoff_test.go                              # exponencial, jitter total, teto, semente
│   ├── metrics/
│   │   ├── catalog.go                                   # Metric{Name, Unit, Kind, Formula, Threshold, Owner}, Catalog()
│   │   ├── labels.go                                    # Labels builder fechado
│   │   ├── instruments.go                               # construtores sobre metric.Meter
│   │   ├── catalog_test.go                              # nomes em MET-02; fórmula não vazia; dez séries + 1 local
│   │   └── labels_test.go                               # reflexão: nenhuma chave proibida
│   ├── tracing/
│   │   ├── attributes.go                                # Attributes builder fechado (TRC-04, TRC-15)
│   │   ├── record.go                                    # RecordError por categoria (TRC-12); AttemptEvent (TRC-11)
│   │   ├── attributes_test.go                           # TenantID vazio omite; nenhuma chave proibida
│   │   └── record_test.go                               # status Error sem mensagem
│   ├── logging/
│   │   ├── handler.go                                   # NewHandler: JSON + campos obrigatórios do ctx (LOG-02, LOG-04)
│   │   ├── sampling.go                                  # classes alinhadas a TRC-13; error nunca amostrado
│   │   ├── handler_test.go                              # campos injetados; tenant ausente registrado como ausente
│   │   └── sampling_test.go                             # error 100%
│   ├── redact/
│   │   ├── redact.go                                    # Attr por allowlist; Error → categoria + código
│   │   └── redact_test.go                               # texto sensível não atravessa log, trace nem métrica
│   ├── audit/
│   │   ├── sink.go                                      # Sink, Event, NewJSONSink, Recording
│   │   └── sink_test.go                                 # canal separado; sem amostragem; campos obrigatórios
│   └── usecase/                                         # realização do gancho de dmpfapplication
│       ├── instrumentation.go                           # Instrumentation, New, SubjectFunc, BeginOperation, Audit
│       ├── instrumentation_test.go                      # satisfaz a interface estruturalmente (var _ = ...)
│       └── orders_integration_test.go                   # composition root: ordersapp + memory + Budget + OTel em memória
├── libs/backend/go/dmpf-application/
│   ├── instrumentation.go                               # CRIAR — Instrumentation, EndOperation, OutcomeCategory, AuditEvent, NoInstrumentation
│   ├── instrumentation_test.go                          # CRIAR — NoInstrumentation é inerte
│   ├── README.md                                        # MODIFICAR — seção "Instrumentação do caso de uso"
│   └── example/orders/
│       ├── service.go                                   # MODIFICAR — campo Instrumentation; helper instrumentation()
│       ├── add_item.go                                  # MODIFICAR — BeginOperation antes do passo 1, End após o 9, Audit após commit
│       ├── place_order.go                               # MODIFICAR — idem
│       ├── find_order.go                                # MODIFICAR — Begin/End sem Audit
│       └── instrumentation_test.go                      # CRIAR — ordem Begin → passos → End; Denied sem Audit; Failed
├── tools/dmpf-baseline/units-baseline.json              # MODIFICAR — +1 entrada, digest (commit próprio)
├── docs/adr/035-observabilidade-otel-e-retry-por-conjuncao-em-go.md  # CRIAR — Etapa 6
├── docs/adr/README.md                                   # MODIFICAR — linha do ADR-035
├── AGENTS.md                                            # MODIFICAR — inventário de libs (seis)
├── .golangci.yml                                        # INTOCADO — nenhum glob casa o módulo
├── tools/dmpf-gate-check.sh                             # INTOCADO — módulo só-provider é pulado
├── libs/backend/go/dmpf-domain/                         # INTOCADO
├── libs/backend/go/dmpf-ports/                          # INTOCADO
├── libs/backend/go/dmpf-contracts/                      # INTOCADO
├── libs/backend/go/dmpf-conformance/                    # INTOCADO — instrumento, não objeto
└── docs/dmpf/                                           # INTOCADO — acervo normativo
```

Import path canônico da unidade nova (a `canonical_key`):

- `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability`
  e cada subpackage de produção listado acima em `include`.

## Design

### Arquitetura

```text
                       app (composition root / suíte de integração)
        ┌────────────────────────────────────────────────────────────────┐
        │ ctx = retry.WithBudget(ctx)          rt, _ := otelboot.Start() │
        │ svc := ordersapp.Service{…, Instrumentation: usecase.New(rt…)} │
        └───────────────┬───────────────────────────────┬────────────────┘
                        │ chama                          │ liga (estrutural)
   application ┌────────▼─────────────────┐   provider  ┌▼───────────────────────────┐
   (pure +     │ ordersapp.Service        │   (permis-  │ dmpf-observability         │
   observab.)  │  Begin ─ passos 1..9 ─ End│   sivo)     │  usecase.Instrumentation   │
               │  Audit após commit        │             │  otelboot · resilience     │
               │ dmpfapplication.          │             │  retry · metrics · tracing │
               │  Instrumentation (iface)  │             │  logging · audit · redact  │
               └────────┬──────────────────┘             └──────────┬─────────────────┘
                        │ usa (permitida)                           │ usa (permitida)
   port        ┌────────▼──────────┐                     ┌──────────▼──────────┐
   (pure)      │ dmpfports          │◄────────────────────┤ dmpfports.Clock      │
               │ UnitOfWork, Outbox │  provider → port    │ (relógio injetado)   │
               └────────┬───────────┘                     └─────────────────────┘
                        │
   domain      ┌────────▼──────────┐
   (pure)      │ dmpfdomain, orders │      ✗ nenhuma seta chega aqui vinda do provider
               └────────────────────┘

   Setas proibidas e ausentes: application → provider, provider → application.
```

A ordem canônica de `RES-22` é a ordem em que `Compose` aninha as funções, de
fora para dentro:

```text
tracing → metrics → logging → bulkhead → breaker → [rate limit] → retry → timeout → chamada
```

O retry fica **dentro** de bulkhead e breaker (uma tentativa repetida não
adquire recurso novo nem fura o breaker aberto) e **fora** do timeout (cada
tentativa tem prazo próprio, derivado por `RES-06`). Métricas ficam fora do
retry: a latência medida inclui a espera de backoff, que é o número que o
chamador percebeu.

### Fluxo principal — uma chamada de provider sob falha transitória

1. A borda cria o contexto com prazo e `Budget` (`retry.WithBudget`), abre o
   span de borda e invoca o application service.
2. O application service chama `Instrumentation.BeginOperation(ctx, "orders.AddItem")`
   — o provider abre o span `dmpf.usecase.orders.AddItem` com
   `dmpf.traffic_class = "write"` e devolve o `ctx` filho (`TRC-16`).
3. Os passos 1 a 5 de FND-04 §3.2 correm como no `KRN-04`; no passo 6 a
   realização da UoW (um provider, no `KRN-06` o Postgres) chama sua
   dependência através de `Call` composto por `Compose(sheet, slots)`.
4. `Call` entra em `tracing` (span filho `dmpf.dependency.<dep>.<method>`),
   `metrics` (inicia o histograma), `logging`, adquire o `bulkhead` (ou
   rejeita em 100 ms), consulta o `breaker` (aberto ⇒ `ErrBreakerOpen`,
   categoria própria, sem tentativa).
5. `retry` executa a tentativa 1 dentro de `timeout` com prazo
   `min(op.Deadline, remanescente, orçamento − 0)`. A chamada falha com erro
   que o `Classifier` marca `Retryable`.
6. `retry.Evaluate` recebe `Attempt = 1`, calcula `Wait = Backoff.Next(1)` e
   verifica os quatro fatores: retentável ✓, idempotente ✓ (a `Sheet` declara o
   método), orçamento ✓ (`Budget` inicializa com metade do remanescente na
   primeira falha e tem saldo para `Wait` + estimativa), prazo ✓. `Allowed`.
7. O decorator emite o evento `dmpf.retry.attempt{attempt=2, previous_category}`,
   incrementa `dmpf_dependency_retries_total`, **debita `Wait` do `Budget`** e
   dorme por `Sleeper`. Repete a tentativa com prazo recalculado.
8. A tentativa 2 conclui. `Call` devolve; `metrics` registra a duração total;
   `tracing` encerra o span da dependência sem erro.
9. Os passos 7 a 9 concluem; o application service chama
   `EndOperation(Accepted)` e `Audit(ctx, AuditEvent{Object, Action, Outcome, At})`;
   o provider registra `dmpf_service_request_duration_seconds` e
   `dmpf_service_requests_total`, encerra o span do caso de uso e encaminha o
   evento ao `audit.Sink`, fora do pipeline de log.

Variante — **esgotamento**: se em (6) o `Budget` não cobre `Wait` + estimativa,
`Verdict.Denied = FactorBudget`; o decorator marca
`dmpf.retry.budget_exhausted = true`, incrementa
`dmpf_dependency_budget_exhausted_total` e devolve o erro da última tentativa
com a sua categoria (`RES-36`). Nenhuma nova decisão é tomada.

Variante — **retryability não autoriza**: um método declarado
`Idempotent = false` com erro `Retryable` produz `Denied = FactorIdempotent` na
primeira avaliação; zero repetições (`RES-28`; ADR-026, exemplo 2).

### Pseudocódigo — `retry.Evaluate`

```go
func Evaluate(in Input) Verdict {
    if in.Attempt >= in.MaxAttempts {
        return Verdict{Denied: FactorAttempts}
    }
    wait := in.Backoff.Next(in.Attempt, in.Rand)
    need := wait + in.Operation.EstimatedDuration          // tentativa + sua espera

    if in.Classifier == nil || in.Classifier(in.Err) != Retryable {   // fator 1 — Unknown é falso (RES-29)
        return Verdict{Denied: FactorRetryable}
    }
    if !(in.Operation.Idempotent || in.Operation.EffectAbsent(in.Err)) { // fator 2
        return Verdict{Denied: FactorIdempotent}
    }
    if in.Budget == nil || in.Budget.Remaining() < need {              // fator 3 — saldo > 0 não basta
        return Verdict{Denied: FactorBudget}
    }
    if in.Remaining < need {                                           // fator 4
        return Verdict{Denied: FactorDeadline}
    }
    return Verdict{Allowed: true, Wait: wait}
}
```

A função não lê relógio, não dorme, não emite telemetria: recebe o remanescente
já medido e devolve a decisão. O decorator, que é quem tem `Clock`, `Sleeper` e
`Meter`, aplica o veredicto.

### Pseudocódigo — `Compose`

```go
func Compose(sheet Sheet, s Slots) (Call, error) {
    if err := sheet.Validate(); err != nil { return nil, err }        // RES-21: campo em branco reprova
    // RES-07 não cabe aqui: uma ficha só vê uma dependência. A soma dos hops é
    // verificada por ValidateRoute(remaining, hops) por quem conhece a rota.
    call := s.Invoke
    call = timeout(sheet, call)        // mais interno
    call = retry(sheet, call)
    if s.RateLimit != nil { call = s.RateLimit(call) } else { requireNotApplicable(sheet.RateLimit) }
    call = breaker(sheet, call)
    call = bulkhead(sheet, call)
    call = logging(call)
    call = metrics(call)
    call = tracing(call)               // mais externo
    return guardUnitOfWork(call), nil  // RES-25/RES-34: Operation.Kind == UnitOfWork ⇒ ErrWrapsUnitOfWork
}
```

### Onde cada regra é provada

| Regra | Teste |
| --- | --- |
| `RES-01`, `RES-24`, `TRC-16`, `LOG-11` | Verificador do `KRN-02` sobre o workspace (`DMPF-E001` ausente em `domain`/`port`); `usecase/orders_integration_test.go` prova que o span nasce no application service |
| `RES-21`, `RES-40` | `sheet_test.go`: campo zero reprova; override sem motivo reprova; `Effective()` reflete override e o bootstrap o expõe |
| `RES-22` | `compose_test.go`: sondas em cada posição registram a ordem de entrada e saída |
| `RES-05`, `RES-06`, `RES-07` | `timeout_test.go`: `op.Deadline == 0` reprova; prazo efetivo é o mínimo dos três; soma a jusante que excede reprova em `Compose` |
| `RES-10`, `RES-12` | `breaker_test.go`: 3 chamadas com 1 falha mantêm fechado; 20 chamadas com 50 % abrem; aberto devolve `ErrBreakerOpen` sem invocar `do` |
| `RES-13`, `RES-14` | `bulkhead_test.go`: com pool 2 e fila 2, a quinta chamada é rejeitada em 100 ms de relógio fake |
| `RES-25`, `RES-34` | `retry_decorator_test.go`: `Operation.Kind == UnitOfWork` reprova em `Compose`; integração: `WithinCalls() == 1` sob retry de dependência interna |
| `RES-27`, `RES-28`, `RES-29` | `evaluate_test.go` (tabela): cada fator falso isoladamente produz `Denied` com o seu nome; `Unknown` e `nil` classifier negam; retentável + não idempotente nega |
| `RES-30`, `RES-31`, `RES-36` | `budget_test.go`: primeira falha fixa metade do remanescente; `Debit` por backoff reduz o saldo; três dependências esgotam e a terceira não retenta; exaustão conta uma vez |
| `RES-32`, `RES-33` | `backoff_test.go`: 100 ms × 2ⁿ com jitter em `[0, intervalo)` e teto 5 s, por semente; `MaxAttempts` 3/5 |
| `RES-37`, `RES-38` | `degrade_test.go`: `Degrade` devolve `DegradedResult` e conta; `Ignore` conta omissão; `Defer` reprova |
| `TRC-09` | `start_test.go`: `Propagator == nil` ⇒ `ErrPropagatorRequired`; propagador não-W3C ⇒ `ErrPropagatorNotW3C` |
| `TRC-03` | `version_test.go`: `SemconvVersion` bate com o único import `semconv/vX.Y.Z` do módulo |
| `TRC-13`, `TRC-14` | `sampler_test.go`: matriz root / pai local / pai remoto × amostrado / não amostrado com `TraceID`s construídos — nunca `Drop`; `processor_test.go`: span `RecordOnly` com `codes.Error` chega ao `InMemoryExporter`, span `RecordOnly` sem erro não chega, fila cheia, `ForceFlush`, `Shutdown` ordenado |
| `TRC-04`, `TRC-06`, `TRC-15`, `MET-04`, `MET-07` | `attributes_test.go`, `labels_test.go`: reflexão sobre os builders — nenhuma chave proibida, nenhum método aceita `error` |
| `TRC-11`, `TRC-12` | `record_test.go`: evento de tentativa com número e categoria; erro sem mensagem |
| `MET-02`, `MET-03`, `MET-05` | `catalog_test.go`: dez nomes conformes ao padrão (mais a local do bulkhead), fórmula não vazia, limiar só no breaker |
| `MET-08`, `MET-09`, `MET-10`, `MET-13` | `orders_integration_test.go` com `ManualReader`: histograma e contadores por operação e categoria após `AddItem` aceito, rejeitado e falho |
| `MET-28`, `MET-29` | `retry_decorator_test.go`, `timeout_test.go`: contadores incrementados nos caminhos de retry, exaustão, prazo excedido e cancelamento |
| `LOG-01`, `LOG-02`, `LOG-04`, `LOG-10`, `LOG-12` | `handler_test.go`, `sampling_test.go`: registro JSON com os campos do contexto; `tenant_id` ausente registrado como ausente; `error` 100 % |
| `LOG-06`, `LOG-07`, `DAT-22`, `DAT-23` | `redact_test.go`: erro com texto sensível — o texto não aparece em log, span nem label; o campo aparece redigido |
| `LOG-13`, `LOG-14`, `DAT-25` | `sink_test.go` e integração: evento após commit com os cinco campos; nenhum evento sob `Denied`; `Sink` recebe 100 % sob amostragem de `leitura` |
| Matriz e capabilities | Verificador do `KRN-02` no CI; `instrumentation_test.go` em `usecase` prova a satisfação estrutural sem import de `dmpf-application` em produção (`go list -deps` do package de produção não contém `dmpf-application`) |

## Decisões técnicas

- **Gancho puro em `dmpfapplication`, realização no provider, vínculo no
  composition root**: a matriz proíbe as duas setas entre `application` e
  `provider`, e a `allow` estrita do `.golangci.yml` para `application` não
  inclui OTel. A interface com `context` e tipos próprios mantém
  `dmpf-application` `pure`; Go a satisfaz estruturalmente sem import.
  Alternativa descartada: abrir `go.opentelemetry.io/otel` na `allow` de
  `application` e declarar `external` no seu manifesto — permitido pela
  política (`observability` passa em `application`), mas faria o caso de uso
  conhecer o SDK e travaria a troca de backend; e continuaria sem resolver o
  orçamento, que precisa de `time`.
- **`Budget` como único valor de `context.Context`, criado na borda**: a porta
  não pode carregar tipo de política (`RES-24`), o application service não pode
  importar o provider, e o orçamento precisa atravessar todas as dependências
  da execução (`RES-30`). O valor de contexto **só restringe** — ausente,
  nenhum retry ocorre —, o que preserva `CTX-03`/`CTX-05`: nunca é fonte de
  correção do resultado. Alternativa descartada: `Budget` em `dmpfports` —
  viola `RES-24` pelo caminho mais discreto. Alternativa descartada: orçamento
  por chamada — é exatamente o efeito multiplicativo que `RES-30` proíbe.
  Registrado no ADR-035.
- **Avaliador de retry escrito aqui, como função pura**: nenhuma biblioteca Go
  de retry decide por conjunção de quatro fatores nem conta orçamento por
  execução com débito de backoff. `Evaluate` sem relógio, sem sono e sem
  telemetria é testável por tabela e reusável pelo `KRN-10`. Alternativa
  descartada: `cenkalti/backoff` ou `failsafe-go` — trariam retry por
  retryability isolada, que `RES-28` proíbe.
- **`Classifier` injetado, `Unknown` é falso**: a taxonomia de FND-07 não tem
  realização Go e não é desta história. O predicado é entrada do avaliador;
  ausência resolve para não retentável (`ERR-11`, `RES-29`). Alternativa
  descartada: realizar a taxonomia aqui — invade FND-07 e o `KRN-10`.
- **Regra equivalente de `TRC-14` em processo: sampler próprio + processor
  único**: o SDK decide amostragem no início do span, e os samplers de fábrica
  (`ParentBased`, `TraceIDRatioBased`) devolvem `Drop` no ramo negativo
  (`sampling.go:71-116,176-203`) — um `Drop` não é gravado e nunca chega a
  processor algum. Por isso o `classSampler` é próprio e devolve `RecordOnly`
  fora da taxa, e o `classAwareProcessor` é o **único** processor e dono do
  exporter: exporta `sampled || codes.Error` de forma assíncrona. A garantia
  declarada é "span em erro sempre exportado"; o trace completo de erro é do
  tail sampling no Collector (ADR-035). Alternativas descartadas: dois
  processors sobre o mesmo exporter — `ExportSpans` é síncrono e sem garantia
  de concorrência (`span_exporter.go:16-19`) e `OnEnd` não pode bloquear
  (`span_processor.go:25-27`); `AlwaysSample` em processo e filtrar no
  collector — desloca o custo de rede para 100 % dos traces; buffer por
  `TraceID` até o root encerrar — memória ilimitada sob trace longo.
- **Exportador OTLP/gRPC e destino em OpenTelemetry Collector (ADR-035)**:
  OTLP é o formato da própria convenção e evita o fallback proprietário que
  `TRC-09` veda; o collector concentra tail sampling, retenção e o segundo
  destino da auditoria. O endpoint entra por `Config`, nunca por literal.
  Alternativa descartada: exportador de vendor — acopla ao backend cuja
  escolha FND-08 §1.4 `encaminhou`; stdout — só desenvolvimento, disponível
  pela injeção mas não é o default.
- **Pin `v1.46.0` em todos os módulos OTel e `semconv/v1.43.0`**: a família
  OTel Go versiona os módulos em lockstep e o SDK reprova mistura; `v1.46.0`
  é a mais recente estável em 2026-09-04 e empacota `semconv/v1.43.0`. A
  constante `SemconvVersion` é o "pin declarado" de `TRC-03` até o BOM do
  `KRN-12` existir. Alternativa descartada: importar `semconv` sem constante —
  o `KRN-12` teria de inferir a versão do código.
- **Tempo por `clock.Clock` do provider (`Now`, `After`, `NewTimer`,
  `WithTimeout`) e `Sleeper` injetado**: o provider pode usar `time`, mas o
  teste determinista não pode; e a porta `dmpfports.Clock` só expõe `Now()`
  (`dmpf-ports/clock.go:5-10`) — um relógio que só lê o instante não
  desbloqueia o `select` do bulkhead nem substitui `context.WithTimeout` do
  timeout. O package `clock` traz `System()` e um `Fake` avançável; a porta do
  kernel segue intocada. `Sleeper func(ctx, d) error` isola a espera do
  backoff. Alternativas descartadas: estender `dmpfports.Clock` — viola
  `RES-24` e toca módulo fora do escopo; `time.Now()` direto com testes
  tolerantes — viola o não-funcional de determinismo.
- **`Defer` reprovado nesta entrega**: o modo `difere` de `RES-37` é a outbox
  (`RES-37`, linha 3), mecanismo do `KRN-06`. Aceitá-lo aqui sem mecanismo
  seria promessa vazia. Alternativa descartada: implementar fila local — é
  reexecução disfarçada.
- **Métrica local `dmpf_dependency_bulkhead_rejections_total`**: `RES-23`
  exige que o bulkhead exponha saturação e espera, e o catálogo obrigatório
  cobre a espera por pool em `MET-30` (pool de conexões, `KRN-06`). Uma métrica
  adicional, nomeada por `MET-02` e declarada `local` em `Catalog()`, satisfaz
  `RES-23` sem redefinir `MET-30`.
- **Escopo M assumido com um risco de revisão**: o ticket sugere tamanho M;
  a entrega tem oito packages e uma edição cirúrgica no `KRN-04`. O planner
  decompõe em fases (módulo e governança; retry e decorators; telemetria;
  gancho e integração; documentação) e a revisão pode devolver a divergência de
  tamanho ao refinamento sem alterar o escopo normativo.

## Regras relacionadas

- `AGENTS.md` — "Convenções obrigatórias" (tags 3D, não redeclarar targets),
  "Tooling" (cadeia Go, política de dependências) e "Git e release"
  (git-flow, PR para `develop`).
- `docs/dmpf/resiliencia-observabilidade.md` — FND-08, artefato de origem.
- `docs/dmpf/contexto-erros-seguranca.md` — FND-07, `ERR`, `CTX`, `DAT`.
- `docs/adr/026-*.md`, `docs/adr/015-*.md`, `docs/adr/010-*.md`,
  `docs/adr/014-*.md`, `docs/adr/030-*.md`, `docs/adr/031-*.md`,
  `docs/adr/034-*.md`.
- `contracts/README.md` — precedente de `external[]` com capability declarada
  (`wire.codec`) e de `go.sum` commitado em módulo do kernel.
- SPEC-YRJRADY9 — guarda-chuva; SPEC-ZHE7DN1H — o que se instrumenta;
  SPEC-WTAXFV8B — o que verifica; SPEC-E15TBHCD — de onde a norma veio.

## Verificação e testes

### Critérios de aceite

- [ ] O fechamento de imports de toda unidade `domain` e `port` do workspace não
  alcança biblioteca de trace, log ou métrica: o verificador do `KRN-02` não
  emite `DMPF-E001` para essas unidades, e `go list -deps` de `dmpf-domain` e
  `dmpf-ports` não contém `go.opentelemetry.io` nem `log/slog`.
- [ ] Uma porta que declare política de retry na assinatura reprova: fixture em
  `testdata/` do verificador (`KRN-02`) já cobre `DMPF-E001` por capability;
  esta spec acrescenta o vetor de forma em `compose_test.go` — `Operation.Kind
  == UnitOfWork` reprova em `Compose` — e documenta no README que `dmpfports`
  segue sem parâmetro de política.
- [ ] Removida a configuração do propagador, `otelboot.Start` devolve
  `ErrPropagatorRequired` e `otel.GetTextMapPropagator()` permanece o valor
  anterior — nunca um default silencioso.
- [ ] Erro retentável em operação sem semântica idempotente comprovada não
  autoriza tentativa, e `Verdict.Denied == FactorIdempotent`.
- [ ] Execução que acessa três dependências, cada uma falhando, aborta a
  repetição quando o orçamento por execução se esgota: com prazo remanescente
  de 2 s na primeira falha (orçamento 1 s) e backoff base 100 ms, a terceira
  dependência recebe `Denied = FactorBudget` sem retentar, e
  `dmpf_dependency_budget_exhausted_total == 1`.
- [ ] A espera de backoff é debitada do orçamento, comprovado com relógio fake:
  após uma espera de 200 ms, `Budget.Remaining()` diminuiu exatamente 200 ms
  antes de a tentativa seguinte começar.
- [ ] Nenhuma métrica usa identificador de mensagem, correlação, requisição,
  agregado ou usuário como label (teste de reflexão sobre `metrics.Labels`), e
  trace de erro é sempre exportado (span `RecordOnly` com `codes.Error` chega
  ao `InMemoryExporter`).
- [ ] O span do caso de uso é aberto pelo application service (`TRC-16`):
  `orders_integration_test.go` observa `dmpf.usecase.orders.AddItem` como pai
  do span da dependência e nenhum span criado por `dmpf-domain` ou
  `dmpf-ports`.
- [ ] O evento de auditoria é emitido pelo application service após o commit,
  com sujeito, objeto, ação, desfecho e instante, e **não** passa pelo handler
  de log: `audit.Recording` recebe 1 evento em `Accepted` e `Rejected`, 0 em
  `Denied`, e o `slog` em memória não contém o evento.
- [ ] `WithinCalls() == 1` na realização em memória sob retry de uma
  dependência interna: o retry não reabre a UoW.
- [ ] Todo override de default está na `Sheet` com valor, motivo e data, e o
  valor efetivo aparece como atributo de recurso `dmpf.sheet.<dep>.<field>`.
- [ ] Um erro cujo `Error()` contém `"cpf=123.456.789-00"` não aparece em nenhum
  registro de log, atributo de span ou label de métrica dos três pipelines em
  memória; o campo aparece como `error_category` e `error_code`.
- [ ] `dmpfobservability.SemconvVersion == "1.43.0"` e o único import `semconv`
  do módulo é `go.opentelemetry.io/otel/semconv/v1.43.0`; todos os `require`
  `go.opentelemetry.io/otel*` são `v1.46.0`.
- [ ] Manifesto e baseline vêm em commit próprio, sem código Go; o verificador
  aprova a unidade nova e as sete entradas `external` com
  `capability: observability`.
- [ ] Nenhum artefato desta história declara ou sugere exactly-once fim a fim
  (`grep -ri "exactly.once"` só encontra negações).
- [ ] Testes unitários para `otelboot`, `resilience`, `retry`, `metrics`,
  `tracing`, `logging`, `redact`, `audit`, `usecase` e para o gancho em
  `dmpfapplication` e `ordersapp`.
- [ ] Validação do projeto passando: cadeia Go completa nos módulos tocados,
  `dmpf-gate-check.sh`, verificador de conformidade, `biome ci` e
  `nx affected` de lint, typecheck, test e build.

### Cenários de teste

```text
DADO uma Sheet com Defaults() e um Classifier que marca o erro como Retryable
     e uma Operation{Idempotent: true, Deadline: 500ms}
     e um ctx com deadline em +2s e retry.WithBudget(ctx)
     e uma dependência que falha na tentativa 1 e conclui na 2
QUANDO Call é invocado
ENTÃO do é chamado 2 vezes
  E dmpf_dependency_retries_total{dependency, error_category} == 1
  E o span da dependência tem 1 evento dmpf.retry.attempt{attempt=2}
  E Budget.Remaining() == 1s − (espera_de_backoff + duração_da_tentativa_2)
  E o histograma dmpf_service_request_duration_seconds inclui a espera

DADO a mesma Sheet e uma Operation{Idempotent: false}
     e um Classifier que marca o erro como Retryable
QUANDO Call é invocado e a tentativa 1 falha
ENTÃO do é chamado exatamente 1 vez
  E o erro devolvido é o da tentativa 1
  E Verdict.Denied == FactorIdempotent aparece como atributo dmpf.retry.denied
  E dmpf_dependency_retries_total não é incrementado

DADO um ctx SEM retry.WithBudget e um erro Retryable em Operation idempotente
QUANDO Call é invocado e a tentativa 1 falha
ENTÃO nenhuma repetição ocorre e Denied == FactorBudget

DADO três Operations sobre dependências distintas na mesma execução,
     cada uma com EstimatedDuration = 50ms e MaxAttempts = 3
     e ctx com 2s remanescentes na primeira falha (orçamento = 1s)
     e backoff base 100ms, fator 2, sem jitter (Rand devolve 0)
     e relógio fake em que cada tentativa repetida dura exatamente 50ms
QUANDO cada dependência falha em toda tentativa
ENTÃO a dependência 1 retenta 2 vezes e debita 100+50 + 200+50 = 400ms,
     a dependência 2 retenta 2 vezes e debita mais 400ms (restam 200ms),
     e a dependência 3 retenta 1 vez (need = 100+50 = 150ms ≤ 200ms; restam 50ms)
     e na 2ª avaliação recebe Denied == FactorBudget (need = 200+50 = 250ms > 50ms)
  E dmpf_dependency_budget_exhausted_total == 1
  E o span da execução tem dmpf.retry.budget_exhausted = true

DADO um breaker com Defaults() e 3 chamadas na janela, 1 delas com falha
QUANDO a 4ª chamada chega
ENTÃO o breaker está fechado e do é invocado

DADO um breaker com 20 chamadas na janela, 10 com falha
QUANDO a 21ª chamada chega
ENTÃO do NÃO é invocado, o erro é ErrBreakerOpen com categoria própria,
     dmpf_dependency_breaker_state == 2 e a resposta chega antes do timeout

DADO otelboot.Config sem Propagator
QUANDO Start é chamado
ENTÃO devolve ErrPropagatorRequired e nenhum TracerProvider global é alterado

DADO Sampling{write: 0.10} e 1 000 spans root de classe write com TraceIDs construídos
     de modo que exatamente 100 caem dentro da taxa (bits baixos < limiar) e 900 fora,
     e 10 dos 900 fora da taxa terminam com codes.Error
QUANDO todos são encerrados e o processor faz ForceFlush
ENTÃO o InMemoryExporter contém exatamente 110 spans: os 100 amostrados e os 10 com erro
  E os 890 spans RecordOnly sem erro não chegam ao exporter
  E nenhum span recebeu decisão Drop (os 900 têm IsRecording() == true)

DADO ordersapp.Service com Instrumentation = usecase.New(rt, audit.Recording, subject "svc-a")
     e AddItem em pedido aberto com ItemLimit não atingido
QUANDO AddItem é executado
ENTÃO o span dmpf.usecase.orders.AddItem existe com outcome_category = accepted
  E dmpf_service_requests_total{operation="orders.AddItem", outcome_category="accepted"} == 1
  E audit.Recording tem 1 evento {Subject: "svc-a", Object: <OrderID>, Action: "orders.AddItem", Outcome: accepted, At: OccurredAt}
  E o slog em memória não contém o evento de auditoria
  E memory.Store.WithinCalls() == 1

DADO AuthorizeFunc que nega o comando
QUANDO AddItem é executado
ENTÃO EndOperation(Denied) é chamado, nenhuma UoW abre e audit.Recording está vazio

DADO um erro cujo Error() contém "cpf=123.456.789-00" devolvido pela dependência
QUANDO Call falha definitivamente
ENTÃO nenhum registro do slog em memória, nenhum atributo de span e nenhum label contém "123.456"
  E o registro de log contém error_category e error_code

DADO uma Sheet com RateLimit em branco (nem valor nem NotApplicable)
QUANDO Compose é chamado
ENTÃO devolve erro nomeando o campo RateLimit
```

<critical_constraints>
- [P0] Sujeito da baseline (`RES-01`, `RES-24`, `TRC-16`, `LOG-11`): nenhuma
  linha desta entrega acrescenta import de telemetria, decorator, parâmetro de
  retry, prazo ou política de falha a `dmpf-domain` nem a `dmpf-ports`.
- [P0] Matriz de blocos: `dmpf-observability` NUNCA importa `dmpf-application`
  em produção; `dmpf-application` NUNCA importa `dmpf-observability`. O gancho é
  satisfeito estruturalmente; o vínculo é do composition root.
- [P0] `dmpf-application` permanece `pure` + `observability`, sem `require`; o
  gancho usa só `context` e tipos do bloco e de `dmpfports`.
- [P0] Conjunção de retry (`RES-27`..`RES-29`): quatro fatores antes de CADA
  tentativa; indeterminado é falso; o veredicto nomeia o fator falso; nenhuma
  configuração inverte o default.
- [P0] Orçamento por execução (`RES-30`, `RES-31`, `RES-36`): um `Budget` por
  execução, debitado pelo backoff; ausente ⇒ nenhum retry; único valor de
  contexto admitido, nunca fonte de correção.
- [P0] Retry é da chamada remota (`RES-25`, `RES-34`): nunca envolve `Within`,
  o caso de uso ou chamada pós-commit.
- [P0] Propagador explícito (`TRC-09`): sem W3C Trace Context na `Config`, o
  bootstrap falha; nunca default silencioso.
- [P0] Allowlists (`TRC-15`, `MET-04`, `MET-07`, `DAT-22`): atributos e labels
  só por chave declarada; nenhum identificador como label; erro por categoria,
  nunca por mensagem ou payload.
- [P0] Auditoria separada (`LOG-13`, `LOG-14`): `audit.Sink` não deriva do
  log, não é amostrado, recebe sujeito, objeto, ação, desfecho e instante.
- [P0] Overrides declarados, versionados e expostos (`RES-40`).
- [P0] Classificação declarada com `external[]` completo; manifesto e baseline
  em commit próprio.
- [P0] Nenhum artefato declara nem sugere exactly-once fim a fim.
- [P0] Divergência com a fundação vira ADR-035, nunca alteração silenciosa.
- [P1] Pin único `v1.46.0` em todos os módulos OTel; `semconv/v1.43.0`;
  `SemconvVersion` bate com o import; `go.sum` e `go.work.sum` commitados.
- [P1] Testes deterministas: tempo por `clock.Clock` do provider (fake
  avançável), jitter por semente e espera injetada; `dmpf-ports` intocado.
</critical_constraints>

## Escopo fora

- **Admissão por rota e tenant, rate limit de entrada e `MET-12`** (`RES-16`,
  `RES-17`): instalação nos endpoints é do provider HTTP do `KRN-10`. A posição
  `RateLimit` de `Compose` existe e fica `NotApplicable` aqui.
- **Modelagem de cache** (`RES-19`, `RES-20`): `encaminhada` por FND-08 §1.4
  aos épicos de kernel; a posição existe na ordem canônica e fica
  `NotApplicable`.
- **Métricas de pool e conflito de escrita** (`MET-30`), **de relay, outbox,
  inbox e contenção** (`MET-14` a `MET-27`): sinais dos mecanismos do `KRN-06`,
  `KRN-07` e `KRN-08`, que os emitem com os construtores deste módulo.
- **Saturação do próprio serviço** (`MET-11`) e **span de borda** (`TRC-02`,
  `TRC-05`): são do bloco `app`, que nasce em `apps/backend/dmpf-reference` no
  `KRN-08`/`KRN-12`. A suíte de integração desempenha o papel, não o entrega.
- **Continuidade no salto assíncrono** (`TRC-07`, `TRC-08`, links entre F2 e
  F3): depende do envelope de `KRN-05` atravessar um transporte (`KRN-10`).
- **Taxonomia de erro, retryability por categoria e as duas projeções de
  `ERR-20`**: FND-07, consumida por `Classifier` injetado. Realização é
  pendência registrada na SPEC-YRJRADY9, não desta história.
- **Identidade autenticada e `Subject` real** (`IDN-*`): FND-07; aqui
  `SubjectFunc` injetado e ausência registrada como ausente.
- **Dashboards, alarmes em produção, runbook (§8), SLO por serviço e escolha de
  vendor**: `encaminhados` por FND-08 §1.4 e excluídos pelo ticket. O ADR-035
  fixa exportador e destino (collector), não o backend atrás dele.
- **Fixação da versão das *semantic conventions* no BOM**: `KRN-12`; esta
  spec entrega a constante que ele copia.
- **Modo `Defer` de degradação**: é a outbox do `KRN-06`; aqui reprovado.
- **Retry de conflito de escrita** (`RES-35`, `UOW-09`, `UOW-10`): a política
  é do caso de uso e o `KRN-04` a deixou explícita como não automática; este
  módulo fornece espaçamento e teto, e o `KRN-06` decide se os usa.
- **Colisão de numeração de ADR com o `KRN-06`**: a `ARQ-525` corre em branch
  paralela e pode reivindicar `035`. Esta spec ocupa o `035`; quem mergear
  por último renumera, como o `KRN-05` fez com o `033`. Não é resolvido aqui.
- **Test kit exportado dos decorators e do avaliador** (fixtures pareadas
  Go ↔ TS, vetores de `RES-27` como golden): `KRN-11` (`dmpf-testkit`).
- **Consumo do módulo fora do workspace** (`require` com versão publicada,
  tags): `KRN-12`. Aqui a resolução é do `go.work`.
- **Alterar `.golangci.yml`, `tools/dmpf-gate-check.sh`, `nx.json`, `ci.yml`
  ou os targets inferidos**: nenhum glob casa o módulo novo; o gate
  autoritativo é o verificador; a cadeia já cobre projetos `stack:go` afetados.
- **Instrumentar `dmpf-conformance`**: é instrumento, não objeto; o módulo
  não é sujeito da baseline nesta entrega.
