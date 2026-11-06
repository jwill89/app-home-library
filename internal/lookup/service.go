package lookup

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Cache keeps each source's raw answer. A nil body is a source that had nothing.
type Cache interface {
	Get(ctx context.Context, key, source string) (body []byte, fetched time.Time, ok bool)
	Put(ctx context.Context, key, source string, body []byte)
}

// Candidate is one result of a title search.
type Candidate struct {
	Source    string   `json:"source"`
	Title     string   `json:"title"`
	Subtitle  string   `json:"subtitle"`
	Authors   []string `json:"authors"`
	Publisher string   `json:"publisher"`
	Published string   `json:"published"`
	ISBN13    string   `json:"isbn13"`
	Thumbnail string   `json:"thumbnail" doc:"A small picture to show beside the result"`
	// The covers the server can fetch and keep, at full size. Google's image host refuses the
	// droplet, so its thumbnails are for looking at only.
	Covers []CoverChoice `json:"covers"`
}

// Result is a merged lookup and what each source made of it.
type Result struct {
	Proposal Proposal          `json:"proposal"`
	Answered []string          `json:"answered" doc:"Sources that knew the book"`
	Failed   map[string]string `json:"failed" doc:"Sources that could not be asked, with why"`
	asked    int
}

// Found reports whether any source knew the book.
func (r *Result) Found() bool {
	return len(r.Answered) > 0
}

// Unreached reports whether no source could be asked at all, so the book's being unknown
// says nothing yet.
func (r *Result) Unreached() bool {
	return len(r.Answered) == 0 && r.asked > 0 && len(r.Failed) == r.asked
}

// Options configure a Service.
type Options struct {
	UserAgent      string
	GoogleKey      string
	HardcoverToken string
	Cache          Cache
	Logger         *slog.Logger
	HTTPClient     *http.Client
	MaxAge         time.Duration // how long a cached answer is used without asking again
}

// Service asks every source and merges their answers.
type Service struct {
	LoC         *LoC
	OpenLibrary *OpenLibrary
	Google      *Google
	Crossref    *Crossref
	OpenAlex    *OpenAlex
	Headings    *Headings
	Hardcover   *Hardcover
	HathiTrust  *HathiTrust
	cache       Cache
	logger      *slog.Logger
	maxAge      time.Duration
	warned      sync.Map
}

// New builds a Service against the real sources.
func New(o Options) *Service {
	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.MaxAge == 0 {
		o.MaxAge = 90 * 24 * time.Hour
	}
	return &Service{
		// Open Library asks for no more than a request a second from one client; the
		// Library of Congress publishes no rate but drops connections under load.
		LoC:         &LoC{BaseURL: LoCURL, fetch: newFetcher(client, o.UserAgent, time.Second)},
		OpenLibrary: &OpenLibrary{BaseURL: OpenLibraryURL, CoversURL: "https://covers.openlibrary.org", fetch: newFetcher(client, o.UserAgent, time.Second)},
		Google:      &Google{BaseURL: GoogleURL, Key: o.GoogleKey, fetch: newFetcher(client, o.UserAgent, 200*time.Millisecond)},
		Crossref:    &Crossref{BaseURL: CrossrefURL, fetch: newFetcher(client, o.UserAgent, time.Second)},
		OpenAlex:    &OpenAlex{BaseURL: OpenAlexURL, fetch: newFetcher(client, o.UserAgent, 200*time.Millisecond)},
		Headings:    &Headings{BaseURL: HeadingsURL, fetch: newFetcher(client, o.UserAgent, 300*time.Millisecond)},
		HathiTrust:  &HathiTrust{BaseURL: HathiTrustURL, fetch: newFetcher(client, o.UserAgent, time.Second)},
		Hardcover:   &Hardcover{URL: HardcoverURL, Token: strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(o.HardcoverToken), "Bearer ")), fetch: newFetcher(client, o.UserAgent, time.Second)},
		cache:       o.Cache,
		logger:      o.Logger,
		maxAge:      o.MaxAge,
	}
}

// cached returns the source's answer for the key, from the cache unless fresh is asked
// for or the cached one is old.
func (s *Service) cached(ctx context.Context, key, source string, fresh bool, fetch func(context.Context) ([]byte, error)) ([]byte, error) {
	if !fresh && s.cache != nil {
		if body, at, ok := s.cache.Get(ctx, key, source); ok && time.Since(at) < s.maxAge {
			if len(body) == 0 {
				return nil, nil // the source had nothing, last time
			}
			return body, nil
		}
	}
	body, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		s.cache.Put(ctx, key, source, body)
	}
	return body, nil
}

// ByISBN asks every source about an ISBN-13 at once and merges what they say: the Library
// of Congress, HathiTrust, Open Library, Google Books, and Hardcover when its token is set.
// fresh skips the cache.
func (s *Service) ByISBN(ctx context.Context, isbn13 string, fresh bool) *Result {
	type answer struct {
		source string
		rec    *Record
		err    error
	}
	jobs := map[string]func(context.Context) (*Record, error){
		SourceLoC: func(ctx context.Context) (*Record, error) {
			body, err := s.cached(ctx, isbn13, SourceLoC, fresh, func(ctx context.Context) ([]byte, error) { return s.LoC.FetchISBN(ctx, isbn13) })
			if err != nil || body == nil {
				return nil, err
			}
			recs, err := ParseLoC(body, isbn13)
			if err != nil || len(recs) == 0 {
				return nil, err
			}
			return recs[0], nil
		},
		SourceOpenLibrary: func(ctx context.Context) (*Record, error) {
			body, err := s.cached(ctx, isbn13, SourceOpenLibrary, fresh, func(ctx context.Context) ([]byte, error) { return s.OpenLibrary.FetchISBN(ctx, isbn13) })
			if err != nil || body == nil {
				return nil, err
			}
			return ParseOpenLibrary(body, s.OpenLibrary.CoversURL)
		},
		SourceHathiTrust: func(ctx context.Context) (*Record, error) {
			body, err := s.cached(ctx, isbn13, SourceHathiTrust, fresh, func(ctx context.Context) ([]byte, error) { return s.HathiTrust.FetchISBN(ctx, isbn13) })
			if err != nil || body == nil {
				return nil, err
			}
			return ParseHathiTrust(body, isbn13)
		},
		SourceGoogle: func(ctx context.Context) (*Record, error) {
			body, err := s.cached(ctx, isbn13, SourceGoogle, fresh, func(ctx context.Context) ([]byte, error) { return s.Google.FetchISBN(ctx, isbn13) })
			if err != nil || body == nil {
				return nil, err
			}
			return ParseGoogle(body, isbn13)
		},
	}
	if s.Hardcover.Token != "" {
		jobs[SourceHardcover] = func(ctx context.Context) (*Record, error) {
			body, err := s.cached(ctx, isbn13, SourceHardcover, fresh, func(ctx context.Context) ([]byte, error) { return s.Hardcover.FetchISBN(ctx, isbn13) })
			if err != nil || body == nil {
				return nil, err
			}
			return ParseHardcover(body)
		}
	}
	answers := make(chan answer, len(jobs))
	for src, job := range jobs {
		go func() {
			rec, err := job(ctx)
			answers <- answer{src, rec, err}
		}()
	}
	recs := map[string]*Record{}
	res := &Result{Failed: map[string]string{}, asked: len(jobs)}
	for range jobs {
		a := <-answers
		switch {
		case a.err != nil:
			res.Failed[a.source] = s.reason(a.source, a.err)
		case a.rec != nil:
			recs[a.source] = a.rec
			res.Answered = append(res.Answered, a.source)
		}
	}
	res.Proposal = Merge(isbn13, recs)
	if p := &res.Proposal; p.Sources["subjects"] == SourceOpenLibrary {
		if p.Subjects = s.englishSubjects(ctx, p.Subjects); len(p.Subjects) == 0 {
			delete(p.Sources, "subjects")
		}
	}
	return res
}

// reason is what the owner is told about a source that failed. A refused key is logged
// once, not on every book of an import.
func (s *Service) reason(source string, err error) string {
	if errors.Is(err, ErrUnavailable) {
		if source == SourceGoogle && s.Google.Key == "" {
			return "no API key is set"
		}
		if _, seen := s.warned.LoadOrStore(source, true); !seen {
			s.logger.Warn("lookup source refused the request", "source", source, "err", err)
		}
		return "refused the request: " + err.Error()
	}
	s.logger.Warn("lookup failed", "source", source, "err", err)
	return err.Error()
}

// RelatedClass finds another edition of a work in the Library of Congress catalog by title
// and author, for a book whose own ISBN the catalog lacks, and returns the first one with a
// Dewey number. Classification belongs to the work, so any edition's will do.
func (s *Service) RelatedClass(ctx context.Context, title, author string) (*Record, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, nil
	}
	// The catalog's creator index holds headings ("Ryan, Anthony"), so the surname alone
	// matches it.
	surname := ""
	if f := strings.Fields(author); len(f) > 0 {
		surname = strings.Trim(f[len(f)-1], ",.")
	}
	key := strings.ToLower(title + "|" + surname)
	body, err := s.cached(ctx, key, SourceLoCRelated, false, func(ctx context.Context) ([]byte, error) {
		return s.LoC.FetchTitle(ctx, title, surname)
	})
	if err != nil || body == nil {
		return nil, err
	}
	recs, err := ParseLoC(body, "")
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		if r.DDC != "" && strings.EqualFold(normalTitle(r.Title), normalTitle(title)) {
			return r, nil
		}
	}
	return nil, nil
}

func normalTitle(s string) string {
	s = strings.ToLower(s)
	for _, article := range []string{"the ", "a ", "an "} {
		s = strings.TrimPrefix(s, article)
	}
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}), " ")
}

// Search looks for books by title or author words in Google Books and Open Library, Google
// first, one result an ISBN.
func (s *Service) Search(ctx context.Context, q string) ([]Candidate, map[string]string) {
	failed := map[string]string{}
	var g, o []Candidate
	var wg sync.WaitGroup
	wg.Go(func() {
		var err error
		if g, err = s.Google.Search(ctx, q); err != nil {
			failed[SourceGoogle] = s.reason(SourceGoogle, err)
		}
	})
	var olErr error
	wg.Go(func() { o, olErr = s.OpenLibrary.Search(ctx, q) })
	wg.Wait()
	if olErr != nil {
		failed[SourceOpenLibrary] = s.reason(SourceOpenLibrary, olErr)
	}
	seen := map[string]bool{}
	var out []Candidate
	for _, c := range append(g, o...) {
		key := c.ISBN13
		if key == "" {
			key = strings.ToLower(c.Title + "|" + strings.Join(c.Authors, ","))
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out, failed
}
