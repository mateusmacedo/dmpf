# ADR-040: Realizar os test kits, o carregador de golden fixture e a fitness function em Go como um módulo de onze unidades classificadas por bloco

## Status

Aceito — 2026-09-07. Implementa SPEC-SJ66880S.

## Contexto

O FND-09 (`docs/dmpf/testes-interop.md`) fixou a forma da prova do kernel — a
pirâmide de cinco camadas com um kit por camada e critério de aprovação
decidível (`KIT-01` a `KIT-06`), o determinismo de toda suíte (`KIT-07`,
`KIT-08`), o formato da golden fixture com todo escalar como string (`FIX-05` a
`FIX-13`), os três oráculos do round-trip reportados em separado (`ORA-01` a
`ORA-07`), a projeção observável do desfecho de domínio (`ORA-30` a `ORA-40`),
a regra de dependência como fitness function na suíte (`FIT-01` a `FIT-04`) e o
CI em estágios com a camada distribuída em pipeline separado (`KIT-09` a
`KIT-11`) — e parou aí de propósito: a realização é dos épicos de kernel.

`KRN-02` a `KRN-10` entregaram o kernel Go com testes por módulo, mas sem o
instrumento: cada módulo tinha o seu relógio fake, o seu gerador de
identificadores e o seu harness; a suíte de contrato de `UnitOfWork` e de
`Inbox` vivia em `_test.go` de `ports`, duplicada à mão pelas realizações
em memória e Postgres porque um `_test.go` não é importável; o oráculo 3 do
round-trip era um `t.Log` informativo; e as 36 células da matriz eram provadas
no oráculo interno do `decide` do verificador, não por par de vetores
executável fora dele.

Restrições herdadas: o verificador trata todo package não-teste como produção
(`FIT-01`), então um kit é unidade do universo e precisa de bloco; os blocos
`domain` e `contract` não podem importar `testing`, `os` nem `time`
(capabilities `observability`, `io.filesystem`, `io.clock` fora da allowlist);
`provider` não alcança `application` nem `app` (células 26 e 27); a stack
TypeScript está fora do ticket; e o CI corre em `act_runner`, onde o bloco
`services:` não resolve DNS (ADR-035).

## Decisão

**Um módulo, onze unidades, quatro blocos.** `testkit` declara uma unidade
por package, cada uma no bloco que as arestas daquele package permitem:
`domainkit` é `domain`; `golden` é `contract`; `serviceskit`, `providerkit`,
`clock`, `ids` e `stable` são `provider`; `appkit`, `distkit`, `fitness` e `tb`
são `app`. Um kit único de bloco `app` não provaria nada sobre a camada que
certifica — um `domainkit` só é kit de domínio se ele próprio for `domain`
(célula 1). O precedente é `kernel/example-memory`, unidade `provider`
dentro de `application`.

**Veredicto por valor; `testing.TB` é adaptador.** Cada kit devolve uma lista
de diagnósticos com a regra violada, vazia no passe; `tb.Require` a converte em
`t.Errorf`. `KIT-01` exige critério decidível, e `domain` e `contract` não podem
importar `testing`. A interface `Verdict` do `tb` é estrutural
(`Failures() []string`) porque cada kit mantém o próprio tipo — `domainkit` não
pode importar `tb` (domain → app) e `golden` não pode importar `domainkit`
(contract → domain).

**Package exportado `fitness` no `conformance`, sem mover `internal/`.**
Expõe `Workspace`, `Units`, `Diagnostics`, `Diagnose`, `Graph` e
`StandardCapability`, com aliases dos tipos e dos dezesseis códigos, e omite o
baseline e o `--base` — a fronteira de `FIT-03`. É unidade `app` com
`public_integration_surface: true`, porque o kit é `bounded_context:
kernel` e o verificador é `conformance`: sem a superfície pública a
aresta reprovaria por `DMPF-D002`.

**As duas direções do round-trip no lado Go.** O consumidor lê os bytes
(oráculos 2 e 1, e a identidade de `Any.value` após envelopar e desenvelopar
como oráculo 3); o produtor escreve a partir dos campos declarados (oráculos 3 e
2) e o oráculo 3 **reprova**, porque a direção produtor é o caso de `ENV-24`.
O `Report` é a evidência que `KRN-12` consome. O contracts segue dono das
fixtures de wire e do gerador; o kit é dono do oráculo (FND-05 §8.4). Um
round-trip Go → Go não é o cross-stack de FND-09 §8.4, e este ADR não o chama
assim.

**Fixtures de projeção observável em `contracts/fixtures/<ctx>/projection/v1/`.**
A projeção é a fonte única que o kernel TypeScript consumirá (`ORA-35`); o codec
vive em `tb`, não em `domainkit`, porque `encoding/json` é `wire.codec`. O
`domainkit` recebe a UPR, a projeção e o clone por valor (`Subject[S,R]`) e
detecta o segundo acessor de eventos pela forma (`ORA-37`) e a mutação sob
`Rejected` pela leitura dupla do estado (`ORA-38`).

**Os testes que precisam de duplo se movem; o SUT não.** O vetor `V29`/`V30`
do kit reprovou duas unidades `domain` do próprio verificador — `internal/rule`
(o duplo `memGraph` realizava a porta) e `internal/baseline` (os testes rodavam
`git`). Pelo `PIR-17`, os testes foram reclassificados: os vetores com duplo
foram para `internal/conformance` (application) e o teste de repositório para
`internal/fsstore` (provider); as unidades continuam `domain`. O mesmo vetor
reprovou depois o próprio kit, quando o pool Postgres em `tb` arrastou o
provider para o fechamento do teste de domínio; o pool foi para `tb/pg`.

**Suítes de conformidade exportadas, duplicatas removidas.**
`providerkit.UnitOfWork`, `Inbox` e `Outbox` substituem os `_contract_test.go`
de `ports`, de `example/memory` e do exemplo Postgres de reservas;
`ports` não passa a depender do kit — é o bloco mais baixo. `OutboxStore`
é genérico em `Claimed` e estrutural, porque `provider` não importa
`app/relay`. Cláusula que um candidato não consegue exercitar vai para
`Skipped`, nunca fica ausente em silêncio.

**Processos OS reais sobre Redpanda no harness distribuído.** `PIR-14` exige
dois ou mais processos; o `distkit` re-executa `os.Executable()` com
`-test.run=^TestDistkitRole$` e argumentos explícitos, o papel numa variável de
ambiente lida antes de qualquer montagem, e o transporte com o relógio real —
um fake no processo consumidor congelaria timers que só outro processo poderia
avançar. O `Sink` devolve erro só quando o adapter não dispôs da entrega: o
worker do Kafka aplica a decisão do `Acknowledger`, não o erro.

**Estágios como steps ordenados de um job; a camada distribuída em workflow
próprio.** O `act_runner` não resolve `services:`, e jobs separados subiriam a
infra por job. A seleção é por uma quarta tag Nx, `layer:*`, aditiva às três
tags de taxonomia; a lista de projetos afetados por camada é lida de
`nx show projects --affected --projects=tag:layer:<x>`, que emite um array
JSON.

**O code review em duas frentes mudou o instrumento em seis pontos.** (1) Um
oráculo que o consumidor não consegue alcançar — envelope malformado, viagem
que falha — é reportado como `not evaluated` e reprova; antes o `Report` o
fabricava como passe por omissão. (2) A corrida de duas inserções da suíte de
inbox sinaliza quando o `Register` **retorna** e o vencedor só completa depois
dos dois retornarem: é a única forma de garantir a sobreposição que `INB-06`
prova, e uma realização *check-then-insert* de fixture é reprovada por ela;
uma realização sem teto de espera nunca devolve o perdedor, e um prazo de
sessenta segundos nomeia isso em vez de travar. (3) O ledger do `serviceskit`
carrega a identidade da transação em cada gesto, então uma porta que escapou de
uma transação e é usada em outra é nomeada — ela escreve numa cópia descartada
(`memory/tx.go`); um caso de uso commita exatamente uma vez, vazio sob recusa,
porque rollback ou ausência de `begin` seriam indistinguíveis de falha técnica
(`UOW-06`, rationale). (4) A suíte de outbox compara o estado **inteiro** do
registro antes e depois de cada transição por claim substituído (`OBX-11`), e
`Pending` e `Purge` são obrigatórios — purga e sinais são parte do contrato que
`KIT-04` certifica, não capacidade opcional. (5) `OBX-11` é sobre
**substituição**, não expiração: um claim vencido sem sucessor ainda escreve
(`docs/dmpf/uow-inbox-outbox.md`), e a spec, o README e o godoc dizem isso.
(6) No verificador, o segundo perfil de build declara as tags `integration` e
`distributed`, porque os harnesses do kit são os primeiros arquivos de
produção atrás de build tag do workspace e sem o perfil ficavam fora do
universo; e o `V29`/`V30` nega por omissão qualquer dependência de terceiro em
teste de unidade `domain`, como `DMPF-E001` faz para produção.

**O CI ganhou uma guarda e mudou a ordem dos gates.** Um step compara
`tag:stack:go` com a união das cinco camadas e falha se algum módulo Go ficar
sem `layer:*` ou em duas camadas; os gates DMPF correm depois dos estágios 1 e
2 e antes da infraestrutura, porque decidem sobre a árvore; os gates Buf fecham
o estágio 3, como `KIT-09` pede, sem mais suprimir o gate de dependência quando
falham.

## Alternativas descartadas

- **Um módulo por camada** (`testkit-domain`, …): cinco `go.mod` sem
  ganho sobre a unidade por package.
- **Kits recebendo `*testing.T`**: amarraria o contrato ao framework e violaria
  a capability de `domain` e `contract`.
- **Invocar o binário do verificador por subprocesso**: não reusa `decide` nem
  os tipos, e exige `--base` — o gate e a fitness function têm contratos
  diferentes.
- **Fundar o primeiro projeto TypeScript aqui**: o ticket o exclui, e um
  consumidor TS de fixture sem kernel TS seria código sem dono.
- **Duas goroutines com clientes Kafka distintos** no lugar de dois processos:
  compartilham memória e não provam o que `V32` prova.
- **floci/SQS como broker do `KIT-06`**: a reentrega em SQS depende do timeout
  de visibilidade e tornaria o harness lento e dependente de relógio.
- **Um target por camada** (`test-domain`, …): duplicaria `test-race` em todos
  os módulos.
- **Exceções declaradas no vetor `V29`/`V30`** para as unidades do verificador
  que reprovaram: registraria a violação em vez de repará-la pelo `PIR-17`.
- **Estender a varredura de `V31` a `docs/`**: cada menção ao termo no acervo e
  nos ADRs é uma negação em prosa livre («não entrega exactly-once», «nada aqui
  declara ou sugere…»), e uma varredura por expressão regular sobre prosa
  viraria uma lista de exceções sem fim. `RAS-12` nomeia contratos, READMEs e
  configuração, e é isso que a varredura cobre.
- **Um package de apoio para os helpers de teste do verificador** (`codigos`,
  `exigeCodigos`, duplicados entre `rule_test` e `conformance_test`): seria uma
  unidade do universo com bloco e baseline próprios, só para dois helpers de
  quinze linhas. A duplicação fica, declarada.

## Consequências

**Positivas:**

- Cada regra de FND-09 que o ticket cobre tem vetor positivo e negativo
  executável: as 36 células por par de universos, os cinco kits com fixture não
  conforme reprovada, os três oráculos falhando um a um, `V32` com dois
  processos reais e o consumidor ingênuo reprovado com `DMPF-R004`.
- O vetor `V29`/`V30` já pagou por si: encontrou duplos de infraestrutura em
  testes de duas unidades `domain` do verificador e, depois, no próprio kit.
- As três realizações de `UnitOfWork`/`Inbox` (`ports` fakes, memória,
  Postgres) deixaram de duplicar a suíte; a corrida de duas inserções passou a
  correr de verdade contra o Postgres.
- Os estágios do CI deixam uma regressão de domínio reprovar sem subir Postgres;
  o `dmpf-distributed.yml` isola o custo do harness de reentrega.

**Negativas:**

- **Custo aceito:** o kit depende de `google.golang.org/protobuf` (`wire.codec`),
  `pgx` (`io.storage`) e `franz-go`/`kadm` (`io.messaging`), todos declarados
  em `external[]` do manifesto com faixa de versão, embora só os packages `app`
  alcancem os dois últimos e o verificador não exija a declaração em bloco
  permissivo: sem ela, a dependência entraria no workspace sem capability nem
  faixa, ao contrário dos módulos irmãos.
- **Custo aceito:** o `test-race` do `postgres` roda duas vezes
  no `ci.yml` — no estágio 4, junto do kit, e de novo no estágio 5, porque o
  `dependsOn` do `app` o arrasta e o Nx não deduplica entre invocações
  distintas de um target sem cache (~20 s). Fundir os dois estágios numa só
  invocação eliminaria a repetição, mas apagaria a fronteira que `KIT-09` exige
  entre providers e apps; remover o `dependsOn` reabriria a decisão do KRN-07
  sobre o Postgres compartilhado. O estágio 4 separa quem exige a infra do job
  (`test-race` com `cache: false`, `--parallel=1`) de quem não exige
  (`--parallel=3`), que é onde o paralelismo perdido realmente estava.
- `InboxSubject.Within` recebe o `context.Context` da suíte: a corrida de duas
  inserções passa o contexto do seu próprio prazo, para que uma realização sem
  teto de espera tenha a transação cancelada quando o prazo vence, em vez de
  deixar goroutines e um lock na chave vivos depois do veredicto. `clock.Fake.Set`
  para trás com timer pendente entra em `panic` nomeando a violação: descartar o
  timer em silêncio deixava quem esperava por ele pendurado sem diagnóstico.
- `tb/pg.OpenPool` só aceita DSN de host loopback: o reset das tabelas é
  destrutivo, e um Postgres compartilhado nunca é fixture de teste. As actions
  dos workflows e as imagens dos serviços do CI ficam presas por SHA e digest,
  porque os steps alcançam o socket do Docker do host.
- O `depguard` não alcança `testkit/domainkit` (a regra `domain` seleciona
  por `**/*-domain/**`); o verificador e o teste de capability do kit são as
  linhas de defesa. Estender o glob fica registrado, não incluído.
- A cláusula de falha de commit da suíte de UoW não é exercitável contra o
  Postgres (pgx não injeta a falha) e fica declarada em `Skipped`.
- O grafo de projetos do Nx conta imports de `_test.go`: `application`,
  `contracts` e `postgres` passam a apontar para o kit, e o
  kit aponta para `app`, `application` e `postgres` em
  produção. Com `dependsOn: ^build` dos `targetDefaults` isso fecha um ciclo de
  tasks que o compilador Go não vê. O `build` do kit declara `dependsOn: []` —
  `go build` resolve os irmãos do `go.work` por fonte — e as arestas dos
  consumidores para o kit ficam, para que uma mudança no kit os marque como
  afetados no CI.
- O critério de aceite da spec que diz "reprova só no oráculo 3 quando o hash
  foi gravado sobre os bytes divergentes" contradiz o requisito e o cenário da
  própria spec (produtor reprova nos oráculos 3 e 2); a realização seguiu o
  requisito, e a linha do critério merece ajuste na spec.
