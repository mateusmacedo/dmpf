# s3

Bloco `provider` do kernel DMPF que realiza `ports.ObjectStore` sobre o Amazon S3 com Object Lock, com `aws-sdk-go-v2`. As garantias são estas:

- toda escrita leva o checksum SHA-256 e a cifra declarada;
- quando pedida, a escrita leva uma retenção datada e sob o teto declarado (DAT-14, DAT-17);
- toda outra operação nomeia a versão;
- a loja nunca contorna GOVERNANCE.

Encurtar ou apagar uma versão GOVERNANCE por teto superveniente (DAT-16) é o pacote `admin`, com credencial própria.

Criado pela SPEC-CWBA4BPV (`docs/specs/SPEC-CWBA4BPV-porta-object-storage-worm.md`) e registrado no [ADR-073](../../../../docs/adr/073-porta-de-object-storage-e-provider-s3.md).

## Unidades do manifesto

| Unidade | Bloco | Packages |
| --- | --- | --- |
| `kernel/provider-s3` | `provider` | raiz do módulo, `admin`, `internal/sdk`, `internal/worm` |

Dependências externas declaradas, com capability `io.storage`:

- `github.com/aws/aws-sdk-go-v2` (`aws`);
- `service/s3` (com `types`);
- `github.com/aws/smithy-go`, pela interface `APIError` e pelo pacote `logging`.

Com capability `observability`, `go.opentelemetry.io/otel/trace`.

O `config`, o `credentials`, o `service/kms` e o `service/sts` entram só nos testes. Em produção, quem resolve a `aws.Config` é o composition root.

## O que o módulo contém

- **`config.go`**:
  - campos: `Config{AWS, Bucket, Endpoint, InsecureForDevelopmentOnly, UsePathStyle, Declaration, Clock, Sheet, Tracer, Instruments, LoggerProvider, Rand}`;
  - `Declaration` é a `ports.ObjectStoreDeclaration`: escopo, cifra e teto com origem;
  - `Validate` recusa região, bucket ou relógio ausentes, `UsePathStyle` sem `Endpoint` explícito e endpoint sem TLS sem o opt-out (`ErrTLSRequired`), além de declaração ou ficha incompletas;
  - uma chave KMS é declarada pelo ARN, a forma em que o S3 a devolve no `Head`; alias ou key id são recusados, porque a cifra lida nunca bateria com a declarada (DAT-08).
- **`store.go`**, `s3.Store`, com o mapeamento da porta para o S3:

  | Porta | S3 |
  | --- | --- |
  | `Put` | `PutObject` com `ChecksumSHA256`, cifra, modo e data de retenção em segundo inteiro, e o metadado `dmpf-stored-at` (o instante da loja, que o `PutObject` não devolve) |
  | `Get`, `Head` | `GetObject` e `HeadObject` com `VersionId` e `ChecksumMode` |
  | `Retain` | `PutObjectRetention`, depois de ler a retenção corrente |
  | `SetLegalHold` | `PutObjectLegalHold` |
  | `Purge` | `DeleteObject` com `VersionId`, depois de ler a versão |

  Nenhuma entrada montada pela loja preenche `BypassGovernanceRetention`.
- **`admin/`**: `Shorten` e `Purge` de versão GOVERNANCE.
  - O teto e a origem são obrigatórios (DAT-15, DAT-16).
  - O limite conta a partir da escrita: o mais tardio entre o `dmpf-stored-at` e o `LastModified` do servidor, para que nem um relógio adiantado nem um metadado forjado antecipem o encurtamento ou a purga.
  - `Purge` só apaga a versão que já passou do limite, com ou sem retenção vigente; dentro do limite devolve `ErrWithinCeiling`.
  - COMPLIANCE é recusado antes de qualquer chamada (`ErrComplianceImmutable`), e o legal hold nunca é contornado.
  - A evidência sai pelo retorno, para a auditoria do operador (DAT-18, DAT-25): `RetentionReceipt{Ref, From, To, Reason, At}` no encurtamento e `admin.PurgeReceipt` (o recibo da porta mais a origem do teto) na purga.
- **`internal/worm`**: as regras da porta que o S3 não aplica por quem chama e que a loja e o `admin` compartilham:
  - prefixos físicos `tenant/<id>/` e `platform/` (IDN-14 a IDN-20);
  - retenção datada sob o teto inclusivo;
  - prorrogação e promoção de GOVERNANCE a COMPLIANCE;
  - quando a versão pode ser apagada;
  - tradução dos cabeçalhos do S3, com falha fechada (`ErrUnexpectedResponse`).
- **`internal/sdk`**: o cliente, sem retry do SDK, porque só a ficha autoriza repetição (RES-27). O logger do SDK só emite o nível e uma mensagem fixa (DAT-22). O classificador de retentativa e a tabela fechada de categorias de FND-07 (ERR-11) também ficam aqui.

### Prontidão e ordem das recusas

`New` não toca o bucket. `Ready(ctx)` confere três coisas: versionamento `Enabled`, Object Lock `Enabled` e nenhuma retenção padrão no bucket. A retenção padrão reteria escritas que a porta declara livres, inclusive numa loja sem teto (DAT-17), e acima do teto declarado (DAT-14). Até confirmar, toda operação devolve `ErrNotReady`, que casa com `ErrBucketNotImmutable`. Uma falha pode ser repetida, e a confirmação vale enquanto a loja existir, porque o S3 não deixa desligar o lock.

As recusas seguem a ordem que o kit observa, e nenhuma faz I/O:

1. versão ausente;
2. chave inválida;
3. contexto, escopo e tenant;
4. integridade e retenção;
5. só então o portão de prontidão.

### Efeito desconhecido do `Put`

O `Put` não é repetido depois de enviado, nem pelo SDK nem pela ficha. Uma segunda tentativa criaria outra versão, e uma versão COMPLIANCE duplicada não se apaga. Quando a requisição pode ter chegado ao S3 sem resposta conclusiva, o erro casa com `ErrPutOutcomeUnknown`. Isso vale para deadline, resposta 5xx (decidida pelo status HTTP, porque o SDK do S3 não marca a falha como do servidor) e conexão caída depois de aberta. Também vale para a resposta de sucesso sem `versionId` ou com outro checksum: a versão já foi gravada. A falha que comprovadamente não saiu, como falha de dial ou de DNS, ou breaker aberto, é repetível; a resposta 4xx é recusa.

Todo erro que não é uma sentinela da porta chega opaco ao chamador: desembrulha para o erro do SDK, para `errors.Is` e `errors.As`, mas o texto só traz a operação, a categoria, o código e o status, nunca a URL com a chave física que um erro de transporte imprime (DAT-22).

Quem recebe `ErrPutOutcomeUnknown` reconcilia fora da porta. A forma é listar as versões da chave física (`s3:ListBucketVersions` com `prefix`) e comparar o `ChecksumSHA256` com o hash que enviou. A versão encontrada fica retida até o vencimento.

## Privilégio mínimo

Permissões S3 do principal da aplicação, por interface da porta, sempre restritas ao prefixo do escopo: `arn:aws:s3:::<bucket>/tenant/*` para a loja de tenant e `arn:aws:s3:::<bucket>/platform/*` para a de plataforma.

| Interface | Ações |
| --- | --- |
| `Ready` | `s3:GetBucketVersioning`, `s3:GetBucketObjectLockConfiguration` (no bucket) |
| `ObjectWriter` (`Put`) | `s3:PutObject`, `s3:PutObjectRetention` (retenção na escrita), `kms:GenerateDataKey` na chave declarada |
| `ObjectReader` (`Get`, `Head`) | `s3:GetObjectVersion`, `s3:GetObjectRetention` e `s3:GetObjectLegalHold` (sem elas o S3 omite os cabeçalhos de lock: o `Head` reporta a versão sem retenção nem hold, e as operações destrutivas seguem barradas pelo próprio S3, com `AccessDenied`), `kms:Decrypt` |
| `RetentionManager` (`Retain`, `SetLegalHold`) | as do `ObjectReader`, mais `s3:PutObjectRetention` e `s3:PutObjectLegalHold` |
| `ObjectPurger` (`Purge`) | as do `ObjectReader`, mais `s3:DeleteObjectVersion` |

Nenhuma interface recebe `s3:BypassGovernanceRetention`.

A política do bucket reforça isso em profundidade, com duas negações:

- nega o bypass a todo principal que não seja o administrativo;
- nega retenção acima do teto declarado (DAT-14), pela condição `s3:object-lock-remaining-retention-days`.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "BypassOnlyByTheAdministrativePrincipal",
      "Effect": "Deny",
      "Principal": "*",
      "Action": "s3:BypassGovernanceRetention",
      "Resource": "arn:aws:s3:::<bucket>/*",
      "Condition": { "ArnNotEquals": { "aws:PrincipalArn": "<arn da role administrativa>" } }
    },
    {
      "Sid": "RetentionWithinTheCeiling",
      "Effect": "Deny",
      "Principal": "*",
      "Action": ["s3:PutObject", "s3:PutObjectRetention"],
      "Resource": "arn:aws:s3:::<bucket>/*",
      "Condition": { "NumericGreaterThan": { "s3:object-lock-remaining-retention-days": "<teto em dias>" } }
    },
    {
      "Sid": "TransportEncrypted",
      "Effect": "Deny",
      "Principal": "*",
      "Action": "s3:*",
      "Resource": ["arn:aws:s3:::<bucket>", "arn:aws:s3:::<bucket>/*"],
      "Condition": { "Bool": { "aws:SecureTransport": "false" } }
    }
  ]
}
```

O principal administrativo é distinto da role da aplicação e é o único com `s3:BypassGovernanceRetention`. Além dele, recebe, no prefixo que administra:

- `s3:PutObjectRetention` e `s3:DeleteObjectVersion`;
- `s3:GetObjectVersion`, `s3:GetObjectRetention` e `s3:GetObjectLegalHold`.

Uma versão COMPLIANCE não é alcançada nem por ele: o S3 não deixa ninguém encurtá-la nem apagá-la.

## O que o módulo não contém

Ficam fora deste módulo:

- o provisionamento de buckets e chaves;
- a auditoria dos recibos, que é de quem chama (DAT-25);
- corpo por `io.Reader` e multipart;
- a retenção variável do S3 (event hold com duração);
- o delete sem versão, porque a porta não o tem.

## Como rodar os testes

Os testes de unidade rodam sem rede, sobre uma API falsa e um relógio falso. Eles cobrem:

- as regras de `internal/worm`;
- a tradução dos cabeçalhos;
- o `Ready` (concorrência, nova tentativa e cancelamento);
- cada operação;
- o classificador;
- o vazamento de chave e corpo na telemetria.

O teste com middleware do SDK (`Finalize`) passa as oito operações pelo cliente real, contra um `httptest`, e prova que a loja nunca envia `x-amz-bypass-governance-retention`. O mesmo teste prova que o `admin` envia o header no `PutObjectRetention` e no `DeleteObject`, mas não no `HeadObject`.

Os testes de integração levam a build tag `integration` e leem `S3_ENDPOINT`. Sem a variável eles pulam; com `CI` definido, falham. As credenciais vêm pelo SDK (`AWS_*`). Cada teste cria o próprio bucket com Object Lock e rodam:

- o kit `providerkit.ObjectStore`, sem cláusula pulada;
- o `Ready` contra bucket sem versionamento e sem lock;
- a matriz de erros do emulador;
- o `admin`.

```bash
pnpm nx run s3:test-race
```

O target sobe a infra de testes (`testkit:test-infra-up`). O `tools/dmpf-plugin/scripts/test-env.sh` preenche `S3_ENDPOINT` e as `AWS_*` com o floci, na porta 14566. A unidade de retenção do kit contra o floci é de 5 s (`S3_KIT_RETENTION_UNIT`). O kit inteiro leva cerca de 10 s.

## Contrato contra o S3 real (hmg)

O floci aceita o que o S3 recusa: escrita com retenção sem checksum, data no passado e bypass sem IAM. Por isso três provas só valem contra o S3 real:

- o kit;
- a resistência do COMPLIANCE a um principal amplo;
- a segregação do bypass.

Elas estão em `contract_test.go` (`TestContract*`, tag `integration`). Sem as variáveis `S3_CONTRACT_*` os testes pulam. Com `S3_CONTRACT_REQUIRED=1`, que o workflow define, uma variável ausente reprova.

### Provisionamento, uma vez, pelo mantenedor

1. **Environment `hmg`** no GitHub, com revisores obrigatórios e deployment branches restritos à `develop` e à `master`: a confiança OIDC olha o environment, e o workflow manual pode ser disparado de qualquer ref.
2. **Provedor OIDC** `token.actions.githubusercontent.com` na conta AWS, com audiência `sts.amazonaws.com`.
3. **Role do runner**:
   - confiança restrita a `repo:mateusmacedo/dmpf:environment:hmg`;
   - permissões `s3:*` nos buckets `arn:aws:s3:::<prefixo>-*` e nos objetos deles (o principal amplo do contrato, com o bypass, que o próprio contrato confere apagando uma versão GOVERNANCE);
   - `s3:ListAllMyBuckets`, para a varredura dos buckets que uma execução anterior deixou;
   - `kms:GenerateDataKey` e `kms:Decrypt` na chave;
   - `sts:AssumeRole` nas roles da aplicação e administrativa.
4. **Role da aplicação**:
   - confiança na role do runner;
   - as permissões por interface da tabela acima no bucket de GOVERNANCE, sem bypass.
5. **Role administrativa**:
   - confiança na role do runner;
   - as permissões do principal administrativo acima no bucket de GOVERNANCE.
6. **Chave KMS** simétrica, cuja política libera o runner e a aplicação.
7. **Bucket de GOVERNANCE**:
   - criado com Object Lock e sem retenção padrão (o `Ready` recusa o bucket que tiver);
   - com a política de bucket acima, nomeando a role administrativa;
   - com o teto em dias igual ou maior que um.
8. **Variáveis do environment `hmg`**:
   - `AWS_REGION`;
   - `S3_CONTRACT_RUNNER_ROLE_ARN`;
   - `S3_CONTRACT_BUCKET_PREFIX`, um prefixo de nome de bucket que o runner pode criar;
   - `S3_CONTRACT_KMS_KEY_ARN`;
   - `S3_CONTRACT_GOVERNANCE_BUCKET`;
   - `S3_CONTRACT_APP_ROLE_ARN`;
   - `S3_CONTRACT_ADMIN_ROLE_ARN`.

### Disparo e resultado

```bash
gh workflow run dmpf-contract-aws.yml --ref develop -f retention-unit=10s
```

O contrato passa quando:

- o kit roda sem cláusula pulada;
- `DeleteObject` com e sem bypass e `PutObjectRetention` menor ou rebaixado sobre COMPLIANCE voltam `AccessDenied`;
- a promoção de GOVERNANCE a COMPLIANCE é aceita;
- o S3 recusa escrita com retenção sem checksum ou com data no passado;
- a role da aplicação recebe `AccessDenied` ao enviar o bypass, enquanto a administrativa encurta e apaga a versão GOVERNANCE.

As versões do contrato usam retenções de poucas unidades e expiram sozinhas. Os buckets que o kit criou são apagados no fim. Um bucket com versão ainda retida fica, e a execução seguinte varre os buckets do prefixo criados há mais de uma hora.

O menor `RetainUntilDate` que o S3 aceita para a retenção explícita não está documentado. Se a unidade de 10 s for recusada, aumente o input `retention-unit`.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test-race,govulncheck -p s3
go run ./tools/dmpf-conformance/cmd/conformance --root .
```

O `vet` deste módulo roda com `-tags=integration`, para alcançar o código do contrato que só executa em hmg.

## Referências

- `docs/specs/SPEC-CWBA4BPV-porta-object-storage-worm.md`: requisitos, cláusulas do kit e critérios de aceite.
- `docs/dmpf/contexto-erros-seguranca.md`: DAT-08, DAT-11, DAT-14 a DAT-18, DAT-22, DAT-25 e IDN-11 a IDN-20.
- `docs/dmpf/resiliencia-observabilidade.md`: RES-05, RES-21 e RES-27.
- `libs/backend/go/ports/README.md`: a porta `ObjectStore`.
- `libs/backend/go/testkit/README.md`: o kit `providerkit.ObjectStore`.
