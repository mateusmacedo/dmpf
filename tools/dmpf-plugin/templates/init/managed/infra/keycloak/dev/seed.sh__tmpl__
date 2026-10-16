#!/usr/bin/env bash
# WHY: cada chamada do kcadm.sh sobe uma JVM, então o script lê cada lista uma
# vez em vez de consultar item a item. Os dados são de teste: nunca rodar no hmg.
set -euo pipefail

: "${KEYCLOAK_URL:?KEYCLOAK_URL ausente}"
admin_user="${KC_BOOTSTRAP_ADMIN_USERNAME:-admin}"
admin_password="${KC_BOOTSTRAP_ADMIN_PASSWORD:-admin}"
seed_password="${KEYCLOAK_SEED_PASSWORD:-dmpf-dev-password}"
otp_secret="${KEYCLOAK_SEED_OTP_SECRET:-dmpfdmpfdmpfdmpfdmpf}"
realm=dmpf
config="$(mktemp)"
body="$(mktemp)"
trap 'rm -f "$config" "$body"' EXIT

fail() {
  echo "seed: $1" >&2
  exit 1
}

case "$seed_password$otp_secret" in
  *[\"\\]*) fail "KEYCLOAK_SEED_PASSWORD e KEYCLOAK_SEED_OTP_SECRET não aceitam aspas nem barra invertida" ;;
esac

kc() { /opt/keycloak/bin/kcadm.sh "$@" --config "$config"; }
field() { grep ",$2\$" <<<"$1" | cut -d, -f1 || true; }
contains() { grep -qx "$2" <<<"$1"; }

logged_in=""
for _ in $(seq 1 60); do
  if kc config credentials --server "$KEYCLOAK_URL" --realm master \
    --user "$admin_user" --password "$admin_password" >/dev/null 2>&1; then
    logged_in=1
    break
  fi
  sleep 2
done
[ -n "$logged_in" ] || fail "login no realm master falhou em $KEYCLOAK_URL"

ensure_org() {
  local name=$1 alias=$2 tenant=$3 id
  id="$(field "$orgs" "$alias")"
  if [ -z "$id" ]; then
    id="$(kc create organizations -r "$realm" -s "name=$name" -s "alias=$alias" -i)"
  fi
  kc update "organizations/$id" -r "$realm" -s "attributes={\"tenant_id\":[\"$tenant\"]}"
  echo "$id"
}

write_user() {
  local username=$1 first=$2 last=$3 otp=$4 credentials
  credentials="{\"type\": \"password\", \"value\": \"$seed_password\", \"temporary\": false}"
  if [ "$otp" = otp ]; then
    credentials="$credentials, {\"type\": \"otp\", \"userLabel\": \"seed\",
      \"secretData\": \"{\\\"value\\\":\\\"$otp_secret\\\"}\",
      \"credentialData\": \"{\\\"subType\\\":\\\"totp\\\",\\\"digits\\\":6,\\\"period\\\":30,\\\"algorithm\\\":\\\"HmacSHA1\\\"}\"}"
  fi
  cat >"$body" <<JSON
{"username": "$username", "enabled": true, "emailVerified": true, "email": "$username@dmpf.test",
 "firstName": "$first", "lastName": "$last", "credentials": [$credentials]}
JSON
}

ensure_user() {
  local username=$1 first=$2 last=$3 otp=${4:-} id
  id="$(field "$users" "$username")"
  if [ -n "$id" ] && [ "$otp" = otp ]; then
    kc delete "users/$id" -r "$realm"
    id=""
  fi
  write_user "$username" "$first" "$last" "$otp"
  if [ -z "$id" ]; then
    kc create users -r "$realm" -f "$body" -i
    return
  fi
  kc update "users/$id" -r "$realm" -s enabled=true -s emailVerified=true \
    -s "email=$username@dmpf.test" -s "firstName=$first" -s "lastName=$last"
  kc set-password -r "$realm" --userid "$id" --new-password "$seed_password"
  echo "$id"
}

members_of() {
  kc get "organizations/$1/members" -r "$realm" -q max=1000 --fields id --format csv --noquotes
}

ensure_members() {
  local org=$1 current user
  shift
  current="$(members_of "$org")"
  for user in "$@"; do
    contains "$current" "$user" || kc create "organizations/$org/members" -r "$realm" -b "\"$user\""
  done
}

orgs="$(kc get organizations -r "$realm" -q max=1000 --fields id,alias --format csv --noquotes)"
acme="$(ensure_org Acme acme t-acme)"
globex="$(ensure_org Globex globex t-globex)"

users="$(kc get users -r "$realm" -q max=1000 --fields id,username --format csv --noquotes)"
alice="$(ensure_user alice Alice Silva)"
bob="$(ensure_user bob Bob Souza)"
ensure_user carol Carol Lima >/dev/null
dave="$(ensure_user dave Dave Costa otp)"

ensure_members "$acme" "$alice" "$bob" "$dave"
ensure_members "$globex" "$bob"
kc add-roles -r "$realm" --uid "$dave" --rolename mfa-required

acme_members="$(members_of "$acme")"
globex_members="$(members_of "$globex")"
acme_org="$(kc get "organizations/$acme" -r "$realm")"
globex_org="$(kc get "organizations/$globex" -r "$realm")"
dave_credentials="$(kc get "users/$dave/credentials" -r "$realm" --fields type --format csv --noquotes)"
dave_roles="$(kc get-roles -r "$realm" --uid "$dave" --effective --fields name --format csv --noquotes)"

grep -q '"t-acme"' <<<"$acme_org" || fail "pós-condição falhou: tenant de acme"
grep -q '"t-globex"' <<<"$globex_org" || fail "pós-condição falhou: tenant de globex"
for member in "$alice" "$bob" "$dave"; do
  contains "$acme_members" "$member" || fail "pós-condição falhou: membro de acme $member"
done
contains "$globex_members" "$bob" || fail "pós-condição falhou: bob em globex"
contains "$dave_credentials" otp || fail "pós-condição falhou: dave com OTP"
contains "$dave_roles" mfa-required || fail "pós-condição falhou: dave com mfa-required"
echo "seed: realm $realm convergido"
