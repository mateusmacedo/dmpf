#!/usr/bin/env bash
# Roda um comando de teste com os valores do .env.example da raiz para as
# variáveis que o ambiente ainda não declara: localmente, os testes apontam
# para a infra de testes (tools/test-infra.sh); no CI, prevalece o env do job.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
while IFS='=' read -r key value; do
  [[ "$key" =~ ^[A-Z_][A-Z0-9_]*$ ]] || continue
  [ -n "${!key:-}" ] || export "$key=$value"
done < "$root/.env.example"
exec "$@"
