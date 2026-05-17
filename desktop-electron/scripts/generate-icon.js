const fs = require('fs');
const path = require('path');
const sharp = require('sharp');
const toIco = require('png-to-ico').default || require('png-to-ico');
const png2icons = require('png2icons');

const resourcesDir = path.join(__dirname, '..', 'resources');
if (!fs.existsSync(resourcesDir)) {
  fs.mkdirSync(resourcesDir, { recursive: true });
}

// SVG icon design - Hermind: H + center dot + gradient
const svg = `<svg width="1024" height="1024" viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1024" y2="1024">
      <stop offset="0%" stop-color="#0f172a"/>
      <stop offset="100%" stop-color="#1e1b4b"/>
    </linearGradient>
    <linearGradient id="h" x1="256" y1="256" x2="768" y2="768">
      <stop offset="0%" stop-color="#22d3ee"/>
      <stop offset="100%" stop-color="#818cf8"/>
    </linearGradient>
  </defs>
  <rect width="1024" height="1024" rx="240" fill="url(#bg)"/>
  <path d="M320 240 L320 784 M704 240 L704 784 M320 512 L704 512" 
        stroke="url(#h)" stroke-width="100" stroke-linecap="round" fill="none"/>
  <circle cx="512" cy="512" r="60" fill="white"/>
</svg>`;

const svgPath = path.join(resourcesDir, 'icon.svg');
fs.writeFileSync(svgPath, svg);
console.log('Saved icon.svg');

async function generate() {
  const png1024Path = path.join(resourcesDir, 'icon.png');
  
  // Generate 1024x1024 PNG
  await sharp(Buffer.from(svg))
    .resize(1024, 1024)
    .png()
    .toFile(png1024Path);
  console.log('Saved icon.png (1024x1024)');

  // Generate multi-size ICO for Windows
  const sizes = [16, 32, 48, 64, 128, 256];
  const pngBuffers = await Promise.all(
    sizes.map(async (size) => {
      return await sharp(Buffer.from(svg))
        .resize(size, size)
        .png()
        .toBuffer();
    })
  );

  const icoBuffer = await toIco(pngBuffers);
  const icoPath = path.join(resourcesDir, 'icon.ico');
  fs.writeFileSync(icoPath, icoBuffer);
  console.log('Saved icon.ico (Windows)');

  // Generate ICNS for macOS
  const icnsBuffer = png2icons.createICNS(
    fs.readFileSync(png1024Path),
    png2icons.BILINEAR,
    0
  );
  if (icnsBuffer) {
    const icnsPath = path.join(resourcesDir, 'icon.icns');
    fs.writeFileSync(icnsPath, icnsBuffer);
    console.log('Saved icon.icns (macOS)');
  }

  // Generate Linux icons in build/icons
  const iconsDir = path.join(__dirname, '..', 'build', 'icons');
  if (!fs.existsSync(iconsDir)) {
    fs.mkdirSync(iconsDir, { recursive: true });
  }
  
  const linuxSizes = [16, 32, 48, 64, 128, 256, 512, 1024];
  for (const size of linuxSizes) {
    await sharp(Buffer.from(svg))
      .resize(size, size)
      .png()
      .toFile(path.join(iconsDir, `${size}x${size}.png`));
  }
  console.log('Saved Linux icons to build/icons/');
}

generate().catch(console.error);
