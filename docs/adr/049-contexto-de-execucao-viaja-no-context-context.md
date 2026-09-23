# ADR-049: O contexto de execução viaja no `context.Context`, do ingress ao provider

## Status

Aceito — 2026-09-21. Supersede parcialmente `CTX-03` e `CTX-05` de FND-07 (`docs/dmpf/contexto-erros-seguranca.md` §3.2), e com eles o godoc de `ports.UnitOfWork.Within` que os citava. Do `CTX-03`, supersede a primeira metade — "o contexto é passado explicitamente como argumento ao `application service`"; a segunda metade, que mantém a UPR fora do alcance do contexto, **permanece intacta**. Do `CTX-05`, supersede a proibição de que a correção dependa de valor ambiental, restrita ao contexto de execução. `CTX-01`, `CTX-02`, `CTX-04` e `CTX-06` permanecem.

Implementa [SPEC-9B6SHEH8](../specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md).

## Contexto

A Fase 3 da spec realiza `IDN-14` — a imposição do isolamento por tenant não pode depender de convenção de código. O mecanismo escolhido é o que a própria norma nomeia entre os legítimos: um cliente de persistência que injeta o escopo, o `postgres.Table[ID,S]`, espelhando o `memory.Table[ID,S]` que já existe.

Ao desenhá-lo, a premissa do plano encontrou um impedimento nas assinaturas do kernel. `ports.Repository[ID,S]` e `ports.Reader[ID,S]` declaram `Load(ctx context.Context, id ID)` e `Save(ctx context.Context, id ID, state S, expected Version)`. Nenhuma delas recebe o contexto de execução, e nenhuma pode recebê-lo sem que o tipo inteiro mude. O `ExecutionContext` chegava ao `application service` como argumento e morria ali: `Within(ctx, fn)` não o repassava, e o `bind` que a composition root escreve só via o `*postgres.Tx`. O `Reader` do caminho de consulta era pior — campo fixo do `Service`, montado uma vez no wiring, enquanto `IDN-11` alcança toda leitura.

As três saídas mapeadas mediam assim:

| Saída | Alcance medido |
|---|---|
| `Within` passa a tomar o contexto | 68 call sites de `.Within(`, 8 realizações, ~40 `NewUnitOfWork` com `bind`; o `Reader` fora da transação continua sem canal |
| Tipo escopado novo no `port` | 3 services, 7 call sites de negócio, composition roots e harnesses; dois tipos onde havia um |
| O contexto viaja no `context.Context` | 16 arquivos de produção, 27 de teste, e a emenda normativa deste ADR |

A terceira esbarrava em `CTX-05`, que proíbe exatamente isso — e o godoc de `ports/uow.go` já havia traduzido a proibição em decisão de projeto: "R is built by the realization from the open transaction and is the only path by which transactional ports reach fn: not context.Context (CTX-05)".

## Decisão

**Informação transitória do escopo de uma requisição vive no `context.Context`, do início ao fim dessa requisição.** O contexto de execução é a informação transitória por excelência — nasce no ingress, acompanha cada passo da mesma requisição e morre com ela —, então é ali que ele viaja.

O critério vale mesmo onde contradiz decisão anterior deste repositório, e mesmo onde o custo de aplicá-lo é grande. Foi por esse critério que a decisão foi tomada, não pelo tamanho do diff de cada alternativa.

### O que muda

O kernel passa a declarar o carrier canônico em `ports`, ao lado do tipo:

```go
func WithExecutionContext(ctx context.Context, ec ExecutionContext) context.Context
func ExecutionContextFrom(ctx context.Context) (ExecutionContext, bool)
func RequireExecutionContext(ctx context.Context) (ExecutionContext, error)
```

Os dois carriers locais que existiam — em `libs/backend/go/http/identity.go` e, duplicado, nos `app/rpc/interceptors.go` de `orders` e de `reservations` — passam a delegar a ele. A borda continua montando a instância, como `CTX-02` exige; o que muda é que ela a deposita no `ctx` em vez de entregá-la adiante por parâmetro.

O parâmetro `execution ports.ExecutionContext` sai das assinaturas: dos onze métodos dos três `application service`, de `application.AuthorizeWithContext[C]` e de `http.ResolveIdentity`. Quem precisa do contexto passa a obtê-lo do `ctx` que já recebe.

### O que não muda

A UPR continua sem receber o contexto, e o que alcança o `domain` continua sendo valor extraído. Essa metade do `CTX-03` é a que preserva FND-03 `FRT-03`, e ela não está em jogo aqui: nenhuma linha do bloco `domain` é tocada.

`CTX-01` (os nove campos e a presença por campo), `CTX-02` (tipo no `port`, instância montada no `app`), `CTX-04` (imutabilidade depois de montado) e `CTX-06` (sujeito e tenant nunca têm por fonte um campo da entrada) permanecem como estão.

### O fail-closed que substitui a garantia do compilador

Com o parâmetro, esquecer o contexto não compilava. Sem ele, a ausência passa a ser condição de execução, e precisa de resposta declarada em vez de comportamento acidental.

`RequireExecutionContext` devolve erro quando o `ctx` não traz contexto, e cada consumidor o trata como negação, não como caminho permissivo. `IDN-15` já normatiza o desfecho — "ausência não é permissão, e a negação não depende de haver regra explícita para o caso" —, e `IDN-17` já fecha o default: omissão nega. A autorização e o provider de persistência negam na ausência; nenhum dos dois infere tenant nem sujeito, o que `IDN-20` proíbe.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| `Within(ctx, execution, fn)` e `bind func(tx, execution) R` | Mantém o contexto fora do `ctx`, que é o critério desta decisão. Move 68 call sites e 8 realizações, altera o contrato que a suíte `RunUnitOfWorkContract` executa, e ainda deixa o `Reader` fora da transação sem canal |
| `ports.ScopedUnitOfWork[R]` com `For(ExecutionContext) UnitOfWork[R]` | Mesmo desvio do critério, e cria dois tipos onde havia um. Enquanto o `UnitOfWork` não escopado seguisse construível por uma composition root de contexto, existiria rota de contorno — a dependência de disciplina que `IDN-14` proíbe |
| Row-level security com `SET LOCAL` | Descartada por medição anterior a este ADR: nos três ambientes o role que abre a conexão é superuser e dono das tabelas, e RLS não se aplica a ele. O registro fica em ADR próprio da Fase 8 |
| Manter o parâmetro e apenas acrescentar o carrier | Deixaria duas fontes para o mesmo valor, com a pergunta "qual vale quando divergem?" sem resposta declarada. Uma fonte única é o que torna o critério verificável |

## Consequências

**Positivas:**

- O contexto alcança o provider de persistência, que é o que a Fase 3 precisa para realizar `IDN-14` sem mudar `ports.Repository` nem `ports.Reader`.
- `ports.UnitOfWork.Within` fica intacto: as 8 realizações, os 68 call sites e a suíte de conformidade seguem válidos.
- Uma regra única passa a governar toda informação transitória de requisição, em vez de decisão caso a caso sobre o que viaja por parâmetro e o que viaja por ambiente.
- Os dois carriers locais duplicados entre `orders` e `reservations` deixam de existir, absorvidos pelo canônico do `port`.

**Negativas:**

- A garantia por compilador é trocada por verificação em execução. A mitigação é `RequireExecutionContext` com negação declarada, mas ela só age quando o caminho é exercido; uma rota sem teste que esqueça de depositar o contexto falha em execução, não em compilação.
- A Fase 2B da mesma spec é parcialmente desfeita. O commit `699c662` existiu para trocar `AuthorizeFunc` por `AuthorizeWithContext`, acrescentando o parâmetro que este ADR remove. O tipo não regride ao anterior: passa a `Authorize[C] func(ctx, cmd) error`, lendo do `ctx`.
- Dois artefatos do acervo normativo são emendados — `contexto-erros-seguranca.md` e `testes-interop.md`, este último porque registra o modo de verificação de `CTX-03` e `CTX-05`.
- O modo de verificação dessas duas regras muda de natureza. Deixa de ser inspeção de assinatura e passa a ser inspeção de onde o `ctx` é enriquecido e de que nenhum consumidor prossegue sem contexto.
