"use client";

import Link from "next/link";
import { Rating, Skeleton } from "@mantine/core";
import type { MySightings, MyStats } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { colorNames, shortDate, weekdayAbbr } from "@/lib/format";
import { bandsOf, usePalette } from "@/lib/palette";
import { Crown } from "@/components/Tower";
import { ErrorNote, Shell } from "@/components/Shell";

// /log is your nights: what you've seen and how you rated it.
export default function LogPage() {
    const { me } = useAuth();
    return (
        <Shell>
            {me === null ? (
                <div className="flex flex-col gap-3">
                    <h1 className="text-3xl font-bold">Your log</h1>
                    <p className="text-lg text-muted">
                        <Link href="/login" className="underline">
                            Sign in
                        </Link>{" "}
                        to see the nights you&apos;ve seen.
                    </p>
                </div>
            ) : me ? (
                <Log />
            ) : (
                <Skeleton height={300} radius="lg" />
            )}
        </Shell>
    );
}

function Log() {
    const palette = usePalette();
    const stats = useApi<MyStats>("/me/stats");
    const { data, error, reload } = useApi<MySightings>("/me/sightings");
    if (error) return <ErrorNote error={error} retry={reload} />;
    if (!data || !stats.data) return <Skeleton height={300} radius="lg" />;
    const st = stats.data;
    return (
        <div className="flex flex-col gap-6">
            <h1 className="text-3xl font-bold">Your log</h1>
            <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <Tile label="Nights seen" value={String(st.seen)} />
                <Tile label="In a row" value={String(st.streak)} />
                <Tile label="Average" value={st.rated ? `★ ${st.average.toFixed(1)}` : "–"} />
                <div className="flex flex-col gap-1 rounded-xl bg-panel p-4">
                    <dt className="text-sm text-muted">Your colors</dt>
                    <dd className="flex gap-1.5 pt-1">
                        {st.favorites.length
                            ? st.favorites.map((f) => (
                                  <span key={f.color} title={`${colorNames([f.color], palette)}: ${f.count}`}>
                                      <Crown bands={bandsOf([f.color], palette)} className="h-7 w-7" />
                                  </span>
                              ))
                            : "–"}
                    </dd>
                </div>
            </dl>
            {data.nights.length === 0 ? (
                <p className="rounded-xl bg-panel p-6 text-lg text-muted">
                    Nothing yet. When you see the crown lit, open{" "}
                    <Link href="/" className="underline">
                        tonight
                    </Link>{" "}
                    and tap &ldquo;I saw it&rdquo;. You can log the past week too.
                </p>
            ) : (
                <ul className="flex flex-col divide-y divide-line overflow-hidden rounded-xl bg-panel">
                    {data.nights.map((n) => (
                        <li key={n.date}>
                            <Link
                                href={`/night?d=${n.date}`}
                                className="flex items-start gap-4 px-4 py-3 transition-colors hover:bg-raised"
                            >
                                <Crown
                                    bands={bandsOf(n.lighting.colors, palette)}
                                    className="mt-1 h-11 w-11"
                                />
                                <span className="w-16 shrink-0">
                                    <span className="block font-bold">{weekdayAbbr(n.date)}</span>
                                    <span className="block text-sm text-muted">{shortDate(n.date)}</span>
                                </span>
                                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                                    <span className="truncate text-lg">
                                        {colorNames(n.lighting.colors, palette)}
                                    </span>
                                    <span className="truncate text-muted">
                                        {n.lighting.scheduled ? n.lighting.cause : "No special lighting"}
                                    </span>
                                    {n.rating > 0 && <Rating value={n.rating} readOnly size="xs" />}
                                    {n.note && <span className="text-fg">&ldquo;{n.note}&rdquo;</span>}
                                </span>
                            </Link>
                        </li>
                    ))}
                </ul>
            )}
        </div>
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
