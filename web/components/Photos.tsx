"use client";

import { useRef, useState } from "react";
import { Button, Modal } from "@mantine/core";
import { IconCamera, IconEyeOff } from "@tabler/icons-react";
import type { Sighting } from "@/interfaces/api";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { photoURL, uploadPhoto } from "@/lib/photo";

// Thumb is a sighting's photo, small; a tap opens it full screen. While
// Prudle hides the night it's covered (a photo shows the colors).
export function Thumb({
    sighting,
    covered,
    onRemoved,
    size = "h-20 w-20",
}: {
    sighting: Sighting;
    covered: boolean;
    onRemoved: () => void;
    size?: string;
}) {
    const [open, setOpen] = useState(false);
    const p = sighting.photo;
    if (!p) return null;
    if (covered) {
        return (
            <span
                className={`${size} flex shrink-0 flex-col items-center justify-center gap-1 rounded-lg bg-raised p-1 text-center text-xs text-muted`}
                title="Play tonight's Prudle to see it"
            >
                <IconEyeOff size={18} aria-hidden /> Photo hidden
            </span>
        );
    }
    return (
        <>
            <button
                type="button"
                onClick={() => setOpen(true)}
                className={`${size} shrink-0 overflow-hidden rounded-lg border border-line`}
                aria-label={`${sighting.mine ? "Your" : `${sighting.name}'s`} photo, larger`}
            >
                {/* eslint-disable-next-line @next/next/no-img-element -- a members-only api image, not a static asset */}
                <img
                    src={photoURL(p, "thumb")}
                    alt=""
                    loading="lazy"
                    width={p.width}
                    height={p.height}
                    className="h-full w-full object-cover"
                />
            </button>
            <Viewer sighting={sighting} opened={open} onClose={() => setOpen(false)} onRemoved={onRemoved} />
        </>
    );
}

function Viewer({
    sighting,
    opened,
    onClose,
    onRemoved,
}: {
    sighting: Sighting;
    opened: boolean;
    onClose: () => void;
    onRemoved: () => void;
}) {
    const { me } = useAuth();
    const p = sighting.photo!;
    const takeDown = async () => {
        await api<void>(`/admin/photos/${p.sha256}`, { method: "DELETE" });
        onClose();
        onRemoved();
    };
    return (
        <Modal
            opened={opened}
            onClose={onClose}
            fullScreen
            title={sighting.mine ? "Your photo" : `${sighting.name}'s photo`}
        >
            <figure className="flex flex-col items-center gap-3">
                {/* eslint-disable-next-line @next/next/no-img-element -- a members-only api image */}
                <img
                    src={photoURL(p, "full")}
                    alt={`The Prudential Tower, photographed by ${sighting.mine ? "you" : sighting.name}`}
                    width={p.width}
                    height={p.height}
                    className="h-auto max-h-[80vh] w-auto max-w-full rounded-lg"
                />
                {sighting.note && <figcaption className="text-lg">&ldquo;{sighting.note}&rdquo;</figcaption>}
                {me?.is_admin && !sighting.mine && (
                    <Button variant="subtle" color="red" onClick={takeDown}>
                        Remove photo
                    </Button>
                )}
            </figure>
        </Modal>
    );
}

// MyPhoto is your photo of a night you saw: add one, or replace or remove it.
// One per night.
export function MyPhoto({ mine, canAdd, onChange }: { mine: Sighting; canAdd: boolean; onChange: () => void }) {
    const input = useRef<HTMLInputElement>(null);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string>();

    const pick = async (file: File | undefined) => {
        if (!file) return;
        setBusy(true);
        setError(undefined);
        try {
            await uploadPhoto(mine.date, file);
            onChange();
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setBusy(false);
            if (input.current) input.current.value = "";
        }
    };
    const remove = async () => {
        setBusy(true);
        try {
            await api<void>(`/me/sightings/${mine.date}/photo`, { method: "DELETE" });
            onChange();
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setBusy(false);
        }
    };

    if (!mine.photo && !canAdd) return null;
    return (
        <div className="flex flex-col gap-2">
            <input
                ref={input}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={(e) => pick(e.currentTarget.files?.[0])}
            />
            {mine.photo ? (
                <div className="flex items-center gap-3">
                    <Thumb sighting={mine} covered={false} onRemoved={onChange} size="h-24 w-24" />
                    <div className="flex flex-wrap gap-x-3">
                        {canAdd && (
                            <button
                                type="button"
                                className="min-h-11 px-1 text-muted underline hover:text-fg"
                                onClick={() => input.current?.click()}
                                disabled={busy}
                            >
                                {busy ? "Sending…" : "Replace"}
                            </button>
                        )}
                        <button
                            type="button"
                            className="min-h-11 px-1 text-muted underline hover:text-fg"
                            onClick={remove}
                            disabled={busy}
                        >
                            Remove
                        </button>
                    </div>
                </div>
            ) : (
                <Button
                    size="md"
                    variant="default"
                    leftSection={<IconCamera size={20} />}
                    loading={busy}
                    onClick={() => input.current?.click()}
                    className="self-center sm:self-start"
                >
                    Add a photo
                </Button>
            )}
            {error && <p className="text-danger">{error}</p>}
        </div>
    );
}
