package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	git "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v52/github"
	"github.com/google/gousb"
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
		log.Printf("error updating arduinos: %v", err)
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
		return fmt.Errorf("error flashing arduinos: %v", err)
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
	ctx := gousb.NewContext()
	defer ctx.Close()

	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		fmt.Printf("%03d.%03d %s:%s\n", desc.Bus, desc.Address, desc.Vendor, desc.Product)

		// The configurations can be examined from the DeviceDesc, though they can only
		// be set once the device is opened.  All configuration references must be closed,
		// to free up the memory in libusb.
		for _, cfg := range desc.Configs {
			// This loop just uses more of the built-in and usbid pretty printing to list
			// the USB devices.
			fmt.Printf("  %s:\n", cfg)
			for _, intf := range cfg.Interfaces {
				fmt.Printf("    --------------\n")
				for _, ifSetting := range intf.AltSettings {
					fmt.Printf("    %s\n", ifSetting)
					for _, end := range ifSetting.Endpoints {
						fmt.Printf("      %s\n", end)
					}
				}
			}
			fmt.Printf("    --------------\n")
		}

		// After inspecting the descriptor, return true or false depending on whether
		// the device is "interesting" or not.  Any descriptor for which true is returned
		// opens a Device which is retuned in a slice (and must be subsequently closed).
		return false
	})

	// All Devices returned from OpenDevices must be closed.
	defer func() {
		for _, d := range devs {
			d.Close()
		}
	}()

	// OpenDevices can occasionally fail, so be sure to check its return value.
	if err != nil {
		log.Fatalf("list: %s", err)
	}

	for _, dev := range devs {
		// Once the device has been selected from OpenDevices, it is opened
		// and can be interacted with.
		fmt.Println(dev.Desc.Path)
	}

	return []string{}, fmt.Errorf("no devices found")
}

func (h *handler) cleanupDir() error {
	if err := os.RemoveAll(h.workingDir); err != nil {
		return fmt.Errorf("error calling os.Remove: %v", err)
	}

	return nil
}
