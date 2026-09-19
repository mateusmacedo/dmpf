# syntax=docker/dockerfile:1

# Imagem do contexto reservations: um binário, três papéis por `--role` (api gRPC, relay e consumer).
#
# Build a partir da RAIZ do repositório — o go.work resolve os módulos irmãos
# (libs/backend/go e tools/dmpf-conformance) e recusa um `use` ausente do
# contexto, então o COPY leva todos os módulos do workspace Go:
#   docker build -f apps/backend/reservations/Dockerfile -t reservations:local .
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
      -o /out/reservations ./apps/backend/reservations/cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/reservations /reservations
EXPOSE 9090
ENTRYPOINT ["/reservations"]
CMD ["--role", "api"]
