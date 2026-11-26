# Backup & Restore

VidVeil has no backup page or web upload flow. Operators create and restore
archives with the server maintenance CLI. Restore requires the operator token,
root access, or an empty database, and stops a managed service while replacing
its data.

```sh
# Create a backup using server.backup.encryption_password when configured.
vidveil --maintenance backup

# Restore an archive; encrypted archives prompt for their password.
vidveil --maintenance restore /path/to/backup.tar.gz

# Verify an archive, including its manifest, database, and PGP keypair.
vidveil --maintenance backup test /path/to/backup.tar.gz
```

Backups include the server configuration and database, and may include custom
templates, themes, SSL certificates, and data according to the configured
backup options. Archives contain a manifest and checksums. Encryption uses
AES-256-GCM with an Argon2id-derived key. The password is stored in
`secrets.yml`, never in `server.yml`, a manifest, an archive, or a command-line
argument. When an encrypted archive is tested or restored, the operator enters
its password interactively. Without a terminal, those commands report that an
interactive password is required.

Scheduled daily backups create a full archive and, when a previous full is
available with the same encryption password, one cumulative daily incremental.
Hourly incrementals are disabled by default. Restoring an incremental applies
it over its referenced full archive and removes paths listed as deleted.
Retention protects the referenced full while its incremental remains.

`backup test` does not replace live configuration or data. It verifies the
archive checksum and manifest, checks the SQLite database, and confirms that
the encrypted PGP private key in the backup matches its public key using the
installation secret from that same backup. With no file argument, the command
selects the newest readable `snapshot_at`; encrypted archives may require a
password before the newest snapshot can be identified. If retained encrypted
archives use different passwords, specify the archive path explicitly.

Retention is configured in `server.yml` under `server.backup.retention`. The defaults keep one daily backup
and disable weekly, monthly, and yearly retention. See the
[configuration guide](../configuration.md) and [CLI reference](../cli.md).
