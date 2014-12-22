package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/cors"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
	"github.com/nu7hatch/gouuid"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
)

var skipUpload string = os.Getenv("SKIP_S3_UPLOAD")

func main() {
	m := martini.Classic()
	m.Use(cors.Allow(&cors.Options{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	m.Post("/:uuidv4", validateUUID(), func(w http.ResponseWriter, params martini.Params, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered in Post", r)
			}
		}()
		streamHandler(chunkedReader)(w, params, r)
	})
	m.Get("/:uuidv4", validateUUID(), continueUpload)

	m.Run()
}

func validateUUID() martini.Handler {
	return func(w http.ResponseWriter, params martini.Params, r *http.Request) {
		id := params["uuidv4"]
		_, err := uuid.ParseHex(id)
		if err != nil {
			http.Error(w, "Not valid uuidv4", http.StatusBadRequest)
		}
	}
}

type ByChunk []os.FileInfo

func (a ByChunk) Len() int      { return len(a) }
func (a ByChunk) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByChunk) Less(i, j int) bool {
	ai, _ := strconv.Atoi(a[i].Name())
	aj, _ := strconv.Atoi(a[j].Name())
	return ai < aj
}

type streamHandler func(http.ResponseWriter, martini.Params, *http.Request)

type FlowFile struct {
	name string
}

func createFlowFile(params martini.Params, r *http.Request) *FlowFile {
	return &FlowFile{params["uuidv4"] + r.FormValue("flowIdentifier")}
}

func (ff *FlowFile) getBolt() *bolt.DB {
	db, err := bolt.Open(os.Getenv("BOLT_IMAGES"), 0600, nil)
	if err != nil {
		panic(fmt.Sprintf("Bolt Open Error %s", err.Error()))
	}
	return db
}

func (ff *FlowFile) getChunkNum(r *http.Request) string {
	return r.FormValue("flowChunkNumber")
}

func (ff *FlowFile) ChunkExists(r *http.Request) bool {
	db := ff.getBolt()
	defer db.Close()
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ff.name))
		if bucket != nil {
			chunk := bucket.Get([]byte(ff.getChunkNum(r)))
			if chunk != nil {
				return nil
			}
		}
		return errors.New("Chunk does not exist")
	})
	return err == nil
}

func (ff *FlowFile) SaveChunkBytes(r *http.Request, chunkBytes []byte) {
	db := ff.getBolt()
	defer db.Close()
	err := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(ff.name))
		if err != nil {
			return err
		}
		err = bucket.Put([]byte(ff.getChunkNum(r)), chunkBytes)
		return err
	})
	if err != nil {
		panic(err)
	}
}

func (ff *FlowFile) NumberOfChunks() int {
	db := ff.getBolt()
	defer db.Close()
	var numKeys int
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ff.name))
		if bucket != nil {
			stats := bucket.Stats()
			numKeys = stats.KeyN
		}
		return nil
	})
	return numKeys
}

//we can assume that params["uuidv4"] is a valid uuid version 4
func continueUpload(w http.ResponseWriter, params martini.Params, r *http.Request) {
	ff := createFlowFile(params, r)
	if !ff.ChunkExists(r) {
		w.WriteHeader(404)
		return
	}
}

func chunkedReader(w http.ResponseWriter, params martini.Params, r *http.Request) {
	r.ParseMultipartForm(25)

	ff := createFlowFile(params, r)
	for _, fileHeader := range r.MultipartForm.File["file"] {
		src, err := fileHeader.Open()
		if err != nil {
			panic(err.Error())
		}
		defer src.Close()

		chunkBytes, err := ioutil.ReadAll(src)
		if err != nil {
			panic(err.Error())
		} else {
			ff.SaveChunkBytes(r, chunkBytes)
		}

		var chunkTotal = r.FormValue("flowTotalChunks")

		cT, err := strconv.Atoi(chunkTotal)
		if err != nil {
			panic(err.Error())
		}
		if ff.NumberOfChunks() == cT && skipUpload == "" {
			url, err := exportFlowFile(r)
			if err != nil {
				panic(err.Error())
			}
			w.Write([]byte(url))
		}
	}
}

func exportFlowFile(r *http.Request) (string, error) {
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	client := s3.New(auth, aws.USEast)
	bucket := client.Bucket(os.Getenv("S3_BUCKET"))
	imageBytes, err := getFlowFileBytes(r)
	hash := sha256.New()
	hash.Write(imageBytes)
	md := hash.Sum(nil)
	fileName := hex.EncodeToString(md)
	fileExt := getFlowFileKeyExt(r)
	filePath := fileName + fileExt
	mimeType := mime.TypeByExtension(fileExt)
	putError := bucket.Put(filePath, imageBytes, mimeType, s3.PublicRead)
	if putError != nil {
		return "", putError
	}
	removeChunksFromMap(r)
	return bucket.URL(filePath), nil
}

func getFlowFileBytes(r *http.Request) ([]byte, error) {
	chunks, ok := getFlowFile(r)
	if ok {
		image := make([]byte, 0)
		for _, chunk := range chunks {
			image = append(image, chunk...)
		}
		return image, nil
	}
	return nil, errors.New("Image bytes not found")
}
