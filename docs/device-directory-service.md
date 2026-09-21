# Device directory capability

The read-only Agent remains unchanged. The production relay optionally accepts
`POST /v1/device-directories/ensure` when `deviceDirectoriesEnabled` is true.
Existing bearer authentication and the four-request concurrency cap apply.

Input has exactly four strings: `tenant`, `device`, `stage`, `caseDirectory`.
The relay invokes a fixed helper through SSH as NAS_FA, with JSON on stdin:

```
/share/ZFS530_DATA/.qnap-device-directory/qnap-device-directory --root /share/ZFS20_DATA/xigu-fa/_device-inbox
```

The Linux helper opens each component relative to directory descriptors using
O_NOFOLLOW. Tenant/device directories must already exist. Only a named stage and
one case directory may be created; existing directories are verified, never
replaced. There are no ACL, chmod, chown, delete, file-write or shell parameters.
The helper inherits the NAS account's OS permissions, not sudo/root privileges.
Deployment installs its executable as root-owned code, without credentials.

The response echoes the exact request and `verified: true` after descriptor-based
directory verification. An SSH timeout or malformed receipt is an unknown outcome,
not permission to replay blindly. Read the target before retrying.
Directory creation is not atomic across its two levels: an interrupted request
may leave a stage without the case directory. It does not prove SMB client access
or durable recovery after sudden storage loss.

Production canary on 2026-09-21 created/verified FA-26-01013 under all five stages
of SC-2002-03. One request took 152ms; a repeated request succeeded and independent
Agent listing confirmed the directory. Official MCP file listing remained faulty.
