import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const messagesRoot = path.dirname(fileURLToPath(import.meta.url));
const localeRoot = path.join(messagesRoot, "messages");

function load(locale, fileName) {
  return JSON.parse(fs.readFileSync(path.join(localeRoot, locale, fileName), "utf8"));
}

function leafPaths(value, prefix = "") {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return [prefix];
  }
  return Object.entries(value).flatMap(([key, child]) => leafPaths(child, prefix ? `${prefix}.${key}` : key));
}

const englishFiles = fs.readdirSync(path.join(localeRoot, "en-US")).filter((fileName) => fileName.endsWith(".json")).sort();
for (const fileName of englishFiles) {
  const englishPath = leafPaths(load("en-US", fileName));
  const chineseFilePath = path.join(localeRoot, "zh-CN", fileName);
  assert.equal(fs.existsSync(chineseFilePath), true, `zh-CN is missing message file ${fileName}`);
  const chinesePathSet = new Set(leafPaths(load("zh-CN", fileName)));
  const missing = englishPath.filter((key) => !chinesePathSet.has(key));
  assert.deepEqual(missing, [], `zh-CN/${fileName} is missing translation keys`);
}

const englishMarkdownKeys = leafPaths(load("en-US", "chat.json").markdown).sort();
const chineseMarkdownKeys = leafPaths(load("zh-CN", "chat.json").markdown).sort();
assert.deepEqual(
  chineseMarkdownKeys,
  englishMarkdownKeys,
  "zh-CN chat.markdown must keep the same translation-key shape as en-US",
);

console.log(`translation contracts passed (${englishFiles.length} files)`);
