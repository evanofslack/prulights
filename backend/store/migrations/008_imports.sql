-- Imports are one-off data loads that run themselves once per database (the
-- archived 2025 schedule, from backend/history). name says which; a new name
-- runs again.
create table imports (
    name       text primary key,
    applied_at text not null
);
