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
    return (
        <div className="flex flex-col gap-8">
            {error && <ErrorNote error={error} retry={reload} />}
            {!data && !error && <NightSkeleton />}
            {data && data.date === d && (
                <Night
                    lighting={data}
                    tonight={meta.data?.tonight}
                    publishedThrough={meta.data?.published_through}
                />
            )}
            <nav className="flex justify-between gap-2">
                <Button
                    component={Link}
                    href={`/night?d=${addDays(d, -1)}`}
                    variant="default"
                    leftSection={<IconChevronLeft size={18} />}
                >
                    {shortDate(addDays(d, -1))}
                </Button>
                <Button component={Link} href="/" variant="subtle">
                    Tonight
                </Button>
                <Button
                    component={Link}
                    href={`/night?d=${addDays(d, 1)}`}
                    variant="default"
                    rightSection={<IconChevronRight size={18} />}
                >
                    {shortDate(addDays(d, 1))}
                </Button>
            </nav>
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
