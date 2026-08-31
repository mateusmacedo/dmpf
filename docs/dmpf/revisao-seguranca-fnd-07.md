# Parecer da revisão de Segurança — DMPF FND-07 §7 e §8

| Campo | Valor |
|-------|-------|
| **Status** | `parecer de revisão` — não normativo; nada aqui obriga |
| **Objeto** | `contexto-erros-seguranca.md` §7 (threat model) e §8 (governança do dado) |
| **Gate** | `THR-03` — «este threat model não está satisfeito enquanto não for revisado por Segurança» |
| **Revisor** | Mateus Macedo Dos Anjos, por [ADR-029](../adr/029-titular-da-revisao-de-seguranca-fnd-07.md) |
| **Independência** | **Ausente.** O revisor é o autor do artefato. A limitação é declarada, não contornada — ver §1.3 e o ADR-029 |
| **Épico** | [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436) — Golden Path para Sistemas Orientados a Domínio e Mensagens |
| **Story** | [ARQ-488](https://lider-cap.atlassian.net/browse/ARQ-488) |
| **Spec** | [SPEC-DK8QQSDQ](../specs/SPEC-DK8QQSDQ-revisao-seguranca-fnd-07.md) |
| **Data** | 2026-08-30 |
| **Desfecho** | **Aprovado com ressalvas** — duas, ambas de insumo externo ausente; ver §7 e §8 |

> **O que este documento é.** Evidência de que a revisão exigida por `THR-03`
> foi executada, e registro do que ela encontrou. Nenhuma regra nasce aqui:
> nenhum identificador `THR-*` ou `DAT-*` é criado, alterado ou renumerado. Onde
> este parecer e uma regra do artefato divergirem, prevalece a regra.

---

## §1. Escopo, método e limitações

### §1.1 O que foi revisado

`THR-03` nomeia três objetos, e são exatamente eles que delimitam a revisão:

1. a **varredura** das seis categorias STRIDE em cada um dos sete vetores;
2. as **exclusões justificadas**;
3. a **atribuição de owner** de cada linha.

O cabeçalho do artefato acrescenta §8 ao encargo do representante de Segurança —
«um representante de Segurança para §7 e §8, obrigatório» —, e a ARQ-488 nomeia
quatro pontos para atenção específica: `CTX-25`, `IDN-14`, `DAT-10` e `THR-01`.

Fica **fora**: o mérito de `CTX`, `IDN`, `ERR` e `MAP`, revisados por Plataforma
e Arquitetura no PR #10; as pendências 1, 2, 3 e 5 de §11.4, cada uma com
destino já nomeado; e o estado do documento, que permanece `draft normativo`
como os demais oito artefatos FND.

### §1.2 Método: o mecânico antes do julgamento

A conferência foi executada em duas camadas, nesta ordem, e a ordem importa.

A **camada mecânica** aplica critérios objetivos, reproduzíveis por varredura do
artefato: presença de categoria, presença de motivo, presença de owner,
existência do identificador citado, cardinalidade. Ela não depende do julgamento
de quem a executou, e o seu resultado foi capturado **antes** de qualquer
redação de mérito.

A **camada de julgamento** avalia o que nenhum critério objetivo alcança: se o
motivo de uma exclusão sustenta a ausência, se a atribuição de owner é a certa,
se uma regra faz o que se propõe. Ela está isolada em §5 e em §7, e é onde a
limitação de §1.3 incide.

Os critérios mecânicos, para reconferência por terceiros:

| Critério | Procedimento |
|----------|--------------|
| Cobertura STRIDE | Para cada vetor, cruzar as categorias que têm linha de ameaça própria com as nomeadas na linha de exclusões; a união deve ser as seis |
| Owner por linha | Coluna 4 de cada linha de dados das sete tabelas, incluindo a linha de exclusões |
| Regras citadas existem | Extrair os identificadores `` `XXX-NN` `` das colunas Ameaça e Mitigação e casá-los com as definições em `docs/dmpf/*.md` |
| Coerência de âncora | Para cada owner que cite `FND-XX sob ANC-YY`, conferir o par contra o registro de RFC §12.3 |
| Cardinalidade do baseline | Contar os bullets de Parte-1 §14 e as linhas da matriz de §8.8 |
| Integridade normativa | Contagem de identificadores por prefixo, e continuidade da numeração |

### §1.3 Limitações declaradas

**A revisão não é independente.** O revisor é o autor do artefato conferido. O
ADR-029 registra a decisão, o motivo — não há área de Segurança constituída na
organização — e a condição de re-revisão. A consequência prática: a camada
mecânica de §2 a §4 e §6 é reproduzível por qualquer pessoa e não depende de
quem a executou; a camada de julgamento de §5 e §7 depende, e é exatamente onde
uma revisão independente teria mais valor. Nada neste parecer elimina essa
fraqueza — este parágrafo existe para que ela não passe despercebida.

**A conferência de §6 depende de fonte não versionada.** A matriz de §8.8 é
conferida contra `Parte-1 §14`, citada na convenção que a RFC fixa em §1.3. A
base conceitual não está no repositório, por decisão registrada no
[ADR-007](../adr/007-fronteira-plugin-template.md). Quem não tiver o arquivo
reproduz §2 a §5, mas não §6.

**A suficiência do conjunto de vetores não foi revisada.** `THR-03` manda
conferir a varredura das seis categorias «em cada um dos **sete** vetores» — o
conjunto é dado pela regra, não posto em questão por ela. Esta revisão confere
que os sete estão completos e corretamente varridos; ela **não** responde se
sete são os vetores certos, nem se algum vetor material do DMPF ficou de fora.
Essa pergunta existe, é legítima, e cabe a uma revisão de escopo diferente —
tomar este parecer por validação da suficiência do conjunto seria estendê-lo
além do que ele mediu.

---

## §2. Conferência da varredura STRIDE

### §2.1 Matriz de cobertura

`M` — a categoria tem linha de ameaça material própria no vetor.
`X` — a categoria é nomeada na linha de exclusões do vetor, com motivo.

| Vetor | *Spoofing* | *Tampering* | *Repudiation* | *Information disclosure* | *Denial of service* | *Elevation of privilege* |
|-------|:---:|:---:|:---:|:---:|:---:|:---:|
| V1 — Adapters | M | M | M | M | X | M |
| V2 — Desserialização | X | M | M | M | X | M |
| V3 — Contratos | X | M | X | M | X | M |
| V4 — Mensageria | M | M | M | M | X | M |
| V5 — Secrets | M | X | X | M | X | M |
| V6 — PII | X | M | M | M | X | X |
| V7 — Permissões | M | X | M | M | X | M |

**Resultado: nenhuma lacuna.** As 42 células estão preenchidas — 27 como ameaça
material e 15 como exclusão nomeada. Não há categoria que apareça em nenhuma das
duas posições em nenhum vetor, que é a condição que `§7.1` estabelece ao exigir
a linha de exclusões: «categoria ausente sem justificativa é indistinguível de
categoria não avaliada».

### §2.2 Por que 33 linhas ocupam 27 células

As tabelas dos sete vetores somam **33 linhas de ameaça material**, mas apenas
27 células da matriz. A diferença de seis não é inconsistência: são segundas e
terceiras ameaças **distintas** da mesma categoria no mesmo vetor.

| Vetor | Categoria | Linhas | O que as distingue |
|-------|-----------|:------:|--------------------|
| V4 — Mensageria | *Elevation of privilege* | 2 | Sujeito reconstruído do envelope (`CTX-25`) × `tenantid` ausente substituído por `default` (`CTX-26`, `IDN-20`) |
| V5 — Secrets | *Information disclosure* | 2 | Segredo em log, trace ou DLQ (`DAT-02`, `DAT-06`) × segredo versionado em código (cláusula do baseline) |
| V6 — PII | *Information disclosure* | 3 | Dado pessoal no pipeline de log × campo sem classificação × dado pessoal legível em repouso |
| V7 — Permissões | *Elevation of privilege* | 3 | Autenticado tratado como autorizado × pertencimento a tenant no lugar de permissão × cache de autorização reusado |

Distribuição por vetor: V1=5, V2=4, V3=3, V4=6, V5=4, V6=5, V7=6. Total 33, mais
7 linhas de exclusões — **40 linhas conferidas**.

---

## §3. Conferência das exclusões

Cada uma das sete linhas de exclusões foi conferida quanto a três coisas: a
categoria está nomeada; o motivo está declarado; e o motivo sustenta a ausência.
Onde a exclusão **remete a outro vetor**, a remissão foi seguida até o destino e
a presença da categoria lá foi confirmada.

| Vetor | Categorias excluídas | Motivo declarado | Remissão verificada | Veredicto |
|-------|----------------------|------------------|---------------------|-----------|
| V1 — Adapters | *Denial of service* | Avaliado; mitigação de FND-08 sob ANC-06 (§7.9) | — | Conforme |
| V2 — Desserialização | *Spoofing*, *Denial of service* | O decode não estabelece identidade, que é de §4, vetor 1 e vetor 4; decompression bomb e payload excessivo avaliados | *Spoofing* material em **V1** e **V4** | Conforme |
| V3 — Contratos | *Spoofing*, *Repudiation*, *Denial of service* | O contrato é declaração, não canal: identidade e rastreabilidade são dos vetores 1 e 4 | *Spoofing* e *Repudiation* materiais em **V1** e **V4** | Conforme |
| V4 — Mensageria | *Denial of service* | Poison message e retry ilimitado avaliados; FND-04 `GAR-08` cobre o mecanismo; dimensionamento de FND-08 | `GAR-08` existe em `uow-inbox-outbox.md` | Conforme |
| V5 — Secrets | *Tampering*, *Repudiation*, *Denial of service* | Integridade e auditoria do cofre são de plataforma, fora das âncoras do épico | — (encaminhamento, não remissão) | Conforme |
| V6 — PII | *Spoofing*, *Elevation of privilege*, *Denial of service* | Identidade e permissão são dos vetores 1 e 7 | *Spoofing* material em **V1**; *EoP* material em **V7** | Conforme |
| V7 — Permissões | *Tampering*, *Denial of service* | A alteração do modelo de permissões é matéria de plataforma e de IdP, fora desta âncora | — (encaminhamento) | Conforme |

**Resultado: sete de sete conformes.** Nenhuma exclusão sem categoria nomeada,
nenhuma sem motivo, e as seis remissões a outro vetor têm destino verificado —
a categoria excluída em um vetor aparece como ameaça material no vetor para o
qual a exclusão remete.

Os sete encaminhamentos de *Denial of service* apontam para §7.9 e para FND-08,
sem exceção. A fronteira é conferida em §5.4.

---

## §4. Conferência da atribuição de owner

`§7.1` normatiza: «nenhuma linha sem owner. Uma ameaça cuja mitigação não tenha
dono nomeado — regra deste artefato pela ID, artefato irmão pela âncora, ou
plataforma — não está mitigada: está listada».

### §4.1 Presença

**As 40 linhas têm owner.** 33 materiais e 7 de exclusões, nenhuma com a coluna
vazia ou preenchida com travessão. Conferido por varredura da quarta coluna de
todas as tabelas de §7.2 a §7.8.

### §4.2 Qualidade — os owners que apontam para fora

Vinte das 40 linhas atribuem owner fora do próprio artefato: 13 materiais e as 7
de exclusões. Para cada uma, foram conferidas a existência da âncora citada no
registro de RFC §12.3 e a coerência do par `FND-XX sob ANC-YY`.

| Artefato irmão | Âncora | Ocorrências | Existe na RFC §12.3 | Par coerente |
|----------------|--------|:-----------:|:-------------------:|:------------:|
| FND-03 — `upr-decision-mensagens.md` | ANC-01 | 1 | Sim | Sim |
| FND-04 — `uow-inbox-outbox.md` | ANC-02 | 2 | Sim | Sim |
| FND-05 — `cloudevents-protobuf-buf.md` | ANC-03 | 4 | Sim | Sim |
| FND-06 — `politicas-transporte.md` | ANC-04 | 3 | Sim | Sim |
| FND-08 — `resiliencia-observabilidade.md` | ANC-06 | 7 | Sim | Sim |
| `plataforma` | — | 10 | Sujeito admitido por `§7.1` | — |

**Resultado: nenhuma âncora inexistente e nenhuma citação incoerente.** Todo par
`FND-XX sob ANC-YY` no §7 corresponde ao mapeamento oficial do registro de
extensão, e todos os cinco artefatos irmãos citados existem versionados em
`docs/dmpf/`.

### §4.3 As regras citadas nas mitigações

A coluna Mitigação de §7 cita **54 identificadores distintos**. Todos os 54
existem como regra definida. Três são definidos fora do FND-07, e cada um no
artefato que o owner da linha declara:

| Regra | Definida em | Owner declarado na linha |
|-------|-------------|--------------------------|
| `ENV-05` | `cloudevents-protobuf-buf.md` (FND-05) | FND-05 sob ANC-03 |
| `GAR-07` | `uow-inbox-outbox.md` (FND-04) | FND-04 sob ANC-02 |
| `GAR-08` | `uow-inbox-outbox.md` (FND-04) | FND-04, FND-08 |

**Nenhuma citação órfã.** Uma mitigação que apontasse para regra inexistente
seria mitigação aparente — o defeito que esta conferência procura e não
encontrou.

### §4.4 Integridade normativa do artefato

Contagem de identificadores por prefixo, conferida para garantir que a revisão
não alterou o que revisava:

| Prefixo | Qtd | Faixa | Contígua |
|---------|:---:|-------|:--------:|
| `CTX` | 28 | `CTX-01`..`CTX-28` | Sim |
| `IDN` | 20 | `IDN-01`..`IDN-20` | Sim |
| `ERR` | 28 | `ERR-01`..`ERR-28` | Sim |
| `MAP` | 7 | `MAP-01`..`MAP-07` | Sim |
| `THR` | 3 | `THR-01`..`THR-03` | Sim |
| `DAT` | 26 | `DAT-01`..`DAT-26` | Sim |
| **Total** | **112** | | |

O total confere com o que `testes-interop.md` §11.1 declara de forma
independente — «Total: 112, contíguo por prefixo, sem lacuna nem repetição» —,
o que dá um segundo ponto de aferição não derivado desta revisão.

---

## §5. Conferência de §8 e dos pontos nominados

Esta seção é julgamento, não varredura. A limitação de §1.3 incide aqui.

### §5.1 Classificação e o critério de sensibilidade (§8.1)

`DAT-01` nomeia oito superfícies onde o dado de negócio vive, e a lista é
fechada, não exemplificativa — o que a torna conferível. `DAT-02` fixa o
critério de sensibilidade por quatro testes disjuntivos. `DAT-03` fecha por
omissão: campo sem classificação é tratado como sensível. `DAT-04` compõe a
taxonomia corporativa, onde existir, com o critério técnico.

**Conferido.** A composição das quatro regras não deixa buraco lógico: ou há
taxonomia corporativa e ela prevalece no que cobrir, ou o teste técnico de
`DAT-02` se aplica, ou não há classificação e `DAT-03` a trata como sensível. O
`rationale` da subseção está correto ao apontar que `DAT-03` sozinho tornaria
tudo sensível por omissão, e que o par `DAT-02`/`DAT-04` é o que dá piso sem
usurpar a política corporativa.

**Ressalva R-01, registrada em §7.**

### §5.2 Cifra em repouso (§8.3) e o ponto `DAT-10`

`DAT-08` está formulado como **pós-condição** — «permanece cifrado em repouso»,
verificável por inspeção da superfície armazenada e não do código que a escreve.
A escolha é a correta sob M4: prescrever algoritmo, modo ou custódia seria regra
de plataforma escrita no lugar errado, e a formulação como propriedade
observável mantém a regra verificável sem invadir a competência alheia.

`DAT-09` fecha a falácia de que cifra basta: dado cifrado e legível por qualquer
processo com a chave está protegido pelo controle de acesso que a cifra
pressupõe, não pela cifra.

**`DAT-10` — ponto nominado pela ARQ-488.** A extensão da pós-condição às
superfícies de contenção é materialmente relevante e está corretamente
formulada. O envelope preservado por FND-04 `GAR-07` carrega dado de negócio e,
como o `rationale` observa, frequentemente permanece armazenado por mais tempo
que o original: a DLQ nasce como recurso operacional, é povoada por incidente e
retém envelope íntegro por prazos que ninguém definiu. **Conferido: a regra
alcança a superfície que quase sempre escapa.**

A verificação não fica sem dono. O FND-09 ancora `DAT-08` e `DAT-10` no oráculo
`RAS-38`, com procedimento concreto — gravar, por cada caminho de escrita de
`DAT-01`, um valor reconhecível de classe protegida, e então abrir a superfície,
inclusive DLQ e quarantine. **Conferido.**

### §5.3 Acesso (§8.4), retenção (§8.5) e o ponto `IDN-14`

`DAT-11` sujeita o acesso ao dado **fora da aplicação** — console de banco,
ferramenta de análise, extração, acesso de plantão — ao regime de nomeado,
autorizado e registrado. `DAT-12` impede que DLQ, quarantine e outbox escapem
para o regime mais frouxo que a natureza «operacional» sugeriria.

`DAT-13` é a mais valiosa das três, e o seu `rationale` identifica com precisão o
erro que ela previne: um isolamento por tenant impecável na aplicação não diz
nada sobre o engenheiro que abre o console do banco, e as duas defesas cobrem
superfícies disjuntas. **Conferido.**

**`IDN-14` — ponto nominado pela ARQ-488.** O isolamento por tenant é exigido
como resultado *fail-closed*, e a imposição não pode depender de convenção de
código. A formulação está correta: exigir o resultado e encaminhar o mecanismo
ao provider e ao kernel é o que M4 permite, e a verificação executável está no
FND-09. **Conferido.**

§8.5 formula a retenção como **desigualdade** — a efetiva é menor ou igual ao
teto externo —, sem fixar o valor. `DAT-16` faz o teto prevalecer sobre o prazo
operacional, fechando o que FND-04 §4.3 encaminhou; `DAT-17` recai no mais
estrito quando não há classificação; `DAT-18` exige purga com evidência.
`DAT-14` tem oráculo em `RAS-39`, que testa inclusive o caso «teto ausente sem
recair no mais estrito». **Conferido.**

**Ressalva R-02, registrada em §7.**

### §5.4 A fronteira do eixo *Denial of service* e o ponto `THR-01`

**`THR-01` — ponto nominado pela ARQ-488.** A pergunta a responder é se a
fronteira está no lugar certo: o eixo *D* é avaliado nos sete vetores e a
mitigação é encaminhada a FND-08 sob ANC-06, sem gerar norma aqui.

O registro de RFC §12.3 declara o escopo permitido de ANC-06: «baselines de
telemetria, políticas de retry e degradação». Limiar, orçamento, degradação,
dimensionamento e política de contenção de carga — precisamente o que `THR-01`
recusa normatizar — cabem nesse escopo. **A fronteira está no lugar certo.**

O `rationale` de §7.9 acerta ao explicar por que o registro é necessário mesmo
sem norma: sem ele, a revisão de Segurança não saberia se *Denial of service* foi
considerado e descartado ou simplesmente esquecido. Esta revisão confirma o
efeito pretendido — a distinção foi verificável, e é o que §2.1 registra.

`THR-02` identifica corretamente o que **é** deste artefato e alcança o eixo *D*
indiretamente: `ERR-11`, o default fechado da retryability, e `ERR-24`, a
vedação de inferir retryability de exceção desconhecida. Nenhuma das duas é
política de resiliência. **Conferido.**

### §5.5 `CTX-25` — proveniência não é autorização

**Ponto nominado pela ARQ-488.** A regra estabelece que, no consumo assíncrono,
o sujeito que originou a cadeia é tratado como proveniência e nunca como
autorização.

**Conferido, e é a regra mais consequente do conjunto.** Ela impede elevação de
privilégio diferida: sem ela, quem consegue publicar no tópico escolhe a
identidade com que o consumidor age, e o controle de acesso do produtor passa a
governar o do consumidor. É a mitigação principal da primeira linha de
*Elevation of privilege* do vetor 4, e `IDN-04` a complementa definindo o que
conta como entrada autenticada no consumo. O par cobre a ameaça pelos dois
lados: o que não autoriza, e o que autoriza.

---

## §6. Conferência da matriz do baseline (§8.8)

A matriz enumera as cláusulas de `Parte-1 §14` **por texto**, e `DAT-26` exige
estado declarado para cada uma. A conferência seguiu esse critério, e não a
contagem de itens — como o `rationale` da subseção determina.

**Cardinalidade: 13 bullets na fonte, 13 linhas na matriz.** A correspondência é
1:1 e na mesma ordem. O `rationale` de §8.8 está correto ao registrar que uma
formulação anterior exigia «os 14 itens do baseline» e que a contagem estava
errada.

| Estado | Cláusulas | Dono do que não é consolidado |
|--------|:---------:|-------------------------------|
| `Vigente` na Parte-1 | 3 | plataforma; governança de engenharia |
| `Consolidada` | 5 | — |
| `Parcialmente consolidada` | 2 | plataforma (privilégio de infraestrutura); ANC-03/ANC-04 e plataforma (mecanismo de assinatura) |
| `Encaminhada` | 3 | ANC-03/FND-05; ANC-04/FND-06; plataforma |

**Resultado: as 13 cláusulas têm estado declarado, e toda cláusula não
consolidada tem dono nomeado.** Nenhuma linha declara consolidação que o
artefato não entregou: a cláusula de threat modeling é marcada `Consolidada` em
§7 — o que este parecer confere —, e a de least privilege é marcada
`Parcialmente consolidada`, reconhecendo que `DAT-11` cobre o acesso ao dado de
negócio e não o privilégio de infraestrutura.

---

## §7. Achados e ressalvas

Nenhum critério objetivo falhou. As duas ressalvas abaixo são da mesma natureza:
regras corretamente formuladas cujo **insumo externo ainda não existe**. Elas não
invalidam a varredura nem apontam defeito no artefato — registram uma condição
de aplicação que ninguém havia declarado.

| ID | Ressalva | Severidade | Destino |
|----|----------|:----------:|---------|
| **R-01** | `DAT-04` compõe o critério técnico com a **taxonomia corporativa de classificação de dados**, que não existe declarada em lugar nenhum do repositório — a `SPEC-XQWGGAXF` a lista como entrada consumida, no escopo fora. Na prática, hoje opera apenas o piso de `DAT-02`, e o teto que `DAT-04` pressupõe não está posto. A regra está certa; falta o insumo | Baixa | Contexto de negócio e governança de engenharia — fora das âncoras do épico |
| **R-02** | `DAT-15` exige **origem declarada** do teto de retenção, e `DAT-17` manda recair no «mais estrito declarado no contexto» quando não há classificação. Nenhum teto com origem declarada existe hoje. `RAS-39` detecta a violação da desigualdade, mas não supre a ausência do valor de referência: sem teto declarado, a desigualdade não tem contra o que ser aferida | Baixa | Contexto de negócio e requisito externo, como `DAT-15` já atribui |

Nenhuma das duas é pendência **desta** revisão nem do FND-07: ambas dependem de
ato organizacional fora das âncoras do épico, e ambas são consistentes com o que
o artefato já declara ao encaminhá-las.

**Não são achados**, e ficam registrados para que ninguém os reabra como se
fossem:

- A verificação da pós-condição de `DAT-08` e `DAT-10` **tem** dono e método —
  `RAS-38`, no FND-09. Foi verificado antes de virar ressalva, e não é uma.
- O estado `draft normativo` do artefato não é defeito: os nove artefatos FND
  estão no mesmo estado, e ele é independente deste gate.
- A ausência de owner nomeado que travava `THR-03` foi resolvida pelo ADR-029, e
  é a condição de existência deste parecer — não um achado dele.

---

## §8. Desfecho

**Aprovado com ressalvas.**

Os três objetos que `THR-03` nomeia foram conferidos e estão conformes:

1. **A varredura das seis categorias em cada um dos sete vetores** — 42 células,
   nenhuma lacuna (§2.1).
2. **As exclusões justificadas** — sete de sete com categoria nomeada e motivo
   declarado, e as seis remissões a outro vetor com destino verificado (§3).
3. **A atribuição de owner de cada linha** — 40 de 40 linhas com owner; as 20
   que apontam para fora citam âncoras existentes, com par `FND-XX sob ANC-YY`
   coerente com o registro de RFC §12.3 (§4).

§8 foi conferida nos quatro eixos, e os quatro pontos que a ARQ-488 nomeia —
`CTX-25`, `IDN-14`, `DAT-10` e `THR-01` — receberam tratamento nominal (§5). A
matriz do baseline confere 1:1 com as treze cláusulas da fonte, com estado
declarado e dono do que não é consolidado (§6).

As ressalvas R-01 e R-02 têm severidade baixa, destino nomeado e não impedem o
fechamento: ambas dependem de insumo organizacional que o próprio artefato já
encaminha para fora das suas âncoras.

**O gate `THR-03` fica satisfeito**, e o critério de aceite correspondente da
`SPEC-XQWGGAXF` pode ser marcado.

**Com a validade que o ADR-029 lhe dá.** Esta revisão foi executada sem
independência entre autor e revisor, e vale até que a organização constitua a
área de Segurança — momento em que §7 e §8 voltam à revisão por titular
independente, conforme a condição registrada naquele ADR.

### Onde o estado passa a viver

O FND-07 **não é editado** por este parecer. A pendência 4 de §11.4 e o registro
de §7.10 permanecem como foram promovidos, porque artefato promovido não se
corrige retroativamente — a convenção está em `navegacao.md` e o precedente em
`REC-009`. O mesmo vale para a pendência 10 de FND-09 §14, que a referencia.

O estado vigente das duas passa a ser **`REC-011`** no
[ledger de reconciliação](./reconciliacao.md), na condição `parcial`: o owner
nomeado e a revisão executada ficam quitados; a re-revisão por titular
independente segue encaminhada, condicionada à constituição da área de
Segurança.
