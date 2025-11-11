package httpserver

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

const (
	contentLength = "Content-Length"
)

type Request struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

type HandleFunc func(rw ResponseWriter, r Request)

type ResponseWriter interface {
	Write(statusCode int, headers map[string]string, body []byte) error
}

type responseWriter struct {
	connection net.Conn
}

func normalizeHeaderName(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, "-")
}

func makeHeader(name string, value string) string {
	return fmt.Sprintf("%s: %s", name, value)
}

func (rw *responseWriter) Write(statusCode int, headers map[string]string, body []byte) error {
	textResponseStr := fmt.Sprintf("HTTP/1.1 %d", statusCode)

	textResponseStr += "\r\n"

	for k, v := range headers {
		name := normalizeHeaderName(k)

		if name == contentLength {
			// content length is set automatically
			continue
		}

		textResponseStr += makeHeader(name, v)
		textResponseStr += "\r\n"
	}

	// headers
	textResponseStr += makeHeader(contentLength, strconv.Itoa(len(body)))
	textResponseStr += "\r\n"

	// terminate headers with a blank line
	textResponseStr += "\r\n"

	_, err := rw.connection.Write([]byte(textResponseStr))

	if err != nil {
		return err
	}

	_, err = rw.connection.Write(body)

	if err != nil {
		return err
	}

	return nil
}

type Server struct {
	handlers map[string]HandleFunc
}

func NewServer() *Server {
	return &Server{handlers: make(map[string]HandleFunc)}
}

func (s *Server) handleConnection(connection net.Conn, handleFunc HandleFunc) {
	defer connection.Close()

	reader := bufio.NewReader(connection)

	request := Request{
		Method:  "",
		Path:    "",
		Headers: make(map[string]string),
		Body:    nil,
	}

	readLines := 0

	for {
		line, err := reader.ReadString('\n')

		if err != nil {
			log.Fatal(err)
		}

		if line == "\r\n" {
			contentLenStr, hasBody := request.Headers[contentLength]

			contentLen, err := strconv.Atoi(contentLenStr)

			if err != nil {
				contentLen = 0
			}

			if hasBody {
				bodyBytes := make([]byte, contentLen)

				n, err := reader.Read(bodyBytes)

				if err != nil {
					log.Fatal(err)
				}

				if n != contentLen {
					log.Fatalf("Expected to read %v bytes but read %v bytes", contentLenStr, n)
				}

				request.Body = bodyBytes
			}

			break
		}

		readLines++

		if readLines == 1 {
			splits := strings.Split(line, " ")
			request.Method = splits[0]
			request.Path = splits[1]
		} else {
			colon := strings.IndexByte(line, ':')

			if colon < 0 {
				// invalid header format, skip
				continue
			}

			name := normalizeHeaderName(strings.TrimSpace(line[:colon]))
			value := strings.TrimSpace(line[colon+1:])

			request.Headers[name] = value
		}
	}

	handleFunc(&responseWriter{connection}, request)
}

func (s *Server) Serve(port int, handleFunc HandleFunc) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))

	if err != nil {
		return err
	}

	defer listener.Close()

	log.Printf("Listening on port %d", port)

	for {
		conn, err := listener.Accept()

		if err != nil {
			return err
		}

		go s.handleConnection(conn, handleFunc)
	}
}

func (s *Server) HandleFunc(path string, handlerFunc HandleFunc) {
	s.handlers[path] = handlerFunc
}
