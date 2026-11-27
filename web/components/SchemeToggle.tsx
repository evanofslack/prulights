"use client";

import { useEffect } from "react";
import { ActionIcon, Tooltip, useComputedColorScheme, useMantineColorScheme } from "@mantine/core";
import { useMounted } from "@mantine/hooks";
import { IconMoon, IconSun } from "@tabler/icons-react";
import { BAR } from "@/lib/theme";

// SchemeToggle flips between the night sky and paper. Until it's used the site
// follows the system; after, Mantine keeps the choice in localStorage.
// The exported html can't know that choice, so the icon follows the scheme
// Mantine marks on <html> (css, right from the first paint) and the label waits
// for mount: anything else in the first render would fail hydration.
export function SchemeToggle() {
    const { setColorScheme } = useMantineColorScheme();
    const scheme = useComputedColorScheme("dark", { getInitialValueInEffect: true });
    const mounted = useMounted();
    // the browser's bar follows the page, not the system, once someone chooses
    useEffect(() => {
        for (const m of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]'))
            m.content = BAR[scheme];
    }, [scheme]);
    const next = scheme === "dark" ? "light" : "dark";
    const label = !mounted ? "Light or dark" : next === "light" ? "Light mode" : "Dark mode";
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
                <IconSun size={22} className="hidden dark:block" />
                <IconMoon size={22} className="dark:hidden" />
            </ActionIcon>
        </Tooltip>
    );
}
