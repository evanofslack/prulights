"use client";

import { useEffect, useState } from "react";
import { Button } from "@mantine/core";
import type { Color, ColorKey, Lighting } from "@/interfaces/schedule";
import { Tower } from "@/components/Tower";
import { bandsOf, soft } from "@/lib/palette";
import { longDate } from "@/lib/format";
import {
    type Game,
    type Mark,
    type Rules,
    type Turn,
    CLOSE,
    GLYPH,
    GRID,
    byMark,
    canPick,
    countMark,
    counts,
    guess,
    guessError,
    newGame,
    row,
    shareText,
} from "@/lib/prudle";

// Board is one night's game. The screen holds the state, not a log: the grid
// marks each color on, close or off (a family is a row, so "close" means
// another color in its row is on), the count meter holds what's known of how
// many, and a card reads the last guess back one color per line. The history
// is glyph rows. In the lab a game never ends on misses; the budget only
// draws the line.

const UNLIT = "#3b3f4a"; // a band not found yet
const COUNT_MAX = 4; // the most colors a night has had

export interface Played {
    date: string;
    program: string;
    cause: string;
    n: number; // colors in the answer
    misses: number;
    solved: boolean; // false: gave up
    turns: number;
    first: number; // the size of the first guess
    ms: number;
    log: { picks: string; count: string }[];
}

interface Props {
    night: Lighting;
    rules: Rules;
    palette: Map<ColorKey, Color>;
    showCause: boolean;
    onDone: (p: Played) => void;
    onNext: () => void;
    onBack: () => void;
}

export function Board({ night, rules, palette, showCause, onDone, onNext, onBack }: Props) {
    const [game, setGame] = useState<Game>(() => newGame(night.colors));
    const [picks, setPicks] = useState<ColorKey[]>([]);
    const [gaveUp, setGaveUp] = useState(false);
    const [started] = useState(() => Date.now());
    const over = game.solved || gaveUp;
    const error = guessError(game, picks, rules);
    const name = (k: ColorKey) => palette.get(k)?.label ?? k;

    const finish = (g: Game, solved: boolean) =>
        onDone({
            date: night.date,
            program: night.program,
            cause: night.cause,
            n: g.answer.length,
            misses: g.misses,
            solved,
            turns: g.turns.length,
            first: g.turns[0]?.size ?? 0,
            ms: Date.now() - started,
            log: g.turns.map((t) => ({
                picks: `${"● ".repeat(row(t).carried)}${byMark(t)
                    .map((p) => `${GLYPH[p.mark]} ${p.key}`)
                    .join(" ")}`,
                count: countMark(t),
            })),
        });

    const submit = () => {
        if (error) return;
        const g = guess(game, picks, rules);
        setGame(g);
        setPicks([]);
        if (g.solved) finish(g, true);
    };
    const giveUp = () => {
        if (over) return;
        setGaveUp(true);
        finish(game, false);
    };
    const toggle = (k: ColorKey) => {
        if (!canPick(game, k)) return;
        setPicks((p) => (p.includes(k) ? p.filter((x) => x !== k) : [...p, k]));
    };

    // Enter guesses (or goes on), → next, ← back, Backspace clears
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (e.target instanceof HTMLInputElement || e.metaKey || e.ctrlKey || e.altKey) return;
            if (e.key === "Enter") over ? onNext() : submit();
            else if (e.key === "ArrowRight") onNext();
            else if (e.key === "ArrowLeft") onBack();
            else if (e.key === "Backspace") setPicks([]);
            else return;
            e.preventDefault();
        };
        window.addEventListener("keydown", onKey);
        return () => window.removeEventListener("keydown", onKey);
    });

    // the crown: found colors, then empty bands once the count shows, or one "?"
    const lit = game.found.length ? bandsOf(game.found, palette) : [];
    const unlit = game.bands ? game.answer.length - game.found.length : lit.length ? 0 : 1;
    const bands = over ? bandsOf(game.answer, palette) : [...lit, ...Array<string>(unlit).fill(UNLIT)];
    // families with a close mark: their open colors are the ones to try
    const warm = new Set(CLOSE.filter((f) => f.some((k) => game.out.get(k) === "close")).flat());
    const last = game.turns.at(-1);

    return (
        <div className="flex flex-col gap-4">
            <div className="relative overflow-hidden rounded-2xl">
                <Tower
                    bands={bands.length ? bands : [UNLIT]}
                    night={night.date}
                    className={`block w-full transition-[height] ${over ? "h-[42dvh] min-h-[300px]" : "h-[24dvh] min-h-[180px]"}`}
                />
                <div className="absolute inset-x-0 top-0 bg-linear-to-b from-black/70 to-transparent p-4 text-white">
                    <div className="text-sm opacity-80">{longDate(night.date)}</div>
                    <div className="text-2xl leading-tight font-bold">
                        {showCause || over ? night.cause || "No cause listed" : "Cause hidden"}
                    </div>
                </div>
            </div>

            {over ? (
                <Result
                    night={night}
                    game={game}
                    gaveUp={gaveUp}
                    rules={rules}
                    palette={palette}
                    onNext={onNext}
                />
            ) : (
                <>
                    <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
                        <CountMeter game={game} palette={palette} />
                        <Misses misses={game.misses} budget={rules.budget} />
                    </div>

                    {last ? <LastGuess turn={last} name={name} palette={palette} /> : <Intro />}

                    <div className="flex flex-col gap-3">
                        {GRID.map((row, i) => (
                            <div key={i} className="grid grid-cols-3 gap-2">
                                {row.map((k) => {
                                    const c = palette.get(k);
                                    if (!c) return null;
                                    const mark = game.found.includes(k) ? "hit" : game.out.get(k);
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

                    <div className="flex items-center gap-3">
                        <Button size="lg" className="flex-1" disabled={!!error} onClick={submit}>
                            {error && picks.length
                                ? error
                                : `Guess ${game.found.length + picks.length || ""}`.trim()}
                        </Button>
                        <button type="button" onClick={giveUp} className="px-2 py-3 text-muted hover:text-fg">
                            give up ›
                        </button>
                    </div>

                    {game.turns.length > 1 && <History turns={game.turns} />}
                </>
            )}
        </div>
    );
}

// Light is a mark as a lamp: filled for on, half for close, hollow for off.
function Light({ mark, size = 14 }: { mark: Mark; size?: number }) {
    const r = size / 2 - 1.25;
    const c = size / 2;
    return (
        <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="shrink-0" aria-hidden>
            <circle
                cx={c}
                cy={c}
                r={r}
                fill={mark === "hit" ? "currentColor" : "none"}
                stroke="currentColor"
                strokeWidth={2}
            />
            {mark === "close" && (
                <path d={`M${c} ${c - r} A${r} ${r} 0 0 0 ${c} ${c + r} Z`} fill="currentColor" />
            )}
        </svg>
    );
}

const WORD: Record<Mark, string> = { hit: "On", close: "Close", miss: "Off" };

function Tag({ mark }: { mark: Mark }) {
    return (
        <span
            className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-bold tracking-wide uppercase ${
                mark === "hit" ? "bg-fg text-surface" : "bg-surface text-fg"
            }`}
        >
            <Light mark={mark} size={12} />
            {WORD[mark]}
        </span>
    );
}

function Intro() {
    return (
        <div className="flex flex-col gap-2 rounded-xl bg-panel px-4 py-3">
            <p className="text-lg font-bold">Which colors are on tonight?</p>
            <p className="text-muted">Tap them, then Guess.</p>
            <ul className="flex flex-col gap-1 pt-1 text-sm">
                <li className="flex items-center gap-2">
                    <Light mark="hit" /> <b>On</b>
                </li>
                <li className="flex items-center gap-2">
                    <Light mark="close" /> <b>Close</b>
                    <span className="text-muted">another color in its row is on</span>
                </li>
                <li className="flex items-center gap-2">
                    <Light mark="miss" /> <b>Off</b>
                </li>
            </ul>
        </div>
    );
}

// LastGuess reads the last guess back, one color per line, good news first.
function LastGuess({
    turn,
    name,
    palette,
}: {
    turn: Turn;
    name: (k: ColorKey) => string;
    palette: Map<ColorKey, Color>;
}) {
    return (
        <ul className="flex flex-col divide-y divide-line rounded-xl bg-panel px-4" aria-live="polite">
            {byMark(turn).map((p) => {
                const c = palette.get(p.key);
                return (
                    <li
                        key={p.key}
                        className={`flex items-center gap-3 py-2 ${p.mark === "miss" ? "text-muted" : ""}`}
                    >
                        <Light mark={p.mark} size={16} />
                        {c && (
                            <span
                                className="h-5 w-5 rounded-full border border-line"
                                style={{ background: swatch(c, palette) }}
                            />
                        )}
                        <span className="text-lg font-medium">{name(p.key)}</span>
                        <span className="ml-auto text-right">
                            <b>{WORD[p.mark]}</b>
                            {p.mark === "close" && (
                                <span className="block text-sm text-muted">try its row</span>
                            )}
                        </span>
                    </li>
                );
            })}
        </ul>
    );
}

// CountMeter is what's known of how many colors: the numbers still possible,
// then, once known, a slot per color filling as they're found.
function CountMeter({ game, palette }: { game: Game; palette: Map<ColorKey, Color> }) {
    if (game.bands) {
        return (
            <div className="flex items-center gap-2" aria-label={`${game.answer.length} colors`}>
                <span className="text-sm text-muted">{game.answer.length} colors</span>
                {game.answer.map((_, i) => {
                    const k = game.found[i];
                    const c = k ? palette.get(k) : undefined;
                    return (
                        <span
                            key={i}
                            className={`h-7 w-7 rounded-full border-2 ${c ? "border-fg" : "border-dashed border-muted"}`}
                            style={c ? { background: swatch(c, palette) } : undefined}
                        />
                    );
                })}
            </div>
        );
    }
    const open = counts(game, COUNT_MAX);
    return (
        <div className="flex items-center gap-2">
            <span className="text-sm text-muted">How many?</span>
            {Array.from({ length: COUNT_MAX }, (_, i) => i + 1).map((n) => {
                const ok = open.includes(n);
                return (
                    <span
                        key={n}
                        className={`flex h-7 w-7 items-center justify-center rounded-full text-base font-bold tabular-nums ${
                            ok ? "border-2 border-fg" : "text-subtle line-through"
                        }`}
                    >
                        {n}
                    </span>
                );
            })}
        </div>
    );
}

function Misses({ misses, budget }: { misses: number; budget: number }) {
    const over = Math.max(0, misses - budget);
    return (
        <div
            className="flex items-center gap-1.5 text-sm text-muted"
            aria-label={`${misses} of ${budget} misses`}
        >
            {Array.from({ length: budget }, (_, i) => {
                const f = misses - i;
                return <Light key={i} mark={f >= 1 ? "hit" : f > 0 ? "close" : "miss"} />;
            })}
            <span className="ml-1">
                {fmt(misses)}/{budget} misses
                {over > 0 && <span className="font-bold text-danger"> · {fmt(over)} over</span>}
            </span>
        </div>
    );
}

function Swatch({
    c,
    palette,
    mark,
    picked,
    hint,
    onClick,
}: {
    c: Color;
    palette: Map<ColorKey, Color>;
    mark: Mark | undefined;
    picked: boolean;
    hint: boolean;
    onClick: () => void;
}) {
    return (
        <button
            type="button"
            onClick={onClick}
            disabled={!!mark}
            aria-pressed={picked || mark === "hit"}
            aria-label={mark ? `${c.label}: ${WORD[mark]}` : c.label}
            className={`flex flex-col items-center gap-1 rounded-xl p-1.5 text-base font-medium transition ${
                picked
                    ? "bg-raised ring-3 ring-accent"
                    : mark === "hit"
                      ? "bg-raised ring-2 ring-fg"
                      : hint
                        ? "outline-2 outline-offset-1 outline-fg/70 outline-dashed"
                        : ""
            } ${mark === "miss" ? "opacity-40" : mark === "close" ? "opacity-70" : "active:scale-95"}`}
        >
            <span
                className="relative flex h-11 w-full items-center justify-center overflow-hidden rounded-lg border border-line"
                style={{ background: swatch(c, palette) }}
            >
                {mark === "miss" && (
                    <svg
                        className="absolute inset-0 h-full w-full text-fg/60"
                        preserveAspectRatio="none"
                        viewBox="0 0 10 10"
                        aria-hidden
                    >
                        <line x1="0" y1="10" x2="10" y2="0" stroke="currentColor" strokeWidth="0.5" />
                    </svg>
                )}
                {mark && <Tag mark={mark} />}
            </span>
            {c.label}
        </button>
    );
}

// History is the guesses as glyph rows: the share, and in the real game
// behind a tap.
function History({ turns }: { turns: Turn[] }) {
    return (
        <ol className="flex flex-col gap-1 rounded-xl bg-panel px-4 py-3 font-mono text-lg">
            {turns.map((t, i) => (
                <li key={i} className="flex items-center gap-4">
                    <span className="w-4 text-sm text-muted">{i + 1}</span>
                    <span className="flex gap-1">
                        {row(t).marks.map((m, j) => (
                            // carried colors were found before: dimmer than this turn's news
                            <span key={j} className={j < row(t).carried ? "opacity-40" : ""}>
                                <Light mark={m} size={18} />
                            </span>
                        ))}
                    </span>
                    <span className="ml-auto text-base text-muted">{countMark(t)}</span>
                </li>
            ))}
        </ol>
    );
}

function Result({
    night,
    game,
    gaveUp,
    rules,
    palette,
    onNext,
}: {
    night: Lighting;
    game: Game;
    gaveUp: boolean;
    rules: Rules;
    palette: Map<ColorKey, Color>;
    onNext: () => void;
}) {
    const [copied, setCopied] = useState(false);
    const n = game.turns.length;
    const outAt = !gaveUp && game.misses > rules.budget;
    return (
        <div className="flex flex-col gap-4 rounded-xl bg-panel p-4">
            <div>
                <div className="text-2xl font-bold">
                    {gaveUp
                        ? "Passed"
                        : outAt
                          ? "Out of misses"
                          : `Got it in ${n} ${n === 1 ? "guess" : "guesses"}`}
                </div>
                <div className="text-muted">
                    {gaveUp || outAt
                        ? `${fmt(game.misses)} ${game.misses === 1 ? "miss" : "misses"}${outAt ? `, solved in ${n}` : ""}`
                        : `${fmt(game.misses)} ${game.misses === 1 ? "miss" : "misses"}`}
                </div>
            </div>
            <ul className="flex flex-wrap gap-2">
                {game.answer.map((k) => {
                    const c = palette.get(k);
                    return (
                        <li
                            key={k}
                            className="flex items-center gap-2 rounded-full bg-raised py-1 pr-3 pl-1 text-lg"
                        >
                            {c && (
                                <span
                                    className="h-7 w-7 rounded-full border border-line"
                                    style={{ background: swatch(c, palette) }}
                                />
                            )}
                            {c?.label ?? k}
                        </li>
                    );
                })}
            </ul>
            {n > 0 && <History turns={game.turns} />}
            <div className="flex gap-3">
                <Button size="lg" className="flex-1" onClick={onNext}>
                    Next night
                </Button>
                <Button
                    size="lg"
                    variant="default"
                    onClick={() => {
                        navigator.clipboard
                            .writeText(shareText(night.date, game, !gaveUp))
                            .then(() => setCopied(true));
                    }}
                >
                    {copied ? "Copied" : "Share"}
                </Button>
            </div>
        </div>
    );
}

function swatch(c: Color, palette: Map<ColorKey, Color>): string {
    const bands = bandsOf([c.key], palette);
    if (bands.length === 1) return soft(bands[0]);
    return `linear-gradient(90deg, ${bands.map((b) => soft(b)).join(", ")})`;
}

const fmt = (n: number) => (Number.isInteger(n) ? String(n) : n.toFixed(1));
