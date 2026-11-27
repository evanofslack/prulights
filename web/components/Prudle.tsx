"use client";

import { Button } from "@mantine/core";
import type { Color, ColorKey } from "@/interfaces/schedule";
import { bandsOf, soft } from "@/lib/palette";
import { longDate } from "@/lib/format";
import { UNLIT } from "@/lib/blackout";
import {
    type Mark,
    type Turn,
    GRID,
    byMark,
    canPick,
    countMark,
    countText,
    glyphs,
    known,
    warm,
} from "@/lib/prudle";
import { Tower } from "./Tower";

// The Prudle board, shared by the game on / (PrudleGame.tsx) and the lab
// (/prudle-lab). It's laid out like tonight on /: the tower on the left (on a
// phone, on top) and the game beside it. The crown fills in as colors are
// found; the count line says what the guesses said of how many; the grid is
// small chips, a family to a row, marked on, close or off.

export const WORD: Record<Mark, string> = { hit: "On", close: "Close", miss: "Off" };

// Light is a mark as a lamp: filled for on, half for close, hollow for off.
export function Light({ mark, size = 14 }: { mark: Mark; size?: number }) {
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

// crown is the tower's bands: the answer once it's shown, else the colors
// found, then empty bands once the count is known, or one unlit band.
export function crown(
    on: ColorKey[],
    turns: Turn[],
    answer: ColorKey[] | undefined,
    palette: Map<ColorKey, Color>,
): string[] {
    if (answer?.length) return bandsOf(answer, palette);
    const lit = on.length ? bandsOf(on, palette) : [];
    const n = known(turns);
    const unlit = n !== undefined ? Math.max(0, n - on.length) : lit.length ? 0 : 1;
    const bands = [...lit, ...Array<string>(unlit).fill(UNLIT)];
    return bands.length ? bands : [UNLIT];
}

interface BoardProps {
    date: string;
    label: string; // "Tonight's Prudle"
    cause: string;
    bands: string[];
    palette: Map<ColorKey, Color>;
    turns: Turn[];
    on: ColorKey[];
    out: Map<ColorKey, Mark>;
    tries: number;
    maxPicks: number;
    over: boolean;
    picks: ColorKey[];
    error: string; // why the picks aren't a guess yet
    busy?: boolean;
    onToggle: (k: ColorKey) => void;
    onSubmit: () => void;
    // beside the Guess button: "give up", "Just show me"
    aside?: React.ReactNode;
    // in place of the game once it's over
    result?: React.ReactNode;
    // under the game: a failed guess
    note?: React.ReactNode;
}

export function PrudleBoard(p: BoardProps) {
    const last = p.turns.at(-1);
    const hints = warm(p.out);
    const size = p.on.length + p.picks.length;
    const g = { on: p.on, out: p.out, won: p.over };
    return (
        <div className="flex flex-col gap-5 sm:flex-row sm:items-start sm:gap-8">
            <div className="relative overflow-hidden rounded-2xl shadow-lg sm:sticky sm:top-4 sm:w-[42%] sm:shrink-0">
                <Tower
                    bands={p.bands}
                    night={p.date}
                    className="block h-[34dvh] min-h-[220px] w-full sm:h-[64dvh] sm:max-h-[620px] sm:min-h-[420px]"
                    title={
                        p.over ? "The Prudential Tower" : "The Prudential Tower, its colors still to guess"
                    }
                />
            </div>

            <div className="flex w-full min-w-0 flex-col gap-4 sm:pt-4">
                <div>
                    <p className="text-lg text-muted">
                        <span className="font-bold text-fg">{p.label}</span> · {longDate(p.date)}
                    </p>
                    <h1 className="text-3xl leading-tight font-bold sm:text-4xl">{p.cause}</h1>
                </div>

                {p.over ? (
                    p.result
                ) : (
                    <>
                        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1">
                            <span className="text-xl font-bold" aria-live="polite">
                                {countText(p.turns, p.maxPicks)}
                            </span>
                            <Tries used={p.turns.length} tries={p.tries} />
                        </div>

                        {last ? (
                            <LastGuess turn={last} palette={p.palette} />
                        ) : (
                            <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted">
                                Pick the colors, then Guess.
                                <Key />
                            </p>
                        )}

                        <ColorGrid
                            palette={p.palette}
                            mark={(k) => (p.on.includes(k) ? "hit" : p.out.get(k))}
                            picked={(k) => p.picks.includes(k)}
                            hint={(k) => hints.has(k) && canPick(g, k)}
                            onClick={p.onToggle}
                        />

                        {p.note}
                        <div className="flex items-center gap-3">
                            <Button
                                size="lg"
                                className="flex-1"
                                disabled={!!p.error || p.busy}
                                loading={p.busy}
                                onClick={p.onSubmit}
                            >
                                {p.error && (p.picks.length || p.on.length)
                                    ? p.error
                                    : `Guess ${size || ""} ${size === 1 ? "color" : size ? "colors" : ""}`.trim()}
                            </Button>
                            {p.aside}
                        </div>

                        {p.turns.length > 1 && <History turns={p.turns} tries={p.tries} />}
                    </>
                )}
            </div>
        </div>
    );
}

// ColorGrid is the palette as small chips, one family to a row (GRID), so
// "close" reads as "another color in this row is on". Report colors uses it
// too, with no marks.
export function ColorGrid({
    palette,
    mark,
    picked,
    hint,
    onClick,
}: {
    palette: Map<ColorKey, Color>;
    mark?: (k: ColorKey) => Mark | undefined;
    picked: (k: ColorKey) => boolean;
    hint?: (k: ColorKey) => boolean;
    onClick: (k: ColorKey) => void;
}) {
    return (
        <div className="flex flex-col gap-1.5">
            {GRID.map((row, i) => (
                <div key={i} className="grid grid-cols-3 gap-1.5">
                    {row.map((k) => {
                        const c = palette.get(k);
                        if (!c) return null;
                        return (
                            <Chip
                                key={k}
                                c={c}
                                palette={palette}
                                mark={mark?.(k)}
                                picked={picked(k)}
                                hint={hint?.(k) ?? false}
                                onClick={() => onClick(k)}
                            />
                        );
                    })}
                </div>
            ))}
        </div>
    );
}

// Chip is one color: a small swatch and its name. Colors marked on are
// locked into the next guess.
function Chip({
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
            className={`flex min-h-11 min-w-0 items-center gap-2 rounded-lg px-1.5 py-1 text-left text-sm font-medium transition ${
                picked || mark === "hit"
                    ? "bg-raised ring-2 ring-accent"
                    : hint
                      ? "outline-2 -outline-offset-2 outline-fg/60 outline-dashed"
                      : "hover:bg-panel"
            } ${mark === "miss" ? "opacity-40" : mark === "close" ? "opacity-70" : "active:scale-95"}`}
        >
            <span
                className="relative flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full border border-line"
                style={{ background: swatch(c, palette) }}
            >
                {mark === "miss" && (
                    <svg
                        className="absolute inset-0 h-full w-full text-fg/70"
                        viewBox="0 0 10 10"
                        aria-hidden
                    >
                        <line x1="1" y1="9" x2="9" y2="1" stroke="currentColor" strokeWidth="0.8" />
                    </svg>
                )}
                {mark && mark !== "miss" && (
                    <span className="rounded-full bg-surface/80 p-0.5 text-fg">
                        <Light mark={mark} size={12} />
                    </span>
                )}
            </span>
            <span className="truncate">{c.label}</span>
        </button>
    );
}

// Key is the marks in one line.
export function Key() {
    return (
        <span className="inline-flex items-center gap-3">
            {(["hit", "close", "miss"] as Mark[]).map((m) => (
                <span key={m} className="inline-flex items-center gap-1">
                    <Light mark={m} size={12} />
                    {WORD[m]}
                </span>
            ))}
        </span>
    );
}

// Tries is the guesses as dots: filled for used, then how far past the limit
// (the lab plays on past it).
export function Tries({ used, tries }: { used: number; tries: number }) {
    const over = used - tries;
    return (
        <span
            className="flex items-center gap-1 text-sm text-muted"
            aria-label={`${used} of ${tries} guesses`}
        >
            {Array.from({ length: tries }, (_, i) => (
                <Light key={i} mark={i < used ? "hit" : "miss"} size={12} />
            ))}
            <span className="ml-1 tabular-nums">
                {over > 0 ? <span className="font-bold text-danger">{over} over</span> : `${used}/${tries}`}
            </span>
        </span>
    );
}

// LastGuess is the last guess as one row of small chips, good news first,
// and what its size said of the count.
function LastGuess({ turn, palette }: { turn: Turn; palette: Map<ColorKey, Color> }) {
    return (
        <div className="flex flex-col gap-1.5 rounded-xl bg-panel px-3 py-2" aria-live="polite">
            <ul className="flex flex-wrap gap-x-3 gap-y-1">
                {byMark(turn).map((p) => {
                    const c = palette.get(p.key);
                    return (
                        <li
                            key={p.key}
                            className={`inline-flex items-center gap-1.5 ${p.mark === "miss" ? "text-muted" : ""}`}
                        >
                            {c && (
                                <span
                                    className="h-4 w-4 rounded-full border border-line"
                                    style={{ background: swatch(c, palette) }}
                                />
                            )}
                            <span className="font-medium">{c?.label ?? p.key}</span>
                            <Light mark={p.mark} size={12} />
                            <span className="text-sm">{WORD[p.mark]}</span>
                        </li>
                    );
                })}
            </ul>
            <p className="text-sm text-muted">
                {turn.count === "fewer"
                    ? `Fewer than ${turn.size} colors tonight.`
                    : turn.count === "more"
                      ? `More than ${turn.size} ${turn.size === 1 ? "color" : "colors"} tonight.`
                      : turn.count === "exact"
                        ? `${turn.size} is the right number.`
                        : `Not ${turn.size}.`}
                {turn.picks.some((p) => p.mark === "close") && " Close: another color in its row is on."}
            </p>
        </div>
    );
}

// History is the guesses as glyph rows with their count.
export function History({ turns, tries }: { turns: Turn[]; tries: number }) {
    return (
        <ol className="flex flex-col gap-0.5 rounded-xl bg-panel px-3 py-2 font-mono text-sm">
            {turns.map((t, i) => (
                <li key={i} className={`flex items-center gap-3 ${i >= tries ? "text-danger" : ""}`}>
                    <span className="w-4 text-muted">{i + 1}</span>
                    <span className="tracking-widest">{glyphs(t)}</span>
                    <span className="ml-auto text-muted">{countMark(t)}</span>
                </li>
            ))}
        </ol>
    );
}

// Answer is a night's colors as named pills.
export function Answer({ keys, palette }: { keys: ColorKey[]; palette: Map<ColorKey, Color> }) {
    return (
        <ul className="flex flex-wrap gap-2">
            {keys.map((k) => {
                const c = palette.get(k);
                return (
                    <li
                        key={k}
                        className="flex items-center gap-2 rounded-full bg-panel py-1 pr-3 pl-1 text-lg"
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
    );
}

export function swatch(c: Color, palette: Map<ColorKey, Color>): string {
    const bands = bandsOf([c.key], palette);
    if (bands.length === 1) return soft(bands[0]);
    return `linear-gradient(90deg, ${bands.map((b) => soft(b)).join(", ")})`;
}
