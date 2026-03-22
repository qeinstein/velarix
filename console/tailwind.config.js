/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        'v-bg': '#050505',
        'v-bg-card': '#0a0a0a',
        'v-bg-glass': 'rgba(15, 15, 15, 0.6)',
        'v-text': '#ededed',
        'v-text-muted': '#a1a1aa',
        'v-accent': '#06b6d4',      // Cyan
        'v-accent-glow': 'rgba(6, 182, 212, 0.2)',
        'v-success': '#10b981',     // Emerald
        'v-error': '#ef4444',       // Red
        'v-border': '#27272a',
      },
      fontFamily: {
        sans: ['Inter', 'sans-serif'],
        display: ['Outfit', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'monospace'],
      },
      animation: {
        'pulse-slow': 'pulse 4s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'pulse-fast': 'pulse 1.5s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'blob': 'blob 10s infinite',
      },
      keyframes: {
        blob: {
          '0%': { transform: 'translate(0px, 0px) scale(1)' },
          '33%': { transform: 'translate(30px, -50px) scale(1.1)' },
          '66%': { transform: 'translate(-20px, 20px) scale(0.9)' },
          '100%': { transform: 'translate(0px, 0px) scale(1)' },
        }
      }
    },
  },
  plugins: [],
}
