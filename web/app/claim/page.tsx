"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Alert, Button, Skeleton, TextInput } from "@mantine/core";
import { IconCircleCheck, IconPlugConnected } from "@tabler/icons-react";
import type { ClaimStatus, DeviceInfo, SignupInfo } from "@/interfaces/api";
import { api, useApi } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Shell } from "@/components/Shell";

// /claim?code= is where a frame's QR code lands. Signed in, it connects the
// frame to you. Signed out, it asks you to sign in (or join) and come back.

const clean = (code: string) => code.toUpperCase().replace(/[^A-Z0-9]/g, "");

function Claim() {
    const { me } = useAuth();
    const params = useSearchParams();
    const [code, setCode] = useState("");
    const [frameName, setFrameName] = useState("Frame");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();
    const [claimed, setClaimed] = useState<DeviceInfo>();

    useEffect(() => setCode(params.get("code") ?? ""), [params]);
    const complete = clean(code).length === 6;
    const { data: status } = useApi<ClaimStatus>(complete ? `/devices/claim/${clean(code)}` : null);
    const { data: signup } = useApi<SignupInfo>(me === null ? "/auth/signup" : null);

    if (me === undefined) return <Skeleton height={240} radius="lg" />;

    if (claimed) {
        return (
            <>
                <h1 className="flex items-center gap-2 text-3xl font-bold">
                    <IconCircleCheck size={32} className="text-accent" /> Connected
                </h1>
                <p className="text-lg">
                    <b>{claimed.name}</b> is yours. It shows tonight&apos;s colors the next time it checks in,
                    within a couple of minutes. To update it now, press its button twice.
                </p>
                <Button component={Link} href="/settings" size="lg" variant="default">
                    Frame settings
                </Button>
            </>
        );
    }

    const codeField = (
        <TextInput
            size="lg"
            label="Code"
            description="It's on the frame's screen, under the QR code."
            placeholder="ABC-123"
            value={code}
            onChange={(e) => setCode(e.currentTarget.value)}
            autoCapitalize="characters"
            autoComplete="off"
            required
            error={complete && status && !status.waiting ? "No frame is waiting for that code." : undefined}
        />
    );
    const fail = (
        <>
            {error && (
                <Alert color="red" variant="light">
                    {error}
                </Alert>
            )}
        </>
    );

    // signed in: connect it to you
    if (me) {
        const claim = async (e: React.FormEvent) => {
            e.preventDefault();
            setBusy(true);
            setError(undefined);
            try {
                setClaimed(
                    await api<DeviceInfo>("/devices/claim", {
                        method: "POST",
                        body: JSON.stringify({ code, name: frameName }),
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
                {codeField}
                <TextInput
                    size="lg"
                    label="Call it"
                    description="So you can tell your frames apart."
                    value={frameName}
                    onChange={(e) => setFrameName(e.currentTarget.value)}
                    maxLength={40}
                />
                <Button
                    type="submit"
                    size="lg"
                    leftSection={<IconPlugConnected />}
                    loading={busy}
                    disabled={!complete || status?.waiting === false}
                >
                    Connect this frame
                </Button>
                {fail}
            </form>
        );
    }

    // signed out: sign in (or join) and come back here
    const back = encodeURIComponent(`/claim${complete ? `?code=${clean(code)}` : ""}`);
    return (
        <div className="flex flex-col gap-5">
            <h1 className="text-3xl font-bold">Connect a frame</h1>
            {!complete || !status ? (
                codeField
            ) : status.waiting ? (
                <p className="text-lg text-muted">
                    Sign in to connect this frame to your account.{" "}
                    {signup?.open
                        ? "New here? Join with the family code first."
                        : "New here? Ask the person who gave it to you for an invite."}
                </p>
            ) : (
                codeField
            )}
            <Button component={Link} href={`/login?next=${back}`} size="lg">
                Sign in
            </Button>
            {signup?.open && (
                <Button component={Link} href={`/join?next=${back}`} size="lg" variant="default">
                    Join with the family code
                </Button>
            )}
        </div>
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
