#!/usr/bin/env bash
# Gates Buf do repositório de contratos: lint | pins | generate-check | breaking.
# Fail-closed (BUF-12): condição que o gate não consegue avaliar reprova; não há bypass.
# Diretórios temporários ficam em /tmp (runner e boot os descartam), como em
# tools/dmpf-gate-check.sh, para não depender de utilitário de lixeira.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "REPROVADO: fora de um repositorio git" >&2; exit 2; }
cd "$ROOT" || exit 2

CONTRACTS=contracts
BUF_YAML="$CONTRACTS/buf.yaml"
BUF_GEN="$CONTRACTS/buf.gen.yaml"
BUF_SH="$ROOT/tools/buf.sh"
LIB=libs/shared/go/dmpf-contracts
GEN_DIR="$LIB/gen/go"
MARK_PREFIX=contracts-baseline

buf() { bash "$BUF_SH" "$@"; }
reprovar() { echo "REPROVADO: $*" >&2; exit 1; }
aviso() { echo "AVISO: $*" >&2; }

modulos() {
  grep -E '^\s*-\s*path:' "$BUF_YAML" | sed -E 's/^\s*-\s*path:\s*//; s/\s*$//'
}

gate_lint() {
  buf format --diff --exit-code "$CONTRACTS" || reprovar "buf format encontrou diferencas em $CONTRACTS"
  buf lint "$CONTRACTS" || reprovar "buf lint (STANDARD) reprovou"
  # P0-3 e structurally reviewable: a vedacao vale para qualquer texto do artefato.
  if grep -rIliE 'exactly[ -]once' "$CONTRACTS" "$LIB" 2>/dev/null | grep -q .; then
    grep -rIliE 'exactly[ -]once' "$CONTRACTS" "$LIB" >&2
    reprovar "artefato declara ou sugere exactly-once (RFC 2.3, P0-3)"
  fi
  echo "lint: OK"
}

gate_pins() {
  local cli_pins plugin_pin plugin_ver runtime_ver deps_line
  cli_pins="$(grep -oE '@v[0-9]+\.[0-9]+\.[0-9]+([^0-9.]|$)' "$BUF_SH" | wc -l | tr -d ' ')"
  [ "$cli_pins" -eq 1 ] || reprovar "$BUF_SH precisa de exatamente um pin @vX.Y.Z da CLI Buf (encontrados: $cli_pins) (BUF-06)"
  if grep -qE '@(latest|v[0-9]+(\.[0-9]+)?([^0-9.]|$)|v[0-9]+\.[0-9]+\.x)' "$BUF_SH"; then
    reprovar "$BUF_SH usa pin nao exato (latest, faixa ou major/minor) (BUF-06)"
  fi

  plugin_pin="$(grep -oE 'protoc-gen-[a-z0-9-]+@[^[:space:]"]+' "$BUF_GEN" || true)"
  [ -n "$plugin_pin" ] || reprovar "$BUF_GEN sem plugin local pinado (BUF-06)"
  while IFS= read -r pin; do
    echo "$pin" | grep -qE '@v[0-9]+\.[0-9]+\.[0-9]+$' || reprovar "plugin sem pin exato: $pin (BUF-06)"
  done <<<"$plugin_pin"
  if grep -qiE 'buf\.build/[^[:space:]]+/[^[:space:]]+:(latest|v[0-9]+$)' "$BUF_GEN"; then
    reprovar "$BUF_GEN referencia plugin remoto sem pin exato (BUF-06)"
  fi

  plugin_ver="$(echo "$plugin_pin" | grep -oE 'protoc-gen-go@v[0-9]+\.[0-9]+\.[0-9]+' | sed 's/.*@//')"
  runtime_ver="$(grep -oE 'google\.golang\.org/protobuf v[0-9]+\.[0-9]+\.[0-9]+' "$LIB/go.mod" | awk '{print $2}')"
  [ -n "$plugin_ver" ] || reprovar "protoc-gen-go sem versao em $BUF_GEN"
  [ -n "$runtime_ver" ] || reprovar "google.golang.org/protobuf ausente de $LIB/go.mod"
  # O gerado exige runtime >= versao do plugin; igualar os dois e o unico estado sem drift silencioso.
  [ "$plugin_ver" = "$runtime_ver" ] || reprovar "protoc-gen-go $plugin_ver difere de google.golang.org/protobuf $runtime_ver no go.mod"

  deps_line="$(grep -E '^deps:' "$BUF_YAML" || true)"
  [ -n "$deps_line" ] || reprovar "$BUF_YAML sem chave deps (BUF-02)"
  if ! echo "$deps_line" | grep -qE '^deps:\s*\[\s*\]\s*$'; then
    [ -f "$CONTRACTS/buf.lock" ] || reprovar "deps declaradas sem buf.lock versionado (BUF-02)"
  fi
  echo "pins: OK (buf $(grep -oE '@v[0-9.]+' "$BUF_SH"), protoc-gen-go@$plugin_ver = protobuf $runtime_ver)"
}

gate_generate_check() {
  local t1 t2
  t1="$(mktemp -d)" || reprovar "mktemp"
  t2="$(mktemp -d)" || reprovar "mktemp"
  (cd "$CONTRACTS" && buf generate -o "$t1/$CONTRACTS") || reprovar "buf generate (1) falhou"
  (cd "$CONTRACTS" && buf generate -o "$t2/$CONTRACTS") || reprovar "buf generate (2) falhou"
  diff -r "$t1" "$t2" >/dev/null || { diff -r "$t1" "$t2" | head -20 >&2; reprovar "duas geracoes consecutivas divergem (BUF-11)"; }
  [ -d "$t1/$GEN_DIR" ] || reprovar "geracao nao produziu $GEN_DIR"
  diff -r "$t1/$GEN_DIR" "$GEN_DIR" >/dev/null || { diff -r "$t1/$GEN_DIR" "$GEN_DIR" | head -40 >&2; reprovar "drift entre o gerado e o versionado em $GEN_DIR (BUF-11, REP-02)"; }
  echo "generate-check: OK"
}

gate_breaking() {
  local base modulo marca tagger primeiro_autor base_dir
  base="${NX_BASE:-}"
  [ -n "$base" ] || reprovar "baseline nao declarado: NX_BASE vazio (BUF-05)"
  git rev-parse --verify --quiet "${base}^{commit}" >/dev/null || reprovar "baseline irresolvivel: $base (BUF-05)"

  for modulo in $(modulos); do
    marca="refs/tags/$MARK_PREFIX/$modulo"
    if git rev-parse --verify --quiet "$marca" >/dev/null; then
      tagger="$(git for-each-ref --format='%(taggeremail)' "$marca")"
      [ -n "$tagger" ] || reprovar "marca $marca nao e uma tag anotada (sem tagger) (BUF-08)"
      primeiro_autor="$(git log --diff-filter=A --format='<%ae>' --reverse -- "$CONTRACTS/$modulo" | head -1)"
      [ -n "$primeiro_autor" ] || reprovar "sem historico de $CONTRACTS/$modulo para conferir a autoria (BUF-08)"
      [ "$tagger" != "$primeiro_autor" ] || reprovar "marca $marca criada pelo autor do modulo: autoria e autorizacao coincidem (BUF-08)"

      base_dir="$(mktemp -d)" || reprovar "mktemp"
      git archive --format=tar "$base" -- "$CONTRACTS" | tar -x -C "$base_dir" || reprovar "nao foi possivel extrair $CONTRACTS de $base (BUF-05)"
      [ -d "$base_dir/$CONTRACTS/$modulo" ] || reprovar "baseline $base nao contem $CONTRACTS/$modulo (BUF-05)"
      buf breaking "$CONTRACTS" --against "$base_dir/$CONTRACTS" || reprovar "buf breaking (FILE) reprovou contra $base"
      echo "breaking: OK ($modulo contra $base)"
    else
      if git ls-tree "$base" -- "$CONTRACTS/$modulo" | grep -q .; then
        reprovar "modulo $modulo existe em $base sem marca $marca: estado invalido, nao 'sem baseline' (BUF-08)"
      fi
      aviso "modulo $modulo em estado 'sem baseline': buf breaking dispensado ate a marca $marca existir (BUF-08)"
    fi
  done

  # Um pacote publicado nao muda de modulo nem de caminho para renascer 'sem baseline'.
  while IFS= read -r dir; do
    [ -z "$dir" ] && continue
    [ -d "$dir" ] || reprovar "diretorio de pacote publicado em $base ausente em HEAD: $dir (BUF-08)"
  done < <(git ls-tree -r --name-only "$base" -- "$CONTRACTS" 2>/dev/null | grep -E '\.proto$' | xargs -rn1 dirname | sort -u)
}

case "${1:-}" in
  lint) gate_lint ;;
  pins) gate_pins ;;
  generate-check) gate_generate_check ;;
  breaking) gate_breaking ;;
  *) echo "uso: tools/buf-gate.sh lint | pins | generate-check | breaking" >&2; exit 2 ;;
esac
