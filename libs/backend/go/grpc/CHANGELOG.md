## 1.0.0-rc.2 (2026-10-09)

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.2
- Updated transport to 1.0.0-rc.2
- Updated ports to 1.0.0-rc.2

## 1.0.0-rc.1 (2026-10-06)

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.1
- Updated transport to 1.0.0-rc.1
- Updated ports to 1.0.0-rc.1

## 1.0.0-rc.0 (2026-10-06)

### 🚀 Features

- **grpc:** Adicionar Unary, Method, Uncovered e APIEnv ao kernel ([0eebaed](https://github.com/mateusmacedo/dmpf/commit/0eebaed))
- **grpc:** Adicionar StatusOf e o protocolo de hop ao kernel ([f6bfab4](https://github.com/mateusmacedo/dmpf/commit/f6bfab4))
- **grpc:** Registrar cada chamada no stats.End e recuperar panic ([8d4f0d8](https://github.com/mateusmacedo/dmpf/commit/8d4f0d8))
- **grpc:** Exigir a chave de idempotência nos métodos de comando ([894d3fc](https://github.com/mateusmacedo/dmpf/commit/894d3fc))

### 🧱 Updated Dependencies

- Updated observability to 1.0.0-rc.0
- Updated transport to 1.0.0-rc.0
- Updated ports to 1.0.0-rc.0

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 0.1.0 (2026-09-30)

### 🚀 Features

- **grpc:** Registrar cada chamada servida com o código de status ([e85a9ae](https://github.com/mateusmacedo/dmpf/commit/e85a9ae))
- **grpc:** Promover a cadeia de interceptors de servidor ao kernel ([39794b5](https://github.com/mateusmacedo/dmpf/commit/39794b5))
- **grpc:** verify the calling workload by mutual TLS and a SAN allowlist ([73523b0](https://github.com/mateusmacedo/dmpf/commit/73523b0))
- **grpc:** Promover a composição do servidor e o gesto de servir ([b13fb90](https://github.com/mateusmacedo/dmpf/commit/b13fb90))

### 🩹 Fixes

- **grpc:** Levar a correlação e o tenant ao log da chamada ([b1ce497](https://github.com/mateusmacedo/dmpf/commit/b1ce497))

### 🔥 Performance

- **grpc:** Não montar o log da chamada com o nível desligado ([26540cd](https://github.com/mateusmacedo/dmpf/commit/26540cd))

### 🧱 Updated Dependencies

- Updated observability to 0.1.0
- Updated transport to 0.1.0
- Updated ports to 0.1.0

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo