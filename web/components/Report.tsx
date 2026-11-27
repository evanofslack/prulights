"use client";

import { useEffect, useState } from "react";
import { Button, Modal, Textarea } from "@mantine/core";
import type { NightReports, Report, ReportRequest } from "@/interfaces/api";
import type { ColorKey } from "@/interfaces/schedule";
import { api, json, useApi } from "@/lib/api";
import { usePrudle } from "@/lib/blackout";
import { colorNames } from "@/lib/format";
import { usePalette } from "@/lib/palette";
import { ColorGrid } from "./Prudle";

const MAX_COLORS = 6;

// useReports is a night's open color reports ("Colors wrong?"), for the "I saw
// it" panel to show beside each person (components/SawIt.tsx). A report
// changes nothing until an admin accepts it. Undefined while loading, and while
// Prudle hides the night (a report gives its colors away).
export function useReports(date: string) {
    const hidden = usePrudle().hidden(date);
    const { data, reload } = useApi<NightReports>(hidden ? null : `/reports/${date}`);
    if (hidden || !data || data.date !== date) return undefined;
    const open = data.reports.filter((r) => r.status === "open");
    return {
        mine: open.find((r) => r.mine),
        others: open.filter((r) => !r.mine),
        canReport: data.can_report,
        reload,
        withdraw: async () => {
            await api<void>(`/me/reports/${date}`, { method: "DELETE" });
            reload();
        },
    };
}

// ReportModal picks the colors someone really saw, and an optional note.
export function ReportModal({
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
                body: json<ReportRequest>({ colors: picked, note }),
            });
            onSent();
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setBusy(false);
        }
    };

    return (
        <Modal opened={opened} onClose={onClose} title="What colors was it?" centered size="md">
            <div className="flex flex-col gap-4">
                <p className="text-muted">Pick the colors you saw.</p>
                {palette && (
                    <ColorGrid palette={palette} picked={(k) => picked.includes(k)} onClick={toggle} />
                )}
                <p className="min-h-7 text-lg" aria-live="polite">
                    {picked.length > 0 ? (
                        colorNames(picked, palette)
                    ) : (
                        <span className="text-muted">Nothing picked</span>
                    )}
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
