-- name: KeepProposal :exec
INSERT INTO lookup_proposals (edition_id, proposal) VALUES (?, ?)
ON CONFLICT (edition_id) DO UPDATE SET proposal = excluded.proposal;

-- name: GetProposal :one
SELECT proposal FROM lookup_proposals WHERE edition_id = ?;

-- name: ListProposalBooks :many
-- The books the sources have a proposal for, in the order the page lists them.
SELECT e.id, e.title, e.cover, CAST(COALESCE(ea.authors, '') AS TEXT) AS authors
FROM lookup_proposals p
JOIN editions e ON e.id = p.edition_id
LEFT JOIN edition_authors ea ON ea.edition_id = e.id
ORDER BY e.title COLLATE NOCASE, e.id;

-- name: ListDeclinedSuggestions :many
SELECT field, value FROM declined_suggestions WHERE edition_id = ?;

-- name: DeclineSuggestion :exec
INSERT INTO declined_suggestions (edition_id, field, value) VALUES (?, ?, ?)
ON CONFLICT (edition_id, field) DO UPDATE SET value = excluded.value;
