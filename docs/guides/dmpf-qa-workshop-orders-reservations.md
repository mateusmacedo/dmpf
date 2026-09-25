# Workshop: orders e reservations como aceite automatizado

Guia **derivado e não normativo**. Duração sugerida: **90 minutos**.
Pré-requisito de leitura: [playbook QA](./dmpf-qa-playbook.md) §§1–3.
Norma: [FND-09](../dmpf/testes-interop.md). Código: as fixtures em `contracts/fixtures/{orders,reservations}/projection/v1/` e o molde do teste em `libs/backend/go/testkit/domainkit/fixture_test.go`.

Objetivo: o time de QA abre os arquivos **já versionados**, vê como cada caso vira oráculo, e sai capaz de escrever o próximo `.golden` no mesmo formato.

---

## Agenda

| Min | Bloco | Arquivo |
| --- | --- | --- |
| 0–10 | Mapa do exemplo | este guia, §1 |
| 10–30 | Projeção `orders` | `contracts/fixtures/orders/projection/v1/order.golden` |
| 30–45 | Ponte Dev: `domainkit` | `fixture_test.go` |
| 45–60 | Projeção `reservations` e a costura | `reservation.golden` |
| 60–75 | Wire (o que o QA descreve vs o que o gerador preenche) | `contracts/fixtures/*/event/v1/*.golden` |
| 75–90 | Exercício + comando | §6 |

Infra (Postgres/Redpanda) **não** entra neste workshop. Aqui a prova é domínio + contrato.

---

## 1. O exemplo em uma frase

`orders` publica `item-added` e `order-placed`. `reservations` consome o pedido colocado e confirma estoque com `Reserve`. A chave natural da reserva **é o identificador do pedido**.

```text
AddItem / Place  →  eventos orders.*  →  Reserve  →  reservations.reservation-confirmed
     domínio              wire              domínio (outro contexto)
```

Os dois agregados têm fixture de projeção **e** golden de evento. O teste de domínio **não** lê Kafka: lê JSON de projeção e executa a UPR em memória.

---

## 2. Orders — cinco oráculos de negócio

Abra `contracts/fixtures/orders/projection/v1/order.golden`.

Identidade: `context=orders`, `aggregate=order`. Cinco casos, duas UPRs.

| Caso | UPR | Ramo | O que o QA está afirmando |
| --- | --- | --- | --- |
| `add-item-accepted` | `add-item` | accepted | Pedido `open` com 1 item aceita o segundo; evento `orders.item-added`; `items.count` vai a `"2"` |
| `add-item-limit-exceeded` | `add-item` | rejected | `item_limit` `"1"` + segundo item → `orders/item-limit-exceeded`; estado congelado |
| `place-accepted` | `place` | accepted | Pedido `open` com item → `placed` e `orders.order-placed` |
| `place-empty-order` | `place` | rejected | Sem itens → `orders/empty-order` |
| `place-order-not-open` | `place` | rejected | Já `placed` → `orders/order-not-open` |

Pontos a marcar no arquivo (não copie o JSON; leia no disco):

1. `"item_limit": "3"` — aspas. Sem aspas o carregador recusa (`FIX-07`).
2. `"at": "1755432000"` — instante injetado no comando. A UPR **não** chama relógio.
3. No rejected, `events` é `[]` e `state_after` replica `state_before`.
4. `doc` cita a regra (FND-03 / `ORA-38`). Isso é o comentário que o JSON não tem.

Controle negativo mental: se alguém trocar `add-item-limit-exceeded` para `branch: accepted` e deixar o estado igual, o `domainkit` reprova — o Dev não “ajusta o assert”.

---

## 3. O Dev não reescreve o esperado

Abra `libs/backend/go/testkit/domainkit/fixture_test.go`. É o molde: dirige o
agregado `counter` do próprio kit pela fixture
`domainkit/testdata/counter.golden`, no mesmo formato `ORA-30` das fixtures de
`orders` e `reservations`.

`TestTheCounterMatchesTheProjectionFixture`:

- `tb.LoadProjection` no **mesmo** caminho da fixture;
- um subteste por `c.Name`;
- `domainkit.Run` na UPR real (`bump` / `reset`);
- `domainkit.Equal(got, c.Expected.Projection())`;
- `ReadTwice` no mesmo sujeito.

O switch `c.Command["upr"]` é a **única** tradução Dev: kebab da fixture → tipo Go. O restante (estado, eventos, rejeição) vem do arquivo do QA.

Para `orders` (`AddItem` / `PlaceOrder`) e `reservations` (`Reserve`), o teste
com essa forma pertence ao `domain` de cada contexto, em
`apps/backend/{orders,reservations}/domain` — os exemplos saíram do kit quando
`libs/backend/go` passou a guardar só o kernel de reuso (ADR-046), e o kit não
pode depender de um contexto para provar a si mesmo.

---

## 4. Reservations — consumidor com chave do pedido

Abra `contracts/fixtures/reservations/projection/v1/reservation.golden`.

| Caso | Ramo | Código / evento |
| --- | --- | --- |
| `reserve-accepted` | accepted | `reservations.reservation-confirmed`; `pending` → `confirmed`; `items` `"2"` |
| `reserve-nothing` | rejected | `reservations/nothing-to-reserve` (`items` `"0"`) |
| `reserve-already-reserved` | rejected | `reservations/already-reserved` (já `confirmed`) |

Campo `"order": "r-1"`: a reserva **não** inventa outro ID. QA de um contexto consumidor declara a chave natural do produtor.

A costura ponta a ponta (relay, inbox, Ack) é `appkit`/`distkit` — os harnesses do contexto, em `apps/backend/reservations` — e fica para um segundo workshop. Neste, o aceite de `Reserve` já está fechado **sem broker**.

---

## 5. Wire — três goldens de evento

| Fixture | Casos (nomes) | Por que existem |
| --- | --- | --- |
| `orders/event/v1/item-added.golden` | `all-conditionals-present`, `all-conditionals-absent`, `quantity-zero` | Envelope ± condicionais; quantidade zero no wire |
| `orders/event/v1/order-placed.golden` | os dois condicionais, `channel-unspecified`, `total-cents-beyond-double`, `time-with-nanos`, `item-count-present` | Discriminadores de precisão (`ORA-08`) |
| `reservations/event/v1/reservation-confirmed.golden` | condicionais ±, `item-count-zero` | Espelho no consumidor |

QA descreve **campos e discriminação**. Bytes (`payload_bytes_hex`, `payload_hash`) saem do gerador em `contracts`. Três oráculos: `DMPF-R001` semântica, `DMPF-R002` hash, `DMPF-R003` bytes — reportados em separado.

Proto correspondente: `contracts/proto/` (árvore em `contracts/README.md`). A golden **espelha** o caminho do contrato (`FIX-10`).

---

## 6. Exercício (15 min) e comando

**No papel ou num branch descartável**, acrescente um caso à projeção de `orders` (não precisa mergear):

> Pedido `open` com `item_limit` `"1"` e **zero** itens: `add-item` com `quantity` `"1"` deve ser `accepted` **ou** o limite conta o primeiro item? Escreva `doc`, `state_before`, `command`, `expected`. Se a spec atual não decidir, o exercício **é** achar o buraco — a fixture não inventa regra.

Depois rode o que já está verde (sem o caso novo):

```bash
pnpm nx run testkit:test-race
```

O teste `TestTheCounterMatchesTheProjectionFixture` tem de passar. Os testes que consumiam `order.golden` e `reservation.golden` saíram do kit junto com os exemplos (ADR-046); reescrevê-los no molde do `counter`, dentro do `domain` de cada contexto (`pnpm nx run orders:test-race`), é item aberto dos contextos — até lá, as duas fixtures ficam sem consumidor. Sem `PG_DSN`, suítes de infra fazem skip **local**; não trate skip como aceite de `appkit`.

Opcional, só contratos:

```bash
pnpm nx run contracts:buf-lint
```

---

## 7. Encerramento — o que cada papel leva

| Papel | Próximo passo |
| --- | --- |
| QA | Preencher o [playbook](./dmpf-qa-playbook.md) §5 para o bounded context real |
| Dev | Ligar a UPR nova com o mesmo padrão de `fixture_test.go`, no `domain` do contexto |
| Ambos | Um PR de fixture **antes ou junto** do PR de código, nunca depois do “já funciona na API” |

Fonte do esqueleto de implementação: [dmpf-implementation.md](./dmpf-implementation.md) passo 1 (spec) e §2.5 (pirâmide).
