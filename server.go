package main

import (
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/cors"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

type flowFileChunks map[string][]byte

var flowFiles map[string]flowFileChunks = make(map[string]flowFileChunks)

func main() {
	m := martini.Classic()
	m.Use(cors.Allow(&cors.Options{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	m.Post("/", streamHandler(chunkedReader))
	m.Get("/", continueUpload)

	m.Run()
}

type ByChunk []os.FileInfo

func (a ByChunk) Len() int      { return len(a) }
func (a ByChunk) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByChunk) Less(i, j int) bool {
	ai, _ := strconv.Atoi(a[i].Name())
	aj, _ := strconv.Atoi(a[j].Name())
	return ai < aj
}

type streamHandler func(http.ResponseWriter, *http.Request) error

func (fn streamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func getFlowFileKey(r *http.Request) string {
	return r.FormValue("flowFilename")
}

func getFlowFile(r *http.Request) (flowFileChunks, bool) {
	chunks, ok := flowFiles[getFlowFileKey(r)]
	return chunks, ok
}

func flowFileExist(r *http.Request) bool {
	if _, ok := getFlowFile(r); ok {
		return ok
	} else {
		return !ok
	}
}

func flowFileNumberOfChunks(r *http.Request) int {
	if chunks, ok := getFlowFile(r); ok {
		return len(chunks)
	} else {
		return 0
	}
}

func saveChunkBytes(r *http.Request, bytes []byte) {
	if chunks, ok := getFlowFile(r); ok {
		chunks[r.FormValue("flowChunkNumber")] = bytes
		saveFlowFileChunks(r, chunks)
	} else {
		flowFiles[getFlowFileKey(r)] = make(flowFileChunks)
		flowFiles[getFlowFileKey(r)][r.FormValue("flowChunkNumber")] = bytes
	}
}

func saveFlowFileChunks(r *http.Request, chunks flowFileChunks) {
	flowFiles[getFlowFileKey(r)] = chunks
}

func continueUpload(w http.ResponseWriter, r *http.Request) {
	if flowFileExist(r) {
		w.WriteHeader(404)
		return
	}
}

func chunkedReader(w http.ResponseWriter, r *http.Request) error {
	r.ParseMultipartForm(25)

	for _, fileHeader := range r.MultipartForm.File["file"] {
		src, err := fileHeader.Open()
		if err != nil {
			fmt.Print(err)
			return err
		}
		defer src.Close()

		bytes, err := ioutil.ReadAll(src)
		if err != nil {
			fmt.Print(err)
			return err
		} else {
			saveChunkBytes(r, bytes)
		}

		var chunkTotal = r.FormValue("flowTotalChunks")

		cT, err := strconv.Atoi(chunkTotal)
		if err != nil {
			fmt.Print(err)
			return err
		}
		if flowFileNumberOfChunks(r) == cT {
			fmt.Print("All done")
			w.Write([]byte("Completed Upload"))
		} else {
			fmt.Print("not done yet")
		}
	}
	return nil
}
