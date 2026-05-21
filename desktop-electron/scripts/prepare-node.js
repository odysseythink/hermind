const fs = require('fs');
const path = require('path');
const https = require('https');
const { execSync } = require('child_process');

const NODE_VERSION = process.env.NODE_VERSION || '20.17.0';
const NODE_ZIP_URL = `https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-win-x64.zip`;
const ROOT_DIR = path.resolve(__dirname, '..');
const RESOURCES_DIR = path.join(ROOT_DIR, 'resources');
const TEMP_DIR = path.join(ROOT_DIR, '.tmp-node');
const NODE_EXE_PATH = path.join(RESOURCES_DIR, 'node.exe');
const DOC_SCRIPTS_SRC = path.resolve(ROOT_DIR, '..', 'document-scripts');
const DOC_SCRIPTS_DEST = path.join(RESOURCES_DIR, 'document-scripts');

function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);

    const request = https.get(url, { headers: { 'User-Agent': 'node' } }, (response) => {
      if (response.statusCode === 301 || response.statusCode === 302) {
        file.close();
        fs.unlinkSync(dest);
        downloadFile(response.headers.location, dest).then(resolve).catch(reject);
        return;
      }

      if (response.statusCode !== 200) {
        file.close();
        fs.unlinkSync(dest);
        reject(new Error(`Failed to download ${url}: status ${response.statusCode}`));
        return;
      }

      response.pipe(file);
      file.on('finish', () => {
        file.close(resolve);
      });
    });

    request.on('error', (err) => {
      fs.unlinkSync(dest);
      reject(err);
    });

    file.on('error', (err) => {
      fs.unlinkSync(dest);
      reject(err);
    });
  });
}

async function main() {
  if (fs.existsSync(NODE_EXE_PATH)) {
    console.log(`Node.exe already exists at ${NODE_EXE_PATH}, skipping download.`);
  } else {
    if (!fs.existsSync(RESOURCES_DIR)) {
      fs.mkdirSync(RESOURCES_DIR, { recursive: true });
    }

    if (!fs.existsSync(TEMP_DIR)) {
      fs.mkdirSync(TEMP_DIR, { recursive: true });
    }

    const zipPath = path.join(TEMP_DIR, 'node.zip');
    console.log(`Downloading Node.js v${NODE_VERSION}...`);
    await downloadFile(NODE_ZIP_URL, zipPath);
    console.log(`Downloaded to ${zipPath}`);

    console.log('Extracting node.exe...');
    execSync(
      `tar -xf '${zipPath}' -C '${TEMP_DIR}'`
    );

    const extractedNodePath = path.join(TEMP_DIR, `node-v${NODE_VERSION}-win-x64`, 'node.exe');
    if (!fs.existsSync(extractedNodePath)) {
      throw new Error(`Extracted node.exe not found at ${extractedNodePath}`);
    }

    fs.copyFileSync(extractedNodePath, NODE_EXE_PATH);
    console.log(`Copied node.exe to ${NODE_EXE_PATH}`);
  }

  if (!fs.existsSync(DOC_SCRIPTS_SRC)) {
    throw new Error(`document-scripts source not found at ${DOC_SCRIPTS_SRC}`);
  }

  if (fs.existsSync(DOC_SCRIPTS_DEST)) {
    console.log(`Removing existing ${DOC_SCRIPTS_DEST}...`);
    fs.rmSync(DOC_SCRIPTS_DEST, { recursive: true, force: true });
  }

  console.log(`Copying document-scripts to ${DOC_SCRIPTS_DEST}...`);
  fs.cpSync(DOC_SCRIPTS_SRC, DOC_SCRIPTS_DEST, { recursive: true });

  console.log('Installing document-scripts dependencies...');
  execSync('npm install --production', {
    cwd: DOC_SCRIPTS_DEST,
    stdio: 'inherit'
  });

  const entryPoint = path.join(DOC_SCRIPTS_DEST, 'bin', 'generate-doc.js');
  if (!fs.existsSync(entryPoint)) {
    throw new Error(`Entry point not found at ${entryPoint}`);
  }
  console.log(`Verified entry point: ${entryPoint}`);

  if (fs.existsSync(TEMP_DIR)) {
    console.log(`Cleaning up ${TEMP_DIR}...`);
    fs.rmSync(TEMP_DIR, { recursive: true, force: true });
  }

  console.log('prepare-node.js completed successfully.');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
