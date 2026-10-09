## 1.0.0-rc.2 (2026-10-09)

### 🚀 Features

- **conformance:** Ler o kernel por versão e parametrizar o infrasync ([9d588b8](https://github.com/mateusmacedo/dmpf/commit/9d588b8))

### 🩹 Fixes

- **conformance:** Validar o nome de app no infrasync ([1dd6298](https://github.com/mateusmacedo/dmpf/commit/1dd6298))
- **conformance:** Aceitar manifesto de kernel só sob o prefixo do DMPF ([1cc6e2e](https://github.com/mateusmacedo/dmpf/commit/1cc6e2e))
- **conformance:** Ler o kernel com -mod=readonly mesmo com vendor/ ([12ebab5](https://github.com/mateusmacedo/dmpf/commit/12ebab5))
- **conformance:** Recalcular a build list após os --require do modsync ([604132b](https://github.com/mateusmacedo/dmpf/commit/604132b))

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 1.0.0-rc.1 (2026-10-06)

Initial release

## 1.0.0-rc.0 (2026-10-06)

### 🚀 Features

- ⚠️  **conformance:** Revogar o DMPF-T002 e a aprovação dupla (ADR-058) ([349c1e5](https://github.com/mateusmacedo/dmpf/commit/349c1e5))
- **conformance:** Apontar o semconv do BOM e publicar o swagger local ([30ffc98](https://github.com/mateusmacedo/dmpf/commit/30ffc98))

### ⚠️  Breaking Changes

- **conformance:** Revogar o DMPF-T002 e a aprovação dupla (ADR-058)  ([349c1e5](https://github.com/mateusmacedo/dmpf/commit/349c1e5))

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 0.1.0 (2026-09-30)

### 🚀 Features

- **conformance:** Cobrir o contrato de contexto na regra B012 ([83db3c5](https://github.com/mateusmacedo/dmpf/commit/83db3c5))
- **conformance:** Gerar a infra agregada a partir do deploy/infra.json de cada app ([22d42a1](https://github.com/mateusmacedo/dmpf/commit/22d42a1))
- **conformance:** Exigir tag Go ancestral no BOM com o DMPF-B012 ([6feb566](https://github.com/mateusmacedo/dmpf/commit/6feb566))

### 🩹 Fixes

- **conformance:** Recusar no infrasync o valor que escaparia do gerado ([39468e4](https://github.com/mateusmacedo/dmpf/commit/39468e4))
- **conformance:** Reprovar na B012 o contrato sem entrada no BOM ([d26c7f2](https://github.com/mateusmacedo/dmpf/commit/d26c7f2))
- **conformance:** Validar os manifestos e a senha de dev no infrasync ([4281578](https://github.com/mateusmacedo/dmpf/commit/4281578))
- **infra:** Provisionar bancos e broker no infra-up com o profile provisioning ([58fcaee](https://github.com/mateusmacedo/dmpf/commit/58fcaee))

### ❤️ Thank You

- Mateus Macedo Dos Anjos