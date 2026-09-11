# Armadilhas verificadas

Cada entrada foi observada de fato — na prova do generator (ARQ-546), no shared
kernel (ARQ-553) ou na construção do golden `bookings` (ARQ-554). Quando o
golden for regenerado e algo novo morder, a entrada nova vem para cá.

1. **`time` no bloco `domain` reprova.** O verificador classifica o package
   `time` inteiro como `io.clock` (`dmpf-conformance/internal/rule/stdlib.go`,
   `capability.go`), e o `depguard` do bloco nega o import. O instante chega
   por parâmetro, como inteiro de nanossegundos. `errors.New`, `fmt.Errorf`,
   `panic` e `fmt.Print*` são os outros red controls do `forbidigo`, restritos
   a `-domain/`.
2. **`golangci-lint` ignora arquivo com cabeçalho `DO NOT EDIT` por default**
   (`exclusions.generated: strict`). Quem quer provar o lint sobre código
   gerado precisa desligar esse filtro — a prova do generator faz isso.
3. **`go.mod` gerado não tem `require`.** O módulo resolve as dependências
   pelo `go.work`. Um módulo cujo `use` ainda não está no `go.work` não
   compila pelos targets Nx — o commit do `go.work` fecha a cadeia.
4. **`exceptions` e `public_integration_surface` do manifesto são
   preservados no merge** — editados à mão por convenção; o generator nunca
   os regrava.
5. **`.proto` publicado nunca é regravado.** Drift entre a definição e o
   `.proto` existente é recusa nomeando o evento; evolução é rito próprio.
6. **Sem a unidade `<ctx>/contract` no manifesto do `dmpf-contracts`, o
   `gen/go` novo cai em `DMPF-U001`** e o `--write-baseline` aborta. A
   unidade entra antes do `generate`.
7. **`pnpm install` é obrigatório depois de gerar.** O módulo novo tem
   `package.json` (`private: true`) e vira importer do pnpm; sem o install os
   targets falham de forma obscura.
8. **A composition root fica fora do generator.** Cabear processo é copiar
   `apps/backend/dmpf-reference/cmd/dmpf-reference`; a skill aponta o passo,
   não o executa.
9. **Sem pluralização automática.** O identificador vai literal para tabela,
   rota e package: `<ctx>_<agregado>`, `/<ctx>/<agregado>`.
10. **Renomear contexto ou agregado é rito próprio** (ADR-017): `bounded_context`
    é identidade estável; não se renomeia por refactor.
11. **A prova em worktree precisa preservar o `node_modules` da raiz**:
    `pnpm_config_verify_deps_before_run=false`, sem definir `CI`, com guard
    sobre `node_modules/*` e `node_modules/@*/*` (o pnpm corrompe symlinks se
    reinstalar por baixo de um worktree).
12. **`tb/pg.OpenPool` é do kernel, não do contexto.** O `resetStatement` é
    constante com as cinco tabelas do kernel e a migração chamada é
    `dmpfpostgres.Migrate` (`libs/backend/go/dmpf-testkit/tb/pg/pool.go:22`).
    Ele não migra nem reseta `<ctx>_*`: teste verde por vacuidade ou linhas
    vazadas entre casos. O contexto tem harness próprio, no molde de
    `dmpf-provider-postgres/example/orders/testing_test.go`, que migra o kernel
    **e** o contexto e trunca as tabelas de ambos.
13. **Os harnesses Postgres truncam tabelas do kernel** (`dmpf_outbox`,
    `dmpf_inbox`, `dmpf_quarantine`), que todo contexto compartilha. O CI já
    serializa por estágio (`.github/workflows/ci.yml`, `--parallel=1` nos
    `test-race` com `cache: false`); localmente, `nx run-many -t test-race`
    sem `--parallel=1` faz duas suítes truncarem a outbox uma da outra. O
    `dependsOn` inter-contexto (`<name>-provider-postgres-go` →
    `dmpf-provider-postgres-go`) dá paridade parcial; `--parallel=1` é a
    garantia.
14. **D002 sem shared kernel.** Sem `dmpf-kernel/*` em `shared_kernel_units`
    do baseline, `bookings-domain → dmpf-kernel/domain` reprova com
    `DMPF-D002`. Hoje a designação existe (ARQ-553, quinze unidades); o
    `self-test` do `tools/dmpf-harness-check.sh` sabota a lista para provar
    que o gate ainda morde.
