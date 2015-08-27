package main

import (
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var apiKey string
var directory string

type EmojiResponse struct {
	OK           bool              `json:"ok"`
	Emoji        map[string]string `json:"emoji"`
	ErrorMessage string            `json:"error"`
}

func (er EmojiResponse) Error() string {
	return er.ErrorMessage
}

func (er EmojiResponse) String() string {
	return er.ErrorMessage
}

func Filename(k, url string) string {
	ext := filepath.Ext(url)
	if ext == "" {
		ext = ".png"
	}
	return filepath.Join(directory, k+ext)
}

func cp(dst, src string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	// no need to check errors on read only file, we already got everything
	// we need from the filesystem, so nothing can go wrong now.
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}

func SaveEmoji(name, url string) error {
	filename := Filename(name, url)
	if _, err := os.Stat(filename); err == nil {
		log.Println("exists", filename)
		return nil
	}

	log.Println("saved", filename)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	blob, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, blob, 0644)
}

func BackupEmoji() error {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		if err = os.MkdirAll(directory, 0755); err != nil {
			return err
		}
	}

	resp, err := http.Get("https://slack.com/api/emoji.list?token=" + apiKey)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	blob, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var er EmojiResponse

	if err := json.Unmarshal(blob, &er); err != nil {
		return err
	}

	if !er.OK {
		return er
	}

	type emoPair struct {
		name string
		url  string
	}

	var wg sync.WaitGroup

	emoChan := make(chan emoPair)

	for i := 1; i <= 15; i++ {
		wg.Add(1)
		go func() {
			for {
				ep, ok := <-emoChan
				if !ok {
					wg.Done()
					return
				}
				err := SaveEmoji(ep.name, ep.url)
				if err != nil {
					log.Printf("API error: %s\n", err)
				}
			}
		}()
	}

	for k, v := range er.Emoji {
		if strings.HasPrefix(v, "alias:") {
			continue
		}
		emoChan <- emoPair{k, v}
	}

	close(emoChan)
	wg.Wait()

	for k, v := range er.Emoji {
		if !strings.HasPrefix(v, "alias:") {
			continue
		}

		alias := strings.Replace(v, "alias:", "", -1)
		image := Filename(alias, er.Emoji[alias])
		ext := filepath.Ext(image)
		if ext == "" {
			ext = ".png"
		}
		link := filepath.Join(directory, k+ext)

		if _, err := os.Stat(link); err == nil {
			log.Println("exists", link)
			continue
		}

		log.Println("linked", link)
		_ = cp(image, link)
	}

	return nil
}

func main() {
	flag.StringVar(&apiKey, "key", "", "Slack API key (see https://api.slack.com/web)")
	flag.Parse()

	if apiKey == "" {
		log.Fatal("API key is required")
	}

	directory = flag.Arg(0)
	if directory == "" {
		directory = "emoji"
	}

	err := BackupEmoji()

	if err != nil {
		log.Fatal(err)
	}
}
