import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../../");

function source(relativePath) {
  return fs.readFileSync(path.join(webRoot, relativePath), "utf8");
}

function assertRoute(route, relativePath, sourceName) {
  const absolutePath = path.join(webRoot, relativePath);
  assert.equal(
    fs.existsSync(absolutePath),
    true,
    `${sourceName} points to ${route}, but the route module is missing: ${relativePath}`,
  );
}

function hrefsFrom(relativePath) {
  return [...source(relativePath).matchAll(/href:\s*["']([^"']+)["']/g)].map((match) => match[1]);
}

const projectRoutes = {
  "/recent": "app/(app)/(project)/recent/page.tsx",
  "/canvas": "app/(app)/(project)/canvas/page.tsx",
  "/files": "app/(app)/(project)/files/page.tsx",
  "/doc-cards": "app/(app)/(project)/doc-cards/page.tsx",
  "/artifacts": "app/(app)/(project)/artifacts/page.tsx",
  "/knowledges": "app/(app)/(project)/knowledges/page.tsx",
  "/skills-prompt": "app/(app)/(project)/skills-prompt/page.tsx",
  "/agent-groups": "app/(app)/(project)/agent-groups/page.tsx",
};

for (const href of hrefsFrom("features/layouts/model/navigation-items.ts")) {
  assertRoute(href, projectRoutes[href], "main navigation");
}

for (const href of hrefsFrom("features/settings/components/settings-sidebar.tsx")) {
  assertRoute(href, `app/(app)/(project)/setting${href}/page.tsx`, "settings navigation");
}

for (const href of hrefsFrom("features/admin/model/admin-sections.ts")) {
  assertRoute(href, `app/(app)/(project)/admin${href}/page.tsx`, "admin navigation");
}

assertRoute("/share", "app/(app)/share/page.tsx", "share navigation");
assertRoute("/share/artifact", "app/(app)/share/artifact/page.tsx", "artifact share link");
assertRoute("/share/file", "app/(app)/share/file/page.tsx", "file share link");

assert.match(source("shared/api/artifacts.ts"), /\/share\/artifact/);
assert.match(source("shared/api/file.ts"), /\/share\/file/);

console.log("navigation route smoke checks passed");
