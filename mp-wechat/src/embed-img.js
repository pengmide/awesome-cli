const fs = require("fs/promises");
const path = require("path");

const DEFAULT_IMAGE_COUNT = 5;
const SUPPORTED_IMAGE_TYPES = {
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".png": "image/png",
  ".gif": "image/gif",
};

function isSupportedImagePath(filePath) {
  return Boolean(SUPPORTED_IMAGE_TYPES[path.extname(filePath).toLowerCase()]);
}

function buildDefaultEmbeddedPath(markdownPath) {
  const parsed = path.parse(markdownPath);
  return path.join(parsed.dir, `${parsed.name}.embedded${parsed.ext || ".md"}`);
}

function parseImageCount(value) {
  if (value === "" || value === undefined || value === null) {
    return DEFAULT_IMAGE_COUNT;
  }

  const count = Number(value);
  if (!Number.isInteger(count) || count < 1) {
    throw new Error("--count must be a positive integer");
  }
  return count;
}

function detectNewline(text) {
  return text.includes("\r\n") ? "\r\n" : "\n";
}

async function collectImagePaths(imageInputPath, count) {
  let stat;
  try {
    stat = await fs.stat(imageInputPath);
  } catch (error) {
    throw new Error(`Image path not found: ${imageInputPath}`);
  }

  if (stat.isFile()) {
    if (!isSupportedImagePath(imageInputPath)) {
      throw new Error(`Unsupported image type: ${imageInputPath}`);
    }
    return [imageInputPath];
  }

  if (!stat.isDirectory()) {
    throw new Error(`Image path must be a file or directory: ${imageInputPath}`);
  }

  const entries = await fs.readdir(imageInputPath, {withFileTypes: true});
  const imagePaths = entries
    .filter((entry) => entry.isFile() && isSupportedImagePath(entry.name))
    .map((entry) => path.join(imageInputPath, entry.name))
    .sort((a, b) => path.basename(a).localeCompare(path.basename(b), "zh-Hans-CN"));

  if (imagePaths.length === 0) {
    throw new Error(`No supported images found in directory: ${imageInputPath}`);
  }

  return imagePaths.slice(0, count);
}

async function imagePathToMarkdown(imagePath) {
  const ext = path.extname(imagePath).toLowerCase();
  const mime = SUPPORTED_IMAGE_TYPES[ext];
  if (!mime) {
    throw new Error(`Unsupported image type: ${imagePath}`);
  }

  const encoded = (await fs.readFile(imagePath)).toString("base64");
  if (!encoded) {
    throw new Error(`Empty image file: ${imagePath}`);
  }

  return `![](data:${mime};base64,${encoded})`;
}

function splitParagraphBlocks(markdownText) {
  const normalized = markdownText.replace(/\r\n/g, "\n");
  const blocks = normalized.split(/\n{2,}/);
  const nonEmptyBlocks = blocks
    .map((block, index) => ({block, index}))
    .filter((item) => item.block.trim());

  return {
    blocks,
    nonEmptyIndexes: nonEmptyBlocks.map((item) => item.index),
  };
}

function chooseInsertionIndexes(nonEmptyIndexes, imageCount) {
  if (nonEmptyIndexes.length === 0) {
    return Array.from({length: imageCount}, () => -1);
  }

  const selected = [];
  const used = new Set();
  const distributedCount = Math.min(imageCount, nonEmptyIndexes.length);
  for (let i = 0; i < distributedCount; i += 1) {
    const position = Math.floor(((i + 1) * nonEmptyIndexes.length) / (distributedCount + 1));
    let blockIndex = nonEmptyIndexes[Math.min(position, nonEmptyIndexes.length - 1)];

    while (used.has(blockIndex)) {
      const nextPosition = nonEmptyIndexes.indexOf(blockIndex) + 1;
      if (nextPosition >= nonEmptyIndexes.length) {
        blockIndex = nonEmptyIndexes.find((index) => !used.has(index)) ?? -1;
        break;
      }
      blockIndex = nonEmptyIndexes[nextPosition];
    }

    selected.push(blockIndex);
    if (blockIndex >= 0) {
      used.add(blockIndex);
    }
  }

  while (selected.length < imageCount) {
    selected.push(-1);
  }
  return selected;
}

function insertImageBlocks(markdownText, imageMarkdownBlocks) {
  const newline = detectNewline(markdownText);
  const {blocks, nonEmptyIndexes} = splitParagraphBlocks(markdownText);
  const insertionIndexes = chooseInsertionIndexes(nonEmptyIndexes, imageMarkdownBlocks.length);
  const imagesByBlock = new Map();
  const appendImages = [];

  imageMarkdownBlocks.forEach((imageBlock, imageIndex) => {
    const blockIndex = insertionIndexes[imageIndex];
    if (blockIndex < 0) {
      appendImages.push(imageBlock);
      return;
    }
    const images = imagesByBlock.get(blockIndex) || [];
    images.push(imageBlock);
    imagesByBlock.set(blockIndex, images);
  });

  const outputBlocks = [];
  blocks.forEach((block, index) => {
    if (block.trim() || outputBlocks.length > 0 || index < blocks.length - 1) {
      outputBlocks.push(block);
    }
    const images = imagesByBlock.get(index) || [];
    images.forEach((imageBlock) => outputBlocks.push(imageBlock));
  });
  appendImages.forEach((imageBlock) => outputBlocks.push(imageBlock));

  const embeddedText = outputBlocks.join("\n\n").replace(/\n/g, newline);
  return embeddedText.endsWith(newline) ? embeddedText : `${embeddedText}${newline}`;
}

async function embedImagesInMarkdown({imageInputPath, markdownPath, outPath = "", count = DEFAULT_IMAGE_COUNT}) {
  const imageCount = parseImageCount(count);
  const inputStat = await fs.stat(markdownPath).catch(() => null);
  if (!inputStat || !inputStat.isFile()) {
    throw new Error(`Markdown file not found: ${markdownPath}`);
  }

  const finalOutPath = outPath || buildDefaultEmbeddedPath(markdownPath);
  if (path.resolve(finalOutPath) === path.resolve(markdownPath)) {
    throw new Error("Output path must be different from the input Markdown path");
  }

  const imagePaths = await collectImagePaths(imageInputPath, imageCount);
  const markdownText = await fs.readFile(markdownPath, "utf8");
  const imageMarkdownBlocks = [];
  for (const imagePath of imagePaths) {
    imageMarkdownBlocks.push(await imagePathToMarkdown(imagePath));
  }

  const embeddedText = insertImageBlocks(markdownText, imageMarkdownBlocks);
  await fs.mkdir(path.dirname(finalOutPath), {recursive: true});
  await fs.writeFile(finalOutPath, embeddedText, "utf8");

  return {
    outputPath: finalOutPath,
    embeddedImageCount: imageMarkdownBlocks.length,
    imagePaths,
  };
}

module.exports = {
  DEFAULT_IMAGE_COUNT,
  buildDefaultEmbeddedPath,
  embedImagesInMarkdown,
  insertImageBlocks,
  parseImageCount,
};
