import type { Lighting } from "@/interfaces/schedule";
import { MAX_PICKS } from "@/lib/prudle";

// Practice Prudle (/prudle): past nights from the rest of the year, played
// in the browser, so the game is familiar before December. A night whose
// cause is coming up in December is never dealt: the family would see its
// colors before its game.

// FILLER are words too common in causes to tell two apart.
const FILLER = new Set([
    "a",
    "an",
    "and",
    "the",
    "of",
    "for",
    "to",
    "in",
    "at",
    "on",
    "inc",
    "co",
    "day",
    "night",
    "foundation",
    "association",
    "society",
    "center",
    "centre",
    "fund",
    "program",
    "programs",
    "new",
    "england",
    "boston",
    "massachusetts",
    "ma",
    "national",
    "international",
]);

// words is a cause as its telling words: lowercase, no punctuation, no filler.
export function words(cause: string): Set<string> {
    const plain = cause
        .toLowerCase()
        .normalize("NFKD")
        .replace(/[̀-ͯ]/g, "")
        .replace(/[’']/g, "")
        .replace(/[^a-z0-9]+/g, " ");
    return new Set(plain.split(" ").filter((w) => w && !FILLER.has(w)));
}

// sameCause is whether two causes look like the same one: one's telling
// words hold the other's, or most of them are shared. Typos and "/ Save the
// Harbor" don't hide a December cause.
export function sameCause(a: string, b: string): boolean {
    const x = words(a);
    const y = words(b);
    if (!x.size || !y.size) return false;
    const both = [...x].filter((w) => y.has(w)).length;
    return both === Math.min(x.size, y.size) || both / new Set([...x, ...y]).size >= 0.6;
}

// pool is the past nights worth playing: a cause, one to MAX_PICKS colors,
// not in December, and no cause December is about to show.
export function pool(past: Lighting[], december: Lighting[]): Lighting[] {
    const coming = december.filter((l) => l.cause).map((l) => l.cause);
    const seen = new Set<string>();
    return past.filter((l) => {
        const n = new Set(l.colors).size;
        if (!l.scheduled || !l.cause || n < 1 || n > MAX_PICKS) return false;
        if (l.date.slice(5, 7) === "12" || seen.has(l.date)) return false;
        seen.add(l.date);
        return !coming.some((c) => sameCause(c, l.cause));
    });
}

export type Mode = "easy" | "random";

// most is how many colors Easy start allows after done finished games: one
// for the first 3, up to two through the 8th, then any.
export const most = (done: number) => (done < 3 ? 1 : done < 8 ? 2 : MAX_PICKS);

// deal is the next night to play, from a shuffled order fixed by the seed,
// skipping played nights. Easy start keeps to most(done) colors while any
// such night is left. Once every night is played, the longest ago comes back.
export function deal(nights: Lighting[], s: Saved): Lighting | undefined {
    if (!nights.length) return undefined;
    const played = new Set(s.played);
    const open = shuffled(nights, s.seed).filter((l) => !played.has(l.date));
    const cap = s.mode === "easy" ? most(s.done) : MAX_PICKS;
    return (
        open.find((l) => new Set(l.colors).size <= cap) ??
        open[0] ??
        nights.find((l) => l.date === s.played[0]) ??
        nights[0]
    );
}

// shuffled is a seeded shuffle (mulberry32), so the order holds on reload.
function shuffled<T>(xs: T[], seed: number): T[] {
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

const STORE = "prulights.practice.v1";

export interface Saved {
    seed: number;
    played: string[]; // dates dealt and left (finished or skipped), oldest first
    done: number; // games finished: won, lost or shown (not skipped)
    mode: Mode;
}

export function load(): Saved {
    const fresh: Saved = { seed: Math.floor(Math.random() * 1e9) + 1, played: [], done: 0, mode: "easy" };
    try {
        const s = JSON.parse(localStorage.getItem(STORE) ?? "null");
        if (s && typeof s.seed === "number" && Array.isArray(s.played)) {
            return {
                ...fresh,
                seed: s.seed,
                played: s.played,
                done: typeof s.done === "number" ? s.done : s.played.length,
                mode: s.mode === "random" ? "random" : "easy",
            };
        }
    } catch {}
    return fresh;
}

export function save(s: Saved) {
    try {
        localStorage.setItem(STORE, JSON.stringify(s));
    } catch {}
}
