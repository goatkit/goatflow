/**
 * Common JavaScript utilities for GOTRS
 */

/**
 * Enhanced fetch wrapper that automatically includes credentials for API calls
 */
function apiFetch(url, options = {}) {
    // If it's an API call, include credentials
    if (url.startsWith('/api/') || url.startsWith('/api/v1/')) {
        options.credentials = options.credentials || 'include';
    }

    return fetch(url, options);
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