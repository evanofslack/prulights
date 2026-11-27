"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { TonightResponse } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { colorNames, longDate } from "@/lib/format";
import { bandsOf, usePalette } from "@/lib/palette";
import { Tower } from "@/components/Tower";
import { useSky } from "@/lib/sky";

// /display is for a spare screen left on: the lit tower and tonight, nothing
// else. It re-checks every 5 minutes (and so rolls over at 4am), keeps the
// screen awake where the browser allows, hides the cursor when idle, and
// drifts slowly so nothing burns in.
export default function DisplayPage() {
    const { data, error } = useApi<TonightResponse>("/tonight", 5 * 60_000);
    const palette = usePalette();
    useWakeLock();
    const idle = useIdle(3000);
    const drift = useDrift();
    const sky = useSkyOf(data);
    const router = useRouter();

    // Esc (or the Exit link) leaves the display, and fullscreen with it
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => e.key === "Escape" && router.push("/");
        window.addEventListener("keydown", onKey);
        return () => window.removeEventListener("keydown", onKey);
    }, [router]);
    useEffect(
        () => () => {
            if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
        },
        [],
    );

    return (
        <main
            className={`fixed inset-0 overflow-hidden bg-[#05070d] text-[#eef1f8] ${idle ? "cursor-none" : ""}`}
            style={{ colorScheme: "dark" }}
            // always the night sky, whatever the site's look
            data-mantine-color-scheme="dark"
        >
            <Link
                href="/"
                className={`fixed top-4 left-4 z-10 rounded-md bg-white/10 px-3 py-1.5 text-sm text-[#eef1f8] transition-opacity duration-500 hover:bg-white/20 ${
                    idle ? "pointer-events-none opacity-0" : "opacity-100"
                }`}
            >
                ← Exit
            </Link>
            <div
                className="flex h-full w-full items-center justify-center gap-[5vw] px-[5vw] transition-transform duration-[60s] ease-linear"
                style={{ transform: `translate(${drift.x}px, ${drift.y}px)` }}
            >
                {data && (
                    <>
                        <Tower
                            bands={bandsOf(data.lighting.colors, palette)}
                            sky={sky}
                            night={data.date}
                            live
                            className="h-[94vh] w-[42vw] max-w-[620px] shrink-0 rounded-[2vh]"
                        />
                        <div className="flex max-w-[46vw] min-w-0 flex-col gap-[1.5vh] pb-[2vh]">
                            <p className="text-[3vh] text-[#98a1b8]">
                                <span className="font-bold text-[#eef1f8]">Tonight</span> ·{" "}
                                {longDate(data.date)}
                            </p>
                            <h1 className="text-[9vh] leading-tight font-bold">
                                {colorNames(data.lighting.colors, palette)}
                            </h1>
                            <p className="text-[4.4vh] leading-snug">
                                {data.lighting.scheduled
                                    ? `for ${data.lighting.cause}`
                                    : "No special lighting tonight"}
                            </p>
                        </div>
                    </>
                )}
                {!data && (
                    <p className="self-center text-[3vh] text-[#98a1b8]">
                        {error ? "Can't reach the schedule. Retrying…" : "Loading…"}
                    </p>
                )}
            </div>
        </main>
    );
}

function useWakeLock() {
    useEffect(() => {
        let lock: WakeLockSentinel | undefined;
        const request = async () => {
            try {
                lock = await navigator.wakeLock?.request("screen");
            } catch {}
        };
        request();
        const onVisible = () => document.visibilityState === "visible" && request();
        document.addEventListener("visibilitychange", onVisible);
        return () => {
            document.removeEventListener("visibilitychange", onVisible);
            lock?.release().catch(() => {});
        };
    }, []);
}

function useIdle(ms: number): boolean {
    const [idle, setIdle] = useState(false);
    useEffect(() => {
        let t = setTimeout(() => setIdle(true), ms);
        const wake = () => {
            setIdle(false);
            clearTimeout(t);
            t = setTimeout(() => setIdle(true), ms);
        };
        window.addEventListener("pointermove", wake);
        return () => {
            clearTimeout(t);
            window.removeEventListener("pointermove", wake);
        };
    }, [ms]);
    return idle;
}

// useDrift moves the layout a few pixels each minute, around a small circle.
function useDrift(): { x: number; y: number } {
    const [step, setStep] = useState(0);
    useEffect(() => {
        const id = setInterval(() => setStep((s) => s + 1), 60_000);
        return () => clearInterval(id);
    }, []);
    const angle = (step * Math.PI) / 15;
    return { x: Math.round(Math.cos(angle) * 12), y: Math.round(Math.sin(angle) * 8) };
}

// useSkyOf is the sky for the display's night, night until the data loads.
function useSkyOf(data: TonightResponse | undefined) {
    const placeholder = { date: "", lights_on: "" } as TonightResponse["lighting"];
    return useSky(data?.lighting ?? placeholder, data?.date);
}
