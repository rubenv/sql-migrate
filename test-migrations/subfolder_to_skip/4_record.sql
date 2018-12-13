-- +migrate Up
INSERT INTO people (id) VALUES (3);

-- +migrate Down
DELETE FROM people WHERE id=3;
