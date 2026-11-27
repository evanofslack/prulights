"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import {
    startAuthentication,
    startRegistration,
    type PublicKeyCredentialCreationOptionsJSON,
    type PublicKeyCredentialRequestOptionsJSON,
} from "@simplewebauthn/browser";
import type { Ceremony, MeResponse, RegisterBegin, RenameRequest } from "@/interfaces/api";
import { api, ApiError } from "./api";

// Who's signed in, and the passkey flows. The session is an HttpOnly cookie;
// the page only ever learns the user from GET /me.

interface Auth {
    // undefined while loading, null when signed out
    me: MeResponse | null | undefined;
    signIn: () => Promise<MeResponse>;
    // register makes a passkey for someone new (or a re-invite), from an
    // invite link or the family code.
    register: (from: RegisterFrom, name: string) => Promise<MeResponse>;
    signOut: () => Promise<void>;
    // rename changes the name everyone sees.
    rename: (name: string) => Promise<MeResponse>;
}

type RegisterFrom = { invite: string } | { code: string };

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

    const register = useCallback(async (from: RegisterFrom, name: string) => {
        const begin = await post<Ceremony>("/auth/register/begin", { ...from, name } satisfies RegisterBegin);
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

    const rename = useCallback(async (name: string) => {
        const user = await api<MeResponse>("/me/name", {
            method: "PUT",
            body: JSON.stringify({ name } satisfies RenameRequest),
        });
        setMe(user);
        return user;
    }, []);

    return (
        <AuthContext.Provider value={{ me, signIn, register, signOut, rename }}>
            {children}
        </AuthContext.Provider>
    );
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

// nextPath is where to go after signing in or up: ?next= when it's a path on
// this site, else tonight.
export function nextPath(): string {
    const next = new URLSearchParams(window.location.search).get("next") ?? "";
    return next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/\\") ? next : "/";
}
