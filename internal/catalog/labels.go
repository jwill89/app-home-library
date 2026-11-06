package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"homelibrary/internal/db"
	"homelibrary/internal/labels"
)

// LabelKinds are the labels each copy has: its barcode and its call number on the spine.
var LabelKinds = []string{labels.Barcode.Kind, labels.Spine.Kind}

// QueuedLabel is a label waiting to be printed.
type QueuedLabel struct {
	Kind       string   `json:"kind" enum:"barcode,spine"`
	CopyID     int64    `json:"copyId"`
	Barcode    string   `json:"barcode"`
	BookID     int64    `json:"bookId"`
	Title      string   `json:"title"`
	Section    string   `json:"section"`
	CallNumber []string `json:"callNumber"`
	Missing    []string `json:"missing" doc:"What the call number lacks: dewey (a Dewey section's book with no number), volume (a series book with no volume number)"`
	QueuedAt   string   `json:"queuedAt"`
}

// LabelSheet is a kind of label sheet: its layout, how many labels wait for it, and where this
// printer's labels land on it.
type LabelSheet struct {
	labels.Sheet
	Waiting int64 `json:"waiting"`
	Enabled bool  `json:"enabled" doc:"Printed at all; spine labels are off until the owner turns them on"`
	XMM     int64 `json:"xMm" doc:"Millimetres to move the printing right; negative is left"`
	YMM     int64 `json:"yMm" doc:"Millimetres to move the printing down; negative is up"`
}

// LabelSheetInput is the owner's settings for a kind of label: whether it is printed, and
// how far to move the printing to meet this printer's sheets.
type LabelSheetInput struct {
	XMM     int64 `json:"xMm" minimum:"-10" maximum:"10"`
	YMM     int64 `json:"yMm" minimum:"-10" maximum:"10"`
	Enabled bool  `json:"enabled"`
}

// ShelvingRule is how one section's copies are shelved, which their call numbers follow.
type ShelvingRule struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Shelving string `json:"shelving" enum:"dewey,author,series"`
	Prefix   string `json:"prefix"`
	Copies   int64  `json:"copies"`
}

// ShelvingInput is a section's rule as the owner sets it.
type ShelvingInput struct {
	Shelving string `json:"shelving" enum:"dewey,author,series"`
	Prefix   string `json:"prefix" maxLength:"10" doc:"The class line for author and series shelving (FIC, MANGA); for Dewey, what stands in for a missing number"`
}

var prefixRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9 .-]*$`)

// LabelQueue lists the labels waiting, each kind in shelf order, which is the order they print
// in: a sheet of spine labels goes onto the shelves in one walk along them.
func (s *Service) LabelQueue(ctx context.Context) ([]QueuedLabel, error) {
	rows, err := s.read().ListLabelQueue(ctx)
	if err != nil {
		return nil, err
	}
	shelving := func(r *db.ListLabelQueueRow) *db.CopyShelving {
		return &db.CopyShelving{CopyID: r.CopyID, EditionID: r.EditionID, Section: r.Section, Shelving: r.Shelving,
			LabelPrefix: r.LabelPrefix, Ddc: r.Ddc, Title: r.Title, Series: r.Series, SeriesNumber: r.SeriesNumber, AuthorSort: r.AuthorSort}
	}
	// In shelf order, then each kind together, still in shelf order.
	sortShelved(rows, func(r **db.ListLabelQueueRow) *db.CopyShelving { return shelving(*r) })
	slices.SortStableFunc(rows, func(a, b *db.ListLabelQueueRow) int { return strings.Compare(a.Kind, b.Kind) })
	out := make([]QueuedLabel, 0, len(rows))
	for _, r := range rows {
		out = append(out, QueuedLabel{
			Kind: r.Kind, CopyID: r.CopyID, Barcode: Barcode(r.CopyID), BookID: r.EditionID, Title: r.Title,
			Section: r.Section, CallNumber: CallNumber(shelving(r)), Missing: missing(shelving(r)), QueuedAt: r.QueuedAt,
		})
	}
	return out, nil
}

// LabelSheets lists the two sheets with what waits for each and this printer's offsets.
func (s *Service) LabelSheets(ctx context.Context) ([]LabelSheet, error) {
	q := s.read()
	offsets, err := q.ListLabelOffsets(ctx)
	if err != nil {
		return nil, err
	}
	// Counted, not listed: the list is sorted into shelf order, which only printing needs.
	waiting, err := q.CountLabelQueue(ctx)
	if err != nil {
		return nil, err
	}
	out := []LabelSheet{}
	for _, kind := range LabelKinds {
		sheet, _ := labels.SheetFor(kind)
		ls := LabelSheet{Sheet: sheet}
		for _, o := range offsets {
			if o.Kind == kind {
				ls.XMM, ls.YMM, ls.Enabled = o.XMm, o.YMm, o.Enabled == 1
			}
		}
		for _, w := range waiting {
			if w.Kind == kind {
				ls.Waiting = w.N
			}
		}
		out = append(out, ls)
	}
	return out, nil
}

// SetLabelSheet turns a kind of label on or off and records how far to move its printing.
func (s *Service) SetLabelSheet(ctx context.Context, kind string, in LabelSheetInput) error {
	enabled := int64(0)
	if in.Enabled {
		enabled = 1
	}
	n, err := db.New(s.DB.Write).SetLabelSheet(ctx, db.SetLabelSheetParams{XMm: in.XMM, YMm: in.YMM, Enabled: enabled, Kind: kind})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// QueueLabel puts a copy's label back in the queue, as for one torn or worn.
func (s *Service) QueueLabel(ctx context.Context, kind string, copyID int64) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetCopy(ctx, copyID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		return q.QueueLabel(ctx, db.QueueLabelParams{CopyID: copyID, Kind: kind})
	})
}

// UnqueueLabels takes labels off the queue: printed, or not wanted.
func (s *Service) UnqueueLabels(ctx context.Context, kind string, copyIDs []int64) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		for _, id := range copyIDs {
			if _, err := q.UnqueueLabel(ctx, db.UnqueueLabelParams{CopyID: id, Kind: kind}); err != nil {
				return err
			}
		}
		return nil
	})
}

// LabelPDF draws the queued labels of a kind for the copies asked for, in the queue's order;
// a copy whose label is not waiting is left out.
func (s *Service) LabelPDF(ctx context.Context, kind string, copyIDs []int64) ([]byte, error) {
	sheet, ok := labels.SheetFor(kind)
	if !ok {
		return nil, ErrNotFound
	}
	queue, err := s.LabelQueue(ctx)
	if err != nil {
		return nil, err
	}
	var items []labels.Label
	for _, l := range queue {
		if l.Kind == kind && slices.Contains(copyIDs, l.CopyID) {
			items = append(items, labels.Label{Barcode: l.Barcode, Title: l.Title, CallNumber: l.CallNumber})
		}
	}
	if len(items) == 0 {
		return nil, &FieldError{"copies", "none of those labels is waiting to be printed"}
	}
	off, err := s.offset(ctx, kind)
	if err != nil {
		return nil, err
	}
	return labels.Render(sheet, items, off)
}

// AlignmentPDF draws a kind of sheet's label outlines, moved by the offset recorded for it.
func (s *Service) AlignmentPDF(ctx context.Context, kind string) ([]byte, error) {
	sheet, ok := labels.SheetFor(kind)
	if !ok {
		return nil, ErrNotFound
	}
	off, err := s.offset(ctx, kind)
	if err != nil {
		return nil, err
	}
	return labels.Alignment(sheet, off), nil
}

func (s *Service) offset(ctx context.Context, kind string) (labels.Offset, error) {
	offsets, err := s.read().ListLabelOffsets(ctx)
	if err != nil {
		return labels.Offset{}, err
	}
	for _, o := range offsets {
		if o.Kind == kind {
			return labels.Offset{X: int(o.XMm), Y: int(o.YMm)}, nil
		}
	}
	return labels.Offset{}, nil
}

// ShelvingRules lists each section's rule.
func (s *Service) ShelvingRules(ctx context.Context) ([]ShelvingRule, error) {
	rows, err := s.read().ListShelvingRules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ShelvingRule, 0, len(rows))
	for _, r := range rows {
		out = append(out, ShelvingRule{ID: r.ID, Name: r.Name, Shelving: r.Shelving, Prefix: r.LabelPrefix, Copies: r.Copies})
	}
	return out, nil
}

// SetShelvingRule changes how a section is shelved. Its copies' call numbers follow at once;
// labels already on the books keep what they say until printed again.
func (s *Service) SetShelvingRule(ctx context.Context, id int64, in ShelvingInput) (*ShelvingRule, error) {
	in.Prefix = strings.ToUpper(strings.TrimSpace(in.Prefix))
	if in.Prefix != "" && !prefixRe.MatchString(in.Prefix) {
		return nil, &FieldError{"prefix", "use letters, digits, spaces, dots, and dashes, like FIC or YA FIC"}
	}
	if in.Shelving != ShelveByDewey && in.Prefix == "" {
		return nil, &FieldError{"prefix", "shelving by author or series needs a prefix to head the call number, like FIC"}
	}
	q := db.New(s.DB.Write)
	n, err := q.SetShelvingRule(ctx, db.SetShelvingRuleParams{Shelving: in.Shelving, LabelPrefix: in.Prefix, ID: id})
	if err != nil {
		return nil, fmt.Errorf("shelving rule: %w", err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	r, err := q.GetShelvingRule(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ShelvingRule{ID: r.ID, Name: r.Name, Shelving: r.Shelving, Prefix: r.LabelPrefix, Copies: r.Copies}, nil
}

// queueLabels puts both of a new copy's labels in the queue.
func queueLabels(ctx context.Context, q *db.Queries, copyID int64) error {
	for _, kind := range LabelKinds {
		if err := q.QueueLabel(ctx, db.QueueLabelParams{CopyID: copyID, Kind: kind}); err != nil {
			return fmt.Errorf("queue %s label: %w", kind, err)
		}
	}
	return nil
}

// withLabels puts each copy's call number on it, and the labels it has waiting.
func withLabels(ctx context.Context, q *db.Queries, editionID int64, copies []Copy) error {
	shelving, err := q.ListEditionShelving(ctx, editionID)
	if err != nil {
		return err
	}
	waiting, err := q.ListEditionLabels(ctx, editionID)
	if err != nil {
		return err
	}
	for i := range copies {
		for _, sh := range shelving {
			if sh.CopyID == copies[i].ID {
				copies[i].CallNumber = CallNumber(sh)
			}
		}
		for _, w := range waiting {
			if w.CopyID == copies[i].ID {
				copies[i].Labels = append(copies[i].Labels, w.Kind)
			}
		}
	}
	return nil
}

// missing says what a copy's call number lacks, which the owner can fill on the book's page
// before its spine label prints.
func missing(s *db.CopyShelving) []string {
	out := []string{}
	if s.Shelving == ShelveByDewey && s.Ddc == "" {
		out = append(out, "dewey")
	}
	if s.Series != "" && strings.TrimSpace(s.SeriesNumber) == "" {
		out = append(out, "volume")
	}
	return out
}
