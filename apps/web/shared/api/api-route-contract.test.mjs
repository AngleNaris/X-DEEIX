import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const repoRoot = path.resolve(webRoot, "../..");

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
}

const contracts = [
  {
    name: "active generation stream",
    frontend: "apps/web/shared/api/conversation.ts",
    frontendNeedle: "/api/v1/conversation-runs/stream",
    backend: "backend/internal/transport/http/conversation/router.go",
    backendNeedle: 'authRequired.GET("/conversation-runs/stream"',
  },
  {
    name: "artifact list",
    frontend: "apps/web/shared/api/artifacts.ts",
    frontendNeedle: "/api/v1/artifacts?page=",
    backend: "backend/internal/transport/http/artifact/router.go",
    backendNeedle: 'authRequired.GET("/artifacts"',
  },
  {
    name: "document cards",
    frontend: "apps/web/shared/api/doc-cards.ts",
    frontendNeedle: "/api/v1/doc-cards",
    backend: "backend/internal/transport/http/doccard/router.go",
    backendNeedle: 'authRequired.GET("/doc-cards"',
  },
  {
    name: "files",
    frontend: "apps/web/shared/api/file.ts",
    frontendNeedle: "/api/v1/files",
    backend: "backend/internal/transport/http/conversation/router.go",
    backendNeedle: 'authRequired.GET("/files"',
  },
  {
    name: "credentials",
    frontend: "apps/web/shared/api/credentials.ts",
    frontendNeedle: "/api/v1/credentials",
    backend: "backend/internal/transport/http/credentials/module.go",
    backendNeedle: 'authRequired.GET("/credentials"',
  },
  {
    name: "agent groups",
    frontend: "apps/web/shared/api/agent-groups.ts",
    frontendNeedle: "/api/v1/conversation-agent-groups",
    backend: "backend/internal/transport/http/agentgroup/router.go",
    backendNeedle: 'authRequired.GET("/conversation-agent-groups"',
  },
  {
    name: "knowledge bases",
    frontend: "apps/web/shared/api/knowledge-bases.ts",
    frontendNeedle: "/api/v1/knowledge-bases",
    backend: "backend/internal/transport/http/knowledgebase/router.go",
    backendNeedle: 'group.GET("/knowledge-bases"',
  },
  {
    name: "skills",
    frontend: "apps/web/shared/api/skills.ts",
    frontendNeedle: "/api/v1/skills",
    backend: "backend/internal/transport/http/skill/router.go",
    backendNeedle: 'authRequired.GET("/skills"',
  },
  {
    name: "public artifact share",
    frontend: "apps/web/shared/api/artifacts.ts",
    frontendNeedle: "/api/v1/shared-artifacts/",
    backend: "backend/internal/transport/http/artifact/router.go",
    backendNeedle: 'public.GET("/shared-artifacts/:share_id"',
  },
  {
    name: "public file share",
    frontend: "apps/web/shared/api/file.ts",
    frontendNeedle: "/api/v1/shared-files/",
    backend: "backend/internal/transport/http/conversation/router.go",
    backendNeedle: 'public.GET("/shared-files/:share_id"',
  },
];

for (const contract of contracts) {
  const frontendSource = read(contract.frontend);
  const backendSource = read(contract.backend);
  assert.ok(
    frontendSource.includes(contract.frontendNeedle),
    `${contract.name}: frontend request path is missing from ${contract.frontend}`,
  );
  assert.ok(
    backendSource.includes(contract.backendNeedle),
    `${contract.name}: backend route is missing from ${contract.backend}`,
  );
}

console.log(`API route contract checks passed (${contracts.length})`);
