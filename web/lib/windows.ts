// The lit windows of the web tower and its neighbors, through a night. Each
// date gets its own fixed night (a reload shows the same windows), and the
// windows follow people: an office tower's tenants hold a few floors each and
// empty out zone by zone, some lights are left on until the building switches
// them off at 11, a cleaning crew works along a floor at a time; homes light a
// room at a time in the evening and go dark toward midnight. Weekends and
// holidays leave the offices nearly dark. Web only: the frame's picture doesn't
// change, so it keeps its own windows.
//
// The glass is dark and smoked, so a lit window is never full strength: an
// occupied office is BRIGHT, lights left on are DIM, and the glow of a lit floor
// through its dark windows is GLOW.
//
// Times are Boston minutes from noon on the night's date: 0 is noon, 540 is
// 9pm, 720 midnight, 960 the 4am rollover.

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

const BRIGHT = 0.8;
const DIM = 0.45;
const GLOW = 0.2;

// office light: a tenant's fluorescent white, a little cooler or warmer
const OFFICE = ["#dfe4e6", "#e6e4dc", "#d8dee6"];
// the observatory on the top floors: warm, open until 10
const OBSERVATORY = "#e8cb92";
// homes: lamps, a warm white, the rare TV
const LAMP = "#e6b96c";
const WARM = "#ece1c6";
const TV = "#8ea2c4";

// smooth is value noise: like noise, but neighbors are alike, so what it picks
// comes in patches about sx by sy cells.
function smooth(x: number, y: number, seed: number, sx: number, sy = sx): number {
    const fx = x / sx;
    const fy = y / sy;
    const x0 = Math.floor(fx);
    const y0 = Math.floor(fy);
    const ease = (t: number) => t * t * (3 - 2 * t);
    const tx = ease(fx - x0);
    const ty = ease(fy - y0);
    const a = noise(x0, y0, seed) + (noise(x0 + 1, y0, seed) - noise(x0, y0, seed)) * tx;
    const b = noise(x0, y0 + 1, seed) + (noise(x0 + 1, y0 + 1, seed) - noise(x0, y0 + 1, seed)) * tx;
    return a + (b - a) * ty;
}

// Day is what kind of day the night follows: offices are busy on weekdays,
// leave early on Fridays, and are nearly empty on weekends and holidays.
type Day = "weekday" | "friday" | "off";

export function dayOf(night: string): Day {
    const [y, m, d] = night.split("-").map(Number);
    if (!y || !m || !d) return "weekday";
    const dow = new Date(Date.UTC(y, m - 1, d)).getUTCDay();
    if (dow === 0 || dow === 6) return "off";
    if (["1-1", "6-19", "7-4", "11-11", "12-24", "12-25", "12-31"].includes(`${m}-${d}`)) return "off";
    // which of the month's Mondays (or Thursdays…) it is, and whether it's the last
    const nth = Math.ceil(d / 7);
    const last = d + 7 > new Date(Date.UTC(y, m, 0)).getUTCDate();
    if (
        dow === 1 &&
        ((nth === 3 && [1, 2, 4].includes(m)) ||
            (m === 9 && nth === 1) ||
            (m === 10 && nth === 2) ||
            (m === 5 && last))
    )
        return "off";
    // Thanksgiving and the day after
    if (m === 11 && ((dow === 4 && nth === 4) || (dow === 5 && Math.ceil((d - 1) / 7) === 4))) return "off";
    return dow === 5 ? "friday" : "weekday";
}

// zonesOf splits a floor into zones of about size windows, the edges a little
// ragged so the zones don't line up floor to floor.
function zonesOf(row: Win[], size: number, f: number, seed: number): Win[][] {
    const out: Win[][] = [];
    let start = 0;
    while (start < row.length) {
        const n = Math.max(2, size + Math.round((noise(start, f, seed) - 0.5) * size * 0.6));
        out.push(row.slice(start, start + n));
        start += n;
    }
    return out;
}

// leaving is when a tenant's people mostly go home, or undefined for a tenant
// who isn't in (vacant floors, a weekend).
function leaving(day: Day, r: (k: number) => number): number | undefined {
    if (r(0) < 0.12) return undefined; // floors to let
    if (day === "off") return r(1) < 0.12 ? 300 + 180 * r(2) : undefined;
    const early = day === "friday" ? 30 : 0;
    const late = r(3);
    if (late < 0.03) return ROLLOVER; // the trading desk, the data center
    if (late < (day === "friday" ? 0.08 : 0.14)) return 480 + 180 * r(4) - early;
    return 270 + 200 * r(5) ** 1.6 - early;
}

// officeFloors lights floors of offices (top first). tenants hold a few floors
// each; their zones empty out in patches around the tenant's time, a quarter
// of zones leave their lights on until 11, and while anyone is on a floor its
// dark windows glow faintly.
function officeFloors(
    floors: Win[][],
    day: Day,
    seed: number,
    group: number,
    zone = 7,
    strays = STRAYS,
): Light[] {
    const out: Light[] = [];
    let tenant = 0;
    let span = 0;
    let leave: number | undefined;
    let color = OFFICE[0];
    floors.forEach((row, f) => {
        if (span-- <= 0) {
            tenant++;
            span = 1 + Math.floor(noise(tenant, 0, seed) * 5);
            leave = leaving(day, (k) => noise(tenant, k + 1, seed));
            color = OFFICE[Math.floor(noise(tenant, 9, seed) * OFFICE.length)];
        }
        out.push(...stray(row, f, strays, seed + 11, group, color));
        if (leave === undefined) return;
        const zones = zonesOf(row, zone, f, seed + 1);
        // each zone: when its people leave, and when its lights finally go off
        const ends = zones.map((_, z) => {
            const off =
                leave === ROLLOVER
                    ? ROLLOVER
                    : Math.min(ROLLOVER, leave! + 180 * (smooth(z, f, seed + 2, 1.6, 4) - 0.5));
            // lights left on come in patches too: one team's corner
            const left = smooth(z, f, seed + 3, 1.5, 3) - 0.2;
            const lights =
                left < 0.04 ? ROLLOVER : left < 0.28 ? Math.max(off, 660 + 40 * noise(z, f, seed + 4)) : off;
            return { off, lights };
        });
        const floorOff = Math.max(...ends.map((e) => e.off));
        zones.forEach((wins, z) => {
            const { off, lights } = ends[z];
            for (const w of wins) {
                const n = noise(w.x, w.y, seed + 5);
                // people leave one by one; an empty desk is only dim
                const out1 = Math.min(ROLLOVER, off + 10 * noise(w.x, w.y, seed + 6));
                out.push({ ...w, group, color, alpha: n < 0.1 ? DIM : BRIGHT, on: MORNING, off: out1 });
                if (lights > out1) out.push({ ...w, group, color, alpha: DIM, on: out1, off: lights });
                const dark = Math.max(out1, lights);
                if (floorOff > dark && n > 0.35)
                    out.push({ ...w, group, color, alpha: GLOW, on: dark, off: floorOff });
            }
        });
    });
    return out;
}

// Strays are not realistic, just for the eye: now and then a run of a few
// windows lit late on a floor that's otherwise dark, so big empty stretches
// get a little life. chance is per floor, most is the longest run.
interface Strays {
    chance: number;
    most: number;
}
const STRAYS: Strays = { chance: 0.06, most: 3 };
// the skyline's buildings get more of them, and shorter
const SKYLINE_STRAYS: Strays = { chance: 0.14, most: 2 };

function stray(
    row: Win[],
    f: number,
    { chance, most }: Strays,
    seed: number,
    group: number,
    color: string,
): Light[] {
    if (row.length === 0 || noise(f, 0, seed) >= chance) return [];
    const n = 1 + Math.floor(noise(f, 1, seed) * most);
    const at = Math.floor(noise(f, 2, seed) * Math.max(1, row.length - n));
    const off = 600 + 300 * noise(f, 3, seed);
    const alpha = noise(f, 4, seed) < 0.5 ? DIM : BRIGHT;
    return row.slice(at, at + n).map((w) => ({ ...w, group, color, alpha, on: MORNING, off }));
}

// tower is the Prudential's floors (top first): the observatory on top, offices
// below, and cleaning crews from 7pm, each lighting a patch that moves along a
// floor, then up a floor or two.
function tower(floors: Win[][], day: Day, seed: number): Light[] {
    const TOP = 2;
    const out: Light[] = [];
    for (const row of floors.slice(0, TOP))
        for (const w of row)
            if (noise(w.x, w.y, seed + 20) < 0.8)
                out.push({
                    ...w,
                    group: -1,
                    color: OBSERVATORY,
                    alpha: DIM,
                    on: MORNING,
                    off: 600 + 10 * noise(w.x, w.y, seed + 21),
                });
    out.push(...officeFloors(floors.slice(TOP), day, seed, -1));
    const crews = day === "off" ? 1 : 2;
    for (let c = 0; c < crews; c++) {
        let t = 420 + 30 * c + 20 * noise(c, 0, seed + 7);
        let f = floors.length - 1 - Math.floor(noise(c, 1, seed + 7) * floors.length * 0.3);
        for (let k = 0; t < 690 && f >= TOP; k++) {
            const row = floors[f];
            // half a floor, from one end or the other, a window a minute or so
            const half = Math.floor(row.length / 2);
            const from = noise(c, k, seed + 8) < 0.5 ? 0 : half;
            const step = 1.2 + 0.6 * noise(c, k, seed + 9);
            for (let i = 0; i < half; i++) {
                const w = row[from + i];
                const on = t + i * step;
                out.push({ ...w, group: -1, color: OFFICE[0], alpha: BRIGHT, on, off: on + 9 });
            }
            t += half * step + 6;
            f -= 1 + Math.floor(noise(c, k, seed + 10) * 2);
        }
    }
    return out;
}

// roomsOf groups a building's windows (row by row) into rooms of 2–3 windows.
function roomsOf(wins: Win[], seed: number): Win[][] {
    const out: Win[][] = [];
    let cur: Win[] = [];
    let size = 2;
    for (const w of wins) {
        const prev = cur[cur.length - 1];
        if (prev && (prev.y !== w.y || cur.length >= size || w.x - prev.x > 6)) {
            out.push(cur);
            cur = [];
        }
        if (!cur.length) size = noise(w.x, w.y, seed) < 0.4 ? 3 : 2;
        cur.push(w);
    }
    if (cur.length) out.push(cur);
    return out;
}

// homes lights rooms the way people at home do: the living room on from dusk
// to 9:30–11:30, a few up late, and as they go dark some bedrooms come on for
// an hour. Curtains make some rooms dimmer; a TV is rare. Holidays and
// weekends are busier and later.
function homes(wins: Win[], density: number, day: Day, seed: number, group: number): Light[] {
    const out: Light[] = [];
    const busy = day === "off" ? 1.25 : 1;
    const later = day === "off" ? 30 : 0;
    for (const room of roomsOf(wins, seed)) {
        const { x, y } = room[0];
        const r = noise(x, y, seed + 1);
        let on: number;
        let off: number;
        if (r < density * busy) {
            on = 240 + 210 * noise(x, y, seed + 2) ** 1.5;
            off =
                noise(x, y, seed + 3) < 0.15
                    ? 690 + 180 * noise(x, y, seed + 4)
                    : 570 + 120 * noise(x, y, seed + 4) + later;
        } else if (r < density * busy * 1.5) {
            // a bedroom, at bedtime
            on = 570 + 90 * noise(x, y, seed + 2) + later;
            off = on + 30 + 60 * noise(x, y, seed + 4);
        } else continue;
        const t = noise(x, y, seed + 5);
        const color = t < 0.05 ? TV : t < 0.3 ? WARM : LAMP;
        const alpha = t < 0.05 ? GLOW : noise(x, y, seed + 6) < 0.4 ? DIM : BRIGHT;
        for (const w of room) {
            if (noise(w.x, w.y, seed + 7) < 0.15) continue; // a blind down
            out.push({ ...w, group, color, alpha, on, off: off + 4 * noise(w.x, w.y, seed + 8) });
        }
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
    // other buildings: their windows, offices or homes, and for homes how
    // many rooms have someone in them
    homes: { wins: Win[]; offices?: boolean; density: number }[];
}

const plans = new Map<string, Plan>();

// planFor is a night's lights, worked out once per date.
export function planFor(night: string, hood: Neighborhood): Plan {
    let p = plans.get(night);
    if (p) return p;
    const seed = seedOf(night);
    const day = dayOf(night);
    const lights = [
        ...tower(hood.floors, day, seed),
        ...hood.homes.flatMap((h, i) =>
            h.offices
                ? officeFloors(floorsOf(h.wins), day, seed + 100 * (i + 1), i, 4, SKYLINE_STRAYS)
                : homes(h.wins, h.density, day, seed + 100 * (i + 1), i),
        ),
    ];
    const edges = [...new Set(lights.flatMap((l) => [l.on, l.off]))]
        .filter((t) => t > MORNING && t < ROLLOVER)
        .sort((a, b) => a - b);
    p = { lights, edges };
    plans.set(night, p);
    return p;
}

// floorsOf groups windows into floors by their row, top first.
function floorsOf(wins: Win[]): Win[][] {
    const rows = new Map<number, Win[]>();
    for (const w of wins) rows.set(w.y, [...(rows.get(w.y) ?? []), w]);
    return [...rows.keys()].sort((a, b) => a - b).map((y) => rows.get(y)!);
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
