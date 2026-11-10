-- A frame can carry an invite: while it's marked, whoever scans its code
-- signed out makes an account and claims it in one step. And each frame has
-- its own setup hotspot (not a home network), drawn on its Wi-Fi screen.
alter table devices add column invite_until text;
alter table devices add column invited_by integer references users (id) on delete set null;
alter table devices add column ap_ssid text not null default '';
alter table devices add column ap_password text not null default '';
