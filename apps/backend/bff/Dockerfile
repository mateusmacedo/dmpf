# syntax=docker/dockerfile:1

# Imagem do bff: a única borda REST pública da topologia de
# referência, que chama os contextos orders e reservations por gRPC.
#
# Build a partir da RAIZ do repositório — o go.work resolve os módulos irmãos
# (libs/backend/go e tools/dmpf-conformance) e recusa um `use` ausente do
# contexto, então o COPY leva todos os módulos do workspace Go:
#   docker build -f apps/backend/bff/Dockerfile -t bff:local .
#
# Esta imagem NÃO entra no release Docker do workspace (nx-release.yml filtra
# `tag:type:app,!tag:stack:go`, ADR-041): ela serve ao compose local
# (infra/local) e aos manifestos Kustomize (infra/k8s).

FROM golang:1.26.6-alpine AS build
WORKDIR /src

COPY go.work go.work.sum ./
COPY libs/backend/go ./libs/backend/go
COPY tools/dmpf-conformance ./tools/dmpf-conformance
COPY apps/backend ./apps/backend

RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \
      -o /out/bff ./apps/backend/bff/cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bff /bff
COPY contracts/openapi /contracts/openapi
ENV DMPF_OPENAPI_ORDERS_PATH=/contracts/openapi/orders/v1/openapi.yaml
ENV DMPF_OPENAPI_RESERVATIONS_PATH=/contracts/openapi/reservations/v1/openapi.yaml
EXPOSE 8080
ENTRYPOINT ["/bff"]
