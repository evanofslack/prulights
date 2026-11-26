"use client";

import { useEffect, useId, useMemo, useState } from "react";
import { soft } from "@/lib/palette";
import { type Stop, banded, mute } from "@/lib/print";
import {
    type Light,
    type Neighborhood,
    type Plan,
    type Win,
    NINE_PM,
    litAt,
    minuteOf,
    nextChange,
    noise,
    planFor,
} from "@/lib/windows";

// Tower draws the Prudential Tower over the Back Bay skyline, its crown lit in
// the night's colors where the real lights are: the spire, the thin ring at
// the roofline, the thick ring under the PRUDENTIAL letters, and the setback.
// The geometry is the frame's pixel art (backend/render/art.go) in the same
// units, so the web and the frame show the same building.
//
// It's a sharp svg with a printed look (lib/print.ts): flat colors from the
// frame's inks, gradients in hard bands, a light halftone in the sky and the
// crown's glow, and paper grain over everything. The windows follow the night
// (lib/windows.ts): tonight's switch on and off with the clock, other nights
// are shown at 9pm.

export type Sky = "night" | "dusk" | "day";

interface TowerProps {
    bands: string[];
    className?: string;
    // the sky behind it; the tonight page follows the clock
    sky?: Sky;
    // the skyline around the tower; off, the tower stands alone
    skyline?: boolean;
    title?: string;
    // the night (a date), which picks its windows
    night?: string;
    // follow the clock: the windows switch as the night goes on
    live?: boolean;
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

// the frame's red ink, for the aircraft lights
const BEACON_RED = "#b83422";

const topOf = (f: number) => H - f * (H - (TOP + RING_TOP));

// grid is the windows in a box: w×h each, on a pitch.
function grid(x0: number, y0: number, x1: number, y1: number, w = 2, h = 3, px = 5, py = 7): Win[] {
    const out: Win[] = [];
    for (let y = y0; y + h <= y1; y += py) for (let x = x0; x + w <= x1; x += px) out.push({ x, y, w, h });
    return out;
}

// the neighbors: [x0, x1] from the center line, and height as a share of the tower
const NEIGHBORS: [number, number, number][] = [
    [-246, -176, 0.3],
    [120, 182, 0.38],
    [176, 246, 0.27],
    [86, 132, 0.2],
];
const HANCOCK = { x0: CX - 232, x1: CX - 156, top: topOf(0.74) };
const DOME = { x0: CX - 152, x1: CX - 98, top: topOf(0.46) };
const DOME_MID = (DOME.x0 + DOME.x1) / 2;
const DOME_R = (DOME.x1 - DOME.x0) / 2 - 4;
const DARK = { x0: CX + 70, x1: CX + 142, top: topOf(0.6) };

// The drawing that never changes, worked out once: the tower's floors and
// mechanical bands, the brownstones' roofs, every window, the stars.
const ART = (() => {
    const floors: Win[][] = [];
    const mech: number[] = [];
    for (let f = 0, y = TOP + SHAFT_TOP + 5; y < H; f++, y += FLOOR) {
        if (f % 14 === 13) {
            mech.push(y - 2);
            continue;
        }
        const row: Win[] = [];
        for (let x = -TOWER_HALF + 4; x + 2 <= TOWER_HALF - 3; x += 4) row.push({ x: CX + x, y, w: 2, h: 5 });
        floors.push(row);
    }
    // brownstones: a ragged run of roofs and chimneys
    let roofs = `M0 ${H}`;
    const brownstones: Win[] = [];
    for (let x = 0, i = 0; x < W; i++) {
        const w = 24 + Math.floor(noise(i, 0, 41) * 30);
        const h = 20 + Math.floor(noise(i, 1, 41) * 34);
        roofs += `V${H - h}`;
        if (noise(i, 2, 41) < 0.5) {
            const c = x + 4 + Math.floor(noise(i, 3, 41) * (w - 10));
            roofs += `H${c}v-7h4v7`;
        }
        roofs += `H${x + w}`;
        brownstones.push(...grid(x + 3, H - h + 6, x + w - 3, H - 2));
        x += w;
    }
    roofs += `V${H}Z`;
    const hood: Neighborhood = {
        floors,
        homes: [
            { wins: grid(DARK.x0 + 16, DARK.top + 6, DARK.x1, H), density: 0.14 },
            ...NEIGHBORS.map(([x0, x1, f]) => ({
                wins: grid(CX + x0 + 4, topOf(f) + 4, CX + x1 - 4, H),
                density: 0.3,
            })),
            { wins: grid(DOME.x0 + 4, DOME.top + 8, DOME.x1 - 4, H), density: 0.36 },
            { wins: brownstones, density: 0.5 },
        ],
    };
    // stars, more and less bright, in the upper sky
    const stars = Array.from({ length: 48 }, (_, i) => ({
        x: noise(i, 0, 71) * W,
        y: noise(i, 1, 71) * H * 0.5,
        r: 0.5 + 0.9 * noise(i, 2, 71) ** 2,
        o: 0.35 + 0.6 * noise(i, 3, 71),
    }));
    return { mech, roofs, hood, stars };
})();

const SKIES: Record<
    Sky,
    {
        stops: [string, string, string];
        far: string;
        near: string;
        // the tower's face, top to bottom
        shaft: [string, string];
        glass: [string, string];
        // the city's glow low in the sky
        city: [string, number];
        glow: number;
        // how strongly lit windows show
        windows: number;
        unlit: string;
        // the halftone in the sky: its ink and how strong it is at the top and the horizon
        dots: [string, number, number];
    }
> = {
    night: {
        stops: ["#02040a", "#070c1c", "#16224a"],
        far: "#070a14",
        near: "#03050a",
        shaft: ["#1f2a52", "#0a0f22"],
        glass: ["#0c1631", "#1b2f5c"],
        city: ["#d98a4a", 0.3],
        glow: 1,
        windows: 1,
        unlit: "#2e3d6e",
        dots: ["#2254ac", 0.1, 0.5],
    },
    dusk: {
        stops: ["#151c3d", "#4a3f6b", "#e0895c"],
        far: "#151726",
        near: "#0b0c16",
        shaft: ["#262640", "#0b0c16"],
        glass: ["#3b4374", "#d08a63"],
        city: ["#f0a060", 0.2],
        glow: 0.7,
        windows: 0.85,
        unlit: "#2a2f4c",
        dots: ["#e3ce34", 0, 0.35],
    },
    day: {
        // mostly paper with a few dots of blue, like the frame's daytime look
        stops: ["#a8c2e2", "#d3dfee", "#f0f2f2"],
        far: "#8a93a4",
        near: "#3b4250",
        shaft: ["#596276", "#363c4a"],
        glass: ["#c2d4e8", "#e8eef3"],
        city: ["#ffffff", 0],
        glow: 0.25,
        windows: 0.2,
        unlit: "#727c92",
        dots: ["#2254ac", 0.45, 0],
    },
};

// the red aircraft lights: the spire's tip and the roof's corners, [x, y, css radius]
const BEACONS: [number, number, number][] = [
    [CX, TOP, 1.7],
    [CX - 59, TOP + 103, 1.2],
    [CX + 59, TOP + 103, 1.2],
];

export function Tower({
    bands,
    className,
    sky = "night",
    skyline = true,
    title,
    night = "",
    live = false,
}: TowerProps) {
    const plan = useMemo(() => planFor(night, ART.hood), [night]);
    const minute = useNightMinute(night, live, plan);
    const lights = useMemo(() => litAt(plan, minute), [plan, minute]);
    const key = bands.join(",");
    const colors = useMemo(() => key.split(",").map((c) => mute(c)), [key]);

    return (
        <div
            role="img"
            aria-label={title ?? "The Prudential Tower"}
            className={`relative overflow-hidden ${className ?? ""}`}
            style={{ background: SKIES[sky].stops[1] }}
        >
            <Scene bands={colors} sky={sky} skyline={skyline} lights={lights} />
            {/* the beacons are their own layer, so their blinking never repaints the scene */}
            <svg
                viewBox={`0 0 ${W} ${H}`}
                preserveAspectRatio="xMidYMin slice"
                className="pointer-events-none absolute inset-0 h-full w-full"
                aria-hidden
            >
                {BEACONS.map(([x, y, r], i) => (
                    <circle
                        key={i}
                        cx={x}
                        cy={y}
                        r={r}
                        fill={BEACON_RED}
                        className="beacon"
                        style={{ animationDelay: `${i * 0.45}s` }}
                    />
                ))}
            </svg>
        </div>
    );
}

// useNightMinute is the minute of the night the windows show: the clock's,
// woken at each switch, when live; 9pm otherwise.
function useNightMinute(night: string, live: boolean, plan: Plan): number {
    const on = live && night !== "";
    const [minute, setMinute] = useState(() => (on ? minuteOf(Date.now(), night) : NINE_PM));
    useEffect(() => {
        if (!on) {
            setMinute(NINE_PM);
            return;
        }
        let t: ReturnType<typeof setTimeout>;
        const tick = () => {
            const m = minuteOf(Date.now(), night);
            setMinute(m);
            const next = nextChange(plan, m);
            const wait = next === undefined ? 30 * 60_000 : (next - m) * 60_000 + 50;
            t = setTimeout(tick, Math.min(30 * 60_000, Math.max(1000, wait)));
        };
        tick();
        return () => clearTimeout(t);
    }, [on, night, plan]);
    return minute;
}

interface SceneProps {
    bands: string[];
    sky: Sky;
    skyline: boolean;
    lights: Light[];
}

// Scene is the drawing.
function Scene({ bands, sky, skyline, lights }: SceneProps) {
    const id = useId().replace(/:/g, "");
    const s = SKIES[sky];
    const n = bands.length || 1;
    const first = bands[0] ?? "#ffffff";

    // lit windows, one path per color and brightness, in three layers: the
    // tower's, the skyline's, and the brownstones' in front of everything
    const lit = useMemo(() => {
        const front = ART.hood.homes.length - 1;
        const layers = {
            tower: new Map<string, string>(),
            skyline: new Map<string, string>(),
            front: new Map<string, string>(),
        };
        for (const l of lights) {
            const m = l.group < 0 ? layers.tower : l.group === front ? layers.front : layers.skyline;
            const k = `${l.color}|${Math.round(l.alpha * 4) / 4}`;
            m.set(k, (m.get(k) ?? "") + `M${l.x} ${l.y}h${l.w}v${l.h}h${-l.w}z`);
        }
        const draw = (m: Map<string, string>) => (
            <g opacity={s.windows} shapeRendering="crispEdges">
                {[...m].map(([k, d]) => {
                    const [color, a] = k.split("|");
                    return <path key={k} d={d} fill={color} opacity={Number(a)} />;
                })}
            </g>
        );
        return { tower: draw(layers.tower), skyline: draw(layers.skyline), front: draw(layers.front) };
    }, [lights, id, s.windows]);

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

    return (
        <svg
            viewBox={`0 0 ${W} ${H}`}
            className="absolute inset-0 h-full w-full"
            aria-hidden
            preserveAspectRatio="xMidYMin slice"
        >
            <defs>
                <linearGradient id={`sky-${id}`} x1="0" y1="0" x2="0" y2="1">
                    {stops(
                        banded(
                            [
                                [0, s.stops[0]],
                                [0.55, s.stops[1]],
                                [1, s.stops[2]],
                            ],
                            7,
                        ),
                    )}
                </linearGradient>
                <linearGradient id={`city-${id}`} x1="0" y1="0" x2="0" y2="1">
                    {stops(
                        banded(
                            [
                                [0, s.city[0], 0],
                                [1, s.city[0], s.city[1]],
                            ],
                            5,
                        ),
                    )}
                </linearGradient>
                <radialGradient id={`halo-${id}`}>
                    {stops(
                        banded(
                            [
                                [0, first, 0.32 * s.glow],
                                [0.45, first, 0.12 * s.glow],
                                [1, first, 0],
                            ],
                            9,
                        ),
                    )}
                </radialGradient>
                <linearGradient id={`glass-${id}`} x1="0" y1="0" x2="1" y2="1">
                    {stops(
                        banded(
                            [
                                [0, s.glass[0]],
                                [1, s.glass[1]],
                            ],
                            5,
                        ),
                    )}
                </linearGradient>
                <linearGradient id={`shaft-${id}`} x1="0" y1="0" x2="0" y2="1">
                    {stops(
                        banded(
                            [
                                [0, s.shaft[0]],
                                [1, s.shaft[1]],
                            ],
                            6,
                        ),
                    )}
                </linearGradient>
                <linearGradient id={`spill-${id}`} x1="0" y1="0" x2="0" y2="1">
                    {stops(
                        banded(
                            [
                                [0, first, 0.28 * s.glow],
                                [0.3, first, 0.08 * s.glow],
                                [1, first, 0],
                            ],
                            5,
                        ),
                    )}
                </linearGradient>
                {/* the halftone: an ordered screen of fine dots, shown through a banded mask */}
                {screen(`screen-${id}`, s.dots[0])}
                {screen(`screen-halo-${id}`, first)}
                <linearGradient id={`sky-dots-${id}`} x1="0" y1="0" x2="0" y2="1">
                    {stops(
                        banded(
                            [
                                [0, "#fff", s.dots[1]],
                                [1, "#fff", s.dots[2]],
                            ],
                            5,
                        ),
                    )}
                </linearGradient>
                <radialGradient id={`halo-dots-${id}`}>
                    {stops(
                        banded(
                            [
                                [0, "#fff", 0.55 * s.glow],
                                [1, "#fff", 0],
                            ],
                            5,
                        ),
                    )}
                </radialGradient>
                <mask id={`sky-mask-${id}`} maskUnits="userSpaceOnUse" x="0" y="0" width={W} height={H}>
                    <rect width={W} height={H} fill={`url(#sky-dots-${id})`} />
                </mask>
                <mask id={`halo-mask-${id}`} maskUnits="userSpaceOnUse" x="0" y="0" width={W} height={H}>
                    <ellipse cx={CX} cy={TOP + RING_LOW} rx={170} ry={100} fill={`url(#halo-dots-${id})`} />
                </mask>
                {/* paper grain: fine noise, laid over everything softly */}
                <filter id={`grain-${id}`} x="0" y="0" width="100%" height="100%">
                    <feTurbulence
                        type="fractalNoise"
                        baseFrequency="0.85"
                        numOctaves="2"
                        seed="7"
                        stitchTiles="stitch"
                    />
                    <feColorMatrix type="saturate" values="0" />
                </filter>
                <pattern
                    id={`win-${id}`}
                    width="4"
                    height={FLOOR}
                    patternUnits="userSpaceOnUse"
                    x={CX - TOWER_HALF + 4}
                    y={TOP + SHAFT_TOP + 5}
                >
                    <rect width="2" height="5" fill={s.unlit} shapeRendering="crispEdges" />
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
            {/* the city's glow on the low sky */}
            <rect y={H * 0.45} width={W} height={H * 0.55} fill={`url(#city-${id})`} />
            {sky === "night" &&
                ART.stars.map((t, i) => (
                    <circle key={i} cx={t.x} cy={t.y} r={t.r} fill="#e6ebff" opacity={t.o} />
                ))}
            <ellipse cx={CX} cy={TOP + RING_LOW} rx={190} ry={110} fill={`url(#halo-${id})`} />
            {/* the halftone over the sky and in the crown's glow */}
            <rect width={W} height={H} fill={`url(#screen-${id})`} mask={`url(#sky-mask-${id})`} />
            <rect width={W} height={H} fill={`url(#screen-halo-${id})`} mask={`url(#halo-mask-${id})`} />

            {skyline && (
                <g>
                    {/* the Hancock's glass slab and its notch */}
                    <rect
                        x={HANCOCK.x0}
                        y={HANCOCK.top}
                        width={HANCOCK.x1 - HANCOCK.x0}
                        height={H}
                        fill={`url(#glass-${id})`}
                    />
                    <rect
                        x={HANCOCK.x0 + 22}
                        y={HANCOCK.top}
                        width={3}
                        height={H}
                        fill={s.far}
                        opacity={0.8}
                    />
                    {/* a dark tower behind the Pru's right side, with a mast */}
                    <rect x={DARK.x0} y={DARK.top} width={DARK.x1 - DARK.x0} height={H} fill={s.far} />
                    <rect x={DARK.x1 - 30} y={DARK.top - 6} width={20} height={6} fill={s.far} />
                    <line
                        x1={DARK.x1 - 20}
                        y1={DARK.top - 24}
                        x2={DARK.x1 - 20}
                        y2={DARK.top - 6}
                        stroke={s.far}
                        strokeWidth="1.5"
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

            {/* the rooftop: machinery, antennas (the beacons are drawn over it) */}
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

            {/* the shaft, its windows, its mechanical floors, the crown's light on it */}
            <rect
                x={CX - TOWER_HALF}
                y={TOP + SHAFT_TOP}
                width={2 * TOWER_HALF}
                height={H}
                fill={`url(#shaft-${id})`}
            />
            <rect
                x={CX - TOWER_HALF + 4}
                y={TOP + SHAFT_TOP + 5}
                width={2 * TOWER_HALF - 7}
                height={H}
                fill={`url(#win-${id})`}
            />
            {ART.mech.map((y) => (
                <rect
                    key={y}
                    x={CX - TOWER_HALF}
                    y={y}
                    width={2 * TOWER_HALF}
                    height={FLOOR}
                    fill={`url(#shaft-${id})`}
                />
            ))}
            {lit.tower}
            <rect
                x={CX - TOWER_HALF}
                y={TOP + SHAFT_TOP}
                width={2 * TOWER_HALF}
                height={150}
                fill={`url(#spill-${id})`}
            />
            {/* a warm edge where the city's light catches it, a cool one away */}
            <rect
                x={CX - TOWER_HALF}
                y={TOP + SHAFT_TOP}
                width={1.5}
                height={H}
                fill="#ffcf9a"
                opacity={0.2 * s.glow + 0.05}
            />
            <rect
                x={CX + TOWER_HALF - 1}
                y={TOP + SHAFT_TOP}
                width={1}
                height={H}
                fill="#bcd0ff"
                opacity={0.08}
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
                    {NEIGHBORS.map(([x0, x1, f], i) => (
                        <Neighbor key={i} x0={CX + x0} x1={CX + x1} top={topOf(f)} roof={i} fill={s.near} />
                    ))}
                    {/* 111 Huntington: a round-shouldered tower under a ribbed dome */}
                    <rect x={DOME.x0} y={DOME.top} width={DOME.x1 - DOME.x0} height={H} fill={s.near} />
                    <rect
                        x={DOME.x0 + 2}
                        y={DOME.top - 4}
                        width={DOME.x1 - DOME.x0 - 4}
                        height={4}
                        fill={s.near}
                    />
                    <path
                        d={`M${DOME_MID - DOME_R} ${DOME.top - 4}A${DOME_R} 24 0 0 1 ${DOME_MID + DOME_R} ${DOME.top - 4}Z`}
                        fill={s.near}
                    />
                    <g stroke="#e8dcc0" strokeWidth="0.9" opacity={sky === "day" ? 0.35 : 0.75} fill="none">
                        <path
                            d={`M${DOME_MID - DOME_R} ${DOME.top - 4}A${DOME_R} 24 0 0 1 ${DOME_MID + DOME_R} ${DOME.top - 4}`}
                        />
                        {[-0.8, -0.45, 0, 0.45, 0.8].map((f) => (
                            <line
                                key={f}
                                x1={DOME_MID + f * DOME_R}
                                y1={DOME.top - 4 - 24 * Math.sqrt(1 - f * f)}
                                x2={DOME_MID + f * DOME_R * 0.9}
                                y2={DOME.top - 4}
                            />
                        ))}
                        <line
                            x1={DOME_MID - DOME_R}
                            y1={DOME.top - 5}
                            x2={DOME_MID + DOME_R}
                            y2={DOME.top - 5}
                        />
                        <line x1={DOME_MID} y1={DOME.top - 40} x2={DOME_MID} y2={DOME.top - 28} />
                    </g>
                    {lit.skyline}
                    {/* brownstones in front */}
                    <path d={ART.roofs} fill={s.near} />
                    {lit.front}
                </g>
            )}

            {/* the paper's grain over it all */}
            <rect
                width={W}
                height={H}
                filter={`url(#grain-${id})`}
                opacity={sky === "day" ? 0.4 : 0.3}
                style={{ mixBlendMode: "overlay" }}
            />
        </svg>
    );
}

// stops draws a gradient's stops.
function stops(list: Stop[]) {
    return list.map(([o, c, a = 1], i) => <stop key={i} offset={o} stopColor={c} stopOpacity={a} />);
}

// screen is a halftone pattern: an ordered grid of fine dots in one ink, a
// big dot and a small one in each cell.
function screen(id: string, ink: string) {
    return (
        <pattern id={id} width="3" height="3" patternUnits="userSpaceOnUse">
            <circle cx="0.75" cy="0.75" r="0.6" fill={ink} />
            <circle cx="2.25" cy="2.25" r="0.42" fill={ink} />
        </pattern>
    );
}

// Neighbor is a building of the skyline, its top made a little irregular: a
// setback, a water tank on legs, a mast, or two setbacks.
function Neighbor({
    x0,
    x1,
    top,
    roof,
    fill,
}: {
    x0: number;
    x1: number;
    top: number;
    roof: number;
    fill: string;
}) {
    const w = x1 - x0;
    return (
        <g fill={fill}>
            <rect x={x0} y={top} width={w} height={H} />
            {roof === 0 && <rect x={x0 + 10} y={top - 9} width={w - 28} height={9} />}
            {roof === 1 && (
                <g>
                    <rect x={x0 + 12} y={top - 14} width={11} height={9} rx={1} />
                    <path d={`M${x0 + 11} ${top - 14}l6.5 -4l6.5 4z`} />
                    <path
                        d={`M${x0 + 13} ${top - 5}v5M${x0 + 22} ${top - 5}v5`}
                        stroke={fill}
                        strokeWidth="1.2"
                    />
                </g>
            )}
            {roof === 2 && <path d={`M${x0 + 22} ${top}v-22`} stroke={fill} strokeWidth="1.4" />}
            {roof === 3 && (
                <g>
                    <rect x={x0 + 6} y={top - 6} width={w - 12} height={6} />
                    <rect x={x0 + 14} y={top - 11} width={w - 28} height={5} />
                </g>
            )}
        </g>
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
                style={{ fill: soft(c) }}
                opacity={opacity}
            />
        ));
    return (
        <svg viewBox="0 0 40 44" className={`shrink-0 ${className ?? "h-8 w-8"}`} aria-hidden>
            <rect x="19.2" y="0" width="1.6" height="13" style={{ fill: soft(bands[0]) }} />
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
