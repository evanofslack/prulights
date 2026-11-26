"use client";

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Button, SegmentedControl, Skeleton } from "@mantine/core";
import type { LightingsResponse } from "@/interfaces/api";
import type { Lighting } from "@/interfaces/schedule";
import { api, useMeta } from "@/lib/api";
import { usePalette } from "@/lib/palette";
import { addDays } from "@/lib/format";
import {
    type BandsRule,
    type CloseRule,
    type CountRule,
    type Rules,
    DEFAULT_RULES,
    rulesKey,
    shuffled,
} from "@/lib/prudle";
import { Shell, ErrorNote } from "@/components/Shell";
import { Board, type Played } from "./Board";
import { Stats } from "./Stats";

// /prudle-lab is a dev page for tuning Prudle: it deals every listed night in
// the schedule as a game, with the rules in the url, and keeps each result in
// localStorage per rule set so the stats can say whether the budget is right.

const STORE = "prulights.prudle-lab.v1";
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
        budget: num("budget", 1, 8, DEFAULT_RULES.budget),
        maxPicks: num("picks", 4, 6, DEFAULT_RULES.maxPicks),
        count: pick<CountRule>("count", ["dir", "exact"], DEFAULT_RULES.count),
        close: pick<CloseRule>("close", ["none", "half", "full"], DEFAULT_RULES.close),
        bands: pick<BandsRule>("bands", ["exact", "known"], DEFAULT_RULES.bands),
        cause: q.get("cause") !== "0",
        seed: num("seed", 1, 1e9, 1),
        program: pick("program", ["", "regular", "31nights"] as const, ""),
        year: /^\d{4}$/.test(q.get("year") ?? "") ? q.get("year")! : "",
        i: num("i", 0, 1e6, 0),
    };
}

function write(s: Settings): string {
    const q = new URLSearchParams();
    if (s.budget !== DEFAULT_RULES.budget) q.set("budget", String(s.budget));
    if (s.maxPicks !== DEFAULT_RULES.maxPicks) q.set("picks", String(s.maxPicks));
    if (s.count !== DEFAULT_RULES.count) q.set("count", s.count);
    if (s.close !== DEFAULT_RULES.close) q.set("close", s.close);
    if (s.bands !== DEFAULT_RULES.bands) q.set("bands", s.bands);
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
                    (n) => (!s.program || n.program === s.program) && (!s.year || n.date.startsWith(s.year)),
                ),
                s.seed,
            ),
        [nights, s.program, s.year, s.seed],
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
        <div className="grid gap-6 lg:grid-cols-[minmax(0,28rem)_1fr]">
            <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between text-sm text-muted">
                    <span>
                        Night {i + 1} of {deck.length}
                        {playedHere && " · played"}
                    </span>
                    <span>Enter guess · ← → · ⌫ clear</span>
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

            <div className="flex flex-col gap-6">
                <section className="flex flex-col gap-3 rounded-xl bg-panel p-4">
                    <h1 className="text-xl font-bold">Prudle lab</h1>
                    <Knob
                        label="Misses allowed"
                        value={String(s.budget)}
                        data={["2", "3", "4", "5", "6"]}
                        onChange={(v) => set({ budget: Number(v) })}
                    />
                    <Knob
                        label="Colors per guess"
                        value={String(s.maxPicks)}
                        data={["4", "5"]}
                        onChange={(v) => set({ maxPicks: Number(v), i: 0 })}
                    />
                    <Knob
                        label="Count feedback"
                        value={s.count}
                        data={[
                            { value: "dir", label: "fewer / more / exact" },
                            { value: "exact", label: "exact only" },
                        ]}
                        onChange={(v) => set({ count: v as CountRule })}
                    />
                    <Knob
                        label="Close colors"
                        value={s.close}
                        data={[
                            { value: "half", label: "½ miss" },
                            { value: "full", label: "hint, 1 miss" },
                            { value: "none", label: "plain miss" },
                        ]}
                        onChange={(v) => set({ close: v as CloseRule })}
                    />
                    <Knob
                        label="Empty bands show"
                        value={s.bands}
                        data={[
                            { value: "exact", label: "on an exact count" },
                            { value: "known", label: "once count is known" },
                        ]}
                        onChange={(v) => set({ bands: v as BandsRule })}
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
                        data={[{ value: "", label: "all" }, ...years.map((y) => ({ value: y, label: y }))]}
                        onChange={(v) => set({ year: v, i: 0 })}
                    />
                    <div className="flex flex-wrap gap-2 pt-1">
                        <Button
                            variant="default"
                            onClick={() => set({ seed: Math.floor(Math.random() * 1e9) + 1, i: 0 })}
                        >
                            Reshuffle
                        </Button>
                        <Button
                            variant="default"
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
                        Results are kept per rule set ({key}); misses allowed is applied after, so changing it
                        keeps them.
                    </p>
                </section>
                <Stats results={results} rules={rules} answers={answers} colors={colors} />
            </div>
        </div>
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
        <Shell wide>
            <Suspense fallback={<Skeleton height={600} radius="lg" />}>
                <Lab />
            </Suspense>
        </Shell>
    );
}
