// The lit windows of the web tower and its neighbors, through a night. Each
// date gets its own fixed night (a reload shows the same windows), and the
// windows follow people: offices empty out from 5pm, a cleaning crew works up
// the tower, homes light up in the evening and go dark toward midnight. Web
// only: the frame's picture doesn't change, so it keeps its own windows.
//
// Times are Boston minutes from noon on the night's date: 0 is noon, 540 is
// 9pm, 720 midnight, 960 the 4am rollover.

export const NOON = 0;
export const NINE_PM = 540;
const MORNING = -720; // lit since before the picture starts
const ROLLOVER = 960;

// noise is the frame's fixed pseudo-random number for a spot (render/art.go).
export function noise(x: number, y: number, seed: number): number {
    let h = (Math.imul(x, 374761393) + Math.imul(y, 668265263) + Math.imul(seed, -2048144777)) >>> 0;
    h = Math.imul(h ^ (h >>> 13), 1274126177) >>> 0;
    h = (h ^ (h >>> 16)) >>> 0;
    return (h % 10007) / 10007;
}

// seedOf turns a date into a number for noise
function seedOf(s: string): number {
    let h = 2166136261;
    for (let i = 0; i < s.length; i++) h = Math.imul(h ^ s.charCodeAt(i), 16777619);
    return (h >>> 0) % 100000;
}

export interface Win {
    x: number;
    y: number;
    w: number;
    h: number;
}

// Light is a window lit from on until off. group is its building: -1 the
// tower, else the index in Neighborhood.homes.
export interface Light extends Win {
    group: number;
    color: string;
    alpha: number;
    on: number;
    off: number;
}

// window light colors: the frame's inks, so a window is a solid block of ink
const WHITE = "#eceae2";
const YELLOW = "#e3ce34";
const TV = "#2254ac";

// offices lights a tower of office floors (top floor first): suites of 3–10
// windows that empty out together, mostly 5:30–8pm with a tail to 11pm, a few
// lit all night, some vacant; and two cleaning crews working up the tower from
// 7pm, a floor at a time.
function offices(floors: Win[][], seed: number): Light[] {
    const out: Light[] = [];
    floors.forEach((row, f) => {
        for (let i = 0, s = 0; i < row.length; s++) {
            const n = 3 + Math.floor(noise(s, f, seed) * 8);
            const r = noise(s, f, seed + 1);
            const leave =
                r < 0.18 ? undefined : r > 0.95 ? ROLLOVER : 270 + 390 * noise(s, f, seed + 2) ** 1.8;
            const c = noise(s, f, seed + 3);
            const color = c < 0.7 ? WHITE : YELLOW;
            if (leave !== undefined)
                for (const w of row.slice(i, i + n)) {
                    if (noise(w.x, w.y, seed + 4) < 0.12) continue; // an empty desk
                    out.push({
                        ...w,
                        group: -1,
                        color,
                        alpha: 1,
                        on: MORNING,
                        // people leave one by one
                        off: Math.min(ROLLOVER, leave + 8 * noise(w.x, w.y, seed + 6)),
                    });
                }
            i += n;
        }
    });
    for (let c = 0; c < 2; c++) {
        let t = 420 + 25 * c + 20 * noise(c, 0, seed + 7);
        let f = floors.length - 1 - Math.floor(noise(c, 1, seed + 7) * floors.length * 0.3);
        for (let k = 0; t < 660 && f >= 0; k++) {
            const stay = 20 + 20 * noise(c, k, seed + 8);
            for (const w of floors[f])
                if (noise(w.x, w.y, seed + 9 + c) < 0.9)
                    out.push({ ...w, group: -1, color: WHITE, alpha: 1, on: t, off: t + stay });
            t += stay;
            f -= 1 + Math.floor(noise(c, k, seed + 10) * 3);
        }
    }
    return out;
}

// homes lights windows the way people at home do: on through the evening,
// off around 11pm, a few up late, now and then the blue of a TV. density is
// how many windows have anyone behind them.
function homes(wins: Win[], density: number, seed: number, group: number): Light[] {
    const out: Light[] = [];
    for (const w of wins) {
        if (noise(w.x, w.y, seed) >= density) continue;
        const late = noise(w.x, w.y, seed + 2) < 0.08;
        const t = noise(w.x, w.y, seed + 4);
        out.push({
            ...w,
            group,
            color: t < 0.1 ? TV : t < 0.3 ? WHITE : YELLOW,
            alpha: 1,
            on: 240 + 240 * noise(w.x, w.y, seed + 1),
            off: late ? 840 + 120 * noise(w.x, w.y, seed + 3) : 540 + 300 * noise(w.x, w.y, seed + 3) ** 1.3,
        });
    }
    return out;
}

// Plan is one night's lights and the moments any of them switch.
export interface Plan {
    lights: Light[];
    edges: number[];
}

export interface Neighborhood {
    // the tower's floors, top first
    floors: Win[][];
    // other buildings' windows, and how many have someone behind them
    homes: { wins: Win[]; density: number }[];
}

const plans = new Map<string, Plan>();

// planFor is a night's lights, worked out once per date.
export function planFor(night: string, hood: Neighborhood): Plan {
    let p = plans.get(night);
    if (p) return p;
    const seed = seedOf(night);
    const lights = [
        ...offices(hood.floors, seed),
        ...hood.homes.flatMap((h, i) => homes(h.wins, h.density, seed + 100 * (i + 1), i)),
    ];
    const edges = [...new Set(lights.flatMap((l) => [l.on, l.off]))]
        .filter((t) => t > MORNING && t < ROLLOVER)
        .sort((a, b) => a - b);
    p = { lights, edges };
    plans.set(night, p);
    return p;
}

// litAt is the lights on at a minute.
export function litAt(plan: Plan, minute: number): Light[] {
    return plan.lights.filter((l) => l.on <= minute && minute < l.off);
}

// nextChange is the first minute after this one when a light switches.
export function nextChange(plan: Plan, minute: number): number | undefined {
    let lo = 0;
    let hi = plan.edges.length;
    while (lo < hi) {
        const mid = (lo + hi) >> 1;
        if (plan.edges[mid] <= minute) lo = mid + 1;
        else hi = mid;
    }
    return plan.edges[lo];
}

const boston = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/New_York",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
});

// minuteOf is a moment as Boston minutes from noon on the night's date.
export function minuteOf(ms: number, night: string): number {
    const p = Object.fromEntries(boston.formatToParts(new Date(ms)).map((x) => [x.type, x.value]));
    const days = (Date.UTC(+p.year, +p.month - 1, +p.day) - Date.parse(`${night}T00:00:00Z`)) / 86_400_000;
    return days * 1440 + +p.hour * 60 + +p.minute + +p.second / 60 - 720;
}
