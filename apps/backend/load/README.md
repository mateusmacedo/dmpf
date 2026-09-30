# load

Suíte de teste de carga em k6 que exercita a topologia de referência só pela borda pública, o `bff`, sobre jornadas de `orders`, `reservations` e `bookings`. Mede latência, erro técnico e ponto de ruptura antes de cada release. A especificação está em `docs/specs/SPEC-XJWRJPVZ-suite-carga-bff-k6.md`.

O projeto não tem `test`, `build`, `typecheck` nem `e2e`: a carga só roda por target explícito, nunca pelo `nx affected` do CI. O único target que o CI alcança é o `lint`.

## Pré-requisitos

- Docker com Compose v2. O k6 roda no container `grafana/k6:2.3.0`, fixado por digest em `infra/local/compose/k6.yml`; nada é instalado no host.
- A topologia no ar:

  ```bash
  docker compose -f infra/local/docker-compose.yml --profile dmpf up -d --build --wait
  ```

O serviço `k6` fica no perfil Compose `load`, fora de `all` e `dmpf`, e por isso não entra na soma do `bff:infra-budget`.

## Rodar

```bash
pnpm nx run load:smoke
RATE=20 DURATION=5m pnpm nx run load:average
bash apps/backend/load/scripts/run.sh --check breakpoint
```

Cada target chama `scripts/run.sh <perfil>`, que valida os parâmetros, gera o `TESTID` (`<perfil>-<UTC yyyymmddThhmmss>`), roda o k6 no container e grava o resultado em `dist/load/<TESTID>/`. Com `--check`, só valida os parâmetros e sai, sem Docker.

O alvo é restrito a `http://dmpf-bff`, `http://localhost` e `http://127.0.0.1`: o mock de identidade forja qualquer sujeito e nunca pode apontar para `dev` ou `hmg`. Pelo `run.sh`, o k6 roda dentro do container, na rede do Compose, e só `http://dmpf-bff:8080` alcança o BFF; `localhost` e `127.0.0.1` servem apenas para um k6 rodando no host.

## Perfis

`RATE` é a taxa total de iterações de jornada por segundo; cada iteração faz de 2 a 6 requisições, mais o polling da convergência.

| Perfil | Forma | Duração default | Pergunta |
| --- | --- | --- | --- |
| `smoke` | 1 VU por jornada | 1 min | o script e a topologia funcionam? |
| `average` | rampa até `RATE` em 2 min, platô de 10 min, descida em 1 min | 13 min | latência e erro na carga esperada |
| `stress` | 2×`RATE` por 10 min e 3×`RATE` por 5 min, com rampas | 20 min | quanto degrada acima do esperado |
| `spike` | `RATE`, salto a 8×`RATE` em 30 s por 1 min, volta a `RATE` | 5 min | a borda volta ao normal depois do pico? |
| `soak` | `RATE` constante | 60 min | degradação ou vazamento ao longo do tempo |
| `breakpoint` | de 0 a 30×`RATE` em 30 min, 5 tenants, sem descida | até o abort | onde a topologia rompe |
| `admission` | 80 req/s numa rota, 1 tenant | 2 min | a admissão recusa acima de 50/s? |

Os thresholds de cada perfil estão em `src/profiles.js`. Os de `average` e `soak` são metas iniciais: a primeira execução de `average` numa máquina de referência vira a baseline.

## Parâmetros

| Variável | Default | Regra |
| --- | --- | --- |
| `RATE` | `10` | inteiro de 1 a 1000 |
| `DURATION` | do perfil | platô de `average`, `stress`, `soak` e `admission`, como `30s`, `10m` ou `1h` |
| `TENANTS` | `4` | de 1 a 5; `breakpoint` usa 5 e `admission` usa 1 |
| `WEIGHTS` | `orders=30,reservations=25,bookings=35,reads=10` | as quatro jornadas, somando 100 |
| `CANCEL_RATIO` | `0.2` | de 0 a 1; somado a `RESERVE_RATIO`, no máximo 1 |
| `RESERVE_RATIO` | `0.2` | de 0 a 1 |
| `CONVERGENCE_TIMEOUT` | `10s` | espera da reserva criada pelo consumer |
| `SEED` | `1` | semente dos valores sorteados |
| `BASE_URL` | `http://dmpf-bff:8080` | allowlist acima |

Cada perfil pode pré-alocar no máximo 1200 VUs, o que cabe no 1 GiB do container k6 (medido: 881 MiB no `breakpoint` com `RATE=10`). Acima disso, o `--check` recusa e nomeia `RATE`; no `breakpoint`, isso limita `RATE` a 10.

O pool de tenants para em 5 porque a admissão do BFF guarda um bucket por par (rota, tenant) com teto de 64 chaves: 11 rotas × 5 tenants usam 55. Acima disso, o bucket evictado renasce com burst cheio e a recusa deixa de refletir o limite.

## Jornadas

- `orders`: 1 a 3 itens, `place` e leitura do pedido colocado.
- `reservations`: três variantes sorteadas numa faixa única. Na fração `CANCEL_RATIO`, cancela antes da decisão e confere que a reserva segue `canceled` depois do pedido. Na fração `RESERVE_RATIO`, reserva direto. O restante espera a reserva criada pelo consumer a partir do `OrderPlaced`. Cancelar reserva já confirmada é recusado pelo domínio (`apps/backend/reservations/domain/cancel.go`).
- `bookings`: recurso novo, reserva, leitura, listagem com um elemento e cancelamento na fração `CANCEL_RATIO`.
- `reads`: leituras sobre os 10 pedidos por tenant que o `setup()` cria.

## Workflow

`.github/workflows/load.yml` roda o mesmo target num runner, com os inputs `profile`, `rate`, `duration`, `tenants`, `weights` e `runner`. Ele sobe a topologia no próprio runner, publica `dist/load/` como artifact por 14 dias e anexa o `summary.md` ao resumo do job. O job tem teto de 6 h, o limite dos runners hospedados. Dispara só por `workflow_dispatch`, e o GitHub só oferece o disparo quando o arquivo existe na branch default (`master`). Para comparar execuções, use um runner dedicado no input `runner`.

## Resultado

- `dist/load/<TESTID>/summary.md`: thresholds, requisições por status, p95 e p99 por tipo (`read`, `write`, `poll`) e, no `breakpoint`, a taxa-alvo no fim.
- `dist/load/<TESTID>/summary.json`: o objeto completo do `handleSummary`.
- Código de saída do k6, reportado pelo `run.sh`: 0 quando todos os thresholds passam, 99 quando algum falha, 107 em erro de script, 108 em abort do `setup()`. O `pnpm nx run` normaliza qualquer falha para 1. Parâmetro inválido sai com 2 antes do k6.

Métricas próprias da suíte:

| Métrica | O que conta |
| --- | --- |
| `admission_rejections` | fração de respostas 429, da borda e do contexto |
| `context_admission_rejections` | 429 vindos do contexto, que a borda devolve sem `Retry-After` |
| `reservation_convergence` | tempo do fim do `place` à primeira leitura 200 da reserva |
| `reservation_convergence_timeouts` | esperas que passaram de `CONVERGENCE_TIMEOUT` |

`http_req_failed` conta só 5xx e erro de transporte, inclusive a requisição que passa de 10 s sem resposta. 404 durante o polling e 429 ficam fora dele.

## Dashboard

As métricas vão por Prometheus remote write, com native histograms, para o Prometheus da topologia. O dashboard `load-bff` (`http://localhost:3000/d/load-bff`, arquivo `infra/observability/grafana/dashboards/load-bff.json`) filtra pela variável `testid` e tem cinco linhas:

1. k6: VUs, taxa por jornada e rota, p95 e p99 por rota, falha técnica, recusas e convergência.
2. Borda do BFF: taxa e p95 dos spans `HTTP <método> <rota>` (span-metrics do Tempo), a curva do k6 contra a da borda e a razão k6 ÷ borda por rota, que fica em 1 quando a borda vê tudo o que o k6 envia.
3. BFF para os contextos: MET-08, MET-09 e MET-10 por operação, retries, bulkhead, prazos e breaker.
4. Contextos: MET-08, MET-09, MET-10 e MET-12 de `orders`, `reservations` e `bookings`.
5. Recursos: CPU e memória por container (cAdvisor), conexões do Postgres por banco e lag do grupo `reservations`.

Os painéis do kernel usam janela fixa de 2 min porque as métricas chegam por OTLP a cada 60 s. Os painéis de erro, recusa e resiliência mostram 0 quando não há evento. O lag depende de `enable_consumer_group_metrics` no Redpanda; se o painel ficar vazio, ligue com `rpk cluster config set enable_consumer_group_metrics '["group", "partition"]'`.

Para a visão genérica do k6, importe no Grafana o dashboard 18030 ("k6 Prometheus (Native Histograms)") pelo ID. O 19665 não serve aqui, porque exige trend stats em vez de native histograms.

Depois do fim de uma execução, as séries do k6 recebem stale markers: consultas instantâneas voltam vazias, e é preciso usar intervalo ou `last_over_time`.

## Limitações

- Gerador e alvo dividem o host. O container `k6` tem teto de 1 vCPU e aparece no painel de CPU; `dropped_iterations` acima de 0 indica gerador ou pool de VUs saturado, não o alvo.
- Cada serviço da topologia tem teto de 0,15 vCPU. No `smoke`, cada VU dispara sem pausa, satura a cota e o throttling do CFS aparece como latência de dezenas de milissegundos; o `smoke` prova funcionamento, não latência.
- A topologia roda com `LOG_LEVEL=debug` e 100% de amostragem de traces, o que encarece cada requisição; a suíte mede a topologia como ela está configurada.
- Depois de um `breakpoint` ou `spike`, o consumer de `reservations` leva minutos para drenar o lag de `orders.events`. Espere o painel de lag zerar antes da próxima rodada; senão o `setup()` sai com 107, porque a reserva dos pedidos de massa não converge.
- Os números servem para comparar versões na mesma máquina, não para dimensionar produção.
