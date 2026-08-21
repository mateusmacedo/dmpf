# Testes e interoperabilidade entre stacks — DMPF FND-09

| Campo | Valor |
|-------|-------|
| **Status** | `draft normativo` — promovido para revisão em PR |
| **Adiciona a** | RFC DMPF Foundation v0.1, pela âncora ANC-07 (RFC §12.3) |
| **Owner** | Mateus Macedo Dos Anjos (assignee de [ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)) |
| **Épico** | [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436) — Golden Path para Sistemas Orientados a Domínio e Mensagens |
| **Story** | [ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446) (DMPF-FND-09) |
| **Spec** | [SPEC-6RQBN98G](../specs/SPEC-6RQBN98G-dmpf-testes-interop.md) |
| **Data** | 2026-08-20 |
| **Revisão** | Plataforma e Arquitetura no PR; o ARQ-446 pede **um representante de cada stack — Go e TypeScript**, e esse gate é **externo e não bloqueante** para a promoção deste artefato: ele afere a exequibilidade dos instrumentos nas duas stacks, não a força normativa do texto |

> **O que este documento obriga.** As regras rotuladas `normativo` valem para
> todo trabalho novo do DMPF, na mesma força da RFC à qual elas se adicionam.
> Nem todo bloco aqui obriga: §1.3 define as categorias de conteúdo e a
> fronteira de cada uma. Enquanto o status for `draft normativo`, o documento
> está em revisão; a promoção ocorre no aceite do PR.

---

## §1. Fronteira, herança e ANC-07

Esta seção vem antes de qualquer regra porque um artefato que adiciona a uma
norma compartilhada precisa dizer, primeiro, **até onde** ele pode obrigar. A RFC
já respondeu à pergunta do alcance ao registrar a âncora ANC-07; o que segue é a
leitura dessa autorização, a convenção com que o texto se lê, as categorias de
força que ele emprega e a ordem de prevalência entre ele, a RFC e os artefatos
irmãos.

Há uma razão a mais para começar por aqui, e ela condiciona toda a seção. Esta é
a única sub-spec da série cuja matéria é **prova**, não assunto: as seis
anteriores decidiram o que a UPR é, como a outbox drena, que atributo o envelope
carrega; esta não decide assunto próprio — ela dá a cada regra já decidida um
mecanismo que a torne conferível. §1.4 desenvolve o que essa inversão muda. Aqui
basta registrar que ela é a origem da forma desta seção: a fronteira de FND-09
não separa um bloco de código de outro, e sim o que este artefato **prova** do
que ele **herda** e do que ele **encaminha**.

### §1.1 A autorização: âncora ANC-07

`normativo`

Este artefato **adiciona** à RFC DMPF Foundation v0.1 pela âncora ANC-07
(RFC §12.3). Ele não edita a RFC e não incrementa a versão dela: adição por
âncora dentro do escopo permitido é revisão em PR, sem incremento (RFC §14.2). O
conteúdo vive aqui, e a RFC o alcança pelo endereço estável da âncora (RFC §12.1).

| Campo | Valor, conforme o registro de ANC-07 |
|-------|--------------------------------------|
| Assunto | Testes e interoperabilidade |
| Escopo permitido | Estratégia de testes e critérios de interoperabilidade entre stacks |
| Invariantes intocáveis | RFC §9 (o domínio é executável e testável em memória); RFC §4.5 regra 3 (o teste não reclassifica o código sob teste) |
| Monotonicidade | M1–M4 (RFC §12.2): detalhar e restringir, nunca relaxar, revogar ou reinterpretar |
| Artefato sucessor | Spec e seção própria |
| Condição de fechamento | FND-09 concluída e revisada |
| Impacto de versão na RFC | Nenhum, se dentro do escopo |
| ADR exigido | **Não** — sem a condicional «salvo alteração de invariante»; a análise que sustenta o «não» está em §12 |

`registro` — **A ausência da condicional é literal e tem consequência.** O
registro de ANC-05 e o de ANC-06 fecham o campo «ADR exigido» com «Não, salvo
alteração de invariante»; o de ANC-07 fecha com «Não», sem ressalva
(RFC §12.3). A diferença não é de redação. As duas invariantes de ANC-07 — RFC §9
e RFC §4.5 regra 3 — pertencem à disciplina que outras âncoras e a RFC já fixam
(o critério do domínio em memória é de RFC §9, base da ANC-01; a regra anti-bypass
é de RFC §4.5), e este artefato só pode **respeitá-las**, nunca alterá-las.
Alterar qualquer uma seria mudança de versão da RFC com ADR aceito, por
RFC §12.2 M3 — não adição por esta âncora, que por RFC §12.2 M2 nem sequer pode
reinterpretá-las. A §12 registra a análise em detalhe, no padrão de FND-07 §10.

`normativo` — **As duas invariantes são citadas ao longo do texto, e nenhuma
regra deste artefato as afrouxa.**

| Invariante | Como este artefato a trata | Onde |
|------------|----------------------------|------|
| RFC §9 — domínio executável e testável em memória | Reproduzida e usada como critério de projeto do kit de domínio: nenhum instrumento deste artefato faz o domínio depender de processo externo, rede, banco, broker, disco, relógio de parede ou entropia para ser exercitado, e mock de infraestrutura não o torna conforme (RFC §9.2) | §2, §7 |
| RFC §4.5 regra 3 — o teste não reclassifica o código sob teste | Preservada e **restaurada**: onde um teste de domínio precisa de infraestrutura, o reparo é remover a dependência do código sob teste, não reetiquetar a unidade; a reclassificação eventual recai sobre o teste, jamais sobre o SUT | §2 |

### §1.2 Convenção de referência

`normativo`

Nove documentos de numeração própria e sobreposta participam desta cadeia, então
toda referência é qualificada:

| Forma | Designa |
|-------|---------|
| `§N` sem prefixo | uma seção **deste artefato** |
| `RFC §N` | uma seção da RFC DMPF Foundation v0.1 (o artefato FND-02 da série) |
| `Parte-1 §N` | um capítulo da base conceitual `Parte-1-conceitual.md` |
| `FND-03 §N` | uma seção de `upr-decision-mensagens.md`, promovido sob a ANC-01 |
| `FND-04 §N` | uma seção de `uow-inbox-outbox.md`, promovido sob a ANC-02 |
| `FND-05 §N` | uma seção de `cloudevents-protobuf-buf.md`, promovido sob a ANC-03 |
| `FND-06 §N` | uma seção de `politicas-transporte.md`, promovido sob a ANC-04 |
| `FND-07 §N` | uma seção de `contexto-erros-seguranca.md`, promovido sob a ANC-05 |
| `FND-08 §N` | uma seção de `resiliencia-observabilidade.md`, promovido sob a ANC-06 |

`normativo` — O prefixo nulo tem referente **local**: dentro deste documento,
`§N` é sempre uma seção deste documento. A convenção difere da de RFC §1.3, onde
o prefixo nulo designa a própria RFC, e segue a de FND-04 §1.2, FND-05 §1.2,
FND-06 §1.2 e FND-07 §1.2, que resolvem o prefixo nulo em si mesmas. Uma citação
a este artefato feita **de fora** dele usa o nome do arquivo mais a seção.

`normativo` — **Regras dos artefatos irmãos são citadas pelo identificador
estável, não pelo número de linha.** `INT-05`, `ENV-24`, `OBX-13`, `GAR-01` e
`CTX-15` designam a regra onde ela estiver. Número de linha e número de subseção
podem aparecer como auxílio de localização, nunca como âncora única da citação.
A exigência importa em dobro aqui: este artefato depende de sete fontes
normativas extensas, que renumeram a cada revisão, e uma citação por linha é uma
citação com prazo — o risco está registrado, e a regra que o cobre, em FND-05
§1.2 e FND-07 §1.2.

`registro` — **FND-08 já está promovido e entra na tabela.** A âncora ANC-06 e a
história ARQ-445 continuam registradas, e agora há documento promovido
(`resiliencia-observabilidade.md`, sob ANC-06) a citar por subseção, como os demais
irmãos. Onde a matéria de resiliência ou observabilidade toca este artefato e
permanece de FND-08, a referência é à subseção concreta e à ANC-06, como fronteira
por matéria; §1.4 e §12 tratam dessa consequência.

`normativo` — **Os identificadores próprios de FND-09.** Este artefato numera as
regras que estabelece com sete prefixos, um por matéria:

| Prefixo | Matéria | Seção |
|---------|---------|-------|
| `PIR` | Pirâmide de testes: camadas, escopo, infraestrutura e ownership | §2 |
| `CEN` | Catálogo de cenários distribuídos | §4 |
| `FIX` | Golden fixtures Go ↔ TS: formato, pipeline e metadado de versão | §5 |
| `ORA` | Os três oráculos, a precisão de tipos e a equivalência do desfecho | §6, §7 |
| `KIT` | Test kits por camada, determinismo e pipeline de CI | §8 |
| `FIT` | Testes de arquitetura como fitness function | §9 |
| `RAS` | Rastreabilidade: diagnóstico estável e par de vetores | §10, §11 |

`normativo` — **`RAS`, e não `VER`.** O prefixo da rastreabilidade é `RAS` por
decisão deliberada. A cadeia que cada regra `RAS` instancia termina em «par de
vetores positivo e negativo, pareados Go/TS» (RFC §14.5), e vetor de conformidade
é exatamente o que a RFC já numera com `V01`–`V32` — o grupo de trinta e dois
vetores pareados da RFC. Um prefixo `VER`, de verificação, encostado em `V`
produziria ambiguidade no ponto exato em que os dois se encontram: a terminação
da cadeia de FND-09 é o território dos `V` da RFC. `RAS` mantém os dois legíveis.

`normativo` — **Nenhum dos sete colide com os prefixos já em uso no acervo.** Os
identificadores estáveis herdados distribuem-se em quarenta e cinco prefixos, da RFC
a FND-08; `PIR`, `CEN`, `FIX`, `ORA`, `KIT`, `FIT` e `RAS` são todos novos,
verificado por varredura. O índice completo de identificadores — os próprios e o
mapa por regra que cobre o acervo — vive na §13, que é a fonte da verificação e
não este preâmbulo.

`rationale` — **Por que `FIT` e não `ARQ`.** O nome natural da matéria da §9 seria
`ARQ`, de arquitetura, e foi assim que a decisão de planejamento o fixou. A escolha
não sobrevive ao teste que este próprio artefato aplica em `RAS`: `ARQ` é a chave do
projeto que hospeda o épico e todas as histórias do DMPF, e `ARQ-01` conviveria com
`ARQ-446` — a história que encomenda este documento — no mesmo parágrafo. Tentou-se
primeiro desambiguar por convenção de forma, reservando colchete e link à chave do
rastreador; a convenção é frágil, porque depende de disciplina de formatação em todo
uso futuro e não sobrevive a uma varredura que só veja o token. `FIT`, de *fitness
function* — o termo que a §9 já usa para o instrumento —, não ocorre em nenhum
artefato do acervo e elimina a ambiguidade na origem, em vez de administrá-la.

### §1.3 As categorias de conteúdo

`normativo`

Nem todo bloco deste artefato obriga. Cada bloco carrega um rótulo de força, e os
rótulos são os mesmos dos irmãos — este documento não cria categoria nova:

| Rótulo | Significado | Obriga? |
|--------|-------------|---------|
| `normativo` | Regra que este artefato estabelece sobre o instrumento de prova — pirâmide, cenário, fixture, oráculo, test kit, teste de arquitetura ou cadeia de rastreabilidade —, no escopo da ANC-07 | **Sim** |
| `recepcionado` | Conteúdo reproduzido da Parte-1, da RFC ou de um artefato irmão para dar contexto contíguo, sem força nova e sem reabertura | Não — a força permanece a da fonte |
| `encaminhado` | Assunto de outra sub-spec, de provider ou de kernel, citado apenas como fronteira rotulada, com dona explícita | Não — passa a obrigar quando a dona normatizar |
| `rationale` | Justificativa de uma decisão normativa, incluindo alternativas descartadas | Não |
| `registro` | Metodologia declarada, a matriz de obrigações herdadas (§3), a análise que sustenta a ausência de ADR (§12) e o índice de identificadores mais o mapa por regra do acervo (§13) | Não |

`normativo` — `normativo` não é o rótulo default: um bloco sem rótulo é prosa de
ligação e não obriga nada. Os rótulos `normativo` e `rationale` têm aqui
exatamente o sentido de RFC §1.1; `recepcionado` usa o verbo no mesmo sentido de
RFC §1.3; `encaminhado` nomeia o gesto que RFC §1.4 pratica sem nomear.

`rationale` — **Por que este artefato não repete o segundo eixo de FND-07.** O
FND-07 acrescentou, a cada regra `normativo`, além do bloco, o **sujeito** que ela
obriga, porque a sua matéria — contexto, erro, segurança do dado — atravessa
blocos e o bloco sozinho não dizia quem age. A matéria de FND-09 é prova, e a
pergunta «quem age» já tem endereço próprio nesta série: a **camada da pirâmide**
(§2) diz onde o instrumento vive, e a **cadeia de rastreabilidade** (§10, §13)
liga cada regra do acervo ao mecanismo que a prova e ao épico de kernel que o
executa (FND-05 §8.4). Repetir isso como eixo de §1 seria redundante; a cada
regra basta declarar a força.

### §1.4 O que esta iteração tem de diferente

As seis sub-specs anteriores normatizaram **matéria**. FND-03 definiu o que a UPR
é e a forma da `Decision`; FND-04, como a outbox drena e a inbox deduplica;
FND-05, que atributos o envelope carrega e como o contrato evolui; FND-06, a
política de cada transporte; FND-07, o que o contexto propaga, como o erro se
classifica e o que o controle de segurança garante. FND-09 não normatiza matéria
própria. A sua obrigação central, encomendada pelo ARQ-446, é dar a cada garantia
normativa já decidida **ao menos um mecanismo que a prove** — e o critério de
aceite do épico incide sobre a garantia inteira do acervo, não sobre o que FND-09
inventa. É a única sub-spec cuja fronteira separa prova de matéria, e três
consequências decorrem disso.

**Primeira: o acervo é o escopo.** Os **sete** artefatos promovidos trazem **682
cláusulas `normativo`**, endereçáveis por **672 identificadores** em quarenta e cinco
prefixos, da RFC a FND-08. As duas contagens medem coisas distintas e não
coincidem por construção: uma cláusula pode obrigar sem receber identificador
próprio, e um grupo de identificadores pode ser índice de encaminhamento em vez de
regra verificável. A divergência maior está na RFC — 110 cláusulas frente a 126 identificadores — e em
FND-08, que traz 147 cláusulas em 121 regras; nos demais as duas contagens coincidem.
O mapa da §13 indexa os **identificadores** e declara ali a sua própria contagem: 664
estáveis, 654 com linha, **650 com mecanismo de prova** — e as quatro exceções
nomeadas, porque linha não é cobertura. A cadeia de rastreabilidade
de RFC §14.5 — regra → diagnóstico estável → par de vetores positivo e negativo,
pareados Go/TS — incide sobre esse conjunto inteiro. FND-09 não escolhe quais
garantias provar: escolhe **como** prová-las, e o critério de completude é o
próprio acervo. A §13 é o índice que fecha essa cobertura, regra a regra.

**Segunda: herdar não é obedecer, mas também não é reescrever.** Seis artefatos
encaminharam matéria a FND-09 por escrito. FND-05 §8.4 nomeia FND-09 dona do
oráculo executável, do formato de arquivo, do pipeline e do diagnóstico da golden
fixture. FND-04 §11.5 encaminha o diagnóstico estável por regra e o par de
vetores positivo e negativo que RFC §14.5 pede. FND-07 §4.4 encaminha a
verificação executável do isolamento por tenant, e FND-07 §11.2 encaminha o
instrumento das regras `runtime-testable` de contexto e segurança. Herdar essas
obrigações não é repeti-las nem redecidi-las: é dar-lhes o mecanismo que a dona
pediu, na fronteira que a dona traçou. A §3 exibe a matriz linha a linha, e cada
linha nomeia o artefato irmão que a força.

**Terceira: fronteira nomeada não conta como cobertura, mesmo com FND-08 publicado.** A
resiliência e a observabilidade são de ANC-06, cujo artefato — FND-08, história
ARQ-445 — já está promovido em `docs/dmpf/resiliencia-observabilidade.md`. Onde um cenário depende de
matéria de FND-08 (o caso de backpressure é o exemplo), este artefato declara
**fronteira nomeada**, com dona e condição de fechamento, e **não** inventa a
regra ausente. A consequência procedimental é dura e está registrada em §12: uma
fronteira nomeada é a ausência de uma regra, não a sua cobertura, e o item
correspondente do critério de aceite fica **bloqueado** — declarar-lhe cobertura por
um slot vazio seria exatamente o risco que o épico nomeia, o de decisões apenas
documentais. O backpressure percorreu esse caminho até o fim: ficou bloqueado
enquanto a regra de resultado não existia e passou a **coberto** por `CEN-44` quando
FND-08 a publicou (§4.8). O bloqueio não era uma posição sobre o assunto, e sim sobre
o que se podia provar naquele momento.

### §1.5 Precedências

`normativo` — **Onde este artefato e a RFC divergirem, prevalece a RFC.** Uma
divergência não se resolve neste documento: por M4 (RFC §12.2), ela indica que a
fronteira de RFC §1.4 mudou, e mudar fronteira é mudança de versão da RFC, com o
rito de RFC §14.2. A adição por âncora só pode **detalhar** e **restringir**
dentro do escopo permitido (M1); relaxar, revogar ou reinterpretar uma invariante
é vedado (M2), e relaxar constraint P0 ou célula de §7.4 exige nova versão com ADR
(M3). Este artefato acrescenta especificidade ao instrumento de prova; não move
nenhuma dessas fronteiras.

`normativo` — **Onde este artefato e um artefato irmão já promovido divergirem
sobre matéria que a âncora do irmão possui, prevalece o irmão.** FND-09 toca quase
todas as âncoras, porque prova regras de todas — e provar a regra de outra dona
não é possuí-la. O caso concreto está na golden fixture: FND-05 §8.4 divide o
ownership em três, e a divisão é vinculante. O **conteúdo obrigatório** da fixture
e a **definição dos três oráculos e do seu escopo** são de FND-05 (FND-05 §8.2,
§8.3); o **arquivo**, o **oráculo executável**, o **pipeline** que a roda e o
**diagnóstico** que ela emite são de FND-09. Onde este artefato tratar do que é de
FND-05 — o que um oráculo significa, que campos a fixture precisa conter —, ele
**recepciona** e restringe, sem redefinir; a matéria continua de FND-05, e FND-05
prevalece nela.

`normativo` — **Este artefato não edita nenhum irmão.** Onde ele discorda da
própria spec `SPEC-6RQBN98G`, a reconciliação é da spec, aplicada e registrada na
§3 — nunca uma correção silenciosa de artefato alheio. Onde a matéria é de outra
dona e a spec a atribuía a FND-09 por excesso, o gesto é **encaminhar**, com a
dona nomeada. Nenhuma regra deste documento reabre, reescreve ou revoga regra da
RFC nem dos seis irmãos promovidos: a herança se paga com mecanismo de prova, não com reedição da
matéria provada.

`rationale` — Declarar as duas precedências no corpo do texto, e não deixá-las
implícitas em M4, segue o precedente de FND-04 §1.1, FND-05 §1.1, FND-06 §1.1 e
FND-07 §1.1. O risco que a segunda cláusula cobre é agudo aqui: um artefato de
prova encosta em matéria alheia a cada regra que instrumenta, e a tentação de
«corrigir de passagem» a regra que se está provando é proporcional a essa
frequência. A regra de prova pode tornar mais estrito o que verifica; não pode
redecidir o que é verificado.

---

---

## §2. A pirâmide de testes

`normativo`

Onde os artefatos anteriores normatizaram matéria — o que a UPR é, como a
outbox drena, que atributo o envelope carrega —, esta seção normatiza a forma da
**prova**. A pirâmide de testes é a taxonomia dos testes do DMPF: cinco camadas,
da base rápida e em memória ao topo lento e integrado, cada uma com um escopo
próprio, uma garantia que sustenta, uma infraestrutura que admite e um dono que a
escreve. Ela existe porque cada regra já decidida precisa de um mecanismo que a
torne conferível, e o mecanismo não é o mesmo em todas as camadas.

A pirâmide não se confunde com a taxonomia dos **blocos** de RFC §4. Os blocos
classificam o **código sob teste**; a pirâmide classifica os **testes**. Os dois
eixos se cruzam — o SUT de cada camada é um dos seis blocos —, mas nunca se
determinam: a camada de um teste jamais altera o bloco do SUT (RFC §4.5 regra 3),
e o bloco de uma unidade jamais fixa a camada de um teste. Manter os dois eixos
separados é o que torna decidível a regra em dois passos de §2.6.

`normativo` `PIR-01` — A estratégia de teste do DMPF tem **exatamente cinco
camadas**, da base ao topo: **domínio**, **services**, **providers**, **apps** e
**fluxos distribuídos**. Todo teste pertence a **exatamente uma** camada, decidida
pelo seu escopo — o que ele exercita e de que infraestrutura depende —, nunca pelo
diretório em que reside nem pelo bloco do SUT. O custo de feedback cresce da base
ao topo, e a base é a mais povoada.

`normativo` `PIR-02` — A pirâmide é a taxonomia dos **testes**; a taxonomia dos
**blocos** de RFC §4 classifica o **código sob teste**. Os dois eixos são
independentes: a camada de um teste nunca reclassifica o bloco do SUT (RFC §4.5
regra 3), e a classificação de um bloco nunca fixa a camada de um teste. O limite
que a spec chama de «FND-02» — a matéria de blocos e regra de dependência da RFC
(RFC §4, §7, §9) — **não** define a taxonomia de testes: ela é matéria desta
seção.

`registro` — **A especificação de cada camada é de FND-09; a implementação e a
execução são dos épicos de kernel.** A divisão é a de FND-05 §8.4, generalizada
das golden fixtures para as cinco camadas: FND-09 especifica, por camada, o
escopo, a garantia provada, a infraestrutura admitida, o oráculo executável, o
formato e o diagnóstico; a implementação nas duas stacks (Go e TS) e a execução
efetiva das suítes são dos épicos de kernel — fora do escopo desta entrega (spec,
«Escopo fora»). Nenhuma camada abaixo declara teste executado; cada uma declara o
que o teste, quando escrito, tem de provar. A forma executável dos kits está em
§8; o catálogo de cenários distribuídos, em §4; as golden fixtures e os oráculos,
em §5 e §6.

A tabela consolida as cinco camadas, com o escopo e a infraestrutura no vocabulário
da spec (`Design` → `Pirâmide`); as subseções §2.1 a §2.5 detalham cada uma.

| Camada | Escopo | Infraestrutura | SUT (bloco de RFC §4) |
|--------|--------|----------------|-----------------------|
| domínio | UPR e `Decision` | nenhuma — em memória | `domain library` |
| services | caso de uso com UoW | ports fakes | `application service` |
| providers | contrato port ↔ implementação | dependência real ou container | `provider` (contra o `port`) |
| apps | borda a borda de um serviço | serviço isolado | `app` |
| fluxos distribuídos | dois ou mais serviços via broker | ambiente integrado | apps compostos, transporte real |

`registro` — Dois dos seis blocos de RFC §4 não recebem camada própria, e isso é
cobertura total, não lacuna. O bloco `port` é interface pura: é exercitado por
baixo (services, com o duplo de porta de §2.2) e por cima (providers, na
conformidade de §2.3). O `contract package` é o contrato de wire: é exercitado na
dimensão de round-trip das golden fixtures, que vive na camada de providers (§2.3)
e reaparece na de fluxos distribuídos (§2.5). Não há bloco sem teste, e não há
camada sem bloco.

### §2.1 Camada de domínio

`normativo` — SUT: `domain library` (RFC §4.1).

| Dimensão | Conteúdo |
|----------|----------|
| Escopo | UPR (FND-03 §2) e o desfecho `Decision` (FND-03 §3), exercitados em memória |
| O que prova | Determinismo da UPR e exaustividade tipada do desfecho (FND-03 §2.4, §3.1) |
| Infraestrutura | Nenhuma — em memória (RFC §9.1); nenhum duplo de teste dentro do domínio (RFC §9.2) |
| Ownership | Especificação: FND-09. Escrita e manutenção: o épico de kernel dono da `domain library`, em cada stack (FND-05 §8.4) |

`normativo` `PIR-03` — Um teste é de **domínio** quando exercita exclusivamente uma
`domain library` (RFC §4.1) — a UPR (FND-03 §2) e o desfecho `Decision` (FND-03
§3) — sem atravessar nenhuma fronteira de aplicação. O SUT permanece `domain
library` independentemente do teste que o exercita (RFC §4.5 regra 3).

`normativo` `PIR-04` — A camada de domínio **não admite infraestrutura**. Exercitar
qualquer comportamento não pode iniciar processo externo, abrir socket, tocar
banco, broker ou sistema de arquivos, nem depender de relógio de parede ou de fonte
de entropia (RFC §9.1, metade dinâmica). Um teste de domínio **não contém** duplo de
teste — mock, fake ou stub — de infraestrutura: se o domínio precisa do duplo, é
porque depende da infraestrutura, e a dependência é a violação, não a ausência do
duplo (RFC §9.2). O que a decisão precisa — instante, identificador novo, semente —
chega como **valor de entrada**, resolvido pelo application service antes da chamada
(RFC §9.3; FND-03 §2.4).

`normativo` `PIR-05` — A camada prova o **determinismo** da UPR — fixados a requisição
e o estado de entrada, duas execuções produzem a mesma variante de `Decision`, a
mesma resposta e a **mesma sequência ordenada** de eventos de domínio (FND-03 §2.4)
— e a **exaustividade tipada** do desfecho, com `Decision` como união exaustiva
(FND-03 §3.1). A ordenação dos eventos não é acessório: sem ela, «mesmo desfecho»
não teria significado comparável entre Go e TS, e a fixture de interoperabilidade
não teria critério (FND-03 §2.4). A garantia é `runtime-testable` (RFC §2.2): é a
metade dinâmica de RFC §9.1, e os vetores que a exercitam nas duas stacks são de
FND-09 (FND-03 §2.4, encaminhado).

### §2.2 Camada de services

`normativo` — SUT: `application service` (RFC §4.1).

| Dimensão | Conteúdo |
|----------|----------|
| Escopo | Um caso de uso ponta a ponta, com a UoW exercitada (FND-04 §3) |
| O que prova | A sequência canônica de escrita: inbox, efeitos locais e outbox derivada numa única fronteira transacional (FND-04 §3.1) |
| Infraestrutura | Ports fakes; a transação local real não entra (é do `provider` — FND-04 §3.1) |
| Ownership | Especificação: FND-09. Escrita e manutenção: o épico de kernel dono do `application service` (FND-05 §8.4) |

`normativo` `PIR-06` — Um teste é de **services** quando exercita um caso de uso ponta
a ponta sobre um `application service` (RFC §4.1) — a orquestração de autorização,
transação, sequência e persistência por porta —, com o I/O entregue por **ports
fakes**. Um duplo de teste para uma **porta** é legítimo e ocorre na camada de
aplicação, que é quem possui a porta (RFC §9.2); é essa titularidade que separa o
duplo legítimo de services do duplo proibido no domínio (§2.1, `PIR-04`).

`normativo` `PIR-07` — A camada exercita a **Unit of Work**: a fronteira de aplicação
que envolve **uma única** transação local e entrega ao callback as portas a ela
vinculadas (FND-04 §3.1). O teste prova que a inbox, os efeitos locais e a outbox
derivada compartilham uma única fronteira transacional (FND-04 §3, `UOW-01` a
`UOW-04`), e que uma porta usada fora do callback não participa do commit. A UoW é
distinta da transação local — que é mecanismo do banco e vive no `provider` —, e a
transação real não entra nesta camada. A camada prova o lado da **escrita**: a
entrega efetiva (redelivery, at-least-once) é da camada de fluxos distribuídos
(§2.5).

`normativo` `PIR-08` — O aparato de **determinismo** dos testes — relógio fake, semente
fixa, ordenação estável — vive em services (ou providers), **nunca** no domínio:
introduzi-lo no domínio seria o duplo que RFC §9.2 proíbe. Esta seção fixa a
residência do aparato; a sua forma executável é especificada em §8.

### §2.3 Camada de providers

`normativo` — SUT: `provider` (RFC §4.1), contra o `port` que implementa.

| Dimensão | Conteúdo |
|----------|----------|
| Escopo | Conformidade de um `provider` contra o contrato do `port` (RFC §7.4, célula 28) |
| O que prova | Que o provider satisfaz o port; e, na dimensão de wire, o round-trip Go↔TS das golden fixtures pelos três oráculos (FND-05 §8.3) |
| Infraestrutura | Dependência real ou container; determinístico (sem porta aleatória, relógio de parede ou ordem de map) |
| Ownership | Oráculo executável, formato, pipeline e diagnóstico: FND-09. Conteúdo da fixture e definição dos oráculos: FND-05. Implementação e execução nas duas stacks: épicos de kernel e de contratos (FND-05 §8.4) |

`normativo` `PIR-09` — Um teste é de **providers** quando verifica a **conformidade** de
uma implementação `provider` (RFC §4.1) contra o contrato do `port` que ela
implementa (RFC §7.4, célula 28: `provider → port` é permitida). É a camada que o
test kit de conformidade de §8 certifica. A dependência real ou o container entra:
sem a tecnologia concreta não há o que conformar, e o duplo de porta — legítimo em
services — devolveria o teste à camada de baixo.

`normativo` `PIR-10` — Na dimensão de **wire**, a camada de providers exercita o
round-trip **bidirecional** das golden fixtures Go↔TS — Go produz e TS consome, e o
inverso, as duas direções exigidas (FND-05 `INT-03`) — pelos **três oráculos** de
FND-05 §8.3: equivalência semântica e igualdade do `payload_hash` sempre, nas duas
direções; identidade de bytes **apenas** onde `ENV-24` a exige. Os três são
avaliados e **reportados separadamente** (FND-05 `INT-04`), porque cada um falha por
motivo distinto. A divisão de ownership é a de FND-05 §8.4: o conteúdo obrigatório
da fixture e a definição dos oráculos são de FND-05; o oráculo executável, o formato
de arquivo, o pipeline e o diagnóstico são de FND-09 (§5, §6); a implementação nas
duas stacks e a execução efetiva são dos épicos de kernel e de contratos.

`normativo` `PIR-11` — O teste de provider é **determinístico**: não depende de porta
aleatória, de relógio de parede nem de ordem de map (spec, não-funcionais). A
dependência real ou o container é isolado ao teste, e o seu ciclo de vida não vaza
para outros testes da suíte.

### §2.4 Camada de apps

`normativo` — SUT: `app` (RFC §4.1).

| Dimensão | Conteúdo |
|----------|----------|
| Escopo | Borda a borda de **um** serviço isolado |
| O que prova | Que a composição liga adapters, portas e providers na borda; validação de formato e mapeamento wire ↔ domínio (RFC §4.6) |
| Infraestrutura | Serviço isolado — o wiring real, sem um segundo serviço |
| Ownership | Especificação: FND-09. Escrita e manutenção: o épico de kernel dono do `app` / composition root (FND-05 §8.4) |

`normativo` `PIR-12` — Um teste é de **apps** quando exercita **um** serviço isolado da
borda à borda: o `app` (RFC §4.1) — adapter de protocolo, composition root, wiring e
lifecycle — com as dependências concretas compostas, mas **sem** um segundo serviço.
O `app` pode depender de todos os blocos (RFC §7.4, linha `app`, permitida para
todas as colunas), e é isso que a camada exercita: a composição real, não um recorte
dela.

`normativo` `PIR-13` — A camada prova que a composição do serviço liga corretamente
adapters, portas e providers, e que a **borda faz o que é da borda**: validação de
formato de entrada e mapeamento wire ↔ domínio, ambos `app` por RFC §4.6. Não prova
comportamento **entre** processos — reentrega, corrida de inbox e ordem de ACK ficam
para a camada de fluxos distribuídos (§2.5): um teste de apps que dependesse de um
segundo serviço já seria daquela camada.

### §2.5 Camada de fluxos distribuídos

`normativo` — SUTs: apps compostos, ligados por transporte real (FND-06).

| Dimensão | Conteúdo |
|----------|----------|
| Escopo | Dois ou mais serviços comunicando por broker |
| O que prova | A semântica oficial at-least-once com efeitos idempotentes, sob reentrega deliberada (FND-04 §7.1); comportamento por transporte (FND-06) |
| Infraestrutura | Ambiente integrado, broker real; roda em pipeline separado (spec, não-funcional P1) |
| Ownership | Especificação: FND-09. Harness de reentrega, verificação cross-stack e execução: épicos de kernel (FND-04 §7.1, encaminhado; FND-05 §8.4) |

`normativo` `PIR-14` — Um teste é de **fluxos distribuídos** quando envolve **dois ou
mais processos** comunicando por broker em ambiente integrado. É a única camada que
prova o que só se demonstra com o sistema em movimento: reentrega real, corrida de
inbox (FND-04 §7.3, `#7`), ordem do ACK. **Nenhum duplo substitui o broker** aqui —
substituí-lo devolveria o teste às camadas de baixo e desfaria justamente a prova
que só o movimento oferece.

`normativo` `PIR-15` — A camada prova a **semântica oficial** — entrega at-least-once
com efeitos efetivamente idempotentes (FND-04 §7.1, constraint P0-3) — sob reentrega
**deliberada**. Essa garantia é `runtime-testable` (RFC §2.2): FND-04 §7.1 é
explícito em que o vetor `V32` (efeito idempotente sob redelivery) **não** é
satisfeito por revisão de código — só a reentrega executada o demonstra (RFC §11.3).
O catálogo dos cenários que esta camada exercita — commit antes do ACK, redelivery,
duplicata concorrente, poison e graceful shutdown — está em §4.

`normativo` `PIR-16` — A camada roda em **pipeline separado**, para não penalizar o
feedback rápido das camadas de baixo (spec, não-funcional P1). O comportamento de
**resiliência** que um fluxo distribuído também expõe — tempo de retry, DLQ,
quarantine, telemetria de entrega e backpressure — é matéria de **FND-08** (ANC-06)
e **não** é decidido aqui: a camada nomeia a fronteira e para. O cenário de
backpressure existe no catálogo (`CEN-44`, §4.8) porque FND-08 publicou a regra de
resultado sob saturação — o que esta camada **exercita**, e não decide. A garantia de
resultado continua não pertencendo a FND-09, e é isso que a fronteira preserva.

### §2.6 A regra em dois passos (esclarecimento aditivo)

`normativo`

O terceiro cenário de teste da spec descreve um teste da camada de domínio que
precisa de um mock de repositório para executar, e manda sinalizá-lo como violação
do limite da RFC **e** reclassificá-lo para services. A regra abaixo torna
explícito o que a spec deixa implícito sobre o SUT, sem editar a spec: são **dois
fatos distintos**, e eles convivem.

`normativo` `PIR-17` — Quando um teste da camada de domínio precisa de um duplo de
infraestrutura, valem ao mesmo tempo **dois fatos**, e **ambos** são registrados:

1. **Acoplamento no SUT.** A `domain library` sob teste depende de infraestrutura.
   Isso é violação do limite da RFC (§9): a unidade **continua** `domain
   library` o tempo todo (RFC §4.5 regra 3), e o duplo **não** a torna conforme (RFC
   §9.2). O reparo é **remover a dependência**, não manter o duplo.
2. **Teste mal categorizado.** Se o comportamento genuinamente exercitado é de
   `application service`, então o **teste** — e não o SUT — se move para a camada de
   services.

Os dois passos convivem e nenhum dispensa o outro: sinalizar a violação (passo 1)
não conclui a reclassificação do teste (passo 2), e reclassificar o teste não sana a
violação de acoplamento do SUT.

`rationale` — Este é um **esclarecimento aditivo, não uma correção da RFC.** RFC §4.5
regra 3 fala do **código sob teste**: «Teste não altera a classificação do código
sob teste (...) uma unidade `domain` não deixa de sê-lo por ter teste que usa
infraestrutura — embora §9 proíba que ela **precise** de infraestrutura para ser
testada». A spec reclassifica o **teste**, objeto que RFC §4.5 regra 3 não governa —
a RFC não define a taxonomia de testes (`PIR-02`). A leitura de que a spec
contradiria RFC §4.5 regra 3 foi levantada e **refutada**: não há contradição,
porque os dois enunciados têm sujeitos distintos — o SUT e o teste —, e a própria
regra 3 antecipa este caso ao dizer que a unidade `domain` continua `domain` embora
§9 proíba que ela precise de infraestrutura. A spec só deixa implícito o desfecho do
SUT; `PIR-17` o torna explícito.

### §2.7 O que a pirâmide não decide

`normativo` `PIR-18` — A pirâmide de testes **não decide**:

1. O **framework de teste por stack** — cada stack usa o idiomático; a escolha é dos
   épicos de kernel (spec, «Escopo fora»), não desta fundação normativa.
2. **Testes de carga e de performance** — a fundação não define metas de throughput,
   e eles estão fora do escopo do ARQ-446 (spec, «Escopo fora»).
3. O **comportamento de resiliência, observabilidade, retry e telemetria** — matéria
   de **FND-08** (ANC-06), nomeada como fronteira nas camadas de providers (§2.3) e
   de fluxos distribuídos (§2.5, `PIR-16`), e nunca decidida aqui.

`registro` — A **verificação executável** das regras herdadas de FND-07 que ANC-07
encaminha a FND-09 — isolamento por tenant, ciclo de vida e cancelamento do contexto,
cifra em repouso e teto de retenção — é instrumentada em §11, não enumerada nesta
seção: são regras `runtime-testable` exercitadas pelas camadas de services, apps e
fluxos distribuídos, cujo oráculo e cujo par de vetores esta seção não fixa (FND-07
§4.4, encaminhado a ANC-07). A pirâmide diz **em que camada** cada uma dessas
verificações vive; §11 diz **o que** cada uma tem de constatar.

---

## §3. As obrigações herdadas

Este artefato é **o único devedor de sete credores**. Os precedentes promovidos —
FND-02, FND-03, FND-04, FND-05, FND-06, FND-07 e FND-08 — encaminharam-lhe matéria de
teste e interoperabilidade por escrito, e nenhum deles pode mais ser editado. A
assimetria é diferente da que FND-07 enfrentou: lá o débito era com três irmãos e
a matéria era de proteção do dado; aqui o débito é com sete e a matéria é **a
prova**. FND-09 não decide um assunto próprio que os irmãos ignoram — decide o
mecanismo que torna conferível cada regra que eles já decidiram. Por isso a cadeia
de rastreabilidade garantia → teste de RFC §14.5 incide sobre o acervo inteiro
(682 regras), não sobre o que FND-09 inventa, e por isso a matriz precisa provar,
linha a linha, que cada obrigação encaminhada tem destino — e nomear, quando não
tem, a quem passou e por quê.

`registro` — A convenção de referência é a de §1.2: cada fonte é citada pela
**subseção** do artefato irmão versionado em `docs/dmpf/`. As delegações
theme-level — a linha «Testes e interoperabilidade» da tabela de fronteira de cada
irmão (§1.4) e a âncora `ANC-07` de RFC §12.3 — vivem em §1; esta seção decompõe as
obrigações **específicas** que essas delegações carregam.

### §3.1 Os quatro estados e o que cada um exige

`registro`

| Estado | Significado | O que a linha precisa exibir |
|--------|-------------|------------------------------|
| `quitada` | Este artefato **decide** a matéria delegada e a torna conferível | ID normativo da regra, o predicado ou diagnóstico decidível e — quando a regra é `runtime-testable` — o par de vetores |
| `encaminhada` | Este artefato **não** decide: nomeia a dona e o rito | dona explícita e a seção onde a fronteira aparece |
| `em redação` | A matéria é deste artefato e a seção está prevista, mas a regra ainda não foi escrita | apenas sujeito e seção — é estado de trabalho |
| `bloqueada` | Não decide e não tem a quem encaminhar: falta insumo | o insumo que falta e a pendência que o registra |

`normativo` — **Nenhuma obrigação permanece `em redação` no documento promovido.**
Esta matriz é escrita em duas passadas: a de **atribuição**, que nasce com a
fronteira e é a que esta subseção §3.2 exibe, e a de **prova**, que só é
preenchível depois que as seções §4 a §11 existem, e que fecha cada estado quando a
§3 é revisitada ao fim da redação. Uma linha `em redação` no aceite do PR é defeito
de entrega, não estado válido.

`normativo` — **Declarar fronteira não quita obrigação.** Uma linha cujo estado
seja `quitada` mas cuja seção apenas nomeie a dona de outro assunto é defeito de
rastreabilidade, e a correção é do artefato — não do estado. A consequência
operacional dessa regra nesta iteração é dura e explícita: um slot reservado a
FND-08 (§3.5) **não** conta como cenário coberto.

`normativo` — **`quitada` exige a prova calibrada pelo modo.** A exigência do par
de vetores é a de FND-07, mas aqui ela é instanciada regra a regra e **calibrada
pelo modo de verificação**, porque tratar as 682 regras do acervo como iguais
produziria vetor de execução para regra que só se confere por inspeção. Uma linha
só é `quitada` quando exibe:

- para regra `runtime-testable` — o oráculo aplicável e o **par de vetores**
  positivo e negativo, ambos reconhecíveis por quem revisa. O vetor negativo é o
  que impede a alegação vazia: «as duas stacks concordam» e «o isolamento vale» são
  afirmações que qualquer implementação pode dizer cumprir;
- para regra `structurally reviewable` — o **critério de inspeção decidível**: o
  que caracteriza conformidade e o que a viola, sem vetor de execução;
- para regra `import-verifiable` — o **diagnóstico estável reusado** de
  `DMPF-D001`/`DMPF-D002` ou `DMPF-E001`..`E004` e a aresta positiva e negativa, o
  diagnóstico já existente na RFC §10.3.

`rationale` — O quarto estado (`em redação`) é estado de método, não de conteúdo.
Uma matriz que herda de sete fontes, escrita numa única passada antes das seções
que ela indexa, só admitiria dois desfechos ruins: inventar o estado final antes de
a regra existir, ou deixar a matriz para o fim e redigir o artefato inteiro sem o
mapa que organiza o escopo. O estado de trabalho torna a matriz utilizável durante
a redação e explicitamente incompleta enquanto o é — o mesmo mecanismo que FND-07
§2.1 introduziu e que este artefato replica.

### §3.2 A matriz de obrigações herdadas

`registro` — Cada linha nomeia a fonte pela subseção do artefato irmão, a obrigação
**atômica** que dela decorre, o sujeito que o mecanismo exercita, a seção que a
trata, o destino previsto e o estado.

`registro` — **A matriz foi escrita em duas passadas, e a tabela abaixo exibe o
estado final.** A passada de **atribuição** nasceu com a fronteira, antes de as
seções existirem: nela nenhuma linha podia ser `quitada`, porque a regra que a
quitaria ainda não havia sido escrita, e todas as 25 mapeadas à época estavam `em redação` — o estado
de trabalho de §3.1. A passada de **prova** fechou cada linha depois que as regras
`PIR`, `CEN`, `FIX`, `ORA`, `KIT`, `FIT` e `RAS` de §2 e de §4 a §11 passaram a
existir, e é o que se lê aqui: cada estado cita a regra própria que o decide, o que
torna a linha conferível sem sair do documento. O resultado da passada de prova está
registrado ao fim desta subseção.

`registro` — **Convenção de ID.** Os identificadores desta matriz seguem o padrão
`H<n>-<k>`: `H` de herança, `<n>` é o número do artefato irmão de origem (2 a 8) e
`<k>` é o índice sequencial da obrigação dentro daquela origem. O ID codifica a
fonte no próprio nome — que é exatamente o eixo de atomização desta matriz — e não
colide com os prefixos próprios de FND-09 (`PIR`/`CEN`/`FIX`/`ORA`/`KIT`/`FIT`/`RAS`,
§13) nem com os 40 prefixos do acervo. O **Sujeito** nomeia a camada da pirâmide de
§2 que o mecanismo exercita (domínio, services, provider, app, fluxo distribuído),
ou `artefato` quando o instrumento é transversal ao acervo — e não o bloco que age,
como nos precedentes, porque numa obrigação de teste o que importa é a camada
exercitada.

| # | ID | Fonte da delegação | Obrigação atômica | Sujeito | Seção | Destino previsto | Estado |
|---|----|--------------------|-------------------|---------|-------|------------------|--------|
| 1 | `H2-1` | RFC §1.4, §12.3 (`ANC-07`) | Assumir a titularidade de testes e interoperabilidade sob `ANC-07`, respeitando as invariantes §9 (domínio executável em memória, sem infra) e §4.5 regra 3 (o teste não reclassifica o SUT); por `M1`, detalhar e restringir; por `M2`, nunca relaxar, revogar nem reinterpretar | artefato | §1, §2, §9 | §1.1; §2 | `quitada` — §1.1 institui a titularidade sob `ANC-07` (monotonicidade `M1`–`M4`); as duas invariantes intocáveis aterrissam em `PIR-17` (regra em dois passos, o teste reclassifica-se, nunca o SUT) e `PIR-18` (o que a pirâmide não decide), e o domínio em memória em `PIR-02`/§2.1 e §7. `structurally reviewable`: critério de inspeção — titularidade assumida, monotonicidade respeitada, invariantes não afrouxadas (§1.1) |
| 2 | `H3-1` | FND-03 §2.2 | Códigos de diagnóstico das invariantes import-verifiable (`UPR-I07`..`UPR-I12`) e par de vetores pareados Go/TS das doze `UPR-I` | domínio | §10, §11 | §9/§10; §7/§8 | `quitada` — doze `UPR-I` mapeadas em §13.4, calibradas por modo: `UPR-I07`..`UPR-I12` `import-verifiable` reusam `DMPF-D001`/`DMPF-E001` (§9 `FIT`, §10 `RAS-08`) com aresta positiva/negativa; `UPR-I03`/`I05`/`I06` `runtime-testable` por `ORA-30`..`ORA-40` (§7)/`KIT-02` (§8); `UPR-I01`/`I02` `structurally reviewable`; `UPR-I04` é permissão (nada a verificar, registrado) |
| 3 | `H3-2` | FND-03 §2.4 | Vetores que exercitam o determinismo da UPR (mesma entrada → mesmo desfecho e mesma sequência ordenada de eventos) nas duas stacks | domínio | §4, §7 | §7; §8 | `quitada` — `ORA-34` (§7): fixados entrada e estado, a projeção é a mesma variante, resposta e sequência ordenada, «observacionalmente iguais na mesma stack e entre stacks»; `UPR-L01`..`UPR-L05` `runtime-testable` por `KIT-02` (§8). Par de vetores em §7.3 (tabelas 1 e 2) |
| 4 | `H3-3` | FND-03 §3.4, §10.4 | Oráculo de equivalência **observável** do desfecho `Decision` (ramos `Accepted`/`Rejected`) entre Go e TS, cobrindo cada linha da tabela de observações de §3.4 e `DEC-04`/`DEC-08`/`DEC-10`/`DEC-11`/`DEC-12`/`DEC-13` | domínio | §7 | §7 | `quitada` — `ORA-30`..`ORA-40` (§7), fixture de projeção observável (sem wire, sem `payload_hash`): §7.3 cobre as onze observações de §3.4 e as seis `DEC` (`DEC-04`/`08`/`10`/`11`/`12`/`13`), com par de vetores por ramo e negativo por proibição (`ORA-39`). Classe de oráculo distinta registrada em C2 de §3.4; `DEC-08` com decidibilidade partida (`ORA-40`, lado-entrega declarado a FND-04) |
| 5 | `H3-4` | FND-03 §4.3 | Test kit do application service com portas substituídas (ports fakes); o teste de domínio em memória sem mock de infra como base da pirâmide | services | §8, §2 | §8 | `quitada` no critério, com a execução `encaminhada` (épicos de kernel) — `KIT-03` (kit de services com portas fakes) e `KIT-02` (domínio em memória, base da pirâmide, `PIR-02`/§2.1) fixam o instrumento e o oráculo; a execução/implementação nas duas stacks é dos épicos de kernel (FND-05 §8.4) |
| 6 | `H3-5` | FND-03 §10.1 | Elo de RFC §14.5 para as regras de FND-03: diagnóstico estável e par de vetores Go/TS por regra, nomeando o cenário pelo ID herdado | artefato | §10, §13 | §10; §13.4 | `quitada` — a cadeia de RFC §14.5 é instanciada regra a regra em §13.4 (54 regras de FND-03), reusando o diagnóstico de RFC §10.3 (`RAS-08`) e a forma do par de vetores de §10 (`RAS-01`..`RAS-16`), cada linha nomeando o ID herdado e onde a prova vive |
| 7 | `H3-6` | FND-03 §10.4 | Declarar o modo de verificação das 42 regras sem modo (`UPR-L`, `DEC`, `FRT`, `MSG-N`, `CTR`, `ESC`) e, onde import-verifiable, o diagnóstico estável compartilhado com o linter de dependências de RFC §10 | artefato | §11, §9 | §11.2 | `quitada` — `RAS-32` (§11.2) declara o modo das 42 (`UPR-L` 5, `DEC` 13, `FRT` 4, `MSG-N` 4, `CTR` 7, `ESC` 9), com duas exceções por ID em `ESC` (`ESC-04` runtime, `ESC-05` import); onde `import-verifiable` (`FRT-01`/`02`, `CTR-01`/`02`/`03`, `ESC-05`), o diagnóstico de aresta proibida é compartilhado com o linter de RFC §10 (§9 `FIT`) |
| 8 | `H3-7` | FND-03 §10.4, §8 | Fixtures de projeção observável derivadas dos quatro exemplos de §8 como base do oráculo do desfecho | domínio | §7, §5 | §7 | `quitada` — §7.3 registra as quatro fixtures de base: §8.2 e §8.3 originam as fixtures de projeção positivas (`Accepted`/`Rejected`, `ORA-30`..`ORA-40`); §8.4 fundamenta a neutralidade de codificação (`ORA-33`; `UPR-I11` é `import-verifiable`, não fixture) e FND-03 §8.5 delimita a fronteira (`ORA-40`). Os quatro papéis declarados, não inflados |
| 9 | `H4-2` | FND-04 §6.5 | Oráculo cross-stack que prova empiricamente `H1` (mesmo conteúdo de negócio → mesmo `payload_hash` em qualquer stack) entre Go e TS, usando o hash como detector de divergência de serialização | provider / wire | §6 | §6 | `quitada`, com precondição declarada — oráculo 2 `ORA-03` (§6.1): igualdade do `payload_hash` sobre `Any.value` transportado (`ENV-17`..`ENV-19`, `INB-13`/H1), par positivo/negativo (bytes reescritos → hashes divergem). Precondição = fórmula de FND-05 §4.3 + byte-preservação de FND-06 §5 (C6 de §3.4); não é bloqueio |
| 10 | `H4-3` | FND-04 §7.1, §11.1, §11.6 (`M4`) | Instrumento concreto de `V31` (varredura por promessa de exactly-once, structurally reviewable) e `V32` (efeito idempotente sob redelivery, por execução real de reentrega): a suíte, o harness de reentrega e a verificação cross-stack | fluxo distribuído | §10, §4 | §10.5; §4/§8 | `quitada` no instrumento, com execução/implementação `encaminhada` (épicos de kernel) — `RAS-12` (§10.5): `V31` `structurally reviewable`, instrumento = varredura, par positivo/negativo de artefato; `RAS-13` (§10.5): `V32` `runtime-testable`, instrumento = harness de reentrega (`KIT-06`, §8), par de vetores; `RAS-14` fixa «não verificado» sem instrumento. Cenários de reentrega `CEN-03`/`06`/`12` (§4.2). Harness executado nos épicos de kernel |
| 11 | `H4-4` | FND-04 §7.3, §11.3 (crit. 25) | Converter os doze failure modes de §7.3 em catálogo de cenários executáveis, respeitando a classificação de modo (1–7/11/12 runtime-testable; 8–9 com produtor instrumentado; 10 por evidência de execução operacional) | fluxo distribuído | §4 | §4.2 | `quitada` — `CEN-01`..`CEN-12` (§4.2) convertem os doze failure modes, cada `CEN` citando o `#` e preservando a categoria de desfecho e o modo de FND-04: `#1`–`#7`/`#11`/`#12` `runtime-testable`, `#8`/`#9` `runtime-testable com produtor instrumentado` (`CEN-08`/`CEN-09`), `#10` por evidência de execução operacional (`CEN-10`, não teste automatizado) |
| 12 | `H4-5` | FND-04 §11.2, §11.5 (pend. 3) | Diagnóstico estável e par de vetores positivo/negativo por regra (`BLK`/`UOW`/`OBX`/`INB`/`GAR`), nomeando o cenário pelo ID estável de §11.2 | artefato | §10, §13 | §10; §13.4 | `quitada` — as 64 regras de FND-04 mapeadas em §13.4, calibradas por modo: `BLK` `structurally reviewable` (§9 `FIT`); `UOW`/`OBX`/`INB` `runtime-testable` por `KIT-03`/`KIT-04`/`KIT-06` (§8) e `CEN` (§4); `GAR` por `KIT-06`/`CEN` e `V31`/`V32` (§10 `RAS`). Exceções declaradas: `OBX-02`/`03`, `GAR-01`/`06`/`10`/`11` por inspeção; `GAR-07`/`09`/`INB-15`/`16` por evidência; `DEC-08` partida |
| 13 | `H5-1` | FND-05 §1.4, §8.4, §10.4 pend. 8 | Formato de arquivo da golden fixture, oráculo executável e pipeline do round-trip bidirecional (o **conteúdo obrigatório** e o que ela prova permanecem de FND-05, §8.2) | provider / wire | §5 | §5 | `quitada` no ownership de FND-09, com execução com evidência `encaminhada` (épicos de kernel e de contratos) — `FIX-01`..`FIX-13` (§5) fixam formato do arquivo, oráculo executável, pipeline e diagnóstico; o conteúdo obrigatório e o que a fixture prova permanecem de FND-05 (reconciliação 2 de §3.3). Execução com evidência dos épicos |
| 14 | `H5-2` | FND-05 §8.3 (`INT-04`) | Os três oráculos do round-trip (1 equivalência semântica, 2 igualdade do `payload_hash`, 3 identidade de bytes) avaliados e reportados **em separado** | provider / wire | §6 | §6 | `quitada` — `INT-04` via `ORA-06` (§6.1): oráculo 1 (`ORA-02`, equivalência), oráculo 2 (`ORA-03`, `payload_hash`) e oráculo 3 (`ORA-04`, identidade de bytes) avaliados e reportados em separado; forma do relatório em `FIX-12`/`FIX-13` (§5.4). Par de vetores por oráculo |
| 15 | `H5-3` | FND-05 §8.4 (`INT-04`) | Diagnóstico distinto por oráculo: um cenário que reprove só no oráculo 3 tem diagnóstico diferente do que reprova no 1 | provider / wire | §6 | §6 | `quitada` — `ORA-06` (§6.1) + `FIX-12`/`FIX-13` (§5.4): reprovar só no oráculo 3 emite diagnóstico distinto do oráculo 1; vetor negativo = critério único «bytes conferem» sem separação por oráculo (`INT-04`) |
| 16 | `H5-4` | FND-05 `PTB-11`/§5.3, §10.1 | Instrumento/cenário que roda os pares de vetores de compatibilidade retroativa `PTB-08`..`PTB-12` (consumidor `N-1` lê versão `N`, campos desconhecidos preservados) | provider / wire | §4, §10 | §4.6; §6/§8.3 | `quitada` — `PTB-08`..`PTB-12` mapeadas em §13.5: `PTB-10` `runtime-testable` por `ORA-04` (§6.1) + `CEN-38` (§4.6), `PTB-11` `runtime-testable` por `ORA-01` (§6.1) + `CEN-39` (§4.6); `PTB-08`/`09`/`12` `structurally reviewable` por gate `buf` (`KIT-10`, §8.3) e `CEN-36`/`37`/`40` (§4.6). Par de vetores nos runtime |
| 17 | `H5-5` | FND-05 §8.5 | Reconciliar, com a posição de FND-05 em mãos, as duas divergências registradas como handoff (critério «mesmo byte» e ownership integral da fixture) | artefato | §5, §3 | §3.3; §5/§6 | `quitada` (reconciliada em §3.3) — reconciliação 1 (o «mesmo byte» estreita-se a `oráculo 1 ∧ oráculo 2 ∧ (oráculo 3 sob ENV-24)`, `ORA-05`/`INT-05`) e reconciliação 2 (ownership dividido: arquivo/oráculo/pipeline/diagnóstico de FND-09, conteúdo de FND-05, `FIX`) de §3.3; conflito C1 de §3.4 resolvido contra a spec |
| 18 | `H5-6` | FND-05 §10.4 pend. 9, §3.5 | Forma de expressão executável do perfil de validação do envelope (schema/asserções/validador) que executa os predicados de `ENV-08` a `ENV-13` | provider / wire | §11 | §11.6 | `quitada` no instrumento, com a localização `encaminhada` (co-decisão com FND-06) — `RAS-40` (§11.6): validador local, fail-closed, executa `ENV-08`..`ENV-13`; par de vetores em `ENV-08`..`ENV-13` (§13.5) e `CEN-26` (§4.4). A localização do instrumento é co-decisão com FND-06 (FND-06 §18.2 obr. 15) |
| 19 | `H5-7` | FND-05 §10.1 | Vetores executáveis (cadeia RFC §14.5) que fecham o elo ID de regra ↔ reprovação mecânica/oráculo para as regras de FND-05 | artefato | §10 | §10; §13.5 | `quitada` — a cadeia de RFC §14.5 é instanciada em §13.5 (64 regras de FND-05), fechando ID de regra ↔ reprovação mecânica/oráculo (`ORA`/`FIX`/`KIT`) com a forma do par de vetores de §10 (`RAS-01`..`RAS-16`) |
| 20 | `H6-2` | FND-06 §18.4, §3.3 DoD/§18.1 CA-8 | Materializar os treze grupos de cenários por transporte: ACK por disposição (7 disposições nos 2 transportes com falha injetada), offset `n+1`, retry inline Kafka, perda de partição, três relógios/janela_redelivery, deadline propagado (3 saltos), limites de entrada, ordenação por chave, campos FIFO, vedação de exactly-once, coexistência, contenção de envelope inválido | fluxo distribuído | §4 | §4.4 | `quitada`, com validação por execução dos vetores Kafka limitada pela base as-is (dependência declarada, `TRP-41`) — os treze grupos em `CEN-14`..`CEN-26` (§4.4): ACK por disposição (`CEN-15`), offset `n+1` (`CEN-16`), retry inline Kafka (`CEN-17`), perda de partição (`CEN-18`), relógios/janela (`CEN-19`), deadline propagado (`CEN-20`), limites de entrada (`CEN-21`), ordenação por chave (`CEN-22`), FIFO (`CEN-23`), vedação de exactly-once (`CEN-24`), coexistência (`CEN-25`), contenção de envelope inválido (`CEN-26`). Dependência declarada, não bloqueio |
| 21 | `H6-3` | FND-06 §5.2 | Vetores negativos executáveis a partir de cada hop «não conforme» da matriz de byte-preservação (SNS sem raw delivery, SMT que reserializa, duplo Base64, transcodificação JSON, gRPC-JSON) | fluxo distribuído | §4, §6 | §4.5; §6.1 | `quitada`, com o caminho conforme de `TRP-21` dependente da modalidade claim-check (dependência declarada, `ANC-03`/FND-05, `ENV-20`) — os quatro hops não conformes em `CEN-27`..`CEN-30` (§4.5) com oráculo 3/2 `ORA-04`/`ORA-03` (§6.1): SMT/interceptor que reserializa (`CEN-27`/`TRP-17`), transcodificação e gRPC-JSON (`CEN-29`/`CEN-30`), atributos migrados (`CEN-33`/`TRP-18`); duplo Base64 é conforme sob `TRP-19` e só vira negativo por dupla codificação. Dependência declarada, não bloqueio |
| 22 | `H6-4` | FND-06 §15 | Os cinco vetores negativos de coexistência `V-COE-1`..`V-COE-5` (troca de binding em retry incerto, `consumer_name` divergente, ponte que reserializa `Any.value`, canal com ordem migrado sem drenagem, mesmo canal nos dois transportes) | fluxo distribuído | §4 | §4.5 | `quitada` — os cinco `V-COE-1`..`V-COE-5` em `CEN-31`..`CEN-35` (§4.5), mapeados a `COE-01`..`COE-08` em §13.5: troca de binding em retry incerto (`CEN-31`/`COE-02`), `consumer_name` divergente (`CEN-32`/`COE-04`), ponte que reserializa `Any.value` (`CEN-33`/`COE-05`), canal com ordem migrado sem drenagem (`CEN-34`/`COE-07`), mesmo canal nos dois transportes (`CEN-35`/`COE-01`) |
| 23 | `H6-5` | FND-06 §2.3 obr. 15/§18.2, §7.2 | Forma de expressão executável do perfil de validação do envelope (`ENV-08`..`ENV-13`, `TRP-33`..`TRP-36`): FND-06 fixa conteúdo, onde, gesto e destino; o instrumento é de FND-09 | provider / wire | §11 | §11.6; §4.4 | `quitada` no instrumento, com a localização `encaminhada` (co-decisão com FND-06) — `RAS-40` (§11.6) + `CEN-26` (§4.4): validação no `consumer adapter` antes de efeito e da inbox, executa `ENV-08`..`ENV-13` e `TRP-33`..`TRP-36`; conteúdo/onde/gesto/destino de FND-06, instrumento de FND-09. Localização a co-decidir com FND-06 |
| 24 | `H7-2` | FND-07 §4.4, §1.5, §2.2 (`MT-4`) | Instrumento e oráculo do resultado fail-closed do isolamento por tenant (`IDN-11`..`IDN-14`): acesso a dado de outro tenant não devolve o dado, a imposição não depende de convenção de código, e o tenant atravessa queries, constraints e autorização | provider e app | §11 | §11.3; §4 | `quitada` — `IDN-11`..`IDN-14` `runtime-testable` por `RAS-33`/`RAS-34` (§11.3) via `CEN-41` (§4): isolamento fail-closed (acesso cross-tenant não devolve o dado, `RAS-33`), negação ativa registrada (`IDN-12`), «negado» ≠ «vazio» asseverado sobre o registro interno (`RAS-34`/`IDN-13`), e vetor de caminho — a imposição não depende de convenção de código (`IDN-14`). Par positivo/negativo em §11.3 |
| 25 | `H7-3` | FND-07 §11.2 | Oráculo executável e pipeline das regras runtime-testable de FND-07: ciclo de vida e reuso do contexto (`CTX-15`..`CTX-17`), cancelamento e deadline (`CTX-21`, `CTX-22`), cifra em repouso por inspeção de superfície (`DAT-08`, `DAT-10`), teto de retenção efetivo (`DAT-14`) e a parte runtime de `ERR-02`/`ERR-11` | services, app e plataforma | §11 | §11.4/§11.5; §4 | `quitada`, composta pelo modo — par de vetores onde `runtime-testable`: `CTX-15`..`CTX-17` por `RAS-35`/`CEN-42`, `CTX-21`/`CTX-22` por `RAS-36`/`CEN-43`; parte runtime de `ERR-02`/`ERR-11` pelo oráculo das bordas de erro `RAS-37` (sem `CEN` dedicado). Critério de inspeção decidível onde a fonte rotula «por inspeção de superfície»: `DAT-08`/`DAT-10` calibrados a `structurally reviewable` por `RAS-38`, `DAT-14` (desigualdade) por `RAS-39` (§11.5) |
| 26 | `H8-1` | FND-08 §13.5 | Verificar que toda métrica obrigatória declara nome, unidade e fórmula, na convenção de `MET-02`/`MET-03` (FND-08 §6.1) | artefato | §11, §13 | §13.7; §11.1 | `quitada` — instrumento de inspeção da baseline de observabilidade instanciado em §13.7 sobre `MET-02`/`MET-03` (FND-08 §6.1): `structurally reviewable`, critério de inspeção decidível — cada métrica obrigatória exibe nome estável, unidade e fórmula; vetor negativo = métrica exposta sem unidade ou sem fórmula. Calibração por modo de §11.1 |
| 27 | `H8-2` | FND-08 §13.5 | Verificar a ausência de label de alta cardinalidade nas métricas, sob o limite declarado de `MET-07` (FND-08 §6.1) | artefato | §11, §13 | §13.7; §11.1 | `quitada` — instanciado em §13.7 sobre `MET-07` (FND-08 §6.1): `structurally reviewable` — nenhuma métrica usa identificador de alta cardinalidade como label (o `message_id` e afins vivem em trace e log amostrados); vetor negativo = métrica com label de cardinalidade ilimitada. Calibração por modo de §11.1 |
| 28 | `H8-3` | FND-08 §13.5 | Verificar a presença dos atributos comuns de `TRC-04` no span de cada um dos três fluxos críticos de `TRC-01` (FND-08 §5.1, §5.3) | app, provider e fluxo distribuído | §11, §13 | §13.7; §11.1 | `quitada` — instanciado em §13.7 sobre `TRC-04`/`TRC-01` (FND-08 §5): `runtime-testable`, par de vetores — os três fluxos (REST, gRPC e assíncrono) emitem span com os atributos comuns de `TRC-04`; vetor negativo = span de um dos fluxos sem um atributo comum. Calibração por modo de §11.1 |
| 29 | `H8-4` | FND-08 §13.5 | Verificar a continuidade do trace no salto assíncrono sob fronteira confiável, pelo `traceparent` do envelope, conforme `TRC-07` (FND-08 §5.4) | fluxo distribuído | §11, §13 | §13.7; §11.1 | `quitada` — instanciado em §13.7 sobre `TRC-07` (FND-08 §5.4): `runtime-testable`, par de vetores — sob fronteira confiável o span de consumo liga-se ao produtor pelo `traceparent` do envelope (veículo `ENV-08`, predicado de fronteira `CTX-27`); vetor negativo = trace rompido no salto, ou continuidade forçada fora da fronteira confiável. Calibração por modo de §11.1 |
| 30 | `H8-5` | FND-08 §13.5 | Verificar a inexistência de sujeito `domain` ou `port` em qualquer regra da baseline de observabilidade, conforme `RES-01` (FND-08 §1.3) | artefato | §9, §13 | §13.7; §11.1 | `quitada` — instanciado em §13.7 sobre `RES-01` (FND-08 §1.3), no eixo dos testes de arquitetura de §9: `structurally reviewable` — nenhuma regra da baseline obriga `domain` nem `port` como sujeito (RFC §6.2, princípio 11 da Parte-1); vetor negativo = regra da baseline cujo sujeito seja `domain` ou `port`. Calibração por modo de §11.1 |
| 31 | `H8-6` | FND-08 §13.5 | Verificar a presença dos campos obrigatórios de `LOG-02` em todo registro estruturado, no formato JSON de `LOG-01` (FND-08 §7.1) | app e application service | §11, §13 | §13.7; §11.1 | `quitada` — instanciado em §13.7 sobre `LOG-02`/`LOG-01` (FND-08 §7.1): `runtime-testable`, par de vetores — todo registro estruturado carrega os campos obrigatórios de `LOG-02`; vetor negativo = registro estruturado sem um campo obrigatório. Calibração por modo de §11.1 |

`registro` — **A matriz desconsolida as 18 obrigações do acervo em 25 obrigações
atômicas por fonte e acrescenta as seis que FND-08 §13.5 encaminhou à baseline de
observabilidade — 31 linhas atômicas no total.** Uma linha consolidada cuja origem cita mais de um artefato foi
partida em uma linha por artefato, cada uma com a sua citação-âncora. As 18 do acervo,
com o encaminhamento de FND-08 §13.5, mapeiam assim (obrigação consolidada → linhas
atômicas):

| Obr. | Descrição breve | Linhas atômicas |
|------|-----------------|-----------------|
| 1 | golden fixture round-trip | `H5-1` (formato/pipeline, FND-05) |
| 2 | `payload_hash` cross-stack | `H4-2` (FND-04 §6.5), `H5-2` (oráculo 2, FND-05 §8.3) |
| 3 | três oráculos + diagnóstico por oráculo | `H5-2`, `H5-3` |
| 4 | `V31`/`V32` | `H4-3` (FND-04); modo por RFC §11.3 (ver nota) |
| 5 | RFC §14.5 — diagnóstico + par de vetores por regra | `H3-5`, `H4-5`, `H5-7`, `H3-1` |
| 6 | catálogo dos doze failure modes | `H4-4` |
| 7 | catálogo por transporte (13 grupos) | `H6-2` |
| 8 | matriz de hops (byte-preservação) | `H6-3` |
| 9 | `V-COE-1`..`V-COE-5` | `H6-4` |
| 10 | equivalência do desfecho `Decision` | `H3-3`, `H3-7` (fixtures) |
| 11 | determinismo em duas stacks | `H3-2` |
| 12 | modo por regra das 42 sem modo | `H3-6` |
| 13 | test kit da camada services | `H3-4` |
| 14 | instrumento do perfil de validação do envelope | `H5-6` (FND-05), `H6-5` (FND-06) |
| 15 | isolamento por tenant | `H7-2` |
| 16 | regras runtime-testable de FND-07 | `H7-3` |
| 17 | `PTB-08`..`PTB-12` (compat. retroativa) | `H5-4` |
| 18 | reconciliação das divergências de FND-05 | `H5-5` |
| 19 | baseline de observabilidade (FND-08 §13.5, DoD) | `H8-1`..`H8-6` |

`registro` — **A origem de cada obrigação é conferível sem sair da cadeia.** As sete
fontes desta matriz estão versionadas em `docs/dmpf/` e cada subseção citada contém
a delegação nominal a FND-09 — verificado por varredura antes desta redação. Uma
linha cuja fonte não resolva no arquivo do irmão é defeito de rastreabilidade, no
mesmo critério que FND-07 §2.2 fixou.

`registro` — **Uma origem citada no acervo consolidado não virou linha atômica, e o
motivo é fidelidade.** A obrigação 4 (`V31`/`V32`) tem origem consolidada «RFC
§11.3; FND-04 §7.1/§11.1/§11.6». RFC §11.3 classifica o **modo** de `V31`
(structurally reviewable) e `V32` (runtime-testable) e adverte que um verificador
que os reporte aprovados sem meio de avaliá-los está incorreto — mas **não** os
delega a FND-09 com âncora própria: a tabela de obrigações encaminhadas de FND-02 a
FND-09 tem exatamente duas linhas (§1.4 e `ANC-07`, ambas theme-level). A delegação
formal do **instrumento** de `V31`/`V32` está em FND-04 §7.1/§11.6, e é `H4-3` que a
carrega; a classificação de modo da RFC §11.3 entra como referência dentro de
`H4-3` e da §10, não como linha de delegação inventada. O mesmo vale para a
referência à suíte `V01`–`V32` da RFC §11.2 na obrigação 5: ela é o objeto que a
cadeia endereça, e as delegações resolvem por FND-03 §10.1, FND-04 §11.2 e FND-05
§10.1 (`H3-5`, `H4-5`, `H5-7`).

`normativo` — **Cinco linhas têm porção `encaminhada` e o estado a declara.** A
especificação do mecanismo é de FND-09; a execução efetiva e a implementação nas
duas stacks são dos épicos de kernel, por FND-05 §8.4 — a divisão vale para `H3-4`,
`H4-3` e `H5-1`. A localização do instrumento do perfil de validação do envelope é
uma co-decisão com FND-06 (FND-06 §18.2 obr. 15 registra «a decisão é entre este
artefato e FND-09»), e vale para `H5-6` e `H6-5`. Nenhuma dessas cinco é `quitada`
sem ressalva, e nenhuma é `bloqueada`: as cinco têm dona nomeada do lado que falta.

`normativo` — **Nenhuma obrigação herdada ficou `bloqueada`.** O bloqueio existiria
se faltasse insumo e não houvesse a quem encaminhar. A matéria genuinamente
bloqueada quando esta matriz foi escrita — backpressure e o restante da
resiliência — **não é
obrigação herdada**: nenhum irmão a delegou a FND-09 como teste, porque ela está
ausente do acervo. Ela vive na §3.5, como fronteira, não como linha desta matriz.
Distinção que a §3.5 sustenta: fronteira declarada não é cobertura.

`rationale` — **A matriz é atômica por fonte de propósito.** A tentação era manter
as 18 linhas consolidadas do acervo, do jeito que a síntese as deduplicou. O
agrupamento esconderia que uma mesma obrigação aparente — «a cadeia de RFC §14.5» —
é, na verdade, delegada em separado por FND-03 (§10.1), FND-04 (§11.2) e FND-05
(§10.1), cada uma sobre as **suas** regras, com a sua âncora. Numa linha só, a
citação-âncora teria de ser inventada como composta, e a prova de que cada credor
foi atendido — que é a razão de a matriz existir — perderia a granularidade. O
número da desconsolidação do acervo, 25, é maior que os 24 de FND-07 porque as fontes
promovidas eram o dobro; com as seis linhas que FND-08 §13.5 encaminhou, a matriz
chega a 31 — resultado da atomização e do sétimo credor, não meta perseguida.

`registro` — **Resultado da passada de prova.** As 31 obrigações estão fechadas: **`quitada`: 31; `encaminhada` pura: 0; `bloqueada`: 0; `em redação`: 0.** Nenhuma linha permanece `em redação`. As seis obrigações que FND-08 §13.5 encaminhou à baseline de observabilidade (`H8-1`..`H8-6`) são `quitada`, com o instrumento calibrado por modo — inspeção onde a baseline se confere por leitura (`MET-02`/`MET-03`, `MET-07`, `RES-01`) e par de vetores onde há execução (`TRC-04`, `TRC-07`, `LOG-02`) — e instanciadas em §13.7. Cinco quitadas trazem porção `encaminhada` declarada, com dona nomeada do lado que falta: `H3-4`, `H4-3` e `H5-1` têm a execução/implementação nas duas stacks `encaminhada` aos épicos de kernel (FND-05 §8.4); `H5-6` e `H6-5` têm a localização do instrumento do perfil de validação `encaminhada` à co-decisão com FND-06 (§18.2 obr. 15). Três quitadas carregam **dependência declarada** — `H4-2` (fórmula de FND-05 §4.3 + byte-preservação de FND-06 §5), `H6-2` (base as-is de Kafka, `TRP-41`) e `H6-3` (caminho conforme de `TRP-21`, claim-check) —, que condiciona a execução mas não é insumo ausente, e por isso não é bloqueio. Cada linha exibe a regra própria que a decide (`PIR`/`CEN`/`FIX`/`ORA`/`KIT`/`FIT`/`RAS`) e, quando `runtime-testable`, o par de vetores; onde a regra é `structurally reviewable` ou de inspeção de superfície, exibe o critério de inspeção decidível, sem vetor de execução — a calibração por modo de §3.1. A matéria de resiliência não é obrigação herdada e vive na §3.5 como fronteira nomeada a FND-08, não como linha desta matriz. O backpressure, que fazia parte dela, deixou de ser fronteira quando FND-08 publicou a regra de resultado, e hoje tem cenário próprio (`CEN-44`, §4.8).
### §3.3 As reconciliações da spec

Quatro afirmações da `SPEC-6RQBN98G` não sobrevivem ao confronto com o acervo. Cada
reconciliação é aplicada na mesma branch, registrada aqui e refletida nos critérios
de aceite — **nenhuma é silenciosa** — e cada uma nomeia o artefato irmão que a
força, citado por subseção.

| # | Local na spec | Afirmação original | Correção | Forçada por |
|---|---------------|--------------------|----------|-------------|
| 1 | Decisões técnicas, l. 122 | A fixture como fonte única «garante que Go e TS concordem sobre o mesmo byte», como propriedade geral do round-trip | Identidade de bytes só onde `ENV-24` a exige (publicação sem reserialização, contenção, replay); os oráculos 1 (equivalência semântica) e 2 (`payload_hash`) valem sempre, nas duas direções | FND-05 §8.3, `INT-05` |
| 2 | RF l. 39, Localização l. 73, AC l. 143 | «formato, **ownership** e pipeline» das golden fixtures, ownership integral em FND-09 | Ownership do **arquivo, do oráculo executável, do pipeline e do diagnóstico**; o conteúdo obrigatório e a definição do que a fixture prova são de FND-05 | FND-05 §8.4 |
| 3 | Catálogo, l. 92 e l. 96 | Os seis cenários «Derivados das sequências de FND-04», entre eles «Duplicata» e «Backpressure» | Proveniência declarada por cenário; «duplicata concorrente» deriva do failure mode 7; backpressure não deriva de FND-04 e fica bloqueado | FND-04 §7.3, §5.3 |
| 4 | Camadas afetadas / Requisitos | Escopo de teste é interop de contrato/wire mais os cenários de FND-04 | Acrescentar as verificações executáveis herdadas de FND-07: isolamento por tenant, ciclo de vida e cancelamento do contexto, cifra em repouso e teto de retenção | FND-07 §4.4, §11.2 |

**Reconciliação 1 — o «mesmo byte».** A spec **acerta** o critério operacional: o
passo 3 do round-trip (l. 108) compara «campos e payload hash», e o cenário 1 (l.
152) confere «campo a campo **e** o payload hash é idêntico» — exatamente os
oráculos 1 e 2. O que ela erra é a **justificativa** da l. 122, que promete «mesmo
byte» como propriedade geral. FND-05 `INT-05` diz o contrário, literalmente: «O
oráculo 3 **não** é exigido entre produtores independentes da mesma mensagem
lógica. Duas stacks que serializem o mesmo conteúdo a partir da fixture podem
produzir bytes diferentes sem violar nada deste artefato — porque Protobuf não
oferece forma canônica normativa entre implementações. O que elas **não** podem é
divergir nos oráculos 1 e 2 sobre os bytes que efetivamente circulam.» Exigir «mesmo
byte» em geral colidiria com `H1` de FND-04 `INB-13`. O passe do round-trip é, então,
`oráculo 1 ∧ oráculo 2 ∧ (oráculo 3 quando ENV-24 se aplicar)`.

**Reconciliação 2 — o ownership.** FND-05 §8.4 («Os três owners») divide a
responsabilidade sem ambiguidade: a FND-09 cabem «o oráculo executável, o formato de
arquivo da fixture, o pipeline que a roda e o diagnóstico que ela emite»; o conteúdo
obrigatório da fixture (§8.2) e o que ela deve provar são de contrato — de FND-05.
A reivindicação de «ownership» integral da spec estreita-se para o arquivo, o oráculo,
o pipeline e o diagnóstico. A obrigação `H5-5` (§8.5) é o handoff que esta
reconciliação fecha.

**Reconciliação 3 — a proveniência dos cenários.** A spec declara seis cenários
«Derivados das sequências de FND-04» (l. 92), mas nem todos derivam de lá. FND-04
§7.3 traz, no failure mode **7**, literalmente «**Duplicata concorrente (corrida de
inbox)**»: as duas transações chamam `insert-if-absent`, uma recebe `R1` e aplica o
efeito, a outra recebe `R2` e curto-circuita, ambas permanecem vivas — classificado
`Recuperação`. A «duplicata» genérica da spec (l. 96) estreita-se para esse cenário,
com o `#` citado e os IDs `INB` aplicáveis. Graceful shutdown não é failure mode:
deriva de FND-04 §5.3 / `OBX-13` (capacidade do drenador), citada como fonte
distinta. **Backpressure está ausente de todo o FND-04** — verificado, zero
ocorrências — e é matéria de resiliência: vira fronteira nomeada a FND-08 (§3.5),
não cenário derivado. A afirmação «derivados das sequências de FND-04» é corrigida
para ser precisa por cenário.

**Reconciliação 4 — o escopo herdado de FND-07.** FND-07 §4.4 e §11.2 roteiam a
FND-09 verificações executáveis que a spec destinatária não escopa. §4.4 nomeia,
sobre `IDN-11`..`IDN-14`, que «a verificação executável do isolamento é de FND-09
sob ANC-07»; §11.2 encaminha o oráculo e o pipeline das regras runtime-testable —
`CTX-15`..`CTX-17`, `CTX-21`, `CTX-22`, `DAT-08`, `DAT-10`, `DAT-14` e a parte runtime
de `ERR-02`/`ERR-11`. A spec não menciona tenant, isolamento, ciclo de vida de
contexto, cifra em repouso nem retenção — verificado. `ANC-07` nomeia FND-09 dona da
«verificação executável», e o escopo em `ANC-06` reserva a FND-08 telemetria, retry e
degradação, não estas pós-condições; logo a matéria é de FND-09. Entra no escopo,
como `H7-2` e `H7-3`, com task e critério de aceite próprios.

`rationale` — **Uma quinta edição foi proposta e retirada.** A revisão anterior
alegava contradição entre a spec e RFC §4.5 regra 3 na regra em dois passos do teste
de domínio que precisa de mock de infra. Não há contradição: a §4.5 regra 3 fala do
**SUT** — «Teste não altera a classificação do código sob teste» —, e a spec
reclassifica o **teste**, não o SUT, continuando a sinalizar a dependência ilegal.
Os dois fatos convivem, e a RFC não define a taxonomia dos testes. A regra em dois
passos entra como esclarecimento aditivo em §2, sem editar a spec. A edição não
passava no teste de «cada reconciliação cita o irmão que a força», e por isso saiu.

### §3.4 Divergências resolvidas

Os seis conflitos da varredura, cada um com o desfecho e a fonte que o decide. Onde
o conflito se resolveu porque a leitura da spec estava errada, está dito; onde se
resolveu **contra** a spec, aponta-se a reconciliação de §3.3 que o executa.

| Conflito | Sev. | Desfecho | Fonte que decide |
|----------|------|----------|------------------|
| **C1** — «mesmo byte» + ownership integral | bloqueante | Contra a spec: adotado o modelo de três oráculos, o passe é `oráculo 1 ∧ oráculo 2 ∧ (oráculo 3 sob ENV-24)`, e o ownership é dividido — reconciliações **1** e **2** de §3.3 | FND-05 §8.3/§8.4/§8.5, `INT-05` |
| **C2** — equivalência de `Decision` sem mecanismo de wire | alta | A spec não estava errada, estava incompleta: `Decision` é objeto de domínio que por `UPR-I11` nunca toca o wire, então o aparato de round-trip não o alcança. Classe de oráculo distinta — projeção observável, sem serialização de wire nem `payload_hash` (`H3-3`, `H3-7`, §7) | FND-03 §3.4/§8.1 |
| **C3** — «backpressure e graceful shutdown derivam de FND-04» | alta | Contra a spec: proveniência por cenário — reconciliação **3** de §3.3. Graceful shutdown deriva de §5.3/`OBX-13`; backpressure é fronteira (§3.5) | FND-04 §7.3, §5.3 |
| **C4** — obrigações de FND-07 que a spec não reconhece | alta | Contra a spec: escopo ampliado — reconciliação **4** de §3.3. `ANC-07` torna a verificação executável (isolamento por tenant, pós-condições runtime) matéria de FND-09, não de FND-08 | FND-07 §1.4/§4.4/§11.2 |
| **C5** — reparo do teste de domínio com mock de infra | média | Nenhuma edição de spec: a spec e a RFC concordam no diagnóstico e divergem só no reparo. Regra em dois passos em §2 (acoplamento no SUT → corrigir o domínio; teste mal categorizado → mover para services), alinhada a §4.5 r3/§9.2/`V29` | RFC §4.5 r3, §9.2 |
| **C6** — precondição do oráculo do `payload_hash` | média | A canonicalização **não** está aberta: FND-05 §4.3 fixa a fórmula (SHA-256 sobre os bytes transportados) e FND-06 §5 fecha a byte-preservação para hops conformes. O oráculo 2 declara a precondição como parte do enunciado (`H4-2`); hops não conformes são os vetores negativos de `H6-3` | FND-04 §6.5, FND-05 §4.3, FND-06 §5 |

`registro` — C2 e C5 resolvem-se **sem** editar a spec: em C2 a spec estava
incompleta, não errada, e a lacuna é preenchida por um oráculo próprio; em C5 a
divergência é de remediação, resolvida por esclarecimento em §2. C1, C3 e C4
resolvem-se **contra** a spec e correspondem, respectivamente, às reconciliações 1+2,
3 e 4 de §3.3. C6 dissolve uma pendência que a revisão anterior supunha aberta: a
precondição do oráculo é declarada, não uma canonicalização a reabrir.

### §3.5 O que não tinha insumo no acervo

Nem toda matéria que a spec atribuiu a FND-09 tem insumo no acervo. O caso central é
o **backpressure**: a spec o lista como cenário «derivado das sequências de FND-04»
(l. 98), mas FND-04 não o menciona em lugar nenhum — é matéria de resiliência, do
recorte de `ANC-06`. Quando esta matriz foi escrita, FND-09 não dispunha da regra de resultado que
permitiria escrever o teste, e inventá-la seria decidir matéria de outra dona. Recebeu,
então, **fronteira nomeada** a FND-08 (ARQ-445), com slot reservado e condição de
fechamento — nunca dívida condicionada nem regra inventada. A condição foi cumprida
com a publicação de FND-08, e o item passou a coberto por `CEN-44` (§4.8); a linha
abaixo permanece como registro do que estava bloqueado e do que o destravou.

| Item | Dona | Condição de fechamento | Estado |
|------|------|------------------------|--------|
| Backpressure (produção acima da capacidade de consumo) | FND-08 (ARQ-445) | Regra de resultado de resiliência promovida em FND-08 | **fechada** — `CEN-44` (§4.8) |
| Timing de retry, DLQ, quarantine | FND-08 (ARQ-445) | Idem | fronteira mantida: matéria de ANC-06, verificável por inspeção (§4.8) |
| Replay operacional | FND-08 (ARQ-445) | Idem | fronteira mantida: evidência de execução operacional, não cenário |
| Observabilidade de entrega (consumer lag, profundidade da DLQ, depth/age da outbox — lacunas de FND-01 §4.2) | FND-08 (ARQ-445) | Idem | catálogo publicado em FND-08 §6; as regras `MET` são instrumentadas em §13.7 |

`normativo` — **Fronteira declarada não é cobertura.** Reservar um slot a FND-08
**não** satisfaz item de critério de aceite: declarar o AC satisfeito por slot vazio
seria exatamente o risco que o épico nomeia, «decisões apenas documentais». A regra
vale para toda fronteira desta seção, e o histórico do backpressure a ilustra. Ele
ficou **bloqueado** enquanto FND-08 não publicava a regra de resultado, e passou a
**coberto** por `CEN-44` (§4.8) quando ela foi publicada — não por reinterpretação
aqui, mas porque a condição de fechamento que a fronteira declarava foi cumprida pela
dona. **Os seis** cenários do AC-10 têm hoje `CEN` com setup, ação e resultado (§4).
As demais fronteiras desta tabela seguem sem cenário, e a razão está em §4.8: são de
outra dona e se verificam por inspeção ou por evidência operacional.

`registro` — **Bloqueio não é o mesmo que dependência declarada.** Distinto dos itens
acima, três precondições **não** bloqueiam a obrigação, apenas condicionam a sua
execução, e por isso vivem nas linhas da matriz (`H4-2`, `H6-2`, `H6-3`) e não nesta
tabela: a fórmula do `payload_hash` (FND-05 §4.3, já fixada) somada à byte-preservação
(FND-06 §5); a base as-is prospectiva de Kafka (FND-06 `TRP-41`), que limita a
validação por execução dos vetores Kafka mas não impede especificá-los; e a
modalidade claim-check (`ANC-03`/FND-05, `ENV-20`), da qual depende o caminho conforme
de `TRP-21`. Cada uma é dependência nomeada com dona, não insumo ausente — a linha
correspondente é `quitada` com dependência declarada, não `bloqueada`. O mecanismo
está inteiro no artefato; o que a precondição condiciona é a **execução** do vetor,
não a existência da regra.

---

## §4. O catálogo de cenários distribuídos

Os artefatos precedentes normatizaram **o que precisa ser verdade** — a semântica
at-least-once com efeito idempotente (FND-04 §7.1), a byte-preservação do payload
hop a hop (FND-06 §5), a compatibilidade dentro de uma major (FND-05 §5.3). Esta
seção converte cada uma dessas verdades em algo que **falha de modo observável**
quando o desenho é violado. É o coração executável do artefato: sem ela, a
estratégia de testes é uma lista de propriedades desejáveis, e o próprio épico
nomeia esse risco — «decisões apenas documentais», estratégia que vira lista de
desejos sem nada que a execute.

O catálogo não inventa cenário. Cada `CEN` desce de um failure mode já catalogado
(FND-04 §7.3), de uma capacidade já exigida (FND-04 §5.3), de um grupo já
encaminhado (FND-06 §18.4), de uma linha da matriz de hops (FND-06 §5.2), de um
vetor de coexistência (FND-06 §15), de uma regra de compatibilidade (FND-05
`PTB-08`..`PTB-12`) ou de uma regra de contexto e multi-tenancy herdada de FND-07
(§3.4, §3.5, §4.4). A proveniência é citada em toda linha, e o cenário que não tiver
resultado decidível não entra.

### §4.1 A forma do cenário

`normativo` — Todo cenário do catálogo exibe, sem exceção, seis elementos:

| Elemento | O que fixa |
|----------|-----------|
| **Identificador** | Um ID `CEN-NN`, estável e único |
| **Proveniência** | O `#` do failure mode ou a subseção do acervo de onde o cenário desce, citada literalmente |
| **Setup** | O estado observável em que o sistema é colocado antes da ação |
| **Ação** | O gesto único e nomeado que se executa ou se inspeciona |
| **Resultado esperado** | O oráculo — uma proposição **decidível**, verdadeira ou falsa sem julgamento |
| **Herança e modo** | Os IDs herdados que o cenário prova e o modo pelo qual se verifica |

`normativo` — **Um cenário sem resultado verificável não entra no
catálogo.** «Comporta-se corretamente» ou «recupera bem» não são resultados: são
adiamentos do oráculo. O resultado é escrito de modo que um verificador leia o
estado final e conclua aprovado ou reprovado sem interpretar — «exatamente um
vencedor aplica o efeito, o outro curto-circuita, nenhum dos dois falha» é
resultado; «resolve a corrida» não é. É a exigência que separa este catálogo da
lista de desejos que o épico proíbe.

`normativo` — **O modo de verificação é declarado por cenário, e há dois.**
Um cenário `runtime-testable` só se demonstra **executando** o sistema — teste de
integração, teste de propriedade, harness de reentrega. Um cenário
`structurally reviewable` se **confere por inspeção** de código, contrato ou
configuração, sem executar. A distinção é herdada da RFC §11.1 e é vinculante:
um verificador que reporte um cenário `runtime-testable` como «aprovado» sem ter
executado está incorreto, e o resultado correto é **não verificado** (RFC
§11.3). Onde a demonstração exige um produtor que emita o defeito de propósito, o
modo é `runtime-testable com produtor instrumentado`.

`normativo` — **Todo cenário do catálogo é determinístico.** Nenhum `CEN`
depende de relógio de parede, de ordem de iteração de mapa ou de porta aleatória:
onde o tempo importa, o relógio é injetado; onde a ordem importa, ela é fixada por
chave; onde há concorrência, o cenário declara o entrelaçamento que exercita. Um
cenário não determinístico não tem oráculo — o mesmo estado final ora aprova, ora
reprova — e pela regra de forma acima não entra.

`normativo` — **O identificador `CEN` é diagnóstico estável.** Um `CEN`,
uma vez atribuído, não muda de assunto nem é reutilizado: quando um cenário sai do
catálogo, o número é aposentado, não reciclado. É a mesma disciplina que FND-06
§18.5 aplica aos seus prefixos, e é o que permite a um relatório de falha
referenciar `CEN-07` e significar sempre a mesma corrida de inbox.

`rationale` — A coluna «Herança e modo» é o elo que RFC §14.5 exige: da regra de
garantia ao teste que a exercita. Sem ela, o catálogo provaria cenários sem que
ninguém soubesse **qual invariante** cada um protege, e a rastreabilidade
garantia → teste ficaria por fazer. Com ela, cada `CEN` é uma aresta nomeada dessa
cadeia.

### §4.2 Cenários derivados dos failure modes

`normativo` — Os **doze** failure modes de FND-04 §7.3 são o insumo declarado deste
catálogo (FND-04 §7.3, bloco `encaminhado`, e §11.5, pendência 3). Cada `CEN`
abaixo cita o `#` do failure mode e preserva a **categoria de desfecho** e o **modo**
que FND-04 lhe atribuiu — a classificação não é reescrita aqui. FND-04 §7.1
encaminha a esta seção o instrumento dos vetores `V32` (efeito idempotente sob
redelivery, `runtime-testable`) e `V31` (vedação a exactly-once E2E,
`structurally reviewable`): `V32` é exercitado pelos cenários de reentrega abaixo
(`#3`, `#6`, `#12`), e `V31` pela varredura de `CEN-24` (§4.4).

`normativo` — **Nenhum dos doze resulta em perda de fato commitado.** A
perda só é possível fora do desenho — publicando no broker dentro da transação do
caso de uso, ou confirmando o consumo antes do commit —, os dois caminhos que
FND-04 §7.1 e §6.3 vedam. O resultado esperado de todo `CEN` deste grupo pressupõe
essa vedação; um cenário que produza perda reprova o desenho, não o teste.

| `CEN` | Proveniência (§7.3) | Setup | Ação | Resultado esperado | Herança e modo |
|-------|---------------------|-------|------|--------------------|----------------|
| `CEN-01` | `#1` — UoW de escrita não commita | Caso de uso prestes a persistir estado + outbox na mesma UoW | Injetar crash antes do commit (passo 8), conflito de optimistic locking (passo 6) ou falha de serialização (passo 7) | Nada persistiu: nem estado, nem registro de outbox, nem efeito parcial. O pendente é a operação de negócio, que não ocorreu; quem decide é o chamador do caso de uso | **Contenção**; `GAR-05`, `GAR-06`; `runtime-testable` |
| `CEN-02` | `#2` — crash após commit, antes de publicar | Registro de outbox commitado em `pending`, relay ainda não publicou | Derrubar o processo entre o commit e a publicação | O relay reivindica o registro `pending` no ciclo seguinte e publica; nenhuma mensagem se perde | **Recuperação**; `OBX-09`; `runtime-testable` |
| `CEN-03` | `#3` — publicou e não marcou | Relay publicou a mensagem no broker (passo 2) | Falhar antes de marcar `published` (passo 3) | Republicação no ciclo seguinte com o **mesmo** `message_id`; o consumidor deduplica pela inbox (R2) e confirma sem reaplicar o efeito | **Recuperação**; `OBX-14`, R2; `runtime-testable` |
| `CEN-04` | `#4` — lease expirado com worker vivo | Worker A reivindicou um lote e o lease expirou sem A morrer | Deixar B reivindicar e publicar o mesmo registro | O registro volta ao pool e B publica; a republicação é duplicata aceitável sob at-least-once | **Recuperação**; `OBX-09`; `runtime-testable` |
| `CEN-05` | `#5` — claimant expirado escreve tarde | Worker B já publicou e marcou `published`; A retorna do broker com claim morto | Deixar A tentar escrever `published` sobre o registro já concluído | A transição é condicional ao claim corrente: a escrita do claim morto é **rejeitada**; o registro publicado não retorna ao pool e não é republicado indefinidamente | **Recuperação**; `OBX-10`; `runtime-testable` |
| `CEN-06` | `#6` — crash após commit, antes do ACK | Consumidor commitou o efeito localmente (passo 6), ACK ainda não emitido | Derrubar o consumidor entre o commit local e o ACK (passo 7) | Redelivery da mesma mensagem; a inbox classifica **R2** e o adapter confirma sem reaplicar | **Recuperação**; R2, `INB-01`; `runtime-testable` |
| `CEN-07` | `#7` — duplicata concorrente (corrida de inbox) | Duas entregas da mesma mensagem processadas em paralelo (passo 4) | As duas transações chamam `insert-if-absent` ao mesmo tempo | **Exatamente uma** recebe R1 e aplica o efeito; a outra recebe R2 e curto-circuita; **ambas permanecem vivas** — nenhuma aborta por erro de constraint | **Recuperação**; `INB-01`, §6.2 (`insert-if-absent`); `runtime-testable` |
| `CEN-08` | `#8` — colisão de `message_id` com payload divergente | Inbox já tem um registro para o `message_id`; chega mensagem com o mesmo id e payload diferente (passo 4) | Processar a segunda mensagem | Classificação **R4**: o efeito **não** é aplicado sob identificador reutilizado, e a mensagem sai do fluxo normal. O pendente é o efeito da mensagem divergente; decide quem opera o produtor | **Contenção**; R4, `GAR-06`; `runtime-testable com produtor instrumentado` |
| `CEN-09` | `#9` — poison message | Mensagem que falha em todo processamento | Reentregar até esgotar as tentativas | Quarantine ou DLQ **sem loop** e sem bloquear partição nem grupo FIFO; o pendente é o efeito daquela mensagem; decide operação, por replay ou descarte auditado | **Contenção**; `GAR-08`, `GAR-11`; `runtime-testable com produtor instrumentado` |
| `CEN-10` | `#10` — replay conduzido a partir da DLQ | Mensagem contida na DLQ com envelope preservado (`GAR-07`) | Reprocessar por ferramenta, com auditoria, declarando a zona de proteção (FND-04 §6.6) | Reexecução auditada; dentro da retenção da inbox a proteção é dupla, fora dela a única proteção é a idempotência de efeito. Replay sem auditoria ou sem zona declarada reprova | **Reparação assistida**; `GAR-09`; **evidência de execução operacional**, não teste automatizado |
| `CEN-11` | `#11` — publicação esgota as tentativas | Relay tentou publicar e esgotou o retry (passo 3c) | Deixar o registro transitar a `failed` | O registro sai do ciclo automático **sem perda**: o fato commitado permanece na outbox e é retomável; o pendente é a publicação daquela mensagem; decide operação | **Contenção**; `GAR-07`; `runtime-testable` |
| `CEN-12` | `#12` — falha transitória de consumo que depois sucede | Consumidor sofre falha transitória (R1×D3) e faz rollback | Reentregar; a tentativa seguinte sucede | O rollback não deixou registro na inbox; a redelivery seguinte é classificada **R1** e o efeito é aplicado uma vez | **Recuperação**; R1×D3; `runtime-testable` |

`rationale` — `CEN-01` é **contenção**, não recuperação, e o critério é o de FND-04
§7.3: pergunta-se se o efeito pretendido ocorre, e nesse cenário ele não ocorre — a
operação de negócio não aconteceu, e nenhuma norma obriga o chamador a reexecutar.
O sistema conteve sem perda; classificá-lo como recuperação apoiaria a categoria em
um comportamento externo não especificado. O oráculo, portanto, verifica ausência
de estado e de registro de outbox, e **não** verifica reexecução.

`rationale` — `CEN-07` é a materialização direta da decisão D7 do plano: a corrida
de inbox **não** é lacuna do acervo, é o failure mode `#7`, já catalogado e
classificado `Recuperação`. O oráculo é preciso justamente onde a redação intuitiva
falharia — «resolve a corrida» seria indecidível; «exatamente um vencedor aplica, o
outro curto-circuita, ambos vivos» é decidível e reproduz a semântica de
`insert-if-absent` que a porta de §6.2 do FND-04 oferece.

### §4.3 Graceful shutdown

`normativo` — Graceful shutdown **não** é failure mode de §7.3. É capacidade do
drenador, exigida por FND-04 §5.3 e `OBX-13`, e desce por outra seção. É o sexto
cenário nomeado do AC-10 que **tem** base no acervo — distinto de backpressure, que
não tem (§4.8).

| `CEN` | Proveniência | Setup | Ação | Resultado esperado | Herança e modo |
|-------|--------------|-------|------|--------------------|----------------|
| `CEN-13` | FND-04 §5.3, `OBX-13` | Relay drenando um lote reivindicado, com claims vivos | Emitir sinal de encerramento durante a drenagem | O relay **para de reivindicar novos lotes**, conclui ou libera os claims vivos e só então encerra: nenhum trabalho em curso é perdido, nenhum ACK indevido é emitido, e nenhum registro é publicado pela metade. Um encerramento com claim vivo não perde mensagem — o lease expira e o registro volta ao pool — mas o oráculo exige a liberação ordenada, não a expiração | `OBX-13`; `runtime-testable` (harness de encerramento) |

`rationale` — O oráculo separa o **correto** do meramente **não catastrófico**.
Encerrar abandonando claims não perde mensagem, porque o lease expira; mas atrasa a
drenagem pelo prazo do lease sem necessidade, e `OBX-13` exige mais que a ausência
de perda — exige a liberação ordenada. O resultado esperado verifica os dois lados:
zero trabalho perdido **e** zero ACK indevido **e** liberação antes do prazo do
lease.

### §4.4 Cenários por transporte

`normativo` — FND-06 §18.4 encaminha **treze grupos** de cenário por transporte,
cada um com origem citada. Cada grupo vira um `CEN`, preservando o transporte de
cada um. Dois grupos — byte-preservação por hop (`CEN-14`) e coexistência
(`CEN-25`) — decompõem-se em vetores **negativos** que a §4.5 detalha; aqui o `CEN`
prova o lado **positivo** ou o comportamento de grupo, e remete a §4.5 para os
caminhos vedados.

| `CEN` | Proveniência (FND-06 §18.4) | Transporte | Setup | Ação | Resultado esperado | Herança e modo |
|-------|----------------------|------------|-------|------|--------------------|----------------|
| `CEN-14` | Byte-preservação por hop — §5.2 | Todos | Hop **conforme** (publicação direta Kafka; SNS→SQS com raw delivery; gRPC `application/proto`) | Publicar e consumir a mensagem pelo hop | Os bytes de `Any.value` entregues são idênticos aos publicados; o `payload_hash` recomputado no consumidor é igual ao do produtor. Os hops **não conforme** são vetores negativos em §4.5 | `TRP-13`; oráculos 2 e 3 (§8.3 de FND-05); `runtime-testable` |
| `CEN-15` | Gesto de ACK por disposição — §6.2, `TRP-27` | Kafka e SNS/SQS | Consumidor configurado para cada uma das sete disposições (R1×D1..D4, R2, R3, R4) | Forçar cada disposição e **injetar falha entre as duas ações** de cada ramo terminal | A confirmação ocorre **depois** do commit local, nunca antes (`TRP-26`); na contenção a publicação na DLQ **precede** o avanço do offset ou o delete (`TRP-30`); falha entre as ações produz redelivery ou duplicata em DLQ, **nunca perda** | `TRP-26`, `TRP-27`, `TRP-30`; `runtime-testable` |
| `CEN-16` | Commit de offset — `TRP-29` | Kafka | Lote consumido com um registro pendente no meio | Dispor contiguamente até o registro `n` e commitar | O valor commitado é `n + 1`, o offset de retomada; commitar `n` reentrega `n` indefinidamente; o registro pendente no meio fixa o teto no sucessor do último contíguo antes dele | `TRP-29`; `runtime-testable` |
| `CEN-17` | Retry inline em Kafka — `TRP-47` | Kafka | Consumidor com `enable.auto.commit=false` (`TRP-28`) e atribuição de partição ativa | Não commitar o registro em falha transitória | A ausência de commit **não** reentrega enquanto a atribuição durar; o retry é laço sobre o mesmo registro ou `pause` e `seek` ao offset dele, dentro do intervalo máximo entre buscas | `TRP-47`, `TRP-28`; `runtime-testable` |
| `CEN-18` | Perda de partição — `TRP-48` | Kafka | Consumidor processando registros de uma partição | Emitir aviso de revogação ou perda de atribuição durante o processamento | O consumidor cancela o trabalho pendente da partição e **não** commita o que sobrar; o registro é reentregue ao novo dono e a inbox o classifica; trabalho assíncrono destacado é vedado nesse caminho | `TRP-48`; `runtime-testable` |
| `CEN-19` | Os três relógios — §6.1 | Kafka e SNS/SQS | Canal com `janela_redelivery` declarada por fórmula, com os parâmetros de `TRP-24b` instanciados | Verificar `retenção_inbox ≥ janela_redelivery` (`INB-14`) | A invariante `INB-14` é verificável sobre valor conhecido; um canal que não declare a fórmula ou omita qualquer parâmetro de `TRP-24b` não a satisfaz. Replay operacional **não** entra na janela (`TRP-25`) | `TRP-23`, `TRP-24`, `TRP-24b`, `INB-14`; **inspeção** (fórmula) **+ execução** (janela efetiva) |
| `CEN-20` | Deadline propagado — §10.2 | gRPC | Cadeia síncrona de três saltos com deadline na borda | Propagar o prazo pela cadeia | O prazo restante **decresce, não reinicia** a cada salto, e a cadeia é **cancelada** em vez de exceder o limite da borda | `GRP-04`..`GRP-07`; `runtime-testable` |
| `CEN-21` | Limites de entrada — §7.1 | Todos | Teto de tamanho de mensagem declarado | Enviar mensagem acima do teto e conteúdo comprimido | A mensagem acima do teto é **contida sem decode**; conteúdo comprimido não expande além do teto declarado | `TRP-49`..`TRP-51`; `runtime-testable` |
| `CEN-22` | Ordenação por chave — §11.2, §12.2 | Kafka e SQS FIFO | Duas mensagens da mesma chave | Publicar na ordem A depois B | Entregues na ordem de publicação, dentro da partição (Kafka) e dentro do grupo (FIFO) | `KFK-05`..`KFK-09`, `SQS-04`; `runtime-testable` |
| `CEN-23` | Derivação de campos FIFO — `SQS-05`, `SQS-06`, `SQS-06b` | SQS FIFO | Envelope com identificador mais longo que o limite do transporte | Derivar `MessageGroupId` e `MessageDeduplicationId` | Os campos ficam dentro dos limites do transporte **preservando igualdade**: identificadores iguais produzem grupo e deduplicação iguais | `SQS-05`, `SQS-06`, `SQS-06b`; `runtime-testable` |
| `CEN-24` | Vedação de exactly-once — §13 | Todos | Proposta de canal | Varrer documento, contrato e configuração por promessa de entrega exactly-once fim a fim | Uma proposta que prometa exactly-once E2E é **rejeitada**; a política aplicável é ao-menos-uma-vez com consumo idempotente. É o instrumento de `V31` | `V31`, `TRP-42`..`TRP-44`; `structurally reviewable` |
| `CEN-25` | Coexistência — FND-06 §15 | Kafka e SNS/SQS | Canal sem ordenação exigida, dois adapters com `consumer_name` lógico idêntico e binding imutável por mensagem | Executar drenagem paralela: produção migra ao destino, consumo do backlog de origem continua | Durante a janela, produção em um transporte e consumo nos dois; uma redelivery cross-transport é absorvida pela inbox como **R2** (`COE-04`); o fim da janela é declarado e o backlog observável (`GAR-12`). Os cinco vetores negativos estão em §4.5 | `COE-04`, `COE-08`; `runtime-testable` |
| `CEN-26` | Contenção de envelope inválido — §7 | Todos | Mensagem com envelope inválido ou schema desconhecido | Recebê-la no consumidor | Vai para quarantine ou DLQ **sem passar por disposição** — recusada no passo 1 de FND-04 §6.3, antes da UoW — e **sem loop** | `TRP-33`..`TRP-36`, `TRP-54`; `runtime-testable` |

### §4.5 Vetores negativos de transporte

`normativo` — Um **vetor negativo** descreve um caminho que **quebra** uma
propriedade normatizada, e o resultado esperado é a **detecção** da quebra, não a
sua tolerância. Esta seção converte cada linha marcada `não conforme` na matriz de
hops de FND-06 §5.2 e cada um dos cinco vetores `V-COE` de FND-06 §15 em um `CEN`.
Cada linha `não conforme` alimenta a inbox com bytes reescritos, convertendo
redelivery legítima em `R4` (colisão de identificador) — é por isso que é vetor
negativo de FND-09: descreve um hop que **não pode** alimentar a inbox pela
comparação de `payload_hash`.

`normativo` — **`não conforme` é relativo ao caminho de mensagem que
alimente a inbox, não absoluto** (`TRP-16`). Uma superfície REST transcodificada
continua legítima como interface externa; o que ela não pode ser é o hop de uma
mensagem de domínio cuja idempotência dependa de `ENV-17`. O resultado esperado de
cada `CEN` abaixo é escrito com esse escopo.

| `CEN` | Proveniência (§5.2) | Transporte | Setup | Ação | Resultado esperado | Herança e modo |
|-------|---------------------|------------|-------|------|--------------------|----------------|
| `CEN-27` | Kafka Connect com transform (SMT) | Kafka | SMT no caminho de mensagem de domínio | O transform converte o valor — desserializa e reserializa por construção | O `payload_hash` recomputado **diverge**; a redelivery legítima chega como `R4`; o hop é rejeitado como caminho de mensagem que alimente inbox | `TRP-13`, `TRP-14`; oráculos 2 e 3; `runtime-testable` |
| `CEN-28` | SNS→SQS **sem** raw message delivery | SNS/SQS | Assinatura sem raw message delivery | Publicar via SNS e consumir da SQS | O corpo original vira campo de um JSON de notificação; quem lê o corpo como payload lê a **envoltória**, e os bytes divergem — não conforme | `TRP-13`; oráculo 3; `runtime-testable` |
| `CEN-29` | Connect em `application/json` | gRPC | Caminho JSON que faz unpack e repack de `Any` | Trafegar a mensagem por esse caminho | ProtoJSON representa `bytes` em Base64, mas o unpack/repack de `Any` **reescreve** os bytes — não conforme para mensagem que alimente inbox | `TRP-13`, `TRP-14`; oráculo 3; `runtime-testable` |
| `CEN-30` | Envoy gRPC-JSON Transcoder | gRPC | Filtro de transcodificação como hop de mensagem de domínio | Trafegar a mensagem pelo transcoder | O filtro converte JSON e Protobuf por descriptors — transcodificação por definição, que reescreve os bytes; não conforme como hop de mensagem de domínio (legítimo como interface externa, `TRP-16`) | `TRP-13`, `TRP-16`; oráculo 3; `runtime-testable` |
| `CEN-31` | `V-COE-1` — FND-06 §15 | Kafka e SNS/SQS | Dois transportes configurados; publicação confirmada, relay falha antes de marcar, binding trocado | Retry publica no **outro** transporte | Vedado por `COE-02`, detectável por `TRP-46` (o item traz o endereço da primeira tentativa); com `COE-04` a inbox absorve a segunda entrega como `R2`; o que se perde é a ordenação (`COE-06`) e o trabalho de consumo | `COE-02`, `TRP-46`, `COE-04`; `runtime-testable` |
| `CEN-32` | `V-COE-2` — FND-06 §15 | Kafka e SNS/SQS | Dois adapters, um por transporte, com `consumer_name` **diferente**, para o mesmo efeito | Processar a mesma mensagem nos dois adapters | Vedado por `COE-04`; se ocorrer, **duas entradas** na inbox e efeito duplicado — a inbox funcionando exatamente como especificada | `COE-04`, `INB-01`; `runtime-testable` |
| `CEN-33` | `V-COE-3` — FND-06 §15 | SNS/SQS → Kafka | Ponte de SQS para Kafka que reserializa o payload **dentro de `Any.value`** | Atravessar a ponte | Vedado por `COE-05` e `TRP-14`; o `payload_hash` diverge e a redelivery legítima vira `R4`. Reserializar o envelope **preservando** `Any.value` byte a byte não produz `R4`, mas viola `TRP-18` se mover atributo para header | `COE-05`, `TRP-14`, `TRP-18`; oráculos 2 e 3; `runtime-testable` |
| `CEN-34` | `V-COE-4` — FND-06 §15 | Kafka e SNS/SQS | Canal com ordenação exigida migrado **sem** drenagem | Publicar no destino com backlog de origem ainda consumível | Vedado por `COE-07`; mensagens da mesma chave existem nos dois transportes e a ordem entre eles **não existe** (`COE-06`) | `COE-07`, `COE-06`; `runtime-testable` |
| `CEN-35` | `V-COE-5` — FND-06 §15 | Kafka e SNS/SQS | Mesmo canal lógico publicado nos dois transportes «para comparar» | Publicar o mesmo canal simultaneamente nos dois | Vedado por `COE-01`: um canal lógico tem um transporte por vez; duas ordens independentes e duas linhas de redelivery para o mesmo fluxo | `COE-01`; **inspeção** (configuração) **+ execução** |

`rationale` — A matriz de §5.2, e não uma proibição geral, é o instrumento. «Não
reserialize» é verdadeiro e inútil: quem implementa um adapter não sabe que um SMT
do Kafka Connect reserializa por construção, nem que o SNS sem raw delivery embrulha
o corpo, nem que o transcoder do Envoy é transcodificação e não proxy. Cada `CEN`
acima nomeia **onde medir**, que é o que fecha a pendência 10 de FND-05 pela via
executável.

### §4.6 Compatibilidade de versão

`normativo` — As cinco regras de compatibilidade dentro de uma major — `PTB-08` a
`PTB-12` de FND-05 §5.3 — descem para o catálogo cada uma com **par de vetores**,
positivo e negativo, no formato da RFC §11.2. O eixo executável da seção é a
matriz de majors: uma fixture gerada na versão `N`, um consumidor gerado em `N-1`, e
a preservação de campos desconhecidos após reserialização. `PTB-11` é o **oráculo
executável de FND-09** (FND-05 §5.3 e §8.4); `PTB-10` é verificado na certificação
de toolchain (`BUF-07`) e pelo oráculo de identidade de bytes (§8.3, oráculo 3);
`PTB-08` e `PTB-09` são gates de `buf` (§6.6 de FND-05); `PTB-12` é revisão de PR
sobre a estrutura do repositório.

| `CEN` | Regra | Vetor positivo (setup → ação → resultado) | Vetor negativo (setup → ação → resultado) | Herança e modo |
|-------|-------|--------------------------------------------|--------------------------------------------|----------------|
| `CEN-36` | `PTB-08` — evolução aditiva | Acrescentar `optional string coupon_code = 5` a `OrderPlaced` na major `v1` → rodar `buf breaking` → passa, e o consumidor anterior segue lendo | Acrescentar campo reaproveitando o número `3` (hoje de `total_cents`) → rodar `buf breaking` → **reprova** por reuso de número de campo | `PTB-08`; gate `buf breaking` (`structurally reviewable`) |
| `CEN-37` | `PTB-09` — zero de enum | `ORDER_CHANNEL_UNSPECIFIED = 0` presente e sem uso de negócio → gerar e ler → mensagem que omite o campo não afirma canal | `ORDER_CHANNEL_WEB = 0` → o default do campo passa a ser um canal real, e toda mensagem que omita o campo afirma «web» | `PTB-09`; gate `buf lint` (`structurally reviewable`) |
| `CEN-38` | `PTB-10` — campos desconhecidos preservados | Consumidor `N-1` recebe mensagem `N` com campo novo → reserializa → o campo novo **continua nos bytes** | Consumidor cuja biblioteca **descarta** desconhecidos → reserializa → o replay perde o campo e o `payload_hash` de `ENV-17` diverge dos bytes originais | `PTB-10`, `BUF-07`, `ENV-17`; `runtime-testable` (oráculo 3) |
| `CEN-39` | `PTB-11` — consumidor `N-1` lê `N` | Produtor na `v1` com `coupon_code` preenchido → consumidor gerado **antes** de o campo existir processa a mensagem → ignora o campo, sem erro e sem perda dos campos que conhece | Produtor troca `total_cents` de `int64` para `string` → consumidor `N-1` → **falha ou trunca**, reprovado por `PTB-07` (`buf breaking`) antes de chegar a produção | `PTB-11`, `PTB-07`; **oráculo executável de FND-09** (`runtime-testable`) |
| `CEN-40` | `PTB-12` — majors coexistem | `company.orders.event.v1` e `...v2` publicados como pacotes e artefatos independentes → consumidos em paralelo → cada consumidor lê o seu | `v2` sobrescrevendo os arquivos de `v1` no mesmo diretório → o consumidor de `v1` **perde o contrato** do qual depende, sem que nenhum gate acuse | `PTB-12`, `REP-01`; revisão de PR (`structurally reviewable`) |

`rationale` — O par de vetores é o que dá diagnóstico. `PTB-10` parece detalhe de
biblioteca e é requisito de correção: sem a preservação de desconhecidos, o replay a
partir de um consumidor `N-1` reescreve os bytes e desliga a deduplicação da inbox
pela via do `payload_hash`. O vetor negativo de `CEN-38` prova exatamente esse elo —
descartar desconhecidos faz o hash divergir — que o positivo, sozinho, não
exercitaria.

### §4.7 Cenários das regras herdadas de FND-07

`normativo` — FND-07 roteou a esta âncora, sob ANC-07, um conjunto de regras
`runtime-testable` de contexto de execução e multi-tenancy — a tabela de modo de
FND-07 §11 classifica `CTX-15`..`CTX-17`, `CTX-21`, `CTX-22` e `IDN-11`..`IDN-14`
como `runtime-testable`, porque exigem execução: tempo de vida do contexto,
respeito ao cancelamento e o resultado fail-closed do isolamento. A calibração de
D10 exige que **toda regra `runtime-testable` nomeie o cenário `CEN` que a
exercita**; os três cenários abaixo existem para que a §11 tenha `CEN` para onde
apontar. A proveniência é a subseção de FND-07, e o modo é `runtime-testable` nos
três.

`normativo` — **Cifra em repouso (`DAT-08`, `DAT-10`) e teto de retenção
(`DAT-14`) não recebem `CEN`.** A verificação delas é a inspeção da superfície
armazenada sobre dado que o próprio teste grava — instrumento próprio, especificado em
§11.5 —, e não um cenário distribuído do catálogo. A
§11 os instancia como critério de inspeção, sem vetor de execução. Criar cenário de
execução para eles contrariaria a calibração de modo: o que se confere por inspeção
não vira `runtime-testable`.

| `CEN` | Proveniência | Camada | Setup | Ação | Resultado esperado | Herança e modo |
|-------|--------------|--------|-------|------|--------------------|----------------|
| `CEN-41` | FND-07 §4.4 — isolamento por tenant | `provider` / apps | Dado de negócio de dois tenants; contexto autenticado no tenant A | Acesso legítimo a dado do tenant A **e** tentativa de acesso a dado do tenant B pelo mesmo contexto | O acesso legítimo devolve o dado de A. A tentativa cross-tenant **não devolve** o dado (fail-closed), é registrada como evento de segurança com sujeito, tenant do contexto e tenant do dado alcançado (`IDN-12`), e a resposta ao chamador é **indistinguível** de recurso inexistente (`IDN-13`). O oráculo assevera sobre o **registro interno**, não sobre a resposta — é o que distingue «negado» de «vazio»: uma resposta vazia, sozinha, mascararia isolamento inexistente. Uma resposta que distinga «não encontrado» de «proibido» reprova, por ser oráculo de enumeração | `IDN-11`, `IDN-12`, `IDN-13`, `IDN-14`; `runtime-testable` (fail-closed) |
| `CEN-42` | FND-07 §3.4 — término e vedação de reuso | `application service` / apps | Handler que atende requisições de tenants distintos em sequência | Encerrar uma execução e iniciar a seguinte; observar se o contexto ou uma decisão dele derivada sobrevive | O contexto **termina** com a execução, e a requisição seguinte monta o seu (`CTX-14`, `CTX-15`): um contexto reaproveitado de pool ou de escopo mais longo reprova, ainda que nenhum campo tenha mudado. Decisão de autorização em cache só é reusada com chave que inclua sujeito **e** tenant e validade que não exceda a execução (`CTX-16`) — chave sem tenant que carregue a decisão de um tenant para outro reprova. Trabalho que continua após a resposta monta o próprio contexto, encadeado por `correlation_id`, não pelo objeto (`CTX-17`) | `CTX-15`, `CTX-16`, `CTX-17`; `runtime-testable` |
| `CEN-43` | FND-07 §3.5 — deadline e cancelamento | `app` / `application service` / `provider` | Contexto com sinal de cancelamento observável e `deadline` como instante absoluto, com I/O em andamento | Cancelar ou deixar expirar o `deadline` durante trabalho pendente | O `provider` **respeita** o sinal: nenhuma operação remota é iniciada com contexto cancelado ou já expirado, e uma chamada que apareça nos registros do dependente **depois** do instante do `deadline` reprova (`CTX-21`). O cancelamento **interrompe** o trabalho pendente e **não desfaz** efeito de negócio já commitado — a reversão, quando necessária, é ação de negócio própria, nunca consequência implícita (`CTX-22`) | `CTX-21`, `CTX-22`; `runtime-testable` |

`rationale` — O oráculo de `CEN-41` é o ponto que faz o cenário valer, e é o que
`IDN-13` sustenta. Um teste que só olhasse a resposta não separaria «o isolamento
negou o acesso» de «o dado simplesmente não existe» — as duas respostas são, por
desenho, indistinguíveis. A distinção vive no registro interno, que `IDN-12` obriga
a existir com sujeito e os dois tenants. Assertar sobre a resposta provaria menos do
que a regra exige; assertar sobre o registro prova o fail-closed. É a mesma razão
pela qual `CEN-41` prova `IDN-14` (o isolamento não depende de convenção de código)
por execução, e não a mera presença de uma condição de tenant na consulta.

`rationale` — `CEN-42` e `CEN-43` são `runtime-testable` porque a propriedade que
protegem só aparece **entre** execuções ou **durante** uma execução: o reuso de
contexto (`CTX-15`) não se vê num objeto imutável, só na segunda requisição que o
recebe; e o desrespeito ao cancelamento (`CTX-21`) só se vê na chamada que conclui
depois do prazo. Nenhum dos dois é conferível por inspeção estática, o que os separa
das regras `DAT` excluídas acima.

### §4.8 Backpressure e as fronteiras remanescentes

`normativo` — Backpressure — produção acima da capacidade de consumo — está
**ausente de todo o FND-04**. Não é failure mode de §7.3 nem capacidade de §5.3: é
matéria de resiliência, sob a âncora ANC-06, e portanto de FND-08. Este artefato não
dispunha da **regra de resultado** que permitiria escrever o teste, e por isso
declarou o item do AC-10 bloqueado, com a condição de fechamento «FND-08 publicar a
regra de resultado do controle de fluxo».

`registro` — **A condição foi satisfeita.** FND-08 ([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445))
foi publicado e fixa o desfecho observável sob saturação, e não apenas limiar,
telemetria e política:

| Regra de FND-08 | O que ela dá ao oráculo |
|-----------------|-------------------------|
| `RES-14` (§3.4) | «A fila de espera por recurso do pool é limitada, e **a saturação é rejeição rápida**» — default de fila do tamanho do pool e prazo de aquisição de 100 ms. É o resultado: o excedente é recusado, não enfileirado sem limite nem bloqueado sem prazo |
| `RES-08` (§3.2) | O teto de espera da porta de inbox tem valor declarado, e o estouro **permanece o que `INB-17` fixa** — R1×D3, transação abandonada, mensagem devolvida ao transporte com backoff. É o resultado pelo lado do **consumo** |
| `RES-16` (§3.5) | Limite de taxa e de concorrência na borda de entrada, declarados por rota e por tenant. É o setup declarável |
| `RES-17` (§3.5) | A recusa por admissão é **categorizada**, ocorre **antes** de decode, validação e acesso a banco, e produz sinal por rota e por tenant. «Recusa que só apareça no total de erros do serviço é indistinguível de falha da aplicação» — o vetor negativo |
| `MET-11`, `MET-12` (§6.2) | `pool_utilization` e `queue_depth` tornam a saturação observável; `dmpf_service_admission_rejections_total` separa «recusou por política» de «falhou» |

`normativo` — **O item de backpressure do AC-10 passa a coberto, por `CEN-44`.** O
cenário abaixo tem setup, ação e resultado decidível, e o resultado é o que FND-08
normatizou — não uma invenção deste artefato. A fronteira que existia era temporal,
não de matéria: a matéria continua sendo de ANC-06, e o que mudou é que a dona a
decidiu.

| `CEN` | Proveniência | Camada | Setup | Ação | Resultado esperado | Herança e modo |
|-------|--------------|--------|-------|------|--------------------|----------------|
| `CEN-44` | FND-08 §3.4 (`RES-14`), §3.2 (`RES-08`) e §3.5 (`RES-16`, `RES-17`); sinal em §6.2 (`MET-11`, `MET-12`). Item «Backpressure» do AC-10 | `app` (borda de entrada) e `application service` (consumo) | Serviço com pool de limite declarado e fila de espera limitada (`RES-13`/`RES-14`; default de fila igual ao pool e aquisição em 100 ms, ou o override declarado na ficha de `RES-21`); borda de entrada com limite de taxa e de concorrência por rota e por tenant (`RES-16`); porta de inbox com teto de espera declarado (`RES-08`); relógio de aquisição injetado; `MET-11` e `MET-12` observáveis, com o contador em zero para a rota e o tenant sob teste | Produtor instrumentado emite carga determinística **acima da capacidade declarada** — concorrência além do limite, com a fila de espera já cheia — pelas duas vias: a borda de entrada e o consumo de mensagens | **(a)** O excedente é recusado dentro do prazo de aquisição declarado: nenhuma requisição excedente bloqueia além dele nem é enfileirada sem limite (`RES-14`). **(b)** A recusa ocorre antes de o pipeline consumir recurso a jusante — sem decode, sem validação e sem acesso a banco para a requisição recusada (`RES-17`). **(c)** A recusa carrega categoria de admissão própria e incrementa `MET-12` por rota e por tenant, uma unidade por recusa, **sem** entrar no total de erros do serviço — recusa que apareça só ali reprova (`RES-17`). **(d)** No lado do consumo, o estouro do teto da porta de inbox resolve em R1×D3, com transação abandonada e mensagem devolvida ao transporte com backoff (`RES-08`, `INB-17`), e **não** em latência crescente. **(e)** As requisições dentro da capacidade permanecem admitidas e concluídas: a recusa é seletiva do excedente, não colapso do serviço, e `MET-11` reflete a saturação na janela | `RES-08`, `RES-14`, `RES-16`, `RES-17`; sinal por `MET-11` e `MET-12`; `runtime-testable com produtor instrumentado` |

`normativo` — **As demais matérias de resiliência e observabilidade de entrega
seguem sem cenário próprio, e a razão não é mais a ausência do artefato.** FND-08
está publicado e a condição de fechamento de cada uma foi tecnicamente satisfeita —
retry e orçamento em §4.3 e §4.4, runbook e replay em §8, métricas de entrega em §6.5.
O que as mantém fora do catálogo é o **modo**: catálogo de métricas se verifica por
inspeção, e cadência de replay por evidência de execução operacional, como `CEN-10`.
Convertê-las em cenário distribuído seria a mesma sobre-especificação que §4.7 recusa
para `DAT-08` e `DAT-10`. Nenhuma é item do AC-10. Cada uma permanece fronteira
nomeada, com dona e condição:

| Matéria | Dona | Condição de fechamento |
|---------|------|------------------------|
| Timing de retry, DLQ e quarantine (limiares, prazos de backoff, limites de fila) | FND-08 (ARQ-445) | FND-08 publicar o catálogo de operação e os limiares |
| Replay operacional — runbook e cadência | FND-08 (ARQ-445) | FND-08 publicar o runbook de replay. O **mecanismo** de replay (auditoria, zona de proteção) já é coberto por `CEN-10`; o que fica bloqueado é a operação temporizada |
| Observabilidade de entrega (consumer lag, profundidade da DLQ, `depth`/`age` da outbox) | FND-08 (ARQ-445) | FND-08 publicar o catálogo de métricas, nomes, unidades e alarmes |

`rationale` — A distinção entre `CEN-10` e a linha de replay operacional acima é
deliberada e evita contradição. FND-04 §7.4 (`GAR-09`) exige que o replay preserve
informação suficiente, seja auditado e declare a zona de proteção — propriedade do
mecanismo, que `CEN-10` prova por evidência de execução. O **timing** do replay — de
quanto em quanto tempo, sob qual alarme, com qual runbook — é catálogo de operação,
de FND-08. A primeira afirmação faz o cenário existir; a segunda o torna operável, e
é de outra dona. Inventar a segunda aqui violaria a fronteira que ANC-06 estabelece
e reproduziria o defeito que D4 do plano proíbe.

### §4.9 Correspondência com o AC-10

`registro` — O AC-10 do épico lista seis cenários, e **os seis recebem `CEN`**. Cinco
têm base no acervo desde a redação; o sexto — backpressure — passou a tê-la quando
FND-08 publicou a regra de resultado sob saturação (§4.8). A tabela fecha o gate da
Fase 2 do plano: cada item mapeado ao `CEN` que o cobre e ao seu estado. Os cenários `CEN-41`
a `CEN-43` (§4.7) **não** são itens do AC-10 — cobrem as regras `runtime-testable`
herdadas de FND-07 e servem à rastreabilidade de D10 exercida em §11 —, e por isso
não aparecem nesta tabela.

| Item do AC-10 | Proveniência | `CEN` que o cobre | Estado |
|---------------|--------------|-------------------|--------|
| Commit antes do ACK | FND-04 §7.3, failure modes `#1` e `#6` | `CEN-01`, `CEN-06` | **coberto** |
| Redelivery | FND-04 §7.3, failure mode `#12` | `CEN-12` | **coberto** |
| Duplicata concorrente | FND-04 §7.3, failure mode `#7` | `CEN-07` | **coberto** |
| Poison | FND-04 §7.3, failure mode `#9` | `CEN-09` | **coberto** |
| Graceful shutdown | FND-04 §5.3, `OBX-13` | `CEN-13` | **coberto** |
| Backpressure | FND-08 §3.4 (`RES-14`), §3.2 (`RES-08`), §3.5 (`RES-16`/`RES-17`) | `CEN-44` | **coberto** |

`normativo` — **Os seis itens do AC-10 têm cenário `CEN`** com setup, ação e
resultado decidível. O catálogo completo desta seção vai de `CEN-01` a `CEN-44`,
sequencial e sem lacuna: `CEN-01`..`CEN-40` cobrem cinco dos itens do AC-10, os
transportes e a compatibilidade de versão; `CEN-41`..`CEN-43` cobrem as regras
`runtime-testable` herdadas de FND-07, fora do AC-10; e `CEN-44` cobre o sexto item,
o backpressure, com a regra de resultado de FND-08. O prefixo `CEN` é reservado a cenários; as regras
de forma e de fronteira desta seção são blocos `normativo` sem ID próprio, no padrão
do acervo.

---

## §5. As golden fixtures Go ↔ TypeScript

O critério de aceite do épico (AC-10) pede a evidência de que «um contrato de
exemplo serializado numa stack é desserializado na outra sem perda semântica». A
fonte dessa evidência é a **golden fixture**. FND-05 §8 já especificou **o que a
fixture é** e **o que ela prova**; esta seção especifica o que compete a FND-09 —
o **formato do arquivo**, a **fonte única**, o **pipeline** que roda a fixture e o
**metadado de versão** — e reconcilia, com a posição de FND-05 em mãos, a segunda
das duas divergências que FND-05 §8.5 registrou como handoff a este artefato (a
reivindicação de ownership integral pela spec). A primeira divergência — «mesmo
byte» como critério do round-trip — é matéria de oráculo e é reconciliada em §6.1.

A separação de ownership é o eixo desta seção, e é o ponto em que a leitura
anterior errava: FND-09 é dona do **arquivo e do mecanismo**, nunca do **conteúdo
do contrato**. Confundir os dois faz a estratégia de teste normatizar o que
pertence ao perfil de envelope, e o excesso seria inválido por M4 ainda que
tecnicamente correto.

### §5.1 Formato, fonte única e pipeline

`normativo`

| ID | Regra |
|----|-------|
| `FIX-01` | O **formato de arquivo** da golden fixture é normatizado por este artefato. A fixture é um arquivo declarativo, versionado no repositório de contratos, que descreve de forma completa e **independente de stack** uma instância do contrato. O que a fixture **deve conter** — os quinze atributos de envelope de `ENV-08` com valores fixos, os três condicionais (`aggregateversion`, `tenantid`, `tracestate`) em ambos os estados do predicado, o payload campo a campo, o enum no valor `UNSPECIFIED` e um caso de campo desconhecido — é fixado por FND-05 `INT-01` e §8.2 e **não** é redefinido aqui |
| `FIX-02` | A fixture é a **fonte única** das duas stacks: o mesmo arquivo alimenta a suíte Go e a suíte TypeScript, e nenhuma stack mantém cópia própria (requisito não-funcional P0 da spec; FND-05 `INT-01`). A divergência de fixture entre stacks fica, por construção, impossível: não há dois artefatos para divergir |
| `FIX-03` | O **pipeline** de round-trip é **bidirecional**. Numa direção, o produtor Go serializa a partir da fixture, o consumidor TypeScript desserializa os bytes, e o resultado é comparado pelos oráculos de §6; na outra, TypeScript produz e Go consome. As duas direções são exigidas, e o passe em uma **não** dispensa a outra (FND-05 `INT-03`) |
| `FIX-04` | A fixture declara, como metadado próprio, **quais majors do perfil e do contrato ela cobre**. É o instrumento da exigência «versões suportadas» do AC-10. O metadado é parte do formato de arquivo e, portanto, de FND-09 |

`rationale` — A bidirecionalidade de `FIX-03` não é simetria decorativa. FND-05
`INT-03` registra que assimetrias de serialização de campo opcional e de valor
default aparecem **apenas** no sentido não testado: uma stack que omita um campo
com valor default e outra que o emita produzem bytes diferentes para o mesmo
conteúdo, e o defeito permanece invisível enquanto só uma delas produzir. Rodar
uma única direção esconde metade da classe de defeitos que a fixture existe para
capturar.

`rationale` — O metadado de versão de `FIX-04` fecha uma lacuna concreta: sem ele,
uma fixture escrita para a major *N* de um contrato seria rodada em silêncio contra
a major *N+1* gerada, e o round-trip passaria ou falharia por uma razão que ninguém
declarou. O metadado torna a cobertura de versão uma afirmação verificável do
arquivo, não uma suposição do pipeline.

### §5.2 Os três owners

`registro` — Três responsáveis distintos aparecem no cenário da golden fixture, e a
fronteira entre eles é a que FND-05 §8.4 fixou. Este artefato a reproduz sem
alterá-la, e a torna operativa pelo lado do mecanismo:

| Owner | Responsabilidade |
|-------|------------------|
| **FND-05** (perfil de envelope, sob ANC-03) | O contrato de exemplo, o **conteúdo obrigatório** da fixture (§8.2) e a **definição dos três oráculos e do seu escopo** (§8.3) |
| **FND-09** (este artefato, sob ANC-07) | O **formato de arquivo** da fixture, o **oráculo executável**, o **pipeline** que a roda, o **diagnóstico** que ela emite e o **metadado de versão** (`FIX-01`..`FIX-04`) |
| Épicos de kernel e de contratos | A **implementação** do oráculo e do pipeline nas duas stacks e a **execução efetiva** do cenário, com a evidência de runtime registrada |

`normativo` — Este artefato **não** declara o round-trip executado. Um artefato
normativo não produz evidência de runtime; o critério do AC-10 permanece **aberto**
até que os épicos de execução o satisfaçam, e a cadeia de rastreabilidade (§10)
registra o critério como pendente de execução, não como cumprido aqui.

`registro` — **Reconciliação da divergência 2 de FND-05 §8.5.** A spec
[SPEC-6RQBN98G](../specs/SPEC-6RQBN98G-dmpf-testes-interop.md) reivindicava para
FND-09 «formato, ownership e pipeline» das fixtures — uma reivindicação de
ownership **integral**. FND-05 §8.4 divide: o conteúdo obrigatório e a definição
dos oráculos são de FND-05; o formato de arquivo, o oráculo executável, o pipeline
e o diagnóstico são de FND-09. Este artefato **adota a divisão de §8.4**: o termo
«ownership» da spec fica estreitado ao **arquivo e ao mecanismo** da fixture, nunca
ao **conteúdo do contrato**. A reconciliação encerra o handoff registrado em FND-05
§8.5 pelo lado de FND-09.

### §5.3 O formato do arquivo

`normativo` — O AC-10 exige que a golden fixture tenha «formato, ownership e
pipeline definidos». O ownership e o pipeline estão em §5.1 e §5.2; esta subseção
especifica o **formato** de fato, que FND-05 §8.4 atribui a FND-09. O conteúdo que a
fixture carrega continua sendo de FND-05 §8.2; o que se fixa aqui é a **estrutura do
arquivo** e os **requisitos de forma** que o tornam utilizável pelas duas stacks.

| ID | Regra |
|----|-------|
| `FIX-05` | O arquivo de fixture tem **seções obrigatórias**: **(a) identificação** — a fixture e o contrato qualificado que ela instancia (bounded context, categoria e major, na forma de `PTB-01`); **(b) envelope** — os quinze atributos de `ENV-08` com valores fixos, incluindo os três condicionais em ambos os estados do predicado; **(c) payload** — campo a campo, na forma do contrato; **(d) casos discriminatórios** — os de precisão de tipo exigidos por `ORA-08` (§6.2); **(e) metadado de versão** — as majors cobertas, por `FIX-04`. O **conteúdo** de (b), (c) e (d) é fixado por FND-05 §8.2 e `ENV-08`; o **seccionamento** é de FND-09 |
| `FIX-06` | A sintaxe do arquivo é **JSON**. A escolha é normativa e não é preferência de estilo: JSON tem parser na biblioteca padrão de Go e de TypeScript, sem dependência externa em nenhuma das duas; não tem construção específica de linguagem; e não depende de indentação significativa nem de âncoras, que são as armadilhas que fariam duas stacks lerem o mesmo arquivo de formas diferentes |
| `FIX-07` | **Todo valor escalar é representado como string JSON** — número, booleano, timestamp e decimal inclusive. A regra existe porque o número JSON é ambíguo entre implementações: em TypeScript ele se torna um `double` de dupla precisão, e um inteiro de 64 bits ou um decimal monetário perdem precisão silenciosamente na leitura. Como a fixture é a fonte de verdade dos **casos discriminatórios** de precisão de tipo (`ORA-08`, §6.2), um formato que corrompa o valor na entrada destruiria exatamente o que o caso existe para provar. A conversão do texto ao tipo do contrato é do carregador de cada stack, e o texto é o que a revisão de código lê |
| `FIX-08` | O arquivo satisfaz, além da sintaxe, **requisitos de forma**: é legível e **diffável** em revisão de código, e carrega **apenas valores fixos e literais** — nada computado no momento da execução (relógio de parede, ordem de `map`, identificador aleatório), em conformidade com o requisito não-funcional P0 de determinismo. A **documentação** de um caso — por que aquele valor existe, o que ele discrimina — vive em campo próprio da seção que o contém, porque JSON não admite comentário |
| `FIX-09` | O arquivo declara a **versão do próprio formato**, distinta das majors que a fixture cobre (`FIX-04`). Um carregador que encontre versão de formato que não conhece **falha**, em vez de adivinhar: ler uma fixture nova com carregador antigo produziria conformidade falsa, e é o oposto do que a fixture existe para provar |
| `FIX-10` | A fixture **vive no repositório de contratos**, sob `fixtures/`, e o seu caminho **espelha** o do contrato que instancia (mesma convenção de `REP-01`), enraizado em `fixtures/` como a árvore de FND-05 §7.1 mostra: `fixtures/<bounded_context>/<categoria>/<major>/<nome>.golden`. A extensão `.golden` e a posição são as de FND-05 §8.2 e §7.1; a fixture acompanha o PR que publica ou altera o contrato (§7.3, passo 4; `INT-02`) |
| `FIX-11` | A **identidade estável** de uma fixture é o contrato qualificado que ela instancia **mais a major** (`REP-01` + `FIX-04`). Há **exatamente uma** golden fixture canônica por contrato-major, no caminho espelhado de `FIX-10`; uma segunda fixture para o mesmo contrato-major, ou fora do caminho espelhado, é ambígua e **não conforme**. `INT-02` exige ao menos uma fixture por contrato de evento; a unicidade de caminho de `FIX-10` é o que impede duas |

`rationale` — **Por que a sintaxe é decidida aqui, e não reservada.** FND-05 fixou a
**extensão** (`.golden`, §8.2) e a **localização** (§7.1), e atribuiu o **formato** a
FND-09 (§8.4). Reservar a sintaxe aos épicos que implementam o oráculo seria devolver
ao implementador a decisão que esta âncora recebeu — e a consequência é concreta: sem
sintaxe normativa, o épico Go e o épico TypeScript decidiriam separadamente
representação de tipo, *escaping*, ausência frente a nulo, cardinalidade e tratamento
de campo desconhecido, produziriam parsers incompatíveis e **ambos** poderiam alegar
conformidade com os requisitos de forma. A fixture deixaria de ser fonte única no
único ponto em que isso importa. «Textual, declarativo e seccionado» é requisito de
forma, não formato interoperável.

`rationale` — **Por que JSON, e não textproto ou YAML.** As três sintaxes satisfazem os
requisitos de forma; a diferença está na simetria entre as stacks. Textproto tem
afinidade direta com o conteúdo que a fixture instancia e admite comentário, mas o
suporte em TypeScript é sensivelmente mais fraco que em Go — assimetria que contraria
o princípio da fonte única. YAML é mais legível, mas depende de biblioteca externa nas
duas stacks e traz normalização implícita de tipo e âncoras, que são precisamente as
armadilhas que a fixture não pode ter. JSON não é a opção mais expressiva; é a que
duas stacks leem do mesmo modo com o que já têm na biblioteca padrão. A perda — não
haver comentário — é compensada por campo de documentação próprio (`FIX-08`), e a
armadilha que restaria — o número ambíguo — é fechada por `FIX-07`.

### §5.4 O diagnóstico do round-trip

`normativo` — FND-05 §8.4 dá a FND-09 «o diagnóstico que ela emite», e a nota de
FND-05 §1.4 confirma que qual diagnóstico o instrumento emite é de ANC-07. `ORA-06`
já exige que os três oráculos sejam reportados em separado; esta subseção fixa a
**forma** do diagnóstico.

| ID | Regra |
|----|-------|
| `FIX-12` | Todo diagnóstico de **reprovação** do round-trip exibe, no mínimo: **a fixture** (identidade de `FIX-08`), **a direção** do round-trip (Go → TS ou TS → Go), **o oráculo** que reprovou (1, 2 ou 3), **o campo ou atributo divergente**, **o valor esperado frente ao obtido** e **o código estável** da reprovação (`DMPF-R001`, `DMPF-R002` ou `DMPF-R003`, conforme §10.3). É a instanciação do requisito não-funcional P0 de falha informativa — a divergência aponta o campo, não apenas «os hashes diferem» — e do diagnóstico separado por oráculo de `ORA-06` / `INT-04` |
| `FIX-13` | O diagnóstico é **estável**: a mesma reprovação produz o mesmo código, determinístico e reproduzível, entre execuções e entre stacks. O identificador vem da família **`DMPF-R`**, catalogada em §10.3 — e não do namespace `RAS`, que nomeia as **regras** desta seção, não os diagnósticos. A estabilidade é o que permite a `RAS-04` exigir que um vetor negativo nomeie o código que a reprovação deve emitir |

`registro` — Não há família de código estável do acervo aplicável ao round-trip. Os
diagnósticos `DMPF-D001`/`DMPF-D002` e `DMPF-E001`..`DMPF-E004` da RFC §10.3
cobrem a **regra de dependência** e os seus erros — matéria de teste de arquitetura
(§9) —, não a divergência de serialização entre stacks. Por isso a forma do
diagnóstico de round-trip é fixada aqui (`FIX-12`, `FIX-13`) como matéria própria de
FND-09, e o identificador estável correspondente é atribuído na §10, onde a
rastreabilidade instancia diagnóstico e par de vetores por regra.

---

## §6. Os três oráculos e a precisão de tipos

O round-trip não tem **um** critério de sucesso: tem três, e eles são
independentes. FND-05 §8.3 os definiu e fixou o escopo de cada um; §6.1 reproduz
esse regime **exato** — sem reduzi-lo a «mesmo byte» — e reconcilia a primeira
divergência de FND-05 §8.5. §6.2 trata da precisão de tipos, e o faz sob uma
restrição dura: a fixture de precisão **não escolhe** representação de wire nova; onde
a codificação ainda não foi decidida por FND-05, ela declara uma dependência e para.

### §6.1 Os três oráculos e o escopo de cada um

`normativo` — O regime dos três oráculos, e o escopo de cada um, são de FND-05
§8.3. Este artefato os reproduz para pendurar neles o oráculo executável, o
pipeline e o diagnóstico:

| # | Oráculo | O que compara | Onde é exigido |
|---|---------|---------------|----------------|
| 1 | **Equivalência semântica** | Cada atributo de envelope e cada campo de payload recuperado na stack consumidora é igual ao declarado na fixture — atributo a atributo, campo a campo | **Sempre, nas duas direções** |
| 2 | **Igualdade do `payload_hash`** | O hash recomputado por `ENV-17` na stack consumidora é igual ao computado na produtora sobre os mesmos bytes | **Sempre, nas duas direções** |
| 3 | **Identidade de bytes de `Any.value`** | Os bytes de `Any.value` recebidos são idênticos aos publicados | **Apenas** onde `ENV-24` a exige: publicação sem reserialização, contenção e replay |

`normativo`

| ID | Regra |
|----|-------|
| `ORA-01` | O passe do round-trip é `oráculo 1 ∧ oráculo 2 ∧ (oráculo 3 quando ENV-24 se aplicar)`. Reduzir o passe ao oráculo 2, ou tratar «mesmo byte» como critério geral, é leitura incorreta de FND-05 §8.3 |
| `ORA-02` | O **oráculo 1** (equivalência semântica) é avaliado **sempre e nas duas direções**. Ele compara o valor recuperado com o valor declarado na fixture, atributo a atributo e campo a campo; a igualdade é semântica, não de bytes |
| `ORA-03` | O **oráculo 2** (igualdade do `payload_hash`) é avaliado **sempre e nas duas direções**, sob a precondição de `ORA-07`. Ele compara o hash do produtor com o hash recomputado pelo consumidor **sobre os mesmos bytes** da mesma mensagem — nunca entre dois produtores independentes |
| `ORA-04` | O **oráculo 3** (identidade de bytes de `Any.value`) é exigido **apenas** onde `ENV-24` o exige: no caminho de publicação sem reserialização, na contenção e no replay. Fora desse escopo, ele **não** é critério de passe do round-trip |
| `ORA-05` | `INT-05` restringe **somente** o oráculo 3 entre **produtores independentes** da mesma mensagem lógica: duas stacks que serializem o mesmo conteúdo a partir da fixture podem produzir bytes diferentes sem violar nada, porque Protobuf não oferece forma canônica normativa entre implementações. Entre produtores independentes **não se comparam bytes entre si**; os oráculos 1 e 2 rodam **separadamente** em cada fluxo produtor → consumidor |
| `ORA-06` | Os três oráculos são avaliados e **reportados em separado** (`INT-04`), e cada reprovação emite o **código estável** da sua própria classe, definido em §10.3: oráculo 1 reprova como **`DMPF-R001`**, oráculo 2 como **`DMPF-R002`**, oráculo 3 como **`DMPF-R003`**. São diagnósticos distintos porque apontam causas distintas: `DMPF-R003` indica reserialização no caminho, `DMPF-R001` indica divergência de contrato ou de geração. Um mecanismo que emita um código único de «round-trip falhou» viola esta regra. O diagnóstico aponta **o campo divergente**, não apenas «os hashes diferem» (requisito não-funcional P0 de falha informativa) |
| `ORA-07` | O oráculo 2 é válido **apenas** sob duas condições, ambas externas a este artefato: **(a)** a fórmula de FND-05 §4.3 — SHA-256 sobre os bytes de `Any.value` **como transportados**, sem desserializar nem reserializar (`ENV-17`..`ENV-20`); e **(b)** a byte-preservação de FND-06 §5, que fecha a condição **apenas para hops conformes**. Hops não conformes — que truncam, reserializam, reordenam campos ou normalizam valores, contra `ENV-24` — são os **vetores negativos** do oráculo, não falhas dele |

`registro` — **A canonicalização do `payload_hash` NÃO é pendência aberta.** FND-05
§4.3 abre declarando que **quita** a obrigação delegada por FND-04 §6.5, e decide o
algoritmo, a canonicalização, o escopo dos bytes e o versionamento em `ENV-17`..`ENV-20`.
A matriz de obrigações de FND-05 §2.3 registra essa obrigação como
`quitada, com H1 condicionada`. O único elemento que permanece condicionado é a
**hipótese H1** de FND-04 `INB-13` — «as stacks veem os mesmos bytes» —, cuja
condição é a byte-preservação por transporte; e essa condição FND-06 §5 declara
**quitada para os hops conformes** da sua matriz de hops. Portanto FND-09 **não
reabre** a fórmula: torna a precondição parte do enunciado do oráculo 2 (`ORA-07`)
e declara a dependência como handoff nomeado, nunca como matéria a decidir aqui.

`rationale` — A separação em três oráculos é o que dá **diagnóstico** ao round-trip.
Com um critério único — «os bytes conferem» —, toda falha produz a mesma mensagem, e
a causa pode ser um campo renomeado, um plugin de geração desatualizado, um gateway
que reserializa ou uma biblioteca que descarta desconhecidos. Com três, a combinação
de resultados aponta a família da causa antes de qualquer investigação. É também por
isso que exigir identidade de bytes fora do escopo de `ENV-24` seria um erro material:
o teste reprovaria cenários legítimos, e a «correção» proposta seria relaxar o oráculo
2 — justamente a regra cuja violação desliga a deduplicação da inbox (FND-04 §6.5).

`registro` — **Reconciliação da divergência 1 de FND-05 §8.5.** A spec justificava a
fonte única prometendo que Go e TS «concordem sobre o mesmo byte» como propriedade
**geral** do round-trip. FND-05 §8.3 e `INT-05` contestam: identidade de bytes é o
**terceiro** de três oráculos e não é exigível entre produtores independentes. Este
artefato adota o regime de três oráculos e reescreve o critério: o passe é `ORA-01`,
e a identidade de bytes fica restrita ao escopo de `ENV-24` (`ORA-04`). A spec
**acerta** o critério operacional — o passo de comparação confere «campos e payload
hash» (oráculos 1 e 2) —; o que se corrige é a **justificativa** de «mesmo byte» em
geral. A reconciliação encerra o handoff de FND-05 §8.5 pelo lado de FND-09.

### §6.2 Precisão de tipos

`normativo` — A prova de round-trip só é forte se a fixture exercitar os pontos onde
Go e TypeScript divergem por construção. FND-09 exige, portanto, fixtures
**discriminatórias** de precisão de tipo — mas sob a restrição dura de que a fixture
**não decide** codificação de wire: ela ou **afirma** uma codificação que FND-05 já
fixou, ou **reserva o slot** e declara a dependência.

| ID | Regra |
|----|-------|
| `ORA-08` | A golden fixture inclui, obrigatoriamente, casos **discriminatórios** para: **timestamps**, **decimais** (representação monetária e precisão), **enums** (valor conhecido, zero-value proto / `UNSPECIFIED`, e valor **desconhecido**) e **campo desconhecido**. Um caso feliz típico não prova as regras que mais dependem de verificação cross-stack |
| `ORA-09` | Uma fixture discriminatória só fixa a codificação que **FND-05 já decidiu**. Onde a codificação de um tipo ainda **não** foi decidida por FND-05, a fixture **não escolhe** representação: ela reserva o caso e declara dependência nomeada a **ANC-03 / FND-05** (`ORA-11`) |
| `ORA-10` | **Enum e campo desconhecido têm a codificação decidida por FND-05.** O tratamento de campo desconhecido é `PTB-10`; o enum no valor `UNSPECIFIED` e o caso de campo desconhecido já são conteúdo obrigatório da fixture por FND-05 §8.2. A fixture discriminatória **afirma** essa codificação: valor de enum conhecido, zero-value / `UNSPECIFIED`, e um valor de enum **desconhecido** (número fora do conjunto gerado), mais o campo desconhecido de `PTB-10` |
| `ORA-11` | **Timestamps e decimais NÃO têm codificação cross-contrato fixada por FND-05.** A representação temporal de payload (por exemplo, `time.Time` de Go frente a `Date` de TypeScript) e a representação de valores decimais/monetários são decisões de **camada de wire** ainda não decididas. FND-09 **não as fixa**: a fixture reserva o caso discriminatório e declara dependência nomeada a **ANC-03 / FND-05**. Quando FND-05 fixar a codificação, a fixture passa a afirmá-la sob `ORA-10`, sem edição deste artefato |
| `ORA-12` | O caso discriminatório distingue **campo em zero-value** de **campo ausente** — a assimetria que FND-05 `INT-03` nomeia: uma stack que omita o valor default e outra que o emita produzem bytes distintos para o mesmo conteúdo. Essa distinção é semântica de proto3 (fixada pelo codec, não por este artefato) e é o insumo que torna os oráculos 1 e 3 falsificáveis nesse ponto |
| `ORA-13` | A decisão de codificação de tipo **vive na camada de wire e é de FND-05**, sob ANC-03. FND-03 §8.1 recusa deliberadamente fixar largura de tipo numérico no **domínio** — «fixá-la escolheria uma stack» —, e FND-09 **não** a fixa no teste. Afirmar «FND-09 fixa a codificação de X» é defeito, mesmo quando conveniente para escrever um oráculo de bytes |

`rationale` — A restrição de `ORA-09` a `ORA-13` protege a fronteira entre teste e
contrato. A interop com `payload_hash` e identidade de bytes **exige** que a
codificação de timestamps, decimais e enums seja fixada em algum lugar — mas fixá-la
na fixture faria de FND-09 a autora de uma representação de wire, que é matéria de
FND-05 sob ANC-03. Fixar a codificação de um tipo é escolher uma stack; FND-03 §8.1
recusa isso no domínio pela mesma razão pela qual FND-09 o recusa no teste. A saída é
a dependência declarada: a fixture nomeia o que precisa ser decidido e por quem, e
para, em vez de decidir por inércia.

`registro` — Enquanto a codificação de timestamps e decimais permanecer não fixada
por FND-05, os casos discriminatórios correspondentes de `ORA-08` ficam **reservados**
e não são exercidos como oráculo de bytes (`ORA-11`). Isso não bloqueia o round-trip
para os tipos já decididos — enum, campo desconhecido e os atributos de envelope de
§5.1 rodam normalmente; apenas os slots dependentes de decisão de wire aguardam a
dona nomeá-la.

---

## §7. A equivalência do desfecho de domínio

`normativo` — obrigações 10 e 11 da §3; decisão D5. Esta seção define uma
**classe de oráculo distinta** das três de §6: a que prova que Go e TypeScript
produzem o **mesmo desfecho de domínio** para a mesma entrada. Os prefixos de
regra são `ORA`, na faixa `ORA-30` em diante — a faixa `ORA-01`..`ORA-13`
pertence aos oráculos de wire de §6.

A distinção não é de ênfase, e sim de objeto. Os oráculos de §6 observam bytes
na fronteira de wire; o oráculo desta seção observa o desfecho `Decision` na
fronteira de domínio, que nunca vira bytes. As duas seções compartilham o prefixo
`ORA` porque as duas produzem um veredicto de equivalência entre stacks; separam-se
porque o mecanismo de uma não alcança o objeto da outra.

### §7.1 Por que o aparato de wire não serve

`rationale` — FND-03 §10.4 encaminha a FND-09 a prova de que as duas realizações
do desfecho — a de Go e a de TypeScript — produzem observações equivalentes na
fronteira. A tentação imediata é reusar o aparato de §5 e §6: materializar os
quatro exemplos de FND-03 §8 como uma golden fixture serializada, rodar o
round-trip e comparar `payload_hash`. Três fatos independentes tornam esse
caminho inviável, e cada um sozinho já bastaria.

`registro` — **Primeiro: `Decision` não tem forma de wire.** A invariante
`UPR-I11` de FND-03 §2.2 fixa que a UPR "não recebe nem produz tipos gerados de
Protobuf" — o desfecho é objeto exclusivamente de domínio e, por construção, nunca
toca o wire. O aparato de §5 e §6 (fixture serializada, `payload_hash`, identidade
de bytes) opera sobre mensagens que têm codificação de wire declarada por FND-05.
Um objeto sem forma de wire não é serializável por esse aparato sem que se lhe
invente uma — e inventá-la seria decidir, aqui, matéria que `UPR-I11` proíbe a
própria UPR de tocar. A contraprova de FND-03 §8.4 (o tipo gerado de contrato usado
como estado de domínio, recusado por `UPR-I11`, `CTR-02` e P0-2) é exatamente o
anti-padrão que uma fixture de `payload_hash` para o desfecho reintroduziria.

`registro` — **Segundo: fixar a codificação contraria FND-03 §8.1.** Os exemplos
de FND-03 §8, que §10.4 nomeia como "base das fixtures", são deliberadamente
abstratos quanto a tipo numérico: a tabela de convenções de leitura de §8.1 diz
"Números são abstratos; nenhum exemplo fixa largura de tipo numérico", e justifica
— "Fixá-la escolheria uma stack". Materializar esses exemplos como fixture de
`payload_hash` exigiria fixar exatamente a codificação e a largura de tipo que o
artefato irmão recusa fixar. O `rationale` de FND-03 §8.1 é explícito: a paridade
que o épico pede é **conceitual, não sintática**, e um exemplo escrito no idioma de
uma stack vira, na prática, a especificação que os dois kernels tentariam
reproduzir. Um oráculo de bytes tornaria a norma sintática à revelia da fonte.

`registro` — **Terceiro: a camada de domínio da pirâmide é em memória e
single-stack.** A base da pirâmide de §2 executa o domínio sem infraestrutura, e o
critério da RFC §9.1 exige que a `domain library` seja "executável e testável em
memória", sem processo externo, rede, banco, broker, arquivo, relógio de parede nem
entropia. Não há, nessa camada, o hop produtor→consumidor cross-stack que o
round-trip de §6 pressupõe. O desfecho é comparado onde é produzido: na fronteira,
em memória.

`normativo` — Os três fatos não dispensam a prova; FND-03 §10.4 a encaminha e a
obrigação 10 da §3 a registra como **bloqueante**. O que eles estabelecem é que a
prova exige uma **classe distinta** de oráculo — não uma adaptação do round-trip de
wire. A tensão está registrada como conflito C2 na pesquisa de viabilidade, e é
esta seção que a resolve. O aparato de §6 permanece intocado; §7 acrescenta uma
quarta classe ao lado das três de lá.

### §7.2 A fixture de projeção observável

`normativo` — bloco `domain library`. A classe de oráculo desta seção é a
**fixture de projeção observável**. Ela não usa serialização de wire e não usa
`payload_hash`. Isso a distingue, por construção, dos três oráculos de §6, e a
distinção é normativa: um vetor desta seção que serialize o desfecho, ou que
compare bytes, está fora de escopo e é defeito.

| ID | Regra |
|----|-------|
| `ORA-30` | O desfecho de domínio é verificado por **fixture de projeção observável**: um arquivo único descreve a **entrada** (estado e requisição, mais o estado observável do alvo antes da chamada) e a **projeção esperada** do desfecho. O arquivo não carrega bytes serializados nem `payload_hash` |
| `ORA-31` | A **projeção observável** é o conjunto do que um observador obtém na fronteira: o ramo (`Accepted` ou `Rejected`); em `Accepted`, a resposta de domínio; em `Rejected`, a rejeição tipada com código estável no formato `contexto/motivo`; e, nos dois ramos, a **sequência ordenada de eventos de domínio** (vazia em `Rejected`, por `DEC-11`) |
| `ORA-32` | **Não** entram na projeção: representação em memória, ordem de avaliação interna, identidade de instância, e o mecanismo sintático da realização (união discriminada, par com discriminante, tipo de resultado — qualquer um da tabela de FND-03 §3.4 é conforme). O único estado interno observável é o **estado do alvo**, e só quanto à pós-condição de `DEC-10` |
| `ORA-33` | A projeção é escrita em **codificação abstrata, neutra a stack**. Nenhuma largura de tipo numérico é fixada (FND-03 §8.1); nenhum campo usa recurso disponível em apenas uma stack. Números são abstratos, como nos exemplos de FND-03 §8 |
| `ORA-34` | **Determinismo** (FND-03 §2.4): fixados a entrada e o estado, a projeção é sempre a mesma — a mesma variante, a mesma resposta e a **mesma sequência** de eventos, na mesma ordem. A projeção não depende de relógio de parede, de ordem de iteração de mapa nem de identidade de instância. Duas execuções, na mesma stack e entre stacks, são **observacionalmente iguais** |
| `ORA-35` | As duas stacks **executam o domínio em memória** (RFC §9.1) e asseveram a projeção **na fronteira**, comparando projeção contra projeção — nunca byte a byte |
| `ORA-36` | Nenhum duplo de teste dentro do domínio (RFC §9.2). A fixture alimenta a execução **por valor**: estado e requisição chegam prontos, como em FND-03 §2.4. Precisar de mock para executar o desfecho é sintoma de violação, não parte do oráculo |
| `ORA-37` | **Acessor único e sequência fechada** (`DEC-08` e `DEC-11` pelo lado do domínio; `DEC-13`): a projeção declara a sequência ordenada como o **único** acessor de eventos do desfecho. Nenhum segundo acessor, nenhuma coleção pendente retendo evento após o retorno, nenhum meio de acrescentar, reordenar ou filtrar eventos depois de produzido |
| `ORA-38` | **Pós-condição de estado e imutabilidade** (`DEC-10` e `DEC-12`): a fixture inclui o estado observável do alvo antes da chamada e assevera que, após `Rejected`, ele é idêntico. Uma segunda leitura do desfecho após o retorno devolve a mesma projeção — o desfecho não oferece operação que o altere |
| `ORA-39` | **Par de vetores por ramo e vetor negativo por proibição.** Cada ramo tem um vetor positivo; cada linha "proibido" da tabela de FND-03 §3.4 tem um vetor negativo em que uma realização não conforme é **reprovada** pelo oráculo. Um oráculo que só possua vetores positivos não prova a norma |
| `ORA-40` | **Fronteira de decidibilidade.** Onde uma regra não é decidível apenas pela projeção em memória — em particular a entrega do mesmo evento por dois mecanismos de transporte (`DEC-08` pelo lado da entrega) — a asserção correspondente é **dependência declarada** a §3 e a FND-04, não é resolvida aqui. O oráculo desta seção cobre o lado-domínio do desfecho |

`rationale` — `ORA-32` recusa observar o mecanismo de propósito. Impor a união
discriminada como mecanismo, e não apenas como forma, daria fidelidade máxima ao
enunciado, ao custo de contrariar a premissa do épico de que a paridade é
conceitual, e de antecipar decisão que pertence aos épicos de kernel (P0-4).
FND-03 §3.4 já fixa que a forma do desfecho é normativa e o mecanismo é de kernel;
`ORA-32` é a projeção dessa escolha no oráculo.

`rationale` — `ORA-37` e `ORA-40` separam o que a projeção em memória decide do que
ela não decide. `DEC-08` tem duas metades: "o desfecho não expõe um segundo
acessor para os eventos" é observável na projeção e cabe em `ORA-37`; "o mesmo
evento não é entregue por dois mecanismos" é propriedade da entrega, que ocorre
fora do domínio, na camada de FND-04, e cabe em `ORA-40`. Sem essa divisão, o
oráculo prometeria decidir o que não observa — e um oráculo que reporta como
aprovado o que não tem meio de avaliar está incorreto.

### §7.3 Cobertura

`normativo` — as duas tabelas abaixo ligam cada obrigação de FND-03 a um vetor
concreto. A primeira percorre as onze linhas da tabela de observações de FND-03
§3.4; a segunda percorre as seis regras `DEC` que §3.4 declara necessárias além da
própria tabela. Cada linha é decidível por quem revisa: nomeia o vetor positivo, o
negativo (quando a linha é uma proibição) e o exemplo de FND-03 §8 que serve de
base à fixture. Os IDs `RAS` do par de vetores por regra são resolvidos em §10; o
que esta seção fixa é a cobertura e a base.

`registro` — **As quatro fixtures de base.** Os quatro exemplos de FND-03 §8
originam as fixtures de projeção assim:

| Exemplo de FND-03 §8 | O que origina | Papel na cobertura |
|----------------------|---------------|--------------------|
| §8.2 — Exemplo 1, determinismo em dupla execução (`Accepted`, pedido P-100, itens 2→3) | Fixture positiva do ramo `Accepted` **e** vetor de determinismo (dupla execução) | Linhas 1, 2, 3, 11 de §3.4; `ORA-34` (obrigação 11, cujo cenário é catalogado em §4) |
| §8.3 — Exemplo 2, rejeição tipada, sem exceção e sem evento (`Rejected`, itens 3, limite 3, `pedidos/limite-itens-excedido`) | Fixture positiva do ramo `Rejected` | Linhas 1, 3, 4, 5 de §3.4; `DEC-04`, `DEC-09`, `DEC-10`, `DEC-11` |
| §8.4 — Exemplo 3, contraprova: tipo de wire como estado de domínio (não conforme) | Fundamento da neutralidade da codificação (`ORA-33`), não uma fixture de projeção | `UPR-I11` é `import-verifiable`: verificado pelo regime de imports (§11 deste artefato e o linter da RFC §10), não por projeção. Dependência declarada, não vetor de §7 |
| FND-03 §8.5 — Exemplo 4, consumo que dispara nova UPR | Delimita onde o desfecho termina e a camada de FND-04 começa | Fronteira, não fixture. `ORA-40` e o handoff de §7.4 herdam essa divisão |

`registro` — **Tabela 1: as onze linhas de observação de FND-03 §3.4.**

| # | Observação na fronteira (§3.4) | Regra `ORA` | Vetor positivo | Vetor negativo | Base §8 |
|---|--------------------------------|-------------|----------------|----------------|---------|
| 1 | O ramo é distinguível sem inspecionar texto de mensagem | `ORA-31` | Projeção nomeia o ramo em ambos os exemplos | Realização que só distingue os ramos pelo texto da mensagem | §8.2, §8.3 |
| 2 | Resposta de domínio acessível (`Accepted`) | `ORA-31` | Projeção de §8.2 expõe `ItemAdicionado{...}` | Realização que aceita sem resposta acessível | §8.2 |
| 3 | Sequência ordenada de eventos acessível | `ORA-31`, `ORA-34` | Projeção de §8.2 expõe a sequência na ordem; §8.3 expõe sequência vazia | Realização com eventos em estrutura não ordenada ou ordem divergente entre stacks | §8.2, §8.3 |
| 4 | Rejeição tipada com código estável acessível (`Rejected`) | `ORA-31` | Projeção de §8.3 expõe `pedidos/limite-itens-excedido` | Rejeição sem código estável, ou legível só pela mensagem | §8.3 |
| 5 | Estado observável do alvo | `ORA-38` | Projeção de §8.3 assevera itens = 3 (inalterado) após `Rejected` | Realização que deixa o alvo parcialmente mutado após recusa (`DEC-10`) | §8.3 |
| 6 | Chegada por canal indistinguível de falha técnica — proibido | `ORA-39` | Projeção de §8.3 chega como valor devolvido | Rejeição entregue pelo mesmo canal das falhas de infraestrutura → oráculo reprova (`DEC-04`) | §8.3 |
| 7 | Chegada por lançamento que interrompe o fluxo do chamador — proibido | `ORA-39` | Desfecho devolvido, não lançado | Realização que lança (mesmo exceção tipada) → o harness não recebe `Decision` e reprova (`DEC-04`) | §8.3 |
| 8 | Mesmo evento acessível por um segundo caminho — proibido | `ORA-37`, `ORA-40` | Projeção com acessor único de eventos | Desfecho com segundo acessor de eventos → reprova (lado-domínio de `DEC-08`); entrega por dois mecanismos → §3/FND-04 | §8.2 |
| 9 | Coleção pendente retendo evento após o retorno — proibido | `ORA-37` | Projeção de §8.3 sem coleção pendente | Realização que retém "tentativa recusada" em coleção interna após `Rejected` → reprova (`DEC-11`) | §8.3 |
| 10 | Meio de alterar o desfecho depois de produzido — proibido | `ORA-38`, `ORA-37` | Segunda leitura devolve a mesma projeção | Tentativa de mutar o desfecho ou de acrescentar/reordenar a sequência após o retorno → reprova (`DEC-12`, `DEC-13`) | §8.2, §8.3 |
| 11 | Exaustividade verificável no chamador | `ORA-31` | Projeção sempre em exatamente um dos dois ramos | Desfecho com terceiro estado, ausência de retorno ou valor nulo → reprova (`DEC-01`) | §8.2, §8.3 |

`registro` — **Tabela 2: as seis regras `DEC` que §3.4 exige além da tabela.**

| Regra (FND-03) | O que a projeção assevera | Vetor positivo | Vetor negativo | Decidibilidade |
|----------------|---------------------------|----------------|----------------|----------------|
| `DEC-04` | A rejeição chega como dado devolvido, não por exceção, panic ou canal indistinguível de falha técnica | §8.3 devolve `Rejected` | Realização que lança ou usa o canal de infraestrutura → reprova | Total na projeção |
| `DEC-08` | O desfecho não expõe um segundo acessor para o mesmo evento | Acessor único (`ORA-37`) | Segundo acessor no desfecho → reprova | Lado-domínio total; lado-entrega em §3/FND-04 (`ORA-40`) |
| `DEC-10` | Após `Rejected`, o estado observável do alvo é idêntico ao de antes da chamada | §8.3 mantém itens = 3 | Alvo parcialmente mutado após recusa → reprova | Total na projeção (`ORA-38`) |
| `DEC-11` | `Rejected` não carrega eventos, e não há event bag oculto | §8.3 com sequência vazia e sem coleção pendente | Evento em `Rejected` ou coleção pendente após retorno → reprova | Total na projeção (`ORA-37`) |
| `DEC-12` | O desfecho é imutável; não há operação que o altere | Segunda leitura idêntica (`ORA-38`) | Desfecho com operação que altera conteúdo → reprova | Total na projeção (por comportamento, não por palavra-chave) |
| `DEC-13` | A sequência de eventos é fechada no retorno | Nenhuma operação de acréscimo/reordenação/filtro | Meio de mutar a sequência após o retorno → reprova | Total na projeção (`ORA-37`) |

`rationale` — a decidibilidade de `DEC-12` e `DEC-13` é por **comportamento**, não
por palavra-chave, como manda o `rationale` de FND-03 §3.5: uma stack oferece
congelamento de estrutura e a outra trata imutabilidade como convenção sustentada
por cópia. Exigir a palavra-chave excluiria uma stack; o oráculo exige o
comportamento — a segunda leitura idêntica e a ausência de operação de mutação —,
que vincula as duas. O `DEC-08` é a única regra com decidibilidade partida, e
`ORA-40` a declara.

`registro` — a obrigação 11 (determinismo, FND-03 §2.4) aterrissa em duas seções: o
**cenário** de dupla execução é catalogado em §4, e a **equivalência da projeção**
entre as duas execuções e entre as duas stacks é asseverada aqui, por `ORA-34`,
sobre a fixture de §8.2. A obrigação 10 (equivalência do desfecho) é integralmente
desta seção e é bloqueante.

### §7.4 O que esta seção não decide

`normativo` — esta seção decide a **classe de oráculo** e a **cobertura**. Ela não
decide o conteúdo do desfecho, a forma concreta do arquivo, nem a implementação e a
execução nas duas stacks. A divisão de propriedade segue o princípio que FND-05
§8.4 ([ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442)) fixa para as golden
fixtures, aplicado aqui à fixture de projeção:

| Owner | Responsabilidade nesta seção |
|-------|------------------------------|
| **FND-03** (`upr-decision-mensagens.md`) | O **conteúdo** do desfecho: a forma de `Decision`, as regras `DEC`, a tabela de observações de §3.4, os quatro exemplos de §8. É a fonte; onde esta seção divergir dela, prevalece FND-03 |
| **FND-09** (este artefato, [ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)) | A **classe de oráculo** (fixture de projeção observável), a cobertura de §7.3 e o formato do arquivo — que **pode reusar** o formato definido em §5, sem herdar sua serialização de wire |
| Épicos de kernel (Go e TypeScript) | A **implementação** do oráculo e a **execução** efetiva das fixtures nas duas stacks, com a evidência registrada. Um artefato normativo não produz evidência de runtime |

`encaminhado` — **Handoff nomeado.** O oráculo executável de projeção e a execução
das fixtures nas duas stacks são entregáveis dos épicos de kernel. A condição de
fechamento é a tabela de handoff de FND-03 §10.4, que lista o que a prova precisa
cobrir para fechar o elo final da cadeia de rastreabilidade de FND-03 §10.1: os
dois ramos exercitados nas duas stacks (§3.1), cada linha da tabela de §3.4
verificada em ambas, `DEC-04`, `DEC-10`/`DEC-11`, `DEC-08`/`DEC-11`,
`DEC-12`/`DEC-13`, e os quatro exemplos de §8 como base. Até que o oráculo seja
implementado e executado, a equivalência do desfecho é **norma declarada e não
verificada mecanicamente**, exatamente como FND-03 §10.4 registra para o próprio
`Decision`.

`normativo` — **Fronteiras.** Esta seção não decide matéria de FND-08
([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445)): resiliência,
observabilidade de entrega e comportamento sob carga ficam com aquele artefato. Não
usa `payload_hash` nem serialização de wire — isso é de §6. E não fixa largura nem
codificação de tipo numérico: se a implementação em alguma stack precisar de uma
codificação concreta para representar um valor da projeção, essa escolha é
**dependência declarada** a FND-05 e à âncora ANC-03, jamais decisão de FND-09 —
fixá-la nesta seção contrariaria FND-03 §8.1.

---

## §8. Test kits, determinismo e pipeline

`normativo` — sujeito: a suíte de testes de cada stack (Go e TypeScript) e os
épicos de kernel que a implementam.

As seções §2 a §7 fixaram **o que** a estratégia prova — a pirâmide, o catálogo
de cenários distribuídos, as golden fixtures, os três oráculos e o oráculo de
desfecho. Esta seção fixa **com que instrumento** cada camada prova, sem
executá-lo. Um kit é a especificação do contrato observável de um teste; não é o
teste. A distinção não é retórica: a camada `providers` da pirâmide (§2) é
nomeada pela spec, mas nenhum artefato mergeado especifica a forma do seu kit de
conformidade — o que o kit exercita, contra qual porta, com dependência real ou
container, e qual o contrato de aprovação. Esse é o slot vazio que o ARQ-446
manda preencher, e §8.1 o preenche modelando um provider de ponta a ponta e
generalizando a forma para os demais.

`registro` — A implementação e a execução dos kits **não** são desta seção.
FND-05 §8.4 divide os papéis: FND-09 é dona do formato, do oráculo executável, do
pipeline e do diagnóstico; os épicos de kernel fazem a implementação nas duas
stacks e a execução efetiva, com a evidência registrada. §8.4 nomeia esse
handoff. Determinismo e resiliência sob carga também têm fronteiras: o mecanismo
de determinismo é fixado em §8.2, mas o oráculo de equivalência do desfecho que o
consome é de §7; e a resiliência (backpressure, timing de retry/DLQ/quarantine)
é de FND-08 — `KIT-06` a nomeia como fronteira, não a decide.

### §8.1 Os cinco kits, um por camada da pirâmide

`normativo` — A pirâmide de §2 tem **cinco** camadas, e cada uma recebe
**exatamente um** kit. Todo kit declara seu **contrato observável** em três
partes: o que **exige** do candidato, o que **exercita** e o que **caracteriza
aprovação**. Um kit cujo critério de aprovação não seja decidível é defeito — ele
deixa de ser instrumento de prova e vira texto que ninguém consegue reprovar.

| Camada (§2) | Kit | Infraestrutura |
|-------------|-----|----------------|
| domínio | `KIT-02` | nenhuma — em memória |
| services | `KIT-03` | ports fakes |
| providers | `KIT-04` | dependência real ou container |
| apps | `KIT-05` | serviço isolado |
| fluxos distribuídos | `KIT-06` | ambiente integrado |

| ID | Regra |
|----|-------|
| `KIT-01` | Cada uma das cinco camadas da pirâmide de §2 tem **exatamente um** kit, e todo kit declara contrato observável decidível: o que exige do candidato, o que exercita, o que caracteriza aprovação. **Critério de aprovação não decidível é defeito.** Os cinco kits são `KIT-02`..`KIT-06`, na ordem da tabela acima |
| `KIT-02` | **Kit de domínio em memória.** Exige o domínio (UPR e `Decision`) executável **sem infraestrutura e sem duplo de teste** dentro do domínio. Exercita o domínio com instante, identidade e aleatoriedade chegando como **valores de entrada já resolvidos** (FND-03 §2.4), nunca por porta de domínio. Aprova quando, fixadas as entradas, o domínio produz a mesma variante de `Decision`, a mesma resposta e a **mesma sequência ordenada** de eventos (FND-03 §2.4). Um domínio que **precise** de porta de `Clock`/`IdGenerator` para ser testado — ainda que sob interface, ainda que substituível — não é conforme: a dependência é a violação, não a ausência do duplo (RFC §9.2, `UPR-I07`). A presença do duplo é o próprio sinal que a pirâmide diagnostica (§2, regra em dois passos) |
| `KIT-03` | **Kit de services com ports fakes.** Exige o application service com a UoW exercitável e os ports (porta de escrita, porta de inbox, outbox store) **substituídos por fakes** que honram o contrato da porta. Exercita a Unit of Work do service (FND-04 §3). Aprova quando os passos 6 e 7 commitam juntos e o passo 8 é o único ponto de visibilidade (`UOW-07`), nenhum passo publica no broker (`UOW-08`) e, sob `Rejected`, nada é persistido nem enfileirado (`UOW-06`). Os fakes vivem **aqui**, legitimamente — é também onde vive o relógio fake de §8.2 |
| `KIT-04` | **Kit de conformidade de provider, ponta a ponta.** É o kit que o ARQ-446 exige modelado por completo (§8.1.1). Certifica **uma implementação de provider contra o contrato da sua porta**, com dependência real ou container. O provider modelado de ponta a ponta é o **outbox store**; a forma generaliza para as demais portas (§8.1.1) |
| `KIT-05` | **Kit de app borda a borda.** Exige **um** serviço isolado, executável nas suas bordas (adapter de protocolo na entrada, efeito na saída), com o cabeamento interno real (domínio + services + providers do próprio serviço) mas sem serviços pares (broker in-process ou substituído). Exercita o caminho completo adapter → application service → provider dentro de um só serviço (FND-04 §6.3). Aprova quando uma entrada na borda de protocolo produz o desfecho especificado na borda de efeito — a resposta e o registro de outbox derivado (ou nenhum, sob `Rejected`). A validação de **formato** de entrada pertence ao `app` (RFC §4.6) e é exercitada aqui |
| `KIT-06` | **Kit de harness de fluxo distribuído com reentrega.** Exige **dois ou mais** processos (produtor e consumidor) sobre um broker real, em ambiente integrado, com **reentrega deliberada** injetada. Exercita a entrega fim a fim e hospeda os cenários `CEN` de §4 que precisam de mais de um processo (commit antes do ACK, redelivery, duplicata concorrente, poison, graceful shutdown) e a verificação cross-stack (produtor Go → consumidor TS, e o inverso). Aprova `V32` quando a reentrega produz o **mesmo efeito final** e reprova quando a duplica. É o instrumento que `V32` exige — reentrega **executada**, nunca inspeção (FND-04 §7, `encaminhado`; RFC §11.3, `V32` `runtime-testable`). **Fronteira:** backpressure e o timing de resiliência são de FND-08, não deste kit (`KIT-06` dá o mecanismo; a regra de resultado que escreveria o teste de backpressure é de ARQ-445 — §4/D4) |

#### §8.1.1 O kit de conformidade de provider, modelado no outbox store

`normativo` — O `KIT-04` é o único cuja forma nenhum artefato mergeado
especificava. Modela-se aqui um provider de ponta a ponta — o **outbox store**,
como o ARQ-446 sugere — e a forma se generaliza. O outbox store é o provider de
persistência atrás de FND-04 §4 (a outbox) e §5 (o relay): o writer anexa o
registro dentro da própria transação local, e o relay o drena.

| Dimensão do contrato | Especificação para o outbox store |
|----------------------|-----------------------------------|
| **Porta certificada** | O outbox store — porta de persistência da outbox, consumida pela porta de escrita (na anexação) e pelo relay (na drenagem) |
| **Operações exercitadas** | Anexação do registro no mesmo commit da UoW de escrita (`UOW-07`, `UOW-08`); claim por lease escrevendo `status`, `locked_by`, `locked_until` e `attempt_count` num único commit (`OBX-16`), com elegibilidade decidida por estado e prazos vencidos (`OBX-09`) e **sem lock de banco durante o I/O com o broker** (`OBX-07`); transição final **condicional ao claim corrente** (`OBX-10`); nenhum estado alterado por claim substituído (`OBX-11`); ausência de transição `publishing → pending` (`OBX-04`); purga apenas de `published` (`OBX-17`); graceful shutdown que conclui ou libera os claims vivos (`OBX-13`) |
| **Dependência** | **Real ou container.** O outbox store é provider de persistência: as semânticas de lease e de transição condicional ao claim só se exercitam contra o store real (ou um container efêmero equivalente). Substituí-lo por um fake reimplementaria a própria coisa sob teste — por isso a camada `providers` da pirâmide fixa «dependência real ou container» |
| **Vetor positivo** | Um store que honra `OBX-10`/`OBX-11`: um registro reivindicado por lease vivo só transita sob o claim corrente; um claim vencido ou substituído **não** muta o registro |
| **Vetor negativo** | Um store que deixa um claim expirado/substituído marcar o registro como `published`: viola `OBX-11` e descarta em silêncio a garantia de que a re-drenagem reentrega — exatamente o defeito que o at-least-once (`GAR-01`) depende de não ocorrer |

`normativo` — **Generalização.** A mesma forma de quatro dimensões (porta,
operações, dependência, par de vetores) instancia o `KIT-04` para os demais
ports:

- **Porta de inbox** — operações: `insert-if-absent` com retorno serializando na
  chave (`INB-06`), `concluir` obrigatória em toda R1 que commita (`INB-05`),
  classificação sempre de transação commitada (`INB-18`). Vetor negativo: a porta
  sinalizando erro de constraint em vez de devolver classificação (`INB-04`).
- **Porta de escrita** — operações: a UoW com exatamente uma transação local
  (`UOW-01`) sobre um único recurso transacional (`UOW-02`), sem repetição
  automática do callback (`UOW-09`). Vetor negativo: uma porta recebida fora do
  callback participando da fronteira transacional (`UOW-04`).

`registro` — A escolha de dependência **real ou container** por provider é do
épico de kernel que implementa o kit; este artefato exige que a escolha exista e
que o par de vetores seja decidível, não uma ferramenta concreta de container.

### §8.2 Determinismo

`normativo` — sujeito: a suíte de cada stack e as camadas `services`/`provider`.
Deriva do não-funcional **[P0] Determinismo** da spec e de FND-03 §2.4, cujos
vetores de determinismo em ambas as stacks são `encaminhados` a FND-09.

| ID | Regra |
|----|-------|
| `KIT-07` | **Nenhum cenário depende de relógio de parede, ordem de map ou porta aleatória** ([P0] da spec). O determinismo é obtido por três instrumentos: **relógio fake** injetado como valor de entrada resolvido; **seed** explícita para toda aleatoriedade; e **ordenação estável** de toda coleção comparada — em particular a sequência de eventos, cuja ordem entra no enunciado de FND-03 §2.4 justamente para que a fixture que compara Go e TS tenha critério de decisão |
| `KIT-08` | **Restrição dura: o duplo de teste vive em `services` ou `provider`, nunca no domínio.** O relógio fake, a seed e o gerador de identidade são resolvidos na camada de aplicação e passados ao domínio como valores (FND-03 §2.4); pô-los como porta no domínio — ainda que substituível em teste — é a própria violação que a pirâmide diagnostica (RFC §9.2, `UPR-I07`). Um kit de domínio (`KIT-02`) que precise de duplo para determinismo está mal categorizado: o comportamento é de `services` (§2, regra em dois passos) |

`rationale` — A ordem estável de `KIT-07` não é preferência de estilo. Sem ela,
duas realizações da mesma regra emitiriam o mesmo conjunto de eventos em ordens
diferentes e ambas se declarariam conformes, e o oráculo de equivalência do
desfecho (§7) não teria como decidir entre elas (FND-03 §2.4, `rationale`). O
determinismo do mecanismo (esta seção) sustenta a comparabilidade que o oráculo
(§7) exige.

### §8.3 Pipeline de CI

`normativo` — sujeito: o pipeline de integração que roda as suítes. Preenche a
topologia que FND-05 §10.4 pede («o pipeline que roda os gates de §6.6») e o
não-funcional **[P1]** da spec.

| ID | Regra |
|----|-------|
| `KIT-09` | O pipeline roda as suítes em **estágios ordenados**, cada um com o seu gate, e **o que bloqueia merge é declarado por estágio** (§8.3.1). As camadas de baixo da pirâmide (domínio, services) rodam primeiro e barram cedo; providers, apps e contrato vêm depois |
| `KIT-10` | O estágio de **contrato** reusa os gates de FND-05 §6.6 — `buf format`, `buf lint`, `buf breaking` e a **dupla geração byte a byte** de `BUF-11` (duas gerações em ambiente limpo, comparadas entre si por reprodutibilidade e cada uma com o artefato versionado por *drift*, em cada stack). Esses gates **barram o merge** sem julgamento humano, sem advisory e sem caminho de bypass (`BUF-12`). FND-09 **não** os redefine: os herda e os posiciona no pipeline |
| `KIT-11` | A camada de **fluxos distribuídos** (`KIT-06`) roda em **pipeline separado**, para não penalizar o feedback rápido das camadas de baixo ([P1] da spec; alternativa descartada: tudo no mesmo pipeline, porque a lentidão da camada integrada leva o time a ignorar a suíte inteira). O pipeline separado tem seu próprio gate; sua ausência **não** libera o merge das camadas de baixo, e sua falha **não** é mascarada por elas |

#### §8.3.1 Estágios e gates

`normativo` — A sequência abaixo é a ordem dos estágios; cada um reprova por
conta própria e o que bloqueia merge está na coluna «Gate».

| # | Estágio | Suíte | Gate (bloqueia merge?) |
|---|---------|-------|------------------------|
| 1 | Domínio | `KIT-02` (em memória, determinístico) | Sim — falha reprova |
| 2 | Services | `KIT-03` (ports fakes) | Sim — falha reprova |
| 3 | Contrato | gates de FND-05 §6.6 + `BUF-11` (dupla geração byte a byte) | Sim — `BUF-12`, sem bypass |
| 4 | Providers | `KIT-04` (dependência real ou container) | Sim — falha reprova |
| 5 | Apps | `KIT-05` (serviço isolado) | Sim — falha reprova |
| 6 | Fluxos distribuídos | `KIT-06` (harness com reentrega) — **pipeline separado** | Sim, no pipeline próprio (`KIT-11`) |

`rationale` — Os estágios 1 e 2 são os mais baratos e os que dão feedback ao
domínio; ficam primeiro por isso. O estágio 6 é o mais caro e o mais lento
(broker real, múltiplos processos); isolá-lo em pipeline separado é o que
`KIT-11` exige, e é o que impede que a sua lentidão contamine o ciclo de quem
mexe só no domínio.

### §8.4 O que os kits não decidem

`registro` — Esta seção especifica os kits; **não** os implementa nem os executa.
A fronteira é a de FND-05 §8.4.

| Matéria | Dona |
|---------|------|
| Formato de arquivo do kit, oráculo executável, pipeline e diagnóstico | FND-09 (esta seção) |
| Implementação de cada kit nas duas stacks e execução efetiva, com evidência | Épicos de kernel Go e TS |
| Framework de teste por stack, ferramenta concreta de container | Decisão **idiomática** de cada kernel |

`encaminhado` — O backlog dos cinco kits (`KIT-02`..`KIT-06`), com o mecanismo de
determinismo (§8.2) e a topologia de pipeline (§8.3), é entregue aos **épicos de
kernel Go e TS**, que implementam e executam. FND-09 fixa o contrato observável
de cada kit; o kernel escolhe o framework e produz a evidência. A condição de
fechamento deste handoff é a suíte dos cinco kits rodando em CI, verde, com a
evidência do round-trip cross-stack registrada — critério que FND-05 §8.4 já
declara **aberto** até que os épicos de execução o satisfaçam, porque um artefato
normativo não produz evidência de runtime.

---

## §9. Os testes de arquitetura

`normativo` — sujeito: a suíte de testes de cada stack e o épico de kernel que a
implementa. A regra de dependência da RFC é `import-verifiable`, mas nenhum
artefato do acervo especifica como ela é exercitada **como teste** na suíte,
distinto do linter de produção — esta seção preenche essa lacuna.

A regra de dependência da RFC é uma **função**, não uma célula de matriz:
`decide(source_block, target_block, source_bc, target_bc, target_surface) →
PERMITIDA | PROIBIDA` (§7), conjunção de duas condições independentes — C1 (entre
blocos) e C2 (entre bounded contexts). O classificador de §4 fixa o bloco de cada
unidade. Juntos, os dois tornam a regra verificável por análise de imports. Esta
seção a instancia como **fitness function executável na suíte** — as regras
`FIT` — que reusa os diagnósticos estáveis já cunhados pela RFC e coexiste com
o linter de produção sem o substituir.

### §9.1 A regra de dependência como fitness function

| ID | Regra |
|----|-------|
| `FIT-01` | A regra de dependência da RFC (§7) e o classificador de §4 entram na suíte como **fitness function executável**: um teste que, sobre o **universo de produção**, assevera que **nenhuma aresta proibida** existe segundo `decide(...)` e que as capabilities externas e imports respeitam §6/§10.3. O teste incide sobre código de produção; testes estão **fora** do universo verificável (RFC §4.5 r3, §4.6), e um teste que importe um driver **não** reclassifica a unidade sob teste |
| `FIT-02` | O teste de arquitetura **reusa** os diagnósticos estáveis da RFC — **não cunha diagnóstico novo** para o mesmo defeito. C1 falhando é `DMPF-D001`; C2 falhando é `DMPF-D002`; capability externa não permitida é `DMPF-E001`; fechamento `pure` impuro é `DMPF-E002`; import não resolvido é `DMPF-E003`; dinâmico indeterminável é `DMPF-E004` (§10.3). Os pares de vetores positivo/negativo já existem na RFC §11 (`V13`..`V27`): o teste os **instancia**, não os reespecifica |

`registro` — Correspondência entre defeito, diagnóstico reusado e o vetor da
RFC que o teste instancia:

| Defeito | Diagnóstico (§10.3) | Vetor da RFC §11 |
|---------|---------------------|---------------------|
| Aresta proibida entre blocos | `DMPF-D001` | `V13`..`V17`, `V26`, `V27`, `V28` |
| Aresta proibida entre bounded contexts | `DMPF-D002` | `V19` |
| Capability externa não permitida para o bloco | `DMPF-E001` | `V21`, `V23` |
| Dependência `pure` com fechamento impuro | `DMPF-E002` | `V22` |
| Import não resolvido | `DMPF-E003` | `V24` |
| Import dinâmico com alvo não determinável | `DMPF-E004` | `V25` |

`rationale` — A granularidade curta de `FIT-02` é deliberada: no modo
`import-verifiable` (§10/D10), a linha da rastreabilidade é curta porque o
diagnóstico **já existe** na RFC §10.3. Cunhar `DMPF-*` novo para a aresta
proibida seria duplicar a verdade e abrir a porta a dois veredictos para o mesmo
código.

### §9.2 Distinção frente ao linter de produção

`normativo` — O teste de arquitetura da suíte **não** é o verificador de RFC §10
(«Contrato do verificador» — o linter de produção). O linter está **deferido**
aos épicos de kernel por **ANC-10**, cuja condição de fechamento é «verificador
conforme passando nos 32 vetores de §11». Os dois cobrem o mesmo defeito por
caminhos distintos e coexistem.

| Aspecto | Teste de arquitetura (`FIT`, suíte) | Linter de produção (RFC §10) |
|---------|-------------------------------------|------------------------------|
| Dona | FND-09 especifica; o épico de kernel executa na suíte | Épicos de kernel (`ANC-10`) |
| O que cobre | assevera, dentro da suíte da aplicação e junto das demais camadas, que nenhuma aresta proibida existe — a regra de dependência exercitada como teste | o verificador conforme de §10: schema do metadado (§10.1), **trust model** (§10.2, manifesto × baseline autorizado) e semântica de grafo (§10.3), como gate de CI/pré-commit sobre o metadado auto-declarado |
| Diagnóstico | **reusa** `DMPF-D*`/`DMPF-E*` | **emite** `DMPF-D*`/`DMPF-E*` (mesma família) |
| Entrada | a classificação por bloco/bounded context das unidades | o `metadata_container` auto-declarado (§10.1), implantado pela mesma `ANC-10` |
| Estado | especificado aqui | deferido; fechamento = verificador conforme nos 32 vetores de §11 |

| ID | Regra |
|----|-------|
| `FIT-03` | O teste de arquitetura e o linter de produção **coexistem e não se substituem**. O linter é o gate **autoritativo** sobre o metadado auto-declarado, com trust model (§10.2) e cobertura do repositório inteiro; o teste de arquitetura é a fitness function **dentro da suíte** da aplicação, que reprova uma aresta proibida no tempo de teste, junto das demais camadas. O teste de arquitetura **não** exerce o trust model de §10.2 (não compara manifesto com baseline autorizado): ele assevera as arestas e reusa os diagnósticos; adjudicar a autodeclaração continua sendo do linter |
| `FIT-04` | **Enquanto o linter de produção não existe** — está deferido por `ANC-10`, fechamento pendente — a regra de dependência permaneceria «norma declarada e não verificada mecanicamente» no gate de produção. O teste de arquitetura de `FIT-01` é o instrumento que FND-09 dá para que a regra tenha verificação executável **na suíte de cada aplicação** desde já, sem esperar o linter. Ele **não fecha** `ANC-10`: a condição de fechamento continua sendo o verificador conforme passando nos 32 vetores de §11 |

`encaminhado` — A implementação do teste de arquitetura nas duas stacks é do
épico de kernel; FND-09 fixa a fitness function (o que ela assevera, o
diagnóstico que reusa, os vetores da RFC §11 que instancia). O linter de
produção de RFC §10 continua sendo entregável de `ANC-10` (artefato sucessor:
«implementação dos linters Go e TS»), com a condição de fechamento acima. Os dois
são handoffs distintos ao mesmo dono de execução, e nenhum dispensa o outro.

---

## §10. A cadeia de rastreabilidade e os vetores

### §10.1 A cadeia, e por que ela é instanciada por regra

`recepcionado` — RFC §14.5 fixa a cadeia que liga cada regra normativa à sua
origem e à sua verificação. Ela é o motivo de existir de FND-09: enquanto o elo
final não fecha, a garantia é norma declarada, não norma provada. Instanciada
para este artefato, a cadeia é:

```text
garantia normativa do acervo — constraint P0 / RFC / Parte-1 / decisão de uma sub-spec
        ↓  ID estável da regra herdada (BLK, UOW, OBX, INB, GAR, ENV, PTB, DEC, CTX, …)
    cláusula a provar
        ↓  §10.3 — diagnóstico estável (reusado de RFC §10.3 ou próprio de FND-09)
    identificador do defeito
        ↓  §10.4 — modo de verificação da regra (RFC §2.2)
    instrumento: par de vetores, critério de inspeção, ou oráculo mais cenário
        ↓  §13 — mapa por ID, regra a regra
    o mecanismo que prova a garantia
```

`normativo` `RAS-01` — **Toda regra normativa do acervo tem, ao fim desta cadeia,
ao menos um mecanismo que a prove, e o elo é instanciado regra a regra, não por
família.** A cadeia de RFC §14.5 termina em «vetor positivo + vetor negativo,
pareados Go/TS»; um elo quebrado — regra sem diagnóstico quando `import-verifiable`,
ou diagnóstico sem vetor — é defeito deste documento, exatamente como RFC §14.5 o
declara para si. A instância de cada regra vive na §13; esta seção fixa **a forma**
que a §13 aplica.

`rationale` — A obrigação vem de duas fontes convergentes. RFC §14.5 exige que
cada cláusula normativa tenha diagnóstico e vetor sob pena de elo quebrado; e
FND-04 §11.2, ao entregar o «ID citável» de cada uma das suas regras, encaminha
em texto literal — `encaminhado` — que «o **diagnóstico estável** por regra e o
par de vetores positivo e negativo que RFC §14.5 pede são de FND-09». FND-04
§11.1 desenha a própria cadeia com o último elo marcado «pendente: diagnóstico
estável e vetores, por regra», e §11.5 escala o handoff. Fechar esse elo para as
682 regras do acervo é a obrigação central deste artefato (§1.4).

`normativo` `RAS-02` — **Especificar a forma, o oráculo e o diagnóstico de cada
regra é de FND-09; implementar e executar é dos épicos de kernel.** A divisão é a
de FND-05 §8.4: a FND-09 pertencem «o oráculo executável, o formato de arquivo da
fixture, o pipeline que a roda e o diagnóstico que ela emite»; aos épicos de
kernel, «a implementação nas duas stacks e a execução efetiva». A especificação
regra a regra é, portanto, deste artefato — não dos kernels.

`rationale` — Delegar a **especificação** regra a regra aos épicos de kernel, e
entregar aqui apenas «a forma» com um mapa por prefixo, inverteria a divisão de
FND-05 §8.4: os kernels executam, não especificam. Um mapa por família deixaria
cada épico decidir, por conta própria, qual defeito uma regra herdada exibe e que
vetor a exercita — que é precisamente a decisão normativa que FND-05 §8.4 reserva
a FND-09. Por isso a granularidade da §13 é por ID, e não por prefixo.

### §10.2 A forma do par de vetores

`normativo` `RAS-03` — **Um vetor positivo é um caso conforme mínimo que o
mecanismo deve aprovar.** Ele nomeia a regra herdada pelo ID estável, descreve o
setup que a satisfaz e declara o desfecho esperado: passa. Um positivo que
reprove indica erro do vetor ou do mecanismo, não da regra.

`normativo` `RAS-04` — **Um vetor negativo é um caso violador mínimo que o
mecanismo deve reprovar, com o diagnóstico esperado nomeado.** Ele altera o
positivo no ponto exato da violação — e só nele —, cita o mesmo ID da regra e
declara qual diagnóstico estável (§10.3) a reprovação deve emitir. O negativo que
não nomeia o diagnóstico esperado não é conferível: um mecanismo que reprove pela
razão errada passaria nesse vetor sem provar a regra.

`normativo` `RAS-05` — **O par é realizado nas duas stacks, salvo assimetria
declarada.** RFC §11.1 exige o par positivo/negativo pareado Go/TS para toda
classe normativa; a única exceção admitida é o vetor single-stack declarado como
tal (§10.6). Ausência de par sem declaração de assimetria é elo quebrado.

`normativo` `RAS-06` — **Um par cujo vetor negativo não seja reprovado pelo
mecanismo não prova nada: vetor negativo que passa é defeito do mecanismo ou da
regra, nunca conformidade.** É o negativo, e não o positivo, que distingue um
mecanismo correto de um permissivo — a razão que RFC §11.1 registra: um verificador
que aprovasse tudo passaria em todos os positivos. Por isso o negativo que passa
exige diagnóstico da causa (mecanismo cego ou regra mal formulada), jamais registro
de conformidade.

### §10.3 A forma do diagnóstico estável

`normativo` `RAS-07` — **Um diagnóstico é estável quando a mesma violação produz
o mesmo identificador, independente de stack e de execução.** O identificador é a
âncora entre a mensagem que o mecanismo emite, a regra herdada e o vetor negativo
que a exercita (RFC §10.3): sem estabilidade, o negativo de RAS-04 não teria o que
nomear. Mensagem legível para humanos pode variar; o código, não.

`normativo` `RAS-08` — **Onde o defeito já tem código em RFC §10.3, reusa-se o
código existente; cunhar código novo para o mesmo defeito é proibido.** As
famílias de RFC §10.3 — `DMPF-U` (cobertura de unidades), `DMPF-M` (manifesto),
`DMPF-T` (integridade do baseline), `DMPF-D` (arestas proibidas, §7) e `DMPF-E`
(capabilities e imports, §6) — já nomeiam os defeitos `import-verifiable` do
acervo. Uma regra herdada que viole a regra de dependência reprova sob
`DMPF-D001`/`DMPF-D002`; uma que viole a política de capabilities, sob
`DMPF-E001`..`DMPF-E004`. Reusá-los mantém um único código por defeito em toda a
série; um código novo para o mesmo defeito quebraria a estabilidade que RAS-07
exige.

`normativo` `RAS-09` — **O defeito próprio de FND-09, que a RFC não cataloga, é
catalogado aqui, na mesma gramática de RFC §10.3.** O código segue o padrão
`DMPF-<família><NNN>`, e a família reservada a este artefato é **`DMPF-R`** — de
**r**ound-trip e **r**eentrega, as duas classes de defeito que FND-09 introduz e que
nenhum verificador anterior podia emitir. Os códigos são estes, e são o conjunto
fechado desta entrega:

| Código | Defeito | Onde a regra o estabelece |
|--------|---------|---------------------------|
| `DMPF-R001` | Divergência na **equivalência semântica** entre o valor recuperado e o declarado na fixture — reprovação do oráculo 1 | §6.1, `ORA-02` |
| `DMPF-R002` | Divergência do **`payload_hash`** entre produtor e consumidor da mesma mensagem — reprovação do oráculo 2 | §6.1, `ORA-03` |
| `DMPF-R003` | Divergência de **identidade de bytes** de `Any.value` no escopo de `ENV-24` — reprovação do oráculo 3 | §6.1, `ORA-04` |
| `DMPF-R004` | **Efeito duplicado sob reentrega** — a reentrega da mesma mensagem produz efeito de negócio mais de uma vez | §10.5, `RAS-13` |

`normativo` — **A letra `R` não colide.** As famílias de diagnóstico em uso no
acervo são `DMPF-D`, `DMPF-E`, `DMPF-M`, `DMPF-T` e `DMPF-U` (RFC §10.3); as letras
`A` a `P` estão tomadas pelos identificadores provisórios de decisão, na forma
`ADR-DMPF-<letra>` (RFC §13.2). `R` está livre nos dois espaços, e a escolha segue o
mesmo critério que levou esta seção a preferir `RAS` a `VER` (§1.2): eliminar a
ambiguidade na origem em vez de administrá-la por convenção de escrita.

`rationale` — Uma família própria é necessária, e não é ornamento. `RAS-04` exige
que todo vetor negativo **nomeie o diagnóstico esperado**, sob o argumento de que um
mecanismo que reprove pela razão errada passaria no vetor sem provar a regra. Os
defeitos de round-trip e de reentrega não têm código em RFC §10.3 — a RFC cataloga
defeito de dependência, de universo e de manifesto, não de interoperabilidade. Sem
`DMPF-R`, os vetores negativos dos oráculos e da idempotência não teriam diagnóstico
a nomear, e a exigência de `RAS-04` recairia, vazia, sobre a própria classe de
defeito que este artefato introduz. Manter a gramática de RFC §10.3 é o que permite
ler o diagnóstico de FND-09 e o do verificador de RFC §10 sob a mesma convenção.

`registro` — O detalhamento do que cada oráculo compara é matéria de §6, não desta
seção; aqui ficam a forma e o catálogo. A exigência de `INT-04` — que um caso
reprovado só no oráculo de bytes emita código distinto do reprovado na equivalência
semântica — é satisfeita pela separação de `DMPF-R001`, `DMPF-R002` e `DMPF-R003`:
três códigos distintos para três reprovações distintas, e não um código único de
«round-trip falhou».

### §10.4 A calibração por modo

`normativo` `RAS-10` — **O modo de verificação da regra (RFC §2.2) determina a
forma da instância da cadeia.** Tratar as 682 regras como iguais produziria vetor
de execução para regra que só se confere por inspeção, e o inverso. A §13
instancia cada ID conforme a linha correspondente:

| Modo da regra | O que a linha instancia |
|---------------|-------------------------|
| `import-verifiable` | ID → diagnóstico **reusado** de `DMPF-D001`/`DMPF-D002` ou `DMPF-E001`..`DMPF-E004` (RAS-08) → aresta positiva e negativa. Linha curta: o diagnóstico já existe em RFC §10.3 |
| `structurally reviewable` | ID → critério de inspeção decidível → o que caracteriza conformidade e o que a viola. **Sem** vetor de execução |
| `runtime-testable` | ID → oráculo aplicável → vetor positivo → vetor negativo → o cenário `CEN` (§4) que o exercita |
| `não declarado` | Declarar o modo primeiro — como §11 faz para as 42 regras de FND-03 sem modo (`RAS-30`) — e então instanciar conforme a linha do modo resolvido |

`normativo` `RAS-11` — **Exigir vetor de execução de regra que só se confere por
inspeção é defeito; conferir por inspeção o que exige execução também.** Esta é a
forma geral. A sua aplicação às 42 regras de FND-03 sem modo declarado, e à
recusa de simular execução onde ela não ocorre, é de §11 (`RAS-30`, `RAS-31`) — o
texto não se repete aqui. O que é próprio desta seção é a regra de que a §13 lê o
modo antes de escolher a coluna, e a linha errada é elo quebrado tanto por excesso
(vetor onde bastava inspeção) quanto por falta (inspeção onde a execução é a única
prova, como em `V32`, §10.5).

### §10.5 O instrumento de `V31` e `V32`

`recepcionado` — Os dois vetores de RFC §11 que a P0-3 sustenta chegam a FND-09
com o modo já atribuído por RFC §11.3, reafirmado em FND-04 §7.1:

| Vetor | Regra verificada | Modo | Instrumento |
|-------|------------------|------|-------------|
| `V31` | Vedação a exactly-once fim a fim | `structurally reviewable` | Varredura de documentação, contratos e configuração por promessa de exactly-once |
| `V32` | Efeito idempotente sob redelivery | `runtime-testable` | Teste de integração com reentrega deliberada |

`normativo` `RAS-12` — **O instrumento de `V31` é uma varredura, não uma
execução.** `V31` protege a vedação de P0-3 — `GAR-01` de FND-04: nenhum
documento, contrato, README ou configuração produzido sob a fundação pode
prometer exactly-once fim a fim. Por ser `structurally reviewable` (RFC §11.3;
FND-04 §11.2 registra que `GAR-01` é `structurally reviewable` por varredura, «é
V31»), a instância do par exercita a varredura: o positivo é um artefato que
declara at-least-once com efeito idempotente; o negativo, um que promete
exactly-once. Não há cenário de execução.

`normativo` `RAS-13` — **O instrumento de `V32` é a reentrega executada, e só
ela.** `V32` só se demonstra por reentrega deliberada — FND-04 §7.1 é explícito:
«V32 não é satisfeito por revisão de código; só a reentrega executada demonstra o
efeito». Por ser `runtime-testable`, a sua instância é o harness de reentrega da
camada de fluxos distribuídos (§2.5), especificado como `KIT-06` em §8; o positivo
é a reentrega que produz o mesmo efeito final, o negativo é a reentrega que
duplica o efeito — e a reprovação do negativo emite **`DMPF-R004`** (§10.3). FND-04 `GAR-04` fixa o que o vetor de fato verifica: é a
idempotência de efeito de negócio que sustenta `V32`, não a inbox — um contexto
que implemente a inbox e não a idempotência de efeito falha `V32` e falha em
silêncio, porque a reentrega da **mesma** mensagem passa.

`normativo` `RAS-14` — **Um verificador que reporte `V31` ou `V32` como aprovado
sem ter meio de avaliá-los está incorreto: o resultado correto é «não
verificado».** A regra é de RFC §11.3, reafirmada em FND-04 §7.1. Aprovado exige
que a varredura (`V31`) ou a reentrega (`V32`) tenham de fato ocorrido; na
ausência do instrumento, o desfecho conferível é `não verificado`, nunca
`aprovado`.

### §10.6 `V27` como assimetria declarada

`recepcionado` — RFC §11.4 registra `V27` como o único vetor sem par Go: «Go não
possui import apagado em compilação». O vetor verifica que o `import type` conta
como aresta (diagnóstico `DMPF-D001`) — positivo: `domain` fazendo `import type`
de tipo do próprio `domain`; negativo: `domain` fazendo `import type` de entidade
de ORM. Em TypeScript o `import type` some na emissão sem deixar dependência de
runtime; em Go o mecanismo de burla não existe.

`normativo` `RAS-15` — **O par de `V27` é declarado assimétrico: realizado só em
TypeScript, com o motivo, e a ausência do par Go não conta como lacuna de
cobertura.** O par não finge simetria — não se inventa um vetor Go para um
mecanismo que Go não tem. RFC §11.4 dá a razão: a assimetria «não enfraquece a
paridade conceitual, porque a regra que ele protege — acoplamento de contrato
conta como dependência — vale nas duas stacks; o que muda é a existência do
mecanismo de burla». Em Go, essa mesma regra é exercitada pelos demais vetores
`DMPF-D001`, sem necessidade do caso do `import type`.

`normativo` `RAS-16` — **A assimetria é registrada, não dissimulada.** O mapa da
§13 marca `V27` como single-stack, com o motivo de RFC §11.4; simular um par Go
inexistente para preencher a coluna seria defeito, e omitir o vetor de TypeScript
por não haver contraparte também. A regra vale como forma geral: onde uma stack
não possui o mecanismo que o vetor exercita, o par é declarado assimétrico com
motivo, e a cobertura da regra protegida é aferida na outra stack pelos vetores
que ali a alcançam.

---

## §11. A verificação das regras herdadas

A §10 fixou a **forma** da cadeia de rastreabilidade — o diagnóstico estável e o
par de vetores, com os IDs `RAS-01` a `RAS-16`. Esta seção faz o que nenhuma
seção irmã fez, e faz duas coisas distintas. Primeiro, **declara o modo de
verificação** de um grupo de regras que os irmãos publicaram sem modo — as 42
regras de FND-03 fora das `UPR-I` —, porque uma cadeia não se instancia sobre uma
regra cujo modo ninguém fixou. Segundo, **instancia o oráculo** das regras que
FND-07 roteou a esta âncora por escrito: o isolamento por tenant, o ciclo de vida
e o cancelamento do contexto, e as pós-condições de segurança do dado em repouso.
Fecha ainda o instrumento do perfil de validação do envelope, que FND-05 e FND-06
deixaram a esta sub-spec.

`normativo` — **A fronteira que sustenta esta seção.** As pós-condições de
segurança e de contexto verificadas aqui são de **ANC-07**, não de ANC-06. FND-07
§4.4 e §11.2 nomeiam literalmente FND-09 dona da «verificação executável» dessas
regras; ANC-06 reserva a FND-08 a resiliência — telemetria, orçamento de retry e
degradação —, e nenhuma dessas é matéria desta seção. A distinção importa mesmo
agora que FND-08 está publicado (§1.2, §12): tratar isolamento por tenant ou ciclo de
vida do contexto como matéria de FND-08 os deixaria sem dona verificável, porque a
baseline de FND-08 reserva o seu sujeito a `app`, `provider` ou `application service`
(`RES-01`) e não alcança essas pós-condições. Eles têm dona, e é esta. Esta seção quita as linhas `H3-6`, `H5-6`, `H6-5`, `H7-2` e `H7-3` da
matriz de §3.

### §11.1 Os quatro modos de verificação

`normativo`

A calibração de que a cadeia se instancia **por regra, calibrada pelo modo** é a
chave de leitura desta seção e da §13. Tratar as 682 regras do acervo como iguais
produziria vetor de execução para regra que só se confere por inspeção, e critério
de inspeção para regra cuja garantia só a execução demonstra — os dois são
defeito. Os modos são os três canônicos de RFC §2.2, usados sem alteração de
sentido, mais o caso da regra publicada sem modo, que esta seção resolve antes de
instanciar.

| `RAS-30` — Modo | O que a linha da cadeia instancia |
|-----------------|-----------------------------------|
| `import-verifiable` | ID → diagnóstico **reusado** de `DMPF-D001`/`DMPF-D002` ou `DMPF-E001`..`DMPF-E004` (RFC §10.3) → aresta positiva e negativa. Linha curta: o diagnóstico já existe |
| `structurally reviewable` | ID → critério de inspeção decidível → o que caracteriza conformidade e o que a viola. **Sem** vetor de execução |
| `runtime-testable` | ID → oráculo aplicável → vetor positivo → vetor negativo → o cenário `CEN` de §4 que o exercita |
| `não declarado` | Declarar o modo primeiro (§11.2), depois instanciar conforme a linha correspondente acima |

`normativo` — **`RAS-31`. O descasamento entre modo e instrumento é defeito, nas
duas direções.** Exigir vetor de execução — par positivo/negativo rodado — de
regra `import-verifiable` ou `structurally reviewable` é defeito: fabrica prova
que a regra não comporta e cria a ilusão de uma execução onde há uma inspeção.
Exigir apenas critério de inspeção de regra `runtime-testable` é o defeito
simétrico: uma garantia que só a execução demonstra — que duas stacks concordam,
que o isolamento nega de fato — passa a depender de leitura de código, que é
exatamente o que a regra existe para não depender. Uma linha da §13 cujo
instrumento não case com o modo declarado da regra é reprovada na revisão da §3.

`rationale` — A calibração não é conveniência de tamanho, embora também reduza o
acervo instanciável. Ela é a diferença entre uma cadeia honesta e uma cadeia que
mente sobre o próprio rigor. Uma regra de nomenclatura — «o nome do evento está no
passado» — não tem par de vetores Go/TS que a prove por execução; tem um critério
de inspeção. Escrever-lhe um «vetor» seria decorar a regra com um instrumento que
não a verifica. O inverso — resolver isolamento por tenant «por inspeção do
código» — é o erro que `IDN-14` foi escrita para proibir (§11.3).

### §11.2 O modo das regras de FND-03 publicadas sem modo

`registro` — FND-03 §10.4 é explícito: «das sete famílias, apenas as doze `UPR-I`
têm modo de verificação declarado individualmente, e o artefato não emite código
de diagnóstico algum». As outras seis famílias — 42 regras — são, nas palavras da
fonte, «norma **declarada e não verificada mecanicamente**» até que o modo e o
diagnóstico existam. FND-03 §10.4 nomeia esse trabalho como sendo «do linter de
dependências (RFC §10) e a FND-09». Esta subseção o executa: declara o modo de
cada uma das 42, no vocabulário de RFC §2.2.

`normativo` — **Uma regra de inspeção cujo predicado exige julgamento humano é
declarada como tal.** O critério de inspeção decidível que `RAS-31` exige supõe que
dois revisores apliquem o mesmo teste e cheguem ao mesmo resultado. Onde o predicado
é a aderência a um vocabulário — o caso de `MSG-N04`, linguagem ubíqua —, isso não se
sustenta: o julgamento pertence a quem detém a linguagem do bounded context, e
nenhuma varredura o substitui. A regra permanece verificável, mas o instrumento é
**revisão humana declarada**. Apresentá-la como critério mecânico afirmaria uma
decidibilidade que ela não tem, e é a única regra do acervo nessa situação.

`normativo` — **`RAS-32`. FND-09 declara o modo de verificação das 42 regras de
FND-03 publicadas sem modo, e a declaração precede a instanciação.** O modo é
atribuído por família quando a família inteira o compartilha, e por ID quando a
família é heterogênea. A tabela de UPR-I de FND-03 §2.2 é a régua: onde a regra
descreve o que a unidade **não pode alcançar**, o modo é `import-verifiable`; onde
descreve a **estrutura** conferível sem executar, é `structurally reviewable`;
onde a garantia só aparece **executando**, é `runtime-testable`.

| Grupo | IDs | Qtd. | Modo declarado por FND-09 | Fundamento na fonte | Onde a cadeia instancia |
|-------|-----|------|---------------------------|---------------------|-------------------------|
| `UPR-L` | `UPR-L01`..`UPR-L05` | 5 | `runtime-testable` | Estado retido, estado mutável compartilhado e trabalho agendado **passam pela verificação de imports** e só afloram como quebra de determinismo ou vazamento entre casos de uso (FND-03 §2.3, `rationale`) | Oráculo de determinismo por dupla execução (§4, §7) |
| `DEC` | `DEC-01`..`DEC-13` | 13 | `runtime-testable` | A conformidade incide sobre o que se observa na fronteira do desfecho (FND-03 §3.1–§3.5); FND-03 §3.4 roteia a esta âncora exatamente a equivalência observável do `Decision` | Oráculo de projeção observável (§7), com os quatro exemplos de FND-03 §8 como base |
| `FRT` | `FRT-01`, `FRT-02` | 2 | `import-verifiable` | Ausência de import de repositório, persistência ou transporte na unidade de domínio — a mesma verificação de grafo de `UPR-I07`/`UPR-I09` | Diagnóstico de aresta proibida reusado (`DMPF-D001`/`DMPF-D002`, `DMPF-E*`), §9/§10 |
| `FRT` | `FRT-03`, `FRT-04` | 2 | `structurally reviewable` | Inspeção da assinatura (não recebe contexto, deadline nem cancelamento — `FRT-03`) e da ausência de orquestração de caso de uso (`FRT-04`) | Critério de inspeção (§9) |
| `MSG-N` | `MSG-N01`..`MSG-N03` | 3 | `structurally reviewable` | Nomenclatura com predicado objetivo: nome no particípio passado, sem versão e sem transporte — dois revisores aplicam o mesmo teste e chegam ao mesmo resultado | Critério de inspeção |
| `MSG-N` | `MSG-N04` | 1 | `structurally reviewable`, por **revisão humana declarada** | Aderência à linguagem ubíqua do bounded context. O predicado é a concordância com o glossário do contexto, e o instrumento é a revisão de quem detém a linguagem — nenhuma varredura o substitui | Revisão humana declarada |
| `CTR` | `CTR-01`, `CTR-02`, `CTR-03` | 3 | `import-verifiable` | Separação dos três níveis por grafo de tipos e de imports (nenhum tipo atravessa; consumidor não importa o domínio do produtor) — expressão de P0-2 e das células 6 e 12 de RFC §7.4 | Diagnóstico de aresta proibida reusado, §9/§10 |
| `CTR` | `CTR-04`..`CTR-07` | 4 | `structurally reviewable` | Inspeção da fronteira de conversão: ocorre fora da UPR, é unidirecional, só os fatos declarados atravessam, e a promoção a contrato público é declarada | Critério de inspeção |
| `ESC` | `ESC-01`..`ESC-09` | 9 | `structurally reviewable` | Política de extensão opt-in: inspeção de que a adoção preserva as invariantes de §2/§3/§5/§6 e permanece decisão por bounded context | Critério de inspeção |

`registro` — **Total: 42 regras, e a contagem bate com o acervo.** `UPR-L` 5 +
`DEC` 13 + `FRT` 4 + `MSG-N` 4 + `CTR` 7 + `ESC` 9 = **42** — o número que FND-03
§10.4 declara sem modo, conferido contra o inventário de RFC §14.5 no §13. As doze
`UPR-I` já têm modo individual em FND-03 §2.2 e não entram nesta declaração; o que
lhes falta é o diagnóstico e o par de vetores, que a §10 instancia.

`normativo` — **Duas exceções por ID dentro de `ESC`.** A família é
`structurally reviewable` como um todo, mas duas regras não seguem o grupo, e
declará-las com ele seria o descasamento que `RAS-31` proíbe:

- `ESC-04` (a rejeição **não** se expressa como sequência vazia de eventos) é
  `runtime-testable`: é uma observação do desfecho, e a instância vive no oráculo
  de projeção observável de §7 — um `Accepted` de sequência vazia e um `Rejected`
  precisam permanecer distinguíveis por quem observa (FND-03 §7.3);
- `ESC-05` (ler o fluxo de eventos de outro contexto é `CTR-03` violada) é
  `import-verifiable`: reusa o diagnóstico de aresta proibida de `CTR-03`, porque
  a violação é um import do modelo interno alheio (FND-03 §7.4).

`rationale` — `UPR-L` recebe `runtime-testable`, e não `import-verifiable`, apesar
de parecer regra de estrutura, porque FND-03 §2.3 o diz de forma direta: um cache
de resultados em memória, um contador de execuções, um pool reaproveitado ou uma
referência retida ao último agregado «passam por qualquer verificação de imports e
quebram o determinismo de §2.4 ou vazam estado entre casos de uso». O import não
os vê; a dupla execução, sim. Instanciar `UPR-L` como `import-verifiable`
prometeria uma prova que o grafo de imports não entrega — o defeito de `RAS-31` na
sua forma mais silenciosa, porque um linter verde daria a impressão de cobertura.

### §11.3 Isolamento por tenant

`normativo` — Regras `IDN-11` a `IDN-14`, de FND-07 §4.4, classificadas ali como
`runtime-testable` e roteadas a esta âncora: «a verificação executável do
isolamento é de FND-09 sob ANC-07 [...] este artefato declara o resultado que o
teste tem de constatar». O sujeito é `provider` e `app` (o mecanismo de `IDN-14` é
do provider; o escopo, do application service). Quita a linha `H7-2` da matriz.

`normativo` — **`RAS-33`. O oráculo constata o resultado fail-closed do
isolamento por tenant.** O cenário `CEN-41` de §4 é o que o exercita, e o par de
vetores é:

| Vetor | Entrada | Resultado que o oráculo exige |
|-------|---------|-------------------------------|
| Positivo | Acesso legítimo: o tenant do contexto lê o próprio dado | O dado é devolvido |
| Negativo | Tentativa de acesso ao dado de **outro** tenant | O dado **não** é devolvido (`IDN-12`), e a tentativa é registrada como evento de segurança com sujeito, tenant do contexto e tenant do dado alcançado |

`normativo` — **`RAS-34`. O oráculo distingue «negado» de «vazio», e a distinção
é o que torna a regra `runtime-testable` verificável de fato.** Um resultado vazio
**não** satisfaz o vetor negativo: um sistema sem isolamento algum também devolve
vazio quando a consulta, por acaso, não casa linhas de outro tenant. O vetor
negativo só passa quando o oráculo constata a **negação ativa** — a tentativa
falha e é registrada como evento de segurança (`IDN-12`) —, e não a mera ausência
de dado. Em complemento, `IDN-14` exige um vetor de caminho: um caminho de leitura
ou de escrita que **omita** o escopo de tenant não pode devolver nem alterar dado
de outro tenant. Se a correção depender de cada autor lembrar de acrescentar a
condição, a regra não está satisfeita, ainda que nenhum caminho esteja errado
hoje — a próxima consulta escrita sem a condição é o caso que a regra cobre.

`rationale` — `IDN-13` reconcilia a resposta e o registro, e o oráculo herda a
reconciliação: a resposta ao chamador pode ser indistinguível de inexistência do
recurso — para não revelar existência a quem não tem acesso —, mas o registro
interno distingue os dois casos. É por isso que o oráculo não pode se contentar
com a resposta ao chamador: ela é, por desenho, ambígua entre «não existe» e
«existe e não é seu». A prova do isolamento está no registro, não na resposta.
Mascarar isolamento ausente por um resultado vazio é o modo de falha que este
oráculo existe para pegar, e é a razão de a regra ser `runtime-testable` e não
`structurally reviewable`: nenhuma inspeção de código distingue o vazio legítimo
do vazio que esconde a ausência de escopo.

`encaminhado` — O **mecanismo** que realiza `IDN-14` — constraint composta,
row-level security, chave particionada, cliente de persistência que injeta o
escopo — é do provider e dos épicos de kernel (FND-07 §4.4). O oráculo constata o
resultado; não prescreve o mecanismo, e não é o controle de acesso ao dado em
repouso de FND-07 §8, que governa quem alcança o armazenamento **fora** da
aplicação (`DAT-11`, `DAT-13`).

### §11.4 Ciclo de vida e cancelamento do contexto

`normativo` — Regras `runtime-testable` de FND-07 §11.2, roteadas a esta âncora
(«o oráculo executável e o pipeline que o roda são de lá»). O sujeito é `services`
e `app`. Quita a parte de contexto da linha `H7-3` da matriz.

`normativo` — **`RAS-35`. O oráculo do tempo de vida e da vedação de reuso do
contexto** (`CTX-15`, `CTX-16`, `CTX-17`). O cenário `CEN-42` de §4, na camada de
services ou apps, o exercita:

| Vetor | Entrada | Resultado que o oráculo exige |
|-------|---------|-------------------------------|
| Positivo | Cada execução monta o seu contexto; o trabalho que continua após a resposta (publicação, drenagem, tarefa agendada) monta o próprio, com cadeia preservada pelo `correlation_id` (`CTX-17`) | Nenhum campo de um contexto terminado é fonte de valor para trabalho novo (`CTX-14`) |
| Negativo | Contexto retido em estrutura de vida mais longa que a execução — pool, cache de módulo, variável de escopo de processo, closure capturada por handler — e reusado na requisição seguinte | Reprovado: a segunda requisição, de outro tenant, é vista com o sujeito/tenant da primeira (`CTX-15`), ou uma decisão de autorização é reutilizada além do término da execução que a produziu (`CTX-16`) |

`rationale` — O vetor negativo de `RAS-35` reproduz o bug que `CTX-15` foi escrita
para pegar, e que a imutabilidade sozinha não pega: não é alguém trocar o tenant
do contexto — é o contexto inteiro, correto e imutável, ser reaproveitado na
requisição seguinte, de outro tenant. O objeto nunca foi alterado; a imutabilidade
não vê o caso. O tempo de vida vê, e por isso o oráculo observa a fronteira entre
duas execuções, não o estado de um contexto isolado. Para `CTX-16`, o vetor
negativo é o cache de decisão de autorização cuja chave omite sujeito ou tenant —
que é como o resultado de um usuário passa a valer para outro.

`normativo` — **`RAS-36`. O oráculo do respeito ao cancelamento e ao deadline**
(`CTX-21`, `CTX-22`). O cenário `CEN-43` de §4 o exercita, e o oráculo é
observável no dependente:

| Vetor | Entrada | Resultado que o oráculo exige |
|-------|---------|-------------------------------|
| Positivo | `provider` executa I/O com contexto vigente; ao ser cancelado, interrompe o trabalho pendente | O efeito de negócio já commitado permanece (`CTX-22`); a reversão, quando cabível, é ação de negócio própria, nunca consequência implícita do cancelamento |
| Negativo | Operação remota **iniciada** com contexto já cancelado ou já expirado, concluída assim mesmo (`CTX-21`) | Reprovado, e é detectável: a chamada aparece nos registros do dependente **depois** do instante do `deadline` |

`rationale` — `RAS-36` tem um oráculo barato e decidível porque `CTX-21` já nomeou
a evidência: o `deadline` é instante absoluto (`CTX-18`), então o julgamento é uma
comparação — o timestamp da chamada no dependente contra o instante do `deadline`.
Não é preciso instrumentar o provider por dentro; basta o registro do dependente.
O oráculo não reabre a Unit of Work (a relação entre cancelamento e transação é de
FND-04); constata apenas o resultado observável do lado do contexto, como `CTX-22`
o declara.

`normativo` — **`RAS-37`. O oráculo das bordas de erro em runtime** (`ERR-02`,
`ERR-11`). FND-07 §11.2 classifica a taxonomia e o mapeamento como
`structurally reviewable`, «com parte `runtime-testable`»: «a totalidade de
`ERR-02` e o default de `ERR-11` exigem exercício das bordas». A matriz de §3 (linha
`H7-3`) roteia essa parte runtime a esta seção. O oráculo exercita a borda:

| Vetor | Entrada | Resultado que o oráculo exige |
|-------|---------|-------------------------------|
| Positivo | Erro conhecido cruzando a borda | Mapeia para exatamente uma categoria da taxonomia (`ERR-02`); um erro cuja natureza permite repetição é marcado como tal |
| Negativo | Panic, exceção não classificada, ou erro de retryability não declarada | Reprovado se escapar como sucesso ou como categoria errada; a totalidade exige que caia na categoria reservada, e a retryability não declarada assume o default **fail-closed** — não-repetível (`ERR-11`) |

`rationale` — A parte de `ERR-02`/`ERR-11` é `runtime-testable` e não de inspeção
porque a totalidade é uma propriedade das **bordas** exercitadas, não do catálogo
lido: o catálogo pode estar completo no papel e a borda ainda deixar um panic
escapar sem categoria. O default fail-closed de `ERR-11` só se demonstra
injetando um erro de retryability desconhecida e observando que ele **não** é
marcado repetível — a afirmação inversa, «é repetível por omissão», é o modo de
falha que induz retry onde não cabe.

### §11.5 Cifra em repouso e retenção

`normativo` — Regras `DAT-08`, `DAT-10` e `DAT-14` de FND-07 §8, roteadas a esta
âncora por §11.2. O sujeito é `plataforma`. Quita a parte de dado em repouso da
linha `H7-3`.

`normativo` — **O modo é o que a fonte declarou: `runtime-testable`, na
qualificação «por inspeção de superfície».** FND-07 §11.2 classifica
`DAT-08`/`DAT-10`/`DAT-14` como «`runtime-testable` **por inspeção de superfície**»:
«abrir o armazenamento e verificar a pós-condição e a retenção efetiva». Este
artefato **não** reclassifica o modo, e a razão é a definição da própria RFC: `RFC
§2.2` define `structurally reviewable` como inspeção de código ou de configuração
**sem executar o sistema**, e abrir o armazenamento para observar o que de fato está
gravado não é isso — é observação de **estado operacional**, que só existe depois de
o sistema ter rodado. O que a qualificação da fonte diz é **onde** se observa (na
superfície armazenada, não no código que a escreve, como `DAT-08` é explícita em
exigir), não que a verificação dispense execução.

`normativo` — **A inspeção exige dado discriminatório, gravado de propósito.** Este é
o ponto que faz a regra valer: inspecionar uma superfície **sem** ter gravado nada
não prova nada. Uma superfície vazia — ou que por acaso só contenha dado de classe
não protegida — satisfaz «o dado protegido está cifrado» trivialmente, enquanto o
caminho de escrita pode estar gravando outra classe em claro, e a purga pode estar
falhando sem que ninguém veja. Por isso o instrumento é: **para cada caminho de
escrita nomeado em `DAT-01`, gravar um dado de classe protegida, reconhecível, e só
então abrir a superfície.** O par de vetores é sobre o dado gravado, não sobre o
estado que se encontrar por acaso.

| Regra | Setup — o dado discriminatório | Vetor positivo | Vetor negativo |
|-------|-------------------------------|----------------|----------------|
| `DAT-08` | Gravar, por **cada** caminho de escrita de `DAT-01`, um valor reconhecível de classe protegida | Abrir a superfície e não encontrar o valor legível: está cifrado em repouso | Um caminho grava o valor e ele é **legível** na superfície — cifra ausente naquele caminho, ainda que presente nos demais |
| `DAT-10` | O mesmo valor levado à **contenção**: forçar a mensagem à DLQ e à quarantine, com envelope preservado (FND-04 `GAR-07`) | O valor não é legível na DLQ nem na quarantine: a contenção está no mesmo regime do caminho normal | O valor é legível na DLQ ou na quarantine — é a cópia que mais escapa, porque nasce operacional e retém o envelope íntegro |
| `DAT-14` | Gravar o valor com marca temporal conhecida e aguardar o prazo declarado, ou avançar o relógio do ambiente | O valor deixou de existir antes do teto: retenção efetiva **≤** teto externo aplicável (`DAT-15`) | O valor persiste depois do teto; ou não há teto declarado e a retenção não recai no mais estrito (`DAT-17`) |

`normativo` — **`RAS-38`. O oráculo de `DAT-08` e `DAT-10` é a pós-condição
observada na superfície, sobre dado que o próprio teste gravou.** É o gesto de quem
audita — abrir o armazenamento e verificar —, e não o de quem lê o código: a
satisfação não depende de conhecer algoritmo, modo ou custódia de chave, que são de
plataforma e não desta âncora (FND-07 §8.3). O que o teste acrescenta à auditoria é o
**dado plantado**: sem ele, a inspeção não distingue «está cifrado» de «não há nada
ali». A superfície de contenção não é regime mais frouxo, e `DAT-10` a alcança com a
mesma força.

`normativo` — **`RAS-39`. O teto de retenção de `DAT-14` é uma desigualdade
verificável, não um valor.** O critério é a relação retenção efetiva ≤ teto
externo, não um prazo fixo — o valor do teto varia por dado, jurisdição e contrato,
e não é fixado nem aqui nem em FND-07. A inspeção confere a desigualdade contra o
teto de origem declarada (`DAT-15`), e a purga que a realiza é operação com
evidência (`DAT-18`). O prazo concreto, o agendamento e o alarme de crescimento são
de FND-08 sob ANC-06; a desigualdade restringe esse prazo por cima, não o define.

`rationale` — A escolha de instanciar `DAT-08`/`DAT-10`/`DAT-14` por inspeção, e
não por par de vetores, é o exemplo mais claro de por que a calibração de `RAS-30`
existe. FND-07 as chamou `runtime-testable` porque a verificação abre o
armazenamento em tempo de operação — mas o que se abre é uma superfície, e o que se
confere é uma propriedade dela, não o comportamento de um fluxo. A linha da §13
para estas três regras exibe critério e não vetor; uma linha com par de vetores
seria reprovada por descasamento de modo.

### §11.6 O instrumento do perfil de validação do envelope

`normativo` — Obrigação que FND-05 §10.4 (pendência 9, e §3.5) e FND-06 §7.2 (e a
obrigação 15 de §18.2) deixam a esta sub-spec: a **forma executável** do perfil de
validação do envelope. FND-06 fixa **onde** a validação ocorre, o seu gesto e o
seu destino; o instrumento que a executa é de FND-09 sob ANC-07. Quita as linhas
`H5-6` e `H6-5` da matriz de §3, no que diz respeito ao instrumento.

`normativo` — **`RAS-40`. O instrumento é uma expressão executável dos predicados
do perfil — schema, conjunto de asserções ou validador — que roda `ENV-08` a
`ENV-13` (o perfil do envelope, FND-05) e `TRP-33` a `TRP-36` (a posição, o gesto e
o destino da validação, FND-06).** É local e fail-closed: não consulta rede, não
resolve schema remoto e **não desserializa o payload de negócio** — `ENV-13`
garante que a conformidade do envelope é verificável sem isso, e `ENV-02`,
`ENV-23` e `TRP-34` fecham o resto. Na dúvida, o envelope é inválido. O par de
vetores:

| Vetor | Entrada | Resultado que o oráculo exige |
|-------|---------|-------------------------------|
| Positivo | Envelope conforme ao perfil | Passa a validação, no `consumer adapter`, antes de qualquer efeito e antes da porta de inbox (`TRP-33`) |
| Negativo | Envelope com atributo obrigatório ausente, tipo errado, ou extensão fora do conjunto fechado (`ENV-11`) | Inválido: contido, não devolvido ao broker (`TRP-35`), e roteado à **quarantine**, não à DLQ do canal (`TRP-36`) — nunca `nack`, nunca loop |

`encaminhado` — **`RAS-41`. Onde o instrumento vive é pendência nomeada, a
co-decidir com FND-06.** A decisão entre o instrumento residir como **validação
por transporte**, de FND-06, ou como **instrumento de verificação**, de FND-09,
permanece aberta: FND-05 §10.4 pendência 9 a registra, e FND-06 §7.2 e a obrigação
15 a declaram «a decisão é entre este artefato e FND-09». Esta seção especifica o
instrumento — os predicados, a localidade fail-closed, o par de vetores — e **não**
decide unilateralmente a titularidade da sua residência. A pendência entra em §14
com dona compartilhada (FND-06 sob ANC-04 e FND-09 sob ANC-07) e condição de
fechamento: a co-decisão do bloco em que o validador executável reside no
repositório de contratos.

`rationale` — Separar o instrumento da sua residência é o que mantém esta linha
honesta. FND-09 tem competência para dizer **o que** o instrumento verifica —
porque a matéria é prova — e a forma dele decorre de predicados que FND-05 e FND-06
já fixaram. Mas **onde** ele roda é decisão de desenho de transporte tanto quanto
de verificação: um validador acoplado ao `consumer adapter` (FND-06) e um
instrumento invocável pela suíte de conformidade (FND-09) são a mesma expressão de
predicados em dois lugares, e escolher um lugar sem a dona do transporte seria
decidir matéria dela por inércia. A fronteira nomeada é a forma correta da decisão
ainda não tomada, não a sua evasão.

---

## §12. Fronteiras nomeadas e o registro sobre ADR

`registro` — Esta seção tem duas funções. A primeira é declarar o que FND-09 **não**
decide e a quem cada assunto passa: uma fronteira só é honesta quando nomeia o assunto,
a dona e a condição que a fecha. A segunda é registrar, por escrito, a análise que
sustenta a **ausência de ADR** nesta entrega — no mesmo padrão do FND-07 §10, para que
o «não» seja auditável e não apenas afirmado.

Uma advertência atravessa toda a seção e vem antes das fronteiras: **fronteira não é
cobertura** (§12.2). Declarar que um assunto pertence a outra âncora não satisfaz
critério de aceite nem conta como cenário coberto. Confundir as duas coisas é
exatamente o risco que o épico nomeia — «decisões apenas documentais» —, e é por isso
que o item de backpressure do `AC-10` ficava **bloqueado**, não satisfeito. A
condição de fechamento que essa fronteira declarava — FND-08 publicar a regra de
resultado do controle de fluxo — foi **satisfeita** durante esta entrega, e o item
passou a coberto por `CEN-44` (§4.8). A fronteira era temporal, não de matéria: o
assunto continua sendo de ANC-06, e o que mudou é que a dona o decidiu.

### §12.1 A fronteira a FND-08

`normativo` — Quatro assuntos que tocam o caminho de uma mensagem, mas que FND-09 não
pode transformar em teste, pertencem a FND-08 ([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445)),
sob a âncora **ANC-06**. O fundamento é o registro literal da ANC-06 em RFC §12.3, que
reserva a FND-08 o *«Escopo permitido: Baselines de telemetria, políticas de retry e
degradação»*, com owner *«FND-08 — ARQ-445»* e condição de fechamento *«FND-08 concluída
e revisada»*. O que FND-09 não pode fazer é **suprir** a regra de resultado dessa
matéria — o limiar, o desfecho esperado, a política de tempo —, porque decidi-la aqui
invadiria a âncora alheia. Onde FND-08 já a publicou, este artefato a **cita e
instrumenta**, como fez com o backpressure em `CEN-44`; onde não, declara a fronteira
e para.

A distinção é fina e precisa ser mantida: FND-04 §7.4 já nomeia os estados de retry,
DLQ, quarantine e replay, e FND-04 §7.3 cataloga os failure modes de consumo. O que
fica com FND-08 **não** é a existência desses estados — é a **política operacional** que
os quantifica: quando reprocessar, qual profundidade de DLQ é anômala, qual atraso de
consumo dispara ação. Essa política é baseline de telemetria e de degradação, e a
âncora a atribui a FND-08.

| Assunto (fronteira) | Dona | Fundamento | Condição de fechamento |
|---------------------|------|------------|------------------------|
| **Backpressure** — produção acima da capacidade de consumo | FND-08 ([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445)), sob ANC-06 | Ausente de todo o FND-04; matéria de resiliência, reservada à ANC-06 (RFC §12.3) | FND-08 publicar, sob ANC-06, a regra de resultado (limiar e desfecho), e ser revisada; então FND-09 instrumenta o vetor |
| **Timing de retry, DLQ e quarantine** | FND-08, sob ANC-06 | FND-04 §7.4 define os estados; os limiares e a política de tempo são baseline de retry/telemetria da ANC-06 | idem — a regra de resultado publicada por FND-08 |
| **Replay operacional** | FND-08, sob ANC-06 | FND-04 §7.4 nomeia o replay como estado; a política de quando e como reprocessar é resiliência | idem |
| **Observabilidade de entrega** — consumer lag, profundidade de DLQ, depth/age da outbox | FND-08, sob ANC-06 | Lacunas já registradas em FND-01 §4.2: *«Métricas de lag/DLQ/consumer lag Kafka»* (`não existe` no código inventariado) e *«Métricas padronizadas de outbox (depth, age)»* (`Lacuna`) — catálogo de métricas e limiares é de FND-08 | idem |

`rationale` — A fronteira de observabilidade de entrega não é uma projeção deste
artefato: FND-01 §4.2 já a registrou como `não medido`/`não existe`, com
`owner não identificado`. FND-09 apenas confirma que o assunto tem dona declarada — o
catálogo de métricas e os limiares são de FND-08, hoje publicados em §6 dele — e não o
reescreve aqui. O que resta a esta âncora é instrumentar, e a §13.7 mapeia as regras
`MET` uma a uma.

### §12.2 Fronteira não é cobertura

`normativo` — Uma fronteira declarada **não** conta como cenário coberto e **não**
satisfaz critério de aceite. Nomear a dona de um assunto é o oposto de resolvê-lo:
registra precisamente que este artefato não o resolve.

O item de **backpressure** do `AC-10` do épico (`ARQ-436`) foi o caso que exercitou
essa regra, e o desfecho dele mostra o que a regra protege. Enquanto FND-08 não
publicava a regra de resultado, este artefato declarou o item **bloqueado — não
satisfeito**, em vez de lhe atribuir um slot vazio: um cenário sem setup, sem ação e
sem desfecho verificável seria exatamente o risco que o épico nomeia, a «decisão
apenas documental», o critério marcado como atendido sem prova por trás. Um item
bloqueado com dona e condição de fechamento é honesto; um item declarado coberto sem
teste é dívida disfarçada.

`registro` — **A condição de fechamento foi cumprida, e o item passou a coberto.**
FND-08 publicou o desfecho observável sob saturação (§12.1, e a análise em §4.8), e o
item recebeu `CEN-44`. Os **seis** itens do `AC-10` estão hoje cobertos — commit antes
do ACK, redelivery, duplicata concorrente, poison, graceful shutdown e backpressure —,
cada um com cenário próprio e proveniência citada; o catálogo está na §4.

`rationale` — O episódio é a melhor defesa da regra desta subseção. Se o artefato
tivesse contado o slot vazio como cobertura, o critério de aceite apareceria satisfeito
antes de existir qualquer coisa que o provasse — e ninguém teria voltado a olhar quando
FND-08 publicasse a regra. Foi o bloqueio declarado, com dona e condição, que fez a
publicação do irmão ser lida como o evento que ela era: o destravamento de um teste
específico, e não uma nota de rodapé. Fronteira honesta é o que permite reconhecer o
fechamento quando ele chega.

`normativo` — O estado do item é declarado em **três lugares**, e essa redundância é
deliberada: aqui, nesta §12; na spec `SPEC-6RQBN98G` (catálogo de cenários e critérios
de aceite); e no `ARQ-446`. Registrá-lo uma vez só o deixaria invisível para quem lê a
spec ou o ticket sem abrir o artefato — e valeu para o bloqueio como vale agora para o
fechamento.

### §12.3 Dependências declaradas, não decididas

`registro` — Quatro elos ficam abertos ao fim desta entrega porque a decisão é de outra
dona. FND-09 os declara e não os inventa; onde o teste depende de um deles, ele opera
sobre o que a dona fixar, não sobre uma escolha própria.

| # | Dependência | Dona | Estado / condição |
|---|-------------|------|-------------------|
| 1 | **Codificação de tipo** — a escolha de codec de wire e de registry, ainda não fechada por FND-05 | FND-05 ([ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442)), sob ANC-03 | RFC §13.3 reserva a FND-05 «escolha de codec de wire e registry»; `ADR-DMPF-M`/`ADR-DMPF-N` acionados, ainda não redigidos. O round-trip de FND-09 verifica a fórmula que FND-05 fixar |
| 2 | **Base as-is prospectiva de Kafka** — a política de transporte não passou por revisão de infraestrutura de mensageria | FND-06 ([ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443)) e a revisão nomeada | FND-06 §18.3 pendência 3; `TRP-41` limita a política de Kafka a **norma de desenho**, não autorização de operação. Os vetores de transporte encaminhados por FND-06 §18.4 herdam essa limitação até a revisão |
| 3 | **Localização e forma executável do validador do perfil de envelope** | Co-decisão FND-06 × FND-09 | FND-05 §10.4 pendência 9 e FND-06 §7.2 / obrigação 15: FND-06 fixa **onde** a validação ocorre, o gesto e o destino, e o **instrumento** é de FND-09; o que resta co-decidir é a forma de expressão e onde ela vive no repositório de contratos. FND-09 não a decide sozinho |
| 4 | **Linter de produção** | Épicos de kernel, sob ANC-10 | RFC §12.3 ANC-10 defere a implementação dos linters Go e TS aos épicos de kernel, com condição de fechamento «verificador conforme passando nos 32 vetores de §11». FND-09 define os vetores de conformidade; a ferramenta que os roda em produção é de ANC-10 |

`rationale` — **A fórmula do `payload_hash` não é uma dependência aberta, e não entra
nesta lista.** FND-05 §4.3 a declara `quita`: `ENV-17` a `ENV-20` fixam SHA-256 sobre os
bytes do payload de negócio exatamente como transportados (`Any.value` do `data`), sem
desserializar nem reserializar. A byte-preservação de que a hipótese H1 depende é fechada
por FND-06 §5. Verificar cross-stack essa igualdade é trabalho de FND-09 (é oráculo, não
decisão); a **fórmula** e a **preservação** já estão decididas em outra parte, e reabri-las
aqui seria erro — a autoridade da fórmula é FND-05 §4.3, não a seção de validação do Buf.

### §12.4 O registro sobre ADR

`registro`

**Este artefato não aciona ADR, porque nenhuma decisão desta entrega altera invariante.**
A afirmação tem duas fontes independentes que convergem, e uma condicional que a mantém
honesta.

**As duas fontes.**

| Fonte | O que ela diz | Consequência |
|-------|---------------|--------------|
| Registro da ANC-07, em RFC §12.3 | *«ADR exigido: **Não**»* — **sem** a condicional «salvo alteração de invariante» que ANC-05 e ANC-06 carregam | Nesta âncora o ADR não é sequer condicionado a um gatilho; o registro é incondicional |
| RFC §13.3 | Lista as sub-specs que acionarão ADR — codec e registry (FND-05), mecanismo de relay (FND-04), políticas por transporte (FND-06) e processo de autorização da classificação (FND-10, via ANC-08) | **FND-09 não está na lista.** A RFC não previu acionamento por esta âncora |

As duas respondem a perguntas diferentes: a primeira, se a âncora exige ADR (não); a
segunda, se a RFC previu acionamento por esta sub-spec (não). Lidas juntas, não deixam
margem.

**As invariantes que a ANC-07 lista.** O registro da âncora nomeia duas, e só duas: RFC
**§9** (o domínio executável em memória) e **§4.5 regra 3** (o teste não reclassifica o
código sob teste — o SUT). São essas, e nenhuma outra, que uma decisão desta entrega
teria de tocar para engajar o regime de monotonicidade.

**O teste de cada decisão candidata.** Cada decisão substantiva foi testada contra as
invariantes da ANC-07 e contra a regra de monotonicidade correta de RFC §12.2. Nenhuma
as toca.

| Decisão | Toca invariante da ANC-07? | Análise |
|---------|----------------------------|---------|
| **D1** — o critério do round-trip (oráculo 1 ∧ oráculo 2 ∧ oráculo 3 quando `ENV-24` se aplica) | não | O critério não é RFC §9 nem §4.5 regra 3. Não havendo invariante tocada, **o regime de monotonicidade sequer se engaja** |
| **D3** — a pirâmide responde em dois passos (acoplamento no SUT × teste mal categorizado) | não | **Tangencia** §4.5 regra 3 e a **restaura**: reclassifica o teste, nunca o SUT, e segue sinalizando a dependência ilegal. `M1` autoriza detalhar e restringir dentro do escopo — não é relaxamento |
| **D4** — declarar fronteira a FND-08 em vez de decidir backpressure | não | Declarar fronteira não altera invariante alguma: passa o assunto à dona (§12.1), não o resolve nem o reinterpreta |

**A condicional que sustenta a afirmação.** O «não» é firme porque nenhuma invariante é
tocada — não porque ADR seja impossível por princípio. O regime de monotonicidade só se
engaja quando uma decisão relaxaria uma invariante listada, uma constraint P0 ou uma
célula de RFC §7. Se isso ocorresse, o caminho **não** seria um ADR isolado por âncora:

| Regra (RFC §12.2) | O que determina |
|-------------------|-----------------|
| `M2` | Uma sub-spec **não pode** relaxar, revogar ou reinterpretar invariante listada na âncora |
| `M3` | Relaxar constraint P0 ou célula de §7 exige **nova versão desta RFC acompanhada de ADR aceito** — os dois, cumulativos, nunca um em lugar do outro — e jamais adição por âncora |
| `M4` | Adição fora do escopo permitido é inválida ainda que tecnicamente correta; ADR isolado não a cura |

Nenhuma decisão desta entrega aciona esse caminho. **A análise sustenta o «não».**

### §12.5 O gate externo pendente

`registro` — O `ARQ-446` pede, no critério de aceite e no Definition of Done, a revisão
deste artefato por **um representante de cada stack** — Go e TypeScript —, já que a
matéria é interoperabilidade entre as duas. Essa revisão ainda não ocorreu.

Fica registrada como **pendente** e **não bloqueante** para a promoção deste artefato: a
promoção segue o fluxo normal de PR, e o gate de dupla-stack é condição de fechamento da
**story**, não da existência do documento. A dona é o próprio `ARQ-446`; o desfecho — a
revisão executada, com a posição de cada stack registrada — é dele, e será tabulado no
PR.

---

## §13. Identificadores estáveis e o índice de regras

Esta seção é a fonte da verificação de cobertura. Ela tem duas metades e elas
respondem a perguntas diferentes: a §13.1 declara **o que este artefato institui**,
e as §13.2 a §13.5 declaram **o que este artefato prova do acervo alheio**. A
primeira é um índice; a segunda é a cadeia de RFC §14.5 aterrissada regra a regra.

### §13.1 As regras próprias de FND-09

`registro` — Sete prefixos, instituídos em §1.2, distribuídos pelas seções que os
usam. A coluna de faixa é a autoridade: um identificador fora dela não é regra
deste artefato.

| Prefixo | Matéria | Faixa | Qtd. | Seção |
|---------|---------|-------|------|-------|
| `PIR` | Pirâmide de testes: camadas, escopo, infraestrutura, ownership | `PIR-01`..`PIR-18` | 18 | §2 |
| `CEN` | Catálogo de cenários distribuídos | `CEN-01`..`CEN-44` | 44 | §4 |
| `FIX` | Golden fixtures: sintaxe, formato, pipeline, ownership, diagnóstico | `FIX-01`..`FIX-13` | 13 | §5 |
| `ORA` | Os três oráculos de wire e a precisão de tipos | `ORA-01`..`ORA-13` | 13 | §6 |
| `ORA` | Oráculo de projeção observável do desfecho de domínio | `ORA-30`..`ORA-40` | 11 | §7 |
| `KIT` | Test kits por camada, determinismo, pipeline de CI | `KIT-01`..`KIT-11` | 11 | §8 |
| `FIT` | Testes de arquitetura como fitness function | `FIT-01`..`FIT-04` | 4 | §9 |
| `RAS` | Forma do diagnóstico estável e do par de vetores | `RAS-01`..`RAS-16` | 16 | §10 |
| `RAS` | Verificação das regras herdadas e calibração por modo | `RAS-30`..`RAS-41` | 12 | §11 |

**Total: 142 regras próprias.**

`normativo` — **Duas faixas ficam reservadas e não são reutilizadas.** `ORA-14` a
`ORA-29` e `RAS-17` a `RAS-29` não estão atribuídos. A reserva é deliberada: os dois
prefixos servem a duas seções cada um — `ORA` aos oráculos de wire (§6) e ao oráculo
de domínio (§7), `RAS` à forma da cadeia (§10) e à sua aplicação às regras herdadas
(§11) —, e a separação de faixa mantém legível a qual matéria um identificador
pertence. Preencher a lacuna depois deslocaria essa leitura; o número não atribuído
permanece não atribuído, no mesmo regime que FND-06 aplicou a `TRP-45`.

### §13.2 O mapa por regra: como ler

`registro` — As quatro subseções seguintes percorrem o acervo por artefato de
origem. Cada linha declara, para **um** identificador estável: o **modo** de
verificação, o **instrumento** que o prova, o **par de vetores** quando o modo o
admite, e **onde** neste artefato a prova vive.

`normativo` — **A linha é calibrada pelo modo**, conforme §10.4 e §11.1. Regra
`import-verifiable` reusa diagnóstico já cunhado em RFC §10.3 e exibe aresta
positiva e negativa; regra `structurally reviewable` exibe critério de inspeção
decidível e **traz «—» nas colunas de vetor**, porque vetor de execução para regra
de inspeção é defeito; regra `runtime-testable` exibe oráculo, par de vetores e o
cenário `CEN` que a exercita. Onde a fonte não declarou modo, o modo é declarado
aqui ou na §11, e a declaração é registrada como tal — nunca apresentada como se
fosse leitura literal da fonte.

`normativo` — **Cobertura declarada, e o que «cobertura» quer dizer.** O acervo
enumera **672 identificadores**, dos quais **664 são estáveis**: os oito
`ADR-DMPF-A`..`ADR-DMPF-H` são **provisórios** por declaração da própria RFC (§13.2,
que prevê a substituição pela numeração definitiva em FND-11), e por isso não entram
no universo estável. Das 664, as dez âncoras `ANC-01`..`ANC-10` são endereço de
extensão, não regra verificável: o que nelas obriga é o ato de encaminhar, cuja
verificação é a existência do artefato sucessor, não um vetor.

Restam **654 regras** com linha neste mapa. Delas, **650 têm mecanismo de prova** e
**quatro não têm**, por razão declarada:

| ID | Por que não tem mecanismo aqui | Dona |
|----|-------------------------------|------|
| `THR-01` | Eixo *Denial of service* — limiar, orçamento, degradação e contenção de carga | FND-08 (§12.1) |
| `THR-02` | Mesmo eixo; o que está em escopo reafirma `ERR-11` e `ERR-24`, sem mecanismo próprio | FND-08 (§12.1) |
| `THR-03` | Gate de revisão de Segurança **sem owner nomeado** — pendência, não cobertura | FND-07 (§14) |
| `TRP-31` | Timing e orçamento de retry, sobre o limite de tentativas que a regra obriga a declarar | FND-08 (§12.1) |

`normativo` — **Linha não é cobertura.** Uma linha que declare fronteira ou
pendência **não** conta como regra provada, pela mesma razão que §12.2 dá: fronteira
declarada não é cenário coberto. O numerador conferível deste mapa é, portanto,
**650 de 664**, e as quatro exceções estão nomeadas acima. Contar qualquer uma delas
entre as provadas seria o mesmo defeito que este artefato recusa no item de
backpressure do AC-10.

`rationale` — A distinção parece contábil e não é. Um índice que some 654 «linhas
escritas» e chame o resultado de cobertura mede a diligência do redator, não a
verificabilidade do acervo — e é exatamente a leitura que permitiria declarar um
critério de aceite satisfeito por uma linha vazia. O número que importa é o das
regras que alguém consegue reprovar.

`rationale` — A contagem de identificadores (672) não coincide com a de cláusulas
`normativo` (682), e §1.4 explica por quê. Este mapa indexa identificadores, então
664 estáveis, 654 com linha e 650 provadas são os números conferíveis aqui. Contar cláusulas produziria um total
diferente e não endereçável — e o que a cadeia de RFC §14.5 exige é justamente o
endereço estável.

### §13.3 A RFC DMPF Foundation v0.1

**Cobertura desta parte:** 126 IDs enumerados para a RFC (`rfc-dmpf-foundation-v0.1.md`) — soma dos 15 grupos de prefixo. Dos 126, oito são os `ADR-DMPF-A`..`ADR-DMPF-H`, **provisórios** por RFC §13.2 e fora
do universo estável (§13.2); dos 118 estáveis, **108 são regras verificáveis
instanciadas abaixo** (P0, I, C, célula, T, G, DMPF-U, DMPF-M, DMPF-T, DMPF-D, DMPF-E, V, M) e **18 são registro/encaminhamento não verificáveis por FND-09** (ANC-01..10, ADR-DMPF-A..H), declarados no bloco `registro` ao final. A calibração por modo (D10; §10.4, `RAS-10`) governa cada linha: `import-verifiable` reusa diagnóstico de RFC §10.3 (`RAS-08`), `structurally reviewable` recebe critério de inspeção e `—` nas colunas de vetor, `runtime-testable` recebe oráculo e par de vetores.

#### Constraints P0 (RFC §2.3) — 4

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `P0-1` | `import-verifiable` (metade estática); `runtime-testable` (complemento §9, domínio em memória) | `DMPF-D001` reusado (RFC §10.3); metade dinâmica pelo kit de domínio | `V13`: `domain` importando só `domain` do mesmo context | `V13`: `domain` importando `provider` | §9 (`FIT-01`, `FIT-02`), §10 (`RAS-08`); metade dinâmica em §2.1 (`KIT-02`) |
| `P0-2` | `import-verifiable` | `DMPF-D001` reusado | `V14`: `contract` isolado, consumido por `app` | `V14`: `domain` importando `contract package` | §9 (`FIT-01`, `FIT-02`), §10 (`RAS-08`) |
| `P0-3` | `structurally reviewable` (vedação a exactly-once E2E); `runtime-testable` (efeito idempotente sob redelivery) — dupla, conforme RFC §2.3 declara | Vedação: varredura de doc/contrato/config (`V31`, `RAS-12`). Efeito: harness de reentrega (`V32`, `RAS-13`) | `V31`/`V32` (ver linhas em §11) | `V31`/`V32` (ver linhas em §11) | Vedação: §10.5 (`RAS-12`) + §4 (`CEN-24`). Efeito: §10.5 (`RAS-13`) + §2.5 e §8 (`KIT-06`) |
| `P0-4` | `structurally reviewable` | Inspeção: ausência de entregável de implementação (kernels/providers) nas sub-specs | — | — | §12 (fronteiras nomeadas; §12.1/§12.3) |

#### Invariantes da `verification_unit` (RFC §3.2) — 6

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `I1` | `import-verifiable` | `DMPF-U001` reusado (cobertura total; RFC §3.6) | `V01`: todo arquivo de produção coberto por um `include` | `V01`: arquivo de produção fora de todo `include` | §10 (`RAS-08`) |
| `I2` | `import-verifiable` | `DMPF-U002` reusado (não sobreposição; RFC §3.6) | `V02`: `include` disjuntos | `V02`: dois `include` cobrindo o mesmo arquivo | §10 (`RAS-08`) |
| `I3` | não declarado (a fonte não declara modo por invariante) | Critério de inspeção do binding: independência de build system — §9 o trata como `structurally reviewable` | — | — | §9 |
| `I4` | `import-verifiable` | `DMPF-D001` reusado (opacidade a alias; RFC §3.5) | `V26`: mesmo arquivo por caminho relativo e por alias produz a mesma aresta | `V26`: aresta proibida via barrel reprova igual ao import direto | §9 (`FIT-02` instancia `V26`), §10 |
| `I5` | não declarado (a fonte não declara modo por invariante) | Critério de inspeção do binding: mover/renomear não reclassifica em silêncio — §9 o trata como `structurally reviewable` | — | — | §9 |
| `I6` | não declarado (a fonte não declara modo por invariante) | Critério de inspeção do binding: determinismo (mesma árvore → mesma atribuição) — §9 o trata como `structurally reviewable` | — | — | §9 |

#### Condições da função de decisão de dependência (RFC §7.1) — 2

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `C1` (bloco) | `import-verifiable` | `DMPF-D001` reusado (par `(source_block, target_block)` na matriz RFC §7.3) | `V13`..`V17`: aresta de blocos permitida passa | `V13`..`V17`: aresta de blocos proibida reprova | §9 (`FIT-01`, `FIT-02`), §10 |
| `C2` (contexto) | `import-verifiable` | `DMPF-D002` reusado (`same_bounded_context` ou `public_integration_surface`) | `V18`/`V20`: intra-context, ou destino em superfície pública | `V19`: importar `domain` de outro `bounded_context` | §9, §10 |

#### Matriz de decisão por célula (RFC §7.4) — 36 (agrupadas)

Cada célula é `import-verifiable` por `DMPF-D001` (condição C1). Agrupadas por decisão; as cinco com vetor `V` dedicado saem em linha própria. Total coberto: 5 + 14 + 17 = **36**.

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `célula 5` (domain → provider · P0-1) | `import-verifiable` | `DMPF-D001` reusado | `V13` (positivo) | `V13` (negativo) | §9, §10 |
| `célula 6` (domain → contract · P0-2) | `import-verifiable` | `DMPF-D001` reusado | `V14` (positivo) | `V14` (negativo) | §9, §10 |
| `célula 4` (domain → port) | `import-verifiable` | `DMPF-D001` reusado | `V15` (positivo) | `V15` (negativo) | §9, §10 |
| `célula 11` (application → provider) | `import-verifiable` | `DMPF-D001` reusado | `V16` (positivo) | `V16` (negativo) | §9, §10 |
| `célula 23` (port → provider) | `import-verifiable` | `DMPF-D001` reusado | `V17` (positivo) | `V17` (negativo) | §9, §10 |
| `células proibidas restantes` (2, 3, 9, 12, 20, 21, 24, 26, 27, 31, 32, 33, 34, 35 — 14) | `import-verifiable` | `DMPF-D001` reusado | ausência da aresta / alternativa conforme passa | aresta proibida reprova sob `DMPF-D001` | §9 (`FIT-01`), §10 |
| `células permitidas` (1, 7, 8, 10, 13, 14, 15, 16, 17, 18, 19, 22, 25, 28, 29, 30, 36 — 17) | `import-verifiable` | `DMPF-D001` (C1) + `DMPF-D002` (C2) | aresta permitida passa C1 (`V18`, intra-context) | mesma aresta reprovada por C2 inter-context (`V19`, `DMPF-D002`) | §9, §10 |

#### Trust model (RFC §10.2) — 6

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `T1` | `structurally reviewable` | Inspeção: manifesto é fonte canônica; baseline é cópia independente, fora do `ownership_module` que descreve | — | — | §10; adjudicação pelo linter de produção (§9.2, `FIT-03`) |
| `T2` | `structurally reviewable` | Inspeção: baseline cobre `block` e `bounded_context` de cada `canonical_key` + digest | — | — | §10 |
| `T3` | `import-verifiable` | `DMPF-T001` reusado (divergência manifesto × baseline reprova) | `V10`/`V12`: manifesto e baseline coincidem/atualizados juntos | `V10`/`V12`: `block` alterado só no manifesto | §10 (`RAS-08`) |
| `T4` | `structurally reviewable` | Inspeção do histórico (`V11`, `DMPF-T002`): mudança de `block`/`bounded_context` isolada em commit próprio, com autorização distinta da autoria | — | — | §10; §9.2 (`FIT-03`) |
| `T5` | `structurally reviewable` | Inspeção (`V11`, `DMPF-T002`): ausência de evidência de autorização reprova — fail-closed; sem meio de avaliar, `não verificado` | — | — | §10 |
| `T6` | `structurally reviewable` | Inspeção: criação e remoção de unidade seguem T4 (`DMPF-T002`) | — | — | §10 |

#### Rastreabilidade dos diagramas derivados (RFC §8.6) — 3

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `G1` | `structurally reviewable` | Inspeção do diagrama: toda seta `import`/`implements` corresponde a uma célula de RFC §7.4, rotulada com o número | — | — | §10.4 (calibração `structurally reviewable`) |
| `G2` | `structurally reviewable` | Inspeção: nenhuma seta permitida corresponde a célula `PROIBIDA`; proibidas só com rótulo `PROIBIDA` | — | — | §10.4 |
| `G3` | `structurally reviewable` | Inspeção: alteração em RFC §7.4 revisa os diagramas na mesma mudança | — | — | §10.4 |

#### Diagnósticos estáveis — cobertura de unidades (RFC §10.3) — 4

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `DMPF-U001` | `import-verifiable` | Próprio código (linter RFC §10.3); reusado, não recunhado | `V01`: arquivo coberto por um `include` | `V01`: arquivo fora de todo `include` | §10 (`RAS-08`) |
| `DMPF-U002` | `import-verifiable` | Próprio código | `V02`: `include` disjuntos | `V02`: dois `include` sobre o mesmo arquivo | §10 |
| `DMPF-U003` | `import-verifiable` | Próprio código | `V03`: `canonical_key` únicas | `V03`: duas unidades com a mesma chave | §10 |
| `DMPF-U004` | `import-verifiable` | Próprio código | `V04`: módulo com código e manifesto | `V04`: módulo com código e sem manifesto | §10 |

#### Diagnósticos estáveis — metadado/manifesto (RFC §10.3) — 3

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `DMPF-M001` | `import-verifiable` | Próprio código (campo obrigatório ausente) | `V05`/`V08`: unidade declara os campos próprios | `V05`/`V08`: unidade sem `bounded_context` / omitindo campo por herança | §10 (`RAS-08`) |
| `DMPF-M002` | `import-verifiable` | Próprio código (valor fora do conjunto fechado) | `V06`/`V07`: `block` entre os seis; `public_integration_surface` em `contract` | `V06`/`V07`: `block: "core"`; `public_integration_surface: true` em `domain` | §10 |
| `DMPF-M003` | `import-verifiable` | Próprio código (unidade duplicada) | `V09`: `id` únicos | `V09`: dois `id` iguais | §10 |

#### Diagnósticos estáveis — baseline/trust (RFC §10.3) — 2

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `DMPF-T001` | `import-verifiable` | Próprio código (divergência manifesto × baseline) | `V10`/`V12`: manifesto e baseline coincidem | `V10`/`V12`: `block` alterado só no manifesto | §10 (`RAS-08`) |
| `DMPF-T002` | `structurally reviewable` | Inspeção do histórico (`V11`; RFC §11.3): mudança normativa sem evidência de autorização; sem meio de avaliar, `não verificado` | — | — | §10; §9.2 (`FIT-03`, adjudicação é do linter de produção) |

#### Diagnósticos estáveis — arestas proibidas (RFC §10.3) — 2

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `DMPF-D001` | `import-verifiable` | Próprio código (aresta proibida entre blocos, RFC §7 · C1) | `V13`..`V17`, `V26`..`V28`, `V30`: aresta conforme passa | os mesmos `V`: aresta proibida reprova | §9 (`FIT-02`), §10 |
| `DMPF-D002` | `import-verifiable` | Próprio código (aresta proibida entre bounded contexts, RFC §7 · C2) | `V19` (positivo em `V18`) | `V19`: `domain` de outro `bounded_context` | §9 (`FIT-02`), §10 |

#### Diagnósticos estáveis — capabilities e imports (RFC §10.3) — 4

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `DMPF-E001` | `import-verifiable` | Próprio código (capability externa não permitida ao bloco, RFC §6) | `V21`/`V23`: `domain` importando dependência `pure` da allowlist; subpath declarado | `V21`/`V23`: `application` importando SDK de broker; subpath impuro | §9 (`FIT-02`), §10 |
| `DMPF-E002` | `import-verifiable` | Próprio código (dependência `pure` com fechamento impuro) | `V22`: pacote `pure` cujo fechamento é `pure` | `V22`: `pure` cujo fechamento alcança `io.network` | §9, §10 |
| `DMPF-E003` | `import-verifiable` | Próprio código (import não resolvido) | `V24`: todos os imports resolvem | `V24`: import para módulo inexistente | §9, §10 |
| `DMPF-E004` | `import-verifiable` | Próprio código (import dinâmico indeterminável) | `V25`: import dinâmico com alvo literal | `V25`: import dinâmico com alvo computado em runtime | §9, §10 |

#### Vetores de conformidade (RFC §11.2) — 32

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `V01` | `import-verifiable` | `DMPF-U001` | todo arquivo de produção coberto por um `include` | arquivo de produção fora de todo `include` | §10 |
| `V02` | `import-verifiable` | `DMPF-U002` | `include` disjuntos | dois `include` cobrindo o mesmo arquivo | §10 |
| `V03` | `import-verifiable` | `DMPF-U003` | `canonical_key` únicas no universo | duas unidades com a mesma chave | §10 |
| `V04` | `import-verifiable` | `DMPF-U004` | módulo com código de produção e manifesto | módulo com código de produção e sem manifesto | §10 |
| `V05` | `import-verifiable` | `DMPF-M001` | unidade com `block` e `bounded_context` | unidade sem `bounded_context` | §10 |
| `V06` | `import-verifiable` | `DMPF-M002` | `block` entre os seis valores | `block: "core"` | §10 |
| `V07` | `import-verifiable` | `DMPF-M002` | `public_integration_surface` em `contract` | `public_integration_surface: true` em unidade `domain` | §10 |
| `V08` | `import-verifiable` | `DMPF-M001` | cada unidade declara os próprios campos | unidade omitindo campo por existir no módulo pai | §10 |
| `V09` | `import-verifiable` | `DMPF-M003` | `id` únicos | dois `id` iguais | §10 |
| `V10` | `import-verifiable` | `DMPF-T001` | manifesto e baseline coincidem | `block` alterado só no manifesto | §10 |
| `V11` | `structurally reviewable` | Inspeção do histórico (`DMPF-T002`, RFC §11.3): positivo é mudança de `block` em commit próprio, aprovada por revisor distinto; negativo é `block` alterado no manifesto e no baseline no mesmo commit, sem autorização — sem meio de avaliar, `não verificado` | — | — | §10 |
| `V12` | `import-verifiable` | `DMPF-T001` | manifesto e baseline atualizados juntos, com autorização | `block` alterado só no manifesto, baseline intacto: reprova mesmo com imports compatíveis | §10 |
| `V13` | `import-verifiable` (P0-1) | `DMPF-D001` | `domain` importando só `domain` do mesmo context | `domain` importando `provider` | §9, §10 |
| `V14` | `import-verifiable` (P0-2) | `DMPF-D001` | `contract` isolado, consumido por `app` | `domain` importando `contract package` | §9, §10 |
| `V15` | `import-verifiable` | `DMPF-D001` | policy computacional classificada como `domain` | `domain` importando unidade `port` | §9, §10 |
| `V16` | `import-verifiable` | `DMPF-D001` | `application` acessando I/O por `port` | `application` importando `provider` | §9, §10 |
| `V17` | `import-verifiable` | `DMPF-D001` | assinatura de porta em tipos do consumidor | porta com tipo de driver na assinatura | §9, §10 |
| `V18` | `import-verifiable` | `DMPF-D002` (par distribuído: positivo de `V19`; RFC §11.2) | duas unidades `domain` com o mesmo `bounded_context` | — (negativo em `V19`) | §9, §10 |
| `V19` | `import-verifiable` | `DMPF-D002` (par distribuído: negativo de `V18`) | — (positivo em `V18`) | unidade importando `domain` de outro `bounded_context` | §9, §10 |
| `V20` | `import-verifiable` | Predicado C2 / superfície pública (`DMPF-D002`) | unidade importando `contract` de outro context | unidade importando `application` não pública de outro context | §9, §10 |
| `V21` | `import-verifiable` | `DMPF-E001` | `domain` importando dependência `pure` da allowlist | `application` importando SDK de broker | §9, §10 |
| `V22` | `import-verifiable` | `DMPF-E002` | pacote `pure` cujo fechamento é `pure` | pacote `pure` cujo fechamento alcança `io.network` | §9, §10 |
| `V23` | `import-verifiable` | `DMPF-E001` | import do subpath declarado | import de subpath impuro do mesmo pacote | §9, §10 |
| `V24` | `import-verifiable` | `DMPF-E003` | todos os imports resolvem | import para módulo inexistente | §9, §10 |
| `V25` | `import-verifiable` | `DMPF-E004` | import dinâmico com alvo literal | import dinâmico com alvo computado em runtime | §9, §10 |
| `V26` | `import-verifiable` | `DMPF-D001` (I4 / RFC §3.5) | mesmo arquivo por caminho relativo e por alias produz a mesma aresta | aresta proibida via barrel reprova igual ao import direto | §9, §10 |
| `V27` | `import-verifiable` (single-stack, só TypeScript) | `DMPF-D001`; sem par Go (RFC §11.4 — Go não tem `import type` apagado); assimetria declarada, não lacuna | `domain` fazendo `import type` de tipo do próprio `domain` (TS) | `domain` fazendo `import type` de entidade de ORM (TS) | §10.6 (`RAS-15`, `RAS-16`) |
| `V28` | `import-verifiable` | `DMPF-D001` (RFC §4.5 r2) | código gerado de wire classificado como `contract` | aresta proibida em arquivo gerado consumido em runtime | §9, §10 |
| `V29` | `import-verifiable` | `DMPF-D001` (par distribuído: positivo de `V30`; RFC §4.5 r3) | teste de unidade `domain` usando infraestrutura, sem alterar a classificação | — (negativo em `V30`) | §9, §10 |
| `V30` | `import-verifiable` | `DMPF-D001` (par distribuído: negativo de `V29`; RFC §4.5 r4) | — (positivo em `V29`) | código de produção rotulado como teste para escapar da regra | §9, §10 |
| `V31` | `structurally reviewable` (P0-3) | Varredura de doc/contrato/config por promessa de exactly-once E2E (`RAS-12`): positivo declara at-least-once com efeito idempotente; negativo promete exactly-once fim a fim — sem varredura, `não verificado` (`RAS-14`) | — | — | §10.5 (`RAS-12`), §4 (`CEN-24`) |
| `V32` | `runtime-testable` (P0-3) | Harness de reentrega deliberada (`RAS-13`, `KIT-06`) — só a reentrega executada demonstra o efeito; sem execução, `não verificado` (`RAS-14`) | reentrega produz o mesmo efeito final | reentrega duplica o efeito → `DMPF-R004`| §10.5 (`RAS-13`), §2.5, §8 (`KIT-06`) |

#### Monotonicidade das âncoras de extensão (RFC §12.2) — 4

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `M1` | `structurally reviewable` | Inspeção da sub-spec: adição que **detalha** (acrescenta especificidade) ou **restringe** (torna mais estrita) dentro do escopo da âncora é conforme | — | — | §1.1, §12.4 |
| `M2` | `structurally reviewable` | Inspeção: adição que relaxa, revoga ou reinterpreta invariante listada na âncora viola | — | — | §1.1, §12.4 |
| `M3` | `structurally reviewable` | Inspeção: relaxar constraint P0 ou célula de RFC §7 sem nova versão da RFC + ADR aceito viola (adição por âncora é insuficiente) | — | — | §12.4 |
| `M4` | `structurally reviewable` | Inspeção: adição que excede o escopo permitido da âncora é inválida, ainda que tecnicamente correta (indica mudança de fronteira RFC §1.4) | — | — | §12.4 |

`registro` — **Cobertura da RFC DMPF Foundation v0.1, por grupo de prefixo.** As regras verificáveis instanciadas acima somam **108 IDs**; a tabela lista a contagem por grupo, com o desdobramento de cada um.

| Grupo | Qtd | Detalhe |
|-------|-----|---------|
| `P0` | 4 | `P0-1`..`P0-4` |
| `I` | 6 | `I1`..`I6` (`I3`/`I5`/`I6` com modo `não declarado` na fonte — ver abaixo) |
| `C` | 2 | `C1`, `C2` |
| célula | 36 | agrupadas por decisão: 5 com `V` dedicado + 14 proibidas + 17 permitidas |
| `T` | 6 | `T1`..`T6` |
| `G` | 3 | `G1`..`G3` |
| `DMPF-U` | 4 | `DMPF-U001`..`U004` |
| `DMPF-M` | 3 | `DMPF-M001`..`M003` |
| `DMPF-T` | 2 | `DMPF-T001`, `DMPF-T002` |
| `DMPF-D` | 2 | `DMPF-D001`, `DMPF-D002` |
| `DMPF-E` | 4 | `DMPF-E001`..`E004` |
| `V` | 32 | `V01`..`V32` |
| `M` | 4 | `M1`..`M4` |
| **Subtotal mapeado** | **108** | regras verificáveis instanciadas |

`registro` — **Os 18 IDs não mapeados como regra verificável (registro/encaminhamento).** Nenhuma linha é forçada para eles. `ANC-01`..`ANC-10` (10) são `registro de extensão` (RFC §12.3): endereços de âncora que sub-specs adicionam, não regras verificáveis; o que neles é aferível — que a adição respeitou o escopo da âncora — é verificado pelas regras de monotonicidade `M1`..`M4`, que estão mapeadas. `ANC-07` é a própria autorização deste artefato (§1.1); `ANC-06` encaminha a FND-08, cuja matéria não é decidida aqui. `ADR-DMPF-A`..`ADR-DMPF-H` (8) são IDs provisórios em estado `acionado` (RFC §13.2): nomeados, com assunto e encaminhados ao FND-11 (ARQ-448), sem redação nem aceite — encaminhamento, não regra verificável por FND-09; a redação, o aceite e a numeração definitiva são matéria de FND-11. Somados aos 108 verificáveis, os 18 fecham os **126** IDs enumerados para a RFC — dos quais 118 são estáveis e oito, provisórios (§13.2).

`registro` — **IDs cujo modo a fonte não permitiu determinar (`não declarado`): `I3`, `I5`, `I6`.** A RFC §3.2 lista as seis invariantes do binding como critério de aceitação de um binding sob marcador `normativo`, mas não declara modo por invariante. `I1`/`I2`/`I4` foram resolvidos porque a RFC os amarra a diagnóstico e vetor com modo declarado (`I1`→`DMPF-U001`/`V01`; `I2`→`DMPF-U002`/`V02`; `I4`→`DMPF-D001`/`V26`, RFC §3.5/§3.6). `I3` (independência de tooling), `I5` (move/rename explícito) e `I6` (determinismo) não têm vetor nem diagnóstico pareado na RFC — ficam `não declarado`, com o instrumento apontando §9 (critério de inspeção do binding, cuja execução pertence ao kernel via `ANC-10`).

`rationale` — **Decisões de calibração por modo (D10).** `P0-3` é a única constraint P0 com dois modos declarados na fonte (RFC §2.3): `structurally reviewable` para a vedação a exactly-once E2E e `runtime-testable` para o efeito idempotente; é instanciada com os dois vetores herdados, `V31` (varredura) e `V32` (reentrega). `V27` é assimetria declarada — single-stack (TypeScript), sem par Go por construção (RFC §11.4), e o par Go não é simulado; instrumento e motivo em §10.6 (`RAS-15`/`RAS-16`). Para `V31` (structurally reviewable) e `V32` (runtime-testable), um verificador que reporte qualquer dos dois como aprovado sem meio de avaliá-los — sem varredura ou sem reentrega executada — está incorreto: o resultado correto é `não verificado` (RFC §11.3; §10.5, `RAS-14`), com `V31` pela varredura e `V32` só pela reentrega executada (`KIT-06`). As regras `structurally reviewable` (`P0-4`, `T1`/`T2`/`T4`/`T5`/`T6`, `DMPF-T002`, `G1`..`G3`, `M1`..`M4`, `V11`, `V31`, `I3`/`I5`/`I6`) recebem `—` nas colunas de vetor, com o critério de inspeção na coluna Instrumento — regra de inspeção não recebe vetor de execução. Os diagnósticos (`DMPF-U`/`M`/`T`/`D`/`E`) são reusados de RFC §10.3, nunca recunhados (`RAS-08`/`FIT-02`); onde a RFC já traz o par `V01`..`V32`, o `V` é citado em vez de inventar outro.

`registro` — **Precisões sobre a contagem da RFC.** Quatro pontos calibram a leitura dos 126 IDs.

1. **110 cláusulas `normativo` × 126 IDs enumerados.** As duas contagens medem coisas distintas e não coincidem por construção. As 110 são **cláusulas** marcadas `normativo` no corpo de FND-02 (muitas seções carregam o marcador sem serem um ID estável — RFC §1.2, §1.3, §2.4, §3.1, etc.); a enumeração de identificadores dos 15 grupos soma **126**, dos quais 118 estáveis. Não há reconciliação aritmética entre elas: os 126 IDs enumerados são cobertos e classificados um a um (108 verificáveis + 18 registro/encaminhamento), sem forçar as duas métricas a coincidir — o acervo é contado por prefixos e cláusulas, não por igualdade entre as duas contagens, como registra a §1.4.

2. **A RFC §3.2 não declara modo por invariante.** Ela lista as seis invariantes do binding do grupo `I` sob marcador `normativo`, mas sem atribuir modo a cada uma. Por isso `I1`/`I2`/`I4` resolvem para `import-verifiable` pelo amarre a diagnóstico e vetor, e `I3`/`I5`/`I6` ficam `não declarado`; não há modo agregado "misto" declarado na fonte.

3. **`ADR-DMPF-A`..`ADR-DMPF-H` são encaminhamento, não regra verificável.** RFC §13.2 abre com marcador `normativo`, mas o que é normativo ali é o **ato de acionamento** — nomear e encaminhar —, não os ADR-DMPF como regras verificáveis: são IDs provisórios em estado `acionado`, com redação e aceite deferidos ao FND-11. Ficam registrados como encaminhamento, não mapeados como regra.

4. **As 36 células de RFC §7.4 estão integralmente cobertas.** A tabela de evidência declara 17 células permitidas e 19 proibidas (= 36), e o agrupamento por decisão (5 com `V` dedicado + 14 proibidas restantes + 17 permitidas) cobre exatamente as 36, sem deixar célula de fora.

### §13.4 FND-03 e FND-04

`registro` — 118 regras com ID estável mapeadas: **54** de FND-03 (`UPR-I` 12, `UPR-L` 5, `DEC` 13, `FRT` 4, `MSG-N` 4, `CTR` 7, `ESC` 9) e **64** de FND-04 (`BLK` 5, `UOW` 11, `OBX` 18, `INB` 18, `GAR` 12). Cada linha instancia a cadeia calibrada pelo modo (D10): `import-verifiable` reusa o diagnóstico de RFC §10.3; `structurally reviewable` traz o critério de inspeção e não recebe vetor de execução (`—`); `runtime-testable` traz oráculo e par de vetores.

`registro` — **Origem do modo em FND-03.** As 12 `UPR-I` trazem modo próprio, declarado em FND-03 §2.2. As outras 42 (`UPR-L`, `DEC`, `FRT`, `MSG-N`, `CTR`, `ESC`) não têm modo na fonte; o modo abaixo é o que a §11 deste artefato lhes atribui (`RAS-32`). Em FND-04, todo modo vem de FND-04 §11.2.

#### FND-03 — `UPR-I` (§2.2, modo próprio da fonte)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `UPR-I01` | structurally reviewable | critério: entrada é mensagem de domínio semanticamente tipada | — | — | §9 `FIT` |
| `UPR-I02` | structurally reviewable | critério: opera sobre entidade/agregado identificado | — | — | §9 `FIT` |
| `UPR-I03` | runtime-testable | oráculo de domínio `KIT-02`: invariante de negócio protegida | comando que fere invariante retorna `Rejected` com alvo preservado | invariante violada é aceita/persistida | §8 `KIT-02` |
| `UPR-I04` | — (permissão) | não é obrigação — nada a verificar | — | — | n/a (permissão; `RAS-32` a registra) |
| `UPR-I05` | runtime-testable | projeção observável `ORA-30`..`40` (§7): desfecho explícito | ramo distinguível sem inspecionar texto de mensagem | ausência de retorno / desfecho implícito consultado à parte | §7 `ORA-30`..`40` |
| `UPR-I06` | runtime-testable | projeção `ORA-30`..`40` (§7): eventos declarados no desfecho | sequência de eventos acessível no retorno | eventos descobertos inspecionando o agregado | §7 `ORA-30`..`40` |
| `UPR-I07` | import-verifiable + runtime-testable | `DMPF-D001` (domain→provider) / `DMPF-E001` (SDK de I/O); oráculo `KIT-02` confirma execução sem I/O | `domain` importa só `domain`; execução não faz I/O | `domain` importa provider ou SDK de rede/banco | §9 `FIT`; §8 `KIT-02` |
| `UPR-I08` | import-verifiable | `DMPF-D001` (domain→provider/UoW) | sem import de gerenciador de transação | `domain` importa porta/driver transacional | §9 `FIT` |
| `UPR-I09` | import-verifiable | `DMPF-E001` (capability de broker) | sem import de SDK de publicação | `domain` importa SDK de broker | §9 `FIT` |
| `UPR-I10` | import-verifiable | `DMPF-E001` (capability de log/telemetria) | sem import de logger/telemetria | `domain` importa SDK de log/telemetria | §9 `FIT` |
| `UPR-I11` | import-verifiable | `DMPF-D001` (domain→contract package) | domínio usa tipos nativos | `domain` importa tipo gerado de Protobuf | §9 `FIT` |
| `UPR-I12` | import-verifiable + structurally reviewable | `DMPF-D001` (domain→contract/transporte) + critério: assinatura sem parâmetro de transporte | sem tipo de transporte na fronteira da UPR | assinatura recebe contexto/tipo de transporte | §9 `FIT` |

#### FND-03 — `UPR-L` (§2.3; modo por `RAS-32`)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `UPR-L01` | runtime-testable | oráculo `KIT-02`: estado chega por parâmetro, nada retido | duas execuções isoladas produzem o mesmo desfecho | estado retido entre execuções altera o desfecho | §8 `KIT-02` |
| `UPR-L02` | runtime-testable | `KIT-02`: escopo de vida é a própria chamada | o retorno encerra a execução | trabalho persiste após o retorno | §8 `KIT-02` |
| `UPR-L03` | runtime-testable | `KIT-02`: sem recurso adquirido/descartado | execução não abre nada a fechar | recurso pendente de liberação após o retorno | §8 `KIT-02` |
| `UPR-L04` | runtime-testable | `KIT-02`: sem estado mutável compartilhado | execuções concorrentes não interferem | estado mutável compartilhado entre execuções | §8 `KIT-02` |
| `UPR-L05` | runtime-testable | `KIT-02`: sem trabalho agendado que sobreviva ao retorno | nenhuma continuação pendente | callback/continuação/concorrência deixada em curso | §8 `KIT-02` |

#### FND-03 — `DEC` (§3; modo por `RAS-32`; oráculo = projeção observável de §7)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `DEC-01` | runtime-testable | `ORA-30`..`40` (§7): exaustividade verificável no chamador | um de dois ramos, sempre presente | ausência de retorno / valor nulo / terceiro desfecho | §7 `ORA-30`..`40` |
| `DEC-02` | runtime-testable | `ORA-30`..`40` (§7): ramos mutuamente exclusivos | exatamente um ramo | desfecho parcialmente aceito | §7 `ORA-30`..`40` |
| `DEC-03` | runtime-testable | `ORA-30`..`40` (§7): rejeição como dado tipado no desfecho | rejeição tipada acessível no retorno | rejeição fora do desfecho | §7 `ORA-30`..`40` |
| `DEC-04` | runtime-testable | `ORA-30`..`40` (§7): rejeição não chega por canal indistinguível de falha técnica | rejeição devolvida como valor | rejeição por exceção não classificada / panic / canal de falha | §7 `ORA-30`..`40` |
| `DEC-05` | runtime-testable | `ORA-30`..`40` (§7): `Accepted` carrega resposta explícita | resposta de domínio acessível (vazia explícita quando não útil) | resposta ausente | §7 `ORA-30`..`40` |
| `DEC-06` | runtime-testable | `ORA-30`..`40` (§7): sequência ordenada de eventos, possivelmente vazia | sequência acessível e ordenada | ordem/presença da sequência indeterminada | §7 `ORA-30`..`40` |
| `DEC-07` | runtime-testable | `ORA-30`..`40` (§7): eventos declarados no desfecho | eventos lidos do retorno | eventos descobertos inspecionando o agregado | §7 `ORA-30`..`40` |
| `DEC-08` | runtime-testable (decidibilidade partida) | lado-domínio: `ORA-40` (§7, acessor único); lado-entrega: dependência declarada a FND-04 | evento aparece em um único caminho no desfecho | mesmo evento por coleta de pendentes e pelo desfecho | §7 `ORA-40` |
| `DEC-09` | runtime-testable | `ORA-30`..`40` (§7): rejeição tipada com código estável | código de domínio estável acessível | rejeição sem código / com código HTTP/gRPC | §7 `ORA-30`..`40` |
| `DEC-10` | runtime-testable | `ORA-30`..`40` (§7): estado do alvo inalterado após `Rejected` | estado idêntico ao de antes da chamada | mutação parcial sobrevive à recusa | §7 `ORA-30`..`40` |
| `DEC-11` | runtime-testable | `ORA-30`..`40` (§7): `Rejected` sem eventos e sem event bag oculto | sequência vazia e nenhuma coleção pendente | evento retido após a recusa | §7 `ORA-30`..`40` |
| `DEC-12` | runtime-testable | `ORA-30`..`40` (§7): desfecho imutável | nenhuma operação altera o desfecho produzido | conteúdo do desfecho mutável após a produção | §7 `ORA-30`..`40` |
| `DEC-13` | runtime-testable | `ORA-30`..`40` (§7): sequência fechada no retorno | sem meio de acrescentar/reordenar/filtrar eventos | eventos alterados depois de produzidos | §7 `ORA-30`..`40` |

#### FND-03 — `FRT` (§4.1; modo por `RAS-32`)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `FRT-01` | import-verifiable | `DMPF-D001` (domain→repositório/porta) | recebe o estado por parâmetro | `domain` importa repositório/porta de leitura | §9 `FIT` |
| `FRT-02` | import-verifiable | `DMPF-D001` (domain→provider) / `DMPF-E001` (broker) | devolve o desfecho e termina | `domain` persiste/confirma/publica | §9 `FIT` |
| `FRT-03` | structurally reviewable | critério: assinatura sem contexto/deadline/cancelamento | — | — | §9 `FIT` |
| `FRT-04` | structurally reviewable | critério: não sequencia passos de aplicação nem invoca UPR de outro agregado | — | — | §9 `FIT` |

#### FND-03 — `MSG-N` (§5.3; modo por `RAS-32`)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `MSG-N01` | structurally reviewable | critério: nome de domain event no particípio passado | — | — | §9 `FIT` |
| `MSG-N02` | structurally reviewable | critério: nome de mensagem de domínio sem versão | — | — | §9 `FIT` |
| `MSG-N03` | structurally reviewable | critério: nome sem transporte/formato/tecnologia | — | — | §9 `FIT` |
| `MSG-N04` | structurally reviewable, por revisão humana declarada | critério: nome concorda com o glossário do bounded context; instrumento é a revisão de quem detém a linguagem, não varredura (§11.2) | — | — | §11.2 |

#### FND-03 — `CTR` (§6.2, §6.3; modo por `RAS-32`)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `CTR-01` | import-verifiable | `DMPF-D001` (aresta entre os três níveis) | nenhum tipo atravessa domínio/aplicação/wire | um mesmo tipo nos três níveis | §9 `FIT` |
| `CTR-02` | import-verifiable | `DMPF-D001` (domain→contract package) | tipo de wire isolado do domínio | tipo gerado de wire como estado/entrada/evento de domínio | §9 `FIT` |
| `CTR-03` | import-verifiable | `DMPF-D002` (domain inter-context) | consumidor não importa domínio do produtor | consumidor importa tipo de domínio do produtor | §9 `FIT` |
| `CTR-04` | structurally reviewable | critério: conversão reside fora da UPR (bloco `encaminhado` a FND-04/FND-05) | — | — | §9 `FIT` |
| `CTR-05` | structurally reviewable | critério: só os fatos publicáveis deixam o domínio; o resto fica interno | — | — | §9 `FIT` |
| `CTR-06` | structurally reviewable | critério: conversão unidirecional, sem volta integration→domain event | — | — | §9 `FIT` |
| `CTR-07` | structurally reviewable | critério: promoção a evento de integração é decisão declarada, nunca automática | — | — | §9 `FIT` |

#### FND-03 — `ESC` (§7; modo por `RAS-32`)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `ESC-01` | structurally reviewable | critério: nenhuma regra pressupõe Event Sourcing/CQRS físico; estado é o padrão | — | — | §9 `FIT` |
| `ESC-02` | structurally reviewable | critério: adoção de ES não revoga regra de §2/§3/§5/§6 | — | — | §9 `FIT` |
| `ESC-03` | structurally reviewable | critério: adoção é decisão de um contexto, não imposta a outros | — | — | §9 `FIT` |
| `ESC-04` | runtime-testable | `ORA-30`..`40` (§7): rejeição ≠ sequência vazia de eventos | `Rejected` carrega rejeição tipada; `Accepted` vazio é distinto | sequência vazia usada para significar recusa | §7 `ORA-30`..`40` |
| `ESC-05` | import-verifiable | `DMPF-D002` (leitura de fluxo de outro contexto) | integração pela conversão de FND-03 §6.3 (integration event) | consumidor lê o event stream de outro contexto | §9 `FIT` |
| `ESC-06` | structurally reviewable | critério: eventos antigos legíveis não versionam tipo de domínio como contrato público | — | — | §9 `FIT` |
| `ESC-07` | structurally reviewable | critério: responder query não exige carregar o alvo nem executar UPR | — | — | §9 `FIT` |
| `ESC-08` | structurally reviewable | critério: query servida por UPR obedece §2 (em particular `UPR-I07`: recebe o estado) | — | — | §9 `FIT` |
| `ESC-09` | structurally reviewable | critério: separação escrita/leitura é lógica; separação física é decisão de contexto | — | — | §9 `FIT` |

#### FND-04 — `BLK` (§2; `structurally reviewable` vs matriz de RFC §7.4)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `BLK-01` | structurally reviewable | inspeção vs RFC §7.4 c.11: escrita da outbox por porta | — | — | §9 `FIT` |
| `BLK-02` | structurally reviewable | inspeção: drenagem em `app` dedicado, com composition root e lifecycle próprios | — | — | §9 `FIT` |
| `BLK-03` | structurally reviewable | inspeção: mapeamento no `provider` da outbox; serialização na escrita | — | — | §9 `FIT` |
| `BLK-04` | structurally reviewable | inspeção: `destination` é destino lógico, não tópico/fila/endereço | — | — | §9 `FIT` |
| `BLK-05` | structurally reviewable | inspeção: a escrita não preenche estado de drenagem além do valor inicial | — | — | §9 `FIT` |

#### FND-04 — `UOW` (§3; `runtime-testable`)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `UOW-01` | runtime-testable | oráculo de services `KIT-03`: uma transação local por UoW | UoW com exatamente uma transação | UoW com duas transações | §8 `KIT-03` |
| `UOW-02` | runtime-testable | `KIT-03`: UoW não abrange dois recursos transacionais | um único recurso transacional | dois recursos transacionais na mesma UoW | §8 `KIT-03` |
| `UOW-03` | runtime-testable | `KIT-03`: UoW visível no service, sem contexto global | correção via callback recebido | correção depende de estado global | §8 `KIT-03` |
| `UOW-04` | runtime-testable | `KIT-03`: porta fora do callback está fora da fronteira transacional | porta transacional recebida pelo callback | porta externa participa da transação | §8 `KIT-03` |
| `UOW-05` | runtime-testable | `KIT-03`: o ramo da `Decision` decide os passos 6 e 7 | `Accepted` grava; `Rejected` não | passos 6/7 executados sob `Rejected` | §8 `KIT-03` |
| `UOW-06` | runtime-testable | `KIT-03`: sob `Rejected` nada persiste nem enfileira (`DEC-10`/`DEC-11`) | rollback total sob rejeição | estado/outbox gravados sob `Rejected` | §4 `CEN-01` |
| `UOW-07` | runtime-testable | `KIT-03`: passos 6 e 7 commitam juntos; passo 8 é o único ponto de visibilidade | estado e outbox no mesmo commit | outbox visível sem o estado (ou vice-versa) | §4 `CEN-01` |
| `UOW-08` | runtime-testable | `KIT-03`: nenhum passo da escrita publica no broker | sequência de escrita sem publicação | publicação dentro da transação | §4 `CEN-01` |
| `UOW-09` | runtime-testable | `KIT-03`: a UoW não repete o callback automaticamente | conflito propaga ao chamador | UoW reexecuta o callback sozinha | §4 `CEN-01` |
| `UOW-10` | runtime-testable | `KIT-03`: retry de conflito é política explícita, só em operação idempotente | retry declarado sobre operação idempotente | retry implícito, ou sobre operação não idempotente | §8 `KIT-03` |
| `UOW-11` | runtime-testable | `KIT-03`: query não abre UoW de escrita nem grava outbox | leitura sem transação de escrita | query grava outbox | §8 `KIT-03` |

#### FND-04 — `OBX` (§4, §5; `runtime-testable`, exceto `OBX-02`/`OBX-03` por inspeção)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `OBX-01` | runtime-testable | oráculo de provider `KIT-04`: `message_id` único por schema | inserção duplicada rejeitada | dois registros com o mesmo `message_id` | §4 `CEN-08` |
| `OBX-02` | structurally reviewable | inspeção: `metadata` sem dado sensível, credencial ou segredo | — | — | §9 `FIT` |
| `OBX-03` | structurally reviewable | inspeção: `last_error` guarda diagnóstico sanitizado | — | — | §9 `FIT` |
| `OBX-04` | runtime-testable | `KIT-04`: máquina de estados sem transição `publishing`→`pending` | sem essa aresta | registro volta de `publishing` a `pending` | §4 `CEN-05` |
| `OBX-05` | runtime-testable | `KIT-04`: estado inicial `pending` | todo registro nasce `pending` | estado inicial diferente de `pending` | §4 `CEN-02` |
| `OBX-06` | runtime-testable | `KIT-04`: `failed` terminal no ciclo automático | `failed` não retorna ao ciclo | ciclo automático reprocessa `failed` | §4 `CEN-11` |
| `OBX-07` | runtime-testable | harness `KIT-06`: nenhum lock de banco durante o I/O com o broker | publicação sem lock ativo | lock de banco mantido durante o I/O | §8 `KIT-06` |
| `OBX-08` | runtime-testable | `KIT-06`: `locked_by` identifica a execução do claim | claim identificado por execução | `locked_by` identifica o processo | §4 `CEN-04` |
| `OBX-09` | runtime-testable | `KIT-06`: elegibilidade decidida por estado e prazos vencidos | registro elegível por lease vencido | claim ignora prazo/estado | §4 `CEN-04` |
| `OBX-10` | runtime-testable | `KIT-06`: transição final condicional ao claim corrente | só o claim corrente conclui | claim morto conclui a transição | §4 `CEN-05` |
| `OBX-11` | runtime-testable | `KIT-06`: claim substituído não altera o estado do registro | claim expirado sem efeito | claim morto sobrescreve registro publicado | §4 `CEN-05` |
| `OBX-12` | runtime-testable | `KIT-06` (observabilidade): drenador expõe `pending`/`lag`/`attempts`/`failures` | os quatro sinais expostos | sinais ausentes | §8 `KIT-06` (limiares/alarmes: fronteira FND-08 §12) |
| `OBX-13` | runtime-testable | `KIT-06`: graceful shutdown conclui ou libera os claims vivos | shutdown sem claim órfão | claims vivos abandonados no encerramento | §4 `CEN-13` |
| `OBX-14` | runtime-testable | `KIT-04`: relay não interpreta o `payload` nem decide publicar por ele | publica sem inspecionar o payload | decisão de publicar baseada no payload | §8 `KIT-04` |
| `OBX-15` | runtime-testable | `KIT-04`: CDC é extensão, mesma semântica de entrega | caminho CDC entrega igual ao polling | CDC altera a semântica de entrega | §8 `KIT-04` |
| `OBX-16` | runtime-testable | `KIT-06`: claim escreve `status`/`locked_by`/`locked_until`/`attempt_count` no mesmo commit | os quatro campos num único commit | campos do claim em commits separados | §4 `CEN-04` |
| `OBX-17` | runtime-testable | `KIT-04`: `published` purgável após confirmação; `failed`/`pending`/`publishing` não | só `published` confirmado é purgado | purga de `failed`/`pending`/`publishing` | §8 `KIT-04` |
| `OBX-18` | runtime-testable | `KIT-06`: falha transitória libera `locked_until` com recálculo de `available_at` | lease liberado e reagendado | `locked_until` retido após falha transitória | §8 `KIT-06` |

#### FND-04 — `INB` (§6; `runtime-testable`)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `INB-01` | runtime-testable | `KIT-04`: chave `(consumer_name, message_id)`, `consumer_name` lógico e estável | dedup pela chave composta | chave só por `message_id`, ou `consumer_name` volátil | §8 `KIT-04` |
| `INB-02` | runtime-testable | `KIT-04`: `status` só `processed`/`rejected`, ambos terminais | dois estados terminais | terceiro estado / estado não terminal | §8 `KIT-04` |
| `INB-03` | runtime-testable | `KIT-04`: inbox só contém mensagens cujo processamento commitou | registro só após commit | registro de processamento não commitado | §4 `CEN-12` |
| `INB-04` | runtime-testable | harness `KIT-06`: a porta devolve classificação, nunca erro de constraint | classificação retornada sob corrida | erro de constraint propagado | §4 `CEN-07` |
| `INB-05` | runtime-testable | `KIT-04`: `concluir` obrigatória em toda transação R1 que commita | `concluir` em R1×D1 e R1×D2 | commit R1 sem `concluir` | §8 `KIT-04` |
| `INB-06` | runtime-testable | `KIT-06`: sob concorrência serializa na chave (exatamente um vencedor) | um R1 aplica o efeito, o outro R2 curto-circuita | dois vencedores aplicam o efeito | §4 `CEN-07` |
| `INB-07` | runtime-testable | `KIT-04`: dedup + efeito local + outbox derivada numa única transação | os três no mesmo commit | dedup/efeito/outbox em transações separadas | §8 `KIT-04` |
| `INB-08` | runtime-testable | `KIT-06`: efeito de broker vem depois do desfecho da transação | ACK/publicação após o commit | efeito de broker antes do commit | §4 `CEN-06` |
| `INB-09` | runtime-testable | `KIT-04`: D3/D4 decididos pela classificação do erro | disposição derivada da classe do erro | disposição por julgamento ad-hoc | §8 `KIT-04` |
| `INB-10` | runtime-testable | `KIT-04`: envelope inválido recusado antes da UoW, não é D4 | envelope inválido → quarantine antes da UoW | envelope inválido tratado como D4 | §8 `KIT-04` |
| `INB-11` | runtime-testable | `KIT-04`: eixo 1 precede eixo 2; eixo 2 só existe sob R1 | ordem eixo 1 → eixo 2 sob R1 | eixo 2 avaliado sem R1 | §8 `KIT-04` |
| `INB-12` | runtime-testable | `KIT-04`: R3 não reemite o rejection event | R3 sem reemissão | R3 reemite o rejection event | §8 `KIT-04` |
| `INB-13` | runtime-testable | `KIT-04`: `payload_hash` estável (H1) e só sobre conteúdo de negócio (H2) | hash estável, conteúdo de negócio | hash inclui envelope/metadados voláteis | §4 `CEN-08` |
| `INB-14` | runtime-testable | `KIT-04`: `retenção_inbox ≥ janela_redelivery` | retenção cobre a janela de redelivery | retenção menor que a janela | §8 `KIT-04` (janelas concretas: FND-06/FND-08 §12) |
| `INB-15` | runtime-testable (evidência) | `KIT-06`: toda operação de replay declara a zona de proteção | replay nomeia a zona (dupla ou só idempotência) | replay sem zona declarada | §4 `CEN-10` |
| `INB-16` | runtime-testable (evidência) | `KIT-06`: purga da inbox é operação com evidência | purga registrada com evidência | purga sem evidência | §8 `KIT-06` |
| `INB-17` | runtime-testable | `KIT-04`: a espera da porta tem teto; o estouro é R1×D3 | timeout → R1×D3 | espera sem teto | §4 `CEN-12` |
| `INB-18` | runtime-testable | `KIT-04`: a classificação devolvida é sempre de transação commitada | classificação pós-commit | classificação de transação não commitada | §4 `CEN-07` |

#### FND-04 — `GAR` (§7; `runtime-testable`, exceto `GAR-01`/`GAR-06`/`GAR-10`/`GAR-11` por inspeção/varredura)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `GAR-01` | structurally reviewable | varredura `V31`: docs/contratos/config sem promessa de exactly-once fim a fim | — | — | §10 `RAS` (`V31`) |
| `GAR-02` | runtime-testable | `KIT-06`: garantia por transação local + outbox, nunca transação distribuída | entrega recuperada pela outbox local | uso de transação distribuída / 2PC banco+broker | §8 `KIT-06`; §4 `CEN-01`..`12` |
| `GAR-03` | runtime-testable | `KIT-06` (`V32`): dedup por inbox não cobre os três casos de efeito | idempotência de efeito cobre `message_id` novo / purga+replay / duas mensagens distintas | inbox tratada como suficiente; o efeito duplica → `DMPF-R004`| §10 `RAS` (`V32`); §8 `KIT-06` |
| `GAR-04` | runtime-testable | oráculo `V32` (reentrega deliberada, harness `KIT-06`): idempotência de efeito | efeito ocorre uma vez sob redelivery (chave natural, `GAR-10`) | efeito duplica sob redelivery / `message_id` novo → `DMPF-R004`| §10 `RAS` (`V32`); §8 `KIT-06` |
| `GAR-05` | runtime-testable | suíte `CEN-01`..`12`: cada failure mode atinge o desfecho automático declarado | recuperação/contenção conforme a categoria da fonte | failure mode sem desfecho automático | §4 `CEN-01`..`12` |
| `GAR-06` | structurally reviewable | inspeção: toda contenção nomeia o pendente e quem decide | — | — | §4 `CEN-01`/`08`/`09`/`11` (inspeção) |
| `GAR-07` | runtime-testable (evidência) | `KIT-06`: contenção preserva informação suficiente para replay conforme | mensagem na DLQ reprocessável sob as mesmas regras | representação na DLQ não permite reprocessar | §4 `CEN-10` (forma do envelope: fronteira FND-05) |
| `GAR-08` | runtime-testable | `KIT-06`: poison message não bloqueia partição nem grupo FIFO | limite de tentativas retira a mensagem; o fluxo segue | retry ilimitado trava a partição/FIFO | §4 `CEN-09` (limiares de retry: fronteira FND-06/FND-08 §12) |
| `GAR-09` | runtime-testable (evidência) | `KIT-06`: replay com ferramenta, auditoria e proteção contra duplicidade | replay auditado e protegido | reenvio manual sem registro/proteção | §4 `CEN-10` (runbook: fronteira FND-08 §12) |
| `GAR-10` | structurally reviewable | inspeção: o domínio declara chave natural permanente da operação | — | — | §10 `RAS` (efeito demonstrado por `V32`/`GAR-04`) |
| `GAR-11` | structurally reviewable | inspeção: quarantine e DLQ distintos; mapeamento declarado quando há os dois | — | — | §9 `FIT` |
| `GAR-12` | runtime-testable | `KIT-06` (observabilidade): consumo expõe profundidade de DLQ/quarantine e taxa de recusa | os sinais expostos | sinais ausentes | §8 `KIT-06` (limiares/alarmes: fronteira FND-08 §12) |

`registro` — **Cobertura.** As 118 regras estão mapeadas: FND-03 = 54 (`UPR-I` 12, `UPR-L` 5, `DEC` 13, `FRT` 4, `MSG-N` 4, `CTR` 7, `ESC` 9); FND-04 = 64 (`BLK` 5, `UOW` 11, `OBX` 18, `INB` 18, `GAR` 12). Nenhum ID de FND-03 ou FND-04 ficou de fora.

`registro` — **IDs cujo instrumento a fonte não permite fechar aqui.** `UPR-I04` é permissão, não obrigação — não há o que verificar. `DEC-08` tem decidibilidade partida: o lado-domínio fecha por `ORA-40` (§7), o lado-entrega é dependência declarada a FND-04, não instanciável neste artefato. `GAR-07`, `GAR-09`, `INB-15` e `INB-16` fecham por **evidência de execução** (cenário 10 de FND-04 §7.3 é operação, não teste automatizado), não por vetor pareado.

`registro` — **Fronteiras FND-08 (remetidas à §12).** `OBX-12` e `GAR-12` verificam aqui a **exposição** dos sinais; os limiares e alarmes são de FND-08. `GAR-08` verifica que a poison message não bloqueia; os limiares de retry são de FND-06/FND-08. `GAR-09` verifica ferramenta/auditoria; o runbook é de FND-08. `INB-14` verifica a invariante `retenção ≥ janela`; os valores concretos das janelas são de FND-06/FND-08. Nenhuma dessas regras é pura fronteira FND-08 — todas têm núcleo verificável neste artefato.

`registro` — **Precisões de mapeamento.** Os 118 IDs são contíguos, sem lacunas. Além de `OBX-02`/`OBX-03` (inspeção) e `GAR-01` (varredura, `V31`), FND-04 §11.2 declara `GAR-06`, `GAR-10` e `GAR-11` como `structurally reviewable` por inspeção: as três recebem critério de inspeção e `—` nas colunas de vetor, não oráculo de execução. `UPR-I11`, cujo enunciado exato em FND-03 §2.2 é «não recebe nem produz tipos gerados de Protobuf», é `import-verifiable`, sem oráculo de execução.

### §13.5 FND-05 e FND-06

`registro` — 195 regras: FND-05 = 64 (`ENV` 25, `PTB` 16, `BUF` 12, `REP` 6, `INT` 5); FND-06 = 131 (`TRP` 54, `RST` 4, `GRP` 19, `KFK` 23, `SQS` 19, `ASY` 4, `COE` 8). `TRP-45` foi retirado na revisão que precedeu a promoção e o número não é reutilizado — não há linha para ele. Nenhuma regra de FND-05 ou FND-06 é `import-verifiable`: as famílias `DMPF-D`/`DMPF-E` de RFC §10.3 nomeiam apenas os defeitos da regra de dependência, instanciada em §9 (`FIT`), não o conteúdo destes dois artefatos. A calibração por modo segue §10.4 (`RAS-10`/`RAS-11`) e §11.1 (`RAS-30`/`RAS-31`).

**`ENV` — envelope, identidade e payload (FND-05 §2–§4) — 25 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `ENV-01` | structurally reviewable | inspeção do `contract package`: só contratos de wire (`wire.codec`), sem lógica | — | — | §9 (`FIT-01`); cf. FND-05 §2.1 |
| `ENV-02` | structurally reviewable | inspeção estática: contrato legível sem I/O; sustenta a localidade de `RAS-40` | — | — | §11.6 (`RAS-40`); cf. FND-05 §2.1 |
| `ENV-03` | structurally reviewable | inspeção: tipo gerado é objeto de wire; a fitness function de dependência barra vazamento ao domínio | — | — | §9 (`FIT-01`/`FIT-02`) |
| `ENV-04` | structurally reviewable | inspeção do contrato: conversão produz envelope + payload de contrato `event` | — | — | §5 (`FIX`); §6.1 (`ORA-02`) |
| `ENV-05` | structurally reviewable | inspeção do repositório: um contrato por tipo, sem campo genérico de escape | — | — | §9; cf. FND-05 §2.2 |
| `ENV-06` | structurally reviewable | inspeção: conversão não reversível pelo contrato | — | — | §5 (`FIX`) |
| `ENV-07` | structurally reviewable | inspeção do contrato do envelope (Protobuf CloudEvents + perfil) | — | — | §11.6 (`RAS-40`); §6.1 |
| `ENV-08` | runtime-testable | validador do perfil (`RAS-40`), local e fail-closed | envelope com os 15 atributos e presença conforme passa | atributo obrigatório ausente → inválido, à quarantine | §11.6 (`RAS-40`); §4.4 (`CEN-26`) |
| `ENV-09` | runtime-testable | `RAS-40`: atributo vive no envelope | atributo do perfil presente no envelope | atributo do perfil embutido no payload → inválido | §11.6 (`RAS-40`) |
| `ENV-10` | runtime-testable | `RAS-40`: charset e limite de nome de extensão | nome de extensão dentro do charset e do limite | nome fora do charset ou do limite → inválido | §11.6 (`RAS-40`) |
| `ENV-11` | runtime-testable | `RAS-40`: conjunto de extensões fechado | só extensões do conjunto | extensão fora do conjunto → à quarantine | §11.6 (`RAS-40`); §4.4 (`CEN-26`) |
| `ENV-12` | runtime-testable | `RAS-40`: predicado decidível de `tenantid` (FND-05 §3.4) | `tenantid` presente quando o predicado exige | ausente sob o predicado, ou valor de preenchimento → inválido | §11.6 (`RAS-40`) |
| `ENV-13` | runtime-testable | `RAS-40` local/fail-closed: conformidade sem desserializar nem rede | validação decide só com atributos | validação que exigisse desserializar o payload ou resolver rede → defeito | §11.6 (`RAS-40`) |
| `ENV-14` | structurally reviewable | inspeção: uma só autoridade de representação por informação | — | — | §6.2 (`ORA-08`..`ORA-13`); cf. FND-05 §4.1 |
| `ENV-15` | structurally reviewable | inspeção do envelope/fixture: `proto_data` com `Any`; `binary_data` ausente | — | — | §5 (`FIX`); §6.1 |
| `ENV-16` | structurally reviewable | inspeção: coerência entre `datacontenttype`, `dataschema`, type URL e `type` | — | — | §11.6 (`RAS-40`); §6.1 (`ORA-02`) |
| `ENV-17` | runtime-testable | oráculo 2 (`ORA-03`): igualdade do `payload_hash` sobre `Any.value` como transportado | hash do consumidor = hash do produtor sobre os mesmos bytes | bytes reescritos no hop → hashes divergem → `DMPF-R002`| §6.1 (`ORA-03`, `ORA-07`); §4.4 (`CEN-14`) |
| `ENV-18` | runtime-testable | oráculo 2, precondição `ORA-07`: hash sem desserializar nem reserializar | hash sobre os bytes transportados | hash recomputado após reserialização → diverge → `DMPF-R002`| §6.1 (`ORA-07`) |
| `ENV-19` | runtime-testable | oráculo 2 recomputa por `ENV-17`: SHA-256, hex minúsculo, fórmula v1 | SHA-256 hex sobre `Any.value` confere | algoritmo/forma divergente → hashes não batem → `DMPF-R002`| §6.1 (`ORA-03`) |
| `ENV-20` | structurally reviewable | gate de versão: fórmula não transportada, mudá-la é breaking do perfil | — | — | §4.6 (compat); cf. FND-05 §6.6 |
| `ENV-21` | structurally reviewable | inspeção: versão do contrato = major do pacote | — | — | §4.6 (`CEN-40`) |
| `ENV-22` | structurally reviewable | inspeção: `type` é a autoridade da versão | — | — | §4.6 (`CEN-40`); §6.1 |
| `ENV-23` | structurally reviewable | inspeção; `RAS-40` fecha: `dataschema` é identificador, não endereço a resolver | — | — | §11.6 (`RAS-40`) |
| `ENV-24` | runtime-testable | oráculo 3 (`ORA-04`): identidade de bytes de `Any.value`, no escopo que `ENV-24` fixa | bytes idênticos na publicação sem reserialização, na contenção e no replay | reserialização na contenção → bytes divergem → `DMPF-R003`| §6.1 (`ORA-04`); §4.4 (`CEN-14`) |
| `ENV-25` | runtime-testable | oráculo 1 (`ORA-02`) sobre o envelope contido | atributos preservados por valor, sem acréscimo | atributo alterado ou acrescido na contenção | §6.1 (`ORA-02`) |

**`PTB` — política Protobuf (FND-05 §5) — 16 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `PTB-01` | structurally reviewable | gate `buf lint`: forma do pacote com categoria e major | — | — | §8.3 (`KIT-10`); cf. FND-05 §5.1 |
| `PTB-02` | structurally reviewable | gate `buf lint`: major-only, sem versão menor nem sufixo de instabilidade | — | — | §8.3 (`KIT-10`) |
| `PTB-03` | structurally reviewable | gate `buf lint`: forma do `type` do envelope | — | — | §8.3 (`KIT-10`) |
| `PTB-04` | structurally reviewable | inspeção/lint: nome sem transporte, destino, ambiente nem tecnologia | — | — | §8.3 (`KIT-10`) |
| `PTB-05` | structurally reviewable | gate `buf breaking`: número de campo nunca reutilizado | — | — | §4.6 (`CEN-36`); §8.3 (`KIT-10`) |
| `PTB-06` | structurally reviewable | gate `buf breaking`: `reserved` de número e nome na remoção | — | — | §8.3 (`KIT-10`) |
| `PTB-07` | structurally reviewable | gate `buf breaking`: tipo e label de campo publicado não mudam | — | — | §4.6 (`CEN-39`); §8.3 (`KIT-10`) |
| `PTB-08` | structurally reviewable | gate `buf breaking`: evolução aditiva passa; reuso de número reprova | — | — | §4.6 (`CEN-36`); §8.3 (`KIT-10`) |
| `PTB-09` | structurally reviewable | gate `buf lint`: zero de enum = `_UNSPECIFIED` | — | — | §4.6 (`CEN-37`); §8.3 (`KIT-10`) |
| `PTB-10` | runtime-testable | oráculo 3 (`ORA-04`) + toolchain certificada (`BUF-07`) | consumidor `N-1` reserializa e o campo novo permanece nos bytes | biblioteca descarta desconhecidos → `payload_hash` diverge → `DMPF-R002`| §4.6 (`CEN-38`); §6.1 (`ORA-04`) |
| `PTB-11` | runtime-testable | oráculo executável de round-trip de FND-09 (`ORA-01`) | consumidor gerado antes do campo processa a mensagem `N` e ignora o campo, sem perda | mudança de tipo (`int64`→`string`) → falha/trunca, barrada por `PTB-07` (`buf breaking`) | §4.6 (`CEN-39`); §6.1 (`ORA-01`) |
| `PTB-12` | structurally reviewable | revisão de PR sobre a estrutura do repositório: majors como pacotes/artefatos distintos | — | — | §4.6 (`CEN-40`) |
| `PTB-13` | structurally reviewable | inspeção do contrato: depreciação declarada, com prazo e sucessor | — | — | §11.1 (`RAS-30`); cf. FND-05 §5.4 |
| `PTB-14` | structurally reviewable | inspeção: janela mínima de suporte de 180 dias declarada | — | — | §11.1 (`RAS-30`); cf. FND-05 §5.4 |
| `PTB-15` | structurally reviewable | inspeção do commit de remoção: janela vencida e sem consumidor declarado | — | — | §11.1 (`RAS-30`); cf. FND-05 §7.2 |
| `PTB-16` | structurally reviewable | gate `buf`: depreciação não relaxa §5.2 nem os gates de §6 | — | — | §8.3 (`KIT-10`) |

**`BUF` — governança e gates (FND-05 §6) — 12 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `BUF-01` | structurally reviewable | inspeção do `buf.yaml`: workspace v2 com módulos explícitos | — | — | §8.3 (`KIT-10`); cf. FND-05 §6.1 |
| `BUF-02` | structurally reviewable | inspeção do `buf.lock`: versionado, dependência declarada | — | — | §8.3 (`KIT-10`) |
| `BUF-03` | structurally reviewable | gate `buf lint STANDARD`, sem exceção por diretório | — | — | §8.3 (`KIT-10`) |
| `BUF-04` | structurally reviewable | gate `buf breaking FILE` | — | — | §8.3 (`KIT-10`) |
| `BUF-05` | structurally reviewable | inspeção do baseline: identidade declarada e resolvível | — | — | §8.3 (`KIT-10`) |
| `BUF-06` | structurally reviewable | inspeção: pinning de CLI, versão e revisão de plugin | — | — | §8.3 (`KIT-10`) |
| `BUF-07` | structurally reviewable | critérios de toolchain certificada; sustenta `PTB-10` (oráculo 3) | — | — | §8.3 (`KIT-10`); §4.6 (`CEN-38`) |
| `BUF-08` | structurally reviewable | inspeção da máquina de bootstrap, fail-closed | — | — | §8.3 (`KIT-10`) |
| `BUF-09` | structurally reviewable | inspeção: validação no repositório é o Buf, local | — | — | §8.3 (`KIT-10`) |
| `BUF-10` | structurally reviewable | inspeção do `buf.gen.yaml`: managed mode, `.proto` neutro de linguagem | — | — | §8.3 (`KIT-10`) |
| `BUF-11` | runtime-testable | dupla geração byte a byte do pipeline (`KIT-10`), comparada por reprodutibilidade e por drift | duas gerações limpas por stack idênticas | gerações divergentes ou drift do artefato versionado | §8.3 (`KIT-09`..`KIT-11`) |
| `BUF-12` | structurally reviewable | inspeção do pipeline: gates barram merge, sem bypass nem julgamento humano | — | — | §8.3 (`KIT-10`, `KIT-09`) |

**`REP` — repositório de contratos (FND-05 §7) — 6 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `REP-01` | structurally reviewable | inspeção da árvore: caminho espelha o pacote, com major no path | — | — | §4.6 (`CEN-40`); §8.3 |
| `REP-02` | structurally reviewable | gate de drift + revisão: código gerado versionado, nunca editado à mão | — | — | §8.3 (`KIT-10`) |
| `REP-03` | structurally reviewable | inspeção do `CODEOWNERS`: owner por bounded context, e é equipe | — | — | §11.1 (`RAS-30`); cf. FND-05 §7.1 |
| `REP-04` | structurally reviewable | inspeção do registro de consumidores por major | — | — | §11.1 (`RAS-30`); cf. FND-05 §7.2 |
| `REP-05` | structurally reviewable | revisão de owner no PR, além dos gates | — | — | §11.1 (`RAS-30`) |
| `REP-06` | structurally reviewable | inspeção: `openapi/` e `asyncapi/` registrados, não normatizados | — | — | §11.1 (`RAS-30`) |

**`INT` — interoperabilidade e fixture (FND-05 §8) — 5 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `INT-01` | structurally reviewable | inspeção: golden fixture como fonte única, independente de stack | — | — | §5.1 (`FIX`) |
| `INT-02` | structurally reviewable | gate de PR: toda categoria `event` tem fixture no PR que a publica | — | — | §5.1 (`FIX`) |
| `INT-03` | runtime-testable | pipeline de round-trip nas duas direções (`ORA-01`) | round-trip Go→TS e TS→Go passa | uma direção reprova | §5, §6.1 (`ORA-01`); §8.3 (`KIT-06`) |
| `INT-04` | runtime-testable | oráculos 1/2/3 reportados em separado (`ORA-06`; forma em `FIX-12`/`FIX-13`) | reprovar só no oráculo 3 emite diagnóstico distinto do oráculo 1 | critério único «bytes conferem», sem separação por oráculo → `DMPF-R001`| §6.1 (`ORA-06`); §5.4 (`FIX-12`/`FIX-10`) |
| `INT-05` | runtime-testable | `ORA-05`: restringe **somente** o oráculo 3 entre produtores independentes; oráculos 1 e 2 rodam em cada fluxo | duas stacks produzem bytes diferentes do mesmo conteúdo sem reprovar | exigir identidade de bytes entre produtores independentes → reprovação incorreta → `DMPF-R003`| §6.1 (`ORA-05`) |

**`TRP` — invariantes transversais de transporte (FND-06) — 54 regras** (`TRP-45` retirado, sem linha)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `TRP-01` | structurally reviewable | inspeção do sujeito: comportamento do `provider` | — | — | §11.1 (`RAS-30`); cf. FND-06 §1.3 |
| `TRP-02` | structurally reviewable | inspeção: extensão a `app` só no conjunto fechado, com origem citada | — | — | §11.1 (`RAS-30`); cf. FND-06 §1.3 |
| `TRP-03` | structurally reviewable | inspeção `M4`: fora de `provider`/extensão a regra é inválida | — | — | §11.1 (`RAS-30`); cf. FND-06 §1.3 |
| `TRP-04` | structurally reviewable | inspeção da política contra as portas do bloco; violação concreta reprova sob `DMPF-E001` (RFC §10.3) | — | — | §9 (`FIT-02`); §11.1 |
| `TRP-05` | structurally reviewable | varredura `V31` de documentação, contratos e config por promessa de exactly-once | — | — | §4.4 (`CEN-24`); §10.5 (`RAS-12`) |
| `TRP-06` | structurally reviewable | varredura: mecanismo nativo de broker é otimização, nunca substituto da idempotência | — | — | §4.4 (`CEN-24`) |
| `TRP-07` | structurally reviewable | inspeção: endereço concreto na config do `provider`, no composition root | — | — | §11.1 (`RAS-30`); cf. FND-06 §4.1 |
| `TRP-08` | runtime-testable | kit de conformidade de provider (`KIT-04`) | `provider` construído com endereço resolvido opera | endereço não resolvido → recusa na construção, não na 1ª publicação | §8 (`KIT-04`); cf. FND-06 §4.2 |
| `TRP-09` | runtime-testable | harness (`KIT-06`): estabilidade do endereço por mensagem | todas as tentativas de uma mensagem usam o mesmo endereço | retry publica em endereço diferente | §4.5 (`CEN-31`); §8 (`KIT-06`) |
| `TRP-10` | structurally reviewable | inspeção: mudar binding é migração, não configuração | — | — | §11.1 (`RAS-30`); cf. FND-06 §4.2 |
| `TRP-11` | runtime-testable | cenário `CEN-22`: chave de partição vem do envelope | chave do envelope → mesma partição e ordem | chave inferida do payload → partição/ordem erradas | §4.4 (`CEN-22`) |
| `TRP-46` | structurally reviewable | inspeção: estabilidade exige estado persistido, gravado antes do 1º I/O | — | — | §4.5 (`CEN-31`); §11.1 |
| `TRP-12` | structurally reviewable | inspeção: instante da publicação é observação do `provider`, não entra no envelope | — | — | §11.1 (`RAS-30`); cf. FND-06 §4.3 |
| `TRP-13` | runtime-testable | oráculos 2 e 3 (`ORA-03`/`ORA-04`): bytes de `Any.value` idênticos | hop conforme entrega os bytes publicados; hash confere | hop reescreve os bytes → hash diverge, redelivery vira `R4` → `DMPF-R002`| §6.1 (`ORA-03`/`ORA-04`); §4.4 (`CEN-14`) |
| `TRP-14` | runtime-testable | oráculo 3: o gesto proibido (reserialização) é nomeado | payload transportado é o mesmo que entrou | camada reserializa `Any.value` → bytes divergem → `DMPF-R003`| §4.5 (`CEN-27`, `CEN-33`) |
| `TRP-15` | runtime-testable | oráculo 2 (`ORA-03`/`ORA-07`): hash sobre os bytes do payload | hash sobre `Any.value`, não sobre a forma de transporte | hash sobre a forma de transporte → não recomputável → `DMPF-R002`| §6.1 (`ORA-03`, `ORA-07`) |
| `TRP-16` | structurally reviewable | inspeção: `não conforme` é relativo ao caminho que alimenta a inbox, não absoluto | — | — | §4.5 |
| `TRP-17` | runtime-testable | oráculo 3: interceptor observa, não substitui | objeto publicado é o mesmo que entrou | SMT/interceptor reserializa o valor → bytes divergem → `DMPF-R003`| §4.5 (`CEN-27`); §6.1 (`ORA-04`) |
| `TRP-18` | structurally reviewable | inspeção do hop: nenhum atributo do perfil migra para header/atributo | — | — | §4.5 (`CEN-33`); §11.6 |
| `TRP-19` | runtime-testable | oráculo 3: codificação textual aplicada exatamente uma vez (hop conforme sob `TRP-19`, não `CEN-27`..`CEN-30`) | corpo textual codificado uma vez, round-trip confere | camada intermediária recodifica → duplo Base64, bytes divergem → `DMPF-R003`| §6.1 (`ORA-04`); cf. FND-06 §5.2 |
| `TRP-20` | structurally reviewable | inspeção: atributo/metadado lateral não entra no envelope nem no cálculo do hash | — | — | §6.1 (`ORA-07`); cf. FND-06 §5.2 |
| `TRP-21` | structurally reviewable | dependência declarada: claim-check condicionado a evolução do perfil (ANC-03/FND-05); até lá não conforme | — | — | §12.3 (dependências declaradas) |
| `TRP-22` | structurally reviewable | `CEN-19` (inspeção): o canal declara qual dos três relógios está declarando | — | — | §4.4 (`CEN-19`); cf. FND-06 §6.1 |
| `TRP-23` | structurally reviewable + runtime-testable | `CEN-19`: fórmula da `janela_redelivery` declarada (inspeção) + janela efetiva (execução) | `retenção_inbox ≥ janela_redelivery` sobre valor conhecido (`INB-14`) | canal sem fórmula ou parâmetro → `INB-14` não verificável | §4.4 (`CEN-19`) |
| `TRP-24` | structurally reviewable + runtime-testable | `CEN-19`: fórmula por canal, com variáveis e unidade, produz limite superior fechado | fórmula com limite superior fechado | fórmula sem limite fechado | §4.4 (`CEN-19`) |
| `TRP-24b` | structurally reviewable + runtime-testable | `CEN-19`: o canal declara os parâmetros que instanciam a fórmula | parâmetros instanciados → janela calculável | parâmetro omitido → janela indeterminada | §4.4 (`CEN-19`) |
| `TRP-25` | runtime-testable | `CEN-19`: replay operacional não entra na `janela_redelivery` | replay operacional fora da janela | replay contado como redelivery na janela | §4.4 (`CEN-19`) |
| `TRP-26` | runtime-testable | `CEN-15`: confirmação depois do commit local | confirma no broker após o commit local | confirma antes ou na mesma operação do commit | §4.4 (`CEN-15`) |
| `TRP-27` | runtime-testable | `CEN-15`: gesto de ACK por disposição (7 disposições, falha injetada) | cada disposição confirma pelo gesto correto | falha entre as ações produz redelivery/duplicata, nunca perda | §4.4 (`CEN-15`) |
| `TRP-28` | runtime-testable | `CEN-17`: em Kafka, `enable.auto.commit=false` | consumidor com auto-commit desligado | auto-commit ligado → offset avança sem disposição | §4.4 (`CEN-17`) |
| `TRP-29` | runtime-testable | `CEN-16`: offset commitado é `n+1`, nunca ultrapassa registro não disposto | valor commitado = sucessor do último contíguo disposto | commitar `n` → reentrega `n` indefinidamente | §4.4 (`CEN-16`) |
| `TRP-30` | runtime-testable | `CEN-15`: na contenção, publicação na DLQ precede o avanço do offset/delete | DLQ antes do avanço do offset | avanço do offset antes da DLQ → perda | §4.4 (`CEN-15`) |
| `TRP-31` | structurally reviewable | inspeção: limite de tentativas obrigatório e efeito sobre a ordem declarado (timing/orçamento de retry é fronteira FND-08, §12) | — | — | §11.1 (`RAS-30`); §12 |
| `TRP-32` | structurally reviewable | inspeção: canal com ordenação por chave não usa retry em canal separado | — | — | §4.4 (`CEN-22`); §11.1 |
| `TRP-47` | runtime-testable | `CEN-17`: em Kafka, não commitar não reentrega | ausência de commit não reentrega enquanto durar a atribuição | tratar não-commit como reentrega | §4.4 (`CEN-17`) |
| `TRP-48` | runtime-testable | `CEN-18`: perda de partição invalida o resultado em curso | cancela o trabalho pendente e não commita o restante | commitar após perder a atribuição | §4.4 (`CEN-18`) |
| `TRP-52` | runtime-testable | `KIT-06`: o `provider` **expõe** a contagem de tentativas — do contador do broker ou de metadado lateral que ele propague —, e nunca dentro do envelope. Limiar, orçamento e alarme sobre essa contagem são de FND-08 (§12.1); a exposição é daqui | `provider` expõe a contagem por metadado lateral, consultável no consumo | `provider` não expõe contagem, ou a carrega dentro do envelope | §8 `KIT-06`; §4 `CEN-17` |
| `TRP-33` | runtime-testable | `CEN-26` + `RAS-40`: validação no `consumer adapter`, antes de efeito e da inbox | envelope validado antes de qualquer efeito | efeito ou inbox antes da validação | §4.4 (`CEN-26`); §11.6 (`RAS-40`) |
| `TRP-34` | runtime-testable | `RAS-40`: validação local e fail-closed | na dúvida, envelope inválido | validação que consulta rede ou falha aberta | §11.6 (`RAS-40`); §4.4 (`CEN-26`) |
| `TRP-35` | runtime-testable | `CEN-26`: rejeição é a contenção, não o nack | inválido contido, não devolvido ao broker | `nack` do inválido → loop | §4.4 (`CEN-26`); §11.6 (`RAS-40`) |
| `TRP-36` | runtime-testable | `CEN-26`: destino do inválido é a quarantine, não a DLQ do canal | inválido roteado à quarantine | inválido na DLQ do canal | §4.4 (`CEN-26`); §11.6 (`RAS-40`) |
| `TRP-54` | structurally reviewable | inspeção: `provider` não trata a superfície de contenção como infraestrutura | — | — | §4.4 (`CEN-26`); §11.1 |
| `TRP-49` | runtime-testable | `CEN-21`: teto de tamanho aplicado antes de desserializar | mensagem acima do teto contida sem decode | decode antes de aplicar o teto | §4.4 (`CEN-21`) |
| `TRP-50` | runtime-testable | `CEN-21`: `provider` não expande conteúdo recebido sem limite | conteúdo comprimido não expande além do teto | expansão sem limite | §4.4 (`CEN-21`) |
| `TRP-51` | runtime-testable | `CEN-21`: profundidade de aninhamento limitada e verificada antes do decode | aninhamento dentro do limite antes do decode | decode do payload antes de verificar a profundidade | §4.4 (`CEN-21`) |
| `TRP-37` | structurally reviewable | inspeção: escolha de transporte por critérios declarados | — | — | §11.1 (`RAS-30`); cf. FND-06 §8.1 |
| `TRP-38` | structurally reviewable | inspeção da matriz de decisão de transporte | — | — | §11.1 (`RAS-30`); cf. FND-06 §8.2 |
| `TRP-39` | structurally reviewable | inspeção: Kafka é o transporte-alvo do assíncrono de domínio | — | — | §11.1 (`RAS-30`); cf. FND-06 §8.2 |
| `TRP-40` | structurally reviewable | inspeção: SNS/SQS normatizado, não tolerado | — | — | §11.1 (`RAS-30`); cf. FND-06 §8.2 |
| `TRP-41` | structurally reviewable | dependência declarada: política de §11 é norma de desenho até revisão de infraestrutura, não autorização de operação | — | — | §12.3; §14 |
| `TRP-42` | structurally reviewable | varredura `V31`: nenhum mecanismo de transporte substitui as duas camadas de consumo | — | — | §4.4 (`CEN-24`) |
| `TRP-43` | structurally reviewable | varredura: dedup nativa descrita pelo efeito, nunca pela promessa | — | — | §4.4 (`CEN-24`) |
| `TRP-44` | structurally reviewable | varredura: `provider` não é o lugar da idempotência de efeito | — | — | §4.4 (`CEN-24`) |
| `TRP-53` | structurally reviewable | inspeção: gesto por categoria de erro por composição, sem segunda tabela | — | — | §11.1 (`RAS-30`); cf. FND-06 §12.4 |

**`RST` — REST e JSON como interface externa (FND-06 §9) — 4 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `RST-01` | structurally reviewable | inspeção: REST/HTTP/JSON só para consumidor externo à organização | — | — | §4.5 (`TRP-16`); cf. FND-06 §9 |
| `RST-02` | structurally reviewable | inspeção: `provider` HTTP não retenta método sem semântica idempotente | — | — | §11.1 (`RAS-30`); cf. FND-06 §9 |
| `RST-03` | structurally reviewable | inspeção: timeout em toda chamada de saída, derivado do deadline | — | — | §4.4 (`CEN-20`); §11.1 |
| `RST-04` | structurally reviewable | inspeção: não expõe canal externo cujo contrato publicado não possa referenciar | — | — | §11.1 (`RAS-30`); cf. FND-06 §9 |

**`GRP` — gRPC como síncrono interno (FND-06 §10) — 19 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `GRP-01` | structurally reviewable | inspeção: gRPC/HTTP2 é o síncrono interno quando as duas pontas são nossas | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.1 |
| `GRP-02` | structurally reviewable | inspeção: três perfis com critério de escolha declarado | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.1 |
| `GRP-03` | structurally reviewable | inspeção: Connect JSON e transcodificação gRPC-JSON não alimentam inbox | — | — | §4.5 (`CEN-29`, `CEN-30`) |
| `GRP-04` | runtime-testable | `CEN-20`: toda chamada gRPC de saída carrega deadline | chamada com deadline propagado | chamada sem deadline | §4.4 (`CEN-20`) |
| `GRP-05` | runtime-testable | `CEN-20`: deadline propagado, nunca reiniciado | prazo restante decresce por salto | salto reinicia o deadline | §4.4 (`CEN-20`) |
| `GRP-06` | runtime-testable | `CEN-20`: no wire transmite a duração restante, receptor reconstrói o instante | receptor reconstrói o prazo a partir da duração | transmite instante absoluto de relógio distinto | §4.4 (`CEN-20`) |
| `GRP-07` | runtime-testable | `CEN-20`: propaga cancelamento nos dois sentidos | cadeia cancelada em vez de exceder a borda | cancelamento não propagado | §4.4 (`CEN-20`) |
| `GRP-16` | structurally reviewable | inspeção: prazo declarado por método, não por serviço nem processo | — | — | §4.4 (`CEN-20`); §11.1 |
| `GRP-17` | runtime-testable | `CEN-20`: prazo de método menor que o do chamador, com folga do salto | prazo do método < prazo do chamador | prazo do método ≥ chamador | §4.4 (`CEN-20`) |
| `GRP-18` | structurally reviewable | inspeção: prazo da borda derivado do requisito de quem chama, não do tempo atual | — | — | §11.1 (`RAS-30`); §4.4 (`CEN-20`) |
| `GRP-08` | structurally reviewable | inspeção: política de retry declarada por método, derivada da idempotência | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.3 |
| `GRP-09` | structurally reviewable | inspeção: método sem idempotência comprovada não recebe retry automático | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.3 |
| `GRP-10` | structurally reviewable | inspeção: backoff exponencial com jitter configurado | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.3 |
| `GRP-11` | structurally reviewable | inspeção: balanceamento entre endereços resolvidos configurado explicitamente | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.3 |
| `GRP-12` | structurally reviewable | inspeção: serviço expõe verificação de saúde pelo protocolo padrão | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.3 |
| `GRP-12b` | structurally reviewable | inspeção: cliente habilita a verificação e usa política de balanceamento compatível | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.3 |
| `GRP-13` | structurally reviewable | inspeção: modelo de erro é o do gRPC (código, mensagem, detalhes) | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.4 |
| `GRP-14` | structurally reviewable | inspeção: mapeamento de código na transcodificação segue a tabela canônica | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.4 |
| `GRP-15` | structurally reviewable | inspeção: TLS obrigatório em qualquer transporte gRPC em produção | — | — | §11.1 (`RAS-30`); cf. FND-06 §10.5 |

**`KFK` — Kafka (FND-06 §11) — 23 regras** (política prospectiva sem base as-is; `TRP-41` limita a norma de desenho)

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `KFK-01` | structurally reviewable | inspeção: nome do tópico é endereço concreto, vive na config do `provider` | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.1 |
| `KFK-01b` | structurally reviewable | inspeção: derivação do nome é transformação total, declarada por canal | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.1 |
| `KFK-01c` | structurally reviewable | inspeção: alfabeto uniforme em todos os segmentos, prefixo de ambiente incluído | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.1 |
| `KFK-02` | structurally reviewable | inspeção: ambiente não entra no nome sob isolamento por cluster/conta | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.1 |
| `KFK-03` | structurally reviewable | inspeção: por padrão, um tópico transporta um tipo de evento | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.1 |
| `KFK-03b` | structurally reviewable | inspeção: múltiplos tipos no mesmo tópico exigem as condições cumulativas | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.1 |
| `KFK-04` | structurally reviewable | inspeção: contagem de partições declarada, com particionador e codificação da chave | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.1 |
| `KFK-04b` | structurally reviewable | inspeção: aumentar partições é operação de migração | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.1 |
| `KFK-05` | runtime-testable | `CEN-22`: chave do registro é a chave de partição do envelope | mesma chave → mesma partição e ordem | chave divergente da do envelope | §4.4 (`CEN-22`) |
| `KFK-06` | runtime-testable | `CEN-22`: ordenação garantida por partição, pela chave, nunca global | ordem preservada dentro da partição | promessa de ordem global | §4.4 (`CEN-22`) |
| `KFK-07` | structurally reviewable | inspeção: grupo de consumo declarado por canal e consumidor lógico | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.2 |
| `KFK-08` | structurally reviewable | inspeção: `consumer_name` da inbox é o consumidor lógico, não o grupo de consumo | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.2 |
| `KFK-09` | runtime-testable | `CEN-22`: concorrência de consumo respeita a partição | concorrência sem violar a ordem por partição | consumo concorrente reordena a partição | §4.4 (`CEN-22`) |
| `KFK-10` | runtime-testable | `CEN-17`: retry inline com backoff, limitado | laço/`pause`+`seek` sobre o mesmo registro, dentro do intervalo | retry que avança o offset ou excede o intervalo | §4.4 (`CEN-17`) |
| `KFK-11` | structurally reviewable | inspeção: canal com ordenação declarada não usa tópico de retry | — | — | §4.4 (`CEN-22`); §11.1 |
| `KFK-12` | runtime-testable | `CEN-15`: DLQ nomeada, publicação nela precede o avanço do offset | DLQ antes do avanço do offset | offset avança antes da DLQ | §4.4 (`CEN-15`) |
| `KFK-13` | structurally reviewable | dependência declarada: regras de registry condicionais a `ADR-DMPF-N` | — | — | §12.3 (dependências declaradas) |
| `KFK-14` | structurally reviewable | inspeção: registry é catálogo/contenção, nunca pré-requisito de desserialização | — | — | §12.3; §11.1 |
| `KFK-15` | structurally reviewable | inspeção: cliente de produção com auto-registro de schema desabilitado | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.4 |
| `KFK-16` | structurally reviewable | inspeção: identificador do schema derivado do canal, não do tipo do registro | — | — | §11.1 (`RAS-30`); cf. FND-06 §11.4 |
| `KFK-17` | structurally reviewable | inspeção: compatibilidade do registry não mais permissiva que o gate de contrato | — | — | §8.3 (`KIT-10`); §11.1 |
| `KFK-18` | structurally reviewable | inspeção: validação no broker não substitui a validação no cliente (`RAS-40`) | — | — | §11.6 (`RAS-40`); §11.1 |
| `KFK-19` | structurally reviewable | inspeção: prazo de processamento do registro menor que o intervalo máximo entre buscas | — | — | §11.1 (`RAS-30`); cf. FND-06 §12.4 |

**`SQS` — SNS e SQS (FND-06 §12) — 19 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `SQS-01` | structurally reviewable | inspeção: corpo é o envelope serializado, codificado uma única vez (`TRP-19`) | — | — | §6.1 (`ORA-04`); cf. FND-06 §5.2 |
| `SQS-02` | structurally reviewable | inspeção: entrega de SNS para SQS usa modo bruto (raw delivery) | — | — | §4.5 (`CEN-28`) |
| `SQS-03` | structurally reviewable | inspeção: atributos transportam só metadado de operação e roteamento | — | — | §11.1 (`RAS-30`); §5.2 (`TRP-20`) |
| `SQS-03b` | structurally reviewable | inspeção: número de atributos limitado e validado antes da publicação | — | — | §11.1 (`RAS-30`); cf. FND-06 §12.1 |
| `SQS-03c` | structurally reviewable | varredura `V31`: filtro de assinatura vedado em tópico FIFO (sustenta a vedação de exactly-once) | — | — | §4.4 (`CEN-24`); §10.5 (`RAS-12`) |
| `SQS-04` | structurally reviewable | inspeção: escolha FIFO/standard declarada por canal, com o critério de §8.1 | — | — | §11.1 (`RAS-30`); §4.4 (`CEN-22`) |
| `SQS-05` | runtime-testable | `CEN-23`: derivação de `MessageGroupId` em FIFO | grupo derivado dentro do limite, preservando igualdade | derivação que perde a igualdade de grupo | §4.4 (`CEN-23`) |
| `SQS-06` | runtime-testable | `CEN-23`: `MessageDeduplicationId` derivado de `source`, id e `payload_hash` | identificadores iguais → dedup igual | derivação que colide ou diverge | §4.4 (`CEN-23`) |
| `SQS-06b` | runtime-testable | `CEN-23`: derivação de `SQS-05`/`SQS-06` dentro dos limites do transporte | valores dentro do limite preservando igualdade | valor fora do limite do transporte | §4.4 (`CEN-23`) |
| `SQS-07` | runtime-testable | harness (`KIT-06`): poison message em FIFO | poison confirmada/retirada libera o grupo | poison não tratada bloqueia o grupo indefinidamente | §8 (`KIT-06`) |
| `SQS-08` | runtime-testable | harness (`KIT-06`): visibilidade estendida enquanto processa, ato explícito | extensão explícita durante o processamento | visibilidade não estendida ou estendida por default | §8 (`KIT-06`) |
| `SQS-08b` | structurally reviewable | inspeção: extensão tem teto; trabalho que possa excedê-lo é decomposto | — | — | §11.1 (`RAS-30`); cf. FND-06 §12.3 |
| `SQS-09` | runtime-testable | `CEN-15`: delete pelo id de recebimento, depois do commit local | delete após o commit local | delete antes do commit | §4.4 (`CEN-15`) |
| `SQS-10` | runtime-testable | `CEN-15`: falha transitória não deleta e não estende indefinidamente | falha transitória retorna à fila para redelivery | delete ou extensão indefinida na falha | §4.4 (`CEN-15`) |
| `SQS-11` | structurally reviewable | inspeção: estratégia de contenção escolhida por disposição, não por canal | — | — | §4.4 (`CEN-15`); §11.1 |
| `SQS-11b` | structurally reviewable | inspeção: os dois mecanismos não se aplicam à mesma disposição | — | — | §11.1 (`RAS-30`); cf. FND-06 §12.3 |
| `SQS-12` | structurally reviewable | inspeção: limiar do claim-check é o tamanho da mensagem | — | — | §12.3; §11.1 |
| `SQS-12b` | structurally reviewable | dependência declarada: claim-check depende da condição de `TRP-21` | — | — | §12.3 (dependências declaradas) |
| `SQS-13` | structurally reviewable | inspeção: prazo de processamento declarado, menor que a visibilidade aplicada | — | — | §11.1 (`RAS-30`); cf. FND-06 §12.4 |

**`ASY` — catalogação de canal assíncrono (FND-06 §16) — 4 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `ASY-01` | structurally reviewable | inspeção: `provider` não opera canal assíncrono não catalogado | — | — | §11.1 (`RAS-30`); cf. FND-06 §16 |
| `ASY-02` | structurally reviewable | inspeção do documento de canal: o que o `provider` precisa encontrar | — | — | §11.1 (`RAS-30`); cf. FND-06 §16 |
| `ASY-03` | structurally reviewable | inspeção: documento de canal referencia o contrato, não o duplica | — | — | §11.1 (`RAS-30`); cf. FND-06 §16 |
| `ASY-04` | structurally reviewable | inspeção: catalogação não introduz atributo de envelope (`RAS-40`) | — | — | §11.6 (`RAS-40`); §11.1 |

**`COE` — coexistência dos dois transportes (FND-06 §15) — 8 regras**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `COE-01` | structurally reviewable + runtime-testable | `CEN-35` (`V-COE-5`): inspeção de config + execução | um canal lógico tem um transporte por vez | mesmo canal publicado nos dois transportes → duas ordens independentes | §4.5 (`CEN-35`) |
| `COE-02` | runtime-testable | `CEN-31` (`V-COE-1`): binding estável por mensagem e por tentativa | retry publica no mesmo transporte | retry publica no outro transporte (detectável por `TRP-46`) | §4.5 (`CEN-31`) |
| `COE-03` | structurally reviewable | inspeção: troca de transporte é migração, com drenagem/transferência declarada | — | — | §4.5 (`CEN-34`); §11.1 |
| `COE-04` | runtime-testable | `CEN-25`/`CEN-31`/`CEN-32`: `consumer_name` lógico e estável entre transportes | mesmo `consumer_name` → redelivery cross-transport absorvida como `R2` | `consumer_name` divergente → duas entradas na inbox, efeito duplicado | §4.4 (`CEN-25`); §4.5 (`CEN-32`) |
| `COE-05` | runtime-testable | `CEN-33` (`V-COE-3`): ponte preserva a identidade da mensagem | ponte preserva `Any.value` byte a byte | ponte reserializa dentro de `Any.value` → `payload_hash` diverge, `R4` | §4.5 (`CEN-33`) |
| `COE-06` | runtime-testable | `CEN-31`/`CEN-34`: não há ordenação entre os dois transportes, e a promessa não é feita | ausência de promessa de ordem cross-transport | promessa de ordem entre transportes independentes | §4.5 (`CEN-31`, `CEN-34`) |
| `COE-07` | runtime-testable | `CEN-34` (`V-COE-4`): migração de canal com ordenação requer corte com drenagem | migração com drenagem preserva a ordem | migração sem drenagem → mesma chave nos dois transportes, ordem inexistente | §4.5 (`CEN-34`) |
| `COE-08` | runtime-testable | `CEN-25`: canal sem ordenação admite drenagem paralela declarada | drenagem paralela como transferência declarada de backlog | drenagem paralela tratada como ausência de transferência | §4.4 (`CEN-25`) |

`registro` — **Contagem.** FND-05 = 64 (`ENV` 25, `PTB` 16, `BUF` 12, `REP` 6, `INT` 5). FND-06 = 131 (`TRP` 54, `RST` 4, `GRP` 19, `KFK` 23, `SQS` 19, `ASY` 4, `COE` 8). Total 195. Três precisões contadas na fonte: (a) `TRP` soma 54 **ativas** — `TRP-01`..`TRP-54` menos `TRP-45` (retirado) mais `TRP-24b`; a notação «TRP-01 a TRP-54» do inventário oculta o `TRP-24b` e o buraco de `TRP-45`, mas a soma bate; (b) `COE` são `COE-01`..`COE-08` (8) — as ocorrências `COE-1`..`COE-5` no texto são fragmentos de `V-COE-1`..`V-COE-5`, não IDs de regra; (c) `GRP` são `GRP-01`..`GRP-18` mais `GRP-12b` (19), e `KFK`/`SQS` batem com os sufixos de letra listados em §18.5 (`KFK` +`01b`/`01c`/`03b`/`04b`; `SQS` +`03b`/`03c`/`06b`/`08b`/`11b`/`12b`).

`registro` — **O que não foi coberto por vetor de execução, e por quê.** (1) `TRP-45` não tem linha: retirado na revisão que precedeu a promoção (a regra contrariava FND-04 §7.2), número não reutilizado. (2) `TRP-52` **não** é fronteira: FND-06 §6.2 já decide que o `provider` expõe a contagem de tentativas, e essa exposição é decidível aqui (`KIT-06`); só limiar, orçamento e alarme sobre ela são matéria de resiliência/telemetria de ANC-06/ARQ-445, com slot em §12 — o timing/orçamento de retry de `TRP-31` remete à mesma fronteira sem virar linha própria. (3) As regras marcadas **dependência declarada** — `TRP-21`, `TRP-41`, `KFK-13`, `KFK-14`, `SQS-12b` — não recebem vetor inventado: `TRP-41` limita a política de Kafka a norma de desenho até revisão de infraestrutura (§14), e o claim-check (`TRP-21`/`SQS-12`/`SQS-12b`) e o registry (`KFK-13`/`KFK-14`) dependem de evolução do perfil (ANC-03/FND-05) ou de `ADR-DMPF-N`; a consequência é remissão a §12.3/§14, não vetor. (4) A matriz de hops de FND-06 §5.2 tem exatamente quatro linhas `não conforme` (`CEN-27`..`CEN-30`); o hop de duplo Base64 é conforme sob `TRP-19` e só vira negativo se codificar duas vezes — por isso `TRP-19` é `runtime-testable` pelo oráculo 3, e **não** uma quinta linha não conforme.

`registro` — **IDs cujo modo/instrumento a fonte não determina sozinha, resolvidos por esta calibração.** Nenhuma regra de FND-05 ou FND-06 declara modo de verificação no próprio bloco; a única menção literal a modo nas fontes é FND-05 §1.5 fixando a vedação de exactly-once como `structurally reviewable`. O modo de cada linha acima foi **calibrado por FND-09** (D10, `RAS-10`/`RAS-11`, `RAS-30`/`RAS-31`), reusando a calibração já fixada pelo próprio artefato em §4.4/§4.5/§4.6 (coluna «Herança e modo»), §6 (`ORA`), §8 (`KIT`) e §11.6 (`RAS-40`). Onde o artefato não deu instrumento dedicado, a linha aponta §11.1 (`RAS-30`) como o critério de inspeção decidível, e o subitem da fonte como localizador. Nenhuma regra destes dois artefatos é `import-verifiable`: `TRP-04` é o caso mais próximo (política de capability), mas a sua verificação é inspeção da política contra as portas do bloco — a família `DMPF-E` de RFC §10.3 só se aplica à violação concreta de import, instanciada em §9 (`FIT-02`).

### §13.6 FND-07

**112 regras** de `contexto-erros-seguranca.md` (FND-07, sob ANC-05): `CTX` 28, `IDN` 20, `ERR` 28, `MAP` 7, `THR` 3, `DAT` 26. Contíguas por prefixo, sem lacuna nem repetição (confere FND-07 §11.1). Modo por regra conforme FND-07 §11.2; instanciação calibrada por D10 (`RAS-30`).

Convenção da coluna **Onde**: `§N` é seção deste artefato (FND-09); `FND-07 §N` é a fonte. Regra `runtime-testable` cita o cenário (§4, `CEN`) e o oráculo (§11, `RAS`); regra `structurally reviewable` herdada cita a seção-fonte onde o critério decidível é inspecionado (FND-09 não acrescenta cenário); regra `import-verifiable` cita §9 (`FIT`) e o diagnóstico reusado em `RFC §10.3`.

**`CTX` — contexto de execução (28)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `CTX-01` | structurally reviewable | critério: o contexto tem os nove campos de Parte-1 §11 (`request_id`, `correlation_id`, `causation_id`, `trace_context`, `authenticated_subject`, `tenant_id`, `permissions`, `deadline`, `locale`) | — | — | FND-07 §3.1 |
| `CTX-02` | import-verifiable + structurally reviewable | diagnóstico reusado de `RFC §10.3` (grafo: tipo no `port`, instância no `app`, I/O no `provider`) + inspeção da montagem | contexto montado só no `app` | outro bloco monta contexto → aresta proibida (`DMPF-D001`) | §9 (`FIT`); `RFC §10.3`; FND-07 §3.1 |
| `CTX-03` | import-verifiable + structurally reviewable | diagnóstico reusado de `RFC §10.3` + inspeção: contexto passado explícito ao `application service`; `domain` recebe valores extraídos | `application service` recebe o contexto como argumento; UPR não o recebe (FND-03 `FRT-03`) | `domain`/UPR recebe o contexto → aresta proibida | §9 (`FIT`); `RFC §10.3`; FND-07 §3.1 |
| `CTX-04` | structurally reviewable | critério: contexto imutável após montado; derivar sub-contexto é montar outro | — | — | FND-07 §3.1 |
| `CTX-05` | import-verifiable + structurally reviewable | diagnóstico reusado de `RFC §10.3` + inspeção: mecanismo ambiental (`AsyncLocalStorage`, `context.Context`, thread-local) não é fonte de valor de correção | correção não depende de leitura ambiental (ambiente só enriquece log/trace) | valor de correção lido do ambiente → aresta/diagnóstico | §9 (`FIT`); `RFC §10.3`; FND-07 §3.1 |
| `CTX-06` | structurally reviewable | critério: `authenticated_subject` e `tenant_id` nunca têm fonte em campo da entrada; divergência recusa a requisição | — | — | FND-07 §3.2 |
| `CTX-07` | structurally reviewable | critério: `correlation_id` de fronteira confiável e válido é preservado; ausente/malformado/não confiável é gerado nesta borda | — | — | FND-07 §3.2 |
| `CTX-08` | structurally reviewable | critério: `causation_id` = passo imediatamente anterior, nunca igual ao `request_id` corrente | — | — | FND-07 §3.2 |
| `CTX-09` | structurally reviewable | critério: `trace_context` propagado na forma W3C (`traceparent`/`tracestate` por `ENV-08`) | — | — | FND-07 §3.2 |
| `CTX-10` | structurally reviewable | critério: `locale` não participa de autorização, roteamento nem regra de negócio; só formatação/mensagem | — | — | FND-07 §3.2 |
| `CTX-11` | structurally reviewable | critério: toda travessia resolve, para cada um dos nove campos, exatamente uma das quatro ações da matriz de §3.3 | — | — | FND-07 §3.3 |
| `CTX-12` | structurally reviewable | critério: `authenticated_subject`/`permissions` não atravessam fan-out/downstream; a identidade de chamada é a do próprio serviço | — | — | FND-07 §3.3 |
| `CTX-13` | structurally reviewable | critério: `tenant_id` atravessa preservado; ausência = cadeia sem sujeito (`ENV-12`), sem preenchimento | — | — | FND-07 §3.3 |
| `CTX-14` | structurally reviewable | critério: o contexto termina com a execução; nenhum campo é fonte de valor após o término (corroborado no vetor positivo de `CEN-42`) | — | — | FND-07 §3.4 |
| `CTX-15` | runtime-testable | oráculo do tempo de vida e vedação de reuso (`RAS-35`) via `CEN-42` | cada execução monta o seu contexto; nada de contexto terminado alimenta trabalho novo | contexto retido (pool/cache/closure) reusado na requisição seguinte, de outro tenant | §4 `CEN-42`; §11 `RAS-35` |
| `CTX-16` | runtime-testable | oráculo (`RAS-35`) via `CEN-42`: cache de decisão derivada com chave (sujeito+tenant) e validade ≤ execução | decisão de autz em cache expira ao fim da execução | chave sem tenant carrega decisão de um tenant para outro | §4 `CEN-42`; §11 `RAS-35` |
| `CTX-17` | runtime-testable | oráculo (`RAS-35`) via `CEN-42`: trabalho pós-resposta monta o próprio contexto, encadeado por `correlation_id` | publicação/drenagem/tarefa agendada monta contexto próprio | trabalho continuado herda o contexto da requisição | §4 `CEN-42`; §11 `RAS-35` |
| `CTX-18` | structurally reviewable | critério: `deadline` é instante absoluto, não duração; conversão ocorre na montagem | — | — | FND-07 §3.5 |
| `CTX-19` | structurally reviewable | critério: propagação monotônica — `deadline` derivado ≤ o de origem; nenhum bloco o estende | — | — | FND-07 §3.5 |
| `CTX-20` | structurally reviewable | critério: contexto carrega sinal de cancelamento observável, recebido pelo `application service` (fecha FND-03 `FRT-03`) | — | — | FND-07 §3.5 |
| `CTX-21` | runtime-testable | oráculo do respeito a cancelamento/deadline (`RAS-36`) via `CEN-43`, observável no dependente | I/O com contexto vigente; ao cancelar, interrompe o pendente | chamada iniciada com contexto cancelado/expirado aparece nos registros do dependente após o `deadline` | §4 `CEN-43`; §11 `RAS-36` |
| `CTX-22` | runtime-testable | oráculo (`RAS-36`) via `CEN-43`: cancelamento interrompe o pendente e não desfaz efeito commitado | efeito de negócio commitado permanece; reversão é ação de negócio própria | cancelamento desfaz efeito já commitado como consequência implícita | §4 `CEN-43`; §11 `RAS-36` |
| `CTX-23` | structurally reviewable | critério: estouro de `deadline` e cancelamento têm categorias próprias em §5, distintas de indisponibilidade do dependente | — | — | FND-07 §3.5 |
| `CTX-24` | structurally reviewable | critério: no consumo, contexto reconstruído do envelope com a autoridade de `ENV-14` | — | — | FND-07 §3.6 |
| `CTX-25` | structurally reviewable | critério: `authenticated_subject` não é reconstruído do envelope como autorizador; sujeito de origem é proveniência | — | — | FND-07 §3.6 |
| `CTX-26` | structurally reviewable | critério: `tenant_id` ausente no envelope = plataforma sem sujeito (`ENV-12`); sem default/`system`/sintético | — | — | FND-07 §3.6 |
| `CTX-27` | structurally reviewable | critério: entrada autenticada no consumo = integridade do envelope + confiança da fronteira de transporte (§4) | — | — | FND-07 §3.6 |
| `CTX-28` | structurally reviewable | critério: contexto reconstruído tem `request_id` e `deadline` próprios do consumidor, não lidos do envelope | — | — | FND-07 §3.6 |

**`IDN` — identidade, autorização e multi-tenancy (20)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `IDN-01` | structurally reviewable | critério: requisição autenticada = credencial apresentada + verificada + sujeito resolvido | — | — | FND-07 §4.1 |
| `IDN-02` | structurally reviewable | critério: herança de confiança de canal (rede interna, gateway, mesh) não autentica sujeito | — | — | FND-07 §4.1 |
| `IDN-03` | structurally reviewable | critério: fronteira confiável = workload verificado sob domínio administrativo declarado; autoriza preservar correlação/trace, não aceitar sujeito/tenant | — | — | FND-07 §4.1 |
| `IDN-04` | structurally reviewable | critério: no consumo assíncrono, entrada autenticada exige integridade do envelope + confiança da fronteira (o que `CTX-27` invoca) | — | — | FND-07 §4.1 |
| `IDN-05` | structurally reviewable | critério: autenticação (quem) e autorização (se pode) em blocos/etapas distintos; sujeito autenticado ≠ autorizado | — | — | FND-07 §4.2 |
| `IDN-06` | structurally reviewable | critério: falha de autenticação e de autorização produzem categorias distintas em §5; uma não substitui a outra | — | — | FND-07 §4.2 |
| `IDN-07` | structurally reviewable | critério: autorização de aplicação é o passo 1 do fluxo de escrita (FND-03 §4.2), antes da Unit of Work | — | — | FND-07 §4.3 |
| `IDN-08` | structurally reviewable | critério: permissão de operação e pertencimento a tenant são verificações independentes, ambas obrigatórias quando aplicáveis | — | — | FND-07 §4.3 |
| `IDN-09` | structurally reviewable | critério: invariante de negócio sobre o estado do agregado não é controle de acesso | — | — | FND-07 §4.3 |
| `IDN-10` | import-verifiable + structurally reviewable | diagnóstico reusado de `RFC §10.3` + inspeção: `permissions` já resolvidas, o `application service` decide sobre o valor recebido | `application service` decide sobre `permissions` do contexto | serviço consulta a autoridade de identidade para decidir → aresta proibida | §9 (`FIT`); `RFC §10.3`; FND-07 §4.3 |
| `IDN-11` | runtime-testable | oráculo do isolamento fail-closed (`RAS-33`) via `CEN-41` | leitura/escrita escopada ao tenant do contexto devolve o próprio dado | caminho que produz dado observável sem escopo de tenant | §4 `CEN-41`; §11 `RAS-33` |
| `IDN-12` | runtime-testable | oráculo (`RAS-33`) via `CEN-41`: negação ativa registrada | acesso legítimo devolve o dado do tenant A | tentativa cross-tenant não devolve o dado e é registrada como evento de segurança (sujeito, tenant do contexto, tenant do dado) | §4 `CEN-41`; §11 `RAS-33` |
| `IDN-13` | runtime-testable | oráculo que distingue «negado» de «vazio» (`RAS-34`): assevera sobre o **registro interno**, não sobre a resposta | resposta ao chamador indistinguível de inexistência, mas registro interno distingue os dois casos | resposta que distingue «não encontrado» de «proibido» (enumeração), ou vazio sem negação ativa | §4 `CEN-41`; §11 `RAS-34` |
| `IDN-14` | runtime-testable | oráculo (`RAS-33`/`RAS-34`) via `CEN-41`: vetor de caminho — a imposição não depende de convenção de código | caminho que omite o escopo não devolve nem altera dado de outro tenant | isolamento que depende de cada autor lembrar de acrescentar a condição | §4 `CEN-41`; §11 `RAS-33` |
| `IDN-15` | structurally reviewable | critério: operação que exige identidade/tenant e não os encontra resolvidos é negada; ausência não é permissão | — | — | FND-07 §4.5 |
| `IDN-16` | structurally reviewable | critério: cada operação declara o que exige (sujeito, tenant, ambos, nenhum); a declaração é do `app` | — | — | FND-07 §4.5 |
| `IDN-17` | structurally reviewable | critério: operação sem declaração exige sujeito e tenant; a omissão nega (default fechado) | — | — | FND-07 §4.5 |
| `IDN-18` | structurally reviewable | critério: cadeia de plataforma sem sujeito é caso legítimo (`ENV-12`), não negada por ausência | — | — | FND-07 §4.5 |
| `IDN-19` | structurally reviewable | critério: operação de plataforma declara o escopo de dado; alcance irrestrito é declarado, nunca efeito colateral | — | — | FND-07 §4.5 |
| `IDN-20` | structurally reviewable | critério: nenhum bloco inventa sujeito/tenant; `system`/`default`/`anonymous`/`unknown`/tenant sintético proibidos | — | — | FND-07 §4.5 |

**`ERR` — taxonomia de erros (28)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `ERR-01` | structurally reviewable | critério: todo erro que cruza fronteira de bloco tem exatamente uma categoria de §5.3 | — | — | FND-07 §5.1 |
| `ERR-02` | structurally reviewable + parte runtime-testable | inspeção: classificação total (catch-all `Unexpected`); parte runtime — oráculo das bordas de erro (`RAS-37`) | erro conhecido cruzando a borda mapeia para exatamente uma categoria | panic/exceção não classificada escapa como sucesso ou sem categoria | FND-07 §5.1; §11 `RAS-37` |
| `ERR-03` | structurally reviewable | critério: categoria atribuída onde a falha é conhecida, não reatribuída a jusante para obter outra disposição | — | — | FND-07 §5.1 |
| `ERR-04` | structurally reviewable | critério: erro público carrega as seis propriedades (código, categoria, mensagem segura, detalhes, retryability, causa técnica) | — | — | FND-07 §5.2 |
| `ERR-05` | structurally reviewable | critério: retryability explícita no erro, não inferida do código de protocolo | — | — | FND-07 §5.2 |
| `ERR-06` | structurally reviewable | critério: mensagem segura não reproduz dado de negócio, credencial, segredo, id interno nem stack | — | — | FND-07 §5.2 |
| `ERR-07` | structurally reviewable | critério: causa técnica permanece interna, projetada por FND-07 §5.6, nunca na resposta externa | — | — | FND-07 §5.2 |
| `ERR-08` | structurally reviewable | critério: catálogo de categorias é o conjunto do DMPF; bounded context não cria categoria própria | — | — | FND-07 §5.3 |
| `ERR-09` | structurally reviewable | critério: todo erro concreto resolve para booleano de retryability; `condicional` é da categoria, não do erro concreto | — | — | FND-07 §5.4 |
| `ERR-10` | structurally reviewable | critério: `condicional` só resolve retentável quando o predicado declarado é satisfeito e verificável na classificação | — | — | FND-07 §5.4 |
| `ERR-11` | structurally reviewable + parte runtime-testable | inspeção: condicional sem predicado decidível → não retentável (default fechado); parte runtime — oráculo das bordas (`RAS-37`) | retryability desconhecida assume default fail-closed (não repetível) | erro de retryability não declarada marcado repetível por omissão | FND-07 §5.4; §11 `RAS-37` |
| `ERR-12` | structurally reviewable | critério: retryability declara se repetir pode ter desfecho diferente; não é política de repetição | — | — | FND-07 §5.4 |
| `ERR-13` | structurally reviewable | critério: código de erro é contrato público; mudar o significado de código existente é breaking change | — | — | FND-07 §5.5 |
| `ERR-14` | structurally reviewable | critério: código qualificado pelo bounded context na forma `<contexto>/<identificador>` | — | — | FND-07 §5.5 |
| `ERR-15` | structurally reviewable | critério: mensagem não é identificador; alterar/traduzir texto não muda o código | — | — | FND-07 §5.5 |
| `ERR-16` | structurally reviewable | critério: código que deixa de ser emitido entra em depreciação declarada, com substituto nomeado | — | — | FND-07 §5.5 |
| `ERR-17` | structurally reviewable | critério: a categoria de um código não muda ao longo da vida; natureza diferente é código novo | — | — | FND-07 §5.5 |
| `ERR-18` | structurally reviewable | critério: diagnóstico tem esquema fixo (código, categoria, retryability resolvida, instante, id da tentativa, causa técnica sanitizada) | — | — | FND-07 §5.6 |
| `ERR-19` | structurally reviewable | critério: diagnóstico tem limite de tamanho declarado; o truncamento preserva os cinco elementos antes da causa textual | — | — | FND-07 §5.6 |
| `ERR-20` | structurally reviewable | critério: duas projeções — pública (código, categoria, retryability, mensagem segura) e interna (`last_error`) | — | — | FND-07 §5.6 |
| `ERR-21` | structurally reviewable | critério: projeção interna lida por operação; sanitização integral (`OBX-02`/`OBX-03` não relaxados) | — | — | FND-07 §5.6 |
| `ERR-22` | structurally reviewable | critério: panic (Go) e exceção não classificada (TS) são `Unexpected`, nunca `DomainRejection`/`Validation`/`Conflict` | — | — | FND-07 §5.7 |
| `ERR-23` | structurally reviewable | critério: a borda de cada stack converte a falha não classificada em erro da taxonomia antes de responder/dispor | — | — | FND-07 §5.7 |
| `ERR-24` | structurally reviewable | critério: a conversão de `ERR-23` não infere retryability; `Unexpected` sem predicado → não retentável (`ERR-11`) | — | — | FND-07 §5.7 |
| `ERR-25` | structurally reviewable | critério: erro de outro bounded context é traduzido, não repassado; código do próprio catálogo (`ERR-14`) | — | — | FND-07 §5.8 |
| `ERR-26` | structurally reviewable | critério: tradução preserva a retryability conhecida; resolve para não retentável quando desconhecida | — | — | FND-07 §5.8 |
| `ERR-27` | structurally reviewable | critério: tradução não expõe código do produtor, id interno, stack nem topologia na projeção pública | — | — | FND-07 §5.8 |
| `ERR-28` | structurally reviewable | critério: `DomainRejection` do produtor traduz para `DomainRejection`/`Validation` só quando a regra violada é do próprio consumidor | — | — | FND-07 §5.8 |

**`MAP` — mapeamento por transporte (7)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `MAP-01` | structurally reviewable | critério: mapeamento total — toda categoria de §5.3 tem linha em §6.2, valor literal por célula | — | — | FND-07 §6 |
| `MAP-02` | structurally reviewable | critério: mapeamento ocorre no `app`, última conversão antes da resposta/disposição; não refeito a jusante | — | — | FND-07 §6 |
| `MAP-03` | structurally reviewable | critério: mapeamento não altera categoria nem retryability; só as representa no vocabulário do transporte | — | — | FND-07 §6 |
| `MAP-04` | structurally reviewable | critério: coluna de mensageria descreve disposição sob recepção `R1` (FND-04), não contenção de envelope | — | — | FND-07 §6.1 |
| `MAP-05` | structurally reviewable | critério: recepções `R2`/`R3`/`R4` curto-circuitam a disposição e não produzem erro da taxonomia | — | — | FND-07 §6.1 |
| `MAP-06` | structurally reviewable | critério: `499` de `Cancelled` não é código IANA; convenção HTTP, com fallback onde não suportado | — | — | FND-07 §6.2 |
| `MAP-07` | structurally reviewable | critério: `Conflict` e `DeadlineExceeded` têm célula de mensageria dependente de predicado, resolvida na classificação | — | — | FND-07 §6.2 |

**`THR` — threat model STRIDE (3)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `THR-01` | inspeção | fronteira FND-08 — eixo *Denial of service* (limiar, orçamento, degradação, contenção de carga) não normatizado aqui | — | — | §12.1 (fronteira a FND-08) |
| `THR-02` | inspeção | fronteira FND-08 (eixo *D*); em escopo, reafirma `ERR-11` e `ERR-24`, que impedem trabalho inútil de se repetir | — | — | §12.1; FND-07 §7.9 |
| `THR-03` | inspeção | gate de revisão de Segurança sobre as seis categorias × sete vetores — **sem owner nomeado: pendência, não cobertura** | — | — | §14 (pendência); FND-07 §7.10 |

**`DAT` — governança do dado de negócio (26)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `DAT-01` | structurally reviewable | critério: todo campo de dado de negócio tem classificação declarada pelo contexto que o produz (superfícies: `payload` da outbox, `data` do envelope, etc.) | — | — | FND-07 §8.1 |
| `DAT-02` | structurally reviewable | critério: dado é sensível se satisfaz ≥1 dos quatro testes (identifica pessoa; credencial/segredo/chave; divulgação danosa; …) | — | — | FND-07 §8.1 |
| `DAT-03` | structurally reviewable | critério: campo sem classificação é tratado como sensível (fecha; alcança cifra FND-07 §8.3, retenção §8.5 e redaction §8.7) | — | — | FND-07 §8.1 |
| `DAT-04` | structurally reviewable | critério: taxonomia corporativa prevalece no que cobrir; `DAT-02` cobre o restante; as duas compõem | — | — | FND-07 §8.1 |
| `DAT-05` | structurally reviewable | critério: `data` carrega só o necessário ao consumidor declarado do contrato | — | — | FND-07 §8.2 |
| `DAT-06` | structurally reviewable | critério: dado sensível não entra em `metadata`, `last_error`, log, trace nem chave de partição (complementa `OBX-02`/`OBX-03`) | — | — | FND-07 §8.2 |
| `DAT-07` | structurally reviewable | critério: minimização avaliada por contrato publicado; incluir campo sensível é decisão registrada | — | — | FND-07 §8.2 |
| `DAT-08` | runtime-testable (por inspeção de superfície) | `RAS-38`: gravar, por cada caminho de escrita de `DAT-01`, um valor reconhecível de classe protegida e então abrir a superfície | valor gravado não é legível na superfície: cifrado em repouso | um caminho grava o valor e ele é legível — cifra ausente naquele caminho | §11 `RAS-38`; FND-07 §8.3 |
| `DAT-09` | structurally reviewable | critério: cifra não substitui minimização (§8.2) nem controle de acesso (§8.4) | — | — | FND-07 §8.3 |
| `DAT-10` | runtime-testable (por inspeção de superfície) | `RAS-38`: o mesmo valor levado à contenção — DLQ e quarantine, com envelope preservado (FND-04 `GAR-07`) | valor não legível na DLQ nem na quarantine: contenção no mesmo regime | valor legível na DLQ ou na quarantine | §11 `RAS-38`; FND-07 §8.3 |
| `DAT-11` | structurally reviewable | critério: acesso ao dado armazenado sem passar pela aplicação é nomeado, autorizado e registrado | — | — | FND-07 §8.4 |
| `DAT-12` | structurally reviewable | critério: DLQ, quarantine e outbox são superfícies de dado de negócio; o acesso satisfaz `DAT-11` | — | — | FND-07 §8.4 |
| `DAT-13` | structurally reviewable | critério: controle de §8.4 não é o de §4; `IDN-11`..`IDN-14` escopam a aplicação, `DAT-11` governa acesso fora dela | — | — | FND-07 §8.4 |
| `DAT-14` | runtime-testable (por inspeção de superfície) | `RAS-39`: gravar o valor com marca temporal conhecida e aguardar o prazo declarado, ou avançar o relógio do ambiente | valor deixou de existir antes do teto: retenção efetiva ≤ teto externo (`DAT-15`) | valor persiste depois do teto, ou teto ausente sem recair no mais estrito (`DAT-17`) | §11 `RAS-39`; FND-07 §8.5 |
| `DAT-15` | structurally reviewable | critério: o teto tem origem declarada (legal, contratual ou corporativa), acompanhando a classificação de `DAT-01` | — | — | FND-07 §8.5 |
| `DAT-16` | structurally reviewable | critério: teto mais estrito que o prazo operacional prevalece (fecha FND-04 §4.3) | — | — | FND-07 §8.5 |
| `DAT-17` | structurally reviewable | critério: sem classificação não há teto determinável; por `DAT-03` aplica-se o teto mais estrito declarado | — | — | FND-07 §8.5 |
| `DAT-18` | structurally reviewable | critério: a purga que realiza o teto é operação com evidência (o que foi purgado e até qual instante ficam registrados) | — | — | FND-07 §8.5 |
| `DAT-19` | structurally reviewable | critério: `metadata` carrega metadado técnico de correlação/proveniência (três testes: técnico, não-negócio, necessário) | — | — | FND-07 §8.6 |
| `DAT-20` | structurally reviewable | critério: `metadata` não é campo de extensão de contrato; informação de negócio vai no `data` | — | — | FND-07 §8.6 |
| `DAT-21` | structurally reviewable | critério: `correlationid`/`causationid`/`traceparent` satisfazem `DAT-19` e são conteúdo admissível em `metadata` | — | — | FND-07 §8.6 |
| `DAT-22` | structurally reviewable | critério: dado sensível é redigido na origem; filtrar no agregador não satisfaz | — | — | FND-07 §8.7 |
| `DAT-23` | structurally reviewable | critério: redaction preserva utilidade diagnóstica — campo identificado com valor substituído, não omitido | — | — | FND-07 §8.7 |
| `DAT-24` | structurally reviewable | critério: a projeção interna do diagnóstico (`ERR-20`) não é exceção à redaction (`last_error`, DLQ lidas por operação) | — | — | FND-07 §8.7 |
| `DAT-25` | structurally reviewable | critério: onde há requisito regulatório, a auditoria é separada da observabilidade (retenção, acesso e integridade próprios) | — | — | FND-07 §8.7 |
| `DAT-26` | structurally reviewable | critério: cada cláusula do baseline (FND-07 §8.8) tem estado declarado (`Vigente`/`Encaminhada`) e continua obrigando na força da fonte/dona | — | — | FND-07 §8.8 |

`registro`

**Contagem por grupo.** `CTX` 28 (`CTX-01`..`CTX-28`) · `IDN` 20 (`IDN-01`..`IDN-20`) · `ERR` 28 (`ERR-01`..`ERR-28`) · `MAP` 7 (`MAP-01`..`MAP-07`) · `THR` 3 (`THR-01`..`THR-03`) · `DAT` 26 (`DAT-01`..`DAT-26`). **Total: 112**, contíguo por prefixo, sem lacuna nem repetição (confere FND-07 §11.1).

**Distribuição por modo.** `runtime-testable` (9): `CTX-15`, `CTX-16`, `CTX-17` (`CEN-42`/`RAS-35`); `CTX-21`, `CTX-22` (`CEN-43`/`RAS-36`); `IDN-11`, `IDN-12`, `IDN-13`, `IDN-14` (`CEN-41`/`RAS-33`/`RAS-34`). `structurally reviewable + parte runtime-testable` (2): `ERR-02`, `ERR-11` (parte runtime via `RAS-37`). `runtime-testable por inspeção de superfície`, calibrado a `structurally reviewable` (3): `DAT-08`, `DAT-10` (`RAS-38`), `DAT-14` (`RAS-39`). `import-verifiable + structurally reviewable` (4): `CTX-02`, `CTX-03`, `CTX-05`, `IDN-10`. `inspeção`/fronteira (3): `THR-01`, `THR-02`, `THR-03`. `structurally reviewable` puro: as demais 91.

**O que não recebe cobertura, e por quê.**
- `THR-03` — gate de revisão de Segurança **sem owner nomeado**; a fonte o registra como pendência (FND-07 §11.4, pendência 4). Fica como **pendência (§14), não como cobertura**.
- Eixo *Denial of service* (`THR-01`, `THR-02`) — **fronteira FND-08** sob ANC-06 (limiar, orçamento, degradação, contenção de carga não são desta matéria); remetido a §12.1, cuja matéria não é decidida aqui.
- `DAT-08`/`DAT-10`/`DAT-14` — modo **preservado** como a fonte o declarou, `runtime-testable` «por inspeção de superfície», com par de vetores sobre dado discriminatório gravado pelo teste (`RAS-38`/`RAS-39`, §11.5). Não recebem `CEN` porque o instrumento é a inspeção da superfície, não um cenário distribuído (critério decidível, sem par de vetores rodado). `RAS-38`/`RAS-39` fazem essa calibração explícita. `DAT-14` é uma **desigualdade** verificável (teto externo prevalece quando mais estrito).

**IDs cujo instrumento a fonte não determinou por completo.**
- `ERR-02`, `ERR-11` (parte runtime) — a fonte **não anexa um `CEN` dedicado** à parte runtime; o oráculo é o das **bordas de erro** (`RAS-37`), que existe por si e é roteado pela matriz de §3 (linha `H7-3`). A coluna Onde aponta `§11 RAS-37`, sem `CEN`.
- ~30 regras `structurally reviewable` **não nomeadas individualmente** na tabela fina de FND-07 §11.2 (`CTX-01`, `CTX-04`, `CTX-08`..`CTX-10`, `CTX-14`, `CTX-18`..`CTX-20`, `CTX-23`..`CTX-28`; `IDN-09`, `IDN-15`..`IDN-20`; `DAT-09`, `DAT-11`..`DAT-13`, `DAT-15`..`DAT-18`). A §11.2 classifica explicitamente só as exceções `runtime`/`import-verifiable`; estas ficam classificadas como `structurally reviewable` (a postura de inspeção default do artefato, da qual só as exceções foram destacadas). Onde a §11.2 não as nomeia, a classificação é determinação de calibração desta seção, não declaração literal por regra.

### §13.7 FND-08

**121 regras** de `resiliencia-observabilidade.md` (FND-08, sob ANC-06): `RES` 40, `TRC` 16, `MET` 31 (`MET-01`..`MET-30` + `MET-05a`), `LOG` 14, `RUN` 20 (`RUN-01`..`RUN-18` + `RUN-11a` + `RUN-18a`). Contíguas por prefixo, com os três sufixos de letra que FND-08 §13.1 declara (`registro` — 121 regras `normativo`, nenhuma com sujeito `domain` ou `port`).

`registro` — **FND-08 não declara modo de verificação de RFC §2.2.** A `Classificação de força` de FND-08 §1.2 rotula por força — `normativo`, `recepcionado`, `encaminhado`, `rationale`, `registro` — e nenhuma regra traz `import-verifiable`, `structurally reviewable` nem `runtime-testable`. O modo de cada linha abaixo é, portanto, **declaração de FND-09** no vocabulário de RFC §2.2, no mesmo regime em que `RAS-32` (§11.2) declarou o modo das 42 regras de FND-03 publicadas sem modo — a coluna **Modo** registra `não declarado` para toda a família, e a coluna **Instrumento** abre com o modo atribuído em backticks. A atribuição não é leitura literal da fonte.

`registro` — **Os seis critérios que FND-08 §13.5 encaminha a esta âncora como «verificáveis por instrumento»** têm o instrumento nomeado pela própria fonte e estão assinalados `[§13.5]`: `RES-01` (inexistência de sujeito `domain`/`port`), `MET-03` (nome, unidade e fórmula em toda métrica obrigatória), `MET-07` (ausência de label de alta cardinalidade), `TRC-04` (atributos comuns nos três fluxos), `TRC-07` (continuidade do trace no salto assíncrono sob fronteira confiável) e `LOG-02` (campos obrigatórios em todo registro estruturado).

`registro` — **Fronteira ativa e o que ela difere.** §12.1 e §4.8 reservam a FND-08, sob ANC-06, a política operacional (limiares, `timing`) e a observabilidade de entrega. A publicação de FND-08 satisfaz a condição de fechamento daquelas fronteiras no que toca à **regra de resultado**: os limiares, o catálogo de métricas e os procedimentos existem e são inspecionáveis. Onde a prova de uma regra é a **execução** dessa política — a montagem de trace ponta a ponta, o disparo de um sinal de observabilidade de entrega —, o cenário e a execução permanecem **dependência declarada** dos épicos de kernel (§12.3, item 4; ANC-10): a linha declara o modo e o oráculo, e o `CEN` não é fabricado. Valor de limiar ou default que só o contexto local declara (`MET-05`; defaults de `RES-08`, `RES-10`, `RES-14`, `RES-30`, `RES-32`, `RES-33`, `RUN-11`) é dependência declarada, não lacuna.

Convenção da coluna **Onde**: `§N` é seção deste artefato (FND-09); `FND-08 §N` é a fonte. Regra atribuída `runtime-testable` cita o oráculo (§10/§11, `RAS`) e o cenário (§4, `CEN`) ou o `KIT` (§8) quando existe, e declara a dependência (§4.8/§12.3) quando o cenário ainda não foi instanciado; regra `structurally reviewable` cita a seção-fonte onde o critério decidível é inspecionado, com «—» nas colunas de vetor; regra `import-verifiable` cita §9 (`FIT`) e o diagnóstico reusado em `RFC §10.3`.

**`RES` — resiliência: sujeito e fronteira, limites por dependência, retry, orçamento, backoff e degradação (40)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `RES-01` `[§13.5]` | não declarado | `import-verifiable` + `structurally reviewable`: varredura da baseline (nenhuma regra tem `domain`/`port` como sujeito) + `FIT` de §9 (`domain`/`port` sem telemetria), diagnóstico reusado `DMPF-D001`/`DMPF-D002` | baseline com sujeito `app`/`provider`/`application service` por subseção; `domain`/`port` sem observabilidade | regra normatiza telemetria obrigando o tipo de domínio a emiti-la → aresta proibida | §9 (`FIT`); `RFC §10.3`; FND-08 §1.3, §13.7 |
| `RES-02` | não declarado | `structurally reviewable` — critério: excedente entre o assunto e o escopo da ANC-06 sai `encaminhado` com dona nomeada; §1.4 é a lista exaustiva | — | — | FND-08 §1.4, §13.7 |
| `RES-03` | não declarado | `structurally reviewable` — critério: nenhuma regra redefine existência, semântica ou disposição de mecanismo de outro artefato; por M1, o mecanismo prevalece | — | — | FND-08 §2.1, §13.7 |
| `RES-04` | não declarado | `structurally reviewable` — critério: toda obrigação herdada tem estado (`quitada`/`encaminhada`/`condicionada`) e fonte `arquivo:linha` que resolve | — | — | FND-08 §2.2, §2.3 |
| `RES-05` | não declarado | `structurally reviewable` — critério: nenhuma chamada a dependência externa ocorre sem timeout e sem política de falha declarada | — | — | FND-08 §3.1 |
| `RES-06` | não declarado | `structurally reviewable` — critério: prazo da etapa = `min`(prazo do método, prazo remanescente, orçamento de `RES-30`), nunca valor fixo isolado | — | — | FND-08 §3.2 |
| `RES-07` | não declarado | `structurally reviewable` — critério: soma dos orçamentos a jusante ≤ prazo remanescente, com folga declarada; verificação de composição, detectável antes da execução | — | — | FND-08 §3.2 |
| `RES-08` | não declarado | `structurally reviewable` — critério: teto de espera da porta de inbox declarado (default 2 s, de `INB-17`), overridável por consumidor com motivo na ficha; valor = dependência declarada | — | — | FND-08 §3.2 |
| `RES-09` | não declarado | `structurally reviewable` — critério: o artefato não fixa valor de prazo por método nem por transporte (valor de FND-06 `GRP-16`..`GRP-18`); fixa obrigatoriedade, derivação, composição e orçamento | — | — | FND-08 §3.2, §13.7 |
| `RES-10` | não declarado | `structurally reviewable` — critério: dependência tolerável por degradação tem breaker com estado observável (`MET-28`); ficha declara janela, limiar, amostras, cooldown e sondas (defaults declarados) | — | — | FND-08 §3.3 |
| `RES-11` | não declarado | `structurally reviewable` — critério: breaker construído no `provider`, nunca no `application service` (inspeção da composição) | — | — | FND-08 §3.3 |
| `RES-12` | não declarado | `structurally reviewable` — critério: breaker aberto responde imediatamente, com erro de categoria própria distinguível de falha da dependência, sem aguardar o timeout | — | — | FND-08 §3.3 |
| `RES-13` | não declarado | `structurally reviewable` — critério: cada dependência externa tem pool de recursos próprio, com limite declarado na ficha | — | — | FND-08 §3.4 |
| `RES-14` | não declarado | `structurally reviewable` — critério: fila de espera do pool limitada, saturação = rejeição rápida (defaults: fila = pool, aquisição 100 ms) | — | — | FND-08 §3.4 |
| `RES-15` | não declarado | `structurally reviewable` — critério: rate limit de saída declarado por dependência, ou o motivo de não haver declarado | — | — | FND-08 §3.5 |
| `RES-16` | não declarado | `structurally reviewable` — critério: borda exposta a chamador não controlado tem limite de taxa e de concorrência, declarados por rota e por tenant | — | — | FND-08 §3.5 |
| `RES-17` | não declarado | `structurally reviewable` — critério: recusa por admissão categorizada, observável (`MET-12` por rota/tenant) e barata, antes de decode e validação | — | — | FND-08 §3.5 |
| `RES-18` | não declarado | `structurally reviewable` — critério: aqui só o limite de taxa; limites de forma (`TRP-49`..`TRP-51`) permanecem de FND-06, verificados antes do decode | — | — | FND-08 §3.5 |
| `RES-19` | não declarado | `structurally reviewable` — critério: cache-aside reduz pressão de leitura e não é fonte de verdade; decisão de negócio não se apoia em cache sem revalidação | — | — | FND-08 §3.6 |
| `RES-20` | não declarado | `structurally reviewable` — critério: leitura degradada de cache declara a janela de obsolescência e é observável como degradada (`MET-13`) | — | — | FND-08 §3.6 |
| `RES-21` | não declarado | `structurally reviewable` — critério: cada dependência tem ficha de resiliência versionada, cobrindo os seis mecanismos + orçamento/backoff/teto/degradação, com valor ou «não se aplica» justificado; campo em branco reprova | — | — | FND-08 §3, §13.6 (índice de termos) |
| `RES-22` | não declarado | `structurally reviewable` — critério: ordem de composição dos decorators declarada e estável (canônica de fora para dentro); ordem indeterminada é defeito | — | — | FND-08 §3 |
| `RES-23` | não declarado | `structurally reviewable` — critério: todo decorator instalado expõe ao menos um sinal do catálogo de §6 | — | — | FND-08 §3 |
| `RES-24` | não declarado | `import-verifiable`: nenhum decorator em `domain`/`port`; composição no composition root do `app` ou na construção do `provider`; diagnóstico reusado `DMPF-D001`/`DMPF-D002` | decorators no composition root do `app` / na construção do `provider` | porta declara retry, prazo ou política na assinatura → aresta proibida | §9 (`FIT`); `RFC §10.3`; FND-08 §3 |
| `RES-25` | não declarado | `structurally reviewable` — critério: retry é da chamada remota, na mesma execução; não reabre a UoW, não repete a decisão de negócio, não reexecuta o caso de uso | — | — | FND-08 §4.1 |
| `RES-26` | não declarado | `runtime-testable`: reexecução por redelivery com idempotência de efeito de negócio (`GAR-03`/`GAR-04`); oráculo de reentrega `RAS-13` (`V32`) no harness `KIT-06` | reentrega da mesma mensagem produz o mesmo efeito final | reentrega duplica o efeito de negócio → `DMPF-R004` | §4 `CEN-03`/`CEN-06`/`CEN-12`; §8 `KIT-06`; §11 `RAS-13`; FND-08 §4.1 |
| `RES-27` | não declarado | `structurally reviewable` — critério: tentativa de repetição autorizada se e somente se os quatro fatores são simultaneamente verdadeiros, verificados antes de cada tentativa | — | — | FND-08 §4.2 |
| `RES-28` | não declarado | `structurally reviewable` — critério: retryability é condição necessária, não suficiente; disparar retry só da classificação viola `ERR-12` | — | — | FND-08 §4.2 |
| `RES-29` | não declarado | `structurally reviewable` — critério: default fail-closed de `ERR-11` herdado; fator indeterminado é falso; «retentar quando não se sabe» é violação de M1 | — | — | FND-08 §4.2 |
| `RES-30` | não declarado | `structurally reviewable` — critério: orçamento de repetição por execução (default = metade do prazo remanescente na primeira falha), consumido por todas as dependências | — | — | FND-08 §4.3 |
| `RES-31` | não declarado | `structurally reviewable` — critério: a espera de backoff consome o orçamento de repetição | — | — | FND-08 §4.3 |
| `RES-32` | não declarado | `structurally reviewable` — critério: espaçamento exponencial com jitter obrigatório (defaults: base 100 ms, fator 2, jitter total, teto 5 s) | — | — | FND-08 §4.4 |
| `RES-33` | não declarado | `structurally reviewable` — critério: teto de tentativas obrigatório e declarado (default 3 síncrono, 5 assíncrono, contando a original); compõe com `GAR-08`, gesto por transporte de FND-06 | — | — | FND-08 §4.4 |
| `RES-34` | não declarado | `structurally reviewable` — critério: retry não atravessa fronteira de commit; repetição após o commit é operação nova ou drenagem de outbox, não retry (`RES-25`) | — | — | FND-08 §4.4 |
| `RES-35` | não declarado | `structurally reviewable` — critério: retry de conflito de escrita é política explícita nos termos de `UOW-09`/`UOW-10`; admissibilidade permanece de FND-04 | — | — | FND-08 §4.4 |
| `RES-36` | não declarado | `structurally reviewable` — critério: esgotamento do orçamento tem desfecho observável (atributo de span + `MET-28`); no assíncrono, a disposição é a que FND-04 fixa | — | — | FND-08 §4.3 |
| `RES-37` | não declarado | `structurally reviewable` — critério: cada dependência declara o modo de degradação na ficha; os modos admissíveis são quatro (`falha`/`degrada`/`difere`/`ignora`) | — | — | FND-08 §4.5, §13.6 |
| `RES-38` | não declarado | `structurally reviewable` — critério: degradar não silencia erro; modo `degrada` produz resposta distinguível + `MET-13`, modo `ignora` produz sinal de omissão | — | — | FND-08 §4.5 |
| `RES-39` | não declarado | `structurally reviewable` — critério: contenção de repetição é conjunta (retry + breaker + admissão = política única); a ficha de `RES-21` é avaliada como conjunto na revisão de mudança de configuração | — | — | FND-08 §4.5 |
| `RES-40` | não declarado | `structurally reviewable` — critério: todo override de default é declarado, versionado e observável (metadado de telemetria); default alterado em runtime sem registro reprova | — | — | FND-08 §4.5 |

**`TRC` — tracing: fluxos, spans, atributos, continuidade assíncrona, amostragem e redaction (16)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `TRC-01` | não declarado | `runtime-testable`: oráculo — os três fluxos traçados ponta a ponta, com todos os saltos ligados por parentesco ou link, sem interrupção no salto de processo; cenário e execução = dependência declarada (§4.8/§12.3, sem `CEN` de tracing nesta entrega) | trace em que todos os saltos do fluxo aparecem ligados | salto de processo interrompe o trace | FND-08 §5.1; §4.8, §12.1 |
| `TRC-02` | não declarado | `structurally reviewable` — critério: span de borda aberto no estágio `tracing/metrics/logging` do pipeline de entrada, antes da validação e da autorização | — | — | FND-08 §5.2 |
| `TRC-03` | não declarado | `structurally reviewable` — critério: nome e atributos seguem as semantic conventions do OpenTelemetry na versão fixada no BOM (Parte-1 §17.2); migração de versão é governada | — | — | FND-08 §5.3 |
| `TRC-04` `[§13.5]` | não declarado | `runtime-testable`: oráculo — todo span dos três fluxos carrega os atributos comuns; cenário e captura = dependência declarada (§4.8/§12.3, sem `CEN` de tracing nesta entrega) | spans dos três fluxos carregam o conjunto comum de atributos | span de um dos três fluxos omite atributo comum | FND-08 §5.3; §13.5; §4.8 |
| `TRC-05` | não declarado | `structurally reviewable` — critério: span do transporte distinto do span do `application service`; um mede a espera do cliente, o outro mede o caso de uso | — | — | FND-08 §5.2 |
| `TRC-06` | não declarado | `structurally reviewable` — critério: `correlation_id` é atributo de span nos três fluxos, ligando trace, log e métrica sem transportar PII | — | — | FND-08 §5.3 |
| `TRC-07` `[§13.5]` | não declarado | `runtime-testable`: oráculo — continuidade do trace no salto assíncrono usa o `traceparent` do envelope só sob fronteira confiável (`CTX-27`); fora dela, inicia trace novo e registra o valor como proveniência; cenário e execução = dependência declarada (§4.8/§12.3) | salto assíncrono sob fronteira confiável: trace filho liga ao pai pelo `traceparent` do envelope | continua o trace sob fronteira não confiável, ou perde a continuidade sob fronteira confiável | FND-08 §5.4; §13.5; §4.8 |
| `TRC-08` | não declarado | `structurally reviewable` — critério: quando a relação entre spans é de lote, o vínculo é `link`, não parentesco | — | — | FND-08 §5.4 |
| `TRC-09` | não declarado | `structurally reviewable` — critério: propagador configurado explicitamente na forma W3C Trace Context (`ENV-08`/`CTX-09`); sem detecção automática nem fallback silencioso | — | — | FND-08 §5.4 |
| `TRC-10` | não declarado | `structurally reviewable` — critério: span de consumo abre com `request_id` próprio por tentativa (`CTX-28`); duas tentativas do mesmo `message_id` são dois spans ligados ao mesmo `correlation_id` | — | — | FND-08 §5.4 |
| `TRC-11` | não declarado | `structurally reviewable` — critério: cada tentativa de operação repetida é observável individualmente (span próprio ou evento datado, com o número da tentativa e o motivo da anterior) | — | — | FND-08 §5.3 |
| `TRC-12` | não declarado | `structurally reviewable` — critério: erro no span registrado por status e categoria, sem payload (`ERR-20`, redaction de `DAT-24`) | — | — | FND-08 §5.3 |
| `TRC-13` | não declarado | `structurally reviewable` — critério: amostragem declarada por classe de tráfego (quatro classes), não por serviço | — | — | FND-08 §5.5, §13.6 |
| `TRC-14` | não declarado | `structurally reviewable` — critério: erro é sempre amostrado; onde a decisão precede o desfecho, usa decisão tardia (tail-based) ou regra equivalente | — | — | FND-08 §5.5 |
| `TRC-15` | não declarado | `structurally reviewable` — critério: atributos de span são allowlist; nenhuma implementação copia corpo, envelope ou resultado (`DAT-22` alcança o pipeline de trace na origem) | — | — | FND-08 §5.6 |
| `TRC-16` | não declarado | `import-verifiable`: nenhuma instrumentação de trace em `domain`/`port`; span de regra aberto pelo `application service`; diagnóstico reusado `DMPF-D001`/`DMPF-D002` | span de negócio aberto pelo `application service` que invoca a regra | `domain`/`port` abre span de regra de negócio → aresta proibida | §9 (`FIT`); `RFC §10.3`; FND-08 §5.6 |

**`MET` — métricas: convenção, catálogo por componente, limiar condicional e cardinalidade (31)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `MET-01` | não declarado | `structurally reviewable` — critério: catálogo obrigatório é o mínimo que torna cada failure mode observável; §9 de FND-08 verifica em sentido inverso, um failure mode por vez | — | — | FND-08 §6.1 |
| `MET-02` | não declarado | `structurally reviewable` — critério: nome segue `dmpf_<componente>_<sinal>[_<unidade>]`, minúsculas e `snake_case`, com `_total` marcando contador monotônico | — | — | FND-08 §6.1 |
| `MET-03` `[§13.5]` | não declarado | `structurally reviewable` — critério: varredura do catálogo §6.2–§6.8 — toda métrica obrigatória declara nome (`MET-02`), unidade e fórmula (definição operacional) | — | — | FND-08 §6.1; §13.5 |
| `MET-04` | não declarado | `structurally reviewable` — critério: o conjunto de labels é allowlist; nenhum valor sensível vira label, sem mitigação por agregação posterior (`DAT-22` alcança a métrica) | — | — | FND-08 §6.1 |
| `MET-05` | não declarado | `structurally reviewable` (predicado decidível) — critério: limiar fixado aqui se e somente se derivado de invariante normatizada; nos demais casos declara owner e parâmetro local (dependência declarada) | — | — | FND-08 §6.1, §13.6, §13.7 |
| `MET-05a` | não declarado | `structurally reviewable` — critério: condição de forma (tendência sustentada por N janelas) não é limiar de valor; o número de janelas é parâmetro local declarado | — | — | FND-08 §6.1 |
| `MET-06` | não declarado | `structurally reviewable` — critério: todo alarme declara o sinal, a condição (expressa sobre a fórmula) e o procedimento (subseção de §8) que dispara | — | — | FND-08 §6.1 |
| `MET-07` `[§13.5]` | não declarado | `structurally reviewable` (predicado decidível) — critério: identificadores de alta cardinalidade não são labels; `tenant_id` só onde a quantidade de inquilinos é limitada e declarada | — | — | FND-08 §6.1; §13.5; §13.7 |
| `MET-08` | não declarado | `structurally reviewable` — critério: `dmpf_service_request_duration_seconds` no catálogo com nome, unidade e fórmula (`MET-03`) — histograma de latência por serviço/operação, medido na borda | — | — | FND-08 §6.2 |
| `MET-09` | não declarado | `structurally reviewable` — critério: `dmpf_service_requests_total` no catálogo (`MET-03`) — throughput por serviço/operação e desfecho | — | — | FND-08 §6.2 |
| `MET-10` | não declarado | `structurally reviewable` — critério: `dmpf_service_errors_total` no catálogo (`MET-03`) — erro por categoria da taxonomia de FND-07, com a categoria como label | — | — | FND-08 §6.2 |
| `MET-11` | não declarado | `structurally reviewable` — critério: `dmpf_service_pool_utilization` e `dmpf_service_queue_depth` no catálogo (`MET-03`) — saturação do próprio serviço | — | — | FND-08 §6.2 |
| `MET-12` | não declarado | `structurally reviewable` — critério: `dmpf_service_admission_rejections_total` no catálogo (`MET-03`) — recusa por admissão, por rota e tenant, sob `MET-07`; sinal de `RES-17` | — | — | FND-08 §6.2 |
| `MET-13` | não declarado | `structurally reviewable` — critério: `dmpf_service_degraded_total` e `dmpf_service_omitted_total` no catálogo (`MET-03`), por dependência; sinal que torna `RES-38` verificável | — | — | FND-08 §6.2 |
| `MET-14` | não declarado | `structurally reviewable` — critério: `dmpf_outbox_pending` no catálogo (`MET-03`) — gauge de linhas elegíveis ou em espera | — | — | FND-08 §6.3 |
| `MET-15` | não declarado | `structurally reviewable` — critério: `dmpf_outbox_lag_seconds` no catálogo (`MET-03`) — gauge, indicador primário de relay parado; limiar acima do lease é derivado de invariante | — | — | FND-08 §6.3 |
| `MET-16` | não declarado | `structurally reviewable` — critério: `dmpf_outbox_attempts` no catálogo (`MET-03`) — histograma e gauge de tentativas por linha, expondo a aproximação ao teto de `RES-33` | — | — | FND-08 §6.3 |
| `MET-17` | não declarado | `structurally reviewable` — critério: `dmpf_outbox_failures_total` com `dmpf_outbox_failed`/`_oldest_seconds` no catálogo (`MET-03`) — falha de publicação e o pendente que `GAR-06` exige | — | — | FND-08 §6.3 |
| `MET-18` | não declarado | `structurally reviewable` — critério: `dmpf_relay_claims_active` e `dmpf_relay_leases_expired_total` no catálogo (`MET-03`) — leases do relay, sinal do failure mode 4 | — | — | FND-08 §6.3 |
| `MET-19` | não declarado | `structurally reviewable` — critério: `dmpf_outbox_pending_rate` e `dmpf_outbox_purged_total` no catálogo (`MET-03`) — crescimento sustentado (derivada) e purga | — | — | FND-08 §6.3 |
| `MET-20` | não declarado | `structurally reviewable` — critério: `dmpf_relay_batch_size` e `dmpf_relay_message_outcomes_total` no catálogo (`MET-03`) — um registro de desfecho por mensagem, não por lote | — | — | FND-08 §6.3 |
| `MET-21` | não declarado | `structurally reviewable` — critério: o catálogo de consumo (inbox) consta de §6.4 (`MET-03`), com a espera na porta de limiar derivado do default de `RES-08` | — | — | FND-08 §6.4 |
| `MET-22` | não declarado | `structurally reviewable` — critério: `dmpf_dlq_depth`, `dmpf_dlq_oldest_seconds` e `dmpf_dlq_replays_total` no catálogo (`MET-03`), com a zona de `RUN-15` como label | — | — | FND-08 §6.5 |
| `MET-23` | não declarado | `structurally reviewable` — critério: `dmpf_quarantine_depth` no catálogo (`MET-03`), sinal separado da DLQ (`GAR-11`) | — | — | FND-08 §6.5 |
| `MET-24` | não declarado | `structurally reviewable` — critério: `dmpf_consumer_rejections_total` no catálogo (`MET-03`), com as disposições R1×D4 e R4 como labels; segundo sinal de `GAR-12` | — | — | FND-08 §6.5 |
| `MET-25` | não declarado | `structurally reviewable` — critério: `dmpf_contract_validation_failures_total` no catálogo (`MET-03`), com labels de contrato e causa; alarme na primeira ocorrência | — | — | FND-08 §6.6 |
| `MET-26` | não declarado | `structurally reviewable` — critério: `dmpf_codegen_drift_total`, `_gate_failures_total` e `dmpf_codegen_drift_ratio` no catálogo (`MET-03`) — drift de código gerado e reprovação de gate | — | — | FND-08 §6.6 |
| `MET-27` | não declarado | `structurally reviewable` — critério: `dmpf_saga_steps_pending`/`_overdue` e `dmpf_saga_compensations_total` no catálogo (`MET-03`); torna `GAR-06` verificável em saga | — | — | FND-08 §6.7 |
| `MET-28` | não declarado | `structurally reviewable` — critério: `dmpf_dependency_retries_total`, `_budget_exhausted_total` e `dmpf_dependency_breaker_state` no catálogo (`MET-03`) — repetição, orçamento de `RES-30` e estado do breaker | — | — | FND-08 §6.8 |
| `MET-29` | não declarado | `structurally reviewable` — critério: `dmpf_dependency_deadline_exceeded_total` e `dmpf_dependency_cancellations_total` no catálogo (`MET-03`), contadores separados por diagnóstico oposto | — | — | FND-08 §6.8 |
| `MET-30` | não declarado | `structurally reviewable` — critério: `dmpf_pool_connections_in_use`/`_waiting`, `dmpf_pool_acquire_duration_seconds` e `dmpf_uow_write_conflicts_total` no catálogo (`MET-03`) — recursos e contenção de escrita | — | — | FND-08 §6.8 |

**`LOG` — logging e auditoria: formato, campos, redaction, severidade, amostragem e auditoria (14)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `LOG-01` | não declarado | `structurally reviewable` — critério: log de aplicação estruturado, em JSON, com um evento por registro; linha livre não é log de aplicação | — | — | FND-08 §7.1 |
| `LOG-02` `[§13.5]` | não declarado | `structurally reviewable` — critério: o schema do logger garante os campos obrigatórios em todo registro estruturado; fecha a obrigação recebida de FND-03 (rastro de execução é telemetria com correlação, não campo de domínio) | — | — | FND-08 §7.1; §13.5 |
| `LOG-03` | não declarado | `structurally reviewable` — critério: código de erro estável e independente da mensagem; filtro e alarme referenciam o código | — | — | FND-08 §7.1 |
| `LOG-04` | não declarado | `structurally reviewable` — critério: correlação automática (injetada pelo contexto no ponto de emissão), nunca manual | — | — | FND-08 §7.1 |
| `LOG-05` | não declarado | `structurally reviewable` — critério: payload completo não é registrado por default; conteúdo só por campo da allowlist do serviço, sob redaction de `DAT-22` | — | — | FND-08 §7.2 |
| `LOG-06` | não declarado | `structurally reviewable` — critério: `DAT-22` cumprida na origem (ponto de emissão); o valor sensível não é entregue ao pipeline (log, trace e métrica) | — | — | FND-08 §7.2 |
| `LOG-07` | não declarado | `structurally reviewable` — critério: redaction preserva a utilidade diagnóstica (`DAT-23`); campo identificado com valor substituído por marcador, nunca suprimido em silêncio | — | — | FND-08 §7.2 |
| `LOG-08` | não declarado | `structurally reviewable` — critério: a projeção interna de diagnóstico não é exceção à redaction (`DAT-24`); `last_error`, diagnóstico da DLQ e campos lidos por operação obedecem à mesma classificação | — | — | FND-08 §7.2 |
| `LOG-09` | não declarado | `structurally reviewable` — critério: segredo, credencial e material de chave nunca são registrados, em nenhuma severidade e em nenhum ambiente (`DAT-02`/`DAT-06`) | — | — | FND-08 §7.2 |
| `LOG-10` | não declarado | `structurally reviewable` — critério: severidade declarada por significado — `error`/`warn`/`info`/`debug`, com faixas nomeadas; `debug` desabilitado por default em produção | — | — | FND-08 §7.3 |
| `LOG-11` | não declarado | `import-verifiable`: log técnico fora do domínio; o tipo de domínio não recebe, injeta nem expõe logger em assinatura; diagnóstico reusado `DMPF-D001`/`DMPF-D002` | `application service` registra a trilha da regra que invocou | `domain` recebe ou injeta logger, mesmo por parâmetro opcional → aresta proibida | §9 (`FIT`); `RFC §10.3`; FND-08 §7.3 |
| `LOG-12` | não declarado | `structurally reviewable` — critério: amostragem de log por classe de tráfego, nas mesmas classes de `TRC-13`; `error` nunca é amostrado | — | — | FND-08 §7.3 |
| `LOG-13` | não declarado | `structurally reviewable` — critério: onde há requisito regulatório, a auditoria é canal separado da observabilidade (`DAT-25`), com retenção, acesso e integridade próprios, não derivada do pipeline de log | — | — | FND-08 §7.4 |
| `LOG-14` | não declarado | `structurally reviewable` — critério: evento de auditoria emitido pelo `application service`, com sujeito, objeto, ação, desfecho e instante (`DAT-11` + operações de manutenção de §8) | — | — | FND-08 §7.4 |

**`RUN` — runbook mínimo: DLQ, replay, backlog, poison, saga, encerramento e pressão (20)**

| ID | Modo | Instrumento | Vetor positivo | Vetor negativo | Onde |
|----|------|-------------|----------------|----------------|------|
| `RUN-01` | não declarado | `structurally reviewable` — critério: procedimento executável por quem está de plantão sem conhecimento tácito; nomeia sintoma, sinal, passos, decisão que exige autorização e registro | — | — | FND-08 §8.1 |
| `RUN-02` | não declarado | `structurally reviewable` — critério: o runbook opera o que existe, não cria nem altera mecanismo; passo que exija capacidade inexistente é pendência (`RES-03`) | — | — | FND-08 §8.1, §13.7 |
| `RUN-03` | não declarado | `structurally reviewable` — critério: todo procedimento declara o sinal que o dispara e todo alarme de §6 aponta um procedimento (correspondência bidirecional); destinatário declarado | — | — | FND-08 §8.1 |
| `RUN-04` | não declarado | `structurally reviewable` — critério: toda operação de manutenção (replay, purga, override, intervenção manual) é auditada por `LOG-14`, com operador, alvo, instante e resultado | — | — | FND-08 §8.1 |
| `RUN-05` | não declarado | `structurally reviewable` — critério: o procedimento de backlog de outbox tem quatro passos, e a purga é agendada, não reativa | — | — | FND-08 §8.3 |
| `RUN-06` | não declarado | `structurally reviewable` — critério: relay travado é reiniciado (o lease expira, `OBX-13`), não desbloqueado à mão; alterar `locked_until` no armazenamento é vedado | — | — | FND-08 §8.3 |
| `RUN-07` | não declarado | `structurally reviewable` — critério: nenhum procedimento marca linha de outbox como publicada sem evidência; «publicou e não marcou» resolve pela idempotência do consumo (`CEN-03`) | — | — | FND-08 §8.3 |
| `RUN-08` | não declarado | `structurally reviewable` — critério: a inspeção da DLQ precede qualquer decisão e é leitura (não consome, move nem reordena); envelope preservado + diagnóstico (`GAR-07`) | — | — | FND-08 §8.4 |
| `RUN-09` | não declarado | `structurally reviewable` — critério: a inspeção classifica cada mensagem retida em um dos quatro desfechos, e o desfecho determina o procedimento | — | — | FND-08 §8.4 |
| `RUN-10` | não declarado | `structurally reviewable` — critério: poison message contida sem bloquear partição nem grupo FIFO (mecanismo de `GAR-08`, disposição de `GAR-11`); confirma pelo sinal de `MET-21`; mecanismo em §4.2 `CEN-09` | — | — | FND-08 §8.4 |
| `RUN-11` | não declarado | `structurally reviewable` — critério: a DLQ tem retenção declarada (default 14 dias); valor por canal com motivo, e o valor é desta baseline | — | — | FND-08 §8.4 |
| `RUN-11a` | não declarado | `structurally reviewable` — critério: sinal de integração (`MET-25`, `MET-26`) tratado na origem, mesmo procedimento do «poison estrutural» de `RUN-09`; a correção é do produtor (`BUF-09` de FND-05) | — | — | FND-08 §8.4 |
| `RUN-12` | não declarado | `structurally reviewable` — critério: o replay exige ferramenta, autorização e proteção contra duplicidade, os três; reinjeção manual no canal é vedada; mecanismo em §4.2 `CEN-10` | — | — | FND-08 §8.5 |
| `RUN-13` | não declarado | `structurally reviewable` — critério: o replay preserva o envelope byte a byte (`ENV-24`/`TRP-13`); reserializar, mudar `message_id` ou «corrigir» payload derrota a inbox | — | — | FND-08 §8.5 |
| `RUN-14` | não declarado | `structurally reviewable` — critério: o replay é auditado por lote e por mensagem (operador, autorização, conjunto, instante, desfecho por mensagem); mecanismo em §4.2 `CEN-10` | — | — | FND-08 §8.5 |
| `RUN-15` | não declarado | `structurally reviewable` — critério: o replay declara a sua zona; dentro da retenção, dedup por `(consumer_name, message_id)` de `INB-01`; além dela, só a idempotência de efeito (`GAR-03`) | — | — | FND-08 §8.5 |
| `RUN-16` | não declarado | `structurally reviewable` — critério: replay assistido de saga é reparação, não recuperação automática (`GAR-05`); nomeia o decisor humano, o passo de retomada e a compensação já aplicada | — | — | FND-08 §8.6 |
| `RUN-17` | não declarado | `structurally reviewable` — critério: encerramento ordenado observado e tratado, nunca redefinido (`OBX-13`); verifica em `MET-18` se restaram claims vivos; mecanismo em §4.3 `CEN-13` | — | — | FND-08 §8.7 |
| `RUN-18` | não declarado | `structurally reviewable` — critério: sob pressão de entrada, a ordem de atuação é confirmar a recusa por admissão (`MET-12`), verificar a saturação própria (`MET-11`) e só então considerar capacidade | — | — | FND-08 §8.7 |
| `RUN-18a` | não declarado | `structurally reviewable` — critério: sob repetição excessiva, a ficha de `RES-21` é avaliada como conjunto (`MET-28`, orçamento de `RES-30`, composição de `RES-39`), nunca mecanismo a mecanismo | — | — | FND-08 §8.7 |

`registro`

**Contagem por grupo.** `RES` 40 (`RES-01`..`RES-40`) · `TRC` 16 (`TRC-01`..`TRC-16`) · `MET` 31 (`MET-01`..`MET-30` + `MET-05a`) · `LOG` 14 (`LOG-01`..`LOG-14`) · `RUN` 20 (`RUN-01`..`RUN-18` + `RUN-11a` + `RUN-18a`). **Total: 121 regras `normativo`**, contíguo por prefixo, com os três sufixos de letra — o número que FND-08 §13.1 declara. As 118 regras numeradas são as das cinco faixas; `MET-05a`, `RUN-11a` e `RUN-18a` são regras acrescentadas entre faixas na revisão, contam na baseline e recebem linha aqui.

**Distribuição por modo (atribuição de FND-09, não leitura literal da fonte).** `import-verifiable` puro (2): `RES-24`, `TRC-16`, `LOG-11` — nenhum decorator, span ou logger em `domain`/`port`, diagnóstico reusado de `RFC §10.3`. `import-verifiable` + `structurally reviewable` (1): `RES-01`. `runtime-testable` (4): `RES-26` (instrumento real — harness `KIT-06`, oráculo `RAS-13`, `V32`, diagnóstico `DMPF-R004`); `TRC-01`, `TRC-04`, `TRC-07` (oráculo declarado, com cenário e execução como dependência declarada — FND-09 não instancia `CEN` de tracing nesta entrega, consistente com §4.8). `structurally reviewable` puro: as demais 113.

**Modo não declarado pela fonte.** FND-08 classifica só por força (§1.2) e não usa o vocabulário de modo de RFC §2.2 para nenhuma regra. A coluna Modo registra `não declarado` para toda a família, e o modo instrumental é atribuído por FND-09 no regime de `RAS-32` — declarar o modo antes de instanciar. Onde a §11.2 fez isso para as 42 regras de FND-03, esta subseção o faz para as 121 de FND-08.

**Os seis critérios de §13.5.** `RES-01`, `MET-03`, `MET-07`, `TRC-04`, `TRC-07` e `LOG-02` são os que FND-08 §13.5 encaminha a esta âncora como verificáveis por instrumento — instrumento nomeado pela fonte, assinalados `[§13.5]` na tabela. Dos seis, `RES-01` é `import-verifiable` + varredura, `MET-03`/`MET-07`/`LOG-02` são `structurally reviewable` (varredura decidível do catálogo e do schema), e `TRC-04`/`TRC-07` são `runtime-testable` com execução como dependência declarada.

**Dependências declaradas, não lacunas.** Valores de limiar e default que só o contexto local declara não são lacuna deste mapa: o teto de `RES-08` (2 s), os defaults de `RES-10`, `RES-14`, `RES-30`, `RES-32` e `RES-33`, a retenção de `RUN-11` (14 dias) e os limiares com owner local de `MET-05`. A execução runtime da observabilidade de entrega — o disparo efetivo das métricas de §6.3–§6.8 e a montagem de trace de `TRC-01`/`TRC-04`/`TRC-07` — pertence aos épicos de kernel (ANC-10; §12.3, item 4), no mesmo regime em que §4.8 e §12.1 declaram a observabilidade de entrega fronteira de FND-08. O que este mapa entrega é a especificação (modo + oráculo + critério); a execução fica diferida, e o diferimento é declarado, não omitido.

**IDs sem mecanismo.** Nenhum. As 121 regras recebem instrumento — inspeção decidível, aresta de import com diagnóstico reusado, ou oráculo com par de vetores. Diferente de `THR-01`/`THR-02`/`TRP-31` (§13.2), que ficam sem mecanismo por serem eles próprios a matéria fronteira de FND-08, as regras de FND-08 têm agora fonte publicada que sustenta a inspeção. A publicação de FND-08 satisfaz a condição de fechamento de §4.8/§12.1 no que toca à regra de resultado; instrumentar os vetores de execução da observabilidade de entrega permanece trabalho dos kernels sob ANC-10.

---

## §14. Pendências e handoffs

`registro` — Onze elos ficam abertos ao fim desta entrega, cada um com dona e
condição de fechamento nomeadas. Nenhum é contornado no texto; todos são
escalados. Esta seção não decide matéria de outra dona: onde a pendência é de
artefato irmão ou de âncora, este artefato a **referencia**, não a reabre. O que a
§3.5 registra como bloqueado e o que a §12 detalha como fronteira nomeada aparecem
aqui em uma linha cada, remetendo à seção que os trata — sem repetir o conteúdo.

| # | O que fica aberto | Dona | Condição de fechamento | Onde este artefato a registra |
|---|-------------------|------|------------------------|-------------------------------|
| 1 | **Implementação e execução dos kits e das fixtures nas duas stacks** — o código dos cinco test kits (§8) e das golden fixtures (§5), e a execução efetiva do round-trip com evidência registrada. É o **backlog técnico dos test kits** que o *Definition of Done* do ARQ-446 vincula aos épicos de kernel; fica aqui como handoff nomeado | Épicos de kernel, Go e TS (FND-05 §8.4) | Kits e fixtures implementados e o round-trip executado nas duas stacks, com a evidência registrada — FND-09 especifica o instrumento, o épico o executa e produz a evidência | §5, §8; matriz de §3, linhas `H3-4` e `H4-3` (implementação/execução `encaminhada`) |
| 2 | **Localização e forma do validador executável do perfil de envelope** — FND-09 tem o instrumento (§9); **onde** ele reside no repositório de contratos, e sob qual dona, permanece em aberto | Co-decisão FND-06 (ARQ-443) × FND-09 (ARQ-446) | Fechada a decisão de ownership: o validador executável reside como validação por transporte (FND-06) ou como instrumento de verificação (FND-09) | §9; referencia FND-05 §10.4 (pend. 9) e FND-06 §7.2 |
| 3 | **Codificação de wire dos tipos ainda não fixados** — timestamps e valores decimais/monetários. A fixture reserva o caso discriminatório de precisão de tipo e **não escolhe** a representação | FND-05, sob `ANC-03` | FND-05 fixa, sob `ANC-03`, a codificação de wire desses tipos; até lá a fixture reserva o caso. Codificação já decidida vira fixture nesta entrega; codificação nova volta à âncora | §6 (precisão de tipos) |
| 4 | **Base as-is prospectiva de Kafka** — a política Kafka de FND-06 não tem caso observado que a lastreie, e `TRP-41` a limita a norma de desenho; por consequência, os vetores Kafka do catálogo são especificáveis, mas não validáveis por execução contra caso real | FND-06 (ARQ-443) | Revisão de infraestrutura de mensageria de FND-06 concluída (a nomeada no cabeçalho de FND-06); até lá `TRP-41` vigora | §3.5 (dependência declarada) e §4 (vetores Kafka); referencia FND-06 §18.3 / `TRP-41` |
| 5 | **Linter de produção nas duas stacks** — deferido por `ANC-10`. O teste de arquitetura da §9 é *fitness function* na suíte, **distinto** do linter, e **não** fecha `ANC-10` | Épicos de kernel (`ANC-10`) | Verificador conforme passando nos 32 vetores de RFC §11 (RFC §12.3, registro de `ANC-10`) | §9; referencia RFC §12.3 (`ANC-10`) |
| 6 | ~~**Backpressure**~~ — **fechada nesta entrega** | FND-08 (ARQ-445) | **Satisfeita**: FND-08 publicou a regra de resultado (`RES-08`, `RES-14`, `RES-16`, `RES-17`), e o item do AC-10 passou a coberto por `CEN-44` | §4.8; o item correspondente do AC-10 fica declarado **bloqueado**, não coberto |
| 7 | **Timing de retry, DLQ e quarantine** | FND-08 (ARQ-445) | *Idem* — regra de resultado de resiliência promovida em FND-08 | §12 e §3.5 |
| 8 | **Replay operacional** | FND-08 (ARQ-445) | *Idem* | §12 e §3.5 |
| 9 | **Observabilidade de entrega** — consumer lag, profundidade da DLQ, depth/age da outbox (lacunas de FND-01 §4.2) | FND-08 (ARQ-445) | Baseline de telemetria de entrega promovido em FND-08 (`ANC-06`) | §12 e §3.5 |
| 10 | **Owner nomeado da revisão de Segurança** — FND-07 declara o **papel** («um representante de Segurança para §7 e §8, obrigatório»), não a **pessoa**; sem ela o gate de `THR-03` não é acionável e o critério de aceite correspondente de FND-07 não pode ser marcado. Não é pendência de FND-09: entra aqui porque a §9 instancia as verificações herdadas de FND-07 e herda a cadeia até ela | FND-07 (ARQ-444) | Pessoa atribuída como representante de Segurança e revisão de `THR-03` executada — a varredura das seis categorias em cada um dos sete vetores, as exclusões justificadas e o owner de cada linha | FND-07 §7.10 e §11.4 (pend. 4); referenciado aqui |
| 11 | **Gate de revisão pelos representantes de stack** — o ARQ-446 pede um representante de Go e um de TypeScript para aferir a exequibilidade dos instrumentos nas duas stacks. Gate **externo e não bloqueante** para a promoção deste artefato | Story ARQ-446, no PR | Um representante de Go e um de TypeScript revisam a exequibilidade dos instrumentos nas duas stacks; a força normativa do texto não depende deste gate | Cabeçalho (campo Revisão) e esta seção |

`rationale` — **O que não é pendência, e por quê.** A canonicalização do
`payload_hash` **não** consta desta seção. FND-05 §4.3 declara a obrigação
`quita` — SHA-256 sobre os bytes do payload como transportados, `ENV-17`..`ENV-20`
—, e FND-06 §5 fecha a byte-preservação por hop conforme (`TRP-13`..`TRP-15`). O
que resta é a **precondição** de que os bytes cheguem inalterados, que vive como
dependência declarada na linha `H4-2` da matriz de §3 — não como pendência aberta.

`registro` — **A passada de prova da matriz de §3 fecha nesta entrega.** A matriz
de obrigações herdadas é escrita em duas passadas, no mecanismo de FND-07 §2.1: a
de **atribuição**, que nasce com a fronteira e mantém cada linha em estado de
trabalho (`em redação`, `encaminhada` ou `bloqueada`); e a de **prova**, só
preenchível depois que as regras `PIR`/`CEN`/`FIX`/`ORA`/`KIT`/`FIT`/`RAS` de §2 e
de §4 a §11 existem, que leva cada linha ao seu estado terminal. A §3.2 exibe o
resultado da segunda, e registra ali a contagem por estado. A dona
desta passada é a própria redação de FND-09, e a sua condição de fechamento é
única: no aceite do PR, nenhuma linha da matriz de §3 permanece `em redação`. Uma
linha nesse estado no aceite **é defeito de entrega**, não pendência escalável.

`normativo` — Toda pendência desta seção tem **dona** e **condição de fechamento**.
Uma pendência sem uma das duas é defeito de entrega, não item aberto: sem dona,
ninguém a fecha; sem condição de fechamento, não há como saber que foi fechada.
