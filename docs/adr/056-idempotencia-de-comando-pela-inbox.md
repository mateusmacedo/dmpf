# ADR-056: Fazer da inbox o registro da idempotência de comando, com a chave derivada na borda pública

## Status

Aceito — 2026-09-30. Implementa [SPEC-JJKWG4JP](../specs/SPEC-JJKWG4JP-idempotencia-ponta-a-ponta.md). Emenda o [ADR-044](./044-bff-rest-e-contextos-grpc-de-referencia.md), no retry restrito aos `Find*` e na cadeia de interceptors do contexto, e o [ADR-050](./050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md), na inbox, que passa a guardar entradas de comando.

## Contexto

A inbox protegia só a entrada assíncrona: a mensagem reentregue era classificada por `(consumer_name, message_id)` e não reaplicava o efeito (FND-04 §6). A entrada síncrona não tinha proteção nenhuma. Um cliente que repetia um comando porque a resposta se perdeu recebia outro desfecho, e em parte dos casos outro efeito:

1. **REST.** O BFF exigia `Idempotency-Key` nos `POST`, mas só validava o formato: a chave não chegava a lugar algum que a usasse.
2. **gRPC.** Os contextos recebiam a metadata `idempotency-key` e a ignoravam. A repetição de `AddItem` somava o item de novo; a de `PlaceOrder`, `Reserve`, `Cancel` e `CancelBooking` virava 422, e a de `ReserveBooking`, um 409 de conflito de versão que nenhum retry resolvia.
3. **Retry.** Pelo ADR-044, só os `Find*` eram retentáveis. Um comando que falhava por indisponibilidade não podia ser repetido pelo BFF sem risco de duplicar o efeito.
4. **Consumo e publicação.** A quarentena gravava a mesma mensagem uma vez por contenção, o produtor Kafka não fixava as garantias de escrita e a transação da UoW herdava o isolamento do servidor.

## Decisão

**Todo comando passa pela inbox do próprio contexto, na mesma transação do efeito, identificado pelo tenant e por uma chave que a borda pública deriva do sujeito. Um comando repetido dentro da retenção devolve o desfecho gravado, sem executar de novo.** As regras ficam em FND-04 §7.6, família `IDM`.

1. **A inbox é o registro do comando.** A entrada de comando tem `consumer_name` `<contexto>.commands`, `message_id` `<tenant>/<chave>`, `message_type` igual à operação e `payload_hash` igual ao hex do fingerprint. A `inbox` ganha `outcome` e `expires_at`, anuláveis, e o índice `inbox_consumer_name_expires_at_idx`. A porta `Inbox` ganha `Receipt.WaitUntil`, `Receipt.ExpiresAt`, `Completion.Outcome` e `Reception.Stored`, e os providers ganham `Tx.CommandInbox`. A entrada de comando vencida é tratada como ausente, e a de mensagem nunca vence. Não há tabela nem porta própria para a idempotência.
2. **Chave, escopo e portador.**
   - A chave segue `^[A-Za-z0-9._-]{1,128}$` (`ports.IdempotencyKeyPattern`).
   - O provider embute o tenant no `message_id`. A inbox continua sem coluna nem predicado de tenant: o tenant é parte da chave, não escopo da tabela, e o ADR-050 vale como estava para mensagens.
   - O BFF envia ao contexto `hex(sha256(subject ‖ 0x00 ‖ chave do cliente))`, e o sujeito não atravessa (`CTX-12`, `IDN-02`).
   - A chave viaja num portador próprio do `context.Context` (`ports.WithIdempotencyKey`), fora do `ExecutionContext`, cujos nove campos `CTX-01` fixa.
   - O caso de uso revalida a chave com `ports.ValidIdempotencyKey`, atrás da borda, e recusa a que está fora do formato com `ports.ErrIdempotencyKeyInvalid`.
3. **Fingerprint e desfecho.**
   - `usecase.NewFingerprint` codifica a operação e os campos do comando com tipo e tamanho por campo, sem depender do Protobuf.
   - O digest cobre a operação registrada seguida da codificação canônica: um fingerprint montado sem `NewFingerprint`, ou para outra operação, ainda diverge (R4).
   - O SHA-256 entra pela `IdempotencyPolicy.Digest`, que `app.IdempotencyPolicy(wait, retention)` monta, porque o depguard do bloco `application` não admite `crypto/sha256`.
   - O desfecho, aceito ou recusado, é gravado num envelope versionado; os varints seguem o layout de `encoding/binary`.
   - Um desfecho ilegível é erro, nunca nova execução.
4. **Espera com teto.** O registro espera até `min(agora + IdempotencyWait, prazo − 100ms)`, e o provider limita também pelo prazo real do `context.Context`, menos 100ms, para o `lock_timeout` disparar antes do cancelamento. O `lock_timeout` é de pelo menos 1ms, e o estouro vira `ErrIdempotencyInFlight`.
5. **Borda gRPC.**
   - `kernelgrpc.WithCommands` declara os métodos de comando, e o interceptor de contexto recusa comando sem chave ou com chave inválida, com `InvalidArgument` e o `ErrorInfo` `MISSING_IDEMPOTENCY_KEY` ou `INVALID_IDEMPOTENCY_KEY`.
   - No replay bem-sucedido, a resposta leva o header `idempotent-replayed: true`.
   - O log de chamada registra `idempotency_key` só quando a chave está no formato; fora dele, registra `idempotency_key_invalid`.
   - `kernelgrpc.IdempotencyStatus` mapeia divergência para `FailedPrecondition` (`REUSED_IDEMPOTENCY_KEY`), em andamento para `Aborted` (`IN_FLIGHT_IDEMPOTENCY_KEY`), chave ausente ou fora do formato para `InvalidArgument` (`MISSING_IDEMPOTENCY_KEY` ou `INVALID_IDEMPOTENCY_KEY`) e `ports.ErrAlreadyExists` para `AlreadyExists`, que só o `ReserveBooking` devolve.
   - A cadeia do contexto descrita no ADR-044 é anterior ao log de chamada. A vigente é span → log de chamada → admissão → deadline → contexto, e é o último elo que exige a chave.
6. **Borda REST.**
   - O BFF devolve 422 `reused-idempotency-key`, 409 `in-flight-idempotency-key`, 409 `already-exists` e 400 `missing-idempotency-key` ou `invalid-idempotency-key`.
   - O replay repete status e corpo com `Idempotent-Replayed: true`, exposto no CORS. A marca só sai com status abaixo de 500: um replay que o BFF não consegue responder vira 500 sem ela.
   - O log de acesso registra `idempotency_key` só quando a chave está no formato; fora dele, registra `idempotency_key_invalid`.
   - Os sete comandos passam a `Idempotent: true` com retry só em `Unavailable`, o que emenda o ADR-044.
7. **Classificação de falha.** `DeadlineExceeded`, os SQLSTATE `08*`, `40001`, `40P01` e `57P01`, o erro que o driver declara seguro para repetir (`SafeToRetry()`) e o timeout de rede (`Timeout()`) são transitórios (D3). `Canceled` continua terminal (D4), porque FND-07 §5 declara `Cancelled` não retentável.
8. **Purga no processo dono.**
   - `app.RunPurge` hospeda três laços, em lotes com `FOR UPDATE SKIP LOCKED`: `PurgeExpiredInbox` no `serve-api`, `PurgePublished` no `serve-relay` e `PurgeInbox` no `serve-consumer`.
   - Padrões: idempotência 24h, outbox 168h, inbox 192h, intervalo 15min e lote 1000.
   - O `serve-consumer` recusa iniciar com retenção da inbox menor que a janela de redelivery do canal que consome (`INB-14`).
   - A purga da outbox começa depois do `AssertOwnOutbox`, e o laço recusa relógio, logger ou função de purga nil.
9. **Consumo e publicação.**
   - A quarentena grava uma linha por `(consumer_name, envelope_digest)`. O upgrade preenche o digest e deduplica as linhas antigas só no boot que cria a coluna.
   - O produtor Kafka fixa acks de todas as réplicas em sincronia, com escrita idempotente.
   - A UoW abre a transação em `READ COMMITTED` explícito, qualquer que seja o padrão do servidor.
10. **Schema sem lock exclusivo no boot.** As colunas e os índices novos entram em blocos `DO` guardados por `pg_attribute` e `to_regclass`, e o boot sem mudança de schema não pede `ACCESS EXCLUSIVE`. `envelope_digest` é anulável, para a réplica antiga seguir contendo durante o rollout; o `NOT NULL` fica para um release posterior.

## Implantação

- **Ordem obrigatória:** migração do schema, depois os contextos, depois o BFF. O rollback segue a ordem inversa: um contexto só volta depois do BFF.
- **`MIGRATE=false`** exige a migração aplicada antes do binário novo: sem `outcome` e `expires_at` na `inbox`, todo comando falha.
- **Janela de versões misturadas, aceita:**
  - Com o BFF antigo e o contexto novo, a chave chega crua e escopada só pelo tenant: dois sujeitos do mesmo tenant com a mesma chave de cliente dividem a entrada. Um comando repetido através da troca do BFF executa de novo, porque a chave derivada é outra.
  - Com o BFF novo e um contexto antigo, o retry em `Unavailable` pode duplicar o efeito, porque o contexto antigo ignora a chave. A ordem acima evita esse caso, e o rollback fora de ordem o reabre.

## Desvios da spec

| Spec | Implementado | Motivo |
| ---- | ------------ | ------ |
| `Canceled` como D3 | `Canceled` continua D4 | FND-07 §5 declara `Cancelled` não retentável; a spec foi emendada |
| Atributo `idempotency.outcome` | `dmpf.idempotency_outcome` | Os atributos próprios do kernel levam o prefixo `dmpf.` |
| Laço de purga num package `app/purge` | `app.RunPurge`, no package `app` | Uma unidade nova exigiria regravar o baseline, que é ato humano de classificação |
| SHA-256 calculado no `application` | Digest injetado pela política, montada no `app` | O depguard do bloco `application` não admite `crypto/sha256`, e o gate não foi afrouxado |
| Requisito novo no context-check | Nenhum | A inbox já é capability do conjunto `KERNEL` |

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Tabela `idempotency` própria, ao lado da inbox | Duplicaria a porta de classificação, a serialização na chave e a regra de transação commitada, e a primeira divergência entre as duas seria um modo de falha sem cenário no catálogo |
| Coluna `tenant_id` na inbox | Reabriria o ADR-050 para todas as entradas, quando só a de comando precisa do tenant, e ele cabe na chave |
| Chave como campo do `ExecutionContext` | `CTX-01` fixa os nove campos; a chave só existe para comandos e não se propaga |
| Fingerprint sobre a serialização Protobuf | A serialização não é canônica entre releases, e a mesma chamada poderia divergir de si mesma |
| `NOWAIT` ou advisory lock na espera | O `NOWAIT` devolve 409 a todo retry rápido que poderia virar replay; o advisory lock é um segundo mecanismo de serialização |
| Gravar só o aceite | O retry de uma recusa poderia virar aceite depois que o estado mudasse |
| Acrescentar `crypto/sha256` à allowlist do bloco `application` | É mudança de gate, fora do escopo de quem escreve o caso de uso |

## Consequências

**Positivas:**

- Um cliente pode repetir qualquer comando com a mesma chave, em qualquer ponto da cadeia, e recebe o mesmo desfecho, sem efeito novo.
- O BFF retenta comandos por indisponibilidade sem risco de duplicar o efeito.
- A inbox segue como o único mecanismo de deduplicação do kernel, para mensagens e comandos.
- A mesma mensagem contida várias vezes gera uma linha na quarentena.

**Negativas:**

- Todo comando escreve uma linha na inbox, e cada contexto que serve comando passa a migrar a capability `Inbox`.
- A proteção dura a retenção: um comando repetido depois de 24h executa como novo, e só a idempotência de efeito (`GAR-03`, `GAR-10`) o protege.
- **Custo aceito:** cada contexto declara um codec por resposta de comando e os campos do fingerprint à mão, e um campo esquecido no fingerprint torna iguais comandos que deveriam divergir.
- **Custo aceito:** o codec de cada resposta e o fingerprint não têm versão de layout. Mudar os campos de um deles dentro da retenção faz o replay de uma entrada antiga falhar como ilegível, ou o comando repetido virar 422 `reused-idempotency-key`; a mudança espera a retenção ou aceita esses erros por 24h.
- **Corrida aceita no vencimento:** a entrada vence no `expires_at` que o relógio da réplica gravou. Um comando repetido no instante do vencimento, ou avaliado por réplica com relógio adiantado, executa de novo.
