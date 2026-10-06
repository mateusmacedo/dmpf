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