"use client";

import { MantineProvider } from "@mantine/core";
import { resolver, theme } from "@/lib/theme";
import { PaletteProvider } from "@/lib/palette";
import { AuthProvider } from "@/lib/auth";

export function Providers({ children }: { children: React.ReactNode }) {
    return (
        <MantineProvider theme={theme} cssVariablesResolver={resolver} defaultColorScheme="auto">
            <AuthProvider>
                <PaletteProvider>{children}</PaletteProvider>
            </AuthProvider>
        </MantineProvider>
    );
}
