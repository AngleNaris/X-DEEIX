#!/usr/bin/env bash
set -Eeuo pipefail

SHA="69b3360ea1a15cc592a4292019ad4a4817450ab4"
SHORT_SHA="69b3360ea1a1"
VERSION="0.3.6"
APP_IMAGE="deeix-chat:${SHORT_SHA}"
EXPECTED_OLD_APP_IMAGE="deeix-chat:03071e525cb6"
EXPECTED_SANDBOX_IMAGE="deeix-sandbox-mcp:7c6c0838115f"
EXPECTED_SANDBOX_BASE_IMAGE="deeix-sandbox-base:7c6c0838115f"
EXPECTED_APP_IMAGE_ID="sha256:edaa2cce3e1081de2846edaf0c475eafc094e66880e28588db3ecde3c6c24945"
APP_DIR="/opt/deeix-chat"
MCP_DIR="/opt/deeix-mcp"
RELEASE_DIR="${APP_DIR}/releases/${SHORT_SHA}"
APP_COMPOSE="${APP_DIR}/docker-compose.yml"
MCP_COMPOSE="${MCP_DIR}/docker-compose.yml"
APP_OVERRIDE="${APP_DIR}/docker-compose.override.yml"
MCP_OVERRIDE="${MCP_DIR}/docker-compose.override.yml"
 APP_CONTAINER="deeix-chat-app"
 SANDBOX_CONTAINER="deeix-sandbox-mcp"
 # PG 容器名含 1Panel 随机后缀，允许环境覆盖；不存在时按前缀自动发现，避免重建后写死失效。
 PG_CONTAINER="${PG_CONTAINER:-1Panel-postgresql-tRkp}"
 if ! docker inspect "$PG_CONTAINER" >/dev/null 2>&1; then
   PG_CONTAINER="$(docker ps --format '{{.Names}}' 2>/dev/null | grep -E '^1Panel-postgresql-' | head -n1)"
 fi
 PG_DATABASE="deeix_chat"
 # 公网版本门禁默认域名可覆盖；公网不可达时可用 SKIP_PUBLIC_GATE=1 转为警告（需人工确认反代状态）。
 PUBLIC_VERSION_URL="${PUBLIC_VERSION_URL:-https://ai.3efs.com/api/v1/version}"

wait_url() {
  local url="$1"
  for _ in $(seq 1 60); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

 # 回滚时若备份中无 override（如首次部署），必须显式 pin 旧镜像，禁止删文件后退到 compose 默认值（可能是上游 latest）。
 restore_app_override() {
   if [[ -f "${BACKUP_DIR}/app.override.yml" ]]; then
     cp -a "${BACKUP_DIR}/app.override.yml" "${APP_OVERRIDE}.next"
     mv -f "${APP_OVERRIDE}.next" "$APP_OVERRIDE"
   elif [[ -n "${OLD_APP_IMAGE:-}" ]]; then
     printf 'services:\n  app:\n    image: %s\n' "$OLD_APP_IMAGE" > "${APP_OVERRIDE}.next"
     chmod 600 "${APP_OVERRIDE}.next"
     mv -f "${APP_OVERRIDE}.next" "$APP_OVERRIDE"
   else
     echo "rollback: OLD_APP_IMAGE is empty, refusing to drop override" >&2
     return 1
   fi
 }
 
 rollback() {
   local status=0
   set +e
   trap - ERR
   echo "ROLLBACK_START"
   # shellcheck disable=SC1090
   source "${BACKUP_DIR}/rollback.env" || status=1
   docker image inspect "$OLD_APP_IMAGE" >/dev/null 2>&1 || gzip -dc "${BACKUP_DIR}/rollback-app-image.tar.gz" | docker load || status=1
   restore_app_override || status=1
   (cd "$APP_DIR" && docker compose up -d --no-deps app) || status=1
   wait_url "http://127.0.0.1:8088/readyz" || status=1
   wait_url "http://127.0.0.1:8081/healthz" || status=1
   [[ "$(docker inspect "$APP_CONTAINER" --format '{{.Config.Image}}')" == "$OLD_APP_IMAGE" ]] || status=1
   [[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{.Config.Image}}')" == "$EXPECTED_SANDBOX_IMAGE" ]] || status=1
   [[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^SANDBOX_BASE_IMAGE=//p' | head -n1)" == "$EXPECTED_SANDBOX_BASE_IMAGE" ]] || status=1
   if [[ "$status" == "0" ]]; then
     echo "ROLLBACK_DONE"
   else
     echo "ROLLBACK_FAILED" >&2
   fi
   return "$status"
 }
 
 if [[ "${1:-deploy}" == "rollback" ]]; then
   # deployment-result.env 缺失时（如切换后 SSH 断连）回退到同 SHORT_SHA 最新备份目录，保证回滚入口可达。
   if [[ -f "${RELEASE_DIR}/deployment-result.env" ]]; then
     # shellcheck disable=SC1090
     source "${RELEASE_DIR}/deployment-result.env"
   else
     echo "warning: deployment-result.env missing, discovering latest backup" >&2
     BACKUP_DIR="$(ls -1d /opt/backups/deeix-chat-${SHORT_SHA}-* 2>/dev/null | sort | tail -n1)"
     [[ -n "$BACKUP_DIR" && -f "${BACKUP_DIR}/rollback.env" ]] || { echo "rollback: no usable backup found" >&2; exit 1; }
     export BACKUP_DIR
     export ROLLBACK_WITHOUT_RESULT_ENV=1
   fi
   rollback
   exit $?
 fi

[[ "$(id -u)" == "0" ]]
cd "$RELEASE_DIR"
umask 077

grep -Fx "commit=${SHA}" manifest.env >/dev/null
grep -Fx "version=${VERSION}" manifest.env >/dev/null
grep -Fx "image=${APP_IMAGE}" manifest.env >/dev/null
grep -Fx "image_id=${EXPECTED_APP_IMAGE_ID}" manifest.env >/dev/null
grep -Fx "sandbox_image=${EXPECTED_SANDBOX_IMAGE}" manifest.env >/dev/null
grep -Fx "sandbox_base_image=${EXPECTED_SANDBOX_BASE_IMAGE}" manifest.env >/dev/null
grep -Fx "platform=linux/amd64" manifest.env >/dev/null
sha256sum -c SHA256SUMS

[[ "$(docker inspect "$APP_CONTAINER" --format '{{.State.Status}}')" == "running" ]]
[[ "$(docker inspect "$APP_CONTAINER" --format '{{.Config.Image}}')" == "$EXPECTED_OLD_APP_IMAGE" ]]
[[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{.State.Status}}')" == "running" ]]
[[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{.Config.Image}}')" == "$EXPECTED_SANDBOX_IMAGE" ]]
[[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^SANDBOX_BASE_IMAGE=//p' | head -n1)" == "$EXPECTED_SANDBOX_BASE_IMAGE" ]]
curl -fsS "http://127.0.0.1:8088/healthz" >/dev/null
curl -fsS "http://127.0.0.1:8088/readyz" >/dev/null
curl -fsS "http://127.0.0.1:8088/api/v1/version" >/dev/null
curl -fsS "http://127.0.0.1:8081/healthz" >/dev/null

available_bytes="$(df -PB1 /opt | awk 'NR==2 {print $4}')"
(( available_bytes >= 3221225472 ))

for dir in "${MCP_DIR}/shared" "${MCP_DIR}/imports"; do
  [[ -d "$dir" ]]
  permissions="$(stat -c '%a' "$dir")"
  group_digit="${permissions: -2:1}"
  other_digit="${permissions: -1}"
  [[ ! "$group_digit" =~ [2367] ]]
  [[ ! "$other_digit" =~ [2367] ]]
done

app_hmac="$(docker inspect "$APP_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^SANDBOX_META_HMAC_KEY=//p' | head -n1)"
sandbox_hmac="$(docker inspect "$SANDBOX_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^SANDBOX_META_HMAC_KEY=//p' | head -n1)"
[[ -n "$app_hmac" && "$app_hmac" == "$sandbox_hmac" ]]
unset app_hmac sandbox_hmac

PG_USER="$(docker inspect "$PG_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^POSTGRES_USER=//p' | head -n1)"
[[ -n "$PG_USER" ]]
docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" -d "$PG_DATABASE" >/dev/null

BACKUP_DIR="/opt/backups/deeix-chat-${SHORT_SHA}-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -m 700 "$BACKUP_DIR"

OLD_APP_IMAGE="$(docker inspect "$APP_CONTAINER" --format '{{.Config.Image}}')"
printf 'OLD_APP_IMAGE=%q\n' "$OLD_APP_IMAGE" > "${BACKUP_DIR}/rollback.env"

cp -a "$APP_COMPOSE" "${BACKUP_DIR}/app.compose.yml"
cp -a "$MCP_COMPOSE" "${BACKUP_DIR}/mcp.compose.yml"
[[ ! -f "$APP_OVERRIDE" ]] || cp -a "$APP_OVERRIDE" "${BACKUP_DIR}/app.override.yml"
[[ ! -f "$MCP_OVERRIDE" ]] || cp -a "$MCP_OVERRIDE" "${BACKUP_DIR}/mcp.override.yml"
[[ ! -f "${APP_DIR}/config.yaml" ]] || cp -a "${APP_DIR}/config.yaml" "${BACKUP_DIR}/config.yaml"
[[ ! -f "${APP_DIR}/.env.release" ]] || cp -a "${APP_DIR}/.env.release" "${BACKUP_DIR}/app.env.release"
[[ ! -f "${MCP_DIR}/.env" ]] || cp -a "${MCP_DIR}/.env" "${BACKUP_DIR}/mcp.env"
[[ ! -f "${MCP_DIR}/.env.release" ]] || cp -a "${MCP_DIR}/.env.release" "${BACKUP_DIR}/mcp.env.release"
(cd "$APP_DIR" && docker compose config) > "${BACKUP_DIR}/app.compose.rendered.yml"
(cd "$MCP_DIR" && docker compose config) > "${BACKUP_DIR}/mcp.compose.rendered.yml"
docker inspect "$APP_CONTAINER" > "${BACKUP_DIR}/app.inspect.json"
docker inspect "$SANDBOX_CONTAINER" > "${BACKUP_DIR}/sandbox.inspect.json"
docker image inspect "$OLD_APP_IMAGE" > "${BACKUP_DIR}/rollback-app-image.inspect.json"
docker image save "$OLD_APP_IMAGE" | gzip -1 > "${BACKUP_DIR}/rollback-app-image.tar.gz"

docker exec "$PG_CONTAINER" psql -v ON_ERROR_STOP=1 -At -U "$PG_USER" -d "$PG_DATABASE" -c \
  "SELECT current_database(), current_user, pg_size_pretty(pg_database_size(current_database()));
   SELECT extname || '=' || extversion FROM pg_extension WHERE extname = 'vector';
   SELECT table_name || '.' || column_name || '=' || udt_name
     FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name IN ('file_chunks','chat_message_chunks','user_memories') AND column_name = 'embedding'
    ORDER BY table_name;" > "${BACKUP_DIR}/database-preflight.txt"
 docker exec "$PG_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DATABASE" -Fc > "${BACKUP_DIR}/deeix_chat.dump"
 [[ -s "${BACKUP_DIR}/deeix_chat.dump" ]]
 docker exec -i "$PG_CONTAINER" pg_restore --list < "${BACKUP_DIR}/deeix_chat.dump" > "${BACKUP_DIR}/deeix_chat.restore.list"
 [[ -s "${BACKUP_DIR}/deeix_chat.restore.list" ]]
 # 文件卷快照：DB dump 不含 /app/storage（本地卷 deeix-chat-app-storage）。此处记录卷元数据与用量，
 # 文件级恢复需另行快照宿主机卷；回滚镜像后若新版本已重编码文件，需人工比对。
 docker volume inspect deeix-chat-app-storage > "${BACKUP_DIR}/app-storage.volume.json" 2>/dev/null || echo '{"warning":"app-storage volume inspect failed"}' > "${BACKUP_DIR}/app-storage.volume.json"
 docker system df -v > "${BACKUP_DIR}/docker-disk.txt" 2>/dev/null || true
cat > "${BACKUP_DIR}/STORAGE_BACKUP_NOTE.txt" <<'NOTE'
DB dump 不含应用文件卷（/app/storage -> deeix-chat-app-storage）。
本次备份仅含卷元数据（app-storage.volume.json），无文件级恢复点。
需要文件级回滚时，请在切换前对宿主机卷做独立快照（含 webp 缩略图与生成文件）。
NOTE
find "$BACKUP_DIR" -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > "${BACKUP_DIR}/SHA256SUMS"
sha256sum -c "${BACKUP_DIR}/SHA256SUMS"

docker load -i "deeix-chat-${SHORT_SHA}-linux-amd64.tar" > load-app.log
[[ "$(docker image inspect "$APP_IMAGE" --format '{{.Id}}')" == "$EXPECTED_APP_IMAGE_ID" ]]

cat > "${APP_OVERRIDE}.next" <<EOF
services:
  app:
    image: ${APP_IMAGE}
EOF
chmod 600 "${APP_OVERRIDE}.next"
docker compose -f "$APP_COMPOSE" -f "${APP_OVERRIDE}.next" config --images | grep -Fx "$APP_IMAGE" >/dev/null

SANDBOX_CONTAINER_ID_BEFORE="$(docker inspect "$SANDBOX_CONTAINER" --format '{{.Id}}')"
SANDBOX_STARTED_AT_BEFORE="$(docker inspect "$SANDBOX_CONTAINER" --format '{{.State.StartedAt}}')"
SANDBOX_RESTARTS_BEFORE="$(docker inspect "$SANDBOX_CONTAINER" --format '{{.RestartCount}}')"
CUTOVER_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
cutover_started=1
trap 'status=$?; trap - ERR; if [[ ${cutover_started:-0} == 1 ]]; then rollback; fi; exit "$status"' ERR

mv -f "${APP_OVERRIDE}.next" "$APP_OVERRIDE"
(cd "$APP_DIR" && docker compose up -d --no-deps app)
wait_url "http://127.0.0.1:8088/readyz"
curl -fsS "http://127.0.0.1:8088/healthz" > healthz.json
curl -fsS "http://127.0.0.1:8088/api/v1/version" > version.json
jq -e --arg sha "$SHA" --arg version "$VERSION" '.commit == $sha and .version == $version' version.json >/dev/null
[[ "$(docker inspect "$APP_CONTAINER" --format '{{.Config.Image}}')" == "$APP_IMAGE" ]]
[[ "$(docker inspect "$APP_CONTAINER" --format '{{.RestartCount}}')" == "0" ]]

[[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{.Id}}')" == "$SANDBOX_CONTAINER_ID_BEFORE" ]]
[[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{.State.StartedAt}}')" == "$SANDBOX_STARTED_AT_BEFORE" ]]
[[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{.RestartCount}}')" == "$SANDBOX_RESTARTS_BEFORE" ]]
[[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{.Config.Image}}')" == "$EXPECTED_SANDBOX_IMAGE" ]]
[[ "$(docker inspect "$SANDBOX_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^SANDBOX_BASE_IMAGE=//p' | head -n1)" == "$EXPECTED_SANDBOX_BASE_IMAGE" ]]
curl -fsS "http://127.0.0.1:8081/healthz" >/dev/null

app_hmac="$(docker inspect "$APP_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^SANDBOX_META_HMAC_KEY=//p' | head -n1)"
sandbox_hmac="$(docker inspect "$SANDBOX_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^SANDBOX_META_HMAC_KEY=//p' | head -n1)"
[[ -n "$app_hmac" && "$app_hmac" == "$sandbox_hmac" ]]
unset app_hmac sandbox_hmac

 if [[ "${SKIP_PUBLIC_GATE:-0}" == "1" ]]; then
   echo "warning: SKIP_PUBLIC_GATE=1, public version gate skipped (verify reverse proxy manually)" >&2
 else
   curl -fsS "$PUBLIC_VERSION_URL" > public-version.json
   jq -e --arg sha "$SHA" --arg version "$VERSION" '.commit == $sha and .version == $version' public-version.json >/dev/null
 fi

if docker logs --since "$CUTOVER_TIME" "$APP_CONTAINER" 2>&1 | grep -Eai 'fatal|panic|segmentation|unhandled|(^|[^a-z])error([^a-z]|$)'; then
  exit 1
fi

printf 'BACKUP_DIR=%q\nCUTOVER_TIME=%q\nSANDBOX_CONTAINER_ID=%q\nSANDBOX_STARTED_AT=%q\nSANDBOX_RESTARTS=%q\n' \
  "$BACKUP_DIR" "$CUTOVER_TIME" "$SANDBOX_CONTAINER_ID_BEFORE" "$SANDBOX_STARTED_AT_BEFORE" "$SANDBOX_RESTARTS_BEFORE" > deployment-result.env
chmod 600 deployment-result.env
cutover_started=0
trap - ERR
echo "DEPLOYMENT_OK"
echo "BACKUP_DIR=${BACKUP_DIR}"
