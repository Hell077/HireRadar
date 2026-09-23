import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import openapiTS, { astToString } from "openapi-typescript";

const schemaUrl =
  process.env.API_SCHEMA_URL ?? "http://localhost:8080/openapi.json";
const outputPath = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../apps/web/lib/api/generated.ts",
);

const ast = await openapiTS(schemaUrl);
await mkdir(dirname(outputPath), { recursive: true });
await writeFile(outputPath, astToString(ast));
console.log(`Generated API types from ${schemaUrl} -> ${outputPath}`);
