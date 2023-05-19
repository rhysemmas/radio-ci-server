package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	git "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v52/github"
	"github.com/karalabe/usb"
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

	if err := h.updateArduinosWithTaggedCode(); err != nil {
		log.Printf("error updating arduino: %v", err)
	}

	log.Printf("cleaning up any downloaded files")
	if err := h.cleanupDir(); err != nil {
		log.Printf("error cleaning up dir %v: %v", h.workingDir, err)
	}
}

func (h *handler) updateArduinosWithTaggedCode() error {
	if *h.event.RefType != "tag" {
		return fmt.Errorf("event not a tag, got ref: %v", *h.event.Ref)
	}

	log.Printf("downloading version %v of arduino-lora", *h.event.Ref)
	if err := h.gitCloneAndCheckoutRef(); err != nil {
		return fmt.Errorf("error downloading code: %v", err)
	}

	log.Print("flashing arduinos...")
	if err := h.flashArduinos(); err != nil {
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

func (h *handler) flashArduinos() error {
	paths, err := h.findAllArduinoUnos()
	if err != nil {
		return fmt.Errorf("error finding arduinos: %v", err)
	}

	for _, path := range paths {
		cmd := exec.Command("pio", "run", "--upload-port", path, "-t", "upload")
		cmd.Dir = h.workingDir

		log.Printf("flashing arduino at path: %v", path)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error execing command: %v", err)
		}
	}

	return nil
}

func (h *handler) findAllArduinoUnos() ([]string, error) {
	var devicePaths []string

	// Arduino SA Uno R3 has Vendor ID: 2341, 0043
	devices, err := usb.Enumerate(2341, 0043)
	if err != nil {
		return devicePaths, fmt.Errorf("error enumerating usb devices: %v", err)
	}

	for _, device := range devices {
		devicePaths = append(devicePaths, device.Path)
	}

	if len(devicePaths) == 0 {
		return devicePaths, fmt.Errorf("no devices found")
	}

	return devicePaths, nil
}

func (h *handler) cleanupDir() error {
	if err := os.RemoveAll(h.workingDir); err != nil {
		return fmt.Errorf("error calling os.Remove: %v", err)
	}

	return nil
}
