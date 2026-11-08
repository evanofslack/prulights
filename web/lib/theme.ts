import { createTheme, type MantineColorsTuple } from "@mantine/core";

// The palette lives here; the role tokens in styles/index.css point at these colors.
// The app is a night sky by default: deep navy-black page, soft white text, and a
// warm gold accent like the lit crown.

// dark slots Mantine uses: 0 text, 2 dimmed, 4 borders, 5 hover, 6 default
// buttons and inputs, 7 the page, 8 panels, 9 deepest
const dark: MantineColorsTuple = [
    "#eef1f8",
    "#c9cfdd",
    "#98a1b8",
    "#6b7590",
    "#3d465e",
    "#323a50",
    "#262d40",
    "#121726",
    "#0c1020",
    "#06080f",
];

const gold: MantineColorsTuple = [
    "#fff9e6",
    "#fdefc4",
    "#fbe39a",
    "#f8d56c",
    "#f5c946",
    "#f2bf2c",
    "#d9a71d",
    "#b48913",
    "#8e6b0c",
    "#6a4f06",
];

export const theme = createTheme({
    primaryColor: "gold",
    primaryShade: { light: 7, dark: 4 },
    colors: { dark, gold },
    fontFamily: "var(--font-body), system-ui, sans-serif",
    headings: { fontFamily: "var(--font-body), system-ui, sans-serif", fontWeight: "700" },
    defaultRadius: "md",
    autoContrast: true,
});
