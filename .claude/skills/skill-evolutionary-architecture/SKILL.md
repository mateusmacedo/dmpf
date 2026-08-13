---
name: skill-evolutionary-architecture
description: >
  Use esta skill quando o usuário pedir para "refatoração incremental", "fitness functions",
  "migração de código", "trade-offs arquiteturais", "evolução do projeto",
  ou mencionar migração local para compartilhada, Last Responsible Moment, YAGNI ou decisões
  sobre quando mover código entre camadas.
  Cobre refatoração incremental, fitness functions (lint/typecheck/test), migração local→compartilhada,
  trade-offs, YAGNI e Last Responsible Moment.
  Para estrutura de pastas, ver `skill-architecture-patterns`; para refatoração de código limpo, ver `skill-clean-code`.
model: sonnet
---

# Arquitetura evolutiva

## Quando usar
- Ao planejar mudanças incrementais de arquitetura.
- Ao migrar código entre camadas (local → compartilhado, módulo → serviço).
- Ao avaliar trade-offs antes de decidir.

## Princípios

### Last Responsible Moment

Adie decisões até ter informação suficiente.

```
Dia 1: "Vou criar um helper local dentro do módulo"
Dia N: "3 módulos usam o mesmo helper → mover para camada compartilhada"
```

Na prática: código pode começar local (dentro do módulo/feature) e migrar para camada compartilhada (por exemplo `application/` ou `infra/`) quando 2+ consumidores de pastas diferentes passarem a usar.

### YAGNI

```typescript
// Over-engineering: abstração para um único uso
const createPriceCalculatorFactory = (config: PriceConfig) =>
  new PriceCalculatorBuilder().withAddons(config.addons).build()

// Simples: função pura
export const calculateTotalPlanPriceWithAddons = ({
  planType,
  basePlanPrice,
  additionalCredits,
}: Props): number => {
  if (planType === 'ia') return basePlanPrice + additionalCredits * PRICE_PER_CREDIT
  return basePlanPrice
}
```

### Fitness functions

Métricas automatizadas que validam a arquitetura continuamente:

```bash
lint do projeto       # imports, formatação, regras
typecheck do projeto  # TypeScript em modo strict
test do projeto       # regras de negócio, serviços, use cases
```

Git hooks podem rodar essas validações automaticamente (pre-commit, pre-push).

## Refatoração incremental

### Local → compartilhado

Quando um service/helper local passa a ser usado por mais de um módulo:

```
Passo 1: Código nasce local
  domain/petition/services/searchPetition.ts

Passo 2: Outro módulo precisa do mesmo serviço
  → Mover para camada compartilhada (ex.: application/services/petition/)
  → Atualizar imports
  → Validar com lint, typecheck e testes

Passo 3: Verificar que nenhum import quebrou
```

### Extração de módulo

Quando um arquivo cresce demais para a complexidade que representa:

```
Passo 1: Identificar trecho coeso para extrair
Passo 2: Criar módulo separado com index.ts e types.ts
Passo 3: Mover testes junto
Passo 4: Validar que o arquivo original ficou menor e mais coeso
```

### Extração de lógica para `domain/`

Quando lógica de negócio está misturada em infra/controller:

```
Passo 1: Identificar função pura (entrada → saída, sem side effects)
Passo 2: Criar em domain/contexto/nomeFuncao/
  ├── index.ts
  ├── types.ts
  └── __tests__/test.ts
Passo 3: Substituir lógica inline pelo import de domain/
Passo 4: Validar com testes
```

## Decisões arquiteturais

### Processo

1. Qual o problema? — descrever claramente.
2. Quais as opções? — listar 2 ou 3 abordagens.
3. Quais os trade-offs? — prós e contras de cada.
4. Qual a recomendação? — escolha justificada.
5. Como reverter? — plano caso não funcione.

### Exemplo: onde colocar um novo serviço

| Opção | Prós | Contras |
|-------|------|---------|
| Local (`domain/contexto/`) | Coeso, fácil de encontrar | Não reutilizável |
| Aplicação (`application/`) | Reutilizável entre módulos | Pode ser prematuro |
| Infra (`infra/`) | Compartilhado, próximo da implementação | Acoplado à infra |

Regra prática: comece local; migre quando precisar.

## Evolução segura

### Strangler Fig (refatorações grandes)

```
Fase 1: criar nova implementação ao lado da antiga
Fase 2: migrar consumidores gradualmente
Fase 3: remover implementação antiga quando ninguém mais usa
```

Cada fase deve manter lint, typecheck e testes passando.

### Validação após cada passo

```bash
# Após cada mudança incremental:
lint do projeto       # imports organizados, sem violações
typecheck do projeto  # tipos corretos em toda a cadeia
test do projeto       # comportamento preservado
```

Se algo falhar, corrigir antes de prosseguir.

## Checklist

- [ ] Decisão adiada até ser necessária (Last Responsible Moment).
- [ ] YAGNI aplicado (sem abstrações para um uso único).
- [ ] Fitness functions passando (lint, typecheck, test).
- [ ] Refatoração em passos incrementais.
- [ ] Cada passo validado antes do próximo.
- [ ] Imports atualizados ao mover arquivos.

Para definição de arquitetura inicial (camadas, dependências, estrutura de pastas), ver `skill-architecture-patterns`.

---

## 🔹 Go: arquitetura evolutiva em Go

Os princípios (Last Responsible Moment, YAGNI, fitness functions, Strangler Fig) são universais. Em Go, o pacote é a unidade primária de evolução; mover código entre pacotes é a operação típica.

### Last Responsible Moment

```
Dia 1: helper local em internal/petition/util.go
Dia N: 3 pacotes precisam do mesmo helper
       → mover para internal/shared/helpers/ ou pacote próprio
       → atualizar imports
       → go vet ./... + go test ./... ainda passam
```

A toolchain ajuda: imports são caminhos absolutos no `go.mod`, então `gopls` (LSP) detecta quebras imediatamente após o move.

### YAGNI em Go

Go é particularmente sensível a abstrações prematuras. O ditado "Accept interfaces, return structs" reforça YAGNI:

```go
// Over-engineering: interface antes de ter 2 implementações
type PriceCalculator interface {
    Calculate(input Input) (int64, error)
}

// Idiomático: função pura ou struct concreta
func CalculateTotalWithAddons(plan PlanType, base int64, addons int) int64 {
    if plan == PlanIA {
        return base + int64(addons)*pricePerCredit
    }
    return base
}
```

A interface entra em cena quando há um segundo implementador (test fake, alternativa real).

### Fitness functions

Em Go, a cadeia de validação automática é robusta:

```bash
gofmt -l .                    # diff vazio = ok
go vet ./...                  # análise estática built-in
golangci-lint run             # agregador (errcheck, staticcheck, gocyclo, ...)
go build ./...                # typecheck implícito
go test ./... -race           # comportamento + race detector
govulncheck ./...             # CVEs alcançadas
```

CI deve falhar em qualquer diff de `gofmt` ou warning de `vet`. Hooks via `lefthook` ou `pre-commit` são equivalentes a Husky no JS.

### Refatoração incremental: local → compartilhado

#### Antes — helper colocalizado

```
internal/petition/
├── petition.go
├── service.go
└── slugify.go         # helper específico
```

#### Depois — helper compartilhado

```
internal/petition/
├── petition.go
└── service.go

internal/shared/text/
└── slugify.go         # consumidores: petition + document
```

Passos:

1. Mover `slugify.go` para `internal/shared/text/`.
2. Atualizar declaração `package text`.
3. `gopls` (ou IDE) atualiza imports automaticamente — verificar com `go build ./...`.
4. Rodar `go test ./...` para confirmar comportamento.

### Extração de pacote

Quando um pacote acumula responsabilidades não relacionadas, extrair em subpacotes:

```
Antes
internal/order/
├── order.go        (entidade + 600 linhas)

Depois
internal/order/
├── order.go        (entidade)
├── pricing/
│   └── pricing.go  (cálculos de preço)
└── shipping/
    └── shipping.go (cálculos de frete)
```

Cuidados:
- Evitar import cíclico — se `order` importa `pricing` e vice-versa, há acoplamento mal modelado.
- Mover testes (`*_test.go`) junto com o código.

### Strangler Fig em Go

Para reescrever um pacote sem big-bang:

```
Fase 1
internal/legacy/billing/   ← implementação atual
internal/billing/          ← nova implementação (vazia ou parcial)

Fase 2
Use case decide qual usar via flag/config; novos endpoints batem na nova.

Fase 3
Endpoints antigos migrados; deletar internal/legacy/billing.
```

Cada fase mantém `go test -race` verde.

### Decisões: onde colocar um novo pacote

| Opção | Prós | Contras |
|-------|------|---------|
| `internal/<contexto>/` | Encapsulado por bounded context | Não compartilhável entre contextos |
| `internal/shared/` | Reutilizável internamente | Pode virar dumping ground se não disciplinado |
| `pkg/` | Exportável fora do módulo | Compromisso público — API estável |
| Módulo separado (`go.mod`) | Versionável independente | Overhead de versionamento |

Regra prática: comece em `internal/<contexto>/`; só promova para `internal/shared/` quando 2+ pacotes consumirem; `pkg/` apenas se for API pública estável.

### Validação após cada passo (Go)

```bash
gofmt -l .                    # sem diff
go vet ./...                  # sem warnings
go build ./...                # compila
go test ./... -race           # passa com race detector
golangci-lint run             # agregado ok
```

`go mod tidy` ao final para limpar imports não usados.

### Anti-patterns evolutivos (Go)

- Criar interface antes de ter 2 implementações (over-engineering).
- Mover para `pkg/` antes de existir consumidor externo (compromete API pública).
- Quebrar pacote em subpacotes pequenos demais (sobrecarrega imports).
- Refactor "big bang" sem fitness functions — em Go, race detector e `vet` precisam acompanhar cada passo.

### Checklist (Go)

- [ ] Decisão adiada (Last Responsible Moment).
- [ ] Sem interfaces especulativas.
- [ ] `gofmt`, `go vet`, `golangci-lint`, `go test -race` passam após cada passo.
- [ ] Imports atualizados em todos os consumidores após mover pacote.
- [ ] `go mod tidy` ao final.
- [ ] Strangler Fig usado para reescritas grandes.
