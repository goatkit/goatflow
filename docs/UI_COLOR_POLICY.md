# UI Color Policy

## Button and Action Colors

This document defines the standard color scheme for all interactive elements across the GoatFlow application. These colors must be consistently applied throughout the entire project.

## Color Conventions

### Action Types and Their Colors

| Action Type | Color | Tailwind Classes | Usage |
|------------|-------|------------------|-------|
| **Edit/Modify** | Blue | `text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300` | Editing existing records, modifying data |
| **Enable/Activate** | Green | `text-green-600 hover:text-green-900 dark:text-green-400 dark:hover:text-green-300` | Positive actions: enabling features, activating users, confirming success |
| **Disable/Deactivate** | Orange/Yellow | `text-yellow-600 hover:text-yellow-900 dark:text-yellow-400 dark:hover:text-yellow-300` | Warning actions: disabling features, deactivating users, cautionary operations |
| **Delete/Remove** | Red | `text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300` | Destructive actions: deleting records, removing items, canceling operations |

## Implementation Guidelines

### 1. Button Styling
- Use text-only coloring (no background) for action buttons
- Maintain consistent icon sizes: `w-5 h-5 inline` for inline icons
- Include hover states for better interactivity
- Always provide dark mode variants

### 2. Example Implementation
```html
<!-- Edit Button -->
<button class="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300">
    <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <!-- icon path -->
    </svg>
</button>

<!-- Enable Button (for inactive items) -->
<button class="text-green-600 hover:text-green-900 dark:text-green-400 dark:hover:text-green-300">
    <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <!-- icon path -->
    </svg>
</button>

<!-- Disable Button (for active items) -->
<button class="text-yellow-600 hover:text-yellow-900 dark:text-yellow-400 dark:hover:text-yellow-300">
    <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <!-- icon path -->
    </svg>
</button>

<!-- Delete Button -->
<button class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300">
    <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <!-- icon path -->
    </svg>
</button>
```

### 3. Contextual Color Usage

#### Status Indicators
- **Active/Enabled**: Green text or badge
- **Inactive/Disabled**: Gray or muted text
- **Warning/Pending**: Orange/Yellow
- **Error/Failed**: Red

#### Form Elements
- **Success messages**: Green (`text-green-600`)
- **Warning messages**: Orange/Yellow (`text-yellow-600`)
- **Error messages**: Red (`text-red-600`)
- **Info messages**: Blue (`text-blue-600`)

### 4. Accessibility Requirements
- Always include `title` attributes on icon-only buttons
- Ensure sufficient color contrast ratios
- Don't rely on color alone to convey meaning
- Provide text labels where possible

## Toast/Alert Colors

For toast notifications and alerts, use background colors with appropriate text:

- **Success**: `bg-green-100 text-green-800 dark:bg-green-800 dark:text-green-100`
- **Warning**: `bg-yellow-100 text-yellow-800 dark:bg-yellow-800 dark:text-yellow-100`
- **Error**: `bg-red-100 text-red-800 dark:bg-red-800 dark:text-red-100`
- **Info**: `bg-blue-100 text-blue-800 dark:bg-blue-800 dark:text-blue-100`

## Migration Notes

When updating existing code to follow this policy:
1. Replace all round colored button backgrounds with text-only styling
2. Ensure enable/disable buttons use the correct semantic colors
3. Update any custom color implementations to use these standards
4. Test in both light and dark modes

## Enforcement

This color policy should be enforced through:
- Code reviews
- Component libraries/templates
- Developer documentation
- Automated linting where possible

---

*Last Updated: August 23, 2025*
*Policy Version: 1.0*