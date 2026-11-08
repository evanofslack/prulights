"use client";

import { MantineProvider } from "@mantine/core";
import { theme } from "@/lib/theme";

export function Providers({ children }: { children: React.ReactNode }) {
    return (
        <MantineProvider theme={theme} defaultColorScheme="dark">
            {children}
        </MantineProvider>
    );
}
