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
        await p.locator('svg[role=img][aria-label*="Prudential Tower"]').isVisible(),
    );
    check(`${kind}: tonight names its colors`, (await p.locator("h1").textContent()).length > 2);
    check(
        `${kind}: seven nights coming up`,
        (await p.locator("main ul li a[href^='/night?d=']").count()) === 7,
    );
    check(
        `${kind}: credits the source`,
        await p.locator('footer a[href*="prudentialcenter.com"]').isVisible(),
    );
    check(`${kind}: tonight has no sideways scroll`, !(await sideways(p)));
    await shot(p, `${kind}-tonight`);

    // a listed night, and its neighbors
    await p.goto(`${URL}/night?d=2026-12-10`);
    await p.waitForSelector("text=Dress for Success");
    check(`${kind}: dec 10 is red`, (await p.locator("h1").textContent()) === "Red");
    check(`${kind}: dec 10 lights on at 5pm`, await p.locator("text=Lights on around 5:00pm").isVisible());
    await p.click("a[href='/night?d=2026-12-11']");
    await p.waitForURL(/d=2026-12-11/);
    await p.waitForSelector("text=Pawsitively 4Pink");
    check(`${kind}: next night`, true);
    await shot(p, `${kind}-night`);

    // an unlisted night is white
    await p.goto(`${URL}/night?d=2027-08-03`);
    await p.waitForSelector("text=shines white");
    check(`${kind}: unlisted night is white`, (await p.locator("h1").textContent()) === "White");

    // the calendar
    await p.goto(`${URL}/calendar?m=2026-12`);
    await p.waitForSelector("h1:text('December 2026')");
    await p.waitForSelector("main a[href='/night?d=2026-12-31']");
    check(
        `${kind}: december has 31 nights`,
        (await p.locator("main a[href^='/night?d=2026-12-']").count()) === 31,
    );
    check(`${kind}: calendar has no sideways scroll`, !(await sideways(p)));
    await p.click("[aria-label='Next month']");
    await p.waitForSelector("h1:text('January 2027')");
    check(`${kind}: next month`, true);
    await shot(p, `${kind}-calendar`);

    // the display, opened from tonight, and out again
    await p.goto(URL);
    await p.click(btn("Open as display"));
    await p.waitForURL(/\/display$/);
    await p.waitForSelector("h1");
    check(`${kind}: display shows seven nights`, (await p.locator("main ul li").count()) === 7);
    check(`${kind}: display has no header`, (await p.locator("header").count()) === 0);
    await shot(p, `${kind}-display`);
    await p.keyboard.press("Escape");
    await p.waitForURL(`${URL}/`);
    check(`${kind}: esc leaves the display`, true);

    check(`${kind}: no console errors`, p.errors.length === 0, p.errors.join("; "));
    await p.context().close();
}

// the docs are swagger on its own page
const p = await open(browser, views.desktop);
await p.goto(`${URL}/docs`);
await p.waitForSelector(".swagger-ui .opblock", { timeout: 20000 });
check("docs list the endpoints", (await p.locator(".swagger-ui .opblock").count()) >= 8);
check("docs have no site header", (await p.locator("header nav a:text('Calendar')").count()) === 0);
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
