# ADR-067: Ler a pré-condição relacional na UoW sob guarda declarada no Table

## Status

Aceito — 2026-10-18. Implementa [SPEC-HFDSHFR1](../specs/SPEC-HFDSHFR1-leitura-relacional-na-uow.md). Emenda o [ADR-051](./051-escopo-de-tenant-por-choke-point-em-go.md), no item do pool só como `ReadPool` (o `Relation` passa a ler também na `*Tx`, sob guarda). O [ADR-034](./034-fronteira-de-uow-em-go.md) não é emendado: a UoW segue uma transação, sem retry, e a guarda é interna ao `Table` e à `Tx`. No [ADR-066](./066-unicidade-declarada-como-rejeicao.md), o lock da guarda entra antes do `SAVEPOINT` do batch de unicidade.

## Contexto

Dois comandos que leem um agregado e gravam outro decidem cada um sobre um estado que o outro pode mudar antes do commit, e os dois commitam: é o write skew entre agregados. O `Save` protege só o agregado gravado (`version = $3`, e zero linhas viram `ErrVersionConflict`), e o `Relation` lê no `ReadPool`, em autocommit, fora da UoW. O consumidor externo antecipa-market (SPEC-9TEQ9EV9, DEVS-78, kernel `v1.0.0-rc.5`) tem o caso. `ChangePartnerStage` lê acordos e capacidades por `Relation("partner_id")` e grava só o Partner; `EnableCapability` lê acordos e perfis do mesmo modo e grava só a Capability. A série lida muda por dois caminhos: por `UPDATE` da linha, como em `SuspendCapability`, e por `INSERT` de uma nova versão com `supersedes_id`, como em `DeclareCapability`, `RegisterAgreement` e `RegisterIntegrationProfile`, sem FK sobre `supersedes_id`. A vigência vive no `snapshot` e é avaliada em Go, então a pré-condição não cabe num predicado SQL. A Nota (†) da SPEC-9TEQ9EV9 aceita o skew entre `ChangePartnerStage` e `SuspendCapability`, e entre `EnableCapability` e a supersessão de acordo ou de perfil, «até o kernel oferecer leitura relacional na transação (DEVS-132)».

O kernel impõe três restrições. A UoW abre `READ COMMITTED` fixo, sem opção por comando, e essa é a premissa de INB-18 e da claim de idempotência, presa por teste. Ela não repete o callback: o retry de deadlock ou de serialização é política do caso de uso (UOW-09, UOW-10). E a regra do contexto manda as consultas correrem fora da UoW, de modo que o `Relation` fica fora da transação, como o ADR-066 registrou ao recusar a pré-checagem por leitura na unicidade. O kernel já toma `pg_advisory_xact_lock` com chave `int8` fixa (`migrateLockID`), e o `40P01` só é reconhecido como transitório no consumo (R1×D3): no caminho síncrono, `CategoryOf` o deixa sem categoria.

Fatos sondados no PostgreSQL 16.15, imagem `postgres:16-alpine`, a mesma de `infra/test/compose.yml`:

- `SELECT ... FOR SHARE` sobre a série bloqueia o `UPDATE` concorrente da linha lida (o `UPDATE` esperou 1,6 s pelo commit do leitor) e **não** bloqueia o `INSERT` de uma versão que supersede a lida (0,08 s): o fantasma passa.
- `FOR SHARE` que espera um `UPDATE` não commitado devolve, em `READ COMMITTED`, a versão nova da linha depois do commit.
- `FOR SHARE` exige o privilégio `UPDATE`: uma role com só `SELECT` recebe `42501`. O advisory lock não exige privilégio de tabela.
- Uma transação `SERIALIZABLE` que lê a capacidade e grava o Partner commita sem erro quando o escritor concorrente roda em `READ COMMITTED`, e o estado final foi Partner `active` com capacidade `suspended`. A SSI só detecta o write skew se todos os participantes forem `SERIALIZABLE`.
- `pg_advisory_xact_lock_shared` é compatível com outro compartilhado (0,06 s) e espera o exclusivo (1,5 s).
- **O snapshot é da instrução.** O leitor espera o lock compartilhado enquanto o escritor segura o exclusivo e insere. Com o lock e o `SELECT` em instruções separadas, o leitor vê a linha inserida (2 linhas); com o lock numa subconsulta da mesma instrução, não vê (1 linha).
- Duas sessões que seguram o compartilhado e pedem o exclusivo da mesma chave terminam em `40P01`, com uma delas abortada.
- `hashtextextended(text, int8)` existe e devolve `int8`.

**Verificado na implementação.** Lock e `SELECT` no mesmo `pgx.Batch` recebem snapshots distintos em `READ COMMITTED`: a leitura que esperou o lock enxerga o que o detentor do exclusivo commitou. E o `ROLLBACK TO SAVEPOINT` libera o advisory lock tomado depois do `SAVEPOINT`: com o lock depois dele, o teste da guarda após a violação de unicidade reprova. Por isso a decisão 3 toma o lock antes do `SAVEPOINT`.

## Decisão

1. **Guarda declarada no `Table`.** `postgres.Table` ganha `Guard string`, opcional, a coluna cujo valor identifica a série lida como pré-condição. Quando não vazia, o `validate` exige identificador SQL presente em `Columns` e entra em pânico com mensagem que nomeia a tabela e a coluna, no estilo das demais validações. A tabela sem `Guard` não muda: SQL, idas ao servidor e erros idênticos aos do rc.5 e do commit `26d115fe`. O contexto só declara `Guard`, sem mudança de schema, e a role de runtime do ADR-063 não precisa de grant novo.

2. **Leitura na UoW sob lock compartilhado.** `Relation.QueryWithin(ctx, tx *Tx, value any) ([]S, error)` devolve o mesmo que `Query`, inclusive `ports.CrossTenantAccess` por `miss`. Toma `pg_advisory_xact_lock_shared(<chave>)` na `Tx` e só depois executa o `SELECT` e o `miss`, **em instruções separadas**: o snapshot é da instrução, e a leitura que esperou o lock precisa de um snapshot posterior à aquisição dele para ver o que o escritor commitou. Entra em pânico se a coluna do `Relation` não for a `Guard` da tabela, porque é erro de programação: leitura na UoW sem guarda daria aparência de consistência sem serializar nada. O `Relation.Query` no `ReadPool` segue como está, para a consulta fora da UoW.

3. **Escrita sob lock exclusivo, no choke point.** Com `Guard`, o `Save` toma `pg_advisory_xact_lock(<chave>)` com o valor codificado da coluna de guarda antes do `INSERT` ou do `UPDATE`. Toda escrita do kernel na tabela, `UPDATE` ou `INSERT` de supersessão, espera então o commit dos leitores, e o escritor não precisa lembrar de travar: o choke point do ADR-051 trava por ele. Com `Unique`, o lock entra no batch do ADR-066 **antes** do `SAVEPOINT dmpf_unique`, porque o `ROLLBACK TO SAVEPOINT` libera o advisory lock tomado depois dele, verificado na implementação; tomado antes, a guarda segue presa depois de uma violação de unicidade desfeita. Sem `Unique`, lock e escrita vão num batch de uma ida.

4. **Chave.** `hashtextextended(concat_ws(chr(31), '<tabela>', '<coluna>', length($tenant::text), $tenant::text, $valor::text), 0)`, em `int8`, calculada no servidor, com tabela e coluna literais compiladas uma vez por `Table`. Leitura e escrita produzem a mesma chave para o mesmo valor. O tenant entra na chave (IDN-11), precedido do seu comprimento, porque o `TenantID` só recusa o vazio e um ID com `chr(31)` chegaria, sem ele, à chave de outro tenant: um tenant nunca espera por outro, exceto por colisão de hash. Uma colisão causa só espera indevida, nunca leitura errada, e o espaço `int8` é dividido com `migrateLockID`, com probabilidade de colisão de 2^-64. Nenhum valor da chave entra em log, span ou mensagem de erro além do que o kernel já registra (ERR-07).

5. **Valor da guarda só `string`.** `QueryWithin` e o valor que o `Encode` produz para a coluna de guarda aceitam só o kind `string`, inclusive tipos nomeados `~string`. Outro kind volta erro sem tocar o banco: valores equivalentes com representações textuais distintas (UUID em caixas diferentes, `timestamptz` com offsets, decimal com escalas) dariam chaves diferentes, e duas decisões sobre a mesma série não se serializariam.

6. **Guarda imutável no `UPDATE`.** O `UPDATE` de uma tabela com `Guard` inclui `<guard> = $n` no `WHERE`, e trocar a coluna de guarda de um agregado existente volta `ports.ErrVersionConflict`. O lock exclusivo é o do valor a gravar: sem essa cláusula, a linha sairia da série de origem sem o lock da chave de origem, e os leitores dela não esperariam.

7. **Deadlock com guarda é `Conflict`.** A `Tx` registra que tomou uma guarda, compartilhada ou exclusiva. Daí em diante, o `40P01` em qualquer instrução de `Load`, `Save` ou `QueryWithin` dessa `Tx` volta embrulhado em `ports.ErrVersionConflict`, porque o ciclo pode vitimar o `UPDATE` e não o lock. `CategoryOf` o classifica como `Conflict`, e o gRPC o devolve como `Aborted`, «replay the call». O `40P01` da guarda significa que a decisão concorria com outra sobre a mesma série, e a repetição fica com o chamador, como manda UOW-10. Uma `Tx` que não tomou guarda mantém o erro atual.

8. **Garantia de ordem, não invariante permanente.** A pré-condição vale no commit do comando: a escrita concorrente na série espera esse commit e é serializada depois dele. Se a escrita posterior também precisa preservar o estado do outro agregado, ela tem a sua própria pré-condição, declarada pelo contexto.

9. **A UoW não muda.** Segue `READ COMMITTED`, com uma transação e sem retry automático (UOW-01, UOW-09, UOW-10, INB-18), e `Within` não ganha parâmetro nem opção. O `bind` entrega a `*Tx` ao reader da pré-condição, que a recebe como porta do callback (UOW-04). Por isso o ADR-034 não é emendado, e o `uow-inbox-outbox.md` não recebe emenda normativa: UOW-04 já enuncia a propriedade, e este ADR registra a realização do kernel.

10. **Instrução do contexto.** A regra do contexto que diz «Consultas correm fora da UoW.» passa a manter a regra para queries (UOW-11) e a mandar a pré-condição relacional de comando ler por `QueryWithin` dentro da UoW. O texto muda no template em `tools/dmpf-plugin/templates/ai/`, e a cópia é renderizada pelo `init`.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| `FOR SHARE` na série lida | Bloqueia o `UPDATE` da linha lida, mas não o `INSERT` de uma versão que a supersede: o fantasma passa. Exige o privilégio `UPDATE` na role, e uma role com só `SELECT` recebe `42501` |
| `SERIALIZABLE` por comando | Commita sem erro quando o escritor concorrente roda em `READ COMMITTED` (Partner `active` com capacidade `suspended`), porque a SSI só detecta o skew se todos os participantes forem `SERIALIZABLE`. Quebra INB-18 e exige retry, vedado como default por UOW-09 e UOW-10 |
| Advisory lock explícito no caso de uso | Depende de cada escritor da série lembrar de chamar o lock, e um esquecido reabre o skew em silêncio: o isolamento voltaria a depender de disciplina, o que o ADR-051 recusa |
| Lock e `SELECT` na mesma instrução | O snapshot é da instrução: o leitor que esperou o lock não vê a linha inserida pelo escritor (1 linha, contra 2 com instruções separadas) |
| Chave em dois `int4` | O hash de 32 bits pioraria as colisões entre guardas. A colisão só causa espera, mas a chave `int8` a torna desprezível |

## Consequências

**Positivas:**

- A pré-condição relacional de um comando fica serializada com toda escrita que possa mudá-la, `UPDATE` ou `INSERT` de supersessão, sem sair de `READ COMMITTED`, sem retry e sem tocar o contrato de `Within`.
- O escritor não depende de lembrar do lock: a declaração de `Guard` no `Table` o impõe no `Save`, e o consumidor só declara a coluna.
- Os leitores da mesma chave não esperam uns pelos outros (compartilhados são compatíveis), e tenants distintos nunca se esperam, exceto por colisão de hash.
- A tabela sem `Guard` não paga nada, e a role de runtime não ganha privilégio.
- O deadlock chega ao chamador como `Aborted`, com a mesma categoria que o kernel já devolve para conflito de versão, e não como erro sem categoria.
- A prova detecta a corrida: os testes ordenados (A→B e B→A, sincronizados por espera observada em `pg_locks`) falham sem a guarda, as 50 rodadas de estresse cobrem `UPDATE` e `INSERT`, e o mesmo cenário por `Relation.Query` no `ReadPool` reproduz o skew em ao menos uma rodada.

**Negativas:**

- Dois comandos que leem e escrevem a mesma chave em concorrência terminam em deadlock: ambos seguram o compartilhado e pedem o exclusivo. Um deles recebe `ErrVersionConflict` (`Aborted`) e o chamador repete. **Custo aceito:** o retry automático é vedado por UOW-09 e UOW-10, e o deadlock é a resposta correta para duas decisões que concorriam sobre a mesma série.
- A leitura paga o lock antes do `SELECT`, em instruções separadas do mesmo batch, sem ida a mais. Medido em escritas no socket, no modo padrão (`CacheStatement`) e com conexão quente: `QueryWithin` com linhas paga 1, vazio paga 2 (a consulta de dono, como hoje), e o `Save` com guarda paga 1, com ou sem `Unique`. Com `Unique`, o primeiro `Save` de cada escrita na conexão e os modos `CacheDescribe` e `DescribeExec` enviam lock e `SAVEPOINT` sozinhos antes do batch, como o `SAVEPOINT` já fazia; o lock é preparado por nome junto da escrita, para que um batch recusado não o tire do cache.
- A garantia é de ordem. O estado do outro agregado pode mudar depois do commit do comando, e quem precisa preservá-lo declara outra pré-condição (decisão 8). **Custo aceito:** é o que a serialização por chave entrega sem `SERIALIZABLE`.
- O valor da guarda é só `string`, e a guarda do agregado não muda depois de gravado. **Custo aceito:** a coluna de guarda é `text` e o contexto passa o valor já canônico, porque a chave é o texto enviado pelo Go e, em `uuid` ou `citext`, valores iguais com grafias diferentes tomariam locks diferentes; a troca de série é outro agregado.
- Não há `lock_timeout` por comando: a espera segue a configuração do pool do contexto, e o limite por comando é outra decisão. A transação que leu pela guarda bloqueia todo `Save` da série até o commit, o que a leitura MVCC fora da UoW não fazia: a duração da UoW passa a ser tempo de bloqueio da série, e o contexto que adota a guarda configura `lock_timeout` e `idle_in_transaction_session_timeout` no pool.
- Escritas em linhas diferentes da mesma série passam a correr em fila, do lock exclusivo até o commit, e não mais linha a linha; leituras pela guarda não esperam umas pelas outras. **Custo aceito:** é a serialização que fecha a corrida; o contexto mede o throughput de escrita em lote por série antes de adotar a guarda numa série quente.
- Cada valor distinto lido por `QueryWithin` ocupa uma entrada na tabela de locks compartilhada até o fim da transação, o que o lock de linha não fazia; uma transação que lê muitas séries esbarra em `max_locks_per_transaction` (`53200`) e deve ser fatiada.
- Com o contexto cancelado durante a espera, o pgx fecha a conexão, mas o backend só deixa a fila do lock quando percebe a queda; até lá, um escritor nessa fila retém os leitores que chegam depois. O lock de linha já se comportava assim, e `client_connection_check_interval` no servidor encurta a janela. A pré-condição negativa sobre o tenant inteiro, sem coluna de guarda, fica fora: o caso filtra sempre por um valor de chave.
- O `memory` não tem `Relation`, e a guarda não existe nele. **Custo aceito:** os contextos usam dublês nas portas próprias, e a prova fica no `postgres`; o `bookings` não tem pré-condição entre agregados e não exercita a guarda.
- O consumidor só adota a guarda depois da `1.0.0-rc.6`, que versiona os 21 projetos e não só o `postgres`, e a Nota (†) do antecipa-market segue valendo até lá. **Custo aceito:** o consumidor depende da versão publicada, não deste repositório.

## Referências

- [SPEC-HFDSHFR1](../specs/SPEC-HFDSHFR1-leitura-relacional-na-uow.md) (DEVS-132) e [SPEC-7SRQ3RPK](../specs/SPEC-7SRQ3RPK-unicidade-como-rejeicao.md), o batch com savepoint em que o lock entra.
- [ADR-034](./034-fronteira-de-uow-em-go.md), [ADR-051](./051-escopo-de-tenant-por-choke-point-em-go.md), [ADR-063](./063-duas-roles-de-banco-por-app.md) (role de runtime sem grant novo) e [ADR-066](./066-unicidade-declarada-como-rejeicao.md).
- `docs/dmpf/uow-inbox-outbox.md` — UOW-01, UOW-04, UOW-09, UOW-10, UOW-11 e INB-18.
- `docs/dmpf/contexto-erros-seguranca.md` — IDN-11 e ERR-07.
- `libs/backend/go/postgres/README.md` — `Guard`, `QueryWithin` e o padrão de deadlock, documentados na onda do kernel.
- antecipa-market SPEC-9TEQ9EV9, Nota (†) (DEVS-78) — consumidor que adota a guarda.
