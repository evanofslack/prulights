"use client";

import { useEffect, useState } from "react";
import { Button } from "@mantine/core";
import type { Color, ColorKey, Lighting } from "@/interfaces/schedule";
import { Tower } from "@/components/Tower";
import { bandsOf } from "@/lib/palette";
import { longDate } from "@/lib/format";
import { UNLIT } from "@/lib/blackout";
import {
    COUNT_MAX,
    CountMeter,
    History,
    Intro,
    LastGuess,
    Misses,
    Swatch,
    fmt,
    swatch,
} from "@/components/Prudle";
import {
    type Game,
    type Rules,
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
                        <CountMeter
                            known={game.bands ? game.answer.length : undefined}
                            found={game.found}
                            open={counts(game, COUNT_MAX)}
                            palette={palette}
                        />
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
                            .writeText(shareText(night.date, game, gaveUp ? "passed" : "won"))
                            .then(() => setCopied(true));
                    }}
                >
                    {copied ? "Copied" : "Share"}
                </Button>
            </div>
        </div>
    );
}
