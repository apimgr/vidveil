# Project Audit

Started: 2026-11-18

Scope: concrete compliance findings verified against the current AI.md. Findings are ordered by implementation priority. AI.md remains read-only. The primary agent owns implementation and validation.

## Pass 5: Spec Compliance

- [ ] [P1] PART 16, Contact antispam: `src/server/handler/server.go:736-780` only checks the old `contact_hp` honeypot, captcha, required fields, email syntax and field lengths before delivery. The form/API do not implement required shape constraints, the `website` honeypot, signed `form_token`/fill-time validation, per-IP/email/global submission limits, content rules, dedupe LRU, `contact.spam_blocked` logging or `contact_blocked_total`. The API form description at `src/server/handler/server.go:1287-1320` omits `form_token`. Implement layers 1–7 before delivery, expose the matching GET/API token contract, and ensure rejection logging/metrics comply with PART 16 lines 28647-28673.
- [ ] [P1] PART 11 + PART 16, security-report protections: `src/server/handler/server.go:821-857` applies length/field validation and captcha but no shape/token/rate-limit layers; `src/server/handler/server.go:1382-1441` applies neither captcha nor any layers 1–5 before creating a report and sending notifications. Bring both HTML and API submissions under layers 1–5 and delivery-safety rules (AI.md PART 16 lines 28653-28671; PART 11 security-report submission flow).
- [ ] [P1] PART 19, canonical GeoIP sources and filenames: `src/server/service/geoip/geoip.go:21-26` still downloads ASN and Country from deprecated jsDelivr npm packages, and Country uses the explicitly forbidden combined `geo-whois-asn-country` dataset. AI.md lines 30136-30149 require GitHub release assets `iptoasn-asn.mmdb` and `iptoasn-country.mmdb`; `src/server/service/geoip/geoip.go:144-155,228-245` stores/opens them under legacy `asn.mmdb` and `country.mmdb`. Replace source URLs and use the canonical dataset names. City release URLs are already on the required source.
- [ ] [P1] PART 19, GeoIP integrity: `src/server/service/geoip/geoip.go:182-217` downloads directly to a temporary file then renames it without fetching/verifying the required SHA-256 checksum. `Update` uses the same path (`src/server/service/geoip/geoip.go:537-573`). AI.md line 30138 requires verifying `.../releases/download/checksum/{file}.sha256` before replacing a live database. Checksum verification must precede atomic replacement for first download and scheduled updates.
- [ ] [P1] PART 16, preference code precedence: `src/server/handler/preferences_api.go:223-236` overwrites each submitted form value with every same-named imported value before validation. AI.md line 24546 says valid code values override form values key by key; invalid imported values must not erase valid form values. Filter imported entries through the preference validator before applying per-key overrides.
- [ ] [P2] PART 16, Reset completeness: `src/server/handler/preferences_api.go:212-218` deletes a hard-coded cookie list and `vidveil_pref_*`, but omits the noncredential engine-selection cookies `vidveil_pref_engines_selected` and `vidveil_pref_engines` declared at `src/server/handler/handlers.go:153-154` (the latter matches the prefix but the former does not). AI.md rule 15 at line 24547 requires deleting every noncredential cookie the app sets. Include all app-set noncredential cookies, preferably based on a centralized inventory, while retaining credentials.
- [ ] [P2] PART 19, GeoIP default directory: `src/server/service/geoip/geoip.go:105-112` defaults to `{config}/security/geoip`; AI.md lines 30175-30182 specify `{data_dir}/security/geoip`. Align the fallback with the required data directory so GeoIP data is kept with runtime data rather than configuration.

## Pass 1: Security

- [ ] [P1] PART 16 contact abuse controls are absent as detailed above; public form submissions can reach `notifyDispatcher.Send` after only captcha/basic validation (`src/server/handler/server.go:749-779`), with no specified submission rate limits or duplicate suppression. Implement the required bounded in-memory controls before delivery.

## Pass 2: Code Quality

- [ ] [P2] PART 19, ignored cleanup errors: `src/server/service/geoip/geoip.go:194` discards response-body close errors and lines 209/213 discard temporary-file removal errors. Handle cleanup failures without masking the primary download error, consistent with project error-handling rules.

## Pass 3: Logic and Correctness

- [ ] [P1] PART 19, first-run availability: `src/server/service/geoip/geoip.go:120-136` returns download/open failures from initialization. Confirmed `src/main.go` caller behavior still needs to preserve the PART 19 fail-open contract: a missing/unavailable/stale DB must warn and let normal request processing continue, never make a GeoIP lookup error deny requests (AI.md lines 30121, 30133-30134, 30234). Verify the service startup caller and country restriction path when implementing the source/checksum fix.

## Pass 4: Documentation Completeness

- [ ] [P2] PART 19, IDEA.md/docs: after correcting GeoIP sources, names and checksum behavior, synchronize the project description and operator-facing GeoIP documentation. AI.md is explicit that the old WHOIS dataset/terminology is invalid (lines 30138-30151); do not retain outdated source claims.

## Pass 6: Code Flow Trace

- [ ] [P1] PART 16, contact request path: HTML submission flows from `handleContactSubmit` (`src/server/handler/server.go:736`) directly into notification delivery (`:772-779`); API metadata and submit handling are separate paths (`:1287-1320`, `:1382-1441`). The specified shared form definition/token and antispam layers are not present, so the API path currently bypasses even the HTML captcha check for security reports. Unify validation before persistence/delivery and verify both entry paths use it.

## Completed

- Read the current AI.md sections for PART 16 preferences/contact, PART 19 GeoIP and PART 31 Tor. Tor startup is wired from `src/main.go:995-1018`; the managed Tor setup uses an auto control port, disables relay/exit behavior, and maps the hidden service to a dedicated loopback backend (`src/server/service/tor/service.go:1205-1308`). No Tor finding is recorded from this scoped review.
- Confirmed required DB-IP attribution appears visibly in `src/server/template/page/server-about.tmpl:40`; no attribution finding.
