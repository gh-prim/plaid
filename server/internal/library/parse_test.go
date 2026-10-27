package library

import (
	"bufio"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// readTable reads a " | "-separated table from testdata, skipping comments.
func readTable(t *testing.T, name string, cols int) [][]string {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows [][]string
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if strings.TrimSpace(text) == "" || strings.HasPrefix(text, "#") {
			continue
		}
		parts := strings.Split(text, " | ")
		if len(parts) != cols {
			t.Fatalf("%s:%d: %d columns, want %d", name, line, len(parts), cols)
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		rows = append(rows, parts)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

func dash(s string) string {
	if s == "-" {
		return ""
	}
	return s
}

func formatHints(h map[string]string) string {
	if len(h) == 0 {
		return ""
	}
	var parts []string
	for k, v := range h {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func TestParseMovieTable(t *testing.T) {
	rows := readTable(t, "names_movies.txt", 5)
	if len(rows) < 40 {
		t.Errorf("only %d movie names, the plan asks for at least 40", len(rows))
	}
	for _, r := range rows {
		t.Run(r[0], func(t *testing.T) {
			m, ok := ParseMovie(r[0])
			if r[1] == "" {
				if ok {
					t.Errorf("got %+v, want unrecognized", m)
				}
				return
			}
			year, _ := strconv.Atoi(r[2])
			if !ok || m.Title != r[1] || m.Year != year || m.Edition != dash(r[3]) || formatHints(m.Hints) != dash(r[4]) {
				t.Errorf("got ok=%v title=%q year=%d edition=%q hints=%q\nwant title=%q year=%d edition=%q hints=%q",
					ok, m.Title, m.Year, m.Edition, formatHints(m.Hints), r[1], year, dash(r[3]), dash(r[4]))
			}
		})
	}
}

func TestParseMovieVersion(t *testing.T) {
	m, _ := ParseMovie("Movie (2019) - 2160p.mkv")
	if m.Version != "2160p" {
		t.Errorf("version = %q, want 2160p", m.Version)
	}
}

func TestParseEpisodeTable(t *testing.T) {
	rows := readTable(t, "names_episodes.txt", 5)
	if len(rows) < 40 {
		t.Errorf("only %d episode names, the plan asks for at least 40", len(rows))
	}
	for _, r := range rows {
		t.Run(r[0], func(t *testing.T) {
			e, ok := ParseEpisode(r[0])
			if r[1] == "" {
				if ok {
					t.Errorf("got %+v, want unrecognized", e)
				}
				return
			}
			season, _ := strconv.Atoi(r[2])
			var eps []int
			if r[3] != "-" {
				for _, s := range strings.Split(r[3], ",") {
					n, _ := strconv.Atoi(s)
					eps = append(eps, n)
				}
			}
			if !ok || e.Show != r[1] || e.Season != season || !slices.Equal(e.Episodes, eps) || e.AirDate != dash(r[4]) {
				t.Errorf("got ok=%v show=%q season=%d episodes=%v airDate=%q\nwant show=%q season=%d episodes=%v airDate=%q",
					ok, e.Show, e.Season, e.Episodes, e.AirDate, r[1], season, eps, dash(r[4]))
			}
		})
	}
}

func TestParseSeasonFolder(t *testing.T) {
	for name, want := range map[string]int{
		"Season 01": 1, "Season 1": 1, "S01": 1, "Specials": 0, "Saison 3": 3, "Сезон 2": 2,
		"Season.4": 4, "Extras": -1, "Season": -1,
	} {
		if got := parseSeasonFolder(name); got != want {
			t.Errorf("parseSeasonFolder(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestNormTitle(t *testing.T) {
	if a, b := normTitle("The.Matrix"), normTitle("the matrix"); a != b {
		t.Errorf("%q != %q", a, b)
	}
	if got := normTitle("Charlotte's Web!"); got != "charlottes web" {
		t.Errorf("normTitle = %q", got)
	}
}
