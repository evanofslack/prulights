"use client";

import { useEffect, useState } from "react";
import type { TonightResponse } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { colorNames, lightsOnTime, longDate, relativeName, shortDate, weekdayAbbr } from "@/lib/format";
import { bandsOf, usePalette } from "@/lib/palette";
import { Crown, Tower } from "@/components/Tower";

// /display is for a spare screen left on: the lit tower, tonight, and the week
// ahead. It re-checks every 5 minutes (and so rolls over at 4am), keeps the
// screen awake where the browser allows, hides the cursor when idle, and
// drifts slowly so nothing burns in.
export default function DisplayPage() {
    const { data, error } = useApi<TonightResponse>("/tonight", 5 * 60_000);
    const palette = usePalette();
    useWakeLock();
    const idle = useIdle(3000);
    const drift = useDrift();

    return (
        <main
            className={`fixed inset-0 overflow-hidden bg-[#05070d] text-[#eef1f8] ${idle ? "cursor-none" : ""}`}
            style={{ colorScheme: "dark" }}
        >
            <div
                className="flex h-full w-full items-end justify-center gap-[4vw] px-[5vw] pb-[6vh] transition-transform duration-[60s] ease-linear"
                style={{ transform: `translate(${drift.x}px, ${drift.y}px)` }}
            >
                {data && (
                    <>
                        <Tower
                            bands={bandsOf(data.lighting.colors, palette)}
                            glow={1}
                            className="h-[86vh] w-auto shrink-0"
                        />
                        <div className="flex max-w-[48vw] min-w-0 flex-col gap-[3vh] pb-[2vh]">
                            <div>
                                <p className="text-[2.4vh] text-[#98a1b8]">
                                    <span className="font-bold text-[#eef1f8]">Tonight</span> ·{" "}
                                    {longDate(data.date)}
                                </p>
                                <h1 className="text-[7vh] leading-tight font-bold">
                                    {colorNames(data.lighting.colors, palette)}
                                </h1>
                                <p className="text-[3.6vh] leading-snug">
                                    {data.lighting.scheduled
                                        ? `for ${data.lighting.cause}`
                                        : "No special lighting tonight"}
                                </p>
                                <p className="mt-[1vh] text-[2.2vh] text-[#98a1b8]">
                                    Lights on around {lightsOnTime(data.lighting.lights_on)}
                                </p>
                            </div>
                            <ul className="flex flex-col gap-[1.2vh]">
                                {data.upcoming.map((l) => (
                                    <li key={l.date} className="flex items-center gap-[1.4vh] text-[2.3vh]">
                                        <Crown
                                            bands={bandsOf(l.colors, palette)}
                                            className="h-[4vh] w-[4vh]"
                                        />
                                        <span className="w-[13vh] shrink-0 text-[#98a1b8]">
                                            {relativeName(l.date, data.date) === "Tomorrow"
                                                ? "Tomorrow"
                                                : `${weekdayAbbr(l.date)} ${shortDate(l.date)}`}
                                        </span>
                                        <span className="truncate">
                                            {colorNames(l.colors, palette)}
                                            {l.scheduled && (
                                                <span className="text-[#98a1b8]"> · {l.cause}</span>
                                            )}
                                        </span>
                                    </li>
                                ))}
                            </ul>
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
