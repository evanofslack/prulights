"use client";

import { useEffect } from "react";
import { ActionIcon, Tooltip, useComputedColorScheme, useMantineColorScheme } from "@mantine/core";
import { IconMoon, IconSun } from "@tabler/icons-react";
import { BAR } from "@/lib/theme";

// SchemeToggle flips between the night sky and paper. Until it's used the site
// follows the system; after, Mantine keeps the choice in localStorage.
export function SchemeToggle() {
    const { setColorScheme } = useMantineColorScheme();
    const scheme = useComputedColorScheme("dark", { getInitialValueInEffect: true });
    // the browser's bar follows the page, not the system, once someone chooses
    useEffect(() => {
        for (const m of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]'))
            m.content = BAR[scheme];
    }, [scheme]);
    const next = scheme === "dark" ? "light" : "dark";
    const label = next === "light" ? "Light mode" : "Dark mode";
    return (
        <Tooltip label={label}>
            <ActionIcon
                variant="subtle"
                color="gray"
                size="xl"
                radius="md"
                aria-label={label}
                onClick={() => setColorScheme(next)}
            >
                {scheme === "dark" ? <IconSun size={22} /> : <IconMoon size={22} />}
            </ActionIcon>
        </Tooltip>
    );
}
