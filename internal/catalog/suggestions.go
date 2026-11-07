package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

// What a lookup the owner asked for offers to change is worked out whenever it is read, from
// what the sources proposed then and the book as it is now: a detail the owner has since taken
// or typed is no longer offered. A value the owner declined is not offered again while the
// sources still give it; a source that later gives something else offers that.

// Suggestions are what a lookup the owner asked for offers to change: the book with every
// suggested change made, and which fields those are. The book is as it was saved.
type Suggestions struct {
	Book      *Book     `json:"book"`
	Fields    []string  `json:"fields" enum:"title,subtitle,contributors,series,seriesNumber,lccn,publisher,publishPlace,published,editionStatement,format,pages,heightMm,widthMm,thicknessMm,language,ddc,lcc,description"`
	Suggested BookInput `json:"suggested"`
}

// SuggestedBook is a book the sources offer changes to, and the details they would change.
type SuggestedBook struct {
	BookID  int64    `json:"bookId"`
	Title   string   `json:"title"`
	Authors string   `json:"authors"`
	Cover   Cover    `json:"cover"`
	Fields  []string `json:"fields" enum:"title,subtitle,contributors,series,seriesNumber,lccn,publisher,publishPlace,published,editionStatement,format,pages,heightMm,widthMm,thicknessMm,language,ddc,lcc,description"`
}

// Declined is the details of a book the owner keeps as they are, against what the sources give.
type Declined struct {
	Fields []string `json:"fields" minItems:"1" maxItems:"19" enum:"title,subtitle,contributors,series,seriesNumber,lccn,publisher,publishPlace,published,editionStatement,format,pages,heightMm,widthMm,thicknessMm,language,ddc,lcc,description"`
}

// Refresh is the owner's "look up again": every source is asked afresh, not answered from what
// it said before, the blanks are filled as Enrich fills them, and each detail the sources now
// give otherwise comes back as a suggestion to take or leave.
func (s *Service) Refresh(ctx context.Context, id int64) (*Suggestions, error) {
	res, err := s.enrich(ctx, id, true)
	if err != nil {
		return nil, err
	}
	if res != nil {
		if err := s.keepProposal(ctx, id, &res.Proposal); err != nil {
			return nil, err
		}
	}
	return s.Suggested(ctx, id)
}

// keepProposal keeps what the sources proposed for an edition in place of what they proposed
// before, for what they give otherwise to be offered.
func (s *Service) keepProposal(ctx context.Context, id int64, p *lookup.Proposal) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("proposal: %w", err)
	}
	if err := db.New(s.DB.Write).KeepProposal(ctx, db.KeepProposalParams{EditionID: id, Proposal: string(raw)}); err != nil {
		return fmt.Errorf("keep proposal: %w", err)
	}
	return nil
}

// Suggested is what the sources' last proposal for a book offers to change now; nothing for a
// book never looked up at the owner's asking.
func (s *Service) Suggested(ctx context.Context, id int64) (*Suggestions, error) {
	book, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &Suggestions{Book: book, Fields: []string{}, Suggested: book.Input()}
	p, declined, err := s.proposal(ctx, id)
	if err != nil || p == nil {
		return out, err
	}
	out.Fields, out.Suggested, err = offered(book, p, declined)
	return out, err
}

// SuggestionList is every book the sources offer changes to, by title; with ids, only those
// books, as the change feed names them, and nothing is worked out for the rest.
func (s *Service) SuggestionList(ctx context.Context, ids []int64) ([]SuggestedBook, error) {
	rows, err := s.read().ListProposalBooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggestions: %w", err)
	}
	out := []SuggestedBook{}
	for _, r := range rows {
		if len(ids) > 0 && !slices.Contains(ids, r.ID) {
			continue
		}
		sg, err := s.Suggested(ctx, r.ID)
		if errors.Is(err, ErrNotFound) {
			continue // removed since the list was read
		}
		if err != nil {
			return nil, err
		}
		if len(sg.Fields) == 0 {
			continue
		}
		out = append(out, SuggestedBook{BookID: r.ID, Title: r.Title, Authors: r.Authors, Cover: s.cover(r.Cover), Fields: sg.Fields})
	}
	return out, nil
}

// DeclineSuggestions keeps the book as it is in the fields named: what the sources' last
// proposal gives for them is not offered again while they still give it. A field the proposal
// no longer offers to change is passed over.
func (s *Service) DeclineSuggestions(ctx context.Context, id int64, fields []string) error {
	book, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	p, _, err := s.proposal(ctx, id)
	if err != nil || p == nil {
		return err
	}
	in := book.Input()
	changed := differences(&in, p)
	given, err := fieldValues(&in)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *db.Queries) error {
		for _, f := range fields {
			if !slices.Contains(changed, f) {
				continue
			}
			if err := q.DeclineSuggestion(ctx, db.DeclineSuggestionParams{EditionID: id, Field: f, Value: string(given[f])}); err != nil {
				return fmt.Errorf("decline %s: %w", f, err)
			}
		}
		return nil
	})
}

// proposal is the sources' last proposal for an edition, and the values the owner declined by
// field; nil when the owner never had it looked up.
func (s *Service) proposal(ctx context.Context, id int64) (*lookup.Proposal, map[string]string, error) {
	raw, err := s.read().GetProposal(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("proposal: %w", err)
	}
	var p lookup.Proposal
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, nil, fmt.Errorf("proposal: %w", err)
	}
	rows, err := s.read().ListDeclinedSuggestions(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("declined: %w", err)
	}
	declined := make(map[string]string, len(rows))
	for _, r := range rows {
		declined[r.Field] = r.Value
	}
	return &p, declined, nil
}

// offered is the fields the proposal gives otherwise than the book, less those whose value the
// owner declined, and the book's input with only those changes made.
func offered(book *Book, p *lookup.Proposal, declined map[string]string) ([]string, BookInput, error) {
	in := book.Input()
	changed := differences(&in, p)
	given, err := fieldValues(&in)
	if err != nil {
		return nil, in, err
	}
	fields, back := []string{}, []string{}
	for _, f := range changed {
		if v, ok := declined[f]; ok && v == string(given[f]) {
			back = append(back, f)
		} else {
			fields = append(fields, f)
		}
	}
	if len(back) == 0 {
		return fields, in, nil
	}
	// A declined change is put back as the book has it, with its source.
	was := book.Input()
	had, err := fieldValues(&was)
	if err != nil {
		return nil, in, err
	}
	sources := in.Sources
	for _, f := range back {
		given[f] = had[f]
		if from, ok := was.Sources[f]; ok {
			sources[f] = from
		} else {
			delete(sources, f)
		}
	}
	raw, err := json.Marshal(given)
	if err != nil {
		return nil, in, err
	}
	var out BookInput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, in, err
	}
	out.Sources = sources
	return fields, out, nil
}

// fieldValues is each field of the input as JSON, by its name, which is how a declined value
// is kept and compared.
func fieldValues(in *BookInput) (map[string]json.RawMessage, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
