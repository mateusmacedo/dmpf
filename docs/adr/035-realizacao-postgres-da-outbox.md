# ADR-035: Realizar a outbox em PostgreSQL com serialização na escrita e mapeamento por bounded context

## Status

Aceito — 2026-09-04. Implementa SPEC-3R80KNMS.

## Contexto

O FND-04 (`docs/dmpf/uow-inbox-outbox.md`) fixa a tabela de outbox em dezoito
campos (§4.1), os valores com que um registro nasce (`OBX-05`), a unicidade de
`message_id` como propriedade do schema (`OBX-01`) e a repartição de autoria
entre o application service e o provider (§2.3, `BLK-04`, `BLK-05`). O ADR-021
acrescenta que a serialização acontece **na escrita**: os bytes do fato congelam
no momento em que o evento entra na outbox, e a drenagem os transporta sem
reserializar.

O ADR-034 entregou a fronteira — `UnitOfWork[R]`, `Repository[ID, S]` e `Outbox`
em tipos de domínio — mas a única realização era em memória: uma fatia de
`OutboxEntry` sem bytes de wire, sem unicidade e sem estado de drenagem, que o
próprio `doc.go` declara não provar isolamento. A sequência canônica de nove
passos rodava, mas contra um par de mutexes.

Faltava, portanto, decidir o que a norma deixa ao provider: **o que exatamente
vai na coluna `payload`**, como o tempo atravessa a fronteira relacional, quem
escreve `available_at`, o que ocupa `metadata` antes de FND-07, e onde vive a
tradução do evento de domínio para o contrato Protobuf — já que o bloco `port`
não pode conhecer wire e o bloco `application` não pode conhecer codec.

Uma quarta força é do gate: as células 26 (`provider → application`) e 12
(`application → contract`) da RFC §7.3 passaram a ser alcançáveis com o módulo
novo, e o `tools/dmpf-gate-check.sh` não as decide — o `depguard` seleciona por
nome de diretório e não enxerga aresta entre unidades.

## Decisão

**`payload` guarda os bytes do `Any` do integration event, não o CloudEvent
completo.** O `envelope.Validate()` exige `correlationid`, `causationid` e
`traceparent` (`envelope.go:81-103`), atributos de contexto de FND-07 que nem
`OutboxEntry` nem o application service de referência autoram. Gravar o envelope
inteiro obrigaria o provider a inventá-los na escrita. O relay (KRN-08) monta o
CloudEvent na drenagem a partir das colunas — `message_id`→`id`,
`message_type`→`type`, `schema_version`→`dataschema`, `occurred_at`→`time`,
`partition_key`, `aggregate_version` — e de `metadata`. Os bytes do fato, que é
o que o ADR-021 protege, já estão congelados.

**`payload_hash` é a única coluna acrescida aos dezoito de §4.1.** A norma
permite ao provider acrescentar colunas ao schema concreto, e esta existe para
que "os bytes não mudam" seja verificável sem reserializar, como `ENV-18` exige.

**O tempo é `bigint` de nanossegundos, nunca `timestamptz`.** É a mesma decisão
que o ADR-034 tomou para `Instant` no bloco `port`, mantida na travessia
relacional: converter para tipo temporal do banco reintroduziria fuso, precisão
e arredondamento numa fronteira que a norma define como inteiro.

**`available_at` é gravado explicitamente pelo provider, igual a `occurred_at`,
com `CHECK (available_at >= occurred_at)`.** Um `DEFAULT` relacional esconderia
a regra de `OBX-05` no schema, onde nenhum teste da porta a alcança; o `CHECK`
mantém a invariante mesmo quando o relay passar a adiar entregas.

**`metadata` nasce `'{}'`, escrito pelo provider e não por `DEFAULT`.** O
conteúdo do contexto de mensagem é de FND-07; enquanto ele não existir, o
provider grava o objeto vazio em vez de inventar campos ou deixar nulo.

**O mapeamento evento de domínio → contrato Protobuf vive no provider, por
bounded context.** `EventMapper` é declarado no provider e realizado por
`orderspg.Mapper`. O bloco `port` não pode conhecer wire (`ADR-034`), e o bloco
`application` não pode importar `contract` — célula 12. O provider é o único
lugar onde as duas pontas já são visíveis. Um evento sem contrato registrado
devolve `ErrUnmappedEvent` e a transação inteira desfaz: o que ninguém sabe
publicar não é gravado.

**A conferência de major é feita na escrita, comparando o sufixo do pacote
Protobuf com o sufixo do envelope type.** As duas majors vêm de fontes
independentes — a do type é autorada pelo mapeador, a do pacote é do código
gerado — e sem essa comparação um mapeador poderia apontar mensagem `v1` para
type `v2`, publicando uma inconsistência que só o consumidor descobriria
(`ENV-16`).

**Testes de banco levam a build tag `integration` e rodam apenas no target
`test-race` do módulo.** Sem `DMPF_PG_DSN` o comportamento diverge por ambiente:
`t.Fatal` quando `CI` está definido, `t.Skip` com instrução do compose fora dele.
Um skip silencioso no CI deixaria a UoW e a outbox sem prova executável, que é
exatamente o que esta entrega existe para dar.

**No CI, o Postgres sobe por `docker run` compartilhando o namespace de rede do
job, não pelo bloco `services:`.** A primeira tentativa usou `services:`, e o
primeiro push mostrou por que ele não serve aqui: o `act_runner` cria o serviço e
o job na **bridge default** do Docker, que não faz resolução DNS por nome de
container, e o próprio log declara que ignora rede customizada
(`--network and --net in the options will be ignored`). O serviço subia, mas
`lookup postgres` falhava. Publicar a porta e mirar o gateway da bridge
resolveria, ao custo de um IP que varia por runner. Como o job monta
`/var/run/docker.sock`, subir o container com `--network container:$(hostname)`
o coloca no mesmo namespace de rede do job: alcançável em `localhost:5432`, sem
DNS, sem IP fixo e sem porta publicada. O nome do container carrega o id do job
pelo mesmo motivo de o socket vir do host: um nome fixo colidiria entre jobs
concorrentes na mesma máquina, ou com o resíduo de um job morto sem limpeza.

**O target `test-race` roda com `cache: false` no Nx e `-count=1` no `go test`.**
São dois caches distintos e ambos devolvem resultado antigo para teste de banco:
o do Nx ignora que o banco mudou, e o do `go test` também — ele indexa binário,
argumentos e variáveis consultadas, nunca o estado do Postgres. Este é o único
módulo do workspace com teste de banco, e por isso o único com as duas guardas.

**O piso e o toolchain do Go sobem de `1.26.4` para `1.26.6`, no `go.work` e nos
seis `go.mod`.** Não é higiene oportunista: o `govulncheck` reprova este módulo
com quatro CVEs da biblioteca padrão — GO-2026-6090 e GO-2026-5856
(`crypto/tls`), GO-2026-6088 (`encoding/xml`) e GO-2026-5972 (`encoding/asn1`) —
alcançadas pelo caminho `pgxpool.Pool.Exec` → `tls.Conn.Read`. Este é o primeiro
módulo do workspace que chega a `crypto/tls`, e é por isso que os outros cinco
seguiam limpos com o mesmo toolchain. Como `1.26.6` é patch da mesma minor, o
bump não move o piso de linguagem de ninguém. As fixtures sintéticas sob
`internal/golist/testdata/` ficam em `1.26.4`: são módulos de teste fora do
universo do verificador, e piso menor continua válido.

**As células 26 e 12 são provadas por `tools/dmpf-cell-check.sh`, sobre um
`git worktree` descartável.** O fixture proibido nunca entra na árvore de
trabalho, onde apareceria para o editor, para o `go build` local e para os
outros gates rodando em paralelo. Ele também não é adicionado ao git de
propósito: o verificador inventaria unidades por `git ls-files` e lê imports por
`go list`, então um arquivo não rastreado é visto na aresta sem alterar a
inventariação.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Serializar o `cloudeventsv1.CloudEvent` inteiro na coluna `payload` | Exigiria atributos de contexto que ninguém autora nesta fase e faria o provider inventar `correlationid`, `causationid` e `traceparent` na escrita. |
| `timestamptz` para `occurred_at` e `available_at` | Reintroduz fuso e arredondamento na fronteira que FND-04 §4.2 define como inteiro, e divergiria de `Instant` no bloco `port`. |
| `available_at` por `DEFAULT` do schema | Esconde a regra de `OBX-05` onde teste de porta nenhum a alcança, e some quando o relay passar a adiar entregas. |
| Mapeador no bloco `application`, injetado no provider | Faria o application service importar `contract` — célula 12, a mesma aresta que este ADR manda o gate reprovar. |
| Detectar duplicata por `SELECT` antes do `INSERT` | Duas transações concorrentes passariam no `SELECT` e colidiriam no `INSERT`: a unicidade tem de ser do schema, como `OBX-01` determina, e o SQLSTATE `23505` é a prova. |
| Testes de banco sem build tag, como a spec original previa | Colocaria teste que exige Postgres no `go test ./...` comum, contrariando `.claude/rules/testing-conventions.md` e quebrando o `pre-push` de quem não tem o compose no ar. |
| `testcontainers-go` no lugar do Postgres do CI | Acrescenta dependência pesada a um módulo cujo `go.mod` tem duas entradas. O primeiro push confirmou que o `gitea-runner` tem Docker, então o impedimento restante é o peso, não a capacidade. |
| Manter `services:` e mirar o gateway da bridge com a porta publicada | Funcionaria, mas amarra o DSN a um IP de gateway que varia por runner e por configuração de rede do host. |
| Vetores de célula como fixtures sintéticas sob `testdata/` | Tocaria o `dmpf-conformance`, fora do escopo desta história, e provaria a regra genérica que o verificador já cobre em vez das células que este módulo torna alcançáveis. |
| Provar as células com fixture na árvore de trabalho, como o `dmpf-gate-check.sh` faz | Lá o fixture só precisa sobreviver ao `depguard`; aqui ele precisa sobreviver a um `go list` sobre o módulo inteiro, e um `.go` proibido visível na árvore real quebraria o build de quem trabalha em paralelo. |
| Confiar só no `cache: false` do Nx para o `test-race` | São dois caches: o do `go test` também devolveria resultado antigo, porque indexa binário e variáveis consultadas, nunca o estado do banco. |

## Consequências

**Positivas:**

- O KRN-07 recebe a UoW já provada contra banco real e escreve a inbox na mesma
  transação, sem redecidir schema nem serialização.
- O KRN-08 recebe registros que nascem `pending`, com `available_at` ancorado e
  bytes congelados, e implementa claim, lease, `SKIP LOCKED` e publicação lendo
  colunas — nunca reserializando.
- O KRN-10 recebe `destination` como nome de fluxo lógico, com a forma imposta
  na escrita, e traduz para alvo físico sem reinterpretar o que foi gravado.
- A célula 26 fica provada por vetor executável, fechando uma lacuna que o
  ADR-034 declarou: lá a técnica esbarrava no ciclo de imports, que cai antes da
  regra de bloco. Com o provider num módulo próprio, que ninguém importa, a
  aresta proibida chega ao verificador como aresta de bloco e reprova com
  `DMPF-D001`.

**Negativas:**

- **Custo aceito:** o workspace passa a ter um módulo cujo teste exige
  infraestrutura. O job `main` do CI sobe `services.postgres`, o `pre-push`
  local pula os testes de integração de quem não subiu o compose, e o
  `test-race` deste módulo não é cacheável em nenhuma das duas camadas.
- O `payload_hash` é redundante enquanto ninguém o lê: quem passa a conferi-lo
  na drenagem é o KRN-08. Ele existe agora porque o critério "os bytes não
  mudam" precisa ser verificável nesta entrega, não na próxima.
- A prova de commit falho depende de cancelar o `ctx` pai dentro do callback,
  porque um banco real não tem falha de commit injetável como a realização em
  memória tem. É o gesto disponível, não o ideal.
- **Consequência declarada, não decidida:** enquanto a marca
  `contracts-baseline/proto` não existir no remote, o gate `buf-breaking`
  reprova por construção — o `tools/buf-gate.sh` trata a ausência como não
  verificada, e não verificada reprova. A criação da marca é ato de uma segunda
  pessoa (ADR-033) e não é resolvida por esta entrega.

Nada aqui declara ou sugere entrega exactly-once fim a fim. A garantia é
at-least-once, e a deduplicação é do consumidor, via inbox (KRN-07).

## Referências

- `docs/specs/SPEC-3R80KNMS-dmpf-outbox-postgres.md` — spec desta entrega.
- `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §2.3, §3.1 a §3.3, §4.1, §4.2.
- `docs/adr/021-*.md` — serialização na escrita.
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira que este ADR realiza.
- `docs/adr/033-*.md` — marca de baseline dos contratos (`BUF-08`).
- `docs/guides/dmpf-manifesto.md` — `external[]` e classificação de unidades.
- `libs/backend/go/dmpf-provider-postgres/README.md` — como rodar localmente.
