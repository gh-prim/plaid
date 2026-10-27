package playback

// StreamPrefix is where a session's streams live; the token follows (R25).
const StreamPrefix = "/api/v1/stream/"

// StreamURL is what the client plays, relative to the server: the file
// itself for Direct Play, else the HLS master playlist.
func StreamURL(s Session) string {
	if s.Decision.Method == DirectPlay {
		return StreamPrefix + s.token + "/file"
	}
	return StreamPrefix + s.token + "/master.m3u8"
}

// DeliveryOf says how a subtitle track reaches the client if it's (or
// were) selected in this session: the selected one as decided, the others
// as they would be.
func DeliveryOf(s Session, sub Subtitle) string {
	d := s.Decision
	if d.Subtitle != nil && d.Subtitle.Index == sub.Index {
		return d.SubtitleDelivery
	}
	return subtitleDelivery(&sub, s.Profile, d.Method == DirectPlay)
}
