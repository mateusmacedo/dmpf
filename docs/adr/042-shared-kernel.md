# ADR-042: Designar unidades do kernel como shared kernel e estender a condição de contexto ao destino designado

## Status

Aceito — 2026-09-09. Implementa SPEC-XMNBMY50.

Estende [ADR-017](./017-bounded-context-declarado-superficie-publica.md), que
fixou a condição de contexto C2, e [ADR-028](./028-processo-de-autorizacao-da-classificacao.md),
que instituiu o ato de classificação — a designação de shared kernel é um ato
novo sob aquele processo, não uma exceção a ele.

## Contexto

O kernel DMPF existe para ser consumido. É essa a razão de `domain`,
`ports`, `application` e os providers terem nascido como módulos
próprios em vez de pacotes internos de uma aplicação: outros bounded contexts
devem poder construir sobre eles. A regra de dependência, como estava, tornava
isso impossível.

A regra decide cada aresta por duas condições independentes (`decide.go:34-39`).
C1 é a matriz de blocos. C2 é a condição de contexto, fixada pelo ADR-017: uma
aresta entre bounded contexts distintos só passa quando o destino é do bloco
`contract` ou declara `public_integration_surface: true`. E essa declaração é
inválida em bloco `domain` — o manifesto que a fizesse reprovaria com
`DMPF-M002` (`diagnostic.go:47`, RFC §10.1 e §7.2).

O choque é estrutural, não acidental. `application.Outcome[R]` é genérico
sobre o resultado de negócio e `ports.OutboxEntry`
(`libs/backend/go/ports/outbox.go:26`) carrega tipos de domínio no seu
próprio campo: quem importa qualquer bloco do kernel importa, por transitividade
de tipo, o `domain` do kernel. Consumir o kernel de outro contexto emitia
`DMPF-D002`, e não havia declaração legítima capaz de evitá-lo. As duas saídas
disponíveis eram degradar o kernel — desfazer a tipagem que o torna útil — ou
declarar algo inválido no manifesto e reprovar em `M002`.

Nenhum mecanismo existente cobria o caso. O bloco `contract` cobre a superfície
de integração assíncrona, que é o envelope e os eventos, não a API de runtime.
O `public_integration_surface` cobre a API pública de um contexto para outro,
mas o ADR-017 o proibiu deliberadamente em `domain`, porque abrir o domínio de
um contexto ao exterior é justamente o que a regra existe para impedir. O que
faltava era nomear um conjunto **específico** de unidades como compartilhado,
sem transformar isso em propriedade do bloco nem em propriedade do contexto
inteiro.

## Decisão

Fica instituída a noção de **shared kernel**: um conjunto nominal de unidades,
designado por ato de classificação, importável por qualquer bounded context.
Sete pontos definem a decisão.

**A designação vive no baseline governado, não no manifesto.** A chave
`shared_kernel_units` do `tools/dmpf-baseline/units-baseline.json` lista as
chaves canônicas designadas. O manifesto de cada módulo permanece intocado. A
razão é de autoridade: o manifesto é escrito por quem desenvolve o módulo, e a
designação de shared kernel é decisão de arquitetura sobre o repositório
inteiro. Colocá-la no manifesto entregaria ao autor do módulo o poder de se
declarar público.

**C2 passa a aceitar o destino designado, e só o destino.** A fórmula é
`SameBoundedContext(source, target) || PublicIntegrationSurface(target) ||
target.SharedKernel` (`decide.go:37`). A exceção é unidirecional por
construção: uma unidade designada não ganha licença para importar de fora do seu
contexto. Uma variante com `source.SharedKernel ||` abriria o kernel ao
exterior, o inverso do que se quer, e o vetor `origem no shared kernel não
propaga` existe para reprovar quem a reintroduzir.

**C1 e `DMPF-M002` não mudam.** A matriz de blocos segue idêntica: shared kernel
relaxa a condição de contexto, nunca a de bloco. Uma aresta que viola a matriz
continua emitindo `DMPF-D001` mesmo com o destino designado, e
`public_integration_surface: true` em `domain` continua inválido. A exceção nova
não é uma porta lateral para as duas regras anteriores.

**Chave que não resolve é fail-closed.** `DMPF-M004` — "Unidade de shared kernel
não resolvida" (`diagnostic.go:49`) — cobre os três modos de falha da
designação: chave sem nenhuma entrada correspondente, chave com duas ou mais, e
chave que resolve para uma entrada cujo `UnitKey` não está no universo. O
diagnóstico interrompe a fase (`PhaseHalted`) em vez de seguir decidindo
arestas: decidir sobre designação inválida seria pior que não decidir, porque
produziria um veredicto com aparência de conformidade. O terceiro modo não
constava do desenho original e foi acrescentado justamente porque, sem ele, a
designação falharia em silêncio.

**Mudar a designação é ato de classificação.** Vale o processo do ADR-028 e o
mecanismo mínimo da RFC §10.2: `DMPF-T002` reprova o commit que altera
`shared_kernel_units` junto com código. Designar é redesenhar a fronteira entre
contextos, e essa decisão precisa de commit próprio, revisável isoladamente.

**A ausência da chave é distinta da lista vazia.** `shared_kernel_units` ausente
significa "este repositório não adotou a designação"; `[]` significa "adotou e
não designou ninguém". A distinção é observável no wire por `json.RawMessage`,
porque em `[]string` puro a chave ausente e o valor `null` colapsam ambos em
`nil` — verificado empiricamente. O digest do baseline só incorpora a lista
quando a chave está presente, o que preserva byte a byte a fórmula legada e
mantém conforme, sem migração, todo baseline anterior a esta decisão. `null`
explícito é recusado: é ambíguo entre as duas intenções.

**`Input.SharedKernelUnits` não é autoritativo.** O `fitness` aceita a lista
por parâmetro, mas apenas quando não há store de baseline; havendo store, o
baseline é a única fonte e o parâmetro é ignorado. Sem essa restrição, uma
suíte de teste poderia conceder a si mesma a exceção que o baseline nega. Os
dois caminhos convergem no mesmo `baseline.Designar`, de modo que `M004` vale
igual nos dois.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Flag `shared_kernel: true` no manifesto do módulo | Transfere ao autor do módulo uma decisão de arquitetura sobre o repositório: qualquer módulo poderia se declarar público. Também escaparia do `DMPF-T002`, que vigia o baseline, não o manifesto. |
| Designar o bounded context inteiro | Contradiz o interior privado por default do ADR-017. Os agregados de exemplo (`example/orders`, `example/reservations`) e as composition roots do kernel passariam a ser importáveis, e são precisamente o que deve permanecer privado. |
| Refatorar `ports` e `application` para não expor tipos de `domain` | Resolveria o `D002` sem exceção nova, ao custo de destruir o que torna o kernel útil: `Outcome[R]` e `OutboxEntry` são genéricos sobre o domínio por design. Trocaria segurança de tipo por conformidade — o oposto do que a regra existe para proteger. |
| Distinguir presença por `nil` × não-nil, sem campo de presença | Impossível em `[]string`: `json.Unmarshal` colapsa chave ausente e `null` no mesmo `nil`, e `json.Marshal` de slice nil grava `null`. Sem `json.RawMessage` não há como separar as duas intenções, e o digest do baseline legado deixaria de fechar. |

## Consequências

**Positivas:**

- O kernel passa a ser consumível como SDK por outros bounded contexts sem
  furar C1, sem furar `M002` e sem degradar a sua tipagem.
- A exceção é nominal, versionada e auditável: quem está designado consta de um
  arquivo governado, e mudá-lo exige commit próprio sob o ADR-028.
- A unidirecionalidade impede o efeito colateral mais provável — abrir o kernel
  ao exterior ao tentar abri-lo ao consumo.
- Designação inválida interrompe a verificação em vez de aprovar em silêncio.
- Todo baseline anterior segue conforme sem migração, porque o digest só muda
  quando a chave está presente.

**Negativas:**

- `DMPF-M004` reprova com o mesmo peso dos demais diagnósticos. O catálogo do
  verificador não tem severidade — `CodeSpec` carrega `Code`, `Summary`,
  `Section` e `Applicable` (`diagnostic.go:33-38`) —, então não existe a
  possibilidade de emitir a designação malformada como aviso.
- A superfície que o `DMPF-T002` precisa vigiar cresce: além da classificação,
  agora também a designação.
- O caminho não é exercitado por nenhum PR normal, porque o baseline real não
  designa ninguém até a adoção. Cobri-lo exigiu um gate próprio
  (`tools/dmpf-shared-kernel-check.sh`) com fixture sintética e commits em
  worktree descartável — o primeiro gate do repositório que precisa de
  histórico git, já que `T002` decide sobre a mensagem de commit.
- A numeração salta de 040 para 042. O 041 está reservado pela branch do
  ARQ-531, ainda não integrada; reaproveitá-lo criaria colisão no merge.

## Referências

- `docs/specs/SPEC-XMNBMY50-dmpf-shared-kernel.md` — a spec implementada
- RFC §7.2 (condição de contexto, aqui estendida) e §10.2 (mecanismo mínimo de
  autorização), em `docs/dmpf/rfc-dmpf-foundation-v0.1.md`
- [ADR-017](./017-bounded-context-declarado-superficie-publica.md) — C2 e o
  interior privado por default
- [ADR-028](./028-processo-de-autorizacao-da-classificacao.md) — o ato de
  classificação e a evidência persistida
- [ADR-031](./031-verificador-de-conformidade-dmpf-em-go.md) — o verificador que
  realiza a decisão
- `docs/guides/dmpf-manifesto.md` — o rito operacional da designação
- `libs/backend/go/ports/outbox.go:26` — `OutboxEntry`, a exposição de tipo
  de domínio que motiva o caso
