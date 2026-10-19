---
id: SPEC-HFDSHFR1
slug: leitura-relacional-na-uow
title: DMPF — Leitura relacional na UoW com guarda declarada para pré-condições entre agregados
stage: building
priority: P0
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-132/spec-hfdshfr1-dmpf-leitura-relacional-na-uow-com-guarda-declarada-para
subtask_urls: []
created: 2026-10-18
---

# SPEC-HFDSHFR1: Leitura relacional na UoW com guarda declarada

## Resumo

Dar ao `postgres.Table` uma **coluna de guarda** declarada e ao `Relation` uma leitura
dentro da transação da UoW. Essa leitura toma um advisory lock compartilhado sobre a
chave `(tabela, coluna, tenant, valor)` antes de ler. Todo `Save` de uma tabela com
guarda toma o lock exclusivo da mesma chave antes de escrever. Assim, a pré-condição
relacional de um comando (por exemplo, "ao menos uma capacidade habilitada para o
parceiro") fica serializada com toda escrita que possa mudá-la, inclusive a inserção de
uma nova versão da série. Tudo isso continua em `READ COMMITTED` e sem retry automático.

A garantia é de ordem: a pré-condição vale no commit do comando, porque a escrita
concorrente na série espera esse commit e é serializada depois dele. Não é uma
invariante permanente entre agregados. Se a escrita posterior também precisa preservar
o estado do outro agregado, ela tem a sua própria pré-condição, declarada pelo contexto.

Origem: o consumidor externo antecipa-market, SPEC-9TEQ9EV9 (DEVS-78). Lá, a Nota (†)
aceita o write skew entre `ChangePartnerStage` e `SuspendCapability`, e entre
`EnableCapability` e a supersessão de acordo ou de perfil, "até o kernel oferecer
leitura relacional na transação (DEVS-132)".

## Contexto

### Estado atual

Verificado no código de `lidercap-platform` em 2026-10-18. Os caminhos `.go` são
relativos a `libs/backend/go/`.

| Ponto | Evidência |
| ----- | --------- |
| `Relation.Query` só aceita a struct `ReadPool` e roda em autocommit, fora da UoW | `postgres/table.go:49,106-112` |
| O SQL do `Relation` filtra por igualdade numa coluna, sem ordem nem `LIMIT` | `postgres/table.go:78-92,103-104` |
| `load` e `miss` já leem por uma interface `querier`, que a Tx satisfaz | `postgres/table.go:258-262,287` |
| A UoW abre `READ COMMITTED` fixo, sem opção por comando, e entrega a `*Tx` ao `bind` | `postgres/uow.go:38,52,62` |
| `READ COMMITTED` é premissa de INB-18 e da claim de idempotência, presa por teste | `postgres/uow_test.go:177`; `docs/dmpf/uow-inbox-outbox.md:1125-1137` |
| A UoW não repete o callback; retry de deadlock ou de serialização é política do caso de uso | `docs/dmpf/uow-inbox-outbox.md:568-577` (UOW-09, UOW-10) |
| Porta usada no callback e não recebida por ele está fora da fronteira | `docs/dmpf/uow-inbox-outbox.md:485-488` (UOW-04) |
| `Save` escreve com `version = $3`, que protege só o agregado gravado; 0 linhas viram `ErrVersionConflict` | `postgres/table.go:347-362` |
| Com `Unique`, o `Save` vai num batch com `SAVEPOINT dmpf_unique` | `postgres/table.go:349-351`; `postgres/unique.go:61-131` |
| O kernel já usa `pg_advisory_xact_lock` com chave `int8` fixa (`migrateLockID`) | `postgres/migrate.go:18,88`; `postgres/grants.go:66` |
| Não existe `FOR SHARE`, `SERIALIZABLE` nem lock em `Table`/`Relation` | busca em `libs/backend/go` |
| `40P01` só é reconhecido como transitório no consumo (R1D3); no caminho síncrono, `CategoryOf` o deixa sem categoria | `application/disposition.go:85-93`; `application/failure.go:69-85` |
| A regra do contexto diz que consultas correm fora da UoW | `.claude/rules/dmpf-bounded-context.md:60`, renderizada de `tools/dmpf-plugin/templates/ai/` |
| O ADR-066 recusou a pré-checagem por leitura para unicidade; o `Relation` fica fora da transação | `docs/adr/066-unicidade-declarada-como-rejeicao.md` |

### O caso do consumidor

Em `antecipa-market`, na branch `feat/DEVS-78-partner-management` (HEAD `8048568`,
kernel `v1.0.0-rc.5`):

- `ChangePartnerStage` lê acordos e capacidades por `Relation("partner_id")` no
  `ReadPool` (`application/preconditions.go:99,115`) e grava só o Partner.
- `EnableCapability` lê acordos e perfis do mesmo modo (`preconditions.go:99,107`) e
  grava só a Capability.
- A série muda por dois caminhos:
  - por `UPDATE` da linha, como em `SuspendCapability`;
  - por `INSERT` de uma nova versão com `supersedes_id`, como em `DeclareCapability`,
    `RegisterAgreement` e `RegisterIntegrationProfile` (`application/declare_capability.go:40`).
  Não há FK sobre `supersedes_id` (`provider/schema.sql`).
- A vigência fica no `snapshot` e é avaliada em Go (`domain/effective.go`). Por isso, a
  pré-condição não cabe num predicado SQL.

### Fatos do PostgreSQL

Sondados nesta redação no PostgreSQL 16.15, imagem `postgres:16-alpine`, a mesma de
`infra/test/compose.yml:10`:

- `SELECT ... FOR SHARE` sobre a série bloqueia o `UPDATE` concorrente da linha lida (o
  `UPDATE` esperou 1,6 s pelo commit do leitor). Ele **não** bloqueia o `INSERT` de uma
  versão que supersede a lida (0,08 s): o fantasma passa.
- `FOR SHARE` que espera um `UPDATE` não commitado devolve, em `READ COMMITTED`, a
  versão nova da linha depois do commit.
- `FOR SHARE` exige o privilégio `UPDATE`: uma role com só `SELECT` recebe `42501`. O
  advisory lock não exige privilégio de tabela.
- Uma transação `SERIALIZABLE` que lê a capacidade e grava o Partner commita sem erro
  quando o escritor concorrente roda em `READ COMMITTED`. O estado final foi Partner
  `active` com capacidade `suspended`. A SSI só detecta o write skew se todos os
  participantes forem `SERIALIZABLE`.
- `pg_advisory_xact_lock_shared` é compatível com outro compartilhado (0,06 s) e espera o
  exclusivo (1,5 s).
- **O snapshot é da instrução.** O leitor espera o lock compartilhado enquanto o escritor
  segura o exclusivo e insere. Com o lock e o `SELECT` em instruções separadas, o leitor
  vê a linha inserida (2 linhas). Com o lock numa subconsulta da mesma instrução, não vê
  (1 linha).
- Duas sessões que seguram o compartilhado e pedem o exclusivo da mesma chave terminam
  em `40P01`, com uma delas abortada.
- `hashtextextended(text, int8)` existe e devolve `int8`.

### Alternativas avaliadas

| Mecanismo | Cobre `UPDATE` | Cobre `INSERT` de supersessão | Compatível com UoW em RC e sem retry | Veredito |
| --------- | -------------- | ----------------------------- | ------------------------------------ | -------- |
| `FOR SHARE` na série lida | sim | **não** | sim, mas exige `UPDATE` na role | insuficiente |
| `SERIALIZABLE` por comando | só se todos os escritores forem `SERIALIZABLE` | idem | **não**: quebra INB-18 e exige retry (UOW-09, UOW-10) | recusado |
| Advisory lock por chave declarada, compartilhado na leitura e exclusivo no `Save` | sim | sim | sim | **adotado** |

### Verificado na implementação

Os dois pontos que a redação deixou em aberto foram provados por teste em 2026-10-18:

- Lock e `SELECT` no mesmo `pgx.Batch` recebem snapshots distintos: a leitura que esperou
  o lock enxerga a linha que o detentor do exclusivo inseriu e commitou
  (`TestTheGuardMakesThePreconditionReadWaitForTheSeriesWrite`). O `QueryWithin` paga uma ida.
- O `ROLLBACK TO SAVEPOINT` **libera** o advisory lock tomado depois do `SAVEPOINT`: com o
  lock depois dele, `TestTheGuardOutlivesTheSavepointARefusedWriteRollsBackTo` reprova. O
  lock vem antes do `SAVEPOINT`, e o teste prende essa ordem.

<constraints>
- [P0] A UoW continua `READ COMMITTED`, com uma transação e sem retry automático (UOW-01, UOW-09, UOW-10, INB-18).
- [P0] A serialização cobre toda escrita do kernel na tabela com guarda, `UPDATE` e `INSERT`: nenhuma escrita por `Table.Repository` muda a série sem o lock exclusivo da chave.
- [P0] O lock e a leitura são instruções separadas: a leitura nunca usa um snapshot anterior à aquisição do lock.
- [P0] A chave do lock inclui o tenant (IDN-11): um tenant nunca espera por outro, exceto por colisão de hash.
- [P0] Uma tabela sem `Guard` não muda: SQL, idas ao servidor e erros idênticos aos do rc.5 e do commit `26d115fe`.
- [P1] Nenhum valor da chave entra em log, span ou mensagem de erro além do que o kernel já registra hoje (ERR-07).
</constraints>

## Requisitos

### Funcionais

#### Onda 0 — ADR e instruções (antes do código)

- [ ] **[P0] ADR-067**: registrar a decisão em `docs/adr/067-leitura-relacional-na-uow-com-guarda.md`, com a tabela de alternativas e os fatos sondados, e indexá-lo em `docs/adr/README.md`. Emenda ADR-051 (o pool deixa de ser o único executor do `Relation`) e cita ADR-034 e ADR-066.
- [ ] **[P0] Regra do contexto**: em `tools/dmpf-plugin/templates/ai/.claude/rules/dmpf-bounded-context.md__tmpl__:60`, trocar "Consultas correm fora da UoW." por um texto que mantenha a regra para queries (UOW-11) e mande a pré-condição relacional de comando ler pela guarda dentro da UoW. Renderizar a cópia com o `init`.

#### Onda 1 — kernel (`postgres`)

- [ ] **[P0] `Table.Guard`**: campo opcional `Guard string`. Quando não vazio, `validate` exige identificador SQL presente em `Columns` e entra em pânico com mensagem nomeando a tabela e a coluna, no mesmo estilo de `table.go:204-224`.
- [ ] **[P0] Leitura na UoW**: `Relation.QueryWithin(ctx, tx *Tx, value any) ([]S, error)`, com o mesmo resultado de `Query`, inclusive `ports.CrossTenantAccess` por `miss`.
  - Entra em pânico se a coluna do `Relation` não for a `Guard` da tabela: leitura relacional na UoW sem guarda não serializa nada.
  - Toma `pg_advisory_xact_lock_shared(<chave>)` na Tx e depois executa o `SELECT` e o `miss`, como instruções separadas.
- [ ] **[P0] Escrita com guarda**: com `Guard` declarada, `Save` toma `pg_advisory_xact_lock(<chave>)` com o valor codificado da coluna de guarda antes do `INSERT` ou `UPDATE`.
  - Com `Unique`, o lock entra no batch antes do `SAVEPOINT dmpf_unique`.
  - Sem `Unique`, lock e escrita vão num batch de uma ida.
- [ ] **[P0] Chave**: `hashtextextended(concat_ws(chr(31), '<tabela>', '<coluna>', length($tenant::text), $tenant::text, $valor::text), 0)`, com tabela e coluna literais compiladas uma vez por `Table`. A leitura e a escrita produzem a mesma chave para o mesmo valor. O comprimento do tenant fixa a fronteira entre tenant e valor: sem ele, um tenant cujo ID contém `chr(31)` chegaria à chave de outro.
- [ ] **[P0] Valor da guarda só texto**: `QueryWithin` e o valor que o `Encode` produz para a coluna de guarda aceitam só kind `string` (inclusive tipos nomeados `~string`). Outro kind volta erro sem tocar o banco: valores equivalentes com representações textuais distintas (UUID em caixas diferentes, `timestamptz` com offsets, decimal com escalas) dariam chaves diferentes.
- [ ] **[P1] Guarda imutável**: o `UPDATE` de uma tabela com `Guard` inclui `<guard> = $n` no `WHERE`. Trocar a coluna de guarda de um agregado existente volta `ports.ErrVersionConflict`.
- [ ] **[P1] Deadlock com guarda**: a `Tx` registra que tomou uma guarda, compartilhada ou exclusiva. Daí em diante, `40P01` em qualquer instrução de `Load`, `Save` ou `QueryWithin` dessa Tx volta embrulhado em `ports.ErrVersionConflict`, porque o ciclo pode vitimar o `UPDATE` e não o lock. Assim, `CategoryOf` o classifica como `Conflict`, e o gRPC o devolve como `Aborted`, "replay the call" (`grpc/status.go:77`). Uma Tx que não tomou guarda mantém o erro atual.
- [ ] **[P2] Documentação do módulo**: `libs/backend/go/postgres/README.md` documenta `Guard`, `QueryWithin` e o padrão de deadlock (ler e escrever a mesma chave em comandos concorrentes).

#### Onda 2 — prova

- [ ] **[P0] Write skew por `UPDATE`**: teste de integração em `postgres/guard_concurrency_test.go`, 50 rodadas, com dois agregados de tabelas distintas. Um comando lê a série por `QueryWithin` e grava o outro agregado; o concorrente atualiza a linha lida. Nenhuma rodada termina com a decisão tomada sobre o estado já sobrescrito.
- [ ] **[P0] Write skew por `INSERT`**: o mesmo teste, em que o concorrente insere uma linha nova da série (supersessão).
- [ ] **[P0] Ordens determinísticas**: antes do código, testes de write skew nas ordens A→B e B→A, sincronizados por espera observada em `pg_locks`. As 50 rodadas são estresse adicional.
- [ ] **[P1] Snapshot do batch**: teste que prova que a leitura vê a escrita commitada pelo detentor do lock exclusivo, no formato de envio adotado.
- [ ] **[P1] Guarda após savepoint**: depois de uma violação de unicidade desfeita por `ROLLBACK TO SAVEPOINT`, outra Tx continua esperando a guarda até o fim da Tx externa, nos caminhos `inline` e não-`inline` de `unique.go`.

#### Onda 3 — publicação

- [ ] **[P0] Release `1.0.0-rc.6`**: publicar a versão pela sequência de `CONTRIBUTING.md` (preparação com o `require` dos irmãos em `v1.0.0-rc.6` e `modsync --write`, promoção por fast-forward de `release/1.0.0-rc.6`, tags pelo `nx-release.yml`, back-merge). A pré-release versiona os 21 projetos (`.github/workflows/nx-release.yml:160-166`), não só o `postgres`.
  - Recuperação por estágio: dispatch com `version=1.0.0-rc.6` só se o workflow falhou antes do push das tags (`nx-release.yml:186-198`); depois dele, reexecutar só o job que falhou.
  - O release tem issue própria no Linear, como a DEVS-115 na rc.5.

### Não-funcionais

- [ ] Custo: `QueryWithin` e `Save` com guarda mantêm uma ida ao servidor com conexão quente, medida por teste que conta escritas no socket, como `TestASaveUnderTheSavepointCostsOneRoundTrip`. A relação vazia soma uma ida, a da consulta de dono, como hoje. O batch de leitura preserva o snapshot (ver Verificado na implementação), e o número é fixado por teste.
- [ ] Compatibilidade: pgx `>=5.10 <6` e PostgreSQL 16, como em `postgres/dmpf-units.json`. Sem mudança de schema exigida do contexto além de declarar `Guard`.
- [ ] Privilégio: a role de runtime de ADR-063 não precisa de grant novo.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| `domain`, `ports`, `application` | [ ] | Sem mudança. `ErrVersionConflict` e `CategoryOf` já cobrem a classificação |
| `postgres` (provider) | [x] | `Guard`, `QueryWithin`, lock no `Save`, marca de guarda na `Tx` e tradução do `40P01` |
| `memory` | [ ] | Não tem `Relation`; fora do escopo |
| `grpc`, `http`, `transport` | [ ] | `Aborted` já existe para `Conflict` |
| Instruções de IA (`tools/dmpf-plugin/templates/ai`) | [x] | Regra do contexto, linha 60 |
| Golden `bookings` | [ ] | Sem pré-condição entre agregados |

## Localização de código

```text
libs/backend/go/postgres/
  table.go                     — Guard, validate, QueryWithin, lock no Save, WHERE da guarda
  guard.go                     — compilação da chave e statements de lock, tradução do 40P01
  unique.go                    — lock antes do SAVEPOINT no batch existente
  uow.go                       — marca de guarda tomada na Tx
  guard_test.go                — validação da Guard e do kind do valor, pânico sem guarda
  guard_table_test.go          — leitura, lock, Save, savepoint, tenant, deadlock, snapshot, idas
  guard_concurrency_test.go    — write skew ordenado e de estresse, por UPDATE e por INSERT
  README.md                    — documentação
docs/adr/067-leitura-relacional-na-uow-com-guarda.md
docs/adr/README.md
tools/dmpf-plugin/templates/ai/.claude/rules/dmpf-bounded-context.md__tmpl__
.claude/rules/dmpf-bounded-context.md   — renderizada pelo init
```

## Design

### Fluxo principal

1. O contexto declara `Guard: "partner_id"` nas tabelas cuja série é lida como
   pré-condição (`capabilities`, `partner_agreements`, `integration_profiles`).
2. No `bind` da UoW, o reader da pré-condição recebe a `*Tx` e chama
   `xTable.Relation("partner_id").QueryWithin(ctx, tx, partnerID)`.
3. `QueryWithin` toma o lock compartilhado da chave e, numa instrução seguinte, lê a série.
   Os comandos que só leem a mesma chave não esperam uns pelos outros.
4. O comando decide e grava o seu agregado. O lock compartilhado dura até o commit.
5. O `Save` de qualquer linha da série (`UPDATE` ou `INSERT` de supersessão) pede o lock
   exclusivo da chave e espera o commit dos leitores. Em seguida, os leitores que chegam
   esperam o commit do escritor e leem a série nova.

### Pseudocódigo

```text
QueryWithin(ctx, tx, value):
    assert relation.filter == table.Guard          // pânico: erro de programação
    tenant = tenantOf(ctx)
    exec tx "SELECT pg_advisory_xact_lock_shared(key($1,$2))" tenant, value
        on 40P01 -> wrap ErrVersionConflict
    rows = query tx relation.statement tenant, value   // instrução separada
    if rows empty -> miss(ctx, tx, ...)
    return decode(rows)

Save(ctx, id, state, expected):
    values = Encode(state)
    if table.Guard != "":
        batch += "SELECT pg_advisory_xact_lock(key($1,$2))" tenant, values[guardIndex]
    if table.Unique: batch += SAVEPOINT; write; RELEASE   // fluxo atual de unique.go
    else:            batch += write
    send batch; 40P01 no lock -> wrap ErrVersionConflict
```

## Decisões técnicas

- **Advisory lock por chave declarada**: é o único mecanismo sondado que cobre o
  `INSERT` de supersessão e mantém `READ COMMITTED`.
  - Alternativa descartada: `FOR SHARE`, porque não bloqueia o fantasma e exige `UPDATE` na role.
  - Alternativa descartada: `SERIALIZABLE` por comando, porque não detecta o skew contra
    escritor em `READ COMMITTED`, quebra INB-18 e exige retry, proibido como default por UOW-09 e UOW-10.
- **Guarda declarada na `Table`, lock exclusivo no `Save`**: o escritor não precisa
  lembrar de travar, porque o choke point do ADR-051 trava por ele.
  - Alternativa descartada: advisory lock explícito no caso de uso, porque depende de cada
    escritor da série chamar o lock, e um esquecido reabre o skew em silêncio.
- **`QueryWithin` exige a coluna de guarda**: ler na UoW por outra coluna daria a
  aparência de consistência sem serializar nada.
- **Chave por `hashtextextended` no servidor**: uma ida, sem hash próprio em Go. Uma
  colisão causa só espera indevida, nunca leitura errada. A chave `int8` divide espaço com
  `migrateLockID`, com probabilidade de colisão de 2^-64.
- **Deadlock como `Conflict`**: o `40P01` da guarda significa que a decisão concorria com
  outra sobre a mesma série. "Repita a chamada" é a resposta correta, e a repetição fica
  com o chamador, como manda UOW-10.

## Regras relacionadas

- `docs/dmpf/uow-inbox-outbox.md` — UOW-01, UOW-04, UOW-09, UOW-10, UOW-11, INB-18.
- ADR-034 (fronteira da UoW), ADR-051 (choke point de tenant), ADR-063 (roles), ADR-066 (unicidade com savepoint).
- SPEC-7SRQ3RPK — o batch com savepoint em que o lock da guarda entra.
- `antecipa-market` SPEC-9TEQ9EV9, Nota (†) — consumidor que adota a guarda.

## Verificação e testes

### Critérios de aceite

- [ ] Os testes ordenados A→B e B→A falham sem a guarda e passam com ela.
- [ ] Os dois testes de write skew (`UPDATE` e `INSERT`) passam 50 de 50 rodadas com `-race`.
- [ ] Os mesmos cenários com `Relation.Query` no `ReadPool` reproduzem o skew em ao menos 1 rodada, o que prova que o teste detecta a corrida.
- [ ] `QueryWithin` sobre coluna que não é a `Guard` entra em pânico, `Guard` inválida reprova em `validate`, e valor de guarda que não é `string` volta erro.
- [ ] Um `40P01` numa Tx que tomou guarda, seja na instrução de lock ou no `UPDATE` de um ciclo misto, chega como `ports.ErrVersionConflict` e `CategoryOf` devolve `Conflict`.
- [ ] Depois de uma violação de unicidade desfeita pelo savepoint, a guarda continua presa até o fim da Tx.
- [ ] Tabela sem `Guard`: os testes existentes de `table_test.go`, `unique_*_test.go` e `uow_test.go` passam sem alteração.
- [ ] Contagem de idas fixada por teste para relação com linhas, vazia e `Save` com guarda, conforme o requisito não-funcional de custo.
- [ ] `pnpm nx run postgres:test-race`, `fmt-check`, `vet` e `lint` verdes; `go run ./tools/dmpf-conformance/cmd/conformance --root .` verde; `biome check` verde.
- [ ] Tag `libs/backend/go/postgres/v1.0.0-rc.6` publicada e `GOWORK=off go list -m github.com/mateusmacedo/dmpf/libs/backend/go/postgres@v1.0.0-rc.6` resolve; `develop` com o back-merge da rc.6.

### Cenários de teste

```text
DADO capacidade c1 enabled do parceiro p1 e Partner p1 homologating
QUANDO o comando A lê capacidades por QueryWithin e, antes do commit, o comando B tenta
      atualizar c1 para suspended
ENTÃO B espera o commit de A, e o estado final corresponde a A seguido de B

DADO a série de p1 com c1 enabled corrente
QUANDO B segura o lock exclusivo e insere c2 que supersede c1 com status declared, e A
      chama QueryWithin antes do commit de B
ENTÃO A espera o commit de B, lê c1 e c2 e decide sobre a série com c2 corrente

DADO dois comandos que leem a série de p1 por QueryWithin e depois gravam uma linha dela
QUANDO ambos seguram o compartilhado e pedem o exclusivo
ENTÃO um deles recebe ports.ErrVersionConflict, com categoria Conflict, e o outro commita

DADO a série de p1 no tenant t1
QUANDO um comando do tenant t2 segura o lock exclusivo de p1
ENTÃO QueryWithin em t1 não espera

DADO Table sem Guard
QUANDO o contexto chama Relation("partner_id").QueryWithin
ENTÃO pânico nomeando a tabela e a coluna
```

<critical_constraints>
- [P0] A UoW continua `READ COMMITTED`, com uma transação e sem retry automático (UOW-01, UOW-09, UOW-10, INB-18).
- [P0] A serialização cobre toda escrita do kernel na tabela com guarda, `UPDATE` e `INSERT`: nenhuma escrita por `Table.Repository` muda a série sem o lock exclusivo da chave.
- [P0] O lock e a leitura são instruções separadas: a leitura nunca usa um snapshot anterior à aquisição do lock.
- [P0] A chave do lock inclui o tenant (IDN-11): um tenant nunca espera por outro, exceto por colisão de hash.
- [P0] Uma tabela sem `Guard` não muda: SQL, idas ao servidor e erros idênticos aos do rc.5 e do commit `26d115fe`.
- [P1] Nenhum valor da chave entra em log, span ou mensagem de erro além do que o kernel já registra hoje (ERR-07).
</critical_constraints>

## Escopo fora

- **Adoção no partner-management e atualização da Nota (†)**: ficam no `antecipa-market`
  (DEVS-78), depois da publicação da versão do `postgres` com a guarda. O consumidor
  depende da versão, não deste repositório.
- **`memory.Table` com `Relation`/`QueryWithin`**: o adaptador em memória não tem
  `Relation` hoje (`memory/store.go:39-104`), e os contextos usam dublês nas portas próprias.
- **Golden `bookings`**: não tem pré-condição entre agregados. Criar uma só para exercitar
  a guarda mudaria o domínio de referência sem necessidade; a prova fica no `postgres`.
- **Retry automático do deadlock**: proibido como default por UOW-09 e UOW-10. O chamador
  repete a partir de `Aborted`.
- **`lock_timeout` por comando**: o tempo de espera segue a configuração do pool do
  contexto. Limite por comando é outra decisão.
- **Pré-condição negativa sobre o tenant inteiro** (sem coluna de guarda): fora do caso da
  DEVS-132, que filtra sempre por um valor de chave.
- **Emenda normativa em `uow-inbox-outbox.md`**: UOW-04 já enuncia a propriedade de que a
  leitura fora do callback está fora da fronteira. O ADR-067 registra a realização do kernel.
