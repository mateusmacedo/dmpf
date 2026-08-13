---
paths:
  - "**/*.ts"
  - "**/*.go"
---

# Organização de arquivos

## Princípio de colocalização

1. Crie funções, tipos e utilitários próximos de quem os usa.
2. Evite colocar em camada compartilhada quando há apenas um consumidor.
3. Promova para camada compartilhada quando dois ou mais módulos de pastas diferentes passarem a depender do código.

## Camadas (exemplo)

| Camada | Responsabilidade |
|--------|------------------|
| `domain/` | Entidades, value objects, contratos, erros de domínio, serviços de domínio |
| `application/` ou `modules/` | Casos de uso, serviços, controllers, DTOs, validação |
| `infra/` | Implementações de repositórios, clientes externos, consumers de fila, config de infra |
| `main/` ou raiz do módulo | Composition root ou módulo raiz do framework |
| `shared/` | Código cross-cutting |

A nomenclatura das camadas pode variar conforme a arquitetura (Clean, hexagonal, modular monolith). Mantenha o padrão vigente no projeto.

## Estrutura típica de módulo

```
module/
├── domain/
│   ├── entities/
│   ├── contracts/
│   └── errors/
├── application/
│   ├── usecases/
│   ├── dtos/
│   └── services/
├── infra/
│   ├── repositories/
│   ├── consumers/
│   └── clients/
└── __tests__/
```

## Recomendações

- Um conceito por arquivo, idealmente.
- Tamanhos de arquivos e funções dentro dos limites adotados pelo projeto (ver `rules/file-size-limits.md`).
- Posição dos testes: colocalizados (`__tests__/` ao lado do código) ou em diretório espelhando `src/`. Siga o padrão do repositório.
- Barrel files (`index.ts` agrupando exports) têm trade-offs de ergonomia vs. bundle/tree-shaking. Se o projeto decidiu evitá-los, mantenha a decisão.
- Evite arquivos `.ts` soltos com lógica sem um módulo que faça sentido.

---

## 🔹 Go: organização de arquivos e pacotes

A unidade de organização em Go é o **pacote** (diretório), não o arquivo. Vários arquivos no mesmo diretório formam um único pacote — eles se enxergam mutuamente sem import.

### Princípio de colocalização

Igual ao TS, mas reforçado:

1. Um pacote é uma unidade coesa; tudo dentro dele compartilha visibilidade `camelCase` (privada).
2. Promover algo a "compartilhado" significa criar um pacote dedicado e exportar o símbolo (`PascalCase`).
3. Pacote `internal/` é uma garantia da toolchain: só é importável por código no mesmo módulo. Use para isolar implementação interna.

### Camadas (exemplo de Clean Architecture em Go)

| Camada | Pasta típica |
|--------|--------------|
| Domain | `internal/domain/` ou `internal/<contexto>/domain/` |
| Application / Use cases | `internal/application/` ou `internal/<contexto>/usecase/` |
| Infra (repositórios, clients, queues) | `internal/infra/` ou `internal/<contexto>/infra/` |
| Composition root | `cmd/<binário>/main.go` |
| Compartilhado interno | `internal/shared/` ou `pkg/` (público) |

### Estrutura típica de módulo Go

```
projeto/
├── cmd/
│   └── api/
│       └── main.go         # composition root
├── internal/
│   ├── user/
│   │   ├── domain.go       # entities + interfaces
│   │   ├── usecase.go      # casos de uso
│   │   ├── repository.go   # implementação concreta
│   │   ├── http.go         # handler HTTP
│   │   └── usecase_test.go # testes colocalizados
│   └── shared/
│       ├── httputil/
│       └── dbutil/
├── pkg/                    # APIs públicas (raro em apps internos)
├── go.mod
└── go.sum
```

### Recomendações

- **Um conceito por arquivo é ideal**, mas *vários arquivos por pacote é o normal* — não fragmente em sub-pastas só pra "organizar arquivos".
- Testes colocalizados são a convenção: `foo.go` + `foo_test.go` no mesmo diretório. `_test.go` é tratado especial pela toolchain.
- Não há barrel files; o pacote inteiro é o "barrel".
- Evite pacotes `util` ou `common` genéricos — Go favorece nomes orientados a domínio.
- O nome do pacote (declarado em `package xxx`) costuma bater com o nome da pasta, mas não é obrigatório.
- Imports cíclicos são erro de compilação, não warning. Estrutura com bom desenho de dependências previne ciclos.
