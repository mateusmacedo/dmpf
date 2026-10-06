# ADR-055: Medir a topologia sob carga só pela borda do BFF, com k6 num perfil Compose próprio e métricas no Prometheus local

## Status

Aceito — 2026-09-30. Implementa [SPEC-XJWRJPVZ](../specs/SPEC-XJWRJPVZ-suite-carga-bff-k6.md).

## Contexto

A topologia de referência tinha testes de correção (unitários, integração e o e2e do BFF), mas nenhuma medição de capacidade. Não se sabia a partir de que taxa a borda recusa ou falha, nem quanto a convergência assíncrona de `reservations` atrasa sob pressão. Três restrições do ambiente pesaram:

1. O `bff:infra-budget` soma os perfis `all` e `dmpf` e já estava no teto de CPU do host.
2. A borda autentica por um mock de identidade que forja qualquer sujeito, aceitável só na topologia local.
3. A admissão guarda um bucket por par (rota, tenant) com teto de 64 chaves; acima dele, o bucket evictado renasce com burst cheio.

## Decisão

**A carga entra só pelo BFF, gerada por k6 num serviço Compose do perfil `load`, com métricas por remote write para o Prometheus da topologia e um dashboard que as correlaciona com as séries do kernel; o mesmo script atende a máquina local e um workflow manual.**

1. **Projeto `load`.** `apps/backend/load` é projeto Nx `type:e2e`, `stack:universal`, sem `package.json`, fora de release group e sem `test`, `build`, `typecheck` ou `e2e`, para o `nx affected` do CI nunca disparar carga. Os sete perfis são targets que chamam `scripts/run.sh <perfil>`.
2. **Gerador no perfil `load`.** O serviço `k6` (`grafana/k6:2.3.0`, fixado por digest, teto de 1 vCPU e 1 GiB) fica fora de `all` e `dmpf` e, por isso, fora do `infra-budget`, e alcança o BFF pela rede do projeto `lidercap-local`.
3. **Alvo restrito.** O script recusa `BASE_URL` fora de `http://dmpf-bff`, `localhost` e `127.0.0.1`, no `--check` e no `init` do k6; o workflow sobe a topologia no próprio runner e nunca mira `dev` nem `hmg`.
4. **Pool de até 5 tenants.** 11 rotas × 5 tenants usam 55 das 64 chaves de admissão.
5. **Native histograms.** O Prometheus 3.14 ingere sem flag, e `histogram_quantile` agrega por jornada e rota no dashboard. O número de aceite vem do `summary.json`, calculado sobre a execução inteira.
6. **Classificação.** `http_req_failed` conta só 5xx e erro de transporte; o 429 vai para `admission_rejections`, e o 429 que o contexto devolve sem `Retry-After` também para `context_admission_rejections`.
7. **Dashboard próprio.** `load-bff` filtra por `testid` e junta k6, span-metrics da borda, MET-08/09/10/12 do BFF e dos contextos, resiliência, cAdvisor, Postgres e lag do Redpanda. Os painéis do kernel usam `[$__rate_interval]` com intervalo mínimo de 2 min no painel, o que dá janela de 2 min 15 s (o Grafana soma o `timeInterval` de 15 s do datasource), porque as métricas chegam por OTLP a cada 60 s.
8. **Dois gatilhos, um script.** O workflow `load.yml` (`workflow_dispatch`) valida os inputs pelo `run.sh --check` e chama o mesmo target Nx; artifact e resumo sobrevivem à saída 99 do k6.

## Desvios da spec

A spec foi ajustada durante a execução, com aprovação, a partir de medições:

- Cancelar reserva já confirmada é recusado pelo domínio (DEC-10); a jornada `reservations` ganhou a variante de cancelamento antecipado.
- A recusa do contexto chega sem `Retry-After`; o 429 foi separado pela origem.
- A cauda das iterações chegou a ~3 s no `average`; a pré-alocação de VUs passou de ×2 para ×4, com teto de 1200 VUs (881 MiB medidos no `breakpoint`).
- Com o consumer parado, a Trend de convergência não sobe; entrou o threshold `reservation_convergence_timeouts count==0`.
- O push do remote write passou a 2 s para o dashboard ficar dentro de 10 s do terminal; toda requisição tem timeout de 10 s.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| InfluxDB ou Grafana Cloud k6 | Componente novo no orçamento de recursos ou dados fora do ambiente local |
| Trend stats e os dashboards 19665/18030 vendorados | Percentis não agregáveis entre séries e cerca de 7 vezes mais séries; o 19665 exige trend stats |
| Carga direta em gRPC, Kafka ou Postgres | Contorna admissão e prazo de rota, que são o que a suíte quer medir |
| Workflow com k6 inline ou disparado por PR | Duplicaria parâmetros e montagem, e carga não pertence ao caminho do merge |

## Consequências

**Positivas:**

- Cada release pode ser comparada pelos mesmos sete perfis, com resultado em `summary.md` e no dashboard.
- As rodadas expuseram limites reais: a borda rompe por volta de 81 iterações/s pelo bulkhead do BFF, que devolve 500, e o consumer de `reservations` acumula lag depois do pico.

**Negativas:**

- **Custo aceito:** gerador e alvo dividem o host; os números comparam versões, não dimensionam produção.
- **Custo aceito:** o workflow só dispara a partir de `master` e sobre runner compartilhado, a menos que um runner dedicado seja informado.
- **Custo aceito:** a saída `experimental-prometheus-rw` e a flag `native-histograms` são experimentais no k6; a imagem fixada contém a mudança de comportamento, e uma atualização exige nova rodada de aceite.
