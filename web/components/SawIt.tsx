"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, Rating, Textarea } from "@mantine/core";
import { IconCheck, IconEye } from "@tabler/icons-react";
import type { NightSightings, NightStatsResponse, Sighting, SightingRequest } from "@/interfaces/api";
import { api, json, sightingsChanged, useApi } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { usePrudle } from "@/lib/blackout";
import { MyPhoto, Thumb } from "./Photos";
import { Reports } from "./Report";

// SawIt is the "I saw it" panel under a night: mark it seen, rate it, leave a
// note, and see who else saw it. Signed out, it shows the anonymous totals.
export function SawIt({ date, tonight }: { date: string; tonight?: string }) {
    const { me } = useAuth();
    if (me === undefined) return null;
    if (me === null) return <PublicTotals date={date} tonight={tonight} />;
    return <Members date={date} />;
}

function Stars({ value, size = "sm" }: { value: number; size?: "xs" | "sm" | "md" }) {
    return value > 0 ? (
        <Rating value={value} readOnly size={size} aria-label={`${value} of 5 stars`} />
    ) : null;
}

function PublicTotals({ date, tonight }: { date: string; tonight?: string }) {
    const { data } = useApi<NightStatsResponse>(`/lightings/${date}/stats`);
    const future = tonight !== undefined && date > tonight;
    return (
        <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1 text-muted sm:justify-start">
            {data && data.seen > 0 && (
                <span className="inline-flex items-center gap-2">
                    <IconEye size={18} aria-hidden /> Seen by {data.seen}
                    {data.rated > 0 && (
                        <>
                            {" · "}
                            <Stars value={Math.round(data.average)} size="xs" /> {data.average.toFixed(1)}
                        </>
                    )}
                </span>
            )}
            {!future && (
                <Link href="/login" className="underline hover:text-fg">
                    Sign in to log
                </Link>
            )}
        </div>
    );
}

function Members({ date }: { date: string }) {
    const { data, reload } = useApi<NightSightings>(`/sightings/${date}`);
    const hidden = usePrudle().hidden(date);
    const mine = data?.sightings.find((x) => x.mine);
    const others = data?.sightings.filter((x) => !x.mine) ?? [];
    const [note, setNote] = useState("");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();

    useEffect(() => setNote(mine?.note ?? ""), [mine?.note, date]);

    // a calendar night with your note links here as #note: bring it into view
    // once it's there (too late for the browser to follow the hash itself)
    const loaded = data?.date === date;
    useEffect(() => {
        if (loaded && location.hash === "#note") {
            document.getElementById("note")?.scrollIntoView({ block: "center" });
        }
    }, [loaded]);

    if (!data || data.date !== date) return null;

    const save = async (change: { rating?: number; note?: string }) => {
        setBusy(true);
        setError(undefined);
        try {
            await api<Sighting>(`/me/sightings/${date}`, {
                method: "PUT",
                body: json<SightingRequest>({ rating: mine?.rating ?? 0, note: mine?.note ?? "", ...change }),
            });
            reload();
            sightingsChanged();
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setBusy(false);
        }
    };
    const remove = async () => {
        setBusy(true);
        try {
            await api<void>(`/me/sightings/${date}`, { method: "DELETE" });
            setNote("");
            reload();
            sightingsChanged();
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setBusy(false);
        }
    };

    // the note opens under the stars once you've rated (or already wrote one);
    // every change adds or removes rows below, so nothing above it moves
    const noting = mine !== undefined && (mine.rating > 0 || (mine.note ?? "") !== "" || note.trim() !== "");
    return (
        <section aria-label="Sightings" className="flex flex-col gap-3 rounded-xl bg-panel p-4 text-left">
            {data.can_log && !mine && (
                <Button
                    size="md"
                    variant="light"
                    leftSection={<IconEye size={20} />}
                    loading={busy}
                    onClick={() => save({})}
                    className="self-center sm:self-start"
                >
                    I saw it
                </Button>
            )}
            {mine && (
                <div className="flex flex-col gap-3">
                    <div className="flex items-center gap-3">
                        <span className="inline-flex shrink-0 items-center gap-1.5 text-lg font-bold text-accent">
                            <IconCheck size={20} /> You saw it
                        </span>
                        {data.can_log ? (
                            <Rating
                                value={mine.rating}
                                onChange={(rating) => save({ rating })}
                                size="lg"
                                aria-label="Your rating"
                                readOnly={busy}
                            />
                        ) : (
                            <Stars value={mine.rating} size="md" />
                        )}
                        {data.can_log && (
                            <button
                                type="button"
                                className="ml-auto min-h-11 shrink-0 px-1 text-muted underline hover:text-fg"
                                onClick={remove}
                            >
                                Undo
                            </button>
                        )}
                    </div>
                    {data.can_log && noting && (
                        <div id="note" className="flex scroll-mt-4 flex-col gap-2">
                            <Textarea
                                aria-label="Your note"
                                placeholder="A note about it (optional)"
                                value={note}
                                onChange={(e) => setNote(e.currentTarget.value)}
                                maxLength={280}
                                size="md"
                                autosize
                                minRows={2}
                            />
                            <div className="flex min-h-11 flex-wrap items-center gap-3">
                                {note.trim() !== (mine.note ?? "") && (
                                    <Button
                                        size="sm"
                                        variant="light"
                                        loading={busy}
                                        onClick={() => save({ note })}
                                    >
                                        Save note
                                    </Button>
                                )}
                                {note.length > 240 && (
                                    <span className="text-sm text-muted">{280 - note.length} left</span>
                                )}
                                {mine.rating > 0 && (
                                    <button
                                        type="button"
                                        className="ml-auto min-h-11 px-1 text-muted underline hover:text-fg"
                                        onClick={() => save({ rating: 0 })}
                                    >
                                        Clear stars
                                    </button>
                                )}
                            </div>
                        </div>
                    )}
                    {!data.can_log && mine.note && (
                        <p id="note" className="scroll-mt-4 text-lg">
                            &ldquo;{mine.note}&rdquo;
                        </p>
                    )}
                    <MyPhoto
                        mine={mine}
                        canAdd={data.can_log}
                        onChange={() => {
                            reload();
                            sightingsChanged();
                        }}
                    />
                </div>
            )}
            {!data.can_log && !mine && others.length === 0 && (
                <p className="text-muted">Nobody logged this night.</p>
            )}
            {error && <p className="text-danger">{error}</p>}
            {others.length > 0 && (
                <ul className="flex flex-col gap-2 border-t border-line pt-3" aria-label="Also seen by">
                    {others.map((x) => (
                        <li key={x.name} className="flex items-start gap-3">
                            <Thumb sighting={x} covered={hidden} onRemoved={reload} />
                            <span className="flex min-w-0 flex-col">
                                <span className="flex items-center gap-2">
                                    <span className="font-bold">{x.name}</span> saw it{" "}
                                    <Stars value={x.rating} size="xs" />
                                </span>
                                {x.note && <span className="text-muted">&ldquo;{x.note}&rdquo;</span>}
                            </span>
                        </li>
                    ))}
                </ul>
            )}
            <Reports date={date} />
        </section>
    );
}
