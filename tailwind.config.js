/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    "./templates/**/*.{html,js,pongo2}",
    "./static/js/**/*.js",
  ],
  theme: {
    // Fluid container - expands to fill available space
    container: {
      center: true,
      padding: {
        DEFAULT: '1rem',
        sm: '1.5rem',
        lg: '2rem',
        xl: '3rem',
        '2xl': '4rem',
      },
      // No max-width caps - let content fill available space
      // Padding provides the margins on large screens
    },
    extend: {
      // Enhanced icon sizing scale
      spacing: {
        '18': '4.5rem',
        '22': '5.5rem',
      },
      // Icon-specific sizing
      iconSize: {
        'xs': '12px',
        'sm': '16px',
        'md': '20px',
        'lg': '24px',
        'xl': '32px',
        '2xl': '40px',
      },
      // GoatKit theme-aware colors using CSS custom properties
      colors: {
        // GoatKit semantic colors - mapped to CSS variables
        'gk': {
          'primary': 'var(--gk-primary)',
          'primary-hover': 'var(--gk-primary-hover)',
          'primary-active': 'var(--gk-primary-active)',
          'primary-subtle': 'var(--gk-primary-subtle)',
          'secondary': 'var(--gk-secondary)',
          'secondary-hover': 'var(--gk-secondary-hover)',
          'secondary-subtle': 'var(--gk-secondary-subtle)',
          'tertiary': 'var(--gk-tertiary)',
        },
        // Semantic background colors
        'surface': {
          'base': 'var(--gk-bg-base)',
          'DEFAULT': 'var(--gk-bg-surface)',
          'elevated': 'var(--gk-bg-elevated)',
          'overlay': 'var(--gk-bg-overlay)',
        },
        // Text colors
        'content': {
          'primary': 'var(--gk-text-primary)',
          'secondary': 'var(--gk-text-secondary)',
          'muted': 'var(--gk-text-muted)',
          'inverse': 'var(--gk-text-inverse)',
        },
        // Status colors
        'status': {
          'success': 'var(--gk-success)',
          'success-subtle': 'var(--gk-success-subtle)',
          'warning': 'var(--gk-warning)',
          'warning-subtle': 'var(--gk-warning-subtle)',
          'error': 'var(--gk-error)',
          'error-subtle': 'var(--gk-error-subtle)',
          'info': 'var(--gk-info)',
          'info-subtle': 'var(--gk-info-subtle)',
        },
        // Legacy goatflow colors - now mapped to theme variables for backwards compatibility
        'goatflow': {
          50: 'var(--gk-primary-subtle)',
          100: 'var(--gk-primary-subtle)',
          200: 'var(--gk-primary-subtle)',
          300: 'var(--gk-primary)',
          400: 'var(--gk-primary)',
          500: 'var(--gk-primary)',
          600: 'var(--gk-primary)',
          700: 'var(--gk-primary-hover)',
          800: 'var(--gk-primary-active)',
          900: 'var(--gk-primary-active)',
        },
        // Icon colors - now use theme variables
        'icon': {
          'primary': 'var(--gk-primary)',
          'primary-dark': 'var(--gk-primary)',
          'secondary': 'var(--gk-text-secondary)',
          'secondary-dark': 'var(--gk-text-muted)',
        }
      },
      // Border colors
      borderColor: {
        'gk': {
          'DEFAULT': 'var(--gk-border-default)',
          'strong': 'var(--gk-border-strong)',
        },
      },
      // Box shadows including glows
      boxShadow: {
        'gk-sm': 'var(--gk-shadow-sm)',
        'gk-md': 'var(--gk-shadow-md)',
        'gk-lg': 'var(--gk-shadow-lg)',
        'gk-xl': 'var(--gk-shadow-xl)',
        'gk-glow-primary': 'var(--gk-glow-primary)',
        'gk-glow-secondary': 'var(--gk-glow-secondary)',
        'gk-glow-success': 'var(--gk-glow-success)',
        'gk-glow-warning': 'var(--gk-glow-warning)',
        'gk-glow-error': 'var(--gk-glow-error)',
        'gk-focus': 'var(--gk-focus-ring)',
      },
      // Background images for gradients
      backgroundImage: {
        'gk-gradient-primary': 'var(--gk-gradient-primary)',
        'gk-gradient-sunset': 'var(--gk-gradient-sunset)',
        'gk-gradient-hero': 'var(--gk-gradient-hero)',
      },
      // Font families
      fontFamily: {
        'heading': 'var(--gk-font-heading)',
        'body': 'var(--gk-font-body)',
        'mono': 'var(--gk-font-mono)',
      },
      // Ring (focus) utilities
      ringColor: {
        'gk': 'var(--gk-primary)',
      },
      // Border radius
      borderRadius: {
        'gk-sm': 'var(--gk-radius-sm)',
        'gk-md': 'var(--gk-radius-md)',
        'gk-lg': 'var(--gk-radius-lg)',
        'gk-xl': 'var(--gk-radius-xl)',
      },
      // Transitions
      transitionDuration: {
        'gk-fast': '150ms',
        'gk-normal': '200ms',
        'gk-slow': '300ms',
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
  ],
}
