package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	git "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v52/github"
)

func main() {
	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatal("TOKEN env var not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	workingDir := os.Getenv("WORKING_DIR")
	if workingDir == "" {
		workingDir = "/tmp/arduino-lora"
	}

	h := newHandler(token, workingDir)

	mux := http.NewServeMux()
	mux.HandleFunc("/", h.slashHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: mux,
	}

	log.Printf("Starting server on %s:%v", "localhost", port)
	log.Println(server.ListenAndServe())
}

type handler struct {
	token      []byte
	workingDir string

	event *github.CreateEvent
}

func newHandler(token, workingDir string) handler {
	return handler{token: []byte(token), workingDir: workingDir}
}

func (h *handler) slashHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	log.Printf("[%v] - %v - %v - %v", r.RemoteAddr, r.Method, r.URL.Path, r.UserAgent())

	payload, err := github.ValidatePayload(r, h.token)
	if err != nil {
		log.Printf("error validating payload: %v", err)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Printf("error parsing webhook: %v", err)
		return
	}

	switch event := event.(type) {
	case *github.CreateEvent:
		h.handleGithubCreateEvent(event)
	}
}

func (h *handler) handleGithubCreateEvent(event *github.CreateEvent) {
	h.event = event

	if err := h.updateArduinoWithTaggedCode(); err != nil {
		log.Printf("error updating arduino: %v", err)
	}

	log.Printf("cleaning up any downloaded files")
	if err := h.cleanupDir(); err != nil {
		log.Printf("error cleaning up dir %v: %v", h.workingDir, err)
	}
}

func (h *handler) updateArduinoWithTaggedCode() error {
	if *h.event.RefType != "tag" {
		return fmt.Errorf("event not a tag, got ref: %v", *h.event.Ref)
	}

	log.Printf("downloading version %v of arduino-lora", *h.event.Ref)
	if err := h.gitCloneAndCheckoutRef(); err != nil {
		return fmt.Errorf("error downloading code: %v", err)
	}

	log.Print("flashing arduino...")
	if err := h.flashArduino(); err != nil {
		return fmt.Errorf("error flashing arduino: %v", err)
	}
	log.Print("done!")

	return nil
}

func (h *handler) gitCloneAndCheckoutRef() error {
	repository, err := git.PlainClone(h.workingDir, false, &git.CloneOptions{
		URL: "https://github.com/rhysemmas/arduino-lora",
	})
	if err != nil {
		return fmt.Errorf("error cloning repository: %v", err)
	}

	ref, err := repository.Tag(*h.event.Ref)
	if err != nil {
		return fmt.Errorf("error getting tag %v: %v", *h.event.Ref, err)
	}

	workTree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("erorr getting worktree: %v", err)
	}

	log.Printf("checking out ref: %v - sha: %v", ref.Name().String(), ref.Hash().String())
	if err := workTree.Checkout(&git.CheckoutOptions{Hash: ref.Hash()}); err != nil {
		return fmt.Errorf("error checking out to commit: %v", ref.Hash().String())
	}

	return nil
}

func (h *handler) flashArduino() error {
	cmd := exec.Command("pio", "run", "-t", "upload")
	cmd.Dir = h.workingDir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error execing command: %v", err)
	}

	return nil
}

func (h *handler) cleanupDir() error {
	if err := os.RemoveAll(h.workingDir); err != nil {
		return fmt.Errorf("error calling os.Remove: %v", err)
	}

	return nil
}
