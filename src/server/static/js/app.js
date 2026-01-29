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
    localStorage.setItem('vidveil-theme', theme);

    // Update meta theme-color for mobile browsers
    updateMetaThemeColor(theme);
}

function getTheme() {
    // Default to 'dark' per AI.md PART 16: Dark theme is DEFAULT
    return localStorage.getItem('vidveil-theme') || 'dark';
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
    metaTheme.content = effectiveTheme === 'light' ? '#f5f5f5' : '#282a36';
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
    resultsPerPage: 0,  // 0 = infinite scroll (no pagination)
    openNewTab: true,
    defaultPreviewOnly: true,
    defaultDuration: '',
    defaultQuality: '',
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
    localStorage.removeItem('vidveil-theme');
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
function updateSort(sortBy) {
    const grid = document.getElementById('results');
    if (!grid) return;

    const cards = Array.from(grid.querySelectorAll('.video-card'));

    cards.sort((a, b) => {
        switch (sortBy) {
            case 'duration-desc':
                return (parseInt(b.dataset.duration) || 0) - (parseInt(a.dataset.duration) || 0);
            case 'duration-asc':
                return (parseInt(a.dataset.duration) || 0) - (parseInt(b.dataset.duration) || 0);
            case 'views':
                return (parseInt(b.dataset.views) || 0) - (parseInt(a.dataset.views) || 0);
            default:
                return 0; // Keep original order for relevance
        }
    });

    // Re-append in sorted order
    cards.forEach(card => grid.appendChild(card));
}

function filterBySource(source) {
    const cards = document.querySelectorAll('.video-card');
    cards.forEach(card => {
        if (!source || card.dataset.source === source) {
            card.style.display = '';
        } else {
            card.style.display = 'none';
        }
    });
}

// ============================================================================
// Unified Filter Panel - Toggle and Count
// ============================================================================
function toggleFilters() {
    var toggle = document.getElementById('filters-toggle');
    var content = document.getElementById('filters-content');
    if (!toggle || !content) return;

    var isExpanded = toggle.getAttribute('aria-expanded') === 'true';
    toggle.setAttribute('aria-expanded', !isExpanded);
    content.classList.toggle('expanded', !isExpanded);
}

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

// Handle filter changes - updates count and applies filters
function handleFilterChange() {
    updateFilterCount();

    // Apply filters to search results (if on search page)
    var duration = document.getElementById('filter-duration');
    var quality = document.getElementById('filter-quality');
    var sort = document.getElementById('filter-sort');

    if (duration) filterByDuration(duration.value);
    if (quality) filterByQuality(quality.value);
    // Source filter is now handled independently via updateSourceFilter()
    if (sort) sortResults(sort.value);
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
window.toggleFilters = toggleFilters;
window.updateFilterCount = updateFilterCount;
window.handleFilterChange = handleFilterChange;

function filterByDuration(duration) {
    const cards = document.querySelectorAll('.video-card');
    cards.forEach(card => {
        const seconds = parseInt(card.dataset.duration) || 0;
        let show = true;

        switch (duration) {
            case 'short':
                show = seconds < 600; // < 10 min
                break;
            case 'medium':
                show = seconds >= 600 && seconds <= 1800; // 10-30 min
                break;
            case 'long':
                show = seconds > 1800; // > 30 min
                break;
            default:
                show = true;
        }

        card.style.display = show ? '' : 'none';
    });
}

// ============================================================================
// Video Preview - Hover (desktop) and Tap (mobile)
// ============================================================================
function setupVideoPreview() {
    const containers = document.querySelectorAll('.thumb-container[data-preview]');
    const isTouchDevice = 'ontouchstart' in window || navigator.maxTouchPoints > 0;

    containers.forEach(container => {
        const video = container.querySelector('.thumb-preview');
        const staticImg = container.querySelector('.thumb-static');
        if (!video) return;

        let hoverTimeout;
        let isPlaying = false;

        // Desktop: hover behavior
        if (!isTouchDevice) {
            container.addEventListener('mouseenter', () => {
                hoverTimeout = setTimeout(() => {
                    video.style.opacity = '1';
                    staticImg.style.opacity = '0';
                    video.play().catch(() => {});
                    isPlaying = true;
                }, 200);
            });

            container.addEventListener('mouseleave', () => {
                clearTimeout(hoverTimeout);
                video.style.opacity = '0';
                staticImg.style.opacity = '1';
                video.pause();
                video.currentTime = 0;
                isPlaying = false;
            });
        } else {
            // Mobile: swipe right to preview
            let touchStartX = 0;
            let touchEndX = 0;
            
            container.addEventListener('touchstart', (e) => {
                touchStartX = e.changedTouches[0].screenX;
            }, { passive: true });
            
            container.addEventListener('touchend', (e) => {
                touchEndX = e.changedTouches[0].screenX;
                handleSwipeGesture();
            }, { passive: true });
            
            function handleSwipeGesture() {
                const swipeThreshold = 50; // minimum swipe distance
                const swipeDistance = touchEndX - touchStartX;
                
                // Swipe right - show preview
                if (swipeDistance > swipeThreshold && !isPlaying) {
                    video.style.opacity = '1';
                    staticImg.style.opacity = '0';
                    video.play().catch(() => {});
                    isPlaying = true;
                    
                    // Auto-stop after 5 seconds
                    setTimeout(() => {
                        if (isPlaying) {
                            video.style.opacity = '0';
                            staticImg.style.opacity = '1';
                            video.pause();
                            video.currentTime = 0;
                            isPlaying = false;
                        }
                    }, 5000);
                }
                // Swipe left - stop preview
                else if (swipeDistance < -swipeThreshold && isPlaying) {
                    video.style.opacity = '0';
                    staticImg.style.opacity = '1';
                    video.pause();
                    video.currentTime = 0;
                    isPlaying = false;
                }
            }
            
            // Tap to navigate (when not previewing)
            container.addEventListener('click', (e) => {
                // If not previewing, allow navigation
                if (!isPlaying) {
                    return; // Let link work normally
                }
                // If previewing, stop it
                e.preventDefault();
                e.stopPropagation();
                video.style.opacity = '0';
                staticImg.style.opacity = '1';
                video.pause();
                video.currentTime = 0;
                isPlaying = false;
            });
        }
    });
}

// ============================================================================
// Lazy Loading Images
// ============================================================================
function setupLazyLoading() {
    if ('IntersectionObserver' in window) {
        const observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    const img = entry.target;
                    if (img.dataset.src) {
                        img.src = img.dataset.src;
                        img.removeAttribute('data-src');
                    }
                    observer.unobserve(img);
                }
            });
        }, { rootMargin: '50px' });

        document.querySelectorAll('img[data-src]').forEach(img => observer.observe(img));
    }
}

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

        showNotification('Preferences saved!', 'success');
    });
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
async function fetchAPI(endpoint, options = {}) {
    try {
        const response = await fetch(`/api${endpoint}`, {
            headers: {
                'Content-Type': 'application/json',
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
        return data.engines?.length || 45;
    } catch {
        return 45; // Default fallback
    }
}

// ============================================================================
// Initialize
// ============================================================================
document.addEventListener('DOMContentLoaded', async function() {
    // Set theme
    setTheme(getTheme());

    // Setup theme selector if present
    const themeSelect = document.getElementById('theme');
    if (themeSelect) {
        themeSelect.value = getTheme();
        themeSelect.addEventListener('change', function() {
            setTheme(this.value);
        });
    }

    // Setup lazy loading
    setupLazyLoading();

    // Setup video preview on hover
    setupVideoPreview();

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
    localStorage.setItem('adminCollapsed', JSON.stringify(collapsed));
}

function loadCollapsedState() {
    try {
        var collapsed = JSON.parse(localStorage.getItem('adminCollapsed')) || [];
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

// Admin toast notification system
function showToast(message, type) {
    type = type || 'info';
    var container = document.getElementById('toast-container');
    if (!container) return;
    var toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    toast.innerHTML = '<span>' + message + '</span><button class="toast-close" onclick="this.parentElement.remove()">&times;</button>';
    container.appendChild(toast);
    setTimeout(function() { toast.classList.add('show'); }, 10);
    setTimeout(function() {
        toast.classList.remove('show');
        setTimeout(function() { toast.remove(); }, 300);
    }, 5000);
}

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
        '<div class="modal-body"><p id="' + id + '-desc">' + message + '</p></div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary cancel-btn">Cancel</button>' +
        '<button type="button" class="btn btn-primary confirm-btn">Confirm</button>' +
        '</div>';
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
// Local Favorites (IDEA.md: localStorage-only bookmarks)
// ============================================================================
(function() {
    'use strict';
    var FAVORITES_KEY = 'vidveil_favorites';
    var MAX_FAVORITES = 500;

    function getFavorites() {
        try {
            return JSON.parse(localStorage.getItem(FAVORITES_KEY) || '[]');
        } catch (e) {
            return [];
        }
    }

    function saveFavorites(favorites) {
        try {
            localStorage.setItem(FAVORITES_KEY, JSON.stringify(favorites));
        } catch (e) {
            console.error('Failed to save favorites:', e);
        }
    }

    function addFavorite(video) {
        var favorites = getFavorites();
        // Check for duplicate by URL
        if (favorites.some(function(f) { return f.url === video.url; })) {
            showInfo('Already in favorites');
            return false;
        }
        favorites.unshift({
            id: video.id || '',
            title: video.title || 'Untitled',
            url: video.url,
            thumbnail: video.thumbnail || '',
            duration: video.duration || '',
            source: video.source || '',
            sourceDisplay: video.source_display || video.source || '',
            savedAt: Date.now()
        });
        // Limit favorites
        if (favorites.length > MAX_FAVORITES) {
            favorites = favorites.slice(0, MAX_FAVORITES);
        }
        saveFavorites(favorites);
        showSuccess('Added to favorites');
        return true;
    }

    function removeFavorite(url) {
        var favorites = getFavorites().filter(function(f) { return f.url !== url; });
        saveFavorites(favorites);
        showSuccess('Removed from favorites');
        return true;
    }

    function isFavorite(url) {
        return getFavorites().some(function(f) { return f.url === url; });
    }

    function toggleFavorite(video) {
        if (isFavorite(video.url)) {
            removeFavorite(video.url);
            return false;
        } else {
            addFavorite(video);
            return true;
        }
    }

    function exportFavorites() {
        var data = JSON.stringify(getFavorites(), null, 2);
        var blob = new Blob([data], {type: 'application/json'});
        var url = URL.createObjectURL(blob);
        var a = document.createElement('a');
        a.href = url;
        a.download = 'vidveil-favorites.json';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        showSuccess('Favorites exported');
    }

    function importFavorites(file) {
        var reader = new FileReader();
        reader.onload = function(e) {
            try {
                var imported = JSON.parse(e.target.result);
                if (Array.isArray(imported)) {
                    saveFavorites(imported);
                    showSuccess('Favorites imported (' + imported.length + ' items)');
                } else {
                    showError('Invalid file format');
                }
            } catch (err) {
                showError('Failed to parse file');
            }
        };
        reader.readAsText(file);
    }

    function clearFavorites() {
        showConfirm('Are you sure you want to clear all favorites?', function() {
            saveFavorites([]);
            showSuccess('All favorites cleared');
        });
    }

    // Export to Vidveil namespace
    window.Vidveil = window.Vidveil || {};
    window.Vidveil.Favorites = {
        get: getFavorites,
        add: addFavorite,
        remove: removeFavorite,
        isFavorite: isFavorite,
        toggle: toggleFavorite,
        export: exportFavorites,
        import: importFavorites,
        clear: clearFavorites
    };
})();

// ============================================================================
// Long-Press Context Menu (IDEA.md: mobile quick actions)
// ============================================================================
(function() {
    'use strict';
    var longPressTimer = null;
    var LONG_PRESS_DURATION = 500;
    var activeMenu = null;

    function closeContextMenu() {
        if (activeMenu) {
            activeMenu.remove();
            activeMenu = null;
        }
    }

    function showVideoContextMenu(card, touch) {
        closeContextMenu();

        // Extract video data from card
        var link = card.querySelector('a[href]');
        var url = link ? link.href : '';
        var title = card.querySelector('h3 a');
        var titleText = title ? title.textContent : 'Video';
        var downloadLink = card.querySelector('.download-link');
        var downloadUrl = downloadLink ? downloadLink.href : '';
        var thumb = card.querySelector('img');
        var thumbnail = thumb ? (thumb.dataset.src || thumb.src) : '';
        var source = card.dataset.source || '';
        var sourceDisplay = card.querySelector('.source');
        var sourceText = sourceDisplay ? sourceDisplay.textContent : source;
        var duration = card.querySelector('.duration');
        var durationText = duration ? duration.textContent : '';

        var video = {
            url: url,
            title: titleText,
            thumbnail: thumbnail,
            source: source,
            source_display: sourceText,
            duration: durationText
        };

        var isFav = window.Vidveil.Favorites.isFavorite(url);

        var menu = document.createElement('div');
        menu.className = 'context-menu';
        menu.style.position = 'fixed';
        menu.style.zIndex = '10000';

        // Position menu at touch point, adjusting for screen edges
        var x = touch.clientX;
        var y = touch.clientY;
        var menuWidth = 200;
        var menuHeight = 160;
        if (x + menuWidth > window.innerWidth) x = window.innerWidth - menuWidth - 10;
        if (y + menuHeight > window.innerHeight) y = window.innerHeight - menuHeight - 10;
        if (x < 10) x = 10;
        if (y < 10) y = 10;
        menu.style.left = x + 'px';
        menu.style.top = y + 'px';

        var menuHtml = '<ul class="context-menu-list">';
        menuHtml += '<li class="context-menu-item" data-action="favorite">' + (isFav ? '&#x2665; Remove from Favorites' : '&#x2661; Add to Favorites') + '</li>';
        menuHtml += '<li class="context-menu-item" data-action="open">&#x2197; Open in New Tab</li>';
        menuHtml += '<li class="context-menu-item" data-action="copy">&#x1F4CB; Copy Link</li>';
        if (downloadUrl) {
            menuHtml += '<li class="context-menu-item" data-action="download">&#x21E9; Download</li>';
        }
        menuHtml += '</ul>';
        menu.innerHTML = menuHtml;

        document.body.appendChild(menu);
        activeMenu = menu;

        // Handle menu item clicks
        menu.addEventListener('click', function(e) {
            var item = e.target.closest('.context-menu-item');
            if (!item) return;
            var action = item.dataset.action;

            switch (action) {
                case 'favorite':
                    window.Vidveil.Favorites.toggle(video);
                    break;
                case 'open':
                    window.open(url, '_blank', 'noopener,noreferrer');
                    break;
                case 'copy':
                    if (navigator.clipboard) {
                        navigator.clipboard.writeText(url).then(function() {
                            showSuccess('Link copied to clipboard');
                        });
                    } else {
                        // Fallback
                        var input = document.createElement('input');
                        input.value = url;
                        document.body.appendChild(input);
                        input.select();
                        document.execCommand('copy');
                        document.body.removeChild(input);
                        showSuccess('Link copied');
                    }
                    break;
                case 'download':
                    handleDownloadClick(e, downloadUrl);
                    break;
            }
            closeContextMenu();
        });

        // Close on outside tap (delayed to prevent immediate close)
        setTimeout(function() {
            document.addEventListener('touchstart', function handler(e) {
                if (!menu.contains(e.target)) {
                    closeContextMenu();
                }
                document.removeEventListener('touchstart', handler);
            });
            document.addEventListener('click', function handler(e) {
                if (!menu.contains(e.target)) {
                    closeContextMenu();
                }
                document.removeEventListener('click', handler);
            });
        }, 100);
    }

    function setupLongPress() {
        document.addEventListener('touchstart', function(e) {
            var card = e.target.closest('.video-card');
            if (!card) return;

            longPressTimer = setTimeout(function() {
                e.preventDefault();
                showVideoContextMenu(card, e.touches[0]);
            }, LONG_PRESS_DURATION);
        }, { passive: false });

        document.addEventListener('touchend', function() {
            if (longPressTimer) {
                clearTimeout(longPressTimer);
                longPressTimer = null;
            }
        });

        document.addEventListener('touchmove', function() {
            if (longPressTimer) {
                clearTimeout(longPressTimer);
                longPressTimer = null;
            }
        });
    }

    // Initialize on DOMContentLoaded
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', setupLongPress);
    } else {
        setupLongPress();
    }

    // Export
    window.Vidveil = window.Vidveil || {};
    window.Vidveil.ContextMenu = {
        show: showVideoContextMenu,
        close: closeContextMenu
    };
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
// Home Page Functions
// ============================================================================
(function() {
    'use strict';

    var homeInput = null;
    var homeDropdown = null;
    var homeHistoryDiv = null;
    var homeSelectedIndex = -1;
    var homeSuggestions = [];
    var homeSuggestionType = 'search';
    var homeDebounceTimer;

    function initHomePage() {
        homeInput = document.getElementById('search-input');
        homeDropdown = document.getElementById('autocomplete-dropdown');
        homeHistoryDiv = document.getElementById('search-history');

        if (!homeInput) return; // Not on home page

        // Setup event listeners
        homeInput.addEventListener('input', function() {
            clearTimeout(homeDebounceTimer);
            homeDebounceTimer = setTimeout(fetchHomeAutocomplete, 150);
        });

        homeInput.addEventListener('keydown', function(e) {
            if (!homeDropdown || !homeDropdown.classList.contains('visible')) return;

            if (e.key === 'ArrowDown') {
                e.preventDefault();
                homeSelectedIndex = Math.min(homeSelectedIndex + 1, homeSuggestions.length - 1);
                renderHomeSuggestions();
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                homeSelectedIndex = Math.max(homeSelectedIndex - 1, 0);
                renderHomeSuggestions();
            } else if (e.key === 'Enter' && homeSelectedIndex >= 0) {
                e.preventDefault();
                selectHomeSuggestion(homeSelectedIndex);
            } else if (e.key === 'Escape') {
                hideHomeDropdown();
            } else if (e.key === 'Tab' && homeSelectedIndex >= 0) {
                e.preventDefault();
                selectHomeSuggestion(homeSelectedIndex);
            }
        });

        if (homeDropdown) {
            homeDropdown.addEventListener('click', function(e) {
                var item = e.target.closest('.autocomplete-item');
                if (item) {
                    selectHomeSuggestion(parseInt(item.dataset.index, 10));
                }
            });
        }

        document.addEventListener('click', function(e) {
            if (!e.target.closest('.search-wrapper')) {
                hideHomeDropdown();
            }
        });

        // Render history on page load
        renderHomeSearchHistory();
    }

    function handleSearchSubmit(form) {
        // Find submit button (try type="submit" first, then any button)
        var btn = form.querySelector('button[type="submit"]') || form.querySelector('button');
        if (!btn) return true; // No button found, allow form to submit
        if (btn.disabled) return false;
        btn.disabled = true;

        // For icon-only buttons (compact/inline), add loading class
        // For text buttons, replace with spinner text
        if (btn.classList.contains('search-btn--compact') || btn.querySelector('svg')) {
            btn.classList.add('btn-loading');
        } else {
            btn.innerHTML = '<span class="btn-spinner"></span> Searching...';
        }

        // Hide dropdown if on home page
        if (typeof hideHomeDropdown === 'function') {
            hideHomeDropdown();
        }

        // Save to history
        var query = form.querySelector('input[name="q"]');
        if (query && query.value) {
            saveHomeSearchToHistory(query.value);
        }

        return true;
    }

    function showHomeDropdown() {
        if (homeDropdown) homeDropdown.classList.add('visible');
    }

    function hideHomeDropdown() {
        if (homeDropdown) homeDropdown.classList.remove('visible');
        homeSelectedIndex = -1;
    }

    function renderHomeSuggestions() {
        if (!homeDropdown) return;
        if (homeSuggestions.length === 0) {
            hideHomeDropdown();
            return;
        }
        var html = homeSuggestions.map(function(s, i) {
            var cls = 'autocomplete-item' + (i === homeSelectedIndex ? ' selected' : '');
            if (homeSuggestionType === 'bang' || homeSuggestionType === 'bang_start') {
                // Bang suggestions have short_code and display_name
                return '<div class="' + cls + '" data-index="' + i + '" role="option">' +
                       '<span class="bang-code">' + escapeHtmlUtil(s.short_code || s.Bang || '') + '</span>' +
                       '<span class="bang-name">' + escapeHtmlUtil(s.display_name || s.EngineName || '') + '</span>' +
                       '</div>';
            } else {
                // Search term suggestions have term field
                var term = s.term || s.Term || s;
                return '<div class="' + cls + '" data-index="' + i + '" role="option">' +
                       '<span class="search-term">' + escapeHtmlUtil(term) + '</span>' +
                       '</div>';
            }
        }).join('');
        homeDropdown.innerHTML = html;
        showHomeDropdown();
    }

    function selectHomeSuggestion(index) {
        if (index < 0 || index >= homeSuggestions.length) return;
        var s = homeSuggestions[index];
        var val = homeInput.value;
        var words = val.split(/\s+/);

        if (homeSuggestionType === 'bang' || homeSuggestionType === 'bang_start') {
            // Bang suggestion - replace the bang being typed
            var bangCode = s.short_code || s.Bang || '';
            for (var i = words.length - 1; i >= 0; i--) {
                if (words[i].startsWith('!')) {
                    words[i] = bangCode;
                    break;
                }
            }

            // If no bang found at end, check if whole query is a bang
            if (val.trim().startsWith('!') && words.length === 1) {
                words[0] = bangCode + ' ';
            }

            homeInput.value = words.join(' ');
        } else {
            // Search term suggestion - replace entire query or last word
            var term = s.term || s.Term || s;
            if (words.length <= 1) {
                // Single word or empty - replace entirely
                homeInput.value = term;
            } else {
                // Multiple words - replace last word
                words[words.length - 1] = term;
                homeInput.value = words.join(' ');
            }
        }

        hideHomeDropdown();
        homeInput.focus();
    }

    function fetchHomeAutocomplete() {
        if (!homeInput) return;
        var q = homeInput.value;
        if (!q || q.length < 2) {
            hideHomeDropdown();
            return;
        }

        fetch('/api/v1/bangs/autocomplete?q=' + encodeURIComponent(q))
            .then(function(r) { return r.json(); })
            .then(function(data) {
                if (data.ok && data.suggestions && data.suggestions.length > 0) {
                    homeSuggestions = data.suggestions;
                    homeSuggestionType = data.type || 'search';
                    homeSelectedIndex = -1;
                    renderHomeSuggestions();
                } else {
                    hideHomeDropdown();
                }
            })
            .catch(function() { hideHomeDropdown(); });
    }

    function getHomeSearchHistory() {
        try {
            return JSON.parse(localStorage.getItem('vidveil_history') || '[]');
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

        // Keep only last 20
        if (history.length > 20) history = history.slice(0, 20);

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

        var html = '<div class="history-header"><span>Recent Searches</span><button type="button" onclick="Vidveil.Home.clearHistory()" class="history-clear" aria-label="Clear search history">Clear</button></div>';
        html += '<div class="history-items">';

        history.slice(0, 8).forEach(function(item) {
            html += '<div class="history-item">';
            html += '<a href="/search?q=' + encodeURIComponent(item.query) + '" class="history-link" onclick="showSearchSpinner(this, event)">' + escapeHtmlUtil(item.query) + '</a>';
            html += '<span class="history-time">' + formatTimeAgo(item.timestamp) + '</span>';
            html += '<button type="button" onclick="event.preventDefault();Vidveil.Home.removeFromHistory(\'' + escapeHtmlUtil(item.query).replace(/'/g, "\\'") + '\')" class="history-remove" aria-label="Remove from history">×</button>';
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
    var isSearching = true;
    var enginesCompleted = 0;
    var enginesWithResults = new Set();
    var sourcesSet = new Set();
    var searchCurrentDurationFilter = '';
    var searchCurrentQualityFilter = '';
    var searchCurrentSourceFilters = new Set(); // Multiple sources allowed
    var searchCurrentSort = '';
    var searchPreviewOnly = false;
    var startTime = Date.now();
    var isTouchDevice = 'ontouchstart' in window || navigator.maxTouchPoints > 0;
    var currentPage = 1;
    var isLoadingMore = false;
    var hasMoreResults = true;
    var infiniteScrollObserver = null;

    // Preferences loaded from storage
    var userPrefs = {};

    function initSearchPage() {
        var searchMeta = document.getElementById('search-meta');
        if (!searchMeta) return; // Not on search page

        searchQuery = searchMeta.dataset.query || new URLSearchParams(window.location.search).get('q') || '';

        // Load preferences from localStorage
        try {
            userPrefs = JSON.parse(localStorage.getItem('vidveil_prefs') || '{}');
        } catch (e) {
            userPrefs = {};
        }
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

        // Apply default filters from preferences
        if (userPrefs.defaultPreviewOnly) {
            searchPreviewOnly = true;
            var previewCheckbox = document.getElementById('filter-preview-only');
            if (previewCheckbox) previewCheckbox.checked = true;
        }
        if (userPrefs.defaultDuration) {
            searchCurrentDurationFilter = userPrefs.defaultDuration;
            var durationSelect = document.getElementById('filter-duration');
            if (durationSelect) durationSelect.value = userPrefs.defaultDuration;
        }
        if (userPrefs.defaultQuality) {
            searchCurrentQualityFilter = userPrefs.defaultQuality;
            var qualitySelect = document.getElementById('filter-quality');
            if (qualitySelect) qualitySelect.value = userPrefs.defaultQuality;
        }
        if (userPrefs.defaultSort) {
            searchCurrentSort = userPrefs.defaultSort;
            var sortSelect = document.getElementById('filter-sort');
            if (sortSelect) sortSelect.value = userPrefs.defaultSort;
        }

        // Save to search history
        if (searchQuery) {
            saveSearchPageHistory(searchQuery);
            streamResults(minDuration);
        }
    }

    function streamResults(minDuration) {
        if (!searchQuery) return;

        var eventSource = new EventSource('/api/v1/search?q=' + encodeURIComponent(searchQuery));
        var firstResult = true;

        eventSource.onmessage = function(event) {
            var data = JSON.parse(event.data);

            // Final done message
            if (data.done && data.engine === 'all') {
                eventSource.close();
                isSearching = false;
                var elapsed = Date.now() - startTime;
                var timeContainer = document.getElementById('search-time-container');
                if (timeContainer) timeContainer.textContent = 'in ' + elapsed + 'ms';
                updateSearchStatus();

                if (allResults.length === 0) {
                    var loadingEl = document.getElementById('initial-loading');
                    if (loadingEl) {
                        loadingEl.innerHTML = '<p>No results found.</p>';
                        loadingEl.classList.remove('hidden');
                    }
                    hasMoreResults = false;
                    // A11Y: Announce no results to screen readers
                    announce('No results found for ' + searchQuery);
                } else {
                    // Setup infinite scroll after initial results load
                    setupInfiniteScroll();
                    // Apply default filters from preferences
                    applySearchFiltersAndSort();
                    // A11Y: Announce result count to screen readers
                    announce(allResults.length + ' results found');
                    // Fetch and display related searches
                    fetchRelatedSearches(searchQuery);
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

            // Got a result
            if (data.result && data.result.title) {
                var r = data.result;

                // Apply min duration filter
                if (minDuration > 0 && r.duration_seconds > 0 && r.duration_seconds < minDuration) {
                    return;
                }

                // Show UI on first result
                if (firstResult) {
                    firstResult = false;
                    hideSearchElement('initial-loading');
                    showSearchElement('search-meta');
                    showSearchElement('filters');
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
            eventSource.close();
            // SSE failed - fallback to JSON API
            if (allResults.length === 0) {
                fallbackToJSONSearch(minDuration);
            } else {
                isSearching = false;
                updateSearchStatus();
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

        fetch('/api/v1/search?q=' + encodeURIComponent(searchQuery), {
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
            var elapsed = Date.now() - startTime;
            var timeContainer = document.getElementById('search-time-container');
            if (timeContainer) timeContainer.textContent = 'in ' + elapsed + 'ms';

            if (!data.ok || !data.data || !data.data.results || data.data.results.length === 0) {
                var loadingEl = document.getElementById('initial-loading');
                if (loadingEl) {
                    loadingEl.innerHTML = '<p>No results found.</p>';
                    loadingEl.classList.remove('hidden');
                }
                hasMoreResults = false;
                announce('No results found for ' + searchQuery);
                updateSearchStatus();
                return;
            }

            // Process results
            hideSearchElement('initial-loading');
            showSearchElement('search-meta');
            showSearchElement('filters');

            var results = data.data.results;
            for (var i = 0; i < results.length; i++) {
                var r = results[i];
                // Apply min duration filter
                if (minDuration > 0 && r.duration_seconds > 0 && r.duration_seconds < minDuration) {
                    continue;
                }
                allResults.push(r);
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
        })
        .catch(function(err) {
            isSearching = false;
            var loadingEl = document.getElementById('initial-loading');
            if (loadingEl) {
                loadingEl.innerHTML = '<p>Search failed. Please try again.</p>';
            }
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

        var previewUrl = r.preview_url || '';
        var hasPreview = previewUrl && previewUrl.length > 0;
        card.dataset.hasPreview = hasPreview ? '1' : '';
        var downloadUrl = r.download_url || '';
        var hasDownload = downloadUrl && downloadUrl.length > 0;

        // Check open in new tab preference (default true)
        var targetAttr = userPrefs.openNewTab !== false ? ' target="_blank"' : '';
        var html = '<a href="' + escapeHtmlUtil(r.url) + '"' + targetAttr + ' rel="noopener noreferrer nofollow" class="card-link">';
        html += '<div class="thumb-container"' + (hasPreview ? ' data-preview="' + escapeHtmlUtil(previewUrl) + '"' : '') + '>';
        html += '<img class="thumb-static" src="' + escapeHtmlUtil(r.thumbnail || '/static/images/placeholder.svg') + '" alt="' + escapeHtmlUtil(r.title) + '" loading="lazy" onerror="this.src=\'/static/images/placeholder.svg\'">';

        if (hasPreview) {
            html += '<video class="thumb-preview" muted loop playsinline preload="none">';
            html += '<source src="' + escapeHtmlUtil(previewUrl) + '" type="video/mp4">';
            html += '</video>';
            if (isTouchDevice) {
                html += '<div class="swipe-hint">Swipe to preview</div>';
            }
        }

        if (duration) html += '<span class="duration">' + escapeHtmlUtil(duration) + '</span>';
        if (r.quality) html += '<span class="quality-badge">' + escapeHtmlUtil(r.quality) + '</span>';
        html += '</div></a>';

        // Card menu button
        html += '<button type="button" class="card-menu-btn" aria-label="Video options" onclick="toggleCardMenu(this)">';
        html += '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="5" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="12" cy="19" r="2"/></svg>';
        html += '</button>';
        html += '<div class="card-menu" role="menu">';
        html += '<button type="button" class="card-menu-item" onclick="openInNewTab(\'' + escapeHtmlUtil(r.url).replace(/'/g, "\\'") + '\')"><span>Open in new tab</span></button>';
        html += '<button type="button" class="card-menu-item" onclick="copyVideoLink(\'' + escapeHtmlUtil(r.url).replace(/'/g, "\\'") + '\')"><span>Copy link</span></button>';
        html += '<button type="button" class="card-menu-item" onclick="toggleFavorite(this, ' + JSON.stringify({url: r.url, title: r.title || 'Untitled', thumbnail: r.thumbnail || '', source: r.source || ''}).replace(/"/g, '&quot;') + ')"><span>Add to favorites</span></button>';
        if (hasDownload) {
            html += '<button type="button" class="card-menu-item" onclick="openInNewTab(\'' + escapeHtmlUtil(downloadUrl).replace(/'/g, "\\'") + '\')"><span>Download</span></button>';
        }
        html += '</div>';

        html += '<div class="info">';
        html += '<h3><a href="' + escapeHtmlUtil(r.url) + '"' + targetAttr + ' rel="noopener noreferrer nofollow">' + escapeHtmlUtil(r.title || 'Untitled') + '</a></h3>';
        html += '<div class="meta"><span class="source">' + escapeHtmlUtil(r.source_display || r.source || '') + '</span>';
        if (r.views) html += '<span>' + escapeHtmlUtil(r.views) + '</span>';
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
        var previewDelay = parseInt(userPrefs.previewDelay) || 200;

        var isPlaying = false;
        var hoverTimeout;
        var touchStartX = 0;
        var touchStartY = 0;

        if (!isTouchDevice && autoplayEnabled) {
            // Desktop: hover behavior (only if autoplay enabled)
            container.addEventListener('mouseenter', function() {
                hoverTimeout = setTimeout(function() {
                    video.classList.add('preview-active');
                    staticImg.classList.add('preview-active');
                    video.play().catch(function() {});
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
                    if (!isPlaying) {
                        video.classList.add('preview-active');
                        staticImg.classList.add('preview-active');
                        if (swipeHint) swipeHint.classList.add('hidden');
                        video.play().catch(function() {});
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

    function searchFilterByDuration(value) {
        searchCurrentDurationFilter = value;
        applySearchFiltersAndSort();
    }

    function searchFilterByQuality(value) {
        searchCurrentQualityFilter = value;
        applySearchFiltersAndSort();
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
        applySearchFiltersAndSort();
    }

    function addSourceCheckbox(source, displayName) {
        var sourceOptions = document.getElementById('source-options');
        if (!sourceOptions) return;
        var label = document.createElement('label');
        label.className = 'source-option';
        label.innerHTML = '<input type="checkbox" name="source-filter" value="' + escapeHtmlUtil(source) + '" checked onchange="updateSourceFilter()"><span>' + escapeHtmlUtil(displayName) + '</span>';
        sourceOptions.appendChild(label);
    }

    function toggleSourceFilter() {
        var dropdown = document.getElementById('source-filter-list');
        var toggle = document.getElementById('source-filter-toggle');
        if (!dropdown || !toggle) return;
        var expanded = toggle.getAttribute('aria-expanded') === 'true';
        toggle.setAttribute('aria-expanded', !expanded);
        dropdown.classList.toggle('visible', !expanded);
    }

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

        // Apply filter (empty set = show all)
        searchCurrentSourceFilters = allChecked ? new Set() : new Set(selectedSources);
        applySearchFiltersAndSort();
        updateFilterCount();
    }

    function updatePreviewFilter(checked) {
        searchPreviewOnly = checked;
        applySearchFiltersAndSort();
        updateFilterCount();
    }

    function searchSortResults(value) {
        searchCurrentSort = value;
        applySearchFiltersAndSort();
    }

    function applySearchFiltersAndSort() {
        var cards = document.querySelectorAll('.video-card');

        cards.forEach(function(card) {
            var duration = parseInt(card.dataset.duration) || 0;
            var source = card.dataset.source || '';
            var quality = (card.dataset.quality || '').toUpperCase();
            var show = true;

            // Duration filter
            if (searchCurrentDurationFilter === 'short' && duration >= 600) show = false;
            else if (searchCurrentDurationFilter === 'medium' && (duration < 600 || duration > 1800)) show = false;
            else if (searchCurrentDurationFilter === 'long' && duration <= 1800) show = false;

            // Quality filter
            if (searchCurrentQualityFilter === '4k' && !quality.includes('4K') && !quality.includes('2160')) show = false;
            else if (searchCurrentQualityFilter === '1080' && !quality.includes('1080') && !quality.includes('HD')) show = false;
            else if (searchCurrentQualityFilter === '720' && !quality.includes('720')) show = false;

            // Source filter (multiple selection)
            if (searchCurrentSourceFilters.size > 0 && !searchCurrentSourceFilters.has(source)) show = false;

            // Preview filter
            if (searchPreviewOnly && !card.dataset.hasPreview) show = false;

            if (show) {
                card.classList.remove('hidden');
            } else {
                card.classList.add('hidden');
            }
        });

        // Sorting
        if (searchCurrentSort) {
            var grid = document.getElementById('video-grid');
            var cardArray = Array.from(grid.querySelectorAll('.video-card'));

            cardArray.sort(function(a, b) {
                var aDur = parseInt(a.dataset.duration) || 0;
                var bDur = parseInt(b.dataset.duration) || 0;
                var aViews = parseInt(a.dataset.views) || 0;
                var bViews = parseInt(b.dataset.views) || 0;
                var aQuality = (a.dataset.quality || '').toUpperCase();
                var bQuality = (b.dataset.quality || '').toUpperCase();

                if (searchCurrentSort === 'duration-desc') return bDur - aDur;
                if (searchCurrentSort === 'duration-asc') return aDur - bDur;
                if (searchCurrentSort === 'views') return bViews - aViews;
                if (searchCurrentSort === 'quality') {
                    var getQualityScore = function(q) {
                        if (q.includes('4K') || q.includes('2160')) return 4;
                        if (q.includes('1080')) return 3;
                        if (q.includes('720')) return 2;
                        if (q.includes('480')) return 1;
                        return 0;
                    };
                    return getQualityScore(bQuality) - getQualityScore(aQuality);
                }
                return 0;
            });

            cardArray.forEach(function(card) { grid.appendChild(card); });
        }

        // Update visible count
        var visibleCards = document.querySelectorAll('.video-card:not(.hidden)');
        var countEl = document.getElementById('result-count');
        if (countEl) countEl.textContent = visibleCards.length;
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
            if (statusText) statusText.textContent = allResults.length + ' results (streaming...)';
            if (engineStatus) engineStatus.textContent = enginesWithResults.size + ' engines responding';
        } else {
            var msg = allResults.length + ' results found';
            if (minDuration > 0) {
                msg += ' (min ' + Math.floor(minDuration / 60) + ' min)';
            }
            if (statusText) statusText.textContent = msg;
            if (engineStatus) engineStatus.textContent = enginesWithResults.size + ' engines';
        }
    }

    // Fetch and display related searches
    function fetchRelatedSearches(query) {
        if (!query) return;

        // Fetch from API (JSON format includes related_searches)
        fetch('/api/v1/search?q=' + encodeURIComponent(query) + '&nocache=1', {
            headers: { 'Accept': 'application/json' }
        })
        .then(function(response) { return response.json(); })
        .then(function(data) {
            if (data.ok && data.data && data.data.related_searches && data.data.related_searches.length > 0) {
                displayRelatedSearches(data.data.related_searches);
            }
        })
        .catch(function(err) {
            // Silently fail - related searches are not critical
        });
    }

    function displayRelatedSearches(searches) {
        var container = document.getElementById('related-searches');
        var tagsContainer = document.getElementById('related-tags');
        if (!container || !tagsContainer || !searches.length) return;

        tagsContainer.innerHTML = '';
        var maxVisible = 12;
        var totalSearches = Math.min(searches.length, 20);

        for (var i = 0; i < totalSearches; i++) {
            var tag = document.createElement('a');
            tag.className = 'related-tag' + (i >= maxVisible ? ' related-tag--hidden' : '');
            tag.href = '/search?q=' + encodeURIComponent(searches[i]);
            tag.innerHTML = '<svg class="related-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg><span>' + escapeHtmlUtil(searches[i]) + '</span>';
            tag.style.animationDelay = (i * 0.03) + 's';
            tagsContainer.appendChild(tag);
        }

        // Add "Show more" button if there are hidden tags
        if (totalSearches > maxVisible) {
            var showMoreBtn = document.createElement('button');
            showMoreBtn.type = 'button';
            showMoreBtn.className = 'related-show-more';
            showMoreBtn.innerHTML = '<span>+' + (totalSearches - maxVisible) + ' more</span>';
            showMoreBtn.onclick = function() {
                tagsContainer.classList.toggle('related-tags--expanded');
                var hiddenTags = tagsContainer.querySelectorAll('.related-tag--hidden');
                if (tagsContainer.classList.contains('related-tags--expanded')) {
                    showMoreBtn.innerHTML = '<span>Show less</span>';
                    hiddenTags.forEach(function(t) { t.classList.add('related-tag--visible'); });
                } else {
                    showMoreBtn.innerHTML = '<span>+' + (totalSearches - maxVisible) + ' more</span>';
                    hiddenTags.forEach(function(t) { t.classList.remove('related-tag--visible'); });
                }
            };
            tagsContainer.appendChild(showMoreBtn);
        }

        container.classList.remove('hidden');
    }

    // Infinite scroll - loads more pages as user scrolls
    function setupInfiniteScroll() {
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

        // Stream next page of results
        var eventSource = new EventSource('/api/v1/search?q=' + encodeURIComponent(searchQuery) + '&page=' + currentPage);
        var gotResults = false;

        eventSource.onmessage = function(event) {
            var data = JSON.parse(event.data);

            // Final done message
            if (data.done && data.engine === 'all') {
                eventSource.close();
                isLoadingMore = false;
                if (loadIndicator) loadIndicator.classList.add('hidden');

                // If no results on this page, stop infinite scroll
                if (!gotResults) {
                    hasMoreResults = false;
                    // Remove sentinel
                    var sentinel = document.getElementById('scroll-sentinel');
                    if (sentinel && infiniteScrollObserver) {
                        infiniteScrollObserver.unobserve(sentinel);
                    }
                }
                updateSearchStatus();
                return;
            }

            // Skip done/error from individual engines
            if (data.done || data.error) return;

            // Got a result
            if (data.result && data.result.title) {
                gotResults = true;
                var r = data.result;

                // Check for duplicates by URL
                var isDupe = allResults.some(function(existing) {
                    return existing.url === r.url;
                });

                if (!isDupe) {
                    allResults.push(r);
                    addResultCard(r);

                    var countEl = document.getElementById('result-count');
                    if (countEl) countEl.textContent = allResults.length;
                }
            }
        };

        eventSource.onerror = function() {
            eventSource.close();
            isLoadingMore = false;
            if (loadIndicator) loadIndicator.classList.add('hidden');
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
        toggleSourceFilter: toggleSourceFilter,
        toggleAllSources: toggleAllSources,
        updateSourceFilter: updateSourceFilter,
        updatePreviewFilter: updatePreviewFilter
    };
    window.filterByDuration = searchFilterByDuration;
    window.filterByQuality = searchFilterByQuality;
    window.filterBySource = searchFilterBySource;
    window.sortResults = searchSortResults;
    window.toggleSourceFilter = toggleSourceFilter;
    window.toggleAllSources = toggleAllSources;
    window.updateSourceFilter = updateSourceFilter;
    window.updatePreviewFilter = updatePreviewFilter;

    // Close source filter dropdown when clicking outside
    document.addEventListener('click', function(e) {
        var wrapper = document.querySelector('.source-filter-wrapper');
        var dropdown = document.getElementById('source-filter-list');
        var toggle = document.getElementById('source-filter-toggle');
        if (wrapper && dropdown && toggle && !wrapper.contains(e.target)) {
            toggle.setAttribute('aria-expanded', 'false');
            dropdown.classList.remove('visible');
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
// Card Menu Functions
// ============================================================================
function toggleCardMenu(btn) {
    var card = btn.closest('.video-card');
    var menu = card.querySelector('.card-menu');
    if (!menu) return;

    // Close all other open menus first
    document.querySelectorAll('.card-menu.visible').forEach(function(m) {
        if (m !== menu) m.classList.remove('visible');
    });

    menu.classList.toggle('visible');

    // Update favorite button text based on current state
    var favBtn = menu.querySelector('.card-menu-item:nth-child(3) span');
    if (favBtn) {
        var url = card.querySelector('.card-link').href;
        var favorites = getFavorites();
        var isFavorite = favorites.some(function(f) { return f.url === url; });
        favBtn.textContent = isFavorite ? 'Remove from favorites' : 'Add to favorites';
    }
}

function openInNewTab(url) {
    window.open(url, '_blank', 'noopener,noreferrer');
    closeAllCardMenus();
}

function copyVideoLink(url) {
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
    closeAllCardMenus();
}

function toggleFavorite(btn, videoData) {
    var favorites = getFavorites();
    var index = favorites.findIndex(function(f) { return f.url === videoData.url; });

    if (index >= 0) {
        // Remove from favorites
        favorites.splice(index, 1);
        showNotification('Removed from favorites', 'info');
        btn.querySelector('span').textContent = 'Add to favorites';
    } else {
        // Add to favorites
        videoData.added_at = new Date().toISOString();
        favorites.unshift(videoData);
        showNotification('Added to favorites', 'success');
        btn.querySelector('span').textContent = 'Remove from favorites';
    }

    saveFavorites(favorites);
    closeAllCardMenus();
}

function getFavorites() {
    try {
        return JSON.parse(localStorage.getItem('vidveil_favorites') || '[]');
    } catch (e) {
        return [];
    }
}

function saveFavorites(favorites) {
    localStorage.setItem('vidveil_favorites', JSON.stringify(favorites));
}

function closeAllCardMenus() {
    document.querySelectorAll('.card-menu.visible').forEach(function(m) {
        m.classList.remove('visible');
    });
}

// Close card menus when clicking outside
document.addEventListener('click', function(e) {
    if (!e.target.closest('.card-menu-btn') && !e.target.closest('.card-menu')) {
        closeAllCardMenus();
    }
});

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
