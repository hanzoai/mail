# Hanzo Mail

Fork of Mox. Mail server (SMTP, IMAP4, webmail, DKIM/SPF/DMARC, autoconfig, ACME).
This is the foundation for replacing Google Workspace email at Hanzo.

## Lineage

| | |
|---|---|
| Upstream | [github.com/mjl-/mox](https://github.com/mjl-/mox) |
| Upstream author | Mechiel Lukkien <mechiel@ueber.net> |
| Upstream license | MIT |
| Fork point | `9bbad6af30ae1c429dde9cdc2d04130532e1fed1` (v0.0.15 + 55 commits, 2026-07-21) |
| This repo | [github.com/hanzoai/mail](https://github.com/hanzoai/mail) |
| Module path | `github.com/hanzoai/mail` |

This is a real GitHub fork (`isFork: true`, parent `mjl-/mox`), matching how
`hanzoai/git` (← go-gitea/gitea) and `hanzoai/deploy` (← argoproj/argo-cd) were
done. Provenance is therefore visible in the GitHub UI as well as in `NOTICE`.
Full upstream history is preserved; `git remote upstream` points at mjl-/mox.

## Licensing — what applies to what

Verified by reading the tree, not assumed. `NOTICE` is the authoritative
statement; `mail licenses` prints it from the running binary.

1. **Mox source + Hanzo's changes** — MIT (`LICENSE.MIT`), © 2021 Mechiel
   Lukkien, plus Hanzo AI, Inc. for modifications. Not relicensed.
2. **`publicsuffix/public_suffix_list.txt`** — MPL-2.0 (`LICENSE.MPLv2.0`),
   Mozilla's Public Suffix List. **This one data file is the only thing the MPL
   covers.** Mox is *not* dual-licensed. Proof: `mox-/licenses.go` labels
   `LICENSE.MPLv2.0` as `https://publicsuffix.org - Public Suffix List Mozilla`,
   separate from mox's own MIT, and `README.md` says the same.
3. **`vendor/**`** — third-party deps under their own licenses. Copies collected
   verbatim in `licenses/` (42 files) by `genlicenses.sh`, which reads `vendor/`.
   Includes BSD-3-Clause from The Go Authors (`golang.org/x/...` and the
   Go-derived forks `mjl-/{adns,autocert,flate,sherpa}`) and Apache-2.0
   (Prometheus, protobuf). The `licenses/` tree is *dependency* licenses — it is
   not a license grant over Mox or Hanzo Mail.

Mox was chosen over Stalwart deliberately: Stalwart is AGPL-3.0-only plus a
proprietary per-file SELv2 enterprise license. AGPL's network clause would force
publishing our fork and the SELv2 files cannot be forked commercially. Mox is
MIT and Go, which matches our stack and our licensing posture.

## What was renamed

Only the Go module path, and only where it affects the build:

- `go.mod` module line.
- 268 `.go` files — import paths `github.com/mjl-/mox/...` → `github.com/hanzoai/mail/...`.
- 5 scripts that `go install` the module and would otherwise build **upstream**
  instead of this fork: `genwebsite.sh`, `docker-release.sh`, `apidiff.sh`,
  `test-upgrade.sh`, `Dockerfile.release`.
- `licenses.go` — embeds `NOTICE`; `mox-/licenses.go` prints it first and labels
  the MIT section as a fork of mjl-/mox, so the binary states its own lineage.

## What was NOT touched — and why

- **`LICENSE.MIT`, `LICENSE.MPLv2.0`, `licenses/`** — preserved verbatim. Never
  edit these.
- **`vendor/`** — no dependency was added, removed, or bumped.
- **`apidiff/v0.0.9.txt`, `v0.0.10.txt`, `v0.0.11.txt`, `next.txt`** — historical
  records of *upstream's* API at upstream releases. Rewriting the paths in them
  would falsify a historical record. Leave them.
- **`website/*.md`, `develop.txt`** — upstream's own website prose.
  (`develop.txt` also references `mjl-/mox-website-files`, a *different* repo —
  do not let a careless `sed` catch that prefix.) Note `website/website.go` *is*
  Go source and did change: the rename also repointed its "Sources at github"
  and "feedback?" links at `hanzoai/mail`, which is the right outcome for a fork.
- **The `mox` command name, config keys, data directory layout, and the `mox`
  Go package name in `mox-/`.** Renaming these is a separate, larger job that
  breaks on-disk config and data compatibility. The binary still prints
  `usage: mox ...`. Deliberate: the module rename is what makes the fork
  independently buildable; the rest is branding with real breakage risk.
- **`webmail/webmail.ts` / `webmail.js`** — still link to `mjl-/mox/issues` for
  bug reports. `webmail.js` is generated from the `.ts`, so this needs the
  frontend toolchain, not a `sed`. **Blocking before any deploy** — otherwise our
  webmail sends Hanzo users to file bugs on upstream's tracker.

## Remaining references to upstream — all intentional

`grep -rc 'mjl-/mox' --exclude-dir={.git,vendor,licenses} .` should return only
these. Anything else is an accident and should be reviewed:

| File | Count | Why |
|---|---|---|
| `apidiff/v0.0.9.txt`, `v0.0.10.txt`, `v0.0.11.txt`, `next.txt` | 63 | Historical records of upstream's API. Falsifying them would be dishonest. |
| `website/index.md`, `install/index.md`, `features/index.md` | 22 | Upstream website prose, not rebranded. |
| `README.md`, `LLM.md`, `NOTICE` | 16 | Deliberate attribution. |
| `webmail/webmail.ts`, `webmail.js` | 5 | Upstream issue-tracker links — **the pre-deploy blocker above.** |
| `mox-/licenses.go` | 1 | The "fork of github.com/mjl-/mox" label in `mail licenses`. |
| `develop.txt` | 1 | Points at `mjl-/mox-website-files`, a different repo. |

Verify the fork's exact delta at any time with `git diff fork-point..main`. The
`fork-point` tag is an annotated tag on upstream commit
`9bbad6af30ae1c429dde9cdc2d04130532e1fed1`. That diff must never show changes to
`LICENSE.MIT`, `LICENSE.MPLv2.0`, `licenses/`, or `vendor/` — currently it shows
none.

## Build and test

```sh
export PATH=$PATH:/usr/local/go/bin
export GOWORK=off GOFLAGS=-mod=vendor   # a parent go.work in ~/work/hanzo shadows this repo
go build ./...
go test ./...
```

`GOWORK=off` is required — without it `go build ./...` fails with "directory
prefix . does not contain modules listed in go.work".

Integration tests are behind `//go:build integration` and need Docker; the
default `go test ./...` excludes them.

**Test flakiness is pre-existing upstream, not caused by the fork.** `go test
./...` runs up to 32 package binaries at once on this box; mox's server tests
use hard 1s deadlines (`server_test.go:414` "server not done within 1s") and
starve. The failing set differs every run — baseline upstream (before any change)
failed `TestQresyncUIDOnly`/`TestSubscribe`; the renamed tree failed a different
set. Both pass in isolation. Use `go test -p 1 ./...` for a clean signal.

## Standing it up — next steps

Nothing is deployed. No DNS points anywhere. Repo setup only.

1. **Mailbox storage.** Per-account BoltDB index plus message files on disk:
   `<DataDir>/accounts/<name>/index.db` and `<DataDir>/accounts/<name>/msg/<shard>/<id>`.
   This is a *stateful, single-writer* design — it is not a stateless service and
   does not shard across replicas. It needs a real PersistentVolume with backups
   (`mail backup <destdir>` and `mail verifydata <data-dir>` are built in), not
   an emptyDir. Decide the storage class and backup target before anything else;
   this is the main architectural constraint of the whole project.
2. **Config and keys.** `mail quickstart user@domain` generates `mox.conf`,
   `domains.conf`, and DKIM keys. DKIM private keys and the admin/account
   password hashes are secrets — they go in KMS (`kms.hanzo.ai`) and reach the
   cluster via KMSSecret CRDs. Never commit them. Passwords are hashed by mox
   (bcrypt/SCRAM credentials); do not add any plaintext path.
3. **DKIM / SPF / DMARC / MX / DNS.** `mail config dnsrecords <domain>` prints
   every record to publish; `mail config dnscheck <domain>` verifies them.
   Publish in Cloudflare. MX records must **not** be proxied. Sequence matters:
   stand the server up and verify with `dnscheck` *before* moving MX off Google,
   or mail bounces.
4. **Migration from Google.** There is **no built-in IMAP puller** — verified, not
   assumed. Two real options: (a) Google Takeout → mbox → `mail import mbox
   <account> <mailbox> <file>` (also available as mbox/maildir `.zip`/`.tgz`
   upload in the web account UI); or (b) an external IMAP-to-IMAP sync tool run
   against Hanzo Mail's IMAP server. Pick one and pilot it on a single throwaway
   account before touching anyone's real mail.
5. **CI/CD and image.** Images build in CI only — never locally — and publish to
   `ghcr.io/hanzoai/mail`. Note `docker-release.sh`/`Dockerfile.release` still
   carry upstream's release flow and registry; repoint them (or replace with a
   `.hanzo/workflows` pipeline) as part of the deploy work, not before.
6. **IAM.** Mox has its own account/password store. Integrating Hanzo IAM
   (hanzo.id, OIDC) is a genuine design question, because IMAP and SMTP clients
   authenticate with SASL, not OIDC — expect app passwords or a SASL-to-IAM
   bridge. Design it before promising SSO.

## Syncing upstream

`git fetch upstream && git merge upstream/main`. The module rename touches nearly
every `.go` file, so import-path conflicts are expected on merge; resolve by
keeping our path. Do not resolve a conflict by reverting `go.mod`.
