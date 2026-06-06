import type { Config } from "tailwindcss"

const config: Config = {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    container: {
      center: true,
      padding: "2rem",
      screens: { "2xl": "1400px" },
    },
    extend: {
      fontFamily: {
        sans: ['"Golos Text"', "system-ui", "sans-serif"],
        mono: ['"JetBrains Mono"', "monospace"],
      },
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        // Brand accents (kit.css). `azure` mirrors primary; `cyan` is the live signal.
        azure: { DEFAULT: "hsl(var(--azure))", 600: "hsl(var(--azure-600))" },
        cyan: "hsl(var(--cyan))",
        magenta: "hsl(var(--magenta))",
        vk: "hsl(var(--vk))",
        success: "hsl(var(--success))",
        warning: "hsl(var(--warning))",
      },
      backgroundImage: {
        // Authentic platform brand gradient (Instagram glyph). Kept on purpose.
        "ig-gradient":
          "linear-gradient(125deg,#FEDA77 0%,#F58529 18%,#DD2A7B 55%,#8134AF 85%,#515BD4 100%)",
      },
      boxShadow: {
        // Soft, hue-tinted elevation — replaces the old neon `glow`.
        soft: "0 1px 2px hsl(222 47% 11% / 0.06), 0 8px 24px -12px hsl(223 60% 30% / 0.18)",
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
}

export default config
