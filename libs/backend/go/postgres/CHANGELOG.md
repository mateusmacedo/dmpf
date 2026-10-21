## 1.0.0-rc.6 (2026-10-20)

### 🚀 Features

- **postgres:** Ler entre tenants e projetar no destino declarado ([c6c5d862](https://github.com/mateusmacedo/dmpf/commit/c6c5d862))
- **postgres:** Ler a pré-condição relacional na UoW sob guarda ([2d500a29](https://github.com/mateusmacedo/dmpf/commit/2d500a29))
- **postgres:** Traduzir a violação de unicidade declarada no Save ([26d115fe](https://github.com/mateusmacedo/dmpf/commit/26d115fe))

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.6
- Updated contracts to 1.0.0-rc.6
- Updated testkit to 1.0.0-rc.6
- Updated domain to 1.0.0-rc.6
- Updated ports to 1.0.0-rc.6

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 1.0.0-rc.5 (2026-10-17)

### 🚀 Features

- **postgres:** Conceder e conferir os grants da role de runtime ([168ec9e5](https://github.com/mateusmacedo/dmpf/commit/168ec9e5))

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.5
- Updated contracts to 1.0.0-rc.5
- Updated testkit to 1.0.0-rc.5
- Updated domain to 1.0.0-rc.5
- Updated ports to 1.0.0-rc.5

### ❤️ Thank You

- Mateus Macedo Dos Anjos

## 1.0.0-rc.4 (2026-10-12)

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.4
- Updated contracts to 1.0.0-rc.4
- Updated testkit to 1.0.0-rc.4
- Updated domain to 1.0.0-rc.4
- Updated ports to 1.0.0-rc.4

## 1.0.0-rc.3 (2026-10-10)

### 🩹 Fixes

- **postgres:** Subir o Go para 1.26.9 ([11b7413f](https://github.com/mateusmacedo/dmpf/commit/11b7413f))

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.3
- Updated contracts to 1.0.0-rc.3
- Updated testkit to 1.0.0-rc.3
- Updated domain to 1.0.0-rc.3
- Updated ports to 1.0.0-rc.3

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 1.0.0-rc.2 (2026-10-09)

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.2
- Updated contracts to 1.0.0-rc.2
- Updated testkit to 1.0.0-rc.2
- Updated domain to 1.0.0-rc.2
- Updated ports to 1.0.0-rc.2

## 1.0.0-rc.1 (2026-10-06)

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.1
- Updated contracts to 1.0.0-rc.1
- Updated testkit to 1.0.0-rc.1
- Updated domain to 1.0.0-rc.1
- Updated ports to 1.0.0-rc.1

## 1.0.0-rc.0 (2026-10-06)

### 🚀 Features

- **postgres:** Adicionar SnapshotTable para agregados em snapshot ([9e88084](https://github.com/mateusmacedo/dmpf/commit/9e88084))
- **postgres:** Instrumentar consultas e pool pela semconv de banco ([6ce4e87](https://github.com/mateusmacedo/dmpf/commit/6ce4e87))
- **postgres:** Registrar comandos na inbox com purga e quarentena única ([5311cde](https://github.com/mateusmacedo/dmpf/commit/5311cde))

### 🩹 Fixes

- **postgres:** Devolver o erro do contexto quando ele vence na consulta ([d2356c3](https://github.com/mateusmacedo/dmpf/commit/d2356c3))

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.0
- Updated contracts to 1.0.0-rc.0
- Updated testkit to 1.0.0-rc.0
- Updated domain to 1.0.0-rc.0
- Updated ports to 1.0.0-rc.0

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 0.1.0 (2026-09-30)

### 🚀 Features

- **postgres:** Esperar as tabelas que outro papel migra ([47fb4b5](https://github.com/mateusmacedo/dmpf/commit/47fb4b5))
- ⚠️  **postgres:** Migrar o schema do kernel por capacidade ([36dd1af](https://github.com/mateusmacedo/dmpf/commit/36dd1af))
- ⚠️  **postgres:** Renomear as tabelas do kernel sem o prefixo dmpf ([3ad29da](https://github.com/mateusmacedo/dmpf/commit/3ad29da))
- **postgres:** report a relation value held by another tenant as cross-tenant access ([04b2942](https://github.com/mateusmacedo/dmpf/commit/04b2942))
- **postgres:** close the driver behind ReadPool and probe cross-tenant misses ([1aeaf51](https://github.com/mateusmacedo/dmpf/commit/1aeaf51))
- **postgres:** carry the tenant on the outbox metadata ([a669376](https://github.com/mateusmacedo/dmpf/commit/a669376))
- **postgres:** scope every aggregate read and write by tenant ([eb27214](https://github.com/mateusmacedo/dmpf/commit/eb27214))
- **postgres:** Promover o pool, o tracer de query e a guarda da outbox ([072aac9](https://github.com/mateusmacedo/dmpf/commit/072aac9))

### 🩹 Fixes

- **postgres:** Nomear no log a tabela que o WaitForTables espera ([9dce4c8](https://github.com/mateusmacedo/dmpf/commit/9dce4c8))

### ⚠️  Breaking Changes

- **postgres:** Migrar o schema do kernel por capacidade  ([36dd1af](https://github.com/mateusmacedo/dmpf/commit/36dd1af))
  Migrate passa a exigir as capacidades e deixa de criar
  dmpf_example_orders e dmpf_example_reservations.
- **postgres:** Renomear as tabelas do kernel sem o prefixo dmpf  ([3ad29da](https://github.com/mateusmacedo/dmpf/commit/3ad29da))
  dmpf_outbox, dmpf_inbox e dmpf_quarantine passam a outbox,
  inbox e quarantine, sem migração; os bancos existentes precisam ser recriados.

### 🧱 Updated Dependencies

- Updated observability to 0.1.0
- Updated contracts to 0.1.0
- Updated testkit to 0.1.0
- Updated domain to 0.1.0
- Updated ports to 0.1.0

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo