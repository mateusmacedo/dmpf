---
id: SPEC-CWBA4BPV
slug: porta-object-storage-worm
title: DMPF — Porta de object storage com versionId, retenção WORM e legal hold; provider S3; floci ≥ 2.2.0 com S3 e KMS
stage: backlog
priority: P1
depends_on: [SPEC-XMNBMY50, SPEC-VJMM2DE5]
ticket_url: https://linear.app/mmda/issue/DEVS-95/spec-cwba4bpv-dmpf-porta-de-object-storage-com-versionid-retencao-worm
subtask_urls: []
created: 2026-10-12
---

# SPEC-CWBA4BPV: Porta de object storage com retenção WORM

## Resumo

Dar ao kernel uma porta de object storage que devolve o `versionId` de cada
escrita, aplica retenção WORM (COMPLIANCE e GOVERNANCE) e legal hold, e não
oferece nenhuma operação que apague versão sob retenção, encurte retenção ou
contorne o modo. A porta nasce com fake em memória e kit de conformidade no
kernel (Onda 1) e segue com o provider S3, em módulo novo no molde do `sqs`, e
com o floci ≥ 2.2.0 com S3 e KMS na infra (Onda 2).

A origem é o consumidor externo antecipa-market, SPEC-GXGASP24 (DEVS-76): uma
plataforma de antecipação de recebíveis sobre o kernel `1.0.0-rc.4`, com AWS como
nuvem-alvo e floci como emulador, que grava selos periódicos e evidência bruta de
um ledger de compliance em bucket com Object Lock.

Como mantenedor do kernel, quero que a imutabilidade do storage seja um contrato
testável e igual em memória, no emulador e na AWS, para que nenhum contexto
reimplemente retenção por convenção. A entrega segue duas ondas, uma por PR; a
Onda 2 só começa depois das investigações P0 da seção Requisitos.

## Contexto

- **Problema**: o kernel não tem porta, provider, fake nem kit de object storage.
  - A capability `io.storage` já cobre «cliente de objeto»
    (`docs/dmpf/rfc-dmpf-foundation-v0.1.md:747`, ADR-015), mas toda
    declaração dela no workspace é do pgx (`libs/backend/go/postgres/dmpf-units.json:13-18`,
    repetida no `testkit` e nos três contextos). O `providerkit`
    cobre UoW, repositório, inbox e outbox (`libs/backend/go/testkit/providerkit/doc.go:1-4`).
  - O único SDK da AWS do kernel é o do `sqs` (`libs/backend/go/sqs/go.mod:6-9`);
    nenhum outro `go.mod` o requer.
  - A infra tem floci 2.0.1, descrito como emulador de SQS e SNS:
    `infra/local/compose/floci.yml:1-7`, `infra/test/compose.yml:84-94` (digest fixo)
    e `.github/workflows/dmpf-go-ci.yml:241-258`. O ADR-039 escolheu o floci para o
    hop SNS → SQS e vetou o LocalStack
    (`docs/adr/039-providers-de-transporte-sink-e-gesto-de-release.md:112-116`).
- **Impacto**: sem a porta, o consumidor escreve o cliente S3 dentro do contexto,
  sem fake nem kit compartilhados, e a retenção depende da convenção de cada autor.
- **Escopo de plataforma**: o selo do Compliance Ledger do consumidor é de plataforma e
  não tem tenant (SPEC-MSA607GT, chave e selo globais). Uma loja que exigisse tenant em
  toda operação o recusaria, e dar ao selo um tenant sintético é vedado (IDN-20).
- **Normas conciliadas** (`docs/dmpf/contexto-erros-seguranca.md`): DAT-08 (cifra em
  repouso verificável na superfície armazenada, `:1464`), DAT-11 (menor privilégio por
  superfície, inclusive armazenamento de objeto, `:1492`), DAT-14 a DAT-18 (retenção
  efetiva ≤ teto externo de origem declarada; o teto mais estrito prevalece; sem teto
  não é retenção indefinida; purga com evidência, `:1511-1515`), DAT-25 (auditoria com
  integridade própria, `:1560`), IDN-11 a IDN-14 (escopo de tenant que não depende de
  convenção, `:878-881`) e IDN-16 a IDN-20 (operação de plataforma: declara o que exige,
  cadeia sem sujeito, alcance declarado, sem tenant inventado, `:926-930`).
- **Fatos externos, conferidos em 12/10/2026**:
  - S3 Object Lock exige bucket versionado. No modo COMPLIANCE nenhum usuário, root
    inclusive, apaga ou sobrescreve a versão, e a retenção não encurta nem o modo muda. O GOVERNANCE
    exige `s3:BypassGovernanceRetention` e o cabeçalho
    `x-amz-bypass-governance-retention:true`. O modo sobe de GOVERNANCE a COMPLIANCE e nunca desce: a AWS descreve o GOVERNANCE como ensaio de um prazo COMPLIANCE (https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock-overview.html). O legal hold não tem prazo e se põe e
    se tira com `s3:PutObjectLegalHold`. Retenção e hold protegem a versão, não a
    chave: um DELETE sem `versionId` só cria delete marker. Habilitado o Object Lock,
    o bucket não o desabilita nem suspende o versionamento. `PutObject` com retenção
    exige `Content-MD5` ou `x-amz-sdk-checksum-algorithm`. A chave de condição
    `s3:object-lock-remaining-retention-days` limita a retenção na política do bucket
    (https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html e
    https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock-managing.html).
  - As notas da 2.2.0 do floci (06/10/2026) listam «s3: allow delete markers and new
    versions over object-locked versions (#4390)», que fecha a issue #4389, aberta
    contra a 2.1.0. A tag `floci/floci:2.2.0` existe no Docker Hub, com digest de lista
    `sha256:e97cd0c1dc2aa14e7697fb5ef5018404c4d6345169dbd276315f0bed2c7b0520`, e o README
    da 2.2.0 lista S3 (com Object Lock) e KMS entre os serviços em processo
    (https://github.com/floci-io/floci/releases/tag/2.2.0 e
    https://github.com/floci-io/floci/issues/4389).
- **Não verificado nesta redação** (cada item tem requisito de investigação): o
  comportamento da 2.2.0 além do que as notas da release afirmam; as divergências que
  o consumidor externo reportou na 2.0.1 (DELETE sem `versionId` sob COMPLIANCE
  recusado, `PutBucketVersioning Suspended` aceito em bucket com lock, lock aceito sem
  data de retenção e em bucket sem versionamento); a expiração real da retenção no
  floci; Terraform e IaC contra o floci.

<constraints>
- [P0] Nenhuma operação da porta apaga versão sob retenção ou hold, encurta retenção, rebaixa o modo (COMPLIANCE para GOVERNANCE) ou contorna o GOVERNANCE; o provider da porta nunca envia `x-amz-bypass-governance-retention`. O bypass só existe no caminho administrativo segregado: credencial própria, fora da role da app, só para versão GOVERNANCE e com evidência (DAT-16, DAT-18).
- [P0] Toda referência carrega o `versionId`; a porta não tem delete sem versão, porque o delete marker do S3 não remove nada.
- [P0] O bloco `port` permanece puro (RFC §6.2): a porta usa só `context`, `errors`, `[]byte` e `Instant`, sem `time`, SDK ou pacote ausente da tabela de stdlib pura.
- [P0] Retenção é sempre datada e sob o teto declarado na construção da loja, na escrita, na prorrogação e na promoção a COMPLIANCE, que não se encurta; sem teto declarado, a loja recusa gravar com retenção (DAT-14, DAT-17).
- [P0] A loja é construída com escopo declarado, e escopo ausente recusa a construção (IDN-17). No escopo de tenant, o tenant vem do `ExecutionContext`, prefixa a chave física e, ausente, recusa a operação (IDN-14, IDN-15, IDN-20). No escopo de plataforma, a operação não exige sujeito nem tenant (IDN-16), roda na cadeia de plataforma (IDN-18), usa o prefixo físico fixo de plataforma, com alcance declarado na construção (IDN-19), e nunca lê nem inventa tenant (IDN-20).
- [P0] O mesmo kit roda contra o fake, o floci e o S3 real; a cláusula que um subject não exerce fica em `Skipped` com nome, nunca fora do kit.
- [P0] A garantia WORM só se prova contra S3 real: o floci dá paridade de ambiente, e o teste de contrato em hmg não é opcional.
- [P1] Unidade nova exige `include` no manifesto (`DMPF-U001`) e entra no baseline por `--write-baseline` (`DMPF-T001`), feito por quem mantém o kernel.
- [P1] Arquivo gerado pelo `init` só muda pelo template em `tools/dmpf-plugin/templates/init/` (ADR-060); projetos Nx distintos vão em commits separados.
</constraints>

## Requisitos

### Funcionais

#### Investigações (antes da Onda 2)

- [ ] **[P0] Investigar o comportamento do floci 2.2.0 na prática**: subir `floci/floci:2.2.0` e rodar as cláusulas do kit da Onda 1 sobre S3 com Object Lock; registrar quais divergências da 2.0.1 persistem (DELETE com e sem `versionId` sob COMPLIANCE, versionamento `Suspended` em bucket com lock, lock sem data e em bucket sem versionamento, cabeçalhos de cifra no `HeadObject`, checksum SHA-256 no `PutObject`, promoção de GOVERNANCE para COMPLIANCE e recusa do rebaixamento). Cada divergência que persistir vira cláusula `Skipped` nomeada no kit contra o floci, registrada no ADR novo.
- [ ] **[P0] Investigar a expiração real da retenção no floci**: se um `RetainUntilDate` vencido libera o `DeleteObject` com `versionId`, qual o menor `RetainUntilDate` que o floci e o S3 real aceitam e se o estado de lock sobrevive a reinício conforme `FLOCI_STORAGE_MODE`. Sem resposta, a cláusula de purga após vencimento roda só contra o fake e a lacuna fica declarada.
- [ ] **[P0] Investigar Terraform e IaC contra o floci**: se o provider AWS do HashiCorp cria no floci 2.2.0 um bucket com Object Lock e uma chave KMS (guia do floci em `docs/getting-started/terraform.md`), para decidir se hmg e os ambientes locais provisionam por IaC. A spec só entrega o bucket que cada teste cria.

#### Onda 1 — porta, fake e kit de conformidade (sem nuvem)

- [ ] **[P0] Porta em `ports`**: novo `libs/backend/go/ports/objectstore.go` com quatro interfaces pequenas, `ObjectWriter` (`Put`), `ObjectReader` (`Get`, `Head`), `RetentionManager` (`Retain`, `SetLegalHold`) e `ObjectPurger` (`Purge`), para que o privilégio do chamador seja o da interface que ele recebe (DAT-11). `ObjectStore` as compõe. O corpo é `[]byte` e os instantes são `Instant`.
- [ ] **[P0] `versionId` em toda referência**: `Put` devolve `ObjectVersion{Key, Version, ContentSHA256, Size, StoredAt}`; as demais operações recebem `ObjectRef{Key, Version}` e recusam `Version` vazio com `ErrVersionRequired`. A porta não tem delete sem versão.
- [ ] **[P0] Integridade na escrita**: `PutRequest` carrega o `ContentSHA256` calculado pelo chamador; a escrita recusa corpo divergente com `ErrIntegrity`, sem gravar, e devolve o hash verificado. No S3 o mesmo hash viaja como checksum da requisição e atende à exigência de checksum do `PutObject` com retenção.
- [ ] **[P0] Retenção datada e sob teto**: `Retention{Mode, Until}` com `Mode` em `COMPLIANCE` ou `GOVERNANCE`. A escrita recusa `Until` ausente ou vencido (`ErrRetentionUnbounded`) e `Until` acima do teto (`ErrRetentionAboveCeiling`). O teto e a origem dele (DAT-15) são declarados na construção da loja, e a loja sem teto recusa gravar com retenção (DAT-17).
- [ ] **[P0] Retenção só cresce e só sobe de modo**: `Retain` aceita `Until` maior que o corrente, no mesmo modo, e a promoção de GOVERNANCE para COMPLIANCE com `Until` maior ou igual ao corrente; o S3 permite a promoção e veda o inverso. `Until` menor que o corrente, ou igual sem mudança de modo, devolve `ErrRetentionNotExtended`; o rebaixamento de COMPLIANCE para GOVERNANCE devolve `ErrRetentionModeChange`; `Until` acima do teto devolve `ErrRetentionAboveCeiling`, também na prorrogação e na promoção. COMPLIANCE nunca recebe `Until` acima do teto declarado (DAT-14): como o S3 não o encurta, ele nasce com o prazo declarado pelo contexto e só é prorrogado, dentro do teto, e um teto superveniente mais estrito não alcança uma versão COMPLIANCE já gravada (risco residual registrado no ADR novo; onde o teto pode apertar, o contexto grava em GOVERNANCE). Nenhum método da porta contorna GOVERNANCE.
- [ ] **[P0] `Purge` só de versão livre**: devolve `ErrObjectLocked` enquanto houver retenção vigente ou hold e, ao apagar, devolve `PurgeReceipt{Ref, PurgedAt, RetainedUntil}` (DAT-18, no molde de `libs/backend/go/postgres/purge.go:18-26`). A loja não grava auditoria: quem chama persiste o recibo na auditoria própria (DAT-25).
- [ ] **[P0] Legal hold independente, de origem declarada**: `SetLegalHold(ctx, ref, LegalHold{On, Reason})` põe ou tira o hold sem tocar a retenção; `Reason` é obrigatório ao pôr e é a origem declarada da obrigação (DAT-15), e a operação devolve `HoldReceipt{Ref, On, Reason, At}`. O hold não tem prazo, então a loja mede o teto só sobre a retenção datada; mas ele não é uma exceção à DAT-14 que a loja ou o contexto declare para si: é obrigação de origem declarada, e manter uma versão sob hold além do teto só é conforme depois de um ADR novo do kernel que emende a DAT-14 para admiti-lo. Até lá o ADR desta spec registra a lacuna, e o contexto responde por tirar o hold quando a obrigação cessa.
- [ ] **[P0] Escopo declarado da loja**: a loja se constrói com `Scope` (`ScopeTenant` ou `ScopePlatform`), e escopo ausente recusa a construção (IDN-17). No escopo de tenant, resolve o tenant do `ExecutionContext` como `libs/backend/go/postgres/table.go:233-243`, prefixa a chave física com `tenant/<id>/` e recusa tenant ausente ou com `/`; referência de outro tenant não resolve e devolve `ErrObjectNotFound`, que desembrulha para `ports.ErrNotFound` (IDN-13). Como a chave física só se monta com o tenant do contexto, a leitura cruzada é impossível por construção (IDN-14), e o registro da tentativa (IDN-12) cabe a quem autoriza a chamada, acima da loja. No escopo de plataforma, a operação declara que não exige sujeito nem tenant (IDN-16) e roda na cadeia de plataforma (IDN-18), cuja autorização por identidade de workload é de quem chama; a loja usa o prefixo físico fixo `platform/`, que nenhum id de tenant alcança, nunca lê nem inventa tenant (IDN-20) e recusa contexto que traz tenant (`ErrScopeMismatch`). O alcance é só esse prefixo, declarado na construção (IDN-19), e os recibos de `Purge` e `SetLegalHold` alimentam a auditoria do chamador (DAT-25). A referência de um escopo não resolve no outro (`ErrObjectNotFound`).
- [ ] **[P0] Cifra declarada e verificável**: a loja se constrói com a cifra declarada (`Encryption{Algorithm, KeyRef}`) e recusa gravar sem ela (DAT-03, DAT-08); `Head` devolve a cifra da versão armazenada, para a DAT-08 se conferir na superfície, e não no código que escreve.
- [ ] **[P0] Fake em `memory`**: `libs/backend/go/memory/objectstore.go` implementa a porta com `ports.Clock` injetado para vencer retenção sem esperar e devolve os mesmos erros. A unidade `kernel/provider-memory` não ganha dependência externa.
- [ ] **[P0] Kit `providerkit.ObjectStore`**: `libs/backend/go/testkit/providerkit/objectstore.go` devolve `Verdict` com `Diagnostic{Clause, Rule}` e `Skipped` nomeado (`providerkit/suite.go:7-43`), consumido por `tb.Require`. O subject abre a loja no escopo pedido, troca de tenant, declara teto e cifra esperados e, se puder, avança o relógio. As cláusulas estão em Design.
- [ ] **[P1] Subject do fake**: `providerkit.MemoryObjectStore()`, no molde de `MemoryUnitOfWork` (`providerkit/memory.go:15`), e teste do fake contra o kit em `memory`.

#### Onda 2 — provider S3 e infra

- [ ] **[P0] Módulo `libs/backend/go/s3`**: unidade `kernel/provider-s3` (`dmpf-units.json` com `service/s3` em `io.storage`, na faixa de `aws-sdk-go-v2` do `sqs`, `sqs/dmpf-units.json:13-18`), `package.json` privado na versão do kernel e `project.json` com tags `type:lib`, `scope:backend`, `stack:go`, `layer:providers` e `test-race` com `-tags=integration` (`sqs/project.json:38-50`). Entra no `go.work` por `modsync` e no grupo `go-libs` pelo seletor de diretório (`nx.json:81-86`), sem editar o `nx.json`; a unidade entra em `shared_kernel_units` (`tools/dmpf-baseline/units-baseline.json:526-534`).
- [ ] **[P0] Mapeamento porta → S3**: `Put` vira `PutObject` com checksum SHA-256, modo e data de retenção e a cifra declarada; `Retain` vira `PutObjectRetention`, inclusive na promoção a COMPLIANCE; `SetLegalHold` vira `PutObjectLegalHold`; `Purge` vira `DeleteObject` com `VersionId`; `Get` e `Head` levam o `VersionId`. Um teste com middleware do SDK prova que nenhuma operação de `s3.Store` envia `x-amz-bypass-governance-retention`.
- [ ] **[P0] Pré-condição do bucket**: `Ready(ctx)` confere versionamento `Enabled` e Object Lock habilitado, e a loja recusa operar sem isso (`ErrBucketNotImmutable`). O provider só detecta bucket mal configurado, porque o S3 real já veda desabilitar o lock depois de habilitado. Um teste de integração cria um bucket sem versionamento e outro sem Object Lock e prova a recusa, o que o floci pode aceitar.
- [ ] **[P0] Config, resiliência e telemetria no molde do `sqs`**: `Config` com `AWS`, `Bucket`, `Endpoint`, `InsecureForDevelopmentOnly`, `UsePathStyle` (só com `Endpoint` explícito), cifra, teto, `Scope`, `Clock` e `Sheet`; `Validate` recusa config incompleta e endpoint sem TLS (`sqs/config.go:27-28,49`); `Put` sem retry interno do SDK e demais operações com a retentativa autorizada na ficha (RES-05, RES-21, RES-27); classificador de erro da AWS para as categorias do kernel, fechado por omissão (ERR-11; `sqs/classifier.go:28-50`); telemetria sem corpo nem chave física (DAT-22).
- [ ] **[P0] Privilégio mínimo documentado**: o README do módulo lista, por interface da porta, as permissões S3 do principal, sem `s3:BypassGovernanceRetention`, e a política de bucket com `s3:object-lock-remaining-retention-days` igual ao teto, como defesa em profundidade da DAT-14. O mesmo README separa o principal administrativo (`s3:BypassGovernanceRetention`, `s3:PutObjectRetention`, `s3:DeleteObjectVersion`), que a política de bucket nomeia como o único com o bypass, e escopa as permissões por prefixo: o principal da loja de plataforma só alcança `platform/*`.
- [ ] **[P0] Caminho administrativo de GOVERNANCE, segregado**: quando um teto superveniente mais estrito (DAT-16) exigir encurtar ou apagar uma versão GOVERNANCE, o caminho é o pacote `libs/backend/go/s3/admin`, fora da porta e de `s3.Store`, com credencial separada: um principal distinto da role da app, o único com `s3:BypassGovernanceRetention`, que a política de bucket nega a todo outro principal. O pacote só opera versão GOVERNANCE (recusa COMPLIANCE, que o S3 não deixa contornar), exige o novo teto e a origem dele (DAT-15, DAT-16) e devolve a evidência (`PurgeReceipt`, ou `RetentionReceipt{Ref, From, To, Reason, At}` no encurtamento, DAT-18) para o operador gravar na auditoria própria (DAT-25). Um teste com middleware do SDK prova que só esse pacote envia `x-amz-bypass-governance-retention`.
- [ ] **[P0] floci ≥ 2.2.0 fixado por digest**: trocar `floci/floci:2.0.1` por `floci/floci:2.2.0@sha256:<digest conferido na hora>` em `infra/local/compose/floci.yml`, `infra/test/compose.yml`, `.github/workflows/dmpf-go-ci.yml` e nos templates `tools/dmpf-plugin/templates/init/managed/infra/local/compose/floci.yml__tmpl__` e `.../infra/test/compose.yml__tmpl__`; atualizar os comentários («SQS e SNS» passa a «SQS, SNS, S3 e KMS»), o hash de `infra/local/compose/floci.yml` em `dmpf.rendered.json:24` e `infra/README.md:15,78,110`. Os serviços S3 e KMS vêm do mesmo endpoint 4566.
- [ ] **[P0] Integração contra o floci**: testes `//go:build integration` que leem `S3_ENDPOINT` e pulam sem ele, como o `sqs` (`sqs/testing_test.go:24-32`); a variável entra no `.env.example`, no template dele e no `env` de `.github/workflows/dmpf-go-ci.yml:48`. Cada teste cria o próprio bucket, com Object Lock e nome único, e roda o kit.
- [ ] **[P0] Teste de contrato contra S3 real em hmg**: workflow `workflow_dispatch` em `environment: hmg`, com credencial por OIDC, que roda o mesmo kit contra um bucket dedicado e prova que um principal com permissões amplas de S3 não apaga nem encurta uma versão COMPLIANCE. As versões COMPLIANCE do teste usam a menor retenção apurada na investigação e expiram sozinhas. O teste prova ainda que o S3 real aceita a promoção GOVERNANCE para COMPLIANCE e recusa o rebaixamento, e que a role da app recebe `AccessDenied` ao enviar o bypass, que só a role administrativa exerce sobre versão GOVERNANCE.
- [ ] **[P1] ADR novo**: registra a porta, as cláusulas do kit e seus identificadores, o módulo `s3`, o bump do floci e as divergências que persistirem. Passa em `node tools/adr-verify.mjs --from 63` e entra no índice `docs/adr/README.md`.
- [ ] **[P1] Documentação**: README do módulo `s3`, `libs/backend/go/testkit/README.md` (kit novo) e `docs/guides/dmpf-consumo-externo.md` (floci com S3 e KMS).

### Não-funcionais

- [ ] Compatibilidade: Go `1.26.9` (`sqs/go.mod:3`), `aws-sdk-go-v2` na faixa `>=1.46.0 <2`, sem trocar o `@nx-go/nx-go`.
- [ ] Reprodutibilidade: o kit é determinístico contra o fake (relógio injetado) e sem estado residual no floci (tmpfs na infra de testes).
- [ ] Custo de dependência: o módulo `ports` não ganha `require`; o `memory` não ganha dependência externa; o SDK da AWS só entra no módulo `s3`.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Port (`ports`) | Porta, tipos e erros de object storage |
| Provider (`memory`, `s3`) | Fake em memória, provider S3 novo e o caminho administrativo de GOVERNANCE (`s3/admin`) |
| Testkit (`providerkit`) | Kit de conformidade e subject do fake |
| Infra | floci 2.2.0 no Compose local, na infra de testes, no CI e nos templates do `init` |
| CI/CD | `dmpf-go-ci.yml` (endpoint e imagem) e workflow de contrato em hmg |
| Documentação | ADR novo, READMEs, guia de consumo externo e `infra/README.md` |

## Localização de código

```text
libs/backend/go/ports/objectstore.go              porta, tipos e erros (Onda 1)
libs/backend/go/memory/objectstore.go             fake em memória (Onda 1)
libs/backend/go/testkit/providerkit/objectstore.go   kit e subject (Onda 1)
libs/backend/go/s3/                               provider S3: go.mod, package.json, project.json,
                                                  dmpf-units.json, config, store, classifier (Onda 2)
libs/backend/go/s3/admin/                         caminho administrativo de GOVERNANCE, credencial própria (Onda 2)
infra/local/compose/floci.yml, infra/test/compose.yml, .github/workflows/dmpf-go-ci.yml
tools/dmpf-plugin/templates/init/managed/infra/{local/compose/floci.yml,test/compose.yml}__tmpl__
.github/workflows/dmpf-contract-aws.yml           contrato em hmg, manual (Onda 2)
docs/adr/0NN-porta-de-object-storage-e-provider-s3.md   ADR novo (número a resolver)
```

## Design

### Arquitetura

```text
contexto (application)              ports (puro)                   providers
 recebe só a interface  ─────▶  ObjectWriter
 de que precisa                 ObjectReader        ObjectStore ◀── memory.ObjectStore (fake, relógio injetado)
                                RetentionManager    (composição) ◀── s3.Store ──▶ S3 real (hmg) | floci ≥ 2.2.0
                                ObjectPurger
testkit/providerkit.ObjectStore(subject) ──▶ Verdict: as mesmas cláusulas nos três subjects
```

### Assinatura da porta

```go
type Scope string // "tenant" | "platform"; declarado na construção da loja
type ObjectRef struct{ Key ObjectKey; Version VersionID }
type Retention struct{ Mode RetentionMode; Until Instant } // COMPLIANCE | GOVERNANCE
type PutRequest struct {
	Key ObjectKey; Body []byte; ContentSHA256 string; ContentType string; Retention *Retention
}
type ObjectWriter interface { Put(ctx context.Context, req PutRequest) (ObjectVersion, error) }
type ObjectReader interface {
	Get(ctx context.Context, ref ObjectRef) (Object, error)
	Head(ctx context.Context, ref ObjectRef) (ObjectInfo, error) // retenção, hold, cifra
}
type RetentionManager interface {
	Retain(ctx context.Context, ref ObjectRef, to Retention) (ObjectInfo, error)
	SetLegalHold(ctx context.Context, ref ObjectRef, hold LegalHold) (HoldReceipt, error)
}
type ObjectPurger interface { Purge(ctx context.Context, ref ObjectRef) (PurgeReceipt, error) }
```

### Escopo da loja

```text
Scope      prefixo físico        contexto aceito                       recusa
tenant     tenant/<id>/<chave>   com tenant (do ExecutionContext)      tenant ausente ou com barra
platform   platform/<chave>      sem tenant (cadeia de plataforma)     contexto com tenant (ErrScopeMismatch)
```

Os dois prefixos são disjuntos: o de tenant leva o segmento `tenant/`, então nem o
tenant de id `platform` alcança o prefixo de plataforma. A loja de plataforma serve o
selo do Compliance Ledger do consumidor, que não tem tenant (SPEC-MSA607GT).

### Cláusulas do kit

| Cláusula | Prova | Norma |
| -------- | ----- | ----- |
| `put.version` | duas escritas na mesma chave dão `versionId` distintos e `Get` por versão devolve cada conteúdo | — |
| `put.integrity` | hash divergente é recusado e nada é gravado | — |
| `retention.dated` | `Until` ausente, vencido ou acima do teto é recusado, na escrita e no `Retain`; loja sem teto recusa | DAT-14, DAT-17 |
| `retention.compliance` | `Purge` antes do vencimento, `Retain` menor ou igual, rebaixamento de modo e `Until` acima do teto são recusados; `Retain` maior, dentro do teto, é aceito | DAT-14 |
| `retention.governance` | o mesmo, mais a promoção a COMPLIANCE com `Until` maior ou igual (aceita), sem nenhum caminho de bypass na porta | DAT-11, DAT-14 |
| `hold.independent` | o hold mantém o `Purge` recusado depois do vencimento, `Reason` é obrigatório ao pôr e o recibo o devolve; sem hold nem retenção o `Purge` passa | DAT-15, DAT-18 |
| `purge.receipt` | `Purge` após vencimento devolve recibo e o `Get` seguinte dá `ErrObjectNotFound` (`Skipped` se o subject não avança o relógio) | DAT-18 |
| `tenant.scope` | chave ou `versionId` de outro tenant dá `ErrObjectNotFound` (== `ErrNotFound`) e nenhum byte dele é lido; tenant ausente ou com `/` é recusado | IDN-13 a IDN-15 |
| `platform.scope` | a loja sem escopo recusa a construção; a de plataforma grava sob `platform/`, não lê nem inventa tenant e recusa contexto com tenant (`ErrScopeMismatch`); a referência de um escopo não resolve no outro (`ErrObjectNotFound`), inclusive com um tenant de id `platform` | IDN-16 a IDN-20 |
| `head.state` | `Head` devolve retenção, hold e a cifra declarada | DAT-08 |
| `overwrite.keeps-locked` | `Put` na chave de uma versão travada cria versão nova e a antiga segue travada | issue #4389 |

### Conciliação com as normas

| Norma | Como a porta a realiza |
| ----- | ---------------------- |
| DAT-08 | cifra declarada na construção; `Head` a expõe para inspeção da versão armazenada |
| DAT-11 | quatro interfaces com privilégios distintos; sem bypass; política de bucket mínima no README |
| DAT-14 a DAT-17 | teto e origem declarados na loja; retenção sem teto ou além dele é recusada, inclusive na prorrogação; COMPLIANCE nasce no prazo declarado e só prorroga dentro do teto; teto superveniente só alcança GOVERNANCE, pelo caminho administrativo com evidência; o hold é obrigação de origem declarada, e além do teto só com ADR do kernel que emende a DAT-14 |
| DAT-18, DAT-25 | `PurgeReceipt` e `HoldReceipt` saem pelo retorno, e o caminho administrativo devolve a evidência do encurtamento e da purga; a auditoria é do chamador, não do pipeline de log |
| IDN-16 a IDN-20 | escopo de plataforma declarado na construção: prefixo fixo, nenhum tenant lido nem inventado, alcance só esse prefixo |
| IDN-11 a IDN-14 | tenant do `ExecutionContext` no prefixo da chave; referência de outro tenant não resolve, e o escopo não depende do chamador |

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Onde mora a porta | `ports`, puro, com fake em `memory` e kit em `providerkit` | Pacote novo só para storage (quebra o molde dos demais ports e do ADR-015) |
| Corpo da escrita | `[]byte` | `io.Reader` (`io` é puro, mas o checksum de corpo inteiro e o multipart ficam para outra spec) |
| Remoção | só `Purge` por versão, sem delete lógico | `Delete` sem versão (cria delete marker e dá falsa sensação de remoção) |
| GOVERNANCE | modo existe e o bypass não está na porta; o encurtamento por teto superveniente é um caminho administrativo segregado, de credencial própria e com evidência | Expor bypass por opção na porta (reabre o caminho que o COMPLIANCE fecha); nenhum caminho de encurtamento (um teto superveniente mais estrito, DAT-16, ficaria sem como se cumprir) |
| Mudança de retenção | prorrogação e promoção de GOVERNANCE a COMPLIANCE, nunca encurtamento nem rebaixamento | Permitir trocar o modo nos dois sentidos (o COMPLIANCE não volta, e a porta não pode prometer o contrário) |
| COMPLIANCE e o teto | nasce no prazo declarado, só prorroga e fica sempre dentro do teto | COMPLIANCE já no teto máximo (um teto superveniente mais estrito não teria como ser cumprido) |
| Tentativa entre tenants | impossível por construção; a loja não registra evento de segurança | Consultar o prefixo de outro tenant para registrar a tentativa (quebraria o isolamento que a IDN-14 exige) |
| Teto de retenção | declarado na loja, com origem | Teto por requisição (cada chamador reinventaria a DAT-14) |
| Legal hold | sem prazo, com `Reason` (origem declarada, DAT-15) e recibo; não é exceção autodeclarada à DAT-14: além do teto só com ADR novo do kernel que a emende | Hold com prazo (deixa de ser hold); hold como exceção que o contexto declare para si (o teto deixaria de ser teto) |
| Escopo da loja | declarado na construção: de tenant (prefixo `tenant/<id>/`) ou de plataforma (prefixo fixo `platform/`), disjuntos | Tenant obrigatório em toda operação (recusaria o selo de plataforma); tenant sintético para o selo (vedado pela IDN-20); escopo implícito (a omissão tem de negar, IDN-17) |
| Capability do SDK S3 | `io.storage` | `io.network` (a tabela do RFC já classifica «cliente de objeto» como storage) |
| Módulo do provider | `libs/backend/go/s3`, um módulo por lib (ADR-030, mantido pelo ADR-045) | Dentro do `sqs` (mistura mensageria e storage no mesmo `go.mod`) |
| Emulador | floci ≥ 2.2.0, porque a 2.2.0 traz a correção do #4389 | Ficar na 2.0.1 com cláusulas puladas; LocalStack (vetado, ADR-039); MinIO, SeaweedFS e Ceph (fora do alvo AWS) |
| Prova de WORM | kit também contra S3 real em hmg | Só floci (projeto de 8 meses, com divergências reportadas de Object Lock) |

## Regras relacionadas

- `docs/dmpf/contexto-erros-seguranca.md`: DAT-03, DAT-08, DAT-11, DAT-14 a DAT-18, DAT-22, DAT-25, IDN-11 a IDN-20
- `docs/dmpf/resiliencia-observabilidade.md`: RES-05 (`:411`), RES-21 (`:430`), RES-27 (`:671`)
- `docs/dmpf/uow-inbox-outbox.md`: OBX-17 e INB-16 (purga com evidência, o modelo do `PurgeReceipt`)
- ADR-015 (capabilities por bloco), ADR-030 e ADR-045 (um módulo por lib), ADR-039 (floci), ADR-047 (tags de módulo), ADR-054 item 7 (infra de testes), ADR-060 (consumidor externo)
- SPEC-XMNBMY50 (shared kernel) e SPEC-VJMM2DE5 (infra de testes separada)
- SPEC-MSA607GT (assinatura do selo, que consome o escopo de plataforma desta loja)

## Verificação e testes

### Critérios de aceite

- [ ] `providerkit.ObjectStore` passa contra o fake sem cláusula `Skipped`, e contra o floci ≥ 2.2.0 e o S3 real com as `Skipped` nomeadas que a investigação 1 registrou.
- [ ] No S3 real, um principal com permissões amplas de S3 não apaga nem encurta uma versão COMPLIANCE: `DeleteObject` com `versionId` e `PutObjectRetention` menor são recusados.
- [ ] Um teste de unidade prova que `s3.Store` nunca envia `x-amz-bypass-governance-retention`, e que só `s3/admin` o envia.
- [ ] No S3 real, o caminho administrativo, com a credencial própria, encurta e apaga uma versão GOVERNANCE e devolve a evidência, e a role da app recebe `AccessDenied` ao enviar o bypass; a promoção de GOVERNANCE para COMPLIANCE é aceita e o rebaixamento é recusado.
- [ ] A loja de plataforma grava e lê sob `platform/` num contexto sem tenant e recusa contexto com tenant, e a de tenant não resolve a referência dela (nem o inverso), no fake, no floci e no S3 real.
- [ ] `rg 'floci/floci:2\.0\.1'` não encontra nada; os cinco pontos de imagem (`infra/local/compose/floci.yml:6`, `infra/test/compose.yml:85`, `.github/workflows/dmpf-go-ci.yml:248` e os templates `floci.yml__tmpl__` e `compose.yml__tmpl__` do `init`) e o hash de `dmpf.rendered.json` estão em dia, e o CI reprova divergência entre template e arquivo gerado.
- [ ] `go run ./tools/dmpf-conformance/cmd/conformance --root .` e `go run ./tools/dmpf-conformance/cmd/modsync --root . --check` passam com `ports` sem `require` novo e o SDK só em `s3`.
- [ ] `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source` e, nos projetos Go tocados, `fmt-check`, `vet`, `test-race` e `govulncheck` passam.
- [ ] As três investigações P0 têm resultado registrado no ADR novo, e `node tools/adr-verify.mjs --from 63` passa.

### Cenários de teste

```text
DADO uma loja com teto de 400 dias e a cifra declarada
QUANDO Put grava um selo com retenção COMPLIANCE de 365 dias
ENTÃO devolve ObjectVersion com versionId e o hash verificado, Head devolve modo, data e cifra, e Get por versionId devolve o mesmo corpo

DADO uma versão em COMPLIANCE com retenção vigente
QUANDO Purge, Retain com Until menor e Retain com modo GOVERNANCE são chamados, nesta ordem
ENTÃO os três são recusados (ErrObjectLocked, ErrRetentionNotExtended, ErrRetentionModeChange) e a versão segue intacta

DADO uma versão com legal hold cuja retenção já venceu
QUANDO Purge é chamado e depois SetLegalHold(Off) e Purge de novo
ENTÃO o primeiro é recusado com ErrObjectLocked, e o segundo devolve PurgeReceipt com a data do vencimento

DADO uma loja com teto de 400 dias
QUANDO Put pede retenção de 800 dias, e outro Put pede retenção sem data
ENTÃO o primeiro dá ErrRetentionAboveCeiling, o segundo dá ErrRetentionUnbounded, e Head não encontra nenhuma das duas escritas

DADO o tenant A com uma versão gravada
QUANDO o tenant B chama Get e Purge com a chave e o versionId de A
ENTÃO ambos devolvem ErrObjectNotFound (errors.Is com ports.ErrNotFound) e a versão de A segue intacta e travada

DADO uma versão travada no floci 2.2.0
QUANDO Put grava a mesma chave de novo
ENTÃO nasce uma versão nova, a anterior segue travada e Purge da anterior continua recusado

DADO uma loja de escopo de plataforma e um contexto sem tenant, de uma cadeia de plataforma
QUANDO Put grava o selo e Get lê por versionId, e depois um contexto com tenant chama Get
ENTÃO a chave física tem o prefixo platform/, nenhum tenant é lido nem inventado, o Get com tenant dá ErrScopeMismatch, e a mesma referência numa loja de escopo de tenant dá ErrObjectNotFound

DADO uma versão GOVERNANCE com retenção vigente e uma loja com teto de 400 dias
QUANDO Retain promove a COMPLIANCE com o mesmo Until, e depois Retain pede Until de 800 dias
ENTÃO a promoção é aceita e Head devolve COMPLIANCE, e o segundo dá ErrRetentionAboveCeiling

DADO uma versão GOVERNANCE com retenção de 365 dias e um teto superveniente de 180 dias, de origem declarada
QUANDO o caminho administrativo, com a credencial própria, encurta a retenção para o novo teto, e a role da app tenta o mesmo com o bypass
ENTÃO o primeiro devolve a evidência com a data anterior e a nova, e o S3 recusa o segundo com AccessDenied
```

<critical_constraints>
- [P0] Nenhuma operação da porta apaga versão sob retenção ou hold, encurta retenção, rebaixa o modo (COMPLIANCE para GOVERNANCE) ou contorna o GOVERNANCE; o provider da porta nunca envia `x-amz-bypass-governance-retention`. O bypass só existe no caminho administrativo segregado: credencial própria, fora da role da app, só para versão GOVERNANCE e com evidência (DAT-16, DAT-18).
- [P0] Toda referência carrega o `versionId`; a porta não tem delete sem versão, porque o delete marker do S3 não remove nada.
- [P0] O bloco `port` permanece puro (RFC §6.2): a porta usa só `context`, `errors`, `[]byte` e `Instant`, sem `time`, SDK ou pacote ausente da tabela de stdlib pura.
- [P0] Retenção é sempre datada e sob o teto declarado na construção da loja, na escrita, na prorrogação e na promoção a COMPLIANCE, que não se encurta; sem teto declarado, a loja recusa gravar com retenção (DAT-14, DAT-17).
- [P0] A loja é construída com escopo declarado, e escopo ausente recusa a construção (IDN-17). No escopo de tenant, o tenant vem do `ExecutionContext`, prefixa a chave física e, ausente, recusa a operação (IDN-14, IDN-15, IDN-20). No escopo de plataforma, a operação não exige sujeito nem tenant (IDN-16), roda na cadeia de plataforma (IDN-18), usa o prefixo físico fixo de plataforma, com alcance declarado na construção (IDN-19), e nunca lê nem inventa tenant (IDN-20).
- [P0] O mesmo kit roda contra o fake, o floci e o S3 real; a cláusula que um subject não exerce fica em `Skipped` com nome, nunca fora do kit.
- [P0] A garantia WORM só se prova contra S3 real: o floci dá paridade de ambiente, e o teste de contrato em hmg não é opcional.
- [P1] Unidade nova exige `include` no manifesto (`DMPF-U001`) e entra no baseline por `--write-baseline` (`DMPF-T001`), feito por quem mantém o kernel.
- [P1] Arquivo gerado pelo `init` só muda pelo template em `tools/dmpf-plugin/templates/init/` (ADR-060); projetos Nx distintos vão em commits separados.
</critical_constraints>

## Escopo fora

- **Streaming, multipart e objetos grandes**: o corpo é `[]byte`; objeto de dezenas de megabytes pede outra spec.
- **Escrita condicional** (`If-None-Match`) e **lifecycle**: a chave nova nasce por versão, e a expiração é da retenção.
- **Criar conta, roles (a da app e a administrativa), bucket e chave de hmg ou produção, e IaC**: pré-requisito operacional do teste de contrato; a spec entrega o workflow que o consome.
- **Declarar storage no `infra.json` e provisionar no `infrasync`**: cada teste cria o próprio bucket; o provisionamento por app é decisão futura.
- **Replicação, SSE-C e rotação de chave de cifra**: custódia e rotação são de plataforma (DAT-08).
- **Outros storages** (GCS, Azure Blob, MinIO, SeaweedFS, Ceph): a porta é neutra, mas só o S3 ganha provider aqui.
- **Processo de aprovação do caminho administrativo**: quem autoriza o encurtamento e como se aciona é operação; a spec entrega a operação segregada, a evidência e a política de bucket.
- **Emendar a DAT-14 para admitir o hold além do teto**: ADR novo do kernel, em spec própria; esta spec só registra a dependência.
- **Âncora RFC 3161 e o ledger do consumidor**: a porta guarda o selo; o que se assina e quando é do consumidor (assinatura é a SPEC-MSA607GT).
