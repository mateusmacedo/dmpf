#!/usr/bin/env bash
# Confere os tetos do Compose de infra/local (profiles all e dmpf): reprova serviço
# sem teto de CPU ou de memória e teto de CPU acima dos núcleos do host. A CPU não é
# somada, porque cada teto é um disjuntor e cotas acima do host só geram disputa; a
# memória é, contra INFRA_BUDGET_FRACTION (0,97), porque o teto mata o processo.
# WHY: infra/README.md, seção "Orçamento de recursos".
set -euo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE="${RAIZ}/infra/local/docker-compose.yml"
FRACAO="${INFRA_BUDGET_FRACTION:-0.97}"
PERFIS=(--profile all --profile dmpf)

cpu_host="$(nproc)"
mem_host_bytes="$(awk '/^MemTotal:/ {print $2 * 1024}' /proc/meminfo)"

# `docker compose config` resolve include, anchors e variáveis: conferir o
# resultado é conferir o que vai rodar de fato, não o que está escrito. O JSON vai
# por arquivo porque o stdin do python já carrega o próprio script.
config="$(mktemp)"
trap 'trash "${config}" 2>/dev/null || true' EXIT
docker compose -f "${COMPOSE}" "${PERFIS[@]}" config --format json > "${config}"

python3 - "$cpu_host" "$mem_host_bytes" "$FRACAO" "${config}" <<'PY'
import json, sys

cpu_host, mem_host, fracao = float(sys.argv[1]), float(sys.argv[2]), float(sys.argv[3])
with open(sys.argv[4], encoding="utf-8") as arquivo:
    servicos = json.load(arquivo).get("services", {})

def bytes_de(valor):
    if isinstance(valor, (int, float)):
        return float(valor)
    texto = str(valor).strip()
    sufixos = {"b": 1, "k": 1024, "m": 1024**2, "g": 1024**3, "kb": 1024, "mb": 1024**2, "gb": 1024**3}
    for sufixo in sorted(sufixos, key=len, reverse=True):
        if texto.lower().endswith(sufixo):
            return float(texto[: -len(sufixo)]) * sufixos[sufixo]
    return float(texto)

linhas, sem_teto, acima_do_host, cpu_total, mem_total = [], [], [], 0.0, 0.0
for nome, servico in sorted(servicos.items()):
    limites = servico.get("deploy", {}).get("resources", {}).get("limits", {})
    if not limites.get("cpus") or not limites.get("memory"):
        sem_teto.append(nome)
        continue
    cpu, mem = float(limites["cpus"]), bytes_de(limites["memory"])
    if cpu > cpu_host:
        acima_do_host.append(f"{nome} ({cpu:.2f})")
    cpu_total += cpu
    mem_total += mem
    linhas.append(f"  {nome:<26} {cpu:>6.2f} vCPU   {mem / 1024**2:>7.0f} MiB")

mem_teto = mem_host * fracao
print(f"host: {cpu_host:.0f} vCPU, {mem_host / 1024**3:.2f} GiB")
print(f"teto de memória ({fracao:.0%}): {mem_teto / 1024**3:.2f} GiB\n")
print("\n".join(linhas))
print(f"\n  {'memória somada':<26} {mem_total / 1024**2:>19.0f} MiB ({mem_total / mem_host:.1%} do host)")
print(f"  {'CPU (informativo)':<26} {cpu_total:>6.2f} vCPU de tetos somados ({cpu_total / cpu_host:.2f}x o host)")

falhas = []
if sem_teto:
    falhas.append(f"serviços sem teto de CPU ou de memória: {', '.join(sem_teto)}")
if acima_do_host:
    falhas.append(f"teto de CPU acima dos {cpu_host:.0f} vCPU do host: {', '.join(acima_do_host)}")
if mem_total > mem_teto:
    falhas.append(f"memória: {mem_total / 1024**3:.2f} > {mem_teto / 1024**3:.2f} GiB")

if falhas:
    print("\nREPROVADO:", file=sys.stderr)
    for falha in falhas:
        print(f"  - {falha}", file=sys.stderr)
    sys.exit(1)
print("\nOK: todo serviço tem teto, nenhum teto de CPU passa do host e a memória cabe na fração do host.")
PY
