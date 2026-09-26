---
paths:
  - "libs/backend/go/**"
  - "contracts/**"
  - "apps/backend/**"
---

# Bounded context sobre o kernel DMPF

Normas que quem escreve um bounded context — pessoa ou agente — obedece. Cada
regra é curta e aponta a fonte; a norma mora em `docs/dmpf/` e nos ADRs, e
quando este texto divergir dela, a fonte vence. Complementa as demais rules de
`.claude/rules/`, não as repete. O passo a passo com os arquivos-molde está na
skill `.agents/skills/dmpf-bounded-context/`.

## Fronteiras (ADR-010, ADR-017, shared kernel)

- Seis blocos, pertencimento único por unidade: `domain`, `port`,
  `application`, `provider`, `app`, `contract`. Um package por bloco, dentro
  do módulo único do contexto (ADR-045); nenhum arquivo pertence a dois
  (ADR-010).
- A regra de dependência é fail-closed e decidida sobre o grafo real de
  imports pelo `conformance`. Células proibidas não têm exceção por
  conveniência (ADR-010; `.golangci.yml` `depguard` por bloco).
- Todo contexto declara o seu `bounded_context`, estável e único; contextos
  distintos só se tocam pela superfície pública ou por unidade designada no
  shared kernel (ADR-017; `tools/dmpf-baseline/units-baseline.json`,
  `shared_kernel_units`). Importar o kernel é permitido porque ele **é** shared
  kernel — sem essa designação o verificador reprova com `DMPF-D002`.

## Classificação (ADR-012)

- A unidade é o que o manifesto `dmpf-units.json` declara, nunca o que o
  diretório sugere. Todo package de produção está no `include` de alguma
  unidade; package fora do manifesto reprova com `DMPF-U001`.
- Esqueleto — `project.json`, `go.mod`, `go.work`, `dmpf-units.json` — nasce
  do generator `bounded-context`; à mão só o `include` de packages novos, por
  merge, com `exceptions` e `public_integration_surface` preservados.
- Regravar o baseline (`--write-baseline`) é ato de classificação: commit
  próprio, só com o baseline, executado por pessoa (`DMPF-T002`; `docs/guides/dmpf-manifesto.md`).

## Domínio (ADR-032; FND-04)

- A UPR devolve `(Accepted[R], *Rejection)`: o segundo retorno é o tipo
  concreto, nunca `error`. Rejeição tem código estável
  `<ctx>/<agregado>/<rejeicao>`; a pré-condição pertence ao comando, não ao
  agregado (ADR-032).
- O bloco `domain` não importa `time` — o verificador classifica o package
  inteiro como `io.clock`; o instante entra por parâmetro como inteiro de
  nanossegundos. `errors.New`, `fmt.Errorf`, `panic` e `fmt.Print*` são
  proibidos ali (`forbidigo` restrito a `-domain/`).
- Agregados se relacionam por identidade, nunca por referência a objeto.

## Unit of Work e outbox (ADR-034, ADR-035)

- Um caso de uso é um `Within` sobre `UnitOfWork[R]`; quem monta `R` é o
  composition root, pelo `bind` (ADR-034). Consultas correm fora da UoW.
- O caso de uso produtor percorre os nove passos de FND-04 §3.2, na ordem —
  identidade antes da transação, autorização pelo gancho, evento para a outbox
  na **mesma** transação do estado (ADR-035).
- A outbox guarda os bytes do `Any` do integration event, serializados na
  escrita; o envelope CloudEvents é montado na publicação, pelo relay
  (ADR-035).

## Inbox e consumo (ADR-036; FND-04 §6.4)

- Só contexto que **consome** realiza a inbox. A classificação de recepção é
  tipo fechado com `Match` exaustivo; `Pending` só sob R1 (ADR-036).
- O consumo ramifica pelas sete disposições de FND-04 §6.4; `ErrSchemaMismatch`
  e `ErrMalformed` do envelope viram `Failure(Validation)` **sem** abrir UoW.
- O efeito de broker (`Ack`, `Release`, contenção) vem sempre depois do
  retorno da transação (`INB-08`).

## Contrato (ADR-033; PTB-01, REP-01)

- A fonte é `contracts/proto/company/<name>/event/v1/<evento>.proto`, package
  `company.<name>.event.v1` — `company` é fixo (PTB-01, REP-01). O gerado
  vive em `libs/backend/go/contracts/gen/go/` e só nasce pelo rito
  `tools/buf.sh generate`; nunca à mão (ADR-033).
- Contrato publicado é imutável: `.proto` existente não é regravado; evolução
  é rito próprio. `contracts/buf.yaml` é módulo único — não se cria um por
  contexto.
- A unidade `<ctx>/contract` entra no manifesto do `contracts` **antes**
  do `generate`; sem ela o gerado cai em `DMPF-U001`.

## Borda (ADR-044, ADR-053)

- O contexto serve só gRPC, em `app/rpc`: `ServiceDesc` no bloco `app`,
  `rpc.Methods()` para os limites por método e `errors.go` mapeando para
  status gRPC. A cadeia de interceptors é a do kernel
  (`kernelgrpc.ServerInterceptors`). `app/http` em contexto reprova no
  `tools/dmpf-context-check.sh`.
- O REST público é do `bff`. Toda rota declara `ContractRef` para o OpenAPI
  publicado em `contracts/openapi/<name>/v1/` (RST-04), que declara
  `bearerAuth`.
- Idempotência por método: `POST` de criação só com chave de idempotência
  declarada; `PATCH` nunca é idempotente (RST-02). Handlers validam a forma
  do corpo (`maxLength`, `pattern`, `additionalProperties: false`).

## Banco e persistência (ADR-053)

- Banco próprio por app: database e role com o nome da app, schema `public`.
  Tabela do kernel é substantivo sem prefixo (`outbox`, `inbox`,
  `quarantine`); tabela de agregado é o agregado no plural, sem prefixo;
  coluna de id `<agregado>_id`; índice `<tabela>_<colunas>_idx`; constraint
  `<tabela>_<colunas>_{pkey,key,check,fkey}`. O
  `tools/dmpf-context-check.sh` reprova DDL, DSN e banco fora disso.
- O DDL do contexto vive em `provider/schema.sql`; o composition root o aplica
  com `postgres.Migrate`, pedindo só as capacidades que usa (`Outbox`; `Inbox`
  se consome).
- Persistência híbrida: coluna tipada só para o que uma consulta filtra; o
  resto do estado vai em `snapshot` `jsonb`, por um struct de estado privado
  do provider com tags JSON estáveis.
- Cada projeto testa no seu banco `<projeto>_test`, derivado do servidor de
  `PG_DSN` pelo `tb/pg`; suítes de projetos distintos rodam em paralelo.

## Configuração e nomes (ADR-053)

- A configuração vem só do ambiente, sem prefixo `DMPF_` (`PG_DSN`,
  `GRPC_ADDR`, `KAFKA_BROKERS`), pela forma `Defaults(role)`, `FromEnv` e
  `Validate`; a falta de uma variável obrigatória recusa a partida.
- Imports do kernel com o alias do papel: `kernel` (domain), `usecase`
  (application), `port` (ports), `kernelgrpc`, `kernelhttp` e `kernelapp`.
- Nomes Go canônicos (`Code<Agregado><Motivo>`, evento no passado sem sufixo,
  status curto); o valor publicado no fio não muda com o nome Go.

## Observabilidade mínima (FND-08)

- O caso de uso passa pelo gancho de instrumentação do kernel
  (`observability/usecase`); rota e consumer expõem as três posições de
  observabilidade que `RES-23` exige. Erro é sempre amostrado (`TRC-14`).

## O que nunca se faz

- Escrever `project.json`, `go.mod`, `go.work` ou `dmpf-units.json` à mão
  (esqueleto pelo generator; `include` por merge).
- Regravar baseline ou `gen/go` (classificação e rito Buf são passos humanos).
- Afrouxar um gate para o contexto passar — o contexto passa pelos mesmos
  gates de qualquer módulo.
- Servir REST no contexto ou dividir um banco entre apps.
- Mudar um valor de fio já publicado (código de rejeição, enum, nome de
  evento) por causa de um rename no Go.
