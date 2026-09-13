---
paths:
  - "**/*.ts"
  - "**/*.go"
---

# Disciplina de comentários

Comentários são a **exceção**. O default ao gerar e ao editar código é **zero
comentários**. A verdade vive no código; um comentário que repete, narra ou
decora só acrescenta ruído e apodrece quando o código muda (Robert C. Martin,
*Clean Code*, cap. 4).

Antes de escrever qualquer comentário, torne o código revelador: nomes
melhores, extração de função, tipos e estrutura. Comente o **porquê**, nunca o
**quê**.

## Quando comentar (única porta de entrada)

Escreva um comentário **somente** se as duas condições abaixo falharem em
resolver a leitura:

1. O código **não é revelador nem autoexplicativo** para um humano que conhece
   a linguagem e o domínio, **e** não dá para torná-lo assim sem prejuízo.
2. A complexidade cognitiva para um humano é **alta** (invariante sutil,
   restrição externa, workaround, consequência perigosa, escolha
   contra-intuitiva).

Se o nome da função, dos parâmetros e o corpo já dizem o que acontece, **não
comente**. Na dúvida, não comente.

## Onde o comentário **não** vai

- **Não** coloque comentário no topo de cada comportamento (função, método,
  handler, caso de uso, bloco, ramo). Preâmbulo em todo símbolo é ruído de
  geração — proibido.
- **Não** comente o corpo da implementação por hábito. Comentário **dentro**
  da função só em **extrema necessidade**, e só para cumprir as duas
  condições acima. Um bloco que precisa de narração passo-a-passo deve ser
  extraído e nomeado, não legendado.
- **Não** use comentário para “documentar o óbvio” (o que a assinatura já diz,
  o que o identificador já nomeia, o que o próximo `if` já faz).

## Prefira código autoexplicativo

- Renomeie identificadores até a intenção ficar evidente sem comentário.
- Extraia uma função com nome descritivo no lugar de um comentário que resume
  um bloco.
- Reserve o comentário para o que o código não consegue expressar sozinho: a
  razão de uma escolha.

## Evite (comentários ruins)

- **Redundante** — repete o identificador adjacente (`// incrementa contador`
  antes de `counter += 1`).
- **Preâmbulo de comportamento** — `// Creates the order`, `// Reserve reserva
  o pedido`, JSDoc/`godoc` em função interna só porque a função existe.
- **Narração passo-a-passo** — `// Passo 1: ...`, `// First, we ...` (frases-
  âncora típicas de geração por IA), inclusive **dentro** do corpo.
- **Banner decorativo** — `// ====`, `// ----`, `// ### Helpers ###`.
- **Código comentado** — o histórico vive no git; remova.
- **Ruído / óbvio** — `// construtor`, `// retorna`, comentário de fechamento
  de chave.
- **Volume excessivo** — densidade alta de comentários ou blocos longos
  narrando o que o código já diz.

## Prefira (comentários bons — raros)

Só depois de passar pela porta de entrada:

- **Porquê / intenção** — decisão não óbvia (`// linear search: N < 8 in
  practice, hashing does not pay off`).
- **Aviso de consequência** — `// not thread-safe: serialize access`.
- **Marcador canônico** — `WHY:`, `INVARIANT:`, `WORKAROUND(<id>)`, `SAFETY:` e
  `NOTE:` documentam intenção declarada; trate-os como legítimos, não como
  ruído. Não os espalhe em todo símbolo.
- **TODO com ID de tracker** — `TODO(ENG-123): ...`. Um `TODO` sem ID fica sem
  rastreabilidade.
- **Contrato público que o identificador não carrega** — restrições, erros,
  invariantes de uma API **exportada**. Não parafraseie o nome.
- **Cabeçalho legal** — licença/copyright, quando o arquivo já o exige.

## Quando o comentário "vira docs/"

Justificativa arquitetural, decisão entre arquivos ou explicação longa (acima
de poucas linhas) pertence a um documento durável: mova para `docs/` ou
registre um ADR em `docs/adr/` e deixe no código apenas um marcador `WHY:`
apontando a decisão.

## Revisão de comentários

Esta regra orienta a revisão manual; não é aplicada por hook. Ao revisar um
diff (e ao gerar código), use o catálogo acima como checklist:

1. **Comentário introduzido agora** — se não passa na porta de entrada, remova.
   Se passa, reescreva o porquê ou use um marcador canônico. Preâmbulo no topo
   de função e narração no corpo são rejeitados por padrão.
2. **Comentário pré-existente** — preserve por padrão; registre no resumo da
   entrega que o encontrou e sugira uma limpeza dedicada. Altere apenas com
   pedido explícito, para não misturar escopos.
3. **Sempre** — liste no resumo final o que foi limpo e o que foi deixado,
   dando rastreabilidade à revisão.

Sinais úteis ao revisar: densidade alta de comentários num arquivo, comentário
imediatamente acima de quase toda função, blocos longos que narram o código e
semelhança textual forte entre um comentário e o identificador que ele precede.

---

## Go: disciplina de comentários equivalente

Em Go o comentário de identificador **exportado** tem papel formal: godoc
transforma esse comentário em documentação de API. Isso **não** autoriza
comentar cada função, método ou bloco.

### O que muda

- **Default continua zero.** Funções, métodos e tipos **não exportados** não
  levam comentário no topo. Comportamentos internos não ganham preâmbulo.
- **Doc comment só em exportado, e só como contrato.** Começa com o próprio
  nome (`// User represents an authenticated account`). Descrever o "quê" aqui
  é o contrato público (`go doc`, `pkg.go.dev`) — **não** uma permissão para
  parafrasear o identificador nem para copiar o padrão em símbolos internos.
- **Dentro de funções** (exportadas ou não): extrema necessidade. Porquê, não
  quê. Narração passo-a-passo e redundância são ruído.
- **TODOs** seguem `// TODO(usuário): descrição`; prefira o ID do tracker
  (`// TODO(ENG-123): ...`).
- **Marcadores** `WHY:`, `INVARIANT:`, `WORKAROUND(<id>)`, `SAFETY:` e `NOTE:`
  funcionam igual — raros, só quando a porta de entrada se aplica.
- **Diretivas** como `//nolint:...` ou `//go:build` não são prosa; mantenha-as
  sem espaço após `//`, como a toolchain exige.

### Justificativa longa

A orientação de mover explicação longa para `docs/` ou `docs/adr/` se aplica
igualmente. Em Go deixe no código apenas um `// WHY:` apontando a decisão.
