import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import FileDownloadCard from './FileDownloadCard';

describe('FileDownloadCard', () => {
  it('renders filename and size', () => {
    render(<FileDownloadCard filename="report.docx" storageFilename="docx-abc.docx" fileSize={15360} />);
    expect(screen.getByText('report.docx')).toBeInTheDocument();
    expect(screen.getByText('15.0 KB')).toBeInTheDocument();
  });

  it('has correct download link', () => {
    render(<FileDownloadCard filename="report.docx" storageFilename="docx-abc.docx" fileSize={1024} />);
    const link = screen.getByText('Download') as HTMLAnchorElement;
    expect(link.href).toContain('/api/generated-files/docx-abc.docx');
    expect(link.download).toBe('report.docx');
  });

  it('shows correct icon for file types', () => {
    const { rerender } = render(<FileDownloadCard filename="a.docx" storageFilename="x" fileSize={1} />);
    expect(screen.getByText('📄')).toBeInTheDocument();

    rerender(<FileDownloadCard filename="b.pptx" storageFilename="x" fileSize={1} />);
    expect(screen.getByText('📊')).toBeInTheDocument();

    rerender(<FileDownloadCard filename="c.xlsx" storageFilename="x" fileSize={1} />);
    expect(screen.getByText('📈')).toBeInTheDocument();
  });
});
