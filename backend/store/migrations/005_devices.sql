-- Frames (and the emulator) that show the schedule. A frame registers on its
-- own, shows a claim code, and someone signed in claims it. Tokens are stored
-- only as sha256 hashes.
create table devices (
    id              integer primary key,
    token_hash      blob    not null unique,
    user_id         integer references users (id) on delete set null, -- who claimed it
    name            text    not null default '',
    claim_code      text    unique,  -- set while unclaimed
    settings        text    not null default '{}', -- json
    created_at      text    not null,
    claimed_at      text,
    last_checkin_at text,
    battery_mv      integer not null default 0,
    power           text    not null default '', -- usb | battery
    fw_version      text    not null default ''
);
create index devices_user on devices (user_id);

-- What a frame reports: button presses, boots, errors. They arrive in batches
-- and can be late (sent after the frame was offline).
create table device_events (
    id          integer primary key,
    device_id   integer not null references devices (id) on delete cascade,
    type        text    not null,
    at          text    not null,
    date        text    not null default '',
    detail      text    not null default '',
    received_at text    not null
);
create index device_events_device on device_events (device_id, at);

-- Rendered frames, by the sha256 of their packed pixels. A bundle lists
-- them; frames download the ones they don't have.
create table device_images (
    sha256       text primary key,
    bin          blob not null, -- gzipped panel framebuffer
    created_at   text not null,
    last_used_at text not null
);
