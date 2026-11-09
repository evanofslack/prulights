// The frame, end to end in the emulator's device mode: register, claim it on
// /claim, sync, the button (short cycles views, long logs tonight), settings
// that change the bundle, offline queueing, running past the bundle, the
// admin list, and letting it go.
import { execFileSync } from "node:child_process";
import { URL, btn, check, finish, launch, open, shot } from "./lib.mjs";

const invite = (...args) => {
    const out = execFileSync(process.env.BIN, ["invite", "-no-qr", ...args], { encoding: "utf8" });
    return "/invite?" + out.trim().split("\n").pop().split("?")[1];
};

async function person(browser, name, ...inviteArgs) {
    const p = await open(browser, { viewport: { width: 1400, height: 1000 } });
    const cdp = await p.context().newCDPSession(p);
    await cdp.send("WebAuthn.enable");
    await cdp.send("WebAuthn.addVirtualAuthenticator", {
        options: {
            protocol: "ctap2",
            transport: "internal",
            hasResidentKey: true,
            hasUserVerification: true,
            isUserVerified: true,
            automaticPresenceSimulation: true,
        },
    });
    await p.goto(URL + invite(...inviteArgs));
    await p.fill("input[autocomplete=given-name]", name);
    await p.click(btn("Create passkey"));
    await p.waitForSelector("[aria-label=Account]");
    return p;
}

const browser = await launch();
const fran = await person(browser, "Fran", "-admin");
const frame = "canvas[aria-label='The frame']";
const showing = () => fran.getAttribute(frame, "data-showing");
const showingStarts = (prefix) =>
    fran.waitForFunction(
        ([sel, prefix]) => document.querySelector(sel)?.dataset.showing?.startsWith(prefix),
        [frame, prefix],
    );
const logHas = (text) =>
    fran.waitForFunction(
        (text) => document.querySelector("[aria-label='Event log']")?.textContent.includes(text),
        text,
    );
const device = `${URL}/emulator?mode=device`;

// a new frame shows its setup screen
await fran.goto(device);
await fran.click(btn("Register a frame"));
const code = await (await fran.waitForSelector("[data-claim-code]")).getAttribute("data-claim-code");
await showingStarts("setup");
check("a new frame shows its setup screen", /^[A-Z0-9]{3}-[A-Z0-9]{3}$/.test(code), code);
const painted = await fran.evaluate((sel) => {
    const c = document.querySelector(sel);
    const d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data;
    const seen = new Set();
    for (let i = 0; i < d.length; i += 4 * 97) seen.add(`${d[i]},${d[i + 1]},${d[i + 2]}`);
    return seen.size;
}, frame);
check("the decoded picture is drawn", painted >= 2, `${painted} colors`);

// claim it the way the QR code would
await fran.goto(`${URL}/claim?code=${code}`);
await fran.waitForSelector("input[value='" + code + "']");
await fran.getByLabel("Call it").fill("Kitchen");
await fran.click(btn("Connect this frame"));
await fran.waitForSelector("text=Connected");
await shot(fran, "frame-claimed");
check("claiming says it's connected", true);

// back on the frame: it syncs at boot and shows tonight
await fran.goto(device);
await fran.waitForSelector("[data-claimed]:text('Kitchen')");
await showingStarts("tonight ");
check("the claimed frame shows tonight", (await fran.getAttribute("[data-items]", "data-items")) === "29");

await fran.click(btn("Short press"));
await showingStarts("week ");
check("a short press shows the week", true);

await fran.click(btn("Long press"));
await logHas("long press: seen");
await fran.waitForFunction(() => document.querySelector("[data-queued]")?.dataset.queued === "0");
const stats = await (await fran.request.get(`${URL}/api/v1/me/stats`)).json();
check("a long press logs tonight", stats.seen === 1, JSON.stringify(stats));
await showingStarts("tonight ");
check("after syncing it shows the default view", true);
await shot(fran, "frame-emulator");

// settings: only tonight, so the bundle shrinks and the button stays put
await fran.goto(`${URL}/settings`);
await fran.waitForSelector("form[aria-label='Frame Kitchen']");
for (const label of ["The week ahead", "The month", "Your log"]) await fran.getByLabel(label).uncheck();
await fran.click(btn("Save"));
await fran.waitForSelector("[role=status]:has-text('Saved')");
check("settings save", true);
await fran.setViewportSize({ width: 390, height: 844 });
await shot(fran, "frame-settings");
await fran.setViewportSize({ width: 1400, height: 1000 });

await fran.goto(device);
await fran.waitForFunction(() => document.querySelector("[data-items]")?.dataset.items === "8");
check("one view: a week of tonight plus the stale picture", true);
await showingStarts("tonight ");
await fran.click(btn("Short press"));
await logHas("short press: tonight");
check("the button stays on the only view", (await showing()).startsWith("tonight "));

// offline: presses wait for the next sync
await fran.click("label:has-text('Offline')");
await fran.waitForURL(/offline=1/);
await fran.click(btn("Double press"));
await logHas("offline, keeping");
check("offline keeps events", Number(await fran.getAttribute("[data-queued]", "data-queued")) > 0);
await fran.click("label:has-text('Offline')");
await fran.waitForURL((u) => !u.search.includes("offline"));
await fran.click(btn("Double press"));
await fran.waitForFunction(() => document.querySelector("[data-queued]")?.dataset.queued === "0");
check("back online they're sent", true);

// a week on with no sync, the frame shows the out-of-date picture
await fran.click(label("Offline"));
for (let i = 0; i < 7; i++) await fran.click(btn("+1 day"));
await fran.waitForFunction((sel) => document.querySelector(sel)?.dataset.showing?.endsWith(" stale"), frame);
check("past the bundle it shows the stale picture", true);
await fran.click(label("Offline"));
await fran.click(btn("Now"));

// the admin page lists it
await fran.goto(`${URL}/admin`);
await fran.waitForSelector("[aria-label=Frames] >> text=Kitchen");
check("admin lists the frame", (await fran.locator("[aria-label=Frames] >> text=Fran").count()) === 1);

// letting go sends it back to setup with a new code
await fran.goto(`${URL}/settings`);
await fran.click(btn("Disconnect this frame"));
await fran.click(btn("Disconnect"));
await fran.waitForSelector("text=No frame yet");
await fran.goto(device);
await showingStarts("setup");
const code2 = await fran.getAttribute("[data-claim-code]", "data-claim-code");
check("disconnected, it's back to setup with a new code", code2 && code2 !== code, code2);

check("no console errors", fran.errors.length === 0, fran.errors.join("; "));
await browser.close();
finish();

function label(text) {
    return `label:has-text('${text}')`;
}
