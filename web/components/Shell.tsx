"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { ActionIcon, Popover, Tooltip } from "@mantine/core";
import { IconApi, IconBrandGithub, IconInfoCircle, IconMaximize } from "@tabler/icons-react";
import type { DeviceInfo } from "@/interfaces/api";
import { useApi, useMeta } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Account } from "./Account";
import { SchemeToggle } from "./SchemeToggle";

// Shell is the page frame: a slim header, the page, and a one-line footer of
// icons. The site is one page, so the header only links what isn't on it: the
// frame settings and, in dev mode, the emulator. Both hide on phones (Frame is
// in the account menu too; the emulator previews need a wide screen).
export function Shell({ children, wide }: { children: React.ReactNode; wide?: boolean }) {
    const path = usePathname();
    const meta = useMeta();
    const { me } = useAuth();
    const frames = useApi<DeviceInfo[]>(me ? "/me/devices" : null);
    const links = [
        ...(me && (frames.data?.length || path.startsWith("/settings"))
            ? [{ href: "/settings", label: "Frame", active: path.startsWith("/settings") }]
            : []),
        ...(meta?.dev || path.startsWith("/emulator")
            ? [{ href: "/emulator", label: "Emulator", active: path.startsWith("/emulator") }]
            : []),
        ...(meta?.dev || path.startsWith("/prudle-lab")
            ? [{ href: "/prudle-lab", label: "Prudle", active: path.startsWith("/prudle-lab") }]
            : []),
    ];
    const width = wide ? "max-w-6xl" : "max-w-3xl";
    return (
        <div className="flex min-h-dvh flex-col">
            <header className="border-b border-line bg-panel">
                <nav className={`mx-auto flex items-center gap-0.5 px-3 py-2 sm:gap-1 sm:px-4 ${width}`}>
                    <Link href="/" className="mr-auto py-2 text-lg font-bold tracking-tight">
                        prulights
                    </Link>
                    {links.map((l) => (
                        <Link
                            key={l.href}
                            href={l.href}
                            aria-current={l.active ? "page" : undefined}
                            className={`hidden rounded-md px-2.5 py-2 text-base sm:block sm:px-3 ${
                                l.active ? "bg-raised font-bold" : "text-muted hover:text-fg"
                            }`}
                        >
                            {l.label}
                        </Link>
                    ))}
                    <SchemeToggle />
                    <Account />
                </nav>
            </header>
            <main className={`mx-auto w-full flex-1 px-4 py-6 ${width}`}>{children}</main>
            <Footer width={width} />
        </div>
    );
}

// Footer is one right-aligned row of icons. The credit to the source sits
// behind the info icon: hover or focus on a computer, tap on a phone.
function Footer({ width }: { width: string }) {
    const router = useRouter();
    // the credit opens on hover or tap (a tap also sends mouseenter, so a click
    // only ever opens) and closes on leaving, a tap elsewhere or esc. Leaving
    // waits a moment so the pointer can cross the gap to click the link.
    const [credit, setCredit] = useState(false);
    const leaving = useRef<ReturnType<typeof setTimeout>>(undefined);
    const show = () => {
        clearTimeout(leaving.current);
        setCredit(true);
    };
    const hide = () => {
        clearTimeout(leaving.current);
        leaving.current = setTimeout(() => setCredit(false), 150);
    };
    useEffect(() => () => clearTimeout(leaving.current), []);
    const fullScreen = async () => {
        try {
            await document.documentElement.requestFullscreen?.();
        } catch {}
        router.push("/display");
    };
    const icon = { variant: "subtle", color: "gray", size: "xl", radius: "md" } as const;
    return (
        <footer className="border-t border-line bg-panel">
            <div className={`mx-auto flex items-center justify-end gap-1 px-3 py-2 sm:px-4 ${width}`}>
                <Tooltip label="Full screen">
                    <ActionIcon {...icon} aria-label="Full screen" onClick={fullScreen}>
                        <IconMaximize size={22} />
                    </ActionIcon>
                </Tooltip>
                <Tooltip label="API docs">
                    <ActionIcon {...icon} component="a" href="/docs" aria-label="API docs">
                        <IconApi size={22} />
                    </ActionIcon>
                </Tooltip>
                <Tooltip label="Source">
                    <ActionIcon
                        {...icon}
                        component="a"
                        href="https://github.com/evanofslack/prulights"
                        target="_blank"
                        rel="noopener"
                        aria-label="Source"
                    >
                        <IconBrandGithub size={22} />
                    </ActionIcon>
                </Tooltip>
                <Popover opened={credit} onChange={setCredit} position="top-end" width={280} shadow="md">
                    <Popover.Target>
                        <ActionIcon
                            {...icon}
                            aria-label="About the schedule"
                            aria-expanded={credit}
                            onClick={show}
                            onMouseEnter={show}
                            onMouseLeave={hide}
                        >
                            <IconInfoCircle size={22} />
                        </ActionIcon>
                    </Popover.Target>
                    <Popover.Dropdown onMouseEnter={show} onMouseLeave={hide} className="text-sm text-muted">
                        Schedule from the{" "}
                        <a
                            className="underline hover:text-fg"
                            href="https://www.prudentialcenter.com/experiences/events/31-nights-of-light/"
                            target="_blank"
                            rel="noopener"
                        >
                            Prudential Center
                        </a>
                        . Not affiliated with the Prudential Center, BXP or Prudential Financial.
                    </Popover.Dropdown>
                </Popover>
            </div>
        </footer>
    );
}

export function ErrorNote({ error, retry }: { error: Error; retry?: () => void }) {
    return (
        <div role="alert" className="rounded-xl bg-panel p-4 text-muted">
            Couldn't load the schedule ({error.message}).{" "}
            {retry && (
                <button type="button" className="underline" onClick={retry}>
                    Try again
                </button>
            )}
        </div>
    );
}
