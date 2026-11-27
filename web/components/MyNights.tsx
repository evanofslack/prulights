"use client";

import { useState } from "react";
import Link from "next/link";
import { Button, Rating, Skeleton } from "@mantine/core";
import type { MySightings, MyStats } from "@/interfaces/api";
import { useApi, useOnSightingsChanged } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { colorNames, shortDate, weekdayAbbr } from "@/lib/format";
import { bandsOf, usePalette } from "@/lib/palette";
import { HIDDEN, HIDDEN_NAME, usePrudle } from "@/lib/blackout";
import { Crown } from "./Tower";
import { ErrorNote } from "./Shell";

const RECENT = 5;

// MyNights is the signed-in section of the home page: what you've seen and
// how you rated it. Signed out it isn't there at all (sign-in is invite only).
export function MyNights() {
    const { me } = useAuth();
    if (!me) return null;
    return (
        <section
            id="nights"
            aria-label="Your nights"
            className="flex scroll-mt-4 flex-col gap-4 rounded-2xl bg-raised p-3 shadow-lg sm:p-6"
        >
            <h2 className="text-2xl font-bold">Your nights</h2>
            <Log />
        </section>
    );
}

function Log() {
    const palette = usePalette();
    const { hidden } = usePrudle();
    const stats = useApi<MyStats>("/me/stats");
    const { data, error, reload } = useApi<MySightings>("/me/sightings");
    useOnSightingsChanged(stats.reload);
    useOnSightingsChanged(reload);
    const [all, setAll] = useState(false);
    if (error) return <ErrorNote error={error} retry={reload} />;
    if (!data || !stats.data) return <Skeleton height={300} radius="lg" />;
    const st = stats.data;
    const shown = all ? data.nights : data.nights.slice(0, RECENT);
    // a hidden night you logged would show in your colors: leave them out till it shows
    const favorites = data.nights.some((n) => hidden(n.date)) ? [] : st.favorites;
    // nothing logged yet: just the invitation, not a row of zeros
    if (data.nights.length === 0) {
        return (
            <p className="rounded-xl bg-panel p-6 text-lg text-muted">
                Nothing yet. Tap <span className="font-bold text-fg">I saw it</span> when the crown is lit.
            </p>
        );
    }
    return (
        <>
            <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-2">
                <Tile label="Nights seen" value={String(st.seen)} />
                <Tile label="Streak" value={String(st.streak)} />
                <Tile label="Average" value={st.rated ? `★ ${st.average.toFixed(1)}` : "–"} />
                <div className="flex flex-col gap-1 rounded-xl bg-panel p-4">
                    <dt className="text-sm text-muted">Your colors</dt>
                    <dd className="flex gap-1.5 pt-1">
                        {favorites.length
                            ? favorites.map((f) => (
                                  <span key={f.color} title={`${colorNames([f.color], palette)}: ${f.count}`}>
                                      <Crown bands={bandsOf([f.color], palette)} className="h-7 w-7" />
                                  </span>
                              ))
                            : "–"}
                    </dd>
                </div>
            </dl>
            <ul className="flex flex-col divide-y divide-line overflow-hidden rounded-xl bg-panel">
                {shown.map((n) => (
                    <li key={n.date}>
                        <Link
                            href={`/night?d=${n.date}`}
                            className="flex items-start gap-4 px-4 py-3 transition-colors hover:bg-raised"
                        >
                            <Crown
                                bands={hidden(n.date) ? HIDDEN : bandsOf(n.lighting.colors, palette)}
                                className="mt-1 h-11 w-11"
                            />
                            <span className="w-16 shrink-0">
                                <span className="block font-bold">{weekdayAbbr(n.date)}</span>
                                <span className="block text-sm text-muted">{shortDate(n.date)}</span>
                            </span>
                            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                                <span className="truncate text-lg">
                                    {hidden(n.date) ? HIDDEN_NAME : colorNames(n.lighting.colors, palette)}
                                </span>
                                <span className="truncate text-muted">
                                    {n.lighting.scheduled ? n.lighting.cause : "Plain white"}
                                </span>
                                {n.rating > 0 && <Rating value={n.rating} readOnly size="sm" />}
                                {n.note && <span className="text-fg">&ldquo;{n.note}&rdquo;</span>}
                            </span>
                        </Link>
                    </li>
                ))}
            </ul>
            {data.nights.length > RECENT && (
                <Button variant="default" size="md" onClick={() => setAll(!all)} className="self-start">
                    {all ? "Show fewer" : `Show all (${data.nights.length})`}
                </Button>
            )}
        </>
    );
}

function Tile({ label, value }: { label: string; value: string }) {
    return (
        <div className="flex flex-col gap-1 rounded-xl bg-panel p-4">
            <dt className="text-sm text-muted">{label}</dt>
            <dd className="text-2xl font-bold">{value}</dd>
        </div>
    );
}
