# ADR-025: Adotar Kafka como transporte-alvo do assíncrono de domínio e manter SNS/SQS normatizado no acervo

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

## Contexto

Um sistema orientado a mensagens precisa dizer, para cada evento de domínio que
publica de forma assíncrona, por qual transporte ele trafega — e, escolhido o
transporte, como o canal ordena, confirma, retenta e contém o que falha. Sem
essa decisão fixada, cada equipe resolve por conta própria, e o acervo diverge
em convenções incompatíveis de ACK, retry e destino da mensagem morta.

Duas forças puxam em direções opostas. De um lado, o inventário da organização é
inteiramente SNS/SQS: há serviços em runtime consumindo filas e zero clientes
Kafka versionados nos repositórios. A base observada, tomada como argumento,
mandaria consolidar SNS/SQS como o transporte oficial. De outro, a decisão de
plataforma é convergir o assíncrono de domínio para Kafka — pela ordenação por
partição, pela retenção do log e pela releitura por offset, que a fila não
oferece. A norma que fixa isso, portanto, normatiza um transporte sem prática
vigente, e precisa dizê-lo em vez de simular experiência que não tem.

Sobre as duas pesa um invariante não relaxável: nenhum transporte promete
entrega ou processamento exactly-once fim a fim (P0-3). A semântica oficial é
at-least-once com efeitos idempotentes, e o modo de falha mais provável dessa
regra não é alguém escrever "exactly-once" num documento — é habilitar a
deduplicação nativa de um broker, ver as duplicatas sumirem do teste e concluir
que a inbox ficou dispensável. A decisão de transporte precisa fechar essa porta.

Esta decisão cobre apenas os transportes assíncronos. A interface síncrona —
REST/JSON na borda externa e gRPC no interior — é decisão distinta, registrada
em separado, e não é matéria deste ADR.

## Decisão

Adotar **Kafka como transporte-alvo do evento de domínio assíncrono** e manter
**SNS/SQS normatizado no acervo**, fixando em cada transporte a ordenação, o
gesto de ACK, o retry e a contenção, tudo sob a vedação de exactly-once fim a
fim. O sujeito é o comportamento do `provider` de transporte.

- **Kafka é o default do assíncrono novo** (`TRP-39`). Um canal novo de evento
  de domínio usa Kafka, salvo quando um critério declarado de seleção discrimina
  a favor de SNS/SQS — e o caso é registrado no canal, com o critério que o
  decidiu.
- **SNS/SQS permanece normatizado, não tolerado** (`TRP-40`). O acervo continua
  conforme enquanto observar a política da fila; migrar não é condição de
  conformidade. Fan-out com filtro por atributo e fila de trabalho sem ordenação
  são os dois casos em que SNS/SQS é a escolha técnica superior, não legado.
- **A política Kafka vale como norma de desenho, não como autorização de
  operação, até a revisão de infraestrutura de mensageria** (`TRP-41`). Por não
  ter base as-is, ela é prospectiva e não foi validada contra operação real.
- **Cada transporte fixa as quatro dimensões.** Ordenação: por partição em Kafka
  (`KFK-06`), por grupo em SQS FIFO (`SQS-05`), nenhuma em standard (`SQS-04`).
  ACK: sempre depois do commit local (`TRP-26`) — commit de offset contíguo em
  Kafka (`TRP-29`), delete por identificador de recebimento em SQS (`SQS-09`).
  Retry: inline com limite (`KFK-10`) ou visibilidade com backoff e limite de
  recebimentos (`SQS-10`). Contenção: DLQ publicada antes do avanço do offset
  (`KFK-12`), redirecionamento gerenciado ou publicação explícita — nunca ambos
  na mesma disposição — em SQS (`SQS-11`).
- **Deduplicação nativa é otimização, nunca substituto da inbox** (`TRP-42`,
  `TRP-43`). A idempotência de produtor em Kafka e a deduplicação de FIFO no SQS
  atuam na publicação, com janela; a inbox e a idempotência de efeito atuam no
  consumo. As duas coisas são verdadeiras e não se somam.
- **A coexistência é entre canais, nunca dentro do mesmo canal** (`COE-01`). Um
  canal lógico tem um transporte por vez; o `consumer_name` da inbox é lógico e
  estável entre transportes (`COE-04`), e trocar o transporte de um canal é
  migração com drenagem ou transferência declarada de backlog (`COE-03`).

A escolha de registry de schemas em runtime é decisão à parte (`ADR-DMPF-N`,
acionada pelo FND-05 e ainda pendente); este ADR cita a dependência e não a
decide. O cronograma e o plano de migração para Kafka são de adoção
organizacional e ficam fora daqui.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Consolidar SNS/SQS como transporte-alvo, pelo peso da base as-is | A base observada é um fato de ausência — zero clientes Kafka, mensageria SQS-cêntrica —, e o princípio da RFC é que "a evidência lastreia; ela não cria a norma". Tomar o as-is como decisão de arquitetura confunde o que existe com o que deve ser o alvo; a adoção de Kafka é premissa de plataforma recebida, não conclusão a extrair do inventário (§8.2, bloco `registro`). |
| Adotar Kafka declarando SNS/SQS depreciado de imediato | SNS/SQS não é apenas legado: há dois casos — fan-out com filtro por atributo e fila de trabalho sem ordem — em que é a escolha técnica superior. Depreciá-lo forçaria migração onde ela piora o desenho e tornaria não conforme um acervo que já satisfaz a norma (`TRP-40`). |
| Dar estatuto igual aos dois transportes, sem declarar alvo | Não eleger alvo algum deixaria cada canal novo sem default e não cumpriria a premissa de plataforma — recebida, não concluída aqui — de convergir o assíncrono de domínio para Kafka (`TRP-39`); estatuto igual sem direção declarada reproduziria a divergência de convenções incompatíveis de ACK, retry e destino da mensagem morta que o Contexto descreve (§8.2, bloco `registro`). |
| Publicar o mesmo canal nos dois transportes durante a transição | Publicar um canal lógico em Kafka e em SNS/SQS ao mesmo tempo cria duas ordens independentes e duas linhas de redelivery para o mesmo fluxo, sem relógio nem sequência comuns entre eles (`COE-01`; vetor V-COE-5). |
| Tratar a deduplicação nativa do broker como substituto da inbox | Habilitar a dedup nativa e concluir que a inbox é redundante é "o erro mais frequente do padrão": o mecanismo do broker atua na publicação, com janela; a inbox e a idempotência de efeito atuam no consumo, sem janela. As duas são verdadeiras e não se somam (§13, bloco `rationale`). |

As alternativas acima são as que o acionamento de §17.2 registra como descartadas
desta decisão (RFC §13.2). Por ser escolha estrutural, os seus motivos vivem em
blocos `registro` e `normativo` além do `rationale` de §13, não num único `rationale`.

## Consequências

**Positivas:**

- O caminho de depreciação imediata quebraria a conformidade do acervo e
  forçaria migração onde ela piora o desenho; esta decisão evita essa quebra —
  nenhum serviço SNS/SQS existente precisa migrar para permanecer conforme.
- Cada canal fica verificável: ordenação, ACK, retry e contenção têm regra por
  transporte, e a `janela_redelivery` é declarada com fórmula fechada.
- O modo de falha mais provável de P0-3 fica fechado: separar a dedup de
  publicação (com janela) da idempotência de efeito (sem janela) impede a
  conclusão de que a inbox é dispensável (`TRP-42` a `TRP-44`).
- Nos dois casos em que SNS/SQS é a escolha técnica ótima — fan-out com filtro,
  fila de trabalho sem ordem —, o desenho permanece no transporte de melhor
  ajuste, em vez de ser empurrado a Kafka por uniformidade.

**Negativas:**

- **Custo aceito:** a política Kafka é prospectiva. Sem nenhum cliente Kafka no
  acervo, a norma da §11 não foi validada contra operação real e vale apenas
  como norma de desenho até a revisão de infraestrutura de mensageria
  (`TRP-41`); assume-se o risco de essa revisão exigir ajustes na política antes
  da primeira operação.
- Manter os dois transportes normatizados, em vez de convergir para um só, dobra
  a superfície que quem implementa precisa conhecer — dois modelos de ACK, de
  retry e de contenção — e obriga a coexistência a reconciliar binding estável,
  `consumer_name` lógico idêntico e regras de migração entre eles (`COE-01` a
  `COE-08`).
- A decisão fixa o alvo e as convenções, mas não encerra dependências abertas: a
  escolha de registry de schemas (`ADR-DMPF-N`), a operação de DLQ e o runbook
  (encaminhados ao FND-08) e o cronograma de migração (adoção organizacional).

## Referências

- ADR-010 — regra de dependência e os seis blocos de pertencimento único. Fixa
  que `provider` é um bloco distinto; é ele o sujeito de toda regra deste ADR
  (`TRP-01`), e é por ser bloco próprio que o transporte é normatizado sem tocar
  domínio, aplicação ou contrato.
- ADR-014 — proibir a aresta `domain → port` e reservar `port` à fronteira. É o
  que permite o endereço concreto do transporte — nome de tópico, fila, ARN —
  viver só na configuração do `provider`, sem que domínio ou contrato o conheçam
  (`TRP-07`).
- ADR-015 — capabilities externas por bloco, com default deny e allowlist por
  entrypoint. Sustenta que o broker e o SDK vivem no `provider`: as capabilities
  de rede e conexão são permitidas porque a porta as requer, e um `provider` que
  precise de resolução remota para desserializar não tem porta que o justifique
  (`TRP-04`).
- Artefato de origem — `docs/dmpf/politicas-transporte.md` (FND-06): §8.2 (a
  matriz de decisão de transporte), §11 (Kafka), §12 (SNS e SQS), §13 (os três
  escopos de deduplicação), §14 (Kafka e SNS/SQS lado a lado) e §15
  (coexistência dos dois transportes).
- SPEC-DBTRMM3X — especificação da série de ADRs do DMPF, que este ADR implementa.
- ARQ-448 — https://lider-cap.atlassian.net/browse/ARQ-448 (FND-11, redação,
  promoção e aceite da série de ADRs do DMPF).
