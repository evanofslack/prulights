"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const links = [
    { href: "/", label: "Tonight" },
    { href: "/calendar", label: "Calendar" },
    { href: "/docs", label: "API" },
];

// Shell is the page frame: a slim header with the main places, the page, and
// the credit to the source.
export function Shell({ children, wide }: { children: React.ReactNode; wide?: boolean }) {
    const path = usePathname();
    return (
        <div className="flex min-h-dvh flex-col">
            <header className="border-b border-line">
                <nav
                    className={`mx-auto flex items-center gap-1 px-4 py-2 ${wide ? "max-w-6xl" : "max-w-3xl"}`}
                >
                    <Link href="/" className="mr-auto py-2 text-lg font-bold tracking-tight">
                        prulights
                    </Link>
                    {links.map((l) => {
                        const active =
                            l.href === "/" ? path === "/" || path === "/night" : path.startsWith(l.href);
                        return (
                            <Link
                                key={l.href}
                                href={l.href}
                                aria-current={active ? "page" : undefined}
                                className={`rounded-md px-3 py-2 text-base ${active ? "bg-raised font-bold" : "text-muted hover:text-fg"}`}
                            >
                                {l.label}
                            </Link>
                        );
                    })}
                </nav>
            </header>
            <main className={`mx-auto w-full flex-1 px-4 py-6 ${wide ? "max-w-6xl" : "max-w-3xl"}`}>
                {children}
            </main>
            <footer className="border-t border-line px-4 py-5 text-center text-sm text-subtle">
                Schedule from the{" "}
                <a
                    className="underline hover:text-fg"
                    href="https://www.prudentialcenter.com/experiences/events/31-nights-of-light/"
                    target="_blank"
                    rel="noopener"
                >
                    Prudential Center
                </a>
                . Not affiliated with the Prudential Center, BXP or Prudential Financial. ·{" "}
                <a
                    className="underline hover:text-fg"
                    href="https://github.com/evanofslack/prulights"
                    target="_blank"
                    rel="noopener"
                >
                    Source
                </a>
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
