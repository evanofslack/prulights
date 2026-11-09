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
