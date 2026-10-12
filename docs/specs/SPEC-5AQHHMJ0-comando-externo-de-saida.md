---
id: SPEC-5AQHHMJ0
slug: comando-externo-de-saida
title: DMPF — Ciclo de comando externo de saída: chave idempotente, desfecho incerto, consulta de estado e reconciliação
stage: backlog
priority: P1
depends_on: [SPEC-E15TBHCD, SPEC-JJKWG4JP, SPEC-NYD18TGD]
ticket_url: https://linear.app/mmda/issue/DEVS-96/spec-5aqhhmj0-dmpf-ciclo-de-comando-externo-de-saida-chave-idempotente
subtask_urls: []
created: 2026-10-12
---

# SPEC-5AQHHMJ0: Ciclo de comando externo de saída

## Resumo

Dar ao kernel o ciclo de um comando que o sistema envia a um parceiro externo:
uma chave idempotente de saída derivada da intenção e estável entre tentativas,
reenvios e reentregas; a classificação do desfecho, com o desfecho incerto como
classe própria; as transições para a consulta de estado, quando o parceiro a
oferece, ou para a reconciliação, quando não oferece; e o fake e as cláusulas de
teste que cobrem toda a taxonomia. Hoje o kernel diz quando não repetir uma
chamada de saída e não diz o que fazer quando o efeito é incerto. A spec nasce do
consumidor externo antecipa-market, SPEC-GXGASP24 (DEVS-76), extensão K5, que
integra registradoras e parceiros cuja resposta pode se perder.

A entrega segue três ondas, e cada onda é um PR.

## Contexto

### Problema

- **A entrada tem norma; a saída tem só o lado negativo.** O ADR-056 e a família
  `IDM` (FND-04 §7.6) fazem da inbox o registro do comando recebido. Para a
  saída, `RST-02` (FND-06 §9) veda o retry de `POST` sem chave, e `RES-25` a
  `RES-28` dizem quando não repetir. `Operation.EffectAbsent`
  (`libs/backend/go/observability/retry/evaluate.go:19-21`, `:127-132`;
  `observability/resilience/operation.go:33-37`) é booleano: falso quer dizer
  "o efeito pode existir", e o kernel não tem onde registrar essa dúvida nem
  como resolvê-la.
- **Não há porta de chamada.** `ports/outbox.go:37-39` declara que o bloco não tem
  porta de publicação. O guia prescreve intenção na outbox e callback, com a chamada
  fora da UoW (`UOW-07`, `UOW-08`), e afirma que "a saída é não fazer chamada
  nenhuma" (`docs/guides/dmpf-implementation.md:583-597`). O desenho vale quando o
  parceiro consome o contrato e responde por callback; não cobre o parceiro que só
  expõe requisição e resposta, em que alguém precisa fazer a chamada.
- **A chave de saída não é estável.** O `http.Client` sorteia uma chave de 128
  bits por `Do` quando o chamador não informa (`http/client.go:147-150`,
  `:248-250`): ela vale para as tentativas de uma chamada e muda na seguinte. O
  `app.Attempt.RequestID` também é sorteado por tentativa (`app/attempt.go:14-19`).
  Uma reentrega, ou a reemissão de um registro que terminou em `failed`
  (`OBX-06`), produz chave nova e, no parceiro, efeito duplicado. O guia resolve
  o caso com um identificador da intenção no payload
  (`dmpf-implementation.md:601-626`), sem mecanismo no kernel. `IDM-10` veda
  propagar a chave de comando de entrada.
- **O `net/http` pode reenviar sozinho.** Em erro de rede sobre conexão
  reaproveitada, o `Transport` reenvia o pedido que traz o cabeçalho
  `Idempotency-Key` (https://pkg.go.dev/net/http#Transport). Uma tentativa do kernel
  pode chegar duas vezes ao parceiro, e só a chave estável a torna inócua.
- **O desfecho não carrega o que se sabe do efeito.** `http.Classifier` trata
  `net.Error` como retentável e prazo expirado como não retentável
  (`http/classifier.go:33-46`), sem separar o pedido que não saiu do pedido escrito
  sem resposta. As onze categorias do FND-07 (`application/failure.go:12-28`)
  descrevem a falha do dependente, não o efeito.
- **O testkit não tem parceiro.** `serviceskit.Faults` injeta falha de `Load`,
  `Save` e `Enqueue` (`testkit/serviceskit/faults.go:9-16`). `KIT-01` fixa um kit
  por camada, então o parceiro roteirizado entra como fake do `KIT-03` e a
  conformidade do adaptador como cláusulas do `KIT-04`, sem sexto kit.

### O que o consumidor externo precisa

O consumidor envia comandos a registradoras e a outros parceiros por
adaptadores. Cada adaptador traduz os códigos de falha e de incerteza do
parceiro para oito classes, às quais o Design acrescenta `Accepted` (nove no
total):

- falha de transporte antes de o pedido chegar, que se retenta com a mesma chave;
- parceiro indisponível ou limitando, que se retenta com espera crescente;
- rejeição final por validação, por regra de negócio ou por autorização, que não
  se retenta;
- pedido enviado sem resposta, e resposta recebida que não se interpreta, que
  exigem consultar o estado;
- resposta fora do contrato, que suspende a operação.

Código desconhecido do parceiro vale como resposta não interpretável, nunca como
retentável. Nem todo parceiro oferece consulta de estado: quando falta, o
timeout perde a recuperação automática e vira caso de reconciliação. O parceiro
também pode reenviar a mesma notificação mais de uma vez. O consumidor mantém a
própria máquina de estados de negócio; do kernel precisa da chave, da
classificação, das transições e dos duplos de teste.

### Não verificado

- **Escrita do pedido no transporte.** `httptrace.ClientTrace.WroteRequest`
  existe (https://pkg.go.dev/net/http/httptrace#ClientTrace), mas não se
  verificou que o sinal sobreviva à composição `otelhttp` e `resendCounter` do
  cliente nem em HTTP/2. Sem ele, o provider HTTP não separa "não saiu" de
  "saiu sem resposta" (requisito de investigação da Onda 2).
- **Prazo da chamada frente ao ciclo de consumo.** `TRP-48` veda trabalho
  destacado do ciclo, e o cancelamento por rebalance interrompe a chamada em
  voo, que é exatamente o caso de desfecho incerto (requisito de investigação da
  Onda 2).
- **Capacidades dos parceiros.** Semântica de chave, consulta de estado e janela
  de processamento são fatos de cada parceiro, não do repositório. Entram como
  perfil declarado, nunca como suposição do kernel.

<constraints>
- [P0] A chamada ao parceiro nunca ocorre dentro de UoW: a intenção e o registro da tentativa commitam antes, o resultado commita depois (`UOW-07`, `UOW-08`, `RES-34`).
- [P0] Desfecho incerto nunca vira sucesso nem falha final sem consulta de estado ou reconciliação; sem resolução, o comando permanece incerto e visível.
- [P0] A chave de saída é a mesma em toda tentativa, reenvio, reentrega e reemissão da mesma intenção; só uma decisão de negócio explícita, a nova geração, a troca.
- [P0] A chave de saída nunca é a chave de comando de entrada nem o identificador da tentativa (`IDM-10`).
- [P0] Classe não reconhecida resolve para desfecho incerto não retentável, nunca para retentável (`ERR-11`, `RES-29`).
- [P0] O kernel não promete exactly-once com o parceiro (`GAR-01`): sem semântica de chave e sem consulta, o ciclo detecta e reconcilia, não previne.
- [P0] Nenhum texto de regra `normativo` existente muda; as regras novas somam, em subseção marcada como decisão nova (M1 a M4 da RFC).
- [P0] `ports` e `application` não importam rede, relógio nem SDK (`docs/adr/015-capabilities-externas-por-bloco.md`); a chamada vive em `provider`, e o SHA-256 entra por política injetada, como em `IdempotencyPolicy` (`application/idempotent.go:19-26`).
- [P1] Nenhum sexto kit (`KIT-01`): fake em `serviceskit`, cláusulas em `providerkit`.
- [P1] Nenhum nome de parceiro, protocolo de registradora ou formato de arquivo entra no kernel.
- [P1] `node tools/dmpf-verify.mjs`, `conformance` e `modsync --check` seguem verdes em cada onda.
</constraints>

## Requisitos

### Funcionais

#### Onda 0 — norma, vocabulário e funções puras

- [ ] **[P0] ADR novo**: registrar o ciclo e emendar o ADR-050, que fixa `outbox`, `inbox` e `quarantine` como tabelas de plataforma sem escopo de tenant, ao acrescentar o ledger de tentativas, com o tenant na chave como em `IDM-03`.
  - Trazer a tabela "altera invariante?" no formato de FND-08 §12.4, uma linha por regra nova, e as alternativas descartadas de Decisões técnicas.
- [ ] **[P0] Regras ERR novas** (FND-07, subseção nova depois de §5.8, a partir de `ERR-29`): estender a tradução de `ERR-25` a `ERR-28` ao parceiro externo.
  - A classe de desfecho é fechada e total; cada classe resolve para uma categoria de §5.3 e para uma retryability booleana (`ERR-01`, `ERR-09`).
  - A classe de pedido enviado sem resposta é retentável só quando o pedido levou chave e o perfil declara semântica de chave, o predicado verificável que `ERR-10` exige; sem ele, vale `ERR-11`.
  - O catálogo de onze categorias não ganha linha (`ERR-08`), e o diagnóstico de `ERR-18` mantém os seis elementos: o conhecimento do efeito vive na classe e no registro da tentativa.
- [ ] **[P0] Regras RES novas** (FND-08, subseção nova depois de §4.5, a partir de `RES-41`):
  - O desfecho incerto encerra o retry da chamada sem virar falha final e é o desfecho declarado que `RES-36` exige.
  - O reenvio posterior à chamada é operação nova (`RES-34`) e usa a mesma chave.
  - A espera pela consulta e pela reconciliação tem prazo e dono declarados (`GAR-05`, `GAR-06`).
  - O perfil do parceiro é declaração própria e não campo novo da ficha de `RES-21`.
  - A intenção percorre a outbox, que é o mecanismo do modo `difere` de `RES-37`.
- [ ] **[P0] `IDM-11` e regra `KIT` nova**: `IDM-11` (FND-04 §7.6) diz que a chamada a parceiro externo leva a chave de saída e que a chave de comando de entrada não atravessa, com `IDM-10` intacta. A regra de FND-09 (§8.1) fixa o parceiro roteirizado como fake do `KIT-03` e a conformidade do adaptador como cláusula do `KIT-04`.
- [ ] **[P0] Manifestos do verificador**: atualizar `profiles.json`, `counts.json` e `line-refs.json` em `tools/tests/dmpf-verify/` para as regras novas e para as linhas deslocadas, e fechar com `node tools/dmpf-verify.mjs` sem violação. O verificador ancora regra e referência por número de linha (ADR-058, Consequências).
- [ ] **[P0] Tipos em `ports`**: acrescentar o que o `provider` devolve e o `application` consome, sem tipo de driver, SDK ou wire.
  - `OutboundKey`; `ExternalClass` (as nove classes de Design); `Finding` (`Applied`, `Absent`, `Rejected`, `Pending`, `Inconclusive`).
  - `Observation` com chave, achado, instante da fonte, identificador do provedor, referência externa e identidade própria, para a duplicata ser reconhecida.
  - `ExternalProfile` com semântica de chave (`Native`, `Header`, `None`), eficácia (`OnSubmit`, `ByObservation`), janela de quiescência e `AuthoritativeAbsence`.
  - As portas `ExternalCommand` (`Submit`) e `ExternalState` (`Query`), esta opcional: a ausência é a declaração "sem consulta de estado" e nenhum nil vale como "sim" (`evaluate.go:19-21`).
- [ ] **[P0] Chave de saída**: `application` deriva a `OutboundKey` da codificação canônica de tenant, destino, operação, identidade da intenção e geração, com o digest injetado pela política.
  - A identidade da intenção é cunhada uma vez, no commit da intenção, e viaja no payload do evento de despacho; a reemissão de um registro `failed` carrega a mesma.
  - O perfil fixa alfabeto e comprimento máximo do parceiro; a derivação recusa comprimento abaixo de 128 bits, o mesmo piso da chave sorteada hoje.
  - A geração só avança por decisão explícita do caso de uso, depois de rejeição final corrigida.
- [ ] **[P0] Função de passo pura**: `application.NextStep` recebe classe, perfil, indicação de consulta disponível, tentativa e achado, e devolve `Confirm`, `Await`, `Retry`, `Fail`, `Query`, `Reconcile` ou `Suspend`, como `retry.Evaluate` (`evaluate.go:94-95`): sem relógio, rede ou transação.
  - Classe incerta nunca devolve `Confirm` nem `Fail`; o teste enumera todas as combinações de classe, perfil e achado.

#### Onda 1 — ledger e ciclo

- [ ] **[P0] Ledger de tentativas**: porta `AttemptLedger` (`Begin`, `Finish`, `Stale`) com realização em `memory` e em `postgres`, tabela `external_attempt` em arquivo `.sql` embutido como `inbox.sql` e `outbox.sql` (`postgres/migrate.go:12-15`), declarada como capability do `postgres` ao lado de `Outbox` e `Inbox` (`postgres/migrate.go:22-40`).
  - A linha guarda chave com tenant, número da tentativa, instante de início e de fim, classe e a observação; o estado admite `begun` e `finished`.
  - `Begin` serializa na chave, como `INB-06`; tentativa `begun` não terminada torna a seguinte incerta, não nova.
  - A purga remove só `finished` além da retenção; `begun` não é purgável, como `failed` e `pending` na outbox (`OBX-17`).
  - A migração é aditiva e sem lock exclusivo no boot, no padrão de `postgres/inbox.sql`.
  - A role de runtime precisa de `SELECT`, `INSERT`, `UPDATE` e `DELETE` na tabela (o `DELETE` é a purga de `finished`): o mecanismo de grants por capability da SPEC-QPRR24C4 os concede quando estiver em vigor, e um teste sob a role de runtime prova `Begin`, `Finish`, `Stale` e a purga. Esta spec não depende dela.
- [ ] **[P0] `Dispatch`**: esqueleto em `application` que grava `Begin` em transação própria do provider, como o claim do relay (`OBX-16`), chama `ExternalCommand.Submit` sem transação aberta (`OBX-07`) e grava `Finish` junto do recebimento da mensagem e do efeito do passo na UoW de consumo, com a inbox como primeira escrita (`INB-07`) e o ACK depois do commit (`TRP-26`).
  - O efeito do passo é um callback do contexto: o kernel decide o passo, o contexto aplica o estado do agregado e os eventos.
  - Cancelamento ou prazo depois da escrita do pedido grava `UnknownSent`.
  - Falha retentável com o orçamento esgotado grava `Finish` em transação própria e devolve a falha categorizada, sem registrar a mensagem na inbox: a disposição é a de FND-04 (`RES-36`, `MAP-07`), e a redelivery chama o parceiro de novo com a mesma chave.
  - Tentativa `begun` encontrada no início (o `Begin` devolve o instante de início) só vale como `UnknownSent` depois do prazo da chamada mais a margem, o mesmo corte da varredura, e então não chama o parceiro. Antes dele o handler dono ainda pode ter a chamada em voo: a redelivery não encerra a tentativa nem chama o parceiro e devolve falha retentável categorizada, sem registrar a mensagem na inbox (`RES-36`, `MAP-07`).
  - `Finish` sobre tentativa já `finished` por outro handler (a redelivery ou a varredura que a encerrou como `UnknownSent`) com a mesma classe é no-op; com classe diferente é conflito registrado e visível, com a classe tardia como evidência para a consulta ou a reconciliação. O desfecho gravado não é sobrescrito, o efeito do passo do handler tardio não é aplicado e o passo segue o do desfecho gravado.
- [ ] **[P0] `Resolve`**: aplicar uma `Observation`, vinda de consulta, de callback ou de reconciliação, de forma idempotente.
  - A mesma identidade de observação duas vezes é no-op; observação contraditória com o desfecho gravado, como `Applied` depois de `Rejected`, vira conflito registrado e visível, sem sobrescrever.
  - `Absent` com semântica de chave devolve `Retry` com a mesma chave; sem chave, só `AuthoritativeAbsence` o permite, e o resto vai a `Reconcile`.
- [ ] **[P0] Varredura de tentativas órfãs**: laço `RunSweep` em `app`, modelado em `RunPurge` (`app/purge.go:98`), que encerra como `UnknownSent` a tentativa `begun` mais velha que o prazo da chamada mais a margem, quando a redelivery não a retomou (mensagem contida ou perdida), e entrega o comando à consulta ou à reconciliação.
  - O laço recusa relógio, logger ou função nil, como a purga.
  - Nunca reenvia por conta própria.
- [ ] **[P1] Observabilidade**: cada tentativa, desfecho e resolução sai em span, log e métrica com a classe e `dmpf.outcome_category`; a chave não vira rótulo de métrica nem entra crua no log, só o formato validado.

#### Onda 2 — provider HTTP, fake e cláusulas

- [ ] **[P0] Investigar a escrita do pedido no transporte HTTP**: verificar se `WroteRequest` e o primeiro byte da resposta sobrevivem à composição `otelhttp`, `resendCounter` e `Client.Do`, em HTTP/1.1 e HTTP/2, e se o reenvio interno do `Transport` por `Idempotency-Key` aparece no mesmo sinal.
  - Sem resposta positiva, o provider HTTP classifica como `UnknownSent`, o conservador, todo erro de rede que não seja falha de conexão (`dial`), e só a falha de conexão como `RetryableTransport`.
- [ ] **[P0] Investigar o prazo da chamada frente ao ciclo de consumo**: medir o que o rebalance faz com a chamada em voo (`TRP-48`, `KFK-09`) e definir o prazo máximo do `Submit` dentro do handler.
- [ ] **[P0] Classificação no provider HTTP**: auxiliar em `libs/backend/go/http` que mapeia a resposta e o erro de rede para `ExternalClass` por tabela declarada e versionada pelo adaptador, com código desconhecido em `UnknownAmbiguous`. O `Do` aceita a chave informada pelo chamador (`client.go:147`), então o adaptador passa a `OutboundKey` e a aleatória deixa de valer para comando externo.
- [ ] **[P0] Parceiro roteirizado em `serviceskit`**: fake in-process que implementa `ExternalCommand` e `ExternalState` com roteiro por chave e por tentativa.
  - Cobre as nove classes, a consulta que responde cada achado, o parceiro sem consulta, o webhook duplicado e a resposta perdida depois de efeito aplicado.
  - Registra cada chave recebida, para o teste provar que o reenvio usou a mesma.
  - Determinístico: `clock`, `ids` e `stable`, sem `time.Sleep` (`.agents/skills/dmpf-testkit/SKILL.md:49-51`).
- [ ] **[P0] Cláusulas em `providerkit`**: o `KIT-04` ganha o conjunto que certifica um adaptador `ExternalCommand` (cada código de uma tabela de parceiro cai na classe declarada, desconhecido cai em `UnknownAmbiguous`) e uma realização de `AttemptLedger` (serialização na chave, `begun` não purgável).
- [ ] **[P1] Processo encerrado entre o envio e o resultado**, em `testkit/tb/dist` (`KIT-06`): o processo morre depois de `Begin` e antes de `Finish`, e a varredura resolve como `UnknownSent`.
- [ ] **[P1] Guia e skills**: corrigir "a saída é não fazer chamada nenhuma" no guia para dois caminhos (parceiro consumidor do contrato; parceiro de requisição e resposta), e atualizar a skill `dmpf-testkit` pela fonte em `tools/dmpf-plugin/templates/ai/`.

### Não-funcionais

- [ ] **Determinismo**: nenhum teste do ciclo usa `time.Sleep`, relógio de parede ou aleatoriedade.
- [ ] **Compatibilidade**: Go `1.26.9`, a diretiva dos `go.mod`; nenhum requisito direto novo de terceiros em `ports`, `application` ou `testkit`.
- [ ] **Custo**: o ciclo acrescenta duas escritas no ledger por tentativa e nenhuma consulta fora da varredura e da resolução.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Normas (`docs/dmpf/`) | FND-07 (subseção nova depois de §5.8), FND-08 (depois de §4.5), FND-04 §7.6 (`IDM-11`), FND-09 §8.1; manifestos do verificador |
| `ports` | tipos, `ExternalCommand`, `ExternalState`, `AttemptLedger` |
| `application` | `OutboundKey`, `NextStep`, `Dispatch`, `Resolve` |
| `memory`, `postgres` | realizações do ledger, `external_attempt.sql` e a capability nova |
| `http` | classificação do desfecho |
| `app` | `RunSweep` |
| `testkit` | `serviceskit`, `providerkit`, `tb/dist` |
| Documentação | ADR novo, guia de implementação, READMEs dos módulos |

## Localização de código

```text
libs/backend/go/ports/external.go                     novo: tipos e portas do ciclo
libs/backend/go/application/outbound_key.go           novo: OutboundKey
libs/backend/go/application/next_step.go              novo: NextStep
libs/backend/go/application/dispatch.go               novo: Dispatch e Resolve
libs/backend/go/memory/attempt_ledger.go              novo
libs/backend/go/postgres/external_attempt.sql         novo, embutido em migrate.go, que ganha a capability
libs/backend/go/postgres/attempt_ledger.go            novo
libs/backend/go/http/outcome.go                       novo: classificação
libs/backend/go/app/sweep.go                          novo: RunSweep
libs/backend/go/testkit/serviceskit/partner.go        novo: parceiro roteirizado
libs/backend/go/testkit/providerkit/external.go       novo: cláusulas do KIT-04
docs/dmpf/contexto-erros-seguranca.md                 subseção nova, ERR-29 em diante
docs/dmpf/resiliencia-observabilidade.md              subseção nova, RES-41 em diante
docs/dmpf/uow-inbox-outbox.md                         IDM-11
docs/dmpf/testes-interop.md                           regra nova em §8.1
docs/adr/                                             ADR novo
tools/tests/dmpf-verify/                              profiles.json, counts.json, line-refs.json
```

Os nomes são indicativos e se fixam no PR da Onda 0. Nenhuma unidade nova entra
nos `dmpf-units.json`: os pacotes novos pertencem a unidades já declaradas.

## Design

### Ciclo

```text
UoW0   caso de uso do contexto aceita o comando, cunha a identidade da intenção
       e grava na outbox o evento de despacho                  (UOW-07)
         relay → canal → consumer
TxA    AttemptLedger.Begin(chave, n), transação própria        commit
call   ExternalCommand.Submit(chave), sem transação aberta     (UOW-08, OBX-07, RES-34)
       sob a composição de RES-22, com o retry de RES-27
UoW2   inbox (INB-07) + Finish(chave, n, classe) + efeito do passo   commit → ACK (TRP-26)
```

Se o processo morre depois da `TxA` e antes da `UoW2`, a mensagem não foi confirmada
e a redelivery volta ao handler, que, passados o prazo da chamada e a margem, trata a
tentativa `begun` como `UnknownSent` sem chamar o parceiro (antes disso, devolve falha
retentável). Se a mensagem sai de circulação (contenção ou perda), a varredura encerra
a tentativa. Por isso o registro da tentativa precede a chamada e a varredura existe.

### Classes de desfecho

| Classe | Significado | Efeito no parceiro |
| ------ | ----------- | ------------------ |
| `Accepted` | o parceiro tomou o comando (aceite, não eficácia) | sim, ou a confirmar |
| `RetryableTransport` | falha antes de o pedido chegar | ausente |
| `RetryableUpstream` | parceiro indisponível ou limitando | ausente, por declaração do parceiro |
| `FinalValidation`, `FinalBusiness`, `FinalAuthorization` | rejeição definitiva | ausente |
| `UnknownSent` | pedido escrito, resposta não recebida | desconhecido |
| `UnknownAmbiguous` | resposta recebida e não interpretável, ou código desconhecido | desconhecido |
| `SchemaMismatch` | resposta fora do contrato | desconhecido |

A tradução proposta ao catálogo do FND-07 é: `RetryableTransport` e
`RetryableUpstream` em `TransientDependency` (`RateLimited` quando o parceiro
limita), retentáveis; `FinalValidation` em `Validation`; `FinalBusiness` em
`Conflict`, pela regra do parceiro (`ERR-28`); `FinalAuthorization` em
`Forbidden`; `UnknownSent` em `TransientDependency` ou `DeadlineExceeded`,
retentável só sob o predicado de chave; `UnknownAmbiguous` e `SchemaMismatch` em
`Unexpected`, não retentáveis. O ADR novo fixa a tabela.

### Função de passo

| Entrada | Perfil | Passo |
| ------- | ------ | ----- |
| `Accepted` | eficácia `OnSubmit` | `Confirm` |
| `Accepted` | eficácia `ByObservation` | `Await` |
| `RetryableTransport`, `RetryableUpstream` | qualquer | `Retry` com a mesma chave, dentro de `RES-27` a `RES-33`; esgotado o orçamento, a falha sai categorizada e a disposição é a de FND-04 (`RES-36`) |
| `Final*` | qualquer | `Fail`; reenviar exige nova geração |
| `UnknownSent`, `UnknownAmbiguous` | com consulta | `Query` |
| `UnknownSent`, `UnknownAmbiguous` | sem consulta | `Reconcile` |
| `SchemaMismatch` | qualquer | `Suspend`, e depois o passo da linha incerta correspondente |
| achado `Applied` | qualquer | `Confirm` |
| achado `Rejected` | qualquer | `Fail` |
| achado `Absent` | chave `Native` ou `Header` | `Retry` com a mesma chave, depois da quiescência |
| achado `Absent` | chave `None` | `Retry` só com `AuthoritativeAbsence`; senão `Reconcile` |
| achado `Pending` | qualquer | `Await`, com nova consulta dentro do orçamento |
| achado `Inconclusive` | qualquer | `Query` de novo até o teto e depois `Reconcile` |

`Reconcile` entrega o comando ao estado "aguardando reconciliação", que
nomeia o que ficou pendente e quem decide (`GAR-06`). O algoritmo de
reconciliação é do contexto; a saída dele volta ao kernel como `Observation`.

### Chave de saída

```text
OutboundKey = formato_do_parceiro( digest( canônico(tenant, destino, operação, intenção, geração) ) )
```

A codificação canônica reaproveita o desenho de `application.Fingerprint`
(`application/fingerprint.go:8-33`): tipo e tamanho por campo, sem depender da
serialização Protobuf. A chave não guarda segredo e não carrega a chave de
comando de entrada.

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Conhecimento do efeito | Classe de desfecho devolvida pelo provider, com tradução às onze categorias | Categoria nova no catálogo (altera `ERR-08` e exige linha em `MAP-01` nos três transportes, para um estado que não atravessa transporte); elemento novo no diagnóstico (altera o esquema fixo de `ERR-18`) |
| Registro da tentativa | Tabela de plataforma própria, gravada antes da chamada | Reusar a inbox (só admite `processed` e `rejected`, ambos terminais: `postgres/inbox.sql:13`); guardar no agregado do contexto (cada contexto reimplementaria o passo mais perigoso) |
| Quem faz a chamada | Handler do consumer, fora da UoW | Chamar dentro da UoW (viola `UOW-08`); `Sink` do relay (at-least-once, `failed` terminal por `OBX-06`, sem classes de desfecho) |
| Chave | Derivada da intenção e da geração, digest injetado | Aleatória por `Do` (estado atual); `message_id` (não cobre a reemissão: `dmpf-implementation.md:601-626`); chave de comando de entrada (`IDM-10`) |
| Reenvio depois do incerto | Só depois de achado `Absent` ou de reconciliação, com a mesma chave | Reenvio imediato pela retryability (`RES-28` veda) |
| Perfil do parceiro | Estrutura própria | Campo novo na ficha de `RES-21`: `resilience.Sheet` tem dez campos (`sheet.go:135-150`) e ficha em branco reprova, então todas as fichas existentes quebrariam |

## Regras relacionadas

- FND-04: `UOW-07`, `UOW-08`, `INB-01`, `INB-06`, `OBX-06`, `OBX-17`, `IDM-03`, `IDM-10`, `GAR-01`, `GAR-03`, `GAR-05`, `GAR-06`
- FND-06: `RST-02`, `TRP-26`, `TRP-48`
- FND-07: `ERR-01`, `ERR-08` a `ERR-12`, `ERR-18`, `ERR-25` a `ERR-28`
- FND-08: `RES-21`, `RES-25` a `RES-29`, `RES-34`, `RES-36`, `RES-37`
- FND-09: `KIT-01`, `KIT-03`, `KIT-04`, `KIT-06`
- ADR-050, ADR-056, ADR-058; SPEC-JJKWG4JP, SPEC-NYD18TGD, SPEC-E15TBHCD

## Verificação e testes

### Critérios de aceite

- [ ] O teste exaustivo de `NextStep` prova que nenhuma combinação com `UnknownSent`, `UnknownAmbiguous` ou `SchemaMismatch` devolve `Confirm` ou `Fail`.
- [ ] Duas execuções do mesmo comando (reentrega, reemissão pela outbox, reinício do processo) derivam a mesma chave; outra geração deriva chave distinta; a chave difere da de comando de entrada e do identificador da tentativa.
- [ ] No fake, o reenvio depois de `UnknownSent` envia a mesma chave, e o registro de chaves recebidas mostra uma só.
- [ ] Com o processo encerrado entre `Begin` e `Finish`, a redelivery depois do prazo e da margem, ou a varredura, resolve como `UnknownSent`, nenhuma tentativa `begun` passa do prazo e nenhuma segunda chamada ocorre sem consulta; um `Finish` tardio sobre tentativa encerrada não sobrescreve o desfecho e deixa conflito visível.
- [ ] Observação duplicada não muda nada; observação contraditória gera conflito visível.
- [ ] Todo código de uma tabela de parceiro cai na classe declarada no `providerkit`; o desconhecido cai em `UnknownAmbiguous`.
- [ ] ADR novo e subseções de norma presentes, com a tabela "altera invariante?" e `node tools/dmpf-verify.mjs` sem violação.
- [ ] Ao fim de cada onda, `go run ./tools/dmpf-conformance/cmd/conformance --root .` e `go run ./tools/dmpf-conformance/cmd/modsync --root . --check` passam, assim como `fmt-check`, `vet`, `test-race` e `govulncheck` de cada projeto Go tocado.

### Cenários de teste

**Happy path — aceite com confirmação**
- DADO um parceiro com eficácia `OnSubmit` e uma intenção de chave `K`
- QUANDO `Dispatch` chama `Submit` e o parceiro devolve `Accepted`
- ENTÃO `Begin` precede a chamada, `Finish` grava `Accepted`, o passo é `Confirm` e o fake recebeu `K` uma única vez

**Edge case — pedido enviado sem resposta, parceiro com consulta**
- DADO um parceiro com semântica de chave `Header` e consulta de estado
- QUANDO a primeira chamada devolve `UnknownSent` e a consulta responde `Absent`
- ENTÃO o passo é `Retry`, o reenvio usa a mesma chave `K`, e o resultado só vira `Confirm` quando o parceiro devolve `Accepted` ao reenvio

**Edge case — parceiro sem consulta de estado**
- DADO um parceiro com chave `None` e sem `ExternalState`
- QUANDO a chamada devolve `UnknownSent`
- ENTÃO o passo é `Reconcile`, o comando fica aguardando reconciliação e nenhum reenvio ocorre

**Edge case — notificação duplicada**
- DADO um comando confirmado por uma `Observation` `Applied` de identidade `E1`
- QUANDO a mesma observação `E1` chega de novo, por callback duplicado ou por nova consulta
- ENTÃO `Resolve` é no-op e nenhum evento novo é enfileirado

**Erro — código desconhecido do parceiro**
- DADO um adaptador cuja tabela não lista o código `X77`
- QUANDO o parceiro responde com `X77`
- ENTÃO a classe é `UnknownAmbiguous`, a retryability é falsa e o passo é consulta ou reconciliação, nunca `Retry`

**Erro — processo encerrado depois do envio**
- DADO uma tentativa `begun` e o processo morto antes de `Finish`
- QUANDO a mensagem é reentregue ao handler, ou a varredura passa, depois do prazo da chamada e da margem
- ENTÃO a tentativa fecha como `UnknownSent`, o comando segue para consulta ou reconciliação e nem o handler nem a varredura chamam `Submit`

**Edge case — `Finish` tardio sobre tentativa já encerrada**
- DADO uma tentativa que a redelivery fechou como `UnknownSent` depois do prazo e da margem, com o primeiro handler pausado e ainda com a chamada em voo
- QUANDO esse handler retoma, recebe `Accepted` e grava `Finish`
- ENTÃO o ledger mantém `UnknownSent`, registra conflito visível com `Accepted` como evidência, não aplica o efeito do passo e o comando segue para consulta ou reconciliação

<critical_constraints>
- [P0] A chamada ao parceiro nunca ocorre dentro de UoW: a intenção e o registro da tentativa commitam antes, o resultado commita depois (`UOW-07`, `UOW-08`, `RES-34`).
- [P0] Desfecho incerto nunca vira sucesso nem falha final sem consulta de estado ou reconciliação; sem resolução, o comando permanece incerto e visível.
- [P0] A chave de saída é a mesma em toda tentativa, reenvio, reentrega e reemissão da mesma intenção; só uma decisão de negócio explícita, a nova geração, a troca.
- [P0] A chave de saída nunca é a chave de comando de entrada nem o identificador da tentativa (`IDM-10`).
- [P0] Classe não reconhecida resolve para desfecho incerto não retentável, nunca para retentável (`ERR-11`, `RES-29`).
- [P0] O kernel não promete exactly-once com o parceiro (`GAR-01`): sem semântica de chave e sem consulta, o ciclo detecta e reconcilia, não previne.
- [P0] Nenhum texto de regra `normativo` existente muda; as regras novas somam, em subseção marcada como decisão nova (M1 a M4 da RFC).
- [P0] `ports` e `application` não importam rede, relógio nem SDK (`docs/adr/015-capabilities-externas-por-bloco.md`); a chamada vive em `provider`, e o SHA-256 entra por política injetada, como em `IdempotencyPolicy` (`application/idempotent.go:19-26`).
- [P1] Nenhum sexto kit (`KIT-01`): fake em `serviceskit`, cláusulas em `providerkit`.
- [P1] Nenhum nome de parceiro, protocolo de registradora ou formato de arquivo entra no kernel.
- [P1] `node tools/dmpf-verify.mjs`, `conformance` e `modsync --check` seguem verdes em cada onda.
</critical_constraints>

## Escopo fora

- **Algoritmo de reconciliação**: camadas, severidade das divergências e formato dos arquivos dos parceiros são do domínio do consumidor. O kernel entrega o estado "aguardando reconciliação" e a `Observation` que volta.
- **Reprocessamento do payload bruto**: reinterpretar o que já foi recebido sem reenviar comando é operação do consumidor; o ciclo daqui trata do envio.
- **Providers de arquivo, fila ou protocolos próprios de registradora** (mTLS e assinatura de mensagem): os formatos seguem abertos no consumidor, e só o provider HTTP entra. O ciclo é independente de protocolo.
- **Borda que recebe o callback do parceiro**: o ADR-044 faz do `bff` a única borda REST, e onde o callback entra, com verificação de origem e deduplicação por identificador do parceiro, é decisão do consumidor. O kernel entrega só a aplicação idempotente da `Observation`.
- **Valores do perfil de cada parceiro**: semântica de chave, consulta e janela de processamento são declaradas pelo consumidor, com o parceiro real.
- **Armazenamento da evidência bruta**: pertence à spec de object storage (K2), não a este ciclo.
- **Novo mecanismo de retry, backoff ou breaker**: `RES-27` a `RES-33` seguem como estão; esta spec só decide o que acontece quando o retry termina em incerteza.
- **Categoria nova no catálogo do FND-07**: descartada em Decisões técnicas; reabre só com ADR próprio.
