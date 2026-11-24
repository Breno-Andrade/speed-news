// Builds the Go backend and places it where Tauri expects a sidecar:
// src-tauri/binaries/news-api-<rust target triple>[.exe]
import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const desktopDirectory = path.resolve(scriptDirectory, "..");
const backendDirectory = path.resolve(desktopDirectory, "../../apps/api");
const binariesDirectory = path.join(desktopDirectory, "src-tauri", "binaries");

const targetByPlatform = {
  "linux-x64": { goOperatingSystem: "linux", goArchitecture: "amd64", rustTargetTriple: "x86_64-unknown-linux-gnu" },
  "linux-arm64": { goOperatingSystem: "linux", goArchitecture: "arm64", rustTargetTriple: "aarch64-unknown-linux-gnu" },
  "darwin-x64": { goOperatingSystem: "darwin", goArchitecture: "amd64", rustTargetTriple: "x86_64-apple-darwin" },
  "darwin-arm64": { goOperatingSystem: "darwin", goArchitecture: "arm64", rustTargetTriple: "aarch64-apple-darwin" },
  "win32-x64": { goOperatingSystem: "windows", goArchitecture: "amd64", rustTargetTriple: "x86_64-pc-windows-msvc" },
};

const platformKey = `${process.platform}-${process.arch}`;
const buildTarget = targetByPlatform[platformKey];
if (!buildTarget) {
  console.error(`Unsupported platform for sidecar build: ${platformKey}`);
  process.exit(1);
}

const executableExtension = buildTarget.goOperatingSystem === "windows" ? ".exe" : "";
const outputPath = path.join(binariesDirectory, `news-api-${buildTarget.rustTargetTriple}${executableExtension}`);
mkdirSync(binariesDirectory, { recursive: true });

execFileSync("go", ["build", "-trimpath", "-ldflags=-s -w", "-o", outputPath, "./cmd/news-api"], {
  cwd: backendDirectory,
  stdio: "inherit",
  env: {
    ...process.env,
    GOOS: buildTarget.goOperatingSystem,
    GOARCH: buildTarget.goArchitecture,
    CGO_ENABLED: "0",
  },
});
console.log(`sidecar built: ${path.relative(desktopDirectory, outputPath)}`);
