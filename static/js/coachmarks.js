/**
 * GoatKit Coachmarks - Feature spotlight tooltips
 *
 * Lightweight system for showing contextual tips to users.
 * Tips are defined declaratively, tracked via localStorage + server preferences,
 * and styled to match the active GoatKit theme.
 *
 * Usage:
 *   GoatCoach.register({
 *     id: 'theme-switcher',
 *     target: '#theme-selector-btn',
 *     title: 'Customise your experience!',
 *     message: 'Try different themes and colour modes.',
 *     position: 'bottom',     // top|bottom|left|right
 *     maxViews: 3,            // auto-dismiss after N views
 *     pages: ['/dashboard', '/agent/dashboard', '/customer'],
 *     delay: 1000             // ms before showing
 *   });
 *
 *   GoatCoach.init();  // call after DOM ready
 */

var GoatCoach = (function () {
    'use strict';

    var tips = [];
    var STORAGE_KEY = 'gk_coachmarks';
    var activeTip = null;

    // Get dismissed/viewed state from localStorage
    function getState() {
        try {
            return JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
        } catch (e) {
            return {};
        }
    }

    function saveState(state) {
        try {
            localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
        } catch (e) { /* ignore */ }
    }

    function register(tip) {
        tips.push(tip);
    }

    function dismiss(tipId) {
        var state = getState();
        state[tipId] = { dismissed: true, at: Date.now() };
        saveState(state);
        hide();

        // Also persist to server (best-effort)
        fetch('/api/preferences/coachmarks/dismiss', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: tipId })
        }).catch(function () { /* silent */ });
    }

    function hide() {
        if (activeTip) {
            activeTip.remove();
            activeTip = null;
        }
        // Remove any backdrop
        var backdrop = document.getElementById('gk-coachmark-backdrop');
        if (backdrop) backdrop.remove();
    }

    function show(tip) {
        hide(); // clean up any existing

        var target = document.querySelector(tip.target);
        if (!target) return;

        // Record view count
        var state = getState();
        if (!state[tip.id]) state[tip.id] = {};
        state[tip.id].views = (state[tip.id].views || 0) + 1;
        saveState(state);

        // Create backdrop (subtle, click-to-dismiss)
        var backdrop = document.createElement('div');
        backdrop.id = 'gk-coachmark-backdrop';
        backdrop.onclick = function () { dismiss(tip.id); };
        document.body.appendChild(backdrop);

        // Create tooltip
        var el = document.createElement('div');
        el.className = 'gk-coachmark gk-coachmark-' + (tip.position || 'bottom');
        el.innerHTML =
            '<div class="gk-coachmark-arrow"></div>' +
            '<div class="gk-coachmark-content">' +
            (tip.title ? '<div class="gk-coachmark-title">' + tip.title + '</div>' : '') +
            '<div class="gk-coachmark-message">' + tip.message + '</div>' +
            '<div class="gk-coachmark-actions">' +
            '<button class="gk-coachmark-dismiss" onclick="GoatCoach.dismiss(\'' + tip.id + '\')">' +
            (tip.dismissText || 'Got it!') +
            '</button>' +
            '</div>' +
            '</div>';

        document.body.appendChild(el);
        activeTip = el;

        // Position relative to target
        positionTip(el, target, tip.position || 'bottom');

        // Reposition on scroll/resize
        var reposition = function () {
            if (activeTip === el) positionTip(el, target, tip.position || 'bottom');
        };
        window.addEventListener('scroll', reposition, { passive: true });
        window.addEventListener('resize', reposition);

        // Add entrance animation
        requestAnimationFrame(function () {
            el.classList.add('gk-coachmark-visible');
        });
    }

    function positionTip(el, target, position) {
        var rect = target.getBoundingClientRect();
        var tipRect = el.getBoundingClientRect();
        var gap = 12;
        var scrollX = window.pageXOffset || document.documentElement.scrollLeft;
        var scrollY = window.pageYOffset || document.documentElement.scrollTop;

        var top, left;

        switch (position) {
            case 'top':
                top = rect.top + scrollY - tipRect.height - gap;
                left = rect.left + scrollX + (rect.width / 2) - (tipRect.width / 2);
                break;
            case 'bottom':
                top = rect.bottom + scrollY + gap;
                left = rect.left + scrollX + (rect.width / 2) - (tipRect.width / 2);
                break;
            case 'left':
                top = rect.top + scrollY + (rect.height / 2) - (tipRect.height / 2);
                left = rect.left + scrollX - tipRect.width - gap;
                break;
            case 'right':
                top = rect.top + scrollY + (rect.height / 2) - (tipRect.height / 2);
                left = rect.right + scrollX + gap;
                break;
        }

        // Keep within viewport
        var maxLeft = window.innerWidth - tipRect.width - 8;
        if (left < 8) left = 8;
        if (left > maxLeft) left = maxLeft;

        el.style.top = top + 'px';
        el.style.left = left + 'px';
    }

    function init() {
        var path = window.location.pathname;
        var state = getState();

        for (var i = 0; i < tips.length; i++) {
            var tip = tips[i];

            // Skip dismissed tips
            if (state[tip.id] && state[tip.id].dismissed) continue;

            // Skip if max views exceeded
            if (tip.maxViews && state[tip.id] && state[tip.id].views >= tip.maxViews) continue;

            // Check page match (if pages specified)
            if (tip.pages && tip.pages.length > 0) {
                var matched = false;
                for (var j = 0; j < tip.pages.length; j++) {
                    if (path === tip.pages[j] || path.indexOf(tip.pages[j]) === 0) {
                        matched = true;
                        break;
                    }
                }
                if (!matched) continue;
            }

            // Show first eligible tip (with delay)
            (function (t) {
                setTimeout(function () {
                    // Re-check in case something changed during delay
                    var currentState = getState();
                    if (currentState[t.id] && currentState[t.id].dismissed) return;
                    show(t);
                }, t.delay || 1500);
            })(tip);

            break; // only one tip at a time
        }
    }

    // Public API
    return {
        register: register,
        dismiss: dismiss,
        hide: hide,
        init: init,
        reset: function (tipId) {
            var state = getState();
            if (tipId) {
                delete state[tipId];
            } else {
                state = {};
            }
            saveState(state);
        }
    };
})();
