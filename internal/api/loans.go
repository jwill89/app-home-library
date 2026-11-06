package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
)

// Loans and borrowers are the owner's alone: a visitor sees only that a copy is out.
func (h *handlers) registerLoans() {
	type loanOutput struct {
		Body *catalog.Loan
	}

	huma.Register(h.api, h.op("list-loans", http.MethodGet, "/loans", "Every loan: the copies out, then the history"),
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Loans []catalog.Loan `json:"loans"`
			}
		}, error) {
			loans, err := h.Catalog.Loans(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Loans []catalog.Loan `json:"loans"`
				}
			}{}
			out.Body.Loans = loans
			return out, nil
		})

	lend := h.op("create-loan", http.MethodPost, "/copies/{id}/loans", "Lend a copy")
	lend.DefaultStatus = http.StatusCreated
	huma.Register(h.api, lend, func(ctx context.Context, in *struct {
		ID   int64 `path:"id"`
		Body catalog.LoanInput
	}) (*loanOutput, error) {
		l, err := h.Catalog.Lend(ctx, in.ID, in.Body)
		if err != nil {
			return nil, h.fail(err)
		}
		return &loanOutput{Body: l}, nil
	})

	huma.Register(h.api, h.op("update-loan", http.MethodPut, "/loans/{id}", "Replace a loan's details; a return date brings the copy back"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.LoanInput
		}) (*loanOutput, error) {
			l, err := h.Catalog.UpdateLoan(ctx, in.ID, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			return &loanOutput{Body: l}, nil
		})

	huma.Register(h.api, h.op("delete-loan", http.MethodDelete, "/loans/{id}", "Remove a loan from the history"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			if err := h.Catalog.DeleteLoan(ctx, in.ID); err != nil {
				return nil, h.fail(err)
			}
			return nil, nil
		})

	huma.Register(h.api, h.op("list-borrowers", http.MethodGet, "/borrowers", "Everyone the owner has lent to"),
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Borrowers []catalog.Borrower `json:"borrowers"`
			}
		}, error) {
			list, err := h.Catalog.Borrowers(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Borrowers []catalog.Borrower `json:"borrowers"`
				}
			}{}
			out.Body.Borrowers = list
			return out, nil
		})

	huma.Register(h.api, h.op("update-borrower", http.MethodPut, "/borrowers/{id}", "Rename a borrower or change how to reach them"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.BorrowerInput
		}) (*struct{ Body *catalog.Borrower }, error) {
			b, err := h.Catalog.UpdateBorrower(ctx, in.ID, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.Borrower }{Body: b}, nil
		})

	huma.Register(h.api, h.op("delete-borrower", http.MethodDelete, "/borrowers/{id}", "Remove a borrower and their loans from the history"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			if err := h.Catalog.DeleteBorrower(ctx, in.ID); err != nil {
				return nil, h.fail(err)
			}
			return nil, nil
		})
}
