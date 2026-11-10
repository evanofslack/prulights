"use client";

import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Alert, Button, Skeleton, TextInput } from "@mantine/core";
import { IconFingerprint } from "@tabler/icons-react";
import type { InviteInfo } from "@/interfaces/api";
import { useApi } from "@/lib/api";
import { passkeyError, useAuth } from "@/lib/auth";
import { Shell } from "@/components/Shell";

function Invite() {
    const token = useSearchParams().get("t") ?? "";
    const { data: info, error: invalid } = useApi<InviteInfo>(token ? `/auth/invites/${token}` : null);
    const { me, register } = useAuth();
    const router = useRouter();
    const [name, setName] = useState("");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();

    if (!token || invalid) {
        return (
            <div className="flex flex-col gap-3">
                <h1 className="text-3xl font-bold">This link doesn&apos;t work</h1>
                <p className="text-lg text-muted">
                    {invalid?.message ?? "It's missing its invite code."} Ask for a new one.
                </p>
            </div>
        );
    }
    if (!info) return <Skeleton height={220} radius="lg" />;

    const reinvite = Boolean(info.for_user);
    const submit = async (e: React.FormEvent) => {
        e.preventDefault();
        setBusy(true);
        setError(undefined);
        try {
            await register({ invite: token }, name);
            router.push("/");
        } catch (err) {
            setError(passkeyError(err));
        } finally {
            setBusy(false);
        }
    };

    return (
        <form onSubmit={submit} className="flex flex-col gap-5">
            <h1 className="text-3xl font-bold">
                {reinvite ? `Welcome back, ${info.for_user}` : "You're invited"}
            </h1>
            <p className="text-lg text-muted">
                {reinvite
                    ? "Make a passkey on this device to sign in again. Everything you've logged stays."
                    : "See what color the Pru is tonight, and keep a log of the nights you see it. Pick a name, then make a passkey: no password, just your face, fingerprint or passcode."}
            </p>
            {me && (
                <Alert variant="light" color="gray">
                    You&apos;re signed in as {me.name} on this device. Continuing signs this device in as{" "}
                    {reinvite ? info.for_user : "the new person"}.
                </Alert>
            )}
            {!reinvite && (
                <TextInput
                    size="lg"
                    label="Your name"
                    description="Everyone invited will see it next to your sightings."
                    value={name}
                    onChange={(e) => setName(e.currentTarget.value)}
                    maxLength={40}
                    required
                    autoComplete="given-name"
                />
            )}
            <Button type="submit" size="lg" leftSection={<IconFingerprint />} loading={busy}>
                Create passkey
            </Button>
            {error && (
                <Alert color="red" variant="light">
                    {error}
                </Alert>
            )}
        </form>
    );
}

export default function InvitePage() {
    return (
        <Shell>
            <div className="mx-auto max-w-md pt-6">
                <Suspense fallback={<Skeleton height={220} radius="lg" />}>
                    <Invite />
                </Suspense>
            </div>
        </Shell>
    );
}
