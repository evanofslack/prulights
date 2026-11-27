// Prudle end to end, on its own server whose clock says Dec 10, 2026 (Red,
// for Dress for Success): signed out you see the colors and an invite to play;
// signed in tonight is a game and December's colors are hidden; red and pink
// doesn't win, red alone does and shows them; someone else peeks and sees the
// first player's marks, never colors; turning Prudle off shows the rest of
// December.
import { btn, check, finish, launch, open, shot } from "./lib.mjs";

const URL = process.env.PRUDLE_URL || "http://localhost:8097";
const night = (d) => `a[href^="/night?d=${d}"]`;

async function person(browser, name) {
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
    if (!name) return p;
    await p.goto(`${URL}/join?code=back%20bay%20lights`);
    await p.waitForSelector("input[data-signup-code]");
    await p.fill("input[autocomplete=given-name]", name);
    await p.click(btn("Create passkey"));
    await p.waitForSelector("[aria-label=Account]");
    return p;
}

const browser = await launch();

// signed out: tonight's colors, and an invite to play
const guest = await person(browser);
await guest.goto(URL);
await guest.waitForSelector("h1");
check("signed out sees tonight's colors", (await guest.locator("h1").first().textContent()).includes("Red"));
check("signed out is asked to play", await guest.locator("text=Sign in to play").isVisible());

// Wren plays: the game stands in for tonight
const wren = await person(browser, "Wren");
await wren.goto(URL);
await wren.waitForSelector("[aria-label=\"Tonight's Prudle\"]");
check("tonight is a game, its cause the clue", (await wren.locator("h1").first().textContent()).includes("Dress for Success"));
const fresh = await (await wren.request.get(`${URL}/api/v1/me/prudle/2026-12-10`)).json();
check("the server keeps the answer", fresh.answer === undefined && fresh.can_play, JSON.stringify(fresh));
await wren.waitForSelector(`${night("2026-12-11")} [data-mark=hidden]`);
check("later nights are hidden on the calendar", true);
check("no colors named in the week", !(await wren.locator("#week").textContent()).includes("Red"));
await shot(wren, "prudle-board");

// red and pink holds the answer but isn't it: no win, and red stays in
await wren.click('button[aria-label="Red"]');
await wren.click('button[aria-label="Pink"]');
await wren.click(btn("Guess 2 colors"));
await wren.waitForSelector('[aria-live=polite] >> text=Close');
check("a guess with an extra color doesn't win", await wren.locator("text=Fewer than 2 colors tonight.").isVisible());
check("pink is close to red", await wren.locator('button[aria-label="Pink: Close"]').isVisible());
check("a guess is a try", await wren.locator("text=1/6").isVisible());
await wren.click(btn("Guess 1 color"));
await wren.waitForSelector("text=Prudle: got it in 2 guesses");
check("solving shows tonight's colors", (await wren.locator("h1").first().textContent()).includes("Red"));
check("tonight shows on the calendar", (await wren.locator(`${night("2026-12-10")} [data-mark=hidden]`).count()) === 0);
check("tomorrow stays hidden", (await wren.locator(`${night("2026-12-11")} [data-mark=hidden]`).count()) === 1);
await shot(wren, "prudle-won");

// Opal peeks, then sees how Wren did, without colors
const opal = await person(browser, "Opal");
await opal.goto(URL);
await opal.waitForSelector("[aria-label=\"Tonight's Prudle\"]");
const locked = await (await opal.request.get(`${URL}/api/v1/prudle/2026-12-10`)).json();
check("the family board waits until you've played", locked.locked && locked.players.length === 0);
await opal.click("text=Just show me");
await opal.click(btn("Show me"));
await opal.waitForSelector("text=Prudle: you peeked");
check("peeking shows the colors", (await opal.locator("h1").first().textContent()).includes("Red"));
await opal.waitForSelector('[aria-label="Tonight, everyone"]');
const family = await opal.locator("#prudle").textContent();
check("the family board shows Wren", family.includes("Wren") && family.includes("got it"), family);
check("the family board names no colors", !/Red|Pink/.test(family), family);
await shot(opal, "prudle-family");

// Opal turns Prudle off: the rest of December shows
await opal.goto(`${URL}/settings`);
await opal.click(`label:has-text("Play Prudle")`);
await opal.waitForFunction(() => !document.querySelector("input[type=checkbox]")?.checked);
await opal.goto(`${URL}/?m=2026-12`);
await opal.waitForSelector(night("2026-12-25"));
await opal.waitForFunction(() => document.querySelectorAll("[data-mark=hidden]").length === 0);
check("off shows every color", true);

check("no console errors", ![guest, wren, opal].some((p) => p.errors.length), [guest, wren, opal].flatMap((p) => p.errors).join("; "));
await browser.close();
finish();
