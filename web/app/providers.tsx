"use client";

import { MantineProvider } from "@mantine/core";
import { theme } from "@/lib/theme";
import { PaletteProvider } from "@/lib/palette";

export function Providers({ children }: { children: React.ReactNode }) {
    return (
        <MantineProvider theme={theme} defaultColorScheme="dark">
            <PaletteProvider>{children}</PaletteProvider>
        </MantineProvider>
    );
}
