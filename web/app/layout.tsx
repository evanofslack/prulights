import type { Metadata, Viewport } from "next";
import { Atkinson_Hyperlegible } from "next/font/google";
import { ColorSchemeScript, mantineHtmlProps } from "@mantine/core";
import { Providers } from "./providers";
import { BAR } from "@/lib/theme";

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
    appleWebApp: { capable: true, title: "Pru tonight", statusBarStyle: "default" },
};

export const viewport: Viewport = {
    themeColor: [
        { media: "(prefers-color-scheme: dark)", color: BAR.dark },
        { media: "(prefers-color-scheme: light)", color: BAR.light },
    ],
    width: "device-width",
    initialScale: 1,
    viewportFit: "cover",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
    return (
        <html lang="en" {...mantineHtmlProps} className={body.variable}>
            <head>
                <ColorSchemeScript defaultColorScheme="auto" />
            </head>
            <body>
                <Providers>{children}</Providers>
            </body>
        </html>
    );
}
