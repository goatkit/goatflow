/**
 * Common JavaScript utilities for GoatFlow
 */

/**
 * Track which submit button was clicked on any form.
 * After a successful save via fetch(), call shouldCloseAfterSave(form)
 * to check if the user clicked "Save & Close" (continue=0).
 *
 * Usage in form handlers:
 *   const closeAfterSave = shouldCloseAfterSave(form);
 *   // ... do the save ...
 *   if (closeAfterSave) window.location.href = '/admin/list-page';
 */
(function() {
    document.addEventListener('click', function(e) {
        var btn = e.target.closest('button[type="submit"][name="continue"]');
        if (btn && btn.form) {
            btn.form._lastContinueValue = btn.value;
        }
    });
})();

function shouldCloseAfterSave(formOrEl) {
    var form = formOrEl;
    if (form && form.tagName !== 'FORM') form = form.closest('form');
    if (!form) return false;
    var val = form._lastContinueValue;
    form._lastContinueValue = undefined; // reset
    return val === '0';
}

/**
 * Copy text to clipboard and show toast notification
 * @param {string} text - The text to copy to clipboard
 * @param {boolean} showNotification - Whether to show toast notification (default: true)
 */
function copyToClipboard(text, showNotification = true) {
    const notify = function (success) {
        if (showNotification && typeof showToast === "function") {
            showToast(
                success ? "Copied to clipboard" : "Failed to copy",
                success ? "success" : "error",
            );
        }
    };

    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard
            .writeText(text)
            .then(function () {
                notify(true);
            })
            .catch(function (err) {
                console.error("Failed to copy: ", err);
                fallbackCopy(text, notify);
            });
    } else {
        fallbackCopy(text, notify);
    }
}

/**
 * Fallback copy method using execCommand for older browsers
 * @param {string} text - The text to copy
 * @param {function} notify - Callback function to notify success/failure
 */
function fallbackCopy(text, notify) {
    const area = document.createElement("textarea");
    area.value = text;
    area.style.position = "fixed";
    area.style.left = "-9999px";
    document.body.appendChild(area);
    area.select();
    try {
        document.execCommand("copy");
        if (notify) notify(true);
    } catch (e) {
        console.error("Fallback copy failed: ", e);
        if (notify) notify(false);
    }
    document.body.removeChild(area);
}

/**
 * Enhanced fetch wrapper that automatically includes credentials for API calls
 */
function apiFetch(url, options = {}) {
    // If it's an API call, include credentials and set Accept header
    if (
        url.startsWith("/api/") ||
        url.startsWith("/api/v1/") ||
        url.startsWith("/agent/")
    ) {
        options.credentials = options.credentials || "include";
        // Ensure headers object exists
        options.headers = options.headers || {};
        // Set Accept header to request JSON (prevents HTML redirect on auth failure)
        if (!options.headers["Accept"]) {
            options.headers["Accept"] = "application/json";
        }
    }

    return fetch(url, options)
        .then((response) => {
            // Trigger Guru overlay if server flagged an error explicitly
            const guruMsg =
                response.headers &&
                response.headers.get &&
                response.headers.get("X-Guru-Error");
            if (guruMsg) {
                const code =
                    response.status === 401 || response.status === 403
                        ? "00000008.CAFEBABE"
                        : response.status >= 500
                          ? "0000000A.BADF00D"
                          : "00000009.BADC0DE";
                try {
                    showGuruMeditation(
                        `${guruMsg}\n\n${url} [${response.status} ${response.statusText}]`,
                        code,
                    );
                } catch (_) {}
            }
            // Auto‑trigger Guru Meditation for auth and server errors; let callers still handle inline
            if (
                !response.ok &&
                (response.status >= 500 ||
                    response.status === 401 ||
                    response.status === 403)
            ) {
                const code =
                    response.status === 401 || response.status === 403
                        ? "00000008.CAFEBABE"
                        : "0000000A.BADF00D";
                const msg = `Request failed: ${url} [${response.status} ${response.statusText}]`;
                try {
                    showGuruMeditation(msg, code);
                } catch (_) {}
            }
            // Parse JSON response - callers expect data, not Response object
            return response.json();
        })
        .catch((err) => {
            // Network failures also surface the Guru overlay
            const msg = `Network error calling ${url}: ${err && err.message ? err.message : "Unknown error"}`;
            try {
                showGuruMeditation(msg, "0000000B.NETERR01");
            } catch (_) {}
            throw err;
        });
}

// Display an Amiga-style Guru Meditation overlay for critical errors
function showGuruMeditation(message, code = "00000007.DEADBEEF") {
    // Avoid stacking many overlays
    if (document.getElementById("guru-meditation-overlay")) return;
    const wrapper = document.createElement("div");
    wrapper.id = "guru-meditation-overlay";
    wrapper.style.position = "fixed";
    wrapper.style.inset = "0";
    wrapper.style.zIndex = "99999";
    wrapper.style.display = "flex";
    wrapper.style.alignItems = "center";
    wrapper.style.justifyContent = "center";
    wrapper.style.background = "rgba(0,0,0,0.6)";
    wrapper.innerHTML = `
            <div id="guru-meditation" class="cursor-pointer" style="border: 8px solid #ff0000; background:#000; color:#fff; max-width: 800px; width: calc(100% - 48px); padding: 24px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, \"Liberation Mono\", \"Courier New\", monospace;">
                <div style="text-align:center; margin-bottom: 8px;">Software Failure.    Click to continue.</div>
                <div style="text-align:center; font-size: 20px; font-weight: 700; margin-bottom: 12px;">Guru Meditation #${code}</div>
                <div style="white-space: pre-wrap; word-break: break-word; font-size: 13px; color:#ddd;">${message || ""}</div>
            </div>`;
    // Click anywhere to dismiss
    wrapper.addEventListener("click", () => {
        wrapper.remove();
    });
    document.body.appendChild(wrapper);
}

function ensureToastContainer() {
    if (typeof document === "undefined") {
        return null;
    }
    const body = document.body;
    if (!body) {
        return null;
    }
    let container = document.getElementById("toast-container");
    if (!container) {
        container = document.createElement("div");
        container.id = "toast-container";
        // Use fixed positioning without bottom-0 class - bottom is set dynamically
        container.className =
            "fixed right-0 p-6 space-y-4 z-50 pointer-events-none";
        container.style.bottom = "0px";
        body.appendChild(container);
    } else if (!container.classList.contains("pointer-events-none")) {
        container.classList.add("pointer-events-none");
    }
    // Update position to account for any persistent notifications (reminder deck)
    updateToastContainerPosition();
    return container;
}

// Update toast container position to sit above the reminder deck
function updateToastContainerPosition() {
    const container = document.getElementById("toast-container");
    if (!container) return;

    const reminderDeck = document.getElementById("reminder-deck");
    if (reminderDeck && reminderDeck.children.length > 0) {
        // Position toasts above the reminder deck with some padding
        const deckRect = reminderDeck.getBoundingClientRect();
        const deckHeight = deckRect.height;
        // Toast container is at right-0 p-6 (24px padding), reminder deck is at bottom-6 right-6
        // So we need to offset by deck height + spacing
        container.style.bottom = `${deckHeight + 32}px`;
    } else {
        // No reminder deck visible, position at bottom
        container.style.bottom = "0px";
    }
}

function removeToastElement(toast) {
    if (!toast || toast.dataset.dismissed) {
        return;
    }
    toast.dataset.dismissed = "1";
    toast.classList.add("opacity-0", "translate-y-2");
    setTimeout(() => {
        if (toast.parentNode) {
            toast.parentNode.removeChild(toast);
        }
    }, 180);
}

// Toast style map - uses CSS variables from GoatKit theme when available
const toastStyleMap = {
    success: {
        indicator: "gk-indicator-success",
        indicatorStyle: "background: var(--gk-success, #22c55e);",
        border: "gk-toast-success",
        borderStyle: "border-color: var(--gk-success, #22c55e);",
        text: "",
        textStyle: "color: var(--gk-success, #22c55e);",
    },
    error: {
        indicator: "gk-indicator-error",
        indicatorStyle: "background: var(--gk-error, #ef4444);",
        border: "gk-toast-error",
        borderStyle: "border-color: var(--gk-error, #ef4444);",
        text: "",
        textStyle: "color: var(--gk-error, #ef4444);",
    },
    warning: {
        indicator: "gk-indicator-warning",
        indicatorStyle: "background: var(--gk-warning, #f59e0b);",
        border: "gk-toast-warning",
        borderStyle: "border-color: var(--gk-warning, #f59e0b);",
        text: "",
        textStyle: "color: var(--gk-warning, #f59e0b);",
    },
    info: {
        indicator: "gk-indicator-info",
        indicatorStyle: "background: var(--gk-info, var(--gk-primary, #3b82f6));",
        border: "gk-toast-info",
        borderStyle: "border-color: var(--gk-primary, #3b82f6);",
        text: "",
        textStyle: "color: var(--gk-primary, #3b82f6);",
    },
};

function displayToastInternal(message, type = "info", options = {}) {
    const container = ensureToastContainer();
    if (!container) {
        return null;
    }

    const text = message == null ? "" : String(message);
    const style = toastStyleMap[type] || toastStyleMap.info;

    const toast = document.createElement("div");
    toast.className =
        "pointer-events-auto max-w-sm w-full rounded-lg border shadow-lg p-4 space-y-3 transition transform duration-150 ease-out opacity-0 translate-y-2";
    toast.style.cssText = "background: var(--gk-bg-surface, #fff); border-color: var(--gk-border-default, #e5e7eb); box-shadow: var(--gk-shadow-lg, 0 10px 15px rgba(0,0,0,0.1));";
    toast.setAttribute("role", "status");
    toast.setAttribute("aria-live", "polite");

    // Apply type-specific border color
    if (style.borderStyle) {
        toast.style.cssText += style.borderStyle;
    }

    const row = document.createElement("div");
    row.className = "flex items-start gap-3";

    const indicator = document.createElement("span");
    indicator.className = "mt-1.5 inline-flex h-2 w-2 rounded-full";
    if (style.indicatorStyle) {
        indicator.style.cssText = style.indicatorStyle;
    }
    row.appendChild(indicator);

    const textWrap = document.createElement("div");
    textWrap.className = "flex-1 text-sm leading-snug";
    textWrap.style.cssText = "color: var(--gk-text-primary, #111827);";
    textWrap.textContent = text;
    row.appendChild(textWrap);

    const closeBtn = document.createElement("button");
    closeBtn.type = "button";
    closeBtn.className = "ml-auto transition-colors";
    closeBtn.style.cssText = "color: var(--gk-text-muted, #9ca3af);";
    closeBtn.onmouseover = function() { this.style.color = "var(--gk-text-primary, #111827)"; };
    closeBtn.onmouseout = function() { this.style.color = "var(--gk-text-muted, #9ca3af)"; };
    closeBtn.setAttribute("aria-label", "Dismiss notification");
    closeBtn.textContent = "x";
    closeBtn.addEventListener("click", () => removeToastElement(toast));
    row.appendChild(closeBtn);

    toast.appendChild(row);

    container.prepend(toast);
    requestAnimationFrame(() => {
        toast.classList.remove("opacity-0", "translate-y-2");
        toast.classList.add("opacity-100");
    });

    const duration =
        typeof options.autoHideMs === "number" ? options.autoHideMs : 4000;
    if (duration > 0) {
        setTimeout(() => removeToastElement(toast), duration);
    }

    return toast;
}

function showToast(message, type = "info", options = {}) {
    if (typeof type === "object" && options === undefined) {
        return displayToastInternal(message, undefined, type);
    }
    return displayToastInternal(message, type, options);
}

document.addEventListener("show-toast", (event) => {
    const detail = event && event.detail ? event.detail : {};
    if (!detail || typeof detail.message === "undefined") {
        return;
    }
    displayToastInternal(
        detail.message,
        detail.type || "info",
        detail.options || {},
    );
});

if (typeof window !== "undefined") {
    window.showToast = showToast;
}

// Preserve plain text formatting in ticket description if Tailwind prose collapses newlines.
document.addEventListener("DOMContentLoaded", () => {
    const el = document.getElementById("descriptionViewer");
    if (!el) return;
    // If server marked it as plain text (data-plain) collapse excessive blank lines only
    if (el.dataset && el.dataset.plain === "1") {
        const original = el.textContent;
        const collapsed = original.replace(/\n{4,}/g, "\n\n");
        if (collapsed !== original) el.textContent = collapsed;
        return;
    }
    // HTML description: no changes
});

(function () {
    const tracked = new Set();
    let timer;

    function formatDuration(seconds) {
        const value = Math.max(0, Math.round(seconds));
        const hours = Math.floor(value / 3600);
        const minutes = Math.floor((value % 3600) / 60);
        const secs = value % 60;
        const parts = [];
        if (hours > 0) parts.push(`${hours}h`);
        if (minutes > 0) parts.push(`${minutes}m`);
        if (hours === 0 && secs > 0) parts.push(`${secs}s`);
        if (parts.length === 0) return "0s";
        return parts.join(" ");
    }

    function formatSince(diffMs, dateObj) {
        let diff = diffMs;
        if (diff < 0) diff = 0;
        const seconds = Math.floor(diff / 1000);
        if (seconds < 60) return "just now";
        if (seconds < 120) return "1 minute ago";
        if (seconds < 3600) return `${Math.floor(seconds / 60)} minutes ago`;
        if (seconds < 7200) return "1 hour ago";
        if (seconds < 86400) return `${Math.floor(seconds / 3600)} hours ago`;
        if (seconds < 172800) return "1 day ago";
        if (seconds < 604800) return `${Math.floor(seconds / 86400)} days ago`;
        return dateObj.toISOString().slice(0, 10);
    }

    function updateElement(el, meta, nowMs) {
        const target = meta.timestamp.getTime();
        if (meta.mode === "since") {
            el.textContent = formatSince(nowMs - target, meta.timestamp);
            return;
        }
        let diff;
        if (meta.direction === "future") {
            diff = target - nowMs;
        } else if (meta.direction === "past") {
            diff = nowMs - target;
        } else {
            diff = Math.abs(nowMs - target);
        }
        el.textContent = formatDuration(diff / 1000);
    }

    function refreshTracked() {
        const now = Date.now();
        tracked.forEach((el) => {
            if (!el.isConnected || !el._relativeTime) {
                tracked.delete(el);
                return;
            }
            updateElement(el, el._relativeTime, now);
        });
        if (tracked.size === 0 && timer) {
            clearInterval(timer);
            timer = undefined;
        }
    }

    function startTimer() {
        if (timer) return;
        timer = setInterval(refreshTracked, 1000);
    }

    function setupElement(el) {
        const ts = el.getAttribute("data-timestamp");
        if (!ts) return;
        const dateObj = new Date(ts);
        if (Number.isNaN(dateObj.getTime())) return;
        const mode = el.getAttribute("data-mode") || "duration";
        const direction = el.getAttribute("data-direction") || "auto";
        el._relativeTime = { timestamp: dateObj, mode, direction };
        tracked.add(el);
        updateElement(el, el._relativeTime, Date.now());
    }

    function initRelativeTime(root) {
        if (!root) return;
        if (root.matches && root.matches("[data-relative-time]")) {
            setupElement(root);
        }
        const nodes = root.querySelectorAll
            ? root.querySelectorAll("[data-relative-time]")
            : [];
        nodes.forEach(setupElement);
        if (tracked.size > 0) startTimer();
    }

    document.addEventListener("DOMContentLoaded", () => {
        initRelativeTime(document);
    });

    if (document.body) {
        document.body.addEventListener("htmx:afterSwap", (event) => {
            initRelativeTime(event.target || document);
        });
    }
})();

// Pending Reminder Deck - Stacked notification cards with bulk actions
(function () {
    const POLL_INTERVAL_MS = 45000;
    const SNOOZE_PRESETS = [15, 60, 240]; // 15m, 1h, 4h
    let intervalId;
    let inFlight = false;
    let stopped = false;
    let failureCount = 0;
    let currentReminders = []; // Track all active reminders

    // Load reminder i18n from embedded JSON
    let reminderI18n = {};
    try {
        const i18nEl = document.getElementById("reminder-i18n");
        if (i18nEl) {
            reminderI18n = JSON.parse(i18nEl.textContent) || {};
        }
    } catch (e) {
        console.warn("Failed to load reminder i18n:", e);
    }
    const ri18n = (key, fallback) => reminderI18n[key] || fallback;

    function formatShortDuration(ms) {
        const totalSeconds = Math.max(0, Math.round(Math.abs(ms) / 1000));
        const totalMinutes = Math.floor(totalSeconds / 60);
        const totalHours = Math.floor(totalMinutes / 60);
        const totalDays = Math.floor(totalHours / 24);

        if (totalDays >= 365) {
            const years = Math.floor(totalDays / 365);
            const label = years !== 1 ? ri18n("years", "years") : ri18n("year", "year");
            return `${years} ${label}`;
        }
        if (totalDays >= 60) {
            const months = Math.floor(totalDays / 30);
            const label = months !== 1 ? ri18n("months", "months") : ri18n("month", "month");
            return `${months} ${label}`;
        }
        if (totalDays >= 14) {
            const weeks = Math.floor(totalDays / 7);
            const label = weeks !== 1 ? ri18n("weeks", "weeks") : ri18n("week", "week");
            return `${weeks} ${label}`;
        }
        if (totalDays >= 2) {
            return `${totalDays} ${ri18n("days", "days")}`;
        }
        if (totalDays === 1) {
            return `1 ${ri18n("day", "day")}`;
        }
        if (totalHours >= 1) {
            const hours = totalHours;
            const minutes = totalMinutes % 60;
            return minutes === 0 ? `${hours}h` : `${hours}h ${minutes}m`;
        }
        if (totalMinutes >= 1) {
            return `${totalMinutes}m`;
        }
        return `${Math.max(1, totalSeconds)}s`;
    }

    function formatSnoozeLabel(minutes, all = false) {
        const suffix = all ? " all" : "";
        if (minutes % 60 === 0 && minutes >= 60) {
            return `+${minutes / 60}h${suffix}`;
        }
        return `+${minutes}m${suffix}`;
    }

    // Snooze a single reminder (returns promise)
    async function snoozeReminderAPI(reminder, minutes) {
        const pendingUntil = reminder?.pending_until ? new Date(reminder.pending_until) : null;
        const baseMs = Math.max(Date.now(), pendingUntil ? pendingUntil.getTime() : Date.now());
        const nextAt = new Date(baseMs + minutes * 60000);
        const params = new URLSearchParams();
        params.set("status", reminder?.state_name || ri18n("pending_reminder", "pending reminder"));
        params.set("pending_until", nextAt.toISOString());

        const response = await fetch(`/api/tickets/${reminder.ticket_id}/status`, {
            method: "POST",
            credentials: "include",
            headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
            body: params,
        });
        const data = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(data?.error || ri18n("snooze_failed", "Failed to snooze reminder"));
        }
        return nextAt;
    }

    // Snooze all reminders
    async function snoozeAllReminders(minutes, button) {
        if (currentReminders.length === 0) return;

        const originalText = button.textContent;
        button.disabled = true;
        button.textContent = "...";

        let successCount = 0;
        let lastSnoozeTime = null;

        for (const reminder of [...currentReminders]) {
            try {
                lastSnoozeTime = await snoozeReminderAPI(reminder, minutes);
                successCount++;
            } catch (err) {
                console.error(`Failed to snooze reminder ${reminder.ticket_id}:`, err);
            }
        }

        button.disabled = false;
        button.textContent = originalText;

        if (successCount > 0) {
            showToast(
                `${ri18n("snoozed", "Snoozed")} ${successCount} ${ri18n("reminders", "reminders")}`,
                "success"
            );
            currentReminders = [];
            renderReminderDeck();
        }
    }

    // Snooze single reminder with UI feedback
    async function snoozeSingleReminder(reminder, minutes, button) {
        const originalText = button.textContent;
        button.disabled = true;
        button.textContent = "...";

        try {
            const nextAt = await snoozeReminderAPI(reminder, minutes);
            showToast(
                `${ri18n("snoozed_until", "Reminder snoozed until")} ${nextAt.toLocaleString()}`,
                "success"
            );
            currentReminders = currentReminders.filter(r => r.ticket_id !== reminder.ticket_id);
            renderReminderDeck();
        } catch (err) {
            console.error("pending reminder snooze failed", err);
            showToast(err?.message || ri18n("snooze_failed", "Failed to snooze reminder"), "error");
            button.disabled = false;
            button.textContent = originalText;
        }
    }

    // Dismiss a single reminder from the UI
    function dismissReminder(ticketId) {
        currentReminders = currentReminders.filter(r => r.ticket_id !== ticketId);
        renderReminderDeck();
    }

    // Dismiss all reminders from the UI
    function dismissAllReminders() {
        currentReminders = [];
        renderReminderDeck();
    }

    // Get or create the reminder deck container
    function ensureReminderDeck() {
        if (typeof document === "undefined" || !document.body) return null;

        let deck = document.getElementById("reminder-deck");
        if (!deck) {
            deck = document.createElement("div");
            deck.id = "reminder-deck";
            deck.className = "fixed bottom-6 right-6 z-50 w-80";
            document.body.appendChild(deck);
        }
        return deck;
    }

    // Render the entire reminder deck
    function renderReminderDeck() {
        const deck = ensureReminderDeck();
        if (!deck) return;

        deck.innerHTML = "";

        if (currentReminders.length === 0) {
            // Update toast position after clearing deck
            if (typeof updateToastContainerPosition === "function") {
                updateToastContainerPosition();
            }
            return;
        }

        // Sort reminders by pending_until (oldest/most overdue first)
        const sorted = [...currentReminders].sort((a, b) => {
            const aTime = a.pending_until ? new Date(a.pending_until).getTime() : 0;
            const bTime = b.pending_until ? new Date(b.pending_until).getTime() : 0;
            return aTime - bTime;
        });

        // Add bulk actions header if multiple reminders
        if (sorted.length > 1) {
            deck.appendChild(createBulkActionsHeader(sorted.length));
        }

        // Create deck container with stacking effect
        const stackContainer = document.createElement("div");
        stackContainer.className = "relative";
        stackContainer.style.minHeight = `${Math.min(sorted.length - 1, 4) * 8 + 180}px`;

        // Render stacked cards (bottom cards first for z-index stacking)
        const maxVisible = Math.min(sorted.length, 5);
        for (let i = maxVisible - 1; i >= 0; i--) {
            const reminder = sorted[i];
            const isTop = i === 0;
            const card = createReminderCard(reminder, isTop, i, sorted.length);
            stackContainer.appendChild(card);
        }

        deck.appendChild(stackContainer);

        // Update toast container position after deck is rendered
        // Use requestAnimationFrame to ensure DOM has been updated
        requestAnimationFrame(() => {
            if (typeof updateToastContainerPosition === "function") {
                updateToastContainerPosition();
            }
        });
    }

    // Create bulk actions header
    function createBulkActionsHeader(count) {
        const header = document.createElement("div");
        header.className = "mb-2 p-2 rounded-lg flex items-center justify-between gap-2";
        header.style.cssText = `
            background: var(--gk-bg-elevated, #1f2937);
            border: 1px solid var(--gk-warning, #f59e0b);
            box-shadow: var(--gk-glow-warning, none);
        `;

        const counter = document.createElement("span");
        counter.className = "text-sm font-semibold";
        counter.style.color = "var(--gk-warning, #f59e0b)";
        counter.textContent = `${count} ${ri18n("reminders", "reminders")}`;
        header.appendChild(counter);

        const actions = document.createElement("div");
        actions.className = "flex gap-1";

        SNOOZE_PRESETS.forEach((minutes) => {
            const btn = document.createElement("button");
            btn.type = "button";
            btn.className = "px-2 py-1 text-xs font-medium rounded transition-colors";
            btn.style.cssText = `
                background: transparent;
                color: var(--gk-warning, #f59e0b);
                border: 1px solid var(--gk-warning, #f59e0b);
            `;
            btn.textContent = formatSnoozeLabel(minutes, true);
            btn.addEventListener("mouseenter", () => {
                btn.style.background = "var(--gk-warning-subtle, rgba(245, 158, 11, 0.2))";
            });
            btn.addEventListener("mouseleave", () => {
                btn.style.background = "transparent";
            });
            btn.addEventListener("click", () => snoozeAllReminders(minutes, btn));
            actions.appendChild(btn);
        });

        const dismissAllBtn = document.createElement("button");
        dismissAllBtn.type = "button";
        dismissAllBtn.className = "px-2 py-1 text-xs rounded ml-1";
        dismissAllBtn.style.cssText = `color: var(--gk-text-muted, #9ca3af); background: transparent;`;
        dismissAllBtn.innerHTML = "&#x2715;";
        dismissAllBtn.title = ri18n("dismiss_all", "Dismiss all");
        dismissAllBtn.addEventListener("click", dismissAllReminders);
        actions.appendChild(dismissAllBtn);

        header.appendChild(actions);
        return header;
    }

    // Create a single reminder card
    function createReminderCard(reminder, isTop, index, total) {
        const card = document.createElement("div");
        card.className = "rounded-lg overflow-hidden transition-all duration-200";

        const offset = index * 8;
        const scale = 1 - (index * 0.03);
        const zIndex = total - index;

        card.style.cssText = `
            position: absolute;
            bottom: ${offset}px;
            left: ${index * 4}px;
            right: ${index * 4}px;
            z-index: ${zIndex};
            transform: scale(${scale});
            transform-origin: bottom center;
            background: var(--gk-bg-surface, #111827);
            border: 1px solid var(--gk-warning, #f59e0b);
            box-shadow: var(--gk-glow-warning, none), var(--gk-shadow-lg, 0 10px 15px rgba(0,0,0,0.2));
            opacity: ${isTop ? 1 : 0.85 - (index * 0.1)};
        `;

        if (isTop) {
            card.style.position = "relative";
            card.style.bottom = "0";
            card.style.left = "0";
            card.style.right = "0";
            card.style.transform = "none";
            card.style.opacity = "1";
            card.appendChild(createExpandedCardContent(reminder));
        } else {
            card.appendChild(createCollapsedCardContent(reminder));
        }

        return card;
    }

    // Create expanded card content (for top card)
    function createExpandedCardContent(reminder) {
        const content = document.createElement("div");
        content.className = "p-4 space-y-3";

        // Header
        const header = document.createElement("div");
        header.className = "flex items-start justify-between gap-3";

        const titleWrap = document.createElement("div");
        titleWrap.className = "flex-1 min-w-0";
        const heading = document.createElement("div");
        heading.className = "text-sm font-semibold truncate";
        heading.style.color = "var(--gk-text-primary, #f3f4f6)";
        heading.textContent = reminder.title || ri18n("ticket_reminder", "Ticket reminder");
        titleWrap.appendChild(heading);

        const subline = document.createElement("div");
        subline.className = "text-xs truncate";
        subline.style.color = "var(--gk-text-secondary, #9ca3af)";
        subline.textContent = `${ri18n("ticket_label", "Ticket")} #${reminder.ticket_number || reminder.ticket_id} • ${reminder.queue_name || "Queue"}`;
        titleWrap.appendChild(subline);
        header.appendChild(titleWrap);

        const dismiss = document.createElement("button");
        dismiss.type = "button";
        dismiss.className = "text-lg leading-none flex-shrink-0";
        dismiss.style.color = "var(--gk-text-muted, #6b7280)";
        dismiss.innerHTML = "&#x2715;";
        dismiss.addEventListener("click", () => dismissReminder(reminder.ticket_id));
        header.appendChild(dismiss);
        content.appendChild(header);

        // Timing
        const due = reminder?.pending_until ? new Date(reminder.pending_until) : null;
        const now = new Date();
        let timingLabel = ri18n("reminder_ready", "Reminder is ready");
        let isOverdue = false;
        if (due) {
            const diffMs = due.getTime() - now.getTime();
            isOverdue = diffMs < 0;
            timingLabel = isOverdue
                ? `${ri18n("overdue_by", "Overdue by")} ${formatShortDuration(diffMs)}`
                : `${ri18n("due_in", "Due in")} ${formatShortDuration(diffMs)}`;
        }

        const timing = document.createElement("div");
        timing.className = "text-sm font-medium";
        timing.style.color = isOverdue ? "var(--gk-error, #ef4444)" : "var(--gk-warning, #f59e0b)";
        timing.textContent = timingLabel;
        content.appendChild(timing);

        if (due) {
            const exact = document.createElement("div");
            exact.className = "text-xs";
            exact.style.color = "var(--gk-text-muted, #6b7280)";
            exact.textContent = `${ri18n("reminder_at", "Reminder at")} ${due.toLocaleString()}`;
            content.appendChild(exact);
        }

        // Actions
        const actions = document.createElement("div");
        actions.className = "flex flex-wrap gap-2 pt-2";

        const openLink = document.createElement("a");
        openLink.href = `/agent/tickets/${reminder.ticket_id}`;
        openLink.className = "px-3 py-1.5 rounded-md text-sm font-semibold transition-colors";
        openLink.style.cssText = `background: var(--gk-primary, #3b82f6); color: var(--gk-text-inverse, #ffffff);`;
        openLink.textContent = ri18n("open_ticket", "Open Ticket");
        openLink.addEventListener("click", () => dismissReminder(reminder.ticket_id));
        actions.appendChild(openLink);

        SNOOZE_PRESETS.forEach((minutes) => {
            const btn = document.createElement("button");
            btn.type = "button";
            btn.className = "px-3 py-1.5 rounded-md text-sm font-medium transition-colors";
            btn.style.cssText = `background: transparent; color: var(--gk-warning, #f59e0b); border: 1px solid var(--gk-warning, #f59e0b);`;
            btn.textContent = formatSnoozeLabel(minutes);
            btn.addEventListener("mouseenter", () => {
                btn.style.background = "var(--gk-warning-subtle, rgba(245, 158, 11, 0.2))";
            });
            btn.addEventListener("mouseleave", () => {
                btn.style.background = "transparent";
            });
            btn.addEventListener("click", () => snoozeSingleReminder(reminder, minutes, btn));
            actions.appendChild(btn);
        });

        content.appendChild(actions);
        return content;
    }

    // Create collapsed card content (for stacked cards)
    function createCollapsedCardContent(reminder) {
        const content = document.createElement("div");
        content.className = "px-4 py-2";

        const title = document.createElement("div");
        title.className = "text-xs truncate";
        title.style.color = "var(--gk-text-secondary, #9ca3af)";
        title.textContent = `#${reminder.ticket_number || reminder.ticket_id} • ${reminder.title || "Reminder"}`;
        content.appendChild(title);

        return content;
    }

    async function pollReminders() {
        if (inFlight || stopped) return;

        inFlight = true;
        try {
            const response = await fetch("/api/notifications/pending", {
                credentials: "include",
                headers: { Accept: "application/json" },
            });

            if (response.status === 401 || response.status === 403) {
                stopped = true;
                if (intervalId) {
                    clearInterval(intervalId);
                    intervalId = undefined;
                }
                return;
            }

            const payload = await response.json().catch(() => null);
            if (payload?.success && Array.isArray(payload.data?.reminders)) {
                const newReminders = payload.data.reminders;
                newReminders.forEach((newR) => {
                    const existingIndex = currentReminders.findIndex(r => r.ticket_id === newR.ticket_id);
                    if (existingIndex >= 0) {
                        currentReminders[existingIndex] = newR;
                    } else {
                        currentReminders.push(newR);
                    }
                });
                renderReminderDeck();
            }
            failureCount = 0;
        } catch (err) {
            failureCount += 1;
            console.error("pending reminder poll failed", err);
            if (failureCount > 3) {
                stopped = true;
                if (intervalId) {
                    clearInterval(intervalId);
                    intervalId = undefined;
                }
            }
        } finally {
            inFlight = false;
        }
    }

    function startInterval() {
        if (intervalId) clearInterval(intervalId);
        intervalId = setInterval(() => {
            if (!document.hidden && !stopped) pollReminders();
        }, POLL_INTERVAL_MS);
    }

    function init() {
        if (stopped) return;
        if (!ensureReminderDeck()) return;
        pollReminders();
        startInterval();
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
    } else {
        init();
    }

    document.addEventListener("visibilitychange", () => {
        if (!document.hidden) pollReminders();
    });

    if (typeof window !== "undefined") {
        window.addEventListener("focus", pollReminders);
    }
})();
