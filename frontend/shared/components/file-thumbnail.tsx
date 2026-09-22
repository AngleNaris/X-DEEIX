"use client";

import * as React from "react";

import { cn } from "@/lib/utils";
import { fetchFileContent } from "@/shared/api/file";
import type { FileObjectDTO } from "@/shared/api/file.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { isFileReady, isImageFile, resolveFileIcon } from "@/shared/lib/file-display";
import { loadCachedFileThumbnail } from "@/shared/lib/file-thumbnail-cache";

type FileThumbnailSource = Pick<FileObjectDTO, "fileID" | "fileName" | "mimeType" | "status"> & {
  sha256?: string;
};

export async function loadFileThumbnailBlob(file: FileThumbnailSource): Promise<Blob | null> {
  if (!isImageFile(file) || !isFileReady(file.status)) {
    return null;
  }

  return loadCachedFileThumbnail(file, async () => {
    const accessToken = await resolveAccessToken();
    if (!accessToken) {
      throw new Error("Authentication required");
    }
    const result = await fetchFileContent(accessToken, file.fileID);
    const contentType = (result.contentType || result.blob.type).split(";")[0]?.trim().toLowerCase() ?? "";
    if (!contentType.startsWith("image/")) {
      throw new Error("File content is not an image");
    }
    return result.blob.type.startsWith("image/")
      ? result.blob
      : result.blob.slice(0, result.blob.size, contentType);
  });
}

function useFileThumbnailURL(file: FileThumbnailSource, enabled: boolean): string | null {
  const [objectURL, setObjectURL] = React.useState<string | null>(null);
  const { fileID, fileName, mimeType, status, sha256 } = file;

  React.useEffect(() => {
    let active = true;
    let nextObjectURL: string | null = null;
    setObjectURL(null);
    if (!enabled) {
      return;
    }

    void loadFileThumbnailBlob({ fileID, fileName, mimeType, status, sha256 })
      .then((blob) => {
        if (!active || !blob) {
          return;
        }
        nextObjectURL = URL.createObjectURL(blob);
        setObjectURL(nextObjectURL);
      })
      .catch(() => {
        // The file-type icon remains visible when the thumbnail cannot load.
      });

    return () => {
      active = false;
      if (nextObjectURL) {
        URL.revokeObjectURL(nextObjectURL);
      }
    };
  }, [enabled, fileID, fileName, mimeType, status, sha256]);

  return objectURL;
}

export function FileThumbnail({
  file,
  enabled = true,
  src,
  className,
  iconClassName,
}: {
  file: FileThumbnailSource;
  enabled?: boolean;
  src?: string;
  className?: string;
  iconClassName?: string;
}) {
  const FileIcon = resolveFileIcon(file);
  const objectURL = useFileThumbnailURL(file, enabled && !src);
  const [failedURL, setFailedURL] = React.useState<string | null>(null);
  const imageURL = src || objectURL;
  const showImage = imageURL && failedURL !== imageURL;

  return (
    <span className={cn("flex shrink-0 items-center justify-center overflow-hidden rounded-sm", className)}>
      {showImage ? (
        <img
          src={imageURL}
          alt=""
          loading="lazy"
          decoding="async"
          className="h-full w-full object-cover"
          onError={() => setFailedURL(imageURL)}
        />
      ) : (
        <FileIcon className={cn("text-muted-foreground", iconClassName)} strokeWidth={1.6} />
      )}
    </span>
  );
}
