# Governança, BOM, pilotos e prontidão — DMPF FND-10

| Campo | Valor |
|-------|-------|
| **Status** | `draft normativo` — promovido para revisão em PR |
| **Adiciona a** | RFC DMPF Foundation v0.1, pelas âncoras **ANC-08** e **ANC-09** (RFC §12.3) |
| **Owner** | Mateus Macedo Dos Anjos (assignee de ARQ-447) |
| **Épico** | ARQ-436 — Golden Path para Sistemas Orientados a Domínio e Mensagens |
| **Story** | ARQ-447 (DMPF-FND-10) |
| **Spec** | [SPEC-VVR1X71Q](../specs/SPEC-VVR1X71Q-dmpf-governanca-bom-pilotos.md) |
| **Data** | 2026-08-21 |
| **Revisão** | Plataforma e Arquitetura no PR; **Segurança** para §5.2, porque a autoridade sobre a classificação é controle de integridade e não formalidade de processo; os owners das duas squads nomeadas em §6.2, sem cujo aceite o charter permanece proposta — o AC-11 item 4 exige participação formal |

> **O que este documento obriga.** As regras rotuladas `normativo` valem para todo
> trabalho novo do DMPF, na mesma força da RFC à qual elas se adicionam. Nem todo
> bloco aqui obriga: §1.2 define as categorias de conteúdo, §1.3 define os quatro
> **estados observáveis** que separam «escrito» de «em efeito» — leitura obrigatória
> antes de qualquer regra, porque este é o primeiro artefato do acervo cuja norma
> depende, em parte, de evidência que ele não produz — e §1.6 lista o que pertence a
> outras donas. Enquanto o status for `draft normativo`, o documento está em revisão;
> a promoção ocorre no aceite do PR.

---

## §1. Fronteira, autorização e estado

Esta seção vem antes de qualquer regra porque um artefato que adiciona a uma norma
compartilhada precisa dizer, primeiro, **até onde** ele pode obrigar. A RFC já
respondeu a essa pergunta ao registrar duas âncoras; o que segue é a leitura dessa
autorização, a convenção de leitura do texto, e — o ponto que distingue este
artefato dos sete precedentes — a distinção entre a regra estar **definida** e a
regra estar **vigente**.

Três assimetrias em relação aos precedentes condicionam todo o resto.

A primeira é de **cardinalidade de âncora**. Cada uma das sub-specs anteriores
opera sob uma âncora única. Esta opera sob **duas**, e elas não são variações do
mesmo assunto: a ANC-09 governa o produto DMPF (versão, suporte, BOM, escape
hatch, pilotos); a ANC-08 governa a integridade do metadado de classificação —
quem pode reclassificar uma unidade arquitetural, por qual rito, com qual
evidência. Um assunto é de gestão de produto; o outro é de controle de integridade.
§1.1 as trata separadamente, e nenhuma regra deste artefato invoca as duas ao mesmo
tempo como se fossem uma licença só.

A segunda é de **impacto na RFC**. Das dez âncoras do registro de RFC §12.3, nove não
exigem incremento de versão: oito declaram «Nenhum, se dentro do escopo» e a ANC-10
declara «Nenhum», sem a condicional. A ANC-08 é a única que declara **«Menor — esta
RFC passa a citar o processo definido»**. Definir o
processo, aqui, cria uma pendência na RFC que este PR **não** resolve: §14.3 exige,
para `0.2` em diante, revisão pelas quatro áreas e reconciliação com baseline
aprovado, e o aceite de 2026-08-14 que dispensou ambos vale apenas para a `0.1`.
§1.4 e §8 registram essa pendência em vez de a executar.

A terceira é de **dependência de evidência externa**. Os precedentes normatizam
mecanismos cuja verificação é técnica: um linter, um teste, um campo de log. Vários
itens que os critérios de aceite AC-11 e AC-12 cobram deste artefato dependem de
atos que só pessoas praticam — a assinatura de uma squad, a criação de um épico no
tracker, o aceite de um titular para uma função de autoridade, o registro formal de
uma decisão de continuidade. Uma entrega documental pode especificar o instrumento,
o rito e o critério de verificação de cada um; não pode produzir o ato. Escrever a
norma como se o ato já tivesse ocorrido seria a falha mais fácil e mais grave deste
artefato. §1.3 é o mecanismo que a impede.

### §1.1 As duas autorizações: âncoras ANC-08 e ANC-09

`normativo`

Este artefato **adiciona** à RFC DMPF Foundation v0.1 por duas âncoras distintas
(RFC §12.3). Ele não edita a RFC nesta entrega. Para a ANC-09, adição por âncora
dentro do escopo permitido é revisão em PR, sem incremento de versão (RFC §14.2). Para
a ANC-08, a própria âncora declara impacto «Menor», o que significa que a RFC
precisará citar o processo definido aqui — pendência registrada em §1.4 e §8, com a
condição de fechamento explícita.

**ANC-09 — governança, BOM e pilotos**, conforme o registro:

| Campo | Valor |
|-------|-------|
| Assunto | Governança, BOM e pilotos |
| Owner / autoridade | FND-10 — ARQ-447 |
| Escopo permitido | Matriz de versões certificadas, escape hatches e seleção de pilotos |
| Invariantes | §12.2 M1–M4; **escape hatch não pode contornar constraint P0** |
| Artefato sucessor | Spec e seção própria |
| Condição de fechamento | FND-10 concluída e revisada |
| Impacto de versão | Nenhum, se dentro do escopo |
| ADR exigido | Não |

**ANC-08 — processo de autorização da classificação**, conforme o registro:

| Campo | Valor |
|-------|-------|
| Assunto | **Processo de autorização da classificação** — quem aprova mudança de `block` e `bounded_context`, por qual rito, e o que constitui evidência de autorização |
| Owner / autoridade | FND-10 — ARQ-447 |
| Escopo permitido | Definir autoridade, rito e artefato de evidência que satisfaçam **T4–T6 de §10.2** |
| Invariantes | **T1–T6 de §10.2 permanecem**; o mecanismo mínimo de §10.2 vale **até o fechamento** |
| Artefato sucessor | Spec de governança e seção própria |
| Condição de fechamento | FND-10 concluída e revisada; **até lá, R1 permanece parcialmente mitigado** |
| Impacto de versão | **Menor** — esta RFC passa a citar o processo definido |
| ADR exigido | **Sim** — a autoridade de classificação é decisão estrutural |

`rationale` — Por que as duas numa entrega só, e não uma spec separada para a
ANC-08. O registro de âncoras nomeia FND-10 como owner de ambas, e RFC §12.3 fecha
o registro: «uma sub-spec sem âncora correspondente» não normatiza. Deixar a ANC-08
sem tratamento a manteria aberta sem dona ativa, perpetuando o mecanismo mínimo
provisório de §10.2 — e com ele o estado «R1 parcialmente mitigado», que a própria
âncora amarra ao fechamento de FND-10. A alternativa de abrir uma spec própria foi
considerada e descartada: ela não altera o owner registrado nem antecipa o
fechamento, e adia por um ciclo de planejamento a única coisa que fecha R1.

`registro` — **Divergência de destino, declarada.** O entregável 1 da ARQ-447 pede um
«Capítulo da RFC: governança do produto», e o DoD da story repete: «Capítulo de
governança integrado à RFC». Este artefato é uma **seção própria** em `docs/dmpf/`, e
não um capítulo da RFC. A razão é normativa, não de conveniência: o campo «Artefato
sucessor» de ambas as âncoras fixa «Spec e seção própria» e «Spec de governança e
seção própria», e RFC §12.1 é o mecanismo — o conteúdo vive no artefato e a RFC o
alcança pelo endereço estável da âncora, sem editá-la e sem incrementar a sua versão
(RFC §14.2). Escrever um capítulo dentro da RFC exigiria editá-la, o que muda o rito e
a versão. O registro da âncora prevalece sobre a formulação do Jira, e é o mesmo
tratamento que os sete artefatos precedentes deram ao mesmo pedido.

`normativo` `GOV-01` — **Precedência.** Onde este artefato e a RFC divergirem,
prevalece a RFC. Uma divergência não se resolve neste documento: por M4, ela indica
que a fronteira de RFC §1.4 mudou, e mudar fronteira é mudança de versão da RFC,
com o rito de RFC §14.2.

`normativo` `GOV-02` — **Nenhuma regra deste artefato invoca as duas âncoras
simultaneamente como fundamento.** Toda regra `normativo` declara sob qual âncora
opera. Uma regra sob ANC-09 não pode alcançar a integridade da classificação, e uma
regra sob ANC-08 não pode alcançar versionamento, BOM ou piloto. O motivo é a
disciplina M4 aplicada a duas fronteiras em vez de uma: escopos permitidos
diferentes não somam num escopo maior.

| Seção | Âncora | Prefixo e faixa |
|-------|--------|-----------------|
| §1 fronteira, força e estados | ANC-08 e ANC-09, declaradas por regra | `GOV-01` a `GOV-09` |
| §3 modelo de versão, suporte e instrumentos | ANC-09 | `GOV-10` a `GOV-25` |
| §4 BOM e certificação | ANC-09 | `BOM-01`+ |
| §5 abertura — a não substituição entre os dois ritos | ANC-09 | `GOV-26` |
| §5.1 escape hatch | ANC-09 | `GOV-30`+ |
| §5.2 autorização da classificação | **ANC-08** | `AUT-01`+ |
| §6.1 adoção e faseamento | ANC-09 | `ADO-01`+ |
| §6.2 pilotos e charter | ANC-09 | `PIL-01`+ |
| §7 prontidão e continuidade | ANC-09 | `RDY-01`+ |

`normativo` — **Reserva de faixa declarada.** O prefixo `GOV` é particionado em
faixas disjuntas — `01`–`09` (fronteira), `10`–`25` (modelo de versão), `26`–`29`
(abertura de §5, da qual só `26` está em uso e `27`–`29` ficam **reservados sem
uso**), `30`+ (escape hatch) — e nenhuma delas é reutilizada. Faixa reservada e não
usada é declarada aqui em vez de ser preenchida por conveniência: renumerar regra
publicada é o que a estabilidade de ID existe para evitar. A partição segue o precedente
do FND-09, que dividiu `ORA` em duas faixas pelo mesmo motivo: manter a
endereçabilidade por domínio sem multiplicar prefixos. O escape hatch fica sob `GOV`
porque a base conceitual o classifica como instrumento de governança
(`Parte-1 §17.3`), e porque o prefixo mnemônico `ESC` **está ocupado** —
`ESC-01` a `ESC-09` designam extensões opt-in (Event Sourcing e CQRS físico) em
`upr-decision-mensagens.md`, do FND-03. Reutilizá-lo repetiria a ambiguidade que o
FND-09 teve de corrigir renomeando um prefixo já publicado.

`normativo` — **Monotonicidade em concreto.** As invariantes das duas âncoras são
citadas ao longo do texto, e nenhuma regra deste artefato as afrouxa:

| Invariante | Âncora | Como este artefato a trata | Onde |
|------------|--------|----------------------------|------|
| **Escape hatch não pode contornar constraint P0** | ANC-09 | Convertida em regra de negação com universo positivo fechado: `GOV-31` enumera o que é excepcionável e `GOV-32` nega o resto, incluindo célula de §7 e invariante de âncora | §5.1 |
| §12.2 M1 — não esvaziar regra vigente | ANC-09 | `PTB-13` a `PTB-16` do FND-05, `ERR-16` do FND-07 e `REP-03` são **recepcionados**, nunca redefinidos; §1.6 declara a fronteira | §1.6, §3.4 |
| §12.2 M2 — não relaxar, revogar ou reinterpretar invariante | ambas | Nenhuma regra daqui altera T1–T6, célula de §7 ou constraint P0; o escape hatch é o único instrumento que poderia, e `GOV-32` o proíbe | §5.1, §5.2 |
| §12.2 M3 — relaxar P0 exige nova versão da RFC + ADR | ANC-09 | Declarado como via única em `GOV-32`; o escape hatch não é caminho alternativo para o efeito de M3 | §5.1 |
| §12.2 M4 — não exceder o escopo permitido | ambas | §1.6 lista o excedente `encaminhado` com dona; §1.4 separa o definido do vigente | §1.4, §1.6 |
| **T1–T6 de §10.2 permanecem** | ANC-08 | Recepcionados literalmente em §5.2 e usados como critério de suficiência do rito: um rito que não satisfaça T4, T5 e T6 é inválido | §5.2 |
| **O mecanismo mínimo de §10.2 vale até o fechamento** | ANC-08 | `AUT-09` declara a transição, e ela é condicional — o mecanismo mínimo não é encerrado por este documento | §1.3, §5.2 |

### §1.2 Convenção de citação e classificação de força

`normativo` — **Convenção de citação.** Neste artefato, `§N` sem prefixo designa uma
seção **deste documento**; a RFC aparece sempre como `RFC §N`; e a base conceitual
sempre como `Parte-1 §N`. A convenção é a dos sete artefatos precedentes, e é a
inversa da que vale **dentro** da RFC, onde `§N` designa a própria RFC (RFC §1.3) —
por isso o prefixo é obrigatório aqui. O arquivo físico da base conceitual vive fora
do versionamento, então a citação é sempre pelo rótulo `Parte-1 §N`, nunca por
caminho.

`normativo` — **Classificação de força.**

| Rótulo | Significado | Obriga? |
|--------|-------------|---------|
| `normativo` | Regra que este artefato estabelece sobre governança do produto (ANC-09) ou sobre autorização da classificação (ANC-08), dentro do escopo permitido da âncora declarada | **Sim**, no estado que §1.3 lhe atribuir |
| `recepcionado` | Conteúdo reproduzido da Parte-1, da RFC ou de um artefato precedente para dar contexto contíguo, sem força nova e sem reabertura | Não — a força permanece da fonte |
| `encaminhado` | Assunto de outra sub-spec ou de outro épico, citado apenas como fronteira rotulada, com dona explícita | Não — passa a obrigar quando a dona normatizar |
| `gate externo` | Ato que uma entrega documental não pratica: assinatura, criação de item no tracker, aceite de titular, registro formal de decisão. O artefato define o instrumento e o critério de verificação; o ato fica pendente com dona e condição | Não — é a **condição** de vigência ou de fechamento, nunca a satisfação dele |
| `rationale` | Justificativa de uma decisão normativa, incluindo alternativas descartadas | Não |
| `registro` | Ledger de obrigações (§2), matrizes de evidência (§6.2, §7), acionamento de ADR (§8), rastreabilidade e pendências (§8) | Não |

`rationale` — O rótulo `gate externo` é novo no acervo; os sete precedentes não
precisaram dele. A razão é a terceira assimetria da abertura desta seção: os
critérios AC-11 e AC-12 cobram cinco atos de terceiros. Sem um rótulo próprio, cada
um deles teria de aparecer ou como `normativo` — o que afirmaria uma vigência
inexistente — ou como `encaminhado` — o que sugeriria que outra sub-spec o
normatizará, quando o que falta não é norma, e sim o ato. A alternativa considerada
foi diluí-los em prosa na seção de pendências; foi descartada porque prosa não é
endereçável, e um gate que não se endereça é um gate que se lê como fechado.

### §1.3 Os quatro estados observáveis

`normativo`

Este artefato **nunca** usa «definido» como sinônimo de «vigente», nem «vigente»
como sinônimo de «fechado». Toda regra deste artefato, e toda pendência, declara em
qual destes quatro estados se encontra:

| Estado | Significado | Quem o constata |
|--------|-------------|-----------------|
| `definido` | A regra está escrita e aprovada em PR neste artefato | O próprio PR de promoção |
| `vigente` | A regra está em efeito e substitui o que havia antes | A condição normativa declarada **por regra** — nunca presumida |
| `âncora fechada` | ANC-08 ou ANC-09 com a condição de fechamento do registro satisfeita | «FND-10 concluída e revisada» (RFC §12.3) |
| `AC fechado` | AC-11 ou AC-12 do épico satisfeito | Evidência externa: assinatura das squads, épicos criados e vinculados, decisão de continuidade registrada |

`normativo` `GOV-03` — **Uma regra deste artefato passa de `definido` a `vigente`
apenas pela condição que ela mesma declara.** Uma regra sem condição declarada é
`vigente` na promoção do PR. Uma regra com condição declarada permanece `definido`
enquanto a condição não for satisfeita, e o texto da regra nomeia quem constata a
satisfação. Nenhuma regra deste artefato se declara `vigente` por conveniência de
redação, e nenhuma seção afirma vigência em nome de outra.

`normativo` `GOV-04` — **`AC fechado` e `âncora fechada` não se inferem de
`vigente`.** Uma regra vigente pode conviver com o critério de aceite que ela serve
ainda aberto, quando o que falta é um `gate externo`. **§8.4 é o único lugar deste
artefato que enuncia estado de AC e de âncora**, e o faz por tabela com condição
explícita; §7.2 enuncia o estado das métricas de conclusão do épico, que é outra
coisa. Nenhuma outra seção declara AC ou âncora fechados.

`rationale` — Por que quatro estados, e por que não escolhemos um lado da leitura
ambígua da RFC. O texto de §10.2 é ambíguo quanto ao momento em que o mecanismo
mínimo cessa. O corpo da subseção diz: «Até que o FND-10 **defina** o processo, vale
o mecanismo mínimo». O título do mesmo bloco diz: «Comportamento até o FND-10
**concluir**». E o registro de ANC-08 fixa a condição de fechamento em «FND-10
concluída e revisada». As três formulações admitem três marcos diferentes — a
definição do processo, a conclusão da sub-spec e a revisão dela.

Declarar o mecanismo mínimo encerrado por conta própria, com base na leitura mais
favorável, produziria duas normas concorrentes sobre o mesmo ato durante o intervalo
em que o processo definido aqui ainda não tem titular aceito: o rito de §5.2 e o
mecanismo mínimo de §10.2 valeriam ao mesmo tempo, com autoridades diferentes. A
alternativa oposta — tratar tudo como provisório até a revisão — esvaziaria a
entrega, porque nada do que este artefato escreve obrigaria antes de um ato que ele
não controla.

Os quatro estados resolvem a ambiguidade sem a decidir: o processo fica `definido`
pelo PR, e a transição para `vigente` fica amarrada a uma condição explícita em
`AUT-09`, que nomeia o titular aceito como constatador. Enquanto ela não ocorre, o
mecanismo mínimo continua sendo a norma aplicável, por invariante da própria âncora
— e isso é dito, não subentendido.

### §1.4 O que é normativo agora, e o que ainda não está vigente

`registro`

| Conteúdo | Seção | Estado na promoção do PR | Condição para `vigente` |
|----------|-------|--------------------------|-------------------------|
| Modelo de versão, suporte e depreciação de produto | §3 | `vigente` | — |
| Release train: existência de cadência declarada e data de corte antecipada | §3.4 | `vigente` | — |
| Release train: **valor** da periodicidade | §3.4 | `definido` | `GOV-16` — primeira release publicada; constata o owner do épico de kernel |
| Matriz de compatibilidade por sujeito versionado | §3.3 | `vigente` | — |
| Os dez instrumentos de `Parte-1 §17.3`: os normatizados | §3.6 | `vigente` | — |
| Os dez instrumentos: os `encaminhado` | §3.6 | `definido` como fronteira | A dona nomeada normatizar |
| Template e schema do BOM, e a máquina de estados de certificação | §4 | `vigente` | — |
| Conteúdo do BOM (versões reais) | §4 | fora de escopo | Primeira instância acompanha o kernel — épico 1 ou 2 de §20 |
| Escape hatch: requisitos, universo positivo, regra de negação, ciclo de vida | §5.1 | `vigente` | — |
| Processo de autorização da classificação: autoridade, rito, evidência | §5.2 | `definido` | `AUT-09` — titular aceito para a função de autoridade |
| Mecanismo mínimo de RFC §10.2 | §5.2 | `vigente` por invariante de ANC-08 | Permanece até `AUT-09` ser satisfeita |
| Roadmaps de adoção: critérios, decisão por canal, rollback | §6.1 | `definido` | `ADO-00` — ampliação do escopo da ANC-09 por RFC §14.2 (P13); a força normativa está suspensa até lá |
| Charter dos dois pilotos: fluxo, cadeia, critérios de entrada | §6.2 | `definido` | Aceite formal das squads — `gate externo` |
| Matriz de métricas dos pilotos: campos, instrumento, prazo | §6.2 | `vigente` como instrumento | — |
| Valores de baseline das métricas | §6.2 | ausentes | Coleta pelo instrumento declarado, com owner e prazo |
| Gabarito de verificação de Ready contra a DoR §17 | §7 | `vigente` | — |
| Estado `Ready` dos cinco épicos | §7 | não declarado | `gate externo` — criação, vinculação e estimativa pela equipe |
| Instrumento de registro da decisão de continuidade | §7.3 | `vigente` | — |
| A decisão de continuidade em si | §7.3 | ausente | `gate externo` — revisão final do épico |
| Acionamento dos dois ADRs | §8.1 | `vigente` como acionamento | Redação e numeração são de FND-11 |
| Citação do processo em RFC §10.2 | §8.4 | pendente | RFC `0.2`: revisão pelas quatro áreas + baseline aprovado (§14.3) |

### §1.5 Os cinco gates externos

`gate externo`

Nenhum dos cinco é satisfeito por este artefato, e nenhum deles é apresentado como
satisfeito em qualquer seção normativa. A lista é exaustiva por construção: um gate
que apareça no texto sem estar aqui é defeito de fronteira.

| # | Ato pendente | Cobrado por | Dona | Condição de satisfação | Onde é tratado |
|---|--------------|-------------|------|------------------------|----------------|
| G1 | Aceite formal das duas squads para os fluxos-piloto | AC-11 item 4; DoD do épico item 8 | Owner do ARQ-447 junto às lideranças das duas squads | Registro nominal de aceite por squad, com owner e data | §6.2 |
| G2 | Aceite de titular para a função de autoridade de classificação | ANC-08; T4–T6 de RFC §10.2 | Arquitetura, com Segurança como revisora | `AUT-09` — função provida por titular nomeado e aceito | §5.2 |
| G3 | Criação e vinculação dos cinco épicos subsequentes no tracker | AC-12 itens 1 e 2; DoD item 10 | Owner do ARQ-436 | Cinco épicos criados, vinculados ao ARQ-436 e verificados contra a DoR §17 | §7 |
| G4 | Registro formal da decisão de continuidade | AC-12 item 4; DoD item 12 | Revisão final do épico ARQ-436 | Decisão `seguir` / `ajustar` / `interromper` registrada pelo instrumento de §7.3 | §7.3 |
| G5 | Citação do processo de §5.2 em RFC §10.2, com o incremento `0.2` | ANC-08, impacto «Menor» | Owner da RFC | Revisão pelas quatro áreas e reconciliação com baseline aprovado (RFC §14.3) | §8.4 |

`normativo` `GOV-05` — **Um `gate externo` não é satisfeito por declaração deste
artefato, nem por ausência de menção.** Uma seção que dependa de um dos cinco cita
o identificador `G1` a `G5` e declara o efeito da pendência sobre o estado da regra.
Marcar critério de aceite como atendido por antecipação é defeito de rastreabilidade,
no mesmo sentido em que RFC §13.2 trata a citação de ADR por número definitivo antes
da promoção.

### §1.6 O que este artefato encaminha

`registro` — Assuntos que aparecem no texto como fronteira rotulada, com dona. As
quatro primeiras já estão normatizadas em outro lugar: redefini-las aqui produziria
duplicata ou contradição, e ambas violam M1.

| Assunto | Dona | Regra que prevalece | Onde é citado |
|---------|------|---------------------|---------------|
| Depreciação de **contrato** | FND-05 — `cloudevents-protobuf-buf.md`, **publicado**, sob ANC-03 | `PTB-13` a `PTB-16`: depreciar é estado declarado no `.proto`, com janela mínima de **180 dias** (`PTB-14`), sem relaxar os gates de §6 (`PTB-16`) | §3.4 — aqui a depreciação do **produto**, que é outro sujeito |
| Depreciação de **código de erro** | FND-07 — `contexto-erros-seguranca.md`, **publicado**, sob ANC-05 | `ERR-16`: código que deixa de ser emitido entra em depreciação declarada com substituto nomeado | §3.4 |
| Ownership de diretório de **contrato** | FND-05, **publicado** | `REP-03`: todo bounded context tem owner em `CODEOWNERS`, e o owner é equipe, não pessoa | §3.5, §5.2 — aqui a autoridade sobre a **classificação**, que não é a mesma coisa |
| Governança do **documento** RFC | RFC §14.2 e §14.3 | Versionamento, evolução e promoção de status da própria RFC | §3.1, §8.4 |
| **Registro autoritativo de stacks, pins de Buf e predicados de runtime** | FND-05, **publicado**, sob ANC-03 | O registro é de lá; o BOM o **referencia** por coordenada, sem duplicar valor | §4.3 |
| **Gesto** da repetição e do retry em cada transporte | FND-06 — `politicas-transporte.md`, **publicado**, sob ANC-04 | Políticas por transporte, incluindo as regras normativas da migração de canal | §6.1 — aqui o planejamento da adoção, não a execução |
| Baseline de telemetria e limiares de resiliência | FND-08 — `resiliencia-observabilidade.md`, **publicado**, sob ANC-06 | Catálogo de sinais e políticas de retry e degradação | §6.1, §6.2 — aqui o faseamento da adoção e a métrica do piloto |
| **Redação e numeração definitiva dos ADRs** | FND-11 — ARQ-448, `backlog` | Promoção para `docs/adr/` na faixa `010`–`024` | §8.1, §8.2 |
| **Reconciliação de cardinalidade** da série de ADRs | FND-11 | — | §8.2 |
| Evidência de interoperabilidade Go ↔ TypeScript | FND-09 — ARQ-446, **mergeado em `develop` durante esta entrega** (PR #14) | Fixtures cruzadas e critérios de interoperabilidade, sob ANC-07 | §6.2 — este artefato **não** cita conteúdo do FND-09; ver `GOV-06` |
| Execução dos pilotos | Épico 5 de ARQ-436 §20 | — | §6.2, §7 |
| Migração dos serviços existentes | Épicos de kernel e o épico 5 | — | §6.1 |

`normativo` `GOV-06` — **Este artefato não cita conteúdo do FND-09.** O FND-09
aparece somente como fronteira nomeada e como dependência em §7, nunca como regra
citada.

`registro` — **A razão original desta regra caducou durante a entrega, e o registro
fica.** Quando §1 a §8 foram redigidas, o artefato do FND-09 não estava no histórico
versionado, e citar regra dele produziria referência que o PR não resolveria. O PR #14
do FND-09 foi **mergeado em `develop` antes deste PR**, o que remove esse impedimento.
A decisão de não citar é **mantida por escopo**, não mais por indisponibilidade:
incorporar as regras do FND-09 exigiria revisitar a justificativa do par de pilotos em
`PIL-02` — que é precisamente a fronteira entre cobertura dual-stack e evidência de
interoperabilidade —, e isso é reabrir §6.2, não acrescentar uma citação. A
reconciliação com o FND-09 fica registrada como pendência P9, com dona nomeada.
Verificado no merge: o FND-09 usa os prefixos `PIR`, `CEN`, `FIX`, `ORA`, `KIT`,
`FIT` e `RAS`, **nenhum deles em colisão** com os seis deste artefato, e não endereça
obrigação nova a FND-10 — o ledger de §2.1 permanece completo em doze.

`rationale` — Nomear a fronteira em vez de a resolver por silêncio tem custo
visível: a evidência de interoperabilidade é exatamente o que sustentaria a
justificativa mais forte para o par de pilotos escolhido. §6.2 assume esse custo e
declara a justificativa mais fraca que o disco sustenta — cobertura dual-stack — em
vez da mais forte que o disco não sustenta.

---

## §2. Ledger de obrigações herdadas

`registro`

Os sete artefatos publicados e a RFC endereçam obrigações nominalmente a esta
sub-spec. Esta seção as trata uma a uma. A modelagem é **obrigação única com N
fontes**: quando duas ou mais fontes cobram o mesmo ato, elas aparecem na mesma
linha como fontes comprobatórias, nunca como obrigações distintas. Contar evidência
como obrigação inflaria o ledger e faria uma única delegação parecer duas.

### §2.1 As doze obrigações

| # | Obrigação | Fontes | Âncora | Destino | Estado |
|---|-----------|--------|--------|---------|--------|
| O1 | Cronograma, faseamento e plano de migração da adoção de **Kafka** na organização | FND-06 `politicas-transporte.md` §15, §1.4 e §17 — três citações concordantes | ANC-09 | §6.1 | `especificada, força suspensa` — critérios e rito definidos; vigência depende de `ADO-00` (P13) |
| O2 | Cronograma e faseamento da adoção da **baseline de resiliência e observabilidade** no acervo | FND-08 `resiliencia-observabilidade.md` §13 | ANC-09 | §6.1 | `especificada, força suspensa` — ver `ADO-00` (P13) |
| O3 | Fixação da **versão das *semantic conventions* de mensageria** do OpenTelemetry no BOM, **com owner da primeira instância real** | FND-08 §5 (`resiliencia-observabilidade.md`, três citações) | ANC-09, via `Parte-1 §17.2` | §4.4 | `atendida com pendência de titular` — slot e owner definidos; o valor acompanha a primeira instância |
| O4 | Definir **autoridade, rito e artefato de evidência** que satisfaçam T4–T6 de §10.2 | RFC §10.2 | **ANC-08** | §5.2 | `definida, não vigente` — depende de **G2** |
| O5 | **Acionar o ADR** do processo de autorização da classificação | RFC §13.3 | **ANC-08** | §8.1 | `atendida` — acionado; redação é de FND-11 |
| O6 | **Selecionar e formalizar os dois fluxos-piloto** | FND-01 `inventario-as-is.md` §5 **e** o §10 dos inventários de `legado-titulos-shared-services` e `legado-ativavel-services` — fontes comprobatórias da mesma delegação | ANC-09 | §6.2 | `selecionada, não formalizada` — depende de **G1** |
| O7 | ADR de **BOM, compatibilidade, versionamento e depreciação** (ADR-015 da tabela §8 do épico) | Épico ARQ-436 §8; ARQ-447 entregável 5 e DoD | ANC-09, por exigência do épico — a âncora em si diz «ADR exigido: Não» | §8.1 | `atendida` — acionado como provisório, sem número definitivo |
| O8 | Os **quatro instrumentos de governança** que a base conceitual prescreve e a spec não recolheu: owners por pacote, codemods e generators junto de breaking changes, office hours e canal de suporte, métricas de adoção e fricção | `Parte-1 §17.3`, vigente por RFC §14.4 | ANC-09 | §3.6 | `atendida` — cada um normatizado ou `encaminhado` individualmente |
| O9 | **Certificação da toolchain do FND-05** no BOM — pins de Buf e plugins, predicados de runtime, autoridade do registro de stacks: **referenciar, não duplicar** | FND-05 `cloudevents-protobuf-buf.md`, registro autoritativo | ANC-09 | §4.3 | `atendida` |
| O10 | Pré-requisito de **mais de um aprovador**: autor e autorizador coincidentes reprovam | FND-05 (rito de bootstrap); T4 de RFC §10.2 | **ANC-08** | §5.2 | `atendida` — incorporada em `AUT-04` |
| O11 | **Regras normativas da migração de canal**: decisão por canal, revisão de infraestrutura, drenagem de backlog, identidade e `consumer_name` estável | FND-06 `politicas-transporte.md` §15 e correlatas | ANC-09 | §6.1 | `atendida` — `COE-01`..`COE-06` e `TRP-10` são **recepcionadas** e seguem vigentes por suas próprias âncoras, independentemente de `ADO-00` |
| O12 | **Reconciliação de cardinalidade** da série de ADRs — não apenas a omissão de FND-10 na tabela de cadência | Aritmética de `SPEC-DBTRMM3X` contra o acervo (§8.2) | ANC-09 | §8.2 | `encaminhada` com dona **FND-11** e mapa proposto como insumo |

`normativo` `GOV-07` — **Nenhuma obrigação deste ledger permanece no estado «em
redação» após a promoção do PR.** Cada linha declara `atendida`, `atendida com
pendência de titular`, `definida, não vigente` ou `encaminhada` — e, nos três
últimos casos, nomeia a dona e a condição. Uma obrigação sem destino e sem estado é
defeito de fronteira, não omissão tolerável.

### §2.2 As três listas: dependências, insumos normativos e gates de fechamento

`registro`

Separar as três impede a confusão que motivou a decisão de iniciar esta sub-spec em
paralelo às duas irmãs abertas: **redigível** não é o mesmo que **fechável**.

`start_dependencies` — o que precisa existir para redigir este artefato. Todas
satisfeitas na abertura da redação:

| Fonte | Estado |
|-------|--------|
| FND-01 — `inventario-as-is.md` | mergeado; é a dependência declarada da spec |
| RFC DMPF Foundation v0.1 | mergeada, status `normativo` |
| FND-05 — `cloudevents-protobuf-buf.md` | mergeado |
| FND-06 — `politicas-transporte.md` | mergeado |
| FND-07 — `contexto-erros-seguranca.md` | mergeado |
| FND-08 — `resiliencia-observabilidade.md` | mergeado |
| Base conceitual — `Parte-1 §17`, `§18`, `§20` | recepcionada pela RFC §1.3 |

`normative_inputs` — o que este artefato cita como norma vigente de outra fonte,
sem reabrir: RFC (§7, §10.1, §10.2, §12.2, §12.3, §13.2, §13.3, §14.2, §14.3,
§14.4); FND-05 (`PTB-13` a `PTB-16`, `REP-03`, registro de stacks); FND-06
(políticas por transporte e regras de migração de canal); FND-07 (`ERR-16`); FND-08
(catálogo de sinais e a exigência de versão das *semantic conventions*);
`Parte-1 §17.2`, `§17.3` e `§18`.

`closure_gates` — o que falta para fechar, com dona. Distinto das duas listas
anteriores: nenhum destes impedia a redação, e todos impedem o fechamento:

| # | Gate | Dona | Efeito de continuar aberto |
|---|------|------|----------------------------|
| C1 | **G1** — aceite formal das squads | Owner do ARQ-447 | AC-11 permanece aberto; O6 fica `selecionada, não formalizada` |
| C2 | **G2** — titular aceito para a autoridade de classificação | Arquitetura, revisão de Segurança | §5.2 fica `definido`; o mecanismo mínimo de RFC §10.2 segue vigente; R1 segue parcialmente mitigado |
| C3 | **G3** — cinco épicos criados e vinculados | Owner do ARQ-436 | AC-12 itens 1 e 2 permanecem abertos |
| C4 | **G4** — decisão de continuidade registrada | Revisão final do épico | AC-12 item 4 permanece aberto |
| C5 | **G5** — citação em RFC §10.2 e incremento `0.2` | Owner da RFC | ANC-08 não fecha: o registro declara impacto «Menor», e ele não foi realizado |
| C6 | Valores de baseline das nove métricas | Owners dos dois pilotos | AC-11 item 5 fica parcial: critérios acordados, partida não medida |
| C7 | Reconciliação de cardinalidade dos ADRs (O12) | FND-11 — ARQ-448 | A série segue com mais acionamentos do que slots reservados |
| C8 | Reconciliação com o FND-09, **mergeado em `develop` durante esta entrega** | ARQ-446 e o owner desta sub-spec | Sem colisão de prefixo e sem obrigação nova (`GOV-06`); resta revisitar `PIL-02` à luz das fixtures cruzadas. Não afeta as âncoras deste artefato |

`normativo` `GOV-08` — **A condição de fechamento de ANC-08 e de ANC-09 é
verificada contra a tabela `closure_gates`, não contra a conclusão da redação.**
«FND-10 concluída e revisada» inclui os gates que a própria sub-spec declara. §8.4
enuncia o estado de cada âncora e o mantém aberto enquanto houver gate pendente que
lhe corresponda.

`normativo` `GOV-09` — **A decisão de redigir esta sub-spec em paralelo ao FND-09 e
ao FND-11 não antecipa nenhum fechamento.** A spec declara que FND-10 «consolida, então
encerra por último», e a umbrella registra a mesma ordem. Redigir em paralelo é
admissível porque as `start_dependencies` estão satisfeitas; fechar em paralelo não
é. C7 e C8 registram as duas irmãs abertas, e §8.4 as trata como condição.

---

## §3. Modelo de versão, suporte e instrumentos de governança

`normativo` — Toda esta seção opera sob **ANC-09**, no escopo permitido «matriz de
versões certificadas». IDs na faixa `GOV-10` a `GOV-25`.

O problema que esta seção resolve: o risco §15 do épico registra «congelar versões
no documento arquitetural», e a base conceitual fecha `Parte-1 §17.2` com a mesma
advertência — «o documento arquitetural não deve congelar indefinidamente versões
de bibliotecas». A mitigação declarada é «gerenciar versões em BOM publicada por
release». Mas BOM sem modelo de versão é planilha: para dizer que uma combinação é
suportada, é preciso antes dizer **o que** é versionado, **o que** significa a
compatibilidade de cada coisa, e **quem** decide.

### §3.1 Leitura do escopo permitido

`rationale` — Uma tensão de fronteira tratada aqui em vez de por silêncio. O
**assunto** registrado da ANC-09 é «Governança, BOM e pilotos»; o **escopo
permitido** é mais estreito: «Matriz de versões certificadas, escape hatches e
seleção de pilotos». Depreciação de produto e janela de suporte aparecem no assunto,
no AC-11 item 1 («compatibilidade, depreciação e suporte estão definidos») e na
spec — mas não aparecem, com essas palavras, no escopo permitido. Pela disciplina M4
do FND-08, o excedente sairia `encaminhado`.

A leitura adotada é outra, e ela é declarada: **a matriz de versões certificadas é
o instrumento em que a janela de suporte se materializa.** Declarar que uma
combinação é certificada é declarar que ela é suportada; declarar que deixa de ser é
depreciá-la. Suporte e depreciação de produto não são matéria adicional à matriz —
são o conteúdo dela lido no eixo do tempo. A alternativa de encaminhar era
inexequível na prática: não há outra dona registrada para o assunto, e o AC-11 item
1 não fecha sem ele. O que **fica** fora, por não ser produto, está em §1.6 e §3.5:
depreciação de contrato é do FND-05, de código de erro é do FND-07, e do documento
RFC é de RFC §14.

### §3.2 Os cinco sujeitos versionados

`normativo` `GOV-10` — **Uma regra de compatibilidade deste artefato declara o seu
sujeito.** «Compatibilidade de pelo menos N e N−1» é indecidível sem sujeito: N de
quê, compatível com quê, na avaliação de quem. O texto que esta regra detalha —
`Parte-1 §17.3`, quinto instrumento, «compatibilidade de pelo menos N e N−1 conforme
criticidade» — não nomeia sujeito, direção nem autoridade, e o FND-05 já usa `N−1`
com um sujeito próprio. Uma regra sem sujeito declarado é inaplicável, não
permissiva.

| Sujeito | Unidade versionada | O que significa `N` |
|---------|--------------------|---------------------|
| **Produto DMPF** | A release do produto, à qual corresponde exatamente um BOM publicado | A release corrente |
| **Kernel** | O módulo Go e o pacote TypeScript do kernel, versionados de forma independente entre si | A major corrente de cada kernel |
| **Contrato de wire** | A major do pacote Protobuf | **Fora deste artefato** — `PTB-01` a `PTB-16`, FND-05 |
| **Runtime certificado** | A versão da linguagem e do runtime: Go, Node.js, TypeScript | A versão que o BOM da release corrente declara certificada |
| **Ferramenta de geração** | A CLI Buf e cada plugin, por versão **e** revisão | O pin exato declarado, sem faixa (`BUF-06`, FND-05) |

### §3.3 Matriz de compatibilidade por sujeito

`normativo` `GOV-11` — **A compatibilidade de cada sujeito é a declarada na matriz
abaixo.** Nenhuma outra leitura de «N e N−1» é conforme.

| Sujeito | Direção da compatibilidade | Piso obrigatório | Classe de criticidade | Autoridade que decide | Evidência |
|---------|----------------------------|------------------|-----------------------|-----------------------|-----------|
| **Produto DMPF** | Do consumidor para o produto: uma aplicação que adota a release `N` não é obrigada a migrar no mesmo ciclo | `N` e `N−1` publicadas simultaneamente | `crítica` por definição — é o sujeito que carrega os demais | Arquitetura, com Plataforma | Dois BOMs vigentes no repositório canônico |
| **Kernel** | Do código de aplicação para o kernel: código escrito contra `N−1` compila e executa contra `N` sem alteração | `N` e `N−1` | Declarada por release na matriz do BOM | Arquitetura | Suite de compatibilidade do kernel, executada contra as duas majors |
| **Contrato de wire** | — | — | — | **FND-05** | `PTB-08` a `PTB-16`; **este artefato não redefine** |
| **Runtime certificado** | Do kernel para o runtime: o kernel suporta `N` e `N−1` do runtime | `N` e `N−1` | `crítica` para runtime em suporte upstream; `padrão` fora dele | Plataforma | Combinação certificada no BOM, com evidência de execução (§4.4) |
| **Ferramenta de geração** | Nenhuma: o pin é exato | `N` apenas — **não há `N−1`** | — | Plataforma, no repositório de contratos | `BUF-06`: pin de versão e revisão em arquivo versionado |

`normativo` `GOV-12` — **A ferramenta de geração é a exceção declarada ao piso `N` e
`N−1`.** `BUF-06` do FND-05 exige pin exato de CLI e de cada plugin, por versão e
revisão, e trata pin por faixa como não conforme. Admitir `N−1` de plugin aqui
reintroduziria a irreprodutibilidade que aquela regra fecha. Duas versões de plugin
em uso simultâneo não são compatibilidade — são dois artefatos gerados diferentes
para o mesmo contrato.

`normativo` `GOV-13` — **Classe de criticidade é declarada, não inferida.** Um
sujeito cuja classe não esteja declarada na matriz do BOM da release corrente é
tratado como `crítica`: o piso mais estrito é o default. A classe governa o que
excede o piso, nunca o que fica abaixo dele — `padrão` não autoriza suportar menos
que `N` e `N−1`.

`rationale` — O default `crítica` inverte a conveniência de propósito. O erro
provável, sob pressão de entrega, é deixar a classe em branco e resolver a
ambiguidade depois, na primeira vez que alguém quiser remover `N−1`. Com o default
no lado permissivo, a omissão viraria licença; com ele no lado estrito, a omissão
custa a quem quer remover, que é quem tem o incentivo de declarar.

### §3.4 SemVer, rito de mudança e release train

`normativo` `GOV-14` — **Produto e kernels seguem Semantic Versioning.** Major para
mudança incompatível, minor para adição compatível, patch para correção compatível.
O que constitui incompatibilidade do kernel é a quebra da direção declarada em
`GOV-11`: código de aplicação escrito contra a major anterior deixar de compilar ou
de executar.

`normativo` `GOV-15` — **Mudança normativa do DMPF passa por RFC antes de virar
norma.** A norma do DMPF vive na RFC e nos artefatos que a ela se adicionam por
âncora; alterá-la segue RFC §14.2, e não o release train do produto. Publicar uma
release não promove nem revoga norma, e uma release não é o instrumento para
introduzir regra que a RFC não tenha.

`recepcionado` — RFC §14.2 fixa a tabela de tipos de mudança e o incremento
correspondente: adição por âncora dentro do escopo é revisão em PR sem incremento;
regra normativa nova exige ADR e incremento menor; relaxamento de constraint P0
exige ADR aceito, aprovação das quatro áreas e incremento maior.

`normativo` `GOV-16` — **O release train tem periodicidade declarada, e a
periodicidade concreta é `definido` com condição.** O que este artefato fixa é a
forma: a cadência é declarada no BOM da release corrente, é a mesma para produto e
kernels, e a data de corte de cada release é conhecida antes do início do ciclo. O
**valor** da periodicidade permanece `definido` até que exista a primeira release
real — ela acompanha o épico de kernel, e fixar aqui um número sem uma release
publicada seria inventar cadência que ninguém pode cumprir nem medir.

| Campo do release train | Estado | Quem constata |
|------------------------|--------|---------------|
| Existência de cadência declarada e igual para produto e kernels | `vigente` | Revisão de PR do BOM |
| Data de corte conhecida antes do início do ciclo | `vigente` | Revisão de PR do BOM |
| **Valor** da periodicidade | `definido` | Owner do épico de kernel, na primeira release |

`rationale` — A alternativa era fixar um valor plausível — trimestral, por exemplo —
e ajustá-lo depois. Descartada: uma cadência declarada e não cumprida é pior que
uma cadência pendente, porque a primeira dá a aparência de previsibilidade e a
segunda pede a decisão de quem pode tomá-la. O AC-11 item 1 pede que o release train
esteja «definido», e a forma está.

### §3.5 Depreciação de produto, e a fronteira contra as três depreciações alheias

`normativo` `GOV-17` — **Este artefato governa a depreciação do produto:** release
do DMPF, major de kernel, runtime certificado e combinação do BOM. Ele **não**
governa a depreciação de contrato de wire, de código de erro nem do documento RFC.

| Depreciação de | Dona | Regra vigente |
|----------------|------|---------------|
| Contrato de wire | FND-05 | `PTB-13` a `PTB-16`: estado declarado no `.proto`, janela mínima de **180 dias**, remoção só com janela vencida e sem consumidor declarado, e depreciação que **não** relaxa os gates |
| Código de erro | FND-07 | `ERR-16`: depreciação declarada com substituto nomeado |
| Documento RFC | RFC §14 | Versionamento e promoção de status do próprio documento |
| **Produto, kernel, runtime e combinação** | **este artefato** | `GOV-18` a `GOV-21` |

`normativo` `GOV-18` — **Depreciar é um estado declarado no BOM, com data de fim de
suporte e sucessor nomeado.** Aviso em canal de comunicação não deprecia: o
consumidor que lê o BOM tem de ver o estado. A máquina de estados que rege a
transição está em §4.4, e `depreciada` é um dos seus estados.

`normativo` `GOV-19` — **A janela mínima de suporte de uma major depreciada de
produto ou de kernel é de 180 dias**, contados da declaração de `GOV-18`. Durante a
janela, a major depreciada continua publicada e continua recebendo correção de
defeito e de CVE.

`rationale` — O valor de 180 dias é **decisão deste artefato**, não recepção de
`PTB-14`. Os sujeitos são diferentes: `PTB-14` governa a major de um pacote
Protobuf, esta regra governa a major de um kernel e a release de um produto.
Adotar deliberadamente o mesmo piso evita a única alternativa pior, que é ter dois
calendários de depreciação correndo em paralelo sobre o mesmo repositório — um
consumidor que precise migrar de contrato e de kernel ao mesmo tempo teria de
reconciliar duas janelas. A escolha é de alinhamento, e o alinhamento é a
justificativa; nenhuma regra do FND-05 é reaberta, e a evolução independente dos
dois valores continua possível porque as donas são distintas.

`normativo` `GOV-20` — **Remover uma major depreciada exige, cumulativamente:**
janela de `GOV-19` vencida, e ausência de consumidor declarado. A remoção é um commit
próprio, revisável, com a evidência das duas condições no corpo do PR. A declaração
de consumidor segue a mesma lógica de `REP-04` do FND-05: ela é a única forma de
bloquear remoção, e quem não declara não é notificado.

`normativo` `GOV-21` — **Depreciação não relaxa nenhuma regra deste artefato nem da
RFC.** Uma combinação depreciada continua sujeita à matriz de compatibilidade e aos
gates aplicáveis até ser removida. «Está depreciado» não é justificativa para
alteração incompatível nem para conceder escape hatch.

### §3.6 Os dez instrumentos de governança da base conceitual

`recepcionado` — `Parte-1 §17.3` prescreve dez instrumentos, e está **vigente** por
RFC §14.4, que consolida apenas §3, §4, cap. 19 e cap. 20 da Parte-1 — §17 não está
entre os consolidados. Esta subseção os endereça **um a um**. A obrigação O8 do
ledger existe porque a spec recolheu seis e omitiu quatro; omissão silenciosa não
os quita.

| # | Instrumento (`Parte-1 §17.3`) | Tratamento aqui | Estado |
|---|-------------------------------|-----------------|--------|
| 1 | SemVer | `GOV-14` | `vigente` |
| 2 | RFC para mudanças públicas | `GOV-15`, sobre RFC §14.2 | `vigente` |
| 3 | **Owners claros por pacote** | `GOV-22` | `vigente` |
| 4 | Release train previsível | `GOV-16` — forma vigente, valor `definido` | parcial, com condição |
| 5 | Compatibilidade de pelo menos `N` e `N−1` conforme criticidade | `GOV-10` a `GOV-13` | `vigente` |
| 6 | Depreciação com prazo e guia de migração | `GOV-17` a `GOV-21`, e `GOV-23` para o guia | `vigente` |
| 7 | **Codemods e generators junto de breaking changes** | `GOV-23` | `vigente` |
| 8 | **Office hours e canal de suporte** | `GOV-24` — `encaminhado` | `definido` como fronteira |
| 9 | **Métricas de adoção e fricção** | `GOV-25` | `vigente` como instrumento; valores em §6.2 |
| 10 | Escape hatch documentado por ADR | §5.1, `GOV-30`+ | `vigente` |

`normativo` `GOV-22` — **Todo pacote publicado do DMPF tem owner declarado, e o
owner é uma equipe, não uma pessoa.** A declaração vive em `CODEOWNERS` do
repositório que publica o pacote. Pacote publicado sem owner não é conforme: um
artefato público sem dono não tem quem responda pela sua evolução nem pela sua
depreciação.

`normativo` — **Fronteira contra `REP-03`.** `REP-03` do FND-05 obriga owner por
diretório de **bounded context** no repositório de contratos. `GOV-22` obriga owner
por **pacote publicado** do produto. Os sujeitos são distintos e as duas regras
compõem: um repositório de contratos satisfaz `REP-03` e, se publicar pacote,
também `GOV-22`. Nenhuma das duas satisfaz a outra por implicação.

`evidência` — O inventário AS-IS registra `CODEOWNERS` ausente em 10 de 10
repositórios, com `owner não identificado` como valor padrão. `GOV-22` é, portanto,
uma regra que nenhum repositório do acervo satisfaz hoje. Ela obriga o trabalho
novo, e o faseamento da adoção no acervo existente é `ADO-06` em §6.1 — não é
declarada satisfeita por antecipação.

`normativo` `GOV-23` — **Breaking change de kernel é publicada com instrumento de
migração no mesmo release.** O instrumento é, na ordem de preferência: codemod
executável, generator atualizado, ou guia de migração passo a passo. Breaking change
sem nenhum dos três não é conforme. O guia satisfaz o instrumento 6 de `Parte-1
§17.3` («guia de migração») e é o piso; codemod e generator são a forma preferida
porque transferem o custo da migração de cada consumidor para quem introduziu a
quebra.

`encaminhado` `GOV-24` — **Office hours e canal de suporte** são instrumentos de
operação da plataforma, não de norma arquitetural: eles exigem alocação de pessoas
e um canal mantido, e nenhum dos dois é decidível por documento. Dona: **owner do
épico 5 de ARQ-436 §20** («Piloto, Estabilização e Adoção»), cujo objetivo literal
é «validar com squads reais, remover atritos e preparar a versão 1.0» — o canal é
instrumento direto de remoção de atrito. Condição para virar norma: o épico declarar
o canal e a cadência das office hours. Até então este instrumento está `definido`
como fronteira, e não é contado como satisfeito no AC-11.

`normativo` `GOV-25` — **Métricas de adoção e fricção são instrumento declarado do
produto, e as suas nove instâncias são as do catálogo de `ARQ-436 §16.2`.** Este
artefato fixa que o produto mede adoção e fricção, e que a matriz de §6.2 é o
instrumento; ele não fixa valores, porque valor de métrica é medição e não norma.
A distinção entre o instrumento (vigente) e os valores (ausentes, com owner e prazo)
é a de `GOV-03`.

---

## §4. BOM e certificação

`normativo` — Toda esta seção opera sob **ANC-09**, no escopo permitido «matriz de
versões certificadas». IDs `BOM-01`+.

O BOM é o instrumento que a mitigação do risco §15 do épico nomeia: em vez de
congelar versões no documento arquitetural, o documento fixa o **modelo** e o BOM
publica os **valores**, por release. Esta seção define o conteúdo, o schema, a
autoridade e — o que a spec não tinha — o que faz de uma combinação uma combinação
**certificada** em vez de uma combinação declarada.

### §4.1 Os seis itens canônicos

`recepcionado` — `Parte-1 §17.2` prescreve o conteúdo do BOM, publicado **por
release**: versões de Go, Node.js e TypeScript; versões de generators Protobuf e
OpenAPI; drivers, clients e SDKs certificados; combinações suportadas; datas de
depreciação; CVEs e correções relevantes.

`normativo` `BOM-01` — **Um BOM que omita qualquer um dos seis itens não é
conforme.** Um item sem instância na release corrente é declarado explicitamente
vazio, com a razão. Ausência de linha e ausência declarada não são a mesma coisa: a
primeira é omissão, a segunda é informação.

`normativo` `BOM-02` — **Cada release do produto tem exatamente um BOM, e o BOM é
versionado no repositório canônico.** BOM em wiki, planilha ou canal de comunicação
não é conforme: o BOM é revisável por PR, como qualquer norma, e o seu histórico é
o histórico do repositório.

### §4.2 Schema do BOM

`normativo` `BOM-03` — **Toda entrada do BOM declara os campos abaixo.** Campo
ausente reprova a entrada; campo com valor desconhecido é declarado como tal, nunca
omitido.

| Campo | Obrigatório | Significado |
|-------|-------------|-------------|
| `subject` | sim | Um dos cinco sujeitos de `GOV-10`, ou `driver`, `client`, `sdk` |
| `identity` | sim | Coordenada exata: módulo, pacote, imagem ou binário, com o namespace que o identifica sem ambiguidade |
| `version` | sim | Versão exata. Faixa não é valor de `version` — a faixa é a matriz de compatibilidade, e ela é outro campo |
| `state` | sim | Um dos cinco estados de `BOM-07` |
| `criticality` | sim | `crítica` ou `padrão`, conforme `GOV-13`; ausente é lido como `crítica` |
| `compatible_with` | sim | As versões dos demais sujeitos com que esta foi exercitada em conjunto |
| `evidence_uri` | quando `state` é `certificada` | Endereço estável do resultado da verificação de `BOM-07` |
| `evidence_digest` | quando `state` é `certificada` | Digest do artefato de evidência, que torna a evidência não substituível em silêncio |
| `approved_by` | quando `state` é `certificada` | Autoridade de `BOM-05` que aprovou a promoção |
| `certified_at` | quando `state` é `certificada` | Data da promoção |
| `valid_until` | quando `state` é `certificada` | Fim da validade da certificação (`BOM-08`) |
| `deprecated_at` | quando `state` é `depreciada` | Data da declaração, que inicia a janela de `GOV-19` |
| `successor` | quando `state` é `depreciada` | A entrada que sucede esta |
| `cve` | sim, ainda que vazio | CVEs conhecidas e o estado de cada uma: corrigida, mitigada ou aberta com owner |

`normativo` `BOM-04` — **`compatible_with` declara combinação exercitada, não
combinação presumida.** Uma entrada que liste em `compatible_with` uma versão contra
a qual a verificação de `BOM-07` não foi executada é defeito de evidência. É esta
regra que dá conteúdo à decisão da spec — «BOM por combinação certificada, não por
faixa de versão» —, e o motivo é o que a própria spec registra: a combinação
específica que quebra é justamente a que nunca foi exercitada.

### §4.3 Autoridade, e o que o BOM referencia sem duplicar

`normativo` `BOM-05` — **A autoridade que promove uma entrada a `certificada` é
Plataforma, com revisão de Arquitetura.** A promoção é um PR no repositório
canônico, e a aprovação de quem promove não substitui a evidência: `BOM-07` é
condição, não formalidade.

`normativo` `BOM-06` — **O BOM referencia o registro autoritativo de cada domínio
alheio; ele não o duplica.** Duplicar valor normativo cria duas fontes que divergem
no primeiro update de uma delas.

| Matéria | Registro autoritativo | O que o BOM faz |
|---------|-----------------------|-----------------|
| Stacks com toolchain certificada | `BUF-07` do FND-05, e o registro que vive no repositório de contratos junto da configuração de geração | Referencia a coordenada do registro; não replica a lista de stacks |
| Predicados de certificação de stack | `BUF-07`: suporte a `google.protobuf.Any` (`ENV-15`) e preservação de campos desconhecidos (`PTB-10`) | Referencia; não reenuncia os predicados |
| Pins de CLI Buf e de plugins | `BUF-06` do FND-05, em arquivo versionado no repositório de contratos | Referencia o arquivo e a sua coordenada; não copia os números |
| Dependências externas de contrato | `BUF-02`: declaradas em `deps`, com `buf.lock` versionado | Referencia |
| Baseline de telemetria e limiares | Catálogo do FND-08 | Referencia |

`rationale` — A regra é a resposta à obrigação O9, e o erro que ela evita é
concreto: replicar no BOM a lista de stacks certificadas do FND-05 produziria, no
primeiro momento em que uma stack fosse recertificada, um BOM que declara conforme
o que o registro autoritativo já rejeita. A referência por coordenada tem custo — o
leitor precisa de dois documentos — e é o custo certo.

### §4.4 A máquina de estados da certificação

`normativo` `BOM-07` — **Uma entrada do BOM percorre exatamente estes cinco
estados**, e nenhuma transição ocorre por omissão ou por decurso de prazo sem ato
declarado:

```text
proposta ──► candidata ──► certificada ──► depreciada ──► não suportada
   │             │                              ▲
   └─────────────┴──────────────────────────────┘
        rejeitada volta a proposta ou é removida
```

| Transição | Condição cumulativa | Evidência exigida |
|-----------|---------------------|-------------------|
| `proposta` → `candidata` | Coordenada exata declarada (`identity`, `version`), sujeito e criticidade declarados | Entrada no BOM com os campos de `BOM-03` que não dependem de execução |
| `candidata` → `certificada` | Suite e fixtures nomeadas; execução concluída; resultado aprovado; `compatible_with` exercitado; aprovador de `BOM-05`; validade fixada | `evidence_uri` **e** `evidence_digest`, `approved_by`, `certified_at`, `valid_until` |
| `certificada` → `depreciada` | Declaração de `GOV-18` com sucessor nomeado | `deprecated_at`, `successor` |
| `depreciada` → `não suportada` | Janela de `GOV-19` vencida **e** ausência de consumidor declarado | Commit próprio com a evidência das duas condições (`GOV-20`) |
| qualquer → `proposta` | Recertificação após troca de biblioteca ou vencimento de validade | Nova execução; a evidência anterior não é reaproveitada |

`normativo` `BOM-08` — **Certificação tem validade declarada, e certificação vencida
não é certificação.** Uma entrada cujo `valid_until` esteja no passado volta a
`candidata` até nova execução. Sem validade, «certificada» significaria «foi
exercitada uma vez, em data desconhecida, contra versões que talvez já não existam».

`normativo` `BOM-09` — **CVE aberta em entrada `certificada` exige owner nomeado e
não suspende a certificação por si.** O que a CVE aberta suspende é a promoção de
entradas novas que dependam dela. Suspender automaticamente a certificação vigente
transferiria para o BOM uma decisão de risco que é de Segurança, e deixaria o
consumidor sem combinação suportada no momento de maior exposição.

`normativo` `BOM-10` — **O BOM tem um slot nomeado para a versão das *semantic
conventions* de mensageria do OpenTelemetry, e o slot tem owner.** `TRC-03` do
FND-08 adota a convenção e remete a **versão** ao BOM, e a advertência que motiva a
remessa é que as convenções de mensageria ainda evoluem. O slot é um `subject` do
BOM como qualquer outro, sujeito a `BOM-03` e à máquina de estados.

| Campo | Valor |
|-------|-------|
| `subject` | `semantic_conventions_messaging` |
| Owner do slot | Plataforma |
| Estado na promoção deste artefato | `proposta` — sem valor, porque não há release publicada |
| Owner da **primeira instância real** | Owner do épico de kernel que publicar a primeira release (épico 1 ou 2 de `ARQ-436 §20`) |
| Condição para `certificada` | Versão fixada e exercitada pela suite do kernel, com `evidence_uri` e `evidence_digest` |

`normativo` — **O slot vazio não quita a obrigação O3.** O que este artefato entrega
é o slot, o owner e o critério de promoção; o valor acompanha a primeira release. A
obrigação permanece registrada em §2.1 como `atendida com pendência de titular`, e
não como `atendida` sem qualificação, precisamente porque a diferença é verificável:
um BOM publicado com esse `subject` em `proposta` é conforme hoje e deixa de ser
quando existir release.

---

## §5. Controles de governança

Esta seção define **dois ritos distintos**, sob âncoras distintas, e a razão de
estarem juntos é precisamente que confundi-los é o modo de falha a evitar:

| | §5.1 — escape hatch | §5.2 — autorização da classificação |
|---|---------------------|-------------------------------------|
| Âncora | ANC-09 | **ANC-08** |
| O que regula | Divergir do golden path **mantendo** a classificação | Alterar a **classificação** de uma unidade |
| Pergunta que responde | «esta unidade pode, excepcionalmente, depender daquilo?» | «esta unidade pode passar a ser outra coisa?» |
| Instrumento | Exceção nominal com prazo | Autorização distinta da autoria |
| Prefixo | `GOV-30`+ | `AUT-01`+ |

`normativo` `GOV-26` — **Nenhum dos dois ritos satisfaz o outro, e nenhum dos dois
produz o efeito do outro.** Obter escape hatch não autoriza reclassificar, e
reclassificar não dispensa escape hatch para a dependência que a classificação nova
passe a permitir. `GOV-32` e `AUT-08` fecham os dois sentidos.

`rationale` — O modo de falha é assimétrico e vale nomeá-lo. Reclassificar é o
caminho **mais barato** para o mesmo efeito de um escape hatch: em vez de pedir
exceção nominal com prazo e plano de convergência, basta declarar que a unidade
pertence a outro bloco, e a dependência antes proibida passa a ser permitida por
política, sem exceção alguma a registrar. É o risco R1 da RFC — reclassificação
oportunista — visto do lado da governança de produto. Um artefato que definisse os
dois ritos sem os amarrar entregaria a porta dos fundos junto com a fechadura da
frente.

### §5.1 Escape hatch

`normativo` — Sob **ANC-09**. IDs `GOV-30`+.

`normativo` `GOV-30` — **Divergir do golden path é permitido e rastreado. A
concessão exige quatro itens, cumulativamente:**

| # | Item | Conteúdo mínimo |
|---|------|-----------------|
| 1 | **ADR** | Registro da decisão, com a alternativa considerada e descartada |
| 2 | **Owner** | Equipe nomeada, não pessoa — a mesma disciplina de `REP-03` e `GOV-22` |
| 3 | **Justificativa técnica** | O que o golden path não resolve neste caso |
| 4 | **Plano de convergência com prazo** | Data e condição do retorno ao golden path |

`normativo` — **Faltando qualquer um dos quatro, a exceção é negada.** A spec
endurece o AC-11 item 3, que pede plano de convergência «quando aplicável»: aqui,
**exceção sem prazo é negada**, sem a condicional. Esse endurecimento é `M1`
(restringir dentro do escopo da âncora) e é legítimo por RFC §12.2.

`rationale` — Por que a condicional foi removida. «Quando aplicável» é
autoavaliado por quem pede a exceção, e quem pede tem incentivo para concluir que
não se aplica. O resultado previsível é a exceção permanente: sem prazo, ela deixa
de ser desvio e passa a ser um segundo padrão, não revisado, que a fundação não
governa. O custo do endurecimento é real — haverá casos em que a convergência
genuinamente não é planejável — e o tratamento desses casos é `GOV-34`, que admite
prazo de **revisão** em vez de prazo de convergência, com autoridade mais alta. O que
não se admite é ausência de data.

`normativo` `GOV-31` — **Universo positivo: o que é excepcionável.** Uma exceção só
pode incidir sobre um destes três objetos, e o pedido declara qual:

| # | Objeto excepcionável | Exemplo do que a exceção autoriza |
|---|----------------------|-----------------------------------|
| E1 | **Dependência externa** de uma unidade sobre pacote não certificado ou sobre capability não permitida ao bloco por allowlist de entrypoint | Uma unidade `app` depender de SDK ainda não certificado no BOM |
| E2 | **Combinação fora do BOM** — versão ou par de versões que não está `certificada` | Um serviço em runtime uma minor à frente do certificado, durante migração |
| E3 | **Instrumento de governança deste artefato**, quando o instrumento admitir exceção no próprio texto | Prazo de `GOV-19` estendido para um consumidor nomeado |

`normativo` `GOV-32` — **Regra de negação: o que nenhuma exceção pode alcançar.** O
pedido que incida sobre qualquer item abaixo é **negado na admissão**, sem exame de
mérito, e a negação não é recorrível por este rito:

| # | Objeto **não** excepcionável | Fundamento |
|---|------------------------------|------------|
| N1 | Qualquer **constraint P0** | ANC-09, invariante literal: «escape hatch não pode contornar constraint P0»; e RFC §12.2 M3, que exige nova versão da RFC com ADR aceito e aprovação das quatro áreas |
| N2 | Qualquer **célula da regra de dependência de RFC §7** — incluindo o resultado da função `decide` de §7.1 | RFC §12.2 M3 |
| N3 | Qualquer **invariante listada numa âncora** de RFC §12.3 | RFC §12.2 M2 |
| N4 | Os requisitos **T1 a T6** de RFC §10.2 | Invariante de ANC-08; e §5.2 é o rito próprio, não substituível por este |
| N5 | A **classificação** de uma unidade — `block` ou `bounded_context` | `GOV-26`; o rito é §5.2 |
| N6 | O **pinning exato** de CLI e plugin de geração (`BUF-06`) | `GOV-12`: pin por faixa não é conforme, e exceção aqui reintroduziria a irreprodutibilidade |
| N7 | A exigência de **evidência** de uma certificação (`BOM-07`) | Exceção sobre evidência esvazia a certificação: o que restaria seria a declaração |

`normativo` — **A via para o efeito de N1 e N2 é única**: nova versão da RFC com ADR
aceito, pelo rito de RFC §14.2. O escape hatch não é caminho alternativo, nem
temporário, para o mesmo efeito. Uma exceção «por 90 dias» a uma constraint P0 é
uma constraint P0 relaxada por 90 dias, e M3 não distingue relaxamento permanente de
relaxamento temporário.

`rationale` — O universo positivo de `GOV-31` existe porque os quatro requisitos de
`GOV-30`, sozinhos, não bastam. Um pedido pode trazer ADR, owner, justificativa e
prazo — os quatro em ordem — e ainda assim pedir o que N1 proíbe. Sem o universo
positivo, o avaliador teria de deduzir a negação da ausência de autorização, e
dedução de ausência é exatamente o que não sobrevive à pressão de um caso urgente. A
alternativa avaliada foi listar apenas as negações; foi descartada porque uma lista
só de negações é sempre incompleta contra um pedido criativo, enquanto um universo
positivo fechado é completo por construção: o que não está em E1, E2 ou E3 não é
excepcionável, ainda que ninguém tenha pensado nele.

`normativo` `GOV-33` — **A exceção é nominal.** Ela identifica o par exato
`(unidade, objeto excepcionado)`, com a coordenada da unidade e a identidade do
objeto. Exceção por categoria, por prefixo de pacote, por diretório ou por equipe é
**proibida**.

`recepcionado` — Esta é a mesma disciplina de RFC §6.4, que fixa a exceção nominal
para política de bloco: «Exceção por categoria, por prefixo de pacote ou por
diretório é proibida — ela deixa de ser exceção e vira política paralela não
revisada». O schema de §10.1 já reserva o campo `exceptions`, com `unit`,
`dependency`, `reason`, `owner` e `review_by`.

`normativo` — **Composição com RFC §6.4, declarada.** As duas regras compõem e não
se sobrepõem: RFC §6.4 governa a **forma** de uma exceção à política de bloco e o seu
registro no `metadata_container`; `GOV-30` a `GOV-35` governam o **rito de
concessão** — quem concede, contra qual universo, com qual prazo e sob qual revisão.
Uma exceção do tipo E1 satisfaz as duas: a forma de RFC §6.4 e o rito daqui. Nenhuma
das duas dispensa a outra, e onde o objeto for E2 ou E3 — que não são dependência de
unidade — o registro vive no BOM, não no `metadata_container`.

`normativo` `GOV-34` — **Ciclo de vida da exceção.** Toda exceção concedida tem os
cinco campos abaixo, e o vencimento tem efeito declarado:

| Campo | Regra |
|-------|-------|
| **Vigência** | Data de início e data de fim. Sem data de fim, a exceção é inválida (`GOV-30` item 4) |
| **Revisão** | Data de revisão obrigatória, não superior ao fim da vigência |
| **Renovação** | Renovar é conceder de novo: passa pelo mesmo rito, com o mesmo universo de `GOV-31`, e exige que o plano de convergência seja **atualizado** com o motivo do atraso. Renovação automática não existe |
| **Revogação** | A autoridade concedente revoga quando a justificativa técnica deixa de valer, sem esperar o vencimento |
| **Efeito do vencimento** | No vencimento sem renovação, a exceção **cessa** e a unidade passa a estar não conforme. O vencimento não prorroga por omissão e não converte a exceção em política |

`normativo` — **Exceção sem convergência planejável.** Quando a convergência
genuinamente não é planejável — o golden path não cobre o caso e não há previsão de
que passe a cobrir —, admite-se prazo de **revisão** no lugar de prazo de
convergência, com duas condições cumulativas: a concessão é aprovada por Arquitetura
**e** Plataforma, e o pedido declara o que faria a convergência voltar a ser
planejável. O que não se admite é ausência de data: uma exceção sem nenhuma das duas
datas é negada por `GOV-30`.

`normativo` `GOV-35` — **A evidência da concessão é persistida e endereçável.** O
registro vive no repositório — no `metadata_container` para E1, no BOM para E2 e E3 —
e o ADR de `GOV-30` item 1 é referenciado por identificador. Concessão registrada
apenas em canal de comunicação, ticket ou ata não é conforme: a exceção precisa ser
descobrível por quem lê o repositório, e não apenas por quem participou da conversa.

`normativo` `GOV-36` — **Métrica de saúde do instrumento.** O número de exceções
vigentes, o número de renovações por exceção e o número de exceções vencidas sem
convergência são publicados junto do BOM da release. O instrumento é `vigente`; os
valores acompanham a primeira release, como em `GOV-25`.

`rationale` — A métrica existe porque escape hatch é o instrumento cuja degeneração
é silenciosa. Uma exceção é sempre defensável isoladamente; o que não é defensável é
o agregado — trinta exceções vigentes, metade renovada duas vezes, significa que o
golden path não cobre o caso comum, e isso é informação sobre a fundação, não sobre
as equipes. Sem contagem publicada, essa informação só aparece quando alguém decide
auditar.

### §5.2 Autorização da classificação

`normativo` — Sob **ANC-08**. IDs `AUT-01`+. Escopo permitido, literal: «Definir
autoridade, rito e artefato de evidência que satisfaçam T4–T6 de §10.2».

`recepcionado` — RFC §10.2 fixa os requisitos, e eles **permanecem** por invariante
da âncora. Nada nesta subseção os altera:

| # | Requisito de RFC §10.2 |
|---|------------------------|
| T1 | O manifesto é a **fonte canônica** da classificação. O baseline é uma cópia independente, mantida fora do `ownership_module` que descreve |
| T2 | O baseline cobre `block` **e** `bounded_context` de cada `canonical_key`, mais um digest do conjunto |
| T3 | Divergência entre manifesto e baseline **reprova** |
| T4 | Alteração de `block` ou de `bounded_context` de uma unidade existente é **mudança normativa** e exige autorização distinta da autoria da mudança |
| T5 | Ausência de evidência de autorização em uma mudança que satisfaça T4 **reprova** — fail-closed |
| T6 | Criação de unidade nova e remoção de unidade seguem T4 |

`recepcionado` — O problema que os requisitos endereçam, nas palavras da RFC: «o
metadado é **auto-declarado**. A unidade declara a classificação que deveria
restringi-la», e comparar manifesto com baseline «não impede que o autor altere os
dois no mesmo commit, deixando a comparação verde e liberando imports antes
proibidos».

`evidência` — O contexto que a RFC registra e que condiciona todo o desenho desta
subseção: `CODEOWNERS` está ausente em **10 de 10** repositórios inventariados, com
`owner não identificado` como valor padrão. Não existe hoje autoridade de aprovação
instalada sobre a qual apoiar o rito. É por isso que `AUT-02` atribui uma **função**
e `AUT-07` separa a função do seu titular.

#### O ato regulado

`normativo` `AUT-01` — **O rito desta subseção é disparado por qualquer um dos
quatro atos abaixo**, e por nenhum outro:

| # | Ato | Fundamento |
|---|-----|------------|
| A1 | Alterar o `block` de uma `verification_unit` existente | T4 |
| A2 | Alterar o `bounded_context` de uma `verification_unit` existente | T4 |
| A3 | Criar unidade nova | T6 |
| A4 | Remover unidade existente | T6 |

`normativo` — **O ato regulado é o delta efetivo `arquivo → (canonical_key, block,
bounded_context)`, não a edição de um campo.** Qualquer alteração de `include`, de
root ou de caminho que mude o `block` **ou** o `bounded_context` efetivo de um trecho
de código dispara A1 ou A2, ainda que nenhum campo `block` ou `bounded_context` seja
tocado e nenhuma unidade seja criada ou removida. Alterar `include` sem mudar nenhum
dos dois efetivos não dispara este rito.

| Movimento de código por `include` | Efetivo que muda | Ato |
|-----------------------------------|------------------|-----|
| De `src/domain/**` para o `include` de uma unidade `app` | `block` | **A1** |
| Entre duas unidades de **`bounded_context` distintos**, ainda que ambas do mesmo `block` | `bounded_context` | **A2** |
| Entre duas unidades do mesmo `block` **e** do mesmo `bounded_context` | nenhum | não dispara |

`rationale` — Sem esta regra, o rito seria evitável por construção: bastaria não
tocar em `block` nem em `bounded_context` e obter o mesmo efeito reorganizando
`include`. **A segunda linha da tabela é a que fecha o caminho menos óbvio, e ela
foi acrescentada por revisão adversarial** — a formulação anterior desta regra falava
apenas de «bloco efetivo» e deixava passar o movimento entre contextos. O caminho
concreto que ela agora barra: duas unidades ambas `domain`, em `bounded_context`
distintos `A` e `B`; um arquivo de `B` importa domínio de `A`, o que é aresta
inter-context; o commit altera apenas os `include`, atribuindo esse arquivo à unidade
de `A`, e atualiza manifesto e baseline juntos. Nenhum campo `block` ou
`bounded_context` muda de valor, nenhuma unidade nasce ou morre, e o `block` efetivo
continua `domain` — mas a aresta que RFC §7.1 reprovava por **C2**
(`same_bounded_context` **ou** `public_integration_surface(destino)`), emitindo
`DMPF-D002`, passa a ser permitida sem que T4 tenha sido acionado.

`recepcionado` — O fundamento de que o ato existe está na própria RFC: a
`verification_unit` é «o menor conjunto **de código** a que uma classificação de
bloco se aplica integralmente», e em TypeScript esse conjunto é determinado pelos
roots declarados. Reatribuir o código entre unidades é, portanto, reclassificá-lo. A
RFC prevê o caso adjacente em T3, ao exigir que o baseline cubra `block` **e**
`bounded_context` de cada `canonical_key` com digest do conjunto — o digest é o que
torna a mudança de composição detectável. Esta regra declara a consequência normativa
daquela detecção.

#### A autoridade

`normativo` `AUT-02` — **A autoridade aprovadora é uma função organizacional
fechada, não uma pessoa e não uma equipe genérica.** A função é
**Autoridade de Classificação Arquitetural**, com estas propriedades:

| Propriedade | Valor |
|-------------|-------|
| Natureza | Função organizacional, provida por titular nomeado |
| Área responsável pela indicação | Arquitetura |
| Revisão da indicação | Segurança — a função é controle de integridade, e o risco que ela mitiga é adversarial |
| Escopo de competência | Exclusivamente os quatro atos de `AUT-01` |
| Impedimento | O titular não autoriza mudança de cuja autoria participou (`AUT-04`) |
| Delegação | Admitida para suplente nomeado, pelo mesmo rito de indicação. Delegação genérica ou tácita não existe |

`normativo` `AUT-03` — **O rito.** Uma mudança que satisfaça `AUT-01` é apresentada
e aprovada assim, cumulativamente:

| # | Passo | Condição |
|---|-------|----------|
| R1 | **Commit próprio**, separado de mudança de código | O commit contém a alteração do manifesto e do baseline, e nada mais |
| R2 | **Declaração do ato** no corpo do PR: qual dos quatro atos de `AUT-01`, a `canonical_key` afetada, o valor anterior e o novo | Ato não declarado reprova |
| R3 | **Justificativa** de por que a classificação anterior estava errada, ou o que mudou no código que a torna incorreta | Justificativa que se limite a descrever a mudança não satisfaz R3 |
| R4 | **Aprovação da Autoridade de Classificação Arquitetural**, distinta da autoria | `AUT-04` |
| R5 | **Enumeração das arestas que a reclassificação passa a permitir** e que antes eram proibidas pela função `decide` de RFC §7.1 | Enumeração ausente reprova: é o que torna o efeito da mudança visível |

`normativo` `AUT-04` — **Autor e autorizador coincidentes reprovam, e a aprovação
exige mais de um aprovador.** São duas exigências distintas: a distinção de pessoa
(T4, literal) e a pluralidade de aprovadores. A segunda vem do mesmo princípio que
o rito de bootstrap do FND-05 aplica, e existe porque um aprovador único é ponto
único de falha num controle cuja razão de ser é adversarial.

| Configuração | Resultado |
|--------------|-----------|
| Autor aprova a própria mudança | **Reprova** — T4 |
| Um aprovador, distinto do autor, sem ser a Autoridade | **Reprova** — R4 |
| A Autoridade aprova, e é o autor | **Reprova** — impedimento de `AUT-02` |
| A Autoridade aprova, distinta do autor, sem segundo aprovador | **Reprova** — `AUT-04` |
| A Autoridade mais um segundo aprovador, ambos distintos do autor | **Aprova** |

`rationale` — A pluralidade tem custo de fricção e ele é assumido deliberadamente.
A RFC já declara que T1–T6 deixam R1 apenas **parcialmente** mitigado, porque «um
aprovador desatento, ou conluio entre autor e aprovador, continua suficiente». Exigir
dois aprovadores não elimina o conluio — eleva o seu custo de duas pessoas para três.
É o máximo que um rito pode fazer contra conluio, e este artefato não promete mais
do que isso, pelo mesmo princípio que levou a RFC a não prometer.

#### O artefato de evidência

`normativo` `AUT-05` — **A evidência de autorização é o registro abaixo, persistido
no repositório e endereçável.** Aprovação existente apenas na interface da forge, sem
registro persistido, satisfaz T4 no momento da revisão e deixa de ser verificável
depois — o que não satisfaz T5, cuja verificação é posterior ao merge.

| Campo | Conteúdo |
|-------|----------|
| `act` | Um dos quatro de `AUT-01` |
| `canonical_key` | A unidade afetada |
| `from` / `to` | Valor anterior e novo de `block` e `bounded_context` |
| `moved_paths` | Quando o ato é remapeamento por `include`, root ou caminho: os caminhos movidos, com a `canonical_key` de **origem** e de **destino** de cada um. Sem este campo, um remapeamento é indistinguível de uma edição de campo na auditoria posterior |
| `justification` | O texto de R3 |
| `newly_permitted_edges` | A enumeração de R5 |
| `authority` | A função, e o titular que a exerceu |
| `approvers` | Os aprovadores, distintos do autor |
| `author` | A autoria da mudança |
| `commit` | O commit próprio de R1 |
| `authorized_at` | Data |

`normativo` `AUT-06` — **Fail-closed, e o que significa «não verificada».**
Ausência de evidência de autorização numa mudança que satisfaça `AUT-01` **reprova**
(T5). Um verificador que não consiga avaliar a condição reporta **não verificada**,
nunca conforme — a RFC já o fixa para o mecanismo mínimo, e a regra vale igualmente
para o rito definido aqui. «Não verificada» é resultado de reprovação para efeito de
merge: ela não é tratada como aprovação provisória.

#### Titular, estado e a transição de vigência

`normativo` `AUT-07` — **A função é definida por este artefato; o titular não.** O
registro abaixo separa os dois, e o campo `titular aceito` **não** é preenchido por
este documento:

| Campo | Valor na promoção deste artefato |
|-------|----------------------------------|
| `autoridade normativa proposta` | Autoridade de Classificação Arquitetural, com as propriedades de `AUT-02` |
| `área indicante` | Arquitetura |
| `área revisora` | Segurança |
| `titular aceito` | **ausente** — `gate externo` **G2** |
| `evidência de aceite` | ausente |
| `suplente` | ausente |
| `estado` | `definido` |

`normativo` — **Este artefato não nomeia pessoa nem declara aceite.** Nomear titular
por documento seria atribuir responsabilidade sem consentimento, e a evidência de
que não há estrutura instalada é factual (`CODEOWNERS` em 0 de 10). O que a norma
entrega é a função, as suas propriedades, o rito que ela opera e o critério de
suficiência; o aceite é ato de pessoa.

`normativo` `AUT-08` — **Vetores negativos.** Cada linha é um pedido ou uma
tentativa que o rito **rejeita**, com o fundamento:

| # | Tentativa | Resultado | Fundamento |
|---|-----------|-----------|------------|
| V1 | Alterar `block` e baseline no mesmo commit, junto de mudança de código | Reprova | R1 |
| V2 | Alterar `block` com aprovação do próprio autor | Reprova | T4, `AUT-04` |
| V3 | Alterar `include` para mudar o **bloco** efetivo do código, sem tocar em `block` | Reprova | `AUT-01`, delta efetivo, linha 1 da tabela |
| V3a | Remapear código por `include` entre duas unidades **ambas `domain`** de `bounded_context` distintos, liberando aresta que C2 reprovava, sem tocar em nenhum campo | Reprova | `AUT-01`, delta efetivo, linha 2 — é A2, e exige o rito completo mais `moved_paths` em `AUT-05` |
| V4 | Pedir **escape hatch** para obter a dependência que a reclassificação permitiria | Negado na admissão | `GOV-32` N5, `GOV-26` |
| V5 | **Reclassificar** para dispensar escape hatch que seria necessário | Passa pelo rito, e a manobra fica visível | Espelho de V4: o ato é `AUT-01`, então o rito se aplica; R5 obriga a enumerar as arestas que a reclassificação libera, e `GOV-26` impede que o resultado dispense o escape hatch que a classificação nova ainda exija |
| V6 | Reclassificar com justificativa que apenas descreve a mudança | Reprova | R3 |
| V7 | Reclassificar sem enumerar as arestas liberadas | Reprova | R5 |
| V8 | Aprovar sem registro persistido, apenas na forge | Reprova | `AUT-05` |
| V9 | Invocar o mecanismo mínimo de RFC §10.2 para contornar `AUT-04` depois de `AUT-09` satisfeita | Reprova | O mecanismo mínimo é o piso enquanto o processo não vige; satisfeita `AUT-09`, ele deixa de ser alternativa disponível |
| V10 | Tratar «não verificada» como aprovação provisória para desbloquear merge | Reprova | `AUT-06`, T5 |

`normativo` `AUT-09` — **Transição de `definido` para `vigente`.** O processo desta
subseção passa a viger quando, cumulativamente:

1. a função de `AUT-02` tiver **titular aceito**, com a evidência de aceite
   registrada em `AUT-07` (gate **G2**);
2. Segurança tiver revisado a indicação, conforme `AUT-02`; **e**
3. a **ANC-08 estiver fechada** — o que exige, além de G2, a citação do processo em
   RFC §10.2 com o incremento `0.2` (gate **G5**) e a conclusão e revisão formal
   desta sub-spec, conforme a condição de fechamento do registro da âncora.

Quem constata: a área indicante quanto a G2, e o owner da RFC quanto a G5. Até que as
**três** condições sejam satisfeitas, **o mecanismo mínimo de RFC §10.2 permanece
vigente** — commit próprio, separado de mudanças de código, com aprovação por revisor
distinto do autor.

`rationale` — **A terceira condição foi acrescentada por revisão adversarial, e ela
corrige uma violação de M2.** A formulação anterior condicionava a transição apenas a
G2 e à revisão de Segurança. Como §8.4 declara a ANC-08 aberta por **G2 e G5**, havia
uma janela — depois de G2, antes de G5 — em que o rito de `AUT-03` passaria a viger e
o mecanismo mínimo seria desligado **enquanto a âncora continuasse aberta**. Isso
reinterpreta a invariante literal da ANC-08, que diz que o mecanismo mínimo «vale até
o fechamento» e fixa o fechamento em «FND-10 concluída e revisada» — e reinterpretar
invariante de âncora é o que M2 veda. Amarrar a transição ao fechamento efetivo
elimina a janela: enquanto houver gate de fechamento aberto, o piso da RFC vale.

`normativo` — **Enquanto `AUT-09` não é satisfeita, não há duas normas concorrentes.**
A norma aplicável é o mecanismo mínimo; o rito de `AUT-03` está `definido` e não se
aplica. Depois de satisfeita, o rito de `AUT-03` é a norma, e o mecanismo mínimo
deixa de ser alternativa (`AUT-08` V9). Em nenhum momento os dois valem ao mesmo
tempo sobre o mesmo ato.

`rationale` — A leitura ambígua de RFC §10.2 está registrada em §1.3, e é ela que
obriga esta construção. O corpo da subseção diz «até que o FND-10 **defina** o
processo»; o título do bloco diz «até o FND-10 **concluir**»; o registro da âncora
diz «FND-10 concluída e revisada». Adotar a primeira leitura faria o rito de
`AUT-03` viger na promoção deste PR — com uma função sem titular, o que produziria
um controle inoperante no lugar de um controle mínimo operante. A construção adotada
prefere o controle operante: o piso da RFC continua valendo, e a substituição ocorre
quando existir quem a exerça.

`normativo` `AUT-10` — **Alcance do R1, sem promessa excedente.** Com o rito desta
subseção vigente, o risco de reclassificação oportunista fica mitigado além do que
T1–T6 alcançam: o ato passa a exigir autoridade nomeada, dois aprovadores distintos
do autor, enumeração explícita das arestas liberadas e evidência persistida. Ele
**não** fica eliminado: conluio entre autor, autoridade e segundo aprovador continua
suficiente. Este artefato não promete resistência que não tem, pelo mesmo motivo que
a RFC declarou o alcance parcial dela.

---

## §6. Adoção e pilotos

`normativo` — Toda esta seção opera sob **ANC-09**. IDs `ADO-01`+ em §6.1 e
`PIL-01`+ em §6.2.

### §6.1 Adoção e faseamento

`normativo` `ADO-00` — **A força das regras desta subseção está condicionada à
ampliação do escopo da ANC-09, e até então elas valem como especificação
`encaminhada`, não como norma vigente.** O escopo permitido da ANC-09 é fechado em
«Matriz de versões certificadas, escape hatches e seleção de pilotos». Adoção
organizacional, faseamento e plano de migração **não** estão entre os três, e um
encaminhamento feito por sub-spec não amplia o escopo registrado de uma âncora — só a
RFC o faz, pelo rito de RFC §14.2. Por M4, adição fora do escopo é inválida ainda que
tecnicamente correta.

`registro` — **A tensão de autoridade, declarada em vez de resolvida por silêncio.**
Dois artefatos publicados endereçam este assunto nominalmente a FND-10 **sob
ANC-09**: o FND-06 §15 declara que não define «cronograma, faseamento nem ordem de
migração — isso é adoção organizacional, `encaminhada` a ANC-09 / FND-10», e o FND-08
encaminha «cronograma e faseamento da adoção da baseline nos serviços do acervo» ao
mesmo destino. A delegação existe e é dos irmãos; o que falta é escopo na âncora para
recebê-la. É defeito de fronteira da RFC, não invenção desta sub-spec — e resolvê-lo é
mudança de fronteira, que exige nova versão da RFC.

| Item | Estado |
|------|--------|
| Conteúdo de `ADO-01` a `ADO-08` | `definido` — escrito, aprovado em PR, e pronto para viger |
| Força normativa | **suspensa** até a ampliação do escopo da ANC-09 |
| Norma aplicável no intervalo | As regras do FND-06 e do FND-08 que já valem por suas próprias âncoras (`COE-01`..`COE-06`, `TRP-10`, catálogo de sinais) |
| Condição para viger | Ampliação do escopo registrado da ANC-09, ou âncora nova, por RFC §14.2 |
| Dona | **Owner da RFC** — pendência P13 |

`rationale` — A alternativa era manter `ADO-01` a `ADO-08` como `normativo` pleno,
apoiando-se em que os irmãos delegaram o assunto. Foi descartada por revisão
adversarial: se um encaminhamento de sub-spec pudesse ampliar o escopo de uma âncora,
o registro de RFC §12.3 deixaria de recortar, e M4 viraria formalidade — exatamente o
raciocínio que o FND-08 usou em `RES-02` ao mandar o excedente da própria âncora sair
`encaminhado`. A alternativa oposta, apagar §6.1 desta entrega, também foi descartada:
o conteúdo é o que dois irmãos pediram e ele fica escrito e endereçável, apenas com a
força suspensa e a condição nomeada, em vez de perdido.

`normativo` `ADO-01` — **Esta subseção especifica o planejamento da adoção; ela não
executa migração.** A fronteira é declarada porque duas fontes parecem divergir e não
divergem:

| Fonte | Texto | Leitura |
|-------|-------|---------|
| `SPEC-VVR1X71Q`, escopo fora | «Migração dos serviços existentes: pós-fundação» | A **execução** está fora |
| FND-06 §15 | «Ela **não** define cronograma, faseamento nem ordem de migração — isso é adoção organizacional, `encaminhada` a ANC-09 / FND-10» | O **planejamento** está aqui |
| FND-08, matriz de encaminhamentos | «Cronograma e faseamento da adoção da baseline nos serviços do acervo → FND-10, sob ANC-09» | O **planejamento** está aqui |

`normativo` — **A distinção é entre o instrumento e o ato.** Esta subseção fixa os
critérios de decisão, as pré-condições, a ordem de precedência e o rito de corte e
rollback. Ela não fixa datas de calendário, não atribui migração a equipe nomeada e
não executa nada. Datas e alocação pertencem aos épicos de kernel e ao épico 5 de
`ARQ-436 §20`, e um plano de adoção com datas mas sem critério é cronograma, não
governança — a inversão que o `rationale` abaixo trata.

`rationale` — Por que critérios em vez de cronograma. As duas fontes que delegam a
«adoção» a FND-10 pedem, literalmente, «cronograma e faseamento». Entregar um
cronograma aqui seria possível e inútil: nenhuma das datas seria cumprível, porque o
que as determina — capacidade das squads, prontidão do kernel, janelas de mudança de
infraestrutura — não existe no momento desta redação e não é decidível por documento
normativo. O que **é** decidível agora, e é o que envelhece bem, é o critério: quando
um canal deve migrar, o que precisa ser verdade antes, em que ordem, e como se
desfaz. Um cronograma sem critério é uma lista de datas que a primeira reprogramação
invalida; um critério sem cronograma é aplicável a qualquer cronograma que venha.

#### Adoção de Kafka

`normativo` `ADO-02` — **A decisão de migrar é tomada por canal lógico, nunca por
serviço nem por repositório.** Cada canal recebe uma decisão explícita
`migrar` / `não migrar` / `reavaliar em <data>`, com o critério que a sustenta.
Ausência de decisão registrada é `não migrar`: o default é o transporte vigente.

| # | Critério | Peso na decisão |
|---|----------|-----------------|
| K1 | O canal precisa de **retenção e releitura** do histórico | Favorece `migrar` — é a propriedade que SNS/SQS não tem |
| K2 | O canal precisa de **ordenação por chave** com paralelismo por partição | Favorece `migrar` |
| K3 | O canal tem **múltiplos consumidores independentes** com posições próprias | Favorece `migrar` |
| K4 | O canal é **fan-out simples** sem releitura nem ordenação | Favorece `não migrar` — SNS/SQS atende, e migrar adiciona operação sem benefício |
| K5 | O volume e a criticidade do canal justificam operar o transporte novo | Condição — Kafka **não existe hoje no acervo**, e o primeiro canal migrado carrega o custo de instalar a operação inteira |

`evidência` — O inventário AS-IS registra Kafka **ausente** do acervo, e as métricas
de lag e de consumer lag como `não existe` — não é lacuna de medição, é ausência da
infraestrutura. Por isso `ADO-03` é pré-condição e não recomendação.

`normativo` `ADO-03` — **Revisão de infraestrutura é pré-condição do primeiro canal
migrado, não etapa paralela.** Antes de qualquer migração, precisam existir e estar
validados: o cluster e o seu modelo de operação; a observabilidade de lag, de
consumer lag e de DLQ do transporte novo, conforme o catálogo do FND-08; e o rito de
resposta a incidente do transporte. Migrar canal para transporte sem observabilidade
instalada é trocar um modo de falha conhecido por um cego.

`recepcionado` — As regras normativas da coexistência e da troca de canal já estão
fixadas pelo FND-06 e **não são reabertas aqui**. Elas são as condições que qualquer
plano de adoção precisa satisfazer:

| Regra | Conteúdo |
|-------|----------|
| `COE-01` | Um canal lógico tem **um transporte por vez**; publicar o mesmo canal nos dois cria duas ordens e duas linhas de redelivery |
| `COE-02` | O binding é estável por mensagem e por tentativa (`TRP-09`) — com dois transportes configurados, é a única proteção contra publicação dupla do mesmo item de outbox |
| `COE-03` | Trocar transporte é **migração**, com drenagem ou transferência declarada do backlog (`TRP-10`); não existe troca por recarga de configuração |
| `COE-04` | O `consumer_name` da inbox é **lógico e estável entre transportes** (`KFK-08`); nomes diferentes criam duas entradas para a mesma mensagem e o efeito ocorre duas vezes |
| `COE-05` | Ponte entre transportes **preserva a identidade** da mensagem — identificador novo torna a mensagem irreconhecível para a inbox |
| `COE-06` | **Não há ordenação entre os dois transportes**, e a promessa não é feita |
| `TRP-10` | A mudança de binding só se aplica a mensagens ainda não publicadas por nenhuma tentativa |

`normativo` `ADO-04` — **O plano de migração de cada canal declara os seis campos
abaixo.** Um plano incompleto não autoriza corte:

| Campo | Conteúdo | Regra que o obriga |
|-------|----------|--------------------|
| `channel` | O canal lógico, pela identidade que o FND-06 lhe dá | `COE-01` |
| `decision` | `migrar` / `não migrar` / `reavaliar em <data>`, com o critério de `ADO-02` | `ADO-02` |
| `backlog_strategy` | **Drenagem** ou **transferência explícita**, com o critério de conclusão | `COE-03`, `TRP-10` |
| `consumer_name` | O nome lógico preservado entre transportes | `COE-04` |
| `identity_preservation` | Como identificador, origem, tipo e bytes atravessam a ponte, se houver ponte | `COE-05` |
| `owner` | Equipe responsável pelo corte e pelo rollback | `GOV-22` |

`normativo` `ADO-05` — **Rollback é planejado antes do corte, e o critério de
acionamento é declarado.** O plano de `ADO-04` inclui a condição objetiva que dispara
o retorno ao transporte anterior e o estado em que o backlog fica ao retornar.
Rollback sem critério declarado é decisão tomada sob incidente, que é o pior momento
para a tomar.

`normativo` — **O rollback está sujeito às mesmas regras do corte.** Voltar um canal
ao transporte anterior é outra migração, e `COE-03` se aplica: o backlog do
transporte novo é drenado ou transferido explicitamente. Não existe rollback por
recarga de configuração, pela mesma razão que não existe corte por recarga.

#### Adoção da baseline de resiliência e observabilidade

`normativo` `ADO-06` — **A adoção da baseline do FND-08 no acervo é faseada por
capacidade, e a ordem das fases é a de dependência técnica, não a de conveniência.**

| Fase | Conteúdo | Pré-condição | Fundamento da ordem |
|------|----------|--------------|---------------------|
| F1 | **Logs estruturados e correlação** | Nenhuma | É o que torna as fases seguintes diagnosticáveis; o inventário já registra Pino e Logrus em uso, então o delta é de padronização, não de introdução |
| F2 | **Métricas e health checks** padronizados pelo catálogo do FND-08 | F1 | Métrica sem correlação com log não fecha diagnóstico |
| F3 | **Tracing** com as *semantic conventions* na versão do BOM | F1, e `BOM-10` em estado `certificada` | Fixar a versão antes de instrumentar evita instrumentar duas vezes |
| F4 | **Limites de resiliência** por dependência: timeout, retry, breaker, bulkhead, rate limiting, cache | F2 | O limite precisa ser observável para ser calibrado; sem F2, o valor é arbitrado |
| F5 | **Runbook operacional** | F2, F4, e a validação por SRE/Cloud | O FND-08 registra a validação por SRE como `gate externo, aberto` — a fase herda a pendência |

`normativo` — **A adoção de `GOV-22` (owner por pacote) precede F1 no acervo
existente.** O inventário registra `CODEOWNERS` ausente em 10 de 10 repositórios;
sem owner declarado, nenhuma fase tem a quem ser atribuída. Este é o faseamento que
`GOV-22` remete a esta subseção.

`normativo` `ADO-07` — **Cada fase tem owner e critério de conclusão declarados
antes de iniciar.** «Fase concluída» é medida pelo percentual de serviços em escopo
que satisfazem a capacidade, e o escopo é enumerado — não é «os serviços
relevantes». A métrica 6 de `ARQ-436 §16.2` («percentual de serviços com logs,
métricas, traces e health checks padronizados») é exatamente o instrumento de
conclusão de F1 a F3.

`normativo` `ADO-08` — **Datas de calendário e alocação de equipe são `gate
externo`.** Elas pertencem aos épicos de kernel e ao épico 5 de `ARQ-436 §20`. Esta
subseção entrega critérios, ordem, pré-condições e campos obrigatórios; não entrega
cronograma, e a ausência dele é declarada em vez de suprida por estimativa.

### §6.2 Os dois fluxos-piloto

`normativo` `PIL-01` — **O piloto é um fluxo, não um repositório.** A unidade de
seleção é a cadeia concreta `entrada → caso de uso → persistência → publicação →
consumidor → efeito observado`, com os deployables nomeados. Um repositório inteiro
não é piloto: ele contém código que a fundação não alcança, e declarar o repositório
como piloto tornaria a métrica inauditável — não se saberia sobre qual código o
número foi medido.

`rationale` — A distinção não é formal. `legado-titulos-shared-services` é um repositório
**híbrido** — o inventário registra `Stack: ambas — TypeScript/NestJS + Go` —, e
declarar «o repositório» como piloto Go seria factualmente errado. O que é Go é um
trecho dele, e é esse trecho que o charter nomeia.

#### Justificativa da seleção

`normativo` `PIL-02` — **A justificativa do par é cobertura dual-stack, não
interoperabilidade.** Os dois pilotos provam que a fundação é adotável em Go e em
TypeScript, cada um no seu lado. Eles **não** provam que bytes produzidos por um são
consumidos pelo outro: essa é a evidência de interoperabilidade do DoD do épico item
7, e ela exige fluxo equivalente nas duas stacks com fixtures cruzadas — matéria do
FND-09, sob ANC-07, `encaminhada` em §1.6.

`rationale` — Esta justificativa substitui uma anterior, mais forte e incorreta. A
seleção do par foi motivada por «exercitar a interoperabilidade Go ↔ TypeScript», e
dois pilotos independentes em stacks diferentes não a exercitam — cada um exercita a
sua stack. A correção importa porque a justificativa errada faria o charter parecer
satisfazer o item 7 do DoD, que ele não satisfaz. Cobertura dual-stack é o que o par
entrega, e é suficiente para o que o AC-11 item 4 pede: dois fluxos reais.

`registro` — Base factual da seleção, conforme FND-01 §5 e os inventários por
repositório:

| Piloto | Classificação FND-01 | Base registrada |
|--------|----------------------|-----------------|
| `legado-titulos-shared-services` | **`Forte`** | «cobre os quatro eixos DMPF em runtime real (SQS, outbox/inbox, OpenAPI, New Relic/Prometheus) com domínio de produto e dual-stack… melhor laboratório E2E de at-least-once + UoW do que libs isoladas» |
| `legado-ativavel-services` | **`Sim c/ ressalvas`** | «candidatos naturais AS-IS = `api-v2` (CI ativo, Swagger, producer SQS) + `delivery-worker` (consumer isolado, New Relic)»; ressalvas: domínio com I/O, ausência de outbox, idempotência frágil |

`normativo` — **As ressalvas do lado TypeScript são o delta a medir, não defeito de
seleção.** Domínio com I/O, ausência de outbox e idempotência frágil são exatamente
as três propriedades que a fundação existe para corrigir. Um piloto que já as
tivesse resolvidas mediria menos, não mais. O que a ressalva obriga é declarar o
estado de partida com honestidade, o que `PIL-05` faz.

`registro` — Os dois inventários encerram a seção de candidatura com «decisão formal
cabe a `SPEC-VVR1X71Q`». Esta subseção é essa decisão, no que uma decisão documental
pode ser: a seleção está feita e justificada; a **formalização** com as squads é o
`gate externo` **G1**.

#### Charter do piloto Go

`normativo` `PIL-03` — **Piloto A — cadeia de títulos, trecho Go de
`legado-titulos-shared-services`.**

| Campo | Valor |
|-------|-------|
| Stack do piloto | **Go** |
| Repositório | `legado-titulos-shared-services` — **híbrido**, `Stack: ambas — TypeScript/NestJS + Go` conforme o inventário |
| Trecho em escopo | Os deployables **Go** de títulos, e o worker de entrega que expõe `/metrics` Prometheus |
| Trecho **fora** de escopo | Os deployables TypeScript/NestJS do mesmo repositório |
| Entrada | Requisição de operação de título pela superfície OpenAPI |
| Caso de uso / domínio | Operação de título, com a UoW explícita do FND-04 |
| Persistência | Banco relacional do domínio de títulos, na mesma transação da outbox |
| Publicação | Outbox transacional, com relay para SNS/SQS |
| Consumidor | Worker de entrega, com inbox |
| Efeito observado | Efeito de negócio da entrega, idempotente por `(consumer_name, message_id)` |
| Owner | **ausente** — `gate externo` **G1** |
| Critério de entrada | `ADO-06` F1 e F2 satisfeitas no escopo do piloto; `GOV-22` satisfeita no repositório |

`normativo` — **Por que este trecho, e não o repositório.** O inventário o
classifica `Forte` por cobrir os quatro eixos em **runtime real** — SQS, outbox e
inbox, OpenAPI, e observabilidade com New Relic e Prometheus. É o único candidato do
acervo com outbox em produção, o que o torna o único capaz de medir as métricas 5 e 7
de `ARQ-436 §16.2` contra comportamento observado em vez de contra implementação
nova.

#### Charter do piloto TypeScript

`normativo` `PIL-04` — **Piloto B — cadeia de ativação, `legado-ativavel-services`.**

| Campo | Valor |
|-------|-------|
| Stack do piloto | **TypeScript** |
| Deployables em escopo | **`api-v2`** (CI ativo, Swagger, producer SQS) e **`delivery-worker`** (consumer isolado, New Relic) |
| Deployable **fora** de escopo | `events-pauta-reader` (S3 → PostgreSQL) — o inventário o registra como candidato «em menor escopo»; ele não participa da cadeia de ativação e entra apenas se a squad o incluir na formalização |
| Entrada | Requisição de ativação pela superfície OpenAPI de `api-v2` |
| Caso de uso / domínio | Ativação, **com o domínio a extrair do I/O** — é o delta declarado |
| Persistência | Banco relacional da ativação |
| Publicação | SQS por `api-v2`; **outbox a introduzir** — hoje ausente, é o segundo delta |
| Consumidor | `delivery-worker`; **inbox a introduzir** — idempotência hoje frágil, é o terceiro delta |
| Efeito observado | Efeito de negócio da entrega, que passa a ser idempotente pela inbox |
| Owner | **ausente** — `gate externo` **G1** |
| Critério de entrada | `ADO-06` F1 e F2 satisfeitas no escopo do piloto; `GOV-22` satisfeita no repositório |

`normativo` — **Os três deltas de `PIL-04` são o objeto da medição**, e são
declarados como estado de partida em `PIL-05`: domínio acoplado a I/O, ausência de
outbox e idempotência frágil. O piloto B mede a introdução das três propriedades; o
piloto A mede a operação delas onde já existem. É a assimetria que torna o par
informativo — e é razão adicional para o par, além da cobertura de stack.

#### Matriz de métricas

`normativo` `PIL-05` — **Cada uma das nove métricas de `ARQ-436 §16.2` é declarada
por piloto com os campos abaixo, separados.** A separação entre `baseline_status` e
`baseline_value` é a regra central desta subseção:

| Campo | Conteúdo |
|-------|----------|
| `baseline_status` | Estado da **coleta**: `medido`, `não medido`, `não existe`, `a coletar` |
| `baseline_value` | O **valor** de partida, com unidade. Ausente enquanto `baseline_status` não for `medido` |
| `unit` | Unidade da medida |
| `window` | Janela ou amostra sobre a qual o valor é apurado |
| `instrument` | Como se mede, nominalmente |
| `owner` | Equipe responsável pela coleta |
| `due` | Prazo da coleta |
| `target` | Alvo. Ausente enquanto não houver `baseline_value` e aceite do owner |

`normativo` `PIL-06` — **`não medido` é estado da coleta, nunca valor de partida, e
o gate de baseline permanece aberto enquanto `baseline_value` estiver ausente.** O
DoD do épico item 9 admite métricas «registradas **ou** possuindo plano de coleta» —
o que satisfaz o DoD com plano é o **DoD**, não o requisito não-funcional da spec,
que pede «valor de partida e alvo». Os dois são declarados separadamente, e nenhum é
apresentado como o outro.

`normativo` `PIL-07` — **Nenhum `target` é fixado sem `baseline_value` e sem aceite
do owner.** Alvo derivado de baseline ausente é número inventado, e o preâmbulo do
próprio catálogo o veda: «as metas definitivas devem ser calibradas depois do
inventário AS-IS». O inventário está feito e registra as lacunas; calibrar exige a
coleta, não a publicação do inventário.

`registro` — Matriz das nove métricas. `baseline_status` conforme FND-01 §4.1
(`Fato`) e §4.2 (lacunas). Nenhuma linha traz `baseline_value` ou `target`, porque
nenhuma tem coleta concluída — a ausência é o registro, não uma omissão:

| # | Métrica (`ARQ-436 §16.2`) | `baseline_status` A (Go) | `baseline_status` B (TS) | `unit` | `instrument` | `owner` | `due` |
|---|---------------------------|--------------------------|--------------------------|--------|--------------|---------|-------|
| 1 | Lead time entre scaffold e execução local | `a coletar` | `a coletar` | minutos | Cronometragem do scaffold ao primeiro `run` local, por pessoa, no onboarding do piloto | G1 | G1 |
| 2 | Tempo para implementar a primeira feature vertical | `a coletar` | `a coletar` | dias úteis | Do início da feature ao merge, medido na primeira feature vertical de cada piloto | G1 | G1 |
| 3 | Código técnico repetido por serviço | `não medido` | `não medido` | linhas ou blocos duplicados | Detector de duplicação no CI, com o escopo restrito ao trecho do piloto | G1 | G1 |
| 4 | Violações de dependência detectadas no CI | `não existe` | `não existe` | violações por PR | Verificador de RFC §10 — **não instalado**: ANC-10 é a âncora da implantação, e ela é dos épicos de kernel | G1, após ANC-10 | G1 |
| 5 | Cobertura dos cenários de redelivery, inbox/outbox e DLQ | `não medido` | `não existe` | percentual dos cenários mínimos | Suite de cenários do AC-07; em B, inbox e outbox **não existem** hoje | G1 | G1 |
| 6 | Serviços com logs, métricas, traces e health checks padronizados | `não medido` | `não medido` | percentual dos serviços em escopo | Conferência contra o catálogo do FND-08; é o mesmo instrumento de conclusão de `ADO-06` F1 a F3 | G1 | G1 |
| 7 | Defeitos e incidentes associados a integração | `não medido` | `não medido` | incidentes por período | Classificação de incidente por causa, na ferramenta de operação | G1 | G1 |
| 8 | Tempo de onboarding | `a coletar` | `a coletar` | dias úteis | Do primeiro dia à primeira entrega em produção, por pessoa | G1 | G1 |
| 9 | Satisfação e atrito percebido pelas squads | `a coletar` | `a coletar` | escala declarada no instrumento | Instrumento qualitativo aplicado à squad, com a escala declarada antes da primeira aplicação | G1 | G1 |

`normativo` — **Distinção entre os três estados de partida da matriz.** `não medido`
é capacidade instalada sem medição — o valor é obtenível assim que alguém medir.
`não existe` é ausência da capacidade — não há o que medir até que ela seja
introduzida, e é o caso da métrica 4 nos dois pilotos (o verificador de RFC §10 não
está implantado; ANC-10 é a âncora dessa implantação, e é dos épicos de kernel) e da
métrica 5 no piloto B (inbox e outbox ausentes). `a coletar` é medida de processo que
só existe quando o processo ocorre — não há histórico a recuperar, e o primeiro valor
nasce na primeira execução.

`normativo` `PIL-08` — **A métrica 4 tem dependência declarada e não é exigível
antes dela.** Ela mede «violações de dependência detectadas no CI», e a detecção
depende do verificador de RFC §10, cuja implantação é ANC-10 — «Adoção do metadado
nos repositórios existentes», owner «Épicos de kernel», com condição de fechamento
«verificador conforme passando nos 32 vetores de §11». Cobrar a métrica 4 do piloto
antes de ANC-10 seria cobrar medição de instrumento inexistente.

`normativo` `PIL-09` — **O charter é formalizado, não declarado.** A formalização é
o `gate externo` **G1**, e ela consiste em: owner nomeado por piloto, aceite
registrado da squad responsável, e aceite dos `target` de `PIL-07` pelos owners.
Enquanto G1 estiver aberto, o estado de `PIL-03` e `PIL-04` é `definido`, o AC-11
item 4 permanece aberto, e a obrigação O6 do ledger permanece `selecionada, não
formalizada`.

---

## §7. Prontidão dos épicos subsequentes e continuidade

`normativo` — Sob **ANC-09**. IDs `RDY-01`+.

Esta seção entrega o **gabarito de verificação** de prontidão e o **instrumento** de
registro da decisão de continuidade. Ela não declara nenhum épico `Ready` e não
registra a decisão, porque nenhum dos dois é ato de documento.

### §7.1 O que «Ready» é, e por que nenhum épico o é aqui

`normativo` `RDY-01` — **`Ready` é estado verificado contra os sete itens da DoR de
`ARQ-436 §17`, e a verificação é feita por quem tem a evidência.** Este artefato não
declara nenhum dos cinco épicos `Ready`. Declarar seria afirmar, por documento, fatos
que dependem de ato de terceiro: que existem owner e reviewers nomeados (item 3) e
que a equipe responsável estimou o trabalho (item 7).

`recepcionado` — Os sete itens da DoR, literais: «Um item filho pode entrar em
execução quando:»

| # | Item da DoR (`ARQ-436 §17`) | Natureza |
|---|-----------------------------|----------|
| D1 | possui problema e resultado esperado claros | documental |
| D2 | identifica decisões da RFC e ADRs relacionadas | documental |
| D3 | possui owner e reviewers definidos | **ato de terceiro** |
| D4 | explicita dependências e artefatos de entrada | documental |
| D5 | contém critérios de aceite verificáveis | documental |
| D6 | não depende de decisão estrutural ainda sem owner | **condicional a G2** |
| D7 | está estimado pela equipe responsável | **ato de terceiro** |

`normativo` `RDY-02` — **D6 está aberto para todos os cinco épicos enquanto o gate
G2 estiver aberto.** «Não depende de decisão estrutural ainda sem owner» é uma
condição que a autoridade de classificação de §5.2 afeta diretamente: ela é decisão
estrutural — a própria ANC-08 o declara ao exigir ADR porque «a autoridade de
classificação é decisão estrutural» — e ela está **sem titular** (`AUT-07`). Todo
épico cujo escopo inclua declarar ou alterar classificação de unidade depende dela.

`rationale` — Este é o achado menos confortável desta seção e o mais consequente. A
métrica de conclusão do épico exige «Épicos subsequentes em estado Ready = 100% dos
cinco», e D6 é o item que a torna dependente de §5.2. A leitura alternativa seria
tratar D6 como satisfeito porque a decisão **está tomada** neste artefato — o
processo está definido. Ela foi descartada: D6 não diz «decisão estrutural sem
processo», diz «sem **owner**», e owner de uma função sem titular aceito não existe.
Registrar D6 como satisfeito converteria o gate G2 em formalidade e faria os cinco
épicos parecerem prontos sob uma condição que ninguém verificou.

### §7.2 Matriz dos cinco épicos contra a DoR

`registro` — Os cinco épicos de `ARQ-436 §20`, com o objetivo literal, verificados
item a item. `n/a por este artefato` significa que a verificação não é deste
documento e não que o item esteja satisfeito.

| Épico (`§20`) | Objetivo literal | D1 | D2 | D4 | D5 | D3 | D6 | D7 |
|---------------|------------------|----|----|----|----|----|----|----|
| **1. Kernel e SDK de Referência Go** | «Implementar a Parte 2 de forma idiomática e aderente à fundação em Monorepo NX utilizando Golang nas implementações» | a redigir no épico | RFC §4–§7, §9, §10; FND-03 a FND-08; `ADR-DMPF-A`..`Q` | `RDY-03` | a redigir no épico | **aberto** — G3 | **aberto** — G2 | **aberto** — G3 |
| **2. Kernel e SDK de Referência TypeScript** | «Implementar a Parte 2 de forma idiomática e aderente à fundação em Monorepo NX utilizando Node.js nas implementações» | a redigir no épico | idem, com o binding TS de RFC §3 | `RDY-03` | a redigir no épico | **aberto** — G3 | **aberto** — G2 | **aberto** — G3 |
| **3. Contratos, Adapters e Mensageria Confiável** | «Implementar Protobuf, REST, gRPC, Kafka, SNS/SQS, PostgreSQL, inbox/outbox e relay» | a redigir no épico | FND-04, FND-05, FND-06; `ADR-DMPF-K`..`P` | `RDY-03` | a redigir no épico | **aberto** — G3 | **aberto** — G2 | **aberto** — G3 |
| **4. Golden Path e Aplicações de Referência** | «Entregar uma vertical completa equivalente nas duas stacks» | a redigir no épico | FND-09 (ANC-07), e os dois kernels | `RDY-03` | a redigir no épico | **aberto** — G3 | **aberto** — G2 | **aberto** — G3 |
| **5. Piloto, Estabilização e Adoção** | «Validar com squads reais, remover atritos e preparar a versão 1.0» | a redigir no épico | §6 deste artefato; `ADR-DMPF-R` | `RDY-03` | a redigir no épico | **aberto** — G3 | **aberto** — G2 | **aberto** — G3 |

`normativo` `RDY-03` — **Dependências e artefatos de entrada dos cinco épicos**, que
é o item D4 e o que o AC-12 item 2 cobra como «dependências, responsáveis e critérios
de entrada explícitos»:

| Épico | Depende de | Artefatos de entrada | Critério de entrada |
|-------|------------|----------------------|---------------------|
| 1. Kernel Go | RFC; FND-03 a FND-08; ANC-10 para o verificador | RFC e os sete artefatos publicados; BOM inicial | BOM com runtime Go `certificada` (`BOM-07`); `GOV-22` satisfeita no repositório do kernel |
| 2. Kernel TS | idem, com o binding TS de RFC §3 | idem | BOM com Node.js e TypeScript `certificada` |
| 3. Contratos e mensageria | FND-04, FND-05, FND-06; **`ADO-03`** para Kafka | FND-05 (registro de stacks, pins), FND-06 (políticas) | `ADO-03` satisfeita antes de qualquer canal Kafka; `BUF-07` com stack certificada |
| 4. Golden path | Épicos 1, 2 e 3; **FND-09** para a evidência de interoperabilidade | Os dois kernels; fixtures cruzadas do FND-09, **já mergeadas em `develop`** | FND-09 mergeado; resta a reconciliação de C8 |
| 5. Piloto e adoção | Épico 4; **G1** para os charters; `ADO-06` para o faseamento | §6 deste artefato: charters, matriz de métricas, faseamento | G1 satisfeita: owners nomeados e squads com aceite registrado |

`normativo` `RDY-04` — **Criar e vincular os cinco épicos é `gate externo` G3.** Este
artefato nomeia os cinco, fixa objetivo, dependências, artefatos de entrada e
critério de entrada de cada um, e entrega o gabarito de verificação. A criação no
tracker, a vinculação ao ARQ-436 e a estimativa pela equipe são atos de terceiro.

`normativo` `RDY-05` — **As três métricas de conclusão de `ARQ-436 §16.1` que recaem
sobre FND-10 têm o seu estado declarado aqui**, e nenhuma é apresentada como
satisfeita:

| Métrica (`§16.1`) | Meta | Estado | Condição |
|-------------------|------|--------|----------|
| Fluxos-piloto com owner e baseline | 2 | **0** — dois selecionados, nenhum com owner nem baseline | G1 (owner) e C6 (baseline) |
| Riscos críticos sem owner | 0 | **≥ 1** — a autoridade de classificação é decisão estrutural sem titular | G2 |
| Épicos subsequentes em estado Ready | 100% dos cinco | **0 de 5** | G2, G3 |

`rationale` — Declarar `0`, `≥ 1` e `0 de 5` é o oposto do que um artefato de
fechamento é tentado a fazer, e é o que o torna utilizável. A alternativa seria
reportar «2 pilotos selecionados» na primeira linha e deixar a leitura de «com owner
e baseline» para quem conferisse — o número da meta é 2, e há 2 pilotos. Mas a
métrica não conta pilotos: conta pilotos **com owner e baseline**, e não há nenhum.

### §7.3 Instrumento de registro da decisão de continuidade

`normativo` `RDY-06` — **A decisão de continuidade é registrada pelo instrumento
abaixo, e o registro é `gate externo` G4.** O AC-12 item 4 pede que «a revisão final
registre a decisão formal de seguir, ajustar ou interromper a iniciativa»; o DoD item
12 repete. Este artefato define a forma; a revisão final produz o conteúdo.

| Campo | Conteúdo exigido |
|-------|------------------|
| `decision` | Exatamente um de: `seguir` · `ajustar` · `interromper` |
| `decided_at` | Data da revisão final |
| `deciders` | Quem decidiu, por área |
| `evidence_reviewed` | Os artefatos e as evidências considerados, nominalmente |
| `ac_status` | O estado de AC-01 a AC-12 no momento da decisão, item a item |
| `open_gates` | Os gates externos ainda abertos, com dona |
| `conditions` | Quando `decision` é `ajustar`: o que muda, com dona e prazo |
| `rationale` | Por que esta decisão e não as outras duas |

`normativo` — **`ajustar` sem o campo `conditions` preenchido é registro
incompleto**, e não satisfaz G4. «Ajustar» sem dizer o que se ajusta é indistinguível
de «seguir» na prática, e a distinção entre as três opções é a razão de o critério
existir.

`normativo` `RDY-07` — **A decisão de continuidade não é tomada com gate externo
aberto sem que o gate conste em `open_gates`.** A decisão pode legitimamente ser
`seguir` com gates abertos — é decisão de quem decide, não deste documento. O que a
regra impede é que ela seja tomada sem que os gates estejam à vista de quem a toma.

---

## §8. ADRs, rastreabilidade e pendências

`registro` — Esta seção não estabelece regra nova. Ela aciona os ADRs, registra a
contradição de cardinalidade da série, liga cada regra à sua origem e à sua
verificação, e lista o que fica aberto com dona e condição.

### §8.1 ADRs acionados

`normativo` — Estado `acionado` significa, conforme RFC §13.1: nomeado, com assunto
definido e encaminhado. **Nenhum destes ADRs está redigido ou aceito.** A redação e a
numeração definitiva são do FND-11 (ARQ-448).

| ID provisório | Nome | Assunto | Origem | Destino | Owner | Estado |
|---------------|------|---------|--------|---------|-------|--------|
| `ADR-DMPF-R` | Governança de produto, BOM e compatibilidade | Adotar BOM por **combinação certificada com evidência persistida e validade declarada**, em vez de faixa de versão por dependência; fixar a compatibilidade por **sujeito versionado** com piso `N`/`N−1` e classe de criticidade declarada, com o pin exato da ferramenta de geração como exceção; governar a depreciação de **produto** com janela mínima de 180 dias, mantendo a de contrato no FND-05; e exigir do escape hatch **universo positivo fechado** com regra de negação sobre constraint P0, célula de RFC §7 e invariante de âncora | §3, §4, §5.1 deste artefato; exigido pela tabela §8 do épico ARQ-436 (assunto do «ADR-015») e pelo entregável 5 da ARQ-447 | ARQ-448 | FND-11 | `acionado` |
| `ADR-DMPF-S` | Processo de autorização da classificação | Adotar **função organizacional fechada** (Autoridade de Classificação Arquitetural) indicada por Arquitetura e revisada por Segurança, com rito de commit próprio, declaração do ato, justificativa da incorreção anterior, **enumeração das arestas que a reclassificação libera**, e **mais de um aprovador** distinto do autor; fixar a evidência como registro persistido no repositório, e amarrar a vigência ao aceite de titular, mantendo o mecanismo mínimo de RFC §10.2 até então | §5.2 deste artefato; RFC §10.2 e §13.3 — a RFC declara que o **processo** é acionado por FND-10 via ANC-08 | ARQ-448 | FND-11 | `acionado` |

`normativo` — **`ADR-DMPF-S` não reaciona `ADR-DMPF-D`.** Os dois tratam do mesmo
tema em níveis distintos, e a distinção é da própria RFC:

| ADR | Nível | Conteúdo | Quem acionou |
|-----|-------|----------|--------------|
| `ADR-DMPF-D` | **Requisito** | «Exigir autorização distinta da autoria para mudança de `block` e `bounded_context`, com fail-closed na ausência de evidência» | RFC §13.2 — «esta RFC aciona o que ela mesma decide» |
| `ADR-DMPF-S` | **Processo** | Quem é a autoridade, por qual rito, com qual evidência | Este artefato, por RFC §13.3 |

`normativo` — **Alternativas descartadas**, insumo obrigatório da redação por RFC
§13.2: «um ADR que omita a alternativa considerada perde a função de registro».

| ADR | Alternativas descartadas |
|-----|-------------------------|
| `ADR-DMPF-R` | Declarar faixas SemVer amplas por dependência em vez de combinação certificada — descartada porque a combinação específica que quebra é a que nunca foi exercitada (`BOM-04`); tratar «`N` e `N−1`» como regra única sem sujeito — descartada por indecidibilidade (`GOV-10`); admitir `N−1` de plugin de geração — descartada por reintroduzir irreprodutibilidade contra `BUF-06` (`GOV-12`); default de criticidade no lado permissivo — descartada porque a omissão viraria licença (`GOV-13`); manter «plano de convergência quando aplicável» do AC-11 — descartada porque quem pede a exceção avalia a aplicabilidade (`GOV-30`); listar apenas negações no escape hatch, sem universo positivo — descartada por incompletude contra pedido criativo (`GOV-31`); fixar cadência de release train sem release publicada — descartada por inventar número não cumprível (`GOV-16`) |
| `ADR-DMPF-S` | Nomear pessoa ou equipe específica como autoridade — descartada por atribuir responsabilidade sem consentimento, com `CODEOWNERS` ausente em 10 de 10 repositórios (`AUT-07`); aprovador único distinto do autor — descartada por ser ponto único de falha em controle adversarial (`AUT-04`); fazer o rito viger na promoção deste PR, pela leitura de «até que o FND-10 **defina**» — descartada porque produziria controle inoperante (função sem titular) no lugar do controle mínimo operante da RFC (`AUT-09`); tratar «não verificada» como aprovação provisória — descartada por esvaziar T5 (`AUT-06`); regular apenas os campos `block` e `bounded_context`, sem alcançar mudança de `include` que altere o bloco efetivo — descartada porque tornaria o rito evitável por construção (`AUT-01`) |

### §8.2 A contradição de cardinalidade da série de ADRs

`registro` — Uma contradição **pré-existente** entre a faixa que o FND-11 reserva e o
número de acionamentos que o acervo já produziu. FND-10 não a cria; ela a agrava de
17 para 19.

| Item | Valor | Fonte |
|------|-------|-------|
| Faixa reservada para a promoção | `docs/adr/010`–`024` = **15 slots** | `SPEC-DBTRMM3X`, requisito funcional P0 |
| Critério de aceite do FND-11 | «**15 ADRs mínimos** promovidos»; «README com as **15** entradas» | `SPEC-DBTRMM3X`, Verificação |
| Tabela de ADRs do épico | `ADR-001` a `ADR-015` = **15** linhas | `ARQ-436 §8` |
| Acionamentos já no acervo | `ADR-DMPF-A` a `ADR-DMPF-Q` = **17** | RFC §13.2 (A–H) e os artefatos publicados (I–Q) |
| Com os dois deste artefato | `ADR-DMPF-A` a `ADR-DMPF-S` = **19** | §8.1 |

`normativo` — **Este artefato não escreve «ADR-015» como identificador vigente.** A
RFC §13.2 é explícita: «referenciar um ADR desta tabela por número definitivo antes
da promoção é erro de rastreabilidade». O que §8.1 registra é a **correspondência de
assunto** entre `ADR-DMPF-R` e a linha `ADR-015` da tabela §8 do épico — não a
identidade de número.

`registro` — **Mapa de consolidação proposto**, oferecido como **insumo** ao FND-11 e
não como decisão tomada aqui. A dona da reconciliação é o FND-11 (obrigação O12):

| Linha do épico `§8` | Acionamentos que lhe correspondem |
|---------------------|-----------------------------------|
| ADR-001 — Limites arquiteturais e regra de dependência | `A`, `E`, `F` |
| ADR-002 — UPR síncrona e determinística com `Decision` explícita | `I` |
| ADR-003 — Separação entre contratos de domínio, aplicação e wire | `J`, `L` |
| ADR-004 — Protobuf como contrato de wire e não modelo de domínio | `M` (parte) |
| ADR-005 — CloudEvents Protobuf e perfil organizacional | `M`, `N` |
| ADR-006 — Unit of Work explícita no application service | — (acionamento não identificado; lacuna a registrar) |
| ADR-007 — Outbox transacional e relay por polling com leasing | `K` |
| ADR-008 — Inbox transacional e semântica de ACK/redelivery | — (idem) |
| ADR-009 — At-least-once com efeitos idempotentes | — (idem) |
| ADR-010 — REST/OpenAPI externo e gRPC/Protobuf interno | `O` |
| ADR-011 — Políticas para Kafka e SNS/SQS | `P` |
| ADR-012 — Execution context, identidade e multi-tenancy | — (idem) |
| ADR-013 — Taxonomia de erros e retryability | — (idem) |
| ADR-014 — Observabilidade e convenções OpenTelemetry | `Q` |
| ADR-015 — BOM, compatibilidade, versionamento e depreciação | **`R`** |
| *sem linha correspondente* | `B` (unidade arquitetural e binding), `C` (classificação por metadado), `D` (autoridade — requisito), `G` (domínio executável), `H` (identidade de bounded context), **`S`** (autoridade — processo) |

`normativo` — **A última linha do mapa é o núcleo da contradição, e ela não é
resolvida aqui.** Seis acionamentos não têm linha correspondente na tabela de 15 do
épico, e cinco linhas do épico não têm acionamento identificado. Consolidar 19 em 15
não é aritmética: é decidir quais acionamentos se fundem, quais linhas do épico ficam
sem ADR próprio, e se a faixa `010`–`024` precisa ser ampliada. A decisão é do
FND-11, que detém o mecanismo de promoção e o critério de aceite que a mede.

`registro` — **Omissão a reconciliar, além da cardinalidade.** A tabela de cadência
de `SPEC-DBTRMM3X` lista os grupos acionados por FND-02 a FND-08 e **omite FND-10**,
embora a RFC §13.3 atribua acionamento a esta sub-spec e o épico §8 cubra o assunto do
ADR-015. O FND-09 legitimamente não está na tabela, porque ANC-07 declara «ADR
exigido: Não» sem condicional. FND-10 está fora por **omissão**, não por decisão —
ANC-08 declara «ADR exigido: **Sim**». Dona da correção: FND-11.

### §8.3 Rastreabilidade

`registro` — Regras por família, com sujeito e âncora:

| Família | Faixa | Seções | Assunto | Âncora |
|---------|-------|--------|---------|--------|
| `GOV` | `GOV-01` a `GOV-09` | §1 | Fronteira, precedência, estados observáveis, gates externos | ANC-08 e ANC-09, declaradas por regra |
| `GOV` | `GOV-10` a `GOV-25` | §3 | Sujeitos versionados, compatibilidade, SemVer, release train, depreciação de produto, instrumentos | ANC-09 |
| `GOV` | `GOV-26`; `27`–`29` **reservados sem uso** | §5 abertura | Não substituição entre os dois ritos | ANC-09 |
| `GOV` | `GOV-30` a `GOV-36` | §5.1 | Escape hatch: requisitos, universo positivo, negação, nominalidade, ciclo de vida, evidência, métrica | ANC-09 |
| `BOM` | `BOM-01` a `BOM-10` | §4 | Conteúdo, schema, autoridade, referência sem duplicação, máquina de estados, validade, CVE, slot das convenções | ANC-09 |
| `AUT` | `AUT-01` a `AUT-10` | §5.2 | Ato regulado, autoridade, rito, pluralidade, evidência, fail-closed, titular, vetores negativos, vigência, alcance de R1 | **ANC-08** |
| `ADO` | `ADO-00` a `ADO-08` | §6.1 | Fronteira planejamento × execução, critérios por canal, pré-condição de infraestrutura, plano de migração, rollback, faseamento da baseline | ANC-09, **com força suspensa** por `ADO-00` até P13 |
| `PIL` | `PIL-01` a `PIL-09` | §6.2 | Piloto como fluxo, justificativa, os dois charters, matriz de métricas, campos separados, formalização | ANC-09 |
| `RDY` | `RDY-01` a `RDY-07` | §7 | Definição de Ready, D6 e G2, dependências dos épicos, métricas de §16.1, instrumento de continuidade | ANC-09 |

`registro` — **78 regras com identificador**: 77 `normativo` e uma `encaminhado`
(`GOV-24`, office hours e canal de suporte, encaminhada ao **owner do épico 5** de
`ARQ-436 §20`, conforme a regra declara).
A contagem exclui os parágrafos `normativo` sem ID, que qualificam ou compõem uma
regra identificada em vez de estabelecer regra nova — e exclui os blocos
`recepcionado`, cuja força permanece da fonte.

`registro` — **Duas irregularidades de numeração, ambas declaradas.** A primeira é
`ADO-00`, que abre a faixa em zero em vez de um: ela foi acrescentada por revisão
adversarial **depois** de `ADO-01` a `ADO-08` estarem escritas, e precisa preceder
todas elas porque suspende a força do conjunto. Renumerar a faixa inteira invalidaria
referência cruzada, que é o que a estabilidade de ID existe para evitar. A segunda é o
vetor **`V3a`** em `AUT-08`, acrescentado entre `V3` e `V4` pela mesma revisão — é a
convenção que o FND-06 e o FND-08 já usam com `SQS-08b`, `MET-05a`, `RUN-11a` e
`RUN-18a`, e ela vale para os vetores enumerados de uma tabela, não para IDs de regra.
Nenhum **ID de regra** deste artefato recebe sufixo de letra; a faixa `GOV-27`–`29`
segue reservada e vazia para que §5 possa crescer sem sufixar.

`registro` — Cadeia `obrigação → regra → verificação → estado` para as doze
obrigações do ledger de §2.1:

| Obrigação | Regras que a atendem | Verificação | Estado |
|-----------|----------------------|-------------|--------|
| O1 Kafka | `ADO-02` a `ADO-05` | Revisão de PR do plano por canal, contra os seis campos de `ADO-04` | `atendida` |
| O2 Baseline | `ADO-06`, `ADO-07` | Percentual de serviços em escopo por fase (métrica 6 de §16.2) | `atendida` |
| O3 *Semantic conventions* | `BOM-10` | `subject` presente no BOM, com owner; promoção por `BOM-07` | `atendida com pendência de titular` |
| O4 Autoridade T4–T6 | `AUT-02` a `AUT-06` | Rito de `AUT-03` verificável no PR; evidência de `AUT-05` no repositório | `definida, não vigente` — G2 |
| O5 ADR do processo | §8.1, `ADR-DMPF-S` | Presença na tabela de acionamento | `atendida` |
| O6 Pilotos | `PIL-01` a `PIL-04`, `PIL-09` | Charter com cadeia completa; formalização por G1 | `selecionada, não formalizada` — G1 |
| O7 ADR de BOM | §8.1, `ADR-DMPF-R` | Presença na tabela; correspondência de assunto com ADR-015 | `atendida` |
| O8 Quatro instrumentos | `GOV-22` a `GOV-25`; tabela de §3.6 | Dez de dez linhas endereçadas | `atendida` |
| O9 Toolchain do FND-05 | `BOM-06` | Referência por coordenada, sem valor duplicado | `atendida` |
| O10 Mais de um aprovador | `AUT-04` | Tabela de cinco configurações; `AUT-08` V2 e V4 | `atendida` |
| O11 Migração de canal | `ADO-04`, e `COE-01`..`COE-06` recepcionadas | Seis campos do plano; regras do FND-06 inalteradas | `atendida` |
| O12 Cardinalidade dos ADRs | §8.2, com mapa proposto | Reconciliação pelo FND-11 | `encaminhada` — C7 |

### §8.4 Estado das âncoras e pendências

`registro` — **Estado das duas âncoras**, verificado contra os `closure_gates` de
§2.2, conforme `GOV-08`:

| Âncora | Escopo entregue | Condição de fechamento | Estado |
|--------|-----------------|------------------------|--------|
| **ANC-09** | Matriz de versões certificadas (§3, §4), escape hatches (§5.1), seleção de pilotos (§6.2) | «FND-10 concluída e revisada» | **aberta** — C1 (G1), C3 (G3), C4 (G4), C6 (baseline), C7 (O12) |
| **ANC-08** | Autoridade, rito e artefato de evidência que satisfazem T4–T6 (§5.2) | «FND-10 concluída e revisada; até lá, R1 permanece parcialmente mitigado» | **aberta** — C2 (G2) e C5 (G5). O impacto de versão «Menor» **não** foi realizado |

`normativo` — **Nenhuma das duas âncoras é declarada fechada por este artefato.** O
escopo permitido de ambas está coberto, e é isso que a redação entrega; o fechamento
depende dos gates. Declarar fechamento com gate aberto seria o defeito que `GOV-05`
proíbe.

`registro` — **Pendências, com dona e condição:**

| # | Pendência | Dona | Condição de fechamento |
|---|-----------|------|------------------------|
| P1 | Aceite formal das duas squads e owners dos pilotos (**G1**) | Owner do ARQ-447, com as lideranças das squads | Registro nominal de aceite por piloto |
| P2 | Titular aceito para a Autoridade de Classificação Arquitetural (**G2**) | Arquitetura, revisão de Segurança | `AUT-09`: titular aceito com evidência registrada, e revisão de Segurança |
| P3 | Criação e vinculação dos cinco épicos (**G3**) | Owner do ARQ-436 | Cinco épicos criados, vinculados, verificados contra a DoR §17 |
| P4 | Registro da decisão de continuidade (**G4**) | Revisão final do épico | Instrumento de `RDY-06` preenchido, com `conditions` se `ajustar` |
| P5 | Citação do processo de §5.2 em **RFC §10.2**, com incremento `0.2` (**G5**) | Owner da RFC | RFC §14.3: revisão pelas quatro áreas **e** reconciliação com baseline aprovado. O aceite de 2026-08-14 que dispensou ambos vale só para a `0.1` |
| P6 | Valores de baseline das nove métricas por piloto | Owners dos pilotos (após G1) | `baseline_value` presente; `target` com aceite do owner (`PIL-07`) |
| P7 | Reconciliação de cardinalidade da série de ADRs, e inclusão de FND-10 na tabela de cadência | **FND-11** — ARQ-448 | Mapa de consolidação decidido; faixa reservada compatível com o número de acionamentos |
| P8 | Redação e numeração de `ADR-DMPF-R` e `ADR-DMPF-S` | **FND-11** — ARQ-448 | Promoção para `docs/adr/`, com as alternativas descartadas de §8.1 |
| P9 | **Reconciliação com o FND-09**, mergeado em `develop` durante esta entrega | Owner desta sub-spec, com o do ARQ-446 | Revisitar `PIL-02` à luz das fixtures cruzadas do FND-09. Verificado: sem colisão de prefixo e sem obrigação nova para FND-10 (`GOV-06`). Não afeta as âncoras deste artefato |
| P10 | Valor da periodicidade do release train | Owner do épico de kernel | `GOV-16`: primeira release publicada |
| P11 | Office hours e canal de suporte | Owner do épico 5 de `§20` | `GOV-24`: canal declarado e cadência das office hours |
| P12 | Instalação do verificador de RFC §10, de que a métrica 4 depende | Épicos de kernel, sob **ANC-10** | Verificador conforme passando nos 32 vetores de RFC §11 (`PIL-08`) |
| P13 | **Ampliação do escopo da ANC-09 para receber adoção e faseamento**, ou âncora nova para o assunto | **Owner da RFC** | RFC §14.2. Enquanto aberta, `ADO-01` a `ADO-08` permanecem `definido` com força suspensa (`ADO-00`). Os irmãos FND-06 §15 e FND-08 §13 já delegaram o assunto a FND-10, mas o escopo registrado da âncora não o cobre |

`registro` — **Estado dos critérios de aceite que este artefato serve:**

| AC | Item | Estado | O que falta |
|----|------|--------|-------------|
| AC-11 | 1 — SemVer, RFCs, release train, compatibilidade, depreciação e suporte definidos | **satisfeito**, com P10 aberta para o valor da cadência | `GOV-16` |
| AC-11 | 2 — Template de BOM com runtimes, generators, drivers, SDKs, CVEs e combinações certificadas | **satisfeito** | — |
| AC-11 | 3 — Escape hatches exigem ADR, owner, justificativa e plano de convergência | **satisfeito**, e endurecido: exceção sem prazo é negada | — |
| AC-11 | 4 — Dois fluxos-piloto selecionados com participação formal das squads | **parcial** — selecionados e justificados; sem participação formal | G1 |
| AC-11 | 5 — Métricas de baseline e critérios de sucesso acordados | **parcial** — instrumento e critérios definidos; sem baseline e sem aceite | G1, P6 |
| AC-12 | 1 — Cinco épicos criados e vinculados | **aberto** | G3 |
| AC-12 | 2 — Dependências, responsáveis e critérios de entrada explícitos | **parcial** — dependências e critérios de entrada definidos (`RDY-03`); **responsáveis pendentes** | G3 |
| AC-12 | 3 — Nenhum risco arquitetural crítico sem owner | **aberto** — a autoridade de classificação é decisão estrutural sem titular | G2 |
| AC-12 | 4 — Revisão final registra a decisão de continuidade | **aberto** — instrumento definido, registro pendente | G4 |
| ARQ-447 | 10 — ADR do BOM registrado com contexto, alternativas, trade-offs e consequências | **parcial** — acionado com assunto e alternativas descartadas (§8.1); contexto, trade-offs e consequências são conteúdo da redação | P8 |

`normativo` — **A última linha não pertence a AC-11 nem a AC-12.** Os critérios de
aceite da ARQ-447 são **dez**: nove deles remetem a um item de AC-11 ou AC-12 do
épico, e o décimo — «ADR-015 registrado com contexto, alternativas, trade-offs e
consequências» — é critério próprio da story, repetido no DoD dela como «ADR-015 com
status Aceito». Ele não fecha aqui: o acionamento é deste artefato, a redação e o
aceite são de FND-11 (P8). Registrá-lo é o que impede que a contagem de nove pareça
completa.
