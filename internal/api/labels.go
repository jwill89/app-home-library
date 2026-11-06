package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
)

// fileOutput is a file the API answers with: a PDF the browser opens in its own viewer, to be
// printed from there, or an export it saves.
type fileOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

func pdf(name string, b []byte) *fileOutput {
	return &fileOutput{ContentType: "application/pdf", ContentDisposition: `inline; filename="` + name + `.pdf"`, Body: b}
}

// download is a file the browser saves, under name.
func download(name, contentType string, b []byte) *fileOutput {
	return &fileOutput{ContentType: contentType, ContentDisposition: `attachment; filename="` + name + `"`, Body: b}
}

type labelKind struct {
	Kind string `path:"kind" enum:"barcode,spine"`
}

func (h *handlers) registerLabels() {
	huma.Register(h.api, h.op("list-labels", http.MethodGet, "/labels", "The labels waiting to be printed, each kind in shelf order"),
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Labels []catalog.QueuedLabel `json:"labels"`
			}
		}, error) {
			list, err := h.Catalog.LabelQueue(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Labels []catalog.QueuedLabel `json:"labels"`
				}
			}{}
			out.Body.Labels = list
			return out, nil
		})

	huma.Register(h.api, h.op("queue-label", http.MethodPut, "/labels/{kind}/{copyId}", "Put a copy's label in the queue to print again"),
		func(ctx context.Context, in *struct {
			Kind   string `path:"kind" enum:"barcode,spine"`
			CopyID int64  `path:"copyId"`
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.QueueLabel(ctx, in.Kind, in.CopyID))
		})

	huma.Register(h.api, h.op("unqueue-labels", http.MethodDelete, "/labels/{kind}", "Take labels off the queue: printed, or not wanted"),
		func(ctx context.Context, in *struct {
			Kind   string  `path:"kind" enum:"barcode,spine"`
			Copies []int64 `query:"copies" minItems:"1" maxItems:"2000" doc:"The copies whose labels to take off"`
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.UnqueueLabels(ctx, in.Kind, in.Copies))
		})

	huma.Register(h.api, h.op("print-labels", http.MethodGet, "/labels/{kind}/sheets", "The queued labels of these copies, drawn on their sheets as a PDF"),
		func(ctx context.Context, in *struct {
			Kind   string  `path:"kind" enum:"barcode,spine"`
			Copies []int64 `query:"copies" minItems:"1" maxItems:"2000"`
		}) (*fileOutput, error) {
			b, err := h.Catalog.LabelPDF(ctx, in.Kind, in.Copies)
			if err != nil {
				return nil, h.fail(err)
			}
			return pdf(in.Kind+"-labels", b), nil
		})

	huma.Register(h.api, h.op("list-label-sheets", http.MethodGet, "/label-sheets", "The two label sheets: layout, labels waiting, and this printer's offsets"),
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Sheets []catalog.LabelSheet `json:"sheets"`
			}
		}, error) {
			list, err := h.Catalog.LabelSheets(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Sheets []catalog.LabelSheet `json:"sheets"`
				}
			}{}
			out.Body.Sheets = list
			return out, nil
		})

	huma.Register(h.api, h.op("set-label-sheet", http.MethodPut, "/label-sheets/{kind}", "Turn a kind of label on or off, and move its printing to meet this printer's labels"),
		func(ctx context.Context, in *struct {
			Kind string `path:"kind" enum:"barcode,spine"`
			Body catalog.LabelSheetInput
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.SetLabelSheet(ctx, in.Kind, in.Body))
		})

	huma.Register(h.api, h.op("alignment-sheet", http.MethodGet, "/label-sheets/{kind}/alignment", "Every label's outline on a sheet, to print on plain paper and hold over the labels"),
		func(ctx context.Context, in *labelKind) (*fileOutput, error) {
			b, err := h.Catalog.AlignmentPDF(ctx, in.Kind)
			if err != nil {
				return nil, h.fail(err)
			}
			return pdf(in.Kind+"-alignment", b), nil
		})

	huma.Register(h.api, h.op("list-sections", http.MethodGet, "/sections", "Each section and how it is shelved"),
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Sections []catalog.ShelvingRule `json:"sections"`
			}
		}, error) {
			list, err := h.Catalog.ShelvingRules(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Sections []catalog.ShelvingRule `json:"sections"`
				}
			}{}
			out.Body.Sections = list
			return out, nil
		})

	huma.Register(h.api, h.op("set-shelving", http.MethodPut, "/sections/{id}", "Change how a section is shelved, which its call numbers follow"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.ShelvingInput
		}) (*struct{ Body *catalog.ShelvingRule }, error) {
			r, err := h.Catalog.SetShelvingRule(ctx, in.ID, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.ShelvingRule }{Body: r}, nil
		})
}
