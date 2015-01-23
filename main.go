package main

import (
	"encoding/json"
	"flag"
	"github.com/facebookgo/errgroup"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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
	return filepath.Join("emoji", k+ext)
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

func BackupEmoji(key string) error {
	if _, err := os.Stat("emoji"); os.IsNotExist(err) {
		if err = os.Mkdir("emoji", 755); err != nil {
			return err
		}
	}

	if _, err := os.Stat("aliases"); os.IsNotExist(err) {
		if err = os.Mkdir("aliases", 755); err != nil {
			return err
		}
	}

	resp, err := http.Get("https://slack.com/api/emoji.list?token=" + key)
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

	wg := errgroup.Group{}

	for k, v := range er.Emoji {
		if strings.HasPrefix(v, "alias:") {
			continue
		}

		wg.Add(1)

		go func(name, url string) {
			err := SaveEmoji(name, url)
			if err != nil {
				wg.Error(err)
			} else {
				wg.Done()
			}
		}(k, v)
	}

	if err = wg.Wait(); err != nil {
		return err
	}

	for k, v := range er.Emoji {
		if !strings.HasPrefix(v, "alias:") {
			continue
		}

		alias := strings.Replace(v, "alias:", "", -1)
		image := Filename(alias, er.Emoji[alias])
		link := filepath.Join("aliases", k)

		if _, err := os.Stat(link); err == nil {
			log.Println("exists", link)
			continue
		}

		log.Println("linked", link)
		_ = os.Link(image, link)
	}

	return nil
}

func main() {
	flag.Parse()

	err := BackupEmoji(flag.Arg(0))

	if err != nil {
		log.Fatal(err)
	}
}
