---
name: VidVeil integration test gotchas
description: Durable details discovered while validating the Docker integration harness.
type: project
---

# Integration test gotchas

- The operator credential is `server.token` under the `server` mapping in `secrets.yml`. Other nested `token` keys exist, so the test parser must target the exact two space indentation of `server.token`.
- Token issuance and auth-check API tests that parse JSON must send `Accept: application/json`; the default response is content-negotiated text.
- Operator routes distinguish token validity from role: missing or invalid credentials return 401, a valid owner token returns 403 with `insufficient_scope`, and the operator token succeeds. Keep raw token values out of test output.
