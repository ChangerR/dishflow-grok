/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        brand: { DEFAULT: "#ea580c", dark: "#c2410c", soft: "#fff7ed" },
        canvas: "#f3eee7",
        ink: "#1c1917",
      },
      fontFamily: {
        sans: [
          '"PingFang SC"',
          '"Hiragino Sans GB"',
          '"Microsoft YaHei UI"',
          '"Microsoft YaHei"',
          '"Noto Sans SC"',
          "ui-sans-serif",
          "system-ui",
          "sans-serif",
        ],
      },
      boxShadow: {
        card: "0 1px 2px rgba(28,25,23,.05), 0 10px 28px rgba(28,25,23,.05)",
        lift: "0 8px 30px rgba(234,88,12,.18)",
      },
    },
  },
  plugins: [],
};
