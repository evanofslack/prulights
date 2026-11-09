"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useMeta } from "@/lib/api";
import { Account } from "./Account";

// Shell is the page frame: a slim header with the main places, the page, and
// the credit to the source. The emulator shows up when the server is in dev
// mode, since its previews only exist there, and only on wider screens.
export function Shell({ children, wide }: { children: React.ReactNode; wide?: boolean }) {
    const path = usePathname();
    const meta = useMeta();
    const links = [
        { href: "/", label: "Tonight", active: path === "/" || path === "/night" },
        { href: "/calendar", label: "Calendar", active: path.startsWith("/calendar") },
        ...(meta?.dev || path.startsWith("/emulator")
            ? [{ href: "/emulator", label: "Emulator", active: path.startsWith("/emulator"), wide: true }]
            : []),
    ];
    const width = wide ? "max-w-6xl" : "max-w-3xl";
    return (
        <div className="flex min-h-dvh flex-col">
            <header className="border-b border-line">
                <nav className={`mx-auto flex items-center gap-0.5 px-3 py-2 sm:gap-1 sm:px-4 ${width}`}>
                    <Link href="/" className="mr-auto py-2 text-lg font-bold tracking-tight">
                        prulights
                    </Link>
                    {links.map((l) => (
                        <Link
                            key={l.href}
                            href={l.href}
                            aria-current={l.active ? "page" : undefined}
                            className={`rounded-md px-2.5 py-2 text-base sm:px-3 ${l.active ? "bg-raised font-bold" : "text-muted hover:text-fg"} ${
                                "wide" in l ? "hidden sm:block" : ""
                            }`}
                        >
                            {l.label}
                        </Link>
                    ))}
                    <Account />
                </nav>
            </header>
            <main className={`mx-auto w-full flex-1 px-4 py-6 ${width}`}>{children}</main>
            <footer className="flex flex-col gap-2 border-t border-line px-4 py-5 text-center text-sm text-subtle">
                <p className="flex justify-center gap-4">
                    <Link className="underline hover:text-fg" href="/display">
                        Display
                    </Link>
                    <a className="underline hover:text-fg" href="/docs">
                        API
                    </a>
                    <a
                        className="underline hover:text-fg"
                        href="https://github.com/evanofslack/prulights"
                        target="_blank"
                        rel="noopener"
                    >
                        Source
                    </a>
                </p>
                <p>
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
                </p>
            </footer>
        </div>
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
