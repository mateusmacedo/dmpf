#!/usr/bin/env bash
# Autoteste de tools/buf-gate.sh: cada gate reprova o que deve reprovar e libera o
# que deve liberar, em repositórios git descartáveis. Os repositórios ficam em /tmp
# e não são apagados, como em tools/dmpf-gate-check.sh (sem utilitário de lixeira).
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositorio git" >&2; exit 2; }
cd "$ROOT" || exit 2

AUTOR=autor@exemplo.test
REVISOR=revisor@exemplo.test
falhas=0
casos=0

# Sandbox com contracts/, tools/ e a lib (go.mod + gen/) copiados do repositório real.
novo_sandbox() {
  local dir
  dir="$(mktemp -d)" || exit 2
  mkdir -p "$dir/tools" "$dir/libs/shared/go/dmpf-contracts"
  cp -R "$ROOT/contracts" "$dir/contracts"
  cp "$ROOT/tools/buf.sh" "$ROOT/tools/buf-gate.sh" "$dir/tools/"
  cp "$ROOT/libs/shared/go/dmpf-contracts/go.mod" "$ROOT/libs/shared/go/dmpf-contracts/go.sum" "$dir/libs/shared/go/dmpf-contracts/"
  cp -R "$ROOT/libs/shared/go/dmpf-contracts/gen" "$dir/libs/shared/go/dmpf-contracts/gen"
  git -C "$dir" init -q -b main
  git -C "$dir" config user.email "$AUTOR"
  git -C "$dir" config user.name autor
  echo "$dir"
}

commitar() { # dir mensagem
  git -C "$1" add -A
  git -C "$1" commit -q -m "$2"
}

marcar_baseline() { # dir email-do-tagger
  GIT_COMMITTER_EMAIL="$2" GIT_COMMITTER_NAME=tagger \
    git -C "$1" tag -a "contracts-baseline/proto" -m "baseline estabelecido"
}

# Executa o gate dentro do sandbox, sem herdar GIT_* do ambiente atual.
gate() { # dir subcomando [NX_BASE]
  local dir="$1" sub="$2" base="${3-__unset__}"
  if [ "$base" = "__unset__" ]; then
    (cd "$dir" && env -u NX_BASE -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE bash tools/buf-gate.sh "$sub")
  else
    (cd "$dir" && env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE NX_BASE="$base" bash tools/buf-gate.sh "$sub")
  fi
}

esperar() { # descricao esperado(0|1) status
  casos=$((casos + 1))
  if [ "$2" -eq 0 ] && [ "$3" -eq 0 ]; then echo "PASS  $1 (liberou)"; return; fi
  if [ "$2" -ne 0 ] && [ "$3" -ne 0 ]; then echo "PASS  $1 (reprovou)"; return; fi
  echo "FAIL  $1: esperado exit $2, obtido $3"
  falhas=$((falhas + 1))
}

echo "== vetores positivos =="
S="$(novo_sandbox)"; commitar "$S" "contratos"
gate "$S" lint >/dev/null 2>&1;            esperar "lint na arvore limpa" 0 $?
gate "$S" pins >/dev/null 2>&1;            esperar "pins na configuracao versionada" 0 $?
gate "$S" generate-check >/dev/null 2>&1;  esperar "generate-check sem drift" 0 $?

echo "== breaking: bootstrap e estado estabelecido =="
S="$(novo_sandbox)"; mv "$S/contracts" "$S/.contracts-futuro"; commitar "$S" "base sem contratos"
mv "$S/.contracts-futuro" "$S/contracts"; commitar "$S" "primeiro conteudo do modulo"
saida="$(gate "$S" breaking HEAD~1 2>&1)"; status=$?
esperar "bootstrap legitimo (modulo novo, sem marca) libera com aviso" 0 $status
echo "$saida" | grep -q "sem baseline" || { echo "FAIL  bootstrap sem o aviso 'sem baseline'"; falhas=$((falhas + 1)); }

S="$(novo_sandbox)"; commitar "$S" "contratos"; marcar_baseline "$S" "$REVISOR"
gate "$S" breaking HEAD >/dev/null 2>&1;   esperar "estado estabelecido sem mudanca compativel" 0 $?
gate "$S" breaking "" >/dev/null 2>&1;     esperar "NX_BASE vazio com marca presente" 1 $?
gate "$S" breaking >/dev/null 2>&1;        esperar "NX_BASE ausente com marca presente" 1 $?
gate "$S" breaking refs/heads/nao-existe >/dev/null 2>&1; esperar "baseline irresolvivel" 1 $?

echo "== breaking: vetores negativos de BUF-08 =="
S="$(novo_sandbox)"; commitar "$S" "contratos"
echo "// comentario" >> "$S/contracts/proto/company/orders/event/v1/order_placed.proto"; commitar "$S" "mudanca"
gate "$S" breaking HEAD~1 >/dev/null 2>&1; esperar "marca ausente em modulo com historico" 1 $?

S="$(novo_sandbox)"; commitar "$S" "contratos"; marcar_baseline "$S" "$AUTOR"
gate "$S" breaking HEAD >/dev/null 2>&1;   esperar "tagger igual ao autor do modulo" 1 $?

S="$(novo_sandbox)"; commitar "$S" "contratos"; marcar_baseline "$S" "$REVISOR"
git -C "$S" mv contracts/proto/company contracts/proto/empresa; commitar "$S" "redeclaracao"
gate "$S" breaking HEAD~1 >/dev/null 2>&1; esperar "pacote publicado movido de caminho" 1 $?

S="$(novo_sandbox)"; commitar "$S" "contratos"; marcar_baseline "$S" "$REVISOR"
sed -i 's/int64 total_cents/int32 total_cents/' "$S/contracts/proto/company/orders/event/v1/order_placed.proto"; commitar "$S" "tipo"
gate "$S" breaking HEAD~1 >/dev/null 2>&1; esperar "breaking FILE: int64 -> int32" 1 $?

echo "== lint =="
S="$(novo_sandbox)"
printf 'syntax = "proto3";\n\npackage company.orders.event.v1;\n\nenum Foo {\n  A = 0;\n}\n' > "$S/contracts/proto/company/orders/event/v1/foo.proto"
gate "$S" lint >/dev/null 2>&1;            esperar "enum sem sufixo _UNSPECIFIED" 1 $?

S="$(novo_sandbox)"
echo "exactly-once delivery" > "$S/contracts/NOTAS.md"
gate "$S" lint >/dev/null 2>&1;            esperar "exactly-once em artefato de contrato" 1 $?

echo "== generate-check =="
S="$(novo_sandbox)"; commitar "$S" "contratos"
printf '\n// drift\n' >> "$S/libs/shared/go/dmpf-contracts/gen/go/company/orders/event/v1/order_placed.pb.go"
gate "$S" generate-check >/dev/null 2>&1;  esperar "byte alterado no gerado (drift)" 1 $?

echo "== pins =="
S="$(novo_sandbox)"
sed -i 's/@v[0-9.]*/@latest/' "$S/tools/buf.sh"
gate "$S" pins >/dev/null 2>&1;            esperar "@latest na CLI" 1 $?

S="$(novo_sandbox)"
sed -i -E 's/(google\.golang\.org\/protobuf v[0-9]+\.[0-9]+\.)[0-9]+/\10/' "$S/libs/shared/go/dmpf-contracts/go.mod"
gate "$S" pins >/dev/null 2>&1;            esperar "plugin e runtime protobuf divergentes" 1 $?

S="$(novo_sandbox)"
sed -i 's/^deps: \[\]/deps:\n  - buf.build\/exemplo\/dep/' "$S/contracts/buf.yaml"
gate "$S" pins >/dev/null 2>&1;            esperar "deps declaradas sem buf.lock" 1 $?

echo
echo "casos: $casos, falhas: $falhas"
[ "$falhas" -eq 0 ]
