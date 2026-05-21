#!/usr/bin/env node

const { createDocx } = require('../lib/docx/create');
const { createPptx } = require('../lib/pptx/create');

async function main() {
  const chunks = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk);
  }
  const input = Buffer.concat(chunks).toString('utf8').trim();

  let params;
  try {
    params = JSON.parse(input);
  } catch (err) {
    console.error('Invalid JSON on stdin:', err.message);
    process.exit(1);
  }

  const { type, outputDir } = params;

  if (!type || typeof type !== 'string') {
    console.error('Missing or invalid field: type');
    process.exit(1);
  }

  if (!outputDir || typeof outputDir !== 'string') {
    console.error('Missing or invalid field: outputDir');
    process.exit(1);
  }

  let filePath;
  try {
    if (type === 'docx') {
      filePath = await createDocx(params);
    } else if (type === 'pptx') {
      filePath = await createPptx(params);
    } else {
      console.error(`Unsupported type: ${type}`);
      process.exit(1);
    }
  } catch (err) {
    console.error(err.message || err);
    process.exit(1);
  }

  console.log(filePath);
}

main().catch(err => {
  console.error(err.message || err);
  process.exit(1);
});
