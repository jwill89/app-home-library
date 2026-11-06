package catalog

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// LoansCalendar is the feed of the loans' due dates, as iCalendar (RFC 5545): each copy still
// out with a day it is due back is an event that day, with a reminder that morning. Each event
// is named for its loan, so a calendar knows it again on a later fetch.
func (s *Service) LoansCalendar(ctx context.Context, now time.Time) ([]byte, error) {
	loans, err := s.Loans(ctx)
	if err != nil {
		return nil, fmt.Errorf("loans calendar: %w", err)
	}
	var b strings.Builder
	line := func(s string) { b.WriteString(foldLine(s)) }
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Home Library//Loans//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:Library loans due")
	line("REFRESH-INTERVAL;VALUE=DURATION:PT6H")
	line("X-PUBLISHED-TTL:PT6H")
	stamp := now.UTC().Format("20060102T150405Z")
	for _, l := range loans {
		if l.ReturnedOn != "" || l.DueOn == "" {
			continue
		}
		due, err := time.Parse(time.DateOnly, l.DueOn)
		if err != nil {
			continue
		}
		summary := fmt.Sprintf("Due back from %s: %s (%s)", l.Borrower, l.Title, l.Barcode)
		line("BEGIN:VEVENT")
		line(fmt.Sprintf("UID:loan-%d@home-library", l.ID))
		line("DTSTAMP:" + stamp)
		line("DTSTART;VALUE=DATE:" + due.Format("20060102"))
		line("DTEND;VALUE=DATE:" + due.AddDate(0, 0, 1).Format("20060102"))
		line("SUMMARY:" + escapeText(summary))
		line("DESCRIPTION:" + escapeText(fmt.Sprintf("Lent to %s on %s.", l.Borrower, l.LentOn)))
		line("TRANSP:TRANSPARENT")
		line("BEGIN:VALARM")
		line("ACTION:DISPLAY")
		line("DESCRIPTION:" + escapeText(summary))
		line("TRIGGER;RELATED=START:PT9H")
		line("END:VALARM")
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	return []byte(b.String()), nil
}

// escapeText writes a value as iCalendar text: backslashes, commas, semicolons, and new lines
// escaped.
func escapeText(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`).Replace(s)
}

// foldLine ends a content line with CRLF, folding it at 75 octets as the standard asks, never
// inside a character.
func foldLine(s string) string {
	var b strings.Builder
	width := 0
	for _, r := range s {
		n := utf8.RuneLen(r)
		if width+n > 75 {
			b.WriteString("\r\n ")
			width = 1
		}
		b.WriteRune(r)
		width += n
	}
	b.WriteString("\r\n")
	return b.String()
}
