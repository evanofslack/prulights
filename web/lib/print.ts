// The web tower is a sharp svg with a printed look: flat colors from the
// frame's ink family, gradients in hard steps like a screen print, a light
// halftone in the big soft areas and paper grain over it. These are the helpers.

type RGB = [number, number, number];

function rgb(hex: string): RGB {
    let h = hex.replace("#", "");
    if (h.length === 3) h = [...h].map((c) => c + c).join("");
    const n = parseInt(h.slice(0, 6), 16);
    return Number.isNaN(n) ? [255, 255, 255] : [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function hex([r, g, b]: RGB): string {
    return `#${[r, g, b]
        .map((v) =>
            Math.round(Math.min(255, Math.max(0, v)))
                .toString(16)
                .padStart(2, "0"),
        )
        .join("")}`;
}

// mix is a and b, t of the way to b.
export function mix(a: string, b: string, t: number): string {
    const x = rgb(a);
    const y = rgb(b);
    return hex([0, 1, 2].map((k) => x[k] + (y[k] - x[k]) * t) as RGB);
}

// mute takes the edge off a lighting color, like ink on paper: a little of its
// own grey mixed in. The api keeps the true colors.
export function mute(c: string, amount = 0.16): string {
    const [r, g, b] = rgb(c);
    const y = 0.3 * r + 0.55 * g + 0.15 * b;
    return hex([r + (y - r) * amount, g + (y - g) * amount, b + (y - b) * amount]);
}

// whiteness is how near a color is to plain white light (0–1): light and
// without much color. White is the most glaring light on a night picture.
export function whiteness(c: string): number {
    const [r, g, b] = rgb(c);
    const hi = Math.max(r, g, b);
    const chroma = (hi - Math.min(r, g, b)) / 255;
    const light = Math.min(1, Math.max(0, (hi / 255 - 0.6) / 0.4));
    return light * Math.min(1, Math.max(0, 1 - chroma / 0.25));
}

// ink is a lighting color as the tower shows it: muted, and white taken down
// toward a soft warm grey so it doesn't glare.
export function ink(c: string): string {
    return mix(mute(c), "#c9c7c0", 0.42 * whiteness(c));
}

// a gradient stop: offset, color, opacity
export type Stop = [number, string, number?];

// banded turns a smooth gradient into n flat bands with hard edges, the first
// band the gradient's start and the last its end.
export function banded(stops: Stop[], n: number): Stop[] {
    const at = (t: number): [string, number] => {
        let i = 0;
        while (i < stops.length - 2 && t > stops[i + 1][0]) i++;
        const [o0, c0, a0 = 1] = stops[i];
        const [o1, c1, a1 = 1] = stops[i + 1] ?? stops[i];
        const f = o1 > o0 ? Math.min(1, Math.max(0, (t - o0) / (o1 - o0))) : 0;
        return [mix(c0, c1, f), a0 + (a1 - a0) * f];
    };
    const out: Stop[] = [];
    for (let k = 0; k < n; k++) {
        const [c, a] = at(n > 1 ? k / (n - 1) : 0);
        out.push([k / n, c, a], [(k + 1) / n, c, a]);
    }
    return out;
}
