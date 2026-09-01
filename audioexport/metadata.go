package audioexport

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"
)

const MetadataSchemaVersion = 1

// TrackMetadata is the stable JSON contract written next to an exported Ogg
// file. It deliberately contains only owned Go values so it can safely outlive
// the protobuf objects used while resolving playback.
type TrackMetadata struct {
	SchemaVersion int             `json:"schemaVersion"`
	Spotify       SpotifyMetadata `json:"spotify"`
	Track         TrackDetails    `json:"track"`
	Artists       []Artist        `json:"artists"`
	Album         *Album          `json:"album,omitempty"`
}

type SpotifyMetadata struct {
	URI              string  `json:"uri,omitempty"`
	TrackID          string  `json:"trackId,omitempty"`
	RequestedURI     string  `json:"requestedUri,omitempty"`
	RequestedTrackID string  `json:"requestedTrackId,omitempty"`
	RequestedGID     string  `json:"requestedGid,omitempty"`
	GID              string  `json:"gid,omitempty"`
	FileID           string  `json:"fileId,omitempty"`
	AudioFormat      *string `json:"audioFormat,omitempty"`
}

type TrackDetails struct {
	Title                 string          `json:"title,omitempty"`
	OriginalTitle         string          `json:"originalTitle,omitempty"`
	VersionTitle          string          `json:"versionTitle,omitempty"`
	CanonicalURI          string          `json:"canonicalUri,omitempty"`
	TrackNumber           *int32          `json:"trackNumber,omitempty"`
	DiscNumber            *int32          `json:"discNumber,omitempty"`
	DurationMs            *int32          `json:"durationMs,omitempty"`
	Popularity            *int32          `json:"popularity,omitempty"`
	Explicit              *bool           `json:"explicit,omitempty"`
	HasLyrics             *bool           `json:"hasLyrics,omitempty"`
	EarliestLiveTimestamp *int64          `json:"earliestLiveTimestamp,omitempty"`
	LicensorUUID          string          `json:"licensorUuid,omitempty"`
	Languages             []string        `json:"languages"`
	Tags                  []string        `json:"tags"`
	ISRC                  string          `json:"isrc,omitempty"`
	ExternalIDs           []ExternalID    `json:"externalIds"`
	AudioFiles            []AudioFile     `json:"audioFiles"`
	PreviewFiles          []AudioFile     `json:"previewFiles"`
	Restrictions          []Restriction   `json:"restrictions"`
	SalePeriods           []SalePeriod    `json:"salePeriods"`
	Availability          []Availability  `json:"availability"`
	OriginalAudio         *OriginalAudio  `json:"originalAudio,omitempty"`
	AudioFormats          []OriginalAudio `json:"audioFormats"`
	ContentRatings        []ContentRating `json:"contentRatings"`
	OriginalVideoGIDs     []string        `json:"originalVideoGids"`
}

type ExternalID struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
}

type AudioFile struct {
	FileID string  `json:"fileId,omitempty"`
	Format *string `json:"format,omitempty"`
}

type Artist struct {
	GID  string `json:"gid,omitempty"`
	Name string `json:"name,omitempty"`
	Role string `json:"role,omitempty"`
}

type Album struct {
	GID                    string         `json:"gid,omitempty"`
	Name                   string         `json:"name,omitempty"`
	Type                   *string        `json:"type,omitempty"`
	TypeString             string         `json:"typeString,omitempty"`
	Label                  string         `json:"label,omitempty"`
	OriginalTitle          string         `json:"originalTitle,omitempty"`
	VersionTitle           string         `json:"versionTitle,omitempty"`
	Popularity             *int32         `json:"popularity,omitempty"`
	PrereleaseEndTimestamp *int64         `json:"prereleaseEndTimestamp,omitempty"`
	Date                   *Date          `json:"date,omitempty"`
	Artists                []AlbumArtist  `json:"artists"`
	ExternalIDs            []ExternalID   `json:"externalIds"`
	Copyrights             []Copyright    `json:"copyrights"`
	Images                 []Image        `json:"images"`
	Restrictions           []Restriction  `json:"restrictions"`
	SalePeriods            []SalePeriod   `json:"salePeriods"`
	Availability           []Availability `json:"availability"`
}

type AlbumArtist struct {
	GID  string `json:"gid,omitempty"`
	Name string `json:"name,omitempty"`
}

type Date struct {
	Year   *int32 `json:"year,omitempty"`
	Month  *int32 `json:"month,omitempty"`
	Day    *int32 `json:"day,omitempty"`
	Hour   *int32 `json:"hour,omitempty"`
	Minute *int32 `json:"minute,omitempty"`
}

type Copyright struct {
	Type *string `json:"type,omitempty"`
	Text string  `json:"text,omitempty"`
}

type Image struct {
	FileID string  `json:"fileId,omitempty"`
	Size   *string `json:"size,omitempty"`
	Width  *int32  `json:"width,omitempty"`
	Height *int32  `json:"height,omitempty"`
}

type Restriction struct {
	Catalogues         []string `json:"catalogues"`
	CatalogueStrings   []string `json:"catalogueStrings"`
	Type               *string  `json:"type,omitempty"`
	CountriesAllowed   string   `json:"countriesAllowed,omitempty"`
	CountriesForbidden string   `json:"countriesForbidden,omitempty"`
}

type Availability struct {
	CatalogueStrings []string `json:"catalogueStrings"`
	Start            *Date    `json:"start,omitempty"`
}

type SalePeriod struct {
	Restrictions []Restriction `json:"restrictions"`
	Start        *Date         `json:"start,omitempty"`
	End          *Date         `json:"end,omitempty"`
}

type OriginalAudio struct {
	UUID   string  `json:"uuid,omitempty"`
	Format *string `json:"format,omitempty"`
}

type ContentRating struct {
	Country string   `json:"country,omitempty"`
	Tags    []string `json:"tags"`
}

// NewTrackMetadata copies a resolved track and its selected audio file into an
// immutable sidecar snapshot. resolvedURI/ID identify the track actually used
// for playback; requestedURI/ID preserve relinking context.
func NewTrackMetadata(resolvedURI, resolvedID, requestedURI, requestedID, requestedGID string, track *metadatapb.Track, file *metadatapb.AudioFile) TrackMetadata {
	metadata := TrackMetadata{
		SchemaVersion: MetadataSchemaVersion,
		Spotify: SpotifyMetadata{
			URI:              resolvedURI,
			TrackID:          resolvedID,
			RequestedURI:     requestedURI,
			RequestedTrackID: requestedID,
			RequestedGID:     requestedGID,
		},
		Artists: make([]Artist, 0),
	}

	if file != nil {
		metadata.Spotify.FileID = hexBytes(file.GetFileId())
		if file.Format != nil {
			metadata.Spotify.AudioFormat = stringPointer(file.Format.String())
		}
	}
	if track == nil {
		return metadata
	}

	metadata.Spotify.GID = hexBytes(track.GetGid())
	metadata.Track = TrackDetails{
		Title:                 track.GetName(),
		OriginalTitle:         track.GetOriginalTitle(),
		VersionTitle:          track.GetVersionTitle(),
		CanonicalURI:          track.GetCanonicalUri(),
		TrackNumber:           int32Pointer(track.Number),
		DiscNumber:            int32Pointer(track.DiscNumber),
		DurationMs:            int32Pointer(track.Duration),
		Popularity:            int32Pointer(track.Popularity),
		Explicit:              boolPointer(track.Explicit),
		HasLyrics:             boolPointer(track.HasLyrics),
		EarliestLiveTimestamp: int64Pointer(track.EarliestLiveTimestamp),
		Languages:             copyStrings(track.GetLanguageOfPerformance()),
		Tags:                  copyStrings(track.GetTags()),
		ExternalIDs:           externalIDs(track.GetExternalId()),
		AudioFiles:            audioFiles(track.GetFile()),
		PreviewFiles:          audioFiles(track.GetPreview()),
		Restrictions:          restrictions(track.GetRestriction()),
		SalePeriods:           salePeriods(track.GetSalePeriod()),
		Availability:          availabilities(track.GetAvailability()),
		OriginalAudio:         originalAudio(track.GetOriginalAudio()),
		AudioFormats:          trackAudioFormats(track.GetAudioFormats()),
		ContentRatings:        contentRatings(track.GetContentRating()),
		OriginalVideoGIDs:     videoGIDs(track.GetOriginalVideo()),
	}
	if track.GetLicensor() != nil {
		metadata.Track.LicensorUUID = hexBytes(track.GetLicensor().GetUuid())
	}
	metadata.Track.ISRC = findISRC(metadata.Track.ExternalIDs)
	metadata.Artists = artists(track)
	metadata.Album = album(track.GetAlbum())
	return metadata
}

func (m TrackMetadata) MarshalJSONIndented() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func writeMetadata(directory string, overwrite bool, fileID, data []byte) (string, error) {
	if len(fileID) == 0 {
		return "", fmt.Errorf("audio export metadata: empty file id")
	}
	if directory == "" {
		return "", fmt.Errorf("audio export metadata: empty directory")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("audio export metadata: create directory: %w", err)
	}

	name := hex.EncodeToString(fileID) + ".json"
	finalPath := filepath.Join(directory, name)
	if !overwrite {
		if _, err := os.Stat(finalPath); err == nil {
			return finalPath, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("audio export metadata: stat target: %w", err)
		}
	}

	tmp, err := os.CreateTemp(directory, "."+name+".*.part")
	if err != nil {
		return "", fmt.Errorf("audio export metadata: create temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return "", fmt.Errorf("audio export metadata: write JSON: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("audio export metadata: sync JSON: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("audio export metadata: close JSON: %w", err)
	}
	if !overwrite {
		if _, err := os.Stat(finalPath); err == nil {
			return finalPath, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("audio export metadata: stat target before rename: %w", err)
		}
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", fmt.Errorf("audio export metadata: finalize JSON: %w", err)
	}
	committed = true
	return finalPath, nil
}

func hexBytes(value []byte) string {
	return hex.EncodeToString(value)
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func stringPointer(value string) *string {
	return &value
}

func int32Pointer(value *int32) *int32 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func int64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func boolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalInt32String(value *int32) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(int64(*value), 10)
}

func externalIDs(values []*metadatapb.ExternalId) []ExternalID {
	result := make([]ExternalID, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		result = append(result, ExternalID{Type: value.GetType(), ID: value.GetId()})
	}
	return result
}

func findISRC(ids []ExternalID) string {
	for _, id := range ids {
		if strings.EqualFold(strings.TrimSpace(id.Type), "isrc") && strings.TrimSpace(id.ID) != "" {
			return id.ID
		}
	}
	return ""
}

func audioFiles(values []*metadatapb.AudioFile) []AudioFile {
	result := make([]AudioFile, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		file := AudioFile{FileID: hexBytes(value.GetFileId())}
		if value.Format != nil {
			file.Format = stringPointer(value.Format.String())
		}
		result = append(result, file)
	}
	return result
}

func artists(track *metadatapb.Track) []Artist {
	result := make([]Artist, 0, len(track.GetArtistWithRole())+len(track.GetArtist()))
	seenEntries := make(map[string]struct{})

	for _, value := range track.GetArtistWithRole() {
		if value == nil {
			continue
		}
		artist := Artist{GID: hexBytes(value.GetArtistGid()), Name: value.GetArtistName(), Role: artistRole(value.Role)}
		identity := artistIdentity(artist.GID, artist.Name)
		entry := identity + "\x00" + artist.Role
		if _, ok := seenEntries[entry]; ok {
			continue
		}
		seenEntries[entry] = struct{}{}
		result = append(result, artist)
	}

	for _, value := range track.GetArtist() {
		if value == nil {
			continue
		}
		artist := Artist{GID: hexBytes(value.GetGid()), Name: value.GetName(), Role: "UNKNOWN"}
		if mergeFallbackArtist(result, artist) {
			continue
		}
		result = append(result, artist)
	}
	return result
}

func mergeFallbackArtist(existing []Artist, fallback Artist) bool {
	if fallback.GID != "" {
		matchedGID := false
		for i := range existing {
			if existing[i].GID == fallback.GID {
				if existing[i].Name == "" {
					existing[i].Name = fallback.Name
				}
				matchedGID = true
			}
		}

		name := normalizedArtistName(fallback.Name)
		enriched := false
		for i := range existing {
			if existing[i].GID == "" && name != "" && normalizedArtistName(existing[i].Name) == name {
				existing[i].GID = fallback.GID
				enriched = true
			}
		}
		return matchedGID || enriched
	}

	name := normalizedArtistName(fallback.Name)
	if name == "" {
		return false
	}
	for _, artist := range existing {
		if normalizedArtistName(artist.Name) == name {
			return true
		}
	}
	return false
}

func normalizedArtistName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func artistIdentity(gid, name string) string {
	if gid != "" {
		return "gid:" + gid
	}
	return "name:" + strings.ToLower(strings.TrimSpace(name))
}

func artistRole(role *metadatapb.ArtistWithRole_ArtistRole) string {
	if role == nil {
		return ""
	}
	return strings.TrimPrefix(role.String(), "ARTIST_ROLE_")
}

func album(value *metadatapb.Album) *Album {
	if value == nil {
		return nil
	}
	result := &Album{
		GID:                    hexBytes(value.GetGid()),
		Name:                   value.GetName(),
		TypeString:             value.GetTypeStr(),
		Label:                  value.GetLabel(),
		OriginalTitle:          value.GetOriginalTitle(),
		VersionTitle:           value.GetVersionTitle(),
		Popularity:             int32Pointer(value.Popularity),
		PrereleaseEndTimestamp: int64Pointer(value.PrereleaseEndDate),
		Date:                   date(value.GetDate()),
		Artists:                make([]AlbumArtist, 0, len(value.GetArtist())),
		ExternalIDs:            externalIDs(value.GetExternalId()),
		Copyrights:             copyrights(value.GetCopyright()),
		Images:                 albumImages(value),
		Restrictions:           restrictions(value.GetRestriction()),
		SalePeriods:            salePeriods(value.GetSalePeriod()),
		Availability:           availabilities(value.GetAvailability()),
	}
	if value.Type != nil {
		result.Type = stringPointer(value.Type.String())
	}
	for _, artist := range value.GetArtist() {
		if artist != nil {
			result.Artists = append(result.Artists, AlbumArtist{GID: hexBytes(artist.GetGid()), Name: artist.GetName()})
		}
	}
	return result
}

func date(value *metadatapb.Date) *Date {
	if value == nil {
		return nil
	}
	return &Date{
		Year:   int32Pointer(value.Year),
		Month:  int32Pointer(value.Month),
		Day:    int32Pointer(value.Day),
		Hour:   int32Pointer(value.Hour),
		Minute: int32Pointer(value.Minute),
	}
}

func copyrights(values []*metadatapb.Copyright) []Copyright {
	result := make([]Copyright, 0, len(values))
	for _, value := range values {
		if value != nil {
			copyright := Copyright{Text: value.GetText()}
			if value.Type != nil {
				copyright.Type = stringPointer(value.Type.String())
			}
			result = append(result, copyright)
		}
	}
	return result
}

func albumImages(value *metadatapb.Album) []Image {
	images := make([]*metadatapb.Image, 0, len(value.GetCover()))
	if value.GetCoverGroup() != nil {
		images = append(images, value.GetCoverGroup().GetImage()...)
	}
	images = append(images, value.GetCover()...)

	result := make([]Image, 0, len(images))
	seen := make(map[string]struct{})
	for _, value := range images {
		if value == nil {
			continue
		}
		image := Image{FileID: hexBytes(value.GetFileId()), Width: int32Pointer(value.Width), Height: int32Pointer(value.Height)}
		if value.Size != nil {
			image.Size = stringPointer(value.Size.String())
		}
		identity := image.FileID
		if identity == "" {
			identity = optionalString(image.Size) + ":" + optionalInt32String(image.Width) + ":" + optionalInt32String(image.Height)
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		result = append(result, image)
	}
	return result
}

func restrictions(values []*metadatapb.Restriction) []Restriction {
	result := make([]Restriction, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		catalogues := make([]string, 0, len(value.GetCatalogue()))
		for _, catalogue := range value.GetCatalogue() {
			catalogues = append(catalogues, catalogue.String())
		}
		restriction := Restriction{
			Catalogues:         catalogues,
			CatalogueStrings:   copyStrings(value.GetCatalogueStr()),
			CountriesAllowed:   value.GetCountriesAllowed(),
			CountriesForbidden: value.GetCountriesForbidden(),
		}
		if value.Type != nil {
			restriction.Type = stringPointer(value.Type.String())
		}
		result = append(result, restriction)
	}
	return result
}

func availabilities(values []*metadatapb.Availability) []Availability {
	result := make([]Availability, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, Availability{CatalogueStrings: copyStrings(value.GetCatalogueStr()), Start: date(value.GetStart())})
		}
	}
	return result
}

func salePeriods(values []*metadatapb.SalePeriod) []SalePeriod {
	result := make([]SalePeriod, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, SalePeriod{Restrictions: restrictions(value.GetRestriction()), Start: date(value.GetStart()), End: date(value.GetEnd())})
		}
	}
	return result
}

func originalAudio(value *metadatapb.Audio) *OriginalAudio {
	if value == nil {
		return nil
	}
	result := &OriginalAudio{UUID: hexBytes(value.GetUuid())}
	if value.Format != nil {
		result.Format = stringPointer(value.Format.String())
	}
	return result
}

func trackAudioFormats(values []*metadatapb.TrackAudioFormat) []OriginalAudio {
	result := make([]OriginalAudio, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		if audio := originalAudio(value.GetOriginalAudio()); audio != nil {
			result = append(result, *audio)
		}
	}
	return result
}

func contentRatings(values []*metadatapb.ContentRating) []ContentRating {
	result := make([]ContentRating, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, ContentRating{Country: value.GetCountry(), Tags: copyStrings(value.GetTag())})
		}
	}
	return result
}

func videoGIDs(values []*metadatapb.Video) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, hexBytes(value.GetGid()))
		}
	}
	return result
}
