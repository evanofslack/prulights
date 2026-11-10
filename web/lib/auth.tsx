"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import {
    startAuthentication,
    startRegistration,
    type PublicKeyCredentialCreationOptionsJSON,
    type PublicKeyCredentialRequestOptionsJSON,
} from "@simplewebauthn/browser";
import type { Ceremony, MeResponse } from "@/interfaces/api";
import { api, ApiError } from "./api";

// Who's signed in, and the passkey flows. The session is an HttpOnly cookie;
// the page only ever learns the user from GET /me.

interface Auth {
    // undefined while loading, null when signed out
    me: MeResponse | null | undefined;
    signIn: () => Promise<MeResponse>;
    // register makes a passkey for someone new (or a re-invite), from an
    // invite link or from the code of a frame marked for a new person.
    register: (from: { invite: string } | { claim_code: string }, name: string) => Promise<MeResponse>;
    signOut: () => Promise<void>;
}

const AuthContext = createContext<Auth | undefined>(undefined);

const post = <T,>(path: string, body?: unknown) =>
    api<T>(path, { method: "POST", body: body === undefined ? undefined : JSON.stringify(body) });

export function AuthProvider({ children }: { children: React.ReactNode }) {
    const [me, setMe] = useState<MeResponse | null>();

    useEffect(() => {
        api<MeResponse | undefined>("/me")
            .then((u) => setMe(u ?? null))
            .catch(() => setMe(null));
    }, []);

    const signIn = useCallback(async () => {
        const begin = await post<Ceremony>("/auth/login/begin");
        const response = await startAuthentication({
            optionsJSON: begin.options as PublicKeyCredentialRequestOptionsJSON,
        });
        const user = await post<MeResponse>(`/auth/login/finish?ceremony=${begin.ceremony}`, response);
        setMe(user);
        return user;
    }, []);

    const register = useCallback(async (from: { invite: string } | { claim_code: string }, name: string) => {
        const begin = await post<Ceremony>("/auth/register/begin", { ...from, name });
        const response = await startRegistration({
            optionsJSON: begin.options as PublicKeyCredentialCreationOptionsJSON,
        });
        const user = await post<MeResponse>(`/auth/register/finish?ceremony=${begin.ceremony}`, response);
        setMe(user);
        return user;
    }, []);

    const signOut = useCallback(async () => {
        await post<void>("/auth/logout");
        setMe(null);
    }, []);

    return <AuthContext.Provider value={{ me, signIn, register, signOut }}>{children}</AuthContext.Provider>;
}

export function useAuth(): Auth {
    const auth = useContext(AuthContext);
    if (!auth) throw new Error("useAuth outside AuthProvider");
    return auth;
}

// passkeyError turns a failed passkey flow into words for a person.
export function passkeyError(e: unknown): string {
    if (e instanceof ApiError) return e.message.charAt(0).toUpperCase() + e.message.slice(1) + ".";
    if (e instanceof Error) {
        if (e.name === "NotAllowedError" || e.name === "AbortError")
            return "The passkey prompt was closed. Try again?";
        if (e.name === "InvalidStateError")
            return "This device already has a passkey for this account. Sign in instead.";
        return e.message;
    }
    return "Something went wrong.";
}
