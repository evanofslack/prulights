import type { ColorKey } from "@/interfaces/schedule";

// Prudle: guess the night's colors. Each turn is a whole guess, a set of
// colors; the ones already found stay in it for free. Every color comes back
// a hit, close (the same family, see CLOSE) or a miss, and the guess's size
// comes back against the night's count: fewer, more, or exactly right. Misses
// cost; hits don't. Solved when every color is found. Pure, no React: the lab
// (/prudle-lab) plays it in the browser; the real game is graded on the
// server (backend/prudle, the same rules at DEFAULT_RULES, and the copy that
// counts), and this file only draws its state.

export type CountRule = "dir" | "exact"; // fewer/more/exact, or only exact
export type CloseRule = "none" | "half" | "full"; // no close mark; close costs ½; close is marked but costs 1
export type BandsRule = "exact" | "known"; // show the empty bands on an exact count, or once the count is known

export interface Rules {
    budget: number; // misses allowed: a game with more is lost (the lab applies it after)
    maxPicks: number; // the most colors in one guess
    count: CountRule;
    close: CloseRule;
    bands: BandsRule;
}

export const DEFAULT_RULES: Rules = { budget: 5, maxPicks: 4, count: "dir", close: "half", bands: "exact" };

export type Mark = "hit" | "close" | "miss";
export type CountMark = "fewer" | "more" | "exact" | "wrong"; // wrong: not exact, under count=exact

export interface Turn {
    picks: { key: ColorKey; mark: Mark }[]; // the new colors of this guess
    size: number; // the whole guess, found colors included
    count: CountMark;
}

export interface Game {
    answer: ColorKey[];
    turns: Turn[];
    found: ColorKey[]; // in the order found
    out: Map<ColorKey, Mark>; // guessed and not in it: miss or close
    misses: number;
    lo: number; // what the guesser knows of the count
    hi: number; // Infinity until a "fewer"
    bands: boolean; // the crown shows its empty bands
    solved: boolean;
}

// CLOSE are the families a near guess counts in: gold for yellow says "close".
// Each color is in at most one, so the grid can lay a family out as a row and
// "close" reads as "another color in this row is on". Black and rainbow are
// close to nothing.
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

// byMark orders a turn's picks good news first: on, close, off.
export function byMark(t: Turn): Turn["picks"] {
    return [...t.picks].sort((a, b) => MARK_ORDER.indexOf(a.mark) - MARK_ORDER.indexOf(b.mark));
}

// row is a turn as the whole guess it was: the colors already found (carried,
// on) and then the new picks, good news first. It has one mark per color, so
// it lines up with the turn's count.
export function row(t: Turn): { marks: Mark[]; carried: number } {
    const carried = t.size - t.picks.length;
    return { marks: [...Array<Mark>(carried).fill("hit"), ...byMark(t).map((p) => p.mark)], carried };
}

// counts is every count still possible, 1..max, from all the turns so far.
export function counts(g: Game, max: number): number[] {
    const not = new Set(g.turns.filter((t) => t.count === "wrong").map((t) => t.size));
    const out: number[] = [];
    for (let n = 1; n <= max; n++) if (n >= g.lo && n <= g.hi && !not.has(n)) out.push(n);
    return out;
}

// countMark is a turn's count in a few characters: "< 3", "> 1", "= 2", "≠ 3".
export function countMark(t: Turn): string {
    return { fewer: "<", more: ">", exact: "=", wrong: "≠" }[t.count] + " " + t.size;
}

// shareText is a finished game without its colors.
export function shareText(
    date: string,
    g: { turns: Turn[]; misses: number },
    result: "won" | "lost" | "passed",
): string {
    const n = g.turns.length;
    const misses = `${g.misses} ${g.misses === 1 ? "miss" : "misses"}`;
    const head =
        result === "won"
            ? `Prudle ${date} · ${n} ${n === 1 ? "guess" : "guesses"} · ${misses}`
            : result === "lost"
              ? `Prudle ${date} · out of misses`
              : `Prudle ${date} · peeked`;
    return [
        head,
        ...g.turns.map(
            (t) =>
                `${row(t)
                    .marks.map((m) => GLYPH[m])
                    .join("")}  ${countMark(t)}`,
        ),
    ].join("\n");
}

export function newGame(answer: ColorKey[]): Game {
    return {
        answer: [...new Set(answer)],
        turns: [],
        found: [],
        out: new Map(),
        misses: 0,
        lo: 1,
        hi: Infinity,
        bands: false,
        solved: false,
    };
}

// canPick is whether a color can go in the next guess.
export function canPick(g: Game, key: ColorKey): boolean {
    return !g.solved && !g.found.includes(key) && !g.out.has(key);
}

// guessError says why picks aren't a guess yet, or "" when they are.
export function guessError(g: Game, picks: ColorKey[], rules: Rules): string {
    if (g.solved) return "Solved";
    if (!picks.length) return "Pick a color";
    if (picks.some((k) => !canPick(g, k))) return "Already guessed";
    if (g.found.length + picks.length > rules.maxPicks) return `At most ${rules.maxPicks} colors`;
    return "";
}

export function guess(g: Game, picks: ColorKey[], rules: Rules): Game {
    if (guessError(g, picks, rules)) return g;
    const found = [...g.found];
    const out = new Map(g.out);
    let misses = g.misses;
    const marked = picks.map((key) => {
        let mark: Mark = "miss";
        if (g.answer.includes(key)) mark = "hit";
        else if (rules.close !== "none" && g.answer.some((a) => isClose(a, key))) mark = "close";
        if (mark === "hit") found.push(key);
        else {
            out.set(key, mark);
            misses += mark === "close" && rules.close === "half" ? 0.5 : 1;
        }
        return { key, mark };
    });
    const size = g.found.length + picks.length;
    const n = g.answer.length;
    let count: CountMark = size === n ? "exact" : size > n ? "fewer" : "more";
    if (rules.count === "exact" && count !== "exact") count = "wrong";
    let { lo, hi } = g;
    if (count === "exact") lo = hi = n;
    else if (count === "fewer") hi = Math.min(hi, size - 1);
    else if (count === "more") lo = Math.max(lo, size + 1);
    lo = Math.max(lo, found.length);
    const solved = found.length === n;
    const bands = g.bands || count === "exact" || (rules.bands === "known" && lo === hi) || solved;
    return {
        ...g,
        turns: [...g.turns, { picks: marked, size, count }],
        found,
        out,
        misses,
        lo,
        hi,
        bands,
        solved,
    };
}

// Bots: clueless strategies (they never read the cause) for a baseline. Each
// guesses the most common colors first and sizes its guess by its opener,
// then keeps within what it knows of the count.

export interface Bot {
    name: string;
    opener: number; // the size of the first guess
}

export const BOTS: Bot[] = [
    { name: "one at a time", opener: 1 },
    { name: "opens with 2", opener: 2 },
    { name: "opens with 3", opener: 3 },
    { name: "opens with max", opener: 99 },
];

// frequencies counts each color across the nights, most common first.
export function frequencies(answers: ColorKey[][]): ColorKey[] {
    const n = new Map<ColorKey, number>();
    for (const a of answers) for (const k of new Set(a)) n.set(k, (n.get(k) ?? 0) + 1);
    return [...n.entries()].sort((a, b) => b[1] - a[1]).map(([k]) => k);
}

export function playBot(
    bot: Bot,
    answer: ColorKey[],
    rules: Rules,
    order: ColorKey[],
    all: ColorKey[],
): Game {
    let g = newGame(answer);
    const ranked = [...order, ...all.filter((k) => !order.includes(k))];
    for (let turn = 0; !g.solved && turn < 40; turn++) {
        // prefer colors close to a close mark, then by how common they are
        const near = ranked.filter((k) => [...g.out].some(([o, m]) => m === "close" && isClose(o, k)));
        const pool = [...near, ...ranked.filter((k) => !near.includes(k))].filter((k) => canPick(g, k));
        if (!pool.length) break;
        let size = turn === 0 ? bot.opener : Math.max(g.lo, Math.min(bot.opener, g.found.length + 1));
        size = Math.min(size, g.hi, rules.maxPicks);
        const want = Math.max(1, size - g.found.length);
        g = guess(g, pool.slice(0, Math.min(want, rules.maxPicks - g.found.length)), rules);
    }
    return g;
}

// rulesKey names a rule set for saved results; the budget is applied after
// the game, so it isn't part of it.
export function rulesKey(r: Rules, cause: boolean): string {
    return `p${r.maxPicks}-${r.count}-${r.close}-${r.bands}-${cause ? "cause" : "blind"}`;
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
