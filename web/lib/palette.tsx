"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { Color, ColorKey } from "@/interfaces/schedule";
import type { PaletteResponse } from "@/interfaces/api";
import { api } from "./api";

// The palette comes from the api once and is kept in localStorage so the
// crown has its colors on the next visit before the network answers.

const KEY = "prulights.palette.v1";
const PaletteContext = createContext<Map<ColorKey, Color> | undefined>(undefined);

function load(): Color[] | undefined {
    try {
        const raw = localStorage.getItem(KEY);
        return raw ? (JSON.parse(raw) as Color[]) : undefined;
    } catch {
        return undefined;
    }
}

export function PaletteProvider({ children }: { children: React.ReactNode }) {
    const [colors, setColors] = useState<Color[]>();
    useEffect(() => {
        setColors((c) => c ?? load());
        api<PaletteResponse>("/palette")
            .then((p) => {
                setColors(p.colors);
                try {
                    localStorage.setItem(KEY, JSON.stringify(p.colors));
                } catch {}
            })
            .catch(() => {});
    }, []);
    const map = useMemo(() => (colors ? new Map(colors.map((c) => [c.key, c])) : undefined), [colors]);
    return <PaletteContext.Provider value={map}>{children}</PaletteContext.Provider>;
}

export const usePalette = () => useContext(PaletteContext);

// UNKNOWN is the crown's color before the palette loads.
const UNKNOWN = "#3d465e";

// bandsOf expands a night's colors into the bands drawn on the crown: rainbow
// becomes its six bands, everything else one band per color.
export function bandsOf(keys: ColorKey[], palette: Map<ColorKey, Color> | undefined): string[] {
    const out: string[] = [];
    for (const k of keys) {
        const c = palette?.get(k);
        if (c?.bands?.length) out.push(...c.bands.map((b) => palette?.get(b)?.hex ?? UNKNOWN));
        else out.push(c?.hex ?? UNKNOWN);
    }
    return out.length ? out : [UNKNOWN];
}
