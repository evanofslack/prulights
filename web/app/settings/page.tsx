"use client";

import { useState } from "react";
import Link from "next/link";
import { Alert, Button, Checkbox, SegmentedControl, Skeleton, Switch, TextInput } from "@mantine/core";
import { IconPlugConnected, IconPlugConnectedX } from "@tabler/icons-react";
import type { DeviceInfo, DeviceSettings } from "@/interfaces/api";
import { api, useApi } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Shell } from "@/components/Shell";

// /settings is for your frames: what each shows, quiet hours, pause, and
// letting one go.

const THEMES = [
    { value: "dark", label: "Dark" },
    { value: "auto", label: "Dark at night" },
    { value: "light", label: "Light" },
];

const THEME_NOTES: Record<string, string> = {
    dark: "The night sky, all day.",
    auto: "Light by day. Dark from when the crown lights up until sunrise.",
    light: "Black on white, all day.",
};

const SCENES = [
    { value: "tower", label: "Just the tower" },
    { value: "skyline", label: "With the skyline" },
    { value: "river", label: "Across the river" },
];

const WORDS = [
    { value: "none", label: "Picture only" },
    { value: "color", label: "Colors" },
    { value: "cause", label: "And cause" },
];

const VIEWS = [
    { value: "tonight", label: "Tonight", long: "Tonight's colors" },
    { value: "week", label: "Week", long: "The week ahead" },
    { value: "month", label: "Month", long: "The month" },
    { value: "log", label: "Log", long: "Your log" },
];

export default function SettingsPage() {
    const { me } = useAuth();
    const { data, error, reload } = useApi<DeviceInfo[]>(me ? "/me/devices" : null);

    return (
        <Shell>
            <div className="flex flex-col gap-6">
                <h1 className="text-3xl font-bold">Your frame</h1>
                {me === undefined || (me && !data && !error) ? (
                    <Skeleton height={320} radius="lg" />
                ) : me === null ? (
                    <div className="flex flex-col gap-4">
                        <p className="text-lg text-muted">Sign in to set up your frame.</p>
                        <Button
                            component={Link}
                            href="/login?next=/settings"
                            size="lg"
                            className="self-start"
                        >
                            Sign in
                        </Button>
                    </div>
                ) : error ? (
                    <Alert color="red" variant="light">
                        {error.message}
                    </Alert>
                ) : data && data.length === 0 ? (
                    <div className="flex flex-col gap-4 rounded-xl bg-panel p-5">
                        <p className="text-lg">No frame yet.</p>
                        <p className="text-muted">
                            A new frame shows a QR code: scan it with your phone&apos;s camera. Or type the
                            code under it.
                        </p>
                        <Button
                            component={Link}
                            href="/claim"
                            size="lg"
                            leftSection={<IconPlugConnected />}
                            className="self-start"
                        >
                            Enter a code
                        </Button>
                    </div>
                ) : (
                    data?.map((d) => <FrameCard key={d.id} device={d} onChange={reload} />)
                )}
            </div>
        </Shell>
    );
}

function status(d: DeviceInfo): string {
    const parts: string[] = [];
    if (d.last_checkin_at) {
        const mins = Math.round((Date.now() - new Date(d.last_checkin_at).getTime()) / 60_000);
        parts.push(
            mins < 2
                ? "Checked in just now"
                : mins < 90
                  ? `Checked in ${mins} minutes ago`
                  : mins < 48 * 60
                    ? `Checked in ${Math.round(mins / 60)} hours ago`
                    : `Checked in ${Math.round(mins / 1440)} days ago`,
        );
    } else {
        parts.push("Hasn't checked in yet");
    }
    if (d.power === "usb") parts.push("plugged in");
    else if (d.power === "battery" && d.battery_mv > 0)
        parts.push(`on battery${d.battery_mv < 3500 ? ", low" : ""} (${(d.battery_mv / 1000).toFixed(2)} V)`);
    return parts.join(" · ");
}

function FrameCard({ device, onChange }: { device: DeviceInfo; onChange: () => void }) {
    const [name, setName] = useState(device.name);
    const [s, setS] = useState<DeviceSettings>(device.settings);
    const [busy, setBusy] = useState(false);
    const [saved, setSaved] = useState(false);
    const [error, setError] = useState<string>();
    const [confirming, setConfirming] = useState(false);

    const dirty = name !== device.name || JSON.stringify(s) !== JSON.stringify(device.settings);
    const change = (c: Partial<DeviceSettings>) => {
        setS((prev) => ({ ...prev, ...c }));
        setSaved(false);
    };
    const quiet = s.quiet_start !== "" && s.quiet_end !== "";

    const save = async (e: React.FormEvent) => {
        e.preventDefault();
        setBusy(true);
        setError(undefined);
        try {
            await api(`/me/devices/${device.id}`, {
                method: "PATCH",
                body: JSON.stringify({ name, settings: s }),
            });
            setSaved(true);
            onChange();
        } catch (err) {
            setError((err as Error).message);
        } finally {
            setBusy(false);
        }
    };
    const disconnect = async () => {
        setBusy(true);
        try {
            await api(`/me/devices/${device.id}`, { method: "DELETE" });
            onChange();
        } catch (err) {
            setError((err as Error).message);
            setBusy(false);
        }
    };

    return (
        <form
            onSubmit={save}
            aria-label={`Frame ${device.name}`}
            className="flex flex-col gap-6 rounded-xl bg-panel p-5"
        >
            <div className="flex flex-col gap-1">
                <TextInput
                    size="md"
                    label="Name"
                    value={name}
                    onChange={(e) => {
                        setName(e.currentTarget.value);
                        setSaved(false);
                    }}
                    maxLength={40}
                    required
                />
                <p className="text-muted">{status(device)}</p>
            </div>

            <Looks device={device} s={s} change={change} />

            <Checkbox.Group
                label="The button steps through"
                size="md"
                value={s.views}
                onChange={(v) => change({ views: VIEWS.map((x) => x.value).filter((x) => v.includes(x)) })}
                error={s.views.length === 0 ? "Pick at least one" : undefined}
            >
                <div className="mt-2 grid gap-3 sm:grid-cols-2">
                    {VIEWS.map((v) => (
                        <Checkbox key={v.value} value={v.value} label={v.long} />
                    ))}
                </div>
            </Checkbox.Group>

            <div className="flex flex-col gap-2">
                <span className="font-bold">Shows first</span>
                <SegmentedControl
                    size="md"
                    value={s.default_view}
                    onChange={(v) => change({ default_view: v })}
                    data={VIEWS.map((v) => ({ value: v.value, label: v.label }))}
                    aria-label="Shows first"
                />
                <p className="text-sm text-muted">After it updates, and when the night changes.</p>
            </div>

            <div className="flex flex-col gap-3">
                <Switch
                    size="md"
                    label="Quiet at night"
                    description="Shows “Good night” and doesn't change."
                    checked={quiet}
                    onChange={(e) =>
                        change(
                            e.currentTarget.checked
                                ? { quiet_start: "22:00", quiet_end: "07:00" }
                                : { quiet_start: "", quiet_end: "" },
                        )
                    }
                />
                {quiet && (
                    <div className="flex gap-3">
                        <TextInput
                            type="time"
                            size="md"
                            label="From"
                            value={s.quiet_start}
                            onChange={(e) => change({ quiet_start: e.currentTarget.value })}
                        />
                        <TextInput
                            type="time"
                            size="md"
                            label="Until"
                            value={s.quiet_end}
                            onChange={(e) => change({ quiet_end: e.currentTarget.value })}
                        />
                    </div>
                )}
                <Switch
                    size="md"
                    label="Pause"
                    description="Keeps the picture it has and checks in once a day."
                    checked={s.paused}
                    onChange={(e) => change({ paused: e.currentTarget.checked })}
                />
            </div>

            <div className="flex flex-col gap-2">
                <Button type="submit" size="lg" loading={busy} disabled={!dirty || s.views.length === 0}>
                    Save
                </Button>
                {saved && !dirty && (
                    <p role="status" className="text-muted">
                        Saved. The frame picks it up the next time it checks in, or press its button twice to
                        update it now.
                    </p>
                )}
                {error && (
                    <Alert color="red" variant="light">
                        {error}
                    </Alert>
                )}
            </div>

            <div className="flex flex-wrap items-center gap-3 border-t border-line pt-4">
                {confirming ? (
                    <>
                        <span>It goes back to its setup screen. Disconnect it?</span>
                        <Button color="red" onClick={disconnect} loading={busy}>
                            Disconnect
                        </Button>
                        <Button variant="default" onClick={() => setConfirming(false)}>
                            Keep it
                        </Button>
                    </>
                ) : (
                    <Button
                        variant="subtle"
                        color="red"
                        leftSection={<IconPlugConnectedX size={18} />}
                        onClick={() => setConfirming(true)}
                    >
                        Disconnect this frame
                    </Button>
                )}
            </div>
        </form>
    );
}

// preview is the frame's picture with choices that may not be saved yet.
function preview(id: number, s: DeviceSettings, o: { light: boolean; scene?: string; words?: string }) {
    const q = new URLSearchParams({
        view: "tonight",
        scene: o.scene ?? s.scene,
        words: o.words ?? s.words,
        ...(o.light ? { light: "1" } : {}),
    });
    return `/api/v1/me/devices/${id}/preview.png?${q}`;
}

// Looks: how tonight's picture is drawn, chosen by looking at it.
function Looks({
    device,
    s,
    change,
}: {
    device: DeviceInfo;
    s: DeviceSettings;
    change: (c: Partial<DeviceSettings>) => void;
}) {
    // the picture the choices are drawn on: dark, unless it's always light
    const light = s.theme === "light";
    return (
        <fieldset className="flex flex-col gap-5">
            <legend className="mb-3 text-xl font-bold">How it looks</legend>

            <div className="flex flex-col gap-2">
                <span className="font-bold" id={`theme-${device.id}`}>
                    Colors
                </span>
                <SegmentedControl
                    size="md"
                    value={s.theme}
                    onChange={(v) => change({ theme: v })}
                    data={THEMES}
                    fullWidth
                    aria-labelledby={`theme-${device.id}`}
                />
                <p className="text-sm text-muted">{THEME_NOTES[s.theme]}</p>
            </div>

            <div className="flex flex-col gap-2">
                <span className="font-bold" id={`scene-${device.id}`}>
                    Picture
                </span>
                <div
                    role="radiogroup"
                    aria-labelledby={`scene-${device.id}`}
                    className="grid grid-cols-3 gap-3"
                >
                    {SCENES.map((o) => {
                        const on = s.scene === o.value;
                        return (
                            <button
                                key={o.value}
                                type="button"
                                role="radio"
                                aria-checked={on}
                                onClick={() => change({ scene: o.value })}
                                className={`flex flex-col items-center gap-2 rounded-lg p-2 text-center transition-colors ${
                                    on ? "bg-raised ring-2 ring-accent" : "hover:bg-raised"
                                }`}
                            >
                                {/* eslint-disable-next-line @next/next/no-img-element */}
                                <img
                                    src={preview(device.id, s, { light, scene: o.value, words: "none" })}
                                    alt=""
                                    width={480}
                                    height={800}
                                    className="aspect-[3/5] w-full rounded-sm bg-black"
                                />
                                <span className={on ? "font-bold" : ""}>{o.label}</span>
                            </button>
                        );
                    })}
                </div>
            </div>

            <div className="flex flex-col gap-2">
                <span className="font-bold" id={`words-${device.id}`}>
                    Words under the tower
                </span>
                <SegmentedControl
                    size="md"
                    value={s.words}
                    onChange={(v) => change({ words: v })}
                    data={WORDS}
                    fullWidth
                    aria-labelledby={`words-${device.id}`}
                />
                <p className="text-sm text-muted">
                    {s.words === "none"
                        ? "Just the tower."
                        : s.words === "color"
                          ? "The date and tonight's colors."
                          : "The date, tonight's colors, and what they're for."}
                </p>
            </div>

            <figure className="flex flex-col gap-2">
                <div className="flex gap-4">
                    {(s.theme === "auto" ? [true, false] : [light]).map((l) => (
                        <div key={String(l)} className="flex flex-1 flex-col items-center gap-2 sm:flex-none">
                            {/* eslint-disable-next-line @next/next/no-img-element */}
                            <img
                                src={preview(device.id, s, { light: l })}
                                alt={`Tonight on the frame${s.theme === "auto" ? (l ? ", by day" : ", at night") : ""}`}
                                width={480}
                                height={800}
                                className="aspect-[3/5] w-full max-w-[240px] rounded-md border-[10px] border-[#1b1d22] bg-black shadow-lg"
                            />
                            {s.theme === "auto" && (
                                <span className="text-sm text-muted">{l ? "By day" : "At night"}</span>
                            )}
                        </div>
                    ))}
                </div>
                <figcaption className="text-sm text-muted">
                    Tonight, as the frame will show it once you save.
                </figcaption>
            </figure>
        </fieldset>
    );
}
