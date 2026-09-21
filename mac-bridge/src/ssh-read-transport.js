import { spawn } from "node:child_process";

const routes = new Set([
  "GET /v1/health", "GET /v1/shares", "GET /v1/shares/smb-status",
  "GET /v1/files/list", "GET /v1/files/stat",
  "POST /v1/files/read", "POST /v1/files/checksum",
]);

export function curlConfiguration(method, path, body, token) {
  const url = new URL(path, "http://127.0.0.1:8756");
  if (url.origin !== "http://127.0.0.1:8756" || !path.startsWith("/v1/")
      || !routes.has(`${method} ${url.pathname}`) || url.hash
      || typeof token !== "string" || !/^[!-~]{20,8192}$/.test(token)) {
    throw new Error("Read transport request rejected");
  }
  if (Buffer.byteLength(body) > 65536) throw new Error("Request too large");
  return [
    `url = ${JSON.stringify(url.href)}`,
    `request = ${JSON.stringify(method)}`,
    `header = ${JSON.stringify(`Authorization: Bearer ${token}`)}`,
    'header = "Content-Type: application/json"',
    ...(body ? [`data = ${JSON.stringify(body)}`] : []),
    'write-out = "\\n%{http_code}"',
  ].join("\n") + "\n";
}

export async function sshReadRequest(config, method, path, body = "") {
  const input = curlConfiguration(method, path, body, config.agentToken);
  if (!/^[A-Za-z0-9._-]+$/.test(config.sshUser)
      || !/^[A-Za-z0-9.-]+$/.test(config.sshHost)
      || !Number.isInteger(config.sshPort) || config.sshPort < 1 || config.sshPort > 65535) {
    throw new Error("Invalid SSH target");
  }
  const child = spawn("/usr/bin/ssh", [
    "-T", "-o", "StrictHostKeyChecking=yes", "-o", `UserKnownHostsFile=${config.knownHosts}`,
    "-o", "ConnectTimeout=5", "-o", "NumberOfPasswordPrompts=1",
    "-o", "PreferredAuthentications=password,keyboard-interactive",
    "-p", String(config.sshPort), `${config.sshUser}@${config.sshHost}`,
    "/sbin/curl --silent --show-error --max-time 15 --config -",
  ], {
    stdio: ["pipe", "pipe", "pipe"],
    env: { PATH: "/usr/bin:/bin", DISPLAY: "qacs",
      SSH_ASKPASS_REQUIRE: "force", SSH_ASKPASS: config.askpass,
      QACS_SSH_PASSWORD_FILE: config.passwordFile },
  });
  child.stdin.on("error", () => {});
  child.stdin.end(input);
  child.stderr.resume();
  let output = "";
  let overflow = false;
  child.stdout.setEncoding("utf8");
  child.stdout.on("data", chunk => {
    output += chunk;
    if (Buffer.byteLength(output) > 2 * 1024 * 1024) {
      overflow = true;
      child.kill("SIGTERM");
    }
  });
  const timer = setTimeout(() => child.kill("SIGTERM"), 20000);
  const code = await new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", resolve);
  }).finally(() => clearTimeout(timer));
  if (code !== 0 || overflow) throw new Error("NAS SSH read transport failed");
  const separator = output.lastIndexOf("\n");
  const status = Number(output.slice(separator + 1));
  if (separator < 0 || !Number.isInteger(status) || status < 100 || status > 599) {
    throw new Error("NAS returned invalid HTTP status");
  }
  return { status, body: output.slice(0, separator) };
}
