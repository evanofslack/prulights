// Runs the e2e scripts against a server already up, printing each script's
// output as it finishes: node run.mjs [script...]
import { spawn } from "node:child_process";
import { readdirSync } from "node:fs";

const all = readdirSync(".")
    .filter((f) => f.endsWith(".mjs") && !["lib.mjs", "run.mjs"].includes(f))
    .map((f) => f.slice(0, -4));
const named = process.argv.slice(2);
for (const s of named) {
    if (!all.includes(s)) {
        console.log(`no script ${s}.mjs`);
        process.exit(1);
    }
}

function run(name) {
    const start = Date.now();
    return new Promise((resolve) => {
        const child = spawn(process.execPath, [`${name}.mjs`], { stdio: ["ignore", "pipe", "pipe"] });
        let out = "";
        child.stdout.on("data", (d) => (out += d));
        child.stderr.on("data", (d) => (out += d));
        child.on("exit", (code) => {
            console.log(`== ${name} (${Math.round((Date.now() - start) / 1000)}s)\n${out.trimEnd()}\n`);
            resolve({ name, ok: code === 0 });
        });
    });
}

const results = await Promise.all((named.length ? named : all).map(run));
const failed = results.filter((r) => !r.ok).map((r) => r.name);
console.log(`${results.length - failed.length}/${results.length} scripts passed`);
if (failed.length) console.log(`failed: ${failed.join(" ")}`);
process.exit(failed.length ? 1 : 0);
