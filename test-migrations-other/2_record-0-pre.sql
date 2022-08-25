-- +migrate Up
INSERT INTO people_test (id) VALUES (4);

-- +migrate Down
DELETE FROM people_test WHERE id=4;
