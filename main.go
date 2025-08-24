package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

type Base struct {
	count int
	Data  map[string]string `json:"data"`
}

func (b *Base) Set(key, value string) {
	if b.Data == nil {
		b.Data = make(map[string]string)
	}
	b.Data[key] = value
}

func (b *Base) Get(key string) (string, bool) {
	resp := b.Data[key]
	if resp != "" {
		return resp, true
	}
	return resp, false
}

func (b *Base) Delete(key string) {
	b.Data[key] = ""
}

func (b *Base) List() {
	for key, value := range b.Data {
		fmt.Printf("| \t%10s|\t%10s |\n", key, value)
	}
}

func (b *Base) Save() {
	path, err := os.UserHomeDir()
	if err != nil {
		log.Println("failed to find home directory", err)
	}

	data, err := json.Marshal(b)
	if err != nil {
		log.Println("failed to marshal to json", err)
	}
	path = filepath.Join(path, ".blue")
	file, _ := os.Create(path)
	defer file.Close()
	_, _ = file.Write(data)
	fmt.Println("File created successfully")
}

func (b *Base) Load() {
	if b.Data == nil {
		b.Data = make(map[string]string)
	}
	path, _ := os.UserHomeDir()
	path = filepath.Join(path, ".blue")
	file, _ := os.Open(path)
	defer file.Close()
	data, _ := io.ReadAll(file)
	json.Unmarshal(data, b)
}

func main() {
	var b Base
	b.Set("sleep", "me")
	b.Set("spinach", "test")
	b.Save()

	c := Base{}
	c.Load()
	c.List()
}
