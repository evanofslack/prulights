"use client";

import { useEffect, useMemo, useState } from "react";
import { Button, SegmentedControl, Skeleton } from "@mantine/core";
import type { LightingsResponse, TonightResponse } from "@/interfaces/api";
import type { Color, ColorKey, Lighting } from "@/interfaces/schedule";
import { useApi } from "@/lib/api";
import { addDays, fullDate } from "@/lib/format";
import { usePalette } from "@/lib/palette";
import { type Mode, type Saved, deal, load, pool, save } from "@/lib/practice";
import { type Game, MAX_PICKS, TRIES, guess, guessError, lost, newGame } from "@/lib/prudle";
import { History, PrudleBoard, crown, swatch } from "@/components/Prudle";
import { ErrorNote, Shell } from "@/components/Shell";

// /prudle is practice: Prudle on past nights from the rest of the year,
// graded in the browser, nothing saved but which nights were played and the
// mode. Easy start ramps from one-color nights up; Random is any night.

// usePool loads the last year of nights and this December's list, and keeps
// the past nights December won't show again.
function usePool() {
    const tonight = useApi<TonightResponse>("/tonight");
    const t = tonight.data?.date;
    const past = useApi<LightingsResponse>(
        t ? `/lightings?from=${addDays(t, -365)}&to=${addDays(t, -1)}&fill=false` : null,
    );
    const dec = useApi<LightingsResponse>(
        t ? `/lightings?from=${t.slice(0, 4)}-12-01&to=${t.slice(0, 4)}-12-31&fill=false` : null,
    );
    const nights = useMemo(
        () => (past.data && dec.data ? pool(past.data.lightings, dec.data.lightings) : undefined),
        [past.data, dec.data],
    );
    const error = tonight.error ?? past.error ?? dec.error;
    const retry = () => [tonight, past, dec].forEach((x) => x.error && x.reload());
    return { nights, error, retry };
}

const MODES: { value: Mode; label: string }[] = [
    { value: "easy", label: "Easy start" },
    { value: "random", label: "Random" },
];

export default function PracticePage() {
    const { nights, error, retry } = usePool();
    const palette = usePalette();
    const [saved, setSaved] = useState<Saved>();
    useEffect(() => setSaved(load()), []);
    const night = useMemo(() => (nights && saved ? deal(nights, saved) : undefined), [nights, saved]);

    const update = (s: Saved) => {
        save(s);
        setSaved(s);
    };
    // leave the night: finished games count toward Easy start, skips don't
    const leave = (finished: boolean) => {
        if (!saved || !night) return;
        update({
            ...saved,
            played: [...saved.played.filter((d) => d !== night.date), night.date],
            done: saved.done + (finished ? 1 : 0),
        });
        scrollTo({ top: 0 });
    };

    return (
        <Shell wide>
            <h1 className="sr-only">Practice Prudle</h1>
            {error && !nights && <ErrorNote error={error} retry={retry} />}
            {!error && (!nights || !palette || !saved) && <Skeleton height={620} radius="lg" />}
            {nights && palette && saved && !night && (
                <p className="rounded-xl bg-panel p-4 text-lg text-muted">No past nights to play yet.</p>
            )}
            {nights && palette && saved && night && (
                <Practice
                    key={night.date}
                    night={night}
                    palette={palette}
                    onNext={() => leave(true)}
                    onSkip={() => leave(false)}
                    mode={
                        <div className="mt-4 flex items-center gap-3">
                            <span className="shrink-0 text-lg font-bold">Nights</span>
                            <SegmentedControl
                                size="md"
                                fullWidth
                                color="gold"
                                className="flex-1 border-2 border-line"
                                aria-label="Which nights"
                                data={MODES}
                                value={saved.mode}
                                onChange={(m) => update({ ...saved, mode: m as Mode })}
                            />
                        </div>
                    }
                />
            )}
        </Shell>
    );
}

type End = "won" | "lost" | "shown";

function Practice({
    night,
    palette,
    onNext,
    onSkip,
    mode,
}: {
    night: Lighting;
    palette: Map<ColorKey, Color>;
    onNext: () => void;
    onSkip: () => void;
    mode: React.ReactNode;
}) {
    const [game, setGame] = useState<Game>(() => newGame(night.colors));
    const [picks, setPicks] = useState<ColorKey[]>([]);
    const [shown, setShown] = useState(false);
    const end: End | undefined = game.won ? "won" : lost(game) ? "lost" : shown ? "shown" : undefined;
    const error = guessError(game, picks, MAX_PICKS);

    const submit = () => {
        if (error || end) return;
        setGame(guess(game, picks));
        setPicks([]);
    };
    const toggle = (k: ColorKey) => {
        if (end || game.on.includes(k) || game.out.has(k)) return;
        setPicks((p) => (p.includes(k) ? p.filter((x) => x !== k) : [...p, k]));
    };
    const quiet = "px-2 py-3 text-muted hover:text-fg";

    return (
        <PrudleBoard
            scene
            date={night.date}
            label="Practice"
            when={fullDate(night.date)}
            cause={night.cause}
            bands={crown(game.on, game.turns, end ? game.answer : undefined, palette)}
            palette={palette}
            turns={game.turns}
            on={game.on}
            out={game.out}
            tries={TRIES}
            maxPicks={MAX_PICKS}
            over={!!end}
            picks={picks}
            error={error}
            onToggle={toggle}
            onSubmit={submit}
            aside={
                <span className="flex shrink-0">
                    <button type="button" onClick={() => setShown(true)} className={quiet}>
                        Show me
                    </button>
                    <button type="button" onClick={onSkip} className={quiet}>
                        Skip
                    </button>
                </span>
            }
            result={end && <Result game={game} end={end} palette={palette} onNext={onNext} />}
            foot={mode}
        />
    );
}

function Result({
    game,
    end,
    palette,
    onNext,
}: {
    game: Game;
    end: End;
    palette: Map<ColorKey, Color>;
    onNext: () => void;
}) {
    const n = game.turns.length;
    return (
        <div className="flex flex-col gap-4" aria-live="polite">
            <p className="text-2xl font-bold">
                {end === "won"
                    ? `Got it in ${n} ${n === 1 ? "guess" : "guesses"}`
                    : end === "lost"
                      ? "Out of guesses"
                      : "The colors"}
            </p>
            <ul className="flex flex-wrap gap-2">
                {game.answer.map((k) => {
                    const c = palette.get(k);
                    return (
                        <li
                            key={k}
                            className="flex items-center gap-2 rounded-full bg-panel py-1 pr-4 pl-1 text-xl"
                        >
                            {c && (
                                <span
                                    className="h-8 w-8 rounded-full border border-line"
                                    style={{ background: swatch(c, palette) }}
                                />
                            )}
                            {c?.label ?? k}
                        </li>
                    );
                })}
            </ul>
            {n > 0 && <History turns={game.turns} />}
            <Button size="xl" onClick={onNext} autoFocus>
                Next night
            </Button>
        </div>
    );
}
