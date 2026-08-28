# DMPF — artefatos promovidos

Índice versionado da fundação **DMPF** (épico [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436)).

Os drafts de trabalho continuam em área local, fora do versionamento. Quando
estão prontos para revisão, são **promovidos** para cá; a aprovação formal
ocorre no PR (reviewers de Plataforma/Arquitetura). Specs permanecem em
`docs/specs/`. ADRs do DMPF, quando acionados, usam a faixa `docs/adr/010`–`024`
(contínua aos `001`–`009` do template).

## Artefatos

| Artefato | Status | Spec / Issue |
|----------|--------|--------------|
| [inventario-as-is.md](./inventario-as-is.md) | **baseline candidato** (promovido para revisão; AC externo aberto) | [SPEC-K9H204F1](../specs/SPEC-K9H204F1-dmpf-inventario-as-is.md) / [ARQ-438](https://lider-cap.atlassian.net/browse/ARQ-438) |
| [rfc-dmpf-foundation-v0.1.md](./rfc-dmpf-foundation-v0.1.md) | **`normativo`** (aceito em 2026-08-14) | [SPEC-8YVF0RR5](../specs/SPEC-8YVF0RR5-dmpf-rfc-limites-deps.md) / [ARQ-439](https://lider-cap.atlassian.net/browse/ARQ-439) |
| [upr-decision-mensagens.md](./upr-decision-mensagens.md) | **promovido para revisão** (adição à RFC pela âncora ANC-01, sem editá-la) | [SPEC-8MNDEWDP](../specs/SPEC-8MNDEWDP-dmpf-upr-decision-mensagens.md) / [ARQ-440](https://lider-cap.atlassian.net/browse/ARQ-440) |
| [uow-inbox-outbox.md](./uow-inbox-outbox.md) | **promovido para revisão** (adição à RFC pela âncora ANC-02, sem editá-la) | [SPEC-7PJ5WVCS](../specs/SPEC-7PJ5WVCS-dmpf-uow-inbox-outbox.md) / [ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441) |
| [cloudevents-protobuf-buf.md](./cloudevents-protobuf-buf.md) | **promovido para revisão** (adição à RFC pela âncora ANC-03, sem editá-la) | [SPEC-7H08RZDG](../specs/SPEC-7H08RZDG-dmpf-cloudevents-protobuf-buf.md) / [ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442) |
| [politicas-transporte.md](./politicas-transporte.md) | **promovido para revisão** (adição à RFC pela âncora ANC-04, sem editá-la) | [SPEC-YWFGNPG5](../specs/SPEC-YWFGNPG5-dmpf-politicas-transporte.md) / [ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443) |
| [contexto-erros-seguranca.md](./contexto-erros-seguranca.md) | **promovido para revisão** (adição à RFC pela âncora ANC-05, sem editá-la) | [SPEC-XQWGGAXF](../specs/SPEC-XQWGGAXF-dmpf-contexto-erros-seguranca.md) / [ARQ-444](https://lider-cap.atlassian.net/browse/ARQ-444) |
| [resiliencia-observabilidade.md](./resiliencia-observabilidade.md) | **promovido para revisão** (adição à RFC pela âncora ANC-06, sem editá-la) | [SPEC-E15TBHCD](../specs/SPEC-E15TBHCD-dmpf-resiliencia-observabilidade.md) / [ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445) |
| [testes-interop.md](./testes-interop.md) | **promovido para revisão** (adição à RFC pela âncora ANC-07, sem editá-la) | [SPEC-6RQBN98G](../specs/SPEC-6RQBN98G-dmpf-testes-interop.md) / [ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446) |
| [governanca-bom-pilotos.md](./governanca-bom-pilotos.md) | **promovido para revisão** (adição à RFC pelas âncoras **ANC-08 e ANC-09**, sem editá-la) | [SPEC-VVR1X71Q](../specs/SPEC-VVR1X71Q-dmpf-governanca-bom-pilotos.md) / [ARQ-447](https://lider-cap.atlassian.net/browse/ARQ-447) |
| [reconciliacao.md](./reconciliacao.md) | **registro vivo** (não promovido; fora de M1–M4 — estado das pendências cruzadas, atualizável) | [SPEC-4W1BQK93](../specs/SPEC-4W1BQK93-dmpf-consolidacao-horizontal.md) / [ARQ-492](https://lider-cap.atlassian.net/browse/ARQ-492) |
| [navegacao.md](./navegacao.md) | **registro vivo** (não promovido; fora de M1–M4 — mapa de entrada por prefixo, documento e pergunta) | [SPEC-4W1BQK93](../specs/SPEC-4W1BQK93-dmpf-consolidacao-horizontal.md) / [ARQ-492](https://lider-cap.atlassian.net/browse/ARQ-492) |

A RFC obriga: as regras nela escritas valem para todo trabalho novo do DMPF.

Duas ressalvas ficam registradas na própria RFC. A revisão por área não chegou a
produzir parecer (RFC §14.6), e a promoção ocorreu com o inventário AS-IS ainda
em `baseline candidato` (RFC §1.5) — se a aprovação do inventário alterar
denominadores ou conclusões, as decisões que citam a evidência afetada devem ser
revisitadas. Para a `0.2` em diante, revisão por área e reconciliação com
baseline aprovado voltam a ser exigidas (RFC §14.3).

### Sobre `upr-decision-mensagens.md`

Adiciona à RFC pelo mecanismo de âncora (ANC-01, RFC §12.3), sob as regras de
monotonicidade M1–M4: detalha o bloco `domain library` sem editar a RFC e sem
incremento de versão. Três pendências ficam registradas no próprio artefato:

- a revisão dos exemplos por um representante de cada stack, gate externo não
  bloqueante (§10.3);
- os vetores de equivalência do desfecho e o diagnóstico estável por regra,
  encaminhados a FND-09 (§10.4) e **quitados** por ele — matriz de §3.2, obrigações
  `H3-5` e `H3-6`, com as 54 regras instanciadas em §13.4; ver `REC-001` no
  [ledger de reconciliação](./reconciliacao.md);
- a marcação de Parte-1 §§5 e 6 como consolidados na tabela de RFC §14.4, que o
  artefato não pode fazer sozinho (§1.5).

### Sobre `uow-inbox-outbox.md`

Adiciona à RFC pela âncora ANC-02 (RFC §12.3), sob as mesmas regras de
monotonicidade M1–M4. Diferente da ANC-01, que recorta um bloco, a ANC-02
recorta um **mecanismo** que atravessa `application service`, `provider` e `app`
— por isso cada regra do artefato declara o bloco a que se aplica (§1.3), e a
tabela de sucessão da Parte-1 §§9–10 é por subseção, com coluna de ressalva
(§1.5).

O artefato aciona **dois** ADRs, sem redigir nem aceitar nenhum: `ADR-DMPF-K`
(mecanismo de relay, polling × CDC), exigido pelo próprio registro da ANC-02, e
`ADR-DMPF-L` (bloco do mapeamento e momento da serialização), que responde à
obrigação delegada por escrito em `upr-decision-mensagens.md` §6.3. Os
identificadores são provisórios: a numeração definitiva é do FND-11.

As 64 regras `normativo` substantivas têm **ID estável** (`BLK`, `UOW`, `OBX`,
`INB`, `GAR`), marcado no bloco que enuncia cada uma e indexado em §11.2 — é
por esse ID que o FND-09 vai nomear o cenário que a verifica.

Três pendências ficam registradas no próprio artefato (§11.5):

- refletir na tabela de RFC §14.4 a sucessão de Parte-1 §§9–10, preservando
  §10.7 e §10.8 como vigentes sob ANC-04;
- recolher no glossário de RFC §14.1 os vinte termos que o artefato introduz
  (§11.4);
- converter os doze failure modes de §7.3 em cenários executáveis, com
  diagnóstico estável e par de vetores por regra, encaminhado a FND-09.

### Sobre `cloudevents-protobuf-buf.md`

Adiciona à RFC pela âncora ANC-03 (RFC §12.3), sob as mesmas regras de
monotonicidade M1–M4. Como a ANC-01, a ANC-03 recorta um **bloco** —
`contract package` —, mas com uma tensão própria: o bloco é declarativo, e quem
materializa o contrato em bytes é o `provider` (célula 30 de RFC §7.4). Por isso o
artefato declara, em §1.3, que o **sujeito** de toda regra é o contrato, nunca o
comportamento de outro bloco: a forma do envelope obriga aqui, e quem preenche
cada campo continua sendo de FND-04.

Duas particularidades diferenciam esta adição das anteriores.

A primeira é que o artefato é **devedor** dos dois precedentes: FND-03 e FND-04
lhe delegaram, por escrito, dez obrigações. A matriz de §2.3 lista cada uma com a
fonte em `arquivo:linha`, a seção que a trata, como conferir e o estado — **nove
`quitada` e uma `encaminhada`**. A encaminhada é a escolha de registry, que a
própria âncora manda decidir por ADR; declarar fronteira não conta como quitação.

A segunda é que o **assunto registrado na âncora é maior do que a entrega**: a
ANC-03 nomeia «Protobuf, CloudEvents, OpenAPI e AsyncAPI», e o artefato normatiza
os dois primeiros. Parte-1 §7.6 e §7.7 seguem vigentes, sem sucessor e sem
transferência a outra âncora — a lacuna é nomeada em §10.4 e afeta a condição de
fechamento da ANC-03. A sucessão de Parte-1 §7 é declarada por subseção, com duas
**parciais**: §7.1 (os usos por transporte ficam com ANC-04) e §7.2 (os diretórios
`openapi/` e `asyncapi/` e os descriptors permanecem vigentes).

O artefato aciona **dois** ADRs, sem redigir nem aceitar nenhum: `ADR-DMPF-M`
(codec de wire e modalidade de payload da primeira major) e `ADR-DMPF-N` (escolha
de registry e autoridade de validação). Ambos são exigidos pelo próprio registro
da ANC-03 e confirmados por RFC §13.3. Os identificadores são provisórios,
continuando a série que o FND-04 deixou em `ADR-DMPF-L`: a numeração definitiva é
do FND-11.

As 64 regras `normativo` substantivas têm **ID estável** (`ENV`, `PTB`, `BUF`,
`REP`, `INT`), marcado no bloco que enuncia cada uma e indexado em §10.2. §10.1
declara, regra a regra, o que é decidido mecanicamente e o que não é: os quatro
gates de §6.6 — `buf format`, `buf lint`, `buf breaking` e dupla geração — decidem
`PTB-01`/`PTB-02`/`PTB-04`, `PTB-05` a `PTB-09`, `BUF-01` a `BUF-12` e `REP-01`/
`REP-02` sem julgamento humano. O perfil do envelope (`ENV-08` a `ENV-13`) não tem
gate neste artefato — é propriedade de instância, e a lacuna do instrumento é
pendência registrada; `PTB-10` depende da certificação de toolchain e `PTB-11` do
oráculo de FND-09.

Três decisões deste artefato merecem leitura atenta na revisão:

- a modalidade de payload é **única** nesta major (`proto_data` com
  `google.protobuf.Any`), e as outras duas do `oneof` oficial — `binary_data` e
  `text_data` — ficam vedadas: nenhuma como fallback, nenhuma como exceção por
  contexto (§4.2);
- a fórmula do `payload_hash` é decidida sob H1 e H2 de FND-04 `INB-13`, com duas
  alternativas avaliadas: hash SHA-256 sobre os bytes transportados, sem
  reserialização, com a projeção canônica descartada e o motivo registrado. H2 é
  satisfeita pela fórmula; **H1 é satisfeita sob condição** — a byte-preservação
  pelos transportes é dependência declarada sobre FND-06, registrada como
  pendência (§4.3, §10.4);
- o bootstrap do baseline de `buf breaking` é uma **máquina de estados** de dois
  estados com transição única e autorização distinta da autoria, com três vetores
  negativos — remoção, renomeação e falsificação do marcador (§6.4).

Dez pendências ficam registradas no próprio artefato (§10.4), entre elas: o
recorte incompleto da ANC-03 acima; o realinhamento com FND-09 (§8.5), **já
reconciliado** antes da redação daquele artefato — o critério do round-trip são os
três oráculos, não «mesmo byte» em geral, e o ownership da fixture está declarado em
FND-09 §8.4; ver `REC-004` no [ledger de reconciliação](./reconciliacao.md); e a
ausência de campo dedicado, no schema mínimo da outbox de
FND-04 §4.1, para três atributos que o envelope torna obrigatórios —
`correlationid`, `causationid` e `traceparent` (§4.1).

### Sobre `politicas-transporte.md`

Adiciona à RFC pela âncora ANC-04 (RFC §12.3), sob as mesmas regras de
monotonicidade M1–M4. A ANC-04 recorta um bloco — `provider` —, mas com a
dificuldade inversa da ANC-03: os precedentes encaminharam a este artefato, por
escrito, obrigações cujo sujeito é o bloco `app` (gesto de ACK do adapter,
validação do envelope na recepção, destino do envelope inválido, nack e extensão de
visibilidade). Ler a âncora ao pé da letra deixaria o gesto de ACK sem dona em toda
a fundação, porque não existe âncora que o receba.

Por isso §1.3 declara o sujeito admissível: `TRP-01` fixa o `provider` como sujeito
da regra; `TRP-02` admite a extensão a `app` **apenas** no conjunto fechado das
quatro obrigações recebidas nominalmente, cada uma com a origem citada; e `TRP-03`
invalida por M4 o que estiver fora — em particular a superfície de uma API REST, que
é contrato e adapter de entrada. A tensão entre o escopo literal da âncora e as
obrigações recebidas fica registrada como a **primeira** das nove pendências (§18.3),
e a sua correção definitiva exige rito de versão da RFC.

Este artefato consolida Parte-1 §7.1 (usos por transporte), §10.5 (ACK do adapter),
§10.7 (Kafka), §10.8 (SNS e SQS) e §10.9 (retry por transporte) — as subseções que o
FND-04 preservou vigentes sob ANC-04. Parte-1 §7.6 (REST e OpenAPI) **não** é
sucedida, por `TRP-03`, e §7.7 (AsyncAPI) é parcial: a catalogação por transporte vem
para cá, e a fonte de verdade documental permanece a pendência 2 da ANC-03.

Três entregas merecem leitura atenta na revisão:

- **A pendência 10 do FND-05 está quitada.** Ela pedia a byte-preservação do payload
  pelos transportes, de que depende a hipótese H1 da fórmula do `payload_hash`. §5 a
  fecha com uma **matriz de hops** que classifica cada caminho e nomeia quatro como
  `não conforme` para mensagem que alimente inbox: SNS sem modo bruto, transform que
  reserializa, Connect em `application/json` e transcodificação gRPC-JSON. Cada linha
  é vetor negativo para o FND-09.
- **Os três relógios.** O escopo da story pedia a relação entre «visibility timeout e
  lease do inbox», expressão sem referente: no FND-04 o lease é do relay e da outbox,
  e a inbox não tem lease — tem `INB-14`. §6.1 separa lease de claim, intervalo entre
  tentativas e horizonte de redelivery, e é o terceiro que alimenta `INB-14`.
- **Kafka é o transporte-alvo declarado, sem base as-is.** O inventário registra zero
  clientes Kafka versionados; a política de §11 é prospectiva, o que a RFC §1.6
  autoriza e este artefato declara em vez de simular. `TRP-41` limita a política a
  norma de desenho até a revisão de infraestrutura de mensageria. §15 normatiza a
  coexistência com SNS/SQS; o cronograma de migração é de ANC-09.

O artefato aciona **dois** ADRs, sem redigir nem aceitar nenhum: `ADR-DMPF-O`
(divisão dos transportes síncronos e o governo do tempo) e `ADR-DMPF-P` (Kafka como
alvo do assíncrono, com SNS/SQS no acervo). §17.3 demonstra, transporte por
transporte, como dois ADRs satisfazem a exigência de «ADR por transporte adotado» da
âncora. Os identificadores continuam a série que o FND-05 deixou em `ADR-DMPF-N`, e
são provisórios: a numeração definitiva é do FND-11.

As 131 regras `normativo` têm **ID estável** em sete prefixos (`TRP`, `RST`, `GRP`,
`KFK`, `SQS`, `ASY`, `COE`), indexadas em §18.5. O artefato também recebeu, já em
revisão, seis obrigações do FND-07 (ARQ-444) — o valor do prazo por transporte, o
cabeçalho de tentativa, os limites de tamanho e profundidade antes do decode, a
proteção contra expansão abusiva, os valores de erro por broker e a assinatura em
fronteira não confiável —, e cinco delas ficam quitadas nesta entrega (§18.2).

Nove pendências ficam registradas (§18.3). Duas são de natureza distinta das outras:
não descrevem trabalho que falta, e sim **autoridade que falta** — o escopo literal da
ANC-04 não cobre quatro obrigações que os precedentes encaminharam a esta âncora, e a
tabela de RFC §14.4 ainda não reflete a sucessão que §1.5 propõe. Ambas exigem rito de
versão da RFC, e a segunda vem acumulando desde o FND-04.
### Sobre `contexto-erros-seguranca.md`

Adiciona à RFC pela âncora ANC-05 (RFC §12.3), sob as mesmas regras de
monotonicidade M1–M4. A diferença de forma em relação aos três precedentes
condiciona o artefato inteiro: a ANC-01 recortou um **bloco**, a ANC-02 um
**mecanismo** que atravessa três blocos, a ANC-03 um bloco cujo assunto era maior
que a entrega — e a ANC-05 recorta **três assuntos transversais** (propagação de
contexto, taxonomia de erros e controles de segurança) que incidem sobre todos os
blocos e sobre quase todas as demais âncoras, sem possuir nenhum deles.

Por isso cada regra declara **dois** eixos, e não um: o bloco a que se aplica, como
no FND-04, e o **sujeito** que ela obriga (§1.3). O segundo eixo existe porque sete
das obrigações desta entrega não têm bloco — classificação de dado é decisão de
negócio, cifra em repouso é capacidade de plataforma, teto de retenção costuma ser
externo à engenharia. Sem declarar o sujeito, essas sete pareceriam obrigações de
squad e seriam cobradas de quem não as pode cumprir.

Três particularidades diferenciam esta adição das anteriores.

A primeira é que o artefato é o **maior devedor** da cadeia: FND-03, FND-04 e FND-05
lhe delegaram, por escrito, **24 obrigações atômicas**. A matriz de §2 as trata em
duas passadas — atribuição (§2.2) e prova (§2.3) —, e a passada de prova exige, por
linha, o ID normativo, o predicado ou diagnóstico e o **par de vetores**: o caso que
satisfaz e o caso que viola. Nenhuma obrigação ficou `bloqueada`; duas têm porção
`encaminhada` com dona nomeada. Seis das 24 não constavam da spec original — eram
trabalho invisível — e a spec foi reconciliada antes da redação.

A segunda é que duas obrigações **fecham lacuna já aberta** em artefato mergeado. O
FND-04 opera hoje com placeholder explícito de retryability, «até que FND-07 a fixe»,
que `ERR-11` encerra com um default fechado: categoria condicional sem predicado
decidível resolve para não retentável. E o FND-04 declara textualmente que «declara
onde o dado vive e **não o protege**» — a §8 é a metade que faltava, com cifra em
repouso como **pós-condição** verificável e teto de retenção como **desigualdade**
(`retenção efetiva ≤ teto externo aplicável`), em vez de prescrição de mecanismo.

A terceira é que este artefato **não aciona ADR**, ao contrário do FND-04 e do
FND-05, que acionaram dois cada. A conclusão tem duas fontes convergentes: o registro
da ANC-05 diz «ADR exigido: não, salvo alteração de invariante», e RFC §13.3, ao
enumerar quem aciona, não lista o FND-07. §10.3 testa cada decisão substantiva contra
o gatilho e nenhuma o satisfaz. A afirmação é condicional, não absoluta — §10.2
declara o caminho de M3 como **conjuntivo**: nova versão da RFC **acompanhada de** ADR
aceito, os dois. A story ARQ-444, que lista ADR-012 e ADR-013 como entregáveis, fica
registrada como pendência nomeada em vez de resolvida por texto.

A sucessão de Parte-1 §§11–14 é declarada **por cláusula**, e não por subseção como no
FND-04 (§1.5). A granularidade é imposta pela fonte: Parte-1 §12.1 é um pipeline de
nove estágios, dos quais quatro pertencem a esta âncora, e Parte-1 §11 é uma lista de
nove regras cujo ownership varia de linha para linha — uma delas pedia mecanismo de
persistência que M4 põe fora daqui.

O **threat model** de §7 usa STRIDE como lente de varredura, com saída tabulada por
vetor nos sete vetores nomeados pela spec: adapters, desserialização, contratos,
mensageria, secrets, PII e permissões. Cada tabela traz a linha de **exclusões
justificadas** — categoria ausente sem justificativa é indistinguível de categoria
não avaliada — e nenhuma linha fica sem owner. O eixo *Denial of service* é avaliado
nos sete vetores e **não gera norma** aqui: resiliência é o assunto declarado da
ANC-06. STRIDE é metodologia nova nesta cadeia; RFC §10.2 trata de trust model sobre
classificação autodeclarada e não é precedente disto.

Duas decisões merecem leitura atenta na revisão:

- `CTX-25` — no consumo assíncrono, o sujeito que originou a cadeia é **proveniência,
  não autorização**. Reconstruí-lo do envelope e autorizar com ele parece a
  continuação natural da cadeia e é uma elevação de privilégio diferida: quem
  consegue publicar no tópico passa a escolher a identidade com que o consumidor age;
- `IDN-14` — o isolamento por tenant é normatizado como **resultado** fail-closed, e a
  imposição não pode depender de convenção de código. O mecanismo (constraint, RLS,
  chave particionada) é encaminhado ao provider e ao kernel, e a verificação executável
  a FND-09 sob ANC-07. É a única fronteira em que o artefato **reduz** o que a própria
  spec pedia, e a redução está justificada por M4.

As 112 regras `normativo` têm **ID estável** (`CTX`, `IDN`, `ERR`, `MAP`, `THR`,
`DAT`), contíguo por prefixo e indexado em §11.1 — é por esse ID que o FND-09 vai
nomear o cenário que verifica cada uma. §11.2 declara o modo de verificação por grupo,
distinguindo o que se confere por inspeção do que exige execução.

Cinco pendências ficam registradas no próprio artefato (§11.4), entre elas: o
**owner nomeado** da revisão de Segurança, sem o qual o gate de `THR-03` não é
acionável; a forma de persistir os três atributos obrigatórios sem coluna no schema
da outbox, que este artefato resolveu pelo lado do conteúdo e permanece aberta pelo
lado do schema; e a sugestão de consolidar numa única passagem as quatro pendências
já acumuladas sobre a tabela de RFC §14.4.

### Sobre `resiliencia-observabilidade.md`

Adiciona à RFC pela âncora ANC-06 (RFC §12.3), sob as mesmas regras de
monotonicidade M1–M4. Duas assimetrias em relação aos precedentes condicionam o
artefato.

A primeira é de **volume**: são **30 obrigações herdadas**, o maior conjunto da
cadeia — o FND-07 recebeu 24. Não é acidente. Toda vez que um precedente fixou um
mecanismo, ele precisou declarar que o mecanismo é observável e parou antes de
nomear a métrica, e essa parada tem um só destinatário. O critério que decide a
fronteira foi enunciado pelo FND-04 e é citado literalmente em §2.1: «capacidade é
deste artefato; catálogo é de FND-08».

A segunda é de **recorte**: a ANC-06 registra um assunto («resiliência e
observabilidade») mais amplo que o escopo permitido («baselines de telemetria,
políticas de retry e degradação»). Por M4, o excedente sai `encaminhado` com dona
nomeada — `RES-02` fixa a regra e §1.4 é a lista. É por isso que SLO por serviço,
dashboards, alertas em produção e modelagem de cache **não** estão aqui, embora
sejam o que um leitor esperaria encontrar.

Três entregas merecem leitura atenta na revisão:

- **A matriz de obrigações fecha sem nenhuma pendente.** É a primeira vez na
  cadeia: as 30 saem `quitada`, nenhuma `encaminhada` nem `condicionada` (§2.2). A
  razão é estrutural — obrigação de catálogo não depende de decisão de terceiro
  para existir. As duas que estiveram perto de ficar condicionadas vinham do
  FND-06 e se resolveram quando a ARQ-443 entrou em `develop`.
- **A autorização de retry é uma conjunção, não uma consequência.** `RES-27` exige
  quatro fatores simultâneos — erro retentável, operação idempotente ou efeito
  conhecidamente ausente, orçamento **suficiente para a tentativa e o seu backoff**,
  e prazo remanescente —, verificados **por tentativa**. O orçamento entra no mínimo
  que `RES-06` calcula, de modo que nenhuma tentativa isolada o exceda. É a leitura direta de `ERR-12` do FND-07, que declara que a
  retryability «não autoriza, ordena nem dimensiona a repetição», e o orçamento de
  `RES-30` é por execução justamente para impedir o efeito multiplicativo entre
  dependências.
- **O limiar do catálogo é condicional.** `MET-05` fixa valor de limiar apenas
  onde ele é derivado de invariante já normatizada; nos demais casos declara owner
  e parâmetro local. `MET-05a` separa disso a condição de **forma** — a persistência
  de uma tendência ao longo de janelas —, que é critério de detecção com parâmetro
  local, não limiar de valor. Exigir valor universal produziria número inventado e
  invadiria o SLO por serviço, que o escopo da spec exclui; não fixar nenhum
  esvaziaria a obrigação de limiares recebida do FND-04.

O artefato aciona **um** ADR, sem redigir nem aceitar: `ADR-DMPF-Q` (baseline de
resiliência e observabilidade, com OpenTelemetry como convenção). O identificador
continua a série que o FND-06 deixou em `ADR-DMPF-P` e é provisório — a numeração
definitiva é do FND-11. §12.2 registra por que o campo «ADR exigido: não» da
âncora **não** dispensa o acionamento: ele diz que o ADR não é requisito de
validade da adição, e a tabela de cadência de `SPEC-DBTRMM3X` atribui o grupo
«resiliência, observabilidade» a esta sub-spec. §12.4 testa cada decisão
substantiva contra o gatilho de invariante e nenhuma o satisfaz, o que confirma a
adição dentro do escopo, sem incremento de versão.

As 121 regras `normativo` — incluídos os sufixos `MET-05a`, `RUN-11a` e `RUN-18a`
— têm **ID estável** em cinco prefixos (`RES`, `TRC`,
`MET`, `LOG`, `RUN`), indexadas em §13.1. Nenhuma tem `domain` ou `port` como
sujeito — o que é a verificação direta da invariante da âncora, e a razão pela
qual `RES-01` abre o artefato em vez de aparecer como observação.

§9 faz a rastreabilidade **1:1** que a story exige: os 12 failure modes do FND-04
com o sinal que os torna observáveis. Onze têm gauge ou contador; o failure mode 1
— a UoW que não commita — é o único cujo pendente não é materializado em lugar
nenhum, e por isso o sinal ali é a categoria do erro devolvido. Declará-lo
observável por gauge exigiria inventar estado durável que o mecanismo de FND-04
não tem.

Quatro pendências ficam registradas no próprio artefato (§13.4). Duas são **gates
externos** que a entrega não pode satisfazer por si e que impedem o fechamento da
ARQ-445: a validação do runbook por SRE, ainda **sem owner nomeado** — mesmo
tratamento que o FND-07 deu ao gate de Segurança em `THR-03` —, e o aceite de
`ADR-DMPF-Q` pelo FND-11. A terceira é a atualização de RFC §14.4, agora a quarta
da série a acumular.
### Sobre `testes-interop.md`

Adiciona à RFC pela âncora ANC-07 (RFC §12.3), sob as mesmas regras de
monotonicidade M1–M4. A diferença em relação a todos os precedentes não é de recorte,
e sim de natureza: as seis adições anteriores normatizaram **matéria** — o que a UPR
é, como a outbox drena, que atributo o envelope carrega —, e esta normatiza **prova**.
É a única sub-spec cuja obrigação central não é decidir um assunto próprio, e sim dar
a cada regra já decidida um mecanismo que a torne conferível.

Três consequências decorrem disso, e cada uma tem efeito visível no artefato.

A primeira é que **o acervo é o escopo**. O artefato herda de **sete** fontes — a
RFC e os seis irmãos promovidos (FND-03 a FND-08) —, e a matriz de §3 trata
**31 obrigações atômicas** (`H2-1`..`H8-6`; contra as 24 de FND-07, que herdou de
três). A §13 fecha a cadeia de RFC §14.5 por **identificador**: o acervo enumera
672 identificadores, dos quais **664 são estáveis** — os oito `ADR-DMPF-A`..`H` são
provisórios e ficam fora desse universo. Das 664, **654 recebem linha** e **650 têm
mecanismo de prova**, com o modo de verificação de cada uma calibrando o que a linha
exige. As dez âncoras `ANC-01`..`ANC-10` são índice de encaminhamento, não regra
verificável, e as quatro exceções de mecanismo estão nomeadas em §13.1 — a ausência
é declarada em vez de omitida. A distinção entre 682 cláusulas `normativo` e 672
identificadores está registrada em §1.4: as duas métricas medem coisas diferentes, e
a divergência está na RFC — 110 cláusulas frente a 126 identificadores — e também no
FND-08, com 147 cláusulas em 121 regras.

A segunda é que **a calibração por modo é o que torna a cobertura viável e honesta**.
Exigir vetor de execução de regra que só se confere por inspeção produz teatro de
rastreabilidade; exigir inspeção de regra que só a execução decide produz falso
verde. A §10 fixa a forma — diagnóstico estável reusado de RFC §10.3 quando o defeito
já tem código, critério de inspeção decidível, ou oráculo com par de vetores e o
cenário que o exercita — e a §11 aplica essa régua às regras herdadas, inclusive
declarando o modo das **42 regras de FND-03** publicadas sem modo. Onde a fonte rotula
uma regra `runtime-testable` qualificando «por inspeção de superfície» — o caso de
`DAT-08`, `DAT-10` e `DAT-14` —, a recalibração é explícita e argumentada, nunca
silenciosa.

A terceira é que **fronteira declarada não conta como cobertura** — e a história
desse ponto é instrutiva. Backpressure está ausente de todo o FND-04: é matéria de
resiliência, portanto de ANC-06 e de FND-08. Enquanto FND-08 não tinha artefato
publicado, o artefato declarou o item correspondente do AC-10 **bloqueado, não
satisfeito**, em vez de contar um slot vazio como cenário coberto — que seria o risco
de «decisões apenas documentais» que o épico nomeia.

FND-08 foi publicado durante a redação e fixou o desfecho observável sob saturação:
`RES-14` («a saturação é rejeição rápida»), `RES-08` (o estouro do teto da porta de
inbox resolvendo na disposição que `INB-17` já fixava), `RES-16`/`RES-17` (recusa
categorizada, antes de decode e validação, com sinal por rota e por tenant) e
`MET-11`/`MET-12` (o sinal que separa «recusou por política» de «falhou»). A condição
de fechamento que a própria fronteira declarava foi satisfeita, e o item passou a
coberto por `CEN-44`: **os seis itens do AC-10 ficam cobertos**, com a proveniência
citada por cenário. A fronteira era temporal, não de matéria — o assunto continua
sendo de ANC-06, e o que mudou é que a dona o decidiu.

As demais fronteiras a FND-08 permanecem, por motivo diferente: timing de retry,
cadência de replay e catálogo de métricas se verificam por inspeção ou por evidência
de execução operacional, não por cenário distribuído, e nenhuma é item do AC-10.

Duas particularidades de forma merecem registro. O artefato define uma **classe
distinta de oráculo** para o desfecho de domínio (§7): como `UPR-I11` mantém o
`Decision` fora do wire, o aparato de fixture serializada não o alcança, e a prova de
equivalência entre as stacks se faz por **projeção observável** em codificação neutra,
sem `payload_hash` e sem fixar largura de tipo numérico — respeitando a recusa
deliberada de FND-03 §8.1. E o prefixo dos testes de arquitetura é **`FIT`**, não
`ARQ`: o nome natural da matéria colidiria com a chave do projeto que hospeda o épico,
fazendo `FIT-01` conviver com `ARQ-446` no mesmo parágrafo. O registro da escolha está
em §1.2.

Este artefato **não aciona ADR**, e o fundamento é mais forte que o do FND-07: o
registro da ANC-07 diz «ADR exigido: **Não**», sem a condicional «salvo alteração de
invariante» que ANC-05 e ANC-06 carregam. As duas invariantes que a âncora lista —
RFC §9 (domínio executável em memória) e RFC §4.5 regra 3 (o teste não reclassifica o
código sob teste) — são respeitadas e, no caso da segunda, **restauradas**: a regra em
dois passos de §2 separa o acoplamento no código sob teste, que se repara removendo a
dependência, do teste mal categorizado, que se move de camada.

A spec foi **reconciliada em quatro pontos** antes da redação, cada um citando o irmão
que o força: o critério do round-trip (que não é «mesmo byte» em geral, e sim os três
oráculos, cada um no seu escopo — FND-05 §8.3), o ownership da golden fixture (do
arquivo, do oráculo, do pipeline e do diagnóstico, não do conteúdo — §8.4), a
proveniência dos cenários e o escopo herdado de FND-07. Uma quinta edição foi proposta
e **retirada**: alegava contradição com RFC §4.5 regra 3 onde não havia, porque a
regra fala do código sob teste e a spec reclassificava o teste. O motivo da retirada
está registrado em §3.3.

As pendências ficam em §14, cada uma com dona e condição de fechamento — entre elas a
implementação dos kits e das fixtures pelos épicos de kernel, a localização do
validador do perfil de envelope a co-decidir com FND-06, e o gate de revisão por um
representante de cada stack, que a story pede e que não bloqueia a promoção.

### Sobre `governanca-bom-pilotos.md`

O artefato do FND-10 (ARQ-447) é o **primeiro do acervo a operar sob duas âncoras**:
a ANC-09 («Governança, BOM e pilotos») e a ANC-08 («Processo de autorização da
classificação»). Os assuntos são disjuntos — um é gestão de produto, o outro é
controle de integridade do metadado de classificação —, e `GOV-02` proíbe que uma
regra invoque as duas como se fossem uma licença só.

A ANC-08 também é a única das dez âncoras do registro de RFC §12.3 com **impacto de
versão «Menor»**: definir o processo faz a RFC precisar citá-lo, o que abre a `0.2`.
Esse incremento **não** foi realizado nesta entrega, porque RFC §14.3 exige, de `0.2`
em diante, revisão pelas quatro áreas e reconciliação com baseline aprovado — e o
aceite de 2026-08-14 que dispensou as duas condições vale só para a `0.1`. A pendência
está registrada como gate `G5`.

Três traços distinguem este artefato dos sete precedentes:

- **Os quatro estados observáveis** (§1.3). Ele é o primeiro cujo conteúdo depende de
  atos que só pessoas praticam — assinatura de squad, aceite de titular, criação de
  épico, registro de decisão. Por isso separa `definido` de `vigente`, e ambos de
  `âncora fechada` e `AC fechado`. A construção nasceu de uma ambiguidade real: RFC
  §10.2 diz «até que o FND-10 **defina** o processo» no corpo, «até o FND-10
  **concluir**» no título do mesmo bloco, e ANC-08 fixa o fechamento em «FND-10
  concluída e revisada». O artefato registra a ambiguidade e amarra a transição a
  **três condições cumulativas** (`AUT-09`) — titular aceito, revisão de Segurança e
  **fechamento efetivo da ANC-08** — em vez de escolher a leitura mais favorável, que
  faria o rito viger com uma função sem titular, trocando um controle mínimo operante
  por um controle inoperante.
- **O rótulo `gate externo`**, novo no acervo (§1.2). Cinco atos de terceiro (`G1` a
  `G5`) são endereçáveis por identificador, e `GOV-05` proíbe tratá-los como
  satisfeitos por declaração ou por ausência de menção. §8.4 declara AC-11 item 4 e 5
  como **parciais** e AC-12 itens 1, 3 e 4 como **abertos**, e `RDY-05` reporta as
  três métricas de `ARQ-436 §16.1` que recaem sobre FND-10 como `0`, `≥ 1` e `0 de 5`
  — o oposto do que um artefato de fechamento é tentado a reportar.
- **Os dois ritos mutuamente fechados** (§5). Escape hatch e autorização da
  classificação são instrumentos distintos, e `GOV-26` impede que um produza o efeito
  do outro. O motivo é assimétrico e está declarado: reclassificar é o caminho **mais
  barato** para o efeito de um escape hatch — em vez de exceção nominal com prazo e
  plano de convergência, basta declarar que a unidade pertence a outro bloco. É o
  risco R1 da RFC visto do lado da governança de produto, e `GOV-32` N5 mais `AUT-08`
  V4 e V5 fecham os dois sentidos.

Três correções de fundo que a revisão externa do plano forçou, e que ficam
registradas porque mudam o que o artefato afirma:

- `shared-titulos-services` **não é um piloto Go** — o inventário o registra como
  híbrido (`Stack: ambas — TypeScript/NestJS + Go`). O charter de `PIL-03` nomeia o
  **trecho** Go e coloca os deployables TypeScript do mesmo repositório fora do
  escopo, porque piloto é fluxo e não repositório (`PIL-01`).
- A justificativa do par de pilotos é **cobertura dual-stack**, não
  interoperabilidade (`PIL-02`). Dois pilotos independentes provam adoção em cada
  stack; não provam que bytes de uma são consumidos pela outra. Essa evidência exige
  fluxo equivalente com fixtures cruzadas e é do FND-09, sob ANC-07. A justificativa
  anterior faria o charter parecer satisfazer o item 7 do DoD do épico, que ele não
  satisfaz.
- **`não medido` é estado da coleta, nunca valor de partida** (`PIL-06`). A matriz de
  §6.2 separa `baseline_status` de `baseline_value`, e nenhuma das nove métricas traz
  valor ou alvo. O DoD do épico item 9 admite «registradas **ou** com plano de
  coleta»; o requisito não-funcional da spec pede «valor de partida e alvo». Os dois
  são declarados separadamente, e nenhum é apresentado como o outro.

O artefato registra ainda uma **contradição pré-existente** que não cria e agrava
(§8.2): `SPEC-DBTRMM3X` reserva 15 slots em `docs/adr/010`–`024` e cobra «15 ADRs
mínimos», mas o acervo já tem 17 acionamentos (`ADR-DMPF-A`..`Q`); com
`ADR-DMPF-R` e `ADR-DMPF-S` passa a 19. Os dois são declarados **provisórios e sem
promessa de número final** — RFC §13.2 trata citar número definitivo antes da
promoção como erro de rastreabilidade —, e a reconciliação vai para o FND-11 com um
mapa de consolidação oferecido como insumo. A mesma seção registra que a tabela de
cadência de `SPEC-DBTRMM3X` **omite FND-10** por omissão, não por decisão: ANC-08
declara «ADR exigido: Sim».

Duas correções vieram de revisão adversarial externa e mudam o que o artefato
obriga, então ficam registradas:

- **`AUT-01` cobria só o `block` efetivo.** A regra fazia o rito de autorização
  disparar quando uma mudança de `include` alterasse o *bloco* efetivo de um trecho de
  código, e deixava passar a alteração do *`bounded_context`* efetivo. O caminho que
  isso abria: duas unidades ambas `domain`, em contextos distintos; mover um arquivo de
  uma para a outra por `include`, sem tocar em nenhum campo, libera uma aresta que RFC
  §7.1 reprovava por **C2** (`DMPF-D002`) — sem T4, sem autorização. O ato regulado
  passou a ser o **delta efetivo** `arquivo → (canonical_key, block, bounded_context)`,
  com tabela de três casos, o campo `moved_paths` em `AUT-05` e o vetor negativo `V3a`.
- **§6.1 excedia o escopo da ANC-09.** O escopo registrado é «Matriz de versões
  certificadas, escape hatches e seleção de pilotos»; adoção organizacional e
  faseamento não estão entre os três. O FND-06 §15 e o FND-08 §13 delegam o assunto a
  FND-10 «sob ANC-09», mas **encaminhamento de sub-spec não amplia escopo de âncora** —
  só a RFC o faz, e M4 invalida adição fora do escopo ainda que correta. `ADO-00`
  passou a **suspender a força normativa** de `ADO-01` a `ADO-08`, que ficam `definido`
  até a ampliação da fronteira (pendência **P13**, dona: owner da RFC). No intervalo, a
  norma aplicável é a dos irmãos, que vale por âncora própria (`COE-01`..`COE-06`,
  `TRP-10`). É o mesmo mecanismo que o FND-08 aplicou a si mesmo em `RES-02`.

Fronteira com o FND-09: este artefato **não cita conteúdo** do `testes-interop.md`
(`GOV-06`). A razão original — o FND-09 não estava no histórico versionado durante a
redação — **caducou durante a própria entrega**, porque o PR #14 do FND-09 foi
mergeado em `develop` antes deste PR. A decisão é mantida por **escopo**: incorporar
as regras do FND-09 exigiria revisitar `PIL-02`, que é justamente a fronteira entre
cobertura dual-stack e evidência de interoperabilidade, e isso é reabrir §6.2 em vez
de acrescentar uma citação. A reconciliação fica registrada como pendência P9. O
merge foi verificado: o FND-09 usa `PIR`, `CEN`, `FIX`, `ORA`, `KIT`, `FIT` e `RAS`,
**sem colisão** com os seis prefixos deste artefato, e não endereça obrigação nova a
FND-10 — o ledger de doze permanece completo.

## Evidências do inventário

Há **13 relatórios** cobrindo **10 repositórios** (alguns monorepos têm mais de
um corte) em [`docs/specs/SPEC-K9H204F1/`](../specs/SPEC-K9H204F1/).
