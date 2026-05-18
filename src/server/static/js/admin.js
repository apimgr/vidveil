// AI.md PART 16: Admin panel JavaScript — no inline scripts, no alert()
'use strict';

// Resolve the admin API base from the script tag's data-api-base attribute.
var _adminAPIBase = (function() {
    var el = document.querySelector('script[data-api-base]');
    return el ? el.getAttribute('data-api-base') : '';
}());

// Show a non-blocking admin toast notification
function adminToast(message, type) {
    var toast = document.createElement('div');
    toast.className = 'admin-toast admin-toast-' + (type || 'info');
    toast.textContent = message;
    toast.setAttribute('role', 'alert');
    toast.setAttribute('aria-live', 'assertive');
    document.body.appendChild(toast);
    setTimeout(function() { toast.classList.add('admin-toast-visible'); }, 10);
    setTimeout(function() {
        toast.classList.remove('admin-toast-visible');
        setTimeout(function() { toast.remove(); }, 300);
    }, 4000);
}

// Engine toggle (enable / disable)
function toggleEngine(name, enable) {
    var base = _adminAPIBase;
    fetch(base + '/' + encodeURIComponent(name), {
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({enabled: enable})
    })
    .then(function(resp) { return resp.json(); })
    .then(function(data) {
        if (data.ok) {
            location.reload();
        } else {
            adminToast('Error: ' + (data.error || 'unknown'), 'error');
        }
    })
    .catch(function(e) { adminToast('Error: ' + e.message, 'error'); });
}

// Circuit breaker reset
function resetCircuit(name) {
    var base = _adminAPIBase;
    fetch(base + '/' + encodeURIComponent(name) + '/reset', {method: 'POST'})
    .then(function(resp) { return resp.json(); })
    .then(function(data) {
        if (data.ok) {
            location.reload();
        } else {
            adminToast('Error: ' + (data.error || 'unknown'), 'error');
        }
    })
    .catch(function(e) { adminToast('Error: ' + e.message, 'error'); });
}

// API token reveal toggle
var _tokenVisible = false;
function toggleToken() {
    var el = document.getElementById('api-token');
    var btn = document.querySelector('.toggle-token-btn');
    if (!el) { return; }
    if (_tokenVisible) {
        el.textContent = '••••••••••••••••';
        if (btn) { btn.textContent = 'Show'; }
        _tokenVisible = false;
    } else {
        var raw = el.getAttribute('data-token');
        if (raw) { el.textContent = raw; }
        if (btn) { btn.textContent = 'Hide'; }
        _tokenVisible = true;
    }
}
