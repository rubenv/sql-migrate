-- +gorp Up
INSERT INTO people (id) VALUES (1);

-- +gorp Down
DELETE FROM people WHERE id=1;
