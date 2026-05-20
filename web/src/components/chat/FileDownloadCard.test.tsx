import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import FileDownloadCard from './FileDownloadCard';

describe('FileDownloadCard', () => {
  it('renders filename and size', () => {
    render(<FileDownloadCard filename="report.docx" storageFilename="docx-abc.docx" fileSize={15360} />);
    expect(screen.getByText('report.docx')).toBeInTheDocument();
    expect(screen.getByText('15.0 KB')).toBeInTheDocument();
  });

  it('opens download on click', () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
    render(<FileDownloadCard filename="report.docx" storageFilename="docx-abc.docx" fileSize={1024} />);
    fireEvent.click(screen.getByText('Download'));
    expect(openSpy).toHaveBeenCalledWith('/api/generated-files/docx-abc.docx', '_blank');
    openSpy.mockRestore();
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
