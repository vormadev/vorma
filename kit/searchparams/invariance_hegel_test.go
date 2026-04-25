package searchparams

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_invariance_form struct {
	Query   string          `json:"q"`
	Tags    []string        `json:"tags"`
	Flags   map[string]bool `json:"flags"`
	Address struct {
		City string `json:"city"`
	} `json:"address"`
}

type property_invariance_case struct {
	query_mode   int
	tags_mode    int
	flags_mode   int
	address_mode int
	noise_mode   int
}

type property_invariance_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestParseIgnoresUnrelatedKeys(t *testing.T) {
	t.Run("generated_noise_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_invariance_case_space{}).draw_case(ht)
		tc.assert_ignores_unrelated_keys(ht)
	}, hegel.WithTestCases(750)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_invariance_case_space) draw_case(
	ht *hegel.T,
) property_invariance_case {
	return property_invariance_case{
		query_mode:   hegel.Draw(ht, hegel.Integers(0, 1)),
		tags_mode:    hegel.Draw(ht, hegel.Integers(0, 2)),
		flags_mode:   hegel.Draw(ht, hegel.Integers(0, 1)),
		address_mode: hegel.Draw(ht, hegel.Integers(0, 1)),
		noise_mode:   hegel.Draw(ht, hegel.Integers(0, 3)),
	}
}

func (tc property_invariance_case) assert_ignores_unrelated_keys(ht *hegel.T) {
	tc.note(ht)

	base, err := ParseToStruct[property_invariance_form](tc.request(false))
	if err != nil {
		ht.Fatalf("unexpected base parse error: %v", err)
	}
	with_noise, err := ParseToStruct[property_invariance_form](tc.request(true))
	if err != nil {
		ht.Fatalf("unexpected noisy parse error: %v", err)
	}

	if !reflect.DeepEqual(base, with_noise) {
		ht.Fatalf("base = %#v, with_noise = %#v", base, with_noise)
	}
}

func (tc property_invariance_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))
}

func (tc property_invariance_case) request(with_noise bool) *http.Request {
	query := url.Values{}

	if tc.query_mode == 1 {
		query.Add("q", "hello")
	}
	switch tc.tags_mode {
	case 1:
		query.Add("tags", "a")
		query.Add("tags", "b")
	case 2:
		query.Add("tags.alpha", "a")
		query.Add("tags.beta", "b")
	}
	if tc.flags_mode == 1 {
		query.Add("flags.x", "true")
		query.Add("flags.y", "false")
	}
	if tc.address_mode == 1 {
		query.Add("address.city", "NYC")
	}

	if with_noise {
		switch tc.noise_mode {
		case 1:
			query.Add("qq", "ignored")
			query.Add("tag", "ignored")
			query.Add("flag", "ignored")
		case 2:
			query.Add("tags_extra.alpha", "ignored")
			query.Add("addressx.city", "ignored")
			query.Add("flagsExtra.x", "true")
		case 3:
			query.Add("q.child", "ignored")
			query.Add("tagsx.beta", "ignored")
			query.Add("address.city.extra", "ignored")
		}
	}

	req := &http.Request{URL: &url.URL{}}
	req.URL.RawQuery = query.Encode()
	return req
}
