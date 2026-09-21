const stages = new Set(["01_待上传", "02_处理中", "03_已归集", "04_历史记录", "99_异常待确认"]);
export function deviceDirectoryRequest(body) {
  const value = JSON.parse(body);
  const keys = ["tenant", "device", "stage", "caseDirectory"];
  if (!value || typeof value !== "object" || Array.isArray(value)
      || Object.keys(value).length !== keys.length || !keys.every(key => typeof value[key] === "string")
      || !/^[A-Za-z0-9_-]{1,80}$/.test(value.tenant)
      || !/^.+-[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(value.device)
      || !stages.has(value.stage)) throw new Error("Invalid directory request");
  for (const part of [value.device, value.caseDirectory]) {
    if (!part || part === "." || part === ".." || Buffer.byteLength(part) > 255
        || /[\\/\u0000-\u001f\u007f-\u009f]/.test(part)) throw new Error("Invalid directory component");
  }
  return value;
}
