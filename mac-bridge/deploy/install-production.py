import json
import os
import pathlib
import pwd
import shutil
import socket
import subprocess
import sys

if socket.gethostname() != "vm1-supacloud":
    raise SystemExit("Unexpected production host")
if not pathlib.Path("/opt/supacloud/functions/vxmnblbzsxzutzrjntyu").is_dir():
    raise SystemExit("FA project missing")
root = pathlib.Path("/opt/qnap-smb-supplement")
settings = pathlib.Path("/etc/qnap-smb-supplement")
if root.exists() or settings.exists():
    raise SystemExit("Existing supplement deployment requires reviewed upgrade")
secrets = json.load(sys.stdin)
if set(secrets) != {"ssh_password", "agent_token", "client_token"}:
    raise SystemExit("Invalid credential keys")
if any(not isinstance(value, str) or "\n" in value or not value for value in secrets.values()):
    raise SystemExit("Invalid credential data")
try:
    account = pwd.getpwnam("qnap-supplement")
except KeyError:
    subprocess.run(["useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", "qnap-supplement"], check=True)
    account = pwd.getpwnam("qnap-supplement")
root.mkdir(mode=0o755)
subprocess.run(["tar", "-xf", "/tmp/qnap-production-release.tar", "-C", str(root)], check=True)
settings.mkdir(mode=0o750)
os.chown(settings, 0, account.pw_gid)
for name, value in secrets.items():
    path = settings / name
    fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o640)
    with os.fdopen(fd, "w") as out:
        out.write(value + "\n")
    os.chown(path, 0, account.pw_gid)
known_hosts = subprocess.run(
    ["ssh-keygen", "-F", "[192.168.1.50]:2222", "-f", "/home/ubuntu/.ssh/known_hosts"],
    check=True, capture_output=True, text=True
).stdout
(settings / "known_hosts").write_text(known_hosts)
config = {
    "sshHost": "192.168.1.50", "sshPort": 2222, "sshUser": "NAS_FA",
    "knownHosts": str(settings / "known_hosts"),
    "passwordFile": str(settings / "ssh_password"),
    "agentTokenFile": str(settings / "agent_token"),
    "clientTokenFile": str(settings / "client_token"),
    "askpass": str(root / "deploy/askpass.sh"),
    "relayPort": 18756, "mcpPort": 18757,
}
(settings / "config.json").write_text(json.dumps(config, indent=2) + "\n")
os.chmod(root / "deploy/askpass.sh", 0o755)
shutil.copyfile(root / "deploy/qnap-smb-supplement.service", "/etc/systemd/system/qnap-smb-supplement.service")
subprocess.run(["systemctl", "daemon-reload"], check=True)
subprocess.run(["systemctl", "enable", "--now", "qnap-smb-supplement.service"], check=True)
print("Production supplement installed; credentials not included in receipt.")
