# Políticas de transporte: REST, gRPC, Kafka, SNS/SQS e AsyncAPI — DMPF FND-06

| Campo | Valor |
|-------|-------|
| **Status** | `draft normativo` — promovido para revisão em PR |
| **Adiciona a** | RFC DMPF Foundation v0.1, pela âncora ANC-04 (RFC §12.3) |
| **Owner** | Mateus Macedo Dos Anjos (assignee de [ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443)) |
| **Épico** | [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436) — Golden Path para Sistemas Orientados a Domínio e Mensagens |
| **Story** | [ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443) (DMPF-FND-06) |
| **Spec** | [SPEC-YWFGNPG5](../specs/SPEC-YWFGNPG5-dmpf-politicas-transporte.md) |
| **Data** | 2026-08-19 |
| **Revisão** | Plataforma e Arquitetura no PR; **infraestrutura de mensageria** para §11 e §12, sem cuja participação as políticas de broker não passam de proposta; um representante de cada stack (Go e TypeScript) para §10 |

> **O que este documento obriga.** As regras rotuladas `normativo` valem para todo
> trabalho novo do DMPF, na mesma força da RFC à qual elas se adicionam. Nem todo
> bloco aqui obriga: §1.2 define as categorias de conteúdo e a fronteira de cada
> uma, e §1.3 define o **sujeito** que uma regra deste artefato pode ter — leitura
> obrigatória antes de qualquer regra, porque este artefato normatiza comportamento
> de bloco, e a âncora que o autoriza nomeia um bloco só. Enquanto o status for
> `draft normativo`, o documento está em revisão; a promoção ocorre no aceite do PR.

---

## §1. Fronteira, referência e sucessão

Esta seção vem antes de qualquer regra porque um artefato que adiciona a uma norma
compartilhada precisa dizer, primeiro, **até onde** ele pode obrigar. A RFC já
respondeu a essa pergunta ao registrar a âncora ANC-04; o que segue é a leitura
dessa autorização, a convenção de leitura do texto e o efeito deste documento sobre
a base conceitual a que ele sucede.

Há uma assimetria em relação aos três precedentes, e ela condiciona todo o resto. A
ANC-01 recortou um bloco e o FND-03 o esgotou. A ANC-02 recortou um mecanismo que
atravessa três blocos, e o FND-04 declarou o bloco de cada regra. A ANC-03 recortou
um bloco cujo assunto era maior do que o FND-05 entregava, e ele nomeou a lacuna em
vez de transferi-la a quem não a pediu.

A ANC-04 traz a dificuldade inversa das três: ela recorta **um** bloco —
`provider` —, mas os artefatos precedentes encaminharam a este, nominalmente,
obrigações cujo sujeito é outro bloco. O gesto de ACK do adapter, a validação do
envelope na recepção e o destino do envelope inválido foram todos escritos como
dívida deste artefato, e nenhum deles é comportamento de `provider` pela
classificação de RFC §4.1. §1.3 resolve isso declarando o sujeito admissível de
uma regra daqui, e §18 registra a tensão como pendência da própria âncora, em
vez de resolvê-la por silêncio.

### §1.1 A autorização: âncora ANC-04

`normativo`

Este artefato **adiciona** à RFC DMPF Foundation v0.1 pela âncora ANC-04
(RFC §12.3). Ele não edita a RFC e não incrementa a versão dela: adição por âncora
dentro do escopo permitido é revisão em PR, sem incremento (RFC §14.2). O conteúdo
vive aqui, e a RFC o alcança pelo endereço estável da âncora (RFC §12.1).

| Campo | Valor, conforme o registro de ANC-04 |
|-------|--------------------------------------|
| Assunto | Políticas por transporte |
| Escopo permitido | Políticas específicas de cada transporte, no bloco `provider` |
| Invariantes intocáveis | **P0-3** — a vedação a exactly-once fim a fim não é relaxável por transporte; RFC §6.2 |
| Monotonicidade | M1–M4 (RFC §12.2): detalhar e restringir, nunca relaxar, revogar ou reinterpretar |
| Artefato sucessor | Spec e seção própria |
| Condição de fechamento | FND-06 concluída e revisada |
| Impacto de versão na RFC | Nenhum, se dentro do escopo |
| ADR exigido | **Sim** — por transporte adotado. Acionado em §17 |

`normativo` — **Precedência.** Onde este artefato e a RFC divergirem, prevalece a
RFC. Uma divergência não se resolve neste documento: por M4, ela indica que a
fronteira de RFC §1.4 mudou, e mudar fronteira é mudança de versão da RFC, com o
rito de RFC §14.2.

`normativo` — **Monotonicidade em concreto.** As invariantes da âncora são citadas
ao longo do texto, e nenhuma regra deste artefato as afrouxa:

| Invariante | Como este artefato a trata | Onde |
|------------|----------------------------|------|
| **P0-3** — nenhuma promessa de exactly-once fim a fim | Reafirmada e **restringida**: §2.2 fixa o critério de rejeição, e nenhum mecanismo nativo de broker é apresentado como substituto da idempotência de efeito | §2.2, §13, §14 |
| RFC §6.2 — `provider` tem política permissiva, limitada à capability correspondente à porta que implementa | Reproduzida e usada como critério: uma política de transporte que exija do `provider` capability sem porta correspondente excede o bloco | §2.1 |
| RFC §6.2 — `observability` não é permitida em `domain` nem em `port` | Preservada: a propagação de contexto de tracing que este artefato exige ocorre no `provider` e no `app`, nunca por dependência do domínio | §2.1 |

### §1.2 Classificação de força

`normativo`

| Rótulo | Significado | Obriga? |
|--------|-------------|---------|
| `normativo` | Regra que este artefato estabelece sobre o comportamento de transporte, no escopo da ANC-04 e sob o sujeito admissível de §1.3 | **Sim** |
| `recepcionado` | Conteúdo reproduzido da Parte-1, da RFC ou de um artefato precedente para dar contexto contíguo, sem força nova e sem reabertura | Não — a força permanece da fonte |
| `encaminhado` | Assunto de outra sub-spec, citado apenas como fronteira rotulada, com dona explícita | Não — passa a obrigar quando a dona normatizar |
| `rationale` | Justificativa de uma decisão normativa, incluindo alternativas descartadas | Não |
| `registro` | Matriz de obrigações (§2.3), rastreio de requisitos (§3), acionamento de ADR (§17), rastreabilidade e pendências (§18) | Não |

### §1.3 O sujeito das regras deste artefato

`normativo`

A ANC-04 autoriza normatizar «políticas específicas de cada transporte, no bloco
`provider`». O recorte é de um bloco, como em ANC-01 e ANC-03 — mas com uma tensão
que nenhuma das duas tinha: os artefatos precedentes escreveram como dívida deste
as obrigações abaixo, e o sujeito de cada uma é o bloco `app`, não `provider`.

| Obrigação recebida | Origem | Sujeito pela classificação de RFC §4.1 |
|--------------------|--------|----------------------------------------|
| Gesto de ACK do adapter | `uow-inbox-outbox.md` §1.4, §10; `cloudevents-protobuf-buf.md` §1.4 | `app` — adapter de entrada |
| Validação do envelope na recepção | `uow-inbox-outbox.md` §1.4 (Parte-1 §10.5) | `app` — RFC §4.6 põe validação de formato de entrada em `app` |
| Destino do envelope inválido, e onde a validação ocorre | `cloudevents-protobuf-buf.md` §3.5 | `app` |
| Nack e extensão de visibilidade no fluxo de consumo | `uow-inbox-outbox.md` §6.4 | `app` |

`rationale` — **Por que a leitura estritamente literal foi descartada.** Ela é mais
limpa de defender e produz um vazio verificável: as quatro obrigações acima sairiam
`encaminhadas` sem dona, porque não existe âncora que as receba. A ANC-05 cobre
contexto, erros e segurança; a ANC-06, resiliência e observabilidade; a ANC-02
declarou explicitamente que fixa a existência dos mecanismos e remete o
comportamento por transporte a este artefato. Declarar `encaminhado` o que ninguém
pediu é a falha que o FND-05 §1.5 nomeou ao recusar transferir Parte-1 §7.6 e §7.7
— «transferir seria inventar dona». O espelho dessa falha é devolver ao remetente:
igualmente inválido, e com o custo de deixar o gesto de ACK sem norma em toda a
fundação.

`normativo` `TRP-01` — **O sujeito de uma regra `normativo` deste artefato é o
comportamento do `provider` de transporte.** Uma regra daqui pode dizer qual chave
de partição o publisher usa, que o cliente gRPC não reinicia o deadline recebido,
que o consumer não confirma offset antes do commit local, e que o publisher não
reserializa o payload que transporta.

`normativo` `TRP-02` — **A extensão a `app` é admitida apenas onde um artefato
precedente encaminhou a obrigação nominalmente a este.** As quatro obrigações
tabuladas nesta subseção são o conjunto fechado dessa extensão. Toda regra deste
artefato cujo sujeito seja `app` declara o sujeito — no próprio enunciado ou no
preâmbulo da subseção que a contém — e cita a origem do encaminhamento; sem as duas
coisas, é defeito de redação. As subseções que exercem a extensão são §6.2 e §7, e
ambas a declaram no preâmbulo.

`normativo` `TRP-03` — **Fora de `provider` e da extensão de `TRP-02`, uma regra
deste artefato é inválida por M4, ainda que tecnicamente correta.** Em particular,
este artefato não normatiza a superfície de uma API REST — desenho de recurso,
paginação, forma do corpo de erro —, porque isso é contrato e adapter de entrada, e
nenhum precedente o encaminhou a esta âncora. §9 declara o que fica de fora, e §18
registra o efeito sobre o critério de aceite correspondente.

`registro` — A tensão entre o escopo literal de ANC-04 e as obrigações que este
artefato recebeu é real e não se resolve por redação. `TRP-02` a administra com um
conjunto fechado e rastreável; a correção definitiva é reescrever o escopo
permitido da ANC-04, o que exige rito de versão da RFC (§14.2) e não cabe aqui.
Registrado como pendência em §18, com dona.

### §1.4 O que este artefato encaminha

`registro` — Assuntos que aparecem no texto como fronteira rotulada, com dona:

| Assunto | Dona | Onde é citado |
|---------|------|---------------|
| Taxonomia de erros de domínio, a sua retryability e o mapeamento semântico | FND-07 — `contexto-erros-seguranca.md`, **publicado**, sob ANC-05 | §10 e §12.4 — aqui só o mapeamento **estrutural** de código de protocolo, e o gesto que a disposição já implica (`TRP-53`) |
| Contexto de execução: campo do prazo, monotonicidade da propagação, sinal de cancelamento e categoria do estouro | FND-07, **publicado** (`CTX-18` a `CTX-23`) | §10.2 — aqui só o **valor** do prazo, que aquele artefato encaminhou (`GRP-16` a `GRP-18`) |
| Mecanismo de assinatura e verificação em fronteira não confiável | ANC-03 no contrato, plataforma no mecanismo; o critério de fronteira é `IDN-03` do FND-07 | §18.3, pendência 9 |
| Circuit breaker, bulkhead, degradação e o runbook de operação de DLQ | FND-08 ([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445)), sob ANC-06 | §11, §12 |
| Instrumento executável de verificação dos vetores, e o oráculo do round-trip | FND-09 ([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)), sob ANC-07 | §5, §18 |
| Cronograma, faseamento e plano de migração da adoção de Kafka na organização | FND-10 ([ARQ-447](https://lider-cap.atlassian.net/browse/ARQ-447)), sob ANC-09 | §15 |
| Redação, numeração definitiva e aceite dos ADRs acionados | FND-11 ([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)) | §17 |
| Escolha de registry de schemas em runtime — se existe, e qual é | `ADR-DMPF-N`, acionado por FND-05 §9.2, pendente no FND-11 | §11 — aqui só a operação, e condicionada |
| Fonte de verdade documental de OpenAPI e AsyncAPI: onde o arquivo vive, quem o mantém, como versiona | Pendência 2 da ANC-03 (FND-05 §10.4), sem dona declarada | §9, §16 |
| Dimensionamento de partições, retenção, IAM e topologia de broker em produção | Épicos de kernel e providers, com infraestrutura | §11, §12 |
| Implementação dos adapters de transporte | Épico de kernel e providers | todo o artefato |

### §1.5 Sucessão sobre a Parte-1

`registro` — **Esta tabela é uma proposta de sucessão, não a sua efetivação.** A RFC
§14.4 é explícita: «não há revogação implícita; um capítulo da Parte-1 só deixa de
valer quando esta tabela o declarar consolidado» — e a tabela da RFC lista os
capítulos §5 a §18 como **vigentes**. Nenhuma sub-spec pode consolidá-los por conta
própria, e as três anteriores registraram o mesmo realinhamento como pendência. A
coluna abaixo diz, portanto, o estado que este artefato **propõe** para a tabela da
RFC; até que ela seja alterada pelo rito de §14.2, a Parte-1 continua valendo no que
lá está declarado, e onde as duas divergirem prevalece o que este artefato normatiza
dentro da ANC-04, por autorização da própria âncora.

`registro` — Este artefato propõe a sucessão das subseções abaixo da base conceitual
(`plans/references/Parte-1-conceitual.md`, documento de trabalho não versionado).
<!-- ephemeral-ref-ok: a base conceitual é a fonte que este artefato sucede; nomear a origem é requisito de rastreabilidade da sucessão -->
Onde a coluna diz `consolidado`, o conteúdo daqui prevalece; onde diz `vigente`, a
Parte-1 segue valendo e este artefato não a substituiu.

| Subseção da Parte-1 | Estado | Observação |
|---------------------|--------|------------|
| §7.1 Uso de Protobuf — a lista de usos **por transporte** | `consolidação proposta` em §8, §10, §11 e §12 | O FND-05 §1.5 declarou esta parte reservada a ANC-04 e a manteve vigente na Parte-1 até aqui |
| §10.5 Fluxo de consumo — validação de envelope e comportamento de ACK do adapter | `consolidação proposta` em §6 e §7 | Sob a extensão de `TRP-02`; o restante de §10.5 permanece com FND-04 |
| §10.7 Kafka | `consolidação proposta` em §11 | A subseção inteira, que o FND-04 §1.5 manteve vigente |
| §10.8 SNS e SQS | `consolidação proposta` em §12 | A subseção inteira, idem |
| §10.9 Retry, DLQ e quarantine — o retry específico de cada transporte | `consolidação proposta` em §11 e §12 | A parte de operação e runbook segue `encaminhada` a FND-08, como o FND-04 §1.5 registrou |
| §7.6 REST e OpenAPI | `vigente` na Parte-1 | Não sucedida: por `TRP-03`, a superfície de uma API REST não é matéria desta âncora. §9 declara o recorte |
| §7.7 AsyncAPI | `consolidação parcial proposta` em §16 | A catalogação **por transporte** vem para cá; a fonte de verdade documental do canal permanece pendência 2 da ANC-03 |

`rationale` — **Por que duas sucessões ficam parciais.** Declarar Parte-1 §7 e §10
inteiros consolidados seria mais simples de ler e falso em dois pontos
verificáveis: §7.6 desenha a superfície de uma API, que `TRP-03` exclui, e §7.7
mistura a catalogação por transporte — que é desta âncora — com o endereço e a
manutenção do documento, que não são. Declarar consolidado o que não foi
substituído revoga norma por descuido, e revogação silenciosa é o pior defeito
possível num documento cuja função é ser a referência de quem implementa.

---

## §2. O bloco, a autoria e as obrigações herdadas

### §2.1 O bloco `provider` e o que a sua política permite

`recepcionado` — RFC §4.1 atribui ao `provider` «implementar uma ou mais portas com
tecnologia concreta», e nomeia como pertencentes a ele adapters, drivers, clientes
de SDK, repositórios e publishers. Não lhe pertencem regra de negócio, decisão de
caso de uso e a definição da porta que implementa.

`recepcionado` — RFC §6.2 dá ao `provider` política **permissiva**, com as
capabilities limitadas às «correspondentes à porta que implementa». É a única
política permissiva fora do composition root de `app`, e é o que torna o
`provider` o lugar do broker, do cliente HTTP e do SDK de nuvem.

`normativo` `TRP-04` — **Uma política de transporte que exija do `provider` uma
capability sem porta correspondente excede o bloco.** O critério é o de RFC §6.2:
a capability é permitida porque a porta a requer. Um publisher que precise ler
configuração de rede, resolver DNS e abrir conexão o faz porque a porta de
publicação existe; um `provider` que precise consultar um serviço remoto para
**desserializar** não tem porta que o justifique, e é vedado por `ENV-02` e
`ENV-23` do FND-05.

### §2.2 P0-3 e o critério de rejeição

`recepcionado` — RFC §2.3 declara que «nenhum componente, documento, configuração
ou contrato pode prometer exactly-once fim a fim», e RFC §12.3 registra
explicitamente, no campo de invariantes da ANC-04, que a vedação **não é relaxável
por transporte**. A semântica oficial é at-least-once com efeitos idempotentes.

`normativo` `TRP-05` — **Nenhuma linha deste artefato, e nenhuma política de
transporte que a ele se conforme, promete entrega ou processamento exactly-once fim
a fim.** A vedação alcança o nome do mecanismo: um recurso de broker comercializado
como «exactly-once» pode ser adotado pelo que ele faz de fato, e é descrito por
esse efeito, nunca pela promessa.

`normativo` `TRP-06` — **Um mecanismo nativo de broker que reduza duplicatas é
otimização, nunca substituto da idempotência de efeito.** §13 separa os três
escopos em que a deduplicação ocorre e fixa qual deles a inbox do FND-04 detém com
exclusividade.

`rationale` — A regra existe porque o vetor de violação de P0-3 mais provável não é
alguém escrever «garantimos exactly-once» num documento de arquitetura. É alguém
habilitar producer idempotence no Kafka, ou deduplicação de FIFO no SQS, e concluir
que a inbox ficou dispensável. As duas coisas são verdadeiras e não se somam: o
mecanismo do broker atua na publicação e numa janela limitada; a idempotência de
efeito é propriedade do consumo e não tem janela.

### §2.3 Matriz de obrigações herdadas

`registro` — A unidade desta matriz é a **obrigação atômica** — uma decisão
verificável —, não a ocorrência de texto. Uma mesma obrigação encaminhada por
várias linhas dos precedentes aparece aqui uma vez, com todas as origens. O estado
`condicionada` significa que este artefato a cumpre sob uma condição que não
controla, declarada na coluna de observação.

| # | Obrigação | Origem | Sujeito | Onde | Estado |
|---|-----------|--------|---------|------|--------|
| 1 | Mapeamento do nome lógico para o endereço concreto do transporte | `cloudevents-protobuf-buf.md` §1.4, §3.5 | `provider` | §4 | `quitada` |
| 2 | Convenção de tópico e de fila | `cloudevents-protobuf-buf.md` §1.4; `uow-inbox-outbox.md` §1.4 | `provider` | §11, §12 | `quitada` |
| 3 | Binding por protocolo, e o uso de Protobuf por transporte | `cloudevents-protobuf-buf.md` §1.4 (Parte-1 §7.1) | `provider` | §10, §11, §12 | `quitada` |
| 4 | **Byte-preservação do payload pelos transportes**, da qual depende a hipótese H1 da fórmula do `payload_hash` (`ENV-17`, `ENV-18`) | `cloudevents-protobuf-buf.md` §10.4, pendência 10 | `provider` | §5 | `quitada` |
| 5 | Gesto de ACK por transporte | `uow-inbox-outbox.md` §1.4 e §6.4; `cloudevents-protobuf-buf.md` §1.4; `contexto-erros-seguranca.md`, após `MAP-07` | `app` (extensão `TRP-02`) | §6 | `quitada` |
| 6 | `janela_redelivery`, que alimenta a invariante `INB-14` | `uow-inbox-outbox.md` §6.6 e §11.2 | `provider` | §6 | `quitada` |
| 7 | Nack e extensão de visibilidade no fluxo de consumo | `uow-inbox-outbox.md` §6.4 | `app` (extensão `TRP-02`) | §6, §12 | `quitada` |
| 8 | Commit de offset, visibilidade e redrive policy | `uow-inbox-outbox.md` §6.4 | `provider` | §11, §12 | `quitada` |
| 9 | Retry específico de cada transporte | `uow-inbox-outbox.md` §1.4, §7.4 (Parte-1 §10.9) | `provider` | §11, §12 | `quitada` |
| 10 | Retenção da DLQ e rito de replay | `cloudevents-protobuf-buf.md` §6.4; `uow-inbox-outbox.md` §7.4 | `provider` | §11, §12 | `condicionada` — a operação e o runbook são de FND-08 |
| 11 | Onde a validação do envelope ocorre, o gesto de rejeição e o destino do inválido | `cloudevents-protobuf-buf.md` §3.5 | `app` (extensão `TRP-02`) | §7 | `quitada` |
| 12 | Instante da publicação como observação de transporte | `cloudevents-protobuf-buf.md` §3.2 | `provider` | §4 | `quitada` |
| 13 | **Operação** do Schema Registry | `cloudevents-protobuf-buf.md` §1.4, §6.5, §9.2 | `provider` | §11 | `condicionada` — a **escolha** é `ADR-DMPF-N`, pendente |
| 14 | Endereço concreto de distribuição externa dos contratos: registro de módulos, pacotes npm, módulos Go, distribuição por transporte | `cloudevents-protobuf-buf.md` §7 | `provider` | §16 | `condicionada` — partilhada com os épicos de contratos |
| 15 | Forma de expressão executável do perfil de validação do envelope | `cloudevents-protobuf-buf.md` §10.4, pendência 9 | — | §7, §18 | `encaminhada` — a decisão é entre este artefato e FND-09; §7 fixa o conteúdo da validação, e o instrumento fica com FND-09 |
| 16 | *(consolidada na 4)* — o FND-05 §4.3 registra, em segundo ponto do texto, a mesma dívida da obrigação 4: um transporte que reescreva os bytes faz o hash divergir e converte redelivery legítima em R4 pela inbox de **FND-04** §6.4 | `cloudevents-protobuf-buf.md` §4.3 | `provider` | §5 | `quitada` na 4 — não é obrigação distinta |
| 17 | Catalogação AsyncAPI por transporte | `cloudevents-protobuf-buf.md` §1.5 (Parte-1 §7.7) | `provider` | §16 | `quitada` |
| 18 | Políticas por transporte, no geral | `rfc-dmpf-foundation-v0.1.md` §2.3 e §12.3 | `provider` | todo o artefato | `quitada` |

`registro` — Regras dos precedentes que este artefato **recepciona** e usa como
restrição, sem redefinir. A citação é a forma de cumprir M1: detalhar sem reabrir.

| Regra | Fonte | O que impõe a este artefato |
|-------|-------|------------------------------|
| `ENV-24` | FND-05 | A preservação do payload é byte-idêntica; truncar, reserializar, reordenar campos ou normalizar valores desqualifica o replay. É o critério de §5 |
| `ENV-17`, `ENV-18` | FND-05 | A fórmula do `payload_hash` e a hipótese H1, que §5 sustenta ou derruba |
| `ENV-11` | FND-05 | O conjunto de extensões do envelope é **fechado**: extensão nova entra por alteração daquele artefato, nunca por acordo entre produtor e consumidor. É o limite de `TRP-12` e `TRP-18` |
| `ENV-02`, `ENV-23` | FND-05 | A desserialização não depende de resolução remota. É o limite de §11 |
| `PTB-04` | FND-05 | Nome de contrato não carrega transporte, destino físico, ambiente nem tecnologia: `kafka`, `sqs`, `prod` e `staging` não aparecem em nome de contrato. É o limite de §4 |
| `BUF-09` | FND-05 | A autoridade de validação no repositório de contratos é o Buf, local e fail-closed |
| `INB-01` | FND-04 | A chave de deduplicação da inbox é `(consumer_name, message_id)`, e a unicidade é do schema. O `payload_hash` **não** integra a chave: é comparador posterior, que separa R2 e R3 de R4 |
| `INB-13`, `INB-14` | FND-04 | As duas propriedades do `payload_hash` (H1 e H2) e a invariante `retenção_inbox >= janela_redelivery`. É a segunda que §6 alimenta com valor |
| `GAR-08` | FND-04 | Uma poison message não bloqueia indefinidamente uma partição nem um grupo FIFO; o limite de tentativas é obrigatório |
| `GAR-11`, `GAR-12` | FND-04 | Quarantine e DLQ são distintos, quem implementa os dois declara o mapeamento, e o lado de consumo expõe profundidade e taxa de recusa |
| `OBX-09`, `OBX-18` | FND-04 | A elegibilidade ao claim decidida por estado e prazos vencidos, e a liberação de `locked_until` junto do recálculo de `available_at`. É o primeiro dos três relógios de §6, e não é reaberto aqui |
| `UOW-09`, `UOW-10` | FND-04 | A UoW não repete o callback automaticamente, e retry de conflito é política explícita, só em operação comprovadamente idempotente |

---

## §3. Rastreio de requisitos

`registro` — Esta seção transcreve os requisitos da story e fixa, para cada um, a
seção que o satisfaz e o seu estado. Ela abre o rastreio; §18 o fecha com o estado
final. Um requisito que este artefato não alcança sai marcado como tal, com dona:
encaminhamento não é aceite, e o rastreio não converte um no outro.

### §3.1 Critérios de aceite da ARQ-443

| # | Critério, como a story o enuncia | Seção | Estado previsto |
|---|----------------------------------|-------|-----------------|
| CA-1 | REST/JSON/OpenAPI definido como contrato externo com convenções publicadas | §9 | **parcial** — o papel na matriz e o comportamento do `provider` HTTP são satisfeitos; a superfície da API e a publicação documental ficam fora por `TRP-03`, com o efeito registrado em §18 |
| CA-2 | gRPC/Protobuf definido como padrão síncrono interno com deadline e cancelamento obrigatórios | §10 | previsto satisfeito |
| CA-3 | Kafka, SNS e SQS possuem convenções de envelope, chave de partição, ordenação, ACK e retry | §6, §11, §12 | previsto satisfeito |
| CA-4 | AsyncAPI definido como documentação dos canais e bindings assíncronos | §16 | previsto satisfeito no que é catalogação por transporte; a fonte de verdade documental permanece pendência da ANC-03 |
| CA-5 | Nenhuma promessa de exactly-once end-to-end em nenhum transporte | §2.2, §13, §14 | previsto satisfeito, com verificação própria na auditoria |
| CA-6 | Matriz de decisão validada contra ≥3 casos reais do inventário | §8 | previsto satisfeito para a matriz; a validação **não** alcança a política Kafka, que não tem base as-is |
| CA-7 | Fronteira explícita entre dedupe nativo de broker e inbox do DMPF documentada | §13 | previsto satisfeito |
| CA-8 | ADR-010 e ADR-011 registrados com contexto, alternativas, trade-offs e consequências | §17 | **acionamento**, não redação: os ADRs saem nomeados, com assunto, origem, alternativas e insumos, e a redação é do FND-11 |

### §3.2 Entregáveis da ARQ-443

| # | Entregável | Onde | Estado previsto |
|---|-----------|------|-----------------|
| 1 | Capítulo da RFC: políticas de transporte (matriz e política por transporte) | este artefato | previsto satisfeito |
| 2 | Tabela comparativa Kafka × SNS/SQS nas dimensões normativas | §14 | previsto satisfeito |
| 3 | Diretriz AsyncAPI com exemplo de documento de canal | §16 | previsto satisfeito |
| 4 | ADR-010 e ADR-011 **aceitos** | §17 | **fora do alcance deste artefato** — a redação e o aceite são do FND-11 ([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)); aqui os dois ficam no estado `acionado` |

### §3.3 Definition of Done da ARQ-443

| # | Item | Estado previsto |
|---|------|-----------------|
| 1 | Capítulo integrado à RFC no repositório canônico | previsto satisfeito — este arquivo, com o índice de `docs/dmpf/README.md` atualizado |
| 2 | ADR-010 e ADR-011 com status Aceito | **fora do alcance** — FND-11 |
| 3 | Revisão por infraestrutura de mensageria e um representante por stack concluída | **fora do alcance** — ato de revisão humana, registrado como pendência em §18 e no campo de revisão do cabeçalho |
| 4 | Cenários por transporte encaminhados ao catálogo de testes | previsto satisfeito — encaminhamento registrado em §18, com FND-09 como dona |

`rationale` — Os itens 2 e 3 do DoD, e o entregável 4, não são alcançáveis por
nenhum artefato desta série: um depende de outra âncora e o outro, de pessoas. A
alternativa seria omiti-los do rastreio e deixar a story parecer fechada. Ela foi
descartada porque o efeito prático é o pior possível — quem for fechar a ARQ-443
precisa saber exatamente o que ainda falta, e de quem depende.

---

## §4. Identidade lógica e endereçamento concreto

Um canal tem dois nomes: o lógico, que o contrato e o domínio conhecem, e o
concreto, que só o `provider` conhece. Esta seção fixa a relação entre os dois, e
por que ela precisa ser estável em uma granularidade mais fina do que «por
ambiente».

### §4.1 A separação dos dois nomes

`recepcionado` — `PTB-04` do FND-05 veda que o nome de um contrato carregue
transporte, destino físico, ambiente ou tecnologia: `kafka`, `sqs`, `prod` e
`staging` não aparecem em nome de contrato.

`normativo` `TRP-07` — **O endereço concreto do transporte — nome de tópico, nome de
fila, ARN, URL de endpoint — vive na configuração do `provider`, resolvida no
composition root.** Nenhum artefato de contrato, e nenhum tipo de domínio, o
menciona.

`normativo` `TRP-08` — **O `provider` recusa operar canal cujo endereço concreto não
esteja resolvido, e recusa na construção, não na primeira publicação.** A
pós-condição é observável nele: construído, o `provider` ou tem endereço resolvido
para todo canal que atende, ou falhou. Onde a resolução acontece, e como o
composition root a alimenta, é matéria de `app` — que este artefato não normatiza.

### §4.2 Estabilidade do binding: por mensagem, não por processo

`normativo` `TRP-09` — **Escolhido o endereço concreto para uma mensagem, todas as
tentativas de publicação daquela mensagem usam o mesmo endereço.** A estabilidade é
por mensagem e atravessa reinício de processo, recarga de configuração e mudança de
binding: uma tentativa cuja publicação anterior teve desfecho **incerto** não pode
ser repetida contra outro endereço.

`normativo` `TRP-46` — **A estabilidade exige estado persistido, e o `provider` o
grava antes do primeiro I/O.** O endereço concreto resolvido — ou uma revisão
imutável e resolvível do binding que o produza — é gravado junto ao item que será
publicado, antes de qualquer tentativa, e toda tentativa posterior lê esse estado em
vez de reconsultar a configuração vigente.

`rationale` — `TRP-09` sem `TRP-46` é inexequível, e a razão está no schema da
outbox: o FND-04 §4.1 exige apenas o destino **lógico**, sem endereço concreto, sem
transporte e sem versão de binding. Depois de uma publicação de desfecho incerto,
crash do relay e recarga de configuração, o processo que retoma não tem como saber
qual endereço a tentativa anterior usou — a informação nunca foi gravada. Persistir a
revisão do binding é o que torna a estabilidade verificável em vez de aspiracional.

`rationale` — **O que a instabilidade de binding causa, e o que ela não causa.** Ela
não produz, por si só, efeito duplicado: se `COE-04` for observado, o mesmo
`consumer_name` atende os dois transportes, a chave `(consumer_name, message_id)` de
`INB-01` colide, e a segunda entrega é classificada R2 ou R3 — absorvida. O dano real
é outro, e é triplo: a ordenação por chave se perde, porque `COE-06` nega ordem entre
transportes independentes; o trabalho de consumo é gasto duas vezes; e se os dois
adapters divergirem no `consumer_name` — o que `COE-04` proíbe justamente por isso —
aí sim há duas entradas de inbox e efeito duplicado.

`normativo` `TRP-10` — **Mudar o binding de um canal é operação de migração, não de
configuração.** A mudança só se aplica a mensagens ainda não publicadas por nenhuma
tentativa, e exige que o backlog do endereço anterior tenha sido drenado ou
explicitamente transferido. §15 fixa o gesto durante a coexistência de dois
transportes.

`normativo` `TRP-11` — **A chave de partição vem do envelope, nunca é inferida do
payload.** O `provider` a lê do atributo do envelope definido pelo FND-05 e a mapeia
para o mecanismo do transporte; um publisher que a derive por inspeção do conteúdo de
negócio quebra a ordenação sempre que o conteúdo mudar de forma sem mudar de
agregado.

### §4.3 O instante da publicação

`recepcionado` — O FND-05 §3.4 fixa que o atributo `time` do envelope é o **instante
do fato**, e que preenchê-lo com o momento da serialização ou da publicação destrói
a única informação temporal que o consumidor não consegue reconstruir. O instante da
publicação foi `encaminhado` a este artefato.

`normativo` `TRP-12` — **O instante da publicação é observação do `provider` e não
entra no envelope.** Quando necessário para operação ou auditoria, é registrado como
metadado do transporte — atributo de mensagem, header, campo de log ou span — e
nunca sobrescreve `time`, nem acrescenta atributo ao envelope, o que `ENV-11`
proíbe.

---

## §5. Byte-preservação do payload

Esta seção fecha a pendência 10 do FND-05, e é a de maior efeito operacional do
artefato. Ela não introduz uma regra nova de conteúdo: ela demonstra, hop por hop,
onde uma propriedade **já normatizada** pode se perder.

### §5.1 A obrigação recebida

`recepcionado` — Três regras do FND-05 constituem a dependência:

| Regra | O que fixa |
|-------|-----------|
| `ENV-17` | O `payload_hash` é computado exclusivamente sobre os bytes de `Any.value` do `data`, **exatamente como transportados**; nenhum atributo do envelope entra no cálculo |
| `ENV-18` | O cálculo ocorre sem desserializar e sem reserializar o payload; recomputar a partir de estrutura desserializada não satisfaz `ENV-17`, ainda que coincida por acidente na stack testada |
| `ENV-24` | A preservação é **byte-idêntica**: truncar, reserializar, reordenar campos ou normalizar valores desqualifica o replay, ainda que o conteúdo permaneça semanticamente equivalente |

`recepcionado` — O FND-05 §10.4 registra que a hipótese H1 da fórmula «é satisfeita
sob condição — a de que os bytes cheguem inalterados», e que essa condição «depende
da byte-preservação por transporte, que é de FND-06». O efeito de não a satisfazer é
nomeado na mesma pendência: um transporte que reescreva os bytes do payload
**converte redelivery legítima em R4** na inbox — colisão de identificador, a
disposição que descarta a mensagem para contenção.

`normativo` `TRP-13` — **Os bytes de `Any.value` entregues ao consumidor são
idênticos aos publicados pelo produtor.** A propriedade vale de ponta a ponta do
caminho de transporte, incluindo todo intermediário, e é obrigação do `provider` de
cada hop.

`normativo` `TRP-14` — **O gesto proibido é nomeado, não inferido.** No caminho de
uma mensagem, nenhum `provider` ou intermediário pode: desserializar o payload e
reserializá-lo; truncar; reordenar campos; normalizar valores; re-encodar de uma
representação para outra e de volta. A proibição alcança o caminho de retry e o de
contenção.

`normativo` `TRP-15` — **O hash é calculado sobre os bytes do payload, nunca sobre a
sua forma de transporte.** Onde o transporte exige envoltória textual, a envoltória é
desfeita antes do cálculo, e o cálculo incide sobre o `Any.value` recuperado.

### §5.2 A matriz de hops

`normativo` — A matriz é o instrumento de verificação de `TRP-13`. Uma linha marcada
`não conforme` descreve um caminho que **não pode** alimentar a inbox pela
comparação de `payload_hash`; usá-lo é violação, não escolha de trade-off.

| Hop | Onde os bytes vivem | Risco concreto | Transformação admitida | Conforme? |
|-----|---------------------|----------------|------------------------|-----------|
| Kafka, publicação direta | `record.value` | Nenhum, se o serializer escreve os bytes recebidos | Nenhuma | sim |
| Kafka, com `ProducerInterceptor` | `record.value` | Um interceptor que desserialize para inspecionar e devolva o objeto reserializado reescreve os bytes | Nenhuma: o interceptor pode ler, não substituir | sim, sob `TRP-17` |
| Kafka, headers | header de mensagem | Mover atributo do envelope para header e reconstruí-lo na recepção altera o envelope, não o payload — mas quebra o envelope autocontido | Nenhuma | sim, sob `TRP-18` |
| Kafka Connect com transform (SMT) | `record.value` | Um `transform` que converta o valor desserializa e reserializa por construção | Nenhuma | **não conforme** no caminho de mensagem de domínio |
| SNS → SQS **com** raw message delivery | corpo da mensagem | Nenhum: a mensagem chega sem a formatação de notificação do SNS | Um único par encode/decode de Base64 | sim |
| SNS → SQS **sem** raw message delivery | corpo da mensagem, dentro da envoltória JSON de notificação | O corpo original vira campo de um JSON de notificação; quem lê o corpo como payload lê a envoltória | Nenhuma | **não conforme** |
| SQS, publicação direta | corpo da mensagem | Duplo Base64 por camadas que codificam de novo o corpo já codificado | Um único par encode/decode | sim, sob `TRP-19` |
| Atributos de mensagem (SQS) e metadados (SNS) | atributo | Fundir atributo ao envelope, ou incluí-lo no cálculo do hash | Nenhuma: são laterais ao envelope | sim, sob `TRP-20` |
| gRPC unário, `application/grpc` | corpo binário | Nenhum: o transporte é opaco ao conteúdo | Nenhuma | sim |
| Connect em `application/proto` | corpo binário | Nenhum | Nenhuma | sim |
| Connect em `application/json` | corpo textual | ProtoJSON representa `bytes` em Base64, mas um caminho JSON que faça unpack e repack de `Any` reescreve os bytes | Nenhuma | **não conforme** para mensagem que alimente inbox |
| Envoy gRPC-JSON Transcoder | corpo textual | O filtro converte JSON e Protobuf a partir de descriptors — é transcodificação por definição | Nenhuma | **não conforme** para mensagem que alimente inbox |
| Claim-check | referência na mensagem, bytes no storage | O objeto transportado deixa de ser o payload; o hash calculado sobre a referência não é o hash do payload | Substituição do payload pela referência, sob `TRP-21` | sim, sob `TRP-21` |

`normativo` `TRP-16` — **Um caminho `não conforme` não é proibido em absoluto: é
proibido para mensagem que alimente a inbox pela comparação de `payload_hash`.** Uma
superfície REST transcodificada continua legítima como interface externa; o que ela
não pode é ser o hop de uma mensagem de domínio cuja idempotência dependa de
`ENV-17`.

`normativo` `TRP-17` — **Interceptor, filtro e middleware no caminho de publicação ou
de consumo podem observar o payload; não podem substituí-lo.** A distinção é
verificável: o objeto publicado é o mesmo que entrou, ou não é.

`normativo` `TRP-18` — **O envelope permanece autocontido.** Headers e atributos de
mensagem transportam apenas metadados de operação; nenhum atributo exigido pelo
perfil do envelope migra para eles.

`normativo` `TRP-19` — **A codificação textual de transporte é aplicada exatamente
uma vez.** Onde o corpo é textual, o `provider` que publica codifica, e o que consome
decodifica; nenhuma camada intermediária codifica de novo o que já está codificado.

`normativo` `TRP-20` — **Atributo de mensagem e metadado de notificação não entram no
envelope nem no cálculo do hash.** São laterais por construção: sobrevivem ou não ao
hop conforme o transporte, e depender deles quebra o envelope autocontido.

`normativo` `TRP-21` — **O claim-check está condicionado a uma evolução do perfil do
envelope, e até ela não é caminho conforme.** A razão é que ele colide com `ENV-17`:
se a referência ocupa o payload transportado, o `payload_hash` normativo passa a ser o
hash **da referência**, e não do conteúdo de negócio; se o hash continuar sendo o do
conteúdo no armazenamento, ele deixa de ser recomputável a partir do que trafega, o
que `ENV-18` exige. Nenhuma das duas leituras é compatível com o perfil vigente.

`encaminhado` — A modalidade de referência — contrato do ponteiro, fórmula de hash
aplicável e o predicado que distingue payload de referência — é matéria de ANC-03 e
depende de nova major do perfil (FND-05 §4.2, `ENV-20`). Este artefato registra a
necessidade e as três propriedades que a evolução precisa preservar: identidade de
bytes entre gravação e leitura no armazenamento; hash recomputável a partir do que
trafega; e proibição de reutilizar a mesma referência para conteúdo diferente. §18.3
registra a pendência.

`rationale` — **Por que a matriz, e não uma proibição geral.** «Não reserialize» é
verdadeiro e inútil: quem implementa um adapter não sabe que um SMT do Kafka Connect
reserializa por construção, nem que o SNS sem raw delivery embrulha o corpo, nem que
o transcoder do Envoy é transcodificação e não proxy. A pendência 10 não se fecha com
uma proibição; fecha-se dizendo **onde medir**. Cada linha `não conforme` acima é um
vetor negativo que o FND-09 pode transformar em teste.

---

## §6. Os três relógios e o gesto de ACK

O escopo da ARQ-443 pede a relação entre «visibility timeout e lease do inbox». A
expressão não tem referente: no FND-04 o lease é do claim do relay (§5.1) e da
outbox (`OBX-09`), e a **inbox não tem lease** — a sua relação temporal é `INB-14`.
Esta seção nomeia os três relógios que existem de fato, e só então fixa o gesto de
ACK que depende do terceiro.

### §6.1 Os três relógios

`recepcionado` — **Três relógios, com donos distintos.** Apenas os dois últimos são
matéria desta âncora; o primeiro está decidido pelo FND-04 e aparece aqui para que a
distinção seja legível, não para ser reaberto.

| Relógio | O que mede | Dono | Onde é decidido |
|---------|-----------|------|-----------------|
| **Lease de claim** | Por quanto tempo um item da outbox fica reservado a um worker de relay | `app` (relay) e o `provider` da outbox | FND-04 §5.1 e §5.4 — `OBX-09` decide elegibilidade por estado e prazos vencidos; `OBX-18` trata a liberação com recálculo de `available_at`. **Não** é reaberto aqui |
| **Intervalo entre tentativas** | Quanto tempo passa entre uma entrega não confirmada e a próxima | `provider` do transporte | §11 e §12, por transporte |
| **Horizonte de redelivery** | Por quanto tempo, no total, o transporte ainda pode reentregar automaticamente a mesma mensagem | `provider` do transporte | `TRP-23`, `TRP-24`; alimenta `INB-14` |

`normativo` `TRP-22` — **Confundir os três é erro de dimensionamento, e o canal declara
qual dos três está declarando.** O intervalo entre tentativas não é o horizonte, e
nenhum dos dois é o lease do relay: um valor declarado sem dizer qual relógio mede não
satisfaz `TRP-23`.

`recepcionado` — `INB-14` do FND-04 exige `retenção_inbox >= janela_redelivery`. O
valor da `janela_redelivery` foi encaminhado a este artefato.

`normativo` `TRP-23` — **A `janela_redelivery` é o horizonte de redelivery, não o
intervalo entre tentativas, e é declarada por canal com a sua fórmula de cálculo.**
Um canal que não a declare não satisfaz `INB-14`, porque a invariante não pode ser
verificada sobre valor desconhecido.

`normativo` `TRP-24` — **A fórmula é declarada por canal, com variáveis nomeadas e
unidade de tempo, e produz um limite superior fechado.** «Depende da configuração» não
satisfaz `TRP-23`: sem número, `INB-14` não é verificável.

| Transporte | Variáveis que compõem o limite superior |
|------------|----------------------------------------|
| SQS | `maxReceiveCount` × (visibilidade aplicada por tentativa + extensões concedidas, com o teto de `SQS-08b`), tudo limitado pela retenção da fila — o menor dos dois é o horizonte |
| SNS → SQS | A soma de **duas** janelas: a da política de entrega da assinatura, que pode reentregar à fila por dias, e a da própria fila pela linha acima. Tomar apenas a da fila subestima o horizonte, e é o erro que esta regra corrige |
| Kafka | Retenção efetiva do registro — o menor entre retenção por tempo e por tamanho, considerando política de compactação e armazenamento remoto quando houver — combinada com a política de posição inicial do grupo. O ponto de reinício é offset, não duração: só a retenção fornece a unidade de tempo |

`normativo` `TRP-24b` — **O canal declara, junto da fórmula, os parâmetros que a
instanciam.** Em Kafka: política de limpeza, retenção por tempo, retenção por tamanho,
armazenamento remoto e política de posição inicial. Em SQS: limite de recebimentos,
visibilidade base e retenção. Em SNS: a política de entrega da assinatura. Um canal que
omita qualquer um deles não tem `janela_redelivery` verificável.

`normativo` `TRP-25` — **Replay operacional não é redelivery automática e não entra
na `janela_redelivery`.** Reprocessar por ferramenta, a partir de DLQ, quarantine ou
do log além da retenção, é o gesto de `GAR-09` do FND-04, com auditoria e zona de
proteção declarada. Dimensionar a retenção da inbox para cobrir replay arbitrário
confunde as duas coisas e não tem limite superior.

### §6.2 A ordem do ACK

`recepcionado` — O FND-04 §6.3 fixa a sequência de consumo, e §6.4 fixa o **efeito
pretendido** de cada uma das sete disposições, encaminhando a este artefato «o gesto
concreto de cada efeito no broker — nack, extensão de visibilidade, `redrive policy`,
commit de offset».

`normativo` `TRP-26` — **A confirmação no broker ocorre depois do commit local, nunca
antes e nunca na mesma operação.** Não há transação distribuída entre a base e o
broker; a ordem é o que substitui a atomicidade, e inverter a ordem troca duplicata
por perda.

`normativo` `TRP-27` — **Gesto por disposição.** A tabela abaixo é o cumprimento da
obrigação 5 da matriz de §2.3. `app` é o sujeito nas colunas de gesto, sob a extensão
de `TRP-02`.

| Disposição (FND-04 §6.4) | Efeito pretendido | Gesto em Kafka | Gesto em SQS e SNS→SQS | Falha entre as ações |
|--------------------------|-------------------|----------------|------------------------|----------------------|
| **R1×D1** aplicado | confirma | commit do offset após o commit local | `DeleteMessage` pelo receipt handle após o commit local | Redelivery: a inbox classifica R2 e confirma sem escrever |
| **R1×D2** rejeitado por negócio | confirma | idem R1×D1 — `rejected` é estado commitado | idem R1×D1 | Redelivery: a inbox classifica R3, confirma e **não** reemite o rejection event (`INB-12`) |
| **R1×D3** falha transitória | não confirma; redelivery com backoff | nenhum commit de offset **e** retentar o mesmo registro no próprio consumidor, por laço ou por `pause` e `seek` ao offset dele — ver `TRP-47` | nenhum delete; deixar expirar a visibilidade, ou reduzi-la explicitamente para antecipar a tentativa | Nada a perder: em SQS a ausência de delete **é** o gesto; em Kafka o gesto é o laço, não a ausência de commit |
| **R1×D4** falha terminal | retira do fluxo, sem loop | publicar na DLQ **e depois** commitar o offset | `SendMessage` para a DLQ e depois `DeleteMessage`, ou redrive gerenciado ao atingir o limite de recebimentos | Duplicata na DLQ, nunca perda — `TRP-29` |
| **R2** reentrega de aplicada | confirma | commit do offset | `DeleteMessage` | Redelivery: R2 outra vez, idempotente |
| **R3** reentrega de rejeitada | confirma | commit do offset | `DeleteMessage` | Redelivery: R3 outra vez, sem reemissão |
| **R4** colisão de identificador | retira do fluxo, sem loop | igual a R1×D4 | igual a R1×D4 | igual a R1×D4 |

`normativo` `TRP-47` — **Em Kafka, não commitar não reentrega.** A posição de leitura
do consumidor avança a cada busca de registros, independentemente do offset
commitado: enquanto a atribuição da partição durar, o registro não disposto não volta
sozinho. O retry inline de `KFK-10` é implementado no próprio consumidor — laço sobre
o mesmo registro, ou suspensão da partição e reposicionamento no offset dele —, e o
orçamento de tempo desse laço respeita o intervalo máximo entre buscas configurado,
sob pena de o consumidor ser considerado morto e a partição, reatribuída.

`normativo` `TRP-28` — **Em Kafka, `enable.auto.commit` é `false`.** O commit
automático avança o offset por tempo decorrido, sem relação com o commit local, e
viola `TRP-26` sem produzir nenhum sinal.

`normativo` `TRP-29` — **O offset commitado nunca ultrapassa um registro ainda não
disposto, e o valor commitado é o do registro **seguinte**.** Disposto contiguamente
até o registro `n`, commita-se `n + 1` — é o offset de onde o consumo deve retomar.
Commitar `n` reentrega `n` indefinidamente. Em consumo por lote, um registro pendente
no meio do lote fixa o teto: commita-se o sucessor do último contíguo antes dele.

`normativo` `TRP-48` — **Perda de partição durante o processamento invalida o
resultado em curso.** No aviso de revogação ou de perda de atribuição, o consumidor
encerra ou cancela o trabalho pendente daquela partição e **não** commita o que
sobrar; o registro será reentregue ao novo dono, e a inbox o classificará. Trabalho
assíncrono destacado do ciclo de consumo é vedado nesse caminho: sem cancelamento
cooperativo, ele continua correndo enquanto outro membro já processa a mesma
partição.

`normativo` `TRP-30` — **Na contenção, a publicação na DLQ precede o avanço do
offset ou o delete.** A ordem inversa perde a mensagem se a publicação falhar; a
ordem correta, se falhar entre as duas ações, produz uma entrada duplicada na DLQ.
Duplicata em DLQ é operacionalmente recuperável; perda não é.

`normativo` `TRP-52` — **A contagem de tentativas de uma mensagem é observável no
transporte, e o `provider` a expõe.** O FND-07 encaminhou o «cabeçalho de tentativa» a
esta âncora, e o gesto varia por transporte: onde o broker mantém a contagem, o
`provider` a lê de lá; onde não mantém, ele a propaga em metadado de mensagem, sob
`TRP-18` — metadado de operação, nunca atributo de envelope. Sem contagem observável,
o limite de `TRP-31` não é verificável e `GAR-12` do FND-04 não tem o que expor.

`normativo` `TRP-31` — **O limite de tentativas é obrigatório, e a estratégia de
retry declara o seu efeito sobre a ordem.** `GAR-08` do FND-04 exige que uma poison
message não bloqueie indefinidamente uma partição nem um grupo FIFO. As duas
estratégias têm efeitos opostos, e nenhuma é neutra:

| Estratégia | Efeito sobre a ordem | Efeito sobre o bloqueio |
|------------|---------------------|-------------------------|
| Retry inline, na mesma partição ou grupo | Preserva a ordem por chave | Bloqueia as mensagens seguintes da mesma chave enquanto tenta — exige limite estrito para satisfazer `GAR-08` |
| Retry em canal separado | **Quebra** a ordem por chave: a mensagem retentada é reprocessada fora da sequência | Não bloqueia o fluxo principal |

`normativo` `TRP-32` — **Um canal que declare ordenação por chave não usa retry em
canal separado para mensagens desse canal.** As duas propriedades são incompatíveis;
declarar as duas é defeito de configuração, e §11 fixa qual prevalece por tipo de
canal.

---

## §7. Validação do envelope, rejeição e destino do inválido

Esta seção cumpre a obrigação 11 da matriz de §2.3, que é composta: **onde** a
validação ocorre, **qual** é o gesto de rejeição e **para onde** vai o envelope
inválido. Sujeito `app`, sob a extensão de `TRP-02`.

`recepcionado` — `INB-10` do FND-04 fixa que **envelope inválido não é D4**: a
validação ocorre no passo 1 da sequência canônica de consumo (FND-04 §6.3), no
`consumer adapter`, antes da UoW e antes de `registrar`. Uma mensagem com envelope inválido nunca recebe classificação de
recepção e não cai em nenhuma das sete disposições; o seu tratamento é contenção
direta, sem loop.

`recepcionado` — `ENV-13` do FND-05 fixa que a conformidade do envelope é verificável
**sem** desserializar o payload e **sem** acesso a rede: os atributos, os seus tipos
e os predicados de presença estão todos no envelope.

`normativo` `TRP-33` — **A validação do envelope ocorre no `consumer adapter`, antes
de qualquer efeito e antes da porta de inbox.** É o primeiro gesto do consumo, e a
sua posição não é negociável: validar depois de abrir a UoW gasta transação com
mensagem que não será processada, e validar depois de `registrar` grava chave de
mensagem que nunca foi válida.

`normativo` `TRP-34` — **A validação é local e fail-closed.** Ela não consulta rede,
não resolve schema remoto e não desserializa o payload de negócio — as três coisas
seriam necessárias apenas se o envelope não fosse autocontido, e `ENV-02`, `ENV-23` e
`ENV-13` garantem que ele é. Na dúvida, o envelope é inválido.

`normativo` `TRP-35` — **O gesto de rejeição é a contenção, não o nack.** Um envelope
inválido não volta ao fluxo: devolvê-lo ao broker produz o loop que `INB-10` existe
para evitar, porque a próxima tentativa falhará na mesma validação. O `provider`
retira a mensagem do fluxo normal no mesmo gesto de `R1×D4`, com a ordem de
`TRP-30`.

`normativo` `TRP-36` — **O destino do envelope inválido é a quarantine, não a DLQ do
canal.** `GAR-11` do FND-04 estabelece que quarantine e DLQ são mecanismos distintos
e que quem implementa os dois declara o mapeamento; este artefato faz a declaração
para o caso do envelope: DLQ recebe mensagem cujo **processamento** falhou de forma
terminal, e a quarantine recebe mensagem que nunca chegou a ser processável.
Um canal que implemente apenas um dos dois usa o que tem e declara a fusão.

`recepcionado` — **A contenção não é superfície de regime mais frouxo, e este artefato
não a trata como tal.** O FND-07 fecha esse ponto: `DAT-10` estende a proteção do dado
em repouso às superfícies de contenção, com a mesma força das do caminho normal, porque
o envelope que `GAR-07` manda preservar carrega dado de negócio e costuma ficar
armazenado por mais tempo que o original; `DAT-12` declara DLQ, quarantine e outbox
superfícies de dado de negócio, sujeitas ao regime de acesso nomeado e registrado, e
não ao regime «operacional»; e `DAT-14` limita a retenção.

`normativo` `TRP-54` — **O `provider` não trata a superfície de contenção como
infraestrutura.** A consequência prática para esta âncora é uma só, e é de desenho: a
fila, o tópico ou a tabela de contenção que ele configura fica no mesmo regime de
proteção e de retenção da superfície do caminho normal. Um canal cuja DLQ viva fora
desse regime não é conforme, ainda que todas as regras de gesto e de ordem de §6 sejam
observadas.

`rationale` — A regra existe porque o efeito combinado das outras é perverso se ela
faltar: `TRP-13` obriga a preservar os bytes, `GAR-07` a preservar informação suficiente
para replay, e `TRP-30` a publicar na contenção antes de confirmar. Somadas, as três
produzem a cópia mais completa e mais duradoura do dado de negócio — e o FND-07 nomeia
como anti-padrão exatamente deixá-la sem proteção «porque é infraestrutura
operacional».

`rationale` — A distinção entre DLQ e quarantine não é burocrática. O conteúdo de uma
DLQ é replayável pelo mesmo consumidor depois de corrigir o defeito no consumidor; o
conteúdo de uma quarantine de envelope não é — se o envelope está inválido, replayar contra o mesmo
consumidor reproduz a falha, e o que precisa ser corrigido está no produtor. Misturar
os dois transforma a fila de replay num depósito de mensagens que nunca serão
replayadas.

### §7.1 Limites de entrada, antes do decode

`recepcionado` — O FND-07 encaminhou a este artefato, na matriz de §8.8 e no vetor de
*tampering* de §7, a validação de **tamanho e profundidade antes da desserialização** e
a proteção contra **payload excessivo e expansão abusiva** — no que é transporte; a
parte de contrato é de ANC-03.

`normativo` `TRP-49` — **Todo canal declara um teto de tamanho para a mensagem
recebida, e o `provider` o aplica antes de desserializar.** O teto é do canal, não do
processo, e vale sobre os bytes como chegaram — antes de qualquer decodificação de
transporte. Uma mensagem acima do teto é contida por `TRP-35`, sem decode e sem loop.

`normativo` `TRP-50` — **O `provider` não expande conteúdo recebido sem limite.** Onde o
transporte ou o codec admitirem compressão, o `provider` aplica teto ao tamanho
**expandido** e à razão de expansão, e ambos são declarados no canal. Sem os dois, uma
mensagem pequena dentro do teto de `TRP-49` pode consumir memória arbitrária ao ser
descomprimida.

`normativo` `TRP-51` — **A profundidade de aninhamento é limitada e verificada antes do
decode do payload de negócio.** A validação do envelope de §7 é rasa por construção —
`ENV-13` garante que ela não desserializa o payload —, e é justamente por isso que o
teto de profundidade precisa ser aplicado como limite do decodificador, não como
inspeção da estrutura já materializada.

`registro` — Estas três regras são de transporte, e param onde começa o contrato: o
que o **schema** admite como tamanho de campo e como cardinalidade é de ANC-03. A
divisão é a mesma que o FND-07 registrou ao encaminhar o assunto às duas âncoras.

### §7.2 Onde a forma executável do perfil ainda falta

`encaminhado` — A **forma executável** do perfil de validação — o artefato que expressa
os predicados de modo verificável, e onde ele vive no repositório de contratos —
permanece a pendência 9 do FND-05 §10.4. Este artefato fixa **onde** a validação
ocorre, o seu gesto e o seu destino; o instrumento que a executa é de FND-09
([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)), sob ANC-07.

---

## §8. A matriz de decisão de transporte

### §8.1 Os critérios, antes da matriz

`normativo` `TRP-37` — **A escolha do transporte é decidida por critérios
declarados, não por preferência de equipe.** Os critérios abaixo são avaliados nesta
ordem, e o primeiro que discrimina decide:

| Ordem | Critério | Pergunta que ele responde |
|-------|----------|---------------------------|
| 1 | **Quem consome** | O consumidor é externo à organização, ou as duas pontas são nossas? |
| 2 | **Acoplamento temporal** | O chamador precisa da resposta para prosseguir, ou o efeito pode ocorrer depois? |
| 3 | **Cardinalidade do destino** | Um destino conhecido, ou N consumidores que podem crescer sem o produtor saber? |
| 4 | **Ordenação exigida** | Existe ordem que o consumidor precisa observar? Por qual chave? |
| 5 | **Retenção e reprocessamento** | O consumidor precisa reler o que já foi entregue? |

`rationale` — A ordem importa porque os critérios não são independentes. «Quem
consome» vem primeiro porque nenhuma vantagem técnica de Protobuf compensa exigir
que um parceiro externo gere stubs; «acoplamento temporal» vem antes de
cardinalidade porque uma chamada síncrona com muitos destinos é um problema de
desenho, não de transporte.

### §8.2 A matriz

`normativo` `TRP-38` — **Matriz de decisão.** Nenhuma linha promete exactly-once fim
a fim.

| Uso | Transporte | Contrato | Garantia de entrega | Ordenação |
|-----|-----------|----------|---------------------|-----------|
| Interface para consumidor externo | REST/JSON sobre HTTP | OpenAPI | request/response, sem retentativa implícita | não se aplica |
| Chamada síncrona interna, com resposta necessária | gRPC sobre HTTP/2 | `.proto` versionado | request/response com deadline propagado | não se aplica |
| Evento de domínio, consumidor interno, **caso novo** | **Kafka** | CloudEvents em Protobuf | at-least-once | por partição, pela chave declarada |
| Evento de domínio, consumidor interno, **acervo existente** | SNS/SQS | CloudEvents em Protobuf | at-least-once | por grupo, em FIFO; nenhuma, em standard |
| Fan-out para N consumidores com filtro por atributo | SNS → SQS | CloudEvents em Protobuf | at-least-once | por grupo, em FIFO |
| Trabalho enfileirado, um consumidor por mensagem, sem ordem | SQS standard | CloudEvents em Protobuf | at-least-once | nenhuma |

`normativo` `TRP-39` — **Kafka é o transporte-alvo do assíncrono de domínio.** Um
canal novo de evento de domínio usa Kafka, salvo quando um dos critérios de §8.1
discrimina a favor de SNS/SQS — e o caso é registrado no canal, com o critério que o
decidiu.

`normativo` `TRP-40` — **SNS/SQS permanece normatizado, não tolerado.** O acervo
existente continua conforme enquanto observar §12; migrar não é condição de
conformidade. Os dois casos em que SNS/SQS é escolha técnica superior, e não legado,
são o fan-out com filtro por atributo de mensagem e a fila de trabalho sem ordenação.

`registro` — **A ausência de base as-is de Kafka, declarada.** O inventário da
organização registra, como fato de ausência, zero clientes Kafka versionados nos
repositórios inventariados, e caracteriza a mensageria observada como SQS-cêntrica.
A política de §11 é portanto **prospectiva**: ela não descreve prática vigente, e não
foi validada contra operação real. A RFC §1.6 fixa que «a evidência lastreia; ela não
cria a norma» — o que autoriza normatizar sem base observada, e exige declarar a
ausência em vez de simulá-la. A adoção de Kafka como modelo-alvo da organização é
decisão de plataforma, registrada aqui como premissa recebida, não como conclusão
deste artefato.

`normativo` `TRP-41` — **Enquanto a política de §11 não passar por revisão de
infraestrutura de mensageria, ela vale como norma de desenho e não como autorização
de operação.** Um canal Kafka novo exige a revisão nomeada no cabeçalho deste
documento; §18 registra a pendência.

### §8.3 Confronto com o acervo

`registro` — O que segue é a classificação dos casos reais do inventário pela matriz
de §8.2. O confronto demonstra que a matriz **classifica** o acervo; ele não valida a
política Kafka, que não tem caso observado.

| Caso do inventário | Como está hoje | Linha da matriz | Escolha de transporte |
|--------------------|----------------|-----------------|-----------|
| `shared-ro-sync-services` — sync de read-only de Postgres para MSSQL | SNS → SQS com `FilterPolicy`; consumidores idempotentes por upsert | fan-out com filtro por atributo | **conforme e permanece**: é o caso em que SNS/SQS é superior, não legado |
| `shared-titulos-services` | SQS com outbox e inbox; worker lê a outbox depois do commit | evento de domínio, acervo existente | **conforme**: a sequência já é a de FND-04; a migração para Kafka é opcional |
| `telesena-live-services` | SQS; delete depois do sucesso; retry por visibility timeout; unique index e guards parciais | evento de domínio, acervo existente | **conforme com ressalva**: o gesto de ACK satisfaz `TRP-26`, mas a idempotência por unique index é de negócio, não a inbox de FND-04 — §13 separa as duas camadas |
| `rendafacil-services` — API Fiber com `pkg/sqs` | REST na borda e SQS internamente | duas linhas: interface externa e evento interno | **classificada**: a separação REST na borda e mensageria dentro é o que a matriz prescreve |
| `telesena-ativavel-services` | SQS via cliente compartilhado | evento de domínio, acervo existente | **classificada**: o cliente compartilhado favorece a convergência de §12 |

`registro` — **Classificar a escolha de transporte não é atestar conformidade da
implementação.** O inventário registra, para dois dos casos acima, lacuna que este
artefato não pode declarar resolvida: em `rendafacil-services`, delete em erro que pode
perder mensagem, com idempotência ponta a ponta não garantida; em
`telesena-ativavel-services`, lacuna de idempotência no consumidor. Ambas violam a
ordem de ACK de `TRP-26` ou a semântica que P0-3 fixa. A coluna acima diz que a matriz
**classifica** o caso; o veredicto de conformidade da implementação é do épico que a
corrigir, e CA-6 não o usa como validação.

`registro` — Duas observações do inventário que a matriz não resolve, e que ficam
nomeadas: os repositórios com SQS em runtime usam **envelope JSON ad hoc**, que o
FND-05 substitui; e `golibs` mantém **dois caminhos SQS paralelos**, um sobre
Watermill e outro com SDK v1. Convergir os dois é trabalho dos épicos de kernel, não
deste artefato.

---

## §9. REST e JSON como interface externa

Esta é a seção mais curta do artefato, e o recorte é deliberado. Por `TRP-03`, a
superfície de uma API REST — desenho de recurso, paginação, forma do corpo de erro,
versionamento da URL — é contrato e adapter de entrada, e nenhum precedente a
encaminhou a esta âncora. O que segue é o que **é** política de transporte.

`normativo` `RST-01` — **REST sobre HTTP com corpo JSON é o transporte da interface
para consumidor externo à organização.** O critério é o primeiro de §8.1: obrigar um
consumidor externo a gerar stubs de Protobuf é custo de integração sem ganho de
interoperabilidade.

`normativo` `RST-02` — **O `provider` HTTP não retenta método sem semântica
idempotente.** A retentativa automática é admitida onde o método a garante — `GET`,
`HEAD`, `PUT`, `DELETE` — e vedada em `POST`, salvo quando o endpoint declara chave
de idempotência e o `provider` a envia.

`normativo` `RST-03` — **O `provider` HTTP aplica timeout em toda chamada de saída, e
o timeout deriva do deadline em vigor.** Um cliente HTTP sem timeout herda a espera
indefinida que §10 proíbe no transporte irmão; a diferença entre os dois é que o HTTP
não carrega o deadline no protocolo, e por isso o `provider` o converte.

`normativo` `RST-04` — **O `provider` HTTP não expõe canal externo cujo contrato
publicado ele não possa referenciar.** A pós-condição é dele: a configuração do canal
aponta o contrato vigente, e um canal sem essa referência não é operável.

`encaminhado` — **A existência, a forma e a manutenção do documento OpenAPI são de
`contract package`**, matéria de ANC-03, e a fonte de verdade documental permanece a sua
pendência 2 (§1.4). Este artefato não obriga a publicar; obriga o `provider` a não
operar sem referência ao que foi publicado.

`encaminhado` — Desenho de recurso, paginação, forma do corpo de erro e versionamento
de superfície permanecem na base conceitual (Parte-1 §7.6, `vigente` por §1.5), sem
dona declarada. §18 registra o efeito sobre o critério de aceite CA-1.

`rationale` — **Por que não normatizar a superfície aqui, mesmo com o ticket
pedindo.** Seria fácil, e seria inválido por M4. A RFC §4.1 põe a adaptação de
protocolo e a validação de entrada em `app`; RFC §4.6 confirma a validação de formato
como `app`; e a ANC-04 autoriza `provider`. Uma regra sobre paginação teria sujeito
`app` sem encaminhamento nominal que a autorize — a exata hipótese que `TRP-03`
invalida. A alternativa honesta é entregar menos e dizer o que falta, em vez de
entregar fora do escopo e criar norma que a próxima leitura da RFC derruba.

---

## §10. gRPC como padrão síncrono interno

Esta seção consolida o estudo de padronização de comunicação síncrona anexo ao épico,
que já circulou na organização. Onde este artefato divergir daquele estudo, a
divergência é explícita e a justificativa vai para o ADR acionado em §17.

### §10.1 O transporte e os perfis

`normativo` `GRP-01` — **gRPC sobre HTTP/2 é o transporte da chamada síncrona interna
quando as duas pontas são nossas.** O contrato é o `.proto` versionado governado pelo
FND-05.

`normativo` `GRP-02` — **Três perfis, com critério de escolha declarado:**

| Perfil | Quando | Característica |
|--------|--------|----------------|
| gRPC puro | chamada serviço a serviço interna | `application/grpc`, trailers nativos, HTTP/2 |
| Connect | cliente de browser, edge, cliente HTTP diverso | unary em `application/proto` ou `application/json`; HTTP/1.1 para a maior parte dos RPCs |
| Transcodificação gRPC-JSON | quando superfície REST/JSON canônica for requisito formal | depende de descriptor set e de anotação HTTP no `.proto` |

`normativo` `GRP-03` — **Os perfis Connect em `application/json` e a transcodificação
gRPC-JSON não são caminho de mensagem que alimente inbox.** A matriz de §5.2 os
classifica como transcodificação: eles reescrevem a representação por construção, e a
identidade de bytes de `TRP-13` não sobrevive. Como interface de leitura ou de
comando externo, permanecem legítimos.

### §10.2 Deadline e cancelamento

`normativo` `GRP-04` — **Toda chamada gRPC de saída carrega deadline.** Chamada sem
deadline é violação de política, não omissão tolerável: sem ele, o cliente pode
esperar indefinidamente e o servidor não tem como saber que ninguém aguarda a
resposta.

`normativo` `GRP-05` — **O deadline é propagado e nunca reiniciado.** Cada salto
recebe o prazo restante e o repassa ao próximo já decrescido pelo que consumiu. Um
`provider` que substitua o prazo recebido por um valor de configuração própria quebra
a cadeia: a borda deixa de governar o tempo total.

`recepcionado` — `CTX-18` e `CTX-19` do FND-07 fixam que, **no contexto de execução**,
o prazo é um instante absoluto e a propagação é monotônica: nenhum bloco o estende, nem
em retry. O anti-padrão que aquele artefato nomeia é o contexto carregar uma duração
fixa e cada serviço reiniciar a contagem.

`normativo` `GRP-06` — **No wire, o `provider` gRPC transmite a duração restante, e o
receptor reconstrói o instante.** A conversão é feita no momento do envio, descontando
o tempo já decorrido, porque é assim que o cabeçalho de timeout do protocolo é
definido; o receptor a soma ao seu próprio relógio para recuperar o instante de
`CTX-18`. Isso **não** é o anti-padrão de `CTX-19`: a duração transmitida decresce a
cada salto, e nenhum salto reinicia a contagem. Transmitir instante absoluto no wire é
que seria defeito, porque o desvio de relógio entre as máquinas o deslocaria.

`normativo` `GRP-07` — **O `provider` propaga o cancelamento, nos dois sentidos.** O
cliente sinaliza a desistência ao servidor; o servidor expõe o sinal recebido a quem
executa o trabalho, e o repassa às chamadas de saída que originou. Isso é o que o
`provider` controla, e é o que esta regra obriga.

`recepcionado` — **Encerrar o trabalho ao receber o sinal já está normatizado, e não
por este artefato.** A biblioteca de transporte propaga o cancelamento; ela não
interrompe a execução do caso de uso. O FND-07 fecha essa coluna: `CTX-21` obriga o
`provider` que executa I/O a respeitar o cancelamento e o prazo, e declara defeito
iniciar operação remota com contexto já cancelado ou expirado; `CTX-22` fixa que o
cancelamento interrompe trabalho pendente e **não** desfaz efeito já concluído. O que
`GRP-07` acrescenta é o gesto no protocolo; o dever de observar o sinal é de lá.

`recepcionado` — O FND-07 fixa o campo do prazo, a monotonicidade da propagação, o
sinal de cancelamento, o dever de respeitá-lo e a categoria própria do estouro, e
encaminha a este artefato uma única coisa: **o valor**. «Quanto tempo é o prazo não é
matéria daqui», diz aquele artefato — é matéria desta seção.

`normativo` `GRP-16` — **O prazo é declarado por método, não por serviço nem por
processo.** Métodos do mesmo serviço têm custos diferentes, e um valor único ou é
frouxo para o mais rápido ou é apertado para o mais lento. A declaração acompanha a
configuração de cliente de `GRP-08`.

`normativo` `GRP-17` — **O prazo de um método é menor que o do chamador que o invoca,
com folga para o próprio salto.** É o que torna a monotonicidade de `CTX-19`
alcançável na prática: se cada salto declarar o mesmo valor da borda, o último salto
começa já sem prazo. A folga é declarada junto do valor.

`normativo` `GRP-18` — **O prazo da borda é derivado do requisito de quem chama, não
do tempo que a implementação leva hoje.** Calibrar o prazo pela latência observada
transforma degradação em norma: o valor é o limite acima do qual a resposta deixa de
ter valor para o chamador, e a latência observada serve para verificar se a
implementação cabe nele — nunca para definí-lo.

### §10.3 Retry, balanceamento e disponibilidade

`normativo` `GRP-08` — **A política de retry é declarada por método e derivada da
idempotência dele.** A configuração declara explicitamente os códigos de status
retentáveis; lista vazia significa nenhum retry, e a ausência de política não é
interpretada como retry livre.

`normativo` `GRP-09` — **Um método sem semântica idempotente comprovada não recebe
retry automático.** O critério é o mesmo de `UOW-10` do FND-04 para retry de
conflito: «só em operação comprovadamente idempotente». Retentar uma operação de
efeito colateral não idempotente porque o transporte reportou indisponibilidade
duplica o efeito.

`normativo` `GRP-10` — **O backoff é exponencial com jitter.** Sem jitter, os
clientes que falharam ao mesmo tempo retornam ao mesmo tempo, e o retry converte uma
indisponibilidade curta em tempestade sincronizada.

`normativo` `GRP-11` — **O balanceamento entre múltiplos endereços resolvidos é
configurado explicitamente.** O comportamento padrão de conectar ao primeiro endereço
não distribui carga: ele resolve conectividade, não balanceamento, e depender dele
concentra o tráfego num único backend.

`normativo` `GRP-12` — **Todo serviço gRPC expõe verificação de saúde pelo protocolo
padrão de health checking**, com o estado por serviço, não apenas no processo.

`normativo` `GRP-12b` — **Expor não basta: o cliente habilita a verificação e usa
política de balanceamento compatível com ela.** O comportamento de reter tráfego até o
backend estar pronto, e de suspender o envio quando um backend saudável deixa de
estar, depende de o cliente ativar o acompanhamento na sua configuração de serviço e
de a política de balanceamento observar o resultado — não decorre da existência do
endpoint no servidor. `GRP-11` e esta regra são interdependentes.

### §10.4 Erro

`normativo` `GRP-13` — **O modelo de erro é o do próprio gRPC: código, mensagem e
detalhes opcionais.** Os códigos são parte do contrato operacional do método, e
vários deles — argumento inválido, não encontrado, já existe, pré-condição falhou,
abortado — só existem se a aplicação os produzir; a biblioteca não os gera.

`normativo` `GRP-14` — **Onde houver transcodificação para HTTP, o mapeamento de
código segue a tabela canônica do ecossistema**, e não uma convenção local:

| Código gRPC | HTTP |
|-------------|------|
| `OK` | 200 |
| `INVALID_ARGUMENT` | 400 |
| `UNAUTHENTICATED` | 401 |
| `PERMISSION_DENIED` | 403 |
| `NOT_FOUND` | 404 |
| `ALREADY_EXISTS` | 409 |
| `RESOURCE_EXHAUSTED` | 429 |
| `UNAVAILABLE` | 503 |
| `DEADLINE_EXCEEDED` | 504 |
| `CANCELLED` | 499 — não padronizado em HTTP, mas canônico no ecossistema |

`recepcionado` — A **taxonomia de erros de domínio** — o que é falha retentável, o que
é terminal, e como um erro de negócio se distingue de um erro técnico — está
normatizada pelo FND-07, sob ANC-05, e não é reaberta aqui. Este artefato fixa apenas o
mapeamento **estrutural** de código de protocolo.

`recepcionado` — O ponto de contato entre as duas âncoras está fechado: `ERR-09` do
FND-07 garante que todo erro concreto resolve para um booleano de retryability, e
`MAP-07` que o consumer adapter recebe esse booleano já resolvido e **deriva D3 ou D4
dele**. A exigência de `INB-09` do FND-04 — que a classificação exista e que a
disposição derive dela — passa a ter fonte. `TRP-53` fecha o elo até o gesto.

### §10.5 Segurança do transporte

`normativo` `GRP-15` — **TLS é obrigatório em qualquer transporte gRPC em produção**,
e a autenticação mútua por certificado de cliente é o padrão recomendado para tráfego
interno entre serviços.

`registro` — **Divergências em relação ao estudo anexo.** Nenhuma, no plano
normativo: as decisões de perfil, versionamento, governança de schema, retry,
balanceamento, health checking e mapeamento de erro são as do estudo. Este artefato
acrescenta três regras que o estudo não tinha, e nenhuma o contradiz: `GRP-03`
(transcodificação não é caminho de mensagem que alimente inbox), `GRP-06` (o prazo
propagado é relativo) e `GRP-09` (o vínculo explícito entre retry e a idempotência
exigida por `UOW-10`). As três decorrem de regras já publicadas pelo FND-04 e pelo
FND-05, que o estudo, anterior a eles, não podia considerar.

---

## §11. Kafka

`registro` — A política desta seção é prospectiva, pelo que §8.2 registrou: não há
cliente Kafka versionado no acervo inventariado. Ela vale como norma de desenho, e a
autorização de operação depende da revisão de infraestrutura nomeada em `TRP-41`.

### §11.1 Nomenclatura e topologia de tópico

`normativo` `KFK-01` — **O nome do tópico é endereço concreto e vive na configuração
do `provider`** (`TRP-07`). Ele deriva do tipo de evento do envelope e da major do
contrato, na forma `<contexto>.<agregado>.<evento>.v<major>`, em minúsculas, com
ponto como separador, e o resultado tem no máximo **249 caracteres** — limite do
próprio Kafka, incluindo o prefixo de ambiente de `KFK-02` quando houver.

`normativo` `KFK-01b` — **A derivação do nome a partir do tipo do envelope é uma
transformação total e declarada por canal.** O tipo do envelope tem a forma que o
FND-05 fixou, e não decompõe por si só em contexto, agregado e evento: `credito`,
`proposta` e `aprovada` são separáveis, mas um fato nomeado como palavra composta não
é. Por isso a decomposição é **declarada** no canal (`ASY-02`), não inferida por
algoritmo, e duas declarações distintas não podem produzir o mesmo nome de tópico.

`normativo` `KFK-01c` — **O alfabeto é o mesmo em todos os segmentos, prefixo de
ambiente incluído**: minúsculas, dígitos, ponto, hífen e sublinhado. Como o Kafka
trata ponto e sublinhado como colidentes na sua telemetria, dois canais cujos nomes
difiram apenas por esses dois caracteres são proibidos.

`normativo` `KFK-02` — **O ambiente não entra no nome do tópico quando há isolamento
por cluster ou por conta.** Onde o isolamento não existir, o prefixo de ambiente é
declarado na configuração do canal, e a declaração é parte da conformidade — o que
não se admite é o mesmo nome designar ambientes diferentes sem isolamento nem
prefixo.

`normativo` `KFK-03` — **Por padrão, um tópico transporta um tipo de evento.**

`normativo` `KFK-03b` — **Múltiplos tipos no mesmo tópico exigem, cumulativamente:**
conjunto **fechado** e declarado na catalogação do canal; variação modelada no
contrato, e não por acréscimo livre ao longo do tempo; e ordenação declarada válida
para o conjunto inteiro. Sem os três, vale `KFK-03`.

`normativo` `KFK-04` — **A contagem de partições é propriedade do canal, declarada,
junto do particionador em uso e da codificação de bytes da chave.** Sob o
particionador padrão, o destino é função da chave e do número de partições; sem
declarar os três, `KFK-06` não é verificável.

`normativo` `KFK-04b` — **Aumentar partições é operação de migração.** Sob o
particionador padrão, mensagens de uma mesma chave passam a cair em partição
diferente das anteriores, e fazê-lo com backlog pendente quebra a ordenação de
`KFK-06`: exige drenagem prévia ou aceitação declarada da quebra. **Drenar não
basta**: os offsets das partições novas precisam ser inicializados para o grupo antes
de a produção ser liberada, porque um grupo que só resolve a posição inicial ao
encontrar a partição pode pular o que foi escrito nela no intervalo.

### §11.2 Chave, ordenação e consumo

`recepcionado` — A base conceitual fixa, para Kafka, `record.key = partition_key` e
`record.value = CloudEvent serializado em Protobuf`, com ordenação existindo apenas
dentro da partição e o offset confirmado após o commit do efeito.

`normativo` `KFK-05` — **A chave do registro é a chave de partição do envelope**, lida
conforme `TRP-11`, e o valor do registro é o envelope serializado, preservado byte a
byte conforme `TRP-13`.

`normativo` `KFK-06` — **A ordenação garantida é por partição, pela chave declarada —
nunca global.** Um canal que precise de ordem total precisa de partição única, e
partição única é limite de throughput declarado, não configuração invisível.

`normativo` `KFK-07` — **O grupo de consumo é declarado por canal e por
consumidor lógico**, e o seu nome é estável: ele determina onde o offset é retomado, e
trocá-lo reprocessa desde o início da retenção ou perde o backlog, conforme a política
de reinício.

`normativo` `KFK-08` — **O `consumer_name` da inbox de FND-04 é o consumidor lógico, e
não o grupo de consumo do transporte.** A distinção é o que permite ao mesmo
consumidor lógico ser servido por transportes diferentes durante a coexistência sem
duplicar efeito — §15 depende desta regra.

`normativo` `KFK-09` — **A concorrência de consumo respeita a partição**: dentro do
grupo, o desenho garante um processador por partição por vez. Processar em paralelo os
registros de uma mesma partição anula a ordenação de `KFK-06`.

`registro` — A exclusividade é **do protocolo de atribuição, não do relógio**. Durante
um rebalance, o aviso de perda de atribuição pode chegar depois de outro membro já ter
assumido a partição; nesse intervalo, trabalho ainda em curso no membro anterior
concorre com o novo dono. É `TRP-48` que fecha a janela, exigindo cancelamento e
vedando trabalho destacado do ciclo de consumo — não este enunciado.

`normativo` — O commit de offset, o comportamento em rebalance e o gesto por
disposição são os de §6.2 (`TRP-26` a `TRP-32`). Esta seção não os repete.

### §11.3 Retry e contenção

`normativo` `KFK-10` — **O retry de falha transitória é inline, com backoff, e
limitado.** É a estratégia que preserva a ordem por chave; o limite é o que satisfaz
`GAR-08` do FND-04, que veda a poison message bloqueando a partição
indefinidamente.

`normativo` `KFK-11` — **Canal com ordenação declarada não usa tópico de retry** —
`TRP-32`. Onde a ordem não for exigida, o tópico de retry é admitido, e o canal
declara que a ordem não é garantida.

`normativo` `KFK-12` — **Cada canal tem DLQ nomeada, e a publicação nela precede o
avanço do offset** (`TRP-30`). A DLQ preserva o envelope original byte a byte
(`TRP-13`), acompanhado do diagnóstico que `GAR-07` do FND-04 exige — informação
suficiente para um replay conforme —, com o erro sanitizado conforme a regra adjacente
daquela subseção.

`encaminhado` — Limiar de tentativas, prazos de backoff, retenção da DLQ e o runbook
de operação são de FND-08 ([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445)),
sob ANC-06, como o FND-04 §7.4 já havia encaminhado.

### §11.4 Operação de registry de schemas

`recepcionado` — `BUF-09` do FND-05 fixa a autoridade de validação de contrato **no
repositório**, local e fail-closed. `ENV-02` e `ENV-23` vedam que a desserialização
dependa de resolução remota. A **escolha** de registry em runtime — se existe, e qual
é — é `ADR-DMPF-N`, acionado pelo FND-05 e pendente no FND-11. A **operação** foi
encaminhada a este artefato.

`normativo` `KFK-13` — **As regras desta subseção são condicionais: valem se, e
somente se, `ADR-DMPF-N` decidir pela existência de um registry em runtime.** Este
artefato não escolhe existência, produto nem autoridade — fazê-lo anteciparia decisão
de outra dona.

`normativo` `KFK-14` — **Se existir registry, ele é catálogo e contenção operacional,
nunca pré-requisito de desserialização.** Um consumidor que não consiga desserializar
sem consultar o registry viola `ENV-02` e `ENV-23`, e o canal está mal configurado.

`normativo` `KFK-15` — **O cliente de produção roda com auto-registro de schema
desabilitado.** É configuração do `provider`, e é a única parte deste assunto que esta
âncora alcança: um adapter que registre schema ao publicar transforma um erro de
contrato em fato consumado no registry. Em ambiente de desenvolvimento o auto-registro
é admitido.

`encaminhado` — **O pré-registro do schema no pipeline de entrega** — quem o executa,
em que etapa e com qual autorização — é governança de contrato, de ANC-03, e operação
de entrega. `KFK-15` obriga o adapter a não registrar; não obriga o pipeline a
registrar, porque o pipeline não é `provider`.

`normativo` `KFK-16` — **O identificador do schema no registry é derivado do canal, e
não do tipo do registro.** Enunciada como invariante porque o nome do mecanismo varia
por produto: onde existir a noção de estratégia de nomeação de subject, adota-se a
baseada no tópico; onde o produto expuser outro mecanismo, adota-se o que preserve a
propriedade — um conjunto de tipos controlado por canal, como exige `KFK-03`. O
mapeamento para o produto escolhido é do `ADR-DMPF-N`.

`normativo` `KFK-17` — **O modo de compatibilidade do registry não é mais permissivo
que o gate de contrato do repositório.** O FND-05 governa a evolução do `.proto` com
verificação de breaking em granularidade de arquivo; um registry configurado para não
checar compatibilidade criaria uma segunda autoridade, mais frouxa, sobre a mesma
decisão. Desligar a checagem é admitido apenas em janela de bootstrap ou migração
declarada, com prazo, nunca como configuração permanente.

`normativo` `KFK-18` — **Validação no broker, onde o produto a oferecer, não
substitui validação no cliente.** O que essa camada alcança é o identificador de
schema, o formato de wire e a estratégia de nomeação; ela **não** valida o payload
contra o schema. Tratá-la como verificador semântico é o erro que `TRP-34` já veda no
envelope, repetido uma camada abaixo. A existência do recurso depende do produto, e a
escolha é do `ADR-DMPF-N`.

---

## §12. SNS e SQS

`recepcionado` — A base conceitual fixa, para SNS e SQS: corpo textual com o
CloudEvent Protobuf em Base64; entrega em modo bruto preferível de SNS para SQS;
`MessageDeduplicationId` igual ao identificador da mensagem em FIFO;
`MessageGroupId` igual à chave de partição em FIFO; visibilidade estendida para
trabalhos longos; delete somente após o commit; claim-check para payload acima do
limite do transporte.

### §12.1 Envelope no corpo textual

`normativo` `SQS-01` — **O corpo da mensagem é o envelope serializado, codificado uma
única vez para a forma textual do transporte** (`TRP-19`), e o `payload_hash` é
calculado sobre o `Any.value` recuperado, nunca sobre a forma codificada
(`TRP-15`).

`normativo` `SQS-02` — **A entrega de SNS para SQS usa modo bruto.** Sem ele, o corpo
publicado passa a ser um campo dentro da estrutura de notificação do SNS, e o
consumidor que leia o corpo como envelope lê a envoltória — a linha `não conforme` da
matriz de §5.2.

`normativo` `SQS-03` — **Atributos de mensagem transportam apenas metadado de
operação e roteamento** (`TRP-18`, `TRP-20`). Filtro de assinatura pode ler atributo;
nenhum atributo exigido pelo envelope migra para lá.

`normativo` `SQS-03b` — **O número de atributos é limitado e validado antes da
publicação.** Na entrega em modo bruto de SNS para SQS o teto é de dez atributos, e
excedê-lo faz a plataforma **descartar** a mensagem como erro de cliente — perda
silenciosa, não falha visível. O `provider` valida a contagem antes do envio.

`normativo` `SQS-03c` — **Filtro de assinatura é vedado em tópico FIFO para canal do
DMPF.** A plataforma documenta que habilitar filtro numa assinatura de tópico FIFO
altera a entrega para no-máximo-uma-vez, e no-máximo-uma-vez é incompatível com a
semântica oficial ao-menos-uma-vez que P0-3 fixa: uma mensagem perdida não é
recuperável por redelivery nem detectável pela inbox. Fan-out com filtro usa tópico
standard; onde a ordenação por grupo for exigida junto do fan-out, o filtro sai do
tópico e vira decisão do consumidor.

### §12.2 FIFO e standard

`normativo` `SQS-04` — **A escolha entre FIFO e standard é declarada por canal, com o
critério de §8.1.** FIFO onde há ordenação exigida por chave; standard onde não há.
Um canal standard não promete ordem, e o consumidor não pode depender dela.

`normativo` `SQS-05` — **Em FIFO, o grupo de mensagem é **derivado** da chave de
partição do envelope** (`TRP-11`), preservando igualdade: chaves iguais produzem
grupos iguais, chaves distintas produzem grupos distintos. A ordenação garantida é
dentro do grupo — o análogo exato da partição de `KFK-06`.

`normativo` `SQS-06` — **Em FIFO, o identificador de deduplicação é derivado de
`source`, do identificador da mensagem e do `payload_hash` do envelope**, nesta
composição. Usar só o identificador é insuficiente por duas razões: ele é único apenas
no escopo de `source`, então duas fontes distintas colidiriam; e a reutilização do
mesmo identificador com payload divergente — que a inbox precisa ver como R4 — seria
suprimida antes de chegar lá. É otimização de publicação, sob `TRP-43`, e não
substitui a inbox.

`normativo` `SQS-06b` — **A derivação de `SQS-05` e `SQS-06` produz valor dentro dos
limites do transporte.** Os campos de grupo e de deduplicação admitem no máximo 128
caracteres, num alfabeto próprio, e o FND-05 não restringe o tamanho nem o alfabeto de
`id` e da chave de partição. Copiar a string do envelope é, portanto, inseguro: a
derivação é uma função de comprimento fixo — um resumo criptográfico em hexadecimal —
declarada por canal e igual nas duas pontas.

`normativo` `SQS-07` — **Em FIFO, uma poison message bloqueia o grupo até ser
confirmada ou retirada do fluxo.** É o mesmo efeito da partição em Kafka, e a resposta
é a mesma: limite de tentativas obrigatório, por `GAR-08`.

`registro` — **A ser confirmado antes do aceite: SNS FIFO com filtro de assinatura.**
Há indicação de que a combinação de tópico FIFO com filtro de mensagem altera a
semântica de entrega documentada pelo provedor de nuvem, possivelmente para
no-máximo-uma-vez. Se a indicação se confirmar, a combinação é **incompatível** com a
semântica oficial do DMPF, que é ao-menos-uma-vez com efeitos idempotentes (P0-3), e
passa a ser vedada por este artefato. A verificação em documentação primária é
pendência de §18, atribuída à revisão de infraestrutura.

### §12.3 Visibilidade, retry e contenção

`normativo` `SQS-08` — **A visibilidade é estendida enquanto o processamento estiver
em curso, e a extensão é ato explícito do consumidor.** Deixar a visibilidade expirar
com o processamento em andamento produz entrega concorrente da mesma mensagem — a
inbox a classifica e evita o efeito duplicado, mas o trabalho é desperdiçado.

`normativo` `SQS-08b` — **A extensão tem teto, e trabalho que possa excedê-lo é
decomposto.** O acúmulo de extensões é limitado a doze horas contadas do recebimento;
depois disso a mensagem volta à fila independentemente do que o consumidor faça. Um
processamento cuja duração possa aproximar-se desse teto é quebrado em etapas com
ponto de retomada persistido, em vez de sustentado por extensão.

`normativo` `SQS-09` — **O delete ocorre pelo identificador de recebimento, depois do
commit local** (`TRP-26`). Um identificador de recebimento de uma tentativa anterior
não é reutilizado.

`normativo` `SQS-10` — **A falha transitória não deleta e não estende
indefinidamente.** O gesto é o de `R1×D3`: deixar a visibilidade expirar, ou reduzi-la
explicitamente para antecipar a próxima tentativa.

`normativo` `SQS-11` — **A estratégia de contenção é escolhida por disposição, não por
canal.** Redirecionamento gerenciado e publicação explícita resolvem problemas
diferentes, e um canal real precisa dos dois:

| Caso | Gesto |
|------|-------|
| D3 com tentativas esgotadas, e falha sem classificação | redirecionamento gerenciado, pelo limite de recebimentos declarado |
| D4 e R4 | publicação explícita na fila de contenção, seguida do delete (`TRP-30`) |
| Envelope inválido | publicação explícita na quarantine, seguida do delete (`TRP-35`, `TRP-36`) |

`rationale` — A versão anterior desta regra obrigava o canal a escolher **um** dos dois
mecanismos, e era inimplementável. O redirecionamento gerenciado reage apenas à
contagem de recebimentos: não existe comando de aplicação que diga «mova agora, porque
classifiquei D4». Um canal que só use redirecionamento faz a mensagem terminal voltar
até esgotar o limite, contrariando o «sem loop» de `TRP-27`; um canal que só use
publicação explícita perde o mecanismo natural para a D3 esgotada.

`normativo` `SQS-11b` — **Os dois mecanismos não se aplicam à mesma disposição.**
Publicação explícita seguida de delete, sob um limite de recebimentos que também
redireciona, **permite** duplicata na fila de contenção nos intercalamentos de falha —
quando a publicação explícita conclui e o delete falha ou fica incerto, e a mensagem
volta a ser recebida até atingir o limite. Não é efeito inevitável, e é por isso que a
vedação é por disposição.

`normativo` `SQS-12` — **O limiar do claim-check é o tamanho da mensagem **final**, no
menor limite do caminho completo.** O que conta é o envelope já serializado e
codificado para a forma textual, e o caminho inteiro: uma publicação que passe por
tópico antes da fila é limitada pelo menor dos dois tetos, não pelo da fila. O
`provider` avalia o tamanho final antes de decidir.

`normativo` `SQS-12b` — **O uso de claim-check depende da condição de `TRP-21`.**
Enquanto o perfil do envelope não admitir a modalidade de referência, um payload que
exceda o limite do caminho não tem transporte conforme por esta seção, e o canal é
redesenhado — por decomposição da mensagem ou por outro recorte de agregado — em vez
de transportar referência.

### §12.4 Prazo e categorias de erro no assíncrono

`normativo` `SQS-13` — **O consumo declara o seu prazo de processamento, e ele é menor
que a visibilidade aplicada.** A relação é o análogo assíncrono de `GRP-17`: prazo
maior que a visibilidade produz entrega concorrente enquanto o primeiro consumidor
ainda trabalha — que `SQS-08` manda evitar por extensão, e `SQS-08b` limita no teto.
Onde o prazo puder aproximar-se do teto, vale a decomposição de `SQS-08b`.

`normativo` `KFK-19` — **Em Kafka, o prazo de processamento de um registro é menor que
o intervalo máximo entre buscas configurado.** Exceder esse intervalo faz o consumidor
ser considerado morto e a partição, reatribuída — com o trabalho em curso caindo no
caso de `TRP-48`. É o mesmo raciocínio de `SQS-13`, com o relógio que o Kafka usa.

`recepcionado` — O FND-07 §5 e §6 fixam as categorias de erro e, para cada uma, a
retryability; `ERR-09` garante que todo erro concreto resolve para um booleano, e
`MAP-07` que o consumer adapter recebe esse booleano já resolvido e **deriva D3 ou
D4 dele**. Aquele artefato encaminhou a esta âncora os «valores por broker» dessas
categorias.

`normativo` `TRP-53` — **O gesto por categoria de erro é obtido por composição, não por
uma segunda tabela.** A categoria resolve a retryability por `ERR-09`; a retryability
resolve a disposição por `MAP-07`; a disposição resolve o gesto concreto por `TRP-27`,
que já o declara para Kafka e para SQS. Um canal que declare gesto próprio por
categoria, fora dessa composição, cria uma segunda autoridade sobre a mesma decisão.

`rationale` — A alternativa era tabular as nove categorias contra os dois transportes,
com dezoito células. Ela foi descartada porque duplicaria a decisão: as categorias
mapeiam para duas disposições, e as disposições já têm gesto declarado. Dezoito células
que derivam de duas divergem na primeira manutenção.

---

## §13. Os três escopos de deduplicação

O escopo da ARQ-443 pede a «fronteira entre dedupe nativo de broker e inbox do DMPF».
A palavra fronteira sugere duas coisas do mesmo tipo disputando território. Não é o
caso: são três camadas com escopos distintos, e nenhuma substitui outra.

`recepcionado` — **As duas camadas de baixo já estão decididas pelo FND-04 §7.2, e
este artefato não as reabre.** A tabela abaixo reproduz a decisão de lá, e a única
linha cujo sujeito é `provider` — e portanto matéria desta âncora — é a primeira.

| Camada | Chave | Alcance no tempo | Onde vive | Força |
|--------|-------|------------------|-----------|-------|
| **Deduplicação de publicação** | mecanismo do broker | janela e condições do mecanismo | `provider` de publicação | `normativo` aqui (`TRP-42`) |
| **Redelivery do transporte** | — | é o mecanismo que **produz** a duplicata legítima | transporte | descrito em §6 |
| **Deduplicação por identidade de mensagem** | `(consumer_name, message_id)`, por `INB-01` | **limitada pela retenção** da inbox (FND-04 §6.6) | inbox, no `application service` | `recepcionado` de FND-04 |
| **Idempotência do efeito de negócio** | chave natural da operação, no domínio | **permanente** | `domain library` — constraint natural, verificação de estado ou operação convergente | `recepcionado` de FND-04 `GAR-10` |

`recepcionado` — `GAR-03` do FND-04: a deduplicação por inbox **não substitui** a
idempotência do efeito de negócio. E `GAR-04`: é a idempotência de efeito que
sustenta a invariante de não duplicação, **não** a inbox — que protege da reaplicação
da mesma mensagem, e apenas enquanto a retiver.

`normativo` `TRP-42` — **Nenhum mecanismo de transporte substitui qualquer das duas
camadas de consumo.** É a única regra que esta seção precisa acrescentar, e o seu
sujeito é o `provider`: um canal cuja conformidade dependa de deduplicação nativa do
broker para não duplicar efeito está mal desenhado, porque o alcance do mecanismo
nativo é a janela de publicação.

`normativo` `TRP-43` — **A deduplicação nativa do broker é descrita pelo efeito, nunca
pela promessa, e é configurada quando disponível.** A deduplicação de envio em fila
FIFO e a idempotência de produtor em Kafka reduzem duplicatas de publicação sob
condições específicas; ambas são desejáveis, e nenhuma autoriza dispensar a inbox nem
a chave natural do domínio.

`normativo` `TRP-44` — **O `provider` não é o lugar da idempotência de efeito.** Um
adapter que tente garanti-la — deduplicando por conta própria, guardando estado de
processamento, consultando o destino antes de aplicar — assume responsabilidade de
`application service` e de `domain library`, e o faz sem a transação que a torna
correta.

`rationale` — Esta seção foi reescrita depois da revisão: a versão anterior afirmava
que a idempotência de efeito era exclusiva da inbox e que não tinha janela. As duas
afirmações contrariam o FND-04 §7.2, que tabula a inbox como limitada pela retenção e
a idempotência de efeito como permanente e residente no domínio. É a confusão que o
próprio FND-04 nomeia como «o erro mais frequente do padrão», e ela sobreviveu à
primeira escrita deste artefato apesar de a fonte estar citada ao lado.

`rationale` — A regra existe por causa do modo de falha mais provável de P0-3, já
nomeado em §2.2: alguém habilita a deduplicação nativa, observa que as duplicatas
desapareceram do teste, e conclui que a inbox é redundante. As duas coisas são
verdadeiras e não se somam — uma atua na publicação, com janela; a outra, no consumo,
sem janela. Descrever o mecanismo pelo efeito, e não pelo nome comercial, é o que
impede a conclusão errada.

---

## §14. Kafka e SNS/SQS, lado a lado

`registro` — A comparação é nas mesmas dimensões normativas, para que a escolha de
§8.2 seja verificável. Nenhuma célula promete exactly-once fim a fim.

| Dimensão | Kafka | SNS / SQS |
|----------|-------|-----------|
| Envelope | `record.value`, byte a byte (`KFK-05`) | corpo textual, codificado uma vez (`SQS-01`) |
| Chave de ordenação | chave do registro, do envelope (`KFK-05`) | grupo de mensagem, do envelope, só em FIFO (`SQS-05`) |
| Unidade de ordenação | partição (`KFK-06`) | grupo de mensagem (`SQS-05`); nenhuma em standard (`SQS-04`) |
| Consumidor lógico | grupo de consumo, distinto do `consumer_name` (`KFK-07`, `KFK-08`) | fila; o `consumer_name` continua lógico (`KFK-08`) |
| Confirmação | commit de offset contíguo, após o commit local (`TRP-28`, `TRP-29`) | delete por identificador de recebimento, após o commit local (`SQS-09`) |
| Não confirmação | ausência de commit; reentrega na próxima atribuição (`TRP-27`) | visibilidade expira ou é reduzida (`SQS-10`) |
| Prorrogação em curso | não há mecanismo equivalente; o limite é a retenção e o tempo de sessão | extensão explícita de visibilidade (`SQS-08`) |
| Horizonte de redelivery | retenção do log e ponto de reinício do grupo (`TRP-24`) | recebimentos até o redirecionamento, visibilidade e retenção da fila (`TRP-24`) |
| Retry | inline com limite; tópico de retry só sem ordenação (`KFK-10`, `KFK-11`) | visibilidade com backoff; limite de recebimentos (`SQS-10`, `SQS-11`) |
| Contenção | DLQ por canal, publicada antes do avanço do offset (`KFK-12`) | redirecionamento gerenciado ou publicação explícita, nunca ambos (`SQS-11`) |
| Poison message | bloqueia a partição até o limite (`KFK-10`, `GAR-08`) | bloqueia o grupo FIFO até o limite (`SQS-07`, `GAR-08`) |
| Deduplicação de publicação | idempotência de produtor, quando disponível (`TRP-44`) | identificador de deduplicação em FIFO (`SQS-06`) |
| Fan-out com filtro | consumidores independentes por grupo de consumo, sem filtro no broker | filtro de assinatura por atributo (`SQS-03`) |
| Payload grande | claim-check (`TRP-21`) | claim-check (`SQS-12`) |
| Governança de schema em runtime | registry condicional a `ADR-DMPF-N` (`KFK-13` a `KFK-18`) | não há mecanismo nativo; o gate é o do repositório (`BUF-09`) |
| Base as-is na organização | nenhuma (§8.2) | cinco serviços em runtime (§8.3) |

---

## §15. Coexistência dos dois transportes

Enquanto Kafka for alvo e SNS/SQS for acervo, os dois transportes convivem. Esta
seção fixa o que precisa ser verdade durante a convivência. Ela **não** define
cronograma, faseamento nem ordem de migração — isso é adoção organizacional,
`encaminhada` a ANC-09 / FND-10 (§1.4).

`normativo` `COE-01` — **Um canal lógico tem um transporte por vez.** A convivência é
entre canais diferentes, não dentro do mesmo canal: publicar o mesmo canal nos dois
transportes simultaneamente cria duas ordens independentes e duas linhas de
redelivery para o mesmo fluxo.

`normativo` `COE-02` — **O binding de um canal é estável por mensagem e por
tentativa** (`TRP-09`). Durante a convivência a regra deixa de ser precaução e passa a
ser a única proteção contra o modo de falha de `TRP-09`: com dois transportes
configurados, uma tentativa incerta repetida após troca de binding publica o mesmo
item de outbox nos dois.

`normativo` `COE-03` — **A troca de transporte de um canal é migração, com drenagem
ou transferência declarada do backlog** (`TRP-10`). Não existe troca por recarga de
configuração.

`normativo` `COE-04` — **O `consumer_name` da inbox é lógico e estável entre
transportes** (`KFK-08`). Dois adapters que produzam o mesmo efeito de negócio usam o
mesmo `consumer_name`, porque a chave de deduplicação da inbox é
`(consumer_name, message_id)`, por `INB-01`: nomes diferentes criam duas entradas para
a mesma mensagem, e o efeito ocorre duas vezes com a inbox funcionando exatamente
como especificada.

`normativo` `COE-05` — **Uma ponte entre transportes preserva a identidade da
mensagem.** O identificador da mensagem, a origem, o tipo e os bytes do payload
atravessam a ponte inalterados. Gerar identificador novo na ponte torna a mensagem
irreconhecível para a inbox do consumidor, que a classificará como primeira recepção
e aplicará o efeito de novo.

`normativo` `COE-06` — **Não há ordenação entre os dois transportes, e a promessa não
é feita.** Chave de partição igual não cria ordem entre um log de Kafka e uma fila
FIFO: são infraestruturas independentes, sem relógio comum e sem sequência comum. Um
canal migrado observa ordem dentro do transporte de origem até a drenagem, e dentro do
transporte de destino depois dela — nunca entre os dois.

`normativo` `COE-07` — **A migração de um canal com ordenação exigida requer corte
com drenagem.** A sequência é: parar de publicar no transporte de origem, drenar o
backlog até o fim, e só então publicar no destino. Sem isso, mensagens da mesma chave
existem simultaneamente nos dois transportes, e `COE-06` diz que a ordem entre elas
não existe.

`normativo` `COE-08` — **Canal sem ordenação exigida admite drenagem paralela, que é
uma forma declarada de transferência de backlog, não a sua ausência.** A produção
passa para o transporte de destino e o consumo do backlog de origem continua até
esvaziar; durante a janela, o canal tem produção em um transporte e consumo nos dois.
É a exceção que `TRP-10` e `COE-01` admitem, e ela exige, cumulativamente:

| Condição | Regra que a sustenta |
|----------|---------------------|
| Ordenação não declarada no canal | `SQS-04`, `KFK-06` |
| Binding imutável por mensagem, com o endereço persistido antes do primeiro I/O | `TRP-09`, `TRP-46` |
| `consumer_name` lógico idêntico nos dois adapters | `COE-04` |
| Identidade e bytes preservados em qualquer ponte | `COE-05`, `TRP-13` |
| Estado do backlog de origem observável, e fim de janela declarado | `GAR-12` |

`rationale` — A versão anterior desta regra dizia que o canal «pode migrar sem
drenagem», e contradizia `TRP-10`, `COE-01` e `COE-03` de frente: com backlog antigo
ainda consumível e mensagens novas no destino, o canal opera nos dois transportes
simultaneamente. O que muda não é a permissão, é o nome e as condições — drenagem
paralela é transferência de backlog com estado observável, e não a dispensa dela.

`registro` — **Vetores negativos desta seção**, para o catálogo de FND-09:

| Vetor | Situação | Resultado esperado |
|-------|----------|--------------------|
| V-COE-1 | Publicação confirmada, relay falha antes de marcar, binding trocado, retry publica no outro transporte | Vedado por `COE-02` e detectável por `TRP-46`: o item deve trazer o endereço da primeira tentativa. Com `COE-04` observado a inbox absorve a segunda entrega como R2; o que se perde é a ordenação (`COE-06`) e o trabalho de consumo |
| V-COE-2 | Dois adapters, um por transporte, com `consumer_name` diferente, para o mesmo efeito | Vedado por `COE-04`; se ocorrer, duas entradas na inbox e efeito duplicado |
| V-COE-3 | Ponte de SQS para Kafka que reserializa **o payload dentro de `Any.value`** | Vedado por `COE-05` e `TRP-14`; o `payload_hash` divergirá e a redelivery legítima virá como R4. Reserializar o envelope **preservando** `Any.value` byte a byte não produz R4 — o hash de `ENV-17` não alcança os bytes do envelope externo —, mas viola `TRP-18` se mover atributo para header |
| V-COE-4 | Canal com ordem migrado sem drenagem | Vedado por `COE-07`; a ordem entre transportes não existe, por `COE-06` |
| V-COE-5 | Mesmo canal publicado nos dois transportes «para comparar» | Vedado por `COE-01` |

---

## §16. Catalogação de canal assíncrono

O que esta seção normatiza é a **catalogação por transporte**: qual canal existe, o
que ele transporta e sob quais parâmetros de binding. O que ela não normatiza é a
fonte de verdade documental — onde o arquivo do catálogo vive, quem o mantém, como
versiona. Essa parte é contrato, matéria de ANC-03, e permanece a sua pendência 2
(§1.4).

`normativo` `ASY-01` — **O `provider` não opera canal assíncrono não catalogado.** A
pós-condição é dele, e é verificável na construção: o canal que ele atende tem
catalogação resolvível, ou não é operado. AsyncAPI é o formato adotado para essa
catalogação.

`encaminhado` — **A forma do documento AsyncAPI — o seu schema, a sua versão, onde vive
e quem o mantém — é matéria de `contract package`, sob ANC-03.** Este artefato declara
o **conteúdo operacional** que o `provider` precisa encontrar na catalogação para
operar o canal; não normatiza a estrutura do artefato que o carrega.

`normativo` `ASY-02` — **O que o `provider` precisa encontrar na catalogação de um
canal**, sem o que não o opera:

| Item | Por que é obrigatório |
|------|----------------------|
| Endereço concreto e transporte | É o binding de `TRP-08`, e sem ele o canal não é localizável |
| Tipo de evento e major do contrato | Liga o canal ao contrato governado pelo FND-05 |
| Chave de ordenação, ou a declaração de que não há ordem | `KFK-06` e `SQS-04` exigem que a promessa seja explícita |
| Unidade de ordenação | Partição ou grupo — o consumidor precisa saber o escopo da garantia |
| `janela_redelivery` e a sua fórmula | `TRP-23`: sem isso, `INB-14` não é verificável |
| Destino de contenção | DLQ, quarantine, ou a fusão declarada por `TRP-36` |
| Estratégia de retry e o seu efeito sobre a ordem | `TRP-31`: nenhuma das duas estratégias é neutra |

`normativo` `ASY-03` — **O documento de canal referencia o contrato, não o duplica.**
O payload é apontado por referência ao contrato Protobuf versionado; reproduzir o
schema no documento de canal cria uma segunda definição que divergirá da primeira.

`normativo` `ASY-04` — **A catalogação não introduz atributo de envelope.** Ela
descreve o binding e os parâmetros de operação; o envelope é o do FND-05, e `ENV-11`
fecha o conjunto de extensões.

`registro` — **Exemplo de documento de canal.** Ilustrativo dos itens de `ASY-02`, não
normativo na sua forma:

```yaml
asyncapi: 3.0.0
info:
  title: Canal de proposta aprovada
  version: 1.0.0
channels:
  propostaAprovada:
    address: credito.proposta.aprovada.v1
    messages:
      propostaAprovada:
        $ref: '#/components/messages/propostaAprovada'
    bindings:
      kafka:
        topic: credito.proposta.aprovada.v1
        partitions: 12
operations:
  receberPropostaAprovada:
    action: receive
    channel:
      $ref: '#/channels/propostaAprovada'
    bindings:
      kafka:
        groupId: credito-analise-consumer
components:
  messages:
    propostaAprovada:
      contentType: application/protobuf
      payload:
        schemaFormat: application/vnd.google.protobuf
        schema:
          $ref: 'credito/proposta/v1/eventos.proto#PropostaAprovada'
      x-dmpf:
        chaveOrdenacao: partitionkey
        unidadeOrdenacao: particao
        janelaRedelivery: retencaoLog + reinicioGrupo
        contencao: credito.proposta.aprovada.v1.dlq
        retry: inline-com-limite
```

`encaminhado` — **Endereço de distribuição externa dos contratos.** O FND-05 §7
encaminhou a este artefato «o endereço concreto onde cada contrato é publicado para
consumo — registro de módulos, pacotes de linguagem, distribuição por transporte»,
partilhado com os épicos de contratos. Este artefato cumpre a parte que é transporte:
o canal declara o contrato que transporta, por referência versionada (`ASY-03`). A
publicação dos artefatos gerados em registros de pacote é dos épicos de contratos, e a
fonte de verdade documental é a pendência da ANC-03.

---

## §17. Acionamento de ADRs estruturais

### §17.1 O que este artefato aciona

`recepcionado` — RFC §13.1 define o gesto, e este artefato o repete sem alteração:
acionar é **nomear** o ADR, **definir o seu assunto**, **registrar a origem** e
**encaminhar** ao FND-11 ([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)), a
quem cabem a redação, a promoção para a faixa reservada em `docs/adr/` e o aceite.

`normativo` — Nenhum ADR é redigido nem aceito aqui. Os dois registros de §17.2
continuam a série que o FND-05 deixou em `ADR-DMPF-N`, e ficam no estado `acionado`.

`normativo` — Os identificadores `ADR-DMPF-O` e `ADR-DMPF-P` são **provisórios**, pela
regra de RFC §13.2: a numeração definitiva é atribuída pelo FND-11 na promoção, e
citar um deles por número definitivo antes disso é erro de rastreabilidade.

`registro` — A ARQ-443 pede, na sua descrição, os identificadores `ADR-010` e
`ADR-011`. Eles não são utilizáveis aqui pelo mesmo motivo que o FND-04 registrou para
a sua faixa e o FND-05 para a dele: a atribuição definitiva é do FND-11.
`ADR-DMPF-O` corresponde ao que a story chama de ADR-010, e `ADR-DMPF-P` ao que ela
chama de ADR-011.

### §17.2 Registro de acionamento

| ID provisório | Nome | Assunto | Origem | Destino | Owner | Estado |
|---------------|------|---------|--------|---------|-------|--------|
| `ADR-DMPF-O` | Divisão dos transportes síncronos e o governo do tempo | Adotar REST/JSON como interface para consumidor externo e gRPC como padrão síncrono interno, com deadline obrigatório e propagado sem reinício, cancelamento propagado a jusante, e retry condicionado à idempotência do método | §8.2, §9, §10; exigido pelo registro de ANC-04 («ADR por transporte adotado») | ARQ-448 | FND-11 | `acionado` |
| `ADR-DMPF-P` | Kafka como transporte-alvo do assíncrono, com SNS/SQS no acervo | Adotar Kafka como transporte-alvo do evento de domínio, manter SNS/SQS normatizado para o acervo e para fan-out com filtro e fila sem ordem, e fixar em cada um a ordenação, o gesto de ACK, o retry e a contenção — sob a vedação de exactly-once fim a fim | §8.2, §11, §12, §13, §14, §15; exigido pelo mesmo registro de ANC-04 | ARQ-448 | FND-11 | `acionado` |

`registro` — Alternativas descartadas e insumos, obrigatórios na redação (RFC §13.2):

| ID | Alternativas descartadas | Onde estão registradas |
|----|--------------------------|------------------------|
| `ADR-DMPF-O` | REST em todas as chamadas internas; gRPC exposto diretamente a consumidor externo; transcodificação como padrão em vez de exceção; timeout apenas no cliente em vez de deadline propagado; retry livre por indisponibilidade, sem exigir idempotência do método | §8.1 `rationale`; §9 `rationale`; `GRP-04` a `GRP-09` |
| `ADR-DMPF-P` | Manter SNS/SQS como transporte-alvo, pelo peso da base as-is; adotar Kafka declarando SNS/SQS depreciado de imediato; dar estatuto igual aos dois, sem alvo declarado; publicar o mesmo canal nos dois transportes durante a transição; tratar deduplicação nativa de broker como substituto da inbox | §8.2 `registro`; `TRP-40`; `COE-01`; §13 `rationale` |

`registro` — **Insumos externos disponíveis para a redação.** Os dois estudos internos
anexados ao épico ARQ-436: o de padronização de comunicação síncrona com gRPC,
Protocol Buffers e Buf sustenta as decisões de perfil, versionamento, retry,
balanceamento, health checking e mapeamento de erro que §10 normatiza — e a redação de
`ADR-DMPF-O` depende materialmente dele, inclusive das três divergências registradas
em §10.5. O de Schema Registry e validação de esquemas sustenta `KFK-13` a `KFK-18`, e
a redação de `ADR-DMPF-P` depende dele na parte de governança de schema em runtime.

### §17.3 Por que dois, e como isso satisfaz «ADR por transporte adotado»

`rationale` — O registro de ANC-04 exige ADR «por transporte adotado». A leitura de que
isso significa um ADR por transporte produziria quatro ou cinco documentos, e três
deles registrariam a mesma decisão vista de ângulos diferentes. A leitura adotada é a
literal: **cada transporte adotado precisa estar coberto por um ADR** — e a cobertura
é demonstrada abaixo, transporte por transporte.

| Transporte adotado | Decisão estrutural que a sua adoção implica | ADR que a registra |
|--------------------|---------------------------------------------|--------------------|
| REST/JSON | Ser a interface do consumidor externo, e não o transporte interno geral | `ADR-DMPF-O` |
| gRPC | Ser o síncrono interno, com o tempo governado pela borda por deadline propagado | `ADR-DMPF-O` |
| Kafka | Ser o alvo do assíncrono de domínio, com ordenação por partição e confirmação por offset | `ADR-DMPF-P` |
| SNS/SQS | Permanecer normatizado no acervo e ser escolha superior em fan-out com filtro e fila sem ordem | `ADR-DMPF-P` |

`rationale` — O agrupamento segue o critério que o FND-05 §9.3 usou: um ADR estrutural
registra a decisão que altera **qual bloco conhece qual**, ou que a âncora exige
nominalmente. REST e gRPC compartilham uma única decisão estrutural — quem fala com
quem, e quem governa o tempo da chamada —, e separá-los produziria dois ADRs que se
citam mutuamente para dizer a mesma coisa. Kafka e SNS/SQS compartilham a decisão de
qual é o alvo e o que acontece com o acervo, que não é decidível para um sem o outro.

`normativo` — **O que fica fora do escopo destes dois ADRs.** A escolha de registry de
schemas em runtime é `ADR-DMPF-N`, acionado pelo FND-05 e pendente; `ADR-DMPF-P` cita
essa dependência e **não** a decide. O cronograma e o plano de migração da adoção de
Kafka são de ANC-09 / FND-10 e não pertencem a nenhum dos dois.

---

## §18. Rastreabilidade

### §18.1 Critérios de aceite: estado final

`registro` — Fechamento do rastreio aberto em §3.1. Um critério cujo estado não é
«satisfeito» traz a razão e a dona.

| # | Critério | Estado | Onde, ou de quem depende |
|---|----------|--------|--------------------------|
| CA-1 | REST/JSON/OpenAPI como contrato externo, com convenções publicadas | **parcial** | Satisfeito no que é transporte: `RST-01` a `RST-04` e a linha da matriz em §8.2. **Não** satisfeito quanto à superfície da API e à fonte de verdade documental — fora do escopo por `TRP-03`, e pendência 2 da ANC-03 |
| CA-2 | gRPC/Protobuf como síncrono interno, com deadline e cancelamento obrigatórios | satisfeito | `GRP-01` a `GRP-18`. Existência e propagação em `GRP-04` a `GRP-07`; o **valor** do prazo, que o FND-07 encaminhou, em `GRP-16` a `GRP-18` |
| CA-3 | Kafka, SNS e SQS com convenções de envelope, chave de partição, ordenação, ACK e retry | satisfeito | Envelope: `KFK-05`, `SQS-01`. Chave: `TRP-11`, `KFK-05`, `SQS-05`. Ordenação: `KFK-06`, `SQS-04`, `SQS-05`. ACK: `TRP-26` a `TRP-32`, `TRP-47`, `TRP-48`. Retry: `KFK-10`, `KFK-11`, `SQS-10`, `SQS-11` |
| CA-4 | AsyncAPI como documentação dos canais e bindings assíncronos | satisfeito na catalogação | `ASY-01` a `ASY-04`. A forma e a manutenção do documento são de ANC-03, declaradas em §16 |
| CA-5 | Nenhuma promessa de exactly-once fim a fim | satisfeito | `TRP-05`, `TRP-06`, `TRP-42` a `TRP-44`; §14 sem nenhuma célula que a prometa. A ressalva que a primeira versão deixou aberta está **resolvida**: `SQS-03c` veda filtro de assinatura em tópico FIFO, porque a plataforma documenta a degradação para no-máximo-uma-vez |
| CA-6 | Matriz de decisão validada contra ≥3 casos reais do inventário | **parcial** | §8.3 classifica cinco casos quanto à **escolha de transporte**. Não atesta conformidade das implementações — o inventário registra lacuna em dois deles — e não valida a política Kafka, que não tem caso observado (`TRP-41`) |
| CA-7 | Fronteira explícita entre dedupe nativo de broker e inbox | satisfeito, com reformulação | §13 substitui a noção de fronteira por camadas de escopo distinto, e recepciona de FND-04 as duas que não são desta âncora |
| CA-8 | ADR-010 e ADR-011 registrados com contexto, alternativas, trade-offs e consequências | satisfeito como **acionamento** | §17.2 nomeia, define assunto, registra origem, alternativas descartadas e insumos. A redação e o aceite são do FND-11 — o entregável 4 e o item 2 do DoD **não** são alcançáveis aqui |

### §18.2 Obrigações herdadas: estado final

`registro` — Fechamento da matriz de §2.3, mais as obrigações que o FND-07 encaminhou
a esta âncora depois da primeira redação deste artefato.

| Estado | Obrigações de §2.3 | Observação |
|--------|--------------------|------------|
| `quitada` | 1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 17, 18 | Treze obrigações distintas, cada uma com regra normativa própria. A 4 (byte-preservação) é quitada **para os caminhos admitidos**: a modalidade de referência do claim-check fica condicionada a `TRP-21`. A 6 (`janela_redelivery`) exige fórmula fechada com variáveis nomeadas, por `TRP-24` e `TRP-24b` |
| `condicionada` | 10, 13, 14 | 10: retenção da DLQ e rito de replay dependem de FND-08. 13: a operação do registry vale se `ADR-DMPF-N` adotar registry em runtime. 14: a distribuição externa dos contratos é partilhada com os épicos de contratos |
| `encaminhada` | 15 | A forma executável do perfil de validação fica com FND-09; §7 fixa onde a validação ocorre, o gesto e o destino |
| consolidada | 16 | Não é obrigação distinta: é a obrigação 4 registrada em segundo ponto do FND-05 |

| # | Obrigação encaminhada pelo FND-07 | Origem | Onde | Estado |
|---|----------------------------------|--------|------|--------|
| 19 | **Valor** do prazo por transporte — o FND-07 fixa campo, monotonicidade, sinal e categoria do estouro, e declara que o valor não é matéria de lá | §3, após `CTX-23` | `GRP-16` a `GRP-18`, `SQS-13`, `KFK-19` | `quitada` |
| 20 | Cabeçalho de tentativa | após `MAP-07` | `TRP-52` | `quitada` |
| 21 | Validação de tamanho e profundidade antes do decode, no que é transporte | §7, matriz §8.8 | `TRP-49`, `TRP-51` | `quitada` |
| 22 | Proteção contra payload excessivo e expansão abusiva | matriz §8.8 | `TRP-49`, `TRP-50` | `quitada` |
| 23 | Valores por broker das categorias de erro | sucessão de Parte-1 §13 | `TRP-53` | `quitada` por composição — a categoria resolve a retryability, que resolve a disposição, que já tem gesto em `TRP-27` |
| 24 | Assinatura e verificação em fronteira não confiável | matriz §8.8 | — | `encaminhada` — dividida entre ANC-03, ANC-04 e plataforma; o mecanismo não é de transporte, e este artefato não tem regra que o alcance sem exceder `TRP-03` |

`registro` — **Duas colunas que o FND-07 fechou, e que este artefato deixava abertas.**
`CTX-21` e `CTX-22` obrigam o `provider` a respeitar cancelamento e prazo e fixam o
efeito do cancelamento — o que `GRP-07` encaminhava a uma âncora que ainda não tinha
artefato. E `ERR-09` com `MAP-07` fixam que o consumer adapter recebe a retryability
resolvida em booleano e deriva D3 ou D4 dela, fechando a lacuna que `INB-09` do FND-04
declarava aberta.

`registro` — **A pendência 10 do FND-05 está quitada.** §5 a fecha com `TRP-13` a
`TRP-21` e a matriz de hops de §5.2, que nomeia os pontos onde os bytes se perdem e
classifica cada caminho. A condição da hipótese H1 deixa de ser suposição e passa a ser
propriedade verificável por vetor.

### §18.3 Pendências abertas

`registro` — Nove elos ficam abertos ao fim desta entrega. Nenhum é contornado no
texto; todos são escalados.

| # | Pendência | Estado | Destino |
|---|-----------|--------|---------|
| 1 | **O escopo literal da ANC-04 não cobre as obrigações que este artefato recebeu.** `TRP-02` administra a tensão com um conjunto fechado de quatro obrigações de sujeito `app`; a correção definitiva é reescrever o escopo permitido da âncora | pendente | Rito de versão da RFC (§14.2), com quem fechar a ANC-04. É a mais consequente das nove: afeta a validade formal de `TRP-27` na coluna de gesto e de `TRP-33` a `TRP-36` |
| 2 | **A tabela de RFC §14.4 não reflete a sucessão que §1.5 propõe.** Enquanto não refletir, os capítulos da Parte-1 seguem vigentes por aquela tabela, e a proposta desta entrega é só proposta | pendente | Rito de versão da RFC. O FND-04 e o FND-05 registraram a mesma pendência; ela acumula |
| 3 | **A política de Kafka não passou por revisão de infraestrutura de mensageria**, e não tem base as-is que a lastreie | pendente | Revisão nomeada no cabeçalho; até ela, `TRP-41` limita a política a norma de desenho |
| 4 | **Modalidade de referência no envelope**, sem a qual o claim-check de `TRP-21` não é caminho conforme e payload acima do limite do transporte não tem solução declarada | pendente | ANC-03 / FND-05, por nova major do perfil (`ENV-20`) |
| 5 | Fonte de verdade documental de OpenAPI e AsyncAPI — onde vive, quem mantém, como versiona | pendente | Pendência 2 da ANC-03, herdada do FND-05 §10.4, ainda sem dona |
| 6 | Superfície de API REST: desenho de recurso, paginação, forma do corpo de erro, versionamento | pendente | Sem dona; a base conceitual §7.6 segue vigente |
| 7 | Numeração definitiva de `ADR-DMPF-O` e `ADR-DMPF-P`, e a redação de ambos | pendente | FND-11 ([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)) |
| 8 | Escolha de registry de schemas em runtime, de que `KFK-13` a `KFK-18` dependem | pendente | `ADR-DMPF-N`, acionado pelo FND-05, no FND-11 |
| 9 | Mecanismo de assinatura e verificação em fronteira não confiável — obrigação 24 | pendente | ANC-03 no contrato, plataforma no mecanismo; o critério de fronteira confiável já é `IDN-03` do FND-07 |

`normativo` — As pendências 1 e 2 são de natureza distinta das outras sete: elas não
descrevem trabalho que falta, e sim **autoridade que falta**. Enquanto durarem, as
regras de sujeito `app` deste artefato valem por autorização derivada do encaminhamento
nominal dos precedentes, e a sucessão de §1.5 vale como proposta. Quem fechar a ANC-04
precisa decidir entre reescrever o escopo permitido e realocar aquelas quatro
obrigações.

### §18.4 Encaminhamento ao catálogo de vetores

`registro` — O Definition of Done da story pede que os cenários por transporte sejam
encaminhados ao catálogo de testes. Encaminhados a FND-09
([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)), sob ANC-07:

| Grupo | Origem | O que verifica |
|-------|--------|----------------|
| Byte-preservação por hop | §5.2 | Cada linha da matriz, com as `não conforme` como vetores negativos: SNS sem modo bruto, transform que reserializa, duplo Base64, transcodificação JSON |
| Gesto de ACK por disposição | §6.2, `TRP-27` | As sete disposições do FND-04 nos dois transportes, com falha injetada entre as duas ações de cada ramo terminal |
| Commit de offset | `TRP-29` | Disposto o registro `n`, o valor commitado é `n + 1`; commitar `n` reentrega `n` indefinidamente |
| Retry inline em Kafka | `TRP-47` | Ausência de commit **não** reentrega enquanto a atribuição durar; o retry é laço ou reposicionamento |
| Perda de partição | `TRP-48` | Trabalho pendente é cancelado no aviso de revogação, e nada é commitado depois dele |
| Os três relógios | §6.1 | `INB-14` verificada sobre a `janela_redelivery` declarada, com as variáveis de `TRP-24b` instanciadas |
| Deadline propagado | §10.2 | Cadeia de três saltos: o prazo restante decresce, não reinicia, e a cadeia é cancelada em vez de exceder o limite da borda |
| Limites de entrada | §7.1 | Mensagem acima do teto é contida sem decode; conteúdo comprimido não expande além do teto declarado |
| Ordenação por chave | §11.2, §12.2 | Duas mensagens da mesma chave entregues na ordem de publicação, dentro da partição e dentro do grupo |
| Derivação de campos FIFO | `SQS-05`, `SQS-06`, `SQS-06b` | Envelope com identificador longo produz grupo e identificador de deduplicação dentro dos limites do transporte, preservando igualdade |
| Vedação de exactly-once | §13 | Proposta de canal que prometa entrega exactly-once fim a fim é rejeitada, e a política aplicável é ao-menos-uma-vez com consumo idempotente |
| Coexistência | §15 | Os cinco vetores `V-COE-1` a `V-COE-5` |
| Contenção de envelope inválido | §7 | Envelope inválido vai para quarantine sem passar por disposição e sem loop |

### §18.5 Índice de regras

`registro` — 131 regras, em sete prefixos. Nenhum prefixo colide com os já usados
pelos artefatos precedentes, incluindo os do FND-07 (`CTX`, `ERR`, `MAP`, `IDN`, `DAT`,
`THR`).

| Prefixo | Domínio | Quantidade |
|---------|---------|-----------|
| `TRP-` | Invariantes transversais de transporte | 54 |
| `RST-` | REST e JSON como interface externa | 4 |
| `GRP-` | gRPC como síncrono interno | 19 |
| `KFK-` | Kafka | 23 |
| `SQS-` | SNS e SQS | 19 |
| `ASY-` | Catalogação de canal assíncrono | 4 |
| `COE-` | Coexistência dos dois transportes | 8 |

`registro` — **Convenção de numeração.** Um identificador com sufixo de letra
(`KFK-01b`, `SQS-03c`) é regra acrescentada à mesma matéria da regra base, e não uma
versão dela: as duas obrigam. O identificador `TRP-45` foi **retirado** na revisão que
precedeu a promoção — a regra que o ocupava afirmava que a idempotência de negócio era
camada subordinada à inbox, o que contraria o FND-04 §7.2 — e o número não é
reutilizado.

| Seção | Título | Regras |
|-------|--------|--------|
| §1.3 | O sujeito das regras deste artefato | `TRP-01`, `TRP-02`, `TRP-03` |
| §2.1 | O bloco `provider` e o que a sua política permite | `TRP-04` |
| §2.2 | P0-3 e o critério de rejeição | `TRP-05`, `TRP-06` |
| §4.1 | A separação dos dois nomes | `TRP-07`, `TRP-08` |
| §4.2 | Estabilidade do binding: por mensagem, não por processo | `TRP-09`, `TRP-10`, `TRP-11`, `TRP-46` |
| §4.3 | O instante da publicação | `TRP-12` |
| §5.1 | A obrigação recebida | `TRP-13`, `TRP-14`, `TRP-15` |
| §5.2 | A matriz de hops | `TRP-16`, `TRP-17`, `TRP-18`, `TRP-19`, `TRP-20`, `TRP-21` |
| §6.1 | Os três relógios | `TRP-22`, `TRP-23`, `TRP-24`, `TRP-24b`, `TRP-25` |
| §6.2 | A ordem do ACK | `TRP-26`, `TRP-27`, `TRP-28`, `TRP-29`, `TRP-30`, `TRP-31`, `TRP-32`, `TRP-47`, `TRP-48`, `TRP-52` |
| §7 | Validação do envelope, rejeição e destino do inválido | `TRP-33`, `TRP-34`, `TRP-35`, `TRP-36`, `TRP-54` |
| §7.1 | Limites de entrada, antes do decode | `TRP-49`, `TRP-50`, `TRP-51` |
| §8.1 | Os critérios, antes da matriz | `TRP-37` |
| §8.2 | A matriz | `TRP-38`, `TRP-39`, `TRP-40`, `TRP-41` |
| §9 | REST e JSON como interface externa | `RST-01`, `RST-02`, `RST-03`, `RST-04` |
| §10.1 | O transporte e os perfis | `GRP-01`, `GRP-02`, `GRP-03` |
| §10.2 | Deadline e cancelamento | `GRP-04`, `GRP-05`, `GRP-06`, `GRP-07`, `GRP-16`, `GRP-17`, `GRP-18` |
| §10.3 | Retry, balanceamento e disponibilidade | `GRP-08`, `GRP-09`, `GRP-10`, `GRP-11`, `GRP-12`, `GRP-12b` |
| §10.4 | Erro | `GRP-13`, `GRP-14` |
| §10.5 | Segurança do transporte | `GRP-15` |
| §11.1 | Nomenclatura e topologia de tópico | `KFK-01`, `KFK-01b`, `KFK-01c`, `KFK-02`, `KFK-03`, `KFK-03b`, `KFK-04`, `KFK-04b` |
| §11.2 | Chave, ordenação e consumo | `KFK-05`, `KFK-06`, `KFK-07`, `KFK-08`, `KFK-09` |
| §11.3 | Retry e contenção | `KFK-10`, `KFK-11`, `KFK-12` |
| §11.4 | Operação de registry de schemas | `KFK-13`, `KFK-14`, `KFK-15`, `KFK-16`, `KFK-17`, `KFK-18` |
| §12.1 | Envelope no corpo textual | `SQS-01`, `SQS-02`, `SQS-03`, `SQS-03b`, `SQS-03c` |
| §12.2 | FIFO e standard | `SQS-04`, `SQS-05`, `SQS-06`, `SQS-06b`, `SQS-07` |
| §12.3 | Visibilidade, retry e contenção | `SQS-08`, `SQS-08b`, `SQS-09`, `SQS-10`, `SQS-11`, `SQS-11b`, `SQS-12`, `SQS-12b` |
| §12.4 | Prazo e categorias de erro no assíncrono | `SQS-13`, `KFK-19`, `TRP-53` |
| §13 | Os três escopos de deduplicação | `TRP-42`, `TRP-43`, `TRP-44` |
| §15 | Coexistência dos dois transportes | `COE-01`, `COE-02`, `COE-03`, `COE-04`, `COE-05`, `COE-06`, `COE-07`, `COE-08` |
| §16 | Catalogação de canal assíncrono | `ASY-01`, `ASY-02`, `ASY-03`, `ASY-04` |
