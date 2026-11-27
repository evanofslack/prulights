"use client";

import { Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button } from "@mantine/core";
import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import type { Lighting } from "@/interfaces/schedule";
import type { MetaResponse } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { addDays, shortDate } from "@/lib/format";
import { Night, NightSkeleton } from "@/components/Night";
import { SawIt } from "@/components/SawIt";
import { ErrorNote, Shell } from "@/components/Shell";

const DATE = /^\d{4}-\d{2}-\d{2}$/;

function NightPage() {
    const d = useSearchParams().get("d") ?? "";
    const valid = DATE.test(d);
    const { data, error, reload } = useApi<Lighting>(valid ? `/lightings/${d}` : null);
    const meta = useApi<MetaResponse>("/meta");
    if (!valid) {
        return <ErrorNote error={new Error("that link is missing its date")} />;
    }
    // the day nav sits in the details column, under the night it steps from
    const nav = (
        <nav className="flex items-center justify-center gap-2 pt-2 sm:justify-start">
            <Button
                component={Link}
                href={`/night?d=${addDays(d, -1)}`}
                variant="default"
                size="md"
                leftSection={<IconChevronLeft size={18} />}
            >
                {shortDate(addDays(d, -1))}
            </Button>
            <Button component={Link} href="/" variant="default" size="md">
                Tonight
            </Button>
            <Button
                component={Link}
                href={`/night?d=${addDays(d, 1)}`}
                variant="default"
                size="md"
                rightSection={<IconChevronRight size={18} />}
            >
                {shortDate(addDays(d, 1))}
            </Button>
        </nav>
    );
    return (
        <div className="flex flex-col gap-8">
            {error && <ErrorNote error={error} retry={reload} />}
            {error && nav}
            {!data && !error && <NightSkeleton />}
            {data && data.date === d && (
                <Night
                    lighting={data}
                    tonight={meta.data?.tonight}
                    publishedThrough={meta.data?.published_through}
                >
                    <SawIt date={data.date} tonight={meta.data?.tonight} />
                    {nav}
                </Night>
            )}
        </div>
    );
}

export default function Page() {
    return (
        <Shell>
            <Suspense fallback={<NightSkeleton />}>
                <NightPage />
            </Suspense>
        </Shell>
    );
}
