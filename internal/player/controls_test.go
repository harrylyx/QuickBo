package player

import "testing"

type propMap map[string]string

func (m propMap) SetProp(name, value string) error { m[name] = value; return nil }
func (m propMap) Prop(name string) (string, error) { return m[name], nil }
func (m propMap) Command(...string) error          { return nil }

func TestTracksReadsTrackList(t *testing.T) {
	m := propMap{
		"track-list/count": "3",
		"track-list/0/id":  "1", "track-list/0/type": "video",
		"track-list/1/id": "1", "track-list/1/type": "audio", "track-list/1/lang": "jpn", "track-list/1/selected": "yes",
		"track-list/2/id": "2", "track-list/2/type": "sub", "track-list/2/title": "简中", "track-list/2/external": "yes",
	}
	got := Tracks(m)
	if len(got) != 3 || got[1].Kind != "audio" || !got[1].Selected || got[1].Label() != "日语" {
		t.Fatalf("%+v", got)
	}
	if got[2].Label() != "简中 · 外挂" {
		t.Fatalf("label %q", got[2].Label())
	}
	if (Track{ID: 4}).Label() != "轨道 4" {
		t.Fatal("unnamed track")
	}
}
