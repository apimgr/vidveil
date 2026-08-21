// Vidveil - Frontend JavaScript
// AI.md PART 16: Single app.js file for all frontend functionality

// ============================================================================
// Theme Management - AI.md PART 16: Supports dark, light, auto modes
// ============================================================================
function setTheme(theme) {
    // Per AI.md PART 16: Use class instead of data-theme attribute
    // Supports: 'dark', 'light', 'auto' (auto uses prefers-color-scheme)
    document.documentElement.classList.remove('theme-dark', 'theme-light', 'theme-auto');
    document.documentElement.classList.add('theme-' + theme);

    // Cookie is the authoritative preference — server reads it to render <html class="theme-X">
    // with no init JS and no FOUC (AI.md PART 16: "Store preference in the theme cookie")
    var maxAge = 365 * 24 * 3600; // 1 year
    document.cookie = 'theme=' + encodeURIComponent(theme) + '; path=/; max-age=' + maxAge + '; SameSite=Lax';

    // Also persist in vidveil_prefs for localStorage-reading JS paths
    try {
        var prefs = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
        prefs.theme = theme;
        localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
    } catch (e) {}

    // Update meta theme-color for mobile browsers
    updateMetaThemeColor(theme);
}

function getTheme() {
    // Cookie is the primary source — server reads it per AI.md PART 16
    var match = document.cookie.match(/(?:^|;\s*)theme=([^;]*)/);
    if (match) return decodeURIComponent(match[1]);
    // Fall back to vidveil_prefs.theme in localStorage
    try {
        var prefs = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
        if (prefs.theme) return prefs.theme;
    } catch (e) {}
    // Default 'auto' per AI.md PART 16
    return 'auto';
}

// Get the effective theme (resolves 'auto' to actual light/dark)
function getEffectiveTheme() {
    var theme = getTheme();
    if (theme === 'auto') {
        return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
    }
    return theme;
}

// Update meta theme-color based on current theme
function updateMetaThemeColor(theme) {
    var metaTheme = document.querySelector('meta[name="theme-color"]');
    if (!metaTheme) {
        metaTheme = document.createElement('meta');
        metaTheme.name = 'theme-color';
        document.head.appendChild(metaTheme);
    }

    var effectiveTheme = theme;
    if (theme === 'auto') {
        effectiveTheme = window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
    }

    // Set appropriate theme-color for mobile browser chrome
    metaTheme.content = effectiveTheme === 'light' ? '#ffffff' : '#282a36';
}

// Listen for system preference changes when in auto mode
function setupThemeMediaListener() {
    var mediaQuery = window.matchMedia('(prefers-color-scheme: light)');

    function handleChange() {
        // Only react if we're in auto mode
        if (getTheme() === 'auto') {
            updateMetaThemeColor('auto');
            // Dispatch custom event for any components that need to know
            window.dispatchEvent(new CustomEvent('themechange', {
                detail: { theme: 'auto', effective: getEffectiveTheme() }
            }));
        }
    }

    // Modern browsers
    if (mediaQuery.addEventListener) {
        mediaQuery.addEventListener('change', handleChange);
    } else if (mediaQuery.addListener) {
        // Older Safari
        mediaQuery.addListener(handleChange);
    }
}

// Initialize theme listener on load
setupThemeMediaListener();

// Re-apply theme when page is restored from bfcache (browser back/forward).
// Without this, history.back() restores the old DOM class from the cached snapshot.
window.addEventListener('pageshow', function(event) {
    if (event.persisted) {
        setTheme(getTheme());
    }
});

// ============================================================================
// Screen Reader Announcements (AI.md PART 31: A11Y)
// ============================================================================
var announcer = null;

function initAnnouncer() {
    if (announcer) return;
    announcer = document.createElement('div');
    announcer.setAttribute('role', 'status');
    announcer.setAttribute('aria-live', 'polite');
    announcer.setAttribute('aria-atomic', 'true');
    announcer.className = 'sr-only';
    announcer.id = 'a11y-announcer';
    document.body.appendChild(announcer);
}

// Announce messages to screen readers without moving focus
function announce(message, priority) {
    if (!announcer) initAnnouncer();
    // Clear first, then set after delay to trigger announcement
    announcer.textContent = '';
    announcer.setAttribute('aria-live', priority === 'assertive' ? 'assertive' : 'polite');
    setTimeout(function() {
        announcer.textContent = message;
    }, 100);
}

// Initialize announcer when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAnnouncer);
} else {
    initAnnouncer();
}

// ============================================================================
// Preferences Management
// ============================================================================
const PREFS_KEY = 'vidveil_prefs';
const defaultPrefs = {
    theme: 'auto',  // Per AI.md PART 16: 'auto' uses system preference
    gridDensity: 'default',
    thumbnailSize: 'medium',
    autoplayPreview: true,
    previewDelay: 0,  // Instant
    // Server-authoritative (IDEA.md "Search Settings") — mirrored into the
    // results_per_page cookie on save; 0 = infinite scroll (the default)
    resultsPerPage: 0,
    openNewTab: true,
    defaultPreviewOnly: true,
    showAIContent: false,  // AI content hidden by default
    defaultDuration: '',
    minQuality: '360',  // Default to 360p+ quality
    defaultSort: '',
    minDuration: 600,  // 10 minutes in seconds
    maxHistory: 0,  // 0 = unlimited
    autoClearHistory: 0,
    useTor: false,
    proxyImages: true,
    enabledEngines: [] // Empty means all enabled
};

function getPreferences() {
    try {
        const stored = localStorage.getItem(PREFS_KEY);
        return stored ? { ...defaultPrefs, ...JSON.parse(stored) } : defaultPrefs;
    } catch {
        return defaultPrefs;
    }
}

function savePreferences(prefs) {
    localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
}

function resetPreferences() {
    localStorage.removeItem(PREFS_KEY);
    // Expire the theme cookie — server reads it, so clearing here resets server-rendered theme
    document.cookie = 'theme=; path=/; max-age=0; SameSite=Lax';
    location.reload();
}

// ============================================================================
// Engine Selection
// ============================================================================
function selectAllEngines() {
    document.querySelectorAll('input[name="engines"]').forEach(cb => cb.checked = true);
}

function selectNoneEngines() {
    document.querySelectorAll('input[name="engines"]').forEach(cb => cb.checked = false);
}

function selectTier(maxTier) {
    document.querySelectorAll('.tier').forEach((tier, index) => {
        const checkboxes = tier.querySelectorAll('input[name="engines"]');
        checkboxes.forEach(cb => cb.checked = (index + 1) <= maxTier);
    });
}

// ============================================================================
// Search Results Sorting/Filtering
// ============================================================================
// ============================================================================
// Unified Filter Panel - Uses HTML5 details/summary for toggle
// ============================================================================
// Update filter count badge
function updateFilterCount() {
    var countEl = document.getElementById('filter-count');
    if (!countEl) return;

    var count = 0;
    var selects = document.querySelectorAll('.filters-content select');
    selects.forEach(function(select) {
        if (select.value && select.value !== '') {
            count++;
        }
    });

    if (count > 0) {
        countEl.textContent = count;
        countEl.classList.remove('hidden');
    } else {
        countEl.classList.add('hidden');
    }
}

// Handle filter changes - records the requested duration/quality/sort, then
// (on the search-results page) re-runs the search server-side so the server
// stays authoritative for the actual filtered/sorted set - AI.md PART 16
// "JavaScript enhances, it does not enable". On the home page (no
// #filters-form yet, no results to refetch), this only updates the count
// badge; the values still submit natively with the main search form.
function handleFilterChange() {
    updateFilterCount();

    var duration = document.getElementById('filter-duration');
    var quality = document.getElementById('filter-quality');
    var sort = document.getElementById('filter-sort');

    if (duration) filterByDuration(duration.value);
    if (quality) filterByQuality(quality.value);
    // Source filter is now handled independently via updateSourceFilter()
    if (sort) sortResults(sort.value);

    if (document.getElementById('filters-form') && window.Vidveil && window.Vidveil.Search && window.Vidveil.Search.refetch) {
        window.Vidveil.Search.refetch();
    }
}

// Close filters when clicking outside (for compact mode)
document.addEventListener('click', function(e) {
    var panel = document.getElementById('filters-panel');
    var toggle = document.getElementById('filters-toggle');
    if (!panel || !toggle) return;

    // Check if panel is in compact mode
    if (!panel.classList.contains('filters-panel--compact')) return;

    // If click is outside the panel, close it
    if (!panel.contains(e.target)) {
        var content = document.getElementById('filters-content');
        if (content && content.classList.contains('expanded')) {
            toggle.setAttribute('aria-expanded', 'false');
            content.classList.remove('expanded');
        }
    }
});

// Export functions globally
// Note: toggleFilters removed - using native HTML5 details/summary for toggle
window.updateFilterCount = updateFilterCount;
window.handleFilterChange = handleFilterChange;

// Note: filterByDuration/filterByQuality/sortResults used by
// handleFilterChange() above resolve to window.filterByDuration etc., which
// the search-page IIFE assigns further down (window.filterByDuration =
// searchFilterByDuration, etc.) before any filter-change event can fire.

// Lazy loading: Uses native loading="lazy" attribute on images - no JS needed

// ============================================================================
// Keyboard Shortcuts
// ============================================================================
function setupKeyboardShortcuts() {
    document.addEventListener('keydown', (e) => {
        // Focus search on '/' key
        if (e.key === '/' && document.activeElement.tagName !== 'INPUT') {
            e.preventDefault();
            const searchInput = document.querySelector('.search-form input');
            if (searchInput) searchInput.focus();
        }

        // Clear search on Escape
        if (e.key === 'Escape') {
            const searchInput = document.querySelector('.search-form input');
            if (searchInput && document.activeElement === searchInput) {
                searchInput.blur();
            }
        }
    });
}

// ============================================================================
// Preferences Form
// ============================================================================
function setupPreferencesForm() {
    const form = document.getElementById('preferences-form');
    if (!form) return;

    // If preferences.tmpl already set up the form, don't interfere
    if (form.dataset.managed === 'true') return;

    const prefs = getPreferences();

    // Set form values from preferences
    const themeSelect = document.getElementById('theme');
    if (themeSelect) themeSelect.value = prefs.theme;

    const gridDensitySelect = document.getElementById('grid-density');
    if (gridDensitySelect) gridDensitySelect.value = prefs.gridDensity || 'default';

    const thumbnailSizeSelect = document.getElementById('thumbnail-size');
    if (thumbnailSizeSelect) thumbnailSizeSelect.value = prefs.thumbnailSize || 'medium';

    const autoplayCheckbox = document.getElementById('autoplay-preview');
    if (autoplayCheckbox) autoplayCheckbox.checked = prefs.autoplayPreview !== false;

    const previewDelaySelect = document.getElementById('preview-delay');
    if (previewDelaySelect) previewDelaySelect.value = prefs.previewDelay ?? 0;

    const resultsSelect = document.getElementById('results-per-page');
    if (resultsSelect) resultsSelect.value = prefs.resultsPerPage || 0;

    const openNewTabCheckbox = document.getElementById('open-new-tab');
    if (openNewTabCheckbox) openNewTabCheckbox.checked = prefs.openNewTab !== false;

    const defaultPreviewOnlyCheckbox = document.getElementById('default-preview-only');
    if (defaultPreviewOnlyCheckbox) defaultPreviewOnlyCheckbox.checked = prefs.defaultPreviewOnly !== false;

    const defaultDurationSelect = document.getElementById('default-duration');
    if (defaultDurationSelect) defaultDurationSelect.value = prefs.defaultDuration || '';

    const defaultQualitySelect = document.getElementById('default-quality');
    if (defaultQualitySelect) defaultQualitySelect.value = prefs.defaultQuality || '';

    const defaultSortSelect = document.getElementById('default-sort');
    if (defaultSortSelect) defaultSortSelect.value = prefs.defaultSort || '';

    const minDurationSelect = document.getElementById('min-duration');
    if (minDurationSelect) minDurationSelect.value = prefs.minDuration ?? 600;

    const torCheckbox = document.getElementById('use-tor');
    if (torCheckbox) torCheckbox.checked = prefs.useTor || false;

    const proxyCheckbox = document.getElementById('proxy-images');
    if (proxyCheckbox) proxyCheckbox.checked = prefs.proxyImages !== false;

    // Restore engine selections from localStorage
    if (prefs.enabledEngines && prefs.enabledEngines.length > 0) {
        document.querySelectorAll('input[name="engines"]').forEach(cb => {
            cb.checked = prefs.enabledEngines.includes(cb.value);
        });
    }

    // Handle form submission
    form.addEventListener('submit', (e) => {
        e.preventDefault();

        const engines = [];
        document.querySelectorAll('input[name="engines"]:checked').forEach(cb => {
            engines.push(cb.value);
        });

        const newPrefs = {
            theme: document.getElementById('theme')?.value || 'auto',
            gridDensity: document.getElementById('grid-density')?.value || 'default',
            thumbnailSize: document.getElementById('thumbnail-size')?.value || 'medium',
            autoplayPreview: document.getElementById('autoplay-preview')?.checked ?? true,
            previewDelay: parseInt(document.getElementById('preview-delay')?.value ?? 0),
            resultsPerPage: parseInt(document.getElementById('results-per-page')?.value ?? 0),
            openNewTab: document.getElementById('open-new-tab')?.checked ?? true,
            defaultPreviewOnly: document.getElementById('default-preview-only')?.checked ?? true,
            defaultDuration: document.getElementById('default-duration')?.value || '',
            defaultQuality: document.getElementById('default-quality')?.value || '',
            defaultSort: document.getElementById('default-sort')?.value || '',
            minDuration: parseInt(document.getElementById('min-duration')?.value ?? 600),
            useTor: document.getElementById('use-tor')?.checked || false,
            proxyImages: document.getElementById('proxy-images')?.checked ?? true,
            enabledEngines: engines
        };

        savePreferences(newPrefs);
        setTheme(newPrefs.theme);
        mirrorServerPrefsToCookies(newPrefs);

        showNotification('Preferences saved!', 'success');
    });
}

// Mirrors the two server-authoritative preferences (IDEA.md "Search Settings":
// resultsPerPage, openNewTab) into their cookies so the server can decide
// pagination/link-target for both this JS client and any no-JS client that
// later loads the same browser profile. Same pattern as the theme cookie
// above (setTheme) — cookie name/values match resultsPerPageCookieName /
// openNewTabCookieName in src/server/handler/handlers.go.
function mirrorServerPrefsToCookies(prefs) {
    var maxAge = 365 * 24 * 3600; // 1 year
    var resultsPerPage = parseInt(prefs.resultsPerPage ?? 0, 10);
    if (![0, 20, 50, 100].includes(resultsPerPage)) resultsPerPage = 0;
    document.cookie = 'results_per_page=' + resultsPerPage + '; path=/; max-age=' + maxAge + '; SameSite=Lax';
    var openNewTab = prefs.openNewTab !== false;
    document.cookie = 'open_new_tab=' + (openNewTab ? '1' : '0') + '; path=/; max-age=' + maxAge + '; SameSite=Lax';
}

// ============================================================================
// Notifications
// ============================================================================
function showNotification(message, type = 'info') {
    // Remove existing notifications
    document.querySelectorAll('.notification').forEach(n => n.remove());

    // Create notification element - styles are in common.css per AI.md PART 16
    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    notification.textContent = message;

    document.body.appendChild(notification);

    setTimeout(() => {
        notification.classList.add('notification-slide-out');
        setTimeout(() => notification.remove(), 300);
    }, 3000);
}

// ============================================================================
// API Helpers
// ============================================================================
// Read the csrf_token cookie for the CSRF double-submit pattern (AI.md PART 16)
function getCsrfToken() {
    var match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/);
    return match ? decodeURIComponent(match[1]) : '';
}

async function fetchAPI(endpoint, options = {}) {
    try {
        var method = (options.method || 'GET').toUpperCase();
        // Add X-CSRF-Token header on non-GET/HEAD/OPTIONS requests (AI.md PART 16 CSRF)
        var csrfHeaders = {};
        if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
            csrfHeaders['X-CSRF-Token'] = getCsrfToken();
        }
        const response = await fetch(`/api${endpoint}`, {
            headers: {
                'Content-Type': 'application/json',
                ...csrfHeaders,
                ...options.headers
            },
            ...options
        });

        if (!response.ok) {
            throw new Error(`API error: ${response.status}`);
        }

        return await response.json();
    } catch (error) {
        console.error('API request failed:', error);
        throw error;
    }
}

async function getEngineCount() {
    try {
        const data = await fetchAPI('/engines');
        return data.engines?.length || 43;
    } catch {
        return 43; // Default fallback
    }
}

// ============================================================================
// Initialize
// ============================================================================
document.addEventListener('DOMContentLoaded', async function() {
    // Set theme
    setTheme(getTheme());

    // Setup theme selector if present. The preferences page manages its own
    // #theme select (live preview + persist on save), so skip binding there to
    // avoid a duplicate change listener that would persist before the user saves.
    const themeSelect = document.getElementById('theme');
    if (themeSelect && !document.getElementById('preferences-form')) {
        themeSelect.value = getTheme();
        themeSelect.addEventListener('change', function() {
            setTheme(this.value);
        });
    }

    // Lazy loading uses native loading="lazy" attribute - no JS setup needed

    // Setup keyboard shortcuts
    setupKeyboardShortcuts();

    // Setup preferences form
    setupPreferencesForm();

    // Update engine count on home page
    const engineCountEl = document.getElementById('engine-count');
    if (engineCountEl) {
        const count = await getEngineCount();
        engineCountEl.textContent = count;
    }

    // Animation styles are now in common.css per AI.md PART 16

    // Initialize autocomplete for nav search (present on all pages except home)
    setupAutocomplete('nav-search-input', 'autocomplete-dropdown-nav');

    // Initialize home page features
    initHomePage();

    // Initialize search page features
    initSearchPage();
});

// ============================================================================
// Mobile Navigation - AI.md PART 13
// Slides in from RIGHT edge
// ============================================================================
function toggleNav() {
    const panel = document.getElementById('nav-panel');
    const overlay = document.getElementById('nav-overlay');
    if (panel && overlay) {
        panel.classList.toggle('open');
        overlay.classList.toggle('open');
        document.body.style.overflow = panel.classList.contains('open') ? 'hidden' : '';
    }
}

function closeNav() {
    const panel = document.getElementById('nav-panel');
    const overlay = document.getElementById('nav-overlay');
    if (panel && overlay) {
        panel.classList.remove('open');
        overlay.classList.remove('open');
        document.body.style.overflow = '';
    }
}

// Close nav on escape key
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        closeNav();
    }
});

// ============================================================================
// Admin Panel Functions - AI.md PART 16
// ============================================================================

// Admin section collapse state management
function toggleSection(name) {
    var section = document.getElementById('section-' + name);
    if (section) {
        section.classList.toggle('collapsed');
        saveCollapsedState();
    }
}

function saveCollapsedState() {
    var collapsed = [];
    document.querySelectorAll('.nav-section.collapsed').forEach(function(el) {
        collapsed.push(el.id.replace('section-', ''));
    });
    localStorage.setItem('vidveil_admin_collapsed', JSON.stringify(collapsed));
}

function loadCollapsedState() {
    try {
        var collapsed = JSON.parse(localStorage.getItem('vidveil_admin_collapsed')) || [];
        collapsed.forEach(function(name) {
            var section = document.getElementById('section-' + name);
            if (section) section.classList.add('collapsed');
        });
    } catch(e) {}

    // Auto-expand section containing active link
    var activeLink = document.querySelector('.nav-section-links .nav-link.active');
    if (activeLink) {
        var section = activeLink.closest('.nav-section');
        if (section) section.classList.remove('collapsed');
    }
}

// Toast notification system per AI.md PART 16: max 5 stacked, auto-dismiss
// Success 3s / Info 3s / Warning 5s, Error never auto-dismisses,
// pause-on-hover, Escape dismisses the topmost toast.
var TOAST_DISMISS_MS = { success: 3000, info: 3000, warning: 5000, error: 0 };
function showToast(message, type) {
    type = type || 'info';
    var container = document.getElementById('toast-container');
    if (!container) return;
    // Max 5 stacked — drop the oldest to make room
    while (container.children.length >= 5) {
        container.removeChild(container.firstChild);
    }
    var toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    // Build via DOM methods so message text can never inject HTML
    var msgSpan = document.createElement('span');
    msgSpan.textContent = message;
    var closeBtn = document.createElement('button');
    closeBtn.type = 'button';
    closeBtn.className = 'toast-close';
    closeBtn.setAttribute('data-action', 'close-toast');
    closeBtn.innerHTML = '&times;';
    toast.appendChild(msgSpan);
    toast.appendChild(closeBtn);
    container.appendChild(toast);
    setTimeout(function() { toast.classList.add('show'); }, 10);
    function dismissToast() {
        toast.classList.remove('show');
        setTimeout(function() { toast.remove(); }, 300);
    }
    var dismissMs = TOAST_DISMISS_MS[type] !== undefined ? TOAST_DISMISS_MS[type] : 3000;
    if (dismissMs > 0) {
        var dismissTimer = setTimeout(dismissToast, dismissMs);
        // Pause auto-dismiss while hovered, restart on leave
        toast.addEventListener('mouseenter', function() { clearTimeout(dismissTimer); });
        toast.addEventListener('mouseleave', function() { dismissTimer = setTimeout(dismissToast, dismissMs); });
    }
}

// Escape dismisses the topmost (newest) toast per AI.md PART 16
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        var toastContainer = document.getElementById('toast-container');
        if (toastContainer && toastContainer.lastElementChild) {
            toastContainer.lastElementChild.remove();
        }
    }
});

function showSuccess(msg) { showToast(msg, 'success'); }
function showError(msg) { showToast(msg, 'error'); }
function showWarning(msg) { showToast(msg, 'warning'); }
function showInfo(msg) { showToast(msg, 'info'); }

// Confirmation modal per AI.md PART 16 & PART 31 (A11Y)
var confirmModalCounter = 0;
function showConfirm(message, onConfirm, onCancel) {
    var id = 'confirm-modal-' + (++confirmModalCounter);
    var modal = document.createElement('dialog');
    modal.className = 'modal confirm-modal';
    modal.setAttribute('aria-modal', 'true');
    modal.setAttribute('aria-labelledby', id + '-title');
    modal.setAttribute('aria-describedby', id + '-desc');
    modal.innerHTML = '<div class="modal-header">' +
        '<h3 class="modal-title" id="' + id + '-title">Confirm Action</h3>' +
        '<button type="button" class="modal-close" aria-label="Close">&times;</button>' +
        '</div>' +
        '<div class="modal-body"><p id="' + id + '-desc"></p></div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary cancel-btn">Cancel</button>' +
        '<button type="button" class="btn btn-primary confirm-btn">Confirm</button>' +
        '</div>';
    // Message set via textContent so it can never inject HTML
    modal.querySelector('#' + id + '-desc').textContent = message;
    document.body.appendChild(modal);
    var triggerElement = document.activeElement;
    modal.showModal();
    // Focus on confirm button per PART 31 (first focusable element)
    modal.querySelector('.confirm-btn').focus();

    function closeModal(callback) {
        modal.close();
        modal.remove();
        // Return focus to trigger element per PART 31
        if (triggerElement && triggerElement.focus) {
            triggerElement.focus();
        }
        if (callback) callback();
    }

    modal.querySelector('.modal-close').onclick = function() { closeModal(onCancel); };
    modal.querySelector('.cancel-btn').onclick = function() { closeModal(onCancel); };
    modal.querySelector('.confirm-btn').onclick = function() { closeModal(onConfirm); };
    modal.addEventListener('cancel', function() { closeModal(onCancel); });
}

// ============================================================================
// Download Privacy Warning (IDEA.md: one-time warning for direct downloads)
// ============================================================================
var DOWNLOAD_WARNING_KEY = 'vidveil_download_warning_dismissed';

function isDownloadWarningDismissed() {
    try {
        return localStorage.getItem(DOWNLOAD_WARNING_KEY) === 'true';
    } catch (e) {
        return false;
    }
}

function dismissDownloadWarning() {
    try {
        localStorage.setItem(DOWNLOAD_WARNING_KEY, 'true');
    } catch (e) {}
}

function handleDownloadClick(event, downloadUrl) {
    if (isDownloadWarningDismissed()) {
        return true; // Allow navigation
    }
    event.preventDefault();
    event.stopPropagation();
    showConfirm(
        'Downloads connect directly to the source site, which exposes your IP address. ' +
        'Consider using a VPN or Tor Browser for privacy. This warning will not be shown again.',
        function() {
            dismissDownloadWarning();
            window.open(downloadUrl, '_blank', 'noopener,noreferrer');
        }
    );
    return false;
}

// ============================================================================
// Favorites (AI.md PART 16/32: server-side storage via anonymous visitor
// cookie is the source of truth and keeps /favorites and the Preferences
// page fully functional without JS. This module layers instant (no-reload)
// feedback on top via the JSON API through fetchAPI(), AND mirrors the list
// into localStorage so favorites survive a server/DB wipe on this browser:
// on first load each session the mirror and the server list are reconciled
// (any entry present only in the mirror is re-POSTed to the server, so a
// wiped server is restored from the browser; any entry the server already
// has is left alone), and every successful server read/write re-writes the
// mirror so both sides stay in sync going forward.
// ============================================================================
(function() {
    'use strict';
    var cache = null;
    var loadPromise = null;
    var synced = false;
    var STORAGE_KEY = 'vidveil_favorites_mirror';

    var hasStorage = (function() {
        try {
            var t = '__vidveil_storage_test__';
            window.localStorage.setItem(t, '1');
            window.localStorage.removeItem(t);
            return true;
        } catch (e) {
            return false;
        }
    })();

    function normalize(list) {
        return (list || []).map(function(f) {
            return {
                id: f.id,
                url: f.url,
                title: f.title || 'Untitled',
                thumbnail: f.thumbnail || '',
                source: f.source || ''
            };
        });
    }

    function readMirror() {
        if (!hasStorage) {
            return [];
        }
        try {
            return normalize(JSON.parse(window.localStorage.getItem(STORAGE_KEY) || '[]'));
        } catch (e) {
            return [];
        }
    }

    function writeMirror(list) {
        if (!hasStorage) {
            return;
        }
        try {
            window.localStorage.setItem(STORAGE_KEY, JSON.stringify(list || []));
        } catch (e) {
            // Storage full/unavailable mid-session; server stays source of truth.
        }
    }

    function loadFromDataIsland() {
        var el = document.getElementById('favorites-data');
        if (!el) {
            return null;
        }
        try {
            return normalize(JSON.parse(el.textContent || '[]'));
        } catch (e) {
            return null;
        }
    }

    function refresh() {
        loadPromise = fetchAPI('/v1/favorites').then(function(data) {
            cache = normalize(data.favorites);
            writeMirror(cache);
            return cache;
        }).catch(function() {
            cache = cache || readMirror();
            return cache;
        });
        return loadPromise;
    }

    // restoreMissing re-creates on the server any mirror entries the server
    // doesn't currently have (e.g. after a server/DB wipe), then re-reads the
    // server list so cache/mirror end up with real server-assigned ids.
    function restoreMissing(missing) {
        return Promise.all(missing.map(function(f) {
            return fetchAPI('/v1/favorites', {
                method: 'POST',
                body: JSON.stringify({
                    url: f.url,
                    title: f.title || 'Untitled',
                    thumbnail: f.thumbnail || '',
                    source: f.source || ''
                })
            }).catch(function() { return null; });
        })).then(refresh);
    }

    // reconcileWithMirror runs once per page load: compares the localStorage
    // mirror against the current server list (preferring the already-fresh
    // server-rendered data island over an extra fetch when it fully covers
    // the mirror) and restores anything the server is missing.
    function reconcileWithMirror() {
        synced = true;
        var island = loadFromDataIsland();
        if (!hasStorage) {
            if (island) {
                cache = island;
                return Promise.resolve(cache);
            }
            return refresh();
        }
        var mirror = readMirror();
        if (island) {
            var islandUrls = {};
            island.forEach(function(f) { islandUrls[f.url] = true; });
            var missingFromIsland = mirror.filter(function(f) { return !islandUrls[f.url]; });
            if (!missingFromIsland.length) {
                cache = island;
                writeMirror(cache);
                return Promise.resolve(cache);
            }
            return restoreMissing(missingFromIsland);
        }
        if (!mirror.length) {
            return refresh();
        }
        return fetchAPI('/v1/favorites').then(function(data) {
            var serverList = normalize(data.favorites);
            var serverUrls = {};
            serverList.forEach(function(f) { serverUrls[f.url] = true; });
            var missing = mirror.filter(function(f) { return !serverUrls[f.url]; });
            if (!missing.length) {
                cache = serverList;
                writeMirror(cache);
                return cache;
            }
            return restoreMissing(missing);
        }).catch(function() {
            cache = mirror;
            return cache;
        });
    }

    function ensureLoaded() {
        if (cache) {
            return Promise.resolve(cache);
        }
        if (!synced) {
            return reconcileWithMirror();
        }
        if (loadPromise) {
            return loadPromise;
        }
        return refresh();
    }

    function isFavorite(url) {
        return !!(cache && cache.some(function(f) { return f.url === url; }));
    }

    function findByUrl(url) {
        return cache ? cache.find(function(f) { return f.url === url; }) : null;
    }

    function add(video) {
        return fetchAPI('/v1/favorites', {
            method: 'POST',
            body: JSON.stringify({
                url: video.url,
                title: video.title || 'Untitled',
                thumbnail: video.thumbnail || '',
                source: video.source || ''
            })
        }).then(function() {
            return refresh();
        });
    }

    function removeByUrl(url) {
        var existing = findByUrl(url);
        if (!existing) {
            return Promise.resolve();
        }
        return fetchAPI('/v1/favorites/' + existing.id, { method: 'DELETE' }).then(function() {
            return refresh();
        });
    }

    function removeById(id) {
        return fetchAPI('/v1/favorites/' + id, { method: 'DELETE' }).then(function() {
            return refresh();
        });
    }

    function clear() {
        return fetchAPI('/v1/favorites', { method: 'DELETE' }).then(function() {
            return refresh();
        });
    }

    function toggle(video) {
        if (isFavorite(video.url)) {
            return removeByUrl(video.url).then(function() { return false; });
        }
        return add(video).then(function() { return true; });
    }

    // Export to Vidveil namespace
    window.Vidveil = window.Vidveil || {};
    window.Vidveil.Favorites = {
        ensureLoaded: ensureLoaded,
        isFavorite: isFavorite,
        toggle: toggle,
        add: add,
        removeByUrl: removeByUrl,
        removeById: removeById,
        clear: clear,
        refresh: refresh
    };

    document.addEventListener('DOMContentLoaded', function() {
        // ensureLoaded() triggers reconcileWithMirror(), which re-POSTs any
        // mirror-only entries back to the server (restoring a wiped DB) but
        // only updates the in-memory cache — the /favorites page's grid was
        // already server-rendered from the (pre-restore) DB state, so without
        // this it silently shows empty/stale until the visitor reloads.
        ensureLoaded().then(function(list) {
            if (typeof window.renderFavoritesGrid === 'function') {
                window.renderFavoritesGrid(list);
            }
        });
    });
})();

// Admin keyboard shortcuts per AI.md PART 15
function setupAdminKeyboardShortcuts() {
    var keySequence = '';
    var keyTimeout = null;
    document.addEventListener('keydown', function(e) {
        // Skip if in input/textarea
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return;

        // Ctrl+S: Save current form
        if ((e.ctrlKey || e.metaKey) && e.key === 's') {
            e.preventDefault();
            var saveBtn = document.querySelector('button[type="submit"], .btn-primary');
            if (saveBtn) saveBtn.click();
            return;
        }

        // Escape: Close modal/menu
        if (e.key === 'Escape') {
            var modal = document.querySelector('.modal.show, .modal[open]');
            if (modal) modal.remove();
            return;
        }

        // /: Focus search
        if (e.key === '/') {
            e.preventDefault();
            var search = document.querySelector('input[type="search"], input[name="search"], input[name="q"]');
            if (search) search.focus();
            return;
        }

        // ?: Show shortcuts help
        if (e.key === '?') {
            window.location.href = '/admin/help';
            return;
        }

        // Handle g + key sequences
        clearTimeout(keyTimeout);
        keySequence += e.key.toLowerCase();
        keyTimeout = setTimeout(function() { keySequence = ''; }, 500);

        if (keySequence === 'gd') {
            window.location.href = '/admin/dashboard';
        } else if (keySequence === 'gs') {
            window.location.href = '/admin/server/settings';
        } else if (keySequence === 'gl') {
            window.location.href = '/admin/server/logs';
        }
    });
}

// Initialize admin-specific features if on admin page
function initAdmin() {
    if (document.querySelector('.admin-nav')) {
        loadCollapsedState();
        setupAdminKeyboardShortcuts();
    }
}

// Run admin init on DOMContentLoaded
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAdmin);
} else {
    initAdmin();
}

// ============================================================================
// Autocomplete System - Reusable for all search inputs
// ============================================================================
(function() {
    'use strict';

    // Track all active autocomplete instances for global click handling
    var autocompleteInstances = [];

    // Create autocomplete for a search input
    function setupAutocomplete(inputId, dropdownId) {
        var input = document.getElementById(inputId);
        var dropdown = document.getElementById(dropdownId);
        if (!input || !dropdown) return null;

        var state = {
            input: input,
            dropdown: dropdown,
            selectedIndex: -1,
            suggestions: [],
            suggestionType: 'search',
            // Trailing phrase the autocomplete API says a selection replaces
            replaceToken: '',
            debounceTimer: null
        };

        function show() {
            dropdown.classList.add('visible');
            dropdown.hidden = false;
        }

        function hide() {
            dropdown.classList.remove('visible');
            dropdown.hidden = true;
            state.selectedIndex = -1;
        }

        function render() {
            if (state.suggestions.length === 0) {
                hide();
                return;
            }
            var html = state.suggestions.map(function(s, i) {
                var cls = 'autocomplete-item' + (i === state.selectedIndex ? ' selected' : '');
                if (state.suggestionType === 'bang' || state.suggestionType === 'bang_start') {
                    // Bang suggestions (engine shortcuts)
                    return '<div class="' + cls + '" data-index="' + i + '" role="option">' +
                           '<svg class="autocomplete-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>' +
                           '<span class="bang-code">' + escapeHtmlUtil(s.short_code || s.Bang || '') + '</span>' +
                           '<span class="bang-name">' + escapeHtmlUtil(s.display_name || s.EngineName || '') + '</span>' +
                           '</div>';
                } else {
                    // Search or performer suggestions
                    var term = s.term || s.Term || s;
                    var suggType = s.type || 'search';
                    var icon = '';
                    var typeLabel = '';

                    if (suggType === 'performer') {
                        // Performer icon (person silhouette)
                        icon = '<svg class="autocomplete-icon autocomplete-icon--performer" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>';
                        typeLabel = '<span class="suggestion-type suggestion-type--performer">performer</span>';
                    } else {
                        // Search icon (magnifying glass)
                        icon = '<svg class="autocomplete-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>';
                        typeLabel = '';
                    }

                    return '<div class="' + cls + '" data-index="' + i + '" role="option">' +
                           icon +
                           '<span class="search-term">' + escapeHtmlUtil(term) + '</span>' +
                           typeLabel +
                           '</div>';
                }
            }).join('');
            dropdown.innerHTML = html;
            show();
        }

        function select(index) {
            if (index < 0 || index >= state.suggestions.length) return;
            var s = state.suggestions[index];
            var val = input.value;
            var words = val.split(/\s+/);

            if (state.suggestionType === 'bang' || state.suggestionType === 'bang_start') {
                var bangCode = s.short_code || s.Bang || '';
                for (var i = words.length - 1; i >= 0; i--) {
                    if (words[i].startsWith('!')) {
                        words[i] = bangCode;
                        break;
                    }
                }
                if (val.trim().startsWith('!') && words.length === 1) {
                    words[0] = bangCode + ' ';
                }
                input.value = words.join(' ');
            } else {
                var term = s.term || s.Term || s;
                // Prefer the server's `replace` field: it names the exact
                // trailing phrase the suggestion should substitute (e.g. the
                // whole "@mia kh" for a multi-word performer name), so the
                // suggestion replaces instead of appending.
                var rep = state.replaceToken;
                if (rep && val.toLowerCase().endsWith(rep.toLowerCase())) {
                    input.value = val.slice(0, val.length - rep.length) + term;
                } else if (words.length <= 1) {
                    input.value = term;
                } else {
                    words[words.length - 1] = term;
                    input.value = words.join(' ');
                }
            }
            hide();
            input.focus();
        }

        function fetch_suggestions() {
            var q = input.value;
            if (!q || q.length < 2) {
                hide();
                return;
            }

            fetch('/api/v1/bangs/autocomplete?q=' + encodeURIComponent(q))
                .then(function(r) { return r.json(); })
                .then(function(data) {
                    if (data.ok && data.suggestions && data.suggestions.length > 0) {
                        state.suggestions = data.suggestions;
                        state.suggestionType = data.type || 'search';
                        // Trailing phrase the API says a selection should replace
                        state.replaceToken = data.replace || '';
                        state.selectedIndex = -1;
                        render();
                    } else {
                        hide();
                    }
                })
                .catch(function() { hide(); });
        }

        // Event listeners
        input.addEventListener('input', function() {
            clearTimeout(state.debounceTimer);
            state.debounceTimer = setTimeout(fetch_suggestions, 150);
        });

        input.addEventListener('keydown', function(e) {
            if (!dropdown.classList.contains('visible')) return;

            if (e.key === 'ArrowDown') {
                e.preventDefault();
                state.selectedIndex = Math.min(state.selectedIndex + 1, state.suggestions.length - 1);
                render();
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                state.selectedIndex = Math.max(state.selectedIndex - 1, 0);
                render();
            } else if (e.key === 'Enter' && state.selectedIndex >= 0) {
                e.preventDefault();
                select(state.selectedIndex);
            } else if (e.key === 'Escape') {
                hide();
            } else if (e.key === 'Tab' && state.selectedIndex >= 0) {
                e.preventDefault();
                select(state.selectedIndex);
            }
        });

        input.addEventListener('focus', function() {
            if (state.suggestions.length > 0) {
                render();
            }
        });

        dropdown.addEventListener('click', function(e) {
            var item = e.target.closest('.autocomplete-item');
            if (item) {
                select(parseInt(item.dataset.index, 10));
            }
        });

        autocompleteInstances.push({ hide: hide, input: input });
        return { hide: hide, fetch: fetch_suggestions };
    }

    // Global click handler to hide all dropdowns
    document.addEventListener('click', function(e) {
        if (!e.target.closest('.search-wrapper')) {
            autocompleteInstances.forEach(function(inst) { inst.hide(); });
        }
    });

    // Export for use by other modules
    window.setupAutocomplete = setupAutocomplete;
    window.hideAllAutocomplete = function() {
        autocompleteInstances.forEach(function(inst) { inst.hide(); });
    };
})();

// ============================================================================
// Home Page Functions
// ============================================================================
(function() {
    'use strict';

    var homeHistoryDiv = null;
    var homeAutocomplete = null;

    function initHomePage() {
        var homeInput = document.getElementById('search-input');
        homeHistoryDiv = document.getElementById('search-history');

        if (!homeInput) return; // Not on home page

        // Setup autocomplete for home search
        homeAutocomplete = setupAutocomplete('search-input', 'autocomplete-dropdown');

        // Render history on page load
        renderHomeSearchHistory();
    }

    function handleSearchSubmit(form) {
        var btn = form.querySelector('button[type="submit"]') || form.querySelector('button');
        if (!btn) return true;
        if (btn.disabled) return false;
        btn.disabled = true;

        if (btn.classList.contains('search-btn--compact') || btn.querySelector('svg')) {
            btn.classList.add('btn-loading');
        } else {
            btn.innerHTML = '<span class="btn-spinner"></span> Searching...';
        }

        // Hide all autocomplete dropdowns
        if (window.hideAllAutocomplete) window.hideAllAutocomplete();

        // Save to history
        var query = form.querySelector('input[name="q"]');
        if (query && query.value) {
            saveHomeSearchToHistory(query.value);
        }

        // Note: the engine tier <select name="engines"> (home page filters
        // panel) is now a DOM descendant of this form - see index.tmpl -
        // so it submits natively with the query. No manual hidden-field
        // copy needed (and copying it here would duplicate the param).

        return true;
    }

    function getHomeSearchHistory() {
        try {
            var history = JSON.parse(localStorage.getItem('vidveil_history') || '[]');
            // Auto-clear old entries based on preference
            var prefs = {};
            try { prefs = JSON.parse(localStorage.getItem('vidveil_prefs') || '{}'); } catch(e) {}
            var autoClear = parseInt(prefs.autoClearHistory) || 0;
            if (autoClear > 0) {
                var cutoff = Date.now() - (autoClear * 86400000);
                var filtered = history.filter(function(h) { return h.timestamp >= cutoff; });
                if (filtered.length !== history.length) {
                    localStorage.setItem('vidveil_history', JSON.stringify(filtered));
                }
                return filtered;
            }
            return history;
        } catch (e) {
            return [];
        }
    }

    function saveHomeSearchToHistory(query) {
        if (!query || query.trim().length < 2) return;
        query = query.trim(); // Strip whitespace per AI.md
        var history = getHomeSearchHistory();

        // Remove duplicate if exists (case-insensitive per AI.md)
        var queryLower = query.toLowerCase();
        history = history.filter(function(h) { return h.query.toLowerCase().trim() !== queryLower; });

        // Add to front
        history.unshift({ query: query, timestamp: Date.now() });

        // Respect maxHistory preference (0 = unlimited)
        var prefs = {};
        try { prefs = JSON.parse(localStorage.getItem('vidveil_prefs') || '{}'); } catch(e) {}
        var maxHist = parseInt(prefs.maxHistory) || 0;
        if (maxHist > 0 && history.length > maxHist) history = history.slice(0, maxHist);
        else if (maxHist === 0 && history.length > 200) history = history.slice(0, 200); // hard cap

        try {
            localStorage.setItem('vidveil_history', JSON.stringify(history));
        } catch (e) {}
    }

    function removeFromHomeHistory(query) {
        var history = getHomeSearchHistory();
        history = history.filter(function(h) { return h.query !== query; });
        try {
            localStorage.setItem('vidveil_history', JSON.stringify(history));
        } catch (e) {}
        renderHomeSearchHistory();
    }

    function clearHomeSearchHistory() {
        try {
            localStorage.removeItem('vidveil_history');
        } catch (e) {}
        renderHomeSearchHistory();
    }

    function formatTimeAgo(timestamp) {
        var seconds = Math.floor((Date.now() - timestamp) / 1000);
        if (seconds < 60) return 'just now';
        var minutes = Math.floor(seconds / 60);
        if (minutes < 60) return minutes + 'm ago';
        var hours = Math.floor(minutes / 60);
        if (hours < 24) return hours + 'h ago';
        var days = Math.floor(hours / 24);
        if (days < 7) return days + 'd ago';
        return new Date(timestamp).toLocaleDateString();
    }

    function renderHomeSearchHistory() {
        if (!homeHistoryDiv) return;

        var history = getHomeSearchHistory();
        if (history.length === 0) {
            homeHistoryDiv.innerHTML = '';
            homeHistoryDiv.style.display = 'none';
            return;
        }

        // Deduplicate history (case-insensitive) per AI.md
        var seen = {};
        var deduped = [];
        history.forEach(function(item) {
            var key = item.query.toLowerCase().trim();
            if (!seen[key]) {
                seen[key] = true;
                deduped.push(item);
            }
        });
        history = deduped;

        var html = '<div class="history-header"><span>Recent Searches</span><button type="button" data-action="home-clear-history" class="history-clear" aria-label="Clear search history">Clear</button></div>';
        html += '<div class="history-items">';

        history.slice(0, 8).forEach(function(item) {
            html += '<div class="history-item">';
            html += '<a href="/search?q=' + encodeURIComponent(item.query) + '" class="history-link" data-action="search-spinner">' + escapeHtmlUtil(item.query) + '</a>';
            html += '<span class="history-time">' + formatTimeAgo(item.timestamp) + '</span>';
            html += '<button type="button" data-action="home-remove-history" data-query="' + escapeHtmlUtil(item.query) + '" class="history-remove" aria-label="Remove from history">×</button>';
            html += '</div>';
        });

        html += '</div>';
        homeHistoryDiv.innerHTML = html;
        homeHistoryDiv.style.display = 'block';
    }

    // Show spinner when clicking search history link
    function showSearchSpinner(link, event) {
        // Change link text to spinner
        link.innerHTML = '<span class="btn-spinner"></span> Searching...';
        link.classList.add('searching');
        // Allow navigation to continue
        return true;
    }

    window.showSearchSpinner = showSearchSpinner;

    // Export history to JSON file
    function exportHistory() {
        var data = JSON.stringify(getHomeSearchHistory(), null, 2);
        var blob = new Blob([data], {type: 'application/json'});
        var url = URL.createObjectURL(blob);
        var a = document.createElement('a');
        a.href = url;
        a.download = 'vidveil-history.json';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        showSuccess('History exported');
    }

    // Import history from JSON file
    function importHistory(file) {
        var reader = new FileReader();
        reader.onload = function(e) {
            try {
                var imported = JSON.parse(e.target.result);
                if (Array.isArray(imported)) {
                    localStorage.setItem('vidveil_history', JSON.stringify(imported));
                    showSuccess('History imported (' + imported.length + ' items)');
                    renderHomeSearchHistory();
                } else {
                    showError('Invalid file format');
                }
            } catch (err) {
                showError('Failed to parse file');
            }
        };
        reader.readAsText(file);
    }

    // Export home functions
    window.initHomePage = initHomePage;
    window.handleSearchSubmit = handleSearchSubmit;
    window.Vidveil = window.Vidveil || {};
    window.Vidveil.Home = {
        clearHistory: clearHomeSearchHistory,
        removeFromHistory: removeFromHomeHistory,
        saveToHistory: saveHomeSearchToHistory,
        getHistory: getHomeSearchHistory,
        exportHistory: exportHistory,
        importHistory: importHistory
    };
    window.clearSearchHistory = clearHomeSearchHistory;
    window.removeFromHistory = removeFromHomeHistory;
})();

// ============================================================================
// Search Page Functions
// ============================================================================
(function() {
    'use strict';

    var searchQuery = '';
    var RESULTS_PER_BATCH = 20;
    var allResults = [];
    var displayedCount = 0;
    // True while a refetchSearchResults() background update is in flight and
    // the still-visible #video-grid content is the OLD (pre-refetch) result
    // set. The grid is only wiped once new data actually arrives (AI.md PART
    // 16: "JS enhances only, never enables" - the old, already-correct
    // server-rendered results stay usable the whole time, same as a real
    // page navigation leaves the old page intact until the new one is ready).
    var pendingGridSwap = false;
    var isSearching = true;
    // Tracks the in-flight SSE connection (initial stream or infinite-scroll
    // page fetch) so it can be closed on pagehide. An open EventSource is a
    // known bfcache blocker (Chrome/Firefox): leaving it open forces a full
    // network reload instead of an instant snapshot restore on browser
    // back/forward, which is what made the back button "reload results" or
    // transiently show "no results found" mid-refetch.
    var activeEventSource = null;
    var enginesCompleted = 0;
    // Live tracker, used only while streaming (isSearching)
    var enginesWithResults = new Set();
    // Total engines queried, from the SSE 'done' payload
    var enginesTotal = 0;
    // Authoritative final count, from the SSE 'done' payload
    var enginesWithResultsFinal = null;
    var sourcesSet = new Set();
    var searchCurrentDurationFilter = '';
    var searchCurrentQualityFilter = '';
    var searchCurrentSourceFilters = new Set(); // Multiple sources allowed
    var searchCurrentSort = '';
    var searchPreviewFirst = false; // Sort priority, not exclusive filter
    var startTime = 0; // Reset right before each search request for accuracy
    var isTouchDevice = 'ontouchstart' in window || navigator.maxTouchPoints > 0;
    var currentPage = 1;
    var isLoadingMore = false;
    var hasMoreResults = true;
    var infiniteScrollObserver = null;
    // Opaque per-search session token, generated once per new search and sent
    // as a passthrough query param on every page request (initial + infinite
    // scroll + fallback) so the server can dedup results across pages.
    var searchSessionID = '';

    // Preferences loaded from storage
    var userPrefs = {};

    // Note: Deduplication is handled server-side in manager.go, scoped per
    // searchSessionID (see SessionDedupStore). The client performs NO dedup
    // logic itself — it only generates and forwards the opaque session token.
    // AND-based term filtering with synonym expansion is also handled server-side
    // in manager.go using taxonomy.go. Client-side only handles duration/quality/source/preview filters.

    // Generates a random opaque session token for the current search.
    // Uses crypto.randomUUID() where available, falling back to Math.random()
    // for older browsers (progressive enhancement — dedup simply narrows to
    // per-request scope if the token is empty, per server contract).
    function generateSearchSessionID() {
        if (window.crypto && typeof window.crypto.randomUUID === 'function') {
            return window.crypto.randomUUID();
        }
        return 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'.replace(/x/g, function() {
            return Math.floor(Math.random() * 16).toString(16);
        });
    }

    function initSearchPage() {
        var searchMeta = document.getElementById('search-meta');
        if (!searchMeta) return; // Not on search page

        // Initialize autocomplete for results page search
        setupAutocomplete('results-search-input', 'autocomplete-dropdown-results');

        searchQuery = searchMeta.dataset.query || new URLSearchParams(window.location.search).get('q') || '';

        // Load preferences merged with defaults so defaultPreviewOnly:true and
        // other defaults take effect even when user has only partially saved prefs.
        userPrefs = getPreferences();
        var minDuration = parseInt(userPrefs.minDuration) || 0;

        // Apply grid density and thumbnail size (skip default values as they use base CSS)
        var grid = document.getElementById('video-grid');
        if (grid) {
            if (userPrefs.gridDensity && userPrefs.gridDensity !== 'default') {
                grid.classList.add('grid-' + userPrefs.gridDensity);
            }
            if (userPrefs.thumbnailSize && userPrefs.thumbnailSize !== 'medium') {
                grid.classList.add('thumbs-' + userPrefs.thumbnailSize);
            }
        }

        // Apply default filters from preferences. Preview-first stays a
        // client-side sort priority (mirrors the server's own previewFirst
        // stream ordering, never a data filter). Duration/quality/sort are
        // real server-authoritative filters (AI.md PART 16) but the defaults
        // themselves live in localStorage only (IDEA.md "User preferences" -
        // only results_per_page/open_new_tab are server-side cookies), so the
        // server can't have applied them to the page it already rendered. A
        // saved default takes effect via an in-place refetch (same request
        // parseResultFilterOptions/SearchWithOperators would apply from a
        // real query param) - never a full-page navigation, which was
        // causing a visible second page load (grid reset, scroll jump,
        // "in Xms" flicker) on every search that had a saved default.
        if (userPrefs.defaultPreviewOnly) {
            searchPreviewFirst = true;
            var previewCheckbox = document.getElementById('filter-preview-first');
            if (previewCheckbox) previewCheckbox.checked = true;
        }
        var urlParams = new URLSearchParams(window.location.search);
        var missingDefaults = new URLSearchParams();
        if (userPrefs.defaultDuration && !urlParams.get('duration')) {
            missingDefaults.set('duration', userPrefs.defaultDuration);
        }
        if (userPrefs.defaultQuality && !urlParams.get('quality')) {
            missingDefaults.set('quality', userPrefs.defaultQuality);
        }
        if (userPrefs.defaultSort && !urlParams.get('sort')) {
            missingDefaults.set('sort', userPrefs.defaultSort);
        }
        var hasMissingDefaults = [...missingDefaults.keys()].length > 0;
        if (hasMissingDefaults) {
            missingDefaults.forEach(function(value, key) { urlParams.set(key, value); });
        }
        searchCurrentDurationFilter = urlParams.get('duration') || '';
        searchCurrentQualityFilter = urlParams.get('quality') || '';
        searchCurrentSort = urlParams.get('sort') || '';
        var durationSelect = document.getElementById('filter-duration');
        if (durationSelect) durationSelect.value = searchCurrentDurationFilter;
        var qualitySelect = document.getElementById('filter-quality');
        if (qualitySelect) qualitySelect.value = searchCurrentQualityFilter;
        var sortSelect = document.getElementById('filter-sort');
        if (sortSelect) sortSelect.value = searchCurrentSort;

        // Save to search history
        if (searchQuery) {
            searchSessionID = generateSearchSessionID();
            saveSearchPageHistory(searchQuery);
            if (hasMissingDefaults) {
                // The page the server rendered didn't have these defaults
                // applied yet - refetch once, in place, with them included.
                refetchSearchResults();
            } else if (!hydrateServerResults(minDuration)) {
                // Hydrate from the server-rendered inline payload (no second
                // search). Only fall back to SSE streaming if missing.
                streamResults(minDuration);
            }
        }
    }

    // Enhance the server-rendered first page: read the inline JSON payload the
    // server already computed, rebuild richer cards (preview hover, favorites,
    // filter/sort data attributes) applying localStorage-only prefs, then apply
    // client-side sort/filter. No network round-trip — the search ran once,
    // server-side. Returns false when no payload is present (e.g. an old cached
    // page) so the caller can fall back to SSE streaming.
    function hydrateServerResults(minDuration) {
        var dataEl = document.getElementById('search-results-data');
        if (!dataEl) return false;

        var serverResults;
        try {
            serverResults = JSON.parse(dataEl.textContent || '[]');
        } catch (e) {
            return false;
        }
        if (!Array.isArray(serverResults)) return false;

        isSearching = false;
        hideSearchElement('initial-loading');

        // Replace the plain no-JS cards with enhanced cards built from the payload.
        var grid = document.getElementById('video-grid');
        if (grid) grid.innerHTML = '';

        if (serverResults.length === 0) {
            showNoResultsMessage();
            hasMoreResults = false;
            showSearchElement('search-meta');
            announce(getSearchI18n().noResults || 'No results found');
            updateSearchStatus();
            return true;
        }

        hideNoResultsMessage();
        showSearchElement('search-meta');
        showSearchElement('filters');

        for (var i = 0; i < serverResults.length; i++) {
            var r = serverResults[i];
            // Client-side min-duration filter (localStorage preference)
            if (minDuration > 0 && r.duration_seconds > 0 && r.duration_seconds < minDuration) {
                continue;
            }
            allResults.push(r);
            var source = r.source || '';
            if (source) enginesWithResults.add(source);
            addResultCard(r);
            if (source && !sourcesSet.has(source)) {
                sourcesSet.add(source);
                addSourceCheckbox(source, r.source_display || source);
            }
        }

        var countEl = document.getElementById('result-count');
        if (countEl) countEl.textContent = allResults.length;

        // The server rendered page 1; infinite scroll fetches further pages via SSE.
        currentPage = 1;
        hasMoreResults = true;
        setupInfiniteScroll();
        applySearchFiltersAndSort();
        announce(allResults.length + ' results found');
        initRelatedSearchesToggle();
        updateSearchStatus();
        return true;
    }

    function streamResults(minDuration) {
        if (!searchQuery) return;

        // Build search URL with optional parameters
        var searchUrl = '/api/v1/search?q=' + encodeURIComponent(searchQuery) + '&session=' + encodeURIComponent(searchSessionID);

        // Add show_ai parameter if user has enabled AI content in preferences
        if (userPrefs.showAIContent) {
            searchUrl += '&show_ai=1';
        }

        // Add min_quality parameter if user has set a minimum quality preference
        if (userPrefs.minQuality && parseInt(userPrefs.minQuality) > 0) {
            searchUrl += '&min_quality=' + userPrefs.minQuality;
        }

        // Tell the server to sort each engine batch preview-first during streaming
        if (searchPreviewFirst) {
            searchUrl += '&preview_first=1';
        }

        // Engines to query: the filter panel's own source selection (if the
        // visitor has narrowed it) takes priority over the blanket enabled-
        // engines preference, same precedence as the no-JS query param path.
        var streamEngineList = (searchCurrentSourceFilters.size > 0)
            ? Array.from(searchCurrentSourceFilters)
            : (userPrefs.enabledEngines || []);
        if (streamEngineList.length > 0) {
            searchUrl += '&engines=' + encodeURIComponent(streamEngineList.join(','));
        }

        // Duration/quality filters are server-authoritative (AI.md PART 16) -
        // forwarded as real query params, same ones parseResultFilterOptions
        // reads on a plain no-JS page load.
        if (searchCurrentDurationFilter) {
            searchUrl += '&duration=' + encodeURIComponent(searchCurrentDurationFilter);
        }
        if (searchCurrentQualityFilter) {
            searchUrl += '&quality=' + encodeURIComponent(searchCurrentQualityFilter);
        }

        // Send minimum duration preference to server for early filtering
        if (userPrefs.minDuration && parseInt(userPrefs.minDuration) > 0) {
            searchUrl += '&min_duration=' + parseInt(userPrefs.minDuration);
        }

        startTime = performance.now();
        var eventSource = new EventSource(searchUrl);
        activeEventSource = eventSource;
        var firstResult = true;
        var streamDone = false;

        // Watchdog: some reverse proxies buffer text/event-stream and never
        // flush the final "done" message or fire onerror, leaving the
        // connection open indefinitely and the spinner stuck forever (IDEA.md
        // "SSE -> JSON fallback" error handling requirement). Bound the wait
        // to the server's own per-search budget (engine timeout, default 15s)
        // plus margin, then fall back to the JSON API if nothing arrived.
        var streamWatchdog = setTimeout(function() {
            if (streamDone) return;
            eventSource.close();
            if (allResults.length === 0) {
                fallbackToJSONSearch(minDuration);
            } else {
                isSearching = false;
                pendingGridSwap = false;
                updateSearchStatus();
                hideSearchElement('status-bar');
                showToast('Some engines failed to respond', 'warning');
            }
        }, 25000);

        eventSource.onmessage = function(event) {
            var data = JSON.parse(event.data);

            // Final done message
            if (data.done && data.engine === 'all') {
                streamDone = true;
                clearTimeout(streamWatchdog);
                eventSource.close();
                isSearching = false;
                if (data.engines_total != null) enginesTotal = data.engines_total;
                if (data.engines_with_results != null) enginesWithResultsFinal = data.engines_with_results;
                var elapsed = data.elapsed_ms != null ? data.elapsed_ms : Math.round(performance.now() - startTime);
                var timeContainer = document.getElementById('search-time-container');
                if (timeContainer) timeContainer.textContent = 'in ' + elapsed + 'ms';
                updateSearchStatus();
                hideSearchElement('status-bar');

                if (allResults.length === 0) {
                    // Confirmed: the new filter/sort truly has zero results -
                    // now it's safe to clear the old (stale) grid/count and
                    // replace them with the "no results" state.
                    if (pendingGridSwap) {
                        pendingGridSwap = false;
                        var staleGrid = document.getElementById('video-grid');
                        if (staleGrid) staleGrid.innerHTML = '';
                        var staleCountEl = document.getElementById('result-count');
                        if (staleCountEl) staleCountEl.textContent = '0';
                    }
                    showNoResultsMessage();
                    hasMoreResults = false;
                    // A11Y: Announce no results to screen readers
                    announce(getSearchI18n().noResults || 'No results found');
                } else {
                    hideNoResultsMessage();
                    // Setup infinite scroll after initial results load
                    setupInfiniteScroll();
                    // Apply default filters from preferences
                    applySearchFiltersAndSort();
                    // A11Y: Announce result count to screen readers
                    announce(allResults.length + ' results found');
                    // Related searches are server-rendered — just add show-more toggle
                    initRelatedSearchesToggle();
                }
                return;
            }

            // Engine completed
            if (data.done) {
                enginesCompleted++;
                updateSearchStatus();
                return;
            }

            // Error from engine
            if (data.error) {
                enginesCompleted++;
                updateSearchStatus();
                return;
            }

            // Got a result (already deduplicated server-side)
            if (data.result && data.result.title) {
                var r = data.result;

                // Apply min duration filter (client-side additional filter)
                if (minDuration > 0 && r.duration_seconds > 0 && r.duration_seconds < minDuration) {
                    return;
                }

                // Show UI on first result
                if (firstResult) {
                    firstResult = false;
                    hideSearchElement('initial-loading');
                    showSearchElement('search-meta');
                    showSearchElement('filters');
                    // First new result of a background refetch - this is the
                    // moment new data has actually arrived, so it's safe to
                    // swap out the old (still-valid) grid contents now.
                    if (pendingGridSwap) {
                        pendingGridSwap = false;
                        var oldGrid = document.getElementById('video-grid');
                        if (oldGrid) oldGrid.innerHTML = '';
                        var oldCountEl = document.getElementById('result-count');
                        if (oldCountEl) oldCountEl.textContent = '0';
                    }
                }

                // Add to results and display immediately
                allResults.push(r);
                enginesWithResults.add(data.engine);
                addResultCard(r);

                var countEl = document.getElementById('result-count');
                if (countEl) countEl.textContent = allResults.length;

                // Add to source filter if new
                var source = r.source || '';
                if (source && !sourcesSet.has(source)) {
                    sourcesSet.add(source);
                    addSourceCheckbox(source, r.source_display || source);
                }
            }
        };

        eventSource.onerror = function(err) {
            // EventSource fires onerror when server closes connection (normal after done)
            if (streamDone) {
                eventSource.close();
                return;
            }
            clearTimeout(streamWatchdog);
            eventSource.close();
            // A named "event: error" SSE frame (the server's RATE_LIMITED
            // overload signal) arrives here with a data payload; a transport
            // drop does not. Don't JSON-fallback a deliberate overload
            // response into a second search — it would just get a 429 too.
            if (err && typeof err.data === 'string') {
                var overloaded = false;
                try {
                    overloaded = JSON.parse(err.data).error === 'RATE_LIMITED';
                } catch (e) {
                    overloaded = false;
                }
                if (overloaded) {
                    isSearching = false;
                    pendingGridSwap = false;
                    updateSearchStatus();
                    hideSearchElement('status-bar');
                    showToast('Search is busy — please try again shortly', 'warning');
                    return;
                }
            }
            // SSE failed - fallback to JSON API
            if (allResults.length === 0) {
                // Show user-friendly error message
                showToast('Connection interrupted. Retrying...', 'warning');
                fallbackToJSONSearch(minDuration);
            } else {
                // We have some results, just finish gracefully
                isSearching = false;
                pendingGridSwap = false;
                updateSearchStatus();
                hideSearchElement('status-bar');
                showToast('Some engines failed to respond', 'warning');
            }
        };

        // Update loading text
        var loadingText = document.getElementById('loading-text');
        if (loadingText) loadingText.textContent = 'Searching engines...';
    }

    // Fallback to JSON API when SSE fails (e.g., proxy doesn't support SSE)
    function fallbackToJSONSearch(minDuration) {
        var loadingText = document.getElementById('loading-text');
        if (loadingText) loadingText.textContent = 'Loading results...';

        var fallbackUrl = '/api/v1/search?q=' + encodeURIComponent(searchQuery) + '&session=' + encodeURIComponent(searchSessionID);
        var fallbackEngineList = (searchCurrentSourceFilters.size > 0)
            ? Array.from(searchCurrentSourceFilters)
            : (userPrefs.enabledEngines || []);
        if (fallbackEngineList.length > 0) {
            fallbackUrl += '&engines=' + encodeURIComponent(fallbackEngineList.join(','));
        }
        if (searchCurrentDurationFilter) {
            fallbackUrl += '&duration=' + encodeURIComponent(searchCurrentDurationFilter);
        }
        if (searchCurrentQualityFilter) {
            fallbackUrl += '&quality=' + encodeURIComponent(searchCurrentQualityFilter);
        }
        if (searchCurrentSort) {
            fallbackUrl += '&sort=' + encodeURIComponent(searchCurrentSort);
        }

        fetch(fallbackUrl, {
            headers: { 'Accept': 'application/json' }
        })
        .then(function(response) {
            if (!response.ok) {
                throw new Error('Search request failed');
            }
            return response.json();
        })
        .then(function(data) {
            isSearching = false;
            // Prefer server-reported time; fall back to client measurement
            var elapsed = (data.data && data.data.search_time_ms != null)
                ? data.data.search_time_ms
                : Math.round(performance.now() - startTime);
            var timeContainer = document.getElementById('search-time-container');
            if (timeContainer) timeContainer.textContent = 'in ' + elapsed + 'ms';

            if (!data.ok || !data.data || !data.data.results || data.data.results.length === 0) {
                // Confirmed: the new filter/sort truly has zero results - now
                // it's safe to clear the old (stale) grid/count.
                if (pendingGridSwap) {
                    pendingGridSwap = false;
                    var staleGrid = document.getElementById('video-grid');
                    if (staleGrid) staleGrid.innerHTML = '';
                    var staleCountEl = document.getElementById('result-count');
                    if (staleCountEl) staleCountEl.textContent = '0';
                }
                hideSearchElement('initial-loading');
                showNoResultsMessage();
                hasMoreResults = false;
                announce(getSearchI18n().noResults || 'No results found');
                updateSearchStatus();
                hideSearchElement('status-bar');
                return;
            }

            // Process results. New data has actually arrived - safe to swap
            // out the old (still-valid) grid contents now.
            if (pendingGridSwap) {
                pendingGridSwap = false;
                var oldGrid = document.getElementById('video-grid');
                if (oldGrid) oldGrid.innerHTML = '';
                var oldCountEl = document.getElementById('result-count');
                if (oldCountEl) oldCountEl.textContent = '0';
            }
            hideSearchElement('initial-loading');
            hideNoResultsMessage();
            showSearchElement('search-meta');
            showSearchElement('filters');

            // Results already deduplicated server-side
            var results = data.data.results;
            for (var i = 0; i < results.length; i++) {
                var r = results[i];
                // Apply min duration filter (client-side additional filter)
                if (minDuration > 0 && r.duration_seconds > 0 && r.duration_seconds < minDuration) {
                    continue;
                }
                allResults.push(r);
                // Track engine for status display
                if (r.source) {
                    enginesWithResults.add(r.source);
                }
                addResultCard(r);

                // Add to source filter if new
                var source = r.source || '';
                if (source && !sourcesSet.has(source)) {
                    sourcesSet.add(source);
                    addSourceCheckbox(source, r.source_display || source);
                }
            }

            var countEl = document.getElementById('result-count');
            if (countEl) countEl.textContent = allResults.length;

            setupInfiniteScroll();
            // Apply default filters from preferences
            applySearchFiltersAndSort();
            announce(allResults.length + ' results found');
            updateSearchStatus();
            hideSearchElement('status-bar');
        })
        .catch(function(err) {
            isSearching = false;
            if (pendingGridSwap) {
                // Background refetch failed - the still-visible old results
                // are still correct, leave them in place; just notify.
                pendingGridSwap = false;
                hideSearchElement('status-bar');
                showToast('Search failed - check your connection', 'error');
                return;
            }
            var loadingEl = document.getElementById('initial-loading');
            if (loadingEl) {
                loadingEl.innerHTML = '<p>Connection error. <button type="button" data-action="reload" class="retry-btn">Retry</button></p>';
            }
            showToast('Search failed - check your connection', 'error');
            updateSearchStatus();
        });
    }

    function addResultCard(r) {
        var grid = document.getElementById('video-grid');
        if (!grid) return;

        var card = document.createElement('article');
        card.className = 'video-card';
        card.setAttribute('role', 'listitem');
        card.setAttribute('aria-label', r.title || 'Video result');

        var duration = r.duration || '';
        if (duration && !duration.includes(':')) {
            var secs = parseInt(duration);
            if (!isNaN(secs)) {
                var mins = Math.floor(secs / 60);
                var s = secs % 60;
                duration = mins + ':' + (s < 10 ? '0' : '') + s;
            }
        }

        card.dataset.source = r.source || '';
        card.dataset.duration = r.duration_seconds || 0;
        card.dataset.views = r.views_count || 0;
        card.dataset.quality = r.quality || '';
        card.dataset.title = (r.title || '').toLowerCase();
        card.dataset.tags = (r.tags || []).join(',').toLowerCase();
        card.dataset.performer = (r.performer || '').toLowerCase();

        var previewUrl = r.preview_url || '';
        var hasPreview = previewUrl && previewUrl.length > 0;
        card.dataset.hasPreview = hasPreview ? '1' : '';
        // Proxy preview URL to avoid CORS issues
        var proxiedPreviewUrl = hasPreview ? '/api/v1/proxy/videos?url=' + encodeURIComponent(previewUrl) : '';
        var downloadUrl = r.download_url || '';
        var hasDownload = downloadUrl && downloadUrl.length > 0;

        // Check open in new tab preference (default true)
        var targetAttr = userPrefs.openNewTab !== false ? ' target="_blank"' : '';
        var html = '<a href="' + escapeHtmlUtil(r.url) + '"' + targetAttr + ' rel="noopener noreferrer nofollow" class="card-link">';
        html += '<div class="thumb-container"' + (hasPreview ? ' data-preview="' + escapeHtmlUtil(proxiedPreviewUrl) + '"' : '') + '>';
        // Proxy thumbnail based on proxyImages preference (default: true for privacy)
        var thumbSrc = '/static/images/placeholder.svg';
        if (r.thumbnail) {
            thumbSrc = userPrefs.proxyImages !== false
                ? '/api/v1/proxy/thumbnails?url=' + encodeURIComponent(r.thumbnail)
                : r.thumbnail;
        }
        html += '<img class="thumb-static" src="' + escapeHtmlUtil(thumbSrc) + '" alt="' + escapeHtmlUtil(r.title) + '" loading="lazy" data-fallback="/static/images/placeholder.svg">';

        if (hasPreview) {
            html += '<video class="thumb-preview" src="' + escapeHtmlUtil(proxiedPreviewUrl) + '" muted loop playsinline preload="none"></video>';
            if (isTouchDevice) {
                html += '<div class="swipe-hint">Swipe to preview</div>';
            }
        }

        if (duration) html += '<span class="duration">' + escapeHtmlUtil(duration) + '</span>';
        if (r.quality) html += '<span class="quality-badge">' + escapeHtmlUtil(r.quality) + '</span>';
        html += '</div></a>';

        // Card menu using HTML5 details/summary (no JS toggle needed)
        html += '<details class="card-menu-container">';
        html += '<summary aria-label="Video options"><svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="5" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="12" cy="19" r="2"/></svg></summary>';
        html += '<div class="card-menu" role="menu">';
        html += '<button type="button" class="card-menu-item" data-action="newtab" data-url="' + escapeHtmlUtil(r.url) + '"><span>Open in new tab</span></button>';
        html += '<button type="button" class="card-menu-item" data-action="copy" data-url="' + escapeHtmlUtil(r.url) + '"><span>Copy link</span></button>';
        html += '<button type="button" class="card-menu-item" data-action="favorite" data-video=\'' + escapeHtmlUtil(JSON.stringify({url: r.url, title: r.title || 'Untitled', thumbnail: r.thumbnail || '', source: r.source || ''})) + '\'><span>Add to favorites</span></button>';
        if (hasDownload) {
            html += '<button type="button" class="card-menu-item" data-action="newtab" data-url="' + escapeHtmlUtil(downloadUrl) + '"><span>Download</span></button>';
        }
        html += '</div></details>';

        html += '<div class="info">';
        html += '<h3><a href="' + escapeHtmlUtil(r.url) + '"' + targetAttr + ' rel="noopener noreferrer nofollow">' + escapeHtmlUtil(r.title || 'Untitled') + '</a></h3>';
        html += '<div class="meta"><span class="source">' + escapeHtmlUtil(r.source_display || r.source || '') + '</span>';
        if (r.views) html += '<span>' + escapeHtmlUtil(r.views) + ' views</span>';
        html += '</div></div>';

        card.innerHTML = html;
        grid.appendChild(card);

        // Setup video preview for this card
        setupSearchCardPreview(card);
        displayedCount++;
    }

    function setupSearchCardPreview(card) {
        var container = card.querySelector('.thumb-container[data-preview]');
        if (!container) return;

        var video = container.querySelector('.thumb-preview');
        var staticImg = container.querySelector('.thumb-static');
        var swipeHint = container.querySelector('.swipe-hint');
        if (!video) return;

        // Check autoplay preference (default true)
        var autoplayEnabled = userPrefs.autoplayPreview !== false;
        var previewDelayRaw = parseInt(userPrefs.previewDelay, 10);
        var previewDelay = isNaN(previewDelayRaw) ? 0 : previewDelayRaw;

        var isPlaying = false;
        var videoFailed = false;
        var hoverTimeout;
        var touchStartX = 0;
        var touchStartY = 0;

        // Handle video load errors - hide preview and remove indicator
        video.addEventListener('error', function() {
            videoFailed = true;
            video.style.display = 'none';
            container.removeAttribute('data-preview');
            if (swipeHint) swipeHint.style.display = 'none';
        });

        if (!isTouchDevice && autoplayEnabled) {
            // Desktop: hover behavior (only if autoplay enabled)
            container.addEventListener('mouseenter', function() {
                if (videoFailed) return;
                hoverTimeout = setTimeout(function() {
                    if (videoFailed) return;
                    video.classList.add('preview-active');
                    staticImg.classList.add('preview-active');
                    video.play().catch(function() {
                        videoFailed = true;
                        video.classList.remove('preview-active');
                        staticImg.classList.remove('preview-active');
                        video.style.display = 'none';
                        container.removeAttribute('data-preview');
                    });
                    isPlaying = true;
                }, previewDelay);
            });

            container.addEventListener('mouseleave', function() {
                clearTimeout(hoverTimeout);
                video.classList.remove('preview-active');
                staticImg.classList.remove('preview-active');
                video.pause();
                video.currentTime = 0;
                isPlaying = false;
            });
        } else if (isTouchDevice) {
            // Mobile: swipe right to preview
            var didSwipe = false;

            container.addEventListener('touchstart', function(e) {
                touchStartX = e.touches[0].clientX;
                touchStartY = e.touches[0].clientY;
                didSwipe = false;
            }, { passive: true });

            container.addEventListener('touchend', function(e) {
                var touchEndX = e.changedTouches[0].clientX;
                var touchEndY = e.changedTouches[0].clientY;
                var deltaX = touchEndX - touchStartX;
                var deltaY = Math.abs(touchEndY - touchStartY);

                // Swipe right detected - start preview
                if (deltaX > 50 && deltaY < 50) {
                    didSwipe = true;
                    e.preventDefault();
                    if (!isPlaying && !videoFailed) {
                        video.classList.add('preview-active');
                        staticImg.classList.add('preview-active');
                        if (swipeHint) swipeHint.classList.add('hidden');
                        video.play().catch(function() {
                            videoFailed = true;
                            video.classList.remove('preview-active');
                            staticImg.classList.remove('preview-active');
                            video.style.display = 'none';
                            container.removeAttribute('data-preview');
                            if (swipeHint) swipeHint.style.display = 'none';
                        });
                        isPlaying = true;

                        // Auto-stop after 8 seconds
                        setTimeout(function() {
                            if (isPlaying) {
                                video.classList.remove('preview-active');
                                staticImg.classList.remove('preview-active');
                                video.pause();
                                video.currentTime = 0;
                                isPlaying = false;
                            }
                        }, 8000);
                    }
                }
                // Swipe left to stop preview
                else if (deltaX < -50 && deltaY < 50 && isPlaying) {
                    didSwipe = true;
                    e.preventDefault();
                    video.classList.remove('preview-active');
                    staticImg.classList.remove('preview-active');
                    video.pause();
                    video.currentTime = 0;
                    isPlaying = false;
                }
            }, { passive: false });

            // Prevent click navigation after swipe or when preview is playing
            container.addEventListener('click', function(e) {
                if (didSwipe) {
                    e.preventDefault();
                    e.stopPropagation();
                    didSwipe = false;
                    return;
                }
                // If preview is playing, stop it instead of navigating
                if (isPlaying) {
                    e.preventDefault();
                    e.stopPropagation();
                    video.classList.remove('preview-active');
                    staticImg.classList.remove('preview-active');
                    video.pause();
                    video.currentTime = 0;
                    isPlaying = false;
                }
            });
        }
    }

    // Duration/quality/sort/source are server-authoritative filters (AI.md
    // PART 16 "JavaScript enhances, it does not enable") - these setters only
    // record the requested value; refetchSearchResults() below is what
    // actually asks the server for the filtered/sorted set. They never
    // filter or reorder already-rendered cards themselves.
    function searchFilterByDuration(value) {
        searchCurrentDurationFilter = value;
    }

    function searchFilterByQuality(value) {
        searchCurrentQualityFilter = value;
    }

    function searchFilterBySource(sources) {
        // sources can be an array or Set of source names
        if (Array.isArray(sources)) {
            searchCurrentSourceFilters = new Set(sources);
        } else if (sources instanceof Set) {
            searchCurrentSourceFilters = sources;
        } else if (sources) {
            searchCurrentSourceFilters = new Set([sources]);
        } else {
            searchCurrentSourceFilters = new Set();
        }
    }

    function addSourceCheckbox(source, displayName) {
        var sourceOptions = document.getElementById('source-options');
        if (!sourceOptions) return;
        var label = document.createElement('label');
        label.className = 'source-option';
        // name="engines" matches the server's <select>/checkbox parsing
        // (handlers.go accepts repeated "engines" values) so this checkbox
        // also submits correctly if the filters form is posted without JS.
        label.innerHTML = '<input type="checkbox" name="engines" value="' + escapeHtmlUtil(source) + '" checked data-action="source-filter-change"><span>' + escapeHtmlUtil(displayName) + '</span>';
        sourceOptions.appendChild(label);
    }

    // toggleSourceFilter removed - now uses HTML5 details/summary

    function toggleAllSources(checked) {
        var checkboxes = document.querySelectorAll('#source-options input[type="checkbox"]');
        checkboxes.forEach(function(cb) { cb.checked = checked; });
        updateSourceFilter();
    }

    function updateSourceFilter() {
        var checkboxes = document.querySelectorAll('#source-options input[type="checkbox"]');
        var allCheckbox = document.getElementById('source-all');
        var selectedSources = [];
        var allChecked = true;

        checkboxes.forEach(function(cb) {
            if (cb.checked) {
                selectedSources.push(cb.value);
            } else {
                allChecked = false;
            }
        });

        // Update "All Sources" checkbox state
        if (allCheckbox) {
            allCheckbox.checked = allChecked;
        }

        // Update label
        var label = document.getElementById('source-filter-label');
        if (label) {
            if (allChecked || selectedSources.length === 0) {
                label.textContent = 'All Sources';
            } else if (selectedSources.length === 1) {
                label.textContent = selectedSources[0];
            } else {
                label.textContent = selectedSources.length + ' sources';
            }
        }

        // Empty set = query all engines again
        searchCurrentSourceFilters = allChecked ? new Set() : new Set(selectedSources);
        updateFilterCount();
        refetchSearchResults();
    }

    function updatePreviewFirst(checked) {
        searchPreviewFirst = checked;
        // Preview-first is a display-order priority, not a data filter - it
        // mirrors the server's own previewFirst stream ordering, so this can
        // stay a pure client-side reorder of the already-rendered cards.
        applySearchFiltersAndSort();
        updateFilterCount();
    }

    function searchSortResults(value) {
        searchCurrentSort = value;
    }

    // Re-runs the search against the server with the current
    // duration/quality/sort/source filter state (a JS-enhanced equivalent of
    // submitting #filters-form and letting the browser reload /search with
    // the same query params - see AI.md PART 16). Never computes the
    // filtered/sorted result set itself.
    function refetchSearchResults() {
        if (!searchQuery) return;

        var params = new URLSearchParams(window.location.search);
        if (searchCurrentDurationFilter) params.set('duration', searchCurrentDurationFilter); else params.delete('duration');
        if (searchCurrentQualityFilter) params.set('quality', searchCurrentQualityFilter); else params.delete('quality');
        if (searchCurrentSort) params.set('sort', searchCurrentSort); else params.delete('sort');
        params.delete('engines');
        if (searchCurrentSourceFilters.size > 0) {
            searchCurrentSourceFilters.forEach(function(s) { params.append('engines', s); });
        }
        history.replaceState(null, '', window.location.pathname + '?' + params.toString());

        if (infiniteScrollObserver) {
            infiniteScrollObserver.disconnect();
            infiniteScrollObserver = null;
        }
        allResults = [];
        sourcesSet = new Set();
        enginesWithResults = new Set();
        enginesWithResultsFinal = null;
        enginesTotal = 0;
        enginesCompleted = 0;
        hasMoreResults = true;
        currentPage = 1;
        isLoadingMore = false;
        isSearching = true;
        searchSessionID = generateSearchSessionID();

        // Do NOT clear #video-grid/#result-count or show the full-page
        // #initial-loading spinner here - the currently displayed results are
        // still correct and usable (AI.md PART 16: JS enhances only, never
        // blocks/replaces working content). Leave them in place and only
        // swap them out once the background update actually has new data
        // (see the firstResult/success branches in streamResults() and
        // fallbackToJSONSearch()). Surface the in-progress update via the
        // small, non-blocking status bar instead of a full-page spinner.
        pendingGridSwap = true;
        var statusText = document.getElementById('status-text');
        if (statusText) statusText.textContent = 'Updating results...';
        showSearchElement('status-bar');

        var minDuration = parseInt(userPrefs.minDuration) || 0;
        if (searchCurrentSort) {
            // Global sort requires the synchronous JSON API - the SSE stream
            // only orders results within each engine's own batch (see
            // SearchStreamWithOperators, which has no SortBy parameter).
            fallbackToJSONSearch(minDuration);
        } else {
            streamResults(minDuration);
        }
    }

    // Preview-first-only reorder of the already-rendered cards. Duration/
    // quality/source/sort are never applied here - those are always fetched
    // fresh from the server via refetchSearchResults().
    function applySearchFiltersAndSort() {
        var grid = document.getElementById('video-grid');
        if (!grid) return;
        if (!searchPreviewFirst) return;

        var cardArray = Array.from(grid.querySelectorAll('.video-card'));
        cardArray.sort(function(a, b) {
            var aHasPreview = a.dataset.hasPreview ? 1 : 0;
            var bHasPreview = b.dataset.hasPreview ? 1 : 0;
            return bHasPreview - aHasPreview; // Preview videos first
        });
        cardArray.forEach(function(card) { grid.appendChild(card); });
    }

    function updateSearchStatus() {
        var statusText = document.getElementById('status-text');
        var engineStatus = document.getElementById('engine-status');

        // Load min duration from prefs for display
        var prefs = {};
        try {
            prefs = JSON.parse(localStorage.getItem('vidveil_prefs') || '{}');
        } catch (e) {}
        var minDuration = parseInt(prefs.minDuration) || 0;

        if (isSearching) {
            // Still streaming: no authoritative server count yet, so this is a
            // live client-side tally (progress indicator only, not the final claim).
            if (statusText) statusText.textContent = allResults.length + ' results (streaming...)';
            if (engineStatus) engineStatus.textContent = enginesWithResults.size + ' engines responding';
        } else {
            var msg = allResults.length + ' results found';
            if (minDuration > 0) {
                msg += ' (min ' + Math.floor(minDuration / 60) + ' min)';
            }
            if (statusText) statusText.textContent = msg;
            // Final state: use the server-authoritative counts from the SSE
            // 'done' payload, not the client-side tally, per PART 14's
            // server-side-processing philosophy.
            if (engineStatus) {
                if (enginesWithResultsFinal != null && enginesTotal > 0) {
                    engineStatus.textContent = enginesWithResultsFinal + ' of ' + enginesTotal + ' engines had results';
                } else {
                    engineStatus.textContent = enginesWithResults.size + ' engines';
                }
            }
        }
    }

    // Progressive enhancement for server-rendered related searches.
    // The tags are already in the DOM from the server. This just adds a
    // "Show more" toggle button if there are hidden items (class related-tag--hidden).
    function initRelatedSearchesToggle() {
        var tagsContainer = document.getElementById('related-tags');
        if (!tagsContainer) return;

        var hiddenTags = tagsContainer.querySelectorAll('.related-tag--hidden');
        if (hiddenTags.length === 0) return;

        var showMoreBtn = document.createElement('button');
        showMoreBtn.type = 'button';
        showMoreBtn.className = 'related-show-more';
        showMoreBtn.innerHTML = '<span>+' + hiddenTags.length + ' more</span>';
        showMoreBtn.onclick = function() {
            tagsContainer.classList.toggle('related-tags--expanded');
            var all = tagsContainer.querySelectorAll('.related-tag--hidden');
            if (tagsContainer.classList.contains('related-tags--expanded')) {
                showMoreBtn.innerHTML = '<span>Show less</span>';
                all.forEach(function(t) { t.classList.add('related-tag--visible'); });
            } else {
                showMoreBtn.innerHTML = '<span>+' + all.length + ' more</span>';
                all.forEach(function(t) { t.classList.remove('related-tag--visible'); });
            }
        };
        tagsContainer.appendChild(showMoreBtn);
    }

    // Infinite scroll - loads more pages as user scrolls.
    // Server-authoritative per IDEA.md "Search Settings": the server always
    // decides page size/content; JS only decides *when* to request the next
    // page, and auto-fetches by default (results_per_page cookie "0",
    // "Infinite scroll") unless the visitor picks 20/50/100 in Preferences.
    // In that case the server renders real Prev/Next links
    // (search.tmpl #pagination-container) and this function returns
    // immediately without touching them.
    function setupInfiniteScroll() {
        var resultsPerPage = parseInt((userPrefs && userPrefs.resultsPerPage) ?? 0, 10);
        if (resultsPerPage !== 0) {
            return;
        }

        var grid = document.getElementById('video-grid');
        if (!grid || infiniteScrollObserver) return;

        // Create sentinel element at end of grid
        var sentinel = document.createElement('div');
        sentinel.className = 'infinite-scroll-sentinel';
        sentinel.id = 'scroll-sentinel';
        grid.parentNode.insertBefore(sentinel, grid.nextSibling);

        // Create load more indicator
        var loadIndicator = document.createElement('div');
        loadIndicator.className = 'load-more-indicator hidden';
        loadIndicator.id = 'load-more-indicator';
        loadIndicator.innerHTML = '<div class="spinner"></div><span>Loading more results...</span>';
        grid.parentNode.insertBefore(loadIndicator, sentinel);

        // Setup intersection observer
        infiniteScrollObserver = new IntersectionObserver(function(entries) {
            entries.forEach(function(entry) {
                if (entry.isIntersecting && !isLoadingMore && hasMoreResults && !isSearching) {
                    loadMoreResults();
                }
            });
        }, {
            rootMargin: '200px' // Start loading 200px before sentinel is visible
        });

        infiniteScrollObserver.observe(sentinel);
    }

    function loadMoreResults() {
        if (isLoadingMore || !hasMoreResults) return;

        isLoadingMore = true;
        currentPage++;

        var loadIndicator = document.getElementById('load-more-indicator');
        if (loadIndicator) loadIndicator.classList.remove('hidden');

        // Stream next page of results (include AI, quality, preview, and engine preferences)
        var pageUrl = '/api/v1/search?q=' + encodeURIComponent(searchQuery) + '&page=' + currentPage + '&session=' + encodeURIComponent(searchSessionID);
        if (userPrefs.showAIContent) {
            pageUrl += '&show_ai=1';
        }
        if (userPrefs.minQuality && parseInt(userPrefs.minQuality) > 0) {
            pageUrl += '&min_quality=' + userPrefs.minQuality;
        }
        if (searchPreviewFirst) {
            pageUrl += '&preview_first=1';
        }
        if (userPrefs.enabledEngines && userPrefs.enabledEngines.length > 0) {
            pageUrl += '&engines=' + encodeURIComponent(userPrefs.enabledEngines.join(','));
        }
        if (userPrefs.minDuration && parseInt(userPrefs.minDuration) > 0) {
            pageUrl += '&min_duration=' + parseInt(userPrefs.minDuration);
        }
        var eventSource = new EventSource(pageUrl);
        activeEventSource = eventSource;
        var gotResults = false;
        var streamDone = false;

        eventSource.onmessage = function(event) {
            var data = JSON.parse(event.data);

            // Final done message
            if (data.done && data.engine === 'all') {
                streamDone = true;
                eventSource.close();
                isLoadingMore = false;
                if (loadIndicator) loadIndicator.classList.add('hidden');

                // If no results on this page, stop infinite scroll
                if (!gotResults) {
                    hasMoreResults = false;
                    currentPage--; // revert so a retry would re-try same page
                    var sentinel = document.getElementById('scroll-sentinel');
                    if (sentinel) {
                        if (infiniteScrollObserver) infiniteScrollObserver.unobserve(sentinel);
                        // Replace sentinel with end-of-results message
                        var endMsg = document.createElement('p');
                        endMsg.className = 'no-more-results';
                        endMsg.textContent = 'No more results';
                        sentinel.replaceWith(endMsg);
                    }
                } else {
                    // New page appended raw via addResultCard() above —
                    // re-apply preview-first / user sort so infinite-scroll
                    // pages stay ordered the same way the initial page is
                    // (IDEA.md "Preview First (toggle)" / "Client-Side Sorting").
                    applySearchFiltersAndSort();
                }
                updateSearchStatus();
                return;
            }

            // Skip done/error from individual engines
            if (data.done || data.error) return;

            // Got a result (already deduplicated server-side)
            if (data.result && data.result.title) {
                gotResults = true;
                var r = data.result;

                allResults.push(r);
                // Track engine for status display
                if (data.engine) {
                    enginesWithResults.add(data.engine);
                }
                addResultCard(r);

                var countEl = document.getElementById('result-count');
                if (countEl) countEl.textContent = allResults.length;
            }
        };

        eventSource.onerror = function(event) {
            // EventSource fires onerror when server closes connection (normal after done)
            if (streamDone) {
                eventSource.close();
                return;
            }
            eventSource.close();
            isLoadingMore = false;
            if (loadIndicator) loadIndicator.classList.add('hidden');

            // A named "event: error" SSE frame from the server (the
            // RATE_LIMITED overload signal) dispatches here with a data
            // payload; a plain transport drop has no data. Surface the
            // overload distinctly and revert the page counter so the next
            // scroll retries this same page.
            if (event && typeof event.data === 'string') {
                currentPage--;
                var overloaded = false;
                try {
                    overloaded = JSON.parse(event.data).error === 'RATE_LIMITED';
                } catch (e) {
                    overloaded = false;
                }
                showToast(overloaded ? 'Search is busy — please try again shortly' : 'Failed to load more results', 'warning');
                return;
            }

            // Transport closed without the final done frame. If this page
            // already streamed results, finish gracefully instead of showing
            // the false-positive "error loading results" at end of stream —
            // keep the page counter so the next scroll requests the next page
            // (server session dedup prevents repeats either way).
            if (gotResults) {
                applySearchFiltersAndSort();
                updateSearchStatus();
                return;
            }

            // No results and no done frame: a real failure — revert the page
            // counter so the next scroll retries this same page.
            currentPage--;
            // Show subtle error - don't block the user
            showToast('Failed to load more results', 'warning');
        };
    }

    function hideSearchElement(id) {
        var el = document.getElementById(id);
        if (el) el.classList.add('hidden');
    }

    function showSearchElement(id) {
        var el = document.getElementById(id);
        if (el) el.classList.remove('hidden');
    }

    function saveSearchPageHistory(q) {
        if (!q || q.trim() === '') return;
        var key = 'vidveil_history';
        var history = [];
        try {
            history = JSON.parse(localStorage.getItem(key) || '[]');
        } catch (e) {}

        // Remove if already exists
        history = history.filter(function(h) { return h.query !== q; });

        // Add to beginning
        history.unshift({ query: q, timestamp: Date.now() });

        // Keep only last 50
        if (history.length > 50) history = history.slice(0, 50);

        try {
            localStorage.setItem(key, JSON.stringify(history));
        } catch (e) {}
    }

    // Export search functions
    window.initSearchPage = initSearchPage;
    window.Vidveil = window.Vidveil || {};
    window.Vidveil.Search = {
        filterByDuration: searchFilterByDuration,
        filterByQuality: searchFilterByQuality,
        filterBySource: searchFilterBySource,
        sortResults: searchSortResults,
        toggleAllSources: toggleAllSources,
        updateSourceFilter: updateSourceFilter,
        updatePreviewFirst: updatePreviewFirst,
        refetch: refetchSearchResults
    };
    window.filterByDuration = searchFilterByDuration;
    window.filterByQuality = searchFilterByQuality;
    window.filterBySource = searchFilterBySource;
    window.sortResults = searchSortResults;
    window.toggleAllSources = toggleAllSources;
    window.updateSourceFilter = updateSourceFilter;
    window.updatePreviewFirst = updatePreviewFirst;

    // Close source filter dropdown when clicking outside (details element)
    document.addEventListener('click', function(e) {
        var wrapper = document.getElementById('source-filter-wrapper');
        if (wrapper && wrapper.open && !wrapper.contains(e.target)) {
            wrapper.removeAttribute('open');
        }
    });

    // Close any in-flight SSE connection before the page is torn down/hidden.
    // A still-open EventSource makes the search page bfcache-ineligible, so
    // pressing back forces a full network reload (visible as a results
    // "reload" or a transient "no results found" flash) instead of an
    // instant restore of the already-correct previous page state.
    window.addEventListener('pagehide', function() {
        if (activeEventSource) {
            activeEventSource.close();
            activeEventSource = null;
        }
    });
})();

// ============================================================================
// Shared Utility Functions
// ============================================================================
function escapeHtmlUtil(str) {
    if (!str) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// ============================================================================
// Card Menu Functions - uses HTML5 details/summary, minimal JS
// ============================================================================

// Close all open card menus (details elements)
function closeAllCardMenus() {
    document.querySelectorAll('.card-menu-container[open]').forEach(function(d) {
        d.removeAttribute('open');
    });
}

// Delegated event handler for card menu actions
document.addEventListener('click', function(e) {
    // Generic data-action dispatch (CSP-safe replacement for inline onclick).
    // Returns true when it handled the action; card-menu actions (newtab/copy/
    // favorite) are not claimed here and fall through to the logic below.
    var actionEl = e.target.closest('[data-action]');
    if (actionEl && dispatchClickAction(actionEl, e)) {
        return;
    }

    var btn = e.target.closest('.card-menu-item');
    if (!btn) {
        // Click outside - close all menus
        if (!e.target.closest('.card-menu-container')) {
            closeAllCardMenus();
        }
        return;
    }

    var action = btn.dataset.action;
    var details = btn.closest('.card-menu-container');

    if (action === 'newtab') {
        window.open(btn.dataset.url, '_blank', 'noopener,noreferrer');
    } else if (action === 'copy') {
        var url = btn.dataset.url;
        navigator.clipboard.writeText(url).then(function() {
            showNotification('Link copied to clipboard', 'success');
        }).catch(function() {
            // Fallback for older browsers
            var input = document.createElement('input');
            input.value = url;
            document.body.appendChild(input);
            input.select();
            document.execCommand('copy');
            document.body.removeChild(input);
            showNotification('Link copied to clipboard', 'success');
        });
    } else if (action === 'favorite') {
        var videoData = JSON.parse(btn.dataset.video);
        var span = btn.querySelector('span');
        window.Vidveil.Favorites.toggle(videoData).then(function(added) {
            showNotification(added ? 'Added to favorites' : 'Removed from favorites', added ? 'success' : 'info');
            if (span) {
                span.textContent = added ? 'Remove from favorites' : 'Add to favorites';
            }
        });
    }

    // Close menu after action
    if (details) details.removeAttribute('open');
});

// Update favorite button text when menu opens
document.addEventListener('toggle', function(e) {
    if (!e.target.matches('.card-menu-container') || !e.target.open) return;

    // Close other open menus
    document.querySelectorAll('.card-menu-container[open]').forEach(function(d) {
        if (d !== e.target) d.removeAttribute('open');
    });

    // Update favorite button text
    var favBtn = e.target.querySelector('[data-action="favorite"] span');
    if (favBtn) {
        var videoData = JSON.parse(e.target.querySelector('[data-action="favorite"]').dataset.video);
        window.Vidveil.Favorites.ensureLoaded().then(function() {
            favBtn.textContent = window.Vidveil.Favorites.isFavorite(videoData.url) ? 'Remove from favorites' : 'Add to favorites';
        });
    }
}, true);

// ============================================================================
// Export for global access
// ============================================================================
window.Vidveil = window.Vidveil || {};
Object.assign(window.Vidveil, {
    setTheme: setTheme,
    getTheme: getTheme,
    getEffectiveTheme: getEffectiveTheme,
    getPreferences: getPreferences,
    savePreferences: savePreferences,
    resetPreferences: resetPreferences,
    selectAllEngines: selectAllEngines,
    selectNoneEngines: selectNoneEngines,
    selectTier: selectTier,
    updateSort: updateSort,
    filterBySource: filterBySource,
    filterByDuration: filterByDuration,
    showNotification: showNotification,
    fetchAPI: fetchAPI,
    toggleNav: toggleNav,
    closeNav: closeNav,
    announce: announce
});

// Make nav functions globally available for onclick handlers
window.toggleNav = toggleNav;
window.closeNav = closeNav;

// Export admin functions globally
window.toggleSection = toggleSection;
window.showToast = showToast;
window.showSuccess = showSuccess;
window.showError = showError;
window.showWarning = showWarning;
window.showInfo = showInfo;
window.showConfirm = showConfirm;
window.handleDownloadClick = handleDownloadClick;
window.announce = announce;

// PWA Service Worker Registration (AI.md PART 16)
if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js', { scope: '/' })
        .catch(function() {});
}

// Offline / online indicator (AI.md PART 16)
window.addEventListener('online', function() {
    var indicator = document.getElementById('offline-indicator');
    if (indicator) { indicator.hidden = true; }
});
window.addEventListener('offline', function() {
    var indicator = document.getElementById('offline-indicator');
    if (!indicator) {
        indicator = document.createElement('div');
        indicator.id = 'offline-indicator';
        indicator.className = 'offline-banner';
        indicator.textContent = 'You are offline. Some features may be unavailable.';
        document.body.insertBefore(indicator, document.body.firstChild);
    }
    indicator.hidden = false;
});

// Admin globals — read from data attributes on <body> (set by admin layout template)
(function() {
    var body = document.body;
    if (!body) { return; }
    var apiBase = body.getAttribute('data-api-base');
    var adminPath = body.getAttribute('data-admin-path');
    if (apiBase)   { window.API_BASE   = apiBase; }
    if (adminPath) { window.ADMIN_PATH = adminPath; }
}());

// Copy button handler (AI.md PART 16: code-block copy button)
// Handles .copy-btn elements with data-copy attribute or preceding .code-content sibling
document.addEventListener('click', function(e) {
    var btn = e.target.closest('.copy-btn');
    if (!btn) { return; }
    var text = btn.dataset.copy;
    if (!text) {
        var prev = btn.previousElementSibling;
        if (prev) { text = prev.textContent; }
    }
    if (!text) { return; }
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(function() {
            var icon = btn.querySelector('.copy-icon');
            if (icon) {
                var orig = icon.textContent;
                icon.textContent = '✓';
                btn.classList.add('copied');
                setTimeout(function() {
                    icon.textContent = orig;
                    btn.classList.remove('copied');
                }, 2000);
            } else {
                btn.classList.add('copied');
                setTimeout(function() { btn.classList.remove('copied'); }, 2000);
            }
        }).catch(function() {});
    } else {
        var ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand('copy'); } catch(ex) {}
        document.body.removeChild(ta);
        btn.classList.add('copied');
        setTimeout(function() { btn.classList.remove('copied'); }, 2000);
    }
});

// PWA install prompt (AI.md PART 16)
var deferredPrompt = null;
window.addEventListener('beforeinstallprompt', function(e) {
    e.preventDefault();
    deferredPrompt = e;
    var installBtn = document.getElementById('pwa-install-btn');
    if (installBtn) { installBtn.hidden = false; }
});
window.addEventListener('appinstalled', function() {
    deferredPrompt = null;
    var installBtn = document.getElementById('pwa-install-btn');
    if (installBtn) { installBtn.hidden = true; }
});

// App update notification (AI.md PART 16: service worker update flow)
function showUpdateNotification() {
    if (document.getElementById('update-banner')) { return; }
    var banner = document.createElement('div');
    banner.id = 'update-banner';
    banner.className = 'update-banner';
    banner.innerHTML = '<span>A new version is available</span>'
        + '<button type="button" data-action="update-app" class="btn btn-primary btn-sm">Update Now</button>'
        + '<button type="button" data-action="dismiss-update" class="btn btn-secondary btn-sm">Later</button>';
    document.body.appendChild(banner);
}

function updateApp() {
    navigator.serviceWorker.ready.then(function(reg) {
        if (reg.waiting) {
            reg.waiting.postMessage({ type: 'SKIP_WAITING' });
        }
    });
    navigator.serviceWorker.addEventListener('controllerchange', function() {
        window.location.reload();
    });
}

// Check for SW updates and notify user (AI.md PART 16)
if ('serviceWorker' in navigator) {
    navigator.serviceWorker.ready.then(function(reg) {
        reg.addEventListener('updatefound', function() {
            var newWorker = reg.installing;
            if (!newWorker) { return; }
            newWorker.addEventListener('statechange', function() {
                if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
                    showUpdateNotification();
                }
            });
        });
    });
}

window.showUpdateNotification = showUpdateNotification;
window.updateApp = updateApp;

// ============================================================================
// Announcement Banner Dismissal (AI.md PART 16)
// Enhancement only — the POST form works without JS (server appends id to cookie)
// ============================================================================
(function() {
    'use strict';
    // Intercept dismiss forms to avoid a full page reload while keeping the
    // dismissed_announcements cookie so the server never renders them again.
    // Dismissal is keyed on the announcement id — changing the id resets dismissals.
    document.querySelectorAll('.site-banner .site-banner-dismiss').forEach(function(form) {
        form.addEventListener('submit', function(event) {
            event.preventDefault();
            var banner = form.closest('.site-banner');
            if (!banner) return;
            var id = banner.dataset.announcementId;
            if (!id) return;
            var match = document.cookie.match(/(?:^|;\s*)dismissed_announcements=([^;]*)/);
            var ids = match ? decodeURIComponent(match[1]).split(',').filter(Boolean) : [];
            if (ids.indexOf(id) === -1) {
                ids.push(id);
            }
            document.cookie = 'dismissed_announcements=' + encodeURIComponent(ids.join(',')) +
                '; path=/; max-age=31536000; SameSite=Lax';
            banner.remove();
        });
    });
})();

// ============================================================================
// CSP-safe delegated dispatchers (AI.md PART 11: script-src 'self')
// All inline event handlers were migrated here. Elements carry data-action
// (plus data-* args) and are wired through these document-level listeners.
// ============================================================================

// Run fn once the DOM is ready (app.js loads at end of <body>).
function onReady(fn) {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', fn);
    } else {
        fn();
    }
}

// Where to send the user after closing/saving preferences — the page they
// arrived from, captured server-side (Referer, validated by safeReturnPath
// in handlers.go) and threaded onto #preferences-form as data-return-to.
// Deterministic by design: window.history.back() was tried here previously
// but silently did nothing useful when history.length was 1 (direct URL,
// bookmark, new tab) or when the prior entry was /preferences itself.
function preferencesReturnTo() {
    var form = document.getElementById('preferences-form');
    var target = form && form.dataset.returnTo;
    return target || '/';
}

// Delegated click actions. Returns true when handled. Card-menu actions
// (newtab/copy/favorite) are intentionally not claimed here so the existing
// card-menu dispatcher handles them.
function dispatchClickAction(el, e) {
    switch (el.dataset.action) {
        case 'leave-site':
            window.location.href = 'https://www.google.com';
            return true;
        case 'close-dialog': {
            var dlg = el.closest('dialog');
            if (dlg) dlg.close();
            return true;
        }
        case 'toggle-nav':
            toggleNav();
            return true;
        case 'close-nav':
            closeNav();
            return true;
        case 'close-toast': {
            var toast = el.closest('.toast');
            if (toast) toast.remove();
            return true;
        }
        case 'reload':
            location.reload();
            return true;
        case 'update-app':
            updateApp();
            return true;
        case 'dismiss-update':
            if (el.parentElement) el.parentElement.remove();
            return true;
        case 'home-clear-history':
            if (window.Vidveil && window.Vidveil.Home) window.Vidveil.Home.clearHistory();
            return true;
        case 'home-remove-history':
            e.preventDefault();
            if (window.Vidveil && window.Vidveil.Home) window.Vidveil.Home.removeFromHistory(el.dataset.query || '');
            return true;
        case 'search-spinner':
            // On a history <a>: show spinner, allow navigation to continue.
            showSearchSpinner(el, e);
            return true;
        case 'close-prefs':
            window.location.href = preferencesReturnTo();
            return true;
        case 'export-history':
            if (typeof window.exportHistory === 'function') window.exportHistory();
            return true;
        case 'clear-history':
            if (typeof window.clearHistory === 'function') window.clearHistory();
            return true;
        case 'select-all-engines':
            if (typeof window.selectAllEngines === 'function') window.selectAllEngines();
            return true;
        case 'select-none-engines':
            if (typeof window.selectNoneEngines === 'function') window.selectNoneEngines();
            return true;
        case 'reset-preferences':
            if (typeof window.resetPreferences === 'function') window.resetPreferences();
            return true;
        case 'copy-field': {
            var targetId = el.dataset.target;
            var field = targetId ? document.getElementById(targetId) : null;
            var copyMsg = 'Copied!';
            var copyIsland = document.getElementById('preferences-export-i18n');
            if (copyIsland) {
                try { copyMsg = JSON.parse(copyIsland.textContent).copied || copyMsg; } catch (e) {}
            }
            if (field && navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(field.value).then(function() {
                    showNotification(copyMsg, 'success');
                });
            } else if (field) {
                field.select();
            }
            return true;
        }
        default:
            return false;
    }
}

// Delegated change actions (no pre-existing change dispatcher to extend).
document.addEventListener('change', function(e) {
    var el = e.target.closest('[data-action]');
    if (!el) return;
    switch (el.dataset.action) {
        case 'filter-change':
            handleFilterChange();
            break;
        case 'preview-first':
            updatePreviewFirst(el.checked);
            break;
        case 'toggle-all-sources':
            toggleAllSources(el.checked);
            break;
        case 'source-filter-change':
            updateSourceFilter();
            break;
        case 'import-history':
            if (typeof window.importHistory === 'function') window.importHistory(el.files[0]);
            break;
    }
});

// Reads the #search-i18n data island rendered by search.tmpl.
function getSearchI18n() {
    var el = document.getElementById('search-i18n');
    if (!el) return {};
    try {
        return JSON.parse(el.textContent || '{}');
    } catch (e) {
        return {};
    }
}

// Shows the in-body "no results" state. Reuses the server-rendered
// #no-results-message element when present (initial zero-result page load);
// creates it in place, right after #search-meta, when a later client-side
// re-search (filter/sort change) finds zero results and the element was
// never server-rendered. Always lands inside <main>, never near the footer.
function showNoResultsMessage() {
    var i18n = getSearchI18n();
    var msg = i18n.noResults || 'No results found';
    var el = document.getElementById('no-results-message');
    if (!el) {
        el = document.createElement('p');
        el.id = 'no-results-message';
        el.className = 'no-results';
        el.setAttribute('role', 'status');
        el.setAttribute('aria-live', 'polite');
        var meta = document.getElementById('search-meta');
        if (meta && meta.parentNode) {
            meta.parentNode.insertBefore(el, meta.nextSibling);
        } else {
            var main = document.getElementById('main-content');
            if (main) main.appendChild(el);
        }
    }
    el.textContent = msg;
    el.classList.remove('hidden');
}

// Hides/clears the "no results" state left over from a previous search.
function hideNoResultsMessage() {
    var el = document.getElementById('no-results-message');
    if (el) el.classList.add('hidden');
}

// Delegated submit actions.
function getFavoritesI18n() {
    var el = document.getElementById('favorites-i18n') || document.getElementById('preferences-i18n');
    if (!el) return {};
    try {
        return JSON.parse(el.textContent || '{}');
    } catch (e) {
        return {};
    }
}

function updateFavoritesCountLabel(count) {
    var i18n = getFavoritesI18n();
    // /favorites page label.
    var label = document.getElementById('favorites-count-label');
    if (label) {
        label.textContent = count === 1 ? (i18n.countSingular || i18n.favCountSingular || String(count)) :
            (i18n.countPlural || '%d').replace('%d', count);
    }
    // Preferences page count span (server-supplied data-format, e.g. "%d favorites").
    var prefsCount = document.getElementById('favorites-count');
    if (prefsCount) {
        var fmt = prefsCount.getAttribute('data-format') || '%d';
        prefsCount.textContent = fmt.replace('%d', count);
    }
}

// Rebuilds the /favorites page grid from the resolved favorites list.
// Called after ensureLoaded() so entries restored from the localStorage
// mirror (server/DB-wipe recovery, see the Favorites module comment above)
// appear immediately instead of only after a manual page reload — the SSR
// grid was built from the DB state before restoreMissing() ran.
function renderFavoritesGrid(list) {
    var grid = document.getElementById('favorites-grid');
    if (!grid) {
        return;
    }
    list = list || [];

    // Skip the rebuild when the server-rendered grid already matches —
    // avoids flicker (thumbnail reload, lost scroll position) on the common
    // case where nothing needed restoring.
    var rendered = {};
    var renderedCount = 0;
    grid.querySelectorAll('[data-fav-id]').forEach(function(card) {
        rendered[card.getAttribute('data-fav-id')] = true;
        renderedCount++;
    });
    var same = list.length === renderedCount &&
        list.every(function(f) { return rendered[String(f.id)]; });
    if (same) {
        return;
    }

    var i18n = getFavoritesI18n();
    var removeLabel = i18n.remove || 'Remove from favorites';
    var csrf = getCsrfToken();
    grid.innerHTML = '';
    list.forEach(function(f) {
        var card = document.createElement('div');
        card.className = 'video-card';
        card.setAttribute('role', 'listitem');
        card.setAttribute('data-source', f.source || '');
        card.setAttribute('data-fav-id', f.id);
        var html = '<a href="' + escapeHtmlUtil(f.url) + '" rel="noopener noreferrer nofollow" aria-label="' +
            escapeHtmlUtil((f.title || 'Untitled') + ' - ' + (f.source || '')) + '">';
        html += '<img src="/api/v1/proxy/thumbnails?url=' + encodeURIComponent(f.thumbnail || '') + '" alt="' +
            escapeHtmlUtil(f.title || 'Untitled') + '" loading="lazy">';
        html += '<div class="video-info"><h3 class="video-title">' + escapeHtmlUtil(f.title || 'Untitled') +
            '</h3><p class="video-source">' + escapeHtmlUtil(f.source || '') + '</p></div></a>';
        html += '<form action="/favorites" method="post" class="favorite-remove-form" data-action="remove-fav-form">';
        if (csrf) {
            html += '<input type="hidden" name="csrf_token" value="' + escapeHtmlUtil(csrf) + '">';
        }
        html += '<input type="hidden" name="_method" value="DELETE">';
        html += '<input type="hidden" name="id" value="' + escapeHtmlUtil(String(f.id)) + '">';
        html += '<button type="submit" class="video-card-fav-btn video-card-fav-btn--active" data-fav-id="' +
            escapeHtmlUtil(String(f.id)) + '" aria-label="' + escapeHtmlUtil(removeLabel) + '" title="' +
            escapeHtmlUtil(removeLabel) + '">&#9733;</button></form>';
        card.innerHTML = html;
        grid.appendChild(card);
    });

    var empty = document.getElementById('favorites-empty');
    if (empty) {
        if (list.length) {
            empty.style.display = 'none';
        } else {
            empty.style.removeProperty('display');
        }
    }
    updateFavoritesCountLabel(list.length);
}
window.renderFavoritesGrid = renderFavoritesGrid;

document.addEventListener('submit', function(e) {
    var el = e.target.closest('[data-action]');
    if (!el) return;
    if (el.dataset.action === 'search-submit') {
        if (handleSearchSubmit(el) === false) {
            e.preventDefault();
        }
    }
    if (el.dataset.action === 'filters-submit') {
        // #filters-form (search-results page only) posts GET /search with
        // duration/quality/sort/engines - works with JS disabled. With JS,
        // intercept and re-fetch via the same server-authoritative path
        // (refetchSearchResults) instead of a full page reload.
        if (window.Vidveil && window.Vidveil.Search && window.Vidveil.Search.refetch) {
            e.preventDefault();
            window.Vidveil.Search.refetch();
        }
    }
    if (el.dataset.action === 'remove-fav-form') {
        // Server-rendered form works without JS via a full POST + redirect.
        // With JS, intercept and remove the card in place (no reload).
        e.preventDefault();
        var idInput = el.querySelector('input[name="id"]');
        var id = idInput ? idInput.value : '';
        if (!id) {
            el.submit();
            return;
        }
        window.Vidveil.Favorites.removeById(id).then(function() {
            var card = el.closest('.video-card');
            if (card) card.remove();
            var grid = document.getElementById('favorites-grid');
            var count = grid ? grid.querySelectorAll('.video-card').length : 0;
            updateFavoritesCountLabel(count);
            if (grid && count === 0) {
                var empty = document.getElementById('favorites-empty');
                if (empty) empty.style.removeProperty('display');
            }
        }).catch(function() {
            el.submit();
        });
    }
    if (el.dataset.action === 'favorite-toggle-form') {
        // Server-rendered form on the search results page works without JS
        // via a full POST + redirect back to the same results page. With JS,
        // intercept and toggle the star in place (no reload).
        e.preventDefault();
        var urlInput = el.querySelector('input[name="url"]');
        var titleInput = el.querySelector('input[name="title"]');
        var thumbInput = el.querySelector('input[name="thumbnail"]');
        var sourceInput = el.querySelector('input[name="source"]');
        if (!urlInput) {
            el.submit();
            return;
        }
        var video = {
            url: urlInput.value,
            title: titleInput ? titleInput.value : '',
            thumbnail: thumbInput ? thumbInput.value : '',
            source: sourceInput ? sourceInput.value : ''
        };
        var btn = el.querySelector('button[type="submit"]');
        window.Vidveil.Favorites.toggle(video).then(function(added) {
            var i18n = getFavoritesI18n();
            if (btn) {
                btn.classList.toggle('video-card-fav-btn--active', added);
                var label = added ? (i18n.remove || 'Remove from favorites') : (i18n.add || 'Add to favorites');
                btn.setAttribute('aria-label', label);
                btn.setAttribute('title', label);
            }
            var methodInput = el.querySelector('input[name="_method"]');
            if (added) {
                if (!methodInput) {
                    methodInput = document.createElement('input');
                    methodInput.type = 'hidden';
                    methodInput.name = '_method';
                    el.appendChild(methodInput);
                }
                methodInput.value = 'DELETE';
            } else if (methodInput) {
                methodInput.remove();
            }
        }).catch(function() {
            el.submit();
        });
    }
    if (el.dataset.action === 'clear-favs-form' || el.dataset.action === 'clear-favorites-form') {
        e.preventDefault();
        var message = el.getAttribute('data-confirm') || 'Remove all favorites?';
        showConfirm(message, function() {
            window.Vidveil.Favorites.clear().then(function() {
                var i18n = getFavoritesI18n();
                showSuccess(i18n.cleared || i18n.favCleared || 'Favorites cleared');
                var grid = document.getElementById('favorites-grid');
                if (grid) {
                    grid.innerHTML = '';
                    var empty = document.getElementById('favorites-empty');
                    if (empty) empty.style.removeProperty('display');
                }
                updateFavoritesCountLabel(0);
            }).catch(function() {
                el.submit();
            });
        });
    }
});

// Image fallback (replaces inline onerror; error does not bubble, use capture).
document.addEventListener('error', function(e) {
    var img = e.target;
    if (!img || img.tagName !== 'IMG') return;
    var fb = img.getAttribute('data-fallback');
    if (!fb || img.getAttribute('data-fallback-applied') === '1') return;
    img.setAttribute('data-fallback-applied', '1');
    img.src = fb;
}, true);

// ============================================================================
// Cookie Consent — AI.md PART 12
// Enhancement only: the banner forms POST to /server/consent and work with
// zero JS (server renders the banner only when no cookie_consent cookie
// exists). This module intercepts the submits to skip the reload, writing the
// same cookie_consent cookie the server would set.
// ============================================================================
(function() {
    function saveAndApplyConsent(consent) {
        document.cookie = 'cookie_consent=' + encodeURIComponent(JSON.stringify(consent)) +
            '; path=/; max-age=31536000; SameSite=Lax';
        var banner = document.getElementById('cookie-consent');
        if (banner) banner.remove();
        applyConsent(consent);
    }
    function applyConsent(consent) {
        if (!consent.preferences) {
            document.cookie = 'theme=; path=/; max-age=0; SameSite=Lax';
            document.cookie = 'lang=; path=/; max-age=0; SameSite=Lax';
        }
    }
    window.saveAndApplyConsent = saveAndApplyConsent;
    window.applyConsent = applyConsent;

    onReady(function() {
        var banner = document.getElementById('cookie-consent');
        if (!banner) return;
        // Intercept the zero-JS form submits so the choice applies in place
        banner.addEventListener('submit', function(e) {
            var form = e.target;
            if (!form || form.getAttribute('action') !== '/server/consent') return;
            e.preventDefault();
            var choiceInput = form.querySelector('input[name="choice"]');
            var accepted = choiceInput && choiceInput.value === 'accept';
            saveAndApplyConsent({
                essential: true,
                preferences: accepted,
                analytics: false,
                timestamp: Math.floor(Date.now() / 1000)
            });
        });
    });
})();

// ============================================================================
// Health page auto-refresh (moved from healthz.tmpl) — runs only on /healthz
// ============================================================================
(function() {
    onReady(function() {
        var countdown = document.getElementById('countdown');
        if (!countdown) return;
        var seconds = 30;

        function statusClass(val) { return val === 'ok' ? 'status-ok' : 'status-error'; }
        function statusLabel(val) { return val === 'ok' ? '✅ OK' : '❌ Error'; }

        function applyUpdate(d) {
            var banner = document.querySelector('.status-banner');
            if (banner) {
                banner.className = 'status-banner status-' + (d.status === 'healthy' ? 'healthy' : d.status === 'unhealthy' ? 'unhealthy' : 'degraded');
                var icon = banner.querySelector('.status-icon');
                var text = banner.querySelector('.status-text');
                if (icon) icon.textContent = d.status === 'healthy' ? '✅' : d.status === 'unhealthy' ? '🔴' : '⚠️';
                if (text) text.textContent = d.status === 'healthy' ? 'All Systems Operational' : d.status === 'unhealthy' ? 'System Unhealthy' : 'System Degraded';
            }
            var checks = d.checks || {};
            var checkMap = {database: '🗄️ Database', cache: '💾 Cache', disk: '💿 Disk', scheduler: '⏰ Scheduler'};
            document.querySelectorAll('.checks-table tbody tr').forEach(function(row) {
                var label = row.cells[0] && row.cells[0].textContent.trim();
                for (var key in checkMap) {
                    if (checkMap.hasOwnProperty(key) && label === checkMap[key] && row.cells[1]) {
                        row.cells[1].className = statusClass(checks[key]);
                        row.cells[1].textContent = statusLabel(checks[key]);
                    }
                }
            });
            var stats = d.stats || {};
            var statRows = document.querySelectorAll('.stats-grid dd');
            if (statRows[0]) statRows[0].textContent = stats.requests_total || 0;
            if (statRows[1]) statRows[1].textContent = stats.requests_24h || 0;
            if (statRows[2]) statRows[2].textContent = stats.active_connections || 0;
            var ts = document.getElementById('hz-timestamp');
            if (ts && d.timestamp) {
                var dt = new Date(d.timestamp);
                ts.setAttribute('datetime', d.timestamp);
                ts.textContent = dt.toLocaleString();
            }
        }

        setInterval(function() {
            seconds--;
            if (countdown) countdown.textContent = seconds;
            if (seconds <= 0) {
                seconds = 30;
                fetch('/healthz.json', {cache: 'no-store'})
                    .then(function(r) { return r.json(); })
                    .then(applyUpdate)
                    .catch(function() { location.reload(); });
            }
        }, 1000);
    });
})();

// ============================================================================
// Preferences page (moved from preferences.tmpl) — runs only on /preferences
// i18n strings come from the #preferences-i18n data island.
// ============================================================================
(function() {
    onReady(function() {
        var form = document.getElementById('preferences-form');
        if (!form) return;

        var i18n = {};
        var island = document.getElementById('preferences-i18n');
        if (island) {
            try { i18n = JSON.parse(island.textContent); } catch (e) { i18n = {}; }
        }

        var STORAGE_KEY = 'vidveil_prefs';
        var HISTORY_KEY = 'vidveil_history';

        var defaults = {
            theme: 'auto',
            gridDensity: 'default',
            thumbnailSize: 'medium',
            autoplayPreview: true,
            previewDelay: '0',
            resultsPerPage: '0',
            openNewTab: true,
            defaultPreviewOnly: true,
            showAIContent: false,
            defaultDuration: '',
            minQuality: '360',
            defaultSort: '',
            minDuration: '600',
            maxHistory: '0',
            autoClearHistory: '0',
            useTor: false,
            proxyImages: true,
            engines: []
        };

        function loadPreferences() {
            var saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
            var prefs = Object.assign({}, defaults, saved);

            document.getElementById('theme').value = prefs.theme;
            document.documentElement.classList.remove('theme-dark', 'theme-light', 'theme-auto');
            document.documentElement.classList.add('theme-' + prefs.theme);
            document.getElementById('grid-density').value = prefs.gridDensity;
            document.getElementById('thumbnail-size').value = prefs.thumbnailSize;

            document.getElementById('autoplay-preview').checked = prefs.autoplayPreview;
            document.getElementById('preview-delay').value = prefs.previewDelay;

            document.getElementById('results-per-page').value = prefs.resultsPerPage;
            document.getElementById('open-new-tab').checked = prefs.openNewTab;

            document.getElementById('default-preview-only').checked = prefs.defaultPreviewOnly;
            document.getElementById('show-ai-content').checked = prefs.showAIContent;
            document.getElementById('default-duration').value = prefs.defaultDuration;
            document.getElementById('min-quality').value = prefs.minQuality;
            document.getElementById('default-sort').value = prefs.defaultSort;
            document.getElementById('min-duration').value = prefs.minDuration;

            document.getElementById('max-history').value = prefs.maxHistory;
            document.getElementById('auto-clear-history').value = prefs.autoClearHistory;

            document.getElementById('use-tor').checked = prefs.useTor;
            document.getElementById('proxy-images').checked = prefs.proxyImages;

            var savedEngines = prefs.enabledEngines || prefs.engines || [];
            if (savedEngines.length > 0) {
                document.querySelectorAll('input[name="engines"]').forEach(function(cb) {
                    cb.checked = savedEngines.includes(cb.value);
                });
            }
        }

        // Single-submit-only guard (AI.md PART 16: "Never let a single-submit
        // form button be clickable twice"). Without this, rapid repeat clicks
        // on Save each independently queue their own delayed navigate-away
        // call, so the redirect fires once per click instead of once total.
        var saving = false;

        function savePreferences(e) {
            e.preventDefault();
            if (saving) return;
            saving = true;

            var saveBtn = document.getElementById('preferences-save-btn');
            if (saveBtn) {
                saveBtn.disabled = true;
                saveBtn.textContent = i18n.saving;
            }

            var engines = [];
            document.querySelectorAll('input[name="engines"]:checked').forEach(function(cb) {
                engines.push(cb.value);
            });

            var prefs = {
                theme: document.getElementById('theme').value,
                gridDensity: document.getElementById('grid-density').value,
                thumbnailSize: document.getElementById('thumbnail-size').value,
                autoplayPreview: document.getElementById('autoplay-preview').checked,
                previewDelay: document.getElementById('preview-delay').value,
                resultsPerPage: document.getElementById('results-per-page').value,
                openNewTab: document.getElementById('open-new-tab').checked,
                defaultPreviewOnly: document.getElementById('default-preview-only').checked,
                showAIContent: document.getElementById('show-ai-content').checked,
                defaultDuration: document.getElementById('default-duration').value,
                minQuality: document.getElementById('min-quality').value,
                defaultSort: document.getElementById('default-sort').value,
                minDuration: document.getElementById('min-duration').value,
                maxHistory: document.getElementById('max-history').value,
                autoClearHistory: document.getElementById('auto-clear-history').value,
                useTor: document.getElementById('use-tor').checked,
                proxyImages: document.getElementById('proxy-images').checked,
                enabledEngines: engines
            };

            localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));

            document.documentElement.classList.remove('theme-dark', 'theme-light', 'theme-auto');
            document.documentElement.classList.add('theme-' + prefs.theme);
            var maxAge = 365 * 24 * 3600;
            // Cookie name must match getTheme()/setTheme() ("theme") so the
            // server's theme cookie reader stays authoritative regardless of
            // which of the two preferences-save handlers ran (see
            // mirrorServerPrefsToCookies for the same pattern applied to
            // resultsPerPage/openNewTab).
            document.cookie = 'theme=' + encodeURIComponent(prefs.theme) + '; path=/; max-age=' + maxAge + '; SameSite=Lax';
            if (typeof mirrorServerPrefsToCookies === 'function') {
                mirrorServerPrefsToCookies(prefs);
            }

            showToast(i18n.saved, 'success');
            setTimeout(function() {
                window.location.href = preferencesReturnTo();
            }, 800);
        }

        window.resetPreferences = function() {
            localStorage.removeItem(STORAGE_KEY);
            loadPreferences();
            showToast(i18n.resetDone, 'info');
        };

        window.selectAllEngines = function() {
            document.querySelectorAll('input[name="engines"]').forEach(function(cb) { cb.checked = true; });
            document.querySelectorAll('.tier-toggle input').forEach(function(cb) { cb.checked = true; });
        };
        window.selectNoneEngines = function() {
            document.querySelectorAll('input[name="engines"]').forEach(function(cb) { cb.checked = false; });
            document.querySelectorAll('.tier-toggle input').forEach(function(cb) { cb.checked = false; });
        };

        window.exportHistory = function() {
            var history = JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]');
            downloadJSON(history, 'vidveil-history.json');
            showToast(i18n.historyExported || 'History exported', 'success');
        };

        window.importHistory = function(file) {
            if (!file) return;
            var reader = new FileReader();
            reader.onload = function(e) {
                try {
                    var data = JSON.parse(e.target.result);
                    if (Array.isArray(data)) {
                        localStorage.setItem(HISTORY_KEY, JSON.stringify(data));
                        showToast((i18n.historyImported || '%d items imported').replace('%d', data.length), 'success');
                    } else {
                        showToast(i18n.historyInvalidFile || 'Invalid history file', 'error');
                    }
                } catch (err) {
                    showToast(i18n.historyParseFailed || 'Failed to parse file', 'error');
                }
            };
            reader.readAsText(file);
        };

        window.clearHistory = function() {
            showConfirm(i18n.historyClearConfirm || 'Clear all search history?', function () {
                localStorage.removeItem(HISTORY_KEY);
                showToast(i18n.historyCleared || 'History cleared', 'info');
            });
        };

        function downloadJSON(data, filename) {
            var blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
            var url = URL.createObjectURL(blob);
            var a = document.createElement('a');
            a.href = url;
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
        }

        document.getElementById('theme').addEventListener('change', function() {
            document.documentElement.classList.remove('theme-dark', 'theme-light', 'theme-auto');
            document.documentElement.classList.add('theme-' + this.value);
        });

        function initEngineTiers() {
            var container = document.getElementById('engine-tiers');
            var engines = container.querySelectorAll('.engine-toggle[data-tier]');

            var tiers = {};
            var tierNames = {
                1: i18n.tier1,
                2: i18n.tier2,
                3: i18n.tier3,
                4: i18n.tier4,
                5: i18n.tier5,
                6: i18n.tier6
            };

            engines.forEach(function(eng) {
                var tier = eng.dataset.tier;
                if (!tiers[tier]) tiers[tier] = [];
                tiers[tier].push(eng);
            });

            container.innerHTML = '';

            var sortedTiers = Object.keys(tiers).sort(function(a, b) { return parseInt(a) - parseInt(b); });

            sortedTiers.forEach(function(tier) {
                var tierEngines = tiers[tier];
                var tierName = tierNames[tier] || 'Tier ' + tier;

                var group = document.createElement('div');
                group.className = 'tier-group';
                group.dataset.tier = tier;

                var header = document.createElement('div');
                header.className = 'tier-header';
                header.innerHTML =
                    '<label class="toggle tier-toggle">' +
                        '<input type="checkbox" data-tier="' + tier + '" checked>' +
                        '<span class="slider"></span>' +
                        '<span class="toggle-label">' + tierName + ' (' + tierEngines.length + ' ' + (i18n.tierEngines || 'engines') + ')</span>' +
                    '</label>' +
                    '<button type="button" class="tier-expand" aria-expanded="false" aria-label="' + (i18n.expandTier || 'Expand tier') + '">' +
                        '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">' +
                            '<polyline points="6 9 12 15 18 9"></polyline>' +
                        '</svg>' +
                    '</button>';
                group.appendChild(header);

                var list = document.createElement('div');
                list.className = 'tier-engines collapsed';
                tierEngines.forEach(function(eng) {
                    eng.hidden = false;
                    list.appendChild(eng);
                });
                group.appendChild(list);

                container.appendChild(group);

                var tierToggle = header.querySelector('input[data-tier]');
                tierToggle.addEventListener('change', function() {
                    var checked = this.checked;
                    tierEngines.forEach(function(eng) {
                        eng.querySelector('input').checked = checked;
                    });
                });

                tierEngines.forEach(function(eng) {
                    eng.querySelector('input').addEventListener('change', function() {
                        var allChecked = tierEngines.every(function(en) { return en.querySelector('input').checked; });
                        var someChecked = tierEngines.some(function(en) { return en.querySelector('input').checked; });
                        tierToggle.checked = allChecked;
                        tierToggle.indeterminate = someChecked && !allChecked;
                    });
                });

                var expandBtn = header.querySelector('.tier-expand');
                expandBtn.addEventListener('click', function() {
                    var expanded = list.classList.toggle('collapsed');
                    this.setAttribute('aria-expanded', !expanded);
                });
            });
        }

        initEngineTiers();

        form.addEventListener('submit', savePreferences);
        form.dataset.managed = 'true';
        loadPreferences();
    });
})();
