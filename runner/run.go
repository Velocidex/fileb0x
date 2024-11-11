package runner

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Velocidex/fileb0x/compression"
	"github.com/Velocidex/fileb0x/config"
	"github.com/Velocidex/fileb0x/custom"
	"github.com/Velocidex/fileb0x/dir"
	"github.com/Velocidex/fileb0x/file"
	"github.com/Velocidex/fileb0x/template"
	"github.com/Velocidex/fileb0x/updater"
	"github.com/Velocidex/fileb0x/utils"
)

func Process(filename string) error {
	var (
		err     error
		cfg     *config.Config
		files   = make(map[string]*file.File)
		dirs    = new(dir.Dir)
		cfgPath string

		startTime = time.Now()

		modTimeStart = []byte("// modified(")
		modTimeEnd   = []byte(")")
	)

	// create config and try to get b0x file from args
	f := &config.File{
		FilePath: filename,
		Mode:     strings.TrimPrefix(path.Ext(filename), "."),
	}

	// load b0x file's config
	cfg, err = f.Load()
	if err != nil {
		panic(err)
	}

	err = cfg.Defaults()
	if err != nil {
		panic(err)
	}

	cfgPath = f.FilePath

	if err := cfg.Updater.CheckInfo(); err != nil {
		return err
	}

	cfg.Updater.IsUpdating = false

	// creates a config that can be inserTed into custom
	// without causing a import cycle
	sharedConfig := new(custom.SharedConfig)
	sharedConfig.Output = cfg.Output
	sharedConfig.Updater = cfg.Updater
	sharedConfig.Compression = compression.NewGzip()
	sharedConfig.Compression.Options = cfg.Compression

	// loop through b0x's [custom] objects
	for _, c := range cfg.Custom {
		err = c.Parse(&files, &dirs, sharedConfig)
		if err != nil {
			return err
		}
	}

	// builds remap's list
	var (
		remap    string
		modHash  string
		mods     []string
		lastHash string
	)

	for _, f := range files {
		remap += f.GetRemap()
		mods = append(mods, f.Modified)
	}

	// sorts modification time list and create a md5 of it
	sort.Strings(mods)
	modHash = stringMD5Hex(strings.Join(mods, "")) + "." + stringMD5Hex(string(f.Data))
	exists := fileExists(cfg.Dest + cfg.Output)

	if exists {
		/*
			// gets the modification hash from the main b0x file
			lastHash, err = getModification(cfg.Dest+cfg.Output, hashStart, hashEnd)
			if err != nil {
				panic(err)
			}
		*/
	}

	if !exists || lastHash != modHash {
		// create files template and exec it
		t := new(template.Template)
		t.Set("files")
		t.Variables = struct {
			ConfigFile       string
			Now              string
			Pkg              string
			Files            map[string]*file.File
			Tags             string
			Spread           bool
			Remap            string
			DirList          []string
			Compression      *compression.Options
			Debug            bool
			Updater          updater.Config
			ModificationHash string
		}{
			ConfigFile:       filepath.Base(cfgPath),
			Now:              time.Now().String(),
			Pkg:              cfg.Pkg,
			Files:            files,
			Tags:             cfg.Tags,
			Remap:            remap,
			Spread:           cfg.Spread,
			DirList:          dirs.Clean(),
			Compression:      cfg.Compression,
			Debug:            cfg.Debug,
			Updater:          cfg.Updater,
			ModificationHash: modHash,
		}

		tmpl, err := t.Exec()
		if err != nil {
			return err
		}

		if err := os.MkdirAll(cfg.Dest, 0770); err != nil {
			return err
		}

		// gofmt
		if cfg.Fmt {
			tmpl, err = format.Source(tmpl)
			if err != nil {
				return err
			}
		}

		// write final execuTed template into the destination file
		err = ioutil.WriteFile(cfg.Dest+cfg.Output, tmpl, 0640)
		if err != nil {
			return err
		}
	}

	// write spread files
	var (
		finalList   []string
		changedList []string
	)
	if cfg.Spread {
		a := strings.Split(path.Dir(cfg.Dest), "/")
		dirName := a[len(a)-1:][0]

		for _, f := range files {
			a := strings.Split(path.Dir(f.Path), "/")
			fileDirName := a[len(a)-1:][0]

			if dirName == fileDirName {
				continue
			}

			// transform / to _ and some other chars...
			customName := "b0xfile_" + utils.FixName(f.Path) + ".go"
			finalList = append(finalList, customName)

			exists := fileExists(cfg.Dest + customName)
			var mth string
			if exists {
				mth, err = getModification(cfg.Dest+customName, modTimeStart, modTimeEnd)
				if err != nil {
					return err
				}
			}

			changed := mth != f.Modified
			if changed {
				changedList = append(changedList, f.OriginalPath)
			}

			if !exists || changed {
				// creates file template and exec it
				t := new(template.Template)
				t.Set("file")
				t.Variables = struct {
					ConfigFile   string
					Now          string
					Pkg          string
					Path         string
					Name         string
					Dir          [][]string
					Tags         string
					Data         string
					Compression  *compression.Options
					Modified     string
					OriginalPath string
				}{
					ConfigFile:   filepath.Base(cfgPath),
					Now:          time.Now().String(),
					Pkg:          cfg.Pkg,
					Path:         f.Path,
					Name:         f.Name,
					Dir:          dirs.List,
					Tags:         f.Tags,
					Data:         f.Data,
					Compression:  cfg.Compression,
					Modified:     f.Modified,
					OriginalPath: f.OriginalPath,
				}
				tmpl, err := t.Exec()
				if err != nil {
					return err
				}

				// gofmt
				if cfg.Fmt {
					tmpl, err = format.Source(tmpl)
					if err != nil {
						return err
					}
				}

				// write final execuTed template into the destination file
				if err := ioutil.WriteFile(cfg.Dest+customName, tmpl, 0640); err != nil {
					return err
				}
			}
		}
	}

	// remove b0xfiles when [clean] is true
	// it doesn't clean destination's folders
	if cfg.Clean {
		matches, err := filepath.Glob(cfg.Dest + "b0xfile_*.go")
		if err != nil {
			return err
		}

		// remove matched file if they aren't in the finalList
		// which contains the list of all files written by the
		// spread option
		for _, f := range matches {
			var found bool
			for _, name := range finalList {
				if strings.HasSuffix(f, name) {
					found = true
				}
			}

			if !found {
				err = os.Remove(f)
				if err != nil {
					return err
				}
			}
		}
	}

	// main b0x
	if lastHash != modHash {
		log.Printf("fileb0x: took [%dms] to write [%s] from config file [%s] at [%s]",
			time.Since(startTime).Nanoseconds()/1e6, cfg.Dest+cfg.Output,
			filepath.Base(cfgPath), time.Now().String())
	} else {
		log.Printf("fileb0x: no changes detected")
	}

	// log changed files
	if cfg.Lcf && len(changedList) > 0 {
		log.Printf("fileb0x: list of changed files [%s]", strings.Join(changedList, " | "))
	}

	return nil
}

func getModification(path string, start []byte, end []byte) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var data []byte
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			return "", err
		}

		if !bytes.HasPrefix(line, start) || !bytes.HasSuffix(line, end) {
			continue
		}

		data = line
		break
	}

	hash := bytes.TrimPrefix(data, start)
	hash = bytes.TrimSuffix(hash, end)

	return string(hash), nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func stringMD5Hex(data string) string {
	hash := md5.New()
	hash.Write([]byte(data))
	return fmt.Sprintf("%x", hash.Sum(nil))
}