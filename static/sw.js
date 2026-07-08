// GoatFlow Service Worker
var CONFIG_URL = '/sw-config.json';
var FALLBACK_CACHE = 'goatflow-bootstrap';
var OFFLINE_URL = '/static/offline.html';
var DEFAULT_CONFIG = {
    enabled: true,
    version: 'bootstrap',
    cache_name: FALLBACK_CACHE,
    offline_url: OFFLINE_URL,
    precache_urls: [
        OFFLINE_URL,
        '/static/css/output.css',
        '/static/css/fonts.css',
        '/static/js/alpine.min.js',
        '/static/js/htmx.min.js',
        '/static/js/common.js',
        '/static/images/goatflow-logo.svg',
        '/static/images/icon-192.png'
    ],
    default_navigation_strategy: 'network-first',
    routes: []
};

var currentConfig = DEFAULT_CONFIG;
var lastConfigFetch = 0;
var CONFIG_REFRESH_MS = 60000;

function sameOriginURL(input) {
    try {
        var url = new URL(input, self.location.origin);
        return url.origin === self.location.origin ? url : null;
    } catch (e) {
        return null;
    }
}

function requestAcceptsEventStream(request) {
    var accept = request.headers.get('Accept') || '';
    return accept.toLowerCase().indexOf('text/event-stream') !== -1;
}

function isPluginEventStreamPath(pathname) {
    var parts = pathname.split('/');
    return parts.length === 7 &&
        parts[1] === 'api' &&
        parts[2] === 'v1' &&
        parts[3] === 'plugins' &&
        !!parts[4] &&
        parts[5] === 'events' &&
        !!parts[6];
}

function shouldBypassServiceWorker(request, url) {
    if (requestAcceptsEventStream(request) || isPluginEventStreamPath(url.pathname)) return true;
    // Auth redirects/callbacks must never be cached — they redirect to external IdPs
    if (url.pathname.indexOf('/auth/') === 0) return true;
    return false;
}

function cacheName() {
    return currentConfig.cache_name || FALLBACK_CACHE;
}

function loadConfig(force) {
    var now = Date.now();
    if (!force && now - lastConfigFetch < CONFIG_REFRESH_MS) {
        return Promise.resolve(currentConfig);
    }
    lastConfigFetch = now;
    return fetch(CONFIG_URL, { cache: 'no-store' }).then(function(response) {
        if (!response.ok) {
            throw new Error('service worker config failed');
        }
        return response.json();
    }).then(function(config) {
        currentConfig = Object.assign({}, DEFAULT_CONFIG, config || {});
        if (!Array.isArray(currentConfig.precache_urls)) {
            currentConfig.precache_urls = DEFAULT_CONFIG.precache_urls;
        }
        if (!Array.isArray(currentConfig.routes)) {
            currentConfig.routes = [];
        }
        return currentConfig;
    }).catch(function() {
        return currentConfig;
    });
}

function precacheConfiguredURLs() {
    return loadConfig(true).then(function(config) {
        var urls = (config.precache_urls || []).slice();
        (config.routes || []).forEach(function(rule) {
            if (rule && rule.path && rule.strategy !== 'network-only') {
                urls.push(rule.path);
            }
        });
        urls = urls.filter(function(raw) {
            return !!sameOriginURL(raw);
        });
        return caches.open(cacheName()).then(function(cache) {
            return Promise.all(urls.map(function(url) {
                return cache.add(url).catch(function() {
                    return Promise.resolve();
                });
            }));
        });
    });
}

// Install: pre-cache key assets and configured offline routes.
self.addEventListener('install', function(event) {
    event.waitUntil(
        precacheConfiguredURLs().then(function() {
            return self.skipWaiting();
        })
    );
});

// Activate: clean old GoatFlow caches.
self.addEventListener('activate', function(event) {
    event.waitUntil(
        loadConfig(true).then(function() {
            return caches.keys();
        }).then(function(cacheNames) {
            var keep = cacheName();
            return Promise.all(
                cacheNames.filter(function(name) {
                    return name.indexOf('goatflow-') === 0 && name !== keep;
                }).map(function(name) {
                    return caches.delete(name);
                })
            );
        }).then(function() {
            return self.clients.claim();
        })
    );
});

function routeMatches(rulePath, pathname) {
    if (!rulePath) return false;
    if (rulePath === '/') return pathname === '/';
    if (rulePath === pathname) return true;
    return rulePath.charAt(rulePath.length - 1) === '/' && pathname.indexOf(rulePath) === 0;
}

function matchingRule(pathname) {
    var routes = currentConfig.routes || [];
    var best = null;
    for (var i = 0; i < routes.length; i++) {
        var rule = routes[i];
        if (!rule || !routeMatches(rule.path, pathname)) {
            continue;
        }
        if (!best || rule.path.length > best.path.length) {
            best = rule;
        }
    }
    return best;
}

function putIfOK(cache, request, response) {
    if (response && response.ok) {
        cache.put(request, response.clone());
    }
    return response;
}

function offlineFallback() {
    return caches.match(currentConfig.offline_url || OFFLINE_URL);
}

function networkFirst(request) {
    return caches.open(cacheName()).then(function(cache) {
        return fetch(request).then(function(response) {
            return putIfOK(cache, request, response);
        }).catch(function() {
            return caches.match(request).then(function(cached) {
                return cached || offlineFallback();
            });
        });
    });
}

function cacheFirst(request) {
    return caches.open(cacheName()).then(function(cache) {
        return caches.match(request).then(function(cached) {
            return cached || fetch(request).then(function(response) {
                return putIfOK(cache, request, response);
            });
        });
    });
}

function staleWhileRevalidate(request) {
    return caches.open(cacheName()).then(function(cache) {
        var network = fetch(request).then(function(response) {
            return putIfOK(cache, request, response);
        });
        return caches.match(request).then(function(cached) {
            return cached || network.catch(function() {
                return offlineFallback();
            });
        });
    });
}

function networkOnly(request) {
    return fetch(request).catch(function() {
        if (request.mode === 'navigate') {
            return offlineFallback();
        }
        throw new Error('network unavailable');
    });
}

function respondWithStrategy(strategy, request) {
    switch (strategy) {
    case 'cache-first':
        return cacheFirst(request);
    case 'stale-while-revalidate':
        return staleWhileRevalidate(request);
    case 'network-only':
        return networkOnly(request);
    case 'network-first':
    default:
        return networkFirst(request);
    }
}

// Fetch: config-driven same-origin strategies with offline navigation fallback.
self.addEventListener('fetch', function(event) {
    var request = event.request;
    if (request.method !== 'GET') return;

    var url = sameOriginURL(request.url);
    if (!url) return;
    if (shouldBypassServiceWorker(request, url)) return;

    event.respondWith(
        loadConfig(false).then(function(config) {
            if (config.enabled === false) {
                return fetch(request);
            }

            var rule = matchingRule(url.pathname);
            var strategy = rule ? rule.strategy : null;

            if (!strategy && url.pathname.indexOf('/static/') === 0) {
                strategy = 'cache-first';
            }
            if (!strategy && request.mode === 'navigate') {
                strategy = config.default_navigation_strategy || 'network-first';
            }
            if (!strategy) {
                return fetch(request);
            }

            return respondWithStrategy(strategy, request);
        })
    );
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
        tag: data.tag || 'goatflow-notification'
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
            for (var i = 0; i < clients.length; i++) {
                if (clients[i].url.indexOf(url) !== -1 && 'focus' in clients[i]) {
                    return clients[i].focus();
                }
            }
            return self.clients.openWindow(url);
        })
    );
});
