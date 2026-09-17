# Armadilhas verificadas

Cada entrada foi observada de fato — na prova do generator (ARQ-546), no shared
kernel (ARQ-553) ou na construção do golden `bookings` (ARQ-554). Quando o
golden for regenerado e algo novo morder, a entrada nova vem para cá.

1. **`time` no bloco `domain` reprova.** O verificador classifica o package
   `time` inteiro como `io.clock` (`conformance/internal/rule/stdlib.go`,
   `capability.go`), e o `depguard` do bloco nega o import. O instante chega
   por parâmetro, como inteiro de nanossegundos. `errors.New`, `fmt.Errorf`,
   `panic` e `fmt.Print*` são os outros red controls do `forbidigo`, restritos
   a `/domain/`.
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
6. **Sem a unidade `<ctx>/contract` no manifesto do `contracts`, o
   `gen/go` novo cai em `DMPF-U001`** e o `--write-baseline` aborta. A
   unidade entra antes do `generate`.
7. **`pnpm install` deixou de ser passo do rito.** Enquanto o contexto nascia
   em `libs/backend/go`, o `package.json` gerado o tornava importer do pnpm e
   os targets falhavam de forma obscura sem o install. Em `apps/backend/<name>`
   o módulo fica fora dos globs do `pnpm-workspace.yaml` (`apps/*`): o
   `package.json` continua a existir para o Nx, e só ele o lê.
8. **A composition root fica fora do generator.** Cabear processo é copiar
   `apps/backend/orders/cmd/orders`; a skill aponta o passo,
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
    `postgres.Migrate` (`libs/backend/go/testkit/tb/pg/pool.go:22`).
    Ele não migra nem reseta `<ctx>_*`: teste verde por vacuidade ou linhas
    vazadas entre casos. O contexto tem harness próprio, no molde de
    `apps/backend/orders/provider/testing_test.go`, que migra o kernel
    **e** o contexto e trunca as tabelas de ambos.
13. **Os harnesses Postgres truncam tabelas do kernel** (`dmpf_outbox`,
    `dmpf_inbox`, `dmpf_quarantine`), que todo contexto compartilha. O CI já
    serializa por estágio (`.github/workflows/ci.yml`, `--parallel=1` nos
    `test-race` com `cache: false`); localmente, `nx run-many -t test-race`
    sem `--parallel=1` faz duas suítes truncarem a outbox uma da outra. O
    `dependsOn` inter-contexto (`<ctx>` → `postgres`) dá paridade parcial;
    `--parallel=1` é a garantia.
14. **D002 sem shared kernel.** Sem `kernel/*` em `shared_kernel_units`
    do baseline, `bookings/domain → kernel/domain` reprova com
    `DMPF-D002`. Hoje a designação existe (ARQ-553, dezesseis unidades com o
    `kernel/provider-memory`); o
    `self-test` do `tools/dmpf-harness-check.sh` sabota a lista para provar
    que o gate ainda morde.
15. **Um package por bloco.** O agente tende a criar subpackages por agregado
    (`bookings/`, `resources/`) dentro do bloco. A unidade do manifesto aponta
    o import path do package do bloco (`<módulo>/<bloco>`), então subpackage
    vira unidade não declarada e o verificador reprova com `DMPF-U*`. Um
    agregado é um par de arquivos no package (`booking.go`, `resource.go`),
    não um diretório.
16. **O contexto mora em uma pasta; cada bloco é um subdiretório dela.**
    `apps/<scope>/<ctx>/<bloco>` — não `<ctx>-<bloco>` como módulo irmão, e
    não em `libs/backend/go`, que só guarda o kernel de reuso. O `dirName` de cada bloco está em
    `tools/dmpf-plugin/src/generators/bounded-context/blocks.ts` (`LAYOUTS`), e
    é o generator que decide o caminho. O contexto é **um** módulo Go
    (`go.mod` na raiz de `<ctx>`), e o nome do projeto Nx é `<ctx>`, sem
    prefixo nem sufixo (ADR-045).
17. **`gofmt` reprova import fora de ordem alfabética dentro do grupo.** O
    agente agrupa por origem (kernel primeiro, contexto depois), o que é
    legítimo, mas erra a ordem dentro do grupo — `domain` antes de
    `bookings/domain` parece certo pela leitura e é errado pelo alfabeto. Foi
    a única reprovação de `fmt-check` em toda a entrega, e em seis arquivos de
    uma vez. `gofmt -w` resolve; rodá-lo antes do `fmt-check` evita o ciclo.
18. **Bloco `port` sem superfície é bloco morto.** Os genéricos do kernel
    (`ports.Repository[ID,S]`, `Outbox`, `Reader[ID,S]`, `UnitOfWork[R]`)
    expressam quase toda a fronteira por instanciação, sem tipo novo. O que
    eles não expressam é a consulta que atravessa uma relação — e ela tende a
    ser declarada onde é consumida, no `application`. O resultado compila, passa
    nos gates e deixa a unidade `<ctx>/ports` declarada e vazia. A porta de
    consulta pertence ao bloco `port`, com o nome da consulta declarada na spec.
19. **Código gerado não sobrevive a substituição de texto.** O `rawDesc` de um
    `.pb.go` é o descritor do arquivo `.proto` serializado, com prefixos de
    comprimento: trocar um import path por outro de tamanho diferente sem
    recalcular o varint corrompe o descritor, e o pacote entra em
    `panic: slice bounds out of range` já no `init()`. Qualquer renomeação que
    alcance `gen/go` se conclui **regerando** pelo rito Buf, nunca editando.
20. **Suíte sem `BaselineStore` precisa declarar o shared kernel.**
    `Input.SharedKernelUnits` só vale quando `Baseline` é nil
    (`tools/dmpf-conformance/internal/conformance/check.go:30`) — que
    é exatamente o caso do `fitness` e do `selfcheck`, por desenho (`FIT-03`:
    a suíte julga a regra de dependência, não a autoridade sobre a
    classificação). Enquanto só existe o kernel, a omissão não aparece; o
    primeiro contexto de negócio a consumi-lo faz brotar um `DMPF-D002` por
    aresta. A correção é ler a designação do baseline governado e passá-la na
    `Input`, sem adotar o store — restar a lista no teste a faria divergir.
21. **A prova de regressão também envelhece.** O `tools/dmpf-harness-check.sh`
    carrega o layout do golden em `MODULOS` e nos globs de comparação. Depois
    da virada para pasta por contexto, ele seguiu procurando
    `libs/backend/go/<ctx>-<bloco>` e casando o diagnóstico por `<ctx>-domain`,
    quando o verificador imprime `<ctx>/domain`. Rodar o `self-test` a cada
    mudança de forma é o que expõe isso — foi ele que apontou os dois.
