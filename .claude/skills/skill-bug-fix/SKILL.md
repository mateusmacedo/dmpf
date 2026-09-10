---
name: skill-bug-fix
description: >
  Use esta skill quando o usuário pedir para "corrigir bug", "fix bug", "teste de regressão",
  "processo de correção", "validar fix" ou mencionar investigação, causa raiz, checklist de
  correção ou regressão. Cobre: isolamento de escopo, correção mínima, teste de regressão e
  validação pós-fix. Para debugging/investigação inicial, ver o agente `debug`; para refatoração,
  ver `skill-clean-code`; para testes em geral, ver `skill-unit-integration-testing`.
model: opus
---

# Bug Fix

## Objetivo

Oferecer um processo sistemático para correção de bugs: isolamento de escopo, correção mínima, teste de regressão e validação.

## Quando usar

- Ao investigar e isolar um bug reproduzível.
- Ao aplicar correção mínima com teste de regressão.
- Ao validar que o fix não quebrou fluxos.

## Pré-requisitos

Antes de corrigir, verifique se:

- [ ] O problema foi reproduzido localmente.
- [ ] A causa raiz foi identificada (não apenas o sintoma).
- [ ] Há clareza sobre por que o bug acontece.

Se algum item estiver em aberto, considere o agente `debug` primeiro.

## Processo

### 1. Isolar o escopo

```markdown
## Escopo da correção

### Causa raiz
[Uma frase explicando a causa]

### Arquivos a modificar
- `src/path/file.ts` — [o que muda]

### Arquivos que não devem mudar
- [Listar quando houver risco]
```

### 2. Escrever teste primeiro (quando possível)

Um teste que falha antes da correção e passa depois dela é uma prova concreta do problema e da solução.

```ts
describe('bugfix: [descrição]', () => {
  it('should [comportamento esperado]', () => {
    // Arrange
    // Act
    // Assert
  })
})
```

### 3. Aplicar correção mínima

- Corrigir apenas o bug; refatorações adjacentes ficam para commits separados.
- Menor mudança possível que resolve o problema.
- Manter o estilo do código existente.

### 4. Validar

Rode a cadeia de validação do projeto: o teste específico do bug, toda a suíte, typecheck e lint.

### 5. Verificar regressões

- [ ] Funcionalidades relacionadas continuam funcionando.
- [ ] Casos de borda tratados.
- [ ] Diferentes estados (loading, error, empty) continuam corretos.

### 6. Testar manualmente

Mesmo com testes automatizados:

- [ ] Reproduzir o cenário original e confirmar que o bug não ocorre mais.
- [ ] Exercitar o fluxo completo.
- [ ] Verificar via API/CLI (ou UI) que o comportamento faz sentido.

## Checklist de qualidade

### Antes do commit

- [ ] Teste de regressão escrito e passando.
- [ ] Toda a suíte de testes passa.
- [ ] Typecheck passa.
- [ ] Lint passa.
- [ ] Verificação manual realizada.

### Commit message (quando o projeto usa Conventional Commits)

```
fix(escopo): descrição curta do que foi corrigido

Causa: [explicação breve da causa raiz]
Solução: [explicação breve da correção]
```

Exemplo:

```
fix(auth): prevent redirect loop on expired session

Causa: token refresh não atualizava cookie antes do redirect
Solução: aguardar refresh antes de redirecionar
```

## Categorias comuns

### Bugs de lógica

- Condicionais incorretas.
- Edge cases não tratados.
- Ordem de operações errada.

Recomendação: adicionar teste para cada edge case descoberto.

### Bugs de estado

- Race conditions.
- Estado stale.
- Sincronização incorreta.

Recomendação: logs estruturados ajudam a observar transições de estado.

### Bugs de infraestrutura

- Conexão com banco (pool, timeout, deadlock).
- Timeout em serviços externos.
- Falhas de fila.
- Cache (dados stale, conexão, serialização).

Recomendação: verificar logs de conexão, health checks e métricas.

### Bugs de integração

- APIs retornando formato inesperado.
- Timeout ou erro de rede.
- Dados inconsistentes.

Recomendação: adicionar validação e tratamento explícito de erro.

## Anti-padrões

- Corrigir sem teste quando seria viável adicioná-lo.
- Misturar fix e refactor no mesmo commit.
- Corrigir o sintoma sem tratar a causa raiz.
- Copiar solução sem entender.
- Pular a verificação manual quando o fix afeta um fluxo visível.

## Quando não corrigir agora

Às vezes, a melhor ação é adiar:

- Bug de baixo impacto com alto risco de regressão.
- Correção exige refatoração ampla.
- Causa raiz vive em dependência externa.

Nesses casos, documente e abra uma issue para acompanhamento.

---

## 🔹 Go: processo equivalente

O processo (isolar → testar → corrigir mínimo → validar) é universal. Ajustes específicos:

### Teste de regressão em Go

```go
func TestBugfix_RedirectLoopOnExpiredSession(t *testing.T) {
    // Arrange
    sut := setupAuth(t)
    expiredToken := makeExpiredToken()

    // Act
    redirected, err := sut.HandleExpired(context.Background(), expiredToken)

    // Assert
    require.NoError(t, err)
    assert.False(t, redirected, "should not redirect after refresh")
}
```

Para tabelas de regressão, use table-driven com nomes que descrevem o caso (`"expired token without refresh"`, `"expired token with stale cookie"`).

### Validação após o fix

Cadeia em Go:

```bash
go vet ./...
golangci-lint run
go build ./...
go test ./... -run TestBugfix    # roda só o teste de regressão primeiro
go test ./... -race              # roda tudo com race detector
```

### Categorias específicas em Go

#### Bugs de concorrência

- Data race detectado por `go test -race`.
- Goroutine leak: detectar com `runtime.NumGoroutine()` antes/depois ou `pprof`.
- Deadlock total: runtime panica; deadlock parcial fica silencioso (timeout ou pool esgotado).
- Closure capture em loop (Go < 1.22): variável de range capturada por referência.

#### Bugs de nilness

- Receiver nil em método.
- Map não inicializado em escrita: `panic: assignment to entry in nil map`.
- Interface "nil-tipada": `var err error = (*MyErr)(nil); err == nil` é `false`.
- `*T` retornado e usado sem checagem.

#### Bugs de erro silencioso

- `_ = err` ou `err` ignorado: `errcheck` linter pega.
- Erro mascarado por wrapping mal feito.
- `defer rows.Close()` esquecido — connection leak.

#### Bugs de context

- `context.Background()` em código de request — não cancela.
- Timeout não propagado para queries (`db.Query` sem `QueryContext`).

### Commit message (Go segue Conventional Commits)

Igual ao TS:

```
fix(auth): prevent redirect loop on expired session

Causa: token refresh não atualizava cookie antes do redirect
Solução: aguardar refresh antes de redirecionar
```

### Anti-padrões adicionais (Go)

- Adicionar `recover()` para mascarar panic em vez de tratar a causa.
- "Corrigir" data race com `time.Sleep` em vez de sync.
- `goroutine` órfã sem `context` para cancelamento.
