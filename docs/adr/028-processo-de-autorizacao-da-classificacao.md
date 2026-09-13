# ADR-028: Instituir o processo de autorização da classificação com autoridade fechada e evidência persistida

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

Estendido pelo [ADR-042](./042-shared-kernel.md) — 2026-09-09: a designação de
shared kernel no baseline governado passa a ser ato de classificação sujeito a
este processo, e `DMPF-T002` reprova o commit que a altera junto com código.

## Contexto

A classificação de cada unidade arquitetural — o `block` e o `bounded_context`
de uma `verification_unit` — é metadado **autodeclarado**: a própria unidade
declara a classificação que deveria restringi-la. Comparar o manifesto com o
baseline flagra a divergência acidental, mas não fecha a brecha deliberada: o
autor altera os dois no mesmo commit, a comparação fica verde e imports antes
proibidos passam a ser liberados sem que nada acuse.

O ADR-013 já fixou o **requisito** que endereça essa brecha — mudar `block` ou
`bounded_context` de uma unidade existente é mudança normativa e exige
autorização distinta da autoria, com reprovação fail-closed na ausência de
evidência (T4–T6 de RFC §10.2). Um requisito, porém, não se executa sozinho. Sem
autoridade nomeada, rito e artefato de evidência, RFC §10.2 deixou apenas um
**mecanismo mínimo** provisório como piso, e a âncora ANC-08 amarra o fechamento
do risco de reclassificação oportunista precisamente à definição deste processo,
que a RFC delega ao FND-10 por RFC §13.3.

Três forças condicionam o desenho:

- **Não há estrutura de aprovação instalada.** O inventário AS-IS registra
  `CODEOWNERS` ausente em 10 dos 10 repositórios. A autoridade não pode se apoiar
  num owner pré-existente; precisa ser uma função separável do seu titular.
- **O ato a regular não é a edição de um campo.** Reorganizar `include`, root ou
  caminho pode mudar o `block` ou o `bounded_context` **efetivo** de um trecho de
  código sem tocar em nenhum campo — inclusive movendo código entre dois
  `bounded_context` distintos e liberando uma aresta inter-context. Um rito que
  só olhasse os campos seria evitável por construção.
- **O controle é adversarial.** A sua razão de ser é conter quem quer destravar
  uma dependência proibida. Nomear um titular por documento atribuiria
  responsabilidade sem consentimento, e desligar o piso da RFC antes de existir
  quem exerça o novo rito produziria um controle inoperante no lugar de um
  operante.

## Decisão

Instituir a **Autoridade de Classificação Arquitetural**: uma função
organizacional fechada, indicada por Arquitetura e revisada por Segurança, com
competência exclusiva sobre os quatro atos regulados — alterar `block`, alterar
`bounded_context`, criar unidade e remover unidade.

O ato regulado é o **delta efetivo** `arquivo → (canonical_key, block,
bounded_context)`, não a edição de um campo. Qualquer mudança de `include`, root
ou caminho que altere o `block` **ou** o `bounded_context` efetivo de um trecho
dispara o rito, ainda que nenhum campo seja tocado e nenhuma unidade nasça ou
morra.

Toda mudança que dispare o rito é aprovada, cumulativamente: **commit próprio**,
separado de código; **declaração do ato** no PR (qual dos quatro, a
`canonical_key`, o valor anterior e o novo); **justificativa** de por que a
classificação anterior estava errada; **aprovação da Autoridade**, distinta da
autoria; e **enumeração das arestas que a reclassificação passa a permitir** e
que a regra de dependência antes proibia. Autor e autorizador coincidentes
reprovam, e a aprovação exige **mais de um aprovador** distinto do autor.

A evidência é um **registro persistido e endereçável** no repositório — não uma
aprovação existente apenas na interface da forge —, com os campos do ato,
`moved_paths` para remapeamentos, as arestas recém-permitidas, a autoridade que
exerceu, os aprovadores e o commit. A verificação é fail-closed: ausência de
evidência reprova, e «não verificada» é reprovação, nunca aprovação provisória.

Este ADR **define a função, não o titular**. A vigência do rito fica amarrada,
cumulativamente, ao aceite de um titular (gate G2), à revisão da indicação por
Segurança e ao fechamento da ANC-08 (que exige o gate G5). Até que as três
condições sejam satisfeitas, o mecanismo mínimo de RFC §10.2 permanece vigente, e
as duas normas nunca se aplicam ao mesmo tempo sobre o mesmo ato.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Nomear uma pessoa ou equipe específica como autoridade | Atribuiria responsabilidade sem consentimento; com `CODEOWNERS` ausente em 10 dos 10 repositórios, não há estrutura instalada sobre a qual apoiar a indicação. A função é definida aqui; o titular é ato de pessoa (`AUT-07`). |
| Exigir um único aprovador, distinto do autor | Aprovador único é ponto único de falha num controle cuja razão de ser é adversarial; a pluralidade eleva o custo do conluio de duas para três pessoas (`AUT-04`). |
| Fazer o rito viger já na promoção deste PR | Pela leitura de «até que o FND-10 defina», o rito vigeria com uma função sem titular — um controle inoperante no lugar do controle mínimo operante da RFC (`AUT-09`). |
| Tratar «não verificada» como aprovação provisória para desbloquear o merge | Esvaziaria T5, cuja verificação é posterior ao merge; o resultado ambíguo é reprovação para efeito de merge (`AUT-06`). |
| Regular apenas os campos `block` e `bounded_context` | Tornaria o rito evitável por construção: bastaria remapear `include` para obter o mesmo efeito sem tocar em campo algum (`AUT-01`). |

## Consequências

**Positivas:**

- O que o ADR-013 deixava sem executor passa a ter desfecho nomeado por
  tentativa: alterar `block` no mesmo commit da mudança de código, aprovar a
  própria alteração, justificar apenas descrevendo o que mudou, omitir as arestas
  liberadas ou aprovar somente na interface da forge reprovam, cada qual por uma
  regra citável, em vez de dependerem do julgamento de quem revisa (`AUT-08`,
  vetores V1 a V8).
- A tentativa de liberar uma aresta inter-context remapeando `include` entre
  `bounded_context` distintos, sem tocar em nenhum campo, passa a reprovar como
  uma alteração de `bounded_context` sujeita ao rito completo: o caminho de
  evasão mais sutil deixa de existir, e o remapeamento fica auditável pelo
  `moved_paths` que a evidência registra.
- A evidência persistida e endereçável torna T5 verificável depois do merge, e
  não apenas no instante da revisão.
- Separar a função do titular permite instalar o controle sem nomear pessoa sem
  consentimento e sem depender de uma estrutura de owners que não existe hoje.
- A enumeração das arestas liberadas entrega ao revisor, ainda dentro do PR e
  antes do merge, cada reclassificação já medida contra a função `decide()`: o
  efeito sobre a regra de dependência chega explícito à revisão, não implícito
  na mudança de metadado.
- Amarrar a vigência ao fechamento efetivo da âncora elimina a janela em que o
  piso da RFC seria desligado com a ANC-08 ainda aberta.

**Negativas:**

- A pluralidade de aprovadores impõe fricção a toda reclassificação e, ainda
  assim, não elimina o conluio: autor, autoridade e segundo aprovador em conluio
  continuam suficientes. **Custo aceito:** o rito eleva o custo do ataque, não o
  zera, e este ADR não promete a resistência que não tem.
- Enquanto o titular não é aceito (G2) e a ANC-08 não fecha (G5), o processo
  permanece `definido` e não vigente. **Custo aceito:** o controle efetivo
  continua sendo o mecanismo mínimo da RFC, e o risco de reclassificação
  oportunista segue apenas parcialmente mitigado até a transição.
- Cada ato exige commit próprio, `moved_paths` no remapeamento e a enumeração das
  arestas liberadas, sob pena de reprovação. **Custo aceito:** a disciplina
  manual em cada mudança de classificação é o preço de tornar o ato auditável, e
  a ausência de qualquer um desses elementos reprova.

## Referências

- **ADR-013** — exige autorização distinta da autoria para mudar `block` e
  `bounded_context`, fail-closed na ausência de evidência. É o **requisito** que
  este ADR executa: aqui entram a autoridade, o rito e a evidência que o
  satisfazem, sem reabri-lo.
- **ADR-012** — classifica por metadado declarado. É a declaração cuja
  integridade este rito protege: o metadado autodeclarado é a brecha que o
  processo fecha.
- **ADR-011** — fixa a `verification_unit` e o seu binding por stack. O delta
  efetivo por `include` depende dela: reatribuir código entre unidades, cujos
  roots determinam o conjunto, é reclassificá-lo.
- **ADR-010** — decide a regra de dependência como função `decide()` sobre os
  seis blocos. A enumeração de arestas liberadas mede a reclassificação contra o
  que essa função passaria a permitir.
- **ADR-017** — torna o `bounded_context` obrigatório (dona de C2). O
  remapeamento entre contextos distintos só é ato regulado porque C2 reprovava a
  aresta inter-context que a manobra liberaria.
- **Origem:** FND-10 — `governanca-bom-pilotos.md`, §5.2 (autorização da
  classificação, `AUT-01`–`AUT-10`), acionado em §8.1 como `ADR-DMPF-S`; a
  subseção recepciona T1–T6 de RFC §10.2 e opera sob RFC §13.3.
- **SPEC-DBTRMM3X** — spec da série de ADRs mínimos do DMPF (sub-spec FND-11).
- **ARQ-448** — story FND-11, que redige, promove e numera esta série.
