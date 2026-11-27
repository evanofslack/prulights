"use client";

import { useState } from "react";
import { Alert, Button, Modal, Textarea } from "@mantine/core";
import { IconCheck, IconSend } from "@tabler/icons-react";
import type { FeedbackRequest } from "@/interfaces/api";
import { api, json } from "@/lib/api";

// FeedbackModal is the account menu's "Send feedback": anything you'd like the
// site to do, or something that bugs you. The admins read it in /admin.
export function FeedbackModal({ opened, onClose }: { opened: boolean; onClose: () => void }) {
    const [text, setText] = useState("");
    const [sending, setSending] = useState(false);
    const [sent, setSent] = useState(false);
    const [error, setError] = useState("");
    const close = () => {
        setText("");
        setSent(false);
        setError("");
        onClose();
    };
    const send = async (e: React.FormEvent) => {
        e.preventDefault();
        setSending(true);
        setError("");
        try {
            await api("/me/feedback", { method: "POST", body: json<FeedbackRequest>({ text }) });
            setText("");
            setSent(true);
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setSending(false);
        }
    };
    return (
        <Modal opened={opened} onClose={close} title="Send feedback" centered>
            {sent ? (
                <div className="flex flex-col gap-4">
                    <p className="flex items-center gap-2 text-lg">
                        <IconCheck size={20} /> Thanks, we got it.
                    </p>
                    <Button size="lg" variant="default" onClick={close}>
                        Close
                    </Button>
                </div>
            ) : (
                <form onSubmit={send} className="flex flex-col gap-3">
                    <Textarea
                        size="lg"
                        aria-label="Your feedback"
                        placeholder="I wish it would…"
                        autosize
                        minRows={3}
                        maxLength={2000}
                        data-autofocus
                        value={text}
                        onChange={(e) => setText(e.currentTarget.value)}
                    />
                    <Button
                        type="submit"
                        size="lg"
                        loading={sending}
                        disabled={text.trim() === ""}
                        leftSection={<IconSend size={20} />}
                    >
                        Send
                    </Button>
                    {error && (
                        <Alert color="red" variant="light">
                            {error}
                        </Alert>
                    )}
                </form>
            )}
        </Modal>
    );
}
