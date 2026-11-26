"use client";

import { Suspense, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Skeleton } from "@mantine/core";
import { Shell } from "@/components/Shell";

// /calendar moved into the home page; old links land on its calendar section.
function Redirect() {
    const router = useRouter();
    const m = useSearchParams().get("m");
    useEffect(() => {
        router.replace(`/${m ? `?m=${encodeURIComponent(m)}` : ""}#calendar`);
    }, [router, m]);
    return <Skeleton height={480} radius="lg" />;
}

export default function Page() {
    return (
        <Shell>
            <Suspense fallback={<Skeleton height={480} radius="lg" />}>
                <Redirect />
            </Suspense>
        </Shell>
    );
}
