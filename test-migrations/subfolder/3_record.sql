-- +migrate Up
INSERT INTO people (id) VALUES (2);

-- +migrate Down
DELETE FROM people WHERE id=2;
