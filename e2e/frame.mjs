// The frame, end to end in the emulator's device mode: register, claim it on
// /claim, sync, the button (short cycles views, long logs tonight), settings
// that change the bundle, offline queueing, running past the bundle, the
// admin list, and letting it go. Then a prepared frame for someone new: the
// admin prepares it, the emulator is "flashed" with it, a person with no
// account signs up from its code, and it loses and finds Wi-Fi.
import { execFileSync } from "node:child_process";
import { URL, btn, check, finish, launch, open, shot } from "./lib.mjs";

const invite = (...args) => {
    const out = execFileSync(process.env.BIN, ["invite", "-no-qr", ...args], { encoding: "utf8" });
    return "/invite?" + out.trim().split("\n").pop().split("?")[1];
};

async function passkeyPage(browser, viewport = { width: 1400, height: 1000 }) {
    const p = await open(browser, { viewport, acceptDownloads: true });
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
    return p;
}

async function person(browser, name, ...inviteArgs) {
    const p = await passkeyPage(browser);
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
check("the claimed frame shows tonight", (await fran.getAttribute("[data-items]", "data-items")) === "30");

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
await fran.waitForFunction(() => document.querySelector("[data-items]")?.dataset.items === "9");
check("one view: a week of tonight, the stale picture and the Wi-Fi screen", true);
await showingStarts("tonight ");
await fran.click(btn("Short press"));
await logHas("short press: tonight");
check("the button stays on the only view", (await showing()).startsWith("tonight "));

// offline: presses wait for the next sync
await fran.click("label:has-text('Out of range')");
await fran.waitForURL(/offline=1/);
await fran.click(btn("Double press"));
await logHas("no known network answers");
check("offline keeps events", Number(await fran.getAttribute("[data-queued]", "data-queued")) > 0);
await fran.click("label:has-text('Out of range')");
await fran.waitForURL((u) => !u.search.includes("offline"));
await fran.click(btn("Double press"));
await fran.waitForFunction(() => document.querySelector("[data-queued]")?.dataset.queued === "0");
check("back online they're sent", true);

// a week on with no sync, the frame shows the out-of-date picture
await fran.click(label("Out of range"));
for (let i = 0; i < 7; i++) await fran.click(btn("+1 day"));
await fran.waitForFunction((sel) => document.querySelector(sel)?.dataset.showing?.endsWith(" stale"), frame);
check("past the bundle it shows the stale picture", true);
await fran.click(label("Out of range"));
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

// a frame prepared for someone new
await fran.goto(`${URL}/admin`);
await fran.waitForSelector("text=Prepare a frame");
const [download] = await Promise.all([fran.waitForEvent("download"), fran.click(btn("Prepare a frame"))]);
const file = await download.path();
const prepared = JSON.parse(await (await import("node:fs/promises")).readFile(file, "utf8"));
await fran.waitForSelector(`text=for a new person until`);
check(
    "admin prepares a marked frame",
    prepared.version === 1 && prepared.token && /^prulights-/.test(prepared.ap.ssid),
    JSON.stringify({ ...prepared, token: "…" }),
);
await shot(fran, "frame-prepare");

// the emulator, flashed with it, boots to the welcome screen
const emu = await open(browser, { viewport: { width: 1400, height: 1000 } });
await emu.goto(device);
await emu.waitForSelector(btn("Load a prepared frame"));
await emu.setInputFiles("input[type=file]", file);
await emu.waitForSelector(`[data-claim-code='${prepared.claim_code}']`);
await emu.waitForFunction((sel) => document.querySelector(sel)?.dataset.showing?.startsWith("setup"), frame);
check("a prepared frame boots to its welcome screen", true);

// someone with no account scans it and signs up there and then
const mabel = await passkeyPage(browser, { width: 390, height: 844 });
await mabel.goto(`${URL}/claim?code=${prepared.claim_code}`);
await mabel.waitForSelector("text=Set up your frame");
await shot(mabel, "frame-claim-new");
await mabel.fill("input[autocomplete=given-name]", "Mabel");
await mabel.click(btn("Create passkey"));
await mabel.waitForSelector("text=Welcome, Mabel");
await shot(mabel, "frame-claim-welcome");
const mine = await (await mabel.request.get(`${URL}/api/v1/me/devices`)).json();
check("signing up from the frame's code claims it", mine.length === 1 && mine[0].id === prepared.device_id);
await emu.click(btn("Double press"));
await emu.waitForFunction(
    (sel) => document.querySelector(sel)?.dataset.showing?.startsWith("tonight "),
    frame,
);
check("the frame then shows tonight", true);

// no known network: the Wi-Fi screen, then its setup page gets it back online
await emu.click(label("Out of range"));
await emu.click(btn("Double press"));
await emu.click(btn("Double press"));
await emu.waitForFunction((sel) => document.querySelector(sel)?.dataset.showing?.startsWith("wifi"), frame);
check("with no Wi-Fi it shows the Wi-Fi screen", true);
await emu.waitForSelector("[aria-label='Setup page']");
await shot(emu, "frame-wifi");
await emu.click("[aria-label='Setup page'] label:has-text('Cape House')");
await emu.click(btn("Join"));
await emu.waitForFunction(
    (sel) => document.querySelector(sel)?.dataset.showing?.startsWith("tonight "),
    frame,
);
check(
    "joining a network on the setup page brings it back",
    (await emu.textContent("body")).includes("Cape House"),
);

// a member signed in on a frame that isn't marked just claims it
await emu.click(btn("Forget this frame"));
await emu.click(btn("Register a frame"));
const code3 = await (await emu.waitForSelector("[data-claim-code]")).getAttribute("data-claim-code");
const anon = await open(browser, { viewport: { width: 390, height: 844 } });
await anon.goto(`${URL}/claim?code=${code3}`);
await anon.waitForSelector("text=needs an invite");
check("signed out, an unmarked frame asks for an invite", true);
await mabel.goto(`${URL}/claim?code=${code3}`);
await mabel.click(btn("Connect this frame"));
await mabel.waitForSelector("text=Connected");
check("a member claims an unmarked frame", true);

for (const [who, p] of [
    ["fran", fran],
    ["emu", emu],
    ["mabel", mabel],
    ["anon", anon],
])
    check(`no console errors (${who})`, p.errors.length === 0, p.errors.join("; "));
await browser.close();
finish();

function label(text) {
    return `label:has-text('${text}')`;
}
