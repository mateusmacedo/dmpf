# ADR-021: Alojar o mapeamento no `provider` da outbox e serializar na escrita

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

## Contexto

O padrão outbox do DMPF não cabe em um único bloco: o `application service`
escreve a mensagem na mesma transação do estado, o `provider` persiste a tabela
e o `app` (relay) drena e publica. Em algum ponto desse caminho, o `domain
event` produzido pelo domínio precisa virar um `integration event` — a forma de
wire que os consumidores recebem — e ser serializado. Essa conversão é uma
dependência de código: quem a executa passa a conhecer o `contract package`.
Este ADR responde a duas perguntas que ficaram abertas — em qual bloco a
conversão reside e em que momento ela ocorre.

A pergunta não é nova. O FND-03 fechou a sua §6.3 delegando explicitamente a
esta fundação «o bloco em que o mapeamento reside, escolhido dentro do que a
matriz de RFC §7.4 permite», e o fez porque havia recusado a atribuição original
da base conceitual (Parte-1 §6.3), que punha «um mapper na camada de aplicação».
Essa atribuição colide com a matriz: a aresta `application → contract` (célula
12) é proibida por P0-2, porque contrato de wire é do adapter, não da aplicação.

A matriz de RFC §7.4 reduz o espaço de escolha, mas não decide sozinha. Dos
quatro blocos que poderiam conhecer o `contract package`, ela elimina dois —
`application → contract` (célula 12) e `port → contract` (célula 24), ambos por
P0-2 — e deixa dois admissíveis, sob a condição de contexto C2: `provider →
contract` (célula 30) e `app → contract` (célula 18). Os dois são permitidos; a
matriz não os distingue. O que os distingue não é permissão, é **momento**: no
`provider` da outbox, a conversão ocorre na escrita, dentro da transação; no
`app` do relay, ocorreria na drenagem, depois do commit.

O momento importa por uma razão de correção, não de estilo. Entre a escrita de
um registro e a sua drenagem pode haver um deploy, e com ele um schema de wire
novo. Serializar depois do commit faria um fato antigo sair sob o contrato
vigente no instante da publicação, e não sob o vigente quando o fato ocorreu.

## Decisão

Adotar o `provider` da outbox como o bloco em que o mapeamento `domain event →
integration event` reside, e fixar a serialização **na escrita**, dentro da
transação vinculada à UoW (§2.2, `BLK-03`).

Em concreto:

- A porta da outbox recebe `(domain event, intenção de publicação)` — apenas
  tipos de domínio e de aplicação. O `provider` que a implementa mapeia e
  serializa dentro da transação, no passo de escrita da sequência canônica
  (§3.2).
- A porta expõe tipo de domínio, nunca tipo de wire: quem conhece o wire é quem
  implementa a porta. É o que mantém a célula 24 (`port → contract`) fora do
  caminho.
- A cadeia de dependências atravessa apenas células permitidas — `application →
  port` (10), `provider → domain` (25) e `provider → contract` (30), todas sob
  C2 — e nenhuma proibida (11, 12, 24).
- Serializar na escrita congela os bytes no commit, de modo que a mensagem
  publicada é contemporânea do fato.

Esta decisão fixa **onde** o mapeamento reside e **quando** ele ocorre. O que o
mapeamento produz — formato do integration event, codec, registry, namespacing,
política de compatibilidade e a assinatura concreta do mapeamento — não é
decidido aqui: é de FND-05, sob a âncora ANC-03.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Mapear e serializar na drenagem, no relay (bloco `app`, célula 18) — a matriz permite | Entre a escrita e a drenagem pode haver deploy com schema de wire novo; serializar na drenagem faria um fato antigo sair sob o contrato vigente na publicação, e não sob o vigente quando o fato ocorreu (§2.2 `rationale`) |
| Manter o mapeamento fora do `provider`, expondo o integration event na assinatura da porta (célula 24) | Uma porta que expõe tipo de wire deixa de ser abstração de saída e vira o próprio contrato, arrastando `application service` para o `contract package` por transitividade; a célula 24 é proibida por P0-2 (§2.2 `rationale`) |
| Alojar o mapper na camada de aplicação, como propunha a Parte-1 §6.3 (célula 12) | Contrato de wire é do adapter, contrato de aplicação é outro artefato; a célula 12 é proibida por P0-2, e o FND-03 §6.3 já recusara a atribuição (matriz de RFC §7.4) |

## Consequências

**Positivas:**

- A mensagem publicada é contemporânea do fato: os bytes ficam congelados no
  commit e imunes a uma correção de contrato aplicada entre a escrita e a
  drenagem.
- A célula 11 (`application → provider`) e a célula 12 (`application →
  contract`) permanecem intactas — o caso de uso grava a outbox por uma porta
  que expõe apenas tipo de domínio, sem conhecer driver, tabela ou wire.
- A fronteira com FND-05 fica nítida: este ADR responde por bloco e por momento,
  e o formato e o codec ficam com quem os detém.

**Negativas:**

- Custo aceito: uma correção do contrato de wire **não alcança** o que já está
  gravado na outbox. Um evento serializado sob um schema antigo é publicado como
  está, mesmo que o mapeamento tenha sido corrigido entre a escrita e a
  drenagem — e o que já está na outbox é justamente o que não deveria mudar.
  Reprocessá-lo exige reescrever o registro, não apenas reconfigurar o relay.
- O `provider` da outbox passa a conhecer o `contract package` como parte do
  caminho de escrita — uma superfície de dependência maior do que a de um
  armazenamento neutro que guardasse o fato em forma crua.

## Referências

- **ADR-010** — fixa a regra de dependência como a função `decide()` sobre os
  seis blocos e a condição de bloco C1 (matriz de blocos, RFC §7.3). É essa regra
  de dependência, aplicada à matriz de células de RFC §7.4, que reprova as células
  12 e 24 e admite as células 18 e 30 — estas ainda sujeitas a C2 (ADR-017); sem
  ela, o espaço de escolha deste ADR não se reduz de quatro candidatos a dois.
- **ADR-017** — decide a condição de contexto C2. As células 18 e 30 são
  permitidas **sob C2**, satisfeita aqui porque o `contract package` é superfície
  pública de integração; a residência do mapeamento no `provider` só é legítima
  por isso.
- Origem: `docs/dmpf/uow-inbox-outbox.md` (FND-04), §2.2, com a obrigação
  delegada por FND-03 §6.3 e a matriz de RFC §7.4 (células 12, 18, 24, 25 e 30).
- **SPEC-DBTRMM3X** — spec que esta série de ADRs implementa.
- **ARQ-448** (FND-11) — sub-spec que redige, promove e aceita os ADRs
  acionados.
