# ADR-015: Restringir capabilities externas por bloco: default deny com allowlist por entrypoint e pureza transitiva

## Status

Aceito — 2026-08-28. Implementa SPEC-DBTRMM3X.

## Contexto

A regra de dependência do DMPF (ADR-010, RFC §7) governa apenas as arestas entre
unidades do universo verificável — ela decide se um bloco pode depender de outro. O que ela
não enxerga é o import que sai desse universo: um driver de banco, um ORM, um SDK
de broker ou um cliente HTTP não são unidades classificadas, são dependências
externas, e a matriz de blocos passa por cima delas. Sem uma política própria, a
regra mais consequente do modelo ficaria descoberta: um `application service`
poderia importar um driver concreto e ainda assim sair verde na verificação.

A lacuna não é hipotética. Ela já se materializou em produção — a evidência
registrada em RFC §6.2 mostra `legado-titulos-shared-services` importando GORM dentro de
um caso de uso (`virar_campanha.usecase.go`), exatamente o acesso a I/O que a
camada de aplicação não deveria conhecer.

Três forças estão em tensão. A primeira: domínio e portas precisam permanecer sem
I/O para serem executáveis em memória e testáveis sem infraestrutura, o que exige
uma proibição rígida. A segunda: proibir "qualquer biblioteca externa" de forma
ingênua inviabilizaria o reuso de bibliotecas puras legítimas e tornaria a política
impraticável. A terceira: a fronteira entre puro e impuro não é dada pelo nome do
pacote — um mesmo pacote pode ter um subpath puro e outro que toca disco ou rede, e
uma dependência hoje pura pode deixar de sê-lo em uma versão futura. A política
precisa, então, classificar o *acesso* que cada dependência concede, ser
inegociável onde a pureza é essencial e permissiva onde ela não se aplica.

## Decisão

Adotar uma política de capabilities externas por bloco, ancorada em RFC §6.

Toda dependência cujo import não resolve para um arquivo do universo verificável é
**externa** (RFC §6.1) e recebe uma *capability* declarada — a natureza do acesso
que ela concede (`io.storage`, `io.messaging`, `io.network`, `runtime.framework`,
`wire.codec`, `observability`, `pure`, entre outras). A capability é declarada na
allowlist, nunca inferida do nome do pacote.

A política por bloco (RFC §6.2), aplicada sobre a classificação já declarada por
unidade (ADR-012), é:

- `domain library` e `port` em **default deny**, admitindo apenas `pure` — a
  assinatura de uma porta não expõe tipo de driver, SDK ou wire.
- `application service` em default deny com exceção nominal, sem nenhuma capability
  `io.*`, `runtime.framework` nem `wire.codec`: precisar de I/O significa precisar
  de uma porta.
- `contract package` restrito a `pure` e `wire.codec`.
- `provider` permissivo para qualquer capability da porta que implementa.
- `app` permissivo apenas no composition root, onde a instanciação de provider
  concreto ocorre.

Observability tem regra própria (RFC §6.2): é proibida no domínio e na porta ainda
que a biblioteca de logging seja tecnicamente pura — telemetria vive nos adapters,
pipelines e providers.

Um bloco default deny só pode usar uma dependência externa se ela constar de uma
**allowlist** (RFC §6.3) que declare, por entrada, o pacote, a faixa de versões, os
entrypoints permitidos e a capability. A pureza é **transitiva**: computada a
partir dos entrypoints declarados, na faixa de versão declarada, ela reprova quando
o fechamento introduz capability não-pura, quando a versão sai da faixa, quando um
import dinâmico não é estaticamente determinável, ou quando a dependência não
resolve — sempre fail-closed, nunca tratada como ausente.

Exceções só valem se **nominais** (RFC §6.4): par (unidade, dependência),
justificativa, owner e data de revisão. Exceção por categoria, por prefixo de
pacote ou por diretório é proibida.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Computar a pureza transitiva a partir do pacote inteiro — todos os subpaths — em vez de restringi-la aos entrypoints declarados na allowlist | RFC §6.3 (`rationale`): qualquer pacote grande o bastante conteria, em algum subpath, um acesso a I/O e seria classificado impuro; o default deny do domínio degeneraria em proibição total de biblioteca externa, tornando a allowlist inutilizável. A regra dos entrypoints é justamente o que a torna praticável |

RFC §6 traz um único bloco `rationale` (em §6.3), do qual sai a alternativa acima.
As demais escolhas da seção — inferir capability pelo nome, confiar apenas na
matriz de §7, liberar observability por ser tecnicamente pura — estão fixadas em
blocos `normativo`/`evidência`, não em `rationale`, e por isso não figuram aqui
como alternativa considerada.

## Consequências

**Positivas:**

- Fecha a lacuna que a matriz de RFC §7 não cobre: um `application service` não
  consegue mais importar um driver concreto e passar verde, porque a dependência
  externa é classificada por capability e barrada no bloco default deny.
- Domínio e portas permanecem executáveis em memória e testáveis sem
  infraestrutura, porque só admitem a capability `pure`.
- A allowlist com entrypoints declarados permite reaproveitar bibliotecas externas
  puras sem abrir mão do rigor: um pacote com subpath puro e subpath impuro pode
  ter apenas o primeiro declarado.
- A pureza atrelada à faixa de versão torna a política resistente a regressão
  silenciosa — um bump que introduza I/O no fechamento reprova até a allowlist ser
  revista.
- A exceção nominal preserva rastreabilidade: cada desvio tem par (unidade,
  dependência), justificativa, owner e data.

**Negativas:**

- **Custo aceito:** manter a allowlist é curadoria recorrente e explícita. Cada
  dependência externa usada por um bloco default deny exige uma entrada com pacote,
  faixa de versões, entrypoints e capability, e todo bump que altere o fechamento
  força revisão da entrada antes de o verificador voltar ao verde. Aceita-se esse
  trabalho contínuo em troca de pureza verificável no domínio e na porta.
- O fail-closed sobre imports dinâmicos não estaticamente determináveis pode barrar
  padrões legítimos até que sejam tornados determináveis ou registrados como
  exceção nominal.
- A atribuição de capability é declarada, não inferida: depende de disciplina
  humana e de um catálogo mantido — um erro de classificação na allowlist propaga
  política errada de forma silenciosa.
- Proibir observability no domínio e na porta empurra a telemetria para adapters,
  pipelines e providers, o que pode aumentar a verbosidade de instrumentação fora
  do núcleo mesmo quando a biblioteca é tecnicamente pura.

## Referências

- ADR-010 — regra de dependência e os seis blocos. Institui a matriz de blocos e a
  função de decisão sobre a qual esta política incide: a política de capabilities é
  declarada por bloco, e a lacuna que este ADR fecha é justamente a aresta externa
  que a matriz de RFC §7 não enxerga.
- ADR-012 — classificação por metadado declarado. Esta decisão pressupõe a
  classificação já declarada por unidade: só assim se sabe qual bloco cada unidade
  ocupa e, portanto, qual política de capability aplicar. O universo verificável
  contra o qual "dependência externa" se define é o conjunto dessas unidades
  classificadas.
- ADR-016 — domínio executável em memória. O critério de execução em memória apoia
  a sua metade estática na pureza (`pure`) que este ADR estabelece; este ADR é
  pré-requisito daquele, não o contrário.
- RFC DMPF Foundation v0.1 — `docs/dmpf/rfc-dmpf-foundation-v0.1.md`: RFC §6 (origem
  da decisão), RFC §6.1 (o que é dependência externa), RFC §6.2 (política por
  bloco), RFC §6.3 (allowlist e pureza transitiva), RFC §6.4 (exceção nominal), RFC
  §7 (regra de dependência, a matriz que esta política complementa), RFC §4.1 (os
  seis blocos) e RFC §13.2 (acionamento `ADR-DMPF-F`).
- SPEC-DBTRMM3X — spec que esta série de ADRs implementa.
- ARQ-448 — ARQ-448 (FND-11, redação,
  promoção e aceite da série de ADRs do DMPF).
