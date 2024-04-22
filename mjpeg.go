package rpicamvid

import "bytes"

var (
	jpegTrailer = []byte{0xFF, 0xD9}
)

// mjpegSplitFunc splits an MJPEG stream into individual JPEG frames by finding the JPEG trailer bytes at the end of
// each frame.
// It is intended to be used as a bufio.SplitFunc for bufio.Scanner.
//
//	scanner := bufio.NewScanner(stdout)
//	scanner.Split(rpicamvid.mjpegSplitFunc)
func mjpegSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.Index(data, jpegTrailer); i >= 0 {
		return i + 2, data[0 : i+2], nil
	}

	// Request more data.
	return 0, nil, nil
}
