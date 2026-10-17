---
id: SPEC-SWN93XHJ
slug: revisao-kafka-msk
title: DMPF — Revisão de infraestrutura de mensageria Kafka para autorizar operação (TRP-41) com alvo AWS MSK
stage: backlog
priority: P2
depends_on: [SPEC-EAGAXQN1]
ticket_url: https://linear.app/mmda/issue/DEVS-98/spec-swn93xhj-dmpf-revisao-de-infraestrutura-de-mensageria-kafka-para
subtask_urls: []
created: 2026-10-12
---

# SPEC-SWN93XHJ: Revisão de infraestrutura Kafka para autorizar operação (TRP-41)

## Resumo

Executar a revisão de infraestrutura de mensageria que `TRP-41` exige antes de a
política Kafka de FND-06 §11 valer como autorização de operação, contra os dois
alvos do projeto: AWS MSK em produção e Redpanda em dev e na infra de testes. O hmg
das apps usa Kafka externo ao cluster; o MSK real de teste é o do ambiente de contrato
AWS. A revisão confere retenção, armazenamento em camadas, autenticação, criação de
tópicos e partições, posição inicial e a `janela_redelivery` verificável (`TRP-23`,
`TRP-24`, `TRP-24b`), mede a paridade entre Redpanda e MSK nos pontos que a
política usa e registra a conclusão. É uma spec de revisão: o produto é um
parecer, um ADR e o estado vigente da pendência, não código do kernel. A spec
nasce do consumidor externo antecipa-market, SPEC-GXGASP24 (DEVS-76), extensão
K8, cuja decisão de broker fica como norma de desenho enquanto `TRP-41` vale.

## Contexto

### Problema

- **A política é prospectiva.** `TRP-39` faz do Kafka o transporte-alvo, `TRP-40`
  mantém SNS/SQS normatizado, e `TRP-41` (`docs/dmpf/politicas-transporte.md:831`)
  diz que, enquanto §11 não passar por revisão de infraestrutura de mensageria,
  ela vale como norma de desenho e não como autorização de operação; canal Kafka
  novo exige a revisão nomeada no cabeçalho. O próprio documento registra a
  ausência de base as-is de Kafka (`:821`).
- **A pendência segue aberta nos três pontos do artefato.** O campo `Revisão` do
  cabeçalho nomeia "infraestrutura de mensageria para §11 e §12" (`:12`), o
  critério 3 do rastreio é "fora do alcance — ato de revisão humana" (`:334`), e
  a pendência 3 de §18.3 está `pendente` (`:1791`). O ADR-025 assume o risco de a
  revisão exigir ajustes na política antes da primeira operação
  (`docs/adr/025-kafka-transporte-alvo-sns-sqs-acervo.md:108-111`).
- **O kernel já implementa o que a revisão confere.** O provider `kafka` não
  provisiona tópicos (`libs/backend/go/kafka/README.md:170`) e autentica por
  SCRAM-SHA-256, SCRAM-SHA-512 ou certificado de cliente; qualquer outro
  mecanismo cai em `ErrSASLMechanism` (`kafka/auth.go:16-39`). O canal Kafka
  declara retenção de 7 dias, sem limite de tamanho, limpeza `delete`, posição
  inicial `earliest` e uma partição (`transport/channel/kafka_channel.go:13-15`,
  `:30-37`), e `KafkaWindow` deriva a `janela_redelivery` desses parâmetros
  (`transport/channel/window.go:78-115`). O consumer lê `latest` do canal e, fora
  disso, começa do início (`kafka/consumer.go:120-125`), e o `serve-consumer`
  recusa iniciar com retenção da inbox menor que a janela (ADR-056, item 8;
  `INB-14`).
- **A infra do kernel só tem Redpanda.** A imagem é `v26.2.2` (`infra/local/compose/redpanda.yml:12`),
  os usuários nascem em SCRAM-SHA-256 e as ACLs no `redpanda-init`, com a
  autorização ligada por último (`infra/local/compose/provisioning.generated.yml:35-70`);
  o overlay `hmg` não sobe broker: Postgres e Kafka são externos ao cluster, com SASL
  (`infra/k8s/overlays/hmg/kustomization.yaml:4-5`; `infra/README.md:183`). Não há
  infra de MSK, e o floci só o emula em parte (relata o consumidor externo: sobe
  um Redpanda pelo socket do Docker, https://github.com/floci-io/floci), de modo
  que não serve de evidência desta revisão.
- **Há precedente de forma.** A revisão de segurança do FND-07 foi uma spec com
  parecer versionado (`docs/dmpf/revisao-seguranca-fnd-07.md`), ADR do titular e
  estado no ledger de reconciliação, sem reescrever o artefato revisado
  (SPEC-DK8QQSDQ, `REC-011`). O ADR-058 (projeto solo) retirou a exigência de
  segunda pessoa dos controles que listou; `TRP-41` não está entre eles.

### Verificado na documentação oficial (acesso em 12/10/2026)

| Ponto | Redpanda | AWS MSK |
| ----- | -------- | ------- |
| Criação automática de tópico | `auto_create_topics_enabled` padrão `false` | `auto.create.topics.enable` padrão `false` |
| Retenção padrão | `log_retention_ms` de 1 semana | 7 dias com armazenamento em camadas; no Serverless, `retention.ms` de 7 dias e `retention.bytes` de 250 GiB |
| Armazenamento em camadas | exige licença enterprise; `redpanda.remote.read` e `.write` padrão `false` | só cluster provisionado Standard; `retention.ms` obrigatório, no mínimo 3 dias; sem tópico compactado; `cleanup.policy` não muda depois de criado; brokers Express têm armazenamento gerenciado |
| Autenticação | SCRAM padrão; `OAUTHBEARER` e `GSSAPI` exigem enterprise | provisionado: IAM, TLS mútuo com ACL ou SASL/SCRAM com ACL; Serverless: só IAM; as mensagens de erro citam SCRAM-SHA-512 |
| ACL sem regra | autorização ligada pelo `redpanda-init` | `allow.everyone.if.no.acl.found` padrão `true` |
| Offsets de grupo | `group_offset_retention_sec` de 1 semana | não verificado |

Fontes: https://docs.redpanda.com/current/manage/tiered-storage/,
https://docs.redpanda.com/current/reference/properties/topic-properties/,
https://docs.redpanda.com/current/reference/properties/cluster-properties/,
https://docs.aws.amazon.com/msk/latest/developerguide/msk-default-configuration.html,
https://docs.aws.amazon.com/msk/latest/developerguide/msk-tiered-storage.html,
https://docs.aws.amazon.com/msk/latest/developerguide/msk-guidelines-tiered-storage-topic-level-config.html,
https://docs.aws.amazon.com/msk/latest/developerguide/serverless-config.html,
https://aws.amazon.com/msk/faqs/,
https://aws.amazon.com/msk/features/ e
https://repost.aws/knowledge-center/msk-cluster-iam-sasl-scram. A documentação do
Redpanda é a `current`; a imagem do kernel é a `v26.2.2`.

### Não verificado

Cada item abaixo vira requisito de investigação.

- Se o MSK aceita SCRAM-SHA-256, que o kernel e o `redpanda-init` usam, ou só
  SCRAM-SHA-512, e se o cliente do kernel pode ganhar o mecanismo IAM do MSK.
- O efeito do armazenamento em camadas sobre `RemoteStorage` e `SizeHorizon` na
  derivação da janela, e se o `retention.ms` governa as duas camadas.
- `offsets.retention.minutes` no MSK e o efeito sobre a posição inicial.
- Aumento de partições (`KFK-04b`) e inicialização dos offsets novos no MSK e no
  Redpanda, e os limites de partição por tipo de cluster.
- Se o operador lê os cinco parâmetros de `TRP-24b` por `DescribeConfigs` em cada
  alvo, que é o que torna a janela verificável.
- Os recursos de MSK Express e a versão do floci acima da 2.1.0 para MSK.

<constraints>
- [P0] A revisão confere e não reescreve: nenhuma regra `KFK` ou `TRP` muda de texto ou de numeração; ajuste de política exigido por achado sai em ADR ou spec próprios.
- [P0] A conclusão é resultado, não premissa: o parecer pode aprovar, aprovar com ressalvas ou reprovar, e `TRP-41` só deixa de bloquear nos dois primeiros desfechos.
- [P0] Nenhuma conclusão se apoia no floci nem em documentação sem a URL e a data de acesso registradas.
- [P0] A edição do artefato promovido se limita ao campo `Revisão` do cabeçalho, à linha da pendência 3 de §18.3 e a uma nota ao fim, sem acrescentar nem remover linha antes dela: o `dmpf-verify` ancora regra e referência por número de linha.
- [P0] O estado vigente da pendência vive no ledger de reconciliação, com entrada nova na tabela e no histórico append-only.
- [P1] O escopo é §11 (as 23 regras `KFK`); §12, SNS e SQS, não entra e fica nomeado como não coberto.
- [P1] Cada ressalva traz a condição que a fecha, e cada achado traz regra ou seção, natureza, severidade e destino.
</constraints>

## Requisitos

### Funcionais

#### Papel, escopo e decisão

- [ ] **[P0] ADR novo**: nomear o titular do papel "infraestrutura de mensageria" (o mantenedor, nos termos do ADR-058, sem revisor independente), registrar a condição de re-revisão e fixar as condições de operação que o parecer autorizar (tipo de cluster MSK, mecanismo de autenticação e meio de provisionar tópicos).
  - Seguir o precedente do ADR-029 na forma: decisão, alternativa descartada, owner, justificativa e condição de retorno.
- [ ] **[P0] Escopo da revisão**: §11 inteira, as 23 regras `KFK`, mais `TRP-23`, `TRP-24`, `TRP-24b` e `TRP-37` a `TRP-41`. O registry de schemas (`KFK-13` a `KFK-18`) é conferido apenas quanto ao que depende do broker; a escolha do registry segue a pendência 8 de §18.3.

#### Investigação dos recursos do MSK

- [ ] **[P0] Investigar o armazenamento em camadas**: confirmar nas páginas de armazenamento em camadas do MSK o que governa a vida do registro nas duas camadas, o mínimo de 3 dias, a obrigatoriedade de `retention.ms` e a restrição a `cleanup.policy=delete`, e dizer como `RemoteStorage` e `SizeHorizon` de `KafkaWindow` devem ser preenchidos para a janela bater com a retenção efetiva.
- [ ] **[P0] Investigar a autenticação IAM e SASL**: confirmar o mecanismo SCRAM que o MSK aceita, as portas e o registro de segredos, e se o IAM pede um mecanismo que `kafka/auth.go` não tem (`:36-37`).
  - Dizer qual combinação de cluster e autenticação preserva a ligação principal-tópico por ACL que o ADR-052 exige, e o que muda no `requirements()` do contexto (`ClientAuth.Missing`).
- [ ] **[P0] Investigar a criação de tópicos e partições**: confirmar que a criação automática vem desligada nos dois alvos, definir o meio de provisionar (infraestrutura como código ou job de init como o `redpanda-init`), a contagem de partições por canal e a retenção por tópico, e o procedimento de `KFK-04b`.
- [ ] **[P0] Investigar a posição inicial e os offsets de grupo**: confirmar a retenção de offsets do grupo no MSK e dizer se a política `earliest` ou `latest` do canal (`kafka/consumer.go:120-125`) continua verdadeira depois de um grupo ocioso perder o offset.
- [ ] **[P0] Investigar o tipo de cluster**: comparar MSK Standard, Express e Serverless quanto aos cinco parâmetros de `TRP-24b`, à autenticação do provider e a `KFK-04b`, e recomendar o que atende os três.

#### Conferência

- [ ] **[P0] Matriz de paridade Redpanda × MSK**: uma linha por ponto que a política e o provider usam, com valor ou comportamento em cada alvo, a divergência e a ação (ajuste de infra, ajuste de política ou ressalva).
  - Pontos mínimos: retenção por tempo, retenção por tamanho, armazenamento remoto, política de limpeza, autenticação, ACL, criação de tópico, partições, posição inicial, offsets de grupo e leitura da janela.
  - Cada célula traz a URL e a data de acesso, ou `arquivo:linha` do kernel.
- [ ] **[P0] Conferência regra a regra**: cada regra do escopo recebe `confere`, `confere com ressalva`, `não confere` ou `não aplicável`, com motivo. A soma das linhas bate com a contagem declarada no verificador.
- [ ] **[P0] Provas executáveis**: o que se prova contra o Redpanda do compose ou da infra de testes, e o que só se prova contra MSK real, fica nomeado no parecer.
  - Provar que `KafkaWindow` devolve o limite lido do broker para tópicos com os parâmetros de cada alvo.
  - Provar que o consumer recusa iniciar com retenção de inbox menor que a janela.
  - Provar que, sem ACL para o principal, o acesso é negado no Redpanda, e dizer o que garante o mesmo no MSK.
- [ ] **[P0] Desfecho declarado**: aprovado, aprovado com ressalvas ou reprovado. A ressalva que depende de MSK real (teste de contrato contra um cluster MSK real do ambiente de contrato AWS) fica registrada com a condição que a fecha.

#### Registro

- [ ] **[P0] Parecer versionado**: `docs/dmpf/revisao-infra-mensageria-kafka.md`, com cabeçalho no padrão dos artefatos de `docs/dmpf/`, matriz de paridade, conferência por regra, achados e desfecho.
- [ ] **[P0] Estado no ledger**: `docs/dmpf/reconciliacao.md` recebe a entrada nova na tabela de estado atual e a entrada datada no histórico.
  - O estado é `quitada` para §11 se aprovado, `parcial` com a partição na célula se aprovado com ressalvas, e `aberta` se reprovado; a cobertura de §12 consta como não feita.
- [ ] **[P0] Cabeçalho e §18**: registrar a conclusão no campo `Revisão` (`politicas-transporte.md:12`) e na linha da pendência 3 de §18.3 (`:1791`), reescrevendo o texto das duas linhas sem alterar a contagem de linhas, e acrescentar uma nota ao fim que aponta o parecer, o ADR novo e o ledger.
  - Confirmar com `node tools/dmpf-verify.mjs` que a edição não desloca âncora.
- [ ] **[P1] Índice do acervo**: `docs/dmpf/README.md` (`:216-220`) aponta a entrada do ledger e o parecer, no lugar de repetir que a revisão está pendente.
- [ ] **[P1] Pendências derivadas**: cada achado com destino técnico (extensão de autenticação do provider, parâmetros de canal, infra de MSK) vira item rastreável fora desta spec.

### Não-funcionais

- [ ] **Rastreabilidade**: cada linha do parecer cita regra ou seção, fonte e data, e é conferível sem consultar esta spec.
- [ ] **Sem mudança normativa**: a contagem de regras das famílias `KFK` e `TRP` em `counts.json` permanece idêntica.
- [ ] **Reprodutibilidade**: os comandos de leitura dos parâmetros em cada alvo ficam registrados no parecer.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Código (`libs/`, `apps/`) | Nenhum. Achados com destino técnico saem em item próprio |
| Normas (`docs/dmpf/`) | `politicas-transporte.md`: campo `Revisão`, linha da pendência 3 e nota ao fim, sem deslocar linha |
| Registros vivos (`docs/dmpf/`) | `reconciliacao.md`, `README.md` e o parecer novo |
| Decisões (`docs/adr/`) | ADR novo |
| Verificador | `tools/tests/dmpf-verify/` só se a edição mexer em âncora |

## Localização de código

```text
docs/dmpf/politicas-transporte.md        :12 (Revisão), :1791 (pendência 3), nota ao fim
docs/dmpf/revisao-infra-mensageria-kafka.md   novo: parecer
docs/dmpf/reconciliacao.md               entrada nova (estado atual e histórico)
docs/dmpf/README.md                      índice (:216-220)
docs/adr/                                ADR novo
libs/backend/go/kafka/                   lido: auth.go, consumer.go, README.md
libs/backend/go/transport/channel/       lido: kafka_channel.go, window.go
infra/local/compose/redpanda.yml, provisioning.generated.yml   lidos
infra/test/compose.yml                   lido: Redpanda SASL dos testes
```

## Design

### Método

1. Levantar os parâmetros de `TRP-24b`, a autenticação e o provisionamento em cada alvo, na fonte oficial.
2. Montar a matriz de paridade e conferir `KFK-01` a `KFK-19`, `TRP-23`, `TRP-24` e `TRP-24b` contra ela.
3. Provar o que o Redpanda local prova; nomear o que só o MSK real prova.
4. Declarar o desfecho e os achados com destino.
5. Registrar: parecer, ADR novo, ledger, linhas do artefato e nota ao fim.

### Matriz de paridade (pontos de partida)

O hmg das apps usa Kafka externo ao cluster, com os brokers entregues por Secret de fora
(`infra/k8s/overlays/hmg/kustomization.yaml:4-6`), e não entra como coluna; a prova contra
MSK real fica com o ambiente de contrato AWS.

| Ponto | Política e provider | Redpanda (dev, infra de testes) | MSK | Situação inicial |
| ----- | ------------------- | ------------------- | --- | ---------------- |
| Retenção por tempo | 7 dias (`kafka_channel.go:15`) | padrão de 1 semana | 7 dias no tiered e no Serverless | a conferir por tópico |
| Retenção por tamanho | `-1` (`kafka_channel.go:31`) | `retention.bytes` herda do cluster | Serverless: 250 GiB por padrão | divergência possível: o tamanho pode encurtar a janela |
| Armazenamento remoto | `RemoteStorage` (`window.go:86`) | licença enterprise | só Standard provisionado | dev e infra de testes não provam o remoto |
| Limpeza | `delete` | `delete` | tiered exige `delete`, imutável | conferir |
| Autenticação | SCRAM-256/512 ou certificado | SCRAM; init em 256 | IAM, TLS, SCRAM; Serverless só IAM | lacuna de IAM; 256 não verificado |
| ACL | ADR-052 | ligada pelo init | `allow.everyone.if.no.acl.found=true` | achado provável |
| Tópicos | provider não provisiona | `rpk topic create` | criação automática desligada | meio a definir |
| Partições | 1 por canal (`kafka_channel.go:36`) | `rpk` | limites por tipo de cluster | não verificado |
| Posição inicial | `earliest` ou `latest` | offsets por 1 semana | não verificado | não verificado |
| Janela legível | `KafkaWindow` | `DescribeConfigs` | não verificado | não verificado |

### Desfecho e efeito sobre `TRP-41`

| Desfecho | Ledger | Efeito |
| -------- | ------ | ------ |
| Aprovado | `quitada` | `TRP-41` deixa de bloquear canal Kafka novo nas condições do ADR novo |
| Aprovado com ressalvas | `parcial` | canal Kafka novo é permitido onde as condições da ressalva valem; o ledger nomeia o que falta |
| Reprovado | `aberta` | `TRP-41` segue bloqueando; os achados viram itens com destino |

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Escopo | §11 apenas | §11 e §12: `TRP-41` só condiciona §11 (`:831-834`), e o cabeçalho cita §12 como revisão separada, que o ledger registra como não coberta |
| Onde registrar | Parecer, ADR novo, ledger e edição no lugar das duas linhas, com nota ao fim | Só o ledger (precedente da SPEC-DK8QQSDQ), porque `TRP-41` e o critério 3 nomeiam o cabeçalho e §18; reescrever §18, porque uma linha a mais no cabeçalho produz 143 violações do `dmpf-verify` numa cópia de teste (medido em 12/10/2026; a base tem zero) |
| Evidência | Documentação oficial com data, Redpanda local e MSK real quando existir | floci, que emula o MSK em parte |
| Titular | Mantenedor, nos termos do ADR-058 | Revisor independente, que o projeto não tem |
| Ajuste de política | Item separado | Reescrever `KFK-*` na revisão, o que a transforma em mudança normativa |

## Regras relacionadas

- FND-06: `TRP-23`, `TRP-24`, `TRP-24b`, `TRP-37` a `TRP-41`, `KFK-01` a `KFK-19`
- FND-04: `INB-14`, `INB-15`, `GAR-08`
- ADR-025, ADR-052, ADR-056 (item 8), ADR-058, ADR-029 (precedente de forma)
- RFC DMPF, M1 a M4 (a revisão não relaxa invariante)
- SPEC-EAGAXQN1 (providers de transporte), SPEC-DK8QQSDQ (precedente), SPEC-4W1BQK93 (verificador e ledger)

## Verificação e testes

### Critérios de aceite

- [ ] O parecer cobre as 23 regras `KFK` mais `TRP-23`, `TRP-24`, `TRP-24b` e `TRP-37` a `TRP-41`, com um resultado e um motivo por regra.
- [ ] A matriz de paridade tem ao menos os onze pontos mínimos, cada célula com fonte e data.
- [ ] As cinco investigações têm resposta registrada ou a marca "não verificado" com destino.
- [ ] O desfecho está declarado e cada ressalva traz a condição que a fecha.
- [ ] Há entrada nova no ledger (tabela e histórico), e o campo `Revisão` e a pendência 3 de §18.3 refletem a conclusão sem mudar a contagem de linhas.
- [ ] `node tools/dmpf-verify.mjs` termina sem violação, e as contagens de `KFK` e `TRP` em `counts.json` são as mesmas de antes.
- [ ] `TRP-41` deixa de bloquear canal Kafka novo nas condições do ADR novo, ou as condições que faltam ficam registradas no ledger.

### Cenários de teste

**Happy path — aprovação sem ressalva bloqueante**
- DADO a matriz completa, sem divergência não resolvida, e provas do Redpanda local passando
- QUANDO o desfecho é declarado
- ENTÃO o ledger registra `quitada` para §11, o ADR novo fixa as condições de operação e canal Kafka novo nessas condições não depende mais da revisão

**Edge case — retenção do MSK maior que a retenção da inbox**
- DADO um tópico MSK com os cinco parâmetros de `TRP-24b`: limpeza `delete`, `retention.ms` de 30 dias (720 h), `retention.bytes` de `-1` (sem limite), armazenamento remoto desligado e posição inicial `earliest`, e a inbox no padrão de 192 horas
- QUANDO `KafkaWindow` deriva a janela e o `serve-consumer` confere `INB-14`
- ENTÃO o menor horizonte efetivo é o de 720 h, porque não há limite de tamanho, a inbox de 192 h fica abaixo dele, o consumer recusa iniciar e o parecer registra a ação: subir a retenção da inbox ou reduzir a do tópico

**Edge case — retenção por tamanho menor que `retention.ms`**
- DADO um tópico MSK com limpeza `delete`, `retention.ms` de 30 dias (720 h), `retention.bytes` que o operador mediu em 6 dias (144 h) de tráfego, armazenamento remoto desligado e posição inicial `earliest`, e a inbox no padrão de 192 horas
- QUANDO `KafkaWindow` recebe `SizeHorizon` de 144 h e deriva a janela (`TRP-24`: o menor entre a retenção por tempo e a por tamanho, `docs/dmpf/politicas-transporte.md:565-579`)
- ENTÃO o menor horizonte efetivo é o de 144 h, e não o de 720 h, a inbox de 192 h o cobre, o consumer inicia e o parecer registra que ler só `retention.ms` recusaria a partida sem necessidade

**Edge case — cluster Serverless com autenticação só por IAM**
- DADO o MSK Serverless, que aceita só IAM
- QUANDO a conferência de autenticação compara o cluster com `kafka/auth.go`
- ENTÃO o achado nomeia a lacuna (o provider não tem mecanismo IAM) e o destino: cluster provisionado com SCRAM, ou extensão do provider em spec própria

**Erro — ACL ausente com a configuração padrão do MSK**
- DADO `allow.everyone.if.no.acl.found=true` e um tópico sem ACL para o principal de um contexto
- QUANDO a conferência de ADR-052 roda
- ENTÃO o binding principal-tópico não é imposto, o achado é bloqueante e o ADR novo exige a propriedade em `false` ou ACL por tópico antes da operação

**Erro — registro que desloca âncora**
- DADO uma edição que acrescenta uma linha ao cabeçalho de `politicas-transporte.md`
- QUANDO `node tools/dmpf-verify.mjs` roda
- ENTÃO reprova por âncora de linha, e a edição precisa voltar à mesma contagem de linhas

**Edge case — sem cluster MSK real**
- DADO que nenhum cluster MSK real, no ambiente de contrato AWS, existe quando o parecer é fechado
- QUANDO o desfecho é declarado
- ENTÃO é "aprovado com ressalvas", o ledger fica `parcial` e a ressalva diz que o teste de contrato contra MSK real fecha o resto

<critical_constraints>
- [P0] A revisão confere e não reescreve: nenhuma regra `KFK` ou `TRP` muda de texto ou de numeração; ajuste de política exigido por achado sai em ADR ou spec próprios.
- [P0] A conclusão é resultado, não premissa: o parecer pode aprovar, aprovar com ressalvas ou reprovar, e `TRP-41` só deixa de bloquear nos dois primeiros desfechos.
- [P0] Nenhuma conclusão se apoia no floci nem em documentação sem a URL e a data de acesso registradas.
- [P0] A edição do artefato promovido se limita ao campo `Revisão` do cabeçalho, à linha da pendência 3 de §18.3 e a uma nota ao fim, sem acrescentar nem remover linha antes dela: o `dmpf-verify` ancora regra e referência por número de linha.
- [P0] O estado vigente da pendência vive no ledger de reconciliação, com entrada nova na tabela e no histórico append-only.
- [P1] O escopo é §11 (as 23 regras `KFK`); §12, SNS e SQS, não entra e fica nomeado como não coberto.
- [P1] Cada ressalva traz a condição que a fecha, e cada achado traz regra ou seção, natureza, severidade e destino.
</critical_constraints>

## Escopo fora

- **Código do kernel e infra de MSK**: a spec produz o parecer e as decisões; extensão do provider (IAM), parâmetros de canal e a infra de MSK saem como itens com destino.
- **§12, SNS e SQS**: `TRP-41` só condiciona §11, e o acervo SQS tem base as-is. O ledger registra §12 como não coberta pelo campo `Revisão`.
- **Registry de schemas** (`KFK-13` a `KFK-18`): a escolha segue a pendência 8 de §18.3; aqui entra só o que depende do broker.
- **Dimensionamento por volumetria**: partições e retenção finais dependem do volume do consumidor, que não está medido; a revisão confere o mecanismo, não o número.
- **Provisionar o cluster MSK de produção e o do ambiente de contrato AWS**: é trabalho de infraestrutura da conta AWS, fora do repositório do kernel; o Kafka externo do hmg das apps chega por Secret de fora.
- **Migração do acervo SQS para Kafka**: a coexistência segue §15 e o cronograma de ANC-09.
