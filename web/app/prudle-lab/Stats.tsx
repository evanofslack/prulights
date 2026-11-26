"use client";

import { useMemo } from "react";
import type { ColorKey } from "@/interfaces/schedule";
import { BOTS, type Rules, frequencies, playBot } from "@/lib/prudle";
import type { Played } from "./Board";

// Stats answers "is the budget right": games are played without a limit, so
// the win rate at any budget comes from the same plays. Bots (who never read
// the cause) play the whole deck under the same rules for a baseline.

const BUDGETS = [2, 3, 4, 5, 6];

interface Props {
    results: Played[];
    rules: Rules;
    answers: ColorKey[][]; // the deck
    colors: ColorKey[]; // the palette
}

export function Stats({ results, rules, answers, colors }: Props) {
    const bots = useMemo(() => {
        const order = frequencies(answers);
        return BOTS.map((b) => {
            const games = answers.map((a) => playBot(b, a, rules, order, colors));
            return {
                name: b.name,
                misses: games.map((g) => (g.solved ? g.misses : Infinity)),
                turns: games.map((g) => g.turns.length),
            };
        });
    }, [answers, rules, colors]);

    const mine = results.map((r) => (r.solved ? r.misses : Infinity));
    const solved = results.filter((r) => r.solved);
    const avg = (xs: number[]) => (xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : 0);

    return (
        <div className="flex flex-col gap-5">
            <section>
                <h2 className="mb-2 text-lg font-bold">Win rate by misses allowed</h2>
                <table className="w-full text-sm">
                    <thead>
                        <tr className="text-left text-muted">
                            <th className="py-1 font-medium">who</th>
                            {BUDGETS.map((b) => (
                                <th
                                    key={b}
                                    className={`py-1 text-right font-medium ${b === rules.budget ? "text-fg" : ""}`}
                                >
                                    {b}
                                </th>
                            ))}
                            <th className="py-1 text-right font-medium">guesses</th>
                        </tr>
                    </thead>
                    <tbody>
                        <Row
                            name={`you (${results.length})`}
                            misses={mine}
                            turns={results.map((r) => r.turns)}
                            budget={rules.budget}
                            strong
                        />
                        {bots.map((b) => (
                            <Row
                                key={b.name}
                                name={b.name}
                                misses={b.misses}
                                turns={b.turns}
                                budget={rules.budget}
                            />
                        ))}
                    </tbody>
                </table>
                <p className="mt-1 text-xs text-muted">
                    Bots guess the most common colors first and never read the cause ({answers.length}{" "}
                    nights).
                </p>
            </section>

            {results.length > 0 && (
                <>
                    <section>
                        <h2 className="mb-2 text-lg font-bold">Your games</h2>
                        <dl className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm sm:grid-cols-4">
                            <Fact k="solved" v={`${solved.length}/${results.length}`} />
                            <Fact k="avg misses" v={avg(solved.map((r) => r.misses)).toFixed(1)} />
                            <Fact k="avg guesses" v={avg(solved.map((r) => r.turns)).toFixed(1)} />
                            <Fact k="avg time" v={`${Math.round(avg(results.map((r) => r.ms)) / 1000)}s`} />
                        </dl>
                    </section>
                    <section className="grid gap-5 sm:grid-cols-3">
                        <Hist title="Misses (solved)" counts={tally(solved.map((r) => r.misses))} />
                        <Hist
                            title="First guess size"
                            counts={tally(results.map((r) => r.first).filter((n) => n > 0))}
                        />
                        <ByCount results={results} />
                    </section>
                </>
            )}
        </div>
    );
}

function Row({
    name,
    misses,
    turns,
    budget,
    strong,
}: {
    name: string;
    misses: number[];
    turns: number[];
    budget: number;
    strong?: boolean;
}) {
    return (
        <tr className={`border-t border-line ${strong ? "font-bold" : ""}`}>
            <td className="py-1.5">{name}</td>
            {BUDGETS.map((b) => (
                <td key={b} className={`py-1.5 text-right tabular-nums ${b === budget ? "" : "text-muted"}`}>
                    {misses.length
                        ? `${Math.round((100 * misses.filter((m) => m <= b).length) / misses.length)}%`
                        : "–"}
                </td>
            ))}
            <td className="py-1.5 text-right text-muted tabular-nums">
                {turns.length ? (turns.reduce((a, c) => a + c, 0) / turns.length).toFixed(1) : "–"}
            </td>
        </tr>
    );
}

function Fact({ k, v }: { k: string; v: string }) {
    return (
        <div>
            <dt className="text-muted">{k}</dt>
            <dd className="text-lg font-bold tabular-nums">{v}</dd>
        </div>
    );
}

function tally(xs: number[]): [string, number][] {
    const m = new Map<number, number>();
    for (const x of xs) m.set(x, (m.get(x) ?? 0) + 1);
    return [...m.entries()].sort((a, b) => a[0] - b[0]).map(([k, v]) => [String(k), v]);
}

function Hist({ title, counts }: { title: string; counts: [string, number][] }) {
    const max = Math.max(1, ...counts.map(([, n]) => n));
    return (
        <div>
            <h3 className="mb-1 text-sm font-bold">{title}</h3>
            <ul className="flex flex-col gap-1 text-sm">
                {counts.map(([k, n]) => (
                    <li key={k} className="flex items-center gap-2">
                        <span className="w-8 text-right text-muted tabular-nums">{k}</span>
                        <span className="h-3 rounded bg-accent" style={{ width: `${(70 * n) / max}%` }} />
                        <span className="tabular-nums">{n}</span>
                    </li>
                ))}
            </ul>
        </div>
    );
}

function ByCount({ results }: { results: Played[] }) {
    const ns = [...new Set(results.map((r) => r.n))].sort();
    return (
        <div>
            <h3 className="mb-1 text-sm font-bold">By colors in the night</h3>
            <ul className="flex flex-col gap-1 text-sm">
                {ns.map((n) => {
                    const rs = results.filter((r) => r.n === n && r.solved);
                    const all = results.filter((r) => r.n === n).length;
                    const m = rs.length ? rs.reduce((a, r) => a + r.misses, 0) / rs.length : 0;
                    return (
                        <li key={n} className="tabular-nums">
                            <span className="font-bold">{n}</span> <span className="text-muted">·</span> {all}{" "}
                            played, {m.toFixed(1)} misses avg
                        </li>
                    );
                })}
            </ul>
        </div>
    );
}
