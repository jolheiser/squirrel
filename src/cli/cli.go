package cli

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/schollz/fbdb"
	log "github.com/schollz/logger"
	"github.com/schollz/progressbar/v2"
	"github.com/schollz/squirrel/src/get"
	"github.com/urfave/cli/v2"
)

func init() {
	log.SetLevel("trace")
}

func Run() (err error) {

	app := cli.NewApp()
	app.Name = "squirrel"
	app.Version = "v1.1.0-744a887"
	app.Compiled = time.Now()
	app.Usage = "download URLs directly into an SQLite database"
	app.Flags = []cli.Flag{
		&cli.StringSliceFlag{Name: "headers", Aliases: []string{"H"}, Usage: "headers to include"},
		&cli.BoolFlag{Name: "tor"},
		&cli.BoolFlag{Name: "no-clobber", Aliases: []string{"nc"}},
		&cli.StringFlag{Name: "list", Aliases: []string{"i"}},
		&cli.StringFlag{Name: "pluck", Aliases: []string{"p"}, Usage: "file for plucking"},
		&cli.StringFlag{Name: "cookies", Aliases: []string{"c"}},
		&cli.BoolFlag{Name: "compressed"},
		&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}},
		&cli.BoolFlag{Name: "strip-js"},
		&cli.BoolFlag{Name: "strip-css"},
		&cli.IntFlag{Name: "workers", Aliases: []string{"w"}, Value: 1},
		&cli.BoolFlag{Name: "dump", Usage: "dump database file to disk"},
		&cli.StringFlag{Name: "db", Usage: "name of SQLite database to use", Value: "urls.db"},
		&cli.BoolFlag{Name: "debug", Usage: "increase verbosity"},
	}
	app.Action = func(c *cli.Context) error {
		if c.Bool("debug") {
			log.SetLevel("trace")
		} else {
			log.SetLevel("warn")
		}
		if c.Bool("dump") {
			return dump(c)
		}
		return runget(c)
	}

	// ignore error so we don't exit non-zero and break gfmrun README example tests
	return app.Run(os.Args)
}

func runget(c *cli.Context) (err error) {
	w := get.Get{}
	w.Debug = c.Bool("debug")
	w.DBName = c.String("db")
	if c.Args().First() != "" {
		w.URL = c.Args().First()
	} else if c.String("list") != "" {
		w.FileWithList = c.String("list")
	} else {
		return errors.New("need to specify URL")
	}
	if c.Bool("debug") {
		log.SetLevel("trace")
	} else if c.Bool("quiet") {
		log.SetLevel("error")
	} else {
		log.SetLevel("info")
	}
	w.Headers = c.StringSlice("headers")
	w.NoClobber = c.Bool("no-clobber")
	w.UseTor = c.Bool("tor")
	w.StripCSS = c.Bool("strip-css")
	w.StripJS = c.Bool("strip-js")
	w.CompressResults = c.Bool("compressed")
	w.NumWorkers = c.Int("workers")
	w.Cookies = c.String("cookies")
	if w.NumWorkers < 1 {
		return errors.New("cannot have less than 1 worker")
	}
	if c.String("pluck") != "" {
		b, err := ioutil.ReadFile(c.String("pluck"))
		if err != nil {
			return err
		}
		w.PluckerTOML = string(b)
	}

	w2, _ := get.New(w)
	return w2.Run()
}

func dump(c *cli.Context) (err error) {
	start := time.Now()
	_, err = os.Stat(c.String("db"))
	if err != nil {
		return
	}
	fs, err := fbdb.Open(c.String("db"))
	if err != nil {
		return
	}
	numFiles, err := fs.Len()
	if err != nil {
		return
	}
	var bar *progressbar.ProgressBar
	if !c.Bool("debug") {
		bar = progressbar.NewOptions(numFiles,
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
		)
		log.SetLevel("info")
	}
	log.Infof("dumping %s", c.String("db"))
	for i := 0; i < numFiles; i++ {
		if !c.Bool("debug") {
			bar.Add(1)
		}
		var f fbdb.File
		f, err = fs.GetI(i)
		if err != nil {
			return
		}
		pathname, filename := path.Split(f.Name)
		if filename == "" {
			filename = "index.html"
		}
		if !strings.Contains(filename, ".") {
			pathname = path.Join(pathname, filename)
			filename = "index.html"
		}
		log.Debugf("path: '%s', file: '%s'", pathname, filename)
		if _, err = os.Stat(pathname); os.IsNotExist(err) {
			err = os.MkdirAll(pathname, 0755)
			if err != nil {
				log.Error(err)
				continue
			}
		}
		err = ioutil.WriteFile(path.Join(pathname, filename), f.Data, 0644)
		if err != nil {
			log.Errorf("could not write in folder '%s' with file '%s': %s", pathname, filename, err.Error())
			continue
		}
	}
	bar.Finish()
	log.Infof("finished dumping %d records [%s]", numFiles, time.Since(start))
	return
}
