-- name: CreateCourse :one
INSERT INTO courses (school, level, department, number, title, term, year, instructor, notes)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateCourse :exec
UPDATE courses
SET school = ?, level = ?, department = ?, number = ?, title = ?, term = ?, year = ?,
    instructor = ?, notes = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: DeleteCourse :execrows
DELETE FROM courses WHERE id = ?;

-- name: GetCourse :one
SELECT c.*, (SELECT count(*) FROM course_copies cc WHERE cc.course_id = c.id) AS book_count
FROM courses c
WHERE c.id = ?;

-- name: ListCourses :many
-- Latest first: by year, then the season within it.
SELECT c.*, (SELECT count(*) FROM course_copies cc WHERE cc.course_id = c.id) AS book_count
FROM courses c
ORDER BY COALESCE(c.year, 0) DESC,
    CASE c.term WHEN 'fall' THEN 4 WHEN 'summer' THEN 3 WHEN 'spring' THEN 2 WHEN 'winter' THEN 1 ELSE 0 END DESC,
    c.department, c.number, c.title;

-- name: SetCourseCopy :exec
INSERT INTO course_copies (course_id, copy_id, required) VALUES (?, ?, ?)
ON CONFLICT (course_id, copy_id) DO UPDATE SET required = excluded.required;

-- name: RemoveCourseCopy :execrows
DELETE FROM course_copies WHERE course_id = ? AND copy_id = ?;

-- name: ListCourseBooks :many
SELECT
    cc.copy_id, cc.required, c.edition_id, e.title, e.subtitle, e.cover,
    CAST(COALESCE(ea.authors, '') AS TEXT) AS authors
FROM course_copies cc
JOIN copies c ON c.id = cc.copy_id
JOIN editions e ON e.id = c.edition_id
LEFT JOIN edition_authors ea ON ea.edition_id = c.edition_id
WHERE cc.course_id = ?
ORDER BY cc.required DESC, e.title, cc.copy_id;

-- name: ListEditionCourses :many
SELECT cc.copy_id, cc.required, co.id AS course_id
FROM course_copies cc
JOIN copies c ON c.id = cc.copy_id
JOIN courses co ON co.id = cc.course_id
WHERE c.edition_id = ?
ORDER BY COALESCE(co.year, 0) DESC, co.id;

-- name: ListSchools :many
SELECT DISTINCT school FROM courses WHERE school <> '' ORDER BY school;

-- name: ListDepartments :many
SELECT DISTINCT department FROM courses WHERE department <> '' ORDER BY department;

-- name: ListInstructors :many
SELECT DISTINCT instructor FROM courses WHERE instructor <> '' ORDER BY instructor;
