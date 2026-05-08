import { useCallback, useRef, useState } from 'react';
import styles from './AttachmentUploader.module.css';
import { apiUpload } from '../../api/client';
import { UploadResponseSchema } from '../../api/schemas';
import type { Attachment } from '../../state/chat';

interface Props {
  onAttachmentsAdd: (attachments: Attachment[]) => void;
}

export default function AttachmentUploader({ onAttachmentsAdd }: Props) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [isDragging, setIsDragging] = useState(false);

  const handleFiles = useCallback(
    async (files: FileList | null) => {
      if (!files) return;
      const newAttachments: Attachment[] = [];
      for (const file of Array.from(files)) {
        try {
          const raw = await apiUpload('/api/upload', file);
          const parsed = UploadResponseSchema.parse(raw);
          newAttachments.push({
            id: parsed.id,
            name: parsed.name,
            type: parsed.type,
            url: parsed.url,
            size: parsed.size,
          });
        } catch (e) {
          console.error('Upload failed:', e);
        }
      }
      if (newAttachments.length > 0) {
        onAttachmentsAdd(newAttachments);
      }
    },
    [onAttachmentsAdd],
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setIsDragging(false);
      handleFiles(e.dataTransfer.files);
    },
    [handleFiles],
  );

  return (
    <div
      className={`${styles.uploader} ${isDragging ? styles.dragging : ''}`}
      onDragOver={(e) => { e.preventDefault(); setIsDragging(true); }}
      onDragLeave={() => setIsDragging(false)}
      onDrop={handleDrop}
    >
      <button
        className={styles.attachBtn}
        onClick={() => inputRef.current?.click()}
        aria-label="Attach file"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M21.44 11.05l-9.19 9.19a6 6 0 01-8.49-8.49l9.19-9.19a4 4 0 015.66 5.66l-9.2 9.19a2 2 0 01-2.83-2.83l8.49-8.48" />
        </svg>
      </button>
      <input
        ref={inputRef}
        type="file"
        multiple
        className={styles.fileInput}
        onChange={(e) => handleFiles(e.target.files)}
      />
    </div>
  );
}
