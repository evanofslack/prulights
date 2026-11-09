"use client";

import dynamic from "next/dynamic";
import { Shell } from "@/components/Shell";

// Scalar renders the api reference from the spec the go server serves. It only
// runs in the browser, so it's loaded on the client.
const Reference = dynamic(() => import("@/components/ApiReference"), {
    ssr: false,
    loading: () => <p className="text-muted">Loading the API reference…</p>,
});

export default function DocsPage() {
    return (
        <Shell wide>
            <div className="flex flex-col gap-4">
                <div className="flex flex-col gap-2">
                    <h1 className="text-3xl font-bold">API</h1>
                    <p className="max-w-2xl text-lg text-muted">
                        The Pru's lighting schedule, free to use with no key. Start with{" "}
                        <code className="rounded bg-raised px-1.5 py-0.5">GET /api/v1/tonight</code>, or
                        subscribe to{" "}
                        <a className="underline" href="/api/v1/calendar.ics">
                            the calendar feed
                        </a>
                        . The spec is at{" "}
                        <a className="underline" href="/api/v1/openapi.json">
                            openapi.json
                        </a>{" "}
                        and{" "}
                        <a className="underline" href="/api/v1/openapi.yaml">
                            openapi.yaml
                        </a>
                        .
                    </p>
                </div>
                <div className="overflow-hidden rounded-xl bg-white">
                    <Reference />
                </div>
            </div>
        </Shell>
    );
}
