---
id: SPEC-9B6SHEH8
slug: contexto-execucao-identidade-tenant
title: DMPF — Contexto de execução em Go, com identidade e tenant estabelecidos após autenticação
stage: backlog
priority: P0
depends_on: [SPEC-YRJRADY9, SPEC-XQWGGAXF]
ticket_url: null
subtask_urls: []
created: 2026-09-17
---

# SPEC-9B6SHEH8: DMPF — Contexto de execução em Go, com identidade e tenant estabelecidos após autenticação

## Resumo

Realizar em Go o contexto de execução que o FND-07 normatiza: o tipo com os nove
campos de `CTX-01` declarado no bloco `port`, montado na borda pelo bloco `app`,
passado explicitamente ao application service, e com `authenticated_subject` e
`tenant_id` resolvidos **apenas** por autenticação verificada — nunca lidos da
entrada nem de mecanismo ambiental. Fecha, junto, o gancho de autorização que o
kernel deixou declarado e o escopo de tenant que hoje não existe.

É a lacuna registrada no critério **Segurança** da `SPEC-YRJRADY9`, que fechou
com redaction e TLS cumpridos e identidade e tenant não realizados.

## Contexto

### Problema

O kernel Go entregou a instrumentação e o transporte seguros, mas não a
identidade. Três defeitos concretos, medidos em 2026-09-17 na `develop` (`07bf69e`):

| Defeito | Evidência | Regra violada |
| ------- | --------- | ------------- |
| Não existe tipo de contexto de execução em nenhum bloco | grep por `authenticated_subject`, `TenantID` e `tenant_id` em `libs/backend/go/ports/` e `libs/backend/go/application/` não retorna nada | `CTX-01` (nove campos), `CTX-02` (tipo no `port`) |
| O tenant é uma constante literal na borda | `apps/backend/bff/api/routes.go:21` — `Tenant = "public"`; `:119` — `func tenantOf(*http.Request) string { return Tenant }`, que ignora a requisição | `IDN-01`, `IDN-20` (tenant sintético proibido) |
| Não há autenticação na cadeia da borda | `apps/backend/bff/api/middleware.go:33-56` (`withRequestContext`) monta correlação, request ID e trace, e segue direto ao handler | `IDN-01`, `IDN-15` |

O que existe hoje no lugar do contexto é o `rpc.Call`
(`apps/backend/bff/rpc/metadata.go:19-23`), com três campos — `CorrelationID`,
`RequestID`, `IdempotencyKey` — propagados por `context.WithValue`. São três dos
nove campos, nenhum deles de identidade, e o veículo é exatamente o mecanismo
ambiental que `CTX-05` proíbe como fonte de valor de que a correção dependa.

A autorização está declarada e vazia por escolha explícita: `AuthorizeFunc`
existe (`libs/backend/go/application/authorize.go:7-10`), recebe `context.Context`
em vez do contexto de execução, e as duas composition roots a preenchem com
`AllowAll` (`apps/backend/orders/wiring.go:92`,
`apps/backend/reservations/consumer.go:48`). O próprio kernel registra que isso é
provisório, em `libs/backend/go/application/doc.go:20`:

> FND-07's — AuthorizeFunc is only the hook they will fill.

### Por que cabe a uma spec de kernel

O FND-07 delimita o próprio alcance em `docs/dmpf/contexto-erros-seguranca.md:267-270`:

> Fora do épico inteiro, por P0-4: kernels Go e TypeScript, adapters e providers
> de produção. Este artefato especifica o que o contexto carrega, como o erro se
> classifica e o que o controle de segurança precisa garantir; construí-los em
> cada stack é dos épicos de kernel.

A norma define o critério; a realização em Go é desta spec.

### Estado atual medido

| Ponto | Evidência |
| ----- | --------- |
| Nenhum tipo de contexto de execução declarado | `libs/backend/go/ports/` — treze identificadores exportados, nenhum de contexto |
| `AuthorizeFunc` recebe `context.Context`, não o contexto de execução | `libs/backend/go/application/authorize.go:10` |
| As duas composition roots autorizam tudo | `apps/backend/orders/wiring.go:92`, `apps/backend/reservations/consumer.go:48` |
| Metadado da borda: 3 campos por `context.WithValue` | `apps/backend/bff/rpc/metadata.go:19-29` |
| Tenant vai ao trace como literal | `apps/backend/bff/api/middleware.go:46` — `.TenantID(Tenant)` |
| Redaction e TLS já cumpridos (não entram nesta spec) | `libs/backend/go/observability/redact/redact.go:37-63`; `libs/backend/go/grpc/config.go:117-125` |

### Fontes normativas

- `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — `CTX-01`..`CTX-14`, `IDN-01`..`IDN-20`
- `docs/dmpf/contexto-erros-seguranca.md:267-270` — a construção em cada stack é dos épicos de kernel
- FND-04 §3.2 e FND-03 §4.2 — a autorização de aplicação é o passo 1, antes da Unit of Work (`IDN-07`)
- `docs/specs/SPEC-YRJRADY9-dmpf-kernel-sdk-go.md` — critério **Segurança**, que registra esta lacuna
- `docs/specs/SPEC-XQWGGAXF-dmpf-contexto-erros-seguranca.md` — a spec do artefato normativo

<constraints>
- [P0] `authenticated_subject` e `tenant_id` nunca têm por fonte um campo da entrada; divergência entre o que a entrada traz e o que foi resolvido **recusa** a requisição (`CTX-06`).
- [P0] Autenticação exige as três condições de `IDN-01` — credencial apresentada, verificada contra a autoridade e resolvendo um sujeito. Nenhuma é presumida pelas outras.
- [P0] Confiança de canal não autentica sujeito (`IDN-02`): rede interna, gateway ou mesh estabelecem o chamador, não o sujeito.
- [P0] `system`, `default`, `anonymous`, `unknown` e tenant sintético são proibidos como valor; a forma correta da ausência é a ausência (`IDN-20`).
- [P0] O contexto é passado explicitamente como argumento ao application service; a UPR não o recebe, e o que alcança o `domain` são valores extraídos (`CTX-03`).
- [P0] Mecanismo ambiental (`context.Context`, thread-local) não é fonte de valor de que a correção dependa (`CTX-05`).
- [P0] A autorização ocorre no application service **antes** de a Unit of Work ser iniciada (`IDN-07`), e decide sobre `permissions` já resolvidas, sem I/O no meio do caso de uso (`IDN-10`).
- [P0] O tipo do contexto é declarado no `port`; a instância é montada no `app` (`CTX-02`). Nenhum outro bloco monta contexto.
- [P0] Não quebrar os gates DMPF existentes: a matriz de blocos, o verificador de conformidade, o shared kernel e o BOM seguem verdes.
- [P1] O contexto é imutável depois de montado (`CTX-04`); derivar é montar outro.
</constraints>

## Requisitos

### Funcionais

**A. Tipo do contexto no bloco `port`**

- [ ] **[P0] Os nove campos de `CTX-01`**: `request_id`, `correlation_id`, `causation_id`, `trace_context`, `authenticated_subject`, `tenant_id`, `permissions`, `deadline`, `locale`, cada um com a presença (obrigatória, condicional ou ausente) declarada conforme FND-07 §3.1.
- [ ] **[P0] Ausência é ausência**: o tipo distingue campo ausente de campo vazio, de modo que nenhum bloco consiga escrever `""` no lugar de um sujeito não resolvido.
- [ ] **[P0] Imutabilidade**: o valor é imutável após montado; derivar contexto para sub-operação produz outro valor (`CTX-04`).
- [ ] **[P1] Sem dependência de I/O**: o bloco `port` continua sem capability de I/O — o tipo é declaração, e valores que exigem I/O vêm do `provider` (`CTX-02`).

**B. Montagem na borda pelo bloco `app`**

- [ ] **[P0] Autenticação antes do contexto**: a borda HTTP resolve o sujeito pelas três condições de `IDN-01` e só então monta o contexto. Requisição não autenticada em operação que exige sujeito é negada.
- [ ] **[P0] Subject e tenant nunca da entrada**: nenhum header, query ou corpo é fonte de `authenticated_subject` ou `tenant_id`; divergência recusa a requisição (`CTX-06`).
- [ ] **[P0] Consumo assíncrono**: o consumer monta contexto a partir da entrada autenticada de `IDN-04` — integridade do envelope mais confiança da fronteira de transporte. Mensagem que não satisfaça as duas não produz contexto.
- [ ] **[P1] Correlação preservada como hoje**: `correlation_id` de fronteira confiável é preservado, e gerado quando ausente, malformado ou de fronteira não confiável (`CTX-07`). O comportamento atual do BFF já satisfaz e é mantido.

**C. Passagem explícita e autorização**

- [ ] **[P0] Argumento, não ambiente**: o contexto chega ao application service como argumento; `context.Context` segue carregando cancelamento e prazo, nunca os campos de que a correção depende (`CTX-03`, `CTX-05`).
- [ ] **[P0] `AuthorizeFunc` recebe o contexto**: a assinatura passa a receber o contexto de execução junto do comando, e a decisão usa as `permissions` já resolvidas, sem consultar autoridade de identidade (`IDN-10`).
- [ ] **[P0] Antes da Unit of Work**: a invocação continua sendo o passo 1 da sequência canônica, anterior a `Within` (`IDN-07`).
- [ ] **[P0] Autenticado não é autorizado**: a presença de `authenticated_subject` nunca é tratada como permissão (`IDN-05`), e as duas falhas produzem categorias distintas na taxonomia de FND-07 §5 (`IDN-06`).

**D. Declaração por operação**

- [ ] **[P0] Cada operação declara o que exige**: sujeito, tenant, ambos, ou nenhum por ser operação de plataforma. A declaração é do `app`, junto da definição da operação (`IDN-16`).
- [ ] **[P0] Omissão fecha**: operação sem declaração é tratada como exigindo sujeito e tenant (`IDN-17`); exigência não satisfeita nega, sem depender de regra explícita para o caso (`IDN-15`).
- [ ] **[P1] Cadeia de plataforma**: rotina sem sujeito é caso legítimo, autorizada pela identidade do workload e pelo escopo declarado, com os campos ausentes na forma de `ENV-12` (`IDN-18`, `IDN-19`).

**E. Escopo de tenant**

- [ ] **[P0] Toda leitura e escrita escopada**: consulta, comando, agregação e exportação passam pelo tenant do contexto (`IDN-11`).
- [ ] **[P0] Imposição sem depender de convenção**: um caminho que omita o escopo não pode devolver nem alterar dado de outro tenant. Isolamento cuja correção dependa de cada autor lembrar da condição não satisfaz o critério (`IDN-14`).
- [ ] **[P0] Acesso cruzado não devolve dado**: a tentativa falha e é registrada como evento de segurança com sujeito, tenant do contexto e tenant do dado alcançado (`IDN-12`).
- [ ] **[P1] Resposta indistinguível**: ao chamador, a resposta pode ser indistinguível de inexistência; o registro interno distingue os dois casos (`IDN-13`).

**F. Travessia de fronteira**

- [ ] **[P0] Sujeito não atravessa**: `authenticated_subject` e `permissions` não vão ao fan-out nem ao downstream como valor de contexto; a identidade com que este serviço chama outro é a sua própria, e a do sujeito original é proveniência para auditoria (`CTX-12`).
- [ ] **[P0] Tenant atravessa preservado**: e a ausência no envelope significa cadeia sem sujeito, na forma de `ENV-12`, sem valor de preenchimento em nenhuma das quatro fronteiras (`CTX-13`).
- [ ] **[P1] Matriz completa**: toda travessia resolve, para cada um dos nove campos, exatamente uma das quatro ações da matriz de FND-07 §3.3 (`CTX-11`).

**G. Substituição do provisório**

- [ ] **[P0] Fim do tenant literal**: `Tenant = "public"` e `tenantOf` saem de `apps/backend/bff/api/routes.go`.
- [ ] **[P0] Fim do `AllowAll` nas composition roots**: `apps/backend/orders/wiring.go` e `apps/backend/reservations/consumer.go` passam a uma autorização real. O `AllowAll` permanece no kernel como escolha explícita disponível, nunca como default silencioso.
- [ ] **[P1] `rpc.Call` reconciliado**: o metadado de três campos da borda é absorvido pelo contexto ou explicitamente justificado como veículo de transporte, não como fonte.

### Não-funcionais

- [ ] **Verificabilidade**: cada regra `IDN-` e `CTX-` realizada tem vetor executável, positivo e negativo, no padrão do `testkit/fitness`.
- [ ] **Domínio intocado**: nenhuma das mudanças alcança o bloco `domain`; o que a UPR recebe continua sendo valor extraído (`CTX-03`, FND-03 `FRT-03`).
- [ ] **Sem regressão de gate**: matriz de blocos, verificador, shared kernel, generator, conformance e BOM seguem verdes.
- [ ] **Sem I/O no caso de uso**: a decisão de autorização não introduz chamada de rede no meio da sequência canônica (`IDN-10`).

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| `port` | tipo do contexto de execução e declaração de exigência por operação |
| `application` | assinatura de `AuthorizeFunc`; contexto como argumento do service |
| `app` | montagem na borda HTTP e no consumer; autenticação; declaração por operação |
| `provider` | escopo de tenant no repositório e no reader; evento de segurança do acesso cruzado |
| `domain` | nenhum |
| Testes | vetores positivos e negativos por regra realizada |

## Localização de código

```text
libs/backend/go/ports/                       tipo do contexto, declaração de exigência
libs/backend/go/application/authorize.go     assinatura de AuthorizeFunc
libs/backend/go/application/                 contexto como argumento do service
libs/backend/go/postgres/                    escopo de tenant no repositório e no reader
libs/backend/go/testkit/fitness/             vetores das regras IDN e CTX
apps/backend/bff/api/middleware.go           autenticação e montagem do contexto
apps/backend/bff/api/routes.go               remoção do tenant literal; declaração por rota
apps/backend/bff/rpc/metadata.go             reconciliação do metadado de borda
apps/backend/orders/wiring.go                autorização real no lugar de AllowAll
apps/backend/reservations/consumer.go        idem, e montagem no consumo
apps/backend/{orders,reservations}/provider/ escopo de tenant
```

## Design

### Arquitetura

```text
  requisição
      │
      ▼
  [app] borda HTTP
      │  1. autentica (IDN-01: apresentada + verificada + sujeito resolvido)
      │  2. resolve tenant e permissions efetivas
      │  3. monta o contexto (CTX-02), imutável (CTX-04)
      ▼
  [application] service
      │  passo 1: Authorize(ctx, execCtx, cmd)   ← antes da UoW (IDN-07)
      │  passo 2..9: sequência canônica de FND-04 §3.2
      ▼
  [port] UnitOfWork ─ Within ─ [provider] repositório escopado ao tenant (IDN-11)
      │
      ▼
  [domain] UPR recebe valores extraídos, nunca o contexto (CTX-03)
```

### A fronteira do que atravessa

| Campo | Fan-out síncrono | Publicação assíncrona |
| ----- | ---------------- | --------------------- |
| `authenticated_subject` | não atravessa; vira proveniência (`CTX-12`) | idem |
| `permissions` | não atravessam (`CTX-12`) | idem |
| `tenant_id` | preservado (`CTX-13`) | preservado; ausência = cadeia sem sujeito |
| `correlation_id`, `trace_context` | preservados de fronteira confiável (`CTX-07`, `CTX-09`) | idem |
| `causation_id` | passo imediatamente anterior (`CTX-08`) | idem |

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Veículo do contexto | Argumento explícito do application service | `context.Context` como fonte — `CTX-05` o proíbe como valor de que a correção dependa; permanece para cancelamento e prazo |
| Onde o tipo vive | Bloco `port` | `application` (romperia `CTX-02`); `domain` (o domínio não conhece contexto) |
| Ausência de sujeito | Representada como ausência no tipo | Sentinela `""`, `anonymous` ou tenant sintético — `IDN-20` proíbe |
| Imposição do escopo de tenant | Mecânica, no provider, sem depender de o autor lembrar | Convenção de código com revisão — `IDN-14` a recusa explicitamente |
| Mecanismo de autenticação | Consumido, com a escolha do provedor declarada fora | Especificar o IdP aqui — FND-07 o põe fora por decisão própria (`:272-274`) |
| `AllowAll` | Permanece no kernel como escolha explícita | Remover — deixaria a composition root sem forma de declarar "sem autorização" deliberadamente |

## Riscos

| Risco | Mitigação |
| ----- | --------- |
| A mudança de assinatura de `AuthorizeFunc` alcança as duas composition roots e os services de `orders` e `reservations` | A mudança é mecânica e coberta por teste; a sequência canônica não muda de forma |
| Escopo de tenant no provider pode alterar consultas existentes e quebrar o e2e | O e2e caixa-preta do BFF é a rede de segurança; rodar antes e depois |
| Sem IdP escolhido, a autenticação vira mock e o critério `IDN-01` não é de fato provado em produção | A spec entrega a fronteira e o vetor; a escolha do provedor é declarada como pré-requisito operacional, com o mock explicitamente marcado como de desenvolvimento |
| A ausência tipada pode vazar como valor vazio na serialização do envelope | `ENV-12` define a forma da ausência; o golden do contrato prova |

## Verificação e testes

### Critérios de aceite

- [ ] Nenhuma ocorrência de tenant literal ou sintético no código de produção: `Tenant = "public"`, `system`, `default`, `anonymous` e `unknown` não aparecem como valor de sujeito ou tenant.
- [ ] Requisição sem credencial, em rota que exige sujeito, é negada na borda e não alcança o application service.
- [ ] Requisição cuja entrada traz sujeito ou tenant divergente do resolvido é **recusada**, com a categoria de elevação, e não com a de dado redundante.
- [ ] Rota sem declaração de exigência é tratada como exigindo sujeito e tenant, provado por vetor negativo.
- [ ] Uma leitura que omita o escopo de tenant não devolve linha de outro tenant, provado por teste que exercita o caminho omisso.
- [ ] Tentativa de acesso cruzado registra evento de segurança com sujeito, tenant do contexto e tenant do dado alcançado.
- [ ] `authenticated_subject` e `permissions` não aparecem no envelope publicado nem na metadata gRPC do fan-out; `tenant_id` aparece preservado.
- [ ] A autorização é invocada antes de `Within`, provado por ledger do fake de Unit of Work.
- [ ] Os gates DMPF existentes e a cadeia de validação do projeto seguem verdes.

### Cenários de teste

**Happy path — requisição autenticada**
- DADO uma requisição com credencial válida em rota que exige sujeito e tenant
- QUANDO a borda a processa
- ENTÃO o contexto é montado com os nove campos, a autorização roda antes da Unit of Work e a operação alcança apenas dado do tenant resolvido

**Erro — credencial ausente**
- DADO uma requisição sem credencial em rota que exige sujeito
- QUANDO a borda a processa
- ENTÃO é negada com a categoria de autenticação, e nenhum contexto é montado

**Erro — sujeito autenticado sem permissão**
- DADO um sujeito autenticado e sem a permissão da operação
- QUANDO o application service roda
- ENTÃO a negação é de autorização, categoria distinta da de autenticação, e a Unit of Work nunca é iniciada

**Erro — tenant vindo da entrada**
- DADO uma requisição cujo corpo traz `tenant_id` divergente do resolvido
- QUANDO a borda a processa
- ENTÃO a requisição é recusada como tentativa de elevação

**Erro — caminho sem escopo de tenant**
- DADO um reader que omita a condição de tenant
- QUANDO uma consulta roda sob um tenant
- ENTÃO nenhuma linha de outro tenant é devolvida, e o defeito é nomeado pelo vetor

**Edge case — operação de plataforma**
- DADO uma rotina declarada como de plataforma, com escopo de dado declarado
- QUANDO ela roda sem sujeito
- ENTÃO não é negada por ausência, e o alcance respeita o escopo declarado

<critical_constraints>
- [P0] `authenticated_subject` e `tenant_id` nunca têm por fonte um campo da entrada; divergência recusa a requisição (`CTX-06`).
- [P0] Autenticação exige credencial apresentada, verificada e resolvendo um sujeito; nenhuma condição é presumida pelas outras (`IDN-01`).
- [P0] Confiança de canal não autentica sujeito (`IDN-02`).
- [P0] `system`, `default`, `anonymous`, `unknown` e tenant sintético são proibidos como valor (`IDN-20`).
- [P0] O contexto é argumento explícito do application service; a UPR não o recebe (`CTX-03`).
- [P0] Mecanismo ambiental não é fonte de valor de que a correção dependa (`CTX-05`).
- [P0] A autorização ocorre antes de a Unit of Work ser iniciada (`IDN-07`) e não faz I/O (`IDN-10`).
- [P0] O tipo vive no `port`; a instância é montada no `app` (`CTX-02`).
- [P0] O isolamento de tenant não depende de convenção de código (`IDN-14`).
- [P0] Os gates DMPF existentes seguem verdes.
</critical_constraints>

## Escopo fora

- **Escolha do provedor de identidade** (OIDC, mTLS de workload, token opaco): o FND-07 a põe fora por decisão própria (`docs/dmpf/contexto-erros-seguranca.md:272-274`), definindo só o critério do que conta como requisição autenticada. Esta spec consome o mecanismo e declara o pré-requisito.
- **Política corporativa de classificação de dados**: consumida como entrada onde existir (FND-07 §8).
- **Quais dados de cada bounded context são sensíveis**: é de cada contexto de negócio.
- **Redaction e TLS**: já cumpridos pelo `KRN-09` e pelo `KRN-10`, e verificados no fechamento da `SPEC-YRJRADY9`.
- **Kernel TypeScript**: a contraparte é do épico de ordem 2.
- **Taxonomia de erro**: FND-07 §5 já a define; esta spec a consome, não a redefine.
