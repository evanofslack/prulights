// Practice Prudle (/prudle), signed out, on a phone: the home card says what
// the game is (a Play now! sticker) and leads here, under Your nights when
// signed in; the page opens on a
// game. Easy start deals one-color nights first; their colors win, Next night
// deals another, Skip moves on without showing colors, six wrong guesses lose
// and show the answer, Random deals any night. No dealt night is a December
// 2026 cause. The December server has no practice card.
import { URL, btn, check, finish, launch, open, shot, sideways, views } from "./lib.mjs";

const DECEMBER = process.env.PRUDLE_URL || "http://localhost:8097";
const browser = await launch();
const p = await open(browser, views.phone);
const get = async (path) => (await p.request.get(`${URL}/api/v1${path}`)).json();

const tonight = (await get("/tonight")).date;
const palette = (await get("/palette")).colors;
const label = (k) => palette.find((c) => c.key === k).label;
const december = (await get("/lightings?from=2026-12-01&to=2026-12-31&fill=false")).lightings.map((l) =>
    l.cause.toLowerCase(),
);
const board = "p:has-text('Practice ·')";

// the night on the board: "Practice · March 14, 2026" → its lighting
async function dealt() {
    const head = await p.locator(board).first().textContent();
    const date = new Date(`${head.split("·")[1].trim()} 12:00 UTC`).toISOString().slice(0, 10);
    return get(`/lightings/${date}`);
}
async function next(button) {
    const before = await p.locator(board).first().textContent();
    await p.click(btn(button));
    await p.waitForFunction((t) => {
        const head = [...document.querySelectorAll("p")].find((e) => e.textContent.startsWith("Practice ·"));
        return head && head.textContent !== t;
    }, before);
    return dealt();
}

// home: Prudle, what it is, Play now!
await p.goto(URL);
await p.waitForSelector("#week");
const card = p.locator("a[href='/prudle']");
check("home has the Prudle card outside December", await card.isVisible());
check("the card says what the game is", (await card.textContent()).includes("Guess the tower"));
check("the card says Play now!", await card.locator("text=Play now!").isVisible());
await shot(p, "practice-card");

// straight to a game, Easy start: one color first
await card.click();
await p.waitForSelector(board);
check("practice opens on a game", await p.locator(btn("Skip")).isVisible());
check("practice has no sideways scroll", !(await sideways(p)));
const first = await dealt();
check("a past night, not in December", first.date < tonight && first.date.slice(5, 7) !== "12", first.date);
check("easy start begins with one color", first.colors.length === 1, first.colors.join("+"));
await shot(p, "practice-board");
for (const k of first.colors) await p.click(`button[aria-label="${label(k)}"]`);
await p.click(btn("Guess 1 color"));
await p.waitForSelector("text=Got it in 1 guess");
check("the right colors win", true);
await shot(p, "practice-won");

// skip: another night, no colors shown
const causes = [first.cause];
const skipped = await next("Next night");
causes.push(skipped.cause);
check("next night is a different night", skipped.date !== first.date, `${first.date} ${skipped.date}`);
const second = await next("Skip");
causes.push(second.cause);
check("skip moves on", second.date !== skipped.date, `${skipped.date} ${second.date}`);
check(
    "skip shows no colors",
    (await p.locator("p:text-is('The colors')").count()) === 0 && (await p.locator(btn("Skip")).isVisible()),
);

// six wrong guesses lose and show the answer
const wrong = palette.map((c) => c.key).filter((k) => !second.colors.includes(k));
for (let i = 0; i < 6; i++) {
    await p.click(`button[aria-label="${label(wrong[i])}"]`);
    await p.click(btn("Guess 1 color"));
    await p.waitForSelector(`text=${i + 1}/6 guesses`, { timeout: 3000 }).catch(() => {});
}
await p.waitForSelector("text=Out of guesses");
const shown = await p.locator("ul li").allTextContents();
check("losing shows the colors", second.colors.every((k) => shown.some((t) => t.includes(label(k)))), shown.join("|"));
await shot(p, "practice-lost");

// random: any night, none of them a cause December will show
await p.click("label:has-text('Random')");
await p.waitForSelector(board);
for (let i = 0; i < 4; i++) causes.push((await next("Skip")).cause);
check(
    "no dealt night is a December cause",
    !causes.some((c) => december.includes(c.toLowerCase())),
    causes.join(" | "),
);
await p.reload();
await p.waitForSelector(board);
check("the mode is kept", await p.locator("input[value=random]").isChecked());
check("no console errors", p.errors.length === 0, p.errors.join(" | "));

// signed in: the card comes right after Your nights
const me = await open(browser, views.phone);
const cdp = await me.context().newCDPSession(me);
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
await me.goto(`${URL}/join?code=back%20bay%20lights`);
await me.waitForSelector("input[data-signup-code]");
await me.fill("input[autocomplete=given-name]", "Practicer");
await me.click(btn("Create passkey"));
await me.waitForSelector("[aria-label=Account]");
await me.goto(URL);
await me.waitForSelector("#nights");
await me.waitForSelector("a[href='/prudle']");
check(
    "signed in, the card follows Your nights",
    await me.evaluate(() => document.querySelector("#nights")?.nextElementSibling?.matches("a[href='/prudle']")),
);
await shot(me, "practice-card-signed-in");

// December has the real game: no practice card
const dec = await open(browser, views.phone);
await dec.goto(DECEMBER);
await dec.waitForSelector("#week");
check("no practice card in December", (await dec.locator("a[href='/prudle']").count()) === 0);

await browser.close();
finish();
