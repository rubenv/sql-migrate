-- +migrate Up
UPDATE people SET name = 'test' WHERE id = 1;

-- +migrate Down
UPDATE people SET name = NULL WHERE id = 1;
