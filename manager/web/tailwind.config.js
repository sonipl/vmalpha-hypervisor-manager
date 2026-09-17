/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        nova: {
          50: '#E6F5F3',
          100: '#CCEBe7',
          200: '#99D7CF',
          300: '#66C3B7',
          400: '#33AF9F',
          500: '#0C7E72',
          600: '#0A655B',
          700: '#074C44',
          800: '#05332E',
          900: '#021917',
        },
        copper: {
          50: '#FBF0E5',
          100: '#F7E1CB',
          200: '#EFC397',
          300: '#E7A563',
          400: '#DF872F',
          500: '#C46B28',
          600: '#9D5620',
          700: '#764018',
          800: '#4F2B10',
          900: '#271508',
        },
        surface: {
          light: '#FFFFFF',
          DEFAULT: '#F6F3EF',
          dark: '#161B22',
          'dark-alt': '#1C2129',
        },
      },
      fontFamily: {
        display: ['"DM Serif Display"', 'Georgia', 'serif'],
        body: ['"Source Sans 3"', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'Consolas', 'monospace'],
      },
    },
  },
  plugins: [],
}
