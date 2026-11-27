"use client";

import { useState } from "react";
import Link from "next/link";
import {
    Alert,
    Button,
    Checkbox,
    SegmentedControl,
    Skeleton,
    Switch,
    Textarea,
    TextInput,
} from "@mantine/core";
import { IconCheck, IconPlugConnected, IconPlugConnectedX, IconSend } from "@tabler/icons-react";
import type {
    DeviceInfo,
    DeviceSettings,
    DeviceUpdate,
    FeedbackRequest,
    PrudleSettings,
} from "@/interfaces/api";
import { api, json, useApi } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { usePrudle } from "@/lib/blackout";
import { Shell } from "@/components/Shell";

// /settings is everyone's settings: your name, whether you play Prudle in
// December, and, once you have one, your frames (what each shows, quiet hours,
// pause, and letting one go). Ideas for the site go in the box at the bottom.

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
    // frames stay a surprise: the section only shows once you have one (an
    // admin sees it always, to set them up)
    const frames = !!me && (me.is_admin || !!data?.length);

    return (
        <Shell>
            <div className="flex flex-col gap-6">
                <h1 className="text-3xl font-bold">Settings</h1>
                {me === undefined ? (
                    <Skeleton height={120} radius="lg" />
                ) : me === null ? (
                    <div className="flex flex-col gap-4">
                        <p className="text-lg text-muted">Sign in to change your settings.</p>
                        <Button
                            component={Link}
                            href="/login?next=/settings"
                            size="lg"
                            className="self-start"
                        >
                            Sign in
                        </Button>
                    </div>
                ) : (
                    <>
                        <NameSetting />
                        <PrudleSetting />
                    </>
                )}
                {frames && (
                    <section aria-label="Your frame" className="flex flex-col gap-6">
                        <h2 className="text-2xl font-bold">Your frame</h2>
                        {error ? (
                            <Alert color="red" variant="light">
                                {error.message}
                            </Alert>
                        ) : data && data.length === 0 ? (
                            <div className="flex flex-col gap-4 rounded-xl bg-panel p-5">
                                <p className="text-lg">No frame yet.</p>
                                <p className="text-muted">
                                    A new frame shows a QR code: scan it with your phone&apos;s camera. Or
                                    type the code under it.
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
                        ) : data ? (
                            data.map((d) => <FrameCard key={d.id} device={d} onChange={reload} />)
                        ) : (
                            <Skeleton height={320} radius="lg" />
                        )}
                    </section>
                )}
                {me && <Ideas />}
            </div>
        </Shell>
    );
}

// NameSetting changes the name everyone sees on sightings and Prudle. What
// you logged stays yours.
function NameSetting() {
    const { me, rename } = useAuth();
    const [name, setName] = useState(me?.name ?? "");
    const [saving, setSaving] = useState(false);
    const [saved, setSaved] = useState(false);
    const [error, setError] = useState("");
    const dirty = name.trim() !== "" && name.trim() !== me?.name;
    const save = async (e: React.FormEvent) => {
        e.preventDefault();
        setSaving(true);
        setError("");
        try {
            const u = await rename(name);
            setName(u.name);
            setSaved(true);
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setSaving(false);
        }
    };
    return (
        <section aria-label="Your name" className="flex flex-col gap-3 rounded-xl bg-panel p-5">
            <h2 className="text-2xl font-bold">Your name</h2>
            <form onSubmit={save} className="flex flex-col gap-3">
                <TextInput
                    size="lg"
                    label="The name your family sees"
                    value={name}
                    maxLength={40}
                    onChange={(e) => {
                        setName(e.currentTarget.value);
                        setSaved(false);
                    }}
                />
                <div className="flex items-center gap-3">
                    <Button type="submit" size="lg" loading={saving} disabled={!dirty}>
                        Save name
                    </Button>
                    {saved && !dirty && (
                        <span className="flex items-center gap-1 text-muted">
                            <IconCheck size={18} /> Saved
                        </span>
                    )}
                </div>
            </form>
            {error && (
                <Alert color="red" variant="light">
                    {error}
                </Alert>
            )}
        </section>
    );
}

// Ideas is the feedback box: anything you'd like the site to do. The admins
// read them in /admin.
function Ideas() {
    const [text, setText] = useState("");
    const [sending, setSending] = useState(false);
    const [sent, setSent] = useState(false);
    const [error, setError] = useState("");
    const send = async (e: React.FormEvent) => {
        e.preventDefault();
        setSending(true);
        setError("");
        try {
            await api("/me/feedback", { method: "POST", body: json<FeedbackRequest>({ text }) });
            setText("");
            setSent(true);
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setSending(false);
        }
    };
    return (
        <section
            id="ideas"
            aria-label="Ideas"
            className="flex scroll-mt-24 flex-col gap-3 rounded-xl bg-panel p-5"
        >
            <h2 className="text-2xl font-bold">Have an idea?</h2>
            <p className="text-muted">
                Something you&apos;d like the site to do, or something that bugs you.
            </p>
            <form onSubmit={send} className="flex flex-col gap-3">
                <Textarea
                    size="lg"
                    aria-label="Your idea"
                    placeholder="I wish it would…"
                    autosize
                    minRows={3}
                    maxLength={2000}
                    value={text}
                    onChange={(e) => {
                        setText(e.currentTarget.value);
                        setSent(false);
                    }}
                />
                <div className="flex items-center gap-3">
                    <Button
                        type="submit"
                        size="lg"
                        loading={sending}
                        disabled={text.trim() === ""}
                        leftSection={<IconSend size={20} />}
                    >
                        Send
                    </Button>
                    {sent && (
                        <span className="flex items-center gap-1 text-muted">
                            <IconCheck size={18} /> Thanks, we got it.
                        </span>
                    )}
                </div>
            </form>
            {error && (
                <Alert color="red" variant="light">
                    {error}
                </Alert>
            )}
        </section>
    );
}

// PrudleSetting turns the December game (and its hidden colors) on or off.
function PrudleSetting() {
    const { summary, reload } = usePrudle();
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState("");
    const set = async (play: boolean) => {
        setSaving(true);
        setError("");
        try {
            await api("/me/prudle/settings", { method: "PUT", body: json<PrudleSettings>({ off: !play }) });
            reload();
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setSaving(false);
        }
    };
    return (
        <section aria-label="Prudle" className="flex flex-col gap-3 rounded-xl bg-panel p-5">
            <h2 className="text-2xl font-bold">Prudle</h2>
            {summary ? (
                <Switch
                    size="lg"
                    checked={!summary.off}
                    disabled={saving}
                    onChange={(e) => set(e.currentTarget.checked)}
                    label="Play Prudle in December"
                    description="Each December night, guess the crown's colors before you see them. Off shows every color."
                />
            ) : (
                <Skeleton height={48} />
            )}
            {error && (
                <Alert color="red" variant="light">
                    {error}
                </Alert>
            )}
        </section>
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
                body: json<DeviceUpdate>({ name, settings: s }),
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
