// GoatFlow Service Worker
const CACHE_NAME = 'goatflow-v1';
const OFFLINE_URL = '/static/offline.html';

const PRECACHE_URLS = [
    OFFLINE_URL,
    '/static/css/output.css',
    '/static/css/fonts.css',
    '/static/js/alpine.min.js',
    '/static/js/htmx.min.js',
    '/static/js/common.js',
    '/static/images/goatflow-logo.svg',
    '/static/images/icon-192.png',
];

// Install: pre-cache key assets
self.addEventListener('install', function(event) {
    event.waitUntil(
        caches.open(CACHE_NAME).then(function(cache) {
            return cache.addAll(PRECACHE_URLS);
        }).then(function() {
            return self.skipWaiting();
        })
    );
});

// Activate: clean old caches
self.addEventListener('activate', function(event) {
    event.waitUntil(
        caches.keys().then(function(cacheNames) {
            return Promise.all(
                cacheNames.filter(function(name) {
                    return name !== CACHE_NAME;
                }).map(function(name) {
                    return caches.delete(name);
                })
            );
        }).then(function() {
            return self.clients.claim();
        })
    );
});

// Fetch: cache-first for static, network-first for everything else
self.addEventListener('fetch', function(event) {
    var request = event.request;

    // Only handle GET requests
    if (request.method !== 'GET') return;

    // Cache-first for static assets
    if (request.url.includes('/static/')) {
        event.respondWith(
            caches.match(request).then(function(cached) {
                return cached || fetch(request).then(function(response) {
                    if (response.ok) {
                        var clone = response.clone();
                        caches.open(CACHE_NAME).then(function(cache) {
                            cache.put(request, clone);
                        });
                    }
                    return response;
                });
            })
        );
        return;
    }

    // Network-first for navigation requests, offline fallback
    if (request.mode === 'navigate') {
        event.respondWith(
            fetch(request).catch(function() {
                return caches.match(OFFLINE_URL);
            })
        );
        return;
    }
});

// Push notification handler
self.addEventListener('push', function(event) {
    var data = {};
    if (event.data) {
        try {
            data = event.data.json();
        } catch (e) {
            data = { title: 'GoatFlow', body: event.data.text() };
        }
    }

    var title = data.title || 'GoatFlow';
    var options = {
        body: data.body || '',
        icon: '/static/images/icon-192.png',
        badge: '/static/images/icon-192.png',
        data: { url: data.url || '/dashboard' },
        tag: data.tag || 'goatflow-notification',
    };

    event.waitUntil(self.registration.showNotification(title, options));
});

// Notification click handler
self.addEventListener('notificationclick', function(event) {
    event.notification.close();

    var url = (event.notification.data && event.notification.data.url)
        ? event.notification.data.url
        : '/dashboard';

    event.waitUntil(
        self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(function(clients) {
            // Focus existing window if available
            for (var i = 0; i < clients.length; i++) {
                if (clients[i].url.includes(url) && 'focus' in clients[i]) {
                    return clients[i].focus();
                }
            }
            // Otherwise open a new window
            return self.clients.openWindow(url);
        })
    );
});
