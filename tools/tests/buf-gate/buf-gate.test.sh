#!/usr/bin/env bash
# Autoteste de tools/dmpf-plugin/scripts/buf-gate.sh: cada gate reprova o que deve reprovar e libera o
# que deve liberar, em repositórios git descartáveis. Os repositórios ficam em /tmp
# e não são apagados, como em tools/dmpf-plugin/scripts/dmpf-gate-check.sh (sem utilitário de lixeira).
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositorio git" >&2; exit 2; }
cd "$ROOT" || exit 2

AUTOR=autor@exemplo.test
falhas=0
casos=0

# Sandbox com tools/ e o módulo de contrato do kernel copiados do repositório real.
novo_sandbox() {
  local dir
  dir="$(mktemp -d)" || exit 2
  mkdir -p "$dir/tools/dmpf-plugin/scripts" "$dir/libs/backend/go/contracts"
  cp -R "$ROOT/libs/backend/go/contracts/proto" "$dir/libs/backend/go/contracts/proto"
  cp -R "$ROOT/libs/backend/go/contracts/testdata/proto/dmpf" "$dir/libs/backend/go/contracts/proto/dmpf"
  grep -v '^  - path: testdata/proto$' "$ROOT/libs/backend/go/contracts/buf.yaml" > "$dir/libs/backend/go/contracts/buf.yaml"
  grep -v '^  - directory: testdata/proto$' "$ROOT/libs/backend/go/contracts/buf.gen.yaml" > "$dir/libs/backend/go/contracts/buf.gen.yaml"
  cp "$ROOT/tools/dmpf-plugin/scripts/buf.sh" "$ROOT/tools/dmpf-plugin/scripts/buf-gate.sh" "$dir/tools/dmpf-plugin/scripts/"
  cp "$ROOT/tools/dmpf-plugin/versions.json" "$dir/tools/dmpf-plugin/"
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

# Leva um pacote do módulo do kernel para um módulo novo, com buf.yaml, buf.gen.yaml e
# go.mod próprios, mantendo o caminho relativo à raiz do módulo (REP-01).
mover_para_modulo() { # dir destino pacote-relativo
  local dir="$1" destino="$2" pacote="$3"
  mkdir -p "$dir/$destino/proto/$(dirname "$pacote")" || exit 2
  cp "$dir/libs/backend/go/contracts/buf.yaml" "$dir/$destino/buf.yaml" || exit 2
  sed -e 's|value: .*gen/go$|value: example.test/'"$destino"'/gen/go|' \
    -e 's|out: .*|out: gen/go|' "$dir/libs/backend/go/contracts/buf.gen.yaml" > "$dir/$destino/buf.gen.yaml" || exit 2
  cp "$dir/libs/backend/go/contracts/go.mod" "$dir/$destino/go.mod" || exit 2
  mv "$dir/libs/backend/go/contracts/proto/$pacote" "$dir/$destino/proto/$pacote" || exit 2
}

# Leva um pacote do módulo publicado para um módulo sem name no mesmo buf.yaml.
nao_publicar() { # dir pacote-relativo
  local mod="$1/libs/backend/go/contracts" pacote="$2"
  mkdir -p "$mod/testdata/proto/$(dirname "$pacote")" || exit 2
  mv "$mod/proto/$pacote" "$mod/testdata/proto/$pacote" || exit 2
  sed -i 's|^    name: buf.build/mateusmacedo/dmpf$|&\n  - path: testdata/proto|' "$mod/buf.yaml" || exit 2
  sed -i 's|^  - directory: proto$|&\n  - directory: testdata/proto|' "$mod/buf.gen.yaml" || exit 2
}

# Executa o gate dentro do sandbox, sem herdar GIT_* do ambiente atual. MODULO e
# PROJETO escolhem o módulo verificado (padrão: o do kernel, projeto contracts).
gate() { # dir subcomando [NX_BASE]
  local dir="$1" sub="$2" base="${3-__unset__}" mod="${MODULO:-libs/backend/go/contracts}" proj="${PROJETO:-contracts}"
  if [ "$base" = "__unset__" ]; then
    (cd "$dir" && env -u DMPF_BUF_VERSION -u NX_BASE -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE bash tools/dmpf-plugin/scripts/buf-gate.sh "$sub" "$mod" --project "$proj")
  else
    (cd "$dir" && env -u DMPF_BUF_VERSION -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE NX_BASE="$base" bash tools/dmpf-plugin/scripts/buf-gate.sh "$sub" "$mod" --project "$proj")
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
S="$(novo_sandbox)"; mv "$S/libs/backend/go/contracts" "$S.contracts-futuro"; commitar "$S" "base sem contratos"
mv "$S.contracts-futuro" "$S/libs/backend/go/contracts"; commitar "$S" "primeiro conteudo do modulo"
saida="$(gate "$S" breaking HEAD~1 2>&1)"; status=$?
esperar "bootstrap legitimo (modulo novo) libera" 0 $status "$saida"
casos=$((casos + 1))
if echo "$saida" | grep -q "sem baseline"; then echo "PASS  bootstrap avisa 'sem baseline'"; else echo "FAIL  bootstrap sem o aviso 'sem baseline'"; falhas=$((falhas + 1)); fi

S="$(novo_sandbox)"; commitar "$S" "contratos"
verificar "estado estabelecido sem mudanca incompativel" 0 "$S" breaking HEAD
verificar "NX_BASE vazio" 1 "$S" breaking "" "baseline nao declarado"
verificar "NX_BASE ausente" 1 "$S" breaking
verificar "baseline irresolvivel" 1 "$S" breaking refs/heads/nao-existe "baseline irresolvivel"

echo "== breaking: estado decidido pela base (BUF-08) =="
S="$(novo_sandbox)"; commitar "$S" "contratos"
echo "// comentario" >> "$S/libs/backend/go/contracts/proto/dmpf/testing/v1/order_placed.proto"; commitar "$S" "mudanca"
saida="$(gate "$S" breaking HEAD~1 2>&1)"; status=$?
esperar "modulo publicado na base sem mudanca incompativel libera" 0 $status "$saida"
casos=$((casos + 1))
if echo "$saida" | grep -q "breaking: OK (libs/backend/go/contracts/proto"; then echo "PASS  modulo publicado na base executa buf breaking"; else echo "FAIL  modulo publicado na base sem buf breaking"; echo "$saida" | tail -3 | sed 's/^/      /'; falhas=$((falhas + 1)); fi

# Renomear o módulo não o faz renascer 'sem baseline': a identidade é o pacote, e o
# pacote publicado na base continua sob buf breaking no módulo renomeado.
S="$(novo_sandbox)"; commitar "$S" "contratos"
git -C "$S" mv libs/backend/go/contracts/proto libs/backend/go/contracts/proto2
sed -i 's/path: proto$/path: proto2/' "$S/libs/backend/go/contracts/buf.yaml"
sed -i 's/int64 total_cents/int32 total_cents/' "$S/libs/backend/go/contracts/proto2/dmpf/testing/v1/order_placed.proto"; commitar "$S" "renomeia e quebra"
verificar "modulo renomeado continua sob buf breaking" 1 "$S" breaking HEAD~1 "buf breaking (FILE) reprovou"

S="$(novo_sandbox)"; commitar "$S" "contratos"
sed -i 's/int64 total_cents/int32 total_cents/' "$S/libs/backend/go/contracts/proto/dmpf/testing/v1/order_placed.proto"; commitar "$S" "tipo"
verificar "breaking FILE: int64 -> int32" 1 "$S" breaking HEAD~1 "buf breaking (FILE) reprovou"

S="$(novo_sandbox)"; commitar "$S" "contratos"
sed -i 's/^  - path: proto$//' "$S/libs/backend/go/contracts/buf.yaml"
verificar "buf.yaml sem modulos declarados" 1 "$S" breaking HEAD "sem modulos declarados"

S="$(novo_sandbox)"; commitar "$S" "contratos"
mv "$S/libs/backend/go/contracts/buf.yaml" "$S/libs/backend/go/contracts/buf.yaml.fora"
verificar "buf.yaml ausente" 1 "$S" breaking HEAD "ausente"

echo "== breaking: identidade por pacote entre modulos =="
S="$(novo_sandbox)"; commitar "$S" "contratos"
mover_para_modulo "$S" apps/orders/contract dmpf/testing; commitar "$S" "orders em modulo proprio"
MODULO=apps/orders/contract PROJETO=orders-contract verificar "pacote relocado para modulo novo roda contra a base" 0 "$S" breaking HEAD~1
saida="$(MODULO=apps/orders/contract PROJETO=orders-contract gate "$S" breaking HEAD~1 2>&1)"
casos=$((casos + 1))
if echo "$saida" | grep -q "breaking: OK (apps/orders/contract"; then echo "PASS  relocacao executa buf breaking herdado"; else echo "FAIL  relocacao sem buf breaking herdado"; echo "$saida" | tail -3 | sed 's/^/      /'; falhas=$((falhas + 1)); fi
verificar "modulo de origem apos a relocacao" 0 "$S" breaking HEAD~1

S="$(novo_sandbox)"; commitar "$S" "contratos"
mover_para_modulo "$S" apps/orders/contract dmpf/testing
sed -i 's/int64 total_cents/int32 total_cents/' "$S/apps/orders/contract/proto/dmpf/testing/v1/order_placed.proto"; commitar "$S" "relocado e quebrado"
MODULO=apps/orders/contract PROJETO=orders-contract verificar "quebra FILE apos relocar" 1 "$S" breaking HEAD~1 "buf breaking (FILE) reprovou"

S="$(novo_sandbox)"; commitar "$S" "contratos"
mv "$S/libs/backend/go/contracts/proto/dmpf/testing" "$S.pacote-fora"; commitar "$S" "remove pacote"
verificar "pacote publicado ausente de todos os modulos" 1 "$S" breaking HEAD~1 "pacote publicado"

S="$(novo_sandbox)"; commitar "$S" "contratos"
mover_para_modulo "$S" apps/orders/contract dmpf/testing; commitar "$S" "modulo novo"
MODULO=apps/orders/contract PROJETO=orders-contract verificar "modulo do projeto publicado na base roda buf breaking" 0 "$S" breaking HEAD

echo "== breaking: modulo sem name nao e publicado =="
S="$(novo_sandbox)"; nao_publicar "$S" dmpf/testing; commitar "$S" "contratos"
sed -i 's/int64 total_cents/int32 total_cents/' "$S/libs/backend/go/contracts/testdata/proto/dmpf/testing/v1/order_placed.proto"; commitar "$S" "tipo no pacote de teste"
verificar "quebra FILE em modulo sem name libera" 0 "$S" breaking HEAD~1

S="$(novo_sandbox)"; nao_publicar "$S" dmpf/testing; commitar "$S" "contratos"
mv "$S/libs/backend/go/contracts/testdata/proto/dmpf/testing" "$S.teste-fora"
printf 'syntax = "proto3";\n\npackage dmpf.testing.v2;\n\nmessage Probe {\n  string id = 1;\n}\n' > "$S.probe"
mkdir -p "$S/libs/backend/go/contracts/testdata/proto/dmpf/testing/v2"; mv "$S.probe" "$S/libs/backend/go/contracts/testdata/proto/dmpf/testing/v2/probe.proto"
commitar "$S" "troca o pacote de teste"
verificar "pacote de modulo sem name removido nao conta como publicado" 0 "$S" breaking HEAD~1

S="$(novo_sandbox)"; nao_publicar "$S" dmpf/testing; commitar "$S" "contratos"
sed -i 's/string spec_version = 3;/int32 spec_version = 3;/' "$S/libs/backend/go/contracts/proto/io/cloudevents/v1/cloudevents.proto"; commitar "$S" "quebra o publicado"
verificar "quebra FILE no modulo publicado ao lado de um sem name reprova" 1 "$S" breaking HEAD~1 "buf breaking (FILE) reprovou"

echo "== lint =="
S="$(novo_sandbox)"
printf 'syntax = "proto3";\n\npackage dmpf.testing.v1;\n\nenum Foo {\n  A = 0;\n}\n' > "$S/libs/backend/go/contracts/proto/dmpf/testing/v1/foo.proto"
verificar "enum sem sufixo _UNSPECIFIED" 1 "$S" lint "buf lint (STANDARD) reprovou"

S="$(novo_sandbox)"
echo "exactly-once delivery" > "$S/libs/backend/go/contracts/NOTAS.md"
verificar "exactly-once em artefato de contrato" 1 "$S" lint "P0-3"

echo "== generate-check =="
S="$(novo_sandbox)"; commitar "$S" "contratos"
printf '\n// drift\n' >> "$S/libs/backend/go/contracts/gen/go/dmpf/testing/v1/order_placed.pb.go"
verificar "byte alterado no gerado (drift)" 1 "$S" generate-check "drift"

echo "== pins =="
S="$(novo_sandbox)"
sed -i -E 's/"buf": "v[0-9.]+"/"buf": "latest"/' "$S/tools/dmpf-plugin/versions.json"
verificar "@latest na CLI" 1 "$S" pins "BUF-06"

S="$(novo_sandbox)"
sed -i -E 's/(google\.golang\.org\/protobuf v[0-9]+\.[0-9]+\.)[0-9]+/\10/' "$S/libs/backend/go/contracts/go.mod"
verificar "plugin e runtime protobuf divergentes" 1 "$S" pins "difere de google.golang.org/protobuf"

S="$(novo_sandbox)"
sed -i 's/^deps: \[\]/deps:\n  - buf.build\/exemplo\/dep/' "$S/libs/backend/go/contracts/buf.yaml"
verificar "deps declaradas sem buf.lock" 1 "$S" pins "sem buf.lock"

S="$(novo_sandbox)"
mover_para_modulo "$S" apps/orders/contract dmpf/testing
sed -i 's/protoc-gen-go@v[0-9.]*/protoc-gen-go@v1.36.0/' "$S/apps/orders/contract/buf.gen.yaml"
verificar "pins divergentes entre modulos" 1 "$S" pins "" "diverge entre modulos"

echo "== uso =="
S="$(novo_sandbox)"
saida="$(cd "$S" && bash tools/dmpf-plugin/scripts/buf-gate.sh lint 2>&1)"; status=$?
casos=$((casos + 1))
if [ "$status" -eq 2 ]; then echo "PASS  gate sem diretorio do modulo sai com 2"; else echo "FAIL  gate sem diretorio: exit $status"; falhas=$((falhas + 1)); fi

echo
echo "casos: $casos, falhas: $falhas"
[ "$falhas" -eq 0 ]
