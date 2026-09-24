import assert from "node:assert/strict";
import test from "node:test";
import { curlConfiguration, deviceDirectoryCommand } from "../src/ssh-read-transport.js";

test("device directory root supports a confined storage cutover without shell expansion", () => {
  assert.ok(deviceDirectoryCommand({}).endsWith("--root /share/ZFS20_DATA/xigu-fa/_device-inbox"));
  assert.ok(deviceDirectoryCommand({ deviceDirectoryRoot: "/share/ZFS27_DATA/xigu-fas/_device-inbox" })
    .endsWith("--root /share/ZFS27_DATA/xigu-fas/_device-inbox"));
  for (const root of [
    "/", "/share/ZFS27_DATA/../_device-inbox",
    "/share/ZFS27_DATA/xigu-fas/_device-inbox;id",
    "/share/ZFS27_DATA/xigu-fas/_device-inbox\n",
    "/share/ZFS27_DATA/xigu-fas/fa-reports",
    "/share/ZFS27_DATA/$(id)/_device-inbox", 123,
  ]) assert.throws(() => deviceDirectoryCommand({ deviceDirectoryRoot: root }));
});

test("SSH read transport pins origin, methods and routes; escapes request data", () => {
  const token = "a".repeat(48);
  for (const [method, path] of [
    ["GET", "/v1/health"], ["GET", "/v1/shares"],
    ["GET", "/v1/shares/smb-status"], ["GET", "/v1/files/list?path=%2Fshare%2Fxigu-fa"],
    ["GET", "/v1/files/stat"], ["POST", "/v1/files/read"], ["POST", "/v1/files/checksum"],
  ]) assert.ok(curlConfiguration(method, path, "", token).includes("127.0.0.1:8756"));
  for (const [method, path] of [
    ["POST", "/v1/exec"], ["POST", "/v1/acl/set"], ["POST", "/v1/files/write"],
    ["GET", "//example.com/v1/health"], ["GET", "http://example.com/v1/health"],
    ["DELETE", "/v1/health"], ["GET", "/v1/health#fragment"],
  ]) assert.throws(() => curlConfiguration(method, path, "", token));
  const body = JSON.stringify({ path: '/share/xigu-fa/a"\nurl = "http://evil"' });
  const config = curlConfiguration("POST", "/v1/files/read", body, token);
  assert.equal(config.split("\n").filter(line => line.startsWith("url = ")).length, 1);
  assert.throws(() => curlConfiguration("POST", "/v1/files/read", "a".repeat(65537), token));
});
