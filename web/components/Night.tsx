"use client";

import Link from "next/link";
import { Anchor, Skeleton } from "@mantine/core";
import { IconChevronRight, IconExternalLink } from "@tabler/icons-react";
import type { Lighting } from "@/interfaces/schedule";
import { bandsOf, usePalette } from "@/lib/palette";
import { colorNames, dayOfMonth, longDate, relativeName, shortDate, weekdayAbbr } from "@/lib/format";
import { useSky } from "@/lib/sky";
import { HIDDEN, HIDDEN_NAME, usePrudle } from "@/lib/blackout";
import { Crown, Tower } from "./Tower";

// the tower scene's box, the drawing's own shape (480×760, see Tower) so nothing is
// cut off: full width on a phone, and on a computer as tall as the window
// allows while leaving the text beside it room (the `scene` class, styles/index.css)
const SCENE = "scene";

interface NightProps {
    lighting: Lighting;
    tonight?: string;
    publishedThrough?: string;
    // under the details, e.g. the "I saw it" controls
    children?: React.ReactNode;
}

// Night is one night, large: the tower lit in its colors, then what and why.
export function Night({ lighting: l, tonight, publishedThrough, children }: NightProps) {
    const palette = usePalette();
    const hidden = usePrudle().hidden(l.date);
    const bands = hidden ? HIDDEN : bandsOf(l.colors, palette);
    const names = hidden ? HIDDEN_NAME : colorNames(l.colors, palette);
    const when = relativeName(l.date, tonight);
    const unannounced = !l.scheduled && publishedThrough !== undefined && l.date > publishedThrough;
    const sky = useSky(l, tonight);
    return (
        <section className="flex flex-col gap-5 sm:flex-row sm:items-stretch sm:gap-10">
            {/* the whole scene, never cropped: its box keeps the drawing's shape. At night
                the tower runs to the phone's edges and fades into the page; on paper it's a
                night picture in a card. On a computer it stays in view while the notes
                beside it scroll */}
            <div className="relative overflow-hidden rounded-2xl shadow-lg sm:sticky sm:top-4 sm:shrink-0 sm:self-start dark:-mx-4 dark:-mt-8 dark:rounded-none dark:shadow-none sm:dark:mx-0 sm:dark:mt-0 sm:dark:rounded-2xl">
                <Tower
                    bands={bands}
                    sky={sky}
                    night={l.date}
                    live={l.date === tonight}
                    className={`block ${SCENE}`}
                    title={
                        hidden
                            ? "The Prudential Tower, its colors hidden for Prudle"
                            : `The Prudential Tower, crown lit ${names}`
                    }
                />
                <div className="pointer-events-none absolute inset-x-0 bottom-0 hidden h-8 bg-linear-to-b from-transparent to-surface dark:block sm:dark:hidden" />
            </div>
            {/* the text keeps a fixed top: centering it would move it each time the
                "I saw it" panel under it grows */}
            <div className="flex w-full min-w-0 flex-col gap-3 text-center sm:pt-16 sm:text-left">
                <p className="text-lg text-muted">
                    <span className="font-bold text-fg">{when}</span>
                    {when === "Tonight" || when === "Tomorrow" || when === "Last night"
                        ? ` · ${longDate(l.date)}`
                        : `, ${shortDate(l.date)}`}
                </p>
                <h1 className="text-4xl leading-tight font-bold sm:text-5xl">{names}</h1>
                {hidden && (
                    <p className="text-lg text-muted">
                        {l.date === tonight ? (
                            <Link href="/#prudle" className="font-bold text-fg underline">
                                Play tonight&apos;s Prudle to see them
                            </Link>
                        ) : (
                            "December's colors show on their night, after its Prudle."
                        )}
                    </p>
                )}
                {l.scheduled ? (
                    <p className="text-2xl leading-snug">
                        for{" "}
                        {l.cause_url ? (
                            <Anchor
                                href={l.cause_url}
                                target="_blank"
                                rel="noopener"
                                c="var(--color-fg)"
                                underline="always"
                                inherit
                            >
                                {l.cause}
                                <IconExternalLink
                                    size={18}
                                    className="ml-1 inline align-baseline text-muted"
                                    aria-hidden
                                />
                            </Anchor>
                        ) : (
                            l.cause
                        )}
                    </p>
                ) : (
                    <p className="text-xl text-muted">
                        {unannounced ? "Not announced yet. Usually white." : "No special lighting."}
                    </p>
                )}
                {children}
            </div>
        </section>
    );
}

export function NightSkeleton() {
    return (
        <section className="flex flex-col gap-5 sm:flex-row sm:items-end sm:gap-10">
            <Skeleton className={`${SCENE} sm:shrink-0`} radius="lg" />
            <div className="flex w-full flex-col gap-3">
                <Skeleton height={20} width="50%" />
                <Skeleton height={44} width="80%" />
                <Skeleton height={28} width="70%" />
            </div>
        </section>
    );
}

// Upcoming lists nights as rows: crown, day, colors and cause. A run of plain
// white nights is one row, so the special ones stand out, and so is a run of
// nights hidden for Prudle. Each opens that night (a hidden run, the game).
export function Upcoming({ nights, tonight }: { nights: Lighting[]; tonight?: string }) {
    const palette = usePalette();
    const { hidden } = usePrudle();
    const rows: Lighting[][] = [];
    for (const l of nights) {
        const last = rows[rows.length - 1];
        const masked = hidden(l.date);
        const joins =
            last && hidden(last[0].date) === masked && (masked || (!l.scheduled && !last[0].scheduled));
        if (joins) last.push(l);
        else rows.push([l]);
    }
    const dayName = (d: string) => (relativeName(d, tonight) === "Tomorrow" ? "Tomorrow" : weekdayAbbr(d));
    return (
        <ul className="flex flex-col divide-y divide-line overflow-hidden rounded-xl bg-panel">
            {rows.map((run) => {
                const l = run[0];
                const end = run[run.length - 1];
                const masked = hidden(l.date);
                return (
                    <li key={l.date}>
                        <Link
                            href={masked ? "/#prudle" : `/night?d=${l.date}`}
                            className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-raised sm:gap-4"
                        >
                            <Crown
                                bands={masked ? HIDDEN : bandsOf(l.colors, palette)}
                                className="h-11 w-11"
                            />
                            <span className="w-28 shrink-0 whitespace-nowrap">
                                <span className="block font-bold">
                                    {run.length > 1
                                        ? `${dayName(l.date)}–${weekdayAbbr(end.date)}`
                                        : dayName(l.date)}
                                </span>
                                <span className="block text-sm text-muted">
                                    {run.length > 1
                                        ? `${shortDate(l.date)}–${
                                              l.date.slice(0, 7) === end.date.slice(0, 7)
                                                  ? dayOfMonth(end.date)
                                                  : shortDate(end.date)
                                          }`
                                        : shortDate(l.date)}
                                </span>
                            </span>
                            <span className="min-w-0 flex-1">
                                <span className="block truncate text-lg">
                                    {masked ? HIDDEN_NAME : colorNames(l.colors, palette)}
                                </span>
                                <span className="line-clamp-2 text-muted">
                                    {masked ? "Hidden by Prudle" : l.scheduled ? l.cause : "Plain white"}
                                </span>
                            </span>
                            <IconChevronRight size={18} className="-mr-1 shrink-0 text-subtle" aria-hidden />
                        </Link>
                    </li>
                );
            })}
        </ul>
    );
}
