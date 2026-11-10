import type { Bundle, BundleItem, DevInks, DeviceSettings } from "@/interfaces/api";

// What the frame's firmware does, in the browser, for the emulator: read the
// packed pictures the device api sends, keep the clock, and pick which
// picture to show.

export const WIDTH = 480;
export const HEIGHT = 800;

// Picture is a decoded frame: one ink index per portrait pixel.
export type Picture = Uint8Array;

// decodeFrame reads a .bin.gz: gunzip, then two panel pixels a byte (left
// pixel in the high nibble), turned back from the panel's 800×480 to portrait.
export async function decodeFrame(gz: ArrayBuffer, inks: DevInks): Promise<Picture> {
    const stream = new Blob([gz]).stream().pipeThrough(new DecompressionStream("gzip"));
    const packed = new Uint8Array(await new Response(stream).arrayBuffer());
    const { panel_width: pw, panel_height: ph } = inks;
    if (packed.length !== (pw * ph) / 2) throw new Error(`frame is ${packed.length} bytes`);
    const inkOf = new Uint8Array(16);
    inks.inks.forEach((ink, i) => (inkOf[ink.code] = i));
    const out = new Uint8Array(WIDTH * HEIGHT);
    for (let y = 0; y < HEIGHT; y++) {
        for (let x = 0; x < WIDTH; x++) {
            const nx = inks.rotate_clockwise ? y : pw - 1 - y;
            const ny = inks.rotate_clockwise ? ph - 1 - x : x;
            const i = ny * pw + nx;
            const b = packed[i >> 1];
            out[y * WIDTH + x] = inkOf[i & 1 ? b & 0xf : b >> 4];
        }
    }
    return out;
}

// paint draws a picture onto a 480×800 canvas in a palette's colors.
export function paint(canvas: HTMLCanvasElement, pic: Picture, inks: DevInks, palette: "measured" | "ideal") {
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    const rgb = inks.inks.map((ink) => {
        const hex = palette === "ideal" ? ink.ideal : ink.measured;
        return [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));
    });
    const img = ctx.createImageData(WIDTH, HEIGHT);
    for (let i = 0; i < pic.length; i++) {
        const [r, g, b] = rgb[pic[i]];
        img.data[i * 4] = r;
        img.data[i * 4 + 1] = g;
        img.data[i * 4 + 2] = b;
        img.data[i * 4 + 3] = 255;
    }
    ctx.putImageData(img, 0, 0);
}

const bostonDate = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/New_York",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
});
const bostonClock = new Intl.DateTimeFormat("en-GB", {
    timeZone: "America/New_York",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
});

// tonightAt is the night it is in Boston: the evening's date, rolling over at
// 4am so a 1am glance still shows tonight.
export function tonightAt(ms: number): string {
    return bostonDate.format(new Date(ms - 4 * 3_600_000));
}

// clockAt is the Boston wall clock, "22:05".
export const clockAt = (ms: number) => bostonClock.format(new Date(ms));

// quietAt is whether the frame's quiet hours cover a moment.
export function quietAt(ms: number, s: DeviceSettings): boolean {
    if (!s.quiet_start || !s.quiet_end) return false;
    const now = clockAt(ms);
    return s.quiet_start < s.quiet_end
        ? now >= s.quiet_start && now < s.quiet_end
        : now >= s.quiet_start || now < s.quiet_end;
}

// cycle is the order the button steps through: the owner's views, starting
// from the default.
export function cycle(s: DeviceSettings): string[] {
    return s.views.includes(s.default_view) ? s.views : [s.default_view, ...s.views];
}

// pick is the picture the frame shows: the Wi-Fi screen when it can't get
// online (or is being set up), the welcome screen until claimed, "good night"
// in quiet hours, else the view for tonight (by day or by night, for a frame
// that changes with the daylight), else (the bundle ran out) the stale
// picture.
export function pick(
    bundle: Bundle | undefined,
    claimed: boolean,
    settings: DeviceSettings | undefined,
    view: string,
    ms: number,
    wifi = false,
): BundleItem | undefined {
    if (!bundle) return undefined;
    if (wifi) return bundle.items.find((i) => i.view === "wifi");
    const night = tonightAt(ms);
    if (!claimed || !settings) {
        const setup = bundle.items.filter((i) => i.view === "setup");
        return setup.find((i) => i.date === night) ?? setup[setup.length - 1];
    }
    // a night's view; a frame that changes with the daylight has two, and the
    // one whose window holds the time wins over the one without a window
    const day = (v: string) => {
        const all = bundle.items.filter((i) => !i.stale && i.date === night && i.view === v);
        return (
            all.find((i) => i.from && i.until && ms >= Date.parse(i.from) && ms < Date.parse(i.until)) ??
            all.find((i) => !i.from)
        );
    };
    if (quietAt(ms, settings)) {
        const gn = day("goodnight");
        if (gn) return gn;
    }
    return day(view) ?? bundle.items.find((i) => i.stale);
}

// Battery. Every number here is a guess until the frame is measured: the
// PhotoPainter's cell, the board asleep, one Wi-Fi sync, one full Spectra 6
// refresh (~20 s), and a wake for the button without Wi-Fi.
export const POWER = {
    capacityMAh: 1500,
    sleepMA: 0.1,
    syncMAh: 0.35,
    refreshMAh: 0.2,
    wakeMAh: 0.02,
};

export interface Usage {
    syncs: number;
    refreshes: number;
    wakes: number;
}

// daysOnCharge is how long a full battery lasts at a day's usage.
export function daysOnCharge(perDay: Usage): number {
    const mAh =
        POWER.sleepMA * 24 +
        perDay.syncs * POWER.syncMAh +
        perDay.refreshes * POWER.refreshMAh +
        perDay.wakes * POWER.wakeMAh;
    return POWER.capacityMAh / mAh;
}
