/**
 * GoatKit Theme Manager
 * Handles theme and mode switching with persistence
 *
 * Usage:
 *   ThemeManager.toggleMode()           // Toggle light/dark
 *   ThemeManager.applyTheme('synthwave') // Switch theme
 *   ThemeManager.getCurrentMode()       // Get current mode
 *   ThemeManager.getCurrentTheme()      // Get current theme name
 */
const ThemeManager = (function() {
  'use strict';

  // Storage keys
  const STORAGE_KEY_MODE = 'gk-theme-mode';
  const STORAGE_KEY_THEME = 'gk-theme-name';

  // Available themes - add new themes here
  const AVAILABLE_THEMES = ['synthwave', 'goatflow-classic', 'seventies-vibes', 'nineties-vibe'];
  const DEFAULT_THEME = 'synthwave';

  // Built-in themes are in /static/themes/builtin/
  // Community themes are in /static/themes/.cache/ (extracted from ZIP packages)
  const BUILTIN_THEMES = ['synthwave', 'goatflow-classic', 'seventies-vibes', 'nineties-vibe'];

  /**
   * Get the base path for a theme
   * @param {string} themeId - Theme identifier
   * @returns {string} Base path to theme directory
   */
  function getThemeBasePath(themeId) {
    if (BUILTIN_THEMES.includes(themeId)) {
      return '/static/themes/builtin/' + themeId;
    }
    // Community themes are extracted to .cache/
    return '/static/themes/.cache/' + themeId;
  }

  // Theme metadata - SINGLE SOURCE OF TRUTH for all theme display info
  // Used by theme selectors throughout the app
  const THEME_METADATA = {
    'synthwave': {
      name: 'Synthwave',
      nameKey: 'theme.synthwave',
      description: 'Neon retro vibes',
      descriptionKey: 'theme.synthwave_desc',
      gradient: 'linear-gradient(135deg, #00E5FF, #FF2FD4)',
      hasFonts: true
    },
    'goatflow-classic': {
      name: 'Classic',
      nameKey: 'theme.classic',
      description: 'Clean & professional',
      descriptionKey: 'theme.classic_desc',
      gradient: 'linear-gradient(135deg, #3b82f6, #4f46e5)',
      hasFonts: false  // Uses system fonts
    },
    'seventies-vibes': {
      name: '70s Vibes',
      nameKey: 'theme.seventies',
      description: 'Warm earth tones',
      descriptionKey: 'theme.seventies_desc',
      gradient: 'linear-gradient(135deg, #D35400, #F5A623)',
      hasFonts: true
    },
    'nineties-vibe': {
      name: '90s Vibe',
      nameKey: 'theme.nineties',
      description: 'Retro desktop (light) / Terminal (dark)',
      descriptionKey: 'theme.nineties_desc',
      gradient: 'linear-gradient(180deg, #808080, #333333)',
      hasFonts: true
    }
  };

  /**
   * Load fonts for a specific theme dynamically
   * @param {string} themeName - The theme to load fonts for
   */
  function loadThemeFonts(themeName) {
    // Remove previously loaded theme fonts
    document.querySelectorAll('link[data-theme-font]').forEach(function(el) {
      el.remove();
    });

    var meta = THEME_METADATA[themeName];
    if (!meta || !meta.hasFonts) {
      return;
    }

    // Build font CSS path from theme package structure
    var fontCss = getThemeBasePath(themeName) + '/fonts/fonts.css';

    // Inject link element for vendored font CSS
    var link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = fontCss;
    link.setAttribute('data-theme-font', themeName);
    document.head.appendChild(link);
  }

  /**
   * Check if user is authenticated (has logged-in indicator cookie)
   * Note: auth tokens are httpOnly (invisible to JS), so we use a separate indicator
   * Checks both agent (goatflow_logged_in) and customer (goatflow_customer_logged_in) cookies
   * @returns {boolean}
   */
  function isAuthenticated() {
    return document.cookie.includes('goatflow_logged_in=') ||
           document.cookie.includes('goatflow_customer_logged_in=');
  }

  /**
   * Persist theme preferences to server (if authenticated)
   * @param {string} [theme] - Theme name
   * @param {string} [mode] - Mode (light/dark)
   */
  function persistToServer(theme, mode) {
    // Only persist if authenticated
    if (!isAuthenticated()) {
      return;
    }

    const payload = {};
    if (theme) payload.theme = theme;
    if (mode) payload.mode = mode;

    if (Object.keys(payload).length === 0) {
      return;
    }

    // Determine API endpoint based on portal (customer portal has /customer prefix)
    var endpoint = '/api/preferences/theme';
    if (window.location.pathname.startsWith('/customer')) {
      endpoint = '/customer/api/preferences/theme';
    }

    // Fire and forget - don't block UI for persistence
    fetch(endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'same-origin',
      body: JSON.stringify(payload)
    }).catch(function(err) {
      console.warn('[ThemeManager] Failed to persist theme to server:', err);
    });
  }

  /**
   * Get the current color mode
   * @returns {'light'|'dark'} The current mode
   */
  function getCurrentMode() {
    const stored = localStorage.getItem(STORAGE_KEY_MODE);
    if (stored) {
      return stored;
    }
    // Fall back to system preference, then to dark
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    return prefersDark ? 'dark' : 'light';
  }

  /**
   * Get the current theme name
   * @returns {string} The current theme name
   */
  function getCurrentTheme() {
    return localStorage.getItem(STORAGE_KEY_THEME) || DEFAULT_THEME;
  }

  /**
   * Apply a color mode (light/dark)
   * @param {'light'|'dark'} mode - The mode to apply
   * @param {boolean} [persist=true] - Whether to persist to server
   */
  function applyMode(mode, persist) {
    if (persist === undefined) persist = true;
    const html = document.documentElement;

    // Remove both classes and add the new one
    html.classList.remove('light', 'dark');
    html.classList.add(mode);

    // Persist to localStorage
    localStorage.setItem(STORAGE_KEY_MODE, mode);

    // Persist to server (if authenticated)
    if (persist) {
      persistToServer(null, mode);
    }

    // Dispatch event for other components
    document.dispatchEvent(new CustomEvent('gk:theme-mode-change', {
      detail: { mode }
    }));
  }

  /**
   * Apply a theme
   * @param {string} themeName - The theme name to apply
   * @param {boolean} [persist=true] - Whether to persist to server
   */
  function applyTheme(themeName, persist) {
    if (persist === undefined) persist = true;

    // Validate theme name
    if (!AVAILABLE_THEMES.includes(themeName)) {
      console.warn(`[ThemeManager] Theme "${themeName}" not found. Available: ${AVAILABLE_THEMES.join(', ')}`);
      themeName = DEFAULT_THEME;
    }

    // Load theme-specific fonts (if any)
    loadThemeFonts(themeName);

    // Update the theme stylesheet link if it exists
    const themeLink = document.getElementById('gk-theme-stylesheet');
    if (themeLink) {
      themeLink.href = getThemeBasePath(themeName) + '/theme.css';
    }

    // Set data attribute for CSS selectors
    document.documentElement.setAttribute('data-theme', themeName);

    // Persist to localStorage
    localStorage.setItem(STORAGE_KEY_THEME, themeName);

    // Persist to server (if authenticated)
    if (persist) {
      persistToServer(themeName, null);
    }

    // Dispatch event
    document.dispatchEvent(new CustomEvent('gk:theme-change', {
      detail: { theme: themeName }
    }));
  }

  /**
   * Toggle between light and dark modes
   * @returns {'light'|'dark'} The new mode
   */
  function toggleMode() {
    const current = getCurrentMode();
    const newMode = current === 'dark' ? 'light' : 'dark';
    applyMode(newMode);
    return newMode;
  }

  /**
   * Set both theme and mode at once
   * @param {string} themeName - The theme name
   * @param {'light'|'dark'} [mode] - Optional mode override
   */
  function setTheme(themeName, mode) {
    applyTheme(themeName);
    if (mode) {
      applyMode(mode);
    }
  }

  /**
   * Get list of available themes
   * @returns {string[]} Array of theme names
   */
  function getAvailableThemes() {
    return [...AVAILABLE_THEMES];
  }

  /**
   * Get metadata for a specific theme
   * @param {string} themeId - Theme identifier
   * @returns {Object|null} Theme metadata or null if not found
   */
  function getThemeMetadata(themeId) {
    return THEME_METADATA[themeId] || null;
  }

  /**
   * Get metadata for all themes
   * @returns {Object} All theme metadata keyed by theme ID
   */
  function getAllThemeMetadata() {
    return { ...THEME_METADATA };
  }

  /**
   * Check if system prefers dark mode
   * @returns {boolean}
   */
  function systemPrefersDark() {
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  }

  /**
   * Listen for system theme changes
   */
  function setupSystemThemeListener() {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    mediaQuery.addEventListener('change', function(e) {
      // Only auto-switch if user hasn't explicitly set a preference
      if (!localStorage.getItem(STORAGE_KEY_MODE)) {
        // Don't persist to server - this is an automatic system change, not user choice
        applyMode(e.matches ? 'dark' : 'light', false);
      }
    });
  }

  /**
   * Initialize the theme manager
   * Called automatically on load
   */
  function init() {
    const mode = getCurrentMode();
    const theme = getCurrentTheme();

    // Apply immediately
    document.documentElement.classList.add(mode);
    document.documentElement.setAttribute('data-theme', theme);

    // Load theme-specific fonts on initial load
    loadThemeFonts(theme);

    // Set up system preference listener
    setupSystemThemeListener();
  }

  // Auto-initialize
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  // Public API
  return {
    getCurrentMode,
    getCurrentTheme,
    applyMode,
    applyTheme,
    toggleMode,
    setTheme,
    getAvailableThemes,
    getThemeMetadata,
    getAllThemeMetadata,
    systemPrefersDark,
    AVAILABLE_THEMES,
    THEME_METADATA,
    DEFAULT_THEME
  };
})();

// Expose globally
window.ThemeManager = ThemeManager;
