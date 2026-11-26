-- Each frame has its own setup hotspot (not a home network), drawn on its
-- Wi-Fi screen.
alter table devices add column ap_ssid text not null default '';
alter table devices add column ap_password text not null default '';
