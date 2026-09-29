#!/usr/bin/env bash
# Sobe e derruba a infra dos testes de integração (infra/test/compose.yml).
# Separada da infra de runtime local: outro projeto Compose, outras portas e
# só tmpfs, então o down descarta tudo o que os testes criaram.
set -euo pipefail
cd "$(dirname "$0")/.."
compose=(docker compose -f infra/test/compose.yml)

if [ -n "${CI:-}" ]; then
  echo "test-infra: no CI a infra de testes é a do job (ci.yml)" >&2
  exit 0
fi

case "${1:-}" in
  up)
    "${compose[@]}" up -d --wait postgres redpanda redpanda-sasl floci
    "${compose[@]}" run --rm redpanda-sasl-init
    ;;
  down)
    "${compose[@]}" --profile init down --remove-orphans
    ;;
  *)
    echo "uso: $0 up|down" >&2
    exit 2
    ;;
esac
