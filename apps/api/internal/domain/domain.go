// Package domain holds the core entities of Speed News. It has no external dependencies.
package domain

import "time"

// SeriesSlug identifies a racing series ("f1" today, "wec" in the future).
type SeriesSlug string

const SeriesFormula1 SeriesSlug = "f1"

// NewsSource is an external feed we are allowed to read (headline, summary and link only).
type NewsSource struct {
	Slug         string     `yaml:"slug" json:"slug"`
	DisplayName  string     `yaml:"displayName" json:"displayName"`
	SeriesSlug   SeriesSlug `yaml:"seriesSlug" json:"seriesSlug"`
	LanguageCode string     `yaml:"languageCode" json:"languageCode"`
	FeedKind     string     `yaml:"feedKind" json:"feedKind"`
	FeedURL      string     `yaml:"feedUrl" json:"feedUrl"`
	HomepageURL  string     `yaml:"homepageUrl" json:"homepageUrl"`
	// ImageFromArticlePage: the feed carries no pictures, so read the share image (og:image)
	// of each new article page. Only pages on the HomepageURL's site are ever requested.
	ImageFromArticlePage bool `yaml:"imageFromArticlePage" json:"imageFromArticlePage"`
}

// Article is a normalized news item coming from a NewsSource.
type Article struct {
	ID           string     `json:"id"`
	SeriesSlug   SeriesSlug `json:"seriesSlug"`
	SourceSlug   string     `json:"sourceSlug"`
	SourceName   string     `json:"sourceName"`
	LanguageCode string     `json:"languageCode"`
	Title        string     `json:"title"`
	Summary      string     `json:"summary"`
	ArticleURL   string     `json:"articleUrl"`
	ImageURL     string     `json:"imageUrl,omitempty"`
	PublishedAt  time.Time  `json:"publishedAt"`
	FetchedAt    time.Time  `json:"fetchedAt"`
	ReadAt       *time.Time `json:"readAt,omitempty"`
}

// SessionKind is a stable code; the frontend translates it to a label.
type SessionKind string

const (
	SessionPractice1        SessionKind = "practice_1"
	SessionPractice2        SessionKind = "practice_2"
	SessionPractice3        SessionKind = "practice_3"
	SessionSprintQualifying SessionKind = "sprint_qualifying"
	SessionSprint           SessionKind = "sprint"
	SessionQualifying       SessionKind = "qualifying"
	SessionRace             SessionKind = "race"
)

// RaceSession is one session of a race weekend. StartsAt is always UTC.
type RaceSession struct {
	SessionKind        SessionKind `json:"sessionKind"`
	StartsAt           time.Time   `json:"startsAt"`
	StartTimeConfirmed bool        `json:"startTimeConfirmed"`
}

// GeoCoordinates is a latitude/longitude pair in decimal degrees.
type GeoCoordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// RaceEvent is a race weekend (a Grand Prix) with all of its sessions.
type RaceEvent struct {
	ID          string         `json:"id"`
	SeriesSlug  SeriesSlug     `json:"seriesSlug"`
	Season      int            `json:"season"`
	Round       int            `json:"round"`
	EventName   string         `json:"eventName"`
	CircuitID   string         `json:"circuitId"`
	CircuitName string         `json:"circuitName"`
	Locality    string         `json:"locality"`
	Country     string         `json:"country"`
	Coordinates GeoCoordinates `json:"coordinates"`
	Sessions    []RaceSession  `json:"sessions"`
}

// FirstSessionStartsAt returns the start of the earliest session.
func (raceEvent RaceEvent) FirstSessionStartsAt() time.Time {
	var earliestStart time.Time
	for _, raceSession := range raceEvent.Sessions {
		if earliestStart.IsZero() || raceSession.StartsAt.Before(earliestStart) {
			earliestStart = raceSession.StartsAt
		}
	}
	return earliestStart
}

// LastSessionStartsAt returns the start of the latest session (usually the race).
func (raceEvent RaceEvent) LastSessionStartsAt() time.Time {
	var latestStart time.Time
	for _, raceSession := range raceEvent.Sessions {
		if raceSession.StartsAt.After(latestStart) {
			latestStart = raceSession.StartsAt
		}
	}
	return latestStart
}

// CircuitLayout is the track drawing, already projected to an SVG path.
type CircuitLayout struct {
	CircuitID       string `json:"circuitId"`
	LayoutID        string `json:"layoutId"`
	SVGPath         string `json:"svgPath"`
	SVGViewBox      string `json:"svgViewBox"`
	AttributionText string `json:"attributionText"`
	AttributionURL  string `json:"attributionUrl"`
}

// RaceWeekendOverview is what the race card displays.
type RaceWeekendOverview struct {
	RaceEvent        RaceEvent      `json:"raceEvent"`
	CircuitLayout    *CircuitLayout `json:"circuitLayout,omitempty"`
	IsRaceWeekendNow bool           `json:"isRaceWeekendNow"`
}

// DriverResult is one classified driver in a race.
type DriverResult struct {
	Position     int    `json:"position"`
	DriverCode   string `json:"driverCode"`
	DriverName   string `json:"driverName"`
	TeamName     string `json:"teamName"`
	GridPosition int    `json:"gridPosition"`
	// RaceTime is the winner's total time ("1:27:39.624"); for the others it is the gap ("+0.639"),
	// "+1 Lap" or the retirement status, exactly as published.
	RaceTime string `json:"raceTime"`
}

// FastestLap is the quickest single lap of a race.
type FastestLap struct {
	DriverCode string `json:"driverCode"`
	DriverName string `json:"driverName"`
	TeamName   string `json:"teamName"`
	LapTime    string `json:"lapTime"`
	LapNumber  int    `json:"lapNumber"`
}

// RaceResultSummary is what the "last GP" and "last year here" panels show.
type RaceResultSummary struct {
	Season      int            `json:"season"`
	Round       int            `json:"round"`
	EventName   string         `json:"eventName"`
	CircuitID   string         `json:"circuitId"`
	CircuitName string         `json:"circuitName"`
	RaceDate    string         `json:"raceDate"`
	Laps        int            `json:"laps"`
	Podium      []DriverResult `json:"podium"`
	PoleSitter  *DriverResult  `json:"poleSitter,omitempty"`
	FastestLap  *FastestLap    `json:"fastestLap,omitempty"`
}

// CircuitWinner is one past winner of the Grand Prix at a circuit.
type CircuitWinner struct {
	Season     int    `json:"season"`
	DriverName string `json:"driverName"`
	TeamName   string `json:"teamName"`
}

// CircuitFacts come from the f1-circuits dataset.
type CircuitFacts struct {
	LengthMeters       int `json:"lengthMeters"`
	AltitudeMeters     int `json:"altitudeMeters"`
	FirstGrandPrixYear int `json:"firstGrandPrixYear"`
}

// CircuitProfile is the "track" panel: facts plus history at this circuit.
type CircuitProfile struct {
	CircuitID       string             `json:"circuitId"`
	CircuitName     string             `json:"circuitName"`
	Facts           *CircuitFacts      `json:"facts,omitempty"`
	RecentWinners   []CircuitWinner    `json:"recentWinners"`
	PreviousEdition *RaceResultSummary `json:"previousEdition,omitempty"`
}

// SessionForecast is the weather expected at the start of a session.
type SessionForecast struct {
	SessionKind              SessionKind `json:"sessionKind"`
	StartsAt                 time.Time   `json:"startsAt"`
	TemperatureCelsius       float64     `json:"temperatureCelsius"`
	PrecipitationProbability int         `json:"precipitationProbability"`
	WindSpeedKmh             float64     `json:"windSpeedKmh"`
	// WeatherCode follows the WMO code table used by Open-Meteo (0 clear, 61 rain, 95 thunderstorm...).
	WeatherCode int `json:"weatherCode"`
}

// WeekendForecast covers every session of the upcoming race weekend.
type WeekendForecast struct {
	Sessions        []SessionForecast `json:"sessions"`
	AttributionText string            `json:"attributionText"`
	AttributionURL  string            `json:"attributionUrl"`
}

// TimeZonePreference: mode "system" follows the computer, mode "manual" uses TimeZoneID (IANA).
type TimeZonePreference struct {
	Mode       string `json:"mode"`
	TimeZoneID string `json:"timeZoneId,omitempty"`
}

// Interface languages; "system" follows the operating system.
const (
	InterfaceLanguageSystem     = "system"
	InterfaceLanguagePortuguese = "pt-BR"
	InterfaceLanguageEnglish    = "en"
)

// UserSettings holds every persisted preference of the app.
type UserSettings struct {
	TimeZonePreference TimeZonePreference `json:"timeZonePreference"`
	InterfaceLanguage  string             `json:"interfaceLanguage"`
	// NewsLanguageCodes lists which source languages to show ("pt", "en"). Empty means all,
	// so a language added later to the sources shows up without touching the settings.
	NewsLanguageCodes []string `json:"newsLanguageCodes"`
	// CloseOnFocusLoss hides the popover when the user clicks anywhere else (popover behaviour).
	CloseOnFocusLoss bool `json:"closeOnFocusLoss"`
}

// DefaultUserSettings is used before the user changes anything (and fills fields missing from older saves).
func DefaultUserSettings() UserSettings {
	return UserSettings{
		TimeZonePreference: TimeZonePreference{Mode: "system"},
		InterfaceLanguage:  InterfaceLanguageSystem,
		NewsLanguageCodes:  []string{},
		CloseOnFocusLoss:   true,
	}
}
