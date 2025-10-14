/**
 * Common JavaScript utilities for GOTRS
 */

/**
 * Enhanced fetch wrapper that automatically includes credentials for API calls
 */
function apiFetch(url, options = {}) {
        // If it's an API call, include credentials
        if (url.startsWith('/api/') || url.startsWith('/api/v1/') || url.startsWith('/agent/')) {
                options.credentials = options.credentials || 'include';
        }

        return fetch(url, options)
            .then((response) => {
                // Trigger Guru overlay if server flagged an error explicitly
                const guruMsg = response.headers && response.headers.get && response.headers.get('X-Guru-Error');
                if (guruMsg) {
                        const code = (response.status === 401 || response.status === 403) ? '00000008.CAFEBABE' : (response.status >= 500 ? '0000000A.BADF00D' : '00000009.BADC0DE');
                        try { showGuruMeditation(`${guruMsg}\n\n${url} [${response.status} ${response.statusText}]`, code); } catch(_) {}
                }
                // Auto‑trigger Guru Meditation for auth and server errors; let callers still handle inline
                if (!response.ok && (response.status >= 500 || response.status === 401 || response.status === 403)) {
                        const code = (response.status === 401 || response.status === 403) ? '00000008.CAFEBABE' : '0000000A.BADF00D';
                        const msg = `Request failed: ${url} [${response.status} ${response.statusText}]`;
                        try { showGuruMeditation(msg, code); } catch(_) {}
                }
                return response;
            })
            .catch((err) => {
                // Network failures also surface the Guru overlay
                const msg = `Network error calling ${url}: ${err && err.message ? err.message : 'Unknown error'}`;
                try { showGuruMeditation(msg, '0000000B.NETERR01'); } catch(_) {}
                throw err;
            });
}

// Display an Amiga-style Guru Meditation overlay for critical errors
function showGuruMeditation(message, code = '00000007.DEADBEEF') {
        // Avoid stacking many overlays
        if (document.getElementById('guru-meditation-overlay')) return;
        const wrapper = document.createElement('div');
        wrapper.id = 'guru-meditation-overlay';
        wrapper.style.position = 'fixed';
        wrapper.style.inset = '0';
        wrapper.style.zIndex = '99999';
        wrapper.style.display = 'flex';
        wrapper.style.alignItems = 'center';
        wrapper.style.justifyContent = 'center';
        wrapper.style.background = 'rgba(0,0,0,0.6)';
        wrapper.innerHTML = `
            <div id="guru-meditation" class="cursor-pointer" style="border: 8px solid #ff0000; background:#000; color:#fff; max-width: 800px; width: calc(100% - 48px); padding: 24px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, \"Liberation Mono\", \"Courier New\", monospace;">
                <div style="text-align:center; margin-bottom: 8px;">Software Failure.    Click to continue.</div>
                <div style="text-align:center; font-size: 20px; font-weight: 700; margin-bottom: 12px;">Guru Meditation #${code}</div>
                <div style="white-space: pre-wrap; word-break: break-word; font-size: 13px; color:#ddd;">${message || ''}</div>
            </div>`;
        // Click anywhere to dismiss
        wrapper.addEventListener('click', () => {
                wrapper.remove();
        });
        document.body.appendChild(wrapper);
}

/**
 * Show toast notification
 */
function showToast(message, type = 'info') {
    // Create toast element if it doesn't exist
    let toast = document.getElementById('toast-notification');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'toast-notification';
        toast.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 15px 20px;
            border-radius: 4px;
            color: white;
            font-weight: bold;
            z-index: 9999;
            opacity: 0;
            transition: opacity 0.3s ease;
        `;
        document.body.appendChild(toast);
    }

    // Set message and type
    toast.textContent = message;
    toast.className = `toast-${type}`;

    // Set background color based on type
    const colors = {
        success: '#22c55e',
        error: '#ef4444',
        warning: '#f59e0b',
        info: '#3b82f6'
    };
    toast.style.backgroundColor = colors[type] || colors.info;

    // Show toast
    toast.style.opacity = '1';

    // Hide after 3 seconds
    setTimeout(() => {
        toast.style.opacity = '0';
    }, 3000);
}

// Preserve plain text formatting in ticket description if Tailwind prose collapses newlines.
document.addEventListener('DOMContentLoaded', () => {
    const el = document.getElementById('descriptionViewer');
    if (!el) return;
    // If server marked it as plain text (data-plain) collapse excessive blank lines only
    if (el.dataset && el.dataset.plain === '1') {
        const original = el.textContent;
        const collapsed = original.replace(/\n{4,}/g, '\n\n');
        if (collapsed !== original) el.textContent = collapsed;
        return;
    }
    // HTML description: no changes
});