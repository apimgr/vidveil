// Vidveil - Home Page JavaScript
// TEMPLATE.md PART 16: External JS file for home page functionality

(function() {
    'use strict';

    // DOM elements
    var input = document.getElementById('search-input');
    var dropdown = document.getElementById('autocomplete-dropdown');
    var historyDiv = document.getElementById('search-history');
    var selectedIndex = -1;
    var suggestions = [];
    var debounceTimer;

    // ============================================================================
    // Search Form Submit Handler
    // ============================================================================
    function handleSearchSubmit(form) {
        var btn = form.querySelector('button[type="submit"]');
        if (btn.disabled) return false;
        btn.disabled = true;
        btn.innerHTML = '<span class="btn-spinner"></span> Searching...';
        hideDropdown();

        // Save to history
        var query = form.querySelector('input[name="q"]').value;
        saveSearchToHistory(query);

        return true;
    }
    window.handleSearchSubmit = handleSearchSubmit;

    // ============================================================================
    // Autocomplete Dropdown
    // ============================================================================
    function showDropdown() {
        if (dropdown) dropdown.classList.add('visible');
    }

    function hideDropdown() {
        if (dropdown) dropdown.classList.remove('visible');
        selectedIndex = -1;
    }

    function renderSuggestions() {
        if (!dropdown) return;
        if (suggestions.length === 0) {
            hideDropdown();
            return;
        }
        var html = suggestions.map(function(s, i) {
            var cls = 'autocomplete-item' + (i === selectedIndex ? ' selected' : '');
            return '<div class="' + cls + '" data-index="' + i + '" role="option">' +
                   '<span class="bang-code">' + escapeHtml(s.short_code) + '</span>' +
                   '<span class="bang-name">' + escapeHtml(s.display_name) + '</span>' +
                   '</div>';
        }).join('');
        dropdown.innerHTML = html;
        showDropdown();
    }

    function selectSuggestion(index) {
        if (index < 0 || index >= suggestions.length) return;
        var s = suggestions[index];
        var val = input.value;
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

        input.value = words.join(' ');
        hideDropdown();
        input.focus();
    }

    function fetchAutocomplete() {
        if (!input) return;
        var q = input.value;
        if (!q || !q.includes('!')) {
            hideDropdown();
            return;
        }

        fetch('/api/v1/autocomplete?q=' + encodeURIComponent(q))
            .then(function(r) { return r.json(); })
            .then(function(data) {
                if (data.success && data.suggestions && data.suggestions.length > 0) {
                    suggestions = data.suggestions;
                    selectedIndex = -1;
                    renderSuggestions();
                } else {
                    hideDropdown();
                }
            })
            .catch(function() { hideDropdown(); });
    }

    // ============================================================================
    // Search History
    // ============================================================================
    function getSearchHistory() {
        try {
            return JSON.parse(localStorage.getItem('vidveil_history') || '[]');
        } catch (e) {
            return [];
        }
    }

    function saveSearchToHistory(query) {
        if (!query || query.trim().length < 2) return;
        var history = getSearchHistory();

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

    function removeFromHistory(query) {
        var history = getSearchHistory();
        history = history.filter(function(h) { return h.query !== query; });
        try {
            localStorage.setItem('vidveil_history', JSON.stringify(history));
        } catch (e) {}
        renderSearchHistory();
    }

    function clearSearchHistory() {
        try {
            localStorage.removeItem('vidveil_history');
        } catch (e) {}
        renderSearchHistory();
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

    function renderSearchHistory() {
        if (!historyDiv) return;

        var history = getSearchHistory();
        if (history.length === 0) {
            historyDiv.innerHTML = '';
            historyDiv.style.display = 'none';
            return;
        }

        var html = '<div class="history-header"><span>Recent Searches</span><button type="button" onclick="Vidveil.Home.clearHistory()" class="history-clear" aria-label="Clear search history">Clear</button></div>';
        html += '<div class="history-items">';

        history.slice(0, 8).forEach(function(item) {
            html += '<div class="history-item">';
            html += '<a href="/search?q=' + encodeURIComponent(item.query) + '" class="history-link">' + escapeHtml(item.query) + '</a>';
            html += '<span class="history-time">' + formatTimeAgo(item.timestamp) + '</span>';
            html += '<button type="button" onclick="event.preventDefault();Vidveil.Home.removeFromHistory(\'' + escapeHtml(item.query).replace(/'/g, "\\'") + '\')" class="history-remove" aria-label="Remove from history">×</button>';
            html += '</div>';
        });

        html += '</div>';
        historyDiv.innerHTML = html;
        historyDiv.style.display = 'block';
    }

    // ============================================================================
    // Utility Functions
    // ============================================================================
    function escapeHtml(text) {
        if (!text) return '';
        var div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // ============================================================================
    // Event Listeners
    // ============================================================================
    if (input) {
        input.addEventListener('input', function() {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(fetchAutocomplete, 150);
        });

        input.addEventListener('keydown', function(e) {
            if (!dropdown || !dropdown.classList.contains('visible')) return;

            if (e.key === 'ArrowDown') {
                e.preventDefault();
                selectedIndex = Math.min(selectedIndex + 1, suggestions.length - 1);
                renderSuggestions();
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                selectedIndex = Math.max(selectedIndex - 1, 0);
                renderSuggestions();
            } else if (e.key === 'Enter' && selectedIndex >= 0) {
                e.preventDefault();
                selectSuggestion(selectedIndex);
            } else if (e.key === 'Escape') {
                hideDropdown();
            } else if (e.key === 'Tab' && selectedIndex >= 0) {
                e.preventDefault();
                selectSuggestion(selectedIndex);
            }
        });
    }

    if (dropdown) {
        dropdown.addEventListener('click', function(e) {
            var item = e.target.closest('.autocomplete-item');
            if (item) {
                selectSuggestion(parseInt(item.dataset.index, 10));
            }
        });
    }

    document.addEventListener('click', function(e) {
        if (!e.target.closest('.search-wrapper')) {
            hideDropdown();
        }
    });

    // Render history on page load
    renderSearchHistory();

    // ============================================================================
    // Export to global namespace
    // ============================================================================
    window.Vidveil = window.Vidveil || {};
    window.Vidveil.Home = {
        clearHistory: clearSearchHistory,
        removeFromHistory: removeFromHistory,
        saveToHistory: saveSearchToHistory
    };

    // Legacy global functions for onclick handlers
    window.clearSearchHistory = clearSearchHistory;
    window.removeFromHistory = removeFromHistory;

})();
