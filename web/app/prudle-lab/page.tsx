"use client";

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Button, SegmentedControl, Skeleton } from "@mantine/core";
import type { LightingsResponse } from "@/interfaces/api";
import type { Lighting } from "@/interfaces/schedule";
import { api, useMeta } from "@/lib/api";
import { usePalette } from "@/lib/palette";
import { addDays } from "@/lib/format";
import { type CloseRule, type CountRule, type Rules, DEFAULT_RULES, rulesKey, shuffled } from "@/lib/prudle";
import { Shell, ErrorNote } from "@/components/Shell";
import { Board, type Played } from "./Board";
import { Stats } from "./Stats";

// /prudle-lab is a dev page for tuning Prudle: it deals every listed night in
// the schedule as a game, laid out as it would be on /, with the rules in the
// url and tucked under the board. Each result is kept in localStorage per rule
// set so the stats can say whether the limit is right.

const STORE = "prulights.prudle-lab.v2";
const CHUNK = 400; // maxRange in server/public.go

interface Settings extends Rules {
    cause: boolean;
    seed: number;
    program: "" | "regular" | "31nights";
    year: string;
    i: number;
}

function read(q: URLSearchParams): Settings {
    const num = (k: string, lo: number, hi: number, d: number) => {
        const n = Number(q.get(k));
        return q.has(k) && Number.isInteger(n) && n >= lo && n <= hi ? n : d;
    };
    const pick = <T extends string>(k: string, xs: readonly T[], d: T): T => {
        const v = q.get(k) as T;
        return xs.includes(v) ? v : d;
    };
    return {
        tries: num("tries", 3, 8, DEFAULT_RULES.tries),
        maxPicks: num("picks", 3, 4, DEFAULT_RULES.maxPicks),
        count: pick<CountRule>("count", ["dir", "exact"], DEFAULT_RULES.count),
        close: pick<CloseRule>("close", ["hint", "none"], DEFAULT_RULES.close),
        cause: q.get("cause") !== "0",
        seed: num("seed", 1, 1e9, 1),
        program: pick("program", ["", "regular", "31nights"] as const, ""),
        year: /^\d{4}$/.test(q.get("year") ?? "") ? q.get("year")! : "",
        i: num("i", 0, 1e6, 0),
    };
}

function write(s: Settings): string {
    const q = new URLSearchParams();
    if (s.tries !== DEFAULT_RULES.tries) q.set("tries", String(s.tries));
    if (s.maxPicks !== DEFAULT_RULES.maxPicks) q.set("picks", String(s.maxPicks));
    if (s.count !== DEFAULT_RULES.count) q.set("count", s.count);
    if (s.close !== DEFAULT_RULES.close) q.set("close", s.close);
    if (!s.cause) q.set("cause", "0");
    if (s.seed !== 1) q.set("seed", String(s.seed));
    if (s.program) q.set("program", s.program);
    if (s.year) q.set("year", s.year);
    if (s.i) q.set("i", String(s.i));
    return q.toString();
}

// useNights loads every listed night from the first source year through the
// last one the source lists, in chunks the api allows.
function useNights(first: number | undefined, through: string | undefined) {
    const [nights, setNights] = useState<Lighting[]>();
    const [error, setError] = useState<Error>();
    useEffect(() => {
        if (!first || !through) return;
        let gone = false;
        (async () => {
            const out: Lighting[] = [];
            for (let from = `${first}-01-01`; from <= through; from = addDays(from, CHUNK)) {
                const to = addDays(from, CHUNK - 1) < through ? addDays(from, CHUNK - 1) : through;
                const r = await api<LightingsResponse>(`/lightings?from=${from}&to=${to}&fill=false`);
                out.push(...r.lightings);
            }
            if (!gone) setNights(out.filter((l) => l.cause && l.colors.length));
        })().catch((e) => !gone && setError(e));
        return () => {
            gone = true;
        };
    }, [first, through]);
    return { nights, error };
}

type Saved = Record<string, Played[]>;

function load(): Saved {
    try {
        return JSON.parse(localStorage.getItem(STORE) ?? "{}");
    } catch {
        return {};
    }
}

function Lab() {
    const meta = useMeta();
    const palette = usePalette();
    const router = useRouter();
    const path = usePathname();
    const params = useSearchParams();
    const s = read(params);

    const pending = useRef<string | null>(null);
    const set = useCallback(
        (change: Partial<Settings>) => {
            const base = read(new URLSearchParams(pending.current ?? window.location.search));
            const qs = write({ ...base, ...change });
            pending.current = qs;
            router.replace(qs ? `${path}?${qs}` : path, { scroll: false });
        },
        [path, router],
    );
    useEffect(() => {
        if (pending.current === params.toString()) pending.current = null;
    }, [params]);

    const first = meta?.sources.length
        ? Math.min(...meta.sources.map((x) => x.year).filter((y) => y > 0))
        : undefined;
    const { nights, error } = useNights(meta?.dev ? first : undefined, meta?.published_through);
    const years = useMemo(() => [...new Set((nights ?? []).map((n) => n.date.slice(0, 4)))].sort(), [nights]);
    const deck = useMemo(
        () =>
            shuffled(
                (nights ?? []).filter(
                    (n) =>
                        new Set(n.colors).size <= s.maxPicks &&
                        (!s.program || n.program === s.program) &&
                        (!s.year || n.date.startsWith(s.year)),
                ),
                s.seed,
            ),
        [nights, s.program, s.year, s.seed, s.maxPicks],
    );

    const [saved, setSaved] = useState<Saved>({});
    useEffect(() => setSaved(load()), []);
    const key = rulesKey(s, s.cause);
    const results = useMemo(() => saved[key] ?? [], [saved, key]);
    const store = (next: Saved) => {
        setSaved(next);
        try {
            localStorage.setItem(STORE, JSON.stringify(next));
        } catch {}
    };
    const onDone = (p: Played) =>
        store({ ...saved, [key]: [...results.filter((r) => r.date !== p.date), p] });

    const answers = useMemo(() => deck.map((n) => n.colors), [deck]);
    const colors = useMemo(() => [...(palette?.keys() ?? [])], [palette]);

    if (!meta) return <Skeleton height={600} radius="lg" />;
    if (!meta.dev) {
        return (
            <div className="flex max-w-xl flex-col gap-3 rounded-xl bg-panel p-6">
                <h1 className="text-2xl font-bold">Prudle lab</h1>
                <p className="text-muted">
                    The lab only exists when the server runs in dev mode. Locally, run{" "}
                    <code className="rounded bg-raised px-1.5 py-0.5">just start</code>.
                </p>
            </div>
        );
    }
    if (error) return <ErrorNote error={error} />;
    if (!nights || !palette) return <Skeleton height={600} radius="lg" />;
    if (!deck.length) return <p className="text-muted">No nights match.</p>;

    const i = s.i % deck.length;
    const night = deck[i];
    const playedHere = results.some((r) => r.date === night.date);
    const rules: Rules = s;

    return (
        <div className="flex flex-col gap-8">
            <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between text-sm text-muted">
                    <span>
                        Night {i + 1} of {deck.length}
                        {playedHere && " · played"}
                    </span>
                    <span className="hidden sm:inline">Enter guess · ← → nights · ⌫ clear</span>
                    <span className="flex gap-1 sm:hidden">
                        <Button
                            size="compact-sm"
                            variant="subtle"
                            onClick={() => set({ i: (i - 1 + deck.length) % deck.length })}
                        >
                            ‹ back
                        </Button>
                        <Button
                            size="compact-sm"
                            variant="subtle"
                            onClick={() => set({ i: (i + 1) % deck.length })}
                        >
                            next ›
                        </Button>
                    </span>
                </div>
                <Board
                    key={`${night.date}-${key}-${s.i}`}
                    night={night}
                    rules={rules}
                    palette={palette}
                    showCause={s.cause}
                    onDone={onDone}
                    onNext={() => set({ i: (i + 1) % deck.length })}
                    onBack={() => set({ i: (i - 1 + deck.length) % deck.length })}
                />
            </div>

            <div className="flex flex-col gap-2 border-t border-line pt-4">
                <Fold title="Lab settings" note={key}>
                    <div className="grid gap-3 sm:grid-cols-2">
                        <Knob
                            label="Guesses allowed"
                            value={String(s.tries)}
                            data={["3", "4", "5", "6", "7", "8"]}
                            onChange={(v) => set({ tries: Number(v) })}
                        />
                        <Knob
                            label="Colors per guess"
                            value={String(s.maxPicks)}
                            data={["3", "4"]}
                            onChange={(v) => set({ maxPicks: Number(v), i: 0 })}
                        />
                        <Knob
                            label="Count feedback"
                            value={s.count}
                            data={[
                                { value: "dir", label: "fewer / more / right" },
                                { value: "exact", label: "right or not" },
                            ]}
                            onChange={(v) => set({ count: v as CountRule })}
                        />
                        <Knob
                            label="Close colors"
                            value={s.close}
                            data={[
                                { value: "hint", label: "marked close" },
                                { value: "none", label: "just off" },
                            ]}
                            onChange={(v) => set({ close: v as CloseRule })}
                        />
                        <Knob
                            label="Cause"
                            value={s.cause ? "1" : "0"}
                            data={[
                                { value: "1", label: "shown" },
                                { value: "0", label: "hidden" },
                            ]}
                            onChange={(v) => set({ cause: v === "1" })}
                        />
                        <Knob
                            label="Nights"
                            value={s.program}
                            data={[
                                { value: "", label: "all" },
                                { value: "regular", label: "regular" },
                                { value: "31nights", label: "31 Nights" },
                            ]}
                            onChange={(v) => set({ program: v as Settings["program"], i: 0 })}
                        />
                        <Knob
                            label="Year"
                            value={s.year}
                            data={[
                                { value: "", label: "all" },
                                ...years.map((y) => ({ value: y, label: y })),
                            ]}
                            onChange={(v) => set({ year: v, i: 0 })}
                        />
                    </div>
                    <div className="flex flex-wrap gap-2 pt-1">
                        <Button
                            variant="default"
                            size="xs"
                            onClick={() => set({ seed: Math.floor(Math.random() * 1e9) + 1, i: 0 })}
                        >
                            Reshuffle
                        </Button>
                        <Button
                            variant="default"
                            size="xs"
                            onClick={() =>
                                navigator.clipboard.writeText(
                                    JSON.stringify({ rules: key, results }, null, 1),
                                )
                            }
                        >
                            Copy results
                        </Button>
                        <Button
                            variant="subtle"
                            color="red"
                            size="xs"
                            onClick={() => {
                                const next = { ...saved };
                                delete next[key];
                                store(next);
                            }}
                        >
                            Reset these rules
                        </Button>
                    </div>
                    <p className="text-xs text-muted">
                        Results are kept per rule set; guesses allowed is applied after, so changing it keeps
                        them.
                    </p>
                </Fold>
                <Fold title="Results" note={`${results.length} played`}>
                    <Stats results={results} rules={rules} answers={answers} colors={colors} />
                </Fold>
            </div>
        </div>
    );
}

// Fold is a closed section under the board.
function Fold({ title, note, children }: { title: string; note?: string; children: React.ReactNode }) {
    return (
        <details className="group rounded-xl bg-panel">
            <summary className="flex cursor-pointer list-none items-center gap-2 px-4 py-3 select-none [&::-webkit-details-marker]:hidden">
                <span className="text-muted transition group-open:rotate-90">›</span>
                <span className="font-bold">{title}</span>
                {note && <span className="ml-auto truncate text-xs text-muted">{note}</span>}
            </summary>
            <div className="flex flex-col gap-3 px-4 pb-4">{children}</div>
        </details>
    );
}

function Knob({
    label,
    value,
    data,
    onChange,
}: {
    label: string;
    value: string;
    data: (string | { value: string; label: string })[];
    onChange: (v: string) => void;
}) {
    return (
        <div className="flex flex-col gap-1">
            <span className="text-sm text-muted">{label}</span>
            <SegmentedControl size="xs" value={value} data={data} onChange={onChange} />
        </div>
    );
}

export default function Page() {
    return (
        <Shell>
            <Suspense fallback={<Skeleton height={600} radius="lg" />}>
                <Lab />
            </Suspense>
        </Shell>
    );
}
