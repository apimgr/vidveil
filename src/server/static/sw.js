// VidVeil Service Worker
// Per AI.md PART 17 lines 17751-17830
// Cache-first for static assets, network-first for HTML/API

const CACHE_NAME = 'vidveil-v1';
const STATIC_CACHE_NAME = 'vidveil-static-v1';
const MAX_CACHE_AGE = 7 * 24 * 60 * 60 * 1000; // 7 days

// Static assets to cache on install
const STATIC_ASSETS = [
  '/',
  '/static/css/style.css',
  '/static/js/app.js',
  '/static/img/logo.svg',
  '/static/img/icon-192.png',
  '/static/img/icon-512.png',
  '/manifest.json',
  '/offline.html'
];

// Install event - cache static assets
self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(STATIC_CACHE_NAME)
      .then(cache => cache.addAll(STATIC_ASSETS))
      .then(() => self.skipWaiting())
  );
});

// Activate event - clean old caches
self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys().then(cacheNames => {
      return Promise.all(
        cacheNames
          .filter(name => name !== CACHE_NAME && name !== STATIC_CACHE_NAME)
          .map(name => caches.delete(name))
      );
    }).then(() => self.clients.claim())
  );
});

// Fetch event - implement cache strategies
self.addEventListener('fetch', event => {
  const { request } = event;
  const url = new URL(request.url);

  // Skip non-GET requests
  if (request.method !== 'GET') {
    return;
  }

  // Skip API calls (network-only)
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(
      fetch(request).catch(() => {
        return new Response(
          JSON.stringify({ error: 'Offline', code: 'OFFLINE' }),
          { 
            status: 503,
            headers: { 'Content-Type': 'application/json' }
          }
        );
      })
    );
    return;
  }

  // Static assets: Cache-first
  if (
    url.pathname.startsWith('/static/') ||
    url.pathname === '/manifest.json' ||
    url.pathname.match(/\.(css|js|png|jpg|jpeg|svg|woff2?)$/)
  ) {
    event.respondWith(
      caches.match(request).then(cached => {
        if (cached) {
          // Check if cache is stale
          const cacheDate = cached.headers.get('sw-cache-date');
          if (cacheDate) {
            const age = Date.now() - parseInt(cacheDate);
            if (age < MAX_CACHE_AGE) {
              return cached;
            }
          }
        }

        // Fetch and cache
        return fetch(request).then(response => {
          if (response.ok) {
            const responseToCache = response.clone();
            caches.open(STATIC_CACHE_NAME).then(cache => {
              // Add cache date header
              const headers = new Headers(responseToCache.headers);
              headers.set('sw-cache-date', Date.now().toString());
              const cachedResponse = new Response(responseToCache.body, {
                status: responseToCache.status,
                statusText: responseToCache.statusText,
                headers: headers
              });
              cache.put(request, cachedResponse);
            });
          }
          return response;
        }).catch(() => cached || offlineResponse());
      })
    );
    return;
  }

  // HTML pages: Network-first, cache fallback
  event.respondWith(
    fetch(request)
      .then(response => {
        if (response.ok) {
          const responseToCache = response.clone();
          caches.open(CACHE_NAME).then(cache => {
            cache.put(request, responseToCache);
          });
        }
        return response;
      })
      .catch(() => {
        return caches.match(request).then(cached => {
          return cached || caches.match('/offline.html') || offlineResponse();
        });
      })
  );
});

// Push notification handler
self.addEventListener('push', event => {
  const data = event.data ? event.data.json() : { title: 'VidVeil', body: 'New notification' };
  
  event.waitUntil(
    self.registration.showNotification(data.title, {
      body: data.body,
      icon: '/static/img/icon-192.png',
      badge: '/static/img/icon-192.png',
      tag: data.tag || 'vidveil-notification',
      requireInteraction: data.requireInteraction || false,
      data: data.data || {}
    })
  );
});

// Notification click handler
self.addEventListener('notificationclick', event => {
  event.notification.close();
  
  const urlToOpen = event.notification.data?.url || '/';
  
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true })
      .then(windowClients => {
        // Check if already open
        for (let client of windowClients) {
          if (client.url === urlToOpen && 'focus' in client) {
            return client.focus();
          }
        }
        // Open new window
        if (clients.openWindow) {
          return clients.openWindow(urlToOpen);
        }
      })
  );
});

// Background sync (queue actions when offline)
self.addEventListener('sync', event => {
  if (event.tag === 'sync-queue') {
    event.waitUntil(syncQueue());
  }
});

// Helper: Offline response
function offlineResponse() {
  return new Response(
    'You are offline. Please check your connection.',
    {
      status: 503,
      statusText: 'Service Unavailable',
      headers: { 'Content-Type': 'text/plain' }
    }
  );
}

// Helper: Sync queued actions
async function syncQueue() {
  // Implementation for background sync
  // Queue actions stored in IndexedDB when offline
  // Process them when connection restored
  console.log('Background sync triggered');
}
