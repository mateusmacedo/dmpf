#!/usr/bin/env bash
# Roda um comando de teste com os valores do .env.example da raiz para as
# variáveis que o ambiente ainda não declara: localmente, os testes apontam
# para a infra de testes (test-infra.sh); no CI, prevalece o env do job.
set -euo pipefail
root="${DMPF_WORKSPACE_ROOT:-$(git rev-parse --show-toplevel)}"
while IFS='=' read -r key value; do
  [[ "$key" =~ ^[A-Z_][A-Z0-9_]*$ ]] || continue
  # Variáveis que mudam o que o shell, o linker ou as ferramentas executam não
  # vêm de um arquivo de exemplo que um PR edita como se fosse só valor.
  case "$key" in
    PATH | IFS | ENV | BASH_ENV | BASHOPTS | SHELLOPTS | PS4 | PROMPT_COMMAND | LD_* | DYLD_* | \
      NODE_OPTIONS | NPM_CONFIG_* | CGO_* | GOFLAGS | GOPROXY | GOTOOLCHAIN | GOPATH | GOROOT | \
      GOWORK | GOENV | GONOSUMDB | GONOSUMCHECK | GOPRIVATE | GOINSECURE | GOMODCACHE | GOCACHE)
      echo "test-env: ignorada a chave $key do .env.example" >&2
      continue
      ;;
  esac
  [ -n "${!key+x}" ] || export "$key=$value"
done < "$root/.env.example"
exec "$@"
