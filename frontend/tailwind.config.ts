import type { Config } from "tailwindcss";
import daisyui from "daisyui";

export default {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {},
  },
  plugins: [daisyui],
  daisyui: {
    themes: [
      {
        "openscanner-dark": {
          primary: "#00e676",
          "primary-content": "#000000",
          secondary: "#ff9100",
          accent: "#29b6f6",
          neutral: "#1e1e1e",
          "neutral-content": "#e0e0e0",
          "base-100": "#121212",
          "base-200": "#1e1e1e",
          "base-300": "#2d2d2d",
          info: "#29b6f6",
          success: "#00e676",
          warning: "#ffea00",
          error: "#ff1744",
        },
      },
      {
        "openscanner-light": {
          primary: "#2e7d32",
          "primary-content": "#ffffff",
          secondary: "#e65100",
          accent: "#0277bd",
          neutral: "#f5f5f5",
          "neutral-content": "#1e1e1e",
          "base-100": "#ffffff",
          "base-200": "#f5f5f5",
          "base-300": "#e0e0e0",
          info: "#0277bd",
          success: "#2e7d32",
          warning: "#f9a825",
          error: "#c62828",
        },
      },
    ],
    darkTheme: "openscanner-dark",
  },
} satisfies Config;
