"use client";

import { useState } from "react";
import Link from "next/link";
import { Badge, Button, MultiSelect, Skeleton, Switch, Table, TextInput } from "@mantine/core";
import { IconRefresh, IconTrash } from "@tabler/icons-react";
import type {
    AdminDevice,
    FrameConfig,
    OverrideInfo,
    PaletteResponse,
    ScrapeStatus,
    UserInfo,
} from "@/interfaces/api";
import { api, useApi } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { colorNames, shortDate } from "@/lib/format";
import { bandsOf, usePalette } from "@/lib/palette";
import { InviteModal } from "@/components/InviteModal";
import { Crown } from "@/components/Tower";
import { Shell } from "@/components/Shell";

// /admin: the people, corrections to the schedule, the scraper, and frames.
export default function AdminPage() {
    const { me } = useAuth();
    return (
        <Shell wide>
            {me === undefined ? (
                <Skeleton height={400} radius="lg" />
            ) : !me?.is_admin ? (
                <p className="text-lg text-muted">Admins only.</p>
            ) : (
                <div className="flex flex-col gap-10">
                    <h1 className="text-3xl font-bold">Admin</h1>
                    <People />
                    <Corrections />
                    <Scraper />
                    <Frames />
                </div>
            )}
        </Shell>
    );
}

const fmtTime = (iso: string) =>
    new Date(iso).toLocaleString("en-US", {
        month: "short",
        day: "numeric",
        hour: "numeric",
        minute: "2-digit",
    });

function People() {
    const { data } = useApi<UserInfo[]>("/admin/users");
    const [inviting, setInviting] = useState<{ id: number; name: string } | null>(null);
    const [newPerson, setNewPerson] = useState(false);
    return (
        <section className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
                <h2 className="text-xl font-bold">People</h2>
                <Button variant="default" onClick={() => setNewPerson(true)}>
                    Invite someone
                </Button>
            </div>
            {!data ? (
                <Skeleton height={120} />
            ) : (
                <Table.ScrollContainer minWidth={520}>
                    <Table verticalSpacing="sm">
                        <Table.Thead>
                            <Table.Tr>
                                <Table.Th>Name</Table.Th>
                                <Table.Th>Joined</Table.Th>
                                <Table.Th>Nights logged</Table.Th>
                                <Table.Th />
                            </Table.Tr>
                        </Table.Thead>
                        <Table.Tbody>
                            {data.map((u) => (
                                <Table.Tr key={u.id}>
                                    <Table.Td>
                                        {u.name}{" "}
                                        {u.is_admin && (
                                            <Badge size="sm" variant="light" color="gray">
                                                admin
                                            </Badge>
                                        )}
                                    </Table.Td>
                                    <Table.Td>{shortDate(u.created_at.slice(0, 10))}</Table.Td>
                                    <Table.Td>{u.sightings}</Table.Td>
                                    <Table.Td className="text-right">
                                        <Button
                                            size="xs"
                                            variant="subtle"
                                            onClick={() => setInviting({ id: u.id, name: u.name })}
                                        >
                                            New passkey link
                                        </Button>
                                    </Table.Td>
                                </Table.Tr>
                            ))}
                        </Table.Tbody>
                    </Table>
                </Table.ScrollContainer>
            )}
            <InviteModal opened={newPerson} onClose={() => setNewPerson(false)} />
            <InviteModal
                opened={inviting !== null}
                onClose={() => setInviting(null)}
                forUser={inviting ?? undefined}
            />
        </section>
    );
}

function Corrections() {
    const palette = usePalette();
    const colors = useApi<PaletteResponse>("/palette");
    const { data, reload } = useApi<OverrideInfo[]>("/admin/overrides");
    const [date, setDate] = useState("");
    const [picked, setPicked] = useState<string[]>([]);
    const [cause, setCause] = useState("");
    const [url, setURL] = useState("");
    const [note, setNote] = useState("");
    const [error, setError] = useState<string>();

    const save = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(undefined);
        try {
            await api(`/admin/overrides/${date}`, {
                method: "PUT",
                body: JSON.stringify({ colors: picked, cause, cause_url: url, note }),
            });
            setDate("");
            setPicked([]);
            setCause("");
            setURL("");
            setNote("");
            reload();
        } catch (err) {
            setError((err as Error).message);
        }
    };
    const remove = async (d: string) => {
        await api(`/admin/overrides/${d}`, { method: "DELETE" });
        reload();
    };

    return (
        <section className="flex flex-col gap-3">
            <h2 className="text-xl font-bold">Corrections</h2>
            <p className="text-muted">
                When the Prudential Center&apos;s pages are wrong or late, set the night here. A correction
                comes before what the pages say.
            </p>
            <form onSubmit={save} className="grid gap-3 rounded-xl bg-panel p-4 sm:grid-cols-2">
                <TextInput
                    type="date"
                    label="Night"
                    required
                    value={date}
                    onChange={(e) => setDate(e.currentTarget.value)}
                />
                <MultiSelect
                    label="Colors"
                    required
                    data={(colors.data?.colors ?? []).map((c) => ({ value: c.key, label: c.label }))}
                    value={picked}
                    onChange={setPicked}
                    maxValues={6}
                />
                <TextInput
                    label="For"
                    placeholder="Who or what it's for"
                    value={cause}
                    onChange={(e) => setCause(e.currentTarget.value)}
                />
                <TextInput
                    label="Link"
                    placeholder="https://"
                    value={url}
                    onChange={(e) => setURL(e.currentTarget.value)}
                />
                <TextInput
                    label="Why"
                    placeholder="e.g. the site still says blue"
                    value={note}
                    onChange={(e) => setNote(e.currentTarget.value)}
                    className="sm:col-span-2"
                />
                <div className="flex items-center gap-3 sm:col-span-2">
                    <Button type="submit" disabled={!date || picked.length === 0}>
                        Save correction
                    </Button>
                    {error && <span className="text-danger">{error}</span>}
                </div>
            </form>
            {data && data.length > 0 && (
                <ul
                    className="flex flex-col divide-y divide-line rounded-xl bg-panel"
                    aria-label="Corrections"
                >
                    {data.map((o) => (
                        <li key={o.date} className="flex items-center gap-3 px-4 py-3">
                            <Crown bands={bandsOf(o.colors, palette)} className="h-9 w-9" />
                            <Link href={`/night?d=${o.date}`} className="w-20 shrink-0 font-bold underline">
                                {shortDate(o.date)}
                            </Link>
                            <span className="min-w-0 flex-1">
                                <span className="block truncate">
                                    {colorNames(o.colors, palette)}
                                    {o.cause && ` for ${o.cause}`}
                                </span>
                                {o.note && (
                                    <span className="block truncate text-sm text-muted">{o.note}</span>
                                )}
                            </span>
                            <Button
                                size="xs"
                                variant="subtle"
                                color="red"
                                leftSection={<IconTrash size={14} />}
                                onClick={() => remove(o.date)}
                            >
                                Remove
                            </Button>
                        </li>
                    ))}
                </ul>
            )}
        </section>
    );
}

function Scraper() {
    const { data, reload } = useApi<ScrapeStatus>("/admin/scrape", 5000);
    const [error, setError] = useState<string>();
    const now = async () => {
        setError(undefined);
        try {
            await api("/admin/scrape", { method: "POST" });
            reload();
        } catch (e) {
            setError((e as Error).message);
        }
    };
    return (
        <section className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
                <h2 className="text-xl font-bold">Scraper</h2>
                <Button
                    variant="default"
                    leftSection={<IconRefresh size={16} />}
                    loading={data?.running}
                    onClick={now}
                >
                    Check now
                </Button>
            </div>
            {error && <p className="text-danger">{error}</p>}
            {!data ? (
                <Skeleton height={120} />
            ) : data.runs.length === 0 ? (
                <p className="text-muted">No checks yet.</p>
            ) : (
                <Table.ScrollContainer minWidth={560}>
                    <Table verticalSpacing="xs" aria-label="Scrape runs">
                        <Table.Thead>
                            <Table.Tr>
                                <Table.Th>When</Table.Th>
                                <Table.Th>Pages</Table.Th>
                                <Table.Th>Nights</Table.Th>
                                <Table.Th>Changes</Table.Th>
                                <Table.Th>Problems</Table.Th>
                            </Table.Tr>
                        </Table.Thead>
                        <Table.Tbody>
                            {data.runs.map((r) => (
                                <Table.Tr key={r.id}>
                                    <Table.Td>{fmtTime(r.started_at)}</Table.Td>
                                    <Table.Td>
                                        {r.fetched}/{r.pages}
                                    </Table.Td>
                                    <Table.Td>{r.items}</Table.Td>
                                    <Table.Td>
                                        +{r.added} ~{r.changed} −{r.removed}
                                    </Table.Td>
                                    <Table.Td
                                        className={
                                            r.error || r.warnings.length ? "text-danger" : "text-muted"
                                        }
                                    >
                                        {r.error || (r.warnings.length ? r.warnings.join("; ") : "none")}
                                    </Table.Td>
                                </Table.Tr>
                            ))}
                        </Table.Tbody>
                    </Table>
                </Table.ScrollContainer>
            )}
        </section>
    );
}

function Frames() {
    const { data, reload } = useApi<AdminDevice[]>("/admin/devices");
    const [confirm, setConfirm] = useState<number>();
    const [forNew, setForNew] = useState(true);
    const [prepared, setPrepared] = useState<FrameConfig>();
    const [error, setError] = useState<string>();
    const remove = async (id: number) => {
        await api(`/admin/devices/${id}`, { method: "DELETE" });
        setConfirm(undefined);
        reload();
    };
    const mark = async (id: number, on: boolean) => {
        await api(`/admin/devices/${id}/invite`, { method: on ? "POST" : "DELETE" });
        reload();
    };
    // prepare makes a frame's identity and hands it over as a file: the
    // token is only ever shown this once.
    const prepare = async () => {
        setError(undefined);
        try {
            const fc = await api<FrameConfig>("/admin/devices", {
                method: "POST",
                body: JSON.stringify({ invite: forNew }),
            });
            const url = URL.createObjectURL(
                new Blob([JSON.stringify(fc, null, 2) + "\n"], { type: "application/json" }),
            );
            const a = document.createElement("a");
            a.href = url;
            a.download = `frame-${fc.device_id}.json`;
            a.click();
            setTimeout(() => URL.revokeObjectURL(url), 1000);
            setPrepared(fc);
            reload();
        } catch (e) {
            setError((e as Error).message);
        }
    };
    return (
        <section className="flex flex-col gap-3">
            <h2 className="text-xl font-bold">Frames</h2>
            <p className="text-muted">
                Every frame that has registered, including emulators. Removing one makes it register again. A
                frame marked for a new person lets whoever scans its code make an account and get the frame in
                one step, for 30 days.
            </p>
            <div className="flex flex-col gap-3 rounded-xl bg-panel p-4">
                <div className="flex flex-wrap items-center gap-4">
                    <Button onClick={prepare}>Prepare a frame</Button>
                    <Switch
                        label="For a new person"
                        checked={forNew}
                        onChange={(e) => setForNew(e.currentTarget.checked)}
                    />
                </div>
                {prepared && (
                    <div className="flex flex-col gap-1 text-sm" role="status">
                        <span>
                            Frame #{prepared.device_id}, code <b>{prepared.claim_code}</b>: downloaded{" "}
                            <code>frame-{prepared.device_id}.json</code> (it holds the frame&apos;s token,
                            keep it private). Next, add Wi-Fi and write the files to flash:
                        </span>
                        <code className="rounded bg-raised px-2 py-1">
                            just frame-config -from frame-{prepared.device_id}.json -wifi
                            &quot;Home:password&quot; -out sd
                        </code>
                    </div>
                )}
                {error && <span className="text-danger">{error}</span>}
            </div>
            {!data ? (
                <Skeleton height={80} />
            ) : data.length === 0 ? (
                <p className="text-muted">No frames yet.</p>
            ) : (
                <Table.ScrollContainer minWidth={760}>
                    <Table verticalSpacing="sm" aria-label="Frames">
                        <Table.Thead>
                            <Table.Tr>
                                <Table.Th>Frame</Table.Th>
                                <Table.Th>Owner</Table.Th>
                                <Table.Th>Last check-in</Table.Th>
                                <Table.Th>Power</Table.Th>
                                <Table.Th>Firmware</Table.Th>
                                <Table.Th />
                            </Table.Tr>
                        </Table.Thead>
                        <Table.Tbody>
                            {data.map((d) => (
                                <Table.Tr key={d.id} data-frame={d.id}>
                                    <Table.Td>
                                        #{d.id} {d.name}
                                    </Table.Td>
                                    <Table.Td>
                                        {d.owner || (
                                            <span className="flex flex-col gap-1">
                                                <span className="text-muted">
                                                    waiting, code {d.claim_code}
                                                </span>
                                                {d.invite_until && (
                                                    <Badge size="sm" variant="light" color="yellow">
                                                        for a new person until{" "}
                                                        {shortDate(
                                                            new Date(d.invite_until)
                                                                .toISOString()
                                                                .slice(0, 10),
                                                        )}
                                                    </Badge>
                                                )}
                                            </span>
                                        )}
                                    </Table.Td>
                                    <Table.Td>
                                        {d.last_checkin_at ? fmtTime(d.last_checkin_at) : "never"}
                                    </Table.Td>
                                    <Table.Td>
                                        {d.power === "battery" && d.battery_mv > 0
                                            ? `${(d.battery_mv / 1000).toFixed(2)} V`
                                            : d.power || "—"}
                                    </Table.Td>
                                    <Table.Td>{d.firmware || "—"}</Table.Td>
                                    <Table.Td className="text-right whitespace-nowrap">
                                        {!d.owner &&
                                            (d.invite_until ? (
                                                <Button
                                                    size="xs"
                                                    variant="subtle"
                                                    onClick={() => mark(d.id, false)}
                                                >
                                                    Unmark
                                                </Button>
                                            ) : (
                                                <Button
                                                    size="xs"
                                                    variant="subtle"
                                                    onClick={() => mark(d.id, true)}
                                                >
                                                    Invite whoever sets it up
                                                </Button>
                                            ))}
                                        {confirm === d.id ? (
                                            <Button size="xs" color="red" onClick={() => remove(d.id)}>
                                                Really remove
                                            </Button>
                                        ) : (
                                            <Button
                                                size="xs"
                                                variant="subtle"
                                                color="red"
                                                leftSection={<IconTrash size={14} />}
                                                onClick={() => setConfirm(d.id)}
                                            >
                                                Remove
                                            </Button>
                                        )}
                                    </Table.Td>
                                </Table.Tr>
                            ))}
                        </Table.Tbody>
                    </Table>
                </Table.ScrollContainer>
            )}
        </section>
    );
}
