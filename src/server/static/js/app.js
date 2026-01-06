// Vidveil - Frontend JavaScript
// AI.md PART 16: Single app.js file for all frontend functionality

// ============================================================================
// Theme Management
// ============================================================================
function setTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('vidveil-theme', theme);
}

function getTheme() {
    return localStorage.getItem('vidveil-theme') || 'dark';
}

// ============================================================================
// Preferences Management
// ============================================================================
const defaultPrefs = {
    theme: 'dark',
    resultsPerPage: 50,
    openInNewTab: true,
    useTor: false,
    proxyImages: false,
    enabledEngines: [] // Empty means all enabled
};

function getPreferences() {
    try {
        const stored = localStorage.getItem('vidveil-prefs');
        return stored ? { ...defaultPrefs, ...JSON.parse(stored) } : defaultPrefs;
    } catch {
        return defaultPrefs;
    }
}

function savePreferences(prefs) {
    localStorage.setItem('vidveil-prefs', JSON.stringify(prefs));
}

function resetPreferences() {
    localStorage.removeItem('vidveil-prefs');
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

    const prefs = getPreferences();

    // Set form values from preferences
    const themeSelect = document.getElementById('theme');
    if (themeSelect) themeSelect.value = prefs.theme;

    const resultsSelect = document.getElementById('results-per-page');
    if (resultsSelect) resultsSelect.value = prefs.resultsPerPage;

    const torCheckbox = document.getElementById('use-tor');
    if (torCheckbox) torCheckbox.checked = prefs.useTor;

    const proxyCheckbox = document.getElementById('proxy-images');
    if (proxyCheckbox) proxyCheckbox.checked = prefs.proxyImages;

    // Restore engine selections from localStorage
    if (prefs.enabledEngines && prefs.enabledEngines.length > 0) {
        // Uncheck all first, then check only saved ones
        document.querySelectorAll('input[name="engines"]').forEach(cb => {
            cb.checked = prefs.enabledEngines.includes(cb.value);
        });
    }
    // If no saved engines, leave server defaults (all checked)

    // Handle form submission
    form.addEventListener('submit', (e) => {
        e.preventDefault();

        const newPrefs = {
            theme: document.getElementById('theme')?.value || 'dark',
            resultsPerPage: parseInt(document.getElementById('results-per-page')?.value) || 50,
            useTor: document.getElementById('use-tor')?.checked || false,
            proxyImages: document.getElementById('proxy-images')?.checked || false,
            enabledEngines: Array.from(document.querySelectorAll('input[name="engines"]:checked'))
                .map(cb => cb.value)
        };

        savePreferences(newPrefs);
        setTheme(newPrefs.theme);

        // Show success message
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

// Confirmation modal per AI.md PART 16 (replaces confirm())
function showConfirm(message, onConfirm, onCancel) {
    var modal = document.createElement('dialog');
    modal.className = 'modal confirm-modal';
    modal.innerHTML = '<div class="modal-header">' +
        '<h3 class="modal-title">Confirm Action</h3>' +
        '<button type="button" class="modal-close" aria-label="Close">&times;</button>' +
        '</div>' +
        '<div class="modal-body"><p>' + message + '</p></div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary cancel-btn">Cancel</button>' +
        '<button type="button" class="btn btn-primary confirm-btn">Confirm</button>' +
        '</div>';
    document.body.appendChild(modal);
    modal.showModal();
    modal.querySelector('.modal-close').onclick = function() {
        modal.close();
        modal.remove();
        if (onCancel) onCancel();
    };
    modal.querySelector('.cancel-btn').onclick = function() {
        modal.close();
        modal.remove();
        if (onCancel) onCancel();
    };
    modal.querySelector('.confirm-btn').onclick = function() {
        modal.close();
        modal.remove();
        if (onConfirm) onConfirm();
    };
    modal.addEventListener('cancel', function() {
        modal.remove();
        if (onCancel) onCancel();
    });
}

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
        var btn = form.querySelector('button[type="submit"]');
        if (btn.disabled) return false;
        btn.disabled = true;
        btn.innerHTML = '<span class="btn-spinner"></span> Searching...';
        hideHomeDropdown();

        // Save to history
        var query = form.querySelector('input[name="q"]').value;
        saveHomeSearchToHistory(query);

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
            return '<div class="' + cls + '" data-index="' + i + '" role="option">' +
                   '<span class="bang-code">' + escapeHtmlUtil(s.short_code) + '</span>' +
                   '<span class="bang-name">' + escapeHtmlUtil(s.display_name) + '</span>' +
                   '</div>';
        }).join('');
        homeDropdown.innerHTML = html;
        showHomeDropdown();
    }

    function selectHomeSuggestion(index) {
        if (index < 0 || index >= homeSuggestions.length) return;
        var s = homeSuggestions[index];
        var val = homeInput.value;
        var words = val.split(/\s+/);

        // Find and replace the bang being typed
        for (var i = words.length - 1; i >= 0; i--) {
            if (words[i].startsWith('!')) {
                words[i] = s.short_code;
                break;
            }
        }

        // If no bang found at end, check if whole query is a bang
        if (val.trim().startsWith('!') && words.length === 1) {
            words[0] = s.short_code + ' ';
        }

        homeInput.value = words.join(' ');
        hideHomeDropdown();
        homeInput.focus();
    }

    function fetchHomeAutocomplete() {
        if (!homeInput) return;
        var q = homeInput.value;
        if (!q || !q.includes('!')) {
            hideHomeDropdown();
            return;
        }

        fetch('/api/v1/bangs/autocomplete?q=' + encodeURIComponent(q))
            .then(function(r) { return r.json(); })
            .then(function(data) {
                if (data.success && data.suggestions && data.suggestions.length > 0) {
                    homeSuggestions = data.suggestions;
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
        var history = getHomeSearchHistory();

        // Remove duplicate if exists
        history = history.filter(function(h) { return h.query !== query; });

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

        var html = '<div class="history-header"><span>Recent Searches</span><button type="button" onclick="Vidveil.Home.clearHistory()" class="history-clear" aria-label="Clear search history">Clear</button></div>';
        html += '<div class="history-items">';

        history.slice(0, 8).forEach(function(item) {
            html += '<div class="history-item">';
            html += '<a href="/search?q=' + encodeURIComponent(item.query) + '" class="history-link">' + escapeHtmlUtil(item.query) + '</a>';
            html += '<span class="history-time">' + formatTimeAgo(item.timestamp) + '</span>';
            html += '<button type="button" onclick="event.preventDefault();Vidveil.Home.removeFromHistory(\'' + escapeHtmlUtil(item.query).replace(/'/g, "\\'") + '\')" class="history-remove" aria-label="Remove from history">×</button>';
            html += '</div>';
        });

        html += '</div>';
        homeHistoryDiv.innerHTML = html;
        homeHistoryDiv.style.display = 'block';
    }

    // Export home functions
    window.initHomePage = initHomePage;
    window.handleSearchSubmit = handleSearchSubmit;
    window.Vidveil = window.Vidveil || {};
    window.Vidveil.Home = {
        clearHistory: clearHomeSearchHistory,
        removeFromHistory: removeFromHomeHistory,
        saveToHistory: saveHomeSearchToHistory
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
    var searchCurrentSourceFilter = '';
    var searchCurrentSort = '';
    var startTime = Date.now();
    var isTouchDevice = 'ontouchstart' in window || navigator.maxTouchPoints > 0;
    var currentPage = 1;
    var isLoadingMore = false;
    var hasMoreResults = true;
    var infiniteScrollObserver = null;

    function initSearchPage() {
        var searchMeta = document.getElementById('search-meta');
        if (!searchMeta) return; // Not on search page

        searchQuery = searchMeta.dataset.query || new URLSearchParams(window.location.search).get('q') || '';

        // Load preferences from localStorage
        var prefs = {};
        try {
            prefs = JSON.parse(localStorage.getItem('vidveil_prefs') || '{}');
        } catch (e) {}
        var minDuration = parseInt(prefs.minDuration) || 0;

        // Save to search history
        if (searchQuery) {
            saveSearchPageHistory(searchQuery);
            streamResults(minDuration);
        }
    }

    function streamResults(minDuration) {
        if (!searchQuery) return;

        var eventSource = new EventSource('/api/v1/search/stream?q=' + encodeURIComponent(searchQuery));
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
                } else {
                    // Setup infinite scroll after initial results load
                    setupInfiniteScroll();
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
                    var filterSource = document.getElementById('filter-source');
                    if (filterSource) {
                        var opt = document.createElement('option');
                        opt.value = source;
                        opt.textContent = r.source_display || source;
                        filterSource.appendChild(opt);
                    }
                }
            }
        };

        eventSource.onerror = function(err) {
            eventSource.close();
            isSearching = false;
            if (allResults.length === 0) {
                var loadingEl = document.getElementById('initial-loading');
                if (loadingEl) {
                    loadingEl.innerHTML = '<p>Search failed. Please try again.</p>';
                }
            }
            updateSearchStatus();
        };

        // Update loading text
        var loadingText = document.getElementById('loading-text');
        if (loadingText) loadingText.textContent = 'Searching engines...';
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
        var downloadUrl = r.download_url || '';
        var hasDownload = downloadUrl && downloadUrl.length > 0;

        var html = '<a href="' + escapeHtmlUtil(r.url) + '" target="_blank" rel="noopener noreferrer nofollow" class="card-link">';
        html += '<div class="thumb-container"' + (hasPreview ? ' data-preview="' + escapeHtmlUtil(previewUrl) + '"' : '') + '>';
        html += '<img class="thumb-static" src="' + escapeHtmlUtil(r.thumbnail || '/static/img/placeholder.svg') + '" alt="' + escapeHtmlUtil(r.title) + '" loading="lazy" onerror="this.src=\'/static/img/placeholder.svg\'">';

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
        html += '<div class="info">';
        html += '<h3><a href="' + escapeHtmlUtil(r.url) + '" target="_blank" rel="noopener noreferrer nofollow">' + escapeHtmlUtil(r.title || 'Untitled') + '</a></h3>';
        html += '<div class="meta"><span class="source">' + escapeHtmlUtil(r.source_display || r.source || '') + '</span>';
        if (r.views) html += '<span>' + escapeHtmlUtil(r.views) + '</span>';
        // Download link (direct to source, not proxied)
        if (hasDownload) {
            html += '<a href="' + escapeHtmlUtil(downloadUrl) + '" target="_blank" rel="noopener noreferrer nofollow" class="download-link" title="Download video" onclick="event.stopPropagation()">&#x21E9;</a>';
        }
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

        var isPlaying = false;
        var hoverTimeout;
        var touchStartX = 0;
        var touchStartY = 0;

        if (!isTouchDevice) {
            // Desktop: hover behavior
            container.addEventListener('mouseenter', function() {
                hoverTimeout = setTimeout(function() {
                    video.classList.add('preview-active');
                    staticImg.classList.add('preview-active');
                    video.play().catch(function() {});
                    isPlaying = true;
                }, 200);
            });

            container.addEventListener('mouseleave', function() {
                clearTimeout(hoverTimeout);
                video.classList.remove('preview-active');
                staticImg.classList.remove('preview-active');
                video.pause();
                video.currentTime = 0;
                isPlaying = false;
            });
        } else {
            // Mobile: swipe right to preview
            container.addEventListener('touchstart', function(e) {
                touchStartX = e.touches[0].clientX;
                touchStartY = e.touches[0].clientY;
            }, { passive: true });

            container.addEventListener('touchend', function(e) {
                var touchEndX = e.changedTouches[0].clientX;
                var touchEndY = e.changedTouches[0].clientY;
                var deltaX = touchEndX - touchStartX;
                var deltaY = Math.abs(touchEndY - touchStartY);

                // Swipe right detected
                if (deltaX > 50 && deltaY < 50) {
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
                    e.preventDefault();
                    video.classList.remove('preview-active');
                    staticImg.classList.remove('preview-active');
                    video.pause();
                    video.currentTime = 0;
                    isPlaying = false;
                }
            }, { passive: false });
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

    function searchFilterBySource(value) {
        searchCurrentSourceFilter = value;
        applySearchFiltersAndSort();
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

            // Source filter
            if (searchCurrentSourceFilter && source !== searchCurrentSourceFilter) show = false;

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
        var eventSource = new EventSource('/api/v1/search/stream?q=' + encodeURIComponent(searchQuery) + '&page=' + currentPage);
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
        sortResults: searchSortResults
    };
    window.filterByDuration = searchFilterByDuration;
    window.filterByQuality = searchFilterByQuality;
    window.filterBySource = searchFilterBySource;
    window.sortResults = searchSortResults;
})();

// ============================================================================
// Shared Utility Functions
// ============================================================================
function escapeHtmlUtil(str) {
    if (!str) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// ============================================================================
// Export for global access
// ============================================================================
window.Vidveil = window.Vidveil || {};
Object.assign(window.Vidveil, {
    setTheme: setTheme,
    getTheme: getTheme,
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
    closeNav: closeNav
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
