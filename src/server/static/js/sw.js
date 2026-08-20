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

  // Skip non-GET requests
  if (event.request.method !== 'GET') {
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
    );
    return;
  }

  // HTML pages - network first, cache fallback, synthesized last resort
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
});

// Message event - handle SKIP_WAITING from app update per AI.md PART 16
self.addEventListener('message', event => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});
