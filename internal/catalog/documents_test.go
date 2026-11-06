package catalog

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A PDF's first bytes are all the store needs to know it for one.
var tinyPDF = []byte("%PDF-1.4\n1 0 obj << >> endobj\ntrailer << >>\n%%EOF\n")

func TestDocuments(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	course, err := s.CreateCourse(ctx, CourseInput{Department: "MATH", Number: "241", Title: "Calculus III", Level: "unknown", Term: "fall", Year: 2004})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.CreateDocument(ctx, DocumentInput{Title: "  "})
	wantField(t, err, "title")
	_, err = s.CreateDocument(ctx, DocumentInput{Title: "X", DOI: "nature12373"})
	wantField(t, err, "doi")
	_, err = s.CreateDocument(ctx, DocumentInput{Title: "X", Published: "Fall 2004"})
	wantField(t, err, "published")
	_, err = s.CreateDocument(ctx, DocumentInput{Title: "X", CourseID: 999})
	wantField(t, err, "courseId")

	syllabus, err := s.CreateDocument(ctx, DocumentInput{
		Kind: "syllabus", Title: "Calculus III syllabus", Authors: []string{"Dr. Ruiz", " dr. ruiz "}, CourseID: course.ID,
		Notes: "the one with the exam dates",
	})
	if err != nil {
		t.Fatal(err)
	}
	if syllabus.Course != "MATH 241 Calculus III" || !slices.Equal(syllabus.Authors, []string{"Dr. Ruiz"}) {
		t.Fatalf("document %+v", syllabus)
	}
	article, err := s.CreateDocument(ctx, DocumentInput{
		Kind: "article", Title: "Teaching Calculus for Understanding", Authors: []string{"Ana Ruiz"},
		DOI: "https://doi.org/10.1080/00221546.1999.11780773", Container: "The Journal of Higher Education",
	})
	if err != nil {
		t.Fatal(err)
	}
	if article.DOI != "10.1080/00221546.1999.11780773" {
		t.Fatalf("DOI kept as %q", article.DOI)
	}

	// Subjects, as a book's: kept once each, and searched.
	chapter, err := s.CreateDocument(ctx, DocumentInput{
		Kind: "chapter", Title: "Causation as Folk Science", Subjects: []string{"Folk science", "Causality", "folk science", " "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(chapter.Subjects, []string{"Folk science", "Causality"}) {
		t.Fatalf("subjects %q", chapter.Subjects)
	}
	if rows, err := s.Documents(ctx, "causality", nil); err != nil || len(rows) != 1 || rows[0].ID != chapter.ID {
		t.Fatalf("a subject searched: %+v, %v", rows, err)
	}
	_, err = s.CreateDocument(ctx, DocumentInput{Title: "Again", DOI: "10.1080/00221546.1999.11780773"})
	wantConflict(t, err)

	// Files: a PDF and a photo; anything else is refused by its first bytes.
	withPDF, err := s.AddDocumentFile(ctx, syllabus.ID, `C:\Users\me\Syllabus.pdf`, bytes.NewReader(tinyPDF))
	if err != nil {
		t.Fatal(err)
	}
	withPhoto, err := s.AddDocumentFile(ctx, syllabus.ID, "Photo 2.jpg", bytes.NewReader(coverPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddDocumentFile(ctx, syllabus.ID, "notes.txt", strings.NewReader("just some text")); err == nil {
		t.Fatal("a text file was taken")
	}
	pdf, photo := withPDF.Files[0], withPhoto.Files[1]
	if pdf.Name != "Syllabus.pdf" || pdf.ContentType != "application/pdf" || pdf.TextStatus != "pending" || photo.ContentType != "image/png" {
		t.Fatalf("files %+v", withPhoto.Files)
	}

	// The text service: the PDF reads, the photo fails until its tries run out.
	read := func(_ context.Context, path, contentType string) (string, error) {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("asked to read a file that is not there: %v", err)
		}
		if contentType == "application/pdf" {
			return "Office hours Tuesday.\n\nThe final exam covers Stokes' theorem.", nil
		}
		return "", errors.New("tesseract: exit status 1")
	}
	for range 5 {
		if _, err := s.ReadNextFile(ctx, read, nil); err != nil {
			t.Fatal(err)
		}
	}
	if more, _ := s.ReadNextFile(ctx, read, nil); more {
		t.Fatal("a file was still waiting after its tries ran out")
	}
	got, err := s.GetDocument(ctx, syllabus.ID)
	if err != nil {
		t.Fatal(err)
	}
	if f := got.Files[0]; f.TextStatus != "done" || !strings.Contains(f.Text, "Stokes") {
		t.Fatalf("the PDF's text: %+v", f)
	}
	if f := got.Files[1]; f.TextStatus != "failed" || f.TextError == "" {
		t.Fatalf("the photo after three failures: %+v", f)
	}

	// Search reaches inside the files' text, by the start of a word, and gives a snippet.
	found, err := s.Documents(ctx, "stoke exam", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].ID != syllabus.ID || !strings.Contains(found[0].Snippet, "\x02") {
		t.Fatalf("search found %+v", found)
	}
	if found, _ = s.Documents(ctx, `"calculus (`, nil); len(found) != 2 {
		t.Fatalf("searching titles found %d, want 2", len(found))
	}

	// The owner's own text stands over the service's, and is searched.
	got, err = s.SetFileText(ctx, syllabus.ID, photo.ID, "Grading: homework 20%, exams 80%.")
	if err != nil {
		t.Fatal(err)
	}
	if got.Files[1].TextStatus != "owner" {
		t.Fatalf("typed text %+v", got.Files[1])
	}
	if found, _ = s.Documents(ctx, "grading", nil); len(found) != 1 {
		t.Fatal("the owner's text is not searched")
	}
	if got, _ = s.SetFileText(ctx, syllabus.ID, photo.ID, ""); got.Files[1].TextStatus != "pending" {
		t.Fatalf("clearing the text should hand it back to the service: %+v", got.Files[1])
	}

	if v := article.ForVisitor(); v.Notes != "" || v.CourseID != 0 {
		t.Fatal("a visitor sees the owner's details")
	}
	if _, _, _, err := s.FileForReading(ctx, syllabus.ID, pdf.ID, &Scope{Papers: "none"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a visitor reached a file of a paper their link does not show: %v", err)
	}
	path, name, _, err := s.FileForReading(ctx, syllabus.ID, pdf.ID, nil)
	if err != nil || name != "Syllabus.pdf" {
		t.Fatalf("the owner's file: %q %v", name, err)
	}

	detail, err := s.GetCourse(ctx, course.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Documents) != 1 || detail.Documents[0].ID != syllabus.ID {
		t.Fatalf("the course's documents %+v", detail.Documents)
	}

	got, err = s.RemoveDocumentFile(ctx, syllabus.ID, photo.ID)
	if err != nil || len(got.Files) != 1 {
		t.Fatalf("after removing a file: %+v %v", got, err)
	}
	if err := s.DeleteDocument(ctx, syllabus.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a deleted document's file is still on disk")
	}
	if found, _ = s.Documents(ctx, "stokes", nil); len(found) != 0 {
		t.Fatal("a deleted document is still found")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 0 {
		t.Fatalf("files left in the store: %v", entries)
	}
}
