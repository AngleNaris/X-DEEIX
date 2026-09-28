import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const messagesRoot = path.dirname(fileURLToPath(import.meta.url));

function load(locale) {
  return JSON.parse(fs.readFileSync(path.join(messagesRoot, "messages", locale, "chat.json"), "utf8"));
}

function leafPaths(value, prefix = "") {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return [prefix];
  }
  return Object.entries(value).flatMap(([key, child]) => leafPaths(child, prefix ? `${prefix}.${key}` : key));
}

const englishMarkdownKeys = leafPaths(load("en-US").markdown).sort();
const chineseMarkdownKeys = leafPaths(load("zh-CN").markdown).sort();

assert.deepEqual(
  chineseMarkdownKeys,
  englishMarkdownKeys,
  "zh-CN chat.markdown must keep the same translation-key shape as en-US",
);

console.log("chat markdown translation contract passed");
