# Production Supplement

The consumer is the FA production server, not a developer workstation.

Deployment on 2026-09-21:

- Production host: vm1-supacloud; FA project vxmnblbzsxzutzrjntyu.
- NAS: NAS94E4A0, QuTS hero 6.0.2 build 20260819.
- Service: qnap-smb-supplement.service, enabled, non-root qnap-supplement user.
- HTTP read relay: 127.0.0.1:18756.
- Streamable HTTP MCP: 127.0.0.1:18757/mcp.
- Both listeners require a separate production client token.
- The NAS Agent remains on NAS 127.0.0.1:8756, version 2.1.3+32317ba.

The NAS refuses SSH port forwarding. The production relay instead executes
one fixed NAS-local curl command over authenticated SSH. Requests are constrained
to seven read endpoints. Tokens and payloads use stdin; SSH authentication uses
an askpass helper reading a protected credential file. SSH host-key verification
is mandatory. No SSH policy, SMB configuration, ACL or NAS listener was changed.

`mac-bridge/deploy/install-production.py` is a first-install operation, bound to
the verified host and FA project. Existing deployments are rejected for explicit
upgrade review. Credentials arrive as JSON on stdin, never as command arguments.
The deployment archive contains the mac-bridge directory contents, including
installed dependencies, but no credentials. Run it only after authenticating the
target host and verifying the archive hash.

Configuration and credentials live under /etc/qnap-smb-supplement.
The directory is root-owned 0750; secret files are root:qnap-supplement 0640.
Only this dedicated service account and root can read them.
Never include their contents in logs, Git or receipts.

Verified from production, including after service restart:

- MCP initialization and exactly seven read-only tools.
- NAS health/version and shared-folder inventory.
- Listing 17 device directories under /share/xigu-fa/_device-inbox/xgic.
- File stat/read and matching SHA-256 for the retained acceptance fixture.
- Missing authentication, outside-root reads and unavailable write tools rejected.
- Service active/running with NRestarts=0 after the explicit restart.

The SMB status tool returns supported=false because smbstatus is not installed
in the minimal NAS container. Do not report session inspection as supported.
This does not validate device-account SMB authentication, upload, recycle-bin
recovery or file-version recovery.

The FA deployment separately adds an optional actual-directory stat readback
after successful official MCP listings. The official MCP remains responsible for
account provisioning, directory creation and the authoritative directory list.
The supplement does not infer directory absence or create files.

Rollback: first remove both FA_QNAP_SUPPLEMENT_URL and FA_QNAP_SUPPLEMENT_TOKEN
from the FA project's runtime configuration, or restore the recorded previous
Function activations with compare-and-swap. Only then stop/disable this service.
Retain the credential directory and release receipt for deliberate cleanup.
No database migration is involved.
