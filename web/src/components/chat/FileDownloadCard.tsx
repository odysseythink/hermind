import styles from './FileDownloadCard.module.css';

export interface FileDownloadCardProps {
  filename: string;
  storageFilename: string;
  fileSize: number;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function getIcon(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase();
  switch (ext) {
    case 'docx': return '📄';
    case 'pptx': return '📊';
    case 'pdf': return '📑';
    case 'xlsx': return '📈';
    default: return '📝';
  }
}

export default function FileDownloadCard({ filename, storageFilename, fileSize }: FileDownloadCardProps) {
  const handleDownload = () => {
    window.open(`/api/generated-files/${storageFilename}`, '_blank');
  };

  return (
    <div className={styles.card}>
      <span className={styles.icon}>{getIcon(filename)}</span>
      <div className={styles.info}>
        <span className={styles.filename}>{filename}</span>
        <span className={styles.size}>{formatSize(fileSize)}</span>
      </div>
      <button className={styles.button} onClick={handleDownload}>
        Download
      </button>
    </div>
  );
}
