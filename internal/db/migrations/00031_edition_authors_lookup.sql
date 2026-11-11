-- +goose Up

-- An edition's author line is found for that edition alone, through the contributors'
-- primary key: grouped over every contributor, one loan's or one course's books built the
-- line for the whole library first. An edition with no author has an empty line.
DROP VIEW edition_authors;
CREATE VIEW edition_authors AS
SELECT
    e.id AS edition_id,
    CAST(COALESCE((
        SELECT group_concat(name, ', ')
        FROM (
            SELECT p.name
            FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
            WHERE ec.edition_id = e.id AND ec.role = 'author'
            ORDER BY ec.position
        )
    ), '') AS TEXT) AS authors
FROM editions e;

-- +goose Down
DROP VIEW edition_authors;
CREATE VIEW edition_authors AS
SELECT edition_id, CAST(group_concat(name, ', ') AS TEXT) AS authors
FROM (
    SELECT ec.edition_id, p.name
    FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
    WHERE ec.role = 'author'
    ORDER BY ec.edition_id, ec.position
)
GROUP BY edition_id;
