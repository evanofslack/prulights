import { createTheme, type CSSVariablesResolver, type MantineColorsTuple } from "@mantine/core";

// The palette lives here; the role tokens in styles/index.css point at these colors.
// Two looks: the night sky (deep ink-indigo page, soft white text, a candle-gold
// accent like the lit crown) and paper (warm grey-white, ink text, brass), the
// way the e-ink frame looks by day. Both are kept a little muted.
//
// Buttons: one main action per screen, a filled size="lg" Button, full width on
// a phone (w-full sm:w-auto). Everything else is variant="default" or a quiet
// text button at least 44px tall (min-h-11).

// dark slots, as the tokens use them: 0 text, 2 muted, 5 lines, 6 raised
// (inputs, chips), 7 panels (cards, header, footer), 8 the page, 9 deepest.
// 8 → 7 → 6 are even steps, so a panel reads on the page the way paper's does.
const dark: MantineColorsTuple = [
    "#eceef4",
    "#c8ccd8",
    "#9a9fb0",
    "#6d7286",
    "#3f4456",
    "#33384a",
    "#262b3b",
    "#1a1e2c",
    "#0e111b",
    "#07090f",
];

// greys for paper: Mantine's light scheme draws borders, hovers and skeletons
// from these
const gray: MantineColorsTuple = [
    "#f5f3ee",
    "#ebe8e1",
    "#dfdbd1",
    "#cdc8bc",
    "#b5b0a3",
    "#96918a",
    "#6f6c66",
    "#55524d",
    "#3a3834",
    "#24231f",
];

const gold: MantineColorsTuple = [
    "#fbf6e9",
    "#f3e6c4",
    "#ead49b",
    "#e3c784",
    "#dcb96c",
    "#d2ab55",
    "#b8923f",
    "#9a7830",
    "#7c6024",
    "#5f4818",
];

export const theme = createTheme({
    primaryColor: "gold",
    primaryShade: { light: 8, dark: 4 },
    colors: { dark, gray, gold },
    white: "#f5f3ee",
    black: "#1b1d22",
    fontFamily: "var(--font-body), system-ui, sans-serif",
    headings: { fontFamily: "var(--font-body), system-ui, sans-serif", fontWeight: "700" },
    defaultRadius: "md",
    autoContrast: true,
});

// resolver puts Mantine's page and default controls (modals, menus, popovers,
// default buttons) on the same tokens as the rest of the site, in both looks
export const resolver: CSSVariablesResolver = () => ({
    variables: {},
    light: {
        "--mantine-color-body": "var(--color-surface)",
        "--mantine-color-text": "var(--color-fg)",
        "--mantine-color-dimmed": "var(--color-muted)",
        "--mantine-color-default": "var(--color-panel)",
        "--mantine-color-default-hover": "var(--color-raised)",
        "--mantine-color-default-border": "var(--color-line)",
        "--mantine-color-default-color": "var(--color-fg)",
    },
    dark: {
        "--mantine-color-body": "var(--color-surface)",
        "--mantine-color-text": "var(--color-fg)",
        "--mantine-color-dimmed": "var(--color-muted)",
        "--mantine-color-default": "var(--color-raised)",
        "--mantine-color-default-hover": "var(--color-line)",
        "--mantine-color-default-border": "var(--color-line)",
        "--mantine-color-default-color": "var(--color-fg)",
    },
});

// the browser's bar color for each look: the page's surface
export const BAR = { dark: "#0e111b", light: "#e9e6de" } as const;
