package audioexport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"
)

func testPtr[T any](value T) *T { return &value }

func TestNewTrackMetadataPreservesIdentifiersArtistsAndSelectedFile(t *testing.T) {
	track := &metadatapb.Track{
		Gid:                   []byte{0x10, 0x20},
		Name:                  testPtr("Track title"),
		Number:                testPtr(int32(3)),
		DiscNumber:            testPtr(int32(2)),
		Duration:              testPtr(int32(222200)),
		ExternalId:            []*metadatapb.ExternalId{{Type: testPtr("UPC"), Id: testPtr("upc-value")}, {Type: testPtr("IsRc"), Id: testPtr("USRC17607839")}},
		LanguageOfPerformance: []string{"en"},
		ArtistWithRole: []*metadatapb.ArtistWithRole{
			{ArtistGid: []byte{0x01}, ArtistName: testPtr("Main"), Role: metadatapb.ArtistWithRole_ARTIST_ROLE_MAIN_ARTIST.Enum()},
			{ArtistGid: []byte{0x02}, ArtistName: testPtr("Feature"), Role: metadatapb.ArtistWithRole_ARTIST_ROLE_FEATURED_ARTIST.Enum()},
		},
		Artist: []*metadatapb.Artist{
			// artist_with_role can carry a GID even when the fallback list does not.
			{Name: testPtr("Main")},
			{Gid: []byte{0x03}, Name: testPtr("Fallback")},
		},
	}
	file := &metadatapb.AudioFile{FileId: []byte{0xfa, 0xce}, Format: metadatapb.AudioFile_OGG_VORBIS_320.Enum()}

	got := NewTrackMetadata(
		"spotify:track:resolved", "resolved", "spotify:track:requested", "requested", "aabb", track, file,
	)

	if got.Track.ISRC != "USRC17607839" {
		t.Fatalf("unexpected ISRC %q", got.Track.ISRC)
	}
	if len(got.Track.ExternalIDs) != 2 || got.Track.ExternalIDs[0].Type != "UPC" || got.Track.ExternalIDs[1].Type != "IsRc" {
		t.Fatalf("external IDs not preserved: %#v", got.Track.ExternalIDs)
	}
	if len(got.Artists) != 3 {
		t.Fatalf("expected two role artists plus one fallback, got %#v", got.Artists)
	}
	if got.Artists[0].Role != "MAIN_ARTIST" || got.Artists[1].Role != "FEATURED_ARTIST" || got.Artists[2].Role != "UNKNOWN" {
		t.Fatalf("unexpected artist roles: %#v", got.Artists)
	}
	if got.Spotify.GID != "1020" || got.Spotify.FileID != "face" || got.Spotify.AudioFormat == nil || *got.Spotify.AudioFormat != "OGG_VORBIS_320" {
		t.Fatalf("selected identifiers not represented: %#v", got.Spotify)
	}
	if got.Spotify.URI != "spotify:track:resolved" || got.Spotify.RequestedURI != "spotify:track:requested" || got.Spotify.RequestedGID != "aabb" {
		t.Fatalf("resolved/requested identity lost: %#v", got.Spotify)
	}
}

func TestNewTrackMetadataFallsBackToTrackArtists(t *testing.T) {
	track := &metadatapb.Track{Artist: []*metadatapb.Artist{
		{Gid: []byte{0x01}, Name: testPtr("One")},
		{Gid: []byte{0x02}, Name: testPtr("Two")},
	}}

	got := NewTrackMetadata("", "", "", "", "", track, nil)
	if len(got.Artists) != 2 || got.Artists[0].Role != "UNKNOWN" || got.Artists[1].Name != "Two" {
		t.Fatalf("unexpected fallback artists: %#v", got.Artists)
	}
}

func TestNewTrackMetadataPreservesAlbumAndImages(t *testing.T) {
	albumType := metadatapb.Album_SINGLE
	imageSize := metadatapb.Image_LARGE
	track := &metadatapb.Track{Album: &metadatapb.Album{
		Gid:        []byte{0xaa},
		Name:       testPtr("Album"),
		Type:       &albumType,
		TypeStr:    testPtr("single"),
		Label:      testPtr("Label"),
		Date:       &metadatapb.Date{Year: testPtr(int32(2026)), Month: testPtr(int32(8)), Day: testPtr(int32(31))},
		Artist:     []*metadatapb.Artist{{Gid: []byte{0xbb}, Name: testPtr("Album artist")}},
		ExternalId: []*metadatapb.ExternalId{{Type: testPtr("upc"), Id: testPtr("123")}},
		Copyright:  []*metadatapb.Copyright{{Type: metadatapb.Copyright_C.Enum(), Text: testPtr("Copyright")}},
		CoverGroup: &metadatapb.ImageGroup{Image: []*metadatapb.Image{{FileId: []byte{0xcc}, Size: &imageSize, Width: testPtr(int32(640)), Height: testPtr(int32(640))}}},
		// The legacy cover list can contain the same image as cover_group.
		Cover: []*metadatapb.Image{{FileId: []byte{0xcc}, Size: &imageSize}},
	}}

	got := NewTrackMetadata("", "", "", "", "", track, nil)
	if got.Album == nil {
		t.Fatal("album missing")
	}
	if got.Album.GID != "aa" || got.Album.Type == nil || *got.Album.Type != "SINGLE" || got.Album.Date == nil || got.Album.Date.Year == nil || *got.Album.Date.Year != 2026 {
		t.Fatalf("album metadata missing: %#v", got.Album)
	}
	if len(got.Album.Artists) != 1 || len(got.Album.ExternalIDs) != 1 || len(got.Album.Copyrights) != 1 {
		t.Fatalf("album collections missing: %#v", got.Album)
	}
	if len(got.Album.Images) != 1 || got.Album.Images[0].FileID != "cc" || got.Album.Images[0].Width == nil || *got.Album.Images[0].Width != 640 {
		t.Fatalf("album image metadata missing or duplicated: %#v", got.Album.Images)
	}
}

func TestFindISRCIgnoresEmptyValue(t *testing.T) {
	got := findISRC([]ExternalID{
		{Type: "ISRC", ID: ""},
		{Type: "isrc", ID: "USRC17607839"},
	})
	if got != "USRC17607839" {
		t.Fatalf("got %q", got)
	}
}

func TestArtistFallbackEnrichesRoleArtistGID(t *testing.T) {
	track := &metadatapb.Track{
		ArtistWithRole: []*metadatapb.ArtistWithRole{{
			ArtistName: testPtr("Same Artist"),
			Role:       metadatapb.ArtistWithRole_ARTIST_ROLE_MAIN_ARTIST.Enum(),
		}},
		Artist: []*metadatapb.Artist{{Gid: []byte{0xaa}, Name: testPtr(" same artist ")}},
	}

	got := NewTrackMetadata("", "", "", "", "", track, nil)
	if len(got.Artists) != 1 || got.Artists[0].GID != "aa" || got.Artists[0].Role != "MAIN_ARTIST" {
		t.Fatalf("role artist was not enriched: %#v", got.Artists)
	}
}

func TestSameNameArtistsWithDistinctGIDsRemainDistinct(t *testing.T) {
	track := &metadatapb.Track{Artist: []*metadatapb.Artist{
		{Gid: []byte{0x01}, Name: testPtr("Shared Name")},
		{Gid: []byte{0x02}, Name: testPtr("Shared Name")},
	}}

	got := NewTrackMetadata("", "", "", "", "", track, nil)
	if len(got.Artists) != 2 || got.Artists[0].GID != "01" || got.Artists[1].GID != "02" {
		t.Fatalf("distinct artists were collapsed: %#v", got.Artists)
	}
}

func TestArtistRolesRemainDistinctDuringGIDEnrichment(t *testing.T) {
	track := &metadatapb.Track{
		ArtistWithRole: []*metadatapb.ArtistWithRole{
			{ArtistName: testPtr("Same Artist"), Role: metadatapb.ArtistWithRole_ARTIST_ROLE_MAIN_ARTIST.Enum()},
			{ArtistName: testPtr("Same Artist"), Role: metadatapb.ArtistWithRole_ARTIST_ROLE_FEATURED_ARTIST.Enum()},
		},
		Artist: []*metadatapb.Artist{{Gid: []byte{0xaa}, Name: testPtr("Same Artist")}},
	}

	got := NewTrackMetadata("", "", "", "", "", track, nil)
	if len(got.Artists) != 2 || got.Artists[0].GID != "aa" || got.Artists[1].GID != "aa" || got.Artists[0].Role == got.Artists[1].Role {
		t.Fatalf("semantic roles were collapsed: %#v", got.Artists)
	}
}

func TestOptionalProtoEnumsPreservePresence(t *testing.T) {
	absent := NewTrackMetadata("", "", "", "", "", &metadatapb.Track{
		Album:          &metadatapb.Album{Copyright: []*metadatapb.Copyright{{}}},
		File:           []*metadatapb.AudioFile{{}},
		Restriction:    []*metadatapb.Restriction{{}},
		ArtistWithRole: []*metadatapb.ArtistWithRole{{ArtistName: testPtr("Artist")}},
	}, &metadatapb.AudioFile{})
	absentJSON, err := absent.MarshalJSONIndented()
	if err != nil {
		t.Fatal(err)
	}
	assertJSONFieldAbsent(t, absentJSON, "spotify", "audioFormat")
	assertJSONFieldAbsent(t, absentJSON, "album", "type")
	assertJSONFieldAbsent(t, absentJSON, "track", "durationMs")
	assertJSONFieldAbsent(t, absentJSON, "track", "explicit")
	assertJSONArrayFieldAbsent(t, absentJSON, "track", "audioFiles", "format")
	assertJSONArrayFieldAbsent(t, absentJSON, "track", "restrictions", "type")
	assertJSONArrayFieldAbsent(t, absentJSON, "album", "copyrights", "type")
	assertTopLevelArrayFieldAbsent(t, absentJSON, "artists", "role")

	present := NewTrackMetadata("", "", "", "", "", &metadatapb.Track{
		Album: &metadatapb.Album{
			Type:      metadatapb.Album_ALBUM.Enum(),
			Copyright: []*metadatapb.Copyright{{Type: metadatapb.Copyright_P.Enum()}},
		},
		File:        []*metadatapb.AudioFile{{Format: metadatapb.AudioFile_OGG_VORBIS_96.Enum()}},
		Restriction: []*metadatapb.Restriction{{Type: metadatapb.Restriction_STREAMING.Enum()}},
		Duration:    testPtr(int32(0)),
		Explicit:    testPtr(false),
		ArtistWithRole: []*metadatapb.ArtistWithRole{{
			ArtistName: testPtr("Artist"),
			Role:       metadatapb.ArtistWithRole_ARTIST_ROLE_UNKNOWN.Enum(),
		}},
	}, &metadatapb.AudioFile{Format: metadatapb.AudioFile_OGG_VORBIS_96.Enum()})
	presentJSON, err := present.MarshalJSONIndented()
	if err != nil {
		t.Fatal(err)
	}
	assertJSONFieldEquals(t, presentJSON, "spotify", "audioFormat", "OGG_VORBIS_96")
	assertJSONFieldEquals(t, presentJSON, "album", "type", "ALBUM")
	assertJSONFieldEquals(t, presentJSON, "track", "durationMs", float64(0))
	assertJSONFieldEquals(t, presentJSON, "track", "explicit", false)
	assertJSONArrayFieldEquals(t, presentJSON, "track", "audioFiles", "format", "OGG_VORBIS_96")
	assertJSONArrayFieldEquals(t, presentJSON, "track", "restrictions", "type", "STREAMING")
	assertJSONArrayFieldEquals(t, presentJSON, "album", "copyrights", "type", "P")
	assertTopLevelArrayFieldEquals(t, presentJSON, "artists", "role", "UNKNOWN")
}

func decodeMetadataJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func assertJSONFieldAbsent(t *testing.T, data []byte, object, field string) {
	t.Helper()
	decoded := decodeMetadataJSON(t, data)
	value := decoded[object].(map[string]any)
	if _, ok := value[field]; ok {
		t.Fatalf("%s.%s should be absent: %s", object, field, data)
	}
}

func assertJSONFieldEquals(t *testing.T, data []byte, object, field string, want any) {
	t.Helper()
	decoded := decodeMetadataJSON(t, data)
	value := decoded[object].(map[string]any)
	if value[field] != want {
		t.Fatalf("%s.%s = %#v, want %#v", object, field, value[field], want)
	}
}

func assertJSONArrayFieldAbsent(t *testing.T, data []byte, object, array, field string) {
	t.Helper()
	decoded := decodeMetadataJSON(t, data)
	items := decoded[object].(map[string]any)[array].([]any)
	if _, ok := items[0].(map[string]any)[field]; ok {
		t.Fatalf("%s.%s[0].%s should be absent: %s", object, array, field, data)
	}
}

func assertJSONArrayFieldEquals(t *testing.T, data []byte, object, array, field, want string) {
	t.Helper()
	decoded := decodeMetadataJSON(t, data)
	items := decoded[object].(map[string]any)[array].([]any)
	if got := items[0].(map[string]any)[field]; got != want {
		t.Fatalf("%s.%s[0].%s = %#v, want %q", object, array, field, got, want)
	}
}

func assertTopLevelArrayFieldAbsent(t *testing.T, data []byte, array, field string) {
	t.Helper()
	items := decodeMetadataJSON(t, data)[array].([]any)
	if _, ok := items[0].(map[string]any)[field]; ok {
		t.Fatalf("%s[0].%s should be absent: %s", array, field, data)
	}
}

func assertTopLevelArrayFieldEquals(t *testing.T, data []byte, array, field, want string) {
	t.Helper()
	items := decodeMetadataJSON(t, data)[array].([]any)
	if got := items[0].(map[string]any)[field]; got != want {
		t.Fatalf("%s[0].%s = %#v, want %q", array, field, got, want)
	}
}

func TestTrackMetadataSerializationProducesValidJSON(t *testing.T) {
	metadata := NewTrackMetadata("spotify:track:id", "id", "spotify:track:id", "id", "010203", &metadatapb.Track{Name: testPtr("Title")}, &metadatapb.AudioFile{
		FileId: []byte{1, 2, 3},
		Format: metadatapb.AudioFile_OGG_VORBIS_160.Enum(),
	})
	data, err := metadata.MarshalJSONIndented()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded["schemaVersion"] != float64(MetadataSchemaVersion) {
		t.Fatalf("schema version missing: %s", data)
	}
}

func TestWriteMetadataCreatesAtomicJSON(t *testing.T) {
	dir := t.TempDir()
	data := []byte("{\"schemaVersion\":1}\n")
	path, err := writeMetadata(dir, false, []byte{0x01, 0x02, 0x03}, data)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "010203.json") {
		t.Fatalf("unexpected path %q", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("got %q, want %q", got, data)
	}
	parts, err := filepath.Glob(filepath.Join(dir, ".010203.json.*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 0 {
		t.Fatalf("temporary files left behind: %v", parts)
	}
}

func TestWriteMetadataDoesNotOverwriteExistingJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "010203.json")
	if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	gotPath, err := writeMetadata(dir, false, []byte{0x01, 0x02, 0x03}, []byte("replacement"))
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("got path %q, want %q", gotPath, path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing" {
		t.Fatalf("existing metadata was overwritten: %q", got)
	}
}
