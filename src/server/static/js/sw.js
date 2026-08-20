// Service Worker for VidVeil PWA
// AI.md PART 16: PWA Support

const CACHE_NAME = 'vidveil-cache-v1';
const STATIC_ASSETS = [
  '/',
  '/offline.html',
  '/static/css/common.css',
  '/static/js/app.js',
  '/manifest.json',
  '/static/images/placeholder.svg'
];

// Synthesized last-resort response so respondWith never resolves undefined
// (undefined would surface as net::ERR_FAILED instead of a readable page)
function offlineFallbackResponse() {
  return new Response(
    '<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>Offline</title></head>' +
    '<body><h1>You are offline</h1><p>The request could not be completed. Check your connection and try again.</p></body></html>',
    { status: 503, headers: { 'Content-Type': 'text/html; charset=utf-8' } }
  );
}

// Install event - cache static assets
self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(cache => cache.addAll(STATIC_ASSETS))
      .then(() => self.skipWaiting())
  );
});

// Activate event - clean old caches
self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys()
      .then(keys => Promise.all(
        keys.filter(key => key.startsWith('vidveil-') && key !== CACHE_NAME)
            .map(key => caches.delete(key))
      ))
      .then(() => self.clients.claim())
  );
});

// Fetch event - cache-first for static, network-first for API
self.addEventListener('fetch', event => {
  const url = new URL(event.request.url);

  // Only same-origin GET is handled; everything else falls through to the
  // browser untouched (never call respondWith for it)
  if (event.request.method !== 'GET' || url.origin !== self.location.origin) {
    return;
  }

  // Skip API calls - always network
  if (url.pathname.startsWith('/api/')) {
    return;
  }

  // Static assets - cache first
  if (url.pathname.startsWith('/static/')) {
    event.respondWith(
      caches.match(event.request)
        .then(cached => cached || fetch(event.request)
          .then(response => {
            // Only cache successful responses — never error pages
            if (response.ok) {
              const clone = response.clone();
              caches.open(CACHE_NAME).then(cache => cache.put(event.request, clone));
            }
            return response;
          }))
        // A failed subresource must never reject respondWith — guaranteed 504
        .catch(() => new Response('', { status: 504, statusText: 'Gateway Timeout' }))
    );
    return;
  }

  // Navigations - network first, cache fallback, offline page, synthesized 503 last resort
  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request)
        .then(response => {
          // Only cache successful responses — never error pages or the age gate redirect chain
          if (response.ok) {
            const clone = response.clone();
            caches.open(CACHE_NAME).then(cache => cache.put(event.request, clone));
          }
          return response;
        })
        .catch(() => caches.match(event.request)
          .then(cached => cached || caches.match('/offline.html'))
          .then(fallback => fallback || offlineFallbackResponse())
        )
    );
    return;
  }

  // Non-navigation requests - network first, cache fallback, guaranteed 504 (never the offline page)
  event.respondWith(
    fetch(event.request)
      .then(response => {
        // Only cache successful responses — never error pages
        if (response.ok) {
          const clone = response.clone();
          caches.open(CACHE_NAME).then(cache => cache.put(event.request, clone));
        }
        return response;
      })
      .catch(() => caches.match(event.request)
        // A failed subresource must never reject respondWith — guaranteed 504
        .then(cached => cached || new Response('', { status: 504, statusText: 'Gateway Timeout' }))
      )
  );
});

// Message event - handle SKIP_WAITING from app update per AI.md PART 16
self.addEventListener('message', event => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});
