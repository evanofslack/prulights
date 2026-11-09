// Passkeys end to end, with Chrome's virtual authenticator: an admin invite
// from the command line, sign up, stay signed in, invite someone from the
// menu, sign out and back in, a re-invite that adds a passkey, and the ways
// an invite can be refused.
import { execFileSync } from "node:child_process";
import { URL, btn, check, finish, launch, open, shot } from "./lib.mjs";

// invite makes an invite with the server's binary and returns its path
const invite = (...args) => {
    const out = execFileSync(process.env.BIN, ["invite", "-no-qr", ...args], { encoding: "utf8" });
    return "/invite?" + out.trim().split("\n").pop().split("?")[1];
};

// person is a browser with its own passkey store (a phone, say)
async function person(browser) {
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
    return p;
}

// signedInAs opens the account menu and reads the name in it (the header
// shows only an icon on phones)
async function signedInAs(p) {
    await p.click("[aria-label=Account]");
    const name = await p.locator("[data-account-name]").textContent();
    await p.keyboard.press("Escape");
    return name;
}

async function signUp(p, path, name) {
    await p.goto(URL + path);
    await p.waitForSelector(btn("Create passkey"));
    if (name) await p.fill("input[autocomplete=given-name]", name);
    await p.click(btn("Create passkey"));
}

const browser = await launch();

// the first admin, from the command line
const evan = await person(browser);
await signUp(evan, invite("-admin"), "Evan");
await evan.waitForURL(`${URL}/`);
await evan.waitForSelector("[aria-label=Account]");
check("admin signs up with a passkey", (await signedInAs(evan)).includes("Evan"));
await evan.reload();
await evan.waitForSelector("[aria-label=Account]");
check("still signed in after a reload", (await signedInAs(evan)).includes("Evan"));

// evan invites mom from the menu
await evan.click("[aria-label=Account]");
await evan.click("text=Invite someone");
await evan.click(btn("Make invite link"));
const link = await evan.locator("[data-invite-url]").textContent();
check("invite has a QR code", await evan.locator("[aria-label='Invite QR code']").isVisible());
await shot(evan, "auth-invite");
const momPath = link.slice(link.indexOf("/invite?"));

const mom = await person(browser);
await signUp(mom, momPath, "  Mom ");
await mom.waitForURL(`${URL}/`);
await mom.waitForSelector("[aria-label=Account]");
check("mom signs up from evan's link", (await signedInAs(mom)).includes("Mom"));
await mom.click("[aria-label=Account]");
check("mom can't invite", (await mom.locator("text=Invite someone").count()) === 0);
await mom.click("text=Sign out");
await mom.waitForSelector("a:text('Sign in')");
check("sign out", true);

await mom.click("a:text('Sign in')");
await mom.click(btn("Sign in with passkey"));
await mom.waitForURL(`${URL}/`);
await mom.waitForSelector("[aria-label=Account]");
check("mom signs back in with her passkey", (await signedInAs(mom)).includes("Mom"));
check(
    "no console errors",
    [...evan.errors, ...mom.errors].length === 0,
    [...evan.errors, ...mom.errors].join("; "),
);

// a used link
const late = await person(browser);
await late.goto(URL + momPath);
await late.waitForSelector("text=This link doesn't work");
check("an invite works once", true);

// mom gets a new phone: a re-invite adds a passkey to her account
const newPhone = await person(browser);
await signUp(newPhone, invite("-for-user", "mom"), "");
await newPhone.waitForURL(`${URL}/`);
await newPhone.waitForSelector("[aria-label=Account]");
check("re-invite signs in as mom", (await signedInAs(newPhone)).includes("Mom"));
await newPhone.click("[aria-label=Account]");
await newPhone.click("text=Sign out");
await newPhone.click("a:text('Sign in')");
await newPhone.click(btn("Sign in with passkey"));
await newPhone.waitForSelector("[aria-label=Account]");
check("the new phone's passkey signs in as mom", (await signedInAs(newPhone)).includes("Mom"));

// names are taken ignoring case
const copycat = await person(browser);
await signUp(copycat, invite(), "MOM");
await copycat.waitForSelector("text=Someone already has that name.");
check("a taken name is refused", true);
await shot(copycat, "auth-taken");

// a device with no passkey here can't sign in
const stranger = await person(browser);
await stranger.goto(`${URL}/login`);
await stranger.click(btn("Sign in with passkey"));
await stranger.waitForSelector("[role=alert]");
check("no passkey, no sign in", (await stranger.locator("[aria-label=Account]").count()) === 0);

await browser.close();
finish();
