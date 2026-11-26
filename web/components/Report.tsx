"use client";

import { useEffect, useState } from "react";
import { Button, Modal, Textarea } from "@mantine/core";
import { IconFlag } from "@tabler/icons-react";
import type { NightReports, Report } from "@/interfaces/api";
import type { ColorKey } from "@/interfaces/schedule";
import { api, useApi } from "@/lib/api";
import { usePrudle } from "@/lib/blackout";
import { colorNames } from "@/lib/format";
import { usePalette } from "@/lib/palette";
import { Swatch } from "./Prudle";

const MAX_COLORS = 6;

// Reports sits at the foot of the "I saw it" panel: "Colors wrong?" opens a
// picker for what they really were, and the open reports show as one quiet
// line each. A report changes nothing until an admin accepts it. Hidden while
// Prudle hides the night (a report gives its colors away).
export function Reports({ date }: { date: string }) {
    const palette = usePalette();
    const hidden = usePrudle().hidden(date);
    const { data, reload } = useApi<NightReports>(hidden ? null : `/reports/${date}`);
    const [open, setOpen] = useState(false);
    if (hidden || !data || data.date !== date) return null;

    const shown = data.reports.filter((r) => r.status === "open");
    const mine = shown.find((r) => r.mine);
    const others = shown.filter((r) => !r.mine);
    if (!data.can_report && shown.length === 0) return null;

    const withdraw = async () => {
        await api<void>(`/me/reports/${date}`, { method: "DELETE" });
        reload();
    };

    return (
        <div className="flex flex-col gap-2 border-t border-line pt-3">
            {others.map((r) => (
                <p key={r.name} className="text-muted">
                    <span className="font-bold text-fg">{r.name}</span> says it was {colorNames(r.colors, palette)}
                    {r.note && <> &ldquo;{r.note}&rdquo;</>}
                </p>
            ))}
            {mine && (
                <p className="flex flex-wrap items-center gap-x-3 text-muted">
                    <span>
                        You said it was {colorNames(mine.colors, palette)}. An admin will check it.
                    </span>
                    {data.can_report && (
                        <>
                            <button
                                type="button"
                                className="min-h-11 px-1 underline hover:text-fg"
                                onClick={() => setOpen(true)}
                            >
                                Change
                            </button>
                            <button type="button" className="min-h-11 px-1 underline hover:text-fg" onClick={withdraw}>
                                Take back
                            </button>
                        </>
                    )}
                </p>
            )}
            {data.can_report && !mine && (
                <button
                    type="button"
                    className="inline-flex min-h-11 items-center gap-2 self-center px-1 text-muted underline hover:text-fg sm:self-start"
                    onClick={() => setOpen(true)}
                >
                    <IconFlag size={18} aria-hidden /> Colors wrong?
                </button>
            )}
            <ReportModal
                date={date}
                mine={mine}
                opened={open}
                onClose={() => setOpen(false)}
                onSent={() => {
                    setOpen(false);
                    reload();
                }}
            />
        </div>
    );
}

function ReportModal({
    date,
    mine,
    opened,
    onClose,
    onSent,
}: {
    date: string;
    mine: Report | undefined;
    opened: boolean;
    onClose: () => void;
    onSent: () => void;
}) {
    const palette = usePalette();
    const [picked, setPicked] = useState<ColorKey[]>([]);
    const [note, setNote] = useState("");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();

    useEffect(() => {
        if (!opened) return;
        setPicked(mine?.colors ?? []);
        setNote(mine?.note ?? "");
        setError(undefined);
    }, [opened, mine]);

    // the order they're picked is the order on the crown
    const toggle = (k: ColorKey) =>
        setPicked((p) => (p.includes(k) ? p.filter((x) => x !== k) : p.length < MAX_COLORS ? [...p, k] : p));

    const send = async () => {
        setBusy(true);
        setError(undefined);
        try {
            await api<Report>(`/me/reports/${date}`, {
                method: "PUT",
                body: JSON.stringify({ colors: picked, note }),
            });
            onSent();
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setBusy(false);
        }
    };

    return (
        <Modal opened={opened} onClose={onClose} title="What colors was it?" centered size="lg">
            <div className="flex flex-col gap-4">
                <p className="text-muted">Pick the colors you saw. An admin checks before the night changes.</p>
                {palette && (
                    <div className="grid grid-cols-3 gap-2 sm:grid-cols-4">
                        {[...palette.values()].map((c) => (
                            <Swatch
                                key={c.key}
                                c={c}
                                palette={palette}
                                mark={undefined}
                                picked={picked.includes(c.key)}
                                hint={false}
                                onClick={() => toggle(c.key)}
                            />
                        ))}
                    </div>
                )}
                <p className="min-h-7 text-lg" aria-live="polite">
                    {picked.length > 0 ? colorNames(picked, palette) : <span className="text-muted">Nothing picked</span>}
                </p>
                <Textarea
                    aria-label="A note"
                    placeholder="A note (optional)"
                    value={note}
                    onChange={(e) => setNote(e.currentTarget.value)}
                    maxLength={280}
                    size="md"
                    autosize
                    minRows={2}
                />
                {error && <p className="text-danger">{error}</p>}
                <Button size="lg" onClick={send} loading={busy} disabled={picked.length === 0}>
                    Send
                </Button>
            </div>
        </Modal>
    );
}
