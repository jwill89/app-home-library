package catalog

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

// FileLimit is the largest file a document takes: a long scanned PDF, or a phone's photo.
const FileLimit = 40 << 20

// DocumentInput is a document as the owner records it.
type DocumentInput struct {
	Kind      string   `json:"kind" enum:"article,chapter,review,handout,syllabus,notes,exam,other"`
	Title     string   `json:"title" minLength:"1" maxLength:"500"`
	Authors   []string `json:"authors" maxItems:"40"`
	Editors   []string `json:"editors" maxItems:"20" doc:"The editors of the book a chapter is in"`
	Container string   `json:"container" maxLength:"300" doc:"The journal, book, or site it appeared in"`
	Volume    string   `json:"volume" maxLength:"20"`
	Issue     string   `json:"issue" maxLength:"20"`
	Pages     string   `json:"pages" maxLength:"30"`
	Published string   `json:"published" maxLength:"10" doc:"YYYY, YYYY-MM, or YYYY-MM-DD"`
	Publisher string   `json:"publisher" maxLength:"300"`
	DOI       string   `json:"doi" maxLength:"200"`
	URL       string   `json:"url" maxLength:"2000"`
	CourseID  int64    `json:"courseId" minimum:"0" doc:"The course it is from; 0 for none"`
	Abstract  string   `json:"abstract" maxLength:"20000" doc:"What it argues, as the article or chapter gives it; a visitor sees it, as the subjects"`
	Notes     string   `json:"notes" maxLength:"20000"`
	Subjects  []string `json:"subjects" maxItems:"30" doc:"What it is about, as a book's subjects are"` // documentSubjectLimit
}

// documentSubjectLimit is the most subjects a document takes, as DocumentInput's maxItems says.
const documentSubjectLimit = 30

// DocumentFile is one of a document's files, with what was read from it.
type DocumentFile struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"contentType" enum:"application/pdf,image/jpeg,image/png,image/webp"`
	Size        int64  `json:"size"`
	TextStatus  string `json:"textStatus" enum:"pending,done,empty,failed,owner" doc:"Waiting for the text service, read, nothing to read, could not be read, or typed by the owner"`
	Text        string `json:"text"`
	TextError   string `json:"textError"`
}

// Document is a document with its files, as its page shows it.
type Document struct {
	ID int64 `json:"id"`
	DocumentInput
	Course    string         `json:"course"`
	Files     []DocumentFile `json:"files"`
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
}

// DocumentRow is one document in the list. Snippet is set for a search: the words around
// what matched, the matches between \x02 and \x03.
type DocumentRow struct {
	ID        int64    `json:"id"`
	Kind      string   `json:"kind"`
	Title     string   `json:"title"`
	Authors   []string `json:"authors"`
	Container string   `json:"container"`
	Published string   `json:"published"`
	DOI       string   `json:"doi"`
	CourseID  int64    `json:"courseId"`
	Course    string   `json:"course"`
	Files     int64    `json:"files"`
	Reading   int64    `json:"reading" doc:"Files still waiting for the text service"`
	AddedAt   string   `json:"addedAt"`
	Snippet   string   `json:"snippet"`
}

// ForVisitor is a document's row as a share link shows it: without its course, or the files
// still being read.
func (r DocumentRow) ForVisitor() DocumentRow {
	r.CourseID, r.Course, r.Reading = 0, "", 0
	return r
}

// ForVisitor is a document as a share link shows it: without the owner's notes, the
// course, or the text read from its files.
func (d *Document) ForVisitor() *Document {
	v := *d
	v.Notes, v.Course, v.CourseID = "", "", 0
	v.Files = make([]DocumentFile, len(d.Files))
	for i, f := range d.Files {
		v.Files[i] = DocumentFile{ID: f.ID, Name: f.Name, ContentType: f.ContentType, Size: f.Size, TextStatus: "done"}
	}
	return &v
}

var documentKinds = []string{"article", "chapter", "review", "handout", "syllabus", "notes", "exam", "other"}

func (in *DocumentInput) normalize() error {
	for _, f := range []*string{&in.Title, &in.Container, &in.Volume, &in.Issue, &in.Pages, &in.Published, &in.Publisher, &in.URL, &in.Abstract, &in.Notes} {
		*f = strings.TrimSpace(*f)
	}
	if in.Title == "" {
		return &FieldError{"title", "a document needs a title"}
	}
	if in.Kind == "" {
		in.Kind = "other"
	}
	if !slices.Contains(documentKinds, in.Kind) {
		return &FieldError{"kind", "not a kind of document the library knows"}
	}
	in.Authors = cleanList(in.Authors)
	in.Editors = cleanList(in.Editors)
	in.Subjects = cleanList(in.Subjects)
	in.Pages = strings.ReplaceAll(in.Pages, "–", "-")
	if in.Published != "" && !publishedRe.MatchString(in.Published) {
		return &FieldError{"published", "write it as YYYY, YYYY-MM, or YYYY-MM-DD"}
	}
	if d := strings.TrimSpace(in.DOI); d != "" {
		doi, ok := lookup.NormalizeDOI(d)
		if !ok {
			return &FieldError{"doi", "a DOI starts with 10. and has a slash, like 10.1038/nature12373"}
		}
		in.DOI = doi
	} else {
		in.DOI = ""
	}
	if in.URL != "" && !strings.HasPrefix(in.URL, "https://") && !strings.HasPrefix(in.URL, "http://") {
		return &FieldError{"url", "a link starts with https://"}
	}
	return nil
}

func (in DocumentInput) params() db.CreateDocumentParams {
	authors, _ := json.Marshal(in.Authors)
	editors, _ := json.Marshal(in.Editors)
	subjects, _ := json.Marshal(in.Subjects)
	course := sql.NullInt64{}
	if in.CourseID > 0 {
		course = sql.NullInt64{Int64: in.CourseID, Valid: true}
	}
	return db.CreateDocumentParams{
		Kind: in.Kind, Title: in.Title, Authors: string(authors), Container: in.Container, Volume: in.Volume,
		Issue: in.Issue, Pages: in.Pages, Published: in.Published, Publisher: in.Publisher, Doi: in.DOI,
		Url: in.URL, CourseID: course, Notes: in.Notes, Subjects: string(subjects),
		Abstract: in.Abstract, Editors: string(editors),
	}
}

// checkDocument refuses a DOI another document has and a course that is not there.
func checkDocument(ctx context.Context, q *db.Queries, id int64, in DocumentInput) error {
	if in.DOI != "" {
		other, err := q.DocumentByDOI(ctx, in.DOI)
		if err == nil && other != id {
			d, _ := q.GetDocument(ctx, other)
			title := "another document"
			if d != nil {
				title = "“" + d.Title + "”"
			}
			return &ConflictError{"The library already has this DOI, on " + title + "."}
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	if in.CourseID > 0 {
		if _, err := q.GetCourse(ctx, in.CourseID); errors.Is(err, sql.ErrNoRows) {
			return &FieldError{"courseId", "that course is not in the library"}
		} else if err != nil {
			return err
		}
	}
	return nil
}

// CreateDocument records a document. Its files come after, one upload at a time.
func (s *Service) CreateDocument(ctx context.Context, in DocumentInput) (*Document, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	var id int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		if err := checkDocument(ctx, q, 0, in); err != nil {
			return err
		}
		var err error
		if id, err = q.CreateDocument(ctx, in.params()); err != nil {
			return fmt.Errorf("create document: %w", err)
		}
		return index(ctx, q, id)
	})
	if err != nil {
		return nil, err
	}
	return s.GetDocument(ctx, id)
}

// UpdateDocument replaces a document's details; its files stay.
func (s *Service) UpdateDocument(ctx context.Context, id int64, in DocumentInput) (*Document, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	err := s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetDocument(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := checkDocument(ctx, q, id, in); err != nil {
			return err
		}
		p := in.params()
		if err := q.UpdateDocument(ctx, db.UpdateDocumentParams{
			Kind: p.Kind, Title: p.Title, Authors: p.Authors, Container: p.Container, Volume: p.Volume,
			Issue: p.Issue, Pages: p.Pages, Published: p.Published, Publisher: p.Publisher, Doi: p.Doi,
			Url: p.Url, CourseID: p.CourseID, Notes: p.Notes, Subjects: p.Subjects, Abstract: p.Abstract, Editors: p.Editors, ID: id,
		}); err != nil {
			return fmt.Errorf("update document: %w", err)
		}
		return index(ctx, q, id)
	})
	if err != nil {
		return nil, err
	}
	return s.GetDocument(ctx, id)
}

// DeleteDocument removes a document, its files, and its place in the search.
func (s *Service) DeleteDocument(ctx context.Context, id int64) error {
	var stored []string
	err := s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetDocument(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		fs, err := q.ListDocumentFiles(ctx, id)
		if err != nil {
			return err
		}
		for _, f := range fs {
			stored = append(stored, f.Stored)
		}
		if err := q.ClearDocumentSearch(ctx, id); err != nil {
			return err
		}
		return q.DeleteDocument(ctx, id)
	})
	if err != nil {
		return err
	}
	s.removeFiles(stored)
	return nil
}

func (s *Service) removeFiles(stored []string) {
	for _, name := range stored {
		if err := s.Files.Remove(name); err != nil {
			s.Logger.Warn("document file left behind", "file", name, "err", err)
		}
	}
}

// GetDocument returns a document with its files.
func (s *Service) GetDocument(ctx context.Context, id int64) (*Document, error) {
	q := s.read()
	d, err := q.GetDocument(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := &Document{
		ID: d.ID, Course: tidyName(d.CourseName), CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Files: []DocumentFile{},
		DocumentInput: DocumentInput{
			Kind: d.Kind, Title: d.Title, Authors: names(d.Authors), Container: d.Container, Volume: d.Volume,
			Issue: d.Issue, Pages: d.Pages, Published: d.Published, Publisher: d.Publisher, DOI: d.Doi, URL: d.Url,
			CourseID: d.CourseID.Int64, Notes: d.Notes, Subjects: names(d.Subjects), Abstract: d.Abstract, Editors: names(d.Editors),
		},
	}
	fs, err := q.ListDocumentFiles(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, f := range fs {
		out.Files = append(out.Files, DocumentFile{
			ID: f.ID, Name: f.Name, ContentType: f.ContentType, Size: f.Size,
			TextStatus: f.TextStatus, Text: f.Text, TextError: f.TextError,
		})
	}
	return out, nil
}

// Documents lists every document, newest first, or with words to search for, those whose
// details or files have them, best match first. A share link's has only the papers it shows.
func (s *Service) Documents(ctx context.Context, words string, sc *Scope) ([]DocumentRow, error) {
	rows, err := s.read().ListDocuments(ctx)
	if err != nil {
		return nil, err
	}
	all := make([]DocumentRow, 0, len(rows))
	for _, r := range rows {
		row := DocumentRow{
			ID: r.ID, Kind: r.Kind, Title: r.Title, Authors: names(r.Authors), Container: r.Container,
			Published: r.Published, DOI: r.Doi, CourseID: r.CourseID.Int64, Course: tidyName(r.CourseName),
			Files: r.FileCount, Reading: r.PendingCount, AddedAt: r.CreatedAt,
		}
		if sc != nil {
			if !sc.ShowsPaper(row.ID, row.CourseID) {
				continue
			}
			row = row.ForVisitor()
		}
		all = append(all, row)
	}
	match := matchQuery(words)
	if match == "" {
		return all, nil
	}
	// A visitor's words are matched, and quoted, only in what a visitor may read of a paper:
	// never the owner's notes, nor the text read from its files.
	if sc != nil {
		match = "{title authors container subjects abstract} : (" + match + ")"
	}
	found, err := s.read().SearchDocuments(ctx, match)
	if err != nil {
		return nil, err
	}
	out := []DocumentRow{}
	for _, f := range found {
		for _, r := range all {
			if r.ID == f.DocumentID {
				r.Snippet = f.Snippet
				out = append(out, r)
			}
		}
	}
	return out, nil
}

// matchQuery turns what the owner typed into an FTS5 query: every word must appear, each as
// the start of a word, and nothing typed is taken for FTS5's own syntax.
func matchQuery(words string) string {
	var terms []string
	for _, w := range strings.FieldsFunc(words, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		terms = append(terms, `"`+w+`"*`)
	}
	return strings.Join(terms, " ")
}

// index rewrites a document's row in the full-text index from its details and its files' text.
func index(ctx context.Context, q *db.Queries, id int64) error {
	d, err := q.GetDocument(ctx, id)
	if err != nil {
		return err
	}
	body, err := q.DocumentBody(ctx, id)
	if err != nil {
		return err
	}
	return q.IndexDocument(ctx, db.IndexDocumentParams{
		DocumentID: id, Title: d.Title, Authors: strings.Join(slices.Concat(names(d.Authors), names(d.Editors)), "; "),
		Container: d.Container, Subjects: strings.Join(names(d.Subjects), "; "), Abstract: d.Abstract, Notes: d.Notes, Body: body,
	})
}

// AddDocumentFile stores an upload as the document's next file and queues it for the text
// service.
func (s *Service) AddDocumentFile(ctx context.Context, id int64, name string, r io.Reader) (*Document, error) {
	if _, err := s.read().GetDocument(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	stored, contentType, size, err := s.Files.Save(r, FileLimit)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, `\`, "/")))
	if name == "" || name == "." || name == "/" {
		name = "File"
	}
	if r := []rune(name); len(r) > 120 {
		name = string(r[:120])
	}
	err = s.inTx(ctx, func(q *db.Queries) error {
		pos, err := q.NextFilePosition(ctx, id)
		if err != nil {
			return err
		}
		_, err = q.CreateDocumentFile(ctx, db.CreateDocumentFileParams{
			DocumentID: id, Name: name, Stored: stored, ContentType: contentType, Size: size, Position: pos, TextStatus: "pending",
		})
		return err
	})
	if err != nil {
		s.removeFiles([]string{stored})
		return nil, err
	}
	return s.GetDocument(ctx, id)
}

// RemoveDocumentFile deletes one of a document's files.
func (s *Service) RemoveDocumentFile(ctx context.Context, id, fileID int64) (*Document, error) {
	var stored string
	err := s.inTx(ctx, func(q *db.Queries) error {
		f, err := q.GetDocumentFile(ctx, db.GetDocumentFileParams{ID: fileID, DocumentID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		stored = f.Stored
		if err := q.DeleteDocumentFile(ctx, fileID); err != nil {
			return err
		}
		return index(ctx, q, id)
	})
	if err != nil {
		return nil, err
	}
	s.removeFiles([]string{stored})
	return s.GetDocument(ctx, id)
}

// SetFileText records the owner's own text for a file, which the text service then leaves
// alone; an empty text hands the file back to the service to read again.
func (s *Service) SetFileText(ctx context.Context, id, fileID int64, text string) (*Document, error) {
	text = strings.TrimSpace(text)
	status := "owner"
	if text == "" {
		status = "pending"
	}
	err := s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetDocumentFile(ctx, db.GetDocumentFileParams{ID: fileID, DocumentID: id}); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := q.SetFileText(ctx, db.SetFileTextParams{Text: text, TextStatus: status, TextError: "", ID: fileID}); err != nil {
			return err
		}
		if status == "pending" {
			if _, err := q.ResetFileTextTries(ctx, fileID); err != nil {
				return err
			}
		}
		return index(ctx, q, id)
	})
	if err != nil {
		return nil, err
	}
	return s.GetDocument(ctx, id)
}

// FileForReading opens a document's file for the reader: the owner, or a visitor whose link
// shows the document.
func (s *Service) FileForReading(ctx context.Context, id, fileID int64, sc *Scope) (path, name, contentType string, err error) {
	q := s.read()
	d, err := q.GetDocument(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !sc.ShowsPaper(d.ID, d.CourseID.Int64)) {
		return "", "", "", ErrNotFound
	} else if err != nil {
		return "", "", "", err
	}
	f, err := q.GetDocumentFile(ctx, db.GetDocumentFileParams{ID: fileID, DocumentID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", ErrNotFound
	} else if err != nil {
		return "", "", "", err
	}
	p, err := s.Files.Path(f.Stored)
	if err != nil {
		return "", "", "", err
	}
	return p, f.Name, f.ContentType, nil
}

// TextReader reads the words in a stored file; the text service's is extract.Tools.Text.
type TextReader func(ctx context.Context, path, contentType string) (string, error)

// ReadNextFile reads the oldest file waiting for its text, and reports whether there was one.
// A file that fails is tried again later, three times in all; one that the host cannot read
// at all (no tools installed) waits for the tools.
func (s *Service) ReadNextFile(ctx context.Context, read TextReader, unavailable error) (bool, error) {
	q := db.New(s.DB.Write)
	f, err := q.NextFileForText(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	path, err := s.Files.Path(f.Stored)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		return true, s.finishText(ctx, f, "", fmt.Errorf("the file is missing: %w", err))
	}
	text, err := read(ctx, path, f.ContentType)
	if unavailable != nil && errors.Is(err, unavailable) {
		return false, err
	}
	return true, s.finishText(ctx, f, text, err)
}

func (s *Service) finishText(ctx context.Context, f *db.DocumentFile, text string, readErr error) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		// The owner may have typed the text while it was being read; theirs stands.
		now, err := q.GetDocumentFile(ctx, db.GetDocumentFileParams{ID: f.ID, DocumentID: f.DocumentID})
		if errors.Is(err, sql.ErrNoRows) {
			return nil // removed while it was read
		} else if err != nil {
			return err
		}
		if now.TextStatus != "pending" {
			return nil
		}
		if err := q.CountFileTextTry(ctx, f.ID); err != nil {
			return err
		}
		p := db.SetFileTextParams{Text: strings.TrimSpace(text), TextStatus: "done", ID: f.ID}
		switch {
		case readErr != nil && now.TextAttempts+1 >= 3:
			p.TextStatus, p.Text, p.TextError = "failed", "", readErr.Error()
		case readErr != nil:
			p.TextStatus, p.Text, p.TextError = "pending", "", readErr.Error()
		case p.Text == "":
			p.TextStatus = "empty"
		}
		if err := q.SetFileText(ctx, p); err != nil {
			return err
		}
		return index(ctx, q, f.DocumentID)
	})
}

// courseDocuments lists the documents from a course.
func courseDocuments(ctx context.Context, q *db.Queries, courseID int64) ([]DocumentRow, error) {
	rows, err := q.ListCourseDocuments(ctx, sql.NullInt64{Int64: courseID, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]DocumentRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, DocumentRow{ID: r.ID, Kind: r.Kind, Title: r.Title, Authors: names(r.Authors), Published: r.Published, CourseID: courseID})
	}
	return out, nil
}

// DocumentFill is a document's draft with its blanks filled from a record found for it, and
// which fields those are.
type DocumentFill struct {
	Document DocumentInput `json:"document"`
	Filled   []string      `json:"filled" enum:"kind,title,authors,editors,container,volume,issue,pages,published,publisher,url,doi,abstract,notes,subjects"`
}

// FillFromDOI fills a draft's blanks from what Crossref says its DOI is. The DOI itself is
// written as DOIs are kept, bare and in lower case.
func (s *Service) FillFromDOI(ctx context.Context, doi string, draft DocumentInput) (*DocumentFill, error) {
	d, ok := lookup.NormalizeDOI(doi)
	if !ok {
		return nil, &FieldError{"doi", "a DOI starts with 10. and has a slash, like 10.1038/nature12373"}
	}
	p, err := s.Lookup.ByDOI(ctx, d, false)
	if err != nil {
		return nil, fmt.Errorf("doi: %w", err)
	}
	if p == nil {
		return nil, &FieldError{"doi", "Crossref does not know that DOI. Check it, or fill the details in yourself."}
	}
	draft.DOI = cmp.Or(p.DOI, d)
	filled := fillDocumentBlanks(&draft, documentOf(p))
	return &DocumentFill{Document: draft, Filled: filled}, nil
}

// FindDOIs searches Crossref for a paper whose DOI the owner does not have, by its title and
// its first author's surname, best match first.
func (s *Service) FindDOIs(ctx context.Context, title, author string) ([]lookup.DocProposal, error) {
	title = strings.TrimSpace(title)
	if len([]rune(title)) < 3 {
		return nil, &FieldError{"title", "type the paper's title first: the search finds its DOI by it"}
	}
	found, err := s.Lookup.FindDOIs(ctx, title, lookup.Surname(strings.TrimSpace(author)))
	if err != nil {
		return nil, fmt.Errorf("find doi: %w", err)
	}
	return found, nil
}

// DocumentSuggestions are what looking a paper up again by its DOI did and offers: the paper
// with its blanks filled, which those were, and the details Crossref gives otherwise, as the
// paper would be with each taken, to take or leave one by one.
type DocumentSuggestions struct {
	Document  *Document     `json:"document"`
	Filled    []string      `json:"filled" enum:"kind,title,authors,editors,container,volume,issue,pages,published,publisher,url,doi,abstract,notes,subjects"`
	Fields    []string      `json:"fields" enum:"kind,title,authors,editors,container,volume,issue,pages,published"`
	Suggested DocumentInput `json:"suggested"`
}

// RefreshDocument looks a paper up again by its DOI, each source asked afresh: its blanks are
// filled and saved, and what Crossref gives otherwise is offered, never taken.
func (s *Service) RefreshDocument(ctx context.Context, id int64) (*DocumentSuggestions, error) {
	before, err := s.GetDocument(ctx, id)
	if err != nil {
		return nil, err
	}
	if before.DOI == "" {
		return nil, &ConflictError{"it has no DOI to look it up by: its Edit page searches for one"}
	}
	p, err := s.Lookup.ByDOI(ctx, before.DOI, true)
	if err != nil {
		return nil, fmt.Errorf("doi: %w", err)
	}
	if p == nil {
		return nil, &FieldError{"doi", "Crossref does not know its DOI. Check it on the Edit page."}
	}
	from := documentOf(p)
	// Read again after the sources answered, so a change saved meanwhile is kept.
	d, err := s.GetDocument(ctx, id)
	if err != nil {
		return nil, err
	}
	have := d.DocumentInput
	filled := fillDocumentBlanks(&have, from)
	if len(filled) > 0 {
		if d, err = s.UpdateDocument(ctx, id, have); err != nil {
			return nil, err
		}
	}
	suggested, fields := documentDifferences(have, from)
	return &DocumentSuggestions{Document: d, Filled: filled, Fields: fields, Suggested: suggested}, nil
}

// documentDifferences are the details a record gives otherwise than a paper has: the paper as
// it would be with each taken, and which fields those are. Spacing is no difference, nor a
// date the paper has more exactly, nor an article for a review, which Crossref cannot tell
// apart, nor people with the same surnames in the same order, written more or less fully
// ("William R. Newman", "William Newman"). The abstract and the subjects are filled, not
// offered, and so is the publisher: Crossref's is the company that registered the DOI
// ("Informa UK Limited"), not the imprint a citation names ("Routledge").
func documentDifferences(have, from DocumentInput) (DocumentInput, []string) {
	suggested := have
	suggested.Authors, suggested.Editors = slices.Clone(have.Authors), slices.Clone(have.Editors)
	fields := []string{}
	same := func(a, b string) bool {
		return strings.Join(strings.Fields(a), " ") == strings.Join(strings.Fields(b), " ")
	}
	str := func(field string, dst *string, now, then string) {
		if now != "" && then != "" && !same(now, then) {
			*dst = then
			fields = append(fields, field)
		}
	}
	surnames := func(names []string) string {
		var out []string
		for _, n := range names {
			out = append(out, strings.Join(keyWords(lookup.Surname(n)), " "))
		}
		return strings.Join(out, "; ")
	}
	list := func(field string, dst *[]string, now, then []string) {
		if len(now) > 0 && len(then) > 0 && surnames(now) != surnames(then) {
			*dst = slices.Clone(then)
			fields = append(fields, field)
		}
	}
	if from.Kind != "" && from.Kind != "other" && have.Kind != "other" && have.Kind != from.Kind &&
		(have.Kind != "review" || from.Kind != "article") {
		suggested.Kind = from.Kind
		fields = append(fields, "kind")
	}
	str("title", &suggested.Title, have.Title, from.Title)
	list("authors", &suggested.Authors, have.Authors, from.Authors)
	list("editors", &suggested.Editors, have.Editors, from.Editors)
	str("container", &suggested.Container, have.Container, from.Container)
	str("volume", &suggested.Volume, have.Volume, from.Volume)
	str("issue", &suggested.Issue, have.Issue, from.Issue)
	str("pages", &suggested.Pages, have.Pages, from.Pages)
	if !strings.HasPrefix(have.Published, from.Published) {
		str("published", &suggested.Published, have.Published, from.Published)
	}
	return suggested, fields
}

// documentOf is a record found for a document, as the document it describes.
func documentOf(p *lookup.DocProposal) DocumentInput {
	return DocumentInput{
		Kind: p.Kind, Title: p.Title, Authors: p.Authors, Editors: p.Editors, Container: p.Container,
		Volume: p.Volume, Issue: p.Issue, Pages: p.Pages, Published: p.Published, Publisher: p.Publisher,
		DOI: p.DOI, URL: p.URL, Abstract: p.Abstract, Subjects: p.Subjects,
	}
}

// fillDocumentBlanks fills a document's blanks from a record that names it, the one way a
// paper takes what a DOI, an exported citation, or an import gives: nothing it has is
// replaced, a kind of "other" gives way to the record's, and its subjects gain the record's
// (up to what a document takes). It returns the fields filled, by their JSON names.
func fillDocumentBlanks(have *DocumentInput, from DocumentInput) []string {
	filled := []string{}
	// The kind first, so the form names the field after it for what it now is (journal, book).
	if have.Kind == "other" && from.Kind != "" && from.Kind != "other" {
		have.Kind = from.Kind
		filled = append(filled, "kind")
	}
	str := func(field string, dst *string, v string) {
		if *dst == "" && v != "" {
			*dst = v
			filled = append(filled, field)
		}
	}
	list := func(field string, dst *[]string, v []string) {
		if len(*dst) == 0 && len(v) > 0 {
			*dst = slices.Clone(v)
			filled = append(filled, field)
		}
	}
	str("title", &have.Title, from.Title)
	list("authors", &have.Authors, from.Authors)
	list("editors", &have.Editors, from.Editors)
	str("container", &have.Container, from.Container)
	str("volume", &have.Volume, from.Volume)
	str("issue", &have.Issue, from.Issue)
	str("pages", &have.Pages, from.Pages)
	str("published", &have.Published, from.Published)
	str("publisher", &have.Publisher, from.Publisher)
	str("url", &have.URL, from.URL)
	str("doi", &have.DOI, from.DOI)
	str("abstract", &have.Abstract, from.Abstract)
	str("notes", &have.Notes, from.Notes)
	if joined := cleanList(slices.Concat(have.Subjects, from.Subjects)); len(joined) > len(cleanList(have.Subjects)) {
		have.Subjects = joined[:min(len(joined), documentSubjectLimit)]
		filled = append(filled, "subjects")
	}
	return filled
}

// names reads a list this package wrote as JSON: a document's authors, or its subjects.
func names(raw string) []string {
	out := []string{}
	_ = json.Unmarshal([]byte(raw), &out) // written by this package as a JSON list
	return out
}

// tidyName closes the gaps a course with no department or number leaves in its name.
func tidyName(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
