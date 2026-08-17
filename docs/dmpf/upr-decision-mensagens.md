# UPR, Decision e modelo de mensagens — DMPF FND-03

| Campo | Valor |
|-------|-------|
| **Status** | `draft normativo` — promovido para revisão em PR |
| **Adiciona a** | RFC DMPF Foundation v0.1, pela âncora ANC-01 (RFC §12.3) |
| **Owner** | Mateus Macedo Dos Anjos (assignee de [ARQ-440](https://lider-cap.atlassian.net/browse/ARQ-440)) |
| **Épico** | [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436) — Golden Path para Sistemas Orientados a Domínio e Mensagens |
| **Story** | [ARQ-440](https://lider-cap.atlassian.net/browse/ARQ-440) (DMPF-FND-03) |
| **Spec** | [SPEC-8MNDEWDP](../specs/SPEC-8MNDEWDP-dmpf-upr-decision-mensagens.md) |
| **Data** | 2026-08-17 |
| **Revisão** | Plataforma e Arquitetura no PR; um representante por stack (Go, TypeScript) para os exemplos de §8 |

> **O que este documento obriga.** As regras rotuladas `normativo` valem para
> todo trabalho novo do DMPF, na mesma força da RFC à qual elas se adicionam.
> Nem todo bloco aqui obriga: §1.3 define as cinco categorias de conteúdo e a
> fronteira de cada uma. Enquanto o status for `draft normativo`, o documento
> está em revisão; a promoção ocorre no aceite do PR.

---

## §1. Fronteira, referência e sucessão

Esta seção vem antes de qualquer regra porque um artefato que adiciona a uma
norma compartilhada precisa dizer, primeiro, **até onde** ele pode obrigar. A
RFC já respondeu a essa pergunta ao registrar a âncora ANC-01; o que segue é a
leitura dessa autorização, a convenção de leitura do texto e o efeito deste
documento sobre a base conceitual a que ele sucede.

### §1.1 A autorização: âncora ANC-01

`normativo`

Este artefato **adiciona** à RFC DMPF Foundation v0.1 pela âncora ANC-01
(RFC §12.3). Ele não edita a RFC e não incrementa a versão dela: adição por
âncora dentro do escopo permitido é revisão em PR, sem incremento (RFC §14.2).
O conteúdo vive aqui, e a RFC o alcança pelo endereço estável da âncora
(RFC §12.1).

| Campo | Valor, conforme o registro de ANC-01 |
|-------|--------------------------------------|
| Escopo permitido | Definir a unidade de processamento, a forma da decisão e a taxonomia de mensagens **dentro** do bloco `domain library` |
| Invariantes intocáveis | P0-1 (domínio sem I/O); célula 5 e célula 6 de RFC §7.4; RFC §9 (domínio executável em memória) |
| Monotonicidade | M1–M4 (RFC §12.2): detalhar e restringir, nunca relaxar, revogar ou reinterpretar |
| Impacto de versão na RFC | Nenhum |
| ADR exigido | Não, salvo se propuser alteração de invariante — nenhuma regra deste artefato o faz |

`normativo` — **Precedência.** Onde este artefato e a RFC divergirem, prevalece
a RFC. Uma divergência não se resolve neste documento: por M4, ela indica que a
fronteira de RFC §1.4 mudou, e mudar fronteira é mudança de versão da RFC, com o
rito de RFC §14.2.

`rationale` — Declarar a precedência aqui, e não deixá-la implícita em M4, é
deliberado. Uma sub-spec extensa tende a ser lida isoladamente por quem
implementa; sem a cláusula no corpo do próprio texto, um leitor que encontrasse
conflito poderia razoavelmente supor que o documento mais específico vence. Ele
não vence.

### §1.2 Convenção de referência

`normativo`

Três documentos de numeração própria e sobreposta participam desta cadeia, então
toda referência é qualificada:

| Forma | Designa |
|-------|---------|
| `§N` sem prefixo | uma seção **deste artefato** |
| `RFC §N` | uma seção da RFC DMPF Foundation v0.1 |
| `Parte-1 §N` | um capítulo da base conceitual `Parte-1-conceitual.md` |

`normativo` — O prefixo nulo tem referente **local**: dentro deste documento,
`§N` é sempre uma seção deste documento. A convenção difere da de RFC §1.3, onde
o prefixo nulo designa a própria RFC. Não há ambiguidade entre as duas, porque
cada documento resolve o prefixo nulo em si mesmo e prefixa toda referência
externa. Uma citação a este artefato feita **de fora** dele usa o nome do
arquivo mais a seção.

`rationale` — A alternativa era adotar `FND-03 §N` para autorreferência e deixar
`§N` reservado à RFC em todo o corpus. Foi descartada por tornar ilegível a
navegação interna do documento, que é onde a maioria das referências ocorre: um
texto em que cada remissão à seção vizinha carrega um prefixo de sub-spec pesa
sem ganhar precisão. O custo da escolha adotada é uma regra de leitura a mais,
paga uma vez, aqui.

### §1.3 Classificação de força

`normativo`

A ANC-01 autoriza normatizar **dentro** do bloco `domain library`. Parte do que
esta entrega precisa apresentar não cabe nesse bloco: o fluxo do application
service, o ownership dos contratos de aplicação e a evolução dos contratos de
wire pertencem a outras donas (RFC §1.4). Apresentar esse conteúdo com força
normativa excederia o escopo da âncora e seria inválido por M4 — ainda que o
texto estivesse tecnicamente correto.

A solução é declarar, em cada regra, o **bloco** a que ela se aplica e a sua
**força**:

| Rótulo | Significado | Obriga? |
|--------|-------------|---------|
| `normativo` | Regra que este artefato estabelece dentro do bloco `domain library`, no escopo da ANC-01 | **Sim** |
| `recepcionado` | Conteúdo reproduzido da Parte-1 para dar contexto contíguo, sem força nova e sem reabertura | Não — a força permanece a da Parte-1 |
| `encaminhado` | Assunto de outra sub-spec, citado apenas como fronteira rotulada, com dona explícita | Não — passa a obrigar quando a dona normatizar |
| `rationale` | Justificativa de uma decisão normativa, incluindo alternativas descartadas | Não |
| `registro` | Acionamento de ADR (§9) e matriz de rastreabilidade (§10) | Não |

`normativo` não é o rótulo default: um bloco sem rótulo é prosa de ligação e não
obriga nada. Os rótulos `normativo` e `rationale` têm aqui exatamente o sentido
de RFC §1.1. `recepcionado` usa o verbo no mesmo sentido de RFC §1.3;
`encaminhado` nomeia o gesto que RFC §1.4 pratica sem nomear.

`rationale` — Sem essa classificação, o documento teria de escolher entre duas
falhas: omitir o application service e a evolução de contratos, deixando a UPR
descrita sem a sua contraparte — o que a própria story pede para evitar —, ou
descrevê-los como norma e violar M4. A classificação por regra permite
apresentar o quadro inteiro sem que este artefato obrigue além da sua âncora.

### §1.4 O que este artefato não normatiza

`normativo` — espelha RFC §1.4 no recorte que toca a UPR.

Cada tema abaixo aparece neste documento **apenas** como fronteira rotulada
`encaminhado`. Onde ele aparece, o texto diz o que a UPR ou a mensagem faz até a
fronteira, e para: o outro lado é da dona.

| Tema | Dona | Fronteira aparece em |
|------|------|----------------------|
| Unit of Work, inbox, outbox e relay | FND-04 ([ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)) | §4, §8 |
| Protobuf, CloudEvents, OpenAPI e AsyncAPI; codec, registry e versionamento de wire | FND-05 ([ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442)) | §5, §6 |
| Políticas por transporte | FND-06 ([ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443)) | §6 |
| Contexto de execução, taxonomia de erros e segurança | FND-07 ([ARQ-444](https://lider-cap.atlassian.net/browse/ARQ-444)) | §2, §4 |
| Resiliência e observabilidade | FND-08 ([ARQ-445](https://lider-cap.atlassian.net/browse/ARQ-445)) | §2 |
| Testes e interoperabilidade entre stacks | FND-09 ([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)) | §3, §10 |
| Redação, promoção e aceite dos ADRs | FND-11 ([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)) | §9 |

Fora do épico inteiro, por P0-4: kernels Go e TypeScript, adapters e providers
de produção. Este artefato especifica a forma da unidade de processamento e das
mensagens; construí-la em cada stack é dos épicos de kernel.

Fora por natureza: a **modelagem de domínio** de cada bounded context. O
documento normatiza a forma, não o conteúdo do domínio de ninguém — quais
agregados existem, quais invariantes de negócio valem e quais eventos importam
é decisão de cada squad.

### §1.5 Sucessão sobre a Parte-1

`normativo` — detalha RFC §1.7 e continua RFC §14.4 no recorte da ANC-01.

RFC §14.4 registra que as decisões de UPR e de contratos "seguem vigentes na
Parte-1 até as sub-specs correspondentes". Esta é a sub-spec correspondente. A
tabela abaixo declara o estado de cada capítulo da Parte-1 no escopo desta
âncora:

| Capítulo da Parte-1 | Estado |
|---------------------|--------|
| Parte-1 §5 — UPR, Decision e ciclo de vida | **Consolidado** em §2, §3 e §4 — a Parte-1 deixa de ser fonte naquilo que este artefato decide |
| Parte-1 §6 — Mensagens, contratos e níveis | **Consolidado** em §5 e §6 |
| Parte-1 §9.1 — Fluxo canônico de escrita | **Recepcionado** em §4, sem consolidação — o fluxo descreve o application service, cuja parte transacional é de FND-04 |
| Demais capítulos | **Vigentes** na Parte-1, cada um com âncora própria em RFC §12.3 |

`normativo` — Não há revogação implícita. Um capítulo da Parte-1 fora da tabela
acima continua valendo como base conceitual.

`normativo` — Dentro do escopo consolidado, onde este artefato e a Parte-1
divergirem, prevalece este artefato. A divergência material está em §3: a
Parte-1 modela o desfecho da UPR como produto de sucesso com braço de erro
separado, e este artefato normatiza a união exaustiva. A reconciliação é feita
no próprio §3, não por revogação silenciosa.

`rationale` — **Sobre a literalidade de "esta tabela".** RFC §14.4 fecha
dizendo que um capítulo da Parte-1 "só deixa de valer quando esta tabela o
declarar consolidado", e "esta tabela" é a de RFC §14.4. Duas leituras cabem: a
consolidação exigiria editar aquela tabela, ou a cláusula de sucessão vive no
artefato sucessor autorizado pela âncora. Adotou-se a segunda, porque a primeira
colide com RFC §12.1 e RFC §14.2, que determinam adição por âncora **sem**
editar a RFC — a primeira leitura tornaria toda sucessão uma mudança de versão,
esvaziando o mecanismo. A escolha está sinalizada aos revisores no PR desta
entrega. Se prevalecer a leitura literal, a atualização de RFC §14.4 é escalada
como alteração própria, com o rito de versão que ela exigir. Não é contornada
aqui.

`registro` — **Pendência aberta: atualizar a tabela de RFC §14.4.** A revisão
externa desta entrega classificou a questão como material, e com razão: enquanto
a tabela da RFC listar Parte-1 §§5 e 6 como vigentes, dois modelos de desfecho
coexistem formalmente, e quem implementa não tem como saber qual seguir a partir
da RFC sozinha. Fica registrado, no mesmo estatuto da pendência de §10.4:

| Item | Estado | Destino |
|------|--------|---------|
| Marcar Parte-1 §§5 e 6 como consolidados na tabela de RFC §14.4 | pendente | Revisores desta entrega, no PR; se exigir rito de versão, vira alteração própria da RFC |

Até que isso ocorra, a sucessão declarada acima vale por autorização da ANC-01, e
a divergência é conhecida — não silenciosa.

---

## §2. A unidade de processamento de requisição

### §2.1 Definição e assinatura conceitual

`normativo` — bloco `domain library`.

A **unidade de processamento de requisição** (UPR) é a menor unidade explícita
de comportamento de domínio: recebe uma mensagem de domínio semanticamente
tipada e o estado sobre o qual ela incide, e devolve o desfecho da decisão.

```text
UPR(estado do agregado, requisição de domínio) -> Decision
```

`Decision` é a união exaustiva definida em §3. A assinatura é **conceitual**:
ela fixa o que entra, o que sai e — sobretudo — o que não aparece. Não há canal
de contexto, de cancelamento, de deadline nem de falha técnica. A realização em
cada linguagem é decisão de kernel, sob a equivalência observável de §3.4.

`normativo` — Três propriedades definem a unidade. A ausência de qualquer uma
delas descaracteriza o que se está chamando de UPR:

| Propriedade | Enunciado | Onde é detalhada |
|-------------|-----------|------------------|
| **Síncrona** | A decisão se completa na própria chamada. O desfecho não é prometido para depois, e a assinatura não expõe canal de espera | §2.3 (`UPR-L05`) |
| **Determinística** | Fixados requisição e estado de entrada, o desfecho é sempre o mesmo | §2.4 |
| **Sem I/O** | Nenhuma leitura ou escrita fora da memória do processo, direta ou por dependência | §2.2 (`UPR-I07`) |

`normativo` — Ser síncrona é propriedade da **unidade de domínio**, não do caso
de uso que a contém: o application service em torno dela é tipicamente
assíncrono, e isso é esperado (§4). Uma assinatura de UPR que devolva valor
diferido é sintoma, não estilo — indica que I/O atravessou a fronteira e que
`UPR-I07` está violada.

`recepcionado` — Parte-1 §5.1 enuncia a mesma unidade, com a assinatura
`UPR(estado, requisição) -> decisão(resposta, eventos) | erro de domínio`. Este
artefato consolida aquele capítulo (§1.5): o braço `| erro de domínio` é
substituído pela união de §3, e o restante da definição é recepcionado sem
alteração.

`normativo` — **Nomenclatura.** "UPR" designa exclusivamente a unidade de
domínio definida aqui. Um handler de aplicação, um controller HTTP ou um
consumer de mensageria **não** são UPR, ainda que cada um orqueste uma. O termo
fica reservado: aquilo que carrega estado, abre transação ou recebe contexto de
transporte não é uma UPR, e chamá-lo assim é uso incorreto do termo — em
qualquer documento, código ou API que adote este vocabulário.

`encaminhado` — A nomenclatura da API pública dos kernels Go e TypeScript é dos
épicos de kernel, que P0-4 mantém fora do escopo normativo desta fundação. O que
este artefato fixa é o **significado do termo**, que é definição, não obrigação
sobre uma API que ele não pode normatizar.

`rationale` — A vedação de nome vem de Parte-1 §5.2, e é mais que higiene de
vocabulário: se "UPR" passa a nomear qualquer handler, as doze invariantes de
§2.2 perdem sujeito determinado e deixam de ser verificáveis. A regra protege a
verificabilidade, não a estética.

### §2.2 As doze invariantes

`normativo` — bloco `domain library`. Fonte: Parte-1 §5.1, aqui derivada em
regras individuais e tornada verificável. O rótulo do bloco é `normativo`, e não
`recepcionado`, justamente porque a força é nova: a fonte enuncia propriedades
em prosa, e este artefato as transforma em regras com ID e modo de verificação.

Parte-1 §5.1 lista doze propriedades em prosa corrida. Aqui elas recebem
identificador estável, enunciado individual e **modo de verificação**, para
satisfazer a exigência de invariantes "listáveis e verificáveis". Os modos são
os três canônicos de RFC §2.2, usados sem alteração de sentido:
`import-verifiable` (decidível na análise estática do grafo de imports),
`structurally reviewable` (decidível pela inspeção da estrutura, sem executar) e
`runtime-testable` (só demonstrável executando).

| ID | Invariante | Modo | Ancoragem |
|----|------------|------|-----------|
| `UPR-I01` | Recebe uma mensagem de domínio semanticamente tipada | `structurally reviewable` | Parte-1 §5.1 |
| `UPR-I02` | Opera sobre uma entidade ou agregado identificado | `structurally reviewable` | Parte-1 §5.1 |
| `UPR-I03` | Protege as invariantes de negócio do alvo | `runtime-testable` | Parte-1 §5.1 |
| `UPR-I04` | **Pode** alterar o estado interno do alvo | — (permissão) | Parte-1 §5.1 |
| `UPR-I05` | Devolve um desfecho explícito, nunca implícito | `runtime-testable` | §3 |
| `UPR-I06` | Declara no desfecho os eventos de domínio produzidos | `runtime-testable` | Parte-1 §5.3 |
| `UPR-I07` | Não executa I/O | `import-verifiable` + `runtime-testable` | P0-1; célula 5 de RFC §7.4; RFC §9.1 |
| `UPR-I08` | Não abre nem delimita transação | `import-verifiable` | RFC §7.5; ANC-02 |
| `UPR-I09` | Não publica eventos | `import-verifiable` | P0-1; RFC §7.5 |
| `UPR-I10` | Não escreve logs nem emite telemetria | `import-verifiable` | RFC §6.2; princípio 11 |
| `UPR-I11` | Não recebe nem produz tipos gerados de Protobuf | `import-verifiable` | P0-2; célula 6 de RFC §7.4 |
| `UPR-I12` | Não depende do contexto de transporte | `import-verifiable` + `structurally reviewable` | Parte-1 §5.1; RFC §1.4 |

`normativo` — `UPR-I04` é **permissão, não obrigação**. Uma UPR que apenas
decide, sem alterar estado, é conforme; a leitura de que toda UPR precisa mutar
o alvo é incorreta e produziria mutação decorativa em decisões de consulta.

`normativo` — A violação de qualquer uma de `UPR-I07` a `UPR-I12` descaracteriza
a unidade: o que viola não é "uma UPR com defeito", é código de aplicação
alojado no bloco errado, e a correção é movê-lo, não relaxar a invariante.

`rationale` — Separar as seis primeiras das seis últimas não é ornamento. As
positivas descrevem o que a unidade **faz** e dependem de julgamento sobre o
domínio de cada squad — nenhum linter decide se um tipo é "semanticamente
tipado". As negativas descrevem o que ela **não pode alcançar** e, salvo
`UPR-I12`, são decidíveis no grafo de imports. Misturá-las numa lista única, como
na Parte-1, esconde que metade da lista é mecanizável e a outra metade é rito de
revisão.

`encaminhado` — Os códigos de diagnóstico que um verificador emite para cada
invariante `import-verifiable`, e os vetores positivos e negativos pareados
Go/TypeScript que a cadeia de RFC §14.5 exige, pertencem a FND-09
([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)). §10 registra a
pendência.

### §2.3 Ciclo de vida

`normativo` — bloco `domain library`. **Decisão nova**: Parte-1 §5 trata o ciclo
de vida da UPR apenas por implicação.

| ID | Regra |
|----|-------|
| `UPR-L01` | A UPR não retém estado próprio entre execuções; todo estado sobre o qual ela decide chega como parâmetro |
| `UPR-L02` | O escopo de vida de uma execução é a própria chamada: começa ao receber estado e requisição, termina ao devolver o desfecho |
| `UPR-L03` | A UPR não adquire nem descarta recurso — nada que precise ser fechado, liberado ou drenado |
| `UPR-L04` | Duas execuções de UPR não compartilham estado mutável |
| `UPR-L05` | A UPR não agenda trabalho que sobreviva ao retorno: sem callback pendente, sem continuação, sem execução concorrente deixada em curso |

`normativo` — As regras acima restringem a **unidade**, não a sua forma de
realização. Função livre, método de agregado e objeto sem campos mutáveis
satisfazem todas as cinco. O que nenhuma realização pode fazer é manter, entre
chamadas, estrutura mutável alcançável pela decisão.

`rationale` — Poderia parecer que `UPR-I07` (sem I/O) já implica tudo isto: sem
I/O, o que sobraria para reter? Sobra bastante, e nada disso toca I/O — um cache
de resultados em memória, um contador de execuções, um pool de objetos
reaproveitados, uma referência retida ao último agregado processado. Cada um
deles passa por qualquer verificação de imports e quebra o determinismo de §2.4
ou vaza estado entre casos de uso não relacionados. A lacuna é real, e por isso
o ciclo de vida é normatizado aqui em vez de deixado por implicação.

`encaminhado` — O ciclo de vida do **application service** — escopo por
requisição, delimitação da Unit of Work, propagação de contexto e cancelamento —
é de FND-04 ([ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)) na parte
transacional e de FND-07
([ARQ-444](https://lider-cap.atlassian.net/browse/ARQ-444)) na parte de
contexto. §4 apresenta a fronteira entre os dois sem normatizar o outro lado.

### §2.4 Determinismo

`normativo` — bloco `domain library`. Deriva de Parte-1 §5.5; ancora em
RFC §9.3.

**Enunciado.** Fixados a requisição e o estado de entrada, duas execuções da UPR
produzem o mesmo desfecho: a mesma variante de `Decision`, a mesma resposta e a
**mesma sequência** de eventos de domínio, na mesma ordem.

`normativo` — A UPR não obtém tempo, identidade nem aleatoriedade por conta
própria. Cada uma dessas necessidades chega resolvida:

| A decisão precisa de | Como chega |
|----------------------|------------|
| Instante | Valor de entrada, resolvido pelo application service antes da chamada |
| Identificador novo | Valor já gerado, ou derivado por política pura sobre a entrada |
| Aleatoriedade | Semente ou valor sorteado, recebido como entrada |
| Dado de outro agregado | Já carregado pela camada de aplicação e passado como entrada |
| Configuração | Parâmetro de valor, resolvido pelo `app` |

`recepcionado` — a tabela acima é RFC §9.3 restrita ao caso da UPR; Parte-1 §5.5
descreve o mesmo gesto pelo lado do service, que consulta `Clock` e
`IdGenerator`, converte os resultados em valores de domínio e só então chama a
UPR.

`normativo` — Nenhuma linha dessa tabela justifica uma porta no domínio. Uma UPR
que receba `Clock` ou `IdGenerator` como dependência — ainda que sob interface,
ainda que substituível em teste — viola `UPR-I07`, e RFC §9.2 já resolve o
argumento: a dependência é a violação, não a ausência do duplo.

`normativo` — **Fronteira do determinismo.** O determinismo é exigido da UPR,
não do caso de uso. Duas execuções do mesmo caso de uso podem divergir
legitimamente, porque o instante resolvido mudou, o identificador gerado é
outro, ou o estado carregado do repositório evoluiu entre elas. O que a norma
exige é que, fixadas as entradas, a UPR não introduza variação própria.

`rationale` — A ordem dos eventos entra no enunciado deliberadamente. Sem ela,
duas realizações da mesma regra poderiam emitir o mesmo conjunto de eventos em
ordens diferentes e ambas se declararem conformes — e a fixture de
interoperabilidade que compara as duas stacks não teria critério para decidir. O
custo da escolha é proibir realização que acumule eventos em estrutura não
ordenada; o benefício é que "mesmo desfecho" passa a ter significado
comparável entre Go e TypeScript.

`encaminhado` — Os vetores que exercitam determinismo em ambas as stacks são de
FND-09 ([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)). §8 traz o
exemplo de dupla execução em pseudocódigo; §10 registra a pendência de handoff.

---

## §3. O desfecho da UPR

### §3.1 O desfecho é uma união exaustiva

`normativo` — bloco `domain library`.

Toda execução de UPR termina em exatamente um de dois desfechos:

```text
Decision<Response, DomainEvent> =
  | Accepted(response: Response, events: DomainEvent[])
  | Rejected(rejection: Rejection)
```

| ID | Regra |
|----|-------|
| `DEC-01` | **Exaustividade.** Não há terceiro desfecho. Ausência de retorno, valor nulo e desfecho indeterminado não são conformes |
| `DEC-02` | **Exclusividade.** Os dois ramos se excluem; não existe desfecho parcialmente aceito |
| `DEC-03` | A rejeição de negócio é **dado de domínio tipado** e chega ao chamador pelo próprio desfecho |
| `DEC-04` | Nenhuma rejeição de negócio atravessa a fronteira observável por exceção não classificada, panic, ou canal indistinguível de falha técnica |

`normativo` — **Fronteira observável.** É o ponto em que o chamador da UPR — o
application service — recebe o desfecho. As quatro regras acima incidem sobre o
que se observa nesse ponto, e não sobre o caminho interno que o kernel percorre
para chegar lá. §3.4 trata da realização.

`rationale` — `DEC-04` é a cláusula que impede a norma de ser satisfeita apenas
na aparência. Sem ela, uma realização poderia declarar-se conforme por
"devolver uma rejeição tipada" enquanto, na prática, a entrega ocorre por um
mecanismo que o chamador não consegue separar de uma falha de infraestrutura —
e a exaustividade de `DEC-01`, que é o ganho real da união, se perderia no
consumidor.

### §3.2 O ramo `Accepted`

`normativo` — bloco `domain library`.

| ID | Regra |
|----|-------|
| `DEC-05` | Carrega uma resposta de domínio **explícita**. Uma decisão sem resposta útil carrega resposta explicitamente vazia, nunca resposta ausente |
| `DEC-06` | Carrega a **sequência ordenada** de eventos de domínio produzidos, possivelmente vazia |
| `DEC-07` | Os eventos são **declarados no desfecho**. O chamador não os descobre inspecionando o agregado |
| `DEC-08` | Um mesmo evento não é entregue por dois mecanismos: se um kernel oferecer coleta de pendentes por compatibilidade, o evento aparece em um caminho, nunca nos dois |

`recepcionado` — Parte-1 §5.3 fundamenta `DEC-07` e `DEC-08`: a decisão
explícita torna os efeitos observáveis no retorno, evita que o service precise
descobrir eventos pendentes, reduz o risco de salvar o agregado e esquecer a
outbox, simplifica testes e preserva a exigência de "mensagem entra, mensagem
sai".

### §3.3 O ramo `Rejected` e sua pós-condição

`normativo` — bloco `domain library`.

| ID | Regra |
|----|-------|
| `DEC-09` | Carrega uma rejeição tipada, com código estável de domínio |
| `DEC-10` | **Pós-condição de estado.** Após um desfecho `Rejected`, o estado observável do alvo é idêntico ao de antes da chamada. Nenhuma mutação parcial sobrevive à recusa |
| `DEC-11` | **Sem eventos.** `Rejected` não carrega eventos de domínio, e não existe event bag oculto guardando algum. Se a regra recusou, nada aconteceu no domínio |

`normativo` — Conteúdo mínimo da rejeição, do lado do domínio:

| Campo | Obrigatório | Observação |
|-------|-------------|------------|
| Código estável | sim | Identificador de domínio, no formato `contexto/motivo` (exemplo: `orders/empty-order`) |
| Mensagem | sim | Endereçada ao domínio, sem dado sensível e sem detalhe técnico |
| Detalhes estruturados | não | Quando presentes, expressos em tipos de domínio |

`normativo` — A rejeição de domínio **não** carrega código HTTP, código gRPC,
política de retry nem causa técnica encadeada. Os três primeiros são mapeamento
de borda; o quarto não existe, porque não há falha técnica originada numa
unidade que não faz I/O.

`rationale` — A pós-condição `DEC-10` precisa ser dita porque "não persistir" e
"não mutar" não são a mesma coisa. Uma realização poderia alterar o agregado,
descobrir a violação de invariante tarde e devolver `Rejected` com o alvo já
sujo em memória. O application service não persistiria — mas o objeto continua
corrompido no escopo do caso de uso e pode ser lido de novo antes do descarte. A
regra converte uma garantia da camada de persistência em garantia da própria
decisão.

`rationale` — `DEC-11` fecha a porta simétrica: sem ela, uma realização poderia
recusar a requisição e ainda assim registrar um evento "tentativa recusada" no
agregado, que o service coletaria e gravaria na outbox. O efeito seria publicar
consequência de algo que o domínio decidiu não fazer. Quando a recusa precisa
ser comunicada para fora, isso ocorre por uma mensagem de rejeição na fronteira
de aplicação (§5), não por evento de domínio.

### §3.4 Realização por kernel e equivalência observável

`normativo` — bloco `domain library`.

A **forma** do desfecho é normativa. O **mecanismo** de realização é decisão de
cada kernel, e nenhuma construção sintática é imposta.

`normativo` — A tabela abaixo fixa o que um observador na fronteira precisa
obter. Ela é condição **necessária**, e não suficiente: satisfazê-la não dispensa
nenhuma das demais regras de §3 — em particular `DEC-08` (o mesmo evento não
chega por dois caminhos), `DEC-11` (não existe coleção pendente guardando evento
após recusa) e `DEC-12`/`DEC-13` (o desfecho não oferece meio de ser alterado
depois de produzido). Uma realização que passe em todas as linhas e viole
qualquer uma dessas regras não é conforme.

| Observação na fronteira | Em `Accepted` | Em `Rejected` |
|-------------------------|---------------|---------------|
| O ramo é distinguível sem inspecionar texto de mensagem | sim | sim |
| Resposta de domínio acessível | sim | não se aplica |
| Sequência ordenada de eventos acessível | sim | vazia, por `DEC-11` |
| Rejeição tipada com código estável acessível | não se aplica | sim |
| Estado observável do alvo | pode ter mudado | inalterado, por `DEC-10` |
| Chegada por canal indistinguível de falha técnica | proibido | proibido |
| Chegada por lançamento que interrompe o fluxo do chamador | proibido | proibido |
| Mesmo evento acessível por um segundo caminho (`DEC-08`) | proibido | não se aplica |
| Coleção pendente retendo evento após o retorno (`DEC-11`) | proibido | proibido |
| Meio de alterar o desfecho depois de produzido (`DEC-12`, `DEC-13`) | proibido | proibido |
| Exaustividade verificável no chamador | sim | sim |

`normativo` — Satisfazem a tabela, entre outras: união discriminada; par de
valores com discriminante explícito; tipo de resultado com ramo de erro tipado
restrito a rejeições de domínio. **Não** satisfazem: rejeição entregue por
lançamento, ainda que a exceção seja tipada e classificada; rejeição entregue
pelo mesmo canal por onde trafegam falhas de infraestrutura; ausência de retorno
com o desfecho consultado depois, em estado à parte.

`normativo` — A vedação ao lançamento vale para as duas stacks e não admite a
leitura de que "tipar a exceção resolve". A spec decide que a rejeição de
negócio é **valor devolvido**, e uma exceção classificada continua interrompendo
o fluxo do chamador em vez de lhe entregar um desfecho — a exaustividade de
`DEC-01` deixa de ser verificável no ponto da chamada. O que distingue o par
`(decisão, erro)` disso não é a palavra-chave, e sim o gesto: ele **devolve**,
e o chamador precisa olhar os dois valores para saber o que aconteceu.

`rationale` — **Por que um par `(decisão, erro)` em Go pode ser conforme e um
lançamento genérico em TypeScript não.** A objeção imediata é que ambos usam o
"canal de erro" da linguagem, e que aprovar um e reprovar o outro seria
arbitrário. A diferença está no que mais trafega por esse canal. Como a UPR não
executa I/O (`UPR-I07`), não existe falha técnica **originada** dentro dela:
timeout, indisponibilidade e falha de serialização acontecem fora, no service e
nos adapters. O canal de erro de uma UPR em Go transporta, portanto, apenas
rejeições de domínio, e o chamador as distingue por tipo ou valor sentinela — a
exaustividade continua verificável. Já um lançamento de exceção não classificada
mistura a rejeição de negócio com o erro de programação, que Parte-1 §13 já
separa ao registrar que panic e exceções não classificadas "representam falhas
inesperadas, não rejeições normais de negócio"; nesse caso o chamador só
separaria os dois inspecionando texto. A norma incide sobre a
**distinguibilidade** do desfecho, não sobre a palavra-chave que o transporta.

`rationale` — A alternativa descartada era impor a união discriminada como
mecanismo, e não apenas como forma. Ela daria fidelidade máxima ao enunciado da
spec, ao custo de contrariar a premissa do épico de que a paridade entre stacks
é conceitual e não sintática — e de antecipar, aqui, uma decisão que pertence
aos épicos de kernel (P0-4). A alternativa oposta, seguir a realização atual dos
drafts das duas stacks e tratar o desfecho como produto com a rejeição fora,
foi descartada por contradizer as decisões técnicas da spec aprovada.

`encaminhado` — Os vetores que exercitam os dois ramos em Go e TypeScript,
comprovando que as duas realizações produzem observações equivalentes, são de
FND-09 ([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)). Esse
contrato **ainda não existe** naquela sub-spec: §10 registra a pendência de
handoff em vez de pressupor a cobertura.

### §3.5 Imutabilidade do desfecho

`normativo` — bloco `domain library`. **Decisão nova**: Parte-1 §5.3 não afirma
imutabilidade.

| ID | Regra |
|----|-------|
| `DEC-12` | O desfecho é imutável: depois de produzido, seu conteúdo não muda, e a realização não oferece operação que permita alterá-lo |
| `DEC-13` | A sequência de eventos é **fechada no retorno**: o desfecho não expõe meio de acrescentar, reordenar ou filtrar eventos depois de produzido |

`rationale` — A norma é escrita como propriedade de **comportamento**, não como
exigência de mecanismo, porque a garantia mecânica não está disponível nas duas
stacks em pé de igualdade: uma delas oferece congelamento de estrutura, a outra
trata imutabilidade como convenção sustentada por cópia. Exigir a palavra-chave
excluiria uma stack; exigir o comportamento vincula as duas.

`normativo` — As duas regras incidem sobre o **desfecho produzido pelo domínio**:
o que ele é e o que ele não oferece. Elas não obrigam quem o recebe.

`encaminhado` — O que o chamador faz com o desfecho — ler, converter, encaminhar,
persistir — é da camada que o consome, e a parte transacional disso é de FND-04
([ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)). Este artefato fixa
que o desfecho **não dá meios** de ser adulterado; não pode obrigar o consumidor
a se comportar de determinada forma, porque isso está fora da ANC-01.

### §3.6 Três nomes próximos, três conceitos distintos

`normativo` — desambiguação. A proximidade dos nomes é fonte previsível de erro
de implementação, e os três aparecem no mesmo caso de uso.

| Nome | O que é | Onde vive | Dona |
|------|---------|-----------|------|
| `Rejected` | Variante do desfecho da UPR | Domínio, no retorno da UPR | §3 |
| `Rejection` | Tipo de mensagem: resultado negativo esperado e tipado | Domínio e fronteira de aplicação | §5 |
| `DomainRejection` | Categoria na taxonomia de erros, com mapeamento de protocolo | Borda de aplicação | FND-07 ([ARQ-444](https://lider-cap.atlassian.net/browse/ARQ-444)) |

Como os três se encadeiam: a UPR devolve `Rejected`, carregando uma `Rejection`.
Quando o caso de uso precisa comunicar esse desfecho para fora do processo, a
borda o classifica como `DomainRejection` e aplica o mapeamento do protocolo em
uso.

`recepcionado` — Parte-1 §13 registra `DomainRejection` como não-retryable, com
mapeamento para 422 em HTTP, `FAILED_PRECONDITION` em gRPC e ACK ou evento de
rejeição em mensageria. Reproduzido aqui sem força nova: a taxonomia de erros e
o seu mapeamento são de FND-07.

`normativo` — O domínio não conhece 422. A UPR não referencia código de status,
código de protocolo nem política de retry (`UPR-I12`).

---

## §4. UPR e application service

Esta seção descreve uma fronteira, e uma fronteira tem dois lados. Só um deles
cabe na ANC-01. O que segue normatiza o lado do domínio — o que a UPR recebe,
o que ela devolve e o que ela nunca alcança — e apresenta o outro lado como
contexto rotulado, para que a fronteira seja legível sem que este artefato
obrigue além da sua âncora (§1.3).

### §4.1 A fronteira, pelo lado do domínio

`normativo` — bloco `domain library`.

| ID | Regra |
|----|-------|
| `FRT-01` | A UPR recebe o estado já carregado. Não busca, não consulta repositório e não conhece a origem do estado |
| `FRT-02` | A UPR devolve o desfecho e termina. Não persiste, não confirma transação e não publica |
| `FRT-03` | A UPR não recebe contexto de execução, deadline nem sinal de cancelamento |
| `FRT-04` | A UPR não orquestra caso de uso: não sequencia passos de aplicação nem invoca UPR de outro agregado |

`recepcionado` — RFC §4.1 já estabelece os dois lados: UPRs e eventos de
domínio pertencem ao bloco `domain library`, e o application service orquestra
o caso de uso **sem conter regra de negócio**. Reproduzido aqui para
contiguidade de leitura, sem força nova — a regra é da RFC.

### §4.2 O fluxo canônico de escrita

`recepcionado` — Parte-1 §9.1. O fluxo descreve o application service; este
artefato o reproduz para localizar a UPR dentro dele, e classifica a força de
cada passo.

| # | Passo | Bloco | Força neste artefato |
|---|-------|-------|----------------------|
| 1 | Validar autorização de aplicação | application service | `encaminhado` — FND-07 |
| 2 | Resolver tempo e identificadores | application service | `normativo` **apenas** no efeito sobre a UPR: os valores chegam resolvidos (§2.4) |
| 3 | Iniciar Unit of Work | application service | `encaminhado` — FND-04 |
| 4 | Carregar ou criar o agregado | application service | `normativo` **apenas** no efeito sobre a UPR: o estado chega carregado (`FRT-01`) |
| 5 | **Executar a UPR** | `domain library` | `normativo` — §2 e §3, integralmente |
| 6 | Persistir o agregado com optimistic locking | provider, acionado pelo service | `encaminhado` — FND-04 |
| 7 | Mapear domain events para integration events | fora do domínio; o bloco é decidido por FND-04 e FND-05, dentro da matriz de RFC §7.4 | `normativo` **apenas** na regra de conversão (§6.3); o bloco e o mecanismo são `encaminhado` |
| 8 | Serializar e inserir na outbox, na mesma transação | application service e provider | `encaminhado` — FND-04 (atomicidade) e FND-05 (serialização) |
| 9 | Confirmar | application service | `encaminhado` — FND-04 |
| 10 | Devolver a resposta de aplicação | application service | `encaminhado` — o contrato de aplicação aparece em §6 |

`normativo` — Deste fluxo, apenas o **passo 5** ocorre no bloco
`domain library`, e apenas ele é normatizado aqui em sua totalidade. Os passos
2, 4 e 7 são normatizados somente naquilo que restringem a UPR: o que ela
recebe e o que ela produz. Os demais passos aparecem como fronteira rotulada e
não obrigam por este documento.

`rationale` — Reproduzir o fluxo inteiro e normatizar um passo só parece
desequilibrado, e é deliberado. A story pede que UPR e application service
tenham fronteira sem zona cinzenta, o que exige mostrar os dez passos; a
ANC-01 autoriza normatizar dentro do `domain library`, o que restringe a
obrigação a um. A coluna de força é o que permite entregar as duas coisas ao
mesmo tempo, em vez de escolher entre um documento incompleto e uma adição
inválida por M4.

### §4.3 Quem faz o quê

`recepcionado` — Parte-1 §5.2, com a coluna de força acrescentada e uma linha
de ciclo de vida que a fonte não tinha.

| Dimensão | UPR de domínio | Application service | Força |
|----------|----------------|---------------------|-------|
| Execução | Síncrona e determinística | Normalmente assíncrono | `normativo` na coluna da UPR (§2.1) |
| Estado | Opera sobre estado em memória | Carrega e persiste | `normativo` na coluna da UPR (`FRT-01`) |
| Papel | Protege invariantes de negócio | Orquestra o caso de uso | `recepcionado` — RFC §4.1 |
| Contexto | Não recebe contexto de I/O | Recebe contexto, deadline e cancelamento | `normativo` na coluna da UPR (`FRT-03`); a outra coluna é `encaminhado` a FND-07 |
| Transação | Não inicia nem delimita | Delimita a Unit of Work | `normativo` na coluna da UPR (`UPR-I08`); a outra é `encaminhado` a FND-04 |
| Saída | Produz decisão de domínio | Produz resposta de aplicação | `normativo` na coluna da UPR (§3); a outra aparece em §6 |
| Outbox | Não conhece | Converte eventos e grava | `normativo` na coluna da UPR (`UPR-I09`); a outra é `encaminhado` a FND-04 |
| Ciclo de vida | Sem estado entre execuções; escopo da chamada | Escopo do caso de uso | `normativo` na coluna da UPR (§2.3); a outra é `encaminhado` a FND-04 |
| Teste | Executa em memória, sem mock de infraestrutura | Exercitado com portas substituídas | `normativo` na coluna da UPR (`UPR-I07`, modo `runtime-testable` em §2.2); a outra é `encaminhado` a FND-09 |

### §4.4 Casos que costumam confundir

`normativo` — bloco `domain library`, quanto ao veredito de cada caso que
recai sobre a UPR.

A story exige fronteira "sem zona cinzenta nos casos analisados". Os casos
abaixo são os que produzem dúvida recorrente; cada um recebe veredito e o
critério que o sustenta.

| Caso | Onde vive | Critério |
|------|-----------|----------|
| Validação de forma da entrada — campo obrigatório, formato sintático | Fronteira de aplicação | Não é regra de negócio; a UPR recebe mensagem já estruturalmente válida (`UPR-I01`) |
| Validação que depende do estado do agregado — limite excedido, item duplicado | **UPR** | É invariante de negócio (`UPR-I03`) e exige o estado para decidir |
| Autorização "este usuário tem permissão de executar a operação" | Application service | Passo 1 do fluxo; controle de acesso, `encaminhado` a FND-07 |
| Autorização "este titular pode movimentar este saldo" | **UPR** | É regra do domínio expressa sobre o estado do agregado, não controle de acesso |
| Decisão que depende de dado de outro agregado | **UPR**, com o dado recebido | RFC §9.3: o dado chega carregado; a UPR não o busca (`FRT-01`) |
| Decisão que depende de consulta a serviço externo | Não é UPR | Exige I/O; a consulta ocorre antes, e só o resultado entra como valor |
| Cálculo puro complexo — política de preço, tarifação, elegibilidade | **Domínio** | RFC §5.2: policies computacionais são `domain`, não `port` |
| Sequenciar duas decisões sobre agregados distintos | Application service | `FRT-04`: coordenação é orquestração |
| Emitir evento de integração | Application service | A UPR declara evento de domínio (§3); a conversão é de §6 |
| Registrar métrica de negócio | Application service | `UPR-I10`: o domínio não emite telemetria, nem quando o indicador é "de negócio" |

`rationale` — Os dois primeiros pares da tabela são escolhidos de propósito: em
cada um, a mesma palavra — "validação", "autorização" — nomeia duas coisas que
vivem em blocos diferentes, e é daí que nasce a maior parte da zona cinzenta
observada. O critério que separa não é o vocabulário: é a dependência. O que
precisa olhar o estado do agregado para decidir é domínio; o que decide sem
olhar o agregado é aplicação. O último caso da tabela é incluído porque
"métrica de negócio" soa como domínio e não é: a natureza do indicador não
altera o fato de que emiti-lo é efeito observável fora do processo.

---

## §5. Taxonomia de mensagens

### §5.1 Os sete tipos e onde cada um vive

`recepcionado` — Parte-1 §6.1, com a coluna de bloco e a de força acrescentadas.

| Tipo | Semântica | Bloco onde o tipo vive | Força neste artefato |
|------|-----------|------------------------|----------------------|
| **Command** | Solicita que uma ação seja tentada | Domínio, como requisição da UPR; aplicação, como pedido do caso de uso | `normativo` no nível de domínio |
| **Query** | Solicita uma visão ou informação | Domínio ou aplicação, conforme quem responde | `normativo` no nível de domínio |
| **Application response** | Resultado de um caso de uso | Aplicação | `encaminhado` — contrato em §6; mapeamento de borda em FND-07 |
| **Domain event** | Fato interno já ocorrido no bounded context | `domain library` | `normativo` |
| **Integration event** | Fato público versionado para outros contextos | `contract package` | `encaminhado` — FND-05 |
| **Rejection** | Resultado negativo esperado e tipado | Domínio (§3) e fronteira de aplicação | `normativo` no nível de domínio |
| **Job** | Command iniciado por agenda | Aplicação, quanto ao agendamento | `encaminhado` — o agendamento é de aplicação; a decisão que ele dispara é UPR |

`normativo` — Um tipo que aparece em mais de um bloco aparece como **modelos
distintos**, um por bloco, e não como o mesmo tipo reaproveitado (§6). O
`Command` que a UPR recebe e o `Command` que o caso de uso aceita compartilham
o nome do conceito, não a definição.

### §5.2 Emissor, consumidor, ciclo de vida e ownership

`normativo` para as linhas de domínio; `encaminhado` para as demais. As quatro
dimensões abaixo são **decisão nova**: Parte-1 §6.1 registra apenas semântica,
número de owners e se o tipo altera estado.

| Tipo | Emissor | Consumidor | Ciclo de vida | Owners |
|------|---------|------------|---------------|--------|
| Command de domínio | Application service, ao invocar a UPR | A UPR | Existe durante a chamada | Exatamente um destinatário lógico |
| Query de domínio | Application service | A UPR ou o modelo de leitura | Existe durante a chamada | Um |
| Application response | Application service | Quem chamou o caso de uso | Escopo do caso de uso | — (é saída) |
| Domain event | A UPR, no desfecho `Accepted` | O próprio bounded context; o mapeamento que converte para integração | Nasce no desfecho; termina ao ser aplicado ou convertido | Zero ou vários consumidores internos |
| Integration event | O mapeamento de §6.3, fora do domínio | Outros bounded contexts | Contrato público, versionado, com depreciação | Zero ou vários consumidores externos |
| Rejection | A UPR, no desfecho `Rejected` | O application service, que decide como expô-la | Nasce no desfecho; termina na borda | — (é saída) |
| Job | O agendador | Application service, que executa o caso de uso | Escopo da execução agendada | Um |

`normativo` — **Sobre o travessão na coluna de owners.** Parte-1 §6.1 deixa
`Application response` e `Rejection` sem owner. Isso não é lacuna a preencher:
é propriedade dos dois tipos. Ambos são **saídas**, não solicitações — não há
destinatário que "possua" o processamento, porque eles já são o resultado dele.
Atribuir-lhes um dono inverteria a direção da mensagem.

`rationale` — A tentação de completar a célula vazia é grande justamente porque
as outras cinco linhas têm valor. Preencher com "o chamador" confundiria
destinatário com dono: quem recebe uma resposta não é responsável por
processá-la, e a coluna deixaria de significar a mesma coisa em todas as linhas.

`encaminhado` — Emissor, consumidor e ciclo de vida do `Integration event`
aparecem acima para completar o quadro; a norma sobre eles — formato,
versionamento, compatibilidade e depreciação — é de FND-05
([ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442)). O agendamento de
`Job`, incluindo periodicidade, reentrância e garantias de execução, não é
normatizado aqui.

### §5.3 Nomenclatura

`normativo` — **apenas para mensagens de domínio**, que é o alcance da ANC-01.

| Tipo | Forma | Exemplo |
|------|-------|---------|
| Command de domínio | Verbo no imperativo, sobre o agregado | `PlaceOrder`, `ApprovePayment` |
| Query de domínio | Verbo de consulta ou substantivo da visão | `FindCustomer`, `AvailableBalance` |
| Domain event | Fato no particípio, no passado | `OrderPlaced`, `PaymentApproved` |
| Rejection | Código estável no formato `contexto/motivo` | `orders/empty-order` |

| ID | Regra |
|----|-------|
| `MSG-N01` | Nome de domain event está no passado. Um evento nomeado no imperativo é um command disfarçado, e o erro de nome esconde erro de modelagem |
| `MSG-N02` | Nome de mensagem de domínio não carrega versão. Versão é propriedade de contrato público (§6.4) |
| `MSG-N03` | Nome de mensagem de domínio não carrega transporte, formato nem tecnologia (`UPR-I12`) |
| `MSG-N04` | Nome de mensagem de domínio é expresso na linguagem ubíqua do bounded context, não em vocabulário técnico |

`recepcionado` — A nomenclatura dos níveis de aplicação e de wire —
`PlaceOrderRequest`, `company.orders.event.v1.OrderPlaced` — aparece em §6.1
como exemplo reproduzido de Parte-1 §6.2. A convenção normativa de naming,
namespace e versionamento no wire é de FND-05.

### §5.4 Tipo lógico e materialização no kernel

`normativo` — A taxonomia acima classifica mensagens por **papel**. Ela não
obriga cada kernel a materializar sete tipos distintos no seu sistema de tipos.

Uma realização é conforme se o papel de cada mensagem for determinável — por
tipo, por convenção de nome ou por posição no contrato — e se as regras de §5.3
valerem sobre o identificador resultante.

`rationale` — Os testes de viabilidade idiomática das duas stacks mostram
kernels que definem apenas uma noção mínima de mensagem e um evento, e
distinguem command, query, response e job por **convenção de nome**; um deles
sequer trata `Rejection` como mensagem, realizando-a pelo canal de erro. Isso
não contradiz a taxonomia: papel lógico e tipo de programa são coisas
diferentes, e exigir sete tipos nominais seria antecipar decisão de kernel —
exatamente o que P0-4 e o `rationale` de §3.4 recusam. O que a norma exige é
que o papel seja recuperável por quem lê a mensagem, não que ele tenha um tipo
próprio.

---

## §6. Os três níveis de contrato

### §6.1 Os níveis

`recepcionado` — Parte-1 §6.2, com as colunas de bloco e de força acrescentadas.

| Nível | Exemplo | Bloco | Evolução | Força |
|-------|---------|-------|----------|-------|
| **Domínio** | `PlaceOrder`, `OrderPlaced` | `domain library` | Livre dentro da biblioteca | `normativo` |
| **Aplicação** | `PlaceOrderRequest`, `PlaceOrderResponse` | `application service` | Controlada pelo serviço | `recepcionado`; ownership em §6.4 |
| **Wire** | `company.orders.event.v1.OrderPlaced` | `contract package` | Governada e compatível entre consumidores | `encaminhado` — FND-05 |

### §6.2 Não existe DTO universal

`normativo` — bloco `domain library`, que é o lado da separação que este
artefato pode obrigar.

| ID | Regra |
|----|-------|
| `CTR-01` | Os três níveis são modelos distintos. Nenhum tipo atravessa os três |
| `CTR-02` | Um tipo gerado de contrato de wire não é usado como estado de domínio, como entrada de UPR nem como evento de domínio |
| `CTR-03` | Um tipo de domínio não é exposto como contrato público. Consumidores nunca importam tipos do domínio do produtor |

`CTR-02` é a expressão, no nível da mensagem, da constraint P0-2 e da célula 6
de RFC §7.4; `UPR-I11` é a mesma regra vista do lado da UPR. `CTR-03` decorre
de RFC §7.2, que impede a `domain library` de ser superfície pública de
integração.

`recepcionado` — Parte-1 §6.2 enuncia a mesma proibição de forma direta: não se
deve reutilizar o mesmo DTO em domínio, REST, gRPC, persistência e mensageria.

`rationale` — A reutilização é atraente porque o tipo gerado do contrato chega
pronto, com serialização incluída, e escrever um segundo tipo parece
duplicação. O que ela custa aparece depois: o formato de wire passa a ditar a
forma do domínio — campos opcionais onde a regra exige presença, inteiros onde
o domínio tem valor monetário, ausência de invariante no construtor — e cada
mudança do contrato público vira mudança de domínio. O tipo separado não é
duplicação; é a fronteira que permite ao contrato e ao domínio evoluírem em
ritmos distintos.

### §6.3 A conversão de domain event para integration event

`normativo` na fronteira do domínio; `encaminhado` no mecanismo.

Forma canônica da conversão:

```text
mapear(domain event, contexto de publicação) -> integration event
```

| ID | Regra |
|----|-------|
| `CTR-04` | A conversão ocorre **fora da UPR** (`UPR-I09`). Em que bloco ela reside é `encaminhado` — ver a nota de fronteira abaixo |
| `CTR-05` | O evento de domínio **não atravessa inteiro**: apenas os fatos da tabela seguinte deixam o domínio; o restante permanece interno |
| `CTR-06` | A conversão é **unidirecional**: não existe conversão de integration event de volta ao domain event do produtor |
| `CTR-07` | Nenhum evento de domínio é contrato público por existir. A promoção de um evento de domínio a evento de integração é decisão declarada, nunca consequência automática de ele ter sido emitido |

O que atravessa a fronteira e o que fica:

| Atravessa | Fica no domínio |
|-----------|-----------------|
| Identidade do agregado | Referências a objetos internos |
| Os fatos de que outros contextos precisam | Estado intermediário da decisão |
| O instante em que o fato ocorreu | A estrutura interna do agregado |
| A versão do contrato público | Vocabulário interno não publicado |

`recepcionado` — Parte-1 §6.3 caracteriza os dois lados: o domain event usa
tipos nativos da linguagem, pertence ao bounded context, pode mudar em
refatoração interna e não carrega versionamento público no nome; o integration
event é definido no contrato, contém apenas o necessário aos consumidores, tem
owner, versão, compatibilidade e política de depreciação.

`normativo` — **Nota de fronteira: onde a conversão NÃO pode residir.** Parte-1
§6.3 atribui a conversão a "um mapper na camada de aplicação". Essa atribuição
**não é recepcionada**, porque colide com a RFC, que prevalece: construir um
integration event exige conhecer o `contract package`, e a célula 12 de
RFC §7.4 proíbe a aresta `application → contract` por P0-2 — "contrato de wire é
do adapter". A célula 18 autoriza `app → contract`.

Este artefato normatiza, portanto, apenas o lado do domínio: a conversão ocorre
fora da UPR e o evento de domínio não é, por si, contrato público. **Em qual
bloco externo o mapeamento reside é decisão de FND-04 e FND-05**, dentro do que
a matriz de RFC §7.4 já permite — não é escolha deste artefato, e normatizá-la
aqui excederia a ANC-01.

`rationale` — Recepcionar a frase da Parte-1 sem essa ressalva teria criado um
conflito insolúvel para quem implementa: seguir o artefato violaria a célula 12,
e seguir a RFC violaria o artefato. A Parte-1 é fonte conceitual e antecede a
matriz de dependências; onde as duas divergem, a RFC decide (§1.1).

`rationale` — `CTR-07` é a regra que a fonte não tinha e que a prática mais
viola. Ela é enunciada como propriedade do evento de domínio — e não como
obrigação sobre quem publica — de propósito: o que este artefato pode fixar é o
**status** do evento, não o comportamento do bloco que o encaminha. O risco que
ela previne é conhecido: quando ser publicado passa a ser consequência de ter
sido emitido, o conjunto de contratos públicos fica definido por acidente de
implementação, e uma refatoração interna que renomeie ou divida um evento quebra
consumidores externos que ninguém sabia existir.

`encaminhado` — Ficam com FND-05
([ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442)) o formato do
integration event, o codec, o registry, o namespacing, a política de
compatibilidade e a assinatura concreta do mapeamento. Ficam com FND-04
([ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)) a gravação na outbox
dentro da mesma transação e **o bloco em que o mapeamento reside**, escolhido
dentro do que a matriz de RFC §7.4 permite.

### §6.4 Ownership e versionamento por nível

`normativo` no nível de domínio; `recepcionado` e `encaminhado` nos demais.

| Nível | Owner | Como versiona | Força |
|-------|-------|---------------|-------|
| Domínio | O bounded context dono do agregado — um único time | **Não versiona**: evolui livremente, porque não é contrato público | `normativo` |
| Aplicação | O serviço que expõe o caso de uso | Controlada pelo serviço; compatibilidade exigida apenas dentro do próprio processo e dos seus chamadores diretos | `recepcionado` — Parte-1 §6.2 |
| Wire | Declarado no contrato, com owner por pacote | Versionamento maior, compatibilidade entre versões vizinhas, depreciação com prazo | `encaminhado` — FND-05 |

`normativo` — **Ownership no domínio.** Um tipo de domínio tem exatamente um
bounded context dono, e nenhum outro contexto o importa (`CTR-03`). Por
consequência, ele não tem versão: alterar um tipo de domínio é refatoração
interna, e é justamente a ausência de consumidor externo que torna a alteração
segura.

`rationale` — "Não versiona" pode soar como omissão de governança, e é o
oposto. Versionar um tipo que ninguém fora do contexto pode importar adiciona
processo sem beneficiário. Mais útil ainda é o que a regra revela quando é
violada: se aparece a necessidade real de versionar um tipo de domínio, o
sintoma é que ele vazou para fora do contexto. A correção é restaurar a
fronteira, não introduzir a versão — introduzir a versão apenas tornaria o
vazamento permanente e confortável.

`encaminhado` — A governança de contratos públicos — semântica de versão,
suporte a versões vizinhas, rito de depreciação com prazo e owner declarado por
pacote — é de FND-05, e a Parte-1 a descreve em capítulos que seguem vigentes
(§1.5).

---

## §7. Event Sourcing e CQRS como extensões opt-in

### §7.1 Status: extensão, nunca pré-requisito

`recepcionado` — a Parte-1 fixa o caráter incremental em três lugares
convergentes: o resumo executivo lista Event Sourcing, CDC, CQRS físico e
brokers alternativos como "extensões opt-in, nunca pré-requisitos"
(Parte-1 §1); os não objetivos dizem que o DMPF não deve "obrigar Event
Sourcing" nem "obrigar bancos separados para command e query" (Parte-1 §2.4); e
o quadro de decisões consolidadas registra `Event Sourcing: opt-in` e
`CQRS físico: opt-in` (Parte-1 §19).

`normativo` — bloco `domain library`.

| ID | Regra |
|----|-------|
| `ESC-01` | Nenhuma regra deste artefato pressupõe Event Sourcing ou CQRS físico. O modelo orientado a estado é o caminho padrão |
| `ESC-02` | Adotar Event Sourcing não revoga nem flexibiliza regra alguma de §2, §3, §5 e §6. A adoção muda a origem do estado e a forma da persistência, não a natureza da UPR nem a do desfecho |
| `ESC-03` | A adoção é decisão de um bounded context. Um contexto que adota Event Sourcing não a impõe a outros contextos nem a artefatos compartilhados |

`rationale` — `ESC-02` é o que sustenta o "opt-in" na prática. Se adotar Event
Sourcing exigisse reescrever as invariantes da UPR ou a forma do desfecho, a
extensão deixaria de ser opcional: o caminho padrão teria de acomodar as duas
modalidades desde o início, e todos os contextos pagariam o custo daquela que não
usam. Parte-1 §5.4 enuncia a mesma exigência pelo lado da assinatura — ela "deve
permitir a extensão sem impor seu custo a todos os contextos".

### §7.2 As duas modalidades

`recepcionado` — Parte-1 §5.4, com a coluna de status explicitada.

| Modalidade | Forma conceitual | Status |
|------------|------------------|--------|
| Orientada a estado | O alvo processa a requisição e devolve o desfecho; a aplicação grava o alvo | Padrão inicial |
| Orientada a eventos | `decide(estado, comando) -> Decision` para decidir; `evolve(estado, evento) -> estado` para reconstruir | Extensão opt-in |

`normativo` — nas duas modalidades, a função que decide **é** uma UPR no sentido
de §2: recebe estado e requisição por parâmetro, não executa I/O e devolve um
desfecho explícito. `evolve` é igualmente código de domínio puro — dado um estado
e um evento, devolve o estado seguinte, sem I/O e sem efeito observável fora do
retorno.

`normativo` — a diferença de forma não cria uma terceira categoria de função de
domínio. Uma UPR realizada como `decide` continua sujeita a `UPR-I01` a `UPR-I12`
e a `UPR-L01` a `UPR-L05`, e a sequência de eventos que ela devolve continua
sendo o conteúdo do ramo `Accepted` de §3.2, não um canal paralelo de resultado.

`recepcionado` com correção declarada — Parte-1 §5.4 escreve a modalidade como
`decide(state, command) -> events`, devolvendo a sequência de eventos direto. A
tabela acima **não reproduz** essa assinatura: uma função que devolve apenas
eventos não tem onde colocar a resposta de domínio (`DEC-05`) nem o ramo
`Rejected`, e usar a sequência vazia para significar recusa é exatamente o que
`ESC-04` proíbe. A forma recepcionada é a intenção — decidir sobre estado e
comando produzindo eventos —, e o tipo de retorno é o desfecho de §3.

### §7.3 O que permanece sob `decide` e `evolve`

`normativo` — bloco `domain library`. Esta é a tabela que o `ESC-02` resume.

| Regra deste artefato | Sob Event Sourcing |
|----------------------|--------------------|
| `UPR-I07` a `UPR-I12` (§2.2) | Permanecem integralmente. Ler o fluxo de eventos é I/O e ocorre **fora** da UPR: o estado reconstruído chega por parâmetro, como qualquer outro estado |
| `UPR-L01` a `UPR-L05` (§2.3) | Permanecem. `decide` e `evolve` não retêm estado entre execuções nem deixam trabalho agendado após o retorno |
| Determinismo (§2.4) | Permanece. Instante, identidade e aleatoriedade continuam chegando resolvidos; um evento cujo conteúdo depende do relógio recebe o instante como entrada |
| Exaustividade do desfecho (`DEC-01`) | Permanece. `decide` tem os mesmos dois desfechos possíveis, e a rejeição continua sendo dado tipado (`DEC-03`) |
| Pós-condições de `Rejected` (`DEC-10`, `DEC-11`) | Permanecem. Em rejeição não há evento a aplicar, e o estado reconstruído não avança |
| Taxonomia (§5) | Permanece. O evento gravado no fluxo é `Domain event`, não `Integration event` |
| Três níveis de contrato (§6) | Permanecem. `CTR-02` e `CTR-03` valem sem alteração |

`normativo` — `ESC-04`: a rejeição **não** se expressa como sequência vazia de
eventos. Por `DEC-06`, uma sequência vazia é um `Accepted` que nada mudou; a
rejeição é a outra variante do desfecho e carrega a rejeição tipada de `DEC-09`.

`rationale` — a confusão é natural: como nessa modalidade o efeito da decisão *é*
a sequência de eventos, a sequência vazia parece o lugar óbvio para dizer "não
aconteceu nada". Mas os dois casos são distintos e precisam continuar
distinguíveis por quem observa — um comando já aplicado que nada muda foi aceito;
um comando que fere invariante foi recusado e traz o motivo. Colapsar os dois na
sequência vazia apagaria a razão da recusa, que é justamente o ganho de `DEC-01`.

### §7.4 O que muda

`recepcionado` nas linhas que descrevem a persistência; `normativo` nas duas
últimas, que são do domínio.

| Dimensão | Orientada a estado | Orientada a eventos |
|----------|--------------------|---------------------|
| Origem do estado | Carregado como estado corrente do alvo | Reconstruído aplicando `evolve` sobre o fluxo, ou sobre um marco de estado mais o resto do fluxo |
| O que a aplicação grava | O estado resultante | Os eventos do desfecho, por acréscimo ao fim do fluxo |
| Papel do evento de domínio | Notificação de fato já ocorrido | Notificação **e** registro durável do estado |
| Controle de concorrência | Versão do alvo na gravação | Versão esperada do fluxo no acréscimo |
| Custo de mudar a forma do evento | Baixo — o evento é interno e efêmero | Alto — o evento antigo permanece no fluxo e precisa continuar legível |
| Natureza da UPR | §2 | §2, sem mudança |
| Fronteira de publicação | §6.3 | §6.3, sem mudança |

`encaminhado` — a mecânica da persistência de eventos (acréscimo ao fluxo, marcos
de estado, controle de versão, leitura de eventos gravados sob forma antiga) é da
camada de aplicação e da infraestrutura, e não é normatizada aqui; a parte
transacional é de FND-04
([ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)).

`normativo` — `ESC-05`: a durabilidade do evento não o promove a contrato
público. Um contexto que adota Event Sourcing não abre o próprio fluxo para
leitura por outro contexto; a integração continua acontecendo pela conversão de
§6.3, para `Integration event`. Ler o fluxo alheio é `CTR-03` violada na sua
forma mais direta, porque acopla o consumidor ao modelo interno **e** à história
dele.

`normativo` — `ESC-06`: manter eventos antigos legíveis não converte o evento de
domínio em tipo versionado no sentido de §6.4. O que §6.4 recusa é expor versão
de tipo de domínio como contrato público — e adotar Event Sourcing não cria essa
exposição.

`encaminhado` — Como a legibilidade dos eventos já gravados é preservada
(migração de fluxo, leitura de formas antigas, marcos de estado) é mecânica de
persistência, de FND-04. Este artefato registra apenas que esse custo existe e
que ele é consequência da adoção — não impõe a política.

### §7.5 CQRS lógico é o caminho padrão

`recepcionado` — Parte-1 §9.3: uma query não é obrigada a carregar agregados;
pode ser servida por read repository, projeção, view materializada, busca
especializada ou cache. "Esse é CQRS lógico. Bancos fisicamente separados são uma
decisão de contexto, não uma exigência do framework."

| ID | Regra |
|----|-------|
| `ESC-07` | Responder a uma query não exige carregar o alvo nem executar uma UPR |
| `ESC-08` | Quando a query é respondida por uma UPR — a hipótese admitida em §5.2 —, valem todas as regras de §2, em particular `UPR-I07`: a UPR recebe o estado, não vai buscá-lo |
| `ESC-09` | A separação entre escrita e leitura é lógica por padrão. Separação física de armazenamento é decisão de contexto, nunca requisito deste artefato |

`rationale` — as duas formas de CQRS se confundem porque dividem a sigla. O que o
caminho padrão exige é apenas que a leitura não precise atravessar o modelo de
escrita. Separar os armazenamentos é decisão de operação, com custo de
sincronização e de consistência eventual que só se paga sob carga que o
comprove.

### §7.6 Critérios de adoção

`recepcionado` — Parte-1 §5.4 é explícita: Event Sourcing "somente deve ser
adotado quando houver requisitos reais de replay, auditoria temporal ou
reconstrução histórica".

| Situação | Justifica adotar Event Sourcing? |
|----------|----------------------------------|
| Requisito de reexecução de decisões passadas | Sim — é o caso nomeado na fonte |
| Requisito de auditoria temporal: saber o estado em uma data | Sim |
| Requisito de reconstrução histórica do estado | Sim |
| Necessidade de notificar outros contextos sobre o que aconteceu | Não — isso é `Integration event` (§6.3) |
| Desejo de rastrear alterações para depurar | Não, isoladamente — rastro de execução é de FND-08 |
| Preferência estilística, ou uniformidade entre contextos | Não — por `ESC-03`, a decisão é de cada contexto |

`rationale` — as três linhas negativas são leitura desta norma, não do texto de
origem. Cada uma corresponde a uma necessidade que o framework já atende por
outro caminho, e adotar Event Sourcing para obtê-la paga o custo alto da última
coluna de §7.4 por um resultado que a modalidade padrão já entrega.

---

## §8. Exemplos

### §8.1 Como ler estes exemplos

`normativo` — os exemplos **ilustram** regras já escritas nas seções anteriores e
não emitem norma nova. Onde um exemplo e uma regra divergirem, prevalece a regra;
o exemplo é o defeito.

O pseudocódigo abaixo não é Go nem TypeScript, e nenhuma escolha de sintaxe dele
é normativa. As convenções de leitura são estas:

| Convenção | Motivo |
|-----------|--------|
| A assinatura é a conceitual de §2.1: sem canal de contexto, de cancelamento ou de falha técnica | Esses canais são de aplicação (§4), e incluí-los sugeriria uma realização |
| O desfecho aparece com o ramo nomeado; qualquer realização da tabela de §3.4 é conforme | A forma é normativa, o mecanismo é de kernel (§3.4) |
| Imutabilidade, opcionalidade e ausência aparecem em prosa, nunca como palavra-chave | As duas stacks as expressam de modos distintos, e uma delas não tem garantia mecânica (`DEC-12`) |
| Números são abstratos; nenhum exemplo fixa largura de tipo numérico | Fixá-la escolheria uma stack |
| Nenhum exemplo usa recurso que só uma das stacks tem | Correspondência exaustiva, congelamento de estrutura e restrições sobre tipos genéricos não estão disponíveis em pé de igualdade |

`rationale` — a neutralidade aqui é exigência de norma, não estilo. Um exemplo
escrito no idioma de uma stack vira, na prática, a especificação que os dois
kernels tentarão reproduzir — e a paridade que o épico pede é conceitual, não
sintática. Os desvios mais prováveis foram levantados na pesquisa de viabilidade
e estão evitados de propósito nas linhas acima.

### §8.2 Exemplo 1 — determinismo em dupla execução

Ilustra §2.4, `UPR-L01`, `UPR-L04` e `UPR-I07`.

```text
// estado e requisição chegam prontos, resolvidos pela aplicação (§2.4)
// os dois estados são construídos do MESMO snapshot, e não compartilham
// identidade: a segunda execução não recebe o que a primeira produziu
estadoA    = Pedido{ id: P-100, situação: aberto, itens: 2, limite: 3 }
estadoB    = Pedido{ id: P-100, situação: aberto, itens: 2, limite: 3 }
requisição = AdicionarItem{ sku: ABC, quantidade: 1,
                            em: 2026-08-17T12:00:00Z }

primeira = UPR(estadoA, requisição)
segunda  = UPR(estadoB, requisição)

primeira e segunda são observacionalmente iguais:
  ramo     : Accepted
  resposta : ItemAdicionado{ pedido: P-100, itens: 3 }
  eventos  : [ ItemAdicionadoAoPedido{ pedido: P-100, sku: ABC,
                                       em: 2026-08-17T12:00:00Z } ]
```

| O que o exemplo mostra | Regra |
|------------------------|-------|
| A segunda execução recebe um estado **equivalente ao de entrada**, construído do mesmo snapshot, e não o estado produzido pela primeira | `UPR-L01`, `UPR-L04` |
| Os dois estados não compartilham identidade mutável. Reusar a mesma variável tornaria o exemplo dependente de a stack copiar por valor — e `UPR-I04` permite que a decisão altere o alvo | `UPR-L04`, `UPR-I04` |
| O instante chegou dentro da requisição; nenhum relógio foi consultado na decisão | §2.4 |
| A sequência de eventos é a mesma, na mesma ordem | §2.4 |
| Não há leitura de repositório para descobrir o limite: ele veio no estado | `UPR-I07` |

`normativo` — **determinismo não é idempotência.** O exemplo fixa o estado de
entrada nas duas execuções; ele não afirma que aplicar o comando duas vezes ao
mesmo pedido real deixaria três itens. Determinismo é propriedade da função —
mesma entrada, mesmo desfecho. Idempotência é propriedade do efeito acumulado, e
depende de deduplicação na aplicação, que é de FND-04
([ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)).

### §8.3 Exemplo 2 — rejeição tipada, sem exceção e sem evento

Ilustra `DEC-03`, `DEC-04`, `DEC-09`, `DEC-10` e `DEC-11`.

```text
estado     = Pedido{ id: P-100, situação: aberto, itens: 3, limite: 3 }
requisição = AdicionarItem{ sku: XYZ, quantidade: 1,
                            em: 2026-08-17T12:00:00Z }

desfecho = UPR(estado, requisição)

desfecho:
  ramo     : Rejected
  rejeição : LimiteDeItensExcedido{ código: pedidos/limite-itens-excedido,
                                    limite: 3, tentado: 4 }
  eventos  : sequência vazia

estado observável do alvo, depois da chamada: itens = 3   // inalterado
```

| O que o exemplo mostra | Regra |
|------------------------|-------|
| A recusa chega como valor de domínio, pelo próprio desfecho | `DEC-03` |
| Nada é lançado por canal indistinguível de falha técnica | `DEC-04` |
| A rejeição carrega código estável no formato `contexto/motivo` fixado em §3.3 e na tabela de §5.3, e o chamador não precisa ler a mensagem para saber o motivo | `DEC-09` |
| O alvo não ficou parcialmente alterado | `DEC-10` |
| Não há evento algum, nem guardado em coleção interna | `DEC-11` |

`normativo` — o exemplo é deliberadamente o mesmo pedido do §8.2, com um item a
mais no estado de entrada. A única diferença entre aceitar e recusar é o estado
sobre o qual a mesma regra incide — e não a categoria da mensagem, nem o canal
por onde a resposta volta.

### §8.4 Exemplo 3 — contraprova: tipo de wire como estado de domínio

Ilustra `UPR-I11`, `CTR-02` e `CTR-03`, e é a contraprova pedida pelo terceiro
cenário da especificação.

```text
// NÃO CONFORME — contraexemplo
estado     = company.orders.v1.Order          // tipo gerado do contrato de wire
requisição = company.orders.v1.AddItemRequest // idem

desfecho = UPR(estado, requisição)
```

| Por que é recusado | Regra |
|--------------------|-------|
| A UPR recebe tipo gerado de contrato | `UPR-I11` |
| Um tipo de wire é usado como estado de domínio e como entrada de UPR | `CTR-02` |
| A constraint de fronteira do épico proíbe o formato de wire como modelo interno | P0-2; célula 6 de RFC §7.4 |

A forma conforme move a tradução para fora, e a UPR volta a ver apenas domínio:

```text
// CONFORME
// na camada de aplicação, antes de chamar a UPR:
pedido   = traduzir(company.orders.v1.Order)             // wire -> domínio
comando  = traduzir(company.orders.v1.AddItemRequest)    // wire -> domínio

desfecho = UPR(pedido, comando)
```

`rationale` — o contraexemplo é atraente porque o tipo gerado já chega pronto,
com serialização incluída, e escrever o tipo de domínio parece trabalho
duplicado. O custo aparece adiante, e §6.2 já o descreve: o formato de wire passa
a ditar a forma do domínio, com campo opcional onde a regra exige presença e sem
lugar para a invariante. O exemplo está aqui como contraprova porque a violação
é fácil de cometer sem perceber — ela não parece um erro de arquitetura no
momento em que é escrita.

### §8.5 Exemplo 4 — consumo que dispara nova UPR

Ilustra a fronteira de §4.1: o exemplo mostra **onde este artefato começa e onde
termina**, e recusa deliberadamente descrever o que está fora.

```text
camada de aplicação e infraestrutura — não normatizado aqui (FND-04)
  recepção da mensagem, deduplicação de entrega, confirmação ao broker
                              |
============================= | ======= início da fronteira deste artefato
                              v
  entrada já traduzida para domínio + estado do alvo já carregado
                              |
                              v
                 desfecho = UPR(estado, requisição)
                              |
============================= | ======= fim da fronteira deste artefato
                              v
  conversão dos eventos para integração (§6.3), gravação e publicação
  fora do domínio — bloco decidido por FND-04 e FND-05, dentro de RFC §7.4
```

| Trecho | Força | Dona |
|--------|-------|------|
| Recepção, deduplicação e confirmação da mensagem consumida | `encaminhado` | FND-04 ([ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)) |
| Tradução da entrada e carga do estado | `encaminhado` — o resultado é que importa: a UPR recebe domínio, por `UPR-I11`, e recebe o estado pronto, por `UPR-I07` | FND-04 |
| A decisão | `normativo` | §2, §3 |
| Conversão dos eventos de domínio para integração | `normativo` quanto à regra de conversão (§6.3); `encaminhado` quanto ao mecanismo | §6.3; FND-04 e FND-05 ([ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442)) |

`normativo` — um consumidor de mensageria **não** é uma UPR (§2.1). Ele orquestra
uma: recebe do transporte, traduz, carrega estado, chama a decisão e cuida do
desfecho. Chamar o consumidor de UPR faz as doze invariantes de §2.2 perderem
sujeito, porque nenhuma delas se sustenta sobre um componente que executa I/O por
definição.

---

## §9. Acionamento de ADRs estruturais

### §9.1 O que este artefato aciona

`recepcionado` — RFC §13.1 define o gesto e este artefato o repete sem alteração:
acionar é **nomear** o ADR, **definir o seu assunto**, **registrar a origem** e
**encaminhar** ao FND-11
([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)), a quem cabem a
redação, a promoção para `docs/adr/` na faixa `010`–`024` e o aceite.

`normativo` — Nenhum ADR é redigido nem aceito aqui. Os dois registros de §9.2
continuam a série da RFC, que parou em `ADR-DMPF-H`, e ficam no estado
`acionado`.

`normativo` — Os IDs `ADR-DMPF-I` e `ADR-DMPF-J` são **provisórios**, pela mesma
regra de RFC §13.2: a numeração definitiva é atribuída pelo FND-11 na promoção, e
citar um deles por número definitivo antes disso é erro de rastreabilidade.

`normativo` — O acionamento não antecipa a conclusão. Um destes ADRs pode, na
redação, decidir de forma diferente do que este artefato normatiza; nesse caso a
divergência se resolve por nova versão deste artefato, nunca por sobreposição
silenciosa — é a regra de RFC §13.1 aplicada ao sucessor.

### §9.2 Registro de acionamento

`registro` — mesmo formato de RFC §13.2.

| ID provisório | Nome | Assunto | Origem | Destino | Owner | Estado |
|---------------|------|---------|--------|---------|-------|--------|
| `ADR-DMPF-I` | Forma do desfecho da UPR | Adotar a união exaustiva de dois ramos como forma observável do desfecho, com a rejeição de negócio como dado tipado e código estável, deixando o mecanismo de realização a cada kernel sob equivalência observável | §3.1, §3.3, §3.4 | ARQ-448 | FND-11 | `acionado` |
| `ADR-DMPF-J` | Evento de domínio distinto do evento de integração | Adotar a separação entre os três níveis de contrato, com conversão explícita fora do domínio e recusa de tipo único que atravesse os níveis. O bloco em que a conversão reside não é objeto deste ADR: a matriz de RFC §7.4 já o restringe | §6.1, §6.2, §6.3 | ARQ-448 | FND-11 | `acionado` |

`registro` — As alternativas descartadas de cada decisão estão em bloco
`rationale` na seção de origem, e são insumo obrigatório da redação (RFC §13.2):

| ID | Alternativas descartadas | Onde estão registradas |
|----|--------------------------|------------------------|
| `ADR-DMPF-I` | Impor a união discriminada como mecanismo, e não só como forma; tratar o desfecho como produto com a rejeição fora dele, seguindo a realização atual dos testes de viabilidade | §3.4, nos dois blocos `rationale` finais |
| `ADR-DMPF-J` | Reaproveitar o tipo gerado do contrato nos três níveis; publicar o evento de domínio direto, sem conversão | §6.2 e §6.3, em bloco `rationale` |

### §9.3 Por que estes dois, e não mais

`recepcionado` — RFC §13.3 fixa a divisão: cada documento aciona **o que ele
mesmo decide**, e as decisões que pertencem a outra sub-spec são acionadas por
ela. Este artefato aciona, portanto, apenas o que decidiu.

`rationale` — Dentro do que este artefato decide, o corte usa três testes, e eles
são leitura própria, não texto da RFC: a decisão muda a forma de construir, vale
para todo trabalho novo e teve alternativa real descartada. As duas decisões
acima passam nos três. As demais regras deste artefato não passam em pelo menos
um: `ESC-01` a
`ESC-09` recepcionam uma decisão que a Parte-1 §19 já consolidou; `MSG-N01` a
`MSG-N04` são convenção de nomenclatura, não escolha entre arquiteturas; e as
invariantes de §2.2 derivam de constraints que `ADR-DMPF-A` e `ADR-DMPF-G` já
cobrem — reacioná-las aqui duplicaria assunto e criaria dois ADRs concorrentes
sobre a mesma matéria.

`normativo` — Os assuntos de `ADR-DMPF-I` e `ADR-DMPF-J` estão delimitados de
propósito ao desfecho e aos níveis de contrato. Nenhum dos dois decide sobre
pertencimento a bloco, regra de dependência ou pureza executável do domínio:
essas matérias são de `ADR-DMPF-A`, `ADR-DMPF-E` e `ADR-DMPF-G`, já acionados
pela RFC, e ampliar o assunto aqui criaria sobreposição entre ADRs da mesma
série.

---

## §10. Rastreabilidade e checklist de fronteira

### §10.1 A cadeia

`recepcionado` — RFC §14.5 fixa a cadeia que liga cada regra à origem e à
verificação. Instanciada para este artefato, com o elo que ainda não existe
declarado como pendente:

```text
constraint P0 / RFC / Parte-1 / decisão nova
        ↓  declaração de fonte na seção de origem de cada regra
    cláusula normativa deste artefato, com ID estável
        ↓  §10.3
    critério de aceite da story e AC do épico
        ↓  §10.4 — pendente: diagnóstico e vetores
    diagnóstico estável e par de vetores Go e TypeScript (FND-09)
```

`registro` — A fonte de cada regra é declarada **na seção onde ela é enunciada**,
não em uma coluna uniforme: §2.2 traz uma coluna de ancoragem por invariante; as
demais tabelas são `ID | Regra`, e a fonte aparece no bloco `recepcionado` ou na
prosa de ancoragem da subseção.

`normativo` — Elo quebrado é defeito do documento: regra sem fonte declarada na
sua seção de origem, ou critério de aceite sem regra que o satisfaça.

`normativo` — **Dois elos estão abertos, e nenhum é declarado fechado.** A
cadeia de RFC §14.5 exige, além do vetor, um diagnóstico estável para regra
`import-verifiable` — e **este artefato não emite nenhum código de
diagnóstico**. Além disso, apenas
as doze regras `UPR-I` têm modo de verificação declarado individualmente (§2.2);
as outras 42 não. As duas lacunas estão registradas em §10.4 como trabalho
endereçado a FND-09, e não como cobertura existente. A diferença importa: uma
pendência registrada é trabalho a fazer; uma ausência silenciosa passa por
verificação feita.

### §10.2 As regras, por família

`registro` — Cada regra declara a sua fonte na seção onde é enunciada. Esta
tabela é o índice das famílias, não uma segunda cópia das fontes: duplicá-las
aqui criaria duas verdades que envelheceriam em ritmos diferentes.

| Família | IDs | Onde são definidas | Sujeito da obrigação | Matéria |
|---------|-----|--------------------|----------------------|---------|
| `UPR-I` | `UPR-I01` a `UPR-I12` | §2.2 | A UPR | Invariantes da unidade |
| `UPR-L` | `UPR-L01` a `UPR-L05` | §2.3 | A UPR | Ciclo de vida — decisão nova |
| `DEC` | `DEC-01` a `DEC-13` | §3.1, §3.2, §3.3, §3.5 | O desfecho produzido pelo domínio | Forma e conteúdo do desfecho |
| `FRT` | `FRT-01` a `FRT-04` | §4.1 | A UPR, no seu lado da fronteira | Fronteira, pelo lado do domínio |
| `MSG-N` | `MSG-N01` a `MSG-N04` | §5.3 | O nome da mensagem de domínio | Nomenclatura |
| `CTR` | `CTR-01` a `CTR-07` | §6.2, §6.3 | O tipo de domínio e o seu status | Três níveis de contrato |
| `ESC` | `ESC-01` a `ESC-09` | §7.1, §7.3, §7.4, §7.5 | A UPR e o desfecho sob Event Sourcing | Extensões opt-in |

São 54 regras com ID estável. A coluna de sujeito existe porque ela é o teste de
M4: toda regra obriga um artefato do bloco `domain library` — a unidade, o
desfecho que ela produz, o nome ou o status de um tipo de domínio. Nenhuma
obriga o application service, o adapter, o consumidor externo ou o kernel. O que
diz respeito a eles aparece como `recepcionado`, quando já tem norma alhures, ou
`encaminhado`, com dona declarada.

### §10.3 Os critérios de aceite, e onde cada um é satisfeito

`registro` — A tabela é auditável sem sair do repositório: a coluna final diz o
que conferir no próprio artefato.

| # | Critério | Onde é satisfeito | Como conferir |
|---|----------|-------------------|---------------|
| 1 | UPR síncrona, determinística e sem I/O, com invariantes listáveis e verificáveis | §2.1, §2.2 | As três propriedades definidoras estão na tabela de §2.1; as doze invariantes em §2.2, cada uma com natureza de verificação declarada |
| 2 | `Decision` é o resultado oficial, com semântica de sucesso e de rejeição | §3.1 a §3.3 | `DEC-01` a `DEC-11`; os dois ramos têm subseção própria |
| 3 | UPR e application service com responsabilidades e ciclos distintos, sem zona cinzenta | §4.3, §4.4 | A tabela "quem faz o quê" traz coluna de força por linha; §4.4 resolve dez casos de zona cinzenta com veredito e critério |
| 4 | Os sete tipos de mensagem com semântica, ownership e nomenclatura | §5.1, §5.2, §5.3 | Uma linha por tipo em §5.1 e §5.2 (sete cada). §5.3 traz **quatro** linhas: a nomenclatura só é normativa para mensagens de domínio, e o naming de aplicação e wire é `recepcionado`, com dona em FND-05 |
| 5 | Evento de domínio e de integração como modelos distintos, com conversão documentada | §6.1, §6.3 | A tabela dos níveis os separa; §6.3 declara a forma canônica da conversão e o que atravessa |
| 6 | Nenhum DTO tratado como tipo universal entre fronteiras | §6.2 | `CTR-01` a `CTR-03`; a contraprova em pseudocódigo está em §8.4 — vetor executável só existirá com as fixtures de FND-09 (§10.4) |
| 7 | Event Sourcing e CQRS físico documentados como opt-in | §7 | `ESC-01` fixa o status; §7.3 e §7.4 dão o contraste exigido |
| 8 | Os quatro exemplos publicados e submetidos à revisão de um representante por stack | §8 (publicação) | Quatro exemplos em §8.2 a §8.5, sob as convenções de neutralidade de §8.1. **A solicitação da revisão acontece no PR** e é gate externo não bloqueante — não é satisfeita por este arquivo |
| 9 | `ADR-DMPF-I` e `ADR-DMPF-J` registrados como acionados, com material suficiente para a redação | §9 | §9.2 traz o registro no formato de RFC §13.2 e a tabela de alternativas descartadas, com o bloco de origem de cada uma |

`registro` — A spec traz ainda três critérios de aceite próprios, em redação
distinta da story. Dois deles correspondem às linhas 1, 2 e 5 acima; o terceiro
não tinha linha:

| Critério da spec | Onde é satisfeito | Como conferir |
|------------------|-------------------|---------------|
| Convenções de ownership e versionamento de contratos publicadas em nível conceitual | §6.4 | Ownership e versionamento por nível, `normativo` apenas no de domínio, com o teste diagnóstico de quando a necessidade de versionar indica vazamento de contexto |

`normativo` — **Três coisas este artefato não fecha sozinho**, e as três estão
ditas em vez de sugeridas: o critério 8 (a solicitação de revisão acontece no
PR); o critério de aceite da spec que exige `Domain event ≠ integration event`
**aprovado em PR**, que é aprovação externa por definição; e os vetores de
equivalência, pendentes em §10.4. Nenhum dos três é satisfeito por este arquivo.

### §10.4 Pendência de handoff a FND-09

`encaminhado` — §3.4 normatiza que duas realizações do desfecho são conformes
quando produzem observações equivalentes na fronteira, e delega a FND-09
([ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)) os vetores que
comprovam essa equivalência. **Esse contrato ainda não existe naquela sub-spec.**

O que o handoff precisa cobrir, para que o elo final da cadeia de §10.1 feche:

| Item | Origem |
|------|--------|
| Os dois ramos do desfecho exercitados nas duas stacks | §3.1 |
| Cada linha da tabela de observações equivalentes verificada em ambas | §3.4 |
| Rejeição chegando como dado, e não por canal indistinguível de falha técnica | `DEC-04` |
| Pós-condição de estado após recusa | `DEC-10`, `DEC-11` |
| Ausência de segundo caminho de entrega e de coleção pendente | `DEC-08`, `DEC-11` |
| Imutabilidade do desfecho e da sequência após o retorno | `DEC-12`, `DEC-13` |
| Os quatro exemplos de §8 como base das fixtures | §8 |

`normativo` — O handoff acima cobre `Decision`, que é o objeto de §3.4. Ele
**não** cobre as 54 regras: das sete famílias, apenas as doze `UPR-I` têm modo de
verificação declarado individualmente, e o artefato não emite código de
diagnóstico algum. Fechar a cadeia de §10.1 para as demais famílias exige
declarar modo por regra e, onde o modo for `import-verifiable`, um diagnóstico
estável — trabalho que pertence ao linter de dependências (RFC §10) e a FND-09,
não a este artefato.

`normativo` — Até que diagnóstico e vetores existam, as regras deste artefato são
norma **declarada e não verificada mecanicamente**. Registrar isso é obrigação da
cadeia de §10.1; tratá-las como verificadas seria elo quebrado.

### §10.5 Checklist de fronteira

`normativo` — Verificação das quatro regras de monotonicidade de RFC §12.2 e da
fronteira de RFC §1.4, exigida pela âncora ANC-01.

| Regra | Como este artefato a respeita |
|-------|-------------------------------|
| M1 — detalhar ou restringir dentro do escopo | As 54 regras de §10.2 têm por sujeito a UPR, o desfecho que ela produz, ou o nome e o status de um tipo de domínio — todos do bloco `domain library`, o escopo permitido da ANC-01. Elas acrescentam especificidade ou tornam mais estrito o que a fonte deixava aberto |
| M2 — não relaxar nem reinterpretar invariante da âncora | P0-1, P0-2, as células 5 e 6 de RFC §7.4 e RFC §9 aparecem como `recepcionado` e como fonte de regra, nunca reescritos. `UPR-I07` e `UPR-I11` os restringem ao caso da UPR. A **célula 12** (`application → contract`) foi o ponto em que a fonte conceitual divergia da RFC: §6.3 recusa explicitamente a atribuição da Parte-1 em favor da matriz, em vez de propagá-la |
| M3 — relaxar P0 exige nova versão da RFC | Nenhuma regra deste artefato relaxa constraint P0. As decisões novas (`UPR-L`, `DEC-12`, `DEC-13`, `CTR-07`, parte de `ESC`) tratam de matéria que a RFC não cobre, ou são mais estritas que a fonte |
| M4 — não exceder o escopo da âncora | O que pertence a outra sub-spec sai marcado `encaminhado`, com dona nomeada: FND-04 (transação, Unit of Work, outbox, persistência de eventos), FND-05 (wire, versionamento, codec), FND-07 (contexto e borda), FND-08 (observabilidade), FND-09 (vetores) e FND-11 (redação dos ADRs) |

`normativo` — A fronteira de RFC §1.4 é espelhada em §1.4 deste artefato, e o
teste é o mesmo em toda seção: se uma regra obriga alguém fora do bloco
`domain library` a fazer algo, ela não é norma deste artefato. Onde o assunto era
necessário para a compreensão mas está fora da fronteira, o texto o descreve e
marca a dona, em vez de normatizar por conveniência.

`normativo` — O teste acima foi aplicado regra a regra na revisão, e ele **mudou
o texto**. As regras de conversão em §6.3, a imutabilidade do desfecho em §3.5 e
`ESC-06` em §7.4 estavam enunciadas como obrigação sobre quem converte, quem
consome o desfecho e quem persiste o fluxo. Cada uma foi reescrita como
propriedade do artefato de domínio — o que a conversão deixa sair, o que o
desfecho não oferece, o que o evento não se torna — e a obrigação sobre o ator
externo passou a `encaminhado`, com dona. O que o teste rejeita não é o assunto:
é a pretensão de obrigar fora da âncora.

`rationale` — Este checklist existe porque a revisão adversarial do plano
encontrou, na estrutura original, exatamente o risco que M4 nomeia: normatizar o
application service e o ownership dos contratos de aplicação e de wire seria
tecnicamente defensável e ainda assim inválido, porque a RFC prevalece sobre a
sub-spec no conflito. A correção foi classificar força por bloco em cada regra —
e este checklist é o ponto onde essa classificação é auditada de uma vez.
