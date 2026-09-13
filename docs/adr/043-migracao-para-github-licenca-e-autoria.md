# ADR-043: Migrar a plataforma para o GitHub, sob autoria e licença próprias

## Status

Aceito — 2026-09-11.

Supersede o [ADR-005](./005-plataforma-gitea.md) por inteiro e, do
[ADR-004](./004-workflows-verdaccio-release.md), os itens de alvo de publicação
e de runner. Supersede do [ADR-008](./008-identidade-automacao-release.md) a
identidade de automação e o domínio de e-mail do bot; a decisão de remover as
proteções de branch e tag segue válida e não é revista aqui.

## Contexto

O projeto nasceu hospedado no Gitea de uma organização contratante e herdou
dela toda a cadeia de identidade: o import path dos módulos Go, o escopo npm,
o registry de publicação, o runner de CI, o rastreador de issues e o e-mail dos
commits. Encerrado o vínculo, nada disso continua acessível — e parte já nem
existe.

Três dependências caíram de uma vez:

1. **O host.** `gitea.<contratante>` sai do alcance. Com ele vai o remote, a
   API `/api/v1` que o `create-release` chamava e o runner `gitea-runner`, que
   é um label de máquina autogerida, não um runner público.
2. **O registry.** O `publish-libs` publicava num Verdaccio privado, autenticado
   por OIDC contra uma role da AWS da contratante, com a credencial num
   parâmetro do SSM. Os três elos — registry, role e parâmetro — pertencem a ela.
3. **Os reusables.** `detect-apps` e `publish-libs` viviam num repositório
   `actions-templates` da mesma organização. Um workflow que referencia
   `owner/repo/.github/workflows/x.yaml@main` falha de forma opaca quando o
   repositório some: o erro aparece no momento do run, não no push.

Havia ainda material que não era só nome. O acervo trazia inventários AS-IS de
dez repositórios internos da contratante — estrutura de pacotes, decisões,
caminhos de máquina e autoria de terceiros. Publicar isso sob licença MIT num
repositório aberto distribuiria análise de sistemas proprietários dela.

## Decisão

**Plataforma: GitHub**, em `github.com/mateusmacedo/dmpf`. O binário `gh` volta
a ser o caminho padrão de automação — é o que o ADR-005 havia proibido, pela
premissa, agora falsa, de que o host não era o GitHub. O `create-release` troca
a chamada `curl` à API do Gitea pelo comando de abertura de PR do `gh`. Os
cinco jobs em `gitea-runner` passam a `ubuntu-latest`.

**Identidade dos módulos:** o import path Go passa a `github.com/mateusmacedo/dmpf/…`
e o escopo npm a `@mateusmacedo/`. Em Go o import path é a chave canônica da
unidade, e a RFC a exige estável; a troca foi feita de uma vez, em todos os
módulos, antes de qualquer nova classificação de baseline — um baseline gravado
com o path antigo teria de ser refeito.

**Publicação: GitHub Packages.** O escopo npm precisa ser o owner do
repositório — é requisito do registry, não convenção — e `@mateusmacedo/` já
satisfaz. A autenticação passa a ser o `GITHUB_TOKEN` do próprio run, com
`packages: write`; some a cadeia OIDC → role → SSM.

**Os reusables são internalizados.** `detect-apps.yaml` e `publish-libs.yaml`
passam a viver em `.github/workflows/` e os callers os referenciam por
`./.github/workflows/…`. Dependência externa de workflow que não se pode
versionar nem auditar quebra sem aviso; internalizar custa cerca de 210 linhas
e elimina a classe inteira de falha. Os outros vinte e um reusables do template
— AWS, Terraform, ECS, EKS, SonarQube — não são trazidos: nenhum é usado aqui.

**Licença MIT**, copyright de Mateus Macedo Dos Anjos. O `package.json` raiz já
declarava `MIT`, mas sem arquivo `LICENSE`, sem `author` e sem `repository` —
declaração sem lastro. Os três passam a existir.

**O material proprietário sai do repositório.** Os dezesseis inventários AS-IS
detalhados e as duas specs que os originaram são removidos. O FND-01 — a
síntese que a RFC cita como baseline e que `tools/dmpf-verify-emit.mjs` exige
como documento — **permanece**, anonimizado: é peça da fundação, no mesmo nível
da RFC, e removê-lo quebraria a cadeia de rastreabilidade sem ganho de
confidencialidade, já que não traz caminho de máquina nem autoria de terceiros.
Os sistemas que a RFC contrasta como evidência ganham pseudônimos estáveis
(`legado-live-services`, `legado-rendas-services`, `legado-sync-services` e
assim por diante): um rótulo único por sistema preserva a distinção que o texto
normativo faz entre eles, o que "sistema legado" genérico destruiria.

**As URLs do rastreador são removidas, os identificadores ficam.** Os links de
`browse/ARQ-NNN` apontam para um tenant inacessível e expõem o domínio da
contratante; os IDs `ARQ-NNN` seguem no texto, porque são o que amarra cada ADR
e spec à decisão que os originou.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Manter os reusables apontando para `actions-templates` sob a conta nova | O repositório não existe sob essa conta; o workflow falharia no run, não no push |
| Publicar no npm público em vez do GitHub Packages | Exige `NPM_TOKEN` em segredo do repositório; o GitHub Packages autentica com o token nativo do run |
| Desligar a publicação de libs | Hoje nada é publicável (o único `type:lib` é Go, `private: true`), mas desligar removeria a esteira que a primeira lib TypeScript vai precisar |
| Usar `.mailmap` para a autoria dos commits | Só altera a exibição local; o e-mail antigo continua nos objetos e é o que a plataforma lê |
| Reescrever os ADRs da era Gitea no lugar de supersedê-los | Um ADR registra a decisão tomada naquele momento; reescrevê-lo apaga o histórico que ele existe para preservar |
| Remover o FND-01 junto com os inventários | É citado pela RFC como baseline e exigido por `tools/dmpf-verify-emit.mjs`; removê-lo quebra a rastreabilidade sem reduzir exposição |
| Substituir todos os sistemas legados por um rótulo genérico único | A RFC contrasta os sistemas entre si; um rótulo só tornaria ininteligíveis as passagens que comparam abordagens |

## Consequências

**Positivas:**

- Nenhuma dependência de infraestrutura de terceiro permanece na esteira: host,
  registry, runner, reusables e credenciais são todos do próprio repositório ou
  do GitHub.
- A publicação perde três elos (OIDC, role da AWS, parâmetro no SSM) e passa a
  depender apenas do token que o run já tem.
- O repositório fica distribuível: licença declarada com arquivo, autoria
  explícita e sem material proprietário de terceiro.
- Os reusables ficam auditáveis e versionados junto com quem os chama.

**Negativas:**

- Todo import path Go muda. Qualquer consumidor externo do módulo precisa
  atualizar — não há redirecionamento em Go.
- O histórico de commits ainda carrega o e-mail corporativo antigo. Corrigir
  exige reescrever a autoria, o que muda todos os SHAs e invalida os seis
  identificadores de commit citados na documentação.
- Os inventários AS-IS deixam de ser consultáveis a partir do repositório. As
  decisões que se apoiavam neles mantêm o argumento, mas perdem a evidência
  detalhada.
- O `cd-dev-hmg.yml` fica com um exemplo comentado apontando para um
  `deploy-eks.yaml` que ainda não existe aqui. Ele precisa ser internalizado
  quando o CD for ligado.
