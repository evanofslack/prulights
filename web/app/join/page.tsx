"use client";

import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Alert, Button, Skeleton, TextInput } from "@mantine/core";
import { IconFingerprint } from "@tabler/icons-react";
import type { SignupInfo } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { nextPath, passkeyError, useAuth } from "@/lib/auth";
import { Shell } from "@/components/Shell";

// Join signs someone up with the family code (SIGNUP_CODE): a name, then a
// passkey. ?code= fills the code in, so a link or QR is all it takes; ?next=
// is where to go after, as on /login.
function Join() {
    const params = useSearchParams();
    const { data: info } = useApi<SignupInfo>("/auth/signup");
    const { me, register } = useAuth();
    const router = useRouter();
    const [code, setCode] = useState(params.get("code") ?? "");
    const [name, setName] = useState("");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();

    if (!info) return <Skeleton height={300} radius="lg" />;
    if (!info.open) {
        return (
            <div className="flex flex-col gap-3">
                <h1 className="text-3xl font-bold">Invite only</h1>
                <p className="text-lg text-muted">Ask for an invite link to sign up.</p>
            </div>
        );
    }

    const submit = async (e: React.FormEvent) => {
        e.preventDefault();
        setBusy(true);
        setError(undefined);
        try {
            await register({ code }, name);
            router.push(nextPath());
        } catch (err) {
            setError(passkeyError(err));
        } finally {
            setBusy(false);
        }
    };

    return (
        <form onSubmit={submit} className="flex flex-col gap-5">
            <h1 className="text-3xl font-bold">Join</h1>
            <p className="text-lg text-muted">
                Pick a name, then sign in with your face, fingerprint or passcode.
            </p>
            {me && (
                <Alert variant="light" color="gray">
                    You&apos;re signed in as {me.name} on this device. Continuing signs this device in as the
                    new person.
                </Alert>
            )}
            <TextInput
                size="lg"
                label="Code"
                value={code}
                onChange={(e) => setCode(e.currentTarget.value)}
                required
                autoCapitalize="none"
                autoComplete="off"
                spellCheck={false}
                data-signup-code
            />
            <TextInput
                size="lg"
                label="Your name"
                description="Shown next to your sightings."
                value={name}
                onChange={(e) => setName(e.currentTarget.value)}
                maxLength={40}
                required
                autoComplete="given-name"
            />
            <Button type="submit" size="lg" leftSection={<IconFingerprint />} loading={busy}>
                Create passkey
            </Button>
            {error && (
                <Alert color="red" variant="light">
                    {error}
                </Alert>
            )}
            <p className="text-muted">
                Already joined?{" "}
                <Link
                    href={`/login${params.get("next") ? `?next=${encodeURIComponent(params.get("next")!)}` : ""}`}
                    className="text-accent underline"
                >
                    Sign in
                </Link>
            </p>
        </form>
    );
}

export default function JoinPage() {
    return (
        <Shell>
            <div className="mx-auto max-w-md pt-6">
                <Suspense fallback={<Skeleton height={300} radius="lg" />}>
                    <Join />
                </Suspense>
            </div>
        </Shell>
    );
}
