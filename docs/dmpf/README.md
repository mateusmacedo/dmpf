# DMPF — artefatos promovidos

Índice versionado da fundação **DMPF** (épico [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436)).

Os drafts de trabalho continuam em `plans/references/` (gitignored). Quando
estão prontos para revisão, são **promovidos** para cá; a aprovação formal
ocorre no PR (reviewers de Plataforma/Arquitetura). Specs permanecem em
`docs/specs/`. ADRs do DMPF, quando acionados, usam a faixa `docs/adr/010`–`024`
(contínua aos `001`–`009` do template).

## Artefatos

| Artefato | Status | Spec / Issue |
|----------|--------|--------------|
| [inventario-as-is.md](./inventario-as-is.md) | **baseline candidato** (promovido para revisão; AC externo aberto) | [SPEC-K9H204F1](../specs/SPEC-K9H204F1-dmpf-inventario-as-is.md) / [ARQ-438](https://lider-cap.atlassian.net/browse/ARQ-438) |
| [rfc-dmpf-foundation-v0.1.md](./rfc-dmpf-foundation-v0.1.md) | **`normativo`** (aceito em 2026-08-14) | [SPEC-8YVF0RR5](../specs/SPEC-8YVF0RR5-dmpf-rfc-limites-deps.md) / [ARQ-439](https://lider-cap.atlassian.net/browse/ARQ-439) |
| [upr-decision-mensagens.md](./upr-decision-mensagens.md) | **promovido para revisão** (adição à RFC pela âncora ANC-01, sem editá-la) | [SPEC-8MNDEWDP](../specs/SPEC-8MNDEWDP-dmpf-upr-decision-mensagens.md) / [ARQ-440](https://lider-cap.atlassian.net/browse/ARQ-440) |
| [uow-inbox-outbox.md](./uow-inbox-outbox.md) | **promovido para revisão** (adição à RFC pela âncora ANC-02, sem editá-la) | [SPEC-7PJ5WVCS](../specs/SPEC-7PJ5WVCS-dmpf-uow-inbox-outbox.md) / [ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441) |
| [cloudevents-protobuf-buf.md](./cloudevents-protobuf-buf.md) | **promovido para revisão** (adição à RFC pela âncora ANC-03, sem editá-la) | [SPEC-7H08RZDG](../specs/SPEC-7H08RZDG-dmpf-cloudevents-protobuf-buf.md) / [ARQ-442](https://lider-cap.atlassian.net/browse/ARQ-442) |

A RFC obriga: as regras nela escritas valem para todo trabalho novo do DMPF.

Duas ressalvas ficam registradas na própria RFC. A revisão por área não chegou a
produzir parecer (RFC §14.6), e a promoção ocorreu com o inventário AS-IS ainda
em `baseline candidato` (RFC §1.5) — se a aprovação do inventário alterar
denominadores ou conclusões, as decisões que citam a evidência afetada devem ser
revisitadas. Para a `0.2` em diante, revisão por área e reconciliação com
baseline aprovado voltam a ser exigidas (RFC §14.3).

### Sobre `upr-decision-mensagens.md`

Adiciona à RFC pelo mecanismo de âncora (ANC-01, RFC §12.3), sob as regras de
monotonicidade M1–M4: detalha o bloco `domain library` sem editar a RFC e sem
incremento de versão. Três pendências ficam registradas no próprio artefato:

- a revisão dos exemplos por um representante de cada stack, gate externo não
  bloqueante (§10.3);
- os vetores de equivalência do desfecho e o diagnóstico estável por regra,
  encaminhados a FND-09 e ainda inexistentes (§10.4);
- a marcação de Parte-1 §§5 e 6 como consolidados na tabela de RFC §14.4, que o
  artefato não pode fazer sozinho (§1.5).

### Sobre `uow-inbox-outbox.md`

Adiciona à RFC pela âncora ANC-02 (RFC §12.3), sob as mesmas regras de
monotonicidade M1–M4. Diferente da ANC-01, que recorta um bloco, a ANC-02
recorta um **mecanismo** que atravessa `application service`, `provider` e `app`
— por isso cada regra do artefato declara o bloco a que se aplica (§1.3), e a
tabela de sucessão da Parte-1 §§9–10 é por subseção, com coluna de ressalva
(§1.5).

O artefato aciona **dois** ADRs, sem redigir nem aceitar nenhum: `ADR-DMPF-K`
(mecanismo de relay, polling × CDC), exigido pelo próprio registro da ANC-02, e
`ADR-DMPF-L` (bloco do mapeamento e momento da serialização), que responde à
obrigação delegada por escrito em `upr-decision-mensagens.md` §6.3. Os
identificadores são provisórios: a numeração definitiva é do FND-11.

As 64 regras `normativo` substantivas têm **ID estável** (`BLK`, `UOW`, `OBX`,
`INB`, `GAR`), marcado no bloco que enuncia cada uma e indexado em §11.2 — é
por esse ID que o FND-09 vai nomear o cenário que a verifica.

Três pendências ficam registradas no próprio artefato (§11.5):

- refletir na tabela de RFC §14.4 a sucessão de Parte-1 §§9–10, preservando
  §10.7 e §10.8 como vigentes sob ANC-04;
- recolher no glossário de RFC §14.1 os vinte termos que o artefato introduz
  (§11.4);
- converter os doze failure modes de §7.3 em cenários executáveis, com
  diagnóstico estável e par de vetores por regra, encaminhado a FND-09.

### Sobre `cloudevents-protobuf-buf.md`

Adiciona à RFC pela âncora ANC-03 (RFC §12.3), sob as mesmas regras de
monotonicidade M1–M4. Como a ANC-01, a ANC-03 recorta um **bloco** —
`contract package` —, mas com uma tensão própria: o bloco é declarativo, e quem
materializa o contrato em bytes é o `provider` (célula 30 de RFC §7.4). Por isso o
artefato declara, em §1.3, que o **sujeito** de toda regra é o contrato, nunca o
comportamento de outro bloco: a forma do envelope obriga aqui, e quem preenche
cada campo continua sendo de FND-04.

Duas particularidades diferenciam esta adição das anteriores.

A primeira é que o artefato é **devedor** dos dois precedentes: FND-03 e FND-04
lhe delegaram, por escrito, dez obrigações. A matriz de §2.3 lista cada uma com a
fonte em `arquivo:linha`, a seção que a trata, como conferir e o estado — **nove
`quitada` e uma `encaminhada`**. A encaminhada é a escolha de registry, que a
própria âncora manda decidir por ADR; declarar fronteira não conta como quitação.

A segunda é que o **assunto registrado na âncora é maior do que a entrega**: a
ANC-03 nomeia «Protobuf, CloudEvents, OpenAPI e AsyncAPI», e o artefato normatiza
os dois primeiros. Parte-1 §7.6 e §7.7 seguem vigentes, sem sucessor e sem
transferência a outra âncora — a lacuna é nomeada em §10.4 e afeta a condição de
fechamento da ANC-03. A sucessão de Parte-1 §7 é declarada por subseção, com duas
**parciais**: §7.1 (os usos por transporte ficam com ANC-04) e §7.2 (os diretórios
`openapi/` e `asyncapi/` e os descriptors permanecem vigentes).

O artefato aciona **dois** ADRs, sem redigir nem aceitar nenhum: `ADR-DMPF-M`
(codec de wire e modalidade de payload da primeira major) e `ADR-DMPF-N` (escolha
de registry e autoridade de validação). Ambos são exigidos pelo próprio registro
da ANC-03 e confirmados por RFC §13.3. Os identificadores são provisórios,
continuando a série que o FND-04 deixou em `ADR-DMPF-L`: a numeração definitiva é
do FND-11.

As 64 regras `normativo` substantivas têm **ID estável** (`ENV`, `PTB`, `BUF`,
`REP`, `INT`), marcado no bloco que enuncia cada uma e indexado em §10.2. §10.1
declara, regra a regra, o que é decidido mecanicamente e o que não é: os quatro
gates de §6.6 — `buf format`, `buf lint`, `buf breaking` e dupla geração — decidem
`PTB-01`/`PTB-02`/`PTB-04`, `PTB-05` a `PTB-09`, `BUF-01` a `BUF-12` e `REP-01`/
`REP-02` sem julgamento humano. O perfil do envelope (`ENV-08` a `ENV-13`) não tem
gate neste artefato — é propriedade de instância, e a lacuna do instrumento é
pendência registrada; `PTB-10` depende da certificação de toolchain e `PTB-11` do
oráculo de FND-09.

Três decisões deste artefato merecem leitura atenta na revisão:

- a modalidade de payload é **única** nesta major (`proto_data` com
  `google.protobuf.Any`), e as outras duas do `oneof` oficial — `binary_data` e
  `text_data` — ficam vedadas: nenhuma como fallback, nenhuma como exceção por
  contexto (§4.2);
- a fórmula do `payload_hash` é decidida sob H1 e H2 de FND-04 `INB-13`, com duas
  alternativas avaliadas: hash SHA-256 sobre os bytes transportados, sem
  reserialização, com a projeção canônica descartada e o motivo registrado. H2 é
  satisfeita pela fórmula; **H1 é satisfeita sob condição** — a byte-preservação
  pelos transportes é dependência declarada sobre FND-06, registrada como
  pendência (§4.3, §10.4);
- o bootstrap do baseline de `buf breaking` é uma **máquina de estados** de dois
  estados com transição única e autorização distinta da autoria, com três vetores
  negativos — remoção, renomeação e falsificação do marcador (§6.4).

Dez pendências ficam registradas no próprio artefato (§10.4), entre elas: o
recorte incompleto da ANC-03 acima; o realinhamento com FND-09, cuja spec hoje
exige «mesmo byte» como critério geral do round-trip e reivindica o ownership da
fixture (§8.5); e a ausência de campo dedicado, no schema mínimo da outbox de
FND-04 §4.1, para três atributos que o envelope torna obrigatórios —
`correlationid`, `causationid` e `traceparent` (§4.1).

## Evidências do inventário

Há **13 relatórios** cobrindo **10 repositórios** (alguns monorepos têm mais de
um corte) em [`docs/specs/SPEC-K9H204F1/`](../specs/SPEC-K9H204F1/).
