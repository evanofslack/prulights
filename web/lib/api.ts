"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// The api lives on the same origin: the go server serves both, and in dev next
// passes /api through to it (next.config.ts).

export class ApiError extends Error {
    constructor(
        public status: number,
        message: string,
    ) {
        super(message);
    }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await fetch(`/api/v1${path}`, {
        ...init,
        headers: {
            Accept: "application/json",
            ...(init?.body ? { "Content-Type": "application/json" } : {}),
            ...init?.headers,
        },
        credentials: "same-origin",
    });
    if (!res.ok) {
        let msg = res.statusText;
        try {
            msg = (await res.json()).error ?? msg;
        } catch {}
        throw new ApiError(res.status, msg);
    }
    if (res.status === 204) return undefined as T;
    return res.json();
}

export interface Loaded<T> {
    data: T | undefined;
    error: Error | undefined;
    loading: boolean;
    reload: () => void;
}

// useApi loads a path, keeping the last good data while it reloads. A null path
// waits. refreshMs reloads on an interval (and when the tab comes back).
export function useApi<T>(path: string | null, refreshMs?: number): Loaded<T> {
    const [data, setData] = useState<T>();
    const [error, setError] = useState<Error>();
    const [loading, setLoading] = useState(path !== null);
    const [tick, setTick] = useState(0);
    const latest = useRef(path);
    latest.current = path;

    const reload = useCallback(() => setTick((t) => t + 1), []);

    useEffect(() => {
        if (path === null) return;
        let cancelled = false;
        setLoading(true);
        api<T>(path)
            .then((d) => {
                if (!cancelled && latest.current === path) {
                    setData(d);
                    setError(undefined);
                }
            })
            .catch((e: Error) => !cancelled && setError(e))
            .finally(() => !cancelled && setLoading(false));
        return () => {
            cancelled = true;
        };
    }, [path, tick]);

    useEffect(() => {
        if (!refreshMs) return;
        const id = setInterval(reload, refreshMs);
        const onVisible = () => document.visibilityState === "visible" && reload();
        document.addEventListener("visibilitychange", onVisible);
        return () => {
            clearInterval(id);
            document.removeEventListener("visibilitychange", onVisible);
        };
    }, [refreshMs, reload]);

    return { data, error, loading, reload };
}
