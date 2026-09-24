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
  mkdir -p "$dir/tools" "$dir/libs/backend/go/contracts"
  cp -R "$ROOT/contracts" "$dir/contracts"
  cp "$ROOT/tools/buf.sh" "$ROOT/tools/buf-gate.sh" "$dir/tools/"
  cp "$ROOT/libs/backend/go/contracts/go.mod" "$ROOT/libs/backend/go/contracts/go.sum" "$dir/libs/backend/go/contracts/"
  cp -R "$ROOT/libs/backend/go/contracts/gen" "$dir/libs/backend/go/contracts/gen"
  git -C "$dir" init -q -b main
  git -C "$dir" config user.email "$AUTOR"
  git -C "$dir" config user.name autor
  echo "$dir"
}

# Setup quebrado não pode virar "reprovou": o sandbox aborta o autoteste inteiro.
# Premissa: GNU coreutils/sed (o runner gitea-runner é Linux); em macOS o `sed -i` difere.
commitar() { # dir mensagem
  git -C "$1" add -A || exit 2
  git -C "$1" commit -q -m "$2" || exit 2
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

# Vetor positivo: exit 0. Vetor negativo: exit != 0 E a saída traz o REPROVADO
# esperado — sem isso, uma falha de setup (mktemp, go run, git) passaria por reprovação.
esperar() { # descricao esperado(0|1) status saida [trecho-esperado]
  local descricao="$1" esperado="$2" status="$3" saida="$4" trecho="${5:-REPROVADO:}"
  casos=$((casos + 1))
  if [ "$esperado" -eq 0 ]; then
    if [ "$status" -eq 0 ]; then echo "PASS  $descricao (liberou)"; return; fi
    echo "FAIL  $descricao: esperado exit 0, obtido $status"; echo "$saida" | tail -3 | sed 's/^/      /'
    falhas=$((falhas + 1)); return
  fi
  if [ "$status" -ne 0 ] && echo "$saida" | grep -qF -- "$trecho"; then echo "PASS  $descricao (reprovou: $trecho)"; return; fi
  echo "FAIL  $descricao: esperado exit != 0 com '$trecho', obtido exit $status"; echo "$saida" | tail -3 | sed 's/^/      /'
  falhas=$((falhas + 1))
}

# Executa e passa (status, saida) para esperar sem perder nenhum dos dois.
verificar() { # descricao esperado dir subcomando [NX_BASE] [trecho-esperado]
  local descricao="$1" esperado="$2" dir="$3" sub="$4" saida status
  if [ $# -ge 5 ]; then saida="$(gate "$dir" "$sub" "$5" 2>&1)"; else saida="$(gate "$dir" "$sub" 2>&1)"; fi
  status=$?
  esperar "$descricao" "$esperado" "$status" "$saida" "${6:-REPROVADO:}"
}

echo "== vetores positivos =="
S="$(novo_sandbox)"; commitar "$S" "contratos"
verificar "lint na arvore limpa" 0 "$S" lint
verificar "pins na configuracao versionada" 0 "$S" pins
verificar "generate-check sem drift" 0 "$S" generate-check

echo "== breaking: bootstrap e estado estabelecido =="
S="$(novo_sandbox)"; mv "$S/contracts" "$S/.contracts-futuro"; commitar "$S" "base sem contratos"
mv "$S/.contracts-futuro" "$S/contracts"; commitar "$S" "primeiro conteudo do modulo"
saida="$(gate "$S" breaking HEAD~1 2>&1)"; status=$?
esperar "bootstrap legitimo (modulo novo, sem marca) libera" 0 $status "$saida"
casos=$((casos + 1))
if echo "$saida" | grep -q "sem baseline"; then echo "PASS  bootstrap avisa 'sem baseline'"; else echo "FAIL  bootstrap sem o aviso 'sem baseline'"; falhas=$((falhas + 1)); fi

S="$(novo_sandbox)"; commitar "$S" "contratos"; marcar_baseline "$S" "$REVISOR"
verificar "estado estabelecido sem mudanca incompativel" 0 "$S" breaking HEAD
verificar "NX_BASE vazio com marca presente" 1 "$S" breaking "" "baseline nao declarado"
verificar "NX_BASE ausente com marca presente" 1 "$S" breaking
verificar "baseline irresolvivel" 1 "$S" breaking refs/heads/nao-existe "baseline irresolvivel"

echo "== breaking: vetores negativos de BUF-08 =="
S="$(novo_sandbox)"; commitar "$S" "contratos"
echo "// comentario" >> "$S/contracts/proto/company/orders/event/v1/order_placed.proto"; commitar "$S" "mudanca"
verificar "marca ausente em modulo com historico" 1 "$S" breaking HEAD~1 "sem marca"

S="$(novo_sandbox)"; commitar "$S" "contratos"; marcar_baseline "$S" "$AUTOR"
verificar "tagger igual ao autor do modulo" 1 "$S" breaking HEAD "autoria e autorizacao coincidem"

# Renomear o módulo (diretório + path no buf.yaml) faria o novo nome nascer 'sem
# baseline'; os pacotes publicados na base é que denunciam a falsificação.
S="$(novo_sandbox)"; commitar "$S" "contratos"
git -C "$S" mv contracts/proto contracts/proto2
sed -i 's/path: proto$/path: proto2/' "$S/contracts/buf.yaml"; commitar "$S" "renomeia modulo"
verificar "modulo renomeado para renascer sem baseline" 1 "$S" breaking HEAD~1 "diretorio de pacote publicado"

S="$(novo_sandbox)"; commitar "$S" "contratos"; marcar_baseline "$S" "$REVISOR"
sed -i 's/int64 total_cents/int32 total_cents/' "$S/contracts/proto/company/orders/event/v1/order_placed.proto"; commitar "$S" "tipo"
verificar "breaking FILE: int64 -> int32" 1 "$S" breaking HEAD~1 "buf breaking (FILE) reprovou"

S="$(novo_sandbox)"; commitar "$S" "contratos"
sed -i 's/^  - path: proto$//' "$S/contracts/buf.yaml"
verificar "buf.yaml sem modulos declarados" 1 "$S" breaking HEAD "sem modulos declarados"

S="$(novo_sandbox)"; commitar "$S" "contratos"
mv "$S/contracts/buf.yaml" "$S/contracts/buf.yaml.fora"
verificar "buf.yaml ausente" 1 "$S" breaking HEAD "ausente"

echo "== lint =="
S="$(novo_sandbox)"
printf 'syntax = "proto3";\n\npackage company.orders.event.v1;\n\nenum Foo {\n  A = 0;\n}\n' > "$S/contracts/proto/company/orders/event/v1/foo.proto"
verificar "enum sem sufixo _UNSPECIFIED" 1 "$S" lint "buf lint (STANDARD) reprovou"

S="$(novo_sandbox)"
echo "exactly-once delivery" > "$S/contracts/NOTAS.md"
verificar "exactly-once em artefato de contrato" 1 "$S" lint "P0-3"

echo "== generate-check =="
S="$(novo_sandbox)"; commitar "$S" "contratos"
printf '\n// drift\n' >> "$S/libs/backend/go/contracts/gen/go/company/orders/event/v1/order_placed.pb.go"
verificar "byte alterado no gerado (drift)" 1 "$S" generate-check "drift"

echo "== pins =="
S="$(novo_sandbox)"
sed -i 's/@v[0-9.]*/@latest/' "$S/tools/buf.sh"
verificar "@latest na CLI" 1 "$S" pins "BUF-06"

S="$(novo_sandbox)"
sed -i -E 's/(google\.golang\.org\/protobuf v[0-9]+\.[0-9]+\.)[0-9]+/\10/' "$S/libs/backend/go/contracts/go.mod"
verificar "plugin e runtime protobuf divergentes" 1 "$S" pins "difere de google.golang.org/protobuf"

S="$(novo_sandbox)"
sed -i 's/^deps: \[\]/deps:\n  - buf.build\/exemplo\/dep/' "$S/contracts/buf.yaml"
verificar "deps declaradas sem buf.lock" 1 "$S" pins "sem buf.lock"

echo
echo "casos: $casos, falhas: $falhas"
[ "$falhas" -eq 0 ]
