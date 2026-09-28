#!/usr/bin/env bash
# Sessão da infra local dos apps rodando no host: sobe a plataforma (dados,
# provisionamento e observabilidade), espera o provisionamento terminar e, ao
# receber o sinal de parada dos serve-*, derruba os containers. Volumes ficam;
# `pnpm nx run bff:infra-down` apaga tudo.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
COMPOSE=(docker compose -f "$ROOT/infra/local/docker-compose.yml"
  --profile postgres --profile redpanda --profile floci --profile provisioning
  --profile observability --profile exporters)

up() {
  "${COMPOSE[@]}" up -d --wait
  for init in postgres-init redpanda-init; do
    concluido "$init"
  done
}

# `compose wait` não enxerga container que já saiu; o estado final vem do inspect.
concluido() {
  local id status codigo
  for _ in $(seq 120); do
    id="$("${COMPOSE[@]}" ps -a -q "$1")"
    status="$(docker inspect -f '{{.State.Status}}' "$id")"
    if [ "$status" = "exited" ]; then
      codigo="$(docker inspect -f '{{.State.ExitCode}}' "$id")"
      [ "$codigo" = "0" ] && return 0
      "${COMPOSE[@]}" logs "$1" >&2
      echo "infra-session: $1 saiu com $codigo" >&2
      return 1
    fi
    sleep 1
  done
  echo "infra-session: $1 não terminou em 120s" >&2
  return 1
}

down() {
  trap - INT TERM EXIT
  echo "infra-session: derrubando a infra local" >&2
  "${COMPOSE[@]}" down
}

case "${1:-}" in
  up) up ;;
  session)
    trap down INT TERM EXIT
    echo "infra-session: infra ativa; Ctrl+C nos apps derruba os containers" >&2
    sleep infinity &
    wait $!
    ;;
  *) echo "uso: tools/infra-session.sh up | session" >&2; exit 2 ;;
esac
