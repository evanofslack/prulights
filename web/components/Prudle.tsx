"use client";

import type { Color, ColorKey } from "@/interfaces/schedule";
import { bandsOf, soft } from "@/lib/palette";
import { type Mark, type Turn, byMark, countMark, row } from "@/lib/prudle";

// The Prudle board's pieces, shared by the game on / (PrudleBoard.tsx) and the
// lab (/prudle-lab). The screen holds the state, not a log: the grid marks
// each color on, close or off (a family is a row, so "close" means another
// color in its row is on), the count meter holds what's known of how many, and
// a card reads the last guess back one color per line. The history is glyph
// rows.

export const COUNT_MAX = 4; // the most colors a night has had

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

export const WORD: Record<Mark, string> = { hit: "On", close: "Close", miss: "Off" };

export function Tag({ mark }: { mark: Mark }) {
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

export function Intro() {
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
export function LastGuess({
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
export function CountMeter({
    known,
    found,
    open,
    palette,
}: {
    known: number | undefined; // the count, once it's known
    found: ColorKey[];
    open: number[]; // the counts still possible, when not known
    palette: Map<ColorKey, Color>;
}) {
    if (known !== undefined) {
        return (
            <div className="flex items-center gap-2" aria-label={`${known} colors`}>
                <span className="text-sm text-muted">{known} colors</span>
                {Array.from({ length: known }, (_, i) => {
                    const k = found[i];
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

export function Misses({ misses, budget }: { misses: number; budget: number }) {
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

export function Swatch({
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
export function History({ turns }: { turns: Turn[] }) {
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

export function swatch(c: Color, palette: Map<ColorKey, Color>): string {
    const bands = bandsOf([c.key], palette);
    if (bands.length === 1) return soft(bands[0]);
    return `linear-gradient(90deg, ${bands.map((b) => soft(b)).join(", ")})`;
}

export const fmt = (n: number) => (Number.isInteger(n) ? String(n) : n.toFixed(1));
