/** @type {import('tailwindcss').Config} **/
module.exports = {
  content: ["./components/**/*.templ"],
  theme: {
      extend: {},
      fontFamily: {
          'sans': ['Lato', 'sans-serif'],
          'mono': ['Consolas', 'monospace'],
      },
  },
  plugins: [
    require('@tailwindcss/forms'),
  ],
  darkMode: 'selector',
}
