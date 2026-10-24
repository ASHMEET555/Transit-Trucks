#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/.."

export DATABASE_URL="${DATABASE_URL:-postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable}"
go run ./cmd/bench/main.go
