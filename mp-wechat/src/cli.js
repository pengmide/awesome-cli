#!/usr/bin/env node

const fs = require("fs/promises");
const path = require("path");
const {JSDOM} = require("jsdom");
const {convertMarkdownFile, getThemeChoices} = require("./convert");
const {copyHtmlToClipboard} = require("./clipboard");
const {DEFAULT_IMAGE_COUNT, embedImagesInMarkdown} = require("./embed-img");

const TOKEN_URL = "https://api.weixin.qq.com/cgi-bin/token";
const UPLOAD_IMAGE_URL = "https://api.weixin.qq.com/cgi-bin/media/uploadimg";
const UPLOAD_MATERIAL_URL = "https://api.weixin.qq.com/cgi-bin/material/add_material";
const ADD_DRAFT_URL = "https://api.weixin.qq.com/cgi-bin/draft/add";

const MIME_EXTENSIONS = {
  "image/jpeg": "jpg",
  "image/jpg": "jpg",
  "image/png": "png",
  "image/gif": "gif",
};

function printUsage() {
  console.log(`Usage:
  mp-wechat-cli <input.md> [--out <output.html>] [--theme <theme>] [--copy-html]
  mp-wechat-cli embed-img <img-or-img-folder> <file.md> [--count <n>] [--out <output.md>]
  mp-wechat-cli publish-draft <input.html> --appid <appid> --secret <secret> [--out <output.html>] [--title <title>] [--author <author>] [--digest <digest>]

Options:
  --out <path>     Output HTML path. Defaults to mp-wechat/out/<name>.wechat.html
  --theme <name>   Theme name. Available: ${getThemeChoices().join(", ")}
  --copy-html      Also write the rendered HTML into macOS clipboard as text/html
  --count <n>      Number of sorted directory images to embed. Defaults to ${DEFAULT_IMAGE_COUNT}
  --appid <appid>  WeChat Official Account AppID for publish-draft
  --secret <key>   WeChat Official Account AppSecret for publish-draft
  --title <title>  Draft title. Defaults to the first h1 in the HTML
  --author <name>  Draft author
  --digest <text>  Draft digest. Defaults to plain text extracted from the HTML
  --help           Show this help message
`);
}

function parseArgs(argv) {
  const args = argv.slice(2);
  const options = {
    inputPath: "",
    imageInputPath: "",
    outPath: "",
    count: DEFAULT_IMAGE_COUNT,
    theme: "green",
    copyHtml: false,
    command: "render",
    appid: "",
    secret: "",
    title: "",
    author: "",
    digest: "",
  };

  if (args[0] === "publish-draft") {
    options.command = "publish-draft";
    args.shift();
  } else if (args[0] === "embed-img") {
    options.command = "embed-img";
    args.shift();
  }

  for (let i = 0; i < args.length; i += 1) {
    const arg = args[i];
    if (arg === "--help" || arg === "-h") {
      options.help = true;
      return options;
    }
    if (arg === "--out") {
      if (!args[i + 1]) {
        throw new Error("Missing value for --out");
      }
      options.outPath = args[i + 1];
      i += 1;
      continue;
    }
    if (arg === "--theme") {
      if (!args[i + 1]) {
        throw new Error("Missing value for --theme");
      }
      options.theme = args[i + 1];
      i += 1;
      continue;
    }
    if (arg === "--count") {
      if (!args[i + 1]) {
        throw new Error("Missing value for --count");
      }
      options.count = args[i + 1];
      i += 1;
      continue;
    }
    if (arg === "--appid") {
      if (!args[i + 1]) {
        throw new Error("Missing value for --appid");
      }
      options.appid = args[i + 1];
      i += 1;
      continue;
    }
    if (arg === "--secret") {
      if (!args[i + 1]) {
        throw new Error("Missing value for --secret");
      }
      options.secret = args[i + 1];
      i += 1;
      continue;
    }
    if (arg === "--title") {
      if (!args[i + 1]) {
        throw new Error("Missing value for --title");
      }
      options.title = args[i + 1];
      i += 1;
      continue;
    }
    if (arg === "--author") {
      if (!args[i + 1]) {
        throw new Error("Missing value for --author");
      }
      options.author = args[i + 1];
      i += 1;
      continue;
    }
    if (arg === "--digest") {
      if (!args[i + 1]) {
        throw new Error("Missing value for --digest");
      }
      options.digest = args[i + 1];
      i += 1;
      continue;
    }
    if (arg === "--copy-html") {
      options.copyHtml = true;
      continue;
    }
    if (options.command === "embed-img" && !options.imageInputPath) {
      options.imageInputPath = arg;
      continue;
    }
    if (!options.inputPath) {
      options.inputPath = arg;
      continue;
    }
    throw new Error(`Unexpected argument: ${arg}`);
  }

  return options;
}

function buildDefaultPublishedPath(inputPath) {
  const parsed = path.parse(inputPath);
  const base = parsed.name.replace(/\.wechat$/i, "");
  return path.join(parsed.dir, `${base}.published.html`);
}

function parseDataImage(src, index) {
  const match = /^data:(image\/[a-zA-Z0-9.+-]+);base64,([\s\S]+)$/.exec(src || "");
  if (!match) {
    throw new Error(`Invalid data image at index ${index}`);
  }

  const mime = match[1].toLowerCase();
  const ext = MIME_EXTENSIONS[mime];
  if (!ext) {
    throw new Error(`Unsupported data image type "${mime}" at index ${index}`);
  }

  const bytes = Buffer.from(match[2].replace(/\s+/g, ""), "base64");
  if (bytes.length === 0) {
    throw new Error(`Empty data image at index ${index}`);
  }

  return {
    mime,
    ext,
    bytes,
    filename: `inline-${index + 1}.${ext}`,
  };
}

function normalizeText(value) {
  return (value || "").replace(/\s+/g, " ").trim();
}

function extractTitle(document, fallbackPath) {
  const h1 = document.querySelector("h1");
  const title = normalizeText(h1 ? h1.textContent : "");
  if (title) {
    return title.slice(0, 64);
  }
  return path.parse(fallbackPath).name.slice(0, 64);
}

function extractDigest(document) {
  const text = normalizeText(document.body ? document.body.textContent : document.textContent);
  if (!text) {
    return "";
  }
  return Array.from(text).slice(0, 120).join("");
}

async function readJsonResponse(response, action) {
  const text = await response.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch (error) {
    throw new Error(`${action} failed: non-JSON response (${response.status}): ${text.slice(0, 300)}`);
  }

  const errcode = data.errcode || 0;
  if (!response.ok || errcode !== 0) {
    const errmsg = data.errmsg || response.statusText || "unknown error";
    throw new Error(`${action} failed: errcode=${errcode}, errmsg=${errmsg}`);
  }
  return data;
}

async function getAccessToken({appid, secret}) {
  const url = new URL(TOKEN_URL);
  url.searchParams.set("grant_type", "client_credential");
  url.searchParams.set("appid", appid);
  url.searchParams.set("secret", secret);

  const response = await fetch(url);
  const data = await readJsonResponse(response, "get access_token");
  if (!data.access_token) {
    throw new Error("get access_token failed: missing access_token in response");
  }
  return data.access_token;
}

async function uploadInlineImage(accessToken, image) {
  const form = new FormData();
  form.append("media", new Blob([image.bytes], {type: image.mime}), image.filename);

  const url = new URL(UPLOAD_IMAGE_URL);
  url.searchParams.set("access_token", accessToken);

  const response = await fetch(url, {
    method: "POST",
    body: form,
  });
  const data = await readJsonResponse(response, `upload inline image ${image.filename}`);
  if (!data.url) {
    throw new Error(`upload inline image ${image.filename} failed: missing url in response`);
  }
  return data.url;
}

async function uploadCoverMaterial(accessToken, image) {
  const form = new FormData();
  form.append("media", new Blob([image.bytes], {type: image.mime}), image.filename);

  const url = new URL(UPLOAD_MATERIAL_URL);
  url.searchParams.set("access_token", accessToken);
  url.searchParams.set("type", "image");

  const response = await fetch(url, {
    method: "POST",
    body: form,
  });
  const data = await readJsonResponse(response, `upload cover material ${image.filename}`);
  if (!data.media_id) {
    throw new Error(`upload cover material ${image.filename} failed: missing media_id in response`);
  }
  return data.media_id;
}

async function createDraft({accessToken, title, author, digest, html, thumbMediaId}) {
  const url = new URL(ADD_DRAFT_URL);
  url.searchParams.set("access_token", accessToken);

  const body = {
    articles: [
      {
        title,
        author: author || "",
        digest: digest || "",
        content: html,
        thumb_media_id: thumbMediaId,
        show_cover_pic: 1,
      },
    ],
  };

  const response = await fetch(url, {
    method: "POST",
    headers: {"Content-Type": "application/json; charset=utf-8"},
    body: JSON.stringify(body),
  });
  const data = await readJsonResponse(response, "create draft");
  if (!data.media_id) {
    throw new Error("create draft failed: missing media_id in response");
  }
  return data.media_id;
}

async function preparePublishedHtml({html, inputPath, outPath, uploadImage}) {
  const dom = new JSDOM(`<body>${html}</body>`);
  const {document} = dom.window;
  const dataImageNodes = Array.from(document.querySelectorAll("img"))
    .filter((node) => (node.getAttribute("src") || "").startsWith("data:image/"));

  if (dataImageNodes.length === 0) {
    throw new Error("No data:image inline image found. The default cover strategy needs the first inline image.");
  }

  const images = dataImageNodes.map((node, index) => parseDataImage(node.getAttribute("src"), index));
  const uploadedUrls = [];

  for (let i = 0; i < dataImageNodes.length; i += 1) {
    const wechatUrl = await uploadImage(images[i]);
    dataImageNodes[i].setAttribute("src", wechatUrl);
    uploadedUrls.push(wechatUrl);
  }

  const processedHtml = document.body.innerHTML.trim();
  const finalOutPath = outPath || buildDefaultPublishedPath(inputPath);
  await fs.mkdir(path.dirname(finalOutPath), {recursive: true});
  await fs.writeFile(finalOutPath, `${processedHtml}\n`, "utf8");

  return {
    html: `${processedHtml}\n`,
    outputPath: finalOutPath,
    title: extractTitle(document, inputPath),
    digest: extractDigest(document),
    firstImage: images[0],
    uploadedImageUrls: uploadedUrls,
  };
}

async function publishHtmlDraft({
  inputPath,
  outPath,
  appid,
  secret,
  title,
  author,
  digest,
}) {
  if (!appid) {
    throw new Error("Missing required --appid");
  }
  if (!secret) {
    throw new Error("Missing required --secret");
  }

  const html = await fs.readFile(inputPath, "utf8");
  const accessToken = await getAccessToken({appid, secret});

  let firstImageForCover = null;
  const prepared = await preparePublishedHtml({
    html,
    inputPath,
    outPath,
    uploadImage: async (image) => {
      if (!firstImageForCover) {
        firstImageForCover = image;
      }
      return uploadInlineImage(accessToken, image);
    },
  });

  const coverMediaId = await uploadCoverMaterial(accessToken, firstImageForCover || prepared.firstImage);
  const draftMediaId = await createDraft({
    accessToken,
    title: title || prepared.title,
    author: author || "",
    digest: digest || prepared.digest,
    html: prepared.html,
    thumbMediaId: coverMediaId,
  });

  return {
    outputPath: prepared.outputPath,
    uploadedImageCount: prepared.uploadedImageUrls.length,
    coverMediaId,
    draftMediaId,
    title: title || prepared.title,
    digest: digest || prepared.digest,
  };
}

async function main() {
  try {
    const options = parseArgs(process.argv);
    if (options.help || !options.inputPath) {
      printUsage();
      process.exit(options.help ? 0 : 1);
    }

    const cwd = process.cwd();
    const inputPath = path.resolve(cwd, options.inputPath);
    const outPath = options.outPath ? path.resolve(cwd, options.outPath) : "";

    if (options.command === "embed-img") {
      if (!options.imageInputPath) {
        throw new Error("Missing required image file or image folder");
      }
      const imageInputPath = path.resolve(cwd, options.imageInputPath);
      const result = await embedImagesInMarkdown({
        imageInputPath,
        markdownPath: inputPath,
        outPath,
        count: options.count,
      });

      console.log(`Generated: ${result.outputPath}`);
      console.log(`Embedded images: ${result.embeddedImageCount}`);
      result.imagePaths.forEach((imagePath) => {
        console.log(`  - ${imagePath}`);
      });
      return;
    }

    if (options.command === "publish-draft") {
      const result = await publishHtmlDraft({
        inputPath,
        outPath,
        appid: options.appid,
        secret: options.secret,
        title: options.title,
        author: options.author,
        digest: options.digest,
      });

      console.log(`Processed HTML: ${result.outputPath}`);
      console.log(`Uploaded inline images: ${result.uploadedImageCount}`);
      console.log(`Cover media_id: ${result.coverMediaId}`);
      console.log(`Draft media_id: ${result.draftMediaId}`);
      console.log(`Title: ${result.title}`);
      return;
    }

    const result = await convertMarkdownFile({
      inputPath,
      outPath,
      theme: options.theme,
    });

    let clipboardCopied = false;
    let clipboardError = null;

    if (options.copyHtml) {
      try {
        await copyHtmlToClipboard(result.html);
        clipboardCopied = true;
      } catch (error) {
        clipboardError = error;
      }
    }

    console.log(`Generated: ${result.outputPath}`);
    console.log(`Theme: ${result.themeLabel}`);
    if (clipboardCopied) {
      console.log("Copied HTML to macOS clipboard.");
    }
    if (result.warnings.length > 0) {
      console.log("Warnings:");
      result.warnings.forEach((warning) => {
        console.log(`  - ${warning}`);
      });
    }
    if (clipboardError) {
      throw clipboardError;
    }
  } catch (error) {
    console.error(error.message);
    process.exit(1);
  }
}

main();
