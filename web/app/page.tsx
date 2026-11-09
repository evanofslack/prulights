"use client";

import { useRouter } from "next/navigation";
import { Button } from "@mantine/core";
import { IconMaximize } from "@tabler/icons-react";
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
                    <DisplayButton />
                </div>
            )}
        </Shell>
    );
}

// DisplayButton turns this screen into the always-on display: fullscreen
// where the browser allows it, then /display.
function DisplayButton() {
    const router = useRouter();
    const open = async () => {
        try {
            await document.documentElement.requestFullscreen?.();
        } catch {}
        router.push("/display");
    };
    return (
        <div className="flex flex-col items-start gap-2 rounded-xl bg-panel p-4 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-muted">Leaving a screen on? Show tonight full screen, kept up to date.</p>
            <Button
                variant="default"
                leftSection={<IconMaximize size={18} />}
                onClick={open}
                className="shrink-0"
            >
                Open as display
            </Button>
        </div>
    );
}
