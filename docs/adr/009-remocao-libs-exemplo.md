# ADR-009: O template não carrega libs nem generator de exemplo

## Status

Aceito — 2026-08-12. Implementa SPEC-7VQFX9RP (ARQ-470). Substitui os itens 9 e
3 da matriz de "o que não foi portado" do
[ADR-006](./006-fechamento-port-melhorias.md), que registraram as decisões
opostas ("as duas libs intactas" e o generator preservado quando o monorepo de
origem o esvaziou). Os demais itens daquela matriz seguem válidos.
Não altera as decisões do [ADR-004](./004-workflows-verdaccio-release.md) nem do
[ADR-008](./008-identidade-automacao-release.md): o split version/publish, o alvo
Verdaccio e a identidade de automação seguem como estão. O `nx-release.yml`
recebe apenas um guard novo, descrito abaixo.

## Contexto

O `AGENTS.md` declara que o workspace "não contém apps — quem parte deste
template cria as suas". Para as bibliotecas, porém, a promessa era outra: o
repositório entregava `@mateusmacedo/shared-types` (tipos de API, paginação e
ordenação) e `@mateusmacedo/shared-utils` (utilitários de string e objeto), com
23 arquivos versionados, README, CHANGELOG e histórico de versões próprio.

As duas nasceram no port do monorepo de origem, decidido pela a spec de port do monorepo de origem. O
critério aplicado ali separava domínio de genérico: `libs/shared/money` foi
descartada por ser lógica financeira do produto de origem, enquanto `types` e
`utils` foram mantidas por serem utilitários neutros. O item 9 da matriz do
ADR-006 registrou o resultado como "as duas libs intactas".

O critério estava correto para a pergunta que respondia — quais libs da origem
pertencem a um template — mas não respondia a outra: se um template deve
entregar libs. Todo projeto derivado herdava as duas sem tê-las pedido, e
precisava decidir, logo no primeiro dia, entre adotar utilitários que não
escolheu, apagá-los à mão ou conviver com código morto. Nenhum projeto do
próprio workspace as consumia: sem apps, o grafo do Nx não registrava dependência
alguma para elas.

## Decisão

**As duas libs saem, e `libs/shared/` passa a ser diretório de destino vazio**,
marcado com `.gitkeep`, no mesmo padrão que `libs/backend/` e `libs/frontend/` já
seguiam. O workspace derivado nasce com zero projetos de produção, tornando
uniforme a promessa do template entre `apps/` e `libs/`.

**A infraestrutura de release permanece armada, com um guard novo.** O `nx.json`
continua declarando `release.projects: ["tag:type:lib"]`, e os workflows
`nx-release.yml` e `nx-publish-libs.yml` seguem registrados.

O `nx release` **não** conclui vazio quando o release group não casa com projeto
algum: ele aborta com `RELEASE_GROUP_MATCHES_NO_PROJECTS` e `process.exit(1)`
(`nx/dist/src/command-line/release/config/config.js`, verificado no Nx 23.1.0).
Sem tratamento, o job de versionamento falharia no primeiro merge de
`release/**` para `master` após esta remoção.

Por isso o `nx-release.yml` ganha o step `Detect lib release candidates`, que
conta os projetos com `tag:type:lib` e condiciona os steps
`Nx Release (skip publish)` e `Push version commit and tags` a existir ao menos
um. É a réplica exata do guard que o mesmo arquivo já aplicava aos apps Docker —
que são pulados na ausência de `tag:type:app` justamente por causa desse
mecanismo, não por comportamento nativo do Nx. Com o guard, o pipeline fica
armado e ocioso até a primeira lib do consumidor.

**O generator `shared-lib` sai junto**, e com ele o `tools/generators.json` e a
chave `nx.generators` do `package.json`. `tools/generators/` fica como destino
vazio, ao lado de `tools/executors/`, que já era assim.

Ele estava obsoleto por dois motivos independentes. A coleção não resolvia —
`Cannot find module '@dmpf/tools/package.json'`, porque o mapeamento
vivia em `nx.generators` e o Nx procura uma chave `generators` de topo; o
`docs/nx-reference/tasks.md` já registrava essa falha como débito conhecido e
mandava usar os generators do Nx no lugar. E o template
`files/package.json__tmpl__` declarava `"private": true` sem `publishConfig`, de
modo que a lib gerada nem receberia o target `nx-release-publish`.

Corrigir os dois defeitos manteria no repositório um utilitário que duplica
`@nx/js:lib` sem acrescentar nada. **O caminho para criar libs passa a ser o
generator do Nx**, com as flags e as três tags 3D documentadas em
`docs/nx-reference/tasks.md` — que já o indicava como recomendado.

**Nada é despublicado.** As tags git `{projectName}@{version}` já criadas e os
pacotes `@mateusmacedo/shared-types` e `@mateusmacedo/shared-utils` no Verdaccio
permanecem onde estão. Removê-los quebraria qualquer consumidor que já os tenha
instalado, e reescreveria histórico publicado.

## O que muda no ADR-006

| Registro do ADR-006 | Situação após este ADR |
| --- | --- |
| Item 9: "`libs/shared/` \| `types` e `utils` \| A lib de domínio da origem não pertence a um template \| as duas libs intactas" | Substituído: as duas libs saem. O critério original (domínio versus genérico) continua correto para o port; a pergunta respondida aqui é outra — se um template deve entregar libs. |
| Item 3: "`tools/generators.json` \| generator `shared-lib` \| A origem esvaziou (`generators: {}`); o generator é capacidade do template \| arquivo não alterado no diff" | Substituído: o generator sai. Ele nunca foi capacidade real — não resolvia —, e o que se pretendia entregar já vem do `@nx/js:lib`. A origem, que o esvaziou, estava certa. |

Os demais itens da matriz do ADR-006 não são afetados.

## Alternativas consideradas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Remover as libs e desligar junto a infraestrutura de release (workflows e a chave `release` do `nx.json`) | Transferiria ao consumidor exatamente o trabalho que o template existe para poupar, e desfaria as decisões dos ADRs 004 e 008 sem que nada nelas tenha se mostrado errado. O pipeline ocioso não custa nada; remontá-lo custa. |
| Manter uma das duas libs como exemplo vivo | Meia dose do mesmo problema: continua sendo código que o consumidor não pediu e precisa avaliar. O papel de demonstrar a estrutura correta cabe à referência de tasks, que traz o comando com as flags e as tags certas. |
| Corrigir o generator em vez de removê-lo | Seria manutenção num utilitário que duplica `@nx/js:lib` e que nunca funcionou. Os dois defeitos são baratos de corrigir, mas o resultado seria código a mais para manter, com o mesmo efeito do generator nativo. |
| Remover `libs/shared/` por inteiro, sem `.gitkeep` | A entrada `/libs/shared/` do CODEOWNERS ficaria órfã e a estrutura de escopos (`shared`, `backend`, `frontend`) ficaria assimétrica. |
| Editar o ADR-006 em vez de escrever um novo | Um ADR registra a decisão tomada num momento, com o contexto daquele momento. Reescrevê-lo apagaria o registro de que a decisão já foi outra, e por quê. |

## Consequências

- Quem adota o template recebe estrutura e tooling, sem nenhuma biblioteca a
  avaliar ou apagar. `pnpm nx show projects` retorna apenas
  `@mateusmacedo/dmpf-source`.
- O pipeline de release fica ocioso até a primeira lib do consumidor, graças ao
  guard. Um `nx release` chamado à mão nesse intervalo continua abortando com
  exit 1 — quem o invocar fora do workflow precisa saber disso.
- O caminho para criar libs passa a ser exclusivamente o generator do Nx. O
  workspace deixa de ter generator próprio, e `tools/generators/` volta a ser um
  ponto de extensão vazio: quem precisar de um registra a coleção em
  `nx.generators` do `package.json`.
- Some o débito conhecido que o `docs/nx-reference/tasks.md` carregava sobre a
  coleção não resolvível.
- Exemplos de comando e de escopo de commit na documentação passam a usar nomes
  genéricos (`minha-lib`), já que não há projeto real a citar.
- As versões publicadas no Verdaccio continuam instaláveis, mas deixam de
  receber novas versões a partir deste repositório.

## Referências

- SPEC-7VQFX9RP — especificação da remoção
- ARQ-470 — ARQ-470
- [ADR-006](./006-fechamento-port-melhorias.md) — item 9 da matriz, substituído
  por este ADR
- [ADR-001](./001-baseline-monorepo.md) — descrevia as duas libs como parte do
  baseline
- [ADR-004](./004-workflows-verdaccio-release.md) — split version/publish,
  preservado
- `docs/nx-reference/tasks.md` — comando e flags para criar libs com `@nx/js:lib`
