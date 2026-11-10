"use client";

import { useId, useMemo } from "react";

// Tower draws the Prudential Tower over the Back Bay skyline, its crown lit in
// the night's colors where the real lights are: the spire, the thin ring at
// the roofline, the thick ring under the PRUDENTIAL letters, and the setback.
// The geometry is the frame's pixel art (backend/render/art.go) in the same
// units, so the web and the frame show the same building.

export type Sky = "night" | "dusk" | "day";

interface TowerProps {
    bands: string[];
    className?: string;
    // the sky behind it; the tonight page follows the clock
    sky?: Sky;
    // the skyline around the tower; off, the tower stands alone
    skyline?: boolean;
    title?: string;
}

// Geometry, from the spire's tip, x from the center line (art.go).
const W = 480;
const H = 760;
const CX = 240;
const TOP = 30;
const TOWER_HALF = 84;
const CROWN_HALF = 76;
const RING_TOP = 112;
const BAND_TOP = 120;
const RING_LOW = 138;
const SETBACK_TOP = 150;
const SHAFT_TOP = 162;
const FLOOR = 9;

// noise is the frame's fixed pseudo-random number for a spot (art.go).
function noise(x: number, y: number, seed: number): number {
    let h = (Math.imul(x, 374761393) + Math.imul(y, 668265263) + Math.imul(seed, -2048144777)) >>> 0;
    h = Math.imul(h ^ (h >>> 13), 1274126177) >>> 0;
    h = (h ^ (h >>> 16)) >>> 0;
    return (h % 10007) / 10007;
}

// windows is a path of lit windows in a box: w×h each, on a pitch.
function windows(
    x0: number,
    y0: number,
    x1: number,
    y1: number,
    chance: number,
    seed: number,
    w = 2,
    h = 3,
    px = 5,
    py = 7,
): string {
    let d = "";
    for (let y = y0; y + h <= y1; y += py)
        for (let x = x0; x + w <= x1; x += px)
            if (noise(Math.round(x), Math.round(y), seed) < chance) d += `M${x} ${y}h${w}v${h}h${-w}z`;
    return d;
}

const SKIES: Record<
    Sky,
    {
        stops: [string, string, string];
        far: string;
        near: string;
        glass: [string, string];
        glow: number;
        lit: number;
        unlit: string;
    }
> = {
    night: {
        stops: ["#02040a", "#070c1c", "#16224a"],
        far: "#070a14",
        near: "#03050a",
        glass: ["#0c1631", "#1b2f5c"],
        glow: 1,
        lit: 0.1,
        unlit: "#18213a",
    },
    dusk: {
        stops: ["#151c3d", "#4a3f6b", "#e0895c"],
        far: "#151726",
        near: "#0b0c16",
        glass: ["#3b4374", "#d08a63"],
        glow: 0.7,
        lit: 0.14,
        unlit: "#2a2f4c",
    },
    day: {
        stops: ["#5d8fc6", "#9dbedd", "#e2ebf2"],
        far: "#4a5a75",
        near: "#2e3747",
        glass: ["#8db2d8", "#d2e2f0"],
        glow: 0.25,
        lit: 0.02,
        unlit: "#56698a",
    },
};

export function Tower({ bands, className, sky = "night", skyline = true, title }: TowerProps) {
    const id = useId().replace(/:/g, "");
    const s = SKIES[sky];
    const n = bands.length || 1;
    const first = bands[0] ?? "#ffffff";
    const horizon = H;
    const span = horizon - (TOP + RING_TOP);
    const topOf = (f: number) => horizon - f * span;

    // the drawing that doesn't depend on the colors, worked out once
    const art = useMemo(() => {
        let lit = "";
        let warm = "";
        const mech: number[] = [];
        for (let f = 0, y = TOP + SHAFT_TOP + 5; y < H; f++, y += FLOOR) {
            if (f % 14 === 13) {
                mech.push(y - 2);
                continue;
            }
            for (let x = -TOWER_HALF + 4; x + 2 <= TOWER_HALF - 3; x += 4) {
                const v = noise(x, f, 3);
                if (v < s.lit * 0.7) lit += `M${CX + x} ${y}h2v5h-2z`;
                else if (v < s.lit) warm += `M${CX + x} ${y}h2v5h-2z`;
            }
        }
        // brownstones: a ragged run of roofs and chimneys
        let roofs = `M0 ${H}`;
        let lamps = "";
        for (let x = 0, i = 0; x < W; i++) {
            const w = 24 + Math.floor(noise(i, 0, 41) * 30);
            const h = 20 + Math.floor(noise(i, 1, 41) * 34);
            roofs += `V${H - h}`;
            if (noise(i, 2, 41) < 0.5) {
                const c = x + 4 + Math.floor(noise(i, 3, 41) * (w - 10));
                roofs += `H${c}v-7h4v7`;
            }
            roofs += `H${x + w}`;
            lamps += windows(x + 3, H - h + 6, x + w - 3, H - 2, 0.16, 50 + i);
            x += w;
        }
        roofs += `V${H}Z`;
        return { lit, warm, mech, roofs, lamps };
    }, [s.lit]);

    const ring = (y0: number, y1: number, half: number, opacity = 1) =>
        bands.map((c, i) => {
            const x0 = CX - half + ((2 * half) / n) * i;
            return (
                <rect
                    key={i}
                    x={x0}
                    y={TOP + y0}
                    width={(2 * half) / n + 0.4}
                    height={y1 - y0}
                    fill={c}
                    opacity={opacity}
                />
            );
        });

    const hancock = { x0: CX - 232, x1: CX - 156, top: topOf(0.74) };
    const dome = { x0: CX - 152, x1: CX - 98, top: topOf(0.46) };
    const domeMid = (dome.x0 + dome.x1) / 2;
    const domeR = (dome.x1 - dome.x0) / 2 - 4;
    const neighbors: [number, number, number][] = [
        [-246, -176, 0.3],
        [120, 182, 0.38],
        [176, 246, 0.27],
        [86, 132, 0.2],
    ];

    return (
        <svg
            viewBox={`0 0 ${W} ${H}`}
            className={className}
            role="img"
            aria-label={title ?? "The Prudential Tower"}
            preserveAspectRatio="xMidYMin slice"
        >
            <defs>
                <linearGradient id={`sky-${id}`} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0" stopColor={s.stops[0]} />
                    <stop offset="0.55" stopColor={s.stops[1]} />
                    <stop offset="1" stopColor={s.stops[2]} />
                </linearGradient>
                <radialGradient id={`halo-${id}`}>
                    <stop offset="0" stopColor={first} stopOpacity={0.55 * s.glow} />
                    <stop offset="0.45" stopColor={first} stopOpacity={0.18 * s.glow} />
                    <stop offset="1" stopColor={first} stopOpacity="0" />
                </radialGradient>
                <linearGradient id={`glass-${id}`} x1="0" y1="0" x2="1" y2="1">
                    <stop offset="0" stopColor={s.glass[0]} />
                    <stop offset="1" stopColor={s.glass[1]} />
                </linearGradient>
                <linearGradient id={`spill-${id}`} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0" stopColor={first} stopOpacity={0.5 * s.glow} />
                    <stop offset="1" stopColor={first} stopOpacity="0" />
                </linearGradient>
                <pattern
                    id={`win-${id}`}
                    width="4"
                    height={FLOOR}
                    patternUnits="userSpaceOnUse"
                    x={CX - TOWER_HALF + 4}
                    y={TOP + SHAFT_TOP + 5}
                >
                    <rect width="2" height="5" fill={s.unlit} />
                </pattern>
                <filter id={`bloom-${id}`} x="-50%" y="-200%" width="200%" height="500%">
                    <feGaussianBlur stdDeviation="2.5" result="b" />
                    <feMerge>
                        <feMergeNode in="b" />
                        <feMergeNode in="SourceGraphic" />
                    </feMerge>
                </filter>
            </defs>

            <rect width={W} height={H} fill={`url(#sky-${id})`} />
            {sky === "night" &&
                [
                    [38, 52],
                    [92, 128],
                    [61, 214],
                    [140, 40],
                    [22, 300],
                    [118, 262],
                    [350, 70],
                    [412, 34],
                    [438, 150],
                    [376, 196],
                    [452, 248],
                    [330, 118],
                    [406, 312],
                    [70, 380],
                ].map(([x, y], i) => (
                    <circle
                        key={i}
                        cx={x}
                        cy={y}
                        r={(x + y) % 3 === 0 ? 1.2 : 0.7}
                        fill="#dfe6ff"
                        opacity={0.8}
                    />
                ))}
            <ellipse cx={CX} cy={TOP + RING_LOW} rx={190} ry={110} fill={`url(#halo-${id})`} />

            {skyline && (
                <g>
                    {/* the Hancock's glass slab and its notch */}
                    <rect
                        x={hancock.x0}
                        y={hancock.top}
                        width={hancock.x1 - hancock.x0}
                        height={H}
                        fill={`url(#glass-${id})`}
                    />
                    <rect
                        x={hancock.x0 + 22}
                        y={hancock.top}
                        width={3}
                        height={H}
                        fill={s.far}
                        opacity={0.8}
                    />
                    {/* a dark tower behind the Pru's right side */}
                    <rect x={CX + 70} y={topOf(0.6)} width={72} height={H} fill={s.far} />
                    <path
                        d={windows(CX + 86, topOf(0.6) + 6, CX + 142, H, 0.05, 11)}
                        fill="#ffe2a0"
                        opacity={0.8}
                    />
                </g>
            )}

            {/* the spire: a whip, a lattice mast lit in the first color, its legs */}
            <g
                stroke={first}
                strokeWidth="1.2"
                fill="none"
                filter={`url(#bloom-${id})`}
                opacity={0.6 + 0.4 * s.glow}
            >
                <line x1={CX} y1={TOP + 2} x2={CX} y2={TOP + 30} strokeWidth="2" />
                <polyline points={`${CX - 3},${TOP + 30} ${CX - 8},${TOP + 98} ${CX - 16},${TOP + 106}`} />
                <polyline points={`${CX + 3},${TOP + 30} ${CX + 8},${TOP + 98} ${CX + 16},${TOP + 106}`} />
                {Array.from({ length: 8 }, (_, i) => {
                    const y = 30 + i * 8.5;
                    const a = 3 + ((y - 30) * 5) / 68;
                    const b = 3 + ((y + 8.5 - 30) * 5) / 68;
                    return (
                        <path
                            key={i}
                            d={`M${CX - a} ${TOP + y}L${CX + b} ${TOP + y + 8.5}M${CX + a} ${TOP + y}L${CX - b} ${TOP + y + 8.5}`}
                        />
                    );
                })}
                <rect x={CX - 8} y={TOP + 58} width={16} height={16} fill={first} fillOpacity={0.35} />
            </g>
            <circle cx={CX} cy={TOP} r={2.2} fill="#ff3b30" filter={`url(#bloom-${id})`} />

            {/* the rooftop: machinery, antennas, beacons */}
            <rect x={CX - 58} y={TOP + 104} width={116} height={8} fill={s.near} />
            <g stroke="#5b6680" strokeWidth="1">
                {[
                    [-48, 84],
                    [-36, 92],
                    [-20, 96],
                    [30, 90],
                    [46, 80],
                    [54, 94],
                ].map(([x, y]) => (
                    <line key={x} x1={CX + x} y1={TOP + y} x2={CX + x} y2={TOP + 104} />
                ))}
            </g>
            <circle cx={CX - 59} cy={TOP + 103} r={1.4} fill="#ff3b30" />
            <circle cx={CX + 59} cy={TOP + 103} r={1.4} fill="#ff3b30" />

            {/* the shaft, its windows, its mechanical floors, the crown's light on it */}
            <rect x={CX - TOWER_HALF} y={TOP + SHAFT_TOP} width={2 * TOWER_HALF} height={H} fill={s.near} />
            <rect
                x={CX - TOWER_HALF + 4}
                y={TOP + SHAFT_TOP + 5}
                width={2 * TOWER_HALF - 7}
                height={H}
                fill={`url(#win-${id})`}
            />
            {art.mech.map((y) => (
                <rect key={y} x={CX - TOWER_HALF} y={y} width={2 * TOWER_HALF} height={FLOOR} fill={s.near} />
            ))}
            <path d={art.lit} fill="#fff4dc" />
            <path d={art.warm} fill="#ffd27a" />
            <rect
                x={CX - TOWER_HALF}
                y={TOP + SHAFT_TOP}
                width={2 * TOWER_HALF}
                height={40}
                fill={`url(#spill-${id})`}
            />
            <rect
                x={CX - TOWER_HALF}
                y={TOP + SHAFT_TOP}
                width={1}
                height={H}
                fill="#ffffff"
                opacity={0.12}
            />
            <rect
                x={CX + TOWER_HALF - 1}
                y={TOP + SHAFT_TOP}
                width={1}
                height={H}
                fill="#ffffff"
                opacity={0.12}
            />

            {/* the crown: rings of light, the lettered band, the lit setback */}
            <rect
                x={CX - 68}
                y={TOP + SETBACK_TOP}
                width={136}
                height={SHAFT_TOP - SETBACK_TOP}
                fill={s.near}
            />
            {ring(SETBACK_TOP, SHAFT_TOP, 66, 0.55)}
            <rect
                x={CX - CROWN_HALF + 2}
                y={TOP + BAND_TOP}
                width={2 * CROWN_HALF - 4}
                height={RING_LOW - BAND_TOP}
                fill="#080a12"
            />
            <path
                d={Array.from({ length: 37 }, (_, i) => -CROWN_HALF + 4 + i * 4)
                    .filter((x) => x < -54 || x > 52)
                    .map((x) => `M${CX + x} ${TOP + BAND_TOP + 4}h2v10h-2z`)
                    .join("")}
                fill="#e9d9a8"
                opacity={0.75}
            />
            <text
                x={CX}
                y={TOP + BAND_TOP + 13.5}
                textAnchor="middle"
                fontFamily="Georgia, 'Times New Roman', serif"
                fontWeight="700"
                fontSize="12.5"
                letterSpacing="1.4"
                fill="#fbf7ea"
                filter={`url(#bloom-${id})`}
            >
                PRUDENTIAL
            </text>
            <g filter={`url(#bloom-${id})`}>
                {ring(RING_TOP, BAND_TOP, CROWN_HALF)}
                {ring(RING_LOW, SETBACK_TOP, CROWN_HALF + 2)}
            </g>

            {skyline && (
                <g>
                    {neighbors.map(([x0, x1, f], i) => (
                        <g key={i}>
                            <rect x={CX + x0} y={topOf(f)} width={x1 - x0} height={H} fill={s.near} />
                            <path
                                d={windows(CX + x0 + 4, topOf(f) + 4, CX + x1 - 4, H, 0.09, 20 + i)}
                                fill="#ffdc96"
                                opacity={0.85}
                            />
                        </g>
                    ))}
                    {/* 111 Huntington: a round-shouldered tower under a ribbed dome */}
                    <rect x={dome.x0} y={dome.top} width={dome.x1 - dome.x0} height={H} fill={s.near} />
                    <path
                        d={windows(dome.x0 + 4, dome.top + 8, dome.x1 - 4, H, 0.14, 31)}
                        fill="#ffdc96"
                        opacity={0.85}
                    />
                    <rect
                        x={dome.x0 + 2}
                        y={dome.top - 4}
                        width={dome.x1 - dome.x0 - 4}
                        height={4}
                        fill={s.near}
                    />
                    <path
                        d={`M${domeMid - domeR} ${dome.top - 4}A${domeR} 24 0 0 1 ${domeMid + domeR} ${dome.top - 4}Z`}
                        fill={s.near}
                    />
                    <g stroke="#e8dcc0" strokeWidth="0.9" opacity={sky === "day" ? 0.35 : 0.75} fill="none">
                        <path
                            d={`M${domeMid - domeR} ${dome.top - 4}A${domeR} 24 0 0 1 ${domeMid + domeR} ${dome.top - 4}`}
                        />
                        {[-0.8, -0.45, 0, 0.45, 0.8].map((f) => (
                            <line
                                key={f}
                                x1={domeMid + f * domeR}
                                y1={dome.top - 4 - 24 * Math.sqrt(1 - f * f)}
                                x2={domeMid + f * domeR * 0.9}
                                y2={dome.top - 4}
                            />
                        ))}
                        <line x1={domeMid - domeR} y1={dome.top - 5} x2={domeMid + domeR} y2={dome.top - 5} />
                        <line x1={domeMid} y1={dome.top - 40} x2={domeMid} y2={dome.top - 28} />
                    </g>
                    {/* brownstones in front */}
                    <path d={art.roofs} fill={s.near} />
                    <path d={art.lamps} fill="#ffd88a" opacity={0.9} />
                </g>
            )}
        </svg>
    );
}

// Crown is a night's colors as a small crown and spire, for lists and
// calendars: the rings carry the colors, as on the frame.
export function Crown({ bands, className }: { bands: string[]; className?: string }) {
    const n = bands.length || 1;
    const ring = (y: number, h: number, x0: number, w: number, opacity = 1) =>
        bands.map((c, i) => (
            <rect
                key={i}
                x={x0 + (w / n) * i}
                y={y}
                width={w / n + 0.2}
                height={h}
                fill={c}
                opacity={opacity}
            />
        ));
    return (
        <svg viewBox="0 0 40 44" className={`shrink-0 ${className ?? "h-8 w-8"}`} aria-hidden>
            <rect x="19.2" y="0" width="1.6" height="13" fill={bands[0]} />
            <rect x="9" y="11" width="22" height="4" fill="var(--color-tower)" />
            <rect x="2" y="15" width="36" height="5" rx="0.5" fill="var(--color-tower)" />
            {ring(15, 5, 2, 36)}
            <rect x="3" y="20" width="34" height="5" fill="var(--color-tower)" />
            {ring(25, 7, 1, 38)}
            <rect x="5" y="32" width="30" height="3" fill="var(--color-tower)" />
            {ring(32, 3, 5, 30, 0.55)}
            <rect x="0" y="35" width="40" height="9" fill="var(--color-tower)" />
        </svg>
    );
}
