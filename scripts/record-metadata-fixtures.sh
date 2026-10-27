#!/usr/bin/env bash
# Records the TMDB and fanart.tv responses F005's tests replay
# (server/internal/metadata/testdata/{tmdb,fanart}). Run it by hand when the
# fixtures need refreshing, never in CI:
#   TMDB_API_KEY=… FANART_API_KEY=… scripts/record-metadata-fixtures.sh
# ONLY=fanart records fanart.tv alone (ONLY=tmdb, TMDB alone).
# Needs curl, jq and perl. Responses are trimmed to the fields the code reads, so
# the files stay small and reviewable. The key is sent as a query parameter
# and never written to disk.
set -euo pipefail

: "${ONLY:=all}"
root="$(cd "$(dirname "$0")/.." && pwd)"
out="$root/server/internal/metadata/testdata/tmdb"
api="https://api.themoviedb.org/3"
record_fanart() {
  : "${FANART_API_KEY:?set FANART_API_KEY}"
  local dir="$root/server/internal/metadata/testdata/fanart" kind id
  mkdir -p "$dir"
  # Kinds PLAID reads (R16), top 10 each; ids are TMDB for movies, TVDB for shows.
  local trim='with_entries(select(.key | test("^(name|tmdb_id|imdb_id|thetvdb_id|hdmovielogo|hdmovieclearart|moviebackground|movieposter|moviethumb|hdtvlogo|hdclearart|showbackground|tvposter|tvthumb|seasonposter)$")) | if (.value | type) == "array" then .value |= .[:10] else . end)'
  for c in movies/603 movies/348 tv/81189; do
    kind=${c%%/*} id=${c#*/}
    sleep 0.25
    curl -fsS --get "https://webservice.fanart.tv/v3/$kind/$id" --data-urlencode "api_key=$FANART_API_KEY" | jq "$trim" >"$dir/$kind-$id.json"
  done
  echo "fanart.tv fixtures written to ${dir#"$root"/}"
}
if [[ $ONLY == fanart ]]; then
  record_fanart
  exit 0
fi
: "${TMDB_API_KEY:?set TMDB_API_KEY}"
mkdir -p "$out/search" "$out/movie" "$out/tv" "$out/find" "$out/collection"

# get PATH PARAM [CURL ARGS…] → raw JSON on stdout. Stays under TMDB's rate limit.
get() {
  sleep 0.06
  curl -fsS --get "$api$1" --data-urlencode "api_key=$TMDB_API_KEY" --data-urlencode "$2" "${@:3}"
}

slug() { printf '%s' "$1" | perl -CS -MUnicode::Normalize -pe '$_ = lc NFKD($_); s/\p{Mn}//g; s/[^a-z0-9]+/-/g; s/^-|-$//g'; }

# Search results keep the fields the matcher reads, top 10.
trim_search='{total_results, results: [.results[:10][] | {id, title, name, original_title, original_name,
  release_date, first_air_date, popularity, original_language, poster_path, overview: ((.overview // "")[:160])}]}'

# search KIND TITLE YEAR: KIND is movie or tv, YEAR may be empty.
search() {
  local kind=$1 title=$2 year=$3 file yearParam=()
  file="$out/search/$kind-$(slug "$title")-${year:-any}.json"
  if [[ -n $year ]]; then
    if [[ $kind == movie ]]; then yearParam=(--data-urlencode "year=$year"); else yearParam=(--data-urlencode "first_air_date_year=$year"); fi
  fi
  get "/search/$kind" "query=$title" --data-urlencode "language=en-US" ${yearParam[@]+"${yearParam[@]}"} | jq "$trim_search" >"$file"
  # The matcher searches again without the year when nothing is found (R4).
  if [[ -n $year ]] && [[ $(jq .total_results "$file") == 0 ]]; then
    search "$kind" "$title" ""
  fi
}

movies=(
  "The Matrix|1999" "Dune|2021" "Dune|" "Amelie|2001" "Amélie|2001" "Le Fabuleux Destin d'Amélie Poulain|2001"
  "Total Recall|1990" "Total Recall|2012" "Total Recall|" "RoboCop|1987" "RoboCop|2014" "RoboCop|"
  "Les Intouchables|2011" "Intouchables|2011" "The Intouchables|2011" "Der Untergang|2004" "Downfall|2004"
  "2001 A Space Odyssey|1968" "1917|2019" "Se7en|1995" "Seven|1995" "Alien|1979" "Aliens|1986"
  "Blade Runner|1982" "Blade Runner 2049|2017" "Heat|1995" "Spirited Away|2001" "Parasite|2019"
  "The Thing|1982" "The Thing|2011" "Star Wars|1977" "Harry Potter and the Philosophers Stone|2001"
  "Harry Potter and the Sorcerer's Stone|2001" "Mission Impossible Fallout|2018"
  "Spider-Man Into the Spider-Verse|2018" "The Lord of the Rings The Fellowship of the Ring|2001"
  "La Haine|1995" "Oldboy|2003" "Oldboy|2013" "Oldboy|" "Pulp Fiction|1994" "The Godfather Part II|1974"
  "Mad Max Fury Road|2015" "Whiplash|2014" "Inception|2010" "It|2017" "Up|2009" "Her|2013" "Us|2019"
  "Arrival|2016" "Interstellar|2014" "The Departed|2006" "Ratatouille|2007" "Coco|2017" "Joker|2019"
  "Le Diner de cons|1998" "Heat|1996" "Asdfgh Qwerty|2020"
)
shows=(
  "The Office|" "The Office|2005" "The Office|2001" "House of Cards|" "House of Cards|2013" "House of Cards|1990"
  "Dark|2017" "Dark|" "Breaking Bad|" "Lost|2004" "Lost|" "Sherlock|2010" "The Daily Show|" "Doctor Who|2005"
  "Doctor Who|1963" "Shameless|2011" "Shameless|2004" "Money Heist|2017" "La Casa de Papel|2017" "Bref|2011"
  "Kaamelott|2005" "Les Revenants|2012" "Battlestar Galactica|2004" "Battlestar Galactica|1978"
  "Fargo|2014" "Chernobyl|2019" "Arcane|2021" "Asdfgh Qwerty|"
)
for c in "${movies[@]}"; do search movie "${c%%|*}" "${c#*|}"; done
for c in "${shows[@]}"; do search tv "${c%%|*}" "${c#*|}"; done

# Details, in English and French (R9, R11): one call with everything appended.
trim_movie='del(.production_countries, .spoken_languages, .production_companies[]?.logo_path)
  | .credits.cast |= .[:25] | .credits.crew |= map(select(.job == "Director" or .job == "Screenplay" or .job == "Writer"))
  | .images |= with_entries(.value |= .[:10])
  | .translations.translations |= map(select(.iso_639_1 == "en" or .iso_639_1 == "fr"))
  | .release_dates.results |= map(select(.iso_3166_1 == "US" or .iso_3166_1 == "FR" or .iso_3166_1 == "GB" or .iso_3166_1 == "DE"))'
for id in 603 438631 194 348; do
  for lang in en-US fr-FR; do
    get "/movie/$id" "language=$lang" \
      --data-urlencode "append_to_response=credits,videos,images,release_dates,external_ids,translations" \
      --data-urlencode "include_image_language=${lang%%-*},en,null" \
      --data-urlencode "include_video_language=${lang%%-*},en" | jq "$trim_movie" >"$out/movie/$id-$lang.json"
  done
done

trim_tv='del(.production_countries, .spoken_languages, .production_companies[]?.logo_path)
  | .aggregate_credits.cast |= .[:25] | del(.aggregate_credits.crew)
  | .images |= with_entries(.value |= .[:10])
  | .translations.translations |= map(select(.iso_639_1 == "en" or .iso_639_1 == "fr"))
  | .content_ratings.results |= map(select(.iso_3166_1 == "US" or .iso_3166_1 == "FR" or .iso_3166_1 == "GB" or .iso_3166_1 == "DE"))'
trim_season='{id, name, overview, air_date, season_number, poster_path,
  episodes: [.episodes[] | {id, name, overview, air_date, episode_number, season_number, runtime, still_path, vote_average}]}'
# show ID SEASON… : The Office (US), Dark, Breaking Bad, The Daily Show.
show() {
  local id=$1 lang
  shift
  for lang in en-US fr-FR; do
    get "/tv/$id" "language=$lang" \
      --data-urlencode "append_to_response=aggregate_credits,videos,images,content_ratings,external_ids,translations" \
      --data-urlencode "include_image_language=${lang%%-*},en,null" \
      --data-urlencode "include_video_language=${lang%%-*},en" | jq "$trim_tv" >"$out/tv/$id-$lang.json"
    for n in "$@"; do
      get "/tv/$id/season/$n" "language=$lang" | jq "$trim_season" >"$out/tv/$id-s$n-$lang.json"
    done
  done
}
show 2316 1 2
show 70523 1 2
show 1396 0 1
show 2224 29

get "/find/tt0133093" "external_source=imdb_id" | jq '{movie_results: [.movie_results[] | {id, title, release_date}], tv_results}' >"$out/find/tt0133093.json"
for lang in en-US fr-FR; do
  get "/collection/8091" "language=$lang" | jq '{id, name, overview, poster_path, backdrop_path, parts: [.parts[] | {id, title, release_date}]}' >"$out/collection/8091-$lang.json"
done

echo "fixtures written to ${out#"$root"/}: $(find "$out" -name '*.json' | wc -l | tr -d ' ') files"
if [[ $ONLY == all ]]; then
  record_fanart
fi
