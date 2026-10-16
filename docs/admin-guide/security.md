# Operator Security

There is no admin web UI or operator account login. Operator-only endpoints
and sensitive maintenance operations use the server-wide `server.token` from
`server.yml`; the token is not generated or managed through a web page.

See [Security](../security.md) for the threat model,
[Configuration](../configuration.md) for protected settings, and the
[CLI reference](../cli.md) for maintenance commands.
