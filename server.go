package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	_ "github.com/lib/pq"
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
	"strings"
)

var skipUpload string = os.Getenv("SKIP_S3_UPLOAD")
var boltChunks string = "BOLT_CHUNKS"

var s3Bucket string = "S3_BUCKET"
var cloudfrontURL string = os.Getenv("CLOUDFRONT_URL")

func init() {
	bChunks := os.Getenv(boltChunks)
	if bChunks == "" {
		log.Fatal(fmt.Sprintf("Please define %s in your environment.", boltChunks))
	}
	_, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	if os.Getenv(s3Bucket) == "" {
		log.Fatal(fmt.Sprintf("Please define a S3 bucket with %s", s3Bucket))
	}
}

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

	m.Get("/:uuidv4/urls", validateUUID(), func(params martini.Params, w http.ResponseWriter) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered in local file retrievel", r)
			}
		}()
		var urls []string
		urls = getBucketUrls(params["uuidv4"])
		if len(urls) == 0 {
			http.Error(w, "Buckets urls not found", http.StatusNotFound)
		} else {
			w.Header().Set("Content-Type", "application/json")
			list, _ := json.Marshal(urls)
			w.Write(list)
		}
	})

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

type streamHandler func(http.ResponseWriter, martini.Params, *http.Request)

//we can assume that params["uuidv4"] is a valid uuid version 4
func continueUpload(w http.ResponseWriter, params martini.Params, r *http.Request) {
	ff := CreateFlowFile(params, r)
	if !ff.ChunkExists(r) {
		w.WriteHeader(404)
		return
	}
}

func chunkedReader(w http.ResponseWriter, params martini.Params, r *http.Request) {
	r.ParseMultipartForm(25)

	ff := CreateFlowFile(params, r)
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
		if ff.NumberOfChunks() == cT {
			defer ff.Delete()
			imageStruct, err := exportFlowFile(ff, params["uuidv4"], r)
			if err != nil {
				panic(err.Error())
			}
			storeAttributes(imageStruct)
			imageStruct.Url = computeFullUrlFromPath(imageStruct.Url)
			imageStructBytes, err := json.Marshal(imageStruct)
			if err != nil {
				panic(err.Error())
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(imageStructBytes)
		}
	}
}

func computeFullUrlFromPath(path string) string {
	var fullURL string
	if cloudfrontURL != "" {
		cloudfrontURL = strings.TrimSuffix(cloudfrontURL, "/")
		fullURL = cloudfrontURL + "/" + path
	} else {
		auth, err := aws.EnvAuth()
		if err != nil {
			log.Fatal(err)
		}
		client := s3.New(auth, aws.USEast)
		bucket := client.Bucket(os.Getenv(s3Bucket))
		fullURL = bucket.URL(path)
	}
	return fullURL
}

func getDB() *sql.DB {
	connstring := os.Getenv("IMAGES_POSTGRESQL_DATABASE_STRING")
	db, err := sql.Open("postgres", connstring)
	if err != nil {
		panic(err.Error())
	}
	return db
}

func storeAttributes(imageData ImageData) {
	db := getDB()
	defer db.Close()
	uuidv4, url, height, width := imageData.Uuid, imageData.Url, imageData.Height, imageData.Width
	_, err := db.Query("insert into vault (uuid, url, height, width) values ($1, $2, $3, $4)", uuidv4, url, height, width)
	if err != nil {
		panic(err.Error())
	}
}

func getBucketUrls(uuidv4 string) []string {
	db := getDB()
	rows, err := db.Query("select url from vault where uuid = $1", uuidv4)
	if err != nil {
		panic(err.Error())
	}
	var urls []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		urls = append(urls, s)
	}
	return urls
}

func exportFlowFile(ff *FlowFile, uuidv4 string, r *http.Request) (ImageData, error) {
	imageRawBytes := ff.AssembleChunks()
	oldFileExt := ff.FileExtension(r)
	fileExt := ff.FileExtension(r)
	var imageBytes []byte
	if fileExt == ".png" {
		imageBytes = ConvertToJpegFromPng(imageRawBytes)
		fileExt = ".jpeg"
	} else {
		imageBytes = imageRawBytes
	}
	hash := sha256.New()
	hash.Write(imageBytes)
	md := hash.Sum(nil)
	fileName := hex.EncodeToString(md)
	filePath := fileName + fileExt
	fullFilePath := fmt.Sprintf("%s/%s", uuidv4, filePath)
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	client := s3.New(auth, aws.USEast)
	bucket := client.Bucket(os.Getenv(s3Bucket))
	mimeType := mime.TypeByExtension(fileExt)
	headers := map[string][]string{
		"Content-Type":  {mimeType},
		"Cache-Control": {"max-age=31536000"},
	}
	putError := bucket.PutHeader(fullFilePath, imageBytes, headers, s3.PublicRead)
	if putError != nil {
		return ImageData{}, putError
	}

	imageConfig := GetImageConfigFromBytesAndType(oldFileExt, imageRawBytes)

	return ImageData{
		Url:    fullFilePath,
		Uuid:   uuidv4,
		Height: imageConfig.Height,
		Width:  imageConfig.Width,
	}, nil
}
