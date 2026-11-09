"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, CopyButton, Menu, Modal, Switch } from "@mantine/core";
import { IconLogout, IconUserPlus, IconUserCircle } from "@tabler/icons-react";
import { QRCodeSVG } from "qrcode.react";
import type { InviteLink } from "@/interfaces/api";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";

// Account is the header's corner: "Sign in", or the signed-in person's menu.
export function Account() {
    const { me, signOut } = useAuth();
    const router = useRouter();
    const [inviting, setInviting] = useState(false);

    if (me === undefined) return <span className="w-16" />;
    if (me === null) {
        return (
            <Link href="/login" className="rounded-md px-2.5 py-2 text-base text-muted hover:text-fg sm:px-3">
                Sign in
            </Link>
        );
    }
    return (
        <>
            <Menu position="bottom-end" width={200}>
                <Menu.Target>
                    <button
                        type="button"
                        aria-label="Account"
                        className="flex items-center gap-1.5 rounded-md px-2.5 py-2 text-base text-muted hover:text-fg sm:px-3"
                    >
                        <IconUserCircle size={22} />
                        <span className="hidden max-w-24 truncate sm:inline">{me.name}</span>
                    </button>
                </Menu.Target>
                <Menu.Dropdown>
                    <Menu.Label data-account-name>{me.name}</Menu.Label>
                    {me.is_admin && (
                        <Menu.Item leftSection={<IconUserPlus size={16} />} onClick={() => setInviting(true)}>
                            Invite someone
                        </Menu.Item>
                    )}
                    <Menu.Item
                        leftSection={<IconLogout size={16} />}
                        onClick={async () => {
                            await signOut();
                            router.push("/");
                        }}
                    >
                        Sign out
                    </Menu.Item>
                </Menu.Dropdown>
            </Menu>
            <InviteModal opened={inviting} onClose={() => setInviting(false)} />
        </>
    );
}

// InviteModal makes an invite link, with a QR code to scan from a phone.
function InviteModal({ opened, onClose }: { opened: boolean; onClose: () => void }) {
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
                    body: JSON.stringify({ is_admin: admin }),
                }),
            );
        } catch (e) {
            setError((e as Error).message);
        }
    };
    return (
        <Modal opened={opened} onClose={close} title="Invite someone" centered>
            {link ? (
                <div className="flex flex-col items-center gap-4">
                    <div className="rounded-lg bg-white p-3">
                        <QRCodeSVG value={link.url} size={200} aria-label="Invite QR code" />
                    </div>
                    <p className="text-center text-sm text-muted">
                        Scan it with their phone, or send the link. It works once, for 7 days.
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
                    <Switch
                        label="Make them an admin (they can invite others)"
                        checked={admin}
                        onChange={(e) => setAdmin(e.currentTarget.checked)}
                    />
                    <Button onClick={create}>Make invite link</Button>
                    {error && <p className="text-danger">{error}</p>}
                </div>
            )}
        </Modal>
    );
}
