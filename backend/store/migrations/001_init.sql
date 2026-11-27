-- The whole schema. Changes are new numbered files after this one, never
-- edits to it.

-- The schedule: one row per listed night per source page. Scraped rows are
-- never deleted, only marked removed when they disappear from their page.
create table lightings (
    id            integer primary key,
    date          text not null,           -- 2026-12-12, Boston
    program       text not null,           -- regular | 31nights
    colors        text not null,           -- json array of palette keys
    colors_raw    text not null default '',
    cause         text not null default '',
    cause_url     text not null default '',
    kind          text not null,
    source_url    text not null,
    first_seen_at text not null,
    last_seen_at  text not null,
    removed_at    text,
    unique (source_url, date, cause)
);
create index lightings_date on lightings (date);

-- admin corrections; an override replaces everything listed for its night
create table overrides (
    id         integer primary key,
    date       text not null unique,
    colors     text not null,
    cause      text not null default '',
    cause_url  text not null default '',
    kind       text not null default 'nonprofit',
    note       text not null default '',
    created_at text not null,
    updated_at text not null
);

-- what we know about each source page, for conditional requests
create table sources (
    url           text primary key,
    title         text not null default '',
    year          integer not null default 0,
    program       text not null default '',
    lastmod       text not null default '', -- from the sitemap
    etag          text not null default '',
    last_modified text not null default '', -- the Last-Modified header
    items         integer not null default 0,
    checked_at    text not null default '',
    changed_at    text not null default ''
);

create table scrape_runs (
    id          integer primary key,
    started_at  text not null,
    finished_at text,
    pages       integer not null default 0,
    fetched     integer not null default 0,
    items       integer not null default 0,
    added       integer not null default 0,
    changed     integer not null default 0,
    removed     integer not null default 0,
    warnings    text not null default '[]', -- json array of strings
    error       text not null default ''
);

-- People, their passkeys, their sign-in sessions, and the invite links that
-- let them in. Tokens are stored only as sha256 hashes.
create table users (
    id         integer primary key,
    handle     blob    not null unique, -- the passkey user handle: 32 random bytes
    name       text    not null,
    is_admin   integer not null default 0,
    prudle_off integer not null default 0, -- would rather see December's colors than play
    created_at text    not null
);
create unique index users_name on users (name collate nocase);

create table credentials (
    id           blob    primary key, -- the passkey's credential id
    user_id      integer not null references users (id) on delete cascade,
    data         blob    not null,    -- the webauthn credential as json
    created_at   text    not null,
    last_used_at text
);
create index credentials_user on credentials (user_id);

create table sessions (
    token_hash   blob    primary key,
    user_id      integer not null references users (id) on delete cascade,
    created_at   text    not null,
    last_seen_at text    not null,
    expires_at   text    not null
);
create index sessions_user on sessions (user_id);

create table invites (
    token_hash  blob    primary key,
    created_by  integer references users (id) on delete set null,
    for_user_id integer references users (id) on delete cascade, -- a re-invite adds a passkey to this user
    is_admin    integer not null default 0,
    created_at  text    not null,
    expires_at  text    not null,
    used_at     text,
    used_by     integer references users (id) on delete set null
);

-- A sighting is someone saying they saw the crown lit on a night, with an
-- optional 1-5 star rating and a short note. One per person per night.
create table sightings (
    user_id    integer not null references users (id) on delete cascade,
    date       text    not null,
    rating     integer check (rating between 1 and 5),
    note       text    not null default '',
    created_at text    not null,
    updated_at text    not null,
    primary key (user_id, date)
);
create index sightings_date on sightings (date);

-- A photo of the crown on a night someone saw it: at most one per person per
-- night, and it goes when the sighting does. Kept as re-encoded jpegs (no
-- metadata); the blobs come last so listing sightings never reads them.
create table photos (
    user_id    integer not null,
    date       text    not null,
    sha256     text    not null, -- of the full jpeg
    width      integer not null,
    height     integer not null,
    created_at text    not null,
    full       blob    not null,
    thumb      blob    not null,
    primary key (user_id, date),
    foreign key (user_id, date) references sightings (user_id, date) on delete cascade
);
create index photos_sha on photos (sha256);

-- A report is someone saying a night's colors were listed wrong, and what
-- they really were. One per person per night. Reports change nothing on
-- their own: an admin accepting one writes an override.
create table reports (
    id          integer primary key,
    user_id     integer not null references users (id) on delete cascade,
    date        text    not null,
    colors      text    not null,          -- json array of palette keys
    note        text    not null default '',
    status      text    not null default 'open', -- open | accepted | dismissed
    created_at  text    not null,
    updated_at  text    not null,
    resolved_at text,
    unique (user_id, date)
);
create index reports_status on reports (status, date);

-- A Prudle game: one person's go at guessing a December night's colors.
-- The answer is kept as it was at the first guess, so a later correction
-- doesn't regrade a game; the guesses replay to the game (backend/prudle).
create table prudle_games (
    user_id     integer not null references users (id) on delete cascade,
    date        text    not null,
    answer      text    not null, -- json color keys
    guesses     text    not null default '[]', -- json [[key, ...], ...]
    status      text    not null check (status in ('playing', 'won', 'lost', 'passed')),
    created_at  text    not null,
    finished_at text,
    primary key (user_id, date)
);
create index prudle_games_date on prudle_games (date);

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
    -- its own setup hotspot (not a home network), drawn on its Wi-Fi screen
    ap_ssid         text    not null default '',
    ap_password     text    not null default '',
    created_at      text    not null,
    claimed_at      text,
    last_checkin_at text,
    battery_mv      integer not null default 0,
    power           text    not null default '', -- usb | battery
    fw_version      text    not null default ''
);
create index devices_user on devices (user_id);

-- What a frame reports: button presses, boots, errors. They arrive in batches
-- and can be late (sent after the frame was offline). Kept for reading in
-- sqlite when a frame misbehaves; pruned after a while.
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
