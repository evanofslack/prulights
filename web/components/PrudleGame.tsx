"use client";

import { useState } from "react";
import Link from "next/link";
import { Alert, Button, Skeleton } from "@mantine/core";
import type { PrudleGuess, PrudleNight, PrudleState } from "@/interfaces/api";
import type { Color, ColorKey } from "@/interfaces/schedule";
import { api, json, useApi } from "@/lib/api";
import { usePrudle } from "@/lib/blackout";
import { usePalette } from "@/lib/palette";
import { type Mark, type Turn, guessError, shareText } from "@/lib/prudle";
import { History, PrudleBoard as Board, crown } from "./Prudle";

// Prudle on the home page, December only. PrudleBoard is tonight's game, in
// the place of tonight's colors until it's over; the server grades each guess
// and never sends the answer before then. PrudleResult, PrudleSection (your
// wins and streak, and how the family did tonight) and PrudleSignIn come after.

// a guess, or {} to pass
const post = <T,>(path: string, body: PrudleGuess | Record<string, never>) =>
    api<T>(path, { method: "POST", body: json(body) });

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
    return <Game key={data.date} initial={data} palette={palette} />;
}

function Game({ initial, palette }: { initial: PrudleState; palette: Map<ColorKey, Color> }) {
    const prudle = usePrudle();
    const [st, setSt] = useState(initial);
    const [picks, setPicks] = useState<ColorKey[]>([]);
    const [busy, setBusy] = useState(false);
    const [failed, setFailed] = useState("");
    const [peek, setPeek] = useState(false);

    const out = new Map(st.out.map((p) => [p.key as ColorKey, p.mark as Mark]));
    const turns = st.turns as Turn[];
    const on = st.found as ColorKey[];
    const g = { on, out, turns, won: !st.can_play };
    const error = guessError(g, picks, st.max_picks);

    const send = async (path: string, body: PrudleGuess | Record<string, never>) => {
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
    const submit = () => !error && !busy && send(`/me/prudle/${st.date}/guesses`, { colors: picks });
    const toggle = (k: ColorKey) => {
        if (!st.can_play || on.includes(k) || out.has(k)) return;
        setPicks((p) => (p.includes(k) ? p.filter((x) => x !== k) : [...p, k]));
    };

    return (
        <section id="prudle" aria-label="Tonight's Prudle" className="scroll-mt-4">
            <Board
                date={st.date}
                label="Tonight's Prudle"
                cause={st.cause}
                bands={crown(on, turns, st.answer as ColorKey[] | undefined, palette)}
                palette={palette}
                turns={turns}
                on={on}
                out={out}
                tries={st.tries}
                maxPicks={st.max_picks}
                over={!st.can_play}
                picks={picks}
                error={error}
                busy={busy}
                onToggle={toggle}
                onSubmit={submit}
                note={
                    failed && (
                        <Alert color="red" variant="light">
                            {failed}
                        </Alert>
                    )
                }
                aside={
                    peek ? null : (
                        <button
                            type="button"
                            onClick={() => setPeek(true)}
                            className="px-2 py-3 text-muted hover:text-fg"
                        >
                            Just show me
                        </button>
                    )
                }
                result={null}
            />
            {peek && st.can_play && (
                <div className="mt-3 flex flex-wrap items-center justify-end gap-3 text-lg">
                    Show tonight&apos;s colors?
                    <Button variant="default" onClick={() => send(`/me/prudle/${st.date}/pass`, {})}>
                        Show me
                    </Button>
                    <button type="button" className="px-2 py-3 text-muted" onClick={() => setPeek(false)}>
                        Keep playing
                    </button>
                </div>
            )}
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
        const text = shareText(date, turns, status);
        try {
            if (navigator.share) await navigator.share({ text });
            else await navigator.clipboard.writeText(text);
            setShared(true);
        } catch {}
    };
    return (
        <div className="flex flex-col gap-3 rounded-xl bg-panel p-4 text-left" aria-label="Your Prudle">
            <div className="text-xl font-bold">
                {status === "won"
                    ? `Prudle: got it in ${n} ${n === 1 ? "guess" : "guesses"}`
                    : status === "lost"
                      ? "Prudle: out of guesses"
                      : "Prudle: you peeked"}
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

// PrudleSection is a small card: your wins and streak, and once you've played
// tonight, how the family did, marks only.
export function PrudleSection({ tonight }: { tonight: string }) {
    const { summary } = usePrudle();
    if (!summary?.season || summary.off) return null;
    if (!summary.done && summary.games.length === 0) return null;
    return (
        <section
            id={summary.done || !summary.playable ? "prudle" : "prudle-card"}
            aria-label="Prudle"
            className="flex scroll-mt-4 flex-col gap-3"
        >
            <div className="flex flex-wrap items-baseline gap-x-4">
                <h2 className="text-2xl font-bold">Prudle</h2>
                <span className="text-lg text-muted">
                    {summary.won} won
                    {summary.streak > 0 && ` · ${summary.streak} in a row`}
                </span>
            </div>
            {summary.done && <Family date={tonight} />}
        </section>
    );
}

const STATUS: Record<string, string> = { won: "got it", lost: "out of guesses", passed: "peeked" };

// Family is how everyone did tonight as badges, never colors.
function Family({ date }: { date: string }) {
    const { data } = useApi<PrudleNight>(`/prudle/${date}`, 5 * 60_000);
    if (!data || data.locked || data.players.length === 0) return null;
    return (
        <ul className="flex flex-wrap gap-2" aria-label="Tonight, everyone">
            {data.players.map((p) => (
                <li
                    key={p.name}
                    className="inline-flex min-h-11 items-center gap-2 rounded-full bg-panel py-1.5 pr-4 pl-3"
                >
                    <span className="font-mono text-sm tracking-widest" aria-hidden>
                        {p.rows
                            .at(-1)
                            ?.marks.map((m) => ({ hit: "●", close: "◐", miss: "○" })[m])
                            .join("")}
                    </span>
                    <span>
                        <span className="font-bold">{p.mine ? "You" : p.name}</span> {STATUS[p.status]}
                        {p.status === "won" && <span className="text-muted"> in {p.rows.length}</span>}
                    </span>
                </li>
            ))}
        </ul>
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
