"use client";

import Link from "next/link";
import type { RecentSightings } from "@/interfaces/api";
import { useApi, useOnSightingsChanged } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { relativeName } from "@/lib/format";
import { bandsOf, usePalette } from "@/lib/palette";
import { Crown } from "./Tower";

// Lately is a row of small badges for what everyone else saw this past week:
// "Shiv saw it ★4 · Tuesday", each opening that night. Signed in only, and not
// there at all when nobody else has logged a night.
export function Lately({ tonight }: { tonight: string }) {
    const { me } = useAuth();
    if (!me) return null;
    return <Badges tonight={tonight} />;
}

function Badges({ tonight }: { tonight: string }) {
    const palette = usePalette();
    const { data, reload } = useApi<RecentSightings>("/sightings/recent");
    useOnSightingsChanged(reload);
    if (!data || data.sightings.length === 0) return null;
    return (
        <section id="lately" aria-label="Lately" className="flex scroll-mt-4 flex-col gap-3">
            <h2 className="text-2xl font-bold">Lately</h2>
            <ul className="flex flex-wrap gap-2">
                {data.sightings.map((x) => (
                    <li key={`${x.name}-${x.date}`}>
                        <Link
                            href={`/night?d=${x.date}`}
                            className="inline-flex min-h-11 items-center gap-2 rounded-full bg-panel py-1.5 pr-4 pl-2 transition-colors hover:bg-raised"
                        >
                            <Crown bands={bandsOf(x.lighting.colors, palette)} className="h-6 w-6" />
                            <span>
                                <span className="font-bold">{x.name}</span> saw it
                                {x.rating > 0 && <span className="text-accent"> ★{x.rating}</span>}
                                <span className="text-muted"> · {relativeName(x.date, tonight)}</span>
                            </span>
                        </Link>
                    </li>
                ))}
            </ul>
        </section>
    );
}
