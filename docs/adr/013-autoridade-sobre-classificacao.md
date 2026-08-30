# ADR-013: Exigir autorização distinta da autoria para mudança de classificação, com fail-closed

## Status

Aceito — 2026-08-28. Implementa SPEC-DBTRMM3X.

## Contexto

O metadado de classificação do DMPF é **autodeclarado**: cada `verification_unit`
declara no manifesto o `block` e o `bounded_context` que deveriam restringir o
que ela pode importar. A verificação compara esse manifesto com um baseline —
uma cópia independente da mesma classificação. A comparação flagra divergência
**acidental** (alguém mexeu no manifesto e esqueceu o baseline), mas não impede o
ataque deliberado: o autor altera manifesto e baseline no mesmo commit, a
comparação fica verde e imports antes proibidos passam a ser liberados sem que
nada acuse.

A força em tensão é que **autoria e autoridade coincidem**. Quem escreve o código
é quem reclassifica a unidade, sem contrapeso. E mudar `block` ou
`bounded_context` de uma unidade existente não é edição comum — é mudança
**normativa**, porque redefine a fronteira de imports da unidade. Agrava o quadro
a ausência de qualquer estrutura de aprovação sobre a qual se apoiar: `CODEOWNERS`
está ausente em 10 dos 10 repositórios inventariados, e o inventário registra
`owner não identificado` como valor padrão (RFC §10.2, bloco `evidência`). Não há
hoje autoridade estabelecida.

O problema, então, é conter a reclassificação oportunista **sem presumir** um
processo de governança que ainda não existe, e sem que o verificador finja
conformidade quando não tem como avaliar a autorização.

## Decisão

Tratar a alteração de `block` ou de `bounded_context` de uma unidade existente —
e também a criação e a remoção de unidade — como **mudança normativa que exige
autorização distinta da autoria da mudança** (RFC §10.2, T4 e T6).

O manifesto é a **fonte canônica** da classificação; o baseline é uma **cópia
independente**, mantida fora do `ownership_module` que descreve, cobrindo `block`
e `bounded_context` de cada `canonical_key` mais um digest do conjunto (T1-T2).
Divergência entre manifesto e baseline **reprova** (T3). Na ausência de evidência
de autorização em uma mudança que satisfaça T4, o verificador **reprova —
fail-closed** (T5).

A RFC fixa o **requisito** (T4-T6) e delega o **processo** — quem aprova, por qual
rito, qual artefato constitui evidência — ao **FND-10**, via ANC-08 /
[ARQ-447](https://lider-cap.atlassian.net/browse/ARQ-447), com ADR marcado como
exigido (RFC §12). Até o FND-10 concluir, vale o **mecanismo mínimo**: toda
mudança que satisfaça T4 é apresentada em commit próprio, separado de mudanças de
código, e a evidência de autorização é a aprovação desse commit por revisor
distinto do autor. Um verificador que não consiga avaliar essa condição **reporta
"não verificado", nunca "conforme"** (RFC §10.2 e §11.3).

Os vetores de conformidade ancoram a decisão em diagnósticos testáveis: `V10` e
`V12` cobrem a integridade do baseline (`DMPF-T001`) e `V11` cobre a
reclassificação conjunta sem autorização (`DMPF-T002`), cada um pareado nas duas
stacks (RFC §11.2).

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Confiar apenas na comparação entre manifesto e baseline, sem exigir autorização distinta da autoria | Detecta só divergência **acidental**. O autor que altera manifesto e baseline no mesmo commit deixa a comparação verde e libera imports antes proibidos — o ataque deliberado passa. É essa lacuna que T4-T6 fecham (RFC §10.2). |
| Definir na própria RFC a autoridade aprovadora e o rito de autorização, em vez de fixar só o requisito | Não há autoridade estabelecida: `CODEOWNERS` ausente em 10 de 10 repositórios, `owner não identificado` como padrão. Especificar o processo aqui presumiria uma governança inexistente; por isso o requisito fica na RFC e o processo é atribuído ao FND-10 (RFC §10.2 e §12, ANC-08). |
| Fail-open: o verificador reporta "conforme" quando não consegue avaliar a autorização | Reintroduz o falso verde que a decisão existe para eliminar. T5 exige fail-closed, e a §11.3 é explícita: um verificador que reporte `V11` como "aprovado" sem meio de avaliá-lo está **incorreto** — o resultado correto é "não verificado" (RFC §10.2 e §11.3). |

A §10.2, única seção de origem deste ADR, **não contém bloco `rationale`** —
apenas blocos `normativo` e `evidência` —, embora a §13.2 declare o `rationale`
da origem insumo obrigatório da redação. Os motivos acima foram extraídos do
contraste do próprio texto normativo (o que T4-T6 proíbem revela o que foi
rejeitado) e do bloco `evidência`, não de um `rationale` formal. A lacuna está
registrada nas observações desta entrega.

## Consequências

**Positivas:**

- A reclassificação oportunista deixa de ser silenciosa: o ataque passa a exigir
  mudança visível, isolada em commit próprio e aprovada por terceiro (RFC §10.2,
  alcance do R1).
- O fail-closed elimina o falso verde — sem evidência de autorização, o
  verificador reprova em vez de liberar (T5).
- O diagnóstico é rastreável e testável: divergência gera `DMPF-T001`,
  reclassificação sem autorização gera `DMPF-T002`, com os vetores `V10`-`V12`
  pareados Go/TS (RFC §11.2).
- Separar **requisito** de **processo** destrava as sub-specs: a governança fica
  isolada em ANC-08 / FND-10 e não bloqueia o restante do trabalho (RFC §12).
- O baseline como cópia independente, fora do módulo que descreve, impede que uma
  mesma edição altere a norma e a verificação da norma sem deixar rastro (T1-T2).

**Negativas:**

- **Custo aceito:** o risco R1 (reclassificação oportunista) fica **parcialmente
  mitigado, não eliminado** — um aprovador desatento, ou conluio entre autor e
  aprovador, ainda burla o mecanismo. A RFC aceita esse resíduo explicitamente,
  em vez de prometer resistência que não tem (RFC §10.2 e §14.6).
- A garantia depende de um processo ainda inexistente: até o FND-10 concluir,
  vale só o mecanismo mínimo (commit próprio + revisor distinto), e a autoridade
  formal permanece em aberto sob ANC-08.
- A condição de autorização **não é `import-verifiable`**: `V11` é
  `structurally reviewable` e exige inspeção do histórico. Onde o verificador não
  a avalie, ele reporta "não verificado", transferindo a carga para revisão
  humana (RFC §11.3).
- Manter o baseline independente e isolar cada reclassificação em commit próprio
  impõe overhead operacional e disciplina de fluxo que nenhum dos repositórios
  inventariados pratica hoje.

## Referências

- ADR-011 — verification_unit como unidade arquitetural. Define a unidade cuja
  classificação este ADR protege; T4 e T6 governam a criação e a remoção dessa
  unidade.
- ADR-012 — classificação por metadado declarado. Institui `block` e
  `bounded_context` como metadados autodeclarados no manifesto — a classificação
  que esta decisão impede de reclassificar sem autorização distinta da autoria.
  O ADR-012 aponta para este ADR ao registrar que a própria mitigação dele é
  parcial.
- ADR-017 — bounded context declarado e superfície pública. Torna o
  `bounded_context` metadado obrigatório; a alteração de `bounded_context` que
  T4 trata como mudança normativa pressupõe o contexto que ele institui.
- SPEC-DBTRMM3X — especificação da série de ADRs do DMPF.
- RFC DMPF Foundation v0.1:
  - **Origem:** §10.2 (trust model) — fixa o requisito de autorização distinta
    da autoria (T1-T6) e o fail-closed.
  - **Apoio, desdobrado de §10.2:** §11.2 e §11.3 (vetores V10-V12 e diagnósticos
    DMPF-T001/DMPF-T002; resultado "não verificado" em vez de "conforme"), §12
    (ANC-08, delegação do processo ao FND-10) e §14.6 (riscos aceitos). A §13.2
    declara §10.2 como origem única deste ADR; estas seções são apoio que §10.2
    aciona, não origens independentes.
- ARQ-447 — https://lider-cap.atlassian.net/browse/ARQ-447 (FND-10, processo de
  autorização da classificação).
- ARQ-448 — https://lider-cap.atlassian.net/browse/ARQ-448 (destino da série de
  ADRs do DMPF).
