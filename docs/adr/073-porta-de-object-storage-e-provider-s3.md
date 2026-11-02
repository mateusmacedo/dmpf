# ADR-073: Guardar evidência imutável pela porta de object storage sobre o S3 com Object Lock

## Status

Aceito — 2026-11-02. Implementa SPEC-CWBA4BPV. Emenda o [ADR-039](./039-providers-de-transporte-sink-e-gesto-de-release.md) no emulador: o floci passa da 2.0.1 à 2.2.0, fixado por digest, e emula também S3 e KMS. A prova contra o S3 real está escrita e pendente do provisionamento de hmg.

### Pendência declarada

- **owner:** mantenedor do repositório (provisiona o environment `hmg`, as roles OIDC do runner, da aplicação e administrativa, a chave KMS e o bucket de GOVERNANCE, e dispara o `dmpf-contract-aws.yml`).
- **Prazo:** antes de a SPEC-CWBA4BPV ir a `done`; os critérios de aceite que dependem do S3 real seguem abertos até o contrato passar.

## Contexto

Contextos do DMPF precisam guardar evidência que ninguém altere nem apague antes do prazo, inclusive quem tem permissão ampla no bucket. O selo do Compliance Ledger da SPEC-MSA607GT é o primeiro consumidor. O S3 com Object Lock dá essa garantia no modo COMPLIANCE. Ele tem um modo mais fraco, GOVERNANCE, que um principal com `s3:BypassGovernanceRetention` contorna, e exige a versão do objeto em toda operação sobre ela.

As normas pedem quatro coisas:

- teto de retenção declarado com origem (DAT-14 a DAT-17);
- evidência da purga (DAT-18) para a auditoria de quem chama (DAT-25);
- cifra declarada e verificável (DAT-08);
- escopo de tenant ou de plataforma declarado na construção (IDN-11 a IDN-20).

O kernel não tinha porta de armazenamento de objetos, e o emulador da infra de testes, o floci 2.0.1, tinha divergências reportadas de Object Lock.

Três investigações precederam o provider. Elas compararam o floci 2.2.0, 2.3.0 e 2.4.0 com a documentação do S3, sobre Object Lock, sobre a expiração real da retenção e sobre o provisionamento por Terraform.

**O que o floci reproduz como o S3:**

- recusas de COMPLIANCE e GOVERNANCE;
- promoção e rebaixamento;
- legal hold;
- `BadDigest`;
- cabeçalhos de cifra e checksum no `HeadObject`;
- vencimento real da retenção;
- leituras de versionamento e de lock.

**Onde o floci 2.2.0 diverge do S3:** ele aceita o que o S3 recusa:

- `PutObject` com retenção sem checksum;
- suspensão do versionamento num bucket com lock;
- retenção com modo e sem data;
- data no passado;
- retenção e hold em bucket sem lock.

Também trunca a data no segundo e não aplica a cifra padrão do bucket ao objeto gravado sem cifra explícita.

**O que muda nas versões seguintes:** a 2.4.0 fecha quatro dessas divergências e mantém o checksum não exigido e a data no passado. Além disso, a imagem 2.2.0 não traz `curl`.

## Decisão

1. **A porta mora em `ports`, com fake em `memory` e kit em `providerkit`.**

   São quatro interfaces de privilégio distinto, compostas em `ObjectStore` (DAT-11):

   | Interface | Métodos |
   | --- | --- |
   | `ObjectWriter` | `Put` |
   | `ObjectReader` | `Get`, `Head` |
   | `RetentionManager` | `Retain`, `SetLegalHold` |
   | `ObjectPurger` | `Purge` |

   As regras da porta:
   - toda referência leva o `versionId`;
   - a escrita confere o SHA-256 declarado;
   - a retenção é datada, sob o teto inclusivo declarado com origem, e só cresce ou sobe de GOVERNANCE a COMPLIANCE;
   - `Purge` só apaga versão livre e devolve `PurgeReceipt`;
   - o legal hold exige a origem (`Reason`) e devolve `HoldReceipt`;
   - o escopo é declarado na construção: `tenant/<id>/` ou o prefixo fixo `platform/`, disjuntos.

   O kit `providerkit.ObjectStore` tem as cláusulas `put.version`, `put.integrity`, `retention.dated`, `retention.compliance`, `retention.governance`, `hold.independent`, `purge.receipt`, `tenant.scope`, `platform.scope`, `head.state` e `overwrite.keeps-locked`.

2. **O provider é o módulo `libs/backend/go/s3`, unidade `kernel/provider-s3`, com o SDK em `io.storage`.**

   O mapeamento da porta para o S3 é este:

   | Porta | S3 |
   | --- | --- |
   | `Put` | `PutObject` com `ChecksumSHA256`, a cifra declarada e o modo e a data em segundo inteiro |
   | `Retain` | `PutObjectRetention`, depois de ler a retenção corrente |
   | `SetLegalHold` | `PutObjectLegalHold` |
   | `Purge` | `DeleteObject` com `VersionId`, depois de ler a versão |
   | `Get`, `Head` | `GetObject` e `HeadObject` com `VersionId` e `ChecksumMode` |

   As regras da porta que o S3 não aplica por quem chama são replicadas no pacote interno `worm`, sob a mesma prova de kit que o fake: escopo, teto, prorrogação e bloqueio da purga.

   O instante da loja viaja no metadado `dmpf-stored-at`, porque o `PutObject` não devolve data.

   Cabeçalho ausente ou malformado, ou modo de retenção desconhecido, falha fechado (`ErrUnexpectedResponse`). Todo erro que não é sentinela da porta chega opaco: desembrulha para o do SDK, mas o texto não traz a URL com a chave física (DAT-22). O emulador responde assim a uma versão que não resolve:
   - `NoSuchVersion` no `GetObject`, no `PutObjectRetention` e no `PutObjectLegalHold`;
   - `NotFound` no `HeadObject`;
   - sucesso no `DeleteObject`, como o S3.

   Essas respostas viram `ErrObjectNotFound`, e a purga lê a versão antes de apagar.

   `Ready(ctx)` confere versionamento e lock `Enabled` e recusa o bucket com retenção padrão, que reteria escritas declaradas livres (DAT-17) e acima do teto (DAT-14). Até confirmar, toda operação devolve `ErrNotReady`, que casa com `ErrBucketNotImmutable`, e o construtor não faz I/O. Uma chave KMS é declarada pelo ARN, a forma em que o `Head` a devolve (DAT-08).

3. **Só a ficha autoriza repetição, e o `Put` nunca é repetido depois de enviado.**

   O cliente desliga o retry do SDK, e as demais operações seguem a retentativa da ficha (RES-21, RES-27).

   A versão COMPLIANCE duplicada por uma repetição não se apagaria. Por isso o `Put` só repete a falha que comprovadamente não saiu:
   - falha de dial ou de DNS;
   - breaker aberto;
   - bulkhead saturado.

   O resto devolve a sentinela `ErrPutOutcomeUnknown`, sem categoria nova de FND-07. A resposta 5xx entra aqui pelo status HTTP, porque o desserializador do SDK do S3 não marca a falha como do servidor; a 4xx é recusa. A resposta de sucesso sem `versionId` ou com outro checksum também, porque a versão já foi gravada. A reconciliação é listar as versões da chave e comparar o checksum, fora da porta.

4. **O bypass de GOVERNANCE é um caminho administrativo segregado, `s3/admin`, com credencial própria.**
   - O limite conta da escrita: o mais tardio entre o metadado `dmpf-stored-at` e o `LastModified` do servidor, para que nem relógio adiantado nem metadado forjado o antecipem.
   - `Shorten` leva uma versão GOVERNANCE vigente ao limite do teto superveniente (DAT-16) e devolve `RetentionReceipt{Ref, From, To, Reason, At}`.
   - `Purge` apaga só a versão que passou do limite e devolve o recibo da porta com a origem do teto (`admin.PurgeReceipt`).
   - COMPLIANCE é recusado antes de qualquer chamada, e o legal hold nunca é contornado.
   - Um teste com middleware do SDK prova que nenhuma operação de `s3.Store` envia `x-amz-bypass-governance-retention` e que o `admin` o envia só ao encurtar e ao apagar.
   - O README do módulo dá as permissões por interface e a política de bucket que nega o bypass a todo principal fora do administrativo e a retenção acima do teto (`s3:object-lock-remaining-retention-days`).

5. **O emulador passa ao floci 2.2.0, fixado pelo digest de índice, na infra local, na de testes e nos templates do `init`.**
   - O healthcheck do Compose passa a ser o script da própria imagem (`/usr/local/bin/healthcheck.sh`, que lê `/_floci/health` só com bash).
   - `S3_ENDPOINT` entra no `.env.example` e no `env` do job do CI.
   - O kit roda contra o floci sem cláusula pulada, com unidade de retenção de 5 s e relógio que dorme: dez execuções seguidas, cerca de 10 s cada.
   - Uma divergência nova reprova por padrão. Um skip só existe como capacidade declarada do subject, em mudança própria do `testkit`.

6. **O que o floci aceita e o S3 recusa se prova contra o S3 real, pelo workflow manual `dmpf-contract-aws.yml`.**

   O workflow roda no environment `hmg`, com credencial por OIDC, e prova:
   - o kit sem cláusula pulada;
   - `AccessDenied` ao principal amplo que apaga ou encurta COMPLIANCE;
   - a promoção aceita;
   - a recusa da escrita sem checksum ou com data no passado;
   - a role da aplicação recusada ao enviar o bypass, enquanto a administrativa encurta e apaga.

   Os testes ficam na tag `integration`, para o CI compilá-los, e pulam sem as variáveis `S3_CONTRACT_*`. O workflow define `S3_CONTRACT_REQUIRED=1`, e com ele uma variável ausente reprova.

7. **Riscos e lacunas registrados:**
   - um teto superveniente mais estrito não alcança uma versão COMPLIANCE já gravada. O contexto que espera teto móvel grava em GOVERNANCE;
   - manter uma versão sob hold além do teto só é conforme depois de um ADR do kernel que emende a DAT-14;
   - a retenção variável do S3 (event hold com duração), lançada depois da spec, fica fora;
   - o menor `RetainUntilDate` que o S3 aceita para a retenção explícita não está documentado e se mede no primeiro disparo do contrato.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Pacote novo só para storage, fora de `ports` | Quebra o molde dos demais ports e da política de capabilities por bloco (ADR-015) |
| `Delete` sem versão na porta | Cria delete marker e dá a falsa sensação de remoção; a versão segue legível |
| Bypass de GOVERNANCE por opção na porta | Reabre o caminho que o COMPLIANCE fecha; o privilégio passaria a quem recebe a interface |
| Nenhum caminho de encurtamento | Um teto superveniente mais estrito (DAT-16) ficaria sem como se cumprir em GOVERNANCE |
| Extrair as regras da porta para `ports` e reabrir o `memory` | Muda a API pública de `ports` e o fake entregue; o precedente é cada provider ter as próprias regras, e o kit prova a equivalência |
| `LastModified` como instante da escrita | É o relógio do servidor, não vem na resposta do `PutObject` e pediria um `HeadObject` a mais em toda escrita |
| Retry do SDK ligado, desligado só no `Put` | Duas camadas de repetição, e a do SDK escapa da conjunção da RES-27 |
| Tag de build própria para o contrato | O CI nunca compilaria o código que só roda em hmg, e o erro apareceria no primeiro disparo |
| Divergência do emulador como cláusula pulada | O skip do kit é privado e cobre capacidades declaradas; pular esconderia defeito do provider |
| Ficar no floci 2.0.1 com cláusulas puladas, LocalStack, MinIO, SeaweedFS ou Ceph | A 2.0.1 diverge onde a 2.2.0 corrige; LocalStack está vetado (ADR-039); os demais saem do alvo AWS |
| Floci 2.4.0 já nesta entrega | Fecha quatro divergências, mas a escolha de versão foi a 2.2.0; o bump é mudança separada |

## Consequências

**Positivas:**

- O contexto recebe só a interface de que precisa, e a evidência é imutável no S3 sem que a loja tenha como contornar a retenção.
- O mesmo kit prova o fake, o emulador e, no contrato, o S3 real, com as mesmas cláusulas e sem cláusula pulada.
- O `Put` nunca duplica uma versão WORM por repetição, e o resultado incerto chega a quem chama como sentinela própria.
- O emulador da infra de testes passa a cobrir S3 e KMS no mesmo endpoint de SQS e SNS.

**Negativas:**

- As provas que dependem da recusa do servidor (checksum exigido, data no passado, `AccessDenied` por IAM) não rodam no CI. **Custo aceito:** o floci as aceita; elas ficam no contrato manual em hmg, com modo obrigatório que impede o verde sem prova.
- Toda leitura de retenção na `Retain` e na `Purge` custa um `HeadObject` antes da escrita. **Custo aceito:** a regra de prorrogação é da porta, e a recusa local evita a chamada que o S3 recusaria com outro código.
- O kit contra o servidor real dorme o relógio: cerca de 10 s no floci e mais no S3 real. **Custo aceito:** a expiração é do servidor e não se avança por injeção.
- A SPEC-CWBA4BPV não vai a `done` com o merge desta entrega. **Custo aceito:** o contrato em hmg depende de provisionamento na conta AWS do mantenedor, declarado na pendência acima.

## Referências

- `docs/specs/SPEC-CWBA4BPV-porta-object-storage-worm.md`
- `libs/backend/go/s3/README.md`: permissões por interface, política de bucket e runbook de hmg
- `libs/backend/go/testkit/README.md`: o kit `providerkit.ObjectStore`
- `docs/dmpf/contexto-erros-seguranca.md`: DAT-08, DAT-11, DAT-14 a DAT-18, DAT-22, DAT-25, IDN-11 a IDN-20
- `docs/dmpf/resiliencia-observabilidade.md`: RES-05, RES-21, RES-27
