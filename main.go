package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/go-github/v52/github"
)

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", slashHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: mux,
	}

	log.Printf("Starting server on %s:%v", "localhost", port)
	log.Println(server.ListenAndServe())
}

func slashHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	payload, err := github.ValidatePayload(r, []byte("your_mum"))
	if err != nil {
		log.Printf("error validating: %v", err)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Printf("error parsing: %v", err)
		return
	}

	switch event := event.(type) {
	case *github.CreateEvent:
		handleGitHubCreateEvent(event)
	}
}

// TODO list
// * clone latest code from github after receiving push to main branch
//   - how to git clone
//   - how to copy code into right directory
// * flash code to arduino
//   - how to detect what port arduino is on
//   - how to flash, invoke PIO using os.Exec?

func handleGitHubCreateEvent(event *github.CreateEvent) {
	if *event.RefType == "tag" {
		fmt.Println(event)
	} else {
		log.Printf("event not a tag, got ref: %v", *event.Ref)
	}
}
