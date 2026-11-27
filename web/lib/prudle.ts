import type { ColorKey } from "@/interfaces/schedule";

// Prudle: guess the night's exact set of colors. A guess is a whole set; it
// wins only when it is the answer, no more and no fewer. Every color comes
// back on, close (same family, see CLOSE) or off, and the guess's size comes
// back against the night's count (fewer, more or exact), so the count is
// narrowed, never shown. Colors found on stay in the next guess, since
// leaving one out can never win. The server grades every real guess
// (backend/prudle); this file draws the board, checks a guess before it's
// sent, and grades practice games on past nights (guess, the same rules).

// backend/prudle's Tries and MaxPicks, for practice; a real game's come with it.
export const TRIES = 6;
export const MAX_PICKS = 3;

export type Mark = "hit" | "close" | "miss";
export type CountMark = "fewer" | "more" | "exact";

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

// GLYPH is each mark as a light: on, half on, off. Safe to share: it says
// nothing about which colors.
export const GLYPH: Record<Mark, string> = { hit: "●", close: "◐", miss: "○" };

const MARK_ORDER: Mark[] = ["hit", "close", "miss"];

// byMark orders a guess good news first: on, close, off.
export function byMark(t: Turn): Pick[] {
    return [...t.picks].sort((a, b) => MARK_ORDER.indexOf(a.mark) - MARK_ORDER.indexOf(b.mark));
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

// known is the count once a guess was its size.
export function known(turns: Turn[]): number | undefined {
    return turns.find((t) => t.count === "exact")?.size;
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
    return more && fewer < Infinity
        ? `More than ${more}, fewer than ${fewer}`
        : more
          ? `More than ${colors(more)}`
          : fewer < Infinity
            ? `Fewer than ${colors(fewer)}`
            : `1 to ${maxPicks} colors`;
}

// countMark is a turn's count in a few characters: "< 3", "> 1", "= 2".
export function countMark(t: Turn): string {
    return { fewer: "<", more: ">", exact: "=" }[t.count] + " " + t.size;
}

// countWords is a turn's count said plainly: "more than 2", "fewer than 3",
// "exactly 1".
export function countWords(t: Turn): string {
    return { fewer: "fewer than", more: "more than", exact: "exactly" }[t.count] + " " + t.size;
}

// marksLabel is a turn's lights in words, for screen readers: "1 on, 1 close".
export function marksLabel(t: Turn): string {
    const words: Record<Mark, string> = { hit: "on", close: "close", miss: "off" };
    return MARK_ORDER.map((m) => [m, t.picks.filter((p) => p.mark === m).length] as const)
        .filter(([, n]) => n > 0)
        .map(([m, n]) => `${n} ${words[m]}`)
        .join(", ");
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

// Game is a practice game: the answer is known, so guess grades here.
export interface Game {
    answer: ColorKey[];
    turns: Turn[];
    on: ColorKey[]; // found, in the order found
    out: Map<ColorKey, Mark>;
    won: boolean;
}

export const newGame = (answer: ColorKey[]): Game => ({
    answer: [...new Set(answer)],
    turns: [],
    on: [],
    out: new Map(),
    won: false,
});

export const lost = (g: Game) => !g.won && g.turns.length >= TRIES;

// guess grades picks (the new colors; found ones ride along) the way
// backend/prudle's Guess does, or returns the game unchanged when they aren't
// a guess.
export function guess(g: Game, picks: ColorKey[]): Game {
    if (lost(g) || guessError(g, picks, MAX_PICKS)) return g;
    const on = [...g.on];
    const out = new Map(g.out);
    const turn: Turn = {
        picks: g.on.map((key) => ({ key, mark: "hit", carried: true })),
        size: 0,
        count: "exact",
    };
    for (const key of picks) {
        const mark: Mark = g.answer.includes(key)
            ? "hit"
            : g.answer.some((a) => a !== key && CLOSE.some((f) => f.includes(a) && f.includes(key)))
              ? "close"
              : "miss";
        if (mark === "hit") on.push(key);
        else out.set(key, mark);
        turn.picks.push({ key, mark });
    }
    turn.size = turn.picks.length;
    const n = g.answer.length;
    turn.count = turn.size === n ? "exact" : turn.size > n ? "fewer" : "more";
    return { ...g, on, out, turns: [...g.turns, turn], won: turn.size === n && on.length === n };
}
