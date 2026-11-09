"use client";

import { Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ActionIcon, Skeleton } from "@mantine/core";
import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import type { LightingsResponse, MetaResponse } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { addDays, colorNames, dayOfMonth, monthTitle } from "@/lib/format";
import { bandsOf, usePalette } from "@/lib/palette";
import { ErrorNote, Shell } from "@/components/Shell";

const MONTH = /^\d{4}-\d{2}$/;
const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

function shiftMonth(month: string, n: number): string {
    const [y, m] = month.split("-").map(Number);
    const t = new Date(Date.UTC(y, m - 1 + n, 1));
    return t.toISOString().slice(0, 7);
}

function CalendarPage() {
    const router = useRouter();
    const palette = usePalette();
    const meta = useApi<MetaResponse>("/meta");
    const tonight = meta.data?.tonight;
    const param = useSearchParams().get("m");
    const month = param && MONTH.test(param) ? param : tonight?.slice(0, 7);

    const first = month ? `${month}-01` : null;
    const last = month ? addDays(`${shiftMonth(month, 1)}-01`, -1) : null;
    const { data, error, reload } = useApi<LightingsResponse>(
        first && last ? `/lightings?from=${first}&to=${last}` : null,
    );
    const go = (n: number) => month && router.push(`/calendar?m=${shiftMonth(month, n)}`);

    if (meta.error && !month) return <ErrorNote error={meta.error} retry={meta.reload} />;
    if (!month) return <Skeleton height={480} radius="lg" />;

    const lead = new Date(`${month}-01T12:00:00Z`).getUTCDay();
    const nights = data && data.from === first ? data.lightings : undefined;
    return (
        <div className="flex flex-col gap-4">
            <div className="flex items-center gap-2">
                <h1 className="mr-auto text-3xl font-bold">{monthTitle(month)}</h1>
                <ActionIcon variant="default" size="xl" aria-label="Previous month" onClick={() => go(-1)}>
                    <IconChevronLeft />
                </ActionIcon>
                <ActionIcon variant="default" size="xl" aria-label="Next month" onClick={() => go(1)}>
                    <IconChevronRight />
                </ActionIcon>
            </div>
            {error && <ErrorNote error={error} retry={reload} />}
            <div className="grid grid-cols-7 gap-1.5 sm:gap-2">
                {WEEKDAYS.map((w) => (
                    <div key={w} className="pb-1 text-center text-sm text-muted">
                        {w}
                    </div>
                ))}
                {Array.from({ length: lead }, (_, i) => (
                    <div key={`lead${i}`} />
                ))}
                {nights
                    ? nights.map((l) => {
                          const isTonight = l.date === tonight;
                          return (
                              <Link
                                  key={l.date}
                                  href={`/night?d=${l.date}`}
                                  title={`${colorNames(l.colors, palette)}${l.scheduled ? ` for ${l.cause}` : ""}`}
                                  className={`group flex aspect-[3/4] flex-col overflow-hidden rounded-lg bg-panel ring-offset-2 ring-offset-surface sm:aspect-square ${
                                      isTonight ? "ring-2 ring-accent" : ""
                                  }`}
                              >
                                  <span
                                      className={`px-1.5 pt-1 text-sm sm:text-base ${isTonight ? "font-bold text-accent" : "text-muted"}`}
                                  >
                                      {dayOfMonth(l.date)}
                                  </span>
                                  <span
                                      className={`m-1.5 mt-1 flex flex-1 overflow-hidden rounded-md ${l.scheduled ? "" : "opacity-35"}`}
                                  >
                                      {bandsOf(l.colors, palette).map((c, i) => (
                                          <span key={i} className="flex-1" style={{ background: c }} />
                                      ))}
                                  </span>
                              </Link>
                          );
                      })
                    : Array.from({ length: 30 }, (_, i) => (
                          <Skeleton key={i} className="aspect-[3/4] sm:aspect-square" radius="md" />
                      ))}
            </div>
            <p className="text-sm text-muted">Faded days have no special lighting: the crown shines white.</p>
        </div>
    );
}

export default function Page() {
    return (
        <Shell>
            <Suspense fallback={<Skeleton height={480} radius="lg" />}>
                <CalendarPage />
            </Suspense>
        </Shell>
    );
}
