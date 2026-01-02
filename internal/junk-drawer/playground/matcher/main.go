package main

import (
	"fmt"

	"github.com/vormadev/vorma/kit/matcher"
)

var m = matcher.New(&matcher.Options{
	ExplicitIndexSegment:   "_index",
	DynamicParamPrefixRune: '$',
})

var rps = registerPatterns([]string{"/", "/$user", "/$user/*", "/posts"})

func main() {
	fmt.Println("REGISTERED PATTERNS")
	for _, rp := range rps {
		fmt.Printf("Clean: '%s', Original: '%s'\n", rp.NormalizedPattern(), rp.OriginalPattern())
		for _, seg := range rp.NormalizedSegments() {
			fmt.Println(seg)
		}
	}

	fmt.Println()

	var matches []*matcher.BestMatch
	var ok bool
	// matches, ok = m.FindNestedMatches("/")
	match, ok := m.FindBestMatch("/bob")
	if ok {
		matches = append(matches, match)
	}

	if !ok {
		fmt.Println("No match found")
		return
	} else {
		fmt.Println("MATCHES:", len(matches))
	}

	for i, match := range matches {
		fmt.Println()
		fmt.Println("Match", i+1)
		fmt.Println()
		if len(match.Params) > 0 {
			fmt.Println("Params:", match.Params)
		}

		fmt.Println("SplatValues:", match.SplatValues)
		fmt.Printf("Clean: '%s', Original: '%s'\n", match.NormalizedPattern(), match.RegisteredPattern.OriginalPattern())
		fmt.Println()
	}
}

/////////////////////////////////////////////////////////////////////
/////// UTILS
/////////////////////////////////////////////////////////////////////

func registerPatterns(ps []string) []*matcher.RegisteredPattern {
	var rps []*matcher.RegisteredPattern
	for _, p := range ps {
		rps = append(rps, m.RegisterPattern(p))
	}
	return rps
}
