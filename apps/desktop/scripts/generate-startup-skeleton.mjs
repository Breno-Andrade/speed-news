// Writes the startup skeleton (src/StartupSkeleton.tsx rendered to HTML) into index.html, so the
// first frame of a freshly created popover already shows the app's layout. Run it after changing
// any skeleton component; a test fails while index.html is out of date.
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const desktopDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const indexHtmlPath = path.join(desktopDirectory, "index.html");

const viteServer = await createServer({ root: desktopDirectory, server: { middlewareMode: true }, appType: "custom", logLevel: "error" });
try {
  const { withStartupSkeleton } = await viteServer.ssrLoadModule("/src/startupSkeletonMarkup.tsx");
  writeFileSync(indexHtmlPath, withStartupSkeleton(readFileSync(indexHtmlPath, "utf8")));
  console.log("startup skeleton written to index.html");
} finally {
  await viteServer.close();
}
