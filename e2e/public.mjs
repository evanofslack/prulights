// The public pages on desktop and phone: tonight, a night, the calendar, the
// display and the api docs. The server holds the 2026 and 31 Nights fixtures
// (just e2e loads them), so December 2026 is known.
import { URL, btn, check, finish, launch, open, shot, sideways, views } from "./lib.mjs";

const browser = await launch();
for (const [kind, view] of Object.entries(views)) {
    const p = await open(browser, view);

    // tonight: the tower, a heading, seven nights coming up
    await p.goto(URL);
    await p.waitForSelector("h1");
    check(
        `${kind}: tonight shows the tower`,
        await p.locator('[role=img][aria-label*="Prudential Tower"]').isVisible(),
    );
    check(`${kind}: tonight names its colors`, (await p.locator("h1").textContent()).length > 2);
    // runs of plain white nights share a row ("Thu–Mon")
    const rows = await p.locator("#week ul li a[href^='/night?d=']").allTextContents();
    check(
        `${kind}: the week ahead, white runs grouped`,
        rows.length >= 1 && rows.length <= 7 && /^Tomorrow/.test(rows[0]),
        rows.join(" | "),
    );
    check(`${kind}: signed out, no nights section`, (await p.locator("#nights").count()) === 0);
    check(`${kind}: footer is one line`, (await p.locator("footer > div").boundingBox()).height < 70);
    // the credit is behind the info icon: a tap (or hover) shows it, esc hides it
    await p.click("[aria-label='About the schedule']");
    const credit = p.locator('a[href*="prudentialcenter.com"]:text("Prudential Center")');
    await credit.waitFor({ timeout: 3000 }).catch(() => {});
    check(`${kind}: credits the source`, await credit.isVisible());
    await p.keyboard.press("Escape");
    check(`${kind}: tonight has no sideways scroll`, !(await sideways(p)));
    await shot(p, `${kind}-tonight`);

    // the calendar is on the same page; months change in place
    await p.goto(`${URL}/?m=2026-12`);
    await p.waitForSelector("#calendar h2:text('December 2026')");
    await p.waitForSelector("#calendar a[href='/night?d=2026-12-31']");
    check(
        `${kind}: december has 31 nights`,
        (await p.locator("#calendar a[href^='/night?d=2026-12-']").count()) === 31,
    );
    await p.click("[aria-label='Next month']");
    await p.waitForSelector("#calendar h2:text('January 2027')");
    check(`${kind}: next month, same page`, new globalThis.URL(p.url()).pathname === "/");
    await shot(p, `${kind}-calendar`);

    // ?m= opens the calendar on a month
    await p.goto(`${URL}/?m=2026-12#calendar`);
    await p.waitForSelector("#calendar h2:text('December 2026')");
    check(`${kind}: ?m= picks the month`, true);

    // a listed night, and its neighbors
    await p.goto(`${URL}/night?d=2026-12-10`);
    await p.waitForSelector("text=Dress for Success");
    check(`${kind}: dec 10 is red`, (await p.locator("h1").textContent()) === "Red");
    await p.click("a[href='/night?d=2026-12-11']");
    await p.waitForURL(/d=2026-12-11/);
    await p.waitForSelector("text=Pawsitively 4Pink");
    check(`${kind}: next night`, true);
    await shot(p, `${kind}-night`);

    // an unlisted night is white
    await p.goto(`${URL}/night?d=2027-08-03`);
    await p.waitForSelector("text=Not announced yet. Usually white.");
    check(`${kind}: unlisted night is white`, (await p.locator("h1").textContent()) === "White");

    // the display, opened from the footer, and out again
    await p.goto(URL);
    await p.waitForSelector("h1");
    await p.click("footer [aria-label='Full screen']");
    await p.waitForURL(/\/display$/);
    await p.waitForSelector("h1");
    check(`${kind}: display shows only tonight`, (await p.locator("main ul").count()) === 0);
    check(`${kind}: display has no header`, (await p.locator("header").count()) === 0);
    await shot(p, `${kind}-display`);
    await p.keyboard.press("Escape");
    await p.waitForURL(`${URL}/`);
    check(`${kind}: esc leaves the display`, true);

    check(`${kind}: no console errors`, p.errors.length === 0, p.errors.join("; "));
    await p.context().close();
}

// the look follows the system until the sun/moon toggle picks one, which sticks
{
    const p = await open(browser, { ...views.phone, colorScheme: "light" });
    const scheme = () => p.evaluate(() => document.documentElement.dataset.mantineColorScheme);
    await p.goto(URL);
    await p.waitForSelector("h1");
    check("follows a light system", (await scheme()) === "light");
    await p.click("[aria-label='Dark mode']");
    await p.reload();
    await p.waitForSelector("h1");
    check("the toggle's choice survives a reload", (await scheme()) === "dark");
    await shot(p, "phone-toggled-dark");
    await p.click("[aria-label='Light mode']");
    await p.goto(`${URL}/display`);
    await p.waitForSelector("main");
    check(
        "the display stays dark on paper",
        (await p.locator("main[data-mantine-color-scheme=dark]").count()) === 1,
    );
    await p.context().close();
}

// the docs are swagger on its own page
const p = await open(browser, views.desktop);
await p.goto(`${URL}/docs`);
await p.waitForSelector(".swagger-ui .opblock", { timeout: 20000 });
check("docs list the endpoints", (await p.locator(".swagger-ui .opblock").count()) >= 8);
check("docs have no site header", (await p.locator("header nav").count()) === 0);
await p.click(".opblock-get .opblock-summary:has-text('/tonight')");
await p.waitForSelector(".opblock.is-open >> text=No parameters"); // resolved, ready to run
await p.click(btn("Execute"));
await p.waitForSelector(".opblock.is-open >> text=Server response");
const result = await p.locator(".opblock.is-open").innerText();
check(
    "docs try it out against this server",
    result.includes(`${URL}/api/v1/tonight`) && result.includes('"lights_on_source"'),
);
await shot(p, "desktop-docs");

await browser.close();
finish();
