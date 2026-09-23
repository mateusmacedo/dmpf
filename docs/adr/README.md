# Decisões Arquiteturais (ADRs)

Registro das decisões técnicas relevantes do projeto. Cada decisão vive em um
arquivo próprio neste diretório (`docs/adr/`), legível sem nenhuma ferramenta
externa. Este README é o índice e o guia de escrita.

## Quando criar um ADR

Abra um ADR quando a decisão tiver alcance e trade-offs que valha a pena
preservar:

- Escolha de tecnologia, framework ou biblioteca significativa.
- Padrão arquitetural que afeta múltiplos módulos ou apps.
- Mudança de abordagem de algo já estabelecido no projeto.
- Decisão com alternativas descartadas que precisam ficar rastreáveis.

Reserve o registro para o que sobrevive à sprint. Não abra um ADR para:

- Decisão trivial ou óbvia (por exemplo, adotar UTF-8) — siga o default.
- Escolha de implementação local que não cruza fronteiras de módulo — decida
  no próprio código.
- Correção de bug — documente no commit e, se couber, no teste de regressão.
- Mudança sem alternativas em jogo — não há trade-off a registrar.

## Nomenclatura

Um arquivo por decisão, numeração sequencial e slug em kebab-case:

```text
00N-slug-em-kebab-case.md
```

O número é o próximo livre na sequência, um a mais que o último registrado
na tabela abaixo. O título dentro do arquivo repete o número: `# ADR-00N: <título>`.

## Template

Copie o esqueleto abaixo. O formato é local e **sem frontmatter** — o arquivo
começa direto no `# ADR-00N:`.

```markdown
# ADR-00N: Título imperativo em PT-BR

## Status

Aceito — AAAA-MM-DD. Implementa SPEC-XXXXXXXX.

## Contexto

Qual problema ou necessidade levou à decisão. Restrições, requisitos e
contexto relevante.

## Decisão

O que foi decidido e por quê.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Opção A     | Motivo                |
| Opção B     | Motivo                |

## Consequências

**Positivas:**

- Benefício 1

**Negativas:**

- Trade-off 1
```

### `## Status` é opcional

O bloco `## Status` pode ser omitido: ADR-001 e ADR-002 não o trazem, enquanto
ADR-003 a ADR-009 o incluem. Quando presente, o padrão adotado é
`Aceito — AAAA-MM-DD`, seguido, quando aplicável, da referência à spec que a
decisão implementa (por exemplo, `Implementa a spec de port do monorepo de origem`). Uma decisão pode
substituir outra: registre isso na própria `## Decisão`, como ADR-005 fez ao
substituir itens do ADR-004.

### Addendum para revisões datadas

Para atualizar um ADR já aceito sem reescrever a decisão histórica, acrescente
ao final um bloco `## Addendum — AAAA-MM-DD`. Ele preserva o status e a
narrativa originais e apenas registra o estado novo — ver o addendum do
ADR-001.

## Decisões registradas

Duas séries convivem no mesmo índice: os ADRs `001`–`009` vêm do template do
monorepo e os `010`–`028` vêm do acervo normativo DMPF (`docs/dmpf/`),
promovidos pela SPEC-DBTRMM3X. A numeração é contínua e única — não há prefixo
por série.

| Nº | Título | O que decide |
| --- | --- | --- |
| ADR-001 | Baseline do Monorepo Multistack | Parte do preset `--preset=ts` neutro e adiciona os plugins de forma controlada, em vez de herdar um preset focado em framework. |
| ADR-002 | Configuração de Tasks, Cache e Pipelines no NX | Define três camadas de configuração (plugin inferido, `targetDefaults`, `project.json`) e proíbe redeclarar targets para não quebrar o cache. |
| ADR-003 | Hardening do template (tooling, docs, CI/CD, infra) | Centraliza versões no `catalog:` do pnpm, fixa Node 24 / pnpm 11, migra o Biome para o formato 2.x e cria as docs de comunidade. |
| ADR-004 | Split version/publish e alvo Verdaccio | **Parcialmente supersedido pelo ADR-043** (alvo de publicação e runner). Separa o versionamento da publicação de libs — split que segue válido. |
| ADR-005 | A plataforma de hospedagem é Gitea | **Supersedido pelo ADR-043** (2026-09-11). Registro histórico: a automação falava com a plataforma pela API do Gitea. |
| ADR-006 | Fechamento do port de melhorias do monorepo derivado | Fixa a regra de desempate — a origem prevalece por padrão, o template é preservado onde está mais completo — ao portar melhorias. |
| ADR-007 | Fronteira entre o template e o plugin mmda-flow | **Supersedido** (2026-08-13). Registro histórico: bake vs consume. O template é autônomo e não depende de plugin externo. |
| ADR-008 | Destravar o release — proteções do repositório e ajuste do Nx Release ao Gitea | **Parcialmente supersedido pelo ADR-043** (identidade de automação e domínio do bot). Remove as proteções de branch e tag e o provider de release incompatível — itens que seguem válidos. |
| ADR-009 | O template não carrega libs nem generator de exemplo | Remove `shared-types`, `shared-utils` e o generator `shared-lib` obsoleto, deixando `libs/shared/` e `tools/generators/` como destinos vazios; a infraestrutura de release fica armada com um guard para conjunto vazio. |
| ADR-010 | Adotar a regra de dependência como função de decisão sobre seis blocos de pertencimento único | Expressa a regra de dependência como a função `decide(source_block, target_block, source_bc, target_bc, target_surface)` sobre seis blocos de pertencimento único, em vez de célula de matriz, e reprova a aresta quando falha a condição de bloco (`DMPF-D001`) ou a de contexto (`DMPF-D002`). |
| ADR-011 | Adotar a verification_unit como unidade arquitetural com binding idiomático por stack | Fixa um contrato abstrato único de `verification_unit`, governado pelas invariantes I1–I6, com binding idiomático por stack — o package em Go, o root declarado em manifesto no TypeScript — e resolve cada aresta pelo arquivo real, nunca pelo texto do import. |
| ADR-012 | Classificar por metadado declarado, não por convenção de diretório | Ancora `block` e `bounded_context` no manifesto `dmpf-units.json` declarado por `ownership_module`, sem herança e sem inferência de layout, com todo caso degenerado reprovando em vez de ser lido como conformidade. |
| ADR-013 | Exigir autorização distinta da autoria para mudança de classificação, com fail-closed | Trata alterar `block` ou `bounded_context`, criar e remover unidade como mudança normativa que exige autorização distinta da autoria, com baseline mantido fora do módulo que descreve e reprovação na ausência de evidência. |
| ADR-014 | Proibir a aresta `domain → port` e reservar `port` à fronteira | Reserva `port` a capacidades de fronteira, classifica interface puramente computacional como `domain` e torna a aresta `domain → port` proibida sem condicional nem exceção, corrigindo por reclassificação em vez de abrir a matriz. |
| ADR-015 | Restringir capabilities externas por bloco: default deny com allowlist por entrypoint e pureza transitiva | Aplica default deny de capabilities externas ao domínio e à porta, admite exceção apenas nominal por par unidade-dependência e computa a pureza de forma transitiva a partir dos entrypoints e da faixa de versão declarados, sempre fail-closed. |
| ADR-016 | Adotar domínio executável em memória por critério de duas metades | Exige as duas metades em conjunto — fechamento de imports apenas `pure` e teste que não inicia processo, não abre socket, não toca disco e não depende de horário — e recusa mock de infraestrutura como satisfação do critério. |
| ADR-017 | Declarar o bounded context obrigatório e limitar a interação entre contexts à superfície pública | Torna o `bounded_context` declarado e obrigatório por unidade, restringe a interação entre contexts a contrato de integração ou API pública e fixa a condição de contexto C2 que a regra de dependência consome. |
| ADR-018 | Adotar a união exaustiva como forma observável do desfecho da UPR | Fixa o desfecho da UPR como união exaustiva de dois ramos, `Accepted` ou `Rejected`, com a recusa de negócio como dado tipado de código estável, substituindo a modelagem anterior de produto de sucesso com a rejeição fora dele. |
| ADR-019 | Separar os três níveis de contrato e converter evento de domínio em evento de integração fora do domínio | Trata contrato de domínio, de aplicação e de wire como modelos distintos, mantém o tipo gerado do wire fora do domínio e move a conversão de evento de domínio em evento de integração para fora da UPR, sempre unidirecional. |
| ADR-020 | Adotar polling com leasing como relay padrão da outbox, com CDC como extensão | Elege o polling com leasing como relay padrão — claim transacional, publicação fora de qualquer transação de banco — e admite CDC apenas como extensão para volume ou latência, sem alterar a garantia at-least-once. |
| ADR-021 | Alojar o mapeamento no `provider` da outbox e serializar na escrita | Coloca o mapeamento de evento de domínio para evento de integração no `provider` da outbox e congela os bytes na escrita, dentro da transação, mantendo a porta a expor tipo de domínio e nunca tipo de wire. |
| ADR-022 | Adotar o CloudEvents Protobuf oficial, fixar `proto_data` como modalidade única e derivar o `payload_hash` dos bytes transportados | Adota o envelope `io.cloudevents.v1.CloudEvent` oficial com perfil que só acrescenta obrigatoriedade, fixa `proto_data` como modalidade única da primeira major e computa o `payload_hash` sobre os bytes de `Any.value` exatamente como transportados. |
| ADR-023 | Fixar no repositório de contratos a autoridade de validação com Buf e deferir a escolha de registry em runtime | Coloca a autoridade de validação no próprio repositório de contratos, com Buf em gates de CI locais, obrigatórios e fail-closed, e defere a escolha de registry em runtime como pendência declarada, preservando o invariante de que desserializar não depende de resolução remota. |
| ADR-024 | Adotar REST na borda externa e gRPC no síncrono interno, com o tempo governado pela borda | Fixa REST/JSON como transporte do consumidor externo e gRPC sobre HTTP/2 como padrão síncrono interno, com deadline declarado por método, propagado e nunca reiniciado, e retry condicionado à idempotência comprovada. |
| ADR-025 | Adotar Kafka como transporte-alvo do assíncrono de domínio e manter SNS/SQS normatizado no acervo | Torna Kafka o default de canal assíncrono novo, mantém SNS/SQS normatizado em vez de tolerado e fixa por transporte a ordenação, o gesto de ACK, o retry e a contenção, sob a vedação de exactly-once fim a fim. |
| ADR-026 | Adotar a baseline de resiliência e observabilidade com OpenTelemetry, retry por conjunção e auditoria separada | Restringe a baseline a `app`, `provider` e `application service`, adota OpenTelemetry com a versão das convenções fixada no BOM, autoriza retry apenas pela conjunção de quatro fatores e separa a auditoria da observabilidade. |
| ADR-027 | Governar o produto por BOM de combinação certificada, compatibilidade por sujeito versionado e escape hatch de universo fechado | Governa cada release por um BOM de combinação certificada com evidência exercitada e validade declarada, exige sujeito em toda regra de compatibilidade e fecha o escape hatch em universo positivo com regra de negação. |
| ADR-028 | Instituir o processo de autorização da classificação com autoridade fechada e evidência persistida | Cria a Autoridade de Classificação Arquitetural como função fechada, define o ato regulado pelo delta efetivo de classificação e exige commit próprio, mais de um aprovador distinto do autor e evidência persistida no repositório. |
| ADR-029 | Atribuir a revisão de Segurança do FND-07 ao owner do artefato, com divergência declarada e re-revisão condicionada | Torna o gate `THR-03` acionável nomeando o titular do papel na ausência de área de Segurança constituída, declara a perda de independência em vez de contorná-la e condiciona a re-revisão à constituição da área. |
| ADR-030 | Fixar um módulo Go por lib, com BOM declarado e segregação de stack no caminho | Faz módulo Go e projeto Nx coincidirem, fixa o caminho `libs/<scope>/<stack>/<módulo>` antes do segundo módulo nascer, declara o BOM de Go, golangci-lint e govulncheck em um lugar só por item, e explica o `package.json` privado que destrava o Nx Release. |
| ADR-031 | Decidir a regra de dependência do DMPF por execução, sobre o grafo resolvido | Entrega o verificador `conformance`, que decide C1 e C2 sobre o grafo real de imports e reprova o PR no CI. Fixa o `include` por import path exato, a política de capabilities por módulo, o baseline com membership resolvido fora do `ownership_module`, e declara as fronteiras: `DMPF-E004` inaplicável em Go, `fmt`/`time` puros por granularidade de package, e o mecanismo mínimo de RFC §10.2 cobrindo só o commit próprio. |
| ADR-032 | Realizar o desfecho da UPR em Go como par `(Accepted[R], *Rejection)` com tipo concreto | Fixa o mecanismo Go do desfecho que o ADR-018 deixou a cada kernel: segundo retorno com tipo concreto, decide-sobre-cópia para `DEC-10`, cópia defensiva da sequência com o limite declarado de `DEC-12` sobre o conteúdo, instante como valor sem `time`, e `forbidigo` como segunda camada do bloco `domain` por símbolo. Declara a adoção entre bounded contexts como consequência pendente de decisão da fundação. |
| ADR-033 | Adaptar os contratos de wire à fase monorepo sem relaxar a norma | Consolida as quatro adaptações do `KRN-05`: fonte neutra de stack em `contracts/` com o gerado Go dentro da lib `contracts`; marca de `BUF-08` como tag anotada `contracts-baseline/<módulo>` e baseline de comparação em `NX_BASE`; `reflect`/`unsafe` do gerado por exceção nominal de RFC §6.4 com o verificador intocado; e a criação da marca como ato pós-merge de uma segunda pessoa, registrado como pré-requisito organizacional. |
| ADR-034 | Realizar a fronteira de Unit of Work em Go como `UnitOfWork[R]` com vínculo no composition root | Entrega a metade de fora da UPR: fronteira genérica sobre o tipo de recursos do caso de uso, vinculada por `bind` no composition root porque `provider → application` é célula proibida; contrato de `Within` em seis cláusulas enunciadas como suíte executável; `Outcome[R]` separando o canal de negócio do técnico; identidade resolvida antes da transação com a contagem declarada pelo caso de uso; `Instant` e `MessageID` como valores de porta, sem `time`; realização em memória como unidade `provider` com falha de commit injetável, dois mutexes e commit por cópia; e resolução entre módulos irmãos pelo `go.work`, sem `require`, o que tira o `tidy` da cadeia. Declara o gate local não restringindo `provider` e o limite da técnica de vetor sobre módulos reais, em que a aresta proibida cai por ciclo de imports antes da regra de bloco. |
| ADR-035 | Realizar a outbox em PostgreSQL com serialização na escrita e mapeamento por bounded context | Decide o que FND-04 §4.1 deixa ao provider: `payload` guarda os bytes do `Any` do integration event e não o CloudEvent inteiro, porque o envelope exige atributos de FND-07 que ninguém autora nesta fase; `payload_hash` é a única coluna acrescida, para tornar "os bytes não mudam" verificável sem reserializar; tempo em `bigint` de nanossegundos, `available_at` gravado pelo provider sob `CHECK`, `metadata` nascendo `'{}'`. Põe o mapeamento evento→contrato no provider, por bounded context, único lugar onde as duas pontas são visíveis, com conferência de major na escrita. Declara o preço do teste que exige banco: build tag `integration`, Postgres no CI, e `test-race` fora dos dois caches (Nx e `go test`). Fecha a prova das células 26 e 12 com vetor sobre `git worktree`, o que o `depguard` não alcança. |
| ADR-036 | Realizar a classificação de recepção em Go como tipo fechado com `Match` exaustivo e `Pending` só sob R1 | Fecha o lado do consumo do Incremento 3: `Reception` como tipo concreto de construtores fechados com `Match` que exige os quatro ramos, e `Pending` entregue só sob R1 — `Complete` inalcançável fora dele por compilação e ausência de `Complete` dentro dele ruidosa por `ErrPendingNotCompleted`; `Register(ctx, Receipt)` porque o schema exige `message_type` e `received_at`; linha provisória de `dmpf_inbox` sobrescrita por `Complete` na mesma transação; teto de `INB-17` por `SET LOCAL lock_timeout` (o pgx fecha a conexão ao expirar o `ctx`), comprovado com `55P03`; consumer adapter no primeiro módulo `app` do workspace porque só a linha `app` da matriz alcança `contract` e `application`; `Delivery.Raw` e `envelope.Unmarshal`/`Unpack` em `contract` para que o `app` não resserialize nem importe protobuf; quarantine realizada e DLQ declarada; `Acknowledger{Ack, Release}` em `port`; taxonomia de FND-07 consumida por `Failure`/`Classify` fail-closed; `test-race` de dois módulos serializado por `dependsOn`. |
| ADR-037 | Realizar observabilidade e resiliência em Go sobre OpenTelemetry, com sampler e processor próprios e retry por conjunção | Entrega o `observability`: o gancho de instrumentação declarado em `ports` porque Go satisfaz interface por assinatura e não por estrutura; `TRC-14` realizado por sampler que devolve `RecordOnly` em vez de `Drop` e por um processor único, dono exclusivo do exportador, com a garantia declarada sobre o span e não sobre o trace; OTLP/gRPC para o Collector com TLS por default e texto claro exigindo dupla declaração; `Budget` como único valor de contexto da plataforma; retry por conjunção com taxonomia injetada, negando por omissão; e pins de OTel `v1.46.0`, `semconv/v1.43.0` e piso Go `1.26.6`, o menor que o `govulncheck` aceita com os exportadores OTLP. Declara os limites: testcontainers fora da allowlist e fora do alcance do `govulncheck`, telemetria at-least-once sob perda declarada, e a suíte passando a exigir Docker. |

| ADR-038 | Drenar a outbox com lease liberado por `NULL`, montar o envelope na publicação e tratar `payload_hash` divergente como terminal | Fecha o lado da produção do Incremento 3. Decide o que FND-04 §5.4 deixa aberto na drenagem: `Reschedule` grava `locked_until = NULL`, o que obriga o predicado de `OBX-09` a aceitar `IS NULL` — sem essa cláusula o registro ficaria preso em `publishing` para sempre, porque `locked_until <= $1` é desconhecido para um lease ausente. Os três atributos de contexto de `ENV-08` sem coluna própria são lidos de `metadata`, e o registro que não os tem **não é reivindicado**: fica `pending`, intocado, e drena sozinho no dia em que alguém gravá-los — o `KRN-09` mergeou em paralelo e deixou a lacuna aberta. `payload_hash` divergente e `aggregate_version` fora de `int32` são terminais e sanitizados, porque nenhuma nova tentativa os conserta. `envelope.Marshal` entra no bloco `contract` como simétrico de `Unmarshal`, para que o `app` não importe o tipo gerado do CloudEvent. As transições do passo 3 rodam em `context.WithoutCancel` com deadline próprio, para que uma publicação já aceita pelo broker não se perca no encerramento. `cmd/dmpf-relay` fica para o `KRN-10`, junto do transporte que ele comporia. |
| ADR-039 | Realizar os providers de transporte em Go com um módulo de primitivas compartilhadas, ponte por `Sink` com o adapter e gesto de `Release` assimétrico entre Kafka e SQS | Fecha o `KRN-10`: cinco módulos do bloco `provider` — `transport` (prazo por método sobre o `KRN-09`, catalogação de canal com as fórmulas de `janela_redelivery`, admissão por rota e tenant, as três posições de observabilidade de `RES-23`) e os providers gRPC, HTTP, Kafka e SNS/SQS. O provider nunca importa o `app`: a ponte é a interface `Sink` que o adapter realiza, e a contenção é a porta `Containment` como DLQ. `Release` é assimétrico por norma — laço inline com partição pausada em Kafka (`TRP-47`), visibilidade encurtada em SQS (`SQS-10`, sem laço por `SQS-11b`) —, o offset só avança pelo prefixo contíguo e a revogação cancela o trabalho em curso. O `Observer` é só-leitura (`TRP-17`), o corpo SQS é Base64 uma vez (`TRP-19`) e a envoltória do SNS sem raw é recusada (`SQS-02`); a matriz de hops de §5.2 virou suíte. Redpanda e floci sobem no CI; `TRP-46` fica declarado com owner e prazo. |
| ADR-040 | Realizar os test kits, o carregador de golden fixture e a fitness function em Go como um módulo de onze unidades classificadas por bloco | Fecha o `KRN-11`: `testkit` com uma unidade por package, cada uma no bloco que as suas arestas permitem — `domainkit` é `domain`, `golden` é `contract`, os fakes e as suítes de conformidade são `provider`, os harnesses e a fitness function são `app`. Veredicto por valor com `tb.Require` como adaptador, porque `domain` e `contract` não podem importar `testing`. Package exportado `fitness` no verificador, com superfície pública e sem baseline (`FIT-03`). As duas direções do round-trip no lado Go, com o oráculo 3 reprovando na direção produtor (`ENV-24`); fixtures de projeção observável em `contracts/`. O vetor `V29`/`V30` reprovou testes com duplo em duas unidades `domain` do verificador e no próprio kit — os testes se moveram, o SUT não (`PIR-17`). Suítes de UoW, inbox e outbox exportadas e as duplicatas removidas; harness distribuído com dois processos OS sobre Redpanda e `DMPF-R004`; CI em cinco estágios por tag `layer:*` e a camada distribuída em workflow próprio. |
| ADR-041 | Entregar o SDK de referência como composition root sob `apps/`, com a release do produto por tag anotada e BOM certificado | **Parcialmente supersedido pelo ADR-047** (o `go.mod` gerado deixa de ser workspace-only). Abre o `KRN-12` com `apps/backend/reference` (`type:app`, unidade `kernel/reference-app`): um binário com `--role api|relay|consumer`, configuração só por ambiente, um runtime OTel por processo com fallback em memória. Fecha a lacuna de ENV-08 dos ADR-035/038: `MessageContext` em `ports`, `metadata` autorada no provider, `causationid` = próprio `id` na origem e o `id` recebido no consumo. `api` sobre `ServeMux` com admissão por `r.Pattern` antes do corpo, `Route` com `ContractRef` para o OpenAPI de `orders`, `NewReader(pool)` sem transação. Canal nomeado por `ordersapp.Destination`, com assinatura por tipo na ponte `Sink`. Release Docker exclui `stack:go`. Transversais da guarda-chuva: tag anotada `dmpf@<semver>`, `version` efetiva no BOM, "sem edição manual", catálogo N1–N7, um transporte por processo — consolidados pelas sub-specs seguintes por addendum. |
| ADR-042 | Designar unidades do kernel como shared kernel e estender a condição de contexto ao destino designado | Institui o shared kernel como conjunto nominal de unidades designado no baseline governado (`shared_kernel_units`), e não no manifesto do módulo, porque a designação é decisão de arquitetura sobre o repositório e não do autor do módulo. C2 passa a aceitar `target.SharedKernel`, só o destino — a exceção é unidirecional e não abre o kernel ao exterior. C1 e `DMPF-M002` ficam intactas: shared kernel relaxa a condição de contexto, nunca a de bloco. `DMPF-M004` cobre a chave que não resolve para exatamente uma unidade do universo e interrompe a fase em vez de decidir arestas sobre designação inválida. Mudar a designação é ato de classificação sob o ADR-028, com `DMPF-T002` reprovando o commit que a mistura com código. A ausência da chave é distinta de `[]` — separadas no wire por `json.RawMessage`, com `null` recusado —, o que preserva byte a byte o digest legado e mantém conforme todo baseline anterior. `Input.SharedKernelUnits` do `fitness` é não-autoritativo: vale só sem store de baseline. Numeração salta o 041, reservado pela branch do ARQ-531. |
| ADR-043 | Migrar a plataforma para o GitHub, sob autoria e licença próprias | Move host, registry, runner e reusables para o GitHub; internaliza `detect-apps` e `publish-libs`; troca o import path Go e o escopo npm; adota licença MIT com autoria própria e remove o material proprietário da contratante. |
| ADR-044 | Separar a referência em um BFF REST público e dois contextos com gRPC interno | **Parcialmente supersedido pelo ADR-046** (os contextos de referência deixam o `bounded_context` `kernel`). Substitui o `reference` único por três composition roots e seis processos: `bff` é a única borda REST, em `bounded_context` próprio e sem importar domínio nem aplicação; `orders` e `reservations` servem gRPC e drenam cada um a outbox do próprio banco. Os `.proto` de serviço geram só mensagens e descriptor, porque o bloco `contract` não admite `io.network`; o binding vive no bloco `app`. Prazo e contexto de mensagem nascem na borda, retry só nos `Find*`, `Canceled` como segundo estado terminal de `reservations` e e2e caixa-preta com seis processos. A saúde fica fora da cadeia de interceptors do contexto. No kernel, `resilience.Breaker` ganha `CountsAsFailure` e o `grpc` só conta indisponibilidade, então `NOT_FOUND` não abre o breaker. |
| ADR-045 | Nomear projetos e packages pelo basename do diretório e fundir cada contexto de negócio em um módulo Go | **Parcialmente supersedido pelo ADR-046** (o módulo do contexto vive em `apps/backend`, como app; o verificador é tooling em `tools/dmpf-conformance`). Remove o prefixo de produto (`dmpf-`, `dmpf-reference-`), o prefixo de papel (`provider-`) e o sufixo de stack (`-go`) de todo projeto Nx, diretório, module path, pacote npm privado e package Go: `libs/backend/go/grpc` é o projeto `grpc` com `package grpc`. Contextos de negócio passam a um módulo por contexto, com um package por bloco (`bookings/domain`, …), `layer:` do bloco mais alto e `test-race` dependente do `postgres` do kernel; o generator `bounded-context` gera essa forma. Colisão de nome entre packages importados no mesmo arquivo resolve-se com alias pelo papel local (`provider`, `kernel`, `usecase`, `port`), nunca por concatenação. Binários `cmd/*` e `bounded_context` do kernel seguem a mesma regra (`cmd/conformance`, `kernel`). Supersede parcialmente o ADR-030 e o seu addendum de 2026-09-10; o tooling (`tools/dmpf-plugin`, `tools/dmpf-*.sh`) e o vocabulário da RFC (`dmpf-units.json`, `DMPF-D002`, `DMPF_*`) ficam intactos, e o BOM `0.1.0` acompanha a árvore pela máquina de estados de `BOM-07`. |
| ADR-046 | Reservar `libs/backend/go` ao kernel de reuso, com os contextos de exemplo e o golden como apps e o verificador como tooling | Deixa em `libs/backend/go` só os quatorze módulos que um contexto novo importa: o shared kernel, o `testkit` genérico e o novo `memory` (`kernel/provider-memory`, designado shared kernel, com `Table[ID, S]` no lugar dos repositórios tipados pelos exemplos). `orders` e `reservations` viram bounded contexts completos em `apps/backend`, com `bounded_context` próprio e o layout do `bookings`; `appkit` e `distkit` vão com `reservations` como harness do exemplo; `bookings` passa a `type:app` e o generator gera em `apps/backend`; `conformance` vai para `tools/dmpf-conformance`, sem `layer:*` e com step próprio no CI. `contracts` fica em `libs`, com `orders/contract` e `reservations/contract` como unidades dos contextos, e as libs testam com fixture `counter` própria. O baseline é regravado em commit próprio; o BOM e a evidência `0.1.0` divergem da árvore até a próxima release; as tabelas `dmpf_example_*` não são renomeadas. Supersede parcialmente o ADR-044 e o ADR-045. |
| ADR-047 | Separar a tag de módulo Go da tag de release do produto e abrir o consumo dos módulos fora do workspace | Quebra a linha única de tag em três release groups: `go-libs` (`directory:libs/backend/go/*`, padrão `libs/backend/go/{projectName}/v{version}`), `go-tools` (`conformance`, padrão literal) e `npm` (`{projectName}@{version}`), todos com version actions do `@nx/js` e nenhum tocando `go.mod` — a versão vive no `package.json` privado. O `go.mod` deixa de ser workspace-only (ADR-041) e declara `require` versionado dos irmãos; o `replace` versionado fica só no `go.work`, com o `dmpf-modsync` como dono e gate `--check` no CI. A tag do produto sai do rito manual e vira o `dmpf-release.yml`, com BOM, evidência por `workflow_call` e ausência de `dmpf@<semver>` como gates. O `DMPF-B012` exige que a tag `<dir>/v<versão>` de toda entrada `kernel` não rejeitada seja ancestral do commit da release, a partir de `0.2.0`; a ancestralidade entra por `Input.Ancestry`, porque o pacote `bom` não fala com git. O generator devolve callback pós-flush que roda o modsync e nunca escreve no `nx.json`. `release.docker` sai da raiz (forçava `requireSemver: false` em todo grupo), a cascata de versão pelo grafo bruto é aceita e os `require` ficam na versão mínima. |
| ADR-049 | Fazer o contexto de execução viajar no `context.Context`, do ingress ao provider | Supersede parcialmente `CTX-03` e `CTX-05` de FND-07: informação transitória do escopo de uma requisição passa a viver no `context.Context`, e o parâmetro `execution ports.ExecutionContext` sai dos onze métodos dos três application services, de `application.AuthorizeWithContext[C]` e de `http.ResolveIdentity`. O carrier canônico (`WithExecutionContext`, `ExecutionContextFrom`, `RequireExecutionContext`) passa ao `port`, absorvendo os dois locais duplicados entre `orders` e `reservations`. Permanecem `CTX-01`, `CTX-02`, `CTX-04`, `CTX-06` e a metade de `CTX-03` que mantém a UPR fora do alcance do contexto — o `domain` segue recebendo valores extraídos (FND-03 `FRT-03`). A garantia por compilador é trocada por fail-closed em execução: `RequireExecutionContext` devolve erro e o consumidor nega, sob `IDN-15` e `IDN-17`. Habilita o `postgres.Table[ID,S]` da Fase 3 a alcançar o tenant sem mudar `ports.Repository` nem `ports.Reader`, e deixa `ports.UnitOfWork.Within` intacto. Desfaz parcialmente o `699c662`: `AuthorizeWithContext` perde o parâmetro que ele acabara de acrescentar. |
| ADR-050 | Deixar as tabelas de infraestrutura fora do escopo de tenant | `dmpf_outbox`, `dmpf_inbox` e `dmpf_quarantine` são de plataforma: operadas pelo kernel antes do tenant, por cima de todos eles ou sobre mensagem de tenant desconhecido. O tenant atravessa a outbox no `metadata`, resolvido do portador, sem virar escopo dela. |
| ADR-051 | Impor o escopo de tenant por choke point em Go, e não por RLS | RLS medido como inerte (o role é superuser e dono das tabelas nos três ambientes). O escopo vive no `postgres.Table`, com `Tx.Conn()` fechado, o pool só como `ReadPool` e a regra `context-provider` negando o driver nos providers de contexto; o acesso cruzado vira `CrossTenantAccess` e evento de segurança. O híbrido com RLS segue disponível quando houver role de aplicação. |

## Reconciliação com a tabela §8 do épico ARQ-436

O épico ARQ-436 lista, em §8,
quinze linhas de «ADRs mínimos». O acervo DMPF produziu **dezenove**
acionamentos, promovidos um a um para `010`–`028`. Os dois conjuntos não são
idênticos, e a correspondência é **por assunto, não por número**: a série do
épico e a de `docs/adr/` são independentes, e forçar identidade entre elas
colidiria com os `001`–`009` do template.

> **Aviso de leitura.** Na coluna «Linha §8» abaixo, `ADR-001` a `ADR-015` são
> identificadores **da tabela do épico**, uma série externa a este repositório.
> Eles são marcados com o prefixo `§8` e **não** designam os arquivos
> `docs/adr/001`–`docs/adr/015`, que tratam de outros assuntos. Os arquivos deste
> repositório aparecem sem prefixo, na coluna «Desfecho», e estão todos na faixa
> `010`–`028`.

### As quinze linhas do épico

| Linha §8 | Decisão esperada (texto do épico) | Desfecho |
| --- | --- | --- |
| `§8 ADR-001` | Limites arquiteturais e regra de dependência | ADR-010, ADR-014 e ADR-015 |
| `§8 ADR-002` | UPR síncrona e determinística com `Decision` explícita | ADR-018 |
| `§8 ADR-003` | Separação entre contratos de domínio, aplicação e wire | ADR-019 |
| `§8 ADR-004` | Protobuf como contrato de wire e não como modelo de domínio | ADR-019, pela regra `CTR-02` |
| `§8 ADR-005` | CloudEvents Protobuf e perfil organizacional | ADR-022 e ADR-023 |
| `§8 ADR-006` | Unit of Work explícita no application service | **Sem ADR próprio.** `docs/dmpf/uow-inbox-outbox.md` §10.3 declarou o critério — só vira ADR estrutural a decisão que altera qual bloco conhece qual, ou que a RFC exige por âncora — e acionou apenas `K` e `L`. |
| `§8 ADR-007` | Outbox transacional e relay por polling com leasing | ADR-020 e ADR-021 |
| `§8 ADR-008` | Inbox transacional e semântica de ACK/redelivery | **Sem ADR próprio**, pelo mesmo critério de `docs/dmpf/uow-inbox-outbox.md` §10.3: a decisão altera o conteúdo da norma dentro de uma atribuição de blocos já dada. |
| `§8 ADR-009` | At-least-once com efeitos idempotentes | **Sem ADR próprio**, pelo mesmo critério de `docs/dmpf/uow-inbox-outbox.md` §10.3; a garantia é reafirmação de obrigação que a base conceitual já listava. |
| `§8 ADR-010` | REST/OpenAPI externo e gRPC/Protobuf interno | ADR-024 |
| `§8 ADR-011` | Políticas para Kafka e SNS/SQS | ADR-025 |
| `§8 ADR-012` | Execution context, identidade e multi-tenancy | **Sem ADR próprio.** O registro da âncora ANC-05 diz «ADR exigido: Não, salvo alteração de invariante», e `docs/dmpf/contexto-erros-seguranca.md` §10.3 testou cada decisão candidata contra esse gatilho — nenhuma o satisfaz. |
| `§8 ADR-013` | Taxonomia de erros e retryability | **Sem ADR próprio**, pela mesma análise de ANC-05 registrada em `docs/dmpf/contexto-erros-seguranca.md` §10.3. |
| `§8 ADR-014` | Observabilidade e convenções OpenTelemetry | ADR-026 |
| `§8 ADR-015` | BOM, compatibilidade, versionamento e depreciação | ADR-027 |

As cinco linhas sem ADR próprio não são omissão: em cada caso o artefato de
origem registrou o critério e a conclusão de não acionar, e reabri-las aqui
contrariaria a decisão que as fechou, tomada enquanto o contexto estava vivo.

### Os seis acionamentos sem linha no épico

O desequilíbrio de cardinalidade concentra-se na linha `§8 ADR-001`, que o épico
tratou como uma decisão e a RFC decompôs em oito acionamentos (`A`–`H`). Três
deles enunciam o conteúdo da própria linha e aparecem na tabela acima; os cinco
restantes decidem o que a linha pressupõe sem enunciar. Somados a `S`, são seis
decisões adicionais deliberadas, e não excedente acidental.

| Acionamento | ADR | Por que é decisão adicional, e não linha do épico |
| --- | --- | --- |
| `ADR-DMPF-B` | ADR-011 | Decide a **unidade** sobre a qual a regra de dependência incide e o seu binding por stack. A `§8 ADR-001` supõe a unidade dada; sem ela não há vértice a classificar nem aresta a resolver. |
| `ADR-DMPF-C` | ADR-012 | Decide **como** a classificação é atribuída — metadado declarado em manifesto, nunca convenção de diretório. A linha do épico enuncia a regra, não a sua fonte de verdade. |
| `ADR-DMPF-D` | ADR-013 | Decide **quem** pode alterar a classificação, com fail-closed. Sem isso, o metadado autodeclarado seria a porta dos fundos da regra de dependência. |
| `ADR-DMPF-G` | ADR-016 | Decide o critério de **domínio executável em memória**. É requisito do épico em AC-02, não linha da tabela §8, e é o que retira o caso residual que tornaria necessária a aresta que ADR-014 proíbe. |
| `ADR-DMPF-H` | ADR-017 | Decide a **identidade de bounded context** e a condição de contexto C2. A `§8 ADR-001` fala de blocos; a mesma aresta muda de veredicto conforme o contexto, e isso exige decisão própria. |
| `ADR-DMPF-S` | ADR-028 | Decide o **processo** que executa o requisito de `ADR-DMPF-D`. A RFC §13.3 encaminhou o processo ao FND-10, que o acionou em §8.1 de `docs/dmpf/governanca-bom-pilotos.md`; a tabela §8 não previu esse desdobramento. |

Fecha a conta: quinze linhas do épico, todas endereçadas — dez cobertas por ADR
e cinco com razão declarada —, mais seis acionamentos nomeados acima. Nenhum dos
dois lados fica silencioso.

## Mapa dos identificadores provisórios

Antes da promoção, cada decisão do DMPF circulou por um identificador provisório
`ADR-DMPF-<letra>`, atribuído pelo artefato que a acionou. Esses aliases
**continuam vigentes em `docs/dmpf/`**: o acervo é normativo e não é reescrito
retroativamente, então converter as letras lá apagaria o rastro entre o texto que
acionou a decisão e o arquivo que a registra. Este mapa é o que permite ler o
acervo depois da promoção e o que dá efeito à regra de RFC §13.2: o
identificador provisório não promete número final, e citar um ADR por número
definitivo antes da promoção é erro de rastreabilidade.

| Provisório | Definitivo | Artefato de origem | Seções de origem |
| --- | --- | --- | --- |
| `ADR-DMPF-A` | ADR-010 | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` (FND-02) | §4, §7 |
| `ADR-DMPF-B` | ADR-011 | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` (FND-02) | §3 |
| `ADR-DMPF-C` | ADR-012 | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` (FND-02) | §3.4, §4.4, §10.1 |
| `ADR-DMPF-D` | ADR-013 | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` (FND-02) | §10.2 |
| `ADR-DMPF-E` | ADR-014 | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` (FND-02) | §5.2 |
| `ADR-DMPF-F` | ADR-015 | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` (FND-02) | §6 |
| `ADR-DMPF-G` | ADR-016 | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` (FND-02) | §9 |
| `ADR-DMPF-H` | ADR-017 | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` (FND-02) | §5.4, §5.5, §7.2 |
| `ADR-DMPF-I` | ADR-018 | `docs/dmpf/upr-decision-mensagens.md` (FND-03) | §3.1, §3.3, §3.4 |
| `ADR-DMPF-J` | ADR-019 | `docs/dmpf/upr-decision-mensagens.md` (FND-03) | §6.1 a §6.3 |
| `ADR-DMPF-K` | ADR-020 | `docs/dmpf/uow-inbox-outbox.md` (FND-04) | §5.1, §5.5 |
| `ADR-DMPF-L` | ADR-021 | `docs/dmpf/uow-inbox-outbox.md` (FND-04) | §2.2 |
| `ADR-DMPF-M` | ADR-022 | `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05) | §3.1, §4.2, §4.3 |
| `ADR-DMPF-N` | ADR-023 | `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05) | §6.5 — promovido com pendência declarada |
| `ADR-DMPF-O` | ADR-024 | `docs/dmpf/politicas-transporte.md` (FND-06) | §8.2, §9, §10 |
| `ADR-DMPF-P` | ADR-025 | `docs/dmpf/politicas-transporte.md` (FND-06) | §8.2, §11 a §15 |
| `ADR-DMPF-Q` | ADR-026 | `docs/dmpf/resiliencia-observabilidade.md` (FND-08) | §3 a §7 |
| `ADR-DMPF-R` | ADR-027 | `docs/dmpf/governanca-bom-pilotos.md` (FND-10) | §3, §4, §5.1 |
| `ADR-DMPF-S` | ADR-028 | `docs/dmpf/governanca-bom-pilotos.md` (FND-10) | §5.2 |

`ADR-DMPF-N` é o único promovido com matéria em aberto: ADR-023 decide a parte
decidível — a autoridade de validação no repositório de contratos — e mantém a
escolha de registry de schemas em runtime como pendência declarada, com owner e
insumo nomeados no próprio ADR.
