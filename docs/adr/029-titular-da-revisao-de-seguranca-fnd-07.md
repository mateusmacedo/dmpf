# ADR-029: Atribuir a revisão de Segurança do FND-07 ao owner do artefato, com divergência declarada e re-revisão condicionada

## Status

Aceito — 2026-08-30. Implementa SPEC-DK8QQSDQ.

## Contexto

O artefato `docs/dmpf/contexto-erros-seguranca.md` (FND-07) foi promovido em
`develop` pelo PR #10 carregando uma regra que declara a própria entrega
incompleta. `THR-03` diz, com todas as letras, que o threat model **não está
satisfeito** enquanto não for revisado por Segurança, e nomeia os três objetos
dessa revisão: a varredura das seis categorias STRIDE em cada um dos sete
vetores, as exclusões justificadas e a atribuição de owner de cada linha.

A story do FND-07 ([ARQ-444](https://lider-cap.atlassian.net/browse/ARQ-444))
fechou; o gate não. Ele é hoje a última pendência que mantém o épico
[ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436) em desenvolvimento.

O motivo de o gate nunca ter sido acionável é preciso e está registrado no
próprio artefato, em §7.10: o cabeçalho declara o **papel** do revisor — «um
representante de Segurança para §7 e §8, obrigatório» — e não a **pessoa**. Um
gate cujo executor não existe não é um gate exigente; é um gate inerte.

Quatro forças condicionam o desenho desta decisão:

- **Não há área de Segurança constituída.** Nenhum documento do acervo nomeia
  titular, e o `.github/CODEOWNERS` deste repositório permanece com o
  placeholder `@owner` do template. O ADR-028 já havia registrado a mesma
  ausência em escala maior — `CODEOWNERS` ausente em 10 dos 10 repositórios do
  inventário AS-IS — ao desenhar a Autoridade de Classificação Arquitetural
  como função separável do titular. A função de revisor de Segurança está na
  mesma condição: declarada, não provida.

- **Não existe via normativa de dispensa.** O escape hatch do FND-10 tem
  universo positivo fechado em `GOV-31` — dependência externa (E1), combinação
  fora do BOM (E2) e instrumento de governança do próprio FND-10 (E3) — e
  `GOV-32` N1 veda exceção sobre qualquer constraint P0. `THR-03` não é nenhum
  dos três objetos excepcionáveis. A alternativa de «declarar o gate
  dispensado» não está disponível, e `GOV-05` acrescenta que marcar critério de
  aceite por antecipação é defeito de rastreabilidade.

- **A pendência bloqueia o que a resolveria.** O ARQ-436 permanece em
  desenvolvimento por causa deste gate, e o épico subsequente do kernel
  ([ARQ-519](https://lider-cap.atlassian.net/browse/ARQ-519)) está bloqueado
  pelo ARQ-436. Aguardar a constituição de uma área de Segurança para destravar
  a fundação inverte a ordem: a plataforma que instituiria esse controle é
  justamente a que não pode ser construída enquanto ele não existir.

- **A divergência é explícita na origem.** A descrição da
  [ARQ-488](https://lider-cap.atlassian.net/browse/ARQ-488) separa os dois
  papéis sem ambiguidade — «o assignee atual responde pelo encaminhamento, não
  pela revisão». Atribuir a revisão ao assignee contraria esse texto, e a
  separação entre quem redige e quem revisa é precisamente o que dá valor a uma
  revisão de Segurança. Nada nesta decisão desfaz esse fato.

## Decisão

O papel de **representante de Segurança para §7 e §8 do FND-07** é exercido por
**Mateus Macedo Dos Anjos**, owner do artefato, enquanto não houver área de
Segurança constituída na organização.

A decisão é tomada com a divergência **declarada, não contornada**: quem revisa
é quem redigiu, e a revisão que daí resulta é mais fraca do que a que o
cabeçalho do artefato pretendia. Três garantias compensatórias acompanham a
atribuição, e nenhuma delas substitui a revisão independente:

1. **A conferência é reproduzível.** Os critérios objetivos — cobertura das seis
   categorias por vetor, presença de motivo em cada exclusão, presença de owner
   em cada linha, existência das regras citadas nas mitigações — são
   verificáveis por varredura do artefato, sem depender do julgamento de quem a
   executou. O parecer descreve o critério com precisão suficiente para que
   qualquer pessoa reconfira o resultado.

2. **O parecer separa o mecânico do julgamento.** O que foi conferido por
   critério objetivo aparece em seção distinta do que foi julgado quanto ao
   mérito. A separação torna visível onde o viés de auto-revisão pode ter
   atuado, em vez de diluí-lo em prosa uniforme.

3. **A re-revisão é condicionada, não facultativa.** Constituída a área de
   Segurança, §7 e §8 voltam à revisão por titular independente da autoria. A
   condição é registrada aqui e no parecer, e o resultado desta revisão vale até
   lá — não a substitui em definitivo.

O ato desta decisão é **nomear o titular**, não aprovar o threat model. O
desfecho da revisão é matéria do parecer registrado em
`docs/dmpf/revisao-seguranca-fnd-07.md`, e pode ser aprovação, aprovação com
ressalvas ou reprovação. Esta decisão torna o gate acionável; ela não decide o
resultado do acionamento.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Manter o gate aberto até haver área de Segurança constituída | Perpetua a inércia que a ARQ-488 existe para corrigir. A pendência já sobreviveu à entrega que a criou, e a espera bloqueia indefinidamente o ARQ-436 e o ARQ-519 — inclusive a plataforma em que o controle viria a ser exercido |
| Invocar o escape hatch para dispensar `THR-03` | Vedado na admissão. `GOV-31` fecha o universo excepcionável em E1, E2 e E3, e `THR-03` não é nenhum dos três; `GOV-32` N1 veda exceção sobre constraint P0 |
| Declarar o critério de aceite satisfeito sem parecer | `GOV-05` trata marcar critério por antecipação como defeito de rastreabilidade, no mesmo sentido em que RFC §13.2 trata a citação de ADR por número definitivo antes da promoção |
| Contratar revisor externo ad hoc para esta revisão | Descartada por viabilidade, não por mérito — é a alternativa que **produziria o melhor parecer**. Contratar exige processo organizacional que não existe instalado, e o gate bloqueia hoje o fechamento do ARQ-436 e o início do ARQ-519. A ressalva honesta: o custo dessa escolha é justamente a independência que só ela traria, e é por isso que a re-revisão fica condicionada em vez de dispensada |
| Atribuir o papel a Arquitetura como área, e não a uma pessoa | `REP-03` do FND-05 estabelece que owner de diretório de contrato é equipe, não pessoa, mas §7.1 do FND-07 exige owner por **linha de ameaça**, e o cabeçalho pede um **representante**. Trocar a pessoa pela área aqui reintroduziria a inércia: uma área sem titular designado não executa revisão |

## Consequências

**Positivas:**

- O gate `THR-03` passa de inerte a acionável, e o critério de aceite
  correspondente da `SPEC-XQWGGAXF` deixa de estar travado por ausência de
  executor.
- A fraqueza da auto-revisão fica registrada como decisão rastreável, com data e
  condição de reversão, em vez de virar silêncio no histórico.
- O ARQ-436 pode ser concluído, e o ARQ-519 desbloqueado, sem que nenhuma regra
  do acervo seja relaxada.
- Fica um precedente utilizável para a pendência 2 do FND-08 — a validação do
  runbook por SRE, também sem owner nomeado, que já cita o tratamento dado ao
  `THR-03` como referência.

**Negativas:**

- A revisão não tem a independência que o cabeçalho do FND-07 exige. O parecer
  resultante é mais fraco do que seria o de um representante de Segurança
  distinto do autor. **Custo aceito:** as garantias compensatórias limitam a
  fraqueza e a tornam visível; nenhuma delas a elimina, e este ADR não promete a
  independência que não tem.
- O julgamento de mérito sobre as exclusões e sobre a suficiência das mitigações
  permanece exposto ao viés de quem escreveu as regras conferidas. **Custo
  aceito:** a camada mecânica do parecer é reproduzível por terceiros e a de
  julgamento fica isolada em seção própria — o viés é confinado e localizável,
  não neutralizado.
- A condição de re-revisão cria trabalho futuro: constituída a área de
  Segurança, §7 e §8 voltam à fila. **Custo aceito:** repetir a revisão é
  preferível a tratar como definitivo um parecer que nasce com validade
  condicionada.
- Um leitor externo que encontre o parecer sem este ADR poderia tomá-lo por
  revisão independente. **Custo aceito:** a mitigação é referência recíproca — o
  cabeçalho do FND-07, a §7.10 e o próprio parecer apontam para esta decisão —,
  e ela depende de o leitor seguir a referência.

## Referências

- **ADR-028** — institui a Autoridade de Classificação Arquitetural como função
  fechada, «indicada por Arquitetura e revisada por Segurança», e registra
  `CODEOWNERS` ausente em 10 dos 10 repositórios do inventário AS-IS. É o
  precedente direto: uma função de Segurança declarada sem titular provido.
- **ADR-007** — mantém `plans/` fora do versionamento. Condiciona a limitação
  declarada em §1.3 do parecer, já que a conferência da matriz do baseline
  incide sobre `Parte-1 §14`, não versionada.
- **Origem:** FND-07 — `contexto-erros-seguranca.md`, §7.10 (`THR-03`) e §11.4
  (pendência 4), sob a âncora ANC-05 da RFC v0.1. O artefato **não é editado**
  por esta decisão: o estado vigente da pendência passa a ser `REC-011` no
  ledger de reconciliação, na condição `parcial`, pela mesma regra que
  `REC-009` aplicou aos irmãos que transcrevem a faixa antiga de ADRs.
- **SPEC-DK8QQSDQ** — spec desta entrega.
- **SPEC-XQWGGAXF** — spec do FND-07, cujo critério de aceite do threat model
  este ADR destrava.
- **ARQ-488** — story que migrou o gate para fora do FND-07 e o tornou
  entregável.
