# ADR-022: Adotar o CloudEvents Protobuf oficial, fixar `proto_data` como modalidade única e derivar o `payload_hash` dos bytes transportados

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

## Contexto

O envelope assíncrono é a parte do contrato que **todo** consumidor lê, inclusive
o que não conhece o payload: um roteador que decide destino, um coletor que
correlaciona traços e uma ferramenta de operação que inspeciona uma DLQ precisam
dos mesmos metadados, e nenhum deles deveria ter de desserializar o corpo da
mensagem para obtê-los. Ao consolidar a base conceitual da plataforma
(Parte-1 §7), três decisões de forma ficaram em aberto e foram delegadas por
escrito ao perfil de contratos do DMPF.

A primeira é o próprio formato do envelope: adotar o `io.cloudevents.v1.CloudEvent`
do CloudEvents em Protobuf, ou desenhar um envelope proprietário equivalente em
conteúdo. A segunda é a modalidade do corpo. O formato oficial oferece três no
`oneof data` — `binary_data` (bytes opacos), `text_data` (conteúdo textual) e
`proto_data` (com `google.protobuf.Any`) — e a base conceitual recomendava a
última «quando a toolchain certificada oferecer suporte homogêneo», deixando a
condição sem resolver e exigindo padronizar **uma** modalidade por major. A
terceira é a fórmula do `payload_hash` que a inbox usa para detectar reuso de
identificador: a decisão de unidade de trabalho (FND-04) fixou apenas duas
propriedades — H1, o hash é estável sob serializações equivalentes entre stacks;
H2, o hash cobre só o conteúdo de negócio — e nenhuma fórmula.

As forças em jogo são a autodescrição do corpo no wire, a comparabilidade do hash
entre as duas stacks (Go e TypeScript), a interoperabilidade com ferramentas de
terceiros, e a política de capability do bloco `contract package`, que admite
apenas `pure` e `wire.codec` e por isso não pode resolver schema por chamada de
rede durante a desserialização.

## Decisão

Fixar as três formas, no escopo do bloco `contract package`:

- **Formato oficial.** O envelope é o `io.cloudevents.v1.CloudEvent` do formato
  Protobuf oficial do CloudEvents, com um perfil organizacional de validação
  aplicado sobre ele. O perfil só **acrescenta** obrigatoriedade — não remove
  atributo da especificação nem altera a semântica de nenhum. Envelope
  proprietário, ainda que equivalente em conteúdo, não é conforme.
- **Modalidade única.** A modalidade oficial da primeira major é `proto_data`, com
  o payload empacotado em `google.protobuf.Any`. `binary_data` e `text_data` não
  são conformes nesta major — nenhum é fallback, exceção por contexto ou escolha
  do produtor. `datacontenttype` é `application/protobuf` e `dataschema` é o
  *type URL* do `Any`. Uma stack cuja biblioteca não suporte `Any` é exceção de
  toolchain, tratada no rito de bootstrap, não exceção de contrato.
- **Fórmula do hash.** O `payload_hash` é SHA-256, em hexadecimal minúsculo,
  computado **exclusivamente** sobre os bytes de `Any.value` do `data` exatamente
  como transportados, **sem** desserializar e **sem** reserializar. Nenhum
  atributo do envelope entra no cálculo. A versão da fórmula é `1`, é propriedade
  da major do perfil e não viaja no envelope. Alterar qualquer um destes
  elementos é mudança **breaking** do perfil e exige nova major.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Envelope proprietário equivalente em conteúdo | Obrigaria a plataforma a manter o codec do envelope como infraestrutura crítica para sempre, e a primeira integração com um sistema externo o converteria de volta, com perda; o formato oficial já traz o binding por transporte, as bibliotecas das duas stacks e o vocabulário que ferramentas de terceiros reconhecem (§3.1 `rationale`). |
| `binary_data` como modalidade oficial | Elimina a autodescrição onde ela mais importa: `dataschema` passa a ser a única afirmação sobre o tipo do corpo, sem nada que a contradiga quando estiver errada — bytes de um contrato viajando sob o `dataschema` de outro produzem desserialização silenciosamente errada (§4.2 `rationale`). |
| `text_data` como modalidade | Carrega conteúdo textual, mas o payload de negócio é uma mensagem Protobuf; usá-lo exigiria uma representação textual do contrato que nenhuma regra define e que reintroduziria o problema de canonicalização que a fórmula do hash evita (§4.2 `rationale`). |
| Admitir as duas modalidades, com escolha por contexto | Quebra três coisas: o `payload_hash` ganha dois escopos possíveis para os mesmos bytes e a comparação entre produtores deixa de ser significativa; o oráculo de round-trip passaria de duas para quatro combinações; e o consumidor genérico teria de tratar duas formas de extrair o corpo — custo que recairia sobre todos os consumidores, para sempre (§4.2 `rationale`). |
| `payload_hash` sobre projeção canônica e versionada do conteúdo de negócio | Satisfaz H1 sem condição, mas troca uma dependência verificável por uma invisível: Protobuf não oferece forma canônica normativa entre implementações, então a projeção teria de fixar ordem de campos, encoding de cada tipo e tratamento de ausência de forma idêntica em Go e TypeScript, sem árbitro externo — um segundo formato de wire, não padronizado, no caminho crítico da deduplicação (§4.3 `rationale`). |

## Consequências

**Positivas:**

- O consumidor genérico lê os metadados sem o schema do payload, e o *type URL* do
  `Any` viaja **dentro** dos bytes, tornando o par (`dataschema`, *type URL*)
  conferível sem confiar no produtor.
- Uma modalidade única mantém o `payload_hash` com um só escopo, o round-trip com
  duas combinações e a extração do corpo uniforme entre as stacks.
- Adotar o formato oficial herda o mapeamento para cada binding de transporte, as
  bibliotecas de Protobuf das duas stacks e o vocabulário reconhecido por
  ferramentas de terceiros.
- Hashear os bytes transportados dispensa uma canonicalização normativa entre
  implementações que o Protobuf não oferece, e torna a estabilidade do hash
  falsificável com um teste de bytes.

**Negativas:**

- **H1 do `payload_hash` passa a valer sob condição, e essa é a dívida própria
  desta decisão:** o hash só é estável entre stacks enquanto ninguém no caminho
  reescrever os bytes publicados. A byte-preservação por transporte não é
  garantida aqui — é dependência declarada sobre FND-06 (ARQ-443). Se um gateway,
  proxy ou re-emissor reserializar o corpo, o hash recomputado diverge e uma
  redelivery legítima é classificada R4 pela inbox, contida e nunca aplicada.
- **Custo aceito:** escolher a fórmula sobre bytes, em vez da projeção canônica,
  entrega uma condição falsificável no lugar de uma garantia incondicional — e essa
  garantia exigiria manter um segundo formato de wire.
- Fixar uma modalidade única transfere o atrito para quem tem toolchain
  divergente: uma stack sem suporte a `Any` precisa resolvê-lo no bootstrap, sem a
  saída de usar `binary_data`. O custo é assumido conscientemente para não
  fragmentar a modalidade e onerar todos os consumidores para sempre.

## Referências

- **ADR-021** — aloja o mapeamento no `provider` da outbox e serializa **na
  escrita**, uma única vez. É essa decisão que sustenta a condição H1 da fórmula
  escolhida: as duas stacks comparam os mesmos bytes porque a serialização não se
  repete a jusante.
- **ADR-015** — restringe as capabilities externas por bloco (default deny com
  allowlist por entrypoint), fixando o `contract package` em apenas `pure` e
  `wire.codec`. Escolher `proto_data` autodescritivo e vedar a resolução remota de
  schema na desserialização é o que mantém o bloco dentro dessa política, sem
  introduzir `io.network`.
- **ADR-023** — decisão irmã que fixa a autoridade de validação no repositório com
  Buf e defere a escolha de registry em runtime. Divide com este ADR o que a
  âncora ANC-03 exige (codec aqui, registry lá); este decide o codec autodescritivo
  (`proto_data` com `Any`), o que torna a resolução remota de schema **desnecessária**
  na desserialização e é consistente com a fronteira que o próprio ADR-023 fixa
  (`ENV-02`/`ENV-23`): um registry em runtime acrescenta, mas nunca vira pré-requisito
  de leitura do contrato.
- Origem: `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05), §3.1 (formato oficial),
  §4.2 (modalidade de payload) e §4.3 (fórmula do `payload_hash`); registro de
  acionamento de `ADR-DMPF-M` em §9.2, sob a âncora ANC-03.
- **SPEC-DBTRMM3X** — spec que esta série de ADRs implementa.
- **ARQ-448** (FND-11) — sub-spec que redige, promove e aceita os ADRs acionados.
