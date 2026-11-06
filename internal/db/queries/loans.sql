-- name: UpsertBorrower :one
INSERT INTO borrowers (name) VALUES (?)
ON CONFLICT (name) DO UPDATE SET name = borrowers.name
RETURNING id;

-- name: ListBorrowers :many
SELECT
    b.*,
    (SELECT count(*) FROM loans l WHERE l.borrower_id = b.id AND l.returned_on = '') AS out_count,
    (SELECT count(*) FROM loans l WHERE l.borrower_id = b.id) AS loan_count
FROM borrowers b
ORDER BY b.name;

-- name: ListBorrowerNames :many
SELECT name FROM borrowers ORDER BY name;

-- name: GetBorrower :one
SELECT
    b.*,
    (SELECT count(*) FROM loans l WHERE l.borrower_id = b.id AND l.returned_on = '') AS out_count,
    (SELECT count(*) FROM loans l WHERE l.borrower_id = b.id) AS loan_count
FROM borrowers b
WHERE b.id = ?;

-- name: BorrowerNamed :one
SELECT id FROM borrowers WHERE name = ?;

-- name: UpdateBorrower :exec
UPDATE borrowers SET name = ?, contact = ?, notes = ? WHERE id = ?;

-- name: DeleteBorrower :exec
DELETE FROM borrowers WHERE id = ?;

-- name: CreateLoan :one
INSERT INTO loans (copy_id, borrower_id, lent_on, due_on, returned_on, notes)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateLoan :exec
UPDATE loans
SET borrower_id = ?, lent_on = ?, due_on = ?, returned_on = ?, notes = ?
WHERE id = ?;

-- name: DeleteLoan :exec
DELETE FROM loans WHERE id = ?;

-- name: GetLoan :one
SELECT * FROM loan_rows WHERE id = ?;

-- name: ListLoans :many
-- Every loan: the ones out first, then the history, latest first.
SELECT * FROM loan_rows
ORDER BY returned_on <> '', COALESCE(NULLIF(returned_on, ''), lent_on) DESC, id DESC;

-- name: ListEditionLoansOut :many
SELECT * FROM loan_rows WHERE edition_id = ? AND returned_on = '' ORDER BY copy_id;

-- name: CopyOutTo :one
-- Who has a copy now, leaving out one loan: the one being changed.
SELECT b.name
FROM loans l JOIN borrowers b ON b.id = l.borrower_id
WHERE l.copy_id = sqlc.arg(copy_id) AND l.returned_on = '' AND l.id <> sqlc.arg(except_loan)
LIMIT 1;

-- name: EditionOutTo :one
SELECT l.copy_id, b.name
FROM loans l
JOIN borrowers b ON b.id = l.borrower_id
JOIN copies c ON c.id = l.copy_id
WHERE c.edition_id = ? AND l.returned_on = ''
ORDER BY l.copy_id
LIMIT 1;
