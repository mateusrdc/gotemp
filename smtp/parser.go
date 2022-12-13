package smtp

import (
	"bufio"
	b64 "encoding/base64"
	"io"
	"log"
	"mime"
	qp "mime/quotedprintable"
	"net/textproto"
	"regexp"
	"strings"
)

var (
	content_type_regex     = regexp.MustCompile(`(?im)^Content-Type:\s\w*\/\w*.*(\n\s.*.*)?`)
	content_encoding_Regex = regexp.MustCompile(`(?im)^Content-Transfer-Encoding: (.*)$`)
)

func GetHeaders(data string) (*textproto.MIMEHeader, error) {
	reader := textproto.NewReader(bufio.NewReader(strings.NewReader(data)))

	headers, err := reader.ReadMIMEHeader()

	if err != nil {
		return nil, err
	}

	return &headers, nil
}

func decodeMimeHeader(headerContent string) string {
	decoder := new(mime.WordDecoder)
	result, err := decoder.DecodeHeader(headerContent)

	if err != nil {
		return headerContent
	}

	return result
}

func ParseData(data string, trace bool) (string, string) {
	data = normalizeNewlines(data)
	content_types := content_type_regex.FindAllString(data, 10)
	dumb_parse := false

	tracePrintf(trace, "Initializing parser, subject string: \"%s\"\n", data)
	tracePrintln(trace, "Detected content-types: ", content_types)

	if len(content_types) == 0 {
		// Email has no content-type header, use dumb mode
		dumb_parse = true
		tracePrintln(trace, "Falling back to dumb paser due to no content_type")
	} else {
		mimes_baseidx := make(map[string]int)
		preferred_mimes := []string{"text/html", "text/plain"}
		var alternatives_boundary string
		var first_mime string

		// Check if we should use any boundary (when there's multiple content options available)
		for _, value := range content_types {
			var mime_type string

			// Find MIME type
			if idx := strings.Index(value, ";"); idx != -1 {
				mime_type = value[14:idx]
			} else {
				mime_type = value[14:]
			}

			if mime_type == "multipart/alternative" || mime_type == "multipart/mixed" {
				tracePrintf(trace, "%s found (line: %s)\n", mime_type, value)

				// Find boundary
				if start_idx := strings.Index(value, "boundary=\""); start_idx != -1 {
					if end_idx := strings.Index(value[start_idx+10:], "\""); end_idx != -1 {
						alternatives_boundary = value[start_idx+10 : start_idx+10+end_idx]
						tracePrintf(trace, "multipart/alternative boundary found (%s)\n", alternatives_boundary)
					}
				}
			} else if !strings.HasPrefix(mime_type, "multipart/") {
				if first_mime == "" {
					first_mime = mime_type
				}

				// Store the (text position) index of the header
				content_type_regex, err := regexp.Compile("(?i)Content-Type: " + regexp.QuoteMeta(mime_type))

				if err == nil {
					if base_idx := content_type_regex.FindStringIndex(data); base_idx != nil {
						mimes_baseidx[mime_type] = base_idx[0]
					}
				} else {
					if base_idx := strings.Index(data, "Content-Type: "+mime_type); base_idx != -1 {
						mimes_baseidx[mime_type] = base_idx
					}
				}
			}
		}

		// Do we have multiple content options?
		if alternatives_boundary != "" && first_mime != "" {
			// Default to the first mime type found
			base_idx := mimes_baseidx[first_mime]

			tracePrintf(trace, "First MIME (default): \"%s\"\n", first_mime)
			tracePrintf(trace, "Mime base index: %d\n", base_idx)

			// Check if any of our preferred mime types were found
			for _, preferred_mime := range preferred_mimes {
				if idx, ok := mimes_baseidx[preferred_mime]; ok {
					tracePrintf(trace, "Preferred MIME found: \"%s\"\n", preferred_mime)
					tracePrintf(trace, "Preferred MIME index: %d\n", idx)

					base_idx = idx
					break
				}
			}

			// Try to extract the body using the provided MIME's starting position (string index on the document)
			base_str := data[base_idx:]

			tracePrintf(trace, "Body base string: \"%s\"\n", base_str)

			if start_idx := strings.Index(base_str, "\n\n"); start_idx != -1 {
				if end_idx := strings.Index(base_str, "--"+alternatives_boundary); end_idx != -1 {
					body := strings.Trim(data[base_idx+start_idx+2:base_idx+end_idx], "\t \n")

					tracePrintf(trace, "Detected body: \"%s\"\n", body)

					if body != "" {
						var encoding string

						// Find message encoding
						if matches := content_encoding_Regex.FindStringSubmatch(base_str[:start_idx]); matches != nil {
							if len(matches) == 2 {
								encoding = matches[1]
							}
						}

						tracePrintf(trace, "Detected body encoding: \"%s\"\n", encoding)

						return decodeBody(body, encoding), dumbParseHeaders(data)
					}
				}
			}

			// Body not found :(
			// Try the dumb barser
			tracePrintln(trace, "Body not found, resorting to dumb parser.")
			dumb_parse = true
		} else {
			// Resort to dumb parser ¯\_(ツ)_/¯
			dumb_parse = true
			tracePrintln(trace, "No alternative boundary and/or first_mime, using dumb parser.")
		}
	}

	// In the dumb parser mode we just search for two newlinews (\n\n)
	// Everything below the newlines is considered body
	// and everything above is headers
	if dumb_parse {
		tracePrintln(trace, "Starting dumb parser...")

		if idx := strings.Index(data, "\n\n"); idx != -1 {
			body := strings.Trim(data[idx+2:], "\t \n")

			// Try to detect encoding
			var encoding string

			// Find message encoding
			if matches := content_encoding_Regex.FindStringSubmatch(data[:idx]); matches != nil {
				if len(matches) == 2 {
					encoding = matches[1]
				}
			}

			tracePrintf(trace, "Detected encoding: %s\n", encoding)

			return decodeBody(body, encoding), data[:idx]
		}
	}

	return "", dumbParseHeaders(data)
}

func dumbParseHeaders(data string) string {
	if idx := strings.Index(data, "\n\n"); idx != -1 {
		return data[:idx]
	}

	return ""
}

func decodeBody(body string, encoding string) string {
	// Decode base64 if demanded (yes apparently this is a thing)
	if encoding == "base64" {
		bytes, err := b64.StdEncoding.DecodeString(body)

		if err == nil {
			return string(bytes[:])
		}
	}

	// Decode quoted-printable if demanded
	if encoding == "quoted-printable" {
		reader := qp.NewReader(strings.NewReader(body))
		bytes, err := io.ReadAll(reader)

		if err == nil {
			return string(bytes[:])
		}
	}

	return body
}

func normalizeNewlines(data string) string {
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")

	return data
}

func tracePrintln(trace bool, v ...interface{}) {
	if trace {
		v = append([]interface{}{"[PARSER]: "}, v...)

		log.Println(v...)
	}
}

func tracePrintf(trace bool, format string, v ...interface{}) {
	if trace {
		log.Printf("[PARSER]: "+format, v...)
	}
}
