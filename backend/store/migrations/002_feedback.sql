-- Feedback is a member's idea or complaint about the site, for the admins to
-- read. done_at is set when an admin has dealt with it.
create table feedback (
    id         integer primary key,
    user_id    integer not null references users (id) on delete cascade,
    body       text    not null,
    created_at text    not null,
    done_at    text
);
create index feedback_created on feedback (created_at);
