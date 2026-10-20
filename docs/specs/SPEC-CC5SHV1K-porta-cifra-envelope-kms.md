---
id: SPEC-CC5SHV1K
slug: porta-cifra-envelope-kms
title: DMPF — Porta de cifra em envelope para campos PII com provider AWS KMS emulado no floci
stage: backlog
priority: P2
depends_on: [SPEC-CWBA4BPV]
ticket_url: https://linear.app/mmda/issue/DEVS-102/spec-cc5shv1k-dmpf-porta-de-cifra-em-envelope-para-campos-pii-com
subtask_urls: []
created: 2026-10-12
---

# SPEC-CC5SHV1K: Porta de cifra em envelope para campos PII

## Resumo

Dar ao kernel uma porta de cifra de campo em envelope: cada versão de registro ganha
uma chave de dados (DEK) por classe de chave declarada, gerada pelo serviço de chaves
e guardada só embrulhada pela chave mestra (KEK) da classe; o campo é cifrado
localmente com AES-256-GCM e dado associado (AAD) que amarra tenant, tabela, registro,
campo e classe. A cifra entra na fronteira de persistência, dentro da `postgres.Table`,
e não no domínio. A porta nasce com fake determinístico, kit de conformidade e a
`Table` selada (Onda 1) e segue com o provider AWS KMS (SDK Go v2) e o teste de
contrato contra o KMS real (Onda 2). O floci ≥ 2.2.0, que emula o KMS, vem da
SPEC-CWBA4BPV; esta spec só o consome.

A origem é o consumidor externo antecipa-market, SPEC-7KW0HHF6 (DEVS-77): o contexto
`identity-organization` guarda dado pessoal classificado como `PII` (`taxId`,
`legalName`, `tradeName`, `fullName`, `inviteeEmail`) e `PII_SENSITIVE`
(`Person.document`), e o design dele pede envelope para a primeira classe e envelope
com chave segregada para a segunda.

Como mantenedor do kernel, quero que a cifra de campo seja um contrato igual no fake,
no emulador e no KMS real, com a vinculação ao registro feita pelo kernel, para que
nenhum contexto escreva cliente de KMS nem monte AAD por convenção. A entrega segue
duas ondas, uma por PR; a Onda 2 só começa depois das investigações P0.

## Contexto

- **Problema**: o kernel não tem porta de cifra de campo.
  - A `Table` grava o que `Encode(S)` devolve e lê pelo `Decode`, e nenhum dos dois
    recebe contexto, tenant ou id (`libs/backend/go/postgres/table.go:32-33,324`); a
    `SnapshotTable` serializa o estado privado direto no `jsonb`
    (`libs/backend/go/postgres/snapshot_table.go:19-25`). Hoje não há onde chamar o KMS
    com prazo, nem como amarrar o valor cifrado ao registro sem que cada mapper monte
    a AAD à mão.
  - A norma fixa a cifra em repouso como pós-condição verificável na superfície
    armazenada (DAT-08, `docs/dmpf/contexto-erros-seguranca.md:1464`) e encaminha
    «algoritmo, modo de operação, custódia e rotação de chave, e chaveamento por
    tenant» à plataforma e aos épicos de kernel (`:1468-1471`); a matriz do baseline
    repete o encaminhamento (`:1580`).
  - A SPEC-MSA607GT exclui a cifra por KMS (`SPEC-MSA607GT:281`) e proíbe o
    `kms.Signer` de chamar `GenerateDataKey` e `Decrypt` (`:75`); a SPEC-CWBA4BPV cifra
    só o objeto em repouso (`SPEC-CWBA4BPV:120`).
  - O bloco `port` só admite `pure` (`docs/dmpf/rfc-dmpf-foundation-v0.1.md:768`).
    `crypto/hmac`, `crypto/sha256`, `encoding/binary` e `fmt` são puros
    (`tools/dmpf-conformance/internal/rule/stdlib.go:45,77,95,99`); `crypto/rand` é
    `io.random` (`:33`), `encoding/json` é `wire.codec` (`:37`), e `crypto/aes` e
    `crypto/cipher` não constam da tabela. A cifra local mora em provider.
  - A persistência é híbrida: coluna tipada só para o que uma consulta filtra, o resto
    no `snapshot` (ADR-053 item 4, `docs/adr/053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md:24`),
    e agregado com coluna tipada usa a `Table` escrita à mão
    (`libs/backend/go/postgres/README.md:37-38`).
- **Impacto**: até a implementação, o consumidor guarda `PII_SENSITIVE` só como hash e
  máscara e `PII` sem envelope (`antecipa-market/docs/adr/022-precedencia-dmpf-sobre-design.md:49`).
  Sem a porta, cada contexto escreveria o cliente do KMS e a vinculação, sem fake nem kit.
- **Fatos externos, conferidos em 12/10/2026** (AWS KMS):
  - `GenerateDataKey` exige chave simétrica de cifra, devolve a DEK em claro e uma cópia
    cifrada pela KMS key; a AWS manda apagar a cópia em claro da memória depois do uso.
    Com `EncryptionContext`, o `Decrypt` exige o mesmo contexto, igualdade exata e
    sensível a caixa, e falha com `InvalidCiphertextException`; o contexto aparece em
    claro no CloudTrail e não pode levar dado sensível
    (https://docs.aws.amazon.com/kms/latest/APIReference/API_GenerateDataKey.html e
    https://docs.aws.amazon.com/kms/latest/developerguide/encrypt_context.html).
  - Chave HMAC e chave assimétrica não aceitam contexto de cifra (mesma página).
  - `ReEncrypt` decifra e recifra dentro do KMS, para trocar a KMS key ou o contexto,
    sem expor o texto claro ao chamador
    (https://docs.aws.amazon.com/kms/latest/APIReference/API_ReEncrypt.html).
  - A rotação troca só o material corrente; o `Decrypt` escolhe o material antigo sem
    mudança de código. Ela não rotaciona as DEKs, não recifra dado e não mitiga DEK
    comprometida, e a AWS recomenda usar cada DEK uma vez ou poucas vezes
    (https://docs.aws.amazon.com/kms/latest/developerguide/rotate-keys.html).
- **Não verificado nesta redação** (cada item tem requisito de investigação): o suporte
  do KMS do floci 2.2.0 a `GenerateDataKey`, `Decrypt` e `ReEncrypt` com contexto de
  cifra, e a recusa de contexto divergente e de chave inutilizável (o consumidor
  reportou que a 2.0.1 assina com chave `Disabled`, `SPEC-MSA607GT:66-69`); latência,
  custo e cota por chamada; o apagamento efetivo da DEK no Go `1.26.9`.

<constraints>
- [P0] A DEK em claro nunca sai do provider: a porta expõe sessões de registro, não chaves; a DEK vive só na sessão, é apagada no `Close` e não tem cache além da execução (CTX-14, CTX-16, DAT-02).
- [P0] A cifra entra na `postgres.Table`, na fronteira de persistência; domínio e aplicação não cifram, e tenant, tabela, registro e campo da vinculação são resolvidos pelo kernel, nunca montados pelo mapper (IDN-14).
- [P0] Todo campo selado é AES-256-GCM com nonce aleatório de 96 bits e AAD canônica de tenant, tabela, registro, campo e classe; a DEK embrulhada leva contexto de cifra com tenant, tabela, registro e classe, nunca valor de campo.
- [P0] A classe é sempre declarada, de um conjunto fechado pela config; cada classe tem KEK própria, duas classes na mesma KEK são recusadas e não há classe padrão.
- [P0] Falha fechada: falha de KMS ou de integridade aborta o `Save` antes do statement e o `Load` ou a consulta sem estado parcial, com categoria de `application/failure.go`.
- [P0] Texto claro, valor selado, DEK e chave embrulhada nunca entram em log, erro, evento, métrica ou trace (DAT-06, DAT-22, ERR-06, ERR-20).
- [P0] O bloco `port` permanece puro (RFC §6.2): a porta usa só `context`, `errors`, `fmt`, `encoding/binary`, `[]byte` e strings; `crypto/aes`, `crypto/cipher` e `crypto/rand` ficam em provider.
- [P0] O mesmo kit roda contra o fake, o floci e o KMS real; a cláusula que um subject não exerce fica em `Skipped` com nome, nunca fora do kit.
- [P0] A infra do floci (imagem, endpoint, S3 e KMS) é da SPEC-CWBA4BPV; esta spec não altera `floci.yml` nem a imagem.
- [P1] As KEKs de campo são distintas da chave do object storage (SPEC-CWBA4BPV) e da chave de assinatura (SPEC-MSA607GT): chaves segregadas por finalidade.
- [P1] Unidade nova exige `include` no manifesto (`DMPF-U001`) e entra no baseline por `--write-baseline` (`DMPF-T001`), feito por quem mantém o kernel.
</constraints>

## Requisitos

### Funcionais

#### Investigações (antes da Onda 2)

- [ ] **[P0] Investigar o KMS do floci 2.2.0 contra o contrato de envelope**: rodar no floci 2.2.0 `GenerateDataKey` (`KeySpec` `AES_256`) com contexto de cifra, `Decrypt` com contexto divergente, ausente e com `KeyId` de outra chave, as duas operações com chave `Disabled` e `PendingDeletion`, e `ReEncrypt` com troca de chave. Critério: cada recusa que o floci não reproduzir vira cláusula `Skipped` nomeada contra o floci, registrada no ADR novo; se o floci não implementar `GenerateDataKey` ou `Decrypt`, a Onda 2 roda o kit só contra o fake e o KMS real, e a lacuna fica no ADR.
- [ ] **[P0] Investigar latência, custo e cota por chamada**: medir em hmg o p50 e o p99 de `GenerateDataKey` e `Decrypt` e a cota de operações criptográficas da conta, contra a volumetria do consumidor. Critério: se as chamadas de um comando (uma por classe presente na carga e uma por classe gravada na escrita) passarem de 10% do prazo do método (RES-06) no p99, o ADR registra o reúso da DEK da carga na escrita da mesma transação, nunca além dela; abaixo disso, DEK nova a cada escrita.
- [ ] **[P1] Investigar o apagamento da DEK no Go `1.26.9`**: confirmar se a stdlib oferece meio suportado de apagar o estado expandido da chave AES; sem ele, o apagamento é de melhor esforço sobre os bytes que o provider controla, e o risco residual fica no ADR.

#### Onda 1 — porta, fake, kit e `Table` selada (sem nuvem)

- [ ] **[P0] Porta em `ports`**: novo `libs/backend/go/ports/envelope.go` com `EnvelopeCipher` (`Sealer`, `Opener`, `Rewrap`), `RecordSealer` (`Seal`, `Keys`, `Close`), `RecordOpener` (`Open`, `Close`), `Binding`, `KeyClass`, `Sealed`, `WrappedKey` e `WrappedKeys`. Os tipos usam só `context`, `errors`, `fmt`, `encoding/binary`, `[]byte` e strings.
- [ ] **[P0] Vinculação completa**: `Binding{Tenant, Table, Record}` e o campo de cada `Seal` e `Open` são não vazios; faltando um, `ErrBindingIncomplete`, sem chamar o provider. `Binding.AAD(field, class)` é a única codificação da AAD: prefixo `dmpf/envelope/v1` e os cinco valores com prefixo de comprimento, sem ambiguidade entre concatenações.
- [ ] **[P0] Classe declarada**: classe vazia devolve `ErrClassNotDeclared` e classe fora da config, `ErrClassUnsupported`, ambas antes de chamar o KMS. A porta não tem classe padrão; o consumidor declara `pii` e `pii-sensitive`.
- [ ] **[P0] Formato fixado pela porta**: `Sealed` é `0x01 | nonce (12 B) | cifra | tag (16 B)`; `WrappedKeys` mapeia classe para `WrappedKey{KEK, Blob}`, com `KEK` igual ao ARN que o provider resolveu. Todo provider produz e lê esse formato.
- [ ] **[P0] Sessão por versão de registro**: o `RecordSealer` gera a DEK de uma classe no primeiro `Seal` dela, e `Keys()` devolve só as classes usadas; o `RecordOpener` desembrulha a DEK de uma classe no primeiro `Open` dela, então linha sem campo daquela classe não chama o KMS. A sessão vive dentro de uma chamada de `Load`, `Save` ou consulta e não sobrevive ao `ctx` que a criou; depois do `Close`, `Seal` e `Open` devolvem `ErrSessionClosed`. Toda escrita gera DEK nova: recifrar a DEK de um registro é regravá-lo.
- [ ] **[P0] Ocultação por construção**: `Sealed`, `WrappedKeys` e os erros da porta formatam (`%s`, `%v`, `%+v`, `%#v`) como marcador, e erro de abertura leva só classe e identificadores da vinculação (DAT-23, ERR-20).
- [ ] **[P0] `Table` selada em `postgres`**: a `Table` ganha o par opcional `Sealed{Encode, Decode}`, que recebe a sessão, e a coluna `data_keys` (`jsonb`) da tabela, como `version`; `validate` exige exatamente um dos pares e recusa `data_keys` em `Columns`. `Load`, `Save` e `Relation` abrem a sessão com o tenant de `tenantOf` (`table.go:235-245`), o nome da tabela, o id e o `ctx`. `SealedSnapshotTable` é o par selado da `SnapshotTable`, e `SealedRepository(tx, cipher)` e `SealedReader(pool, cipher)` substituem `Repository` e `Reader` na tabela selada, que recusa ser ligada sem cifra. A tabela sem `Sealed` não muda: a mudança é aditiva para `orders`, `bookings` e `reservations`.
- [ ] **[P0] Falha fechada na `Table`**: `Save` não executa statement se a selagem falha, e a UoW desfaz; `Load` devolve erro sem estado; uma linha que não abre derruba a consulta inteira.
- [ ] **[P0] Fake determinístico**: `providerkit.FakeEnvelope(seed)` em `libs/backend/go/testkit/providerkit/envelope.go` deriva uma KEK por classe da semente, embrulha a DEK com AES-GCM local e é determinístico nas chaves, nos identificadores e nos erros; o valor selado não é comparado byte a byte. Expõe `Disable(class)`, `Deny(class)`, `Rotate(class)` e a contagem de chamadas.
- [ ] **[P0] Kit `providerkit.Envelope`**: devolve `Verdict` com `Diagnostic{Clause, Rule}` e `Skipped` nomeado (`providerkit/suite.go:7-43`), consumido por `tb.Require`; o oráculo de formato e de AAD é escrito no kit, independente do subject. As cláusulas estão em Design.
- [ ] **[P1] Subject e prova na superfície**: `providerkit.FakeEnvelopeSubject(seed)` com teste do fake contra o kit, e um teste de integração da `Table` selada que lê a linha crua e prova que o `snapshot` não contém o texto claro (DAT-08 verificado por inspeção da superfície).

#### Onda 2 — provider KMS e contrato

- [ ] **[P0] Pacote `libs/backend/go/kms/envelope`**: mora no módulo `kms` da SPEC-MSA607GT; a spec que entrar primeiro cria a unidade `kernel/provider-kms` (`service/kms` em `io.network`, `docs/dmpf/rfc-dmpf-foundation-v0.1.md:749`, na faixa de `aws-sdk-go-v2` de `sqs/dmpf-units.json:13-18`, `project.json` com `layer:providers` e entrada em `shared_kernel_units`), e a outra só acrescenta o pacote.
- [ ] **[P0] `envelope.Cipher`**: implementa `EnvelopeCipher`. `GenerateDataKey` com `KeySpec` `AES_256`, a KEK da classe e o contexto de cifra; `Decrypt` sempre com o `KeyId` da KEK configurada para a classe, nunca o gravado na linha, e o mesmo contexto; `Rewrap` por `ReEncrypt`, sem DEK em claro fora do KMS. A cifra local é AES-256-GCM da stdlib com nonce de `crypto/rand`. Nenhuma outra operação do KMS sai do pacote: nem `Encrypt`, `Sign` nem `GenerateMac`.
- [ ] **[P0] Config, resiliência e erros no molde do `sqs`**: `Config` com `AWS`, `Endpoint`, `InsecureForDevelopmentOnly`, `Classes` (classe para `KeyRef`), `Clock` e `Sheet`; `Validate` recusa config incompleta, endpoint sem TLS (`sqs/config.go:27-28,49`), classe sem KEK e duas classes na mesma KEK. As três operações não têm efeito de estado, então a ficha (RES-21) autoriza retentativa só para `KeyUnavailableException`, `DependencyTimeoutException`, `KMSInternalException` e limitação de taxa (RES-27). O classificador mapeia `DisabledException` e `KMSInvalidStateException` para `ErrKeyUnusable`, `NotFoundException` para `ErrKeyNotFound`, `AccessDeniedException` para `ErrKeyAccessDenied` e `InvalidCiphertextException` e `IncorrectKeyException` para `ErrSealIntegrity`, fechado por omissão (ERR-11). Os quatro saem como `Unexpected` não retentável; os retentáveis, como `TransientDependency`; o prazo, como `DeadlineExceeded` (`libs/backend/go/application/failure.go:16-28`).
- [ ] **[P0] Telemetria sem conteúdo**: span por chamada ao KMS com operação, classe e desfecho, e contador de chamadas por classe, que alimenta a investigação 2; nenhum valor de campo, DEK, blob ou contexto de cifra além da classe (DAT-22).
- [ ] **[P0] Privilégio mínimo documentado**: o README do pacote lista, por classe, `kms:GenerateDataKey` e `kms:Decrypt` para o principal de runtime, e `kms:ReEncryptFrom` e `kms:ReEncryptTo` só para o principal de troca de KEK; KEK `SYMMETRIC_DEFAULT` com `ENCRYPT_DECRYPT`, rotação automática ligada e política com `kms:EncryptionContextKeys` exigindo as quatro chaves do contexto; a KEK `pii-sensitive` só admite os principais que abrem o campo.
- [ ] **[P0] Integração contra o floci**: testes `//go:build integration` que leem `KMS_ENDPOINT` e pulam sem ele (`sqs/testing_test.go:24-32`); a variável é a mesma da SPEC-MSA607GT e, se esta spec vier antes, entra aqui no `.env.example`, no template dele e no `env` de `.github/workflows/dmpf-go-ci.yml`. O teste cria as duas KEKs e roda o kit, e a `Table` selada roda com o provider real.
- [ ] **[P0] Teste de contrato contra o KMS real em hmg**: job `envelope` no workflow de contrato da SPEC-CWBA4BPV (`environment: hmg`, OIDC), com as KEKs `pii` e `pii-sensitive` `Enabled`, uma terceira `Disabled`, provisionadas fora desta spec, e uma role que só alcança a KEK `pii`. Roda o kit e prova que essa role recebe `ErrKeyAccessDenied` no campo `pii-sensitive` e que o blob não decifra com outro contexto.
- [ ] **[P1] ADR novo**: registra a porta, a vinculação, o formato, a `Table` selada, o pacote `kms/envelope`, os resultados das investigações e as divergências do floci. Passa em `node tools/adr-verify.mjs --from 63` e entra no índice `docs/adr/README.md`.
- [ ] **[P1] Documentação**: README de `postgres` (tabela selada), README do módulo `kms`, `libs/backend/go/testkit/README.md` e `docs/guides/dmpf-consumo-externo.md`.

### Não-funcionais

- [ ] Compatibilidade: Go `1.26.9`, `aws-sdk-go-v2` em `>=1.46.0 <2`; `ports` e `memory` sem `require` novo, `postgres` sem SDK da AWS (a cifra chega pela porta) e o SDK do KMS só no módulo `kms`.
- [ ] Prazo: as chamadas ao KMS derivam o prazo do contexto (RES-05, RES-06) e não fixam valor próprio.
- [ ] Reprodutibilidade: contra o fake, o kit é determinístico; contra o floci, cada teste cria as próprias chaves.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Port (`ports`) | Porta de cifra em envelope, vinculação, formato e erros |
| Provider (`postgres`) | `Table` selada, `SealedSnapshotTable` e a coluna `data_keys` |
| Provider (`kms/envelope`) | Provider AWS KMS com a cifra local |
| Testkit (`providerkit`) | Fake determinístico, kit e subject |
| CI/CD | `KMS_ENDPOINT` no `dmpf-go-ci.yml` e job `envelope` no workflow de contrato |
| Documentação | ADR novo, READMEs e guia de consumo externo |

## Localização de código

```text
libs/backend/go/ports/envelope.go                     porta, vinculação, formato e erros (Onda 1)
libs/backend/go/postgres/{table.go,sealed_table.go}   Table selada e SealedSnapshotTable (Onda 1)
libs/backend/go/testkit/providerkit/envelope.go       fake, kit e subject (Onda 1)
libs/backend/go/kms/envelope/                         config, cipher, classifier (Onda 2; módulo da SPEC-MSA607GT)
.env.example e tools/dmpf-plugin/templates/init/managed/.env.example__tmpl__   KMS_ENDPOINT
.github/workflows/dmpf-go-ci.yml                      env KMS_ENDPOINT
.github/workflows/dmpf-contract-aws.yml               job envelope (o arquivo vem da SPEC-CWBA4BPV)
docs/adr/0NN-cifra-de-campo-em-envelope-e-provider-kms.md   ADR novo (número a resolver)
```

## Design

### Arquitetura

```text
contexto (provider)              postgres (kernel)                   ports (puro)      providers
 declara campos e classes ──▶ Table selada: tenant, tabela, id ──▶ EnvelopeCipher ──▶ kms/envelope ──▶ AWS KMS | floci ≥ 2.2.0
 to/from recebem a sessão      abre a sessão por linha              RecordSealer        providerkit.FakeEnvelope (testes)
 domínio: texto claro em memória; superfície: snapshot selado + data_keys
```

### Assinatura da porta

```go
type KeyClass string // declarada na config do provider; sem classe padrão
type Binding struct{ Tenant TenantID; Table, Record string }
func (b Binding) AAD(field string, class KeyClass) []byte // única codificação da AAD
type Sealed []byte // 0x01 | nonce (12 B) | cifra | tag (16 B)
type WrappedKey struct{ KEK string; Blob []byte }
type WrappedKeys map[KeyClass]WrappedKey
type RecordSealer interface {
	Seal(field string, class KeyClass, plaintext []byte) (Sealed, error)
	Keys() WrappedKeys
	Close()
}
type RecordOpener interface {
	Open(field string, class KeyClass, sealed Sealed) ([]byte, error)
	Close()
}
type EnvelopeCipher interface {
	Sealer(ctx context.Context, rec Binding) (RecordSealer, error)
	Opener(ctx context.Context, rec Binding, keys WrappedKeys) (RecordOpener, error)
	Rewrap(ctx context.Context, rec Binding, keys WrappedKeys, class KeyClass, to string) (WrappedKeys, error)
}
```

### Vinculação e ciclo da chave

| Elemento | Valor | Onde se confere |
| -------- | ----- | --------------- |
| Contexto de cifra da DEK | `dmpf:tenant`, `dmpf:table`, `dmpf:record`, `dmpf:class` | no KMS, a cada `Decrypt`, e no CloudTrail |
| AAD do campo | `dmpf/envelope/v1`, tenant, tabela, registro, campo, classe | no AES-GCM local, a cada `Open` |
| DEK | uma por classe por versão de registro, nova a cada escrita | `data_keys` da linha |
| KEK | uma por classe, rotação automática no KMS | `Decrypt` com o `KeyId` da config |

O valor selado copiado para outro campo, registro, tabela ou tenant não abre, e o blob
transplantado para outra linha não desembrulha. A purga da linha (DAT-18) leva a DEK
embrulhada junto. A rotação da KEK não exige recifra; a troca de KEK usa `Rewrap`.

### Cláusulas do kit

| Cláusula | Prova |
| -------- | ----- |
| `seal.roundtrip` | para cada classe, `Open` devolve os bytes selados, inclusive o vazio, e o valor selado não contém o texto claro |
| `seal.format` | dois `Seal` do mesmo texto dão valores distintos, com versão `0x01` e tamanho `1 + 12 + n + 16` |
| `bind.aad` | o valor aberto com outro campo, registro, tabela, tenant ou classe dá `ErrSealIntegrity` |
| `bind.tamper` | um bit alterado no valor selado ou no blob dá `ErrSealIntegrity` |
| `class.declared` | classe vazia ou fora da config é recusada antes de chamar o KMS |
| `class.segregated` | sem acesso à KEK de uma classe, os campos da outra abrem e os dela dão `ErrKeyAccessDenied` (`Skipped` se o subject não oferece principal restrito) |
| `key.unusable` | KEK inexistente dá `ErrKeyNotFound`; `Disabled` ou `PendingDeletion` dá `ErrKeyUnusable`, sem valor selado nem texto claro (`Skipped` se o subject não oferece a chave) |
| `session.lifecycle` | uma chamada ao KMS por classe usada e nenhuma para classe ausente (`Skipped` se o subject não conta chamadas); após `Close`, `ErrSessionClosed` |
| `rewrap.keeps` | `Rewrap` troca a KEK de uma classe e os campos abrem sem ser regravados |
| `rotation.transparent` | depois da rotação da KEK, o registro selado antes dela abre (`Skipped` se o subject não rotaciona) |
| `redaction` | a formatação de `Sealed`, `WrappedKeys` e dos erros da porta não contém texto claro, valor selado, DEK nem blob |

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Onde a cifra entra | na `postgres.Table`, que resolve tenant, tabela, id e prazo e entrega a sessão ao mapper | domínio (I/O e chave na UPR); porta chamada pela aplicação (o caso de uso conheceria tabela e registro, e o domínio receberia cifra); mapper sozinho (`Encode` e `Decode` não têm `ctx` nem id, e cada autor montaria a AAD, o que a IDN-14 recusa) |
| Granularidade da DEK | uma por classe por versão de registro | DEK por tenant (uma chave exposta abre o tenant inteiro); DEK por campo (uma chamada ao KMS por campo) |
| Cache de DEK | nenhum além da sessão; reúso na mesma transação só se a investigação 2 o exigir | cache em processo com validade (sobrevive à execução, contra CTX-14 e CTX-16, com material de chave sensível pela DAT-02) |
| Cifra local | AES-256-GCM da stdlib, nonce aleatório de 96 bits | AES-GCM-SIV e XChaCha20-Poly1305 (fora da stdlib, dependência nova); nonce por contador (estado entre processos) |
| KEK | uma por classe declarada, distinta das da SPEC-CWBA4BPV e da SPEC-MSA607GT | KEK por tenant (chaveamento por tenant encaminhado pela DAT-08, `:1468-1471`, para spec própria) |
| Vinculação | AAD local e contexto de cifra no KMS | só AAD local (o blob seria transplantável e o CloudTrail não nomearia o registro) |
| `KeyId` no `Decrypt` | sempre a KEK da config | a KEK gravada na linha (a linha adulterada escolheria a chave) |
| Troca de KEK | `ReEncrypt` | `Decrypt` e `Encrypt` no cliente (DEK em claro fora do KMS e `kms:Encrypt` a mais) |
| Busca por igualdade | fora desta spec: o filtro usa o hash que o contexto calcula, e campo selado nunca vai para coluna tipada | índice cego HMAC aqui (ciclo de chave próprio, ver Escopo fora) |
| Onde mora o provider | pacote `kms/envelope` no módulo `kms` | módulo próprio (dois `go.mod` para o mesmo SDK de serviço) |
| Prova | kit também contra o KMS real em hmg | só floci (recusa de chave inutilizável divergente reportada na 2.0.1) |

## Regras relacionadas

- `docs/dmpf/contexto-erros-seguranca.md`: DAT-01 a DAT-03 (`:1425-1427`), DAT-06 (`:1451`), DAT-08 a DAT-10 (`:1464-1466`, encaminhamento em `:1468-1471`), DAT-11 e DAT-13 (`:1492-1494`), DAT-18, DAT-22 a DAT-24 (`:1557-1559`), CTX-14 a CTX-16 (`:697-699`), ERR-02 (`:968`), ERR-06 (`:984`), ERR-11 (`:1034`), ERR-20 (`:1093`), IDN-11 e IDN-14 (`:878-881`)
- `docs/dmpf/resiliencia-observabilidade.md`: RES-05 (`:411`), RES-21 (`:430`), RES-27 (`:671`)
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md`: §6.1 (`:749`) e §6.2 (`:768`)
- ADR-015 (capabilities por bloco), ADR-030 e ADR-045 (um módulo por lib), ADR-039 (floci), ADR-051 (escopo de tenant por choke point), ADR-053 item 4 (persistência híbrida), ADR-054 item 7 (infra de testes)
- SPEC-CWBA4BPV (floci ≥ 2.2.0 com S3 e KMS e workflow de contrato) e SPEC-MSA607GT (módulo `kms`, molde de config e classificador)
- Origem: antecipa-market, SPEC-7KW0HHF6 (DEVS-77; `docs/specs/SPEC-7KW0HHF6-identity-organization.md:64,98,432`), `docs/design/90-modelo-de-dados.md:60-61` (§1.4) e ADR-022 do consumidor, regra 3 (`docs/adr/022-precedencia-dmpf-sobre-design.md:29-30`) e addendum de 2026-10-12, linha 14 (`:7,49`)

## Verificação e testes

### Critérios de aceite

- [ ] `providerkit.Envelope` passa contra o fake sem cláusula `Skipped`, e contra o floci ≥ 2.2.0 e o KMS real com as `Skipped` nomeadas que a investigação 1 registrou.
- [ ] A linha crua de uma `Table` selada, lida fora da aplicação, não contém o texto claro de nenhum campo selado, no fake, no floci e no KMS real (DAT-08).
- [ ] Um teste de unidade, com middleware do SDK, prova que `kms/envelope` só chama `GenerateDataKey`, `Decrypt` e `ReEncrypt`, com o contexto de cifra das quatro chaves e o `KeyId` da config.
- [ ] `ports` não importa `crypto/aes`, `crypto/cipher`, `crypto/rand` nem `encoding/json`, e `postgres` não importa o SDK da AWS (`go list -deps`).
- [ ] `go run ./tools/dmpf-conformance/cmd/conformance --root .` e `go run ./tools/dmpf-conformance/cmd/modsync --root . --check` passam; `ports` e `memory` sem `require` novo.
- [ ] `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source` e, nos projetos Go tocados, `fmt-check`, `vet`, `test-race` e `govulncheck` passam.
- [ ] As três investigações têm resultado no ADR novo, e `node tools/adr-verify.mjs --from 63` passa.

### Cenários de teste

```text
DADO a tabela selada persons com as classes pii e pii-sensitive
QUANDO Save grava uma Person com fullName (pii) e document (pii-sensitive)
ENTÃO data_keys tem duas DEKs embrulhadas, o snapshot não contém nenhum dos dois valores em claro e Load devolve o mesmo agregado

DADO o valor selado de document do registro A
QUANDO ele é copiado, direto no banco, para document do registro B e para fullName de A
ENTÃO o Load de B e o de A falham com ErrSealIntegrity, categoria Unexpected não retentável, sem estado parcial

DADO a KEK pii-sensitive desabilitada
QUANDO Save é chamado para uma Person com document
ENTÃO nenhum statement executa, a UoW desfaz, o erro é ErrKeyUnusable e o registro de log só leva categoria e código

DADO uma role que só alcança a KEK pii no KMS real
QUANDO ela carrega uma Person com document
ENTÃO o Load falha com ErrKeyAccessDenied e nenhum texto claro de fullName é devolvido

DADO uma Organization sem tradeName e um comando que a carrega e grava
QUANDO o comando executa
ENTÃO há uma chamada Decrypt e uma GenerateDataKey, só da classe pii, e nenhuma DEK sobrevive ao fim da execução
```

<critical_constraints>
- [P0] A DEK em claro nunca sai do provider: a porta expõe sessões de registro, não chaves; a DEK vive só na sessão, é apagada no `Close` e não tem cache além da execução (CTX-14, CTX-16, DAT-02).
- [P0] A cifra entra na `postgres.Table`, na fronteira de persistência; domínio e aplicação não cifram, e tenant, tabela, registro e campo da vinculação são resolvidos pelo kernel, nunca montados pelo mapper (IDN-14).
- [P0] Todo campo selado é AES-256-GCM com nonce aleatório de 96 bits e AAD canônica de tenant, tabela, registro, campo e classe; a DEK embrulhada leva contexto de cifra com tenant, tabela, registro e classe, nunca valor de campo.
- [P0] A classe é sempre declarada, de um conjunto fechado pela config; cada classe tem KEK própria, duas classes na mesma KEK são recusadas e não há classe padrão.
- [P0] Falha fechada: falha de KMS ou de integridade aborta o `Save` antes do statement e o `Load` ou a consulta sem estado parcial, com categoria de `application/failure.go`.
- [P0] Texto claro, valor selado, DEK e chave embrulhada nunca entram em log, erro, evento, métrica ou trace (DAT-06, DAT-22, ERR-06, ERR-20).
- [P0] O bloco `port` permanece puro (RFC §6.2): a porta usa só `context`, `errors`, `fmt`, `encoding/binary`, `[]byte` e strings; `crypto/aes`, `crypto/cipher` e `crypto/rand` ficam em provider.
- [P0] O mesmo kit roda contra o fake, o floci e o KMS real; a cláusula que um subject não exerce fica em `Skipped` com nome, nunca fora do kit.
- [P0] A infra do floci (imagem, endpoint, S3 e KMS) é da SPEC-CWBA4BPV; esta spec não altera `floci.yml` nem a imagem.
- [P1] As KEKs de campo são distintas da chave do object storage (SPEC-CWBA4BPV) e da chave de assinatura (SPEC-MSA607GT): chaves segregadas por finalidade.
- [P1] Unidade nova exige `include` no manifesto (`DMPF-U001`) e entra no baseline por `--write-baseline` (`DMPF-T001`), feito por quem mantém o kernel.
</critical_constraints>

## Escopo fora

- **Índice cego para busca por igualdade e rotação do `salt` do hash fiscal** (pendência `90:L8` do consumidor, `SPEC-7KW0HHF6:45,64`): trocar a chave do índice obriga recalcular todos os valores e manter leitura dupla, ciclo distinto do envelope, cuja KEK rotaciona sem regravar; a chave seria uma terceira finalidade, e a HMAC do KMS não aceita contexto de cifra. Spec própria.
- **Cifra de outbox, envelope de evento, DLQ e quarantine**: o contrato do consumidor leva só referência ou hash de `PII` e nunca `PII_SENSITIVE` (`90` §1.4); essas superfícies seguem na cifra em repouso de plataforma (DAT-08, DAT-10).
- **Coluna tipada cifrada e consulta sobre campo selado**: o filtro usa o hash que o contexto calcula.
- **KEK por tenant e descarte criptográfico por tenant**: chaveamento por tenant encaminhado pela DAT-08 (`:1468-1471`).
- **Migração de linha legada em claro**: o consumidor não tem produção (ADR-022 dele, linha 10, `:45`) e usa dado sintético em hmg (linha 13, `:48`).
- **Lote de recifra ou de troca de KEK sobre todas as linhas**: operação; a spec entrega `Rewrap` e a regravação por registro.
- **Criar, rotacionar, desabilitar chaves, política de chave e IaC**: operação do ambiente; a spec consome a KEK pelo `KeyRef` da classe.
- **Tabela sem tenant (escopo de plataforma)**: a `Table` sempre escopa por tenant (ADR-051), e as tabelas de infraestrutura fora dele (ADR-050) não guardam dado de negócio.
- **Geração da tabela selada pelo generator do `dmpf-plugin`** e **outros KMS** (GCP, Azure Key Vault, chaves multirregião): a porta é neutra, mas só o AWS KMS ganha provider.
