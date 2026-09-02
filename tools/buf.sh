#!/usr/bin/env bash
# Único lugar do pin da CLI Buf (BUF-06); o pin do plugin vive em contracts/buf.gen.yaml.
exec go run github.com/bufbuild/buf/cmd/buf@v1.72.0 "$@"
