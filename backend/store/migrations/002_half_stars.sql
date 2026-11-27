-- Ratings go in half stars: half_stars is the rating times two (1 is half a
-- star, 10 is five). It's a new column rather than a rebuilt table because
-- dropping sightings would cascade to photos (migrations run in a transaction,
-- where foreign keys can't be turned off). The old whole-star rating column is
-- copied over and no longer written.
alter table sightings add column half_stars integer check (half_stars between 1 and 10);
update sightings set half_stars = rating * 2 where rating is not null;
