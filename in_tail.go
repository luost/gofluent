package main

import (
	"encoding/json"
	"github.com/ActiveState/tail"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type inputTail struct {
	path     string
	format   string
	tag      string
	pos_file string

	offset int64
}

func (self *inputTail) Init(f map[string]string) error {
	value := f["path"]
	if len(value) > 0 {
		self.path = value
	}

	value = f["format"]
	if len(value) > 0 {
		self.format = value
	}

	value = f["tag"]
	if len(value) > 0 {
		self.tag = value
	}

	value = f["pos_file"]
	if len(value) > 0 {
		self.pos_file = value

		str, err := ioutil.ReadFile(self.pos_file)
		if err != nil {
			Log(err)
		}

		f, err := os.Open(self.path)
		if err != nil {
			Log(err)
		}

		info, err := f.Stat()
		if err != nil {
			Log(err)
		} else {
			offset, _ := strconv.Atoi(string(str))
			if int64(offset) > info.Size() {
				self.offset = info.Size()
			} else {
				self.offset = int64(offset)
			}
		}
	}

	return nil
}

func (self *inputTail) Run(runner InputRunner) error {

	var seek int
	if self.offset > 0 {
		seek = os.SEEK_SET
	} else {
		seek = os.SEEK_END
	}

	t, err := tail.TailFile(self.path, tail.Config{
		Poll:      true,
		ReOpen:    true,
		Follow:    true,
		MustExist: false,
		Location:  &tail.SeekInfo{int64(self.offset), seek}})
	if err != nil {
		return err
	}

	var re regexp.Regexp
	if string(self.format[0]) == string("/") || string(self.format[len(self.format)-1]) == string("/") {
		re = *regexp.MustCompile(strings.Trim(self.format, "/"))
		self.format = "regexp"
	} else if self.format == "json" {

	}

	f, err := os.OpenFile(self.pos_file, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		Log(err)
	}
	defer f.Close()

	for line := range t.Lines {
		pack := <-runner.InChan()

		pack.MsgBytes = []byte(line.Text)
		pack.Msg.Tag = self.tag
		pack.Msg.Timestamp = line.Time.Unix()

		if self.format == "regexp" {
			text := re.FindSubmatch([]byte(line.Text))
			if text == nil {
				continue
			}

			for i, name := range re.SubexpNames() {
				if i != 0 {
					pack.Msg.Data[name] = string(text[i])
				}
			}
		} else if self.format == "json" {
			err := json.Unmarshal([]byte(line.Text), &pack.Msg.Data)
			if err != nil {
				continue
			}
		}

		offset, err := t.Tell()
		if err != nil {
			Log("Tell return error: ", err)
		}

		str := strconv.Itoa(int(offset))

		_, err = f.WriteString(str)
		if err != nil {
			Log(err)
		}

		runner.RouterChan() <- pack
	}

	err = t.Wait()
	if err != nil {
		return err
	}

	return err
}

func init() {
	RegisterInput("tail", func() interface{} {
		return new(inputTail)
	})
}