# Translating VidVeil

VidVeil stores translations in `src/common/i18n/locales/`; all binaries embed the
same locale files. English (`en.json`) is the source for the message keys and
English text. Keep keys unchanged and translate values only.

## Supported languages

The embedded locales currently support English, Arabic, German, Spanish,
French, Italian, Japanese, and Chinese. Locale files are the runtime source for
available languages.

## Translation rules

- Translate every human-readable string. Keep product names, protocol names,
  technical identifiers, code, filenames, configuration keys, URLs, and literal
  values such as `dark` and `auto` unchanged.
- Use consistent formal address and concise, natural wording for each language.
- Preserve every named placeholder exactly, including braces and spelling. For
  example, `{name}` remains `{name}`.
- Keep the same message keys as `en.json`, with no empty or extra values. Use
  UTF-8 JSON, two-space indentation, and no trailing commas.
- Use whole sentences rather than fragments that depend on concatenation. Keep
  HTML out of translation values; templates provide the markup.
- Follow the language's punctuation and typography conventions. Long strings
  must remain readable at 320, 768, and 1280 pixel viewport widths; check RTL
  layouts in their mirrored direction.

## VidVeil terminology

| English term | Guidance |
| --- | --- |
| token | Keep `token`; do not translate it as a coin or token-shaped object |
| favorites | Translate as the user's saved favorites; do not call them pastes or posts |
| owner | Anonymous owner of private favorites, not an account holder |
| linked token | A separately revocable token for an API or MCP client |
| rotate / revoke | Keep the meanings distinct: replace a token / invalidate a token |
| MCP, API, Tor, JSON | Keep the protocol and technology names unchanged |

## Adding or updating a language

1. Add a complete locale file whose message keys match `en.json`.
2. Use the language's native name and correct text direction wherever language
   metadata is represented.
3. Keep every placeholder and translation key in parity with English.
4. Update documentation and the locale integrity tests when adding a language.
5. Run `make test`; this includes the locale key and placeholder checks.
6. Review representative pages in the language at mobile, tablet, and desktop
   widths. Check RTL layout when applicable.

AI and machine translations are a starting point. Use idiomatic wording and
consistent terminology; human translators can refine translations in later
changes. Do not add translator credits unless the translator has chosen to be
credited.
