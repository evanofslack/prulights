-- A Prudle game: one person's go at guessing a December night's colors.
-- The answer is kept as it was at the first guess, so a later correction
-- doesn't regrade a game; the guesses replay to the game (backend/prudle).
create table prudle_games (
    user_id     integer not null references users (id) on delete cascade,
    date        text    not null,
    answer      text    not null, -- json color keys
    guesses     text    not null default '[]', -- json [[key, ...], ...]
    status      text    not null check (status in ('playing', 'won', 'lost', 'passed')),
    misses      real    not null default 0,
    turns       integer not null default 0,
    created_at  text    not null,
    finished_at text,
    primary key (user_id, date)
);
create index prudle_games_date on prudle_games (date);

-- prudle_off: the person would rather see December's colors than play.
alter table users add column prudle_off integer not null default 0;
