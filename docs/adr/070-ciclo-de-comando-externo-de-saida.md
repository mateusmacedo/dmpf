# ADR-070: Dar ao kernel o ciclo do comando externo de saída

## Status

Aceito — 2026-10-20. Implementa [SPEC-5AQHHMJ0](../specs/SPEC-5AQHHMJ0-comando-externo-de-saida.md). Emenda o [ADR-050](./050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md), no primeiro parágrafo da Decisão: o ledger de tentativas é tabela de plataforma com o tenant na chave. Acrescenta regras ao FND-04 (`IDM-11`), ao FND-07 (`ERR-29` a `ERR-31`, §5.9), ao FND-08 (`RES-41` a `RES-45`, §4.6) e ao FND-09 (`KIT-12`, §8.1.2), sem mudar texto de regra vigente.

## Contexto

A entrada tem norma: o ADR-056 e a família `IDM` fazem da inbox o registro do comando recebido. A saída tem só o lado negativo: o `RST-02` veda o retry de `POST` sem chave, e o `RES-25` a `RES-28` dizem quando não repetir. O `Operation.EffectAbsent` do `observability/retry` é booleano, e falso quer dizer apenas «o efeito pode existir»: o kernel não tem onde registrar essa dúvida nem como resolvê-la.

Três lacunas tornam isso concreto:

- **Não há porta de chamada.** O guia prescreve intenção na outbox e callback, com a chamada fora da UoW (`UOW-07`, `UOW-08`). O desenho vale para o parceiro que consome o contrato; não cobre o parceiro que só expõe requisição e resposta, em que alguém precisa fazer a chamada.
- **A chave de saída não é estável.** O `http.Client` sorteia uma chave por `Do` quando o chamador não informa, e o `app.Attempt.RequestID` é sorteado por tentativa. Reentrega e reemissão produzem chave nova e, no parceiro, efeito duplicado. O `net/http` ainda pode reenviar sozinho o pedido que traz `Idempotency-Key` sobre conexão reaproveitada, e só a chave estável torna isso inócuo.
- **O desfecho não carrega o que se sabe do efeito.** O `http.Classifier` não separa o pedido que não saiu do pedido escrito sem resposta, e as onze categorias do FND-07 descrevem a falha, não o efeito.

O consumidor externo antecipa-market (SPEC-GXGASP24, extensão K5) integra registradoras e parceiros cuja resposta pode se perder, e precisa do kernel a chave, a classificação, as transições e os duplos de teste; a máquina de estados de negócio continua dele.

## Decisão

1. **Nove classes de desfecho, fechadas e totais (`ERR-29`).** `ports.ExternalClass`: `ClassAccepted` (o parceiro tomou o comando; aceite, não eficácia), `ClassRetryableTransport` (falha antes de o pedido chegar), `ClassRetryableUpstream` (parceiro indisponível ou limitando), `ClassFinalValidation`, `ClassFinalBusiness`, `ClassFinalAuthorization` (rejeição definitiva), `ClassUnknownSent` (pedido escrito, resposta não recebida), `ClassUnknownAmbiguous` (resposta não interpretável ou código desconhecido) e `ClassSchemaMismatch` (resposta fora do contrato). O valor zero não é classe e vale como incerto.

2. **Tradução ao catálogo do FND-07, sem categoria nova.** Cada classe que não é aceite resolve para uma categoria de §5.3 e para uma retryability booleana (`ERR-01`, `ERR-09`); o aceite não é falha:

   | Classe | Categoria | Retentável |
   | ------ | --------- | ---------- |
   | `ClassAccepted` | nenhuma: não é falha | — |
   | `ClassRetryableTransport` | `TransientDependency` | sim |
   | `ClassRetryableUpstream` | `TransientDependency`, ou `RateLimited` quando o parceiro limita | sim |
   | `ClassFinalValidation` | `Validation` | não |
   | `ClassFinalBusiness` | `Conflict`, pela regra do parceiro (`ERR-28`) | não |
   | `ClassFinalAuthorization` | `Forbidden` | não |
   | `ClassUnknownSent` | `DeadlineExceeded` quando o prazo da chamada expirou, `Unexpected` nos demais casos; ambas condicionais | só sob o predicado de `ERR-30` |
   | `ClassUnknownAmbiguous` | `Unexpected` | não |
   | `ClassSchemaMismatch` | `Unexpected` | não |

   O pedido enviado sem resposta não vai a `TransientDependency`, que é retentável sem condição: a dúvida sobre o efeito exige categoria condicional. O predicado de `ERR-30` é o que o `ERR-10` exige dela: o pedido levou a chave de saída e o perfil do parceiro declara semântica de chave `KeyNative` ou `KeyHeader`. Sem ele, vale o `ERR-11`. Código do parceiro fora da tabela do adaptador resolve para `ClassUnknownAmbiguous` (`ERR-31`). O conhecimento do efeito vive na classe e no registro da tentativa: o catálogo de onze categorias não ganha linha (`ERR-08`), e o diagnóstico do `ERR-18` mantém os seis elementos.

3. **O desfecho incerto encerra o retry sem virar falha final (`RES-41`).** O retry da chamada segue o `RES-27` a `RES-33`, com a mesma chave. Quando ele termina com `ClassUnknownSent`, `ClassUnknownAmbiguous` ou `ClassSchemaMismatch`, esse é o desfecho declarado que o `RES-36` exige: o comando segue para consulta de estado, reconciliação ou suspensão, e nunca para sucesso ou falha final sem uma delas.

4. **Chave de saída derivada da intenção (`IDM-11`).** `application.DeriveOutboundKey` codifica tenant, destino, operação, identidade da intenção e geração no formato canônico de `application.Fingerprint` (tipo e tamanho por campo, rótulo próprio), aplica o digest injetado por `OutboundKeyPolicy` e escreve o resultado no alfabeto do parceiro. A identidade da intenção é cunhada uma vez, no commit da intenção, e viaja no payload do evento de despacho; a reemissão de um registro `failed` carrega a mesma. A geração só avança por decisão explícita do caso de uso, depois de rejeição final corrigida. A chave nunca é a de comando de entrada nem o identificador da tentativa (`IDM-10`). O digest é criptograficamente seguro, com pelo menos 128 bits de força, e a implementação, SHA-256, fica fora de `ports` e de `application` (ADR-015).

5. **Perfil do parceiro como declaração própria (`RES-44`).** `ports.ExternalProfile` declara a semântica de chave (`KeyNative`, `KeyHeader`, `KeyNone`), a eficácia (`EfficacyOnSubmit`, `EfficacyByObservation`), a janela de quiescência, a ausência autoritativa, o teto de consultas (`MaxQueries`) e o alfabeto e o comprimento máximo da chave. O alfabeto é de bytes ASCII visíveis, sem espaço e sem repetição; a chave usa `floor(log2(|alfabeto|))` bits por símbolo, só os primeiros `2^b` símbolos, e o comprimento precisa alcançar 128 bits. `application.ValidateExternalProfile` recusa o perfil fora disso. O perfil não é campo da ficha de `RES-21`.

6. **Função de passo pura.** `application.NextStep(StepInput) Step` devolve `StepConfirm`, `StepAwait`, `StepRetry`, `StepFail`, `StepQuery`, `StepReconcile` ou `StepSuspend`, sem relógio, rede nem transação. Achado presente prevalece sobre a classe, porque é ele que resolve a incerteza:

   | Entrada | Perfil | Passo |
   | ------- | ------ | ----- |
   | `ClassAccepted` | eficácia `EfficacyOnSubmit` | `StepConfirm` |
   | `ClassAccepted` | eficácia `EfficacyByObservation` | `StepAwait` até o teto de consultas, depois `StepReconcile` |
   | `ClassRetryableTransport`, `ClassRetryableUpstream` | qualquer | `StepRetry` com a mesma chave |
   | `ClassFinal*` | qualquer | `StepFail`; reenviar exige nova geração |
   | `ClassUnknownSent`, `ClassUnknownAmbiguous`, classe zero | com consulta e abaixo do teto | `StepQuery` |
   | `ClassUnknownSent`, `ClassUnknownAmbiguous`, classe zero | sem consulta ou no teto | `StepReconcile` |
   | `ClassSchemaMismatch` | qualquer | `StepSuspend` |
   | achado `FindingApplied` | qualquer | `StepConfirm` |
   | achado `FindingRejected` | qualquer | `StepFail` |
   | achado `FindingAbsent` | chave `KeyNative` ou `KeyHeader` | `StepRetry` com a mesma chave |
   | achado `FindingAbsent` | chave `KeyNone` | `StepRetry` só com ausência autoritativa; senão `StepReconcile` |
   | achado `FindingPending` | qualquer | `StepAwait` até o teto, depois `StepReconcile` |
   | achado `FindingInconclusive` | qualquer | `StepQuery` até o teto, depois `StepReconcile` |

   A espera da quiescência antes do reenvio, o intervalo entre consultas e o orçamento de reenvio das classes retentáveis são de quem chama (`RES-27`), não da função. `StepAwait` sem consulta de estado é legítimo, para o parceiro que confirma por callback, e o prazo dessa espera é por tempo e do dono do `RES-43`, não do teto de consultas. Quem retoma uma suspensão, corrigido o contrato ou o adaptador, reentra como `ClassUnknownAmbiguous` (`RES-43`).

7. **Reenvio é operação nova, com a mesma chave (`RES-42`).** O reenvio posterior à chamada não é retry dela (`RES-34`): ocorre só depois de achado `FindingAbsent`, passada a quiescência, ou de reconciliação, e leva a mesma chave.

8. **Espera com prazo e dono (`RES-43`).** A espera pela consulta, pela reconciliação e a suspensão têm prazo e dono declarados (`GAR-05`, `GAR-06`). `StepReconcile` entrega o comando ao estado «aguardando reconciliação», que nomeia o pendente e quem decide; o algoritmo de reconciliação é do contexto, e a saída dele volta ao kernel como `ports.Observation`.

9. **A intenção percorre a outbox (`RES-45`).** A intenção do comando externo é gravada na outbox na UoW do caso de uso, que é o mecanismo do modo `difere` do `RES-37`, e a chamada acontece no handler do consumer, fora de UoW (`UOW-07`, `UOW-08`, `RES-34`). `ports.ExternalCommand` (`Submit`) e `ports.ExternalState` (`Query`) são as portas; a ausência de `ExternalState` declara «sem consulta de estado», e um valor nulo nunca vale como consulta.

10. **Ledger de tentativas como tabela de plataforma, com o tenant na chave.** A tentativa é registrada antes da chamada, em transação própria, e encerrada depois, na UoW de consumo. A tabela `external_attempt` é do kernel, como `outbox`, `inbox` e `quarantine`, mas a chave inclui o tenant, como na entrada de comando do `IDM-03`, porque a mesma chave de saída em outro tenant é outra tentativa. É a emenda ao ADR-050.

11. **Testes sem sexto kit (`KIT-12`).** O parceiro roteirizado é fake do `KIT-03`, em `serviceskit`, e a conformidade de um adaptador `ExternalCommand` e de uma realização do ledger é cláusula do `KIT-04`, em `providerkit` (`KIT-01`).

12. **Entrega em três ondas.** A Onda 0 fixa esta norma, os tipos de `ports` e as funções puras de `application`; a Onda 1, o ledger, o `Dispatch`, o `Resolve` e a varredura de tentativas órfãs; a Onda 2, a classificação no provider HTTP, o parceiro roteirizado e as cláusulas do kit. A tradução da classe à categoria entra em código com o `Dispatch`, que a consome.

13. **Monotonicidade.** As regras novas somam, em subseção marcada como decisão nova, e nenhuma muda texto de regra vigente (M1 a M4 da RFC §12.2):

    | Decisão | Altera invariante? | Verificação |
    | ------- | ------------------ | ----------- |
    | Classe de desfecho fechada e total (`ERR-29`) | Não | Aditiva sobre o `ERR-01` e o `ERR-09`: toda classe que não é aceite resolve para categoria existente e retryability booleana; o catálogo do `ERR-08` não ganha linha |
    | `ClassUnknownSent` retentável só sob predicado (`ERR-30`) | Não | Usa as categorias condicionais `DeadlineExceeded` e `Unexpected` como estão; instância do `ERR-10`, e sem o predicado vale o `ERR-11` |
    | Código desconhecido como incerto não retentável (`ERR-31`) | Não | Restringe: herda o fail-closed do `ERR-11` e do `RES-29`, nunca o inverte |
    | Incerto encerra o retry sem falha final (`RES-41`) | Não | Declara o desfecho que o `RES-36` exige; o `RES-27` a `RES-33` seguem como estão |
    | Reenvio como operação nova com a mesma chave (`RES-42`) | Não | Aplica o `RES-34` e o `RES-28`: o reenvio exige achado ou reconciliação, não retryability |
    | Espera, reconciliação e suspensão com prazo e dono (`RES-43`) | Não | Instância do `GAR-05` e do `GAR-06` |
    | Perfil do parceiro fora da ficha (`RES-44`) | Não | A ficha do `RES-21` não muda; o perfil é declaração própria |
    | Intenção pela outbox (`RES-45`) | Não | É o modo `difere` do `RES-37`, com a chamada fora da UoW (`UOW-08`) |
    | Chave de saída derivada da intenção (`IDM-11`) | Não | Complementa o `IDM-10`: a chave de entrada continua sem atravessar |
    | Parceiro roteirizado e cláusulas do kit (`KIT-12`) | Não | Cabe no `KIT-03` e no `KIT-04`; o `KIT-01` segue com cinco kits |
    | Ledger com tenant na chave (emenda ao ADR-050) | Não | `outbox`, `inbox` e `quarantine` seguem sem escopo de tenant; o ledger repete a chave do `IDM-03` |

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Categoria nova no catálogo do FND-07 | Altera o `ERR-08` e exige linha no `MAP-01` nos três transportes, para um estado que não atravessa transporte |
| Elemento novo no diagnóstico de erro | Altera o esquema fixo do `ERR-18` |
| Reusar a inbox como registro da tentativa | A inbox só admite `processed` e `rejected`, ambos terminais |
| Guardar a tentativa no agregado do contexto | Cada contexto reimplementaria o passo mais perigoso do ciclo |
| Chamar o parceiro dentro da UoW | Viola o `UOW-08` |
| Chamar pelo `Sink` do relay | O relay é at-least-once, o `failed` é terminal (`OBX-06`) e não há classe de desfecho |
| Chave aleatória por `Do` | Muda a cada reentrega e reemissão, e o parceiro aplica o efeito de novo |
| `message_id` como chave | Não cobre a reemissão de um registro `failed` |
| Chave de comando de entrada como chave de saída | Vedado pelo `IDM-10` |
| Reenvio imediato pela retryability depois do incerto | O `RES-28` separa retryability de autorização |
| Perfil do parceiro como campo da ficha de `RES-21` | A ficha tem dez campos e ficha em branco reprova: todas as fichas existentes quebrariam |
| Teto de consultas como parâmetro de `NextStep` | O teto ficaria sem dono declarado, contra o `GAR-05` e o `GAR-06` |
| `NextStep` devolvendo `Suspend` e o passo seguinte juntos | A função deixaria de devolver um único passo para todas as classes |

## Consequências

**Positivas:**

- A mesma intenção deriva a mesma chave em toda tentativa, reentrega, reemissão e reenvio, e o parceiro com semântica de chave deduplica o que o kernel não consegue impedir.
- O desfecho incerto deixa de virar sucesso ou falha por omissão: tem classe, passo e dono.
- O catálogo de categorias, o diagnóstico de erro e a ficha de resiliência não mudam, e nenhum consumidor dos três transportes é afetado.
- A função de passo é pura e testada sobre o produto completo das entradas.

**Negativas:**

- O kernel não promete exactly-once com o parceiro (`GAR-01`): sem semântica de chave e sem consulta de estado, o ciclo detecta e reconcilia, não previne. **Custo aceito:** o perfil declara a capacidade real do parceiro, e o que falta vira reconciliação visível.
- Cada tentativa custa duas escritas no ledger. **Custo aceito:** é o preço de registrar a tentativa antes da chamada, sem o qual a queda do processo entre o envio e o resultado não deixa rastro.
- A força do digest é contrato, não verificação: um `OutboundKeyPolicy` com digest fraco passa nos testes. **Custo aceito:** o composition root injeta SHA-256, e a revisão do adaptador confere a política.
- Até a Onda 2, o kernel não separa «não saiu» de «saiu sem resposta» no provider HTTP, e toda falha de rede que não seja de conexão é `ClassUnknownSent`. **Custo aceito:** é o lado conservador; a investigação do sinal de escrita do pedido é requisito da Onda 2 da spec.

## Referências

- [SPEC-5AQHHMJ0](../specs/SPEC-5AQHHMJ0-comando-externo-de-saida.md) (DEVS-96), [SPEC-JJKWG4JP](../specs/SPEC-JJKWG4JP-idempotencia-ponta-a-ponta.md), [SPEC-NYD18TGD](../specs/SPEC-NYD18TGD-dmpf-resiliencia-observabilidade-go.md) e [SPEC-E15TBHCD](../specs/SPEC-E15TBHCD-dmpf-resiliencia-observabilidade.md); SPEC-GXGASP24 (DEVS-76), fora deste repositório.
- [ADR-015](./015-capabilities-externas-por-bloco.md), [ADR-050](./050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md) e [ADR-056](./056-idempotencia-de-comando-pela-inbox.md).
- `docs/dmpf/uow-inbox-outbox.md` (FND-04) — `UOW-07`, `UOW-08`, `OBX-06`, `IDM-03`, `IDM-10`, `IDM-11`, `GAR-01`, `GAR-05`, `GAR-06`; §7.6.
- `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — `ERR-01`, `ERR-08` a `ERR-11`, `ERR-18`, `ERR-28` a `ERR-31`; §5.3, §5.4 e §5.9.
- `docs/dmpf/resiliencia-observabilidade.md` (FND-08) — `RES-21`, `RES-27` a `RES-29`, `RES-34`, `RES-36`, `RES-37`, `RES-41` a `RES-45`; §4.6.
- `docs/dmpf/testes-interop.md` (FND-09) — `KIT-01`, `KIT-03`, `KIT-04`, `KIT-12`; §8.1.2.
- `libs/backend/go/ports/README.md` e `libs/backend/go/application/README.md`.
