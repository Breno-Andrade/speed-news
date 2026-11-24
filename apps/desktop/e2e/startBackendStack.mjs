// Starts the fixture server (fake RSS/Jolpica/f1-circuits/Open-Meteo) and the real Go backend
// pointed at it, with a brand new database. Used as a Playwright webServer: it stays in the
// foreground and stops both processes when Playwright stops it.
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { E2E_API_TOKEN, E2E_BACKEND_PORT, E2E_FIXTURE_PORT, E2E_UI_ORIGIN } from "./stackPorts.mjs";

const apiDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../api");
const runDirectory = mkdtempSync(path.join(tmpdir(), "speed-news-e2e-"));
const sourcesFile = path.join(runDirectory, "sources.yaml");
const fixtureBaseUrl = `http://127.0.0.1:${E2E_FIXTURE_PORT}`;
const childProcesses = [];

function startProcess(processLabel, commandArguments, extraEnvironment, readyMarker) {
  return new Promise((resolveReady, rejectReady) => {
    const childProcess = spawn("go", commandArguments, { cwd: apiDirectory, env: { ...process.env, ...extraEnvironment } });
    childProcesses.push(childProcess);
    const failIfExits = (exitCode) => rejectReady(new Error(`${processLabel} exited early with code ${exitCode}`));
    childProcess.once("exit", failIfExits);
    childProcess.stdout.on("data", (outputChunk) => {
      process.stdout.write(`[${processLabel}] ${outputChunk}`);
      if (outputChunk.toString().includes(readyMarker)) {
        childProcess.off("exit", failIfExits);
        resolveReady();
      }
    });
    childProcess.stderr.on("data", (outputChunk) => process.stderr.write(`[${processLabel}] ${outputChunk}`));
  });
}

function stopEverything() {
  for (const childProcess of childProcesses) childProcess.kill("SIGTERM");
  rmSync(runDirectory, { recursive: true, force: true });
}
process.on("SIGTERM", () => {
  stopEverything();
  process.exit(0);
});
process.on("SIGINT", () => {
  stopEverything();
  process.exit(0);
});

await startProcess(
  "fixtures",
  ["run", "./cmd/fixture-server", "--port", String(E2E_FIXTURE_PORT), "--write-sources-file", sourcesFile],
  {},
  "FIXTURE_SERVER_READY",
);
await startProcess(
  "backend",
  [
    "run", "./cmd/news-api",
    "--port", String(E2E_BACKEND_PORT),
    "--data-dir", runDirectory,
    "--allowed-origins", E2E_UI_ORIGIN,
    "--sources-file", sourcesFile,
    "--jolpica-base-url", `${fixtureBaseUrl}/jolpica`,
    "--circuits-base-url", `${fixtureBaseUrl}/circuits`,
    "--openmeteo-base-url", `${fixtureBaseUrl}/openmeteo`,
  ],
  { SPEED_NEWS_API_TOKEN: E2E_API_TOKEN },
  "SPEED_NEWS_API_READY",
);
console.log("E2E backend stack ready");
// Keep running until Playwright stops the web server.
await new Promise(() => undefined);
