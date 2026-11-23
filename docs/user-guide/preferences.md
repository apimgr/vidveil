# Preferences

Open **Preferences** at `/server/preferences`. The page stores visitor
preferences in browser cookies so the server can apply them while rendering
pages. It does not create a visitor account or server-side preference profile.

## Main settings

- **Appearance and language**: Choose a theme and one of the supported
  languages, or return language selection to automatic detection.
- **Search and results**: Set result count, tab behavior, default sorting,
  duration and quality filters, preview behavior, and grid and thumbnail size.
- **History**: Set the browser search-history limit and its automatic cleanup
  interval. Search-history entries stay in browser storage.
- **Search engines**: Choose which registered engines to query. With no saved
  selection, all enabled engines are used.
- **Privacy**: Review cookie-consent categories and, where applicable, the
  CCPA opt-out control. Thumbnail requests are proxied by VidVeil.

Save the form to apply changes. **Cancel** discards unsaved edits. **Reset**
opens a confirmation popover; confirming clears the application's
non-credential cookies and browser-managed site storage. It preserves owner
credentials.

## Sync preferences between browsers

Open `/server/preferences/sync` from the **More preferences** card. The page
always displays a full import URL and a short code for the current theme,
language, and valid app preferences. Copy either value to the other browser
and open the URL or paste the value into the import field. The import form
works without JavaScript, and the server does not store the exported values.
Cookie consent, CCPA choices, build stamps, and credentials are not included.

## Search history and favorites

Search history is browser-local. The history controls on the preferences page
export, import, or clear that local list; they do not export favorites.
Favorites are private owner resources stored by the server and managed on the
Favorites page. Use the favorites export/import controls for favorites.

## Owner tokens

Manage owner tokens at `/server/preferences/tokens`. Restore the primary token
when moving to another browser. Create a linked token for each automation or
MCP client so it can be revoked independently. The primary token is required
for token management; do not use it as an automation credential. The page
supports revealing and rotating the primary or linked tokens, renaming,
expiry changes, revocation, and forgetting the saved token in this browser.
Forgetting the browser's saved token does not delete its favorites. The page
works without JavaScript.
