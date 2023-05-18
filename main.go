package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	git "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v52/github"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	payloadToken := os.Getenv("TOKEN")
	if payloadToken == "" {
		log.Fatal("TOKEN env var not set")
	}

	h := NewHandler(payloadToken)

	mux := http.NewServeMux()
	mux.HandleFunc("/", h.slashHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: mux,
	}

	log.Printf("Starting server on %s:%v", "localhost", port)
	log.Println(server.ListenAndServe())
}

type Handler struct {
	token  []byte
	logger Logger
}

func NewHandler(token string) Handler {
	return Handler{token: []byte(token)}
}

type Logger struct {
	log.Logger
}

func (l *Logger) Write(data []byte) (n int, err error) {
	bytesReader := bytes.NewReader(data)
	scanner := bufio.NewScanner(bytesReader)
	for scanner.Scan() {
		l.Print(scanner.Text())
	}

	return len(data), nil
}

func (h *Handler) slashHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	payload, err := github.ValidatePayload(r, h.token)
	if err != nil {
		h.logger.Printf("error validating: %v", err)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		h.logger.Printf("error parsing: %v", err)
		return
	}

	switch event := event.(type) {
	case *github.CreateEvent:
		h.handleGitHubCreateEvent(event)
	}
}

func (h *Handler) handleGitHubCreateEvent(event *github.CreateEvent) {
	if *event.RefType == "tag" {
		log.Printf("downloading version %v of arduino-lora", *event.Ref)
		if err := h.gitCloneLatestCode(); err != nil {
			h.logger.Printf("error cloning latest code: %v", err)
		}

		log.Printf("flashing arduino")
		if err := h.flashArduino(); err != nil {
			h.logger.Printf("error flashing arduino: %v", err)
		}

		log.Printf("cleaning up downloaded files")
		if err := h.cleanupTmp(); err != nil {
			h.logger.Printf("error cleaning up /tmp: %v", err)
		}
	} else {
		h.logger.Printf("event not a tag, got ref: %v", *event.Ref)
	}
}

func (h *Handler) gitCloneLatestCode() error {
	_, err := git.PlainClone("/tmp/arduino-lora", false, &git.CloneOptions{
		URL:      "https://github.com/rhysemmas/arduino-lora",
		Progress: &h.logger,
	})

	if err != nil {
		return fmt.Errorf("error cloning git repository: %v", err)
	}

	return nil
}

func (h *Handler) flashArduino() error {
	cmd := exec.Command("pio", "run", "-t", "upload")
	cmd.Dir = "/tmp/arduino-lora"

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error execing command: %v", err)
	}

	return nil
}

func (h *Handler) cleanupTmp() error {
	if err := os.RemoveAll("/tmp/arduino-lora"); err != nil {
		return fmt.Errorf("error calling os.Remove: %v", err)
	}

	return nil
}
