"use client";

import { createContext, useCallback, useContext } from "react";
import type { PrudleSummary } from "@/interfaces/api";
import { useApi, useMeta } from "./api";
import { useAuth } from "./auth";

// The Prudle blackout: in December, signed in, a night's colors stay hidden
// until it's played. Tonight shows once your game is over (won, lost or "just
// show me"); later nights stay hidden until they're tonight. Earlier nights
// show, played or not. Turning Prudle off in /settings shows everything. It's
// only on the page: the public api and the frames still show every color.

interface Prudle {
    summary: PrudleSummary | undefined;
    tonight: string | undefined;
    // hidden is whether a night's colors are hidden from you right now. While
    // it isn't known yet, December nights are hidden, so nothing flashes.
    hidden: (date: string) => boolean;
    reload: () => void;
}

const PrudleContext = createContext<Prudle | undefined>(undefined);

export const inSeason = (date: string) => date.slice(5, 7) === "12";

export function PrudleProvider({ children }: { children: React.ReactNode }) {
    const { me } = useAuth();
    const meta = useMeta();
    const { data, error, reload } = useApi<PrudleSummary>(me ? "/me/prudle" : null, 10 * 60_000);
    const summary = me ? data : undefined;
    const tonight = summary?.tonight ?? meta?.tonight;
    const hidden = useCallback(
        (d: string) => {
            if (!inSeason(d) || me === null) return false;
            if (error && !summary) return false; // never lock the page on a failed load
            if (me === undefined || !summary || !tonight) return true;
            if (summary.off || d < tonight) return false;
            if (d === tonight) return summary.playable && !summary.done;
            return true;
        },
        [me, summary, error, tonight],
    );
    return (
        <PrudleContext.Provider value={{ summary, tonight, hidden, reload }}>
            {children}
        </PrudleContext.Provider>
    );
}

export function usePrudle(): Prudle {
    const p = useContext(PrudleContext);
    if (!p) throw new Error("usePrudle outside PrudleProvider");
    return p;
}

// HIDDEN is a hidden night's crown: one unlit band.
export const UNLIT = "#3b3f4a";
export const HIDDEN = [UNLIT];
export const HIDDEN_NAME = "Colors hidden";
