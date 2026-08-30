# Mapa de navegação — DMPF

Guia de entrada do acervo. Responde a "onde está a regra sobre X?" sem obrigar a
abrir os oito artefatos, que somam mais de vinte mil linhas.

Este documento **não é artefato promovido**: como o
[ledger de reconciliação](./reconciliacao.md), é registro vivo, fora do rito de
monotonicidade M1–M4. Ele **aponta, nunca copia** — a norma continua morando no
artefato dono, e um mapa que replicasse texto normativo criaria a segunda verdade
que o acervo evita.

O [README](./README.md) continua sendo o índice versionado, com o estado de
promoção de cada artefato e a carta de leitura. Este mapa é a camada de busca:
por prefixo, por tema e por pergunta.

## Por prefixo

Todo identificador de regra tem a forma `PREFIXO-NN`. O prefixo diz o documento
dono — e só o dono define. As 766 regras do acervo se distribuem assim:

| Prefixo | Regras | Documento | O que governa |
|---------|--------|-----------|---------------|
| `DEC` | 13 | FND-03 | O desfecho da decisão: exaustividade, exclusividade, rejeição tipada |
| `CTR` | 7 | FND-03 | Os três níveis de contrato, e o que não atravessa entre eles |
| `UPR-I` | 12 | FND-03 | Invariantes da UPR — o que ela recebe e o que devolve |
| `UPR-L` | 5 | FND-03 | Limites da UPR — o que ela não retém e não conhece |
| `FRT` | 4 | FND-03 | A fronteira: quem carrega o estado antes da UPR |
| `MSG-N` | 4 | FND-03 | Nomenclatura de mensagem: evento no passado, comando no imperativo |
| `ESC` | 9 | FND-03 | Event Sourcing e CQRS: o que a fundação não pressupõe |
| `UOW` | 11 | FND-04 | Unit of Work: uma transação local, visível no service |
| `OBX` | 18 | FND-04 | Outbox: unicidade de `message_id`, escrita e drenagem |
| `INB` | 18 | FND-04 | Inbox: chave de deduplicação e consumo idempotente |
| `GAR` | 12 | FND-04 | Garantias de entrega — e o que a fundação não promete |
| `BLK` | 5 | FND-04 | Em que bloco cada responsabilidade mora |
| `ENV` | 25 | FND-05 | Envelope CloudEvents e o contract package |
| `PTB` | 16 | FND-05 | Forma do pacote Protobuf e versionamento de wire |
| `BUF` | 12 | FND-05 | Workspace Buf, módulos e governança do repositório de contratos |
| `REP` | 6 | FND-05 | Layout do repositório: caminho espelha pacote |
| `INT` | 5 | FND-05 | Golden fixtures como artefato de interoperabilidade |
| `TRP` | 54 | FND-06 | Transporte em geral: identidade lógica, binding, byte-preservação |
| `RST` | 4 | FND-06 | REST sobre HTTP com corpo JSON |
| `GRP` | 19 | FND-06 | gRPC sobre HTTP/2, chamada síncrona interna |
| `KFK` | 23 | FND-06 | Kafka: tópico como endereço concreto, partição, ordem |
| `SQS` | 19 | FND-06 | SNS e SQS: corpo, codificação, deduplicação |
| `ASY` | 4 | FND-06 | AsyncAPI: catálogo obrigatório do canal assíncrono |
| `COE` | 8 | FND-06 | Coexistência: um canal lógico, um transporte por vez |
| `CTX` | 28 | FND-07 | Contexto de execução e os seus nove campos |
| `IDN` | 20 | FND-07 | Identidade: autenticação, autorização, tenant |
| `ERR` | 28 | FND-07 | Taxonomia de erros e categoria única por fronteira |
| `MAP` | 7 | FND-07 | Mapeamento total de categoria para protocolo |
| `THR` | 3 | FND-07 | Limiar e orçamento: o que este artefato **não** normatiza |
| `DAT` | 26 | FND-07 | Classificação de dado e o que ela obriga |
| `RES` | 40 | FND-08 | Resiliência: timeout, retry, circuito, contenção |
| `MET` | 31 | FND-08 | Métricas: catálogo obrigatório por failure mode |
| `TRC` | 16 | FND-08 | Tracing dos três fluxos, ponta a ponta |
| `LOG` | 14 | FND-08 | Log estruturado em JSON, um evento por linha |
| `RUN` | 20 | FND-08 | Runbook executável por quem está de plantão |
| `PIR` | 18 | FND-09 | A pirâmide de teste e as suas cinco camadas |
| `CEN` | 44 | FND-09 | Cenários distribuídos, com setup, ação e resultado |
| `FIX` | 13 | FND-09 | Formato normatizado da golden fixture |
| `ORA` | 24 | FND-09 | Oráculos: o que faz o round-trip passar |
| `KIT` | 11 | FND-09 | Um kit por camada da pirâmide, com contrato declarado |
| `FIT` | 4 | FND-09 | Fitness functions: regra de dependência na suíte |
| `RAS` | 28 | FND-09 | Rastreabilidade: regra → diagnóstico → par de vetores |
| `GOV` | 33 | FND-10 | Governança e precedência entre artefato e RFC |
| `BOM` | 10 | FND-10 | Bill of materials e os seus seis itens |
| `AUT` | 10 | FND-10 | Rito de autoridade e o que o dispara |
| `ADO` | 9 | FND-10 | Adoção, com força condicionada ao estágio |
| `PIL` | 9 | FND-10 | Piloto como fluxo, não como repositório |
| `RDY` | 7 | FND-10 | Prontidão verificada contra os sete itens da DoR |

## Por documento

| Documento | Regras | Âncora na RFC | Assunto |
|-----------|--------|---------------|---------|
| [rfc-dmpf-foundation-v0.1.md](./rfc-dmpf-foundation-v0.1.md) | — | — | Limites arquiteturais e regra de dependência. Obriga todo trabalho novo |
| [inventario-as-is.md](./inventario-as-is.md) (FND-01) | — | — | Baseline candidato: o que existe hoje, com lacunas nomeadas |
| [upr-decision-mensagens.md](./upr-decision-mensagens.md) (FND-03) | 54 | `ANC-01` | UPR, desfecho da decisão e modelo de mensagens |
| [uow-inbox-outbox.md](./uow-inbox-outbox.md) (FND-04) | 64 | `ANC-02` | Fronteira transacional, inbox, outbox e garantias |
| [cloudevents-protobuf-buf.md](./cloudevents-protobuf-buf.md) (FND-05) | 64 | `ANC-03` | Envelope, Protobuf e governança de contratos |
| [politicas-transporte.md](./politicas-transporte.md) (FND-06) | 131 | `ANC-04` | REST, gRPC, Kafka, SNS/SQS e AsyncAPI |
| [contexto-erros-seguranca.md](./contexto-erros-seguranca.md) (FND-07) | 112 | `ANC-05` | Contexto, erros, segurança e multi-tenancy |
| [resiliencia-observabilidade.md](./resiliencia-observabilidade.md) (FND-08) | 121 | `ANC-06` | Resiliência, observabilidade e operação |
| [testes-interop.md](./testes-interop.md) (FND-09) | 142 | `ANC-07` | Testes e interoperabilidade entre stacks |
| [governanca-bom-pilotos.md](./governanca-bom-pilotos.md) (FND-10) | 78 | `ANC-08`, `ANC-09` | Governança, BOM, pilotos e prontidão |

FND-02 não existe. FND-11 — redação e aceite de ADRs — é trabalho futuro
(ARQ-448) e ainda não foi promovido; citações a ele não são verificáveis.

A soma das oito colunas é 766. Esse número é **derivado**: nenhum artefato o
declara, e cada um declara apenas o próprio total.

## Por pergunta

| Se a sua pergunta é… | Comece por |
|----------------------|------------|
| "Quem pode depender de quem?" | RFC, a regra de dependência |
| "O que a minha função de decisão pode receber e devolver?" | FND-03, `UPR-I` e `UPR-L` |
| "Como nomeio um evento ou um comando?" | FND-03, `MSG-N` |
| "Preciso de Event Sourcing para seguir a fundação?" | FND-03, `ESC` — a resposta é não |
| "Como escrevo na outbox dentro da transação?" | FND-04, `UOW` e `OBX` |
| "Recebi a mesma mensagem duas vezes. E agora?" | FND-04, `INB` |
| "A fundação promete exactly-once?" | FND-04, `GAR` — a resposta é não |
| "Que forma tem o envelope da mensagem?" | FND-05, `ENV` |
| "Onde ponho o meu arquivo `.proto`?" | FND-05, `REP` e `PTB` |
| "Posso publicar o mesmo canal em Kafka e em SQS?" | FND-06, `COE` — a resposta é não |
| "Que campos o contexto de execução carrega?" | FND-07, `CTX` |
| "Que categoria de erro devolver, e com que código?" | FND-07, `ERR` e `MAP` |
| "Este campo é sensível. O que isso obriga?" | FND-07, `DAT` |
| "Que métrica preciso emitir?" | FND-08, `MET` |
| "Como configuro timeout e retry?" | FND-08, `RES` |
| "O que vai no runbook?" | FND-08, `RUN` |
| "Que teste preciso escrever para esta regra?" | FND-09, `PIR`, `RAS` e `KIT` |
| "O que faz o round-trip passar?" | FND-09, `ORA` |
| "Como um artefato vira norma?" | FND-10, `GOV` e `AUT` |
| "Estou pronto para começar?" | FND-10, `RDY` |
| "Aquela pendência ainda está aberta?" | [Ledger de reconciliação](./reconciliacao.md) |

## Como citar

A convenção está no [README](./README.md): cite por seção e por ID de regra —
`FND-04 §5.3`, `OBX-12` — e reserve `arquivo:linha` para quando não houver
âncora semântica. Citação por ID e por seção sobrevive à edição do alvo;
citação por linha não.

O verificador `tools/dmpf-verify.mjs` confere mecanicamente, a cada PR que toque
o acervo, que as citações de seção resolvem, que as referências por linha
apontam para o conteúdo registrado, e que as contagens declaradas batem com as
regras que existem.

## Estado das pendências

Pendência registrada num artefato promovido não pode ser riscada nele — o
artefato não é editado retroativamente. Quem carrega o estado vigente é o
[ledger de reconciliação](./reconciliacao.md), uma linha por pendência.
Consulte-o antes de tratar como aberta uma pendência que um irmão posterior já
quitou.
