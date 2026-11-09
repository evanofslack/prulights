"use client";

import type { TonightResponse } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { Night, NightSkeleton, Upcoming } from "@/components/Night";
import { ErrorNote, Shell } from "@/components/Shell";

export default function TonightPage() {
    // re-check every 10 minutes, so a page left open rolls over to the next night
    const { data, error, reload } = useApi<TonightResponse>("/tonight", 10 * 60_000);
    return (
        <Shell>
            {error && !data && <ErrorNote error={error} retry={reload} />}
            {!data && !error && <NightSkeleton />}
            {data && (
                <div className="flex flex-col gap-8">
                    <Night
                        lighting={data.lighting}
                        tonight={data.date}
                        publishedThrough={data.published_through}
                    />
                    <section className="flex flex-col gap-3">
                        <h2 className="text-xl font-bold">Coming up</h2>
                        <Upcoming nights={data.upcoming} tonight={data.date} />
                    </section>
                </div>
            )}
        </Shell>
    );
}
