---
id: SPEC-MSA607GT
slug: porta-assinatura-kms
title: DMPF — Porta de assinatura com provider AWS KMS emulado no floci
stage: backlog
priority: P1
depends_on: [SPEC-CWBA4BPV]
ticket_url: https://linear.app/mmda/issue/DEVS-99/spec-msa607gt-dmpf-porta-de-assinatura-com-provider-aws-kms-emulado-no
subtask_urls: []
created: 2026-10-12
---

# SPEC-MSA607GT: Porta de assinatura com provider AWS KMS

## Resumo

Dar ao kernel uma porta de assinatura e verificação: o chamador declara o
algoritmo, o provider assina com chave assimétrica `SIGN_VERIFY` que nunca sai do
serviço de chaves, e qualquer pessoa verifica a assinatura offline, com a chave
pública exportada. A porta nasce com fake determinístico e kit de conformidade no
`testkit` (Onda 1) e segue com o provider AWS KMS (SDK Go v2) em módulo próprio, a
verificação offline sem SDK e o teste de contrato contra o KMS real (Onda 2). O
floci ≥ 2.2.0, que emula o KMS, vem da SPEC-CWBA4BPV; esta spec só o consome.

A origem é o consumidor externo antecipa-market, SPEC-GXGASP24 (DEVS-76): a
plataforma assina um selo periódico (`SealManifest`, raiz Merkle das cabeças de um
ledger encadeado) com chave em KMS segregada e o grava em S3 Object Lock, pelo escopo de plataforma da loja da SPEC-CWBA4BPV.

Como mantenedor do kernel, quero que a assinatura seja um contrato igual no fake, no
emulador e no KMS real, para que o consumidor prove o selo sem confiar no operador do
banco e sem reimplementar o cliente do KMS. A entrega segue duas ondas, uma por PR.

## Contexto

- **Problema**: o kernel não tem porta de assinatura.
  - O `ports` só verifica credencial de entrada (`Authenticator`,
    `libs/backend/go/ports/authn.go:47`). Fora de `_test.go`, a única assinatura do
    repositório é a de certificados do PKI de teste
    (`libs/backend/go/testkit/tb/pki.go:43,94`, `x509.CreateCertificate`).
  - A norma não fixa porta nem algoritmo: a linha «Assinatura e verificação quando
    mensagens cruzarem fronteiras não confiáveis» é «parcialmente consolidada» e
    encaminha o mecanismo de assinatura à plataforma
    (`docs/dmpf/contexto-erros-seguranca.md:1586`).
  - O kernel não tem o SDK do KMS; o `sqs` só classifica códigos de erro do KMS que o
    SQS devolve (`libs/backend/go/sqs/classifier.go:47-48,99-109`).
- **Impacto**: sem a porta, o consumidor escreve o cliente do KMS no contexto, sem fake
  nem kit, e a verificação do selo depende de conhecer o formato de cada assinatura.
- **Fatos externos, conferidos em 12/10/2026** (AWS KMS):
  - `Sign` exige chave assimétrica com `KeyUsage` `SIGN_VERIFY` e a permissão
    `kms:Sign`; aceita mensagem de até 4096 bytes e, acima disso, um digest com
    `MessageType` `DIGEST`; a AWS manda registrar a chave e o algoritmo usados, que a
    verificação exige, e a resposta traz o ARN da chave que assinou. Chave desabilitada
    devolve `DisabledException`, estado incompatível devolve `KMSInvalidStateException` e
    algoritmo incompatível com a chave devolve `InvalidKeyUsageException`;
    `KeyUnavailableException` pode ser retentada
    (https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html).
  - A assinatura ECDSA é DER (ANSI X9.62, RFC 3279). `ED25519_SHA_512` exige
    `MessageType` `RAW` na chave `ECC_NIST_EDWARDS25519`
    (https://docs.aws.amazon.com/kms/latest/developerguide/asymmetric-key-specs.html).
  - `GetPublicKey` devolve a chave em DER `SubjectPublicKeyInfo` (RFC 5280), com
    `KeyUsage` e `SigningAlgorithms`, e a AWS prevê verificar fora do KMS com ela
    (https://docs.aws.amazon.com/kms/latest/APIReference/API_GetPublicKey.html).
  - O README da 2.2.0 do floci lista o KMS entre os serviços em processo (assinar,
    verificar, aliases), e as notas da release trazem «kms: match the AWS error codes,
    messages and check order (#3968)» (https://github.com/floci-io/floci/releases/tag/2.2.0).
- **Reportado pelo consumidor externo contra a 2.0.1 e não reconferido**: o KMS do floci
  assina com ECC P-256 a P-521, secp256k1 e Ed25519 e a assinatura verifica fora do
  emulador com `openssl`; assina com chave `Disabled` ou `PendingDeletion`; aceita
  `Encrypt` em chave `SIGN_VERIFY`.
- **Não verificado nesta redação** (cada item tem requisito de investigação): o efeito da
  alteração #3968 sobre essas divergências; a verificação offline das assinaturas da
  2.2.0; o `KeyId` que o floci devolve quando o chamador assina por alias.

<constraints>
- [P0] A chave privada nunca entra no kernel: o provider só chama `Sign` e `GetPublicKey`, sem `Encrypt`, `Decrypt`, `GenerateDataKey` nem `GenerateMac`.
- [P0] O algoritmo é sempre declarado pelo chamador, de um conjunto fechado; vazio ou fora do conjunto é recusado, nunca assumido.
- [P0] A assinatura registra a chave resolvida e o algoritmo; a verificação usa a chave pública exportada, sem rede nem credencial.
- [P0] O bloco `port` permanece puro (RFC §6.2): a porta usa só `context`, `errors`, `[]byte` e `Instant`; `crypto/ecdsa`, `crypto/ed25519` e `crypto/x509` ficam em provider.
- [P0] O mesmo kit roda contra o fake, o floci e o KMS real; a cláusula que um subject não exerce fica em `Skipped` com nome, nunca fora do kit.
- [P0] A garantia de que só o KMS assina e de que chave inutilizável não assina se prova contra o KMS real: o floci dá paridade de ambiente, e o teste de contrato não é opcional.
- [P0] A infra do floci (imagem, endpoint, S3 e KMS) é da SPEC-CWBA4BPV; esta spec não altera `floci.yml` nem a imagem.
- [P1] A chave de assinatura é distinta da chave de cifra do object storage (`SIGN_VERIFY` × `ENCRYPT_DECRYPT`): chaves segregadas por finalidade.
- [P1] Unidade nova exige `include` no manifesto (`DMPF-U001`) e entra no baseline por `--write-baseline` (`DMPF-T001`), feito por quem mantém o kernel.
</constraints>

## Requisitos

### Funcionais

#### Investigações (antes da Onda 2)

- [ ] **[P0] Investigar o KMS do floci 2.2.0 contra o contrato**: rodar no floci 2.2.0 as cláusulas `key.unusable` e `algorithm.declared` (assinar com chave `Disabled` e `PendingDeletion`, `Encrypt` em chave `SIGN_VERIFY`, algoritmo incompatível com a chave) e registrar quais divergências da 2.0.1 persistem. Cada uma vira cláusula `Skipped` nomeada contra o floci, registrada no ADR novo.
- [ ] **[P0] Investigar a verificação offline das assinaturas da 2.2.0**: confirmar que as assinaturas ECDSA P-256 e Ed25519 do floci 2.2.0 verificam com `openssl` e com a stdlib do Go a partir do `GetPublicKey`, e se o `Sign` por alias devolve o `KeyId` completo da chave.

#### Onda 1 — porta, fake e kit (sem nuvem)

- [ ] **[P0] Porta em `ports`**: novo `libs/backend/go/ports/signing.go` com `Signer` (`Sign`), `PublicKeySource` (`PublicKey`) e `Verifier` (`Verify`, sem `context` nem I/O). Os tipos usam só `context`, `errors`, `[]byte`, strings e `Instant`.
- [ ] **[P0] Algoritmo declarado em conjunto fechado**: `Algorithm` aceita `ecdsa-p256-sha256` e `ed25519`. Vazio devolve `ErrAlgorithmNotDeclared`; fora do conjunto, `ErrAlgorithmUnsupported`; incompatível com o `KeySpec` da chave, `ErrAlgorithmNotSupportedByKey`. A porta não tem algoritmo padrão.
- [ ] **[P0] Formato fixado pela porta**: a assinatura `ecdsa-p256-sha256` é ASN.1 DER (X9.62) sobre o SHA-256 da mensagem; a `ed25519` tem 64 bytes (RFC 8032, sem pré-hash). `PublicKey.SPKI` é DER `SubjectPublicKeyInfo`. Todo provider normaliza para esse formato.
- [ ] **[P0] Mensagem limitada**: `MaxMessageSize = 4096` na porta. Mensagem maior devolve `ErrMessageTooLarge` e vazia devolve `ErrMessageEmpty`, ambas sem chamar o provider; o documento maior é assinado pelo digest que o chamador calcula. A porta fecha a vazia pelo lado seguro: a AWS fala em 0 a 4096 bytes no texto e em mínimo 1 na restrição de comprimento.
- [ ] **[P0] Assinatura autocontida**: `Signature{Key, Algorithm, Value, SignedAt}`, com `Key` igual ao identificador estável que o provider resolve (o ARN completo no KMS) e nunca o alias recebido, e `SignedAt` vindo de `ports.Clock`. `PublicKey{Key, Algorithm, SPKI}` leva a mesma `Key`.
- [ ] **[P0] Verificação offline**: `Verifier.Verify(pub, message, sig)` devolve `ErrKeyMismatch` se `sig.Key` difere de `pub.Key`, `ErrAlgorithmMismatch` se os algoritmos diferem e `ErrSignatureInvalid` se a assinatura não confere; não faz I/O.
- [ ] **[P0] Fake determinístico**: `providerkit.FakeSigner(seed)` em `libs/backend/go/testkit/providerkit/signer.go` deriva as chaves da semente e do `KeyRef`; é determinístico nas chaves, nos identificadores, nos erros e nos bytes Ed25519. A assinatura ECDSA é válida mas não reproduzível byte a byte, e o kit nunca compara bytes de assinatura. Expõe `Disable(key)` para a cláusula de chave inutilizável.
- [ ] **[P0] Kit `providerkit.Signer`**: devolve `Verdict` com `Diagnostic{Clause, Rule}` e `Skipped` nomeado (`providerkit/suite.go:7-43`), consumido por `tb.Require`. O oráculo de verificação é a stdlib escrita no próprio kit, independente de qualquer `Verifier` sob teste. As cláusulas estão em Design.
- [ ] **[P1] Subject do fake e teste**: `providerkit.FakeSignerSubject(seed)` e um teste do fake contra o kit, no molde de `memory/conformance_test.go`.

#### Onda 2 — provider KMS, verificação offline e contrato

- [ ] **[P0] Módulo `libs/backend/go/kms`**: unidade `kernel/provider-kms` (`dmpf-units.json` com `service/kms` em `io.network`, RFC §6.1, `docs/dmpf/rfc-dmpf-foundation-v0.1.md:749`), na faixa de `aws-sdk-go-v2` do `sqs` (`sqs/dmpf-units.json:13-18`), com `package.json` privado, `project.json` (tags `type:lib`, `scope:backend`, `stack:go`, `layer:providers`; `test-race` com `-tags=integration`, `sqs/project.json:38-50`) e entrada em `shared_kernel_units` (`tools/dmpf-baseline/units-baseline.json:526-534`). Entra no `go.work` por `modsync` e no grupo `go-libs` pelo seletor de diretório (`nx.json:81-86`).
- [ ] **[P0] `kms.Signer`**: implementa `Signer` e `PublicKeySource`. `Sign` chama `kms:Sign` sempre com `MessageType` `RAW`, mapeando `ecdsa-p256-sha256` para `ECDSA_SHA_256` e `ed25519` para `ED25519_SHA_512`, e devolve o `KeyId` da resposta. `PublicKey` chama `GetPublicKey`, confere `KeyUsage` `SIGN_VERIFY` e que o algoritmo consta de `SigningAlgorithms`. Sem cache de chave pública.
- [ ] **[P0] Config, resiliência e erros no molde do `sqs`**: `Config` com `AWS`, `Endpoint`, `InsecureForDevelopmentOnly`, `Clock` e `Sheet`; `Validate` recusa config incompleta e endpoint sem TLS (`sqs/config.go:27-28,49`). `Sign` e `GetPublicKey` não têm efeito de estado, então a ficha (RES-21) autoriza retentativa só para `KeyUnavailableException`, `DependencyTimeoutException` e `KMSInternalException` (RES-27). O classificador mapeia `DisabledException` e `KMSInvalidStateException` para `ErrKeyUnusable`, `NotFoundException` para `ErrKeyNotFound` e `InvalidKeyUsageException` (uso da chave ou algoritmo incompatível) para `ErrAlgorithmNotSupportedByKey`, fechado por omissão (ERR-11; `sqs/classifier.go:28-50`). A telemetria não leva mensagem nem assinatura (DAT-22).
- [ ] **[P0] Verificador offline**: pacote `libs/backend/go/kms/offline` implementa `ports.Verifier` só com a stdlib (`crypto/ecdsa`, `crypto/ed25519`, `crypto/x509`, `crypto/sha256`) e não importa o SDK da AWS, para o auditor verificar sem credencial nem rede. Um teste cruzado verifica com ele as assinaturas do fake e as do KMS.
- [ ] **[P0] Privilégio mínimo documentado**: o README do módulo lista `kms:Sign` e `kms:GetPublicKey` como únicas permissões do principal de runtime, a chave com `KeyUsage` `SIGN_VERIFY` e política de chave própria, distinta da chave que cifra o object storage.
- [ ] **[P0] Integração contra o floci**: testes `//go:build integration` que leem `KMS_ENDPOINT` e pulam sem ele (`sqs/testing_test.go:24-32`); a variável entra no `.env.example`, no template dele e no `env` de `.github/workflows/dmpf-go-ci.yml`. O teste cria a própria chave `SIGN_VERIFY` e roda o kit para os dois algoritmos.
- [ ] **[P0] Teste de contrato contra o KMS real em hmg**: novo job `kms` no workflow de contrato criado pela SPEC-CWBA4BPV (`environment: hmg`, OIDC), com uma chave `Enabled` e outra `Disabled` provisionadas fora desta spec e uma role com `kms:Sign`, `kms:GetPublicKey` e `kms:DescribeKey`. Roda o kit e verifica com `openssl` a assinatura que saiu do KMS.
- [ ] **[P1] ADR novo**: registra a porta, o conjunto fechado de algoritmos, o formato da assinatura, o limite de 4096 bytes, o módulo `kms`, o verificador offline e as divergências do floci que persistirem; distinto do ADR da SPEC-CWBA4BPV. Passa em `node tools/adr-verify.mjs --from 63` e entra no índice `docs/adr/README.md`.
- [ ] **[P1] Documentação**: README do módulo `kms`, `libs/backend/go/testkit/README.md` (kit e fake novos) e a seção de infra de `docs/guides/dmpf-consumo-externo.md`.

### Não-funcionais

- [ ] Compatibilidade: Go `1.26.9`, `aws-sdk-go-v2` em `>=1.46.0 <2`; `ports` e `memory` sem `require` novo, e o SDK do KMS só no módulo `kms`.
- [ ] Prazo: `Sign` deriva o prazo do contexto (RES-05, RES-06) e não fixa valor próprio.
- [ ] Reprodutibilidade: contra o fake, o kit é determinístico; contra o floci, cada teste cria a própria chave.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Port (`ports`) | Porta de assinatura, tipos, limite e erros |
| Provider (`kms`, `kms/offline`) | Módulo novo com o provider e o verificador offline |
| Testkit (`providerkit`) | Fake determinístico, kit e subject |
| CI/CD | `KMS_ENDPOINT` no `dmpf-go-ci.yml` e job `kms` no workflow de contrato |
| Documentação | ADR novo, READMEs e guia de consumo externo |

## Localização de código

```text
libs/backend/go/ports/signing.go                      porta, tipos, limite e erros (Onda 1)
libs/backend/go/testkit/providerkit/signer.go         fake, kit e subject (Onda 1)
libs/backend/go/kms/                                  go.mod, package.json, project.json, dmpf-units.json,
                                                      config, signer, classifier (Onda 2)
libs/backend/go/kms/offline/                          verificador offline, só stdlib (Onda 2)
.env.example e tools/dmpf-plugin/templates/init/managed/.env.example__tmpl__   KMS_ENDPOINT
.github/workflows/dmpf-go-ci.yml                      env KMS_ENDPOINT
.github/workflows/dmpf-contract-aws.yml               job kms (o arquivo vem da SPEC-CWBA4BPV)
docs/adr/0NN-porta-de-assinatura-e-provider-kms.md    ADR novo (número a resolver)
```

## Design

### Arquitetura

```text
contexto (SealManifest)         ports (puro)                         providers
 monta a mensagem ≤ 4096 B ──▶  Signer ─────────────▶ kms.Signer ──▶ AWS KMS | floci ≥ 2.2.0
 (ou o digest dela)             PublicKeySource ────▶ GetPublicKey (SPKI DER)
 auditor, sem rede              Verifier ◀── kms/offline (stdlib)     providerkit.FakeSigner (testes)
```

### Assinatura da porta

```go
type Algorithm string // "ecdsa-p256-sha256" | "ed25519"
type KeyRef string    // id, ARN ou alias; opaco para a porta
type SignRequest struct{ Key KeyRef; Algorithm Algorithm; Message []byte }
type Signature struct{ Key KeyRef; Algorithm Algorithm; Value []byte; SignedAt Instant }
type PublicKey struct{ Key KeyRef; Algorithm Algorithm; SPKI []byte }
type Signer interface{ Sign(ctx context.Context, req SignRequest) (Signature, error) }
type PublicKeySource interface{ PublicKey(ctx context.Context, key KeyRef) (PublicKey, error) }
type Verifier interface{ Verify(pub PublicKey, message []byte, sig Signature) error }
```

### Mapeamento e formato

| Algoritmo da porta | `KeySpec` no KMS | `SigningAlgorithm` | `MessageType` | Valor da assinatura |
| ------------------ | ---------------- | ------------------ | ------------- | ------------------- |
| `ecdsa-p256-sha256` | `ECC_NIST_P256` | `ECDSA_SHA_256` | `RAW` | ASN.1 DER (X9.62), SHA-256 da mensagem |
| `ed25519` | `ECC_NIST_EDWARDS25519` | `ED25519_SHA_512` | `RAW` | 64 bytes (RFC 8032) |

O oráculo do contrato verifica fora do KMS, a partir do `SPKI` exportado e de um
`message.bin` (OpenSSL 3.0.13 conferido nesta redação):

```bash
openssl pkey -pubin -inform DER -in pub.der -outform PEM -out pub.pem
openssl dgst -sha256 -verify pub.pem -signature sig.der message.bin          # ecdsa-p256-sha256
openssl pkeyutl -verify -pubin -inkey pub.pem -rawin -in message.bin -sigfile sig.bin   # ed25519
```

### Cláusulas do kit

| Cláusula | Prova |
| -------- | ----- |
| `sign.verifies` | para cada algoritmo do subject, a assinatura verifica pelo oráculo da stdlib com a chave exportada |
| `sign.key-recorded` | `Signature.Key` é o identificador resolvido, não o alias, e é igual a `PublicKey.Key` |
| `algorithm.declared` | algoritmo vazio, fora do conjunto ou incompatível com a chave é recusado antes de assinar |
| `message.limit` | 4096 bytes assina; 4097 dá `ErrMessageTooLarge` e vazia dá `ErrMessageEmpty`, sem chamar o provider |
| `key.unusable` | chave inexistente dá `ErrKeyNotFound`; `Disabled` ou `PendingDeletion` dá `ErrKeyUnusable` (`Skipped` se o subject não oferece a chave) |
| `verify.rejects` | mensagem alterada, assinatura alterada, outra chave e outro algoritmo dão `ErrSignatureInvalid`, `ErrKeyMismatch` e `ErrAlgorithmMismatch` |
| `public.spki` | `PublicKey.SPKI` é `SubjectPublicKeyInfo` válido e coerente com o algoritmo |

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Algoritmos | conjunto fechado `ecdsa-p256-sha256` e `ed25519`, sem padrão | RSA (três variantes de preenchimento e assinatura grande), ML-DSA e SM2 (sem demanda), `ED25519_PH_SHA_512` (exige `DIGEST` e fragmenta o caminho) |
| Entrada da assinatura | sempre `RAW`, até 4096 bytes | `DIGEST` (o Ed25519 puro não o aceita; um caminho só mantém fake e provider iguais) |
| Verificação | só offline, pela chave pública | `kms:Verify` online (dá ao auditor credencial e rede para a mesma confiança que a chave pública já dá) |
| Onde mora o verificador | `kms/offline`, pacote sem SDK | `ports` (`crypto/ecdsa` e `crypto/x509` não constam da tabela de stdlib pura e o bloco os recusa, `tools/dmpf-conformance/internal/rule/stdlib.go:128-130`); módulo novo só para ele |
| Onde mora o fake | `providerkit`, ao lado do kit | `memory` (realiza as portas de estado; o único consumidor previsto do fake de assinatura é o kit) |
| Chave da assinatura | de plataforma, não de tenant; o selo é global, e a gravação dele usa o escopo de plataforma da loja da SPEC-CWBA4BPV | Chave por tenant (a IDN-11 trata dado de tenant, e o selo não o é) |
| Capability do SDK KMS | `io.network` | `io.storage` (o KMS não guarda dado do usuário) |
| Cache de chave pública | nenhum | Cache com prazo (o export acontece por auditoria, e a chave trocada tem outro `KeyId`) |
| Classificador de erro | um por módulo, como o `sqs` | Módulo comum de erros da AWS (três cópias, `sqs`, `s3` e `kms`, até alguém extrair; fica como decisão futura) |
| Prova de inviolabilidade | kit também contra o KMS real em hmg | Só floci (projeto de 8 meses, com `Sign` em chave inutilizável reportado) |

## Regras relacionadas

- `docs/dmpf/contexto-erros-seguranca.md`: DAT-22, IDN-11, ERR-11 e a linha de assinatura em `:1586`
- `docs/dmpf/resiliencia-observabilidade.md`: RES-05 (`:411`), RES-21 (`:430`), RES-27 (`:671`)
- ADR-015 (capabilities por bloco), ADR-030 e ADR-045 (um módulo por lib), ADR-039 (floci), ADR-047 (tags de módulo), ADR-054 item 7 (infra de testes)
- SPEC-CWBA4BPV (floci ≥ 2.2.0 com S3 e KMS, workflow de contrato, convenção do kit de provider e escopo de plataforma da loja, por onde o selo é gravado)

## Verificação e testes

### Critérios de aceite

- [ ] `providerkit.Signer` passa contra o fake sem cláusula `Skipped`, e contra o floci ≥ 2.2.0 e o KMS real com as `Skipped` nomeadas que a investigação 1 registrou.
- [ ] A assinatura produzida pelo provider, no floci e no KMS real, verifica fora do KMS com `openssl` e com a stdlib do Go, para `ecdsa-p256-sha256` e `ed25519`.
- [ ] Um teste de unidade, com middleware do SDK, prova que `Sign` envia `MessageType` `RAW` e o `SigningAlgorithm` do mapeamento, e que nenhuma operação além de `Sign` e `GetPublicKey` sai do provider.
- [ ] `kms/offline` não importa `github.com/aws/aws-sdk-go-v2` (`go list -deps ./offline`) e `ports` não importa `crypto/ecdsa` nem `crypto/x509`.
- [ ] `go run ./tools/dmpf-conformance/cmd/conformance --root .` e `go run ./tools/dmpf-conformance/cmd/modsync --root . --check` passam; `ports` e `memory` sem `require` novo.
- [ ] `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source` e, nos projetos Go tocados, `fmt-check`, `vet`, `test-race` e `govulncheck` passam.
- [ ] As duas investigações P0 têm resultado no ADR novo, e `node tools/adr-verify.mjs --from 63` passa.

### Cenários de teste

```text
DADO uma chave ECC_NIST_EDWARDS25519 SIGN_VERIFY no KMS e o selo de 32 bytes
QUANDO Sign é chamado com ed25519 e o alias da chave
ENTÃO Signature.Key é o ARN completo da chave, Value tem 64 bytes, e openssl verifica a assinatura com o SPKI de PublicKey

DADO o fake com a mesma semente em duas execuções
QUANDO a mesma mensagem é assinada com ed25519
ENTÃO os bytes da assinatura e os identificadores das chaves são idênticos nas duas execuções

DADO uma mensagem de 4097 bytes e outra vazia
QUANDO Sign é chamado para cada uma
ENTÃO devolve ErrMessageTooLarge e ErrMessageEmpty, e o provider não faz nenhuma chamada ao KMS

DADO uma chave desabilitada
QUANDO Sign é chamado
ENTÃO devolve ErrKeyUnusable no KMS real, e contra um floci que ainda assina com ela a cláusula key.unusable consta de Skipped com o nome

DADO uma assinatura válida e a chave pública exportada
QUANDO Verify recebe a mensagem com um bit alterado, e depois a chave pública de outra chave
ENTÃO devolve ErrSignatureInvalid na primeira chamada e ErrKeyMismatch na segunda

DADO uma chave P-256 e o algoritmo ed25519
QUANDO Sign é chamado
ENTÃO devolve ErrAlgorithmNotSupportedByKey, sem assinatura
```

<critical_constraints>
- [P0] A chave privada nunca entra no kernel: o provider só chama `Sign` e `GetPublicKey`, sem `Encrypt`, `Decrypt`, `GenerateDataKey` nem `GenerateMac`.
- [P0] O algoritmo é sempre declarado pelo chamador, de um conjunto fechado; vazio ou fora do conjunto é recusado, nunca assumido.
- [P0] A assinatura registra a chave resolvida e o algoritmo; a verificação usa a chave pública exportada, sem rede nem credencial.
- [P0] O bloco `port` permanece puro (RFC §6.2): a porta usa só `context`, `errors`, `[]byte` e `Instant`; `crypto/ecdsa`, `crypto/ed25519` e `crypto/x509` ficam em provider.
- [P0] O mesmo kit roda contra o fake, o floci e o KMS real; a cláusula que um subject não exerce fica em `Skipped` com nome, nunca fora do kit.
- [P0] A garantia de que só o KMS assina e de que chave inutilizável não assina se prova contra o KMS real: o floci dá paridade de ambiente, e o teste de contrato não é opcional.
- [P0] A infra do floci (imagem, endpoint, S3 e KMS) é da SPEC-CWBA4BPV; esta spec não altera `floci.yml` nem a imagem.
- [P1] A chave de assinatura é distinta da chave de cifra do object storage (`SIGN_VERIFY` × `ENCRYPT_DECRYPT`): chaves segregadas por finalidade.
- [P1] Unidade nova exige `include` no manifesto (`DMPF-U001`) e entra no baseline por `--write-baseline` (`DMPF-T001`), feito por quem mantém o kernel.
</critical_constraints>

## Escopo fora

- **Verificação online** (`kms:Verify`): a verificação é offline, pela chave pública exportada.
- **Criar, rotacionar, desabilitar e agendar a exclusão de chaves, política de chave e IaC**: operação do ambiente; a spec consome a chave pelo `KeyRef`.
- **Montar o `SealManifest`**, a raiz Merkle e a periodicidade: responsabilidade do consumidor; a porta assina a mensagem que recebe.
- **Assinar o envelope de eventos do kernel**, emitir JWS ou JWT e verificar assinatura de terceiros: outro uso, com outra spec.
- **RSA, ML-DSA, SM2, HMAC e cifra por KMS** (`Encrypt`, `Decrypt`, data keys): a cifra em repouso é configuração do object storage (DAT-08).
- **Âncora RFC 3161 por ACT ICP-Brasil**: decisão de produto e jurídica do consumidor.
- **Chaves multirregião e outros KMS** (GCP, Azure Key Vault): a porta é neutra, mas só o AWS KMS ganha provider.
