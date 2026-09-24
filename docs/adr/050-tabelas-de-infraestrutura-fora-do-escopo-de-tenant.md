# ADR-050: As tabelas de infraestrutura ficam fora do escopo de tenant

## Status

Aceito — 2026-09-23. Implementa [SPEC-9B6SHEH8](../specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md).

## Contexto

A spec realiza `IDN-11` a `IDN-14` de FND-07 (`docs/dmpf/contexto-erros-seguranca.md` §4): toda leitura e toda escrita de **dado de negócio** é escopada ao tenant do contexto, e a imposição não depende de convenção. As quatro tabelas de agregado (`dmpf_example_orders`, `dmpf_example_reservations`, `bookings_booking` e `bookings_resource`) ganharam `tenant_id NOT NULL` e chave primária composta, e o `postgres.Table[ID,S]` escreve o predicado em todo statement.

Restavam as três tabelas que o kernel mantém para a entrega de mensagens: `dmpf_outbox`, `dmpf_inbox` e `dmpf_quarantine`. Nenhum documento do repositório afirmava se elas entram no escopo — foi essa ausência que produziu a pergunta durante o planejamento.

As três têm uma propriedade em comum que as tabelas de agregado não têm: o código que as lê opera **antes** de haver tenant, **por cima** de todos os tenants, ou sobre mensagem cujo tenant **não pode ser conhecido**.

- **Outbox**: o relay reivindica registros por lease (`postgres/claim.go`) sem contexto de requisição algum, e a drenagem é por natureza de todos os tenants. O `message_id` é único globalmente (`dmpf_outbox_message_id_unique`).
- **Inbox**: a chave é `(consumer_name, message_id)` (`dmpf_inbox_key`), e o identificador da mensagem é único no produtor. A deduplicação decide sobre a identidade da mensagem, não sobre o dono do dado.
- **Quarantine**: guarda o envelope bruto de mensagem contida, inclusive a que foi contida **porque** o envelope é ilegível ou veio de fora da fronteira (`invalid-envelope`, `untrusted-boundary`). Para essas, não existe tenant confiável a gravar.

## Decisão

**`dmpf_outbox`, `dmpf_inbox` e `dmpf_quarantine` são tabelas de plataforma e não recebem coluna nem predicado de tenant.** `IDN-11` alcança dado de negócio; o que essas tabelas guardam é estado da entrega, e o acesso a elas é feito só pelo kernel — nenhum contexto escreve SQL sobre elas, porque o gate `context-provider` nega o driver nos providers de contexto.

O tenant **atravessa** a outbox sem ser escopo dela: vai no `metadata` do registro, resolvido pelo provider a partir do portador (`enqueueTenant`, `postgres/outbox.go`), e o relay o põe no atributo `tenantid` do envelope. Quem consome reconstrói o contexto com esse tenant (`CTX-24`), e a escrita de negócio do consumidor é escopada como qualquer outra.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Coluna `tenant_id` nas três tabelas, com predicado em toda consulta | O relay e a purga operam sem contexto, e teriam de iterar tenants ou abrir exceção ao predicado — o que transformaria o escopo em convenção justamente onde ele não agrega isolamento. A quarantine não teria valor confiável a gravar para envelope ilegível ou de fora da fronteira |
| Coluna `tenant_id` só na outbox, sem predicado, como dado | Duplica o que o `metadata` já carrega e cria duas fontes para o mesmo fato; a decisão desta spec foi não abrir segunda fonte para nenhum campo do contexto |
| Uma outbox, inbox e quarantine por tenant | Multiplica tabelas e relays pelo número de tenants sem que nenhuma regra normativa o peça |

## Consequências

**Positivas:**

- O relay, a purga e a deduplicação continuam simples e sem exceção ao predicado de tenant.
- O tenant chega ao consumidor por um caminho só, verificado de ponta a ponta pelo e2e do BFF.

**Negativas:**

- Quem inspeciona essas tabelas vê registros de todos os tenants. Isso é acesso ao dado em repouso fora da aplicação, governado por `DAT-11` e não por `IDN-11` a `IDN-14`, e fica com a operação.
- Uma consulta nova sobre essas tabelas não é pega pelo `Table`: a proteção é o gate que nega o driver nos providers de contexto, não um predicado.
