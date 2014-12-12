package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/cors"
)

var completedFiles = make(chan string, 100)

func main() {
	for i := 0; i < 3; i++ {
		go assembleFile(completedFiles)
	}

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

func continueUpload(w http.ResponseWriter, r *http.Request) {
	chunkDirPath := "./incomplete/" + r.FormValue("flowFilename") + "/" + r.FormValue("flowChunkNumber")
	if _, err := os.Stat(chunkDirPath); err != nil {
		w.WriteHeader(404)
		return
	}
}

func chunkedReader(w http.ResponseWriter, r *http.Request) error {
	r.ParseMultipartForm(25)

	chunkDirPath := "./incomplete/" + r.FormValue("flowFilename")
	err := os.MkdirAll(chunkDirPath, 02750)
	if err != nil {
		return err
	}

	for _, fileHeader := range r.MultipartForm.File["file"] {
		src, err := fileHeader.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.Create(chunkDirPath + "/" + r.FormValue("flowChunkNumber"))
		if err != nil {
			return err
		}
		defer dst.Close()
		io.Copy(dst, src)

		fileInfos, err := ioutil.ReadDir(chunkDirPath)
		if err != nil {
			return err
		}

		var chunkTotal = r.FormValue("flowTotalChunks")

		cT, err := strconv.Atoi(chunkTotal)
		if err != nil {
			return err
		}
		if len(fileInfos) == cT {
			completedFiles <- chunkDirPath
		}
	}
	return nil
}

func assembleFile(jobs <-chan string) {
	for path := range jobs {
		fileInfos, err := ioutil.ReadDir(path)
		if err != nil {
			return
		}

		// create final file to write to
		dst, err := os.Create(strings.Split(path, "/")[2])
		if err != nil {
			return
		}
		defer dst.Close()

		sort.Sort(ByChunk(fileInfos))
		for _, fs := range fileInfos {
			src, err := os.Open(path + "/" + fs.Name())
			if err != nil {
				return
			}
			defer src.Close()
			io.Copy(dst, src)
		}
		os.RemoveAll(path)
	}
}
