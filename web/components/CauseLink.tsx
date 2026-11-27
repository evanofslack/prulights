"use client";

import { Anchor } from "@mantine/core";
import { IconExternalLink } from "@tabler/icons-react";

interface CauseLinkProps {
    cause: string;
    url?: string;
    // icon size, to sit with the text around it
    size?: number;
    className?: string;
}

// CauseLink is a night's cause, linked to the group's own site when the
// schedule gives one (it often doesn't). It opens in a new tab.
export function CauseLink({ cause, url, size = 16, className }: CauseLinkProps) {
    if (!url) return <span className={className}>{cause}</span>;
    return (
        <Anchor
            href={url}
            target="_blank"
            rel="noopener"
            c="var(--color-fg)"
            underline="always"
            inherit
            className={className}
        >
            {cause}
            <IconExternalLink size={size} className="ml-1 inline align-baseline text-muted" aria-hidden />
            <span className="sr-only"> (opens their website)</span>
        </Anchor>
    );
}
