#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SRC="$ROOT/apps/backend/load/src"
COMPOSE_FILE="$ROOT/infra/local/docker-compose.yml"
FORWARDED=(RATE DURATION TENANTS WEIGHTS CANCEL_RATIO RESERVE_RATIO CONVERGENCE_TIMEOUT SEED BASE_URL ADMIN_URL)

usage() {
  echo "uso: run.sh [--check] <perfil>" >&2
  exit 2
}

validate() {
  PROFILE="$1" TESTID="${2:-}" node --disable-warning=MODULE_TYPELESS_PACKAGE_JSON --input-type=module -e '
    try {
      const { parseParams } = await import(`${process.argv[1]}/params.js`);
      const { buildOptions } = await import(`${process.argv[1]}/profiles.js`);
      buildOptions(parseParams(process.env));
    } catch (error) {
      console.error(`run.sh: ${error.message}`);
      process.exit(2);
    }
  ' "$SRC"
}

check=false
if [ "${1:-}" = "--check" ]; then
  check=true
  shift
fi
[ "$#" -eq 1 ] || usage
profile="$1"

if "$check"; then
  validate "$profile"
  echo "run.sh: parâmetros válidos para o perfil $profile"
  exit 0
fi

testid="$profile-$(date -u +%Y%m%dT%H%M%S)"
validate "$profile" "$testid"

mkdir -p "$ROOT/dist/load"
if ! mkdir "$ROOT/dist/load/$testid" 2> /dev/null; then
  echo "run.sh: já existe uma execução com TESTID=$testid; rode de novo em 1 s" >&2
  exit 2
fi
envs=(-e "PROFILE=$profile" -e "TESTID=$testid" -e "K6_PROMETHEUS_RW_LABELS=profile=$profile")
for name in "${FORWARDED[@]}"; do
  if [ -n "${!name:-}" ]; then
    envs+=(-e "$name=${!name}")
  fi
done

echo "run.sh: TESTID=$testid"
echo "run.sh: dashboard em http://localhost:3000/d/load-bff?var-testid=$testid"

set +e
docker compose -f "$COMPOSE_FILE" --profile load run --rm --user "$(id -u):$(id -g)" "${envs[@]}" \
  k6 run --tag "testid=$testid" /load/src/main.js
code=$?
set -e

echo "run.sh: k6 saiu com $code; resultados em dist/load/$testid"
exit "$code"
