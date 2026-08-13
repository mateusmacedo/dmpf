---
paths:
  - "**/*.ts"
  - "**/*.go"
---

# Disciplina de comentários

Comentários são um mal necessário (Robert C. Martin, *Clean Code*, cap. 4). A
verdade vive no código; um comentário que repete, narra ou decora o código só
acrescenta ruído e apodrece quando o código muda. Antes de comentar, prefira
tornar o código autoexplicativo (nomes melhores, função extraída). Comente o
**porquê**, nunca o **quê**.

## Prefira código autoexplicativo

- Renomeie identificadores até a intenção ficar evidente sem comentário.
- Extraia uma função com nome descritivo no lugar de um comentário que resume um bloco.
- Reserve o comentário para o que o código não consegue expressar sozinho: a razão de uma escolha.

## Evite (comentários ruins)

- **Redundante** — repete o identificador adjacente (`// incrementa contador`
  antes de `counter += 1`).
- **Narração passo-a-passo** — `// Passo 1: ...`, `// First, we ...` (frases-âncora
  típicas de geração por IA).
- **Banner decorativo** — `// ====`, `// ----`, `// ### Helpers ###`.
- **Código comentado** — o histórico vive no git; remova.
- **Ruído / óbvio** — `// construtor`, `// retorna`, comentário de fechamento de chave.
- **Volume excessivo** — densidade alta de comentários ou blocos longos narrando
  o que o código já diz.

## Prefira (comentários bons)

- **Porquê / intenção** — explica uma decisão não óbvia (`// usa busca linear:
  N < 8 na prática, hash não compensa`).
- **Aviso de consequência** — `// não thread-safe: serialize o acesso`.
- **Marcador canônico** — `WHY:`, `INVARIANT:`, `WORKAROUND(<id>)`, `SAFETY:` e
  `NOTE:` documentam intenção declarada; trate-os como legítimos, não como ruído.
- **TODO com ID de tracker** — `TODO(ENG-123): ...`. Um `TODO` sem ID fica sem
  rastreabilidade; adicione o identificador da tarefa.
- **Docstring de API pública** — contrato de uma função/classe exportada.
- **Cabeçalho legal** — licença/copyright.

## Quando o comentário "vira docs/"

Justificativa arquitetural, decisão entre arquivos ou explicação longa (acima de poucas
linhas) pertence a um documento durável: mova para `docs/` ou registre um ADR em
`docs/adr/` e deixe no código apenas um marcador `WHY:` apontando a decisão.

## Revisão de comentários

Esta regra orienta a revisão manual; não é aplicada por hook. Ao revisar um diff,
use o catálogo acima como checklist:

1. **Comentário introduzido agora** — remova, reescreva explicando o porquê, ou
   converta num marcador canônico quando a intenção for legítima.
2. **Comentário pré-existente** — preserve por padrão; registre no resumo da
   entrega que o encontrou e sugira uma limpeza dedicada. Altere apenas com
   pedido explícito, para não misturar escopos.
3. **Sempre** — liste no resumo final o que foi limpo e o que foi deixado, dando
   rastreabilidade à revisão.

Sinais úteis ao revisar: densidade alta de comentários num arquivo, blocos longos
que narram o código e semelhança textual forte entre um comentário e o
identificador que ele precede costumam indicar ruído removível.

---

## 🔹 Go: disciplina de comentários equivalente

Em Go o comentário tem papel formal: godoc transforma comentários em documentação
de API. Isso ajusta o equilíbrio "porquê vs quê" para símbolos exportados.

### O que muda

- **Doc comments são API** — todo identificador exportado merece um comentário que
  começa com o próprio nome (`// User represents an authenticated account`,
  `// CreateUser persists a new user`). Aqui descrever o "quê" é intencional: é o
  contrato público, consumido por `go doc` e `pkg.go.dev`.
- **Dentro de funções** vale a mesma regra do TS: comente o porquê, não o quê.
  Narração passo-a-passo e redundância continuam sendo ruído.
- **TODOs** seguem `// TODO(usuário): descrição` e `go vet` reconhece a convenção;
  prefira o ID do tracker (`// TODO(ENG-123): ...`).
- **Marcadores** `WHY:`, `INVARIANT:`, `WORKAROUND(<id>)`, `SAFETY:` e `NOTE:`
  funcionam igual — documentam intenção declarada e são legítimos.
- **Diretivas** como `//nolint:...` ou `//go:build` não são prosa; mantenha-as sem
  espaço após `//`, como a toolchain exige.

### Justificativa longa

A orientação de mover explicação longa para `docs/` ou `docs/adr/` se aplica
igualmente. Em Go é comum manter um `docs/adr/` no próprio repositório e deixar no
código apenas um `// WHY:` apontando a decisão.
