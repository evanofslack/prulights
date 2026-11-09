// The frame emulator in manual mode: views, nights, colors and conditions all
// change the preview, and live in the url. just e2e runs the server with DEV=1.
import { URL, btn, check, finish, launch, open, shot, sideways, views } from "./lib.mjs";

const browser = await launch();
const p = await open(browser, views.desktop);

// a frame finished loading at this src
const frames = () =>
    p.locator("figure img").evaluateAll((imgs) =>
        imgs.map((i) => ({
            src: i.getAttribute("src"),
            w: i.naturalWidth,
            h: i.naturalHeight,
            done: i.complete,
        })),
    );
const settle = async () => {
    await p.waitForFunction(() =>
        [...document.querySelectorAll("figure img")].every((i) => i.complete && i.naturalWidth),
    );
    return frames();
};

await p.goto(URL);
await p.click("header a:text('Emulator')");
await p.waitForURL(/\/emulator$/);
let f = await settle();
check("the header links to the emulator in dev", true);
check("a 480×800 frame", f.length === 1 && f[0].w === 480 && f[0].h === 800, JSON.stringify(f));
check("defaults to tonight", f[0].src.startsWith("/dev/render/tonight?"));

await p.click(btn("month"));
await p.fill("input[type=date]", "2026-12-10");
await p.waitForURL(/view=month.*date=2026-12-10/);
f = await settle();
check("view and night go to the render", f[0].src.includes("/month?date=2026-12-10"), f[0].src);

await p.click("h1"); // out of the date field, where arrows edit the date
await p.keyboard.press("ArrowRight");
await p.waitForURL(/date=2026-12-11/);
await p.keyboard.press("2");
await p.waitForURL(/view=week/);
check("keys step the night and pick the view", true);

await p.click("label:has-text('Low battery')");
await p.click("label:has-text('Sample sightings and stats')");
await p.click("label:has-text('Both')");
await p.waitForURL(/colors=both/);
await p.waitForSelector("figure:nth-of-type(2) img");
f = await settle();
check(
    "both palettes side by side",
    f.length === 2 && f[1].src.includes("palette=ideal"),
    JSON.stringify(f.map((x) => x.src)),
);
check(
    "conditions go to the render",
    f.every((x) => x.src.includes("low_battery=1") && x.src.includes("sample=1")),
);
await shot(p, "emulator");

// the url holds it all
await p.reload();
f = await settle();
check("settings survive a reload", f.length === 2 && f[0].src.includes("/week?date=2026-12-11"));

const phone = await open(browser, views.phone);
await phone.goto(`${URL}/emulator?view=log&sample=1`);
await phone.waitForSelector("figure img");
check("phone: no sideways scroll at true size", !(await sideways(phone)));

check("no console errors", p.errors.length === 0, p.errors.join("; "));
await browser.close();
finish();
