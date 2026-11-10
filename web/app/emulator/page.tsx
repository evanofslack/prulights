"use client";

import { Suspense, useCallback, useEffect, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { ActionIcon, Button, SegmentedControl, Skeleton, Switch, TextInput } from "@mantine/core";
import { IconChevronLeft, IconChevronRight, IconExternalLink } from "@tabler/icons-react";
import { useMeta } from "@/lib/api";
import { addDays, longDate } from "@/lib/format";
import { Shell } from "@/components/Shell";
import { Device, type DeviceParams } from "./Device";

// /emulator previews the e-ink frame. Manual mode: pick a view, a date and
// conditions, and see what the server draws for the 7.3" Spectra 6 panel.
// Device mode (Device.tsx) is a virtual frame on the device api: register,
// claim, the button, syncs and battery. Everything but the device's token
// lives in the url, so a preview can be bookmarked or shared.

const VIEWS = ["tonight", "week", "month", "log", "setup", "goodnight"] as const;
type View = (typeof VIEWS)[number];
type Colors = "measured" | "ideal" | "both";
type Size = "true" | "fit" | "2x" | "4x";
type Mode = "manual" | "device";

// The panel is 800×480 on a 7.3" diagonal: about 128 pixels per inch. A css
// inch is 96px, so at true size the 480px-wide portrait frame is ~361 css px.
const PANEL_PPI = Math.hypot(800, 480) / 7.3;
const TRUE_WIDTH = Math.round((480 / PANEL_PPI) * 96);
const WIDTHS: Record<Size, number> = { true: TRUE_WIDTH, fit: 480, "2x": 960, "4x": 1920 };

const DATE = /^\d{4}-\d{2}-\d{2}$/;

interface Settings extends DeviceParams {
    mode: Mode;
    view: View;
    date: string; // "" for the server's tonight
    colors: Colors;
    size: Size;
    sample: boolean;
    stale: boolean;
    battery: boolean;
}

function read(q: URLSearchParams): Settings {
    const view = q.get("view") as View;
    const colors = q.get("colors") as Colors;
    const size = q.get("size") as Size;
    const date = q.get("date") ?? "";
    const mv = Number(q.get("mv"));
    const clock = q.get("clock") ?? "";
    return {
        mode: q.get("mode") === "device" ? "device" : "manual",
        power: q.get("power") === "battery" ? "battery" : "usb",
        mv: mv >= 3000 && mv <= 4300 ? mv : 3900,
        offline: q.get("offline") === "1",
        clock: /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(clock) ? clock : "",
        slow: q.get("slow") === "1",
        view: VIEWS.includes(view) ? view : "tonight",
        date: DATE.test(date) ? date : "",
        colors: ["measured", "ideal", "both"].includes(colors) ? colors : "measured",
        size: size in WIDTHS ? size : "true",
        sample: q.get("sample") === "1",
        stale: q.get("stale") === "1",
        battery: q.get("low_battery") === "1",
    };
}

function write(s: Settings): string {
    const q = new URLSearchParams();
    if (s.mode === "device") {
        q.set("mode", "device");
        if (s.power !== "usb") q.set("power", s.power);
        if (s.power === "battery" && s.mv !== 3900) q.set("mv", String(s.mv));
        if (s.offline) q.set("offline", "1");
        if (s.clock) q.set("clock", s.clock);
        if (s.slow) q.set("slow", "1");
    }
    if (s.view !== "tonight") q.set("view", s.view);
    if (s.date) q.set("date", s.date);
    if (s.colors !== "measured") q.set("colors", s.colors);
    if (s.size !== "true") q.set("size", s.size);
    if (s.sample) q.set("sample", "1");
    if (s.stale) q.set("stale", "1");
    if (s.battery) q.set("low_battery", "1");
    return q.toString();
}

function renderURL(s: Settings, date: string, palette: "measured" | "ideal"): string {
    const q = new URLSearchParams({ date, palette });
    if (s.sample) q.set("sample", "1");
    if (s.stale) q.set("stale", "1");
    if (s.battery) q.set("low_battery", "1");
    return `/dev/render/${s.view}?${q}`;
}

function Emulator() {
    const meta = useMeta();
    const router = useRouter();
    const path = usePathname();
    const params = useSearchParams();
    const s = read(params);
    const date = s.date || meta?.tonight || "";

    // The url updates a moment after router.replace, so changes made in quick
    // succession build on the last one asked for, not on the url.
    const pending = useRef<string | null>(null);
    const set = useCallback(
        (change: Partial<Settings>) => {
            const base = read(new URLSearchParams(pending.current ?? window.location.search));
            const qs = write({ ...base, ...change });
            pending.current = qs;
            router.replace(qs ? `${path}?${qs}` : path, { scroll: false });
        },
        [path, router],
    );
    useEffect(() => {
        if (pending.current === params.toString()) pending.current = null;
    }, [params]);

    // ← → step a day, 1–6 pick a view
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (e.target instanceof HTMLInputElement || e.metaKey || e.ctrlKey || e.altKey || !date) return;
            if (read(new URLSearchParams(window.location.search)).mode === "device") return;
            if (e.key === "ArrowLeft") set({ date: addDays(date, -1) });
            else if (e.key === "ArrowRight") set({ date: addDays(date, 1) });
            else if (/^[1-6]$/.test(e.key)) set({ view: VIEWS[Number(e.key) - 1] });
        };
        window.addEventListener("keydown", onKey);
        return () => window.removeEventListener("keydown", onKey);
    }, [date, set]);

    if (!meta) return <Skeleton height={600} radius="lg" />;
    if (!meta.dev) {
        return (
            <div className="flex max-w-xl flex-col gap-3 rounded-xl bg-panel p-6">
                <h1 className="text-2xl font-bold">Frame emulator</h1>
                <p className="text-muted">
                    The frame previews only exist when the server runs in dev mode. Locally, run{" "}
                    <code className="rounded bg-raised px-1.5 py-0.5">just start</code> or{" "}
                    <code className="rounded bg-raised px-1.5 py-0.5">just serve</code> with{" "}
                    <code className="rounded bg-raised px-1.5 py-0.5">DEV=1</code>.
                </p>
            </div>
        );
    }

    const palettes: ("measured" | "ideal")[] = s.colors === "both" ? ["measured", "ideal"] : [s.colors];
    const modes = (
        <SegmentedControl
            size="sm"
            value={s.mode}
            onChange={(v) => set({ mode: v as Mode })}
            data={[
                { value: "manual", label: "Manual" },
                { value: "device", label: "Device" },
            ]}
            aria-label="Mode"
        />
    );
    if (s.mode === "device") {
        return (
            <div className="flex flex-col gap-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                        <h1 className="text-xl font-bold">Frame emulator</h1>
                        <p className="text-sm text-muted">
                            A virtual frame on the device api, drawing the same pictures the real one
                            downloads.
                        </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                        <SegmentedControl
                            size="sm"
                            value={s.colors === "ideal" ? "ideal" : "measured"}
                            onChange={(v) => set({ colors: v as Colors })}
                            data={[
                                { value: "measured", label: "Measured" },
                                { value: "ideal", label: "Ideal" },
                            ]}
                            aria-label="Colors"
                        />
                        <SegmentedControl
                            size="sm"
                            value={s.size}
                            onChange={(v) => set({ size: v as Size })}
                            data={[
                                { value: "true", label: "True size" },
                                { value: "fit", label: "1:1 px" },
                                { value: "2x", label: "2×" },
                            ]}
                            aria-label="Size"
                        />
                        {modes}
                    </div>
                </div>
                <Device
                    p={s}
                    set={set}
                    palette={s.colors === "ideal" ? "ideal" : "measured"}
                    width={WIDTHS[s.size]}
                />
            </div>
        );
    }
    return (
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start">
            <aside className="flex w-full shrink-0 flex-col gap-5 rounded-xl bg-panel p-4 lg:sticky lg:top-4 lg:w-80">
                <div className="flex flex-col gap-3">
                    <div>
                        <h1 className="text-xl font-bold">Frame emulator</h1>
                        <p className="text-sm text-muted">
                            What the 7.3&quot; e-ink frame shows, for any view and night. Device mode runs a
                            virtual frame.
                        </p>
                    </div>
                    {modes}
                </div>

                <Field label="View" hint="1–6">
                    <div className="grid grid-cols-3 gap-1.5">
                        {VIEWS.map((v) => (
                            <Button
                                key={v}
                                size="xs"
                                variant={s.view === v ? "filled" : "default"}
                                onClick={() => set({ view: v })}
                                aria-pressed={s.view === v}
                            >
                                {v}
                            </Button>
                        ))}
                    </div>
                </Field>

                <Field label="Night" hint="← →">
                    <div className="flex items-center gap-1.5">
                        <ActionIcon
                            variant="default"
                            size="lg"
                            aria-label="Previous night"
                            onClick={() => set({ date: addDays(date, -1) })}
                        >
                            <IconChevronLeft size={18} />
                        </ActionIcon>
                        <TextInput
                            type="date"
                            aria-label="Night"
                            value={date}
                            onChange={(e) =>
                                DATE.test(e.currentTarget.value) && set({ date: e.currentTarget.value })
                            }
                            className="flex-1"
                        />
                        <ActionIcon
                            variant="default"
                            size="lg"
                            aria-label="Next night"
                            onClick={() => set({ date: addDays(date, 1) })}
                        >
                            <IconChevronRight size={18} />
                        </ActionIcon>
                    </div>
                    <div className="flex items-center justify-between text-sm text-muted">
                        <span>{date && longDate(date)}</span>
                        {s.date && s.date !== meta.tonight && (
                            <button
                                type="button"
                                className="underline hover:text-fg"
                                onClick={() => set({ date: "" })}
                            >
                                Tonight
                            </button>
                        )}
                    </div>
                </Field>

                <Field label="Colors">
                    <SegmentedControl
                        fullWidth
                        size="xs"
                        value={s.colors}
                        onChange={(v) => set({ colors: v as Colors })}
                        data={[
                            { value: "measured", label: "Measured" },
                            { value: "ideal", label: "Ideal" },
                            { value: "both", label: "Both" },
                        ]}
                    />
                    <p className="text-xs text-subtle">
                        Measured is close to what the panel really shows; ideal is the pure inks.
                    </p>
                </Field>

                <Field label="Size">
                    <SegmentedControl
                        fullWidth
                        size="xs"
                        value={s.size}
                        onChange={(v) => set({ size: v as Size })}
                        data={[
                            { value: "true", label: "True size" },
                            { value: "fit", label: "1:1 px" },
                            { value: "2x", label: "2×" },
                            { value: "4x", label: "4×" },
                        ]}
                    />
                    <p className="text-xs text-subtle">
                        True size matches the real panel (~{Math.round(PANEL_PPI)} ppi) on a typical screen.
                    </p>
                </Field>

                <Field label="Conditions">
                    <Switch
                        label="Sample sightings and stats"
                        checked={s.sample}
                        onChange={(e) => set({ sample: e.currentTarget.checked })}
                    />
                    <Switch
                        label="Schedule out of date"
                        checked={s.stale}
                        onChange={(e) => set({ stale: e.currentTarget.checked })}
                    />
                    <Switch
                        label="Low battery"
                        checked={s.battery}
                        onChange={(e) => set({ battery: e.currentTarget.checked })}
                    />
                </Field>
            </aside>

            <section className="flex min-w-0 flex-1 flex-col gap-3">
                <div className="flex flex-wrap gap-6 overflow-x-auto pb-2">
                    {palettes.map((p) => (
                        <Frame key={p} src={renderURL(s, date, p)} width={WIDTHS[s.size]} label={p} />
                    ))}
                </div>
            </section>
        </div>
    );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
    return (
        <div className="flex flex-col gap-2">
            <div className="flex items-baseline justify-between">
                <span className="text-sm font-bold">{label}</span>
                {hint && <kbd className="text-xs text-subtle">{hint}</kbd>}
            </div>
            {children}
        </div>
    );
}

// Frame is the panel in a dark bezel. The image keeps showing the last frame
// while the next one renders, dimmed, like the panel mid-refresh.
function Frame({ src, width, label }: { src: string; width: number; label: string }) {
    const [loaded, setLoaded] = useState<string>();
    const [failed, setFailed] = useState<string>();
    const busy = loaded !== src && failed !== src;
    return (
        <figure className="flex shrink-0 flex-col gap-2">
            <div className="rounded-[14px] bg-[#16181d] p-[18px] shadow-xl ring-1 ring-white/10">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                    src={src}
                    alt={`The frame, ${label} colors`}
                    width={480}
                    height={800}
                    onLoad={() => setLoaded(src)}
                    onError={() => setFailed(src)}
                    className={`block h-auto max-w-none transition-opacity ${busy ? "opacity-60" : ""}`}
                    style={{ width, imageRendering: width > 480 ? "pixelated" : "auto" }}
                />
            </div>
            <figcaption className="flex items-center justify-between text-sm text-muted">
                <span className="capitalize">{label}</span>
                {failed === src ? (
                    <span className="text-danger">Couldn&apos;t render</span>
                ) : (
                    <a
                        href={src}
                        target="_blank"
                        rel="noopener"
                        className="inline-flex items-center gap-1 hover:text-fg"
                    >
                        PNG <IconExternalLink size={14} />
                    </a>
                )}
            </figcaption>
        </figure>
    );
}

export default function Page() {
    return (
        <Shell wide>
            <Suspense fallback={<Skeleton height={600} radius="lg" />}>
                <Emulator />
            </Suspense>
        </Shell>
    );
}
