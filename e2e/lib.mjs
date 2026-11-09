// Shared helpers for the e2e scripts.
import { chromium } from "playwright-core";

export const URL = process.env.URL || "http://localhost:8099";

// launch uses CHROMIUM_PATH if set, else playwright's own chromium
export function launch() {
    const exe = process.env.CHROMIUM_PATH;
    return chromium.launch(exe ? { executablePath: exe } : {});
}

export const views = {
    desktop: { viewport: { width: 1280, height: 800 } },
    phone: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 },
};

let passed = 0;
let failed = 0;

export function check(name, ok, detail = "") {
    if (ok) passed++;
    else failed++;
    console.log(`${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`);
}

// finish prints the tally and exits nonzero on any failure
export function finish() {
    console.log(`\n${passed}/${passed + failed} passed`);
    process.exit(failed ? 1 : 0);
}

// shot saves a screenshot when SHOTS names a directory
export async function shot(page, name) {
    if (process.env.SHOTS) await page.screenshot({ path: `${process.env.SHOTS}/${name}.png`, fullPage: true });
}

// open makes a page in a new browser context, collecting console errors.
export async function open(browser, context = {}) {
    const page = await (await browser.newContext({ colorScheme: "dark", reducedMotion: "reduce", ...context })).newPage();
    page.errors = [];
    page.on("pageerror", (e) => page.errors.push(e.message));
    page.on("console", (m) => m.type() === "error" && page.errors.push(m.text()));
    return page;
}

// sideways is true when the page scrolls horizontally
export const sideways = (page) => page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
