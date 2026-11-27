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
import {
    PracticeCard,
    PrudleBoard,
    PrudleResult,
    PrudleSection,
    PrudleSignIn,
} from "@/components/PrudleGame";
import { inSeason, usePrudle } from "@/lib/blackout";

// / is the whole site on one page: tonight, the week, signed in your nights,
// and the month. Each section loads on its own. In December, signed in,
// tonight is a Prudle until it's played: the game stands in for its colors.
// On a phone it's one column; on a computer the calendar sits beside the rest.
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
        <Shell wide>
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
                    {/* on a computer: the week over the calendar on the left, your
                        side (Prudle or practice, your nights, lately) on the right. Signed out
                        there's no right side, so the calendar takes it. */}
                    <div className="flex flex-col gap-12 lg:grid lg:grid-cols-2 lg:grid-rows-[auto_1fr] lg:items-start lg:gap-x-10">
                        <section
                            id="week"
                            aria-label="This week"
                            className="flex min-w-0 scroll-mt-4 flex-col gap-4 lg:col-start-1 lg:row-start-1"
                        >
                            <h2 className="text-2xl font-bold">This week</h2>
                            <Upcoming nights={data.upcoming} tonight={data.date} />
                            {!season && me === null && <PracticeCard />}
                        </section>
                        {me && (
                            <div className="flex min-w-0 flex-col gap-12 lg:col-start-2 lg:row-span-2 lg:row-start-1">
                                <PrudleSection tonight={data.date} />
                                {!season && <PracticeCard />}
                                <MyNights />
                                <Lately tonight={data.date} />
                            </div>
                        )}
                        <div
                            className={`min-w-0 ${me ? "lg:col-start-1 lg:row-start-2" : "lg:col-start-2 lg:row-span-2 lg:row-start-1"}`}
                        >
                            <Suspense fallback={<Skeleton height={480} radius="lg" />}>
                                <Month tonight={data.date} />
                            </Suspense>
                        </div>
                    </div>
                </div>
            )}
        </Shell>
    );
}
