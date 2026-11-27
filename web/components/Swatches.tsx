"use client";

import { Tooltip } from "@mantine/core";
import type { ColorKey } from "@/interfaces/schedule";
import { colorNames } from "@/lib/format";
import { usePalette } from "@/lib/palette";
import { swatch } from "./Prudle";

// Swatches is a night's colors as small squares, in crown order: shorter than
// their names where there's little room (someone's "Colors wrong?" report).
// The names stay in the tooltip and for screen readers.
export function Swatches({ colors }: { colors: ColorKey[] }) {
    const palette = usePalette();
    const names = colorNames(colors, palette);
    return (
        <Tooltip label={names} withArrow events={{ hover: true, focus: true, touch: true }}>
            <span role="img" aria-label={names} className="inline-flex items-center gap-1 align-middle">
                {colors.map((k, i) => {
                    const c = palette?.get(k);
                    return (
                        <span
                            key={i}
                            className="h-5 w-5 shrink-0 rounded-sm ring-1 ring-line"
                            style={{ background: c && palette ? swatch(c, palette) : "var(--color-raised)" }}
                        />
                    );
                })}
            </span>
        </Tooltip>
    );
}
