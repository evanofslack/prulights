import type { ColorKey } from "@/interfaces/schedule";

// Prudle: guess the night's exact set of colors. A guess is a whole set; it
// wins only when it is the answer, no more and no fewer. Every color comes
// back on, close (same family, see CLOSE) or off, and the guess's size comes
// back against the night's count (fewer, more or exact), so the count is
// narrowed, never shown. Colors found on stay in the next guess, since
// leaving one out can never win. Each guess is a try. Pure, no React: the
// lab (/prudle-lab) plays it in the browser to tune the rules; the real game
// is graded on the server (backend/prudle, the copy that counts), and
// backend/prudle/testdata/games.json keeps the two in step.

export type CountRule = "dir" | "exact"; // fewer/more/exact, or only exact or not
export type CloseRule = "hint" | "none"; // mark close colors, or call them off

export interface Rules {
    tries: number; // guesses allowed (the server's prudle.Tries)
    maxPicks: number; // the most colors in one guess (prudle.MaxPicks)
    count: CountRule;
    close: CloseRule;
}

export const DEFAULT_RULES: Rules = { tries: 6, maxPicks: 3, count: "dir", close: "hint" };

export type Mark = "hit" | "close" | "miss";
export type CountMark = "fewer" | "more" | "exact" | "wrong"; // wrong: not exact, under count=exact

export interface Pick {
    key: ColorKey;
    mark: Mark;
    carried?: boolean; // found on before this guess
}

export interface Turn {
    picks: Pick[]; // the whole guess: carried colors first, then the new ones
    size: number;
    count: CountMark;
}

export interface Game {
    answer: ColorKey[];
    turns: Turn[];
    on: ColorKey[]; // found on, in the order found
    out: Map<ColorKey, Mark>; // guessed and not in it: close or miss
    won: boolean;
}

// CLOSE are the families a near guess counts in: gold for yellow says "close".
// Each color is in at most one, so the grid can lay a family out as a row and
// "close" reads as "another color in this row is on". Black and rainbow are
// close to nothing. backend/prudle's Families is the same.
export const CLOSE: ColorKey[][] = [
    ["red", "maroon", "pink"],
    ["yellow", "gold", "orange"],
    ["blue", "purple"],
    ["green", "teal"],
    ["white", "silver"],
];

// GRID is the swatch layout: the families in rows, the loners last.
export const GRID: ColorKey[][] = [...CLOSE, ["black", "rainbow"]];

export function isClose(a: ColorKey, b: ColorKey): boolean {
    return a !== b && CLOSE.some((f) => f.includes(a) && f.includes(b));
}

// GLYPH is each mark as a light: on, half on, off. Safe to share: it says
// nothing about which colors.
export const GLYPH: Record<Mark, string> = { hit: "●", close: "◐", miss: "○" };

const MARK_ORDER: Mark[] = ["hit", "close", "miss"];

// byMark orders a guess good news first: on, close, off.
export function byMark(t: Turn): Pick[] {
    return [...t.picks].sort((a, b) => MARK_ORDER.indexOf(a.mark) - MARK_ORDER.indexOf(b.mark));
}

export function newGame(answer: ColorKey[]): Game {
    return { answer: [...new Set(answer)], turns: [], on: [], out: new Map(), won: false };
}

const setKey = (keys: ColorKey[]) => [...keys].sort().join(",");

// canPick is whether a color can be added to the next guess.
export function canPick(g: { won?: boolean; on: ColorKey[]; out: Map<ColorKey, Mark> }, key: ColorKey) {
    return !g.won && !g.on.includes(key) && !g.out.has(key);
}

// guessError says why the new picks aren't a guess yet, or "" when they are.
// The guess is the colors found on plus the picks. backend/prudle's Check.
export function guessError(
    g: { won?: boolean; on: ColorKey[]; out: Map<ColorKey, Mark>; turns: Turn[] },
    picks: ColorKey[],
    maxPicks: number,
): string {
    if (g.won) return "Solved";
    const size = g.on.length + picks.length;
    if (!size) return "Pick a color";
    if (picks.some((k) => !canPick(g, k))) return "Already guessed";
    if (size > maxPicks) return `At most ${maxPicks} colors`;
    const key = setKey([...g.on, ...picks]);
    if (g.turns.some((t) => setKey(t.picks.map((p) => p.key)) === key)) return "Already guessed";
    return "";
}

export function guess(g: Game, picks: ColorKey[], rules: Rules): Game {
    if (guessError(g, picks, rules.maxPicks)) return g;
    const on = [...g.on];
    const out = new Map(g.out);
    const fresh: Pick[] = picks.map((key) => {
        let mark: Mark = "miss";
        if (g.answer.includes(key)) mark = "hit";
        else if (rules.close === "hint" && g.answer.some((a) => isClose(a, key))) mark = "close";
        if (mark === "hit") on.push(key);
        else out.set(key, mark);
        return { key, mark };
    });
    const carried: Pick[] = g.on.map((key) => ({ key, mark: "hit", carried: true }));
    const size = g.on.length + picks.length;
    const n = g.answer.length;
    let count: CountMark = size === n ? "exact" : size > n ? "fewer" : "more";
    if (rules.count === "exact" && count !== "exact") count = "wrong";
    const won = size === n && fresh.every((p) => p.mark === "hit");
    return { ...g, turns: [...g.turns, { picks: [...carried, ...fresh], size, count }], on, out, won };
}

// known is the count once a guess was its size.
export function known(turns: Turn[]): number | undefined {
    return turns.find((t) => t.count === "exact")?.size;
}

// possible is every count still possible, from all the turns so far.
export function possible(turns: Turn[], maxPicks: number): number[] {
    const n = known(turns);
    if (n !== undefined) return [n];
    let lo = 1;
    let hi = maxPicks;
    for (const t of turns) {
        if (t.count === "more") lo = Math.max(lo, t.size + 1);
        if (t.count === "fewer") hi = Math.min(hi, t.size - 1);
        lo = Math.max(lo, t.picks.filter((p) => p.mark === "hit").length);
    }
    const not = new Set(turns.filter((t) => t.count === "wrong").map((t) => t.size));
    const out: number[] = [];
    for (let i = lo; i <= hi; i++) if (!not.has(i)) out.push(i);
    return out;
}

const colors = (n: number) => `${n} ${n === 1 ? "color" : "colors"}`;

// countText is what the guesses said of the count, in their own words: "1 to
// 3 colors" before any, then "More than 2 colors", "Fewer than 3 colors" or
// both, and "2 colors" only once a guess was that size.
export function countText(turns: Turn[], maxPicks: number): string {
    const n = known(turns);
    if (n !== undefined) return colors(n);
    const more = Math.max(0, ...turns.filter((t) => t.count === "more").map((t) => t.size));
    const fewer = Math.min(Infinity, ...turns.filter((t) => t.count === "fewer").map((t) => t.size));
    const not = [...new Set(turns.filter((t) => t.count === "wrong").map((t) => t.size))].sort();
    let text =
        more && fewer < Infinity
            ? `More than ${more}, fewer than ${fewer}`
            : more
              ? `More than ${colors(more)}`
              : fewer < Infinity
                ? `Fewer than ${colors(fewer)}`
                : `1 to ${maxPicks} colors`;
    if (not.length) text += `, not ${not.join(" or ")}`;
    return text;
}

// countMark is a turn's count in a few characters: "< 3", "> 1", "= 2", "≠ 3".
export function countMark(t: Turn): string {
    return { fewer: "<", more: ">", exact: "=", wrong: "≠" }[t.count] + " " + t.size;
}

// glyphs is a turn as lights, good news first; it names no colors.
export function glyphs(t: Turn): string {
    return byMark(t)
        .map((p) => GLYPH[p.mark])
        .join("");
}

// shareText is a finished game without its colors.
export function shareText(date: string, turns: Turn[], how: "won" | "lost" | "passed"): string {
    const n = turns.length;
    const head =
        how === "won"
            ? `Prudle ${date} · ${n} ${n === 1 ? "guess" : "guesses"}`
            : `Prudle ${date} · ${how === "lost" ? "out of guesses" : "peeked"}`;
    return [head, ...turns.map((t) => `${glyphs(t)}  ${countMark(t)}`)].join("\n");
}

// families with a close mark: their open colors are the ones to try
export function warm(out: Map<ColorKey, Mark>): Set<ColorKey> {
    return new Set(CLOSE.filter((f) => f.some((k) => out.get(k) === "close")).flat());
}

// Bots: clueless strategies (they never read the cause) for the lab's
// baseline. They guess the most common colors first, try a close color's
// family next, keep the guess's size to what the count can still be, and
// size the first guess by their opener.

export interface Bot {
    name: string;
    opener: number;
}

export const BOTS: Bot[] = [
    { name: "opens with 1", opener: 1 },
    { name: "opens with 2", opener: 2 },
    { name: "opens with max", opener: 99 },
];

// frequencies counts each color across the nights, most common first.
export function frequencies(answers: ColorKey[][]): ColorKey[] {
    const n = new Map<ColorKey, number>();
    for (const a of answers) for (const k of new Set(a)) n.set(k, (n.get(k) ?? 0) + 1);
    return [...n.entries()].sort((a, b) => b[1] - a[1]).map(([k]) => k);
}

export function playBot(bot: Bot, answer: ColorKey[], rules: Rules, ranked: ColorKey[]): Game {
    let g = newGame(answer);
    for (let turn = 0; !g.won && turn < 40; turn++) {
        const near = ranked.filter((k) => [...g.out].some(([o, m]) => m === "close" && isClose(o, k)));
        const pool = [...near, ...ranked.filter((k) => !near.includes(k))].filter((k) => canPick(g, k));
        const tried = new Set(g.turns.map((t) => setKey(t.picks.map((p) => p.key))));
        const sizes = possible(g.turns, rules.maxPicks).filter(
            (n) =>
                n >= g.on.length &&
                n - g.on.length <= pool.length &&
                !(n === g.on.length && tried.has(setKey(g.on))),
        );
        if (!sizes.length) break;
        const size =
            turn === 0 ? Math.min(Math.max(bot.opener, sizes[0]), sizes[sizes.length - 1]) : sizes[0];
        const next = guess(g, pool.slice(0, size - g.on.length), rules);
        if (next === g) break;
        g = next;
    }
    return g;
}

// rulesKey names a rule set for the lab's saved results; tries is applied
// after the game, so it isn't part of it.
export function rulesKey(r: Rules, cause: boolean): string {
    return `v2-p${r.maxPicks}-${r.count}-${r.close}-${cause ? "cause" : "blind"}`;
}

// shuffled is a seeded shuffle (mulberry32), so a deck is the same on reload.
export function shuffled<T>(xs: T[], seed: number): T[] {
    let a = seed >>> 0 || 1;
    const rand = () => {
        a = (a + 0x6d2b79f5) >>> 0;
        let t = a;
        t = Math.imul(t ^ (t >>> 15), t | 1);
        t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
    const out = [...xs];
    for (let i = out.length - 1; i > 0; i--) {
        const j = Math.floor(rand() * (i + 1));
        [out[i], out[j]] = [out[j], out[i]];
    }
    return out;
}
