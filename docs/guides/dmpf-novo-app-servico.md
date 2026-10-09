# Guia prático: criar uma app de serviço DMPF

Este guia conduz da definição do contexto à validação da app, priorizando os
comandos que já existem no workspace. Ele complementa o
[guia de implementação DMPF](./dmpf-implementation.md) e o
[guia de composição](./dmpf-composicao.md).

Use-o para criar um novo serviço de negócio em `apps/backend/<nome>`. Um
contexto de negócio é uma **app** com um package Go por bloco;
`libs/backend/go` fica reservado ao kernel reutilizável.

## O caminho em uma linha

```text
spec revisada → generator em dry-run → esqueleto → implementação por bloco
→ contrato e classificação → testes e gates → README operacional
```

O generator cria a estrutura, os manifestos e a configuração inicial. Ele não
implementa as regras do negócio: essas vêm da spec e são desenvolvidas nos
blocos do contexto.

## 1. Escolha uma referência pelo que ela ensina

Não copie uma app inteira. Comece pelo generator e consulte a app que melhor
corresponde ao comportamento desejado:

| Referência | Use para estudar |
| --- | --- |
| [`bookings`](../../apps/backend/bookings/README.md) | Forma canônica do contexto e harnesses `appkit`/`distkit`; é o golden do workspace |
| [`orders`](../../apps/backend/orders/README.md) | Serviço gRPC com `api` e `relay`, sem consumo de eventos |
| [`reservations`](../../apps/backend/reservations/README.md) | Serviço gRPC que também consome eventos e usa inbox |

Copie **padrões**, não nomes, regras de domínio, permissões, schema, variáveis de
ambiente, contratos ou configuração de transporte da referência.

## 2. Prepare a definição antes do código

Antes de gerar a app, confirme que a spec do bounded context existe em
[`docs/specs/`](../specs/README.md). Use o
[template de bounded context](../../.agents/skills/dmpf-bounded-context/references/template-bounded-context.md)
e registre, no mínimo:

- nome do projeto/app e valor literal de `bounded_context`;
- agregados, identidade, estado e invariantes;
- comandos/UPRs, pré-condições e rejeições esperadas;
- consultas, eventos publicados e consumidos;
- critérios de aceite que possam virar testes nomeados;
- o que fica fora do escopo desta versão.

Não deduza decisões de domínio olhando uma implementação de referência. A spec
é que define o comportamento; os exemplos ajudam a reconhecer onde cada
responsabilidade costuma ficar.

O nome da app e o `bounded_context` são valores distintos. Por exemplo, a app
`bookings` declara `resource-scheduling` como `bounded_context`. Ambos usam
kebab-case: o nome identifica a app e o projeto Nx; o segundo identifica a
unidade de domínio nos manifestos.

## 3. Descubra os projetos e os comandos disponíveis

Consulte o Nx antes de presumir que uma app oferece determinado target:

```bash
# Apps conhecidas pelo Nx
pnpm nx show projects --type app

# Projetos que expõem este target
pnpm nx show projects --withTarget serve-api

# Targets, tags e configuração resolvida de uma app existente
pnpm nx show project bookings --json
```

Depois da geração, troque `bookings` pelo nome da app nova. A configuração
resolvida é a fonte para confirmar os targets disponíveis; não copie uma lista
de comandos de outro contexto sem verificá-la.

## 4. Gere primeiro em modo de simulação

Substitua `<nome-app>` e `<bounded-context>` pelos valores aprovados na spec.
O nome do serviço gRPC pode ser ajustado com `--service-name`; sem a opção, o
generator deriva o default a partir do nome da app.

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <nome-app> \
  --bounded-context <bounded-context> \
  --dry-run
```

Revise a lista de arquivos antes de escrever. Se estiver correta, repita o
comando sem `--dry-run`:

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <nome-app> \
  --bounded-context <bounded-context>
```

O generator cria o módulo de contexto em `apps/backend/<nome-app>` e o módulo
companion `<nome-app>-contract`, além dos manifestos, configuração Nx, estrutura
de deploy e entradas no `go.work`. Também registra o contrato no grupo de
release e inclui o compose da app. Depois de gravar os arquivos, executa
`modsync` e `infrasync`. O generator cria a estrutura do módulo `contract`, mas
não escreve o source Protobuf nem o código gerado: esses seguem o rito Buf. Não
é necessário rodar `pnpm install`: o módulo novo em `apps/backend/<nome-app>` não é importer do pnpm.

Para opções adicionais ou outro diretório permitido, consulte o
[README do plugin](../../tools/dmpf-plugin/README.md). Não ajuste `--blocks`
para omitir uma parte do desenho sem confirmar que a combinação respeita as
dependências entre blocos.

Após gerar, confira o que mudou:

```bash
git status --short
git diff --stat
```

O baseline de classificação não é regravado automaticamente. O generator
imprime a instrução para fazê-lo; trate isso como uma decisão arquitetural e
revise o resultado:

```bash
go run ./tools/dmpf-conformance/cmd/conformance --root . --write-baseline
git diff -- tools/dmpf-conformance
```

Se for usar a geração assistida por spec, o command do workspace é
`/dmpf-new-context SPEC-<id>`. Antes de invocá-lo, confira pré-requisitos,
gates humanos e limites no [guia de composição](./dmpf-composicao.md).

## 5. Implemente de dentro para fora

Trabalhe numa ordem que deixe as regras de negócio explícitas antes de
introduzir infraestrutura:

1. **`domain/`** — agregados, UPRs e rejeições. Escreva testes para caminhos de
   sucesso, pré-condições, transições inválidas e invariantes.
2. **`application/`** — coordene a operação usando os contratos do kernel e as
   portas necessárias. Teste a sequência e as falhas sem depender de banco.
3. **`ports/`** — declare apenas interfaces específicas que o kernel não
   oferece. Não duplique uma capacidade que já existe.
4. **`provider/`** — implemente persistência, consultas e mapeamento do schema.
   Mantenha a tabela do agregado no plural, siga o padrão de tenant do kernel e
   cubra isolamento entre tenants nos testes de integração.
5. **Contrato** — defina o Protobuf da superfície pública necessária. Gere o
   código pelo Buf; não edite arquivos gerados à mão.
6. **`app/` e `cmd/`** — conecte os blocos na composition root e exponha a
   borda gRPC. O REST público pertence ao `bff`, não à app de contexto.
7. **Harnesses** — mantenha `appkit` e `distkit` na forma canônica, adaptando
   cenários e fixtures ao comportamento desta app.
8. **Configuração e deploy** — confirme `deploy/.env.example`, schema, banco,
   roles e comando de cada processo. Use os nomes definidos pelo contexto, sem
   prefixar a configuração com `DMPF_`.

Evite importar diretamente um contexto irmão. Para integração entre contextos,
use o contrato e a fronteira de transporte definidos para o caso de uso.
Quando um package do kernel colidir em nome com um do contexto, aplique o alias
pelo papel local, conforme a convenção do repositório.

## 6. Gere e valide o contrato quando ele mudar

O source Protobuf e os artefatos do contrato seguem o fluxo documentado em
[`dmpf-composicao.md`](./dmpf-composicao.md). Depois de alterar o contrato,
execute os gates Buf:

```bash
pnpm nx run contracts:buf-lint
pnpm nx run contracts:buf-pins
pnpm nx run contracts:buf-generate-check
NX_BASE=develop pnpm nx run contracts:buf-breaking
```

`buf-breaking` compara com a referência configurada para o trabalho; confirme
qual base aplicar antes de rodá-lo. Registre qualquer mudança incompatível como
decisão explícita, não como efeito colateral de regenerar código.

## 7. Rode os gates na app nova

Comece conferindo os targets realmente gerados:

```bash
pnpm nx show project <nome-app> --json
```

Rode a cadeia por app. O primeiro comando combina os gates disponíveis mais
comuns; confira os targets no passo anterior e ajuste a lista se necessário:

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test -p <nome-app>
```

Em seguida, rode os testes que provam a app com as tags e a infraestrutura
necessárias:

```bash
pnpm nx run <nome-app>:test-race
pnpm nx run <nome-app>:test-distributed
pnpm nx run <nome-app>:govulncheck
```

O `test-race` cobre o código com o race detector e pode iniciar a infraestrutura
de teste definida pelo projeto. O `test-distributed` é aplicável quando há
cenário distribuído e depende do broker de teste configurado. Os dois podem
depender de `PG_DSN`, Kafka/Redpanda ou das ferramentas descritas no README da
app. Não aceite um teste como validado só porque foi ignorado por falta de
configuração: siga o
[README da app de referência](../../apps/backend/bookings/README.md) e
[`infra/README.md`](../../infra/README.md) para preparar o ambiente.

Valide também o manifesto e a composição:

```bash
go run ./tools/dmpf-conformance/cmd/conformance --root .
go run ./tools/dmpf-conformance/cmd/modsync --root . --check
bash tools/dmpf-context-check.sh --root .
```

O verificador de contexto cobre a forma canônica da app; não contorne uma
reprovação editando baseline, manifesto ou teste sem primeiro entender a regra
que falhou.

## 8. Rode localmente sem inventar configuração

Descubra os papéis que a app oferece e consulte seu README e
`apps/backend/<nome-app>/deploy/.env.example` para os valores exigidos:

```bash
pnpm nx show project <nome-app> --json
pnpm nx run <nome-app>:serve-api
```

`serve-api` é um exemplo: execute apenas os targets `serve-*` que o Nx lista
para a app, como `serve-relay` ou `serve-consumer`. Configure o banco e, quando
necessário, o broker usando os exemplos versionados do projeto. Flags como
`GRPC_INSECURE=true` e `KAFKA_INSECURE=true` são opt-outs de desenvolvimento,
e não devem ser ativadas em produção. Os valores e comandos exatos de execução devem
ficar no README da app.

## 9. Feche com documentação e revisão

Antes de considerar a app replicável por outra pessoa, confirme que o
`apps/backend/<nome-app>/README.md` explica:

- propósito, nome do projeto e targets disponíveis;
- papéis que o binário executa e como iniciá-los localmente;
- variáveis de configuração e serviços externos necessários;
- testes unitários, de integração e distribuídos, inclusive seus pré-requisitos;
- contrato e localização de código gerado, quando aplicável.

Use esta lista de saída:

- [ ] A spec define o comportamento e os critérios de aceite.
- [ ] O generator foi executado depois de um `--dry-run`.
- [ ] A app tem tags, manifesto e estrutura aprovados pelo verificador.
- [ ] As regras de domínio e os caminhos de rejeição têm testes.
- [ ] Contrato e código gerado passaram pelo rito Buf, quando aplicável.
- [ ] `fmt-check`, `vet`, `lint`, `build` e testes necessários passaram.
- [ ] `conformance`, `modsync --check` e `dmpf-context-check.sh` passaram.
- [ ] O README permite preparar ambiente, executar e testar sem inferir valores.
- [ ] O diff contém apenas arquivos da mudança pretendida.

## Comandos de consulta rápida

| Objetivo | Comando |
| --- | --- |
| Listar apps conhecidas pelo Nx | `pnpm nx show projects --type app` |
| Encontrar projetos que têm um target | `pnpm nx show projects --withTarget serve-api` |
| Ver targets e tags resolvidos | `pnpm nx show project <nome-app> --json` |
| Simular geração | `pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <nome-app> --bounded-context <bounded-context> --dry-run` |
| Validar um projeto em cadeia | `pnpm nx run-many -t fmt-check,vet,lint,build,test -p <nome-app>` |
| Validar arquitetura e módulos Go | `go run ./tools/dmpf-conformance/cmd/conformance --root .` |
| Verificar sincronização de módulos Go | `go run ./tools/dmpf-conformance/cmd/modsync --root . --check` |
