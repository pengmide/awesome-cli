const {spawn} = require("child_process");

function copyHtmlToClipboard(html) {
  return new Promise((resolve, reject) => {
    const swiftSource = `
import AppKit
import Foundation

let html = String(data: FileHandle.standardInput.readDataToEndOfFile(), encoding: .utf8) ?? ""
let pasteboard = NSPasteboard.general
pasteboard.clearContents()
let htmlOk = pasteboard.setString(html, forType: .html)
let textOk = pasteboard.setString(html, forType: .string)

if htmlOk && textOk {
  FileHandle.standardOutput.write(Data("ok\\n".utf8))
  Foundation.exit(0)
} else {
  FileHandle.standardError.write(Data("Failed to write HTML to macOS clipboard.\\n".utf8))
  Foundation.exit(1)
}
`;

    const child = spawn("swift", ["-e", swiftSource], {
      stdio: ["pipe", "pipe", "pipe"],
    });

    let stdout = "";
    let stderr = "";

    child.stdout.on("data", (chunk) => {
      stdout += chunk.toString();
    });

    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString();
    });

    child.on("error", (error) => {
      reject(new Error(`Failed to launch swift: ${error.message}`));
    });

    child.on("close", (code) => {
      if (code === 0) {
        resolve(stdout.trim());
        return;
      }

      const message = stderr.trim() || "Failed to write HTML to macOS clipboard.";
      reject(new Error(message));
    });

    child.stdin.end(html, "utf8");
  });
}

module.exports = {
  copyHtmlToClipboard,
};
