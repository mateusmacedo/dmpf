# syntax=docker/dockerfile:1

# Imagem do contexto orders: um binário, dois papéis por `--role` (api gRPC e relay).
#
# Build a partir da RAIZ do repositório — o go.work resolve os módulos irmãos
# de libs/backend/go, então o contexto precisa do workspace Go inteiro:
#   docker build -f apps/backend/orders/Dockerfile -t orders:local .
#
# Esta imagem NÃO entra no release Docker do workspace (nx-release.yml filtra
# `tag:type:app,!tag:stack:go`, ADR-041): ela serve ao compose local
# (infra/local) e aos manifestos Kustomize (infra/k8s).

FROM golang:1.26.6-alpine AS build
WORKDIR /src

COPY go.work go.work.sum ./
COPY libs/backend/go ./libs/backend/go
COPY apps/backend ./apps/backend

RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \
      -o /out/orders ./apps/backend/orders/cmd/orders

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/orders /orders
EXPOSE 9090
ENTRYPOINT ["/orders"]
CMD ["--role", "api"]
