"use client";

import { useId } from "react";

// Tower draws the Prudential Tower at night with its crown lit in the given
// colors (one band per color, side by side, like the lit faces of the crown).

interface TowerProps {
    bands: string[];
    className?: string;
    // glow strength 0–1; the display mode turns it up
    glow?: number;
    title?: string;
}

const W = 240;
const H = 560;
const BODY = { x: 70, w: 100, top: 140, bottom: 560 };
const CROWN = { x: 76, w: 88, top: 98, bottom: 140 };

export function Tower({ bands, className, glow = 0.75, title }: TowerProps) {
    const id = useId().replace(/:/g, "");
    const bandW = CROWN.w / bands.length;
    const crownBands = bands.map((c, i) => (
        <rect
            key={i}
            x={CROWN.x + i * bandW}
            y={CROWN.top}
            width={bandW + 0.5}
            height={CROWN.bottom - CROWN.top}
            fill={c}
        />
    ));
    return (
        <svg
            viewBox={`0 0 ${W} ${H}`}
            className={className}
            role="img"
            aria-label={title ?? "The Prudential Tower"}
            preserveAspectRatio="xMidYMax meet"
        >
            <defs>
                <filter id={`glow-${id}`} x="-100%" y="-150%" width="300%" height="400%">
                    <feGaussianBlur stdDeviation="16" />
                </filter>
                <linearGradient id={`body-${id}`} x1="0" x2="1">
                    <stop offset="0" stopColor="var(--color-tower)" />
                    <stop offset="0.55" stopColor="var(--color-tower-line)" stopOpacity="0.9" />
                    <stop offset="1" stopColor="var(--color-tower)" />
                </linearGradient>
                <linearGradient id={`spill-${id}`} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0" stopColor="white" stopOpacity="0.28" />
                    <stop offset="1" stopColor="white" stopOpacity="0" />
                </linearGradient>
                <clipPath id={`body-clip-${id}`}>
                    <rect x={BODY.x} y={BODY.top} width={BODY.w} height={BODY.bottom - BODY.top} />
                </clipPath>
            </defs>

            {/* the crown's glow on the sky */}
            <g filter={`url(#glow-${id})`} opacity={glow}>
                {crownBands}
            </g>

            {/* antenna and beacon */}
            <line
                x1={W / 2}
                y1={10}
                x2={W / 2}
                y2={CROWN.top}
                stroke="var(--color-tower-line)"
                strokeWidth="3"
            />
            <line
                x1={W / 2 - 10}
                y1={60}
                x2={W / 2 + 10}
                y2={60}
                stroke="var(--color-tower-line)"
                strokeWidth="2"
            />
            <circle cx={W / 2} cy={10} r={3} fill="#ff4d4d" />

            {/* body: a tall slab with a window grid */}
            <rect
                x={BODY.x}
                y={BODY.top}
                width={BODY.w}
                height={BODY.bottom - BODY.top}
                fill={`url(#body-${id})`}
            />
            <g clipPath={`url(#body-clip-${id})`} stroke="var(--color-window)" strokeWidth="1" opacity="0.55">
                {Array.from({ length: 11 }, (_, i) => (
                    <line
                        key={`v${i}`}
                        x1={BODY.x + 6 + i * 8.8}
                        y1={BODY.top}
                        x2={BODY.x + 6 + i * 8.8}
                        y2={BODY.bottom}
                    />
                ))}
                {Array.from({ length: 52 }, (_, i) => (
                    <line
                        key={`h${i}`}
                        x1={BODY.x}
                        y1={BODY.top + 8 + i * 8}
                        x2={BODY.x + BODY.w}
                        y2={BODY.top + 8 + i * 8}
                    />
                ))}
            </g>
            {/* the crown's light spilling down the face */}
            <rect
                x={BODY.x}
                y={BODY.top}
                width={BODY.w}
                height={70}
                fill={`url(#spill-${id})`}
                opacity={glow * 0.6}
            />

            {/* the crown itself */}
            <rect
                x={CROWN.x - 3}
                y={CROWN.top - 5}
                width={CROWN.w + 6}
                height={5}
                fill="var(--color-tower-line)"
            />
            {crownBands}
            <rect
                x={CROWN.x - 3}
                y={CROWN.bottom - 2}
                width={CROWN.w + 6}
                height={4}
                fill="var(--color-tower-line)"
            />

            {/* Back Bay at its feet */}
            <path
                d={`M0 ${H} V500 h30 v-14 h22 v28 h10 v-40 h8 V${H} Z M178 ${H} V470 h14 v20 h20 v-30 h12 v44 h16 V${H} Z`}
                fill="var(--color-tower)"
                opacity="0.85"
            />
        </svg>
    );
}

// Crown is a small swatch of a night's colors, for lists and calendars.
export function Crown({ bands, className }: { bands: string[]; className?: string }) {
    return (
        <span
            className={`inline-flex shrink-0 overflow-hidden rounded-md ring-1 ring-black/30 ${className ?? "h-8 w-8"}`}
        >
            {bands.map((c, i) => (
                <span key={i} className="h-full flex-1" style={{ background: c }} />
            ))}
        </span>
    );
}
