"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ActionIcon, Skeleton } from "@mantine/core";
import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import type { LightingsResponse } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { addDays, colorNames, dayOfMonth, monthTitle } from "@/lib/format";
import { bandsOf, soft, usePalette } from "@/lib/palette";
import { ErrorNote } from "./Shell";

const MONTH = /^\d{4}-\d{2}$/;
const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

function shiftMonth(month: string, n: number): string {
    const [y, m] = month.split("-").map(Number);
    const t = new Date(Date.UTC(y, m - 1 + n, 1));
    return t.toISOString().slice(0, 7);
}

// Month is the calendar section of the home page: a month of nights as color
// cells, each opening that night. The month lives in ?m= so it can be linked,
// but changing it replaces the url in place: no jump, no history per month.
export function Month({ tonight }: { tonight: string }) {
    const router = useRouter();
    const palette = usePalette();
    const param = useSearchParams().get("m");
    const month = param && MONTH.test(param) ? param : tonight.slice(0, 7);

    const first = `${month}-01`;
    const last = addDays(`${shiftMonth(month, 1)}-01`, -1);
    const { data, error, reload } = useApi<LightingsResponse>(`/lightings?from=${first}&to=${last}`);
    const go = (n: number) => router.replace(`/?m=${shiftMonth(month, n)}#calendar`, { scroll: false });

    const lead = new Date(`${month}-01T12:00:00Z`).getUTCDay();
    const nights = data && data.from === first ? data.lightings : undefined;
    return (
        <section
            id="calendar"
            aria-label="Calendar"
            className="flex scroll-mt-4 flex-col gap-4 rounded-2xl bg-raised p-3 shadow-lg sm:p-6"
        >
            <div className="flex items-center gap-2">
                <h2 className="mr-auto text-2xl font-bold">{monthTitle(month)}</h2>
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
                                  className={`group flex aspect-[3/4] flex-col overflow-hidden rounded-lg bg-panel ring-offset-2 ring-offset-raised sm:aspect-square ${
                                      isTonight ? "ring-2 ring-accent" : ""
                                  }`}
                              >
                                  <span
                                      className={`px-1.5 pt-1 text-sm sm:text-base ${isTonight ? "font-bold text-accent" : "text-muted"}`}
                                  >
                                      {dayOfMonth(l.date)}
                                  </span>
                                  {l.scheduled ? (
                                      <span className="m-1.5 mt-1 flex flex-1 overflow-hidden rounded-md">
                                          {bandsOf(l.colors, palette).map((c, i) => (
                                              <span
                                                  key={i}
                                                  className="flex-1"
                                                  style={{ background: soft(c) }}
                                              />
                                          ))}
                                      </span>
                                  ) : (
                                      // a plain white night: just a hairline, so the special ones stand out
                                      <span className="m-1.5 mt-auto mb-2 h-px bg-line" />
                                  )}
                              </Link>
                          );
                      })
                    : Array.from({ length: 30 }, (_, i) => (
                          <Skeleton key={i} className="aspect-[3/4] sm:aspect-square" radius="md" />
                      ))}
            </div>
            <p className="text-sm text-muted">Blank = plain white.</p>
        </section>
    );
}
