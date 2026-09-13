# ADR-036: Realizar a classificação de recepção em Go como tipo fechado com `Match` exaustivo e `Pending` só sob R1

## Status

Aceito — 2026-09-05. Implementa SPEC-ANZX2WPG.

## Contexto

O FND-04 (`docs/dmpf/uow-inbox-outbox.md`) fixa a porta de inbox em duas
operações na mesma transação — `registrar` devolve a classificação da recepção
e `concluir` fixa o estado terminal (§6.2) — e cerca a porta com propriedades
que o compilador de Go não expressa sozinho: o retorno é uma união de quatro
casos (`INB-04`), `concluir` é obrigatória sob R1 e proibida fora dele
(`INB-05`), a operação serializa na chave sob concorrência e a classificação é
sempre de transação commitada (`INB-06`, `INB-18`), e a espera tem teto cujo
estouro é falha transitória, não erro terminal (`INB-17`). O artefato diz que a
stack com união exaustiva detecta pela assinatura o consumidor que não trata um
caso; Go não tem essa união, e o ADR-032 já havia respondido à mesma lacuna para
o desfecho da UPR com um tipo concreto de construtores fechados.

O ticket ARQ-526 enunciava a porta como `registrar(consumer_name, message_id,
payload_hash)`, mas o schema de §6.1 exige `message_type` e `received_at` na
mesma linha — a assinatura de três escalares não os carrega. O schema também
exige `status NOT NULL` com dois valores terminais (`INB-02`), e a linha nasce
em `registrar`, quando o desfecho ainda não é conhecido.

O ADR-034 deixou a UoW pronta e o ADR-035 a realizou sobre PostgreSQL. Faltava o
lado do consumo: onde vive o consumer adapter, já que ele precisa de `contract`
para validar o envelope e hashear o payload e de `application` para invocar o
caso de uso — e `application → contract` (célula 12) e `provider → application`
(célula 26) são proibidas e têm vetor negativo no CI. E faltava decidir como o
teto de `INB-17` é realizado: a pesquisa mostrou que o handler padrão de
contexto do pgx v5 fecha a conexão quando o `context` expira, o que agrava o
esgotamento do pool no cenário exato que a regra existe para conter.

## Decisão

**A classificação de recepção é um tipo concreto fechado, e o ramificador exige
os quatro casos.** `dmpfports.Reception` tem construtores exportados —
`FirstReception(Pending)`, `ProcessedReception()`, `RejectedReception()`,
`CollisionReception()` — e um único método de consumo, `Match(first, processed,
rejected, collision)`, que panica se qualquer ramo for `nil`. Acrescentar uma
quinta classificação quebra a compilação de todo consumidor, que é a propriedade
que §6.2 pede à stack. É o mesmo movimento do ADR-032, com uma diferença: os
construtores são exportados porque as realizações vivem em outros módulos.

**`Pending` só existe no ramo R1, e sair dele sem `Complete` é erro ruidoso.** O
`Match` entrega o `Pending` apenas à função `first`; sob R2, R3 e R4 não há
caminho de tipo que alcance `Complete` (`INB-05`, imposto pelo compilador). A
revisão externa apontou o que a compilação **não** prova: que `Complete` foi
chamado dentro de R1. Por isso `Pending` expõe `Completed()`, e `Match` devolve
`ErrPendingNotCompleted` quando `first` retorna `nil` sem tê-lo chamado — a
transação desfaz em vez de commitar em silêncio uma linha provisória.

**`Register` recebe um `Receipt`, não três escalares.** `Receipt{Consumer,
MessageID, MessageType, PayloadHash, ReceivedAt}` carrega o que o schema de §6.1
exige e o ticket omitia. Os instantes — `ReceivedAt` e `Completion.At` — são
autorados pelo application service a partir do `Clock`, nunca pelo provider,
pela mesma razão do ADR-034: o bloco `port` não conhece `time`, e a fronteira
transporta `Instant` como inteiro.

**Entre `Register` e `Complete` a linha existe com `status` provisório, dentro
da transação.** `status NOT NULL` com `CHECK` de dois valores obriga um valor no
`INSERT`, antes de o desfecho existir. O provider grava `'processed'` e
`processed_at = received_at` como provisórios e `Complete` os sobrescreve na
mesma transação; ninguém observa o provisório porque a classificação de uma
concorrente só é devolvida depois do commit (`INB-18`). Relaxar o `CHECK` para
admitir `NULL` reintroduziria no schema exatamente o estado intermediário que
`INB-02` remove.

**O teto de espera é `SET LOCAL lock_timeout`; o `context` do chamador é a rede.**
A spec previa a ordem inversa. O handler padrão de contexto do pgx
(`DeadlineContextWatcherHandler`, delay zero) fecha a conexão ao expirar o
`context`; no cenário de `INB-17` — duplicatas empilhando sob visibility
timeout menor que o processamento — isso descarta conexões do pool em vez de
conter o esgotamento. `lock_timeout` aborta só o statement no servidor, a
conexão continua viva e a transação cai em estado abortado, que o `Within` já
trata com rollback. O spike confirmou: a espera da inserção especulativa do
`ON CONFLICT DO NOTHING` por transação concorrente é interrompida com SQLSTATE
`55P03` em ~319 ms para um teto de 300 ms, e a segunda transação comprovadamente
bloqueou até então. O provider traduz `55P03` para `dmpfports.ErrRegisterTimeout`
com dois `%w`, como `ErrDuplicateMessage` faz com `23505`.

**O consumer adapter vive em módulo próprio do bloco `app`.** `dmpf-app` é a
primeira unidade `app` do workspace, e não por preferência: só a linha `app` da
matriz de blocos permite as arestas para `contract` e para `application` ao
mesmo tempo. A composition root do consumo, que antes existia apenas dentro de
um `_test.go` do provider — fora do universo do verificador —, passa a ser
código de produção verificado.

**O adapter recebe os bytes brutos e nunca resserializa.** `Delivery{Raw []byte,
Attempt int}`; a decodificação fica em `contract`, por `envelope.Unmarshal(raw)`
e `envelope.Unpack(env, msg)` — simétricos a `Pack` —, de modo que o `dmpf-app`
não importa `google.golang.org/protobuf` e o seu manifesto declara
`external: []` com verdade. Toda contenção grava `Raw`: um re-marshal do
`CloudEvent` decodificado descartaria campos desconhecidos e quebraria a
identidade byte a byte de `GAR-07`. O teste de `GAR-07` anexa um campo
desconhecido ao `Raw` justamente para que um re-marshal falhe.

**Quarantine é realizada; DLQ é declarada.** `dmpfports.Containment` é porta;
`dmpfpostgres.NewQuarantine(pool)` a realiza fora de qualquer UoW, com envelope
`bytea` e erro sanitizado. A DLQ é destino terminal do transporte e depende de
broker — é do KRN-10. O mapeamento situação → mecanismo de `GAR-11` é dado
revisável em código (`dmpfapp.ContainmentMap`), conferido por teste de
totalidade, e o `Consume` o consulta antes de conter. Os sinais de `GAR-12`
saem da própria tabela, por `GROUP BY reason`: R4 e R1×D4 são `reason`s, não
contadores em memória.

**O efeito de broker é uma porta de duas operações, aplicada depois do
desfecho.** `dmpfports.Acknowledger{Ack, Release}` vive em `port` para que o
KRN-10 a realize de qualquer bloco. D1, D2, R2 e R3 confirmam; D3 libera para
redelivery, ou contém como `attempts-exhausted` quando `Attempt` alcança o
`MaxAttempts` declarado pelo chamador (`GAR-08`); D4 e R4 contêm e então
confirmam; envelope inválido é contido antes de qualquer classificação
(`INB-10`). Se a quarantine falhar, nada é confirmado: contida ou nada. O teste
de `INB-08` lê a inbox de outra conexão no instante do `Ack` e exige a linha
commitada.

**A taxonomia de erros é consumida, não declarada.** O ticket previa declará-la;
FND-07 §5.3 já a fixou e `ERR-11` fecha o placeholder de `INB-09`.
`dmpfapplication.Failure{Category, retryable}` transporta a retryability
resolvida na classificação (`MAP-07`) e `Classify` deriva a disposição
fail-closed: `Failure` pela sua retryability, `ErrRegisterTimeout` → R1×D3,
`context.DeadlineExceeded`/`Canceled` → R1×D4 (`CTX-23`), tudo o mais →
`Unexpected` → R1×D4 (`ERR-11`, `ERR-24`). O único predicado declarado no
exemplo é o do `Conflict`: `ErrVersionConflict` no consumo é retentável porque a
reexecução relê o estado antes de decidir.

**O consumidor de exemplo é um package próprio, com agregado próprio.**
`reservationsapp` não estende `ordersapp`: consumidor e produtor são papéis
distintos, e misturá-los no mesmo `Resources` faria o KRN-07 alterar o exemplo
do KRN-04. A `Reservation` tem chave natural permanente — o identificador do
pedido — realizada por `ON CONFLICT (order_id) DO NOTHING` no repositório, e é
isso que torna o vetor V32 executável: reentrega com `message_id` novo é R1 pela
inbox, e são duas defesas do domínio que impedem o efeito de dobrar: a regra do
agregado (`already-reserved`, que o teste exerce e devolve R1×D2) e o `ON
CONFLICT (order_id) DO NOTHING` do repositório, que só atua sob concorrência
real (`GAR-04`, `GAR-10`).

**Dois módulos com teste de banco não rodam `test-race` em paralelo.** O CI usa
`--parallel=3`, e cada harness faz `TRUNCATE` das mesmas tabelas; isolados, os
dois módulos passam, juntos interferem. O `test-race` do `dmpf-app` declara
`dependsOn` sobre o do provider e o Nx os sequencia. É o custo de compartilhar o
Postgres do job, aceito porque um banco por módulo exigiria mudar o CI para
cada módulo novo.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| `Register` e `Complete` como duas operações independentes na interface | `INB-05` viraria regra de revisão: `Complete` seria chamável sob R2, R3 e R4. |
| `Register` recebendo um callback que devolve o `Status` | Mistura o eixo 1 com o eixo 2 dentro da porta e tira do service o passo 5 como ponto único de ramificação (§6.3). |
| `status` admitindo `NULL` entre `Register` e `Complete` | Reintroduz no schema o estado intermediário que `INB-02` remove; o provisório dentro da transação é invisível por `INB-18`, o `NULL` seria visível para sempre. |
| `context.WithTimeout` como primeira linha do teto | O pgx fecha a conexão ao expirar o `context`; sob pressão isso esgota o pool em vez de o conter. |
| `CancelRequestContextWatcherHandler` no pool | Configuração global que muda o comportamento de todo o módulo; o problema é resolvido no servidor por `lock_timeout`. Fica como escoteiro para KRN-09/10. |
| Adapter dentro de `dmpf-application` ou de `dmpf-provider-postgres` | Células 12 e 26 da matriz, ambas proibidas e provadas por `tools/dmpf-cell-check.sh`. |
| `payloadhash` movido para `dmpf-ports` para a `application` hashear | Viola a autoria do `payload_hash` (ANC-03) e desfaz a razão de o KRN-05 existir. |
| Preservar o envelope por `proto.Marshal` do `CloudEvent` decodificado | Não é byte-idêntico ao transportado (campos desconhecidos, ordem, produtores heterogêneos) — reabriria no consumo o que o ADR-021 fechou na escrita. |
| Estender `ordersapp.Resources` com `Inbox` e `Reservations` | Mistura produtor e consumidor e altera o exemplo do KRN-04/06. |
| Contenção em memória ou em log | Falha `GAR-07` (nada preservado) e `GAR-12` (nada consultável). |
| Adapter que só devolve a disposição e deixa o efeito ao transporte | `INB-08` (ordem) ficaria sem teste e `GAR-08` sem realização. |
| Interface `Retryable() bool` sem categoria | Perde a `Category`, que `last_error` e a quarantine precisam (`ERR-20`). |
| Realizar a DLQ nesta entrega | Depende de broker; o mecanismo é declarado e encaminhado ao KRN-10, como `GAR-11` admite. |
| `pg_advisory_lock` nos harnesses para serializar módulos | Resolveria o mesmo problema tocando quatro arquivos de teste; o `dependsOn` faz o mesmo em uma linha de configuração e deixa a intenção visível no grafo do Nx. |

## Consequências

**Positivas:**

- O KRN-10 recebe a porta `Acknowledger` e o adapter prontos: liga o gesto
  concreto de ACK, `nack` e commit de offset aos efeitos que a tabela de §6.4
  fixa, e realiza a DLQ onde o mapeamento já a declara.
- O KRN-08 recebe registros de outbox derivados do consumo, com bytes
  congelados, para drenar.
- A linha `app` da matriz de blocos passa a ter um módulo real no `develop`, e
  o verificador registra cinco unidades novas no baseline.
- V32 deixa de ser prosa: o teste de reentrega com `message_id` novo roda contra
  Postgres e falharia sem a chave natural.
- O `payload_hash` da outbox, redundante desde o ADR-035, ganha o seu leitor: a
  inbox o compara para separar redelivery legítima de colisão de identificador.

**Negativas:**

- **Custo aceito:** a linha provisória de `dmpf_inbox` exige disciplina do
  service — `Complete` em todo caminho de R1. A disciplina é imposta por
  `ErrPendingNotCompleted`, não por revisão.
- O teto de espera e o limite de tentativas são declarados pelo chamador, e o
  kernel não oferece default: um consumidor que não os declare roda sem teto e
  sem limite. Os valores são de FND-08.
- Réplicas do mesmo consumidor em stacks diferentes podem produzir hashes
  distintos para o mesmo conteúdo enquanto FND-05 não fixar a canonicalização;
  a reentrega roteada para a outra stack vira R4. É a consequência que FND-04
  §6.5 nomeia e não é resolvida aqui.
- Dois módulos com teste de banco compartilham o Postgres do job; um terceiro
  precisará declarar a mesma dependência, ou o CI voltará a interferir.

Nada aqui declara ou sugere entrega exactly-once fim a fim. A garantia é
at-least-once com efeitos efetivamente idempotentes; a inbox absorve a
redelivery e a chave natural do domínio sustenta o resto (`GAR-01`, `GAR-04`).

## Referências

- `docs/specs/SPEC-ANZX2WPG-dmpf-inbox-consumo.md` — spec desta entrega.
- `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §6.1 a §6.6, §7.1, §7.2, §7.4.
- `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — §5.3, §5.4, §6.2.
- `docs/adr/032-realizacao-go-do-desfecho-da-upr.md` — o precedente de forma.
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira em que a inbox entra.
- `docs/adr/035-realizacao-postgres-da-outbox.md` — a outbox que o consumo deriva.
- `libs/backend/go/dmpf-app/README.md` — o adapter e as tabelas de efeito e de contenção.
- `libs/backend/go/dmpf-provider-postgres/README.md` — como rodar os testes de banco.
