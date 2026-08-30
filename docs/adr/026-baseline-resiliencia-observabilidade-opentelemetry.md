# ADR-026: Adotar a baseline de resiliência e observabilidade com OpenTelemetry, retry por conjunção e auditoria separada

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

## Contexto

Um sistema orientado a domínio e mensagens precisa de uma baseline única que
diga, para todo trabalho novo, como uma dependência externa é chamada sob falha
e como cada fluxo é observado. Sem ela, cada equipe fixa timeout, retry, métrica
e log por conta própria, e o acervo diverge em convenções incompatíveis. Fixar
essa baseline colide com o invariante mais central da fundação: a telemetria não
pode viver no domínio nem na porta (RFC §6.2, princípio 11). O lugar mais
conveniente para instrumentar é justamente o proibido, e por isso a baseline
declara, antes de qualquer regra, que o sujeito de toda decisão é `app`,
`provider` ou `application service` — nunca o centro.

Sobre essa restrição pesam trinta obrigações herdadas dos cinco artefatos já
promovidos. Cada precedente que fixou um mecanismo — outbox, inbox, relay,
envelope, transporte, taxonomia de erro — disse que ele é observável e parou
antes de nomear a métrica, o limiar e o alarme. Falta o catálogo, e catálogo não
redefine mecanismo: nomear e dimensionar é desta baseline; a existência e a
semântica do mecanismo permanecem de quem os fixou.

Quatro escolhas substantivas concentram o risco, e cada uma tem um default
errado e sedutor. Adotar a convenção de tracing e métricas sem fixar a versão
deixaria cada serviço migrar sozinho um vocabulário que ainda evolui. Retry
disparado só porque o erro é retentável duplica efeito na operação não
idempotente — FND-07 já advertiu que a retryability não autoriza nem dimensiona a
repetição. O orçamento contado por chamada, e não por execução, multiplica
tentativas entre dependências até o chamador perceber indisponibilidade. E os
limiares ou são inventados universalmente, invadindo o SLO por serviço fora do
escopo, ou não são fixados de todo — enquanto a auditoria, se derivada do
pipeline de observabilidade, herda a amostragem e a retenção curta do log,
contra o requisito regulatório oposto.

## Decisão

Adotar a baseline de resiliência e observabilidade com **sujeito restrito a
`app`, `provider` e `application service`** (`RES-01`): nenhuma regra obriga
`domain` ou `port`, e nenhuma se satisfaz por instrumentação colocada num deles.

- **OpenTelemetry é a convenção, com a versão das *semantic conventions* fixada
  no BOM** (`TRC-03`). Nome de span, atributos e correlação seguem a convenção; a
  migração de versão é mudança governada no BOM (Parte-1 §17.2), não ajuste local.
  O propagador é configurado explicitamente na forma W3C Trace Context, sem
  fallback silencioso para formato proprietário (`TRC-09`).
- **Retry é autorizado se, e somente se, uma conjunção de quatro fatores for
  simultaneamente verdadeira, verificada antes de cada tentativa** (`RES-27`):
  erro retentável, operação idempotente ou efeito conhecidamente ausente,
  orçamento suficiente para a tentativa e o seu backoff, e prazo remanescente
  suficiente. A retryability é condição necessária e não suficiente (`RES-28`), e
  o default fail-closed de `ERR-11` é herdado, nunca invertido (`RES-29`).
- **O orçamento de repetição é por execução, não por chamada** (`RES-30`), com
  default de metade do prazo remanescente na primeira falha; a espera de backoff
  consome orçamento (`RES-31`) e o esgotamento é observável em `MET-28`
  (`RES-36`). Backoff exponencial com jitter e teto de tentativas são
  obrigatórios, e o valor do teto é desta baseline nos dois caminhos, compondo
  com o gesto de cada transporte sem o reabrir (`RES-32`, `RES-33`).
- **O limiar é condicional**: o valor é fixado apenas onde é derivado de
  invariante já normatizada; nos demais casos o catálogo nomeia o owner e o
  parâmetro local, sem fixar o número (`MET-05`). Toda métrica obrigatória
  declara nome, unidade e fórmula (`MET-03`), e todo alarme nomeia o sinal, a
  condição e o procedimento (`MET-06`).
- **A auditoria é canal separado da observabilidade** onde houver requisito
  regulatório, com retenção, controle de acesso e integridade próprios, emitida
  pelo `application service` com sujeito, objeto, ação, desfecho e instante
  (`LOG-13`, `LOG-14`).
- **A amostragem é declarada por classe de tráfego, não por serviço, e o erro é
  sempre amostrado** (`TRC-13`, `TRC-14`).

A redaction de FND-07 (`DAT-22` a `DAT-25`) alcança os pipelines de log, trace e
métrica por igual, cumprida na origem e não no agregador.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Adotar a convenção sem fixar a versão das *semantic conventions* | As convenções de mensageria ainda evoluem; sem a versão fixada, cada serviço migraria sozinho e o trace deixaria de ser comparável entre serviços (`TRC-03`). |
| Padronizar o formato proprietário do backend de telemetria escolhido | Acoplaria a instrumentação a um vendor — cuja escolha está `encaminhada` aos épicos de kernel (§1.4) — e contraria `TRC-09`, que veda fallback silencioso para formato proprietário. |
| Derivar a repetição da retryability isolada | `ERR-12` declara que a retryability «não autoriza, ordena nem dimensiona a repetição»: um `POST` sem chave de idempotência que estoura o prazo é retentável e não idempotente, e repeti-lo duplica o efeito (§4.2 `rationale`; contraprova 2). |
| Deixar a decisão de retry ao critério de cada serviço | Retry generoso, breaker permissivo e admissão ausente não são três decisões independentes, e sim uma retry storm configurada em três lugares (`RES-39`); sem baseline, o acervo reproduz a divergência que ela existe para fechar. |
| Inverter o default de `ERR-11` para «retentar quando não se sabe» | O default fail-closed já resolveu a tensão a favor do fechamento; invertê-lo não é opção de configuração, é violação de M1 (`RES-29`; §4.2 `rationale`). |
| Contar o orçamento de repetição por chamada | Três dependências com três tentativas cada produzem nove tentativas e oito esperas num prazo dimensionado para uma passagem, e o chamador percebe indisponibilidade, não resiliência (§4.3 `rationale`; contraprova 3). |
| Fixar teto de tentativas sem orçamento de tempo | Um teto de contagem não limita o tempo total gasto em tentativas somadas ao backoff, e uma única tentativa longa excede o prazo sem que nada a interrompa (`RES-30`, `RES-31`). |
| Não contar a espera de backoff no orçamento | Contar só o tempo das tentativas torna o orçamento inobservável na única situação em que ele importa — a de muitas falhas seguidas (`RES-31`). |
| Exigir valor de limiar para toda métrica | Produziria números inventados e invadiria o SLO por serviço, que a spec desta entrega exclui — o «latência acima de 300 ms» da contraprova 5 é o caso (§6.1 `rationale`). |
| Não fixar limiar nenhum | Esvaziaria a obrigação de limiares recebida de FND-04, que os pede explicitamente (§6.1 `rationale`). |
| Concentrar auditoria e observabilidade num canal único com retenção longa | Forçaria retenção longa sobre todo o volume operacional, ou auditoria que expira antes do prazo regulatório — as duas exigências não cabem no mesmo canal (§7.4 `rationale`). |
| Derivar a auditoria do pipeline de log | Submeteria a auditoria à amostragem de `LOG-12` e à retenção do log operacional, quando ela exige volume baixo, retenção longa e acesso restrito (`LOG-13`; §7.4 `rationale`). |
| Amostrar de forma uniforme na cabeça | Descartaria justamente o trace raro de erro que interessa ao diagnóstico; por isso o erro é sempre amostrado, por decisão tardia quando preciso (`TRC-14`). |
| Declarar a taxa de amostragem por serviço | A taxa por serviço aplicaria um único percentual a erro e a leitura dentro do mesmo serviço e não garantiria os 100% que o erro exige; a classe de tráfego é a unidade correta (`TRC-13`, `TRC-14`). |

## Consequências

**Positivas:**

- Instrumentar segundo esta baseline não acrescenta nenhum import de
  observabilidade a `domain` nem a `port`: o decorator fica no `provider`
  (`RES-24`), o span da regra de negócio é aberto pelo `application service` que a
  invoca (`TRC-16`) e a trilha do que a regra decidiu é registrada por ele
  (`LOG-11`) — o tipo de domínio segue sem logger, sem tracer e sem meter
  (exemplo 10). Uma porta que declare `retryPolicy` na assinatura reprova ainda
  que nenhuma biblioteca de observabilidade seja importada (contraprova 10).
- Das trinta obrigações herdadas dos cinco artefatos promovidos, nenhuma sai
  `encaminhada` nem `condicionada` — a primeira vez na cadeia em que isso ocorre
  — e cada uma ganha, na matriz de prova, o par de vetores que a torna
  verificável: o caso que satisfaz e o caso que viola (§2.2, §2.3).
- Um `POST` sem chave de idempotência que estoura o prazo deixa de ser repetido,
  ainda que a taxonomia o classifique como retentável (exemplo 2), e a execução
  que acessa três dependências aborta a repetição na terceira em vez de somar
  nove tentativas e oito esperas dentro de um prazo dimensionado para uma
  passagem (exemplo 3, contraprova 3).
- Trace, log e métrica do mesmo evento cruzam pelo `correlation_id` (`TRC-06`) e
  pelas classes de amostragem alinhadas entre trace e log (`LOG-12`), e o
  diagnóstico dispensa dado do titular, porque identificador de mensagem, de
  correlação e de usuário não são labels de métrica (`MET-07`).

**Negativas:**

- **Custo aceito:** separar a auditoria em canal próprio dobra a superfície de
  infraestrutura de telemetria — um segundo destino de coleta, com retenção,
  acesso e integridade próprios. O custo é assumido porque um canal único
  obrigaria a escolher entre reter todo o volume operacional pelo prazo
  regulatório ou expirar a auditoria antes dele (`LOG-13`; §7.4 `rationale`).
- O limiar condicional entrega, para tudo que não deriva de invariante, um owner
  e um parâmetro local em vez de um número pronto: cada serviço ainda carrega o
  trabalho de SLO que esta baseline deliberadamente não faz (`MET-05`; §1.4).
- Os defaults são conservadores de propósito e vão apertar em algum caso
  concreto; cada desvio precisa ser override declarado, versionado e observável
  (`RES-40`), o que troca ajuste local por overhead de governança.

## Referências

- ADR-010 — regra de dependência como função `decide()` sobre seis blocos de
  pertencimento único. É a partição de blocos que dá vocabulário a `RES-01`: o
  sujeito admissível (`app`, `provider`, `application service`) e o proibido
  (`domain`, `port`) são células desse modelo.
- ADR-014 — proibir a aresta `domain → port` e reservar `port` à fronteira.
  Sustenta que a porta declara a operação e não a sua telemetria e que o domínio
  nada emite: é onde `RES-24`, `TRC-16` e `LOG-11` ancoram a proibição.
- ADR-015 — capabilities externas por bloco, com default deny e allowlist por
  entrypoint. `observability` é uma dessas capabilities, e ela tem regra própria
  naquele ADR: é proibida em `domain` e em `port` ainda que a biblioteca de
  logging seja tecnicamente pura, ficando com adapters, pipelines e providers. É
  essa proibição que impede satisfazer qualquer regra desta baseline
  instrumentando uma porta de assinatura tecnicamente pura.
- ADR-025 — Kafka como transporte-alvo do assíncrono e SNS/SQS normatizado. O
  teto de tentativas desta baseline compõe com o gesto de cada canal
  (`maxReceiveCount` no SQS, política do canal em Kafka) sem o reabrir, e a
  reexecução por redelivery se apoia nesse transporte (`RES-33`, `RES-26`).
- Artefato de origem — `docs/dmpf/resiliencia-observabilidade.md` (FND-08): §3
  (limites por dependência), §4 (retry, orçamento, backoff e degradação), §5
  (tracing), §6 (métricas), §7 (logging e auditoria), sob a regra de sujeito de
  §1.3 (`RES-01`), que incide sobre todas elas; acionamento em §12.3
  (`ADR-DMPF-Q`).
- SPEC-DBTRMM3X — especificação da série de ADRs do DMPF, que este ADR implementa.
- ARQ-448 — https://lider-cap.atlassian.net/browse/ARQ-448 (FND-11, redação,
  promoção e aceite da série de ADRs do DMPF).
