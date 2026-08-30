# ADR-024: Adotar REST na borda externa e gRPC no síncrono interno, com o tempo governado pela borda

## Status

Aceito — 2026-08-29. Implementa SPEC-DBTRMM3X.

## Contexto

O DMPF tem duas necessidades de transporte síncrono que a matriz de decisão de
FND-06 (§8.1) manda separar por critérios declarados, avaliados em ordem, e não
por preferência de equipe: falar com um consumidor externo à organização, e fazer
uma chamada serviço a serviço em que as duas pontas são nossas. O primeiro
critério — «quem consome» — decide antes de qualquer vantagem técnica, porque
obrigar um parceiro externo a gerar stubs de Protobuf é custo de integração sem
ganho de interoperabilidade; no interno, ao contrário, o contrato tipado e a
propagação nativa de prazo do gRPC ficam disponíveis sem custo de integração.

A segunda força é o governo do tempo. Uma cadeia de chamadas síncronas sem prazo
propagado deixa o cliente esperar indefinidamente e o servidor sem saber que
ninguém aguarda a resposta. Um timeout apenas no cliente não governa o tempo
total da cadeia; pior, se cada salto reinicia a própria contagem, o último salto
começa já sem orçamento. O contexto de execução do FND-07 já fixa que o prazo é
um instante absoluto e que a propagação é monotônica — cabe a este transporte
transmitir esse prazo pelo fio, decrescido a cada salto, sem reiniciá-lo.

A terceira força é o retry. Retentar uma operação de efeito colateral não
idempotente porque o transporte reportou indisponibilidade duplica o efeito — a
mesma armadilha que a UoW do FND-04 fecha para retry de conflito. A decisão
precisa amarrar o retry automático à idempotência comprovada do método, em vez de
tratá-lo como comportamento livre do cliente.

Este ADR cobre o transporte síncrono e o governo do tempo. O assíncrono de
domínio — Kafka, SNS e SQS — é decisão irmã e não é reaberto aqui.

## Decisão

**REST/JSON sobre HTTP é o transporte da interface para consumidor externo à
organização** (`RST-01`). O `provider` HTTP não retenta método sem semântica
idempotente (`RST-02`), aplica em toda chamada de saída um timeout derivado do
deadline em vigor (`RST-03`) e não expõe canal externo cujo contrato publicado ele
não possa referenciar (`RST-04`). A **superfície** da API — desenho de recurso,
paginação, forma do corpo de erro, versionamento — fica fora deste ADR: é contrato
e adapter de entrada que nenhum precedente encaminhou a esta âncora e que permanece
sem dona declarada (Parte-1 §7.6, `vigente` por §1.5); §9 entrega apenas o que é
política de transporte.

**gRPC sobre HTTP/2 é o transporte da chamada síncrona interna quando as duas
pontas são nossas** (`GRP-01`), com o `.proto` versionado governado pelo FND-05. Os
três perfis têm critério de escolha declarado (`GRP-02`): gRPC puro para serviço a
serviço, Connect para cliente de browser ou edge, e transcodificação gRPC-JSON
apenas quando uma superfície REST canônica for requisito formal — exceção, nunca
default. Os perfis Connect em `application/json` e a transcodificação não são
caminho de mensagem que alimente inbox, porque reescrevem a representação por
construção (`GRP-03`).

**O tempo é governado pela borda.** Toda chamada gRPC de saída carrega deadline
(`GRP-04`); o deadline é propagado e nunca reiniciado (`GRP-05`); no fio, o
`provider` transmite a duração restante e o receptor reconstrói o instante
(`GRP-06`); o cancelamento é propagado nos dois sentidos (`GRP-07`). O prazo é
declarado por método, não por serviço nem por processo (`GRP-16`), é menor que o do
chamador com folga para o próprio salto (`GRP-17`) e deriva do requisito de quem
chama, nunca da latência observada hoje (`GRP-18`).

**O retry é condicionado à idempotência.** A política de retry é declarada por
método e derivada da idempotência dele (`GRP-08`); um método sem semântica
idempotente comprovada não recebe retry automático (`GRP-09`); o backoff é
exponencial com jitter (`GRP-10`); o balanceamento entre endereços resolvidos é
configurado explicitamente (`GRP-11`) e o health checking é exposto e efetivamente
usado pelo cliente (`GRP-12`, `GRP-12b`). O modelo de erro é o do próprio gRPC, e o
mapeamento estrutural para HTTP, onde houver transcodificação, segue a tabela
canônica do ecossistema (`GRP-13`, `GRP-14`); a taxonomia de erros de domínio
permanece com o FND-07 e não é reaberta. TLS é obrigatório em produção, com
autenticação mútua por certificado recomendada no tráfego interno (`GRP-15`).

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| REST em todas as chamadas internas | O critério «quem consome» de §8.1 decide antes da uniformidade: no interno, as duas pontas são nossas, e uniformizar em REST abre mão do contrato tipado e da propagação nativa de prazo que só o gRPC oferece (`GRP-01`). |
| gRPC exposto diretamente a consumidor externo | O `rationale` de §8.1 é explícito: nenhuma vantagem técnica de Protobuf compensa exigir que um parceiro externo gere stubs — é custo de integração sem ganho de interoperabilidade (`RST-01`). |
| Transcodificação como padrão, e não exceção | A transcodificação reescreve a representação por construção e não preserva a identidade de bytes exigida das mensagens que alimentam a inbox; é perfil de exceção, para quando a superfície REST canônica é requisito formal (`GRP-02`, `GRP-03`). |
| Timeout apenas no cliente, em vez de deadline propagado | Um `provider` que substitua o prazo recebido por um valor de configuração própria quebra a cadeia, e a borda deixa de governar o tempo total; sem deadline, o cliente espera indefinidamente e o servidor não sabe que ninguém aguarda (`GRP-04`, `GRP-05`). |
| Retry livre por indisponibilidade, sem exigir idempotência | Retentar operação de efeito colateral não idempotente porque o transporte reportou indisponibilidade duplica o efeito — o mesmo critério de `UOW-10` do FND-04 (`GRP-09`). |

O §17.2 do FND-06 declara a fonte dos descartes como §8.1 `rationale`, §9
`rationale` e `GRP-04` a `GRP-09`. Como §8.2 e §10 não têm bloco `rationale`, os
motivos acima usam esses dois blocos `rationale` (§8.1 e §9) e ancoram os descartes
de exposição externa e de transcodificação nas regras `normativo` `RST-01` (§9) e
`GRP-01`, `GRP-02`, `GRP-03` (§10.1), que a faixa `GRP-04` a `GRP-09` do §17.2 não
nomeia.

## Consequências

**Positivas:**

- A escolha do transporte fica verificável por critério declarado (`TRP-37`), não
  por preferência de equipe: o consumidor externo nunca esbarra em stubs de
  Protobuf, e a chamada interna ganha contrato tipado e prazo propagado
  nativamente.
- A borda governa o tempo total da cadeia: o deadline propagado e nunca reiniciado
  (`GRP-05`) torna a monotonicidade do contexto de execução do FND-07 alcançável na
  prática, e uma chamada sem deadline passa a ser violação de política, não omissão
  tolerável (`GRP-04`).
- O retry não duplica efeito em silêncio: amarrado à idempotência do método
  (`GRP-08`, `GRP-09`) e suavizado por backoff com jitter (`GRP-10`), ele converte
  indisponibilidade curta em contrapressão controlada em vez de tempestade
  sincronizada.
- Um canal HTTP configurado sem referência ao contrato publicado passa a ser
  inoperável por construção (`RST-04`), e nenhuma chamada de saída HTTP fica sem teto
  de tempo — fechando no transporte externo a mesma brecha de espera indefinida que o
  gRPC evita porque carrega o deadline no protocolo (`RST-03`).

**Negativas:**

- Declarar deadline e política de retry **por método** (`GRP-08`, `GRP-16` a
  `GRP-18`) é trabalho de desenho método a método — não há valor global que sirva:
  um prazo único ou é frouxo para o método rápido ou aperta o lento. O custo é pago
  na configuração de cliente, e a má calibração aparece como `DEADLINE_EXCEEDED`
  prematuro. É o custo aceito de governar o tempo pela borda em vez de deixar cada
  cliente com o seu timeout isolado.
- Manter dois transportes síncronos significa duas superfícies de contrato — o
  `.proto` versionado no interno e o OpenAPI na borda externa, cuja superfície este
  ADR deixa deliberadamente fora de escopo (`TRP-03`) —, e uma capability exposta
  nas duas pontas passa a ser descrita duas vezes.

## Referências

- **ADR-014** — reserva o bloco `port` à fronteira e proíbe a aresta
  domain -> port. Importa aqui porque o transporte síncrono é um `provider` que
  implementa uma `port` na fronteira; é o que sustenta `TRP-07` manter o endereço
  concreto — tópico, ARN, URL — fora do domínio e do contrato.
- **ADR-015** — fixa as capabilities externas por bloco em default deny com
  allowlist por entrypoint. Importa aqui porque a capacidade de rede de saída que o
  `provider` REST/gRPC exige (resolver DNS, abrir conexão) só é autorizada porque a
  `port` que ele implementa a requer (RFC §6.2) — a leitura que `TRP-04` aplica ao
  recusar capability sem porta correspondente.
- Artefato de origem: `politicas-transporte.md` (FND-06), §8.2 (matriz de decisão
  de transporte), §9 (REST como interface externa) e §10 (gRPC como padrão síncrono
  interno).
- **SPEC-DBTRMM3X** — spec da série de ADRs mínimos do DMPF, que esta decisão
  implementa.
- **ARQ-448** (FND-11) — sub-spec que redige, promove e aceita os ADRs acionados
  pelo acervo; este ADR redige o acionamento `ADR-DMPF-O`, registrado em §17.2 do
  FND-06.
