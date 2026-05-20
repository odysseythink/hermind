const path = require('path');
const fs = require('fs');
const PptxGenJS = require('pptxgenjs');
const { saveGeneratedFile } = require('../manager');
const { getTheme } = require('./utils');

async function createPptx(params) {
  const {
    filename = 'presentation.pptx',
    title = 'Presentation',
    theme = 'corporate',
    sections = [],
    outputDir,
  } = params;

  if (!outputDir) {
    throw new Error('outputDir is required');
  }

  const displayFilename = filename.endsWith('.pptx') ? filename : `${filename}.pptx`;

  const pptx = new PptxGenJS();
  pptx.title = title;
  pptx.author = 'Hermind';

  const themeColors = getTheme(theme);

  pptx.defineSlideMaster({
    title: 'MASTER_SLIDE',
    background: { color: themeColors.background },
    objects: [
      { rect: { x: 0, y: 0, w: '100%', h: 0.5, fill: themeColors.header } },
    ],
  });

  // Title slide
  const titleSlide = pptx.addSlide();
  titleSlide.background = { color: themeColors.background };
  titleSlide.addText(title, {
    x: 1, y: 2, w: '80%', h: 1.5,
    fontSize: 44, bold: true, color: themeColors.title,
    align: 'center',
  });

  // Content slides
  for (const section of sections) {
    const slide = pptx.addSlide({ masterName: 'MASTER_SLIDE' });
    slide.addText(section.title, {
      x: 0.5, y: 0.7, w: '90%', h: 0.8,
      fontSize: 32, bold: true, color: themeColors.heading,
    });

    if (section.keyPoints && section.keyPoints.length > 0) {
      const bullets = section.keyPoints.map((pt, i) => ({
        text: pt,
        options: { fontSize: 18, color: themeColors.body, bullet: true },
      }));
      slide.addText(bullets, {
        x: 0.5, y: 1.6, w: '90%', h: 4,
        lineSpacing: 32,
      });
    }

    if (section.instructions) {
      slide.addText(section.instructions, {
        x: 0.5, y: 4.5, w: '90%', h: 1,
        fontSize: 14, color: themeColors.muted, italics: true,
      });
    }
  }

  // Thank you slide
  const endSlide = pptx.addSlide();
  endSlide.background = { color: themeColors.background };
  endSlide.addText('Thank You', {
    x: 1, y: 2.5, w: '80%', h: 1,
    fontSize: 40, bold: true, color: themeColors.title,
    align: 'center',
  });

  const buffer = await pptx.write({ outputType: 'nodebuffer' });

  const saved = saveGeneratedFile(outputDir, 'pptx', 'pptx', buffer, displayFilename);
  return saved.storagePath;
}

module.exports = { createPptx };
