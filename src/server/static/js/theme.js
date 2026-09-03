// Vidveil - Theme toggle instant-preview enhancement
// AI.md PART 16 "Theme Toggle": instant-preview enhancement only; the real
// POST still happens on submit. Recomputes the next mode from the LIVE
// <html> class on every click rather than trusting the form's hidden value
// (rendered once at page load, and stale after the first JS-driven switch)
// - this is what keeps repeated clicks cycling instead of sticking after
// the first one.
// `auto` = no class, CSS prefers-color-scheme rules apply.
var THEME_CYCLE = ['dark', 'light', 'auto'];

function themeToggleCurrentTheme() {
    if (document.documentElement.classList.contains('theme-dark')) return 'dark';
    if (document.documentElement.classList.contains('theme-light')) return 'light';
    return 'auto';
}

document.querySelectorAll('.theme-toggle-form').forEach(function (form) {
    form.addEventListener('submit', function () {
        // Does NOT call preventDefault() - the form still submits normally so
        // the server sets the cookie and re-renders with the correct next
        // target.
        var next = THEME_CYCLE[(THEME_CYCLE.indexOf(themeToggleCurrentTheme()) + 1) % THEME_CYCLE.length];
        document.documentElement.className = next === 'auto' ? '' : ('theme-' + next);
    });
});
