"use client";

import Link from "next/link";
import { Anchor, Skeleton } from "@mantine/core";
import { IconClock, IconExternalLink } from "@tabler/icons-react";
import type { Lighting } from "@/interfaces/schedule";
import { bandsOf, usePalette } from "@/lib/palette";
import { colorNames, lightsOnTime, longDate, relativeName, shortDate, weekdayAbbr } from "@/lib/format";
import { useSky } from "@/lib/sky";
import { Crown, Tower } from "./Tower";

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
    const bands = bandsOf(l.colors, palette);
    const when = relativeName(l.date, tonight);
    const unannounced = !l.scheduled && publishedThrough !== undefined && l.date > publishedThrough;
    const sky = useSky(l, tonight);
    const lit = l.date === tonight && sky === "night";
    return (
        <section className="flex flex-col gap-5 sm:flex-row sm:items-stretch sm:gap-10">
            {/* at night the tower runs to the phone's edges and fades into the page; on
                paper it's a night picture in a card */}
            <div className="relative overflow-hidden rounded-2xl shadow-lg sm:w-[46%] sm:shrink-0 dark:-mx-4 dark:-mt-6 dark:rounded-none dark:shadow-none sm:dark:mx-0 sm:dark:mt-0 sm:dark:rounded-2xl">
                <Tower
                    bands={bands}
                    sky={sky}
                    night={l.date}
                    live={l.date === tonight}
                    className="block h-[64dvh] max-h-[640px] min-h-[400px] w-full"
                    title={`The Prudential Tower, crown lit ${colorNames(l.colors, palette)}`}
                />
                <div className="pointer-events-none absolute inset-x-0 bottom-0 hidden h-20 bg-linear-to-b from-transparent to-surface dark:block sm:dark:hidden" />
            </div>
            {/* the text keeps a fixed top: centering it would move it each time the
                "I saw it" panel under it grows */}
            <div className="flex w-full flex-col gap-3 text-center sm:pt-16 sm:text-left">
                <p className="text-lg text-muted">
                    <span className="font-bold text-fg">{when}</span>
                    {when === "Tonight" || when === "Tomorrow" || when === "Last night"
                        ? ` · ${longDate(l.date)}`
                        : `, ${shortDate(l.date)}`}
                </p>
                <h1 className="text-4xl leading-tight font-bold sm:text-5xl">
                    {colorNames(l.colors, palette)}
                </h1>
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
                <p className="inline-flex items-center justify-center gap-1.5 text-lg text-muted sm:justify-start">
                    <IconClock size={20} aria-hidden />
                    {lit ? "Lit since " : "Lights on "}
                    {l.lights_on_source === "sunset" ? "~" : "at "}
                    {lightsOnTime(l.lights_on)}
                </p>
                {children}
            </div>
        </section>
    );
}

export function NightSkeleton() {
    return (
        <section className="flex flex-col gap-5 sm:flex-row sm:items-end sm:gap-10">
            <Skeleton className="h-[64dvh] max-h-[640px] min-h-[400px] sm:!w-[46%]" radius="lg" />
            <div className="flex w-full flex-col gap-3">
                <Skeleton height={20} width="50%" />
                <Skeleton height={44} width="80%" />
                <Skeleton height={28} width="70%" />
            </div>
        </section>
    );
}

// Upcoming lists nights as rows: crown, day, colors and cause. A run of plain
// white nights is one row, so the special ones stand out. Each opens that
// night.
export function Upcoming({ nights, tonight }: { nights: Lighting[]; tonight?: string }) {
    const palette = usePalette();
    const rows: Lighting[][] = [];
    for (const l of nights) {
        const last = rows[rows.length - 1];
        if (!l.scheduled && last && !last[0].scheduled) last.push(l);
        else rows.push([l]);
    }
    const dayName = (d: string) => (relativeName(d, tonight) === "Tomorrow" ? "Tomorrow" : weekdayAbbr(d));
    return (
        <ul className="flex flex-col divide-y divide-line overflow-hidden rounded-xl bg-panel dark:bg-raised dark:shadow-lg">
            {rows.map((run) => {
                const l = run[0];
                const end = run[run.length - 1];
                return (
                    <li key={l.date}>
                        <Link
                            href={`/night?d=${l.date}`}
                            className="flex items-center gap-4 px-4 py-3 transition-colors hover:bg-raised dark:hover:bg-line"
                        >
                            <Crown bands={bandsOf(l.colors, palette)} className="h-11 w-11" />
                            <span className="w-28 shrink-0">
                                <span className="block font-bold">
                                    {run.length > 1
                                        ? `${dayName(l.date)} – ${weekdayAbbr(end.date)}`
                                        : dayName(l.date)}
                                </span>
                                <span className="block text-sm text-muted">
                                    {run.length > 1
                                        ? `${shortDate(l.date)} – ${shortDate(end.date)}`
                                        : shortDate(l.date)}
                                </span>
                            </span>
                            <span className="min-w-0 flex-1">
                                <span className="block truncate text-lg">
                                    {colorNames(l.colors, palette)}
                                </span>
                                <span className="block truncate text-muted">
                                    {l.scheduled ? l.cause : "Plain white"}
                                </span>
                            </span>
                        </Link>
                    </li>
                );
            })}
        </ul>
    );
}
