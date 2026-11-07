# Preferences

Access preferences at `https://x.scour.li/server/preferences`.

## Settings

### Appearance

- **Theme**: Auto, dark, or light
- **Grid Density**: Comfortable, default, or compact
- **Thumbnail Size**: Small, medium, or large

### Video Preview

- **Autoplay Preview**: Enable or disable hover preview playback
- **Preview Delay**: Instant, 200ms, 500ms, or 1s

### Search Settings

- **Results Per Page**: Infinite scroll, 20, 50, or 100
- **Open New Tab**: Choose whether result links open in a new tab

### Default Filters

- **Preview First**: Prefer results with video previews
- **Show AI-Generated Content**: Include or hide AI-generated content; hidden by default
- **Default Duration**: Any, under 10 minutes, 10–30 minutes, or over 30 minutes
- **Minimum Quality**: Any quality up to 4K-only
- **Default Sort**: Relevance, longest, shortest, most viewed, or best quality
- **Minimum Duration**: No minimum up to 30 minutes

### History and Favorites

- **Max History**: Unlimited, 10, 25, 50, or 100 items
- **Auto-Clear History**: Never, 1 day, 7 days, or 30 days
- **Import/Export**: Export or import search history and favorites

### Privacy

- **Use Tor**: Route searches through Tor when available
- **Thumbnail Proxy**: Always on; thumbnails are served through VidVeil to prevent direct browser requests to source sites

### Search Engines

- All configured engines are selected by default until you save a selection.
- Enable or disable engines individually
- Use **Select All** or **Select None** for quick changes

## Storage

Preferences are stored in browser cookies so the server can render the selected
theme, language, and search settings without JavaScript or an account. Some
JavaScript features may keep optional convenience state in `localStorage`, but
it is not required for preferences to work.

## Owner tokens

Manage private favorites and automation credentials separately from display
preferences at [`/server/preferences/tokens`](https://x.scour.li/server/preferences/tokens).
Restore a primary token there when moving to another browser. Create a linked
token for each automation or MCP client so it can be revoked independently.
The primary token is required for token management; do not use it as an
automation credential. The page shows the primary token and linked tokens in
separate cards, supports independent reveal, rotation, rename, expiry, and
revocation actions, and lets you forget the saved token in this browser without
deleting pastes. Choose a preset expiry or open **Custom date** to set an exact
expiry. The page works without JavaScript.

## Reset

Use **Reset preferences** to clear the site's non-credential preferences and
consent cookies. Your owner token is preserved. The reset also clears
browser-managed site storage.
