package main

import (
	"encoding/json"
	"go-practice/internal/httpserver"
	"log"
)

func main() {
	server := httpserver.NewServer()

	err := server.Serve(5877, func(rw httpserver.ResponseWriter, r httpserver.Request) {
		if r.Path == "/json" {
			jsonStruct := struct {
				Hello string `json:"hello"`
			}{
				Hello: "world",
			}

			data, err := json.Marshal(jsonStruct)

			err = rw.Write(200, map[string]string{
				"Content-Type": "application/json",
			}, data)

			if err != nil {
				log.Fatal(err)
			}

			return
		} else if r.Path == "/text" {

			err := rw.Write(200, map[string]string{
				"Content-Type": "text/plain",
			}, []byte("hello world"))

			if err != nil {
				log.Fatal(err)
			}
			return
		}

		err := rw.Write(404, nil, nil)

		if err != nil {
			log.Fatal(err)
		}
	})

	if err != nil {
		log.Fatal(err)
	}
}
