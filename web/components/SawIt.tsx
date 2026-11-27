"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, Rating, Textarea, Tooltip } from "@mantine/core";
import {
    IconArrowBackUp,
    IconCheck,
    IconEye,
    IconFlag,
    IconPencil,
    IconStarOff,
    IconX,
} from "@tabler/icons-react";
import type { NightSightings, NightStatsResponse, Report, Sighting, SightingRequest } from "@/interfaces/api";
import { api, json, sightingsChanged, useApi } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { usePrudle } from "@/lib/blackout";
import { MyPhoto, Thumb } from "./Photos";
import { ReportModal, useReports } from "./Report";
import { Swatches } from "./Swatches";

// SawIt is the "I saw it" panel under a night: first you (mark it seen, rate
// it, leave a note, say the colors were wrong), then everyone else, one row a
// person with their stars, note and any colors they say it really was.
// Signed out, it shows the anonymous totals.
export function SawIt({ date, tonight }: { date: string; tonight?: string }) {
    const { me } = useAuth();
    if (me === undefined) return null;
    if (me === null) return <PublicTotals date={date} tonight={tonight} />;
    return <Members date={date} />;
}

function Stars({ value, size = "sm" }: { value: number; size?: "xs" | "sm" | "md" }) {
    return value > 0 ? (
        <Rating value={value} fractions={2} readOnly size={size} aria-label={`${value} of 5 stars`} />
    ) : null;
}

function PublicTotals({ date, tonight }: { date: string; tonight?: string }) {
    const { data } = useApi<NightStatsResponse>(`/lightings/${date}/stats`);
    const future = tonight !== undefined && date > tonight;
    return (
        <div className="flex flex-col items-center gap-3 text-muted sm:items-start">
            {data && data.seen > 0 && (
                <span className="inline-flex items-center gap-2">
                    <IconEye size={18} aria-hidden /> Seen by {data.seen}
                    {data.rated > 0 && (
                        <>
                            {" · "}
                            <Stars value={Math.round(data.average * 2) / 2} size="sm" /> {data.average.toFixed(1)}
                        </>
                    )}
                </span>
            )}
            {!future && (
                <Button component={Link} href="/login" variant="default" size="md">
                    Sign in to log
                </Button>
            )}
        </div>
    );
}

// Row is one other person on a night: their sighting, their color report, or both.
interface Row {
    name: string;
    sighting?: Sighting;
    report?: Report;
}

function Members({ date }: { date: string }) {
    const { data, reload } = useApi<NightSightings>(`/sightings/${date}`);
    const reports = useReports(date);
    const hidden = usePrudle().hidden(date);
    const mine = data?.sightings.find((x) => x.mine);
    const others = data?.sightings.filter((x) => !x.mine) ?? [];
    const [note, setNote] = useState("");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();
    const [reporting, setReporting] = useState(false);

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

    // run does one change, with the panel busy and any error shown
    const run = async (fn: () => Promise<unknown>) => {
        setBusy(true);
        setError(undefined);
        try {
            await fn();
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setBusy(false);
        }
    };
    const changed = () => {
        reload();
        sightingsChanged();
    };
    const save = (change: { rating?: number; note?: string }) =>
        run(async () => {
            await api<Sighting>(`/me/sightings/${date}`, {
                method: "PUT",
                body: json<SightingRequest>({ rating: mine?.rating ?? 0, note: mine?.note ?? "", ...change }),
            });
            changed();
        });
    const remove = () =>
        run(async () => {
            await api<void>(`/me/sightings/${date}`, { method: "DELETE" });
            setNote("");
            changed();
        });

    // everyone else, one row each: what they saw and what they say it was
    const rows = new Map<string, Row>(others.map((x) => [x.name, { name: x.name, sighting: x }]));
    for (const r of reports?.others ?? []) {
        const row = rows.get(r.name);
        if (row) row.report = r;
        else rows.set(r.name, { name: r.name, report: r });
    }
    const myReport = reports?.mine;
    const canReport = reports?.canReport ?? false;

    return (
        <section aria-label="Sightings" className="flex flex-col gap-3 rounded-xl bg-panel p-4 text-left">
            {data.can_log && !mine && (
                <Button
                    size="lg"
                    leftSection={<IconEye size={22} />}
                    loading={busy}
                    onClick={() => save({})}
                    className="w-full sm:w-auto sm:self-start"
                >
                    I saw it
                </Button>
            )}
            {mine && (
                <div className="flex flex-col gap-3">
                    <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                        <span className="inline-flex shrink-0 items-center gap-1.5 text-lg font-bold text-accent">
                            <IconCheck size={20} /> You saw it
                        </span>
                        {data.can_log ? (
                            <Rating
                                value={mine.rating}
                                onChange={(rating) => save({ rating })}
                                fractions={2}
                                size="xl"
                                aria-label="Your rating"
                                readOnly={busy}
                            />
                        ) : (
                            <Stars value={mine.rating} size="md" />
                        )}
                    </div>
                    {/* the note is there as soon as you've seen it, rated or not */}
                    {data.can_log && (
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
                            <div className="flex flex-wrap items-center gap-3 empty:hidden">
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
                            </div>
                        </div>
                    )}
                    {!data.can_log && mine.note && (
                        <p id="note" className="scroll-mt-4 text-lg">
                            &ldquo;{mine.note}&rdquo;
                        </p>
                    )}
                    <MyPhoto mine={mine} canAdd={data.can_log} onChange={changed} />
                </div>
            )}
            {myReport && (
                <div className="flex flex-col gap-1 text-muted">
                    <span className="flex flex-wrap items-center gap-2">
                        You said <Thinks report={myReport} />
                    </span>
                    {myReport.note && <span>&ldquo;{myReport.note}&rdquo;</span>}
                </div>
            )}
            {/* the take-backs, apart from the stars so a thumb can't hit them by mistake */}
            {((data.can_log && mine) || canReport) && (
                <div className="-mx-2 flex flex-wrap items-center gap-x-2">
                    {data.can_log && mine && (
                        <Action
                            icon={<IconArrowBackUp size={20} />}
                            word="Undo"
                            hint="I didn't see it"
                            onClick={remove}
                        />
                    )}
                    {data.can_log && mine && mine.rating > 0 && (
                        <Action
                            icon={<IconStarOff size={20} />}
                            word="Clear stars"
                            hint="Clear my stars"
                            onClick={() => save({ rating: 0 })}
                        />
                    )}
                    {canReport && !myReport && (
                        <Action
                            icon={<IconFlag size={20} />}
                            word="Colors wrong?"
                            hint="Say what colors you saw"
                            onClick={() => setReporting(true)}
                        />
                    )}
                    {canReport && myReport && (
                        <>
                            <Action
                                icon={<IconPencil size={20} />}
                                word="Change"
                                hint="Change the colors you said"
                                onClick={() => setReporting(true)}
                            />
                            <Action
                                icon={<IconX size={20} />}
                                word="Take back"
                                hint="Take back the colors you said"
                                onClick={() => reports && run(reports.withdraw)}
                            />
                        </>
                    )}
                </div>
            )}
            {!data.can_log && !mine && !myReport && rows.size === 0 && (
                <p className="text-muted">Nobody logged this night.</p>
            )}
            {error && <p className="text-danger">{error}</p>}
            {rows.size > 0 && (
                <ul className="flex flex-col gap-3 border-t border-line pt-3" aria-label="Everyone else">
                    {[...rows.values()].map((x) => (
                        <li key={x.name} className="flex items-start gap-3">
                            {x.sighting && (
                                <Thumb sighting={x.sighting} covered={hidden} onRemoved={reload} />
                            )}
                            <span className="flex min-w-0 flex-col gap-0.5">
                                <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                                    <span className="font-bold">{x.name}</span>
                                    {x.sighting && (
                                        <>
                                            saw it <Stars value={x.sighting.rating} size="sm" />
                                        </>
                                    )}
                                    {x.report && <Thinks report={x.report} />}
                                </span>
                                {x.sighting?.note && (
                                    <span className="text-muted">&ldquo;{x.sighting.note}&rdquo;</span>
                                )}
                                {x.report?.note && (
                                    <span className="inline-flex items-start gap-1.5 text-muted">
                                        <IconFlag size={16} className="mt-1 shrink-0" aria-hidden />
                                        &ldquo;{x.report.note}&rdquo;
                                    </span>
                                )}
                            </span>
                        </li>
                    ))}
                </ul>
            )}
            {reports && (
                <ReportModal
                    date={date}
                    mine={myReport}
                    opened={reporting}
                    onClose={() => setReporting(false)}
                    onSent={() => {
                        setReporting(false);
                        reports.reload();
                    }}
                />
            )}
        </section>
    );
}

// Thinks is someone's "Colors wrong?" report, short: a flag and the colors
// they say it was.
function Thinks({ report }: { report: Report }) {
    return (
        <span className="inline-flex items-center gap-1.5 text-muted">
            <IconFlag size={16} aria-hidden />
            <span className="sr-only">says it was</span>
            <Swatches colors={report.colors} />
        </span>
    );
}

// Action is a quiet button for a take-back: the icon and a word on a phone,
// the icon alone with a tooltip on a computer.
function Action({
    icon,
    word,
    hint,
    onClick,
}: {
    icon: React.ReactNode;
    word: string;
    hint: string;
    onClick: () => void;
}) {
    return (
        <Tooltip label={hint} withArrow openDelay={200}>
            <button
                type="button"
                aria-label={word}
                onClick={onClick}
                className="inline-flex min-h-11 min-w-11 items-center justify-center gap-1.5 rounded-md px-2 text-muted transition-colors hover:bg-raised hover:text-fg"
            >
                {icon}
                <span className="sm:hidden">{word}</span>
            </button>
        </Tooltip>
    );
}
