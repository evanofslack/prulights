"use client";

import { useState } from "react";
import Link from "next/link";
import { Alert, Button, Skeleton } from "@mantine/core";
import type { PrudleNight, PrudleState } from "@/interfaces/api";
import type { Color, ColorKey } from "@/interfaces/schedule";
import { api, useApi } from "@/lib/api";
import { UNLIT, usePrudle } from "@/lib/blackout";
import { dayOfMonth, longDate } from "@/lib/format";
import { bandsOf, usePalette } from "@/lib/palette";
import { CLOSE, GRID, type Mark, type Turn, shareText } from "@/lib/prudle";
import { COUNT_MAX, CountMeter, History, Intro, LastGuess, Light, Misses, Swatch, fmt } from "./Prudle";
import { Tower } from "./Tower";

// Prudle on the home page, December only. PrudleBoard is tonight's game, in
// the place of tonight's colors until it's over; the server grades each guess
// and never sends the answer before then. PrudleResult, PrudleSection (the
// month's doors and how the family did) and PrudleSignIn come after.

const post = <T,>(path: string, body: unknown) =>
    api<T>(path, { method: "POST", body: JSON.stringify(body) });

export function PrudleBoard({ date }: { date: string }) {
    const { data, error, reload } = useApi<PrudleState>(`/me/prudle/${date}`);
    const palette = usePalette();
    if (error && !data) {
        return (
            <Alert color="red" variant="light" title="Prudle didn't load">
                {error.message}{" "}
                <button type="button" className="underline" onClick={reload}>
                    Try again
                </button>
            </Alert>
        );
    }
    if (!data || !palette) return <Skeleton height={640} radius="lg" />;
    return <Board key={data.date} initial={data} palette={palette} />;
}

function Board({ initial, palette }: { initial: PrudleState; palette: Map<ColorKey, Color> }) {
    const prudle = usePrudle();
    const [st, setSt] = useState(initial);
    const [picks, setPicks] = useState<ColorKey[]>([]);
    const [busy, setBusy] = useState(false);
    const [failed, setFailed] = useState("");
    const [peek, setPeek] = useState(false);

    const out = new Map(st.out.map((p) => [p.key, p.mark as Mark]));
    const turns = st.turns as Turn[];
    const last = turns.at(-1);
    const canPick = (k: ColorKey) => st.can_play && !st.found.includes(k) && !out.has(k);
    const size = st.found.length + picks.length;
    const problem = !picks.length
        ? "Pick a color"
        : size > st.max_picks
          ? `At most ${st.max_picks} colors`
          : "";
    // families with a close mark: their open colors are the ones to try
    const warm = new Set(CLOSE.filter((f) => f.some((k) => out.get(k) === "close")).flat());

    const send = async (path: string, body: unknown) => {
        setBusy(true);
        setFailed("");
        try {
            const next = await post<PrudleState>(path, body);
            setSt(next);
            setPicks([]);
            if (next.status !== "playing") prudle.reload(); // the colors show now
        } catch (e) {
            setFailed((e as Error).message);
        } finally {
            setBusy(false);
        }
    };
    const submit = () => !problem && !busy && send(`/me/prudle/${st.date}/guesses`, { colors: picks });
    const toggle = (k: ColorKey) => {
        if (!canPick(k)) return;
        setPicks((p) => (p.includes(k) ? p.filter((x) => x !== k) : [...p, k]));
    };

    // the crown: found colors, then empty bands once the count is known, or one unlit band
    const lit = st.found.length ? bandsOf(st.found, palette) : [];
    const unlit = st.bands ? st.lo - st.found.length : lit.length ? 0 : 1;
    const bands = [...lit, ...Array<string>(Math.max(0, unlit)).fill(UNLIT)];
    const open = Array.from({ length: COUNT_MAX }, (_, i) => i + 1).filter(
        (n) => n >= st.lo && (st.hi === 0 || n <= st.hi),
    );

    return (
        <section id="prudle" aria-label="Tonight's Prudle" className="flex scroll-mt-4 flex-col gap-4">
            <div className="relative overflow-hidden rounded-2xl">
                <Tower
                    bands={bands.length ? bands : [UNLIT]}
                    night={st.date}
                    className="block h-[26dvh] min-h-[200px] w-full"
                    title="The Prudential Tower, its colors still to guess"
                />
                <div className="absolute inset-x-0 top-0 bg-linear-to-b from-black/75 to-transparent p-4 text-white">
                    <div className="text-base opacity-85">Tonight&apos;s Prudle · {longDate(st.date)}</div>
                    <h1 className="text-2xl leading-tight font-bold sm:text-3xl">{st.cause}</h1>
                </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
                <CountMeter
                    known={st.bands ? st.lo : undefined}
                    found={st.found}
                    open={open}
                    palette={palette}
                />
                <Misses misses={st.misses} budget={st.budget} />
            </div>

            {last ? (
                <LastGuess turn={last} name={(k) => palette.get(k)?.label ?? k} palette={palette} />
            ) : (
                <Intro />
            )}

            <div className="flex flex-col gap-3">
                {GRID.map((row, i) => (
                    <div key={i} className="grid grid-cols-3 gap-2">
                        {row.map((k) => {
                            const c = palette.get(k);
                            if (!c) return null;
                            const mark = st.found.includes(k) ? "hit" : out.get(k);
                            return (
                                <Swatch
                                    key={k}
                                    c={c}
                                    palette={palette}
                                    mark={mark}
                                    picked={picks.includes(k)}
                                    hint={!mark && warm.has(k)}
                                    onClick={() => toggle(k)}
                                />
                            );
                        })}
                    </div>
                ))}
            </div>

            {failed && (
                <Alert color="red" variant="light">
                    {failed}
                </Alert>
            )}
            <Button size="xl" disabled={!!problem || busy} loading={busy} onClick={submit}>
                {problem && picks.length ? problem : `Guess ${size || ""}`.trim()}
            </Button>
            <div className="flex justify-center">
                {peek ? (
                    <span className="flex items-center gap-3 text-lg">
                        Show tonight&apos;s colors?
                        <Button variant="default" onClick={() => send(`/me/prudle/${st.date}/pass`, {})}>
                            Show me
                        </Button>
                        <button type="button" className="px-2 py-3 text-muted" onClick={() => setPeek(false)}>
                            Keep playing
                        </button>
                    </span>
                ) : (
                    <button
                        type="button"
                        onClick={() => setPeek(true)}
                        className="px-3 py-3 text-lg text-muted hover:text-fg"
                    >
                        Just show me ›
                    </button>
                )}
            </div>
            {turns.length > 1 && <History turns={turns} />}
        </section>
    );
}

// PrudleResult is how tonight's game went, under tonight's colors.
export function PrudleResult({ date }: { date: string }) {
    const { data } = useApi<PrudleState>(`/me/prudle/${date}`);
    const [shared, setShared] = useState(false);
    if (!data || data.status === "playing") return null;
    const n = data.turns.length;
    const turns = data.turns as Turn[];
    const status = data.status as "won" | "lost" | "passed";
    const share = async () => {
        const text = shareText(date, { turns, misses: data.misses }, status);
        try {
            if (navigator.share) await navigator.share({ text });
            else await navigator.clipboard.writeText(text);
            setShared(true);
        } catch {}
    };
    return (
        <div className="flex flex-col gap-3 rounded-xl bg-panel p-4 text-left" aria-label="Your Prudle">
            <div>
                <div className="text-xl font-bold">
                    {status === "won"
                        ? `Prudle: got it in ${n} ${n === 1 ? "guess" : "guesses"}`
                        : status === "lost"
                          ? "Prudle: out of misses"
                          : "Prudle: you peeked"}
                </div>
                {status !== "passed" && (
                    <div className="text-muted">
                        {fmt(data.misses)} {data.misses === 1 ? "miss" : "misses"}
                    </div>
                )}
            </div>
            {n > 0 && <History turns={turns} />}
            {n > 0 && (
                <Button variant="default" size="md" className="self-start" onClick={share}>
                    {shared ? "Copied" : "Share"}
                </Button>
            )}
        </div>
    );
}

const STATUS: Record<string, string> = { won: "got it", lost: "out of misses", passed: "peeked" };

// PrudleSection is December so far: a door a night, your wins and streak, and
// once you've played tonight, how the family did.
export function PrudleSection({ tonight }: { tonight: string }) {
    const { summary } = usePrudle();
    if (!summary?.season || summary.off) return null;
    const year = tonight.slice(0, 4);
    const games = new Map(summary.games.map((g) => [g.date, g]));
    return (
        <section
            id={summary.done || !summary.playable ? "prudle" : "prudle-month"}
            aria-label="Prudle"
            className="flex scroll-mt-4 flex-col gap-4 rounded-2xl bg-raised p-3 shadow-lg sm:p-6"
        >
            <div className="flex flex-wrap items-baseline gap-x-4">
                <h2 className="mr-auto text-2xl font-bold">Prudle</h2>
                <span className="text-lg text-muted">
                    {summary.won} won · streak {summary.streak}
                </span>
            </div>
            <ol className="grid grid-cols-7 gap-1.5 sm:gap-2" aria-label="December">
                {Array.from({ length: 31 }, (_, i) => {
                    const d = `${year}-12-${String(i + 1).padStart(2, "0")}`;
                    const g = games.get(d);
                    const status = g?.status ?? (d < tonight ? "missed" : d === tonight ? "today" : "later");
                    const mark: Mark | undefined =
                        status === "won"
                            ? "hit"
                            : status === "lost" || status === "passed"
                              ? "miss"
                              : undefined;
                    const cell = (
                        <span
                            className={`flex aspect-square flex-col items-center justify-center gap-0.5 rounded-lg ${
                                d === tonight
                                    ? "bg-panel ring-2 ring-accent"
                                    : status === "later"
                                      ? ""
                                      : "bg-panel"
                            } ${status === "later" || status === "missed" ? "text-subtle" : ""}`}
                            data-status={status}
                        >
                            <span className="text-sm sm:text-base">{dayOfMonth(d)}</span>
                            {mark ? (
                                <Light mark={mark} size={14} />
                            ) : (
                                <span className="h-3.5" aria-hidden>
                                    {status === "missed" ? "–" : ""}
                                </span>
                            )}
                        </span>
                    );
                    return (
                        <li key={d} aria-label={`${dayOfMonth(d)}: ${STATUS[status] ?? status}`}>
                            {d <= tonight ? (
                                <Link href={d === tonight ? "/#prudle" : `/night?d=${d}`}>{cell}</Link>
                            ) : (
                                cell
                            )}
                        </li>
                    );
                })}
            </ol>
            {summary.done && <Family date={tonight} />}
        </section>
    );
}

// Family is how everyone did tonight, marks only, never colors.
function Family({ date }: { date: string }) {
    const { data } = useApi<PrudleNight>(`/prudle/${date}`, 5 * 60_000);
    if (!data || data.locked || data.players.length === 0) return null;
    return (
        <div className="flex flex-col gap-2">
            <h3 className="text-lg font-bold">Tonight, everyone</h3>
            <ul className="flex flex-col divide-y divide-line overflow-hidden rounded-xl bg-panel">
                {data.players.map((p) => (
                    <li key={p.name} className="flex items-center gap-4 px-4 py-3">
                        <span className="w-24 shrink-0 truncate font-bold">{p.mine ? "You" : p.name}</span>
                        <span className="flex flex-col gap-1" aria-hidden>
                            {p.rows.map((r, i) => (
                                <span key={i} className="flex gap-1">
                                    {r.marks.map((m, j) => (
                                        <Light key={j} mark={m as Mark} size={14} />
                                    ))}
                                </span>
                            ))}
                        </span>
                        <span className="ml-auto text-right text-muted">
                            {STATUS[p.status]}
                            {p.status === "won" && (
                                <span className="block text-sm">
                                    {p.rows.length} {p.rows.length === 1 ? "guess" : "guesses"}
                                </span>
                            )}
                        </span>
                    </li>
                ))}
            </ul>
        </div>
    );
}

// PrudleSignIn invites someone signed out to play, under tonight's colors.
export function PrudleSignIn() {
    return (
        <div className="flex flex-col gap-3 rounded-xl bg-panel p-4 text-left">
            <p className="text-xl font-bold">Play Prudle</p>
            <p className="text-lg text-muted">
                Every December night, guess the crown&apos;s colors from the cause before you see them.
            </p>
            <Button component={Link} href="/login?next=/" size="lg" className="self-start">
                Sign in to play
            </Button>
        </div>
    );
}
