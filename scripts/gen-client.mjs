import { spawn } from "node:child_process";
import { once } from "node:events";
import { resolve } from "node:path";
import process from "node:process";
import { setTimeout as delay } from "node:timers/promises";

const root = resolve(import.meta.dirname, "..");
const backendDir = resolve(root, "apps/backend");
const port = process.env.PORT ?? "8080";
const schemaUrl =
  process.env.API_SCHEMA_URL ?? `http://localhost:${port}/openapi.json`;
const startsLocalBackend = process.env.API_SCHEMA_URL === undefined;

async function endpointAvailable() {
  try {
    const response = await fetch(schemaUrl, {
      signal: AbortSignal.timeout(1000),
    });
    return response.ok;
  } catch {
    return false;
  }
}

function startBackend() {
  const backend = spawn("go", ["run", "./cmd"], {
    cwd: backendDir,
    env: { ...process.env, PORT: port },
    stdio: "inherit",
    detached: process.platform !== "win32",
  });

  return backend;
}

async function waitForBackend(backend) {
  const deadline = Date.now() + 60_000;

  while (Date.now() < deadline) {
    if (backend.exitCode !== null) {
      throw new Error(`Backend exited before becoming ready (code ${backend.exitCode}).`);
    }

    if (await endpointAvailable()) return;
    await delay(250);
  }

  throw new Error(`Backend did not serve ${schemaUrl} within 60 seconds.`);
}

async function generateTypes() {
  const command = process.platform === "win32" ? "pnpm.cmd" : "pnpm";
  const child = spawn(command, ["--filter", "web", "gen:client"], {
    cwd: root,
    env: { ...process.env, API_SCHEMA_URL: schemaUrl },
    stdio: "inherit",
    shell: process.platform === "win32",
  });

  const [code, signal] = await once(child, "exit");
  if (code !== 0) {
    throw new Error(
      `Type generation failed (${signal ?? `exit code ${code ?? "unknown"}`}).`,
    );
  }
}

async function stopBackend(backend) {
  if (!backend?.pid || backend.exitCode !== null) return;

  const exited = once(backend, "exit").catch(() => {});
  try {
    if (process.platform === "win32") {
      backend.kill("SIGTERM");
    } else {
      process.kill(-backend.pid, "SIGTERM");
    }
  } catch {
    return;
  }

  await Promise.race([exited, delay(5000)]);
  if (backend.exitCode !== null) return;

  try {
    if (process.platform === "win32") {
      backend.kill("SIGKILL");
    } else {
      process.kill(-backend.pid, "SIGKILL");
    }
  } catch {
    // The process may have exited between the check and the signal.
  }
}

let backend;
try {
  if (startsLocalBackend && !(await endpointAvailable())) {
    console.log(`Starting backend on port ${port}...`);
    backend = startBackend();
    await waitForBackend(backend);
  } else if (startsLocalBackend) {
    console.log("Backend is already running; leaving it running after generation.");
  }

  await generateTypes();
} finally {
  if (backend) {
    console.log("Stopping the backend started for type generation...");
    await stopBackend(backend);
  }
}
