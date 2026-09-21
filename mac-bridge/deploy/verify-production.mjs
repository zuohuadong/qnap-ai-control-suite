import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";

const config = JSON.parse(readFileSync("/etc/qnap-smb-supplement/config.json", "utf8"));
const token = readFileSync(config.clientTokenFile, "utf8").trim();
const endpoint = `http://127.0.0.1:${config.mcpPort}/mcp`;
const fixture = "/share/xigu-fa/_acl-acceptance-20260921-c64ecbb824/service-readonly-acceptance.txt";
assert.equal((await fetch(endpoint)).status, 401);
const client = new Client({ name: "fa-production-acceptance", version: "1.0.0" });
const transport = new StreamableHTTPClientTransport(new URL(endpoint), {
  requestInit: { headers: { Authorization: `Bearer ${token}` } },
});
try {
  await client.connect(transport);
  const { tools } = await client.listTools();
  assert.deepEqual(tools.map(t => t.name).sort(), [
    "nas_health", "nas_share_list", "nas_smb_status",
    "nas_file_list", "nas_file_stat", "nas_file_read", "nas_file_checksum",
  ].sort());
  assert.ok(tools.every(t => t.annotations.readOnlyHint));
  console.log("PASS authenticated MCP initialize and exactly seven read-only tools");
  for (const [name, args] of [
    ["nas_health", {}], ["nas_share_list", {}],
    ["nas_file_list", { path: "/share/xigu-fa/_device-inbox/xgic" }],
    ["nas_file_stat", { path: fixture }],
    ["nas_file_read", { path: fixture, max_bytes: 128 }],
    ["nas_file_checksum", { path: fixture }],
    ["nas_smb_status", {}],
  ]) {
    const result = await client.callTool({ name, arguments: args });
    assert.ok(!result.isError, `${name}: ${JSON.stringify(result)}`);
    const data = result.structuredContent;
    assert.ok(data);
    if (name === "nas_health") assert.equal(data.version, "2.1.3+32317ba");
    if (name === "nas_file_read") {
      assert.equal(Buffer.from(data.content_base64, "base64").toString(), "QNAP read-only supplement acceptance 32317ba\n");
    }
    if (name === "nas_file_checksum") {
      assert.ok(JSON.stringify(data).includes("6d0afa8547618f5fffe2bdbb0e166f9c76b0d871172c2700667da33eb8efa065"));
    }
    console.log(JSON.stringify({ check: name, result: "pass",
      ...(name === "nas_smb_status" ? { capability: data } : {}),
      ...(name === "nas_file_list" ? { listing: data } : {}) }));
  }
  const denied = await client.callTool({ name: "nas_file_read", arguments: { path: "/state/config.json" } });
  assert.equal(denied.isError, true);
  console.log("PASS outside-root file read denied");
  const write = await client.callTool({ name: "nas_file_write", arguments: { path: fixture, content: "must-not-write" } });
  assert.equal(write.isError, true);
  console.log("PASS unavailable write tool rejected");
} finally { await client.close(); }
