#!/usr/bin/env sh
set -eu

export GOWORK="${GOWORK:-off}"
export GOFLAGS="${GOFLAGS:--mod=vendor}"
export GOTOOLCHAIN="${GOTOOLCHAIN:-auto}"

echo "[1/3] Building Hanzo Mail binary..."
go build -o ./mox .

echo "[2/3] Running loopback localserve SMTP -> IMAP -> DKIM & webmail smoke test..."
go test -count=1 -v -run '^TestLocalserveLoopbackSmoke$' .

echo "[3/3] Smoke verification complete."
echo "To run interactively in a fresh directory:"
echo "  ./mox localserve -dir /tmp/mox-localserve"
echo "  (Credentials: mox@localhost / moxmoxmox | Submission: 1587 | IMAP: 1143 | Webmail: http://localhost:1080/webmail/)"
