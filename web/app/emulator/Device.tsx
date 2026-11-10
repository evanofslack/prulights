"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
    Button,
    FileButton,
    NumberInput,
    PasswordInput,
    Radio,
    SegmentedControl,
    Slider,
    Switch,
    TextInput,
} from "@mantine/core";
import {
    IconCheck,
    IconExternalLink,
    IconFileUpload,
    IconPlugConnected,
    IconTrash,
    IconWifi,
} from "@tabler/icons-react";
import type {
    Bundle,
    FrameConfig,
    DevInks,
    DeviceEventIn,
    DeviceSettings,
    DeviceState,
    RegisterResponse,
} from "@/interfaces/api";
import { longDate } from "@/lib/format";
import {
    HEIGHT,
    POWER,
    WIDTH,
    clockAt,
    cycle,
    daysOnCharge,
    decodeFrame,
    paint,
    pick,
    tonightAt,
    type Picture,
} from "@/lib/frame";

// Device mode: a virtual frame. It registers, waits to be claimed, checks in
// on the schedule the server gives it, downloads the pictures it's missing
// and decodes them the way the firmware will, and answers the button: short
// for the next view (no Wi-Fi), long to log "seen" and sync, double to sync.
// The token stays in localStorage; everything else is in the url or memory.

export interface DeviceParams {
    power: "usb" | "battery";
    mv: number;
    offline: boolean;
    clock: string; // Boston wall time "2026-12-12T19:00", "" for now
    slow: boolean;
}

const FIRMWARE = "emulator";
const STORE = "prulights.emulator.device";
const RETRY_MS = 10 * 60_000;
const FAST_REFRESH_MS = 1200;
const SLOW_REFRESH_MS = 20_000;

interface Creds {
    token: string;
    id: number;
    ap: { ssid: string; password: string };
    networks: string[]; // the Wi-Fi it knows, as flashed or added on its setup page
}

const MAX_NETWORKS = 5;
const NEARBY = ["Home", "Cape House", "Linden-5G", "xfinitywifi"];
const FAILS_BEFORE_SETUP = 2;

interface Line {
    at: number;
    text: string;
}

function loadCreds(): Creds | undefined {
    try {
        const v = JSON.parse(localStorage.getItem(STORE) ?? "null");
        return v?.token ? { ap: newHotspot(), networks: ["Home"], ...v } : undefined;
    } catch {
        return undefined;
    }
}

function saveCreds(c: Creds | undefined) {
    try {
        if (c) localStorage.setItem(STORE, JSON.stringify(c));
        else localStorage.removeItem(STORE);
    } catch {}
}

const ALPHABET = "ACDEFGHJKMNPQRTUVWXY34679";
const random = (n: number) =>
    Array.from(crypto.getRandomValues(new Uint8Array(n)), (b) => ALPHABET[b % ALPHABET.length]).join("");

// newHotspot is the frame's own setup network, as firmware would make it.
function newHotspot() {
    return { ssid: `prulights-${random(4)}`, password: random(10).toLowerCase() };
}

// bostonToMs reads a Boston wall time ("2026-12-12T19:00") as an instant.
export function bostonToMs(wall: string): number {
    const asUTC = Date.parse(`${wall}:00Z`);
    const parts = new Intl.DateTimeFormat("en-CA", {
        timeZone: "America/New_York",
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        hourCycle: "h23",
    }).formatToParts(new Date(asUTC));
    const get = (t: string) => parts.find((p) => p.type === t)?.value ?? "00";
    const shown = Date.parse(
        `${get("year")}-${get("month")}-${get("day")}T${get("hour")}:${get("minute")}:00Z`,
    );
    return asUTC - (shown - asUTC);
}

// msToBoston is the Boston wall time of an instant, for the clock input.
function msToBoston(ms: number): string {
    return `${tonightAt(ms + 4 * 3_600_000)}T${clockAt(ms)}`;
}

class Unauthorized extends Error {}

async function deviceFetch(token: string, path: string, init?: RequestInit): Promise<Response> {
    const res = await fetch(`/api/device/v1${path}`, {
        ...init,
        headers: {
            Authorization: `Bearer ${token}`,
            ...(init?.body ? { "Content-Type": "application/json" } : {}),
        },
    });
    if (res.status === 401) throw new Unauthorized();
    if (!res.ok && res.status !== 304) {
        let msg = res.statusText;
        try {
            msg = (await res.json()).error ?? msg;
        } catch {}
        throw new Error(`${res.status} ${msg}`);
    }
    return res;
}

const ago = (ms: number) => {
    const m = Math.round(ms / 60_000);
    if (m < 1) return "just now";
    if (m < 90) return `${m} min ago`;
    return `${Math.round(m / 60)} h ago`;
};
const until = (ms: number) => {
    const s = Math.max(0, Math.round(ms / 1000));
    if (s < 90) return `${s} s`;
    if (s < 90 * 60) return `${Math.round(s / 60)} min`;
    return `${(s / 3600).toFixed(1)} h`;
};

export function Device({
    p,
    set,
    palette,
    width,
}: {
    p: DeviceParams;
    set: (change: Partial<DeviceParams>) => void;
    palette: "measured" | "ideal";
    width: number;
}) {
    const [inks, setInks] = useState<DevInks>();
    const [creds, setCreds] = useState<Creds>();
    const [state, setState] = useState<DeviceState>();
    const [bundle, setBundle] = useState<Bundle>();
    const [picsVersion, setPicsVersion] = useState(0);
    const [view, setView] = useState("tonight");
    const [queue, setQueue] = useState<DeviceEventIn[]>([]);
    const [log, setLog] = useState<Line[]>([]);
    const [syncing, setSyncing] = useState(false);
    const [lastSync, setLastSync] = useState<number>();
    const [nextAt, setNextAt] = useState<number>();
    const [shown, setShown] = useState<string>();
    const [flashing, setFlashing] = useState(false);
    const [feedback, setFeedback] = useState<string>();
    const [usage, setUsage] = useState({ syncs: 0, refreshes: 0, wakes: 0 });
    const [pressesPerDay, setPressesPerDay] = useState(4);
    const [frozen, setFrozen] = useState<number>(); // a paused frame's moment
    // wifi: no known network answered ("lost"), or the button was held to
    // add one ("setup"); either way the frame shows its Wi-Fi screen
    const [wifi, setWifi] = useState<"lost" | "setup">();
    const fails = useRef(0);

    const pics = useRef(new Map<string, Picture>());
    const canvas = useRef<HTMLCanvasElement>(null);

    // the emulated clock: real time, or the time set in the url running on
    // from when it was set
    const base = useRef({ wall: 0, real: 0 });
    useEffect(() => {
        base.current = p.clock ? { wall: bostonToMs(p.clock), real: Date.now() } : { wall: 0, real: 0 };
    }, [p.clock]);
    const clock = useCallback(
        () => (base.current.wall ? base.current.wall + (Date.now() - base.current.real) : Date.now()),
        [],
    );
    const [now, setNow] = useState(() => Date.now());

    // the latest values, for the ticker and async work
    const live = useRef({ p, creds, bundle, queue, syncing, nextAt, inks });
    live.current = { p, creds, bundle, queue, syncing, nextAt, inks };

    const note = useCallback(
        (text: string) => setLog((l) => [{ at: clock(), text }, ...l].slice(0, 200)),
        [clock],
    );

    useEffect(() => {
        fetch("/dev/inks")
            .then((r) => r.json())
            .then(setInks)
            .catch(() => note("couldn't load /dev/inks"));
        setCreds(loadCreds());
    }, [note]);

    const download = useCallback(async (token: string, b: Bundle, ink: DevInks) => {
        const missing = b.items.filter((i) => !pics.current.has(i.sha256));
        const unique = [...new Map(missing.map((i) => [i.sha256, i])).values()];
        let bytes = 0;
        for (const item of unique) {
            const res = await deviceFetch(token, item.url.replace(/^\/api\/device\/v1/, ""));
            const buf = await res.arrayBuffer();
            bytes += buf.byteLength;
            pics.current.set(item.sha256, await decodeFrame(buf, ink));
        }
        setPicsVersion((v) => v + 1);
        return { count: unique.length, bytes };
    }, []);

    const register = useCallback(async () => {
        const ap = live.current.creds?.ap ?? newHotspot();
        const res = await fetch("/api/device/v1/register", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ firmware: FIRMWARE, ap_ssid: ap.ssid, ap_password: ap.password }),
        });
        if (!res.ok) throw new Error(`register: ${res.status}`);
        const r: RegisterResponse = await res.json();
        const c: Creds = {
            token: r.token,
            id: r.device_id,
            ap,
            networks: live.current.creds?.networks ?? ["Home"],
        };
        saveCreds(c);
        setCreds(c);
        live.current.creds = c;
        note(`registered as frame #${r.device_id}, claim code ${r.claim_code}`);
        return c;
    }, [note]);

    // sync is a check-in: send the queued events and the battery, then fetch
    // the bundle if it changed and download the pictures it doesn't have.
    const sync = useCallback(
        async (reason: string, extra: DeviceEventIn[] = []) => {
            const { p, creds, bundle, queue, syncing, inks } = live.current;
            const events = [...queue, ...extra];
            if (!creds || !inks) return;
            if (syncing) {
                setQueue(events);
                return;
            }
            if (p.offline) {
                setQueue(events);
                setNextAt(clock() + RETRY_MS);
                fails.current++;
                note(
                    `${reason}: no known network answers (${creds.networks.join(", ")}), keeping ${events.length} event${events.length === 1 ? "" : "s"} for later`,
                );
                if (fails.current >= FAILS_BEFORE_SETUP) {
                    setWifi((w) => w ?? "lost");
                    note(`showing the Wi-Fi screen; hotspot ${creds.ap.ssid} open for 10 min`);
                }
                return;
            }
            setSyncing(true);
            live.current.syncing = true;
            try {
                const res = await deviceFetch(creds.token, "/events", {
                    method: "POST",
                    body: JSON.stringify({
                        events,
                        battery_mv: p.power === "battery" ? p.mv : 0,
                        power: p.power,
                        firmware: FIRMWARE,
                        ap_ssid: creds.ap.ssid,
                        ap_password: creds.ap.password,
                    }),
                });
                const st: DeviceState = await res.json();
                fails.current = 0;
                setWifi((w) => (w === "lost" ? undefined : w));
                setQueue([]);
                setState(st);
                setUsage((u) => ({ ...u, syncs: u.syncs + 1 }));
                let got = "";
                if (st.bundle_version !== bundle?.version) {
                    const b = await deviceFetch(
                        creds.token,
                        `/bundle${bundle ? `?have=${bundle.version}` : ""}`,
                    );
                    if (b.status === 200) {
                        const nb: Bundle = await b.json();
                        const d = await download(creds.token, nb, inks);
                        setBundle(nb);
                        got = `, new bundle ${nb.version.slice(0, 6)}: ${nb.items.length} pictures, downloaded ${d.count} (${Math.round(d.bytes / 1024)} KB)`;
                    }
                }
                setView(st.settings.default_view);
                const at = clock();
                setLastSync(at);
                setFrozen(at);
                setNextAt(at + st.next_checkin_s * 1000);
                note(
                    `synced (${reason}), sent ${events.length} event${events.length === 1 ? "" : "s"}${got || ", bundle unchanged"}; next in ${until(st.next_checkin_s * 1000)}`,
                );
            } catch (e) {
                if (e instanceof Unauthorized) {
                    note("the server doesn't know this frame any more: registering again");
                    saveCreds(undefined);
                    setState(undefined);
                    setBundle(undefined);
                    setQueue([]);
                    try {
                        await register();
                        live.current.syncing = false;
                        setSyncing(false);
                        await sync("re-registered", [{ type: "boot", at: new Date(clock()).toISOString() }]);
                    } catch (err) {
                        note(`register failed: ${(err as Error).message}`);
                    }
                    return;
                }
                setQueue(events);
                setNextAt(clock() + RETRY_MS);
                note(`${reason}: sync failed (${(e as Error).message}), retrying in 10 min`);
            } finally {
                live.current.syncing = false;
                setSyncing(false);
            }
        },
        [clock, download, note, register],
    );

    // boot: once there's a token and the inks, check in
    const booted = useRef(false);
    useEffect(() => {
        if (booted.current || !creds || !inks) return;
        booted.current = true;
        sync("boot", [{ type: "boot", at: new Date(clock()).toISOString() }]);
    }, [creds, inks, sync, clock]);

    // the clock ticks; when the check-in is due, sync
    useEffect(() => {
        const id = setInterval(() => {
            const t = clock();
            setNow(t);
            const { nextAt, creds, syncing } = live.current;
            if (creds && nextAt && t >= nextAt && !syncing) sync("scheduled");
        }, 1000);
        return () => clearInterval(id);
    }, [clock, sync]);
    useEffect(() => setNow(clock()), [p.clock, clock]);

    const settings: DeviceSettings | undefined = state?.settings;
    const claimed = !!state?.claimed;
    // a paused frame keeps its picture until the button or a sync moves it
    const at = settings?.paused && frozen ? frozen : now;
    const target = pick(bundle, claimed, settings, view, at, !!wifi);

    // redraw only when the picture changes, with a flash like the panel's
    useEffect(() => {
        if (!target || target.sha256 === shown || !pics.current.has(target.sha256)) return;
        setFlashing(true);
        const ms = live.current.p.slow ? SLOW_REFRESH_MS : FAST_REFRESH_MS;
        const id = setTimeout(() => {
            setShown(target.sha256);
            setFlashing(false);
            setUsage((u) => ({ ...u, refreshes: u.refreshes + 1 }));
            note(
                `redraw: ${target.stale ? "out-of-date " : ""}${target.view}${target.date ? ` for ${target.date}` : ""}`,
            );
        }, ms);
        return () => clearTimeout(id);
    }, [target, shown, picsVersion, note]);

    useEffect(() => {
        const pic = shown && pics.current.get(shown);
        if (pic && inks && canvas.current) paint(canvas.current, pic, inks, palette);
    }, [shown, inks, palette, picsVersion]);

    const stamp = () => new Date(clock()).toISOString();
    const short = () => {
        setUsage((u) => ({ ...u, wakes: u.wakes + 1 }));
        setQueue((q) => [...q, { type: "short", at: stamp() }]);
        setFrozen(clock());
        if (!settings || !claimed) return note("short press: nothing to cycle until the frame is claimed");
        const order = cycle(settings);
        const next = order[(order.indexOf(view) + 1) % order.length];
        setView(next);
        note(`short press: ${next}`);
    };
    const long = () => {
        setUsage((u) => ({ ...u, wakes: u.wakes + 1 }));
        const night = tonightAt(clock());
        setFeedback(claimed ? "Logged tonight" : "Not set up yet");
        setTimeout(() => setFeedback(undefined), 2500);
        note(`long press: seen ${night}`);
        sync("long press", [
            { type: "long", at: stamp() },
            { type: "seen", at: stamp(), date: night },
        ]);
    };
    const double = () => {
        setUsage((u) => ({ ...u, wakes: u.wakes + 1 }));
        note("double press: sync now");
        sync("double press", [{ type: "double", at: stamp() }]);
    };
    const hold = () => {
        setUsage((u) => ({ ...u, wakes: u.wakes + 1 }));
        setWifi("setup");
        note(`held 8 s: Wi-Fi setup, hotspot ${creds?.ap.ssid} open`);
    };
    // join is the setup page's "Join": the frame learns the network and,
    // since it answers, gets back online
    const join = (ssid: string) => {
        const c = live.current.creds;
        if (!c) return;
        const networks = [...c.networks.filter((n) => n !== ssid), ssid].slice(-MAX_NETWORKS);
        const next = { ...c, networks };
        saveCreds(next);
        setCreds(next);
        live.current.creds = next;
        live.current.p = { ...live.current.p, offline: false };
        set({ offline: false });
        fails.current = 0;
        setWifi(undefined);
        note(`setup page: joined ${ssid}, hotspot closed`);
        sync(`joined ${ssid}`);
    };
    // loadPrepared reads frame-<id>.json (or a flashed config.json): the
    // emulator becomes that frame, as if flashed with it
    const loadPrepared = async (file: File | null) => {
        if (!file) return;
        try {
            const fc: FrameConfig & { wifi?: { ssid: string }[] } = JSON.parse(await file.text());
            if (fc.version !== 1 || !fc.token || !fc.device_id) throw new Error("not a frame identity");
            forget();
            const c: Creds = {
                token: fc.token,
                id: fc.device_id,
                ap: fc.ap?.ssid ? fc.ap : newHotspot(),
                networks: fc.wifi?.length ? fc.wifi.map((w) => w.ssid) : ["Home"],
            };
            saveCreds(c);
            setCreds(c);
            live.current.creds = c;
            note(`flashed with ${file.name}: frame #${fc.device_id}, code ${fc.claim_code}`);
        } catch (e) {
            note(`couldn't load ${file.name}: ${(e as Error).message}`);
        }
    };
    const forget = () => {
        saveCreds(undefined);
        setCreds(undefined);
        setState(undefined);
        setBundle(undefined);
        setShown(undefined);
        setQueue([]);
        setNextAt(undefined);
        setWifi(undefined);
        fails.current = 0;
        booted.current = false;
        pics.current.clear();
        note("forgot this frame (the server still lists it)");
    };

    const showing = shown && bundle?.items.find((i) => i.sha256 === shown);
    const perDay = {
        syncs: p.power === "usb" ? 24 : 1,
        refreshes: 2 + pressesPerDay,
        wakes: pressesPerDay,
    };

    return (
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start">
            <aside className="flex w-full shrink-0 flex-col gap-5 rounded-xl bg-panel p-4 lg:sticky lg:top-4 lg:w-80">
                <Field label="Frame">
                    {!creds ? (
                        <div className="flex flex-col gap-2">
                            <Button
                                leftSection={<IconPlugConnected size={18} />}
                                onClick={() => register().catch((e) => note(String(e)))}
                            >
                                Register a frame
                            </Button>
                            <FileButton onChange={loadPrepared} accept="application/json,.json">
                                {(props) => (
                                    <Button
                                        variant="default"
                                        leftSection={<IconFileUpload size={18} />}
                                        {...props}
                                    >
                                        Load a prepared frame
                                    </Button>
                                )}
                            </FileButton>
                            <p className="text-xs text-subtle">
                                A prepared frame is <code>frame-&lt;id&gt;.json</code> from the admin page, or
                                a flashed <code>config.json</code>.
                            </p>
                        </div>
                    ) : (
                        <div className="flex flex-col gap-2 text-sm" data-device={creds.id}>
                            <span>
                                Frame #{creds.id} ·{" "}
                                {claimed ? (
                                    <b data-claimed>{state?.name || "claimed"}</b>
                                ) : state ? (
                                    <>
                                        waiting, code{" "}
                                        <b data-claim-code={state.claim_code}>{state.claim_code}</b>
                                    </>
                                ) : (
                                    "…"
                                )}
                            </span>
                            {state && !claimed && state.claim_code && (
                                <a
                                    href={`/claim?code=${state.claim_code}`}
                                    target="_blank"
                                    rel="noopener"
                                    className="inline-flex items-center gap-1 underline hover:text-fg"
                                >
                                    Claim it <IconExternalLink size={14} />
                                </a>
                            )}
                            <Button
                                size="xs"
                                variant="subtle"
                                color="red"
                                leftSection={<IconTrash size={14} />}
                                onClick={forget}
                                className="self-start"
                            >
                                Forget this frame
                            </Button>
                        </div>
                    )}
                </Field>

                <Field label="Clock" hint="Boston">
                    <TextInput
                        type="datetime-local"
                        aria-label="Clock"
                        value={msToBoston(now)}
                        onChange={(e) =>
                            e.currentTarget.value && set({ clock: e.currentTarget.value.slice(0, 16) })
                        }
                    />
                    <div className="flex flex-wrap gap-1.5">
                        <Button
                            size="xs"
                            variant="default"
                            onClick={() => set({ clock: msToBoston(clock() + 3_600_000) })}
                        >
                            +1 hour
                        </Button>
                        <Button
                            size="xs"
                            variant="default"
                            onClick={() => set({ clock: msToBoston(clock() + 86_400_000) })}
                        >
                            +1 day
                        </Button>
                        {p.clock && (
                            <Button size="xs" variant="default" onClick={() => set({ clock: "" })}>
                                Now
                            </Button>
                        )}
                    </div>
                    <p className="text-xs text-subtle">
                        The frame&apos;s own clock: it picks tonight&apos;s picture from its bundle. The
                        server still decides what the bundle holds.
                    </p>
                </Field>

                <Field label="Power">
                    <SegmentedControl
                        fullWidth
                        size="xs"
                        value={p.power}
                        onChange={(v) => set({ power: v as DeviceParams["power"] })}
                        data={[
                            { value: "usb", label: "USB" },
                            { value: "battery", label: "Battery" },
                        ]}
                    />
                    {p.power === "battery" && (
                        <div className="flex flex-col gap-1">
                            <span className="text-sm text-muted">
                                Battery {(p.mv / 1000).toFixed(2)} V{p.mv < 3500 ? " (low)" : ""}
                            </span>
                            <Slider
                                aria-label="Battery voltage"
                                min={3300}
                                max={4200}
                                step={50}
                                value={p.mv}
                                onChange={(mv) => set({ mv })}
                                label={(v) => `${(v / 1000).toFixed(2)} V`}
                            />
                        </div>
                    )}
                </Field>

                <Field label="Wi-Fi">
                    {creds && (
                        <p className="text-sm text-muted">
                            Knows {creds.networks.join(", ")} · hotspot {creds.ap.ssid}
                        </p>
                    )}
                    <Switch
                        label="Out of range of its networks"
                        checked={p.offline}
                        onChange={(e) => set({ offline: e.currentTarget.checked })}
                    />
                    <Switch
                        label="Slow refresh (~20 s, like the panel)"
                        checked={p.slow}
                        onChange={(e) => set({ slow: e.currentTarget.checked })}
                    />
                </Field>
            </aside>

            <section className="flex min-w-0 flex-1 flex-col gap-5">
                <div className="flex flex-wrap items-start gap-6">
                    <figure className="flex shrink-0 flex-col items-center gap-3">
                        <div className="relative rounded-[14px] bg-[#16181d] p-[18px] shadow-xl ring-1 ring-white/10">
                            <canvas
                                ref={canvas}
                                width={WIDTH}
                                height={HEIGHT}
                                aria-label="The frame"
                                data-showing={
                                    showing
                                        ? `${showing.view} ${showing.date}${showing.theme ? ` ${showing.theme}` : ""}${showing.stale ? " stale" : ""}`
                                        : ""
                                }
                                className="block h-auto max-w-none bg-[#bec8c8]"
                                style={{ width, imageRendering: width > WIDTH ? "pixelated" : "auto" }}
                            />
                            {flashing && (
                                <div
                                    aria-hidden
                                    className="eink-refresh pointer-events-none absolute inset-[18px]"
                                    style={{
                                        animationDuration: `${p.slow ? SLOW_REFRESH_MS : FAST_REFRESH_MS}ms`,
                                    }}
                                />
                            )}
                            {feedback && (
                                <div className="absolute inset-x-0 top-1/2 mx-auto flex w-max -translate-y-1/2 items-center gap-2 rounded-full bg-black/80 px-4 py-2 font-bold text-white">
                                    <IconCheck size={18} /> {feedback}
                                </div>
                            )}
                        </div>
                        <div className="flex gap-2" role="group" aria-label="The frame's button">
                            <Button variant="default" onClick={short} disabled={!creds}>
                                Short press
                            </Button>
                            <Button variant="default" onClick={long} disabled={!creds}>
                                Long press
                            </Button>
                            <Button variant="default" onClick={double} disabled={!creds} loading={syncing}>
                                Double press
                            </Button>
                            <Button variant="default" onClick={hold} disabled={!creds}>
                                Hold 8 s
                            </Button>
                        </div>
                        {wifi && creds && (
                            <SetupPage creds={creds} onJoin={join} onClose={() => setWifi(undefined)} />
                        )}
                    </figure>

                    <div className="flex min-w-64 flex-1 flex-col gap-4">
                        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 rounded-xl bg-panel p-4 text-sm">
                            <dt className="text-muted">Clock</dt>
                            <dd>
                                {longDate(tonightAt(now + 4 * 3_600_000))}, {clockAt(now)}
                                {p.clock && <span className="text-accent"> (set)</span>}
                            </dd>
                            <dt className="text-muted">Tonight</dt>
                            <dd>{tonightAt(now)}</dd>
                            <dt className="text-muted">Showing</dt>
                            <dd data-view={showing ? showing.view : ""}>
                                {showing
                                    ? `${showing.stale ? "out of date: " : ""}${showing.view}${showing.date ? `, ${showing.date}` : ""}${showing.theme ? ` (${showing.theme === "light" ? "by day" : "at night"})` : ""}`
                                    : "nothing yet"}
                                {flashing && " (refreshing…)"}
                                {settings?.paused && " · paused"}
                            </dd>
                            <dt className="text-muted">Bundle</dt>
                            <dd data-items={bundle?.items.length ?? 0}>
                                {bundle
                                    ? `${bundle.version.slice(0, 8)} · ${bundle.items.length} pictures from ${bundle.tonight}`
                                    : "none"}
                            </dd>
                            <dt className="text-muted">Last sync</dt>
                            <dd>{lastSync ? ago(now - lastSync) : "never"}</dd>
                            <dt className="text-muted">Next check-in</dt>
                            <dd>{nextAt ? `in ${until(nextAt - now)} (${clockAt(nextAt)})` : "—"}</dd>
                            <dt className="text-muted">Queued</dt>
                            <dd data-queued={queue.length}>
                                {queue.length ? queue.map((e) => e.type).join(", ") : "nothing"}
                            </dd>
                        </dl>

                        <div className="flex flex-col gap-2 rounded-xl bg-panel p-4 text-sm">
                            <span className="font-bold">Battery</span>
                            {p.power === "usb" ? (
                                <p className="text-muted">On USB: charging, checking in hourly.</p>
                            ) : (
                                <>
                                    <p>
                                        About <b>{Math.round(daysOnCharge(perDay))} days</b> on a full charge,
                                        with a sync a day, {perDay.refreshes} redraws and {perDay.wakes}{" "}
                                        button wakes.
                                    </p>
                                    <NumberInput
                                        size="xs"
                                        label="Button presses a day"
                                        min={0}
                                        max={50}
                                        value={pressesPerDay}
                                        onChange={(v) => setPressesPerDay(Number(v) || 0)}
                                        className="max-w-48"
                                    />
                                </>
                            )}
                            <p className="text-xs text-subtle">
                                Guesses until the frame is measured: {POWER.capacityMAh} mAh, {POWER.sleepMA}{" "}
                                mA asleep, {POWER.syncMAh} mAh a sync, {POWER.refreshMAh} mAh a refresh. This
                                session: {usage.syncs} syncs, {usage.refreshes} redraws, {usage.wakes}{" "}
                                presses.
                            </p>
                        </div>
                    </div>
                </div>

                <div className="flex flex-col gap-2">
                    <h2 className="text-sm font-bold">Event log</h2>
                    <ol
                        aria-label="Event log"
                        className="flex max-h-80 flex-col overflow-y-auto rounded-xl bg-panel p-2 font-mono text-xs"
                    >
                        {log.length === 0 && <li className="p-2 text-muted">Nothing yet.</li>}
                        {log.map((l, i) => (
                            <li key={log.length - i} className="flex gap-3 px-2 py-1">
                                <span className="shrink-0 text-subtle">
                                    {tonightAt(l.at + 4 * 3_600_000).slice(5)} {clockAt(l.at)}
                                </span>
                                <span>{l.text}</span>
                            </li>
                        ))}
                    </ol>
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
                {hint && <span className="text-xs text-subtle">{hint}</span>}
            </div>
            {children}
        </div>
    );
}

// SetupPage is the page a phone sees on the frame's hotspot (192.168.4.1):
// pick a network, type its password, join.
function SetupPage({
    creds,
    onJoin,
    onClose,
}: {
    creds: Creds;
    onJoin: (ssid: string) => void;
    onClose: () => void;
}) {
    const [picked, setPicked] = useState(creds.networks[0] ?? NEARBY[0]);
    const [other, setOther] = useState("");
    const [password, setPassword] = useState("");
    const ssid = picked === "other" ? other.trim() : picked;
    return (
        <section
            aria-label="Setup page"
            className="flex w-full max-w-sm flex-col gap-3 rounded-2xl border-4 border-[#16181d] bg-raised p-4"
        >
            <p className="flex items-center gap-2 text-xs text-muted">
                <IconWifi size={14} /> Your phone, on {creds.ap.ssid} · 192.168.4.1
            </p>
            <h3 className="text-lg font-bold">Connect the frame to Wi-Fi</h3>
            <Radio.Group value={picked} onChange={setPicked} label="Networks nearby">
                <div className="mt-2 flex flex-col gap-2">
                    {[...new Set([...NEARBY, ...creds.networks])].map((n) => (
                        <Radio key={n} value={n} label={n} />
                    ))}
                    <Radio value="other" label="Another network" />
                </div>
            </Radio.Group>
            {picked === "other" && (
                <TextInput
                    label="Network name"
                    value={other}
                    onChange={(e) => setOther(e.currentTarget.value)}
                />
            )}
            <PasswordInput
                label="Password"
                value={password}
                onChange={(e) => setPassword(e.currentTarget.value)}
            />
            <div className="flex gap-2">
                <Button onClick={() => onJoin(ssid)} disabled={!ssid}>
                    Join
                </Button>
                <Button variant="subtle" onClick={onClose}>
                    Close
                </Button>
            </div>
        </section>
    );
}
