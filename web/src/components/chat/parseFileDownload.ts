export interface FileDownloadInfo {
  filename: string;
  storageFilename: string;
  fileSize: number;
}

export function tryParseFileDownload(result: string): FileDownloadInfo | null {
  try {
    const parsed = JSON.parse(result);
    if (
      typeof parsed.displayFilename === 'string' &&
      typeof parsed.storageFilename === 'string' &&
      typeof parsed.fileSize === 'number'
    ) {
      return {
        filename: parsed.displayFilename,
        storageFilename: parsed.storageFilename,
        fileSize: parsed.fileSize,
      };
    }
  } catch {
    // not valid JSON or not a file download result
  }
  return null;
}

export function extractFileDownloadsFromContent(content: string): { text: string; downloads: FileDownloadInfo[] } {
  const downloads: FileDownloadInfo[] = [];
  // Look for JSON blobs that match our file download format
  const jsonRegex = /\{[^{}]*"displayFilename"[^{}]*\}/g;
  let match;
  while ((match = jsonRegex.exec(content)) !== null) {
    const info = tryParseFileDownload(match[0]);
    if (info) {
      downloads.push(info);
    }
  }
  // Remove the JSON blobs from content for cleaner display
  const text = content.replace(jsonRegex, '').replace(/\n{3,}/g, '\n\n').trim();
  return { text, downloads };
}
