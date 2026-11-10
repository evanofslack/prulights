"use client";

import Link from "next/link";
import { Anchor, Badge, Skeleton } from "@mantine/core";
import { IconClock, IconExternalLink } from "@tabler/icons-react";
import type { Lighting } from "@/interfaces/schedule";
import { bandsOf, usePalette } from "@/lib/palette";
import {
    colorNames,
    kindLabel,
    lightsOnTime,
    longDate,
    relativeName,
    shortDate,
    weekdayAbbr,
} from "@/lib/format";
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
            <div className="relative -mx-4 -mt-6 overflow-hidden sm:mx-0 sm:mt-0 sm:w-[46%] sm:shrink-0 sm:rounded-2xl">
                <Tower
                    bands={bands}
                    sky={sky}
                    className="block h-[64dvh] max-h-[640px] min-h-[400px] w-full"
                    title={`The Prudential Tower, crown lit ${colorNames(l.colors, palette)}`}
                />
                <div className="pointer-events-none absolute inset-x-0 bottom-0 h-20 bg-linear-to-b from-transparent to-surface sm:hidden" />
            </div>
            <div className="flex w-full flex-col justify-end gap-3 pb-2 text-center sm:text-left">
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
                        {unannounced
                            ? "Nothing announced yet. When the crown has no special lighting it shines white."
                            : "No special lighting. The crown shines white."}
                    </p>
                )}
                <div className="flex flex-wrap items-center justify-center gap-3 text-muted sm:justify-start">
                    <span className="inline-flex items-center gap-1.5 text-lg">
                        <IconClock size={20} aria-hidden />
                        {lit ? "Lit since " : "Lights on "}
                        {l.lights_on_source === "sunset" ? "around " : "at "}
                        {lightsOnTime(l.lights_on)}
                        {l.lights_on_source === "sunset" ? " (sunset)" : ""}
                    </span>
                    {l.scheduled && (
                        <Badge variant="light" color="gray" size="lg" radius="sm">
                            {kindLabel[l.kind] ?? l.kind}
                        </Badge>
                    )}
                </div>
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
        <ul className="flex flex-col divide-y divide-line overflow-hidden rounded-xl bg-panel">
            {rows.map((run) => {
                const l = run[0];
                const end = run[run.length - 1];
                return (
                    <li key={l.date}>
                        <Link
                            href={`/night?d=${l.date}`}
                            className="flex items-center gap-4 px-4 py-3 transition-colors hover:bg-raised"
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
                                    {l.scheduled ? l.cause : "No special lighting"}
                                </span>
                            </span>
                        </Link>
                    </li>
                );
            })}
        </ul>
    );
}
