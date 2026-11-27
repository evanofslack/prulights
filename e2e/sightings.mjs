// Sightings end to end: mom marks tonight seen with stars and a note, it shows
// in her log and to evan, the public sees only totals, the window closes after
// a week, evan sees hers in lately, and evan corrects a night from the admin page.
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

// act selects one of the panel's take-back buttons: an icon (and, on a phone, a
// word), named by its aria-label
const act = (word) => `button[aria-label="${word}"]`;

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
check("the note box is there before any stars", await mom.locator('[aria-label="Your note"]').isVisible());
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

// logging a night never moves its title: the panel only grows downward
// (its place on the page, not on screen: clicks scroll the phone)
const h1y = () => mom.evaluate(() => document.querySelector("h1").getBoundingClientRect().top + scrollY);
for (const [label, size] of [
    ["desktop", { width: 1280, height: 800 }],
    ["phone", { width: 390, height: 844 }],
]) {
    await mom.setViewportSize(size);
    await mom.goto(`${URL}/night?d=${addDays(tonight, -1)}`);
    await mom.waitForSelector(btn("I saw it"));
    const before = await h1y();
    await shot(mom, `sightings-button-${label}`);
    await mom.click(btn("I saw it"));
    await mom.waitForSelector("text=You saw it");
    const seen = await h1y();
    await rate(mom, 3);
    await mom.waitForSelector(act("Clear stars"));
    const rated = await h1y();
    await shot(mom, `sightings-rated-${label}`);
    await mom.click(act("Clear stars"));
    await mom.waitForSelector(act("Clear stars"), { state: "detached" });
    const cleared = await h1y();
    check(
        `${label}: the title stays put while logging`,
        [seen, rated, cleared].every((y) => Math.abs(y - before) < 1),
        `${before} ${seen} ${rated} ${cleared}`,
    );
    await mom.click(act("Undo"));
    await mom.waitForSelector(btn("I saw it"));
}

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

// her nights, at the bottom of the home page
await mom.goto(`${URL}/#nights`);
await mom.waitForSelector("#nights >> text=Glowing over the Charles");
check("her nights list both", (await mom.locator("#nights ul li").count()) === 2);
check(
    "her nights count them",
    (await mom.locator("#nights dt:text('Nights seen') + dd").textContent()) === "2",
);
await shot(mom, "sightings-log");

// the calendar marks her nights; the one with a note opens on it
const noted = `#calendar a[href="/night?d=${tonight}#note"]`;
await mom.waitForSelector(`${noted} [data-mark=note]`);
check("the calendar marks tonight's note", true);
const weekAgo = addDays(tonight, -7);
if (weekAgo.slice(0, 7) === tonight.slice(0, 7)) {
    check(
        "the calendar dots a night she saw",
        (await mom.locator(`#calendar a[href="/night?d=${weekAgo}"] [data-mark=seen]`).count()) === 1,
    );
}
check(
    "unlogged nights have no mark",
    (await mom.locator(`#calendar a[href="/night?d=${addDays(tonight, -1)}"] [data-mark]`).count()) === 0,
);
await shot(mom, "calendar-marks");
await mom.click(noted);
await mom.waitForURL(`**/night?d=${tonight}#note`);
await mom.waitForSelector('[aria-label="Your note"]');
await mom.waitForTimeout(300);
check(
    "it opens with her note in view",
    await mom.evaluate(() => {
        const r = document.querySelector('[aria-label="Your note"]').getBoundingClientRect();
        return r.top >= 0 && r.bottom <= innerHeight;
    }),
);

// evan sees it on tonight
await evan.goto(URL);
await evan.waitForSelector("[aria-label='Everyone else'] >> text=Nana");
check(
    "evan sees mom's sighting and note",
    await evan.locator("[aria-label='Everyone else'] >> text=Glowing over the Charles").isVisible(),
);

// evan says tonight's colors were wrong: mom sees him in his own row, the
// colors as swatches, and he can change or take it back
await evan.click(act("Colors wrong?"));
await evan.click('[role=dialog] button[aria-label="Red"]');
await evan.click(btn("Send"));
await evan.waitForSelector(act("Take back"));
check("evan's report shows his change and take back", (await evan.locator(act("Change")).count()) === 1);
await mom.goto(URL);
await mom.waitForSelector(`[aria-label='Everyone else'] li:has-text("Ada") [role=img][aria-label="Red"]`);
check("mom sees evan's report as a swatch in his row", true);
await shot(mom, "sightings-report");
await evan.click(act("Take back"));
await evan.waitForSelector(act("Colors wrong?"));
check("a report can be taken back", true);

// evan sees her in lately, as a badge; mom doesn't see herself there
await evan.waitForSelector("#lately >> text=Nana saw it");
check("lately shows mom's night to evan", true);
await shot(evan, "lately");
await mom.goto(URL);
await mom.waitForSelector("#nights");
check("lately leaves out her own nights", (await mom.locator("#lately >> text=Nana").count()) === 0);

// the public sees a count, no names
const stranger = await open(browser, { viewport: { width: 390, height: 844 } });
await stranger.goto(URL);
await stranger.waitForSelector("text=Seen by 1");
check("signed out: a count and no names", (await stranger.locator("text=Nana").count()) === 0);
check("signed out: no I saw it button", (await stranger.locator(btn("I saw it")).count()) === 0);
check("signed out: no nights section", (await stranger.locator("#nights").count()) === 0);

// mom changes her mind about the old night
await mom.goto(`${URL}/night?d=${addDays(tonight, -7)}`);
await mom.click(act("Undo"));
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
await evan.click('label:has-text("Colors")');
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
