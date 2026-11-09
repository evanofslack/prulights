"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Menu } from "@mantine/core";
import { IconLogout, IconSettings, IconUserPlus, IconUserCircle } from "@tabler/icons-react";
import { useAuth } from "@/lib/auth";
import { InviteModal } from "./InviteModal";

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
                        <>
                            <Menu.Item
                                leftSection={<IconUserPlus size={16} />}
                                onClick={() => setInviting(true)}
                            >
                                Invite someone
                            </Menu.Item>
                            <Menu.Item
                                component={Link}
                                href="/admin"
                                leftSection={<IconSettings size={16} />}
                            >
                                Admin
                            </Menu.Item>
                        </>
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
