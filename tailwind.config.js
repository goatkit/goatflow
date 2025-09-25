/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    "./templates/**/*.{html,js,pongo2}",
    "./static/js/**/*.js",
  ],
  theme: {
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
      // Better contrast for icons in dark mode
      colors: {
        'gotrs': {
          50: '#eff6ff',
          100: '#dbeafe',
          200: '#bfdbfe',
          300: '#93c5fd',
          400: '#60a5fa',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8',
          800: '#1e40af',
          900: '#1e3a8a',
        },
        'icon': {
          'primary': '#4f46e5',
          'primary-dark': '#818cf8',
          'secondary': '#6b7280',
          'secondary-dark': '#9ca3af',
        }
      }
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
  ],
}