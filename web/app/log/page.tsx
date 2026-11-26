"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { Skeleton } from "@mantine/core";
import { Shell } from "@/components/Shell";

// /log moved into the home page; old links land on its "Your nights" section.
export default function Page() {
    const router = useRouter();
    useEffect(() => router.replace("/#nights"), [router]);
    return (
        <Shell>
            <Skeleton height={300} radius="lg" />
        </Shell>
    );
}
