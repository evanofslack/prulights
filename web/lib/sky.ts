"use client";

import { useEffect, useState } from "react";
import type { Lighting } from "@/interfaces/schedule";
import type { Sky } from "@/components/Tower";

const bostonHour = new Intl.DateTimeFormat("en-US", {
    timeZone: "America/New_York",
    hour: "numeric",
    hourCycle: "h23",
});

// DUSK is how long before lights-on the sky turns to dusk.
const DUSK = 45 * 60_000;

// skyAt is the sky behind a night's tower at a moment: tonight's follows the
// clock (day, dusk in the last 45 minutes before the crown lights, then night
// until early morning); any other night is drawn at night.
export function skyAt(ms: number, l: Lighting, tonight?: string): Sky {
    if (!tonight || l.date !== tonight) return "night";
    const on = Date.parse(l.lights_on);
    if (Number.isNaN(on) || ms >= on) return "night";
    if (Number(bostonHour.format(new Date(ms))) < 6) return "night"; // after the 4am rollover, still dark
    return ms >= on - DUSK ? "dusk" : "day";
}

// useSky is skyAt for now, kept current a minute at a time.
export function useSky(l: Lighting, tonight?: string): Sky {
    const [now, setNow] = useState(() => Date.now());
    useEffect(() => {
        const id = setInterval(() => setNow(Date.now()), 60_000);
        return () => clearInterval(id);
    }, []);
    return skyAt(now, l, tonight);
}
