// Vidveil - Frontend JavaScript

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
            // Mobile: tap to toggle preview
            container.addEventListener('click', (e) => {
                // Don't prevent link navigation on second tap
                if (!isPlaying) {
                    e.preventDefault();
                    e.stopPropagation();
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

    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    notification.textContent = message;
    notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        padding: 1rem 1.5rem;
        background: var(--bg-secondary);
        border: 1px solid var(--${type === 'success' ? 'success' : type === 'error' ? 'error' : 'accent'});
        border-radius: 8px;
        color: var(--text-primary);
        z-index: 1000;
        animation: slideIn 0.3s ease;
    `;

    document.body.appendChild(notification);

    setTimeout(() => {
        notification.style.animation = 'slideOut 0.3s ease';
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

    // Add animation styles
    const style = document.createElement('style');
    style.textContent = `
        @keyframes slideIn {
            from { transform: translateX(100%); opacity: 0; }
            to { transform: translateX(0); opacity: 1; }
        }
        @keyframes slideOut {
            from { transform: translateX(0); opacity: 1; }
            to { transform: translateX(100%); opacity: 0; }
        }
    `;
    document.head.appendChild(style);
});

// ============================================================================
// Mobile Navigation - TEMPLATE.md PART 13
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
// Export for global access
// ============================================================================
window.Vidveil = {
    setTheme,
    getTheme,
    getPreferences,
    savePreferences,
    resetPreferences,
    selectAllEngines,
    selectNoneEngines,
    selectTier,
    updateSort,
    filterBySource,
    filterByDuration,
    showNotification,
    fetchAPI,
    toggleNav,
    closeNav
};

// Make nav functions globally available for onclick handlers
window.toggleNav = toggleNav;
window.closeNav = closeNav;
