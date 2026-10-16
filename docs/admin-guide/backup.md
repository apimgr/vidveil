# Backup & Restore

VidVeil has no backup page or web upload flow. Operators create and restore
archives with the server maintenance CLI. Restore requires the operator token,
root access, or an empty database, and stops a managed service while replacing
its data.

```sh
# Create a backup; the command prompts whether to encrypt it.
vidveil --maintenance backup

# Restore an archive; encrypted archives prompt for their password.
vidveil --maintenance restore /path/to/backup.tar.gz
```

Backups include the server configuration and database, and may include custom
templates, themes, SSL certificates, and data according to the configured
backup options. Archives contain a manifest and checksums. Encryption uses
AES-256-GCM with an Argon2id-derived key; the password is never stored by the
server.

Retention is configured in `server.yml` under `server.backup.retention`. The defaults keep one daily backup
and disable weekly, monthly, and yearly retention. See the
[configuration guide](../configuration.md) and [CLI reference](../cli.md).
