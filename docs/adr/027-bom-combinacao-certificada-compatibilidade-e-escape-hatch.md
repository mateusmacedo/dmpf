# ADR-027: Governar o produto por BOM de combinação certificada, compatibilidade por sujeito versionado e escape hatch de universo fechado

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

## Contexto

O risco §15 do épico ARQ-436 e a advertência de `Parte-1 §17.2` apontam para o mesmo
erro: congelar versões de bibliotecas dentro do documento arquitetural. A mitigação
já nomeada é gerenciar versões num BOM publicado por release. Mas um BOM sem modelo
de versão é planilha. Para afirmar que uma combinação é suportada, é preciso antes
responder três perguntas que a mitigação sozinha não resolve: **o que** é versionado,
**o que** significa compatibilidade para cada coisa versionada, e **quem** decide.

Sob ANC-09, o escopo permitido é «matriz de versões certificadas, escape hatches e
seleção de pilotos» — mais estreito que o assunto «governança, BOM e pilotos». A
janela de suporte e a depreciação de produto, que o AC-11 item 1 cobra, só cabem no
escopo se a matriz certificada for lida como o instrumento em que o suporte se
materializa: declarar uma combinação certificada é declará-la suportada, e removê-la
é depreciá-la. Sem esse elo, o suporte sairia sem dona, e não há outra dona
registrada.

Duas forças adicionais moldam a decisão. A primeira: «combinação certificada, não
faixa de versão» só tem conteúdo se `certificada` significar evidência exercitada — a
combinação específica que quebra é justamente a que nunca foi exercitada em conjunto,
não a que está fora de uma faixa nominal. A segunda: o escape hatch é o único
instrumento capaz de furar a governança, e reclassificar é o caminho **mais barato**
para o mesmo efeito — em vez de pedir exceção nominal com prazo, basta declarar que a
unidade pertence a outro bloco. É o risco R1 (reclassificação oportunista) visto do
lado da governança de produto. Definir o escape hatch sem fechar essa porta dos
fundos entregaria a fechadura junto com a chave reserva.

## Decisão

Adotar, sob ANC-09, o modelo de versão de §3, o BOM de §4 e o escape hatch de §5.1.

**BOM por combinação certificada (§4).** Cada release do produto tem exatamente um
BOM versionado no repositório canônico, com os seis itens de `Parte-1 §17.2`; item
sem instância é declarado vazio, nunca omitido (`BOM-01`, `BOM-02`). Uma entrada só é
`certificada` quando percorre a máquina de estados de §4.4 até ter evidência
exercitada — `evidence_uri` **e** `evidence_digest` —, aprovador (`BOM-05`) e
validade declarada (`BOM-08`); certificação vencida volta a `candidata`. O campo
`compatible_with` declara combinação **exercitada**, não presumida (`BOM-04`). O BOM
**referencia** o registro autoritativo de cada domínio alheio — pins de Buf, stacks,
telemetria — sem duplicá-lo (`BOM-06`).

**Compatibilidade por sujeito versionado (§3.2, §3.3).** Toda regra de compatibilidade
declara o seu sujeito: uma regra sem sujeito é inaplicável (`GOV-10`). Os cinco
sujeitos — produto, kernel, contrato de wire, runtime certificado e ferramenta de
geração — têm cada um direção, piso, autoridade e evidência próprios na matriz de
§3.3 (`GOV-11`). O piso é `N` e `N−1`, com duas ressalvas fixas: a ferramenta de
geração tem pin exato e **não** admite `N−1` (`GOV-12`, sobre `BUF-06`), e a classe de
criticidade é declarada — a ausência é lida como `crítica`, o piso mais estrito
(`GOV-13`).

**Depreciação de produto (§3.5).** Este artefato governa a depreciação de release,
major de kernel, runtime e combinação; a de contrato de wire permanece no FND-05.
Depreciar é estado declarado no BOM, com data de fim de suporte e sucessor nomeado
(`GOV-18`); a janela mínima é de 180 dias (`GOV-19`); a remoção exige,
cumulativamente, janela vencida **e** ausência de consumidor declarado (`GOV-20`).

**Escape hatch de universo fechado (§5.1).** Divergir do golden path é permitido e
rastreado, e a concessão exige quatro itens cumulativos, incluindo plano de
convergência **com prazo** — sem a condicional «quando aplicável» (`GOV-30`). A
exceção só incide sobre o universo positivo E1–E3 (`GOV-31`); a regra de negação
N1–N7 nega, na admissão, exceção sobre constraint P0, célula da regra de dependência
de RFC §7, invariante de âncora, requisitos T1–T6, classificação de unidade, pin de
geração e evidência de certificação (`GOV-32`). O escape hatch **não** autoriza
reclassificar, e reclassificar não o dispensa (`GOV-26`).

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Declarar faixas SemVer amplas por dependência, em vez de combinação certificada | §4.2, `BOM-04`: a combinação específica que quebra é justamente a que nunca foi exercitada em conjunto; uma faixa nominal não captura isso |
| Tratar «`N` e `N−1`» como regra única, sem sujeito | §3.2, `GOV-10`: «N de quê, compatível com quê, na avaliação de quem» é indecidível sem sujeito; o texto de origem não nomeia sujeito, direção nem autoridade |
| Admitir `N−1` de plugin de geração | §3.3, `GOV-12`: duas versões de plugin em uso não são compatibilidade, são dois artefatos gerados; reintroduziria a irreprodutibilidade que `BUF-06` fecha |
| Deixar o default de criticidade no lado permissivo | §3.3 `rationale`, `GOV-13`: com o default permissivo, a omissão sob pressão de entrega viraria licença para remover `N−1`; o default estrito custa a quem quer remover, que é quem tem incentivo de declarar |
| Manter o «plano de convergência quando aplicável» do AC-11 item 3 | §5.1 `rationale`, `GOV-30`: «quando aplicável» é autoavaliado por quem pede a exceção, que tem incentivo para concluir que não se aplica; o resultado previsível é a exceção permanente, um segundo padrão não revisado |
| Listar apenas as negações do escape hatch, sem universo positivo | §5.1 `rationale`, `GOV-31`: uma lista só de negações é sempre incompleta contra um pedido criativo; um universo positivo fechado é completo por construção — o que não está em E1–E3 não é excepcionável |
| Fixar a cadência do release train sem release publicada | §3.4 `rationale`, `GOV-16`: uma cadência declarada e não cumprida é pior que uma pendente, porque aparenta previsibilidade; fixar um número sem release seria inventar cadência não cumprível |

## Consequências

**Positivas:**

- Suporte deixa de ser afirmação e passa a ser verificável: uma combinação
  `certificada` carrega evidência exercitada, digest não substituível em silêncio,
  aprovador e validade, e a máquina de estados de §4.4 impede transição por omissão.
- «N e N−1» torna-se decidível: cada sujeito declara direção, piso, autoridade e
  evidência, e o default `crítica` faz a omissão custar a quem quer remover suporte.
- A janela única de 180 dias evita dois calendários de depreciação correndo sobre o
  mesmo repositório, sem reabrir nenhuma regra do FND-05, que mantém a depreciação de
  contrato.
- O escape hatch é completo por construção: o universo positivo diz o que se pode
  excepcionar e a regra de negação fecha, na admissão, o que nenhuma exceção alcança —
  inclusive a reclassificação como atalho para o mesmo efeito.

**Negativas:**

- **Custo aceito:** certificar por combinação exercitada tem custo recorrente. A
  validade vence e a certificação vencida volta a `candidata` (`BOM-08`), o que obriga
  a reexecutar a suite de compatibilidade a cada vencimento ou troca de biblioteca —
  mais caro que declarar uma faixa de versão, que é precisamente o que este ADR recusa.
- **Custo aceito:** endurecer o escape hatch nega exceção sem prazo. Os casos em que a
  convergência genuinamente não é planejável passam a exigir o prazo de **revisão** de
  `GOV-34`, com aprovação de Arquitetura **e** Plataforma — fricção que o AC-11, na
  formulação original com «quando aplicável», não impunha.
- **Custo aceito:** referenciar em vez de duplicar (`BOM-06`) exige do leitor dois
  documentos — o BOM e o registro autoritativo do FND-05 — para ter o quadro completo
  de pins e stacks. É o custo certo para não criar duas fontes que divergem no
  primeiro update de uma delas.

## Referências

- ADR-010 — institui a regra de dependência como a função `decide` sobre os seis
  blocos e as células de RFC §7. A regra de negação do escape hatch (`GOV-32`, N2)
  torna o resultado dessa função não excepcionável; é sobre a decisão de ADR-010 que
  a negação incide, e por isso ele é dependência direta desta.
- ADR-013 — exige autorização distinta da autoria para mudar `block` ou
  `bounded_context`, com fail-closed. É o que sustenta a fronteira `GOV-26`/`GOV-32`
  N5: o escape hatch não pode ser porta dos fundos para reclassificar, porque a
  classificação tem rito próprio e separado.
- ADR-023 — fixa no repositório de contratos a autoridade de validação com Buf. O pin
  exato da ferramenta de geração como exceção ao piso `N`/`N−1` (`GOV-12`, sobre
  `BUF-06`) e a referência-não-duplicação do BOM (`BOM-06`) repousam sobre essa
  autoridade estar no repositório de contratos.
- Artefato de origem: `docs/dmpf/governanca-bom-pilotos.md` (FND-10) — §3.2 e §3.3
  (sujeitos versionados e matriz de compatibilidade), §3.4 (release train), §3.5
  (depreciação de produto), §4.2 (`compatible_with` exercitado, `BOM-04`), §4.3
  (referência sem duplicação), §4.4 (máquina de estados da certificação) e §5.1
  (escape hatch, universo positivo e regra de negação).
- SPEC-DBTRMM3X — spec que esta série de ADRs implementa.
- ARQ-448 — https://lider-cap.atlassian.net/browse/ARQ-448 (FND-11: redação,
  promoção e aceite da série de ADRs do DMPF).
