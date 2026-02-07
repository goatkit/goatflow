# GoatFlow Theme Packages

## Implementation Status

| Feature | Status |
|---------|--------|
| Self-contained theme directories | ✅ Implemented |
| Built-in themes in `static/themes/builtin/` | ✅ Implemented |
| Theme CSS with variables | ✅ Implemented |
| Relative font paths within themes | ✅ Implemented |
| Backend theme discovery | ✅ Implemented |
| Frontend ThemeManager updates | ✅ Implemented |
| theme.yaml metadata files | ✅ Implemented (structure only) |
| Community themes in `.cache/` | 🔧 Directory ready |
| ZIP package storage | 📋 Planned |
| ZIP extraction on startup | 📋 Planned |
| Admin theme upload UI | 📋 Planned |
| Sound events support | 📋 Planned |

---

## Overview

GoatFlow Theme Packages are self-contained bundles that include all theme assets, enabling:

- **Community sharing** - Themes distributed as zip files (planned)
- **Easy installation** - Upload a zip, it just works (planned)
- **Complete customization** - Fonts, images, sounds, and styles in one package
- **Air-gapped deployment** - All assets vendored, no external dependencies

---

## Current Implementation

### Directory Structure (✅ Implemented)

```
static/themes/
├── builtin/                     # ✅ Ships with GoatFlow
│   ├── synthwave/
│   │   ├── theme.yaml           # Theme metadata
│   │   ├── theme.css            # Main stylesheet
│   │   └── fonts/
│   │       ├── fonts.css        # @font-face with relative paths
│   │       └── space-grotesk/   # Font files
│   ├── goatflow-classic/
│   │   ├── theme.yaml
│   │   ├── theme.css
│   │   └── fonts/               # Empty - uses system fonts
│   ├── seventies-vibes/
│   │   ├── theme.yaml
│   │   ├── theme.css
│   │   └── fonts/
│   │       ├── fonts.css
│   │       ├── righteous/
│   │       └── nunito/
│   └── nineties-vibe/
│       ├── theme.yaml
│       ├── theme.css
│       └── fonts/
│           ├── fonts.css
│           ├── archivo-black/
│           ├── nunito/
│           └── hack/
│
├── packages/                    # 📋 For future ZIP uploads
│
└── .cache/                      # 📋 For extracted community themes
    └── .gitignore
```

### Backend Discovery (✅ Implemented)

The backend scans theme directories to discover available themes:

```go
// internal/api/preferences_handler.go
func getAvailableThemes() []string {
    var themes []string

    // Scan builtin themes
    builtinDir := "static/themes/builtin"
    if entries, err := os.ReadDir(builtinDir); err == nil {
        for _, entry := range entries {
            if entry.IsDir() {
                themePath := builtinDir + "/" + entry.Name()
                if _, err := os.Stat(themePath + "/theme.css"); err == nil {
                    themes = append(themes, entry.Name())
                }
            }
        }
    }

    // Scan community themes (when ZIP support is added)
    cacheDir := "static/themes/.cache"
    // ... same pattern ...

    return themes
}
```

### Frontend ThemeManager (✅ Implemented)

```javascript
// static/js/theme-manager.js
const BUILTIN_THEMES = ['synthwave', 'goatflow-classic', 'seventies-vibes', 'nineties-vibe'];

function getThemeBasePath(themeId) {
    if (BUILTIN_THEMES.includes(themeId)) {
        return '/static/themes/builtin/' + themeId;
    }
    // Community themes extracted to .cache/
    return '/static/themes/.cache/' + themeId;
}

// Theme CSS loaded from: getThemeBasePath(theme) + '/theme.css'
// Font CSS loaded from:  getThemeBasePath(theme) + '/fonts/fonts.css'
```

### theme.yaml Structure (✅ Implemented)

Each theme includes a `theme.yaml` for metadata:

```yaml
name: Synthwave
id: synthwave
description: Neon retro vibes with glowing effects
version: 1.0.0
author: GoatFlow Team
license: MIT

preview:
  gradient: "linear-gradient(135deg, #00E5FF, #FF2FD4)"

modes:
  dark: true
  light: true
  default: dark

assets:
  fonts:
    enabled: true
    css: fonts/fonts.css
    license: SIL-OFL-1.1

features:
  glowEffects: true
  gridBackground: true
  animations: true
  bevels3d: false
  terminalMode: false

compatibility:
  minVersion: "1.0.0"
```

> **Note:** Currently, the backend only checks for `theme.css` existence. Full YAML parsing for metadata display is planned.

### Theme CSS Requirements (✅ Implemented)

```css
:root,
:root.dark,
.dark {
  --gk-theme-name: 'my-theme';
  --gk-theme-mode: 'dark';

  /* Primary palette */
  --gk-primary: #4ecdc4;
  --gk-primary-hover: #3dbdb5;
  --gk-primary-active: #2eaea6;
  --gk-primary-subtle: rgba(78, 205, 196, 0.12);

  /* Secondary palette */
  --gk-secondary: #ff6b6b;
  --gk-secondary-hover: #ff5252;
  --gk-secondary-subtle: rgba(255, 107, 107, 0.15);

  /* Backgrounds */
  --gk-bg-base: #1a1a2e;
  --gk-bg-surface: #25253d;
  --gk-bg-elevated: #30304d;
  --gk-bg-overlay: rgba(26, 26, 46, 0.9);

  /* Text */
  --gk-text-primary: #ffffff;
  --gk-text-secondary: rgba(255, 255, 255, 0.75);
  --gk-text-muted: rgba(255, 255, 255, 0.5);
  --gk-text-inverse: #1a1a2e;

  /* Borders */
  --gk-border-default: rgba(255, 255, 255, 0.1);
  --gk-border-strong: rgba(255, 255, 255, 0.2);

  /* Status colors */
  --gk-success: #00d68f;
  --gk-success-subtle: rgba(0, 214, 143, 0.15);
  --gk-warning: #ffaa00;
  --gk-warning-subtle: rgba(255, 170, 0, 0.15);
  --gk-error: #ff3d71;
  --gk-error-subtle: rgba(255, 61, 113, 0.15);
  --gk-info: #0095ff;
  --gk-info-subtle: rgba(0, 149, 255, 0.15);

  /* Effects */
  --gk-glow-primary: none;
  --gk-glow-secondary: none;

  /* Typography */
  --gk-font-heading: system-ui, sans-serif;
  --gk-font-body: system-ui, sans-serif;
  --gk-font-mono: ui-monospace, monospace;
}

/* Light mode overrides */
:root.light,
.light {
  --gk-theme-mode: 'light';
  /* ... light mode values ... */
}
```

---

## Planned Features

### ZIP Package Support (📋 Planned)

Future storage architecture for community themes:

```
static/themes/
├── packages/                    # Source of truth - immutable zips
│   ├── awesome-theme.zip
│   └── retro-theme.zip
│
└── .cache/                      # Auto-extracted on startup
    ├── awesome-theme/
    │   ├── theme.yaml
    │   ├── theme.css
    │   └── fonts/
    └── retro-theme/
```

**Planned extraction logic:**
```go
// On startup or theme upload
func extractThemeIfNeeded(zipPath string) error {
    themeName := strings.TrimSuffix(filepath.Base(zipPath), ".zip")
    cacheDir := filepath.Join("static/themes/.cache", themeName)

    if isCacheValid(zipPath, cacheDir) {
        return nil
    }

    // Extract to temp, then atomic swap
    tempDir := cacheDir + ".tmp"
    if err := extractZip(zipPath, tempDir); err != nil {
        return err
    }

    os.RemoveAll(cacheDir)
    return os.Rename(tempDir, cacheDir)
}
```

### Admin Theme Management UI (📋 Planned)

1. Navigate to **Admin > System > Themes**
2. Click **"Install Theme"**
3. Upload the zip file or provide URL
4. Theme is validated and extracted
5. Appears in theme selector immediately

**Planned API endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/admin/themes/install` | Upload and install theme zip |
| DELETE | `/api/admin/themes/:id` | Uninstall community theme |
| POST | `/api/admin/themes/:id/enable` | Enable a theme |
| POST | `/api/admin/themes/:id/disable` | Disable a theme |

### Sound Events (📋 Planned)

```yaml
# sounds/sounds.yaml
enabled: true

events:
  ticket.new: notification.wav
  ticket.assigned: assigned.wav
  ticket.resolved: success.wav
  sla.warning: warning.wav
  sla.breach: alert.wav
  message.received: message.wav

settings:
  volume: 0.7
  respectMute: true
```

> **Note:** Sounds must be user-provided due to licensing restrictions.

### Security Considerations (📋 For ZIP Support)

When ZIP support is implemented:

1. **File Type Validation**
   - Only allow: `.css`, `.yaml`, `.woff2`, `.ttf`, `.svg`, `.png`, `.jpg`, `.wav`, `.mp3`
   - Reject: `.js`, `.html`, `.php`, executables

2. **Path Traversal Prevention**
   - Validate all paths stay within theme directory
   - Reject paths containing `..` or absolute paths

3. **Size Limits**
   - Maximum zip size: 50MB
   - Maximum extracted size: 100MB
   - Maximum single file: 10MB

4. **No JavaScript**
   - Themes cannot include executable JavaScript
   - CSS-only customization

5. **Content Security**
   - Scan for embedded scripts in SVG files
   - Validate CSS doesn't contain `javascript:` URLs

---

## Creating a Built-in Theme

To add a new built-in theme:

### Step 1: Create Directory Structure

```bash
mkdir -p static/themes/builtin/my-theme/fonts
```

### Step 2: Create theme.yaml

```yaml
name: My Theme
id: my-theme
description: A cool custom theme
version: 1.0.0
author: Your Name
license: MIT

preview:
  gradient: "linear-gradient(135deg, #COLOR1, #COLOR2)"

modes:
  dark: true
  light: true
  default: dark

assets:
  fonts:
    enabled: true
    css: fonts/fonts.css

features:
  glowEffects: false
  gridBackground: false
  animations: true
```

### Step 3: Create theme.css

See [Theme CSS Requirements](#theme-css-requirements-implemented) above.

### Step 4: Add Fonts (Optional)

Create `fonts/fonts.css` with **relative paths**:

```css
@font-face {
  font-family: 'My Font';
  font-style: normal;
  font-weight: 400;
  font-display: swap;
  src: url('my-font/regular.woff2') format('woff2');
}
```

### Step 5: Register in ThemeManager

Edit `static/js/theme-manager.js`:

```javascript
const AVAILABLE_THEMES = ['synthwave', 'goatflow-classic', 'seventies-vibes', 'nineties-vibe', 'my-theme'];
const BUILTIN_THEMES = ['synthwave', 'goatflow-classic', 'seventies-vibes', 'nineties-vibe', 'my-theme'];

const THEME_METADATA = {
  // ... existing themes ...
  'my-theme': {
    name: 'My Theme',
    nameKey: 'theme.my_theme',
    description: 'A cool custom theme',
    descriptionKey: 'theme.my_theme_desc',
    gradient: 'linear-gradient(135deg, #COLOR1, #COLOR2)',
    hasFonts: true
  }
};
```

### Step 6: Add i18n Translations

Add to all 15 language files in `internal/i18n/translations/*.json`:

```json
"theme": {
    "my_theme": "My Theme",
    "my_theme_desc": "A cool custom theme"
}
```

---

## Future Enhancements

- **Theme Marketplace** - Online repository at themes.goatflow.io
- **Theme Editor** - In-browser theme customization tool
- **Theme Variants** - Child themes that extend base themes
- **Live Preview** - Preview theme before applying
- **Theme Export** - Export customized settings as new theme
- **Version Updates** - Check for theme updates from repository
- **Dynamic Frontend Loading** - Fetch theme list from API instead of hardcoded

---

*Last updated: January 2026*
