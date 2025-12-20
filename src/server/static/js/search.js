// Vidveil - Search Page JavaScript
// TEMPLATE.md PART 16: External JS file for search page functionality

(function() {
    'use strict';

    // Get query from data attribute or URL
    var searchMeta = document.getElementById('search-meta');
    var query = searchMeta ? searchMeta.dataset.query : new URLSearchParams(window.location.search).get('q') || '';

    var RESULTS_PER_BATCH = 20;
    var allResults = [];
    var displayedCount = 0;
    var isSearching = true;
    var enginesCompleted = 0;
    var enginesWithResults = new Set();
    var sourcesSet = new Set();
    var currentDurationFilter = '';
    var currentQualityFilter = '';
    var currentSourceFilter = '';
    var currentSort = '';
    var startTime = Date.now();

    var isTouchDevice = 'ontouchstart' in window || navigator.maxTouchPoints > 0;

    // Load preferences from localStorage
    var prefs = {};
    try {
        prefs = JSON.parse(localStorage.getItem('vidveil_prefs') || '{}');
    } catch (e) {}
    var minDuration = parseInt(prefs.minDuration) || 0;

    // ============================================================================
    // SSE Streaming
    // ============================================================================
    function streamResults() {
        if (!query) return;

        var eventSource = new EventSource('/api/v1/search/stream?q=' + encodeURIComponent(query));
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
                updateStatus();

                if (allResults.length === 0) {
                    var loadingEl = document.getElementById('initial-loading');
                    if (loadingEl) {
                        loadingEl.innerHTML = '<p>No results found.</p>';
                        loadingEl.classList.remove('hidden');
                    }
                }
                return;
            }

            // Engine completed
            if (data.done) {
                enginesCompleted++;
                updateStatus();
                return;
            }

            // Error from engine
            if (data.error) {
                enginesCompleted++;
                updateStatus();
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
                    hideElement('initial-loading');
                    showElement('search-meta');
                    showElement('filters');
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
            updateStatus();
        };

        // Update loading text
        var loadingText = document.getElementById('loading-text');
        if (loadingText) loadingText.textContent = 'Searching engines...';
    }

    // ============================================================================
    // Result Card Creation
    // ============================================================================
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

        var html = '<a href="' + escapeHtml(r.url) + '" target="_blank" rel="noopener noreferrer nofollow" class="card-link">';
        html += '<div class="thumb-container"' + (hasPreview ? ' data-preview="' + escapeHtml(previewUrl) + '"' : '') + '>';
        html += '<img class="thumb-static" src="' + escapeHtml(r.thumbnail || '/static/img/placeholder.svg') + '" alt="' + escapeHtml(r.title) + '" loading="lazy" onerror="this.src=\'/static/img/placeholder.svg\'">';

        if (hasPreview) {
            html += '<video class="thumb-preview" muted loop playsinline preload="none">';
            html += '<source src="' + escapeHtml(previewUrl) + '" type="video/mp4">';
            html += '</video>';
            if (isTouchDevice) {
                html += '<div class="swipe-hint">Swipe to preview</div>';
            }
        }

        if (duration) html += '<span class="duration">' + escapeHtml(duration) + '</span>';
        if (r.quality) html += '<span class="quality-badge">' + escapeHtml(r.quality) + '</span>';
        html += '</div></a>';
        html += '<div class="info">';
        html += '<h3><a href="' + escapeHtml(r.url) + '" target="_blank" rel="noopener noreferrer nofollow">' + escapeHtml(r.title || 'Untitled') + '</a></h3>';
        html += '<div class="meta"><span class="source">' + escapeHtml(r.source_display || r.source || '') + '</span>';
        if (r.views) html += '<span>' + escapeHtml(r.views) + '</span>';
        html += '</div></div>';

        card.innerHTML = html;
        grid.appendChild(card);

        // Setup video preview for this card
        setupCardPreview(card);
        displayedCount++;
    }

    // ============================================================================
    // Video Preview
    // ============================================================================
    function setupCardPreview(card) {
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

    // ============================================================================
    // Filtering and Sorting
    // ============================================================================
    function filterByDuration(value) {
        currentDurationFilter = value;
        applyFiltersAndSort();
    }

    function filterByQuality(value) {
        currentQualityFilter = value;
        applyFiltersAndSort();
    }

    function filterBySource(value) {
        currentSourceFilter = value;
        applyFiltersAndSort();
    }

    function sortResults(value) {
        currentSort = value;
        applyFiltersAndSort();
    }

    function applyFiltersAndSort() {
        var cards = document.querySelectorAll('.video-card');

        cards.forEach(function(card) {
            var duration = parseInt(card.dataset.duration) || 0;
            var source = card.dataset.source || '';
            var quality = (card.dataset.quality || '').toUpperCase();
            var show = true;

            // Duration filter
            if (currentDurationFilter === 'short' && duration >= 600) show = false;
            else if (currentDurationFilter === 'medium' && (duration < 600 || duration > 1800)) show = false;
            else if (currentDurationFilter === 'long' && duration <= 1800) show = false;

            // Quality filter
            if (currentQualityFilter === '4k' && !quality.includes('4K') && !quality.includes('2160')) show = false;
            else if (currentQualityFilter === '1080' && !quality.includes('1080') && !quality.includes('HD')) show = false;
            else if (currentQualityFilter === '720' && !quality.includes('720')) show = false;

            // Source filter
            if (currentSourceFilter && source !== currentSourceFilter) show = false;

            if (show) {
                card.classList.remove('hidden');
            } else {
                card.classList.add('hidden');
            }
        });

        // Sorting
        if (currentSort) {
            var grid = document.getElementById('video-grid');
            var cardArray = Array.from(grid.querySelectorAll('.video-card'));

            cardArray.sort(function(a, b) {
                var aDur = parseInt(a.dataset.duration) || 0;
                var bDur = parseInt(b.dataset.duration) || 0;
                var aViews = parseInt(a.dataset.views) || 0;
                var bViews = parseInt(b.dataset.views) || 0;
                var aQuality = (a.dataset.quality || '').toUpperCase();
                var bQuality = (b.dataset.quality || '').toUpperCase();

                if (currentSort === 'duration-desc') return bDur - aDur;
                if (currentSort === 'duration-asc') return aDur - bDur;
                if (currentSort === 'views') return bViews - aViews;
                if (currentSort === 'quality') {
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

    // ============================================================================
    // Status Updates
    // ============================================================================
    function updateStatus() {
        var statusText = document.getElementById('status-text');
        var engineStatus = document.getElementById('engine-status');

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

    // ============================================================================
    // Utility Functions
    // ============================================================================
    function escapeHtml(str) {
        if (!str) return '';
        return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    function hideElement(id) {
        var el = document.getElementById(id);
        if (el) el.classList.add('hidden');
    }

    function showElement(id) {
        var el = document.getElementById(id);
        if (el) el.classList.remove('hidden');
    }

    // ============================================================================
    // Search History
    // ============================================================================
    function saveSearchHistory(q) {
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

    // ============================================================================
    // Initialize
    // ============================================================================
    // Save to search history
    if (query) {
        saveSearchHistory(query);
        streamResults();
    }

    // ============================================================================
    // Export to global namespace
    // ============================================================================
    window.Vidveil = window.Vidveil || {};
    window.Vidveil.Search = {
        filterByDuration: filterByDuration,
        filterByQuality: filterByQuality,
        filterBySource: filterBySource,
        sortResults: sortResults
    };

    // Global functions for onchange handlers
    window.filterByDuration = filterByDuration;
    window.filterByQuality = filterByQuality;
    window.filterBySource = filterBySource;
    window.sortResults = sortResults;

})();
