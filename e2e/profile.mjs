// Your name and ideas: aunt changes her name in /settings (the menu follows),
// can't take a name someone has, sends an idea from the box, and an admin
// reads it in /admin and marks it done.
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

const browser = await launch();
const boss = await person(browser, "Ideas Admin", "-admin");
const aunt = await person(browser, "Aunt Pat");

await aunt.goto(URL + "/settings");
const name = aunt.getByLabel("Username");
await name.waitFor();
check("settings shows your name", (await name.inputValue()) === "Aunt Pat");

await name.fill("Ideas Admin");
await aunt.click(`section:has(#username) ${btn("Save")}`);
await aunt.waitForSelector("text=someone already has that name");
check("a taken name is refused", true);

await name.fill("  Auntie   Pat ");
await aunt.click(`section:has(#username) ${btn("Save")}`);
await aunt.waitForSelector("text=Saved");
check("name is cleaned up", (await name.inputValue()) === "Auntie Pat");
await aunt.click("[aria-label=Account]");
check("the menu follows", (await aunt.locator("[data-account-name]").textContent()) === "Auntie Pat");

await aunt.click("text=Send feedback");
const box = aunt.locator('[role=dialog] [aria-label="Your feedback"]');
await box.waitFor();
check("the menu opens the feedback dialog in place", !aunt.url().includes("#"));
await box.fill("Show the moon behind the tower");
await aunt.waitForTimeout(400); // let the modal finish fading in
await shot(aunt, "profile-idea");
await aunt.click(`[role=dialog] ${btn("Send")}`);
await aunt.waitForSelector("text=Thanks, we got it.");
check("feedback sent", true);
await aunt.click(btn("Close"));

await boss.goto(URL + "/admin");
const ideas = boss.locator("[aria-label=Ideas]");
await ideas.waitFor();
const row = ideas.locator("li", { hasText: "Show the moon behind the tower" });
check("admin reads the idea with her new name", (await row.textContent()).includes("Auntie Pat"));
await row.locator(btn("Done")).click();
await row.locator(btn("Not done")).waitFor();
check("admin marks it done", true);
await shot(boss, "profile-admin-ideas");

await browser.close();
finish();
