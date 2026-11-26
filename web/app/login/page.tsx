"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Alert, Button } from "@mantine/core";
import { IconFingerprint } from "@tabler/icons-react";
import type { SignupInfo } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { nextPath, passkeyError, useAuth } from "@/lib/auth";
import { Shell } from "@/components/Shell";

export default function LoginPage() {
    const { me, signIn } = useAuth();
    const { data: signup } = useApi<SignupInfo>("/auth/signup");
    const router = useRouter();
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();
    // ?next= rides along to /join
    const [search, setSearch] = useState("");
    useEffect(() => setSearch(window.location.search), []);

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
                        <p className="text-lg text-muted">Use your face, fingerprint or passcode.</p>
                        <Button size="lg" leftSection={<IconFingerprint />} loading={busy} onClick={go}>
                            Sign in with passkey
                        </Button>
                        {error && (
                            <Alert color="red" variant="light">
                                {error}
                            </Alert>
                        )}
                        {signup?.open ? (
                            <Button component={Link} href={`/join${search}`} size="lg" variant="default">
                                New here? Join with a code
                            </Button>
                        ) : (
                            <p className="text-muted">Invite only. New here? Ask for a link.</p>
                        )}
                    </>
                )}
            </div>
        </Shell>
    );
}
