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
