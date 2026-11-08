import type { Metadata, Viewport } from "next";
import { Atkinson_Hyperlegible } from "next/font/google";
import { ColorSchemeScript, mantineHtmlProps } from "@mantine/core";
import { Providers } from "./providers";

import "../styles/index.css";

// Atkinson Hyperlegible: made for low-vision readers, easy at a glance
const body = Atkinson_Hyperlegible({
    subsets: ["latin"],
    weight: ["400", "700"],
    variable: "--font-body",
});

export const metadata: Metadata = {
    title: "prulights",
    description: "What color is the Pru tonight? The Prudential Tower's lighting schedule.",
    applicationName: "prulights",
    appleWebApp: { capable: true, title: "Pru tonight", statusBarStyle: "black-translucent" },
};

export const viewport: Viewport = {
    themeColor: "#0c1020",
    width: "device-width",
    initialScale: 1,
    viewportFit: "cover",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
    return (
        <html lang="en" {...mantineHtmlProps} data-mantine-color-scheme="dark" className={body.variable}>
            <head>
                <ColorSchemeScript defaultColorScheme="dark" />
            </head>
            <body>
                <Providers>{children}</Providers>
            </body>
        </html>
    );
}
