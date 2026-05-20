const DOCUMENT_STYLES = {
  margins: {
    normal: { top: 1440, bottom: 1440, left: 1800, right: 1800 },
    narrow: { top: 720, bottom: 720, left: 720, right: 720 },
    wide: { top: 1440, bottom: 1440, left: 2880, right: 2880 },
  },
  themes: {
    neutral: {
      heading: '2E4057',
      accent: '048A81',
      tableHeader: 'E7E6E6',
      border: 'CCCCCC',
    },
    blue: {
      heading: '1B3A6B',
      accent: '2E86AB',
      tableHeader: 'D6E8F5',
      border: 'A8C8E8',
    },
    warm: {
      heading: '5C3317',
      accent: 'C1440E',
      tableHeader: 'F5ECD7',
      border: 'D4B896',
    },
  },
};

function getTheme(themeName) {
  return DOCUMENT_STYLES.themes[themeName] || DOCUMENT_STYLES.themes.neutral;
}

function getMargins(marginName) {
  return DOCUMENT_STYLES.margins[marginName] || DOCUMENT_STYLES.margins.normal;
}

function htmlToDocxElements(html, docx, themeColors) {
  const { Paragraph, TextRun, Table, TableCell, TableRow, WidthType, BorderStyle } = docx;
  const elements = [];

  // Simple HTML parser for common markdown-generated HTML
  const lines = html.split('\n');
  let inTable = false;
  let tableRows = [];
  let inCode = false;
  let codeContent = [];

  for (let line of lines) {
    line = line.trim();
    if (!line) continue;

    if (line.startsWith('<table')) {
      inTable = true;
      tableRows = [];
      continue;
    }
    if (line.startsWith('</table>')) {
      inTable = false;
      if (tableRows.length > 0) {
        elements.push(new Table({
          rows: tableRows,
          width: { size: 100, type: WidthType.PERCENTAGE },
        }));
      }
      continue;
    }
    if (inTable) {
      if (line.startsWith('<tr')) {
        tableRows.push([]);
      } else if (line.startsWith('</tr>')) {
        // row complete
      } else if (line.startsWith('<td') || line.startsWith('<th')) {
        const isHeader = line.startsWith('<th');
        const text = stripHtmlTags(line);
        const cell = new TableCell({
          children: [new Paragraph({
            children: [new TextRun({
              text,
              bold: isHeader,
              color: isHeader ? themeColors.heading : undefined,
            })],
          })],
          shading: isHeader ? { fill: themeColors.tableHeader } : undefined,
        });
        if (tableRows.length > 0) {
          tableRows[tableRows.length - 1].push(cell);
        }
      }
      continue;
    }

    if (line.startsWith('<pre><code')) {
      inCode = true;
      codeContent = [];
      continue;
    }
    if (line.startsWith('</code></pre>')) {
      inCode = false;
      elements.push(new Paragraph({
        spacing: { before: 200, after: 200 },
        children: [new TextRun({
          text: codeContent.join('\n'),
          font: 'Consolas',
          size: 20,
        })],
      }));
      continue;
    }
    if (inCode) {
      codeContent.push(stripHtmlTags(line));
      continue;
    }

    if (line.startsWith('<h1')) {
      elements.push(new Paragraph({
        spacing: { before: 400, after: 200 },
        children: [new TextRun({
          text: stripHtmlTags(line),
          bold: true,
          size: 36,
          color: themeColors.heading,
        })],
      }));
    } else if (line.startsWith('<h2')) {
      elements.push(new Paragraph({
        spacing: { before: 300, after: 150 },
        children: [new TextRun({
          text: stripHtmlTags(line),
          bold: true,
          size: 28,
          color: themeColors.heading,
        })],
      }));
    } else if (line.startsWith('<h3')) {
      elements.push(new Paragraph({
        spacing: { before: 200, after: 100 },
        children: [new TextRun({
          text: stripHtmlTags(line),
          bold: true,
          size: 24,
          color: themeColors.heading,
        })],
      }));
    } else if (line.startsWith('<ul') || line.startsWith('<ol')) {
      // skip list wrapper
    } else if (line.startsWith('</ul>') || line.startsWith('</ol>')) {
      // skip list wrapper
    } else if (line.startsWith('<li')) {
      elements.push(new Paragraph({
        indent: { left: 720 },
        children: [new TextRun({ text: '• ' + stripHtmlTags(line) })],
      }));
    } else if (line.startsWith('<blockquote')) {
      elements.push(new Paragraph({
        indent: { left: 720 },
        spacing: { before: 100, after: 100 },
        children: [new TextRun({
          text: stripHtmlTags(line),
          italics: true,
          color: themeColors.accent,
        })],
      }));
    } else {
      const text = stripHtmlTags(line);
      if (text) {
        elements.push(...parseInline(text, docx, themeColors));
      }
    }
  }

  return elements;
}

function stripHtmlTags(html) {
  return html.replace(/<[^>]+>/g, '').trim();
}

function parseInline(text, docx, themeColors) {
  const { Paragraph, TextRun } = docx;
  const children = [];
  const parts = text.split(/(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`)/g);

  for (const part of parts) {
    if (!part) continue;
    if (part.startsWith('**') && part.endsWith('**')) {
      children.push(new TextRun({ text: part.slice(2, -2), bold: true }));
    } else if (part.startsWith('*') && part.endsWith('*')) {
      children.push(new TextRun({ text: part.slice(1, -1), italics: true }));
    } else if (part.startsWith('`') && part.endsWith('`')) {
      children.push(new TextRun({ text: part.slice(1, -1), font: 'Consolas', size: 20 }));
    } else {
      children.push(new TextRun({ text: part }));
    }
  }

  return [new Paragraph({ children })];
}

module.exports = {
  getTheme,
  getMargins,
  htmlToDocxElements,
};
