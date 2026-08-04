-- +migrate Up
ALTER TABLE people ADD COLUMN name varchar(255);

-- +migrate Down
ALTER TABLE people DROP COLUMN name varchar(255);
