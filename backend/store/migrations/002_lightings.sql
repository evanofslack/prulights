-- one row per listed night per source page; scraped rows are never deleted,
-- only marked removed when they disappear from their page
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
