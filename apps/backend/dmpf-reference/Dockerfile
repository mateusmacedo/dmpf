# syntax=docker/dockerfile:1

# Imagem do dmpf-reference: um binário, três papéis por `--role`.
#
# Build a partir da RAIZ do repositório — o go.work resolve os módulos irmãos
# de libs/backend/go, então o contexto precisa do workspace Go inteiro:
#   docker build -f apps/backend/dmpf-reference/Dockerfile -t dmpf-reference:local .
#
# Esta imagem NÃO entra no release Docker do workspace (nx-release.yml filtra
# `tag:type:app,!tag:stack:go`, ADR-041): ela serve ao compose local
# (infra/local) e aos manifestos Kustomize (infra/k8s).

FROM golang:1.26.6-alpine AS build
WORKDIR /src

COPY go.work go.work.sum ./
COPY libs/backend/go ./libs/backend/go
COPY apps/backend/dmpf-reference ./apps/backend/dmpf-reference

RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \
      -o /out/dmpf-reference ./apps/backend/dmpf-reference/cmd/dmpf-reference

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/dmpf-reference /dmpf-reference
# The published contract travels with the binary so the api can serve it at
# GET /openapi.yaml (DMPF_OPENAPI_PATH); it is the same file the routes reference.
COPY contracts/openapi /contracts/openapi
ENV DMPF_OPENAPI_PATH=/contracts/openapi/orders/v1/openapi.yaml
EXPOSE 8080
ENTRYPOINT ["/dmpf-reference"]
CMD ["--role", "api"]
