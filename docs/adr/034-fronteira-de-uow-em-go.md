# ADR-034: Realizar a fronteira de Unit of Work em Go como `UnitOfWork[R]` com vínculo no composition root

## Status

Aceito — 2026-09-03. Implementa SPEC-ZHE7DN1H.

## Contexto

O FND-04 (`docs/dmpf/uow-inbox-outbox.md`) fixa a Unit of Work como a fronteira
de aplicação que envolve **uma** transação local e entrega ao callback as portas
a ela vinculadas (§3.1, `UOW-01`..`UOW-04`), a sequência canônica de nove passos
com 6, 7 e 8 na mesma transação (§3.2, `UOW-05`..`UOW-08`), a ausência de retry
automático (§3.3, `UOW-09`, `UOW-10`) e a autoria dos campos da outbox pelo
application service (§2.3, `BLK-04`, `BLK-05`).

O ADR-032 entregou a metade de dentro: o `dmpf-domain-go` devolve a `Decision`
como par `(Accepted[R], *Rejection)`. Mas ninguém a consumia. A norma que
descreve o consumidor não tinha realização, e sem ela o `KRN-06` (outbox
Postgres), o `KRN-07` (inbox) e o `KRN-09` (retry por conjunção) não teriam o
que estender, e a primeira contraprova de FND-04 §9.4 seguiria sem vetor
executável.

Quatro forças restringiram o desenho, todas verificadas no próprio terreno.

A primeira é a **tipagem dos recursos**. `UOW-03` exige que as portas
transacionais cheguem ao callback como argumento tipado, e `UOW-04` que **apenas**
as portas vinculadas cheguem. Só o compilador pode garantir as duas, e para isso
o tipo dos recursos precisa ser do caso de uso — mas um provider não pode
conhecê-lo, porque `provider → application` é célula proibida da matriz (RFC
§7.3).

A segunda é a **ocupação do canal de erro**. No serviço de aplicação, `error` já
transporta conflito de versão, falha de commit e cancelamento. O ADR-018 admite
o par `(valor, error)` de Go apenas quando o canal de erro carrega
exclusivamente rejeições de domínio, o que é verdade na UPR e deixa de ser aqui.

A terceira é a **política de capabilities**. O verificador classifica o package
`time` inteiro como `io.clock` (`internal/rule/stdlib.go`) e a política admite
só `pure` nos blocos `port` e `application` (`internal/rule/capability.go`).
Nenhum dos dois pode importar `time`.

A quarta é a **resolução entre módulos irmãos**. Verificado empiricamente em
2026-09-03: em modo workspace, `go vet`, `go build` e `go test` resolvem o
módulo irmão pelo `go.work` sem `require`, mas `go mod tidy` tenta a rede para
resolvê-lo — e com módulo ainda sem tag isso falharia ou gravaria pseudo-versão
remota divergente da árvore local.

## Decisão

**A fronteira é genérica sobre o tipo de recursos, e o vínculo é do composition
root.** `UnitOfWork[R any]` tem um método,
`Within(ctx, fn func(ctx, resources R) error) error`. `R` é o tipo que o caso de
uso declara. Quem monta `R` a partir da transação aberta é uma função `bind`,
escrita por quem conhece os dois lados: o composition root (bloco `app`; nos
testes, o próprio arquivo de teste). A realização recebe o `bind` e nunca vê o
tipo. Isso resolve `UOW-03` e `UOW-04` no compilador, sem que o provider precise
importar o application.

**O contrato de `Within` é enunciado como suíte executável.** Seis cláusulas:
uma transação sobre um recurso; `ctx.Err()` não nulo antes de abrir devolve o
erro sem invocar `fn`; `fn` invocada exatamente uma vez e nunca repetida; `fn`
devolvendo `nil` commita e devolvendo erro faz rollback com o mesmo erro, sem
embrulho que quebre `errors.Is`; erro de commit devolvido como o provider o
produziu, sem persistir nada; panic em `fn` propaga após rollback (`ERR-22`). A
sétima — `R` como único caminho até as portas — é estrutural. A suíte
`RunUnitOfWorkContract` vive em arquivo `_test.go` do `dmpf-ports`; como arquivo
de teste nunca é importável, a realização em memória **duplica** o corpo. Um
test kit exportado é do `KRN-11`.

**O desfecho de aplicação é `Outcome[R]`, separado de `error`.** Todo caso de
uso de escrita devolve `(Outcome[R], error)`: `error` transporta apenas falha
técnica, e a rejeição de negócio viaja no `Outcome` com `error == nil`. Assim a
recusa nunca compete com a falha técnica pelo mesmo `if err != nil`, e a
exaustividade de `DEC-01` chega ao adapter. `Rejected(nil)` produz panic:
ausência de rejeição não é um terceiro desfecho, e um `Outcome` nesse estado se
leria como aceito.

**A identidade é resolvida no passo 2, com a contagem declarada pelo caso de
uso.** `ResolveIdentity(clock, ids, events)` lê o relógio uma vez e obtém
`events` identificadores, antes de `Within` abrir a transação, porque uma
reexecução geraria identidade nova para o mesmo fato (`UOW-09` rationale). O
caso de uso sabe quantos eventos seu comando pode produzir; o kernel não
adivinha. Produzir mais do que o declarado é defeito de programação e provoca
panic que nomeia a causa.

**`Instant` e `MessageID` são valores de porta, não tipos de biblioteca.**
`Instant` é inteiro de nanossegundos desde a época Unix — nanossegundos porque
`available_at` ancora em `occurred_at` (FND-04 §4.2) e a drenagem ordena por
ele, e segundos colidiriam sob carga. Ler o relógio e obter entropia são I/O, e
por isso `Clock` e `IDGenerator` são portas, nunca tipos do domínio (ADR-014,
ADR-016).

**A realização em memória é unidade `provider` dentro do `dmpf-application`, com
falha de commit injetável.** Não é módulo à parte, o que traria manifesto,
baseline, `package.json`, `project.json` e entrada no `go.work` só para um
exemplo; nem código apenas de teste, que o `KRN-07` e o `KRN-09` não poderiam
reusar e que não exercitaria a célula 28 (`provider → port`) no verificador.
`FailNextCommit` é o que permite provar `UOW-07` sem banco.

**O `Store` guarda dois mutexes.** `txMu` serializa transações e é retido
durante todo o callback, que é o que faz uma `Within` excluir a outra; `dataMu`
protege o estado e é adquirido por operação. Um mutex só para os dois papéis
transformava `Store.Reader()` chamado de dentro do callback em deadlock
permanente, e cancelar o contexto não ajudaria porque `sync.Mutex` não observa
cancelamento. Com a separação, essa leitura devolve o estado commitado — o que
um leitor fora da transação veria. `ctx.Err()` é revalidado **depois** de
adquirir `txMu`, porque é ali que a transação de fato abre e quem esperou na
fila pode ter sido cancelado nesse intervalo.

**O commit instala uma cópia, não o mapa da transação.** Instalar o próprio mapa
da `Tx` deixaria as portas entregues ao callback apontando para o estado
commitado: uma porta guardada além do callback passaria a escrever fora de
qualquer transação e sem `dataMu`. Com a cópia, a porta que escapa escreve no
que foi descartado.

**O repositório é tipado sobre o estado persistido, não sobre o agregado.** A
UPR muta o receptor no aceite (`*o = next`), e um repositório que compartilhasse
o ponteiro com o `Store` faria a mutação vazar antes do commit, destruindo a
prova de atomicidade. O custo é `FromSnapshot` no domínio: construtor puro, sem
UPR, sem evento e sem mudança de classificação.

**`Save` antes de `Enqueue`.** `AggregateVersion` é a versão gravada, e o
conflito de versão precisa interromper antes de existir qualquer registro de
outbox na transação — mesmo que o rollback o apagasse, a ordem torna a prova de
`UOW-09` legível: sob conflito, `Enqueue` nunca é chamado.

**Sem `require` de irmão; a resolução é do `go.work`.** O `go.mod` dos dois
módulos não tem `require` nem `replace`, e não há `go.sum`. Por consequência, o
target `tidy` que o plugin do Nx infere **não** faz parte da cadeia de validação
e não é invocado pelo CI. Consumo fora do workspace, com tag por módulo e
`require` versionado, é matéria do `KRN-12`.

**O gate local não restringe o bloco `provider`.** A regra `application` do
`depguard` seleciona por caminho (`**/*-application/**`) e alcançaria o
subpackage `provider` dentro do módulo. O verificador, gate autoritativo, deixa
`provider` irrestrito, porque quem realiza a porta precisa do I/O que os blocos
de cima não podem ter. O subpackage é portanto excluído do `depguard`, e o
`tools/dmpf-gate-check.sh` o declara como fora do gate local em vez de
exercitá-lo.

## Alternativas descartadas

**`Within(ctx, func(ctx, tx Transaction) error)` com `tx.Repository("orders")`
por nome.** É service locator dentro da fronteira, que `UOW-03` veda
nominalmente: o compilador deixaria de garantir quais portas o callback recebeu.

**Recursos em `context.Context`.** Vedado por `CTX-05`, e destrói a garantia de
`UOW-04`: qualquer código no caminho poderia extrair uma porta não vinculada.

**`(R, error)` com `*Rejection` viajando como `error`.** Só seria admissível se
o canal de erro carregasse exclusivamente rejeições, o que não é o caso no
serviço de aplicação — ver a segunda força do Contexto.

**Gerar `message_id` no passo 7, por evento.** Mantém a identidade dentro da
transação e reabre a porta que `UOW-09` fechou: uma reexecução mintaria
identidade nova para o mesmo fato.

**`time.Time` no domínio, com `depguard` distinguindo por símbolo.** O gate
autoritativo é o verificador, que decide por package; a SPEC-XF9TF9A0 já
registrou essa divergência entre a documentação e o código do `KRN-02`.

**`Order.Clone()` exportado**, em vez de `FromSnapshot`. Expõe mecanismo interno
do agregado e não resolve a reconstituição a partir do que o provider guarda.

**Liberar o mutex durante o callback**, para permitir transações concorrentes.
Introduziria isolamento e conflito de serialização real na realização em
memória, que é justamente o que ela declara não provar e o que o `KRN-06`
entrega.

## Consequências

O `KRN-06` recebe a porta da outbox e a fronteira de UoW já fixadas em tipos de
domínio, e implementa só o provider. O `KRN-07` reusa a UoW para inbox e efeitos
locais na mesma transação. O `KRN-09` recebe a política de retry como ponto
explícito do caso de uso, não como decorator da fronteira. O verificador passa a
ter unidades `port` e `application` reais para exercitar as células 10, 11, 19,
22 e 23 sobre código do kernel.

O gate de dependência local cobre três blocos, com um array de vetores por
bloco: um array único inverteria o resultado, porque `time` é permitido como
import no `domain` — onde a distinção de símbolo fica com o `forbidigo` — e
`log` é permitido em `application`, cuja capability inclui observability.

Uma limitação da técnica de vetor sobre módulos reais ficou registrada: as
arestas `port → provider`, `port → application` e `domain → port` apontam na
direção contrária a arestas de produção que já existem, então o Go recusa por
ciclo de imports antes de o verificador avaliar o bloco, e o diagnóstico é
`DMPF-E003` em vez de `DMPF-D001`. Essas células continuam provadas, e melhor,
pelo oráculo de 36 células do `KRN-02`, que usa módulos sintéticos sem aresta
contrária. Um vetor de módulo para a célula 12 (`application → contract`) exige
uma unidade `contract` no `develop` e fica para o `KRN-06`.

A vedação a entrega exatamente-uma-vez fim a fim (P0-3, `GAR-01`) permanece: os
dois módulos declaram at-least-once com efeitos idempotentes, e nenhum artefato
promete o contrário.

## Referências

- `docs/specs/SPEC-ZHE7DN1H-dmpf-kernel-aplicacao-go.md` — spec do `KRN-04`
- `docs/specs/SPEC-YRJRADY9-dmpf-kernel-sdk-go.md` — guarda-chuva do kernel Go
- `docs/adr/032-realizacao-go-do-desfecho-da-upr.md` — a metade de dentro, do `KRN-03`
- `docs/adr/031-verificador-de-conformidade-dmpf-em-go.md` — o gate autoritativo
- `docs/adr/030-granularidade-modulo-go-e-bom.md` — um módulo Go por lib, caminho por stack
- `docs/adr/021-mapeamento-no-provider-serializacao-na-escrita.md` — a outbox recebe evento de domínio
- `docs/adr/018-forma-do-desfecho-da-upr.md` — a forma normativa do desfecho
- `docs/adr/016-dominio-executavel-em-memoria.md` — o domínio executável e suas duas metades
- `docs/adr/014-proibir-aresta-domain-port.md` — por que o relógio não vive no domínio
- `docs/dmpf/uow-inbox-outbox.md` — FND-04: UoW, sequência canônica, outbox
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §3.3, §4.1, §6.2, §7.3, §9.3, §10.2
