"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Alert, Button } from "@mantine/core";
import { IconFingerprint } from "@tabler/icons-react";
import { passkeyError, useAuth } from "@/lib/auth";
import { Shell } from "@/components/Shell";

// nextPath is where to go after signing in: ?next= when it's a path on this
// site, else tonight.
function nextPath(): string {
    const next = new URLSearchParams(window.location.search).get("next") ?? "";
    return next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/\\") ? next : "/";
}

export default function LoginPage() {
    const { me, signIn } = useAuth();
    const router = useRouter();
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();

    const go = async () => {
        setBusy(true);
        setError(undefined);
        try {
            await signIn();
            router.push(nextPath());
        } catch (e) {
            setError(passkeyError(e));
        } finally {
            setBusy(false);
        }
    };

    return (
        <Shell>
            <div className="mx-auto flex max-w-md flex-col gap-5 pt-6">
                <h1 className="text-3xl font-bold">Sign in</h1>
                {me ? (
                    <p className="text-lg text-muted">You&apos;re signed in as {me.name}.</p>
                ) : (
                    <>
                        <p className="text-lg text-muted">
                            Use the passkey you made when you were invited: your phone or computer will ask
                            for your face, fingerprint or passcode.
                        </p>
                        <Button size="lg" leftSection={<IconFingerprint />} loading={busy} onClick={go}>
                            Sign in with passkey
                        </Button>
                        {error && (
                            <Alert color="red" variant="light">
                                {error}
                            </Alert>
                        )}
                        <p className="text-muted">
                            New here? prulights is invite only: ask for a link. Lost your passkey? Ask for a
                            new link; it keeps everything you&apos;ve logged.
                        </p>
                    </>
                )}
            </div>
        </Shell>
    );
}
