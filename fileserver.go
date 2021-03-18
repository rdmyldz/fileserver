package main

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

func (app *application) handleHome(w http.ResponseWriter, r *http.Request) {
	log.Printf("in handleHome -req path: %q", r.URL.Path)
	tmpl := app.templateCache["homepage.html"]
	files, err := listFiles(*targetDir)
	if err != nil {
		log.Println("error while listing targetDir:", err)
		http.Error(w, "internal server error while listing", http.StatusInternalServerError)
		return
	}
	filesPathes := make([]FileInfo, 0, len(files))
	for i, f := range files {
		fPath := filepath.Join("download", f.Name())
		file := FileInfo{Fname: f.Name(), FPath: fPath, Index: i}
		filesPathes = append(filesPathes, file)
	}

	tmpl.Execute(w, filesPathes)

}

func (app *application) handleServingFiles(w http.ResponseWriter, r *http.Request) {
	log.Println("handleServingFiles just got hit")
	log.Printf("path:%q\n", r.URL.Path)
	tmpl := app.templateCache["files.html"]

	trimmedPath := strings.TrimPrefix(r.URL.Path, "/files/")
	sourcePath := filepath.Join(*sourceDir, trimmedPath)
	files, err := listFiles(sourcePath)
	if err != nil {
		var perr *fs.PathError
		if errors.As(err, &perr) {
			http.ServeFile(w, r, sourcePath) // if it's not a directory,just serve it
			return
		}
		log.Printf("error returned from listFiles(): %v\n", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	filesPathes := make([]FileInfo, 0, len(files))
	for i, f := range files {
		fPath := path.Join(r.URL.Path, f.Name())
		if f.IsDir() {
			file := FileInfo{Fname: f.Name(), FPath: fPath, IsDir: true, Index: i}
			filesPathes = append(filesPathes, file)
			continue
		}

		file := FileInfo{Fname: f.Name(), FPath: fPath, Index: i}
		filesPathes = append(filesPathes, file)
	}

	tmpl.Execute(w, filesPathes)
}

func (app *application) makeZip(w http.ResponseWriter, r *http.Request) {
	log.Printf("inside makeZip handler - %q\n", r.URL.Path)

	val := r.FormValue("zip")
	log.Printf("form value is %q\n", val)
	file := strings.TrimPrefix(val, "/files/")
	log.Printf("file: %q\n", file)
	log.Printf("%s is getting zipped...", file)
	err := zipIt(file, w)
	if err != nil {
		log.Printf("returned error from zipIt: %v\n", err)
		http.Error(w, "internal server error: the file not zipped", http.StatusInternalServerError)
		return
	}

	log.Printf("%s is just zipped...", file)
}

func zipIt(dirname string, w io.Writer) error {
	fmt.Println("*********************")
	baseName := filepath.Base(dirname)
	zipName := baseName + ".zip"
	dirPath := filepath.Dir(dirname)
	fmt.Printf("zipName: %q -- dirPath: %q\n", zipName, dirPath)
	fmt.Println("*********************")
	zw := zip.NewWriter(w)
	defer zw.Close()

	walkdir := filepath.Join(*sourceDir, dirPath, baseName)
	fmt.Printf("walkdir: %q\n", walkdir)
	err := filepath.Walk(walkdir, func(path string, info fs.FileInfo, err error) error {
		log.Printf("inside walkFunc -- walkdir: %q\n", walkdir)
		log.Printf("walking: %#v\n", path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			log.Printf("error while opening the file path: %q\n", path)
			return err
		}
		defer file.Close()
		newPath := strings.TrimPrefix(path, walkdir)
		log.Printf("newPath: %q\n", newPath)
		f, err := zw.Create(newPath)
		if err != nil {
			log.Printf("zw.Create(newPath) - error while creating the file path: %q\n", newPath)
			return err
		}
		_, err = io.Copy(f, file)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (app *application) handleDownload(w http.ResponseWriter, r *http.Request) {
	log.Println("r.url.path:", r.URL.Path)
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	log.Printf("filename: %q\n", filename)
	fPath := filepath.Join(*targetDir, filename)
	log.Printf("fPath: %q\n", fPath)
	http.ServeFile(w, r, fPath)
}

type application struct {
	templateCache map[string]*template.Template
}

// FileInfo holds the file infos
type FileInfo struct {
	IsDir bool // true if its a directory
	FPath string
	Fname string
	Index int
}

func (app *application) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		app.handleHome(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/files/") && r.Method == "GET" {
		app.handleServingFiles(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/files/") && r.Method == "POST" {
		w.Header().Set("Content-Disposition", "attachment; filename=Wiki.zip")
		w.Header().Set("Content-Type", "application/octet-stream")
		app.makeZip(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/download/") && r.Method == "GET" {
		app.handleDownload(w, r)
		return
	}
}

var sourceDir = flag.String("s", "assets", "source directory path")
var targetDir = flag.String("t", "tmp-assets", "source directory path")
var port = flag.String("p", ":8080", "port for listening")

func main() {

	flag.Parse()

	templateCache, err := newTemplateCache("templates")
	if err != nil {
		log.Fatalln("error while making template cache: ", err)
	}

	app := &application{templateCache: templateCache}

	log.Printf("starting server on %s\n", *port)
	err = http.ListenAndServe(*port, app)
	if err != nil {
		log.Fatalln(err)
	}
}

func listFiles(path string) ([]fs.DirEntry, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func newTemplateCache(dir string) (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}
	pages, err := filepath.Glob(filepath.Join(dir, "*.html"))
	log.Printf("pages in the func: %s\n", pages)
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		ts, err := template.New(name).ParseFiles(page)
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	log.Printf("cache map in the func: %+v\n", cache)
	return cache, nil
}
