// The web tower is drawn the way the e-ink frame draws it (backend/render/ink.go):
// every dot is one of the panel's six inks, and a color in between is two inks
// mixed by the same 8×8 ordered dither. The dots are finer than the frame's.

type RGB = [number, number, number];

// INKS are the panel's inks as the web shows them: softer than the ideal
// colors, brighter than a real panel measures (render.Ideal, render.Measured).
// Order: black, white, yellow, red, blue, green.
export const INKS: RGB[] = [
    [18, 19, 24],
    [236, 234, 226],
    [227, 206, 52],
    [184, 52, 34],
    [34, 84, 172],
    [44, 128, 78],
];
export const INK_RED = INKS[3];

// BAYER8 is the frame's ordered dither matrix.
const BAYER8 = [
    [0, 32, 8, 40, 2, 34, 10, 42],
    [48, 16, 56, 24, 50, 18, 58, 26],
    [12, 44, 4, 36, 14, 46, 6, 38],
    [60, 28, 52, 20, 62, 30, 54, 22],
    [3, 35, 11, 43, 1, 33, 9, 41],
    [51, 19, 59, 27, 49, 17, 57, 25],
    [15, 47, 7, 39, 13, 45, 5, 37],
    [63, 31, 55, 23, 61, 29, 53, 21],
];

// dither mixes happen in linear light, as fine dots do to the eye
const linear = (v: number) => {
    v /= 255;
    return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
};
const LIN = INKS.map((c) => c.map(linear) as RGB);
const PAIRS: [number, number][] = [];
for (let a = 0; a < INKS.length; a++) for (let b = a + 1; b < INKS.length; b++) PAIRS.push([a, b]);
// chroma is an ink's OKLab a and b: its hue and colorfulness, not its lightness
const CHROMA = LIN.map(([r, g, b]) => {
    const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
    const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
    const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
    return [
        1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
        0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
    ];
});
// how much mixing two inks of different hues costs (after Yliluoma): blue and
// yellow can average to grey, but it reads as noise; black and white don't pay
const PAIR_PENALTY = 0.3;
// how much each channel counts when picking the closest mix
const WEIGHT: RGB = [0.3, 0.55, 0.15];

// recipe is a color's best two-ink mix, packed: ink a, ink b, and how much
// of b (0–255), like the frame's InkRecipe.
function recipe(r: number, g: number, b: number): number {
    const c = [linear(r), linear(g), linear(b)];
    let best = Infinity;
    let out = 0;
    for (const [i, j] of PAIRS) {
        const A = LIN[i];
        const B = LIN[j];
        let dot = 0;
        let len = 0;
        for (let k = 0; k < 3; k++) {
            const d = B[k] - A[k];
            dot += (c[k] - A[k]) * d * WEIGHT[k];
            len += d * d * WEIGHT[k];
        }
        const m = Math.min(1, Math.max(0, dot / len));
        let err = 0;
        for (let k = 0; k < 3; k++) {
            // the error is judged on a perceptual-ish scale, so darks count
            const e = Math.sqrt(A[k] + m * (B[k] - A[k])) - Math.sqrt(c[k]);
            err += e * e * WEIGHT[k];
        }
        const far = (CHROMA[i][0] - CHROMA[j][0]) ** 2 + (CHROMA[i][1] - CHROMA[j][1]) ** 2;
        err += PAIR_PENALTY * far * m * (1 - m);
        if (err < best) {
            best = err;
            out = i | (j << 4) | (Math.round(m * 255) << 8);
        }
    }
    return out;
}

// recipes are cached by color at 6 bits a channel
const CACHE = new Int32Array(1 << 18).fill(-1);

// inkify turns a picture into inks in place.
export function inkify(img: ImageData): void {
    const d = img.data;
    const w = img.width;
    for (let i = 0, p = 0; i < d.length; i += 4, p++) {
        const key = ((d[i] >> 2) << 12) | ((d[i + 1] >> 2) << 6) | (d[i + 2] >> 2);
        let r = CACHE[key];
        if (r < 0) r = CACHE[key] = recipe(d[i] | 3, d[i + 1] | 3, d[i + 2] | 3);
        const x = p % w;
        const y = (p / w) | 0;
        const ink = (BAYER8[y & 7][x & 7] + 0.5) / 64 < (r >> 8) / 255 ? (r >> 4) & 15 : r & 15;
        const c = INKS[ink];
        d[i] = c[0];
        d[i + 1] = c[1];
        d[i + 2] = c[2];
        d[i + 3] = 255;
    }
}
