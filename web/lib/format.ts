import type { Color, ColorKey, Lighting } from "@/interfaces/schedule";

// Dates are Boston calendar dates ("2026-12-12"). Format them at noon UTC so
// no time zone can push them onto a neighboring day.
function at(date: string): Date {
    return new Date(`${date}T12:00:00Z`);
}

const fmt = (opts: Intl.DateTimeFormatOptions) =>
    new Intl.DateTimeFormat("en-US", { timeZone: "UTC", ...opts });
const weekdayLong = fmt({ weekday: "long" });
const weekdayShort = fmt({ weekday: "short" });
const monthDay = fmt({ month: "long", day: "numeric" });
const monthDayShort = fmt({ month: "short", day: "numeric" });
const monthYear = fmt({ month: "long", year: "numeric" });
const timeBoston = new Intl.DateTimeFormat("en-US", {
    timeZone: "America/New_York",
    hour: "numeric",
    minute: "2-digit",
});

export const longDate = (d: string) => `${weekdayLong.format(at(d))}, ${monthDay.format(at(d))}`;
export const shortDate = (d: string) => monthDayShort.format(at(d));
export const weekday = (d: string) => weekdayLong.format(at(d));
export const weekdayAbbr = (d: string) => weekdayShort.format(at(d));
export const dayOfMonth = (d: string) => Number(d.slice(8, 10));
export const monthTitle = (month: string) => monthYear.format(at(`${month}-15`));
export const lightsOnTime = (iso: string) => timeBoston.format(new Date(iso)).toLowerCase().replace(" ", "");

export function addDays(d: string, n: number): string {
    const t = at(d);
    t.setUTCDate(t.getUTCDate() + n);
    return t.toISOString().slice(0, 10);
}

export function daysBetween(a: string, b: string): number {
    return Math.round((at(b).getTime() - at(a).getTime()) / 86_400_000);
}

// relativeName says "Tonight", "Tomorrow", "Last night" near tonight, else the weekday.
export function relativeName(d: string, tonight: string | undefined): string {
    if (!tonight) return weekday(d);
    const n = daysBetween(tonight, d);
    if (n === 0) return "Tonight";
    if (n === 1) return "Tomorrow";
    if (n === -1) return "Last night";
    return weekday(d);
}

// colorNames reads colors the way the source writes them: "Blue, Pink & Gold".
export function colorNames(keys: ColorKey[], palette: Map<ColorKey, Color> | undefined): string {
    const names = keys.map((k) => palette?.get(k)?.label ?? k.charAt(0).toUpperCase() + k.slice(1));
    if (names.length <= 1) return names.join("");
    return `${names.slice(0, -1).join(", ")} & ${names[names.length - 1]}`;
}

// headline is the one-line answer: what color, and why.
export function headline(l: Lighting, palette: Map<ColorKey, Color> | undefined): string {
    const colors = colorNames(l.colors, palette);
    return l.scheduled && l.cause ? `${colors} for ${l.cause}` : colors;
}

export const kindLabel: Record<string, string> = {
    nonprofit: "Nonprofit partner",
    holiday: "Holiday",
    sports: "Sports",
    civic: "Observance",
    default: "Every night",
};
