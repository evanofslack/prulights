"use client";

import { useEffect, useState } from "react";
import { Button } from "@mantine/core";
import type { Color, ColorKey, Lighting } from "@/interfaces/schedule";
import { Answer, History, PrudleBoard, crown } from "@/components/Prudle";
import {
    type Game,
    type Rules,
    canPick,
    countMark,
    guess,
    guessError,
    newGame,
    shareText,
} from "@/lib/prudle";

// Board is one night's game in the lab: the home page's board
// (components/Prudle.tsx) played in the browser under the lab's rules. A
// game never ends on tries here; the limit only draws the line.

export interface Played {
    date: string;
    program: string;
    cause: string;
    n: number; // colors in the answer
    turns: number;
    solved: boolean; // false: gave up
    first: number; // the size of the first guess
    ms: number;
    log: { guess: string; count: string }[];
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
    const over = game.won || gaveUp;
    const error = guessError(game, picks, rules.maxPicks);

    const finish = (g: Game, solved: boolean) =>
        onDone({
            date: night.date,
            program: night.program,
            cause: night.cause,
            n: g.answer.length,
            turns: g.turns.length,
            solved,
            first: g.turns[0]?.size ?? 0,
            ms: Date.now() - started,
            log: g.turns.map((t) => ({
                guess: t.picks.map((p) => `${p.key}:${p.mark}`).join(" "),
                count: countMark(t),
            })),
        });

    const submit = () => {
        if (error) return;
        const g = guess(game, picks, rules);
        setGame(g);
        setPicks([]);
        if (g.won) finish(g, true);
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

    return (
        <PrudleBoard
            date={night.date}
            label="Prudle"
            cause={showCause || over ? night.cause || "No cause listed" : "Cause hidden"}
            bands={crown(game.on, game.turns, over ? game.answer : undefined, palette)}
            palette={palette}
            turns={game.turns}
            on={game.on}
            out={game.out}
            tries={rules.tries}
            maxPicks={rules.maxPicks}
            over={over}
            picks={picks}
            error={error}
            onToggle={toggle}
            onSubmit={submit}
            aside={
                <button type="button" onClick={giveUp} className="px-2 py-3 text-muted hover:text-fg">
                    give up
                </button>
            }
            result={
                <Result
                    night={night}
                    game={game}
                    gaveUp={gaveUp}
                    rules={rules}
                    palette={palette}
                    onNext={onNext}
                />
            }
        />
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
    const over = n - rules.tries;
    return (
        <div className="flex flex-col gap-4">
            <div>
                <div className="text-2xl font-bold">
                    {gaveUp ? "Passed" : `Got it in ${n} ${n === 1 ? "guess" : "guesses"}`}
                </div>
                {!gaveUp && over > 0 && (
                    <div className="text-danger">
                        {over} past the {rules.tries}-guess limit
                    </div>
                )}
            </div>
            <Answer keys={game.answer} palette={palette} />
            {n > 0 && <History turns={game.turns} tries={rules.tries} />}
            <div className="flex gap-3">
                <Button size="lg" className="flex-1" onClick={onNext}>
                    Next night
                </Button>
                <Button
                    size="lg"
                    variant="default"
                    onClick={() => {
                        navigator.clipboard
                            .writeText(shareText(night.date, game.turns, gaveUp ? "passed" : "won"))
                            .then(() => setCopied(true));
                    }}
                >
                    {copied ? "Copied" : "Share"}
                </Button>
            </div>
        </div>
    );
}
