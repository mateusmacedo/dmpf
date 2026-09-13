# ADR-041: Entregar o SDK de referência como composition root sob `apps/`, com a release do produto por tag anotada e BOM certificado

## Status

Aceito — 2026-09-08. Implementa SPEC-6QT9SBAS, primeira sub-spec de SPEC-8HWBWJCB (KRN-12). As sub-specs seguintes — generator (SPEC-H1A190Y8), BOM e validador (SPEC-538MS2D4), evidência e tag (SPEC-JPP31095) — acrescentam addenda a este ADR em vez de abrir outro; a consolidação final é da última.

## Contexto

O KRN-02 a KRN-11 entregaram o kernel DMPF em Go como quatorze libs, cada uma provada por suíte própria, mas nenhum serviço mostrava os blocos cabeados num processo real: `apps/backend` era um `.gitkeep`, e o único lugar onde instanciar provider concreto é permissivo — a linha `app` da matriz de blocos (ADR-010, ADR-015) — só existia como composition root de exemplo dentro de `dmpf-app`. O ticket ARQ-531 pede o SDK de referência, o generator de bounded context, o BOM de combinação certificada (FND-10) e a evidência da release `0.1.0`; a spec guarda-chuva dividiu isso em quatro sub-specs porque a revisão externa mostrou blockers independentes entre os subsistemas.

Duas lacunas do kernel apareceram ao cabear: (1) ninguém gravava o contexto de mensagem de ENV-08 — o provider Postgres escrevia `metadata = '{}'` (ADR-035) e o relay só reivindica linhas com `correlationid`, `causationid` e `traceparent` (ADR-038), então nenhuma escrita real drenaria; (2) o `orders` autora dois eventos (`ItemAdded`, `OrderPlaced`) para um único destino lógico, e o consumer de `reservations` só entende o segundo.

Restrições herdadas: provider concreto só no bloco `app` (ADR-015); `Ack` estritamente depois do retorno da transação (INB-08); toda rota HTTP referencia um contrato publicado (RST-04); um transporte assíncrono por processo (decisão do usuário na guarda-chuva); nenhuma dependência Go nova; a criação de unidade é ato regulado com baseline em commit próprio (AUT-01, `DMPF-T002`).

## Decisão

**Um binário, três papéis, um módulo `type:app`.** `apps/backend/dmpf-reference` é o projeto `dmpf-reference-go` (`type:app`, `scope:backend`, `stack:go`, `layer:apps`) e a unidade `dmpf-kernel/reference-app` do bloco `app`. O `--role api|relay|consumer` escolhe o processo — a relay nunca divide processo com o caminho da requisição (BLK-02) — e a configuração entra só por variável de ambiente, validada por papel na partida, com exit 2 nomeando a variável ausente. Os três papéis reutilizam os agregados de exemplo do kernel (`orders`, `reservations`) e o padrão de `dmpf-app/example/reservations`.

**O contexto de mensagem é autorado na borda e atravessa o kernel.** `dmpf-ports` ganhou `MessageContext{CorrelationID, CausationID, Traceparent}` com `WithMessageContext`/`MessageContextFrom` e o campo `OutboxEntry.Context`; o provider Postgres serializa o que foi autorado em `metadata` (chave ausente é ausente, nunca `""`, para não produzir linha que o relay rejeitaria); `dmpfapplication.MessageContextFor(ctx, id)` copia o contexto e preenche só a causação de quem inicia a cadeia — o próprio `id`, como FND-05 exige para ENV-08; `dmpf-app.Consumer` põe no contexto do handler o `correlationid`, o `id` recebido como causação e o `traceparent` do envelope. Na `api`, o middleware abre o span de servidor, injeta o `traceparent` pelo propagador W3C e usa `X-Correlation-ID` do cliente ou cunha um. Com isso a linha escrita pelo `api` é drenada de fato — o critério 2 do ticket.

**`api` sobre `net/http.ServeMux`, com a cadeia por rota dentro do mux.** Cada rota é um `dmpfhttp.Route` com `ContractRef` apontando para `contracts/openapi/orders/v1/openapi.yaml` (JSON pointer), validado na construção; a cadeia `Admission → requireIdempotencyKey → withMessageContext → handler` é montada por rota, e não em volta do mux, para que a admissão seja chaveada por `r.Pattern` — três chaves, cardinalidade limitada (MET-07) — e decida antes de ler o corpo (RES-17). `GET /orders/{id}` lê por `orderspg.NewReader(pool)` sem abrir transação (UOW-11), provado por `QueryTracer`. `DMPF_MIGRATE=true` aplica o schema antes de servir.

**Canal nomeado pelo destino do caso de uso; assinatura por tipo na ponte.** O canal Kafka chama-se `ordersapp.Destination` (`orders.events`), porque o publisher resolve pelo destino que o application service autorou; só `Address`, `Group` e `Containment` vêm do ambiente. Como o destino carrega os dois eventos do agregado, o `Sink` da app — a ponte transporte→adapter de FND-06 §11 — confirma sem inbox as entregas de tipo diferente do assinado (`EventType` + major do canal) e entrega ao adapter o tipo assinado e o que não decodifica (INB-10 intacto). Um canal por tipo de evento é matéria do generator (SPEC-H1A190Y8), não desta entrega.

**Telemetria com um runtime por processo e fallback em memória.** `otelboot.Start` roda uma vez por processo; `Run` faz o boot e `RunWith` recebe o runtime pronto, que é como o e2e hospeda os três papéis num binário só. Com `DMPF_OTLP_ENDPOINT` os exportadores são OTLP/gRPC; sem ele, `tracetest` e `ManualReader` em memória, com aviso de modo de desenvolvimento — a app sobe sem Collector.

**Nx e release.** `build`, `fmt-check`, `vet`, `test-race` (com `dependsOn` sobre `dmpf-provider-postgres-go` e `dmpf-app-go`, porque os três compartilham o Postgres do job) e `govulncheck` seguem o padrão das libs; `serve-api`, `serve-relay` e `serve-consumer` são `nx:run-commands`. O `@nx-go/nx-go` deriva o nome do projeto do último segmento do diretório, então infere `build` e `serve` assim que `cmd/dmpf-reference/main.go` existe: o `build` explícito sobrescreve o inferido e o `serve` inferido fica sem uso, documentado. O `nx-release.yml` passa a selecionar candidatos Docker por `tag:type:app,!tag:stack:go` — a imagem é do golden path, não desta app.

**Identidade da release do produto (transversal, da guarda-chuva).** A release do produto DMPF é uma tag git anotada `dmpf@<semver>` em `master`, cunhada pelo rito do BOM, distinta das tags por projeto do `nx release`; um BOM por tag em `bom/dmpf/<semver>.json`, com `version` **efetiva** por entrada (o `package.json` das libs, o SHA curto do commit para a app, o `go.mod` para dependências externas). "Sem edição manual" significa que nenhum byte gerado muda entre o generator e a aprovação pelo verificador — o rito de classificação em commit próprio é ato humano exigido por AUT-01, não edição. O escape hatch admite pedido só por catálogo fechado N1–N7 reconhecido por regra, nunca por texto. Um transporte assíncrono por processo. As sub-specs 2 a 4 realizam esses itens e registram aqui o que decidirem além deles.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Um `cmd/` por papel | BLK-02 pede processo, não binário; três binários triplicariam config, telemetria e cablagem sem separar nada que o `--role` não separe |
| Framework HTTP (chi, gin) ou helper server-side em `dmpf-provider-http` | Dependência Go nova, ou mudança no KRN-10 fora de rito; o `ServeMux` de Go ≥ 1.22 já casa método e path |
| Mux envolto pela admissão, como o plano previa | Fora do mux `r.Pattern` está vazio e a chave de admissão teria de ser o path bruto — cardinalidade aberta (MET-07) |
| Deixar o contexto de mensagem como task bloqueante ou gravá-lo por SQL no e2e | O critério 2 do ticket exige a drenagem real; sem a plumbing o e2e provaria uma linha que nunca sai da outbox |
| Quarentenar o `ItemAdded` no consumer de `reservations` | Toda escrita geraria uma contenção por desenho; o filtro por tipo na ponte é o gesto do transporte, e a inbox continua a ver só o que é do consumidor |
| OTLP obrigatório | A app não subiria sem Collector em desenvolvimento; o fallback em memória é explícito e logado |
| Nome do canal por `DMPF_CHANNEL` | O publisher resolve pelo destino autorado pelo caso de uso; um nome divergente terminaria em `ErrUnknownChannel` |
| Release group do `nx release` para o produto | Exigiria `package.json` com versão agregada e conflitaria com o versionamento independente (ADR-030) |

## Consequências

**Positivas:**

- O kernel passa a ter um serviço executável de referência com escrita, drenagem e consumo provados fim a fim sobre Postgres e Redpanda, incluindo reentrega com `DuplicateIgnored`.
- A lacuna de ENV-08 documentada em ADR-038 fecha: a outbox recebe o contexto de mensagem autorado pela borda, e o sinal `pending` deixa de crescer por desenho.
- `orderspg.NewReader` dá ao lado de leitura a forma que UOW-11 pede, reutilizando o `SELECT` do repositório.
- O contrato OpenAPI de `orders` inaugura `contracts/openapi/` com referência a partir de cada rota (RST-04).

**Negativas:**

- **Custo aceito:** `orders.events` continua a carregar dois tipos de evento; o filtro fica na app, e o `ItemAdded` é confirmado sem inbox — um canal por tipo é dívida do generator.
- **Custo aceito:** o `serve` inferido pelo `nx-go` existe e não faz nada útil; renomear o diretório do comando para evitá-lo daria um binário com nome errado.
- **Custo aceito:** o `test-race` da app exige Postgres e Redpanda e serializa-se atrás de dois módulos; a cadeia local fica mais longa.
- **Custo aceito:** o `api` do e2e é servido por `httptest` sobre o mesmo handler; `ListenAndServe` e o shutdown gracioso do `serveAPI` só são exercitados manualmente.
- **Custo aceito:** o `dmpf-units.json` da app declara o `external` de todos os providers cabeados; qualquer provider novo no golden path exige atualizá-lo.

Nada aqui declara ou sugere entrega exactly-once fim a fim. A garantia é at-least-once com consumidor idempotente pela inbox (KRN-07).

## Addendum — 2026-09-08 (checklist de qualidade da sub-spec 1)

A revisão de segurança, robustez e performance da entrega, feita com tráfego real sobre o Compose de `infra/local`, decidiu o que segue e registrou o que fica.

**Decidido e aplicado:**

- **A borda valida o que o contrato publica.** O domínio de `orders` não valida `quantity` nem `sku`, então a api recusava nada: `quantity: -1` era aceito e publicado. A borda passou a exigir `id` e `X-Correlation-ID` em `[A-Za-z0-9._:-]{1,128}`, `sku` com 1 a 128 caracteres, `quantity ≥ 1`, um único objeto JSON sem campos desconhecidos — `400` com `invalid-request`/`malformed-body`, e o OpenAPI passou a dizer o mesmo (`maxLength`, `pattern`, `additionalProperties: false`). Alternativa descartada: rejeição de domínio para `quantity ≤ 0`, que é invariante do agregado e cabe ao kernel, não a esta task.
- **Correlação do cliente é limitada, não confiada.** Um `X-Correlation-ID` de 8 KB era gravado em `metadata` e publicado em todo envelope da cadeia. Fora do formato, a borda cunha um novo e o ecoa no mesmo header.
- **O servidor HTTP tem todos os prazos** (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes` de 64 KiB), não só o do cabeçalho: `MaxBytesReader` limitava tamanho, não tempo.
- **Todo destino que os casos de uso autoram tem canal.** `reservationsapp.Destination` (`reservations.events`) não estava no catálogo e cada `ReservationConfirmed` terminava `failed` após dez tentativas. O catálogo passou a ter dois canais (`DMPF_KAFKA_RESERVATIONS_TOPIC/DLQ`), e o e2e exige `failed = 0`.
- **W3C explícito nos dois lados.** O kernel já recusava propagador sem `traceparent`/`tracestate` (`ErrPropagatorNotW3C`); a app declara `TraceContext + Baggage` e o Collector `propagators: [tracecontext, baggage]`.
- **Um envelope de log.** A trilha de auditoria saía como `audit.Event` cru (sem `time`, `level`, `service`) e o SDK OpenTelemetry reportava falhas de exportação por `log.Printf`. A auditoria continua canal separado e não amostrado, mas no mesmo envelope do handler (`kind: "audit"`, com `trace_id` e `correlation_id` da requisição), e o `otel.ErrorHandler` passa pelo logger da plataforma. O `Fields` de LOG-01 passou a ser preenchido (`correlation_id`, `tenant_id`).
- **Buckets do histograma em segundos** (`dmpf-observability/metrics/instruments.go`). A série `dmpf_service_request_duration_seconds` gravava segundos com os buckets default do SDK (`5, 10 … 10000`, feitos para milissegundos): toda requisição caía em `(0, 5]` e `histogram_quantile` respondia 2,5 s para uma chamada de 0,5 ms. É a única mudança fora da app; um teste fixa a escala.
- **Grafana fail-closed no Kubernetes.** A base exige login (`grafana-admin`) e o acesso anônimo é patch do overlay `dev`; antes ele estava na base e o `hmg` herdava — um Admin anônimo cria datasource, e datasource é proxy HTTP para qualquer endereço do cluster. `DMPF_OTLP_INSECURE` seguiu o mesmo caminho (base `false`, `dev` liga). No Compose, o anônimo continua (é desenvolvimento) e o formulário de login abre a administração do servidor.
- **NetworkPolicy como controle compensatório da borda sem identidade.** Ingress na api só de pods rotulados como cliente; nenhum ingress em `relay` e `consumer`. O Secret de exemplo do `hmg` deixou de ser resource (um `apply -k` sobrescrevia o Secret real com placeholders) e o ClusterRole do Alloy perdeu `nodes`/`nodes/proxy`, que a coleta pela API não usa.
- **Orçamento de recursos** em todo serviço do Compose, com `tools/infra-budget.sh` reprovando acima de 60% do host: 8,00 vCPU (57,1%) e 7,9 GiB (50,5%) para 20 serviços.

**Dívida registrada** (mudanças maiores ou que exigem validação empírica):

- Identidade e autorização reais na borda (token → subject → `AuthorizeFunc`), que também daria ator à trilha de auditoria; é spec própria.
- Malha completa de `NetworkPolicy` (`default-deny` por namespace + política por componente: Prometheus → alvos, Grafana → datasources, Alloy → Loki, Collector → Tempo/Prometheus).
- Pin por digest das imagens em `hmg` e `imagePullPolicy` declarado; `securityContext` em Postgres e Redpanda das bases; rotação de credencial por rollout (`disableNameSuffixHash`).
- cAdvisor sem `privileged` e Alloy sem `root` no Compose — podem falhar no WSL, precisam de teste antes.
- Publicação em `0.0.0.0` no Compose é decisão do usuário (acesso do Windows); a consequência está no `infra/README.md`.
- Rota de saúde na api (sondas por socket hoje), pool pgx com defaults, `INFRA_BUDGET_FRACTION` sobrescrevível pelo ambiente.

## Addendum — 2026-09-09 (generator `bounded-context` da sub-spec 2)

A implementação da SPEC-H1A190Y8 mudou de desenho duas vezes no mesmo dia, e o resultado final é o que segue. Um mini bounded context fixo (`Request`/`Fulfillment`) gerado em todo contexto foi descartado por produzir sempre o mesmo código; um generator orientado ao domínio, com DSL própria, foi especificado, revisado por duas revisões externas (24 achados, 16 bloqueantes) e deferido pelo custo (`SPEC-8FSD8505`, `SPEC-VZ16X0MS`, `SPEC-F7S5B6KV`). A prova em worktree também expôs que a norma vigente impede qualquer contexto fora de `dmpf-kernel` de consumir o kernel (`DMPF-D002`; `domain` nunca é superfície pública), o que virou spec normativa própria.

**Decidido e aplicado:**

- **O generator gera só o esqueleto; o código de negócio é do harness de agentes.** Por bloco pedido, um módulo com `README.md`, `doc.go`, `go.mod`, `project.json`, `package.json` e `dmpf-units.json`, mais a entrada no `go.work` e a instrução do baseline — a parte que não admite erro e é trivialmente determinística. Agregados, UPRs, casos de uso, repositórios, rotas, OpenAPI e o contrato `.proto` (pelo rito Buf) são escritos por um agente a partir de uma spec de bounded context em template próprio (`SPEC-VDP9XX65`), e a garantia não é bytes idênticos, mas os gates que o repositório já tem: verificador, cadeia Go, prova, `biome`/`gofmt`, testes. Alternativas descartadas: mini contexto fixo (mesmo código em todo contexto) e generator orientado ao domínio (custo da DSL).
- **Shared kernel como pré-requisito, não contorno.** Declarar o contexto gerado como `dmpf-kernel` apagaria a identidade de limite do ADR-017; re-escopar para um esqueleto sem kernel cumpriria o critério 1 de ARQ-531 de forma trivial. A designação de unidades do kernel como importáveis por qualquer contexto é a `SPEC-XMNBMY50`.
- **`formatFiles` não é chamado.** O Prettier não está instalado no workspace e o `formatFiles` do devkit é no-op quando falta, o que faria a saída do generator depender do ambiente e quebrar o determinismo exigido. Os templates saem no formato final, e valores de opção que entram em JSON passam por `JSON.stringify` com `<%- %>` em vez de interpolação crua.
- **O nome entra literal, sem pluralização automática.** `order-fulfillment` produz diretórios e nomes de projeto em kebab-case e package Go raiz `orderfulfillment<bloco>`; não há pluralização (`Category` → `Categories` quebraria um `+s` ingênuo) e identificadores de tabela e de rota são do código de negócio, escrito pelo harness a partir da spec do contexto.
- **A prova roda em worktree sobre `HEAD`, com manifesto de hashes.** Um worktree em `develop` não teria o plugin. E como o pre-commit do Lefthook roda `biome check --write` com `stage_fixed`, um JSON gerado fora do padrão seria corrigido no ato do commit e o `git diff --exit-code` posterior nada acusaria: por isso a prova checa sem escrita antes de commitar e guarda o SHA-256 de cada arquivo gerado fora do worktree, conferindo depois. Os commits declaram identidade de automação por `git -c`, porque o job do CI não tem autor configurado.
- **O worktree da prova compartilha o `node_modules` da raiz por symlink, e isso exige dois cuidados.** Verificado em incidente durante a implementação: com `CI=true` no ambiente, o pnpm 11 considerou o `node_modules` desatualizado ao ver o `package.json` do módulo gerado, rodou `install` por conta própria dentro do worktree e, através do symlink, reescreveu os links do `node_modules` real para o virtual store do diretório temporário — removido junto com o worktree, deixando a raiz sem `nx`. O gatilho é o `.pnpm-workspace-state-v1.json`, que guarda o caminho absoluto de cada projeto: no worktree nenhum bate, então o pnpm reinstalaria em toda execução — inclusive no CI, onde o runner sempre define `CI=true`. Por isso a prova exporta `pnpm_config_verify_deps_before_run=false` (a chave é `pnpm_config_*`, não `npm_config_*` — conferido em `pnpm.mjs`), nunca define `CI` por conta própria, e trata `readlink -f node_modules/nx` fora da raiz do repositório como reprovação: o gate acusa a corrupção em vez de escondê-la. Alternativa descartada: `pnpm install --offline` dentro do worktree, que evita o symlink mas custa dezenas de segundos por fase e duplica o virtual store.
- **`blocks` parcial precisa fechar as dependências entre blocos.** `domain,app` aborta nomeando `port`, `application` e `provider`; `boundedContext` ganhou `pattern` no schema, o que faz o Nx recusar valor com aspas, barra ou quebra de linha antes de a fábrica rodar; `directory` absoluto ou com `..` aborta. Toda validação corre antes do primeiro `tree.write`.
- **`go.mod` sem `require`.** Os módulos gerados são workspace-only: a resolução é do `go.work` e nenhum `go.sum` nasce — o que também mantém a prova legível, já que um `go.sum` novo apareceria como arquivo não rastreado e não no `git diff`.

**Dívida registrada** (fora do escopo desta sub-spec):

- Geração do composition root: `cmd/` com `--role api|relay|consumer` continua fora, e cabear processo segue sendo copiar `dmpf-reference` à mão. É matéria do golden path (sub-spec 3).
- `require` externos quando um módulo gerado sair do workspace (publicação independente); hoje a ausência de `require` presume `go.work`.

## Addendum — 2026-09-12 (harness de bounded contexts da sub-spec 2h)

O addendum anterior decidiu que o generator entrega só o esqueleto e o código de
negócio vem de um agente. A `SPEC-VDP9XX65` realizou esse harness e o exercitou
produzindo `bookings`, o primeiro contexto fora de `dmpf-kernel`. O que o
exercício decidiu, além do que já estava escrito:

- **A prova do harness tem duas fases, e só uma roda sem LLM.** `self-test`
  sabota `shared_kernel_units` sobre o golden commitado, num worktree
  descartável, e exige que o verificador reprove com `DMPF-D002` apontando
  `<ctx>/domain` — prova que o gate normativo ainda morde, sem custo de agente.
  `regen` apaga o golden, aciona o agente sobre a mesma spec e roda os gates
  sobre o resultado; exige LLM, credenciais e tempo, e por isso **não roda no
  CI**. O CI prova o golden como qualquer outro módulo.
- **A garantia é pelos gates, nunca por bytes.** Dois runs do agente divergem em
  forma sobre a mesma spec, e isso é aceito: a prova reporta a divergência como
  informação e reprova só quando um gate reprova. Exigir bytes idênticos tornaria
  o harness inútil na primeira mudança de estilo do modelo.
- **A porta de consulta pertence ao bloco `port`, mesmo quando os genéricos
  bastam para o resto.** Os genéricos do kernel (`Repository[ID,S]`, `Outbox`,
  `Reader[ID,S]`, `UnitOfWork[R]`) expressam quase toda a fronteira por
  instanciação; o que não expressam é a consulta que atravessa uma relação. O
  agente a declarou no `application`, onde é consumida — idiomático em Go,
  incoerente com a matriz de blocos: o resultado compila, passa nos gates e
  deixa a unidade `<ctx>/ports` declarada e sem superfície. Bloco vazio num
  golden que serve para ensinar é o pior tipo de exemplo.
- **Suíte sem `BaselineStore` declara o shared kernel na `Input`.** `FIT-03` põe
  o `fitness` e o `selfcheck` fora do modelo de confiança do baseline, o que
  também os deixa sem a designação de shared kernel. Enquanto só existia o
  kernel, a omissão não aparecia; o primeiro contexto a consumi-lo faz brotar um
  `DMPF-D002` por aresta. As duas suítes passam a ler **apenas a designação** do
  baseline governado, sem adotar o store — a promessa de `FIT-03` fica intacta e
  a lista não é duplicada em teste, onde sairia do lugar.
- **Código gerado não se conserta por edição.** O `rawDesc` de um `.pb.go` é o
  descritor do `.proto` serializado, com prefixos de comprimento: alteração
  textual que mude o tamanho de um campo corrompe o descritor e o pacote entra
  em `panic` no `init()`. Qualquer renomeação que alcance `gen/go` se conclui
  regerando pelo rito Buf. A regra já existia como norma; agora tem a razão
  mecânica registrada.
- **A prova envelhece com o golden.** O `dmpf-harness-check.sh` carrega o layout
  do contexto em `MODULOS` e nos globs de comparação. Depois da virada para
  pasta por contexto (addendum do ADR-030), ele seguiu procurando
  `<ctx>-<bloco>` e casando o diagnóstico por `<ctx>-domain`, quando o
  verificador imprime `<ctx>/domain`. Foi o próprio `self-test` que apontou os
  dois — o que é o argumento para rodá-lo a cada mudança de forma.

O guia operacional do fluxo está em `docs/guides/dmpf-composicao.md`; o catálogo
do que já deu errado, em
`.agents/skills/dmpf-bounded-context/references/armadilhas.md`.

## Referências

- `docs/specs/SPEC-8HWBWJCB-dmpf-sdk-referencia-bom.md` — guarda-chuva do KRN-12.
- `docs/specs/SPEC-538MS2D4-dmpf-bom-validador-escape-hatch.md` — sub-spec 3: BOM, validador e escape hatch.
- `docs/dmpf/governanca-bom-pilotos.md` (FND-10) — §4 (`BOM-01` a `BOM-10`), §5.1 (`GOV-30` a `GOV-36`).
- `docs/specs/SPEC-6QT9SBAS-dmpf-reference-composition-root.md` — spec desta entrega.
- `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05) — ENV-08, `causationid` = `id` quando a mensagem inicia a cadeia.
- `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §3.2, §5.4, §6.3, UOW-11, INB-08.
- `docs/dmpf/politicas-transporte.md` (FND-06) — §9 (RST-02, RST-04), §11 (ponte por `Sink`).
- `docs/adr/015-*.md` — provider concreto só no bloco `app`.
- `docs/adr/035-realizacao-postgres-da-outbox.md` e `docs/adr/038-*.md` — os ADRs cuja lacuna de `metadata` esta entrega fecha (ver os addenda).
- `docs/adr/039-*.md` — ponte por `Sink`, catálogo de canal, admissão.
- `apps/backend/dmpf-reference/README.md` — como rodar os três papéis localmente.

## Addendum — 2026-09-12 (BOM, validador e escape hatch da sub-spec 3)

A `SPEC-538MS2D4` entregou o modelo `dmpf/bom@1`, o validador `dmpf-bom` e a
admissão mecânica das exceções, comum ao verificador (E1) e ao BOM (E2/E3). O
que a execução decidiu:

- **`BOM-03` e `BOM-06` coexistem pela referência resolvida.** `BOM-03` exige
  `version` exata em toda entrada; `BOM-06` proíbe duplicar o registro
  autoritativo — lida isolada, cada regra anula a outra. A entrada declara as
  duas coisas, `version` e `registry_ref {file, selector}`, e o validador resolve
  a referência e reprova divergência (`DMPF-B007`). A duplicação continua
  existindo, mas deixa de poder divergir em silêncio, que é o dano que `BOM-06`
  nomeia. A referência é obrigatória para `runtime`, `generator` e o slot de
  `BOM-10`. Essa é uma leitura desta entrega para as matérias da tabela de
  `BOM-06`, cujo registro de stacks (`BUF-07`) ainda não existe como arquivo. A
  referência precisa ser o registro da própria `identity`, e o `go.mod` citado
  precisa ser módulo do `go.work`; faixa no registro só é aceita como `^`.
- **Certificação vencida é erro até o ato que a rebaixa.** `BOM-07` proíbe
  transição por decurso de prazo; `BOM-08` diz que certificação vencida não é
  certificação. A leitura que honra as duas: o validador não rebaixa sozinho e
  não aceita a vencida — `DMPF-B008` reprova até o commit que declara
  `candidata`. Por isso `certificada → candidata` é transição válida na máquina
  do `DMPF-B003`, que só se avalia com `--base`.
- **O contrato da evidência é o `header`.** O validador lê de
  `bom/evidence/<release>/<subject>.json` só `goversion`, `modules` e
  `externals`, e confere o `evidence_digest` contra a cópia commitada, mesmo
  quando o `evidence_uri` aponta para o forge. `compatible_with[].evidence` nomeia
  o subject; o par `{identity, version}` ausente do header é combinação presumida
  (`BOM-04`). É o mínimo que a sub-spec 4 precisa emitir.
- **Exceção recusada não encerra a verificação.** `DMPF-M*` continuam encerrando
  a fase de manifesto; `DMPF-X*` não, porque a recusa só retira a autorização, e
  o `DMPF-E001` que isso produz na aresta precisa aparecer no mesmo relatório. A
  admissão roda uma vez, em `manifest.Validate`, e o `--write-baseline` consome a
  mesma projeção: regravar sobre exceção recusada daria aval ao que o gate
  reprova.
- **Unidade declarada antes do package volta ao baseline depois.** Classificar em
  commit próprio antes do código mantém a história verde commit a commit, mas
  grava `membership` vazio: quando os packages aparecem, o verificador acusa
  `DMPF-T001` e pede uma segunda regravação, também isolada. A ordem é
  normativo, código, normativo.
- **As métricas de `GOV-36` são conferidas, não declaradas.** `vigentes`,
  `renovacoes` e `vencidas_sem_convergencia` derivam de `exceptions[].history`;
  valor declarado divergente reprova (`DMPF-B010`).
- **O code review endureceu a admissão.** A exceção passou a exigir
  `valid_until`, `review_by` e o par `object.unit`/`object.identity`. Data que
  não parseia reprova em `DMPF-X005`, em vez de valer como ausente. Renovar
  exige vigência nova, e `revoked` ou `converged` encerram a exceção. No
  verificador, o dono de um package passou a sair do universo, e `include` de
  outro módulo reprova em `DMPF-M002`: antes, a exceção de uma unidade podia
  autorizar import numa unidade de outro módulo.
- **A evidência de uma combinação é ancorada.** `compatible_with[].evidence` só
  conta quando alguma entrada do BOM prende o subject por `evidence_digest`, e
  os dois lados da combinação precisam constar do mesmo `header`.
- **Um BOM por release, todos no diretório.** Os BOMs antigos ficam em
  `bom/dmpf/`. O gate valida a maior release (`--release latest`) e a compara
  com o mesmo arquivo no base ou, na release nova, com a maior semver de lá; sem
  BOM no base, toda entrada cai na regra de estado inicial. O custo é que um BOM
  antigo não volta a ser validado.

O schema e os códigos estão em `bom/README.md`; o rito de pedir exceção, em
`docs/guides/dmpf-composicao.md` §9.
