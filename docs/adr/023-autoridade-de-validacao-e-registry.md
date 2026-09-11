# ADR-023: Fixar no repositório de contratos a autoridade de validação com Buf e deferir a escolha de registry em runtime

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

Este ADR promove a **parte decidível** do acionamento `ADR-DMPF-N` (FND-05 §9.2):
a autoridade de validação de contratos, fixada no repositório de contratos. A
**escolha de um registry de schemas em runtime** — se existe, e qual — permanece
**pendente**, registrada na subseção própria da seção Decisão: a FND-05 não traz o
material para decidi-la, pois a redação dessa parte de `ADR-DMPF-N` depende
materialmente do estudo interno de Kafka Schema Registry anexado ao épico ARQ-436
(§9.2), que não integra a FND-05. A escolha permanece, assim, matéria aberta dentro
do próprio `ADR-DMPF-N`, não de um ADR distinto deste.

## Contexto

O DMPF publica contratos de wire — os `.proto` de Protobuf e o perfil CloudEvents
do envelope — a partir de um repositório de contratos dedicado, cuja governança
Buf verifica cada contrato antes da publicação. A âncora ANC-03 levanta uma
pergunta que precisa de resposta explícita: **onde reside a autoridade que valida
um contrato?**

Há dois candidatos, e eles não são intercambiáveis. O primeiro é a validação
**no repositório**, em tempo de build: os gates Buf operam sobre os arquivos e os
descriptors versionados, antes de qualquer publicação. O segundo é a validação
**em runtime**, tipicamente ancorada em um Schema Registry, que verificaria as
mensagens ao trafegarem pelo broker.

Três forças condicionam a decisão. A primeira vem de um estudo interno sobre
Schema Registry: a validação broker-side em runtime confere apenas o
**identificador** da mensagem e o **formato de wire** — ela **não** valida o
payload contra o schema. Uma organização que confie somente nela fica com
contratos verificados no envelope e livres no corpo. A segunda é a política de
capabilities do bloco `contract package`, que admite apenas `pure` e `wire.codec`
(ADR-015): um registry que se tornasse pré-requisito de desserialização colocaria
uma chamada de rede no caminho de leitura do contrato, violando essa política. A
terceira é a fronteira do material disponível: a FND-05, que este ADR promove, não
traz o insumo para escolher o registry. Esse insumo — o estudo interno de Kafka
Schema Registry anexado ao épico ARQ-436 (§9.2) — não integra a FND-05, e sem ele a
escolha não é redigível aqui.

A decisão precisa, então, fixar o que **pode** ser decidido agora — a autoridade
que independe da escolha de runtime — e deixar a escolha de registry aberta dentro
do próprio `ADR-DMPF-N`, até que o insumo do ARQ-436 seja incorporado.

## Decisão

Fixar a autoridade de validação de um contrato **no repositório de contratos**
como o Buf, executado nos gates de CI: a verificação é **local** — opera sobre os
arquivos e os descriptors do repositório —, é **obrigatória** e é **fail-closed**.
Nenhum contrato entra no repositório validado apenas por um serviço externo.

A autoridade de validação fica, assim, **separada em duas**: a autoridade do
repositório, decidida aqui, e qualquer autoridade em runtime, deferida. A do
repositório é a que garante a verificação que importa — a governança do contrato
antes da publicação, sobre os arquivos e os descriptors: lint `STANDARD`, breaking
`FILE` e reprodutibilidade da geração (§6.6) —, e essa garantia vale
**independentemente** do que a decisão de registry vier a resolver sobre runtime.

Fica também fixado o invariante que a decisão deferida **não pode alterar**, porque
decorre da âncora e não de preferência: a desserialização de um contrato **nunca**
depende de resolução remota de schema (`ENV-02`, `ENV-23`). Um registry adotado em
runtime **acrescenta** verificação e catalogação; ele **não** se torna pré-requisito
de leitura do contrato, sob pena de introduzir `io.network` em um bloco cuja
política admite apenas `pure` e `wire.codec` (ADR-015). O atributo `dataschema` é
identificador, não endereço de resolução obrigatória.

### Pendência declarada — o registry de schemas em runtime

Se existe uma segunda autoridade em runtime — um Schema Registry — e qual, **não**
é decidido aqui. Essa é a matéria residual de `ADR-DMPF-N`, e permanece **aberta**:

- **Quem decide (owner):** a série de ADRs estruturais do DMPF, sob FND-11
  (ARQ-448), que detém a redação,
  a promoção e o aceite de `ADR-DMPF-N`. A escolha é **deferida**, não delegada a
  outrem.
- **Insumo obrigatório:** o estudo interno de Kafka Schema Registry e validação de
  esquemas anexado ao épico ARQ-436,
  do qual a redação de `ADR-DMPF-N` depende materialmente.
- **Consumidor condicionado:** FND-06
  (ARQ-443, sob ANC-04), a quem
  cabe a **operação** de qualquer registry adotado, e cuja operação está
  condicionada a esta decisão.
- **Prazo:** a fonte de origem **não fixa prazo nem data** para essa decisão — o
  elo fica explicitamente aberto, e não há data a declarar aqui sem inventá-la.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Adotar a validação broker-side em runtime como **autoridade única** de validação | §6.5 `rationale`: a validação broker-side verifica apenas o identificador e o formato de wire, e **não** valida o payload contra o schema; confiar só nela deixa os contratos verificados no envelope e livres no corpo. Fixar a autoridade no repositório, antes da publicação e sobre os arquivos, garante a verificação que importa |
| Tornar o registry um **pré-requisito de desserialização** — resolver o schema em rede ao ler o contrato | §2.1 `rationale` (`ENV-02`) e §4.4 `rationale` (`ENV-23`): isso colocaria uma chamada `io.network` no caminho de desserialização, dentro de um bloco cuja política admite apenas `pure` e `wire.codec`; `dataschema` é identificador, não endereço de resolução obrigatória. É invariante da âncora, não preferência, e a decisão de registry não pode afrouxá-lo |

O registro de acionamento (§9.2) lista uma terceira alternativa para `ADR-DMPF-N`
— **ausência de qualquer catalogação em runtime**. Ela **não** figura como
descartada aqui porque continua sendo uma das saídas legítimas da decisão de
registry deferida: §6.5 `rationale` confirma que a autoridade do repositório
garante o que importa «independentemente do que o ADR decidir sobre runtime».
Listá-la como rejeitada anteciparia a pendência que este ADR preserva em aberto.

## Consequências

**Positivas:**

- Todo contrato é verificado sobre os seus próprios arquivos e descriptors, antes
  da publicação, num gate fail-closed: lint `STANDARD`, breaking `FILE` e
  reprodutibilidade da geração (§6.6). E porque o schema é governado, `ENV-05` veda
  no payload os campos de escape (`map<string, string>` extra, `bytes` raw, `oneof`
  genérico) que a validação broker-side sozinha, restrita a envelope e formato de
  wire, deixaria livres no corpo.
- O bloco `contract package` mantém a política de capabilities restrita: nenhuma
  chamada `io.network` entra no caminho de desserialização, de modo que os
  consumidores leem o contrato sem ida à rede e o bloco permanece `pure` e
  `wire.codec` (ADR-015 preservado).
- Fixar agora a autoridade do repositório destrava a governança de contratos sem
  esperar pela decisão de registry, que permanece uma decisão de primeira classe e
  revisável — como a ANC-03 exige —, em vez de contrabandeada como configuração.

**Negativas:**

- **Custo aceito:** a única autoridade de validação decidida por este ADR atua **antes
  da publicação**, no repositório, e não sobre a mensagem em trânsito. Enquanto a
  decisão de registry não for tomada, o payload já publicado não tem segunda
  autoridade que o valide em runtime — aceita-se essa lacuna de runtime, que só
  se fecha quando `ADR-DMPF-N` decidir o registry.
- A pendência é, ela própria, um custo: a operação de FND-06 fica condicionada a
  uma decisão que este ADR defere, e a fonte não fixa prazo — o elo permanece
  aberto até FND-11 redigir a escolha de registry a partir do estudo interno.

## Referências

- ADR-015 — restringe as capabilities externas por bloco (default deny), fixando o
  `contract package` em apenas `pure` e `wire.codec`. É a fronteira invariante que
  a escolha de registry não pode cruzar: um registry como pré-requisito de
  desserialização introduziria `io.network` no bloco, e por isso é a alternativa
  rejeitada nesta decisão.
- ADR-010 — institui os seis blocos e a regra de dependência. O `contract package`,
  cuja autoridade de validação este ADR fixa, é um desses blocos; a sua política de
  capabilities é o que sustenta o invariante de que a leitura do contrato não
  depende da rede.
- Artefato de origem: `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05), §6.5
  (autoridade da validação, `BUF-09`) e §9.2 (registro de acionamento de
  `ADR-DMPF-N`, e insumo do ARQ-436 de que a redação depende); a governança que a
  autoridade do repositório aplica está em §6.6 (gates de format, lint `STANDARD`,
  breaking `FILE` e reprodutibilidade), e o veto a escape hatches no payload em §2.2
  (`ENV-05`); fundamentos correlatos do invariante em §2.1 (`ENV-02`) e §4.4
  (`ENV-23`).
- SPEC-DBTRMM3X — spec que esta série de ADRs implementa.
- ARQ-448 — ARQ-448 (FND-11: redação,
  promoção e aceite da série de ADRs do DMPF).
