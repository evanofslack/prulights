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
