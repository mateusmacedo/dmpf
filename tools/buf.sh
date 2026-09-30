#!/usr/bin/env bash
# Único lugar do pin da CLI Buf (BUF-06); o pin do plugin vive no buf.gen.yaml de cada módulo de contrato.
exec go run github.com/bufbuild/buf/cmd/buf@v1.72.0 "$@"
