## 1.0.0-rc.3 (2026-10-10)

### 🩹 Fixes

- **dmpf-plugin:** Fixar o Go 1.26.9 no versions.json ([84475f03](https://github.com/mateusmacedo/dmpf/commit/84475f03))

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 1.0.0-rc.2 (2026-10-09)

### 🚀 Features

- **dmpf-plugin:** Atualizar o workspace consumidor por nx migrate ([fbbad59](https://github.com/mateusmacedo/dmpf/commit/fbbad59))
- **dmpf-plugin:** Registrar em dmpf.rendered.json o que o init gera ([d42ec93](https://github.com/mateusmacedo/dmpf/commit/d42ec93))
- **dmpf-plugin:** Escrever o chamador do CI no init em modo version ([1e62782](https://github.com/mateusmacedo/dmpf/commit/1e62782))
- **dmpf-plugin:** Distribuir os ativos de IA pelo init ([24fda53](https://github.com/mateusmacedo/dmpf/commit/24fda53))
- **dmpf-plugin:** Adicionar o init e desacoplar o bounded-context do platform ([748e6df](https://github.com/mateusmacedo/dmpf/commit/748e6df))
- **dmpf-plugin:** Empacotar os scripts como executors do plugin ([1f96bdb](https://github.com/mateusmacedo/dmpf/commit/1f96bdb))
- **dmpf-plugin:** Tornar o plugin publicável e fixar versões no versions.json ([5e7275a](https://github.com/mateusmacedo/dmpf/commit/5e7275a))

### 🩹 Fixes

- **dmpf-plugin:** Corrigir os achados do SonarCloud no plugin ([2c809cc](https://github.com/mateusmacedo/dmpf/commit/2c809cc))
- **dmpf-plugin:** Endurecer scripts, migrations e init após o checklist ([c0c6135](https://github.com/mateusmacedo/dmpf/commit/c0c6135))
- **dmpf-plugin:** Corrigir os achados do code review ([ca643bf](https://github.com/mateusmacedo/dmpf/commit/ca643bf))
- **dmpf-plugin:** Conceder packages: read no chamador de CI do init ([33d5b0a](https://github.com/mateusmacedo/dmpf/commit/33d5b0a))
- **dmpf-plugin:** Abrir o bloco use no go.work que o init escreve ([e220135](https://github.com/mateusmacedo/dmpf/commit/e220135))
- **dmpf-plugin:** Ler o DDL do kernel pelo go mod download ([5b4725e](https://github.com/mateusmacedo/dmpf/commit/5b4725e))

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 1.0.0-rc.1 (2026-10-06)

Initial release

## 1.0.0-rc.0 (2026-10-06)

### 🚀 Features

- **dmpf-plugin:** Rodar o e2e do contexto sem repetir o test-race ([1e3e935](https://github.com/mateusmacedo/dmpf/commit/1e3e935))
- **dmpf-plugin:** Gerar o e2e do contexto sobre o harness distribuído ([d6e0c76](https://github.com/mateusmacedo/dmpf/commit/d6e0c76))
- **dmpf-plugin:** Instruir o baseline sem AUT-01 nem commit próprio ([9fe6e9d](https://github.com/mateusmacedo/dmpf/commit/9fe6e9d))
- **dmpf-plugin:** Gerar contextos com telemetria canônica e TLS em hmg ([604c8e8](https://github.com/mateusmacedo/dmpf/commit/604c8e8))
- **dmpf-plugin:** Gerar contextos com comandos idempotentes ([185a46e](https://github.com/mateusmacedo/dmpf/commit/185a46e))

### ❤️ Thank You

- Mateus Macedo Dos Anjos @mateusmacedo

## 0.1.0 (2026-09-30)

### 🚀 Features

- **dmpf-plugin:** Gerar o docker:run-relay nos contextos com app ([cd47151](https://github.com/mateusmacedo/dmpf/commit/cd47151))
- **dmpf-plugin:** Neutralizar o publish Docker inferido no app gerado ([8c24ade](https://github.com/mateusmacedo/dmpf/commit/8c24ade))
- **dmpf-plugin:** Subir a infra de testes antes da integração gerada ([2dd1a3b](https://github.com/mateusmacedo/dmpf/commit/2dd1a3b))
- **dmpf-plugin:** Gerar o contrato e o deploy do contexto com o app ([c31d2e5](https://github.com/mateusmacedo/dmpf/commit/c31d2e5))
- **dmpf-plugin:** Gerar a forma canônica do app, do rpc e do schema ([8b7faf8](https://github.com/mateusmacedo/dmpf/commit/8b7faf8))
- **dmpf-plugin:** Emitir a composition root e os kits no esqueleto ([91a3bd2](https://github.com/mateusmacedo/dmpf/commit/91a3bd2))
- **dmpf-plugin:** Rodar o dmpf-modsync no callback pós-flush do generator ([97a8ad2](https://github.com/mateusmacedo/dmpf/commit/97a8ad2))
- **workspace:** [ARQ-554] Harness de bounded context e golden bookings ([29b6e4b](https://github.com/mateusmacedo/dmpf/commit/29b6e4b))
- **dmpf-plugin:** [ARQ-546] Criar o generator de esqueleto ([e80d7d1](https://github.com/mateusmacedo/dmpf/commit/e80d7d1))

### 🩹 Fixes

- **dmpf-plugin:** Gerar o gRPC local escutando só no loopback ([100b17e](https://github.com/mateusmacedo/dmpf/commit/100b17e))
- **dmpf-plugin:** Versionar o .env.example gerado no deploy do contexto ([455c6f8](https://github.com/mateusmacedo/dmpf/commit/455c6f8))
- **dmpf-plugin:** Declarar o protobuf como external do contrato gerado ([db35aef](https://github.com/mateusmacedo/dmpf/commit/db35aef))
- **dmpf-plugin:** Aceitar só apps/backend e a próxima porta gRPC livre ([5e23ad9](https://github.com/mateusmacedo/dmpf/commit/5e23ad9))
- **dmpf-plugin:** Gerar o relay que encerra na falha, sem laço ([3a1198b](https://github.com/mateusmacedo/dmpf/commit/3a1198b))
- **dmpf-plugin:** Esperar as tabelas da outbox no relay gerado ([59e7e72](https://github.com/mateusmacedo/dmpf/commit/59e7e72))
- **dmpf-plugin:** Incluir os buf*.yaml nos inputs do buf-pins gerado ([acd79dd](https://github.com/mateusmacedo/dmpf/commit/acd79dd))

### ❤️ Thank You

- Mateus Macedo Dos Anjos