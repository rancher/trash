package conf

import (
	"testing"
)

func TestDuplicates(t *testing.T) {

	testData := []struct {
		imports    []Import
		duplicates int
	}{
		{[]Import{
			{"package1", "version1", ""},
		}, 0},
		{[]Import{
			{"package1", "version1", ""},
			{"package2", "version1", "repoA"},
		}, 0},
		{[]Import{
			{"package1", "version1", ""},
			{"package2", "version1", "repoA"},
			{"package1", "version1", ""},
		}, 1},
		{[]Import{
			{"package1", "version1", ""},
			{"package2", "version1", "repoA"},
			{"package1", "version1", ""},
			{"package1", "version1", ""},
		}, 2},
		{[]Import{
			{"package1", "version1", ""},
			{"package2", "version1", "repoA"},
			{"package1", "version1", ""},
			{"package1", "version1", ""},
			{"package2", "version2", "repoB"},
			{"package3", "version1", "repoA"},
		}, 3},
	}

	for i, d := range testData {
		trash := Conf{"", d.imports, nil, nil, "", false}
		trash.Dedupe()

		if d.duplicates != len(d.imports)-len(trash.Imports) {
			t.Errorf("Case %d failed: expected %d duplicates but removed %d", i, d.duplicates, len(d.imports)-len(trash.Imports))
		}

	}

}
