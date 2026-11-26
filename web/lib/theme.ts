import { createTheme, type CSSVariablesResolver, type MantineColorsTuple } from "@mantine/core";

// The palette lives here; the role tokens in styles/index.css point at these colors.
// Two looks: the night sky (deep ink-indigo page, soft white text, a candle-gold
// accent like the lit crown) and paper (warm grey-white, ink text, brass), the
// way the e-ink frame looks by day. Both are kept a little muted.

// dark slots Mantine uses: 0 text, 2 dimmed, 4 borders, 5 hover, 6 default
// buttons and inputs, 7 the page, 8 panels, 9 deepest
const dark: MantineColorsTuple = [
    "#eceef4",
    "#c8ccd8",
    "#9a9fb0",
    "#6d7286",
    "#3f4456",
    "#33384a",
    "#282c3b",
    "#141824",
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

// resolver puts Mantine's light page and default controls on the paper tokens
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
    dark: {},
});

// the browser's bar color for each look: the page's surface
export const BAR = { dark: "#0e111b", light: "#e9e6de" } as const;
