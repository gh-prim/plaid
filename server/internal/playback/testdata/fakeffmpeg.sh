#!/bin/sh
# A fake FFmpeg for the hardware tests (F006 R24, plan step 24). It claims
# VAAPI, QSV, NVENC and V4L2 M2M, fails on demand, and hands everything
# else to the real FFmpeg in $REAL_FFMPEG.
#
# FAKE_FFMPEG_FAIL is a space-separated list of rules. A rule is words
# joined by "+"; a command fails when each word is one of its arguments:
# "h264_vaapi" fails VAAPI's test encode and its runs, "h264_vaapi+hls"
# only its runs, "vaapi+null" its test decodes.
set -u
fake_encoders="h264_vaapi hevc_vaapi h264_qsv h264_nvenc h264_v4l2m2m"

has() { # has WORD ARG...: WORD is one of the ARGs
	w=$1
	shift
	for a in "$@"; do
		[ "$a" = "$w" ] && return 0
	done
	return 1
}

if has -hwaccels "$@"; then
	printf 'Hardware acceleration methods:\nvaapi\nqsv\ncuda\n\n'
	exit 0
fi
if has -encoders "$@"; then
	"$REAL_FFMPEG" "$@"
	for e in $fake_encoders; do
		printf ' V....D %-20s Fake %s\n' "$e" "$e"
	done
	exit 0
fi
for rule in ${FAKE_FFMPEG_FAIL:-}; do
	all=1
	for w in $(echo "$rule" | tr '+' ' '); do
		has "$w" "$@" || all=0
	done
	if [ "$all" = 1 ]; then
		echo "fake ffmpeg: failing on $rule" >&2
		exit 1
	fi
done
for w in $fake_encoders vaapi qsv cuda; do
	if has "$w" "$@"; then
		# A test encode or decode that works: write the output file.
		for last in "$@"; do :; done
		case $last in *.mp4) : >"$last" ;; esac
		exit 0
	fi
done
exec "$REAL_FFMPEG" "$@"
