"use client";

import { useState } from "react";
import { Button, CopyButton, Modal, Switch } from "@mantine/core";
import { QRCodeSVG } from "qrcode.react";
import type { InviteLink } from "@/interfaces/api";
import { api } from "@/lib/api";

// InviteModal makes an invite link, with a QR code to scan from a phone. With
// forUser it's a re-invite: the link adds a passkey to that person.
export function InviteModal({
    opened,
    onClose,
    forUser,
}: {
    opened: boolean;
    onClose: () => void;
    forUser?: { id: number; name: string };
}) {
    const [admin, setAdmin] = useState(false);
    const [link, setLink] = useState<InviteLink>();
    const [error, setError] = useState<string>();
    const close = () => {
        setLink(undefined);
        setError(undefined);
        setAdmin(false);
        onClose();
    };
    const create = async () => {
        try {
            setLink(
                await api<InviteLink>("/admin/invites", {
                    method: "POST",
                    body: JSON.stringify(forUser ? { for_user_id: forUser.id } : { is_admin: admin }),
                }),
            );
        } catch (e) {
            setError((e as Error).message);
        }
    };
    return (
        <Modal
            opened={opened}
            onClose={close}
            title={forUser ? `New passkey link for ${forUser.name}` : "Invite someone"}
            centered
        >
            {link ? (
                <div className="flex flex-col items-center gap-4">
                    <div className="rounded-lg bg-white p-3">
                        <QRCodeSVG value={link.url} size={200} aria-label="Invite QR code" />
                    </div>
                    <p className="text-center text-sm text-muted">
                        {forUser
                            ? `Open it on ${forUser.name}'s new device to make a passkey there. Their log stays. It works once, for 7 days.`
                            : "Scan it with their phone, or send the link. It works once, for 7 days."}
                    </p>
                    <CopyButton value={link.url}>
                        {({ copied, copy }) => (
                            <Button fullWidth variant="default" onClick={copy}>
                                {copied ? "Copied" : "Copy link"}
                            </Button>
                        )}
                    </CopyButton>
                    <code className="w-full truncate text-xs text-subtle" data-invite-url>
                        {link.url}
                    </code>
                </div>
            ) : (
                <div className="flex flex-col gap-4">
                    {forUser ? (
                        <p className="text-muted">
                            For when {forUser.name} has a new phone or lost their passkey: the link adds a
                            passkey to their account.
                        </p>
                    ) : (
                        <Switch
                            label="Make them an admin (they can invite others)"
                            checked={admin}
                            onChange={(e) => setAdmin(e.currentTarget.checked)}
                        />
                    )}
                    <Button onClick={create}>Make link</Button>
                    {error && <p className="text-danger">{error}</p>}
                </div>
            )}
        </Modal>
    );
}
