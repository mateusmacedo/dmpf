## 1.0.0-rc.1 (2026-10-06)

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.1
- Updated application to 1.0.0-rc.1
- Updated contracts to 1.0.0-rc.1
- Updated postgres to 1.0.0-rc.1
- Updated domain to 1.0.0-rc.1
- Updated memory to 1.0.0-rc.1
- Updated ports to 1.0.0-rc.1
- Updated conformance to 1.0.0-rc.1

## 1.0.0-rc.0 (2026-10-06)

### 🚀 Features

- **testkit:** Adicionar GoldenSuite e ampliar serviceskit e providerkit ([edf7414](https://github.com/mateusmacedo/dmpf/commit/edf7414))
- **testkit:** Adicionar asserções de domínio, leitores e Reexec ao kit ([1ef210d](https://github.com/mateusmacedo/dmpf/commit/1ef210d))
- **testkit:** Cobrir a inbox de comandos na suíte de conformidade ([ca1b8e2](https://github.com/mateusmacedo/dmpf/commit/ca1b8e2))

### 🩹 Fixes

- **testkit:** Excluir do subject reference o filho do harness do bff ([9720cba](https://github.com/mateusmacedo/dmpf/commit/9720cba))

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.0
- Updated application to 1.0.0-rc.0
- Updated contracts to 1.0.0-rc.0
- Updated postgres to 1.0.0-rc.0
- Updated domain to 1.0.0-rc.0
- Updated memory to 1.0.0-rc.0
- Updated ports to 1.0.0-rc.0
- Updated conformance to 1.0.0-rc.0

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 0.1.0 (2026-09-30)

### 🚀 Features

- ⚠️  **testkit:** Dar a cada projeto um banco de teste com schema limpo ([97abf9b](https://github.com/mateusmacedo/dmpf/commit/97abf9b))
- **testkit:** add the Repository conformance suite ([2b4fa1f](https://github.com/mateusmacedo/dmpf/commit/2b4fa1f))
- **application:** make the execution context the subject of authorization ([699c662](https://github.com/mateusmacedo/dmpf/commit/699c662))
- **testkit:** Aceitar as tabelas próprias do contexto no reset do pool ([24471b7](https://github.com/mateusmacedo/dmpf/commit/24471b7))

### 🩹 Fixes

- **testkit:** Dar um só banco às chamadas simultâneas de um teste ([68a4ebd](https://github.com/mateusmacedo/dmpf/commit/68a4ebd))
- **testkit:** Cobrir no golden da evidência todo contrato com golden ([458a195](https://github.com/mateusmacedo/dmpf/commit/458a195))
- **testkit:** Tirar os contratos do subject reference da evidência ([5e2f086](https://github.com/mateusmacedo/dmpf/commit/5e2f086))

### 🔥 Performance

- **testkit:** Limpar as tabelas do teste só quando ele reabre o banco ([128100b](https://github.com/mateusmacedo/dmpf/commit/128100b))

### ⚠️  Breaking Changes

- **testkit:** Dar a cada projeto um banco de teste com schema limpo  ([97abf9b](https://github.com/mateusmacedo/dmpf/commit/97abf9b))
  pg.OpenPool passa a exigir pg.Options.

### 🧱 Updated Dependencies

- Updated observability to 0.1.0
- Updated application to 0.1.0
- Updated contracts to 0.1.0
- Updated postgres to 0.1.0
- Updated domain to 0.1.0
- Updated memory to 0.1.0
- Updated ports to 0.1.0
- Updated conformance to 0.1.0

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo