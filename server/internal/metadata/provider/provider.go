// Package provider holds the types metadata providers return and the
// interface they implement, so the metadata service and each provider
// (tmdb today, TVDB maybe later) don't depend on each other.
package provider

import "context"

// Kinds a provider can search.
const (
	Movie = "movie"
	Show  = "tv"
)

// Candidate is one search result (R4, R27).
type Candidate struct {
	TMDBID        int
	Title         string
	OriginalTitle string
	Year          int // 0 when unknown
	Overview      string
	PosterPath    string
	Popularity    float64
}

// Genre is a provider genre, named in the requested language.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Person is a cast or crew credit (R8).
type Person struct {
	TMDBID      int
	Name        string
	Character   string // cast
	Role        string // cast, director, writer, creator
	Order       int
	ProfilePath string
}

// Video is a trailer or teaser on YouTube (R22).
type Video struct {
	Site     string `json:"site"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Language string `json:"language"`
	Type     string `json:"type"`
	Official bool   `json:"official"`
}

// Image is an artwork candidate (R15, used by PR C).
type Image struct {
	Path     string
	Language string // "" for no text
	Width    int
	Height   int
	Votes    float64
}

// Images are a movie's or show's artwork candidates.
type Images struct {
	Posters, Backdrops, Logos []Image
}

// CollectionRef is the collection a movie belongs to.
type CollectionRef struct {
	TMDBID int
	Name   string
}

// SeasonRef is a season listed on a show.
type SeasonRef struct {
	Number       int
	Name         string
	AirDate      string
	EpisodeCount int
}

// Details are a movie or a show, in one language with English filled in
// where it was empty (R7–R11).
type Details struct {
	Kind          string
	TMDBID        int
	IMDbID        string
	TVDBID        int
	Title         string
	OriginalTitle string
	Overview      string
	Tagline       string
	// FieldLanguages names the language of each text field that fell back
	// to English (R11): {"overview": "en"}.
	FieldLanguages map[string]string
	ReleaseDate    string // movie release, show first air date
	EndDate        string // show last air date
	Status         string // shows: returning, ended, canceled, in_production
	Runtime        int    // minutes
	Rating         float64
	RatingCount    int
	Genres         []Genre
	Studios        []string // studios, or networks for shows
	// Certifications holds the rating per country (R24): for movies the
	// theatrical one when there is one.
	Certifications map[string]string
	Cast           []Person
	Crew           []Person
	Trailers       []Video // sorted for the requested language (R22), at most 3
	Collection     *CollectionRef
	Images         Images
	Seasons        []SeasonRef
}

// Episode is one episode of a season.
type Episode struct {
	Season         int
	Number         int
	Name           string
	Overview       string
	AirDate        string
	Runtime        int
	StillPath      string
	Rating         float64
	FieldLanguages map[string]string
}

// Season is a season with its episodes.
type Season struct {
	Number     int
	Name       string
	Overview   string
	AirDate    string
	PosterPath string
	Episodes   []Episode
}

// Collection is a TMDB collection (R23).
type Collection struct {
	TMDBID       int
	Name         string
	Overview     string
	PosterPath   string
	BackdropPath string
	Parts        []Candidate
}

// Locale is what metadata is fetched in: the library's language and region.
type Locale struct {
	Language string // ISO 639-1
	Region   string // ISO 3166-1 alpha-2
}

// Provider is a metadata source.
type Provider interface {
	Search(ctx context.Context, kind, query string, year int, loc Locale) ([]Candidate, error)
	// FindIMDb resolves an IMDb id to a TMDB id of the given kind; 0 when
	// unknown.
	FindIMDb(ctx context.Context, kind, imdbID string) (int, error)
	Movie(ctx context.Context, id int, loc Locale) (*Details, error)
	Show(ctx context.Context, id int, loc Locale) (*Details, error)
	Season(ctx context.Context, showID, number int, loc Locale) (*Season, error)
	Collection(ctx context.Context, id int, loc Locale) (*Collection, error)
}
