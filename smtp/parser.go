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
	"slices"
	"strings"
)

var (
	content_type_regex          = regexp.MustCompile(`(?im)^Content-Type:\s\w*\/\w*.*(\n\s.*.*)?`)
	content_type_boundary_regex = regexp.MustCompile(`boundary\=\"?([^\"\r\n]{1,512})`)
	content_encoding_Regex      = regexp.MustCompile(`(?im)^Content-Transfer-Encoding: (.*)$`)
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
		preferred_mimes := []string{"text/html", "text/plain"}
		var alternatives_boundary string
		var mime_types []string

		// Check if we should use any boundary (when there's multiple content options available)
		for _, value := range content_types {
			mime_type := getContentTypeHeaderValue(value)

			if mime_type == "multipart/alternative" || mime_type == "multipart/mixed" {
				tracePrintf(trace, "%s found (line: %s)\n", mime_type, value)

				// Find boundary
				if boundary_matches := content_type_boundary_regex.FindStringSubmatch(value); len(boundary_matches) == 2 {
					alternatives_boundary = boundary_matches[1]
					tracePrintf(trace, "multipart/alternative boundary found (%s)\n", alternatives_boundary)
				}
			} else if !strings.HasPrefix(mime_type, "multipart/") {
				mime_types = append(mime_types, mime_type)
			}
		}

		// Do we have multiple content options?
		if alternatives_boundary != "" && len(mime_types) > 0 {
			target_body := ""
			target_mimetype := mime_types[0]

			// Check if any of our preferred mime types were found
			for _, preferred_mime := range preferred_mimes {
				if slices.Contains(mime_types, preferred_mime) {
					tracePrintf(trace, "Preferred MIME found: \"%s\"\n", preferred_mime)
					target_mimetype = preferred_mime
					break
				}
			}

			// Loop through all the boundary-delimited items
			boundary_with_delimiter := "--" + alternatives_boundary
			boundary_index := strings.Index(data, boundary_with_delimiter)

			for boundary_index != 1 {
				// The last boundary is followed by double dashes ("--")
				// when we find them we know there's no other boundary left
				// so we can break the loop
				if data[boundary_index+len(boundary_with_delimiter):boundary_index+len(boundary_with_delimiter)+2] == "--" {
					break
				}

				// Find the next boundary
				next_boundary_index := indexAfter(data, boundary_with_delimiter, boundary_index+len(boundary_with_delimiter))
				var boundary_contents string

				if next_boundary_index != -1 {
					// This should always be the case as even the last boundary-delimited
					// item needs a closing boundary after the body
					// but check nonetheless to avoid misconstructed emails
					boundary_contents = data[boundary_index+len(boundary_with_delimiter) : next_boundary_index]
				} else {
					boundary_contents = data[boundary_index+len(boundary_with_delimiter):]
				}

				// Continue the loop
				boundary_index = next_boundary_index

				// Check if this boundary contains the desired mimetype
				if content_type := content_type_regex.FindString(boundary_contents); content_type != "" {
					mime_type := getContentTypeHeaderValue(content_type)

					if mime_type == target_mimetype {
						target_body = boundary_contents
						break
					}
				}
			}

			tracePrintf(trace, "Target MIME-type: \"%s\"\n", target_mimetype)

			if target_body != "" {
				tracePrintf(trace, "Body base string: \"%s\"\n", target_body)

				// Extract all the headers, getting only the body text/html
				// it will come after the headers and an empty newline (\n\n)
				if start_idx := strings.Index(target_body, "\n\n"); start_idx != -1 {
					body := strings.Trim(target_body[start_idx+2:], "\t \n")

					tracePrintf(trace, "Detected body: \"%s\"\n", body)

					if body != "" {
						var encoding string

						// Find message encoding
						if matches := content_encoding_Regex.FindStringSubmatch(target_body[:start_idx]); matches != nil {
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

// Similar to strings.Index, but with an offset
func indexAfter(s string, substr string, after int) int {
	result := strings.Index(s[after:], substr)

	if result == -1 {
		return -1
	} else {
		return result + after
	}
}

// Gets the Content-Type header value from its whole line
//
// "Content-Type: text/plain; charset=us-ascii" => "text/plain"
func getContentTypeHeaderValue(s string) string {
	if idx := strings.Index(s, ";"); idx != -1 {
		return s[14:idx]
	} else {
		return s[14:]
	}
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
