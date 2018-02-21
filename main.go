package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os/exec"

	"github.com/machinebox/sdk-go/facebox"
	"gocv.io/x/gocv"
)

const boundary = "informs"

var (
	fbox *facebox.Client
)

func main() {
	http.HandleFunc("/cam", cam)
	http.HandleFunc("/camFacebox", camFacebox)

	fbox = facebox.New("http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func cam(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// open webcam
	webcam, err := gocv.VideoCaptureDevice(int(0))
	if err != nil {
		log.Println("[ERROR] opening webcam", err)
		http.Error(w, "webcam unavailable", http.StatusInternalServerError)
		return
	}
	defer webcam.Close()

	// prepare image matrix
	img := gocv.NewMat()
	defer img.Close()

	for {
		select {
		case <- r.Context().Done():
			log.Println("[INFO] request closed")
			return
		default:
			if ok := webcam.Read(img); !ok {
				log.Println("[ERROR] reading webcam", err)
				return
			}

			if img.Empty() {
				continue
			}

			b, err := gocv.IMEncode(".jpeg", img)
			if err != nil {
				log.Println("[ERROR] encoding to jpeg", err)
				continue
			}

			w.Write([]byte("Content-Type: image/jpeg\r\n"))
			w.Write([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(b))))
			w.Write(b)
			w.Write([]byte("\r\n"))
			w.Write([]byte("--informs\r\n"))

			gocv.WaitKey(1)
		}
	}
}

func camFacebox(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	cmd := exec.CommandContext(r.Context(), "./capture.py")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("[ERROR] Getting the stdout pipe")
		return
	}
	cmd.Start()

	mr := multipart.NewReader(stdout, boundary)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			log.Println("[DEBUG] EOF")
			break
		}
		if err != nil {
			log.Println("[ERROR] reading next part", err)
			return
		}
		jp, err := ioutil.ReadAll(p)
		if err != nil {
			log.Println("[ERROR] reading from bytes ", err)
			continue
		}
		jpReader := bytes.NewReader(jp)
		faces, err := fbox.Check(jpReader)
		if err != nil {
			log.Println("[ERROR] calling facebox", err)
			continue
		}
		for _, face := range faces {
			if face.Matched {
				fmt.Println("I know you ", face.Name)
			} else {
				fmt.Println("I DO NOT know you ")
			}
		}

		// just MJPEG
		w.Write([]byte("Content-Type: image/jpeg\r\n"))
		w.Write([]byte("Content-Length: " + string(len(jp)) + "\r\n\r\n"))
		w.Write(jp)
		w.Write([]byte("\r\n"))
		w.Write([]byte("--informs\r\n"))
	}
	cmd.Wait()
}
