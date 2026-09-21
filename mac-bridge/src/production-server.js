import http from "node:http";
import { readFileSync } from "node:fs";
import { timingSafeEqual } from "node:crypto";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { sshReadRequest } from "./ssh-read-transport.js";

const config = JSON.parse(readFileSync(process.env.QACS_SERVER_CONFIG, "utf8"));
const clientToken = readFileSync(config.clientTokenFile, "utf8").trim();
const agentToken = readFileSync(config.agentTokenFile, "utf8").trim();
if (clientToken.length < 32 || agentToken.length < 32) throw new Error("Invalid credentials");
const authenticate = req => {
  const actual = Buffer.from(req.headers.authorization || "");
  const expected = Buffer.from(`Bearer ${clientToken}`);
  return actual.length === expected.length && timingSafeEqual(actual, expected);
};
const reply = (res, status, body) => {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(body);
};
async function bodyText(req) {
  let body = "";
  req.setEncoding("utf8");
  for await (const chunk of req) {
    body += chunk;
    if (Buffer.byteLength(body) > 65536) throw new Error("Request too large");
  }
  return body;
}
let active = 0;
const relay = http.createServer(async (req, res) => {
  if (!authenticate(req)) return reply(res, 401, '{"ok":false}');
  if (active >= 4) return reply(res, 429, '{"ok":false}');
  active++;
  try {
    const result = await sshReadRequest({ ...config, agentToken }, req.method, req.url, await bodyText(req));
    reply(res, result.status, result.body);
    console.log(JSON.stringify({ event: "nas_read", method: req.method,
      route: new URL(req.url, "http://localhost").pathname, status: result.status }));
  } catch {
    reply(res, 502, '{"ok":false,"error":{"code":"nas_read_transport_failed"}}');
    console.error('{"event":"nas_read_failed"}');
  } finally { active--; }
});
await new Promise((resolve, reject) => {
  relay.once("error", reject);
  relay.listen(config.relayPort, "127.0.0.1", resolve);
});
process.env.QACS_BASE_URL = `http://127.0.0.1:${config.relayPort}`;
process.env.QACS_TOKEN = clientToken;
process.env.QACS_SUPPLEMENT_READ_ONLY = "true";
process.env.QACS_TOOLSETS = "core,files,qnap";
const { createServer } = await import("./server.js");
const mcp = http.createServer(async (req, res) => {
  if (!authenticate(req)) return reply(res, 401, '{"error":"unauthorized"}');
  if (req.url !== "/mcp") return reply(res, 404, '{"error":"not_found"}');
  if (req.method !== "POST") return reply(res, 405, '{"error":"method_not_allowed"}');
  const server = createServer();
  const transport = new StreamableHTTPServerTransport({
    sessionIdGenerator: undefined, enableJsonResponse: true,
  });
  res.on("close", () => { void transport.close(); void server.close(); });
  try {
    const body = JSON.parse(await bodyText(req));
    await server.connect(transport);
    await transport.handleRequest(req, res, body);
  } catch {
    if (!res.headersSent) reply(res, 400, '{"error":"invalid_request"}');
  }
});
await new Promise((resolve, reject) => {
  mcp.once("error", reject);
  mcp.listen(config.mcpPort, "127.0.0.1", resolve);
});
console.log('{"event":"production_supplement_ready","mode":"read_only"}');
for (const signal of ["SIGTERM", "SIGINT"]) process.on(signal, () => {
  mcp.close();
  relay.close();
});
