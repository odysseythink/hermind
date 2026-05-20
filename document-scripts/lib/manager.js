const path = require('path');
const fs = require('fs');
const { v4: uuidv4 } = require('uuid');

function generateFilename(fileType, extension) {
  const fileUUID = uuidv4();
  return `${fileType}-${fileUUID}.${extension}`;
}

function ensureDir(dir) {
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
}

function saveGeneratedFile(outputDir, fileType, extension, buffer, displayFilename) {
  ensureDir(outputDir);
  const filename = generateFilename(fileType, extension);
  const storagePath = path.join(outputDir, filename);
  fs.writeFileSync(storagePath, buffer);
  return {
    filename,
    displayFilename,
    fileSize: buffer.length,
    storagePath,
  };
}

module.exports = {
  generateFilename,
  ensureDir,
  saveGeneratedFile,
};
