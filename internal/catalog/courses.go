package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"homelibrary/internal/db"
)

// CourseInput is a course as the owner records it.
type CourseInput struct {
	School     string `json:"school" maxLength:"200"`
	Level      string `json:"level" enum:"unknown,high_school,undergraduate,graduate,professional,other"`
	Department string `json:"department" maxLength:"40" doc:"MATH, ENGL"`
	Number     string `json:"number" maxLength:"20" doc:"241, 101H"`
	Title      string `json:"title" maxLength:"300"`
	Term       string `json:"term" enum:"unknown,winter,spring,summer,fall"`
	Year       int64  `json:"year" minimum:"0" maximum:"2100" doc:"0 when not known"`
	Instructor string `json:"instructor" maxLength:"200"`
	Notes      string `json:"notes" maxLength:"5000"`
}

// Course is a course a book was used for.
type Course struct {
	ID int64 `json:"id"`
	CourseInput
	BookCount int64 `json:"bookCount"`
}

// CourseBook is a copy a course used, with the book it is a copy of.
type CourseBook struct {
	CopyID   int64  `json:"copyId"`
	Barcode  string `json:"barcode"`
	BookID   int64  `json:"bookId"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Authors  string `json:"authors"`
	Cover    Cover  `json:"cover"`
	Required bool   `json:"required"`
}

// CourseDetail is a course with its books and documents, as its page shows it.
type CourseDetail struct {
	Course
	Books     []CourseBook  `json:"books"`
	Documents []DocumentRow `json:"documents"`
}

// CourseUse is a course one of a book's copies was used for, as the book's page lists it.
type CourseUse struct {
	Course   Course `json:"course"`
	CopyID   int64  `json:"copyId"`
	Required bool   `json:"required"`
}

func (in *CourseInput) normalize() error {
	for _, f := range []*string{&in.School, &in.Department, &in.Number, &in.Title, &in.Instructor, &in.Notes} {
		*f = strings.TrimSpace(*f)
	}
	if in.Title == "" && in.Number == "" {
		return &FieldError{"title", "a course needs a title or a number"}
	}
	if in.Year != 0 && in.Year < 1900 {
		return &FieldError{"year", "write the year in full, like 2004"}
	}
	return nil
}

func (in CourseInput) params() db.CreateCourseParams {
	return db.CreateCourseParams{
		School: in.School, Level: storedChoice(in.Level), Department: in.Department, Number: in.Number,
		Title: in.Title, Term: storedChoice(in.Term), Year: nullInt(in.Year), Instructor: in.Instructor,
		Notes: in.Notes,
	}
}

// CreateCourse records a course.
func (s *Service) CreateCourse(ctx context.Context, in CourseInput) (*CourseDetail, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	id, err := db.New(s.DB.Write).CreateCourse(ctx, in.params())
	if err != nil {
		return nil, fmt.Errorf("create course: %w", err)
	}
	return s.GetCourse(ctx, id)
}

// UpdateCourse replaces a course's details.
func (s *Service) UpdateCourse(ctx context.Context, id int64, in CourseInput) (*CourseDetail, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	err := s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetCourse(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		p := in.params()
		return q.UpdateCourse(ctx, db.UpdateCourseParams{
			School: p.School, Level: p.Level, Department: p.Department, Number: p.Number, Title: p.Title,
			Term: p.Term, Year: p.Year, Instructor: p.Instructor, Notes: p.Notes, ID: id,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.GetCourse(ctx, id)
}

// DeleteCourse removes a course. Its books stay in the library.
func (s *Service) DeleteCourse(ctx context.Context, id int64) error {
	n, err := db.New(s.DB.Write).DeleteCourse(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Courses lists every course, latest first.
func (s *Service) Courses(ctx context.Context) ([]Course, error) {
	rows, err := s.read().ListCourses(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Course, 0, len(rows))
	for _, c := range rows {
		out = append(out, courseOf((*db.GetCourseRow)(c)))
	}
	return out, nil
}

// GetCourse returns a course with its books.
func (s *Service) GetCourse(ctx context.Context, id int64) (*CourseDetail, error) {
	q := s.read()
	c, err := q.GetCourse(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.ListCourseBooks(ctx, id)
	if err != nil {
		return nil, err
	}
	docs, err := courseDocuments(ctx, q, id)
	if err != nil {
		return nil, err
	}
	out := &CourseDetail{Course: courseOf(c), Books: make([]CourseBook, 0, len(rows)), Documents: docs}
	for _, b := range rows {
		out.Books = append(out.Books, CourseBook{
			CopyID: b.CopyID, Barcode: Barcode(b.CopyID), BookID: b.EditionID, Title: b.Title,
			Subtitle: b.Subtitle, Authors: b.Authors, Cover: s.cover(b.Cover), Required: b.Required == 1,
		})
	}
	return out, nil
}

// UseInCourse records that a course used a copy, or changes whether it was required.
func (s *Service) UseInCourse(ctx context.Context, courseID, copyID int64, required bool) (*CourseDetail, error) {
	err := s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetCourse(ctx, courseID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if _, err := q.GetCopy(ctx, copyID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		return q.SetCourseCopy(ctx, db.SetCourseCopyParams{CourseID: courseID, CopyID: copyID, Required: boolInt(required)})
	})
	if err != nil {
		return nil, err
	}
	return s.GetCourse(ctx, courseID)
}

// LeaveCourse takes a copy off a course's list. The copy stays in the library.
func (s *Service) LeaveCourse(ctx context.Context, courseID, copyID int64) error {
	n, err := db.New(s.DB.Write).RemoveCourseCopy(ctx, db.RemoveCourseCopyParams{CourseID: courseID, CopyID: copyID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// bookCourses lists the courses an edition's copies were used for.
func bookCourses(ctx context.Context, q *db.Queries, editionID int64) ([]CourseUse, error) {
	rows, err := q.ListEditionCourses(ctx, editionID)
	if err != nil {
		return nil, err
	}
	out := make([]CourseUse, 0, len(rows))
	for _, r := range rows {
		c, err := q.GetCourse(ctx, r.CourseID)
		if err != nil {
			return nil, fmt.Errorf("course %d: %w", r.CourseID, err)
		}
		out = append(out, CourseUse{Course: courseOf(c), CopyID: r.CopyID, Required: r.Required == 1})
	}
	return out, nil
}

func courseOf(c *db.GetCourseRow) Course {
	return Course{
		ID: c.ID, BookCount: c.BookCount,
		CourseInput: CourseInput{
			School: c.School, Level: shownChoice(c.Level), Department: c.Department, Number: c.Number,
			Title: c.Title, Term: shownChoice(c.Term), Year: c.Year.Int64, Instructor: c.Instructor,
			Notes: c.Notes,
		},
	}
}
