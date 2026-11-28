# Backup & Restore

VidVeil has no backup page or web upload flow. Operators create and restore
archives with the server maintenance CLI. Restore is authorized for a first-run
empty database, a root operator after confirmation, or the service user after
validating the operator token. Unprivileged users cannot restore. A managed
service is stopped while local files are replaced.

```sh
# Create a backup using server.backup.encryption_password when configured.
vidveil --maintenance backup

# Restore an archive; encrypted archives prompt for their password.
vidveil --maintenance restore /path/to/backup.tar.gz

# Verify an archive, including its manifest, database, and PGP keypair.
vidveil --maintenance backup test /path/to/backup.tar.gz
```

Backups include the server configuration, every `app_secrets` row in
`app_secrets.json`, and the local SQLite database when SQLite is configured.
With a remote libSQL database, the remote database is backed up by its
provider; VidVeil archives only `app_secrets.json` from it. Custom templates,
themes, SSL certificates, and data are included according to the configured
backup options. Archives contain a manifest and checksums. Encryption uses
AES-256-GCM with an Argon2id-derived key. The configured backup password is
stored in `secrets.yml` encrypted at rest with the server encryption key, never
in `server.yml`, a manifest, or an archive. If that key is lost and the
password cannot be decrypted, scheduled backups fail closed with
`backup.password_unreadable`. When an encrypted archive is tested or restored,
the operator enters its password interactively. Passwords are not accepted as
command-line arguments. Without a terminal, those commands report that an
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
tests the newest full backup, selected by the UTC date in its filename, and
also tests retained daily and hourly incrementals with their bases. It does not
use the manifest's `snapshot_at` value to select an archive. Incrementals with
different passwords may require testing individually.

When restoring with a remote libSQL database configured, an archive that
contains `server.db` seeds the remote database only when it is empty. A remote
database that already contains application rows is left untouched. Archived
application secrets are applied only when the remote `app_secrets` table has
no rows; live secrets and PGP key material remain authoritative for a populated
remote database.

Retention is configured in `server.yml` under `server.backup.retention`. The defaults keep one daily backup
and disable weekly, monthly, and yearly retention. See the
[configuration guide](../configuration.md) and [CLI reference](../cli.md).
