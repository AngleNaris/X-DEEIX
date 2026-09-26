type FileThumbnailCacheSource = {
  fileID: string;
  sha256?: string;
};

const MAX_CACHE_ENTRIES = 80;
const blobCache = new Map<string, Promise<Blob>>();

function cacheKey(file: FileThumbnailCacheSource): string {
  return `${file.fileID}:${file.sha256 ?? ""}`;
}

function trimCache() {
  while (blobCache.size > MAX_CACHE_ENTRIES) {
    const oldestKey = blobCache.keys().next().value;
    if (!oldestKey) {
      return;
    }
    blobCache.delete(oldestKey);
  }
}

export function loadCachedFileThumbnail(
  file: FileThumbnailCacheSource,
  load: () => Promise<Blob>,
): Promise<Blob> {
  const key = cacheKey(file);
  const cached = blobCache.get(key);
  if (cached) {
    blobCache.delete(key);
    blobCache.set(key, cached);
    return cached;
  }

  const request = load().catch((error: unknown) => {
    if (blobCache.get(key) === request) {
      blobCache.delete(key);
    }
    throw error;
  });
  blobCache.set(key, request);
  trimCache();
  return request;
}

export function clearFileThumbnailCache() {
  blobCache.clear();
}
