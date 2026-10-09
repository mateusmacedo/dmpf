## 1.0.0-rc.2 (2026-10-09)

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.2
- Updated application to 1.0.0-rc.2
- Updated contracts to 1.0.0-rc.2
- Updated postgres to 1.0.0-rc.2
- Updated testkit to 1.0.0-rc.2
- Updated ports to 1.0.0-rc.2
- Updated grpc to 1.0.0-rc.2

## 1.0.0-rc.1 (2026-10-06)

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.1
- Updated application to 1.0.0-rc.1
- Updated contracts to 1.0.0-rc.1
- Updated postgres to 1.0.0-rc.1
- Updated testkit to 1.0.0-rc.1
- Updated ports to 1.0.0-rc.1
- Updated grpc to 1.0.0-rc.1

## 1.0.0-rc.0 (2026-10-06)

### 🚀 Features

- **app:** Adicionar Policies e a validação dela ao kernel ([fcbe9ac](https://github.com/mateusmacedo/dmpf/commit/fcbe9ac))
- **app:** Adicionar contrato de categorias, purga e relay.Instrument ([cd291c7](https://github.com/mateusmacedo/dmpf/commit/cd291c7))
- **app:** Instrumentar relay, consumo e purga com contenção de panic ([bb18917](https://github.com/mateusmacedo/dmpf/commit/bb18917))
- **app:** Montar a política de idempotência e o laço de purga ([813ee62](https://github.com/mateusmacedo/dmpf/commit/813ee62))

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.0
- Updated application to 1.0.0-rc.0
- Updated contracts to 1.0.0-rc.0
- Updated postgres to 1.0.0-rc.0
- Updated testkit to 1.0.0-rc.0
- Updated ports to 1.0.0-rc.0
- Updated grpc to 1.0.0-rc.0

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 0.1.0 (2026-09-30)

### 🚀 Features

- **app:** declare the consumer boundary and rebuild the context in the adapter ([dfb4cdc](https://github.com/mateusmacedo/dmpf/commit/dfb4cdc))
- **app:** put the tenant on the envelope the relay assembles ([635dd9f](https://github.com/mateusmacedo/dmpf/commit/635dd9f))
- **app:** give each consumption its own identity and deadline ([4ca2b54](https://github.com/mateusmacedo/dmpf/commit/4ca2b54))
- **app:** Montar o relay sobre a outbox de Postgres no kernel ([c2daded](https://github.com/mateusmacedo/dmpf/commit/c2daded))

### 🩹 Fixes

- **app:** contain a message whose execution context cannot be rebuilt ([f8da094](https://github.com/mateusmacedo/dmpf/commit/f8da094))

### 🧱 Updated Dependencies

- Updated observability to 0.1.0
- Updated application to 0.1.0
- Updated contracts to 0.1.0
- Updated postgres to 0.1.0
- Updated testkit to 0.1.0
- Updated ports to 0.1.0

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo