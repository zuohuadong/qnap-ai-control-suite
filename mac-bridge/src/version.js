import { existsSync, readFileSync } from "node:fs";

const repositoryVersion = new URL("../../VERSION", import.meta.url);
export const version = existsSync(repositoryVersion)
  ? readFileSync(repositoryVersion, "utf8").trim()
  : JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8")).version;
