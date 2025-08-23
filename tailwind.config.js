/** @type {import('tailwindcss').Config} */
module.exports = {
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