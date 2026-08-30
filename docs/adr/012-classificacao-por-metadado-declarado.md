# ADR-012: Classificar por metadado declarado, não por convenção de diretório

## Status

Aceito — 2026-08-28. Implementa SPEC-DBTRMM3X.

## Contexto

O DMPF classifica cada unidade de código verificável em um dos seis blocos
arquiteturais — `domain library`, `application service`, `app`, `port`,
`provider` e `contract package` (RFC §4.1) — para então provar que as
dependências apontam na direção permitida. A pergunta desta decisão é anterior a
essa prova: de onde o verificador tira a classificação de cada unidade.

A resposta óbvia — ler o nome do diretório — não sobrevive ao acervo real. No
inventário de dez repositórios em Go e TypeScript, o diretório `domain/` designa
três coisas incompatíveis (RFC §4.4): em `backoffice-procap-services` guarda
entidades e invariantes de negócio, sem framework nenhum, e é de fato uma domain
library; em `golibs` guarda o modelo da capability técnica da própria biblioteca,
que não é domain library de negócio; em `telesena-live-services` guarda DTOs
anotados com `@nestjs/swagger` e `class-validator`, que são contract package ou
app. Um classificador que lesse o nome marcaria os três como domain library e
erraria em dois. O caso inverso também ocorre: `rendafacil-services` não tem
diretório `domain/` algum e ainda assim carrega regra de negócio, espalhada por
`apps/*/modules/**` junto de GORM, SQS e HTTP. A ausência do nome não prova a
ausência do bloco.

O layout físico tampouco coincide com a fronteira de camada. Em TypeScript,
`apps/api` costuma ser um único projeto Nx com `src/{domain,application,infra}`
dentro — a fronteira de camada mora abaixo da fronteira de projeto —, e há BFF
sem Nx e sem `domain/` (RFC §3.4). Não existe uma unidade física estável na qual
ancorar a classificação.

Quatro forças puxam ao mesmo tempo. A classificação precisa ser adotável sobre o
layout que já existe, sem exigir reestruturação prévia dos repositórios. Precisa
impedir que um `git mv` reclassifique um arquivo em silêncio. Precisa tornar toda
mudança de classificação revisável em PR e fazer as lacunas de configuração
falharem fechado, em vez de passarem por conformes. E precisa manter um só
contrato válido para Go e TypeScript. A inferência por convenção de diretório
falha nas três primeiras ao mesmo tempo.

## Decisão

Ancorar a classificação em metadado declarado pelo módulo, nunca inferido do
layout (RFC §4.4). Cada `ownership_module` com código de produção contém um
`metadata_container`: o manifesto `dmpf-units.json`, schema `dmpf/units@1`, na sua
raiz. O manifesto declara cada `verification_unit` com dois campos obrigatórios e
explícitos — `block`, um dos seis valores do conjunto fechado (`domain`,
`application`, `app`, `port`, `provider`, `contract`), e `bounded_context` — mais
o campo `include`, que delimita os caminhos da unidade (RFC §10.1).

Em TypeScript, a `verification_unit` é o root declarado no manifesto, não a pasta
nem o pacote (RFC §3.4). O mesmo schema vale para Go e para TypeScript; muda
apenas o binding de `include` — import paths em Go, globs em TS —, de modo que há
um verificador por stack e um contrato só (RFC §10.1).

Herança não existe: cada unidade declara os seus campos por completo, sem herdar
`block` nem `bounded_context` de diretório pai, de módulo ou de outra unidade,
porque herança recriaria a classificação implícita que §4.4 proíbe (RFC §10.1). E
tudo falha fechado: caminho de produção não coberto por nenhum `include`,
sobreposição entre dois `include`, `canonical_key` duplicada no universo, `block`
fora dos seis valores e manifesto ausente em módulo de produção — todos reprovam
(RFC §3.6, §10.1).

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Inferir a classificação pela convenção de diretório — tudo em `src/domain/**` é `domain`, e assim por diante | Não é revisável: um `git mv` reclassifica o arquivo em silêncio. O manifesto é a única opção em que a reclassificação exige edição visível — um arquivo movido para fora de todos os `include` deixa de pertencer a qualquer unidade e falha fechado, e mudar o bloco passa a ser uma alteração no manifesto, revisável em PR (RFC §3.4, bloco `rationale`; corroborado por §4.4). |
| Usar o pacote npm inteiro — o diretório com `package.json` — como `verification_unit` | Granularidade grossa demais: `apps/api` é um único pacote que reúne as três camadas (`domain`, `application`, `infra`), então a unidade conteria três blocos e perderia o pertencimento único a um bloco (RFC §3.4, tabela de alternativas avaliadas, opção C). |
| Permitir herança de classificação — a unidade herda `block` e `bounded_context` do diretório pai ou de outra unidade | Herança recria a classificação implícita que §4.4 proíbe; sem ela, não há caminho para reintroduzir a inferência que esta decisão recusa (RFC §10.1, "Herança não existe"). |

## Consequências

**Positivas:**

- A classificação torna-se adotável sobre o layout existente, sem reestruturação
  prévia dos repositórios (RFC §3.4).
- A reclassificação deixa de ser silenciosa: mover um arquivo para fora dos
  `include` faz a unidade falhar fechado, e mudar o `block` exige edição visível
  no manifesto, revisável em PR (RFC §3.4).
- Um só contrato de schema para Go e TypeScript, variando apenas o binding de
  `include` — um verificador por stack, um contrato só (RFC §10.1).
- Some o erro sistemático do classificador por nome: o `domain/` que designa três
  coisas distintas no acervo deixa de ser lido como um único bloco (RFC §4.4).
- Fail-closed em toda lacuna — caminho não coberto, sobreposição, chave
  duplicada, valor de `block` desconhecido e manifesto ausente reprovam —,
  impedindo que ausência de configuração seja lida como conformidade
  (RFC §3.6, §10.1).

**Negativas:**

- Custo aceito: cada `ownership_module` de produção passa a manter à mão um
  manifesto e a mantê-lo em dia — trabalho declarativo recorrente que não existia
  antes, e cuja ausência reprova o módulo. É o preço deliberado de trocar
  inferência automática por declaração revisável (RFC §10.1, §3.6).
- O metadado é autodeclarado: a própria unidade declara a classificação que
  deveria restringi-la. Comparar o manifesto com um baseline detecta divergência
  acidental, mas não impede que o autor altere manifesto e código no mesmo
  commit. A mitigação é parcial e é objeto do ADR de autoridade sobre a
  classificação — ADR-013 —, não deste (RFC §10.2).
- Não há autoridade de aprovação estabelecida sobre a qual apoiar a autorização
  distinta: `CODEOWNERS` está ausente em 10 de 10 repositórios inventariados. Até
  o FND-10 definir o processo de governança, o requisito se apoia apenas em
  commit isolado revisado por terceiro (RFC §10.2).
- Adotar sobre o layout existente é a escolha mínima: garantias estruturais mais
  fortes — TS project references — ficam como evolução opcional, não exigida;
  quem quiser essa garantia precisa reestruturar os repositórios por conta
  própria (RFC §3.4).

## Referências

- ADR-010 — regra de dependência e seis blocos. Institui os seis blocos
  (`domain library`, `application service`, `app`, `port`, `provider`,
  `contract package`) e a regra de dependência que consome a classificação: a
  classificação por unidade que este ADR ancora é o dado de entrada do
  classificador total de ADR-010.
- ADR-011 — a `verification_unit` e o binding por stack. Decide qual é a
  unidade que carrega o metadado e como ela é delimitada em cada stack (em Go,
  o pacote e o diretório; em TypeScript, o root declarado no manifesto); a
  classificação que este ADR ancora é atributo dessa unidade.
- ADR-013 — autoridade sobre a classificação. Endereça a mitigação parcial
  declarada nas Consequências: exige autorização distinta da autoria para mudar
  `block` ou `bounded_context`, com fail-closed na ausência de evidência.
- ADR-017 — bounded context declarado e superfície pública. Institui o
  `bounded_context` que este ADR exige como campo obrigatório em cada unidade,
  ao lado de `block`.
- RFC de fundação do DMPF — `docs/dmpf/rfc-dmpf-foundation-v0.1.md`: §3.4 (binding
  em TypeScript), §3.6 (casos degenerados), §4.1 (os seis blocos, instituídos por
  ADR-010), §4.4 (o nome não classifica), §10.1 (schema do metadado) e §10.2
  (trust model).
- SPEC-DBTRMM3X — spec que esta série de ADRs implementa.
- ARQ-448 — https://lider-cap.atlassian.net/browse/ARQ-448 (destino da série de
  ADRs do DMPF).
