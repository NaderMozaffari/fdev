package logview

import (
	"strings"
	"testing"
)

func TestFilterWords(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte(sample + `I/flutter (1): ⟪fd⟫{"l":"info","t":"Billing","m":"پرداخت موفق بود"}` + "\n"))
	shown := func(filter string) string {
		m.setFilter(filter)
		var out []string
		for _, e := range m.shownEntries() {
			if e.Kind != KindTool {
				out = append(out, e.Text)
			}
		}
		return strings.Join(out, "|")
	}
	cases := map[string]string{
		"tag:myket":             "pending",
		"-tag:myket level:warn": "",
		"status:5":              "DELETE /v1/me",
		"url:/v1/orders":        "POST /v1/orders/new",
		"method:get":            "GET /v1",
		`"موفق بود"`:            "پرداخت موفق بود",
		"پرداخت -tag:billing":   "",
		"level:info":            "connected|پرداخت موفق بود",
	}
	for filter, want := range cases {
		if got := shown(filter); got != want {
			t.Errorf("%s: %q, want %q", filter, got, want)
		}
	}
	// Persian letters of one sound match each other, and so do digits.
	if fold("كيك ۱۲") != fold("کیک 12") || fold("می‌خواهم") != "میخواهم" {
		t.Errorf("fold: %q %q", fold("كيك ۱۲"), fold("می‌خواهم"))
	}
}
