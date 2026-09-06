# dmpf-provider-sqs-go

Bloco `provider` do kernel DMPF para o transporte normatizado em SNS/SQS
(FND-06 §12, ADR-025), sobre `aws-sdk-go-v2`: envelope no corpo textual
codificado **uma única vez**, grupo e deduplicação FIFO derivados do envelope,
delete por receipt handle depois do commit local, visibilidade estendida por
heartbeat sob o teto de doze horas, e raw message delivery obrigatório no hop
SNS → SQS.

Criado por `KRN-10` (ARQ-529, `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md`).

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `dmpf-kernel/provider-sqs` | `provider` | raiz do módulo |

Dependências externas declaradas (`io.messaging`): `github.com/aws/aws-sdk-go-v2`
(`aws`), `service/sqs` (+`types`), `service/sns` (+`types`) e
`github.com/aws/smithy-go` (só a interface `APIError`, para classificar); e
`go.opentelemetry.io/otel/trace` e `otel/metric` (`observability`). O
`aws-sdk-go-v2/config` entra **só nos testes de integração**
(`LoadDefaultConfig`): em produção a `aws.Config` já resolvida é do
composition root.

## O que o módulo contém

- **`config.go`** — `Config{AWS, Endpoint, InsecureForDevelopmentOnly, Catalog,
  Sheet, Service, Clock, Tracer, Instruments, Logger, Rand}`. `Endpoint` que não
  seja `https://` é recusado sem o opt-out (`ErrTLSRequired`): a assinatura
  SigV4 viaja nos headers. `Channel(destino)` resolve no catálogo
  (TRP-07, ASY-01) e recusa ordenação incoerente com o tipo da fila: FIFO
  (sufixo `.fifo`) ordena por grupo, standard por nada (SQS-04).
- **`body.go`** — `EncodeBody`/`DecodeBody`: Base64 padrão, aplicado uma vez
  (SQS-01, TRP-19). `DecodeBody` reconhece a envoltória de notificação do SNS
  (`Type: Notification`, `TopicArn`, `Message`) e recusa com
  `ErrSNSEnvelopeNotRaw` — a linha não conforme da matriz §5.2 (SQS-02).
- **`derive.go`** — `GroupID(chave)` e `DedupID(source, id, payloadHash)`:
  SHA-256 em hexadecimal, comprimento fixo dentro dos 128 caracteres do
  transporte (SQS-05, SQS-06, SQS-06b); cada campo entra prefixado pelo
  comprimento, para que dois trios distintos nunca compartilhem a pré-imagem.
- **`publisher.go`** — `Publisher.Publish(ctx, destino, bytes)` realiza
  estruturalmente o `Publisher` do relay em SQS: corpo codificado uma vez,
  `MessageGroupId`/`MessageDeduplicationId` só em FIFO, atributo operacional
  `dmpf-published-at` (TRP-18). Antes de enviar: corpo final **mais atributos**
  > 256 KiB → `ErrMessageTooLarge` (SQS-12, que soma os dois; sem claim-check,
  SQS-12b); > 10 atributos → `ErrTooManyAttributes` (SQS-03b). Envio pela composição de RES-22 com o
  `Classifier` do SDK (falha de servidor ou throttle e `net.Error` retentam).
- **`sns.go`** — `NewSNSPublisher(ctx, cfg, api)` **descobre** as assinaturas
  de cada tópico `sns-sqs` do catálogo (`ListSubscriptionsByTopic`, paginado) e
  verifica **na construção** cada assinatura de protocolo `sqs`:
  `RawMessageDelivery == "true"` (`ErrRawDeliveryRequired`, SQS-02), sem
  filtro em tópico FIFO (`ErrFilterOnFifoTopic`, SQS-03c), ao menos uma
  (`ErrNoSubscriptions`); `Publish` com o mesmo corpo, grupo e dedup do SQS.
- **`consumer.go` / `heartbeat.go`** — `Consumer.Run(ctx, api)` toma os slots
  livres (até `Concurrency`) **antes** de cada receive e pede ao broker só esse
  tanto (`MaxNumberOfMessages`), de modo que toda mensagem recebida tem worker
  e heartbeat imediatos — nada espera invisível num buffer. `WaitTime` é
  obrigatório em [1 s, 20 s] (long polling dentro do teto da API) e receive
  que falha espera `Backoff.Next` antes de repetir. `Validate` exige
  `2 × HeartbeatEvery ≤ VisibilityBase`, porque um tick pode consumir um
  intervalo inteiro antes de o próximo ser armado. Cada mensagem:
  `attempt` = `ApproximateReceiveCount` (TRP-52), gravado no contexto
  (`attempt.WithContext`); heartbeat a cada `HeartbeatEvery` estendendo a
  visibilidade para `VisibilityBase`, nunca além do que resta até o teto de
  12 h contadas do recebimento (SQS-08, SQS-08b) — cada tick é limitado pelo
  intervalo, e um tick que falha cancela a tentativa, porque a invisibilidade
  deixou de ser garantida; corpo decodificado uma vez — indecodificável vai
  **cru** ao `Sink`, e o adapter quarentena (INB-10). Erro do `Sink` é sempre
  logado (com o nome do canal, nunca a URL da fila), dizendo se houve gesto;
  retorno sem gesto deixa a visibilidade expirar; panic do `Sink` é tentativa
  sem gesto (`ErrSinkPanicked`), nunca queda do consumer. Não há retry inline: `MaxInlineAttempts != 0` é recusado (SQS-11b)
  — a D3 esgotada é do redrive gerenciado pelo `maxReceiveCount` da fila.
  `MET-11`: utilização = workers ocupados / `Concurrency`; profundidade =
  recebidas ainda não entregues a um worker.
- **`acknowledger.go`** — o gesto terminal por receipt: `Ack` →
  `DeleteMessage` (SQS-09), `Release` → `ChangeMessageVisibility` com o backoff
  em segundos, teto 12 h (SQS-10). Os dois **param e aguardam o heartbeat**
  antes do gesto: nenhuma extensão corre depois do delete nem sobrescreve o
  backoff. Segunda chamada → `ErrAlreadyDisposed` (TRP-27).
- **`dlq.go`** — `DLQ` realiza `dmpfports.Containment` por publicação
  explícita na fila de contenção (SQS-11, D4/R4 e envelope inválido): envelope
  codificado uma vez e intacto, atributos `dmpf-reason`, `dmpf-consumer`,
  `dmpf-message-id`, `dmpf-contained-at`, `dmpf-error` e, quando o contexto
  veio do consumer, `dmpf-attempt` (TRP-52); `dmpf-message-id` e `dmpf-error`,
  que vêm de fora do provider, são cortados em `AttributeValueLimit` (1 KiB) e
  contam no teto de 256 KiB; em FIFO, grupo/dedup derivados do envelope — o
  indecodificável vai no grupo `invalid-envelope`.
  Quem chama deleta só depois do retorno (TRP-30).

### Contrato com o adapter (`dmpf-app`)

O adapter contém em `Delivery.Attempt >= Consumer.MaxAttempts` (D4/R4 e
tentativas esgotadas, por publicação explícita); a fila redireciona em
`maxReceiveCount` (D3 esgotada sem classificação). Os dois mecanismos **não**
se aplicam à mesma disposição (SQS-11b): o composition root declara
`Consumer.MaxAttempts` do adapter **maior** que o `maxReceiveCount` da fila,
para que a contenção explícita só ocorra por classificação (D4/R4/inválido) e
a D3 esgotada fique com o redrive.

## O que o módulo não contém

Provisionamento de filas, tópicos e assinaturas (os testes de integração os
criam para si); claim-check (TRP-21, SQS-12b); retry inline; a identidade de
FND-07; o binding persistido de TRP-09/TRP-46.

## Como rodar os testes localmente

Os unitários rodam sobre `FakeSQS`/`FakeSNS` e `clock.Fake`, com intercalamentos
determinísticos do heartbeat. A `hops_test.go` realiza as linhas SQS e SNS da
matriz FND-06 §5.2 com par positivo e negativo.

Os testes de integração levam a build tag `integration`, exigem
`DMPF_SQS_ENDPOINT` (sem ela fazem `t.Skip` nomeando-a), leem as credenciais
pelo SDK (`AWS_*`) e criam filas e tópicos com sufixo único por execução:

```bash
docker run -d --name floci -p 4566:4566 floci/floci:2.0.1
export DMPF_SQS_ENDPOINT=http://localhost:4566 AWS_REGION=us-east-1 \
  AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test
pnpm nx run dmpf-provider-sqs-go:test-race
```

O floci emula SQS **e** SNS no mesmo endpoint, o que torna o hop SNS → SQS
testável nas duas linhas — com raw delivery (envelope preservado) e sem (a
envoltória, recusada). O `test-race` roda com `cache: false` no Nx e
`-count=1` no `go test`. No CI o floci sobe no job (`ci.yml`, step "Subir
floci").

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test-race,govulncheck -p dmpf-provider-sqs-go
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --base develop
```

O `dmpf-gate-check.sh` não alcança este módulo; o gate autoritativo é o
verificador.

## Referências

- `docs/dmpf/politicas-transporte.md` (FND-06) — §5.2, §6.2 (`TRP-26`,
  `TRP-27`, `TRP-30`, `TRP-52`), §12 (`SQS-01` a `SQS-13`).
- `docs/dmpf/resiliencia-observabilidade.md` (FND-08) — `RES-22`, `MET-11`.
- `docs/adr/025-kafka-transporte-alvo-sns-sqs-acervo.md` — SNS/SQS normatizado.
- `libs/backend/go/dmpf-transport/README.md` — `channel`, `observe`.
- `libs/backend/go/dmpf-app/README.md` — o adapter que realiza o `Sink`.
