#!/usr/bin/env bash
# Sobe e derruba a infra dos testes de integração (infra/test/compose.yml).
# Separada da infra de runtime local: outro projeto Compose, outras portas e
# só tmpfs, então o down descarta tudo o que os testes criaram.
set -euo pipefail
cd "${DMPF_WORKSPACE_ROOT:-$(git rev-parse --show-toplevel)}"
compose=(docker compose -f infra/test/compose.yml)
keycloak=("${compose[@]}" --profile keycloak)

if [ -n "${CI:-}" ]; then
  echo "test-infra: no CI a infra de testes é a do job (ci.yml)" >&2
  exit 0
fi

case "${1:-}" in
  up)
    "${compose[@]}" up -d --wait postgres redpanda redpanda-sasl floci redis
    "${compose[@]}" run --rm redpanda-sasl-init
    ;;
  keycloak)
    keycloak_before="$("${keycloak[@]}" ps -q keycloak)"
    "${keycloak[@]}" up -d --wait keycloak
    if [ "$("${keycloak[@]}" ps -q keycloak)" != "$keycloak_before" ]; then
      if ! "${keycloak[@]}" run --rm keycloak-init; then
        "${keycloak[@]}" rm -sf keycloak
        exit 1
      fi
    fi
    ;;
  down)
    "${compose[@]}" --profile init --profile keycloak down --remove-orphans
    ;;
  *)
    echo "uso: $0 up|keycloak|down" >&2
    exit 2
    ;;
esac
