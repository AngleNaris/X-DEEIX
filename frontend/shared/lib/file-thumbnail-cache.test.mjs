import assert from "node:assert/strict";
import test from "node:test";

import { clearFileThumbnailCache, loadCachedFileThumbnail } from "./file-thumbnail-cache.ts";

test.beforeEach(() => clearFileThumbnailCache());

test("deduplicates concurrent thumbnail loads", async () => {
  let calls = 0;
  const load = async () => {
    calls += 1;
    await Promise.resolve();
    return new Blob(["image"], { type: "image/png" });
  };
  const file = { fileID: "file-1", sha256: "hash-1" };

  const first = loadCachedFileThumbnail(file, load);
  const second = loadCachedFileThumbnail(file, load);

  assert.strictEqual(first, second);
  assert.strictEqual(await first, await second);
  assert.equal(calls, 1);
});

test("reloads when the file version changes", async () => {
  let calls = 0;
  const load = async () => {
    calls += 1;
    return new Blob([String(calls)], { type: "image/png" });
  };

  await loadCachedFileThumbnail({ fileID: "file-1", sha256: "hash-1" }, load);
  await loadCachedFileThumbnail({ fileID: "file-1", sha256: "hash-2" }, load);

  assert.equal(calls, 2);
});

test("keeps the cached content across metadata-only updates", async () => {
  let calls = 0;
  const load = async () => {
    calls += 1;
    return new Blob(["image"], { type: "image/png" });
  };

  await loadCachedFileThumbnail({ fileID: "file-1", sha256: "hash-1", updatedAt: "v1" }, load);
  await loadCachedFileThumbnail({ fileID: "file-1", sha256: "hash-1", updatedAt: "v2" }, load);

  assert.equal(calls, 1);
});

test("allows retry after a failed thumbnail load", async () => {
  let calls = 0;
  const load = async () => {
    calls += 1;
    if (calls === 1) {
      throw new Error("temporary failure");
    }
    return new Blob(["image"], { type: "image/png" });
  };
  const file = { fileID: "file-1", sha256: "hash-1" };

  await assert.rejects(loadCachedFileThumbnail(file, load), /temporary failure/);
  await loadCachedFileThumbnail(file, load);

  assert.equal(calls, 2);
});
