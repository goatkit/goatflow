/**
 * Common JavaScript utilities for GOTRS
 */

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
        container.className =
            "fixed bottom-0 right-0 p-6 space-y-4 z-50 pointer-events-none";
        body.appendChild(container);
    } else if (!container.classList.contains("pointer-events-none")) {
        container.classList.add("pointer-events-none");
    }
    return container;
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

const toastStyleMap = {
    success: {
        indicator: "bg-green-500",
        border: "border-green-200 dark:border-green-500",
        text: "text-green-900 dark:text-green-100",
    },
    error: {
        indicator: "bg-red-500",
        border: "border-red-200 dark:border-red-500",
        text: "text-red-900 dark:text-red-100",
    },
    warning: {
        indicator: "bg-amber-500",
        border: "border-amber-300 dark:border-amber-500",
        text: "text-amber-900 dark:text-amber-100",
    },
    info: {
        indicator: "bg-blue-500",
        border: "border-blue-200 dark:border-blue-400",
        text: "text-blue-900 dark:text-blue-100",
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
        "pointer-events-auto max-w-sm w-full rounded-lg border border-gray-200 dark:border-gray-700 shadow-lg bg-white dark:bg-gray-900 p-4 space-y-3 transition transform duration-150 ease-out opacity-0 translate-y-2";
    toast.setAttribute("role", "status");
    toast.setAttribute("aria-live", "polite");

    if (style.border) {
        style.border
            .split(" ")
            .forEach((token) => token && toast.classList.add(token));
    }

    const row = document.createElement("div");
    row.className = "flex items-start gap-3";

    const indicator = document.createElement("span");
    indicator.className =
        `mt-1.5 inline-flex h-2 w-2 rounded-full ${style.indicator || ""}`.trim();
    row.appendChild(indicator);

    const textWrap = document.createElement("div");
    textWrap.className =
        `flex-1 text-sm leading-snug ${style.text || "text-gray-900 dark:text-gray-100"}`.trim();
    textWrap.textContent = text;
    row.appendChild(textWrap);

    const closeBtn = document.createElement("button");
    closeBtn.type = "button";
    closeBtn.className =
        "ml-auto text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300";
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

(function () {
    const POLL_INTERVAL_MS = 45000;
    const SNOOZE_PRESETS = [15, 60, 240];
    let intervalId;
    let inFlight = false;
    let stopped = false;
    let failureCount = 0;

    function formatShortDuration(ms) {
        const totalMinutes = Math.max(0, Math.round(Math.abs(ms) / 60000));
        if (totalMinutes >= 60) {
            const hours = Math.floor(totalMinutes / 60);
            const minutes = totalMinutes % 60;
            if (minutes === 0) {
                return `${hours}h`;
            }
            return `${hours}h ${minutes}m`;
        }
        if (totalMinutes >= 1) {
            return `${totalMinutes}m`;
        }
        const totalSeconds = Math.max(1, Math.round(Math.abs(ms) / 1000));
        return `${totalSeconds}s`;
    }

    function formatSnoozeLabel(minutes) {
        if (minutes % 60 === 0 && minutes >= 60) {
            const hours = minutes / 60;
            return `+${hours}h`;
        }
        return `+${minutes}m`;
    }

    function snoozeReminder(reminder, minutes, toast, button) {
        const pendingUntil =
            reminder && reminder.pending_until
                ? new Date(reminder.pending_until)
                : null;
        const baseMs = Math.max(
            Date.now(),
            pendingUntil ? pendingUntil.getTime() : Date.now(),
        );
        const nextAt = new Date(baseMs + minutes * 60000);
        const params = new URLSearchParams();
        params.set(
            "status",
            (reminder && reminder.state_name) || "pending reminder",
        );
        params.set("pending_until", nextAt.toISOString());

        let restoreText = "";
        if (button) {
            restoreText = button.textContent;
            button.disabled = true;
            button.classList.add("opacity-60", "cursor-not-allowed");
            button.textContent = "...";
        }

        apiFetch(`/api/tickets/${reminder.ticket_id}/status`, {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: params,
        })
            .then(async (response) => {
                const data = await response.json().catch(() => ({}));
                if (!response.ok) {
                    const msg =
                        data && data.error
                            ? data.error
                            : "Failed to snooze reminder";
                    throw new Error(msg);
                }
                showToast(
                    `Reminder snoozed until ${nextAt.toLocaleString()}`,
                    "success",
                );
                if (toast) {
                    removeToastElement(toast);
                }
            })
            .catch((err) => {
                console.error("pending reminder snooze failed", err);
                showToast(
                    err && err.message
                        ? err.message
                        : "Failed to snooze reminder",
                    "error",
                );
            })
            .finally(() => {
                if (button) {
                    button.disabled = false;
                    button.classList.remove("opacity-60", "cursor-not-allowed");
                    button.textContent =
                        restoreText || formatSnoozeLabel(minutes);
                }
            });
    }

    function renderReminderToast(reminder) {
        if (!reminder || typeof reminder !== "object" || !reminder.ticket_id) {
            return;
        }
        const container = ensureToastContainer();
        if (!container) {
            return;
        }

        const toastId = `pending-reminder-${reminder.ticket_id}-${reminder.pending_until_unix || 0}`;
        const existing = container.querySelector(
            `[data-toast-id="${toastId}"]`,
        );
        if (existing) {
            existing.remove();
        }

        const toast = document.createElement("div");
        toast.dataset.toastId = toastId;
        toast.className =
            "pointer-events-auto max-w-md w-full rounded-lg border border-amber-300 dark:border-amber-500 bg-white dark:bg-gray-900 shadow-lg p-4 space-y-3 transition transform duration-150 ease-out opacity-0 translate-y-2";
        toast.setAttribute("role", "alert");

        const header = document.createElement("div");
        header.className = "flex items-start justify-between gap-3";

        const titleWrap = document.createElement("div");
        titleWrap.className = "flex flex-col";
        const heading = document.createElement("div");
        heading.className =
            "text-sm font-semibold text-amber-900 dark:text-amber-100";
        heading.textContent = reminder.title || "Ticket reminder";
        titleWrap.appendChild(heading);

        const subline = document.createElement("div");
        subline.className = "text-xs text-gray-600 dark:text-gray-300";
        const ticketLabel = reminder.ticket_number || reminder.ticket_id;
        const queueName = reminder.queue_name || "Queue";
        subline.textContent = `Ticket #${ticketLabel} • ${queueName}`;
        titleWrap.appendChild(subline);
        header.appendChild(titleWrap);

        const dismiss = document.createElement("button");
        dismiss.type = "button";
        dismiss.className =
            "text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300";
        dismiss.setAttribute("aria-label", "Dismiss reminder");
        dismiss.textContent = "x";
        dismiss.addEventListener("click", () => removeToastElement(toast));
        header.appendChild(dismiss);
        toast.appendChild(header);

        const due =
            reminder && reminder.pending_until
                ? new Date(reminder.pending_until)
                : null;
        const now = new Date();
        let timingLabel = "Reminder is ready";
        if (due) {
            const diffMs = due.getTime() - now.getTime();
            timingLabel =
                diffMs < 0
                    ? `Overdue by ${formatShortDuration(diffMs)}`
                    : `Due in ${formatShortDuration(diffMs)}`;
        }

        const timing = document.createElement("div");
        timing.className =
            "text-sm font-medium text-amber-700 dark:text-amber-200";
        timing.textContent = timingLabel;
        toast.appendChild(timing);

        if (due) {
            const exact = document.createElement("div");
            exact.className = "text-xs text-gray-500 dark:text-gray-400";
            exact.textContent = `Reminder at ${due.toLocaleString()}`;
            toast.appendChild(exact);
        }

        if (reminder.state_name) {
            const state = document.createElement("div");
            state.className =
                "text-xs uppercase tracking-wide text-amber-600 dark:text-amber-300";
            state.textContent = reminder.state_name;
            toast.appendChild(state);
        }

        const actions = document.createElement("div");
        actions.className = "flex flex-wrap gap-2 pt-2";

        const openLink = document.createElement("a");
        openLink.href = `/agent/tickets/${reminder.ticket_id}`;
        openLink.className =
            "px-3 py-1.5 rounded-md text-sm font-semibold bg-blue-600 text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500";
        openLink.textContent = "Open Ticket";
        openLink.addEventListener("click", () => removeToastElement(toast));
        actions.appendChild(openLink);

        SNOOZE_PRESETS.forEach((minutes) => {
            const btn = document.createElement("button");
            btn.type = "button";
            btn.className =
                "px-3 py-1.5 rounded-md text-sm font-medium border border-amber-400 text-amber-700 hover:bg-amber-50 dark:text-amber-200 dark:border-amber-500 dark:hover:bg-amber-900/30";
            btn.textContent = formatSnoozeLabel(minutes);
            btn.addEventListener("click", () =>
                snoozeReminder(reminder, minutes, toast, btn),
            );
            actions.appendChild(btn);
        });

        toast.appendChild(actions);

        container.prepend(toast);
        requestAnimationFrame(() => {
            toast.classList.remove("opacity-0", "translate-y-2");
            toast.classList.add("opacity-100");
        });
    }

    async function pollReminders() {
        if (inFlight || stopped) {
            return;
        }
        const container = ensureToastContainer();
        if (!container) {
            return;
        }

        inFlight = true;
        try {
            // Use plain fetch instead of apiFetch since we need access to response.status
            // and apiFetch already parses JSON before returning
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
            if (
                payload &&
                payload.success &&
                payload.data &&
                Array.isArray(payload.data.reminders)
            ) {
                payload.data.reminders.forEach(renderReminderToast);
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
        if (intervalId) {
            clearInterval(intervalId);
        }
        intervalId = setInterval(() => {
            if (!document.hidden && !stopped) {
                pollReminders();
            }
        }, POLL_INTERVAL_MS);
    }

    function init() {
        if (stopped) {
            return;
        }
        if (!ensureToastContainer()) {
            return;
        }
        pollReminders();
        startInterval();
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
    } else {
        init();
    }

    document.addEventListener("visibilitychange", () => {
        if (!document.hidden) {
            pollReminders();
        }
    });

    if (typeof window !== "undefined") {
        window.addEventListener("focus", pollReminders);
    }
})();
