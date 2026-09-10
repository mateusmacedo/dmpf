#!/usr/bin/env bash
# Soma os tetos declarados no Compose de infra/local e reprova se passarem da
# fração do host reservada ao ambiente local. O teto é do conjunto, não de cada
# serviço: um contêiner sozinho pode usar mais do que a sua fatia média, mas a
# soma dos limites nunca compromete mais do que a fração declarada.
set -euo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE="${RAIZ}/infra/local/docker-compose.yml"
FRACAO="${INFRA_BUDGET_FRACTION:-0.60}"
PERFIS=(--profile all --profile dmpf)

cpu_host="$(nproc)"
mem_host_bytes="$(awk '/^MemTotal:/ {print $2 * 1024}' /proc/meminfo)"

# `docker compose config` resolve include, anchors e variáveis: somar o
# resultado é somar o que vai rodar de fato, não o que está escrito. O JSON vai
# por arquivo porque o stdin do python já carrega o próprio script.
config="$(mktemp)"
trap 'rm -f "${config}"' EXIT
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

linhas, cpu_total, mem_total, sem_teto = [], 0.0, 0.0, []
for nome, servico in sorted(servicos.items()):
    limites = servico.get("deploy", {}).get("resources", {}).get("limits", {})
    if not limites:
        sem_teto.append(nome)
        continue
    cpu = float(limites.get("cpus", 0) or 0)
    mem = bytes_de(limites.get("memory", 0) or 0)
    cpu_total += cpu
    mem_total += mem
    linhas.append(f"  {nome:<20} {cpu:>6.2f} vCPU   {mem / 1024**2:>7.0f} MiB")

cpu_teto, mem_teto = cpu_host * fracao, mem_host * fracao
print(f"host: {cpu_host:.0f} vCPU, {mem_host / 1024**3:.2f} GiB")
print(f"teto ({fracao:.0%}): {cpu_teto:.2f} vCPU, {mem_teto / 1024**3:.2f} GiB\n")
print("\n".join(linhas))
print(f"\n  {'soma':<20} {cpu_total:>6.2f} vCPU   {mem_total / 1024**2:>7.0f} MiB")
print(f"  {'uso do host':<20} {cpu_total / cpu_host:>6.1%}          {mem_total / mem_host:>7.1%}")

falhas = []
if sem_teto:
    falhas.append(f"serviços sem teto declarado: {', '.join(sem_teto)}")
if cpu_total > cpu_teto:
    falhas.append(f"CPU: {cpu_total:.2f} > {cpu_teto:.2f} vCPU")
if mem_total > mem_teto:
    falhas.append(f"memória: {mem_total / 1024**3:.2f} > {mem_teto / 1024**3:.2f} GiB")

if falhas:
    print("\nREPROVADO:", file=sys.stderr)
    for falha in falhas:
        print(f"  - {falha}", file=sys.stderr)
    sys.exit(1)
print("\nOK: a soma dos tetos cabe na fração declarada do host.")
PY
