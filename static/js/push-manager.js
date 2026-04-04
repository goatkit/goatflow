// GoatFlow Push Notification Manager
(function() {
    'use strict';

    var GoatFlowPush = {
        supported: function() {
            return 'serviceWorker' in navigator && 'PushManager' in window;
        },

        isSubscribed: function() {
            if (!this.supported()) return Promise.resolve(false);
            return navigator.serviceWorker.ready.then(function(reg) {
                return reg.pushManager.getSubscription().then(function(sub) {
                    return sub !== null;
                });
            });
        },

        subscribe: function() {
            if (!this.supported()) {
                return Promise.reject(new Error('Push notifications are not supported'));
            }

            return fetch('/api/push/vapid-key')
                .then(function(resp) { return resp.json(); })
                .then(function(data) {
                    if (!data.publicKey) throw new Error('No VAPID key returned');
                    var key = urlBase64ToUint8Array(data.publicKey);
                    return navigator.serviceWorker.ready.then(function(reg) {
                        return reg.pushManager.subscribe({
                            userVisibleOnly: true,
                            applicationServerKey: key
                        });
                    });
                })
                .then(function(subscription) {
                    var json = subscription.toJSON();
                    return fetch('/api/push/subscribe', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            endpoint: json.endpoint,
                            keys: {
                                p256dh: json.keys.p256dh,
                                auth: json.keys.auth
                            }
                        })
                    });
                })
                .then(function(resp) { return resp.json(); });
        },

        unsubscribe: function() {
            if (!this.supported()) return Promise.resolve();

            return navigator.serviceWorker.ready.then(function(reg) {
                return reg.pushManager.getSubscription();
            }).then(function(subscription) {
                if (!subscription) return;
                var endpoint = subscription.endpoint;
                return subscription.unsubscribe().then(function() {
                    return fetch('/api/push/unsubscribe', {
                        method: 'DELETE',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ endpoint: endpoint })
                    });
                });
            });
        }
    };

    function urlBase64ToUint8Array(base64String) {
        var padding = '='.repeat((4 - base64String.length % 4) % 4);
        var base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
        var rawData = atob(base64);
        var outputArray = new Uint8Array(rawData.length);
        for (var i = 0; i < rawData.length; i++) {
            outputArray[i] = rawData.charCodeAt(i);
        }
        return outputArray;
    }

    window.GoatFlowPush = GoatFlowPush;
})();
