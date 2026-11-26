"use client";

import { MantineProvider } from "@mantine/core";
import { resolver, theme } from "@/lib/theme";
import { PaletteProvider } from "@/lib/palette";
import { AuthProvider } from "@/lib/auth";
import { PrudleProvider } from "@/lib/blackout";

export function Providers({ children }: { children: React.ReactNode }) {
    return (
        <MantineProvider theme={theme} cssVariablesResolver={resolver} defaultColorScheme="auto">
            <AuthProvider>
                <PrudleProvider>
                    <PaletteProvider>{children}</PaletteProvider>
                </PrudleProvider>
            </AuthProvider>
        </MantineProvider>
    );
}
