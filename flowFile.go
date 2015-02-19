package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/go-martini/martini"
	"net/http"
	"os"
	"path/filepath"
)

type FlowFile struct {
	name string
}

func CreateFlowFile(params martini.Params, r *http.Request) *FlowFile {
	return &FlowFile{params["uuidv4"] + r.FormValue("flowIdentifier")}
}

func (ff *FlowFile) getBolt() *bolt.DB {
	db, err := bolt.Open(os.Getenv(boltChunks), 0600, nil)
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
	if err != nil {
		panic(err)
	}
	return numKeys
}

func (ff *FlowFile) AssembleChunks() []byte {
	numChunks := ff.NumberOfChunks()
	chunks := make([][]byte, numChunks, numChunks)
	db := ff.getBolt()
	defer db.Close()
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ff.name))
		if bucket != nil {
			cursor := bucket.Cursor()
			i := 0
			for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
				chunks[i] = v
				i = i + 1
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return bytes.Join(chunks, nil)
}

func (ff *FlowFile) FileExtension(r *http.Request) string {
	return filepath.Ext(r.FormValue("flowFilename"))
}

func (ff *FlowFile) Delete() {
	db := ff.getBolt()
	defer db.Close()
	err := db.Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket([]byte(ff.name))
	})
	if err != nil {
		panic(err)
	}
}
