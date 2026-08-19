# Perfil CloudEvents, política Protobuf e governança Buf — DMPF FND-05

| Campo | Valor |
|-------|-------|
| **Status** | `draft normativo` — promovido para revisão em PR |
| **Adiciona a** | RFC DMPF Foundation v0.1, pela âncora ANC-03 (RFC §12.3) |
| **Owner** | Mateus Macedo Dos Anjos (assignee de [ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442)) |
| **Épico** | [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436) — Golden Path para Sistemas Orientados a Domínio e Mensagens |
| **Story** | [ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442) (DMPF-FND-05) |
| **Spec** | [SPEC-7H08RZDG](../specs/SPEC-7H08RZDG-dmpf-cloudevents-protobuf-buf.md) |
| **Data** | 2026-08-19 |
| **Revisão** | Plataforma e Arquitetura no PR; um representante de cada stack (Go e TypeScript) para §5, §6 e §8; um representante de operação para a máquina de estados de §6.4 |

> **O que este documento obriga.** As regras rotuladas `normativo` valem para
> todo trabalho novo do DMPF, na mesma força da RFC à qual elas se adicionam.
> Nem todo bloco aqui obriga: §1.3 define as cinco categorias de conteúdo e a
> fronteira de cada uma. Enquanto o status for `draft normativo`, o documento
> está em revisão; a promoção ocorre no aceite do PR.

---

## §1. Fronteira, referência e sucessão

Esta seção vem antes de qualquer regra porque um artefato que adiciona a uma
norma compartilhada precisa dizer, primeiro, **até onde** ele pode obrigar. A
RFC já respondeu a essa pergunta ao registrar a âncora ANC-03; o que segue é a
leitura dessa autorização, a convenção de leitura do texto e o efeito deste
documento sobre a base conceitual a que ele sucede.

Há uma assimetria em relação aos dois precedentes, e ela condiciona todo o
resto. A ANC-01 recortou um bloco e o FND-03 o esgotou; a ANC-02 recortou um
mecanismo que atravessa três blocos, e o FND-04 declarou o bloco de cada regra.
A ANC-03 recorta um bloco — `contract package` — mas o **assunto** registrado
nela é maior do que este artefato entrega: ele nomeia «Protobuf, CloudEvents,
OpenAPI e AsyncAPI», e o que segue normatiza os dois primeiros. §1.5 declara a
lacuna por subseção, e §10.4 a registra como pendência aberta, em vez de
transferi-la a quem não a pediu.

### §1.1 A autorização: âncora ANC-03

`normativo`

Este artefato **adiciona** à RFC DMPF Foundation v0.1 pela âncora ANC-03
(RFC §12.3). Ele não edita a RFC e não incrementa a versão dela: adição por
âncora dentro do escopo permitido é revisão em PR, sem incremento
(RFC §14.2). O conteúdo vive aqui, e a RFC o alcança pelo endereço estável da
âncora (RFC §12.1).

| Campo | Valor, conforme o registro de ANC-03 |
|-------|--------------------------------------|
| Assunto | Protobuf, CloudEvents, OpenAPI e AsyncAPI |
| Escopo permitido | Formato, versionamento e evolução dos contratos **dentro** do bloco `contract package` |
| Invariantes intocáveis | **P0-2**; células 6, 12, 24 e 31 de RFC §7.4; RFC §6.2 (`contract package` limitado a `pure` e `wire.codec`) |
| Monotonicidade | M1–M4 (RFC §12.2): detalhar e restringir, nunca relaxar, revogar ou reinterpretar |
| Artefato sucessor | Spec e seção própria |
| Condição de fechamento | FND-05 concluída e revisada |
| Impacto de versão na RFC | Nenhum, se dentro do escopo |
| ADR exigido | **Sim** — escolha de codec e de registry. Acionado em §9 |

`normativo` — **Precedência.** Onde este artefato e a RFC divergirem, prevalece
a RFC. Uma divergência não se resolve neste documento: por M4, ela indica que a
fronteira de RFC §1.4 mudou, e mudar fronteira é mudança de versão da RFC, com
o rito de RFC §14.2.

`normativo` — **Monotonicidade em concreto.** As invariantes da âncora são
citadas ao longo do texto, e nenhuma regra deste artefato as afrouxa:

| Invariante | Como este artefato a trata | Onde |
|------------|----------------------------|------|
| **P0-2** — Protobuf apenas no wire | Reafirmada e **restringida**: o tipo gerado é o objeto de wire e nada mais; §5.2 veda as formas de contrato que existiriam apenas para servir de modelo interno | §2.1, §5.1, §5.2 |
| Célula 6 de RFC §7.4 (`domain → contract`) | Preservada sem exceção: nenhuma regra daqui exige que o domínio conheça um tipo gerado | §2.1, §4.1 |
| Célula 12 (`application → contract`) | Preservada: o perfil é preenchido fora da aplicação, e este artefato não move o preenchimento para ela | §2.2, §4.1 |
| Célula 24 (`port → contract`) | Preservada: nenhuma assinatura de porta neste artefato expõe tipo de wire | §2.2 |
| Célula 31 (`contract → domain`) | Preservada: o `.proto` não importa nem espelha tipo de domínio por dependência | §2.1, §5.2 |
| RFC §6.2 — `contract package` limitado a `pure` e `wire.codec` | Reproduzida e usada como critério de rejeição: um contrato que exija I/O para ser lido, validado ou resolvido excede a política do bloco | §2.1, §6.5 |

`normativo` — **P0-3 é guardada por este artefato, mesmo sem constar do registro
da âncora.** RFC §2.3 declara que «nenhum componente, documento, configuração ou
contrato pode prometer exactly-once fim a fim», e fixa o modo de verificação
como `structurally reviewable`. Contrato e configuração são exatamente a matéria
deste texto: um comentário em `.proto`, um nome de campo do envelope, um nome de
opção ou uma linha de `README` do repositório de contratos podem sugerir a
garantia sem que ninguém tenha decidido prometê-la. A vedação, portanto, incide
aqui por alcance de P0-3 — não por delegação —, e §10.3 registra o vetor
documental negativo que a exercita.

`rationale` — Incluir P0-3 na fronteira é restrição, não alargamento: M1 autoriza
tornar mais estrito, e a alternativa era o silêncio. O silêncio seria o pior
resultado possível, porque o lugar mais provável de aparecer uma promessa de
exactly-once não é o documento que fala de entrega — esse já a proíbe em FND-04
§7.1 — e sim o comentário de um campo de contrato, escrito por quem nunca leu a
RFC.

### §1.2 Convenção de referência

`normativo`

Cinco documentos de numeração própria e sobreposta participam desta cadeia,
então toda referência é qualificada:

| Forma | Designa |
|-------|---------|
| `§N` sem prefixo | uma seção **deste artefato** |
| `RFC §N` | uma seção da RFC DMPF Foundation v0.1 |
| `Parte-1 §N` | um capítulo da base conceitual `Parte-1-conceitual.md` |
| `FND-03 §N` | uma seção do artefato `upr-decision-mensagens.md`, promovido sob a ANC-01 |
| `FND-04 §N` | uma seção do artefato `uow-inbox-outbox.md`, promovido sob a ANC-02 |

`normativo` — O prefixo nulo tem referente **local**: dentro deste documento,
`§N` é sempre uma seção deste documento. A convenção difere da de RFC §1.3,
onde o prefixo nulo designa a própria RFC, e segue a de FND-04 §1.2, que resolve
o prefixo nulo em si mesma. Uma citação a este artefato feita **de fora** dele
usa o nome do arquivo mais a seção.

`rationale` — A quinta forma é necessária porque este artefato é o **devedor**
dos dois precedentes: FND-03 e FND-04 lhe delegaram obrigações por escrito, e a
matriz de §2.3 cita as duas fontes linha a linha. Citar sempre pelo caminho do
arquivo tornaria a matriz ilegível.

### §1.3 Classificação de força e o sujeito da norma

`normativo`

A ANC-03 autoriza normatizar «formato, versionamento e evolução dos contratos
**dentro** do bloco `contract package`». O recorte é de um bloco, como em
ANC-01 — mas com uma tensão que a ANC-01 não tinha: o `contract package` é
**declaração**, e quem o materializa em bytes é outro bloco. RFC §7.4 célula 30
põe a serialização no `provider`, e FND-04 §2.2 já decidiu que o mapeamento
reside no `provider` da outbox, com a serialização ocorrendo na escrita.

| Rótulo | Significado | Obriga? |
|--------|-------------|---------|
| `normativo` | Regra que este artefato estabelece sobre o **contrato** — a forma do envelope, a forma do `.proto`, a governança do repositório e a evolução dos dois —, no escopo da ANC-03 | **Sim** |
| `recepcionado` | Conteúdo reproduzido da Parte-1, da RFC ou de um artefato precedente para dar contexto contíguo, sem força nova e sem reabertura | Não — a força permanece da fonte |
| `encaminhado` | Assunto de outra sub-spec, citado apenas como fronteira rotulada, com dona explícita | Não — passa a obrigar quando a dona normatizar |
| `rationale` | Justificativa de uma decisão normativa, incluindo alternativas descartadas | Não |
| `registro` | Acionamento de ADR (§9), matriz de obrigações (§2.3), rastreabilidade e pendências (§10) | Não |

`normativo` — **O sujeito de toda regra `normativo` deste artefato é o contrato,
nunca o comportamento de um bloco que não seja o `contract package`.** Uma
regra daqui pode dizer que o envelope tem `correlationid` obrigatório, que o
campo removido fica `reserved` e que o baseline de `buf breaking` é resolvível.
Ela **não** pode dizer quem gera o valor de `correlationid`, em que momento a
serialização ocorre ou qual bloco escreve na outbox — isso é de FND-04 e de
FND-07, e aparece aqui apenas como `recepcionado` ou `encaminhado`.

`normativo` — Uma regra deste artefato cujo sujeito seja o comportamento de
`application service`, `provider`, `app`, `port` ou `domain library` é defeito
de redação e inválida por M4, ainda que tecnicamente correta.

`rationale` — Sem esse critério, a fronteira desapareceria por adjacência. Todo
campo de envelope tem alguém que o preenche, e descrever o campo convida a
descrever o preenchimento — que é justamente a matéria que FND-04 §2.3 já
normatizou e que este artefato não pode reabrir. O critério não impede o erro;
torna-o visível na revisão, que é onde M4 se aplica na prática. É o análogo,
para uma âncora de bloco declarativo, da exigência de declarar o bloco em cada
regra que FND-04 §1.3 adotou para uma âncora de mecanismo.

`normativo` não é o rótulo default: um bloco sem rótulo é prosa de ligação e não
obriga nada. Os rótulos `normativo` e `rationale` têm aqui exatamente o sentido
de RFC §1.1; `recepcionado` usa o verbo no sentido de RFC §1.3; `encaminhado`
nomeia o gesto que RFC §1.4 pratica sem nomear.

### §1.4 O que este artefato não normatiza

`normativo` — espelha RFC §1.4 no recorte que toca contratos, envelope e
governança de schema.

Cada tema abaixo aparece neste documento **apenas** como fronteira rotulada
`encaminhado`. Onde ele aparece, o texto diz o que o contrato exige até a
fronteira, e para: o outro lado é da dona.

| Tema | Dona | Fronteira aparece em |
|------|------|----------------------|
| Bloco em que o mapeamento reside, momento da serialização, atomicidade da escrita e autoria dos campos da outbox | FND-04 ([ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)), sob ANC-02 | §2.2, §4.1, §4.3 |
| Endereço concreto do transporte, binding por protocolo, convenção de tópico e de fila, gesto de ACK, operação de Schema Registry e retry por transporte | FND-06 ([ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443)), sob ANC-04 | §3.5, §6.5, §7.3, §9.2 |
| Classificação, minimização, cifra em repouso e controle de acesso ao dado de negócio carregado no `data` do envelope; taxonomia de erros; autorização que resolve o `tenantid` | FND-07 ([ARQ-444](https://lider-cap.atlassian.net/browse/ARQ-444)) | §3.4, §4.2 |
| Catálogo de métricas, limiares e alarmes sobre falha de validação, drift de código gerado e reprovação de gate | FND-08 ([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445)), sob ANC-06 | §6.6 |
| Instrumento de teste, oráculo executável, pipeline das golden fixtures e verificação cross-stack do `payload_hash` | FND-09 ([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)), sob ANC-07 | §8.3, §8.5, §10.4 |
| Redação, promoção e aceite dos ADRs acionados | FND-11 ([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)) | §9 |

`normativo` — A fronteira com FND-09 é a mais fácil de atravessar por descuido,
e por isso é enunciada uma vez, aqui, no critério que a decide: **a fixture e o
que ela deve provar são deste artefato; o oráculo que a executa e o pipeline
que a roda são de FND-09.** Que exista um cenário round-trip de direção dupla,
com três oráculos separados e um contrato de exemplo, é propriedade do contrato
e obriga aqui (§8). Como o teste é escrito, em que suíte roda, com que
instrumento se compara e qual diagnóstico ele emite é de ANC-07 e não obriga
aqui, mesmo que este texto o mencione.

`normativo` — **A existência de um campo neste artefato não isenta a obrigação
de protegê-lo.** Este artefato define a forma do envelope, incluindo o campo que
carrega conteúdo de negócio. A proteção desse conteúdo — classificação,
minimização, cifra em repouso e controle de acesso — é de FND-07, não daqui.
Este artefato declara qual é a forma do dado e onde ele vive; não o protege. A
assimetria contrária seria falsa cobertura, e o precedente está em FND-04 §1.4.

Fora do épico inteiro, por P0-4: kernels Go e TypeScript, adapters e providers
de produção. Este artefato especifica o perfil, a política e os gates;
construí-los em cada stack e criar o repositório de contratos é dos épicos de
contratos e de kernel. Também ficam fora, por decisão da própria spec: a
migração dos contratos hoje em uso — inventariados em FND-01 —, os geradores e
templates de código, e a operação de um Schema Registry em produção.

Fora por natureza: **quais** fatos de negócio cada bounded context publica. O
documento normatiza a forma do contrato público e o rito da sua evolução; o que
entra em cada contrato é decisão do time dono do agregado, sob FND-03 `CTR-07`.

### §1.5 Sucessão sobre a Parte-1

`normativo` — detalha RFC §1.7 e continua RFC §14.4 no recorte da ANC-03.

RFC §14.4 registra que as decisões de contratos «seguem vigentes na Parte-1 até
as sub-specs correspondentes». Esta é a sub-spec correspondente ao recorte da
ANC-03. A tabela abaixo declara o estado de cada subseção da Parte-1 §7 no
escopo desta âncora.

`normativo` — A sucessão é declarada **por subseção**, e não por capítulo, e
duas subseções são sucedidas apenas **em parte**. A coluna **Ressalva** nomeia,
em cada linha, o que não é sucedido aqui e quem responde por ele.

| Subseção da Parte-1 | Estado | Ressalva — o que não é sucedido |
|---------------------|--------|--------------------------------|
| Parte-1 §7.1 Uso de Protobuf | **Parcialmente consolidado** em §5.1 | A lista de usos por transporte — gRPC service-to-service, Kafka, SNS/SQS, request/reply — é matéria de ANC-04 / FND-06 e segue vigente na Parte-1. Aqui consolidam-se o *shape*, o naming e a evolução das categorias de contrato, e a reafirmação de P0-2 |
| Parte-1 §7.2 Repositório de contratos | **Parcialmente consolidado** em §7 | Os diretórios `openapi/` e `asyncapi/` da árvore seguem **vigentes** na Parte-1: este artefato não normatiza OpenAPI nem AsyncAPI (§1.4, §10.4) e não pode declarar sucedido o que não substituiu. Os `descriptors`, que na fonte são gerados a partir dos `.proto`, são alcançados por `REP-02` e `BUF-11` |
| Parte-1 §7.3 Envelope assíncrono | **Consolidado** em §3 e §4.2 | — |
| Parte-1 §7.4 Nomenclatura | **Consolidado** em §5.1 | — |
| Parte-1 §7.5 Evolução de Protobuf | **Consolidado** em §5.2, §5.3, §5.4 e §6 | — |
| Parte-1 §7.6 REST e OpenAPI | **Vigente** na Parte-1 | A subseção inteira. Não é sucedida aqui e não é transferida a outra âncora: a lacuna é nomeada em §10.4 |
| Parte-1 §7.7 AsyncAPI | **Vigente** na Parte-1 | A subseção inteira. A catalogação AsyncAPI por transporte é de ARQ-443; a fonte de verdade documental do canal permanece sem dona declarada, e a lacuna é nomeada em §10.4 |

`normativo` — Não há revogação implícita. Uma subseção da Parte-1 fora da tabela
acima continua valendo como base conceitual, e uma subseção marcada `Vigente`
ou `Parcialmente consolidado` continua sendo a fonte no que a ressalva preserva.

`normativo` — Dentro do escopo consolidado, onde este artefato e a Parte-1
divergirem, prevalece este artefato. As divergências materiais são **duas**, e
nenhuma é silenciosa: a atribuição da conversão a «um mapper na camada de
aplicação» (Parte-1 §6.3), já recusada por FND-03 §6.3 e decidida por FND-04
§2.2, é reconciliada em §2.2; e a preferência da Parte-1 §7.3 por `proto_data`
condicionada à toolchain é reconciliada em §4.2, que decide uma modalidade
única e converte a condição em critério verificável, não em preferência.

`normativo` — Há uma **terceira** divergência, de presença de atributo, e ela é
declarada aqui em vez de ficar implícita na tabela de `ENV-08`: a Parte-1 §7.3
lista `aggregateversion` entre os atributos que «o perfil DMPF torna
obrigatórios», sem qualificação — ao contrário de `tenantid`, que já lá aparece
como «quando aplicável». `ENV-08` o torna **condicional**, obrigatório quando o
fato deriva de agregado versionado e ausente quando não deriva. A razão é a mesma
que sustenta `ENV-12`: obrigatoriedade sem predicado produz valor de preenchimento
em evento que não tem agregado, e um `aggregateversion` de preenchimento corrompe
exatamente a detecção de ordem que ele existe para servir. Dentro do escopo
consolidado prevalece este artefato (§1.5), e a divergência não é silenciosa.

`rationale` — **Por que duas sucessões são parciais.** A alternativa era
declarar Parte-1 §7 inteiro consolidado. Ela é mais limpa de ler e é falsa em
dois pontos verificáveis: a Parte-1 §7.1 lista usos por transporte que ANC-04
reserva a FND-06, e a Parte-1 §7.2 desenha uma árvore com `openapi/` e
`asyncapi/` que este artefato não normatiza. Declarar consolidado o que não foi
substituído revoga norma por descuido — e revogação silenciosa é o pior defeito
possível num documento cuja função é ser a referência de quem implementa.

`rationale` — **Por que Parte-1 §7.6 e §7.7 não são transferidas a FND-06.**
Seria
cômodo escrever «pertencem a ANC-04» e fechar a linha. Mas ANC-04 autoriza
«políticas específicas de cada transporte, no bloco `provider`», e a fonte de
verdade documental de uma API REST não é política de transporte — é contrato. O
assunto da ANC-03 as inclui; este artefato não as entrega. Transferir seria
inventar dona; declarar consolidado seria mentir. O que resta, e o que se faz, é
nomear a lacuna com o seu efeito prático (§10.4): a condição de fechamento da
ANC-03 não é integralmente satisfeita por esta entrega, e quem a fechar precisa
saber disso.

`registro` — **Pendência aberta: atualizar a tabela de RFC §14.4.** Enquanto a
tabela da RFC listar as decisões de contratos da Parte-1 como vigentes sem
qualificação, quem implementa não tem como saber, a partir da RFC sozinha, que
Parte-1 §7.3–§7.5 já foi sucedida e que §7.1–§7.2 o foram em parte. Fica
registrado, no mesmo estatuto das pendências equivalentes de FND-03 §1.5 e
FND-04 §1.5:

| Item | Estado | Destino |
|------|--------|---------|
| Refletir na tabela de RFC §14.4 a sucessão de Parte-1 §7 declarada acima, preservando §7.6 e §7.7 como vigentes e as parciais de §7.1 e §7.2 | pendente | Revisores desta entrega, no PR; se exigir rito de versão, vira alteração própria da RFC |

Até que isso ocorra, a sucessão declarada acima vale por autorização da ANC-03,
e a divergência é conhecida — não silenciosa.

---

## §2. O bloco, a autoria e as obrigações herdadas

Antes de descrever qualquer forma, este artefato precisa dizer três coisas: o
que o bloco `contract package` pode conter, o que ele recebe já decidido pelos
precedentes, e quais dívidas escritas ele vem quitar. As duas primeiras
delimitam; a terceira é a razão de a entrega existir na ordem em que existe —
FND-03 e FND-04 delegaram, por escrito, dez obrigações a este texto, e ficar em
silêncio sobre qualquer uma delas quebra a cadeia de delegação registrada.

### §2.1 O `contract package` e a capability `wire.codec`

`recepcionado` — RFC §4.1 define o bloco; RFC §6.2 fixa a sua política de
capabilities; RFC §7.4 decide as **seis arestas que entram** nele — células 6, 12,
18, 24, 30 e 36 — e as seis que saem dele, na linha `contract` da matriz.

O `contract package` é o bloco dos contratos de wire versionados. A sua política
é **restrita**: `pure` e `wire.codec`, e nada mais. As quatro invariantes da
ANC-03 recortam exatamente esse bloco, e as **quatro** arestas proibidas têm
`P0-2` como fonte na matriz. Três são de entrada: nada em `domain`, `application`
ou `port` pode depender dele (células 6, 12 e 24). A quarta é de saída: ele não
pode depender de `domain` (célula 31). Das arestas de entrada sobram duas
permitidas — `app → contract` (célula 18) e `provider → contract` (célula 30) — e
uma interna, `contract → contract` (célula 36), que é o que torna possível compor
envelope e payload em pacotes distintos.

| ID | Regra |
|----|-------|
| `ENV-01` | O `contract package` contém **declaração** de contrato — arquivos `.proto`, a sua configuração de governança e os artefatos gerados a partir deles. Não contém regra de negócio, decisão de roteamento, acesso a rede, leitura de arquivo, variável de ambiente nem cliente de registry |
| `ENV-02` | Um contrato que só possa ser lido, validado ou resolvido mediante I/O **excede** a política de RFC §6.2 e é inválido. Validação de contrato é operação local sobre bytes e sobre o descriptor; resolver um schema por chamada de rede em tempo de desserialização é dependência `io.network` dentro de um bloco que a proíbe |
| `ENV-03` | Um tipo gerado de Protobuf é o objeto de wire e **apenas** ele. Nenhuma forma de contrato deste artefato existe para servir de estado de domínio, de entrada de UPR ou de evento de domínio — é a mesma regra de FND-03 `CTR-02`, vista do lado do contrato |

`normativo` `ENV-01`, `ENV-02`, `ENV-03` — bloco `contract package`.

`rationale` — `ENV-02` parece redundante diante de RFC §6.2 e não é. A tentação
concreta tem nome: um serializer que, ao encontrar um `dataschema` desconhecido,
busca o schema num registry remoto e prossegue. O comportamento é comum em
ecossistemas de Schema Registry, é conveniente, e coloca uma chamada de rede no
caminho de desserialização de um bloco cuja política permite apenas `pure` e
`wire.codec`. Enunciar a vedação aqui é o que torna a revisão capaz de recusá-la
sem discutir política de bloco no meio de um code review de contrato. Onde a
resolução remota é desejada, ela vive no `provider`, que pode ter `io.network` —
e a decisão sobre haver ou não registry em runtime é do ADR de §9.2.

### §2.2 O que este artefato recebe decidido, e o que ele decide

`recepcionado` — Duas decisões chegam a este texto já tomadas, e ele não as
reabre:

| Decisão | Quem decidiu | Efeito aqui |
|---------|--------------|-------------|
| A conversão `domain event → integration event` ocorre **fora da UPR**, e o evento de domínio não é contrato público por existir | FND-03 §6.3 (`CTR-04`, `CTR-06`, `CTR-07`) | Este artefato define **em que** a conversão resulta; não reabre onde ela não pode residir |
| O mapeamento reside no `provider` da outbox, e a serialização ocorre **na escrita** | FND-04 §2.2 (`BLK-03`), acionado como `ADR-DMPF-L` | Este artefato define a **forma** dos dois lados da conversão; não reabre o bloco nem o momento |

`normativo` — **A assinatura concreta do mapeamento, no nível do contrato.**
FND-03 §6.3 delegou expressamente a este artefato «o formato do integration
event, o codec, o registry, o namespacing, a política de compatibilidade e a
assinatura concreta do mapeamento», e FND-04 §2.2 repetiu a delegação nos mesmos
termos, reservando para si apenas o bloco de residência. A forma canônica que
FND-03 §6.3 enuncia — `mapear(domain event, contexto de publicação) ->
integration event` — recebe aqui os seus dois lados de wire:

| ID | Regra |
|----|-------|
| `ENV-04` | O **produto** da conversão é um envelope CloudEvents conforme §3, cujo `data` é a mensagem Protobuf de um contrato de categoria `event` declarado no repositório de contratos (§7), identificada pelo nome qualificado de §5.1 |
| `ENV-05` | O contrato de saída da conversão é **um por tipo publicado**: um `.proto` de integration event não recebe união de fatos heterogêneos por campo `oneof` de conveniência, nem campo genérico de mapa livre destinado a carregar o que não caberia no schema |
| `ENV-06` | A conversão não é reversível pelo contrato: nenhum contrato deste repositório declara mensagem cuja finalidade seja reconstruir o evento de domínio do produtor — é a expressão, no nível do wire, de FND-03 `CTR-06` |

`normativo` `ENV-04`, `ENV-05`, `ENV-06` — bloco `contract package`.

`normativo` — **Vetores para `ENV-05` e `ENV-06`.** As duas regras têm predicado
que um revisor precisa aplicar, e o par abaixo é o que as torna decidíveis em vez
de opinativas:

| Regra | Vetor positivo — passa | Vetor negativo — reprova |
|-------|------------------------|--------------------------|
| `ENV-05` | `OrderPlaced` com os campos do fato; um `oneof` que modele variantes **do mesmo fato** com esquema declarado em cada braço (por exemplo forma de pagamento) | `map<string, string> extra`, `google.protobuf.Struct payload_extra` ou `bytes raw` destinados a carregar o que não caberia no schema; um `oneof` que agrupe `OrderPlaced`, `OrderCancelled` e `PaymentApproved` numa mensagem `OrderEvent` genérica |
| `ENV-06` | Nenhuma mensagem do repositório declara conversão de volta ao evento de domínio do produtor | Um contrato `OrderPlacedDomainView` cuja finalidade declarada seja reconstruir o evento interno do produtor a partir do integration event |

`normativo` — O teste de `ENV-05` é **conteúdo declarado**: um `oneof` cujos braços
têm schema é forma legítima; um campo cujo conteúdo só é conhecido por acordo fora
do contrato não é. A diferença é verificável no próprio `.proto`, sem consultar o
produtor.

`encaminhado` — Fica com FND-04, sob ANC-02, **o bloco em que o mapeamento
reside e o momento em que ele ocorre** — decididos em FND-04 §2.2 e acionados
como `ADR-DMPF-L`. A assinatura em tipos de programa de cada stack — nomes de
função, forma do parâmetro de contexto, tratamento de erro — é dos épicos de
kernel, por P0-4. Este artefato fixa **em que** a conversão resulta no wire; não
fixa como ela é escrita em Go ou em TypeScript.

`rationale` — `ENV-05` é a regra que a fonte não tinha e que a prática mais
viola, e ela existe por uma razão de evolução, não de estética. Um campo genérico
— `map<string, string> extra`, um `google.protobuf.Struct` de escape, um `oneof`
que agrupa cinco fatos sem relação — desliga a governança de §6 sem quebrar
nenhum gate: `buf breaking` continua passando, porque a forma do contrato não
muda quando o conteúdo do mapa muda. O resultado é um contrato que declara pouco
e transporta tudo, e cujos consumidores dependem de um acordo que não está em
lugar nenhum. Vetar isso no nível do contrato é o que mantém o breaking check
com significado.

### §2.3 Matriz das obrigações herdadas

`registro` — Dez obrigações foram delegadas a este artefato por escrito. Cada
linha nomeia a fonte com `arquivo:linha`, a obrigação **atômica** que dela
decorre, a seção que a trata, como conferir, e o **estado**. Os estados são três,
e a distinção entre eles é o que impede a matriz de virar autoelogio:

| Estado | Significado |
|--------|-------------|
| `quitada` | Este artefato **decide** a matéria delegada, com regra de ID estável ou tabela normativa |
| `encaminhada` | Este artefato **não** decide: nomeia a dona e o rito. Declarar a fronteira não quita a obrigação |
| `bloqueada` | Este artefato não decide e não tem a quem encaminhar: falta insumo, e o item fica aberto em §10.4 |

| # | Fonte da delegação | Obrigação atômica | Seção | Como conferir | Estado |
|---|--------------------|-------------------|-------|---------------|--------|
| 1 | `uow-inbox-outbox.md:159`, `:389`; `upr-decision-mensagens.md:130` | Formato do envelope publicado | §3, §4.2 | Tabela dos atributos do perfil, com força e cardinalidade por atributo, e a modalidade de `data` decidida | `quitada` |
| 2 | `uow-inbox-outbox.md:159`; `upr-decision-mensagens.md:130` | Codec do wire — qual serialização é oficial na primeira major | §4.2 | `ENV-15` fixa a modalidade única; `ADR-DMPF-M` acionado em §9.2 com os trade-offs | `quitada` |
| 3 | `uow-inbox-outbox.md:159`; `upr-decision-mensagens.md:130` | Registry — onde reside a autoridade de validação em runtime | §6.5, §9.2 | `BUF-09` fixa a autoridade **no repositório**; a escolha de registry de runtime é `ADR-DMPF-N`, e a operação é de FND-06 | `encaminhada` |
| 4 | `uow-inbox-outbox.md:159`, `:1344` | Algoritmo, canonicalização, escopo dos bytes e versionamento do `payload_hash` | §4.3 | `ENV-17` a `ENV-20`: fórmula decidida sob H1 e H2 de FND-04 `INB-13`, com duas alternativas avaliadas. **H2 é satisfeita pela própria fórmula; H1 é satisfeita sob condição** — a de que os bytes cheguem inalterados —, sustentada por `BLK-03` e `ENV-24` e dependente da byte-preservação por transporte, que é de FND-06 | `quitada`, com H1 condicionada |
| 5 | `uow-inbox-outbox.md:620`, `:648` | Política de `schema_version` | §4.4 | `ENV-21` a `ENV-23`: o que a versão designa, como se relaciona com `dataschema` e quando muda | `quitada` |
| 6 | `uow-inbox-outbox.md:619`, `:648`, `:968` | Vocabulário de `message_type` — nome qualificado do contrato | §4.1, §5.1 | `PTB-01` a `PTB-03` fixam a forma do nome; §4.1 declara a autoridade do campo | `quitada` |
| 7 | `upr-decision-mensagens.md:907-913`; `uow-inbox-outbox.md:389` | Formato do integration event | §3, §5.1, §5.2 | `ENV-04`, `ENV-05` e as regras `PTB-*` de shape e evolução | `quitada` |
| 8 | `upr-decision-mensagens.md:907-913`; `uow-inbox-outbox.md:389` | Assinatura concreta do mapeamento, no nível do contrato | §2.2 | `ENV-04` a `ENV-06`: os dois lados de wire da forma canônica de FND-03 §6.3. O **bloco** de residência permanece de FND-04 §2.2 / `ADR-DMPF-L` | `quitada` |
| 9 | `upr-decision-mensagens.md:785`, `:923`, `:941`; `uow-inbox-outbox.md:389` | Namespace e versionamento no wire; versionamento maior, compatibilidade entre versões vizinhas e depreciação com prazo | §5.1, §5.3, §5.4 | `PTB-01` a `PTB-04` (nome e major), `PTB-08` a `PTB-12` (compatibilidade), `PTB-13` a `PTB-16` (depreciação com prazo e rito) | `quitada` |
| 10 | `uow-inbox-outbox.md:1609` | Preservação do envelope na contenção: byte-idêntica ou semanticamente equivalente | §4.5 | `ENV-24` e `ENV-25` decidem o critério e nomeiam o que a contenção não pode alterar | `quitada` |

`normativo` — **Nove obrigações são quitadas e uma é encaminhada.** A obrigação
3 é a única que este artefato não decide, e a razão é estrutural: a ANC-03
**exige** ADR para a escolha de registry (RFC §12.3, confirmado por RFC §13.3),
e antecipar a escolha aqui esvaziaria o ADR que a própria âncora manda acionar.
O que este artefato faz, e que é o máximo compatível com M1, é fixar a autoridade
que **não** depende da escolha: a validação no repositório de contratos é de Buf,
é local e é fail-closed (§6.5, `BUF-09`). Se um registry entrar em runtime, ele
acrescenta uma segunda autoridade; não substitui a primeira.

`normativo` — **Declarar fronteira não quita obrigação.** Uma linha desta matriz
cujo estado seja `quitada` mas cuja seção apenas nomeie a dona de outro assunto é
defeito de rastreabilidade, e a correção é do artefato — não do estado. É o teste
que a coluna «Como conferir» existe para permitir: um revisor lendo apenas este
documento consegue ir à seção e encontrar a regra, ou não consegue.

`rationale` — A matriz é atômica de propósito. A tentação era agrupar as quatro
delegações que a tabela de FND-04 §1.4 lista numa única linha — «envelope, codec,
registry e `payload_hash`» —, do jeito que a fonte as escreveu. O agrupamento
esconde exatamente o caso interessante: três dos quatro itens são decididos aqui
e um não é, e numa linha só o estado da linha teria de ser inventado. Quebrar por
obrigação torna o parcial visível.

---

## §3. O perfil CloudEvents

O envelope é a parte do contrato que **todo** consumidor lê, inclusive o que não
conhece o payload. Um roteador que decide destino, um coletor que correlaciona
traços e uma ferramenta de operação que inspeciona uma DLQ precisam dos mesmos
atributos, e nenhum deles deveria precisar desserializar o corpo da mensagem
para obtê-los. É isso que o perfil fixa: quais atributos existem sempre, o que
cada um significa e como eles são representados no wire.

### §3.1 A base é o formato Protobuf oficial

`normativo` — bloco `contract package`; consolida Parte-1 §7.3.

| ID | Regra |
|----|-------|
| `ENV-07` | O envelope assíncrono do DMPF é o `io.cloudevents.v1.CloudEvent` do formato Protobuf oficial do CloudEvents, com um **perfil organizacional de validação** aplicado sobre ele. Envelope proprietário, ainda que equivalente em conteúdo, não é conforme |

`recepcionado` — A especificação CloudEvents declara **quatro** atributos de
contexto como REQUIRED: `id`, `source`, `specversion` e `type`. Os demais
atributos de contexto são OPTIONAL na especificação, e extensões seguem as
mesmas convenções de nome e o mesmo sistema de tipos dos atributos padrão.

`normativo` — O perfil do DMPF **acrescenta obrigatoriedade**; ele não remove
nenhuma da especificação e não altera a semântica de nenhum atributo. Um evento
conforme a este perfil é, por construção, um CloudEvent válido — a recíproca não
vale, e é justamente essa assimetria que faz do perfil um perfil.

`rationale` — Adotar o formato oficial em vez de desenhar um envelope próprio é a
decisão que a Parte-1 §7.3 já recomendava, e a razão continua valendo: o formato
oficial traz o mapeamento para cada binding de transporte, as bibliotecas das
duas stacks e o vocabulário que ferramentas de terceiros reconhecem. Um envelope
proprietário obrigaria a plataforma a manter, para sempre, o codec de envelope
como código de infraestrutura crítico — e a primeira necessidade de integração
com um sistema externo o converteria de volta, com perda.

### §3.2 Os atributos do perfil

`normativo` — bloco `contract package`; consolida Parte-1 §7.3.

| ID | Regra |
|----|-------|
| `ENV-08` | Todo evento de integração publicado no DMPF carrega os atributos abaixo com a força declarada na coluna **Presença**. Um evento que omita atributo `obrigatório`, ou que traga atributo `condicional` sem que o predicado seja satisfeito, é **inválido** |

| Atributo | Tipo CloudEvents | Presença | Finalidade |
|----------|------------------|----------|------------|
| `id` | `String` | obrigatório | Identificador da mensagem, único no escopo do `source`; é a chave de deduplicação a jusante (FND-04 `INB-01`) |
| `source` | `URI-reference` | obrigatório | URI estável do produtor; absoluta, e derivada do bounded context — nunca do host, do pod nem do ambiente |
| `specversion` | `String` | obrigatório | Versão da especificação CloudEvents; `1.0` nesta major do perfil |
| `type` | `String` | obrigatório | Nome qualificado do contrato do evento, na forma de `PTB-03` |
| `subject` | `String` | obrigatório | Recurso ou agregado a que o fato se refere |
| `time` | `Timestamp` | obrigatório | Instante do **fato de negócio**, não o da publicação |
| `dataschema` | `URI` | obrigatório | Referência ao schema publicado do payload (§4.4) |
| `datacontenttype` | `String` | obrigatório | `application/protobuf` nesta major (§4.2) |
| `correlationid` | `String` | obrigatório | Correlaciona toda a cadeia originada de uma mesma requisição |
| `causationid` | `String` | obrigatório | `id` da mensagem que causou esta; igual a `id` quando a mensagem inicia a cadeia |
| `partitionkey` | `String` | obrigatório | Chave lógica de ordenação e distribuição |
| `aggregateversion` | `Integer` | condicional — obrigatório quando o fato deriva de agregado versionado; ausente quando não deriva | Detecção de ordem e de concorrência (FND-04 §4.1) |
| `tenantid` | `String` | condicional — §3.4 | Isolamento multi-tenant |
| `traceparent` | `String` | obrigatório | Contexto de propagação W3C Trace Context |
| `tracestate` | `String` | condicional — presente quando houver estado de vendor a propagar; ausente quando não houver | Estado adicional de propagação W3C |

`normativo` — **`time` é o instante do fato, e essa distinção é normativa.** Um
produtor que preencha `time` com o momento da serialização ou da publicação
destrói a única informação temporal que o consumidor não consegue reconstruir. O
instante da publicação, quando necessário, é observação de transporte e é
`encaminhado` a FND-06.

`normativo` — **`causationid` não é opcional na ausência de causa.** Uma mensagem
que inicia a cadeia carrega `causationid` igual ao próprio `id`. A alternativa —
omitir o atributo — obrigaria todo consumidor a tratar dois casos onde há um, e a
ausência ficaria indistinguível de um produtor que esqueceu de preencher.

`normativo` `ENV-09` — **Representação no wire.** Os atributos declarados como
campos próprios da mensagem `CloudEvent` do formato oficial — `id`, `source`,
`spec_version`, `type` — são carregados nesses campos. Todos os demais atributos
deste perfil, incluindo as extensões organizacionais de §3.3, são carregados no
mapa de atributos do formato oficial, com o tipo de `CloudEventAttributeValue`
correspondente ao tipo CloudEvents declarado na tabela acima. Carregar um
atributo do perfil dentro do payload, em vez do envelope, **não** satisfaz
`ENV-08`.

`rationale` — `ENV-09` existe por causa de um requisito não funcional explícito
da spec desta entrega: os atributos obrigatórios são inspecionáveis **sem
desserializar o payload**. Se `correlationid` viaja dentro da mensagem de
negócio, um roteador ou uma ferramenta de operação precisa do schema do payload
— e da versão certa dele — para ler um metadado que não é de negócio. O envelope
existe para tornar isso desnecessário, e a regra é o que impede a erosão do
contrato pela conveniência de «já que estou serializando o corpo, ponho aqui».

### §3.3 Extensões organizacionais

`normativo` — bloco `contract package`.

Sete atributos deste perfil não são atributos de contexto padrão do CloudEvents.
Eles se dividem em **duas** categorias, e a distinção é normativa porque o DMPF
não tem autoridade sobre a primeira:

| Categoria | Atributos | Quem fixa nome, tipo e semântica |
|-----------|-----------|----------------------------------|
| **Extensões CloudEvents adotadas** | `partitionkey` (extensão *Partitioning*); `traceparent` e `tracestate` (extensão *Distributed Tracing*) | A especificação CloudEvents. O DMPF **adota** e pode restringir a presença; não redefine nome, tipo nem significado |
| **Extensões organizacionais do DMPF** | `correlationid`, `causationid`, `aggregateversion`, `tenantid` | Este artefato |

| ID | Regra |
|----|-------|
| `ENV-10` | O nome de uma extensão **organizacional do DMPF** obedece às convenções de nome de atributo do CloudEvents: apenas letras ASCII minúsculas (`a`–`z`) e dígitos (`0`–`9`), iniciando por letra, com um a vinte caracteres. Sem separador — nem `_`, nem `-`, nem `.` — e sem maiúscula. O nome `data` é reservado pela especificação e não é utilizável. Para as extensões CloudEvents adotadas, o nome é o da extensão oficial e não é escolha do DMPF |
| `ENV-11` | O conjunto de extensões deste perfil é **fechado**: as sete acima, nas duas categorias. Uma extensão nova entra por alteração deste artefato, com categoria e presença declaradas e finalidade registrada — nunca por acordo entre produtor e consumidor. Adotar outra extensão oficial do CloudEvents também exige alteração deste artefato: estar padronizada não a torna automaticamente parte do perfil |

`normativo` — Os quatro nomes organizacionais já satisfazem `ENV-10`, e a
conferência é mecânica: `aggregateversion` tem dezesseis caracteres, que é o mais
longo do conjunto, e nenhum usa separador. Uma extensão organizacional futura cujo
nome natural exceda o limite — `businesstransactionid`, com vinte e um — precisa
ser encurtada, não excepcionada. Os três nomes adotados do CloudEvents também o
satisfazem, mas isso é consequência da especificação de origem, não escolha deste
perfil.

`rationale` — **Por que o conjunto é fechado.** A alternativa era declarar as
sete como mínimo e permitir extensões livres por contexto. Ela é mais flexível e
tem um custo assimétrico: extensão livre é invisível para quem consome, porque o
envelope não declara schema das suas próprias extensões — o mapa de atributos
aceita qualquer chave. O resultado observável é um envelope que passa em toda
validação e transporta convenções locais que ninguém catalogou, exatamente o
mesmo defeito que `ENV-05` veta no payload. Fechar o conjunto mantém o envelope
como contrato, e não como saco de metadados.

`rationale` — **Por que `traceparent` e `tracestate` são extensões adotadas, e não
campos de negócio nem invenção local.** A propagação de contexto é matéria de
FND-07, e a telemetria derivada dela é de FND-08. O que este artefato faz é
**adotar** a extensão oficial *Distributed Tracing*, que já fixa os dois nomes e o
formato do W3C Trace Context, em vez de inventar um campo próprio em cada
contrato. Note a assimetria de força: na extensão oficial `traceparent` é
obrigatório e `tracestate` é opcional, e `ENV-08` mantém exatamente essa relação —
adotar sem redefinir. O conteúdo, a política de amostragem e o que é feito do traço
são das donas nomeadas em §1.4.

### §3.4 `tenantid` é condicional, com predicado decidível

`normativo` — bloco `contract package`.

Duas fontes descrevem o atributo em termos próximos: a Parte-1 §7.3 fala de
«tenant validado, quando aplicável», e a spec desta entrega, de isolamento
multi-tenant «obrigatório após autenticação». As duas formulações estão corretas e
nenhuma é diretamente verificável num contrato: «quando aplicável» e «após
autenticação» são propriedades do fluxo, não do envelope. O perfil as converte em
predicado:

| ID | Regra |
|----|-------|
| `ENV-12` | `tenantid` é **obrigatório** em todo evento cuja cadeia de causação tenha origem em uma requisição autenticada, e o seu valor é a identidade de tenant **resolvida** por essa autenticação. É **ausente** — não vazio, não `unknown`, não `default` — em evento de cadeia sem autenticação: rotina de sistema, migração ou processo de plataforma sem sujeito |

`normativo` — A **representação** do atributo está sempre disponível no envelope:
`ENV-12` decide a presença do valor, não a existência do lugar. Um consumidor
multi-tenant que encontre `tenantid` ausente sabe que está diante de um evento de
plataforma, e essa é informação útil; um consumidor que encontre `tenantid` com
valor de preenchimento não sabe nada.

`normativo` — Um valor de `tenantid` **derivado do payload** ou informado pelo
cliente sem validação não satisfaz `ENV-12`. O atributo carrega identidade
resolvida.

`encaminhado` — Como a autenticação resolve a identidade de tenant, o que conta
como requisição autenticada e o controle de acesso ao dado por tenant são de
FND-07 ([ARQ-444](https://lider-cap.atlassian.net/browse/ARQ-444)). Este artefato
fixa a obrigatoriedade condicional e a vedação ao valor de preenchimento; não
fixa o mecanismo.

`rationale` — A vedação ao valor de preenchimento é a parte que resolve o problema
real. Sem ela, a leitura natural de «obrigatório» produz o pior resultado
possível: todo evento carrega `tenantid`, os de plataforma carregam `system` ou
`default`, e o filtro por tenant de um consumidor passa a incluir uma partição
que não é de ninguém. O erro só aparece quando alguém audita isolamento, e nesse
momento a distinção entre «não tem tenant» e «tem o tenant `default`» já foi
perdida em todo o histórico.

### §3.5 O que o contrato exige da validação

`normativo` — bloco `contract package`.

| ID | Regra |
|----|-------|
| `ENV-13` | A conformidade de um envelope a `ENV-08`, `ENV-10` e `ENV-12` é verificável **sem** desserializar o payload e **sem** acesso a rede: os atributos, os seus tipos e os predicados de presença estão todos no envelope |

`registro` — **Lacuna nomeada: a forma de expressão do perfil.** Este artefato
define o **conteúdo** do perfil — quais atributos, com que tipo e sob que predicado
— e a propriedade de `ENV-13` que torna a conferência possível. Ele **não** fixa o
artefato que expressa o perfil de modo executável no repositório de contratos: um
schema de validação declarativo sobre o envelope, um conjunto de asserções, ou um
validador dedicado. Os quatro gates de §6.6 verificam os `.proto` dos **payloads**;
a conformidade de uma **instância** de envelope é de runtime, e nenhum gate deste
artefato a alcança. A lacuna é a pendência 9 de §10.4, e é distinta do bloco
`encaminhado` abaixo: aqui falta decidir **em que forma** o perfil vive; lá, **onde**
a validação roda.

`encaminhado` — **Onde** a validação roda — no adapter de entrada, no consumidor,
num gateway ou no broker —, qual é o gesto de rejeição em cada transporte e o que
acontece com um envelope inválido são de FND-06
([ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443)), sob ANC-04. FND-04
§6.4 já registra que o envelope inválido é recusado **antes** do espaço das sete
disposições de consumo; este artefato define o que torna um envelope inválido, e
não o que se faz com ele.

`rationale` — `ENV-13` é uma propriedade do perfil, não uma exigência de
implementação, e é o que dá utilidade a todo o resto de §3. Ela é a razão pela
qual `ENV-09` obriga os atributos a viverem no envelope, pela qual `ENV-12` foi
convertido em predicado sobre a cadeia e pela qual `ENV-11` fecha o conjunto de
extensões: as três regras juntas fazem com que «este envelope é conforme» seja
uma pergunta respondível por quem tem apenas os bytes do envelope em mãos.

```text
CloudEvent (io.cloudevents.v1.CloudEvent — formato Protobuf oficial)
├── campos próprios da mensagem
│   ├── id             ← identidade da mensagem (dedup a jusante)
│   ├── source         ← URI do bounded context produtor
│   ├── spec_version   ← "1.0"
│   └── type           ← nome qualificado do contrato (PTB-03)
├── attributes: map<string, CloudEventAttributeValue>
│   ├── subject, time, dataschema, datacontenttype     ← atributos padrão
│   ├── correlationid, causationid, partitionkey       ← extensões, sempre
│   ├── aggregateversion                               ← condicional (agregado)
│   ├── tenantid                                       ← condicional (ENV-12)
│   └── traceparent, tracestate                        ← propagação W3C
└── data (oneof)
    └── modalidade única desta major — decidida em §4.2
```

`registro` — O diagrama é **derivado** de `ENV-08` e `ENV-09`: ele não obriga
nada por si e não substitui as tabelas. Onde diagrama e regra divergirem,
prevalece a regra.

---

## §4. Identidade, payload e versionamento

O envelope de §3 não vive isolado: cada atributo dele tem um correspondente no
mecanismo que FND-04 normatizou, e a mesma informação aparece três vezes — na
outbox que a persiste, no envelope que a transporta e na inbox que a deduplica.
Esta seção fixa a **representação** dessa correspondência, decide a modalidade do
payload, quita a fórmula do `payload_hash` e fecha a política de versão.

### §4.1 Correspondência de campos de wire

`normativo` — bloco `contract package`; **restrita à representação**.

| ID | Regra |
|----|-------|
| `ENV-14` | Cada informação da tabela abaixo tem **uma** autoridade de representação: o campo do envelope indicado. Um consumidor lê o envelope; um campo de outbox ou de inbox que divirja do envelope é defeito do produtor, e o envelope prevalece |

`recepcionado` — Os campos da coluna «outbox» são de FND-04 §4.1; os da coluna
«inbox», de FND-04 §6.1. Este artefato **não** os redefine, não altera a autoria
de nenhum deles e não acrescenta campo a nenhum dos dois schemas.

| Informação | Campo da outbox (FND-04 §4.1) | Atributo do envelope (§3.2) | Campo da inbox (FND-04 §6.1) | Autoridade da representação |
|------------|-------------------------------|------------------------------|------------------------------|------------------------------|
| Identidade da mensagem | `message_id` | `id` | `message_id` | `id` do envelope |
| Contrato da mensagem | `message_type` | `type` | `message_type` | `type` do envelope, na forma de `PTB-03` |
| Versão do contrato | `schema_version` | `dataschema` | — | `type` do envelope (§4.4) |
| Corpo do fato | `payload` | `data` | — (só o hash) | `data` do envelope (§4.2) |
| Detecção de reuso de identificador | — | — | `payload_hash` | recomputado sobre `Any.value` do `data`, nunca transportado (§4.3) |
| Instante do fato | `occurred_at` | `time` | — | `time` do envelope |
| Recurso do fato | `aggregate_id` | `subject` | — | `subject` do envelope |
| Ordem no agregado | `aggregate_version` | `aggregateversion` | — | `aggregateversion` do envelope |
| Chave de ordenação | `partition_key` | `partitionkey` | — | `partitionkey` do envelope |
| Destino lógico | `destination` | **sem correspondente** | — | não é matéria de envelope |

`normativo` — **O envelope não carrega o destino.** `destination` é decisão de
roteamento, e FND-04 `BLK-04` já a fixou como destino **lógico** no
`application service`. Um atributo de envelope que nomeie tópico, fila, ARN ou
fluxo de integração acopla o contrato ao transporte e é inválido por `ENV-11` —
além de exceder ANC-03, porque roteamento é de ANC-04. A tradução do destino
lógico para o endereço concreto continua `encaminhada` a FND-06.

`normativo` — **`aggregate_type` não tem atributo próprio.** O tipo do agregado
já está no nome qualificado do contrato (`PTB-03`), cujo segmento de bounded
context e cujo nome de mensagem o determinam. Duplicá-lo num atributo criaria
duas fontes para o mesmo fato, com a possibilidade de divergirem.

`registro` — **Lacuna nomeada: três atributos obrigatórios do envelope sem campo
dedicado no schema mínimo da outbox.** `correlationid`, `causationid` e
`traceparent` são obrigatórios por `ENV-08`, e o schema mínimo de FND-04 §4.1 não
declara coluna para nenhum dos três — o candidato natural é `metadata`, cuja
autoria é do `application service` e cujo conteúdo FND-04 deixou `encaminhado` a
FND-07. Este artefato **não** resolve a lacuna: normatizar coluna de outbox
excederia a ANC-03 por M4. O que ele faz é registrar que a obrigatoriedade no
envelope implica a existência da informação **antes** da publicação, e encaminhar
a forma de persistir a FND-04. A pendência está em §10.4.

`rationale` — **Por que a tabela é restrita à representação.** A versão anterior
desta seção descrevia também quem preenche cada campo e em que momento — e isso
teria sido inválido por M4 duas vezes: a autoria dos campos da outbox é matéria
de FND-04 §2.3, e o momento da serialização é de FND-04 §2.2 / `ADR-DMPF-L`. A
tabela responde a uma pergunta só, que é de contrato: **onde** cada informação
mora no wire e quem manda quando duas cópias divergem. Quem escreve, quando e
em qual bloco continua sendo de quem já decidiu.

### §4.2 A modalidade de payload da primeira major

`normativo` — bloco `contract package`; consolida Parte-1 §7.3 e decide a
condição que a fonte deixara aberta.

O formato Protobuf oficial oferece **três** modalidades para o corpo do evento,
no `oneof data`: `binary_data`, com bytes opacos; `text_data`, com conteúdo
textual; e `proto_data`, com `google.protobuf.Any`. A Parte-1 §7.3 recomendava a
última «quando a toolchain certificada oferecer suporte homogêneo», mantendo
`binary_data` como fallback, e exigia padronizar **uma** modalidade por major do
envelope.

| ID | Regra |
|----|-------|
| `ENV-15` | A modalidade oficial da **primeira major** do perfil é `proto_data`, com o payload empacotado em `google.protobuf.Any`. `binary_data` e `text_data` **não** são conformes nesta major: nenhum dos dois é fallback, exceção por contexto ou escolha do produtor |
| `ENV-16` | `datacontenttype` é `application/protobuf` e `dataschema` é o *type URL* do `Any`, que carrega o nome proto totalmente qualificado da mensagem do payload — forma de `PTB-01`, não de `PTB-03`. Duas conferências são exigidas, e elas são distintas: (a) *type URL* do `Any` **igual** a `dataschema`, comparação literal; (b) `dataschema` e `type` **identificando o mesmo contrato**, o que se verifica pela concordância de major nos termos de `ENV-22` — nunca por igualdade de string, porque as duas formas são estruturalmente diferentes |

`normativo` — A condição da Parte-1 é satisfeita e verificável: a toolchain
certificada da plataforma são as bibliotecas canônicas de Protobuf para Go e para
TypeScript, e ambas suportam `Any` de forma homogênea. Uma stack cuja biblioteca
não suporte `Any` **não** ganha licença para usar `binary_data`: ela é exceção de
toolchain, tratada no rito de §6.3, e não exceção de contrato.

`rationale` — **Por que uma modalidade única, e não duas admitidas.** Admitir as
duas parece conservador e é o oposto: quebra três coisas ao mesmo tempo. O
`payload_hash` de §4.3 passa a ter dois escopos possíveis para os mesmos bytes,
e a comparação entre produtores de modalidades diferentes deixa de ser
significativa; o oráculo de round-trip de §8 precisaria de quatro combinações em
vez de duas; e o consumidor genérico — o que roteia ou inspeciona sem conhecer o
payload — passa a ter de tratar duas formas de extrair o corpo. O custo de fixar
uma é conhecido e recai sobre quem tem toolchain divergente; o custo de admitir
duas recai sobre todos os consumidores, para sempre.

`rationale` — **Por que `proto_data`, e não `binary_data` nem `text_data`.**
`binary_data` foi avaliado e tem uma vantagem real: bytes opacos são compatíveis
com qualquer biblioteca, inclusive a que não implementa `Any`. Foi descartado
porque elimina a autodescrição no ponto em que ela mais importa. `text_data` é
descartado por um motivo anterior e mais simples: ele carrega conteúdo textual, e
o payload de negócio do DMPF é uma mensagem Protobuf — usá-lo exigiria uma
representação textual do contrato, que nenhuma regra deste artefato define e que
reintroduziria o problema de canonicalização que §4.3 evita. Com `Any`, o *type
URL* viaja **dentro** dos bytes do payload, e o par (`dataschema`, *type URL*) é
conferível sem confiança no produtor — é o que `ENV-16` exige. Com
`binary_data`, `dataschema` passa a ser a única afirmação sobre o tipo do corpo,
sem nada que a contradiga quando estiver errada: bytes de um contrato viajando
sob o `dataschema` de outro produzem desserialização silenciosamente errada, que
é exatamente a classe de falha que P0-2 e a política de `reserved` de §5.2
existem para tornar impossível.

`registro` — A escolha desta subseção é o objeto de `ADR-DMPF-M` (§9.2), com a
alternativa descartada acima como insumo obrigatório da redação (RFC §13.2).

### §4.3 O `payload_hash`

`normativo` — bloco `contract package`; quita a obrigação delegada por FND-04
§6.5.

FND-04 `INB-13` fixou **duas propriedades e nenhuma fórmula**, e encaminhou a
este artefato «o algoritmo, a canonicalização, o escopo exato dos bytes e o
versionamento». As duas propriedades são critério eliminatório, não preferência:

| Propriedade (FND-04 `INB-13`) | O que ela exige da fórmula |
|-------------------------------|----------------------------|
| **H1** — estável sob serializações equivalentes | O mesmo conteúdo de negócio produz o mesmo hash em qualquer stack |
| **H2** — restrito ao conteúdo de negócio | Metadados de transporte, tracing e contadores de tentativa ficam fora |

Duas alternativas foram avaliadas contra H1 e H2:

| Alternativa | H1 | H2 | Custo |
|-------------|----|----|-------|
| **A — hash sobre os bytes do payload como transportados**, sem desserializar nem reserializar | Satisfeita **sob condição**: as stacks recebem os mesmos bytes, porque a serialização ocorre uma vez, na escrita (FND-04 `BLK-03`), e a preservação byte-idêntica é obrigatória em toda reentrega e contenção (§4.5) | Satisfeita por escopo: os bytes do payload não contêm atributo de envelope, tracing nem contador | Depende de o transporte não reescrever o corpo; a condição é verificável e é vetor de §8 |
| **B — hash sobre projeção canônica e versionada do conteúdo de negócio** | Satisfeita por construção, inclusive sob reserialização | Satisfeita se a projeção excluir os atributos | Exige definir, por contrato, a ordem dos campos, o encoding de cada tipo e o tratamento de ausência — e Protobuf não oferece forma canônica normativa entre implementações. A projeção passaria a ser um segundo formato de wire, com o seu próprio risco de divergência entre stacks |

| ID | Regra |
|----|-------|
| `ENV-17` | O `payload_hash` é computado **exclusivamente** sobre os bytes do payload de negócio — o conteúdo de `Any.value` do `data` (§4.2) —, exatamente como transportados. Nenhum atributo do envelope entra no cálculo, e nem o *type URL* do `Any`, que já é conferido por `ENV-16` |
| `ENV-18` | O cálculo ocorre **sem** desserializar e sem reserializar o payload. Recomputar o hash a partir de uma estrutura desserializada não satisfaz `ENV-17`, ainda que o resultado coincida por acidente na stack em que foi testado |
| `ENV-19` | O algoritmo é SHA-256, e o valor é a sua representação hexadecimal em minúsculas. A **versão da fórmula** é `1`, e é propriedade da major do perfil: ela não viaja no envelope, porque `ENV-11` fecha o conjunto de extensões e um hash transportado seria afirmação do produtor sobre bytes que o consumidor já tem |
| `ENV-20` | Mudar qualquer elemento de `ENV-17`, `ENV-18` ou `ENV-19` é mudança **breaking** do perfil e exige nova major dele. Hash calculado sob fórmulas de versões diferentes não é comparável, e comparar é o que a inbox faz |

`normativo` — **A escolha é a alternativa A, com a condição de H1 convertida em
obrigação verificável.** A condição — «as stacks veem os mesmos bytes» — não é
suposição: ela decorre de FND-04 `BLK-03`, que serializa uma única vez na
escrita, e é sustentada por `ENV-24` de §4.5, que proíbe reserializar o payload
em contenção e replay. O que era premissa frágil na fonte passa a ser propriedade
mantida por duas regras e exercitada pelo oráculo 2 de §8.3.

`normativo` — **O que essa escolha desloca, dito sem eufemismo.** FND-04 `INB-13`
pôs H1 na **fórmula**; esta seção a põe na **fórmula mais dois invariantes de
sistema**. A diferença é material e precisa ficar registrada: sob a alternativa A,
H1 vale enquanto os bytes publicados não forem reescritos por ninguém no caminho.
Quem reescreve não é o contrato — é transporte, gateway, proxy ou um produtor que
re-emite a partir da estrutura desserializada. A byte-preservação por transporte é,
portanto, **dependência declarada** deste artefato sobre FND-06, e não uma
propriedade que ele possa garantir sozinho; a matriz de §2.3 registra a obrigação 4
como `quitada, com H1 condicionada` por essa razão, e §10.4 a nomeia como
pendência.

`rationale` — **Por que a alternativa B foi descartada, apesar de satisfazer H1
sem condição.** Ela troca uma dependência verificável por uma dependência
invisível. A projeção canônica precisaria decidir, para cada tipo de campo, como
serializar valores em ponto flutuante, como ordenar entradas de `map`, o que
fazer com campo `optional` ausente e como tratar campo desconhecido — e cada uma
dessas decisões teria de ser implementada de forma idêntica em Go e em
TypeScript, sem que exista uma especificação externa para servir de árbitro. O
resultado seria um segundo formato de wire, não padronizado, no caminho crítico
da deduplicação. A alternativa A tem uma condição que se prova com um teste de
bytes; a B tem uma condição que se prova com uma auditoria de duas
implementações.

`registro` — **Consequência conhecida, e não uma regra adicional.** Se um
transporte ou intermediário reescrever os bytes do payload — reserialização por
gateway, normalização por proxy, conversão de formato —, o hash recomputado
divergirá e uma redelivery legítima será classificada R4 pela inbox de FND-04
§6.4. Isso não é resolvido aqui: o comportamento de cada transporte é de FND-06,
sob ANC-04, e a verificação cross-stack é de FND-09. O que este artefato faz é
nomear a condição em `ENV-18`, torná-la falsificável em §8.3 e apontar as donas.

`encaminhado` — A estratégia de transição da inbox quando a fórmula mudar de
versão — tolerar duas fórmulas durante uma janela, ou expirar a inbox antes de
promover a nova major — é de FND-04, sob ANC-02, porque incide sobre a retenção e
a comparação da inbox. Este artefato fixa que a mudança é breaking (`ENV-20`);
não fixa a transição.

### §4.4 `schema_version`, `dataschema` e a autoridade da versão

`normativo` — bloco `contract package`; quita a obrigação delegada por FND-04
§4.1.

| ID | Regra |
|----|-------|
| `ENV-21` | A versão do contrato de um evento é a **major** do pacote Protobuf que o declara, na forma de `PTB-02`. Não há versão menor de contrato de wire: evolução compatível dentro de uma major não altera a versão (§5.3) |
| `ENV-22` | O `schema_version` da outbox e o `dataschema` do envelope carregam a mesma major que o segmento de versão do `type`. Divergência entre eles é inválida, e a **autoridade é o `type`**: é ele que nomeia o contrato, e os outros dois o repetem |
| `ENV-23` | `dataschema` é **identificador**, não endereço de resolução obrigatória. Um consumidor conforme desserializa o payload sem buscar o schema em rede; resolver o identificador para obter documentação ou descriptor é operação opcional, de outro bloco (`ENV-02`) |

`normativo` — Não existe, nesta major do perfil, atributo de envelope que
carregue versão menor, patch, data de publicação do schema ou revisão de
contrato. A única versão do contrato é a major, e ela aparece três vezes por
redundância deliberada de leitura — nunca como três fontes.

`rationale` — **Por que `schema_version` não ganha semântica própria.** A
tentação era usá-lo para versão menor, dando ao consumidor um sinal de «este
payload tem campos novos». Descartada por dois motivos: o sinal é derivável da
própria mensagem, porque campo novo em major compatível é opcional por `PTB-08`
e a sua ausência é indistinguível de produtor mais antigo; e a versão menor
convida à evolução «compatível na intenção», que é como se introduz
incompatibilidade sem passar pelo gate de §6. Uma major por contrato mantém o
`buf breaking` como único árbitro de compatibilidade.

`rationale` — `ENV-23` fecha a porta que `ENV-02` deixou entreaberta. Um
`dataschema` que **precise** ser resolvido em runtime transforma a
desserialização em operação de rede e coloca o `contract package` em dependência
`io.network`, contra RFC §6.2. A regra não proíbe registry — proíbe que o
contrato dependa dele para ser lido, o que é o que preserva a política do bloco
independentemente da decisão de `ADR-DMPF-N`.

### §4.5 Preservação do envelope na contenção

`normativo` — bloco `contract package`; quita a obrigação delegada por FND-04
§7.4.

FND-04 `GAR-07` exige que a mensagem contida preserve «informação suficiente para
um replay conforme», e encaminhou a este artefato a decisão de **se a preservação
precisa ser byte-idêntica ou apenas semanticamente equivalente**.

| ID | Regra |
|----|-------|
| `ENV-24` | A preservação do payload é **byte-idêntica**: os bytes de `Any.value` do envelope contido são exatamente os publicados. Truncar, reserializar, reordenar campos ou normalizar valores desqualifica o replay, ainda que o conteúdo permaneça semanticamente equivalente |
| `ENV-25` | A preservação dos atributos do envelope é **por valor**, sem reescrita e sem acréscimo: o envelope contido carrega os mesmos atributos, com os mesmos valores, e nenhum atributo de diagnóstico é adicionado a ele. Diagnóstico de contenção vive fora do envelope, no registro que o contém |

`normativo` — A escolha de `ENV-24` não é conservadorismo: ela é **exigida** pela
fórmula de §4.3. Sob `ENV-17`, o hash é dos bytes transportados; um replay que
reserialize produz hash diferente, e a inbox de FND-04 §6.4 classifica a mensagem
replayada como R4 — contida de novo, e nunca aplicada. Equivalência semântica
seria suficiente para um humano ler a mensagem e insuficiente para o mecanismo
reprocessá-la, que é precisamente o que `GAR-07` chama de «perdida com registro».

`normativo` — `ENV-25` é a razão pela qual `ENV-11` fecha o conjunto de
extensões também no caminho de contenção. Anexar `dlqreason`, `attemptcount` ou
`quarantinedat` ao envelope preservado parece útil e viola duas coisas: o
conjunto fechado do perfil e H2 de `INB-13`, porque contador de tentativa dentro
do objeto hasheado é o caso que a própria FND-04 nomeia como «deduplicação que
existe sem funcionar». Sob `ENV-17` o hash não alcança atributos, então o dano
seria menor — mas a regra não depende do escopo do hash para valer, e não deve.

`encaminhado` — A estrutura do registro de contenção, o formato do diagnóstico
anexado, a retenção da DLQ e o rito de replay são de FND-04 §7.4 e de FND-06, sob
ANC-02 e ANC-04. Este artefato decide o que a contenção **não pode fazer com o
objeto de wire**; não decide onde a contenção o guarda.

---

## §5. A política Protobuf

O envelope diz como a mensagem viaja; esta seção diz como o contrato que ela
carrega é nomeado, o que nele nunca muda, o que pode mudar sem quebrar consumidor
e como um contrato é retirado de circulação. As regras aqui são **decidíveis**: 
cada uma admite um par de cenários — um que passa e um que reprova — e §6
transforma a maior parte delas em gate de CI.

`recepcionado` — O escopo desta seção cobre **todas** as categorias de contrato do
repositório — `service`, `command`, `response`, `event` e `reply` —, no nível de
*shape*, nome e evolução. Os usos por transporte de cada categoria seguem
vigentes na Parte-1 §7.1 e são de ANC-04 (§1.5).

### §5.1 Nomenclatura

`normativo` — bloco `contract package`; consolida Parte-1 §7.4.

| ID | Regra |
|----|-------|
| `PTB-01` | O pacote Protobuf tem a forma `<org>.<bounded_context>.<categoria>.<major>`, com `categoria` em `{service, command, response, event, reply}` e `major` na forma `v<N>`. Exemplo: `company.orders.event.v1` |
| `PTB-02` | A versão no pacote é **apenas** a major, sem versão menor, patch ou sufixo de estabilidade. Não existem `v1beta1`, `v1p2` nem `v1.1` no DMPF |
| `PTB-03` | O `type` do envelope (§3.2) tem a forma `<reverse-dns>.<bounded_context>.<fato>.<major>`, com o fato em minúsculas separadas por hífen. Exemplo: `com.company.orders.order-placed.v1`. A major do `type` é a mesma do pacote que declara a mensagem do payload |
| `PTB-04` | Nenhum nome de pacote, de mensagem, de campo ou de `type` carrega transporte, destino físico, ambiente ou tecnologia. Tópico, fila, ARN, `kafka`, `sqs`, `prod` e `staging` não aparecem em nome de contrato |

`recepcionado` — A nomenclatura de mensagens e de campos segue as convenções que
o lint STANDARD verifica mecanicamente (§6.2): mensagem em `PascalCase`, campo em
`snake_case`, enum em `SCREAMING_SNAKE_CASE`. Elas não recebem ID próprio aqui
porque o gate as decide sem julgamento, e duplicá-las em prosa criaria duas
fontes para a mesma obrigação.

`normativo` — Nome de mensagem de evento está no **particípio passado**, como em
FND-03 `MSG-N01` para o nível de domínio: `OrderPlaced`, não `PlaceOrder`. Um
evento nomeado no imperativo é um comando disfarçado, e o erro de nome esconde
erro de modelagem — a regra vale igual nos dois níveis de contrato, e é a única
de `MSG-N*` que este artefato reafirma no wire.

`rationale` — `PTB-02` fecha uma porta que a maioria dos ecossistemas deixa
aberta, e o motivo é o gate. Um sufixo de instabilidade — `v1beta1` — significa,
na prática, «este contrato não está sob `buf breaking`», e a exceção sobrevive ao
beta: o contrato entra em produção, ganha consumidores e permanece formalmente
instável. Ou o contrato é público e é governado, ou não é público. Um contrato em
amadurecimento vive no repositório do time que o propõe, fora do repositório de
contratos, até estabilizar.

`rationale` — `PTB-04` é a mesma regra que Parte-1 §7.4 enunciava como «o destino
físico não faz parte do nome lógico do contrato», estendida a ambiente e
tecnologia. A extensão é necessária porque o defeito reaparece com outra roupa:
um contrato chamado `OrderPlacedKafka` obriga uma migração de transporte a virar
uma major nova, e um `OrderPlacedV2Staging` transforma configuração em contrato.

```protobuf
// contracts/proto/company/orders/event/v1/order_placed.proto
syntax = "proto3";

package company.orders.event.v1;

// Fato: um pedido foi confirmado no bounded context de orders.
// Envelope: type = com.company.orders.order-placed.v1 (PTB-03)
message OrderPlaced {
  string order_id = 1;
  string customer_id = 2;
  int64 total_cents = 3;
  OrderChannel channel = 4;

  reserved 5;
  reserved "legacy_promo_code";
}

enum OrderChannel {
  ORDER_CHANNEL_UNSPECIFIED = 0;
  ORDER_CHANNEL_WEB = 1;
  ORDER_CHANNEL_APP = 2;
}
```

### §5.2 O que nunca muda

`normativo` — bloco `contract package`; consolida Parte-1 §7.5.

| ID | Regra |
|----|-------|
| `PTB-05` | Um número de campo, uma vez publicado, **nunca** é reutilizado para outro campo — nem na mesma major, nem em major posterior do mesmo pacote |
| `PTB-06` | Campo removido tem o **número e o nome** declarados em `reserved`, no mesmo commit da remoção. Reservar apenas o número é insuficiente: um campo novo com o nome antigo confunde leitor humano e ferramenta de geração; reservar apenas o nome não impede o reuso do número |
| `PTB-07` | O tipo e o *label* de um campo publicado não mudam. Trocar `int32` por `int64`, `string` por `bytes`, singular por `repeated` ou remover `optional` de campo cuja presença tenha semântica é **breaking**, ainda quando a codificação no wire seja compatível |

`normativo` — Renomear um campo é, para efeito desta política, **remover e
adicionar**: o nome antigo entra em `reserved` (`PTB-06`) e o campo novo recebe
número novo (`PTB-05`). Não existe renomeação in loco de campo publicado.

`rationale` — `PTB-07` proíbe explicitamente casos que são compatíveis no wire, e
essa é a parte deliberada. `int32` → `int64` preserva os bytes para valores
pequenos e muda o contrato para todo consumidor tipado: a stack que gerou código
com `int32` truncará silenciosamente um valor grande, sem erro de
desserialização. A compatibilidade que interessa a esta política é a do
**consumidor gerado**, não a do decodificador de varint — e é por isso que
`buf breaking` com baseline `FILE` (§6.3) é a categoria adotada.

### §5.3 Compatibilidade dentro da major

`normativo` — bloco `contract package`; consolida Parte-1 §7.5.

| ID | Regra |
|----|-------|
| `PTB-08` | A evolução dentro de uma major é **aditiva**: acrescenta campo, valor de enum ou mensagem. Campo novo é `optional` quando a ausência tiver semântica distinta do valor default, e nunca é obrigatório na prática — um consumidor mais antigo não o preenche |
| `PTB-09` | O primeiro valor de todo enum é `<NOME>_UNSPECIFIED = 0`, e ele significa «não informado» — nunca um valor de negócio válido |
| `PTB-10` | Campos desconhecidos são **preservados** em desserialização e reserialização, nas duas stacks. Uma biblioteca que os descarte não é toolchain conforme (§6.3) |
| `PTB-11` | Um consumidor da versão `N-1` de uma major desserializa mensagem produzida na versão `N` da mesma major sem erro e sem perda dos campos que ele conhece. Essa propriedade é **verificada**, não presumida: o par de vetores está na tabela abaixo |
| `PTB-12` | Duas majors do mesmo contrato **coexistem** durante a migração, como pacotes distintos e artefatos distintos. Uma major nunca é publicada substituindo os arquivos da anterior no mesmo caminho |

`normativo` — **Vetores por regra.** Cada regra desta subseção admite um cenário
que passa e um que reprova. O par é o que a torna verificável; o instrumento que
o executa é de FND-09 (§1.4).

| Regra | Vetor positivo — passa | Vetor negativo — reprova |
|-------|------------------------|--------------------------|
| `PTB-08` | Acrescentar `optional string coupon_code = 5;` a `OrderPlaced` na major `v1`; `buf breaking` passa e o consumidor anterior segue lendo | Acrescentar campo reaproveitando o número `3`, hoje de `total_cents`; `buf breaking` reprova por reuso |
| `PTB-09` | `ORDER_CHANNEL_UNSPECIFIED = 0` presente e sem uso de negócio | `ORDER_CHANNEL_WEB = 0`: o valor default do campo passa a ser um canal real, e toda mensagem que omita o campo afirma «web» |
| `PTB-10` | Consumidor `N-1` recebe mensagem `N` com campo novo, reserializa e o campo novo continua nos bytes | Consumidor cuja biblioteca descarta desconhecidos: o replay a partir dele perde o campo, e o `payload_hash` de `ENV-17` divergirá dos bytes originais |
| `PTB-11` | Produtor na `v1` com `coupon_code` preenchido; consumidor gerado antes do campo existir processa a mensagem e ignora o campo | Produtor troca `total_cents` de `int64` para `string`; o consumidor `N-1` falha ou trunca — reprovado por `PTB-07` antes de chegar a produção |
| `PTB-12` | `company.orders.event.v1` e `company.orders.event.v2` publicados como pacotes e artefatos independentes, consumidos em paralelo | `v2` sobrescrevendo os arquivos de `v1` no mesmo diretório: o consumidor de `v1` perde o contrato do qual depende, sem que nenhum gate acuse |

`rationale` — `PTB-10` parece detalhe de biblioteca e é requisito de correção do
mecanismo inteiro. FND-04 `GAR-07` exige que a contenção preserve informação
suficiente para replay conforme, e `ENV-24` decidiu que a preservação é
byte-idêntica. Uma stack que descarte campos desconhecidos rompe a cadeia no
ponto menos visível: onde a mensagem é **re-emitida** a partir da estrutura
desserializada — uma saga que republica, um serviço que reencaminha o fato a
jusante, um produtor que migrou de stack —, os bytes que saem já não contêm o
campo que a biblioteca descartou, e nada acusa a perda. O replay da DLQ **não** é
esse cenário: `ENV-24` obriga a contenção a preservar os bytes recebidos, de modo
que o replay reenvia os originais em vez de reconstruí-los — desde que a captura
para a contenção ocorra antes de qualquer desserialização que descarte
desconhecidos. É por isso que a preservação de desconhecidos é critério de
certificação de toolchain em `BUF-07`, e não recomendação.

### §5.4 Depreciação e janela de suporte

`normativo` — bloco `contract package`; quita a obrigação de «depreciação com
prazo» delegada por FND-03 §6.4.

| ID | Regra |
|----|-------|
| `PTB-13` | Depreciar é um **estado declarado no contrato**, não um aviso em canal externo: o campo, a mensagem ou o pacote depreciado recebe a opção `deprecated = true` no `.proto`, e o commit que a introduz declara, em comentário adjacente, a data de fim de suporte e o contrato sucessor |
| `PTB-14` | A **janela mínima de suporte** de uma major depreciada é de **180 dias** contados da declaração de `PTB-13`. Antes do fim da janela, a major depreciada continua publicada, gerada e coberta pelos gates de §6 |
| `PTB-15` | Remover uma major do repositório exige, cumulativamente: janela de `PTB-14` vencida, e ausência de consumidor declarado. A remoção é um commit próprio, revisável, com a evidência das duas condições no corpo do PR |
| `PTB-16` | Depreciação **não** relaxa nenhuma regra de §5.2 nem os gates de §6: um contrato depreciado continua sujeito a `buf breaking` até ser removido. «Está depreciado» não é justificativa para alteração incompatível |

`normativo` — **Vetores por regra.**

| Regra | Vetor positivo — passa | Vetor negativo — reprova |
|-------|------------------------|--------------------------|
| `PTB-13` | `option deprecated = true;` no pacote `v1`, com comentário declarando fim de suporte e o sucessor `v2` | Anúncio de depreciação apenas em canal de comunicação, sem marca no `.proto`: o consumidor que lê o contrato não vê nada |
| `PTB-14` | Remoção proposta 190 dias após a declaração | Remoção proposta 30 dias após a declaração, com urgência de limpeza como justificativa |
| `PTB-15` | PR de remoção com janela vencida e nenhum consumidor declarado, ambas as evidências no corpo | PR de remoção com janela vencida e consumidor declarado ainda ativo: reprovado até que o consumidor migre ou se declare |
| `PTB-16` | Correção de comentário e de documentação na major depreciada | Alteração de tipo de campo na major depreciada, com o argumento de que ninguém deveria mais usá-la |

`normativo` — A **ausência de consumidor declarado** de `PTB-15` é propriedade
verificável no próprio repositório: o `CODEOWNERS` e o registro de consumidores de
§7.2 dizem quem depende de cada major. Consumidor não declarado não bloqueia
remoção — e essa é a consequência deliberada de `REP-04`: quem não se declara não
é notificado nem protegido.

`rationale` — **Por que 180 dias, e por que um número.** Um prazo qualitativo —
«tempo razoável», «após comunicação aos consumidores» — não é decidível e, na
prática, é decidido pela pressa de quem quer remover. O número torna a regra
conferível por qualquer revisor com acesso ao histórico do arquivo, e a escolha
de dois trimestres reflete a granularidade de planejamento observada na
organização: uma major depreciada atravessa dois ciclos antes de sair, o que dá a
um consumidor a chance de encaixar a migração num deles sem tratar como
incidente. Um prazo maior imobilizaria contratos em manutenção dupla por tempo
longo; um menor transformaria depreciação em remoção com aviso.

---

## §6. A governança Buf

As regras de §5 só valem o que a sua verificação vale. Esta seção fixa a
configuração que as verifica, a identidade do baseline contra o qual a
compatibilidade é medida, a máquina de estados que impede o baseline de ser
contornado, e o oráculo que prova que a geração de código é reprodutível.

`normativo` — Tudo em §6 é configuração e política **do repositório de
contratos**. Nenhuma regra aqui obriga um consumidor a rodar Buf no seu próprio
repositório; o que ela obriga é que nenhum contrato entre no repositório de
contratos sem passar pelos gates.

### §6.1 O workspace

`normativo` — bloco `contract package`.

| ID | Regra |
|----|-------|
| `BUF-01` | O repositório de contratos declara um **workspace Buf v2** na raiz, com os módulos explicitamente listados. Configuração `v1` e módulos descobertos por convenção de diretório não são conformes |
| `BUF-02` | Toda dependência externa é declarada em `deps`, e o `buf.lock` correspondente é versionado. Uma dependência resolvida por rede no momento do build, sem entrada no lock, não é conforme. Sem dependência externa — o caso do exemplo de §6.1, com `deps: []` — não há lock a versionar, e a regra é satisfeita por vacuidade; ela passa a obrigar no commit que introduz a primeira `dep` |

```yaml
# contracts/buf.yaml — workspace na raiz do repositório de contratos
version: v2
modules:
  - path: proto
    name: buf.build/company/contracts
lint:
  use:
    - STANDARD
  enum_zero_value_suffix: _UNSPECIFIED
breaking:
  use:
    - FILE
deps: []
```

`rationale` — O workspace explícito é o que torna o repositório auditável por
inspeção. Com descoberta por convenção, «quais módulos existem» é resposta que
depende de rodar a ferramenta; com a lista, é resposta que se lê. O custo é
acrescentar uma linha ao criar um módulo — e esse é justamente o momento em que
alguém deve revisar a criação.

### §6.2 Lint

`normativo` — bloco `contract package`.

| ID | Regra |
|----|-------|
| `BUF-03` | A categoria de lint adotada é **`STANDARD`**, aplicada ao workspace inteiro, com `enum_zero_value_suffix: _UNSPECIFIED`. Desabilitar regra por diretório, por arquivo ou por prefixo de pacote é proibido; uma exceção só é válida se for **nominal**, no formato de RFC §6.4 — par (arquivo, regra), justificativa, owner e data de revisão |

`recepcionado` — `STANDARD` herda `BASIC` e `MINIMAL`, e é a categoria que
verifica mecanicamente as convenções de nome que §5.1 deixou de duplicar em
prosa, além do sufixo do valor zero de enum que `PTB-09` exige.

`rationale` — A vedação a exceção por diretório é a mesma de RFC §6.4, e a razão
é a mesma: exceção por categoria deixa de ser exceção e vira política paralela
não revisada. Em governança de contrato o efeito é pior do que em allowlist de
dependência, porque o diretório excepcionado tende a ser justamente o dos
contratos herdados — os que mais precisam de verificação.

### §6.3 Breaking e a identidade do baseline

`normativo` — bloco `contract package`.

| ID | Regra |
|----|-------|
| `BUF-04` | A categoria de breaking adotada é **`FILE`**, a mais conservadora. `PACKAGE`, `WIRE` e `WIRE_JSON` não são adotadas nesta major do perfil: elas admitem mudanças que quebram o **consumidor gerado** ainda que preservem o wire, que é o caso que `PTB-07` proíbe |
| `BUF-05` | O baseline de comparação tem **identidade declarada e resolvível**: é a referência protegida do repositório de contratos, nomeada na configuração do gate, e o gate falha quando ela não resolve. «Baseline ausente» nunca equivale a «nada a comparar» |
| `BUF-06` | A versão da CLI Buf e a versão **e** a revisão de cada plugin de geração são pinadas em arquivo versionado. Plugin sem pin, ou pin por faixa, não é conforme: a geração deixa de ser reprodutível no momento em que o plugin publica uma versão nova |
| `BUF-07` | Uma stack só é **toolchain certificada** do DMPF se a sua biblioteca de runtime satisfizer, cumulativamente: suporte a `google.protobuf.Any` (`ENV-15`) e preservação de campos desconhecidos em desserialização e reserialização (`PTB-10`). Stack não certificada não publica nem consome contrato do repositório |

`normativo` — A certificação de `BUF-07` é propriedade da **biblioteca**, não do
time: duas equipes que usem a mesma biblioteca compartilham a certificação, e
trocar de biblioteca exige recertificar. O registro das stacks certificadas vive
no repositório de contratos, junto da configuração de geração.

`rationale` — **Por que `FILE`, e não `PACKAGE`.** A documentação do Buf trata
`PACKAGE` como opção legítima quando a organização comprova que os seus
consumidores dependem apenas do pacote, e a Parte-1 §7.5 registrava essa
possibilidade. Ela é descartada aqui por uma razão factual: os consumidores do
DMPF são código **gerado** em duas stacks, e código gerado depende de arquivo —
mover uma mensagem entre arquivos do mesmo pacote muda os artefatos gerados em Go
e em TypeScript. Adotar `PACKAGE` autorizaria mecanicamente uma mudança que
quebra a compilação de quem consome, e o gate perderia a propriedade que o torna
útil: reprovar antes de o consumidor descobrir.

### §6.4 A máquina de estados do bootstrap

`normativo` — bloco `contract package`.

`BUF-05` exige baseline resolvível, e há exatamente um momento na vida de um
módulo em que não existe baseline: o primeiro. Tratar esse momento com uma
exceção genérica — «se não houver baseline, passe» — transformaria o gate mais
importante do repositório num opcional, porque o mesmo caminho que serve ao
primeiro commit serve a quem apaga o marcador.

| ID | Regra |
|----|-------|
| `BUF-08` | Todo módulo do workspace está, a cada execução do gate, em **um** dos dois estados abaixo, e a transição entre eles ocorre **uma única vez**, no sentido indicado |

| Estado | Como é reconhecido | Efeito no gate |
|--------|--------------------|----------------|
| `sem baseline` | Ausência da **marca de baseline** do módulo — uma referência anotada e protegida do repositório, nomeada segundo o módulo, cuja criação exige autorização de quem não é o autor do commit | `buf breaking` é **dispensado** para esse módulo, e apenas para ele. `buf lint`, `buf format` e a geração continuam obrigatórios |
| `baseline estabelecido` | Presença da marca de baseline do módulo | `buf breaking` é **obrigatório**. Baseline ausente, irresolvível, corrompido ou inacessível **reprova** — fail-closed |

`normativo` — A transição `sem baseline` → `baseline estabelecido` ocorre no
mesmo PR que publica o primeiro conteúdo do módulo, e é **irreversível**: não
existe transição de volta. Um módulo cuja marca de baseline seja removida está em
estado inválido, não em `sem baseline`.

`normativo` — **Vetores negativos.** Os três cenários abaixo reprovam, e é a
recusa deles que dá conteúdo a `BUF-08`:

| Cenário | Por que reprova |
|---------|-----------------|
| Remoção da marca de baseline de um módulo publicado, seguida de alteração incompatível | A ausência da marca num módulo com histórico é estado inválido, não `sem baseline`. O gate compara a lista de módulos do workspace com as marcas existentes e reprova a divergência |
| Renomeação do módulo, ou criação de módulo novo que redeclara pacote já publicado, para obter estado `sem baseline` | O estado é do **pacote publicado**, não do caminho do módulo: redeclarar `company.orders.event.v1` sob outro módulo é falsificação de bootstrap e reprova |
| Criação da marca de baseline pelo próprio autor do commit que a exige | A criação da marca requer autorização distinta da autoria, no mesmo modelo de RFC §10.2 e de `ADR-DMPF-D`. Autoria e autorização coincidentes reprovam |

`normativo` — A autorização de `BUF-08` é **fail-closed** na ausência de
evidência: sem registro de quem autorizou, o gate reprova. Não há caminho em que
a falta de informação libere a transição.

`registro` — **Consequência conhecida: o caso do aprovador único.** A exigência de
autorização distinta da autoria — que este artefato herda de RFC §10.2 e de
`ADR-DMPF-D`, e não inventa — tem um efeito degenerado: num repositório com um
único mantenedor, ou quando o primeiro módulo é criado por quem também é o único
aprovador, a transição de `sem baseline` para `baseline estabelecido` fica
**impossível**, e o módulo não pode ser publicado. Isso não é defeito da regra: é o
fail-closed operando como projetado, e relaxá-lo aqui reabriria exatamente o vetor
de falsificação que §6.4 fecha. O que este artefato registra é que a condição de
possibilidade da regra — existir mais de uma pessoa com autoridade sobre o
repositório de contratos — é **pré-requisito organizacional**, não detalhe de
implementação: um repositório de contratos com um único aprovador não satisfaz
`BUF-08` e não deve ser criado antes de resolver isso. Quem monta o repositório
verifica esse pré-requisito antes do primeiro módulo, e não depois.

`rationale` — A máquina de estados existe porque a formulação anterior — «no
primeiro commit, permita bootstrap» — é indistinguível, em runtime, de «sempre
que o baseline não resolver, permita». O gate não sabe se é o primeiro commit; ele
sabe se a marca existe. Amarrar a decisão a uma referência protegida cuja criação
exige autorização de terceiro converte uma condição temporal, que ninguém pode
verificar depois, numa condição de estado que qualquer revisor confere.

### §6.5 A autoridade da validação

`normativo` — bloco `contract package`.

| ID | Regra |
|----|-------|
| `BUF-09` | A autoridade de validação de contrato **no repositório de contratos** é o Buf, executado nos gates de §6.6: a verificação é **local** — opera sobre os arquivos e os descriptors do repositório —, é obrigatória e é fail-closed. Nenhum contrato entra no repositório validado apenas por um serviço externo |

`encaminhado` — **Se existe uma segunda autoridade em runtime — um Schema
Registry — e qual — é decisão de `ADR-DMPF-N` (§9.2), e a sua operação é de
FND-06** ([ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443)), sob ANC-04.
Este artefato não escolhe registry: a ANC-03 exige ADR para essa escolha
(RFC §12.3, confirmado por RFC §13.3), e antecipá-la aqui esvaziaria o ADR que a
própria âncora manda acionar. É a obrigação 3 da matriz de §2.3, e é a única
`encaminhada`.

`registro` — O que a decisão de `ADR-DMPF-N` **não** pode alterar, porque decorre
de invariante da âncora e não de preferência: `ENV-02` e `ENV-23` proíbem que a
desserialização dependa de resolução remota de schema. Um registry adotado em
runtime acrescenta verificação e catalogação; ele não se torna pré-requisito de
leitura do contrato, porque isso colocaria `io.network` dentro de um bloco cuja
política de RFC §6.2 permite apenas `pure` e `wire.codec`.

`rationale` — Separar as duas autoridades resolve a pergunta que o estudo interno
sobre Schema Registry levantou: validação broker-side verifica identificador e
formato de wire, e **não** valida o payload contra o schema. Uma organização que
confie apenas nela fica com contratos verificados no envelope e livres no corpo.
Fixar a autoridade no repositório — antes da publicação, sobre os arquivos —
garante a verificação que importa, independentemente do que o ADR decidir sobre
runtime.

### §6.6 Geração determinística e os gates

`normativo` — bloco `contract package`.

| ID | Regra |
|----|-------|
| `BUF-10` | A geração é declarada em `buf.gen.yaml` v2 com **managed mode** habilitado: opções de linguagem — como o pacote Go — são aplicadas na geração, e não escritas nos arquivos `.proto`. O `.proto` permanece neutro de linguagem |
| `BUF-11` | A geração é **reprodutível**, e a propriedade é verificada por um oráculo: duas gerações consecutivas a partir da mesma entrada, em ambiente limpo, produzem artefatos idênticos byte a byte, **em cada uma das duas stacks**. Diferença entre as duas gerações reprova, e diferença entre o artefato gerado e o artefato versionado é *drift* e também reprova |
| `BUF-12` | Os gates de §6 **barram o merge** sem depender de julgamento humano: nenhum deles é advisory, nenhum é ignorável por aprovação de revisor e nenhum tem caminho de bypass documentado. Um gate que só emite aviso não satisfaz esta seção |

```yaml
# contracts/buf.gen.yaml
version: v2
clean: true
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/company/contracts/gen/go
plugins:
  # BUF-06: versão e revisão pinadas. Os números abaixo são a forma exigida;
  # o valor efetivo é o resolvido no bootstrap do repositório de contratos.
  - remote: buf.build/protocolbuffers/go:v1.36.11
    revision: 1
    out: gen/go
    opt: paths=source_relative
  - remote: buf.build/bufbuild/es:v2.10.0
    revision: 1
    out: gen/ts
inputs:
  - directory: proto
```

`normativo` — **Os quatro gates.** A sequência abaixo é a ordem em que os gates
rodam, e cada um reprova por conta própria:

```text
1. buf format --diff --exit-code     → estilo; reprova em qualquer diferença
2. buf lint                          → BUF-03 (STANDARD + sufixo de enum)
3. buf breaking --against <baseline> → BUF-04, BUF-05, BUF-08 (fail-closed)
4. buf generate  (2x, ambiente limpo) → BUF-11 (reprodutibilidade e drift)
```

`normativo` — O passo 4 é executado **duas vezes** por stack, e compara três
coisas: a primeira geração com a segunda (reprodutibilidade), e cada uma com o
artefato versionado (*drift*). Um pipeline que gere uma única vez verifica drift e
não verifica reprodutibilidade — e é a reprodutibilidade que sustenta o requisito
não funcional da spec, segundo o qual a mesma entrada produz artefatos idênticos
byte a byte nas duas stacks.

`registro` — **Consequência conhecida: os gates dependem de rede, e `BUF-12` não
admite bypass.** O exemplo acima usa plugins `remote:`, resolvidos no registro de
plugins do Buf. Combinado com `BUF-11`, são quatro resoluções remotas por PR — duas
gerações em cada uma das duas stacks —, e como `BUF-12` proíbe caminho de exceção,
a indisponibilidade do registro de plugins bloqueia o merge de qualquer contrato. A
consequência é deliberada: a alternativa seria um bypass, e um bypass disponível é
um bypass usado. Mas ela tem realização conforme que a reduz, e vale nomeá-la em vez
de deixar quem monta o pipeline descobrir num incidente: plugin **local**, instalado
no ambiente de CI e pinado por versão e revisão, satisfaz `BUF-06` e `BUF-10`
igualmente e não depende de rede na hora do gate. A escolha entre remoto e local é
de quem implementa o pipeline; o que este artefato exige é o pinning, não o
transporte do plugin.

`encaminhado` — Nomes de métrica, limiares e alarmes sobre reprovação de gate,
frequência de drift e falha de validação são de FND-08
([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445)), sob ANC-06. Este
artefato exige que o gate exista, reprove e seja fail-closed; não define o que se
observa sobre ele. O critério é o mesmo que FND-04 §1.4 enunciou: **capacidade é
deste artefato; catálogo é de FND-08.**

`rationale` — `BUF-12` é a regra que a spec desta entrega pediu com todas as
letras — «rodam em CI e barram o merge, sem depender de revisão humana» — e é a
que mais se erode na prática. A erosão tem uma forma característica: o gate nasce
bloqueante, uma entrega urgente encontra uma reprovação legítima, alguém
acrescenta um caminho de exceção «temporário», e o caminho permanece. Enunciar a
ausência de bypass como propriedade normativa não impede a decisão de criar um —
mas a torna uma alteração deste artefato, revisável, em vez de uma linha de
configuração num arquivo de pipeline.

---

## §7. O repositório de contratos

`normativo` — bloco `contract package`; consolida Parte-1 §7.2 **em parte**
(§1.5): a árvore de Protobuf, o ownership e o fluxo são fixados aqui; os
diretórios de OpenAPI e de AsyncAPI aparecem como registrados, não normatizados.

Este artefato **especifica** o skeleton; criar e popular o repositório é dos
épicos de contratos (§1.4).

### §7.1 A árvore

| ID | Regra |
|----|-------|
| `REP-01` | O caminho de um arquivo `.proto` **espelha** o seu pacote: `proto/<org>/<bounded_context>/<categoria>/<major>/<arquivo>.proto`, com os mesmos segmentos de `PTB-01` e na mesma ordem. A major aparece no caminho, e é por isso que duas majors coexistem como diretórios distintos (`PTB-12`) |
| `REP-02` | O código gerado é versionado sob `gen/<stack>/` e **nunca** é editado manualmente. Uma alteração no artefato gerado que não decorra de `buf generate` é *drift* e reprova por `BUF-11` |
| `REP-03` | Todo diretório de bounded context tem owner declarado em `CODEOWNERS`, e o owner é uma equipe — não uma pessoa. Diretório de contrato sem owner não é conforme: um contrato público sem dono não tem quem responda pela sua evolução nem pela sua depreciação |

```text
contracts/
├── buf.yaml                       # workspace v2 (BUF-01)
├── buf.gen.yaml                   # geração com managed mode (BUF-10)
├── buf.lock                       # versionado (BUF-02)
├── CODEOWNERS                     # owner por bounded context (REP-03)
├── CONSUMERS.md                   # consumidores declarados por major (REP-04)
├── proto/
│   └── company/
│       ├── platform/
│       │   └── messaging/v1/      # envelope e tipos compartilhados
│       └── orders/
│           ├── command/v1/
│           ├── event/v1/
│           ├── event/v2/          # coexistência de majors (PTB-12)
│           ├── response/v1/
│           └── service/v1/
├── fixtures/
│   └── orders/event/v1/           # golden fixtures normativas (§8)
├── gen/
│   ├── go/                        # artefatos gerados — nunca editados (REP-02)
│   └── ts/
├── openapi/                       # registrado, não normatizado (REP-06)
└── asyncapi/                      # registrado, não normatizado (REP-06)
```

| ID | Regra |
|----|-------|
| `REP-06` | Os diretórios `openapi/` e `asyncapi/` constam da árvore porque a Parte-1 §7.2 os declarou e porque removê-los da figura sugeriria revogação. Eles **não** são normatizados por este artefato: nada em §5, §6 ou §7 obriga sobre o seu conteúdo, o seu versionamento ou os seus gates. A lacuna está registrada em §10.4 |

`normativo` — A ausência de norma sobre `openapi/` e `asyncapi/` **não** autoriza
tratá-los como área livre: Parte-1 §7.6 e §7.7 seguem vigentes (§1.5) e continuam
sendo a fonte para quem escreve nesses diretórios. O que falta é o artefato
sucessor, não a norma.

### §7.2 Ownership e consumidores declarados

| ID | Regra |
|----|-------|
| `REP-04` | Cada major publicada tem os seus **consumidores declarados** registrados no repositório de contratos, por bounded context consumidor. A declaração é voluntária no sentido de que ninguém a impõe de fora, e é a única forma de ser notificado de depreciação e de bloquear remoção (`PTB-15`) |
| `REP-05` | A revisão de um PR que altere `.proto` exige aprovação do owner do bounded context (`REP-03`), **além** dos gates de §6. Gate verde não substitui revisão de owner, e aprovação de owner não substitui gate |

`normativo` — Consumidor **não** declarado não bloqueia remoção de major
(`PTB-15`) e não é notificado de depreciação. A consequência é deliberada e é o
incentivo que faz `REP-04` funcionar sem imposição: declarar-se é o que compra
proteção.

`rationale` — **Por que o registro de consumidores é do repositório de contratos,
e não inferido.** A alternativa era derivar os consumidores do grafo de
dependências dos artefatos gerados — quem importa o pacote Go ou npm. Ela é
automática e incompleta de um jeito que não se percebe: um consumidor que
desserialize a partir de descriptor, um que consuma via gateway e um que esteja em
repositório privado não aparecem no grafo. Um registro declarado é menos preciso
no papel e mais honesto na prática, porque a sua incompletude é visível: quem não
está na lista sabe que não está.

### §7.3 O fluxo de contribuição

`normativo` — A sequência abaixo é o caminho **único** pelo qual um contrato entra
ou muda no repositório:

```text
1. Proposta        → PR no repositório de contratos, com o .proto e o motivo
2. Gates (§6.6)    → format, lint, breaking, geração 2x — todos bloqueantes
3. Revisão         → owner do bounded context (REP-03, REP-05)
4. Fixture (§8)    → contrato de evento novo acompanha golden fixture
5. Publicação      → merge; artefatos gerados versionados no mesmo commit
```

`normativo` — Não existe caminho de publicação que dispense o passo 2. Commit
direto na referência protegida, geração local publicada sem PR e alteração de
artefato em `gen/` sem alteração correspondente em `proto/` são todos não
conformes.

`encaminhado` — O endereço concreto onde cada contrato é publicado para consumo —
registro de módulos, pacotes npm, módulos Go, distribuição por transporte — é de
FND-06 e dos épicos de contratos. Este artefato fixa o fluxo dentro do
repositório; não fixa a distribuição para fora dele.

---

## §8. A golden fixture de round-trip

A spec desta entrega pede a evidência de que «um contrato de exemplo serializado
numa stack é desserializado na outra sem perda semântica». Esta seção especifica
**o cenário e o que ele deve provar**; executá-lo é de outros — e a separação é o
que impede que a prova se transforme numa afirmação sobre bytes que ninguém
verificou.

### §8.1 O que a fixture é

`normativo` — bloco `contract package`.

| ID | Regra |
|----|-------|
| `INT-01` | Uma **golden fixture** é um artefato versionado no repositório de contratos que descreve, de forma completa e independente de stack, uma instância do contrato: todos os atributos de envelope de `ENV-08` com valores fixos, e o conteúdo do payload campo a campo. Ela é a **fonte única** das duas stacks — nenhuma stack mantém a sua própria cópia |
| `INT-02` | Todo contrato de categoria `event` publicado no repositório tem ao menos uma golden fixture, e a fixture acompanha o PR que publica ou altera o contrato (§7.3, passo 4) |
| `INT-03` | O cenário de round-trip é **bidirecional**: Go produz e TypeScript consome, e TypeScript produz e Go consome. As duas direções são exigidas, e passar em uma não dispensa a outra |

`rationale` — `INT-03` reproduz uma decisão que a spec de FND-09 já tomara pelo
lado do teste, e a razão vale igual no lado do contrato: assimetrias de
serialização de campo opcional e de valor default aparecem **apenas** no sentido
não testado. Uma stack que omita campo com valor default e outra que o emita
produzem bytes diferentes para o mesmo conteúdo, e o defeito é invisível enquanto
só uma delas produzir.

### §8.2 O contrato de exemplo

`registro` — O contrato de `OrderPlaced` de §5.1 é o exemplo canônico, e a
fixture correspondente cobre: os quinze atributos de `ENV-08`, incluindo os
**três** condicionais — `aggregateversion`, `tenantid` e `tracestate` — em ambos
os estados do predicado, e os quatro campos do payload, incluindo o enum com o
valor `UNSPECIFIED` e um campo `reserved` ausente.

```text
fixtures/orders/event/v1/order-placed.golden
├── envelope
│   ├── id, source, spec_version, type          (campos próprios)
│   ├── subject, time, dataschema,              (atributos padrão)
│   │   datacontenttype
│   ├── correlationid, causationid,             (extensões, sempre)
│   │   partitionkey, traceparent
│   ├── aggregateversion                        (dois casos: presente e ausente)
│   ├── tenantid                                (dois casos: presente e ausente)
│   └── tracestate                              (dois casos: presente e ausente)
└── payload  (company.orders.event.v1.OrderPlaced)
    ├── order_id, customer_id, total_cents
    └── channel = ORDER_CHANNEL_WEB
```

`normativo` — A fixture inclui, obrigatoriamente, **um caso presente e um caso
ausente para cada um dos três atributos condicionais** de `ENV-08` — o predicado
de `tenantid` está em `ENV-12`, e os de `aggregateversion` e `tracestate`, na
própria tabela de `ENV-08` — e **um caso com campo desconhecido** para exercitar
`PTB-10`. Uma fixture que cubra apenas o caminho felizmente típico não prova as
regras que mais dependem de verificação cross-stack: um condicional sem o par
presente/ausente deixa metade do predicado sem oráculo.

### §8.3 Os três oráculos

`normativo` — bloco `contract package`.

O round-trip não tem **um** critério de sucesso: tem três, e eles são
independentes. Confundi-los é o defeito que esta subseção existe para evitar,
porque cada um falha por motivo diferente e exige correção diferente.

| # | Oráculo | O que ele compara | Onde é exigido |
|---|---------|-------------------|----------------|
| 1 | **Equivalência semântica** | Cada atributo de envelope e cada campo de payload recuperado na stack consumidora é igual ao declarado na fixture | Sempre, nas duas direções |
| 2 | **Igualdade do `payload_hash`** | O hash recomputado por `ENV-17` na stack consumidora é igual ao computado na produtora sobre os mesmos bytes | Sempre, nas duas direções |
| 3 | **Identidade de bytes** | Os bytes de `Any.value` recebidos são idênticos aos publicados | **Apenas** onde `ENV-24` a exige: caminho de publicação sem reserialização, contenção e replay |

| ID | Regra |
|----|-------|
| `INT-04` | Os três oráculos são avaliados e **reportados separadamente**. Um cenário que reprove só no oráculo 3 tem diagnóstico distinto de um que reprove no 1: o primeiro indica reserialização no caminho, o segundo indica divergência de contrato ou de geração |
| `INT-05` | O oráculo 3 **não** é exigido entre produtores independentes da mesma mensagem lógica. Duas stacks que serializem o mesmo conteúdo a partir da fixture podem produzir bytes diferentes sem violar nada deste artefato — porque Protobuf não oferece forma canônica normativa entre implementações. O que elas **não** podem é divergir nos oráculos 1 e 2 sobre os bytes que efetivamente circulam |

`normativo` — `INT-05` é a consequência direta da escolha de §4.3: o hash é sobre
os bytes transportados, e por isso o oráculo 2 compara produtor e consumidor da
**mesma** mensagem, não dois produtores da mesma informação. Exigir identidade de
bytes entre produtores independentes seria exigir de Protobuf uma garantia que ele
não dá, e o teste passaria a reprovar por motivo que nenhuma correção de contrato
resolve.

`rationale` — A separação em três oráculos é o que dá diagnóstico ao round-trip.
Com um critério único — «os bytes conferem» —, toda falha produz a mesma
mensagem, e a causa pode ser um campo renomeado, um plugin de geração
desatualizado, um gateway que reserializa ou uma biblioteca que descarta
desconhecidos. Com três, a combinação de resultados aponta a família da causa
antes de qualquer investigação.

### §8.4 Os três owners

`registro` — Três responsáveis distintos aparecem neste cenário, e a fronteira
entre eles é a de §1.4:

| Owner | Responsabilidade |
|-------|------------------|
| **FND-05** (este artefato) | O contrato de exemplo, o conteúdo obrigatório da fixture (§8.2) e a definição dos três oráculos e do seu escopo (§8.3) |
| **FND-09** ([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)) | O oráculo executável, o formato de arquivo da fixture, o pipeline que a roda e o diagnóstico que ela emite |
| Épicos de kernel e de contratos | A implementação nas duas stacks e a execução efetiva do cenário, com a evidência registrada |

`normativo` — Este artefato **não** declara o round-trip executado. A spec desta
entrega lista «contrato serializado numa stack e desserializado na outra, com
evidência» entre os seus critérios, e o critério permanece **aberto** até que os
épicos de execução o satisfaçam: um artefato normativo não produz evidência de
runtime. §10.2 registra explicitamente quais critérios esta entrega satisfaz e
quais ficam pendentes de execução.

### §8.5 Handoff de realinhamento ao FND-09

`registro` — **Conflito potencial, nomeado antes de existir.** A spec de FND-09
([SPEC-6RQBN98G](../specs/SPEC-6RQBN98G-dmpf-testes-interop.md)) hoje declara,
sobre as golden fixtures, que a fonte única «garante que Go e TS concordem sobre o
mesmo byte», e reivindica para si «formato, ownership e pipeline» delas. Duas
divergências com este artefato decorrem disso:

| # | Divergência | Posição deste artefato |
|---|-------------|------------------------|
| 1 | «Mesmo byte» como critério **do** round-trip | O oráculo de bytes é o terceiro de três, e `INT-05` o restringe: entre produtores independentes ele não é exigível, porque Protobuf não garante forma canônica entre implementações. Exigi-lo em geral colide com H1 de FND-04 `INB-13` |
| 2 | Ownership integral da fixture em FND-09 | §8.4 divide: o **conteúdo obrigatório** da fixture é de contrato (FND-05); o **formato de arquivo**, o oráculo executável e o pipeline são de FND-09 |

`normativo` — As duas divergências **não** são resolvidas aqui. Resolvê-las neste
artefato significaria normatizar estratégia de teste, que é de ANC-07, e por M4 o
excesso seria inválido ainda que tecnicamente correto. Ficam registradas como
handoff, com a posição deste artefato declarada, para que o FND-09 as reconcilie
com a informação completa em mãos. A pendência está em §10.4.

`rationale` — Nomear o conflito antes de ele se materializar é mais barato do que
descobri-lo na integração, e o motivo é a assimetria de custo: se FND-09 escrever
o oráculo assumindo «mesmo byte» em geral, o teste reprovará cenários legítimos e
a correção proposta será relaxar o critério de hash — que é justamente a regra
cuja violação desliga a deduplicação da inbox (FND-04 §6.5, H2). O caminho errado
é mais fácil de percorrer do que de desfazer.

---

## §9. Acionamento de ADRs estruturais

### §9.1 O que este artefato aciona

`recepcionado` — RFC §13.1 define o gesto e este artefato o repete sem alteração:
acionar é **nomear** o ADR, **definir o seu assunto**, **registrar a origem** e
**encaminhar** ao FND-11
([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)), a quem cabem a
redação, a promoção para `docs/adr/` na faixa `010`–`024` e o aceite.

`normativo` — Nenhum ADR é redigido nem aceito aqui. Os dois registros de §9.2
continuam a série que o FND-04 deixou em `ADR-DMPF-L`, e ficam no estado
`acionado`.

`normativo` — Os IDs `ADR-DMPF-M` e `ADR-DMPF-N` são **provisórios**, pela regra
de RFC §13.2: a numeração definitiva na faixa `docs/adr/010`–`024` é atribuída
pelo FND-11 na promoção, e citar um deles por número definitivo antes disso é erro
de rastreabilidade.

`registro` — A ARQ-442 pede, na sua descrição, os identificadores `ADR-004` e
`ADR-005`. Eles não são utilizáveis, pelo mesmo motivo que FND-04 §10.1 registrou
para `ADR-006`–`ADR-009`: os arquivos `docs/adr/004` e `005` já existem e tratam
de outros assuntos. É por isso que RFC §13.1 reserva a faixa `010`–`024` aos ADRs
estruturais do DMPF. `ADR-DMPF-M` corresponde ao que a story chama de ADR-005
(perfil e codec) e `ADR-DMPF-N` acrescenta a escolha de registry, que a story
tratava dentro do mesmo item.

`registro` — FND-04 e FND-05 correm em paralelo no grafo do épico, e sufixos
provisórios podem circular em branches distintas. A reconciliação é do FND-11, que
detém a atribuição; este artefato não estabelece convenção própria de desempate e
segue a série na ordem em que a encontrou.

`normativo` — O acionamento não antecipa a conclusão. Um destes ADRs pode, na
redação, decidir de forma diferente do que este artefato normatiza; nesse caso a
divergência se resolve por nova versão deste artefato, nunca por sobreposição
silenciosa.

### §9.2 Registro de acionamento

`registro` — mesmo formato de RFC §13.2.

| ID provisório | Nome | Assunto | Origem | Destino | Owner | Estado |
|---------------|------|---------|--------|---------|-------|--------|
| `ADR-DMPF-M` | Codec de wire e modalidade de payload da primeira major | Adotar o formato Protobuf oficial do CloudEvents com perfil organizacional de validação, e `proto_data` com `google.protobuf.Any` como modalidade única da primeira major, vedando `binary_data` — com a fórmula do `payload_hash` sobre os bytes transportados como consequência vinculada | §3.1, §4.2, §4.3; exigido pelo registro de ANC-03 («escolha de codec») e confirmado por RFC §13.3 | ARQ-448 | FND-11 | `acionado` |
| `ADR-DMPF-N` | Escolha de registry e autoridade de validação | Decidir se existe, e qual é, o registry de schemas em runtime, dado que a autoridade de validação no repositório de contratos é o Buf, local e fail-closed (`BUF-09`) — e que a desserialização não pode depender de resolução remota (`ENV-02`, `ENV-23`) | §6.5; exigido pelo registro de ANC-03 («escolha de registry») e confirmado por RFC §13.3 | ARQ-448 | FND-11 | `acionado` |

`registro` — Alternativas descartadas e insumos, obrigatórios na redação
(RFC §13.2):

| ID | Alternativas descartadas | Onde estão registradas |
|----|--------------------------|------------------------|
| `ADR-DMPF-M` | Envelope proprietário equivalente em conteúdo; `binary_data` como modalidade oficial; admitir as duas modalidades com escolha por contexto; `payload_hash` sobre projeção canônica e versionada do conteúdo de negócio | §3.1 `rationale`; §4.2 `rationale` (dois blocos); §4.3, tabela de alternativas e `rationale` |
| `ADR-DMPF-N` | Validação broker-side como autoridade única; registry como pré-requisito de desserialização; ausência de qualquer catalogação em runtime | §6.5 `rationale` e `registro`; §2.1 `rationale` (`ENV-02`) |

`registro` — **Insumos externos disponíveis para a redação.** Os dois estudos
internos anexados ao épico ARQ-436 — o de padronização gRPC/Protobuf/Buf e o de
Kafka Schema Registry e validação de esquemas — trazem os trade-offs já
levantados: o primeiro sustenta as decisões de workspace v2, lint `STANDARD`,
breaking `FILE`, managed mode e versionamento AIP-185/AIP-180 que §5 e §6
normatizam; o segundo sustenta a distinção entre validação broker-side, que
verifica identificador e formato de wire e **não** valida payload, e validação
client-side, além dos modos de compatibilidade e das opções de produto. A redação
de `ADR-DMPF-N` depende materialmente do segundo.

### §9.3 Por que estes dois, e não mais

`rationale` — Este artefato toma um número grande de decisões com alternativa
registrada, e apenas duas viram ADR. O critério é o de RFC §13.1: um ADR
estrutural registra decisão que altera **qual bloco conhece qual** — a matriz de
dependências em uso — ou que a própria RFC exige por âncora. As duas de §9.2 estão
na segunda categoria: a ANC-03 as exige nominalmente.

| Decisão | Por que não é ADR |
|---------|-------------------|
| Conjunto fechado de extensões do envelope (`ENV-11`) | Restringe o conteúdo de um contrato; nenhum bloco passa a conhecer outro |
| `tenantid` condicional com predicado (`ENV-12`) | Torna decidível uma obrigação que a spec já enunciava |
| Preservação byte-idêntica na contenção (`ENV-24`) | Decorre da fórmula de §4.3, que é matéria do ADR de codec — não é decisão independente |
| Breaking `FILE` em vez de `PACKAGE` (`BUF-04`) | Escolhe a categoria mais conservadora dentro de uma governança já decidida; não move responsabilidade |
| Máquina de estados do bootstrap (`BUF-08`) | É propriedade interna do gate, no repositório que já era do `contract package` |
| Janela de 180 dias (`PTB-14`) | Fixa um prazo operacional; a obrigação de depreciar com prazo vem de FND-03 §6.4 |
| Três oráculos do round-trip (`INT-04`) | Organiza critérios de verificação; o instrumento é de ANC-07 |

`normativo` — `ADR-DMPF-M` e `ADR-DMPF-N` são acionados porque a **própria ANC-03
os exige** («ADR exigido: Sim — escolha de codec e de registry»),
independentemente do critério acima. Nenhuma outra decisão deste artefato
satisfaz o critério de RFC §13.1: todas alteram o conteúdo da norma dentro de uma
atribuição de blocos que já estava dada.

---

## §10. Rastreabilidade

### §10.1 A cadeia

`recepcionado` — RFC §14.5 fixa a cadeia que liga cada regra à origem e à
verificação. Instanciada para este artefato, com os elos ainda inexistentes
declarados como pendentes:

```text
constraint P0 / RFC / Parte-1 / obrigação delegada / decisão nova
        ↓  declaração de força e sujeito em cada regra (§1.3)
    cláusula normativa deste artefato
        ↓  §10.2
    ID estável da regra — ENV, PTB, BUF, REP, INT
        ↓  §6.6 (gate) ou §8.3 (oráculo)
    reprovação mecânica no repositório de contratos, ou oráculo de round-trip
        ↓  §10.4 — pendente: instrumento executável e evidência de execução
    vetores de FND-09, executados pelos épicos de kernel e de contratos
```

`normativo` — Duas propriedades distinguem esta cadeia da de FND-04. A primeira é
que **parte** das regras daqui tem verificação mecânica já especificada, e a
atribuição precisa importa mais do que a impressão de cobertura total:

| Regras | Quem as decide | Julgamento humano? |
|--------|----------------|--------------------|
| `PTB-01`, `PTB-02`, `PTB-04` (forma do pacote e do nome), `PTB-05` a `PTB-09` (número de campo, `reserved`, tipo, evolução aditiva, valor zero de enum) | os gates de §6.6 — `buf lint` e `buf breaking` | Não |
| `BUF-01` a `BUF-12`; `REP-01` e `REP-02` (caminho espelha pacote; *drift* do gerado) | os gates de §6.6, incluindo a dupla geração | Não |
| `PTB-03` (forma do `type` do envelope), `ENV-08` a `ENV-13` (perfil do envelope) | nenhum gate deste artefato: são propriedades de **instância** de envelope, não de arquivo `.proto` — ver a pendência 9 de §10.4 | Sim, até haver instrumento |
| `PTB-10` (campos desconhecidos preservados) | `BUF-07`, na certificação de toolchain, e o oráculo de §8.3 | Sim, na certificação |
| `PTB-11` (consumidor `N-1` lê `N`) | o oráculo executável de FND-09 | Sim, até FND-09 |
| `PTB-12` a `PTB-16` (coexistência, depreciação, remoção), `REP-03` a `REP-06` (ownership, consumidores, fluxo) | revisão de PR, com evidência exigida no corpo (`PTB-15`, `REP-05`) | Sim, por construção |

A segunda propriedade é que o elo final — execução e evidência — depende de código
que P0-4 mantém fora deste épico, e por isso permanece aberto.

`registro` — A fonte de cada regra é declarada **na seção onde ela é enunciada**:
o rótulo `recepcionado` marca o que vem da Parte-1 ou da RFC, a prosa da subseção
nomeia a obrigação delegada quando é o caso, e a matriz de §2.3 liga cada
obrigação herdada à sua fonte com `arquivo:linha`.

### §10.2 As regras, por família

`registro` — Cada regra `normativo` substantiva deste artefato tem **ID estável**,
marcado no bloco em que ela é enunciada. O ID é o endereço citável da regra: é por
ele que FND-09 nomeia o cenário que a verifica, e é por ele que um artefato futuro
a refere sem depender do número da seção, que pode mudar.

`normativo` — Os IDs são **estáveis**. Uma regra removida não tem o seu ID
reaproveitado, e uma regra reescrita mantém o ID enquanto a obrigação for a mesma.
Mudança de obrigação é regra nova, com ID novo.

`registro` — Nem todo bloco `normativo` recebe ID. Ficam sem ID os blocos que
governam a **leitura do documento** — convenção de referência, classificação de
força, sujeito da norma, precedência da RFC, fronteira de âncora e sucessão —
porque não são obrigações sobre o contrato e não há o que verificar num gate. Eles
obrigam do mesmo jeito.

**`ENV` — envelope, identidade e payload** (§2, §3, §4) — 25 regras

| ID | Objeto da regra | Onde |
|----|-----------------|------|
| `ENV-01` | Conteúdo admissível no `contract package` | §2.1 |
| `ENV-02` | Contrato legível sem I/O | §2.1 |
| `ENV-03` | Tipo gerado é objeto de wire e nada mais | §2.1 |
| `ENV-04` | Produto da conversão: envelope + payload de contrato `event` | §2.2 |
| `ENV-05` | Um contrato por tipo publicado; sem campo genérico de escape | §2.2 |
| `ENV-06` | Conversão não reversível pelo contrato | §2.2 |
| `ENV-07` | Envelope é o formato Protobuf oficial do CloudEvents, com perfil | §3.1 |
| `ENV-08` | Os quinze atributos, com presença declarada | §3.2 |
| `ENV-09` | Representação: atributo vive no envelope, não no payload | §3.2 |
| `ENV-10` | Charset e limite de nome de extensão | §3.3 |
| `ENV-11` | Conjunto de extensões é fechado | §3.3 |
| `ENV-12` | `tenantid` condicional, sem valor de preenchimento | §3.4 |
| `ENV-13` | Conformidade verificável sem desserializar e sem rede | §3.5 |
| `ENV-14` | Autoridade única de representação por informação | §4.1 |
| `ENV-15` | Modalidade oficial: `proto_data` com `Any`; `binary_data` vedado | §4.2 |
| `ENV-16` | Coerência entre `datacontenttype`, `dataschema`, *type URL* e `type` | §4.2 |
| `ENV-17` | Escopo do `payload_hash`: bytes de `Any.value` como transportados | §4.3 |
| `ENV-18` | Cálculo sem desserializar nem reserializar | §4.3 |
| `ENV-19` | Algoritmo SHA-256, hex minúsculo, fórmula versão 1, não transportada | §4.3 |
| `ENV-20` | Mudar a fórmula é breaking do perfil | §4.3 |
| `ENV-21` | Versão do contrato é a major do pacote | §4.4 |
| `ENV-22` | `type` é a autoridade da versão | §4.4 |
| `ENV-23` | `dataschema` é identificador, não endereço a resolver | §4.4 |
| `ENV-24` | Preservação do payload na contenção é byte-idêntica | §4.5 |
| `ENV-25` | Preservação dos atributos por valor, sem acréscimo | §4.5 |

**`PTB` — política Protobuf** (§5) — 16 regras

| ID | Objeto da regra | Onde |
|----|-----------------|------|
| `PTB-01` | Forma do pacote, com categoria e major | §5.1 |
| `PTB-02` | Major-only, sem versão menor nem sufixo de instabilidade | §5.1 |
| `PTB-03` | Forma do `type` do envelope | §5.1 |
| `PTB-04` | Nome sem transporte, destino, ambiente ou tecnologia | §5.1 |
| `PTB-05` | Número de campo nunca reutilizado | §5.2 |
| `PTB-06` | `reserved` de número **e** nome na remoção | §5.2 |
| `PTB-07` | Tipo e *label* de campo publicado não mudam | §5.2 |
| `PTB-08` | Evolução aditiva dentro da major | §5.3 |
| `PTB-09` | Primeiro valor de enum é `_UNSPECIFIED = 0` | §5.3 |
| `PTB-10` | Campos desconhecidos preservados nas duas stacks | §5.3 |
| `PTB-11` | Consumidor `N-1` lê versão `N` da mesma major | §5.3 |
| `PTB-12` | Majors coexistem como pacotes e artefatos distintos | §5.3 |
| `PTB-13` | Depreciação é estado declarado no contrato, com prazo e sucessor | §5.4 |
| `PTB-14` | Janela mínima de suporte de 180 dias | §5.4 |
| `PTB-15` | Remoção exige janela vencida e ausência de consumidor declarado | §5.4 |
| `PTB-16` | Depreciação não relaxa §5.2 nem os gates de §6 | §5.4 |

**`BUF` — governança e gates** (§6) — 12 regras

| ID | Objeto da regra | Onde |
|----|-----------------|------|
| `BUF-01` | Workspace v2 com módulos explícitos | §6.1 |
| `BUF-02` | `buf.lock` versionado; dependência declarada | §6.1 |
| `BUF-03` | Lint `STANDARD`, sem exceção por diretório | §6.2 |
| `BUF-04` | Breaking `FILE` | §6.3 |
| `BUF-05` | Baseline com identidade declarada e resolvível | §6.3 |
| `BUF-06` | Pinning de CLI, versão e revisão de plugin | §6.3 |
| `BUF-07` | Critérios de toolchain certificada | §6.3 |
| `BUF-08` | Máquina de estados do bootstrap, fail-closed | §6.4 |
| `BUF-09` | Autoridade de validação no repositório é o Buf, local | §6.5 |
| `BUF-10` | Managed mode; `.proto` neutro de linguagem | §6.6 |
| `BUF-11` | Geração reprodutível: duas gerações limpas por stack, e drift | §6.6 |
| `BUF-12` | Gates barram merge, sem bypass e sem julgamento humano | §6.6 |

**`REP` — repositório de contratos** (§7) — 6 regras

| ID | Objeto da regra | Onde |
|----|-----------------|------|
| `REP-01` | Caminho espelha o pacote, com major no path | §7.1 |
| `REP-02` | Código gerado versionado e nunca editado à mão | §7.1 |
| `REP-03` | Owner por bounded context em `CODEOWNERS`, e é equipe | §7.1 |
| `REP-04` | Consumidores declarados por major | §7.2 |
| `REP-05` | Revisão de owner **além** dos gates | §7.2 |
| `REP-06` | `openapi/` e `asyncapi/` registrados, não normatizados | §7.1 |

**`INT` — interoperabilidade e fixture** (§8) — 5 regras

| ID | Objeto da regra | Onde |
|----|-----------------|------|
| `INT-01` | Golden fixture como fonte única, independente de stack | §8.1 |
| `INT-02` | Toda categoria `event` tem fixture, no PR que a publica | §8.1 |
| `INT-03` | Round-trip bidirecional obrigatório | §8.1 |
| `INT-04` | Três oráculos avaliados e reportados separadamente | §8.3 |
| `INT-05` | Identidade de bytes não é exigível entre produtores independentes | §8.3 |

### §10.3 Os critérios da spec e as obrigações da âncora, e onde cada um é satisfeito

`registro` — A tabela é auditável sem sair do repositório. A coluna **Estado**
distingue o que esta entrega satisfaz do que permanece aberto: um artefato
normativo não produz evidência de runtime, e marcar como satisfeito um critério
que exige execução seria falsa cobertura.

| # | Critério ou obrigação | Origem | Onde | Estado |
|---|----------------------|--------|------|--------|
| 1 | Perfil organizacional promovido para `docs/dmpf/cloudevents-protobuf-buf.md` | spec, CA | este arquivo, §3 | **satisfeito** na promoção; a **aprovação em PR** é do próprio PR desta entrega |
| 2 | Skeleton do repositório de contratos especificado | spec, CA | §7 | **satisfeito** — árvore, ownership, consumidores declarados e fluxo |
| 3 | Checks Buf descritos de forma executável | spec, CA | §6.1, §6.6 | **satisfeito** — `buf.yaml` e `buf.gen.yaml` v2 completos, e a sequência dos quatro gates |
| 4 | Perfil define atributos obrigatórios: payload, correlation, causation, tracing, tenant e partition key | spec, RF | §3.2, §3.3, §3.4 | **satisfeito** — quinze atributos com presença declarada; `tenantid` com predicado |
| 5 | Modalidade oficial de payload da primeira major, com trade-offs registrados | spec, RF | §4.2 | **satisfeito** — `ENV-15`, com as três alternativas descartadas em `rationale` e o vínculo a `ADR-DMPF-M` |
| 6 | Política Protobuf proíbe reuso de field number e estabelece evolução compatível | spec, RF | §5.2, §5.3 | **satisfeito** — `PTB-05`, `PTB-06` e `PTB-08` a `PTB-12`, com par de vetores por regra |
| 7 | `format`, `lint`, `breaking` e geração determinística especificados como gates de CI | spec, RF | §6.6 | **satisfeito** na especificação; a **existência do pipeline** é dos épicos de contratos |
| 8 | Contrato de exemplo serializado numa stack e desserializado na outra, com evidência | spec, RF | §8 | **aberto** — o cenário, a fixture e os três oráculos estão especificados; a execução e a evidência são dos épicos de kernel e de contratos (§8.4) |
| 9 | Decisão registrada sobre a relação Buf × Schema Registry (autoridade de validação) | **ANC-03** | §6.5, §9.2 | **satisfeito no que é normatizável**: `BUF-09` fixa a autoridade no repositório; a escolha de registry é `ADR-DMPF-N`, `acionado` — é a obrigação 3 de §2.3, `encaminhada` |
| 10 | Ownership do repositório de contratos definido | spec, dependência | §7.1, §7.2 | **satisfeito** — `REP-03` e `REP-04` |
| 11 | Compatibilidade retroativa: consumidor de versão anterior lê a seguinte na mesma major | spec, RNF | §5.3 | **satisfeito na norma, aberto na execução** — `PTB-11`, com par de vetores; a verificação executada é de FND-09, e a spec o mantém `[ ]` |
| 12 | Geração determinística: mesma entrada produz artefatos idênticos byte a byte nas duas stacks | spec, RNF | §6.6 | **satisfeito na norma, aberto na execução** — `BUF-11`, com oráculo de duas gerações; a execução é dos épicos, e a spec o mantém `[ ]` |
| 13 | Verificação automática barra o merge sem depender de revisão humana | spec, RNF | §6.6 | **satisfeito na norma, aberto na execução** — `BUF-12`; a existência do pipeline é dos épicos, e a spec o mantém `[ ]` |
| 14 | Legibilidade do envelope: atributos obrigatórios inspecionáveis sem desserializar o payload | spec, RNF | §3.2, §3.5 | **satisfeito** — `ENV-09` e `ENV-13` |
| 15 | ADRs registrados com contexto, alternativas, trade-offs e consequências | **ANC-03** e RFC §13.2 | §9.2 | **acionados**, não redigidos: dois ADRs no formato de sete colunas de RFC §13.2, com a tabela de alternativas descartadas. Redação e aceite são de FND-11 |

`normativo` — Um critério cuja coluna «Onde» não puder ser conferida por um
revisor lendo apenas este artefato é defeito de rastreabilidade, e a correção é do
artefato — não do critério. Um critério marcado **aberto** não é defeito: é escopo
que P0-4 mantém fora deste épico, e §10.4 registra o destino de cada um.

### §10.4 Pendências abertas

`registro` — Dez elos ficam abertos ao fim desta entrega. Nenhum é contornado no
texto; todos são escalados.

| # | Pendência | Estado | Destino |
|---|-----------|--------|---------|
| 1 | Refletir na tabela de RFC §14.4 a sucessão de Parte-1 §7 declarada em §1.5, preservando §7.6 e §7.7 como vigentes e as parciais de §7.1 e §7.2 | pendente | Revisores desta entrega, no PR; se exigir rito de versão, vira alteração própria da RFC |
| 2 | **O assunto da ANC-03 não é esgotado por esta entrega**: OpenAPI e AsyncAPI constam do registro da âncora e não são normatizados aqui (§1.5, `REP-06`). A condição de fechamento «FND-05 concluída e revisada» é satisfeita no recorte de Protobuf e CloudEvents, e não no de OpenAPI e AsyncAPI | pendente | Decisão de quem fecha a ANC-03: abrir sub-spec própria, alargar o escopo de outra âncora por rito de versão da RFC, ou registrar o recorte como definitivo |
| 3 | Realinhamento com FND-09: «mesmo byte» como critério geral do round-trip, e ownership da fixture (§8.5) | pendente | FND-09 ([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)), com a posição deste artefato declarada em §8.5 |
| 4 | `correlationid`, `causationid` e `traceparent` são obrigatórios no envelope (`ENV-08`) e não têm campo dedicado no schema mínimo da outbox de FND-04 §4.1 (§4.1) | pendente | FND-04 ([ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)), sob ANC-02 — normatizar coluna de outbox excederia esta âncora |
| 5 | Transição da inbox quando a fórmula do `payload_hash` mudar de versão (`ENV-20`) | pendente | FND-04, sob ANC-02 — incide sobre retenção e comparação da inbox |
| 6 | Recolher no glossário de RFC §14.1 os termos de §10.5 | pendente | Próxima versão da RFC que abrir o glossário; até lá, §10.5 é a fonte |
| 7 | Numeração definitiva de `ADR-DMPF-M` e `ADR-DMPF-N` na faixa `docs/adr/010`–`024`, e a redação de ambos | pendente | FND-11 ([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)) |
| 8 | Execução do round-trip nas duas stacks, com evidência registrada, e existência do pipeline que roda os gates de §6.6 | pendente | Épicos de kernel e de contratos (§8.4); instrumento e oráculo executável são de FND-09 |
| 9 | Forma de expressão executável do perfil de validação do envelope, e onde ela vive no repositório de contratos (§3.5) | pendente | Decisão a tomar entre FND-06 (validação por transporte) e FND-09 (instrumento de verificação); este artefato define o conteúdo do perfil, não o seu artefato |
| 10 | Byte-preservação do payload pelos transportes, da qual H1 depende sob a fórmula de §4.3 (`ENV-17`, `ENV-18`) | pendente | FND-06 ([ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443)), sob ANC-04 — é dependência declarada, não propriedade que este artefato garanta |

`normativo` — Enquanto a pendência 1 não for resolvida, a sucessão de §1.5 vale por
autorização da ANC-03, e a divergência com a tabela da RFC é conhecida. A pendência
2 é a mais consequente das dez: ela afeta o **fechamento da própria âncora**, e
ignorá-la produziria uma ANC-03 declarada fechada com dois dos quatro assuntos do
seu registro sem artefato sucessor. A pendência 10 é a de maior efeito operacional:
sob a fórmula de §4.3, um transporte que reescreva os bytes do payload converte
redelivery legítima em R4 na inbox de FND-04 — o modo de falha que H1 existe para
prevenir.

### §10.5 Índice de termos

`registro` — Os dezoito termos que este artefato introduz ou redefine, cada um
ligado à seção que o define. Nenhum consta de RFC §14.1; o recolhimento é a
pendência 6 de §10.4.

| Termo | Definido em | Em RFC §14.1? |
|-------|-------------|----------------|
| perfil organizacional de validação | §3.1 | a recolher |
| extensão organizacional | §3.3 | a recolher |
| presença condicional com predicado decidível | §3.4 | a recolher |
| autoridade de representação | §4.1 | a recolher |
| modalidade de payload | §4.2 | a recolher |
| fórmula do `payload_hash`, e a sua versão | §4.3 | a recolher |
| preservação byte-idêntica | §4.5 | a recolher |
| categoria de contrato | §5.1 | a recolher |
| major-only | §5.1 | a recolher |
| janela de suporte | §5.4 | a recolher |
| marca de baseline | §6.4 | a recolher |
| bootstrap de baseline | §6.4 | a recolher |
| toolchain certificada | §6.3 | a recolher |
| drift de código gerado | §6.6 | a recolher |
| consumidor declarado | §7.2 | a recolher |
| golden fixture | §8.1 | a recolher |
| oráculo de round-trip — os três | §8.3 | a recolher |
| extensão CloudEvents adotada, distinta de organizacional | §3.3 | a recolher |

`normativo` — Um termo desta tabela usado em outro artefato do DMPF com sentido
diferente do daqui é divergência a registrar, não licença de reinterpretação.

### §10.6 Checklist de fronteira

`normativo` — Verificação das quatro regras de monotonicidade de RFC §12.2 e da
fronteira de RFC §1.4, exigida pela âncora ANC-03.

| Regra | Como este artefato a respeita |
|-------|-------------------------------|
| **M1** — detalhar ou restringir dentro do escopo | As 64 regras têm por sujeito o contrato: a forma do envelope, a forma do `.proto`, a governança do repositório e a evolução dos dois — «formato, versionamento e evolução dos contratos dentro do bloco `contract package`», que é o escopo literal da ANC-03. Todas acrescentam especificidade ou restringem: a especificação CloudEvents exige quatro atributos e `ENV-08` exige quinze; a Parte-1 admitia duas modalidades de payload e `ENV-15` admite uma; `FILE` é a categoria de breaking mais conservadora das quatro (`BUF-04`). O índice de §10.2 torna a verificação conferível **regra a regra** |
| **M2** — não relaxar nem reinterpretar invariante da âncora | As invariantes estão tabuladas em §1.1 com o ponto em que cada uma é tratada. P0-2 é reafirmada em `ENV-03` e sustentada por `ENV-05`; as células 6, 12, 24 e 31 são preservadas sem exceção, e nenhuma regra daqui exige que `domain`, `application` ou `port` conheçam tipo de wire; RFC §6.2 é usada como critério de rejeição em `ENV-02` e `ENV-23`. **P0-3** é guardada por alcance próprio (§1.1) e exercitada pelo vetor documental negativo do PR desta entrega |
| **M3** — relaxar P0 exige nova versão da RFC | Nenhuma regra relaxa constraint P0. As decisões novas — modalidade única de payload (§4.2), fórmula do `payload_hash` (§4.3), máquina de estados do bootstrap (§6.4), janela de 180 dias (§5.4), três oráculos (§8.3) — tratam de matéria que a RFC não cobre, ou são mais estritas que a fonte conceitual |
| **M4** — não exceder o escopo da âncora | O que pertence a outra âncora sai marcado `encaminhado`, com dona nomeada e tabulado em §1.4: FND-04 sob ANC-02 (bloco e momento do mapeamento, autoria dos campos da outbox, transição da fórmula na inbox), FND-06 sob ANC-04 (transporte, binding, ACK, registry em operação), FND-07 (proteção do dado, autorização, taxonomia de erros), FND-08 sob ANC-06 (catálogo de métricas), FND-09 sob ANC-07 (oráculo executável e pipeline) e FND-11 (redação dos ADRs). A sucessão de §1.5 é declarada **por subseção**, com duas parciais, justamente para não absorver Parte-1 §7.6 e §7.7 |

`normativo` — A fronteira mais tensionada desta entrega é a de ANC-02, porque o
contrato não é descritível sem nomear os campos que a outbox e a inbox carregam. O
critério que a resolve está enunciado uma única vez, em §1.3, e aplicado em §4.1 e
§4.5: **o sujeito da norma é o contrato; quem escreve, quando e em qual bloco é de
quem já decidiu.** A tabela de §4.1 é restrita à representação por essa razão.

`rationale` — Uma âncora que recorta um bloco declarativo corre um risco que a
ANC-01 não corria: o de normatizar por adjacência tudo o que toca o objeto de
wire — quem o produz, quando, com que garantia e sob qual observação. As duas
defesas contra isso são estruturais e estão no início do texto: a regra do sujeito
da norma (§1.3) e a tabela de fronteiras com dona nomeada (§1.4). Ambas existem
para tornar o excesso visível na revisão, que é onde M4 se aplica na prática.
