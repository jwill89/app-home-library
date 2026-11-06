-- +goose Up

-- A name the owner said is no series (a publisher's line, such as Penguin classics, that a
-- source gives as one): its books left it on that day, and no lookup or import puts a book in
-- it again. The owner typing the name makes it a series again.
ALTER TABLE series ADD COLUMN dismissed_on TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE series DROP COLUMN dismissed_on;
