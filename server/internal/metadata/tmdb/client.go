// Package tmdb is the TMDB provider (F005 R3–R11, R22–R24): search, IMDb
// lookup, movie and show details in one call each, seasons and collections.
// Every request goes through httpx (allowlist, rate limit, circuit breaker,
// key redaction).
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
)

// Hosts TMDB requests may go to.
var Hosts = []string{"api.themoviedb.org", "image.tmdb.org"}

// BaseURL is TMDB's API root.
const BaseURL = "https://api.themoviedb.org/3"

// Limits from the spec.
const (
	maxCast     = 20
	maxTrailers = 3
	maxResults  = 10
)

// Client talks to TMDB with one API key.
type Client struct {
	http *httpx.Client
	base string
	key  string
}

var _ provider.Provider = (*Client)(nil)

// New returns a client. base is BaseURL outside tests.
func New(h *httpx.Client, base, key string) *Client {
	return &Client{http: h, base: strings.TrimSuffix(base, "/"), key: key}
}

func lang(loc provider.Locale) string {
	l := loc.Language
	if l == "" {
		l = "en"
	}
	if loc.Region == "" {
		return l
	}
	return l + "-" + loc.Region
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	if q == nil {
		q = url.Values{}
	}
	q.Set("api_key", c.key)
	body, err := c.http.Get(ctx, c.base+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("tmdb %s: %w", path, err)
	}
	return nil
}

// --- Search --------------------------------------------------------------

type searchResult struct {
	ID            int      `json:"id"`
	Title         string   `json:"title"`
	Name          string   `json:"name"`
	OriginalTitle string   `json:"original_title"`
	OriginalName  string   `json:"original_name"`
	ReleaseDate   string   `json:"release_date"`
	FirstAirDate  string   `json:"first_air_date"`
	Popularity    float64  `json:"popularity"`
	PosterPath    string   `json:"poster_path"`
	Overview      string   `json:"overview"`
	OriginCountry []string `json:"origin_country"`
}

func (r searchResult) candidate() provider.Candidate {
	c := provider.Candidate{
		TMDBID: r.ID, Title: first(r.Title, r.Name), OriginalTitle: first(r.OriginalTitle, r.OriginalName),
		Overview: r.Overview, PosterPath: r.PosterPath, Popularity: r.Popularity,
		Countries: r.OriginCountry,
	}
	c.Year = year(first(r.ReleaseDate, r.FirstAirDate))
	return c
}

// Search returns the top results for a title, with an optional year (R4).
func (c *Client) Search(ctx context.Context, kind, query string, yr int, loc provider.Locale) ([]provider.Candidate, error) {
	if kind != provider.Movie && kind != provider.Show {
		return nil, fmt.Errorf("tmdb: unknown kind %q", kind)
	}
	q := url.Values{"query": {query}, "language": {lang(loc)}}
	if yr > 0 {
		if kind == provider.Movie {
			q.Set("year", strconv.Itoa(yr))
		} else {
			q.Set("first_air_date_year", strconv.Itoa(yr))
		}
	}
	var res struct {
		Results []searchResult `json:"results"`
	}
	if err := c.get(ctx, "/search/"+kind, q, &res); err != nil {
		return nil, err
	}
	out := make([]provider.Candidate, 0, min(len(res.Results), maxResults))
	for _, r := range res.Results[:min(len(res.Results), maxResults)] {
		out = append(out, r.candidate())
	}
	return out, nil
}

// FindIMDb resolves an IMDb id (R3).
func (c *Client) FindIMDb(ctx context.Context, kind, imdbID string) (int, error) {
	var res struct {
		Movies []struct{ ID int } `json:"movie_results"`
		Shows  []struct{ ID int } `json:"tv_results"`
	}
	if err := c.get(ctx, "/find/"+url.PathEscape(imdbID), url.Values{"external_source": {"imdb_id"}}, &res); err != nil {
		return 0, err
	}
	list := res.Movies
	if kind == provider.Show {
		list = res.Shows
	}
	if len(list) == 0 {
		return 0, nil
	}
	return list[0].ID, nil
}

// --- Details -------------------------------------------------------------

type translation struct {
	Language string `json:"iso_639_1"`
	Data     struct {
		Title    string `json:"title"`
		Name     string `json:"name"`
		Overview string `json:"overview"`
		Tagline  string `json:"tagline"`
	} `json:"data"`
}

type image struct {
	Path     string  `json:"file_path"`
	Language *string `json:"iso_639_1"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Votes    float64 `json:"vote_average"`
}

type video struct {
	Language  string `json:"iso_639_1"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	Site      string `json:"site"`
	Type      string `json:"type"`
	Official  bool   `json:"official"`
	Published string `json:"published_at"`
}

type castMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
	Job         string `json:"job"`
	Roles       []struct {
		Character string `json:"character"`
	} `json:"roles"`
}

type common struct {
	ID          int              `json:"id"`
	Overview    string           `json:"overview"`
	Tagline     string           `json:"tagline"`
	VoteAverage float64          `json:"vote_average"`
	VoteCount   int              `json:"vote_count"`
	Genres      []provider.Genre `json:"genres"`
	ExternalIDs struct {
		IMDbID string `json:"imdb_id"`
		TVDBID int    `json:"tvdb_id"`
	} `json:"external_ids"`
	Translations struct {
		Translations []translation `json:"translations"`
	} `json:"translations"`
	Images struct {
		Posters   []image `json:"posters"`
		Backdrops []image `json:"backdrops"`
		Logos     []image `json:"logos"`
	} `json:"images"`
	Videos struct {
		Results []video `json:"results"`
	} `json:"videos"`
}

type movieJSON struct {
	common
	Title               string `json:"title"`
	OriginalTitle       string `json:"original_title"`
	ReleaseDate         string `json:"release_date"`
	Runtime             int    `json:"runtime"`
	IMDbID              string `json:"imdb_id"`
	BelongsToCollection *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"belongs_to_collection"`
	ProductionCompanies []struct{ Name string } `json:"production_companies"`
	Credits             struct {
		Cast []castMember `json:"cast"`
		Crew []castMember `json:"crew"`
	} `json:"credits"`
	ReleaseDates struct {
		Results []struct {
			Country string `json:"iso_3166_1"`
			Dates   []struct {
				Certification string `json:"certification"`
				Type          int    `json:"type"`
			} `json:"release_dates"`
		} `json:"results"`
	} `json:"release_dates"`
}

type showJSON struct {
	common
	Name             string                  `json:"name"`
	OriginalName     string                  `json:"original_name"`
	FirstAirDate     string                  `json:"first_air_date"`
	LastAirDate      string                  `json:"last_air_date"`
	Status           string                  `json:"status"`
	EpisodeRunTime   []int                   `json:"episode_run_time"`
	Networks         []struct{ Name string } `json:"networks"`
	CreatedBy        []castMember            `json:"created_by"`
	AggregateCredits struct {
		Cast []castMember `json:"cast"`
	} `json:"aggregate_credits"`
	ContentRatings struct {
		Results []struct {
			Country string `json:"iso_3166_1"`
			Rating  string `json:"rating"`
		} `json:"results"`
	} `json:"content_ratings"`
	Seasons []struct {
		Number       int    `json:"season_number"`
		Name         string `json:"name"`
		AirDate      string `json:"air_date"`
		EpisodeCount int    `json:"episode_count"`
	} `json:"seasons"`
}

func detailsQuery(loc provider.Locale, appendTo string) url.Values {
	l := loc.Language
	if l == "" {
		l = "en"
	}
	return url.Values{
		"language":               {lang(loc)},
		"append_to_response":     {appendTo},
		"include_image_language": {l + ",en,null"},
		"include_video_language": {l + ",en"},
	}
}

// Movie fetches a movie with everything appended, in one call (R9).
func (c *Client) Movie(ctx context.Context, id int, loc provider.Locale) (*provider.Details, error) {
	var m movieJSON
	if err := c.get(ctx, "/movie/"+strconv.Itoa(id),
		detailsQuery(loc, "credits,videos,images,release_dates,external_ids,translations"), &m); err != nil {
		return nil, err
	}
	d := &provider.Details{
		Kind: provider.Movie, TMDBID: m.ID, IMDbID: first(m.IMDbID, m.ExternalIDs.IMDbID), TVDBID: m.ExternalIDs.TVDBID,
		Title: m.Title, OriginalTitle: m.OriginalTitle, Overview: m.Overview, Tagline: m.Tagline,
		ReleaseDate: m.ReleaseDate, Runtime: m.Runtime, Rating: m.VoteAverage, RatingCount: m.VoteCount,
		Genres: nonNil(m.Genres), Certifications: map[string]string{},
	}
	for _, p := range m.ProductionCompanies {
		d.Studios = append(d.Studios, p.Name)
	}
	if b := m.BelongsToCollection; b != nil {
		d.Collection = &provider.CollectionRef{TMDBID: b.ID, Name: b.Name}
	}
	// R24: theatrical (type 3) first, else any release with a rating.
	for _, r := range m.ReleaseDates.Results {
		cert := ""
		for _, rd := range r.Dates {
			if rd.Certification == "" {
				continue
			}
			if rd.Type == 3 {
				cert = rd.Certification
				break
			}
			if cert == "" {
				cert = rd.Certification
			}
		}
		if cert != "" {
			d.Certifications[r.Country] = cert
		}
	}
	d.Cast = cast(m.Credits.Cast)
	for _, p := range m.Credits.Crew {
		role := ""
		switch p.Job {
		case "Director":
			role = "director"
		case "Screenplay", "Writer":
			role = "writer"
		default:
			continue
		}
		if !slices.ContainsFunc(d.Crew, func(x provider.Person) bool { return x.TMDBID == p.ID && x.Role == role }) {
			d.Crew = append(d.Crew, provider.Person{TMDBID: p.ID, Name: p.Name, Role: role, Order: len(d.Crew), ProfilePath: p.ProfilePath})
		}
	}
	c.finish(d, &m.common, loc)
	return d, nil
}

// Show fetches a show with everything appended, in one call (R9).
func (c *Client) Show(ctx context.Context, id int, loc provider.Locale) (*provider.Details, error) {
	var s showJSON
	if err := c.get(ctx, "/tv/"+strconv.Itoa(id),
		detailsQuery(loc, "aggregate_credits,videos,images,content_ratings,external_ids,translations"), &s); err != nil {
		return nil, err
	}
	d := &provider.Details{
		Kind: provider.Show, TMDBID: s.ID, IMDbID: s.ExternalIDs.IMDbID, TVDBID: s.ExternalIDs.TVDBID,
		Title: s.Name, OriginalTitle: s.OriginalName, Overview: s.Overview, Tagline: s.Tagline,
		ReleaseDate: s.FirstAirDate, EndDate: s.LastAirDate, Status: showStatus(s.Status),
		Rating: s.VoteAverage, RatingCount: s.VoteCount, Genres: nonNil(s.Genres), Certifications: map[string]string{},
	}
	if len(s.EpisodeRunTime) > 0 {
		d.Runtime = s.EpisodeRunTime[0]
	}
	for _, n := range s.Networks {
		d.Studios = append(d.Studios, n.Name)
	}
	for _, r := range s.ContentRatings.Results {
		if r.Rating != "" {
			d.Certifications[r.Country] = r.Rating
		}
	}
	d.Cast = cast(s.AggregateCredits.Cast)
	for i, p := range s.CreatedBy {
		d.Crew = append(d.Crew, provider.Person{TMDBID: p.ID, Name: p.Name, Role: "creator", Order: i, ProfilePath: p.ProfilePath})
	}
	for _, sn := range s.Seasons {
		d.Seasons = append(d.Seasons, provider.SeasonRef{Number: sn.Number, Name: sn.Name, AirDate: sn.AirDate, EpisodeCount: sn.EpisodeCount})
	}
	c.finish(d, &s.common, loc)
	return d, nil
}

// finish adds the English fallback, images and trailers shared by movies
// and shows.
func (c *Client) finish(d *provider.Details, m *common, loc provider.Locale) {
	d.FieldLanguages = map[string]string{}
	if loc.Language != "" && loc.Language != "en" {
		for _, t := range m.Translations.Translations {
			if t.Language != "en" {
				continue
			}
			fill(&d.Title, first(t.Data.Title, t.Data.Name), "title", d.FieldLanguages)
			fill(&d.Overview, t.Data.Overview, "overview", d.FieldLanguages)
			fill(&d.Tagline, t.Data.Tagline, "tagline", d.FieldLanguages)
			break
		}
	}
	if d.Title == "" {
		d.Title = d.OriginalTitle
	}
	conv := func(in []image) []provider.Image {
		out := make([]provider.Image, 0, len(in))
		for _, i := range in {
			l := ""
			if i.Language != nil {
				l = *i.Language
			}
			out = append(out, provider.Image{Path: i.Path, Language: l, Width: i.Width, Height: i.Height, Votes: i.Votes})
		}
		return out
	}
	d.Images = provider.Images{Posters: conv(m.Images.Posters), Backdrops: conv(m.Images.Backdrops), Logos: conv(m.Images.Logos)}
	d.Trailers = trailers(m.Videos.Results, loc.Language)
}

// fill sets *dst to fallback when it's empty, recording the language.
func fill(dst *string, fallback, field string, langs map[string]string) {
	if strings.TrimSpace(*dst) == "" && fallback != "" {
		*dst = fallback
		langs[field] = "en"
	}
}

func cast(in []castMember) []provider.Person {
	sorted := slices.Clone(in)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Order < sorted[j].Order })
	out := make([]provider.Person, 0, min(len(sorted), maxCast))
	for _, p := range sorted[:min(len(sorted), maxCast)] {
		character := p.Character
		if character == "" && len(p.Roles) > 0 {
			character = p.Roles[0].Character
		}
		out = append(out, provider.Person{TMDBID: p.ID, Name: p.Name, Character: character, Role: "cast", Order: len(out), ProfilePath: p.ProfilePath})
	}
	return out
}

// trailers keeps YouTube trailers and teasers in the library language or
// English, best first: the library language, then trailers before teasers,
// then official ones, then the newest; at most 3 (R22).
func trailers(in []video, language string) []provider.Video {
	var vs []video
	for _, v := range in {
		if v.Site == "YouTube" && (v.Type == "Trailer" || v.Type == "Teaser") &&
			(v.Language == language || v.Language == "en") {
			vs = append(vs, v)
		}
	}
	key := func(v video) string {
		k := []byte("111")
		if v.Language == language {
			k[0] = '0'
		}
		if v.Type == "Trailer" {
			k[1] = '0'
		}
		if v.Official {
			k[2] = '0'
		}
		return string(k)
	}
	sort.SliceStable(vs, func(i, j int) bool {
		if ki, kj := key(vs[i]), key(vs[j]); ki != kj {
			return ki < kj
		}
		return vs[i].Published > vs[j].Published
	})
	out := make([]provider.Video, 0, min(len(vs), maxTrailers))
	for _, v := range vs[:min(len(vs), maxTrailers)] {
		out = append(out, provider.Video{Site: v.Site, Key: v.Key, Name: v.Name, Language: v.Language, Type: v.Type, Official: v.Official})
	}
	return out
}

func showStatus(s string) string {
	switch s {
	case "Returning Series":
		return "returning"
	case "Ended":
		return "ended"
	case "Canceled":
		return "canceled"
	case "In Production", "Planned", "Pilot":
		return "in_production"
	}
	return ""
}

// --- Seasons and collections ---------------------------------------------

type seasonJSON struct {
	Name       string `json:"name"`
	Overview   string `json:"overview"`
	AirDate    string `json:"air_date"`
	Number     int    `json:"season_number"`
	PosterPath string `json:"poster_path"`
	Episodes   []struct {
		Name      string  `json:"name"`
		Overview  string  `json:"overview"`
		AirDate   string  `json:"air_date"`
		Number    int     `json:"episode_number"`
		Season    int     `json:"season_number"`
		Runtime   int     `json:"runtime"`
		StillPath string  `json:"still_path"`
		Rating    float64 `json:"vote_average"`
	} `json:"episodes"`
}

// placeholderName matches the generic names TMDB returns for untranslated
// episodes ("Épisode 3", "Folge 3").
var placeholderName = regexp.MustCompile(`(?i)^(episode|épisode|episodio|episódio|folge|aflevering|avsnitt|afsnit|jakso|odcinek|эпизод|серия)\s*\d+$`)

// Season fetches a season. Outside English, episodes with no overview or a
// placeholder name get the English one, with one extra call (R11).
func (c *Client) Season(ctx context.Context, showID, number int, loc provider.Locale) (*provider.Season, error) {
	path := "/tv/" + strconv.Itoa(showID) + "/season/" + strconv.Itoa(number)
	var s seasonJSON
	if err := c.get(ctx, path, url.Values{"language": {lang(loc)}}, &s); err != nil {
		return nil, err
	}
	out := &provider.Season{Number: s.Number, Name: s.Name, Overview: s.Overview, AirDate: s.AirDate, PosterPath: s.PosterPath}
	missing := false
	for _, e := range s.Episodes {
		ep := provider.Episode{
			Season: e.Season, Number: e.Number, Name: e.Name, Overview: e.Overview, AirDate: e.AirDate,
			Runtime: e.Runtime, StillPath: e.StillPath, Rating: e.Rating, FieldLanguages: map[string]string{},
		}
		if placeholderName.MatchString(ep.Name) {
			ep.Name = ""
		}
		missing = missing || ep.Name == "" || ep.Overview == ""
		out.Episodes = append(out.Episodes, ep)
	}
	if !missing || loc.Language == "" || loc.Language == "en" {
		return out, nil
	}
	var en seasonJSON
	if err := c.get(ctx, path, url.Values{"language": {"en-US"}}, &en); err != nil {
		if errors.Is(err, httpx.ErrNotFound) {
			return out, nil
		}
		return nil, err
	}
	byNumber := map[int]int{}
	for i, e := range en.Episodes {
		byNumber[e.Number] = i
	}
	for i := range out.Episodes {
		ep := &out.Episodes[i]
		j, ok := byNumber[ep.Number]
		if !ok {
			continue
		}
		fill(&ep.Name, en.Episodes[j].Name, "name", ep.FieldLanguages)
		fill(&ep.Overview, en.Episodes[j].Overview, "overview", ep.FieldLanguages)
	}
	return out, nil
}

// Collection fetches a collection (R23).
func (c *Client) Collection(ctx context.Context, id int, loc provider.Locale) (*provider.Collection, error) {
	var col struct {
		ID           int            `json:"id"`
		Name         string         `json:"name"`
		Overview     string         `json:"overview"`
		PosterPath   string         `json:"poster_path"`
		BackdropPath string         `json:"backdrop_path"`
		Parts        []searchResult `json:"parts"`
	}
	if err := c.get(ctx, "/collection/"+strconv.Itoa(id), url.Values{"language": {lang(loc)}}, &col); err != nil {
		return nil, err
	}
	out := &provider.Collection{TMDBID: col.ID, Name: col.Name, Overview: col.Overview, PosterPath: col.PosterPath, BackdropPath: col.BackdropPath}
	for _, p := range col.Parts {
		out.Parts = append(out.Parts, p.candidate())
	}
	return out, nil
}

// --- helpers -------------------------------------------------------------

func first(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func year(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
