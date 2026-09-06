#!/usr/bin/env bash
# Prova que o gate de dependência reprova o que deve reprovar, nos quatro
# blocos que o `.golangci.yml` conhece: `domain`, `port`, `application` e
# `contract`. A camada por package (depguard, VETORES_*) vale para os quatro; a
# camada por símbolo (forbidigo, SIMBOLOS) só para `domain`, porque fora dele
# `errors.New`, `fmt.Errorf` e `panic` são legítimos.
#
# Descobre os módulos pelo `dmpf-units.json` (a classificação autoritativa da
# RFC), e não por caminho fixo: quando um módulo novo nascer, ele entra aqui
# sozinho. Sem isso o gate seguiria verde sem nunca ter sido exercido nele.
#
# Os fixtures .go são artefatos de runtime deste script: nascem e morrem dentro
# de uma execução. Para retirá-los do módulo o script os MOVE para um diretório
# temporário, em vez de apagá-los — assim não depende de utilitário de lixeira,
# que o runner de CI não tem. O diretório fica em /tmp, que o runner descarta ao
# fim do job e a máquina local limpa no boot.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositorio git" >&2; exit 1; }
cd "$ROOT" || exit 1
DESCARTE="$(mktemp -d)" || { echo "falha ao criar diretorio temporario" >&2; exit 1; }

FIXTURE=""
clausula_de_package() {
  local f
  for f in "$1"/*.go; do
    case "$f" in *_test.go) continue ;; esac
    awk '/^package /{print; exit}' "$f" 2>/dev/null
    return
  done
}

retirar_fixture() {
  local status=$?
  if [ -n "$FIXTURE" ] && [ -e "$FIXTURE" ]; then
    mv -f "$FIXTURE" "$DESCARTE/descartado-$(date +%s%N).go" \
      || echo "AVISO: nao consegui retirar $FIXTURE do modulo" >&2
  fi
  FIXTURE=""
  return $status
}
trap retirar_fixture EXIT INT TERM

# pacote|capability|origem. `dentro` = listado na `deny` do .golangci.yml, prova
# a mensagem por capability. `fora` = ausente da `deny`, prova que a regra é
# fechada (`list-mode: strict`) e não uma mera denylist — sem estes dois, um
# `list-mode` removido por engano passaria despercebido.
#
# Um array por bloco porque as `allow` divergem, e um vetor aplicado ao bloco
# errado inverteria o resultado: `time` é permitido como import no `domain` (a
# distinção de símbolo fica com o forbidigo) e negado em `port` e `application`;
# `log` é negado em `domain` e `port` e PERMITIDO em `application`, cuja
# capability inclui observability (`capability.go:47`).
VETORES_DOMAIN=(
  "net/http|io.network|dentro"
  "os|io.filesystem|dentro"
  "encoding/json|wire.codec|dentro"
  "log|observability|dentro"
  "syscall|io.*|fora"
  "encoding/xml|wire.codec|fora"
)

VETORES_PORT=(
  "time|io.clock|dentro"
  "net/http|io.network|dentro"
  "log|observability|dentro"
  "syscall|io.*|fora"
)

VETORES_APPLICATION=(
  "time|io.clock|dentro"
  "net/http|io.network|dentro"
  "syscall|io.*|fora"
)

# `encoding/json` NAO entra como vetor: e wire.codec, a capability que so este
# bloco tem, e portanto esta na `allow`. `log` entra porque observability fica
# de fora de `contract`, como de `domain` e `port`.
VETORES_CONTRACT=(
  "time|io.clock|dentro"
  "net/http|io.network|dentro"
  "log|observability|dentro"
  "syscall|io.*|fora"
)

# Caminhos que `linters.exclusions.rules` tira do depguard, espelhados aqui como
# os VETORES_* espelham as `deny`. Exigir reprovacao neles seria exigir o que a
# configuracao deliberadamente nao faz; usa-los como alvo de fixture faria o
# gate reportar falha onde nao existe.
FORA_DO_DEPGUARD=(
  "dmpf-application/example/memory/"
  "dmpf-contracts/gen/"
)

fora_do_depguard() {
  local alvo="$1/"
  local excluido
  for excluido in "${FORA_DO_DEPGUARD[@]}"; do
    case "$alvo" in
      *"$excluido"*) return 0 ;;
    esac
  done
  return 1
}

# Diretorios dos `include` do manifesto, relativos a raiz do repositorio.
includes_do_modulo() {
  local manifesto="$1" module_dir="$2" module_path rel inc
  module_path="$(awk '/^module /{print $2; exit}' "$module_dir/go.mod" 2>/dev/null)"
  while IFS= read -r inc; do
    rel="${inc#"$module_path"}"
    rel="${rel#/}"
    [ -z "$rel" ] && continue
    echo "$module_dir/$rel"
  done < <(node -e '
    const m = require(process.argv[1]);
    for (const u of m.units ?? []) for (const i of u.include ?? []) console.log(i);
  ' "$ROOT/$manifesto" 2>/dev/null)
}

# import|corpo|familia. O import é permitido pelo depguard; só o símbolo cai.
# Uma família por vetor: remover um padrão do .golangci.yml reprova aqui. Só
# `domain`: o `exclusions.rules` do .golangci.yml restringe o forbidigo a
# `-domain/`.
SIMBOLOS=(
  'time|var _ = time.Now()|io.clock'
  'fmt|func init() { fmt.Println("x") }|io.stdout'
  '|func init() { println("x") }|builtin'
  'fmt|var s string; func init() { _, _ = fmt.Scanln(&s) }|io.stdin'
  'fmt|var _ = fmt.Errorf("x")|erro nao tipado'
  '|func init() { panic("x") }|lancamento'
)

falhas=0
modulos=0
modulos_domain=0
modulos_port=0
modulos_application=0
modulos_contract=0
fora_de_alcance=0

# Alcance das regras do .golangci.yml, espelhado aqui como os VETORES_*
# espelham as `deny`: as regras selecionam por `**/*-domain/**`,
# `**/*-ports/**` e `**/*-application/**`, isto e, por nome de diretorio.
# Modulo cujo caminho nao casa nenhum dos tres NAO e coberto pelo depguard, e
# provar o gate nele seria provar o que nao existe.
#
# Isso nao e o modulo ficar sem protecao: e a limitacao que o proprio
# .golangci.yml declara e que o verificador do KRN-02 fecha, lendo a
# classificacao autoritativa em vez do nome do diretorio.
#
# A escolha do array vem DESTE caminho, e nao do bloco declarado no manifesto,
# porque e o caminho que o depguard usa para decidir qual regra aplicar. Num
# modulo multi-bloco a leitura pelo manifesto seria ambigua: o
# `dmpf-application` declara unidades `application` e `provider`, e a regra que
# de fato incide sobre as duas e a `application`.
bloco_do_caminho() {
  case "/$1/" in
    *-domain/*)      echo "domain" ;;
    *-ports/*)       echo "port" ;;
    *-application/*) echo "application" ;;
    *-contracts/*)   echo "contract" ;;
    *)               echo "" ;;
  esac
}

while IFS= read -r manifesto; do
  module_dir="$(dirname "$manifesto")"

  # Exclusoes fechadas de RFC 10.3. Modulo sintetico sob testdata/ existe para
  # o verificador reprovar; trata-lo como producao faria o gate falhar pela
  # violacao que a fixture demonstra.
  case "/$module_dir/" in
    */testdata/*|*/vendor/*) continue ;;
  esac

  # só os blocos que o .golangci.yml conhece; um modulo que declare apenas
  # `provider`, `contract` ou `app` nao tem politica local a provar
  node -e '
    const m = require(process.argv[1]);
    const alvos = new Set(["domain", "port", "application", "contract"]);
    process.exit((m.units ?? []).some((u) => alvos.has(u.block)) ? 0 : 1);
  ' "$ROOT/$manifesto" 2>/dev/null || continue

  bloco="$(bloco_do_caminho "$module_dir")"
  if [ -z "$bloco" ]; then
    echo "-- $module_dir: fora do alcance do depguard (**/*-domain/**, **/*-ports/**, **/*-application/**, **/*-contracts/**); coberto pelo verificador do KRN-02"
    fora_de_alcance=$((fora_de_alcance + 1))
    continue
  fi

  case "$bloco" in
    domain)      vetores=("${VETORES_DOMAIN[@]}") ;;
    port)        vetores=("${VETORES_PORT[@]}") ;;
    application) vetores=("${VETORES_APPLICATION[@]}") ;;
    contract)    vetores=("${VETORES_CONTRACT[@]}") ;;
  esac

  project="$(node -p 'require(process.argv[1]).name' "$ROOT/$module_dir/project.json" 2>/dev/null)"
  if [ -z "$project" ]; then
    echo "FALHA  $module_dir: nao consegui ler o nome do projeto no project.json"
    falhas=$((falhas + 1))
    continue
  fi

  # O fixture de package precisa de um diretorio com .go, e a clausula vem do
  # proprio codigo: fixa-la aqui quebraria o fixture em modulo com outro nome de
  # package. A raiz e a primeira escolha; quando ela nao tem codigo, vale o
  # primeiro `include` que tenha — o dmpf-contracts declara tres unidades e
  # nenhum .go na raiz.
  fixture_dir=""
  pkg_clause=""
  for candidato in "$module_dir" $(includes_do_modulo "$manifesto" "$module_dir"); do
    fora_do_depguard "$candidato" && continue
    clausula="$(clausula_de_package "$candidato")"
    if [ -n "$clausula" ]; then
      fixture_dir="$candidato"
      pkg_clause="$clausula"
      break
    fi
  done
  if [ -z "$pkg_clause" ]; then
    echo "FALHA  $module_dir: nenhum .go com clausula de package dentro do alcance do depguard"
    falhas=$((falhas + 1))
    continue
  fi

  modulos=$((modulos + 1))
  case "$bloco" in
    domain)      modulos_domain=$((modulos_domain + 1)) ;;
    port)        modulos_port=$((modulos_port + 1)) ;;
    application) modulos_application=$((modulos_application + 1)) ;;
    contract)    modulos_contract=$((modulos_contract + 1)) ;;
  esac
  if [ "$fixture_dir" = "$module_dir" ]; then
    echo "== $project ($module_dir) [bloco $bloco]"
  else
    echo "== $project ($module_dir) [bloco $bloco, fixture em ${fixture_dir#"$module_dir/"}]"
  fi

  for vetor in "${vetores[@]}"; do
    pkg="${vetor%%|*}"
    resto="${vetor#*|}"
    cap="${resto%%|*}"
    origem="${resto##*|}"

    FIXTURE="$(mktemp "$fixture_dir/zz_gate_XXXXXX.go")" || {
      echo "FALHA  $project: nao consegui criar o fixture"
      falhas=$((falhas + 1))
      break
    }
    printf '%s\n\nimport _ "%s"\n' "$pkg_clause" "$pkg" > "$FIXTURE"

    saida="$(pnpm nx run "$project":lint --skip-nx-cache 2>&1)"
    status=$?
    retirar_fixture

    if [ "$status" -eq 0 ]; then
      echo "  FALHA  $pkg ($cap, $origem): o lint passou, mas deveria reprovar"
      falhas=$((falhas + 1))
    elif ! grep -qi "depguard" <<<"$saida"; then
      echo "  FALHA  $pkg ($cap, $origem): reprovou por outro motivo que nao o depguard"
      falhas=$((falhas + 1))
    else
      echo "  ok     $pkg ($cap, $origem)"
    fi
  done

  if [ "$bloco" = "domain" ]; then
    for simbolo in "${SIMBOLOS[@]}"; do
      imp="${simbolo%%|*}"
      resto="${simbolo#*|}"
      corpo="${resto%%|*}"
      familia="${resto##*|}"

      FIXTURE="$(mktemp "$fixture_dir/zz_gate_XXXXXX.go")" || {
        echo "FALHA  $project: nao consegui criar o fixture"
        falhas=$((falhas + 1))
        break
      }
      if [ -n "$imp" ]; then
        printf '%s\n\nimport "%s"\n\n%s\n' "$pkg_clause" "$imp" "$corpo" > "$FIXTURE"
      else
        printf '%s\n\n%s\n' "$pkg_clause" "$corpo" > "$FIXTURE"
      fi

      saida="$(pnpm nx run "$project":lint --skip-nx-cache 2>&1)"
      status=$?
      retirar_fixture

      if [ "$status" -eq 0 ]; then
        echo "  FALHA  $corpo ($familia): o lint passou, mas deveria reprovar"
        falhas=$((falhas + 1))
      elif ! grep -qi "forbidigo" <<<"$saida"; then
        echo "  FALHA  $corpo ($familia): reprovou por outro motivo que nao o forbidigo"
        falhas=$((falhas + 1))
      else
        echo "  ok     $corpo ($familia, forbidigo)"
      fi
    done
  fi

  # Cada `include` fora da raiz do modulo e uma unidade propria (RFC 3.3). Um
  # fixture em cada uma prova que o glob do depguard chega ao subpackage, em vez
  # de presumi-lo a partir da raiz. Em `domain` o fixture e de simbolo
  # (forbidigo); nos outros blocos e de package, porque o forbidigo esta
  # restrito a `-domain/`.
  #
  # Subpackage cujo bloco declarado nao tem politica local — `provider` e `app`,
  # que o verificador deixa irrestritos — e DECLARADO como fora do gate, nunca
  # exercitado: ele esta excluido do depguard em `linters.exclusions.rules`, e
  # exigir reprovacao ali seria exigir o que a config deliberadamente nao faz.
  module_path="$(awk '/^module /{print $2; exit}' "$module_dir/go.mod" 2>/dev/null)"
  while IFS= read -r linha; do
    inc="${linha%%|*}"
    inc_bloco="${linha##*|}"
    rel="${inc#"$module_path"}"
    rel="${rel#/}"
    [ -z "$rel" ] && continue
    sub_dir_candidato="$module_dir/$rel"

    case "$inc_bloco" in
      domain|port|application|contract) ;;
      *)
        echo "  --     $rel: unidade $inc_bloco, sem politica local; coberta pelo verificador do KRN-02"
        continue
        ;;
    esac

    if fora_do_depguard "$sub_dir_candidato"; then
      echo "  --     $rel: excluida do depguard em exclusions.rules; coberta pelo verificador do KRN-02"
      continue
    fi

    sub_dir="$module_dir/$rel"
    sub_clause="$(clausula_de_package "$sub_dir")"
    if [ -z "$sub_clause" ]; then
      echo "  FALHA  $rel: nenhum .go com clausula de package no include"
      falhas=$((falhas + 1))
      continue
    fi

    FIXTURE="$(mktemp "$sub_dir/zz_gate_XXXXXX.go")" || {
      echo "FALHA  $project: nao consegui criar o fixture em $rel"
      falhas=$((falhas + 1))
      break
    }
    if [ "$bloco" = "domain" ]; then
      printf '%s\n\nimport "time"\n\nvar _ = time.Now()\n' "$sub_clause" > "$FIXTURE"
      esperado="forbidigo"
      rotulo="time.Now() (io.clock, forbidigo, subpackage)"
    else
      printf '%s\n\nimport _ "time"\n' "$sub_clause" > "$FIXTURE"
      esperado="depguard"
      rotulo="import time (io.clock, depguard, subpackage)"
    fi

    saida="$(pnpm nx run "$project":lint --skip-nx-cache 2>&1)"
    status=$?
    retirar_fixture

    if [ "$status" -eq 0 ]; then
      echo "  FALHA  $rel: o fixture passou no subpackage, mas deveria reprovar"
      falhas=$((falhas + 1))
    elif ! grep -qi "$esperado" <<<"$saida"; then
      echo "  FALHA  $rel: reprovou por outro motivo que nao o $esperado"
      falhas=$((falhas + 1))
    else
      echo "  ok     $rel: $rotulo"
    fi
  done < <(node -e '
    const m = require(process.argv[1]);
    for (const u of m.units ?? []) for (const i of u.include ?? []) console.log(i + "|" + u.block);
  ' "$ROOT/$manifesto" 2>/dev/null)

  # Vetor positivo: a árvore limpa precisa passar, senão o gate só sabe dizer não.
  if pnpm nx run "$project":lint --skip-nx-cache >/dev/null 2>&1; then
    echo "  ok     arvore limpa: aprovada"
  else
    echo "  FALHA  arvore limpa: reprovada, mas deveria passar"
    falhas=$((falhas + 1))
  fi
done < <(git ls-files '*dmpf-units.json')

if [ "$modulos" -eq 0 ]; then
  echo "FALHA: nenhum modulo com bloco domain, port, application ou contract encontrado; o gate nao exercitou nada." >&2
  exit 1
fi

if [ "$falhas" -gt 0 ]; then
  echo
  echo "$falhas verificacao(oes) falharam: o gate nao esta protegendo os blocos domain, port, application e contract."
  exit 1
fi

echo
echo "Gate de dependencia: $modulos modulo(s) — domain: $modulos_domain, port: $modulos_port, application: $modulos_application, contract: $modulos_contract."
echo "Vetores de package por bloco: domain ${#VETORES_DOMAIN[@]}, port ${#VETORES_PORT[@]}, application ${#VETORES_APPLICATION[@]}, contract ${#VETORES_CONTRACT[@]}; ${#SIMBOLOS[@]} de simbolo em domain; 1 positivo por modulo. Todos conformes."
if [ "$fora_de_alcance" -gt 0 ]; then
  # Declarado, nunca silencioso: um gate que esconde o proprio alcance passa a
  # informar cobertura que nao tem.
  echo "$fora_de_alcance modulo(s) fora do alcance do depguard, cobertos pelo verificador do KRN-02."
fi
