"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Alert, Button, Skeleton, TextInput } from "@mantine/core";
import { IconCircleCheck, IconPlugConnected } from "@tabler/icons-react";
import type { DeviceInfo } from "@/interfaces/api";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Shell } from "@/components/Shell";

// /claim?code= is where a frame's setup QR code lands: connect the frame to
// your account. Signed out, it sends you to sign in and back.
function Claim() {
    const { me } = useAuth();
    const params = useSearchParams();
    const [code, setCode] = useState("");
    const [name, setName] = useState("Frame");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();
    const [done, setDone] = useState<DeviceInfo>();

    useEffect(() => setCode(params.get("code") ?? ""), [params]);

    if (me === undefined) return <Skeleton height={240} radius="lg" />;
    if (me === null) {
        const back = `/claim${code ? `?code=${encodeURIComponent(code)}` : ""}`;
        return (
            <>
                <h1 className="text-3xl font-bold">Connect a frame</h1>
                <p className="text-lg text-muted">Sign in first, then the frame joins your account.</p>
                <Button component={Link} href={`/login?next=${encodeURIComponent(back)}`} size="lg">
                    Sign in
                </Button>
            </>
        );
    }
    if (done) {
        return (
            <>
                <h1 className="flex items-center gap-2 text-3xl font-bold">
                    <IconCircleCheck size={32} className="text-accent" /> Connected
                </h1>
                <p className="text-lg">
                    <b>{done.name}</b> is yours. It shows tonight&apos;s colors the next time it checks in,
                    within a couple of minutes. To update it now, press its button twice.
                </p>
                <Button component={Link} href="/settings" size="lg" variant="default">
                    Frame settings
                </Button>
            </>
        );
    }

    const claim = async (e: React.FormEvent) => {
        e.preventDefault();
        setBusy(true);
        setError(undefined);
        try {
            setDone(
                await api<DeviceInfo>("/devices/claim", {
                    method: "POST",
                    body: JSON.stringify({ code, name }),
                }),
            );
        } catch (err) {
            setError((err as Error).message);
        } finally {
            setBusy(false);
        }
    };
    return (
        <form onSubmit={claim} className="flex flex-col gap-5">
            <h1 className="text-3xl font-bold">Connect a frame</h1>
            <p className="text-lg text-muted">The code is on the frame&apos;s screen, under the QR code.</p>
            <TextInput
                size="lg"
                label="Code"
                placeholder="ABC-123"
                value={code}
                onChange={(e) => setCode(e.currentTarget.value)}
                autoCapitalize="characters"
                autoComplete="off"
                required
            />
            <TextInput
                size="lg"
                label="Call it"
                description="So you can tell your frames apart."
                value={name}
                onChange={(e) => setName(e.currentTarget.value)}
                maxLength={40}
            />
            <Button
                type="submit"
                size="lg"
                leftSection={<IconPlugConnected />}
                loading={busy}
                disabled={!code.trim()}
            >
                Connect this frame
            </Button>
            {error && (
                <Alert color="red" variant="light">
                    {error}
                </Alert>
            )}
        </form>
    );
}

export default function ClaimPage() {
    return (
        <Shell>
            <div className="mx-auto flex max-w-md flex-col gap-5 pt-6">
                <Suspense fallback={<Skeleton height={240} radius="lg" />}>
                    <Claim />
                </Suspense>
            </div>
        </Shell>
    );
}
