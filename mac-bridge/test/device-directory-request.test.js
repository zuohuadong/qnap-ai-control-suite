import { test } from "node:test";
import assert from "node:assert/strict";
import { deviceDirectoryRequest } from "../src/device-directory-request.js";
test("directory capability accepts only scoped structured input", () => {
  const value = { tenant:"xgic", device:"X光-3212d2fa-0e91-4934-be2c-7c44d7ced872", stage:"01_待上传", caseDirectory:"FA-26-01013_sample" };
  assert.deepEqual(deviceDirectoryRequest(JSON.stringify(value)), value);
  for (const change of [{tenant:"../"}, {device:"other"}, {stage:"../"}, {caseDirectory:"a/b"}, {caseDirectory:"."}, {caseDirectory:"测".repeat(86)}, {shell:"id"}]) {
    assert.throws(() => deviceDirectoryRequest(JSON.stringify({...value,...change})));
  }
});
