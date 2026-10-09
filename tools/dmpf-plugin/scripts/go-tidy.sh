#!/usr/bin/env bash
# go mod tidy de um módulo do workspace, rodado no diretório do módulo. O tidy
# ignora o go.work e buscaria a tag v0.1.0 de cada irmão, ainda não publicada;
# os replace do go.work entram no go.mod só durante o tidy e saem ao fim, e o
# go.mod publicado segue sem replace (ADR-047).
set -euo pipefail
root="${DMPF_WORKSPACE_ROOT:-$(git rev-parse --show-toplevel)}"
module="$(pwd)"

added=()
restore() {
  local flags=()
  for spec in "${added[@]}"; do flags+=("-dropreplace=$spec"); done
  [ "${#flags[@]}" -eq 0 ] || go mod edit "${flags[@]}"
}
trap restore EXIT

existing="$(go mod edit -json | jq -r '.Replace // [] | .[] | .Old.Path + "@" + (.Old.Version // "")')"
flags=()
while read -r path version dir; do
  spec="$path@$version"
  grep -qxF "$spec" <<<"$existing" && continue
  target="$(realpath --relative-to="$module" "$root/$dir")"
  [[ "$target" == ../* ]] || target="./$target"
  flags+=("-replace=$spec=$target")
  added+=("$spec")
done < <(sed -n '/^replace (/,/^)/p' "$root/go.work" | awk '$3 == "=>" {print $1, $2, $4}')
[ "${#flags[@]}" -eq 0 ] || go mod edit "${flags[@]}"

GOWORK=off go mod tidy "$@"
