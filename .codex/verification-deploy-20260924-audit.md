# verification-deploy-20260924-audit

Date: 2026-09-24 (UTC). Branch `custom` HEAD deployed: `d3a09c856765749015f992c7b3256fb08dc7e08b`
(short `d3a09c856765`), version `0.3.6`, build_time `20260924T125813Z`.
Prior commit: merge of `custom-fix-audit-20260924` (`d3a09c85`) into `custom` (ff from `69b3360e`).

## Build

- `powershell -File .codex/build-release.ps1` from exact HEAD → `BUILD_OK`,
  image `deeix-chat:d3a09c856765` linux/amd64.
- `manifest.env` completed manually with `image_manifest_id`, `sandbox_image`,
  `sandbox_base_image` (script only writes base fields; sandbox still `7c6c0838115f`).
- `SHA256SUMS` regenerated LF covering tar + manifest.env + remote-deploy.sh.
- New `release/d3a09c856765/remote-deploy.sh` derived from hardened `69b3360ea1a1`
  script: `EXPECTED_OLD_APP_IMAGE=deeix-chat:69b3360ea1a1`, sandbox pins unchanged.

## image_id incident (first attempt rolled back automatically)

- First deploy failed at the post-load image gate: local containerd
  `docker image inspect .Id` = manifest digest `6f5dc3…`, VPS classic storage
  `.Id` = config digest `9c7d08…`. ERR trap auto-rolled back to `69b3360ea1a1`
  (verified running, restarts=0); no user impact.
- Fix: `EXPECTED_APP_IMAGE_ID`/`manifest image_id` set to config digest
  `sha256:9c7d084f27d3832b8ea9a6428f912cd5e9d2f79cf04a639e59d62b677343c21f`
  (taken from tar `manifest.json` Config + VPS inspect, all three agree).
- `build-release.ps1` now derives `image_id` from the saved tar manifest so this
  cannot recur (plus trailing newline on manifest.env).

## Local smoke (sqlite/memory)

- `docker run` with `DATABASE_DRIVER=sqlite`, `CACHE_DRIVER=memory`, test secrets,
  `PUBLIC_*_BASE_URL=https://ai.3efs.com` → `readyz ok`,
  `/api/v1/version` commit exact match `d3a09c85…`. Container removed afterwards.

## Deploy (second attempt)

- Uploaded tar/manifest/SHA256SUMS/script to `/opt/deeix-chat/releases/d3a09c856765/`,
  remote `sha256sum -c SHA256SUMS` 3/3 OK.
- `remote-deploy.sh deploy` → `DEPLOYMENT_OK`,
  backup `/opt/backups/deeix-chat-d3a09c856765-20260924T130707Z`
  (DB dump + restore list + old image tarball + volume metadata + storage note).

## Production verification

- `deeix-chat-app`: image `deeix-chat:d3a09c856765`, `restarts=0`, running.
- Local `http://127.0.0.1:8088/api/v1/version` and public
  `https://ai.3efs.com/api/v1/version` both report commit `d3a09c85…`, `0.3.6`.
- `deeix-sandbox-mcp:7c6c0838115f` untouched, `restarts=0`.
- Rollback (if ever needed):
  `ssh cutclass-vps 'bash /opt/deeix-chat/releases/d3a09c856765/remote-deploy.sh rollback'`.

## Open follow-ups

- `custom` is ahead of `origin/custom` (release practice: no push; kept local).
- VPS disk ~5.7G free; old `deeix-chat` images pre-`69b3360ea1a1` are rollback-safe
  to prune (rollback uses backup tarballs); pruned 20260924, kept current + previous.
- `conversation` package + 5 channel sqlite tests still require a CGO toolchain
  (ran green: llm/settings/config + new audit regression tests; `go vet` clean).
