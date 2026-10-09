#!/usr/bin/env bash
# O pin da CLI Buf (BUF-06) vive só no versions.json do pacote; o do plugin, no buf.gen.yaml de cada módulo de contrato.
set -euo pipefail
version="${DMPF_BUF_VERSION:-$(node -p 'require(process.argv[1]).buf' "$(dirname "$(realpath "$0")")/../versions.json")}"
exec go run "github.com/bufbuild/buf/cmd/buf@${version}" "$@"
