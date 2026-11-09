// Sightings end to end: mom marks tonight seen with stars and a note, it shows
// in her log and to evan, the public sees only totals, the window closes after
// a week, and evan corrects a night from the admin page.
import { execFileSync } from "node:child_process";
import { URL, btn, check, finish, launch, open, shot } from "./lib.mjs";

const invite = (...args) => {
    const out = execFileSync(process.env.BIN, ["invite", "-no-qr", ...args], { encoding: "utf8" });
    return "/invite?" + out.trim().split("\n").pop().split("?")[1];
};

async function person(browser, name, ...inviteArgs) {
    const p = await open(browser, { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
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

// rate clicks the nth star of the rating control
async function rate(p, n) {
    const input = p.locator(`[aria-label="Your rating"] input[value="${n}"]`);
    await p.click(`label[for="${await input.getAttribute("id")}"]`);
}

const addDays = (d, n) => {
    const t = new Date(`${d}T12:00:00Z`);
    t.setUTCDate(t.getUTCDate() + n);
    return t.toISOString().slice(0, 10);
};

const browser = await launch();
const evan = await person(browser, "Ada", "-admin");
const mom = await person(browser, "Nana");
const tonight = (await (await mom.request.get(`${URL}/api/v1/meta`)).json()).tonight;

// mom saw it tonight: 4 stars and a note
await mom.goto(URL);
await mom.click(btn("I saw it"));
await mom.waitForSelector("text=You saw it");
await rate(mom, 4);
await mom.waitForFunction(
    () => document.querySelector('[aria-label="Your rating"] input[value="4"]')?.checked,
);
await mom.fill('[aria-label="Your note"]', "Glowing over the Charles");
await mom.click(btn("Save note"));
await mom.waitForSelector(btn("Save note"), { state: "detached" });
check("mom marks tonight seen, rated and noted", true);
await shot(mom, "sightings-tonight");

await mom.reload();
await mom.waitForSelector("text=You saw it");
check(
    "it's still there after a reload",
    (await mom.inputValue('[aria-label="Your note"]')) === "Glowing over the Charles" &&
        (await mom.locator('[aria-label="Your rating"] input[value="4"]').isChecked()),
);

// last week is still open; eight nights ago isn't
await mom.goto(`${URL}/night?d=${addDays(tonight, -7)}`);
await mom.click(btn("I saw it"));
await mom.waitForSelector("text=You saw it");
check("a week ago can still be logged", true);
await mom.goto(`${URL}/night?d=${addDays(tonight, -8)}`);
await mom.waitForSelector("text=Nobody logged this night.");
check("eight nights ago can't", (await mom.locator(btn("I saw it")).count()) === 0);
await mom.goto(`${URL}/night?d=${addDays(tonight, 1)}`);
await mom.waitForSelector("h1");
await mom.waitForTimeout(300);
check("tomorrow can't", (await mom.locator(btn("I saw it")).count()) === 0);

// her log
await mom.click("header a:text('Log')");
await mom.waitForSelector("text=Glowing over the Charles");
check("the log lists both nights", (await mom.locator("main ul li").count()) === 2);
check("the log counts them", (await mom.locator("dt:text('Nights seen') + dd").textContent()) === "2");
await shot(mom, "sightings-log");

// evan sees it on tonight
await evan.goto(URL);
await evan.waitForSelector("[aria-label='Also seen by'] >> text=Nana");
check("evan sees mom's sighting and note", await evan.locator("text=Glowing over the Charles").isVisible());

// the public sees a count, no names
const stranger = await open(browser, { viewport: { width: 390, height: 844 } });
await stranger.goto(URL);
await stranger.waitForSelector("text=Seen by 1");
check("signed out: a count and no names", (await stranger.locator("text=Nana").count()) === 0);
check("signed out: no I saw it button", (await stranger.locator(btn("I saw it")).count()) === 0);

// mom changes her mind about the old night
await mom.goto(`${URL}/night?d=${addDays(tonight, -7)}`);
await mom.click("text=I didn't see it");
await mom.waitForSelector(btn("I saw it"));
check("a sighting can be taken back", true);

// evan corrects a night from the admin page
const fix = addDays(tonight, 3);
await evan.setViewportSize({ width: 1280, height: 900 });
await evan.click("[aria-label=Account]");
await evan.click("text=Admin");
await evan.waitForSelector("h2:text('People')");
await evan.waitForSelector("td:text('Nana')");
check("admin lists mom", true);
await evan.fill("input[type=date]", fix);
await evan.click("label:has-text(\"Colors\")");
await evan.keyboard.type("Purple");
await evan.click("[role=option]:has-text('Purple')");
await evan.keyboard.press("Escape");
await evan.fill('input[placeholder="Who or what it\'s for"]', "Testing a correction");
await evan.click(btn("Save correction"));
await evan.waitForSelector("[aria-label=Corrections] >> text=Testing a correction");
await evan.goto(`${URL}/night?d=${fix}`);
await evan.waitForSelector("text=Testing a correction");
check("a correction changes the night", (await evan.locator("h1").textContent()) === "Purple");
await evan.goto(`${URL}/admin`);
await evan.click(btn("Check now"));
await evan.waitForSelector("text=the scraper isn't running here");
check("check now says when there's no scraper", true);
await shot(evan, "sightings-admin");

check(
    "no console errors",
    [...mom.errors, ...stranger.errors].length === 0,
    [...mom.errors, ...stranger.errors].join("; "),
);
await browser.close();
finish();
