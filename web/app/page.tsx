"use client";

import { Suspense, useEffect } from "react";
import { Skeleton } from "@mantine/core";
import type { TonightResponse } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Night, NightSkeleton, Upcoming } from "@/components/Night";
import { SawIt } from "@/components/SawIt";
import { Month } from "@/components/Month";
import { MyNights } from "@/components/MyNights";
import { Lately } from "@/components/Lately";
import { ErrorNote, Shell } from "@/components/Shell";
import { PrudleBoard, PrudleResult, PrudleSection, PrudleSignIn } from "@/components/PrudleGame";
import { inSeason, usePrudle } from "@/lib/blackout";

// / is the whole site on one page: tonight, the week, signed in your nights,
// and the month. Each section loads on its own. In December, signed in,
// tonight is a Prudle until it's played: the game stands in for its colors.
export default function HomePage() {
    // re-check every 10 minutes, so a page left open rolls over to the next night
    const { data, error, reload } = useApi<TonightResponse>("/tonight", 10 * 60_000);
    const { me } = useAuth();
    const prudle = usePrudle();
    const season = !!data && inSeason(data.date);
    // in December, wait to know whether tonight is a game before showing it
    const deciding = season && (me === undefined || (!!me && !prudle.summary));
    const playing = !!data && !!me && !!prudle.summary?.playable && prudle.hidden(data.date);
    // the sections render after their data, too late for the browser to follow
    // a #calendar or #nights link on its own: scroll once they're there
    const ready = data !== undefined && me !== undefined && !deciding;
    useEffect(() => {
        if (!ready || !location.hash) return;
        document.getElementById(location.hash.slice(1))?.scrollIntoView();
    }, [ready]);
    return (
        <Shell>
            {error && !data && <ErrorNote error={error} retry={reload} />}
            {!data && !error && <NightSkeleton />}
            {data && (
                <div className="flex flex-col gap-12">
                    {deciding ? (
                        <NightSkeleton />
                    ) : playing ? (
                        <PrudleBoard date={data.date} />
                    ) : (
                        <Night
                            lighting={data.lighting}
                            tonight={data.date}
                            publishedThrough={data.published_through}
                        >
                            {season && me && prudle.summary?.done && <PrudleResult date={data.date} />}
                            {season && me === null && data.lighting.scheduled && data.lighting.cause && (
                                <PrudleSignIn />
                            )}
                            <SawIt date={data.date} tonight={data.date} />
                        </Night>
                    )}
                    <section id="week" aria-label="This week" className="flex scroll-mt-4 flex-col gap-4">
                        <h2 className="text-2xl font-bold">This week</h2>
                        <Upcoming nights={data.upcoming} tonight={data.date} />
                    </section>
                    {me && <PrudleSection tonight={data.date} />}
                    <MyNights />
                    <Lately tonight={data.date} />
                    <Suspense fallback={<Skeleton height={480} radius="lg" />}>
                        <Month tonight={data.date} />
                    </Suspense>
                </div>
            )}
        </Shell>
    );
}
