# Custom 分支 VPS 发布与回滚

本文只覆盖单应用实例、多用户部署。多实例调度、跨实例审批和分布式锁不在当前支持范围。

## 1. 发布前提

- 所有发布门禁按 `docs/CUSTOM_TEST_PLAN.md` 通过，报告明确区分 Passed、Blocked 和 Not Run。
- 未完成真实 canary 的 Qwen、Gemini、豆包、OCR 或其他 Provider 保持关闭。
- PostgreSQL 已完成备份和恢复命令演练；磁盘空间足够同时保存数据库备份、旧镜像和新镜像。
- 记录当前 Compose 文件、环境文件、镜像引用、运行版本和数据库版本，作为回滚基线。
- 候选必须是已提交的 exact SHA；禁止从脏工作树直接构建。

## 2. 构建不可变制品

在 Linux 或 WSL 中执行，避免 Windows checkout 的 CRLF 影响 `VERSION` 检查：

```bash
set -euo pipefail
SHA="$(git rev-parse HEAD)"
VERSION="$(git show "$SHA:VERSION" | tr -d '\r\n')"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
SHORT_SHA="$(printf '%s' "$SHA" | cut -c1-12)"
WORK="$(mktemp -d)"
OUT="$(pwd)/release/$SHORT_SHA"
IMAGE="deeix-chat:$SHORT_SHA"
SANDBOX_IMAGE="deeix-sandbox-mcp:$SHORT_SHA"
SANDBOX_BASE_IMAGE="deeix-sandbox-base:$SHORT_SHA"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$OUT"
git archive --format=tar "$SHA" | tar -x -C "$WORK"
test "$(tr -d '\r\n' < "$WORK/VERSION")" = "$VERSION"

docker buildx build \
  --platform linux/amd64 \
  --file "$WORK/Dockerfile" \
  --build-arg "GIT_COMMIT=$SHA" \
  --build-arg "BUILD_TIME=$STAMP" \
  --tag "$IMAGE" \
  --load \
  "$WORK"

docker buildx build \
  --platform linux/amd64 \
  --file "$WORK/tools/sandbox-mcp/docker/base.Dockerfile" \
  --tag "$SANDBOX_BASE_IMAGE" \
  --load \
  "$WORK/tools/sandbox-mcp/docker"

docker buildx build \
  --platform linux/amd64 \
  --file "$WORK/tools/sandbox-mcp/deploy/sandbox-mcp.Dockerfile" \
  --tag "$SANDBOX_IMAGE" \
  --load \
  "$WORK/tools/sandbox-mcp"

docker save "$IMAGE" -o "$OUT/deeix-chat-$SHORT_SHA-linux-amd64.tar"
docker save "$SANDBOX_IMAGE" "$SANDBOX_BASE_IMAGE" -o "$OUT/deeix-sandbox-$SHORT_SHA-linux-amd64.tar"
sha256sum "$OUT"/*.tar > "$OUT/SHA256SUMS"
IMAGE_ID="$(docker image inspect "$IMAGE" --format '{{.Id}}')"
SANDBOX_IMAGE_ID="$(docker image inspect "$SANDBOX_IMAGE" --format '{{.Id}}')"
SANDBOX_BASE_IMAGE_ID="$(docker image inspect "$SANDBOX_BASE_IMAGE" --format '{{.Id}}')"
printf 'commit=%s\nversion=%s\nimage=%s\nimage_id=%s\nsandbox_image=%s\nsandbox_image_id=%s\nsandbox_base_image=%s\nsandbox_base_image_id=%s\nplatform=linux/amd64\nbuild_time=%s\n' \
  "$SHA" "$VERSION" "$IMAGE" "$IMAGE_ID" "$SANDBOX_IMAGE" "$SANDBOX_IMAGE_ID" \
  "$SANDBOX_BASE_IMAGE" "$SANDBOX_BASE_IMAGE_ID" "$STAMP" > "$OUT/manifest.env"
sha256sum -c "$OUT/SHA256SUMS"
```

提交两个 `tar`、`SHA256SUMS` 和 `manifest.env` 到 VPS 后，先在 VPS 再执行一次 `sha256sum -c SHA256SUMS`。应用、Sandbox MCP 和 Sandbox Base 的目录及镜像标签都使用 SHA，不复用 `latest`。

## 3. VPS 预检与备份

以下变量由运维按实际环境设置，命令中的占位值不得原样执行：

```bash
set -euo pipefail
APP_DIR="/opt/deeix-chat"
MCP_DIR="/opt/deeix-mcp"
RELEASE_DIR="/opt/deeix-chat/releases/<short-sha>"
BACKUP_DIR="/opt/backups/deeix-chat-<short-sha>-$(date -u +%Y%m%dT%H%M%SZ)"
APP_CONTAINER="deeix-chat-app"
umask 077
mkdir -p "$BACKUP_DIR"

cd "$APP_DIR"
docker compose config > "$BACKUP_DIR/compose.rendered.yaml"
docker inspect "$APP_CONTAINER" > "$BACKUP_DIR/app.inspect.json"
docker inspect "$APP_CONTAINER" --format 'DEEIX_CHAT_IMAGE={{.Config.Image}}' > "$BACKUP_DIR/current-image.env"
docker inspect deeix-sandbox-mcp > "$BACKUP_DIR/sandbox.inspect.json"
docker inspect deeix-sandbox-mcp --format 'SANDBOX_MCP_IMAGE={{.Config.Image}}' > "$BACKUP_DIR/current-sandbox-image.env"
docker inspect deeix-sandbox-mcp --format '{{range .Config.Env}}{{println .}}{{end}}' \
  | grep '^SANDBOX_BASE_IMAGE=' >> "$BACKUP_DIR/current-sandbox-image.env"
cp -a docker-compose.yml config.yaml "$BACKUP_DIR/"
test ! -f .env.release || cp -a .env.release "$BACKUP_DIR/"
docker compose config --images > "$BACKUP_DIR/images.txt"
test ! -f "$MCP_DIR/.env.release" || cp -a "$MCP_DIR/.env.release" "$BACKUP_DIR/mcp.env.release"

# 使用当前 PostgreSQL 运维方式生成一致性备份，并立即校验文件非空。
<postgres-backup-command> > "$BACKUP_DIR/deeix_chat.sql"
test -s "$BACKUP_DIR/deeix_chat.sql"
sha256sum "$BACKUP_DIR"/* > "$BACKUP_DIR/SHA256SUMS"
```

预检还必须确认：数据库可连接、pgvector/扩展版本满足当前 Schema、旧数据可读、反向代理目标正确、`SANDBOX_META_HMAC_KEY` 两端一致、shared/imports 父目录只允许服务账号写入。

生产配置校验（`backend/internal/infra/config/config.go` 生产分支，约 844-875 行）在应用启动时强制以下条件，切换前必须逐项核对环境文件/`config.yaml`，否则新容器无法通过 readyz：

- `PUBLIC_API_BASE_URL`、`PUBLIC_WEB_BASE_URL` 必须是显式 `https` URL；
- `JWT_SECRET` 必须显式设置且长度 ≥ 16；
- `DATA_ENCRYPTION_KEY` 必须显式设置且长度 ≥ 32；
- `CORS_ALLOW_ORIGIN` 必须显式设置且不得为 `*`。

`app_storage` 是本地文件卷（容器内 `/app/storage`，`config.yaml:storage.root_dir` 指向它），保存上传与生成的文件：上方的 PostgreSQL dump 不包含这些文件，必须对文件卷另行快照（停写后 tar 打包或卷快照），否则回滚基线不完整。

## 4. 原子切换

**镜像 pin 的权威机制**：生产镜像由 `/opt/deeix-chat/docker-compose.override.yml` pin——该文件用 `image: deeix-chat:<short-sha>` 覆盖根 compose 的默认值（tag 由 §2 发布脚本的构建产物决定）。Compose 自动合并 override 文件，优先级高于任何 `--env-file` 注入的 `DEEIX_CHAT_IMAGE`。

**仲裁规则**：`docker-compose.override.yml` 与 `.env.release` 同时存在且不一致时，**以 override 为准**；切换/回滚前后都必须用 `docker compose config --images` 核对实际生效镜像。MCP 侧以 `/opt/deeix-mcp/.env`（`SANDBOX_MCP_IMAGE`/`SANDBOX_BASE_IMAGE`）为准，切换前先用 `docker inspect` 核对容器实际镜像，发现 `.env` 与实际不一致（stale）时先修正再切换。

标准切换（更新 override pin）：

```bash
set -euo pipefail
cd "$RELEASE_DIR"
sha256sum -c SHA256SUMS
docker load -i deeix-chat-<short-sha>-linux-amd64.tar
docker load -i deeix-sandbox-<short-sha>-linux-amd64.tar

cd "$APP_DIR"
cp -a docker-compose.override.yml "$BACKUP_DIR/docker-compose.override.yml.prev"
# override 文件应仅含一个 image 行；如结构不同，请手工编辑而非 sed
sed -i "s|^image:.*|image: deeix-chat:<short-sha>|" docker-compose.override.yml
docker compose config --images   # 必须显示 deeix-chat:<short-sha>，不得出现 :latest
docker compose up -d --no-deps app

cd /opt/deeix-mcp
# 沙箱栈按 /opt/deeix-mcp/.env 中 SANDBOX_MCP_IMAGE / SANDBOX_BASE_IMAGE 精确 tag 生效；
# 先用 docker inspect 核对容器实际镜像，修正 .env 后再重启
docker compose --env-file .env up -d --no-deps sandbox-mcp
```

**历史/备用方式**（线上当前未启用 `.env.release`；仅当 override 机制不可用时使用）：

```bash
set -euo pipefail
cd "$APP_DIR"
printf 'DEEIX_CHAT_IMAGE=deeix-chat:<short-sha>\n' > .env.release.next
mv .env.release.next .env.release
docker compose --env-file .env.release up -d --no-deps app

cd /opt/deeix-mcp
printf 'SANDBOX_MCP_IMAGE=deeix-sandbox-mcp:<short-sha>\nSANDBOX_BASE_IMAGE=deeix-sandbox-base:<short-sha>\n' > .env.release.next
mv .env.release.next .env.release
docker compose --env-file .env --env-file .env.release up -d --no-deps sandbox-mcp
```

单实例切换会有短暂重启窗口。迁移失败、健康失败或版本不匹配时不得继续浏览器验收，立即执行回滚。

## 5. 发布后门禁

```bash
set -euo pipefail
EXPECTED_SHA="<full-sha>"
EXPECTED_VERSION="<version>"

for attempt in $(seq 1 60); do
  curl -fsS http://127.0.0.1:8080/readyz && break
  sleep 2
done

curl -fsS http://127.0.0.1:8080/healthz
VERSION_JSON="$(curl -fsS http://127.0.0.1:8080/api/v1/version)"
printf '%s' "$VERSION_JSON" | jq -e --arg sha "$EXPECTED_SHA" --arg version "$EXPECTED_VERSION" \
  '.commit == $sha and .version == $version'
test "$(docker inspect "$APP_CONTAINER" --format '{{.RestartCount}}')" = "0"
test "$(docker inspect deeix-sandbox-mcp --format '{{.Config.Image}}')" = "deeix-sandbox-mcp:<short-sha>"
test "$(docker inspect deeix-sandbox-mcp --format '{{.RestartCount}}')" = "0"
curl -fsS http://127.0.0.1:8081/healthz
if docker logs --since 15m "$APP_CONTAINER" 2>&1 | grep -Eai 'fatal|panic|segmentation|unhandled|(^|[^a-z])error([^a-z]|$)'; then
  echo "fatal/error log gate failed" >&2
  exit 1
fi
if docker logs --since 15m deeix-sandbox-mcp 2>&1 | grep -Eai 'fatal|panic|segmentation|unhandled|(^|[^a-z])error([^a-z]|$)'; then
  echo "Sandbox fatal/error log gate failed" >&2
  exit 1
fi
```

随后通过真实域名完成登录、普通消息、管理员开关、A/B 用户隔离、凭据 CRUD、Agent Group、审批终态、制品 CRUD/分享和刷新持久化验证。仅对计划启用的 Provider 使用受控凭据执行 canary；不得把用户敏感附件作为测试材料。

## 6. 回滚

回滚 = 把 override pin 改回上一版精确 tag。发布成功后回滚基线写入 `/opt/deeix-chat/deployment-result.env`（不入库，见 `.gitignore`）；**该文件缺失时，从最近一次备份目录 `/opt/backups/deeix-chat-<short>-*/rollback.env` 手动恢复**，以其记录的镜像/版本作为 override pin 依据。

**注意：`app_storage` 文件卷没有自动恢复点。** §3 的数据库备份不包含上传/生成的文件；回滚文件卷只能使用 §3 预检生成的文件卷快照手工恢复，否则保持卷不动。

```bash
set -euo pipefail
cd "$APP_DIR"
docker compose config --images   # 记录当前生效镜像（回滚前后各核对一次）
cp -a docker-compose.override.yml "$BACKUP_DIR/docker-compose.override.yml.bak"
# override 文件应仅含一个 image 行；如结构不同，请手工编辑而非 sed
sed -i "s|^image:.*|image: deeix-chat:<prev-short-sha>|" docker-compose.override.yml
docker compose config --images   # 确认已回到上一版精确 tag
docker compose up -d --no-deps app
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8080/api/v1/version

cd /opt/deeix-mcp
# 沙箱栈回滚同样以 /opt/deeix-mcp/.env 为准（docker inspect 核对后修正）
docker compose --env-file .env up -d --no-deps sandbox-mcp
curl -fsS http://127.0.0.1:8081/healthz
```

**历史/备用方式**（`.env.release`，仅当 override 机制不可用时使用）：

```bash
set -euo pipefail
cd "$APP_DIR"
cp -a "$BACKUP_DIR/current-image.env" .env.release.next
mv .env.release.next .env.release
docker compose --env-file .env.release up -d --no-deps app
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8080/api/v1/version

cd /opt/deeix-mcp
cp -a "$BACKUP_DIR/current-sandbox-image.env" .env.release.next
mv .env.release.next .env.release
docker compose --env-file .env --env-file .env.release up -d --no-deps sandbox-mcp
curl -fsS http://127.0.0.1:8081/healthz
```

若新版本已执行不兼容的数据迁移，仅回滚镜像不够；必须先停止应用，再按已演练流程恢复数据库备份。回滚后同样检查版本、RestartCount、最近日志和真实域名登录。
